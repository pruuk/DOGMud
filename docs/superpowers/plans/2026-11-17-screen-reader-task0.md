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
| `_datafiles/html/public/webclient-pure.html` | hint, status region, label, script, wiring |
| `_datafiles/html/public/_header.html`, `static/css/gomud.css`, `static/css/dashboard.css` | the outer page's hint, its `.visually-hidden`, the overlay's `pointer-events: none` |
| `internal/usercommands/start.go`, `start_screenreader_test.go` (new) | the newcomer route keeps light for a screen reader user |
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
		// The drain returns heap order, not delivery order (events.go:893).
		require.ElementsMatch(t, []string{`screenreader`, `combatverbosity`}, drainSettingChanges(t))
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

// A reported screen reader: the notices, then the combat lines; nothing when
// the setting is already on.
func TestTurnOnReportedScreenReader(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7}
	require.Equal(t, append(append([]string{}, ScreenReaderOnNotices...), ScreenReaderCombatLines...), u.TurnOnReportedScreenReader())
	require.True(t, u.ScreenReader)
	light := &UserRecord{UserId: 7, CombatVerbosity: `light`}
	require.Equal(t, ScreenReaderOnNotices, light.TurnOnReportedScreenReader(), "already light: no combat lines")
	require.Empty(t, u.TurnOnReportedScreenReader(), "already on: nothing")
	drainSettingChanges(t)
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

Run: `go test ./internal/users/ -run TestTurnOn -count=1`
Expected: FAIL, `u.TurnOnScreenReader undefined`.

- [ ] **Step 4: Implement**

Create `internal/users/screenreader.go`:

```go
package users

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// ScreenReaderCombatLines tell a player whose screen reader setting just
// turned on that combat text was set to light, and how to undo it. xterm's
// screen reader mode stops announcing after 20 lines of one burst, so full
// combat text would be cut off. Two lines, each under 80 columns, because
// login text reaches telnet unwrapped.
var ScreenReaderCombatLines = []string{
	`Combat text is set to light, so your screen reader can keep up.`,
	`Type set combatverbosity full to see every blow.`,
}

// ScreenReaderOnNotices tell a player that their connection turned the
// account's screen reader setting on, and how to turn it off. Two lines,
// each under 80 columns, because login text reaches telnet unwrapped.
var ScreenReaderOnNotices = []string{
	`Screen reader mode is on for this character.`,
	`Type set screenreader to turn it off.`,
}

// TurnOnReportedScreenReader turns u's setting on for a connection that
// reported a screen reader, and returns the lines to tell the player: the
// notices, then the combat lines when combat text changed. It returns none
// when the setting was already on. A caller must own u: before login
// publishes it, or on MainWorker.
func (u *UserRecord) TurnOnReportedScreenReader() []string {
	if u.ScreenReader {
		return nil
	}
	lines := slices.Clone(ScreenReaderOnNotices)
	if u.TurnOnScreenReader() {
		lines = append(lines, ScreenReaderCombatLines...)
	}
	return lines
}

// TurnOnScreenReader is the only way an account's screen reader setting
// turns on (the creation answer, a connection's report at login, and set
// screenreader). It sets the flag and saves light combat text, firing a
// UserSettingChanged for each. It reports whether the combat level changed,
// so the caller can send ScreenReaderCombatLines. When the setting is
// already on it does nothing and reports false. Both values are saved with
// the account at its next save (autosave, logout or creation), as every set
// command's are. At creation the user has no id yet (UserId 0), so the
// events are dropped by their listeners; the full Char push at spawn carries
// the setting to the client instead.
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

Run: `go test ./internal/users/ -run TestTurnOn -count=1`
Expected: PASS.

- [ ] **Step 6: `set screenreader` turns on through it**

`set screenreader` with no word still toggles. `set screenreader on` and `set screenreader off` set a state, so the web client (which sends one for the player, Task 6 `setCommand`) can never flip the account the wrong way when its belief is stale: a line swallowed by an open prompt (`internal/users/userrecord.promptroute.go:49`) is simply repeated correctly by the next phrase.

In `Set`'s switch (`set.go:44`), change `return cmdSetScreenReader(user)` to `return cmdSetScreenReader(user, args)`, and replace `cmdSetScreenReader` at `set.go:216-230` with:

```go
// cmdSetScreenReader sets the account's screen reader setting: "on" or
// "off" set it, and no word toggles it. Turning it on goes through
// TurnOnScreenReader, which also sets light combat text; turning it off
// leaves combat text as it is. Setting it to the state it already has
// changes nothing and says so.
func cmdSetScreenReader(user *users.UserRecord, args []string) (bool, error) {
	want := !user.ScreenReader
	if len(args) > 0 {
		switch args[0] {
		case `on`:
			want = true
		case `off`:
			want = false
		default:
			user.SendText(messaging.CategoryRefusal, `Type <ansi fg="command">set screenreader on</ansi> or <ansi fg="command">set screenreader off</ansi>.`)
			return true, nil
		}
	}

	if want == user.ScreenReader {
		state := `OFF`
		if want {
			state = `ON`
		}
		user.SendText(messaging.CategorySystem, `ScreenReader mode is already <ansi fg="red">`+state+`</ansi>.`)
		return true, nil
	}

	if !want {
		// The flag goes off before the line is sent, so "OFF" itself is
		// rendered without screen reader stripping. It has no art, so it reads
		// the same either way.
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
		for _, line := range users.ScreenReaderCombatLines {
			user.SendText(messaging.CategorySystem, line)
		}
	}
	return true, nil
}
```

Add a test to `internal/usercommands/usercommands_test.go` beside `TestSetSubCommands` (`:2870`), using the same fixture it uses: `seedAllRegistries()` first, then `getTestUserAndRoom(t)` (`:379`). The user is shared, so restore it and drain the events it queues:

```go
// set screenreader: on saves light combat text; off leaves it as it is.
func TestSetScreenReader_OnSetsLightOffLeavesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, _ := getTestUserAndRoom(t)
	prevSR, prevCV := user.ScreenReader, user.CombatVerbosity
	t.Cleanup(func() {
		user.ScreenReader, user.CombatVerbosity = prevSR, prevCV
		events.DrainQueuedByTypeForTest(`UserSettingChanged`)
	})
	user.ScreenReader = false
	user.CombatVerbosity = ``

	cmdSetScreenReader(user, nil)
	require.True(t, user.ScreenReader)
	require.Equal(t, `light`, user.CombatVerbosity)

	cmdSetScreenReader(user, nil)
	require.False(t, user.ScreenReader)
	require.Equal(t, `light`, user.CombatVerbosity, "turning off leaves combat text alone")

	// Explicit words set a state: repeating one never flips it back.
	cmdSetScreenReader(user, []string{`on`})
	cmdSetScreenReader(user, []string{`on`})
	require.True(t, user.ScreenReader, "on twice stays on")
	cmdSetScreenReader(user, []string{`off`})
	cmdSetScreenReader(user, []string{`off`})
	require.False(t, user.ScreenReader, "off twice stays off")
	cmdSetScreenReader(user, []string{`maybe`})
	require.False(t, user.ScreenReader, "an unknown word changes nothing")
}
```

Run: `go test ./internal/usercommands/ -run TestSetScreenReader -count=1`
Expected: PASS (written after Step 6; it guards the path, it is not a red-first test).

- [ ] **Step 7: Build and run the touched packages**

Run: `go build ./... && go test ./internal/users/ ./internal/usercommands/ ./internal/connections/ -count=1`
Expected: build succeeds, all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/connections/clientsettings.go internal/users/screenreader.go internal/users/screenreader_test.go internal/usercommands/set.go internal/usercommands/usercommands_test.go
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
Expected: FAIL on `TestClientDisplay_StoresScreenReader` and `TestClientDisplay_ScreenReaderMissingOrBadKeeps`. `TestClientDisplay_ScreenReaderIgnoredOnTelnet` passes already (the stored value starts false): it is a regression guard, not a red-first test.

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

- [ ] **Step 4a: The websocket address carries it too, for the connect splash**

The connect splash goes out as the socket opens, before any `Client.Display` (`main.go:1390-1397`), and the 80-column splash is 24 lines of art (`templates/login/connect-splash.template`). A returning blind player would hear star-field punctuation. The client adds `&sr=1` to the address (Task 7 Step 11) when its mode is on; the server records it and draws the words-only splash.

Write the failing test in `internal/users/screenreader_test.go`:

```go
// sr=1 on the websocket address marks the connection; anything else does not.
func TestStoreAddressScreenReader(t *testing.T) {
	const id connections.ConnectionId = 97601
	connections.RegisterTestWebsocketConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	for _, raw := range []string{``, `0`, `true`, `yes`} {
		require.False(t, StoreAddressScreenReader(id, raw), raw)
		require.False(t, connections.GetClientSettings(id).ScreenReader, raw)
	}
	require.True(t, StoreAddressScreenReader(id, `1`))
	require.True(t, connections.GetClientSettings(id).ScreenReader)
}
```

(add `"github.com/GoMudEngine/GoMud/internal/connections"` to the test's imports). Run `go test ./internal/users/ -run TestStoreAddressScreenReader -count=1`: FAIL (undefined). Then add to `internal/users/screenreader.go` (and import `connections` there):

```go
// StoreAddressScreenReader takes the raw sr value of a web client's websocket
// address (/ws?cols=57&sr=1) and, when it is "1", records on the connection
// that its player asked for screen reader mode, so the connect splash goes
// out as words only. It reports whether it did. The caller checks that the
// connection is a websocket.
func StoreAddressScreenReader(id connections.ConnectionId, raw string) bool {
	if raw != `1` {
		return false
	}
	cs := connections.GetClientSettings(id)
	cs.ScreenReader = true
	connections.OverwriteClientSettings(id, cs)
	return true
}
```

Pass the value through:
- `internal/web/web.go:311`: `Listen`'s handler type becomes `func(*websocket.Conn, string, string, string)`; beside `reportedCols := r.URL.Query().Get("cols")` (`:357`) add `reportedSR := r.URL.Query().Get("sr")`, and call `webSocketHandler(conn, clientIP, reportedCols, reportedSR)`. Extend `Listen`'s doc comment: "The fourth is the raw sr value (/ws?sr=1): the player asked for screen reader mode."
- `main.go:1303`: `func HandleWebSocketConnection(conn *websocket.Conn, clientIP string, reportedCols string, reportedSR string)`, with a doc sentence for `reportedSR`. At the splash (`:1395-1397`), replace

```go
	splashTxt, _ := templates.ProcessAtWidth("login/connect-splash", nil, splashWidth)
```

with

```go
	// A screen reader gets its own splash (connect-splash.screenreader.template):
	// no art, no rule of = signs, no spaced-out title, all of which a screen
	// reader reads symbol by symbol or letter by letter.
	var splashTxt string
	if users.StoreAddressScreenReader(connDetails.ConnectionId(), reportedSR) {
		splashTxt, _ = templates.ProcessForWidth("login/connect-splash", nil, true, splashWidth)
	} else {
		splashTxt, _ = templates.ProcessAtWidth("login/connect-splash", nil, splashWidth)
	}
