# Web client base, plan 5: the game window goes live

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Game text is one fixed size on every screen and is never resized to make content fit. The game window sits on the left at 80 columns and changes width only when the player changes the browser. The panels arrange themselves around it, and the web client tells the server its width, so phones finally get the narrow forms plans 3a to 4 built.

**Architecture:** `termfit.js` stops shrinking the font to fit 80 columns. The font is the `--text` token, and the terminal gets 80 columns, or fewer only when the browser is too narrow for 80. It reports the pixel width 80 columns need, and a new pure `layoutpick.js` uses that to choose one of four layouts (full Layout D, the one-column step, the slim column, or the phone layout). `dashboard.js` applies the chosen layout with CSS grid taken from the approved mock. A new `displayreport.js` sends `Client.Display {"cols": n}` when the column count settles; the server handler from plan 1 already stores it. The four screens ruling 3 let scroll sideways cannot scroll in the web game window, so they get narrow forms first (owner ruling below).

**Tech Stack:** Go (server, `text/template` forms), plain browser JavaScript with xterm 4.19, node tests in `tools/jstest`.

Spec: `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` (approved 2026-11-09), sections 1, 2, 4 and the large-screen half of 5, and the plan table rows 5 and 6. Parent epic #783. Approved layout mock: https://claude.ai/artifact/B4v8C7CerfMM3biD8WwzwE (version 7). Reference look: https://claude.ai/artifact/299yMwQcZ6kWdxQhnMb3zg, source `_archive/notes/reports/full-client-mock/` in the main checkout (gitignored), whose `static/css/mock-overrides.css` section 4 holds the Layout D and tablet grid CSS.

---

## Facts verified against source

