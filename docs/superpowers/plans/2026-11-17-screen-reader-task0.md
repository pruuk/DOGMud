# Screen reader opt-in (touch arc Task 0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A blind player can turn on speech in the web client before the first prompt, without anything changing for sighted players; telnet clients that declare a screen reader through MTTS are recognised; turning the setting on saves light combat text.

**Architecture:** The web client gets a small `ScreenReader` module (UMD, node-tested) that the command box's spoken label points to: typing `screen reader on` switches xterm 4.19's `screenReaderMode` at runtime, remembers it in `localStorage`, and reports it over `Client.Display`. The server keeps the report per connection in `ClientSettings`, uses it (and MTTS on telnet) to skip or answer the creation question and to turn an account's setting on at login, and sends the account setting back in `Char.Info`. One `UserRecord.TurnOnScreenReader` is the only way the setting turns on.

**Tech Stack:** Go (server), plain JS in `webclient-pure.html` plus a UMD module, node tests in `tools/jstest`, Playwright for the headless check.

**Spec:** `docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md` (approved 2026-11-17). Branch `feature/screen-reader-task0`, main checkout.

---

## Rules for every task (read before starting)

- Edit files with the Edit and Write tools only. Never rewrite a repo file from the shell (no `sed -i`, no Python `open(...,'w')`, no `>` into a tracked file). A hook blocks it.
- `git add` named paths only; never `-A`, `.` or `commit -a`. Never `--no-verify`. Never stash.
- Commit messages go through a quoted heredoc and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Every new Go declaration needs a doc comment (lint gate #360). Use `min`/`max`, `strings.CutPrefix` and other modern idioms (lint gate #348).
- No em or en dashes in any text you write, including comments and player text.
- Tests run from the repo root: `go test ./internal/<pkg>/ -run <Name> -count=1`.

## File map

| File | Change |
|---|---|
| `internal/connections/clientsettings.go` | `ScreenReader`, `MTTSReported`, `TermTypeReplies`, `TermTypeLast` on `ClientSettings` |
| `internal/users/screenreader.go` (new) | `TurnOnScreenReader`, the combat line |
| `internal/users/screenreader_test.go` (new) | its tests |
| `modules/gmcp/gmcp.ClientDisplay.go` | read `screenreader` from `Client.Display` |
| `modules/gmcp/gmcp.ClientDisplay_test.go` | tests |
| `modules/gmcp/gmcp.Char.go` | `Char.Info.screenreader`; push `Char.Info` on the setting changing |
| `modules/gmcp/gmcp.CharInfo_screenreader_test.go` (new) | tests |
| `internal/usercommands/set.go` | `set screenreader` turns on through `TurnOnScreenReader` |
| `internal/inputhandlers/login_prompt_handler.go` | connection facts written into results before each condition |
| `internal/inputhandlers/login.go` | `loginSteps()`, creation skip and answer, login turn-on |
| `internal/inputhandlers/login_screenreader_test.go` (new) | tests |
| `internal/term/mtts.go` (new), `internal/term/mtts_test.go` (new) | terminal type commands, `ParseMTTS` |
| `internal/inputhandlers/term_iac.go`, `term_iac_test.go` | the MTTS cycle |
| `main.go` | send `IAC DO TTYPE` on telnet connect |
| `_datafiles/html/public/static/js/screenreader.js` (new) | the client module |
| `tools/jstest/screenreader.test.js` (new) | its tests |
| `_datafiles/html/public/webclient-pure.html` | hint, label, script, wiring |
| `_datafiles/world/dogmud/templates/help/set.template` | `set screenreader` row |
| `docs/PATCH_NOTES.md`, `modules/gmcp/context.md`, `internal/connections/context.md`, `internal/inputhandlers/context.md`, `internal/users/context.md`, `docs/README.md` | docs |

---

### Task 1: `ClientSettings` fields and `TurnOnScreenReader`

**Files:**
- Modify: `internal/connections/clientsettings.go:10-15`
- Create: `internal/users/screenreader.go`, `internal/users/screenreader_test.go`
- Modify: `internal/usercommands/set.go:216-230`

- [ ] **Step 1: Add the `ClientSettings` fields**

Replace the struct at `internal/connections/clientsettings.go:10-15` with:

```go
// ClientSettings are what a connection's client has told the server about
// itself. They belong to the connection, not the account.
type ClientSettings struct {
	Display           DisplaySettings
	MSPEnabled        bool // Do they accept sound in their client?
	SendTelnetGoAhead bool // Defaults false, should we send a IAC GA after prompts?
	AsciiMode         bool // Convert UTF-8 decorative chars to ASCII equivalents?
	// ScreenReader is true when the client said its player uses a screen
	// reader: the web client's "screen reader on" (Client.Display) or a
	// telnet client's MTTS bit 64.
	ScreenReader bool
	// MTTSReported is true once a telnet client answered MTTS, with the
	// screen reader bit set or not. Either way it answers the creation
	// question, so the question is not asked.
	MTTSReported bool
	// TermTypeReplies counts the client's terminal type replies, and
	// TermTypeLast is the latest one; the MTTS cycle stops on a repeat.
	TermTypeReplies int
	TermTypeLast    string
}
```

- [ ] **Step 2: Write the failing test for `TurnOnScreenReader`**

Create `internal/users/screenreader_test.go`:

```go
package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

func drainSettingChanges(t *testing.T) []string {
	t.Helper()
	names := []string{}
	for _, e := range events.DrainQueuedByTypeForTest(`UserSettingChanged`) {
		names = append(names, e.(events.UserSettingChanged).Name)
	}
	return names
}

// Turning the setting on saves light combat text from full (stored empty)
// and from medium, and fires both setting changes.
func TestTurnOnScreenReader_SavesLightCombat(t *testing.T) {
	for _, start := range []string{``, `medium`} {
		drainSettingChanges(t)
		u := &UserRecord{UserId: 7, CombatVerbosity: start}
		require.True(t, u.TurnOnScreenReader(), start)
		require.True(t, u.ScreenReader)
		require.Equal(t, `light`, u.CombatVerbosity)
		require.Equal(t, []string{`screenreader`, `combatverbosity`}, drainSettingChanges(t))
	}
}

// Already light: the setting turns on and the combat level is not touched.
func TestTurnOnScreenReader_AlreadyLight(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7, CombatVerbosity: `light`}
	require.False(t, u.TurnOnScreenReader())
	require.True(t, u.ScreenReader)
	require.Equal(t, []string{`screenreader`}, drainSettingChanges(t))
}

// Already on: nothing changes and nothing fires.
func TestTurnOnScreenReader_AlreadyOn(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7, ScreenReader: true}
	require.False(t, u.TurnOnScreenReader())
	require.Equal(t, ``, u.CombatVerbosity)
	require.Empty(t, drainSettingChanges(t))
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/users/ -run TestTurnOnScreenReader -count=1`
Expected: FAIL, `u.TurnOnScreenReader undefined`.

- [ ] **Step 4: Implement**

Create `internal/users/screenreader.go`:

```go
package users

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// ScreenReaderCombatLine tells a player whose screen reader setting just
// turned on that combat text was set to light, and how to undo it. xterm's
// screen reader mode stops announcing after 20 lines of one burst, so full
// combat text would be cut off.
const ScreenReaderCombatLine = `Combat text is set to light, so your screen reader can keep up. Type set combatverbosity full to see every blow.`

// TurnOnScreenReader is the only way an account's screen reader setting
// turns on (the creation answer, a connection's report at login, and set
// screenreader). It sets the flag and saves light combat text, firing a
// UserSettingChanged for each. It reports whether the combat level changed,
// so the caller can send ScreenReaderCombatLine. When the setting is
// already on it does nothing and reports false.
func (u *UserRecord) TurnOnScreenReader() bool {
	if u.ScreenReader {
		return false
	}
	u.ScreenReader = true
	events.AddToQueue(events.UserSettingChanged{UserId: u.UserId, Name: `screenreader`})
	if u.GetCombatVerbosity() == messaging.VerbosityLight {
		return false
	}
	u.CombatVerbosity = messaging.VerbosityLight.String()
	events.AddToQueue(events.UserSettingChanged{UserId: u.UserId, Name: `combatverbosity`})
	return true
}
```

- [ ] **Step 5: Run it to see it pass**

Run: `go test ./internal/users/ -run TestTurnOnScreenReader -count=1`
Expected: PASS.

- [ ] **Step 6: `set screenreader` turns on through it**

Replace `cmdSetScreenReader` at `internal/usercommands/set.go:216-230` with:

```go
func cmdSetScreenReader(user *users.UserRecord) (bool, error) {
	if user.ScreenReader {
		user.ScreenReader = false
		user.SendText(messaging.CategorySystem, `ScreenReader mode toggled <ansi fg="red">OFF</ansi>.`)
		events.AddToQueue(events.UserSettingChanged{
			UserId: user.UserId,
			Name:   `screenreader`,
		})
		return true, nil
	}

	combatChanged := user.TurnOnScreenReader()
	user.SendText(messaging.CategorySystem, `ScreenReader mode toggled <ansi fg="red">ON</ansi>.`)
	if combatChanged {
		user.SendText(messaging.CategorySystem, users.ScreenReaderCombatLine)
	}
	return true, nil
}
```

- [ ] **Step 7: Build and run the touched packages**

Run: `go build ./... && go test ./internal/users/ ./internal/usercommands/ ./internal/connections/ -count=1`
Expected: build succeeds, all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/connections/clientsettings.go internal/users/screenreader.go internal/users/screenreader_test.go internal/usercommands/set.go
git commit -F - <<'EOF'
feat(users): TurnOnScreenReader saves light combat text (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `Client.Display` carries `screenreader`

**Files:**
- Modify: `modules/gmcp/gmcp.ClientDisplay.go` (handler and a new decoder)
- Test: `modules/gmcp/gmcp.ClientDisplay_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `modules/gmcp/gmcp.ClientDisplay_test.go`:

```go
func storedScreenReader(id uint64) bool {
	return connections.GetClientSettings(id).ScreenReader
}

// The web client reports screenreader with its width, or alone.
func TestClientDisplay_StoresScreenReader(t *testing.T) {
	id := clientDisplayConn(t, 80)
	reportCols(id, `{"cols":42,"screenreader":true}`)
	require.True(t, storedScreenReader(id))
	require.Equal(t, uint32(42), storedWidth(id))

	reportCols(id, `{"screenreader":false}`)
	require.False(t, storedScreenReader(id))
	require.Equal(t, uint32(42), storedWidth(id), "a report without cols keeps the width")
}

// A report without the field, or with the wrong type, keeps the stored value.
func TestClientDisplay_ScreenReaderMissingOrBadKeeps(t *testing.T) {
	id := clientDisplayConn(t, 80)
	reportCols(id, `{"screenreader":true}`)
	for _, payload := range []string{`{"cols":50}`, `{"screenreader":"yes"}`, `{"screenreader":null}`, `not json`} {
		reportCols(id, payload)
		require.True(t, storedScreenReader(id), payload)
	}
}

// Only a websocket connection is listened to.
func TestClientDisplay_ScreenReaderIgnoredOnTelnet(t *testing.T) {
	mudlog.SetupLogger(nil, ``, ``, false)
	const id uint64 = 97204
	connections.RegisterTestConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	reportCols(id, `{"screenreader":true}`)
	require.False(t, storedScreenReader(id))
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./modules/gmcp/ -run TestClientDisplay -count=1`
Expected: FAIL on the three new tests (screen reader never stored).

- [ ] **Step 3: Implement**

In `modules/gmcp/gmcp.ClientDisplay.go`, replace the body of `clientDisplayHandler` after the websocket check, so it reads:

```go
	if !connections.IsWebsocket(evt.ConnectionId) {
		return events.Continue
	}
	sr, hasSR := reportedScreenReader(evt.Payload)
	if hasSR {
		storeScreenReader(evt.ConnectionId, sr)
	}
	cols, ok := reportedCols(evt.Payload)
	if !ok {
		if !hasSR {
			mudlog.Debug("gmcp.ClientDisplay", "connectionId", evt.ConnectionId, "ignored", "payload is not a whole column count")
		}
		return events.Continue
	}
	users.StoreReportedCols(evt.ConnectionId, cols)
	return events.Continue
```

Update the handler's doc comment to add: "It also stores the screenreader flag the client sends (the player typed screen reader on or off, or the device remembers it)."

Add at the end of the file:

```go
// reportedScreenReader reads {"screenreader": bool}. A missing key, a null,
// any other type and malformed JSON report false for ok, and the stored
// value is kept.
func reportedScreenReader(payload []byte) (on bool, ok bool) {
	var decoded struct {
		ScreenReader *bool `json:"screenreader"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.ScreenReader == nil {
		return false, false
	}
	return *decoded.ScreenReader, true
}

// storeScreenReader writes a web client's screen reader report on its
// connection, only when it differs. The report belongs to the connection;
// login decides what it means for the account (internal/inputhandlers).
func storeScreenReader(id connections.ConnectionId, on bool) {
	cs := connections.GetClientSettings(id)
	if cs.ScreenReader != on {
		cs.ScreenReader = on
		connections.OverwriteClientSettings(id, cs)
	}
}
```

Note: the two decoders are independent. `reportedScreenReader`'s struct has only `ScreenReader`, so a bad `cols` (say `"42"`) does not stop the flag being read, and `reportedCols` is unchanged, so every existing width test still holds.

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./modules/gmcp/ -run TestClientDisplay -count=1`
Expected: PASS, the old tests included.

- [ ] **Step 5: Commit**

```bash
git add modules/gmcp/gmcp.ClientDisplay.go modules/gmcp/gmcp.ClientDisplay_test.go
git commit -F - <<'EOF'
feat(gmcp): Client.Display reports the web client's screen reader switch (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `Char.Info` tells the client the account's setting

**Files:**
- Modify: `modules/gmcp/gmcp.Char.go` (`GMCPCharModule_Payload_Info` at `:942-947`, `GetCharNode` at `:492-502`, `init` listeners at `:110-140`)
- Create: `modules/gmcp/gmcp.CharInfo_screenreader_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.CharInfo_screenreader_test.go`:

```go
package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// false must reach the client too, so another device turns its mode off.
func TestCharInfo_ScreenReaderAlwaysSent(t *testing.T) {
	b, err := json.Marshal(GMCPCharModule_Payload_Info{})
	require.NoError(t, err)
	require.Contains(t, string(b), `"screenreader":false`)

	b, err = json.Marshal(GMCPCharModule_Payload_Info{ScreenReader: true})
	require.NoError(t, err)
	require.Contains(t, string(b), `"screenreader":true`)
}

// set screenreader re-pushes Char.Info; other settings do not.
func TestCharInfo_PushedWhenScreenReaderChanges(t *testing.T) {
	events.DrainQueuedByTypeForTest(`GMCPCharUpdate`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPCharUpdate`) })
	g := &GMCPCharModule{}

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 5, Name: `tips`})
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPCharUpdate`))

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 5, Name: `ScreenReader`})
	queued := events.DrainQueuedByTypeForTest(`GMCPCharUpdate`)
	require.Len(t, queued, 1)
	require.Equal(t, GMCPCharUpdate{UserId: 5, Identifier: `Char.Info`}, queued[0])

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 0, Name: `screenreader`})
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPCharUpdate`))
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./modules/gmcp/ -run TestCharInfo -count=1`
Expected: FAIL to compile (`ScreenReader` field and `userSettingChangedHandler` undefined).

- [ ] **Step 3: Implement**

Add the field to `GMCPCharModule_Payload_Info` (`gmcp.Char.go:942`):

```go
	// ScreenReader is the account's screen reader setting. Always sent,
	// false included, so the web client on every device follows it.
	ScreenReader bool `json:"screenreader"`
