package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// When the NPC leaves the room (a schedule move, a wander, a despawn or a
// death: every route ends in Room.RemoveMob), the talk menu closes at once
// with a line saying the conversation ended because they left. Before, the
// menu stayed open and the next number read "You break off your talk ..."
// (playtest seat B, #375).
func TestTalk_NpcLeavingClosesTheMenuAtOnce(t *testing.T) {
	for _, route := range []string{"move", "removed"} {
		t.Run(route, func(t *testing.T) {
			user, mob, done := talkFixture(t)
			defer done()
			room := mustRoom(t, user)
			conversation.Talk(user, room, mob)
			require.True(t, user.HasOpenMenu())
			drained(1)

			switch route {
			case "move": // walking, a schedule step and a flee all relocate
				dest := rooms.LoadRoom(2)
				require.NotNil(t, dest)
				actions.RelocateMob(mob, room, "north", dest, false)
			case "removed": // a death or a despawn takes the mob out of the room
				room.RemoveMob(mob.InstanceId)
			}

			require.False(t, user.HasOpenMenu(), "the menu is still open after the NPC left")
			got := deliveredInOrder(t)
			ends := indexOf(got, "Your talk with Skeleton ends because they left.")
			require.GreaterOrEqual(t, ends, 0, "%q", got)
			require.Equal(t, -1, indexOf(got, "break off"), "%q", got)
			if route == "move" {
				require.Greater(t, ends, indexOf(got, "leaves"), "the line came before the NPC was seen leaving: %q", got)
			}
		})
	}
}

// A different NPC leaving leaves the menu open.
func TestTalk_AnotherNpcLeavingKeepsTheMenu(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	room := mustRoom(t, user)
	conversation.Talk(user, room, mob)
	drained(1)
	room.RemoveMob(mob.InstanceId + 1)
	require.True(t, user.HasOpenMenu())
	require.NotContains(t, drained(1), "ends because they left")
}
