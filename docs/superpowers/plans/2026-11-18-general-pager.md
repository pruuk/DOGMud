# General Pager Implementation Plan (#842)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Any reply to a player's typed line that is longer than their screen is shown a page at a time, with Enter for the next page, while combat and other unprompted text are never held.

**Architecture:** `internal/events` marks every `Message` queued for a player while their typed line is being handled (from `processInput` until that line's `InputDone` pops). `Message_SendMessage` holds marked messages on the user record instead of sending them; a new `InputDone` listener sends the reply whole or as a first page, and the prompt becomes a stop line while pages wait. Page cutting and sizing live in a new pure package, `internal/pager`.

**Tech Stack:** Go (server), plain JavaScript (web client, `node` tests in `tools/jstest`).

**Spec:** `docs/superpowers/specs/2026-11-18-general-pager-design.md` (read it first; its fact table has the line references).

---

## Working rules for every task

- Edit files with the Edit and Write tools only. Never `sed -i`, never a shell redirect into a tracked file, never a Python read-modify-write. The hooks block these.
- `git add` named paths only; never `-A`, `.`, or `commit -a`. Never `--no-verify`. Never `git stash`.
- Commit messages through a quoted heredoc (`git commit -F - <<'EOF'`), ending with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Go tests: `go test ./internal/<pkg>/ -run <Name> -count=1`. The root package (`world*.go`) tests: `go test . -run <Name> -count=1`.
- JS tests: `node tools/jstest/<file>.test.js`.
- Lint before the final commit of each task: `bash tools/lint.sh` (pinned golangci-lint). Fix what it reports.

## File map

| File | Change |
|---|---|
| `internal/pager/pager.go` | New. `Size`, `Lines`, `Cut`, `Join`, `StopLine`, constants |
| `internal/pager/pager_test.go` | New |
| `internal/pager/context.md` | New |
| `internal/events/eventtypes.go` | `Message.Reply` |
| `internal/events/events.go` | reply window, marking in `AddToQueue` |
| `internal/events/reply_window_test.go` | New |
| `world.go` | open the window and queue `InputDone` for every typed line; route pager input |
| `world_input_done_test.go` | source pins updated |
| `modules/gmcp/gmcp.InputDone.go` | skip `Seq` 0 |
| `modules/gmcp/gmcp_input_done_test.go` | `Seq` 0 case |
| `internal/users/userrecord.go` | `PageSize` field, `pager` state field |
| `internal/users/userrecord.pager.go` | New. hold, end, route, flush, stop line |
| `internal/users/userrecord_pager_test.go` | New |
| `internal/users/userrecord.prompt.go` | stop line replaces the prompt |
| `internal/hooks/Message_SendMessages.go` | hold marked messages, flush pages in combat |
| `internal/hooks/Pager_ReplyEnd.go` | New. `InputDone` listener |
| `internal/hooks/hooks.go` | register it |
| `internal/hooks/pager_reply_test.go` | New |
| `internal/users/wrapwidth.go` | `StoreReportedRows` |
| `modules/gmcp/gmcp.ClientDisplay.go` | read and store `rows` |
| `modules/gmcp/gmcp_client_display_rows_test.go` | New |
| `_datafiles/html/public/static/js/displayreport.js` | report cols and rows |
| `_datafiles/html/public/static/js/termfit.js` | `onRows` |
| `_datafiles/html/public/webclient-pure.html` | wire rows |
| `tools/jstest/displayreport.test.js`, `tools/jstest/termfit.test.js` | updated |
| `internal/usercommands/set.go` | `set pagesize` |
| `internal/usercommands/set_pagesize_test.go` | New |
| `_datafiles/world/dogmud/templates/help/set.template` | `set pagesize` |
| `internal/usercommands/conversation_pager_mark_test.go` | New. Toke-sized talk is fully marked |
| `internal/users/context.md`, `internal/events/context.md`, `internal/hooks/context.md`, `modules/gmcp/context.md` | documented |
| `docs/README.md` | rows for the plan and `internal/pager/context.md` |

---

### Task 1: The pure pager package

**Files:**
- Create: `internal/pager/pager.go`, `internal/pager/pager_test.go`, `internal/pager/context.md`

- [ ] **Step 1: Write the failing tests**

`internal/pager/pager_test.go`:

```go
package pager

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	cases := []struct {
		name               string
		reported, setting  int
		screenReader       bool
		wantSize           int
		wantOn             bool
	}{
		{"off wins over everything", 50, SettingOff, true, 0, false},
		{"a setting wins over the screen", 50, 30, false, 30, true},
		{"a setting wins for a screen reader too", 50, 30, true, 30, true},
		{"a screen reader without a setting gets the fixed size", 50, 0, true, ScreenReaderSize, true},
		{"the screen less the stop line and prompt", 24, 0, false, 22, true},
		{"never below the floor", 6, 0, false, MinSize, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, on := Size(c.reported, c.setting, c.screenReader)
			require.Equal(t, c.wantOn, on)
			require.Equal(t, c.wantSize, size)
		})
	}
}

func TestLines(t *testing.T) {
	require.Equal(t, []string{"a", "", "b"}, Lines("a\n\nb\n"))
	require.Equal(t, []string{"a"}, Lines("a"))
	require.Empty(t, Lines(""))
}

func TestCut(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}
	require.Equal(t, [][]string{{"1", "2"}, {"3", "4"}, {"5"}}, Cut(lines, 2))
	require.Equal(t, [][]string{{"1", "2", "3", "4", "5"}}, Cut(lines, 5), "an exact fit is one page")
	require.Equal(t, [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}}, Cut(lines, 0), "a size below 1 counts as 1")
	require.Equal(t, [][]string{{"1", "2"}, {"3"}}, Cut([]string{"1", "2", "", "3"}, 2),
		"a blank line is not carried to the top of the next page")
	require.Empty(t, Cut(nil, 3))
}

func TestJoin(t *testing.T) {
	require.Equal(t, "a\nb\n"+resetColour, Join([]string{"a", "b"}))
}

func TestStopLine(t *testing.T) {
	s := StopLine(1, 3)
	require.Equal(t, "More (page 1 of 3): press Enter for the next page, or type q to stop. ", s)
	require.False(t, strings.HasSuffix(s, "\n"), "the stop line is a prompt, with no line break")
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/pager/ -count=1`
Expected: FAIL, the package has no non-test Go files / undefined names.

- [ ] **Step 3: Write the package**

`internal/pager/pager.go`:

```go
// Package pager cuts a long reply into pages that fit a reader's screen
// (#842). It is pure: no I/O and no state. internal/users keeps the pages
// and internal/hooks sends them.
package pager

import (
	"fmt"
	"strings"
)

const (
	// SettingOff is the saved page size that turns paging off
	// (UserRecord.PageSize; 0 follows the screen).
	SettingOff = -1
	// ScreenReaderSize is the page size of a screen reader account with no
	// setting of its own: long enough for a menu and its NPC's words, short
	// enough to reach the choices without a long wait.
	ScreenReaderSize = 20
	// MinSize is the smallest page, however small the screen reports.
	MinSize = 10
	// MaxSize is the largest page a setting or a report may ask for.
	MaxSize = 200
	// reservedRows are the rows a page leaves for the stop line and the
	// prompt.
	reservedRows = 2
	// resetColour ends a page, so a colour opened on its last line does not
	// run into the stop line.
	resetColour = "\x1b[0m"
)

// Size is the page size for a reader and whether paging is on. setting is
// UserRecord.PageSize: SettingOff, 0 to follow the screen, or a size.
// reported is the screen's height in rows.
func Size(reported, setting int, screenReader bool) (int, bool) {
	switch {
	case setting == SettingOff:
		return 0, false
	case setting > 0:
		return setting, true
	case screenReader:
		return ScreenReaderSize, true
	}
	return max(reported-reservedRows, MinSize), true
}

// Lines splits sent text into its screen lines. The text's own final line
// break does not make an extra empty line.
func Lines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// Cut cuts lines into pages of at most size lines. A blank line that would
// open a page is dropped, so no page starts with empty space.
func Cut(lines []string, size int) [][]string {
	size = max(size, 1)
	var pages [][]string
	for len(lines) > 0 {
		n := min(size, len(lines))
		pages = append(pages, lines[:n])
		lines = lines[n:]
		for len(lines) > 0 && lines[0] == "" {
			lines = lines[1:]
		}
	}
	return pages
}

// Join is the text to send for a page.
func Join(page []string) string {
	return strings.Join(page, "\n") + "\n" + resetColour
}

// StopLine is the prompt shown while pages wait: page is the page just
// shown (from 1), of the number of pages. Plain words, so a screen reader
// reads it as it is.
func StopLine(page, of int) string {
	return fmt.Sprintf("More (page %d of %d): press Enter for the next page, or type q to stop. ", page, of)
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/pager/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Write `internal/pager/context.md`**

```markdown
# pager: Package Documentation

## Purpose

`internal/pager` cuts a long reply into pages that fit the reader's screen
(#842, spec `docs/superpowers/specs/2026-11-18-general-pager-design.md`). It
is pure: no I/O, no locks, no state. `internal/users` keeps a player's
pages (`userrecord.pager.go`) and `internal/hooks` sends them.

## Surface

- `Size(reported, setting int, screenReader bool) (int, bool)`: the page size
  and whether paging is on. `setting` is `UserRecord.PageSize`:
  `SettingOff` (-1) turns paging off, a positive number is the size, 0
  follows the screen. With no setting a screen reader account gets
  `ScreenReaderSize` (20); everyone else gets the reported height less 2
  (the stop line and the prompt), never below `MinSize` (10). `MaxSize`
  (200) bounds a setting or a report.
- `Lines(text) []string`: sent text as screen lines; the final line break
  does not add an empty line.
- `Cut(lines, size) [][]string`: pages of at most `size` lines; a blank line
  that would open a page is dropped.
- `Join(page) string`: the text to send for a page, ending with a colour
  reset so a colour left open does not reach the stop line.
- `StopLine(page, of) string`: the prompt while pages wait, "More (page 1
  of 3): press Enter for the next page, or type q to stop." No line break,
  so clients treat it as a prompt.

Lines are counted as the server wrapped them (to `WrapWidth`); a telnet
client narrower than that wraps again and shows more rows than counted.
```

- [ ] **Step 6: Add the docs row**

In `docs/README.md`, after the row for `../internal/menu/context.md` (line 80), add:

```markdown
| [`../internal/pager/context.md`](../internal/pager/context.md) | The pager (#842): `Size` (screen height less 2, `set pagesize`, 20 for screen readers, off), `Lines`, `Cut`, `Join`, `StopLine`; pure, used by `internal/users` and `internal/hooks` |
```

- [ ] **Step 7: Commit**

```bash
git add internal/pager/pager.go internal/pager/pager_test.go internal/pager/context.md docs/README.md
git commit -F - <<'EOF'
feat(pager): pure page cutting and sizing (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Mark the reply in the event queue

**Files:**
- Modify: `internal/events/eventtypes.go:213-220` (the `Message` struct)
- Modify: `internal/events/events.go` (vars near line 15-26; `AddToQueue` at line 90)
- Create: `internal/events/reply_window_test.go`
- Modify: `internal/events/context.md`

- [ ] **Step 1: Write the failing test**

`internal/events/reply_window_test.go`:

```go
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
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/events/ -run TestReplyWindow -count=1`
Expected: FAIL, `undefined: OpenReply` and `mine[0].Reply undefined`.

- [ ] **Step 3: Add the field**

In `internal/events/eventtypes.go`, in `type Message struct`, after the `IsCommunication` line add:

```go
	// Reply is set by AddToQueue, never by a sender: the message was queued
	// for UserId while their typed line was being handled, so it is part of
	// that line's reply and may be paged (#842, OpenReply).
	Reply bool
```

- [ ] **Step 4: Add the window and the marking**

In `internal/events/events.go`, inside the `var ( ... )` block that holds `qLock` (lines 15-26), add before the closing `)`:

```go
	// replyWindow counts, per user, the typed lines whose reply is being
	// built (#842): processInput opens one, the line's InputDone closes it.
	// Guarded by qLock.
	replyWindow = map[int]int{}
```

After `AddToQueue`'s closing brace, add:

```go
// OpenReply starts marking every Message queued for userId as the reply to
// the typed line being handled (#842). world.processInput calls it; the
// pager's InputDone listener (hooks.Pager_ReplyEnd) calls CloseReply.
func OpenReply(userId int) {
	qLock.Lock()
	defer qLock.Unlock()
	replyWindow[userId]++
}

// CloseReply ends one OpenReply for userId. Closing more than was opened
// does nothing.
func CloseReply(userId int) {
	qLock.Lock()
	defer qLock.Unlock()
	if replyWindow[userId] <= 1 {
		delete(replyWindow, userId)
		return
	}
	replyWindow[userId]--
}
```

In `AddToQueue`, directly after the `prio` block (after line 98, before the uniqueness check), add:

```go
	// #842: a message for a player whose typed line is being handled is
	// part of that line's reply. Marked here, the one place every message
	// is queued, so no sender has to know.
	if m, ok := e.(Message); ok && m.UserId > 0 && replyWindow[m.UserId] > 0 {
		m.Reply = true
		e = m
	}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./internal/events/ -count=1`
Expected: `ok`.

- [ ] **Step 6: Document it**

In `internal/events/context.md`, add a section (place it after the section that describes `AddToQueue`; search for `AddToQueue` in the file):

```markdown
### Reply window (#842)

`OpenReply(userId)` and `CloseReply(userId)` keep a per-user count under
`qLock`. While it is above zero, `AddToQueue` sets `Message.Reply` on every
`Message` whose `UserId` is that user. `world.processInput` opens it for a
typed line; the pager's `InputDone` listener closes it. A `Message{RoomId}`
is never marked. No sender sets `Reply` itself.
```

- [ ] **Step 7: Commit**

```bash
git add internal/events/eventtypes.go internal/events/events.go internal/events/reply_window_test.go internal/events/context.md
git commit -F - <<'EOF'
feat(events): mark messages queued during a typed line as its reply (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Every typed line opens the window and gets its done marker

**Files:**
- Modify: `world.go:1007-1012`
- Modify: `world_input_done_test.go:60-74`
- Modify: `modules/gmcp/gmcp.InputDone.go:21`
- Modify: `modules/gmcp/gmcp_input_done_test.go`

- [ ] **Step 1: Update the source pin to the new wiring (failing)**

In `world_input_done_test.go`, replace the comment and the second `require.Regexp` of `TestProcessInput_DefersTheDoneMarker` so the function reads:

```go
// processInput needs a running world, so its wiring is held by its source:
// every typed line, telnet included, opens the reply window (#842) and defers
// the done marker at the top, and both callers pass the event's number.
func TestProcessInput_DefersTheDoneMarker(t *testing.T) {
	src, err := os.ReadFile("world.go")
	require.NoError(t, err)
	s := string(src)
	require.Regexp(t, regexp.MustCompile(
		`func \(w \*World\) processInput\(userId int, inputText string, flags events\.EventFlag, seq uint64\) \{`), s)
	require.Regexp(t, regexp.MustCompile(
		`typed := flags\.Has\(events\.CmdTyped\)\s+`+
			`(//[^\n]*\n\s*)*`+
			`if typed \{\s+(//[^\n]*\n\s*)*events\.OpenReply\(userId\)\s+defer queueInputDone\(userId, seq\)\s+\}`), s)
	require.Equal(t, 2, len(regexp.MustCompile(`w\.processInput\(input\.UserId, input\.InputText, [^\n]*, input\.Seq\)`).FindAllString(s, -1)),
		"both callers pass input.Seq")
}
```

Run: `go test . -run TestProcessInput_DefersTheDoneMarker -count=1`
Expected: FAIL (the regexp does not match `if typed && seq > 0`).

- [ ] **Step 2: Change `processInput`**

In `world.go`, replace:

```go
	// Plan 6 (R6, R7): tell the web client this line is done, after all its
	// text, on every return below. Its number, never its text.
	if typed && seq > 0 {
		defer queueInputDone(userId, seq)
	}
```

with:

```go
	// Plan 6 (R6, R7): mark this line done, after all its text, on every
	// return below; the web client gets its number, never its text. #842:
	// every typed line, telnet included (seq 0), so the pager knows where the
	// reply ends; everything queued for this player until then is the reply.
	if typed {
		events.OpenReply(userId)
		defer queueInputDone(userId, seq)
	}
```

Also update `queueInputDone`'s doc comment (world.go:986-990) first sentence from "queues the marker that the server has finished one typed web client line" to "queues the marker that the server has finished one typed line (web or telnet)".

- [ ] **Step 3: Run the root test**

Run: `go test . -run 'TestProcessInput_DefersTheDoneMarker|TestQueueInputDone|TestInputDonePriority' -count=1`
Expected: `ok`.

- [ ] **Step 4: gmcp skips a line with no number (failing test first)**

Append to `modules/gmcp/gmcp_input_done_test.go` (it uses the file's own `inputDoneUser` and `drainGMCPOut` helpers):

```go
// #842: telnet lines and a web client's macro lines are typed but carry no
// number. Their InputDone is the pager's; the web client is not told.
func TestInputDone_LineWithNoNumberSendsNothing(t *testing.T) {
	u := inputDoneUser(t, 9849, 98849, true)
	inputDoneHandler(events.InputDone{UserId: u.UserId, Seq: 0})
	require.Empty(t, drainGMCPOut(t))
}
```

Run: `go test ./modules/gmcp/ -run TestInputDone_LineWithNoNumberSendsNothing -count=1`
Expected: FAIL (one `Char.Input.Done` is sent).

- [ ] **Step 5: Skip `Seq` 0**

In `modules/gmcp/gmcp.InputDone.go`, change:

```go
	if !ok || evt.UserId == 0 {
```

to:

```go
	// Seq 0: a telnet line or a web client's macro line (#842). Only the
	// pager needs those; the web client counts only its own lines.
	if !ok || evt.UserId == 0 || evt.Seq == 0 {
```

Run: `go test ./modules/gmcp/ -count=1`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add world.go world_input_done_test.go modules/gmcp/gmcp.InputDone.go modules/gmcp/gmcp_input_done_test.go
git commit -F - <<'EOF'
feat(world): every typed line opens the reply window and is marked done (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Pager state on the user record

**Files:**
- Modify: `internal/users/userrecord.go` (fields near line 57 and line 82)
- Create: `internal/users/userrecord.pager.go`
- Create: `internal/users/userrecord_pager_test.go`
- Modify: `internal/users/userrecord.prompt.go:357-367`

- [ ] **Step 1: Write the failing tests**

`internal/users/userrecord_pager_test.go`:

```go
package users

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/require"
)

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
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/users/ -run 'TestEndReply|TestRoutePagerInput|TestFlushPagesInCombat|TestGetCommandPrompt_IsTheStopLine' -count=1`
Expected: FAIL, `u.PageSize undefined`, `u.HoldReply undefined`.

- [ ] **Step 3: Add the fields**

In `internal/users/userrecord.go`, after the `LineWidth` field (line 57) add:

```go
	PageSize        int                   `yaml:"pagesize,omitempty"`        // Lines per page of a long reply (#842): 0 follows the screen, -1 (pager.SettingOff) is off
```

(align the tags with the surrounding fields as gofmt leaves them.)

After the `activeMenu` field (line 82) add:

```go
	pager          pagerState // the reply being built and the pages waiting (#842); guarded by sessionMu
```

- [ ] **Step 4: Write `internal/users/userrecord.pager.go`**

```go
package users

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/pager"
)

// pagerState is one player's reply being built and the pages still to show
// (#842). held collects the text of the messages marked as the reply
// (events.Message.Reply) until the line's InputDone; pages and next are the
// cut reply while the player reads it. Guarded by sessionMu.
type pagerState struct {
	held  []string
	pages [][]string
	next  int // index of the next page to show
}

// HoldReply keeps one message of the reply being built. text is what
// Message_SendMessage would have written, without its line-erase prefix.
func (u *UserRecord) HoldReply(text string) {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	u.pager.held = append(u.pager.held, text)
}

// EndReply ends the reply being built and returns the text to write now:
// the whole reply when it fits a page, when paging is off, or while the
// player is in combat (owner ruling 2026-11-18); otherwise the first page,
// with the rest kept for RoutePagerInput. "" when nothing was held, which
// leaves any waiting pages alone. A reply with text replaces waiting pages.
func (u *UserRecord) EndReply() string {
	size, on := pager.Size(u.ClientSettings().Display.GetScreenHeight(), u.PageSize, u.ScreenReader)
	fighting := u.Character != nil && u.Character.IsInCombat()

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	text := strings.Join(u.pager.held, ``)
	u.pager.held = nil
	if text == `` {
		return ``
	}
	u.pager.pages, u.pager.next = nil, 0
	if !on || fighting {
		return text
	}
	lines := pager.Lines(text)
	if len(lines) <= size {
		return text
	}
	u.pager.pages = pager.Cut(lines, size)
	u.pager.next = 1
	return pager.Join(u.pager.pages[0])
}

// PagesWaiting reports whether a reply has pages still to show.
func (u *UserRecord) PagesWaiting() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return len(u.pager.pages) > 0
}

// pagerStopLine is the prompt while pages wait, or "".
func (u *UserRecord) pagerStopLine() string {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``
	}
	return pager.StopLine(u.pager.next, len(u.pager.pages))
}

// RoutePagerInput reads a typed line while pages wait (owner ruling
// 2026-11-18): Enter shows the next page (out) and q drops the rest; both are
// consumed. Any other line drops the rest and is not consumed, so it reaches
// the open menu or runs as a command. With no pages waiting nothing is
// consumed.
func (u *UserRecord) RoutePagerInput(line string) (out string, consumed bool) {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``, false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == `` {
		page := u.pager.pages[u.pager.next]
		u.pager.next++
		if u.pager.next >= len(u.pager.pages) {
			u.pager.pages, u.pager.next = nil, 0
		}
		return pager.Join(page), true
	}
	u.pager.pages, u.pager.next = nil, 0
	return ``, strings.EqualFold(trimmed, `q`)
}

// FlushPagesInCombat returns every waiting page as one text and clears them
// when the player is in combat, so a fight never sits behind a stop (owner
// ruling 2026-11-18). "" otherwise.
func (u *UserRecord) FlushPagesInCombat() string {
	if u.Character == nil || !u.Character.IsInCombat() {
		return ``
	}
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``
	}
	var rest []string
	for _, p := range u.pager.pages[u.pager.next:] {
		rest = append(rest, p...)
	}
	u.pager.pages, u.pager.next = nil, 0
	return pager.Join(rest)
}
```

- [ ] **Step 5: The stop line replaces the prompt**

In `internal/users/userrecord.prompt.go`, `GetCommandPrompt`, directly after the `if u.activePrompt != nil { ... }` block (it ends at line 366) and before the `goAhead` line, add:

```go
	// #842: while a reply's pages wait, the stop line is the prompt, so every
	// redraw (after a combat line, say) puts it back under the text.
	if stop := u.pagerStopLine(); stop != `` {
		promptOut = stop
	}
