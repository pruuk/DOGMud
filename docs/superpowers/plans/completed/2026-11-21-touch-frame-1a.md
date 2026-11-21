# Touch frame plan 1a (the frame) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put a phone's game column in one stack (game text, a slot holding the controls deck or the open tab, the command box), so an open tab never covers the newest game lines, and build the deck switched off behind a Touch controls setting.

**Architecture:** A new pure module `static/js/touchdeck.js` (UMD, like `layoutpick.js`) owns the setting, whether the deck shows, the deck's shape for its box, and what the slot shows. `webclient-pure.html` gets a `#touch-slot` between `#panel-feed` and `#panel-cmd` and a Settings section; `dashboard.js` moves the existing `#dash-drawer` element into the slot in phone mode (so every `#dash-drawer` rule keeps working) and applies the deck; `dashboard.css` styles the slot and turns the phone drawer from a fixed card into part of the stack.

**Tech Stack:** Plain browser JavaScript (no build), CSS, Node for tests (`tools/jstest`, `tools/webclient-tests`, no packages), headless Edge over the DevTools protocol (`tools/webshot/live-shot.js`).

**Spec:** `docs/superpowers/specs/2026-11-21-touch-frame-design.md` (approved 2026-11-21). This plan is 1a; 1b (keyboard) and 1c (phone polish) get their own plans.

**Branch:** build in a worktree on `feat/touch-frame-1a`, cut from master once
the spec and this plan have merged (they ride `docs/touch-frame-spec`).

---

## Rules for every task (read first)

- **Edit files with the Edit and Write tools only.** No `sed -i`, no Python
  `open(...,'w')`, no `>` or `>>` into a tracked file. A hook (`guard.py`)
  blocks those and the turn is wasted.
- `git add <named paths>` only; never `-A`, `.`, `--all`, `commit -a`. Never
  `--no-verify`. Never stash.
- Commit messages through a quoted heredoc (`git commit -F - <<'EOF'`), ending
  with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes in comments, copy or commit messages.
- Never kill a process you did not start; kill by PID only. The owner's own
  server may be running on this machine: never point a test at it.
- All paths below are relative to the repo root
  `C:\Users\Calabe Davis\workspace\DOGMud`. The web root is
  `_datafiles/html/public/` (shortened to `public/` in prose only; commands use
  the full path).

## Facts verified against source (2026-11-21, master `d99b9706d`)

| Fact | Where |
|---|---|
| Center column is `#dash-col-center`, a flex column holding `#panel-feed` then `#panel-cmd` | `webclient-pure.html:309-324`, `dashboard.css:29-34` |
| `#panel-feed` grows (`flex: 1 1 0`); `#panel-cmd` is `flex: 0 0 auto; margin-top: auto` (hotinput.test.js asserts that exact text) | `dashboard.css:173`, `:209`; `tools/jstest/hotinput.test.js:481` |
| The command box joins the game window only through `+` selectors: `#panel-feed + #panel-cmd` (sight vars, sight colours, the negative margin) and `#panel-feed:has(+ #panel-cmd)` | `dashboard.css:179-194`, `:229-238` |
| `sight-border.js` only requires `#panel-feed.sight-<band> { ... border-color` rules | `tools/webclient-tests/sight-border.js:72-73` |
| The drawer is built once in `_buildPhone` and appended to `#dashboard`'s parent (body); panels move into its `.drawer-body`; `_openDrawer` sets `style.display = ""` then adds `.open`; `_closeDrawer` removes `.open`; `_exitTabbed` sets `style.display = "none"` | `dashboard.js:505-556`, `:590-596`, `:598-670` |
| `_enterPhone` / `_exitPhone` only call `_enterTabbed` / `_exitTabbed` | `dashboard.js:502-503` |
| `_apply` returns early when the mode is unchanged, else exits the old mode, enters the new one, then `_arrangeColumn()` | `dashboard.js:358-388` |
| The phone card's cap is `--drawer-cap` declared on a selector list that includes `#dash-drawer` (30%, Map 45%, 30% of `--visible-h` with the keyboard up) | `dashboard.css:748-767`, `:880-883`, `:1205-1208` |
| `#dash-drawer` is fixed (`position: fixed; bottom: calc(var(--cmdbox-h) + var(--tabbar-h) + var(--tabname-h))`, slid off with `transform`, `visibility: hidden` until `.open`) | `dashboard.css:905-930` |
| hotinput.test.js asserts the `bottom: calc(var(--cmdbox-h) + var(--tabbar-h) + var(--tabname-h))` text and the `#dash-drawer .inv-grid, #dash-drawer .auto-list { flex: 0 1 auto;` text | `tools/jstest/hotinput.test.js:478-490` |
| The phone tab bar is fixed and `#dashboard[data-mode="phone"]` is padded for it (and for the name strip while a tab is open) | `dashboard.css:792-800`, `:825-852` |
| Settings has Sound and Appearance fieldsets; Appearance radios use `class="theme-choice"`, wired in an inline script that runs `showThemeChoice()` and listens to `storage` | `webclient-pure.html:275-299`, `:3810-3827` |
| `layoutpick.js` and `viewportfit.js` load in the head (`:224`, `:236`); `dashboard.js` loads last (`:5248`) | `webclient-pure.html` |
| `ViewportFit.isPhone(w,h)`, `isSidewaysPhone(w,h,orientationType,coarse)`, `orientationType(win)` exist | `viewportfit.js:18`, `:42`, `:50` |
| Tokens exist: `--ink-deep`, `--gold-rule`, `--brass-border`, `--panel-bg`, `--antique-gold` (dark and light) | `gomud.css:4-37`, `:228-240` |
| jstests are plain Node, one `check(name, got, want)`, `process.exit(failures ? 1 : 0)`; CI runs every `tools/jstest/*.test.js` and every `tools/webclient-tests/*.js` | `tools/jstest/layoutpick.test.js`, `.github/workflows/validate.yml:107-128` |
| `dashboard-popout-layout.test.js` loads the real `dashboard.js` with a fake DOM | `docs/README.md:109` |
| `live-shot.js` has no device or touch emulation today; its config is per run (`cfg.width/height`) and sessions are looped at `:153` | `tools/webshot/live-shot.js:134-191` |
| `docs/README.md` lists each new static JS file, each jstest and each plan | `docs/README.md:104-109`, `:337-342` |

## File structure

| File | Change | Responsibility |
|---|---|---|
| `_datafiles/html/public/static/js/touchdeck.js` | Create | Pure: setting, deck on/off, deck shape, slot content |
| `tools/jstest/touchdeck.test.js` | Create | Unit tests for `touchdeck.js` |
| `tools/jstest/touch-targets.test.js` | Create | Guard 6: target sizes at the size matrix, and the CSS reads its sizes from `touchdeck.js` |
| `tools/webclient-tests/touch-frame.js` | Create | Static checks: page wiring and stylesheet of the frame |
| `_datafiles/html/public/webclient-pure.html` | Modify | Slot and deck elements, Settings section, script include, Settings wiring |
| `_datafiles/html/public/static/css/dashboard.css` | Modify | Slot, deck, divider, sight rules through the slot, drawer in the slot, sideways side column |
| `_datafiles/html/public/static/js/dashboard.js` | Modify | Drawer into the slot on phones, `applyDeck`, keyboard watch |
| `tools/webshot/live-shot.js` | Modify | Optional per-session device and touch emulation |
| `docs/README.md` | Modify | Rows for the new files and this plan |

