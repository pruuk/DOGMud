package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A player whose saved room no longer exists (its file was deleted, as the
// Shadow Realm's room 75 was on 2026-11-07) enters the world at their start
// room, not nowhere.
func TestEnteringAMissingSavedRoomLandsAtTheStartRoom(t *testing.T) {
	t.Cleanup(SeedRoomsForTest(
		map[int]*Room{9901: {RoomId: 9901, Zone: "MoveZone"}},
		map[string]*ZoneConfig{"MoveZone": {Name: "MoveZone", RoomId: 9901}},
	))
	u := users.NewTestUser(9902, "lost", "Lost", 99902)
	u.Character.RoomId = 9999 // the saved room, gone
	u.Character.SetMiscData(`StartRoom`, 9901)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9902: u}))

	require.NoError(t, MoveToRoom(9902, 9999, true))
	require.Equal(t, 9901, u.Character.RoomId)
}

// An ordinary move to a missing room still refuses and leaves the player put.
func TestMovingToAMissingRoomStillRefuses(t *testing.T) {
	t.Cleanup(SeedRoomsForTest(
		map[int]*Room{9901: {RoomId: 9901, Zone: "MoveZone"}},
		map[string]*ZoneConfig{"MoveZone": {Name: "MoveZone", RoomId: 9901}},
	))
	u := users.NewTestUser(9903, "walker", "Walker", 99903)
	u.Character.RoomId = 9901
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9903: u}))

	require.Error(t, MoveToRoom(9903, 9999))
	require.Equal(t, 9901, u.Character.RoomId)
}
