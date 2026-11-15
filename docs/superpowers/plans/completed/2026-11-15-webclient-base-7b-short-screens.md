# Web client base plan 7b: short screens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Laptops open the web client full screen like phones, and the layout ladder uses the wide layout only when its square Map would be at least 300px, otherwise column.

**Architecture:** Two pure helpers change in their node-testable modules (`layoutpick.js`, `viewportfit.js`); their callers (`dashboard.js`, `webclient.html`) pass one more value. A jstest guards the copy of the CSS Map formula in JavaScript.

**Tech Stack:** Plain browser JavaScript (UMD modules), node jstests in `tools/jstest/`, CSS.

**Spec:** `docs/superpowers/specs/2026-11-15-webclient-base-7b-short-screens-design.md` (approved 2026-11-15).
**Branch and worktree:** `feature/webclient-base-7-panels` in `C:/tmp/dogmud-p7` (PR #870). All paths below are relative to that worktree. `P` = `_datafiles/html/public`.

**Rules for every worker:** edit files with the Edit and Write tools only (no `sed -i`, no Python or shell writes into tracked files). `git add` named paths only. No dashes (em or en) in comments or copy. Do not touch the LAN test server (process 29796) or any server you did not start.

## Facts verified against source (2026-11-15, `908b49de8`)

| Fact | Where |
|---|---|
| `var WIDE = 360, COLUMN = 200, SLIM = 120;` and `pick(dashW, gameW, phone)` | `P/static/js/layoutpick.js:24`, `:36-43` |
| Export line `return { pick: pick, PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT };` | `P/static/js/layoutpick.js:45` |
| The only caller: `return LayoutPick.pick(w, this.gameW \|\| LayoutPick.GAME_W_DEFAULT, phone);` | `P/static/js/dashboard.js:324` |
| `--map-w: min(58%, calc((100vh - 192px) * 0.52));` | `P/static/css/dashboard.css:464` |
| The wide comment `/* Wide, Layout D: 360px or more right of the game (spec ruling 9). The` | `P/static/css/dashboard.css:443` |
| `isPhone`, `wantFullScreen(pref, phone)`, `enterFullScreenOnConnect(phone, pref, browserCan, browserOn)`, the export block | `P/static/js/viewportfit.js:18-20`, `:81-86`, `:136-141`, `:143-160` |
| The two callers that pass `ViewportFit.isPhone(window.screen.width, window.screen.height)` | `P/webclient.html:107-110`, `:115-119` |
| The Enter Game comment "On a phone, Enter Game also takes the game full screen" | `P/webclient-pure.html:2887` |
| `layoutpick.test.js` uses `G = 785` and nine `pick` cases | `tools/jstest/layoutpick.test.js:36-46` |
| `viewportfit.test.js` full screen cases | `tools/jstest/viewportfit.test.js:76-79`, `:225-229` |
| CI runs every `tools/jstest/*.test.js` | `.github/workflows/validate.yml:114` |

Arithmetic the tests rely on (`wideMapSize = min(0.58 * room, (viewH - 192) * 0.52)`):
- Room 518 gives 300.44, room 517 gives 299.86 (with a tall frame).
- Frame 769 gives 300.88, frame 768 gives 299.52 (with a wide room).
- `pick(1920, 785, false, 1000)`: room 1117, Map min(647.9, 420.2) = 420.2, wide.

---

### Task 1: The ladder checks the Map (Sonnet)

**Files:**
- Modify: `P/static/js/layoutpick.js`
- Modify: `P/static/js/dashboard.js:1-12` (header comment), `:324`
- Modify: `P/static/css/dashboard.css:443-445` (comment only)
- Test: `tools/jstest/layoutpick.test.js`

- [ ] **Step 1: Write the failing tests.** In `tools/jstest/layoutpick.test.js`, replace everything from `var G = 785;` through the line `check('no game width yet keeps the phone layout', ...);` with:

```js
var G = 785;
// A frame tall enough that only width decides (the Map's height share is 420px).
var TALL = 1000;
check('a monitor gets the full layout', LayoutPick.pick(1920, G, false, TALL), 'wide');
// Owner 2026-11-15: wide only when its square Map is at least 300px. Width
// alone now needs 518px of room for that, so 360 to 517px of room, which was
// wide under the 2026-11-12 ladder, is one column.
check('518px of room is the full layout', LayoutPick.pick(12 + G + 6 + 518, G, false, TALL), 'wide');
check('517px of room is one column', LayoutPick.pick(12 + G + 6 + 517, G, false, TALL), 'column');
check('360px of room is one column now', LayoutPick.pick(12 + G + 6 + 360, G, false, TALL), 'column');
check('359px of room is one column', LayoutPick.pick(12 + G + 6 + 359, G, false, TALL), 'column');
check('200px of room is one column', LayoutPick.pick(12 + G + 6 + 200, G, false, TALL), 'column');
check('199px of room is the slim column', LayoutPick.pick(12 + G + 6 + 199, G, false, TALL), 'slim');
check('120px of room is the slim column', LayoutPick.pick(12 + G + 6 + 120, G, false, TALL), 'slim');
check('119px of room is the phone layout', LayoutPick.pick(12 + G + 6 + 119, G, false, TALL), 'phone');
check('narrower than the game is the phone layout', LayoutPick.pick(640, G, false, TALL), 'phone');
check('a phone is always the phone layout', LayoutPick.pick(1920, G, true, TALL), 'phone');
check('no game width yet keeps the phone layout', LayoutPick.pick(1920, null, false, TALL), 'phone');

// A short frame (a laptop with the site header showing) steps down to column.
check('a 769px frame keeps the full layout', LayoutPick.pick(1920, G, false, 769), 'wide');
check('a 768px frame is one column', LayoutPick.pick(1920, G, false, 768), 'column');
// Measured 2026-11-15: a 1440x790 window with the header showing has a 667px
// frame and a 741px game; a 1440x900 screen in browser full screen has 900.
check('a 1440 laptop with the header showing is one column', LayoutPick.pick(1440, 741, false, 667), 'column');
check('a 1440 laptop in full screen is the full layout', LayoutPick.pick(1440, 741, false, 900), 'wide');
// A short frame never pushes a narrow window below column.
check('a short frame does not change slim', LayoutPick.pick(12 + G + 6 + 150, G, false, 500), 'slim');

// wideMapSize is the JavaScript copy of dashboard.css --map-w.
function near(a, b) { return Math.abs(a - b) < 0.01; }
check('wideMapSize is width bound when tall', near(LayoutPick.wideMapSize(500, 1200), 290), true);
check('wideMapSize is height bound when short', near(LayoutPick.wideMapSize(1000, 692), 260), true);
check('wideMapSize at the measured 1440x790 laptop', near(LayoutPick.wideMapSize(681, 667), 247), true);
check('the Map floor is 300px', LayoutPick.MAP_FLOOR, 300);

// The CSS formula and layoutpick.js must not drift apart.
var mapW = /--map-w:\s*min\(([\d.]+)%,\s*calc\(\(100vh - ([\d.]+)px\) \* ([\d.]+)\)\);/.exec(css);
check('the stylesheet still has the --map-w formula', !!mapW, true);
if (mapW) {
    check('the Map width share matches the stylesheet', parseFloat(mapW[1]) / 100, LayoutPick.MAP_SHARE);
    check('the height the Map leaves matches the stylesheet', parseFloat(mapW[2]), LayoutPick.MAP_H_TAKEN);
    check('the Map height share matches the stylesheet', parseFloat(mapW[3]), LayoutPick.MAP_H_SHARE);
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `node tools/jstest/layoutpick.test.js`
Expected: FAIL lines, including `360px of room is one column now` (got wide), `a 768px frame is one column` (got wide) and the `wideMapSize` checks (a TypeError on `LayoutPick.wideMapSize` is also a valid failure; exit code non-zero).

- [ ] **Step 3: Implement.** In `P/static/js/layoutpick.js`, replace the header comment's ladder lines:

```js
//   wide    360px or more: Layout D, every panel open (spec ruling 9)
//   column  200 to 359:    Map, Health and Conditions; the other four on buttons
```

with

```js
//   wide    Layout D, every panel open (spec ruling 9), only when its square
//           Map would be at least 300px (owner 2026-11-15); width alone needs
//           518px of room for that, and a short frame needs more
//   column  200px or more otherwise: Map, Health and Conditions; the other four on buttons
```

Replace `    var WIDE = 360, COLUMN = 200, SLIM = 120;` with:

```js
    var COLUMN = 200, SLIM = 120;
    // Wide only when its square Map is at least this big (owner 2026-11-15).
    var MAP_FLOOR = 300;
    // Wide's Map, as dashboard.css --map-w: min(58%, calc((100vh - 192px) * 0.52)),
    // where 58% is of the panel area, whose width is the room. A jstest holds
    // the two together.
    var MAP_SHARE = 0.58, MAP_H_TAKEN = 192, MAP_H_SHARE = 0.52;

    // room: the panel area's width. viewH: the frame's height (100vh here).
    function wideMapSize(room, viewH) {
        return Math.min(MAP_SHARE * room, (viewH - MAP_H_TAKEN) * MAP_H_SHARE);
    }
```

Replace the `pick` comment line `    // short side is under 600px (viewportfit.isPhone and pointer: coarse).` with:

```js
    // short side is under 600px (viewportfit.isPhone and pointer: coarse).
    // viewH: the frame's height in CSS px (window.innerHeight in the frame).
```

Replace the function:

```js
    function pick(dashW, gameW, phone) {
        if (phone || !gameW) return 'phone';
        var room = dashW - 2 * PAD - gameW - GAP;
        if (room >= WIDE) return 'wide';
        if (room >= COLUMN) return 'column';
```

with

```js
    function pick(dashW, gameW, phone, viewH) {
        if (phone || !gameW) return 'phone';
        var room = dashW - 2 * PAD - gameW - GAP;
        if (room >= COLUMN && wideMapSize(room, viewH) >= MAP_FLOOR) return 'wide';
        if (room >= COLUMN) return 'column';
```

Replace the export line with:

```js
    return {
        pick: pick, wideMapSize: wideMapSize,
        PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT,
        MAP_FLOOR: MAP_FLOOR, MAP_SHARE: MAP_SHARE, MAP_H_TAKEN: MAP_H_TAKEN, MAP_H_SHARE: MAP_H_SHARE
    };
```

In `P/static/js/dashboard.js`, line 324, replace

```js
      return LayoutPick.pick(w, this.gameW || LayoutPick.GAME_W_DEFAULT, phone);
```

with

```js
      return LayoutPick.pick(w, this.gameW || LayoutPick.GAME_W_DEFAULT, phone, window.innerHeight);
```

In the same file's header comment, replace ` *   wide   : Layout D, every side panel open (spec ruling 9)` with:

```js
 *   wide   : Layout D, every side panel open (spec ruling 9), when its
 *            square Map would be at least 300px (owner 2026-11-15)
```

In `P/static/css/dashboard.css`, replace the comment opening line 443

```css
/* Wide, Layout D: 360px or more right of the game (spec ruling 9). The
```

with

```css
/* Wide, Layout D (spec ruling 9): picked when the Map below would be at
   least 300px (layoutpick.js copies --map-w; owner 2026-11-15). The
```

Leave the rest of that comment as it is.

- [ ] **Step 4: Run the tests and see them pass.**

Run: `node tools/jstest/layoutpick.test.js`
Expected: every line `ok`, exit 0.
Run: `node tools/jstest/dashboard-popout-layout.test.js`
Expected: exit 0 (it reads `dashboard.js`; it must still pass).

- [ ] **Step 5: Commit.**

```bash
git add _datafiles/html/public/static/js/layoutpick.js _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/static/css/dashboard.css tools/jstest/layoutpick.test.js
git commit -F - <<'EOF'
feat(webclient): wide layout only when its Map is at least 300px

A short laptop window got every panel open and a 200 to 300px Map. The
ladder now takes the frame's height and steps down to column when wide's
square Map would be under 300px. A jstest holds the JavaScript copy of
the --map-w formula to the stylesheet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Laptops open full screen (Sonnet)

**Files:**
- Modify: `P/static/js/viewportfit.js:18-20`, `:81-86`, `:136-141`, export block
- Modify: `P/webclient.html:107-110`, `:115-119`
- Modify: `P/webclient-pure.html:2887-2888` (comment only)
- Test: `tools/jstest/viewportfit.test.js`

- [ ] **Step 1: Write the failing tests.** In `tools/jstest/viewportfit.test.js`, after the line `check('a desktop player who turned it on stays on', VF.wantFullScreen('1', false), true);` add:

```js
// Laptops (owner 2026-11-15): a screen short side under 1000px that is not
// a phone opens full screen like a phone.
check('a 13 inch MacBook is a short screen', VF.isShortScreen(1440, 900), true);
check('a 14 inch MacBook is a short screen', VF.isShortScreen(1512, 982), true);
check('a 1080p laptop at 125% is a short screen', VF.isShortScreen(1536, 864), true);
check('a 1080p monitor is not', VF.isShortScreen(1920, 1080), false);
check('a phone is not a short screen (it is a phone)', VF.isShortScreen(390, 844), false);
check('a laptop opens full screen', VF.wantFullScreen(null, VF.isShortScreen(1440, 900)), true);
check('a laptop player who turned it off stays off', VF.wantFullScreen('0', VF.isShortScreen(1440, 900)), false);
```

Replace the line `check('not on a laptop or tablet', VF.enterFullScreenOnConnect(false, null, true, false), false);` with:

```js
check('not on a screen that does not default to full screen', VF.enterFullScreenOnConnect(false, null, true, false), false);
check('Enter Game on a laptop goes full screen', VF.enterFullScreenOnConnect(VF.isShortScreen(1440, 900), null, true, false), true);
check('a laptop player who left full screen keeps that', VF.enterFullScreenOnConnect(VF.isShortScreen(1440, 900), '0', true, false), false);
```

- [ ] **Step 2: Run it and see it fail.**

Run: `node tools/jstest/viewportfit.test.js`
Expected: a TypeError (`VF.isShortScreen is not a function`), exit non-zero.

- [ ] **Step 3: Implement.** In `P/static/js/viewportfit.js`, after the `isPhone` function (the closing `}` at line 20) add:

```js

    // A laptop (owner 2026-11-15): a screen whose short side is under 1000px
    // and that is not a phone. It opens full screen as a phone does, so the
    // site's header and footer leave the game their height.
    function isShortScreen(screenW, screenH) {
        return !isPhone(screenW, screenH) && Math.min(screenW, screenH) < 1000;
    }