```

- [ ] **Step 6: Run the tests and see them pass**

Run: `go test ./internal/users/ -count=1`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/users/userrecord.go internal/users/userrecord.pager.go internal/users/userrecord_pager_test.go internal/users/userrecord.prompt.go
git commit -F - <<'EOF'
feat(users): hold a reply, page it, route Enter and q at a stop (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Hold marked messages and send at the reply's end

**Files:**
- Modify: `internal/hooks/Message_SendMessages.go:28-45`
- Create: `internal/hooks/Pager_ReplyEnd.go`
- Modify: `internal/hooks/hooks.go:133-135`
- Create: `internal/hooks/pager_reply_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/hooks/pager_reply_test.go` (uses `captureConnection` and `visibleLastLine` from `redraw_new_prompt_test.go` in the same package):

```go
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
	require.Never(t, func() bool { return strings.Contains(screen.bytes(), "reply 1\n") },
		100*time.Millisecond, 10*time.Millisecond, "nothing of the reply goes out before its end")

	Pager_ReplyEnd(events.InputDone{UserId: 1})
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1})
	require.Eventually(t, func() bool {
		s := screen.bytes()
		return strings.Contains(s, "reply 10\n") && !strings.Contains(s, "reply 11\n") &&
			strings.Contains(visibleLastLine(s), "More (page 1 of 3)")
	}, time.Second, 10*time.Millisecond, "screen %q", screen.bytes())
}

