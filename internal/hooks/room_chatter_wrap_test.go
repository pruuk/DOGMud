package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A shout from next door and the companion's `ask` line arrive unrendered on
// the RoomId branch, so that branch wraps them for each reader. Other RoomId
// messages keep their own lines.
func TestRoomMessageTextFor_WrapsOnlyChatter(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	reader := users.GetByUserId(1)
	reader.LineWidth = 40
	long := `Someone shouts from the <ansi fg="exit">north</ansi> direction, "<ansi fg="yellow">THE BRIDGE IS OUT AND THE RIVER IS RISING AND THE EAST ROAD WILL BE UNDER WATER</ansi>"`

	chatter := roomMessageTextFor(events.Message{RoomId: 1, Text: long + "\n", IsCommunication: true}, reader)
	lines := strings.Split(strings.TrimRight(chatter, "\n"), "\n")
	require.Greater(t, len(lines), 2, "%q", chatter)
	for _, line := range lines {
		require.LessOrEqual(t, util.VisibleWidth(line), 40, "%q", line)
	}
	require.True(t, strings.HasSuffix(chatter, "\n"), "the message still ends its line")

	plain := long + "\n"
	require.Equal(t, plain, roomMessageTextFor(events.Message{RoomId: 1, Text: plain}, reader))
}
