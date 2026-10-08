package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #274 (owner R3, 2026-10-08): a hidden player's emote
// reaches no one, and the player is told why, after their own echo. Every
// form: empty, alias, free text, and the @ form that has no echo.
func TestEmote_HiddenPlayerIsToldNoOneSees(t *testing.T) {
	const note = "No one sees it; you are hidden."
	tags := regexp.MustCompile(`<[^>]*>`)
	cases := []struct {
		name, rest, echo string
	}{
		{"empty", "", "You emote."},
		{"alias", "beam", "You Emote: Hider beams with pride."},
		{"free text", "waves.", "You Emote: Hider waves."},
		{"at form", "@waves.", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			room := &rooms.Room{RoomId: 9950, Zone: "EmoteHidden", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(80)}
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9950: room}, map[string]*rooms.ZoneConfig{}))
			hider := users.NewTestUser(9951, "hider", "Hider", 0)
			watcher := users.NewTestUser(9952, "watcher", "Watcher", 0)
			for _, u := range []*users.UserRecord{hider, watcher} {
				u.Character.RoomId = room.RoomId
			}
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9951: hider, 9952: watcher}))
			room.AddPlayer(9951)
			room.AddPlayer(9952)
			reason := state.TransitionReason{Trigger: "emote_hidden_test"}
			require.NoError(t, hider.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
			hider.Character.Awareness.ResolveConcealment(true, reason)
			require.True(t, hider.Character.IsHidden())
			events.DrainQueuedMessagesForTest(9951)
			events.DrainQueuedMessagesForTest(9952)
			events.DrainQueuedRoomMessagesForTest(9950)

			handled, err := Emote(c.rest, hider, room, 0)
			require.True(t, handled)
			require.NoError(t, err)

			var own []string
			for _, line := range events.DrainQueuedMessagesForTest(9951) {
				own = append(own, strings.TrimSpace(tags.ReplaceAllString(line, "")))
			}
			want := []string{note}
			if c.echo != "" {
				want = []string{c.echo, note}
			}
			require.Equal(t, want, own)
			require.Empty(t, events.DrainQueuedMessagesForTest(9952), "a hidden emote reaches no one")
			require.Empty(t, events.DrainQueuedRoomMessagesForTest(9950), "nothing is queued to the room")
		})
	}
}
