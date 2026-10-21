package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
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

// TestConfirmMenu_StartEnterReasks: the experience question has no default
// (owner 2026-10-21). A newcomer who only presses Enter must not land on a
// path that skips Dewey's tutorial, so Enter asks again.
func TestConfirmMenu_StartEnterReasks(t *testing.T) {
	u := users.NewTestUser(841, "newcomer", "Newcomer", 0)
	u.Character.RoomId = -1
	confirmUsers(t, u)

	_, _ = Start("", u, &rooms.Room{RoomId: -1}, 0)
	out := drainSentText(u.UserId)
	assert.Contains(t, out, "New to DOGMud")
	assert.Contains(t, out, "Type a number.")
	assert.NotContains(t, out, "press Enter for")

	u.RoutePromptInput("", true)
	q := questionFor(t, u, "How much of Gaius do you already know?")
	assert.False(t, q.Done, "Enter picks no path")
	assert.Contains(t, drainSentText(u.UserId), "Pick a number from the list.")
}

func TestConfirmMenu_MailSetAsideLine(t *testing.T) {
	sender := users.NewTestUser(833, "sender3", "Sender3", 0)
	bob := users.NewTestUser(834, "bob3", "Bob3", 0)
	confirmUsers(t, sender, bob)
	run := func() { _, _ = Mail("Bob3", sender, &rooms.Room{}, 0) }
	drive(t, sender, run, "hello there", "0", "none")
	events.DrainQueuedMessagesForTest(sender.UserId)
	routeNonAnswer(t, sender, "north")
	assert.Contains(t, drainSentText(sender.UserId), "You set your unsent mail aside.")
}

// characterRoomUser is a player in a character room with one saved alt, Zed,
// written to a temp data dir.
func characterRoomUser(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	cfg := configs.GetConfig()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "users"), 0755))
	cfg.FilePaths.DataFiles = configs.ConfigString(dir)
	cfg.GamePlay.MaxAltCharacters = 3
	configs.SetConfigForTest(t, cfg)
	u := users.NewTestUser(851, "maven", "Maven", 0)
	confirmUsers(t, u)
	zed := characters.New()
	zed.Name = "Zed"
	require.True(t, characters.SaveAlts(u.UserId, []characters.Character{*zed}))
	return u, &rooms.Room{RoomId: 852, Zone: "TestZone", IsCharacterRoom: true}
}

func altNames(userId int) []string {
	var names []string
	for _, c := range characters.LoadAlts(userId) {
		names = append(names, c.Name)
	}
	return names
}

func TestConfirmMenu_CharacterRoomAltDelete(t *testing.T) {
	for _, tc := range []struct {
		line    string
		deletes bool
	}{
		{"1", true},
		{"y", true},
		{"", false},
	} {
		t.Run("line "+tc.line, func(t *testing.T) {
			u, room := characterRoomUser(t)
			run := func() { _, _ = Character("", u, room, 0) }

			out := drive(t, u, run)
			assert.Contains(t, out, "Delete a character")
			out = drive(t, u, run, "4", "Zed")
			assert.Contains(t, out, "Yes, delete them")
			assert.Contains(t, out, "No, keep them")

			out = drive(t, u, run, tc.line)
			assert.Nil(t, u.GetPrompt())
			if tc.deletes {
				assert.Empty(t, altNames(u.UserId))
				assert.Contains(t, out, "is deleted.")
			} else {
				assert.Equal(t, []string{"Zed"}, altNames(u.UserId))
				assert.Contains(t, out, "Okay. Aborting.")
			}
		})
	}

	t.Run("a command closes it", func(t *testing.T) {
		u, room := characterRoomUser(t)
		run := func() { _, _ = Character("", u, room, 0) }
		drive(t, u, run, "delete", "Zed")
		routeNonAnswer(t, u, "north")
		assert.Equal(t, []string{"Zed"}, altNames(u.UserId))
	})

	t.Run("Enter on the room menu closes it", func(t *testing.T) {
		u, room := characterRoomUser(t)
		run := func() { _, _ = Character("", u, room, 0) }
		drive(t, u, run, "")
		assert.Nil(t, u.GetPrompt())
	})
}