```

In `GetCharNode` (`:493`), add `ScreenReader: user.ScreenReader,` to the `GMCPCharModule_Payload_Info` literal.

In `init`, beside the other listeners (after `events.RegisterListener(events.SightBandChanged{}, g.sightBandChangedHandler)`):

```go
	// Char.Info carries the screen reader setting; re-push it when the
	// setting changes, so the web client switches its mode to match.
	events.RegisterListener(events.UserSettingChanged{}, g.userSettingChangedHandler)
```

Add the handler beside `sightBandChangedHandler`:

```go
// userSettingChangedHandler pushes Char.Info when a player's screen reader
// setting changes (set screenreader, or turned on at login or creation).
func (g *GMCPCharModule) userSettingChangedHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.UserSettingChanged)
	if !typeOk || evt.UserId == 0 || !strings.EqualFold(evt.Name, `screenreader`) {
		return events.Continue
	}
	events.AddToQueue(GMCPCharUpdate{
		UserId:     evt.UserId,
		Identifier: `Char.Info`,
	})
	return events.Continue
}
```

Add `"strings"` to the imports if `gmcp.Char.go` does not already import it.

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./modules/gmcp/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add modules/gmcp/gmcp.Char.go modules/gmcp/gmcp.CharInfo_screenreader_test.go
git commit -F - <<'EOF'
feat(gmcp): Char.Info carries the screen reader setting (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Login and creation

**Files:**
- Modify: `internal/inputhandlers/login_prompt_handler.go` (`advanceAndSendPrompt` at `:462-491`; new constants and `writeConnFacts`)
- Modify: `internal/inputhandlers/login.go` (`GetLoginPromptHandler` at `:229`; creation at `:158-160`; existing login after `users.LoginUser` at `:103`)
- Create: `internal/inputhandlers/login_screenreader_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/inputhandlers/login_screenreader_test.go`:

```go
package inputhandlers

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// No step may use a reserved connection-fact key as its ID: the handler
// overwrites those keys before every condition.
func TestLoginSteps_NoStepUsesAConnFactID(t *testing.T) {
	for _, step := range loginSteps() {
		require.False(t, strings.HasPrefix(step.ID, connFactPrefix), step.ID)
	}
}

