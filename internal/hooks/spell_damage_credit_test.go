package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spellCreditEffects are the harmful effects that land damage on the spot.
// The dot is absent: it credits per tick (condition_tick_credit_test.go).
func spellCreditEffects() []struct {
	name  string
	spell func() *spells.SpellData
} {
	return []struct {
		name  string
		spell func() *spells.SpellData
	}{
		{"damage", physicalHarmSpellForCollapseTest},
		{"knockdown", knockdownSpellForParityTest},
	}
}

// A player's harmful spell on a mob credits the caster with the damage it
// landed, exactly as melee does (combat.go AttackPlayerVsMob). Without it a
// spell kill carries no damaging players, and every MobDeath hook that reads
// them (murder, faction rep, bounty, quest kill credit) skips the kill.
func TestSpellDamage_PlayerCasterIsCreditedOnTheMob(t *testing.T) {
	for _, eff := range spellCreditEffects() {
		t.Run(eff.name, func(t *testing.T) {
			f := newSpellParityFixture(t, spellContestAttackWin())
			spell := eff.spell()
			before := f.targetMob.Character.Health

			resolveAgainstMob(f.casterUser, f.targetMob, f.room, spell,
				spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)

			landed := before - f.targetMob.Character.Health
			require.Greater(t, landed, 0, "the spell must deal damage")
			assert.Equal(t, map[int]int{f.casterUser.UserId: landed}, f.targetMob.Character.PlayerDamage,
				"the caster is credited with exactly what landed")
			events.DrainQueuedPlayerAttackedMobsForTest(0)
		})
	}
}

// A charmed mob's harmful spell credits the player who charmed it, mirroring
// melee's charmed-owner attribution (combat.go AttackMobVsMob). An uncharmed
// mob caster credits no one.
func TestSpellDamage_CharmedMobCasterCreditsItsOwner(t *testing.T) {
	for _, eff := range spellCreditEffects() {
		t.Run(eff.name, func(t *testing.T) {
			for _, charmed := range []bool{true, false} {
				f := newSpellParityFixture(t, spellContestAttackWin())
				if charmed {
					f.casterMob.Character.Charmed = characters.NewCharm(f.casterUser.UserId, -1, "")
				} else {
					f.casterMob.Character.Charmed = nil
				}
				spell := eff.spell()
				before := f.targetMob.Character.Health

				resolveMobSpellAgainstMob(f.casterMob, f.targetMob, f.room, spell,
					spellAttackSideFor(spell, &f.casterMob.Character, nil), spell.EffectMagnitude)

				landed := before - f.targetMob.Character.Health
				require.Greater(t, landed, 0, "the spell must deal damage")
				if charmed {
					assert.Equal(t, map[int]int{f.casterUser.UserId: landed}, f.targetMob.Character.PlayerDamage,
						"the charmer is credited with what its mob landed")
				} else {
					assert.Empty(t, f.targetMob.Character.PlayerDamage, "a free mob credits no player")
				}
				f.casterMob.Character.Charmed = nil
			}
		})
	}
}

// A lethal player spell produces a MobDeath whose damaging players include
// the caster: the list every MobDeath_* hook reads.
func TestSpellDamage_LethalPlayerSpellDeathCarriesTheCaster(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	ghoul := &f.targetMob.Character
	ghoul.Life = life.NewMachine()
	wireAlivenessSubstrate(ghoul)
	ghoul.Health = 1
	events.DrainQueuedCharacterDiedForTest()
	events.DrainQueuedMobDeathsForTest()

	spell := physicalHarmSpellForCollapseTest()
	resolveAgainstMob(f.casterUser, f.targetMob, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1, "the lethal spell queues one attributed death")
	assert.Equal(t, f.casterUser.UserId, died[0].KillerUserId)
	RouteAttributedDeath(died[0])

	deaths := events.DrainQueuedMobDeathsForTest()
	require.Len(t, deaths, 1, "the death reaches the MobDeath hooks")
	assert.Contains(t, deaths[0].PlayerDamage, f.casterUser.UserId,
		"the caster is among the damaging players")
	assert.Greater(t, deaths[0].PlayerDamage[f.casterUser.UserId], 0)
	events.DrainQueuedPlayerAttackedMobsForTest(0)
}
