# Web Client Base, Plan 1: Server Wrap Width Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the server one source of a wrap width per reader (`UserRecord.WrapWidth`), accept the web client's column report over GMCP, and make every wrap in Go read that width, with no client sending a width yet.

**Architecture:** `WrapWidth` in `internal/users` is the only source of a width. A new GMCP `Client.Display` message stores a clamped column count on the websocket connection, on MainWorker, exactly as `Zone.Map.Resync` defers. Text sent to a whole room leaves its senders unwrapped and is wrapped per reader in the pipeline; text sent to one known player keeps its wrap call and passes that player's width. Two guards in `internal/guards` hold the result.

**Tech Stack:** Go 1.25 (builtin `min`/`max`), `testify`, `go/ast` guards, GMCP over websocket.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, section 1 of the Design, the Plans table (plan 1), Testing and guards. Part of #783.

---

## Facts verified against source

Read 2026-11-09 from master at `be17c3cbe`. `origin/master` is `572348425`; its diff against `be17c3cbe` is 9 files (images, two HTML pages, `PATCH_NOTES.md`, two guides) and no Go file, so every line number below holds on it. Everything in the plan's code was also applied in a detached worktree of `572348425` and run: the full suite (`go test . ./internal/... ./modules/...`) was green with it, and each sabotage step below was run and went red as written.

| # | Fact | Where |
|---|---|---|
| 1 | `func (u *UserRecord) GetLineWidth() int` returns 80 when `u == nil \|\| u.LineWidth <= 0` | `internal/users/userrecord.go:543-548` |
| 2 | `func (u *UserRecord) ClientSettings() connections.ClientSettings` reads `connections.GetClientSettings(u.connectionId)` | `internal/users/userrecord.go:215-217` |
| 3 | `func IsWebsocket(id ConnectionId) bool`; `type ConnectionId = uint64` | `internal/connections/connections.go:82`, `:21` |
| 4 | `GetClientSettings` and `OverwriteClientSettings` | `internal/connections/connections.go:342`, `:353` |
| 5 | `RegisterTestConnection` builds no `wsConn`; `IsWebSocket` is `cd.wsConn != nil`; `websocket` is already imported by `connections.go` for `Add` | `internal/connections/connections.go:363-375`, `:50`; `connectiondetails.go:251-252` |
| 6 | A real connection starts at `ScreenWidth: 80`; `GetScreenWidth` reads 0 as 80 | `connectiondetails.go:627`; `clientsettings.go:26-32` |
| 7 | NAWS writes through `GetClientSettings` / `OverwriteClientSettings`. The ANSI screen-size branch is guarded `if err != nil`, so it never stores on success | `internal/inputhandlers/term_iac.go:152-155`; `term_ansi.go:43-48` |
| 8 | The GMCP `switch command`, the `Zone.Map.Resync` case, and the payload-aliases-the-buffer comment | `modules/gmcp/gmcp.go:298`, `:422-423`, `:430-431` |
| 9 | `GMCPZoneResync` type, listener registration, handler | `modules/gmcp/gmcp.Zone.go:71-78`, `:63`, `:84-100` |
| 10 | `gmcpFrame(body string) []byte` test helper | `modules/gmcp/gmcp.Relay_test.go:16` |
| 11 | 8 production `RenderInput` width sites read `GetLineWidth` | `internal/users/userrecord.go:512`; `internal/rooms/rooms.go:271,346,464,538,551,587,621` |
| 12 | `shouldWrap` and its comment; 48 categories admitted, pinned by count and by map | `internal/messaging/pipeline.go:115-170`; `pipeline_test.go:57-121`, `:141`, `:150-163` |
| 13 | `Category` has 69 named values before `categoryMax`; `String()` ends at `CategoryScreen` | `internal/messaging/messaging.go:22-168`, `:311-312` |
| 14 | 39 non-test calls to `SplitStringNL` or `WrapAnsi` (grep, reconciled with the spec); `util.SplitString` adds one more at `roomdetails.go:639` | grep of `internal/` and `modules/` |
| 15 | No Speech, Shout or Emote send is pre-wrapped beyond the sites in Task 5. Checked by reading all 39 calls and every `Category(Speech\|Shout\|Emote)` send. The grep found the known site `shout.go:53`, so it could match | grep |
| 16 | `sendRoomIdleLine` sends on `CategoryRoomDescription`, which `shouldWrap` excludes | `internal/hooks/NewRound_RoomIdle.go:39`; `pipeline.go:122` |
| 17 | 18 non-test `GetLineWidth()` calls: the 8 in row 11, plus `roster.go:24,42`, `arrest.go:416`, `start.go:110`, `quests.go:151`, `motd.go:34`, `mutations.go:31`, `inbox.go:32`, `Quest_HandleQuestUpdate.go:195`, `set.go:518` | grep |
| 18 | `func Render(m Menu) string`; 50 non-test `menu.Render(` calls in 15 files, every one with a known viewer (47 `user`, 3 `u` inside `userrecord.menu.go`) | `internal/menu/menu.go:76`; grep |
| 19 | `GetDetails(r, user, tinymap ...)` dereferences `user` at once, so the viewer is never nil | `internal/rooms/roomdetails.go:43`, `:66` |
| 20 | `GetDescriptionFormatted(lineSplit int, ...)` ignores `lineSplit`; its only callers are tests | `internal/rooms/rooms.go:2074-2076`; `rooms_test.go:1424,1433,2155` |
| 21 | `setScreenWidth` const and its 3 uses, plus 2 uses in a test | `internal/usercommands/set.go:77,119,134,283`; `set_blank_lines_test.go:64-65` |
| 22 | Registry keys holding the literal source text, and the two comments quoting it. The literal is capped at 80 runes; both new keys fit (79 and 77) | `internal/guards/messaging_surface_guard_test.go:1416,1445,777,803,751` |
| 23 | `TestPlayerConditionsTravelTheEventPath` is a line-number allowlist, with keys in both `internal/actions/drink.go` (`\|400`, `\|409`) and `internal/actions/actor_user.go` (`\|79`, `\|86`). Adding one line above them in either file fails it (measured, both) | `internal/guards/condition_apply_path_guard_test.go:369` |
| 24 | `MobActor.SendText` is a no-op and `GetUserId` is 0; `users.GetByUserId` returns nil for an unknown id and `GetLineWidth` is nil-safe | `internal/actions/actor_mob.go:51,64-66`; `internal/users/users.go:179` |
| 25 | Exempt sites read: `lightnotice/store.go:63` and `darkcombat/store.go:57` are authoring limits; `online.go:62` is the admin table's column budget; `sight_aim.go:41` is a length check on a hint line. None reaches a wrap function. `setdesc.go:37` (72) and `admin.server.go:92` (`lineLength := 158 - longestKey`, `:74`) do | files named |
| 26 | `Room.SendTextCommunication` queues one unrendered `RoomId` event. Two callers: a player's next-door shout and the companion's `ask` line. Delivered with no wrap | `internal/rooms/rooms.go:243-253`; `actions/shout.go:74`; `modules/aicompanion/listeners.go:423`; `internal/hooks/Message_SendMessages.go:96` |
| 27 | 11 templates pass a literal width to the `splitstring` template function. Those are plan 3's | `internal/templates/templatesfunctions.go:131`; grep of `_datafiles/world/dogmud/templates` |
| 28 | `Display.GetScreenWidth` is also read by `skill.map.go:144`, `admin.mob.go:171`, `admin.item.go:111` for column layouts. They follow a report on their own and need no change | grep |
| 29 | Measured over 10,925 shipped prose lines (with a `Kesh says, "` prefix) at width 80: 745 break differently between `SplitStringNL` and `WrapAnsi` | scratch test in a detached worktree, removed |
| 30 | `SplitString` merges a following punctuation token into the word (`, ` counts its trailing space) and appends an over-long word to the result before flushing the current line; `WrapAnsi` breaks only at a space or newline | `internal/util/util.go:256-267`; `internal/messaging/wrap.go:145` |

### What the telnet golden test can and cannot claim

The spec says visible text and line breaks at 80 are unchanged. Row 29 says otherwise, so the golden test pins two things separately. Lines whose breaks land away from a boundary case must match exactly. The boundary cases are named, one test row each, with both wrappers' output written out:

1. A word over 80 columns: `SplitString` moves it ahead of its own sentence and leaves a stray leading space.
2. A hyphenated word at the edge: `SplitString` breaks at the hyphen; `WrapAnsi` moves it whole.
3. Punctuation ending exactly at column 80: `SplitString` counts the trailing space and wraps a word early.
4. A closing quote at the edge: `SplitString` strands the quote on its own line.
5. A run of spaces: `WrapAnsi` collapses it to one.

In all five the new wrap is the better one, and no word is lost.

### Decisions this plan makes

- **Room idle lines get a new `CategoryRoomIdle`.** The pipeline does not wrap `CategoryRoomDescription` (it carries the side-by-side minimap block), so the pre-wrap cannot simply go. The new category is colored and normalised exactly as `room-description` is, the precedent being `CategoryCraftSeen`.
- **Every `GetLineWidth` read moves to `WrapWidth`**, not only the 8 pipeline sites. `roster.go`, `arrest.go`, `start.go`, `quests.go`, `motd.go`, `mutations.go`, `inbox.go` and the quest reward line would otherwise stay at 80 on a 40-column screen. Only `set linewidth`'s own status line keeps reading the saved setting.
- **`FormatEmoteText` is the one emote body; `FormatMobEmoteText` delegates to it.** Once neither wraps they are the same line. `emoteLine` is removed.
- **`actorWrapWidth` lives in `roster.go`,** beside the other per-user wraps, and `users` is not imported into `drink.go`, so fact 23's line numbers do not move. The two wrap lines in `drink.go` keep their line count.
- **A next-door player shout wraps in the listener.** The mob's next-door shout rides `CategoryShout` and now wraps; leaving the player's sibling line unwrapped would be an inconsistency this work created. It is wrapped per reader in `Message_SendMessages.go`, with no change of color and the deafen filter intact.
- **`menu.Render` takes a required `width`,** so the compiler enumerates all 50 callers.
- **No viewer is unknowable anywhere in the sweep.** The quest reward Trio's Observer line goes through the pipeline. The only fallback to 80 is `actorWrapWidth` for a mob, whose `SendText` is a no-op.
- **The guard also covers `SplitString`, `wrapPrompt`, `wrapDescriptionParts` and the two private `wrap` helpers,** because a width passed to any of those is the same defect.

---

## File Structure

**Create**
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/users/wrapwidth.go`: `WrapWidth`, `MinWrapWidth`, `MaxAutoWrapWidth`.
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/users/wrapwidth_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/modules/gmcp/gmcp.ClientDisplay.go`: the event, listener and payload rules.
- `C:/Users/Calabe Davis/workspace/DOGMud/modules/gmcp/gmcp.ClientDisplay_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/actions/wrap_golden_test.go`: the telnet golden.
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/actions/wrap_per_recipient_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/actions/actor_wrap_width_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/hooks/room_chatter_wrap_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/rooms/roomdetails_wrap_width_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/usercommands/wrap_width_test.go`
- `C:/Users/Calabe Davis/workspace/DOGMud/internal/guards/wrap_width_guard_test.go`

**Modify:** named in each task. All `internal/...` and `modules/...` paths are relative to the worktree created in Task 1.

**Working rules for every task.** Edit files with the Edit and Write tools only: no `sed -i`, no Python, no `>` into a tracked file (`guard.py` H3). `git add` named paths only. Every task's gate includes `go test . ./internal/guards/`, because of fact 23. Test binaries load Go config defaults, not `config.yaml`; nothing here reads a balance knob, and `Display` and `LineWidth` are set directly in each fixture.

---

### Task 1: Worktree and baseline

**Files:** none changed.

- [ ] **Step 1: Create the branch in its own worktree**

The main checkout holds the owner's uncommitted spec and may back a running server.

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin master
git worktree add -b feature/webclient-base-1-wrap-width C:/tmp/dogmud-wrap-width origin/master
git -C C:/tmp/dogmud-wrap-width log --oneline -1
```

Expected: the last line shows the tip of `origin/master`.

- [ ] **Step 2: Confirm the baseline is green**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go test . ./internal/guards/ ./internal/messaging/ ./internal/menu/ 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: `ok` for all four packages. A red here is not this work's: stop and report it.

---

### Task 2: `WrapWidth` and a websocket test connection

**Files:**
- Create: `internal/users/wrapwidth.go`, `internal/users/wrapwidth_test.go`
- Modify: `internal/connections/connections.go:377`

- [ ] **Step 1: Write the failing test**

Create `internal/users/wrapwidth_test.go`:

```go
package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

func TestWrapWidth(t *testing.T) {
	const webConn, telnetConn uint64 = 97101, 97102
	connections.RegisterTestWebsocketConnection(webConn)
	connections.RegisterTestConnection(telnetConn)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(webConn)
		connections.UnregisterTestConnection(telnetConn)
	})

	tests := []struct {
		name      string
		conn      uint64
		cols      uint32 // Display.ScreenWidth; 0 is "never reported"
		lineWidth int    // UserRecord.LineWidth; 0 is "unset"
		want      int
	}{
		{"telnet, nothing saved", telnetConn, 0, 0, 80},
		{"telnet, saved width above 80 still works", telnetConn, 0, 120, 120},
		{"telnet, saved narrow width", telnetConn, 0, 40, 40},
		{"telnet, a screen size report is not a wrap width", telnetConn, 132, 0, 80},
		{"web, nothing reported or saved", webConn, 0, 0, 80},
		{"web, narrow report", webConn, 42, 0, 42},
		{"web, saved width narrows a wider report", webConn, 60, 40, 40},
		{"web, saved width cannot widen a report", webConn, 42, 120, 42},
		{"web, saved 100 is capped at 80", webConn, 80, 100, 80},
		{"web, report below the floor", webConn, 12, 0, 30},
		{"web, saved width below the floor", webConn, 60, 20, 30},
		{"web, report above the cap", webConn, 200, 0, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := connections.GetClientSettings(tt.conn)
			cs.Display.ScreenWidth = tt.cols
			connections.OverwriteClientSettings(tt.conn, cs)

			u := NewTestUser(9710, "wrapper", "Wrapper", tt.conn)
			u.LineWidth = tt.lineWidth
			require.Equal(t, tt.want, u.WrapWidth())
		})
	}
}

func TestWrapWidth_NilAndUnknownConnection(t *testing.T) {
	var nobody *UserRecord
	require.Equal(t, 80, nobody.WrapWidth(), "a nil user gets the default, as GetLineWidth does")

	gone := NewTestUser(9711, "gone", "Gone", 97199)
	gone.LineWidth = 55
	require.Equal(t, 55, gone.WrapWidth(), "a connection that is not registered is not a websocket")
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/users/ -run 'TestWrapWidth'
```

Expected: build failure, `undefined: connections.RegisterTestWebsocketConnection` and `u.WrapWidth undefined`.

