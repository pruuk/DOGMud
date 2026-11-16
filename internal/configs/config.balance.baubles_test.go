package configs

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestBaubleDefaults(t *testing.T) {
	b := &Balance{}
	b.validateBaubles()

	if b.BaublesEnabled || b.BaubleSearchChancePct != 1 || b.BaubleSkillMaxBonus != 1 || b.BaubleRollsPerWindow != 2 || b.BaubleFeatureWindowMinutes != 60 || b.BaubleUntakenHours != 24 || b.BaubleWindowMinutes != 60 || b.BaubleWindowPerPlayer || b.BaubleRevealSeconds != 3 {
		t.Fatalf("roll defaults: %+v", b)
	}
	if b.BaubleTierWeightCheap != 70 || b.BaubleTierWeightAverage != 25 || b.BaubleTierWeightRare != 5 {
		t.Fatal("tier weight defaults")
	}
	if b.BaubleHouseholdTierWeightCheap != 45 || b.BaubleHouseholdTierWeightAverage != 40 || b.BaubleHouseholdTierWeightRare != 15 {
		t.Fatal("household tier weight defaults")
	}
	if b.BaublePickpocketTierWeightCheap != 50 || b.BaublePickpocketTierWeightAverage != 40 || b.BaublePickpocketTierWeightRare != 10 {
		t.Fatal("pickpocket tier weight defaults")
	}
	if b.BaubleStolenHeatHours != 72 || b.BaubleFenceBuyPct != 60 || b.BaubleReturnsPerCatch != 3 || b.BaubleCatalogKeepDays != 30 ||
		len(b.BaubleHeatAreas[`New Plymouth`]) != 8 ||
		!reflect.DeepEqual([]string(b.BaubleFenceGroups), []string{`fence`}) {
		t.Fatalf("stolen bauble defaults: heat %d fence %d%% %v returns %d",
			b.BaubleStolenHeatHours, b.BaubleFenceBuyPct, b.BaubleFenceGroups, b.BaubleReturnsPerCatch)
	}
	got := []ConfigInt{b.BaubleCheapMinValue, b.BaubleCheapMaxValue, b.BaubleAverageMinValue, b.BaubleAverageMaxValue, b.BaubleRareMinValue, b.BaubleRareMaxValue}
	want := []ConfigInt{1, 6, 10, 15, 40, 200}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ladder defaults: %v", got)
	}
	if !reflect.DeepEqual([]string(b.BaubleBuyerCraftSupports), []string{`general`, `jewelcrafting`}) {
		t.Fatalf("buyers: %v", b.BaubleBuyerCraftSupports)
	}
}

func TestBaubleChanceAndRevealAreCapped(t *testing.T) {
	b := &Balance{BaubleSearchChancePct: 250, BaubleRevealSeconds: 600}
	b.validateBaubles()
	if b.BaubleSearchChancePct != 100 {
		t.Fatal("a chance over 100% is 100%")
	}
	if b.BaubleRevealSeconds != 30 {
		t.Fatal("a find is never held back more than 30 seconds by the pacing knob")
	}
}

func TestBaubleOneZeroWeightIsHonoured(t *testing.T) {
	b := &Balance{BaubleTierWeightCheap: 80, BaubleTierWeightAverage: 20}
	b.validateBaubles()
	if b.BaubleTierWeightRare != 0 || b.BaubleTierWeightCheap != 80 {
		t.Fatal("an explicit zero switches a tier off")
	}
	b = &Balance{BaubleTierWeightRare: -3}
	b.validateBaubles()
	if b.BaubleTierWeightRare != 5 {
		t.Fatal("a negative weight takes its default")
	}
}

