package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parityHarmAmount reads what a harmful effect did to its target: health
// lost for damage and knockdown, the dot record's trigger count for a dot
// (-1 when the record is missing).
func parityHarmAmount(spell *spells.SpellData, target *characters.Character, healthBefore int) int {
	if spell.EffectType == "dot" {
		recs := target.GetConditions(conditions.ConditionIdPoisoned)
		if len(recs) != 1 {
			return -1
		}
		return recs[0].TriggersLeft
	}
	return healthBefore - target.Health
}

// parityWantAmount is the amount every pairing must land. Damage runs the
// shared formula on this pairing's own equalised combatants. The dot's
// duration is written from the fixture's numbers (skill 3, willpower 100),
// not from the production helper, so a pairing that reads the wrong caster
// cannot agree with it by construction.
func parityWantAmount(spell *spells.SpellData, caster, target *characters.Character) int {
	if spell.EffectType == "dot" {
		return calcSpellDuration(spell.BaseFolds, 3, 100) / 3
	}
	return scaleSpellDamageByDefence(
		calcSpellDamageForCharacter(spell, caster, target, spell.EffectMagnitude, false), spellContestAttackWin())
}

// paritySamples is how many casts a damage amount is averaged over. The
// damage roll is dice.RollStat, a normal draw with a 0.15 spread around the
// mitigated mean, on the unseedable global math/rand (rand.Seed is a no-op
// since Go 1.24), so one cast cannot be compared exactly with another roll
// of the formula. Averaged over 400 casts the mean's spread is under 1% of
// the mean, and paritySlack (5%) sits beyond four standard deviations of
// the difference between two such means.
const (
	paritySamples = 400
	paritySlack   = 0.05
)

// parityMeanAmount casts spell through p paritySamples times on f, restoring
// the target's health and footing before each cast, and returns the mean
// health lost beside the mean of the shared formula on the same combatants.
func parityMeanAmount(f *spellParityFixture, p spellParityPairing, spell *spells.SpellData) (got, want float64) {
	casterChar, targetChar := p.caster(f), p.target(f)
	for range paritySamples {
		targetChar.Health = 1000
		targetChar.Position = position.NewMachine()
		want += float64(parityWantAmount(spell, casterChar, targetChar))
		p.cast(f, spell)
		got += float64(1000 - targetChar.Health)
		for _, id := range []int{1, 2, 3} {
			drainPlain(id)
		}
		events.DrainQueuedPlayerAttackedMobsForTest(0)
	}
	f.records = nil
	return got / paritySamples, want / paritySamples
}

// Parity slice 3a: each harmful effect, driven through all four pairings
// with the same caster stats and spell, lands the same amount, turns the
// target on its caster, is recorded once, is seen by a watcher, and (PM
// only) counts as aggression against the mob.
func TestSpellHarmParity_EveryPairingLandsTheSame(t *testing.T) {
	effects := []struct {
		name     string
		spell    func() *spells.SpellData
		roomWord string
	}{
		{"damage", physicalHarmSpellForCollapseTest, "strikes"},
		{"dot", dotSpellForParityTest, "afflicts"},
		{"knockdown", knockdownSpellForParityTest, "to the ground"},
	}
	for _, eff := range effects {
		t.Run(eff.name, func(t *testing.T) {
			amounts := map[string]float64{}
			for _, p := range spellParityPairings() {
				t.Run(p.name, func(t *testing.T) {
					f := newSpellParityFixture(t, spellContestAttackWin())
					spell := eff.spell()
					casterChar, targetChar := p.caster(f), p.target(f)
					want := parityWantAmount(spell, casterChar, targetChar)
					before := targetChar.Health

					p.cast(f, spell)

					got := parityHarmAmount(spell, targetChar, before)
					if eff.name == "dot" {
						// The dot's duration rolls nothing: compare exactly.
						assert.Equal(t, want, got, "the amount must match the shared formula")
						amounts[p.name] = float64(got)
					} else {
						assert.Greater(t, got, 0, "the spell must deal damage")
					}
					if eff.name == "knockdown" {
						assert.True(t, targetChar.IsSupine() || targetChar.IsProne(), "knocked down")
					}
					assert.Equal(t, p.casterRef(f), targetChar.CurrentCombatTarget(),
						"the target must turn on its caster")
					require.Len(t, f.records, 1, "one record per resolved cast")
					assert.Equal(t, p.src, f.records[0].src)
					assert.Equal(t, p.tgt, f.records[0].tgt)
					assert.True(t, f.records[0].hit)
					if eff.name != "dot" {
						assert.Equal(t, got, f.records[0].dmg)
					}
					assert.Equal(t, 1, countContaining(drainPlain(3), eff.roomWord),
						"a watcher sees the spell land")
					attacked := events.DrainQueuedPlayerAttackedMobsForTest(0)
					if p.name == "PM" {
						require.Len(t, attacked, 1, "a player's harm on a mob is aggression")
						assert.Equal(t, 101, attacked[0].MobInstanceId)
					} else {
						assert.Empty(t, attacked, "only a player caster on a mob seeds aggression")
					}
					if eff.name != "dot" {
						meanGot, meanWant := parityMeanAmount(f, p, spell)
						t.Logf("mean landed %.1f, mean formula %.1f over %d casts", meanGot, meanWant, paritySamples)
						assert.InDelta(t, meanWant, meanGot, paritySlack*meanWant,
							"the mean amount must match the shared formula")
						amounts[p.name] = meanGot
					}
				})
			}
			require.Len(t, amounts, 4)
			for name, amount := range amounts {
				assert.InDelta(t, amounts["PM"], amount, paritySlack*amounts["PM"],
					"%s must land what PM lands", name)
			}
		})
	}
}
