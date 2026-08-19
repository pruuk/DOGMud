package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDrainQueuedPlayerAttackedMobsForTest(t *testing.T) {
	DrainQueuedPlayerAttackedMobsForTest(0)
	AddToQueue(PlayerAttackedMob{UserId: 41, MobInstanceId: 301})
	AddToQueue(PlayerAttackedMob{UserId: 42, MobInstanceId: 302})

	require.Equal(t, []PlayerAttackedMob{{UserId: 41, MobInstanceId: 301}},
		DrainQueuedPlayerAttackedMobsForTest(41))
	require.Equal(t, []PlayerAttackedMob{{UserId: 42, MobInstanceId: 302}},
		DrainQueuedPlayerAttackedMobsForTest(0))
}