// The catalog sweep runs every BaubleSweepHours real hours: 6 by default,
// never less than 1.
func TestBaubleSweepHours(t *testing.T) {
	b := &Balance{}
	b.validateBaubles()
	if b.BaubleSweepHours != 6 {
		t.Fatalf("default %d, want 6", b.BaubleSweepHours)
	}
	b.BaubleSweepHours = -2
	b.validateBaubles()
	if b.BaubleSweepHours != 6 {
		t.Fatalf("a negative value resets to %d, want 6", b.BaubleSweepHours)
	}
	b.BaubleSweepHours = 1
	b.validateBaubles()
	if b.BaubleSweepHours != 1 {
		t.Fatalf("1 is allowed, got %d", b.BaubleSweepHours)
	}
}

// A fence never pays more than the whole value; nonsense takes the defaults.
func TestBaubleStolenSettingsAreBounded(t *testing.T) {
	b := &Balance{BaubleStolenHeatHours: -4, BaubleFenceBuyPct: 250, BaubleReturnsPerCatch: -1}
	b.validateBaubles()
	if b.BaubleStolenHeatHours != 72 || b.BaubleFenceBuyPct != 100 || b.BaubleReturnsPerCatch != 3 {
		t.Fatalf("bounded: %+v", b)
	}
	b = &Balance{BaubleReturnsPerCatch: 1}
	b.validateBaubles()
	if b.BaubleReturnsPerCatch != 2 {
		t.Fatalf("a return is always worth less than a catch: %d returns per catch", b.BaubleReturnsPerCatch)
	}
	b = &Balance{BaubleFenceBuyPct: 75, BaubleFenceGroups: ConfigSliceString{`smuggler`}}
	b.validateBaubles()
	if b.BaubleFenceBuyPct != 75 || !reflect.DeepEqual([]string(b.BaubleFenceGroups), []string{`smuggler`}) {
		t.Fatalf("a valid setting is kept: %+v", b)
	}
}

// The household and pickpocket weights follow the same rules as the search
// weights, each set on its own: an explicit zero is kept, a negative takes
// its default, and a set left out entirely takes all three defaults.
func TestBaubleStolenTierWeightsAreValidatedApart(t *testing.T) {
	b := &Balance{
		BaubleHouseholdTierWeightCheap: 60, BaubleHouseholdTierWeightAverage: 40,
		BaublePickpocketTierWeightRare: -1,
	}
	b.validateBaubles()
	if b.BaubleHouseholdTierWeightCheap != 60 || b.BaubleHouseholdTierWeightAverage != 40 || b.BaubleHouseholdTierWeightRare != 0 {
		t.Fatalf("household: an explicit zero switches rare off: %d/%d/%d",
			b.BaubleHouseholdTierWeightCheap, b.BaubleHouseholdTierWeightAverage, b.BaubleHouseholdTierWeightRare)
	}
	if b.BaublePickpocketTierWeightCheap != 0 || b.BaublePickpocketTierWeightAverage != 0 || b.BaublePickpocketTierWeightRare != 10 {
		t.Fatalf("pickpocket: a negative takes its default and the zeros stay: %d/%d/%d",
			b.BaublePickpocketTierWeightCheap, b.BaublePickpocketTierWeightAverage, b.BaublePickpocketTierWeightRare)
	}
	if b.BaubleTierWeightCheap != 70 {
		t.Fatal("the search weights are untouched by the other sets")
	}
}

// A find that must be stolen leans richer than one picked up: the shipped
// defaults give each stolen set a larger share of average and rare finds.
func TestBaubleStolenFindsLeanRicher(t *testing.T) {
	b := &Balance{}
	b.validateBaubles()
	share := func(cheap, average, rare ConfigInt) (float64, float64) {
		total := float64(cheap + average + rare)
		return float64(average) / total, float64(rare) / total
	}
	avg, rare := share(b.BaubleTierWeightCheap, b.BaubleTierWeightAverage, b.BaubleTierWeightRare)
	for name, set := range map[string][3]ConfigInt{
		`household`:  {b.BaubleHouseholdTierWeightCheap, b.BaubleHouseholdTierWeightAverage, b.BaubleHouseholdTierWeightRare},
		`pickpocket`: {b.BaublePickpocketTierWeightCheap, b.BaublePickpocketTierWeightAverage, b.BaublePickpocketTierWeightRare},
	} {
		a, r := share(set[0], set[1], set[2])
		if a <= avg || r <= rare {
			t.Errorf("%s: average %.2f and rare %.2f must both beat search's %.2f and %.2f", name, a, r, avg, rare)
		}
	}
}

