package conditions

import (
	"sort"
	"testing"

	"gopkg.in/yaml.v2"
)

// floatDefenseMultWant is 1.12 * 0.85 computed the way the runtime does it:
// as two already-float64-rounded operands multiplied at runtime. The literal
// expression `1.12 * 0.85` is instead folded by the compiler at arbitrary
// constant precision and rounded to float64 once at the end, which lands one
// ULP away (0.9519999999999999572... vs 0.9520000000000000728...) — a classic
// double-rounding mismatch, not a bug in Conditions.Effect. Computed through a
// variable so Go cannot constant-fold it.
func floatDefenseMultWant() float64 {
	a := 1.12
	b := 0.85
	return a * b
}

func withSpecs(t *testing.T, specs ...*ConditionSpec) {
	t.Helper()
	m := map[int]*ConditionSpec{}
	for _, s := range specs {
		m[s.ConditionId] = s
	}
	t.Cleanup(SeedConditionsForTest(m))
}

func TestEffectValueParsesANumberOrTheWordMagnitude(t *testing.T) {
	var s ConditionSpec
	err := yaml.Unmarshal([]byte("conditionid: 900\nname: Probe\neffects:\n  damage_mult: magnitude\n  defense_mult: 0.85\n"), &s)
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Effects[EffectDamageMult]; !v.UsesMagnitude {
		t.Fatalf("damage_mult should use the magnitude, got %+v", v)
	}
	if v := s.Effects[EffectDefenseMult]; v.UsesMagnitude || v.Literal != 0.85 {
		t.Fatalf("defense_mult should be the literal 0.85, got %+v", v)
	}
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back ConditionSpec
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Effects[EffectDamageMult] != s.Effects[EffectDamageMult] || back.Effects[EffectDefenseMult] != s.Effects[EffectDefenseMult] {
		t.Fatalf("effects must round-trip through yaml, got %+v", back.Effects)
	}
}

func TestValidateRefusesAnUnknownEffectKey(t *testing.T) {
	var s ConditionSpec
	if err := yaml.Unmarshal([]byte("conditionid: 901\nname: Probe\neffects:\n  damage_multt: 1\n"), &s); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("an unknown effect key must be refused at load")
	}
}

func TestValidateRefusesTickFromMagnitudeWithoutAPool(t *testing.T) {
	s := &ConditionSpec{ConditionId: 902, Name: "Probe", TickFromMagnitude: true}
	if err := s.Validate(); err == nil {
		t.Fatal("tick_from_magnitude needs tick_pool")
	}
	s2 := &ConditionSpec{ConditionId: 903, Name: "Probe", TickFromMagnitude: true, TickPool: "health", TickPercent: -0.1, TriggerRate: "1 round", TriggerCount: 3}
	if err := s2.Validate(); err == nil {
		t.Fatal("tick_from_magnitude and tick_percent cannot both be set")
	}
}

func TestEffectMultipliesFlatsSumCapsTakeTheMinimum(t *testing.T) {
	withSpecs(t,
		&ConditionSpec{ConditionId: 910, Name: "Shout", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}, EffectDefenseMult: {UsesMagnitude: true}}},
		&ConditionSpec{ConditionId: 911, Name: "Exposed", TriggerRate: "1 round", TriggerCount: 1, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}, EffectAttacksCap: {Literal: 1}}},
		&ConditionSpec{ConditionId: 912, Name: "Ward", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}},
		&ConditionSpec{ConditionId: 913, Name: "Ward2", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}, EffectAttacksCap: {Literal: 3}}},
	)
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(910, 5, 1.12)
	bs.AddConditionMagnitude(911, 1, 0)
	bs.AddConditionMagnitude(912, 5, 12)
	bs.AddConditionMagnitude(913, 5, 5)

	if got := bs.Effect(EffectDamageMult); got != 1.12 {
		t.Fatalf("damage mult: got %v", got)
	}
	if got := bs.Effect(EffectDefenseMult); got != floatDefenseMultWant() {
		t.Fatalf("defense mult must multiply across records: got %v, want %v", got, floatDefenseMultWant())
	}
	if got := bs.Effect(EffectMitigationFlat); got != 17 {
		t.Fatalf("flats must sum: got %v", got)
	}
	if got := bs.Effect(EffectAttacksCap); got != 1 {
		t.Fatalf("caps take the minimum: got %v", got)
	}
	if got := bs.Effect(EffectRegenMult); got != 1 {
		t.Fatalf("an absent multiplier reads 1: got %v", got)
	}
	if got := bs.Effect(EffectPoolMaxPct); got != 0 {
		t.Fatalf("an absent flat reads 0: got %v", got)
	}
	if !bs.HasEffect(EffectMitigationFlat) || bs.HasEffect(EffectRegenMult) {
		t.Fatal("HasEffect must report declared kinds only")
	}
}

