package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #781 Task 11b: refreshes for one player merge, a find with finds and a dark
// mark with marks. The queue keeps the first of a merged pair, so a mark
// queued first must not swallow a find (and with it the find's Zone.Map).
func TestRoomExitsChanged_FindsAndMarksMergeApartPerPlayer(t *testing.T) {
	DrainQueuedRoomExitsChangedForTest(0)
	t.Cleanup(func() { DrainQueuedRoomExitsChangedForTest(0) })
	AddToQueue(RoomExitsChanged{UserId: 71})
	AddToQueue(RoomExitsChanged{UserId: 71, SecretExitFound: true})
	AddToQueue(RoomExitsChanged{UserId: 71, SecretExitFound: true})
	AddToQueue(RoomExitsChanged{UserId: 71})
	AddToQueue(RoomExitsChanged{UserId: 72, SecretExitFound: true})

	require.ElementsMatch(t,
		[]RoomExitsChanged{{UserId: 71}, {UserId: 71, SecretExitFound: true}},
		DrainQueuedRoomExitsChangedForTest(71))
	require.Equal(t, []RoomExitsChanged{{UserId: 72, SecretExitFound: true}}, DrainQueuedRoomExitsChangedForTest(0))
}

// IsQueued answers by the merge key: pending until drained (or popped), a
// mark apart from a find, and never for an event that does not merge.
func TestIsQueued_ReadsTheMergeKey(t *testing.T) {
	DrainQueuedRoomExitsChangedForTest(0)
	t.Cleanup(func() { DrainQueuedRoomExitsChangedForTest(0) })
	require.False(t, IsQueued(RoomExitsChanged{UserId: 73}))
	AddToQueue(RoomExitsChanged{UserId: 73})
	require.True(t, IsQueued(RoomExitsChanged{UserId: 73}))
	require.False(t, IsQueued(RoomExitsChanged{UserId: 73, SecretExitFound: true}))
	require.False(t, IsQueued(RoomExitsChanged{UserId: 74}))
	DrainQueuedRoomExitsChangedForTest(73)
	require.False(t, IsQueued(RoomExitsChanged{UserId: 73}))
	require.False(t, IsQueued(Message{UserId: 73}), "a message never merges")
}