- [ ] **Step 3: Add the websocket test helper**

In `internal/connections/connections.go`, Edit:

old:
```go
// UnregisterTestConnection removes a test connection (for cleanup)
```
new:
```go
// RegisterTestWebsocketConnection registers a mock websocket connection for
// testing purposes: IsWebSocket reports true because wsConn is set. The
// websocket underneath is an empty value that was never dialled, so a test
// must not write to it or close it; remove it with UnregisterTestConnection.
func RegisterTestWebsocketConnection(id ConnectionId) {
	lock.Lock()
	defer lock.Unlock()

	if id == 0 {
		id = 1
	}

	netConnections[id] = &ConnectionDetails{
		connectionId:   id,
		wsConn:         &websocket.Conn{},
		clientSettings: ClientSettings{},
	}
}

// UnregisterTestConnection removes a test connection (for cleanup)
```

- [ ] **Step 4: Implement `WrapWidth`**

Create `internal/users/wrapwidth.go`:

```go
package users

import "github.com/GoMudEngine/GoMud/internal/connections"

const (
	// MinWrapWidth is the narrowest a web session wraps. WCAG 2.2 AA reflow at
	// 320px needs 30 columns.
	MinWrapWidth = 30
	// MaxAutoWrapWidth is the widest a web session wraps: the game window
	// shows at most 80 columns.
	MaxAutoWrapWidth = 80
)

// WrapWidth is the column width text for this user wraps at, and the only
// source of one: every send and every layout that wraps reads it.
//
// A web session wraps at the smallest of the columns its client reported, 80,
// and the saved LineWidth when one is set, and never below MinWrapWidth, so
// `set linewidth` can only narrow it. The reported value is clamped again
// here, not only where it arrives, because nothing else stops a websocket
// client sending a raw telnet screen-size report. Every other session, and a
// nil receiver, gets GetLineWidth.
func (u *UserRecord) WrapWidth() int {
	if u == nil || !connections.IsWebsocket(u.connectionId) {
		return u.GetLineWidth()
	}
	width := min(u.ClientSettings().Display.GetScreenWidth(), MaxAutoWrapWidth)
	if u.LineWidth > 0 {
		width = min(width, u.LineWidth)
	}
	return max(width, MinWrapWidth)
}
```

- [ ] **Step 5: Run the tests**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/users/ -run 'TestWrapWidth' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: `--- PASS: TestWrapWidth`, `--- PASS: TestWrapWidth_NilAndUnknownConnection`, `ok`.

- [ ] **Step 6: Prove the table can fail**

Edit `internal/users/wrapwidth.go`, changing `return max(width, MinWrapWidth)` to `return width`. Run:

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/users/ -run 'TestWrapWidth$' 2>&1 | grep -E "^\s*--- FAIL"
```

Expected: `TestWrapWidth/web,_report_below_the_floor` and `TestWrapWidth/web,_saved_width_below_the_floor` fail. Undo that one line with Edit and rerun Step 5.

- [ ] **Step 7: Commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test . ./internal/guards/ ./internal/connections/ ./internal/users/ 2>&1 | grep -E "^(ok|FAIL)"
git add internal/users/wrapwidth.go internal/users/wrapwidth_test.go internal/connections/connections.go
git commit -F - <<'EOF'
feat(users): WrapWidth, the one source of a wrap width

A web session wraps at the smallest of its reported columns, 80 and the
saved LineWidth, floored at 30. Everything else is GetLineWidth. Nothing
reads it yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` for all four packages before the commit.

---

### Task 3: GMCP `Client.Display`

**Files:**
- Create: `modules/gmcp/gmcp.ClientDisplay.go`, `modules/gmcp/gmcp.ClientDisplay_test.go`
- Modify: `modules/gmcp/gmcp.go:423`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.ClientDisplay_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/stretchr/testify/require"
)

// clientDisplayConn registers a websocket connection whose stored screen width
// is start, and returns its id.
func clientDisplayConn(t *testing.T, start uint32) uint64 {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the handler logs what it ignores
	const id uint64 = 97201
	connections.RegisterTestWebsocketConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	cs := connections.GetClientSettings(id)
	cs.Display.ScreenWidth = start
	connections.OverwriteClientSettings(id, cs)
	return id
}

func storedWidth(id uint64) uint32 {
	return connections.GetClientSettings(id).Display.ScreenWidth
}

func reportCols(id uint64, payload string) events.ListenerReturn {
	return clientDisplayHandler(GMCPClientDisplay{ConnectionId: id, Payload: []byte(payload)})
}