---

### Task 1: `touchdeck.js`, the pure module

**Files:**
- Create: `_datafiles/html/public/static/js/touchdeck.js`
- Test: `tools/jstest/touchdeck.test.js`

- [ ] **Step 1: Write the failing test**

Create `tools/jstest/touchdeck.test.js`:

```js
// touchdeck.test.js: the touch controls deck's pure functions (touch frame
// plan 1a, docs/superpowers/specs/2026-11-21-touch-frame-design.md).
//
// Run:  node tools/jstest/touchdeck.test.js
'use strict';

var path = require('path');
var TD = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}
function store(init) {
    var m = Object.assign({}, init);
    return {
        getItem: function (k) { return Object.prototype.hasOwnProperty.call(m, k) ? m[k] : null; },
        setItem: function (k, v) { m[k] = String(v); },
        saved: m
    };
}
var blocked = {
    getItem: function () { throw new Error('blocked'); },
    setItem: function () { throw new Error('blocked'); }
};

// The setting, on this device.
check('the storage key', TD.KEY, 'dogmud.touchControls');
check('no choice yet reads Automatic', TD.readPref(store()), 'auto');
check('a saved Always reads back', TD.readPref(store({ 'dogmud.touchControls': 'always' })), 'always');
check('a saved Never reads back', TD.readPref(store({ 'dogmud.touchControls': 'never' })), 'never');
check('junk reads Automatic', TD.readPref(store({ 'dogmud.touchControls': 'sometimes' })), 'auto');
check('blocked storage reads Automatic', TD.readPref(blocked), 'auto');
check('no storage reads Automatic', TD.readPref(null), 'auto');
var s = store();
TD.writePref(s, 'never');
check('a choice is saved', s.saved['dogmud.touchControls'], 'never');
check('junk is saved as Automatic', TD.writePref(store(), 'x'), 'auto');
check('blocked storage does not throw on save', TD.writePref(blocked, 'always'), 'always');

// Whether the deck shows. Owner 2026-11-21: Automatic is off until plan 2b
// puts movement in the deck.
check('the switch plan 2b turns on is still off', TD.AUTOMATIC_SHOWS, false);
check('Automatic on a touch screen: off until plan 2b', TD.deckOn('auto', true), false);
check('Automatic with a mouse: off', TD.deckOn('auto', false), false);
check('Always with a mouse: on', TD.deckOn('always', false), true);
check('Always on touch: on', TD.deckOn('always', true), true);
check('Never on touch: off', TD.deckOn('never', true), false);

// The deck's shape: shape(boxW, boxH, phone, sideways). boxW and boxH are
// the room the deck may fill, inside the slot's side borders.
var p360 = TD.shape(346, 600, true, false);
check('a 360px phone: two rows of six', [p360.rows, p360.cols], [2, 6]);
check('a 360px phone: 48px buttons, 8px gaps', [p360.button, p360.gap], [48, 8]);
check('a 360px phone: 344 x 120', [p360.width, p360.height], [344, 120]);
check('a Pixel 8 keeps six columns (the same slot on every phone)', TD.shape(398, 700, true, false).cols, 6);
var p320 = TD.shape(306, 500, true, false);
check('a 320px phone: six columns of 44px with 5px gaps', [p320.cols, p320.button, p320.gap], [6, 44, 5]);
check('a 320px phone: fits its 306px box', p320.width <= 306, true);
check('a 343px box is too narrow for 48px', TD.shape(343, 600, true, false).button, 44);
check('a 344px box takes 48px', TD.shape(344, 600, true, false).button, 48);
var ipad = TD.shape(806, 1000, false, false);
check('an upright iPad: two rows of as many 48px columns as fit (14)', [ipad.rows, ipad.cols, ipad.button], [2, 14, 48]);
check('an open foldable: 11 columns', TD.shape(676, 800, false, false).cols, 11);
check('a laptop game column: 13 columns', TD.shape(788, 600, false, false).cols, 13);
var narrow = TD.shape(320, 600, false, false);
check('a narrow box that is not a phone falls back to the phone grid', [narrow.cols, narrow.button], [6, 44]);
var side = TD.shape(0, 232, true, true);
check('a phone on its side: a side column, four rows of three', [side.rows, side.cols, side.side], [4, 3, true]);
check('a phone on its side with 232px: 48px', side.button, 48);
var sideShort = TD.shape(0, 220, true, true);
check('a phone on its side with 220px: 44px and it fits', [sideShort.button, sideShort.height <= 220], [44, true]);
check('an upright deck is not a side column', p360.side, false);

// What the slot under the game text shows.
function shows(phone, tabOpen, deckOn, keyboard) {
    return TD.slotShows({ phone: phone, tabOpen: tabOpen, deckOn: deckOn, keyboard: keyboard });
}
check('phone, a tab open: the tab', shows(true, true, true, false), 'tab');
check('phone, a tab open, deck off: the tab', shows(true, true, false, false), 'tab');
check('phone, a tab open, keyboard up: the tab stays (plan 1b changes this)', shows(true, true, true, true), 'tab');
check('deck on, nothing open: the deck', shows(true, false, true, false), 'deck');
check('deck on, keyboard up: empty (the keyboard covers the deck)', shows(true, false, true, true), 'empty');
check('deck off, nothing open: empty', shows(true, false, false, false), 'empty');
check('not a phone: a tab is a card elsewhere, the slot keeps the deck', shows(false, true, true, false), 'deck');
check('not a phone, deck off: empty', shows(false, true, false, false), 'empty');

process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run the test to see it fail**

Run: `node tools/jstest/touchdeck.test.js`
Expected: exits 1 with `Cannot find module` for `touchdeck.js`.

- [ ] **Step 3: Write `touchdeck.js`**

Create `_datafiles/html/public/static/js/touchdeck.js`:

```js
// touchdeck.js: the touch controls deck (touch arc spec 1, plan 1a;
// docs/superpowers/specs/2026-11-21-touch-frame-design.md).
//
// The deck is a block of buttons above the command box. On a phone it sits
// in the slot under the game text, which holds the deck or the open tab
// (dashboard.js). Buttons only ever come from the server (the roadmap's sync
// rule), so in plan 1a the deck draws empty slots.
//
// Pure functions, loaded by the page and by node for the tests:
//   readPref / writePref  the Touch controls setting on this device
//   deckOn                whether the deck shows
//   shape                 rows, columns and sizes for the room it has
//   slotShows             what the slot under the game text holds
// Tested by tools/jstest/touchdeck.test.js and touch-targets.test.js.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;       // node, for the tests
    } else {
        root.TouchDeck = api;       // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // The setting: Automatic, Always or Never, on this device (like Sound
    // and Appearance).
    var KEY = 'dogmud.touchControls';
    var PREFS = { auto: true, always: true, never: true };

    // Automatic shows the deck on a touch screen only once the deck has
    // buttons (owner ruling 2026-11-21). Plan 2b sets this to true.
    var AUTOMATIC_SHOWS = false;

    function readPref(storage) {
        try {
            var v = storage ? storage.getItem(KEY) : null;
            return PREFS[v] === true ? v : 'auto';
        } catch (e) {
            return 'auto';
        }
    }

    function writePref(storage, value) {
        var v = PREFS[value] === true ? value : 'auto';
        try { storage.setItem(KEY, v); } catch (e) { /* blocked: this visit only */ }
        return v;
    }

    // coarse: touch is the main input (pointer: coarse).
    function deckOn(pref, coarse) {
        if (pref === 'always') return true;
        if (pref === 'never') return false;
        return AUTOMATIC_SHOWS && !!coarse;
    }

    // Sizes (roadmap fat-finger rules; spec ruling 3 for the small size).
    var BTN = 48, GAP = 8, SMALL_BTN = 44, SMALL_GAP = 5, PAD = 8;
    var ROWS = 2, PHONE_COLS = 6, SIDE_ROWS = 4, SIDE_COLS = 3;

    // The length n buttons take in a line, with the deck's padding.
    function span(n, btn, gap) { return n * btn + (n - 1) * gap + 2 * PAD; }

    function make(rows, cols, btn, gap, side) {
        return {
            rows: rows, cols: cols, button: btn, gap: gap, pad: PAD, side: side,
            width: span(cols, btn, gap), height: span(rows, btn, gap)
        };
    }

    // The deck's shape for the room it has. boxW, boxH: that room in CSS px,
    // inside the slot's side borders. phone: an upright phone (six columns,
    // so a button keeps its slot on every phone). sideways: a phone on its
    // side (a side column beside the game, four rows of three).
    function shape(boxW, boxH, phone, sideways) {
        if (sideways) {
            var tall = span(SIDE_ROWS, BTN, GAP) <= boxH;
            return make(SIDE_ROWS, SIDE_COLS, tall ? BTN : SMALL_BTN, tall ? GAP : SMALL_GAP, true);
        }
        var fit = Math.floor((boxW - 2 * PAD + GAP) / (BTN + GAP));
        if (phone || fit < PHONE_COLS) {
            var wide = span(PHONE_COLS, BTN, GAP) <= boxW;
            return make(ROWS, PHONE_COLS, wide ? BTN : SMALL_BTN, wide ? GAP : SMALL_GAP, false);
        }
        return make(ROWS, fit, BTN, GAP, false);
    }

    // What the slot under the game text holds: 'tab', 'deck' or 'empty'.
    // o.phone: the phone layout, where an open tab lives in the slot.
    // o.tabOpen, o.deckOn, o.keyboard: the on-screen keyboard is up.
    // Plan 1b changes the keyboard case: typing in the command box will make
    // the tab step aside too.
    function slotShows(o) {
        if (o.phone && o.tabOpen) return 'tab';
        if (o.deckOn && !o.keyboard) return 'deck';
        return 'empty';
    }

    return {
        KEY: KEY, AUTOMATIC_SHOWS: AUTOMATIC_SHOWS,
        readPref: readPref, writePref: writePref, deckOn: deckOn,
        shape: shape, slotShows: slotShows,
        BTN: BTN, GAP: GAP, SMALL_BTN: SMALL_BTN, SMALL_GAP: SMALL_GAP, PAD: PAD
    };
}));
```

- [ ] **Step 4: Run the test to see it pass**

Run: `node tools/jstest/touchdeck.test.js`
Expected: every line `ok`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck.test.js
git commit -F - <<'EOF'
feat(webclient): touchdeck.js, the touch deck's setting, shape and slot (#783)

Touch frame plan 1a, task 1. Pure functions with jstests; nothing loads
it yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Guard 6, touch targets

**Files:**
- Create: `tools/jstest/touch-targets.test.js`

The stylesheet checks at the end fail until Task 4 adds `#touch-deck` and
`.deck-slot`. Write the whole file now; the matrix and can-fail checks pass
at once.

