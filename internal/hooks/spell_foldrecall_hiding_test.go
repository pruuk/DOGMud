package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428: fold-recall's departure and arrival lines went out on plain
// Room.SendText, so a bystander who makes out shapes only (heat sight in an
// unlit room) read the caster's name. Both now go through the room's
// name-hiding sight path, as the rest of the spell narration does.
//
// Aliceia (user 1) recalls from room 1 to her anchor in room 2. Bobrick
// (user 2) watches her leave; Carrow (user 3) watches her arrive. Both see
// by heat only.
func TestFoldRecall_ShapesOnlyBystandersReadAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	darken(t, 2)

	caster := users.GetByUserId(1)
	leaver := users.GetByUserId(2)
	watcher := users.NewTestUser(3, "cara", "Carrow", 1003)
	watcher.Character.RoomId = 2
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: caster, 2: leaver, 3: watcher})
	defer restoreUsers()
	rooms.LoadRoom(2).AddPlayer(3)
	require.True(t, leaver.Character.Conditions.AddCondition(heatEyesConditionId, true))
	require.True(t, watcher.Character.Conditions.AddCondition(heatEyesConditionId, true))
	caster.Character.SetMiscData("fold-anchor-room", 2)
	drainPlain(1)
	drainPlain(2)
	drainPlain(3)

	resolveFoldRecall(actions.NewUserActorInRoom(caster, rooms.LoadRoom(1)))
	require.Equal(t, 2, caster.Character.RoomId, "fixture: the recall must land")

	left, arrived := drainPlain(2), drainPlain(3)
	require.Equal(t, 0, countContaining(left, "Aliceia"), "departure named the caster: %v", left)
	require.Equal(t, 0, countContaining(arrived, "Aliceia"), "arrival named the caster: %v", arrived)
	require.Equal(t, 1, countContaining(left, "figure folds through the Veil and vanishes!"), "departure: %v", left)
	require.Equal(t, 1, countContaining(arrived, "figure folds through the Veil and appears!"), "arrival: %v", arrived)
}

const foldRecallLightCond = 9782 // a carried light, literal strength 60

// #456 review G3, owner ruling 2026-10-09: a mover is seen by the light they
// carry on their own way out. Aliceia carries the only light in dark room 1
// and folds away; Bobrick, left in the dark, saw her vanish by her own light.
// The line was judged after the teleport, when the light had gone with her.
func TestFoldRecall_DepartureIsJudgedByTheCastersOwnLight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		foldRecallLightCond: {ConditionId: foldRecallLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	from := rooms.LoadRoom(1)
	from.Lamp = nil
	darken(t, 1)
	caster, watcher := users.GetByUserId(1), users.GetByUserId(2)
	require.True(t, caster.Character.Conditions.AddCondition(foldRecallLightCond, false))
	require.Equal(t, messaging.SightFull, from.ParticipantSight(watcher.UserId), "fixture: the caster's light lights the room")
	caster.Character.SetMiscData("fold-anchor-room", 2)
	drainPlain(2)

	resolveFoldRecall(actions.NewUserActorInRoom(caster, from))
	require.Equal(t, 2, caster.Character.RoomId, "fixture: the recall must land")
	require.Equal(t, messaging.SightNone, from.ParticipantSight(watcher.UserId), "fixture: the light left with the caster")

	left := drainPlain(2)
	require.Equal(t, 1, countContaining(left, "Aliceia folds through the Veil and vanishes!"), "departure: %v", left)
}

// The unchanged case: bystanders who see clearly still read the caster's name
// on both sides of the fold.
func TestFoldRecall_ClearSightedBystandersReadTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	from, to := rooms.LoadRoom(1), rooms.LoadRoom(2)
	from.Lamp, to.Lamp = rooms.LampPtr(90), rooms.LampPtr(90)

	caster := users.GetByUserId(1)
	leaver := users.GetByUserId(2)
	watcher := users.NewTestUser(3, "cara", "Carrow", 1003)
	watcher.Character.RoomId = 2
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: caster, 2: leaver, 3: watcher})
	defer restoreUsers()
	to.AddPlayer(3)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(leaver.Character, from), "precondition: the leaver sees clearly")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(watcher.Character, to), "precondition: the watcher sees clearly")
	caster.Character.SetMiscData("fold-anchor-room", 2)
	drainPlain(1)
	drainPlain(2)
	drainPlain(3)

	resolveFoldRecall(actions.NewUserActorInRoom(caster, from))
	require.Equal(t, 2, caster.Character.RoomId, "fixture: the recall must land")

	left, arrived := drainPlain(2), drainPlain(3)
	require.Equal(t, 1, countContaining(left, "Aliceia folds through the Veil and vanishes!"), "departure: %v", left)
	require.Equal(t, 1, countContaining(arrived, "Aliceia folds through the Veil and appears!"), "arrival: %v", arrived)
}