// HandleIAC runs on the connection goroutine, so it only queues the report,
// and the queued payload is a copy: the read buffer is reused after it returns.
func TestHandleIAC_ClientDisplayIsQueuedWithACopiedPayload(t *testing.T) {
	mudlog.SetupLogger(nil, ``, ``, false) // HandleIAC logs each command
	events.DrainQueuedByTypeForTest(`GMCPClientDisplay`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPClientDisplay`) })

	frame := gmcpFrame(`Client.Display {"cols":42}`)
	require.True(t, gmcpModule.HandleIAC(97202, frame))
	for i := range frame {
		frame[i] = 'x' // the read loop reuses this buffer
	}

	queued := events.DrainQueuedByTypeForTest(`GMCPClientDisplay`)
	require.Len(t, queued, 1)
	got := queued[0].(GMCPClientDisplay)
	require.Equal(t, uint64(97202), got.ConnectionId)
	require.Equal(t, `{"cols":42}`, string(got.Payload))
}

func TestClientDisplay_StoresAWholeNumberInRange(t *testing.T) {
	id := clientDisplayConn(t, 80)
	require.Equal(t, events.Continue, reportCols(id, `{"cols":42}`))
	require.Equal(t, uint32(42), storedWidth(id))
}

func TestClientDisplay_ClampsOutOfRange(t *testing.T) {
	for _, tt := range []struct {
		payload string
		want    uint32
	}{
		{`{"cols":5}`, 30},
		{`{"cols":0}`, 30},
		{`{"cols":-7}`, 30},
		{`{"cols":30}`, 30},
		{`{"cols":80}`, 80},
		{`{"cols":81}`, 80},
		{`{"cols":5000}`, 80},
	} {
		id := clientDisplayConn(t, 55)
		reportCols(id, tt.payload)
		require.Equal(t, tt.want, storedWidth(id), tt.payload)
	}
}

// Everything that is not a whole number leaves the stored width alone.
func TestClientDisplay_IgnoresABadPayload(t *testing.T) {
	for _, payload := range []string{
		``,
		`not json`,
		`{"cols":`,
		`{}`,
		`{"cols":null}`,
		`{"cols":"42"}`,
		`{"cols":42.5}`,
		`{"cols":[42]}`,
		`{"cols":99999999999999999999}`,
		`[42]`,
	} {
		id := clientDisplayConn(t, 55)
		require.Equal(t, events.Continue, reportCols(id, payload), payload)
		require.Equal(t, uint32(55), storedWidth(id), "payload %q changed the width", payload)
	}
}

func TestClientDisplay_IgnoresANonWebsocketConnection(t *testing.T) {
	mudlog.SetupLogger(nil, ``, ``, false)
	const id uint64 = 97203
	connections.RegisterTestConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	cs := connections.GetClientSettings(id)
	cs.Display.ScreenWidth = 132
	connections.OverwriteClientSettings(id, cs)

	reportCols(id, `{"cols":42}`)

	require.Equal(t, uint32(132), storedWidth(id), "a telnet client's width comes from NAWS only")
}

// A report equal to the effective width writes nothing. The stored 0 reads as
// 80 through GetScreenWidth, so a write of 80 would be visible here.
func TestClientDisplay_UnchangedValueWritesNothing(t *testing.T) {
	id := clientDisplayConn(t, 0)
	reportCols(id, `{"cols":80}`)
	require.Equal(t, uint32(0), storedWidth(id))
}

// The handler reaches the width through the event bus, or the feature is dead
// however green the direct calls are.
func TestClientDisplay_ListenerIsRegistered(t *testing.T) {
	id := clientDisplayConn(t, 80)
	t.Cleanup(events.SwapListenersForTest())
	registerClientDisplayListener()

	events.DoListeners(GMCPClientDisplay{ConnectionId: id, Payload: []byte(`{"cols":61}`)})

	require.Equal(t, uint32(61), storedWidth(id))
}
```

- [ ] **Step 2: Run them to verify they fail**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./modules/gmcp/ -run 'ClientDisplay'
```

Expected: build failure, `undefined: clientDisplayHandler`, `undefined: GMCPClientDisplay`, `undefined: registerClientDisplayListener`.

- [ ] **Step 3: Write the handler**

Create `modules/gmcp/gmcp.ClientDisplay.go`:

```go
package gmcp

import (
	"encoding/json"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	registerClientDisplayListener()
}

// registerClientDisplayListener wires the handler to the event bus. Split from
// init so a test can register it against a swapped bus and drive it through
// DoListeners.
func registerClientDisplayListener() {
	events.RegisterListener(GMCPClientDisplay{}, clientDisplayHandler)
}

// GMCPClientDisplay carries a client's Client.Display report from the
// connection goroutine (HandleIAC) to MainWorker, where clientDisplayHandler
// judges it and stores the width on the connection.
type GMCPClientDisplay struct {
	ConnectionId uint64
	Payload      []byte
}

func (g GMCPClientDisplay) Type() string { return `GMCPClientDisplay` }

// clientDisplayHandler stores a web client's reported column count. The
// payload comes from the client: only a websocket connection is listened to,
// only a whole number counts, the value is clamped before it is stored, and
// it is written only when it differs. A client can change only its own
// connection, so a phone never changes the player's desktop or saved setting.
func clientDisplayHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(GMCPClientDisplay)
	if !ok {
		mudlog.Error("Event", "Expected Type", "GMCPClientDisplay", "Actual Type", e.Type())
		return events.Cancel
	}
	if !connections.IsWebsocket(evt.ConnectionId) {
		return events.Continue
	}
	cols, ok := reportedCols(evt.Payload)
	if !ok {
		mudlog.Debug("gmcp.ClientDisplay", "connectionId", evt.ConnectionId, "ignored", "payload is not a whole column count")
		return events.Continue
	}
	width := min(max(cols, users.MinWrapWidth), users.MaxAutoWrapWidth)

	cs := connections.GetClientSettings(evt.ConnectionId)
	if cs.Display.GetScreenWidth() == width {
		return events.Continue
	}
	cs.Display.ScreenWidth = uint32(width)
	connections.OverwriteClientSettings(evt.ConnectionId, cs)
	return events.Continue
}

// reportedCols reads {"cols": n}. A fraction, a string, a null, a missing key
// and malformed JSON all fail to decode into the *int and are refused.
func reportedCols(payload []byte) (int, bool) {
	var decoded struct {
		Cols *int `json:"cols"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.Cols == nil {
		return 0, false
	}
	return *decoded.Cols, true
}
```

- [ ] **Step 4: Add the switch case**

In `modules/gmcp/gmcp.go`, Edit:

old:
```go
			events.AddToQueue(GMCPZoneResync{ConnectionId: connectionId})

```
new:
```go
			events.AddToQueue(GMCPZoneResync{ConnectionId: connectionId})

		// ---- web client text width (web client base, plan 1) ----
		// The client's column count, after it settles. The width is read off
		// the connection by the wrap sweep and the payload is untrusted, so
		// defer to MainWorker like the ops above; see gmcp.ClientDisplay.go.
		// Copy the payload: it aliases the IAC read buffer, which is reused
		// after HandleIAC returns.
		case `Client.Display`:
			events.AddToQueue(GMCPClientDisplay{
				ConnectionId: connectionId,
				Payload:      append([]byte(nil), payload...),
			})

```

- [ ] **Step 5: Run the tests**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./modules/gmcp/ -run 'ClientDisplay' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: 7 `--- PASS` lines (`TestHandleIAC_ClientDisplayIsQueuedWithACopiedPayload` and the six `TestClientDisplay_` tests), then `ok`.

- [ ] **Step 6: Prove the guards can fail**

Using Edit, delete the three lines of `if !connections.IsWebsocket(evt.ConnectionId) { ... }` and the three lines of `if cs.Display.GetScreenWidth() == width { ... }`. Run:

```bash
cd C:/tmp/dogmud-wrap-width && go test ./modules/gmcp/ -run 'ClientDisplay' 2>&1 | grep -E "^--- FAIL"
```

Expected: `TestClientDisplay_IgnoresANonWebsocketConnection` and `TestClientDisplay_UnchangedValueWritesNothing` fail. Put both blocks back with Edit and rerun Step 5.

- [ ] **Step 7: Gate and commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test . ./internal/guards/ ./modules/gmcp/ 2>&1 | grep -E "^(ok|FAIL)"
git add modules/gmcp/gmcp.ClientDisplay.go modules/gmcp/gmcp.ClientDisplay_test.go modules/gmcp/gmcp.go
git commit -F - <<'EOF'
feat(gmcp): Client.Display, a web client's column report

Deferred to MainWorker the way Zone.Map.Resync is. The value is untrusted:
websocket only, whole numbers only, clamped to 30..80, written only when it
changes. No client sends it yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` for all three.

---

### Task 4: The pipeline reads `WrapWidth`

**Files:**
- Modify: `internal/users/userrecord.go:512`, `internal/rooms/rooms.go:271,346,464,538,551,587,621`, `internal/users/wrapwidth_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/users/wrapwidth_test.go`, Edit:

old:
```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)
```
new:
```go
import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// The pipeline reads WrapWidth: a narrow web reader's narration folds there.
func TestSendText_WrapsAtWrapWidth(t *testing.T) {
	const webConn uint64 = 97103
	connections.RegisterTestWebsocketConnection(webConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(webConn) })
	cs := connections.GetClientSettings(webConn)
	cs.Display.ScreenWidth = 40
	connections.OverwriteClientSettings(webConn, cs)
	u := NewTestUser(9712, "reader", "Reader", webConn)
	events.DrainQueuedMessagesForTest(u.UserId)

	u.SendText(messaging.CategoryTip, "Healing up slow? Some rooms are sanctuaries, temples and certain camps, and they regenerate your health much faster than ordinary rooms do.")

	msgs := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, msgs, 1)
	lines := strings.Split(strings.TrimRight(msgs[0], "\n"), "\n")
	require.Greater(t, len(lines), 2)
	for _, line := range lines {
		require.LessOrEqual(t, util.VisibleWidth(line), 40, "%q", line)
	}
}
```

Run it:

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/users/ -run 'TestSendText_WrapsAtWrapWidth'
```

Expected: `--- FAIL: TestSendText_WrapsAtWrapWidth`, with `"2" is not greater than "2"` (at 80 columns the tip takes two lines). The text must not repeat a word, because the normalizer collapses duplicates on this category.

- [ ] **Step 2: Change the 8 sites**

In `internal/users/userrecord.go`, Edit:

old: `		LineWidth: u.GetLineWidth(),`
new: `		LineWidth: u.WrapWidth(),`

In `internal/rooms/rooms.go`, Edit with `replace_all`:

old: `u.GetLineWidth()`
new: `u.WrapWidth()`

Expected: 7 replacements, confirmed by Step 3.

- [ ] **Step 3: Count and test**

```bash
cd C:/tmp/dogmud-wrap-width && grep -c "u.WrapWidth()" internal/rooms/rooms.go
go test ./internal/users/ ./internal/rooms/ 2>&1 | grep -E "^(ok|FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: `7`, then `ok` for each package.

- [ ] **Step 4: Commit**

```bash
cd C:/tmp/dogmud-wrap-width
git add internal/users/userrecord.go internal/users/wrapwidth_test.go internal/rooms/rooms.go
git commit -F - <<'EOF'
refactor(messaging): every pipeline send reads WrapWidth

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Speech, Shout and Emote wrap per reader

All of this is one commit: admitting a category while a sender still pre-wraps it wraps twice (#430).

**Files:**
- Create: `internal/actions/wrap_golden_test.go`, `internal/actions/wrap_per_recipient_test.go`
- Modify: `internal/messaging/pipeline.go:127-130,157`, `internal/messaging/pipeline_test.go`, `internal/actions/say.go`, `shout.go`, `emote.go`, `actions_test.go:118-146`, `emote_wrap_test.go`, `internal/usercommands/say.go:46`, `shout.go:51`, `internal/conversation/talk.go:440-441`, `internal/mobcommands/sayto.go:21-32`, `sayto_sight_test.go:64-66`, `internal/hooks/Quest_HandleQuestUpdate.go:188-197`

- [ ] **Step 1: Write the golden test**

Create `internal/actions/wrap_golden_test.go`:

```go
package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Web client base, plan 1. Speech, Shout and Emote stopped pre-wrapping at 80
// and joined the pipeline, so a telnet reader at 80 columns now gets WrapAnsi's
// wrap where it got util.SplitStringNL's. The tests below compare what the
// reader sees, tags stripped and CR normalised, because the raw bytes differ
// on purpose: WrapAnsi closes and reopens color tags at each break.

var wrapGoldenTag = regexp.MustCompile(`</?ansi[^>]*>`)

// wrapGoldenVisible is the lines a reader sees.
func wrapGoldenVisible(s string) []string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.Split(wrapGoldenTag.ReplaceAllString(s, ""), "\n")
}

// preSweepLines is the wrap every sender applied before the sweep, pinned
// here as the reference.
func preSweepLines(line string) []string {
	return wrapGoldenVisible(util.SplitStringNL(line, 80))
}

// pipelineLines is the line as a telnet reader at 80 columns gets it now.
func pipelineLines(cat messaging.Category, channel messaging.Channel, line string) []string {
	return wrapGoldenVisible(messaging.RenderForRecipient(messaging.RenderInput{
		Category:      cat,
		Text:          line,
		Channel:       channel,
		SightDecision: messaging.SightFull,
		LineWidth:     80,
	}))
}

func wrapGoldenSay(text string) string {
	return FormatSayText("Kesh", text, "username", "saytext")
}

func wrapGoldenShout(text string) string {
	return formatShoutText("Kesh", text, "username", "yellow")
}

func wrapGoldenEmote(text string) string {
	return FormatEmoteText("Kesh", text, "username")
}

// wrapGoldenCases are lines a player types or a mob speaks, one to three
// lines long, whose breaks land away from the boundary cases listed in
// TestWrapGolden_AcceptedDifferences.
var wrapGoldenCases = []struct {
	name    string
	cat     messaging.Category
	channel messaging.Channel
	line    string
}{
	{"say short", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`Hello there.`)},
	{"say two lines", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`The pool answers first, it is older than the town and more patient than I am, so look into the water and wait.`)},
	{"say three lines", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`Old Fenn keeps the sluice, and Ayres holds the farm west of it, and neither has spoken to the other since the spring the river ran low and the barley failed on both banks of the water.`)},
	{"say player chatter", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`anyone want to group up for the ridge? i have a healer alt and about an hour before i need to log off for dinner`)},
	{"shout short", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`HELP ME`)},
	{"shout two lines", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`THE BRIDGE IS OUT AND THE RIVER IS RISING AND THE WHOLE EAST ROAD WILL BE UNDER WATER BY MORNING`)},
	{"shout three lines", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`TO ARMS TO ARMS THE RAIDERS HAVE COME OVER THE NORTH WALL AND THEY ARE IN THE MARKET AND THE GATE CAPTAIN IS DOWN AND SOMEONE MUST RING THE BELL AT ONCE`)},
	{"emote short", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`waves.`)},
	{"emote two lines", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`rolls a trinket across his knuckles and murmurs that he buys what others will not sell to anyone.`)},
	{"emote three lines", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`turns slowly to examine the ceiling with apparent fascination, one hand hanging loose at his side, and hums something that was almost a song before it lost its way somewhere around the second verse.`)},
}

func TestWrapGolden_VisibleLinesMatchTheOldWrapAt80(t *testing.T) {
	for _, tc := range wrapGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, preSweepLines(tc.line), pipelineLines(tc.cat, tc.channel, tc.line))
		})
	}
}

// Measured over 10,925 shipped prose lines on 2026-11-09, 745 broke
// differently: never a lost word, always the last word or two of a line. Each
// shape is pinned here on both sides, so a change to either wrapper shows up.
func TestWrapGolden_AcceptedDifferences(t *testing.T) {
	long := strings.Repeat("a", 90)
	for _, tc := range []struct {
		name    string
		line    string
		oldWrap []string
		newWrap []string
	}{
		{
			"a word over 80 no longer jumps ahead of its own sentence",
			wrapGoldenSay(long + ` then more words`),
			[]string{long, `Kesh says, " then more words"`},
			[]string{`Kesh says,`, `"` + long, `then more words"`},
		},
		{
			"a hyphenated word moves whole instead of breaking at the hyphen",
			wrapGoldenSay(`You have heard Fenn's side. Ayres holds the farm west of the head-gate, at the ford.`),
			[]string{`Kesh says, "You have heard Fenn's side. Ayres holds the farm west of the head-`, `gate, at the ford."`},
			[]string{`Kesh says, "You have heard Fenn's side. Ayres holds the farm west of the`, `head-gate, at the ford."`},
		},
		{
			"punctuation that ends exactly at column 80 stays on the line",
			wrapGoldenSay(`I have kept the Bough nineteen years. It is the heart of the valley, if I may say so.`),
			[]string{`Kesh says, "I have kept the Bough nineteen years. It is the heart of the`, `valley, if I may say so."`},
			[]string{`Kesh says, "I have kept the Bough nineteen years. It is the heart of the valley,`, `if I may say so."`},
		},
		{
			"a closing quote is no longer stranded on a line of its own",
			wrapGoldenSay(`A tin thimble, pressed a little out of shape. The dimples on its top`),
			[]string{`Kesh says, "A tin thimble, pressed a little out of shape. The dimples on its top`, `"`},
			[]string{`Kesh says, "A tin thimble, pressed a little out of shape. The dimples on its`, `top"`},
		},
		{
			"a run of spaces collapses to one",
			wrapGoldenSay(`hello    there`),
			[]string{`Kesh says, "hello    there"`},
			[]string{`Kesh says, "hello there"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.oldWrap, preSweepLines(tc.line), "the old wrap's behaviour, pinned")
			require.Equal(t, tc.newWrap, pipelineLines(messaging.CategorySpeech, messaging.ChannelAudio, tc.line), "the pipeline's wrap")
		})
	}
}
```

- [ ] **Step 2: Write the per-recipient test**

Create `internal/actions/wrap_per_recipient_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The reason Speech left the senders' hands: one line to a room, two readers
// at two widths. Clara reads at telnet's 80; Dena is on the web at 40 columns.
func TestSay_OneLineWrapsForEachReader(t *testing.T) {
	sc := newSpeechScene(t)
	webConn := users.GetByUserId(speechDeafId).ConnectionId()
	connections.RegisterTestWebsocketConnection(webConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(webConn) })
	cs := connections.GetClientSettings(webConn)
	cs.Display.ScreenWidth = 40
	connections.OverwriteClientSettings(webConn, cs)

	text := `The pool answers first, it is older than the town and more patient than I am, so look into the water and wait.`
	Say(sc.speaker(true, sc.lit), text)

	heard := func(uid int) []string {
		msgs := events.DrainQueuedMessagesForTest(uid)
		require.Len(t, msgs, 1, "user %d hears the line once", uid)
		return wrapGoldenVisible(strings.TrimRight(msgs[0], "\n"))
	}
	telnet := heard(speechClearId)
	web := heard(speechDeafId)

	require.Len(t, telnet, 2, "at 80 the line takes two lines: %q", telnet)
	require.Greater(t, len(web), 3, "at 40 the same line takes more than three: %q", web)
	for _, line := range telnet {
		require.LessOrEqual(t, len([]rune(line)), 80, "telnet line: %q", line)
	}
	for _, line := range web {
		require.LessOrEqual(t, len([]rune(line)), 40, "web line: %q", line)
	}
	require.Equal(t, strings.Join(telnet, " "), strings.Join(web, " "), "the same words reach both")
}
```

- [ ] **Step 3: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/actions/ -run 'TestWrapGolden|TestSay_OneLine'
```

Expected: build failure, `undefined: formatShoutText`.

- [ ] **Step 4: Admit the categories**

In `internal/messaging/pipeline.go`, Edit:

old:
```go
//   - Speech, Whisper, Shout and Emote are ALREADY wrapped, at a
//     hardcoded 80 that ignores the reader's LineWidth (say.go,
//     shout.go, reply.go, whisper.go). Adding them here would wrap the
//     same text twice at two different widths.
```
new:
```go
//   - CategoryWhisper goes to one known reader, so each sender wraps it
//     itself to that reader's WrapWidth (usercommands/whisper.go, reply.go,
//     mobcommands/whisper.go). Adding it here would wrap the same text
//     twice.
```

old:
```go
		CategoryWeather, CategoryTimeOfDay, CategoryLight,
		// Other narration plus tips.
```
new:
```go
		CategoryWeather, CategoryTimeOfDay, CategoryLight,
		// Sent to a whole room, so each reader's own width applies. No sender
		// may wrap these first.
		CategorySpeech, CategoryShout, CategoryEmote,
		// Other narration plus tips.
```

- [ ] **Step 5: Update the pipeline tests**

In `internal/messaging/pipeline_test.go`, Edit:

old:
```go
	CategoryLight:        true,

	// Other narration plus tips.
```
new:
```go
	CategoryLight:        true,

	// Sent to a whole room, so each reader's own width applies (web client
	// base, plan 1).
	CategorySpeech: true,
	CategoryShout:  true,
	CategoryEmote:  true,

	// Other narration plus tips.
```

old:
```go
// the admitted count is exactly 48 (46 before CategoryCraft, #515, and
// CategoryCraftSeen, owner ruling C), so the pin itself cannot be edited
// without updating its own count.
```
new:
```go
// the admitted count is exactly 51 (46 before CategoryCraft, #515, and
// CategoryCraftSeen, owner ruling C; 48 before Speech, Shout and Emote,
// web client base plan 1), so the pin itself cannot be edited without
// updating its own count.
```

old:
```go
	if admitted != 48 {
		t.Errorf("expected exactly 48 categories admitted to wrap, got %d", admitted)
	}
```
new:
```go
	if admitted != 51 {
		t.Errorf("expected exactly 51 categories admitted to wrap, got %d", admitted)
	}
```

old:
```go
		CategorySpeech:          "already wrapped at a hardcoded 80 by say.go, ignoring LineWidth",
		CategoryWhisper:         "already wrapped at a hardcoded 80 by whisper.go, ignoring LineWidth",
		CategoryShout:           "already wrapped at a hardcoded 80 by shout.go, ignoring LineWidth",
		CategoryEmote:           "already wrapped at a hardcoded 80 by reply.go, ignoring LineWidth",
```
new:
```go
		CategoryWhisper:         "goes to one known reader, and each sender wraps it to that reader's WrapWidth",
```

- [ ] **Step 6: Remove the pre-wraps in `internal/actions`**

`internal/actions/say.go`, three Edits:

old:
```go
	msg := fmt.Sprintf(`<ansi fg="%s">%s</ansi> says, "<ansi fg="%s">%s</ansi>"`, nameColor, name, textColor, text)
	return util.SplitStringNL(msg, 80)
```
new:
```go
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi> says, "<ansi fg="%s">%s</ansi>"`, nameColor, name, textColor, text)
```

old:
```go
// textColor is "saytext" for players, "saytext-mob" for mobs.
func FormatSayText
```
new:
```go
// textColor is "saytext" for players, "saytext-mob" for mobs.
// The line is left unwrapped: CategorySpeech wraps per reader in the pipeline.
func FormatSayText
```

old:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/util"
```
new:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
```

`internal/actions/shout.go`, three Edits:

old:
```go
	line := fmt.Sprintf(`<ansi fg="%s">%s</ansi> shouts, "<ansi fg="%s">%s</ansi>"`,
		nameColor, actor.GetName(), textColor, text)
	sendSpoken(actor, room, messaging.CategoryShout, util.SplitStringNL(line, 80), isSneaking)
```
new:
```go
	sendSpoken(actor, room, messaging.CategoryShout,
		formatShoutText(actor.GetName(), text, nameColor, textColor), isSneaking)
```

old:
```go
// wakeSleepers wakes every sleeper
```
new:
```go
// formatShoutText is the room line of a shout, left unwrapped:
// CategoryShout wraps per reader in the pipeline.
func formatShoutText(name, text, nameColor, textColor string) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi> shouts, "<ansi fg="%s">%s</ansi>"`,
		nameColor, name, textColor, text)
}

// wakeSleepers wakes every sleeper
```

old:
```go
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
```
new:
```go
	"github.com/GoMudEngine/GoMud/internal/users"
```

`internal/actions/emote.go`, three Edits:

old:
```go
import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/util"
)
```
new:
```go
import "fmt"
```

old:
```go
// FormatEmoteText formats an emote action for room display.
// nameColor is "username" for players, "mobname" for mobs.
func FormatEmoteText
```
new:
```go
// FormatEmoteText formats an emote action for room display, unwrapped:
// CategoryEmote and CategoryMobEmote both wrap per reader in the pipeline.
// nameColor is "username" for players, "mobname" for mobs.
func FormatEmoteText
```

old:
```go
	return util.SplitStringNL(emoteLine(name, emoteText, nameColor), 80)
}

