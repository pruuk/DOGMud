package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A room line about a hidden mob never names it (epic #382). mobDisplayName
// fed GetMobNameIndexed, whose adjectives include "hidden", so a room-wide
// line named a hidden mob to every reader who sees faces, as
// "Skeleton (hidden)". Every reader is unseen-by-default here: a room-wide
// line has no one viewer to ask Perceives of, so it reads "something", as
// mobSubjectName already did for the spell-channel lines (#274, owner R3).

// hideMob puts m into the Hidden awareness state the way sneak does.
func hideMob(t *testing.T, m *mobs.Mob) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "hidden_mob_room_lines_test"}
	require.NoError(t, m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	m.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, m.Character.IsHidden())
}

// litRoomOneWithHiddenSkeleton seeds the registries and the narration
// conditions, pins room 1 lit, hides the Skeleton (instance 100) and drains
// user 2, who reads faces in room 1.
func litRoomOneWithHiddenSkeleton(t *testing.T) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	hideMob(t, m)
	events.DrainQueuedMessagesForTest(2)
	return m
}

// requireUnnamed asserts got holds want exactly once and never the mob's
// name or its hidden adjective.
func requireUnnamed(t *testing.T, got []string, want string) {
	t.Helper()
	assert.Zero(t, countContaining(got, "Skeleton"), "a faces reader must not read a hidden mob's name: %v", got)
	assert.Zero(t, countContaining(got, "hidden"), "nor its hidden adjective: %v", got)
	assert.Equal(t, 1, countContaining(got, want), "%v", got)
}

func TestHiddenMob_ConditionStartLineDoesNotNameIt(t *testing.T) {
	litRoomOneWithHiddenSkeleton(t)
	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
	requireUnnamed(t, drainPlain(2), "Something glows.")
}

func TestHiddenMob_ConditionEndLineDoesNotNameIt(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	for _, id := range []int{fadeConditionId, shadeConditionId} {
		require.True(t, m.Character.Conditions.AddCondition(id, false))
		expire(t, m.Character.Conditions.List, id)
	}
	PruneConditions(events.NewTurn{TurnNumber: 1})
	got := drainPlain(2)
	// fade authors {actee}, the tagged name; shade authors a bare
	// {actee_plain}, as shipped condition 9 does.
	requireUnnamed(t, got, "Something fades.")
	requireUnnamed(t, got, "Something emerges from the shadows.")
}

func TestHiddenMob_RoundTickTriggerLineDoesNotNameIt(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	require.True(t, m.Character.Conditions.AddCondition(shiverConditionId, false))
	tickMobConditions(m, 100)
	requireUnnamed(t, drainPlain(2), "Something shivers.")
}

func TestHiddenMob_DrainAreaRoomLineDoesNotNameIt(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	t.Cleanup(combat.SetChannelAttackContestRunnerForTest(attackWinContest(t)))
	f.casterMob.Character.Validate()
	hideMob(t, f.casterMob)
	spell := &spells.SpellData{SpellId: "test-core-recharge", Name: "Core Recharge", EffectType: "drain_area",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetArea}

	resolveMobDrainArea(f.casterMob, f.room, spell)

	got := drainPlain(f.watcher.UserId)
	requireUnnamed(t, got, "Something's Core Recharge tears the life from everyone in the room!")
}

// A visible mob is still named: the null probe for the four tests above.
func TestVisibleMob_ConditionStartLineNamesIt(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedMessagesForTest(2)
	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Skeleton glows."))
}

// A per-viewer name follows that viewer's Perceives: a viewer who sees the
// hidden (a see-hidden condition) reads the name, one who does not reads
// "something". Room-wide (viewer 0) is "something" whoever is in the room.
func TestMobDisplayName_HiddenMobFollowsTheViewer(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	room := rooms.LoadRoom(1)
	const seeHiddenId = 7101
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		seeHiddenId: {ConditionId: seeHiddenId, Name: "Test True Sight",
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	seer := users.GetByUserId(1)
	require.True(t, seer.Character.Conditions.AddCondition(seeHiddenId, true))
	require.True(t, seer.Character.Perceives(&m.Character))
	require.False(t, users.GetByUserId(2).Character.Perceives(&m.Character))

	assert.Contains(t, plainText(mobDisplayName(m, room, 1)), "Skeleton")
	assert.Equal(t, "something", plainText(mobDisplayName(m, room, 2)))
	assert.Equal(t, "something", plainText(mobDisplayName(m, room, 0)))
	assert.Equal(t, "Something", plainText(mobSubjectName(m, room)))
}