func TestBaubleBrokenLadderResetsWhole(t *testing.T) {
	cases := map[string]Balance{
		"overlapping tiers": {BaubleCheapMinValue: 1, BaubleCheapMaxValue: 12, BaubleAverageMinValue: 10, BaubleAverageMaxValue: 15, BaubleRareMinValue: 40, BaubleRareMaxValue: 200},
		"inverted tier":     {BaubleCheapMinValue: 1, BaubleCheapMaxValue: 6, BaubleAverageMinValue: 15, BaubleAverageMaxValue: 10, BaubleRareMinValue: 40, BaubleRareMaxValue: 200},
		"zero minimum":      {BaubleCheapMinValue: 0, BaubleCheapMaxValue: 6, BaubleAverageMinValue: 10, BaubleAverageMaxValue: 15, BaubleRareMinValue: 40, BaubleRareMaxValue: 200},
	}
	for name, b := range cases {
		b.validateBaubles()
		if b.BaubleCheapMaxValue != 6 || b.BaubleAverageMinValue != 10 || b.BaubleRareMaxValue != 200 {
			t.Fatalf("%s: ladder not reset: %d-%d %d-%d %d-%d", name,
				b.BaubleCheapMinValue, b.BaubleCheapMaxValue, b.BaubleAverageMinValue, b.BaubleAverageMaxValue, b.BaubleRareMinValue, b.BaubleRareMaxValue)
		}
	}

	custom := Balance{BaubleCheapMinValue: 2, BaubleCheapMaxValue: 8, BaubleAverageMinValue: 12, BaubleAverageMaxValue: 20, BaubleRareMinValue: 50, BaubleRareMaxValue: 300}
	custom.validateBaubles()
	if custom.BaubleCheapMinValue != 2 || custom.BaubleRareMaxValue != 300 {
		t.Fatal("a valid custom ladder is kept")
	}
}

