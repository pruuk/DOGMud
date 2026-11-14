package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

var (
	queueCallRe = regexp.MustCompile(`(?:AddToQueue|sendTextAt)\((.*),\s*(-?[A-Za-z0-9_.]+)\)\s*$`)
	prioConstRe = regexp.MustCompile(`\bconst\s+(\w+)\s*=\s*(\d+)\b`)
)

// priorityConsts reads every `const Name = <int>` in the given sources.
func priorityConsts(sources map[string]string) map[string]int {
	out := map[string]int{}
	for _, src := range sources {
		for _, m := range prioConstRe.FindAllStringSubmatch(src, -1) {
			n, _ := strconv.Atoi(m[2])
			out[m[1]] = n
		}
	}
	return out
}

// queueOverreaches lists "file:line" of every AddToQueue/sendTextAt call whose
// priority (an integer literal, or an identifier found in consts) is at or
// above limit, except the prompt redraw and InputDone itself, which are meant
// to pop at or after it. Unresolvable identifiers are skipped.
func queueOverreaches(sources map[string]string, consts map[string]int, limit int) []string {
	var bad []string
	for file, src := range sources {
		for i, line := range strings.Split(src, "\n") {
			m := queueCallRe.FindStringSubmatch(line)
			if m == nil || strings.Contains(m[1], "RedrawPrompt{") || strings.Contains(m[1], "InputDone{") {
				continue
			}
			prio, err := strconv.Atoi(m[2])
			if err != nil {
				name := m[2][strings.LastIndex(m[2], ".")+1:]
				var ok bool
				if prio, ok = consts[name]; !ok {
					continue
				}
			}
			if prio >= limit {
				bad = append(bad, fmt.Sprintf("%s:%d: priority %d >= %d", file, i+1, prio, limit))
			}
		}
	}
	sort.Strings(bad)
	return bad
}

// The guard can fail: a sample with priority 99 literal and a const that
// resolves to 100 is caught, while the redraw and InputDone lines are not.
func TestQueueOverreaches_CatchesASampleThatMustFail(t *testing.T) {
	sample := map[string]string{"a.go": "const loud = 100\n" +
		"events.AddToQueue(events.Message{UserId: 1}, 99)\n" +
		"u.sendTextAt(cat, txt, loud)\n" +
		"events.AddToQueue(events.Message{UserId: 1}, 50)\n" +
		"events.AddToQueue(events.RedrawPrompt{UserId: 1}, 100)\n" +
		"events.AddToQueue(events.InputDone{UserId: 1}, events.InputDonePriority)\n"}
	bad := queueOverreaches(sample, priorityConsts(sample), 99)
	require.Equal(t, []string{"a.go:2: priority 99 >= 99", "a.go:3: priority 100 >= 99"}, bad)
}

// Nothing may queue at or above InputDonePriority except the prompt redraw, or
// the done marker could pop before a command's last text or menu event.
func TestInputDonePriority_NothingElseQueuesAtOrAbove(t *testing.T) {
	sources := map[string]string{}
	for _, root := range []string{"internal", "modules", "."} {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if root == "." && p != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				b, rerr := os.ReadFile(p)
				if rerr == nil {
					sources[filepath.ToSlash(p)] = string(b)
				}
			}
			return nil
		})
	}
	require.Greater(t, len(sources), 100, "the scan must actually find the source")
	bad := queueOverreaches(sources, priorityConsts(sources), events.InputDonePriority)
	require.Empty(t, bad, "these queue at or above InputDonePriority:\n%s", strings.Join(bad, "\n"))
}
