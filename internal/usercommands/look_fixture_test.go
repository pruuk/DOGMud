package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R9 and Rule 10: a fixture is part of the room. The room look gives it a
// line of its own after the description, lit or unlit, and "On the Ground"
// leaves it out; `look <name>` still finds it.
func TestLookShowsAFixtureAsPartOfTheRoom(t *testing.T) {
	user, room := seedFixtureRoom(t)
	useDogmudTemplates(t)
	t.Cleanup(itemlight.ResetForTest())

	_, err := Look("", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, `The <ansi fg="item">Arch Lantern</ansi> is unlit.`)
	ground := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "On the Ground") {
			ground = line
		}
	}
	assert.Contains(t, ground, "Grey Pebble", "the pebble is still on the ground")
	assert.NotContains(t, ground, "Arch Lantern", "the lantern is listed on the ground")

	lantern, ok := room.FindOnFloor("lantern", false)
	require.True(t, ok)
	itemlight.Set(room.RoomId, lantern.UUID, itemlight.Light, 52)
	Look("", user, room, 0)
	assert.Contains(t, sentTo(user), `The <ansi fg="item">Arch Lantern</ansi> is lit.`)

	Look("lantern", user, room, 0)
	assert.Contains(t, sentTo(user), `You look at the <ansi fg="item">Arch Lantern</ansi> here:`)
}
