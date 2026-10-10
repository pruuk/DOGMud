package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Condition ids clear of the fixture (100, 101), the narration conditions
// (7001-7013) and the notice conditions (7101-7105).
const (
	castWardAId   = 7201 // ward family, all three start lines
	castWardBId   = 7202 // ward family, all three start lines
	castCallerId  = 7203 // names the caster in the holder and room lines
	castNoActorId = 7204 // start_actee and start_observer only
	castHeatId    = 7205 // infravision, so a reader in the dark makes out shapes
	castSightId   = 7206 // see-hidden, so a reader perceives a hidden mob
)

func seedCastLineConditions() func() {
	return conditions.SeedConditionsForTest(castLineSpecs())
}

// seedDistinctCasterWardLines seeds castLineSpecs with Test Ward's caster
// line worded apart from its room line, so a caster who also reads the room
// line is told apart from one who reads start_actor.
func seedDistinctCasterWardLines() func() {
	specs := castLineSpecs()
	specs[castWardAId].StartActorText = "You ward {actee}."
	return conditions.SeedConditionsForTest(specs)
}

func castLineSpecs() map[int]*conditions.ConditionSpec {
	return map[int]*conditions.ConditionSpec{
		castWardAId: {ConditionId: castWardAId, Name: "Test Ward", Family: conditions.FamilyWard, RoundInterval: 1, TriggerCount: 10,
			StartActorText: "A ward settles over {actee}.", StartUserText: "A ward settles over you.", StartRoomText: "A ward settles over {actee_plain}.",
			EndUserText: "Your Test Ward fades.", EndRoomText: "The ward around {actee_plain} fades."},
		castWardBId: {ConditionId: castWardBId, Name: "Test Bulwark", Family: conditions.FamilyWard, RoundInterval: 1, TriggerCount: 10,
			StartActorText: "A bulwark rises around {actee}.", StartUserText: "A bulwark rises around you.", StartRoomText: "A bulwark rises around {actee_plain}.",
			EndUserText: "Your Test Bulwark fades.", EndRoomText: "The bulwark around {actee_plain} fades."},
		castCallerId: {ConditionId: castCallerId, Name: "Test Mark", RoundInterval: 1, TriggerCount: 10,
			StartActorText: "You mark {actee}.", StartUserText: "{actor} marks you.", StartRoomText: "{actor_plain} marks {actee_plain}.",
			EndUserText: "The mark fades."},
		castNoActorId: {ConditionId: castNoActorId, Name: "Test Quietcast", RoundInterval: 1, TriggerCount: 10,
			StartUserText: "You feel quiet.", StartRoomText: "{actee_plain} looks quiet.", EndUserText: "The quiet fades."},
		castHeatId: {ConditionId: castHeatId, Name: "Test Cast Heat Eyes", Secret: true,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		castSightId: {ConditionId: castSightId, Name: "Test Cast True Sight", Secret: true,
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}
}

// seedCastObserver adds user 3, Orin, to room 1 as a third party who is
// neither caster nor holder.
func seedCastObserver(t *testing.T) {
	t.Helper()
	observer := users.NewTestUser(3, "observer", "Orin", 1003)
	observer.Character.RoomId = 1
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{
		1: users.GetByUserId(1), 2: users.GetByUserId(2), 3: observer,
	})
	t.Cleanup(restore)
	rooms.LoadRoom(1).AddPlayer(3)
}

func drainCastParties() {
	drainPlain(1)
	drainPlain(2)
	drainPlain(3)
}

// Cast on another: the caster reads start_actor, the holder start_actee and
// the room start_observer, each exactly once (spec section 2's table).
func TestConditionCast_OnAnotherTellsEachAudienceOneLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedDistinctCasterWardLines()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90) // pin fully lit
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Source: "spell", Caster: state.ActorRef{UserId: 1}})

	caster, holder, room := drainPlain(1), drainPlain(2), drainPlain(3)
	assert.Equal(t, []string{"You ward Bobrick."}, caster)
	assert.NotContains(t, caster, "A ward settles over Bobrick.", "the room line excludes the caster")
	assert.Equal(t, []string{"A ward settles over you."}, holder)
	assert.Equal(t, []string{"A ward settles over Bobrick."}, room)

	got := users.GetByUserId(2).Character.GetConditions(castWardAId)
	require.Len(t, got, 1)
	assert.Equal(t, state.ActorRef{UserId: 1}, got[0].Caster, "the record remembers its caster")
	assert.Equal(t, "spell", got[0].Source, "every door stamps the source")
}

// Self-cast: the holder line is the caster's line, so start_actor is never
// told, and the room reads start_observer.
func TestConditionCast_SelfCastTellsTheHolderAndTheRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 1, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"A ward settles over you."}, drainPlain(1))
	assert.Equal(t, []string{"A ward settles over Aliceia."}, drainPlain(3))
}