func screenReaderStep(t *testing.T) *PromptStep {
	t.Helper()
	for _, step := range loginSteps() {
		if step.ID == `screen-reader-new` {
			return step
		}
	}
	t.Fatal("no screen-reader-new step")
	return nil
}

// Asked only of a new telnet player whose client sent no MTTS.
func TestScreenReaderQuestion_AskedOnlyWithoutAnAnswer(t *testing.T) {
	cond := screenReaderStep(t).Condition
	base := func(ws, mtts string) map[string]string {
		return map[string]string{`username`: `new`, connFactWebsocket: ws, connFactMTTS: mtts, connFactScreenReader: `n`}
	}
	require.True(t, cond(base(`n`, `n`)), "telnet without MTTS is asked")
	require.False(t, cond(base(`y`, `n`)), "web is never asked")
	require.False(t, cond(base(`n`, `y`)), "any MTTS reply answers it")
	login := base(`n`, `n`)
	login[`username`] = `bob`
	require.False(t, cond(login), "a login is never asked")
}

// The answer when asked, else what the connection reported.
func TestNewUserScreenReader(t *testing.T) {
	require.True(t, newUserScreenReader(map[string]string{`screen-reader-new`: `y`}, false))
	require.False(t, newUserScreenReader(map[string]string{`screen-reader-new`: `n`}, false))
	require.True(t, newUserScreenReader(map[string]string{}, true))
	require.False(t, newUserScreenReader(map[string]string{}, false))
}

// The connection's facts are written as y or n under the reserved keys.
func TestWriteConnFacts(t *testing.T) {
	const web, tel connections.ConnectionId = 97401, 97402
	connections.RegisterTestWebsocketConnection(web)
	connections.RegisterTestConnection(tel)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(web)
		connections.UnregisterTestConnection(tel)
	})
	cs := connections.GetClientSettings(web)
	cs.ScreenReader = true
	connections.OverwriteClientSettings(web, cs)
	cs = connections.GetClientSettings(tel)
	cs.MTTSReported = true
	connections.OverwriteClientSettings(tel, cs)

	r := map[string]string{}
	writeConnFacts(r, web)
	require.Equal(t, map[string]string{connFactWebsocket: `y`, connFactMTTS: `n`, connFactScreenReader: `y`}, r)
	r = map[string]string{}
	writeConnFacts(r, tel)
	require.Equal(t, map[string]string{connFactWebsocket: `n`, connFactMTTS: `y`, connFactScreenReader: `n`}, r)
}

// A connection's report turns an account's setting on, never off.
func TestApplyReportedScreenReader(t *testing.T) {
	events.DrainQueuedByTypeForTest(`UserSettingChanged`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`UserSettingChanged`) })

	off := &users.UserRecord{UserId: 8}
	turnedOn, combat := applyReportedScreenReader(off, true)
	require.True(t, turnedOn)
	require.True(t, combat)
	require.True(t, off.ScreenReader)

	on := &users.UserRecord{UserId: 9, ScreenReader: true}
	turnedOn, _ = applyReportedScreenReader(on, false)
	require.False(t, turnedOn)
	require.True(t, on.ScreenReader, "a connection never turns the setting off")

	quiet := &users.UserRecord{UserId: 10}
	turnedOn, _ = applyReportedScreenReader(quiet, false)
	require.False(t, turnedOn)
	require.False(t, quiet.ScreenReader)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/inputhandlers/ -run "TestLoginSteps_NoStepUsesAConnFactID|TestScreenReaderQuestion|TestNewUserScreenReader|TestWriteConnFacts|TestApplyReportedScreenReader" -count=1`
Expected: FAIL to compile (`loginSteps`, `connFactPrefix` and the helpers are undefined).

- [ ] **Step 3: Connection facts in the prompt handler**

In `internal/inputhandlers/login_prompt_handler.go`, after the `ConditionFunc` type, add:

```go
// Reserved result keys. A ConditionFunc sees only the results map, so the
// prompt handler writes the connection's facts there, as "y" or "n", before
// it checks each step. No step may use an ID starting connFactPrefix
// (TestLoginSteps_NoStepUsesAConnFactID).
const (
	connFactPrefix       = `_conn_`
	connFactWebsocket    = `_conn_websocket`
	connFactMTTS         = `_conn_mtts`
	connFactScreenReader = `_conn_screenreader`
)

// writeConnFacts stores what the connection's client has said about itself
// in results under the reserved keys.
func writeConnFacts(results map[string]string, id connections.ConnectionId) {
	cs := connections.GetClientSettings(id)
	results[connFactWebsocket] = yesNo(connections.IsWebsocket(id))
	results[connFactMTTS] = yesNo(cs.MTTSReported)
	results[connFactScreenReader] = yesNo(cs.ScreenReader)
}