func TestMagnitudeZeroOnAMultiplierContributesNothing(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 914, Name: "Hollow", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(914, 5, 0)
	if got := bs.Effect(EffectDamageMult); got != 1 {
		t.Fatalf("a zero magnitude must not zero the product: got %v", got)
	}
}

func TestExpiredRecordsDoNotContribute(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 915, Name: "Brief", TriggerRate: "1 round", TriggerCount: 1, RoundInterval: 1, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}})
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(915, 1, 1)
	if got := bs.Effect(EffectAttacksCap); got != 1 {
		t.Fatalf("held: %v", got)
	}
	bs.Trigger()
	if got := bs.Effect(EffectAttacksCap); got != 0 {
		t.Fatalf("expired by its own trigger, the cap must be gone: %v", got)
	}
}

func TestAddConditionMagnitudeSetsTheSnapshotForATickRecord(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 916, Name: "Venomed", TriggerRate: "1 round", TriggerCount: 4, TickPool: "health", TickFromMagnitude: true})
	bs := Conditions{}
	bs.Validate(true)
	if !bs.AddConditionMagnitude(916, 6, -5) {
		t.Fatal("add refused")
	}
	b := bs.GetConditions(916)[0]
	if b.Magnitude != -5 || b.TickAmount != -5 || b.TriggersLeft != 6 {
		t.Fatalf("magnitude -5 for 6 rounds must become tick snapshot -5 with 6 triggers, got %+v", *b)
	}
	bs.AddConditionMagnitude(916, 3, -9)
	b = bs.GetConditions(916)[0]
	if b.Magnitude != -9 || b.TickAmount != -9 || b.TriggersLeft != 3 {
		t.Fatalf("a re-add overwrites magnitude, snapshot and duration: %+v", *b)
	}
}

// A tick_from_magnitude record's TickAmount must never land on zero while its
// Magnitude is non-zero: the round tick's fallback recomputes from
// TickPercent, which validateEffects forces to 0 on a tick_from_magnitude
// record, so a zero snapshot would tick for nothing forever. int() truncates
// toward zero, same as the old poison/bleed hook, so a magnitude that
// truncates to zero floors to 1 in its own sign instead.
func TestAddConditionMagnitudeTickSnapshotFloorsToOneInItsSign(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 922, Name: "Trickle", TriggerRate: "1 round", TriggerCount: 4, TickPool: "health", TickFromMagnitude: true})
	bs := Conditions{}
	bs.Validate(true)

	cases := []struct {
		magnitude float64
		wantTick  int
	}{
		{-0.5, -1},
		{0.5, 1},
		{0, 0},
		{-7.9, -7},
	}
	for _, c := range cases {
		bs.AddConditionMagnitude(922, 4, c.magnitude)
		b := bs.GetConditions(922)[0]
		if b.TickAmount != c.wantTick {
			t.Fatalf("magnitude %v: got TickAmount %d, want %d", c.magnitude, b.TickAmount, c.wantTick)
		}
	}
}

func TestAddConditionMagnitudeZeroRoundsMeansTheSpecDefault(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 920, Name: "Default", TriggerRate: "1 round", TriggerCount: 7, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(920, 0, 1.5)
	if got := bs.GetConditions(920)[0].TriggersLeft; got != 7 {
		t.Fatalf("rounds 0 must take the spec's triggercount, got %d", got)
	}
}

// Exact triggers (rounds for a one-round interval), not a multiplier:
// AddConditionScaled truncates float64(count) *
// mult, and 3.3 * 10 is 32.999... in binary, which would have shortened a
// 33-round ward to 32. Every former condition passes the integer it computed.
func TestAddConditionMagnitudeRoundsAreExact(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 921, Name: "Exact", TriggerRate: "1 round", TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}})
	bs := Conditions{}
	bs.Validate(true)
	for _, rounds := range []int{1, 3, 33, 37, 250} {
		bs.AddConditionMagnitude(921, rounds, 1)
		if got := bs.GetConditions(921)[0].TriggersLeft; got != rounds {
			t.Fatalf("rounds %d became %d", rounds, got)
		}
	}
}

