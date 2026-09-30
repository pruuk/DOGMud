package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A user's queued Input is drained by user id; a mob's Input and another
// user's stay queued.
func TestDrainQueuedUserInputsForTest(t *testing.T) {
	DrainQueuedUserInputsForTest(7301)
	DrainQueuedUserInputsForTest(7302)
	DrainQueuedInputsForTest(7301)

	AddToQueue(Input{UserId: 7301, InputText: "north"})
	AddToQueue(Input{UserId: 7302, InputText: "south"})
	AddToQueue(Input{MobInstanceId: 7301, InputText: "east"})

	require.Equal(t, []string{"north"}, DrainQueuedUserInputsForTest(7301))
	require.Empty(t, DrainQueuedUserInputsForTest(7301), "a drained input is gone")
	require.Equal(t, []string{"south"}, DrainQueuedUserInputsForTest(7302))
	require.Equal(t, []string{"east"}, DrainQueuedInputsForTest(7301), "a mob input with the same id is not a user input")
}
