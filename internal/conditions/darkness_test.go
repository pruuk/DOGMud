package conditions

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

const (
	testDarkLanternId = 9711 // literal darkness 50, adjustable: the Umbral Dark's shape
	testPallId        = 9712 // magnitude darkness, adjustable, cancellable: the pall's shape
)

func seedDarknessSpecs(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedConditionsForTest(map[int]*ConditionSpec{
		testLanternId: {ConditionId: testLanternId, Name: "Test Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectLightStrength: {Literal: 54}},
			Flags:   []Flag{Adjustable}},
		testDarkLanternId: {ConditionId: testDarkLanternId, Name: "Test Dark Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 50}},
			Flags:   []Flag{Adjustable}},
		testPallId: {ConditionId: testPallId, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []Flag{Adjustable, Cancellable}},
	}))
}

// A darkness record is never a light: LightSources skips it, DarknessSources
// finds it, and the combined walk keeps held order across both kinds.
func TestDarknessIsItsOwnKindOfSource(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	bs.AddCondition(testLanternId, true)

	if got := bs.LightSources(); len(got) != 1 || got[0].ConditionId != testLanternId {
		t.Fatalf("LightSources = %v, want only the lantern", got)
	}
	if got := bs.DarknessSources(); len(got) != 1 || got[0].ConditionId != testDarkLanternId {
		t.Fatalf("DarknessSources = %v, want only the dark lantern", got)
	}
	both := bs.LightAndDarknessSources()
	if len(both) != 2 || both[0].ConditionId != testDarkLanternId || both[1].ConditionId != testLanternId {
		t.Fatalf("LightAndDarknessSources = %v, want dark lantern then lantern (held order)", both)
	}
	if !GetConditionSpec(testDarkLanternId).IsDarknessSource() || GetConditionSpec(testDarkLanternId).IsLightSource() {
		t.Error("the dark lantern must be a darkness source and not a light source")
	}
	if !AnyDarknessSource([]int{testLanternId, testDarkLanternId}) || AnyDarknessSource([]int{testLanternId, 424242}) {
		t.Error("AnyDarknessSource must find the dark lantern and skip lights and unknown ids")
	}
}

// LightMax and LightNow read darkness_strength for a darkness record, through
// the same trim states a light uses.
func TestDarknessRecordStates(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	rec := bs.DarknessSources()[0]
	spec := GetConditionSpec(testDarkLanternId)

	if got := rec.LightMax(spec); got != 50 {
		t.Fatalf("LightMax = %v, want 50", got)
	}
	if v, ok := rec.LightNow(spec); !ok || v != 50 {
		t.Fatalf("fresh dark lantern = (%v, %v), want (50, true)", v, ok)
	}
	rec.SetLightOutput(20)
	if v, ok := rec.LightNow(spec); !ok || v != 20 {
		t.Errorf("trimmed dark lantern = (%v, %v), want (20, true)", v, ok)
	}
	rec.SetLightOutput(math.Inf(-1))
	if _, ok := rec.LightNow(spec); ok {
		t.Error("a darkness trimmed to nothing still takes light away")
	}
	rec.ResetLight()
	if v, ok := rec.LightNow(spec); !ok || v != 50 {
		t.Errorf("reset dark lantern = (%v, %v), want (50, true)", v, ok)
	}
}

// A recast pall is a fresh source at full strength, as a recast glow is.
func TestFreshMagnitudeResetsADarkness(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddConditionMagnitude(testPallId, 4, 68)
	rec := bs.DarknessSources()[0]
	rec.SetLightOutput(10)
	bs.AddConditionMagnitude(testPallId, 4, 68)
	if v, ok := rec.LightNow(GetConditionSpec(testPallId)); !ok || v != 68 {
		t.Errorf("recast pall = (%v, %v), want (68, true)", v, ok)
	}
}

func TestEffectNeverAggregatesDarkness(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	if got := bs.Effect(EffectDarknessStrength); got != 0 {
		t.Errorf("Effect(darkness_strength) = %v, want 0: darkness is per record", got)
	}
}

func TestDarknessSpecValidation(t *testing.T) {
	good := &ConditionSpec{ConditionId: 9713, Name: "Good", TriggerCount: 1, RoundInterval: 1,
		Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 50}}, Flags: []Flag{Adjustable}}
	if err := good.Validate(); err != nil {
		t.Fatalf("control: an adjustable literal darkness should validate, got %v", err)
	}
	cases := map[string]*ConditionSpec{
		"a darkness_strength of 0": {ConditionId: 9714, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 0}}},
		"both light and darkness": {ConditionId: 9715, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectLightStrength: {Literal: 40}, EffectDarknessStrength: {Literal: 40}}},
		"a stacking darkness": {ConditionId: 9716, Name: "Bad", TriggerRate: "1 round", TriggerCount: 4, RoundInterval: 1,
			Flags: []Flag{Stacking}, TickPool: "health", TickFromMagnitude: true,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 5}}},
		"adjustable with neither kind": {ConditionId: 9717, Name: "Bad", TriggerCount: 1, RoundInterval: 1, Flags: []Flag{Adjustable}},
		"two magnitude kinds": {ConditionId: 9718, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {UsesMagnitude: true}, EffectInfraReach: {UsesMagnitude: true}}},
	}
	for name, spec := range cases {
		if err := spec.Validate(); err == nil {
			t.Errorf("%s validated", name)
		}
	}
}

// The darkness spell reads its own knob trios, magnitude and duration, at the
// three reference casters (spec Rule 4).
func TestDarknessSpellScalesOnItsOwnKnobs(t *testing.T) {
	configs.SetConfigForTest(t, configs.GetConfig())
	cases := []struct {
		stat, skill  float64
		wantStrength float64
		wantTriggers int
	}{
		{100, 0, 50, 4},
		{130, 30, 68, 6},
		{175, 65, 90, 9},
	}
	for _, c := range cases {
		if got := SpellScaledMagnitude(EffectDarknessStrength, c.stat, c.skill); got != c.wantStrength {
			t.Errorf("(%v, %v): magnitude %v, want %v", c.stat, c.skill, got, c.wantStrength)
		}
		if got := SpellScaledTriggers(EffectDarknessStrength, c.stat, c.skill); got != c.wantTriggers {
			t.Errorf("(%v, %v): triggers %d, want %d", c.stat, c.skill, got, c.wantTriggers)
		}
	}

	// Move one knob of each darkness trio: darkness follows, light does not.
	cfg := configs.GetConfig()
	cfg.Balance.LightDarknessSpellStrengthBase = 10
	cfg.Balance.LightDarknessSpellDurationBase = 12
	configs.SetConfigForTest(t, cfg)
	if got := SpellScaledMagnitude(EffectDarknessStrength, 100, 0); got != 20 {
		t.Errorf("darkness base 10: magnitude %v, want 20", got)
	}
	if got := SpellScaledTriggers(EffectDarknessStrength, 100, 0); got != 14 {
		t.Errorf("darkness duration base 12: triggers %d, want 14", got)
	}
	if got := SpellScaledMagnitude(EffectLightStrength, 100, 0); got != 50 {
		t.Errorf("light moved with the darkness knob: %v, want 50", got)
	}
	if got := SpellScaledTriggers(EffectLightStrength, 100, 0); got != 4 {
		t.Errorf("light duration moved with the darkness knob: %d, want 4", got)
	}
}