// yesNo is "y" for true and "n" for false, the form prompt results use.
func yesNo(b bool) string {
	if b {
		return `y`
	}
	return `n`
}
```

(If `go build` reports `yesNo` already declared in the package, use the existing one and delete this copy.)

In `advanceAndSendPrompt`, as the first statement inside the `for` loop (before `step := state.Steps[...]`), add:

```go
		writeConnFacts(state.Results, clientInput.ConnectionId)
```

- [ ] **Step 4: `loginSteps()`, the creation condition and answer**

In `internal/inputhandlers/login.go`, turn the inline `loginSteps := []*PromptStep{ ... }` in `GetLoginPromptHandler` into a function, keeping every step as it is:

```go
// loginSteps are the login and creation prompts in order.
func loginSteps() []*PromptStep {
	return []*PromptStep{
		// ... the existing steps, unchanged except screen-reader-new below ...
	}
}

func GetLoginPromptHandler() connections.InputHandler {
	// Create and return the handler using the generic factory function
	return CreatePromptHandler(loginSteps(), FinalizeLoginOrCreate)
}
```

Keep `GetLoginPromptHandler`'s existing doc comment if it has one.

Change the `screen-reader-new` step's `Condition` to:

```go
			// Asked only of a new player on a telnet client that did not answer
			// MTTS: a web client's answer is "screen reader on", and any MTTS
			// reply is an answer (spec 2026-11-17, ruling 4).
			Condition: func(results map[string]string) bool {
				return results["username"] == `new` && askScreenReader(results)
			},
```

Add near `FinalizeLoginOrCreate`:

```go
// askScreenReader reports whether the creation question is asked: never on
// a web client, and never when a telnet client answered MTTS.
func askScreenReader(results map[string]string) bool {
	return results[connFactWebsocket] != `y` && results[connFactMTTS] != `y`
}

// newUserScreenReader is a new account's screen reader setting: the answer
// when the question was asked, else what the connection reported.
func newUserScreenReader(results map[string]string, reported bool) bool {
	if answer, asked := results["screen-reader-new"]; asked {
		return answer == `y`
	}
	return reported
}

// applyReportedScreenReader turns u's setting on when its connection
// reported a screen reader. A connection never turns it off. It reports
// whether the setting turned on and whether combat text changed with it.
func applyReportedScreenReader(u *users.UserRecord, reported bool) (turnedOn bool, combatChanged bool) {
	if !reported || u.ScreenReader {
		return false, false
	}
	return true, u.TurnOnScreenReader()
}

// screenReaderOnNotice tells a player their connection turned the account's
// screen reader setting on, and how to turn it off.
const screenReaderOnNotice = `Screen reader mode is on for this character. Type set screenreader to turn it off.`
```

- [ ] **Step 5: Use them in `FinalizeLoginOrCreate`**

Creation, replace `newUser.ScreenReader = results["screen-reader-new"] == `y`` (`login.go:160`) with:

```go
		creationCombatChanged := false
		if newUserScreenReader(results, connections.GetClientSettings(clientInput.ConnectionId).ScreenReader) {
			creationCombatChanged = newUser.TurnOnScreenReader()
		}
```

After `sharedState["UserObject"] = newUser // For main loop` in the creation branch, add:

```go
		if creationCombatChanged {
			sendLoginText(clientInput.ConnectionId, users.ScreenReaderCombatLine)
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
		}
```

Existing login, after `sharedState["UserObject"] = loggedInUser // For main loop`, add:

```go
			if turnedOn, combatChanged := applyReportedScreenReader(loggedInUser, connections.GetClientSettings(clientInput.ConnectionId).ScreenReader); turnedOn {
				sendLoginText(clientInput.ConnectionId, screenReaderOnNotice)
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
				if combatChanged {
					sendLoginText(clientInput.ConnectionId, users.ScreenReaderCombatLine)
					connections.SendTo(term.CRLF, clientInput.ConnectionId)
				}
			}
```

- [ ] **Step 6: Run the tests to see them pass**

Run: `go test ./internal/inputhandlers/ -count=1`
Expected: PASS, every existing login test included.

- [ ] **Step 7: Commit**

```bash
git add internal/inputhandlers/login_prompt_handler.go internal/inputhandlers/login.go internal/inputhandlers/login_screenreader_test.go
git commit -F - <<'EOF'
feat(login): web and MTTS answer the screen reader question (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Telnet MTTS

**Files:**
- Create: `internal/term/mtts.go`, `internal/term/mtts_test.go`
- Modify: `internal/inputhandlers/term_iac.go` (inside the `for _, iacCmd := range iacCmds` loop, before the NAWS check at `:141`)
- Modify: `internal/inputhandlers/term_iac_test.go`
- Modify: `main.go:724-727` (after the NAWS request)

- [ ] **Step 1: Write the failing `term` test**

Create `internal/term/mtts_test.go`:

```go
package term

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMTTS(t *testing.T) {
	for _, tt := range []struct {
		reply string
		bits  int
		ok    bool
	}{
		{`MTTS 137`, 137, true},
		{`MTTS 64`, 64, true},
		{`MTTS 0`, 0, true},
		{`MTTS -1`, 0, false},
		{`MTTS x`, 0, false},
		{`MUDLET`, 0, false},
		{`XTERM-256COLOR`, 0, false},
		{``, 0, false},
	} {
		bits, ok := ParseMTTS(tt.reply)
		require.Equal(t, tt.ok, ok, tt.reply)
		require.Equal(t, tt.bits, bits, tt.reply)
	}
}

func TestTermTypeIsMatchesAReply(t *testing.T) {
	in := append([]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_IS}, []byte(`MTTS 73`)...)
	in = append(in, TELNET_IAC, TELNET_SE)
	ok, payload := Matches(in, TelnetTermTypeIs)
	require.True(t, ok)
	require.Equal(t, `MTTS 73`, string(payload))
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/term/ -run "TestParseMTTS|TestTermTypeIs" -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `internal/term/mtts.go`**

```go
package term

import (
	"strconv"
	"strings"
)

// Terminal type subnegotiation (RFC 1091) and MTTS, the MUD Terminal Type
// Standard (https://tintin.mudhalla.net/protocols/mtts/). A client cycles
// its replies: its name, its terminal type, then "MTTS <bits>", then it
// repeats the last one.
const (
	// TELNET_TTYPE_IS starts a client's terminal type reply.
	TELNET_TTYPE_IS IACByte = 0
	// TELNET_TTYPE_SEND asks the client for its next terminal type.
	TELNET_TTYPE_SEND IACByte = 1
	// MTTSScreenReader is the MTTS bit a client sets when its player uses a
	// screen reader.
	MTTSScreenReader = 64
)

var (
	// TelnetWillTermType is the client agreeing to report its terminal type.
	TelnetWillTermType = TerminalCommand{[]byte{TELNET_IAC, TELNET_WILL, TELNET_OPT_TERM_TYPE}, []byte{}}
	// TelnetWontTermType is the client declining to report it.
	TelnetWontTermType = TerminalCommand{[]byte{TELNET_IAC, TELNET_WONT, TELNET_OPT_TERM_TYPE}, []byte{}}
	// TelnetTermTypeSend asks for the next terminal type in the cycle.
	TelnetTermTypeSend = TerminalCommand{[]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_SEND, TELNET_IAC, TELNET_SE}, []byte{}}
	// TelnetTermTypeIs is a reply: IAC SB TTYPE IS <text> IAC SE.
	TelnetTermTypeIs = TerminalCommand{[]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_IS}, []byte{TELNET_IAC, TELNET_SE}}
)

// ParseMTTS reads an MTTS reply ("MTTS 137") and returns its bit field.
// Any other reply, or a negative or non-numeric value, reports false.
func ParseMTTS(reply string) (int, bool) {
	rest, ok := strings.CutPrefix(reply, `MTTS `)
	if !ok {
		return 0, false
	}
	bits, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil || bits < 0 {
		return 0, false
	}
	return bits, true
}
```

