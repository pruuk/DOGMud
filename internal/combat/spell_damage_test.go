package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

func spellFixtureCaster(t *testing.T) *characters.Character {
	t.Helper()
	c := parityCombatant(t, "caster")
	c.Stats.Willpower.Base = 120
	c.Stats.Willpower.Recalculate()
	c.Skills[string(skills.Spellcasting)] = 20
	c.ConvictionMax.Value = 1000
	c.Conviction = 1000
	return c
}

// SpellRawDamage is the pipeline mean rollSpellDamage rolls around (#301):
// with no mutations, no caster weapon and a full pool it is CalcRawDamage of
// the spell's primary stat and spellcasting rank.
func TestSpellRawDamage_IsThePipelineMean(t *testing.T) {
	pinParityBalance(t)
	caster := spellFixtureCaster(t)
	sd := &spells.SpellData{SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental}

	raw, rank, ok := SpellRawDamage(sd, caster)
	if !ok {
		t.Fatal("a damaging spell must report ok")
	}
	want := CalcRawDamage(caster.Stats.Willpower.ValueAdj, 20, 1.5, ChannelMagical)
	if rank != 20 || math.Abs(raw-want) > 1e-9 {
		t.Fatalf("SpellRawDamage = %v (rank %d), want %v (rank 20)", raw, rank, want)
	}
}

func TestSpellRawDamage_NotOkWithoutAMultiplier(t *testing.T) {
	pinParityBalance(t)
	if _, _, ok := SpellRawDamage(&spells.SpellData{SpellId: "legacy"}, spellFixtureCaster(t)); ok {
		t.Fatal("a spell with no damage_multiplier takes the legacy path and must not be ok")
	}
}

func TestSpellMitigation_ReadsTheDamageTypesChannel(t *testing.T) {
	pinParityBalance(t)
	target := parityCombatant(t, "target")
	sd := &spells.SpellData{SpellId: "test-bolt", DamageType: combatvocab.DamageMental}
	pct, curve, channel, ok := SpellMitigation(sd, target)
	if !ok || channel != "magical" || pct != target.GetMagicalMitigation() || curve != MitigationCurveFor(ChannelMagical) {
		t.Fatalf("SpellMitigation = %v %v %q %v, want the magical channel", pct, curve, channel, ok)
	}
}

// The conviction penalty is one of the factors a refactor could drop quietly;
// a depleted pool must cost the caster damage by exactly ResourceMultiplier.
func TestSpellRawDamage_ConvictionPenaltyApplies(t *testing.T) {
	pinParityBalance(t)
	sd := &spells.SpellData{SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental}

	full, _, _ := SpellRawDamage(sd, spellFixtureCaster(t))

	caster := spellFixtureCaster(t)
	caster.Conviction = caster.ConvictionMax.Value / 4
	got, _, ok := SpellRawDamage(sd, caster)
	if !ok {
		t.Fatal("a damaging spell must report ok")
	}
	base := CalcRawDamage(caster.Stats.Willpower.ValueAdj, 20, 1.5, ChannelMagical)
	want := base * ResourceMultiplier(caster.Conviction,
		caster.EffectivePoolMax(characters.PoolConviction), float64(configs.GetBalanceConfig().ConvictionPenaltyMax))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("raw = %v, want %v", got, want)
	}
	if !(got < full) {
		t.Fatalf("a quarter pool (%v) must hit softer than a full one (%v)", got, full)
	}
}

// The crash-site Dampened flag scales the mean by CrashSiteSuppressionFactor,
// floored at 1.
func TestSpellRawDamage_DampenedApplies(t *testing.T) {
	pinParityBalance(t)
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		200: {
			ConditionId:  200,
			Name:         "Dampened",
			TriggerCount: 1000000,
			Flags:        []conditions.Flag{conditions.Dampened},
		},
	})
	defer cleanup()
	sd := &spells.SpellData{SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental}

	plain, _, _ := SpellRawDamage(sd, spellFixtureCaster(t))

	caster := spellFixtureCaster(t)
	caster.Conditions = conditions.New()
	if err := caster.AddCondition(200, true); err != nil {
		t.Fatalf("AddCondition(Dampened): %v", err)
	}
	if !caster.HasConditionFlag(conditions.Dampened) {
		t.Fatal("caster should carry the Dampened flag")
	}
	got, _, _ := SpellRawDamage(sd, caster)
	want := math.Max(1, plain*float64(configs.GetBalanceConfig().CrashSiteSuppressionFactor))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("dampened raw = %v, want %v (plain %v)", got, want, plain)
	}
	if !(got < plain) {
		t.Fatalf("dampened (%v) must be below plain (%v)", got, plain)
	}
}

func TestSpellMitigation_EveryChannel(t *testing.T) {
	pinParityBalance(t)
	target := parityCombatant(t, "target")
	cases := []struct {
		name    string
		dt      combatvocab.DamageType
		pct     float64
		ch      DamageChannel
		channel string
		ok      bool
	}{
		{"physical", combatvocab.DamagePhysical, target.GetPhysicalMitigation(), ChannelPhysical, "physical", true},
		{"magical", combatvocab.DamageMental, target.GetMagicalMitigation(), ChannelMagical, "magical", true},
		{"conviction", combatvocab.DamageSocial, target.GetConvictionMitigation(), ChannelConviction, "conviction", true},
		{"non-harm", combatvocab.DamageNonHarm, 0, ChannelPhysical, "none", false},
	}
	for _, c := range cases {
		pct, curve, channel, ok := SpellMitigation(&spells.SpellData{SpellId: "t", DamageType: c.dt}, target)
		if pct != c.pct || curve != MitigationCurveFor(c.ch) || channel != c.channel || ok != c.ok {
			t.Errorf("%s: got %v %v %q %v, want %v %v %q %v", c.name, pct, curve, channel, ok,
				c.pct, MitigationCurveFor(c.ch), c.channel, c.ok)
		}
	}
}