func TestPager_AShortReplyIsSentUnchanged(t *testing.T) {
	_, screen := pagerFixture(t)
	replyOf(3)
	Pager_ReplyEnd(events.InputDone{UserId: 1})
	require.Eventually(t, func() bool {
		return strings.Contains(screen.bytes(), "reply 1\nreply 2\nreply 3\n")
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
		s := screen.bytes()
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
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/hooks/ -run TestPager_ -count=1`
Expected: FAIL, `undefined: Pager_ReplyEnd`.

- [ ] **Step 3: Hold marked messages in `Message_SendMessage`**

In `internal/hooks/Message_SendMessages.go`, in the `if message.UserId > 0` branch, replace:

```go
			connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+textOut), user.ConnectionId())

			events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)
```

with:

```go
			if message.Reply {
				// #842: part of the reply to this player's typed line. Held
				// until the line's InputDone (Pager_ReplyEnd), which sends it
				// whole or a page at a time.
				user.HoldReply(textOut)
			} else {
				// No paging in combat (owner ruling 2026-11-18): a fight that
				// started while pages wait sends them before its first line.
				if rest := user.FlushPagesInCombat(); rest != `` {
					connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+rest), user.ConnectionId())
				}
				connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+textOut), user.ConnectionId())

				events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)
			}
