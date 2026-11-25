package gmcp

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Room.Info marks a pick combination only at the lock's current rotation
// (#781), for a container and for an exit alike: re-locking (SetLocked)
// rotates the lock and retires the combination learned before.
func TestRoomInfo_PickComboFollowsTheLocksRotation(t *testing.T) {
	viewer := roomSightFixture(t, 60)
	room := rooms.LoadRoom(9720)
	room.Containers = map[string]rooms.Container{"chest": {Lock: gamelock.Lock{Difficulty: 4}}}
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 4}
	room.Exits["north"] = north

	const chestId, northId = `9720-chest`, `9720-north`
	t.Cleanup(util.RebootLocksUntilRotatedForTest(4, chestId, northId))
	viewer.Character.SetKey(chestId, util.LockSequence(chestId, 4, 0))
	viewer.Character.SetKey(northId, util.LockSequence(northId, 4, 0))

	chestCombo := func() bool {
		cs := roomInfoFor(t, viewer).Contents.Containers
		require.Len(t, cs, 1)
		return cs[0].HasPickCombo
	}
	northCombo := func() bool {
		return slices.Contains(roomInfoFor(t, viewer).ExitsV2["north"].Details, `player_has_pick_combo`)
	}
	require.True(t, chestCombo(), "premise: the chest's rotation 0 combination counts")
	require.True(t, northCombo(), "premise: north's rotation 0 combination counts")

	chest := room.Containers["chest"]
	chest.Lock.SetLocked()
	room.Containers["chest"] = chest
	room.SetExitLock("north", true)
	require.False(t, chestCombo(), "the chest's combination from before the relock still counts")
	require.False(t, northCombo(), "north's combination from before the relock still counts")

	viewer.Character.SetKey(chestId, util.LockSequence(chestId, 4, 1))
	viewer.Character.SetKey(northId, util.LockSequence(northId, 4, 1))
	require.True(t, chestCombo(), "the chest's combination at the current rotation does not count")
	require.True(t, northCombo(), "north's combination at the current rotation does not count")
}