```

`templates.processPicked` tries `<name>.screenreader.template` first when the config says screen reader (`internal/templates/templates.go:191-194`), before any width form. Create `_datafiles/world/dogmud/templates/login/connect-splash.screenreader.template`, every line at most 30 columns so the template guard passes at every width:

```
{{- /* The connect splash for a screen reader: plain words only. No art,
no rule of = signs and no spaced-out title, which a screen reader reads
symbol by symbol or letter by letter. Chosen when the web client's
websocket address carries sr=1 (main.go). */ -}}
<ansi clear="all"></ansi>
Welcome to Delusions of
Grandeur, a MUD set on the
world of Gaius.
The Chrysalis awaits.
```

Write its test first, in a new `internal/inputhandlers/connect_splash_screenreader_test.go` (the package's `loginRenderSetup(t)`, `login_width_test.go:24`, loads the live templates):

```go
package inputhandlers

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// A screen reader's connect splash is plain words at every width: no art,
// no rule of = signs, no spaced-out title.
func TestConnectSplash_ScreenReaderIsPlainWords(t *testing.T) {
	loginRenderSetup(t)
	for _, w := range []int{30, 42, 80} {
		out, err := templates.ProcessForWidth("login/connect-splash", nil, true, w)
		require.NoError(t, err)
		require.Contains(t, out, `Welcome to Delusions of`, w)
		require.NotContains(t, out, `===`, w)
		require.NotContains(t, out, `D e l u s`, w)
		require.False(t, strings.Contains(out, `*`), "no star art at %d", w)
	}
}
```

Run it before creating the template: FAIL (the 80 base is the art). Create the template: PASS.

Run `go build ./... && go test ./internal/users/ ./internal/web/ ./internal/guards/ ./internal/inputhandlers/ ./internal/templates/ -count=1`: PASS (`internal/guards/websocket_input_cleaned_test.go` finds `HandleWebSocketConnection` by its name prefix, so the signature change is safe; the template guards cover the new template at 30, 42 and 80; if anything fails, read why before changing anything).

- [ ] **Step 5: Commit**

```bash
git add internal/users/screenreader.go internal/users/screenreader_test.go internal/web/web.go main.go modules/gmcp/gmcp.ClientDisplay.go modules/gmcp/gmcp.ClientDisplay_test.go _datafiles/world/dogmud/templates/login/connect-splash.screenreader.template internal/inputhandlers/connect_splash_screenreader_test.go
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
		return map[string]string{`username`: `new`, connFactWebsocket: ws, connFactMTTS: mtts}
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
	require.Equal(t, map[string]string{connFactWebsocket: `y`, connFactMTTS: `n`}, r)
	r = map[string]string{}
	writeConnFacts(r, tel)
	require.Equal(t, map[string]string{connFactWebsocket: `n`, connFactMTTS: `y`}, r)
}

// The handler really writes the facts before a condition: a telnet client
// that answered MTTS skips the question. Leaving the writeConnFacts call
// out of advanceAndSendPrompt fails this. (A telnet test connection: a
// skipped step sends nothing, and the sequence completes.)
func TestAdvanceAndSendPrompt_SkipsTheQuestionOnMTTS(t *testing.T) {
	const tel connections.ConnectionId = 97403
	connections.RegisterTestConnection(tel)
	t.Cleanup(func() { connections.UnregisterTestConnection(tel) })
	cs := connections.GetClientSettings(tel)
	cs.MTTSReported = true
	connections.OverwriteClientSettings(tel, cs)

	state := &PromptHandlerState{
		Steps:            []*PromptStep{screenReaderStep(t)},
		CurrentStepIndex: -1,
		Results:          map[string]string{`username`: `new`},
	}
	require.True(t, advanceAndSendPrompt(state, &connections.ClientInput{ConnectionId: tel}), "the only step is skipped")
	require.Equal(t, `y`, state.Results[connFactMTTS])
}

func drainSettingEvents(t *testing.T) {
	t.Helper()
	events.DrainQueuedByTypeForTest(`UserSettingChanged`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`UserSettingChanged`) })
}

// A connection's report turns an account's setting on, never off, and says so.
func TestReportedScreenReaderLines(t *testing.T) {
	drainSettingEvents(t)

	off := &users.UserRecord{UserId: 8}
	lines := reportedScreenReaderLines(off, true)
	require.True(t, off.ScreenReader)
	require.Equal(t, `light`, off.CombatVerbosity)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	on := &users.UserRecord{UserId: 9, ScreenReader: true}
	require.Empty(t, reportedScreenReaderLines(on, false))
	require.True(t, on.ScreenReader, "a connection never turns the setting off")

	quiet := &users.UserRecord{UserId: 10}
	require.Empty(t, reportedScreenReaderLines(quiet, false))
	require.False(t, quiet.ScreenReader)
}

// A reconnect to a link-dead session continues the user already in the world
// (users.go:240-242), not the record loaded for this login. That user is read
// by MainWorker every round, so this goroutine must not write it: the report
// goes to MainWorker as ScreenReaderReported, and no lines are sent here.
func TestLoginWithScreenReader_LinkDeadReconnect(t *testing.T) {
	drainSettingEvents(t)
	events.DrainQueuedByTypeForTest(`ScreenReaderReported`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`ScreenReaderReported`) })
	loaded := &users.UserRecord{UserId: 11}
	inMemory := &users.UserRecord{UserId: 11}
	fakeLogin := func(u *users.UserRecord) (*users.UserRecord, string, error) {
		require.Same(t, loaded, u)
		require.True(t, u.ScreenReader, "the report is applied before the user is published")
		return inMemory, ``, nil
	}
	got, _, lines, err := loginWithScreenReader(loaded, true, fakeLogin)
	require.NoError(t, err)
	require.Same(t, inMemory, got)
	require.False(t, inMemory.ScreenReader, "the live user is not written from this goroutine")
	require.Empty(t, lines, "the hook tells the player, so login sends nothing")
	queued := events.DrainQueuedByTypeForTest(`ScreenReaderReported`)
	require.Equal(t, []events.Event{events.ScreenReaderReported{UserId: 11}}, queued)

	// No report: nothing queued.
	_, _, _, _ = loginWithScreenReader(&users.UserRecord{UserId: 11}, false, fakeLoginNoCheck(inMemory))
	require.Empty(t, events.DrainQueuedByTypeForTest(`ScreenReaderReported`))
}

func fakeLoginNoCheck(ret *users.UserRecord) func(*users.UserRecord) (*users.UserRecord, string, error) {
	return func(*users.UserRecord) (*users.UserRecord, string, error) { return ret, ``, nil }
}

// The ordinary login: the same record comes back, turned on once.
func TestLoginWithScreenReader_Ordinary(t *testing.T) {
	drainSettingEvents(t)
	loaded := &users.UserRecord{UserId: 12}
	got, _, lines, _ := loginWithScreenReader(loaded, true, func(u *users.UserRecord) (*users.UserRecord, string, error) { return u, ``, nil })
	require.Same(t, loaded, got)
	require.True(t, got.ScreenReader)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	quiet := &users.UserRecord{UserId: 13}
	_, _, lines, _ = loginWithScreenReader(quiet, false, func(u *users.UserRecord) (*users.UserRecord, string, error) { return u, ``, nil })
	require.Empty(t, lines)
	require.False(t, quiet.ScreenReader)
}

