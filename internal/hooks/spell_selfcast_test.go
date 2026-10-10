package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spellHelpParityPairings adds MS, a mob casting on itself through
// resolveMobSpell's real self branch, to the four contested pairings.
func spellHelpParityPairings() []spellParityPairing {
	mobCaster := func(f *spellParityFixture) *characters.Character { return &f.casterMob.Character }
	return append(spellParityPairings(), spellParityPairing{
		name: "MS", src: combat.Mob, tgt: combat.Mob, caster: mobCaster, target: mobCaster,
		casterRef: func(f *spellParityFixture) state.ActorRef {
			return state.ActorRef{MobInstanceId: f.casterMob.InstanceId}
		},
		cast: func(f *spellParityFixture, s *spells.SpellData) {
			resolveMobSpell(f.casterMob, activity.CastingData{SpellId: s.SpellId,
				TargetMobInstanceIds: []int{f.casterMob.InstanceId}}, s, f.room)
		},
	})
}

// Slice 3b: a mob casting on itself takes the shared step and appliers. MS
// recorded nothing, had no purge, and its shield line read differently from
// a player's ("forms around").
func TestSpellSelfCast_AMobOnItselfIsSeenCleansedAndRecorded(t *testing.T) {
	ms := spellHelpParityPairings()[4]
	t.Run("shield", func(t *testing.T) {
		f := newSpellParityFixture(t, spellContestAttackWin())

		ms.cast(f, shieldSpellForParityTest())
		landQueuedConditions()

		assert.Equal(t, 1, countContaining(drainPlain(3), "A faint ward of conviction shimmers around Skeleton"))
		require.Len(t, f.records, 1, "a self-cast is recorded")
		assert.Equal(t, combat.Mob, f.records[0].src)
		assert.True(t, f.records[0].hit)
	})
	t.Run("purge", func(t *testing.T) {
		f := newSpellParityFixture(t, spellContestAttackWin())
		poisonForPurgeTest(t, &f.casterMob.Character)

		ms.cast(f, purgeSpellForParityTest())

		assert.False(t, poisoned(&f.casterMob.Character), "the mob cleanses itself")
	})
}

// A mob never harms itself: a harmful spell that finds the caster in its own
// target list applies nothing to it. This passes before and after the
// change; it pins the guard the new self branch carries.
func TestSpellSelfCast_AMobNeverHarmsItself(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())

	spellHelpParityPairings()[4].cast(f, physicalHarmSpellForCollapseTest())

	assert.Equal(t, 1000, f.casterMob.Character.Health)
	assert.Empty(t, f.records)
}
