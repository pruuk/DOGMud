# Touch frame plan 1b (the keyboard) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Edit files with the Edit and Write tools only (guard.py H3); never write a repo file from the shell.

**Spec:** `docs/superpowers/specs/2026-11-21-touch-frame-design.md`, section "The keyboard (plan 1b)" (approved 2026-11-21; ruled 2026-11-19 and kept by the owner on 2026-11-21). Plan 1a is merged (#932).

**Goal:** On a phone, typing in the command box makes an open tab step aside so the game text gets the room; when the keyboard closes the same tab comes back, scrolled where it was (Comms at its newest message). Typing in any other field (the Automation dialog) moves nothing.

**Architecture:** The decision stays one pure function, `TouchDeck.slotShows`, which gains the field being typed in (`TouchDeck.fieldOf`, built on the existing `ViewportFit.editableFocused`). `dashboard.js applyDeck` already reruns on every keyboard change; it gains a focus watch (focus can move between fields while the keyboard stays up) and keeps the stepped-aside tab's scroll place with two small helpers in `touchdeck.js`. Client only.

**Tech Stack:** Plain browser JS (no build), zero-dependency Node tests (`tools/jstest`, `tools/webclient-tests`), headless Edge shots (`tools/webshot/live-shot.js`).

---

## Facts verified against source (2026-11-21, master `bad2aeba8`)

| Fact | Source |
|---|---|
| `slotShows(o)` returns `'tab'` whenever `o.phone && o.tabOpen`, keyboard or not; its comment says plan 1b changes the keyboard case | `static/js/touchdeck.js:88-97` |
| The jstest pins today's keyboard case: `'phone, a tab open, keyboard up: the tab stays (plan 1b changes this)'` | `tools/jstest/touchdeck.test.js:84` (helper `shows` at `:79-81`) |
| `applyDeck` reads `keyboard-open` from the root, calls `slotShows`, then sets `slot.dataset.shows` and `slot.hidden` | `static/js/dashboard.js:547-565` |
| A MutationObserver reruns `applyDeck` whenever `keyboard-open` flips; nothing reruns it when focus moves while the keyboard stays up | `dashboard.js:222-233` |
| `keyboard-open` is set by the outer page when a text field has focus and the visible area is over 120px short | `webclient.html:24-47`; `static/js/viewportfit.js:124-126` (`keyboardOpen`) |
| `ViewportFit.editableFocused(doc)` already decides whether the focused element takes typed text | `viewportfit.js:128-138` |
| The command box is `#command-input` | `webclient-pure.html:336` |
| The Close keyboard X blurs the command box | `webclient-pure.html:339`, `:3456-3466` |
| Hiding a panel loses its scroll place; `_scrollChatToNewest` puts the active Comms channel at its newest message | `dashboard.js:730-736`; called from `_openDrawer` at `:724` |
| `_openDrawer` sets `_drawerActive` and calls `applyDeck`; `_closeDrawer` clears it and calls `applyDeck` | `dashboard.js:684-728`, `:738-758` |
| The card's panel lives in `this._drawerBody` | `dashboard.js:688`, `:702` |
| With the keyboard up the tab bar is hidden, so no tab can be tapped while it is up | `static/css/dashboard.css:920` |
| The 30%-above-the-keyboard cap (`html.keyboard-open #dash-drawer`) applies to phone and slim | `dashboard.css:926-936` |
| After a chat line the keyboard stays up on a phone (`isChatLine`), after other commands it waits for the answer | `static/js/hotinput.js:160-181` |
| No scroll save/restore helper exists in the client (`grep -i "savedscroll\|restoreScroll\|rememberScroll\|scrollPos"`: none; the same search finds the three `scrollTop` sites) | `static/js/*.js`, `webclient-pure.html` |
| `tools/webclient-tests/touch-frame.js` is static analysis of `webclient-pure.html`, `dashboard.css`, `dashboard.js` | `tools/webclient-tests/touch-frame.js:1-20` |

## File structure

| File | Change | Responsibility |
|---|---|---|
| `_datafiles/html/public/static/js/touchdeck.js` | Modify | `fieldOf`, the keyboard case in `slotShows`, `rememberScroll` / `restoreScroll` |
| `tools/jstest/touchdeck.test.js` | Modify | Cases for all three |
| `_datafiles/html/public/static/js/dashboard.js` | Modify | `applyDeck` passes the field and keeps the scroll place; a focus watch; `_closeDrawer` forgets it |
| `_datafiles/html/public/static/css/dashboard.css` | Modify | Comment on the keyboard cap only |
| `tools/webclient-tests/touch-frame.js` | Modify | Static checks for the wiring |
| `docs/README.md`, `docs/PATCH_NOTES.md` | Modify | Rows and the patch note |

---

### Task 1: `touchdeck.js`, the field and the keyboard rule

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js:88-104`
- Test: `tools/jstest/touchdeck.test.js:79-89`

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/touchdeck.test.js`, replace the `shows` helper and the eight `check` lines after it (`:79-89`) with:

```js
function shows(phone, tabOpen, deckOn, keyboard, field) {
    return TD.slotShows({ phone: phone, tabOpen: tabOpen, deckOn: deckOn, keyboard: keyboard, field: field });
}
check('phone, a tab open: the tab', shows(true, true, true, false, null), 'tab');
check('phone, a tab open, deck off: the tab', shows(true, true, false, false, null), 'tab');
// Plan 1b: typing in the command box sets the tab aside; the game text gets the room.
check('phone, a tab open, typing in the command box: empty', shows(true, true, true, true, 'command'), 'empty');
check('phone, a tab open, deck off, typing in the command box: empty', shows(true, true, false, true, 'command'), 'empty');
check('phone, a tab open, typing in another field: the tab stays', shows(true, true, true, true, 'other'), 'tab');
check('phone, a tab open, command box focused but no keyboard (a hardware keyboard): the tab', shows(true, true, true, false, 'command'), 'tab');
check('deck on, nothing open: the deck', shows(true, false, true, false, null), 'deck');
check('deck on, keyboard up: empty (the keyboard covers the deck)', shows(true, false, true, true, 'command'), 'empty');
check('deck on, keyboard up for another field: empty', shows(true, false, true, true, 'other'), 'empty');
check('deck off, nothing open: empty', shows(true, false, false, false, null), 'empty');
check('not a phone: a tab is a card elsewhere, the slot keeps the deck', shows(false, true, true, false, null), 'deck');
check('not a phone, typing in the command box: empty (the keyboard covers the deck)', shows(false, true, true, true, 'command'), 'empty');
check('not a phone, deck off: empty', shows(false, true, false, false, null), 'empty');

// Which field the player is typing in (plan 1b).
check('nothing editable has focus: no field', TD.fieldOf('command-input', false), null);
check('the command box', TD.fieldOf('command-input', true), 'command');
check('another text field', TD.fieldOf('auto-modal-name', true), 'other');
check('a text field with no id', TD.fieldOf('', true), 'other');

// The scroll place of a tab that steps aside (plan 1b).
function el(top) { return { scrollTop: top }; }
function box(own, kids) {
    return { scrollTop: own, querySelectorAll: function (sel) { return sel === '*' ? kids : []; } };
}
var a = el(120), b = el(0), c = el(37);
var root = box(0, [a, b, c]);
var saved = TD.rememberScroll(root);
check('only scrolled elements are kept', saved.length, 2);
a.scrollTop = 0; c.scrollTop = 0;
TD.restoreScroll(saved);
check('a scrolled element comes back to its place', a.scrollTop, 120);
check('a second one too', c.scrollTop, 37);
check('an unscrolled one stays at the top', b.scrollTop, 0);
var scrolledRoot = box(55, []);
var savedRoot = TD.rememberScroll(scrolledRoot);
scrolledRoot.scrollTop = 0;
TD.restoreScroll(savedRoot);
check('the root itself is kept', scrolledRoot.scrollTop, 55);
check('no root: nothing kept', TD.rememberScroll(null).length, 0);
TD.restoreScroll(null);
check('restoring nothing does nothing', true, true);
```

- [ ] **Step 2: Run the tests and see them fail**

Run: `node tools/jstest/touchdeck.test.js`
Expected: FAIL lines for the `'command'` cases (`got "tab", want "empty"`), and a `TypeError` (`TD.fieldOf is not a function`) that stops the run.

- [ ] **Step 3: Implement**

In `touchdeck.js`, replace the `slotShows` block (`:88-97`) with:

```js
    // What the slot under the game text holds: 'tab', 'deck' or 'empty'.
    // o.phone: the phone layout, where an open tab lives in the slot.
    // o.tabOpen, o.deckOn, o.keyboard: the on-screen keyboard is up.
    // o.field: where the player is typing (fieldOf). Typing in the command
    // box sets the tab aside so the game text gets the room (plan 1b, ruled
    // 2026-11-19); typing in another field (the Automation dialog) moves
    // nothing. The keyboard covers the deck either way.
    function slotShows(o) {
        if (o.phone && o.tabOpen) {
            return o.keyboard && o.field === 'command' ? 'empty' : 'tab';
        }
        if (o.deckOn && !o.keyboard) return 'deck';
        return 'empty';
    }

    // Where the player is typing: 'command' for the command box, 'other' for
    // any other text field, null when nothing that takes text has focus.
    // activeId: document.activeElement's id. editable: ViewportFit.
    // editableFocused(document), the page's one test for a text field.
    var COMMAND_ID = 'command-input';
    function fieldOf(activeId, editable) {
        if (!editable) return null;
        return activeId === COMMAND_ID ? 'command' : 'other';
    }

    // A tab that steps aside is hidden, and a hidden box loses its scroll
    // place (dashboard.js _scrollChatToNewest). rememberScroll lists root and
    // every element under it that is scrolled down; restoreScroll puts each
    // back. root: anything with scrollTop and querySelectorAll.
    function rememberScroll(root) {
        var saved = [];
        if (!root) return saved;
        if (root.scrollTop > 0) saved.push({ el: root, top: root.scrollTop });
        var all = root.querySelectorAll ? root.querySelectorAll('*') : [];
        for (var i = 0; i < all.length; i++) {
            if (all[i].scrollTop > 0) saved.push({ el: all[i], top: all[i].scrollTop });
        }
        return saved;
    }

    function restoreScroll(saved) {
        if (!saved) return;
        for (var i = 0; i < saved.length; i++) saved[i].el.scrollTop = saved[i].top;
    }
```

and replace the `return { ... }` block (`:99-104`) with:

```js
    return {
        KEY: KEY, AUTOMATIC_SHOWS: AUTOMATIC_SHOWS,
        readPref: readPref, writePref: writePref, deckOn: deckOn,
        shape: shape, slotShows: slotShows, fieldOf: fieldOf,
        rememberScroll: rememberScroll, restoreScroll: restoreScroll,
        BTN: BTN, GAP: GAP, SMALL_BTN: SMALL_BTN, SMALL_GAP: SMALL_GAP, PAD: PAD
    };
```

In the header comment list (`:10-13`), add after the `slotShows` line:

```js
//   fieldOf               where the player is typing (plan 1b)
//   rememberScroll / restoreScroll  a stepped-aside tab's scroll place
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `node tools/jstest/touchdeck.test.js` then `node tools/jstest/touch-targets.test.js`
Expected: every line `ok`, exit 0, for both.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck.test.js
git commit -F - <<'EOF'
feat(webclient): typing in the command box sets a phone's tab aside (#783)

Touch frame plan 1b, task 1: slotShows takes the field being typed in
(fieldOf), and two helpers keep a stepped-aside tab's scroll place.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `dashboard.js`, the wiring

**Files:**
- Modify: `_datafiles/html/public/static/js/dashboard.js:222-233`, `:547-565`, `:738-758`
- Modify: `_datafiles/html/public/static/css/dashboard.css:926-932` (comment only)
- Test: `tools/webclient-tests/touch-frame.js`

- [ ] **Step 1: Write the failing static checks**

In `tools/webclient-tests/touch-frame.js`, directly before the summary line `console.log(fails === 0 ? '\nALL CHECKS PASSED' : ...` (the second-to-last line), add:

```js
// Plan 1b: the keyboard rule.
check(/slotShows\(\{[\s\S]{0,300}field:/.test(dash), 'applyDeck passes the field being typed in to slotShows');
check(/TD\.fieldOf\(/.test(dash) && /editableFocused\(document\)/.test(dash), 'the field comes from fieldOf and ViewportFit.editableFocused');
check(/addEventListener\("focusin"/.test(dash) && /addEventListener\("focusout"/.test(dash), 'a focus watch reruns applyDeck when focus moves');
check(/TD\.rememberScroll\(this\._drawerBody\)/.test(dash) && /TD\.restoreScroll\(/.test(dash), 'a stepped-aside tab keeps its scroll place');
check(/_closeDrawer: function \(\) \{[\s\S]{0,400}this\._aside = null/.test(dash), 'closing a tab forgets a kept scroll place');
```

- [ ] **Step 2: Run and see them fail**

Run: `node tools/webclient-tests/touch-frame.js`
Expected: five `FAIL` lines for the checks above, exit non-zero.

- [ ] **Step 3: The focus watch**

In `dashboard.js`, after the MutationObserver block (`:225-233`, ending `}).observe(...); }`), add:

```js
      // Plan 1b: what steps aside depends on which field is being typed in,
      // and focus can move between fields (the command box, the Automation
      // dialog) while the keyboard stays up, which the class watch above
      // does not see.
      var onFocusMove = function () {
        if (document.documentElement.classList.contains("keyboard-open")) self.applyDeck();
      };
      document.addEventListener("focusin", onFocusMove);
      document.addEventListener("focusout", onFocusMove);
```

- [ ] **Step 4: `applyDeck`**

Replace `dashboard.js:555-563` (from `var keyboard = ...` through `slot.hidden = shows === "empty";`) with:

```js
      var keyboard = document.documentElement.classList.contains("keyboard-open");
      var active = document.activeElement;
      var field = TD.fieldOf(active ? active.id : "", !!(VF && VF.editableFocused(document)));
      var shows = TD.slotShows({
        phone: this.mode === "phone",
        tabOpen: !!this._drawerActive,
        deckOn: on,
        keyboard: keyboard,
        field: field
      });
      var was = slot.dataset.shows;
      // Plan 1b: a tab that steps aside for the keyboard keeps its scroll
      // place, since a hidden box loses it.
      if (was === "tab" && shows !== "tab" && this._drawerActive) {
        this._aside = { name: this._drawerActive, scroll: TD.rememberScroll(this._drawerBody) };
      }
      if (slot.dataset.shows !== shows) slot.dataset.shows = shows;
      slot.hidden = shows === "empty";
      // Back in the slot: the same tab returns to its place, and Comms to its
      // newest message, as when a tab opens.
      if (shows === "tab" && was !== "tab" && this._aside) {
        if (this._aside.name === this._drawerActive) {
          TD.restoreScroll(this._aside.scroll);
          this._scrollChatToNewest(document.getElementById("panel-" + this._drawerActive));
        }
        this._aside = null;
      }
```

- [ ] **Step 5: `_closeDrawer` forgets a kept place**

In `_closeDrawer` (`:738-758`), change `this._drawerActive = null;` to:

```js
      this._drawerActive = null;
      this._aside = null;   // a later open starts fresh (plan 1b)
```

- [ ] **Step 6: The stylesheet comment**

In `dashboard.css`, replace the comment above `html.keyboard-open #dash-drawer` (`:926-932`, from `/* With the keyboard up, an open tab sits just above the command box.` through `newest lines. */`) with:

```css
/* With the keyboard up, an open tab sits just above the command box. The tab
   bar and name strip are hidden then. Phone and slim: 30% of what is visible
   above the keyboard (plan 6 R3). On a phone, typing in the command box sets
   the tab aside instead (touch frame plan 1b, touchdeck.js slotShows), so
   there this cap only holds while another field (the Automation dialog) has
   the keyboard. --visible-h is set by the keyboard watch
   (webclient.html fitFrameHeight, or this page's own when opened alone);
   vh cannot be used, because opened alone the page keeps vh at the full
   layout height. Never under 60px. */
```

- [ ] **Step 7: Run every client test**

Run:
```bash
node tools/webclient-tests/touch-frame.js
for f in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$f" >/dev/null 2>&1 || echo "FAIL $f"; done; echo done
for f in $(git diff --name-only master -- '*.js'); do node --check "$f" || echo "BAD $f"; done
```
Expected: touch-frame.js all `OK`; the loop prints only `done`; no `BAD`.

- [ ] **Step 8: Commit**

```bash
git add _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/static/css/dashboard.css tools/webclient-tests/touch-frame.js
git commit -F - <<'EOF'
feat(webclient): a phone's tab steps aside while typing and comes back in place (#783)

Touch frame plan 1b, task 2: applyDeck passes the field being typed in,
keeps the stepped-aside tab's scroll place and restores it (Comms at its
newest message); a focus watch reruns it when focus moves with the
keyboard up.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Headless check (controller, not a subagent)

**Files:** none in the repo; everything in the session scratchpad.

- [ ] **Step 1: Servers.** A test server for this branch and one for master, each in a detached worktree on private ports, instance saves wiped, logins made through real character creation (`new` at the login prompt; memory `feedback-owner-test-logins-via-real-creation`). Note each PID; kill only those.

- [ ] **Step 2: The keyboard, simulated.** With `tools/webshot/live-shot.js` emulating an S21 (`{"width":360,"height":440,"dpr":3,"mobile":true,"touch":true}`, the height left above a keyboard), for each build:

```json
{"eval": "Dashboard._openDrawer('chat'), true"},
{"send": "say hello", "after": 1200},
{"eval": "document.getElementById('command-input').focus(), document.documentElement.classList.add('keyboard-open'), true"},
{"wait": 400},
{"eval": "JSON.stringify({shows: document.getElementById('touch-slot').dataset.shows, rows: term.rows, drawerH: Math.round(document.getElementById('dash-drawer').getBoundingClientRect().height)})"},
{"shot": "<scratch>/<build>-typing.png"}
```

Expected: branch `shows: "empty"`, more `rows` than master; master `shows: "tab"` with the 132px card (the 2026-11-21 measurement).

- [ ] **Step 3: Back in place.** Branch only, at 360 x 800: open Quests (or Inventory), scroll its body down (`document.querySelector('#panel-quests .dash-panel-body').scrollTop = 200`), read the value back, then focus the command box and add `keyboard-open`, then remove the class and blur the box. Read `scrollTop` again.

Expected: the same value both times (or the panel's maximum if it is shorter). With Comms instead: the active channel's `scrollTop + clientHeight` equals its `scrollHeight` (newest message).

- [ ] **Step 4: Another field moves nothing.** Open Automation, open a macro editor (`openAutoModal` path: click `+ New`), focus its text field and add `keyboard-open`. Expected: `shows: "tab"`.

- [ ] **Step 5: Desktop unchanged.** 1366 x 768 and 1920 x 1080, no emulation, branch against master with `keyboard-open` never set: the pair matches and `touch-slot` is hidden.

- [ ] **Step 6: Stop the servers by PID; remove the worktrees** (`git worktree remove`).

If a number misses, fix it in the task that owns it and repeat this task.

---

### Task 4: Docs and the owner's check

**Files:**
- Modify: `docs/README.md`, `docs/PATCH_NOTES.md`

- [ ] **Step 1: README.** In the `touchdeck.js` row (`docs/README.md:346`), change `...and what the slot under the game text holds (the deck, a phone's open tab, or nothing).` to `...and what the slot under the game text holds (the deck, a phone's open tab, or nothing; typing in the command box sets the tab aside, plan 1b), with the stepped-aside tab's scroll place.` In the `tools/jstest/` row's 2026-11-21 entry for `touchdeck.test.js`, add `, the keyboard rule and scroll place (plan 1b)` before its closing parenthesis.

- [ ] **Step 2: Patch note.** At the top of `docs/PATCH_NOTES.md`, under `# DOGMud Patch Notes`, add (player-facing, no numbers, no dashes):

```markdown
## 2026-11-22: Typing on a phone gives the game text the room

- On a phone, when you tap the command box to type, an open tab such as Comms steps aside, so you can read the game above the keyboard. Close the keyboard and the tab comes back where you left it, with Comms at its newest message.
```

(Use the date the PR merges.)

- [ ] **Step 3: Commit**

```bash
git add docs/README.md docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs: touch frame plan 1b's rows and patch note (#783)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

- [ ] **Step 4: The owner's check (after the PR is green, before merge).** On a phone in full screen, ordinary play only, against a local test server with logins made through real creation:

1. Open Comms. Tap the command box. Comms steps aside and the game text fills the space above the keyboard.
2. Type `say hello` and send it. Your line shows in the game text. The keyboard stays up.
3. Close the keyboard (the X beside the command box). Comms comes back at its newest message, with your line in it.
4. Open Quests or Inventory, scroll down, tap the command box, then close the keyboard. The tab comes back where you left it.
5. Open Automation, tap + New, tap its name field. The Automation tab stays where it is.

Record the result on #834.

---

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| The command box is recognised by its id (`command-input`) in a pure `fieldOf`, on top of `ViewportFit.editableFocused` | Reuses the page's one test for a text field; the command box has one fixed id | A data attribute on the input |
| A command box focused with no on-screen keyboard (a hardware keyboard on a phone) keeps the tab | The rule exists to make room above a keyboard; with none there is nothing to make room for | Set the tab aside on focus alone |
| **Changed while building (2026-11-21): no scroll-memory helpers.** Tasks 1 and 2 first built `rememberScroll` / `restoreScroll`. The headless check then showed the card keeps its scroll place on its own: the slot hidden for 0.8s with no restore code, `.drawer-body` scrolled to 300 came back at 300. The fact this plan cited (`dashboard.js:730`) is about moving a panel into the card, not hiding the slot. The helpers were removed; only Comms-to-newest stays | No code without a job (owner rule, 2026-11-21) | If an iPhone (WebKit) loses the place, bring the helpers back from commit `2dbf0aca0` |
| Comms comes back at its newest message even if the player had scrolled up | The spec says so ("as today"), and the new line was just sent | Restore Comms' exact place too |
| The 30% keyboard cap CSS stays; only its comment changes | Slim still uses it, and on a phone it still sizes the tab under the Automation dialog | Remove it for phones by selector |
| The focus watch reruns `applyDeck` only while `keyboard-open` is set | With no keyboard, focus changes cannot change what the slot shows | Rerun on every focus change |
| "The last tab tapped wins" needs no code | The tab bar is hidden while the keyboard is up, so the open tab cannot change until it closes; `_aside` is matched by tab name and cleared on close | Track taps explicitly |
| 1b's headless check simulates the keyboard by a 440px-tall viewport plus the `keyboard-open` class | Headless Edge has no on-screen keyboard; the spec allows this approximation | Wait for the owner's phone only |
