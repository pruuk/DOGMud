package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Playtest of 89c58a9f6: the windup collapsed to one casting round. The
// tree's `cast` is a delayed action, drained by MobRoundTick at the START of
// the next round, before DoCombat; so the cast began, and DoCombat's fold
// step then held it and printed the warning, in that same round. The cast
// line and the warning arrived together and the blast came one round later.
// This drives exactly that order: the cast starts in round N (the drained
// delayed action), DoCombat's fold step runs in round N, then N+1, N+2.
func TestMobMinCastRounds_CastStartedThisRoundIsNotSteppedThisRound(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	area := windupSpell(2)
	area.Targeting = combatvocab.TargetArea
	area.IgnoreMoveCooldown = true
	area.BaseFolds = 2 // two folds: within two steps even at a fold rate of 1
	area.CastRoomText = "{actor}'s core flares."
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-windup": area}))
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.RoomId = room.RoomId
	m.Character.Validate()
	m.Character.ConvictionMax.Value, m.Character.Conviction = 1000, 1000
	m.Character.Cooldowns = nil
	m.Character.SetAggro(2, 0, characters.DefaultAttack)
	drainPlain(1)

	pinScheduleRoundForTest(t, 9000) // round N, MobRoundTick: the delayed cast runs; restored on cleanup
	_, err := mobcommands.Cast("test-windup", m, room)
	require.NoError(t, err)
	require.True(t, m.Character.IsCasting(), "fixture: the cast began")
	handleMobFoldCasting(m, room) // round N, DoCombat
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "core flares"), "round N shows the cast line: %v", got)
	require.Zero(t, countContaining(got, windupWarning), "the warning must not share the cast line's round: %v", got)
	require.True(t, m.Character.IsCasting())

	util.SetRoundCountForTest(9001)
	handleMobFoldCasting(m, room)
	require.Equal(t, 1, countContaining(drainPlain(1), windupWarning), "round N+1 is the warning round")
	require.True(t, m.Character.IsCasting(), "and still held")

	util.SetRoundCountForTest(9002)
	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting(), "round N+2 is the blast: two full rounds after the cast line")
}

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
	// The completing round reads as every other fold round (messaging M6
	// slice 4): it carries the spell's wait line once, then the blast. It is
	// not a held round, which the IsCasting check above proves.
	require.Equal(t, 1, countContaining(drainPlain(1), windupWarning))
}

// windupAreaMob is windupMob with an AREA harm spell whose cast-time target
// list holds both players and an add (instance 999) that is gone by the
// held round, as a killed add's instance is.
func windupAreaMob(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	m, room := windupMob(t, 2)
	area := windupSpell(2)
	area.Targeting = combatvocab.TargetArea
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-windup": area}))
	m.Character.Activity = activity.NewMachine()
	require.NoError(t, m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-windup", FoldsNeeded: 2, FoldsPerRound: 57,
			TotalConvictionCost: 20, TargetUserIds: []int{1, 2}, TargetMobInstanceIds: []int{999}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	return m, room
}

// Branch review of #480: an area harm cast checked its cast-time target list
// on every fold step, so one dead add, or one downed or departed player,
// fizzled the whole Core Discharge without a disruptor. resolveMobSpell
// re-reads the room at landing, so the cast now lands on whoever remains.
func TestMobAreaWindup_KilledAddDoesNotFizzleIt(t *testing.T) {
	m, room := windupAreaMob(t)
	require.Nil(t, mobs.GetInstance(999), "fixture: the add is gone")

	handleMobFoldCasting(m, room)
	require.True(t, m.Character.IsCasting(), "a dead add must not fizzle the held round")
	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting())
	require.NotZero(t, countContaining(drainPlain(2), "Test Bolt"), "the cast lands on the players still standing")
}

func TestMobAreaWindup_DownedPlayerDoesNotFizzleIt(t *testing.T) {
	m, room := windupAreaMob(t)
	handleMobFoldCasting(m, room)
	require.True(t, m.Character.IsCasting())

	downed := users.GetByUserId(1)
	downed.Character.Health = 0
	drainPlain(1)
	drainPlain(2)
	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting(), "round 2 resolves despite a downed player")
	require.NotZero(t, countContaining(drainPlain(2), "Test Bolt"), "the standing player is hit")
	require.Equal(t, 0, downed.Character.Health, "the downed player is not struck again")
}

// The fizzle stays for a single-target cast: its one target down ends it.
func TestMobSingleTargetFold_DownedTargetStillFizzles(t *testing.T) {
	m, room := windupMob(t, 2)
	users.GetByUserId(2).Character.Health = 0
	handleMobFoldCasting(m, room)
	require.False(t, m.Character.IsCasting(), "a single-target harm cast at a downed player ends")
	require.NotZero(t, countContaining(drainPlain(1), "fizzles"))
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
