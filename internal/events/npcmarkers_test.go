package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A hand-in refreshes twice (the topic, then the token landing); one queued
// refresh per player is enough.
func TestNpcMarkersChanged_TwoForOneUserMergeIntoOne(t *testing.T) {
	DrainQueuedNpcMarkersChangedForTest(0)
	AddToQueue(NpcMarkersChanged{UserId: 61})
	AddToQueue(NpcMarkersChanged{UserId: 61})
	AddToQueue(NpcMarkersChanged{UserId: 62})

	require.Equal(t, []NpcMarkersChanged{{UserId: 61}}, DrainQueuedNpcMarkersChangedForTest(61))
	require.Equal(t, []NpcMarkersChanged{{UserId: 62}}, DrainQueuedNpcMarkersChangedForTest(0))
}

// Draining frees the unique slot, as ProcessEvents does, so a later refresh
// for the same player queues again.
func TestNpcMarkersChanged_DrainFreesTheUniqueSlot(t *testing.T) {
	DrainQueuedNpcMarkersChangedForTest(0)
	AddToQueue(NpcMarkersChanged{UserId: 63})
	require.Len(t, DrainQueuedNpcMarkersChangedForTest(63), 1)
	AddToQueue(NpcMarkersChanged{UserId: 63})
	require.Len(t, DrainQueuedNpcMarkersChangedForTest(63), 1)
}