// FormatMobEmoteText formats a mob's emote for room display, unwrapped. A
// mob emote goes out as CategoryMobEmote, which the messaging pipeline
// wraps at each reader's width; FormatEmoteText's own 80-column CRLF wrap
// (kept for the player CategoryEmote, which the pipeline does not wrap)
// made it wrap twice, and the second pass counted the carriage return as a
// column and stranded a word on a line of its own (#430).
func FormatMobEmoteText(name string, emoteText string) string {
	return emoteLine(name, emoteText, "mobname")
}

func emoteLine(name, emoteText, nameColor string) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi> <ansi fg="137">%s</ansi>`, nameColor, name, emoteText)
}
```
new:
```go
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi> <ansi fg="137">%s</ansi>`, nameColor, name, emoteText)
}

// FormatMobEmoteText is FormatEmoteText for a mob. A wrap in either formatter
// would wrap the line twice, and the second pass counted the carriage return
// as a column and stranded a word on a line of its own (#430).
func FormatMobEmoteText(name string, emoteText string) string {
	return FormatEmoteText(name, emoteText, "mobname")
}
```

- [ ] **Step 7: Remove the pre-wraps everywhere else**

| File | Old (exact) | New |
|---|---|---|
| `internal/usercommands/say.go` | `user.SendText(messaging.CategorySpeech, util.SplitStringNL(selfMsg, 80))` | `user.SendText(messaging.CategorySpeech, selfMsg)` |
| `internal/usercommands/shout.go` | `user.SendText(messaging.CategoryShout, util.SplitStringNL(selfMsg, 80))` | `user.SendText(messaging.CategoryShout, selfMsg)` |
| `internal/hooks/Quest_HandleQuestUpdate.go` | `messaging.Say(messaging.CategoryEmote, messaging.WrapAnsi(rewardLines.Observer, 80))` | `messaging.Say(messaging.CategoryEmote, rewardLines.Observer)` |

`internal/conversation/talk.go`, Edit:

old:
```go
	user.SendText(messaging.CategorySpeech, util.SplitStringNL(
		fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, text), 80))
```
new:
```go
	user.SendText(messaging.CategorySpeech,
		fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, text))
```

`internal/mobcommands/sayto.go`, three Edits:

old:
```go
// Both helpers wrap at 80 columns, as a plain NPC say does
// (actions.FormatSayText).
func sayToRoom
```
new:
```go
// CategorySpeech wraps per reader in the pipeline, for both helpers.
func sayToRoom
```

old:
```go
	line = util.SplitStringNL(line, 80)
	room.SendTextHidingNames
```
new:
```go
	room.SendTextHidingNames
```

old:
```go
	line = util.SplitStringNL(line, 80)
	toUser.SendText
```
new:
```go
	toUser.SendText
```

`internal/hooks/Quest_HandleQuestUpdate.go`, Edit:

old:
```go
		// room's line rides CategoryEmote, which the pipeline does not wrap
		// either, so it wraps at the 80-column rule (2026-10-17 newbie-duo
		// playtest: Tarn's quest 41 line read 83 wide).
```
new:
```go
		// room's line rides CategoryEmote, which the pipeline wraps for each
		// reader (2026-10-17 newbie-duo playtest: Tarn's quest 41 line read
		// 83 wide when nothing wrapped it).
```

- [ ] **Step 8: Update the existing tests that relied on the formatter wrapping**

`internal/actions/actions_test.go`, two Edits.

old:
```go
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```
new:
```go
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```

old:
```go
// TestFormatSayTextLineWrapping verifies that very long say messages are
// wrapped at 80 characters per line (via util.SplitStringNL).
func TestFormatSayTextLineWrapping(t *testing.T) {
	longText := strings.Repeat("word ", 30) // Creates a very long message
	formatted := FormatSayText("Speaker", longText, "username", "saytext")
	// Check that the output contains newlines (line wrapping)
	lines := strings.Split(formatted, "\n")
	// With proper wrapping, we should have multiple lines
	assert.Greater(t, len(lines), 1, "long say text should be wrapped across multiple lines")
	// Each line should not exceed 80 characters (accounting for ANSI codes)
	for _, line := range lines {
		// Remove ANSI codes to check actual visual length
		cleanLine := stripANSICodes(line)
		assert.LessOrEqual(t, len(cleanLine), 80, "wrapped line should not exceed 80 visible characters")
	}
}

// TestFormatEmoteTextLineWrapping verifies that very long emote messages are
// wrapped at 80 characters per line.
func TestFormatEmoteTextLineWrapping(t *testing.T) {
	longEmote := strings.Repeat("emote ", 25) // Creates a very long emote
	formatted := FormatEmoteText("Actor", longEmote, "mobname")
	lines := strings.Split(formatted, "\n")
	assert.Greater(t, len(lines), 1, "long emote text should be wrapped across multiple lines")
	for _, line := range lines {
		cleanLine := stripANSICodes(line)
		assert.LessOrEqual(t, len(cleanLine), 80, "wrapped line should not exceed 80 visible characters")
	}
}
```
new:
```go
// assertWrapsOnlyInThePipeline checks a formatted room line carries no line
// break of its own, and that the pipeline folds it to the reader's width.
func assertWrapsOnlyInThePipeline(t *testing.T, cat messaging.Category, formatted string) {
	t.Helper()
	assert.NotContains(t, formatted, "\n", "the formatter must leave the wrap to the pipeline")
	rendered := messaging.RenderForRecipient(messaging.RenderInput{
		Category:      cat,
		Text:          formatted,
		Channel:       messaging.ChannelVisual,
		SightDecision: messaging.SightFull,
		LineWidth:     80,
	})
	lines := strings.Split(rendered, "\n")
	assert.Greater(t, len(lines), 1, "a long line should fold across several lines")
	for _, line := range lines {
		assert.LessOrEqual(t, len(stripANSICodes(line)), 80, "wrapped line should not exceed 80 visible characters")
	}
}

// TestFormatSayTextLineWrapping: a say line is wrapped once, per reader, by
// the pipeline, so the formatter itself must not wrap it.
func TestFormatSayTextLineWrapping(t *testing.T) {
	longText := strings.Repeat("word ", 30) // Creates a very long message
	assertWrapsOnlyInThePipeline(t, messaging.CategorySpeech,
		FormatSayText("Speaker", longText, "username", "saytext"))
}