// The deleted internal/characters/conditions_immunity_test.go (Task 8) also
// pinned two control arms alongside the refusal: a non-poison record still
// lands on the immune holder (every other condition is unaffected), and
// without immunity the poison record lands (immunity is the only thing
// refusing it). Both are reproduced here on the record's replacement,
// AddConditionMagnitude, so poison immunity is not narrowed to "the poison record
// is always refused."
func TestAddConditionMagnitudeRefusesPoisonUnderImmunity(t *testing.T) {
	withSpecs(t,
		&ConditionSpec{ConditionId: 917, Name: "Stone", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{PoisonImmunity}},
		&ConditionSpec{ConditionId: 918, Name: "Toxin", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{Poison}, TickPool: "health", TickFromMagnitude: true},
		&ConditionSpec{ConditionId: ConditionIdBleeding, Name: "Bleeding", TriggerRate: "3 rounds", TriggerCount: 5, Flags: []Flag{Bleeding}, TickPool: "health", TickFromMagnitude: true},
	)
	bs := Conditions{}
	bs.Validate(true)
	bs.AddCondition(917, false)
	if bs.AddConditionMagnitude(918, 5, -3) {
		t.Fatal("a poison record must be refused under poison immunity")
	}

	// Control: every other record still lands while immune to poison only.
	if !bs.AddConditionMagnitude(ConditionIdBleeding, 5, -3) {
		t.Fatal("a non-poison record must still land under poison immunity")
	}

	// Control: without immunity the poison record lands.
	unprotected := Conditions{}
	unprotected.Validate(true)
	if !unprotected.AddConditionMagnitude(918, 5, -3) {
		t.Fatal("without immunity the poison record must land")
	}
}

func TestQuietSilencesBothNotices(t *testing.T) {
	s := &ConditionSpec{ConditionId: 919, Name: "Off Balance", Flags: []Flag{Quiet}, StartUserText: "x", EndUserText: "y"}
	if s.StartUserNotice() != "" || s.EndUserNotice() != "" {
		t.Fatal("a quiet record sends no line at either end")
	}
}

// SeedConditionRecordsForTest is additive: a package fixture that already
// seeded its own conditions (the hooks fixture replaces the whole map) keeps
// them, and the nine condition ids land on top. The cleanup must remove
// exactly the nine ids it added and restore whatever was there before.
func TestSeedConditionRecordsForTestIsAdditiveAndReversible(t *testing.T) {
	pre := &ConditionSpec{ConditionId: ConditionIdWarcry, Name: "Pre-existing"}
	restoreBase := SeedConditionsForTest(map[int]*ConditionSpec{ConditionIdWarcry: pre})
	defer restoreBase()

	restore := SeedConditionRecordsForTest()

	ids := []int{
		ConditionIdWarcry, ConditionIdRally, ConditionIdOffBalance, ConditionIdRecovering,
		ConditionIdConvictionWard, ConditionIdRegenerating, ConditionIdPoisoned, ConditionIdBleeding,
		ConditionIdEnchantWithdrawal,
	}
	for _, id := range ids {
		if GetConditionSpec(id) == nil {
			t.Fatalf("condition id %d did not resolve after seeding", id)
		}
	}
	if GetConditionSpec(ConditionIdWarcry).Name != "Warcry" {
		t.Fatalf("seeding must overwrite id %d with the condition record, got %+v", ConditionIdWarcry, GetConditionSpec(ConditionIdWarcry))
	}

	restore()

	for _, id := range ids {
		if id == ConditionIdWarcry {
			continue
		}
		if GetConditionSpec(id) != nil {
			t.Fatalf("cleanup must remove condition id %d, still present: %+v", id, GetConditionSpec(id))
		}
	}
	if got := GetConditionSpec(ConditionIdWarcry); got != pre {
		t.Fatalf("cleanup must restore the pre-existing spec at id %d, got %+v", ConditionIdWarcry, got)
	}
}

// A kind classified under more than one of Effect's aggregation predicates
// would double-count in its switch (the first matching case always wins),
// silently dropping the kind from whichever mode its other predicate would
// have produced. Counting how many predicates match, rather than checking
// pairs by hand, is what makes this test's name true: it covers every
// predicate Effect dispatches on, including one added after this test was
// written, without needing a new pairwise check bolted on each time.
func TestEveryEffectKindIsClassifiedExactlyOnce(t *testing.T) {
	for _, k := range AllEffectKinds {
		matches := 0
		if k.isMultiplier() {
			matches++
		}
		if k.isCap() {
			matches++
		}
		if k.isMax() {
			matches++
		}
		if matches > 1 {
			t.Fatalf("effect kind %q matches %d aggregation predicates, want at most 1", k, matches)
		}
	}
}

