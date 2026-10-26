package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #558 and the 2026-10-17 pre-deploy playtest: the Tavern Cellar's quest gate
// was a room_enter trigger that teleported the player back out of room 486,
// so `down` printed the refusal, then ">>> You head downward.", then 486's
// "Something notices you in the darkness!", for a move that never happened.
// A room_gate trigger now refuses the step before the move.

const gateTestLine = "You have no reason to go up there."

func registerGateQuest(t *testing.T, inRoom int) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999558,
		Name:    "Gate Test Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       2,
			Conditions: questengine.Conditions{Missing: []string{"999558-start"}, InRoom: inRoom},
			Actions:    []questengine.ActionDef{{SendText: gateTestLine}},
		}},
	})
}

func TestGo_QuestGateRefusesBeforeTheMove(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	require.Equal(t, 1, room.RoomId, "fixture premise: the user starts in room 1")
	registerGateQuest(t, 1)

	user.Character.ActionPoints = 100
	stamina := user.Character.Stamina
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Equal(t, 1, user.Character.RoomId, "the gate kept the player in room 1")
	require.Contains(t, out, gateTestLine, "the gate says why")
	require.NotContains(t, out, "You head", "no departure line for a refused move")
	require.Equal(t, 100, user.Character.ActionPoints, "a refused step costs no action points")
	require.Equal(t, stamina, user.Character.Stamina, "a refused step costs no stamina")
}

// Gates apply to walking through an exit only (owner ruling 2026-10-26): a
// teleport, a system move or the ferry into a gated room is not refused.
func TestQuestGate_TeleportIsUnaffected(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	registerGateQuest(t, 1)
	t.Cleanup(func() { _ = rooms.MoveToRoom(user.UserId, 1) })
	events.DrainQueuedMessagesForTest(user.UserId)

	require.NoError(t, rooms.MoveToRoom(user.UserId, 2))

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Equal(t, 2, user.Character.RoomId, "a teleport is not a walk")
	require.NotContains(t, out, gateTestLine)
	require.True(t, actions.QuestGateBlocks(user, room, rooms.LoadRoom(2)),
		"the gate still stands for a walk from room 1")
}

// in_room is read with the player still on their own side, so a gate on the
// way down never traps a player coming from somewhere else.
func TestGo_QuestGateOnlyGuardsItsOwnSide(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	registerGateQuest(t, 99) // a gate for players standing in room 99 only
	t.Cleanup(func() { _ = rooms.MoveToRoom(user.UserId, 1) })
	user.Character.ActionPoints = 100
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Equal(t, 2, user.Character.RoomId, "the gate does not stand on room 1's side")
	require.NotContains(t, out, gateTestLine)
}
