package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
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
