# Touch arc plan 2c: the Map tab's T and the sideways Menu (#781, #938) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On a phone, the Map tab carries the same movement T and exits button as the deck, beside the map. On a phone held sideways, the session bar and tab bar fold away and a Menu button in the deck's side column brings them back.

**Architecture:** Everything is client side; the server already sends what is needed (plan 2a `Room.Info`). `touchdeck.js` (pure) gains a Map shape, a Menu cell and the bars state. `dashboard.js` draws a second button grid, `#map-deck`, from the same model, level and tap state as the deck, and sets `data-bars` on `#dashboard` for the fold. `dashboard.css` styles both grids through one shared class, `.deck-grid`, and folds the bars.

**Tech Stack:** plain browser JS (no build), CSS, Node jstests (`tools/jstest`), headless Edge (`tools/webshot/live-shot.js`).

**Spec:** `docs/superpowers/specs/2026-11-22-moving-by-touch-design.md`, section "2c: the Map tab and the sideways phone". **Mockup:** `docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html`, frames 7 (Map tab) and 9 (sideways). **Ruling:** #938, owner 2026-11-22, option A.

**Base:** branch `feature/moving-by-touch-2b` at `b9166fd67` (PR #965, merged 2026-11-27 as `6cb8e62ed`). Plan 2c builds on 2b's code. Branch `feature/moving-by-touch-2c` from master.

---

## Facts verified against source (feature/moving-by-touch-2b, b9166fd67)

| # | Fact | Where |
|---|---|---|
| F1 | `shape(boxW, boxH, phone, sideways)` returns `make(rows, cols, btn, gap, side)`: `{rows, cols, button, gap, pad, side, width, height}`. Sideways: 4 rows by 3 columns (`SIDE_ROWS`, `SIDE_COLS`), 48px if `span(4, 48, 8) <= boxH`, else 44px with a 5px gap | `static/js/touchdeck.js:65-93` |
| F2 | `cells(model, s, level, page)` fills `s.rows * s.cols` slots with `{kind:'empty'}`, returns them all empty when `!model.ready`. The exits level puts Back at 0, More at `n - 1` when `others.length > n - 1`, `per = more ? n - 2 : n - 1`. The exits button goes at index 3 when `s.cols >= 4`, else at `2 * s.cols` | `touchdeck.js:249-282` |
| F3 | `T_PLACE`: up [0,0], north [0,1], down [0,2], west [1,0], south [1,1], east [1,2], placed at `row * s.cols + col` | `touchdeck.js:150`, `:270` |
| F4 | `tap(st, c, now)` returns `{level, page, guardUntil, send, typed}`; kinds `move exit other back more dark`. `DOUBLE_TAP_MS` 500 | `touchdeck.js:237`, `:297-326` |
| F5 | `ICONS` (stairs-up, stairs-down, lock, eye) drawn through `lineIcon(doc, box, paths, cls)`; `classOf`, `ariaOf`, `disabledOf`, `render(deck, list, doc)`, `focusAfter(list, cell, levelWas, level)` | `touchdeck.js:334-486` |
| F6 | Exports end at `touchdeck.js:488-497` (`shape`, `cells`, `tap`, `render`, `focusAfter`, `BTN`, `GAP`, `SMALL_BTN`, `SMALL_GAP`, `PAD`, ...) | `touchdeck.js:488` |
| F7 | `Dashboard` state for the deck: `_moves`, `_movesNum`, `_deckMode`, `_deckState {level, page, guardUntil}`, `_deckCells`, `_deckSig`, `_deckDrawnLevel`, `_deckPress`, `_deckType` | `static/js/dashboard.js:164-172` |
| F8 | `init` finds `#touch-slot`, `#touch-deck` and adds one click handler that calls `_deckTap(Number(data-cell))` | `dashboard.js:193-203` |
| F9 | `applyDeck` sets `slot.dataset.shows` from `TD.slotShows`, toggles `deck-on`, returns early when the deck is off, computes `phone`, `sideways` (`VF.isSidewaysPhone`), `s = TD.shape(boxW, boxH, phone && !sideways, sideways)`, sets `--deck-cols/-btn/-gap/-pad` on the deck and `--deck-h` on the slot, `model = TD.moves(...)`, `cells = TD.cells(...)`, toggles `deck-small`, `deck-dark`, redraws when `JSON.stringify(cells)` changes and hands the focus on with `focusAfter` | `dashboard.js:638-705` |
| F10 | `_deckTap(i)` reads `this._deckCells[i]`, runs `TD.tap`, sends `r.send` through `_deckPress` and `r.typed` through `_deckType`, then `applyDeck()` | `dashboard.js:739-747` |
| F11 | The phone's tab bar: `makeTabBar("gtabs-phone", onPick)` inside `#dash-tabbar` (appended to `#dashboard`'s parent); each tab is `button.gtab` with `data-panel`. `onPick` opens or closes the drawer | `dashboard.js:64-90`, `:749-770` |
| F12 | `_openDrawer(name)` moves `#panel-<name>` into the drawer, sets `_drawerActive`, calls `applyDeck()`; `_closeDrawer()` returns it, clears `_drawerActive`, calls `applyDeck()`. In phone mode the drawer lives in `#touch-slot` (`_placeDrawer(true)`) | `dashboard.js:619-632`, `:842-889`, `:941-960` |
| F13 | `applyDeck` never touches the root element's classes (a MutationObserver on `<html>` class watches `keyboard-open`) | `dashboard.js:235-246`, `:636-637` |
| F14 | Page: `#touch-slot` holds `<div id="touch-deck" role="group" aria-label="Movement"></div>`; `#panel-map .map-body` holds `#map-render`, `#map-controls`, `#map-where`, `#map-legend` in that order | `webclient-pure.html:333-335`, `:354-385` |
| F15 | Session bar: `#dash-session` holds `#sess-name`, `#btn-session`, `#sess-confirm` (Leave Game's question, `style="display:none"` until asked), `#btn-settings`, `#settings-menu` (`hidden` until open), `#btn-fullscreen`, `#btn-reset-layout` | `webclient-pure.html:258-320` |
| F16 | `#dashboard` grid: rows `auto minmax(0, 1fr)`; phone mode areas `"sess" "center"`, `padding-bottom: calc(6px + var(--tabbar-h))` | `static/css/dashboard.css:21-25`, `:929-937` |
| F17 | `#touch-deck` rule holds the grid (`repeat(var(--deck-cols, 6), var(--deck-btn, 48px))`, gap, padding, background, `border-top`) and the pitch-dark colour tokens; `#touch-deck.deck-small .deck-name` and `#touch-deck.deck-dark` are id scoped; `.deck-btn` and its kinds are not | `dashboard.css:268-291`, `:363`, `:368` |
| F18 | The sideways side column: `@media (pointer: coarse) and (orientation: landscape) and (max-height: 500px)`, grid on `#dash-col-center:has(> #touch-slot[data-shows="deck"])` | `dashboard.css:1421-1438` |
| F19 | The Map card may take 45% of the screen on a phone (`#dash-drawer:has(#panel-map)`, `--drawer-cap: max(80px, 45dvh)`) | `dashboard.css:1398-1402` |
| F20 | `#panel-map #map-render`: `flex: 0 1 auto; aspect-ratio: 320 / 300; width: 100%` inside a column flex `.map-body`; on touch the map has no button row | `dashboard.css:396-410`, `:428-432` |
| F21 | The input-queue grey-out lists `body.input-queue-full #touch-deck` twice | `dashboard.css:2019-2026` |
| F22 | Tests that read the deck CSS or markup by its id: `touch-targets.test.js:80` (`/#touch-deck \{/`), `:98` (`#touch-deck\.deck-small \.deck-name`); `webclient-input-queue.test.js:64` (`body.input-queue-full #touch-deck`); `tools/webclient-tests/touch-frame.js:35` (the exact `#touch-deck` markup), `:54` | as listed |
| F23 | `touch-targets.test.js` takes the deck box height as `h - 2 * PAD - SESSION - TABBAR` (64 each) for every size, sideways included | `touch-targets.test.js:28`, `:63` |
| F24 | `dashboard-deck.test.js` reads `D._deckCells` (lines 114, 174, 180) and calls `D._deckTap(i)`; its fake DOM's `getElementById` knows `dashboard` (returns `{ parentNode }` only) and `dash-col-center` | `dashboard-deck.test.js:59-75`, `:112-115` |
| F25 | `dashboard.js` must create no SVG itself (`/createElementNS/` is checked absent); icons go through `TouchDeck.lineIcon` | `dashboard-deck.test.js:204` |
| F26 | Sizes measured on 2b today (headless, Tower Top): 800x360 sideways side column is 44px with 5px gaps today because the bars take its height | PR #965 body |

## Owner rulings this plan rests on

- **#938 option A (2026-11-22):** on a sideways phone the session bar and tab bar fold away; the side column carries one **Menu** button that brings both bars back over the game until the player picks something or taps away. The sideways toast stays.
- **Spec 2c:** the Map tab carries the same T and exits button beside the map (about 160px; the map keeps its size as far as the buttons allow); same ids, states and marks; Other exits opens in place of the buttons and the map stays; shown only when the deck would show. Reached sideways through Menu, then Map.
- Plan 2b rulings carry over: brass buttons (R1), every enterable exit gets a button (R2), no client-side text.

## Decisions taken without asking

| # | Decision | Why | If wrong |
|---|---|---|---|
| D1 | The Map tab's buttons are 3 rows of 3 upright (the T, the exits button under it, as frame 7) and 2 rows of 4 sideways (the exits button beside the T) | A sideways Map card is 45% of about 360px: about 130px under its title, too short for three rows (176px) | `mapShape` changes; the tests name both shapes |
| D2 | The deck and the Map tab's buttons share one level and tap state (`_deckState`) | Only one shows at a time (an open tab replaces the deck in the slot); opening exits on one and then the other should agree | Give the map its own `_mapState` |
| D3 | Menu takes the side column's last slot in every state: before the first `Room.Info`, in the dark, and in the exits list (More moves one slot left) | Leave Game and Settings must be reachable while disconnected or between rooms; Menu never moves under the thumb | `cells` reserves nothing and Menu shows only with the T |
| D4 | The bars fold only while the deck is on. With Touch controls on Never there is no deck, so no Menu, and the bars stay | Otherwise Settings would be unreachable | none expected |
| D5 | "Picks something": a click on any button in the bars folds them, unless that click left one of the bars' own dialogs open (the Settings menu, Leave Game's question). Settings stays until it is closed; the question stays until answered | Folding while Settings is open would hide the menu the player just opened | A narrower list of buttons |
| D6 | While shown, the session bar sits fixed over the top of the game and the tab bar in its usual place at the bottom; the game text is not resized | "Over the game" (ruling); resizing would reflow the terminal twice per Menu tap | Push the game instead |
| D7 | Escape folds the bars and puts the focus back on Menu | Keyboard and screen reader users need a way out that is not a tap | none expected |
| D8 | Both grids share a class `.deck-grid`; the rules now scoped to `#touch-deck` (grid, tokens, small, dark, grey-out) move to it. `#touch-deck` keeps only its top rule | Unify rule: a second copy of the grid CSS for `#map-deck` would drift | none expected |
| D9 | The Map tab's buttons show in phone mode only (`Dashboard.mode === "phone"`), which includes an upright tablet | Spec: "On a phone, an open tab covers the deck". In column and slim the Map is not in the slot | Show them wherever the deck is on |
| D10 | A Menu tap sets no tap hold | The bars appear outside the deck; nothing new lands under the finger in the deck | Add `guardUntil` |
| D11 | The Map tab's buttons take the deck's button size (48px, or 44px where the deck is small) | One size rule (guard 6); the Map column is 160px at 48px as frame 7 | none expected |
| D12 | Menu draws three lines (a line icon in `ICONS`) over the word "Menu", like frame 9's ☰, as a stroke path, never an emoji (brand) | Frame 9; brand rule on icons | Another icon |

