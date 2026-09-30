package rooms

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// With nothing registered, every hook is a no-op: exits route normally, every
// entry is allowed, and no room is private.
func TestRoutingHooks_DefaultToNoOp(t *testing.T) {
	SetExitRouter(nil)
	SetEntryGuard(nil)
	SetPrivateRoomCheck(nil)

	_, handled := RouteExit(1, 10, `door`)
	assert.False(t, handled)
	assert.False(t, IsRoutedExit(1, 10, `door`))

	allowed, redirect, refusal := checkEntry(1, 10)
	assert.True(t, allowed)
	assert.Zero(t, redirect)
	assert.Empty(t, refusal)

	assert.False(t, IsPrivateRoom(10))
}

func TestRoutingHooks_DelegateToRegistered(t *testing.T) {
	SetExitRouter(func(userId, fromRoomId int, exitName string) (ExitRoute, bool) {
		if fromRoomId == 10 && exitName == `door` {
			if userId == 1 {
				return ExitRoute{RoomId: 99}, true
			}
			return ExitRoute{Refusal: `no`}, true
		}
		return ExitRoute{}, false
	})
	SetEntryGuard(func(userId, toRoomId int) (bool, int, string) {
		if toRoomId == 99 && userId != 1 {
			return false, 10, `not yours`
		}
		return true, 0, ``
	})
	SetPrivateRoomCheck(func(roomId int) bool { return roomId == 99 })
	defer func() {
		SetExitRouter(nil)
		SetEntryGuard(nil)
		SetPrivateRoomCheck(nil)
	}()

	route, handled := RouteExit(1, 10, `door`)
	assert.True(t, handled)
	assert.Equal(t, 99, route.RoomId)

	route, handled = RouteExit(2, 10, `door`)
	assert.True(t, handled)
	assert.Zero(t, route.RoomId)
	assert.Equal(t, `no`, route.Refusal)

	_, handled = RouteExit(1, 10, `east`)
	assert.False(t, handled)

	// An empty exit name is never routed, so a failed exit lookup cannot be
	// mistaken for a door.
	_, handled = RouteExit(1, 10, ``)
	assert.False(t, handled)

	allowed, redirect, refusal := checkEntry(2, 99)
	assert.False(t, allowed)
	assert.Equal(t, 10, redirect)
	assert.Equal(t, `not yours`, refusal)

	allowed, _, _ = checkEntry(1, 99)
	assert.True(t, allowed)

	assert.True(t, IsPrivateRoom(99))
	assert.False(t, IsPrivateRoom(10))
}

// The loot goblin must never pick a player's own room, however much is on
// its floor.
func TestGetRoomWithMostItems_SkipsPrivateRooms(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	roomManager.rooms[2].Items = []items.Item{{ItemId: 10}, {ItemId: 20}, {ItemId: 30}, {ItemId: 40}}
	roomManager.rooms[4].Items = []items.Item{{ItemId: 10}, {ItemId: 20}}

	SetPrivateRoomCheck(func(roomId int) bool { return roomId == 2 })
	defer SetPrivateRoomCheck(nil)

	roomId, itemCt := GetRoomWithMostItems(false, 1, 0)
	assert.Equal(t, 4, roomId)
	assert.Equal(t, 2, itemCt)
}

// Every room built from disk passes through the registered overlay, both
// template-only and with an instance overlay, so a subsystem can keep
// template-owned fields (exits, description) in its own state.
func TestLoadRoomInstance_AppliesRoomOverlay(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	instancePath := seedTemplateRoom(t, tempDir, 100)

	calls := 0
	SetRoomOverlay(func(r *Room) {
		calls++
		r.Description = `overlaid`
		r.Exits[`north`] = exit.RoomExit{RoomId: 101}
	})
	defer SetRoomOverlay(nil)

	room := LoadRoomInstance(100)
	assert.Equal(t, 1, calls)
	assert.Equal(t, `overlaid`, room.Description)
	assert.Equal(t, 101, room.Exits[`north`].RoomId)

	assert.NoError(t, os.WriteFile(instancePath, []byte("roomid: 100\ngold: 5\n"), 0o644))
	room = LoadRoomInstance(100)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 5, room.Gold)
	assert.Equal(t, `overlaid`, room.Description)
}

// A player's room must never be written back into authored content by the
// builder: it carries that player's exits and text in memory.
func TestSaveRoomTemplate_RefusesPrivateRooms(t *testing.T) {
	SetPrivateRoomCheck(func(roomId int) bool { return roomId == 100 })
	defer SetPrivateRoomCheck(nil)
	assert.Error(t, SaveRoomTemplate(Room{RoomId: 100, Zone: `x`}))
}
