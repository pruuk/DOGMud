package baubles

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// setBaubleConfig overrides bauble settings for one test.
func setBaubleConfig(t *testing.T, change func(b *configs.Balance)) {
	t.Helper()
	cfg := configs.GetConfig()
	change(&cfg.Balance)
	configs.SetConfigForTest(t, cfg)
}

func always(v int) func(int) int { return func(int) int { return v } }

func TestTakeRollSpendsTheWindowThenReopens(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(1_000_000, 0)
	hour := time.Hour

	if !takeRoll(1, 10, false, 2, hour, t0) || !takeRoll(1, 11, false, 2, hour, t0.Add(time.Minute)) {
		t.Fatal("two rolls per window, shared by the room's players")
	}
	if takeRoll(1, 12, false, 2, hour, t0.Add(2*time.Minute)) {
		t.Fatal("a third search in the window gets no roll")
	}
	if !takeRoll(2, 10, false, 2, hour, t0) {
		t.Fatal("another room has its own window")
	}
	// The window is measured from its FIRST roll.
	if takeRoll(1, 10, false, 2, hour, t0.Add(59*time.Minute)) {
		t.Fatal("still closed before the hour is up")
	}
	if !takeRoll(1, 10, false, 2, hour, t0.Add(hour)) {
		t.Fatal("reopens an hour after the first roll")
	}
}

func TestTakeRollPerPlayer(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(1_000_000, 0)
	takeRoll(1, 10, true, 1, time.Hour, t0)
	if takeRoll(1, 10, true, 1, time.Hour, t0) {
		t.Fatal("player 10 has spent their roll")
	}
	if !takeRoll(1, 11, true, 1, time.Hour, t0) {
		t.Fatal("per-player: player 11 has their own")
	}
}

func TestWindowStateAndReset(t *testing.T) {
	resetAllWindowsForTest()
	now := time.Now()
	if _, _, _, open := WindowState(5, 0, now); open {
		t.Fatal("no window before any search")
	}
	takeRoll(5, 0, false, 2, time.Hour, now)
	used, allowed, reopens, open := WindowState(5, 0, now)
	if !open || used != 1 || allowed != 2 || !reopens.Equal(now.Add(time.Hour)) {
		t.Fatalf("state: %d/%d %v %v", used, allowed, reopens, open)
	}
	ResetWindow(5)
	if _, _, _, open := WindowState(5, 0, now); open {
		t.Fatal("reset clears it")
	}
}

func TestPickTierByWeight(t *testing.T) {
	w := [3]int{70, 25, 5}
	cases := map[int]ValueTier{0: TierCheap, 69: TierCheap, 70: TierAverage, 94: TierAverage, 95: TierRare, 99: TierRare}
	for roll, want := range cases {
		if got := pickTier(w, always(roll)); got != want {
			t.Fatalf("roll %d: got %s want %s", roll, got, want)
		}
	}
	if got := pickTier([3]int{0, 0, 5}, always(3)); got != TierRare {
		t.Fatal("a zero weight is skipped")
	}
	if got := pickTier([3]int{0, 0, 0}, always(0)); got != TierCheap {
		t.Fatal("no weights at all is cheap")
	}
	if got := PickTier(always(0)); got != TierCheap {
		t.Fatal("configured weights start with cheap")
	}
}

