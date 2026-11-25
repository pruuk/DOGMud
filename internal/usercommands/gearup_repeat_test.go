package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/require"
)

// gearup queues one engine wear per item, outside the player's pending
// client commands. A second gearup while those still wait is refused, so a
// player repeating it cannot pile up a pack's worth of wears each time; once
// they have run, gearup works again.
func TestGearup_RefusedWhileItsWearsStillWait(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.StoreItem(items.Item{ItemId: 10001})
	user.Character.StoreItem(items.Item{ItemId: 20001})
	events.DrainQueuedUserInputsForTest(user.UserId)
	events.DrainQueuedMessagesForTest(user.UserId)
	t.Cleanup(func() {
		events.DrainQueuedUserInputsForTest(user.UserId)
		events.DrainQueuedMessagesForTest(user.UserId)
	})

	Gearup("", user, room, events.CmdFromClient)
	wears := events.QueuedEngineInputs(user.UserId, gearupWearPrefix)
	require.Equal(t, 2, wears, "fixture: two wears queued")

	Gearup("", user, room, events.CmdFromClient)
	require.Equal(t, wears, events.QueuedEngineInputs(user.UserId, gearupWearPrefix), "the repeat queues nothing")
	lines := events.DrainQueuedMessagesForTest(user.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], refusal.StillGearingUpLine)

	events.DrainQueuedUserInputsForTest(user.UserId) // the wears ran
	Gearup("", user, room, events.CmdFromClient)
	require.Equal(t, 2, events.QueuedEngineInputs(user.UserId, gearupWearPrefix), "gearup works again")
}

// A player's own typed "wear !id" is a client command, not gearup's: it does
// not block gearup.
func TestQueuedEngineInputs_IgnoresClientCommands(t *testing.T) {
	events.DrainQueuedUserInputsForTest(98730)
	t.Cleanup(func() { events.DrainQueuedUserInputsForTest(98730) })
	events.AddToQueue(events.Input{UserId: 98730, InputText: "wear !5", Flags: events.CmdFromClient})
	require.Equal(t, 0, events.QueuedEngineInputs(98730, gearupWearPrefix))
	events.AddToQueue(events.Input{UserId: 98730, InputText: "wear !5"})
	require.Equal(t, 1, events.QueuedEngineInputs(98730, gearupWearPrefix))
}