- [ ] **Step 4: Run the `term` tests to see them pass**

Run: `go test ./internal/term/ -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing cycle tests**

Append to `internal/inputhandlers/term_iac_test.go` (add `"github.com/GoMudEngine/GoMud/internal/mudlog"` and `"github.com/stretchr/testify/require"` to its imports):

```go
func termTypeConn(t *testing.T) connections.ConnectionId {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false)
	const id connections.ConnectionId = 97501
	connections.RegisterTestConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	return id
}

// The usual cycle: name, terminal, MTTS. MTTS ends it and sets the bits.
func TestRecordTermType_MTTSEndsTheCycle(t *testing.T) {
	id := termTypeConn(t)
	require.True(t, recordTermType(id, `MUDLET`))
	require.True(t, recordTermType(id, `ANSI-TRUECOLOR`))
	require.False(t, recordTermType(id, `MTTS 2441`)) // 2441 = 2048+256+128+8+1, bit 64 clear
	cs := connections.GetClientSettings(id)
	require.True(t, cs.MTTSReported)
	require.False(t, cs.ScreenReader)
}

func TestRecordTermType_ScreenReaderBit(t *testing.T) {
	id := termTypeConn(t)
	require.False(t, recordTermType(id, `MTTS 73`)) // 64+8+1
	cs := connections.GetClientSettings(id)
	require.True(t, cs.MTTSReported)
	require.True(t, cs.ScreenReader)
}

// A client without MTTS repeats itself; the repeat ends the cycle, and the
// question will be asked.
func TestRecordTermType_RepeatEndsWithoutMTTS(t *testing.T) {
	id := termTypeConn(t)
	require.True(t, recordTermType(id, `XTERM`))
	require.False(t, recordTermType(id, `XTERM`))
	require.False(t, connections.GetClientSettings(id).MTTSReported)
}

// A client that never repeats is asked at most maxTermTypeReplies times.
func TestRecordTermType_Bounded(t *testing.T) {
	id := termTypeConn(t)
	asks := 0
	for i := range 10 {
		if recordTermType(id, fmt.Sprintf(`T%d`, i)) {
			asks++
		}
	}
	require.Equal(t, maxTermTypeReplies-1, asks)
}

// An over-long reply is kept short.
func TestRecordTermType_LongReplyTrimmed(t *testing.T) {
	id := termTypeConn(t)
	recordTermType(id, strings.Repeat(`A`, 500))
	require.Len(t, connections.GetClientSettings(id).TermTypeLast, maxTermTypeLen)
}

// Through the IAC handler: an MTTS reply is read, and nothing is sent back.
func TestTelnetIACHandler_MTTSReply(t *testing.T) {
	id := termTypeConn(t)
	in := append([]byte{term.TELNET_IAC, term.TELNET_SB, term.TELNET_OPT_TERM_TYPE, term.TELNET_TTYPE_IS}, []byte(`MTTS 64`)...)
	in = append(in, term.TELNET_IAC, term.TELNET_SE)
	TelnetIACHandler(&connections.ClientInput{ConnectionId: id, DataIn: in}, map[string]any{})
	require.True(t, connections.GetClientSettings(id).ScreenReader)
}
```

Also add `"fmt"` and `"strings"` to the test imports.

- [ ] **Step 6: Run them to see them fail**

Run: `go test ./internal/inputhandlers/ -run "TestRecordTermType|TestTelnetIACHandler_MTTSReply" -count=1`
Expected: FAIL to compile (`recordTermType` undefined).

- [ ] **Step 7: Implement the cycle in `term_iac.go`**

Add to `internal/inputhandlers/term_iac.go`:

```go
// maxTermTypeReplies bounds the MTTS cycle: name, terminal type, MTTS, and
// one spare for a client that repeats late.
const maxTermTypeReplies = 4

// maxTermTypeLen is the most of a terminal type reply kept for comparing
// with the next one; a reply comes from the client.
const maxTermTypeLen = 64

// recordTermType handles one terminal type reply and reports whether to ask
// for the next. An MTTS reply records whether the player uses a screen
// reader and ends the cycle. A reply equal to the last one ends it (the
// client has cycled back without MTTS). Otherwise the server asks again, at
// most maxTermTypeReplies times in all.
func recordTermType(id connections.ConnectionId, reply string) bool {
	if len(reply) > maxTermTypeLen {
		reply = reply[:maxTermTypeLen]
	}
	cs := connections.GetClientSettings(id)
	cs.TermTypeReplies++
	last := cs.TermTypeLast
	cs.TermTypeLast = reply

	askAgain := false
	if bits, ok := term.ParseMTTS(reply); ok {
		cs.MTTSReported = true
		cs.ScreenReader = bits&term.MTTSScreenReader != 0
		mudlog.Debug("Received", "type", "IAC (MTTS)", "bits", bits)
	} else {
		mudlog.Debug("Received", "type", "IAC (TTYPE)", "reply", reply)
		askAgain = reply != last && cs.TermTypeReplies < maxTermTypeReplies
	}
	connections.OverwriteClientSettings(id, cs)
	return askAgain
}
```

Inside `TelnetIACHandler`'s loop, before `// Is it a screen size report?`, add:

```go
		// Terminal type and MTTS: the client may declare a screen reader
		// before login (connect sends IAC DO TTYPE, main.go).
		if ok, _ := term.Matches(iacCmd, term.TelnetWillTermType); ok {
			connections.SendTo(term.TelnetTermTypeSend.BytesWithPayload(nil), clientInput.ConnectionId)
			continue
		}
		if ok, _ := term.Matches(iacCmd, term.TelnetWontTermType); ok {
			continue
		}
		if ok, payload := term.Matches(iacCmd, term.TelnetTermTypeIs); ok {
			if recordTermType(clientInput.ConnectionId, string(payload)) {
				connections.SendTo(term.TelnetTermTypeSend.BytesWithPayload(nil), clientInput.ConnectionId)
			}
			continue
		}
```

`term_iac.go` already imports `mudlog` (`term_iac.go:6`).

- [ ] **Step 8: Ask on connect**

In `main.go`, after the `term.TelnetDO(term.TELNET_OPT_NAWS)` send (`:724-727`), add:

```go
	// Ask for the terminal type, so a client can declare a screen reader
	// through MTTS before login (internal/inputhandlers/term_iac.go).
	connections.SendTo(
		term.TelnetDO(term.TELNET_OPT_TERM_TYPE),
		connDetails.ConnectionId(),
	)
```

- [ ] **Step 9: Run the tests and build**

