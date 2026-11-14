# Web client base, plan 6: phones (tab height, keyboard, X, sideways toast)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On a phone, an open tab takes at most 30% of the screen; the keyboard stays up for chat and for any menu or prompt the game opens, and closes after anything else; a Close keyboard X sits in the command box while the keyboard is up; a phone turned sideways gets one note asking for upright.

**Architecture:** The server numbers each text line a web client sends (counted at the websocket, as the client counts its own sends) and, after handling a typed line, sends `Char.Input.Done {seq, menu}` after all of that command's text. The client keeps the keyboard up after chat, and after any other in-game command waits for its line's marker, then closes the keyboard unless a menu or prompt is open. The tab cap, the X and the toast are CSS and small client pieces.

**Tech Stack:** Go (package `main`, `internal/events`, `modules/gmcp`), plain browser JavaScript, node tests in `tools/jstest`, CSS.

Spec: `docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md` (approved 2026-11-14, rulings R1 to R7). Parent spec: `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` (rulings 12 to 15). Epic #783. Approved mock: https://claude.ai/artifact/Fb1BNLYYmRhbo3MdULYioW.

Every agent: edit files with the Edit and Write tools only; never rewrite a tracked file from the shell (`sed -i`, `>`, Python `open(..., 'w')`); `git add` named paths only; never stash; kill only processes you started, by PID.

---

## Facts verified against source

Read 2026-11-14 from master `95e61dddf` plus the spec commits on `feature/webclient-base-6-phones`. `WP` is `_datafiles/html/public/webclient-pure.html`; `CSS` is `_datafiles/html/public/static/css/dashboard.css`; `JS` is `_datafiles/html/public/static/js`.

| Fact | Where |
|---|---|
| The websocket loop: `c := configs.GetConfig()` then `for { _, message, err := conn.ReadMessage()`; the frame type is dropped | `main.go:1400-1403` |
| The copyover token path inside the loop `continue`s before input handling | `main.go:1437-1467` |
| The websocket builds `WorldInput{FromId, InputText}` and calls `worldManager.SendInput(wi)` | `main.go:1555-1561` |
| `type WorldInput struct { FromId int; InputText string; ReadyTurn uint64 }` | `world.go:37-41` |
| The input worker turns each `WorldInput` into `events.Input{UserId, InputText, ReadyTurn: util.GetTurnCount(), Flags: events.CmdTyped}` | `world.go:961-967` |
| `processInput(userId int, inputText string, flags events.EventFlag)`; `typed := flags.Has(events.CmdTyped)` is its first statement after the nil-user return | `world.go:973-981` |
| `processInput` is called from two places | `world.go:141`, `:184` |
| A server macro re-queues each part as `events.Input` with `Flags: flags & events.CmdTyped` and no other fields | `world.go:1014-1031` |
| `type Input struct { UserId, MobInstanceId int; InputText string; ReadyTurn uint64; Flags EventFlag }` | `internal/events/eventtypes.go:144-152` |
| `MenuChanged` and its `Type()` | `internal/events/eventtypes.go:81-89` |
| `events.AddToQueue(e, priority...)`; a higher number pops after every lower one in flight | `internal/events/events.go:91`; `internal/users/userrecord.go:504-506` |
| Test helpers `DrainQueuedByTypeForTest`, `QueuedTypesInOrderForTest` | `internal/events/events.go:901`, `:923` |
| `Char.Menu`: listeners registered in `init`, `GMCPOut{UserId, Module, Payload}` queued at -1 | `modules/gmcp/gmcp.Menu.go:13-17`, `:76-90` |
| `isGMCPEnabled` (`GMCPAccepted && HelloReceived`), `isWebClientConnection` (`Client.Name == "WebClient"`) | `modules/gmcp/gmcp.go:79-97` |
| Test setup for a GMCP user: `users.NewTestUser`, `users.SeedUsersForTest`, `gmcpModule.cache.Add(connId, GMCPSettings{...})` | `modules/gmcp/gmcp_menu_test.go:13-31` |
| `u.StartPrompt("test", "")` opens a prompt, so `MenuOrPromptOpen()` is true | `modules/gmcp/gmcp_menu_test.go:100` |
| Package `main` tests run under a `TestMain` | `main_test.go:20` |
| `HotInput.shouldHideKeyboardAfterSend(s)` and its export | `JS/hotinput.js:149-165` |
| Its tests, and page-source checks that name it | `tools/jstest/hotinput.test.js:262-300` |
| The Enter handler: `var sent = SendData(event.target.value);` then the HotInput call and `textInput.blur()` | `WP:3218-3233` |
| `SendData` sends with `socket.send(dataToSend)`; `SendGMCP` sends a binary `Uint8Array` | `WP:3236-3263` |
| The copyover token is sent with `socket.send(t)` in `socket.onopen` | `WP:2943`, `:2959-2963` |
| `"Char.Menu"` GMCP handler sets `menuOpen` | `WP:1013-1018` |
| `handleGMCP` calls the most specific registered handler first | `WP:2477-2517` |
| `const textInput = document.getElementById('command-input');` and its keydown listener | `WP:2849`, `:3171` |
| `phoneTouchOnly()`, `phoneWidth()` | `WP:3299-3307` |
| The page's own keyboard watch (only when not framed) | `WP:2704-2716` |
| The command box markup | `WP:299-303` |
| Brass buttons use `class="brass"` | `WP:251`, `:286` |
| The outer page's `fitFrameHeight` marks `keyboard-open` on the frame's document | `_datafiles/html/public/webclient.html:26-38` |
| The open-tab cap rules: base `:690-706`; keyboard `:786-799`; column drawer `:525`, `:785` | `CSS` |
| `#dash-drawer #panel-map, #panel-chat` height reads `--drawer-cap` | `CSS:916-921` |
| `.dash-toast` | `CSS:1758-1768` |
| `ViewportFit`: `isPhone`, `isUprightPhone`, `markUprightPhone` (orientation reading with the iOS `window.orientation` fallback), exports | `JS/viewportfit.js:18-51`, `:100-110` |
| `Dashboard.init`, `_toast(msg)` (2.6s) | `JS/dashboard.js:165-200`, `:272-282` |
| `dashboard-popout-layout.test.js` stubs `D._toast` | `tools/jstest/dashboard-popout-layout.test.js:430` |
| CI runs every `tools/jstest/*.test.js` | `.github/workflows/validate.yml:114` |

## Owner rulings for this plan

The spec's R1 to R7, not reopened:
- R1. The keyboard stays up whenever the player's next act is to type again: chat to players, talk to NPCs. `party` and `guild` count only when talking.
- R2. Menus and prompts: the client waits for the game's answer.
- R3. The 30% cap applies where a tab covers game text (phone, slim); column keeps its cap.
- R4. The mock is approved.
- R5. The wait is timed from the game's reply, not from Enter.
- R6. The server marks each command done, with whether a menu or prompt is open.
- R7. The marker never carries typed text: lines are numbered at the socket on both ends.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| The once-per-tab toast rule is a pure `ViewportFit.takeSidewaysToast(storage, memo)` tested in `viewportfit.test.js`; the dashboard only calls it | The spec named "a dashboard test"; the rule is pure, so it is tested where pure helpers live, and the dashboard test keeps its stubbed `_toast` | A dashboard-level test can be added beside the popout test |
| Package `main` wiring (`processInput` defers the marker; the websocket loop counts) is held by source guards; the helpers it calls are unit tested | `processInput` and `HandleWebSocketConnection` need a running world and a socket | An integration test can replace the guards later |
| `processInput` gains a fourth parameter `seq uint64` rather than reading it off `flags` | `EventFlag` is a bit set; a number does not belong in it | None |
| The X uses the existing `.brass` class plus a small rule for size | Matches the session bar's buttons, as the mock shows | Restyle in one rule |

## File structure

Server:
- Modify `internal/events/eventtypes.go`: `Input.Seq`; new `InputDone{UserId, Seq}`.
- Modify `world.go`: `WorldInput.Seq`; `inputEventFor`; `processInput(..., seq)`; `queueInputDone`.
- Modify `main.go`: `countLineFrame`; the websocket loop counts and passes `Seq`.
- Create `world_input_done_test.go` (package `main`): helpers and source guards.
- Create `modules/gmcp/gmcp.InputDone.go` and `modules/gmcp/gmcp_input_done_test.go`.

