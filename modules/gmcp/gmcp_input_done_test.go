package gmcp

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// inputDoneUser seeds one user on a connection that is the web client (web),
// a GMCP telnet client (!web), and has GMCP on.
func inputDoneUser(t *testing.T, userId int, connId uint64, web bool) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(userId, "doneuser", "Doneuser", connId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	s := GMCPSettings{GMCPAccepted: true, HelloReceived: true}
	if web {
		s.Client.Name = `WebClient`
	} else {
		s.Client.Name = `Mudlet`
	}
	gmcpModule.cache.Add(connId, s)
	t.Cleanup(func() { gmcpModule.cache.Remove(connId) })
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	events.DrainQueuedByTypeForTest(`MenuChanged`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`GMCPOut`)
		events.DrainQueuedByTypeForTest(`MenuChanged`)
	})
	return u
}

func TestInputDone_TellsTheWebClientTheLineNumberAndMenuState(t *testing.T) {
	u := inputDoneUser(t, 9841, 98841, true)

	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 4})
	sent := drainGMCPOut(t)
	require.Len(t, sent, 1)
	require.Equal(t, `Char.Input.Done`, sent[0].Module)
	require.Equal(t, map[string]any{"seq": uint64(4), "menu": false}, sent[0].Payload)

	u.StartPrompt(`test`, ``)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 5})
	sent = drainGMCPOut(t)
	require.Len(t, sent, 1)
	require.Equal(t, map[string]any{"seq": uint64(5), "menu": true}, sent[0].Payload,
		"a prompt left open reads as a menu")
}

func TestInputDone_SendsNothingToTelnetOrUnknownUsers(t *testing.T) {
	u := inputDoneUser(t, 9842, 98842, false)
	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 1})
	require.Empty(t, drainGMCPOut(t), "a telnet GMCP client gets nothing")

	inputDoneHandler(events.InputDone{UserId: 0, Seq: 1})
	inputDoneHandler(events.InputDone{UserId: 9899, Seq: 1})
	require.Empty(t, drainGMCPOut(t))
}

func TestInputDone_WebClientWithoutGMCPNegotiatedGetsNothing(t *testing.T) {
	u := inputDoneUser(t, 9844, 98844, true)
	for _, s := range []GMCPSettings{
		{GMCPAccepted: false, HelloReceived: true},
		{GMCPAccepted: true, HelloReceived: false},
	} {
		s.Client.Name = `WebClient`
		gmcpModule.cache.Add(98844, s)
		inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 1})
		require.Empty(t, drainGMCPOut(t))
	}
}

// R7: nothing typed is ever sent back. The event has no field that could
// hold text, and the payload has exactly its two keys.
func TestInputDone_CarriesNoTypedText(t *testing.T) {
	ty := reflect.TypeFor[events.InputDone]()
	for i := 0; i < ty.NumField(); i++ {
		require.NotEqual(t, reflect.String, ty.Field(i).Type.Kind(),
			"InputDone.%s must not hold text", ty.Field(i).Name)
	}
	u := inputDoneUser(t, 9843, 98843, true)
	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 1})
	sent := drainGMCPOut(t)
	require.Len(t, sent, 1)
	keys := []string{}
	for k := range sent[0].Payload.(map[string]any) {
		keys = append(keys, k)
	}
	require.ElementsMatch(t, []string{"seq", "menu"}, keys)
}

// #842: at a page stop the player is expected to press Enter, so the phone
// keyboard stays up as it does for a menu.
func TestInputDone_PagesWaitingReadAsAMenu(t *testing.T) {
	u := inputDoneUser(t, 9845, 98845, true)
	u.PageSize = 10
	for i := 1; i <= 25; i++ {
		u.HoldReply(fmt.Sprintf("line %d\n", i))
	}
	u.EndReply()
	require.True(t, u.PagesWaiting())

	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 6})
	sent := drainGMCPOut(t)
	require.Len(t, sent, 1)
	require.Equal(t, map[string]any{"seq": uint64(6), "menu": true}, sent[0].Payload)
}

// The pager cuts the reply in its own InputDone listener (hooks.Pager_ReplyEnd),
// registered after this module's. The frame must read the pages that
// listener cut in the same pass, so it runs after it whatever the order of
// registration.
func TestInputDone_ReadsThePagesCutInTheSamePass(t *testing.T) {
	u := inputDoneUser(t, 9846, 98846, true)
	u.PageSize = 10
	restore := events.SwapListenersForTest()
	defer restore()
	// gmcp registers at init; hooks later, from main.go.
	registerInputDoneListener()
	events.RegisterListener(events.InputDone{}, hooks.Pager_ReplyEnd)

	for i := 1; i <= 25; i++ {
		u.HoldReply(fmt.Sprintf("line %d\n", i))
	}
	events.DoListeners(events.InputDone{UserId: u.UserId, Seq: 7})
	require.True(t, u.PagesWaiting(), "the pager cut the reply")
	var done []GMCPOut
	for _, g := range drainGMCPOut(t) {
		if g.Module == `Char.Input.Done` {
			done = append(done, g)
		}
	}
	require.Len(t, done, 1)
	require.Equal(t, map[string]any{"seq": uint64(7), "menu": true}, done[0].Payload)
	events.DrainQueuedByTypeForTest(`RedrawPrompt`)
}

// #842: telnet lines and a web client's macro lines are typed but carry no
// number. Their InputDone is the pager's; the web client is not told.
func TestInputDone_LineWithNoNumberSendsNothing(t *testing.T) {
	u := inputDoneUser(t, 9849, 98849, true)
	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 0})
	require.Empty(t, drainGMCPOut(t))
}
