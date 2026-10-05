package items

import (
	"sort"
	"sync"
)

// Fixture kinds (ItemSpec.Fixture, lighting 5e Rule 10).
const (
	FixtureLight    = `light`
	FixtureDarkness = `darkness`
)

// HasBehavior reports whether the item's TEMPLATE names a behaviour tree. A
// tree belongs to the template (spec X4), so an instance with an override
// spec still runs its template's tree.
func (i Item) HasBehavior() bool {
	spec := GetItemSpec(i.ItemId)
	return spec != nil && spec.Behavior != ``
}

// IsFixture reports whether the item's template fixes it to a room's floor.
// Every floor-removal path refuses a fixture (actions.ErrFixture).
func (i Item) IsFixture() bool {
	spec := GetItemSpec(i.ItemId)
	return spec != nil && spec.Fixture != ``
}

// The item tick's holder index (lighting 5e Rule 5). It holds HOLDERS, not
// items: a mob instance or a room that may hold an item with a behaviour
// tree. The tick (internal/hooks.ItemRoundTick) visits only these, plus every
// online player, so no round walks the world; a visited holder with no treed
// item left is dropped.
//
// A mob enters at spawn and whenever it stores or wears a treed item
// (internal/characters.(*Character).IndexTreedItem). A room enters when a
// treed item lands on its floor (Room.AddItem, Room.Prepare's spawn append)
// and when it loads into memory holding one.
var (
	holderMu    sync.Mutex
	mobHolders  = map[int]struct{}{}
	roomHolders = map[int]struct{}{}

	// OnRoomHolderIndexed, when set, is called each time a room ENTERS the
	// index, outside the lock. internal/hooks sets it to evaluate the room's
	// treed floor items at once, so a fixture is never dark for a round on
	// a room's first visit. Nil in a unit test that does not set it.
	OnRoomHolderIndexed func(roomId int)
)

// IndexMobHolder enters a mob instance in the index. Ids below 1 are ignored.
func IndexMobHolder(instanceId int) {
	if instanceId < 1 {
		return
	}
	holderMu.Lock()
	mobHolders[instanceId] = struct{}{}
	holderMu.Unlock()
}

// IndexRoomHolder enters a room in the index, and reports a room that was
// not already in it to OnRoomHolderIndexed.
func IndexRoomHolder(roomId int) {
	holderMu.Lock()
	_, already := roomHolders[roomId]
	roomHolders[roomId] = struct{}{}
	holderMu.Unlock()
	if !already && OnRoomHolderIndexed != nil {
		OnRoomHolderIndexed(roomId)
	}
}

// DropMobHolder removes a mob instance from the index.
func DropMobHolder(instanceId int) {
	holderMu.Lock()
	delete(mobHolders, instanceId)
	holderMu.Unlock()
}

// DropRoomHolder removes a room from the index.
func DropRoomHolder(roomId int) {
	holderMu.Lock()
	delete(roomHolders, roomId)
	holderMu.Unlock()
}

// MobHolders returns the indexed mob instance ids in ascending order.
func MobHolders() []int {
	holderMu.Lock()
	defer holderMu.Unlock()
	return sortedKeys(mobHolders)
}

// RoomHolders returns the indexed room ids in ascending order.
func RoomHolders() []int {
	holderMu.Lock()
	defer holderMu.Unlock()
	return sortedKeys(roomHolders)
}

func sortedKeys(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// ResetHolderIndexForTest empties the index and returns a restore func.
func ResetHolderIndexForTest() func() {
	holderMu.Lock()
	origMobs, origRooms := mobHolders, roomHolders
	mobHolders, roomHolders = map[int]struct{}{}, map[int]struct{}{}
	holderMu.Unlock()
	return func() {
		holderMu.Lock()
		mobHolders, roomHolders = origMobs, origRooms
		holderMu.Unlock()
	}
}