// A household's find and a pickpocketed bauble each take their own weights;
// every other find takes the search weights. Three sets that each allow one
// tier only show which set was used.
func TestStolenFindsUseTheirOwnWeights(t *testing.T) {
	setBaubleConfig(t, func(b *configs.Balance) {
		b.BaubleTierWeightCheap, b.BaubleTierWeightAverage, b.BaubleTierWeightRare = 1, 0, 0
		b.BaubleHouseholdTierWeightCheap, b.BaubleHouseholdTierWeightAverage, b.BaubleHouseholdTierWeightRare = 0, 0, 1
		b.BaublePickpocketTierWeightCheap, b.BaublePickpocketTierWeightAverage, b.BaublePickpocketTierWeightRare = 0, 1, 0
	})
	resetAllWindowsForTest()
	place := NewPlace(4033, `ashwick`, `Windward Marches`, `interior`)
	now := time.Unix(2_000_000, 0)

	if tier, found := RollFind(FindOpts{Place: place, UserId: 7, Feature: `shelf`, Randn: always(0), Now: now}); !found || tier != TierCheap {
		t.Fatalf("a find nobody keeps takes the search weights: %s %v", tier, found)
	}
	if tier, found := RollFind(FindOpts{Place: place, UserId: 7, Feature: `chest`, Household: true, Randn: always(0), Now: now}); !found || tier != TierRare {
		t.Fatalf("a household's find takes the household weights: %s %v", tier, found)
	}
	if tier := PickPocketTier(always(0)); tier != TierAverage {
		t.Fatalf("a pickpocketed bauble takes the pickpocket weights: %s", tier)
	}
	if tier := PickTier(always(0)); tier != TierCheap {
		t.Fatalf("PickTier is still the search weights: %s", tier)
	}
}

func TestRollChance(t *testing.T) {
	if !rollChance(3, always(29999)) || rollChance(3, always(30000)) {
		t.Fatal("3% is 30000 in a million")
	}
	if !rollChance(0.325, always(3249)) || rollChance(0.325, always(3250)) {
		t.Fatal("fine fractional chances are rolled as written")
	}
	if rollChance(0, always(0)) || !rollChance(100, always(9999)) {
		t.Fatal("0 never, 100 always")
	}

	hits := 0
	const n = 20000
	for range n {
		if rollChance(3, util.Rand) {
			hits++
		}
	}
	if pct := float64(hits) * 100 / n; pct < 2.3 || pct > 3.7 {
		t.Fatalf("3%% over %d rolls came out at %.2f%%", n, pct)
	}
}

func TestRollFindWithinTheWindow(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(2_000_000, 0)
	place := NewPlace(4033, `ashwick`, `Windward Marches`, `forest`)

	tier, found := RollFind(FindOpts{Place: place, UserId: 7, Randn: always(0), Now: t0})
	if !found || tier != TierCheap {
		t.Fatalf("find: %s %v", tier, found)
	}

	// A failed roll still spends the window's second roll.
	if _, found := RollFind(FindOpts{Place: place, UserId: 7, Randn: always(9999), Now: t0}); found {
		t.Fatal("a roll of 9999 in a million fails at the forest's 0.25%")
	}
	if _, found := RollFind(FindOpts{Place: place, UserId: 8, Randn: always(0), Now: t0}); found {
		t.Fatal("the window is spent; nothing more for an hour")
	}
	if _, found := RollFind(FindOpts{Place: place, UserId: 8, Randn: always(0), Now: t0.Add(time.Hour)}); !found {
		t.Fatal("an hour later the room offers rolls again")
	}
}

func TestRollFindRespectsTheSwitchAndExcludedZones(t *testing.T) {
	resetAllWindowsForTest()
	place := NewPlace(1, `tutorial`, ``, `city`)

	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleExcludedZones = configs.ConfigSliceString{`Tutorial`} })
	if _, found := RollFind(FindOpts{Place: place, Randn: always(0)}); found {
		t.Fatal("excluded zones never find (case-insensitive)")
	}
	if _, _, _, open := WindowState(1, 0, time.Now()); open {
		t.Fatal("an excluded zone does not even open a window")
	}

	setBaubleConfig(t, func(b *configs.Balance) { b.BaublesEnabled = false })
	if _, found := RollFind(FindOpts{Place: NewPlace(2, `ashwick`, ``, `forest`), Randn: always(0)}); found {
		t.Fatal("off (the default): search finds no baubles")
	}
}

func TestRevealDelay(t *testing.T) {
	if RevealDelay() != 3*time.Second {
		t.Fatalf("default reveal delay: %s", RevealDelay())
	}
}