- [ ] **Step 1: Write the guard**

Create `tools/jstest/touch-targets.test.js`:

```js
// touch-targets.test.js: guard 6 of the touch arc roadmap
// (docs/roadmaps/TOUCH_ARC_ROADMAP.md, "Tests and guards"). At every size in
// the test matrix, every deck slot is at least 48px with 8px gaps. The only
// exception is 44px with 5px gaps where 48px cannot fit (spec ruling 3,
// 2026-11-21). The deck must also fit its box (no sideways scroll), and the
// stylesheet must size it from touchdeck.js, never from numbers of its own.
//
// Run:  node tools/jstest/touch-targets.test.js
'use strict';

var path = require('path');
var fs = require('fs');
var js = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var TD = require(path.join(js, 'touchdeck.js'));
var LayoutPick = require(path.join(js, 'layoutpick.js'));
var css = fs.readFileSync(path.join(js, '..', 'css', 'dashboard.css'), 'utf8');

var failures = 0;
function check(name, ok) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name);
}

var PAD = LayoutPick.PAD;          // #dashboard padding (layoutpick.test.js ties it to the CSS)
var BORDER = 2;                    // the slot's side borders (dashboard.css #touch-slot)
var SESSION = 64, TABBAR = 64;     // the session bar with its gap; the tab bar's room on touch (--tabbar-h)
var GAME_W = LayoutPick.GAME_W_DEFAULT;   // the game column outside phone mode

function span(n, b, g) { return n * b + (n - 1) * g + 2 * TD.PAD; }

// What is wrong with one deck shape in its box: a list, empty when it passes.
function problems(s, boxW, boxH) {
    var out = [];
    var big = s.button >= TD.BTN && s.gap >= TD.GAP;
    var small = s.button >= TD.SMALL_BTN && s.gap >= TD.SMALL_GAP;
    var bigFits = s.side ? span(s.rows, TD.BTN, TD.GAP) <= boxH : span(s.cols, TD.BTN, TD.GAP) <= boxW;
    if (!big && !(small && !bigFits)) out.push(s.button + 'px buttons with ' + s.gap + 'px gaps');
    if (!s.side && s.width > boxW) out.push(s.width + 'px wide in a ' + boxW + 'px box');
    if (s.side && s.height > boxH) out.push(s.height + 'px tall in a ' + boxH + 'px box');
    return out;
}

// The size matrix (roadmap "How we test"), plus the WCAG reflow width.
var MATRIX = [
    ['small iPhone in Safari', 375, 548],
    ['Galaxy S21', 360, 800],
    ['Pixel 8', 412, 915],
    ['WCAG reflow width', 320, 640],
    ['phone on its side', 800, 360],
    ['iPad upright', 820, 1180],
    ['iPad sideways', 1180, 820],
    ['open foldable', 690, 840],
    ['laptop', 1366, 768]
];
MATRIX.forEach(function (m) {
    var w = m[1], h = m[2];
    var phone = Math.min(w, h) < 600;
    var sideways = phone && w > h;
    var mode = LayoutPick.pick(w, GAME_W, phone, h);
    var colW = mode === 'phone' ? w - 2 * PAD : GAME_W;
    var boxW = colW - BORDER;
    var boxH = h - 2 * PAD - SESSION - TABBAR;
    var s = TD.shape(boxW, boxH, phone && !sideways, sideways);
    var p = problems(s, boxW, boxH);
    check(m[0] + ' ' + w + 'x' + h + ': ' + s.rows + 'x' + s.cols + ' at ' + s.button + 'px' +
          (p.length ? ' (' + p.join('; ') + ')' : ''), p.length === 0);
});

// The guard can fail.
check('the guard fails a 47px slot',
    problems({ rows: 2, cols: 6, button: 47, gap: 8, side: false, width: 338, height: 118 }, 346, 600).length > 0);
check('the guard fails 44px where 48px fits',
    problems({ rows: 2, cols: 6, button: 44, gap: 5, side: false, width: 305, height: 109 }, 346, 600).length > 0);
check('the guard fails a deck wider than its box',
    problems({ rows: 2, cols: 6, button: 48, gap: 8, side: false, width: 344, height: 120 }, 300, 600).length > 0);

// The stylesheet sizes the deck from touchdeck.js (dashboard.js sets the
// variables), never with numbers of its own.
var deckRule = (/#touch-deck \{[^}]*\}/.exec(css) || [''])[0];
check('the deck grid takes its button size from --deck-btn', /var\(--deck-btn/.test(deckRule));
check('the deck grid takes its gap from --deck-gap', /var\(--deck-gap/.test(deckRule));
var slotRule = (/\.deck-slot \{[^}]*\}/.exec(css) || [''])[0];
check('a deck slot is styled', slotRule !== '');
check('a deck slot sets no size of its own', !/(^|[;{\s])(width|height|min-width|min-height)\s*:/.test(slotRule));

process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run it**

Run: `node tools/jstest/touch-targets.test.js`
Expected: the nine matrix lines and the three can-fail lines `ok`; the four
stylesheet lines `FAIL` (no `#touch-deck` yet); exit 1. If any matrix line
fails, stop: `touchdeck.js` disagrees with the spec, fix Task 1 first.

