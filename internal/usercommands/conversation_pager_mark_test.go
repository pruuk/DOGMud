package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// #842: everything talk sends the player (the NPC's words, the hints, the
// menu) is queued inside the reply window, so the pager holds all of it and
// nothing arrives ahead of page one.
func TestTalk_EveryLineToThePlayerIsTheReply(t *testing.T) {
	var b strings.Builder
	b.WriteString("mobid: 1\nzone: TestZone\nopener:\n  text: \"Crier's news, fresh as the morning.\"\n  hints: \"Choose a line below.\"\ntopics:\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "  - key: t%d\n    kind: info\n    label: \"Topic number %d\"\n    text: \"Answer %d.\"\n", i, i, i)
	}
	user, mob, cleanup := fixtureWith(t, b.String())
	defer cleanup()
	events.DrainAllQueuedEventsForTest()
	defer events.DrainAllQueuedEventsForTest()

	events.OpenReply(user.UserId)
	conversation.Talk(user, mustRoom(t, user), mob)
	events.CloseReply(user.UserId)

	msgs := events.DrainQueuedMessageEventsForTest(user.UserId)
	require.NotEmpty(t, msgs)
	var all strings.Builder
	for _, m := range msgs {
		require.True(t, m.Reply, "not marked: %q", m.Text)
		all.WriteString(m.Text)
	}
	require.Contains(t, all.String(), "Topic number 20", "the menu is among the marked lines")
	require.Contains(t, all.String(), "fresh as the morning", "the NPC's words are among the marked lines")
}
