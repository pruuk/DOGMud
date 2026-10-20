package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 0, Task 15: every yes/no confirm is a two-entry menu. Each test
// drives a player flow the way world.go does: the command runs, a typed line
// goes through RoutePromptInput, and the command runs again. "1" and "y" pick
// yes, Enter picks no, and on a non-modal flow a line that is not an answer
// ("north") closes the prompt and comes back to run as a command.

// confirmUsers seeds the user manager with the given test users only.
func confirmUsers(t *testing.T, recs ...*users.UserRecord) {
	t.Helper()
	m := map[int]*users.UserRecord{}
	for _, u := range recs {
		m[u.UserId] = u
		events.DrainQueuedMessagesForTest(u.UserId)
	}
	t.Cleanup(users.SeedUsersForTest(m))
	t.Cleanup(func() {
		for _, u := range recs {
			u.ClearPrompt()
		}
	})
}

// routeNonAnswer types a line that is not an answer to a non-modal flow and
// checks it closed the prompt and came back to run as a command.
func routeNonAnswer(t *testing.T, u *users.UserRecord, line string) {
	t.Helper()
	require.NotNil(t, u.GetPrompt(), "the confirm must be open before the line")
	next, had := u.RoutePromptInput(line, true)
	assert.True(t, had)
	assert.Equal(t, line, next, "the line runs as a command")
	assert.Nil(t, u.GetPrompt(), "a non-modal confirm closes on a line it does not recognise")
}

func TestConfirmMenu_GuildDisband(t *testing.T) {
	t.Cleanup(guilds.SetDataDirForTest(t.TempDir()))

	for _, tc := range []struct {
		line     string
		disbands bool
	}{
		{"1", true},
		{"y", true},
		{"", false},
	} {
		t.Run("line "+tc.line, func(t *testing.T) {
			leader := users.NewTestUser(811, "leader", "Leader", 0)
			confirmUsers(t, leader)
			_, err := guilds.Create("TQ", "Test Quorum", leader.UserId, leader.Character.Name)
			require.NoError(t, err)
			t.Cleanup(func() { guilds.Delete("TQ") })
			run := func() { _, _ = Guild("disband", leader, &rooms.Room{}, 0) }

			out := drive(t, leader, run)
			assert.Contains(t, out, "Yes, disband it")
			assert.Contains(t, out, "No, keep it")

			out = drive(t, leader, run, tc.line)
			_, stillThere := guilds.Get("TQ")
			assert.Equal(t, !tc.disbands, stillThere)
			if !tc.disbands {
				assert.Contains(t, out, "Cancelled.")
			}
			assert.Nil(t, leader.GetPrompt())
		})
	}

	t.Run("a command closes it", func(t *testing.T) {
		leader := users.NewTestUser(811, "leader", "Leader", 0)
		confirmUsers(t, leader)
		_, err := guilds.Create("TQ", "Test Quorum", leader.UserId, leader.Character.Name)
		require.NoError(t, err)
		t.Cleanup(func() { guilds.Delete("TQ") })

		_, _ = Guild("disband", leader, &rooms.Room{}, 0)
		routeNonAnswer(t, leader, "north")
		_, stillThere := guilds.Get("TQ")
		assert.True(t, stillThere)
	})
}

func TestConfirmMenu_DeleteCharacter(t *testing.T) {
	const nameQuestion = `To confirm, type your character's name exactly`
	for _, tc := range []struct {
		line      string
		continues bool
	}{
		{"1", true},
		{"y", true},
		{"", false},
	} {
		t.Run("line "+tc.line, func(t *testing.T) {
			u := users.NewTestUser(821, "alice", "Alice", 0)
			confirmUsers(t, u)
			run := func() { _, _ = DeleteCharacter("", u, &rooms.Room{}, 0) }

			out := drive(t, u, run)
			assert.Contains(t, out, "Yes, delete my account")

			drive(t, u, run, tc.line)
			assert.NotNil(t, users.GetByUserId(u.UserId), "one gate never deletes")
			if tc.continues {
				require.NotNil(t, u.GetPrompt())
				assert.True(t, strings.HasPrefix(lastQuestion(t, u), nameQuestion), "yes moves on to the name check")
			} else {
				assert.Nil(t, u.GetPrompt(), "Enter declines and closes")
			}
		})
	}

	t.Run("a command closes it", func(t *testing.T) {
		u := users.NewTestUser(821, "alice", "Alice", 0)
		confirmUsers(t, u)
		_, _ = DeleteCharacter("", u, &rooms.Room{}, 0)
		routeNonAnswer(t, u, "north")
		assert.NotNil(t, users.GetByUserId(u.UserId))
	})
}

func TestConfirmMenu_MailSend(t *testing.T) {
	for _, tc := range []struct {
		line  string
		sends bool
	}{
		{"1", true},
		{"y", true},
		{"", false},
	} {
		t.Run("line "+tc.line, func(t *testing.T) {
			sender := users.NewTestUser(831, "sender", "Sender", 0)
			bob := users.NewTestUser(832, "bob", "Bob", 0)
			confirmUsers(t, sender, bob)
			run := func() { _, _ = Mail("Bob", sender, &rooms.Room{}, 0) }

			// The free-text questions still take any line on a non-modal
			// prompt; only the final confirm is a menu.
			out := drive(t, sender, run, "hello there", "0", "none")
			assert.Contains(t, out, "Yes, send it")

			out = drive(t, sender, run, tc.line)
			assert.Nil(t, sender.GetPrompt())
			if tc.sends {
				assert.Len(t, bob.Inbox, 1, "the mail arrives")
				assert.Contains(t, out, "is on its way")
			} else {
				assert.Empty(t, bob.Inbox, "Enter declines")
				assert.Contains(t, out, "Cancelling the mail.")
			}
		})
	}

	t.Run("a command closes it", func(t *testing.T) {
		sender := users.NewTestUser(831, "sender", "Sender", 0)
		bob := users.NewTestUser(832, "bob", "Bob", 0)
		confirmUsers(t, sender, bob)
		run := func() { _, _ = Mail("Bob", sender, &rooms.Room{}, 0) }
		drive(t, sender, run, "hello there", "0", "none")
		routeNonAnswer(t, sender, "north")
		assert.Empty(t, bob.Inbox)
	})
}

func TestConfirmMenu_StartEnterPicksTwo(t *testing.T) {
	u := users.NewTestUser(841, "newcomer", "Newcomer", 0)
	u.Character.RoomId = -1
	confirmUsers(t, u)

	_, _ = Start("", u, &rooms.Room{RoomId: -1}, 0)
	out := drainSentText(u.UserId)
	assert.Contains(t, out, "New to DOGMud")

	u.RoutePromptInput("", true)
	q := questionFor(t, u, "How much of Gaius do you already know?")
	assert.True(t, q.Done)
	assert.Equal(t, "2", q.Response, "Enter picks New to DOGMud")
}

func TestConfirmMenu_StartIsModal(t *testing.T) {
	u := users.NewTestUser(842, "newcomer2", "Newcomer2", 0)
	u.Character.RoomId = -1
	confirmUsers(t, u)

	_, _ = Start("", u, &rooms.Room{RoomId: -1}, 0)
	next, _ := u.RoutePromptInput("north", true)
	assert.NotNil(t, u.GetPrompt(), "onboarding must be answered")
	assert.Equal(t, "start ", next, "the start command runs again to redraw")
	assert.False(t, questionFor(t, u, "How much of Gaius do you already know?").Done)
}