- [ ] **Step 3: Commit (the stylesheet checks stay red until Task 4)**

```bash
git add tools/jstest/touch-targets.test.js
git commit -F - <<'EOF'
test(webclient): guard 6, every deck slot 48px with 8px gaps (#783)

Touch frame plan 1a, task 2. The stylesheet checks pass once task 4
styles the deck.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: The page, the slot and the Touch controls setting

**Files:**
- Create: `tools/webclient-tests/touch-frame.js`
- Modify: `_datafiles/html/public/webclient-pure.html` (head scripts near `:236`; Settings `:292-299`; center column `:310-323`; inline Settings script near `:3810-3827`)

- [ ] **Step 1: Write the failing static test**

Create `tools/webclient-tests/touch-frame.js`:

```js
#!/usr/bin/env node
/*
 * Guards the touch frame's page wiring and stylesheet (touch frame plan 1a,
 * docs/superpowers/specs/2026-11-21-touch-frame-design.md).
 *
 *   node tools/webclient-tests/touch-frame.js
 *
 * Zero dependencies. Static analysis of webclient-pure.html, dashboard.css
 * and dashboard.js; it does not run the UI.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
const html = fs.readFileSync(path.join(root, 'webclient-pure.html'), 'utf8');
const css = fs.readFileSync(path.join(root, 'static', 'css', 'dashboard.css'), 'utf8');
const dash = fs.readFileSync(path.join(root, 'static', 'js', 'dashboard.js'), 'utf8');

let fails = 0;
function check(ok, what) {
  console.log((ok ? '  OK        ' : '  FAIL      ') + what);
  if (!ok) fails++;
}

// The page.
const feedAt = html.indexOf('id="panel-feed"');
const slotAt = html.indexOf('id="touch-slot"');
const deckAt = html.indexOf('id="touch-deck"');
const cmdAt = html.indexOf('id="panel-cmd"');
check(feedAt > 0 && slotAt > feedAt && cmdAt > slotAt, 'the slot sits between the game text and the command box');
check(deckAt > slotAt && deckAt < cmdAt, 'the deck is inside the slot');
check(/<div id="touch-slot" hidden>/.test(html), 'the slot starts hidden');
check(/<div id="touch-deck" aria-hidden="true"><\/div>/.test(html), 'the deck starts empty and hidden from screen readers');
const scriptAt = html.indexOf('/static/js/touchdeck.js');
const wiringAt = html.indexOf("querySelectorAll('input[name=\"touch-controls\"]')");
check(scriptAt > 0 && wiringAt > scriptAt, 'touchdeck.js loads before the Settings code reads it');
check(/<fieldset id="settings-touch" aria-describedby="touch-note">/.test(html), 'Touch controls is its own Settings section');
['auto', 'always', 'never'].forEach(function (v) {
  check(html.indexOf('<input type="radio" name="touch-controls" value="' + v + '">') > 0, 'Settings offers ' + v);
});

// The stylesheet.
check(/#panel-feed:has\(~ #panel-cmd\)/.test(css), 'the game window joins the command box with the slot between them');
check(!/#panel-feed(\.sight-\w+)?\s*\+\s*#panel-cmd/.test(css), 'no rule still needs the command box right after the game window');
['dark', 'shapes', 'faces', 'dazzled'].forEach(function (b) {
  check(new RegExp('#panel-feed\\.sight-' + b + '\\s*~ #touch-slot \\{ border-color: var\\(--sight-' + b + '\\); \\}').test(css),
    'the slot takes the ' + b + ' sight colour');
});
check(/#touch-slot\[hidden\] \{ display: none; \}/.test(css), 'an empty slot takes no room');
check(/#touch-slot #dash-drawer \{[^}]*position: static;/.test(css), 'an open tab in the slot is part of the stack, not floating');
check(/#touch-slot #dash-drawer:not\(\.open\) \{ display: none; \}/.test(css), 'a closed tab takes no room in the slot');
check(/#touch-slot\[data-shows="tab"\] #touch-deck \{ display: none; \}/.test(css), 'an open tab takes the deck\'s place');
check(/#touch-slot\.deck-on #dash-drawer\.open \{ min-height: var\(--deck-h\); \}/.test(css), 'a short tab keeps the deck\'s height');

// dashboard.js.
check(/_placeDrawer\(true\)/.test(dash) && /_placeDrawer\(false\)/.test(dash), 'a phone moves its tab card into the slot and back out');
check(/applyDeck: function/.test(dash), 'dashboard.js applies the deck');

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails === 0 ? 0 : 1);
```

- [ ] **Step 2: Run it to see the page checks fail**

Run: `node tools/webclient-tests/touch-frame.js`
Expected: FAIL lines for the page, stylesheet and dashboard.js checks; exit 1.

- [ ] **Step 3: Load `touchdeck.js` in the head**

In `_datafiles/html/public/webclient-pure.html`, after the line

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/layoutpick.js?v={{ .ASSET_VERSION }}"></script>
```

add

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/touchdeck.js?v={{ .ASSET_VERSION }}"></script>
```

- [ ] **Step 4: Add the Settings section**

After the Appearance `</fieldset>` (the one closing `settings-appearance`,
just before `</div>` that closes `#settings-menu`), add:

```html
          <!-- Touch controls (touch frame plan 1a): buttons above the command
               box, on this device. Automatic stays off until the deck has
               buttons (owner, 2026-11-21); Always shows the empty frame now. -->
          <fieldset id="settings-touch" aria-describedby="touch-note">
            <legend>Touch controls</legend>
            <p class="settings-note" id="touch-note">Buttons above the command box for moving and acting. Automatic turns them on for phones and tablets once they have buttons. Always shows them now.</p>
            <label class="theme-choice"><input type="radio" name="touch-controls" value="auto"> Automatic</label>
            <label class="theme-choice"><input type="radio" name="touch-controls" value="always"> Always</label>
            <label class="theme-choice"><input type="radio" name="touch-controls" value="never"> Never</label>
          </fieldset>
```

- [ ] **Step 5: Add the slot between the game text and the command box**

In the center column, between `</section>` closing `#panel-feed` and
`<section class="dash-panel" id="panel-cmd" data-panel="cmd">`, add:

```html
        <!-- Touch frame (plan 1a): the slot under the game text holds the
             controls deck, or on a phone the open tab (dashboard.js moves
             #dash-drawer in). Hidden while it holds neither. -->
        <div id="touch-slot" hidden>
          <div id="touch-deck" aria-hidden="true"></div>
        </div>

```

- [ ] **Step 6: Wire the Settings radios**

In the inline script, right after the line `showThemeChoice();` that ends the
Appearance block (`:3827` today), add:

```js

        // Touch controls: Automatic / Always / Never, on this device
        // (touchdeck.js). A change applies at once (dashboard.js applyDeck).
        function touchStore() {
            try { return window.localStorage; } catch (e) { return null; }
        }
        function showTouchChoice() {
            const choice = window.TouchDeck ? TouchDeck.readPref(touchStore()) : 'auto';
            document.querySelectorAll('input[name="touch-controls"]').forEach(function (r) {
                r.checked = (r.value === choice);
            });
        }
        document.querySelectorAll('input[name="touch-controls"]').forEach(function (r) {
            r.addEventListener('change', function () {
                if (!r.checked || !window.TouchDeck) return;
                TouchDeck.writePref(touchStore(), r.value);
                if (window.Dashboard && Dashboard.applyDeck) Dashboard.applyDeck();
            });
        });
        // Another tab changed it.
        window.addEventListener('storage', function (e) {
            if (!window.TouchDeck || e.key !== TouchDeck.KEY) return;
            showTouchChoice();
            if (window.Dashboard && Dashboard.applyDeck) Dashboard.applyDeck();
        });
        showTouchChoice();
```

- [ ] **Step 7: Run the static test**

Run: `node tools/webclient-tests/touch-frame.js`
Expected: the page checks (slot, deck, script order, Settings) `OK`; the
stylesheet and dashboard.js checks still `FAIL` (Tasks 4 and 5).

- [ ] **Step 8: Commit**

```bash
git add tools/webclient-tests/touch-frame.js _datafiles/html/public/webclient-pure.html
git commit -F - <<'EOF'
feat(webclient): the touch slot and the Touch controls setting (#783)

Touch frame plan 1a, task 3: an empty, hidden slot between the game text
and the command box, and Settings > Touch controls (Automatic, Always,
Never) saved on the device.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: The stylesheet

**Files:**
- Modify: `_datafiles/html/public/static/css/dashboard.css` (`:179-194`, `:229-238`, a new block after `:238`, a new block after the drawer rules near `:1104`, a new rule inside or after `:1216-1223`)

- [ ] **Step 1: Let the sight rules reach past the slot**

Replace `:179-194` (from `#panel-feed,` through the last
`+ #panel-cmd { border-color: var(--sight-dazzled); }` line) with:

```css
#panel-feed,
#panel-feed ~ #touch-slot,
#panel-feed ~ #panel-cmd {
  --sight-dark:    #3a2d17;   /* near black, a darkened antique gold */
  --sight-shapes:  #7a6232;   /* dim, a muted gold */
  --sight-faces:   var(--antique-gold);
  --sight-dazzled: #f6e7b0;   /* bright, a pale hot gold */
  transition: border-color 0.4s ease;
}
#panel-feed.sight-dark    { border-color: var(--sight-dark); }
#panel-feed.sight-shapes  { border-color: var(--sight-shapes); }
#panel-feed.sight-faces   { border-color: var(--sight-faces); }
#panel-feed.sight-dazzled { border-color: var(--sight-dazzled); box-shadow: 0 0 6px var(--sight-dazzled); }
/* The touch slot (plan 1a) may sit between the game window and the command
   box, so these reach them with ~, not +. A popped-out game window leaves
   the column, and then none of them match, as before. */
