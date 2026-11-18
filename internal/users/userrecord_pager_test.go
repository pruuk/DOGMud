package users

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/require"
)

// A connection on the AI port (AI players and the playtest harness) never
// pages: an agent's next command is rarely Enter, so a stop would drop the
// rest of the reply.
func TestEndReply_AnAIPortConnectionGetsTheWholeReply(t *testing.T) {
	const connId = 18842
	connections.RegisterTestConnection(connId)
	t.Cleanup(func() { connections.UnregisterTestConnection(connId) })
	connections.Get(connId).SetConnType(connections.ConnAI)

	u := NewTestUser(1, "agent", "Agentia", connId)
	u.PageSize = 10
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	require.Equal(t, strings.Join(numbered(25), ""), u.EndReply())
	require.False(t, u.PagesWaiting())

	// The same reply on a human connection pages, so the test can fail.
	connections.Get(connId).SetConnType(connections.ConnHuman)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.True(t, u.PagesWaiting())
}

// numbered is n lines "line 1" .. "line n", each ending in a line break, as
// held messages arrive.
func numbered(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %d\n", i+1)
	}
	return out
}

func pagerUser(t *testing.T, pageSize int) *UserRecord {
	t.Helper()
	u := NewTestUser(1, "alice", "Aliceia", 0)
	u.PageSize = pageSize
	return u
}

func TestEndReply_AReplyThatFitsIsSentWhole(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(10) {
		u.HoldReply(l)
	}
	require.Equal(t, strings.Join(numbered(10), ""), u.EndReply())
	require.False(t, u.PagesWaiting())
	require.Equal(t, "", u.EndReply(), "nothing held, nothing to send")
}

func TestEndReply_ALongReplySendsPageOneAndWaits(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	first := u.EndReply()
	require.Equal(t, pager.Join(pager.Lines(strings.Join(numbered(10), ""))), first)
	require.True(t, u.PagesWaiting())
	require.Equal(t, pager.StopLine(1, 3), u.pagerStopLine())
}

func TestRoutePagerInput(t *testing.T) {
	long := func() *UserRecord {
		u := pagerUser(t, 10)
		for _, l := range numbered(25) {
			u.HoldReply(l)
		}
		u.EndReply()
		return u
	}

	t.Run("Enter shows each page, then hands Enter back", func(t *testing.T) {
		u := long()
		out, consumed := u.RoutePagerInput("")
		require.True(t, consumed)
		require.Contains(t, out, "line 11\n")
		require.Equal(t, pager.StopLine(2, 3), u.pagerStopLine())
		out, consumed = u.RoutePagerInput("  ")
		require.True(t, consumed)
		require.Contains(t, out, "line 25\n")
		require.False(t, u.PagesWaiting())
		_, consumed = u.RoutePagerInput("")
		require.False(t, consumed, "with no pages left Enter is the menu's again")
	})
	t.Run("q drops the rest", func(t *testing.T) {
		u := long()
		out, consumed := u.RoutePagerInput("Q")
		require.True(t, consumed)
		require.Equal(t, "", out)
		require.False(t, u.PagesWaiting())
	})
	t.Run("anything else drops the rest and runs", func(t *testing.T) {
		u := long()
		out, consumed := u.RoutePagerInput("3")
		require.False(t, consumed)
		require.Equal(t, "", out)
		require.False(t, u.PagesWaiting())
	})
}

func TestEndReply_PagingOffSendsWhole(t *testing.T) {
	u := pagerUser(t, pager.SettingOff)
	for _, l := range numbered(40) {
		u.HoldReply(l)
	}
	require.Equal(t, strings.Join(numbered(40), ""), u.EndReply())
	require.False(t, u.PagesWaiting())
}

func inCombat(t *testing.T, u *UserRecord) {
	t.Helper()
	u.Character.CombatPhase = combatphase.NewMachine()
	require.NoError(t, u.Character.CombatPhase.Inner().TransitionTo(combatphase.Engaging, state.TransitionReason{}))
}

func TestEndReply_InCombatSendsWhole(t *testing.T) {
	u := pagerUser(t, 10)
	inCombat(t, u)
	for _, l := range numbered(40) {
		u.HoldReply(l)
	}
	require.Equal(t, strings.Join(numbered(40), ""), u.EndReply())
	require.False(t, u.PagesWaiting())
}

func TestFlushPagesInCombat(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.Equal(t, "", u.FlushPagesInCombat(), "not fighting: the pages wait")
	require.True(t, u.PagesWaiting())

	inCombat(t, u)
	rest := u.FlushPagesInCombat()
	require.Contains(t, rest, "line 11\n")
	require.Contains(t, rest, "line 25\n")
	require.False(t, u.PagesWaiting())
}

func TestEndReply_ANewReplyReplacesWaitingPages(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	u.HoldReply("short\n")
	require.Equal(t, "short\n", u.EndReply())
	require.False(t, u.PagesWaiting())
}

func TestEndReply_AnEmptyReplyKeepsWaitingPages(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.Equal(t, "", u.EndReply(), "Enter at a stop holds nothing")
	require.True(t, u.PagesWaiting())
}

func TestEndReply_BlankLinesDroppedToOnePageSendsWhole(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(10) {
		u.HoldReply(l)
	}
	u.HoldReply("\n\n\n")
	want := strings.Join(numbered(10), "") + "\n\n\n"
	require.Equal(t, want, u.EndReply())
	require.False(t, u.PagesWaiting(), "one page is never kept as 'page 1 of 1'")
	require.NotPanics(t, func() {
		_, consumed := u.RoutePagerInput("")
		require.False(t, consumed)
	})
}

func openQuestion(t *testing.T, u *UserRecord) {
	t.Helper()
	p, _ := u.StartPrompt("portalstake", "")
	p.Ask("How much gold will you stake?", []string{})
}

func TestPager_AnOpenQuestionBeatsTheStopLine(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.True(t, u.PagesWaiting())

	openQuestion(t, u)
	prompt := u.GetCommandPrompt()
	require.Contains(t, prompt, "How much gold will you stake?")
	require.NotContains(t, prompt, "More (page")

	out, consumed := u.RoutePagerInput("50")
	require.False(t, consumed)
	require.Equal(t, "", out)
	require.True(t, u.PagesWaiting(), "the pages keep waiting while the question is open")

	u.ClearPrompt()
	require.Contains(t, u.GetCommandPrompt(), pager.StopLine(1, 3))
}

func TestEndReply_AnOpenQuestionSendsWhole(t *testing.T) {
	u := pagerUser(t, 10)
	openQuestion(t, u)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	require.Equal(t, strings.Join(numbered(25), ""), u.EndReply())
	require.False(t, u.PagesWaiting())
}

func TestGetCommandPrompt_IsTheStopLineWhilePagesWait(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.Contains(t, u.GetCommandPrompt(), pager.StopLine(1, 3))
	u.RoutePagerInput("q")
	require.NotContains(t, u.GetCommandPrompt(), "More (page")
}
