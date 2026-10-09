package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #455 review: the end-punctuation stage runs on CategoryMobEmote room lines
// and on CategorySystem, so a line that ends in text the player typed, or
// in a coloured vital bar, had a stop written into it.

func endPunctLines(t *testing.T, run func(user *users.UserRecord, room *rooms.Room) (bool, error)) (own, room string) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	user, r := getTestUserAndRoom(t)
	observer := users.GetByUserId(2)
	require.NotNil(t, observer)
	require.Equal(t, r.RoomId, observer.Character.RoomId)
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(observer.UserId)
	_, err := run(user, r)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"),
		strings.Join(events.DrainQueuedMessagesForTest(observer.UserId), "\n")
}

// The afk message is the player's own words; the server adds no stop.
func TestAfkMessageIsNotPunctuated(t *testing.T) {
	own, room := endPunctLines(t, func(user *users.UserRecord, r *rooms.Room) (bool, error) {
		user.Character.Presence = presence.NewPlayerPresence()
		return AFK(`brb`, user, r, 0)
	})
	for name, out := range map[string]string{"own": own, "room": room} {
		require.Contains(t, out, `brb`, name)
		require.NotContains(t, out, `brb.`, "%s line punctuated the player's afk message: %q", name, out)
	}
}

// The report line ends in the last vital bar; no stop lands in its colour.
func TestReportBarsAreNotPunctuated(t *testing.T) {
	own, room := endPunctLines(t, func(user *users.UserRecord, r *rooms.Room) (bool, error) {
		return Report(``, user, r, 0)
	})
	for name, out := range map[string]string{"own": own, "room": room} {
		require.Contains(t, out, `CP`, name)
		require.NotContains(t, out, `.</ansi>`, "%s report line has a stop inside a bar's colour: %q", name, out)
	}
}

// A line ending in a pet's coloured name authors its own stop after the
// name, so the stage does not write one inside the name's colour.
func TestPetLinesStopAfterThePetName(t *testing.T) {
	own, room := endPunctLines(t, func(user *users.UserRecord, r *rooms.Room) (bool, error) {
		user.Character.Pet.Type = `dog`
		user.Character.Pet.Name = `Rex`
		return Pet(`pet`, user, r, 0)
	})
	for name, out := range map[string]string{"own": own, "room": room} {
		require.Contains(t, out, `Rex`, name)
		require.NotContains(t, out, `Rex.</ansi>`, "%s pet line has a stop inside the name's colour: %q", name, out)
	}
}