```

Replace

```js
    // pref is the remembered choice: '1', '0', or null for none yet.
    function wantFullScreen(pref, phone) {
        if (pref === '1') return true;
        if (pref === '0') return false;
        return phone;
    }
```

with

```js
    // pref is the remembered choice: '1', '0', or null for none yet.
    // byDefault: a phone or a laptop (isPhone or isShortScreen).
    function wantFullScreen(pref, byDefault) {
        if (pref === '1') return true;
        if (pref === '0') return false;
        return byDefault;
    }
```

Replace

```js
    function enterFullScreenOnConnect(phone, pref, browserCan, browserOn) {
        return phone && pref !== '0' && browserCan && !browserOn;
    }
```

with

```js
    function enterFullScreenOnConnect(byDefault, pref, browserCan, browserOn) {
        return byDefault && pref !== '0' && browserCan && !browserOn;
    }
```

and in the comment just above it change `// Whether tapping Enter Game should also take a phone full screen` to `// Whether tapping Enter Game should also take a phone or a laptop full screen`. In the export block, add `isShortScreen: isShortScreen,` on the line after `isPhone: isPhone,`.

In `P/webclient.html`, add this function directly above `function shown() {`:

```js
        // Phones and laptops open full screen unless the player chose
        // otherwise (owner 2026-11-07 for phones, 2026-11-15 for laptops).
        function byDefault() {
            const w = window.screen.width, h = window.screen.height;
            return ViewportFit.isPhone(w, h) || ViewportFit.isShortScreen(w, h);
        }
```

