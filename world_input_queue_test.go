package main

import (
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Each player's client may have at most Network.MaxPendingInputs commands
// waiting, and runs at most one per tick (owner rulings 2026-11-25).

// testWorld is a World with the maps NewWorld makes, without registering its
// listeners. processInput logs a missing user, so the logger is set up.
func testWorld() *World {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	return &World{
		ignoreInput:           map[int]uint64{},
		userInputEventTracker: map[int]struct{}{},
		mobInputEventTracker:  map[int]struct{}{},
	}
}

// newPass is what EventLoop does after every pass: the per-pass gate clears.
// The per-tick pace must hold across it.
func newPass(w *World) {
	clear(w.userInputEventTracker)
	clear(w.mobInputEventTracker)
}

// Every line a connection sends is a command from the player's client.
func TestInputEventFor_IsFromTheClient(t *testing.T) {
	ev := inputEventFor(WorldInput{FromId: 7, InputText: "look"})
	require.True(t, ev.Flags.Has(events.CmdFromClient))
	require.True(t, ev.Flags.Has(events.CmdTyped))
}

// One client command per player per tick: a second in the same tick waits,
// even in a later event pass, and runs in the next tick. An engine command
// and another player's command are not held by it. The player has no record
// (as after a disconnect), so processInput returns at once.
func TestHandleInputEvents_OneClientCommandPerTick(t *testing.T) {
	w := testWorld()
	turn := util.GetTurnCount()
	client := func(uid int) events.Input {
		return events.Input{UserId: uid, ReadyTurn: turn, Flags: events.CmdTyped | events.CmdFromClient}
	}

	require.Equal(t, events.Continue, w.HandleInputEvents(client(98510)))
	newPass(w)
	require.Equal(t, events.CancelAndRequeue, w.HandleInputEvents(client(98510)), "a second in the same tick waits")
	newPass(w)
	require.Equal(t, events.CancelAndRequeue, w.HandleInputEvents(client(98510)), "still the same tick")

	newPass(w)
	require.Equal(t, events.Continue, w.HandleInputEvents(events.Input{UserId: 98510, ReadyTurn: turn}),
		"an engine command is not paced by the client")
	newPass(w)
	require.Equal(t, events.Continue, w.HandleInputEvents(client(98511)), "another player runs")

	turn = util.IncrementTurnCount()
	newPass(w)
	require.Equal(t, events.Continue, w.HandleInputEvents(client(98510)), "the next tick runs it")
}

// The pace holds on the ReadyTurn 0 path too, which skips the per-pass gate.
func TestHandleInputEvents_PaceHoldsForReadyNow(t *testing.T) {
	w := testWorld()
	in := events.Input{UserId: 98512, ReadyTurn: 0, Flags: events.CmdFromClient}
	require.Equal(t, events.Continue, w.HandleInputEvents(in))
	require.Equal(t, events.CancelAndRequeue, w.HandleInputEvents(in))
}

// End to end through the queue and the world's handler: three lines queued at
// once run one per tick, and the player's pending count follows them down to
// zero. Nobody holds the user id (a player who disconnected with commands
// waiting): every command still pops, and none stays counted.
func TestInputQueue_DrainsOnePerTickToZero(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 10
	configs.SetConfigForTest(t, cfg)

	restore := events.SwapListenersForTest()
	t.Cleanup(func() { restore(); events.DrainAllQueuedEventsForTest() })
	events.DrainAllQueuedEventsForTest()

	w := testWorld()
	events.RegisterListener(events.Input{}, w.HandleInputEvents)
	pass := func() { w.EventLoop() }

	const uid = 98513
	for range 3 {
		events.AddToQueue(inputEventFor(WorldInput{FromId: uid, InputText: "look"}))
	}
	require.Equal(t, 3, events.PendingInputs(uid))

	pass()
	require.Equal(t, 2, events.PendingInputs(uid), "one ran this tick")
	pass()
	pass()
	require.Equal(t, 2, events.PendingInputs(uid), "more passes in the same tick run nothing")

	util.IncrementTurnCount()
	pass()
	require.Equal(t, 1, events.PendingInputs(uid))
	util.IncrementTurnCount()
	pass()
	require.Equal(t, 0, events.PendingInputs(uid), "the count returns to zero")
	pass()
	require.Equal(t, 0, events.PendingInputs(uid))
}

// orderFixture wires the world's Input handler into the queue and records
// the text of each command it runs (Continue), so a test can read the order.
func orderFixture(t *testing.T, cap int) (w *World, ran *[]string) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = configs.ConfigInt(cap)
	configs.SetConfigForTest(t, cfg)

	restore := events.SwapListenersForTest()
	t.Cleanup(func() { restore(); events.DrainAllQueuedEventsForTest() })
	events.DrainAllQueuedEventsForTest()

	w = testWorld()
	got := []string{}
	events.RegisterListener(events.Input{}, func(e events.Event) events.ListenerReturn {
		r := w.HandleInputEvents(e)
		if r == events.Continue {
			got = append(got, e.(events.Input).InputText)
		}
		return r
	})
	return w, &got
}

