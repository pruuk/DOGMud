package gather

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// win builds a contest result won (or lost, when sigmas < 0) by the given
// number of margin standard deviations, with stdDev 10 per roll.
func win(sigmas float64) contest.Result {
	const sd = 10.0
	m := sigmas * sd * 1.4142135623730951
	return contest.Result{
		AttackRoll: dice.RollResult{StdDev: sd},
		Contested:  true,
		Margin:     m,
		Success:    m > 0,
	}
}

func TestGradeFrom_MarginSetsGrade(t *testing.T) {
	steel := Inputs{UsesTool: true, ToolTier: items.ToolTierMasterwork}
	cases := []struct {
		sigmas float64
		want   items.Quality
		ok     bool
	}{
		{0.1, items.QualityStandard, true},
		{1.2, items.QualityFine, true},
		{2.5, items.QualitySuperb, true},
		{3.4, items.QualityPristine, true},
		{9.0, items.QualityPristine, true}, // clamped
		{-0.3, items.QualityCrude, true},   // narrow loss still yields crude
		{-1.5, items.QualityNone, false},   // clear loss yields nothing
	}
	for _, c := range cases {
		got := gradeFrom(win(c.sigmas), 100, 100, steel)
		if got.Success != c.ok || got.Grade != c.want {
			t.Errorf("sigmas %.1f: got (%v, %v), want (%v, %v)", c.sigmas, got.Success, got.Grade, c.ok, c.want)
		}
	}
}

func TestGradeFrom_ToolTierCapsGrade(t *testing.T) {
	cases := []struct {
		tier items.ToolTier
		want items.Quality
	}{
		{items.ToolTierNone, items.QualityStandard}, // tool job done without the tool = crude cap
		{items.ToolTierCrude, items.QualityStandard},
		{items.ToolTierIron, items.QualityFine},
		{items.ToolTierSteel, items.QualitySuperb},
		{items.ToolTierMasterwork, items.QualityPristine},
	}
	for _, c := range cases {
		got := gradeFrom(win(5), 100, 100, Inputs{UsesTool: true, ToolTier: c.tier})
		if got.Grade != c.want {
			t.Errorf("tier %v: grade %v, want %v", c.tier, got.Grade, c.want)
		}
	}
	// A job with no tool type is capped only at pristine.
	if got := gradeFrom(win(5), 100, 100, Inputs{}); got.Grade != items.QualityPristine {
		t.Errorf("toolless job: grade %v, want pristine", got.Grade)
	}
}

func TestGradeFrom_FlooredWinIsCrude(t *testing.T) {
	cr := contest.Result{Contested: true, Success: true, Floored: true, Margin: 1}
	got := gradeFrom(cr, 50, 150, Inputs{UsesTool: true, ToolTier: items.ToolTierSteel})
	if !got.Success || got.Grade != items.QualityCrude {
		t.Errorf("floored win: got (%v, %v), want crude", got.Success, got.Grade)
	}
	cr = contest.Result{Contested: true, Success: false, Floored: true, Margin: -1}
	if got := gradeFrom(cr, 300, 100, Inputs{}); got.Success {
		t.Error("a floored loss yields nothing")
	}
}

// The whole point of the gather roll: stats and tool outweigh skill.
func TestScore_StatsAndToolOutweighSkill(t *testing.T) {
	base := Inputs{StatAvg: 100, UsesTool: true, ToolTier: items.ToolTierIron, SightMult: 1}
	steel := base
	steel.ToolTier = items.ToolTierSteel
	skilled := base
	skilled.SkillLevel = 5

	if Score(steel)-Score(base) <= Score(skilled)-Score(base) {
		t.Errorf("a steel tool (+%.1f) should beat five skill ranks (+%.1f)",
			Score(steel)-Score(base), Score(skilled)-Score(base))
	}
	strong := base
	strong.StatAvg = 130
	if Score(strong)-Score(base) <= Score(skilled)-Score(base) {
		t.Error("thirty stat points should beat five skill ranks")
	}
	dark := base
	dark.SightMult = 0.5
	if Score(dark) >= Score(base) {
		t.Error("poor sight lowers the score")
	}
}

func TestRounds_ToolSpeed(t *testing.T) {
	fast := Tool{Speed: 2}
	if got := Rounds(6, fast, true); got != 3 {
		t.Errorf("speed 2 on 6 rounds = %d, want 3", got)
	}
	if got := Rounds(6, fast, false); got != 6 {
		t.Errorf("no tool = %d, want 6", got)
	}
	if got := Rounds(1, Tool{Speed: 10}, true); got != 1 {
		t.Errorf("never below 1, got %d", got)
	}
}

