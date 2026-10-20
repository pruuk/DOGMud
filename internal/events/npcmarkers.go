package events

import (
	"container/heap"
	"strconv"
)

// NpcMarkersChanged says the talk markers a player sees on NPCs may have
// changed (a quest token landed, a topic was heard). modules/gmcp answers
// with Room.Info.Contents.Npcs (#375 R9).
type NpcMarkersChanged struct {
	UserId int
}

func (p NpcMarkersChanged) Type() string { return `NpcMarkersChanged` }

// UniqueID merges the refreshes queued for one player before the queue runs
// (a hand-in refreshes for the topic and again when its token lands), the
// RedrawPrompt pattern.
func (p NpcMarkersChanged) UniqueID() string {
	return `NpcMarkersChanged-` + strconv.Itoa(p.UserId)
}

// DrainQueuedNpcMarkersChangedForTest removes the NpcMarkersChanged events for
// userId from the global queue and returns them. Pass 0 to drain every one.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedNpcMarkersChangedForTest(userId int) []NpcMarkersChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []NpcMarkersChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		m, ok := pe.event.(NpcMarkersChanged)
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