// Waiting commands run oldest first. Four lines arrive in four passes of one
// tick; over the next ticks they run c1 c2 c3 c4. A deferred command keeps
// its place in the queue rather than going to the back each pass.
func TestInputQueue_WaitingCommandsRunInOrder(t *testing.T) {
	w, ran := orderFixture(t, 10)
	const uid = 98516
	for _, c := range []string{"c1", "c2", "c3", "c4"} {
		events.AddToQueue(inputEventFor(WorldInput{FromId: uid, InputText: c}))
		w.EventLoop()
	}
	for range 4 {
		util.IncrementTurnCount()
		w.EventLoop()
		w.EventLoop()
	}
	require.Equal(t, []string{"c1", "c2", "c3", "c4"}, *ran)
}

// Under steady input (a new line every pass, past the cap) the oldest
// waiting line still runs each tick: nothing newer overtakes it.
func TestInputQueue_OldestIsNotStarved(t *testing.T) {
	w, ran := orderFixture(t, 4)
	const uid = 98517
	sent := 0
	for range 5 {
		util.IncrementTurnCount()
		for range 3 {
			sent++
			events.AddToQueue(inputEventFor(WorldInput{FromId: uid, InputText: "c" + strconv.Itoa(sent)}))
			w.EventLoop()
		}
	}
	require.Equal(t, []string{"c1", "c2", "c3", "c4", "c5"}, *ran,
		"one per tick, oldest first (lines past the cap were refused)")
}

// inputQueueFixture is a live human-port connection and a user with a
// four-piece macro, the cap pinned to cap.
func inputQueueFixture(t *testing.T, userId int, cap int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = configs.ConfigInt(cap)
	configs.SetConfigForTest(t, cfg)

	server, client := net.Pipe()
	cd := connections.Add(server, nil, connections.ConnHuman)
	id := cd.ConnectionId()
	t.Cleanup(func() { _ = connections.Remove(id); _ = client.Close() })
	go func() { _, _ = io.Copy(io.Discard, client) }()

	u := users.NewTestUser(userId, "queueuser", "Queueuser", id)
	u.Macros = map[string]string{`=1`: `look;look;look;look`}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))

	events.DrainQueuedByTypeForTest(`Input`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`Input`) })
}

// Each piece of a macro the client named counts, and pieces past the cap are
// refused.
func TestProcessInput_MacroPiecesCountAgainstTheCap(t *testing.T) {
	inputQueueFixture(t, 98514, 3)
	(&World{}).processInput(98514, `=1`, events.CmdTyped|events.CmdFromClient, 0)

	queued := events.DrainQueuedByTypeForTest(`Input`)
	require.Len(t, queued, 3, "a cap of 3 takes three of the four pieces")
	for _, e := range queued {
		require.True(t, e.(events.Input).Flags.Has(events.CmdFromClient))
	}
}

// A macro saved before set refused macro names inside one (" =1" passed the
// old check) calls itself. Its pieces carry CmdMacroPiece and never expand a
// macro, so it runs once and the player's queue drains instead of staying
// full for good.
func TestProcessInput_AMacroPieceNeverExpandsAMacro(t *testing.T) {
	inputQueueFixture(t, 98518, 10)
	u := users.GetByUserId(98518)
	u.Macros = map[string]string{`=1`: `=1; =1`}
	// The pieces run as commands, which need a room and the keywords.
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{98518: {RoomId: 98518}}, map[string]*rooms.ZoneConfig{}))
	u.Character.RoomId = 98518

	(&World{}).processInput(98518, `=1`, events.CmdTyped|events.CmdFromClient, 0)
	pieces := events.DrainQueuedByTypeForTest(`Input`)
	require.Len(t, pieces, 2, "the line expands once")
	for _, p := range pieces {
		require.True(t, p.(events.Input).Flags.Has(events.CmdMacroPiece))
	}

	// Each piece runs as a plain command, not as the macro again.
	for _, p := range pieces {
		in := p.(events.Input)
		(&World{}).processInput(98518, in.InputText, in.Flags, 0)
	}
	for _, e := range events.DrainQueuedByTypeForTest(`Input`) {
		require.False(t, e.(events.Input).Flags.Has(events.CmdMacroPiece), "a piece expanded the macro again: %v", e)
	}
	require.Equal(t, 0, events.PendingInputs(98518), "nothing of the client's is left waiting")
}

// A macro the engine ran (no CmdFromClient) is not the client's: its pieces
// neither count nor stop at the cap.
func TestProcessInput_EngineMacroPiecesDoNotCount(t *testing.T) {
	inputQueueFixture(t, 98515, 3)
	(&World{}).processInput(98515, `=1`, 0, 0)

	require.Len(t, events.DrainQueuedByTypeForTest(`Input`), 4)
}

// Telnet no longer drops a line typed within TurnMs of the last: both read
// loops send every line, and the world paces it.
func TestTelnetReadLoops_NoLongerDropFastLines(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	require.NotRegexp(t, regexp.MustCompile(`time\.Since\(lastInput\)`), string(src))
}