```

The `RoomId` branch is unchanged: a `Message{RoomId}` is never marked.

- [ ] **Step 4: Write `internal/hooks/Pager_ReplyEnd.go`**

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Pager_ReplyEnd ends the reply to one typed line (#842). InputDone pops
// after every event that writes the line's text or opens a menu, so the
// whole reply has been held (Message_SendMessage) by now. It closes the
// reply window, then sends the reply whole, or its first page with the stop
// line drawn as the prompt.
func Pager_ReplyEnd(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.InputDone)
	if !ok {
		mudlog.Error("Event", "Expected Type", "InputDone", "Actual Type", e.Type())
		return events.Continue
	}
	events.CloseReply(evt.UserId)
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	if out := user.EndReply(); out != `` {
		connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+out), user.ConnectionId())
		events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)
	}
	return events.Continue
}
```

- [ ] **Step 5: Register it**

In `internal/hooks/hooks.go`, after `events.RegisterListener(events.Message{}, Message_SendMessage)` (line 133) add:

```go
	// The end of a typed line's reply: send it whole or a page at a time (#842)
	events.RegisterListener(events.InputDone{}, Pager_ReplyEnd)
```

- [ ] **Step 6: Run the hook tests and the whole package**

Run: `go test ./internal/hooks/ -count=1`
Expected: `ok`. If a test elsewhere in `internal/hooks` counts registered listeners, update its expected count by one and say so in the commit message.