## File map

| File | Change |
|---|---|
| `_datafiles/html/public/static/js/touchdeck.js` | `make` takes `menu`; sideways `shape` sets it; `mapShape`; `cells` places Menu; `tap` returns `menu`; Menu's icon, class, words; `barsState`, `barsStayAfterPick` |
| `_datafiles/html/public/static/js/dashboard.js` | deck memos replace `_deckCells/_deckSig/_deckDrawnLevel`; `_sizeDeck`, `_drawDeck`; `#map-deck` drawn in `applyDeck`; `_deckTap(i, which)`; the bars (`_barsShown`, `_setBars`, `_showBars`, three listeners) |
| `_datafiles/html/public/webclient-pure.html` | `#touch-deck` gets `class="deck-grid"`; new `#map-deck` after `#map-render` |
| `_datafiles/html/public/static/css/dashboard.css` | `.deck-grid` rules; `#map-deck` beside the map; `.deck-menu`; `data-bars` fold and show |
| `tools/jstest/touchdeck-menu-map.test.js` | new: Menu cells, `mapShape`, `tap`, bars helpers, render of Menu |
| `tools/jstest/dashboard-deck.test.js` | memo names; Map tab buttons; the bars |
| `tools/jstest/touch-targets.test.js` | `.deck-grid` regexes; sideways box without the bars; Map shapes fit their card |
| `tools/jstest/webclient-input-queue.test.js` | grey-out reads `.deck-grid` |
| `tools/webclient-tests/touch-frame.js` | the deck markup with its class; `#map-deck` markup |
| `docs/PATCH_NOTES.md`, `docs/README.md`, `docs/BRAND_STANDARDS.md` (deck section), the spec's 2c heading, `docs/roadmaps/TOUCH_ARC_ROADMAP.md` row 2 | docs |

## Task list

