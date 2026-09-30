package rooms

import "sync"

// Routing hooks let a subsystem that rooms must not import (housing, today)
// take part in three decisions the room layer owns:
//
//   - where a particular player ends up when they take a particular exit
//     (ExitRouter), so one authored door can lead every lodger to their own
//     room;
//   - whether a player may be placed in a room at all (EntryGuard), checked
//     inside MoveToRoom so walking, recall, summons, respawn and login
//     placement are all covered by one rule;
//   - whether a room is private (PrivateRoomCheck), so room-wide sweeps such
//     as the loot goblin leave it alone.
//
// Each is a single registered func, set once at boot, in the same style as
// SetBTreeStateEvictor. Unset hooks are no-ops, so tests and tools that never
// boot the subsystem see ordinary room behaviour.

// ExitRoute is an ExitRouter's answer for one player taking one exit.
type ExitRoute struct {
	// RoomId is where this player goes instead of the exit's authored room.
	// Zero with no Choices means "refused": Refusal says why.
	RoomId int
	// Refusal is shown to the player when they are refused.
	Refusal string
	// Choices, when there are two or more, means this player may go to
	// several places and must pick one (a lodger who is also a guest in
	// other lodgings). RoomId is then zero and the caller asks.
	Choices []ExitChoice
}

// ExitChoice is one destination a routed exit offers.
type ExitChoice struct {
	Label  string // what the player types or picks, e.g. "home", "Alice"
	RoomId int
}

// Leads reports whether the route could take the player to roomId, either
// directly or as one of its choices.
func (r ExitRoute) Leads(roomId int) bool {
	if roomId == 0 {
		return false
	}
	if r.RoomId == roomId {
		return true
	}
	for _, c := range r.Choices {
		if c.RoomId == roomId {
			return true
		}
	}
	return false
}

// ExitRouter reports whether exitName in fromRoomId is a routed exit. When it
// is (handled true), the returned route replaces the authored destination for
// this player, and the exit's lock is not consulted: the router has already
// decided who may pass.
type ExitRouter func(userId int, fromRoomId int, exitName string) (route ExitRoute, handled bool)

// EntryGuard reports whether userId may be placed in toRoomId. When it
// refuses a spawn placement (login, respawn), MoveToRoom sends the player to
// redirectRoomId instead of failing, so nobody is ever stranded with no room.
type EntryGuard func(userId int, toRoomId int) (allowed bool, redirectRoomId int, refusal string)

// PrivateRoomCheck reports whether a room belongs to a player and must be left
// out of room-wide sweeps that take things from floors.
type PrivateRoomCheck func(roomId int) bool

// RoomOverlay adjusts a room's template-owned fields (exits, title,
// description, nouns, coordinates) from state kept outside the room files.
// It runs on every room built from disk, after the instance overlay, because
// those fields are instance:"skip" and would otherwise always come back as
// the template. It must be idempotent and must not load other rooms.
type RoomOverlay func(r *Room)

var (
	routingHooksMu   sync.RWMutex
	exitRouter       ExitRouter
	entryGuard       EntryGuard
	privateRoomCheck PrivateRoomCheck
	roomOverlay      RoomOverlay
)

// SetRoomOverlay registers the room overlay. Passing nil clears it.
func SetRoomOverlay(fn RoomOverlay) {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	roomOverlay = fn
}

// applyRoomOverlay runs the registered overlay, if any, on a freshly built room.
func applyRoomOverlay(r *Room) {
	routingHooksMu.RLock()
	fn := roomOverlay
	routingHooksMu.RUnlock()
	if fn == nil || r == nil {
		return
	}
	fn(r)
}

// SetExitRouter registers the exit router. Passing nil clears it.
func SetExitRouter(fn ExitRouter) {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	exitRouter = fn
}

// SetEntryGuard registers the entry guard. Passing nil clears it.
func SetEntryGuard(fn EntryGuard) {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	entryGuard = fn
}

// SetPrivateRoomCheck registers the private-room check. Passing nil clears it.
func SetPrivateRoomCheck(fn PrivateRoomCheck) {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	privateRoomCheck = fn
}

// RouteExit asks the registered router about one player taking one exit.
// handled is false when no router is registered or the exit is ordinary.
func RouteExit(userId int, fromRoomId int, exitName string) (ExitRoute, bool) {
	routingHooksMu.RLock()
	fn := exitRouter
	routingHooksMu.RUnlock()
	if fn == nil || exitName == `` {
		return ExitRoute{}, false
	}
	return fn(userId, fromRoomId, exitName)
}

// IsRoutedExit reports whether exitName in fromRoomId is handled by the
// router for this player (whether or not they would be let through).
func IsRoutedExit(userId int, fromRoomId int, exitName string) bool {
	_, handled := RouteExit(userId, fromRoomId, exitName)
	return handled
}

// checkEntry asks the registered guard. With no guard every entry is allowed.
func checkEntry(userId int, toRoomId int) (allowed bool, redirectRoomId int, refusal string) {
	routingHooksMu.RLock()
	fn := entryGuard
	routingHooksMu.RUnlock()
	if fn == nil {
		return true, 0, ``
	}
	return fn(userId, toRoomId)
}

// IsPrivateRoom reports whether roomId belongs to a player.
func IsPrivateRoom(roomId int) bool {
	routingHooksMu.RLock()
	fn := privateRoomCheck
	routingHooksMu.RUnlock()
	if fn == nil {
		return false
	}
	return fn(roomId)
}