Replace

```js
                apply(ViewportFit.wantFullScreen(readPref(),
                    ViewportFit.isPhone(window.screen.width, window.screen.height)));
```

with

```js
                apply(ViewportFit.wantFullScreen(readPref(), byDefault()));
```

Replace

```js
                return ViewportFit.enterFullScreenOnConnect(
                    ViewportFit.isPhone(window.screen.width, window.screen.height),
                    readPref(), browserCan(), browserOn());
```

with

```js
                return ViewportFit.enterFullScreenOnConnect(
                    byDefault(), readPref(), browserCan(), browserOn());
```

In `P/webclient-pure.html`, replace

```js
            // On a phone, Enter Game also takes the game full screen, hiding
            // the browser's address bar (Calabe 2026-11-07). The game usually
```

with

```js
            // On a phone or a laptop, Enter Game also takes the game full
            // screen, hiding the browser's own bars (Calabe 2026-11-07 for
            // phones, 2026-11-15 for laptops). The game usually
```

- [ ] **Step 4: Run the tests and see them pass.**

Run: `node tools/jstest/viewportfit.test.js`
Expected: `all checks passed`, exit 0.
Run: `grep -n "isPhone(window.screen" _datafiles/html/public/webclient.html` on its own.
Expected: no output (exit 1 is the expected result here: both callers now go through `byDefault()`).

