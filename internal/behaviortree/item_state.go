package behaviortree

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Item tree state (lighting 5e, Rule 4): one BehaviorState per item
// INSTANCE, keyed by Item.UUID, in memory only. A UUID is minted afresh on
// every load (restart, login, mob instance restore, buy), so this state
// resets with it: cooldowns start over and a schedule recomputes from the
// hour. It is the room_state.go shape plus the round each entry was last
// visited, so the item tick can evict what it no longer reaches.
type itemStateEntry struct {
	state    *BehaviorState
	lastSeen uint64
}

var (
	itemStateMu sync.RWMutex
	itemStates  = make(map[uuid.UUID]*itemStateEntry)
)

// EnsureItemBTreeState returns the item instance's state, creating it on
// first use, and marks it visited this round.
func EnsureItemBTreeState(id uuid.UUID) *BehaviorState {
	round := util.GetRoundCount()
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	e, ok := itemStates[id]
	if !ok {
		e = &itemStateEntry{state: NewBehaviorState()}
		itemStates[id] = e
	}
	e.lastSeen = round
	return e.state
}

// EvictItemBTreeState drops one item instance's state.
func EvictItemBTreeState(id uuid.UUID) {
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	delete(itemStates, id)
}

// EvictUnseenItemBTreeStates drops the state of every item the tick did not
// visit this round, so state follows items the tick still reaches. An item
// handed from a mob to a player is visited in the same or the next tick and
// keeps its state. Returns how many were dropped.
func EvictUnseenItemBTreeStates(round uint64) int {
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	n := 0
	for id, e := range itemStates {
		if e.lastSeen < round {
			delete(itemStates, id)
			n++
		}
	}
	return n
}

// ItemBTreeStateForTest returns an item's state without marking it visited,
// or nil when it has none.
func ItemBTreeStateForTest(id uuid.UUID) *BehaviorState {
	itemStateMu.RLock()
	defer itemStateMu.RUnlock()
	if e, ok := itemStates[id]; ok {
		return e.state
	}
	return nil
}

// ResetItemBTreeStatesForTest empties the item state map and returns a
// restore func.
func ResetItemBTreeStatesForTest() func() {
	itemStateMu.Lock()
	orig := itemStates
	itemStates = make(map[uuid.UUID]*itemStateEntry)
	itemStateMu.Unlock()
	return func() {
		itemStateMu.Lock()
		itemStates = orig
		itemStateMu.Unlock()
	}
}