// The shipped config.yaml and the Go defaults must agree, or a server started
// without the block behaves differently from one started with it.
func TestBaubleShippedConfigMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	shipped := cfg.Balance
	var defaults Balance
	defaults.validateBaubles()

	if shipped.BaublesEnabled != defaults.BaublesEnabled ||
		shipped.BaubleSearchChancePct != defaults.BaubleSearchChancePct ||
		shipped.BaubleSkillMaxBonus != defaults.BaubleSkillMaxBonus ||
		shipped.BaubleFeatureWindowMinutes != defaults.BaubleFeatureWindowMinutes ||
		shipped.BaubleUntakenHours != defaults.BaubleUntakenHours ||
		!reflect.DeepEqual(shipped.BaubleBiomeChancePct, defaults.BaubleBiomeChancePct) ||
		shipped.BaubleRollsPerWindow != defaults.BaubleRollsPerWindow ||
		shipped.BaubleWindowMinutes != defaults.BaubleWindowMinutes ||
		shipped.BaubleWindowPerPlayer != defaults.BaubleWindowPerPlayer ||
		shipped.BaubleRevealSeconds != defaults.BaubleRevealSeconds ||
		shipped.BaublePickpocketChancePct != defaults.BaublePickpocketChancePct ||
		shipped.BaublePickpocketMaxWeight != defaults.BaublePickpocketMaxWeight ||
		shipped.BaublePickpocketGraceSecs != defaults.BaublePickpocketGraceSecs ||
		shipped.BaubleTierWeightCheap != defaults.BaubleTierWeightCheap ||
		shipped.BaubleTierWeightAverage != defaults.BaubleTierWeightAverage ||
		shipped.BaubleTierWeightRare != defaults.BaubleTierWeightRare ||
		shipped.BaubleHouseholdTierWeightCheap != defaults.BaubleHouseholdTierWeightCheap ||
		shipped.BaubleHouseholdTierWeightAverage != defaults.BaubleHouseholdTierWeightAverage ||
		shipped.BaubleHouseholdTierWeightRare != defaults.BaubleHouseholdTierWeightRare ||
		shipped.BaublePickpocketTierWeightCheap != defaults.BaublePickpocketTierWeightCheap ||
		shipped.BaublePickpocketTierWeightAverage != defaults.BaublePickpocketTierWeightAverage ||
		shipped.BaublePickpocketTierWeightRare != defaults.BaublePickpocketTierWeightRare ||
		shipped.BaubleStolenHeatHours != defaults.BaubleStolenHeatHours ||
		!reflect.DeepEqual(shipped.BaubleHeatAreas, defaults.BaubleHeatAreas) ||
		shipped.BaubleFenceBuyPct != defaults.BaubleFenceBuyPct ||
		!reflect.DeepEqual(shipped.BaubleFenceGroups, defaults.BaubleFenceGroups) ||
		shipped.BaubleReturnsPerCatch != defaults.BaubleReturnsPerCatch ||
		shipped.BaubleCatalogKeepDays != defaults.BaubleCatalogKeepDays ||
		shipped.BaubleSweepHours != defaults.BaubleSweepHours ||
		shipped.BaubleCheapMinValue != defaults.BaubleCheapMinValue ||
		shipped.BaubleCheapMaxValue != defaults.BaubleCheapMaxValue ||
		shipped.BaubleAverageMinValue != defaults.BaubleAverageMinValue ||
		shipped.BaubleAverageMaxValue != defaults.BaubleAverageMaxValue ||
		shipped.BaubleRareMinValue != defaults.BaubleRareMinValue ||
		shipped.BaubleRareMaxValue != defaults.BaubleRareMaxValue ||
		!reflect.DeepEqual(shipped.BaubleBuyerCraftSupports, defaults.BaubleBuyerCraftSupports) {
		t.Errorf("shipped bauble config differs from the Go defaults:\n shipped:  %+v\n defaults: %+v", shipped, defaults)
	}
}

// The owner's anchors: buildings 5%, streets about 2%, wilderness 0.25%.
func TestBaubleBiomeChanceDefaults(t *testing.T) {
	b := &Balance{}
	b.validateBaubles()
	want := map[string]float64{
		`interior`: 5, `city_thoroughfare`: 2, `city_backstreet`: 2.5,
		`forest`: 0.25, `plains`: 0.25, `mountains`: 0.25, `water`: 0, `ether`: 0,
	}
	for biome, pct := range want {
		if got, ok := b.BaubleBiomeChancePct[biome]; !ok || got != pct {
			t.Errorf("%s: got %v (listed %v), want %v", biome, got, ok, pct)
		}
	}
	// Every listed chance is a sane percentage, and built places beat the wild.
	for biome, pct := range b.BaubleBiomeChancePct {
		if pct < 0 || pct > 100 {
			t.Errorf("%s: %v is not a percentage", biome, pct)
		}
	}
	if b.BaubleBiomeChancePct[`interior`] <= b.BaubleBiomeChancePct[`city_thoroughfare`] ||
		b.BaubleBiomeChancePct[`city_thoroughfare`] <= b.BaubleBiomeChancePct[`road`] ||
		b.BaubleBiomeChancePct[`road`] <= b.BaubleBiomeChancePct[`forest`] {
		t.Error("buildings > streets > roads > wilderness")
	}
}

func TestBaubleBiomeTableAsGiven(t *testing.T) {
	b := &Balance{BaubleBiomeChancePct: map[string]float64{` Interior `: 8, `forest`: -1, `sewer`: 250}}
	b.validateBaubles()
	if len(b.BaubleBiomeChancePct) != 3 || b.BaubleBiomeChancePct[`interior`] != 8 ||
		b.BaubleBiomeChancePct[`forest`] != 0 || b.BaubleBiomeChancePct[`sewer`] != 100 {
		t.Fatalf("a given table is used as given, cleaned: %v", b.BaubleBiomeChancePct)
	}
	if _, listed := b.BaubleBiomeChancePct[`plains`]; listed {
		t.Fatal("unlisted biomes are not filled in; they use BaubleSearchChancePct")
	}
}

