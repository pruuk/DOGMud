# Web client base plan 7c: a bigger Map in the laptop column Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In the column layout, put Conditions beside a larger square Map with Health in a strip below, swap Health and Conditions when Health grows past a third of the column, and keep today's stacked column where it gives the larger Map.

**Architecture:** A pure function in `layoutpick.js` picks the arrangement and the Map's size from three measured numbers. `dashboard.js` measures, calls it, and sets `data-column` and `--col-map` on `#dashboard`; CSS rules keyed on `data-column` lay the grid out. The ladder's modes are unchanged.

**Tech Stack:** Plain browser JavaScript (UMD module), CSS grid, node jstests in `tools/jstest/`.

**Spec:** `docs/superpowers/specs/2026-11-15-webclient-base-7c-laptop-column-design.md` (approved 2026-11-15).
**Branch and worktree:** `feature/webclient-base-7-panels` in `C:/tmp/dogmud-p7` (PR #870). All paths are relative to that worktree. `P` = `_datafiles/html/public`.

**Rules for every worker:** edit files with the Edit and Write tools only (no `sed -i`, no Python or shell writes into tracked files). `git add` named paths only. Never `git stash`. No em or en dashes in comments or copy. Do not start, stop or touch any server or process. Do not push.

## Facts verified against source (2026-11-15, `7671d566d`)

| Fact | Where |
|---|---|
| `layoutpick.js` declares `PAD`, `GAP`, `COLUMN`, `SLIM`, `MAP_FLOOR`, `MAP_SHARE`, `MAP_H_TAKEN`, `MAP_H_SHARE`, `wideMapSize`, `GAME_W_DEFAULT`, `pick`, and returns them in one object | `P/static/js/layoutpick.js:24-63` |
| Column's panel grid: `gap: var(--dash-gap, 6px)`, `padding-bottom: calc(6px + var(--tabbar-h))`, `overflow-y: auto`, rows `auto auto minmax(96px, 1fr)`, areas `map / vitals / status` | `P/static/css/dashboard.css:507-522` |
| Column's Map: `width: min(100%, max(120px, calc(100vh - 300px - var(--tabbar-h))))`, `justify-self: center`, `aspect-ratio: 1 / 1` | `P/static/css/dashboard.css:542-548` |
| `#dashboard[data-mode="column"] #panel-vitals { grid-area: vitals; min-height: 120px; }` and `... #panel-status { grid-area: status; min-height: 96px; }` | `P/static/css/dashboard.css:551-552` |
| `_apply` measures, returns early when the mode is unchanged (`if (newMode === this.mode) return; // guard: no-op when mode unchanged`), and ends with `if (newMode === "wide") this._repopAfterNarrow();` | `P/static/js/dashboard.js:344-370` |
| `init` adds the window `resize` listener after `this.ready = true;` | `P/static/js/dashboard.js:182-190` |
| Health: `#panel-vitals` holds `.dash-panel-head` and `.dash-panel-body` > `#vitals-bars` (rows 26px, gap 2px, padding 2px) | `P/webclient-pure.html:370-374`, `:3809-3822` |
| Conditions: `#status-conditions` is `display: flex; flex-direction: column; gap: 6px; padding: 6px` with `.status-line` children | `P/webclient-pure.html:3974-3984` |
| `tools/jstest/layoutpick.test.js` reads `dashboard.css` into `css`; `tools/jstest/dashboard-popout-layout.test.js` reads `dashboard.js` into `SRC` and has a `check(name, got, want)` | both files |

Arithmetic the tests rely on (`stacked = min(w, max(120, h - 12 - 216))`; `beside = min(w - 166, h - 6 - under)`, `under` = Health's height for side or 96 for swapped; swapped when `vitalsH > h / 3`):
- 1440 laptop, `w 681, h 568, vitalsH 92`: stacked 340; side `min(515, 470)` = 470. Side, 470.
- 1280 laptop, `w 521, h 568, vitalsH 92`: stacked 340; side `min(355, 470)` = 355. Side, 355.
- Narrow, `w 250, h 568, vitalsH 92`: stacked 250; side 84. Stacked, 250.
- `w 681, h 600`: `vitalsH 200` is not over 200, side `min(515, 394)` = 394; `vitalsH 201` swaps, `min(515, 498)` = 498.
- Short column, `w 681, h 400, vitalsH 300`: swapped `min(515, 298)` = 298 beats stacked `min(681, max(120, 172))` = 172.
- Very tall column, `w 700, h 2000, vitalsH 92`: side 534, stacked 700. Stacked.

---

### Task 1: `columnArrangement` (Sonnet)

**Files:**
- Modify: `P/static/js/layoutpick.js`
- Test: `tools/jstest/layoutpick.test.js`

- [ ] **Step 1: Write the failing tests.** In `tools/jstest/layoutpick.test.js`, insert this block directly above the last line `process.exit(failures ? 1 : 0);`:

```js
// The column layout's arrangements (plan 7c, owner 2026-11-15): Conditions
// beside a larger Map with Health below (side), Health beside it with
// Conditions below once Health passes a third of the column (swapped), or
// today's stack where that gives the larger Map.
function arr(w, h, v) { var r = LayoutPick.columnArrangement(w, h, v); return r.arrangement + ' ' + r.map; }
check('a 1440 laptop puts Conditions beside the Map', arr(681, 568, 92), 'side 470');
check('a 1280 laptop does too, width bound', arr(521, 568, 92), 'side 355');
check('a narrow column stays stacked', arr(250, 568, 92), 'stacked 250');
check('Health at a third stays the strip', arr(681, 600, 200), 'side 394');
check('Health past a third swaps with Conditions', arr(681, 600, 201), 'swapped 498');
check('a tall Health in a short column swaps', arr(681, 400, 300), 'swapped 298');
check('a very tall column keeps the full-width stacked Map', arr(700, 2000, 92), 'stacked 700');
check('columnMap gives the kept arrangement its size', LayoutPick.columnMap('side', 681, 400, 92), 302);
check('columnMap for stacked is the stylesheet formula', LayoutPick.columnMap('stacked', 681, 300, 92), 120);

// The floors and gap columnArrangement assumes are the stylesheet's.
var colVitals = /#dashboard\[data-mode="column"\] #panel-vitals \{ grid-area: vitals; min-height: ([\d.]+)px; \}/.exec(css);
var colStatus = /#dashboard\[data-mode="column"\] #panel-status \{ grid-area: status; min-height: ([\d.]+)px; \}/.exec(css);
var colMapMin = /width: min\(100%, max\(([\d.]+)px, calc\(100vh - 300px - var\(--tabbar-h\)\)\)\);/.exec(css);
var colStrip = /grid-template-rows: var\(--col-map\) ([\d.]+)px;/.exec(css);
check('the Health floor matches the stylesheet', colVitals && parseFloat(colVitals[1]), LayoutPick.VITALS_FLOOR);
check('the Conditions floor matches the stylesheet', colStatus && parseFloat(colStatus[1]), LayoutPick.STATUS_FLOOR);
check('the stacked Map minimum matches the stylesheet', colMapMin && parseFloat(colMapMin[1]), LayoutPick.STACKED_MAP_MIN);
check('the swapped Conditions strip is the Conditions floor', colStrip && parseFloat(colStrip[1]), LayoutPick.STATUS_FLOOR);
check('the column gap matches the stylesheet', /gap: var\(--dash-gap, 6px\)/.test(css) && LayoutPick.COL_GAP, 6);
```

- [ ] **Step 2: Run it and see it fail.** Run `node tools/jstest/layoutpick.test.js`. Expected: a TypeError (`LayoutPick.columnArrangement is not a function`), non-zero exit. (The `colStrip` check also fails until Task 2 adds the CSS; that is expected after Task 1 too. Task 1's run must show every other new check passing.)

- [ ] **Step 3: Implement.** In `P/static/js/layoutpick.js`, directly after the `wideMapSize` function's closing `}`, add:

```js

    // The column layout's arrangements (plan 7c, owner 2026-11-15). Column
    // keeps Map, Health and Conditions open; it lays them out one of three
    // ways, whichever gives the larger Map:
    //   stacked : Map, Health, Conditions one above the other (dashboard.css)
    //   side    : Conditions beside the Map, Health in a strip below
    //   swapped : Health beside the Map, Conditions in a strip below, once
    //             Health is taller than a third of the column
    // The floors, the gap and the stacked Map's minimum are dashboard.css's
    // (a jstest holds them together).
    var COL_GAP = 6, VITALS_FLOOR = 120, STATUS_FLOOR = 96, STACKED_MAP_MIN = 120;
    // The panel beside the Map is at least this wide.
    var SIDE_MIN = 160;
    // Health beside the Map once it is taller than this share of the column.
    var SWAP_SHARE = 1 / 3;

    // The Map's size in one arrangement. w, h: the panel area's content box
    // (h is the height above the tab bar). vitalsH: Health's natural height.
    function columnMap(arrangement, w, h, vitalsH) {
        if (arrangement === 'stacked') {
            return Math.floor(Math.min(w, Math.max(STACKED_MAP_MIN, h - 2 * COL_GAP - VITALS_FLOOR - STATUS_FLOOR)));
        }
        var under = arrangement === 'swapped' ? STATUS_FLOOR : vitalsH;
        return Math.floor(Math.min(w - SIDE_MIN - COL_GAP, h - COL_GAP - under));
    }

    // Which arrangement, and its Map size: { arrangement, map }.
    function columnArrangement(w, h, vitalsH) {
        var beside = vitalsH > h * SWAP_SHARE ? 'swapped' : 'side';
        var besideMap = columnMap(beside, w, h, vitalsH);
        var stackedMap = columnMap('stacked', w, h, vitalsH);
        if (besideMap > stackedMap) return { arrangement: beside, map: besideMap };
        return { arrangement: 'stacked', map: stackedMap };
    }
```

Replace the returned object with:

```js
    return {
        pick: pick, wideMapSize: wideMapSize,
        columnArrangement: columnArrangement, columnMap: columnMap,
        PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT,
        MAP_FLOOR: MAP_FLOOR, MAP_SHARE: MAP_SHARE, MAP_H_TAKEN: MAP_H_TAKEN, MAP_H_SHARE: MAP_H_SHARE,
        COL_GAP: COL_GAP, VITALS_FLOOR: VITALS_FLOOR, STATUS_FLOOR: STATUS_FLOOR,
        STACKED_MAP_MIN: STACKED_MAP_MIN, SIDE_MIN: SIDE_MIN, SWAP_SHARE: SWAP_SHARE
    };
```

In the file's header comment, after the `column` line add:

```
//           Inside column, columnArrangement lays Map, Health and Conditions
//           out stacked, side by side or swapped (plan 7c)
```

- [ ] **Step 4: Run the tests.** Run `node tools/jstest/layoutpick.test.js`. Expected: every check `ok` except `the swapped Conditions strip is the Conditions floor` (Task 2 adds that CSS). Exit 1 only because of that one. Check the arithmetic list above if any other check fails.

- [ ] **Step 5: Commit.**

```bash
git add _datafiles/html/public/static/js/layoutpick.js tools/jstest/layoutpick.test.js
git commit -F - <<'EOF'
feat(webclient): LayoutPick.columnArrangement for the laptop column

Picks stacked, side (Conditions beside the Map, Health below) or swapped
(Health beside, Conditions below once Health passes a third of the
column) by whichever gives the larger Map. Its floors are held to
dashboard.css by the layoutpick jstest.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: The CSS and the wiring (Sonnet)

**Files:**
- Modify: `P/static/css/dashboard.css` (after line 552, the column `#panel-status` rule)
- Modify: `P/static/js/dashboard.js` (`_apply`, `init`, a new `_arrangeColumn`)
- Test: `tools/jstest/dashboard-popout-layout.test.js`

- [ ] **Step 1: Write the failing source guards.** In `tools/jstest/dashboard-popout-layout.test.js`, directly above `if (failures > 0) {` at the end, add:

```js
// The laptop column (plan 7c): dashboard.js lays column out through
// LayoutPick.columnArrangement and clears it in every other mode.
check('the column arrangement is set from LayoutPick',
    /_arrangeColumn: function[\s\S]*?LayoutPick\.columnArrangement\(/.test(SRC)
    && /_arrangeColumn: function[\s\S]*?dataset\.column = /.test(SRC)
    && /_arrangeColumn: function[\s\S]*?setProperty\("--col-map"/.test(SRC), true);
check('the column arrangement is cleared outside column',
    /_arrangeColumn: function[\s\S]*?if \(this\.mode !== "column"\) \{[\s\S]*?delete dash\.dataset\.column;[\s\S]*?removeProperty\("--col-map"\)/.test(SRC), true);
check('the arrangement follows Health and the panel area',
    /new ResizeObserver\([\s\S]*?_arrangeColumn\(\)[\s\S]*?observe\(document\.getElementById\("vitals-bars"\)\)/.test(SRC)
    && /observe\(document\.getElementById\("dash-panels"\)\)/.test(SRC), true);
check('_apply arranges the column even when the mode is unchanged',
    /if \(newMode === this\.mode\) \{ this\._arrangeColumn\(\); return; \}/.test(SRC), true);
check('the keyboard keeps the arrangement',
    /_arrangeColumn: function[\s\S]*?keyboard-open[\s\S]*?LayoutPick\.columnMap\(/.test(SRC), true);
```

- [ ] **Step 2: Run it and see it fail.** Run `node tools/jstest/dashboard-popout-layout.test.js`. Expected: the five new checks FAIL, exit 1.

- [ ] **Step 3: The CSS.** In `P/static/css/dashboard.css`, directly after the line `#dashboard[data-mode="column"] #panel-status { grid-area: status; min-height: 96px; }`, add:

```css
/* The laptop column (plan 7c, owner 2026-11-15). dashboard.js picks one of
   three arrangements (LayoutPick.columnArrangement) and sets data-column and
   --col-map, the Map's size:
   - stacked: the rules above.
   - side: Conditions beside the Map at its height, Health in a strip below
     across the full width, as tall as its rows.
   - swapped: once Health is taller than a third of the column, Health beside
     the Map (it scrolls if its rows do not fit) and Conditions in a strip
     below, its lines in columns. */
#dashboard[data-mode="column"]:is([data-column="side"], [data-column="swapped"]) #dash-panels {
  grid-template-columns: var(--col-map) minmax(0, 1fr);
  align-content: start;
}
#dashboard[data-mode="column"][data-column="side"] #dash-panels {
  grid-template-rows: var(--col-map) auto;
  grid-template-areas:
    "map    status"
    "vitals vitals";
}
#dashboard[data-mode="column"][data-column="swapped"] #dash-panels {
  grid-template-rows: var(--col-map) 96px;
  grid-template-areas:
    "map    vitals"
    "status status";
}
#dashboard[data-mode="column"]:is([data-column="side"], [data-column="swapped"]) #panel-map {
  width: var(--col-map);
  justify-self: start;
}
#dashboard[data-mode="column"]:is([data-column="side"], [data-column="swapped"]) #panel-vitals,
#dashboard[data-mode="column"]:is([data-column="side"], [data-column="swapped"]) #panel-status { min-height: 0; }
#dashboard[data-mode="column"][data-column="swapped"] #status-conditions {
  display: block;
  column-width: 14ch;
  column-gap: 16px;
}
#dashboard[data-mode="column"][data-column="swapped"] #status-conditions > * {
  break-inside: avoid;
  margin-bottom: 6px;
}
```

- [ ] **Step 4: The wiring.** In `P/static/js/dashboard.js`:

Replace `      if (newMode === this.mode) return; // guard: no-op when mode unchanged` with:

```js
      // Unchanged mode: a resize within column still changes its room.
      if (newMode === this.mode) { this._arrangeColumn(); return; }
```

Replace `      if (newMode === "wide") this._repopAfterNarrow();` (the last line of `_apply`) with:

```js
      if (newMode === "wide") this._repopAfterNarrow();
      this._arrangeColumn();
```

Directly after the `_apply` method's closing `},` add:

```js

    // The laptop column (plan 7c): Conditions beside a larger Map with
    // Health below, Health and Conditions swapped once Health passes a third
    // of the column, or the stack where that gives the larger Map
    // (LayoutPick.columnArrangement). Sets data-column and --col-map on
    // #dashboard; clears both in every other mode. While the on-screen
    // keyboard is up the arrangement is kept (as the ladder keeps its
    // height) and only the Map's size follows the smaller room.
    _arrangeColumn: function () {
      var dash = document.getElementById("dashboard");
      if (!dash) return;
      if (this.mode !== "column") {
        delete dash.dataset.column;
        dash.style.removeProperty("--col-map");
        return;
      }
      var panels = document.getElementById("dash-panels");
      var vitals = document.getElementById("panel-vitals");
      var bars = document.getElementById("vitals-bars");
      if (!panels || !vitals || !bars) return;
      var cs = getComputedStyle(panels);
      var w = panels.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
      var h = panels.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom);
      // Health's natural height: its frame and title bar plus its rows, not
      // the cell it was given (the body scrolls when the cell is smaller).
      var body = vitals.querySelector(".dash-panel-body");
      var vitalsH = vitals.offsetHeight - (body ? body.clientHeight : 0) + bars.offsetHeight;
      var keyboard = document.documentElement.classList.contains("keyboard-open");
      var arrangement, map;
      if (keyboard && dash.dataset.column) {
        arrangement = dash.dataset.column;
        map = LayoutPick.columnMap(arrangement, w, h, vitalsH);
      } else {
        var pick = LayoutPick.columnArrangement(w, h, vitalsH);
        arrangement = pick.arrangement;
        map = pick.map;
      }
      if (dash.dataset.column !== arrangement) dash.dataset.column = arrangement;
      dash.style.setProperty("--col-map", Math.max(0, map) + "px");
    },
```

In `init`, directly after the block

```js
      window.addEventListener("resize", function () {
        self._apply();
        self._relayoutTabs();
      });
```

add:

```js
      // The laptop column follows Health (a member joins or leaves) and the
      // panel area (the tab name pill changes its padding) as well as the
      // window (plan 7c).
      if (window.ResizeObserver) {
        var arrange = new ResizeObserver(function () { self._arrangeColumn(); });
        if (document.getElementById("vitals-bars")) arrange.observe(document.getElementById("vitals-bars"));
        if (document.getElementById("dash-panels")) arrange.observe(document.getElementById("dash-panels"));
      }
```

Note: the guard in Step 1 looks for `observe(document.getElementById("vitals-bars"))` and `observe(document.getElementById("dash-panels"))`; keep those exact calls.

- [ ] **Step 5: Run the tests.** Run `node tools/jstest/dashboard-popout-layout.test.js` and `node tools/jstest/layoutpick.test.js` (now all `ok`, including the strip check). Then every jstest: `for t in tools/jstest/*.test.js; do node "$t" > /dev/null || echo "FAIL $t"; done`. Expected: no output.

- [ ] **Step 6: Commit.**

```bash
git add _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/static/js/dashboard.js tools/jstest/dashboard-popout-layout.test.js
git commit -F - <<'EOF'
feat(webclient): a bigger Map in the laptop column

Column lays Map, Health and Conditions out side by side when that gives
the larger Map: Conditions beside the Map and Health in a strip below,
or Health beside it and Conditions below once Health passes a third of
the column. A narrow column keeps the stack. dashboard.js measures the
panel area and Health and follows both with a ResizeObserver.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Patch note (controller)

- [ ] In `docs/PATCH_NOTES.md`, `## 2026-11-14: Clearer panels`, replace the laptop bullet's second sentence ("If you leave full screen on a small laptop, the Map, Health and Conditions stay beside the game and the other panels move to the round buttons, so the Map stays a useful size.") with: "If you leave full screen on a small laptop, the other panels move to the round buttons and the Map grows, with Conditions beside it and Health underneath; a large party puts Health beside the Map instead." Commit by name with a `docs:` message.

### Task 4: Verify (controller)

- [ ] Every jstest; lint (`golangci-lint run --new-from-merge-base=origin/master`); `go test ./...`; boot check on private ports per the `dogmud-shipping` skill (detached `C:/tmp/dogmud-boot-check`, ports 43333 / 19999 / 18090 / 45555, start in the background, record the PID). Expected: no FAIL, 0 issues, all pass, `Server Ready` 1 and 0 panic lines.
- [ ] Headless Edge against `http://127.0.0.1:18090/webclient` with fake Health rows (as the mocks did): screen 1440×900 at 1440×700 with one, two and seven rows (side, side, swapped); screen 1280×800 at 1280×700 (side, Map about 355); a 1100×700 window (stacked). Record `data-column`, `--col-map` and the Map's measured width; confirm the Map is square and nothing overflows the panel area (`#dash-panels` scrollHeight equals clientHeight). Save PNGs of side and swapped to the main checkout's root for the owner.
- [ ] Add a row (via the fake renderer) while side is showing, and check the arrangement and Map follow without a resize.
- [ ] Stop the boot server by PID; `git worktree remove C:/tmp/dogmud-boot-check`.
- [ ] Push; comment on #870 naming plan 7c.
- [ ] Merge the branch into `C:/tmp/dogmud-devtest` (web files are served from disk, so the LAN server picks it up on reload, no restart); confirm with `curl` that the LAN server serves `columnArrangement`.