// The crit marker the spell trio carried moves to the caster's line, and to
// the holder's line on a self-cast.
func TestConditionCast_CritMarkerRidesTheCasterLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}, CasterCrit: true})
	assert.Equal(t, []string{"A ward settles over Bobrick. [CRIT!]"}, drainPlain(1))
	assert.Equal(t, []string{"A ward settles over you."}, drainPlain(2))
	assert.Equal(t, []string{"A ward settles over Bobrick."}, drainPlain(3))

	ApplyConditions(events.Condition{UserId: 1, ConditionId: castWardBId, Caster: state.ActorRef{UserId: 1}, CasterCrit: true})
	assert.Equal(t, []string{"A bulwark rises around you. [CRIT!]"}, drainPlain(1))
}

// {actor} is filled from the caster and hidden from a reader who makes out
// shapes only; the caster's own line hides the holder the same way.
func TestConditionCast_NamesAreHiddenFromAReaderAtShapes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	darken(t, 1)
	for _, uid := range []int{1, 2, 3} {
		require.True(t, users.GetByUserId(uid).Character.Conditions.AddCondition(castHeatId, true))
	}
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"You mark a figure."}, drainPlain(1))
	assert.Equal(t, []string{"A figure marks you."}, drainPlain(2))
	assert.Equal(t, []string{"A figure marks a figure."}, drainPlain(3))
}

// In the light the holder and the room read the caster's name.
func TestConditionCast_ActorTokenNamesTheCasterInTheLight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"You mark Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia marks you."}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia marks Bobrick."}, drainPlain(3))
}

// A caster who is a mob has no client: the holder and the room still read
// their lines.
func TestConditionCast_AMobCasterNamesTheMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{MobInstanceId: 100}})

	assert.Equal(t, []string{"Skeleton marks you."}, drainPlain(2))
	assert.Equal(t, []string{"Skeleton marks Bobrick."}, drainPlain(3))
	assert.Equal(t, []string{"Skeleton marks Bobrick."}, drainPlain(1), "a bystander who is neither party reads only the room line")
}

// R6: a second ward replaces the first, and the holder and the room read the
// replacement line in place of the old ward's end line.
func TestConditionCast_ANewWardReplacesTheOldWithTheReplacementLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 2}})
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardBId, Caster: state.ActorRef{UserId: 2}})
	PruneConditions(events.NewTurn{TurnNumber: 1})

	holder := users.GetByUserId(2).Character
	assert.False(t, holder.HasCondition(castWardAId))
	assert.True(t, holder.HasCondition(castWardBId))
	assert.Equal(t, []string{"Your Test Ward fades as Test Bulwark takes hold.", "A bulwark rises around you."}, drainPlain(2))
	assert.Equal(t, []string{"Bobrick's Test Ward fades as Test Bulwark takes hold.", "A bulwark rises around Bobrick."}, drainPlain(3))
}

// Re-landing the same ward is a refresh: no start lines and no replacement
// line, as before (wasAlreadyActive).
func TestConditionCast_RecastOfTheSameWardIsASilentRefresh(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})
	drainCastParties()
	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 3}})

	assert.Empty(t, drainPlain(1))
	assert.Empty(t, drainPlain(2))
	assert.Empty(t, drainPlain(3))
	assert.Equal(t, state.ActorRef{UserId: 3}, users.GetByUserId(2).Character.GetConditions(castWardAId)[0].Caster,
		"the newest application owns the record")
}

// A condition with no start_actor tells the caster nothing itself; the spell
// keeps its own line for that case (NarratesCastStart), so this hook must not
// invent one.
func TestConditionCast_NoCasterLineAuthoredTellsTheCasterNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castNoActorId, Caster: state.ActorRef{UserId: 1}})
	assert.Empty(t, drainPlain(1))
	assert.Equal(t, []string{"You feel quiet."}, drainPlain(2))
}

// A hidden mob holder: a caster who perceives it reads its name in the
// caster line, as a spell's own lines read (mobDisplayName's viewer rule,
// #382); a reader who does not perceive it reads no room line at all (#458).
func TestConditionCast_ACasterWhoPerceivesAHiddenHolderReadsItsName(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	t.Cleanup(seedDistinctCasterWardLines())
	caster := users.GetByUserId(1)
	require.True(t, caster.Character.Conditions.AddCondition(castSightId, true))
	require.True(t, caster.Character.Perceives(&m.Character))
	drainPlain(1)
	drainPlain(2)

	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})

	casterLines := drainPlain(1)
	assert.Equal(t, []string{"You ward Skeleton."}, casterLines)
	assert.NotContains(t, casterLines, "A ward settles over Skeleton.", "the room line excludes the caster")
	assert.Empty(t, drainPlain(2), "a reader who does not perceive the holder reads nothing")
}