Read 2026-11-12 from branch `feature/webclient-base-4-help` at `b39f53b45` (PR #863, open). `WP` is `_datafiles/html/public/webclient-pure.html`; `JS` is `_datafiles/html/public/static/js`.

| Fact | Where |
|---|---|
| `TermFit` keeps at least 80 columns: `TARGET = 80, MAX_FS = 20, STEP = 0.25`, shrinks the font until 80 fit, then CSS `scale()` below the browser minimum | `JS/termfit.js:20`, `:43-128` |
| xterm is built with `cols: 80, rows: 60, fontSize: 20`, `fontFamily` from `--font-mono` | `WP:433-447` |
| A measure monkeypatch divides xterm's cell size by `termScale` | `WP:464-485` |
| `browserMinFont` probes the smallest font the browser renders | `WP:500-516` |
| `TermFit.create({term, fitAddon, host, minFont, setScale})`, called from `resizeTerminal(force)` on window resize, a ResizeObserver on the feed panel and body, and font loads | `WP:518-584` |
| `--text` is 16px; `:root.upright-phone` makes it 14px; the class is toggled on resize and orientation change | `static/css/gomud.css:166`, `:199`; `JS/viewportfit.js` `watchUprightPhone` |
| The feed never scrolls sideways (`overflow-x: hidden`), and `feedscroll.js` snaps back any drift (playtest 2026-10-17) | `static/css/dashboard.css:219`; `JS/feedscroll.js:50` |
| `SendGMCP(pkg, obj)` sends a binary IAC SB GMCP frame; returns false when the socket is not open | `WP:3202-3212` |
| `socket.onopen` | `WP:2893` |
| Server side of `Client.Display` is done (plan 1): websocket only, whole numbers only, clamped 30 to 80, stored only on change, deferred to MainWorker | `modules/gmcp/gmcp.ClientDisplay.go`; `modules/gmcp/gmcp.go:431` |
| `WrapWidth()` on the web is min(reported, 80, LineWidth), at least 30 | `internal/users/wrapwidth.go:46-55` |
| Template width is read per render (`readerWidth(userId)`), not cached, so a width report needs no cache clear | `internal/templates/templates.go:165-171`; `internal/templates/forms.go:71` |
| Forms: `name.w42.template`, `.w40`, `.w30` beside the base; `FormWidths = {42, 40, 30}`; the widest form that fits is used when the base does not fit | `internal/templates/forms.go:27-56`, `templates.go:205-240` |
| Go-built screens pick a form width with `users.RedrawnFormWidth(user.WrapWidth())` (80 at 80 and over, else 42 or 30) | `internal/users/wrapwidth.go`; used in `internal/usercommands/craft.go:462`, `guild.go:201`, `achievements.go:59`, `internal/actions/scan.go:124` |
| The `map` command: standard map 65 by 21, frame 14 columns (`mapBorderWidth`), `map wide` at level 4 fills `Display.ScreenWidth - 14`; renders `maps/map` | `internal/usercommands/skill.map.go:66-95`, `:146-147`, `:202` |
| `maps/map.template` draws the rope frame from `.LeftBorder`, `.MidBorder`, `.RightBorder`; the legend wraps at `.LegendWidth` with a 7-space indent | `_datafiles/world/dogmud/templates/maps/map.template` |
| `maps/secret` has no caller | `internal/guards/template_width_guard_test.go:75` |
| Lockpicking: `GetLockRender(sequence, entered, viewingUserId)` builds three rows of 5-wide cells; the template adds a 2-space indent, `║ cell ║`, so a lock of n steps is `3 + 8n` columns; 10 steps is 83 | `internal/usercommands/picklock.go:329-388`; `templates/tables/lockpicking.template` |
| `GetLockRender` callers: two, both `user.SendText(CategoryScreen, ...)` | `picklock.go:190`, `:261` |
| `help colors` grids are 48, 36 and 32 columns (4-space indent plus 4-wide swatches `" 016"`); `help map-examples` shows two 79-column example maps | `HELP_DUMP` render 2026-11-12 |
| Template guard class `"wide"` = `maps/map`, `tables/lockpicking`, `help/map-examples`, `help/colors`; class `"fits in usercommands"` is held by `TestNarrowTemplateScreensFitEveryWidth` | `internal/guards/template_width_guard_test.go:58`, `:67-69` |
| `helpWide` in the help guards = `help/map-examples`, `help/colors` | `internal/guards/help_markup_guard_test.go` |
| `helpTemplateFiles` globs `help/*.template`, so a form file `colors.w42.template` would become a topic `colors.w42` | `internal/guards/help_markup_guard_test.go:56-72` |
| `motdBox` gives a narrow reader the ruled form (`motdRuled`), no box; `MotdRestyle.restyle` matches only the box, so it passes the ruled form through | `internal/usercommands/motd.go:39-50`; `JS/motdrestyle.js` header |
| jstest runs every `tools/jstest/*.test.js` in CI | `.github/workflows/validate.yml:93` |
| Dashboard modes today: `wide` (dashboard width 1100 and up, three columns), `rail` (700 to 1100, side panels in a tabbed rail), `phone` (under 700, tab bar and slide-up drawer); `_computeMode`, `_apply`, `_enterWide`/`_enterRail`/`_enterPhone` | `JS/dashboard.js:1-6`, `:242-272`, `:308-553` |
| Wide grid today: `minmax(180px, 22vw)`, 6px splitter, `1fr` game, 6px, `minmax(180px, 22vw)`; rail grid `minmax(0, 2.3fr) minmax(280px, 1fr)` | `static/css/dashboard.css:11-18`, `:437-442` |
| Panels: `map`, `quests`, `vitals` (Health), `art` (Inventory), `chat`, `status` (Conditions), `trig` (Automation) in `#dash-col-left`/`#dash-col-right`; `feed` and `cmd` in `#dash-col-center` | `WP:280-392`; `JS/dashboard.js:11`, `:17-25` |
| The tab bar component `makeTabBar(extraClass, onPick)`: brass circles, one name pill, used by the rail and the phone bar | `JS/dashboard.js:33-122`; `static/css/dashboard.css:460-555` |
| Column splitters (`initSplitters`), panel drag-rearrange (`initRearrange`), pop-out (`initPopout`), saved layout key `dogmud.dashboard.layout.v1` | `JS/dashboard.js:636-1060` |
| Pop-outs are a wide-mode feature: docked when the screen narrows, popped again on return | `JS/dashboard.js:260-303` |
| The approved mock already has Layout D (`data-mode="wide"`) and the sideways-tablet arrangement (`data-mode="tablet"`) as CSS grid, with the side panels wrapped in a `#dash-panels` container | `_archive/notes/reports/full-client-mock/static/css/mock-overrides.css:159-320`; its `index.html:30`, `:640-672` |
| JetBrains Mono letters are 0.6em wide; xterm rounds its cell to device pixels, so 80 columns at 16px are about 720 to 768px plus the 15 to 17px scrollbar | xterm `_updateDimensions`; BAR client finding 2026-11-12 |

## Owner rulings for this plan

- **Wide screens fit the reader on the web (owner 2026-11-12, Option 1 on the mock https://claude.ai/artifact/6h9WMPzFrPomRuvqKJot4g).** The web game window cannot scroll sideways, so the `map` command, lockpicking, `help colors` and `help map-examples` fit the reader's width instead. The map is drawn to the width around you with its frame reduced to rules; lock steps go in rows that fit (lockpicking is already over 80 on desktop, so this fixes 80 too); the colour swatches lose their gaps. This amends spec ruling 3: no screen is exempt from fitting any more. The `"wide"` class and `helpWide` go.
- **The game never resizes the game window or its text to make content fit (owner 2026-11-12).** Content is designed to fit the window. The text is the `--text` token (16px, 14px on an upright phone) and never changes. The game window's width changes only when the player changes it: dragging the browser, zooming, or turning a phone. On a phone or tablet the keyboard and toolbars change only the height. Rejected along the way: desktops shrinking the text until a later plan, clipping the game window, centring it at mid sizes, an empty hatched box beside it, and a vertical button bar.
- **The layout ladder (owner 2026-11-12, mock version 7).** The game window is on the left, exactly as wide as 80 columns. As the browser narrows, only the panels change:

  | Room right of the game | Layout |
  |---|---|
  | 360px or more | Full Layout D: a square Map with Health over Conditions beside it; below, 60/40, Comms over Quests and Inventory over Automation (spec ruling 9) |
  | 200 to 359px | One column: Map, Health and Conditions stacked; Comms, Quests, Inventory and Automation open from four round buttons at the column's foot (spec ruling 10, now applied by fit) |
  | 120 to 199px | Slim column: Health over Conditions beside the game; Map, Comms, Quests, Inventory and Automation on the tab bar under the game |
  | Under 120px | The phone layout: the game window's dark background reaches the right edge, the text stays at 80 columns, every panel on the tab bar under the game |
  | Browser narrower than 80 columns | The phone layout, and the game window follows the browser with fewer columns; the server sends narrow screens |

  The tab bar is always horizontal. A phone (a touch screen whose short side is under 600px; decision 8) always gets the phone layout.
- Spec rulings 1 to 3 and design sections 1, 2 and 4 stand otherwise. This plan takes over the large-screen half of spec section 5 from plan 6; plan 6 keeps the 30% tab height, the keyboard rules, the X button and the sideways toast.

## Decisions taken without asking

1. **Telnet gets the narrow forms too.** Forms follow `WrapWidth()`, and telnet's is `set linewidth` (40 to 240). A telnet player at `set linewidth 40` gets the fitted map. If wrong: keep a telnet branch that always draws the wide forms; nothing else changes.
2. **Lock rows keep the 2-space indent.** So the template stays one file and a row holds `(width - 3) / 8` steps: 9 at 80, 4 at 42, 3 at 30. The mock showed 5 at 42 without the indent. If wrong: a `.w42`/`.w30` lockpicking form without the indent gives 5 and 3.
3. **A narrow map is the reader's full width (30 to 79) and 21 rows tall**, as the standard map is. `map wide` on a narrow screen draws the narrow map (it cannot be wider than the screen). If wrong: a different height is one constant.
4. **The font is the `--text` token, never below the browser minimum.** A raised "Minimum font size" setting gets that size instead, which is what the current code does at its floor, minus the `scale()`.
5. **The width is sent on connect, then only on a column change after 250ms without one.** On a reconnect it is sent again, because the server's new connection starts at 80.
6. **Scrollback may lose its oldest lines when the window narrows.** xterm rewraps wrapped lines on a resize and the scrollback limit then drops the oldest. This happens once per narrowing, not on every keyboard move (a height-only change never resizes columns). The test pins that the newest lines survive.
7. **Below 30 columns (a window under about 300px), the terminal still uses what fits** and the server holds the text at 30, so lines wrap at the edge. Nothing ships smaller than 320px.
8. **The ladder is chosen by width on every device that is not a phone.** A 1024 sideways tablet gets the one-column step, which is ruling 10's arrangement; a 1366 sideways iPad Pro gets full Layout D. "A phone" is a touch screen (`pointer: coarse`) whose short side is under 600px, so a zoomed-in desktop on a small screen still gets the ladder. If wrong: a touch screen is held at the one-column step at most.
9. **Column splitters, drag-rearranging panels and the rail mode retire** (spec choices section, now in this plan). Folding a panel and pop-out stay; pop-outs exist only in full Layout D, as today they exist only in `wide`. The saved-layout key moves to `dogmud.dashboard.layout.v2`, keeping only folded and popped-out panels, so an old saved arrangement cannot fight the ladder. Reset layout stays.
10. **The one-column step stacks Map, Health and Conditions** (as in mock version 7), not the reference mock's Health and Status side by side, because the column is only 200 to 359px wide.
11. **An open panel from a tab bar** opens where it does today on the phone (the drawer over the lower part of the game), except in the one-column step, where it opens over the lower part of that column, as the reference mock's tablet does (`mock-overrides.css:313-320`).
12. **The keyboard tab order puts the game and command box before the panels**, a side effect of wrapping the panels as the mock does. If wrong: `tabindex` order can restore the old sequence.
13. **The 14px upright-phone size needs a touch screen too**, the same test as decision 8, so a zoomed portrait desktop monitor keeps 16px. If wrong: drop the touch test in `viewportfit.js`.
14. **An old saved layout carries over once**: folded panels, popped-out panels and their window places move from the `v1` key to `v2`; the column arrangement and widths are dropped.
15. **The thresholds are measured, not written down.** `layoutpick.js` compares the room beside the game, using the game width termfit reports from the real letters, against 360, 200 and 120px. Those three numbers are the only constants.

Added during the build (2026-11-12, Task 9a; reviewed by a BAR round 2026-11-13 before the owner test):

16. **The web client sends its columns on the websocket address (`/ws?cols=N`)** as well as by `Client.Display`, because the server sends the connect splash the moment the socket opens, before any GMCP report can arrive. The value is parsed as a whole number and clamped exactly as `Client.Display` is (one shared `users.StoreReportedCols`). If wrong: drop the query value and the splash goes back to 80 columns on every screen; nothing else depends on it until login.
17. **The connect splash, the login prompts and the goodbye get words-only narrow forms** (spec ruling 23: full art or words only). The splash's 42 and 30 forms drop the logo; the goodbye's narrow form is "Goodbye.". If wrong: the forms are template files; deleting them restores the art at every width.
18. **Login prompts wrap to the connection's width before there is a user** (`users.LoginWrapWidth`: the websocket's stored columns, else 80; telnet is always 80 and byte for byte unchanged). A prompt keeps its trailing ": " on its last line (`messaging.WrapPrompt`) so the cursor sits after it. If wrong: prompts go back to `templates.Process` at 80.
19. **A script error on `webclient.html` while the game frame is still loading is fixed in this plan** although it was already on master, because plan 5's phone check is what found it.

---

## File structure

Server:
- Modify `internal/usercommands/skill.map.go`: `mapSizeForReader` picks the narrow map size; `Map` uses it.
- Create `_datafiles/world/dogmud/templates/maps/map.w42.template` and `map.w30.template`: title, rules, map lines, legend, no rope frame.
- Modify `internal/usercommands/picklock.go`: `GetLockRender` splits the steps into rows that fit the viewer.
- Create `_datafiles/world/dogmud/templates/help/colors.w42.template`, `colors.w30.template`, `map-examples.w42.template`, `map-examples.w30.template`.
- Modify `internal/guards/template_width_guard_test.go`, `internal/guards/help_markup_guard_test.go`, `internal/guards/help_render_guard_test.go`: the wide classes go; form files are not topics.
- Create `internal/usercommands/skill.map_narrow_test.go` and `picklock_render_test.go`: they hold `maps/map` and `tables/lockpicking` for the guard.

Client:
- Rewrite `JS/termfit.js`: fixed font; 80 columns, or what fits when the browser is narrower; reports the pixel width of 80 columns.
- Create `JS/layoutpick.js`: a pure function from the dashboard width, the game width and "is this a phone" to one of `wide`, `column`, `slim`, `phone`.
- Create `JS/displayreport.js`: settles and sends the column count.
- Modify `JS/dashboard.js`: the four modes; rail, splitters and rearrange retire; layout key `v2`.
- Modify `static/css/dashboard.css`: the four modes' grids, from the mock's section 4.
- Modify `WP`: the `#dash-panels` wrapper; drop the scale monkeypatch and `setScale`; wire termfit, layoutpick, dashboard and displayreport together.
- Rewrite `tools/jstest/termfit.test.js`; create `tools/jstest/layoutpick.test.js` and `tools/jstest/displayreport.test.js`; extend `tools/jstest/motdrestyle.test.js`; check `tools/jstest/dashboard-popout-layout.test.js` still holds.

Docs: `JS` has no context.md; update `internal/templates/context.md`, `internal/usercommands/context.md`, `internal/guards/context.md`, the spec (ruling 3 amendment line), `docs/PATCH_NOTES.md`, `docs/README.md` (new files).

---

### Task 0: Worktree

- [ ] **Step 1: Create the worktree from plan 4's branch** (plan 5 needs plan 4; rebase onto master once #863 merges)

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add C:/tmp/dogmud-p5 -b feature/webclient-base-5-live origin/feature/webclient-base-4-help
```

- [ ] **Step 2: Land this plan in the worktree and commit it**

```bash
cp "C:/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-12-webclient-base-5-live-window.md" C:/tmp/dogmud-p5/docs/superpowers/plans/
cd C:/tmp/dogmud-p5
git add docs/superpowers/plans/2026-11-12-webclient-base-5-live-window.md
git commit -m "docs(plan): web client base plan 5, the game window goes live"
```

Then delete the untracked copy in the main checkout. Record `C:/tmp/dogmud-p5` in memory `project-touch-arc-base-first`.

---

### Task 1: The map fits the reader

**Files:**
- Modify: `internal/usercommands/skill.map.go:79-95`, `:144-147`, `:222-235` (`mapScreenData`)
- Create: `_datafiles/world/dogmud/templates/maps/map.w42.template`, `maps/map.w30.template`
- Test: `internal/usercommands/skill.map_narrow_test.go` (new)

- [ ] **Step 1: Write the failing test for the size**

```go
package usercommands

import "testing"

// A reader under 80 columns gets a map drawn to the narrow form width with
// no frame; at 80 and over the map is as before (owner 2026-11-12, plan 5).
func TestMapSizeForReader(t *testing.T) {
	cases := []struct {
		name          string
		level         int
		wide          bool
		screenW, wrap int
		w, h          int
		narrow        bool
	}{
		{"desktop", 1, false, 80, 80, 65, 21, false},
		{"wide on a big telnet", 4, true, 200, 80, 186, 21, false},
		{"phone", 1, false, 42, 42, 42, 21, true},
		{"phone asking wide", 4, true, 42, 42, 42, 21, true},
		{"between", 1, false, 60, 60, 60, 21, true},
		{"just under 80", 1, false, 79, 79, 79, 21, true},
		{"narrowest", 1, false, 30, 30, 30, 21, true},
	}
	for _, c := range cases {
		w, h, narrow := mapSizeForReader(c.level, c.wide, c.screenW, 40, c.wrap)
		if c.name == "wide on a big telnet" {
			h = 21 // height follows the screen there; not under test
		}
		if w != c.w || h != c.h || narrow != c.narrow {
			t.Errorf("%s: got %dx%d narrow=%v, want %dx%d narrow=%v", c.name, w, h, narrow, c.w, c.h, c.narrow)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/usercommands/ -run TestMapSizeForReader -count=1`
Expected: FAIL, `undefined: mapSizeForReader`.

- [ ] **Step 3: Implement `mapSizeForReader` and use it in `Map`**

Add below `mapSizeFor`:

```go
// mapSizeForReader is the map's size for a reader whose text wraps at wrap
// columns. Under 80 the map has no frame and is the reader's whole width
// (at least 30); `map wide` cannot be wider than the screen there. At 80
// and over it is mapSizeFor.
func mapSizeForReader(skillLevel int, wide bool, screenWidth, screenHeight, wrap int) (int, int, bool) {
	if wrap < users.MaxAutoWrapWidth {
		return max(wrap, users.MinWrapWidth), standardMapHeight, true
	}
	w, h := mapSizeFor(skillLevel, wide, screenWidth, screenHeight)
	return w, h, false
}
```

In `Map`, replace `mapWidth, mapHeight := mapSizeFor(skillLevel, rest == "wide", sw, sh)` with:

```go
	mapWidth, mapHeight, narrow := mapSizeForReader(skillLevel, rest == "wide", sw, sh, user.WrapWidth())
```

and replace the `templates.Process("maps/map", ...)` call with a render at a chosen width, so the form is picked by band and never by measuring:

```go
	// A narrow reader gets the form drawn to its band (42 or 30), whatever
	// the frame would measure; anyone else gets the framed base, even a
	// telnet `map wide` wider than 80 (measuring would have swapped that
	// for a frameless form).
	formWidth := templates.FullWidth
	if narrow {
		formWidth = users.RedrawnFormWidth(user.WrapWidth())
	}
	mapTxt, err := templates.ProcessForWidth("maps/map", mapScreenData(room.Zone, displayLines, width, legend), user.ScreenReader, formWidth)
```

`ProcessForWidth` (`internal/templates/forms.go:105`) uses `formFor`, which returns the base at 80 and over and the form for the band below. The narrow forms draw everything from `.Width` and `.LegendWidth`, so a `.w42` form rendered for a 60-column reader with a 60-column map is 60 wide. `ProcessForWidth` is needed for the other direction: `Process` measures, and would swap a telnet `map wide` wider than 80 for a frameless form. Check `mapScreenData` passes `Width` and `LegendWidth` from the map's own width (it does: `width` is the rendered first line).

- [ ] **Step 4: Write the narrow forms**

`maps/map.w42.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="20">Map for {{ .Title }}</ansi>
<ansi fg="black-bold">{{ padRightX "" "─" .Width }}</ansi>
{{ range $index, $line := .DisplayLines -}}
<ansi fg="map-default">{{ $line }}</ansi>
{{ end -}}
<ansi fg="black-bold">{{ padRightX "" "─" .Width }}</ansi>
<ansi fg="map-legend">Legend</ansi>
{{- $strlen := 6 -}}
{{ range $symbol, $name := .Legend -}}
{{- $proposedLength := add $strlen 3 (len $name) 1 -}}
{{- if gt $proposedLength $.LegendWidth -}}{{- $strlen = 0 -}}{{ "\n" }}{{- end -}}
<ansi fg="map-legend"> [</ansi><ansi fg="map-room"><ansi fg="map-{{mapslug $name}}" bg="mapbg-{{mapslug $name}}">{{ printf "%c" $symbol }}</ansi></ansi> {{$name}}
{{- $strlen = add $strlen 3 (len $name) 1 -}}
{{- end }}
```

The space before `[` separates the entries, as in the base (`map.template:20`); a space after `{{$name}}` would be trimmed by the next line's `{{-`. `$strlen` starts at 6 because `Legend` shares the first line. A wrapped line starts with that same space, so it is one column in.

`maps/map.w30.template` is the same, but its title line is `<ansi fg="black-bold">.:</ansi> <ansi fg="20">{{ .Title }}</ansi>`, without `Map for `: the longest zone name (`New Plymouth Old Quarter`, 24) with `.: Map for ` is 35, over 30. Find the longest zone name yourself before trusting this: `grep -h "^name:" _datafiles/world/dogmud/zones/*/zone-config.yaml` or wherever zone names live (`rooms.FindZoneName`).

Check `padRightX`, `mapslug` and `add` are template funcs (`internal/templates/templatesfunctions.go`: `padRightX` `:36`, `add` `:54`, `mapslug` `:90`).

- [ ] **Step 5: Run the size test and the existing map tests**

Run: `go test ./internal/usercommands/ -run "TestMap|Legend|Symbol" -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/skill.map.go internal/usercommands/skill.map_narrow_test.go _datafiles/world/dogmud/templates/maps/map.w42.template _datafiles/world/dogmud/templates/maps/map.w30.template
git commit -m "feat(map): draw the map to a narrow reader's width, with rules for a frame"
```

---

### Task 2: Lock steps in rows that fit

**Files:**
- Modify: `internal/usercommands/picklock.go:329-388`
- Test: `internal/usercommands/picklock_render_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
)

// Every lock, 2 to 32 steps, fits every width, and no step is lost
// (owner 2026-11-12, plan 5: lockpicking was over 80 already).
func TestLockRenderFitsEveryWidth(t *testing.T) {
	useDogmudTemplates(t) // TestMain points DataFiles at an empty folder
	// util.GetLockSequence clamps a lock to 2 to 32 steps.
	for _, width := range []int{30, 42, 80} {
		for n := 2; n <= 32; n++ {
			seq := strings.Repeat("UD", 16)[:n]
			out := lockRenderAt(seq, seq[:n/2], width, false)
			if w := templates.WidestLine(out); w > width {
				t.Errorf("a %d-step lock at %d is %d wide:\n%s", n, width, w, out)
			}
			if got := strings.Count(out, "?"); got != n-n/2 {
				t.Errorf("a %d-step lock at %d shows %d unknown steps, want %d", n, width, got, n-n/2)
			}
		}
	}
}

func TestLockRowsHoldWhatFits(t *testing.T) {
	for _, c := range []struct{ width, per int }{{80, 9}, {42, 4}, {30, 3}} {
		if got := lockStepsPerRow(c.width); got != c.per {
			t.Errorf("at %d a row holds %d steps, want %d", c.width, got, c.per)
		}
	}
}
```

`useDogmudTemplates` is at `internal/usercommands/template_freeze_test.go:46`. The render holds `<ansi>` tags, not escape codes, and `templates.WidestLine` measures through them (`forms.go:91`). Before trusting a green run, break it once: make `lockStepsPerRow` return 99 and see the width check fail.

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/usercommands/ -run "TestLock" -count=1`
Expected: FAIL, `undefined: lockRenderAt`.

- [ ] **Step 3: Implement**

Split `GetLockRender` so the width is a parameter, and render one table per row of steps:

```go
// lockStepsPerRow is how many lock steps fit a row at width: the template
// indents 2 and each step is 8 columns ("║ cell ") plus the closing "║".
func lockStepsPerRow(width int) int {
	return max(1, (width-3)/8)
}

func GetLockRender(sequence string, entered string, viewingUserId int) string {
	width, screenReader := templates.FullWidth, false
	if u := users.GetByUserId(viewingUserId); u != nil {
		width, screenReader = u.WrapWidth(), u.ScreenReader
	}
	return lockRenderAt(sequence, entered, width, screenReader)
}

// lockRenderAt draws the lock for a reader width columns wide: one table of
// as many steps as fit, then the next, so no lock is wider than the screen.
func lockRenderAt(sequence string, entered string, width int, screenReader bool) string {
	// ...today's body builds rows[0..2] and formatting for every step...
	per := lockStepsPerRow(width)
	var out strings.Builder
	for start := 0; start < len(sequence); start += per {
		end := min(start+per, len(sequence))
		// The template prints ".: " before the title, so a title has
		// width-3 columns. "Lock steps 28 to 32:" is the longest, 20.
		title := `The Lock Sequence Looks like:`
		if len(sequence) > per || 3+len(title) > width {
			title = fmt.Sprintf(`Lock steps %d to %d:`, start+1, end)
		}
		part := make([][]string, len(rows))
		for i := range rows {
			part[i] = rows[i][start:end]
		}
		tbl := templates.GetTable(title, rows[0][start:end], part, formatting[start:end])
		txt, _ := templates.ProcessForWidth("tables/lockpicking", tbl, screenReader, width)
		out.WriteString(txt)
	}
	return out.String()
}
```

`templates.ProcessForWidth(name, data, screenReader, width)` is at `internal/templates/forms.go:105`; `templates.FullWidth` is at `forms.go:25`. Keep today's row-building code unchanged above the loop. A one-row lock keeps today's title where it fits (at 32 and wider); otherwise each row is titled `Lock steps 1 to 3:` and so on. An empty sequence must still draw today's titled empty table (guard the loop: when `len(sequence) == 0`, render once with no steps), so `TestGetLockRender` keeps its meaning. Confirm `util.GetLockSequence` really clamps to 2 to 32 steps before relying on the test's range; if it does not, test up to its real maximum. In the test, call `lockRenderAt(seq, seq[:n/2], width, false)`.

`fixed_width_screen_guard_test.go:66` follows `templates.Process` only, not `ProcessForWidth`; the two sends stay held because `GetLockRender` is listed in that guard's `fixedWidthProducers`. Leave it listed.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/usercommands/ -run "TestLock|Picklock|picklock" -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/picklock.go internal/usercommands/picklock_render_test.go
git commit -m "feat(picklock): show a lock in rows of steps that fit the screen"
```

---

### Task 3: `help colors` and `help map-examples` narrow forms

**Files:**
- Create: `_datafiles/world/dogmud/templates/help/colors.w42.template`, `colors.w30.template`, `map-examples.w42.template`, `map-examples.w30.template`
- Modify: `internal/guards/help_markup_guard_test.go` (`helpTemplateFiles`, `helpWide`), `internal/guards/help_render_guard_test.go`

- [ ] **Step 1: Make the guards see form files as forms, not topics**

In `helpTemplateFiles`, skip a file whose base name ends `.w<digits>` or `.screenreader`, and add them to a second map `helpFormFiles` that `TestHelpFilesStateTheirLayout` also checks (a form is a help file and must state its layout too). Delete `helpWide` and every use of it.

Then fix where the markup guard judges `::keep` widths (`help_markup_guard_test.go:152-155`, today 30 for every file not in `helpWide`): a file is judged at the narrowest width it is ever shown at, which is the narrowest width `formFor` (`forms.go:113-125`) maps to it. A base with forms is only shown at 80 and wider, so its `::keep` lines are judged at 80. A `.w42` form is judged at 42 when the page also has a `.w30`, and at 30 when it is the page's only form (`formFor` then shows it below 42 too). A `.w30` form and a base with no forms are judged at 30. The render guard then fails `help colors` and `help map-examples` at 30 and 42: that is the red.

Run: `go test ./internal/guards/ -run "Help" -count=1`
Expected: FAIL on `colors` and `map-examples` at 30 and 42.

- [ ] **Step 2: Write `colors.w42.template`**

Copy `colors.template`. In every swatch grid, drop the 4-space indent and each swatch's leading space, so a swatch is 3 wide with its background colour and no gap. The base grids are 48, 36 and 32 columns (11, 8 and 7 swatches); they become 33, 24 and 21. Keep every number and every colour. The grids stay in `::keep` blocks, judged at 42 by step 1.

- [ ] **Step 3: Write `colors.w30.template`**

Like `.w42`, but each 11-swatch grid is drawn as two grids one under the other: swatches 1 to 6 (18 columns), then 7 to 11 (15 columns), each with a one-line heading under 30 columns such as `Rainbow, part 1:`. Row k of the first half followed by row k of the second half is row k of the base, swatch for swatch; the parity check in step 5 joins them that way before comparing.

- [ ] **Step 4: Write `map-examples.w42.template` and `.w30.template`**

Same words as the base, except the closing note "You can see roughly 4x as much area in the wide map": on a narrow screen `map wide` draws the narrow map (decision 3), so the forms say instead that `map wide` shows more only on a wide screen. Each example map is redrawn the way the narrow `map` command draws it (Task 1): title line, a rule the width of the form, the map cut to the form width around the `@`, a rule, the legend wrapped to the width. Take the map lines from the base's `::keep` block: they are `( _ __)` + 65 map columns + `( _ __)`; the map columns are characters 8 to 72. Keep the colour tags on the cells you keep.

- [ ] **Step 5: Run the help guards**

Run: `go test ./internal/guards/ -run "Help" -count=1`
Expected: PASS: every page fits 30, 42 and 80, and the word check passes.

`TestHelpLayoutKeepsEveryWord` renders each page at 30 (`help_render_guard_test.go:131`), which picks the `.w30` form, so neither the base nor the `.w42` form of `colors` and `map-examples` is word-checked. Make it render every file at the width it is shown at (a base with forms at 80, each form at its own width, every other page at 30 as today) and check each render against that file's own layout. For `colors` only, add a check that each form holds the same multiset of numbers as its base, in the same order within each grid row (the `.w30` halves joined back as step 3 says), and see it fail once by deleting a swatch from a form. For `map-examples` (whose only number, "4x", the forms drop on purpose), check instead that each form's map rows are slices of the base's map columns (characters 8 to 72 of each `::keep` row) around the `@`, and see it fail once by changing a cell.

`allHelpTemplatePaths` in `internal/templates/u8_help_test.go:157-178` reads `help/` with `os.ReadDir`, so the forms also become `help/colors.w42` and the like in the numeric and cross-reference guards there. Run `go test ./internal/templates/ -count=1`; if a guard there treats a form as a page (for example demanding it be in the help index), make it skip `.wNN` files.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/colors.w42.template _datafiles/world/dogmud/templates/help/colors.w30.template _datafiles/world/dogmud/templates/help/map-examples.w42.template _datafiles/world/dogmud/templates/help/map-examples.w30.template internal/guards/help_markup_guard_test.go internal/guards/help_render_guard_test.go
git commit -m "feat(help): narrow forms for help colors and help map-examples; no help page is exempt"
```

---

### Task 4: No template is exempt from fitting

**Files:**
- Modify: `internal/guards/template_width_guard_test.go:55-60`
- Modify: `internal/usercommands/skill.map_narrow_test.go` (from Task 1)

- [ ] **Step 1: Replace `"wide"` with a class for screens their command sizes**

`"fits in usercommands"` cannot hold these two: `TestFitsInUsercommandsClassIsHeld` (`template_width_guard_test.go:301-311`, `:372-375`) requires `.w42` and `.w30` forms and one fixed data set in `narrowTemplateScreens`, and the map's data depends on the width while the lock has no forms at all. So:

```go
	// Sized by their command for the reader's width, held by the
	// command's own tests in internal/usercommands: the map
	// (TestMapRendersFitEveryWidth) and lockpicking
	// (TestLockRenderFitsEveryWidth).
	"sized by its command": {"maps/map", "tables/lockpicking"},
```

Delete the `"wide"` entry and its comment, and rewrite the file's header comment ("Only 'wide' and the classes...") and the comment at `:173-174` that still names `"wide"`, to say no class exempts a screen from fitting; every class names who holds it. Add a guard check that the two named test functions exist in `internal/usercommands` (grep the package's `_test.go` files for `func TestMapRendersFitEveryWidth(` and `func TestLockRenderFitsEveryWidth(`), so renaming one turns the guard red. Read the rest of the guard file for any other place a class name is listed (a switch, a known-classes list) and add the new class there.

- [ ] **Step 2: Write `TestMapRendersFitEveryWidth` in `internal/usercommands/skill.map_narrow_test.go`**

Call `useDogmudTemplates(t)`. Also render a web `map wide` at 80 (`mapSizeForReader(4, true, 80, 40, 80)`: 66 columns plus the 14-column frame is 80) and assert it fits 80. A telnet `map wide` is sized by the terminal width the client reports (`mapSizeFor` uses `screenWidth`), so it fits that terminal and is not tested here; say so in the class comment. For each width in {30, 42, 60, 79, 80}: build map data the way `Map` does for that width (`DisplayLines` of `mapSizeForReader(1, false, width, 40, width)` columns of `*`, a legend of the five longest legend names in the shipped world, the longest zone name as title), render with `ProcessForWidth("maps/map", data, false, formWidth)` where `formWidth` is chosen as `Map` chooses it, and assert the widest line fits `width` (80 at 80: the framed base is 79). Run it and see it pass; then break it once (render 65 columns of data into the 42 form) to see it fail, and undo that.

- [ ] **Step 3: Run the guards and the package**

Run: `go test ./internal/guards/ ./internal/usercommands/ -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/guards/template_width_guard_test.go internal/usercommands/skill.map_narrow_test.go
git commit -m "test(guards): the map and lockpicking fit every width; the wide exemption goes"
```

---

### Task 5: The game window keeps its text size

**Files:**
- Rewrite: `_datafiles/html/public/static/js/termfit.js`
- Rewrite: `tools/jstest/termfit.test.js`

- [ ] **Step 1: Rewrite the test for the new contract**

Keep the fake page from the current test (`makePage`, `fill`: real xterm buffer, cell `0.6em` by `1.2em`, `proposeDimensions` from the host size) and replace `setScale`/`minFont` with the new options. The checks:

```js
// A phone: the font stays the token size and the columns follow the panel.
var phone = makePage(360, 500, { text: 14, minFont: 1 });
phone.fitter.fit();
check('a phone keeps the 14px text', phone.fontSize === 14, 'font ' + phone.fontSize);
check('a phone gets the columns that fit', phone.real.cols === Math.floor(360 / (14 * 0.6)), 'cols ' + phone.real.cols);

// A desktop: 16px, never more than 80 columns.
var wide = makePage(1600, 900, { text: 16, minFont: 1 });
wide.fitter.fit();
check('a wide panel keeps the 16px text', wide.fontSize === 16, 'font ' + wide.fontSize);
check('a wide panel stops at 80 columns', wide.real.cols === 80, 'cols ' + wide.real.cols);

// A raised browser minimum wins over the token.
var raised = makePage(360, 500, { text: 14, minFont: 20 });
raised.fitter.fit();
check('a raised minimum is used', raised.fontSize === 20, 'font ' + raised.fontSize);
check('nothing is scaled', !raised.host.style.transform, 'transform ' + raised.host.style.transform);

// The keyboard: a height-only change keeps the columns and reports nothing.
var reported = [];
var kb = makePage(360, 500, { text: 14, minFont: 1, onCols: function (c) { reported.push(c); } });
kb.fitter.fit();
var cols = kb.real.cols, n = reported.length;
kb.panel.clientHeight = 300; kb.fitter.fit();
check('the keyboard keeps the columns', kb.real.cols === cols, 'cols ' + kb.real.cols);
check('the keyboard reports nothing', reported.length === n, 'reports ' + reported.join(','));

// Turning upright: the token changes to 14 and the panel narrows; one report.
// Narrowing keeps the newest lines (decision 6).
var turn = makePage(800, 400, { text: 16, minFont: 1, onCols: function (c) { reported.push(c); } });
fill(turn, function () {
    turn.fitter.fit();
    var last = turn.real.buffer.active.getLine(turn.real.buffer.active.length - 2).translateToString(true);
    turn.text = 14; turn.panel.clientWidth = 360; turn.fitter.fit();
    var tail = '';
    for (var i = turn.real.buffer.active.length - 6; i < turn.real.buffer.active.length; i++) {
        tail += turn.real.buffer.active.getLine(i).translateToString(true);
    }
    check('narrowing keeps the newest line', tail.indexOf(last.slice(0, 10)) >= 0, 'tail ' + tail);
});

// A same-size refit does no work; a forced one (font load) measures again.
```

`makePage(W, H, opts)` now takes `opts.text` (the token, readable and changeable as `page.text`), `opts.minFont`, `opts.onCols`, and passes `fontSize: function () { return page.text; }` to `TermFit.create`.

Every check in today's file (`tools/jstest/termfit.test.js:98-215`) is carried over, translated to the new options, not dropped. Each guards a fixed bug: the tiny frame that crashed the tab, never resizing to a trial width (now: each fit calls `resize` at most once; assert `resizes` rises by exactly 1), an unmeasured fit not being remembered, a forced refit while the panel is hidden, the display-scale change at the same panel size, and the very short panel. The old forced-refit checks used the font as a marker; use the `measure` call count instead.

`fill` writes asynchronously, so the narrowing check runs in its callback. Put `process.exit(failures ? 1 : 0)` inside the last callback, as today's file does, never at the end of the file, or the check is skipped and the run is green. In the narrowing case also assert `turn.real.cols === Math.floor(360 / (14 * 0.6))` and that exactly one more column count was reported, so the check fails against the old termfit.

Run: `node tools/jstest/termfit.test.js`
Expected: FAIL (the old termfit, given the new options, throws on its missing `setScale` inside its own try/catch and fits nothing).

- [ ] **Step 2: Rewrite `termfit.js`**

```js
// termfit.js: fit the game window to its panel.
//
// The game text is one size: the --text token (16px, 14px on an upright
// phone), never below the smallest size the browser renders, and nothing
// here ever changes it to make text fit (owner ruling 2026-11-12). The
// layout gives the game window the width of 80 columns (widthFor80) when
// the browser has room, so the terminal has 80 columns; only when the
// browser is narrower than that does the panel, and so the column count,
// follow it. The server wraps to the column count (displayreport.js). A
// height-only change, such as the on-screen keyboard, changes only the rows.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.TermFit = api;        // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var MAX_COLS = 80;

    // opts: term, fitAddon, host (the element xterm opened in), minFont (the
    // smallest size the browser renders), fontSize() (the --text token in px),
    // onCols(cols) (told when the column count changes), onGameWidth(px)
    // (told when the width 80 columns need changes: a font load, zoom,
    // another monitor, an upright phone).
    function create(opts) {
        var term = opts.term, fitAddon = opts.fitAddon, host = opts.host;
        var lastW = 0, lastH = 0, lastDpr = 0, lastFont = 0, lastW80 = 0, forcePending = false;

        function fit(force) {
            if (force) forcePending = true;
            var body = host && host.parentElement;
            // While the page loads the frame can be a few pixels wide; wait
            // for a real size (a tiny frame once made a huge terminal).
            // A panel already fitted at this width may go much shorter: a
            // phone on its side with the keyboard up (today's termfit.js:55).
            if (!body || body.clientWidth < 160 || body.clientHeight < 1 ||
                (body.clientHeight < 80 && body.clientWidth !== lastW)) return;
            var font = Math.max(opts.fontSize(), opts.minFont);
            if (!forcePending && body.clientWidth === lastW && body.clientHeight === lastH &&
                window.devicePixelRatio === lastDpr && font === lastFont) return;
            try {
                if (font !== lastFont) term.setOption('fontSize', font);
                // Measure the letters again unless only the height changed
                // (the keyboard). A font load and a display-scale change (zoom,
                // another monitor) change the cell size at the same panel size,
                // and xterm's own update can arrive after this fit; a stale cell
                // gave too many columns, or rows hidden under the panel after a
                // font load (2026-11-07). Private xterm 4.19 internals.
                var heightOnly = !forcePending && body.clientWidth === lastW &&
                    window.devicePixelRatio === lastDpr && font === lastFont;
                if (!heightOnly && term._core && term._core._charSizeService) {
                    term._core._charSizeService.measure();
                }
                // A zoom or another monitor changes the display scale. The
                // canvas renderer works its cell size out from the scale and
                // updates it only from its own watcher, which can run after
                // this fit; update it now so the columns are counted against
                // the new cell. Private xterm 4.19 internals.
                if (window.devicePixelRatio !== lastDpr && term._core && term._core._renderService &&
                    term._core._renderService.onDevicePixelRatioChange) {
                    term._core._renderService.onDevicePixelRatioChange();
                }
                var d = fitAddon.proposeDimensions();
                if (!d || isNaN(d.cols) || isNaN(d.rows)) return;
                var cols = Math.min(MAX_COLS, d.cols);
                var colsChanged = cols !== term.cols;
                if (colsChanged || d.rows !== term.rows) {
                    term._core._renderService.clear();
                    term.resize(cols, d.rows);
                }
                lastW = body.clientWidth; lastH = body.clientHeight;
                lastDpr = window.devicePixelRatio; lastFont = font; forcePending = false;
                if (colsChanged && opts.onCols) opts.onCols(cols);
                var w80 = widthFor80();
                if (w80 && w80 !== lastW80) {
                    lastW80 = w80;
                    if (opts.onGameWidth) opts.onGameWidth(w80);   // the page keeps the last one (Task 8)
                }
            } catch (e) {}
        }

        // The pixel width the terminal needs for exactly 80 columns at the
        // current font: 80 real cells plus xterm's scrollbar. The layout gives
        // the game window this width (plus its own border) whenever the
        // browser has room, so the terminal there always has 80 columns.
        // Null until xterm has measured its cells. Private xterm 4.19 fields,
        // the same ones xterm-addon-fit reads.
        // Worked from the letter width and the display scale, the way the
        // canvas renderer sizes its cells (scaled cell = floor(letter * dpr)),
        // not from dimensions.actualCellWidth, which is canvasWidth / cols and
        // drifts by a pixel when the terminal has fewer than 80 columns.
        function widthFor80() {
            var core = term._core;
            var cs = core && core._charSizeService;
            if (!cs || !cs.width) return null;
            var dpr = window.devicePixelRatio || 1;
            var sb = core.viewport ? core.viewport.scrollBarWidth : 0;
            return Math.ceil(Math.floor(cs.width * dpr) * MAX_COLS / dpr + sb);
        }

        return { fit: fit, cols: function () { return term.cols; }, widthFor80: widthFor80 };
    }

    return { create: create };
}));
```

Add a check that `widthFor80()` is `ceil(floor(letter * dpr) * 80 / dpr + scrollbar)` in the fake page (give the fake `_core` a `_charSizeService.width` that follows the font size, a `viewport.scrollBarWidth` of 15, and set `window.devicePixelRatio` to 1, 1.25 and 2 in turn), that it is the same when the terminal has 42 columns as when it has 80, that a display-scale change calls the fake `_renderService.onDevicePixelRatioChange`, and that `onGameWidth` fires on the first fit and again only when the width changes. Keep today's very-short-panel check (`termfit.test.js`, the panel under 80px tall at a new width is not fitted).

`fitAddon.fit()` is not used: it resizes to the full panel width, which is over 80 on a desktop. In the test's fake page, give `term._core` a `_charSizeService.measure` that counts calls, and check a keyboard (height-only) fit does not call it and a forced fit does.

In `WP`, also refit on `orientationchange` (`window.addEventListener('orientationchange', function () { resizeTerminal(true); })`), so an upright turn picks up the 14px token even on a browser that fires the event after `resize`.

- [ ] **Step 3: Run the test**

Run: `node tools/jstest/termfit.test.js`
Expected: every line `ok`.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/html/public/static/js/termfit.js tools/jstest/termfit.test.js
git commit -m "feat(webclient): game text keeps its size; columns follow the window, at most 80"
```

---

### Task 6: The client reports its width

**Files:**
- Create: `_datafiles/html/public/static/js/displayreport.js`
- Create: `tools/jstest/displayreport.test.js`

- [ ] **Step 1: Write the failing test**

```js
// displayreport.test.js: the web client tells the server its column count
// once it settles (spec section 1: 250ms, only on a change, again on connect).
//
// Run:  node tools/jstest/displayreport.test.js
'use strict';

var path = require('path');
var DisplayReport = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'displayreport.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

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

var c = clock(), sent = [], open = true;
var r = DisplayReport.create({
    send: function (cols) { if (!open) return false; sent.push(cols); return true; },
    setTimeout: c.setTimeout, clearTimeout: c.clearTimeout
});

r.connected(80);
check('connect sends the width at once', sent.join() === '80', sent.join());

r.columns(60); c.advance(100); r.columns(50); c.advance(100); r.columns(42);
c.advance(249);
check('nothing is sent while the size is still moving', sent.join() === '80', sent.join());
c.advance(1);
check('the settled width is sent once', sent.join() === '80,42', sent.join());

r.columns(42); c.advance(300);
check('the same width is not sent again', sent.join() === '80,42', sent.join());

open = false; r.columns(36); c.advance(300);
check('a closed socket sends nothing', sent.join() === '80,42', sent.join());
open = true; r.connected(36);
check('a reconnect sends the width though it was sent before', sent.join() === '80,42,36', sent.join());

process.exit(failures ? 1 : 0);
```

Run: `node tools/jstest/displayreport.test.js`
Expected: FAIL, cannot find module.

- [ ] **Step 2: Write `displayreport.js`**

```js
// displayreport.js: tell the server how many columns the game window has.
//
// The server wraps every line to this (Client.Display, handled in
// modules/gmcp/gmcp.ClientDisplay.go, which clamps it to 30 to 80). A width
// is sent once it has held for 250ms, so a window being dragged does not
// send every step; the same width is not sent twice; and a new connection
// gets the width at once, because the server starts every connection at 80.

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

    // opts: send(cols) (returns false when it could not send), and
    // setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var lastSent = null, pending = null, timer = null;

        function flush() {
            timer = null;
            if (pending === null || pending === lastSent) return;
            if (opts.send(pending) !== false) lastSent = pending;
        }

        return {
            // The column count changed; send it once it settles.
            columns: function (cols) {
                pending = cols;
                if (timer) clearT(timer);
                timer = setT(flush, SETTLE_MS);
            },
            // A new connection: send the current width now.
            connected: function (cols) {
                if (timer) { clearT(timer); timer = null; }
                lastSent = null;
                pending = cols;
                flush();
            }
        };
    }

    return { create: create };
}));
```

- [ ] **Step 3: Run the test**

Run: `node tools/jstest/displayreport.test.js`
Expected: every line `ok`.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/html/public/static/js/displayreport.js tools/jstest/displayreport.test.js
git commit -m "feat(webclient): report the game window's columns to the server once they settle"
```

---

### Task 7: The layout ladder

**Files:**
- Create: `_datafiles/html/public/static/js/layoutpick.js`, `tools/jstest/layoutpick.test.js`
- Modify: `_datafiles/html/public/static/js/dashboard.js`, `_datafiles/html/public/static/css/dashboard.css`, `WP:280-392` (the panel markup)
- Read first: `_archive/notes/reports/full-client-mock/static/css/mock-overrides.css:159-320` and its `index.html` (in the MAIN checkout, `C:/Users/Calabe Davis/workspace/DOGMud`; it is gitignored, so it is not in the worktree). Diff the mock's `#dashboard` markup against `WP:276-395` to see the `#dash-panels` wrapper it adds.

- [ ] **Step 1: Write the failing test for the choice**

```js
// layoutpick.test.js: which layout the dashboard uses (owner ladder 2026-11-12).
//
// Run:  node tools/jstest/layoutpick.test.js
'use strict';

var path = require('path');
var LayoutPick = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'layoutpick.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + got + ', want ' + want);
}

// The game column is 785px (80 columns, border and slack); the room right of
// it is W - 2*PAD - 785 - GAP. PAD and GAP are read from dashboard.css, as
// viewportfit.test.js:110 reads its CSS, so the test fails if the stylesheet
// and layoutpick.js drift apart.
var fs = require('fs');
var css = fs.readFileSync(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'css', 'dashboard.css'), 'utf8');
var block = css.slice(css.indexOf('#dashboard {'), css.indexOf('}', css.indexOf('#dashboard {')));
var cssPad = parseFloat(/padding:\s*([\d.]+)px/.exec(block)[1]);
// The gap between the game and the panels is an explicit grid track in the
// wide grid (the 6px between the game column and the panels; the column gap
// itself is 0). Read it from the wide-mode grid-template-columns you write in
// step 4; adjust this pattern to match that rule exactly.
var wideStart = css.indexOf('#dashboard[data-mode="wide"] {');
var wide = css.slice(wideStart, css.indexOf('}', wideStart));
var cssGap = parseFloat(/var\(--game-w[^)]*\)\s*\)?\s*([\d.]+)px/.exec(wide)[1]);
check('layoutpick uses the stylesheet padding', LayoutPick.PAD, cssPad);
check('layoutpick uses the stylesheet gap', LayoutPick.GAP, cssGap);
var G = 785;
check('a monitor gets the full layout', LayoutPick.pick(1920, G, false), 'wide');
check('360px of room is the full layout', LayoutPick.pick(12 + G + 6 + 360, G, false), 'wide');
check('359px of room is one column', LayoutPick.pick(12 + G + 6 + 359, G, false), 'column');
check('200px of room is one column', LayoutPick.pick(12 + G + 6 + 200, G, false), 'column');
check('199px of room is the slim column', LayoutPick.pick(12 + G + 6 + 199, G, false), 'slim');
check('120px of room is the slim column', LayoutPick.pick(12 + G + 6 + 120, G, false), 'slim');
check('119px of room is the phone layout', LayoutPick.pick(12 + G + 6 + 119, G, false), 'phone');
check('narrower than the game is the phone layout', LayoutPick.pick(640, G, false), 'phone');
check('a phone is always the phone layout', LayoutPick.pick(1920, G, true), 'phone');
check('no game width yet keeps the phone layout', LayoutPick.pick(1920, null, false), 'phone');

process.exit(failures ? 1 : 0);
```

Run: `node tools/jstest/layoutpick.test.js`
Expected: FAIL, cannot find module.

- [ ] **Step 2: Write `layoutpick.js`**

```js
// layoutpick.js: which layout the dashboard uses (owner ruling 2026-11-12).
//
// The game window is on the left, exactly as wide as 80 columns of the
// game text (termfit.js widthFor80, plus its border). The panels take the
// room right of it, and only they change as the browser narrows:
//   wide    360px or more: Layout D, every panel open (spec ruling 9)
//   column  200 to 359:    Map, Health and Conditions; the other four on buttons
//   slim    120 to 199:    Health over Conditions; the rest on the tab bar
//   phone   under 120, a browser narrower than the game, or a phone: every
//           panel on the tab bar under the game
// The game window and its text never change between wide, column and slim.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;       // node, for the tests
    } else {
        root.LayoutPick = api;      // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var PAD = 6, GAP = 6;                 // #dashboard padding and grid gap (dashboard.css)
    var WIDE = 360, COLUMN = 200, SLIM = 120;

    // dashW: #dashboard's width in CSS px. gameW: the game window's width at
    // 80 columns, or null before it is measured. phone: the screen's short
    // side is under 600px (viewportfit.isPhone).
    function pick(dashW, gameW, phone) {
        if (phone || !gameW) return 'phone';
        var room = dashW - 2 * PAD - gameW - GAP;
        if (room >= WIDE) return 'wide';
        if (room >= COLUMN) return 'column';
        if (room >= SLIM) return 'slim';
        return 'phone';
    }

    return { pick: pick, PAD: PAD, GAP: GAP };
}));
```

`gameW` here is the whole game column: `widthFor80` plus the feed panel's border (2px; check the real border) plus 4px of slack. The slack covers a page zoomed below 100%, where a 1px border snaps to one device pixel and the fit addon's `parseInt` loses a pixel, which gave 79 columns in simulation; the `min(80, ...)` cap in termfit makes the extra harmless. `Dashboard.setGameWidth(px)` adds both and keeps the sum in one place.

Run: `node tools/jstest/layoutpick.test.js`
Expected: every line `ok`.

- [ ] **Step 3: The markup.** In `WP`, `#dash-col-center` sits between the left and right columns (`WP:330-406`). Move it first, then wrap `#dash-col-left` and `#dash-col-right` in `<div id="dash-panels">`, as the mock's `index.html` does, so one grid area holds every side panel. This changes the keyboard tab order: the game and command box now come before the panels, which suits a game played from the command box; say so in the PR. Keep every panel id, `data-panel` and its order inside its column: `dashboard.js` `init` records `homeCol` from the column, and pop-out, redock and reset rely on it.

- [ ] **Step 4: The CSS.** In `dashboard.css`. Port these mock rules and no others; the rest of `mock-overrides.css` section 4 belongs to plan 7 or to things the real client does not have (`#map-where`, `#quests-repeat`, and `#map-render > svg > g:first-of-type text { display: none }`, which would hide the map's own words; leave all three out).
  - `wide`: the grid at `mock-overrides.css:161-174` with the game column exactly `var(--game-w)` (not `minmax(0, ...)`, and no `--mock-panels-min`: the ladder decides); `#dash-panels { grid-area: panels; }` (`:175`) scoped to the modes that have that area; the panel grid `:176-195`; the square Map `:196-199`; the area assignments `:229-241`.
  - `phone`: `#dashboard[data-mode="phone"] #dash-panels { display: contents; }` (`:263`), so the phone layout is today's.
  - `column`: the tablet grid `:268-312`, with the game column `var(--game-w)` and Map, Health and Conditions stacked in one column (decision 10); the four-button bar and its drawer over the foot of that column (`:313-320`). The mock sets their width from `--mock-panels-w` in its script (`index.html:673`); `dashboard.js` sets the same variable (call it `--panels-w`) to the panel area's width on each `_apply` and resize. The `html.mock-tablet` class becomes `html.layout-column`.
  - `slim`: the game column `var(--game-w)`, then one column of Health over Conditions: `#dash-col-left` and `#dash-col-right` are `display: contents` inside `#dash-panels` (as in the tablet rules), and Map, Quests, Inventory, Comms and Automation are hidden there (they open from the tab bar). The tab bar sits under the game column only (left 6px, width `var(--game-w)`), and the game column reserves its height (`padding-bottom` as `[data-mode="phone"]` does at `:607-655`), so the bar never covers the command box (#730). Extend the keyboard rules (`html.keyboard-open`, `:639-655`) and the name-pill strip rule to `slim` and `column` as they apply to `phone`, as separate rules: `tools/jstest/viewportfit.test.js:110` matches the phone rule `html.keyboard-open #dashboard[data-mode="phone"] { padding-bottom: 6px;` exactly, so leave that rule's text as it is (or update that test in the same commit and add it to the commit list).
  - The tab bar shows a subset by mode, as the mock does (`:313-316`): in `column` hide the Map, Health and Conditions buttons; in `slim` hide Health and Conditions. One bar, built once with all seven.
  - Hide the pop-out button outside `wide` (`dashboard.css:821-823` lists rail and phone; make it every mode but `wide`).
  - Delete: every `[data-mode="rail"]` rule, `#dash-rail`, `.gtabs-rail`, `.rail-body` (`grep -n 'rail' dashboard.css`); `.dash-splitter`; the drag-to-rearrange rules (`cursor: grab`, `[draggable]`, `.drag-over`; `grep -n 'drag' dashboard.css`, keeping anything WinBox pop-out still uses).
  - Every size stays a token or a measured variable (the type guard, `internal/guards/client_type_guard_test.go`, fails on a stray `font-size`).

- [ ] **Step 5: `dashboard.js`.**
  - `setGameWidth(px)` stores `this.gameW = px + FEED_BORDER + SLACK` (2 and 4; Step 2), sets `--game-w` to that on `document.documentElement` (not `#dashboard`: the tab bar and drawer are appended to `#dashboard`'s parent, `dashboard.js:459`, `:476`, `:504`, and would not inherit it; the mock sets its variable on `<html>` too), and calls `_apply()`. In `init` it replaces the first `_apply()` call (`:160`), after `_buildRail`'s replacement and `_buildPhone` have run, so the tab bar exists before any mode is entered; then `init` sets `this.ready = true` (Task 8 step 4).
  - `--panels-w` (the panel area's width, used by the column step's tab bar and drawer) is also set on `document.documentElement`, at the top of `_apply` before its early return (`:251`), because it changes on every resize within a mode.
  - Until the first `_apply`, `#dashboard` has no `data-mode`. Give `dashboard.css` a default for that state that is the `wide` grid with the game column at `var(--game-w, 786px)`, so a desktop's first paint is never the phone layout even if the first fit returned early.
  - `_computeMode` returns `LayoutPick.pick(dashboard.clientWidth, this.gameW, phone)`, where `phone` is `ViewportFit.isPhone(screen.width, screen.height) && matchMedia('(pointer: coarse)').matches`. The touch test is needed here: a desktop zoomed to 125% on a 1280 by 720 screen reports a short side under 600 and would otherwise be forced into the phone layout.
  - Modes are `wide`, `column`, `slim`, `phone`. `_enterWide` puts every side panel in `#dash-panels` in the Layout D areas. `_enterColumn` shows Map, Health and Conditions; Comms, Quests, Inventory and Automation open from the tab bar in the drawer over the column's foot. `_enterSlim` shows Health and Conditions; the other five open from the tab bar in the drawer. `_enterPhone` is today's. The one phone tab bar and drawer serve all three folded modes; CSS hides the buttons a mode keeps open (Step 4).
  - Every path that knows the modes: `_apply` tears down the previous mode for every mode (a `_exitColumn`/`_exitSlim` returns a panel in the drawer to its home and closes the drawer, as `_exitPhone` does); `resetLayout` (`:204-209`) re-enters the current mode, whatever it is, instead of sending every mode but rail and phone to `_enterWide`; `_relayoutTabs` (`:221-224`) relays the bar in `column`, `slim` and `phone`; `_dockPoppedForNarrow` and `_repopAfterNarrow` treat every mode but `wide` as narrow (they already compare to `"wide"`; check).
  - Delete the rail: `_buildRail`, `_enterRail`, `_setRailActive`, `_exitRail`, `_railTabs`, `_railBody`, every `"rail"` branch. Delete `initSplitters` and its call in `init`.
  - `initRearrange` holds both the drag-to-swap code and the fold button's wiring (`:983`, the fold handler at `:1073-1083`, pinned by `tools/jstest/dashboard-popout-layout.test.js:341-351`). Move the fold wiring into a new `initCollapse`, call it from `init`, then delete the rest of `initRearrange`. Folding must keep working in every mode; the jstest pin stays and passes.
  - Pop-out: only in `wide` (`_apply` already docks when leaving `wide` and re-pops on return; keep that, with the new mode names).
  - Saved layout: key `dogmud.dashboard.layout.v2` everywhere (`resetLayout`, `saveLayout`, `restoreLayout`); it keeps folded panels, popped-out panels and their window sizes and places (`boxes`, `:693`, `:807-813`; that is the #768 item 14 fix), and drops the column arrangement and column widths. On load, if there is a `v1` entry and no `v2`, carry its folded, popped and `boxes` fields over once, then remove `v1`; all in try/catch.
  - Update the file's header comment (`:1-6`) to the four modes.
  - `html.layout-column` and an `html.layout-slim` class are toggled in `_apply` for the CSS that hides tab-bar buttons and places the bar (Step 4).
  - `tools/jstest/dashboard-popout-layout.test.js` loads `dashboard.js` in a vm sandbox with no `LayoutPick`, `ViewportFit`, `screen` or `matchMedia` (`:205-217`) and drives modes with `resize(900)`/`resize(1400)` against the old 1100/700 thresholds, stubbing only rail and phone (`:236-237`). Load `layoutpick.js` and `viewportfit.js` into the sandbox, give it `screen` and a `matchMedia` stub, set the game width with `D.setGameWidth`, drive the modes with widths the ladder maps to `wide` and `phone`, stub the new enter and exit functions, update to `v2` and its fields, and replace the `arrange` assertions (the arrangement is gone) with checks on where `redock` puts a panel in the DOM. Keep every other check, including the fold pin at `:341-351` and the window-place check.

- [ ] **Step 5a: The 14px size is for phones only.** `viewportfit.js` marks `upright-phone` (the 14px token) from the screen alone (`:25-28`), so a zoomed portrait desktop monitor gets 14px text while the ladder treats it as a desktop. Gate `isUprightPhone` with the same touch test as decision 8 (`matchMedia('(pointer: coarse)')`, passed in so the node test can set it), and add a case to `tools/jstest/viewportfit.test.js`: a portrait screen under 600 wide without touch is not an upright phone.

- [ ] **Step 6: Run the client tests**

Run: `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED $t"; done`
Expected: no `FAILED` line.

Run: `go test ./internal/guards/ -count=1`
Expected: PASS (the client type and colour guards read `dashboard.css` and `WP`).

- [ ] **Step 7: Commit**

```bash
git add _datafiles/html/public/static/js/layoutpick.js tools/jstest/layoutpick.test.js _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/webclient-pure.html tools/jstest/dashboard-popout-layout.test.js _datafiles/html/public/static/js/viewportfit.js tools/jstest/viewportfit.test.js
git commit -m "feat(webclient): the game on the left at 80 columns, panels on a ladder around it"
```

---

### Task 8: Wire it into the web client

**Files:**
- Modify: `WP:215-220` (script tags), `:433-447` (Terminal options), `:464-485` (delete the monkeypatch), `:518-532` (fitter), `:2893` (`socket.onopen`)

- [ ] **Step 1: Load the new scripts** beside `termfit.js`, and `layoutpick.js` before `dashboard.js` (find where `dashboard.js` is loaded: `grep -n "dashboard.js" WP`):

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/displayreport.js?v={{ .ASSET_VERSION }}"></script>
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/layoutpick.js?v={{ .ASSET_VERSION }}"></script>
```

- [ ] **Step 2: Build the terminal at the token size.** Replace `cols: 80,` with `cols: 80,  // resized to what fits by termfit.js` only if xterm needs a starting value (it does), and replace `fontSize: 20,` with:

```js
            fontSize: textSize(),
```

and define, above `const term`:

```js
        // The game text size: the --text token (gomud.css), read each fit
        // because an upright phone changes it.
        function textSize() {
            var px = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--text'));
            return isNaN(px) ? 16 : px;
        }
```

- [ ] **Step 3: Delete the scale monkeypatch** (`var termScale = 1;` and the IIFE that replaces `strat.measure`, `WP:464-485`, with its comment). Nothing scales the terminal any more.

- [ ] **Step 4: Create the fitter with the new options:**

```js
        // Tell the server the game window's columns (static/js/displayreport.js).
        var displayReport = window.DisplayReport.create({
            send: function (cols) { return SendGMCP('Client.Display', { cols: cols }); }
        });
        // Fit the game window to the panel (static/js/termfit.js).
        var termFitter = window.TermFit.create({
            term: term,
            fitAddon: fitAddon,
            host: document.getElementById('terminal'),
            minFont: browserMinFont,
            fontSize: textSize,
            onCols: function (cols) { displayReport.columns(cols); },
            // The layout gives the game window this width (dashboard.js).
            // Kept on window too: dashboard.js loads after this script
            // (WP:4931) and sets itself up on DOMContentLoaded
            // (dashboard.js:1088-1092), so the first reports arrive before
            // it exists. Dashboard.init reads the last one.
            onGameWidth: function (px) {
                window.dogmudGameWidth = px;
                if (window.Dashboard && window.Dashboard.ready) window.Dashboard.setGameWidth(px);
            }
        });
```

`Dashboard.init` calls `this.setGameWidth(window.dogmudGameWidth || null)` before its first `_apply`, then sets `this.ready = true`. So a desktop gets its real layout on its first paint and never shows the phone layout on load.

The width can change once before play: JetBrains Mono replaces the fallback font when it loads (the fallback measures about 80px narrower over 80 columns), so a window near a step can move one step on the login screen. That is before the player is in the game, and it is the only change the player did not cause. Check it happens before the login prompt finishes drawing (Task 9).

Confirm in Task 9 that the sequence on load ends with 80 columns on a 1920 window and never loops: log each fit and each mode change once while checking.

`SendGMCP` is a function declaration later in the same script, so it is hoisted; check that it is in the same scope (`grep -n "function SendGMCP" WP`, and confirm both sit in the same `<script>` block). If not, pass a wrapper that looks it up at call time.

- [ ] **Step 5: Send on connect.** In `socket.onopen`, after the reconnect token is sent (find `socket.send(t)` or the copyover token send in `onopen`; if there is none, after `setSessionState('connected');`):

```js
                // The server starts every connection at 80 columns.
                displayReport.connected(term.cols);
```

After the token, because a copyover reconnect reads the first messages as the token (`main.go:1428-1429` tries each pre-login message, so either order works today, but this order does not rely on that).

- [ ] **Step 6: Update the font-load comment** at `WP:558` ("The 80-col grid is measured from glyph width") to say the column count is measured from glyph width.

- [ ] **Step 6a: Update the client type guard.** `internal/guards/client_type_guard_test.go:60-61` exempts the monkeypatch's two `el.style.font*` lines, and `:180-183` fails when an exemption matches nothing. Delete those two exemptions, and add the file to this task's commit.

- [ ] **Step 7: Run every client test and the template guard for the page**

Run: `for t in tools/jstest/*.test.js; do node "$t" || echo "FAILED $t"; done`
Expected: no `FAILED` line.

Run: `go test ./internal/web/... ./internal/guards/ -count=1` (the web page is a Go template; a guard may parse it).
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html internal/guards/client_type_guard_test.go
git commit -m "feat(webclient): the game window keeps its text size and reports its columns"
```

---

### Task 9: Check the ladder and the narrow screens in a browser

No code unless a check fails.

- [ ] **Step 1: Boot a local server from the worktree** on private ports (the `dogmud-shipping` boot-check recipe, with `HttpPort: 18090`), wipe instance saves first, and open `http://localhost:18090/webclient` in a browser at these window sizes: 1920 by 1080, 1280 by 800, 1100 by 760, 960 by 720, 860 by 700, 760 by 900, 640 by 720, and, with the browser's device emulation set to a touch phone (a desktop window cannot be an upright phone, and cannot go that narrow), 390 by 844 and 320 by 640. A browser tool is used if this session has one; otherwise this step is the owner's (Task 11).

- [ ] **Step 2: At each size, record** the text size, `term.cols`, the dashboard mode (`#dashboard` `data-mode`), the measured `--game-w`, and whether the server got the width (a stored width logs nothing, so check it in play: after narrowing, `help look` lays out at the new width). Expected: 16px everywhere but the two phones (14px). For every other size, work the expected mode out from the measured `--game-w` with `LayoutPick.pick` (in the console) and check the page shows that mode; 80 columns whenever the window holds the game column; 30 or more columns at 320. Write the measured `--game-w` and the resulting thresholds into the PR body; they replace the estimates in the mock.

- [ ] **Step 2a: Drag a desktop window slowly from 1920 down to 640 and back.** The game window must not move or change size, and its text must not rewrap, until the window is narrower than the game window. Only the panels change. Record any frame where the game window jumped; that is a bug to fix before going on.

- [ ] **Step 2c: Zoom and scale.** On a desktop at 1920, zoom the page to 67%, 90%, 110%, 125% and back to 100%, and if a second monitor with another scale is available, drag the window across. Each time: `term.cols` is 80 whenever the layout is not `phone`, no column is cut off at the right edge, and the scrollbar does not cover the last column (xterm measures its scrollbar width once, in its Viewport constructor; if it overlaps after a zoom, refresh `viewport.scrollBarWidth` in `fit` on a scale change).

- [ ] **Step 2d: The command box.** In `slim`, `column` and `phone`, open each tab and type: the tab bar and the drawer never cover the command box (#730).

- [ ] **Step 2b: The pop-out and reset.** In `wide`, pop the Map out, narrow to `column` (it docks), widen back (it pops out again). Press Reset layout in each mode: the game stays connected and the layout matches the ladder.

- [ ] **Step 3: At 320 by 640, check the columns are at least 30.** Worked from the CSS it is about 34 (no padding on `.xterm` or `#panel-feed .dash-panel-body`; xterm reserves 15px for its scrollbar even where scrollbars overlay). If the browser shows fewer than 30, find what takes the width and trim it at that size (spec design section 2, last paragraph). A raised browser minimum font can still give fewer than 30 (decision 7).

- [ ] **Step 4: Log in and run, at 390 wide:** `look`, `status`, `inventory`, `map`, `help`, `help colors`, `help look`, `motd`. Every line fits the window: nothing wraps at the right edge. The MOTD shows as rules and text (`motdRuled`), and `motdrestyle.js` left it alone.

- [ ] **Step 5: Pin the MOTD case in jstest.** Add to `tools/jstest/motdrestyle.test.js` a check that `MotdRestyle.restyle` returns the ruled form unchanged, and a sibling check that the 80-column box with the same text IS changed (so the first check can fail). Take each input exactly as the browser gets it: `motdRuled` (or `motdBox` at 80) for a two-line message, run through `templates.AnsiParse` (`internal/templates/templates.go:547-558`, called from `internal/hooks/Message_SendMessages.go:34`), with the cursor-column and erase-line prefix that file adds (`:37`) and every `\n` turned into `\r\n` as the connection does on write (`internal/connections/connectiondetails.go:348`); `restyle` splits on `\r\n` (`motdrestyle.js:55`). Write it from a throwaway Go test to the session scratchpad, paste it into the jstest as a literal, and delete the throwaway test.

Run: `node tools/jstest/motdrestyle.test.js`
Expected: every line `ok`.

```bash
git add tools/jstest/motdrestyle.test.js
git commit -m "test(webclient): the MOTD's narrow form passes through the restyle untouched"
```

---

### Task 9a: The login screen fits a phone (added during the build)

Not in the plan as approved. Task 9's headless phone check (390 and 320 wide) found that everything a phone sees before login was drawn at 80 columns: the connect splash went out as the socket opened, before `displayreport.js` could send `Client.Display`, so the logo was chopped up and the notes broke mid-word; the login prompts broke mid-word ("Using th" / "is port as an AI risks a ban."); the goodbye art is 50 columns of pictures. It also found a script error on the site page. Built and code-reviewed commit by commit on 2026-11-12; decisions 16 to 19 above; BAR round on these commits 2026-11-13.

| Commit | What |
|---|---|
| `d48cf6f4d`, review fixes `2bf61c14a` | `WP` opens `/ws?cols=<term.cols>`. `web.Listen` passes the raw `cols` query value to the handler; `HandleWebSocketConnection` (`main.go`) parses it with `users.ParseReportedCols` (whole numbers only), stores it with `users.StoreReportedCols` (clamped 30 to 80, written only on change; `gmcp.ClientDisplay` now calls the same function), and renders `login/connect-splash` with `templates.ProcessForWidth` at that width. New `login/connect-splash.w42.template` and `.w30.template`, words only. The template guard renders the splash at every width. |
| `c91e856e3` | `users.LoginWrapWidth(id)`: a websocket's stored columns clamped 30 to 80, else 80; 80 for telnet. `inputhandlers.LoginTemplateAtWidth` renders each login prompt and the password mask with `ProcessForWidth` then `messaging.WrapPrompt` (`WrapAnsi` keeping a trailing ": " on the last line). Validation errors and login refusals go through the same wrap. Telnet output unchanged byte for byte. The template guard moves the login prompts, the mask and `generic/prompt.yn` from `"raw"` to the fitting class and checks every width 30 to 80 keeps the words and the ": " last. |
| `00e549d9c` | `goodbye.w42.template` and `.w30.template` say "Goodbye."; quit and a duplicate login draw the goodbye for the leaving connection's width; telnet keeps the art. The guard's `"raw"` class is now empty. |
| `05a6656ca` | `webclient.html` no longer throws when it marks the game frame for the phone keyboard before the frame has a document. |

Each commit's tests: `internal/users/wrapwidth_test.go`, `internal/inputhandlers/login_width_test.go`, `internal/messaging/wrap_test.go`, `internal/guards/template_width_guard_test.go`, `tools/jstest/webclient-connect.test.js`.

---

### Task 10: Docs

- [ ] **Step 1:** `internal/templates/context.md`: forms section says no template is exempt from fitting; help forms (`help/name.w42.template`) work as other forms do.
- [ ] **Step 2:** `internal/usercommands/context.md`: `map` draws to a narrow reader's width (`mapSizeForReader`), and `GetLockRender` rows (`lockRenderAt`, `lockStepsPerRow`).
- [ ] **Step 3:** `internal/guards/context.md`: the `"wide"` class and `helpWide` are gone; help form files are checked as forms.
- [ ] **Step 4:** `modules/gmcp/context.md`: `Client.Display` is now sent by the web client (`displayreport.js`) on connect and on a settled column change.
- [ ] **Step 5:** The spec:
  - Under ruling 3, and beside the two other places the spec states the exemption (spec lines 151 and 234: `grep -n "exempt" ` the spec), add `Amended 2026-11-12 (plan 5, owner ruling): the web game window cannot scroll sideways, so maps and wide tables fit the reader too; see docs/superpowers/plans/2026-11-12-webclient-base-5-live-window.md.`
  - Under design section 4, add `Amended 2026-11-12 (plan 5, owner ruling): the game never resizes the game window or its text to make content fit; the game window is 80 columns and changes width only when the player changes the browser.`
  - Under design section 5, add `Amended 2026-11-12 (plan 5, owner ruling): the mode table is replaced by the layout ladder in plan 5 (wide, column, slim, phone, chosen by the room beside an 80-column game window); the touch test is not used.`
  - In the plans table, row 5 gains "the large-screen layout", and row 6 keeps "30% tabs, keyboard rules, X, toast".
- [ ] **Step 5a:** `docs/guides/` or wherever the web client's layout modes are described for players or builders: `grep -rln "rail" docs/ --include=*.md | grep -v superpowers` and update any player-facing description of the layouts.
- [ ] **Step 6:** `docs/PATCH_NOTES.md`, 2026-11-12 (or the day it ships), player framing, no numbers in the prose beyond column counts players never see:

```markdown
### The game window fits your screen

- Game text in the web client is now one readable size on every device, and it never shrinks or grows to fit. On a phone the lines fit your screen instead. (If you have raised your browser's minimum text size, the game uses that size.)
- On a computer the game sits on the left at its full width, with the map and your panels to the right. Narrow the window and the panels tuck behind the round buttons; the game itself stays put.
- The map, lockpicking and the colour chart in help redraw themselves to fit a narrow screen. Lockpicking long locks no longer runs off the edge on a desktop either.
- Zooming the page to 200% now makes the text bigger, as it should.
```

- [ ] **Step 7:** `docs/README.md`: rows for `static/js/displayreport.js`, `static/js/layoutpick.js`, the six new template files, and the new tests (`skill.map_narrow_test.go`, `picklock_render_test.go`, `tools/jstest/displayreport.test.js`, `tools/jstest/layoutpick.test.js`); the README lists test files (`docs/README.md:491`), so follow its pattern.
- [ ] **Step 8:** `internal/mapper/context.md:601` documents `map wide` and `mapSizeFor`: add the narrow map (`mapSizeForReader`).
- [ ] **Step 9: Commit**

```bash
git add internal/templates/context.md internal/usercommands/context.md internal/guards/context.md internal/mapper/context.md modules/gmcp/context.md docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs: plan 5, the game window goes live"
```

---

### Task 11: Gates, owner check, PR

- [ ] **Step 1: Gates** (the `dogmud-shipping` pre-push order): `gofmt -l internal/ modules/` empty; `go build ./...`; `golangci-lint run --new-from-merge-base=origin/master` 0 issues; `go test ./...`; every `tools/jstest/*.test.js`; boot check `Server Ready` 1 and 0 panics; `LogToFile: false`.

- [ ] **Step 2: Owner check.** This slice is visible, so the owner tests it. Start a local server from the worktree with his usual overrides, wipe instance saves first, and give him these ordinary player steps:
  1. Open the web client on a desktop browser, maximised. The game is on the left; the Map, Health, Conditions, Comms, Quests, Inventory and Automation are to its right.
  2. Drag the window slowly narrower. The game window and its text do not move or change. The panels go to one column, then a slim Health and Conditions column, then all onto the round buttons under the game.
  3. Keep dragging until the window is narrower than the game. Only now does the game window narrow; the text stays the same size and new lines fit the window.
  4. At that narrow size: `look`, `status`, `map`, `help colors`, `help look`. Each fits; the map has plain rules instead of its rope frame.
  5. Drag it wide again: everything comes back as it was.
  6. Zoom the page to 200%: the text gets bigger, the game window narrows, and the lines fit.
  7. On your phone, if the local server is reachable from it: the text is readable without zooming.
  8. Telnet at 80: `map` and `help colors` look as they did.

- [ ] **Step 3: Real devices.** Malia tests on her devices before merge (spec, "Real devices"; #834). Post the branch and these steps on #834.

- [ ] **Step 4: Push and open the PR** after #863 merges and this branch is rebased on master:

```bash
git fetch origin && git rebase origin/master
git push -u origin feature/webclient-base-5-live
gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-5-live --title "feat(webclient): the game window goes live (web client base plan 5)" --body-file <scratchpad>/pr5-body.md
```

The body lists the owner ruling, the decisions above, the device results, and the deploy note (no migration, no config; a deploy reloads the client, so players see the change on their next page load). No closing keywords for #783. It ends with the Claude Code attribution line.