#panel-feed.sight-dark    ~ #touch-slot { border-color: var(--sight-dark); }
#panel-feed.sight-shapes  ~ #touch-slot { border-color: var(--sight-shapes); }
#panel-feed.sight-faces   ~ #touch-slot { border-color: var(--sight-faces); }
#panel-feed.sight-dazzled ~ #touch-slot { border-color: var(--sight-dazzled); }
#panel-feed.sight-dark    ~ #panel-cmd { border-color: var(--sight-dark); }
#panel-feed.sight-shapes  ~ #panel-cmd { border-color: var(--sight-shapes); }
#panel-feed.sight-faces   ~ #panel-cmd { border-color: var(--sight-faces); }
#panel-feed.sight-dazzled ~ #panel-cmd { border-color: var(--sight-dazzled); }
```

- [ ] **Step 2: Join the frame through the slot**

Replace `:229-238` (`#panel-feed:has(+ #panel-cmd) {` through the closing
`}` of `#panel-feed + #panel-cmd {`) with:

```css
#panel-feed:has(~ #panel-cmd) {
  border-bottom-width: 0;
  border-bottom-left-radius: 0; border-bottom-right-radius: 0;
}
#panel-feed ~ #panel-cmd {
  margin-top: calc(-1 * var(--dash-gap, 6px));
  border-color: var(--sight-faces);
  border-top-left-radius: 0; border-top-right-radius: 0;
  transition: border-color 0.4s ease;
}
```

- [ ] **Step 3: Add the slot, deck and divider**

Directly after the block from Step 2, add:

```css

/* Touch frame (spec 2026-11-21, plan 1a). The slot between the game text
   and the command box holds the controls deck, or on a phone the open tab
   (dashboard.js moves #dash-drawer in), so an open tab pushes the game text
   up instead of covering its newest lines. It is part of the game window's
   frame: side borders in the sight colour, no gap above or below. Hidden
   while it holds neither. touchdeck.js picks what it shows (data-shows) and
   the deck's shape; dashboard.js sets the --deck-* variables. */
#touch-slot {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  min-height: 0;
  margin-top: calc(-1 * var(--dash-gap, 6px));
  border: 0 solid var(--sight-faces);
  border-left-width: 1px;
  border-right-width: 1px;
  background: var(--panel-bg);
  transition: border-color 0.4s ease;
}
#touch-slot[hidden] { display: none; }
#touch-deck {
  display: grid;
  grid-template-columns: repeat(var(--deck-cols, 6), var(--deck-btn, 48px));
  grid-auto-rows: var(--deck-btn, 48px);
  gap: var(--deck-gap, 8px);
  padding: var(--deck-pad, 8px);
  justify-content: center;
  background: var(--ink-deep);
  border-top: 2px solid var(--gold-rule);
}
#touch-slot[data-shows="tab"] #touch-deck { display: none; }
#touch-slot[data-shows="empty"] #touch-deck { display: none; }
/* An empty slot until the server sends buttons (spec 2 onward). */
.deck-slot {
  border: 1px dashed var(--brass-border);
  border-radius: 6px;
}
/* The divider between the deck and the command box: a wider gap and a line
   (roadmap fat-finger rules), only while the deck is on. */
#touch-slot.deck-on::after {
  content: "";
  flex: 0 0 12px;
  background: linear-gradient(var(--gold-rule), var(--gold-rule)) center / calc(100% - 40px) 2px no-repeat;
}
```

