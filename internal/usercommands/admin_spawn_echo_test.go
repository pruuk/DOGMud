package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #449: the admin spawn echoes ran 84 to 93 columns ("You wave your hands
// around and Goblin Scout appears in the air and falls to the ground.").
// They now read "You wave your hands and X appears.", and the room's line
// matches.
func TestSpawn_EchoesAreShortAndMatchTheRoomLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	admin := users.GetByUserId(1)
	admin.Role = users.RoleAdmin
	room := rooms.LoadRoom(1)

	for _, tc := range []struct {
		rest, self, watcher string
	}{
		{"gold 7", "You wave your hands and 7 gold appears.", "Aliceia waves their hands and 7 gold appears."},
		{"container crate", "You wave your hands and crate appears.", "Aliceia waves their hands and crate appears."},
	} {
		craftPlainLines(1)
		craftPlainLines(2)
		_, err := Spawn(tc.rest, admin, room, 0)
		require.NoError(t, err)

		self := craftPlainLines(1)
		require.Contains(t, self, tc.self, "spawn %q", tc.rest)
		for _, line := range self {
			require.LessOrEqual(t, util.VisibleWidth(line), 80, "spawn %q echo over 80 columns: %q", tc.rest, line)
		}
		require.Contains(t, craftPlainLines(2), tc.watcher, "spawn %q room line", tc.rest)
	}
}