Run: `go build ./... && go test ./internal/term/ ./internal/inputhandlers/ -count=1`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/term/mtts.go internal/term/mtts_test.go internal/inputhandlers/term_iac.go internal/inputhandlers/term_iac_test.go main.go
git commit -F - <<'EOF'
feat(telnet): read MTTS so a client can declare a screen reader (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: The client module `screenreader.js`

**Files:**
- Create: `_datafiles/html/public/static/js/screenreader.js`
- Create: `tools/jstest/screenreader.test.js`

- [ ] **Step 1: Write the failing test**

Create `tools/jstest/screenreader.test.js`:

```js
// screenreader.test.js: the web client's screen reader switch (touch arc
// Task 0, docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md).
//
// Run:  node tools/jstest/screenreader.test.js
'use strict';

var path = require('path');
var SR = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'screenreader.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// ---- matchPhrase -----------------------------------------------------------

check('on', SR.matchPhrase('screen reader on', false) === 'on', '');
check('off', SR.matchPhrase('screen reader off', false) === 'off', '');
check('case and spaces', SR.matchPhrase('  Screen   READER  On ', false) === 'on', '');
check('password box never matches', SR.matchPhrase('screen reader on', true) === null, '');
check('other text', SR.matchPhrase('screenreader on', false) === null, '');
check('a command', SR.matchPhrase('look', false) === null, '');
check('not a string', SR.matchPhrase(undefined, false) === null, '');

// ---- create ----------------------------------------------------------------

function fakes(saved) {
    var f = {
        mode: null, label: null, hint: null, lines: [], reports: [], setCommands: 0,
        store: saved === undefined ? {} : { screenReader: saved }, inGame: false
    };
    f.opts = {
        term: { setOption: function (k, v) { if (k === 'screenReaderMode') f.mode = v; } },
        storage: {
            getItem: function (k) { return Object.prototype.hasOwnProperty.call(f.store, k) ? f.store[k] : null; },
            setItem: function (k, v) { f.store[k] = v; }
        },
        input: { setAttribute: function (k, v) { if (k === 'aria-label') f.label = v; } },
        hint: {},
        report: function (on) { f.reports.push(on); return true; },
        setCommand: function () { f.setCommands++; return true; },
        writeLine: function (t) { f.lines.push(t); },
        inGame: function () { return f.inGame; }
    };
    Object.defineProperty(f.opts.hint, 'textContent', { set: function (v) { f.hint = v; } });
    return f;
}

var f = fakes();
var sr = SR.create(f.opts);
sr.load();
check('fresh device: off', f.mode === false && !sr.isOn(), 'mode ' + f.mode);
check('fresh device: label names the phrase', f.label === SR.LABEL_OFF, f.label);
check('fresh device: hint names the phrase', f.hint === SR.HINT_OFF, f.hint);
check('fresh device: nothing reported or stored', f.reports.length === 0 && !('screenReader' in f.store), JSON.stringify(f));

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
check('remembered: on at load', f.mode === true && sr.isOn(), 'mode ' + f.mode);
check('remembered: label shortened', f.label === SR.LABEL_ON, f.label);

f = fakes();
f.opts.storage = { getItem: function () { throw new Error('blocked'); }, setItem: function () { throw new Error('blocked'); } };
sr = SR.create(f.opts);
sr.load();
check('blocked storage: off, no throw', f.mode === false, '');
sr.phrase('on');
check('blocked storage: still turns on', f.mode === true, '');

// Before login: turn on, remember, report, confirm in the game window.
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.phrase('on');
check('phrase on: mode on', f.mode === true, '');
check('phrase on: remembered', f.store.screenReader === 'true', JSON.stringify(f.store));
check('phrase on: reported', f.reports.join() === 'true', f.reports.join());
check('phrase on: confirmed', f.lines.join('|') === SR.SAID_ON, f.lines.join('|'));
check('phrase on: no set command before login', f.setCommands === 0, '');
sr.phrase('off');
check('phrase off: mode off and remembered', f.mode === false && f.store.screenReader === 'false', '');
check('phrase off: confirmed', f.lines[1] === SR.SAID_OFF, f.lines.join('|'));

// In game, account differs: send set screenreader, let the server's line speak.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
check('in game, account off: set command sent', f.setCommands === 1, '');
check('in game, account off: no local line', f.lines.length === 0, f.lines.join('|'));

// In game, account already matches: no set command, local line.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(true);
f.lines = [];
sr.phrase('on');
check('in game, account on: no set command', f.setCommands === 0, '');
check('in game, account on: local line', f.lines.join('|') === SR.SAID_ON, f.lines.join('|'));

// The account's setting arrives: the mode follows it and is remembered.
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.account(true);
check('account on: mode on, remembered', f.mode === true && f.store.screenReader === 'true', '');
sr.account(false);
check('account off: mode off, remembered', f.mode === false && f.store.screenReader === 'false', '');

// Logged out: the account is forgotten, so the phrase confirms locally again.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.loggedOut();
f.inGame = false;
sr.phrase('on');
check('after logout: no set command', f.setCommands === 0, '');
check('after logout: local line', f.lines.join('|') === SR.SAID_ON, f.lines.join('|'));

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node tools/jstest/screenreader.test.js`
Expected: FAIL, `Cannot find module ... screenreader.js`.

- [ ] **Step 3: Implement**

Create `_datafiles/html/public/static/js/screenreader.js`:

```js
// screenreader.js: the web client's screen reader switch (touch arc Task 0,
// docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md).
//
// A blind player hears nothing of the game until xterm's screenReaderMode
// is on, and a browser cannot tell that a screen reader is running. So the
// command box's spoken label, and a hidden line at the top of the page, tell
// screen reader users to type "screen reader on". The page passes that
// phrase here instead of sending it. The choice is remembered on this
// device, reported to the server (Client.Display), and follows the account's
// setting (Char.Info.screenreader) once a character is logged in. Sighted
// players never see any of it, and xterm's mode stays off for them.
//
// tools/jstest/screenreader.test.js is the regression guard.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;          // node, for the tests
    } else {
        root.ScreenReader = api;       // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var STORAGE_KEY = 'screenReader';
    var LABEL_OFF = 'Game commands. Screen reader users: type screen reader on, then press Enter.';
    var LABEL_ON = 'Game commands.';
    var HINT_OFF = 'Screen reader users: type screen reader on, then press Enter.';
    var HINT_ON = 'Screen reader mode is on. Type screen reader off to turn it off.';
    var SAID_ON = 'Screen reader mode is on.';
    var SAID_OFF = 'Screen reader mode is off.';

    // matchPhrase returns 'on', 'off' or null for a typed line. Case and
    // spacing do not matter. A password box never matches, so a password is
    // never taken for the phrase.
    function matchPhrase(line, isPassword) {
        if (isPassword || typeof line !== 'string') return null;
        var s = line.trim().toLowerCase().replace(/\s+/g, ' ');
        if (s === 'screen reader on') return 'on';
        if (s === 'screen reader off') return 'off';
        return null;
    }

    // opts: term (xterm, for setOption), storage (localStorage; any call may
    // throw), input (the command box), hint (the hidden paragraph),
    // report(on) (sends Client.Display), setCommand() (sends "set
    // screenreader"), writeLine(text) (writes into the game window),
    // inGame() (true while a character is logged in).
    function create(opts) {
        var on = false;
        var account = null;    // the account's setting, once Char.Info names it

        function remember(value) {
            try { opts.storage.setItem(STORAGE_KEY, JSON.stringify(value)); } catch (e) { /* storage blocked: this visit only */ }
        }

        function show(value) {
            on = value;
            opts.term.setOption('screenReaderMode', value);
            opts.input.setAttribute('aria-label', value ? LABEL_ON : LABEL_OFF);
            opts.hint.textContent = value ? HINT_ON : HINT_OFF;
        }

        return {
            isOn: function () { return on; },

            // Page load: put the labels in place, and turn the mode on when
            // this device remembers it, before the connection opens.
            load: function () {
                var saved = false;
                try { saved = JSON.parse(opts.storage.getItem(STORAGE_KEY) || 'false') === true; } catch (e) { saved = false; }
                show(saved);
            },

            // The player typed the phrase: which is 'on' or 'off'.
            phrase: function (which) {
                var value = which === 'on';
                show(value);
                remember(value);
                opts.report(value);
                if (opts.inGame() && account !== null && account !== value) {
                    // The server's own "ScreenReader mode toggled" line confirms it.
                    opts.setCommand();
                    return;
                }
                opts.writeLine(value ? SAID_ON : SAID_OFF);
            },

            // Char.Info named the account's setting.
            account: function (value) {
                account = value;
                if (value !== on) {
                    show(value);
                    remember(value);
                }
            },

            // The connection closed: the next login names the account again.
            loggedOut: function () { account = null; }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        create: create
    };
}));
```

- [ ] **Step 4: Run it to see it pass**

Run: `node --check _datafiles/html/public/static/js/screenreader.js && node tools/jstest/screenreader.test.js`
Expected: every line `ok`, then `all passed`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/screenreader.js tools/jstest/screenreader.test.js
git commit -F - <<'EOF'
feat(webclient): screen reader switch module (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Wire the page

**Files:**
- Modify: `_datafiles/html/public/webclient-pure.html` (script tags `:234`, `<body>` `:244`, `#command-input` `:310`, `displayReport` `:540-544`, `Char` handler `:895-910`, socket `onopen` `:3072-3075`, Enter handler `:3342`)

- [ ] **Step 1: Load the module**

After the `displayreport.js` script tag (`:234`), add:

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/screenreader.js?v={{ .ASSET_VERSION }}"></script>
```

- [ ] **Step 2: The hidden first line**

Immediately after `<body onLoad="init(); return false;">` (`:244`), add:

```html
    <!-- Read first by a screen reader, seen by no one (touch arc Task 0;
         static/js/screenreader.js keeps it current). iOS VoiceOver starts at
         the top of the page rather than at the focused command box. -->
    <p id="sr-hint" class="visually-hidden">Screen reader users: type screen reader on, then press Enter.</p>
```

- [ ] **Step 3: The spoken label**

On `#command-input` (`:310`), add `aria-label="Game commands. Screen reader users: type screen reader on, then press Enter."` beside the existing attributes. Change nothing else on the element.

- [ ] **Step 4: Create the switch, and report it with the width**

Replace the `displayReport` creation (`:540-544`) with:

```js
        // Tell the server the game window's columns (static/js/displayreport.js),
        // and whether this player asked for screen reader mode. SendGMCP is
        // declared further down this script, so it is hoisted.
        var displayReport = window.DisplayReport.create({
            send: function (cols) { return SendGMCP('Client.Display', { cols: cols, screenreader: screenReader.isOn() }); }
        });
        // The screen reader switch (static/js/screenreader.js): off unless the
        // player typed "screen reader on" here before, or their account has it.
        var screenReader = window.ScreenReader.create({
            term: term,
            storage: (function () { try { return window.localStorage; } catch (e) { return null; } })(),
            input: document.getElementById('command-input'),
            hint: document.getElementById('sr-hint'),
            report: function (on) { return SendGMCP('Client.Display', { screenreader: on }); },
            setCommand: function () { return SendData('set screenreader'); },
            writeLine: function (text) { term.writeln(text); },
            inGame: function () { return inGame; }
        });
        screenReader.load();
```

(`screenReader` is used inside `send` only when a report goes out, after this runs. `inGame` is declared with `let` further down; `inGame()` is only called on a typed phrase, after the page has loaded.)

- [ ] **Step 5: Follow the account**

In the `"Char"` update handler, after the `companionGlue.hello` block (`:905-907`), add:

```js
                // The account's screen reader setting (Char.Info.screenreader):
                // xterm's mode follows it on every device.
                if ( obj.Info && typeof obj.Info.screenreader === 'boolean' ) {
                    screenReader.account(obj.Info.screenreader);
                }
```

- [ ] **Step 6: Forget the account on a new connection**

In `socket.onopen`, beside `if (companionGlue) { companionGlue.disconnected(); }` (`:3074`), add:

```js
                screenReader.loggedOut();
```

- [ ] **Step 7: Catch the phrase**

In the Enter branch (`if (event.key === 'Enter') {` at `:3342`), as its first statements, before the command history push, add:

```js
                // "screen reader on" / "off" is the client's own switch
                // (static/js/screenreader.js): never sent, never in history.
                var srPhrase = window.ScreenReader.matchPhrase(event.target.value, textInput.type === 'password');
                if (srPhrase) {
                    screenReader.phrase(srPhrase);
                    event.target.value = '';
                    return;
                }
```

- [ ] **Step 8: Run the JS gates**

Run (Git Bash, repo root):

```bash
for f in $(git ls-files '*.js' | grep -v -E 'xterm|vendor|node_modules'); do node --check "$f" || echo "SYNTAX $f"; done
for t in tools/jstest/*.test.js; do node "$t" >/dev/null || echo "FAIL $t"; done
for t in tools/webclient-tests/*.js; do node "$t" >/dev/null || echo "FAIL $t"; done
```

Expected: no `SYNTAX` or `FAIL` lines. (Compare the file list with `.github/workflows/validate.yml`'s javascript job and run exactly what it runs if it differs.)

- [ ] **Step 9: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html
git commit -F - <<'EOF'
feat(webclient): spoken opt-in for screen reader mode (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Help, patch notes and package docs

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/set.template` (the `::rows` block with `set charset`)
- Modify: `docs/PATCH_NOTES.md` (new top entry)
- Modify: `modules/gmcp/context.md:500` (the `gmcp.ClientDisplay.go` row) and its `Char.Info` description
- Modify: `internal/connections/context.md:110-118` (`ClientSettings`)
- Modify: `internal/inputhandlers/context.md` (reserved `_conn_` keys, MTTS)
- Modify: `internal/users/context.md` (`TurnOnScreenReader`, `ScreenReaderCombatLine`)

- [ ] **Step 1: Help row**

In `set.template`, inside the `::rows` block that holds `set charset`, after the `set charset` entry and its continuation line, add a blank line and:

```
  <ansi fg="command">set screenreader</ansi>  Toggles screen reader mode: plainer text without art,
               and combat text set to light. In the web client, typing
               <ansi fg="command">screen reader on</ansi> also turns it on and reads the game aloud.
```

Match the continuation indent of the `set charset` entry exactly (count its spaces). Then run the help guards:

Run: `go test ./internal/guards/ ./internal/templates/ ./internal/devtools/ -count=1`
Expected: PASS. If a width or markup guard fails, rewrap the row as it says; do not change the guard.

- [ ] **Step 2: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, under the title:

```markdown
## 2026-11-17: Screen readers in the web client

- The web client can now read the game aloud to screen reader users. Your screen reader announces how as soon as the page opens: type `screen reader on` in the command box and press Enter. The web client remembers it on that device, and `screen reader off` turns it off.
- Your character's screen reader setting follows you: `set screenreader` turns it on or off wherever you play, and a device you use it on turns it on for your character.
- Turning screen reader mode on sets combat text to light, so your screen reader can keep up. `set combatverbosity full` brings every blow back.
- New characters made in the web client are no longer asked whether they use a screen reader. Telnet clients that tell the game their player uses one are not asked either.
```

- [ ] **Step 3: Package docs**

- `modules/gmcp/context.md`: in the `gmcp.ClientDisplay.go` row, add that the payload may carry `screenreader` (bool), stored on `ClientSettings.ScreenReader` for websockets only, kept when absent or not a bool, and that the client sends it with every width report and alone when the player types the phrase. Where `Char.Info` is described, add the `screenreader` field (always sent, false included) and that `UserSettingChanged{Name: "screenreader"}` re-pushes `Char.Info`.
- `internal/connections/context.md`: add `ScreenReader`, `MTTSReported`, `TermTypeReplies`, `TermTypeLast` to the `ClientSettings` block with one line each.
- `internal/inputhandlers/context.md`: add a short section: the reserved `_conn_` result keys and `writeConnFacts`; `screen-reader-new` is asked only of telnet without MTTS; `applyReportedScreenReader` at login; the MTTS cycle in `term_iac.go` (`recordTermType`, `maxTermTypeReplies`).
- `internal/users/context.md`: add `TurnOnScreenReader` (the only way the setting turns on; saves light combat text) and `ScreenReaderCombatLine`.

Verify every symbol named exists:

Run: `python tools/context_md_audit.py`
Expected: no phantom symbols reported for the four files.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/set.template docs/PATCH_NOTES.md modules/gmcp/context.md internal/connections/context.md internal/inputhandlers/context.md internal/users/context.md
git commit -F - <<'EOF'
docs: screen reader opt-in in help, patch notes and package docs (#733)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Headless check, gates and PR (controller)

This task is the controller's, not a subagent's: it boots a server and shows results to the owner.

- [ ] **Step 1: Boot a test server**

Follow `dogmud-shipping` "Boot check in an isolated worktree": a detached worktree at the branch head, `CONFIG_PATH` pointing at a scratchpad override that moves every port off the owner's, instance saves wiped. Wait for `Server Ready`. Note the web port and the PID.

- [ ] **Step 2: Run the headless check**

Write this to `<scratchpad>/sr-check/check.js` and run `node <scratchpad>/sr-check/check.js http://127.0.0.1:<webport>/webclient`:

```js
// Headless check for touch arc Task 0. Sighted path, opt-in path, creation
// by the accessibility tree, the account round trip, and reload memory.
const { chromium } = require('C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright');

const url = process.argv[2];
const fails = [];
function ok(name, cond, detail) {
    console.log((cond ? 'ok   ' : 'FAIL ') + name + (cond ? '' : '  ' + (detail || '')));
    if (!cond) fails.push(name);
}

async function treeText(page) {
    return page.evaluate(() => {
        const t = document.querySelector('.xterm-accessibility-tree');
        return t ? t.textContent : '';
    });
}
async function waitFor(page, re, ms) {
    const end = Date.now() + (ms || 8000);
    while (Date.now() < end) {
        if (re.test(await treeText(page))) return true;
        await page.waitForTimeout(200);
    }
    return false;
}
async function type(page, text) {
    await page.fill('#command-input', text);
    await page.press('#command-input', 'Enter');
}

(async () => {
    const browser = await chromium.launch();
    const page = await (await browser.newContext()).newPage();
    const sent = [];
    page.on('websocket', ws => ws.on('framesent', f => {
        sent.push(typeof f.payload === 'string' ? f.payload : Buffer.from(f.payload).toString('latin1'));
    }));

    await page.goto(url);
    await page.waitForTimeout(2500);

    // Sighted: nothing runs, the label and hint are in place.
    ok('sighted: no accessibility tree', await page.locator('.xterm-accessibility').count() === 0);
    ok('label names the phrase', /type screen reader on/.test(await page.getAttribute('#command-input', 'aria-label') || ''));
    ok('hint is first in body', await page.evaluate(() => document.body.firstElementChild && document.body.firstElementChild.id === 'sr-hint'));
    ok('hint is not visible', await page.evaluate(() => { const r = document.getElementById('sr-hint').getBoundingClientRect(); return r.width <= 1 && r.height <= 1; }));

    // Opt in.
    await type(page, 'Screen  Reader ON');
    await page.waitForTimeout(500);
    ok('opt-in: accessibility tree exists', await page.locator('.xterm-accessibility').count() === 1);
    ok('opt-in: label shortened', await page.getAttribute('#command-input', 'aria-label') === 'Game commands.');
    ok('opt-in: phrase never sent as text', !sent.some(s => /screen\s*reader/i.test(s) && !s.includes('Client.Display')), JSON.stringify(sent));
    ok('opt-in: reported', sent.some(s => s.includes('Client.Display') && s.includes('"screenreader":true')));
    ok('opt-in: remembered', await page.evaluate(() => localStorage.getItem('screenReader')) === 'true');
    ok('opt-in: confirmation read', await waitFor(page, /Screen reader mode is on\./, 3000));

    // Create a character by the accessibility tree; the question is never asked.
    const name = 'srcheck' + String(Date.now()).slice(-6);
    await type(page, 'new');
    ok('creation: name prompt read', await waitFor(page, /login name/i));
    await type(page, name);
    ok('creation: password prompt read', await waitFor(page, /Choose your password/));
    await type(page, 'Sr-check-pass-1');
    ok('creation: repeat prompt read', await waitFor(page, /Repeat your password/));
    await type(page, 'Sr-check-pass-1');
    if (await waitFor(page, /Email Address/, 4000)) { await type(page, ''); }
    ok('creation: confirm prompt read', await waitFor(page, /create a new user/i));
    ok('creation: screen reader question never asked', !/Are you using a screen reader/.test(await treeText(page)));
    await type(page, 'y');
    ok('creation: combat line read', await waitFor(page, /Combat text is set to light/, 10000));

    // The account came back on in Char.Info; set screenreader turns it off everywhere.
    await page.waitForTimeout(2000);
    ok('in game: mode still on', await page.locator('.xterm-accessibility').count() === 1);
    await type(page, 'set screenreader');
    await page.waitForTimeout(1500);
    ok('set screenreader off: mode off', await page.locator('.xterm-accessibility').count() === 0);
    ok('set screenreader off: remembered off', await page.evaluate(() => localStorage.getItem('screenReader')) === 'false');

    // Reload with the setting remembered on: on before anything is typed.
    await page.evaluate(() => localStorage.setItem('screenReader', 'true'));
    await page.reload();
    await page.waitForTimeout(2500);
    ok('reload: on before typing', await page.locator('.xterm-accessibility').count() === 1);

    await browser.close();
    console.log(fails.length ? fails.length + ' failure(s)' : 'all passed');
    process.exit(fails.length ? 1 : 0);
})();
```

If a prompt regex does not match the live text, read the tree text the check prints on failure, fix the regex in the check (never the game), and rerun. Expected: `all passed`.

- [ ] **Step 3: Telnet check**

With the test server still up, connect a raw telnet session from a scratch script that answers `IAC DO TTYPE` with `IAC WILL TTYPE`, then each `SEND` with `MUDLET`, `ANSI-TRUECOLOR`, `MTTS 73`; create a character; confirm the screen reader question is not asked and `set` shows ScreenReader ON and combatverbosity light. Repeat without answering TTYPE and confirm the question is asked. Record the two transcripts in the scratchpad.

- [ ] **Step 4: Gates**

In this order (`dogmud-shipping` "Pre-push gate order"):

```bash
gofmt -l internal/ modules/ main.go          # prints nothing
go build ./... && go test ./... -count=1     # all pass
~/go/bin/golangci-lint run --new-from-merge-base=origin/master   # 0 issues
```

Plus the JS loop from Task 7 Step 8. Kill the test server by its PID.

- [ ] **Step 5: Owner test (Windows Narrator)**

Hand the owner a short punchlist, player steps only: start Narrator (Ctrl+Win+Enter), open the local web client, hear the instruction, type `screen reader on`, create a character by ear, hear that combat text is light, fight the first rat in the Pothole Coulee, then `set screenreader` and hear it stop. Wait for his result.

- [ ] **Step 6: PR**

Push the branch and open the PR with `gh pr create --repo pruuk/DOGMud --base master`. Body: what players get, the rulings, the tests and checks with their results, "Device test on iPhone VoiceOver and Android TalkBack is #834 (Malia)", and "Part of #733" (never "closes #733": the issue stays open for the rest of its questions). End with the attribution line.