func seedTools(t *testing.T) {
	t.Helper()
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Iron Skinning Knife", Type: items.Object, Tool: &items.ToolSpec{Type: items.ToolKnife, Tier: items.ToolTierIron, Speed: 1}},
		2: {ItemId: 2, Name: "Steel Skinning Knife", Type: items.Object, Tool: &items.ToolSpec{Type: items.ToolKnife, Tier: items.ToolTierSteel, Speed: 1}},
		3: {ItemId: 3, Name: "Dagger", Type: items.Weapon, Subtype: items.Stabbing, Hands: items.OneHanded},
		4: {ItemId: 4, Name: "Greatsword", Type: items.Weapon, Subtype: items.Slashing, Hands: items.TwoHanded},
		5: {ItemId: 5, Name: "Hatchet", Type: items.Weapon, Subtype: items.Cleaving, Hands: items.OneHanded},
		6: {ItemId: 6, Name: "Woodcutter's Axe", Type: items.Weapon, Subtype: items.Cleaving, Hands: items.OneHanded, Tool: &items.ToolSpec{Type: items.ToolAxe, Tier: items.ToolTierIron, Speed: 1}},
	})
	t.Cleanup(cleanup)
}

func TestBestTool_PicksHighestTier(t *testing.T) {
	seedTools(t)
	got, ok := bestToolFrom([]items.Item{{ItemId: 3}, {ItemId: 1}, {ItemId: 2}}, items.ToolKnife)
	if !ok || got.Item.ItemId != 2 || got.Tier != items.ToolTierSteel {
		t.Errorf("want steel knife, got %+v ok=%v", got, ok)
	}
}

func TestBestTool_ImprovisedAndRefused(t *testing.T) {
	seedTools(t)
	got, ok := bestToolFrom([]items.Item{{ItemId: 3}}, items.ToolKnife)
	if !ok || !got.Improvised || got.Tier != items.ToolTierCrude {
		t.Errorf("a dagger is a crude improvised knife, got %+v ok=%v", got, ok)
	}
	if _, ok := bestToolFrom([]items.Item{{ItemId: 4}}, items.ToolKnife); ok {
		t.Error("a two-handed greatsword is not a knife")
	}
	if got, ok := bestToolFrom([]items.Item{{ItemId: 5}}, items.ToolCleaver); !ok || !got.Improvised {
		t.Error("a hatchet is an improvised cleaver")
	}
	// A real tool is never also improvised as something else.
	if _, ok := bestToolFrom([]items.Item{{ItemId: 6}}, items.ToolCleaver); ok {
		t.Error("an item with a tool spec only serves its own tool type")
	}
	if got, ok := bestToolFrom([]items.Item{{ItemId: 6}}, items.ToolAxe); !ok || got.Improvised {
		t.Error("the woodcutter's axe is a real axe")
	}
}

func TestBestTool_InstanceGradeNudgesTier(t *testing.T) {
	seedTools(t)
	got, _ := bestToolFrom([]items.Item{{ItemId: 1, Quality: items.QualityPristine}}, items.ToolKnife)
	if got.Tier != items.ToolTierSteel {
		t.Errorf("a pristine iron knife works as steel, got %v", got.Tier)
	}
	got, _ = bestToolFrom([]items.Item{{ItemId: 2, Quality: items.QualityCrude}}, items.ToolKnife)
	if got.Tier != items.ToolTierIron {
		t.Errorf("a crude steel knife works as iron, got %v", got.Tier)
	}
}

func TestRoll_RequiredToolMissing(t *testing.T) {
	seedTools(t)
	c := &characters.Character{}
	res := Roll(c, nil, JobSkin, 0)
	if !res.NoTool || res.Success {
		t.Errorf("skinning with no knife must refuse, got %+v", res)
	}
}

func TestRoll_UsesCarriedTool(t *testing.T) {
	seedTools(t)
	c := &characters.Character{Items: []items.Item{{ItemId: 2}}}
	res := Roll(c, nil, JobSkin, 0)
	if res.NoTool || !res.HasTool || res.Tool.Item.ItemId != 2 {
		t.Errorf("should skin with the carried steel knife, got %+v", res)
	}
}

