# Phone keyboard hides after each send

> Spec: [`../specs/2026-11-01-webclient-rework-design.md`](../specs/2026-11-01-webclient-rework-design.md),
> "Phone reading", slice A, the keyboard part (Malia, 2026-11-02: "plan to
> have the keyboard hide after each send"). The readable-text part of slice A
> gets its own plan. Status: BAR done (findings at the end, folded in);
> Chat box decided (stays open). Approved by Malia and built 2026-11-02;
> checked on her Android phone.

**Branch:** `feat/game-settings` (PR #768). Files:
`_datafiles/html/public/webclient-pure.html`,
`_datafiles/html/public/static/js/hotinput.js`,
`tools/jstest/hotinput.test.js`. No server work.

## Goal
On a phone, after a player sends a command in the game, the on-screen
keyboard closes so the whole screen shows what happened; tapping the command
box opens it again. Nothing changes on tablets, desktops, laptops, while
logging in, or while a game menu or prompt is waiting for an answer.

## What the code shows
- `textInput` is `#command-input`. Its `keydown` handler (~2984) sends on
  Enter through `SendData()` (returns true when sent, and already calls
  `term.scrollToBottom()`), then clears the box.
- `inGame` (~2602) is true from the first `Char` push after login; reset in
  `onopen` / `onclose`. `menuOpen` (~961) is true while a game menu or prompt
  (`Char.Menu`: shop quantities, yes/no confirmations and similar) is open.
  The box becomes `type="password"` when the server masks input (~2724), in
  the game as well as at login.
- **xterm grabs focus on every press:** its screen element's `mousedown`
  listener runs `preventDefault(); focus()` (`static/js/xterm.4.19.0.js`),
  focusing xterm's hidden helper textarea, which is editable, so a tap on the
  game text opens the phone keyboard. Today the `textOutput` `mouseup`
  handler (~3087) then moves focus to the command box. Android keyboards send
  `key: "Unidentified"`, so the hot-input rule (`hotinput.js`
  `shouldFocusCommandBar`) cannot rescue typing that lands in xterm's
  textarea. A touch scroll cancels the mouse events, so only a tap does this.
- `(hover: none) and (pointer: coarse)` matches phones **and tablets**
  (including an iPad with a keyboard). Phone mode in `dashboard.js` is under
  700px.
- The viewport meta has no `interactive-widget` and the page uses `100dvh`,
  so on current Android Chrome and iOS Safari the layout does not resize when
  the keyboard opens or closes. `SendData` already scrolls the terminal down.
- Nothing else refocuses the box after a send (`socket.onopen` and the login
  nudge run before typing; the only delayed focus is the automation modal's).
- `tools/jstest/hotinput.test.js` tests `HotInput` as a pure module.

## Tasks
1. **One pure decision in `hotinput.js`:**
   `shouldHideKeyboardAfterSend({ touchOnly, phoneWidth, inGame, menuOpen, password, sent })`
   is true only when `touchOnly && phoneWidth && inGame && !menuOpen && !password && sent`.
   Export it beside `shouldFocusCommandBar`.
2. **Command box, Enter branch:** read the inputs at send time (not once, so
   rotation and foldables work): `touchOnly` from
   `matchMedia('(hover: none) and (pointer: coarse)').matches`, `phoneWidth`
   from `matchMedia('(max-width: 699px)').matches`, `password` from
   `textInput.type === 'password'` before the box is cleared, `sent` from
   `SendData`'s return. If the decision is true, `textInput.blur()`.
   No extra scroll call (`SendData` already scrolls; the layout does not
   resize).
3. **Tapping the game text on a phone:** when `touchOnly && phoneWidth`, give
   xterm's helper textarea (`term.textarea`) `inputmode="none"` so its focus
   never raises the keyboard, and skip the `mouseup` refocus of the command
   box. Re-check on resize (rotation into tablet width restores both).
   Desktop and tablets keep today's behaviour.
4. **Leave alone:** `socket.onopen` and the login nudge keep focusing the
   box; the Chat box is unchanged unless Malia decides otherwise (below).
5. **Screen readers:** after a phone send, focus goes to `<body>`. Screen
   reader support in the game window is already weak (no live region); note
   it in the PR as a known gap rather than widen this slice.
6. **Tests** (`tools/jstest/hotinput.test.js`): the decision is true for the
   full set and false when any one of the six inputs flips (each case proved
   able to fail). Static checks on `webclient-pure.html`: the Enter branch
   calls the decision after `SendData` and only on success; the `mouseup`
   refocus is gated; the xterm textarea gets `inputmode="none"` on phones.

## Decision for Malia
**Chat box:** decided (Malia, 2026-11-02): it keeps the keyboard open, as
today. Chat is a back-and-forth; only game commands hide it.

## Definition of done
- Phone, in the game, no menu open: Enter in the command box sends and the
  keyboard closes; the newest output is in view.
- Phone: tapping the game text never opens the keyboard and typing never
  lands in xterm's textarea; tapping the command box opens it.
- Phone: the keyboard stays open between name and password at login, while a
  menu or prompt is open, and for masked (password) input in the game.
- Tablets (any width from 700px, or with a keyboard), desktops and touch
  laptops: focus stays in the box after Enter; arrow history, movement
  shortcuts and F-key macros work as today.
- All `tools/jstest`, `tools/webclient-tests` and
  `tools/test_rgsvg_controls.mjs` pass.

## Verification
- The jstests above (each new check shown failing once by flipping an input).
- Headless Chrome with touch emulation and phone width: after a send,
  `document.activeElement` is not any editable element; after a tap on the
  game text, still not; with a menu open, the box keeps focus. Without touch
  emulation, or at 820px: focus stays. Headless Chrome cannot show whether a
  soft keyboard opens, so it is not the gate.
- **The gate is a real phone:** Malia on Android Chrome
  (`http://<Mac IP>:18090`), and an iOS Safari pass if anyone has an iPhone
  (check the page is not left panned up after the keyboard closes).

## BAR (2026-11-02)
Found and folded in: xterm focuses its own textarea on every press, so
skipping the command-box refocus alone would still open the keyboard and
swallow typing (blocker; now `inputmode="none"` on phones); `inGame` alone
would close the keyboard inside in-game menus and masked prompts (blocker;
`menuOpen` and `password` gates); the media query matches tablets, which
would lose arrow history and macros after a blur (phone width added, read at
send time); the planned extra scroll rested on a false premise about the
layout resizing (dropped); the tests could not catch the main regressions
(decision widened, static checks added); the Chat box is a separate decision
for Malia; screen readers lose their place (noted as a known gap).

## Change after the phone check (Malia, 2026-11-02)
On her phone at login, the keyboard stayed open after she sent her username.
Her rule, restated: **after every send on a phone the keyboard closes, so the
player can read the next step**, at login, in game menus and prompts and for
masked input too. Tapping the box opens it again. This overrides the BAR's
`inGame`, `menuOpen` and `password` gates (owner decision; the cost is one
extra tap between username and password and between answers to a menu).
Still phone only (touch-only and under 700px), still not the Chat box.
- `HotInput.shouldHideKeyboardAfterSend` keeps only `touchOnly`,
  `phoneWidth` and `sent`; the page stops passing the others.
- Tests: the dropped gates are replaced by checks that login, a menu and
  masked input now hide the keyboard too.

## After the merge with master's phone fit (2026-11-03)
"Phone" now uses master's `ViewportFit.isPhone` (the screen's shorter side
under 600px) instead of "under 700px wide": phones held sideways close the
keyboard too; touch tablets in a narrow split view no longer do. Master's
fit-above-the-keyboard (`viewportfit.js`) handles the keyboard while it is
open; this rule closes it after a send.
