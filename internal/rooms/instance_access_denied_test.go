package rooms

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// TestMoveToRoom_InstanceDenialIsSentinel pins that refusing an unauthorized
// player entry to an instanced room returns ErrInstanceAccessDenied, so
// callers (go.go) can tell an ordinary gameplay refusal, which MoveToRoom has
// already narrated, from a server fault (#371).
func TestMoveToRoom_InstanceDenialIsSentinel(t *testing.T) {
	overworldId := 4243
	ephEntry := ephemeralRoomIdMinimum + 310

	cleanupRooms := SeedRoomsForTest(
		map[int]*Room{
			overworldId: {RoomId: overworldId, Zone: "Overworld", Title: "Town Square", players: []int{}},
			ephEntry:    {RoomId: ephEntry, Zone: "TestZone", Title: "Ephemeral Hall", players: []int{}},
		},
		map[string]*ZoneConfig{
			"Overworld": {Name: "Overworld", RoomId: overworldId, RoomIds: map[int]struct{}{overworldId: {}}},
			"TestZone":  {Name: "TestZone", RoomId: ephEntry, RoomIds: map[int]struct{}{ephEntry: {}}},
		},
	)
	defer cleanupRooms()

	const userId = 98
	u := users.NewTestUser(userId, "outsider", "Outsider", uint64(userId))
	u.Character.RoomId = overworldId
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{userId: u})
	defer cleanupUsers()

	origRegistry := instanceRegistry
	defer func() { instanceRegistry = origRegistry }()
	instanceRegistry = NewInstanceRegistry()
	instanceRegistry.Add(&ZoneInstance{
		InstanceId:      ephEntry,
		TemplateZone:    "TestZone",
		AuthorizedUsers: []int{userId + 1},
		OwnerUserId:     userId + 1,
		PortalDuration:  "1m",
		OverworldRoomId: overworldId,
		EntryRoomId:     ephEntry,
		RoomIdMap:       map[int]int{1: ephEntry},
	})

	err := MoveToRoom(userId, ephEntry)
	if !errors.Is(err, ErrInstanceAccessDenied) {
		t.Fatalf("MoveToRoom into an instance not opened for the player = %v, want ErrInstanceAccessDenied", err)
	}
	if u.Character.RoomId != overworldId {
		t.Fatalf("denied player moved to %d, want to stay in %d", u.Character.RoomId, overworldId)
	}
}
