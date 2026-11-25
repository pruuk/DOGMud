package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// The keyring view pads a half-finished pick with one "?" per pin still to
// find, read from the lock the solution is for: a container's own lock, by
// the whole "<roomId>-<name>" lock id picklock learned it under (#781).
func TestKeyRing_APartialPickShowsTheLocksRemainingPins(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	room := tableRoom(t, "Vault", 3)
	room.Containers = map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 6}}}
	u := tableUsers(t, 98701, "Screenly")[0]
	u.Character.KeyRing = map[string]string{fmt.Sprintf("%d-lockbox", tableRoomId): "U"}
	u.Character.KeyRingBoot = util.BootID() // a pick started this boot

	out := tableRun(t, u, 80, func() { _, _ = KeyRing("", u, nil, 0) })
	require.Contains(t, out, "lockbox", "premise: the lockbox's pick is listed")
	require.Equal(t, 5, strings.Count(out, "?"), "a one-pin pick on a six-pin lockbox should show five pins to find:\n%s", out)
}