Client:
- Modify `JS/hotinput.js`: `isChatLine`, `keyboardAfterSend`, `blurAfterWait`, `createSendWait`, constants; `shouldHideKeyboardAfterSend` retires.
- Modify `WP`: count text frames, the wait, the `Char.Input.Done` handler, the X button, `--visible-h` in the page's own keyboard watch.
- Modify `_datafiles/html/public/webclient.html`: `--visible-h` in `fitFrameHeight`.
- Modify `CSS`: the caps, the X, `.dash-toast.lifted`.
- Modify `JS/viewportfit.js`: `orientationType`, `isSidewaysPhone`, `takeSidewaysToast`.
- Modify `JS/dashboard.js`: `_toast(msg, opts)`, `_watchSideways`.
- Modify `tools/jstest/hotinput.test.js`, `tools/jstest/viewportfit.test.js`.

Docs: `internal/events/context.md`, `modules/gmcp/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`.

---

### Task 0: Worktree

- [ ] **Step 1: Free the branch from the main checkout and create the worktree**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git status --short          # expect only this plan file untracked
git checkout master
git worktree add C:/tmp/dogmud-p6 feature/webclient-base-6-phones
```

- [ ] **Step 2: Land this plan in the worktree and commit it**

```bash
cp "C:/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-14-webclient-base-6-phones.md" C:/tmp/dogmud-p6/docs/superpowers/plans/
cd C:/tmp/dogmud-p6
git add docs/superpowers/plans/2026-11-14-webclient-base-6-phones.md
git commit -m "docs(plan): web client base plan 6, phones"
```

Then delete the untracked copy in the main checkout. Record `C:/tmp/dogmud-p6` in memory `project-touch-arc-base-first`.

---

### Task 1: Events carry the line number; the marker event

**Files:**
- Modify: `internal/events/eventtypes.go:81-89`, `:144-152`
- Modify: `world.go:37-41`, `:961-967`
- Create: `world_input_done_test.go`

- [ ] **Step 1: Write the failing test** (`world_input_done_test.go`)

```go
package main