1. `touchdeck.js`: Menu, the Map shape, the bars helpers (TDD)
2. Page markup and the shared `.deck-grid` CSS
3. `dashboard.js`: one draw path for two grids, the Map tab's buttons
4. `dashboard.js` and CSS: the bars fold and Menu
5. Guard 6 and the static checks
6. Docs
7. Full gate
8. HEADLESS: size matrix, Map tab and Menu (controller)
9. OWNER'S CHECK: Malia's S21 (#834)
10. PR

---

### Task 1: `touchdeck.js`: Menu, the Map shape, the bars helpers

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js`
- Create: `tools/jstest/touchdeck-menu-map.test.js`

- [ ] **Step 1: Write the failing test**

Create `tools/jstest/touchdeck-menu-map.test.js`:

```js
// touchdeck-menu-map.test.js: touch arc plan 2c (#781, #938). The sideways
// side column's Menu button, the Map tab's button shape, and the bars a
// sideways phone folds away. Pure functions of touchdeck.js.
//
// Run:  node tools/jstest/touchdeck-menu-map.test.js
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

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function e(kind, id) { return { num: 1, dx: 0, dy: 0, dz: 0, details: [], kind: kind, id: id }; }
function room(v2, extra) {
    var r = { num: 5243, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}
function kinds(list) { return list.map(function (c) { return c.kind === 'move' ? c.dir : c.kind; }); }
var tower = room({ west: e('compass', 'm4'), shrine: e('named', 'x1'), ledge: e('named', 'x2') });

// --- shapes ------------------------------------------------------------------

var SIDE = TD.shape(0, 300, true, true);
var PHONE = TD.shape(344, 600, true, false);
var SMALL = TD.shape(300, 600, true, false);
check('the side column carries Menu', SIDE.menu, true);
check('an upright deck does not', PHONE.menu, false);
check('a tablet deck does not', TD.shape(788, 1000, false, false).menu, false);

var mapUp = TD.mapShape(PHONE, false);
check('the Map tab upright: 3 rows of 3 at the deck size (frame 7, D1)',
    [mapUp.rows, mapUp.cols, mapUp.button, mapUp.gap, mapUp.menu], [3, 3, 48, 8, false]);
check('the Map column is 160px wide at 48px, as frame 7', mapUp.width, 160);
var mapSide = TD.mapShape(SIDE, true);
check('the Map tab sideways: 2 rows of 4 (D1)', [mapSide.rows, mapSide.cols], [2, 4]);
check('the Map tab takes the small size with a small deck (D11)',
    [TD.mapShape(SMALL, false).button, TD.mapShape(SMALL, false).gap], [44, 5]);

// --- Menu in the side column (D3) ------------------------------------------

var top = TD.cells(TD.moves(tower, ''), SIDE, 'top', 0);
check('frame 9: the T, the exits button under it, Menu bottom right',
    kinds(top), ['up', 'north', 'down', 'west', 'south', 'east', 'other', 'empty', 'empty', 'empty', 'empty', 'menu']);
check('before the first Room.Info only Menu shows',
    kinds(TD.cells(TD.moves(null, ''), SIDE, 'top', 0)), ['empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'menu']);
check('in the dark Menu stays',
    kinds(TD.cells(TD.moves(room({}, { dark: true }), ''), SIDE, 'top', 0))[11], 'menu');
check('upright, no Menu anywhere', kinds(TD.cells(TD.moves(tower, ''), PHONE, 'top', 0)).indexOf('menu'), -1);

// The exits list in the side column: 11 slots, Back first, Menu kept last.
var many = {};
'abcdefghijkl'.split('').forEach(function (ch, i) { many['door' + ch] = e('named', 'x' + i); });
var big = TD.moves(room(many), '');
var page0 = kinds(TD.cells(big, SIDE, 'exits', 0));
check('a long list: Back, nine exits, More, Menu', page0,
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'more', 'menu']);
var page1 = kinds(TD.cells(big, SIDE, 'exits', 1));
check('page two: Back, the last three, More, Menu', page1,
    ['back', 'exit', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'more', 'menu']);
var ten = {};
'abcdefghij'.split('').forEach(function (ch, i) { ten['door' + ch] = e('named', 'x' + i); });
check('ten exits fit: no More', kinds(TD.cells(TD.moves(room(ten), ''), SIDE, 'exits', 0)),
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'menu']);
check('the upright deck keeps More in its last slot (plan 2b)',
    kinds(TD.cells(big, PHONE, 'exits', 0)).slice(-1), ['more']);

// --- the Map tab's cells -------------------------------------------------------

check('Map upright: the T, the exits button under it, two empty slots',
    kinds(TD.cells(TD.moves(tower, ''), mapUp, 'top', 0)),
    ['up', 'north', 'down', 'west', 'south', 'east', 'other', 'empty', 'empty']);
check('Map sideways: the exits button beside the T',
    kinds(TD.cells(TD.moves(tower, ''), mapSide, 'top', 0)),
    ['up', 'north', 'down', 'other', 'west', 'south', 'east', 'empty']);
check('Map upright, exits: Back and the list, the map stays (the list replaces only the buttons)',
    kinds(TD.cells(TD.moves(tower, ''), mapUp, 'exits', 0)),
    ['back', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty']);

// --- a Menu tap ----------------------------------------------------------------

var st = { level: 'top', page: 0, guardUntil: 0 };
var r = TD.tap(st, { kind: 'menu' }, 1000);
check('a Menu tap asks for the bars and sends nothing', [r.menu, r.send, r.typed, r.level], [true, '', '', 'top']);
check('a Menu tap sets no hold (D10)', r.guardUntil, 0);
check('any other tap does not ask for the bars', TD.tap(st, { kind: 'back' }, 1000).menu, false);
check('a Menu tap inside a hold is ignored like any tap', TD.tap({ level: 'top', page: 0, guardUntil: 2000 }, { kind: 'menu' }, 1000).menu, false);

// --- the bars --------------------------------------------------------------------

check('no fold: the bars as always', TD.barsState(false, false), '');
check('no fold even if asked', TD.barsState(false, true), '');
check('folded', TD.barsState(true, false), 'folded');
check('shown by Menu', TD.barsState(true, true), 'shown');
check('a pick folds them', TD.barsStayAfterPick(false, false), false);
check('Settings open: they stay (D5)', TD.barsStayAfterPick(true, false), true);
check('Leave Game asking: they stay (D5)', TD.barsStayAfterPick(false, true), true);

// --- drawing Menu ----------------------------------------------------------------

function El(tag) { this.tagName = tag; this.children = []; this.attrs = {}; this.className = ''; this.textContent = ''; this.disabled = false; }
El.prototype.appendChild = function (c) { this.children.push(c); return c; };
El.prototype.removeChild = function (c) { this.children.splice(this.children.indexOf(c), 1); return c; };
El.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
Object.defineProperty(El.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });
var doc = { createElement: function (t) { return new El(t); }, createElementNS: function (ns, t) { return new El(t); } };
var deck = new El('div');
TD.render(deck, [{ kind: 'menu', open: false }], doc);
var m = deck.children[0];
check('Menu is a brass button with its own class', m.className, 'deck-btn brass deck-menu');
check('Menu tells a screen reader what it opens and whether it is open',
    [m.attrs['aria-label'], m.attrs['aria-expanded'], m.attrs['aria-controls']], ['Menu', 'false', 'dash-session dash-tabbar']);
check('Menu draws a line icon and its word', [m.children[0].tagName, m.children[1].textContent], ['svg', 'Menu']);
TD.render(deck, [{ kind: 'menu', open: true }], doc);
check('open: aria-expanded true', deck.children[0].attrs['aria-expanded'], 'true');
check('Menu is never disabled', deck.children[0].disabled, false);

process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run it to see it fail**

Run: `node tools/jstest/touchdeck-menu-map.test.js`
Expected: FAIL from the first check (`SIDE.menu` is `undefined`), then a `TypeError: TD.mapShape is not a function`.

- [ ] **Step 3: Implement in `touchdeck.js`**

3a. Replace `make` and `shape` (`touchdeck.js:71-93`):

```js
    function make(rows, cols, btn, gap, side, menu) {
        return {
            rows: rows, cols: cols, button: btn, gap: gap, pad: PAD, side: side, menu: !!menu,
            width: span(cols, btn, gap), height: span(rows, btn, gap)
        };
    }

    // The deck's shape for the room it has. boxW, boxH: that room in CSS px,
    // inside the slot's side borders. phone: an upright phone (six columns,
    // so a button keeps its slot on every phone). sideways: a phone on its
    // side (a side column beside the game, four rows of three, whose last
    // slot is Menu: the session bar and tab bar fold away there, #938).
    function shape(boxW, boxH, phone, sideways) {
        if (sideways) {
            var tall = span(SIDE_ROWS, BTN, GAP) <= boxH;
            return make(SIDE_ROWS, SIDE_COLS, tall ? BTN : SMALL_BTN, tall ? GAP : SMALL_GAP, true, true);
        }
        var fit = Math.floor((boxW - 2 * PAD + GAP) / (BTN + GAP));
        if (phone || fit < PHONE_COLS) {
            var wide = span(PHONE_COLS, BTN, GAP) <= boxW;
            return make(ROWS, PHONE_COLS, wide ? BTN : SMALL_BTN, wide ? GAP : SMALL_GAP, false, false);
        }
        return make(ROWS, fit, BTN, GAP, false, false);
    }

    // The Map tab's buttons beside the map (plan 2c, spec 2c, mockup frame
    // 7), at the deck's own button size (one size rule, guard 6). Upright:
    // three rows of three, the exits button under the T (160px wide at
    // 48px). Sideways: the Map card is too short for three rows, so two
    // rows of four, the exits button beside the T.
    function mapShape(deck, sideways) {
        return sideways ? make(2, 4, deck.button, deck.gap, false, false)
                        : make(3, 3, deck.button, deck.gap, false, false);
    }
```

**As built (2026-11-27):** `mapShape` passes pad 0 as a seventh argument to `make` (`make(2, 4, deck.button, deck.gap, false, false, 0)` and `make(3, 3, ..., 0)`). The Map grid lost its own padding after the headless run: the card body has the padding, so the grid's width is three buttons and two gaps (160 at 48px), not 176.

3b. In `cells` (`touchdeck.js:249-282`), replace the body from `var n = ...` through the exits level's `return out;` with:

```js
        var n = s.rows * s.cols, room = s.menu ? n - 1 : n, out = [], i;
        for (i = 0; i < n; i++) out.push({ kind: 'empty' });
        // Menu keeps the side column's last slot in every state (plan 2c
        // D3): Leave Game and Settings stay in reach before the first
        // Room.Info, in the dark and in the exits list.
        if (s.menu) out[n - 1] = { kind: 'menu' };
        if (!model || !model.ready) return out;
        if (level === 'exits' && !model.dark && model.others.length) {
            var more = model.others.length > room - 1;
            var per = more ? room - 2 : room - 1;
            var pages = Math.ceil(model.others.length / per);
            var p = (((page || 0) % pages) + pages) % pages;
            out[0] = { kind: 'back' };
            model.others.slice(p * per, p * per + per).forEach(function (o, k) {
                out[1 + k] = {
                    kind: 'exit', id: o.id, name: o.name, label: o.label, arrow: o.arrow,
                    locked: o.locked, own: o.own, combat: model.combat
                };
            });
            if (more) out[room - 1] = { kind: 'more' };
            return out;
        }
```

Update the comment above `cells` to add `'menu'` to the kinds list: `kind: 'empty', 'move', 'other', 'dark', 'back', 'exit', 'more' or 'menu' (the side column's last slot, when s.menu).`

3c. In `tap` (`touchdeck.js:297-326`): the result gains `menu`. Change the first line and add a case:

```js
        var r = { level: st.level, page: st.page, guardUntil: st.guardUntil || 0, send: '', typed: '', menu: false };
```

and before `case 'dark':`

```js
        case 'menu':
            // The bars (#938): dashboard.js shows or folds them. No hold
            // (plan 2c D10): nothing new lands under the finger in the deck.
            r.menu = true;
            break;
```

Update `tap`'s comment: `Returns the next state with send (...), typed (...) and menu (true: show or fold the bars, #938).`

3d. `ICONS` (`touchdeck.js:334-340`): add after `eye`:

```js
        ,
        menu: { box: 26, paths: ['M5 8h16', 'M5 13h16', 'M5 18h16'] }
```

(so the object reads `eye: {...},` then `menu: {...}`; write it as a normal comma-separated entry). Add "and Menu's three lines (plan 2c D12)" to the comment above `ICONS`.

3e. `classOf`: before the final `return`, add `else if (c.kind === 'menu') { k.push('deck-menu'); }` to the chain (after the `'more'` branch).

3f. `ariaOf`: add `case 'menu': return 'Menu';`

3g. `render`: after the `'more'` branch (`} else if (c.kind === 'more') { ... }`), add:

```js
            } else if (c.kind === 'menu') {
                b.appendChild(svgIcon(doc, 'menu', 'deck-icon'));
                b.appendChild(textEl(doc, 'deck-name', 'Menu'));
                // What it opens, and whether it is open (dashboard.js sets
                // open from the bars, #938).
                b.setAttribute('aria-controls', 'dash-session dash-tabbar');
                b.setAttribute('aria-expanded', c.open ? 'true' : 'false');
```

3h. Add the bars helpers after `focusAfter`:

```js
    // ---- The sideways bars (#938, plan 2c) --------------------------------
    // A sideways phone with the deck on folds the session bar and the tab
    // bar away; Menu brings both back over the game. folds: the deck shows
    // Menu now. shown: Menu brought them back. 'folded', 'shown', or '' for
    // the bars as always (dashboard.js sets it as #dashboard's data-bars).
    function barsState(folds, shown) {
        if (!folds) return '';
        return shown ? 'shown' : 'folded';
    }

    // After a click on a button in the bars: they fold again once the player
    // has picked something, unless the pick left one of the bars' own
    // dialogs open: the Settings menu, or Leave Game's question (plan 2c D5).
    function barsStayAfterPick(settingsOpen, confirmOpen) {
        return !!(settingsOpen || confirmOpen);
    }
```

3i. Exports (`touchdeck.js:488-497`): add `mapShape: mapShape, barsState: barsState, barsStayAfterPick: barsStayAfterPick,` and add `mapShape / barsState / barsStayAfterPick  the Map tab's buttons and the sideways bars (plan 2c)` to the header comment's list (`touchdeck.js:11-20`), and `touchdeck-menu-map.test.js` to its "Tested by" line.

- [ ] **Step 4: Run the new test and the existing deck tests**

Run: `node tools/jstest/touchdeck-menu-map.test.js && node tools/jstest/touchdeck.test.js && node tools/jstest/touchdeck-moves.test.js && node tools/jstest/touch-targets.test.js`
Expected: every line `ok`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck-menu-map.test.js
git commit -m "feat(webclient): Menu in the side column, the Map tab's shape, the bars state (#781, #938)"
```

---

### Task 2: Page markup and the shared `.deck-grid` CSS

**Files:**
- Modify: `_datafiles/html/public/webclient-pure.html:333-335`, `:360`
- Modify: `_datafiles/html/public/static/css/dashboard.css:268-293`, `:363`, `:368`, `:2019-2026`
- Modify: `tools/jstest/touch-targets.test.js:80`, `:98`
- Modify: `tools/jstest/webclient-input-queue.test.js:64`
- Modify: `tools/webclient-tests/touch-frame.js:35`

- [ ] **Step 1: Update the static checks first (they fail until the markup and CSS move)**

`tools/webclient-tests/touch-frame.js:35`: replace the check with

```js
check(/<div id="touch-deck" class="deck-grid" role="group" aria-label="Movement"><\/div>/.test(html), 'the deck starts empty and is one labelled group of buttons (plan 2b), styled as a deck grid (plan 2c)');
check(/<div id="map-deck" class="deck-grid" role="group" aria-label="Movement" hidden><\/div>/.test(html), 'the Map tab has its own movement buttons, hidden until the Map tab opens on a phone (plan 2c)');
check(html.indexOf('id="map-render"') >= 0 && html.indexOf('id="map-deck"') > html.indexOf('id="map-render"') &&
      html.indexOf('id="map-deck"') < html.indexOf('id="map-controls"'), 'the Map tab buttons sit right after the map');
```

`tools/jstest/touch-targets.test.js:80` and `:98`: read the shared class.

```js
var deckRule = (/\.deck-grid \{[^}]*\}/.exec(css) || [''])[0];
```

```js
var tight = Number((/\.deck-grid\.deck-small \.deck-name \{[^}]*letter-spacing:\s*(-?[\d.]+)px/.exec(css) || [])[1]);
```

`tools/jstest/webclient-input-queue.test.js:64`: `['.auto-chip', '.auto-row', '.deck-grid'].forEach(...)`.

Run: `node tools/webclient-tests/touch-frame.js; node tools/jstest/touch-targets.test.js; node tools/jstest/webclient-input-queue.test.js`
Expected: FAIL on the new markup checks, the deck grid rule, and the grey-out.

- [ ] **Step 2: Markup**

`webclient-pure.html:334`:

```html
          <div id="touch-deck" class="deck-grid" role="group" aria-label="Movement"></div>
```

and after `<div id="map-render"></div>` (`:360`):

```html
            <!-- Moving by touch on the Map tab (touch arc plan 2c): the deck's
                 T and exits button beside the map, while the Map is the open
                 tab on a phone (an open tab takes the deck's place). Drawn by
                 dashboard.js from the same Room.Info as the deck. -->
            <div id="map-deck" class="deck-grid" role="group" aria-label="Movement" hidden></div>
```

- [ ] **Step 3: CSS**

`dashboard.css:268`: rename the selector `#touch-deck {` to `.deck-grid {` and take `border-top: 2px solid var(--gold-rule);` out of it (the rule keeps its grid, padding, background and every `--deck-*` token unchanged). Right after it add:

```css
/* An element that sets display wins over [hidden]; a hidden grid stays hidden. */
.deck-grid[hidden] { display: none; }
/* The deck in the slot has the rule above it; the Map tab's buttons do not. */
#touch-deck { border-top: 2px solid var(--gold-rule); }
```

`:363`: `#touch-deck.deck-small .deck-name` becomes `.deck-grid.deck-small .deck-name`.
`:368`: `#touch-deck.deck-dark` becomes `.deck-grid.deck-dark`.
`:2021` and `:2026`: `body.input-queue-full #touch-deck` becomes `body.input-queue-full .deck-grid` (both lines).

Then, after `.deck-lock` (`:367`), add the Menu button and the Map tab's column:

```css
/* Menu, the side column's last button on a sideways phone (#938): three
   lines over its word, the site brass like every lit deck button. */
.deck-menu { gap: 2px; }
/* The Map tab's buttons (plan 2c, mockup frame 7): a column beside the
   map, which keeps the rest of the card's width (and its 320 by 300 shape,
   so a narrower map is also shorter). Only while they show. The other
   parts of the map body (the legend, where you are) span both columns. */
#panel-map .map-body:has(> #map-deck:not([hidden])) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  column-gap: 8px;
  align-items: start;
}
#panel-map .map-body:has(> #map-deck:not([hidden])) > * { grid-column: 1 / -1; }
#panel-map .map-body:has(> #map-deck:not([hidden])) > #map-render { grid-column: 1; grid-row: 1; }
#panel-map .map-body:has(> #map-deck:not([hidden])) > #map-deck { grid-column: 2; grid-row: 1; }
#map-deck { background: transparent; }
```

- [ ] **Step 4: Run the checks**

Run: `node tools/webclient-tests/touch-frame.js && node tools/jstest/touch-targets.test.js && node tools/jstest/webclient-input-queue.test.js && node tools/jstest/webclient-colours.test.js`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/css/dashboard.css tools/jstest/touch-targets.test.js tools/jstest/webclient-input-queue.test.js tools/webclient-tests/touch-frame.js
git commit -m "feat(webclient): one deck grid class, the Map tab's button column (#781)"
```

---

### Task 3: `dashboard.js`: one draw path for two grids, the Map tab's buttons

**Files:**
- Modify: `_datafiles/html/public/static/js/dashboard.js:164-172`, `:193-203`, `:638-705`, `:739-747`
- Modify: `tools/jstest/dashboard-deck.test.js`

- [ ] **Step 1: Write the failing test**

In `tools/jstest/dashboard-deck.test.js`:

1a. Rename the memo reads (lines 114, 174, 180): `D._deckCells` becomes `D._deckMemo.cells`.

1b. In the fake `doc.getElementById`, add the dashboard element the bars need (Task 4) and the map grid. Replace the `getElementById` function with:

```js
    getElementById: function (id) {
        if (id === 'dashboard') return dash;
        if (id === 'dash-col-center') return col;
        return null;
    },
```

and above `doc = {` add `var dash = new FakeEl('div'); dash.parentNode = parent;`. (`_buildPhone` reads `getElementById("dashboard").parentNode`, which is still `parent`.)

1c. After the line `D._deck = deck;` add:

```js
var mapDeck = new FakeEl('div');
mapDeck.hidden = true;
D._mapDeck = mapDeck;
```

1d. Before the tab bar section (`// --- the tab bar draws its icons`), add:

```js
// --- the Map tab's buttons (plan 2c) -------------------------------------------

D.mode = 'phone';   // the Map tab's buttons are a phone's (D9)
D.setDeckMode('');
D.setMoves(tower);
check('Map tab closed: its buttons stay hidden', mapDeck.hidden, true);
D._drawerActive = 'map';
D.applyDeck();
check('Map tab open on a phone: its buttons show, 3 by 3 at the deck size (D1, D11)',
    [mapDeck.hidden, mapDeck.children.length, mapDeck.style.getPropertyValue('--deck-cols'), mapDeck.style.getPropertyValue('--deck-btn')],
    [false, 9, '3', '48px']);
function atMap(kind, pred) {
    return D._mapMemo.cells.findIndex(function (c) { return c.kind === kind && (!pred || pred(c)); });
}
var sent = pressed.length;
D._deckTap(atMap('move', WEST), 'map');
check('a Map tab arrow sends the same fixed id as the deck', pressed.slice(sent), ['m4']);
D._deckTap(atMap('other'), 'map');
check('the Map tab exits button opens the list, shared with the deck (D2)', [D._deckState.level, D._mapMemo.cells[0].kind], ['exits', 'back']);
D._deckState.guardUntil = 0;
D._deckTap(atMap('back'), 'map');
check('Back returns both to the T', D._deckState.level, 'top');
D.setMoves(room(6418, {}, { dark: true }));
check('a pitch-dark room: the Map tab buttons go dark too', mapDeck.classList.contains('deck-dark'), true);
D.mode = 'column';
D.applyDeck();
check('outside phone mode the Map tab buttons hide (D9)', mapDeck.hidden, true);
D.mode = 'phone';
D._drawerActive = null;
D.applyDeck();
check('Map tab closed again: hidden', mapDeck.hidden, true);
D.setMoves(null);
```

- [ ] **Step 2: Run it to see it fail**

Run: `node tools/jstest/dashboard-deck.test.js`
Expected: FAIL with `TypeError: Cannot read properties of undefined (reading 'cells')` (`_deckMemo` does not exist yet).

- [ ] **Step 3: Implement**

3a. State (`dashboard.js:168-170`): replace `_deckCells: [],`, `_deckSig: "",` and `_deckDrawnLevel: "top",` with

```js
    // What each grid last drew (cells, their JSON, the level): the deck in
    // the slot, and the Map tab's buttons (plan 2c), which share
    // _deckState, the model and the senders (D2).
    _deckMemo: { sig: "", cells: [], level: "top" },
    _mapDeck: null,     // #map-deck, beside the map (plan 2c)
    _mapMemo: { sig: "", cells: [], level: "top" },
```

and update the comment above (`:161-163`) to say "the cells each grid drew".

3b. `init` (`:193-203`): replace the deck click block with

```js
      this._slot = document.getElementById("touch-slot");
      this._deck = document.getElementById("touch-deck");
      this._mapDeck = document.getElementById("map-deck");
      // One click handler per button grid (plan 2b, 2c). A disabled button
      // gets no click.
      var deckOwner = this;
      [[this._deck, "deck"], [this._mapDeck, "map"]].forEach(function (g) {
        if (!g[0]) return;
        g[0].addEventListener("click", function (e) {
          var b = e.target && e.target.closest ? e.target.closest("button[data-cell]") : null;
          if (b && !b.disabled) deckOwner._deckTap(Number(b.getAttribute("data-cell")), g[1]);
        });
      });
```

3c. `applyDeck` (`:666-705`). Replace from `slot.classList.toggle("deck-on", on);` to the end of the function with:

```js
      slot.classList.toggle("deck-on", on);
      if (!on) {
        if (this._mapDeck) this._mapDeck.hidden = true;
        this._setBars(false);
        return;
      }

      var sw = window.screen ? screen.width : 0, sh = window.screen ? screen.height : 0;
      var phone = !!(VF && VF.isPhone(sw, sh)) && coarse;
      var sideways = !!(VF && VF.isSidewaysPhone(sw, sh, VF.orientationType(window), coarse));
      var col = document.getElementById("dash-col-center");
      var boxW = (col ? col.clientWidth : window.innerWidth) - 2;   // the slot's side borders
      var boxH = col ? col.clientHeight : window.innerHeight;
      var s = TD.shape(boxW, boxH, phone && !sideways, sideways);
      this._sizeDeck(deck, s);
      // The deck's height with its 2px top rule: a short tab keeps it.
      slot.style.setProperty("--deck-h", (s.height + 2) + "px");
      // The bars fold while the side column carries Menu (#938, plan 2c).
      this._setBars(s.menu);
      // The buttons (plan 2b): touchdeck.js lays the last Room.Info out in
      // this shape and draws it, again only when a cell changes.
      var model = TD.moves(this._moves, this._deckMode);
      var st = this._deckState, bars = this._barsShown;
      var cells = TD.cells(model, s, st.level, st.page).map(function (c) {
        return c.kind === "menu" ? { kind: "menu", open: bars } : c;
      });
      deck.classList.toggle("deck-dark", model.dark);
      this._drawDeck(deck, this._deckMemo, cells, active);

      // The Map tab's buttons (plan 2c): on a phone an open tab takes the
      // deck's place, so the Map tab carries the same T beside the map,
      // from the same model and state (D2), in phone mode only (D9).
      var mapDeck = this._mapDeck;
      if (!mapDeck) return;
      var showMap = this.mode === "phone" && this._drawerActive === "map";
      mapDeck.hidden = !showMap;
      if (!showMap) return;
      var ms = TD.mapShape(s, sideways);
      this._sizeDeck(mapDeck, ms);
      mapDeck.classList.toggle("deck-dark", model.dark);
      this._drawDeck(mapDeck, this._mapMemo, TD.cells(model, ms, st.level, st.page), active);
    },

    // A grid's size from its shape (touchdeck.js shape or mapShape).
    _sizeDeck: function (el, s) {
      el.style.setProperty("--deck-cols", String(s.cols));
      el.style.setProperty("--deck-btn", s.button + "px");
      el.style.setProperty("--deck-gap", s.gap + "px");
      el.style.setProperty("--deck-pad", s.pad + "px");
      el.classList.toggle("deck-small", s.button < window.TouchDeck.BTN);
    },

    // Draws cells into a grid when they changed since memo. render rebuilds
    // every button, so a focused one would drop the keyboard and screen
    // reader focus to the page: hand it on (touchdeck.js focusAfter).
    _drawDeck: function (el, memo, cells, active) {
      var TD = window.TouchDeck;
      var sig = JSON.stringify(cells);
      if (sig === memo.sig && el.childElementCount === cells.length) return;
      var had = active && active !== el && el.contains(active) && active.getAttribute
        ? active.getAttribute("data-cell") : null;
      var levelWas = memo.level;
      memo.sig = sig;
      memo.cells = cells;
      memo.level = this._deckState.level;
      TD.render(el, cells, document);
      if (had !== null) {
        var to = el.children[TD.focusAfter(cells, Number(had), levelWas, this._deckState.level)];
        if (to && to.focus) to.focus();
      }
    },
```

(The `deck.style.setProperty(...)` lines and the `deck.classList.toggle("deck-small", ...)` line it replaced are now in `_sizeDeck`. `_setBars` and `_barsShown` come in Task 4; add a stub now so this task runs on its own:

```js
    _barsShown: false,
    _setBars: function () {},
```

next to `_mapMemo`. Task 4 replaces the stub.)

3d. `_deckTap` (`:739-747`):

```js
    // A tap on cell i of a grid: "deck" (the slot) or "map" (the Map tab's
    // buttons, plan 2c). touchdeck.js tap decides what it does.
    _deckTap: function (i, which) {
      var TD = window.TouchDeck;
      var memo = which === "map" ? this._mapMemo : this._deckMemo;
      var c = memo.cells[i];
      if (!TD || !c) return;
      var r = TD.tap(this._deckState, c, Date.now());
      this._deckState = { level: r.level, page: r.page, guardUntil: r.guardUntil };
      if (r.send && this._deckPress) this._deckPress(r.send);
      if (r.typed && this._deckType) this._deckType(r.typed);
      if (r.menu) { this._showBars(!this._barsShown); return; }
      this.applyDeck();
    },
```

and add a stub `_showBars: function () {},` beside the `_setBars` stub (Task 4 replaces both).

- [ ] **Step 4: Run the tests**

Run: `node tools/jstest/dashboard-deck.test.js && node tools/jstest/touchdeck-menu-map.test.js`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/dashboard.js tools/jstest/dashboard-deck.test.js
git commit -m "feat(webclient): the Map tab walks: the deck's T beside the map (#781)"
```

---

### Task 4: `dashboard.js` and CSS: the bars fold and Menu

**Files:**
- Modify: `_datafiles/html/public/static/js/dashboard.js` (the Task 3 stubs; `init`)
- Modify: `_datafiles/html/public/static/css/dashboard.css` (after the side column block, `:1438`)
- Modify: `tools/jstest/dashboard-deck.test.js`
- Modify: `tools/webclient-tests/touch-frame.js`

- [ ] **Step 1: Write the failing tests**

In `dashboard-deck.test.js`, before the tab bar section, add:

```js
// --- the sideways bars (#938, plan 2c) -------------------------------------------

// A phone on its side: viewportfit's isSidewaysPhone reads the screen.
win.screen = { width: 800, height: 360 };
win.ViewportFit = {
    isPhone: function () { return true; },
    isSidewaysPhone: function () { return true; },
    orientationType: function () { return 'landscape-primary'; },
    editableFocused: function () { return false; }
};
sandbox.screen = win.screen;
D._drawerActive = null;
D.setMoves(tower);
check('sideways with the deck on: the bars fold', dash.dataset.bars, 'folded');
var menuAt = D._deckMemo.cells.findIndex(function (c) { return c.kind === 'menu'; });
check('Menu is the side column\'s last slot', menuAt, 11);
D._deckState.guardUntil = 0;   // Back's hold from the Map section above
D._deckTap(menuAt);
check('Menu shows the bars', [dash.dataset.bars, D._deckMemo.cells[menuAt].open], ['shown', true]);
D._deckTap(menuAt);
check('Menu again folds them', [dash.dataset.bars, D._deckMemo.cells[menuAt].open], ['folded', false]);
D._showBars(true);
win.localStorage = { getItem: function () { return 'never'; }, setItem: function () {} };
D.applyDeck();
check('Touch controls on Never: no deck, no Menu, the bars as always (D4)', [dash.dataset.bars, D._barsShown], [undefined, false]);
win.localStorage = { getItem: function () { return 'always'; }, setItem: function () {} };
win.screen = { width: 360, height: 800 };
sandbox.screen = win.screen;
win.ViewportFit.isSidewaysPhone = function () { return false; };
D.applyDeck();
check('upright again: the bars as always', dash.dataset.bars, undefined);
delete win.ViewportFit;
win.screen = undefined;
D.setMoves(null);
```

(The fake `dash` is a `FakeEl`, so `dataset` is a plain object and a deleted key reads `undefined`.)

In `tools/webclient-tests/touch-frame.js`, before its final exit, add CSS checks (the file already reads `css`):

```js
check(/#dashboard\[data-bars="folded"\] #dash-session \{ display: none; \}/.test(css), 'folded: no session bar (#938)');
check(/html:has\(#dashboard\[data-bars="folded"\]\) #dash-tabbar \{ display: none !important; \}/.test(css), 'folded: no tab bar (#938)');
check(/#dashboard\[data-bars="shown"\] #dash-session \{[^}]*position: fixed;/.test(css), 'shown: the session bar over the game (#938, D6)');
```

Run: `node tools/jstest/dashboard-deck.test.js; node tools/webclient-tests/touch-frame.js`
Expected: FAIL (`dash.dataset.bars` is `undefined`; the CSS rules are absent).

- [ ] **Step 2: Implement the bars in `dashboard.js`**

Replace the Task 3 stubs (`_barsShown`, `_setBars`, `_showBars`) with:

```js
    // The sideways bars (#938, plan 2c): whether Menu brought them back.
    _barsShown: false,

    // folds: the deck shows Menu now (a sideways phone with the deck on).
    // Sets data-bars on #dashboard (never on the root, which the keyboard
    // watch observes), and forgets a shown state once nothing folds.
    _setBars: function (folds) {
      var TD = window.TouchDeck, dash = document.getElementById("dashboard");
      if (!folds) this._barsShown = false;
      if (!TD || !dash || !dash.dataset) return;
      var state = TD.barsState(!!folds, this._barsShown);
      if ((dash.dataset.bars || "") === state) return;
      if (state) dash.dataset.bars = state; else delete dash.dataset.bars;
    },

    // Menu shows the bars, or folds them again.
    _showBars: function (on) {
      this._barsShown = !!on;
      this.applyDeck();
    },
```

In `init`, after the `focusin`/`focusout` listeners (`:254-255`), add:

```js
      // The bars Menu brought back (#938) fold again when the player picks
      // something in them, unless the pick left one of their own dialogs
      // open (Settings, Leave Game's question; plan 2c D5), or taps anywhere
      // else. Escape folds them and hands the focus back to Menu (D7).
      document.addEventListener("pointerdown", function (e) {
        if (!self._barsShown) return;
        var t = e.target;
        if (t && t.closest && t.closest("#dash-session, #dash-tabbar, .deck-menu")) return;
        self._showBars(false);
      }, true);
      document.addEventListener("click", function (e) {
        if (!self._barsShown) return;
        var t = e.target;
        if (!(t && t.closest && t.closest("#dash-session button, #dash-tabbar button"))) return;
        // After the page's own handler has opened or closed its dialog.
        setTimeout(function () {
          if (!self._barsShown) return;
          var menu = document.getElementById("settings-menu");
          var confirm = document.getElementById("sess-confirm");
          if (window.TouchDeck.barsStayAfterPick(!!menu && !menu.hidden,
              !!confirm && confirm.style.display !== "none")) return;
          self._showBars(false);
        }, 0);
      });
      document.addEventListener("keydown", function (e) {
        if (e.key !== "Escape" || !self._barsShown) return;
        self._showBars(false);
        var menuBtn = document.querySelector(".deck-menu");
        if (menuBtn) menuBtn.focus();
      });
```

(`self` is already defined in `init` at `:210`. These three listeners sit after it.)

- [ ] **Step 3: CSS**

After the side column block (`dashboard.css:1438`, the closing `}` of the second `@media (pointer: coarse) and (orientation: landscape) and (max-height: 500px)`), add:

```css
/* The sideways bars (#938, owner ruling 2026-11-22 option A; plan 2c). A
   sideways phone with the deck on folds the session bar and the tab bar
   away (dashboard.js sets data-bars from touchdeck.js barsState), and the
   game takes their rows. Menu, the side column's last button, brings both
   back over the game (D6): the session bar across the top, the tab bar in
   its usual place, until the player picks something or taps away. */
#dashboard[data-bars] {
  padding-bottom: 6px;
  grid-template-rows: minmax(0, 1fr);
  grid-template-areas: "center";
}
#dashboard[data-bars="folded"] #dash-session { display: none; }
html:has(#dashboard[data-bars="folded"]) #dash-tabbar { display: none !important; }
#dashboard[data-bars="shown"] #dash-session {
  position: fixed;
  top: 6px;
  left: 6px;
  right: 6px;
  z-index: 51;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.6);
}
```

- [ ] **Step 4: Run the tests**

Run: `node tools/jstest/dashboard-deck.test.js && node tools/webclient-tests/touch-frame.js && node tools/jstest/touchdeck-menu-map.test.js`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/static/css/dashboard.css tools/jstest/dashboard-deck.test.js tools/webclient-tests/touch-frame.js
git commit -m "feat(webclient): a sideways phone folds its bars; Menu brings them back (#938)"
```

---

### Task 5: Guard 6 and the static checks

**Files:**
- Modify: `tools/jstest/touch-targets.test.js:28`, `:59-68`

- [ ] **Step 1: Sideways has the bars' height, and the Map shapes fit their card**

Replace the matrix loop's `boxH` line (`:63`) with:

```js
    // A sideways phone with the deck on folds the session bar and tab bar
    // away (#938, plan 2c), so its side column has their height too.
    var boxH = h - 2 * PAD - (sideways ? 0 : SESSION + TABBAR);
```

and after the existing `check(...)` in the loop add:

```js
    // The Map tab's buttons (plan 2c): the deck's size (D11), inside the
    // Map card (45% of the screen, F19) under its title bar.
    if (phone) {
        var ms = TD.mapShape(s, sideways);
        var cardBody = Math.floor(0.45 * h) - MAP_TITLE;
        var mp = problems(ms, ms.width, cardBody);
        if (ms.height > cardBody) mp.push(ms.height + 'px tall in a ' + cardBody + 'px Map card');
        if (ms.width > boxW - MAP_MIN) mp.push('leaves the map ' + (boxW - ms.width) + 'px');
        check(m[0] + ' Map tab: ' + ms.rows + 'x' + ms.cols + ' at ' + ms.button + 'px' + (mp.length ? ' (' + mp.join('; ') + ')' : ''), mp.length === 0);
    }
```

with, beside the other constants (`:28`):

```js
var MAP_TITLE = 36;   // the Map card's title bar on a phone, the name strip's height (--tabname-h)
var MAP_MIN = 120;    // the narrowest the map may get beside its buttons
```

Add a can-fail check at the end of the "The guard can fail." group:

```js
check('the Map guard fails three rows in a sideways card',
    TD.mapShape(TD.shape(0, 300, true, true), false).height > Math.floor(0.45 * 360) - MAP_TITLE);
```

- [ ] **Step 2: Run it**

Run: `node tools/jstest/touch-targets.test.js`
Expected: every line `ok`. The sideways line now reads `phone on its side 800x360: 4x3 at 48px` (it read 44px before, F26).

- [ ] **Step 3: Commit**

```bash
git add tools/jstest/touch-targets.test.js
git commit -m "test(webclient): guard 6 covers the Map tab's buttons and the folded bars (#781)"
```

---

### Task 6: Docs

**Files:**
- Modify: `docs/PATCH_NOTES.md`, `docs/README.md`, `docs/BRAND_STANDARDS.md`, `docs/roadmaps/TOUCH_ARC_ROADMAP.md`, `docs/superpowers/specs/2026-11-22-moving-by-touch-design.md`

- [ ] **Step 1: PATCH_NOTES** (top, dated the day it is built):

```markdown
## 2026-11-2X: Walk from the Map, and more room sideways

- On a phone, the Map tab now has the same walking buttons as the buttons above the command box, beside the map. The exits button opens the room's other ways out there too, and the map stays in view.
- With your phone on its side, the bar at the top and the round buttons at the bottom fold away, so the game has more room. Tap Menu, the last button beside the game, to bring them back. Pick something or tap anywhere else and they fold away again.
```

- [ ] **Step 2: README rows** for the new files: `tools/jstest/touchdeck-menu-map.test.js` (in the jstest list) and this plan (in the plans list), with one line each in the style of the 2b rows.

- [ ] **Step 3: BRAND_STANDARDS.md** deck section: add Menu (three lines over the word, brass) and the Map tab's column (the deck's button size beside the map).

- [ ] **Step 4: Roadmap row 2** and the spec's "2c" heading: mark 2c built (plan path, PR number when known), as 2a and 2b are marked.

- [ ] **Step 5: Commit**

```bash
git add docs/PATCH_NOTES.md docs/README.md docs/BRAND_STANDARDS.md docs/roadmaps/TOUCH_ARC_ROADMAP.md docs/superpowers/specs/2026-11-22-moving-by-touch-design.md
git commit -m "docs: touch arc plan 2c rows, patch note, brand doc (#781, #938)"
```

---

### Task 7: Full gate

- [ ] **Step 1:** `gofmt -l internal/ modules/ *.go` prints nothing (no Go changes are planned; this catches an accident).
- [ ] **Step 2:** `go build ./... && go test ./... -count=1 2>&1 | tail -40`: every package `ok`.
- [ ] **Step 3:** `bash tools/lint.sh`: `0 issues.`
- [ ] **Step 4:** the JS job, in Git Bash:

```bash
failed=0
while IFS= read -r f; do case "$f" in *.min.js) continue ;; esac; if [ -f .jshintignore ] && grep -Fxq "$f" .jshintignore; then continue; fi; node --check "$f" || failed=1; done < <(git ls-files '*.js')
for t in tools/jstest/*.test.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
for t in tools/webclient-tests/*.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
echo "failed=$failed"
```

Expected: `failed=0`.

- [ ] **Step 5:** `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`: `ok`.

---

### Task 8: HEADLESS: size matrix, Map tab and Menu (controller, not a subagent)

Boot exactly as plan 2b Task 11 Step 1 (detached worktree `C:/tmp/dogmud-2c-shots`, instance saves wiped, the same four-entry manifest and private-port override in the session scratchpad, start in the background, record the PID). Screenshots go to the main checkout's root as `2c-<size>-<state>.png`, not committed.

- [ ] **Step 1: The size matrix at Tower Top.** The plan 2b Task 11 Step 2 session list and measure `eval`, plus one more `eval` per session:

```js
(() => JSON.stringify({ bars: document.getElementById('dashboard').dataset.bars || '', sess: getComputedStyle(document.getElementById('dash-session')).display, menu: !!document.querySelector('#touch-deck .deck-menu') }))()
```

Expected: as plan 2b's table, except 800x360: `48x48`, gap 8 (the bars' height is the column's now), `bars` `folded`, `sess` `none`, `menu` true. Every other size: `bars` empty, `menu` false.

- [ ] **Step 2: The Map tab at 360x800 and 320x640** (`tower` login). Steps: wait 3500; click `.gtab[data-panel="map"]`; wait 800; eval

```js
(() => { const m = document.getElementById('map-deck'); const r = document.getElementById('map-render').getBoundingClientRect(); const b = m.getBoundingClientRect(); return JSON.stringify({ hidden: m.hidden, n: m.children.length, lit: [...m.querySelectorAll('.deck-move:not(.deck-dim)')].map(x => x.getAttribute('aria-label')), mapW: Math.round(r.width), mapH: Math.round(r.height), colW: Math.round(b.width), right: Math.round(b.right), vw: innerWidth, pageW: document.documentElement.scrollWidth }); })()
```

Expected: `hidden` false, `n` 9, `lit` `["Go west"]`, `colW` 160 at 360 (158 at 320), `mapW` at least 120, `right <= vw`, `pageW <= vw`. Shot `map`. (As built, 2026-11-27: `colW` at 320 is 142, not 158. 320 takes 44px buttons, so 3x44 + 2x5 = 142; the Map grid has no padding of its own, the card body has it.) Then click `#map-deck .deck-other`, wait 600, eval `[...document.querySelectorAll('#map-deck .deck-exit')].map(b => b.textContent)` expect `["ledge","shrine"]` and `!!document.querySelector('#map-render svg')` expect `true` (the map stays); shot `map-exits`; click the `shrine` button, wait 2500, eval `!!document.querySelector('#map-deck .deck-back')` expect `false`; shot `map-after-move`.

- [ ] **Step 3: Menu at 800x360** (`junction` login, any room). Steps: wait 3500; shot `side-folded`; click `#touch-deck .deck-menu`; wait 300; eval `document.getElementById('dashboard').dataset.bars + '|' + getComputedStyle(document.getElementById('dash-session')).position + '|' + getComputedStyle(document.getElementById('dash-tabbar')).display` expect `shown|fixed|block`; shot `side-menu`; click `.gtab[data-panel="map"]`; wait 800; eval `document.getElementById('dashboard').dataset.bars + '|' + document.getElementById('map-deck').children.length + '|' + getComputedStyle(document.getElementById('map-deck')).gridTemplateColumns.split(' ').length` expect `folded|8|4` (a pick folds them; the Map tab sideways is 2 by 4); shot `side-map`. Then reopen Menu, click `#btn-settings`, wait 300, eval `dataset.bars` expect `shown` (Settings keeps them, D5); click `#settings-close`, wait 300, expect `folded`. Reopen Menu; dispatch a pointerdown on `#terminal` (`document.getElementById('terminal').dispatchEvent(new PointerEvent('pointerdown', {bubbles: true}))`), expect `folded` (tap away).

- [ ] **Step 4: Record and clean up.** Numbers into the PR body. Stop the server by its PID only; `git worktree remove --force C:/tmp/dogmud-2c-shots`.

---

### Task 9: OWNER'S CHECK: Malia's S21 (#834)

After the PR is green, before merge. Malia's Galaxy S21 on the local test server over Wi-Fi (look up the LAN IP each time). A login made with ordinary character creation (`new`), easy name; ordinary player steps only. Settings, Touch controls on Automatic.

1. Upright, open the Map tab. Beside the map: the arrows, the stairs and "exits", as above the command box. Only the ways out are bright.
2. Tap a bright arrow on the Map tab: you walk, and the map follows you.
3. Tap "exits" on the Map tab: Back and the room's other exits; the map stays. Tap Back: the arrows come back.
4. Close the Map tab: the buttons above the command box are as you left them.
5. Turn the phone on its side. The bar with your name at the top and the round buttons at the bottom are gone; the game is taller. The buttons sit in a column beside the game, with Menu at the bottom right.
6. Tap Menu: both bars come back over the game. Tap the game text: they fold away.
7. Tap Menu, then the Map button: the bars fold, the Map opens with its walking buttons beside the map. Close it with ✕.
8. Tap Menu, then Settings: the bars stay while Settings is open. Close Settings: they fold.
9. Tap Menu, then Leave Game: the question shows and the bars stay. Tap Stay: they fold.
10. Settings, Touch controls on Never: the bars come back for good and Menu is gone. Set it back to Automatic.

Record each result on #834 (`gh issue comment 834 --repo pruuk/DOGMud`).

---

### Task 10: PR

- [ ] **Step 1:** Re-run Task 7 if anything changed since.
- [ ] **Step 2:** Push and open the PR:

```bash
git push -u origin feature/moving-by-touch-2c
gh pr create --repo pruuk/DOGMud --base master --head feature/moving-by-touch-2c --title "Touch arc 2c: walking from the Map tab, and Menu on a sideways phone (#781, #938)" --body-file "$S/pr-body.md"
```

The body (written with the Write tool): what tasks 1 to 6 built, the Task 8 numbers, `Part of #781`, `Part of #938` (never a closing keyword), the attribution line. Confirm the printed URL says `pruuk/DOGMud`.

- [ ] **Step 3:** `gh pr checks <n> --repo pruuk/DOGMud --watch`, then `gh run list --repo pruuk/DOGMud --branch feature/moving-by-touch-2c` to confirm the javascript job ran.
- [ ] **Step 4:** After Malia's check passes, merge (`gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch`), then comment on #781 (2c landed; the touch arc's step 2 is done) and on #938 (built in PR #n), and close #938 if Malia's steps 5 to 10 passed. The owner deploys.

---

## Self-review

**Spec coverage.**

| Spec 2c / ruling item | Task |
|---|---|
| Map tab carries the same T and exits button beside the map, about 160px | 1 (`mapShape`), 2 (markup, CSS), 3 |
| The map pushed left, keeping its size as far as the buttons allow | 2 (grid, `minmax(0, 1fr)`), 5 (`MAP_MIN`), 8 |
| Same ids, states and marks as the deck | 3 (one model, one `cells`, one `render`) |
| Other exits opens in place of the buttons; the map stays | 1 (Map cells), 3, 8 |
| Shown only when the deck would show | 3 (`!on` hides it; phone mode, D9) |
| Sideways: session bar and tab bar fold away | 4, 8 |
| Menu in the side column brings both back over the game | 1, 4, 8 |
| ...until the player picks something or taps away | 4 (D5, pointerdown, click), 8 |
| Map reached sideways through Menu, then Map, with the same T | 1 (`mapShape` sideways), 3, 8 |
| The sideways toast stays | untouched (`_watchSideways`) |
| jstests: the Map tab copy, the sideways Menu button | 1, 3, 4 |
| Guard 6 across the size matrix | 5 |
| Headless matrix, Malia's S21 check | 8, 9 |

**Placeholder scan:** none; the patch note's date is the build day.

**Type consistency:** `shape(...)` and `mapShape(deck, sideways)` return `{rows, cols, button, gap, pad, side, menu, width, height}`; `cells` kinds add `menu` (`{kind:'menu', open}` after dashboard.js marks it [As built, 2026-11-27: no caller puts `open` on a cell. `render` always writes `aria-expanded="false"` and `dashboard.js` sets it in place on the Menu button after drawing, so the deck is not rebuilt]); `tap` returns `{level, page, guardUntil, send, typed, menu}`; `barsState(folds, shown)` returns `'' | 'folded' | 'shown'`; `barsStayAfterPick(settingsOpen, confirmOpen)`. Dashboard: `_deckMemo`, `_mapMemo` (`{sig, cells, level}`), `_mapDeck`, `_barsShown`, `_sizeDeck(el, s)`, `_drawDeck(el, memo, cells, active)`, `_setBars(folds)`, `_showBars(on)`, `_deckTap(i, which)`.

## Open questions for the owner

None. The calls this plan made on its own are in "Decisions taken without asking" (D1 to D12); D1 (the sideways Map shape) and D5 (what counts as picking something) are the two most worth a look.