// Creation: asked and answered yes gets only the combat lines (the player just
// said so); not asked but reported also hears that the mode is on and how to
// turn it off; neither gets nothing.
func TestCreationScreenReaderLines(t *testing.T) {
	drainSettingEvents(t)

	asked := &users.UserRecord{}
	lines := creationScreenReaderLines(asked, map[string]string{`screen-reader-new`: `y`}, false)
	require.True(t, asked.ScreenReader)
	require.Equal(t, users.ScreenReaderCombatLines, lines)

	reported := &users.UserRecord{}
	lines = creationScreenReaderLines(reported, map[string]string{}, true)
	require.True(t, reported.ScreenReader)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	no := &users.UserRecord{}
	require.Empty(t, creationScreenReaderLines(no, map[string]string{`screen-reader-new`: `n`}, true))
	require.False(t, no.ScreenReader, "an answered no wins over a report")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/inputhandlers/ -run "TestLoginSteps_NoStepUsesAConnFactID|TestScreenReaderQuestion|TestNewUserScreenReader|TestWriteConnFacts|TestAdvanceAndSendPrompt_SkipsTheQuestionOnMTTS|TestReportedScreenReaderLines|TestLoginWithScreenReader|TestCreationScreenReaderLines" -count=1`
Expected: FAIL to compile (`loginSteps`, `connFactPrefix` and the helpers are undefined).

- [ ] **Step 3: Connection facts in the prompt handler**

In `internal/inputhandlers/login_prompt_handler.go`, after the `ConditionFunc` type, add:

```go
// Reserved result keys. A ConditionFunc sees only the results map, so the
// prompt handler writes the connection's facts there, as "y" or "n", before
// it checks each step. No step may use an ID starting connFactPrefix
// (TestLoginSteps_NoStepUsesAConnFactID).
const (
	connFactPrefix    = `_conn_`
	connFactWebsocket = `_conn_websocket`
	connFactMTTS      = `_conn_mtts`
)

// writeConnFacts stores what the connection's client has said about itself
// in results under the reserved keys. (The screen reader report itself is
// read from ClientSettings when the account is made, not from here.)
func writeConnFacts(results map[string]string, id connections.ConnectionId) {
	results[connFactWebsocket] = yesNo(connections.IsWebsocket(id))
	results[connFactMTTS] = yesNo(connections.GetClientSettings(id).MTTSReported)
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

// GetLoginPromptHandler returns a new login and creation prompt sequence
// for one connection.
func GetLoginPromptHandler() connections.InputHandler {
	// Create and return the handler using the generic factory function
	return CreatePromptHandler(loginSteps(), FinalizeLoginOrCreate)
}
```

(It has no doc comment today; the lint gate requires one on an edited declaration.)

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

// reportedScreenReaderLines turns u's setting on when its connection
// reported a screen reader, and returns the lines that tell the player so
// (none when nothing changed). A connection never turns the setting off.
// Only for a record no other goroutine can see yet.
func reportedScreenReaderLines(u *users.UserRecord, reported bool) []string {
	if !reported {
		return nil
	}
	return u.TurnOnReportedScreenReader()
}

// loginWithScreenReader logs tmpUser in with login (users.LoginUser in
// production) and applies the connection's screen reader report. On an
// ordinary login it turns tmpUser on before login publishes it, so
// MainWorker never reads the fields mid-write, and returns the lines to
// tell the player. On a reconnect to a link-dead session login returns the
// user already in the world (users.go:240-242), which MainWorker reads every
// round, so this goroutine never writes it: it queues ScreenReaderReported
// and the hook on MainWorker turns it on and tells the player
// (hooks.TurnOnReportedScreenReader). tmpUser's change is then discarded.
func loginWithScreenReader(tmpUser *users.UserRecord, reported bool, login func(*users.UserRecord) (*users.UserRecord, string, error)) (*users.UserRecord, string, []string, error) {
	lines := reportedScreenReaderLines(tmpUser, reported)
	loggedIn, msg, err := login(tmpUser)
	if err != nil {
		return loggedIn, msg, nil, err
	}
	if loggedIn != tmpUser {
		if reported {
			events.AddToQueue(events.ScreenReaderReported{UserId: loggedIn.UserId})
		}
		return loggedIn, msg, nil, nil
	}
	return loggedIn, msg, lines, nil
}

// creationScreenReaderLines sets a new account's screen reader setting and
// returns the lines to tell the player. When the question was asked, the
// answer decides and only the combat lines follow (the player just said
// yes). When it was skipped, the connection's report decides and the player
// also hears that the mode is on and how to turn it off.
func creationScreenReaderLines(u *users.UserRecord, results map[string]string, reported bool) []string {
	if !newUserScreenReader(results, reported) {
		return nil
	}
	lines := []string{}
	if _, asked := results["screen-reader-new"]; !asked {
		lines = append(lines, users.ScreenReaderOnNotices...)
	}
	if u.TurnOnScreenReader() {
		lines = append(lines, users.ScreenReaderCombatLines...)
	}
	return lines
}

// sendLoginLines sends each line as login text, one per line.
func sendLoginLines(id connections.ConnectionId, lines []string) {
	for _, line := range lines {
		sendLoginText(id, line)
		connections.SendTo(term.CRLF, id)
	}
}
```

- [ ] **Step 5: Use them in `FinalizeLoginOrCreate`**

Creation, replace `newUser.ScreenReader = results["screen-reader-new"] == `y`` (`login.go:160`) with:

```go
		srCreationLines := creationScreenReaderLines(newUser, results, connections.GetClientSettings(clientInput.ConnectionId).ScreenReader)
```

After `sharedState["UserObject"] = newUser // For main loop` in the creation branch, add:

```go
		sendLoginLines(clientInput.ConnectionId, srCreationLines)
```

Existing login: replace

```go
			loggedInUser, msg, err := users.LoginUser(tmpUser, clientInput.ConnectionId)
```

with

```go
			// The connection's screen reader report turns the account's setting
			// on (never off): before the user is published, and again on the
			// link-dead session a reconnect continues (loginWithScreenReader).
			loggedInUser, msg, srLines, err := loginWithScreenReader(tmpUser,
				connections.GetClientSettings(clientInput.ConnectionId).ScreenReader,
				func(u *users.UserRecord) (*users.UserRecord, string, error) {
					return users.LoginUser(u, clientInput.ConnectionId)
				})
```

and after `sharedState["UserObject"] = loggedInUser // For main loop`, add:

```go
			sendLoginLines(clientInput.ConnectionId, srLines)
```

If `LoginUser` fails, the change lives only on the discarded `tmpUser`; nothing is saved. Add `"slices"` to `login.go`'s imports (it has none today). The three call sites in `FinalizeLoginOrCreate` are one line each; the decisions are in the helpers the tests above cover, and the Task 9 headless and telnet checks exercise the call sites.

- [ ] **Step 5b: A link-dead session is turned on by MainWorker**

`loginWithScreenReader` queues `events.ScreenReaderReported` instead of writing a user that is already in the world. Add the event to `internal/events/eventtypes.go`, beside `UserSettingChanged` (`:513`):

```go
// ScreenReaderReported asks MainWorker to turn on a logged-in user's screen
// reader setting because their new connection reported a screen reader. Login
// queues it when it continues a link-dead session already in the world, which
// the connection goroutine must not write (inputhandlers.loginWithScreenReader).
type ScreenReaderReported struct {
	UserId int
}

// Type names the event.
func (s ScreenReaderReported) Type() string { return `ScreenReaderReported` }
```

Create `internal/hooks/ScreenReaderReported_TurnOn.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// TurnOnReportedScreenReader turns on the screen reader setting of a user
// whose reconnecting connection reported a screen reader, on MainWorker,
// and tells them so. The setting is never turned off here.
func TurnOnReportedScreenReader(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.ScreenReaderReported)
	if !ok {
		mudlog.Error("Event", "Expected Type", "ScreenReaderReported", "Actual Type", e.Type())
		return events.Cancel
	}
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	for _, line := range u.TurnOnReportedScreenReader() {
		u.SendText(messaging.CategorySystem, line)
	}
	return events.Continue
}
```

Register it in `internal/hooks/hooks.go` beside the `UserSettingChanged` listener (`:138`):

```go
	// A reconnect continued a link-dead session whose new connection reported
	// a screen reader: turn it on from here, not from the login goroutine.
	events.RegisterListener(events.ScreenReaderReported{}, TurnOnReportedScreenReader)
```

The decision is `UserRecord.TurnOnReportedScreenReader`, tested in Task 1; this hook is its wiring, exercised by the link-dead telnet check in Task 9 Step 3. Run `go build ./... && go test ./internal/hooks/ ./internal/events/ -count=1`: PASS.

- [ ] **Step 5a: The newcomer route keeps light for a screen reader user**

`internal/usercommands/start.go:246` (`startInAntechamber`, route "New to text MUDs") sets `user.CombatVerbosity = "medium"` unconditionally, which would undo the light setting a blind player just got. Write the failing test first, in a new `internal/usercommands/start_screenreader_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The tutorial starts newcomers at medium, except a screen reader user,
// who keeps light (spec 2026-11-17, ruling 5).
func TestNewcomerCombatVerbosity(t *testing.T) {
	require.Equal(t, `medium`, newcomerCombatVerbosity(``, false))
	require.Equal(t, `medium`, newcomerCombatVerbosity(`light`, false))
	require.Equal(t, `light`, newcomerCombatVerbosity(`light`, true))
}
```

Run: `go test ./internal/usercommands/ -run TestNewcomerCombatVerbosity -count=1` (expect FAIL: undefined). Then add to `start.go`:

```go
// newcomerCombatVerbosity is the combat text level the "New to text MUDs"
// route starts a player at: medium, unless the player uses a screen reader,
// whose light setting is kept so the reader can keep up.
func newcomerCombatVerbosity(current string, screenReader bool) string {
	if screenReader {
		return current
	}
	return `medium`
}
```

and replace `user.CombatVerbosity = "medium"` at `:246` with `user.CombatVerbosity = newcomerCombatVerbosity(user.CombatVerbosity, user.ScreenReader)`, adding one sentence to the comment above it: "A screen reader user keeps light." Run the test again: PASS.

- [ ] **Step 6: Run the tests to see them pass**

Run: `go test ./internal/inputhandlers/ ./internal/usercommands/ -count=1`
Expected: PASS, every existing login test included.

- [ ] **Step 7: Commit**

```bash
git add internal/inputhandlers/login_prompt_handler.go internal/inputhandlers/login.go internal/inputhandlers/login_screenreader_test.go internal/usercommands/start.go internal/usercommands/start_screenreader_test.go internal/events/eventtypes.go internal/hooks/ScreenReaderReported_TurnOn.go internal/hooks/hooks.go
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

// A client that refuses terminal type changes nothing: the question is asked.
// (A regression guard: it passes before Step 7 too, since an unhandled WONT
// also changes nothing.)
func TestTelnetIACHandler_WontTermTypeChangesNothing(t *testing.T) {
	id := termTypeConn(t)
	before := connections.GetClientSettings(id)
	TelnetIACHandler(&connections.ClientInput{ConnectionId: id, DataIn: []byte{term.TELNET_IAC, term.TELNET_WONT, term.TELNET_OPT_TERM_TYPE}}, map[string]any{})
	require.Equal(t, before, connections.GetClientSettings(id))
}
```

(The `WILL TTYPE` branch is not unit tested: it sends, and `SendTo` on a test connection with no socket removes the connection. Task 9 Step 3's telnet check covers it.)

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

Facts this module is built on (read from `static/js/xterm.4.19.0.js`, verified by the plan review):
- `term.setOption('screenReaderMode', v)` creates or disposes xterm's `AccessibilityManager` at runtime. It puts `.xterm-accessibility` (a row mirror, `.xterm-accessibility-tree`, and an `aria-live="assertive"` `.live-region`) inside the `.xterm` element.
- Only text written AFTER the mode is on reaches the live region. A prompt already on screen is never read.
- The live region appends text and counts lines; at 21 lines it appends "Too much output to announce" and stays silent until `_clearLiveRegion()` resets it. xterm calls that only on a key in its own hidden input or when that input loses focus. Players type in `#command-input`, so without help speech stops for good after 20 lines. The page therefore resets it itself (private API, as `termfit.js` already reaches into `term._core`; xterm is pinned at 4.19, #908; the jstest below fails if the names change).
- Turning the mode off disposes the live region at once, so "off" must be spoken by our own status region.
- `term.write(data, callback)` calls back once `data` is parsed, so a reset between two writes lands between them.

Owner ruling 2026-11-17 (option 1): **an answer to a typed command is read in full; only unprompted output keeps the 20-line limit.**
- An answer runs from the moment the page sends a line (typed, a macro, a numpad move, automation: every `SendData`) until the server's `Char.Input.Done` for that line or a later one (`modules/gmcp/gmcp.InputDone.go`: queued after that command's text at priority 99, logged-in web players only; both ends number text frames from the socket's open), and then until output has been quiet for `QUIET_MS`: a macro's commands, and anything run through `user.Command()`, are queued for later turns and print after the Done (`world.go:1046-1061`). A lost reply ends the answer after `ANSWER_MAX_MS`.
- The client asks the server for a state, `set screenreader on` or `off`, never the toggle: a line swallowed by an open prompt then leaves nothing to flip the wrong way later.
- Before login everything is read in full the same way: the connect splash and the prompts must reach a returning blind player whose device remembers the mode, and there is no `Char.Input.Done` yet.
- On a Mac (and iPadOS Safari, which reports `MacIntel`), `_clearLiveRegion` also takes the region out of the page, and xterm puts it back only if more text arrives after the render. The page's reset puts it back at once.
- While an answer runs, server text is written in pieces of at most `CHUNK_LINES` lines, and the spoken region resets after each piece is parsed, so nothing in an answer is cut off.
- Outside an answer, text is written whole and the region resets after `QUIET_MS` of quiet (a combat round is 4 seconds, `Timing.RoundSeconds`), so each unprompted burst gets its own 20 lines.
- With the mode off, every write goes straight to xterm: nothing changes for sighted players.
- Writes stay in order: once a piece is waiting for its callback, later text queues behind it.

- [ ] **Step 1: Write the failing test**

Create `tools/jstest/screenreader.test.js`:

```js
// screenreader.test.js: the web client's screen reader switch (touch arc
// Task 0, docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md).
//
// Run:  node tools/jstest/screenreader.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var pub = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var SR = require(path.join(pub, 'static', 'js', 'screenreader.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// ---- the xterm internals the page relies on ---------------------------------
// The page resets xterm's spoken region through private names. An xterm
// upgrade that renames them must fail here, not go silent for blind players.
var xterm = fs.readFileSync(path.join(pub, 'static', 'js', 'xterm.4.19.0.js'), 'utf8');
check('xterm has _accessibilityManager', xterm.indexOf('this._accessibilityManager=new') >= 0, '');
check('xterm has _clearLiveRegion', xterm.indexOf('_clearLiveRegion=function') >= 0, '');
// On a Mac (iPadOS Safari reports MacIntel too) a reset takes the region out
// of the page; the page puts it back through these names.
check('xterm has _liveRegion and _accessibilityTreeRoot', xterm.indexOf('this._liveRegion') >= 0 && xterm.indexOf('_accessibilityTreeRoot.appendChild(') >= 0, '');
check('xterm stops at 21 lines (CHUNK_LINES must stay under it)', xterm.indexOf('21===this._liveRegionLineCount') >= 0 && SR.CHUNK_LINES < 21, '');

// ---- matchPhrase -----------------------------------------------------------

check('on', SR.matchPhrase('screen reader on', false) === 'on', '');
check('off', SR.matchPhrase('screen reader off', false) === 'off', '');
check('case and spaces', SR.matchPhrase('  Screen   READER  On ', false) === 'on', '');
check('dictation full stop', SR.matchPhrase('Screen reader on.', false) === 'on', '');
check('one word', SR.matchPhrase('screenreader off', false) === 'off', '');
check('password box never matches', SR.matchPhrase('screen reader on', true) === null, '');
check('other text', SR.matchPhrase('screen reader', false) === null, '');
check('a command', SR.matchPhrase('look', false) === null, '');
check('not a string', SR.matchPhrase(undefined, false) === null, '');

// ---- fakes -----------------------------------------------------------------

// A fake clock: timers run only when the test advances time.
function clock() {
    var now = 0, timers = [];
    return {
        setTimeout: function (f, ms) { var t = { at: now + ms, f: f }; timers.push(t); return t; },
        clearTimeout: function (t) { timers = timers.filter(function (x) { return x !== t; }); },
        advance: function (ms) {
            now += ms;
            var due = timers.filter(function (t) { return t.at <= now; });
            timers = timers.filter(function (t) { return t.at > now; });
            due.forEach(function (t) { t.f(); });
        }
    };
}

function fakes(saved) {
    var c = clock();
    var f = {
        mode: null, label: null, hint: null, status: '', lines: [], reports: [], setCommands: 0, setValues: [],
        cleared: 0, waiting: ['Username (or "new"): '], setOk: true,
        written: [],        // every piece handed to xterm, in order
        events: [],         // 'w:<piece>' and 'clear', interleaved, in order
        callbacks: [],      // write callbacks not yet run (a piece being parsed)
        store: saved === undefined ? {} : { screenReader: saved }, inGame: false, clock: c
    };
    f.parse = function () { var cb = f.callbacks.shift(); if (cb) cb(); };
    f.parseAll = function () { while (f.callbacks.length) f.parse(); };
    f.opts = {
        term: { setOption: function (k, v) { if (k === 'screenReaderMode') f.mode = v; } },
        termWrite: function (data, cb) {
            f.written.push(data); f.events.push('w:' + data);
            if (cb) f.callbacks.push(cb);
        },
        storage: {
            getItem: function (k) { return Object.prototype.hasOwnProperty.call(f.store, k) ? f.store[k] : null; },
            setItem: function (k, v) { f.store[k] = v; }
        },
        input: { setAttribute: function (k, v) { if (k === 'aria-label') f.label = v; } },
        hint: {},
        status: {},
        report: function (on) { f.reports.push(on); return true; },
        setCommand: function (value) { f.setCommands++; f.setValues.push(value); return f.setOk; },
        writeLine: function (t) { f.lines.push(t); },
        currentLines: function () { return f.waiting.slice(); },
        clearSpoken: function () { f.cleared++; f.events.push('clear'); },
        inGame: function () { return f.inGame; },
        setTimeout: c.setTimeout,
        clearTimeout: c.clearTimeout
    };
    Object.defineProperty(f.opts.hint, 'textContent', { set: function (v) { f.hint = v; } });
    Object.defineProperty(f.opts.status, 'textContent', { set: function (v) { f.status = v; }, get: function () { return f.status; } });
    return f;
}

function nLines(n, tag) {
    var s = '';
    for (var i = 1; i <= n; i++) s += tag + i + '\r\n';
    return s;
}
function countNewlines(s) { return (s.match(/\n/g) || []).length; }

// ---- load and storage ------------------------------------------------------

var f = fakes();
var sr = SR.create(f.opts);
sr.load();
check('fresh device: off', f.mode === false && !sr.isOn(), 'mode ' + f.mode);
check('fresh device: label names the phrase', f.label === SR.LABEL_OFF, f.label);
check('fresh device: hint names the phrase', f.hint === SR.HINT_OFF, f.hint);
check('fresh device: nothing reported or stored', f.reports.length === 0 && !('screenReader' in f.store), JSON.stringify(f.store));

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

// ---- writing: off means untouched ------------------------------------------

f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
var big = nLines(50, 'L');
sr.inputSent(1);
sr.write(big);
check('off: written whole, once, no callback', f.written.length === 1 && f.written[0] === big && f.callbacks.length === 0, JSON.stringify(f.written.length));
f.clock.advance(SR.QUIET_MS * 3);
check('off: never resets a region that is off', f.cleared === 0, 'cleared ' + f.cleared);
f.inGame = false;
sr.write(big);
check('off before login: written whole too', f.written.length === 2 && f.callbacks.length === 0, f.written.length);

// ---- writing: an answer is read in full ------------------------------------

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
check('sending a line resets the region', f.cleared === 1, 'cleared ' + f.cleared);
sr.write(big);
f.parseAll();
check('answer: every line written, in order', f.written.join('') === big, '');
check('answer: no piece over CHUNK_LINES', f.written.every(function (p) { return countNewlines(p) <= SR.CHUNK_LINES; }), f.written.map(countNewlines).join());
check('answer: more than one piece for 50 lines', f.written.length >= 3, f.written.length);
// After every piece is parsed, the region is reset before the next piece.
var ok = true, sinceClear = 0;
f.events.forEach(function (e) {
    if (e === 'clear') { sinceClear = 0; return; }
    sinceClear += countNewlines(e.slice(2));
    if (sinceClear > SR.CHUNK_LINES) ok = false;
});
check('answer: never more than CHUNK_LINES between resets', ok, f.events.join(' | ').slice(0, 300));

// Order holds when more text arrives while a piece is still being parsed.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.write(nLines(30, 'A'));
sr.write('after\r\n');
check('queued: later text waits for the piece being parsed', f.written.join('').indexOf('after') < 0, f.written.join(''));
f.parseAll();
check('queued: everything written, in order', f.written.join('') === nLines(30, 'A') + 'after\r\n', f.written.join(''));

// The answer ends QUIET_MS after its Char.Input.Done. Output in that gap is
// still part of it: a macro's commands and anything run through
// user.Command() are queued for later turns, so they print after the Done.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.write('reply\r\n');
f.parseAll();
sr.inputDone(1);
f.written = [];
sr.write(big);
f.parseAll();
check('just after Done (a macro\'s output): still read in full', f.written.length >= 3, f.written.length);
f.clock.advance(SR.QUIET_MS - 1);
f.written = [];
sr.write(big);
f.parseAll();
check('output keeps the answer open', f.written.length >= 3, f.written.length);
f.clock.advance(1); // QUIET_MS since the Done, but output came 1ms ago
f.written = [];
sr.write(big);
f.parseAll();
check('still open QUIET_MS after the Done, because output kept coming', f.written.length >= 3, f.written.length);
f.clock.advance(SR.QUIET_MS);
f.written = []; f.events = [];
sr.write(big);
check('quiet after Done: unprompted text written whole', f.written.length === 1 && f.written[0] === big, f.written.length);
f.parseAll();

// Lines are matched by number: a Done for an earlier line (an automation
// command, a numpad move) does not end a later line's answer.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.inputSent(2);
sr.inputDone(1);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('Done for line 1 while line 2 is out: still answering', f.written.length >= 3, f.written.length);
sr.inputDone(2);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('Done for the last line sent, then quiet: answer over', f.written.length === 1, f.written.length);

// A Done numbered past the last line sent also ends it (the server counted a
// line this page did not, such as a reconnect token).
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(4);
sr.inputDone(5);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('a later Done ends it', f.written.length === 1, f.written.length);

// A line sent before login opens no answer, even once the player is in.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = false;
sr.inputSent(1);
f.inGame = true;
f.written = [];
sr.write(big);
f.parseAll();
check('a login line opens no answer', f.written.length === 1, f.written.length);

// A lost Done does not leave answers on forever.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
f.clock.advance(SR.ANSWER_MAX_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('lost Done: answer ends after ANSWER_MAX_MS', f.written.length === 1, f.written.length);

// Before login everything is read in full: the connect splash and the
// prompts must reach a returning blind player, and there is no Done yet.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = false;
f.written = [];
sr.write(big);
f.parseAll();
check('before login: read in full', f.written.length >= 3 && f.written.join('') === big, f.written.length);

// ---- writing: unprompted output resets after a quiet gap -------------------

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('round 1\r\n');
f.clock.advance(SR.QUIET_MS - 1);
sr.write('round 1 more\r\n');
f.clock.advance(SR.QUIET_MS - 1);
check('output still arriving: no reset', f.cleared === 0, 'cleared ' + f.cleared);
f.clock.advance(1);
check('output quiet: reset once', f.cleared === 1, 'cleared ' + f.cleared);
check('QUIET_MS is under a 4 second combat round', SR.QUIET_MS < 4000, SR.QUIET_MS);

// ---- the phrase ------------------------------------------------------------

// Before login: turn on, remember, report, then say so and repeat the
// waiting prompt (a prompt already on screen is never read by xterm).
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.phrase('on');
check('phrase on: mode on', f.mode === true, '');
check('phrase on: remembered', f.store.screenReader === 'true', JSON.stringify(f.store));
check('phrase on: reported', f.reports.join() === 'true', f.reports.join());
check('phrase on: confirmed, then the waiting prompt', f.lines.join('|') === SR.SAID_ON + '|' + f.waiting[0], f.lines.join('|'));
check('phrase on: no set command before login', f.setCommands === 0, '');
sr.phrase('off');
check('phrase off: mode off and remembered', f.mode === false && f.store.screenReader === 'false', '');
check('phrase off: spoken by the status region', f.status === SR.SAID_OFF, f.status);
check('phrase off: shown in the game window', f.lines[f.lines.length - 1] === SR.SAID_OFF, f.lines.join('|'));

// In game, account differs: send set screenreader once; the server's line speaks.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
check('in game, account off: set command sent', f.setCommands === 1, '');
check('in game: the command names the state (on), never a toggle', f.setValues.join() === 'true', f.setValues.join());
check('in game, account off: no local line', f.lines.length === 0, f.lines.join('|'));
sr.phrase('on');
check('in game: a second phrase does not toggle it back', f.setCommands === 1, 'set commands ' + f.setCommands);

// A repeated Char.Info value is not news: the page's generic Char handler
// re-reads the last Info for Char.Enemies, Char.Stats and the like, and that
// stale value must not undo the phrase before the server answers.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
sr.account(false);   // stale: the same value as the last push
check('stale push: mode stays on', f.mode === true, 'mode ' + f.mode);
sr.phrase('on');
check('stale push: no second set command', f.setCommands === 1, 'set commands ' + f.setCommands);
sr.account(true);    // the server's real answer
check('real push: mode on', f.mode === true, '');
// The mode goes off on this device only (the set command could not be
// sent), then a new login pushes the SAME value as before: it must apply.
f.setOk = false;
sr.phrase('off');
check('local off: mode off', f.mode === false, 'mode ' + f.mode);
sr.loggedOut();
sr.account(true);
check('after logout: the same value applies again', f.mode === true, 'mode ' + f.mode);

// In game, the set command could not be sent: confirm locally instead.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
f.setOk = false;
sr.account(false);
sr.phrase('on');
check('in game, send failed: local line', f.lines[0] === SR.SAID_ON, f.lines.join('|'));

// In game, account already matches: no set command, local line, no prompt repeat.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(true);
f.lines = [];
sr.phrase('on');
check('in game, account on: no set command', f.setCommands === 0, '');
check('in game, account on: local line only', f.lines.join('|') === SR.SAID_ON, f.lines.join('|'));

// The account's setting arrives: the mode follows it and is remembered.
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.account(true);
check('account on: mode on, remembered', f.mode === true && f.store.screenReader === 'true', '');
sr.account(false);
check('account off: mode off, remembered', f.mode === false && f.store.screenReader === 'false', '');

// Logged out: the account and any open answer are forgotten. inGame stays
// true here so only loggedOut can explain the missing set command.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.inputSent(1);
sr.loggedOut();
sr.phrase('on');
check('after logout: no set command', f.setCommands === 0, 'set commands ' + f.setCommands);
check('after logout: local line', f.lines[0] === SR.SAID_ON, f.lines.join('|'));
f.written = [];
sr.write(big);
f.parseAll();
check('after logout: no answer left open', f.written.length === 1, f.written.length);

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
// players never see any of it: with the mode off, write() hands text
// straight to xterm.
//
// xterm 4.19 stops speaking after 20 lines until its spoken region is reset,
// and resets it only on a key in its own hidden input, which players never
// use. So this module resets it (owner ruling 2026-11-17):
//   - an answer to a line the page sent (from inputSent(seq) until the
//     server's Char.Input.Done for that seq or a later one), and everything
//     before login (the connect splash and the prompts), is written in
//     pieces of CHUNK_LINES, with a reset after each piece is parsed, so it
//     is read in full;
//   - other output is written whole, with a reset after QUIET_MS of quiet,
//     so each unprompted burst gets its own 20 lines.
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
    var QUIET_MS = 2000;        // under a 4 second combat round
    var CHUNK_LINES = 18;       // under xterm's 21-line cut-off
    var ANSWER_MAX_MS = 15000;  // a lost Char.Input.Done ends the answer
    var LABEL_OFF = 'Game commands. Screen reader users: type screen reader on, then press Enter.';
    var LABEL_ON = 'Game commands.';
    var HINT_OFF = 'Screen reader users: type screen reader on, then press Enter.';
    var HINT_ON = 'Screen reader mode is on. Type screen reader off to turn it off.';
    var SAID_ON = 'Screen reader mode is on.';
    var SAID_OFF = 'Screen reader mode is off.';

    // matchPhrase returns 'on', 'off' or null for a typed line. Case,
    // spacing and a trailing full stop (phone dictation adds one) do not
    // matter, and "screenreader" works as one word. A password box never
    // matches, so a password is never taken for the phrase.
    function matchPhrase(line, isPassword) {
        if (isPassword || typeof line !== 'string') return null;
        var s = line.toLowerCase().replace(/[.!?,\s]+$/, '').trim().replace(/\s+/g, ' ');
        if (s === 'screen reader on' || s === 'screenreader on') return 'on';
        if (s === 'screen reader off' || s === 'screenreader off') return 'off';
        return null;
    }

    // splitLines cuts text into pieces of at most max newline-ended lines;
    // the last piece may end without a newline (a prompt).
    function splitLines(text, max) {
        var pieces = [], start = 0, count = 0;
        for (var i = 0; i < text.length; i++) {
            if (text.charAt(i) !== '\n') continue;
            count++;
            if (count === max) {
                pieces.push(text.slice(start, i + 1));
                start = i + 1;
                count = 0;
            }
        }
        if (start < text.length) pieces.push(text.slice(start));
        return pieces;
    }

    // opts: term (xterm, for setOption); termWrite(data, cb) (xterm's write,
    // cb once data is parsed); storage (localStorage, any call may throw);
    // input (the command box); hint (the hidden first paragraph); status (a
    // visually hidden role="status" element); report(on) (sends
    // Client.Display); setCommand(on) (sends "set screenreader on" or "off",
    // a state and never a toggle, false when it could not); writeLine(text) (writes a local line into the game
    // window); currentLines() (the waiting prompt, as lines); clearSpoken()
    // (resets xterm's spoken region); inGame() (true while a character is
    // logged in); setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var on = false;
        var account = null;    // the account's setting as this page believes it
        var lastPushed = null; // the last Char.Info.screenreader value seen
        var quietTimer = null;
        var answering = false; // a sent line's answer is still arriving
        var lastSeq = 0;       // the number of the last line sent
        var answerTimer = null;
        var settling = false;  // its Done arrived; waiting for output to go quiet
        var settleTimer = null;
        var queue = [];        // pieces waiting for the one being parsed
        var parsing = false;

        function remember(value) {
            try { opts.storage.setItem(STORAGE_KEY, JSON.stringify(value)); } catch (e) { /* storage blocked: this visit only */ }
        }

        function show(value) {
            on = value;
            if (!value && quietTimer) { clearT(quietTimer); quietTimer = null; }
            opts.term.setOption('screenReaderMode', value);
            opts.input.setAttribute('aria-label', value ? LABEL_ON : LABEL_OFF);
            opts.hint.textContent = value ? HINT_ON : HINT_OFF;
        }

        // say speaks through our own status region: xterm's is gone once the
        // mode is off.
        function say(text) {
            opts.status.textContent = '';
            opts.status.textContent = text;
        }

        function endAnswer() {
            answering = false;
            settling = false;
            if (answerTimer) { clearT(answerTimer); answerTimer = null; }
            if (settleTimer) { clearT(settleTimer); settleTimer = null; }
        }

        // armSettle (re)starts the quiet wait that ends an answer whose
        // Char.Input.Done has arrived.
        function armSettle() {
            if (settleTimer) clearT(settleTimer);
            settleTimer = setT(endAnswer, QUIET_MS);
        }

        function armQuiet() {
            if (quietTimer) clearT(quietTimer);
            quietTimer = setT(function () { quietTimer = null; if (on) opts.clearSpoken(); }, QUIET_MS);
        }

        // pump writes the next queued piece; a piece that needs a reset after
        // it holds the queue until xterm has parsed it.
        function pump() {
            while (!parsing && queue.length) {
                var item = queue.shift();
                if (item.resetAfter) {
                    parsing = true;
                    opts.termWrite(item.text, function () {
                        parsing = false;
                        if (on) opts.clearSpoken();
                        pump();
                    });
                } else {
                    opts.termWrite(item.text);
                    if (on) armQuiet();
                }
            }
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

            // write is the page's only way to put server text in the game
            // window.
            write: function (text) {
                if (!on && !parsing && !queue.length) {
                    opts.termWrite(text);
                    return;
                }
                if (settling) armSettle();
                if (on && (answering || !opts.inGame())) {
                    splitLines(text, CHUNK_LINES).forEach(function (piece) {
                        queue.push({ text: piece, resetAfter: true });
                    });
                } else {
                    queue.push({ text: text, resetAfter: false });
                }
                pump();
            },

            // The page sent line number seq to the server (any line: typed,
            // a macro, a numpad move, automation). Reset now, and read its
            // answer in full. Char.Input.Done comes only after login; before
            // login everything is read in full anyway.
            inputSent: function (seq) {
                if (on) opts.clearSpoken();
                if (!opts.inGame()) return;
                lastSeq = seq;
                answering = true;
                settling = false;
                if (settleTimer) { clearT(settleTimer); settleTimer = null; }
                if (answerTimer) clearT(answerTimer);
                answerTimer = setT(endAnswer, ANSWER_MAX_MS);
            },

            // Char.Input.Done for line seq: the answer is over once the server
            // has finished the last line sent. A Done for an earlier line
            // changes nothing.
            inputDone: function (seq) {
                if (!answering || seq < lastSeq) return;
                // Not over yet: a macro's commands and anything run through
                // user.Command() print on later turns, after this Done. The
                // answer ends once output has been quiet for QUIET_MS.
                settling = true;
                armSettle();
            },

            // The player typed the phrase: which is 'on' or 'off'.
            phrase: function (which) {
                var value = which === 'on';
                // Read the waiting prompt before anything is written over it.
                var waiting = value && !opts.inGame() ? opts.currentLines() : [];
                show(value);
                remember(value);
                opts.report(value);
                if (opts.inGame() && account !== null && account !== value && opts.setCommand(value) !== false) {
                    // The server's "ScreenReader mode toggled" line confirms it.
                    // Count it as done now, so a second phrase before Char.Info
                    // returns does not toggle it back.
                    account = value;
                    if (!value) say(SAID_OFF);
                    return;
                }
                opts.writeLine(value ? SAID_ON : SAID_OFF);
                if (value) {
                    waiting.forEach(function (line) { opts.writeLine(line); });
                } else {
                    say(SAID_OFF);
                }
            },

            // Char.Info named the account's setting. The page's generic Char
            // handler re-reads the last Info for other Char packages, so a
            // value equal to the last one seen is not news and is ignored: it
            // must not undo a phrase the server has not answered yet.
            account: function (value) {
                if (value === lastPushed) return;
                lastPushed = value;
                account = value;
                if (value !== on) {
                    show(value);
                    remember(value);
                }
            },

            // The connection closed: the next login names the account again,
            // and no answer is still running.
            loggedOut: function () {
                account = null;
                lastPushed = null;
                endAnswer();
            }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY, QUIET_MS: QUIET_MS,
        CHUNK_LINES: CHUNK_LINES, ANSWER_MAX_MS: ANSWER_MAX_MS,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        create: create
    };
}));
```

Note: the page calls `inputSent(linesSent)` from inside `SendData` (Task 7), so every line it sends counts, typed or not, including `setCommand`'s `set screenreader`. The server numbers text frames the same way (`Char.Input.Done {seq}`, `modules/gmcp/gmcp.InputDone.go`), so an answer ends exactly when the last line sent is done.

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
- Modify: `_datafiles/html/public/webclient-pure.html` (script tags `:234`, `<body>` `:244`, `#command-input` `:310`, `displayReport` `:540-544`, `Char` handler `:903-906`, `Char.Input.Done` handler `:1059-1064`, `socket.onopen` `:3044` with `companionGlue.disconnected()` at `:3075`, server text write `:3205`, Enter handler `:3342` and its `SendData(line)` call)
- Modify: `_datafiles/html/public/_header.html` (after `<body>` at `:38`)
- Modify: `_datafiles/html/public/static/css/dashboard.css` (beside `.visually-hidden`, `:1959`)
- Modify: `_datafiles/html/public/static/css/gomud.css` (add `.visually-hidden`; it has none)
- Modify: `tools/jstest/roomtitlebold.test.js:33-34` (it pins the exact server-text write line, which Step 10 changes)
- Modify: `tools/jstest/webclient-connect.test.js` (it evaluates the real connect handler, which now calls `screenReader`)

Anchors are given by line and by the code text; trust the code text if a line moved.

- [ ] **Step 1: Load the module**

After the `displayreport.js` script tag (`:234`), add:

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/screenreader.js?v={{ .ASSET_VERSION }}"></script>
```

- [ ] **Step 2: The hidden first line and the status region**

Immediately after `<body onLoad="init(); return false;">` (`:244`), add:

```html
    <!-- Read first by a screen reader, seen by no one (touch arc Task 0;
         static/js/screenreader.js keeps them current). iOS VoiceOver starts at
         the top of the page rather than at the focused command box. The
         status region speaks what xterm's own region cannot, such as "off". -->
    <p id="sr-hint" class="visually-hidden">Screen reader users: type screen reader on, then press Enter.</p>
    <div id="sr-status" class="visually-hidden" role="status" aria-live="polite"></div>
```

- [ ] **Step 3: The outer page**

Players reach the game at `/webclient`, which wraps `webclient-pure.html` in an iframe (`webclient.html:172`) under the site header, so a screen reader starts at the header. In `_datafiles/html/public/_header.html`, immediately after `<body>` (`:38`), add:

```html
{{ if or (eq .REQUEST.URL.Path "/webclient") (eq .REQUEST.URL.Path "/webclient/") (eq .REQUEST.URL.Path "/webclient.html") }}<p class="visually-hidden">Screen reader users: the game is below. In its command box, type screen reader on, then press Enter.</p>{{ end }}
```

Add to `static/css/gomud.css` (it has no such rule):

```css
/* Text for screen readers only (same rule as dashboard.css). */
.visually-hidden {
  position: absolute !important;
  width: 1px; height: 1px;
  padding: 0; margin: -1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}
```

- [ ] **Step 4: The overlay keeps out of the mouse's way**

xterm's `.xterm-accessibility` overlay sits over the game text at z-index 10 (`xterm.css:143-152`), which would stop mouse selection while the mode is on. After the `.visually-hidden` rule in `dashboard.css` (`:1959-1968`), add:

```css
/* xterm's screen reader overlay (static/js/screenreader.js) must not take
   the mouse: selecting game text works as before while the mode is on. */
.xterm .xterm-accessibility { pointer-events: none; }
```

- [ ] **Step 5: The spoken label**

On `#command-input` (`:310`), add `aria-label="Game commands. Screen reader users: type screen reader on, then press Enter."` beside the existing attributes. Change nothing else on the element.

- [ ] **Step 6: Create the switch, and report it with the width**

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
            termWrite: function (data, cb) { term.write(data, cb); },
            storage: (function () { try { return window.localStorage; } catch (e) { return null; } })(),
            input: document.getElementById('command-input'),
            hint: document.getElementById('sr-hint'),
            status: document.getElementById('sr-status'),
            report: function (on) { return SendGMCP('Client.Display', { screenreader: on }); },
            setCommand: function (on) { return SendData('set screenreader ' + (on ? 'on' : 'off')); },
            writeLine: function (text) {
                // Start on a fresh line: a login prompt leaves the cursor after
                // its text. (cursorX is not read: earlier writes may not be
                // parsed yet.)
                term.write('\r\n' + text);
            },
            currentLines: function () {
                // The waiting prompt: the last line of the game window with text on it.
                var buf = term.buffer.active;
                for (var y = buf.baseY + buf.cursorY; y >= 0; y--) {
                    var line = buf.getLine(y);
                    var text = line ? line.translateToString(true) : '';
                    if (text.trim()) return [text];
                }
                return [];
            },
            clearSpoken: function () {
                // xterm 4.19 resets its spoken region only on a key in its own
                // hidden input, which players never use. Private API, as
                // termfit.js already uses; xterm is pinned at 4.19 (#908), and
                // tools/jstest/screenreader.test.js fails if the names change.
                var am = term._core && term._core._accessibilityManager;
                if (!am || !am._clearLiveRegion) return;
                am._clearLiveRegion();
                // On a Mac (and iPadOS Safari) the reset also takes the region
                // out of the page; put it back now, or the next text is spoken
                // only if yet more text follows it.
                if (am._liveRegion && !am._liveRegion.parentNode && am._accessibilityTreeRoot) {
                    am._accessibilityTreeRoot.appendChild(am._liveRegion);
                }
            },
            inGame: function () { return inGame; }
        });
        screenReader.load();
```

(`screenReader` is used inside `send` only when a report goes out, after this runs. `inGame` is declared with `let` further down; `inGame()` is only called after the page has loaded.)

- [ ] **Step 7: Follow the account**

In the `"Char"` update handler, after the `companionGlue.hello` block (`:903-906`), add:

```js
                // The account's screen reader setting (Char.Info.screenreader):
                // xterm's mode follows it on every device.
                if ( obj.Info && typeof obj.Info.screenreader === 'boolean' ) {
                    screenReader.account(obj.Info.screenreader);
                }
```

- [ ] **Step 8: The end of an answer**

In the `"Char.Input.Done"` handler (`:1059-1064`), after `if (sendWait) sendWait.done(d.seq, d.menu === true);`, add:

```js
                // The answer to line d.seq is complete: once it is the last line
                // sent, back to the 20-line limit for unprompted output
                // (static/js/screenreader.js).
                screenReader.inputDone(d.seq);
```

- [ ] **Step 9: Forget the account on a new connection**

In `socket.onopen`, beside `if (companionGlue) { companionGlue.disconnected(); }` (`:3075`), add:

```js
                screenReader.loggedOut();
                // The last account's Info stays in GMCPStructs, and the generic
                // Char handler re-reads it for other Char packages: forget it,
                // so the new login's own Char.Info is the first value seen.
                if (GMCPStructs.Char) { delete GMCPStructs.Char.Info; }
```

- [ ] **Step 10: Server text goes through the switch**

Replace the server text write (`:3205`):

```js
                term.write(window.RoomTitleBold ? window.RoomTitleBold.apply(shown) : shown);
```

with:

```js
                // Through the screen reader switch (static/js/screenreader.js):
                // straight to xterm when the mode is off.
                screenReader.write(window.RoomTitleBold ? window.RoomTitleBold.apply(shown) : shown);
```

Leave the page's own local writes (`term.writeln` status lines, the reconnect dots) as they are.

- [ ] **Step 11: Catch the phrase; mark every sent line; report at connect**

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

In `function SendData(dataToSend)` (`:3380`), directly after `socket.send(dataToSend);` (NOT between `linesSent++;` and the send: `tools/jstest/hotinput.test.js:402-405` requires `linesSent++;` to be followed by `socket.send(dataToSend);`), add:

```js
            // Its answer is read in full, up to the server's Char.Input.Done
            // for this line (static/js/screenreader.js). Every line counts:
            // typed, macros, numpad moves, automation.
            screenReader.inputSent(linesSent);
```

Run `node tools/jstest/hotinput.test.js` after this edit; it must still pass unchanged.

In the connect code (the `connectButton.addEventListener('click', ...)` handler at `:3021`; there is no `connect()` function), change the websocket address (`:3040`):

```js
            var wsAddress = wsProto + location.host + '/ws?cols=' + encodeURIComponent(term.cols);
```

to:

```js
            // sr=1 tells the server before the connect splash that this player
            // asked for screen reader mode, so the splash comes as words only.
            var wsAddress = wsProto + location.host + '/ws?cols=' + encodeURIComponent(term.cols) +
                (screenReader.isOn() ? '&sr=1' : '');
```

- [ ] **Step 11a: The two page-source tests**

`tools/jstest/roomtitlebold.test.js:33-34` checks the exact server-text write. Change its regex to the new line:

```js
check('server text goes through it on the way to the terminal',
    /screenReader\.write\(window\.RoomTitleBold \? window\.RoomTitleBold\.apply\(shown\) : shown\);/.test(game), true);
```

`tools/jstest/webclient-connect.test.js` evaluates the page's real connect handler (`:82-89`) with stand-in globals, and the handler now calls `screenReader.isOn()`, `screenReader.loggedOut()` and `screenReader.write()`. Add a stand-in to the `code` prelude it builds, beside the other `var` lines:

```js
    + 'var screenReader = { isOn: function () { return global.__srOn === true; }, loggedOut: function () {}, write: function (t) { term.write(t); } };\n'
```

(use the same name the test already gives its fake terminal if it is not `term`; read the file first). Then add one check, following the file's own pattern for opening a connection and reading the address the fake `WebSocket` was given: with `global.__srOn = true` the address ends in `&sr=1`, and with it false the address has no `sr`. Run both tests: PASS.

- [ ] **Step 12: Run the JS gates**

Run (Git Bash, repo root):

```bash
for f in $(git ls-files '*.js' | grep -v -E 'xterm|vendor|node_modules'); do node --check "$f" || echo "SYNTAX $f"; done
for t in tools/jstest/*.test.js; do node "$t" >/dev/null || echo "FAIL $t"; done
for t in tools/webclient-tests/*.js; do node "$t" >/dev/null || echo "FAIL $t"; done
```

Expected: no `SYNTAX` or `FAIL` lines. (Compare with `.github/workflows/validate.yml`'s javascript job and run exactly what it runs if it differs.)

- [ ] **Step 13: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html _datafiles/html/public/_header.html _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/static/css/gomud.css tools/jstest/roomtitlebold.test.js tools/jstest/webclient-connect.test.js
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
- Modify: `internal/users/context.md` (`TurnOnScreenReader`, `ScreenReaderCombatLines`)
- Modify: `internal/term/context.md` (`ParseMTTS`, the `TelnetTermType*` commands, `TELNET_TTYPE_*`, `MTTSScreenReader`)
- Modify: `docs/README.md` (rows for the new files)

- [ ] **Step 1: Help row**

In `set.template`, inside the `::rows` block that holds `set charset`, after the `set charset` entry and its continuation line, add a blank line and:

```
  <ansi fg="command">set screenreader</ansi>  Turns screen reader mode on or off: plainer text without
                    art. Turning it on also sets combat text to light. In the
                    web client, type <ansi fg="command">screen reader on</ansi> to hear the game.
```

The continuation lines start at column 20, the entry's own text column (`  set screenreader  ` is 20 characters): `help_markup_guard_test.go:219-223` requires a continuation to start exactly at its entry's text column. Do not copy `set charset`'s 15. Then run the help guards:

Run: `go test ./internal/guards/ ./internal/templates/ ./internal/devtools/ -count=1`
Expected: PASS. If a width or markup guard fails, rewrap the row as it says; do not change the guard.

- [ ] **Step 1a: The Pothole Coulee tutorial accepts any level (owner ruling 2026-11-17, option 1)**

Quest 32's verbosity step moves on for any `set combatverbosity <level>` (`quests/32-first_blood.yaml:188-191`), but every line tells the player to type `full`, which would undo a blind player's light setting minutes into the game. Change only these strings (keep each YAML key, block style and indentation as it is; rewrap folded YAML at the file's existing width):

`_datafiles/world/dogmud/quests/32-first_blood.yaml`
The command must read as ONE command with its level ("type X then full" reads as two commands, and `set combatverbosity` alone only prints the options and does not move the step, `set.go:536-540`). Each hint keeps exactly one "type" outside a help pointer (`typedCount`, `internal/questengine/newbie_topic_steps_test.go:110`) and keeps the text "type set combatverbosity" (`newbie_martial_steps_test.go:152`).

- step `special`, `hint` (`:84-86`): `Combat verbosity sets how much fight text you see. Type set combatverbosity and a level: full, medium or light. With a screen reader, light keeps up best. For more, type help combatverbosity.`
- both `actor:` lines ending "type set combatverbosity full." (`:175-176`, `:185-186`): `That is a special move. Now choose how much combat detail you see: type set combatverbosity and a level, full, medium or light.`

`_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`
- opener variant gated on `32-special` (`:74-76`): `text: Last thing. Choose how much of a fight you see. Full is every blow, light is one line a round. If a screen reader reads to you, take light. Change it when you know what you are changing.` and `hints: Type set combatverbosity and a level: full, medium or light.`
- topic `progress_special` (`:227-229`): `text: Almost. Choose how much of a fight you see. Full shows everything. Start there, or on light if a screen reader reads to you.` and `hints: Type set combatverbosity and a level: full, medium or light.`
- topic `combat_help` (`:245-246`): replace `Start on full. Change it when you know what you are changing:` with `Start on full, or on light if a screen reader reads to you. Change it when you know what you are changing:` (the rest of the sentence stays). In the file that sentence breaks across two lines inside a single-quoted scalar (`Start on full. Change it when` / `    you know what you are changing:`): match the line break and the 4-space indent, and rewrap the new text at the scalar's existing width.

NPC `text` stays first person; `hints` stay narrator voice; no `;`, no dashes. Then validate (the `-dialogue` flag takes the folder named `dialogue`, and `-zones` picks the zone, `cmd/dialoguecheck/main.go:68-70, 124`):

```bash
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones pothole_coulee
go test ./internal/quests/ ./internal/questengine/ ./internal/dialogue/ ./internal/guards/ -count=1
```

Expected: VALIDATION 0, DEAD END 0, RE-OFFER 0, UNSTARTABLE 0 (ASSUMED lines are information, not errors); tests PASS.

Expected: no findings, PASS. The Task 9 boot check loads both files.

- [ ] **Step 2: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, under the title:

```markdown
## 2026-11-17: Screen readers in the web client

- The web client can now read the game aloud to screen reader users. When the page opens, your screen reader tells you what to do: type `screen reader on` in the command box and press Enter. The web client remembers this on that device. Type `screen reader off` to stop it.
- Screen reader mode is saved with your character. `set screenreader` turns it on or off wherever you play. If you turn it on in the web client, your character keeps it on.
- Turning screen reader mode on sets combat text to light, so your screen reader can keep up. `set combatverbosity full` brings every blow back.
- The combat lesson in Pothole Coulee now names full, medium or light, and suggests light to players who use a screen reader.
- New characters made in the web client are no longer asked whether they use a screen reader. Telnet clients that report their settings to the game (MTTS) are not asked either.
```

- [ ] **Step 3: Package docs**

- `modules/gmcp/context.md`: in the `gmcp.ClientDisplay.go` row, add that the payload may carry `screenreader` (bool), stored on `ClientSettings.ScreenReader` for websockets only, kept when absent or not a bool, and that the client sends it with every width report and alone when the player types the phrase. The file does not describe `Char.Info` yet (`grep -n "Char.Info" modules/gmcp/context.md` finds nothing): add a row or short paragraph where the `gmcp.Char.go` packages are listed, naming `Char.Info`'s fields `account`, `name`, `class`, `race`, `screenreader` (always sent, false included) and that `UserSettingChanged{Name: "screenreader"}` re-pushes it.
- `internal/connections/context.md`: the `ClientSettings` block (`:110-118`) is missing `AsciiMode` already; add it with `ScreenReader`, `MTTSReported`, `TermTypeReplies` and `TermTypeLast`, one line each.
- `internal/inputhandlers/context.md`: add a short section: the reserved `_conn_` result keys and `writeConnFacts`; `screen-reader-new` is asked only of telnet without MTTS; the login helpers `reportedScreenReaderLines`, `loginWithScreenReader` (before `LoginUser` publishes the user; a link-dead session goes to MainWorker as `events.ScreenReaderReported`) and `creationScreenReaderLines`, and `sendLoginLines`; the MTTS cycle in `term_iac.go` (`recordTermType`, `maxTermTypeReplies`, `maxTermTypeLen`).
- `internal/users/context.md`: add `TurnOnScreenReader` (the only way the setting turns on; saves light combat text), `TurnOnReportedScreenReader` (its form for a connection's report, returning the lines to send), `ScreenReaderCombatLines`, `ScreenReaderOnNotices` and `StoreAddressScreenReader` (the websocket address's `sr=1`).
- `internal/hooks/context.md` and `internal/events/context.md`: `ScreenReaderReported` and its listener `TurnOnReportedScreenReader`.
- `internal/term/context.md`: add `mtts.go`: `ParseMTTS`, `TelnetWillTermType`, `TelnetWontTermType`, `TelnetTermTypeSend`, `TelnetTermTypeIs`, `TELNET_TTYPE_IS`, `TELNET_TTYPE_SEND`, `MTTSScreenReader`.
- `internal/usercommands/context.md`: one line for `newcomerCombatVerbosity` in `start.go`.

Verify every symbol named exists:

Run: `python tools/context_md_audit.py`
Expected: no phantom symbols reported for the files above.

- [ ] **Step 4: docs/README.md rows for the new files**

Add a row for each new file, in the section where its neighbours are listed (find the section with `grep -n "internal/users/\|internal/term/\|static/js/\|tools/jstest/" docs/README.md`), in the existing row format:

- `internal/users/screenreader.go`: `TurnOnScreenReader`, the only way an account's screen reader setting turns on; it also saves light combat text, and `ScreenReaderCombatLines` says so.
- `internal/term/mtts.go`: terminal type subnegotiation and MTTS (`ParseMTTS`, bit 64 is a screen reader).
- `internal/hooks/ScreenReaderReported_TurnOn.go`: on MainWorker, turns on the screen reader setting of a link-dead session continued by a reconnect whose connection reported a screen reader.
- `_datafiles/world/dogmud/templates/login/connect-splash.screenreader.template`: the connect splash in plain words for a screen reader (no art, no rule, no spaced title), chosen by `sr=1` on the websocket address.
- `internal/inputhandlers/connect_splash_screenreader_test.go`: the screen reader splash is plain words at 30, 42 and 80.
- `_datafiles/html/public/static/js/screenreader.js`: the web client's screen reader switch: the spoken opt-in phrase, device memory, the account's setting, and resets of xterm's spoken region.
- `tools/jstest/screenreader.test.js`: not its own row. `tools/jstest/` is one row with dated "Added ..." notes in one cell (`docs/README.md:106`); add a dated note there.
- `internal/users/screenreader_test.go`, `internal/term/mtts_test.go`, `internal/inputhandlers/login_screenreader_test.go`, `internal/usercommands/start_screenreader_test.go`, `modules/gmcp/gmcp.CharInfo_screenreader_test.go`: one line each saying what they guard.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/set.template _datafiles/world/dogmud/quests/32-first_blood.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml docs/PATCH_NOTES.md modules/gmcp/context.md internal/connections/context.md internal/inputhandlers/context.md internal/users/context.md internal/term/context.md internal/usercommands/context.md internal/events/context.md internal/hooks/context.md docs/README.md
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

The game page is `/webclient-pure.html`; `/webclient` wraps it in an iframe under the site header (`webclient.html:172`). The check drives the inner page directly and then checks the outer page's hint. It reads three things: the whole terminal buffer (what was printed), xterm's `.live-region` (what was handed to the screen reader), and the frames sent.

Write this to `<scratchpad>/sr-check/check.js` and run `node <scratchpad>/sr-check/check.js http://127.0.0.1:<webport>`:

```js
// Headless check for touch arc Task 0.
const { chromium } = require('C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright');

const base = process.argv[2];
const fails = [];
function ok(name, cond, detail) {
    console.log((cond ? 'ok   ' : 'FAIL ') + name + (cond ? '' : '  ' + (detail || '')));
    if (!cond) fails.push(name);
}

// Everything printed, not just the visible rows.
function printed(page) {
    return page.evaluate(() => {
        const b = term.buffer.active, out = [];
        for (let i = 0; i < b.length; i++) { const l = b.getLine(i); if (l) out.push(l.translateToString(true)); }
        return out.join('\n');
    });
}
// What xterm handed to the screen reader since its last reset.
function spoken(page) {
    return page.evaluate(() => { const r = document.querySelector('.xterm .live-region'); return r ? r.textContent : null; });
}
async function waitPrinted(page, re, ms) {
    const end = Date.now() + (ms || 8000);
    while (Date.now() < end) {
        if (re.test(await printed(page))) return true;
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

    // ---- the outer page: its hint comes before the site header ----
    const outer = await (await browser.newContext()).newPage();
    await outer.goto(base + '/webclient');
    ok('outer: hint is first in body', await outer.evaluate(() => {
        const p = document.body.firstElementChild;
        return !!p && p.classList.contains('visually-hidden') && /type screen reader on/.test(p.textContent);
    }));
    await outer.goto(base + '/');
    ok('other site pages: no hint', await outer.evaluate(() => !/type screen reader on/.test(document.body.textContent)));

    // ---- the game page ----
    const page = await (await browser.newContext()).newPage();
    const sent = [];
    page.on('websocket', ws => ws.on('framesent', f => {
        sent.push(typeof f.payload === 'string' ? f.payload : Buffer.from(f.payload).toString('latin1'));
    }));
    await page.goto(base + '/webclient-pure.html');
    await page.waitForTimeout(2500);

    // Sighted: nothing runs, the label and hint are in place, the mouse still selects.
    ok('sighted: no accessibility overlay', await page.locator('.xterm-accessibility').count() === 0);
    ok('label names the phrase', /type screen reader on/.test(await page.getAttribute('#command-input', 'aria-label') || ''));
    ok('hint is first in body', await page.evaluate(() => document.body.firstElementChild && document.body.firstElementChild.id === 'sr-hint'));
    ok('hint is not visible', await page.evaluate(() => { const r = document.getElementById('sr-hint').getBoundingClientRect(); return r.width <= 1 && r.height <= 1; }));

    // Opt in at the name prompt.
    await type(page, 'Screen reader on.');
    await page.waitForTimeout(500);
    ok('opt-in: overlay exists', await page.locator('.xterm-accessibility').count() === 1);
    ok('opt-in: overlay ignores the mouse', await page.evaluate(() => getComputedStyle(document.querySelector('.xterm-accessibility')).pointerEvents === 'none'));
    ok('opt-in: label shortened', await page.getAttribute('#command-input', 'aria-label') === 'Game commands.');
    ok('opt-in: phrase never sent as text', !sent.some(s => /screen\s*reader/i.test(s) && !s.includes('Client.Display')), JSON.stringify(sent));
    ok('opt-in: reported', sent.some(s => s.includes('Client.Display') && s.includes('"screenreader":true')));
    ok('opt-in: remembered', await page.evaluate(() => localStorage.getItem('screenReader')) === 'true');
    await page.waitForTimeout(1200); // the live region fills on a short debounce
    const sp1 = await spoken(page) || '';
    ok('opt-in: confirmation spoken', /Screen reader mode is on\./.test(sp1), sp1);
    ok('opt-in: waiting prompt spoken again', /new/i.test(sp1), sp1);
    await page.press('#command-input', 'ArrowUp');
    ok('opt-in: phrase not in history', !/screen/i.test(await page.inputValue('#command-input')), await page.inputValue('#command-input'));
    await page.fill('#command-input', '');

    // Create a character; the question is never printed.
    const name = 'srcheck' + String(Date.now()).slice(-6);
    await type(page, 'new');
    ok('creation: name prompt printed', await waitPrinted(page, /login name/i));
    await type(page, name);
    ok('creation: password prompt printed', await waitPrinted(page, /Choose your password/));
    await type(page, 'Sr-check-pass-1');
    ok('creation: repeat prompt printed', await waitPrinted(page, /Repeat your password/));
    await type(page, 'Sr-check-pass-1');
    if (await waitPrinted(page, /Email Address/, 4000)) { await type(page, ''); }
    ok('creation: confirm prompt printed', await waitPrinted(page, /create a new user/i));
    await type(page, 'y');
    ok('creation: combat line printed', await waitPrinted(page, /Combat text is set to light/, 10000));
    ok('creation: screen reader question never printed', !/Are you using a screen reader/.test(await printed(page)));

    // Record everything handed to the screen reader from here on: the region
    // is reset between pieces, so its final text is only the last piece.
    await page.evaluate(() => {
        window.__spoken = [];
        const r = document.querySelector('.xterm .live-region');
        new MutationObserver(() => { if (r.textContent) window.__spoken.push(r.textContent); })
            .observe(r, { childList: true, characterData: true, subtree: true });
    });
    const spokenLog = () => page.evaluate(() => window.__spoken.join('\n'));

    // Speech keeps going past 20 lines: entering the world printed well over
    // 20, and a command afterwards must still be spoken.
    await page.waitForTimeout(3000);
    await page.evaluate(() => { window.__spoken = []; });
    await type(page, 'look');
    await page.waitForTimeout(2500);
    ok('past 20 lines: look is spoken (its exits line)', /exits?/i.test(await spokenLog()), await spokenLog());

    // An answer over 20 lines is read to its end (owner ruling, option 1).
    // help set is over 40 lines at 80 columns; its last line names help submission.
    // The line the cursor is on, counted from the top of the buffer: rows
    // below it are empty viewport rows that help text will fill.
    const before = await page.evaluate(() => { const b = term.buffer.active; return b.baseY + b.cursorY + 1; });
    await page.evaluate(() => { window.__spoken = []; });
    await type(page, 'help set');
    await page.waitForTimeout(4000);
    const helpPrinted = (await printed(page)).split('\n').slice(before).filter(l => l.trim());
    const helpLast = helpPrinted.filter(l => /help submission/.test(l)).pop() || '';
    const heard = await spokenLog();
    ok('long answer: over 20 lines printed by help set alone', helpPrinted.length > 20, helpPrinted.length);
    ok('long answer: its last line was spoken', helpLast !== '' && heard.indexOf(helpLast.trim()) >= 0, helpLast);
    ok('long answer: never cut off', !/Too much output/.test(heard), heard.slice(-300));

    // The account came back on in Char.Info; set screenreader turns it off everywhere.
    ok('in game: mode still on', await page.locator('.xterm-accessibility').count() === 1);
    await type(page, 'set screenreader');
    await page.waitForTimeout(1500);
    ok('set screenreader off: mode off', await page.locator('.xterm-accessibility').count() === 0);
    ok('set screenreader off: remembered off', await page.evaluate(() => localStorage.getItem('screenReader')) === 'false');

    // Reload with the setting remembered on: the first report already says so.
    await page.evaluate(() => localStorage.setItem('screenReader', 'true'));
    sent.length = 0;
    await page.reload();
    await page.waitForTimeout(2500);
    const firstDisplay = sent.find(s => s.includes('Client.Display'));
    ok('reload: first report carries screenreader true', !!firstDisplay && firstDisplay.includes('"screenreader":true'), firstDisplay);
    ok('reload: on before typing', await page.locator('.xterm-accessibility').count() === 1);
    ok('reload: the splash came as words only (no art row)', !/\*\s+\.\s+\*/.test(await printed(page)), (await printed(page)).slice(0, 400));
    const heardAtLogin = await page.evaluate(() => { const r = document.querySelector('.xterm .live-region'); return r ? r.textContent : ''; });
    ok('reload: the username prompt is spoken (a returning visit)', /Username|login/i.test(heardAtLogin), heardAtLogin.slice(-300));

    // Off before login: spoken by our own status region.
    await type(page, 'screen reader off');
    await page.waitForTimeout(300);
    ok('off: spoken by the status region', await page.evaluate(() => document.getElementById('sr-status').textContent) === 'Screen reader mode is off.');

    // ---- a Mac (and iPadOS Safari, which reports MacIntel) ----
    // xterm decides isMac at load from navigator.platform; on a Mac a reset
    // takes the spoken region out of the page. The page must put it back.
    const macCtx = await browser.newContext();
    await macCtx.addInitScript(() => {
        Object.defineProperty(Navigator.prototype, 'platform', { get: () => 'MacIntel' });
        try { localStorage.setItem('screenReader', 'true'); } catch (e) {}
    });
    const mac = await macCtx.newPage();
    await mac.goto(base + '/webclient-pure.html');
    await mac.waitForTimeout(3000);
    ok('mac: region still in the page after the login text', await mac.evaluate(() => !!document.querySelector('.xterm .xterm-accessibility .live-region')));
    const macHeard = await mac.evaluate(() => { const r = document.querySelector('.xterm .live-region'); return r ? r.textContent : ''; });
    ok('mac: the username prompt is spoken', /Username|login/i.test(macHeard), macHeard.slice(-300));

    await browser.close();
    console.log(fails.length ? fails.length + ' failure(s)' : 'all passed');
    process.exit(fails.length ? 1 : 0);
})();
```

If a check fails, read the detail it prints. Fix a wrong regex or wait in the check; fix a real defect in the game. Never weaken an assertion to make it pass. Expected: `all passed`.

- [ ] **Step 3: Telnet check**

With the test server still up, connect a raw telnet session from a scratch script that answers `IAC DO TTYPE` with `IAC WILL TTYPE`, then each `SEND` with `MUDLET`, `ANSI-TRUECOLOR`, `MTTS 73`; create a character; confirm the screen reader question is not asked, that the player hears "Screen reader mode is on for this character.", that `set` shows ScreenReader ON, and that `set combatverbosity` (no argument) says light. Repeat without answering TTYPE and confirm the question is asked. Then the link-dead path: log a character in on plain telnet (no TTYPE) with the setting off, cut the socket without `quit` so the session goes link-dead, reconnect with the MTTS-answering script and log in again: the player must hear "Screen reader mode is on for this character." and the two combat lines, and `set` must show ScreenReader ON. Record the three transcripts in the scratchpad.

- [ ] **Step 4: Gates**

In this order (`dogmud-shipping` "Pre-push gate order"):

```bash
gofmt -l internal/ modules/ main.go          # prints nothing
go build ./... && go test ./... -count=1     # all pass
~/go/bin/golangci-lint run --new-from-merge-base=origin/master   # 0 issues
```

Plus the JS loop from Task 7 Step 12. Kill the test server by its PID.

- [ ] **Step 5: Owner test (Windows Narrator)**

Hand the owner a short punchlist, player steps only: start Narrator (Ctrl+Win+Enter), open the local web client, hear the instruction, type `screen reader on`, create a character by ear, hear that combat text is light, type `help set` and hear it read to the last line, type `help set` again while standing where other players or creatures are talking or moving, fight the first rat in the Pothole Coulee, then close the tab, open the web client again and hear the name prompt without typing anything, and finally `set screenreader` and hear it stop. Ask specifically: was any part of `help set` cut off or skipped mid-sentence, and did new output arriving during it interrupt the reading? That is the unverified risk: xterm's spoken region is assertive, so new text may interrupt a long read, and the 2 second quiet reset may clear it while the reader is still speaking. Wait for his result.

Add to #834 (Malia's device test), as a comment on the issue: test VoiceOver on a Mac or an iPad with Safari as well as the iPhone. On those, xterm takes its spoken region out of the page at every reset and the page puts it back; the headless Mac pass proves the region is back in the page, not that VoiceOver speaks text added to a region re-inserted while empty. Unverified until then.

- [ ] **Step 6: PR**

Push the branch and open the PR with `gh pr create --repo pruuk/DOGMud --base master`. Body: what players get, the rulings, the tests and checks with their results, "Device test on iPhone VoiceOver and Android TalkBack is #834 (Malia)", and "Part of #733" (never "closes #733": the issue stays open for the rest of its questions). End with the attribution line.
