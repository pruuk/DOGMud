package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// #254: "You attack the darkness!" covered two different failures. A bare
// `attack` with nobody to pick up reads one line; a typed name that matches
// nothing reads the owner's line (2026-08-15) and never echoes the name, so a
// hider's presence and a dark room give nothing away.
func TestAttack_NoArgumentAndNoFoe_SaysNothingToAttack(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "There is nothing here to attack.")
	require.NotContains(t, out, "darkness")
}

func TestAttack_UnmatchedName_SaysNothingByThatName(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("zzyzx", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "Nothing by that name is in this room.")
	require.NotContains(t, out, "zzyzx", "the typed name is never echoed")
	require.NotContains(t, out, "darkness")
}
