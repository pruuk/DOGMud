package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parityQueuedConditions drains the Condition events queued for p's target.
func parityQueuedConditions(f *spellParityFixture, p spellParityPairing) []events.Condition {
	switch p.target(f) {
	case f.targetUser.Character:
		return events.DrainQueuedConditionsForTest(f.targetUser.UserId)
	case &f.targetMob.Character:
		return events.DrainQueuedMobConditionsForTest(f.targetMob.InstanceId)
	}
	return events.DrainQueuedMobConditionsForTest(f.casterMob.InstanceId)
}

// Parity slice 3b: each helpful effect, driven through all five pairings
// with the same caster stats and spell, lands the same amount on its target
// with no contest, is recorded once as a landed cast, and is seen by a
// watcher. Heal and shield roll nothing, so their amounts compare exactly,
// written from the fixture's numbers (parityHealRounds, parityShieldBonus).
func TestSpellHelpParity_EveryPairingLandsTheSame(t *testing.T) {
	effects := []struct {
		name               string
		spell              func() *spells.SpellData
		roomWord, selfWord string
		prepare            func(t *testing.T, target *characters.Character)
		check              func(t *testing.T, f *spellParityFixture, p spellParityPairing)
	}{
		{name: "heal", spell: healSpellForParityTest,
			roomWord: "in healing light", selfWord: "channels restorative magic",
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
				rec := regenRecord(t, p.target(f))
				assert.Equal(t, 3.0, rec.Magnitude)
				assert.Equal(t, parityHealRounds(), rec.TriggersLeft)
			}},
		{name: "shield", spell: shieldSpellForParityTest,
			roomWord: "ward of conviction shimmers around", selfWord: "ward of conviction shimmers around",
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
				rec := shieldRecord(t, p.target(f))
				assert.Equal(t, parityShieldBonus(), rec.Magnitude)
				assert.Equal(t, calcSpellDuration(4, 3, 100), rec.TriggersLeft)
			}},
		{name: "condition", spell: conditionSpellForParityTest,
			roomWord: "settles over", selfWord: "settles over",
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
				queued := parityQueuedConditions(f, p)
				require.Len(t, queued, 1, "the condition is queued once for the target")
				assert.Equal(t, 100, queued[0].ConditionId)
			}},
		{name: "purge", spell: purgeSpellForParityTest,
			roomWord: "cleanses", selfWord: "cleanses",
			prepare: func(t *testing.T, target *characters.Character) { poisonForPurgeTest(t, target) },
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
				assert.False(t, poisoned(p.target(f)), "the target is cleansed")
			}},
	}
	for _, eff := range effects {
		t.Run(eff.name, func(t *testing.T) {
			for _, p := range spellHelpParityPairings() {
				t.Run(p.name, func(t *testing.T) {
					f := newSpellParityFixture(t, spellContestAttackWin())
					contests := countSpellContests(t)
					if eff.prepare != nil {
						eff.prepare(t, p.target(f))
					}
					events.DrainQueuedMobConditionsForTest(0) // zero drains player ones too

					p.cast(f, eff.spell())

					assert.Zero(t, *contests, "a help spell runs no contest")
					eff.check(t, f, p)
					require.Len(t, f.records, 1, "one record per resolved cast")
					assert.Equal(t, p.src, f.records[0].src)
					assert.Equal(t, p.tgt, f.records[0].tgt)
					assert.True(t, f.records[0].hit, "an uncontested cast landed")
					word := eff.roomWord
					if p.name == "MS" {
						word = eff.selfWord
					}
					assert.Equal(t, 1, countContaining(drainPlain(3), word), "a watcher sees it land")
				})
			}
		})
	}
}
