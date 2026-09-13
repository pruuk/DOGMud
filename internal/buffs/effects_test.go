package buffs

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// floatDefenseMultWant is 1.12 * 0.85 computed the way the runtime does it:
// as two already-float64-rounded operands multiplied at runtime. The literal
// expression `1.12 * 0.85` is instead folded by the compiler at arbitrary
// constant precision and rounded to float64 once at the end, which lands one
// ULP away (0.9519999999999999572... vs 0.9520000000000000728...) — a classic
// double-rounding mismatch, not a bug in Buffs.Effect. Computed through a
// variable so Go cannot constant-fold it.
func floatDefenseMultWant() float64 {
	a := 1.12
	b := 0.85
	return a * b
}

func withSpecs(t *testing.T, specs ...*BuffSpec) {
	t.Helper()
	m := map[int]*BuffSpec{}
	for _, s := range specs {
		m[s.BuffId] = s
	}
	t.Cleanup(SeedBuffsForTest(m))
}

func TestEffectValueParsesANumberOrTheWordMagnitude(t *testing.T) {
	var s BuffSpec
	err := yaml.Unmarshal([]byte("buffid: 900\nname: Probe\neffects:\n  damage_mult: magnitude\n  defense_mult: 0.85\n"), &s)
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
	var back BuffSpec
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Effects[EffectDamageMult] != s.Effects[EffectDamageMult] || back.Effects[EffectDefenseMult] != s.Effects[EffectDefenseMult] {
		t.Fatalf("effects must round-trip through yaml, got %+v", back.Effects)
	}
}

func TestValidateRefusesAnUnknownEffectKey(t *testing.T) {
	var s BuffSpec
	if err := yaml.Unmarshal([]byte("buffid: 901\nname: Probe\neffects:\n  damage_multt: 1\n"), &s); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("an unknown effect key must be refused at load")
	}
}

func TestValidateRefusesTickFromMagnitudeWithoutAPool(t *testing.T) {
	s := &BuffSpec{BuffId: 902, Name: "Probe", TickFromMagnitude: true}
	if err := s.Validate(); err == nil {
		t.Fatal("tick_from_magnitude needs tick_pool")
	}
	s2 := &BuffSpec{BuffId: 903, Name: "Probe", TickFromMagnitude: true, TickPool: "health", TickPercent: -0.1, TriggerRate: "1 round", TriggerCount: 3}
	if err := s2.Validate(); err == nil {
		t.Fatal("tick_from_magnitude and tick_percent cannot both be set")
	}
}

func TestEffectMultipliesFlatsSumCapsTakeTheMinimum(t *testing.T) {
	withSpecs(t,
		&BuffSpec{BuffId: 910, Name: "Shout", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}, EffectDefenseMult: {UsesMagnitude: true}}},
		&BuffSpec{BuffId: 911, Name: "Exposed", TriggerRate: "1 round", TriggerCount: 1, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}, EffectAttacksCap: {Literal: 1}}},
		&BuffSpec{BuffId: 912, Name: "Ward", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}},
		&BuffSpec{BuffId: 913, Name: "Ward2", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}, EffectAttacksCap: {Literal: 3}}},
	)
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(910, 5, 1.12)
	bs.AddBuffMagnitude(911, 1, 0)
	bs.AddBuffMagnitude(912, 5, 12)
	bs.AddBuffMagnitude(913, 5, 5)

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
	withSpecs(t, &BuffSpec{BuffId: 914, Name: "Hollow", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(914, 5, 0)
	if got := bs.Effect(EffectDamageMult); got != 1 {
		t.Fatalf("a zero magnitude must not zero the product: got %v", got)
	}
}

func TestExpiredRecordsDoNotContribute(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 915, Name: "Brief", TriggerRate: "1 round", TriggerCount: 1, RoundInterval: 1, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(915, 1, 1)
	if got := bs.Effect(EffectAttacksCap); got != 1 {
		t.Fatalf("held: %v", got)
	}
	bs.Trigger()
	if got := bs.Effect(EffectAttacksCap); got != 0 {
		t.Fatalf("expired by its own trigger, the cap must be gone: %v", got)
	}
}

func TestAddBuffMagnitudeSetsTheSnapshotForATickRecord(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 916, Name: "Venomed", TriggerRate: "1 round", TriggerCount: 4, TickPool: "health", TickFromMagnitude: true})
	bs := Buffs{}
	bs.Validate(true)
	if !bs.AddBuffMagnitude(916, 6, -5) {
		t.Fatal("add refused")
	}
	b := bs.GetBuffs(916)[0]
	if b.Magnitude != -5 || b.TickAmount != -5 || b.TriggersLeft != 6 {
		t.Fatalf("magnitude -5 for 6 rounds must become tick snapshot -5 with 6 triggers, got %+v", *b)
	}
	bs.AddBuffMagnitude(916, 3, -9)
	b = bs.GetBuffs(916)[0]
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
func TestAddBuffMagnitudeTickSnapshotFloorsToOneInItsSign(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 922, Name: "Trickle", TriggerRate: "1 round", TriggerCount: 4, TickPool: "health", TickFromMagnitude: true})
	bs := Buffs{}
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
		bs.AddBuffMagnitude(922, 4, c.magnitude)
		b := bs.GetBuffs(922)[0]
		if b.TickAmount != c.wantTick {
			t.Fatalf("magnitude %v: got TickAmount %d, want %d", c.magnitude, b.TickAmount, c.wantTick)
		}
	}
}

