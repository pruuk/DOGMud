package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #286: consider rated Drillmaster Vorn "An even contest" while attack
// refused him. Owner call 2026-10-10: say "You can't fight X." instead.
func TestConsider_ProtectedMobSaysYouCantFightIt(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	oldLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = oldLamp })

	const vornId, championId, ruffianId = 786, 787, 788
	vorn := considerTestMob(vornId, "Drillmaster Vorn", room.RoomId)
	vorn.NonCombatant = true
	champion := considerTestMob(championId, "Arena Champion", room.RoomId)
	champion.PlayerAttackImmune = true
	ruffian := considerTestMob(ruffianId, "Dock Ruffian", room.RoomId)
	for id, m := range map[int]*mobs.Mob{vornId: vorn, championId: champion, ruffianId: ruffian} {
		mobs.SetInstanceForTest(id, m)
		room.AddMob(id)
		t.Cleanup(func() {
			room.RemoveMob(id)
			mobs.SetInstanceForTest(id, nil)
		})
	}

	for _, tc := range []struct{ arg, name string }{
		{"drillmaster vorn", "Drillmaster Vorn"},
		{"arena champion", "Arena Champion"},
	} {
		events.DrainQueuedMessagesForTest(user.UserId)
		handled, err := Consider(tc.arg, user, room, 0)
		require.True(t, handled)
		require.NoError(t, err)
		out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
		assert.Contains(t, out, "You can't fight", tc.arg)
		assert.Contains(t, out, tc.name)
		assert.NotContains(t, out, "Your instincts tell you", tc.arg)
	}

	// A mob you may fight is still weighed up.
	events.DrainQueuedMessagesForTest(user.UserId)
	Consider("dock ruffian", user, room, 0)
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), "Your instincts tell you")
}