import (
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
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test -count=1 -run 'TestInputEventFor' .`
Expected: FAIL to compile: `inputEventFor` undefined, unknown field `Seq`.

- [ ] **Step 3: Add the fields and the event**

In `internal/events/eventtypes.go`, `Input` gains `Seq` (after `Flags`):

```go
type Input struct {
	UserId        int
	MobInstanceId int
	InputText     string
	ReadyTurn     uint64
	Flags         EventFlag
	// Seq numbers a web client's line: the count of text frames its
	// connection has sent, the same count the client keeps (plan 6, R7).
	// 0 for telnet and for lines the game queues itself.
	Seq uint64
}
```

After `MenuChanged`'s `Type()` (line 89), add:

```go
// InputDone is queued after the server has handled one typed web client line
// (plan 6, spec 2026-11-14 R6). gmcp pushes it as Char.Input.Done {seq, menu}
// so the phone keyboard can wait for its own line. It carries the line's
// number, never its text (R7).
type InputDone struct {
	UserId int
	Seq    uint64
}

func (i InputDone) Type() string { return `InputDone` }
```

In `world.go`, `WorldInput` gains `Seq`:

```go
type WorldInput struct {
	FromId    int
	InputText string
	ReadyTurn uint64
	// Seq is the web client line number (main.go countLineFrame); 0 for telnet.
	Seq uint64
}
```

Replace the input worker's literal at `world.go:961-967`:

```go
		case wi := <-w.worldInput:

			events.AddToQueue(inputEventFor(wi))
```

and add beside `processInput`:

```go
// inputEventFor is the event a connection line becomes: typed, ready now, and
// carrying the web client's line number for the done marker (plan 6).
func inputEventFor(wi WorldInput) events.Input {
	return events.Input{
		UserId:    wi.FromId,
		InputText: wi.InputText,
		ReadyTurn: util.GetTurnCount(),
		Flags:     events.CmdTyped,
		Seq:       wi.Seq,
	}
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test -count=1 -run 'TestInputEventFor' .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/events/eventtypes.go world.go world_input_done_test.go
git commit -m "feat(input): a web client line carries its number to the world (plan 6)"
```

---

### Task 2: processInput marks a typed line done

**Files:**
- Modify: `world.go:141`, `:184`, `:973-981`
- Modify: `world_input_done_test.go`

- [ ] **Step 1: Write the failing tests** (append to `world_input_done_test.go`; add `"os"` and `"regexp"` to its imports)

```go
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
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -count=1 -run 'TestQueueInputDone|TestProcessInput_DefersTheDoneMarker' .`
Expected: FAIL to compile (`queueInputDone` undefined).

- [ ] **Step 3: Implement**

Add beside `inputEventFor` in `world.go`:

```go
// queueInputDone queues the marker that the server has finished one typed web
// client line (plan 6, R6). Priority 1 puts it after every line of text the
// command queued at the default priority 0, even ones queued later in the
// same event pass, so the client reads the menu state the command left.
func queueInputDone(userId int, seq uint64) {
	events.AddToQueue(events.InputDone{UserId: userId, Seq: seq}, 1)
}
```

Change the signature and add the defer right after `typed := ...`:

```go
func (w *World) processInput(userId int, inputText string, flags events.EventFlag, seq uint64) {

	user := users.GetByUserId(userId)
	if user == nil { // Something went wrong. User not found.
		mudlog.Error("User not found", "userId", userId)
		return
	}

	typed := flags.Has(events.CmdTyped)
	// Plan 6 (R6, R7): tell the web client this line is done, after all its
	// text, on every return below. Its number, never its text.
	if typed && seq > 0 {
		defer queueInputDone(userId, seq)
	}
```

Change both callers:

```go
		w.processInput(input.UserId, input.InputText, input.Flags, input.Seq)
```

```go
	w.processInput(input.UserId, input.InputText, events.EventFlag(input.Flags), input.Seq)
```

The macro re-queue at `world.go:1024-1029` stays as it is: its parts carry no `Seq`, so only the typed line is marked (spec decisions).

- [ ] **Step 4: Run them to see them pass**

Run: `go test -count=1 -run 'TestInputEventFor|TestQueueInputDone|TestProcessInput_DefersTheDoneMarker' .` then `go build ./...`
Expected: PASS; build ok.

- [ ] **Step 5: Commit**

```bash
git add world.go world_input_done_test.go
git commit -m "feat(input): the server marks each typed web client line done (plan 6)"
```

---

### Task 3: The websocket counts text frames

**Files:**
- Modify: `main.go:1400-1403`, `:1555-1558`, and a new helper beside `HandleWebSocketConnection`
- Modify: `world_input_done_test.go`

- [ ] **Step 1: Write the failing tests** (append; add `"github.com/gorilla/websocket"` to the imports)

```go
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
	readAt := regexp.MustCompile(`msgType, message, err := conn\.ReadMessage\(\)`).FindStringIndex(s)[0]
	countAt := regexp.MustCompile(`lineSeq = countLineFrame\(msgType, lineSeq\)`).FindStringIndex(s)
	require.NotNil(t, countAt)
	tokenAt := regexp.MustCompile(`copyover\.ConsumeReconnectToken`).FindStringIndex(s)[0]
	require.True(t, readAt < countAt[0] && countAt[0] < tokenAt,
		"count each frame right after the read, before the copyover token check")
	require.Regexp(t, regexp.MustCompile(`Seq:\s+lineSeq,`), s)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -count=1 -run 'TestCountLineFrame|TestWebSocketLoop' .`
Expected: FAIL to compile (`countLineFrame` undefined).

- [ ] **Step 3: Implement**

Add above `func HandleWebSocketConnection` in `main.go`:

```go
// countLineFrame advances a websocket connection's line count for a text
// frame. The web client counts the same frames (every string it sends: its
// lines and the copyover token), and Char.Input.Done carries the count back so
// the phone keyboard can wait for its own line without the line's text ever
// being sent back (plan 6, R7). Binary frames are the client's GMCP.
func countLineFrame(msgType int, seq uint64) uint64 {
	if msgType == websocket.TextMessage {
		return seq + 1
	}
	return seq
}
```

In the loop (`main.go:1400-1403`):

```go
	c := configs.GetConfig()

	// The number of text frames this connection has sent (countLineFrame).
	var lineSeq uint64

	for {
		msgType, message, err := conn.ReadMessage()

		if err != nil {
```

and right after the `if err != nil { ... break }` block closes (before the `// Copyover:` comment at `main.go:1436`):

```go
		lineSeq = countLineFrame(msgType, lineSeq)
```

At `main.go:1555`:

```go
		wi := WorldInput{
			FromId:    userObject.UserId,
			InputText: string(clientInput.Buffer),
			Seq:       lineSeq,
		}
```

The two telnet sites (`main.go:1085`, `:1265`) stay without `Seq`.

- [ ] **Step 4: Run them to see them pass**

Run: `gofmt -l . | grep -v '^vendor'` (expect nothing), then `go test -count=1 -run 'TestCountLineFrame|TestWebSocketLoop|TestInputEventFor|TestQueueInputDone|TestProcessInput' .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add main.go world_input_done_test.go
git commit -m "feat(webclient): the websocket numbers each text line (plan 6)"
```

---

### Task 4: `Char.Input.Done`

**Files:**
- Create: `modules/gmcp/gmcp.InputDone.go`
- Create: `modules/gmcp/gmcp_input_done_test.go`

- [ ] **Step 1: Write the failing tests** (`modules/gmcp/gmcp_input_done_test.go`)

```go
package gmcp

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
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

// R7: nothing typed is ever sent back. The event has no field that could
// hold text, and the payload has exactly its two keys.
func TestInputDone_CarriesNoTypedText(t *testing.T) {
	ty := reflect.TypeOf(events.InputDone{})
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
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -count=1 -run 'TestInputDone' ./modules/gmcp/`
Expected: FAIL to compile (`inputDoneHandler` undefined).

- [ ] **Step 3: Implement** (`modules/gmcp/gmcp.InputDone.go`)

```go
package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Input.Done tells the web client the server has finished one of its
// typed lines: {"seq": n, "menu": bool}. seq is the line's number (both ends
// count text frames from the socket's open); menu is whether that left a game
// menu or prompt open. The phone keyboard waits for it and stays up when a
// menu opened (plan 6, spec 2026-11-14 R2, R6). It never carries the line's
// text, so an answer typed into the password prompt is not sent back (R7).
// Web client only: telnet GMCP clients get nothing new.
func init() {
	events.RegisterListener(events.InputDone{}, inputDoneHandler)
}

func inputDoneHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.InputDone)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	connId := user.ConnectionId()
	if !isWebClientConnection(connId) || !isGMCPEnabled(connId) {
		return events.Continue
	}
	// Default priority: InputDone itself popped after the command's text, so
	// this frame follows that text too.
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Input.Done`,
		Payload: map[string]any{"seq": evt.Seq, "menu": user.MenuOrPromptOpen()},
	})
	return events.Continue
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test -count=1 ./modules/gmcp/`
Expected: PASS (all of `modules/gmcp`).

- [ ] **Step 5: Commit**

```bash
git add modules/gmcp/gmcp.InputDone.go modules/gmcp/gmcp_input_done_test.go
git commit -m "feat(gmcp): Char.Input.Done tells the web client a line is done (plan 6)"
```

---

### Task 5: HotInput decides and waits

**Files:**
- Modify: `JS/hotinput.js:137-165`
- Modify: `tools/jstest/hotinput.test.js:262-300`

- [ ] **Step 1: Write the failing tests.** In `tools/jstest/hotinput.test.js`, replace the block from `// --- phone keyboard hides after an in-game send` through the line `check('no state at all: keeps the keyboard', HotInput.shouldHideKeyboardAfterSend(), false);` with:

```js
// --- phone keyboard after a send (plan 6) ------------------------------------
//
// docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md. On a
// touch-only phone, in game, after a send: chat keeps the keyboard (R1);
// anything else waits for the server's Char.Input.Done (R2, R5, R6). Login,
// password and menu answers keep it, as do tablets, desktops and a send that
// did not go out (owner ruling on #768, 2026-11-07).

var AFTER = { touchOnly: true, phoneWidth: true, inGame: true, menuOpen: false, password: false, sent: true, line: 'look' };
function after(overrides) {
    var s = {};
    Object.keys(AFTER).forEach(function (k) { s[k] = AFTER[k]; });
    Object.keys(overrides || {}).forEach(function (k) { s[k] = overrides[k]; });
    return HotInput.keyboardAfterSend(s);
}
check('phone, in game, look: waits for the game', after(), 'wait');
check('talk to an NPC: waits (its menu keeps the keyboard)', after({ line: 'talk hadwen' }), 'wait');
check('say: keeps the keyboard', after({ line: 'say hi' }), 'keep');
check('not touch-only (desktop, touch laptop): keeps the keyboard', after({ touchOnly: false }), 'keep');
check('tablet width: keeps the keyboard (arrow history, macros)', after({ phoneWidth: false }), 'keep');
check('at a login prompt: keeps the keyboard', after({ inGame: false }), 'keep');
check('answering a numbered game menu: keeps the keyboard', after({ menuOpen: true }), 'keep');
check('at a password prompt: keeps the keyboard', after({ password: true }), 'keep');
check('nothing was sent: keeps the keyboard', after({ sent: false }), 'keep');
check('no state at all: keeps the keyboard', HotInput.keyboardAfterSend(), 'keep');
check('the old name is gone', typeof HotInput.shouldHideKeyboardAfterSend, 'undefined');

// R1: chat words, as typed. Aliases and macros are out (ruling 13).
['say hi', 'whisper bob hi', 'reply ok', 'chat hello', 'newbie help?', 'trade wts rope',
 'shout hey', 'emote waves', 'yell hey', 'scream hey', 'holler hey', '/w bob hi',
 '. hi', '.hi', '` hi', '`hi', 'SAY hi', '  say hi  ', 'party chat hi', 'party say hi',
 'guild chat hi', 'guild say hi', 'guild gc hi', 'PARTY CHAT hi'
].forEach(function (line) { check('chat line: ' + JSON.stringify(line), HotInput.isChatLine(line), true); });
['look', 'talk hadwen', 'ask hadwen about rope', 'party invite bob', 'party', 'party loot ffa',
 'guild deposit 5', 'guild', 'sayings', 'emoted', '', '   ', null, undefined
].forEach(function (line) { check('not a chat line: ' + JSON.stringify(line), HotInput.isChatLine(line), false); });

// After the wait: close only if no menu, the box still has focus, and the
// player has not started the next command.
check('wait over, no menu, focused, empty: close', HotInput.blurAfterWait({ menu: false, focused: true, value: '' }), true);
check('a menu opened: keep', HotInput.blurAfterWait({ menu: true, focused: true, value: '' }), false);
check('focus moved away: leave it', HotInput.blurAfterWait({ menu: false, focused: false, value: '' }), false);
check('already typing the next command: keep', HotInput.blurAfterWait({ menu: false, focused: true, value: 'n' }), false);

// The wait matches the server's marker by line number (R7), with a ceiling.
(function () {
    var now = 0, timers = [], settled = [];
    function fakeSet(f, ms) { var t = { at: now + ms, f: f, live: true }; timers.push(t); return t; }
    function fakeClear(t) { if (t) t.live = false; }
    function advance(ms) {
        now += ms;
        timers.forEach(function (t) { if (t.live && t.at <= now) { t.live = false; t.f(); } });
    }
    var menuOpenNow = false;
    var w = HotInput.createSendWait({
        setTimeout: fakeSet, clearTimeout: fakeClear, ceilingMs: HotInput.MENU_WAIT_CEILING_MS,
        currentMenu: function () { return menuOpenNow; },
        settle: function (menu) { settled.push(menu); }
    });
    check('the ceiling is 2 seconds', HotInput.MENU_WAIT_CEILING_MS, 2000);

    w.start(5);
    w.done(4, true);
    check("an earlier line's marker does not end the wait", settled.length, 0);
    w.done(5, true);
    check('its own marker ends it with that menu state', JSON.stringify(settled), '[true]');
    advance(3000);
    check('the ceiling does not fire after the marker', settled.length, 1);

    w.start(6);
    w.done(8, false);
    menuOpenNow = true;
    check('a later number means the line never reached the world: ends at once',
        JSON.stringify(settled), '[true,false]');

    w.start(9);
    menuOpenNow = true;
    advance(1999);
    check('no marker: still waiting at 1999ms', settled.length, 2);
    advance(1);
    check('no marker: the ceiling settles with the current menu state', JSON.stringify(settled), '[true,false,true]');

    w.start(10);
    w.start(11);
    w.done(10, false);
    check('a new wait replaces the old one', settled.length, 3);
    w.done(11, false);
    check('the new wait ends on its own marker', JSON.stringify(settled), '[true,false,true,false]');

    w.start(12);
    w.reset();
    advance(5000);
    w.done(12, true);
    check('a new socket clears the wait', settled.length, 4);
})();
```

Also replace the three page checks that name the old function (`hotinput.test.js:294-300`, from `var sendAt = ...` through the `inGame: inGame, menuOpen: ...` check) with:

```js
var sendAt = page.indexOf('var sent = SendData(line);');
var decideAt = page.indexOf('window.HotInput.keyboardAfterSend({');
check('the command box asks HotInput after sending', sendAt >= 0 && decideAt > sendAt, true);
check('the decision is fed the send result and the typed line',
    /phoneWidth: phoneWidth\(\), sent: sent,[\s\S]{0,200}line: line/.test(page), true);
check('the decision is told about login, menus and password prompts',
    /inGame: inGame, menuOpen: menuOpen, password: textInput\.type === "password"/.test(page), true);
check('a wait starts with the line number SendData gave it',
    /=== 'wait'\) \{\s*sendWait\.start\(linesSent\);/.test(page), true);
check('every string sent on the socket is counted (both ends count text frames, R7)',
    (page.match(/socket\.send\(/g) || []).length === 3
    && /linesSent\+\+;\s*socket\.send\(t\);/.test(page)
    && /linesSent\+\+;\s*socket\.send\(dataToSend\);/.test(page), true);
check('a new socket starts the count again',
    /socket\.onopen = function\(\) \{\s*linesSent = 0;\s*if \(sendWait\) sendWait\.reset\(\);/.test(page), true);
check('Char.Input.Done ends the wait by number',
    /"Char\.Input\.Done": function\(\) \{[\s\S]{0,200}sendWait\.done\(d\.seq, d\.menu === true\)/.test(page), true);
```

- [ ] **Step 2: Run them to see them fail**

Run: `node tools/jstest/hotinput.test.js`
Expected: FAIL (`HotInput.keyboardAfterSend is not a function`).

- [ ] **Step 3: Implement.** In `JS/hotinput.js`, replace the comment block and function from `// Should the phone keyboard close after a send?` through the end of `shouldHideKeyboardAfterSend`, and the return object, with:

```js
    // --- The phone keyboard after a send (plan 6) --------------------------
    // docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md.
    //
    // R1: the keyboard stays up when the player's next act is to type again,
    // that is chat to other players. These words are today's data under that
    // principle, not a closed list. Only what was literally typed counts:
    // aliases and macros are out (layout spec ruling 13). party and guild are
    // mostly management, so they count only when talking.
    var CHAT_WORDS = ['say', 'whisper', 'reply', 'chat', 'newbie', 'trade', 'shout',
                      'emote', 'yell', 'scream', 'holler', '/w'];
    var PARTY_TALK = ['chat', 'say'];
    var GUILD_TALK = ['chat', 'say', 'gc'];
    // R2, R5, R6: anything else waits for the server's Char.Input.Done for
    // that line; this long at most if the marker never comes.
    var MENU_WAIT_CEILING_MS = 2000;

    function isChatLine(line) {
        var t = String(line == null ? '' : line).trim();
        if (t === '') return false;
        // The server reads a leading . or backtick as say or chat with or
        // without a space (usercommands.SplitInput).
        var c = t.charAt(0);
        if (c === '.' || c === '`') return true;
        var words = t.toLowerCase().split(/\s+/);
        if (CHAT_WORDS.indexOf(words[0]) >= 0) return true;
        if (words[0] === 'party') return PARTY_TALK.indexOf(words[1]) >= 0;
        if (words[0] === 'guild') return GUILD_TALK.indexOf(words[1]) >= 0;
        return false;
    }

    // What the phone keyboard does after a send: 'keep' it, or 'wait' for the
    // game's answer. Only a touch-only phone-width screen, in game, with no
    // menu open and not at a password prompt ever waits (owner ruling on #768,
    // 2026-11-07, which these checks keep). Tablets keep the keyboard, since a
    // blur there would also stop arrow history and macros working.
    //
    // s: { touchOnly, phoneWidth, sent, inGame, menuOpen, password, line }
    function keyboardAfterSend(s) {
        s = s || {};
        if (!(s.touchOnly && s.phoneWidth && s.sent)) return 'keep';
        if (!s.inGame || s.menuOpen || s.password) return 'keep';
        if (isChatLine(s.line)) return 'keep';
        return 'wait';
    }

    // Once the wait is over: close the keyboard only if no menu or prompt is
    // open, the command box still has focus, and the player has not started
    // typing the next command.
    function blurAfterWait(s) {
        s = s || {};
        return !s.menu && !!s.focused && (s.value || '') === '';
    }

    // One wait at a time for the marker of line `seq` (the client's count of
    // text frames; the server counts the same, R7). opts: { setTimeout,
    // clearTimeout, ceilingMs, currentMenu() -> bool, settle(menu) }.
    function createSendWait(opts) {
        var pending = null;   // { seq, timer }
        function finish(menu) {
            if (!pending) return;
            opts.clearTimeout(pending.timer);
            pending = null;
            opts.settle(menu);
        }
        return {
            start: function (seq) {
                if (pending) opts.clearTimeout(pending.timer);
                var p = { seq: seq };
                p.timer = opts.setTimeout(function () {
                    if (pending === p) finish(opts.currentMenu());
                }, opts.ceilingMs);
                pending = p;
            },
            // The server's marker for line `seq`. A higher number means the
            // waiting line never reached the world (an admin system command, a
            // dropped line): settle now with the current menu state.
            done: function (seq, menu) {
                if (!pending || typeof seq !== 'number') return;
                if (seq === pending.seq) finish(menu === true);
                else if (seq > pending.seq) finish(opts.currentMenu());
            },
            // A new socket: its count starts again, so an old wait is void.
            reset: function () {
                if (pending) opts.clearTimeout(pending.timer);
                pending = null;
            }
        };
    }

    return {
        FOCUSABLE_CONTROL_SELECTOR: FOCUSABLE_CONTROL_SELECTOR,
        MENU_WAIT_CEILING_MS: MENU_WAIT_CEILING_MS,
        isTypingKey: isTypingKey,
        isEditableField: isEditableField,
        isKeyboardControl: isKeyboardControl,
        isPointerFocused: isPointerFocused,
        watch: watch,
        shouldFocusCommandBar: shouldFocusCommandBar,
        isChatLine: isChatLine,
        keyboardAfterSend: keyboardAfterSend,
        blurAfterWait: blurAfterWait,
        createSendWait: createSendWait
    };
```

- [ ] **Step 4: Run the pure checks.** The page checks still fail until Task 6; every other check must pass.

Run: `node tools/jstest/hotinput.test.js`
Expected: only the six page checks added in Step 1 FAIL.

- [ ] **Step 5: Commit** (with Task 6, so no commit leaves CI red). Go straight to Task 6.

---

### Task 6: Wire the wait into the page

**Files:**
- Modify: `WP:1013-1018` (GMCP handlers), `:2943-2963` (`socket.onopen`), `:3218-3244` (Enter handler, `SendData`), and a declaration above `:3171`

- [ ] **Step 1: Count text frames.** Above `textInput.addEventListener('keydown', function(event) {` (`WP:3171`) add:

```js
        // Plan 6 (R7): the server numbers this connection's text frames and
        // answers each typed line with Char.Input.Done {seq, menu}. Count the
        // same frames here: every string sent on the socket (SendData and the
        // copyover token). GMCP goes as binary and is not counted.
        var linesSent = 0;

        // After an in-game command that is not chat, the phone keyboard waits
        // for that line's marker and stays up if the game left a menu or
        // prompt open (spec 2026-11-14, R2, R5, R6).
        var sendWait = window.HotInput ? window.HotInput.createSendWait({
            setTimeout: function (f, ms) { return window.setTimeout(f, ms); },
            clearTimeout: function (t) { window.clearTimeout(t); },
            ceilingMs: window.HotInput.MENU_WAIT_CEILING_MS,
            currentMenu: function () { return menuOpen; },
            settle: function (menu) {
                if (window.HotInput.blurAfterWait({
                    menu: menu,
                    focused: document.activeElement === textInput,
                    value: textInput.value
                })) {
                    textInput.blur();
                }
            }
        }) : null;
```

In `SendData`, count right before the send:

```js
            linesSent++;
            socket.send(dataToSend);
```

In `socket.onopen` (`WP:2943`), make the first two statements:

```js
            socket.onopen = function() {
                linesSent = 0;
                if (sendWait) sendWait.reset();
```

and count the copyover token:

```js
                    linesSent++;
                    socket.send(t);
```

- [ ] **Step 2: The Enter handler.** Replace from `var sent = SendData(event.target.value);` through the closing `}` of the `if (window.HotInput && window.HotInput.shouldHideKeyboardAfterSend({ ... })) { textInput.blur(); }` block with:

```js
                var line = event.target.value;
                var sent = SendData(line);
                if ( sent ) {
                    event.target.value = '';
                } else {
                    term.writeln("Not connected to the server. Click Enter Game to play.");
                }
                // Phones: chat keeps the keyboard; any other in-game command
                // waits for the game's answer and keeps it only if a menu or
                // prompt opened (plan 6, R1 to R7). Login, password and menu
                // answers keep it (#768 ruling). HotInput decides.
                if (window.HotInput && sendWait && window.HotInput.keyboardAfterSend({
                    touchOnly: phoneTouchOnly(), phoneWidth: phoneWidth(), sent: sent,
                    inGame: inGame, menuOpen: menuOpen, password: textInput.type === "password",
                    line: line
                }) === 'wait') {
                    sendWait.start(linesSent);
                }
```

- [ ] **Step 3: The marker handler.** After the `"Char.Menu"` handler (`WP:1013-1018`) add:

```js
            "Char.Input.Done": function() {
                // The server finished one of our typed lines (plan 6): its
                // number and whether a menu or prompt is open. Never its text.
                var d = ((GMCPStructs["Char"] || {}).Input || {}).Done || {};
                if (sendWait) sendWait.done(d.seq, d.menu === true);
            },
```

- [ ] **Step 4: Run the tests**

Run: `node tools/jstest/hotinput.test.js` and every other jstest: `for t in tools/jstest/*.test.js; do node "$t" > /dev/null || echo "FAIL $t"; done`
Expected: `hotinput.test.js` all `ok`; no `FAIL` lines.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/hotinput.js _datafiles/html/public/webclient-pure.html tools/jstest/hotinput.test.js
git commit -m "feat(webclient): the phone keyboard stays up for chat and menus (plan 6)"
```

---

### Task 7: The open tab is 30% where it covers game text

**Files:**
- Modify: `CSS:684-706`, `:786-799`
- Modify: `WP:2704-2716`; `_datafiles/html/public/webclient.html:26-38`
- Modify: `tools/jstest/hotinput.test.js` (CSS checks)

- [ ] **Step 1: Write the failing checks.** Append to the CSS section of `tools/jstest/hotinput.test.js` (after the `a folded tab drops its own height` check):

```js
// Plan 6 R3: an open tab is at most 30% of the visible height where it
// covers game text (phone, slim); column keeps its older cap.
check('phone and slim tabs are 30% of the screen',
    /--drawer-cap: max\(80px, 30vh\);/.test(css) && /--drawer-cap: max\(80px, 30dvh\);/.test(css), true);
check('with the keyboard up, 30% of what is visible',
    /html\.keyboard-open #dash-drawer \{[^}]*--drawer-cap: max\(60px, calc\(0\.3 \* var\(--visible-h, 100dvh\)\)\);/.test(css), true);
check('column keeps its cap, keyboard down',
    /html\.layout-column #dash-drawer \{\s*--drawer-cap: max\(80px, min\(50vh,/.test(css), true);
check('column keeps its cap, keyboard up',
    /html\.layout-column\.keyboard-open #dash-drawer \{\s*--drawer-cap: max\(60px, min\(50vh,/.test(css), true);
check('the page opened on its own sets --visible-h',
    /setProperty\('--visible-h', Math\.floor\(visible\) \+ 'px'\)/.test(page), true);
var outer = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient.html'
), 'utf8');
check('the framed page gets --visible-h from webclient.html',
    /setProperty\('--visible-h', Math\.floor\(visible\) \+ 'px'\)/.test(outer), true);
```

- [ ] **Step 2: Run them to see them fail**

Run: `node tools/jstest/hotinput.test.js`
Expected: the six new checks FAIL.

- [ ] **Step 3: The CSS.** Replace `CSS:684-693` (from `/* The tallest an open tab may get: half the screen,` through the `--drawer-cap: max(80px, min(50vh, ...` line) with:

```css
  /* The tallest an open tab may get where it covers game text (phone and
     slim): 30% of the screen, so the game text above it stays in view
     (layout spec ruling 12; plan 6 R3). Never under 80px, so on a very
     short screen an open tab still shows more than its title bar. */
  --drawer-cap: max(80px, 30vh);
  /* Column's cap (below) stops this far under the top: the session bar
     (64px with its gap), the Game title bar (28px) and about two lines. */
  --drawer-keep: 136px;
```

In the `@supports (height: 1dvh)` block right after (`CSS:698-706`), the declaration becomes:

```css
    --drawer-cap: max(80px, 30dvh);
```

and after that block's closing `}` add:

```css
/* Column puts an open tab beside the game, over the panel column, not over
   the game text, so it keeps the older, taller cap (plan 6 R3): half the
   screen, stopping --drawer-keep below the top. */
html.layout-column #dash-drawer {
  --drawer-cap: max(80px, min(50vh, calc(100vh - var(--cmdbox-h) - var(--tabbar-h) - var(--tabname-h) - var(--drawer-keep))));
}
@supports (height: 1dvh) {
  html.layout-column #dash-drawer {
    --drawer-cap: max(80px, min(50dvh, calc(100dvh - var(--cmdbox-h) - var(--tabbar-h) - var(--tabname-h) - var(--drawer-keep))));
  }
}
```

Replace `CSS:786-799` (from `/* With the keyboard up, an open tab sits just above the command box.` through the closing `}` of its `@supports` block) with:

```css
/* With the keyboard up, an open tab sits just above the command box. The tab
   bar and name strip are hidden then. Phone and slim: 30% of what is visible
   above the keyboard (plan 6 R3). --visible-h is set by the keyboard watch
   (webclient.html fitFrameHeight, or this page's own when opened alone);
   vh cannot be used, because opened alone the page keeps vh at the full
   layout height. Never under 60px, so a Comms being typed into keeps its
   newest lines. */
html.keyboard-open #dash-drawer {
  bottom: var(--cmdbox-h);
  --drawer-cap: max(60px, calc(0.3 * var(--visible-h, 100dvh)));
}
/* Column: the tab is beside the game, so it keeps half of what is visible,
   leaving about two lines of game text (40px) above it. vh first; the dvh
   twin is below. */
