package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/stretchr/testify/require"
)

// Movement parity 4b, owner ruling 2 on the open questions: walking into a
// locked door you cannot open used to cost the step's action points and
// stamina anyway, because the charge ran before the lock check. It now costs
// nothing.
func TestGo_ALockedDoorCostsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	orig, ok := room.Exits["north"]
	require.True(t, ok, "fixture premise: room 1 has a north exit")
	locked := orig
	locked.Lock = gamelock.Lock{Difficulty: 10}
	room.Exits["north"] = locked
	t.Cleanup(func() { room.Exits["north"] = orig })
	require.True(t, room.Exits["north"].Lock.IsLocked(), "fixture premise: the north exit is locked")

	stamina := user.Character.Stamina

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, user.Character.RoomId, "the door stayed locked")
	require.Equal(t, stamina, user.Character.Stamina, "a locked door costs no stamina")
}
