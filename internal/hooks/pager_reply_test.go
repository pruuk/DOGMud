package hooks

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// replyOf sends n marked lines to user 1, as a typed line's reply arrives.
func replyOf(n int) {
	for i := 1; i <= n; i++ {
		Message_SendMessage(events.Message{UserId: 1, Text: fmt.Sprintf("reply %d\n", i), Reply: true})
	}
}

// lf is the screen as written with its telnet line ends (\r\n) read as \n.
func lf(s *screenCapture) string {
	return strings.ReplaceAll(s.bytes(), "\r\n", "\n")
}

func pagerFixture(t *testing.T) (*users.UserRecord, *screenCapture) {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	connId, screen := captureConnection(t)
	u := users.NewTestUser(1, "alice", "Aliceia", connId)
	u.PageSize = 10
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	return u, screen
}

func TestPager_ALongReplyShowsPageOneAndTheStopLine(t *testing.T) {
	_, screen := pagerFixture(t)
	replyOf(25)
	require.Never(t, func() bool { return strings.Contains(lf(screen), "reply 1\n") },
		100*time.Millisecond, 10*time.Millisecond, "nothing of the reply goes out before its end")

	Pager_ReplyEnd(events.InputDone{UserId: 1})
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1})
	require.Eventually(t, func() bool {
		s := lf(screen)
		return strings.Contains(s, "reply 10\n") && !strings.Contains(s, "reply 11\n") &&
			strings.Contains(visibleLastLine(s), "More (page 1 of 3)")
	}, time.Second, 10*time.Millisecond, "screen %q", screen.bytes())
}

func TestPager_AShortReplyIsSentUnchanged(t *testing.T) {
	_, screen := pagerFixture(t)
	replyOf(3)
	Pager_ReplyEnd(events.InputDone{UserId: 1})
	require.Eventually(t, func() bool {
		return strings.Contains(lf(screen), "reply 1\nreply 2\nreply 3\n")
	}, time.Second, 10*time.Millisecond, "screen %q", screen.bytes())
	require.NotContains(t, screen.bytes(), "More (page")
}

func TestPager_AnUnmarkedLineGoesOutDuringAStop(t *testing.T) {
	_, screen := pagerFixture(t)
	replyOf(25)
	Pager_ReplyEnd(events.InputDone{UserId: 1})
	Message_SendMessage(events.Message{UserId: 1, Text: "A wolf bites you!\n"})
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1})
	require.Eventually(t, func() bool {
		s := screen.bytes()
		return strings.Contains(s, "A wolf bites you!") && strings.Contains(visibleLastLine(s), "More (page 1 of 3)")
	}, time.Second, 10*time.Millisecond, "screen %q", screen.bytes())
}

func TestPager_AFightSendsTheWaitingPagesFirst(t *testing.T) {
	u, screen := pagerFixture(t)
	replyOf(25)
	Pager_ReplyEnd(events.InputDone{UserId: 1})
	u.Character.CombatPhase = combatphase.NewMachine()
	require.NoError(t, u.Character.CombatPhase.Inner().TransitionTo(combatphase.Engaging, state.TransitionReason{}))
	Message_SendMessage(events.Message{UserId: 1, Text: "A wolf bites you!\n"})
	require.Eventually(t, func() bool {
		s := lf(screen)
		i, j := strings.Index(s, "reply 25\n"), strings.Index(s, "A wolf bites you!")
		return i >= 0 && j > i
	}, time.Second, 10*time.Millisecond, "screen %q", screen.bytes())
	require.False(t, u.PagesWaiting())
}

func TestPager_ReplyEndClosesTheWindow(t *testing.T) {
	pagerFixture(t)
	events.OpenReply(1)
	Pager_ReplyEnd(events.InputDone{UserId: 1})
	events.AddToQueue(events.Message{UserId: 1, Text: "later\n"})
	got := events.DrainQueuedMessageEventsForTest(1)
	require.Len(t, got, 1)
	require.False(t, got[0].Reply)
}