// picklockFixture is a player with lockpicks and Skullduggery facing a
// locked chest, and the chest's pin sequence.
func picklockFixture(t *testing.T) (*users.UserRecord, *rooms.Room, string, string) {
	t.Helper()
	t.Cleanup(seedDefaultBiome(t))
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "lockpicks", NameSimple: "lockpicks", Type: items.Lockpicks},
	}))
	u := users.NewTestUser(861, "sneak", "Sneak", 0)
	u.Character.SetSkill(string(skills.Skullduggery), 2)
	u.Character.Items = []items.Item{items.New(1)}
	confirmUsers(t, u)
	room := &rooms.Room{RoomId: 862, Zone: "TestZone", Containers: map[string]rooms.Container{
		"chest": {Lock: gamelock.Lock{Difficulty: 6}},
	}}
	lockId := fmt.Sprintf(`%d-%s`, room.RoomId, "chest")
	seq := util.GetLockSequence(lockId, 6, string(configs.GetServerConfig().Seed), 0)
	require.NotEmpty(t, seq)
	return u, room, lockId, seq
}

func TestPicklockMenu(t *testing.T) {
	u, room, lockId, seq := picklockFixture(t)
	run := func() { _, _ = Picklock("chest", u, room, 0) }

	out := drive(t, u, run)
	assert.Contains(t, out, "Stop picking")

	// The short form of the right first pin moves the pick.
	short := strings.ToLower(seq[:1])
	out = drive(t, u, run, short)
	assert.Equal(t, seq[:1], u.Character.GetKey(lockId), "the first pin is on the keyring")
	assert.Contains(t, out, "Stop picking", "the choices are shown again")

	// Enter has no default: the menu stays open and is drawn again.
	out = drive(t, u, run, "")
	assert.Contains(t, out, "Pick a number from the list.")
	assert.Contains(t, out, "Stop picking")
	require.NotNil(t, u.GetPrompt())
	assert.Equal(t, seq[:1], u.Character.GetKey(lockId))

	// A command closes the picking and the progress stays on the keyring.
	routeNonAnswer(t, u, "north")
	assert.Equal(t, seq[:1], u.Character.GetKey(lockId))
}

func TestConfirmMenu_RenameLabels(t *testing.T) {
	setRenameCooldownHours(t, 0)
	u := users.NewTestUser(871, "oldname", "Oldname", 0)
	confirmUsers(t, u)
	run := func() { _, _ = RenameSelf("Newname", u, &rooms.Room{}, 0) }
	out := drive(t, u, run)
	assert.Contains(t, out, "Yes, rename me")
	assert.Contains(t, out, "No, keep my name")
}

// TestStart_VeteranDeclineAnnouncesOnce: a veteran who answers No to the
// skip confirm goes back to the route question. Answering it again must not
// fire the arrival (CharacterCreated and the bug-report line) a second time.
func TestStart_VeteranDeclineAnnouncesOnce(t *testing.T) {
	u := users.NewTestUser(843, "veteran", "Veteran", 0)
	u.Character.RoomId = -1
	confirmUsers(t, u)
	room := &rooms.Room{RoomId: -1}
	run := func() { _, _ = Start("", u, room, 0) }

	all := []string{}
	run()
	all = append(all, drainSentText(u.UserId))
	for _, line := range []string{"3", "", "3", ""} {
		u.RoutePromptInput(line, true)
		run()
		all = append(all, drainSentText(u.UserId))
	}
	joined := strings.Join(all, "\n")
	assert.Equal(t, 1, strings.Count(joined, "One quick thing before you go."), "the arrival is announced once")
	assert.Contains(t, joined, "How much of Gaius do you already know?")
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