// TestFormatEmoteTextLineWrapping is the same rule for an emote.
func TestFormatEmoteTextLineWrapping(t *testing.T) {
	longEmote := strings.Repeat("emote ", 25) // Creates a very long emote
	assertWrapsOnlyInThePipeline(t, messaging.CategoryEmote,
		FormatEmoteText("Actor", longEmote, "mobname"))
}
```

`internal/actions/emote_wrap_test.go`, three Edits. `TestMobEmote_DoubleWrapPathIsCaught` keeps its job by building the old double wrap itself.

old:
```go
	"github.com/GoMudEngine/GoMud/internal/messaging"
)
```
new:
```go
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)
```

old:
```go
// #430: a long mob emote came out as three lines with one word stranded on
// the middle one ("...buys what" / "others" / "will not"). FormatEmoteText
// hard-wraps at 80 with CRLF for the player emote category, which the
// pipeline leaves alone; a mob emote goes out as CategoryMobEmote, which the
// pipeline wraps again, and the second wrap counted the stray carriage
// return as a column. FormatMobEmoteText leaves the wrap to the pipeline, so
// the line is wrapped once at whatever width the reader has.
```
new:
```go
// #430: a long mob emote came out as three lines with one word stranded on
// the middle one ("...buys what" / "others" / "will not"). The formatter
// hard-wrapped at 80 with CRLF and the pipeline wrapped the line again, and
// the second wrap counted the stray carriage return as a column. The
// formatters now leave the wrap to the pipeline, so the line is wrapped once
// at whatever width the reader has.
```

old:
```go
// The old path, FormatEmoteText's own 80-column wrap followed by the
// pipeline's, fails the same check at both widths: at 80 it strands a word
// on a middle line, at 60 it makes three lines where one wrap makes two.
// This is what proves the check above can fail; if FormatMobEmoteText ever
// wraps again, the test above sees this output.
func TestMobEmote_DoubleWrapPathIsCaught(t *testing.T) {
	for _, width := range []int{80, 60} {
		for _, emote := range longMobEmotes {
			plain := renderMobEmoteLine(FormatEmoteText("Sly Tam", emote, "mobname"), width)
			if len(emoteWrapProblems(plain, width)) == 0 {
				t.Errorf("width %d: double-wrapped emote passed the check: %q", width, plain)
			}
		}
	}
	stranded := renderMobEmoteLine(FormatEmoteText("Sly Tam", longMobEmotes[0], "mobname"), 80)
```
new:
```go
// The old path, an 80-column CRLF wrap in the formatter followed by the
// pipeline's, fails the same check at both widths: at 80 it strands a word
// on a middle line, at 60 it makes three lines where one wrap makes two.
// This is what proves the check above can fail; if a formatter ever wraps
// again, the test above sees this output.
func TestMobEmote_DoubleWrapPathIsCaught(t *testing.T) {
	doubleWrapped := func(emote string) string {
		return util.SplitStringNL(FormatMobEmoteText("Sly Tam", emote), 80)
	}
	for _, width := range []int{80, 60} {
		for _, emote := range longMobEmotes {
			plain := renderMobEmoteLine(doubleWrapped(emote), width)
			if len(emoteWrapProblems(plain, width)) == 0 {
				t.Errorf("width %d: double-wrapped emote passed the check: %q", width, plain)
			}
		}
	}
	stranded := renderMobEmoteLine(doubleWrapped(longMobEmotes[0]), 80)
```

`internal/mobcommands/sayto_sight_test.go`, Edit (the assertion stands; only its reason changed):

old:
```go
// An addressed line wraps at 80 columns the way a plain NPC say does
// (actions.FormatSayText): Cleric Hadwen's rite (9100) speaks long lines
// through sayto, one to each asker, since the 2026-10-17 newbie-duo fix.
```
new:
```go
// An addressed line wraps in the pipeline, at each reader's width, the way a
// plain NPC say does: Cleric Hadwen's rite (9100) speaks long lines through
// sayto, one to each asker, since the 2026-10-17 newbie-duo fix. These
// readers have no saved width, so 80.
```

Tests the brief named that need **no** change, confirmed by running them with the sweep applied: `internal/guards/speech_wrapper_guard_test.go` (`FormatSayText` stays in `say.go`), `internal/guards/wrap_live_render_golden_test.go` and `internal/messaging/wrap_preformatted_golden_test.go` (neither touches the three categories).

- [ ] **Step 9: Run**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go test ./internal/actions/ ./internal/messaging/ ./internal/mobcommands/ ./internal/usercommands/ ./internal/conversation/ ./internal/hooks/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
```

Expected: `ok` everywhere.

- [ ] **Step 10: Prove the golden and per-reader tests can fail**

Using Edit, change `CategorySpeech, CategoryShout, CategoryEmote,` in `pipeline.go` to `CategoryShout, CategoryEmote,`. Run:

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/actions/ -run 'TestWrapGolden|TestSay_OneLine' 2>&1 | grep -E "^\s*--- FAIL"
```

Expected: fails include `TestWrapGolden_VisibleLinesMatchTheOldWrapAt80/say_two_lines`, `TestWrapGolden_AcceptedDifferences/a_hyphenated_word_moves_whole_instead_of_breaking_at_the_hyphen` and `TestSay_OneLineWrapsForEachReader`. Undo that one line with Edit and rerun Step 9.

- [ ] **Step 11: Commit**

```bash
cd C:/tmp/dogmud-wrap-width
git add internal/messaging/pipeline.go internal/messaging/pipeline_test.go \
  internal/actions/say.go internal/actions/shout.go internal/actions/emote.go \
  internal/actions/actions_test.go internal/actions/emote_wrap_test.go \
  internal/actions/wrap_golden_test.go internal/actions/wrap_per_recipient_test.go \
  internal/usercommands/say.go internal/usercommands/shout.go internal/conversation/talk.go \
  internal/mobcommands/sayto.go internal/mobcommands/sayto_sight_test.go \
  internal/hooks/Quest_HandleQuestUpdate.go
git commit -F - <<'EOF'
refactor(messaging): Speech, Shout and Emote wrap per reader

Text sent to a whole room joins shouldWrap and every pre-wrap of it goes.
The telnet golden pins what a reader at 80 sees against the old wrap and
names the five boundary shapes where WrapAnsi differs.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Room idle lines get their own category

**Files:**
- Modify: `internal/messaging/messaging.go:163-168,311-312`, `pipeline.go`, `pipeline_test.go`, `normalize.go:35`, `_datafiles/world/dogmud/ansi-aliases.yaml:56`, `internal/hooks/NewRound_RoomIdle.go:39`, `internal/hooks/NewRound_UserRoundTick.go:215`, `internal/hooks/room_idle_ambient_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/hooks/room_idle_ambient_test.go`, Edit:

old:
```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
```
new:
```go
import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
```

old:
```go
func TestRoomIdleChancePer10k(t *testing.T) {
```
new:
```go
// A room idle line reaches everyone in the room, so it wraps for each of them,
// and it keeps the look of the room description it rode before.
func TestRoomIdle_WrapsForEachReader(t *testing.T) {
	roomIdleFixture(t, []string{"unused"}, 0, 0)
	users.GetByUserId(1).LineWidth = 40
	long := "A cart rattles past on the far side of the square, its load shifting and groaning under a stained canvas cover."

	sendRoomIdleLine(rooms.LoadRoom(1), long, false)

	narrow := events.DrainQueuedMessagesForTest(1)
	wide := events.DrainQueuedMessagesForTest(2)
	require.Len(t, narrow, 1)
	require.Len(t, wide, 1)
	require.Contains(t, narrow[0], `fg="`+messaging.CategoryRoomIdle.String()+`"`)
	for _, line := range strings.Split(strings.TrimRight(plainText(narrow[0]), "\n"), "\n") {
		require.LessOrEqual(t, len([]rune(line)), 40, "%q", line)
	}
	require.Equal(t, 1, strings.Count(strings.TrimRight(plainText(wide[0]), "\n"), "\n"), "two lines at 80: %q", wide[0])
}

func TestRoomIdleChancePer10k(t *testing.T) {
```

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/hooks/ -run 'TestRoomIdle_WrapsForEachReader'
```

Expected: build failure, `undefined: messaging.CategoryRoomIdle`.

- [ ] **Step 3: Add the category**

`internal/messaging/messaging.go`, two Edits:

old:
```go
	CategoryScreen

	// categoryMax is
```
new:
```go
	CategoryScreen

	// A room's idle flavor line, sent to everyone in the room. It rode
	// CategoryRoomDescription, which never wraps, so the sender wrapped it at
	// one width for every reader. Appended so no existing value shifts;
	// normalised and colored as CategoryRoomDescription is (alias room-idle,
	// same value as room-description), so at 80 columns only the wrap's owner
	// changes.
	CategoryRoomIdle

	// categoryMax is
```

old:
```go
	case CategoryScreen:
		return "screen"
```
new:
```go
	case CategoryScreen:
		return "screen"
	case CategoryRoomIdle:
		return "room-idle"
```

`internal/messaging/normalize.go`, Edit:

old: `	case CategoryRoomDescription, CategoryRoomEntry, CategoryRoomExit,`
new: `	case CategoryRoomDescription, CategoryRoomIdle, CategoryRoomEntry, CategoryRoomExit,`

`_datafiles/world/dogmud/ansi-aliases.yaml`, Edit:

old:
```yaml
  room-description: 15
```
new:
```yaml
  room-description: 15
  room-idle: 15
```

`internal/messaging/pipeline.go`, Edit:

old: `		CategoryWeather, CategoryTimeOfDay, CategoryLight,`
new: `		CategoryWeather, CategoryTimeOfDay, CategoryLight, CategoryRoomIdle,`

`internal/messaging/pipeline_test.go`, three Edits:

old: `	CategoryLight:        true,`
new:
```go
	CategoryLight:        true,
	CategoryRoomIdle:     true, // web client base, plan 1
```

old:
```go
// the admitted count is exactly 51 (46 before CategoryCraft, #515, and
// CategoryCraftSeen, owner ruling C; 48 before Speech, Shout and Emote,
// web client base plan 1), so the pin itself cannot be edited without
// updating its own count.
```
new:
```go
// the admitted count is exactly 52 (46 before CategoryCraft, #515, and
// CategoryCraftSeen, owner ruling C; 48 before Speech, Shout, Emote and
// RoomIdle, web client base plan 1), so the pin itself cannot be edited
// without updating its own count.
```

old:
```go
	if admitted != 51 {
		t.Errorf("expected exactly 51 categories admitted to wrap, got %d", admitted)
	}
```
new:
```go
	if admitted != 52 {
		t.Errorf("expected exactly 52 categories admitted to wrap, got %d", admitted)
	}
```

- [ ] **Step 4: Use it**

| File | Old (exact) | New |
|---|---|---|
| `internal/hooks/NewRound_RoomIdle.go` | `sendVisualRoomText(room, messaging.CategoryRoomDescription, msg, plan.Exclude...)` | `sendVisualRoomText(room, messaging.CategoryRoomIdle, msg, plan.Exclude...)` |
| `internal/hooks/NewRound_UserRoundTick.go` | `sendRoomIdleLine(room, util.SplitStringNL(msg, 80), idleMsgCt == 1)` | `sendRoomIdleLine(room, msg, idleMsgCt == 1)` |

- [ ] **Step 5: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go test ./internal/hooks/ ./internal/messaging/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/messaging/messaging.go internal/messaging/normalize.go internal/messaging/pipeline.go internal/messaging/pipeline_test.go \
  _datafiles/world/dogmud/ansi-aliases.yaml \
  internal/hooks/NewRound_RoomIdle.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/room_idle_ambient_test.go
git commit -F - <<'EOF'
refactor(messaging): room idle lines get CategoryRoomIdle

CategoryRoomDescription cannot wrap, because it carries the minimap block,
so the idle line wrapped itself at one width for the whole room. The new
category looks the same and wraps per reader.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 7: Next-door player chatter wraps per reader

**Files:**
- Create: `internal/hooks/room_chatter_wrap_test.go`
- Modify: `internal/hooks/Message_SendMessages.go:96`, `internal/actions/shout.go:67`

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/room_chatter_wrap_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A shout from next door and the companion's `ask` line arrive unrendered on
// the RoomId branch, so that branch wraps them for each reader. Other RoomId
// messages keep their own lines.
func TestRoomMessageTextFor_WrapsOnlyChatter(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	reader := users.GetByUserId(1)
	reader.LineWidth = 40
	long := `Someone shouts from the <ansi fg="exit">north</ansi> direction, "<ansi fg="yellow">THE BRIDGE IS OUT AND THE RIVER IS RISING AND THE EAST ROAD WILL BE UNDER WATER</ansi>"`

	chatter := roomMessageTextFor(events.Message{RoomId: 1, Text: long + "\n", IsCommunication: true}, reader)
	lines := strings.Split(strings.TrimRight(chatter, "\n"), "\n")
	require.Greater(t, len(lines), 2, "%q", chatter)
	for _, line := range lines {
		require.LessOrEqual(t, util.VisibleWidth(line), 40, "%q", line)
	}
	require.True(t, strings.HasSuffix(chatter, "\n"), "the message still ends its line")

	plain := long + "\n"
	require.Equal(t, plain, roomMessageTextFor(events.Message{RoomId: 1, Text: plain}, reader))
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/hooks/ -run 'TestRoomMessageTextFor'
```

Expected: build failure, `undefined: roomMessageTextFor`.

- [ ] **Step 3: Implement**

`internal/hooks/Message_SendMessages.go`, three Edits. The first old string carries four leading tabs, which makes it unique: the `UserId` branch at line 33 has three.

old:
```go
				textOut := templates.AnsiParse(message.Text)
```
new:
```go
				textOut := templates.AnsiParse(roomMessageTextFor(message, user))
```

old:
```go
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```
new:
```go
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```

Add at the end of the file:

old:
```go
	return events.Continue

}
```
new:
```go
	return events.Continue

}

// roomMessageTextFor is what user reads of a RoomId-keyed message. Player
// chatter on this branch (a shout from next door, the AI companion's `ask`
// line) arrives unrendered, so it wraps here, once per reader. Every other
// RoomId message keeps the lines it was sent with.
func roomMessageTextFor(message events.Message, user *users.UserRecord) string {
	if !message.IsCommunication {
		return message.Text
	}
	return messaging.WrapAnsi(message.Text, user.WrapWidth())
}
```

`internal/actions/shout.go`, Edit:

old: `	// line stays player chatter (deafen-filtered, byte-identical to before);`
new: `	// line stays player chatter (deafen-filtered, wrapped per reader by the listener);`

- [ ] **Step 4: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go test ./internal/hooks/ ./internal/actions/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/hooks/Message_SendMessages.go internal/hooks/room_chatter_wrap_test.go internal/actions/shout.go
git commit -F - <<'EOF'
refactor(hooks): next-door player chatter wraps per reader

The mob's next-door shout now wraps in the pipeline; this is its sibling
for the player's, which arrives unrendered and keeps its deafen filter.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 8: Text to one known player passes that player's width

**Files:**
- Create: `internal/usercommands/wrap_width_test.go`, `internal/actions/actor_wrap_width_test.go`
- Modify: `internal/usercommands/look.go:350,385,467,662`, `reply.go:38`, `whisper.go:57`, `guild.go:489`, `party.go:818`, `internal/mobcommands/whisper.go:53`, `internal/actions/drink.go:85,436`, `internal/actions/roster.go`, `modules/aicompanion/commands.go:552,567,667,767`, `modules/aicompanion/tiers.go:358`, `internal/guards/messaging_surface_guard_test.go:777,803,1416,1445`

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/wrap_width_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wrapTestLongLine = "the pool answers first, it is older than the town and more patient than I am, so look into the water and wait"

// wrapTestLines is the one message queued for uid, as the lines it reads.
func wrapTestLines(t *testing.T, uid int) []string {
	t.Helper()
	msgs := events.DrainQueuedMessagesForTest(uid)
	require.Len(t, msgs, 1, "user %d should get one message: %q", uid, msgs)
	return strings.Split(strings.TrimRight(setScreenEscRE.ReplaceAllString(msgs[0], ""), "\n"), "\n")
}

// A whisper and a reply wrap for the person reading them, not the sender.
func TestWhisperAndReply_WrapAtTheReadersWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	reader := users.GetByCharacterName("bobrick")
	require.NotNil(t, reader)
	reader.LineWidth = 40
	user.LineWidth = 200
	events.DrainQueuedMessagesForTest(reader.UserId)

	_, err := Whisper("bobrick "+wrapTestLongLine, user, room, 0)
	require.NoError(t, err)
	whispered := wrapTestLines(t, reader.UserId)
	require.Greater(t, len(whispered), 2)
	for _, line := range whispered {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}

	user.LastWhisperFrom = reader.UserId
	reader.LineWidth = 200
	user.LineWidth = 40
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Reply(wrapTestLongLine, reader, room, 0)
	require.NoError(t, err)
	replied := wrapTestLines(t, user.UserId)
	require.Greater(t, len(replied), 2)
	for _, line := range replied {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}
}
```

Create `internal/actions/actor_wrap_width_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestActorWrapWidth(t *testing.T) {
	u := users.NewTestUser(8890, "reader", "Reader", 98890)
	u.LineWidth = 50
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8890: u}))

	require.Equal(t, 50, actorWrapWidth(&UserActor{User: u}))
	require.Equal(t, 80, actorWrapWidth(&MobActor{Mob: &mobs.Mob{}}), "a mob has no reader; its SendText is a no-op")
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/usercommands/ -run 'TestWhisperAndReply_WrapAt' 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
go test ./internal/actions/ -run 'TestActorWrapWidth' 2>&1 | tail -3
```

Expected: `--- FAIL: TestWhisperAndReply_WrapAtTheReadersWidth` (lines over 40), and a build failure, `undefined: actorWrapWidth`.

- [ ] **Step 3: Add `actorWrapWidth`**

It goes in `roster.go` so that neither `drink.go` nor `actor_user.go` gains a line (fact 23).

`internal/actions/roster.go`, Edit:

old:
```go
// RenderRoster renders the room roster ("Also here: ...", the
```
new:
```go
// actorWrapWidth is the width a line for actor alone wraps at. A mob's user id
// 0 finds no user, and a nil user's WrapWidth is the default 80; a mob's
// SendText drops the line anyway.
func actorWrapWidth(actor Actor) int {
	return users.GetByUserId(actor.GetUserId()).WrapWidth()
}

// RenderRoster renders the room roster ("Also here: ...", the
```

- [ ] **Step 4: Pass the reader's width at every site**

| File | Old (exact) | New |
|---|---|---|
| `internal/usercommands/look.go` | `util.SplitStringNL(desc, 80)` | `util.SplitStringNL(desc, user.WrapWidth())` |
| `internal/usercommands/look.go` | `util.SplitStringNL(itemDesc, 80)` | `util.SplitStringNL(itemDesc, user.WrapWidth())` |
| `internal/usercommands/look.go` | `util.SplitStringNL(foundDesc, 80)` | `util.SplitStringNL(foundDesc, user.WrapWidth())` |
| `internal/usercommands/look.go` | `util.SplitStringNL(floorItem.LongDescriptionFor(user.UserId), 80)` | `util.SplitStringNL(floorItem.LongDescriptionFor(user.UserId), user.WrapWidth())` |
| `internal/usercommands/reply.go` | `util.SplitStringNL(replyMsg, 80)` | `util.SplitStringNL(replyMsg, targetUser.WrapWidth())` |
| `internal/usercommands/whisper.go` | `util.SplitStringNL(whisperMsg, 80)` | `util.SplitStringNL(whisperMsg, toUser.WrapWidth())` |
| `internal/usercommands/guild.go` | `util.SplitStringNL(line, 80)` | `util.SplitStringNL(line, u.WrapWidth())` |
| `internal/usercommands/party.go` | `util.SplitStringNL(msg, 80)` | `util.SplitStringNL(msg, u.WrapWidth())` |
| `internal/mobcommands/whisper.go` | `util.SplitStringNL(line, 80)` | `util.SplitStringNL(line, toUser.WrapWidth())` |
| `internal/actions/drink.go` | ``as close to it as you're likely to get.`, 80))`` | ``as close to it as you're likely to get.`, actorWrapWidth(actor)))`` |
| `internal/actions/drink.go` | ``scoured out, and you are left shaking and hollow.`, 80))`` | ``scoured out, and you are left shaking and hollow.`, actorWrapWidth(actor)))`` |
| `modules/aicompanion/commands.go` | `messaging.WrapAnsi(msg, 80)` | `messaging.WrapAnsi(msg, user.WrapWidth())` |
| `modules/aicompanion/commands.go` | `messaging.WrapAnsi(m.setBoundary(c, strings.ToLower(strings.TrimSpace(rest))), 80)` | `messaging.WrapAnsi(m.setBoundary(c, strings.ToLower(strings.TrimSpace(rest))), user.WrapWidth())` |
| `modules/aicompanion/commands.go` | ``strings.Join(labels, ` / `), 80))`` | ``strings.Join(labels, ` / `), user.WrapWidth()))`` |
| `modules/aicompanion/commands.go` | `messaging.WrapAnsi(fmt.Sprintf(format, args...), 80)` | `messaging.WrapAnsi(fmt.Sprintf(format, args...), user.WrapWidth())` |
| `modules/aicompanion/commands.go` | `// here is wrapped at 80 columns before it is sent.` | `// here is wrapped to the reader's width before it is sent.` |
| `modules/aicompanion/tiers.go` | `text = messaging.WrapAnsi(text, 80)` | `text = messaging.WrapAnsi(text, users.GetByUserId(userId).WrapWidth())` |
| `modules/aicompanion/tiers.go` | `// tellOwner sends a player a warning notice, wrapped at 80 columns: the` | `// tellOwner sends a player a warning notice, wrapped to their width: the` |

- [ ] **Step 5: Re-key the two whisper entries in the surface guard**