func TestLadderComesFromConfig(t *testing.T) {
	setBaubleConfig(t, func(b *configs.Balance) {
		b.BaubleRareMinValue = 50
		b.BaubleRareMaxValue = 300
	})
	if r := TierRare.Range(); r.Min != 50 || r.Max != 300 {
		t.Fatalf("rare: %+v", r)
	}
	if TierRare.ClampValue(1000) != 300 {
		t.Fatal("clamp uses the configured ladder")
	}
}

func TestChanceByBiomeAndSkill(t *testing.T) {
	cases := []struct {
		biome string
		skill float64
		want  float64
	}{
		{`interior`, 0, 5},
		{`interior`, 1, 10}, // BaubleSkillMaxBonus 1.0 doubles it
		{`Interior`, 0, 5},  // biome ids ignore case
		{`city_thoroughfare`, 0, 2},
		{`forest`, 0, 0.25},
		{`forest`, 0.5, 0.375},
		{`forest`, 7, 0.5}, // the factor is capped at 1
		{`city`, 0, 1},     // a biome not in the table uses BaubleSearchChancePct
		{``, 0, 1},
		{`water`, 1, 0},
	}
	for _, c := range cases {
		if got := ChanceFor(c.biome, c.skill); got != c.want {
			t.Errorf("%s at skill %.2f: got %v want %v", c.biome, c.skill, got, c.want)
		}
	}
	if BaseChance(`sewer`) != 3 || BaseChance(`nowhere`) != 1 {
		t.Fatal("base chance")
	}
	setBaubleConfig(t, func(b *configs.Balance) {
		b.BaubleBiomeChancePct = map[string]float64{`interior`: 80}
		b.BaubleSkillMaxBonus = 1
	})
	if ChanceFor(`interior`, 1) != 100 {
		t.Fatal("the chance is capped at 100%")
	}
}

func TestRollFindUsesTheRoomAndTheSearcher(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(3_000_000, 0)

	// Deep water: nothing to find, and no window is even opened.
	if _, found := RollFind(FindOpts{Place: NewPlace(501, `lake`, ``, `water`), Randn: always(0), Now: t0}); found {
		t.Fatal("nothing is found in deep water")
	}
	if _, _, _, open := WindowState(501, 0, t0); open {
		t.Fatal("a room where nothing can be found spends no roll")
	}

	// A roll of 60000 in a million (6%) misses an unskilled search indoors
	// (5%) and hits a skilled one (up to 10%).
	if _, found := RollFind(FindOpts{Place: NewPlace(502, `town`, ``, `interior`), Randn: always(60000), Now: t0}); found {
		t.Fatal("6% misses 5%")
	}
	if _, found := RollFind(FindOpts{Place: NewPlace(503, `town`, ``, `interior`), SkillFactor: 1, Randn: always(60000), Now: t0}); !found {
		t.Fatal("6% hits a skilled searcher's 10%")
	}
}

