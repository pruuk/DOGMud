package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// Plan 6 (spec 2026-11-14, R6, R7): a web client line's number rides from the
// websocket to processInput, which marks the line done after all its text.

func TestInputEventFor_CarriesTheLineNumber(t *testing.T) {
	ev := inputEventFor(WorldInput{FromId: 7, InputText: "look", Seq: 12})
	require.Equal(t, 7, ev.UserId)
	require.Equal(t, "look", ev.InputText)
	require.Equal(t, uint64(12), ev.Seq)
	require.True(t, ev.Flags.Has(events.CmdTyped))
}

func TestInputEventFor_TelnetLineHasNoNumber(t *testing.T) {
	ev := inputEventFor(WorldInput{FromId: 7, InputText: "look"})
	require.Equal(t, uint64(0), ev.Seq)
}

func queuedOurs(t *testing.T) []string {
	t.Helper()
	var order []string
	for _, ty := range events.QueuedTypesInOrderForTest() {
		if ty == `InputDone` || ty == `Message` {
			order = append(order, ty)
		}
	}
	return order
}

// The marker pops after every line of text the command queued, even text
// queued after it in the same pass, so the client reads the state the
// command left.
func TestQueueInputDone_PopsAfterTheCommandsText(t *testing.T) {
	events.DrainQueuedByTypeForTest(`InputDone`)
	events.DrainQueuedByTypeForTest(`Message`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`InputDone`)
		events.DrainQueuedByTypeForTest(`Message`)
	})

	queueInputDone(7, 3)
	events.AddToQueue(events.Message{UserId: 7, Text: "You look around."})
	events.AddToQueue(events.Message{UserId: 7, Text: "A menu shown last."}, 50)

	require.Equal(t, []string{`Message`, `Message`, `InputDone`}, queuedOurs(t))
	done := events.DrainQueuedByTypeForTest(`InputDone`)
	require.Len(t, done, 1)
	require.Equal(t, events.InputDone{UserId: 7, Seq: 3}, done[0])
}

// processInput needs a running world, so its wiring is held by its source:
// every typed line, telnet included, defers the done marker at the top, and
// both callers pass the event's number.
func TestProcessInput_DefersTheDoneMarker(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	s := string(src)
	require.Regexp(t, regexp.MustCompile(
		`func \(w \*World\) processInput\(userId int, inputText string, flags events\.EventFlag, seq uint64\) \{`), s)
	require.Regexp(t, regexp.MustCompile(
		`typed := flags\.Has\(events\.CmdTyped\)\s+`+
			`(//[^\n]*\n\s*)*`+
			`if typed \{\s+(//[^\n]*\n\s*)*defer queueInputDone\(userId, seq\)\s+\}`), s)
	require.Equal(t, 2, len(regexp.MustCompile(`w\.processInput\(input\.UserId, input\.InputText, [^\n]*, input\.Seq\)`).FindAllString(s, -1)),
		"both callers pass input.Seq")
}

// #842: a line the pager takes (Enter or q at a stop) opens no reply window,
// or another player's line to this player in the same pass would be held as
// its reply and wipe the waiting pages. The window opens once, after the
// pager has passed the line on, before the menu reads it.
func TestProcessInput_OpensTheReplyAfterThePager(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	s := string(src)
	require.Equal(t, 1, strings.Count(s, "events.OpenReply(userId)"), "processInput opens the window once")
	openAt := strings.Index(s, "events.OpenReply(userId)")
	pagerAt := strings.Index(s, "user.RoutePagerInput(inputText)")
	menuAt := strings.Index(s, "user.RouteMenuInput(inputText, typed)")
	require.Greater(t, pagerAt, 0, "processInput routes to the pager")
	require.Greater(t, openAt, pagerAt, "the window opens after the pager has passed the line on")
	require.Greater(t, menuAt, openAt, "the window opens before the menu reads the line")
}

// #842, owner ruling 8: the login burst is never paged. Every login,
// reconnect and copyover reconnect enters the world through enterWorld, whose
// PlayerSpawn queues the login commands and the first look for the next
// event pass. A typed line handled in that pass ahead of them (one that
// reached the queue before them) has its reply window open while they run;
// entering the world must keep their text out of that reply.
func TestEnterWorld_ItsBurstIsNotAReply(t *testing.T) {
	const userId = 984201
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	(&World{}).enterWorld(userId, 0)
	events.OpenReply(userId) // the typed line ahead of the login commands
	events.AddToQueue(events.Message{UserId: userId, Text: "the message of the day\n"})
	events.CloseReply(userId)

	got := events.DrainQueuedMessageEventsForTest(userId)
	require.Len(t, got, 1)
	require.False(t, got[0].Reply, "text queued while entering the world is not a reply")
}

// Both ends count the same frames: the client counts every string it sends,
// and its GMCP goes as binary, so only text frames advance the count (R7).
func TestCountLineFrame_CountsTextFramesOnly(t *testing.T) {
	require.Equal(t, uint64(1), countLineFrame(websocket.TextMessage, 0))
	require.Equal(t, uint64(5), countLineFrame(websocket.TextMessage, 4))
	require.Equal(t, uint64(4), countLineFrame(websocket.BinaryMessage, 4))
}

// The count must advance before anything in the loop can skip the frame
// (the copyover token, an input handler), and the websocket's line carries it.
func TestWebSocketLoop_CountsEveryFrameAndPassesTheNumber(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	s := string(src)
	require.Regexp(t, regexp.MustCompile(
		`msgType, message, err := conn\.ReadMessage\(\)`), s)
	readIdx := regexp.MustCompile(`msgType, message, err := conn\.ReadMessage\(\)`).FindStringIndex(s)
	require.NotNil(t, readIdx)
	countAt := regexp.MustCompile(`lineSeq = countLineFrame\(msgType, lineSeq\)`).FindStringIndex(s)
	require.NotNil(t, countAt)
	tokenIdx := regexp.MustCompile(`copyover\.ConsumeReconnectToken`).FindStringIndex(s)
	require.NotNil(t, tokenIdx)
	require.True(t, readIdx[0] < countAt[0] && countAt[0] < tokenIdx[0],
		"count each frame right after the read, before the copyover token check")
	require.Len(t, regexp.MustCompile(
		`WorldInput\{\s*FromId:\s+userObject\.UserId,\s*InputText:\s+string\(clientInput\.Buffer\),\s*Seq:\s+lineSeq,`).FindAllString(s, -1), 1,
		"the websocket's WorldInput carries lineSeq")
}

// The input worker must build its event through inputEventFor, or Seq is lost.
func TestInputWorker_UsesInputEventFor(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	require.Len(t, regexp.MustCompile(`events\.AddToQueue\(inputEventFor\(wi\)\)`).FindAllString(string(src), -1), 1)
}

// #842: at a page stop the pager reads a typed line before the menu does, so
// Enter shows the next page instead of picking the menu's default.
func TestProcessInput_PagerReadsBeforeTheMenu(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	s := string(src)
	pagerAt := strings.Index(s, "user.RoutePagerInput(inputText)")
	menuAt := strings.Index(s, "user.RouteMenuInput(inputText, typed)")
	require.Greater(t, pagerAt, 0, "processInput routes to the pager")
	require.Greater(t, menuAt, pagerAt, "the pager reads the line before the menu")
}