- [ ] **Step 4: Make the phone's tab card part of the stack**

After the line `#dash-drawer .ph-popout { display: none; }` (`:1104`), add:

```css

/* On a phone the tab card lives in the touch slot (dashboard.js
   _placeDrawer), so it is part of the game column's stack: the game text
   above it shrinks and stays on its newest line. It keeps its caps
   (--drawer-cap: 30%, the Map 45%, 30% of what is visible with the keyboard
   up) and every #dash-drawer rule inside it. Column and slim keep the
   floating card above. */
#touch-slot #dash-drawer {
  position: static;
  left: auto; right: auto; bottom: auto;
  z-index: auto;
  transform: none;
  visibility: visible;
  transition: none;
  border-top: 2px solid var(--gold-rule);
}
#touch-slot #dash-drawer:not(.open) { display: none; }
/* A tab shorter than the deck keeps the deck's height, so opening Health or
   Quests does not make the game text jump. */
#touch-slot.deck-on #dash-drawer.open { min-height: var(--deck-h); }
```

- [ ] **Step 5: The side column on a phone held sideways**

After the closing `}` of the block that starts
`@media (pointer: coarse) and (orientation: landscape) and (max-height: 500px) {`
(`:1216-1223`), add:

```css
/* A phone on its side with the deck on: the deck is a side column beside
   the game text and the command box (roadmap, Devices; four rows of three,
   touchdeck.js shape). No divider beside it. A tab still opens under the
   game text. */
@media (pointer: coarse) and (orientation: landscape) and (max-height: 500px) {
  #dashboard[data-mode="phone"] #dash-col-center:has(> #touch-slot[data-shows="deck"]) {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: minmax(0, 1fr) auto;
    column-gap: var(--dash-gap, 6px);
    row-gap: 0;
  }
  #dashboard[data-mode="phone"] #dash-col-center:has(> #touch-slot[data-shows="deck"]) > #panel-feed { grid-column: 1; grid-row: 1; }
  #dashboard[data-mode="phone"] #dash-col-center:has(> #touch-slot[data-shows="deck"]) > #panel-cmd { grid-column: 1; grid-row: 2; margin-top: 0; }
  #dashboard[data-mode="phone"] #dash-col-center:has(> #touch-slot[data-shows="deck"]) > #touch-slot {
    grid-column: 2; grid-row: 1 / span 2;
    align-self: start;
    margin-top: 0;
    border-width: 1px;
    border-radius: 6px;
  }
  #dashboard[data-mode="phone"] #touch-slot[data-shows="deck"]::after { display: none; }
}
```

- [ ] **Step 6: Run the stylesheet tests**