// A feature is searched once per BaubleFeatureWindowMinutes, claimed by the
// search itself; its search never spends the room's rolls, and each feature
// has its own window.
func TestFeatureSearchedOncePerWindow(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(4_000_000, 0)
	room := NewPlace(601, `town`, ``, `interior`)
	hit := always(0)

	// Spend the room's two rolls.
	for i := range 2 {
		if _, found := RollFind(FindOpts{Place: room, Randn: hit, Now: t0}); !found {
			t.Fatalf("room roll %d", i)
		}
	}
	if _, found := RollFind(FindOpts{Place: room, Randn: hit, Now: t0}); found {
		t.Fatal("the room's window is spent")
	}

	// The bookshelf can still be searched, once.
	if ok, _, _ := FeatureSearchable(601, 10, `bookshelf`, t0); !ok {
		t.Fatal("an unsearched feature is searchable")
	}
	if !ClaimFeatureSearch(601, 10, `bookshelf`, t0) {
		t.Fatal("first search of the bookshelf")
	}
	if _, found := RollFind(FindOpts{Place: room, Feature: `bookshelf`, Randn: hit, Now: t0}); !found {
		t.Fatal("a claimed feature rolls, whatever the room's window")
	}
	if ClaimFeatureSearch(601, 11, `bookshelf`, t0.Add(59*time.Minute)) {
		t.Fatal("once per hour, whoever searches")
	}
	ok, byYou, reopens := FeatureSearchable(601, 11, `bookshelf`, t0.Add(time.Minute))
	if ok || byYou || !reopens.Equal(t0.Add(time.Hour)) {
		t.Fatalf("another player sees it searched: %v %v %v", ok, byYou, reopens)
	}
	if _, byYou, _ := FeatureSearchable(601, 10, `bookshelf`, t0.Add(time.Minute)); !byYou {
		t.Fatal("the searcher is told it was their own search")
	}
	if !ClaimFeatureSearch(601, 10, `table`, t0) {
		t.Fatal("another feature has its own window")
	}
	if !ClaimFeatureSearch(601, 10, `bookshelf`, t0.Add(time.Hour)) {
		t.Fatal("the bookshelf reopens after the window")
	}
	if ClaimFeatureSearch(601, 10, ``, t0) {
		t.Fatal("the room itself is not a feature")
	}

	used, allowed, _, open := FeatureWindowState(601, 0, `table`, t0)
	if !open || used != 1 || allowed != 1 {
		t.Fatalf("feature window: %d/%d %v", used, allowed, open)
	}
	if used, allowed, _, _ := WindowState(601, 0, t0); used != 2 || allowed != 2 {
		t.Fatalf("room window untouched by features: %d/%d", used, allowed)
	}

	ResetWindow(601)
	if ok, _, _ := FeatureSearchable(601, 10, `table`, t0); !ok {
		t.Fatal("reset clears the room's features too")
	}
}

// The searcher's sight costs the chance (owner ruling 2026-09-28, lighting
// plan 5b's ramp): FindOpts.SightPenalty is 1 - messaging.SightMult, 0 is
// none. A search in the dark still spends its window roll, exactly as one
// in the light does.
func TestRollFindPaysTheSightPenalty(t *testing.T) {
	resetAllWindowsForTest()
	t0 := time.Unix(4_000_000, 0)

	// 40000 in a million is 4%: under an unskilled indoor search's 5%...
	if _, found := RollFind(FindOpts{Place: NewPlace(601, `town`, ``, `interior`), Randn: always(40000), Now: t0}); !found {
		t.Fatal("4% hits 5% in the light")
	}
	// ...and not under 5% x (1 - 0.3) = 3.5%, well clear of the 4% roll
	// (a case exactly on the threshold would hang on float rounding).
	if _, found := RollFind(FindOpts{Place: NewPlace(602, `town`, ``, `interior`), SightPenalty: 0.3, Randn: always(40000), Now: t0}); found {
		t.Fatal("the dark costs the chance")
	}
	if used, _, _, open := WindowState(602, 0, t0); !open || used != 1 {
		t.Fatalf("a search in the dark spends its window roll: used %d open %v", used, open)
	}

	// Out of range is clamped: below 0 is none, above 1 is all.
	if _, found := RollFind(FindOpts{Place: NewPlace(603, `town`, ``, `interior`), SightPenalty: -3, Randn: always(40000), Now: t0}); !found {
		t.Fatal("a negative penalty is none")
	}
	if _, found := RollFind(FindOpts{Place: NewPlace(604, `town`, ``, `interior`), SightPenalty: 7, Randn: always(0), Now: t0}); found {
		t.Fatal("a penalty above 1 leaves no chance")
	}
	if used, _, _, open := WindowState(604, 0, t0); !open || used != 1 {
		t.Fatalf("even a hopeless search spends its window roll: used %d open %v", used, open)
	}
}