- [ ] **Step 5: Commit.**

```bash
git add _datafiles/html/public/static/js/viewportfit.js _datafiles/html/public/webclient.html _datafiles/html/public/webclient-pure.html tools/jstest/viewportfit.test.js
git commit -F - <<'EOF'
feat(webclient): laptops open full screen like phones

A screen whose short side is under 1000px and is not a phone now starts
with the site header and footer hidden, and Enter Game takes it into the
browser's full screen. A saved choice to leave full screen still wins.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Patch note and docs (controller)

**Files:**
- Modify: `docs/PATCH_NOTES.md` (the `## 2026-11-14: Clearer panels` section)
- No `context.md` changes: none of the 155 in the repo names `layoutpick`, the ladder or full screen (checked 2026-11-15; `internal/web/context.md` is the only one about the web client and it covers the server).

- [ ] **Step 1:** Add this bullet at the end of the `## 2026-11-14: Clearer panels` list in `docs/PATCH_NOTES.md`:

```markdown
- On a laptop the web client opens full screen, so every panel fits beside the game. If you leave full screen on a small laptop, the Map, Health and Conditions stay beside the game and the other panels move to the round buttons, so the Map stays a useful size.
```

- [ ] **Step 2: Commit** `docs/PATCH_NOTES.md` by name with a `docs:` message and the Co-Authored-By line.

