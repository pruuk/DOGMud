package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A delayed move_player fires seconds after the event that queued it. By
// then the player may have walked away (1017 newbie-duo verify2: Ossaly
// asked Esk about the arch in 5215, followed her party out, and 37 seconds
// later in 5207 was pulled to the Thornwall temple). These tests pin that
// the move, and the grants queued behind it in the same sequence, only fire
// while the player is still with the mob or in the room whose tree queued
// them.
//
// Quest definitions are not loaded in this package's tests, so a grant is
// observed by the Quest event grant_quest queues, not by the token.

const (
	eskMobId      = 9107
	eskInstanceId = 9107
	eskRoom       = 5215
	eskAwayRoom   = 5207
	eskDestRoom   = 468
)

func questGrantsFor(userId int) []string {
	var tokens []string
	for _, q := range events.DrainQueuedQuestsForTest() {
		if q.UserId == userId {
			tokens = append(tokens, q.QuestToken)
		}
	}
	return tokens
}

// arrivalsAt reports whether a RoomChange for the user into roomId is queued.
// That event is what raises the quest engine's room_enter
// (hooks.QuestRoomEnter), which closes quest 89 on Esk's road (#619).
func arrivalsAt(userId, roomId int) int {
	n := 0
	for _, e := range events.DrainQueuedByTypeForTest(events.RoomChange{}.Type()) {
		if rc, ok := e.(events.RoomChange); ok && rc.UserId == userId && rc.ToRoomId == roomId {
			n++
		}
	}
	return n
}

func seedEskPassage(t *testing.T) {
	t.Helper()
	// Start from an empty delayed queue, quest queue and move queue so only
	// this evaluation's actions count.
	DrainAllDelayedActionsForTest(t)
	events.DrainQueuedQuestsForTest()
	events.DrainQueuedByTypeForTest(events.RoomChange{}.Type())

	cleanRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		eskRoom:     {RoomId: eskRoom, Zone: "TestZone", Title: "Threshold", Exits: map[string]exit.RoomExit{}},
		eskAwayRoom: {RoomId: eskAwayRoom, Zone: "TestZone", Title: "Provisions", Exits: map[string]exit.RoomExit{}},
		eskDestRoom: {RoomId: eskDestRoom, Zone: "TestZone", Title: "Temple", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{})
	t.Cleanup(cleanRooms)
	t.Cleanup(seedTestMob(t, eskMobId, eskInstanceId, eskRoom, "Warden Esk"))
	t.Cleanup(seedTestUser(t, 1, "ossaly", "Ossaly", eskRoom))

	u := requireUser(t, 1)
	u.Character.QuestProgress = map[int]string{30: "end", 89: "start"}

	tree, err := LoadTreeFromFile("../../_datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml")
	if err != nil {
		t.Fatalf("load Esk tree: %v", err)
	}
	ctx := &EvalContext{
		Event: EventContext{
			EventType: "player_topic", // the passage talk topic (#375 R12)
			UserId:    1,
			Text:      "passage",
			RoomId:    eskRoom,
		},
		MobState:   NewBehaviorState(),
		MobId:      eskMobId,
		InstanceId: eskInstanceId,
		RoomId:     eskRoom,
		MobName:    "Warden Esk",
	}
	if res := tree.Evaluate(ctx); res != Success {
		t.Fatalf("Esk passage sequence: expected Success, got %v", res)
	}
}

func TestEskPassage_AskerLeft_NoMoveNoGrant(t *testing.T) {
	seedEskPassage(t)
	u := requireUser(t, 1)

	// The asker walks out before the arch opens.
	u.Character.RoomId = eskAwayRoom

	DrainAllDelayedActionsForTest(t)

	if u.Character.RoomId != eskAwayRoom {
		t.Errorf("asker who left was moved: in room %d, want %d", u.Character.RoomId, eskAwayRoom)
	}
	if got := questGrantsFor(1); len(got) != 0 {
		t.Errorf("asker who left was granted %v, want nothing", got)
	}
	if n := arrivalsAt(1, eskDestRoom); n != 0 {
		t.Errorf("asker who left arrived in %d %d times, want 0", eskDestRoom, n)
	}
}

// Quest 89 closes through its own room_enter trigger on 468, not a grant in
// Esk's tree, so the tree grants nothing and the arrival is what counts.
func TestEskPassage_AskerStayed_MovedAndArrives(t *testing.T) {
	seedEskPassage(t)
	u := requireUser(t, 1)

	DrainAllDelayedActionsForTest(t)

	if u.Character.RoomId != eskDestRoom {
		t.Errorf("asker who stayed: in room %d, want %d", u.Character.RoomId, eskDestRoom)
	}
	if got := questGrantsFor(1); len(got) != 0 {
		t.Errorf("asker who stayed: tree granted %v, want nothing (room_enter on %d closes quest 89)", got, eskDestRoom)
	}
	if n := arrivalsAt(1, eskDestRoom); n != 1 {
		t.Errorf("asker who stayed arrived in %d %d times, want 1", eskDestRoom, n)
	}
}

// A room tree (no mob) queues the same shape: 6467 The Landing moves the
// player at 4.0s and grants at 4.5s. The room whose tree queued the move is
// the one the player must still be in.
const delayedRoomMoveTree = `
tree:
  type: sequence
  children:
    - type: action
      do: move_player
      room_id: 2
      delay: 1.0
    - type: action
      do: grant_quest
      quest: "77-end"
      delay: 1.5
`

func runDelayedRoomMove(t *testing.T, leave bool) (roomId int, grants []string) {
	t.Helper()
	DrainAllDelayedActionsForTest(t)
	events.DrainQueuedQuestsForTest()
	cleanRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1: {RoomId: 1, Zone: "TestZone", Title: "Origin", Exits: map[string]exit.RoomExit{}},
		2: {RoomId: 2, Zone: "TestZone", Title: "Dest", Exits: map[string]exit.RoomExit{}},
		3: {RoomId: 3, Zone: "TestZone", Title: "Elsewhere", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{})
	t.Cleanup(cleanRooms)
	t.Cleanup(seedTestUser(t, 1, "alice", "Aliceia", 1))
	u := requireUser(t, 1)

	tree, err := LoadTreeFromBytes([]byte(delayedRoomMoveTree))
	if err != nil {
		t.Fatalf("load tree: %v", err)
	}
	ctx := &EvalContext{
		Event:    EventContext{EventType: "room_command", UserId: 1, RoomId: 1},
		MobState: NewBehaviorState(),
		RoomId:   1,
	}
	if res := tree.Evaluate(ctx); res != Success {
		t.Fatalf("expected Success, got %v", res)
	}
	if leave {
		u.Character.RoomId = 3
	}
	DrainAllDelayedActionsForTest(t)
	return u.Character.RoomId, questGrantsFor(1)
}

func TestDelayedRoomMove_PlayerLeft_NoMoveNoGrant(t *testing.T) {
	roomId, grants := runDelayedRoomMove(t, true)
	if roomId != 3 {
		t.Errorf("player who left was moved: in room %d, want 3", roomId)
	}
	if len(grants) != 0 {
		t.Errorf("player who left was granted %v, want nothing", grants)
	}
}

func TestDelayedRoomMove_PlayerStayed_MovedAndGranted(t *testing.T) {
	roomId, grants := runDelayedRoomMove(t, false)
	if roomId != 2 {
		t.Errorf("player who stayed: in room %d, want 2", roomId)
	}
	if len(grants) != 1 || grants[0] != "77-end" {
		t.Errorf("player who stayed: grants %v, want [77-end]", grants)
	}
}
