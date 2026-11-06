package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #761: room 498 has a noun and a container both named `strongbox`. `look
// strongbox` showed the container, but `look in strongbox` read the noun's
// text, because the words "in strongbox" were never trimmed to "strongbox"
// and the noun matcher settles for any one word it knows.
func TestTrimLookFiller(t *testing.T) {
	for in, want := range map[string]string{
		`strongbox`:        `strongbox`,
		`in strongbox`:     `strongbox`,
		`inside strongbox`: `strongbox`,
		`in the strongbox`: `strongbox`,
		`at the strongbox`: `strongbox`,
		`at strongbox`:     `strongbox`,
		`the strongbox`:    `strongbox`,
		`In Strongbox`:     `Strongbox`,
		`in`:               `in`,
		`inn keeper`:       `inn keeper`,
	} {
		assert.Equal(t, want, trimLookFiller(in), in)
	}
}

func seedStrongboxRoom(t *testing.T) (*rooms.Room, func(string)) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	oldNouns, oldContainers := room.Nouns, room.Containers
	room.Nouns = map[string]string{"strongbox": "A heavy iron strongbox beneath the table."}
	t.Cleanup(func() { room.Nouns, room.Containers = oldNouns, oldContainers })
	events.DrainQueuedMessagesForTest(user.UserId)
	look := func(rest string) {
		_, err := Look(rest, user, room, 0)
		require.NoError(t, err)
	}
	return room, look
}

func TestLookInContainerShowsTheContainerNotTheSameNamedNoun(t *testing.T) {
	cases := map[string]rooms.Container{
		"open":   {Items: []items.Item{items.New(10001)}},
		"empty":  {},
		"locked": {Lock: gamelock.Lock{Difficulty: 5}},
	}
	for name, box := range cases {
		t.Run(name, func(t *testing.T) {
			room, look := seedStrongboxRoom(t)
			room.Containers = map[string]rooms.Container{"strongbox": box}
			user, _ := getTestUserAndRoom(t)

			look("strongbox")
			plain := sentTo(user)
			for _, form := range []string{"in strongbox", "in the strongbox", "inside strongbox"} {
				look(form)
				assert.Equal(t, plain, sentTo(user), "look %s must read as look strongbox", form)
			}
			assert.NotContains(t, plain, "heavy iron strongbox", "plain look shows the container")
		})
	}
}

// The look_at event is raised on the trimmed words, so `look in strongbox`
// fires the trigger `look strongbox` fires.
func TestLookInRaisesTheSameLookAtAsPlainLook(t *testing.T) {
	for _, form := range []string{"strongbox", "in strongbox", "at the strongbox"} {
		t.Run(form, func(t *testing.T) {
			alice, room := interactScene(t)
			room.Nouns = map[string]string{"strongbox": "A heavy iron strongbox."}
			fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
				999440, "look_at", room.RoomId, questengine.TriggerDef{Noun: "strongbox"})
			_, err := TryCommand("look", form, alice.UserId, 0)
			require.NoError(t, err)
			require.True(t, fired(), "look %s must fire a look_at trigger", form)
		})
	}
}
