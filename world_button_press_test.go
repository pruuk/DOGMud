package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// pressUser seeds one player for the press tests and clears the queue around
// them.
func pressUser(t *testing.T, id int) *users.UserRecord {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	u := users.NewTestUser(id, "presser", "Presser", uint64(id)+90000)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{id: u}))
	t.Cleanup(u.ClearPrompt)
	return u
}

// runInputNoWorld runs processInput with no world loaded. A line that gets
// past the menu and prompt routing runs as a command, which panics here (no
// keywords, rooms or logger); that panic is caught and reported as true, after
// the prompt routing has already done its work, so the caller's assertions on
// the prompt still read what happened.
func runInputNoWorld(userId int, line string, flags events.EventFlag) (reachedCommand bool) {
	defer func() {
		if recover() != nil {
			reachedCommand = true
		}
	}()
	(&World{}).processInput(userId, line, flags, 0)
	return false
}

// #781, D13: a press queued before a prompt opened (a line typed ahead of it
// opened the prompt) never answers it, whether the question is free text or
// a menu that takes free text (AcceptText). The line is dropped silently and
// its InputDone still fires.
func TestProcessInput_AButtonPressNeverAnswersAPrompt(t *testing.T) {
	for name, ask := range map[string]func(*testing.T, *users.UserRecord){
		"a free-text question": func(t *testing.T, u *users.UserRecord) {
			p, ok := u.StartPrompt("mail", "Bob")
			require.True(t, ok)
			p.Ask("Subject?", nil)
		},
		"an AcceptText menu question": func(t *testing.T, u *users.UserRecord) {
			p, ok := u.StartPrompt("server", "config")
			require.True(t, ok)
			p.AskMenu(menu.Menu{Heading: "Pick one:", Leave: "Quit", AcceptText: true,
				Entries: []menu.Entry{{Id: "a", Label: "Alpha"}}}, "quit")
		},
	} {
		t.Run(name, func(t *testing.T) {
			u := pressUser(t, 984311)
			ask(t, u)

			reachedCommand := runInputNoWorld(u.UserId, "north", events.CmdTyped|events.CmdButton)

			p := u.GetPrompt()
			require.NotNil(t, p, "the prompt is still open")
			q := p.GetNextQuestion()
			require.NotNil(t, q, "the question is still unanswered")
			require.False(t, q.Done)
			require.Empty(t, q.Response)
			require.False(t, reachedCommand, "the press stopped before running as a command")
			require.Empty(t, events.DrainQueuedMessagesForTest(u.UserId), "dropped without a word")
			require.Len(t, events.DrainQueuedByTypeForTest(`InputDone`), 1, "the line is still marked done")
		})
	}
}

// A press dropped because a prompt opened ahead of it frees its place in the
// player's pending inputs: through the queue and the world's handler the
// count returns to zero, the prompt is still unanswered, and nothing is said.
func TestInputQueue_ADroppedPressFreesItsSlot(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 10
	configs.SetConfigForTest(t, cfg)

	restore := events.SwapListenersForTest()
	t.Cleanup(restore)
	u := pressUser(t, 984314)
	p, ok := u.StartPrompt("mail", "Bob")
	require.True(t, ok)
	q := p.Ask("Subject?", nil)

	w := testWorld()
	events.RegisterListener(events.Input{}, w.HandleInputEvents)

	events.AddToQueue(events.Input{UserId: u.UserId, InputText: "north", ReadyTurn: util.GetTurnCount(),
		Flags: events.CmdTyped | events.CmdFromClient | events.CmdButton})
	require.Equal(t, 1, events.PendingInputs(u.UserId), "the press counts while it waits")

	w.EventLoop()
	require.Equal(t, 0, events.PendingInputs(u.UserId), "the dropped press holds no slot")
	require.Same(t, p, u.GetPrompt(), "the prompt is still open")
	require.False(t, q.Done, "the press did not answer it")
	require.Empty(t, events.DrainQueuedMessagesForTest(u.UserId), "dropped without a word")
}

// The drop's partner: the same fixture answers a prompt from a typed line, so
// the checks above can see an answer.
func TestProcessInput_ATypedLineStillAnswersAPrompt(t *testing.T) {
	u := pressUser(t, 984312)
	p, ok := u.StartPrompt("mail", "Bob")
	require.True(t, ok)
	q := p.Ask("Subject?", nil)
	_, _ = u.RoutePromptInput("north", true)
	require.True(t, q.Done)
	require.Equal(t, "north", q.Response)
}

// An AcceptText game menu (not a prompt question) never takes free text:
// RouteMenuInput has no Text case, so an unmatched line closes the menu and
// runs as a command, as typing it would. That is why processInput needs no
// button drop before RouteMenuInput.
func TestRouteMenuInput_AnAcceptTextGameMenuTakesNoFreeText(t *testing.T) {
	u := pressUser(t, 984313)
	chosen := false
	_, ok := u.OpenMenu(menu.Menu{Heading: "Pick one:", AcceptText: true,
		Entries: []menu.Entry{{Id: "a", Label: "Alpha"}}},
		menu.Session{OnChoose: func(menu.Entry) { chosen = true }})
	require.True(t, ok)
	require.False(t, u.RouteMenuInput("north", true), "the line runs as a command")
	require.False(t, chosen)
	require.False(t, u.HasOpenMenu(), "the menu closed")
}