html.layout-column.keyboard-open #dash-drawer {
  --drawer-cap: max(60px, min(50vh, calc(100vh - var(--cmdbox-h) - 40px)));
}
@supports (height: 1dvh) {
  html.layout-column.keyboard-open #dash-drawer {
    --drawer-cap: max(60px, min(50dvh, calc(100dvh - var(--cmdbox-h) - 40px)));
  }
}
```

- [ ] **Step 4: `--visible-h`.** In `WP`'s own keyboard watch (`WP:2708-2716`), after `document.body.style.height = Math.floor(visible) + 'px';` add:

```js
                // The tab cap reads this while the keyboard is up (dashboard.css).
                document.documentElement.style.setProperty('--visible-h', Math.floor(visible) + 'px');
```

In `webclient.html` `fitFrameHeight`, inside the `if (frame.contentDocument && frame.contentDocument.documentElement) {` block, after the `classList.toggle('keyboard-open', ...)` call, add:

```js
            // The game's open-tab cap reads this while the keyboard is up
            // (dashboard.css, plan 6). The frame is already this tall.
            if (typeof visible === 'number') {
                frame.contentDocument.documentElement.style.setProperty('--visible-h', Math.floor(visible) + 'px');
            }
```

- [ ] **Step 5: Run the tests and commit**

Run: `node tools/jstest/hotinput.test.js`
Expected: all `ok`.

```bash
git add _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/webclient-pure.html _datafiles/html/public/webclient.html tools/jstest/hotinput.test.js
git commit -m "feat(webclient): an open tab takes 30% where it covers the game (plan 6)"
```

---

### Task 8: The Close keyboard X

**Files:**
- Modify: `WP:299-303`, and a wiring block after the `sendWait` declaration (Task 6)
- Modify: `CSS` (after `#panel-cmd .dash-panel-body`, `CSS:210`)
- Modify: `tools/jstest/hotinput.test.js`

- [ ] **Step 1: Write the failing checks** (append to `hotinput.test.js` before the result section):

```js
// Plan 6, ruling 14: a Close keyboard X at the end of the command box, shown
// only while the on-screen keyboard is open, at least 44px.
var cmdAt = page.indexOf('<section class="dash-panel" id="panel-cmd"');
var xAt = page.indexOf('id="cmd-close-kbd"');
check('the X is inside the command box, after the input',
    cmdAt >= 0 && xAt > page.indexOf('id="command-input"', cmdAt) && xAt < page.indexOf('</section>', cmdAt), true);
check('the X is a real button named for screen readers',
    /<button type="button" id="cmd-close-kbd" class="brass" aria-label="Close keyboard"/.test(page), true);
check('the X is hidden unless the keyboard is open',
    /#cmd-close-kbd \{[^}]*display: none;/.test(css) && /html\.keyboard-open #cmd-close-kbd \{ display: flex; \}/.test(css), true);
check('the X is 48px', /#cmd-close-kbd \{[^}]*width: 48px;[^}]*height: 48px;/.test(css), true);
check('the X closes the keyboard without taking focus first',
    /closeKbd\.addEventListener\('pointerdown', function \(e\) \{ e\.preventDefault\(\); \}\);/.test(page)
    && /closeKbd\.addEventListener\('click', function \(\) \{ textInput\.blur\(\); \}\);/.test(page), true);
```

- [ ] **Step 2: Run them to see them fail**

Run: `node tools/jstest/hotinput.test.js`
Expected: the five new checks FAIL.

- [ ] **Step 3: Implement.** `WP:299-303` becomes:

```html
        <section class="dash-panel" id="panel-cmd" data-panel="cmd">
          <div class="dash-panel-body">
            <input type="text" id="command-input" placeholder="Enter command..." style="flex:1">
            <!-- Plan 6 (ruling 14): closes the on-screen keyboard; shown only
                 while it is open (dashboard.css). -->
            <button type="button" id="cmd-close-kbd" class="brass" aria-label="Close keyboard" title="Close keyboard"><svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true"><path d="M4 4l12 12M16 4L4 16"/></svg></button>
          </div>
        </section>
```

After the `sendWait` declaration from Task 6 add:

```js
        // The Close keyboard X (plan 6, ruling 14). pointerdown keeps focus
        // where it is, so the tap does not move focus before the click.
        (function () {
            var closeKbd = document.getElementById('cmd-close-kbd');
            if (!closeKbd) return;
            closeKbd.addEventListener('pointerdown', function (e) { e.preventDefault(); });
            closeKbd.addEventListener('click', function () { textInput.blur(); });
        })();
```

In `CSS`, after `#panel-cmd .dash-panel-body { display: flex; padding: 4px; }` (`CSS:210`):

```css
/* Plan 6 (ruling 14): the Close keyboard X at the end of the command box,
   only while the on-screen keyboard is open (viewportfit.js), so the box is
   full width the rest of the time. 48px: the command box's height. */
#cmd-close-kbd {
  display: none;
  flex: 0 0 48px;
  width: 48px;
  height: 48px;
  margin-left: 4px;
  padding: 0;
  align-items: center;
  justify-content: center;
}
html.keyboard-open #cmd-close-kbd { display: flex; }
```

- [ ] **Step 4: Run the tests and commit**

Run: `node tools/jstest/hotinput.test.js`
Expected: all `ok`.

```bash
git add _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/css/dashboard.css tools/jstest/hotinput.test.js
git commit -m "feat(webclient): a Close keyboard X in the command box (plan 6)"
```

---

### Task 9: The sideways toast

**Files:**
- Modify: `JS/viewportfit.js:22-51`, `:100-110`
- Modify: `JS/dashboard.js:165-200`, `:271-282`
- Modify: `CSS:1758-1768`
- Modify: `tools/jstest/viewportfit.test.js`

- [ ] **Step 1: Write the failing tests.** Append to `tools/jstest/viewportfit.test.js` before its result lines:

```js
// Plan 6, ruling 15: a phone turned sideways, judged from the screen so the
// keyboard cannot trigger it.
check('sideways phone', VF.isSidewaysPhone(844, 390, 'landscape-primary', true), true);
check('upright phone is not sideways', VF.isSidewaysPhone(390, 844, 'portrait-primary', true), false);
check('iOS sizes do not swap: the orientation wins', VF.isSidewaysPhone(390, 844, 'landscape-secondary', true), true);
check('no orientation API, sideways', VF.isSidewaysPhone(844, 390, undefined, true), true);
check('a sideways tablet is not a sideways phone', VF.isSidewaysPhone(1024, 768, 'landscape-primary', true), false);
check('a desktop is not a sideways phone', VF.isSidewaysPhone(1920, 1080, 'landscape-primary', false), false);

check('orientation from the API',
    VF.orientationType({ screen: { orientation: { type: 'landscape-primary' } } }), 'landscape-primary');
check('iOS before 16.4: window.orientation 90 is landscape',
    VF.orientationType({ screen: {}, orientation: 90 }), 'landscape-primary');
check('iOS before 16.4: window.orientation 0 is portrait',
    VF.orientationType({ screen: {}, orientation: 0 }), 'portrait-primary');
check('no signal at all', VF.orientationType({ screen: {} }), undefined);

// Once per browser tab: sessionStorage survives a reload but not a new tab.
(function () {
    var store = {};
    var ss = { getItem: function (k) { return k in store ? store[k] : null; }, setItem: function (k, v) { store[k] = String(v); } };
    var memo = {};
    check('first time in a tab: show it', VF.takeSidewaysToast(ss, memo), true);
    check('again on this page: not again', VF.takeSidewaysToast(ss, memo), false);
    check('after a reload in the same tab: not again', VF.takeSidewaysToast(ss, {}), false);
    check('a new tab: show it', VF.takeSidewaysToast({ getItem: function () { return null; }, setItem: function () {} }, {}), true);
    var blocked = { getItem: function () { throw new Error('blocked'); }, setItem: function () { throw new Error('blocked'); } };
    var m2 = {};
    check('storage blocked: show it once', VF.takeSidewaysToast(blocked, m2), true);
    check('storage blocked: not twice on one page', VF.takeSidewaysToast(blocked, m2), false);
    check('no storage at all: show it once', VF.takeSidewaysToast(null, {}), true);
})();
check('the toast words', VF.SIDEWAYS_TOAST, 'This game is easier to play with your phone held upright.');

// The dashboard wires it, and its toast can be lifted and held longer.
var dashSrc = fs.readFileSync(path.join(PUBLIC, 'static', 'js', 'dashboard.js'), 'utf8');
check('the dashboard watches for a sideways phone',
    /this\._watchSideways && this\._watchSideways\(\);/.test(dashSrc)
    && /VF\.takeSidewaysToast\(store, self\._sidewaysMemo\)/.test(dashSrc), true);
check('the sideways toast is lifted above the command box and shown 4s',
    /self\._toast\(VF\.SIDEWAYS_TOAST, \{ ms: 4000, lifted: true \}\)/.test(dashSrc), true);
var dashCss = fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8');
check('a lifted toast sits above the command box and tab bar',
    /\.dash-toast\.lifted \{ bottom: calc\(var\(--cmdbox-h, 64px\) \+ var\(--tabbar-h, 64px\) \+ 12px\); \}/.test(dashCss), true);
```

- [ ] **Step 2: Run them to see them fail**

Run: `node tools/jstest/viewportfit.test.js`
Expected: FAIL (`VF.isSidewaysPhone is not a function`).

- [ ] **Step 3: `viewportfit.js`.** Replace `isUprightPhone` and `markUprightPhone` (`JS/viewportfit.js:22-44`) with:

```js
    // An upright phone reads 14/18 instead of 16/20 (gomud.css). Judged from the
    // screen, like isPhone. iOS does not swap screen.width and height when the
    // phone turns, so the orientation API comes first. coarse: the screen is
    // a touch screen (pointer: coarse). A desktop monitor turned upright, or
    // zoomed in, can have a short side under 600 too; it keeps 16px, as the
    // layout ladder treats it as a desktop (plan 5, decision 13).
    function isUprightPhone(screenW, screenH, orientationType, coarse) {
        var landscape = orientationType ? orientationType.indexOf('landscape') === 0 : screenW >= screenH;
        return !!coarse && isPhone(screenW, screenH) && !landscape;
    }

    // A phone held sideways (plan 6, ruling 15): the mirror of isUprightPhone.
    // Judged from the screen, so the on-screen keyboard cannot trigger it.
    function isSidewaysPhone(screenW, screenH, orientationType, coarse) {
        var landscape = orientationType ? orientationType.indexOf('landscape') === 0 : screenW >= screenH;
        return !!coarse && isPhone(screenW, screenH) && landscape;
    }

    // The screen's orientation type, or undefined where the browser gives no
    // signal. iOS before 16.4 has no screen.orientation; window.orientation is
    // its only true signal.
    function orientationType(win) {
        var s = win.screen || {};
        var type = s.orientation && s.orientation.type;
        if (!type && typeof win.orientation === 'number') {
            type = Math.abs(win.orientation) === 90 ? 'landscape-primary' : 'portrait-primary';
        }
        return type;
    }

    function markUprightPhone(win) {
        var s = win.screen;
        var coarse = !!(win.matchMedia && win.matchMedia('(pointer: coarse)').matches);
        win.document.documentElement.classList.toggle('upright-phone',
            isUprightPhone(s.width, s.height, orientationType(win), coarse));
    }

    // The sideways note (layout spec ruling 15), once per browser tab:
    // sessionStorage survives a reload but not a new tab. memo stops a repeat
    // on this page where storage is missing or blocked.
    var SIDEWAYS_TOAST = 'This game is easier to play with your phone held upright.';
    var SIDEWAYS_KEY = 'dogmud.sidewaysToast';
    function takeSidewaysToast(storage, memo) {
        if (memo.shown) return false;
        memo.shown = true;
        try {
            if (storage.getItem(SIDEWAYS_KEY) === '1') return false;
            storage.setItem(SIDEWAYS_KEY, '1');
        } catch (e) { /* missing or blocked storage: memo alone */ }
        return true;
    }
```

Add to the returned object (`JS/viewportfit.js:100-110`):

```js
        isSidewaysPhone: isSidewaysPhone,
        orientationType: orientationType,
        takeSidewaysToast: takeSidewaysToast,
        SIDEWAYS_TOAST: SIDEWAYS_TOAST,
```

- [ ] **Step 4: `dashboard.js`.** Replace `_toast` (`JS/dashboard.js:271-282`) with:

```js
    // Brief, self-dismissing notification (bottom-center, leather-styled).
    // opts: { ms } how long it shows (2.6s by default); { lifted } sits it
    // above the command box and tab bar, for phones (plan 6).
    _toast: function (msg, opts) {
      opts = opts || {};
      var t = document.createElement("div");
      t.className = "dash-toast" + (opts.lifted ? " lifted" : "");
      t.setAttribute("role", "status");
      t.textContent = msg;
      document.body.appendChild(t);
      requestAnimationFrame(function () { t.classList.add("show"); });
      setTimeout(function () {
        t.classList.remove("show");
        setTimeout(function () { if (t.parentNode) t.parentNode.removeChild(t); }, 400);
      }, opts.ms || 2600);
    },

    // A phone turned sideways gets one note per browser tab (layout spec
    // ruling 15, plan 6), at load or when it turns.
    _sidewaysMemo: {},
    _watchSideways: function () {
      var self = this;
      var VF = window.ViewportFit;
      if (!VF) return;
      var check = function () {
        var coarse = !!(window.matchMedia && window.matchMedia("(pointer: coarse)").matches);
        if (!VF.isSidewaysPhone(screen.width, screen.height, VF.orientationType(window), coarse)) return;
        var store = null;
        try { store = window.sessionStorage; } catch (e) { /* blocked: memo alone */ }
        if (!VF.takeSidewaysToast(store, self._sidewaysMemo)) return;
        self._toast(VF.SIDEWAYS_TOAST, { ms: 4000, lifted: true });
      };
      check();
      window.addEventListener("orientationchange", check);
      window.addEventListener("resize", check);
    },
```

In `init` (`JS/dashboard.js:197-199`), after `this.restoreLayout  && this.restoreLayout();` add:

```js
      // The sideways phone note (plan 6).
      this._watchSideways && this._watchSideways();
```

- [ ] **Step 5: CSS.** After `.dash-toast.show { ... }` (`CSS:1768`):

```css
/* Plan 6: on a phone the toast sits above the command box and the tab bar
   rather than over the round buttons. The fallbacks are the phone sizes:
   the toast is on body, outside the elements that set these. */
.dash-toast.lifted { bottom: calc(var(--cmdbox-h, 64px) + var(--tabbar-h, 64px) + 12px); }
```

- [ ] **Step 6: Run the tests and commit**

Run: `node tools/jstest/viewportfit.test.js` and `node tools/jstest/dashboard-popout-layout.test.js`
Expected: all `ok` (the popout test stubs `_toast`, and its fake window must tolerate `_watchSideways`: if it throws on `screen` or `matchMedia`, add `screen: { width: 1920, height: 1080 }` and a `matchMedia` returning `{ matches: false }` to that test's fake window).

```bash
git add _datafiles/html/public/static/js/viewportfit.js _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/static/css/dashboard.css tools/jstest/viewportfit.test.js tools/jstest/dashboard-popout-layout.test.js
git commit -m "feat(webclient): a sideways phone gets one note to hold it upright (plan 6)"
```

---

### Task 10: Check it in a browser

No code unless a check fails.

- [ ] **Step 1: Boot a local server from the worktree** on private ports (the `dogmud-shipping` boot-check recipe, `HttpPort: 18090`), instance saves wiped first.

- [ ] **Step 2: Headless check.** Write `<scratchpad>/p6-check/check.js` and run it with `node`. Playwright comes from `C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright` (1.59.1; its headless Chromium opens no window). It measures the drawer cap on the page opened on its own; the keyboard is simulated by setting the class and `--visible-h` the watch would set.

```js
const { chromium } = require('C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright');
const URL = 'http://localhost:18090/webclient-pure.html';
(async () => {
  const browser = await chromium.launch();
  const out = [];
  async function capOf(page, panel) {
    await page.click(`#dash-tabbar .gtab[data-panel="${panel}"]`);
    await page.waitForTimeout(300);
    return page.evaluate(() => {
      const d = document.getElementById('dash-drawer');
      return { cap: getComputedStyle(d).maxHeight, h: Math.round(d.getBoundingClientRect().height),
               mode: document.getElementById('dashboard').dataset.mode };
    });
  }
  // Phone, keyboard down: expect max-height 253.2px (30% of 844).
  let ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 3 });
  let page = await ctx.newPage();
  await page.goto(URL); await page.waitForTimeout(1500);
  out.push(['phone, keyboard down, Map', await capOf(page, 'map')]);
  // Phone, keyboard up (visible 508): expect max-height 152.4px.
  await page.evaluate(() => {
    document.documentElement.classList.add('keyboard-open');
    document.documentElement.style.setProperty('--visible-h', '508px');
  });
  await page.waitForTimeout(300);
  out.push(['phone, keyboard up', await page.evaluate(() => getComputedStyle(document.getElementById('dash-drawer')).maxHeight)]);
  out.push(['X shown with keyboard up', await page.evaluate(() => getComputedStyle(document.getElementById('cmd-close-kbd')).display)]);
  await ctx.close();
  // Column layout, 1000x700, no touch: expect the older cap (min(350, 700-64-60-36-136=404) = 350px).
  ctx = await browser.newContext({ viewport: { width: 1000, height: 700 } });
  page = await ctx.newPage();
  await page.goto(URL); await page.waitForTimeout(1500);
  out.push(['column, Map', await capOf(page, 'map')]);
  out.push(['X hidden on desktop', await page.evaluate(() => getComputedStyle(document.getElementById('cmd-close-kbd')).display)]);
  await ctx.close();
  // Sideways phone: expect one lifted toast.
  ctx = await browser.newContext({ viewport: { width: 844, height: 390 }, isMobile: true, hasTouch: true, screen: { width: 844, height: 390 } });
  page = await ctx.newPage();
  await page.goto(URL); await page.waitForTimeout(800);
  out.push(['sideways toast', await page.evaluate(() => Array.from(document.querySelectorAll('.dash-toast')).map(t => t.className + ' | ' + t.textContent))]);
  await page.reload(); await page.waitForTimeout(800);
  out.push(['after reload, no toast', await page.evaluate(() => document.querySelectorAll('.dash-toast').length)]);
  await ctx.close();
  await browser.close();
  console.log(JSON.stringify(out, null, 2));
})();
```

Expected: phone keyboard down `253.2px` in mode `phone`; keyboard up `152.4px`; X `flex` with the keyboard up and `none` on the desktop; column `350px` in mode `column`; one toast with class `dash-toast lifted show` and the ruling's words; `0` toasts after the reload. Any other value is a bug to fix before going on. If the column window picks another mode, compute the expected cap for the mode shown and record it.

- [ ] **Step 3: The marker, live.** Log in at the phone size (a throwaway character), open the browser console in a headed run or log from `page.on('websocket')` frames, and type `look`, `say hi`, `password`. Record the `Char.Input.Done` frames: `look` gives `{"seq":n,"menu":false}`; `say hi` gives one too (the client just does not wait for it); `password` gives `"menu":true`; then type a wrong current password: its frame carries only `seq` and `menu`, and no frame anywhere contains the typed password (search all received frames for it). Leave the prompt with an empty line.

---

### Task 11: Docs

- [ ] **Step 1:** `internal/events/context.md`, after the "Menu Events" section: a section "Input numbering and InputDone (plan 6)" with the `Input.Seq` field and the `InputDone` type as in Task 1, and: "`Seq` numbers a web client's text frames from the socket's open (`main.go` `countLineFrame`); the client counts the same frames. `processInput` queues `InputDone` at priority 1 for a typed line with a number, after all its text; `modules/gmcp` pushes it as `Char.Input.Done`. It never carries the line's text (spec 2026-11-14, R7)."
- [ ] **Step 2:** `modules/gmcp/context.md`: a table row after `gmcp.Menu.go`: `| gmcp.InputDone.go | Char.Input.Done | A typed web client line is done: its number and whether a menu or prompt is open ({seq, menu}); see "Char.Input.Done" below |`, and a section after "Char.Menu": payload, web client only, priority reasoning (the event pops at 1, after the command's text; the frame follows), never text (R7), and that the phone keyboard waits on it (`hotinput.js` `createSendWait`).
- [ ] **Step 3:** `docs/PATCH_NOTES.md`, a new top entry dated the day it ships:

```markdown
## 2026-11-14: Playing on a phone

