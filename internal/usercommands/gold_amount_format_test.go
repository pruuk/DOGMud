package usercommands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDropGoldShowsThousandsSeparator pins #405: `drop 25000 gold` printed
// "You drop 25000 gold on the floor." Gold amounts read with separators,
// the same as quest and dialogue gold lines (util.FormatNumber).
func TestDropGoldShowsThousandsSeparator(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	savedGold, savedRoomGold := user.Character.Gold, room.Gold
	defer func() { user.Character.Gold, room.Gold = savedGold, savedRoomGold }()

	user.Character.Gold = 30000
	craftPlainLines(user.UserId)

	handled, err := Drop("25000 gold", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	joined := strings.Join(craftPlainLines(user.UserId), "\n")
	require.Contains(t, joined, "You drop 25,000 gold on the floor.", "got %q", joined)
}