---

### Task 4: Verify (controller)

- [ ] **Step 1: Every jstest.**

Run: `for t in tools/jstest/*.test.js; do node "$t" > /dev/null || echo "FAIL $t"; done`
Expected: no output.

- [ ] **Step 2: Go gates** (no Go changed; run them because the PR gate does): `go build ./...`, `golangci-lint run --new-from-merge-base=origin/master`, `go test ./...`. Expected: build ok, 0 issues, all pass (#878's flaky apiframework test excepted if it is the only failure; rerun it once).

- [ ] **Step 3: Boot check on private ports**, per the `dogmud-shipping` skill's "Boot check in an isolated worktree": detached worktree `C:/tmp/dogmud-boot-check` at this branch's HEAD, override `Network: TelnetPort [43333], LocalPort 19999, HttpPort 18090, AIPort 45555` in the session scratchpad. Start it in the background and record its PID. Expected: `Server Ready` 1, panic lines 0.

- [ ] **Step 4: Headless Edge check** against `http://127.0.0.1:18090/webclient` (Playwright from `C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright`, script in the scratchpad). For each case read `header` display, the frame height, `#dashboard`'s `data-mode` and `#panel-map`'s width:
  - screen 1440×900, viewport 1440×700: header hidden, frame 700, mode `column`.
  - same page after `window.WebclientFrame.setFullScreen(false)`: header shown, mode `column`.
  - screen 1440×900, viewport 1440×900: header hidden, mode `wide`, Map about 368px.
  - screen 1920×1080, viewport 1920×950: header shown, mode `wide`, Map about 330px.
  - screen 390×844 with touch (phone): mode `phone`, header hidden (unchanged).
  Save a PNG of the first and third cases in the scratchpad for the owner.

- [ ] **Step 5:** Stop the boot-check server by its PID, `git worktree remove C:/tmp/dogmud-boot-check`.

- [ ] **Step 6: Push** `feature/webclient-base-7-panels` and add a comment on PR #870 (`gh pr comment 870 --repo pruuk/DOGMud`) naming plan 7b and the spec.

- [ ] **Step 7: The LAN test server.** Restart it with this build only when the owner says the current phone test is finished; until then process 29796 stays up untouched. Then merge the branch into `C:/tmp/dogmud-devtest`, build to the scratchpad, and restart it as on 2026-11-15 (`CONFIG_PATH` = the scratchpad `lan.yaml`), so Malia can check both Macs.
