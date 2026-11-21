# Touch frame plan 1c (phone polish) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Edit files with the Edit and Write tools only (guard.py H3); never write a repo file from the shell.

**Spec:** `docs/superpowers/specs/2026-11-21-touch-frame-design.md`, section "Phone polish (plan 1c)" (approved 2026-11-21). Plans 1a (#932) and 1b (#935) are merged.

**Goal:** On a phone: no Game title bar; sub-tabs (Comms, Inventory, Quests, Automation) in one row that scrolls sideways, fading at the edge with more; a once-per-device tip toward full screen (Android) or the Home Screen (iPhone Safari); Desktop site undone where the browser allows, with a once-per-device tip where it does not; the brand doc updated for the touch frame.

**Architecture:** Client only. Decisions stay pure functions with jstests: `viewportfit.js` gains `takeOnce` (the existing once-only note, generalised; the sideways note now uses it), `isDesktopSite` and `phoneTip`; `touchdeck.js` gains `rowFade` and `scrollLeftToShow`. The outer page (`webclient.html`) rewrites its viewport tag for Desktop site and exposes three facts on `WebclientFrame`; `dashboard.js` wires the sub-tab rows and shows the tip through its existing `_toast`.

**Tech Stack:** Plain browser JS (no build), CSS, zero-dependency Node tests (`tools/jstest`, `tools/webclient-tests`), headless Edge (`tools/webshot/live-shot.js`).

---

## Facts verified against source (2026-11-21, master `5ef19297f`)

| Fact | Source |
|---|---|
| The Game title bar is `#panel-feed > .dash-panel-head`, holding only the pop-out button | `webclient-pure.html:321-323` |
| It is already hidden with the keyboard up on a short screen | `static/css/dashboard.css:953` (`@media (max-height: 320px)`) |
| Layout mode is on `#dashboard[data-mode]`; a sideways phone is also `phone` | `dashboard.js` `_computeMode` (`:368-`); 1a size matrix (800x360: `mode: "phone"`) |
| Four sub-tab rows share `.subtabs`: Quests, Inventory, Comms, Automation | `webclient-pure.html:391`, `:408`, `:428`, `:460` |
| `.subtabs` wraps (`flex-wrap: wrap`) with a 2026-11-07 comment saying it wraps rather than scroll so every channel stays in sight; `.subtabs::after` is a spacer for a wrapped last row | `dashboard.css:1184-1206` |
| `.subtab` is `flex: 1 0 auto` | `dashboard.css:1207-1208` |
| The phone media query used for sub-tab sizes: `(pointer: coarse) and (max-width: 599px), (pointer: coarse) and (max-height: 500px)` | `dashboard.css:1260-1262` |
| The once-only note today: `takeSidewaysToast(storage, memo)` with key `dogmud.sidewaysToast` and `memo.shown`; tested | `static/js/viewportfit.js:66-79`; `tools/jstest/viewportfit.test.js:315-329` |
| `_watchSideways` shows it with `this._toast(VF.SIDEWAYS_TOAST, { ms: 4000, lifted: true })`; `_toast(msg, opts)` exists | `dashboard.js:342-362`, `:325-338` |
| `_openDrawer` ends by calling `this._scrollChatToNewest(panel)` | `dashboard.js:704-748` (`:744`) |
| `markInGame()` runs on the first Char push after connecting | `webclient-pure.html:2962-2967` |
| The outer page's `WebclientFrame` has private `readPref` (`'1'`, `'0'`, null), `browserCan`, `shown`, and exposes `start`, `setFullScreen`, `isFullScreen`, `wantsFullScreenOnConnect`, `onChange` | `webclient.html:56-126` |
| The viewport tag is on the outer page: `<meta name="viewport" content="width=device-width, initial-scale=1.0" />` | `_header.html:6` |
| The site's manifest has `"display": "standalone"` (Add to Home Screen opens with no browser bars) | `static/manifest.json:10` |
| Desktop site on a Pixel 8 lays the page out about 980px wide at scale about 0.42 | `viewportfit.js:108-113` (#927) |
| Brand rule today: "at least 44px tall targets on touch screens", with the 30px phone sub-tab exception | `docs/guides/BRAND_STANDARDS.md:110-115` |
| The roadmap's look and feel rules for the deck (kinds by shape and edge, the ◢ mark, 48/8 sizes) | `docs/roadmaps/TOUCH_ARC_ROADMAP.md`, "Look and feel" |
| The owner's Pixel 8 cannot reach a local test server; Malia's S21 can | memory `reference-lan-test-server-access` (2026-11-21) |

## File structure

| File | Change | Responsibility |
|---|---|---|
| `_datafiles/html/public/static/js/viewportfit.js` | Modify | `takeOnce`, `isDesktopSite`, `phoneTip`, the tip words |
| `tools/jstest/viewportfit.test.js` | Modify | Their tests |
| `_datafiles/html/public/static/js/touchdeck.js` | Modify | `rowFade`, `scrollLeftToShow` |
| `tools/jstest/touchdeck.test.js` | Modify | Their tests |
| `_datafiles/html/public/static/css/dashboard.css` | Modify | No Game title bar on phones; one scrolling sub-tab row with fades |
| `_datafiles/html/public/static/js/dashboard.js` | Modify | Sub-tab row wiring; `maybePhoneTip` |
| `_datafiles/html/public/webclient.html` | Modify | Desktop site viewport rewrite; three `WebclientFrame` facts |
| `_datafiles/html/public/webclient-pure.html` | Modify | `markInGame` asks for the tip |
| `tools/webclient-tests/touch-frame.js` | Modify | Static checks for the wiring |
| `docs/guides/BRAND_STANDARDS.md` | Modify | Touch sizes, the deck, button kinds, the ◢ mark |
| `docs/README.md`, `docs/PATCH_NOTES.md` | Modify | Rows and the patch note |

---

### Task 1: `viewportfit.js`, once-only notes, Desktop site and the tip

**Files:**
- Modify: `_datafiles/html/public/static/js/viewportfit.js:66-79`, `:158-174`
- Test: `tools/jstest/viewportfit.test.js` (before the final `console.log`)

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/viewportfit.test.js`, directly before `console.log(failures ? failures + ' checks FAILED' : 'all checks passed');`, add:

```js
// Plan 1c: one rule for every once-only note.
(function () {
    var store = {};
    var ls = { getItem: function (k) { return k in store ? store[k] : null; }, setItem: function (k, v) { store[k] = String(v); } };
    var memo = {};
    check('takeOnce: first time for a key', VF.takeOnce(ls, 'dogmud.tip.fullscreen', memo), true);
    check('takeOnce: not again on this page', VF.takeOnce(ls, 'dogmud.tip.fullscreen', memo), false);
    check('takeOnce: not again on a later visit', VF.takeOnce(ls, 'dogmud.tip.fullscreen', {}), false);
    check('takeOnce: another key is its own', VF.takeOnce(ls, 'dogmud.tip.desktopsite', memo), true);
    check('takeOnce: no storage, once per page', VF.takeOnce(null, 'k', {}), true);
    var m3 = {};
    VF.takeOnce(null, 'k', m3);
    check('takeOnce: no storage, not twice on one page', VF.takeOnce(null, 'k', m3), false);
    check('the sideways note keeps its own key', store['dogmud.sidewaysToast'], undefined);
})();

// Plan 1c: Desktop site is a touch screen laid out at least 15% wider than the screen.
check('Desktop site on a Pixel 8 (980 laid out, 412 screen)', VF.isDesktopSite(980, 412, true), true);
check('an ordinary phone page is not Desktop site', VF.isDesktopSite(412, 412, true), false);
check('a scrollbar or rounding is not Desktop site (14% wider)', VF.isDesktopSite(469, 412, true), false);
check('exactly 15% wider is Desktop site', VF.isDesktopSite(473.8, 412, true), true);
check('a mouse screen is never Desktop site', VF.isDesktopSite(1600, 1366, false), false);
check('no screen width: not Desktop site', VF.isDesktopSite(980, 0, true), false);

// Plan 1c: which tip a phone gets.
function tip(o) { return VF.phoneTip(o); }
var android = { phone: true, desktopSite: false, standalone: false, turnedOff: false, canFullScreen: true, fullScreenOn: false };
check('Android, not in full screen: the full screen tip', tip(android), 'fullscreen');
check('Android, in full screen: no tip', tip(Object.assign({}, android, { fullScreenOn: true })), null);
check('Android, full screen turned off in the game: no tip', tip(Object.assign({}, android, { turnedOff: true })), null);
check('iPhone Safari (no full screen): the Home Screen tip', tip(Object.assign({}, android, { canFullScreen: false })), 'homescreen');
check('opened from the Home Screen: no tip', tip(Object.assign({}, android, { canFullScreen: false, standalone: true })), null);
check('Desktop site comes first', tip(Object.assign({}, android, { desktopSite: true })), 'desktopsite');
check('Desktop site even with full screen turned off', tip(Object.assign({}, android, { desktopSite: true, turnedOff: true })), 'desktopsite');
check('not a phone: no tip', tip(Object.assign({}, android, { phone: false })), null);
check('the full screen words', VF.TIPS.fullscreen, 'Tap ⛶ for more room to play.');
check('the Home Screen words', VF.TIPS.homescreen, 'For more room, add this page to your Home Screen: tap Share, then Add to Home Screen.');
check('the Desktop site words', VF.TIPS.desktopsite, "This page is showing as a desktop site, so everything is small. Turn off Desktop site in your browser's menu.");
check('the tip keys', VF.TIP_KEY_PREFIX, 'dogmud.tip.');
```

- [ ] **Step 2: Run and see them fail**

Run: `node tools/jstest/viewportfit.test.js`
Expected: a `TypeError: VF.takeOnce is not a function`.

- [ ] **Step 3: Implement**

In `viewportfit.js`, replace the sideways block (`:66-79`, from `// The sideways note (layout spec ruling 15), once per browser tab:` through the closing `}` of `takeSidewaysToast`) with:

```js
    // One rule for every once-only note (plan 1c): true the first time key is
    // taken, false after. storage is sessionStorage (once per browser tab: it
    // survives a reload but not a new tab) or localStorage (once per device).
    // memo stops a repeat on this page where storage is missing or blocked.
    function takeOnce(storage, key, memo) {
        if (memo[key]) return false;
        memo[key] = true;
        try {
            if (storage.getItem(key) === '1') return false;
            storage.setItem(key, '1');
        } catch (e) { /* missing or blocked storage: memo alone */ }
        return true;
    }

    // The sideways note (layout spec ruling 15), once per browser tab.
    var SIDEWAYS_TOAST = 'This game is easier to play with your phone held upright.';
    var SIDEWAYS_KEY = 'dogmud.sidewaysToast';
    function takeSidewaysToast(storage, memo) {
        return takeOnce(storage, SIDEWAYS_KEY, memo);
    }

    // Desktop site (touch frame plan 1c): a touch screen whose page is laid
    // out at least 15% wider than the screen, clear of scrollbars and
    // rounding and far below the 2.4x seen on a Pixel 8. layoutW: the page's
    // layout width (documentElement.clientWidth); screenW: screen.width.
    function isDesktopSite(layoutW, screenW, coarse) {
        return !!coarse && screenW > 0 && layoutW >= screenW * 1.15;
    }

    // The phone tips (plan 1c), once per device each. TIP_KEY_PREFIX plus the
    // kind is the localStorage key.
    var TIP_KEY_PREFIX = 'dogmud.tip.';
    var TIPS = {
        fullscreen: 'Tap ⛶ for more room to play.',
        homescreen: 'For more room, add this page to your Home Screen: tap Share, then Add to Home Screen.',
        desktopsite: "This page is showing as a desktop site, so everything is small. Turn off Desktop site in your browser's menu."
    };

    // Which tip a phone gets: 'desktopsite', 'fullscreen', 'homescreen' or
    // null. Desktop site comes first (full screen cannot help a zoomed-out
    // page). No full screen or Home Screen tip from the Home Screen, or for a
    // player who turned full screen off in the game. o: { phone, desktopSite,
    // standalone (opened from the Home Screen), turnedOff, canFullScreen
    // (false on iPhone Safari), fullScreenOn }.
    function phoneTip(o) {
        if (!o.phone) return null;
        if (o.desktopSite) return 'desktopsite';
        if (o.standalone || o.turnedOff) return null;
        if (o.canFullScreen) return o.fullScreenOn ? null : 'fullscreen';
        return 'homescreen';
    }
```

In the returned object (`:158-174`), add after `takeSidewaysToast: takeSidewaysToast,`:

```js
        takeOnce: takeOnce,
        isDesktopSite: isDesktopSite,
        phoneTip: phoneTip,
        TIPS: TIPS,
        TIP_KEY_PREFIX: TIP_KEY_PREFIX,
```

- [ ] **Step 4: Run and see them pass**

Run: `node tools/jstest/viewportfit.test.js`
Expected: `all checks passed`, exit 0 (the existing sideways checks at `:315-329` included).

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/viewportfit.js tools/jstest/viewportfit.test.js
git commit -F - <<'EOF'
feat(webclient): once-only notes share one rule; Desktop site and phone tip decisions (#783)

Touch frame plan 1c, task 1: takeOnce (the sideways note now uses it),
isDesktopSite and phoneTip, with the tip words.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `touchdeck.js`, the sub-tab row's fades and scroll

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js` (after `fieldOf`, and the returned object)
- Test: `tools/jstest/touchdeck.test.js` (before `process.exit`)

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/touchdeck.test.js`, directly before `process.exit(failures ? 1 : 0);`, add:

```js
// Plan 1c: a sub-tab row that scrolls sideways fades at the edge with more.
check('all in view: no fade', TD.rowFade(0, 300, 300), { left: false, right: false });
check('more to the right', TD.rowFade(0, 300, 520), { left: false, right: true });
check('scrolled to the end: more to the left', TD.rowFade(220, 300, 520), { left: true, right: false });
check('in the middle: both', TD.rowFade(100, 300, 520), { left: true, right: true });
check('a pixel of rounding is not more', TD.rowFade(219.5, 300, 520), { left: true, right: false });

// Plan 1c: the open sub-tab scrolls into view.
check('already in view: no scroll', TD.scrollLeftToShow(50, 60, 0, 300), 0);
check('off to the right: scroll until its right edge shows', TD.scrollLeftToShow(400, 80, 0, 300), 180);
check('off to the left: scroll to its left edge', TD.scrollLeftToShow(20, 80, 150, 300), 20);
check('wider than the row: show its start', TD.scrollLeftToShow(400, 350, 0, 300), 400);
```

- [ ] **Step 2: Run and see them fail**

Run: `node tools/jstest/touchdeck.test.js`
Expected: `TypeError: TD.rowFade is not a function`.

- [ ] **Step 3: Implement**

In `touchdeck.js`, directly after the `fieldOf` function's closing `}`, add:

```js
    // A sub-tab row that scrolls sideways on a phone (plan 1c) fades at each
    // edge with more to scroll. Measures from the row element; a pixel of
    // rounding is not more.
    function rowFade(scrollLeft, clientWidth, scrollWidth) {
        return {
            left: scrollLeft > 1,
            right: scrollLeft + clientWidth < scrollWidth - 1
        };
    }

    // The row's scrollLeft that shows an item: itemLeft and itemWidth in the
    // row's scroll coordinates. Unchanged when it is already in view; an item
    // wider than the row shows its start.
    function scrollLeftToShow(itemLeft, itemWidth, scrollLeft, rowWidth) {
        if (itemLeft < scrollLeft || itemWidth >= rowWidth) return itemLeft;
        if (itemLeft + itemWidth > scrollLeft + rowWidth) return itemLeft + itemWidth - rowWidth;
        return scrollLeft;
    }
```

In the returned object, change `shape: shape, slotShows: slotShows, fieldOf: fieldOf,` to:

```js
        shape: shape, slotShows: slotShows, fieldOf: fieldOf,
        rowFade: rowFade, scrollLeftToShow: scrollLeftToShow,
```

In the header comment list, after the `fieldOf` line, add:

```js
//   rowFade / scrollLeftToShow  a phone's sideways-scrolling sub-tab row (plan 1c)
```

- [ ] **Step 4: Run and see them pass**

Run: `node tools/jstest/touchdeck.test.js` then `node tools/jstest/touch-targets.test.js`
Expected: every line `ok`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck.test.js
git commit -F - <<'EOF'
feat(webclient): fades and scroll for a phone's sideways sub-tab row (#783)

Touch frame plan 1c, task 2: rowFade and scrollLeftToShow.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: The stylesheet and the page wiring

**Files:**
- Modify: `_datafiles/html/public/static/css/dashboard.css:1184-1206` and the phone media block at `:1260`
- Modify: `_datafiles/html/public/static/js/dashboard.js` (init, `_openDrawer`, new methods)
- Modify: `_datafiles/html/public/webclient.html:56-131`
- Modify: `_datafiles/html/public/webclient-pure.html:2962-2967`
- Test: `tools/webclient-tests/touch-frame.js`

- [ ] **Step 1: Write the failing static checks**

In `tools/webclient-tests/touch-frame.js`, directly before the summary line `console.log(fails === 0 ? '\nALL CHECKS PASSED' : ...`, add:

```js
// Plan 1c: phone polish.
const outer = fs.readFileSync(path.join(root, 'webclient.html'), 'utf8');
// Phones only: the rule must sit inside the phone media query, because an
// upright tablet is in phone mode too and keeps its title bar.
const phoneBlock = (css.match(/@media \(pointer: coarse\) and \(max-width: 599px\), \(pointer: coarse\) and \(max-height: 500px\) \{[\s\S]*?\n\}/) || [''])[0];
check(/#dashboard\[data-mode="phone"\] #panel-feed > \.dash-panel-head \{ display: none; \}/.test(phoneBlock), 'no Game title bar on a phone, inside the phone media query');
check(!/^#dashboard\[data-mode="phone"\] #panel-feed > \.dash-panel-head/m.test(css), 'the title bar rule is not outside the phone media query (tablets keep it)');
check(/\.subtabs \{[^}]*flex-wrap: nowrap;[^}]*overflow-x: auto;/.test(css), 'a phone keeps sub-tabs in one sideways-scrolling row');
check(/\.subtabs\.fade-left/.test(css) && /\.subtabs\.fade-right/.test(css), 'the row fades at the edge with more to scroll');
check(/TD\.rowFade\(/.test(dash) && /TD\.scrollLeftToShow\(/.test(dash), 'dashboard.js wires the row fades and scrolls the open sub-tab into view');
check(/maybePhoneTip: function/.test(dash) && /VF\.takeOnce\(/.test(dash) && /VF\.phoneTip\(/.test(dash), 'dashboard.js shows a phone tip once per device');
check(/Dashboard\.maybePhoneTip\(\)/.test(html), 'entering the world asks for the phone tip');
check(/ViewportFit\.isDesktopSite\(/.test(outer) && /setAttribute\('content', 'width=' \+ screen\.width/.test(outer), 'the outer page undoes Desktop site where it can');
check(/if \(!ViewportFit\.isPhone\(screen\.width, screen\.height\)\) return;\s*if \(!ViewportFit\.isDesktopSite\(/.test(outer), 'only a phone undoes Desktop site (tablets keep today\'s layout)');
check(/desktopSite: function/.test(outer) && /turnedOff: function/.test(outer) && /canFullScreen: browserCan/.test(outer), 'the outer page tells the game what the tip needs');
check(!/so every\s+Chat channel stays in sight on a phone/.test(css), 'the 2026-11-07 wrap comment is rewritten');
```

- [ ] **Step 2: Run and see them fail**

Run: `node tools/webclient-tests/touch-frame.js`
Expected: ten `FAIL` lines, exit 1. One of the eleven new checks passes already: "the title bar rule is not outside the phone media query", since no such rule exists yet; it guards against the rule being placed outside the phone block.

- [ ] **Step 3: The stylesheet**

In `dashboard.css`, replace the comment above `.subtabs {` (`:1184-1189`, from `/* One look for every tab strip inside a panel:` through `...the bigger text carries it. */`) with:

```css
/* One look for every tab strip inside a panel: brass buttons that look
   tappable, Comms's reading size, nothing dimmed. The open tab is pressed in.
   On a mouse screen or a tablet the buttons fill the row and wrap to more
   rows. On a phone they sit in one row that scrolls sideways (roadmap
   2026-11-19, touch frame plan 1c), which replaces the 2026-11-07 wrapping
   there: two or three rows of channels took most of an open Comms.
   Measured on the shiny brass with the darker text #1a1206 (#774): label 13.77
   on the light spot, 5.45 mid, 3.08 at the dark edge; the bigger text carries it. */
```

Inside the phone media block that starts `@media (pointer: coarse) and (max-width: 599px), (pointer: coarse) and (max-height: 500px) {` (`:1260`), after the line `.subtab { min-height: 30px; padding: 2px 8px; font-size: var(--text); }`, add:

```css
  /* One row that scrolls sideways (plan 1c). The fade shows which edge has
     more (dashboard.js sets fade-left / fade-right from touchdeck.js
     rowFade); the scrollbar is hidden, the fade stands in for it. */
  .subtabs { flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; overscroll-behavior-x: contain; }
  .subtabs::-webkit-scrollbar { display: none; }
  .subtabs::after { display: none; }
  .subtabs.fade-right {
    -webkit-mask-image: linear-gradient(to right, #000 calc(100% - 28px), transparent);
            mask-image: linear-gradient(to right, #000 calc(100% - 28px), transparent);
  }
  .subtabs.fade-left {
    -webkit-mask-image: linear-gradient(to left, #000 calc(100% - 28px), transparent);
            mask-image: linear-gradient(to left, #000 calc(100% - 28px), transparent);
  }
  .subtabs.fade-left.fade-right {
    -webkit-mask-image: linear-gradient(to right, transparent, #000 28px, #000 calc(100% - 28px), transparent);
            mask-image: linear-gradient(to right, transparent, #000 28px, #000 calc(100% - 28px), transparent);
  }
```

Still inside that media block, after the fade rules above, add:

```css
  /* Phones (touch frame plan 1c): no Game title bar. It holds only the
     pop-out button, which does nothing outside wide, and costs about 1.4
     rows of game text. The sight colour is on the game window's border, so
     it stays. Inside this phone media query on purpose: an upright tablet
     is in phone mode too (no room beside the game), and keeps its bar. */
  #dashboard[data-mode="phone"] #panel-feed > .dash-panel-head { display: none; }
```

- [ ] **Step 4: `dashboard.js`, the sub-tab rows**

After the `_watchSideways` method's closing `},` (`:362`), add:

```js
    // A phone's sub-tab rows scroll sideways (plan 1c): each fades at the
    // edge with more (touchdeck.js rowFade). Off a phone the classes are set
    // too but the stylesheet only draws them on a phone.
    _fadeRow: function (row) {
      var TD = window.TouchDeck;
      if (!TD || !row) return;
      var f = TD.rowFade(row.scrollLeft, row.clientWidth, row.scrollWidth);
      row.classList.toggle("fade-left", f.left);
      row.classList.toggle("fade-right", f.right);
    },
    _watchSubtabRows: function () {
      var self = this;
      var rows = document.querySelectorAll(".subtabs");
      rows.forEach(function (row) {
        row.addEventListener("scroll", function () { self._fadeRow(row); }, { passive: true });
        self._fadeRow(row);
      });
      window.addEventListener("resize", function () { rows.forEach(function (r) { self._fadeRow(r); }); });
    },
    // The open sub-tab in a panel's row scrolls into view (plan 1c).
    _showOpenSubtab: function (panel) {
      var TD = window.TouchDeck;
      var row = panel ? panel.querySelector(".subtabs") : null;
      var item = row ? row.querySelector(".subtab.active") : null;
      if (!TD || !row) return;
      if (item) {
        var left = item.getBoundingClientRect().left - row.getBoundingClientRect().left + row.scrollLeft;
        row.scrollLeft = TD.scrollLeftToShow(left, item.offsetWidth, row.scrollLeft, row.clientWidth);
      }
      this._fadeRow(row);
    },
```

In `init`, directly after the line `this._watchSideways && this._watchSideways();` (`:249`), add:

```js
      this._watchSubtabRows();
```

In `_openDrawer`, directly after `this._scrollChatToNewest(panel);` (`:744`), add:

```js
      this._showOpenSubtab(panel);
```

- [ ] **Step 5: `dashboard.js`, the phone tip**

Directly after the `_showOpenSubtab` method added in Step 4, add:

```js
    // The phone tips (plan 1c): once per device each, a few seconds after a
    // character enters the world, so the Enter Game tap's full screen has
    // taken hold first. Called by webclient-pure.html markInGame. The outer
    // page (webclient.html WebclientFrame) knows full screen and Desktop
    // site; opened on its own this page shows no tip.
    _tipMemo: {},
    maybePhoneTip: function () {
      var self = this;
      var VF = window.ViewportFit;
      if (!VF) return;
      setTimeout(function () {
        var outer = null, win = null;
        try {
          win = window.parent !== window ? window.parent : null;
          outer = win ? win.WebclientFrame : null;
        } catch (e) { return; }
        if (!outer || !outer.desktopSite) return;
        var coarse = !!(window.matchMedia && window.matchMedia("(pointer: coarse)").matches);
        var standalone = !!((win.matchMedia && win.matchMedia("(display-mode: standalone)").matches) ||
                            win.navigator.standalone === true);
        var kind = VF.phoneTip({
          phone: VF.isPhone(screen.width, screen.height) && coarse,
          desktopSite: outer.desktopSite(),
          standalone: standalone,
          turnedOff: outer.turnedOff(),
          canFullScreen: outer.canFullScreen(),
          fullScreenOn: outer.isFullScreen()
        });
        if (!kind) return;
        var store = null;
        try { store = window.localStorage; } catch (e) { /* blocked: memo alone */ }
        if (!VF.takeOnce(store, VF.TIP_KEY_PREFIX + kind, self._tipMemo)) return;
        self._toast(VF.TIPS[kind], { ms: 6000, lifted: true });
      }, 3000);
    },
```

- [ ] **Step 6: `webclient-pure.html`, ask for the tip**

In `markInGame()` (`:2962-2967`), after `if (!leavingGame) setSessionState('connected');`, add:

```js
            // The phone tips (touch frame plan 1c), once per device.
            if (window.Dashboard && Dashboard.maybePhoneTip) Dashboard.maybePhoneTip();
```

- [ ] **Step 7: `webclient.html`, Desktop site and three facts**

In `webclient.html`, in the `WebclientFrame` returned object (`:118-126`), after `isFullScreen: shown,` add:

```js
            // For the game's phone tips (touch frame plan 1c).
            canFullScreen: browserCan,
            turnedOff: function () { return readPref() === '0'; },
            desktopSite: function () {
                const coarse = !!(window.matchMedia && window.matchMedia('(pointer: coarse)').matches);
                return ViewportFit.isDesktopSite(document.documentElement.clientWidth, screen.width, coarse);
            },
```

Directly before `document.addEventListener("DOMContentLoaded", function () { WebclientFrame.start(); });` (`:131`), add:

```js
    // Desktop site (touch frame plan 1c, D): a phone laid out much wider
    // than itself asks for the screen's real width. Where the browser
    // honours it the page fits again; where it does not, the game shows the
    // Desktop site tip (A) once per device, since desktopSite() stays true.
    // Phones only: Android tablets use Desktop site by default
    // (viewportfit.js visibleHeight) and keep today's wide layout.
    (function () {
        const coarse = !!(window.matchMedia && window.matchMedia('(pointer: coarse)').matches);
        if (!ViewportFit.isPhone(screen.width, screen.height)) return;
        if (!ViewportFit.isDesktopSite(document.documentElement.clientWidth, screen.width, coarse)) return;
        const meta = document.querySelector('meta[name="viewport"]');
        if (meta) meta.setAttribute('content', 'width=' + screen.width + ', initial-scale=1');
    })();
```

- [ ] **Step 8: Run every client test**

Run:
```bash
node tools/webclient-tests/touch-frame.js
for f in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$f" >/dev/null 2>&1 || echo "FAIL $f"; done; echo done
for f in $(git diff --name-only master -- '*.js'); do node --check "$f" || echo "BAD $f"; done
```
Expected: touch-frame.js `ALL CHECKS PASSED`; the loop prints only `done`; no `BAD`.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/webclient.html _datafiles/html/public/webclient-pure.html tools/webclient-tests/touch-frame.js
git commit -F - <<'EOF'
feat(webclient): phone polish: no Game title bar, one sub-tab row, tips, Desktop site (#783)

Touch frame plan 1c, task 3: on a phone the Game title bar goes, sub-tabs
sit in one sideways-scrolling row with edge fades and the open one in
view, a once-per-device tip points to full screen or the Home Screen, and
Desktop site is undone where the browser allows (a tip where it does not).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: The brand doc

**Files:**
- Modify: `docs/guides/BRAND_STANDARDS.md:110-115`

- [ ] **Step 1: Replace the Touch bullet** (`:110-115`, from `- **Touch:** at least 44px tall targets on touch screens.` through `Held\n  sideways, the tab name pill is also hidden.`) with:

```markdown
- **Touch:** 48px targets with 8px gaps on touch screens (touch arc,
  2026-11-21; was 44px). Below 360px wide the deck's buttons are 44px with
  5px gaps, so every button keeps its slot on every phone. One exception
  (Calabe and Malia, 2026-11-07, tested on her phone; kept 2026-11-19): on a
  phone (touch, under 600px wide or under 500px tall, either way up) the
  sub-tabs and the panel title buttons are 30px, still above the 24px
  minimum target (WCAG 2.5.8). On a phone the sub-tabs sit in one row that
  scrolls sideways and fades at the edge with more. Held sideways, the tab
  name pill is also hidden.
- **The touch deck** (touch arc spec 1): a block of buttons above the
  command box, in the slot under the game text; an open tab takes its place.
  A divider line sits between it and the command box. Every button is
  brass. **Kinds are told apart by shape and edge, never by colour alone:**
  movement square; contextual commands rounded; other exits dashed edge;
  Back and More heavy square edge. Flee looks like any other command. A
  small **◢** in the corner marks a button that opens a deeper level or does
  more on a long press. Labels at the client's text size, JetBrains Mono,
  regular weight. Red only for warnings; green and blue never carry meaning.
```

- [ ] **Step 2: Commit**

```bash
git add docs/guides/BRAND_STANDARDS.md
git commit -F - <<'EOF'
docs(brand): touch targets 48px, the deck, button kinds and the ◢ mark (#783)

Touch frame plan 1c, task 4, from the touch arc roadmap's look and feel.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Headless check (controller, not a subagent)

- [ ] **Step 1: Servers.** This branch and master in detached worktrees on private ports, instance saves wiped, logins through real character creation. Note the PIDs; kill only those.
- [ ] **Step 2: Title bar.** S21 (`{"width":360,"height":800,"dpr":3,"mobile":true,"touch":true}`), deck Automatic, no tab: `term.rows` on this branch is more than master's (expected about 1 more row; spec estimate 1.4), and `getComputedStyle(document.querySelector('#panel-feed > .dash-panel-head')).display` is `none`. Sideways (800x360): also `none`. 1366x768: the bar shows, as on master.
- [ ] **Step 3: Sub-tab row.** S21, Comms open: the row's height equals one sub-tab's height plus its padding (one row), `scrollWidth > clientWidth`, the row has `fade-right`; scroll it to the end, it has `fade-left` and not `fade-right`. Click Trade, close Comms, open Comms: Trade's rectangle lies inside the row's. A tablet (820x1180) and 1366x768: the row still wraps (`flex-wrap: wrap`), as on master.
- [ ] **Step 4: Desktop unchanged.** 1366x768 and 1920x1080, branch against master: frames match apart from game text.
- [ ] **Step 4b: Tablets unchanged** (owner, 2026-11-21). Upright iPad 820x1180 (phone mode), sideways iPad 1180x820 (column), open foldable 690x840 and a small Android tablet 600x960, all touch, branch against master with Comms open: the Game title bar shows (`display` is not `none`), the sub-tab row wraps (`flex-wrap: wrap`, no `fade-*` drawn), `term.rows` and the frame match master apart from game text, and the outer page's viewport tag is still `width=device-width, initial-scale=1.0` (read it from `/webclient` with a plain fetch: the game page runs on its own in headless).
- [ ] **Step 5: The tips.** Headless runs the game page on its own (no outer page), so the tip cannot show there; it is covered by the jstests (Task 1) and the owner's check. Confirm only that `Dashboard.maybePhoneTip` exists and that nothing throws when it runs on its own page.
- [ ] **Step 6: Stop the servers by PID; remove the worktrees.**

---

### Task 6: Docs and the owner's check

- [ ] **Step 1: README.** (The plan's own row was added with the plan.) In the `tools/jstest/` row's `touchdeck.test.js` entry, after `the keyboard rule and \`fieldOf\` (plan 1b)`, add `, the sub-tab row's fades and scroll (plan 1c)`.

- [ ] **Step 2: Patch note.** At the top of `docs/PATCH_NOTES.md`, under `# DOGMud Patch Notes` (date the PR merges):

```markdown
## 2026-11-21: More room on a phone

- On a phone the Game window no longer has a title bar, so you see more of the game.
- Comms, Inventory, Quests and Automation keep their buttons in one row that you swipe sideways. The row fades at the edge that has more.
- A phone gets one tip, once: tap the full screen button on Android, or add the game to your Home Screen on an iPhone.
- If your browser shows the game as a desktop site, the game asks for your phone's real width, and tells you how to turn Desktop site off if that does not work.
```

- [ ] **Step 3: Commit**

```bash
git add docs/README.md docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs: touch frame plan 1c's rows and patch note (#783)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

- [ ] **Step 4: The owner's check (after the PR is green, before merge).** On Malia's S21 (the Pixel 8 cannot reach a local test server), ordinary play only, logins made through real creation, on a fresh browser profile or after clearing the site's data (the tips are once per device):

1. Enter the game without full screen (turn it off with the session bar's button first, then reload and enter). After a few seconds: "Tap ⛶ for more room to play." Reload and enter again: no tip.
2. Full screen on. The Game window has no title bar.
3. Open Comms. The channel buttons sit in one row; swipe it sideways; the edge with more channels fades.
4. Tap Trade, close Comms, open Comms: Trade is in view.
5. Chrome menu, turn on Desktop site. The game should fit the screen again; if it is still small, the Desktop site tip shows once. Record which on #834 (this decides whether D works).

---

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| `takeSidewaysToast` becomes a call to a general `takeOnce`, used by the tips too | One once-only rule instead of a second copy (owner rule, 2026-11-21) | Keep the sideways note's own code |
| **Tablets keep today's experience** (owner asked, 2026-11-21): the title bar rule sits inside the phone media query (an upright tablet is in phone mode and keeps its bar), Desktop site is undone only on a phone (Android tablets use Desktop site by default and keep today's wide layout), the sub-tab row and the tips were already phone-only | The spec's polish is for phones; a tablet change nobody asked for is a regression | Extend any of the four to tablets in a later plan |
| D and A ship together: the viewport rewrite runs, and the Desktop site tip shows only while the page is still laid out wide | One PR covers both outcomes of the Pixel test; A never shows where D works | Ship D alone and add A after the device test |
| The Desktop site device check is on the S21 | The owner's Pixel 8 cannot reach a local test server (2026-11-21) | Test on the Pixel after deploy |
| All four sub-tab rows (Comms, Inventory, Quests, Automation) scroll on a phone, not only Comms and Inventory | They share `.subtabs`; one rule for one control | Scope the rule to `#inv-tabs` and the Comms row |
| The tip shows 3 seconds after a character enters the world | The Enter Game tap's full screen must take hold first, or the full screen tip would show to everyone | Show it at page load |
| iPhone Safari is recognised as "cannot go full screen" (`canFullScreen` false), not by its user agent | The page already knows this (`browserCan`); a user agent test drifts | Sniff the user agent |
| "Opened from the Home Screen" is `(display-mode: standalone)` or `navigator.standalone` on the outer page | The manifest's standalone mode and iOS's own flag | Either one alone |
| Tips last 6 seconds, lifted above the command box like the sideways note | Longer words than the sideways note's | 4 seconds |
| The tips are not shown in headless runs (no outer page) and are covered by jstests and the owner's check | The webshot tool drives the game page on its own | Extend the tool to drive the outer page |
