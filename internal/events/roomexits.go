package events

import (
	"container/heap"
	"strconv"
)

// RoomExitsChanged says what a player's movement buttons show may have
// changed outside the once-a-round check (#781): a dark mark was recorded, the
// player pressed a button whose id was stale (modules/gmcp pressButton), or
// the player found a secret exit (SecretExitFound, queued by
// characters.Character.LearnSecretExit on a new find only). modules/gmcp
// answers with Room.Info, and on a find with Zone.Map too, since the map
// draws a secret exit only once it is found.
type RoomExitsChanged struct {
	UserId          int
	SecretExitFound bool
}

func (p RoomExitsChanged) Type() string { return `RoomExitsChanged` }

// UniqueID merges the refreshes queued for one player before the queue runs.
// A find keys apart from a dark mark: the queue keeps the first of a merged
// pair, and a mark queued first must not swallow the find's Zone.Map.
func (p RoomExitsChanged) UniqueID() string {
	if p.SecretExitFound {
		return `RoomExitsChanged-found-` + strconv.Itoa(p.UserId)
	}
	return `RoomExitsChanged-` + strconv.Itoa(p.UserId)
}

// DrainQueuedRoomExitsChangedForTest removes the RoomExitsChanged events for
// userId from the global queue and returns them. Pass 0 to drain every one.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedRoomExitsChangedForTest(userId int) []RoomExitsChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []RoomExitsChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		m, ok := pe.event.(RoomExitsChanged)
		if ok && (userId == 0 || m.UserId == userId) {
			found = append(found, m)
			// Free the unique slot as ProcessEvents does, or the next
			// refresh for this player would be dropped as a duplicate.
			delete(uniqueMap, m.UniqueID())
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}
