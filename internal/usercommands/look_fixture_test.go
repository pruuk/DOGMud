package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
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

// seedArchRoom adds a room noun "arch" to the fixture room, as room 4111 has
// beside its Arch Lantern.
func seedArchRoom(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := seedFixtureRoom(t)
	useDogmudTemplates(t)
	orig := room.Nouns
	room.Nouns = map[string]string{"arch": "A simple timber arch."}
	t.Cleanup(func() { room.Nouns = orig })
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

// The full name of a floor fixture beats the room noun its first word
// matches: `look arch lantern` shows the lantern, not the arch.
func TestLookFullFixtureNameBeatsAFirstWordRoomNoun(t *testing.T) {
	user, room := seedArchRoom(t)
	Look("arch lantern", user, room, 0)
	out := sentTo(user)
	assert.Contains(t, out, `You look at the <ansi fg="item">Arch Lantern</ansi> here:`)
	assert.NotContains(t, out, "You look at the arch")
}

// The room noun is still reachable by its own word.
func TestLookRoomNounStillWinsForItsOwnWord(t *testing.T) {
	user, room := seedArchRoom(t)
	Look("arch", user, room, 0)
	out := sentTo(user)
	assert.Contains(t, out, `<ansi fg="noun">arch</ansi>:`)
	assert.NotContains(t, out, "Arch Lantern")
}

// A carried lantern and a floor fixture: the full name still reaches the
// fixture, and `look lantern` with nothing carried shows it.
func TestLookFixtureReachableWithAndWithoutACarriedLantern(t *testing.T) {
	user, room := seedArchRoom(t)
	Look("lantern", user, room, 0)
	assert.Contains(t, sentTo(user), `You look at the <ansi fg="item">Arch Lantern</ansi> here:`)

	const ownId = 999995
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureCmdItemId: {ItemId: fixtureCmdItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight, Value: 1, NotSalable: true},
		pebbleCmdItemId: {ItemId: pebbleCmdItemId, Name: "Grey Pebble", NameSimple: "pebble", Type: items.Object, Weight: 0.1, Value: 1},
		ownId:           {ItemId: ownId, Name: "Tin Lantern", NameSimple: "lantern", Type: items.Object, Weight: 0.1, Value: 1},
	}))
	user.Character.Items = append(user.Character.Items, items.New(ownId))
	events.DrainQueuedMessagesForTest(user.UserId)
	Look("arch lantern", user, room, 0)
	out := sentTo(user)
	assert.Contains(t, out, `You look at the <ansi fg="item">Arch Lantern</ansi> here:`)
	assert.NotContains(t, out, "Tin Lantern")
}