func TestBaubleSkillBonusBounds(t *testing.T) {
	b := &Balance{BaubleSkillMaxBonus: 50}
	b.validateBaubles()
	if b.BaubleSkillMaxBonus != 10 {
		t.Fatalf("skill bonus capped: %v", b.BaubleSkillMaxBonus)
	}
}

// The pickpocket pause ships at its defaults, and the bounds hold.
func TestStealPocketPauseShippedAndBounded(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	b := cfg.Balance
	if b.StealPocketSeconds != defaultStealPocketSeconds || b.StealPocketMinSeconds != defaultStealPocketMinSeconds ||
		b.StealPocketMaxSeconds != defaultStealPocketMaxSeconds {
		t.Fatalf("shipped pause %v/%v/%v differs from the defaults", b.StealPocketSeconds, b.StealPocketMinSeconds, b.StealPocketMaxSeconds)
	}
	var v Balance
	v.StealPocketMinSeconds, v.StealPocketMaxSeconds = 9, 4
	v.validateCombatSteal()
	if v.StealPocketSeconds != defaultStealPocketSeconds || v.StealPocketMinSeconds != 4 || v.StealPocketMaxSeconds != 4 {
		t.Fatalf("absent is the default, and min never exceeds max: %+v", []ConfigFloat{v.StealPocketSeconds, v.StealPocketMinSeconds, v.StealPocketMaxSeconds})
	}

	var c Balance
	c.BaublePickpocketChancePct = -7
	c.validateBaubles()
	if c.BaublePickpocketChancePct != -1 {
		t.Fatal("a negative chance is never")
	}
	var d Balance
	d.validateBaubles()
	if d.BaublePickpocketChancePct != 50 || d.BaublePickpocketMaxWeight != 1.0 {
		t.Fatal("absent: 50 percent, one pound")
	}
}

// A NaN from YAML `.nan` never reaches a bauble chance or weight.
func TestBaubleFloatsRefuseNaN(t *testing.T) {
	nan := ConfigFloat(math.NaN())
	b := Balance{BaubleSearchChancePct: nan, BaubleSkillMaxBonus: nan, BaublePickpocketChancePct: nan,
		BaublePickpocketMaxWeight: nan, BaublePickpocketGraceSecs: nan,
		BaubleBiomeChancePct: map[string]float64{`interior`: math.NaN()}}
	b.validateBaubles()
	b.StealPocketSeconds, b.StealPocketMinSeconds, b.StealPocketMaxSeconds = nan, nan, nan
	b.validateCombatSteal()
	for name, v := range map[string]float64{`search`: float64(b.BaubleSearchChancePct), `skill`: float64(b.BaubleSkillMaxBonus),
		`pickpocket`: float64(b.BaublePickpocketChancePct), `weight`: float64(b.BaublePickpocketMaxWeight),
		`grace`: float64(b.BaublePickpocketGraceSecs), `biome`: b.BaubleBiomeChancePct[`interior`],
		`pause`: float64(b.StealPocketSeconds), `min`: float64(b.StealPocketMinSeconds), `max`: float64(b.StealPocketMaxSeconds)} {
		if math.IsNaN(v) {
			t.Errorf("%s is NaN after validation", name)
		}
	}
}

// Every zone named in the default heat areas is a real zone of the world:
// a misspelt one would silently be an area of its own.
func TestBaubleHeatAreaZonesExist(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "rooms", "*", "zone-config.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no zone configs found: %v", err)
	}
	zones := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var zc struct {
			Name string `yaml:"name"`
		}
		if err := yaml.Unmarshal(data, &zc); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		zones[zc.Name] = true
	}
	for area, list := range defaultBaubleHeatAreas() {
		for _, z := range list {
			if !zones[z] {
				t.Errorf("heat area %q names %q, which is no zone", area, z)
			}
		}
	}
}