- [ ] **Step 7: Document**

In `internal/hooks/context.md`, find the entry for `Message_SendMessage` and add after it:

```markdown
- `Message_SendMessage` holds a `Message` with `Reply` set (#842) on the
  user (`HoldReply`) instead of sending it; any other message first sends the
  pages still waiting if the player is in combat (`FlushPagesInCombat`).
- `Pager_ReplyEnd` (`InputDone`): closes the reply window
  (`events.CloseReply`) and sends `UserRecord.EndReply()`: the whole reply,
  or its first page with the stop line as the next prompt.
```

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/Message_SendMessages.go internal/hooks/Pager_ReplyEnd.go internal/hooks/hooks.go internal/hooks/pager_reply_test.go internal/hooks/context.md
git commit -F - <<'EOF'
feat(hooks): hold a typed line's reply and page it at its end (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Route input at a stop

**Files:**
- Modify: `world.go` (in `processInput`, before `if user.RouteMenuInput(inputText, typed) {` at line 1016)
- Modify: `world_input_done_test.go` (new source pin)

- [ ] **Step 1: Write the failing source pin**

Append to `world_input_done_test.go`:

```go
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
```

Add `"strings"` to the file's imports if it is not already there.

Run: `go test . -run TestProcessInput_PagerReadsBeforeTheMenu -count=1`
Expected: FAIL, "processInput routes to the pager".

- [ ] **Step 2: Route the line**

In `world.go`, directly before the comment `// A typed line goes to an open game menu first (#375).` add:

```go
	// A typed line at a page stop goes to the pager first (#842): Enter shows
	// the next page and q stops; both end here. Any other line has dropped the
	// rest of the reply and goes on to the menu or the command below.
	if typed {
		if out, consumed := user.RoutePagerInput(inputText); consumed {
			if out != `` {
				connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+out), user.ConnectionId())
			}
			connections.SendTo([]byte(templates.AnsiParse(user.GetCommandPrompt())), user.ConnectionId())
			return
		}
	}
```

- [ ] **Step 3: Run the root tests**