- An open tab now takes less of the screen on a phone, so more of the game stays in view.
- The keyboard stays up while you chat, and while you talk to someone or answer a menu. It still closes after other commands, so you can read what happened.
- A new X at the end of the command box closes the keyboard whenever you like.
- Turn your phone sideways and the game suggests holding it upright, once per visit.
```

- [ ] **Step 4:** `docs/README.md`: in the `tools/jstest/` row, add "Added 2026-11-14 (web client base plan 6): `hotinput.test.js` covers the phone keyboard after a send (chat keeps it, anything else waits for `Char.Input.Done`), the 30% tab cap and the Close keyboard X; `viewportfit.test.js` covers the sideways phone and its once-per-tab note." Add rows for `world_input_done_test.go` and `modules/gmcp/gmcp.InputDone.go` beside the nearest listed root test and gmcp file, following each section's pattern (search `grep -n "world_unhandled_mob_command_test\|gmcp.ClientDisplay" docs/README.md`; if neither section lists files of that kind, add the rows to the section that lists `static/js/displayreport.js`).
- [ ] **Step 5: Commit**

```bash
git add internal/events/context.md modules/gmcp/context.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs: plan 6, phones"
```

---

### Task 12: Gates, owner check, PR

- [ ] **Step 1: Gates** (the `dogmud-shipping` pre-push order): `gofmt -l internal/ modules/ *.go` empty; `go build ./...`; `golangci-lint run --new-from-merge-base=origin/master` 0 issues; `go test ./...`; every `tools/jstest/*.test.js`; boot check `Server Ready` 1 and 0 panics; `LogToFile: false`.

- [ ] **Step 2: Owner check.** Start a local server from the worktree with his usual overrides (copy his `config-overrides.yaml` into the worktree), instance saves wiped first, reachable from his phone over Wi-Fi (look up the LAN IP, memory `reference-lan-test-server-access`). Ordinary player steps only:
  1. On your phone, open the web client upright and log in. Tap the Map button: the map opens over the bottom of the game and about two thirds of the game text stays in view.
  2. Tap the command box. The keyboard comes up and an X appears at the end of the box. Tap the X: the keyboard goes away.
  3. Type `say hello`. The keyboard stays up.
  4. Type `party chat hi` (or `party say hi`). The keyboard stays up. Type `party invite` with a name: it closes.
  5. Walk to an NPC with topics and type `talk <name>`. The keyboard stays up while the menu appears; answer with a number without tapping the box again.
  6. Type `look`. The keyboard closes once the room has printed.
  7. With the keyboard up, open Comms: it sits above the command box and the game text above it is still readable.
  8. Turn the phone sideways: a note suggests holding it upright. Turn it back and sideways again: no second note. Reload the page sideways: no note.
  9. On a laptop browser, narrow the window until the panels go to one column, open the Map: it is as tall as before.

- [ ] **Step 3: Real devices.** Post the branch and these steps on #834 for Malia (iOS and Android, once on mobile data for step 5).

- [ ] **Step 4: Push and open the PR**

```bash
git fetch origin && git rebase origin/master
git push -u origin feature/webclient-base-6-phones
gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-6-phones --title "feat(webclient): phones, tab height, keyboard, X, sideways note (web client base plan 6)" --body-file <scratchpad>/pr6-body.md
```

The body lists the owner rulings R1 to R7, the decisions above, the browser check numbers, the device results, and the deploy note (no migration, no config; a new GMCP module `Char.Input.Done` to the web client only; players see the change on their next page load). No closing keywords for #783 or #834. It ends with the Claude Code attribution line.