Run each:
```
node tools/webclient-tests/touch-frame.js
node tools/jstest/touch-targets.test.js
node tools/webclient-tests/sight-border.js
node tools/jstest/hotinput.test.js
```
Expected: `touch-frame.js` passes every check but the two dashboard.js ones;
`touch-targets.test.js` all `ok`, exit 0; `sight-border.js` and
`hotinput.test.js` pass unchanged.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/html/public/static/css/dashboard.css
git commit -F - <<'EOF'
feat(webclient): style the touch slot, deck and phone tab stack (#783)

Touch frame plan 1a, task 4. The sight colours reach the command box
past the slot; on a phone the tab card becomes part of the stack.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: `dashboard.js`, the tab into the slot and the deck applied

**Files:**
- Modify: `_datafiles/html/public/static/js/dashboard.js` (header comment `:1-14`; state `:151-161`; `init` `:166-216`; `_apply` `:358-388`; `_enterPhone`/`_exitPhone` `:502-503`; `_openDrawer` end `:640`; `_closeDrawer` end `:669`)

- [ ] **Step 1: Header comment**

Replace line 13 ` * wide-only feature.` with:

```js
 * wide-only feature.
 *
 * Touch frame (plan 1a, 2026-11-21): on a phone the tab card moves into the
 * touch slot under the game text (_placeDrawer), so an open tab pushes the
 * game up instead of covering its newest lines. applyDeck shows the controls
 * deck in the same slot when the Touch controls setting turns it on
 * (touchdeck.js).
```

- [ ] **Step 2: State**

After `_drawerClose: null, // the phone card's ✕ button` add:

```js
    _slot: null,        // #touch-slot, under the game text (touch frame plan 1a)
    _deck: null,        // #touch-deck, inside the slot
```

- [ ] **Step 3: `init` finds the slot and watches the keyboard**

After `this._buildPhone();` in `init`, add:

```js
      // The touch slot and its deck (plan 1a). Absent in a page without them.
      this._slot = document.getElementById("touch-slot");
      this._deck = document.getElementById("touch-deck");
```

After the `document.fonts.ready` block in `init` (before `// Folding, pop-out and the saved layout`), add:

```js
      // The on-screen keyboard covers the deck (webclient.html sets
      // keyboard-open on this page's root). Act only on a change of that
      // class; applyDeck never touches the root's classes, so this cannot loop.
      if (window.MutationObserver) {
        var kbd = document.documentElement.classList.contains("keyboard-open");
        new MutationObserver(function () {
          var now = document.documentElement.classList.contains("keyboard-open");
          if (now === kbd) return;
          kbd = now;
          self.applyDeck();
        }).observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
      }
```

- [ ] **Step 4: `_apply` applies the deck in both branches**

Replace

```js
      if (newMode === this.mode) { this._arrangeColumn(); return; }
```

with

```js
      if (newMode === this.mode) { this._arrangeColumn(); this.applyDeck(); return; }
```

and after the last line of `_apply`, `this._arrangeColumn();`, add
`this.applyDeck();` so the method ends:

```js
      if (newMode === "wide") this._repopAfterNarrow();
      this._arrangeColumn();
      this.applyDeck();
    },
```

- [ ] **Step 5: The phone moves its card into the slot**

Replace

```js
    _enterPhone:  function () { this._enterTabbed(); },
    _exitPhone:   function () { this._exitTabbed(); },
```

with

```js
    _enterPhone:  function () { this._enterTabbed(); this._placeDrawer(true); },
    _exitPhone:   function () { this._exitTabbed(); this._placeDrawer(false); },

    // The phone's tab card lives in the touch slot, under the game text, so
    // it pushes the game up (plan 1a); column and slim float it from the
    // page's body as before. Moving the element keeps every #dash-drawer
    // rule and the panel inside it.
    _placeDrawer: function (inSlot) {
      var drawer = this._drawer;
      if (!drawer) return;
      var dash = document.getElementById("dashboard");
      var home = inSlot && this._slot ? this._slot : (dash && dash.parentNode);
      if (home && drawer.parentNode !== home) home.appendChild(drawer);
    },
```

- [ ] **Step 6: Opening and closing a tab updates the slot**

At the end of `_openDrawer`, after `this._updateTabbarActive(name);`, add
`this.applyDeck();`. At the end of `_closeDrawer`, after
`this._updateTabbarActive(null);`, add `this.applyDeck();`.

- [ ] **Step 7: `applyDeck`**

Add this method right after `_placeDrawer`:

```js
    // What the touch slot shows and the deck's shape (touchdeck.js). Runs on
    // every layout pass, tab open or close, keyboard change and Settings
    // change. Never touches the root element's classes (the keyboard watch
    // in init observes them).
    applyDeck: function () {
      var TD = window.TouchDeck, VF = window.ViewportFit;
      var slot = this._slot, deck = this._deck;
      if (!TD || !slot || !deck) return;
      var coarse = !!(window.matchMedia && window.matchMedia("(pointer: coarse)").matches);
      var store = null;
      try { store = window.localStorage; } catch (e) { /* blocked */ }
      var on = TD.deckOn(TD.readPref(store), coarse);
      var keyboard = document.documentElement.classList.contains("keyboard-open");
      var shows = TD.slotShows({
        phone: this.mode === "phone",
        tabOpen: !!this._drawerActive,
        deckOn: on,
        keyboard: keyboard
      });
      if (slot.dataset.shows !== shows) slot.dataset.shows = shows;
      slot.hidden = shows === "empty";
      slot.classList.toggle("deck-on", on);
      if (!on) return;

      var sw = window.screen ? screen.width : 0, sh = window.screen ? screen.height : 0;
      var phone = !!(VF && VF.isPhone(sw, sh)) && coarse;
      var sideways = !!(VF && VF.isSidewaysPhone(sw, sh, VF.orientationType(window), coarse));
      var col = document.getElementById("dash-col-center");
      var boxW = (col ? col.clientWidth : window.innerWidth) - 2;   // the slot's side borders
      var boxH = col ? col.clientHeight : window.innerHeight;
      var s = TD.shape(boxW, boxH, phone && !sideways, sideways);
      deck.style.setProperty("--deck-cols", String(s.cols));
      deck.style.setProperty("--deck-btn", s.button + "px");
      deck.style.setProperty("--deck-gap", s.gap + "px");
      deck.style.setProperty("--deck-pad", s.pad + "px");
      // The deck's height with its 2px top rule: a short tab keeps it.
      slot.style.setProperty("--deck-h", (s.height + 2) + "px");
      var want = s.rows * s.cols;
      if (deck.childElementCount !== want) {
        while (deck.firstChild) deck.removeChild(deck.firstChild);
        for (var i = 0; i < want; i++) {
          var cell = document.createElement("span");
          cell.className = "deck-slot";
          deck.appendChild(cell);
        }
      }
    },
```

- [ ] **Step 8: Run every client test**

Run:
```
node tools/webclient-tests/touch-frame.js
node tools/jstest/dashboard-popout-layout.test.js
```
then the whole CI set from bash:
```bash
failed=0; for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" > /dev/null || { echo "FAIL $t"; failed=1; }; done; echo "failed=$failed"
```
Expected: `touch-frame.js` ALL CHECKS PASSED; `failed=0`. If
`dashboard-popout-layout.test.js` fails because its fake DOM lacks
something `applyDeck` or the observer uses, guard that use in `dashboard.js`
(the real page has it); do not edit the test to skip it.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/html/public/static/js/dashboard.js
git commit -F - <<'EOF'
feat(webclient): a phone's open tab pushes the game up; apply the deck (#783)

Touch frame plan 1a, task 5. On a phone the tab card moves into the
touch slot under the game text, so the newest line stays in view.
applyDeck shows the deck when Touch controls turns it on.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Device and touch emulation in `live-shot.js`

**Files:**
- Modify: `tools/webshot/live-shot.js` (header comment `:8-21`; session loop `:153-154`)

- [ ] **Step 1: Document the option**

In the header comment, after the line `//                    {"shot": "C:/.../a.png"} ] } ] }`, add:

```js
//
// A session may carry "emulate": {"width": 360, "height": 800, "dpr": 3,
// "mobile": true, "touch": true}. It sets the page's size, the screen's size
// (what ViewportFit.isPhone reads) and touch input before the page loads.
// Check "matchMedia('(pointer: coarse)').matches" with an "eval" step: the
// touch layout depends on it.
```

- [ ] **Step 2: Apply it before each session's page load**

Replace

```js
    for (const s of cfg.sessions) {
      await dt.send('Page.navigate', { url: cfg.url });
```

with

```js
    for (const s of cfg.sessions) {
      if (s.emulate) {
        const e = s.emulate;
        await dt.send('Emulation.setDeviceMetricsOverride', {
          width: e.width, height: e.height, deviceScaleFactor: e.dpr || 1,
          mobile: !!e.mobile, screenWidth: e.width, screenHeight: e.height,
        });
        await dt.send('Emulation.setTouchEmulationEnabled', { enabled: !!e.touch, maxTouchPoints: e.touch ? 5 : 0 });
      } else {
        await dt.send('Emulation.clearDeviceMetricsOverride');
        await dt.send('Emulation.setTouchEmulationEnabled', { enabled: false });
      }
      await dt.send('Page.navigate', { url: cfg.url });
```

- [ ] **Step 3: Syntax check and commit**

Run: `node --check tools/webshot/live-shot.js`
Expected: no output, exit 0.

```bash
git add tools/webshot/live-shot.js
git commit -F - <<'EOF'
tools(webshot): optional per-session device and touch emulation (#783)

Touch frame plan 1a, task 6, for the headless size matrix.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Headless size matrix (controller, not a subagent)

**Files:** none in the repo. Everything goes in the session scratchpad.

- [ ] **Step 1: Boot a test server**

Follow the `dogmud-shipping` skill's boot check: a detached worktree of this
branch, private ports, the instance-save wipe, and `Playtest.ProfilesManifest`
in its override config so seeded characters and `creds.json` exist. Note the
server's PID; kill only that PID at the end. Never use the owner's server.

- [ ] **Step 2: Confirm the emulation gives a touch phone**

A `shots.json` with one session at `{"width":360,"height":800,"dpr":3,"mobile":true,"touch":true}` and an eval step:

```json
{"eval": "JSON.stringify({coarse: matchMedia('(pointer: coarse)').matches, mode: document.getElementById('dashboard').dataset.mode, sw: screen.width})"}
```

Expected: `coarse: true`, `mode: "phone"`, `sw: 360`. If `coarse` is false,
stop and report: every touch number below depends on it.

- [ ] **Step 3: The newest line, before and after**

For the S21 size, with no deck: open Comms, say a line, and measure whether
the tab covers the game window:

```json
{"eval": "Dashboard._openDrawer('chat'), true"},
{"send": "say the ferry is late", "after": 1500},
{"eval": "JSON.stringify({feedBottom: Math.round(document.getElementById('panel-feed').getBoundingClientRect().bottom), tabTop: Math.round(document.getElementById('dash-drawer').getBoundingClientRect().top), rows: term.rows})"},
{"shot": "<scratch>/s21-comms-after.png"}
```

Run the same against a master worktree (`<scratch>/s21-comms-before.png`).
Expected: on master `feedBottom > tabTop` (covered); on this branch
`feedBottom <= tabTop + 1` (pushed up), and the last terminal line (`tail`
step) is the `say` line.

- [ ] **Step 4: The matrix with the deck on**

One session per size in the matrix (375x548, 360x800, 412x915, 320x640,
800x360, 820x1180, 1180x820, 690x840, 1366x768; touch on for all but the
laptop), each with:

```json
{"eval": "localStorage.setItem('dogmud.touchControls','always'), Dashboard.applyDeck(), true"},
{"wait": 500},
{"eval": "JSON.stringify({rows: term.rows, slot: Math.round(document.getElementById('touch-slot').getBoundingClientRect().height), cols: getComputedStyle(document.getElementById('touch-deck')).getPropertyValue('--deck-cols'), btn: getComputedStyle(document.getElementById('touch-deck')).getPropertyValue('--deck-btn'), scrollW: document.documentElement.scrollWidth, w: innerWidth})"},
{"shot": "<scratch>/<size>-deck.png"},
{"eval": "Dashboard._openDrawer('map'), true"},
{"wait": 500},
{"eval": "JSON.stringify({rowsMap: term.rows})"},
{"shot": "<scratch>/<size>-map.png"},
{"eval": "localStorage.setItem('dogmud.touchControls','auto'), Dashboard._closeDrawer(), Dashboard.applyDeck(), true"}
```

Expected, and record the actual numbers in the PR body:
- `scrollW <= w` at every size (no sideways scroll).
- `btn` 48px everywhere except 320 wide and the sideways phone (44px).
- `cols` 6 on 360, 375, 412 and 320; 14 on the upright iPad; 11 on the foldable; 3 on the sideways phone.
- 360x800: `rows` about 20 (spec estimate); `rowsMap` about 9.
- 375x548: `rows` at least about 9.
- Look at every shot: the empty slots outlined, the divider under the deck, the frame joined (no gap) from game text through deck to command box, and on the sideways phone the deck beside the game.

- [ ] **Step 5: Desktop unchanged**

At 1366x768 and 1920x1080 with no emulation, shoot this branch and master
with the deck at Automatic. Expected: the pair looks the same, and
`document.getElementById('touch-slot').hidden === true`.

- [ ] **Step 6: Stop the server**

Kill the test server by the PID noted in Step 1. Remove the worktrees with
`git worktree remove <path>`.

If a number misses, the frame has a bug: fix it in the task that owns it
(slot sizing in Task 4, slot content in Task 5), re-run that task's tests,
and repeat this task.

---

### Task 8: Docs and the owner's check

**Files:**
- Modify: `docs/README.md`

- [ ] **Step 1: README rows**

In `docs/README.md`:
- In the `../tools/jstest/` row (`:109`), append before its closing ` |`:
  `. Added 2026-11-21 (touch frame plan 1a): \`touchdeck.test.js\` (covers [\`../_datafiles/html/public/static/js/touchdeck.js\`](../_datafiles/html/public/static/js/touchdeck.js): the Touch controls setting, the deck switched off under Automatic until plan 2b, its shape per width, what the slot under the game text shows), \`touch-targets.test.js\` (guard 6 of the touch arc: every deck slot 48px with 8px gaps across the size matrix, 44px and 5px only where 48 cannot fit, and the stylesheet sized from touchdeck.js)`
- In the `../tools/webclient-tests/` row (`:104`), append before its closing ` |`:
  `, and \`touch-frame.js\` (the touch slot between the game text and the command box, the Touch controls setting, the sight colours reaching past the slot, and the phone's tab card in the slot; touch frame plan 1a)`
- In the `../tools/webshot/` row (`:105`), append: ` Sessions may set device and touch emulation (touch frame plan 1a).`
- After the `longpress.js` row (`:341`), add:
  `| [\`../_datafiles/html/public/static/js/touchdeck.js\`](../_datafiles/html/public/static/js/touchdeck.js) | Touch frame plan 1a: the Touch controls setting (Automatic, Always, Never, on the device), whether the deck shows, its rows, columns and button size for the room it has, and what the slot under the game text holds (the deck, a phone's open tab, or nothing). Tested by \`tools/jstest/touchdeck.test.js\` and \`touch-targets.test.js\` |`
- With the other open plans (before the `2026-11-17-screen-reader-task0` plan rows), add:
  `| [\`superpowers/plans/2026-11-21-touch-frame-1a.md\`](superpowers/plans/2026-11-21-touch-frame-1a.md) | Touch frame plan 1a in eight tasks: \`touchdeck.js\` and its tests, guard 6, the slot and the Touch controls setting, the stylesheet (sight colours past the slot, the phone tab card in the stack, the sideways side column), \`dashboard.js\` (the card into the slot, \`applyDeck\`), device emulation in \`live-shot.js\`, the headless size matrix, and the owner's check |`

- [ ] **Step 2: Commit**

```bash
git add docs/README.md
git commit -F - <<'EOF'
docs: list touch frame plan 1a's files (#783)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

- [ ] **Step 3: The owner's check (after the PR is green, before merge)**

Give the owner and Malia these steps, ordinary play only, on the Pixel 8 and
the Galaxy S21 against a local test server they can reach (or after the
owner deploys):

1. Enter the game. Open Comms. Type `say hello`. Your line shows in the
   game text just above the Comms tab, not hidden behind it.
2. Open the Map, then Health. The game text moves up for the Map and does
   not jump for Health.
3. Settings, Touch controls: pick **Always**. Empty outlined boxes appear
   above the command box, two rows of six, with a line between them and the
   command box. Open a tab: it takes the boxes' place. Close it: the boxes
   come back.
4. Turn the phone on its side: the boxes move beside the game text.
5. Pick **Automatic**: the boxes go away.

Record the result on #834.

---

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| The existing `#dash-drawer` element moves into the slot on phones, instead of a new tab holder | Every `#dash-drawer` rule (caps, fold and close buttons, Inventory and Automation sizing) and every test that reads them keeps working | A separate holder with the rules duplicated |
| The fixed tab bar and its padding stay as they are; only the tab card joins the stack | The tab bar never covered game text; the padding already reserves its room | Move the tab bar into the column too |
| The sight rules switch from `+` to `~` so the command box keeps its sight colour with the slot between | `~` matches the same elements as before when the slot is hidden, and still nothing when the game window is popped out | Join the frame by a class set from script |
| The slot's side borders carry the sight colour, so game text, deck and command box read as one frame | The spec mock and the 11-19 mocks draw them as one card | A separate deck card with its own border |
| With the keyboard up, the deck hides and an open phone tab stays (today's 30% cap) until plan 1b | The roadmap says the keyboard covers the deck; the tab rule is 1b's | Ship 1a and 1b together |
| The slot-to-position order for the sideways side column is not built in 1a | The slots are empty and identical; spec 3 defines which button goes where | Build the order now |
| The sideways phone keeps its session bar and tab bar in 1a | The spec says they fold away, but nothing designed brings them back, and Settings (to turn the deck off) lives on the session bar. **Needs a ruling before plan 2c** | Fold them away now and design a way back |
| The Settings section is visible to every player, with a note that the buttons arrive later | A hidden setting cannot be turned on by Malia without admin steps | Hide the section until plan 2b |
| The deck's empty slots are `aria-hidden` spans, not buttons | Nothing to press yet; screen readers should not hear twelve empty buttons | Real disabled buttons |
| `live-shot.js` gets emulation per session, not per run | One run covers the whole matrix | Separate runs per size |
| Guard 6 derives each size's box from `LayoutPick.pick` and the session and tab bar heights, not from a browser | Runs in CI with no browser; Task 7 measures the real thing | Measure in a headless browser in CI |
