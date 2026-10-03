package rooms

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/util"
)

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
// boot the subsystem see ordinary room behaviour. The exit router and the
// entry guard also accept additional hooks (AddExitRouter, AddEntryGuard),
// consulted after the primary one, so a second subsystem (rifts) can take
// part without displacing housing.

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
	// PickRefusal, when set, is what `picklock` says about this exit. Empty
	// keeps picklock's own wording for a routed exit (a housing door).
	PickRefusal string
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

// RoomSaveHook runs on a loaded room just before a bulk save (the autosave
// prepare and SaveAllRooms) reads it, so state kept outside the room files
// can be brought up to date from the live room first. It must not load
// other rooms.
type RoomSaveHook func(r *Room)

var (
	routingHooksMu   sync.RWMutex
	exitRouter       ExitRouter
	entryGuard       EntryGuard
	privateRoomCheck PrivateRoomCheck
	roomOverlay      RoomOverlay
	roomSaveHook     RoomSaveHook

	extraExitRouters []ExitRouter
	extraEntryGuards []EntryGuard
)

// SetRoomSaveHook registers the room save hook. Passing nil clears it.
func SetRoomSaveHook(fn RoomSaveHook) {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	roomSaveHook = fn
}

// runRoomSaveHook runs the registered save hook, if any, on a live room.
func runRoomSaveHook(r *Room) {
	routingHooksMu.RLock()
	fn := roomSaveHook
	routingHooksMu.RUnlock()
	if fn == nil || r == nil {
		return
	}
	fn(r)
}

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

// GetRandomExitFor is GetRandomExit for one traveller (userId zero for a mob):
// a routed exit counts only when its router lets them through, and then
// stands for the room the router names. Flee uses it so that a routed door
// that refuses a walker refuses a fleer too.
func (r *Room) GetRandomExitFor(userId int) (exitName string, roomId int) {
	candidates := map[string]int{}
	add := func(name string, info exit.RoomExit) {
		if info.Secret || info.Lock.IsLocked() {
			return
		}
		if route, routed := RouteExit(userId, r.RoomId, name); routed {
			if route.RoomId == 0 {
				return
			}
			candidates[name] = route.RoomId
			return
		}
		candidates[name] = info.RoomId
	}
	for name, info := range r.Exits {
		add(name, info)
	}
	for mut := range r.ActiveMutators {
		for name, info := range mut.GetSpec().Exits {
			add(name, info)
		}
	}
	if len(candidates) == 0 {
		return ``, 0
	}
	pick := util.Rand(len(candidates))
	for name, id := range candidates {
		if pick == 0 {
			return name, id
		}
		pick--
	}
	return ``, 0
}

// RouteExit asks the registered routers about one player taking one exit: the
// primary router (SetExitRouter) first, then each added one (AddExitRouter) in
// the order they were added. The first router that handles the exit decides.
// handled is false when no router claims the exit.
func RouteExit(userId int, fromRoomId int, exitName string) (ExitRoute, bool) {
	if exitName == `` {
		return ExitRoute{}, false
	}
	routingHooksMu.RLock()
	fns := make([]ExitRouter, 0, 1+len(extraExitRouters))
	if exitRouter != nil {
		fns = append(fns, exitRouter)
	}
	fns = append(fns, extraExitRouters...)
	routingHooksMu.RUnlock()
	for _, fn := range fns {
		if route, handled := fn(userId, fromRoomId, exitName); handled {
			return route, true
		}
	}
	return ExitRoute{}, false
}

// IsRoutedExit reports whether exitName in fromRoomId is handled by the
// router for this player (whether or not they would be let through).
func IsRoutedExit(userId int, fromRoomId int, exitName string) bool {
	_, handled := RouteExit(userId, fromRoomId, exitName)
	return handled
}

// checkEntry asks the registered guards: the primary (SetEntryGuard) first,
// then each added one (AddEntryGuard). The first refusal decides. With no
// guard every entry is allowed.
func checkEntry(userId int, toRoomId int) (allowed bool, redirectRoomId int, refusal string) {
	routingHooksMu.RLock()
	fns := make([]EntryGuard, 0, 1+len(extraEntryGuards))
	if entryGuard != nil {
		fns = append(fns, entryGuard)
	}
	fns = append(fns, extraEntryGuards...)
	routingHooksMu.RUnlock()
	for _, fn := range fns {
		if ok, redirect, why := fn(userId, toRoomId); !ok {
			return false, redirect, why
		}
	}
	return true, 0, ``
}

// AddExitRouter registers an additional exit router, consulted after the
// primary one. It exists so that more than one subsystem can route exits
// (housing owns the primary slot; rifts adds itself here). Routers must be
// free of side effects: look, picklock and unlock ask them too, not only go.
func AddExitRouter(fn ExitRouter) {
	if fn == nil {
		return
	}
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	extraExitRouters = append(extraExitRouters, fn)
}

// AddEntryGuard registers an additional entry guard, consulted after the
// primary one.
func AddEntryGuard(fn EntryGuard) {
	if fn == nil {
		return
	}
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	extraEntryGuards = append(extraEntryGuards, fn)
}

// ClearAddedRoutingHooks drops every router and guard added with
// AddExitRouter / AddEntryGuard. For tests.
func ClearAddedRoutingHooks() {
	routingHooksMu.Lock()
	defer routingHooksMu.Unlock()
	extraExitRouters = nil
	extraEntryGuards = nil
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

// fleeRouting is true while a flee is choosing its exit (WhileFleeing). All
// routing runs on the game loop, so a plain flag is enough.
var fleeRouting bool

// WhileFleeing runs fn with FleeRouting reporting true, so an exit router
// asked inside it knows the traveller is fleeing rather than walking. A
// router may let a flee through a door it shuts to walkers (a rift room held
// shut by the monsters in it: fleeing is the way out, at a price).
func WhileFleeing(fn func()) {
	prev := fleeRouting
	fleeRouting = true
	defer func() { fleeRouting = prev }()
	fn()
}

// FleeRouting reports whether the router being asked is deciding a flee.
func FleeRouting() bool { return fleeRouting }

// NoRecall reports whether roomId keeps its occupants in (temp data
// `allow_recall` false, as every rift room has): recall, the tutorial door,
// fold anchors and scripted teleports out of it are refused. Death and
// admin moves are not.
func NoRecall(roomId int) bool {
	room := LoadRoom(roomId)
	if room == nil {
		return false
	}
	allowed, ok := room.GetTempData(`allow_recall`).(bool)
	return ok && !allowed
}