The fingerprint is the literal source text, so the keys change with it. `internal/guards/messaging_surface_guard_test.go`, four Edits (the padding keeps gofmt's column):

old:
```go
	"usercommands/reply.go|messaging.CategoryWhisper, util.SplitStringNL(replyMsg, 80)":                               {verdictCorrect
```
new:
```go
	"usercommands/reply.go|messaging.CategoryWhisper, util.SplitStringNL(replyMsg, targetUser.WrapWidth())":           {verdictCorrect
```

old:
```go
	"usercommands/whisper.go|messaging.CategoryWhisper, util.SplitStringNL(whisperMsg, 80)":                     {verdictCorrect
```
new:
```go
	"usercommands/whisper.go|messaging.CategoryWhisper, util.SplitStringNL(whisperMsg, toUser.WrapWidth())":     {verdictCorrect
```

old: `// util.SplitStringNL(replyMsg, 80)).`
new: `// util.SplitStringNL(replyMsg, targetUser.WrapWidth())).`

old: `// "util.SplitStringNL(replyMsg, 80)"), which is stable and still distinct`
new: `// "util.SplitStringNL(replyMsg, targetUser.WrapWidth())"), which is stable and still distinct`

- [ ] **Step 6: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && gofmt -l internal modules
go build ./... && go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ ./modules/aicompanion/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/usercommands/wrap_width_test.go internal/usercommands/look.go internal/usercommands/reply.go internal/usercommands/whisper.go \
  internal/usercommands/guild.go internal/usercommands/party.go internal/mobcommands/whisper.go \
  internal/actions/drink.go internal/actions/roster.go internal/actions/actor_wrap_width_test.go \
  modules/aicompanion/commands.go modules/aicompanion/tiers.go internal/guards/messaging_surface_guard_test.go
git commit -F - <<'EOF'
refactor(messaging): text to one player wraps at that player's width

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `gofmt -l` prints nothing; `ok` everywhere. In particular `TestPlayerConditionsTravelTheEventPath` stays green, which is how you know `drink.go` and `actor_user.go` kept their line numbers.

---

### Task 9: The remaining `GetLineWidth` reads

These already follow the saved width, so nothing changes for a telnet reader; Task 13's `TestOnlyWrapWidthReadsTheSavedLineWidth` is what holds them.

**Files:**
- Modify: `internal/actions/roster.go:24,42`, `internal/justice/arrest.go:416`, `internal/usercommands/start.go:110`, `quests.go:151`, `motd.go:34`, `mutations.go:31`, `inbox.go:32`, `internal/hooks/Quest_HandleQuestUpdate.go:195`

- [ ] **Step 1: Edit each site**

| File | Old (exact) | New |
|---|---|---|
| `internal/actions/roster.go` (`replace_all`, 2) | `users.GetByUserId(userId).GetLineWidth()` | `users.GetByUserId(userId).WrapWidth()` |
| `internal/justice/arrest.go` | `			u.GetLineWidth()))` | `			u.WrapWidth()))` |
| `internal/usercommands/start.go` | `			user.GetLineWidth())+term.CRLFStr)` | `			user.WrapWidth())+term.CRLFStr)` |
| `internal/usercommands/quests.go` | `messaging.WrapAnsi(line, user.GetLineWidth())` | `messaging.WrapAnsi(line, user.WrapWidth())` |
| `internal/usercommands/motd.go` | `motdBox(text, user.GetLineWidth())` | `motdBox(text, user.WrapWidth())` |
| `internal/usercommands/mutations.go` | `width := user.GetLineWidth()` | `width := user.WrapWidth()` |
| `internal/usercommands/inbox.go` | ``strings.Repeat(`_`, user.GetLineWidth())`` | ``strings.Repeat(`_`, user.WrapWidth())`` |
| `internal/hooks/Quest_HandleQuestUpdate.go` | `messaging.WrapAnsi(rewardLines.Actor, questUser.GetLineWidth())` | `messaging.WrapAnsi(rewardLines.Actor, questUser.WrapWidth())` |

- [ ] **Step 2: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go test ./internal/actions/ ./internal/justice/ ./internal/usercommands/ ./internal/hooks/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/actions/roster.go internal/justice/arrest.go internal/usercommands/start.go internal/usercommands/quests.go \
  internal/usercommands/motd.go internal/usercommands/mutations.go internal/usercommands/inbox.go internal/hooks/Quest_HandleQuestUpdate.go
git commit -F - <<'EOF'
refactor(users): every wrap reads WrapWidth, not the saved LineWidth

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 10: Room descriptions

**Files:**
- Create: `internal/rooms/roomdetails_wrap_width_test.go`
- Modify: `internal/rooms/rooms.go:2076`, `internal/rooms/roomdetails.go:117,136`

- [ ] **Step 1: Write the failing tests**

Create `internal/rooms/roomdetails_wrap_width_test.go`:

```go
package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// A room description follows the person looking at it.
func TestGetDetails_DescriptionWrapsToTheViewersWidth(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	viewer.LineWidth = 50

	lines := strings.Split(GetDetails(r, viewer).Description, "\n")
	if len(lines) < 2 {
		t.Fatalf("a long description should fold at 50 columns: %q", lines)
	}
	for _, line := range lines {
		if w := util.VisibleWidth(line); w > 50 {
			t.Errorf("description line is %d columns, want <= 50: %q", w, line)
		}
	}
}

// The minimap keeps its seven columns at the right edge of the viewer's width.
func TestGetDetails_MinimapSitsAtTheViewersRightEdge(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	viewer.LineWidth = 60
	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}

	lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
	for i := range tinymap {
		if w := util.VisibleWidth(lines[i]); w != 60 {
			t.Errorf("line %d beside the minimap is %d columns, want 60: %q", i, w, lines[i])
		}
	}
}

// GetDescriptionFormatted used to ignore its width.
func TestGetDescriptionFormatted_UsesTheWidthItIsGiven(t *testing.T) {
	r := &Room{Description: strings.Repeat("a quiet stone hall ", 12)}
	for _, width := range []int{40, 72} {
		for _, line := range strings.Split(strings.ReplaceAll(r.GetDescriptionFormatted(width, false), "\r", ""), "\n") {
			if w := util.VisibleWidth(line); w > width {
				t.Errorf("width %d: line is %d columns: %q", width, w, line)
			}
		}
	}
	if strings.Count(r.GetDescriptionFormatted(40, false), "\n") <= strings.Count(r.GetDescriptionFormatted(72, false), "\n") {
		t.Error("a narrower width must make more lines")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/rooms/ -run 'WrapsToTheViewersWidth|ViewersRightEdge|UsesTheWidthItIsGiven' 2>&1 | grep -E "^(--- FAIL|ok)"
```

Expected: all three `--- FAIL`.

- [ ] **Step 3: Implement**

| File | Old (exact) | New |
|---|---|---|
| `internal/rooms/rooms.go` | `desc := util.SplitStringNL(r.GetDescription(), 80)` | `desc := util.SplitStringNL(r.GetDescription(), lineSplit)` |
| `internal/rooms/roomdetails.go` | `desclineWidth := 80 - 7 // 7 is the width of the tinymap` | `desclineWidth := user.WrapWidth() - 7 // 7 is the width of the tinymap` |
| `internal/rooms/roomdetails.go` | `roomDesc := wrapDescriptionParts(descParts, 80, nouns)` | `roomDesc := wrapDescriptionParts(descParts, user.WrapWidth(), nouns)` |

- [ ] **Step 4: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/rooms/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/rooms/rooms.go internal/rooms/roomdetails.go internal/rooms/roomdetails_wrap_width_test.go
git commit -F - <<'EOF'
refactor(rooms): room descriptions wrap to the viewer's width

GetDescriptionFormatted now uses the width it was always given.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 11: Menus take the viewer's width

**Files:**
- Modify: `internal/menu/menu.go:24,47-49,74-76,84,98,133,144`, `internal/menu/menu_test.go`, `resolve_test.go`, `render_bytes_test.go:57`, `internal/users/userrecord.menu.go:124,126,381`, and the 14 caller files in Step 4

- [ ] **Step 1: Write the failing test and move the existing tests to the new signature**

`internal/menu/menu_test.go`, three Edits, in this order:

With `replace_all`: old `Render(` new `renderAt80(`. (21 occurrences. No test function is named `...Render(`; the `TestRender_` names have an underscore.)

Then:

old:
```go
func plain(s string) string { return tagRe.ReplaceAllString(s, "") }
```
new:
```go
func plain(s string) string { return tagRe.ReplaceAllString(s, "") }

// menuTestWidth is the reader these layout tests draw for.
const menuTestWidth = 80

func renderAt80(m Menu) string { return Render(m, menuTestWidth) }
```

old:
```go
		if n := utf8.RuneCountInString(line); n > lineWidth {
			t.Fatalf("line over %d columns (%d): %q", lineWidth, n, line)
		}
	}
}
```
new:
```go
		if n := utf8.RuneCountInString(line); n > menuTestWidth {
			t.Fatalf("line over %d columns (%d): %q", menuTestWidth, n, line)
		}
	}
}

// A menu's prose follows its reader: the Body and each Detail wrap to the
// width given, so a narrow reader gets more lines and none over the width.
func TestRender_ProseWrapsToTheReadersWidth(t *testing.T) {
	long := "Garve has asked you to thin out the bandits in the wash again, a scout first and then a squatter."
	m := Menu{
		Heading: "Offer",
		Body:    []string{long},
		Entries: []Entry{{Id: "a", Label: "Accept", Detail: long}},
	}
	for _, width := range []int{30, 42, 80} {
		for _, line := range strings.Split(plain(Render(m, width)), "\n") {
			if n := utf8.RuneCountInString(line); n > width {
				t.Errorf("width %d: line over the width (%d): %q", width, n, line)
			}
		}
	}
	narrow := strings.Count(plain(Render(m, 30)), "\n")
	wide := strings.Count(plain(Render(m, 80)), "\n")
	if narrow <= wide {
		t.Errorf("width 30 made %d line breaks, width 80 made %d: the width is not reaching the wrap", narrow, wide)
	}
}
```

`internal/menu/resolve_test.go`, `replace_all`: old `Render(` new `renderAt80(` (2 occurrences).

`internal/menu/render_bytes_test.go`, Edit: old `Text: menu.Render(m), Channel` new `Text: menu.Render(m, 80), Channel`.

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/menu/ 2>&1 | head -8
```

Expected: build failure, `too many arguments in call to Render`.

- [ ] **Step 3: Change `menu.go`**

Seven Edits:

old:
```go
const (
	lineWidth    = 80
	numberColor  = "command"
```
new:
```go
const (
	numberColor  = "command"
```

old:
```go
	// each wrapped at the 80-column width (an offer card's objective and
	// reward). The labels lint does not read it.
```
new:
```go
	// each wrapped to the width given to Render (an offer card's objective
	// and reward). The labels lint does not read it.
```

old:
```go
// Render draws the menu at the 80-column wrap. Callers send it on
// messaging.CategoryScreen (player menus) or CategoryAdmin (builders).
func Render(m Menu) string {
```
new:
```go
// Render draws the menu for a reader width columns wide (their
// UserRecord.WrapWidth). Callers send it on messaging.CategoryScreen (player
// menus) or CategoryAdmin (builders).
func Render(m Menu, width int) string {
```

old: `for _, line := range wrap(para, lineWidth) {` new: `for _, line := range wrap(para, width) {`

old: `renderEntry(&b, i+1, numW, e)` new: `renderEntry(&b, i+1, numW, e, width)`

old: `func renderEntry(b *strings.Builder, n, numW int, e Entry) {` new: `func renderEntry(b *strings.Builder, n, numW int, e Entry, width int) {`

old: `wrap(e.Detail, lineWidth-len(indent))` new: `wrap(e.Detail, width-len(indent))`

- [ ] **Step 4: Pass the viewer at all 50 callers**

Inside `internal/users/userrecord.menu.go`, three Edits:

| Old (exact) | New |
|---|---|
| `u.sendTextAt(messaging.CategoryScreen, menu.Render(m), menuShowLastPriority)` | `u.sendTextAt(messaging.CategoryScreen, menu.Render(m, u.WrapWidth()), menuShowLastPriority)` |
| `u.SendText(messaging.CategoryScreen, menu.Render(m))` | `u.SendText(messaging.CategoryScreen, menu.Render(m, u.WrapWidth()))` |
| `u.SendText(messaging.CategoryScreen, menu.Render(om.m))` | `u.SendText(messaging.CategoryScreen, menu.Render(om.m, u.WrapWidth()))` |

Everywhere else, Edit with `replace_all`, old `menu.Render(X)` new `menu.Render(X, user.WrapWidth())`:

| File | X | Count |
|---|---|---|
| `internal/usercommands/start.go` | `*question.Menu` | 1 |
| `internal/usercommands/start.go` | `*confirm.Menu` | 1 |
| `modules/auctions/auctions.go` | `*questionConfirm.Menu` | 1 |
| `internal/usercommands/renameself.go` | `*q.Menu` | 1 |
| `internal/usercommands/picklock.go` | `*question.Menu` | 2 |
| `internal/usercommands/mail.go` | `*question.Menu` | 1 |
| `internal/usercommands/admin.mudmail.go` | `*question.Menu` | 1 |
| `internal/usercommands/admin.item.go` | `*question.Menu` | 4 |
| `internal/usercommands/admin.item.go` | `typeMenu` | 1 |
| `internal/usercommands/admin.item.go` | `subTypeMenu` | 1 |
| `internal/usercommands/admin.zone.go` | `*question.Menu` | 4 |
| `internal/usercommands/guild.go` | `*q.Menu` | 4 |
| `internal/usercommands/admin.room.exits.go` | `*question.Menu` | 3 |
| `internal/usercommands/admin.server.go` | `*question.Menu` | 2 |
| `internal/usercommands/admin.room.go` | `*question.Menu` | 13 |
| `internal/usercommands/character.go` | `*question.Menu` | 5 |
| `internal/usercommands/character.go` | `*q.Menu` | 1 |
| `internal/usercommands/deletecharacter.go` | `*q1.Menu` | 1 |

Total 47, plus the 3 above, is 50.

- [ ] **Step 5: Let the compiler confirm none are left**

```bash
cd C:/tmp/dogmud-wrap-width && go build ./... && go vet ./internal/menu/ ./internal/users/ ./internal/usercommands/ ./modules/auctions/
```

Expected: no output.

- [ ] **Step 6: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/menu/ ./internal/users/ ./internal/usercommands/ ./internal/prompt/ ./modules/auctions/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/menu/menu.go internal/menu/menu_test.go internal/menu/resolve_test.go internal/menu/render_bytes_test.go \
  internal/users/userrecord.menu.go modules/auctions/auctions.go \
  internal/usercommands/start.go internal/usercommands/renameself.go internal/usercommands/picklock.go internal/usercommands/mail.go \
  internal/usercommands/admin.mudmail.go internal/usercommands/admin.item.go internal/usercommands/admin.zone.go internal/usercommands/guild.go \
  internal/usercommands/admin.room.exits.go internal/usercommands/admin.server.go internal/usercommands/admin.room.go \
  internal/usercommands/character.go internal/usercommands/deletecharacter.go
git commit -F - <<'EOF'
refactor(menu): Render takes the reader's width

Required rather than defaulted, so the compiler finds every caller. Body
and Detail follow it; the fixed two-column admin layout is plan 3's.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 12: The `set` screen

**Files:**
- Modify: `internal/usercommands/set.go:74-77,106-110,119,134,283`, `internal/usercommands/set_blank_lines_test.go:64-65`, `internal/usercommands/wrap_width_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/usercommands/wrap_width_test.go` (Edit, on the file's last closing of `TestWhisperAndReply_WrapAtTheReadersWidth`):

old:
```go
	replied := wrapTestLines(t, user.UserId)
	require.Greater(t, len(replied), 2)
	for _, line := range replied {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}
}
```
new:
```go
	replied := wrapTestLines(t, user.UserId)
	require.Greater(t, len(replied), 2)
	for _, line := range replied {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "%q", line)
	}
}

// The set screen lays out to the reader, as a bare 80 used to pretend they all were.
func TestSetScreen_WrapsToTheReadersWidth(t *testing.T) {
	u := users.NewTestUser(8803, "narrow", "Narrow", 18803)
	u.LineWidth = 40
	u.Character.Description = wrapTestLongLine
	u.SetConfigOption(`fprompt`, shippedFightPrompt)

	lines := renderedSetLines(t, u, func() { displaySetStatus(u) })
	require.Greater(t, len(lines), 5)
	for i, line := range lines {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, "line %d: %q", i, line)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/usercommands/ -run 'TestSetScreen_WrapsToTheReadersWidth' 2>&1 | grep -E "^(--- FAIL|ok)"
```

Expected: `--- FAIL`, lines over 40.

- [ ] **Step 3: Implement**

`internal/usercommands/set.go`, five Edits:

old:
```go
// setScreenWidth is the width the set status screen lays itself out to. The
// screen rides CategoryScreen, which neither normalizes nor wraps, so it
// wraps its own long values (the description, a long fight prompt).
const setScreenWidth = 80

```
new: (delete, empty replacement)

old:
```go
// each sat on its own line.
func displaySetStatus(user *users.UserRecord) {
	c := configs.GetTextFormatsConfig()
```
new:
```go
// each sat on its own line. CategoryScreen neither normalizes nor wraps, so
// the screen wraps its own long values (the description, a long fight prompt)
// to the reader's width.
func displaySetStatus(user *users.UserRecord) {
	c := configs.GetTextFormatsConfig()
	width := user.WrapWidth()
```

old: `row(label, wrapPrompt(prompt, setScreenWidth-len(indent), indent))` new: `row(label, wrapPrompt(prompt, width-len(indent), indent))`

old: `util.SplitStringNL(user.Character.Description, setScreenWidth)` new: `util.SplitStringNL(user.Character.Description, width)`

old: ``wrapPrompt(customPrompt.(string), setScreenWidth-2, `  `)`` new: ``wrapPrompt(customPrompt.(string), user.WrapWidth()-2, `  `)``

`internal/usercommands/set_blank_lines_test.go`, Edit:

old:
```go
			if w := utf8.RuneCountInString(line); w > setScreenWidth {
				t.Errorf("%s line %d is %d columns, over %d: %q", name, i, w, setScreenWidth, line)
			}
```
new:
```go
			if w := utf8.RuneCountInString(line); w > u.WrapWidth() {
				t.Errorf("%s line %d is %d columns, over %d: %q", name, i, w, u.WrapWidth(), line)
			}
```

- [ ] **Step 4: Run, gate, commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/usercommands/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/usercommands/set.go internal/usercommands/set_blank_lines_test.go internal/usercommands/wrap_width_test.go
git commit -F - <<'EOF'
refactor(usercommands): the set screen lays out to the reader's width

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` everywhere.

---

### Task 13: The width guards

**Files:**
- Create: `internal/guards/wrap_width_guard_test.go`

- [ ] **Step 1: Write the guard**

Create `internal/guards/wrap_width_guard_test.go`:

```go
package guards

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Web client base, ruling 1: all wrapping goes through one place and follows
// the device width. UserRecord.WrapWidth is that place. These guards fail a
// literal or constant width handed to a wrap function, a literal width on a
// RenderInput, and any new read of the saved LineWidth where a wrap width was
// meant.

// wrapWidthFuncs names every function that wraps text, and which argument is
// the width.
var wrapWidthFuncs = map[string]int{
	"SplitString":          1,
	"SplitStringNL":        1,
	"WrapAnsi":             1,
	"wrap":                 1, // internal/menu and internal/messaging
	"wrapPrompt":           1, // internal/usercommands/set.go
	"wrapDescriptionParts": 1, // internal/rooms/roomdetails.go
}

// wrapWidthExempt is every fixed width allowed to reach a wrap function, keyed
// on file and the printed width expression. Authoring limits that reject a
// long line (lightnotice/store.go, darkcombat/store.go), a table's own column
// budget (online.go) and a hint's length check (sight_aim.go) never reach a
// wrap function, so they need no entry here.
var wrapWidthExempt = map[string]string{
	"internal/usercommands/setdesc.go|72":              "echoes the description at the 72 columns the character template shows it; a template layout, plan 3's to change",
	"internal/usercommands/admin.server.go|lineLength": "the admin config dump, laid out to 158 columns less the longest key name; staff tooling on a wide screen",
}

// getLineWidthAllowed is who may read the saved width directly.
var getLineWidthAllowed = map[string]string{
	"internal/users/wrapwidth.go":  "WrapWidth itself",
	"internal/usercommands/set.go": "`set linewidth` reports the saved setting, which is not a wrap",
}

// wrapConstIndex is every const declared under internal/ and modules/: bare
// names per directory, and "pkg.Name" for a qualified use.
type wrapConstIndex struct {
	byDir map[string]map[string]bool
	byPkg map[string]bool
}

func newWrapConstIndex() *wrapConstIndex {
	return &wrapConstIndex{byDir: map[string]map[string]bool{}, byPkg: map[string]bool{}}
}

func (x *wrapConstIndex) add(dir string, f *ast.File) {
	if x.byDir[dir] == nil {
		x.byDir[dir] = map[string]bool{}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		d, ok := n.(*ast.GenDecl)
		if !ok || d.Tok != token.CONST {
			return true
		}
		for _, s := range d.Specs {
			for _, name := range s.(*ast.ValueSpec).Names {
				x.byDir[dir][name.Name] = true
				x.byPkg[f.Name.Name+"."+name.Name] = true
			}
		}
		return true
	})
}

// wrapWidthIsFixed reports whether expr has the same value for every reader:
// a literal, a constant, len() of something (a layout's own overhead, which
// says nothing about the reader), or arithmetic of those.
func wrapWidthIsFixed(expr ast.Expr, dir string, consts *wrapConstIndex, fn *ast.FuncDecl) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return wrapWidthIsFixed(e.X, dir, consts, fn)
	case *ast.BinaryExpr:
		return wrapWidthIsFixed(e.X, dir, consts, fn) && wrapWidthIsFixed(e.Y, dir, consts, fn)
	case *ast.CallExpr:
		id, ok := e.Fun.(*ast.Ident)
		return ok && id.Name == "len"
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		return ok && consts.byPkg[pkg.Name+"."+e.Sel.Name]
	case *ast.Ident:
		if consts.byDir[dir][e.Name] {
			return true
		}
		return localOnlyAssignedFixed(e.Name, dir, consts, fn)
	}
	return false
}

// localOnlyAssignedFixed reports whether name is a local of fn whose every
// assignment is fixed, so `w := 80` cannot launder a literal. A parameter, or
// a name fn never assigns, is not fixed.
func localOnlyAssignedFixed(name, dir string, consts *wrapConstIndex, fn *ast.FuncDecl) bool {
	if fn == nil {
		return false
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, id := range field.Names {
				if id.Name == name {
					return false
				}
			}
		}
	}
	values := []ast.Expr{}
	allFixed := true
	ast.Inspect(fn, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
					if len(s.Rhs) != len(s.Lhs) {
						allFixed = false
						continue
					}
					values = append(values, s.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if id.Name == name && i < len(s.Values) {
					values = append(values, s.Values[i])
				}
			}
		}
		return true
	})
	if !allFixed || len(values) == 0 {
		return false
	}
	for _, v := range values {
		if id, ok := v.(*ast.Ident); ok && id.Name == name {
			return false
		}
		if !wrapWidthIsFixed(v, dir, consts, fn) {
			return false
		}
	}
	return true
}

func wrapCalleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func wrapExprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "?"
	}
	return buf.String()
}

// wrapWidthFindings is every fixed width in f, as "printed width expression".
func wrapWidthFindings(fset *token.FileSet, f *ast.File, dir string, consts *wrapConstIndex) []string {
	var found []string
	for _, decl := range f.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		ast.Inspect(decl, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				idx, ok := wrapWidthFuncs[wrapCalleeName(node)]
				if ok && idx < len(node.Args) && wrapWidthIsFixed(node.Args[idx], dir, consts, fn) {
					found = append(found, wrapExprString(fset, node.Args[idx]))
				}
			case *ast.CompositeLit:
				if !strings.HasSuffix(wrapExprString(fset, node.Type), "RenderInput") {
					return true
				}
				for _, el := range node.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "LineWidth" && wrapWidthIsFixed(kv.Value, dir, consts, fn) {
						found = append(found, wrapExprString(fset, kv.Value))
					}
				}
			}
			return true
		})
	}
	return found
}

type wrapGuardFile struct {
	rel  string
	dir  string
	file *ast.File
}

// wrapGuardParse parses every production Go file under internal/ and modules/.
func wrapGuardParse(t *testing.T) (*token.FileSet, []wrapGuardFile, *wrapConstIndex) {
	t.Helper()
	fset := token.NewFileSet()
	consts := newWrapConstIndex()
	var files []wrapGuardFile
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil // a syntax error is the compiler's to report
			}
			dir := filepath.ToSlash(filepath.Dir(path))
			consts.add(dir, f)
			files = append(files, wrapGuardFile{rel: filepath.ToSlash(path), dir: dir, file: f})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(files) < 1000 {
		t.Fatalf("parsed only %d Go files: the walk is not reaching the code, so this guard proves nothing", len(files))
	}
	return fset, files, consts
}

// TestNoFixedWrapWidths is the guard. Use the reader's WrapWidth, or add the
// site to wrapWidthExempt with the reason it has no reader to follow.
func TestNoFixedWrapWidths(t *testing.T) {
	fset, files, consts := wrapGuardParse(t)
	seen := map[string]bool{}
	for _, gf := range files {
		for _, width := range wrapWidthFindings(fset, gf.file, gf.dir, consts) {
			key := gf.rel + "|" + width
			seen[key] = true
			if _, ok := wrapWidthExempt[key]; !ok {
				t.Errorf("%s wraps at the fixed width %q: pass the reader's WrapWidth(), or add %q to wrapWidthExempt with the reason", gf.rel, width, key)
			}
		}
	}
	stale := []string{}
	for key := range wrapWidthExempt {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("wrapWidthExempt entry %q matches nothing: remove it", key)
	}
}

// TestOnlyWrapWidthReadsTheSavedLineWidth keeps GetLineWidth from becoming a
// second source of a wrap width.
func TestOnlyWrapWidthReadsTheSavedLineWidth(t *testing.T) {
	_, files, _ := wrapGuardParse(t)
	seen := map[string]bool{}
	for _, gf := range files {
		ast.Inspect(gf.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "GetLineWidth" {
				seen[gf.rel] = true
				if _, ok := getLineWidthAllowed[gf.rel]; !ok {
					t.Errorf("%s calls GetLineWidth; a wrap width comes from WrapWidth()", gf.rel)
				}
			}
			return true
		})
	}
	for rel := range getLineWidthAllowed {
		if !seen[rel] {
			t.Errorf("getLineWidthAllowed entry %q no longer calls GetLineWidth: remove it", rel)
		}
	}
}

// TestWrapWidthGuard_CanFail is the null probe: each shape a fixed width can
// take fires, and each reader-derived shape does not.
func TestWrapWidthGuard_CanFail(t *testing.T) {
	const header = "package p\nconst fixed = 80\nconst margin = 7\n"
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"literal", `func f(s string) { util.SplitStringNL(s, 80) }`, []string{"80"}},
		{"WrapAnsi literal", `func f(s string) { messaging.WrapAnsi(s, 72) }`, []string{"72"}},
		{"named constant", `func f(s string) { util.SplitStringNL(s, fixed) }`, []string{"fixed"}},
		{"qualified constant", `func f(s string) { util.SplitStringNL(s, p.fixed) }`, []string{"p.fixed"}},
		{"constant arithmetic", `func f(s string) { util.SplitStringNL(s, fixed-margin) }`, []string{"fixed - margin"}},
		{"constant less a length", `func f(s, i string) { wrapPrompt(s, fixed-len(i), i) }`, []string{"fixed - len(i)"}},
		{"local set to a literal", `func f(s string) { w := 80; util.SplitStringNL(s, w) }`, []string{"w"}},
		{"unqualified inside its package", `func f(s string) { SplitString(s, 80) }`, []string{"80"}},
		{"RenderInput literal", `func f() { _ = messaging.RenderInput{LineWidth: 80} }`, []string{"80"}},
		{"reader's width", `func f(s string, u *U) { util.SplitStringNL(s, u.WrapWidth()) }`, nil},
		{"parameter", `func f(s string, width int) { util.SplitStringNL(s, width) }`, nil},
		{"parameter with a floor", `func f(s string, lw int) { if lw < 20 { lw = 20 }; w := lw - 4; util.SplitStringNL(s, w-1) }`, nil},
		{"parameter less a length", `func f(s, i string, width int) { util.SplitStringNL(s, width-len(i), i) }`, nil},
		{"local from the reader", `func f(s string, u *U) { w := u.WrapWidth() - 7; util.SplitStringNL(s, w-1) }`, nil},
		{"RenderInput from the reader", `func f(u *U) { _ = messaging.RenderInput{LineWidth: u.WrapWidth()} }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "probe.go", header+tc.body+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			consts := newWrapConstIndex()
			consts.add("probe", f)
			got := wrapWidthFindings(fset, f, "probe", consts)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run**

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/guards/ -run 'TestNoFixedWrapWidths|TestOnlyWrapWidth|TestWrapWidthGuard' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: `--- PASS` for the three tests, `ok`. A red `TestNoFixedWrapWidths` names a site a task missed: fix the site.

- [ ] **Step 3: Prove it fires on the real tree**

Edit `internal/usercommands/whisper.go`: change `toUser.WrapWidth()` back to `80`. Then:

```bash
cd C:/tmp/dogmud-wrap-width && go test ./internal/guards/ -run 'TestNoFixedWrapWidths' 2>&1 | grep -E "wraps at the fixed"
```

Expected: one line naming `internal/usercommands/whisper.go` and width `"80"`. Undo with Edit and rerun Step 2.

- [ ] **Step 4: Commit**

```bash
cd C:/tmp/dogmud-wrap-width && go test . ./internal/guards/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
git add internal/guards/wrap_width_guard_test.go
git commit -F - <<'EOF'
test(guards): no fixed wrap width, and no second source of one

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `ok` for both.

---

### Task 14: Package documentation

Every symbol named below exists after Tasks 2 to 13. Run the audit at the end.

**Files:**
- Modify: `internal/users/context.md`, `internal/connections/context.md`, `internal/messaging/context.md`, `internal/menu/context.md`, `internal/rooms/context.md`, `modules/gmcp/context.md`, `internal/actions/context.md`, `internal/hooks/context.md`, `internal/guards/context.md`, `modules/aicompanion/context.md`

No edit is needed in `internal/usercommands/context.md` (its one width sentence says "the reader's width" and stays true), `internal/mobcommands`, `internal/conversation` or `internal/justice` (no API, data model or file-list change, and a grep of each for `80`, `LineWidth`, `WrapAnsi` found no stale claim).

- [ ] **Step 1: `internal/users/context.md`**

old:
```markdown
### Game menu session (#375 phase 0)
```
new:
```markdown
### Wrap width (`wrapwidth.go`)

`func (u *UserRecord) WrapWidth() int` is the only source of a wrap width.
A websocket session wraps at the smallest of `Display.GetScreenWidth()`,
`MaxAutoWrapWidth` (80) and `LineWidth` when set, never below `MinWrapWidth`
(30). Everything else, and a nil receiver, gets `GetLineWidth()`, so
`set linewidth` above 80 still works on telnet and can only narrow a web
session. `TestOnlyWrapWidthReadsTheSavedLineWidth` in `internal/guards` keeps
it the only reader of `GetLineWidth` that wraps.

### Game menu session (#375 phase 0)
```

- [ ] **Step 2: `internal/connections/context.md`**

old:
```markdown
## Connection State Management
```
new:
```markdown
`Display.ScreenWidth` comes from NAWS on telnet and, on the web, from the GMCP
`Client.Display` report (`modules/gmcp/gmcp.ClientDisplay.go`), clamped to 30
to 80 before it is stored. `RegisterTestWebsocketConnection` is the websocket
twin of `RegisterTestConnection`; its socket was never dialled, so a test must
not write to it or close it.

## Connection State Management
```

- [ ] **Step 3: `internal/messaging/context.md`**

old:
```markdown
6. **Wrap** at recipient's `UserRecord.LineWidth` (default 80, range
   40-240), ANSI-aware, but only for the narration categories
   `shouldWrap` admits (48 of the 64 `Category` values as of this
   writing, `CategoryLight` among them: lighting plan 3d's transition
   notices wrap exactly as `CategoryTimeOfDay` does; `CategoryCraft`,
   #515, the crafter's own work lines; and `CategoryCraftSeen`, the
   room's lines about a crafter). Pre-formatted
```
new:
```markdown
6. **Wrap** at recipient's `UserRecord.WrapWidth()`, ANSI-aware, but
   only for the categories `shouldWrap` admits (52 of the 70 `Category`
   values as of this writing, `CategoryLight` among them: lighting plan
   3d's transition notices wrap exactly as `CategoryTimeOfDay` does;
   `CategoryCraft`, #515, the crafter's own work lines;
   `CategoryCraftSeen`, the room's lines about a crafter;
   `CategorySpeech`, `CategoryShout` and `CategoryEmote`, sent to a
   whole room so no sender may wrap them first; and `CategoryRoomIdle`,
   a room's idle flavor line, colored and normalised as
   `CategoryRoomDescription` is). Pre-formatted
```

old:
```markdown
   side-by-side minimap block (`RoomDescription`), categories that
   already wrap themselves at a hardcoded 80 (`Speech`, `Whisper`,
   `Shout`, `Emote`), system output owned by a later stage (`Error`,
```
new:
```markdown
   side-by-side minimap block (`RoomDescription`), `Whisper`, which
   each sender wraps to its one reader's `WrapWidth()`, system output
   owned by a later stage (`Error`,
```

old: `whether it wraps at 80 (`CategorySystem` never wraps, which is why #449`
new: `whether it wraps (`CategorySystem` never wraps, which is why #449`

- [ ] **Step 4: `internal/menu/context.md`**

old:
```markdown
  2: the offer card's objective and reward). Each is wrapped at 80 columns on
  its own line(s); `Render` puts a blank line after the heading and one after
```
new:
```markdown
  2: the offer card's objective and reward). Each is wrapped to `Render`'s
  width on its own line(s); `Render` puts a blank line after the heading and one after
```

old:
```markdown
`Render(m)` draws at the 80-column wrap. Callers send the result on
```
new:
```markdown
`Render(m, width)` draws for a reader `width` columns wide, always
`UserRecord.WrapWidth()` in production. Callers send the result on
```

old:
```markdown
- `Detail` wraps at 80 columns under the label, indented past the marker
```
new:
```markdown
- `Detail` wraps to the width under the label, indented past the marker
```

- [ ] **Step 5: `internal/rooms/context.md`**

old:
```markdown
## Special Considerations
```
new:
```markdown
## Width

Every sender passes `UserRecord.WrapWidth()` as the pipeline's width.
`GetDetails` wraps the description to the viewer's `WrapWidth()`, and to
that less the minimap's 7 columns beside a minimap. `GetDescriptionFormatted`
uses the width it is given.

## Special Considerations
```

- [ ] **Step 6: `modules/gmcp/context.md`**

old:
```markdown
`gmcp.Zone.go`, no payload) all do.
```
new:
```markdown
`gmcp.Zone.go`, no payload) and `GMCPClientDisplay` / `clientDisplayHandler`
  (`gmcp.ClientDisplay.go`) all do.
```

old:
```markdown
| `gmcp.Zone.go` | `Zone` |
```
new:
```markdown
| `gmcp.ClientDisplay.go` | `Client.Display` (inbound) | A web client's `{"cols": n}`. Websocket only, whole numbers only, clamped to `users.MinWrapWidth`..`users.MaxAutoWrapWidth`, written to `Display.ScreenWidth` only when it changes |
| `gmcp.Zone.go` | `Zone` |
```

- [ ] **Step 7: `internal/actions/context.md`**

old:
```markdown
`emote.go` has two formatters. `FormatEmoteText` hard-wraps at 80 with CRLF,
for the player `CategoryEmote`, which the messaging pipeline does not wrap.
`FormatMobEmoteText` is the same line unwrapped, for a mob's
`CategoryMobEmote`, which the pipeline does wrap at each reader's width.
Using the pre-wrapped one for a mob wrapped the line twice and stranded a
word on its own line (#430).
```
new:
```markdown
`emote.go` has one formatter body. `FormatEmoteText` builds the line and
`FormatMobEmoteText` is it with the mob's name color. Neither wraps:
`CategoryEmote` and `CategoryMobEmote` both wrap at each reader's width in the
pipeline, as `FormatSayText` and `formatShoutText` leave `CategorySpeech` and
`CategoryShout` to. A formatter that wrapped would wrap the line twice and
strand a word on its own line (#430). A line for one actor alone wraps at
`actorWrapWidth`.
```

- [ ] **Step 8: `internal/hooks/context.md`**

old:
```markdown
`Message_SendMessages.go` is the one place a queued `events.Message` becomes
delivered text; both of its branches gate on
```
new:
```markdown
`Message_SendMessages.go` is the one place a queued `events.Message` becomes
delivered text. Player chatter on its `RoomId` branch arrives unrendered, so
`roomMessageTextFor` wraps it there for each reader. Both of its branches gate on
```

old:
```markdown
  (budget, and a one-line room shows its line once per visit).
```
new:
```markdown
  (budget, and a one-line room shows its line once per visit). The line rides
  `messaging.CategoryRoomIdle`, so the pipeline wraps it for each reader.
```

- [ ] **Step 9: `internal/guards/context.md`**

old:
```markdown
  `messaging_surface_guard_test.go`, the `*_wrapper_guard_test.go` files
```
new:
```markdown
  `messaging_surface_guard_test.go`, `wrap_width_guard_test.go` (no fixed wrap
  width in production Go, and `WrapWidth` the only wrap reader of the saved
  `LineWidth`), the `*_wrapper_guard_test.go` files
```

- [ ] **Step 10: `modules/aicompanion/context.md`**

old:
```markdown
  Its lines are wrapped at 80 columns before sending, since the system
```
new:
```markdown
  Its lines are wrapped to the reader's width before sending, since the system
```

- [ ] **Step 11: Verify every symbol named, and commit**

`tools/context_md_audit.py` reads only fenced Go blocks, and these edits add none, so check the names directly.

```bash
cd C:/tmp/dogmud-wrap-width && for s in WrapWidth MinWrapWidth MaxAutoWrapWidth RegisterTestWebsocketConnection GMCPClientDisplay clientDisplayHandler CategoryRoomIdle roomMessageTextFor actorWrapWidth formatShoutText FormatEmoteText FormatMobEmoteText; do printf "%s %s\n" "$s" "$(grep -rl "$s" internal modules --include=*.go | wc -l)"; done
```

Expected: every line ends in a number above 0.

```bash
cd C:/tmp/dogmud-wrap-width
git add internal/users/context.md internal/connections/context.md internal/messaging/context.md internal/menu/context.md internal/rooms/context.md \
  modules/gmcp/context.md internal/actions/context.md internal/hooks/context.md internal/guards/context.md modules/aicompanion/context.md
git commit -F - <<'EOF'
docs: context.md for the wrap width work

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 15: Ship

**Files:** the spec, this plan and `docs/README.md`, copied from the main checkout if they are not yet on `origin/master`. No `PATCH_NOTES` entry: no client sends a width yet, so this ships as groundwork.

- [ ] **Step 1: Bring the branch up to date**

```bash
cd C:/tmp/dogmud-wrap-width && git fetch origin master && git rebase origin/master && git log --oneline origin/master..HEAD | wc -l
```

Expected: the rebase succeeds and the count is 13, one commit for each of Tasks 2 to 14.

- [ ] **Step 2: Add the spec and plan if `origin/master` lacks them**

```bash
cd C:/tmp/dogmud-wrap-width
MSYS_NO_PATHCONV=1 git cat-file -e origin/master:docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md && echo ON_MASTER || echo MISSING
```

If `MISSING`, copy them and commit:

```bash
cd C:/tmp/dogmud-wrap-width
M="C:/Users/Calabe Davis/workspace/DOGMud"
cp "$M/docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md" docs/superpowers/specs/
cp "$M/docs/superpowers/plans/2026-11-09-webclient-base-1-server-wrap-width.md" docs/superpowers/plans/
cp "$M/docs/README.md" docs/README.md
git diff --stat -- docs/README.md
git add docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md docs/superpowers/plans/2026-11-09-webclient-base-1-server-wrap-width.md docs/README.md
git commit -F - <<'EOF'
docs: web client base spec and plan 1

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: the `docs/README.md` diff is the spec's row and the plan's row, 2 added lines. If it shows anything else, stop and reconcile by hand.

- [ ] **Step 3: Pre-push gate**

```bash
cd C:/tmp/dogmud-wrap-width
gofmt -l internal/ modules/
go build ./...
go test . ./internal/guards/ ./internal/users/ ./internal/connections/ ./internal/messaging/ ./internal/menu/ ./internal/rooms/ ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/hooks/ ./internal/conversation/ ./internal/justice/ ./internal/prompt/ ./modules/gmcp/ ./modules/aicompanion/ ./modules/auctions/ 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
golangci-lint run --new-from-merge-base=origin/master
grep -n "LogToFile" _datafiles/config.yaml
```

Expected: `gofmt -l` prints nothing, every package `ok`, lint `0 issues`, `LogToFile: false`.

- [ ] **Step 4: Isolated boot check**

Set `S` to the Scratchpad directory your system prompt names. Build to the fixed path so the firewall rule applies.

```bash
S="<the Scratchpad directory named in your system prompt, full path>"
cd C:/tmp/dogmud-wrap-width
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
```

Write `$S/boot-override.yaml` with the Write tool:

```yaml
Network:
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
  AIPort: 45555
```

```bash
S="<the Scratchpad directory named in your system prompt, full path>"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
```

Expected: `exit 124`, then `0`, then `1`. Remove the worktree: `git worktree remove --force C:/tmp/dogmud-boot-check` (if Windows holds a lock, PowerShell `Remove-Item -Recurse -Force`, then `git worktree prune`).

- [ ] **Step 5: Push and open the PR**

```bash
cd C:/tmp/dogmud-wrap-width && git push -u origin feature/webclient-base-1-wrap-width
gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-1-wrap-width --title "Web client base 1: one server wrap width" --body-file - <<'EOF'
Part of #783. Plan 1 of the web client base.

`UserRecord.WrapWidth` is now the only source of a wrap width, and a web client can report its columns with GMCP `Client.Display`. No client sends that yet.

What a player can notice:
- A player with a saved `set linewidth` other than 80 now gets it on speech, shouts, emotes, whispers, room descriptions, menus and the `set` screen, which ignored it before. On the web a saved width above 80 is capped at 80.
- At 80 columns the words are unchanged. The last word or two of a long say, shout or emote can break differently: `TestWrapGolden_AcceptedDifferences` names each case.

Guards: `TestNoFixedWrapWidths` and `TestOnlyWrapWidthReadsTheSavedLineWidth`.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

Expected: the printed URL says `pruuk/DOGMud`. If it does not, stop and close it at once.

- [ ] **Step 6: Watch the checks**

```bash
gh pr checks <the PR number gh printed> --repo pruuk/DOGMud --watch
gh run list --repo pruuk/DOGMud --branch feature/webclient-base-1-wrap-width --limit 5
```

Expected: every expected workflow appears in the run list and is green. Leave the merge to the controller, and remove `C:/tmp/dogmud-wrap-width` with `git worktree remove` after it merges.

---

## As built

Review and local testing added work beyond Tasks 1 to 15, on the same branch:
- `WrapAnsi` does constant work at a stray less-than sign, counts display columns, and leaves any line that fits untouched.
- `SplitString` flushes the current line before an over-long word, so fact 30 and accepted difference 1 above describe the old order.
- Menu footers, the fprompt screen and room alert centering follow the reader's width. The width guard covers more forms of a fixed width.
- A local test showed mid-word breaks in text that was never wrapped, so System, Warning, Error, Refusal, Usage, Group and Broadcast joined `shouldWrap`, and their pre-wraps went.
- Fixed-width tables and boxes sent on those categories moved to `CategoryScreen`, held by a guard, so they keep their columns.
- That fix is visible to players, so this shipped with a `PATCH_NOTES` entry.

## Self-review

**Spec coverage.** `WrapWidth` rules: Task 2. `Client.Display`: Task 3. The 8 pipeline sites: Task 4. Speech, Shout, Emote and the corrected comment: Task 5. Literal 80s and known-reader text: Task 8. `set.go`, `roomdetails.go`, `menu.go`: Tasks 12, 10, 11. Authoring limits stay: fact 25, and they do not reach a wrap function. Telnet golden: Task 5, reported honestly against the spec's wording in "What the telnet golden test can and cannot claim". Width guard: Task 13. `set linewidth` above 80 on telnet and narrowing on the web: `TestWrapWidth` rows. Column reporting and `ClearTemplateConfigCache` belong to plans 5 and 3; templates' `splitstring` widths to plan 3.

**Placeholder scan.** The only angle-bracket runtime values are the scratchpad path and the PR number in Task 15, which exist only at execution. Every code step shows the code.

**Type consistency.** `MinWrapWidth`, `MaxAutoWrapWidth`, `WrapWidth`, `RegisterTestWebsocketConnection`, `GMCPClientDisplay`, `clientDisplayHandler`, `registerClientDisplayListener`, `reportedCols`, `formatShoutText`, `actorWrapWidth`, `roomMessageTextFor`, `CategoryRoomIdle`, `Render(m, width)` and the test helpers `wrapGoldenVisible`, `renderAt80`, `menuTestWidth`, `wrapTestLines` are spelled the same wherever used. `wrapGoldenVisible` is defined in Task 5's golden file and used by that task's per-recipient file in the same package.
