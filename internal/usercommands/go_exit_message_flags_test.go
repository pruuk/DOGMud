package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// An exit with a message delays the step: go re-runs itself a moment later.
// That re-run is the engine's, not the client's, so it drops CmdTyped (#375)
// and CmdFromClient: it neither counts against the player's pending inputs
// nor waits for the client pace. A touch button's step drops CmdButton too.
func TestGo_ExitMessageReRunIsNotAClientCommand(t *testing.T) {
	_, mover, cave := goShopWakeScene(t)
	south := cave.Exits["south"]
	south.ExitMessage = "You squeeze through the gap."
	cave.Exits["south"] = south
	events.DrainQueuedByTypeForTest(`Input`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`Input`) })

	handled, err := Go("south", mover, cave, events.CmdTyped|events.CmdFromClient|events.CmdButton)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, mover.Character.RoomId, "fixture: the step waits for the re-run")

	var rerun []events.Input
	for _, e := range events.DrainQueuedByTypeForTest(`Input`) {
		if in := e.(events.Input); in.UserId == mover.UserId {
			rerun = append(rerun, in)
		}
	}
	require.Len(t, rerun, 1)
	require.True(t, rerun[0].Flags.Has(events.CmdIsRequeue), "fixture: this is the delayed re-run")
	require.False(t, rerun[0].Flags.Has(events.CmdFromClient))
	require.False(t, rerun[0].Flags.Has(events.CmdTyped))
	// A press re-runs as a typed step does: a prompt opened in between does
	// not drop it (world.processInput drops only a CmdButton line).
	require.False(t, rerun[0].Flags.Has(events.CmdButton))
}
