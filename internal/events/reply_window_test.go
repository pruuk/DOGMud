package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #842: a Message queued for a player while their typed line is being
// handled is marked as that line's reply; nothing else is.
func TestReplyWindow_MarksOnlyThatPlayersMessages(t *testing.T) {
	DrainAllQueuedEventsForTest()
	defer DrainAllQueuedEventsForTest()

	OpenReply(7)
	AddToQueue(Message{UserId: 7, Text: "yours\n"})
	AddToQueue(Message{UserId: 7, Text: "a menu\n"}, 50)
	AddToQueue(Message{UserId: 8, Text: "someone else\n"})
	AddToQueue(Message{RoomId: 3, Text: "a shout from next door\n"})
	CloseReply(7)
	AddToQueue(Message{UserId: 7, Text: "after the reply\n"})

	mine := map[string]Message{}
	for _, m := range DrainQueuedMessageEventsForTest(7) {
		mine[m.Text] = m
	}
	require.Len(t, mine, 3)
	require.True(t, mine["yours\n"].Reply)
	require.True(t, mine["a menu\n"].Reply)
	require.False(t, mine["after the reply\n"].Reply, "the window closed")
	require.False(t, DrainQueuedMessageEventsForTest(8)[0].Reply)
	require.False(t, DrainQueuedRoomMessagesForTest(3)[0].Reply)
}

// Two typed lines in one pass each open and close their own window.
func TestReplyWindow_Counts(t *testing.T) {
	DrainAllQueuedEventsForTest()
	defer DrainAllQueuedEventsForTest()

	OpenReply(7)
	OpenReply(7)
	CloseReply(7)
	AddToQueue(Message{UserId: 7, Text: "still a reply\n"})
	CloseReply(7)
	CloseReply(7) // one too many is harmless
	AddToQueue(Message{UserId: 7, Text: "not a reply\n"})
	// The extra close must not leave the count negative: a later window
	// still opens.
	OpenReply(7)
	AddToQueue(Message{UserId: 7, Text: "reopened\n"})
	CloseReply(7)

	got := map[string]Message{}
	for _, m := range DrainQueuedMessageEventsForTest(7) {
		got[m.Text] = m
	}
	require.Len(t, got, 3)
	require.True(t, got["still a reply\n"].Reply)
	require.False(t, got["not a reply\n"].Reply)
	require.True(t, got["reopened\n"].Reply)
}
