# Quest Arc Phase 0: Menu Primitive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One menu primitive (`internal/menu`) that every fixed choice in DOGMud renders and resolves through, with non-modal game menus that close on interrupts, modal builder menus, follow-pull lines, typed-only answers, and web client automation that pauses while a menu is open and never fires during login.

**Architecture:** A pure core (`internal/menu`: entries, render, resolve, confirm, lint, interrupt and follow lines) with two thin input layers: a game menu session on `UserRecord` (callbacks, interrupt gate, non-modal routing) and menu questions inside `prompt` (builders stay modal, player flows opt into non-modal). Interrupts are pushed by state-machine observers, the condition-apply hook and a `RoomChange` listener; every choice re-checks the gate. The server tells the web client when a menu or prompt opens (`Char.Menu`), and the client queues automation until it closes.

**Tech Stack:** Go 1.x (module `github.com/GoMudEngine/GoMud`), the `internal/state` FSM framework, GMCP over websocket, vanilla JS in `_datafiles/html/public/webclient-pure.html`, Node-based JS tests in `tools/jstest/`.

**Spec:** `docs/superpowers/specs/2026-10-20-quest-arc-phase0-menu-primitive-design.md`. Arc target: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`.

---

## Facts verified against source (master `a3b82e5ac`, 2026-10-20)

Every task re-checks the lines it edits before editing; line numbers drift.

| Fact | Where |
|---|---|
| `processInput(userId, inputText, flags)` gives every line to an open prompt question first (`activeQuestion.Answer`), then re-runs `cmdPrompt.Command + " " + cmdPrompt.Rest` | `world.go:967-990` |
| Typed input from telnet and websocket goes through `World.SendInput` into `worldInput`; the input worker turns it into `events.Input` with no flags | `main.go:1079,1257,1521`, `world.go:250,955-961` |
| Macro expansion queues `events.Input` per macro command | `world.go:1010-1030` |
| Other `events.Input` producers: `UserRecord.Command` / `CommandFlagged` (`userrecord.go:382,409`), on-login commands (`hooks/PlayerSpawn_HandleJoin.go:304`), admin `build` looks (`admin.build.go:48,139`), admin force-command (`admin.command.go:56,64`), mobs (`mobs/mobs.go:956`) | grep |
| Event flags end at `CmdFollowStep EventFlag = 0b00100000` | `internal/events/const.go:20-31` |
| `UserRecord.activePrompt` sits under `sessionMu`; `StartPrompt`, `GetPrompt`, `ClearPrompt` | `internal/users/userrecord.go:62-78,784-815` |
| `GetCommandPrompt` prints `activeQuestion.String()` | `internal/users/userrecord.prompt.go:287-297` |
| `prompt.Ask` dedups questions by question text; `Answer` does closest-prefix option matching | `internal/prompt/prompt.go:58-93,127-183` |
| `presence` and `perception` expose `RegisterObserver(name, func(from, to State, r state.TransitionReason))`; `combatphase`, `activity`, `life`, `position`, `control` keep `inner *state.Machine[State]` private | `presence.go:169`, `perception.go:104`, `combatphase.go:121`, `activity.go:88`, `life.go:59`, `position.go:115`, `control.go:77` |
| `state.Machine.Subscribe` has no unsubscribe; observers run after the machine unlocks | `internal/state/machine.go:72-77,108-140` |
| Character machines are built in `NewCharacter` (`character.go:423-430`) and filled in `Validate()` when nil (`validate.go:653-689`); `Control` is NOT rebuilt in `Validate` (nil after a load) | source |
| `Character.GetUserId()` exists | `internal/characters/character.go:508` |
| Predicates: `IsInCombat` (`character.go:785`), `IsAlive` (`character.go:925`), `IsActing` (`spells.go:53`), `IsStanding` (`position_predicates.go:16`), `IsBeingControlled` (`position_predicates.go:186`), `HasConditionFlag` (`conditions.go:19`), `HasCondition` (`conditions.go:141`) | source |
| Sleeping is `conditions.Sleeping`; quitting is condition 0 ("Meditating", `conditions/0-meditating.yaml`) queued by `quit` | `conditionspec.go:65`, `quit.go:16` |
| A condition lands in `hooks.ApplyConditions` after `addErr == nil` and `Conditions.Stamp` | `internal/hooks/Condition_ApplyConditions.go:121-134` |
| `RoomChange` listeners are registered in `hooks.RegisterListeners` | `internal/hooks/hooks.go:40-48` |
| Follow moves: follow module `user.Command(followExitName, .25)`; party follow `partyUser.CommandFlagged(rest, events.CmdFollowStep)` | `modules/follow/follow.go:425`, `internal/usercommands/go.go:390` |
| Sight helpers: `messaging.HideNames(text, names, d)`, `messaging.ParticipantSight(observer, room)`, `room.SendTextVisualHidingNames(cat, txt, names, exclude...)` | `messaging/hidenames.go:67`, `messaging/predicates.go:56`, `rooms/rooms.go:288` |
| `messaging.CategoryScreen` is the category for laid-out screens: normalize skipped, never wrapped, blank lines allowed | `internal/messaging/messaging.go:155-163` |
| 19 `numbered-list` renders: `admin.item.go:195,216,344,371`; `admin.room.exits.go:65`; `admin.room.go:147,412,443,474,586,770,799,830`; `admin.server.go:310,408`; `admin.spell.go:72`; `admin.zone.go:140,173,204` | grep |
| 6 menu-index parses (the `restNum > 0 && restNum <= len(...)` block): `admin.item.go:201,356`, `admin.room.go:423,592,779`, `admin.zone.go:149`. Containers and exits parse the same way at `admin.room.go:154` and `admin.room.exits.go:72` | grep |
| 27 fixed-option `prompt.Ask` calls (listed in Task 15) | grep |
| Web client: ticks `rebuildTickTimers` (`webclient-pure.html:1393-1421`), queue block from `var actionQueue = [];` (1513) to `function runTriggers(` (1707), `runTriggers` (1707-1726), `"Char.Automation"` handler (816), `"Commands.State"` handler (829), `socket.onclose` (2582); GMCP dispatch picks the longest matching handler path (2176-2183) | source |
| JS tests extract real code from `webclient-pure.html` and `eval` it; CI runs every `tools/jstest/*.test.js` | `tools/jstest/webclient-action-queue.test.js`, `.github/workflows/validate.yml:90` |
| `Char.Action.Try` runs `u.Command(req.Cmd)` and replies `fired`/`deferred`/`rejected` | `modules/gmcp/gmcp.CharOp.go:85-127` |
| `GMCPOut{UserId, Module, Payload any}`; `isGMCPEnabled(connId)` | `modules/gmcp/gmcp.go:92`, `gmcp.Automation.go:117` |
| `users.NewTestUser(userId, username, charName, connId)` and `users.SeedUsersForTest(map)` | `internal/users/test_helpers.go:29,56` |

## Rules for every task

- Edit files with the Edit and Write tools only. No `sed -i`, no Python file rewrites, no `>` into tracked files (enforced by `~/.claude/hooks/guard.py`).
- `git add <named paths>` only. Never `-A`, `.`, or `commit -a`. Never `--no-verify`.
- Commit messages through a quoted heredoc, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes in any player-facing text or doc prose.
- Test binaries load Go config defaults, not `config.yaml` (`dogmud-writing-tests`). Nothing in this plan reads a balance knob.
- Run `gofmt -l internal/ modules/` before each commit; it must print nothing.

## File structure

| File | Responsibility |
|---|---|
| Create `internal/menu/menu.go` | `Entry`, `Menu`, `Render`, column layout, word wrap |
| Create `internal/menu/resolve.go` | `ResultKind`, `Result`, `Resolve`, `Confirm` |
| Create `internal/menu/lines.go` | `CloseReason`, `InterruptLine`, `FollowLines`, `Session` |
| Create `internal/menu/lint.go` | `LintLabels` (menu language conventions) |
| Create `internal/menu/*_test.go` | core unit tests |
| Create `internal/menu/context.md` | package doc |
| Modify `internal/events/const.go`, `internal/events/eventtypes.go` | `CmdTyped` flag, `MenuChanged` event |
| Modify `world.go` | set `CmdTyped`; route typed lines to menus; non-modal prompts; queued lines never answer |
| Modify 5 machines in `internal/state/{combatphase,activity,life,position,control}` | `RegisterObserver` passthroughs |
| Create `internal/characters/busy_observers.go`; modify `character.go`, `validate.go` | wire observers once per machine, notify `BusyStateChanged` |
| Create `internal/users/userrecord.menu.go`, `internal/users/menu_gate.go` | game menu session, gate, routing, `MenuChanged` |
| Modify `internal/users/userrecord.go` | `activeMenu` field; prompt open/close notify |
| Modify `internal/prompt/prompt.go` | `Question.Menu`, `Prompt.NonModal`, `AskMenu`, `AskConfirm`, `Resolve` |
| Modify `internal/hooks/Condition_ApplyConditions.go`, `internal/hooks/hooks.go`; create `internal/hooks/RoomChange_MenuClose.go` | interrupt pushes |
| Create `internal/actions/menu_follow.go`; modify `modules/follow/follow.go`, `internal/usercommands/go.go` | follow-pull lines |
| Create `modules/gmcp/gmcp.Menu.go`; modify `modules/gmcp/gmcp.CharOp.go` | `Char.Menu` push; defer queued actions while a menu is open |
| Modify `_datafiles/html/public/webclient-pure.html` | queue fixes, pause, login gate |
| Modify `tools/jstest/webclient-action-queue.test.js`; create `tools/jstest/webclient-automation-pause.test.js` | JS tests |
| Create `internal/usercommands/menu_options.go`; modify the admin builders, confirm sites, `character.go`, `start.go`, `picklock.go`, `modules/auctions/auctions.go` | migration |
| Delete `_datafiles/world/{dogmud,default}/templates/tables/numbered-list*.template` | retired templates |
| Create `internal/usercommands/menu_guard_test.go` | guard test |

---

### Task 0: Branch

- [ ] **Step 1: Create the feature branch from master**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git switch -c feat/375-phase0-menu origin/master
git status --short
```
Expected: clean except the untracked files already present on master (`.agents/skills/...`, `Pre-deploy Notes.md`). Do not add them.

---

### Task 1: Menu core, rendering

**Files:**
- Create: `internal/menu/menu.go`
- Test: `internal/menu/menu_test.go`

- [ ] **Step 1: Write the failing test**

```go
package menu

import (
	"regexp"
	"strings"
	"testing"
)

var tagRe = regexp.MustCompile(`<[^>]+>`)

func plain(s string) string { return tagRe.ReplaceAllString(s, "") }

func talkMenu() Menu {
	return Menu{
		Heading: `Kael sets down his whittling. "Ask what you like, I'll answer plain."`,
		Entries: []Entry{
			{Id: "feud", Marker: MarkerQuest, Label: "The Feud at the Well", Detail: "Do you want help settling the feud at the well?"},
			{Id: "pass", Marker: MarkerInfo, Label: "How do I reach the high pass?"},
			{Id: "carve", Label: "What do you carve?", Dim: true},
		},
		Leave: "Leave",
	}
}

func TestRender_TalkMenuLayout(t *testing.T) {
	got := plain(Render(talkMenu()))
	want := strings.Join([]string{
		`Kael sets down his whittling. "Ask what you like, I'll answer plain."`,
		``,
		`  1. [!] The Feud at the Well`,
		`         Do you want help settling the feud at the well?`,
		`  2. [*] How do I reach the high pass?`,
		`  3.     What do you carve?`,
		`  4.     Leave`,
		``,
		`Type a number, or press Enter to leave.`,
	}, "\n")
	if got != want {
		t.Fatalf("render mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRender_DimEntryIsGreyAndOthersAreNot(t *testing.T) {
	out := Render(talkMenu())
	if !strings.Contains(out, `<ansi fg="`+dimColor+`">What do you carve?</ansi>`) {
		t.Fatalf("dim entry not grey: %s", out)
	}
	if strings.Contains(out, `<ansi fg="`+dimColor+`">How do I reach`) {
		t.Fatalf("undimmed entry rendered grey: %s", out)
	}
}

func TestRender_DetailWrapsAtLabelIndentWithin80(t *testing.T) {
	m := Menu{Entries: []Entry{{Id: "a", Label: "A", Detail: strings.Repeat("word ", 30)}}}
	for _, line := range strings.Split(plain(Render(m)), "\n") {
		if len(line) > 80 {
			t.Fatalf("line over 80 columns (%d): %q", len(line), line)
		}
	}
	lines := strings.Split(plain(Render(m)), "\n")
	if !strings.HasPrefix(lines[1], "         word") || !strings.HasPrefix(lines[2], "         word") {
		t.Fatalf("detail not wrapped at the label indent: %q", lines[1:3])
	}
}

func TestRender_NumbersRightAlignPastNine(t *testing.T) {
	m := Menu{}
	for i := 0; i < 10; i++ {
		m.Entries = append(m.Entries, Entry{Id: string(rune('a' + i)), Label: "x"})
	}
	lines := strings.Split(plain(Render(m)), "\n")
	if lines[0] != "   1.     x" || lines[9] != "  10.     x" {
		t.Fatalf("alignment wrong: %q / %q", lines[0], lines[9])
	}
}

func TestRender_TwoColumns(t *testing.T) {
	m := Menu{Columns: 2, Entries: []Entry{
		{Id: "a", Label: "alpha"}, {Id: "b", Label: "beta", Marker: MarkerChecked}, {Id: "c", Label: "gamma"},
	}}
	lines := strings.Split(plain(Render(m)), "\n")
	if !strings.HasPrefix(lines[0], "  1.     alpha") || !strings.Contains(lines[0], "2. [x] beta") {
		t.Fatalf("row 1 wrong: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  3.     gamma") {
		t.Fatalf("row 2 wrong: %q", lines[1])
	}
}

func TestRender_FooterVariants(t *testing.T) {
	noLeave := Menu{Entries: []Entry{{Id: "a", Label: "A"}}}
	if !strings.HasSuffix(plain(Render(noLeave)), "Type a number.") {
		t.Fatalf("no-leave footer: %q", plain(Render(noLeave)))
	}
	withDefault := Menu{Entries: []Entry{{Id: "a", Label: "A"}, {Id: "b", Label: "B"}}, Default: "b"}
	if !strings.HasSuffix(plain(Render(withDefault)), "Type a number, or press Enter for 2.") {
		t.Fatalf("default footer: %q", plain(Render(withDefault)))
	}
	custom := Menu{Entries: []Entry{{Id: "a", Label: "A"}}, Footer: "Pick one."}
	if !strings.HasSuffix(plain(Render(custom)), "Pick one.") {
		t.Fatalf("custom footer: %q", plain(Render(custom)))
	}
}

func TestRender_EmptyMenuStillShowsHeadingAndFooter(t *testing.T) {
	got := plain(Render(Menu{Heading: "Nothing here."}))
	if got != "Nothing here.\n\nType a number." {
		t.Fatalf("empty menu: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/menu/ -run TestRender -v`
Expected: FAIL, `undefined: Menu` (package does not exist yet).

- [ ] **Step 3: Write the implementation**

```go
// Package menu is DOGMud's one menu primitive: a numbered list of choices,
// how it looks, and how a typed line resolves against it. It holds no session
// state and does no I/O; internal/users (game menus) and internal/prompt
// (builder and player flows) supply the input sessions.
package menu

import (
	"fmt"
	"strconv"
	"strings"
)

// Markers carried in the 4-wide marker column. The symbol carries the
// meaning; colour only reinforces it (owner is green/blue colourblind).
const (
	MarkerQuest   = "[!]" // new quest
	MarkerHandIn  = "[?]" // quest ready to hand in
	MarkerInfo    = "[*]" // writer-flagged info not yet heard
	MarkerChecked = "[x]" // a builder toggle that is selected
)

const (
	lineWidth    = 80
	numberColor  = "command"
	markerColor  = "yellow-bold"
	dimColor     = "8"
	columnWidth  = 39
	leftMargin   = "  "
	markerColumn = 4
)

// Entry is one numbered choice.
type Entry struct {
	Id      string   // returned on choice; also matched when typed exactly
	Marker  string   // "", or one of the Marker constants
	Label   string   // natural language, one line
	Detail  string   // optional second line, indented under the label
	Dim     bool     // rendered grey: heard info, small talk
	Aliases []string // extra exact words that pick this entry ("y", "n")
}

// Menu is a heading, numbered entries, and an optional final Leave entry.
type Menu struct {
	Heading    string
	Entries    []Entry
	Leave      string // label of the final numbered leave entry; "" for none
	Default    string // Id chosen on Enter; "" for none
	Footer     string // overrides the computed footer
	Columns    int    // 2 lays name-only lists side by side (long admin lists)
	Prefix     bool   // a unique label prefix resolves (builders only)
	AcceptText bool   // an unmatched line resolves as Text (free-text builders)
}

// all returns the entries as numbered, Leave included.
func (m Menu) all() []Entry {
	if m.Leave == "" {
		return m.Entries
	}
	out := make([]Entry, 0, len(m.Entries)+1)
	out = append(out, m.Entries...)
	return append(out, Entry{Label: m.Leave})
}

// Render draws the menu at the 80-column wrap. Callers send it on
// messaging.CategoryScreen (player menus) or CategoryAdmin (builders).
func Render(m Menu) string {
	var b strings.Builder
	if m.Heading != "" {
		b.WriteString(m.Heading)
		b.WriteString("\n\n")
	}
	entries := m.all()
	if len(entries) > 0 {
		if m.Columns == 2 {
			renderColumns(&b, entries)
		} else {
			numW := len(strconv.Itoa(len(entries)))
			for i, e := range entries {
				renderEntry(&b, i+1, numW, e)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(footer(m))
	return b.String()
}

func numberText(n, numW int) string {
	return fmt.Sprintf(`<ansi fg="%s">%*d.</ansi>`, numberColor, numW, n)
}

func markerText(marker string) string {
	if marker == "" {
		return strings.Repeat(" ", markerColumn)
	}
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, markerColor, marker) +
		strings.Repeat(" ", markerColumn-len(marker))
}

func labelText(e Entry) string {
	if e.Dim {
		return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, dimColor, e.Label)
	}
	return e.Label
}

func renderEntry(b *strings.Builder, n, numW int, e Entry) {
	b.WriteString(leftMargin)
	b.WriteString(numberText(n, numW))
	b.WriteString(" ")
	b.WriteString(markerText(e.Marker))
	b.WriteString(labelText(e))
	b.WriteString("\n")
	if e.Detail == "" {
		return
	}
	indent := strings.Repeat(" ", len(leftMargin)+numW+2+markerColumn)
	for _, line := range wrap(e.Detail, lineWidth-len(indent)) {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func renderColumns(b *strings.Builder, entries []Entry) {
	numW := len(strconv.Itoa(len(entries)))
	labelW := columnWidth - numW - 2 - markerColumn
	for i, e := range entries {
		if i%2 == 0 {
			b.WriteString(leftMargin)
		}
		label := e.Label
		if len(label) > labelW {
			label = label[:labelW]
		}
		cell := numberText(i+1, numW) + " " + markerText(e.Marker) + labelText(Entry{Label: label, Dim: e.Dim})
		b.WriteString(cell)
		if i%2 == 0 && i+1 < len(entries) {
			b.WriteString(strings.Repeat(" ", columnWidth-(numW+2+markerColumn+len(label))))
		} else {
			b.WriteString("\n")
		}
	}
}

func footer(m Menu) string {
	if m.Footer != "" {
		return m.Footer
	}
	if m.Default != "" {
		for i, e := range m.Entries {
			if e.Id == m.Default {
				return fmt.Sprintf("Type a number, or press Enter for %d.", i+1)
			}
		}
	}
	if m.Leave != "" {
		return "Type a number, or press Enter to leave."
	}
	return "Type a number."
}

// wrap splits text into lines of at most width visible characters on word
// boundaries. A single word longer than width gets a line of its own.
func wrap(text string, width int) []string {
	var lines []string
	var cur strings.Builder
	for _, word := range strings.Fields(text) {
		if cur.Len() > 0 && cur.Len()+1+len(word) > width {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteString(" ")
		}
		cur.WriteString(word)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/menu/ -run TestRender -v`
Expected: all `TestRender_*` PASS. If `TestRender_TwoColumns` fails on spacing, fix `renderColumns` (not the test): column 2 must start at visible column `2 + 39`.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ modules/
git add internal/menu/menu.go internal/menu/menu_test.go
git commit -F - <<'EOF'
feat(menu): menu core and rendering (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Resolve and Confirm

**Files:**
- Create: `internal/menu/resolve.go`
- Test: `internal/menu/resolve_test.go`

- [ ] **Step 1: Write the failing test**

```go
package menu

import "testing"

func TestResolve(t *testing.T) {
	m := talkMenu()
	cases := []struct {
		name  string
		input string
		kind  ResultKind
		id    string
	}{
		{"number", "2", Chosen, "pass"},
		{"number with spaces", "  1 ", Chosen, "feud"},
		{"leave by number", "4", Leave, ""},
		{"out of range high", "5", NoSuchNumber, ""},
		{"zero", "0", NoSuchNumber, ""},
		{"enter leaves", "", Leave, ""},
		{"exact label", "what do you carve?", Chosen, "carve"},
		{"exact id", "feud", Chosen, "feud"},
		{"exact leave label", "leave", Leave, ""},
		{"prefix is not an answer in a game menu", "how", NotAnAnswer, ""},
		{"a command is not an answer", "north", NotAnAnswer, ""},
		{"l is look, not leave", "l", NotAnAnswer, ""},
	}
	for _, c := range cases {
		r := Resolve(m, c.input)
		if r.Kind != c.kind {
			t.Errorf("%s: kind %v, want %v", c.name, r.Kind, c.kind)
			continue
		}
		if c.id != "" && (r.Entry == nil || r.Entry.Id != c.id) {
			t.Errorf("%s: entry %+v, want id %q", c.name, r.Entry, c.id)
		}
	}
}

func TestResolve_PrefixInBuilderMenus(t *testing.T) {
	m := Menu{Prefix: true, Entries: []Entry{{Id: "weapon", Label: "weapon"}, {Id: "wearable", Label: "wearable"}, {Id: "food", Label: "food"}}}
	if r := Resolve(m, "fo"); r.Kind != Chosen || r.Entry.Id != "food" {
		t.Fatalf("unique prefix: %+v", r)
	}
	r := Resolve(m, "we")
	if r.Kind != Ambiguous || len(r.Matches) != 2 {
		t.Fatalf("ambiguous prefix: %+v", r)
	}
}

func TestResolve_EnterPicksDefaultBeforeLeave(t *testing.T) {
	m := Menu{Entries: []Entry{{Id: "a", Label: "A"}, {Id: "b", Label: "B"}}, Default: "b", Leave: "Cancel"}
	if r := Resolve(m, ""); r.Kind != Chosen || r.Entry.Id != "b" {
		t.Fatalf("enter with default: %+v", r)
	}
	if r := Resolve(Menu{Entries: []Entry{{Id: "a", Label: "A"}}}, ""); r.Kind != Empty {
		t.Fatalf("enter with neither: %+v", r)
	}
}

func TestResolve_AcceptText(t *testing.T) {
	m := Menu{AcceptText: true, Entries: []Entry{{Id: "server", Label: "server"}}}
	if r := Resolve(m, "server.motd"); r.Kind != Text || r.Text != "server.motd" {
		t.Fatalf("free text: %+v", r)
	}
	if r := Resolve(m, "1"); r.Kind != Chosen {
		t.Fatalf("number still chooses: %+v", r)
	}
}

func TestConfirm(t *testing.T) {
	m := Confirm(`Disband "Iron Wolves"? This cannot be undone.`, "Yes, disband it", "No, keep it")
	for in, want := range map[string]string{"1": "yes", "2": "no", "": "no", "y": "yes", "yes": "yes", "n": "no", "no": "no", "Yes, disband it": "yes"} {
		r := Resolve(m, in)
		if r.Kind != Chosen || r.Entry.Id != want {
			t.Errorf("confirm %q: %+v, want %s", in, r, want)
		}
	}
	if r := Resolve(m, "north"); r.Kind != NotAnAnswer {
		t.Errorf("confirm north: %+v", r)
	}
	if m.Leave != "" {
		t.Errorf("a confirm has no Leave entry")
	}
	if plain(Render(m)) != "Disband \"Iron Wolves\"? This cannot be undone.\n\n  1.     Yes, disband it\n  2.     No, keep it\n\nType a number, or press Enter for 2." {
		t.Errorf("confirm render: %q", plain(Render(m)))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/menu/ -run 'TestResolve|TestConfirm' -v`
Expected: FAIL, `undefined: Resolve`.

- [ ] **Step 3: Write the implementation**

```go
package menu

import (
	"strconv"
	"strings"
)

// ResultKind says what a typed line meant to a menu.
type ResultKind int

const (
	NotAnAnswer  ResultKind = iota // not for this menu: a game menu closes and the line runs as a command
	Chosen                         // Entry is set
	Leave                          // the Leave entry, or Enter on a menu with Leave and no Default
	NoSuchNumber                   // a number outside 1..len
	Ambiguous                      // a prefix matching several entries (Prefix menus only); Matches is set
	Empty                          // Enter on a menu with no Default and no Leave
	Text                           // an unmatched line on an AcceptText menu; Text is set
)

// Result is what Resolve decided.
type Result struct {
	Kind    ResultKind
	Entry   *Entry
	Matches []Entry
	Text    string
}

// Resolve matches a typed line against the menu. Game menus resolve only a
// number, Enter, an exact label, id or alias; a prefix resolves only when
// Prefix is set, because in a non-modal menu "n" or "l" is a movement or look
// command, not a choice.
func Resolve(m Menu, input string) Result {
	in := strings.TrimSpace(input)
	all := m.all()

	if in == "" {
		if m.Default != "" {
			for i := range m.Entries {
				if m.Entries[i].Id == m.Default {
					e := m.Entries[i]
					return Result{Kind: Chosen, Entry: &e}
				}
			}
		}
		if m.Leave != "" {
			return Result{Kind: Leave}
		}
		return Result{Kind: Empty}
	}

	if n, err := strconv.Atoi(in); err == nil {
		if n < 1 || n > len(all) {
			return Result{Kind: NoSuchNumber}
		}
		if m.Leave != "" && n == len(all) {
			return Result{Kind: Leave}
		}
		e := m.Entries[n-1]
		return Result{Kind: Chosen, Entry: &e}
	}

	for i := range m.Entries {
		e := m.Entries[i]
		if strings.EqualFold(e.Label, in) || strings.EqualFold(e.Id, in) {
			return Result{Kind: Chosen, Entry: &e}
		}
		for _, a := range e.Aliases {
			if strings.EqualFold(a, in) {
				return Result{Kind: Chosen, Entry: &e}
			}
		}
	}
	if m.Leave != "" && strings.EqualFold(m.Leave, in) {
		return Result{Kind: Leave}
	}

	if m.Prefix {
		low := strings.ToLower(in)
		var matches []Entry
		for _, e := range m.Entries {
			if strings.HasPrefix(strings.ToLower(e.Label), low) || strings.HasPrefix(strings.ToLower(e.Id), low) {
				matches = append(matches, e)
			}
		}
		leaveMatch := m.Leave != "" && strings.HasPrefix(strings.ToLower(m.Leave), low)
		switch {
		case len(matches) == 1 && !leaveMatch:
			return Result{Kind: Chosen, Entry: &matches[0]}
		case len(matches) == 0 && leaveMatch:
			return Result{Kind: Leave}
		case len(matches) > 1 || (len(matches) == 1 && leaveMatch):
			return Result{Kind: Ambiguous, Matches: matches}
		}
	}

	if m.AcceptText {
		return Result{Kind: Text, Text: in}
	}
	return Result{Kind: NotAnAnswer}
}

// Confirm builds the two-entry yes/no menu. No is the way out and Enter picks
// it. For an action that cannot be undone, the labels name the action
// ("Yes, disband it" / "No, keep it"); otherwise pass "Yes" and "No".
func Confirm(question, yesLabel, noLabel string) Menu {
	return Menu{
		Heading: question,
		Entries: []Entry{
			{Id: "yes", Label: yesLabel, Aliases: []string{"y"}},
			{Id: "no", Label: noLabel, Aliases: []string{"n"}},
		},
		Default: "no",
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/menu/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ modules/
git add internal/menu/resolve.go internal/menu/resolve_test.go
git commit -F - <<'EOF'
feat(menu): resolve typed input and build confirms (#375 phase 0)

Game menus resolve only numbers, Enter and exact labels, ids or aliases,
so a movement or look shortcut typed mid-menu runs as a command. Builder
menus opt into prefix matching and free text.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Close reasons, interrupt and follow lines, label lint

**Files:**
- Create: `internal/menu/lines.go`, `internal/menu/lint.go`
- Test: `internal/menu/lines_test.go`, `internal/menu/lint_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/menu/lines_test.go`:
```go
package menu

import "testing"

func TestInterruptLine(t *testing.T) {
	cases := map[CloseReason][2]string{
		CloseCombat:      {"You break off as the fight begins.", "You break off your talk with Kael as the fight begins."},
		CloseBusy:        {"You break off to focus on your work.", "You break off your talk with Kael to focus on your work."},
		CloseOffFeet:     {"You break off as you go down.", "You break off your talk with Kael as you go down."},
		CloseControlled:  {"You break off as you are seized.", "You break off your talk with Kael as you are seized."},
		CloseAsleep:      {"You break off as you fall asleep.", "You break off your talk with Kael as you fall asleep."},
		CloseQuitting:    {"You break off to meditate.", "You break off your talk with Kael to meditate."},
		CloseRoomChange:  {"You break off as you leave.", "You break off your talk with Kael as you leave."},
		CloseContextLost: {"You break off.", "You break off your talk with Kael."},
	}
	for r, want := range cases {
		if got := InterruptLine(r, ""); got != want[0] {
			t.Errorf("%v generic: %q, want %q", r, got, want[0])
		}
		if got := InterruptLine(r, "talk with Kael"); got != want[1] {
			t.Errorf("%v topic: %q, want %q", r, got, want[1])
		}
	}
	for _, silent := range []CloseReason{CloseNone, CloseChosen, CloseLeft, CloseOtherCommand, CloseReplaced, CloseDead, CloseFollow} {
		if got := InterruptLine(silent, "talk with Kael"); got != "" {
			t.Errorf("%v should be silent, got %q", silent, got)
		}
	}
}

func TestFollowLines(t *testing.T) {
	f, l, r := FollowLines("Mira", "Brannoc", "talk with Kael")
	if f != "You break off your talk with Kael to follow Brannoc." ||
		l != "Mira breaks off a talk with Kael to follow you." ||
		r != "Mira breaks off a talk with Kael and follows Brannoc." {
		t.Fatalf("topic lines: %q / %q / %q", f, l, r)
	}
	f, l, r = FollowLines("Mira", "Brannoc", "")
	if f != "You stop what you were doing to follow Brannoc." ||
		l != "Mira stops what they were doing to follow you." ||
		r != "Mira stops what they were doing and follows Brannoc." {
		t.Fatalf("generic lines: %q / %q / %q", f, l, r)
	}
}
```

`internal/menu/lint_test.go`:
```go
package menu

import (
	"strings"
	"testing"
)

func TestLintLabels_AcceptsTheTalkMenu(t *testing.T) {
	if probs := LintLabels(talkMenu()); len(probs) != 0 {
		t.Fatalf("clean menu flagged: %v", probs)
	}
}

func TestLintLabels_FlagsEachRule(t *testing.T) {
	cases := map[string]Entry{
		"empty label":   {Id: "a", Label: ""},
		"label too long": {Id: "a", Label: strings.Repeat("x", 51)},
		"detail too long": {Id: "a", Label: "ok", Detail: strings.Repeat("x", 73)},
		"em dash":       {Id: "a", Label: "Wait \u2014 what?"},
		"en dash":       {Id: "a", Label: "One \u2013 two"},
		"Ask prefix":    {Id: "a", Label: "Ask: Where is it?"},
		"Task prefix":   {Id: "a", Label: "task: The Feud"},
	}
	for name, e := range cases {
		if probs := LintLabels(Menu{Entries: []Entry{e}}); len(probs) == 0 {
			t.Errorf("%s: not flagged", name)
		}
	}
	dup := Menu{Entries: []Entry{{Id: "a", Label: "Where?"}, {Id: "b", Label: "where?"}}}
	if probs := LintLabels(dup); len(probs) == 0 {
		t.Errorf("duplicate labels not flagged")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/menu/ -run 'TestInterruptLine|TestFollowLines|TestLintLabels' -v`
Expected: FAIL, `undefined: CloseReason`.

- [ ] **Step 3: Write `internal/menu/lines.go`**

```go
package menu

// CloseReason says why an open menu closed.
type CloseReason int

const (
	CloseNone         CloseReason = iota // nothing closed it (gate: no rule holds)
	CloseChosen                          // the player chose an entry
	CloseLeft                            // the player left
	CloseOtherCommand                    // a typed line that was not an answer ran as a command
	CloseReplaced                        // another menu opened
	CloseCombat
	CloseBusy // casting, crafting or salvaging
	CloseDead // dead or respawning
	CloseAsleep
	CloseOffFeet
	CloseControlled
	CloseQuitting
	CloseRoomChange
	CloseFollow // pulled away by a leader; FollowLines speaks instead
	CloseContextLost
)

// Session is what a caller hands internal/users when it opens a game menu.
type Session struct {
	// Topic names the conversation in interrupt and follow lines
	// ("talk with Kael"). "" gives the generic wording.
	Topic string
	// TopicNames are the names inside Topic, hidden per reader by the
	// sight rules in follow lines.
	TopicNames []string
	OnChoose   func(e Entry)
	OnClose    func(r CloseReason)
	// StillValid re-checks the caller's context before a choice acts
	// (phase 1: the NPC is still here and visible by name). nil means valid.
	StillValid func() bool
}

var interruptTail = map[CloseReason]string{
	CloseCombat:      " as the fight begins",
	CloseBusy:        " to focus on your work",
	CloseOffFeet:     " as you go down",
	CloseControlled:  " as you are seized",
	CloseAsleep:      " as you fall asleep",
	CloseQuitting:    " to meditate",
	CloseRoomChange:  " as you leave",
	CloseContextLost: "",
}

// InterruptLine is the one line the player reads when an interrupt closes
// their menu, or "" when the close is silent (their own choice, a command
// whose output speaks for itself, death, a follow pull).
func InterruptLine(r CloseReason, topic string) string {
	tail, ok := interruptTail[r]
	if !ok {
		return ""
	}
	subject := ""
	if topic != "" {
		subject = " your " + topic
	}
	return "You break off" + subject + tail + "."
}

// FollowLines are the three lines when a leader pulls a follower out of a
// menu: to the follower, to the leader, and to the room left behind. Names
// arrive formatted; the caller hides them per reader.
func FollowLines(follower, leader, topic string) (toFollower, toLeader, toRoom string) {
	if topic == "" {
		return "You stop what you were doing to follow " + leader + ".",
			follower + " stops what they were doing to follow you.",
			follower + " stops what they were doing and follows " + leader + "."
	}
	return "You break off your " + topic + " to follow " + leader + ".",
		follower + " breaks off a " + topic + " to follow you.",
		follower + " breaks off a " + topic + " and follows " + leader + "."
}
```

- [ ] **Step 4: Write `internal/menu/lint.go`**

```go
package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxLabel  = 50
	maxDetail = 72
)

// LintLabels checks the menu language conventions a machine can check:
// length, no em or en dashes, no "Ask:" / "Task:" prefix, no duplicate
// labels. Phase 1 wires it into the dialogue validator.
func LintLabels(m Menu) []string {
	var probs []string
	seen := map[string]bool{}
	for i, e := range m.Entries {
		where := fmt.Sprintf("entry %d (%s)", i+1, e.Id)
		label := strings.TrimSpace(e.Label)
		if label == "" {
			probs = append(probs, where+": empty label")
		}
		if utf8.RuneCountInString(e.Label) > maxLabel {
			probs = append(probs, fmt.Sprintf("%s: label over %d characters", where, maxLabel))
		}
		if utf8.RuneCountInString(e.Detail) > maxDetail {
			probs = append(probs, fmt.Sprintf("%s: detail over %d characters", where, maxDetail))
		}
		if strings.ContainsAny(e.Label+e.Detail, "\u2013\u2014") {
			probs = append(probs, where+": em or en dash")
		}
		low := strings.ToLower(label)
		if strings.HasPrefix(low, "ask:") || strings.HasPrefix(low, "task:") {
			probs = append(probs, where+`: "Ask:" or "Task:" prefix`)
		}
		if label != "" && seen[low] {
			probs = append(probs, where+": duplicate label")
		}
		seen[low] = true
	}
	return probs
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/menu/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/menu/lines.go internal/menu/lint.go internal/menu/lines_test.go internal/menu/lint_test.go
git commit -F - <<'EOF'
feat(menu): close reasons, interrupt and follow lines, label lint (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Typed-input flag

**Files:**
- Modify: `internal/events/const.go:31`
- Modify: `world.go:955-961` (input worker), `world.go:1010-1030` (macro expansion)
- Test: `internal/events/const_typed_test.go`

- [ ] **Step 1: Write the failing test**

```go
package events

import "testing"

func TestCmdTyped_IsADistinctBit(t *testing.T) {
	for _, f := range []EventFlag{CmdSkipScripts, CmdSecretly, CmdIsRequeue, CmdBlockInput, CmdUnBlockInput, CmdFollowStep} {
		if f&CmdTyped != 0 {
			t.Fatalf("CmdTyped overlaps flag %b", f)
		}
	}
	var f EventFlag
	if f.Has(CmdTyped) {
		t.Fatal("zero flags must not read as typed")
	}
	f = CmdTyped
	if !f.Has(CmdTyped) {
		t.Fatal("CmdTyped not detected")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/events/ -run TestCmdTyped -v`
Expected: FAIL, `undefined: CmdTyped`.

- [ ] **Step 3: Add the flag after `CmdFollowStep` in `internal/events/const.go`**

```go
	CmdFollowStep EventFlag = 0b00100000

	// A line the player typed (telnet or web client), set only by the world
	// input worker and kept on the player's own macro expansions. Only a
	// typed line may answer a menu or a prompt; an engine-queued command
	// (follow steps, scripts, user.Command) always runs as a command (#375).
	CmdTyped EventFlag = 0b01000000
)
```

- [ ] **Step 4: Set it in the input worker (`world.go`, the `case wi := <-w.worldInput:` block)**

```go
		case wi := <-w.worldInput:

			events.AddToQueue(events.Input{
				UserId:    wi.FromId,
				InputText: wi.InputText,
				ReadyTurn: util.GetTurnCount(),
				Flags:     events.CmdTyped,
			})
```

- [ ] **Step 5: Keep it on macro expansion (`world.go`, inside `if macro, ok := user.Macros[inputText]; ok {`)**

```go
					events.AddToQueue(events.Input{
						UserId:    userId,
						InputText: newCmd,
						ReadyTurn: readyTurn,
						Flags:     flags & events.CmdTyped,
					})
```

- [ ] **Step 6: Run tests and build**

Run: `go test ./internal/events/ -v && go build ./...`
Expected: PASS, build clean.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
git add internal/events/const.go internal/events/const_typed_test.go world.go
git commit -F - <<'EOF'
feat(events): mark typed input with CmdTyped (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Observer passthroughs on five state machines

**Files:**
- Modify: `internal/state/combatphase/combatphase.go`, `internal/state/activity/activity.go`, `internal/state/life/life.go`, `internal/state/position/position.go`, `internal/state/control/control.go`
- Test: `internal/state/observers_passthrough_test.go`

- [ ] **Step 1: Write the failing test**

Check each package's transition helpers first (`grep -n "func (m \*Machine)" internal/state/<pkg>/<pkg>.go`) and use a real allowed transition. The test below uses the public `TransitionTo(to, reason)` that each wrapper exposes; if a wrapper names it differently, call that wrapper's method and keep the assertion.

```go
package state_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

func TestRegisterObserver_ActivityFiresOnTransition(t *testing.T) {
	m := activity.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to activity.State, r state.TransitionReason) { fired++ })
	if err := m.TransitionTo(activity.Crafting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

func TestRegisterObserver_LifeFiresOnTransition(t *testing.T) {
	m := life.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to life.State, r state.TransitionReason) { fired++ })
	if err := m.TransitionTo(life.Dead, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

func TestRegisterObserver_NilMachineIsANoOp(t *testing.T) {
	var a *activity.Machine
	a.RegisterObserver("probe", func(from, to activity.State, r state.TransitionReason) {})
}
```
Add the same shape for `combatphase`, `position` and `control` using one transition each that the package's table allows (read the table in the package before choosing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/state/ -run TestRegisterObserver -v`
Expected: FAIL, `m.RegisterObserver undefined`.

- [ ] **Step 3: Add the passthrough to each of the five packages**

In each file, next to its `State()` method, add (the `State` type is the package's own):

```go
// RegisterObserver installs an observer fired after every transition.
// Mirrors presence.Machine.RegisterObserver; internal/characters uses it so
// an open menu closes the moment the player becomes busy (#375).
func (m *Machine) RegisterObserver(name string, obs func(from, to State, r state.TransitionReason)) {
	if m == nil || m.inner == nil {
		return
	}
	m.inner.Subscribe(name, obs)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/state/... -v -run TestRegisterObserver`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ modules/
git add internal/state/combatphase/combatphase.go internal/state/activity/activity.go internal/state/life/life.go internal/state/position/position.go internal/state/control/control.go internal/state/observers_passthrough_test.go
git commit -F - <<'EOF'
feat(state): RegisterObserver on combatphase, activity, life, position, control (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Wire busy observers on characters

**Files:**
- Create: `internal/characters/busy_observers.go`
- Modify: `internal/characters/character.go` (Character struct; `NewCharacter` after the literal at ~line 436), `internal/characters/validate.go` (end of the machine block, after the Perception guard at ~line 689)
- Test: `internal/characters/busy_observers_test.go`

- [ ] **Step 1: Write the failing test**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

func TestBusyObservers_NotifyTheUserOnTransition(t *testing.T) {
	var got []int
	old := BusyStateChanged
	BusyStateChanged = func(userId int) { got = append(got, userId) }
	defer func() { BusyStateChanged = old }()

	c := NewCharacter()
	c.SetUserId(7)
	c.WireBusyObservers()
	if err := c.Activity.TransitionTo(activity.Crafting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("notified %v, want [7]", got)
	}
}

func TestBusyObservers_WireTwiceRegistersOnce(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := NewCharacter()
	c.SetUserId(7)
	c.WireBusyObservers()
	c.WireBusyObservers()
	_ = c.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	if n != 1 {
		t.Fatalf("notified %d times, want 1", n)
	}
}

func TestBusyObservers_ReplacedMachineIsRewired(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := NewCharacter()
	c.SetUserId(7)
	c.WireBusyObservers()
	c.Activity = activity.NewMachine()
	c.WireBusyObservers()
	_ = c.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	if n != 1 {
		t.Fatalf("notified %d times, want 1", n)
	}
}

func TestBusyObservers_MobCharacterIsSilent(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := NewCharacter() // userId 0
	c.WireBusyObservers()
	_ = c.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	if n != 0 {
		t.Fatalf("mob character notified %d times", n)
	}
}
```
Before running, confirm the constructor's name with `grep -n "^func New" internal/characters/character.go` and use it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run TestBusyObservers -v`
Expected: FAIL, `undefined: BusyStateChanged`.

- [ ] **Step 3: Create `internal/characters/busy_observers.go`**

```go
package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// BusyStateChanged, when set, is told a player's user id whenever one of the
// machines that decide "busy" transitions. internal/users sets it at init so
// an open menu closes the moment its player becomes busy (#375). It is the
// push half of the menu gate; the shared busy check (#640) replaces the gate.
var BusyStateChanged func(userId int)

// busyWiring remembers which machine instances already carry the observer,
// so wiring twice registers once and a replaced machine is wired again
// (state.Machine has no unsubscribe).
type busyWiring struct {
	combat   *combatphase.Machine
	life     *life.Machine
	activity *activity.Machine
	position *position.Machine
	control  *control.Machine
}

// WireBusyObservers registers the busy observer on each present machine not
// yet wired. Safe to call repeatedly; NewCharacter and Validate both call it.
func (c *Character) WireBusyObservers() {
	notify := func() {
		if BusyStateChanged != nil && c.userId > 0 {
			BusyStateChanged(c.userId)
		}
	}
	const name = "menu_busy_gate"
	if c.CombatPhase != nil && c.busyWired.combat != c.CombatPhase {
		c.busyWired.combat = c.CombatPhase
		c.CombatPhase.RegisterObserver(name, func(_, _ combatphase.State, _ state.TransitionReason) { notify() })
	}
	if c.Life != nil && c.busyWired.life != c.Life {
		c.busyWired.life = c.Life
		c.Life.RegisterObserver(name, func(_, _ life.State, _ state.TransitionReason) { notify() })
	}
	if c.Activity != nil && c.busyWired.activity != c.Activity {
		c.busyWired.activity = c.Activity
		c.Activity.RegisterObserver(name, func(_, _ activity.State, _ state.TransitionReason) { notify() })
	}
	if c.Position != nil && c.busyWired.position != c.Position {
		c.busyWired.position = c.Position
		c.Position.RegisterObserver(name, func(_, _ position.State, _ state.TransitionReason) { notify() })
	}
	if c.Control != nil && c.busyWired.control != c.Control {
		c.busyWired.control = c.Control
		c.Control.RegisterObserver(name, func(_, _ control.State, _ state.TransitionReason) { notify() })
	}
}
```

- [ ] **Step 4: Add the field to the `Character` struct (next to `userId` at ~line 348)**

```go
	userId                  int                            // User ID of the character if any
	busyWired               busyWiring                     // which machines carry the menu busy observer (#375); not saved
```

- [ ] **Step 5: Call it in `NewCharacter` (just before `return c` / after the stats roll) and at the end of the machine block in `Validate()` (after the Perception guard)**

```go
	c.WireBusyObservers()
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/characters/ -v -run TestBusyObservers && go build ./...`
Expected: PASS, build clean.

- [ ] **Step 7: Run the whole characters package (catches a struct-copy surprise)**

Run: `go test ./internal/characters/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -l internal/ modules/
git add internal/characters/busy_observers.go internal/characters/busy_observers_test.go internal/characters/character.go internal/characters/validate.go
git commit -F - <<'EOF'
feat(characters): push busy-state changes for the menu gate (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: MenuChanged event and the game menu session

**Files:**
- Modify: `internal/events/eventtypes.go` (after `AutomationChanged`, ~line 79)
- Create: `internal/users/menu_gate.go`, `internal/users/userrecord.menu.go`
- Modify: `internal/users/userrecord.go` (`activeMenu` field; `StartPrompt`/`ClearPrompt` notify)
- Test: `internal/users/userrecord_menu_test.go`

- [ ] **Step 1: Add the event to `internal/events/eventtypes.go`**

```go
// MenuChanged fires when a player's first game menu or prompt opens (Open
// true) or the last one closes (Open false). gmcp pushes it as Char.Menu so
// the web client pauses ticks and triggers while a menu is open (#375).
type MenuChanged struct {
	UserId int
	Open   bool
}

func (m MenuChanged) Type() string { return `MenuChanged` }
```

- [ ] **Step 2: Write the failing session tests**

```go
package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

func testMenu() menu.Menu {
	return menu.Menu{Heading: "Pick", Entries: []menu.Entry{{Id: "a", Label: "Alpha"}, {Id: "b", Label: "Beta"}}, Leave: "Leave"}
}

type rec struct {
	chosen []string
	closed []menu.CloseReason
}

func (r *rec) session() menu.Session {
	return menu.Session{
		OnChoose: func(e menu.Entry) { r.chosen = append(r.chosen, e.Id) },
		OnClose:  func(c menu.CloseReason) { r.closed = append(r.closed, c) },
	}
}

func menuUser() *UserRecord {
	u := NewTestUser(1, "tester", "Mira", 0)
	u.Character.Activity = activity.NewMachine()
	return u
}

func TestOpenMenu_ChooseByNumberCallsOnChooseAndCloses(t *testing.T) {
	u, r := menuUser(), &rec{}
	if _, ok := u.OpenMenu(testMenu(), r.session()); !ok {
		t.Fatal("menu did not open")
	}
	if !u.RouteMenuInput("2", true) {
		t.Fatal("a number was not consumed")
	}
	if len(r.chosen) != 1 || r.chosen[0] != "b" || u.HasOpenMenu() {
		t.Fatalf("chosen %v, open %v", r.chosen, u.HasOpenMenu())
	}
	if len(r.closed) != 1 || r.closed[0] != menu.CloseChosen {
		t.Fatalf("closed %v", r.closed)
	}
}

func TestRouteMenuInput_QueuedLinesNeverAnswer(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if u.RouteMenuInput("1", false) {
		t.Fatal("a queued line answered the menu")
	}
	if !u.HasOpenMenu() || len(r.chosen) != 0 {
		t.Fatal("a queued line changed the menu")
	}
}

func TestRouteMenuInput_OtherCommandClosesAndPassesThrough(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if u.RouteMenuInput("north", true) {
		t.Fatal("north was consumed")
	}
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseOtherCommand {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestRouteMenuInput_OutOfRangeKeepsMenuOpen(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if !u.RouteMenuInput("9", true) || !u.HasOpenMenu() {
		t.Fatal("out of range should be consumed and keep the menu")
	}
}

func TestRouteMenuInput_EnterLeaves(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if !u.RouteMenuInput("", true) || u.HasOpenMenu() || r.closed[0] != menu.CloseLeft {
		t.Fatalf("enter: open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestOpenMenu_RefusedWhileBusy(t *testing.T) {
	u, r := menuUser(), &rec{}
	_ = u.Character.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	reason, ok := u.OpenMenu(testMenu(), r.session())
	if ok || reason != menu.CloseBusy {
		t.Fatalf("ok %v reason %v", ok, reason)
	}
}

func TestOpenMenu_RefusedWhileAPromptIsOpen(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.StartPrompt("build", "")
	if _, ok := u.OpenMenu(testMenu(), r.session()); ok {
		t.Fatal("opened over a prompt")
	}
}

func TestOpenMenu_ReplacesTheOpenOne(t *testing.T) {
	u, first, second := menuUser(), &rec{}, &rec{}
	u.OpenMenu(testMenu(), first.session())
	u.OpenMenu(testMenu(), second.session())
	if len(first.closed) != 1 || first.closed[0] != menu.CloseReplaced {
		t.Fatalf("first closed %v", first.closed)
	}
}

func TestChoice_RecheckedAgainstTheGate(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	// Busy without a push (the observer is not wired on this fixture).
	_ = u.Character.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	u.RouteMenuInput("1", true)
	if len(r.chosen) != 0 || r.closed[0] != menu.CloseBusy {
		t.Fatalf("chosen %v closed %v", r.chosen, r.closed)
	}
}

func TestChoice_StillValidFalseClosesWithContextLost(t *testing.T) {
	u, r := menuUser(), &rec{}
	s := r.session()
	s.StillValid = func() bool { return false }
	u.OpenMenu(testMenu(), s)
	u.RouteMenuInput("1", true)
	if len(r.chosen) != 0 || r.closed[0] != menu.CloseContextLost {
		t.Fatalf("chosen %v closed %v", r.chosen, r.closed)
	}
}

func TestCheckMenuInterrupts_ClosesOnBusy(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	_ = u.Character.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	u.CheckMenuInterrupts()
	if u.HasOpenMenu() || r.closed[0] != menu.CloseBusy {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestCheckMenuInterrupts_ClosesANonModalPromptButNotAModalOne(t *testing.T) {
	u := menuUser()
	p, _ := u.StartPrompt("guild", "")
	p.NonModal = true
	_ = u.Character.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	u.CheckMenuInterrupts()
	if u.GetPrompt() != nil {
		t.Fatal("non-modal prompt survived an interrupt")
	}
	u2 := menuUser()
	u2.StartPrompt("room edit", "")
	_ = u2.Character.Activity.TransitionTo(activity.Crafting, state.TransitionReason{})
	u2.CheckMenuInterrupts()
	if u2.GetPrompt() == nil {
		t.Fatal("modal builder prompt was closed by a game interrupt")
	}
}

func TestMenuOpenState_TracksMenusAndPrompts(t *testing.T) {
	u, r := menuUser(), &rec{}
	if u.MenuOrPromptOpen() {
		t.Fatal("open at start")
	}
	u.OpenMenu(testMenu(), r.session())
	if !u.MenuOrPromptOpen() {
		t.Fatal("menu not counted")
	}
	u.CloseMenu(menu.CloseLeft)
	u.StartPrompt("x", "")
	if !u.MenuOrPromptOpen() {
		t.Fatal("prompt not counted")
	}
	u.ClearPrompt()
	if u.MenuOrPromptOpen() {
		t.Fatal("still open after clear")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/users/ -run 'TestOpenMenu|TestRouteMenuInput|TestChoice|TestCheckMenuInterrupts|TestMenuOpenState' -v`
Expected: FAIL, `u.OpenMenu undefined`. (`p.NonModal` fails too until Task 9; to keep this task self-contained, add the `NonModal bool` field to `prompt.Prompt` now, as in Task 9 Step 3's struct, and Task 9 adds the rest.)

- [ ] **Step 4: Create `internal/users/menu_gate.go`**

```go
package users

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state/control"
)

// quitConditionId is condition 0, "Meditating": the quit timer that `quit`
// starts (conditions/0-meditating.yaml).
const quitConditionId = 0

// menuGate reports the first rule that stops a game menu opening or closes
// an open one, or menu.CloseNone. Owner rules 2026-10-20. Built from the same
// sources as the shared busy check filed as #640; move onto it when it lands.
func menuGate(c *characters.Character) menu.CloseReason {
	switch {
	case c == nil:
		return menu.CloseNone
	case !c.IsAlive():
		return menu.CloseDead
	case c.IsInCombat():
		return menu.CloseCombat
	case c.IsActing():
		return menu.CloseBusy
	case c.HasConditionFlag(conditions.Sleeping):
		return menu.CloseAsleep
	case c.HasCondition(quitConditionId):
		return menu.CloseQuitting
	case !c.IsStanding():
		return menu.CloseOffFeet
	case c.IsBeingControlled(), c.Control != nil && c.Control.State() == control.BecomingControlled:
		return menu.CloseControlled
	}
	return menu.CloseNone
}
```

- [ ] **Step 5: Add the field to `UserRecord` (after `activePrompt`) and update the `sessionMu` comment**

```go
	activePrompt   *prompt.Prompt
	activeMenu     *openMenu // the open game menu (#375); guarded by sessionMu
```
Change the `sessionMu` comment's first line to "sessionMu guards unsentText, suggestText, tempDataStore, activePrompt and activeMenu".

- [ ] **Step 6: Create `internal/users/userrecord.menu.go`**

```go
package users

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

type openMenu struct {
	m menu.Menu
	s menu.Session
}

// The characters push hook lands on the user's interrupt check: a busy-state
// transition closes an open menu at once (#375).
func init() {
	characters.BusyStateChanged = func(userId int) {
		if u := GetByUserId(userId); u != nil {
			u.CheckMenuInterrupts()
		}
	}
}

// MenuOrPromptOpen reports whether the player has a game menu or any prompt
// open: what Char.Menu tells the web client.
func (u *UserRecord) MenuOrPromptOpen() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu != nil || u.activePrompt != nil
}

// HasOpenMenu reports whether a game menu is open.
func (u *UserRecord) HasOpenMenu() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu != nil
}

func (u *UserRecord) notifyMenuState(wasOpen bool) {
	if now := u.MenuOrPromptOpen(); now != wasOpen {
		events.AddToQueue(events.MenuChanged{UserId: u.UserId, Open: now})
	}
}

// OpenMenu opens a game menu and sends it. It refuses, returning the gate's
// reason, while an interrupt rule holds, and refuses (menu.CloseNone, false)
// while a prompt is open. An open game menu is replaced.
func (u *UserRecord) OpenMenu(m menu.Menu, s menu.Session) (menu.CloseReason, bool) {
	if r := menuGate(u.Character); r != menu.CloseNone {
		return r, false
	}
	wasOpen := u.MenuOrPromptOpen()
	u.sessionMu.Lock()
	if u.activePrompt != nil {
		u.sessionMu.Unlock()
		return menu.CloseNone, false
	}
	old := u.activeMenu
	u.activeMenu = &openMenu{m: m, s: s}
	u.sessionMu.Unlock()

	if old != nil && old.s.OnClose != nil {
		old.s.OnClose(menu.CloseReplaced)
	}
	u.SendText(messaging.CategoryScreen, menu.Render(m))
	u.notifyMenuState(wasOpen)
	return menu.CloseNone, true
}

// takeMenu removes om if it is still the open menu and reports whether it was.
func (u *UserRecord) takeMenu(om *openMenu) bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if u.activeMenu != om || om == nil {
		return false
	}
	u.activeMenu = nil
	return true
}

func (u *UserRecord) closeMenuWith(om *openMenu, r menu.CloseReason, announce bool) {
	wasOpen := u.MenuOrPromptOpen()
	if !u.takeMenu(om) {
		return
	}
	if announce {
		if line := menu.InterruptLine(r, om.s.Topic); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
	}
	if om.s.OnClose != nil {
		om.s.OnClose(r)
	}
	u.notifyMenuState(wasOpen)
}

func (u *UserRecord) currentMenu() *openMenu {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu
}

// CloseMenu closes the open game menu, if any, announcing an interrupt reason.
func (u *UserRecord) CloseMenu(r menu.CloseReason) {
	u.closeMenuWith(u.currentMenu(), r, true)
}

// CloseMenuForFollow closes the open game menu or non-modal prompt silently
// because a leader is pulling the player away, and returns the session's
// topic, its names, and whether anything was open. The caller speaks the
// three follow lines (actions.BreakOffMenuToFollow).
func (u *UserRecord) CloseMenuForFollow() (topic string, names []string, had bool) {
	if om := u.currentMenu(); om != nil {
		u.closeMenuWith(om, menu.CloseFollow, false)
		return om.s.Topic, om.s.TopicNames, true
	}
	if p := u.GetPrompt(); p != nil && p.NonModal {
		u.ClearPrompt()
		return "", nil, true
	}
	return "", nil, false
}

// CheckMenuInterrupts closes the open game menu and any non-modal prompt when
// an interrupt rule holds. Pushed by the busy observers, the condition hook,
// and called on every choice.
func (u *UserRecord) CheckMenuInterrupts() {
	r := menuGate(u.Character)
	if r == menu.CloseNone {
		return
	}
	u.closeForRule(r)
}

// CloseMenusForRoomChange is the room-change rule: any move closes the open
// game menu and any non-modal prompt.
func (u *UserRecord) CloseMenusForRoomChange() {
	u.closeForRule(menu.CloseRoomChange)
}

func (u *UserRecord) closeForRule(r menu.CloseReason) {
	if om := u.currentMenu(); om != nil {
		u.closeMenuWith(om, r, true)
	}
	if p := u.GetPrompt(); p != nil && p.NonModal {
		u.ClearPrompt()
		if line := menu.InterruptLine(r, ""); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
	}
}

// RouteMenuInput offers a line to the open game menu. It returns true when
// the menu consumed the line. A line the menu does not recognise closes it
// (menu.CloseOtherCommand) and returns false so the caller runs it as a
// command. Only typed lines may answer.
func (u *UserRecord) RouteMenuInput(line string, typed bool) bool {
	if !typed {
		return false
	}
	om := u.currentMenu()
	if om == nil {
		return false
	}
	res := menu.Resolve(om.m, line)
	switch res.Kind {
	case menu.Chosen:
		if r := menuGate(u.Character); r != menu.CloseNone {
			u.closeMenuWith(om, r, true)
			return true
		}
		if om.s.StillValid != nil && !om.s.StillValid() {
			u.closeMenuWith(om, menu.CloseContextLost, true)
			return true
		}
		u.closeMenuWith(om, menu.CloseChosen, false)
		if om.s.OnChoose != nil {
			om.s.OnChoose(*res.Entry)
		}
		return true
	case menu.Leave:
		u.closeMenuWith(om, menu.CloseLeft, false)
		return true
	case menu.NoSuchNumber:
		u.SendText(messaging.CategoryRefusal, fmt.Sprintf("There is no choice %s.", strings.TrimSpace(line)))
		return true
	case menu.Ambiguous:
		labels := make([]string, 0, len(res.Matches))
		for _, e := range res.Matches {
			labels = append(labels, e.Label)
		}
		u.SendText(messaging.CategoryRefusal, "Which one? "+strings.Join(labels, ", ")+".")
		return true
	case menu.Empty:
		u.SendText(messaging.CategoryScreen, menu.Render(om.m))
		return true
	default:
		u.closeMenuWith(om, menu.CloseOtherCommand, false)
		return false
	}
}
```

Add `"github.com/GoMudEngine/GoMud/internal/characters"` to that file's imports.

- [ ] **Step 7: Notify on prompt open and close in `internal/users/userrecord.go`**

Replace `StartPrompt` and `ClearPrompt` with:

```go
// Prompt related functionality
func (u *UserRecord) StartPrompt(command string, rest string) (*prompt.Prompt, bool) {

	wasOpen := u.MenuOrPromptOpen()

	u.sessionMu.Lock()
	if u.activePrompt != nil {
		// If it's the same prompt, return the existing one
		if u.activePrompt.Command == command && u.activePrompt.Rest == rest {
			p := u.activePrompt
			u.sessionMu.Unlock()
			return p, false
		}
	}

	// If no prompt found or it seems like a new prompt, create a new one and replace the old
	u.activePrompt = prompt.New(command, rest)
	p := u.activePrompt
	u.sessionMu.Unlock()

	u.notifyMenuState(wasOpen)
	return p, true
}

func (u *UserRecord) ClearPrompt() {
	wasOpen := u.MenuOrPromptOpen()
	u.sessionMu.Lock()
	u.activePrompt = nil
	u.sessionMu.Unlock()
	u.notifyMenuState(wasOpen)
}
```

- [ ] **Step 8: Run the tests**

Run: `go test ./internal/users/ -v -run 'TestOpenMenu|TestRouteMenuInput|TestChoice|TestCheckMenuInterrupts|TestMenuOpenState' && go test ./internal/users/... && go build ./...`
Expected: PASS, build clean. If `IsBeingControlled` panics on a nil `Control`, read `position_predicates.go:186` and guard in `menuGate` with `c.Control != nil &&` before it.

- [ ] **Step 9: Commit**

```bash
gofmt -l internal/ modules/
git add internal/events/eventtypes.go internal/users/menu_gate.go internal/users/userrecord.menu.go internal/users/userrecord.go internal/users/userrecord_menu_test.go internal/prompt/prompt.go
git commit -F - <<'EOF'
feat(users): game menu session with interrupt gate and MenuChanged (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Interrupt pushes from conditions and room changes

**Files:**
- Modify: `internal/hooks/Condition_ApplyConditions.go` (after `targetChar.Conditions.Stamp(...)`, ~line 134)
- Create: `internal/hooks/RoomChange_MenuClose.go`
- Modify: `internal/hooks/hooks.go` (RoomChange listener list, ~line 48)
- Test: `internal/hooks/menu_interrupts_test.go`

- [ ] **Step 1: Write the failing tests**

Model the fixture on an existing hooks test that seeds a user (`grep -ln SeedUsersForTest internal/hooks/*_test.go` and copy its setup).

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func TestMenuCloseOnRoomChange_ClosesTheMoversMenu(t *testing.T) {
	u := users.NewTestUser(41, "mover", "Mira", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{41: u})()
	closed := menu.CloseNone
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{OnClose: func(r menu.CloseReason) { closed = r }})

	MenuCloseOnRoomChange(events.RoomChange{UserId: 41, FromRoomId: 1, ToRoomId: 2})

	if u.HasOpenMenu() || closed != menu.CloseRoomChange {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), closed)
	}
}

func TestMenuCloseOnRoomChange_IgnoresMobsAndSameRoom(t *testing.T) {
	u := users.NewTestUser(42, "stayer", "Ilo", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{42: u})()
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{})

	MenuCloseOnRoomChange(events.RoomChange{MobInstanceId: 9, FromRoomId: 1, ToRoomId: 2})
	MenuCloseOnRoomChange(events.RoomChange{UserId: 42, FromRoomId: 3, ToRoomId: 3})

	if !u.HasOpenMenu() {
		t.Fatal("menu closed by a mob move or a same-room event")
	}
}
```

The sleeping push, seeding a test condition the way `internal/actions/speech_sight_test.go` does (add `"github.com/GoMudEngine/GoMud/internal/conditions"` to the imports):

```go
func TestApplyConditions_SleepClosesTheMenu(t *testing.T) {
	const sleepCond = 8962
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepCond: {ConditionId: sleepCond, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	u := users.NewTestUser(43, "sleeper", "Dusk", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{43: u})()
	closed := menu.CloseNone
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{OnClose: func(r menu.CloseReason) { closed = r }})

	ApplyConditions(events.Condition{UserId: 43, ConditionId: sleepCond, LifeEpoch: u.Character.LifeEpoch})

	if u.HasOpenMenu() || closed != menu.CloseAsleep {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), closed)
	}
}
```
If `ApplyConditions` needs a room loaded for its start lines, seed one with `rooms.SeedRoomsForTest` as the speech test does and set `u.Character.RoomId` to it.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/hooks/ -run 'TestMenuCloseOnRoomChange' -v`
Expected: FAIL, `undefined: MenuCloseOnRoomChange`.

- [ ] **Step 3: Create `internal/hooks/RoomChange_MenuClose.go`**

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// MenuCloseOnRoomChange closes a player's game menu and any non-modal prompt
// when they change rooms: walked, fled, or pulled by a leader (owner rule
// 2026-10-20, #375). A follow pull has already closed it with the follow
// lines, so this finds nothing open then.
func MenuCloseOnRoomChange(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok || evt.UserId == 0 || evt.FromRoomId == evt.ToRoomId {
		return events.Continue
	}
	if u := users.GetByUserId(evt.UserId); u != nil {
		u.CloseMenusForRoomChange()
	}
	return events.Continue
}
```

- [ ] **Step 4: Register it in `internal/hooks/hooks.go` after the last `RoomChange` registration**

```go
	events.RegisterListener(events.RoomChange{}, MenuCloseOnRoomChange)
```

- [ ] **Step 5: Push after a condition lands in `internal/hooks/Condition_ApplyConditions.go`, right after `targetChar.Conditions.Stamp(evt.ConditionId, evt.Source, evt.Caster)`**

```go
	// Sleeping and the quit timer (condition 0) close an open menu (#375).
	if evt.UserId > 0 {
		if u := users.GetByUserId(evt.UserId); u != nil {
			u.CheckMenuInterrupts()
		}
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/hooks/ -run 'TestMenuClose|TestApplyConditions' -v && go build ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
git add internal/hooks/RoomChange_MenuClose.go internal/hooks/hooks.go internal/hooks/Condition_ApplyConditions.go internal/hooks/menu_interrupts_test.go
git commit -F - <<'EOF'
feat(hooks): close menus on room change, sleep and quitting (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Menu questions in prompt, and input routing in world.go

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `world.go:967-1000` (prompt intercept) and the tail redraw (~line 1067)
- Modify: `internal/users/userrecord.prompt.go` (`GetCommandPrompt` unchanged; `Question.String` handles menus)
- Test: `internal/prompt/prompt_menu_test.go`

- [ ] **Step 1: Write the failing test**

```go
package prompt

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

func TestAskMenu_ChoosingSetsResponseToTheEntryId(t *testing.T) {
	p := New("room edit containers", "")
	m := menu.Menu{Heading: "Choose one:", Entries: []menu.Entry{{Id: "new", Label: "new"}, {Id: "chest", Label: "chest"}}, Default: "new"}
	q := p.AskMenu(m)
	q.Answer("2")
	if !q.Done || q.Response != "chest" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_EnterUsesTheDefault(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "new", Label: "new"}}, Default: "new"})
	q.Answer("")
	if !q.Done || q.Response != "new" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_LeaveUsesTheLeaveResponse(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}, Leave: "Done"}, "0")
	q.Answer("2")
	if !q.Done || q.Response != "0" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_BadAnswerLeavesItOpenAndRecordsWhy(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	q.Answer("7")
	if q.Done || q.LastResult != menu.NoSuchNumber {
		t.Fatalf("done %v last %v", q.Done, q.LastResult)
	}
}

func TestAskMenu_SameHeadingReplacesTheMenu(t *testing.T) {
	p := New("x", "")
	p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a", Marker: menu.MarkerChecked}}})
	if len(p.Questions) != 1 || q.Menu.Entries[0].Marker != menu.MarkerChecked {
		t.Fatalf("questions %d marker %q", len(p.Questions), q.Menu.Entries[0].Marker)
	}
}

func TestAskConfirm(t *testing.T) {
	p := New("guild", "")
	q := p.AskConfirm("Disband?", "Yes, disband it", "No, keep it")
	q.Answer("y")
	if !q.Done || q.Response != "yes" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestQuestionString_MenuShowsABarePrompt(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "A long heading", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	if s := q.String(); s != `<ansi fg="black-bold">.:</ansi> ` {
		t.Fatalf("menu prompt line: %q", s)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/prompt/ -run 'TestAskMenu|TestAskConfirm|TestQuestionString' -v`
Expected: FAIL, `p.AskMenu undefined`.

- [ ] **Step 3: Extend `internal/prompt/prompt.go`**

Add the import `"github.com/GoMudEngine/GoMud/internal/menu"`. Extend the structs:

```go
type Question struct {
	Question        string     // What's the prompt?
	Options         []string   // What options (if any) are available? None = freeform
	DefaultResponse string     // What is the default response?
	Response        string     // What was the response last submitted
	Done            bool       // Was it seen and responded to?
	Flags           int        // Mask reply etc
	Menu            *menu.Menu // set by AskMenu: the question is a menu (#375)
	LastResult      menu.ResultKind
}

type Prompt struct {
	Command   string         // Where does it call when complete?
	Rest      string         // What is the 'rest' of the command
	Questions []*Question    // All questions so far
	State     map[string]any // Optional place to store state
	// NonModal: a menu question that gets a line it does not recognise
	// closes the prompt and the line runs as a command, and game interrupts
	// close it. Player flows set it; builders stay modal (#375).
	NonModal bool
}
```

Add after `Ask`:

```go
// AskMenu asks a menu question. The menu's Heading identifies the question
// (as Ask's text does), and each call replaces the stored menu so a toggled
// marker shows on the next render. leaveResponse is the Response when the
// player picks the Leave entry.
func (p *Prompt) AskMenu(m menu.Menu, leaveResponse ...string) *Question {
	heading := m.Heading
	if heading == `` {
		heading = `?`
	}
	leave := ``
	if len(leaveResponse) > 0 {
		leave = leaveResponse[0]
	}
	for _, q := range p.Questions {
		if q.Question == heading {
			mm := m
			q.Menu = &mm
			q.DefaultResponse = leave
			return q
		}
	}
	mm := m
	q := &Question{Question: heading, Menu: &mm, DefaultResponse: leave}
	p.Questions = append(p.Questions, q)
	return q
}

// AskConfirm asks a two-entry yes/no menu; Response is "yes" or "no", and
// Enter picks no.
func (p *Prompt) AskConfirm(question, yesLabel, noLabel string) *Question {
	return p.AskMenu(menu.Confirm(question, yesLabel, noLabel))
}

// Resolve reports what a line would mean to this question's menu without
// answering it. A non-menu question reports menu.Text for any line.
func (q *Question) Resolve(answer string) menu.Result {
	if q.Menu == nil {
		return menu.Result{Kind: menu.Text, Text: strings.TrimSpace(answer)}
	}
	return menu.Resolve(*q.Menu, answer)
}
```

At the top of `Answer`, before the existing body:

```go
func (q *Question) Answer(answer string) {
	if q.Menu != nil {
		res := menu.Resolve(*q.Menu, answer)
		q.LastResult = res.Kind
		switch res.Kind {
		case menu.Chosen:
			q.Response, q.Done = res.Entry.Id, true
		case menu.Leave:
			q.Response, q.Done = q.DefaultResponse, true
		case menu.Text:
			q.Response, q.Done = res.Text, true
		}
		return
	}
	// ... existing body unchanged ...
```

At the top of `String`:

```go
func (q *Question) String() string {
	if q.Menu != nil {
		return `<ansi fg="black-bold">.:</ansi> `
	}
	// ... existing body unchanged ...
```

- [ ] **Step 4: Run the prompt tests**

Run: `go test ./internal/prompt/ -v`
Expected: PASS (old tests included).

- [ ] **Step 5: Route input in `world.go` `processInput`**

Replace the block from `var activeQuestion *prompt.Question = nil` through the end of the `if cmdPrompt := user.GetPrompt(); cmdPrompt != nil { ... }` block with:

```go
	typed := flags.Has(events.CmdTyped)

	// A typed line goes to an open game menu first (#375). A line the menu
	// does not recognise has closed it and runs below as a command.
	if user.RouteMenuInput(inputText, typed) {
		return
	}

	var activeQuestion *prompt.Question = nil
	hadPrompt := false
	if cmdPrompt := user.GetPrompt(); cmdPrompt != nil {
		hadPrompt = true
		if activeQuestion = cmdPrompt.GetNextQuestion(); activeQuestion != nil {

			switch {
			case !typed:
				// An engine-queued command (a follow step, a script) never
				// answers a prompt; it runs as a command.
			case cmdPrompt.NonModal && activeQuestion.Menu != nil &&
				activeQuestion.Resolve(inputText).Kind == menu.NotAnAnswer:
				// Non-modal: any other command closes the menu and runs.
				user.ClearPrompt()
			default:
				activeQuestion.Answer(string(inputText))
				switch activeQuestion.LastResult {
				case menu.NoSuchNumber:
					user.SendText(messaging.CategoryRefusal, fmt.Sprintf("There is no choice %s.", strings.TrimSpace(inputText)))
				case menu.Ambiguous:
					user.SendText(messaging.CategoryRefusal, "Which one? Type the number.")
				}
				inputText = ``

				// set the input buffer to invoke the command prompt it was relevant to
				if cmdPrompt.Command != `` {
					inputText = cmdPrompt.Command + " " + cmdPrompt.Rest
				}
			}
		} else {
			// If a prompt was found, but no pending questions, clear it.
			user.ClearPrompt()
		}

	}
```

Add `"github.com/GoMudEngine/GoMud/internal/menu"` to `world.go` imports. Then, just before the "If they had an input prompt, but now they don't" redraw near the end of `processInput`, clear a finished prompt so `Char.Menu` closes promptly:

```go
	// A prompt whose questions are all answered is finished; clear it now
	// rather than on the next input, so Char.Menu reports it closed (#375).
	if p := user.GetPrompt(); p != nil && p.GetNextQuestion() == nil {
		user.ClearPrompt()
	}
```

- [ ] **Step 6: Build and run the touched packages**

Run: `go build ./... && go test ./internal/prompt/ ./internal/users/ ./internal/usercommands/`
Expected: build clean, PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
git add internal/prompt/prompt.go internal/prompt/prompt_menu_test.go world.go
git commit -F - <<'EOF'
feat(prompt): menu questions, non-modal prompts, typed-only answers (#375 phase 0)

A queued command never answers a menu or prompt any more, which also
closes the hole where a follow step could be swallowed as a builder
prompt's answer.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Follow pull lines

**Files:**
- Create: `internal/actions/menu_follow.go`
- Modify: `modules/follow/follow.go` (before `user.Command(followExitName, .25)`, ~line 425), `internal/usercommands/go.go` (before `partyUser.CommandFlagged(rest, events.CmdFollowStep)`, ~line 390)
- Test: `internal/actions/menu_follow_test.go`

- [ ] **Step 1: Write the failing test**

The fixture follows `internal/actions/speech_sight_test.go` (lit and dark rooms by `Lamp`, `rooms.SeedRoomsForTest`, `users.SeedUsersForTest`, `events.DrainQueuedMessageEventsForTest`).

```go
package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	followMiraId    = 8951
	followBrannocId = 8952
	followWitnessId = 8953
	followRoomId    = 8970
)

var followTag = regexp.MustCompile(`<[^>]*>`)

func followScene(t *testing.T, lamp int) (mira, brannoc *users.UserRecord, room *rooms.Room) {
	t.Helper()
	people := map[int]*users.UserRecord{
		followMiraId:    users.NewTestUser(followMiraId, "mira", "Mira", 98951),
		followBrannocId: users.NewTestUser(followBrannocId, "brannoc", "Brannoc", 98952),
		followWitnessId: users.NewTestUser(followWitnessId, "wren", "Wren", 98953),
	}
	t.Cleanup(users.SeedUsersForTest(people))
	room = &rooms.Room{RoomId: followRoomId, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(lamp)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{followRoomId: room}, map[string]*rooms.ZoneConfig{}))
	for id, u := range people {
		u.Character.RoomId = followRoomId
		room.AddPlayer(id)
	}
	for id := range people {
		events.DrainQueuedMessageEventsForTest(id)
	}
	return people[followMiraId], people[followBrannocId], room
}

func followRead(uid int) []string {
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		out = append(out, strings.TrimSpace(followTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

func TestBreakOffMenuToFollow_ClosesAndSpeaksThreeLines(t *testing.T) {
	mira, brannoc, room := followScene(t, 60)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(mira.Character, room))
	_, ok := mira.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}},
		menu.Session{Topic: "talk with Kael", TopicNames: []string{"Kael"}})
	require.True(t, ok)
	events.DrainQueuedMessageEventsForTest(followMiraId) // the menu render

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	require.False(t, mira.HasOpenMenu())
	require.Equal(t, []string{"You break off your talk with Kael to follow Brannoc."}, followRead(followMiraId))
	require.Equal(t, []string{"Mira breaks off a talk with Kael to follow you."}, followRead(followBrannocId))
	witness := followRead(followWitnessId)
	if len(witness) == 0 { // a room-addressed message instead of per-reader lines
		for _, m := range events.DrainQueuedRoomMessagesForTest(followRoomId) {
			witness = append(witness, strings.TrimSpace(followTag.ReplaceAllString(m.Text, "")))
		}
	}
	require.Equal(t, []string{"Mira breaks off a talk with Kael and follows Brannoc."}, witness)
}

func TestBreakOffMenuToFollow_DarkFollowerDoesNotReadTheLeadersName(t *testing.T) {
	mira, brannoc, room := followScene(t, 0)
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(mira.Character, room))
	_, ok := mira.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{})
	require.True(t, ok)
	events.DrainQueuedMessageEventsForTest(followMiraId)

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	got := followRead(followMiraId)
	require.Len(t, got, 1)
	require.NotContains(t, got[0], "Brannoc")
	require.True(t, strings.HasPrefix(got[0], "You stop what you were doing to follow "), got[0])
}

func TestBreakOffMenuToFollow_NothingOpenSaysNothing(t *testing.T) {
	mira, brannoc, room := followScene(t, 60)

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	require.Empty(t, followRead(followMiraId))
	require.Empty(t, followRead(followBrannocId))
	require.Empty(t, followRead(followWitnessId))
	require.Empty(t, events.DrainQueuedRoomMessagesForTest(followRoomId))
}
```
If `OpenMenu` refuses because the test user is "off their feet" or similar, read `NewTestUser` and fix the fixture (not the gate).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/actions/ -run TestBreakOffMenuToFollow -v`
Expected: FAIL, `undefined: BreakOffMenuToFollow`.

- [ ] **Step 3: Create `internal/actions/menu_follow.go`**

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// BreakOffMenuToFollow closes the follower's game menu or non-modal prompt
// because a leader is pulling them away, and tells the follower, the leader
// (when a player) and the room left behind, each through the sight rules
// (owner rule 2026-10-20, #375). Call it before queuing the follower's step.
func BreakOffMenuToFollow(follower *users.UserRecord, leader *characters.Character, leaderUserId int, room *rooms.Room) {
	if follower == nil || leader == nil || room == nil {
		return
	}
	topic, topicNames, had := follower.CloseMenuForFollow()
	if !had {
		return
	}
	leaderTag := `mobname`
	if leaderUserId > 0 {
		leaderTag = `username`
	}
	fName := `<ansi fg="username">` + follower.Character.Name + `</ansi>`
	lName := `<ansi fg="` + leaderTag + `">` + leader.Name + `</ansi>`
	toFollower, toLeader, toRoom := menu.FollowLines(fName, lName, topic)

	hideForFollower := append([]string{leader.Name}, topicNames...)
	follower.SendText(messaging.CategoryRoomExit,
		messaging.HideNames(toFollower, hideForFollower, messaging.ParticipantSight(follower.Character, room)))

	if leaderUserId > 0 {
		if lu := users.GetByUserId(leaderUserId); lu != nil {
			hideForLeader := append([]string{follower.Character.Name}, topicNames...)
			lu.SendText(messaging.CategoryRoomExit,
				messaging.HideNames(toLeader, hideForLeader, messaging.ParticipantSight(lu.Character, room)))
		}
	}

	roomNames := append([]string{follower.Character.Name, leader.Name}, topicNames...)
	room.SendTextVisualHidingNames(messaging.CategoryRoomExit, toRoom, roomNames, follower.UserId, leaderUserId)
}
```

- [ ] **Step 4: Call it at both follow sites**

`modules/follow/follow.go`, immediately before `user.Command(followExitName, .25)`:
```go
						actions.BreakOffMenuToFollow(user, moverCharacter(evt), evt.UserId, fromRoom)
						user.Command(followExitName, .25)
```

`internal/usercommands/go.go`, immediately before `partyUser.CommandFlagged(rest, events.CmdFollowStep)`:
```go
								actions.BreakOffMenuToFollow(partyUser, user.Character, user.UserId, room)
								partyUser.CommandFlagged(rest, events.CmdFollowStep)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/actions/ -run TestBreakOffMenuToFollow -v && go test ./modules/follow/ ./internal/usercommands/ && go build ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/actions/menu_follow.go internal/actions/menu_follow_test.go modules/follow/follow.go internal/usercommands/go.go
git commit -F - <<'EOF'
feat(follow): a leader pulling a follower out of a menu tells all three (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: Char.Menu push and the server-side defer

**Files:**
- Create: `modules/gmcp/gmcp.Menu.go`
- Modify: `modules/gmcp/gmcp.CharOp.go` (`Char.Action.Try` case, after `u == nil` check, ~line 114)
- Test: `modules/gmcp/gmcp_menu_test.go`

- [ ] **Step 1: Write the failing test**

Copy the GMCP-enabled user fixture from an existing gmcp test (`grep -ln "isGMCPEnabled\|GMCPOut" modules/gmcp/*_test.go`). Assert that `menuChangedHandler(events.MenuChanged{UserId: id, Open: true})` queues a `GMCPOut{Module: "Char.Menu", Payload: map[string]bool{"open": true}}` for a GMCP-enabled user, and queues nothing for a user without GMCP.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./modules/gmcp/ -run TestMenuChanged -v`
Expected: FAIL, `undefined: menuChangedHandler`.

- [ ] **Step 3: Create `modules/gmcp/gmcp.Menu.go`**

```go
package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Menu tells the client whether a game menu or prompt is open, so the
// web client queues ticks and triggers until it closes (#375). Telnet
// clients with GMCP can read the same signal in their own scripts.
func init() {
	events.RegisterListener(events.MenuChanged{}, menuChangedHandler)
}

func menuChangedHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MenuChanged)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || !isGMCPEnabled(user.ConnectionId()) {
		return events.Continue
	}
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Menu`,
		Payload: map[string]bool{"open": evt.Open},
	})
	return events.Continue
}
```

- [ ] **Step 4: Defer queued actions while a menu or prompt is open (`gmcp.CharOp.go`, after the `if u == nil { ... }` block in `Char.Action.Try`)**

```go
		// Belt and suspenders for the client's pause (#375): a queued action
		// never runs while a menu or prompt is open.
		if u.MenuOrPromptOpen() {
			sendActionResult(uid, req.Id, "deferred", "menu open")
			break
		}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./modules/gmcp/ -v -run 'TestMenuChanged|Action' && go build ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add modules/gmcp/gmcp.Menu.go modules/gmcp/gmcp_menu_test.go modules/gmcp/gmcp.CharOp.go
git commit -F - <<'EOF'
feat(gmcp): Char.Menu push; defer queued actions while a menu is open (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: Action queue audit (JS)

**Files:**
- Modify: `tools/jstest/webclient-action-queue.test.js` (append cases before the final summary block)
- Modify (only if a case fails): `_datafiles/html/public/webclient-pure.html` queue block

- [ ] **Step 1: Append the audit cases**

Append after the last existing case, reusing the file's `reset`, `trig`, `reply`, `newRound`, `names`, `check` helpers:

```js
// ---- Audit (#375 phase 0): every queue behaviour the pause will rely on ----

// 2. Front after front: both ahead of back, in arrival order.
reset();
enqueueTriggerAction(trig(10, 'B', 'back'), 'kick');   // in flight
enqueueTriggerAction(trig(11, 'F1', 'front'), 'bash');
enqueueTriggerAction(trig(12, 'F2', 'front'), 'trip');
check('front entries keep arrival order ahead of back', JSON.stringify(names()) === '["F1","F2","B"]', JSON.stringify(names()));

// 3. Front arriving while a FRONT entry is in flight.
reset();
enqueueTriggerAction(trig(20, 'F1', 'front'), 'bash'); // in flight
enqueueTriggerAction(trig(21, 'F2', 'front'), 'trip');
reply(20, 'fired');
check('in-flight front retires by identity, F2 stays', JSON.stringify(names()) === '["F2"]', JSON.stringify(names()));

// 4. Dedup: one entry per trigger.
reset();
enqueueTriggerAction(trig(30, 'B', 'back'), 'kick');
enqueueTriggerAction(trig(30, 'B', 'back'), 'kick');
check('a trigger queues once', names().length === 1, JSON.stringify(names()));

// 5. Cap of 10, quiet drop.
reset();
for (var i = 0; i < 12; i++) { enqueueTriggerAction(trig(100 + i, 'Q' + i, 'back'), 'kick'); }
check('queue caps at 10', names().length === 10, String(names().length));

// 6. Rejected leaves the queue.
reset();
enqueueTriggerAction(trig(40, 'B', 'back'), 'kick');
reply(40, 'rejected');
check('rejected entry leaves', names().length === 0, JSON.stringify(names()));

// 7. Deferred retries next round and drops after the staleness window.
reset();
enqueueTriggerAction(trig(50, 'B', 'back'), 'kick');
for (var r = 0; r < 6; r++) { reply(50, 'deferred'); newRound(); }
check('deferred entry ages out after 6 rounds', names().length === 0, JSON.stringify(names()));

// 8. One release per round.
reset();
enqueueTriggerAction(trig(60, 'A', 'back'), 'kick');
enqueueTriggerAction(trig(61, 'B', 'back'), 'bash');
reply(60, 'fired');
check('second entry waits for the next round', sent.length === 1, JSON.stringify(sent));
newRound();
check('next round releases the second', sent.length === 2 && sent[1].body.id === 61, JSON.stringify(sent));

// 9. Login hold: nothing sends before settle.
reset();
(0, eval)('queueLoginSettled = false;');
enqueueTriggerAction(trig(70, 'B', 'back'), 'kick');
check('nothing sends during the login hold', sent.length === 0, JSON.stringify(sent));
(0, eval)('queueLoginSettled = true; drainQueue();');
check('settle releases it', sent.length === 1, JSON.stringify(sent));

// 10. Death clears.
reset();
enqueueTriggerAction(trig(80, 'B', 'back'), 'kick');
GMCPStructs.Commands = { State: { mode: 'dead' } };
(0, eval)('checkDeathClearsQueue();');
check('death clears the queue', names().length === 0, JSON.stringify(names()));
GMCPStructs.Commands = { State: { mode: '' } };
(0, eval)('checkDeathClearsQueue();');
```

- [ ] **Step 2: Run the test file**

Run: `node tools/jstest/webclient-action-queue.test.js`
Expected: every line `ok`. For each `FAIL`, that is a real queue defect: fix the queue code in `webclient-pure.html` (never the test), re-run, and note the defect in the commit message. Do not weaken an expectation to make it pass; if an expectation itself is wrong about intended behaviour, stop and report it to the controller.

- [ ] **Step 3: Run every JS test**

Run: `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED: $t"; done`
Expected: no `FAILED:` line.

- [ ] **Step 4: Commit**

```bash
git add tools/jstest/webclient-action-queue.test.js
# add _datafiles/html/public/webclient-pure.html only if Step 2 fixed a defect
git commit -F - <<'EOF'
test(webclient): audit the action queue before the menu pause (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: Automation pause and login gate (JS)

**Files:**
- Modify: `_datafiles/html/public/webclient-pure.html` (queue block vars; `drainQueue`; `runTriggers`; `rebuildTickTimers`; `"Char.Automation"` and new `"Char.Menu"` handlers; `socket.onclose`)
- Create: `tools/jstest/webclient-automation-pause.test.js`

- [ ] **Step 1: Write the failing test**

```js
// webclient-automation-pause.test.js: ticks and triggers queue while a menu
// is open, release paced after it closes, and never fire during login (#375).
//
// Run:  node tools/jstest/webclient-automation-pause.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
var html = fs.readFileSync(HTML, 'utf8');
function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}

var queueCode = extract('var actionQueue = [];', 'function runTriggers(');
var triggerCode = extract('function runTriggers(', '// Left-click on an auto-chip');
var helpers = extract('function stripAnsi(', 'var triggerLastFire = {};') + 'var triggerLastFire = {}; var TRIGGER_MIN_INTERVAL_MS = 1000;\n'
    + extract('function fireTriggerCommands(', '/////////////////////////////////////////////');
var tickCode = extract('function rebuildTickTimers() {', '/////////////////////////////////////////////');
function lift(name) {
    return extract('"' + name + '": function() {', '\n            },')
        .replace('"' + name + '": function() {', 'function __' + name.replace('.', '_') + '() {') + '\n}\n';
}

var sentData = [], sentGMCP = [], intervals = [];
global.GMCPStructs = { Char: {} };
global.renderAutomation = function () {};
global.SendData = function (d) { sentData.push(d); return true; };
global.SendGMCP = function (pkg, body) { sentGMCP.push({ pkg: pkg, body: body }); };
global.setInterval = function (fn, ms) { intervals.push(fn); return intervals.length; };
global.clearInterval = function () {};
global.tickTimers = {}; global.tickTimersSig = '';
(0, eval)(helpers + queueCode + triggerCode + tickCode + lift('Char.Menu') + lift('Char.Automation'));

var failures = 0;
function check(name, ok, detail) { if (ok) { console.log('ok   ' + name); } else { failures++; console.log('FAIL ' + name + '\n  ' + detail); } }
function setMenu(open) { GMCPStructs.Char.Menu = { open: open }; __Char_Menu(); }
function automation(a) { GMCPStructs.Char.Automation = a; __Char_Automation(); }
function fireTicks() { intervals.forEach(function (fn) { fn(); }); }
function newRound() { (0, eval)('queueFiredThisRound = false; drainQueue();'); }

// Login: nothing before the first Char.Automation.
(0, eval)('queueLoginSettled = true;');
GMCPStructs.Char.Automation = { ticks: [], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] };
runTriggers('You are hungry.', Date.now());
check('no trigger fires before the session automation push', sentData.length === 0, JSON.stringify(sentData));

automation({ ticks: [{ id: 5, enabled: true, intervalSec: 10, commands: 'score;look' }], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] });
fireTicks();
check('ticks send after the automation push', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
sentData = [];

// Menu open: ticks and immediate triggers queue.
setMenu(true);
fireTicks();
runTriggers('You are hungry.', Date.now() + 5000);
check('nothing is sent while a menu is open', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
var q = (0, eval)('actionQueue');
check('the tick and the trigger are queued', q.length === 2, JSON.stringify(q));
fireTicks();
check('a tick queues once (dedup)', (0, eval)('actionQueue').length === 2, String((0, eval)('actionQueue').length));

// Close: paced release, one per round, every command of the entry.
setMenu(false);
check('first entry released on close', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
newRound();
check('second entry released next round', JSON.stringify(sentData) === '["score","look","eat bread"]', JSON.stringify(sentData));

// Socket close: everything stops and resets.
sentData = [];
(0, eval)('stopAutomation();');
fireTicks();
runTriggers('You are hungry.', Date.now() + 10000);
check('nothing fires after a disconnect', sentData.length === 0, JSON.stringify(sentData));
check('queue emptied on disconnect', (0, eval)('actionQueue').length === 0, '');

console.log(failures ? failures + ' FAILED' : 'all ok');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node tools/jstest/webclient-automation-pause.test.js`
Expected: FAIL or a thrown `marker not found: "Char.Menu": function() {`.

- [ ] **Step 3: Add the state and helpers inside the queue block (after `var queueSettleTimer = null;`)**

```js
        var menuOpen = false;           // a game menu or prompt is open (Char.Menu, #375): queue, don't send
        var automationReady = false;    // this session's Char.Automation has arrived: ticks/triggers may run

        // Queue an action that would have fired immediately (a tick, or a
        // trigger with no queue mode) while a menu is open. It keeps all its
        // commands and is sent straight, paced one entry per round, when the
        // menu closes. Ticks use negative ids so they never collide with
        // trigger ids.
        function enqueueDirectAction(id, name, commands) {
            for (var i = 0; i < actionQueue.length; i++) {
                if (actionQueue[i].triggerId === id) return;
            }
            if (actionQueue.length >= QUEUE_CAP) return;
            actionQueue.push({ triggerId: id, name: name, commands: commands, direct: true });
            renderAutomation();
        }

        // Disconnect: stop every timer, empty the queue, and wait for the next
        // session's Char.Automation before anything runs again (#375).
        function stopAutomation() {
            for (var id in tickTimers) { clearInterval(tickTimers[id]); }
            tickTimers = {};
            tickTimersSig = '';
            clearActionQueue();
            queueLoginSettled = false;
            menuOpen = false;
            automationReady = false;
        }
```

- [ ] **Step 4: Gate `drainQueue` and send direct entries**

In `drainQueue`, right after the `if (!queueLoginSettled) { ... return; }` block, add:

```js
            if (menuOpen) return; // paused while a menu is open (#375)
```

and replace the tail starting at `if (!actionQueue.length) return;` with:

```js
            if (!actionQueue.length) return;
            var entry = actionQueue[0];
            if (entry.direct) {
                // A paused tick or immediate trigger: send all its commands now,
                // and count it as this round's release.
                String(entry.commands || "").split(";").forEach(function (c) { c = c.trim(); if (c) SendData(c); });
                queueFiredThisRound = true;
                queueFiredAt = Date.now();
                removeQueueEntry(entry);
                renderAutomation();
                return;
            }
            entry.attempts = entry.attempts || 0;
            var cmd = String(entry.commands || "").split(";")[0].trim(); // one command per queued action
            entry.isCast = /^cast\b/i.test(cmd);
            entry.spell  = entry.isCast ? cmd.replace(/^cast\s+/i, "").split(" ")[0].toLowerCase() : "";
            queueInFlight = entry;
            queueInFlightAt = Date.now();
            SendGMCP("Char.Action.Try", { id: entry.triggerId, cmd: cmd });
```

- [ ] **Step 5: Gate `runTriggers`**

At the top of `runTriggers`, after `if (!a || !a.triggers) return;`:
```js
            if (!automationReady) return; // never during login (#375)
```
and replace the non-queued branch:
```js
                } else if (menuOpen) {
                    enqueueDirectAction(t.id, t.name, substituteCaptures(ok ? t.thenCmds : t.elseCmds, m));
                } else {
                    fireTriggerCommands(ok ? t.thenCmds : t.elseCmds, m);
                }
```

- [ ] **Step 6: Gate the tick timer callback in `rebuildTickTimers`**

```js
                tickTimers[t.id] = setInterval(function() {
                    if (!automationReady) return;
                    if (menuOpen) { enqueueDirectAction(-t.id, "tick: " + (t.name || t.id), cmds); return; }
                    cmds.split(";").forEach(function(c) { c = c.trim(); if (c) SendData(c); });
                }, t.intervalSec * 1000);
```

- [ ] **Step 7: Handlers**

Change the `"Char.Automation"` handler and add `"Char.Menu"` right after it:

```js
            "Char.Automation": function() {
                automationReady = true; // first push of the session: the character is in the world
                renderAutomation();
                rebuildTickTimers();
            },
            "Char.Menu": function() {
                var m = (GMCPStructs["Char"] || {}).Menu || {};
                var wasOpen = menuOpen;
                menuOpen = m.open === true;
                if (wasOpen && !menuOpen) drainQueue();
            },
```

- [ ] **Step 8: Stop automation on disconnect**

First line inside `socket.onclose = function(event) {`:
```js
                stopAutomation(); // nothing fires at the login prompts (#375)
```

- [ ] **Step 9: Run the JS tests**

Run: `node tools/jstest/webclient-automation-pause.test.js && for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED: $t"; done`
Expected: `all ok`, no `FAILED:` line.

- [ ] **Step 10: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html tools/jstest/webclient-automation-pause.test.js
git commit -F - <<'EOF'
feat(webclient): pause ticks and triggers in menus; nothing fires at login (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Builder menus onto the primitive

**Files:**
- Create: `internal/usercommands/menu_options.go`
- Modify: `admin.item.go`, `admin.room.go`, `admin.room.exits.go`, `admin.zone.go`, `admin.server.go`, `admin.spell.go` (all under `internal/usercommands/`)

- [ ] **Step 1: Create the helper**

```go
package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/templates"
)

// optionsMenu turns a builder's NameDescription list into a menu. Each
// entry's Id is the option's Name, so a chosen entry arrives as the name the
// builder's existing name lookup already resolves. A Marked option shows the
// [x] marker. columns 2 lays long name-only lists side by side, as the
// retired numbered-list-doubled template did. Builders match by prefix.
func optionsMenu(heading string, opts []templates.NameDescription, columns int) menu.Menu {
	m := menu.Menu{Heading: heading, Columns: columns, Prefix: true}
	for _, o := range opts {
		e := menu.Entry{Id: o.Name, Label: o.Name}
		if columns != 2 {
			e.Detail = o.Description
		}
		if o.Marked {
			e.Marker = menu.MarkerChecked
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}
```

- [ ] **Step 2: Apply the pattern at every site**

The pattern, shown on the item-type site (`admin.item.go`, "Type Selection" block):

Before:
```go
		question := cmdPrompt.Ask(`What Type of item will it be?`, []string{string(newItemSpec.Type)}, string(newItemSpec.Type))
		if !question.Done {
			tplTxt, _ := templates.Process("tables/numbered-list", typeOptions, user.UserId)
			user.SendText(messaging.CategoryAdmin, tplTxt)
			return true, nil
		}

		typeNameSelection := question.Response
		if restNum, err := strconv.Atoi(typeNameSelection); err == nil {
			if restNum > 0 && restNum <= len(typeOptions) {
				typeNameSelection = typeOptions[restNum-1].Name
			}
		}
```
After:
```go
		typeMenu := optionsMenu(`What Type of item will it be?`, typeOptions, 1)
		typeMenu.Default = string(newItemSpec.Type)
		question := cmdPrompt.AskMenu(typeMenu)
		if !question.Done {
			user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
			return true, nil
		}

		typeNameSelection := question.Response
```
and the re-show after `question.RejectResponse()`:
```go
			user.SendText(messaging.CategoryAdmin, menu.Render(typeMenu))
```

Apply it to each site in this table. "Default" is the entry Id Enter picks; "Leave" adds a Leave entry whose Response is `leaveResponse` (passed as AskMenu's second argument). Keep every existing name-lookup loop; delete only the `if restNum, err := strconv.Atoi(...)` block and the `templates.Process` renders.

| File: block | Heading (unchanged text) | Columns | Default | Leave label / leaveResponse | Notes |
|---|---|---|---|---|---|
| `admin.item.go`: Type Selection (lines ~193-221) | `What Type of item will it be?` | 1 | current type | none | as shown above |
| `admin.item.go`: Subtype (lines ~342-376) | `What Subtype of item will it be?` | 1 | current subtype | `Cancel` / `` | the existing `question.Response == ""` abort branch now fires from Cancel |
| `admin.room.go`: containers (lines ~145-159) | `Choose one:` | 1 | `new` | none | prepend `templates.NameDescription{Name: "new", Description: "create a new container"}` to `containerOptions` before building the menu, if the list does not already carry it (check how `containerOptions` is built above line 140) |
| `admin.room.exits.go`: exits (lines ~63-77) | `Choose one:` | 1 | `new` | none | same as containers, with "create a new exit" |
| `admin.room.go`: trap conditions (lines ~407-480) | `Select a condition to add to the trap, or nothing to continue:` | 2 | none | `Done` / `0` | three renders become `menu.Render(*question.Menu)` after re-asking with the updated marks: call `cmdPrompt.AskMenu(optionsMenu(heading, conditionOptions, 2), "0")` again after the marks loop, then render |
| `admin.room.go`: recipes (lines ~584-605) | `Modify which (or new)?` | 1 | `skip` | none | `new` and `skip` stay entries (already in `recipeOptions`) |
| `admin.room.go`: room mutators (lines ~765-835) | `Select a mutator to add to the room, or nothing to continue:` | 2 | none | `Done` / `0` | same re-ask-after-marks rule as conditions |
| `admin.zone.go`: zone mutators (lines ~135-210) | `Select a mutator to add/remove, or nothing to continue:` | 2 | none | `Done` / `0` | same |
| `admin.server.go`: `server_Config` (lines ~305-410) | `Choose a config option, or "quit":` | 1 | none | `Quit` / `quit` | set `AcceptText: true` on the menu so a typed dotted path still works; replace both `templates.Process("tables/numbered-list", menuOptions, ...)` renders with `menu.Render(optionsMenu(heading, menuOptions, 1))`; a chosen entry's Response is its name, which the existing `fullPath += question.Response` already consumes. Use each option's `Id` (when it is a string) as the entry Id instead of `Name` here: build this one menu by hand with `menu.Entry{Id: fmt.Sprint(o.Id), Label: o.Name, Detail: o.Description}` falling back to `o.Name` when `o.Id` is nil, because `getConfigOptions` carries the path segment in `Id` (verify by reading `getConfigOptions`) |
| `admin.spell.go`: spell list (line ~72) | (display only) | n/a | n/a | n/a | not a choice: replace the template render with a plain list, one `  <name>: <description>` line per spell, sent on `messaging.CategoryAdmin` |

Add `"github.com/GoMudEngine/GoMud/internal/menu"` to each file's imports; remove `strconv` or `templates` imports only if the compiler reports them unused.

- [ ] **Step 3: Build and run the builder tests**

Run: `go build ./... && go test ./internal/usercommands/ -run 'Admin|Room|Item|Zone|Server|Spell' -v`
Expected: build clean; PASS. Any test that asserted the old numbered-list output is updated to assert the menu render (same entries, new layout); say so in the commit.

- [ ] **Step 4: Add one driving test per builder flow**

Create `internal/usercommands/builder_menu_test.go` with, per site in the table, a test that starts the prompt, answers by number, and asserts the field the builder sets (for example: item type by number `"2"` sets `newItemSpec.Type` to the second `items.ItemTypes()` entry; trap conditions `"3"` then `""` stores the third condition id in `trapConditions` and finishes). Copy an existing admin command test's setup for the user, room and role (`grep -ln "admin" internal/usercommands/*_test.go`).

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ modules/
git add internal/usercommands/menu_options.go internal/usercommands/builder_menu_test.go internal/usercommands/admin.item.go internal/usercommands/admin.room.go internal/usercommands/admin.room.exits.go internal/usercommands/admin.zone.go internal/usercommands/admin.server.go internal/usercommands/admin.spell.go
git commit -F - <<'EOF'
refactor(builders): every admin numbered list renders through internal/menu (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 15: Confirms and player menus onto the primitive

**Files:**
- Modify: `internal/usercommands/{admin.item.go,admin.mudmail.go,admin.room.exits.go,admin.room.go,admin.zone.go,character.go,deletecharacter.go,guild.go,mail.go,picklock.go,renameself.go,start.go}`, `modules/auctions/auctions.go`

- [ ] **Step 1: Convert each confirm**

Each `cmdPrompt.Ask(<question>, []string{yes, no}, <default>)` becomes `cmdPrompt.AskConfirm(<question>, <yesLabel>, <noLabel>)`, and its comparison becomes `== "yes"` / `!= "yes"`. Where the old default was `yes` (`util.BoolYN(...)` true or `defaultYN` = `y`), set it with `q.Menu.Default = "yes"` right after the call... except `AskConfirm` returns a question whose menu is stored, so do it as:

```go
	cm := menu.Confirm(`Will this lock have a trap?`, "Yes", "No")
	if len(lock.TrapConditionIds) > 0 {
		cm.Default = "yes"
	}
	question = cmdPrompt.AskMenu(cm)
```

| Site | Question | Yes label / No label | Default | Comparison after |
|---|---|---|---|---|
| `admin.item.go:386` | max uses? | Yes / No | `defaultYN` = y ⇒ yes | `:391 == "yes"` |
| `admin.item.go:456` | Does this look correct? | Yes / No | no | `:495 != "yes"` |
| `admin.mudmail.go:104` | Send this message to everyone? | Yes, send it / No | no | read the next lines and change its check to `== "yes"` |
| `admin.room.exits.go:100` | Delete this exit? | Yes, delete it / No, keep it | no | `:106 == "yes"` |
| `admin.room.exits.go:189` | Is this a hidden exit? | Yes / No | from `secretExitDefault` | `:194 == "yes"` |
| `admin.room.go:182` | Delete this container? | Yes, delete it / No, keep it | no | `:188 == "yes"` |
| `admin.room.go:327` | Will this X be locked? | Yes / No | `lock.Difficulty > 0` | `:332 == "yes"` |
| `admin.room.go:364` | Will this lock have a trap? | Yes / No | has traps | `:369 == "yes"` |
| `admin.room.go:511` | Will this container have recipes? | Yes / No | has recipes | `:516 == "yes"` |
| `admin.zone.go:215` | Should the zone have music? | Yes / No | has music | `:220 == "yes"` |
| `character.go:151` | Are you SURE? ... | Yes / No | no | `:156 == "no"` |
| `character.go:205` | delete NAME? | Yes, delete them / No, keep them | no | `:210 == "no"` |
| `character.go:259` | change to NAME? | Yes / No | no | `:264 == "no"` |
| `character.go:389` | hire price? | Yes, hire them / No | no | `:394 != "yes"` |
| `deletecharacter.go:19` | permanently delete your account? | Yes, delete my account / No, keep it | no | `:26 != "yes"` |
| `guild.go:129` | Found guild for gold? | Yes, found it / No | no | `len(q.Response) == 0 \|\| q.Response[0:1] != "Y"` ⇒ `q.Response != "yes"` |
| `guild.go:193` | Leaving will disband. Proceed? | Yes, disband it / No, stay | no | same change |
| `guild.go:227` | Disband? cannot be undone | Yes, disband it / No, keep it | no | same change |
| `guild.go:428` | Hand leadership to NAME? | Yes, hand it over / No, keep it | no | same change |
| `mail.go:183` | Send this mail to NAME? | Yes, send it / No | no | `question.Response[0:1] != "Y"` ⇒ `question.Response != "yes"` |
| `renameself.go:65` | (confirmText) | Yes / No | no | `:69 != "yes"` |
| `start.go:108` | Skip everything ... ? (y/n) | Yes, skip ahead / No | no | drop " (y/n)" from the question; `:112 != "yes"` |
| `auctions.go:414` | Auction your ITEM? | Yes, auction it / No | no (the old call had no default; Enter picking No is the rule) | `:419 != "yes"` |

- [ ] **Step 2: Mark player flows non-modal**

Right after `StartPrompt(...)` in each player command, set `cmdPrompt.NonModal = true`: `character.go` (the `character` prompt), `deletecharacter.go`, `guild.go` (each `StartPrompt` in the file), `mail.go`, `renameself.go`, `picklock.go`, `modules/auctions/auctions.go`. Leave `start.go` modal (onboarding must be answered) and every `admin.*` builder modal.

- [ ] **Step 3: The character menu (`character.go:64-92`)**

Build the menu from the same option list, with Default `quit`:

```go
	charMenu := menu.Menu{Heading: `What would you like to do?`, Default: `quit`}
	for _, opt := range menuOptions {
		charMenu.Entries = append(charMenu.Entries, menu.Entry{Id: opt, Label: opt})
	}
	question := cmdPrompt.AskMenu(charMenu)
	if !question.Done {
		user.SendText(messaging.CategoryScreen, menu.Render(*question.Menu))
		return true, nil
	}
```
The existing `question.Response == "quit"` checks keep working. The `RejectResponse` re-ask at `character.go:147` becomes `cmdPrompt.AskMenu(charMenu).RejectResponse()` with the same `charMenu` value rebuilt there (it carries only `quit`).

- [ ] **Step 4: Start's tutorial choice (`start.go:63`)**

```go
	question := cmdPrompt.AskMenu(menu.Menu{
		Heading: "How much of Gaius do you already know?",
		Entries: []menu.Entry{
			{Id: "1", Label: "New to text MUDs", Detail: "I'll teach you the basics first."},
			{Id: "2", Label: "New to DOGMud", Detail: "I know MUDs; show me what's different."},
			{Id: "3", Label: "Veteran", Detail: "Skip all tutorials; drop me into the city."},
		},
		Default: "2",
	})
	if !question.Done {
		user.SendText(messaging.CategoryScreen, menu.Render(*question.Menu))
		return true, nil
	}
```
The existing handling of `question.Response` (`"1"`, `"2"`, `"3"`) is unchanged.

- [ ] **Step 5: Picklock (`picklock.go:194`)**

```go
	question := cmdPrompt.AskMenu(menu.Menu{
		Heading: `Move your lockpick?`,
		Entries: []menu.Entry{{Id: `UP`, Label: `Up`, Aliases: []string{`u`}}, {Id: `DOWN`, Label: `Down`, Aliases: []string{`d`}}, {Id: `quit`, Label: `Stop picking`}},
	})
	if !question.Done {
		user.SendText(messaging.CategoryScreen, menu.Render(*question.Menu))
		return true, nil
	}
```
Check how the old code compared `UP`/`DOWN` (read the lines after `:199`) and keep those comparisons, since the Ids are unchanged.

- [ ] **Step 6: Build and test**

Run: `go build ./... && go test ./internal/usercommands/ ./modules/auctions/ -v`
Expected: PASS. Update any test that typed `Yes`/`y` to an old prompt only if it now fails, and keep its intent (a test that typed `y` should still work via the alias).

- [ ] **Step 7: Add the driving tests**

Create `internal/usercommands/confirm_menu_test.go`: for guild disband, character delete, mail send and auctions, drive the flow with `"1"`, with `"y"`, with `""` (must decline), and with `"north"` on a non-modal flow (must clear the prompt). For start, drive `""` (must pick 2).

- [ ] **Step 8: Commit**

```bash
gofmt -l internal/ modules/
git add internal/usercommands/admin.item.go internal/usercommands/admin.mudmail.go internal/usercommands/admin.room.exits.go internal/usercommands/admin.room.go internal/usercommands/admin.zone.go internal/usercommands/character.go internal/usercommands/deletecharacter.go internal/usercommands/guild.go internal/usercommands/mail.go internal/usercommands/picklock.go internal/usercommands/renameself.go internal/usercommands/start.go internal/usercommands/confirm_menu_test.go modules/auctions/auctions.go
git commit -F - <<'EOF'
refactor(prompts): confirms and player choices render through internal/menu (#375 phase 0)

Confirms are two entries, Enter picks No, and actions that cannot be
undone name the action. Player flows are non-modal, so a command typed
mid-choice closes the menu and runs; builders and start stay modal.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 16: Retire the templates and add the guard

**Files:**
- Delete: `_datafiles/world/dogmud/templates/tables/numbered-list.template`, `numbered-list-doubled.template`, and the same two under `_datafiles/world/default/templates/tables/`
- Create: `internal/usercommands/menu_guard_test.go`

- [ ] **Step 1: Confirm nothing references the templates**

Run: `grep -rn "numbered-list" --include=*.go --include=*.template --include=*.md internal modules _datafiles world.go main.go`
Expected: no Go or template hits. A doc hit is fine to leave if it describes history; a live doc describing current use gets updated in Task 17.

- [ ] **Step 2: Check `templates.NameDescription` still has users**

Run: `grep -rn "NameDescription" --include=*.go internal modules | grep -v _test`
Keep the type (the builders still build option lists with it).

- [ ] **Step 3: Delete the four template files**

```bash
git rm _datafiles/world/dogmud/templates/tables/numbered-list.template _datafiles/world/dogmud/templates/tables/numbered-list-doubled.template _datafiles/world/default/templates/tables/numbered-list.template _datafiles/world/default/templates/tables/numbered-list-doubled.template
```

- [ ] **Step 4: Write the guard test**

```go
package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The menu guard (#375 phase 0): every numbered choice renders and resolves
// through internal/menu. A numbered-list template render or the copied
// menu-index range check anywhere else is a regression.
var menuGuardPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"tables/numbered-list`),
	regexp.MustCompile(`restNum > 0 && restNum <= len\(`),
}

func menuGuardViolations(t *testing.T, roots ...string) []string {
	var out []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(filepath.ToSlash(path), "internal/menu/") {
				return nil
			}
			b, _ := os.ReadFile(path)
			for _, re := range menuGuardPatterns {
				if re.Match(b) {
					out = append(out, path+": "+re.String())
				}
			}
			return nil
		})
	}
	return out
}

func TestMenuGuard_NoNumberedMenusOutsideTheMenuPackage(t *testing.T) {
	if v := menuGuardViolations(t, "..", "../../modules"); len(v) > 0 {
		t.Fatalf("numbered menu code outside internal/menu:\n%s", strings.Join(v, "\n"))
	}
}

func TestMenuGuard_CanFail(t *testing.T) {
	dir := t.TempDir()
	bad := "package x\nfunc f() { _ = \"tables/numbered-list\"; if restNum > 0 && restNum <= len(xs) {} }\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := menuGuardViolations(t, dir); len(v) != 2 {
		t.Fatalf("guard missed a planted violation: %v", v)
	}
}
```

- [ ] **Step 5: Run the guard**

Run: `go test ./internal/usercommands/ -run TestMenuGuard -v`
Expected: both PASS. If the first fails, the listed file still has a numbered menu: migrate it (Task 14 pattern) rather than editing the guard.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/usercommands/menu_guard_test.go
git commit -F - <<'EOF'
test(menu): guard numbered menus to internal/menu; retire numbered-list templates (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 17: Docs

**Files:**
- Create: `internal/menu/context.md`
- Modify: `internal/prompt/context.md`, `internal/users/context.md`, `internal/state/context.md`, `internal/characters/context.md`, `modules/follow/context.md`, `modules/gmcp/context.md`, `.claude/skills/dogmud-player-copy/SKILL.md`, `docs/README.md`

- [ ] **Step 1: Write `internal/menu/context.md`**

Sections, each naming only symbols that exist (check with `Select-String -Path internal\menu\*.go -Pattern '^(func|type|const|var)\s'`): Purpose (one menu primitive, pure, no I/O); Files; Data model (`Entry`, `Menu` and every field, marker constants); `Render` layout rules (marker column, Detail wrap, Dim grey, Columns 2, footer rules); `Resolve` rules (game menus exact only and why "n"/"l" must stay commands; `Prefix`, `AcceptText`; `ResultKind` list); `Confirm`; `CloseReason`, `Session`, `InterruptLine`, `FollowLines`; `LintLabels`; the menu language conventions (the nine rules from spec section 8, verbatim); Who uses it (users game session, prompt menu questions, builders, confirms); the guard test.

- [ ] **Step 2: Update the other `context.md` files**

- `internal/prompt/context.md`: `Question.Menu`, `LastResult`, `Prompt.NonModal`, `AskMenu`, `AskConfirm`, `Resolve`; typed-only answers.
- `internal/users/context.md`: game menu session (`OpenMenu`, `CloseMenu`, `RouteMenuInput`, `CheckMenuInterrupts`, `CloseMenusForRoomChange`, `CloseMenuForFollow`, `MenuOrPromptOpen`, `HasOpenMenu`), `menuGate` rules and the #640 note, `MenuChanged`.
- `internal/state/context.md`: the five new `RegisterObserver` methods.
- `internal/characters/context.md`: `BusyStateChanged`, `WireBusyObservers`.
- `modules/follow/context.md`: `actions.BreakOffMenuToFollow` before the follower step.
- `modules/gmcp/context.md`: `Char.Menu {open}` and the `Char.Action.Try` defer.

- [ ] **Step 3: Player-copy skill**

Add a "Menu language" section to `.claude/skills/dogmud-player-copy/SKILL.md` with the nine conventions and a pointer to `internal/menu/context.md`.

- [ ] **Step 4: README and audit**

Add `internal/menu/` to `docs/README.md` where packages are listed (search for an existing `internal/` package row and match its format). Then:

Run: `python tools/context_md_audit.py`
Expected: no phantom symbols reported for the files touched.

- [ ] **Step 5: Commit**

```bash
git add internal/menu/context.md internal/prompt/context.md internal/users/context.md internal/state/context.md internal/characters/context.md modules/follow/context.md modules/gmcp/context.md .claude/skills/dogmud-player-copy/SKILL.md docs/README.md
git commit -F - <<'EOF'
docs(menu): context.md for the menu primitive and its callers (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 18: Full verification

- [ ] **Step 1: Format, build, full suite, race on the touched packages**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./...
go test -race ./internal/menu/ ./internal/users/ ./internal/prompt/ ./internal/characters/ ./internal/hooks/ ./internal/actions/
```
Expected: no gofmt output; build clean; every package `ok`; race clean. Report any failure with its output; do not claim success without it.

- [ ] **Step 2: Lint**

Run: `golangci-lint run --new-from-merge-base=origin/master`
Expected: `0 issues`.

- [ ] **Step 3: JS tests**

Run: `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED: $t"; done`
Expected: no `FAILED:` line.

- [ ] **Step 4: Boot check in a detached worktree (never the owner's server)**

Follow `dogmud-shipping` "Boot check in an isolated worktree": `git worktree add --detach C:/tmp/dogmud-boot-check HEAD`, build `boot-check.exe` there, run it on private ports, confirm `Server Ready`, kill it by its PID, remove the worktree.

- [ ] **Step 5: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`, player framing, no numbers, no dashes: choices now show as numbered lists everywhere; type the number or press Enter; typing another command leaves the choice; a fight, sleep or moving away ends it; web client ticks and triggers wait while you choose and no longer fire while you log in.

- [ ] **Step 6: Commit and report**

```bash
git add docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs(patch-notes): numbered menus everywhere (#375 phase 0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```
Report to the controller: the test, lint, JS and boot output, and the list of any queue defects Task 12 found and fixed. Opening the PR is the controller's step (`gh pr create --repo pruuk/DOGMud`).
