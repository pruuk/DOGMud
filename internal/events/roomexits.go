package events

import (
	"container/heap"
	"strconv"
)

// RoomExitsChanged says what a player's movement buttons show may have
// changed outside the once-a-round check (#781): a dark mark was recorded.
// modules/gmcp answers with Room.Info.
type RoomExitsChanged struct {
	UserId int
}

func (p RoomExitsChanged) Type() string { return `RoomExitsChanged` }

// UniqueID merges the refreshes queued for one player before the queue runs.
func (p RoomExitsChanged) UniqueID() string {
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
