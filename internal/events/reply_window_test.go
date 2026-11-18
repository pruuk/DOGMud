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

	mine := DrainQueuedMessageEventsForTest(7)
	require.Len(t, mine, 3)
	require.True(t, mine[0].Reply)
	require.True(t, mine[1].Reply)
	require.False(t, mine[2].Reply, "the window closed")
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

	got := DrainQueuedMessageEventsForTest(7)
	require.Len(t, got, 2)
	require.True(t, got[0].Reply)
	require.False(t, got[1].Reply)
}