// TestEffectMaxKindTakesStrongestNotSum pins the aggregation mode the vision
// window needs. Two nightvision sources must not stack into a wider window
// than the better one grants, which is what the default summing behaviour
// would do.
func TestEffectMaxKindTakesStrongestNotSum(t *testing.T) {
	if !EffectNightVisionStrength.isMax() {
		t.Fatalf("EffectNightVisionStrength must aggregate as MAX, or two sources would stack")
	}
	if !EffectInfraReach.isMax() {
		t.Fatalf("EffectInfraReach must aggregate as MAX")
	}
	// A summing kind must not have become a max kind by accident.
	if EffectMitigationFlat.isMax() {
		t.Fatalf("EffectMitigationFlat must keep summing")
	}
}

// TestEffectMaxKindArithmeticTakesTheLarger builds two held records declaring
// the same max kind at different values and proves Effect returns the
// larger, not the sum. TestEffectMaxKindTakesStrongestNotSum above only pins
// the mode flag; this pins the arithmetic that flag is supposed to select.
func TestEffectMaxKindArithmeticTakesTheLarger(t *testing.T) {
	withSpecs(t,
		&ConditionSpec{ConditionId: 923, Name: "Weak Nightsight", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectNightVisionStrength: {Literal: 10}}},
		&ConditionSpec{ConditionId: 924, Name: "Strong Nightsight", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectNightVisionStrength: {UsesMagnitude: true}}},
	)
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(923, 5, 0)
	bs.AddConditionMagnitude(924, 5, 18)

	if got := bs.Effect(EffectNightVisionStrength); got != 18 {
		t.Fatalf("max kind must take the larger held value 18, not the sum 28: got %v", got)
	}
}

func TestEffectValuesListsEveryHeldValue(t *testing.T) {
	withSpecs(t,
		&ConditionSpec{ConditionId: 960, Name: "Heat A", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectInfraReach: {UsesMagnitude: true}}},
		&ConditionSpec{ConditionId: 961, Name: "Heat B", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectInfraReach: {Literal: 30}}},
		&ConditionSpec{ConditionId: 962, Name: "Other", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectNightVisionStrength: {Literal: 12}}},
	)
	bs := Conditions{}
	bs.Validate(true)
	bs.AddConditionMagnitude(960, 5, 22)
	bs.AddConditionMagnitude(961, 5, 0)
	bs.AddConditionMagnitude(962, 5, 0)
	got := bs.EffectValues(EffectInfraReach)
	sort.Float64s(got)
	if len(got) != 2 || got[0] != 22 || got[1] != 30 {
		t.Fatalf("EffectValues = %v, want [22 30]", got)
	}
	if bs.Effect(EffectInfraReach) != 30 {
		t.Fatal("Effect must still read the max for a max kind")
	}
}

func TestScaledKind(t *testing.T) {
	cases := []struct {
		name   string
		fx     map[EffectKind]EffectValue
		want   EffectKind
		wantOk bool
	}{
		{"glow", map[EffectKind]EffectValue{EffectLightStrength: {UsesMagnitude: true}}, EffectLightStrength, true},
		{"night sight", map[EffectKind]EffectValue{EffectNightVisionStrength: {UsesMagnitude: true}}, EffectNightVisionStrength, true},
		{"heat sight", map[EffectKind]EffectValue{EffectNightVisionStrength: {Literal: 12}, EffectInfraReach: {UsesMagnitude: true}}, EffectInfraReach, true},
		{"literal only", map[EffectKind]EffectValue{EffectInfraReach: {Literal: 30}}, "", false},
		{"combat magnitude is not a sight kind", map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}, "", false},
	}
	for _, c := range cases {
		s := &ConditionSpec{ConditionId: 970, Name: c.name, Effects: c.fx}
		got, ok := s.ScaledKind()
		if got != c.want || ok != c.wantOk {
			t.Errorf("%s: (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.wantOk)
		}
	}
}

func TestValidateRefusesTwoScaledKinds(t *testing.T) {
	s := &ConditionSpec{ConditionId: 971, Name: "Probe", TriggerRate: "1 round", TriggerCount: 1,
		Effects: map[EffectKind]EffectValue{EffectNightVisionStrength: {UsesMagnitude: true}, EffectInfraReach: {UsesMagnitude: true}}}
	if err := s.Validate(); err == nil {
		t.Fatal("a record carries one magnitude; two magnitude sight kinds must be refused at load")
	}
}