func TestAddBuffMagnitudeZeroRoundsMeansTheSpecDefault(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 920, Name: "Default", TriggerRate: "1 round", TriggerCount: 7, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(920, 0, 1.5)
	if got := bs.GetBuffs(920)[0].TriggersLeft; got != 7 {
		t.Fatalf("rounds 0 must take the spec's triggercount, got %d", got)
	}
}

// Exact rounds, not a multiplier: AddBuffScaled truncates float64(count) *
// mult, and 3.3 * 10 is 32.999... in binary, which would have shortened a
// 33-round ward to 32. Every former condition passes the integer it computed.
func TestAddBuffMagnitudeRoundsAreExact(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 921, Name: "Exact", TriggerRate: "1 round", TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	for _, rounds := range []int{1, 3, 33, 37, 250} {
		bs.AddBuffMagnitude(921, rounds, 1)
		if got := bs.GetBuffs(921)[0].TriggersLeft; got != rounds {
			t.Fatalf("rounds %d became %d", rounds, got)
		}
	}
}

// The deleted internal/characters/conditions_immunity_test.go (Task 8) also
// pinned two control arms alongside the refusal: a non-poison record still
// lands on the immune holder (every other condition is unaffected), and
// without immunity the poison record lands (immunity is the only thing
// refusing it). Both are reproduced here on the record's replacement,
// AddBuffMagnitude, so poison immunity is not narrowed to "the poison record
// is always refused."
func TestAddBuffMagnitudeRefusesPoisonUnderImmunity(t *testing.T) {
	withSpecs(t,
		&BuffSpec{BuffId: 917, Name: "Stone", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{PoisonImmunity}},
		&BuffSpec{BuffId: 918, Name: "Toxin", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{Poison}, TickPool: "health", TickFromMagnitude: true},
		&BuffSpec{BuffId: BuffIdBleeding, Name: "Bleeding", TriggerRate: "3 rounds", TriggerCount: 5, Flags: []Flag{Bleeding}, TickPool: "health", TickFromMagnitude: true},
	)
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuff(917, false)
	if bs.AddBuffMagnitude(918, 5, -3) {
		t.Fatal("a poison record must be refused under poison immunity")
	}

	// Control: every other record still lands while immune to poison only.
	if !bs.AddBuffMagnitude(BuffIdBleeding, 5, -3) {
		t.Fatal("a non-poison record must still land under poison immunity")
	}

	// Control: without immunity the poison record lands.
	unprotected := Buffs{}
	unprotected.Validate(true)
	if !unprotected.AddBuffMagnitude(918, 5, -3) {
		t.Fatal("without immunity the poison record must land")
	}
}

func TestQuietSilencesBothNotices(t *testing.T) {
	s := &BuffSpec{BuffId: 919, Name: "Off Balance", Flags: []Flag{Quiet}, StartUserText: "x", EndUserText: "y"}
	if s.StartUserNotice() != "" || s.EndUserNotice() != "" {
		t.Fatal("a quiet record sends no line at either end")
	}
}

// SeedConditionRecordsForTest is additive: a package fixture that already
// seeded its own buffs (the hooks fixture replaces the whole map) keeps
// them, and the nine condition ids land on top. The cleanup must remove
// exactly the nine ids it added and restore whatever was there before.
func TestSeedConditionRecordsForTestIsAdditiveAndReversible(t *testing.T) {
	pre := &BuffSpec{BuffId: BuffIdWarcry, Name: "Pre-existing"}
	restoreBase := SeedBuffsForTest(map[int]*BuffSpec{BuffIdWarcry: pre})
	defer restoreBase()

	restore := SeedConditionRecordsForTest()

	ids := []int{
		BuffIdWarcry, BuffIdRally, BuffIdOffBalance, BuffIdRecovering,
		BuffIdMinorShield, BuffIdRegenerating, BuffIdPoisoned, BuffIdBleeding,
		BuffIdEnchantWithdrawal,
	}
	for _, id := range ids {
		if GetBuffSpec(id) == nil {
			t.Fatalf("buff id %d did not resolve after seeding", id)
		}
	}
	if GetBuffSpec(BuffIdWarcry).Name != "Warcry" {
		t.Fatalf("seeding must overwrite id %d with the condition record, got %+v", BuffIdWarcry, GetBuffSpec(BuffIdWarcry))
	}

	restore()

	for _, id := range ids {
		if id == BuffIdWarcry {
			continue
		}
		if GetBuffSpec(id) != nil {
			t.Fatalf("cleanup must remove buff id %d, still present: %+v", id, GetBuffSpec(id))
		}
	}
	if got := GetBuffSpec(BuffIdWarcry); got != pre {
		t.Fatalf("cleanup must restore the pre-existing spec at id %d, got %+v", BuffIdWarcry, got)
	}
}

// A kind classified as both a multiplier and a cap would double-count in
// Effect's switch (multiplier branch always wins), silently dropping it from
// the cap aggregation. This keeps a future kind added to only one list from
// failing silently instead of loudly.
func TestEveryEffectKindIsClassifiedExactlyOnce(t *testing.T) {
	for _, k := range AllEffectKinds {
		if k.isMultiplier() && k.isCap() {
			t.Fatalf("effect kind %q is classified as both a multiplier and a cap", k)
		}
	}
}