Run: `go test . -count=1`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add world.go world_input_done_test.go
git commit -F - <<'EOF'
feat(world): Enter and q at a page stop go to the pager first (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: The web client reports its rows

**Files:**
- Modify: `internal/users/wrapwidth.go` (after `StoreReportedCols`, line 46)
- Modify: `modules/gmcp/gmcp.ClientDisplay.go:40-75`
- Create: `modules/gmcp/gmcp_client_display_rows_test.go`
- Modify: `_datafiles/html/public/static/js/displayreport.js`, `_datafiles/html/public/static/js/termfit.js`, `_datafiles/html/public/webclient-pure.html:562,613,3167`
- Modify: `tools/jstest/displayreport.test.js`, `tools/jstest/termfit.test.js`

- [ ] **Step 1: Failing Go test**

Write `modules/gmcp/gmcp_client_display_rows_test.go` (it uses `clientDisplayConn` and `reportCols` from `gmcp.ClientDisplay_test.go` in the same package):

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

func TestReportedRows(t *testing.T) {
	rows, ok := reportedRows([]byte(`{"cols": 60, "rows": 31}`))
	require.True(t, ok)
	require.Equal(t, 31, rows)
	for _, bad := range []string{`{"cols": 60}`, `{"rows": 3.5}`, `{"rows": "31"}`, `{"rows": null}`, `nope`} {
		_, ok := reportedRows([]byte(bad))
		require.False(t, ok, bad)
	}
}

func TestClientDisplay_StoresRowsClamped(t *testing.T) {
	id := clientDisplayConn(t, 80)
	reportCols(id, `{"cols": 60, "rows": 31}`)
	require.Equal(t, 31, connections.GetClientSettings(id).Display.GetScreenHeight())
	reportCols(id, `{"cols": 60, "rows": 4}`)
	require.Equal(t, 10, connections.GetClientSettings(id).Display.GetScreenHeight(), "clamped to the pager's floor")
	reportCols(id, `{"screenreader": true}`)
	require.Equal(t, 10, connections.GetClientSettings(id).Display.GetScreenHeight(), "a report without rows keeps the height")
}
```

Run: `go test ./modules/gmcp/ -run 'TestReportedRows|TestClientDisplay_StoresRows' -count=1`
Expected: FAIL, `undefined: reportedRows`.

- [ ] **Step 2: Store the rows**

In `internal/users/wrapwidth.go`, after `StoreReportedCols` add (and import `github.com/GoMudEngine/GoMud/internal/pager`):

```go
// StoreReportedRows stores a web client's reported row count as the
// connection's screen height, clamped to pager.MinSize..pager.MaxSize, and
// written only when it differs (#842). Telnet heights come from NAWS instead.
func StoreReportedRows(id connections.ConnectionId, rows int) int {
	height := min(max(rows, pager.MinSize), pager.MaxSize)
	connections.UpdateClientSettings(id, func(cs *connections.ClientSettings) {
		if cs.Display.GetScreenHeight() != height {
			cs.Display.ScreenHeight = uint32(height)
		}
	})
	return height
}
```

In `modules/gmcp/gmcp.ClientDisplay.go`, after `reportedCols` add:

```go
// reportedRows reads {"rows": n} the way reportedCols reads cols (#842).
func reportedRows(payload []byte) (int, bool) {
	var decoded struct {
		Rows *int `json:"rows"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.Rows == nil {
		return 0, false
	}
	return *decoded.Rows, true
}
```

In `clientDisplayHandler`, after the screen reader block (`if hasSR { ... }`) add:

```go
	rows, hasRows := reportedRows(evt.Payload)
	if hasRows {
		users.StoreReportedRows(evt.ConnectionId, rows)
	}
```

and change the logging condition inside `if !ok {` from `if !hasSR || hasColsKey(evt.Payload) {` to `if (!hasSR && !hasRows) || hasColsKey(evt.Payload) {`. Update the handler's doc comment's last sentence to: "It also stores the screenreader flag and the row count the client sends."

Run: `go test ./modules/gmcp/ ./internal/users/ -count=1`
Expected: `ok`.

- [ ] **Step 3: Failing JS tests**

Replace `tools/jstest/displayreport.test.js` from the line `var c = clock(), sent = [], open = true;` to the end with:

```js
var c = clock(), sent = [], open = true;
var r = DisplayReport.create({
    send: function (cols, rows) { if (!open) return false; sent.push(cols + 'x' + rows); return true; },
    setTimeout: c.setTimeout, clearTimeout: c.clearTimeout
});

r.connected(80, 30);
check('connect sends the size at once', sent.join() === '80x30', sent.join());

r.columns(60); c.advance(100); r.columns(50); c.advance(100); r.columns(42);
c.advance(249);
check('nothing is sent while the size is still moving', sent.join() === '80x30', sent.join());
c.advance(1);
check('the settled width is sent once', sent.join() === '80x30,42x30', sent.join());

r.columns(42); c.advance(300);
check('the same size is not sent again', sent.join() === '80x30,42x30', sent.join());

r.rows(18); c.advance(300);
check('a height change alone is sent', sent.join() === '80x30,42x30,42x18', sent.join());

open = false; r.columns(36); c.advance(300);
check('a closed socket sends nothing', sent.join() === '80x30,42x30,42x18', sent.join());
open = true; r.connected(36, 18);
check('a reconnect sends the size though it was sent before', sent.join() === '80x30,42x30,42x18,36x18', sent.join());

process.exit(failures ? 1 : 0);
```

Also change the header comment's first line to `// displayreport.test.js: the web client tells the server its columns and rows`.

In `tools/jstest/termfit.test.js`, in `makePage`, after `onCols: opts.onCols,` add `onRows: opts.onRows,`. Then in `syncChecks`, directly after the line `check('the keyboard still fits the rows', ...);` add:

```js
    var rowsSeen = [];
    var kbRows = makePage(360, 500, { text: 14, minFont: 1, onRows: function (r) { rowsSeen.push(r); } });
    kbRows.fitter.fit();
    var firstRows = kbRows.real.rows;
    kbRows.panel.clientHeight = 300; kbRows.fitter.fit();
    check('the first fit reports the rows', rowsSeen[0] === firstRows, 'rows ' + rowsSeen.join(','));
    check('the keyboard reports the new rows', rowsSeen.length === 2 && rowsSeen[1] === kbRows.real.rows && kbRows.real.rows < firstRows,
        'rows ' + rowsSeen.join(','));
```

Run: `node tools/jstest/displayreport.test.js; node tools/jstest/termfit.test.js`
Expected: FAIL lines (`r.rows is not a function`; no rows reported).

- [ ] **Step 4: `displayreport.js` reports both**

Replace `_datafiles/html/public/static/js/displayreport.js` with:

```js
// displayreport.js: tell the server the game window's columns and rows.
//
// The server wraps every line to the columns and pages long replies by the
// rows (Client.Display, handled in modules/gmcp/gmcp.ClientDisplay.go, which
// clamps cols to 30 to 80 and rows to 10 to 200; #842). A size is sent once
// it has held for 250ms, so a window being dragged does not send every step;
// the same size is not sent twice; and a new connection gets the size at
// once, because the server starts every connection at 80 columns.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;          // node, for the tests
    } else {
        root.DisplayReport = api;      // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var SETTLE_MS = 250;

    // opts: send(cols, rows) (returns false when it could not send), and
    // setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var cols = null, rows = null, lastSent = null, timer = null;

        function flush() {
            timer = null;
            if (cols === null) return;
            var key = cols + 'x' + rows;
            if (key === lastSent) return;
            if (opts.send(cols, rows) !== false) lastSent = key;
        }

        function settle() {
            if (timer) clearT(timer);
            timer = setT(flush, SETTLE_MS);
        }

        return {
            // The column count changed; send it once it settles.
            columns: function (n) { cols = n; settle(); },
            // The row count changed; send it once it settles.
            rows: function (n) { rows = n; settle(); },
            // A new connection: send the current size now.
            connected: function (c, r) {
                if (timer) { clearT(timer); timer = null; }
                lastSent = null;
                cols = c; rows = r;
                flush();
            }
        };
    }

    return { create: create };
}));
```

- [ ] **Step 5: `termfit.js` reports the rows**

In `_datafiles/html/public/static/js/termfit.js`:
- In the `opts` comment (lines 24-28), after `onCols(cols) (told when the column count changes),` add ` onRows(rows) (told when the row count changes, the keyboard included),`.
- Change `var colsChanged = false, cols;` (line 48) to `var colsChanged = false, rowsChanged = false, cols;`.
- Change `colsChanged = cols !== term.cols;` to:

```js
                colsChanged = cols !== term.cols;
                rowsChanged = d.rows !== term.rows;
```

- Replace the listener block at the end of `fit`:

```js
            try {
                if (colsChanged && opts.onCols) opts.onCols(cols);
            } catch (e) {}
```

with:

```js
            try {
                if (colsChanged && opts.onCols) opts.onCols(cols);
            } catch (e) {}
            try {
                if (rowsChanged && opts.onRows) opts.onRows(term.rows);
            } catch (e) {}
```

Note: the first fit of a fresh terminal changes the rows from xterm's starting 24, so the first fit reports them; the test in Step 3 relies on that. If `makePage`'s fake terminal starts at the fitted row count, the first check fails: then change the first check to `rowsSeen.length >= 1` only after confirming in the fake (`makePage`) what `term.rows` starts at, and say so in the commit message.

- [ ] **Step 6: Wire the page**

In `_datafiles/html/public/webclient-pure.html`:
- Line 562: `send: function (cols) { return SendGMCP('Client.Display', { cols: cols, screenreader: screenReader.isOn() }); }` becomes:

```js
            send: function (cols, rows) { return SendGMCP('Client.Display', { cols: cols, rows: rows, screenreader: screenReader.isOn() }); }
```

and the comment above it (`// Tell the server the game window's columns ...`) becomes `// Tell the server the game window's columns and rows (static/js/displayreport.js),`.
- Line 613: after `onCols: function (cols) { displayReport.columns(cols); },` add:

```js
            onRows: function (rows) { displayReport.rows(rows); },
```

- Line 3167: `displayReport.connected(term.cols);` becomes `displayReport.connected(term.cols, term.rows);`.
- The screen reader switch also sends `Client.Display` with only `{screenreader: on}` (line 584). Leave it: a payload with no `rows` keeps the stored height.

- [ ] **Step 7: Run every JS test and the Go tests**

Run: `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED $t"; done`
Expected: no `FAILED` line.
Run: `go test ./modules/gmcp/ ./internal/users/ -count=1`
Expected: `ok`.

- [ ] **Step 8: Document**

In `internal/users/context.md`, after the paragraph about `StoreReportedCols` (around line 638), add:

```markdown
`StoreReportedRows(id, rows) int` (#842) does the same for the web
client's row count: clamped to `pager.MinSize`..`pager.MaxSize`, stored in
`Display.ScreenHeight` only when it differs. Telnet heights come from NAWS.
```

In `modules/gmcp/context.md`, find the `Client.Display` entry and add that it also carries `rows`, stored by `users.StoreReportedRows` for websocket connections, and that a payload without `rows` keeps the stored height.

- [ ] **Step 9: Commit**

```bash
git add internal/users/wrapwidth.go internal/users/context.md modules/gmcp/gmcp.ClientDisplay.go modules/gmcp/gmcp_client_display_rows_test.go modules/gmcp/context.md _datafiles/html/public/static/js/displayreport.js _datafiles/html/public/static/js/termfit.js _datafiles/html/public/webclient-pure.html tools/jstest/displayreport.test.js tools/jstest/termfit.test.js
git commit -F - <<'EOF'
feat(webclient): report the game window's rows for paging (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: `set pagesize`

**Files:**
- Modify: `internal/usercommands/set.go` (dispatch at line 60-63; new function after `cmdSetLineWidth`, line 569)
- Create: `internal/usercommands/set_pagesize_test.go`
- Modify: `_datafiles/world/dogmud/templates/help/set.template` (after the `set combatverbosity` line)

- [ ] **Step 1: Write the failing test**

`internal/usercommands/set_pagesize_test.go` (same fixture as `set_tips_test.go`):

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestSetPageSize(t *testing.T) {
	user := users.NewTestUser(8802, "pager", "Pager", 18802)
	events.DrainQueuedMessagesForTest(8802)
	defer events.DrainQueuedMessagesForTest(8802)

	cases := []struct {
		arg  string
		want int
		says string
	}{
		{"30", 30, "Pages are 30 lines."},
		{"off", pager.SettingOff, "Paging is off."},
		{"auto", 0, "Pages follow your screen."},
		{"5", 0, "Page size must be a number between 10 and 200"},
		{"tall", 0, "Page size must be a number between 10 and 200"},
	}
	for _, c := range cases {
		user.PageSize = 0
		_, err := Set("pagesize "+c.arg, user, nil, events.EventFlag(0))
		require.NoError(t, err)
		require.Equal(t, c.want, user.PageSize, c.arg)
		sent := strings.Join(events.DrainQueuedMessagesForTest(8802), "")
		require.Contains(t, sent, c.says, c.arg)
	}
}
```

Run: `go test ./internal/usercommands/ -run TestSetPageSize -count=1`
Expected: FAIL (`pagesize` falls to the macro default branch; `PageSize` stays 0 for "30").

- [ ] **Step 2: Add the command**

In `internal/usercommands/set.go`, after the `case \`linewidth\`:` pair add:

```go
	case `pagesize`:
		return cmdSetPageSize(user, args)
```

After `cmdSetLineWidth` add (import `github.com/GoMudEngine/GoMud/internal/pager`):

```go
// cmdSetPageSize sets how many lines a long reply shows before it stops for
// Enter (#842): a number, "off", or "auto" to follow the screen.
func cmdSetPageSize(user *users.UserRecord, args []string) (bool, error) {
	if len(args) < 1 {
		switch {
		case user.PageSize == pager.SettingOff:
			user.SendText(messaging.CategorySystem, "Paging is off.")
		case user.PageSize > 0:
			user.SendText(messaging.CategorySystem, fmt.Sprintf("Pages are %d lines.", user.PageSize))
		default:
			user.SendText(messaging.CategorySystem, "Pages follow your screen.")
		}
		return true, nil
	}
	switch strings.ToLower(args[0]) {
	case `off`:
		user.PageSize = pager.SettingOff
		user.SendText(messaging.CategorySystem, "Paging is off.")
	case `auto`:
		user.PageSize = 0
		user.SendText(messaging.CategorySystem, "Pages follow your screen.")
	default:
		n, err := strconv.Atoi(args[0])
		if err != nil || n < pager.MinSize || n > pager.MaxSize {
			user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
				`Page size must be a number between %d and %d, or <ansi fg="command">off</ansi> or <ansi fg="command">auto</ansi>.`,
				pager.MinSize, pager.MaxSize))
			return true, nil
		}
		user.PageSize = n
		user.SendText(messaging.CategorySystem, fmt.Sprintf("Pages are %d lines.", n))
	}

	events.AddToQueue(events.UserSettingChanged{
		UserId: user.UserId,
		Name:   `pagesize`,
	})
	return true, nil
}
```

- [ ] **Step 3: Help text**

In `_datafiles/world/dogmud/templates/help/set.template`, after the line containing `set combatverbosity <full|medium|light>` add:

```
  <ansi fg="command">set pagesize <number|off|auto></ansi>  How many lines a long answer shows before it
                                    stops and waits for Enter. <ansi fg="command">auto</ansi> follows your
                                    screen, <ansi fg="command">off</ansi> never stops. While stopped, Enter
                                    shows more and <ansi fg="command">q</ansi> stops; any other command runs.
```

Keep every line within 80 columns.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/usercommands/ -count=1`
Expected: `ok`. A help-template test (if one checks widths or the command list) must pass too; fix the text, not the test.

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/set.go internal/usercommands/set_pagesize_test.go _datafiles/world/dogmud/templates/help/set.template
git commit -F - <<'EOF'
feat(set): set pagesize <number|off|auto> (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: A Toke-sized talk is fully marked

**Files:**
- Create: `internal/usercommands/conversation_pager_mark_test.go`

- [ ] **Step 1: Write the test**

It uses `fixtureWith` and `mustRoom` from `conversation_menu_again_test.go`'s package helpers (same package).

```go
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
```

If the dialogue loader rejects the fixture YAML, compare it with `menuAgainPlainYAML` in `conversation_menu_again_test.go` and match its keys; do not weaken the assertions.

- [ ] **Step 2: Run it**

Run: `go test ./internal/usercommands/ -run TestTalk_EveryLineToThePlayerIsTheReply -count=1`
Expected: `ok`. To prove it can fail: temporarily comment out `events.OpenReply(user.UserId)` in the test, run it, see "not marked", restore the line.

- [ ] **Step 3: Commit**

```bash
git add internal/usercommands/conversation_pager_mark_test.go
git commit -F - <<'EOF'
test(conversation): a long talk is marked as one reply (#842)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Full gate, then the playtest

- [ ] **Step 1: Whole suite and lint**

Run: `go build ./... && go test ./... -count=1` (expect every package `ok`), then `bash tools/lint.sh` (expect no findings), then `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED $t"; done` (expect no `FAILED`).

- [ ] **Step 2: Mutation checks (one at a time, restore after each)**

1. In `events.AddToQueue`, change `replyWindow[m.UserId] > 0` to `false`: `TestReplyWindow_MarksOnlyThatPlayersMessages` and `TestTalk_EveryLineToThePlayerIsTheReply` must fail.
2. In `UserRecord.EndReply`, delete `|| fighting`: `TestEndReply_InCombatSendsWhole` must fail.
3. In `world.go`, move the pager routing block below the `RouteMenuInput` block: `TestProcessInput_PagerReadsBeforeTheMenu` must fail.
4. In `Message_SendMessage`, delete the `FlushPagesInCombat` block: `TestPager_AFightSendsTheWaitingPagesFirst` must fail.

Record each "failed as expected" in the PR description.

- [ ] **Step 3: Playtest (controller, not a subagent)**

Use the `dogmud-playtesting` skill. A local Docker run with a goals file that:
- walks a fresh character to Crier Toke (mob 9106, Pothole Coulee) with `set pagesize 10`, types `talk toke`, and checks: page one and the stop line, Enter for page two, a menu number from an unseen page picks that topic, `q` stops;
- repeats on the human telnet port inside the container (`docker exec <c> nc localhost 33333`) to see the raw stop line;
- starts a fight (for example `mob spawn 205` as admin in room 5000) while a stop is up and checks that the remaining pages arrive before the first combat line and the normal prompt returns.

File anything found as GitHub issues on `pruuk/DOGMud`.

- [ ] **Step 4: Open the PR**

```bash
git push -u origin <branch>
gh pr create --repo pruuk/DOGMud --title "General pager for long replies (#842)" --body-file <scratchpad file>
```

The body lists what changed per task, the mutation results, the playtest result, and ends with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Do not write "closes #842" unless the owner says the issue is done.

---

## Self-review notes

- Spec coverage: section 1 → Tasks 2, 3; section 2 → Task 1; section 3 → Tasks 4, 5; section 4 → Tasks 4, 6; section 5 → Tasks 7, 8; section 6 → Task 1 (`ScreenReaderSize`) and Task 4 (`EndReply` reads `u.ScreenReader`); section 7 → tests in every task plus Task 10; ruling 5 (combat) → Tasks 4, 5.
- Fixtures reused from existing tests, read 2026-11-18: `inputDoneUser`/`drainGMCPOut` (`modules/gmcp/gmcp_input_done_test.go`), `clientDisplayConn`/`reportCols` (`modules/gmcp/gmcp.ClientDisplay_test.go`), `NewTestUser` + `DrainQueuedMessagesForTest` (`internal/usercommands/set_tips_test.go`), `captureConnection`/`visibleLastLine`/`seedAllRegistries` (`internal/hooks`), `fixtureWith`/`mustRoom` (`internal/usercommands`).
- Not verified while writing: whether `makePage`'s fake terminal starts at a row count different from its first fit (Task 7 Step 5 says what to do either way), and the exact text `RenderForRecipient` wraps a `CategorySystem` line into (Task 8 asserts with `Contains`).
