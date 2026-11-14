package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
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

	require.Equal(t, []string{`Message`, `InputDone`}, queuedOurs(t))
	done := events.DrainQueuedByTypeForTest(`InputDone`)
	require.Len(t, done, 1)
	require.Equal(t, events.InputDone{UserId: 7, Seq: 3}, done[0])
}

// processInput needs a running world, so its wiring is held by its source:
// the marker is deferred at the top, for typed numbered lines only, and both
// callers pass the event's number.
func TestProcessInput_DefersTheDoneMarker(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	s := string(src)
	require.Regexp(t, regexp.MustCompile(
		`func \(w \*World\) processInput\(userId int, inputText string, flags events\.EventFlag, seq uint64\) \{`), s)
	require.Regexp(t, regexp.MustCompile(
		`typed := flags\.Has\(events\.CmdTyped\)\s+`+
			`(//[^\n]*\n\s*)*`+
			`if typed && seq > 0 \{\s+defer queueInputDone\(userId, seq\)\s+\}`), s)
	require.Equal(t, 2, len(regexp.MustCompile(`w\.processInput\(input\.UserId, input\.InputText, [^\n]*, input\.Seq\)`).FindAllString(s, -1)),
		"both callers pass input.Seq")
}
