package usercommands

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wrapTestLongLine = "the pool answers first, it is older than the town and more patient than I am, so look into the water and wait"

// wrapTestLines is the one message queued for uid, as the lines it reads.
func wrapTestLines(t *testing.T, uid int) []string {
	t.Helper()
	msgs := events.DrainQueuedMessagesForTest(uid)
	require.Len(t, msgs, 1, "user %d should get one message: %q", uid, msgs)
	return strings.Split(strings.TrimRight(setScreenEscRE.ReplaceAllString(msgs[0], ""), "\n"), "\n")
}

// A whisper and a reply wrap for the person reading them, not the sender.
func TestWhisperAndReply_WrapAtTheReadersWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	reader := users.GetByCharacterName("bobrick")
	require.NotNil(t, reader)
	reader.LineWidth = 40
	user.LineWidth = 200
	events.DrainQueuedMessagesForTest(reader.UserId)

	_, err := Whisper("bobrick "+wrapTestLongLine, user, room, 0)
	require.NoError(t, err)
	whispered := wrapTestLines(t, reader.UserId)
	require.Greater(t, len(whispered), 2)
	for _, line := range whispered {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}

	user.LastWhisperFrom = reader.UserId
	reader.LineWidth = 200
	user.LineWidth = 40
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Reply(wrapTestLongLine, reader, room, 0)
	require.NoError(t, err)
	replied := wrapTestLines(t, user.UserId)
	require.Greater(t, len(replied), 2)
	for _, line := range replied {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}
}

// The set screen lays out to the reader, as a bare 80 used to pretend they all were.
func TestSetScreen_WrapsToTheReadersWidth(t *testing.T) {
	u := users.NewTestUser(8803, "narrow", "Narrow", 18803)
	u.LineWidth = 40
	u.Character.Description = wrapTestLongLine
	u.SetConfigOption(`fprompt`, shippedFightPrompt)

	lines := renderedSetLines(t, u, func() { displaySetStatus(u) })
	require.Greater(t, len(lines), 5)
	for i, line := range lines {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "line %d: %q", i, line)
	}
}