func TestCraftGrade(t *testing.T) {
	won := func(s float64) *contest.Result { r := win(s); return &r }

	if g := CraftGrade(won(3), []items.Item{{ItemId: 1}}, false, items.ToolTierNone); g != items.QualityNone {
		t.Errorf("ungraded inputs and no tool leave the output ungraded, got %v", g)
	}
	if g := CraftGrade(won(3), []items.Item{{ItemId: 1, Quality: items.QualityCrude}, {ItemId: 2, Quality: items.QualityPristine}}, false, items.ToolTierNone); g != items.QualityStandard {
		t.Errorf("capped one above the worst input (crude), got %v", g)
	}
	if g := CraftGrade(won(3), []items.Item{{ItemId: 1, Quality: items.QualityPristine}}, true, items.ToolTierIron); g != items.QualityFine {
		t.Errorf("an iron tool caps at fine, got %v", g)
	}
	if g := CraftGrade(won(0.2), nil, true, items.ToolTierMasterwork); g != items.QualityStandard {
		t.Errorf("a narrow win with a tool is standard, got %v", g)
	}
	if g := CraftGrade(nil, []items.Item{{ItemId: 1, Quality: items.QualityFine}}, false, items.ToolTierNone); g != items.QualityStandard {
		t.Errorf("an instant recipe starts at standard, got %v", g)
	}
	floored := contest.Result{Contested: true, Success: true, Floored: true, Margin: 1}
	if g := CraftGrade(&floored, nil, true, items.ToolTierSteel); g != items.QualityCrude {
		t.Errorf("a floor-granted win is crude, got %v", g)
	}
}

// A forged tool is graded by the smith's margin even from ungraded ingots,
// and no tool cap applies.
func TestCraftGradeOutput_ToolsAlwaysGraded(t *testing.T) {
	won := func(s float64) *contest.Result { r := win(s); return &r }
	if g := CraftGradeOutput(won(3), []items.Item{{ItemId: 1}}, false, items.ToolTierNone, true); g != items.QualityPristine {
		t.Errorf("a three-sigma forging of a tool is pristine, got %v", g)
	}
	if g := CraftGradeOutput(nil, nil, false, items.ToolTierNone, true); g != items.QualityStandard {
		t.Errorf("an instant tool recipe comes out standard, got %v", g)
	}
}

// Tools wear with each job and break at their durability; a well-graded
// tool lasts longer; an improvised weapon never wears.
func TestWearTool(t *testing.T) {
	seedTools(t)
	knife := items.Item{ItemId: 1}
	knife.UUID = items.NewItemUUID()
	c := &characters.Character{Items: []items.Item{knife}}
	tool, ok := BestTool(c, items.ToolKnife)
	if !ok {
		t.Fatal("no knife found")
	}
	d := c.Items[0].ToolDurability()
	if d <= 0 {
		t.Fatalf("an iron knife has a durability, got %d", d)
	}
	for i := 1; i < d; i++ {
		if name := WearTool(c, tool); name != `` {
			t.Fatalf("broke after %d of %d jobs", i, d)
		}
	}
	if c.Items[0].Wear != d-1 {
		t.Fatalf("wear %d, want %d", c.Items[0].Wear, d-1)
	}
	if name := WearTool(c, tool); name == `` {
		t.Fatal("the last job should break it")
	}
	if len(c.Items) != 1 || !c.Items[0].IsBroken() {
		t.Fatalf("a broken tool stays in the pack, broken: %+v", c.Items)
	}
	if _, ok := BestTool(c, items.ToolKnife); ok {
		t.Error("a broken tool cannot be used")
	}
	c.Items[0].Repair()
	if _, ok := BestTool(c, items.ToolKnife); !ok {
		t.Error("a repaired tool works again")
	}

	pristine := items.Item{ItemId: 1, Quality: items.QualityPristine}
	if pristine.ToolDurability() != 2*d {
		t.Errorf("a pristine knife lasts twice as long: %d vs %d", pristine.ToolDurability(), d)
	}

	dagger := items.Item{ItemId: 3}
	dagger.UUID = items.NewItemUUID()
	c = &characters.Character{Items: []items.Item{dagger}}
	tool, _ = BestTool(c, items.ToolKnife)
	for i := 0; i < 500; i++ {
		WearTool(c, tool)
	}
	if len(c.Items) != 1 || c.Items[0].Wear != 0 {
		t.Errorf("an improvised dagger never wears as a tool, got %+v", c.Items)
	}
}
