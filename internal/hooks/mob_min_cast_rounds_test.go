package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #480: min_cast_rounds holds a MOB's fold-cast open for at least N fold
// steps whatever its fold rate, so a boss with a huge fold rate (the Core
// Guardian makes about 57 folds a round) still gives the room a real windup
// that every disruptor can land in. Each held round shows the spell's own
// wait_observer line instead of the generic weave. A player's cast, and a
// spell without the field, are untouched.

const windupWarning = "core shrieks higher"

func windupSpell(minRounds int) *spells.SpellData {
	sd := testHarmSpell()
	sd.SpellId = "test-windup"
	sd.MobOnly = true
	sd.MinCastRounds = minRounds
	sd.WaitRoomText = "{actor}'s " + windupWarning + "."
	return sd
}

// windupMob puts the Skeleton (instance 100) in lit room 2 with players 1
// and 2, mid-cast of test-windup at player 2, with a fold rate far above
// the folds the spell needs, so without the field it completes at once.
func windupMob(t *testing.T, minRounds int) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-windup": windupSpell(minRounds)}))
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.RoomId = room.RoomId
	m.Character.Validate()
	m.Character.ConvictionMax.Value, m.Character.Conviction = 1000, 1000
	m.Character.SetAggro(2, 0, characters.DefaultAttack)
	require.NoError(t, m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-windup", FoldsNeeded: 2, FoldsPerRound: 57,
			TotalConvictionCost: 20, TargetUserIds: []int{2}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	drainPlain(1)
	drainPlain(2)
	return m, room
}

func TestMobMinCastRounds_HoldsTheCastAndWarnsEachRound(t *testing.T) {
	m, room := windupMob(t, 2)

	handleMobFoldCasting(m, room)
	require.True(t, m.Character.IsCasting(), "round 1 of a two-round windup must not resolve")
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "Skeleton's "+windupWarning), "the held round shows the spell's warning: %v", got)
	require.Zero(t, countContaining(got, "weaves magic"), "the warning replaces the generic weave: %v", got)

	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting(), "round 2 resolves the cast")
}

func TestMobMinCastRounds_DisruptorInRoundTwoCancels(t *testing.T) {
	m, room := windupMob(t, 2)

	handleMobFoldCasting(m, room)
	require.True(t, m.Character.IsCasting())

	require.True(t, maybeInterruptSpellOnTarget(&m.Character, "neural-stun", state.ActorRef{UserId: 1}),
		"a disruptor landing in the second round must cancel the cast")
	require.False(t, m.Character.IsCasting())
	drainPlain(2)
	require.False(t, handleMobFoldCasting(m, room), "a cancelled windup steps nothing")
	require.Zero(t, countContaining(drainPlain(2), "Test Bolt"), "and resolves nothing")
}

func TestMobMinCastRounds_AbsentFieldCompletesInOneRound(t *testing.T) {
	m, room := windupMob(t, 0)

	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting(), "without the field the cast completes on its first step, as before")
	require.Zero(t, countContaining(drainPlain(1), windupWarning))
}

// The field is a mob-cast rule. A player's cast of a spell carrying it steps
// exactly as it did before: the fold rate alone decides when it completes.
func TestMobMinCastRounds_PlayerCastIgnoresIt(t *testing.T) {
	windupMob(t, 2)
	u := users.GetByUserId(1)
	u.Character.Activity = activity.NewMachine()
	require.NoError(t, u.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-windup", FoldsNeeded: 2, FoldsPerRound: 57, TotalConvictionCost: 20},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	u.Character.ConvictionMax.Value, u.Character.Conviction = 1000, 1000

	res := processFoldRound(u.Character)
	require.True(t, res.CastComplete, "a player's cast completes on its first step whatever min_cast_rounds says")
}
