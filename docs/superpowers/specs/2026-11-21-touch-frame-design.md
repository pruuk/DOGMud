# Touch frame (touch arc spec 1)

Epic #783. Spec 1 of the work breakdown in
`docs/roadmaps/TOUCH_ARC_ROADMAP.md`, brainstormed with the owner on
2026-11-21. It builds the frame every later touch spec fills: the controls
deck and its slot, the keyboard rule, and the phone polish. It is
client-only. Three plans, each one PR: **1a** the frame, **1b** the
keyboard, **1c** phone polish.

## Facts verified against source (2026-11-21, master `d99b9706d`)

| Fact | Where |
|---|---|
| Layout modes are wide, column, slim and phone; phone is chosen for a touch screen whose short side is under 600px, or when the room beside the game is under 120px | `static/js/layoutpick.js:86-93`, `static/js/dashboard.js:335-347` |
| Phone detection is by screen size plus `pointer: coarse` | `static/js/viewportfit.js:18-20`, `dashboard.js:337-338` |
| On a phone an open tab is a fixed card (`#dash-drawer`) anchored above the command box, tab bar and name strip; the game window is padded only for the tab bar and name strip, so **the card covers the bottom (newest) lines of game text**. Read from the CSS, not screenshotted | `static/css/dashboard.css:792-800`, `:842-844`, `:905-917` |
| The phone card is capped at 30% of the screen, the Map at 45% | `dashboard.css:763`, `:1205-1208` |
| With the keyboard up, the tab bar hides and an open card stays, capped at 30% of what is visible | `dashboard.css:867`, `:880-883` |
| `keyboard-open` is set on the game page by the outer page when a text field has focus and the visible area is more than 120px short | `webclient.html:26-47`, `viewportfit.js:124-126` |
| A Close keyboard ✕ shows beside the command box while the keyboard is up | `webclient-pure.html:321`, `dashboard.css:214-224` |
| The game text refits (rows) whenever the Game window's box changes size, through a ResizeObserver on `#panel-feed` | `webclient-pure.html:683-694` |
| `static/js/` has no `context.md` (the convention covers Go packages under `internal/` and `modules/`) | `ls static/js/context.md`: absent; `CLAUDE.md` |
| No tab has a text field of its own; Automation's editors (macro, alias, tick, trigger) open in a dialog | `webclient-pure.html:4716-4990` (`openAutoModal`, e.g. `:4733`) |
| The game page's Settings menu has two sections, Sound and Appearance | `webclient-pure.html:275-299` |
| Device-wide client settings live in `localStorage` (`soundOn`, `siteTheme`, `dogmud.fullscreen`, `dogmud.dashboard.layout.v2`) | `webclient-pure.html:3721`, `static/js/theme.js:31`, `webclient.html:57`, `dashboard.js:37` |
| Phones and laptops go browser full screen on the Enter Game tap unless the player turned full screen off | `viewportfit.js:150-156`, `webclient.html:83-88`, `:118-123` |
| Leaving full screen with the back gesture does not change the saved choice | `webclient.html:107-111` |
| The site has a web app manifest with `"display": "standalone"`, so Add to Home Screen opens the game with no browser bars | `static/manifest.json`, `_header.html:20`, `webclient-pure.html:7` |
| The viewport tag that sets the page width is on the outer page; the game runs in a frame inside it | `_header.html:6`, `webclient.html:172` |
| The Game title bar holds only the pop-out button, which does nothing outside wide | `webclient-pure.html:311-312`, `dashboard.js:830` |
| Sub-tabs wrap to more rows on purpose (Calabe 2026-11-07, "so every Chat channel stays in sight"); the roadmap (2026-11-19) replaces that with one scrolling row | `dashboard.css:1110-1131`; roadmap, "Phones in general" |
| Phone sub-tabs and panel title buttons are 30px (brand exception) | `dashboard.css:1185-1193`, `docs/guides/BRAND_STANDARDS.md:110-115` |
| The brand rule for touch targets is 44px today | `BRAND_STANDARDS.md:110` |
| One sideways-phone toast exists, once per browser tab (sessionStorage) | `viewportfit.js:66-79`, `dashboard.js:306-327` |
| Client tests: `tools/jstest/*.test.js` (including `layoutpick`, `viewportfit`, `termfit`, `longpress`), `tools/webclient-tests/*.js`, `internal/guards/client_type_guard_test.go`; headless shots with `tools/webshot/live-shot.js` | `tools/jstest/`, `tools/webshot/README.md`, `.github/workflows/validate.yml:107-128` |
| Galaxy S21 360 x 800, text rows 22.4px, session bar 58, command box 62, tab bar 56; Pixel 8 412 x 915 | roadmap facts table (measured 2026-11-19 from screenshots) |

## Owner rulings in this brainstorm (2026-11-21)

1. **The deck is built switched off.** Automatic means off until plan 2b
   puts movement in it; the setting's Always lets the owner and Malia check
   the frame on their phones. Players see no deck until it does something.
2. **An open tab takes the deck's place and pushes the game up**, so the
   newest line always shows. Already implied by the roadmap's row counts
   (about 9 rows with the Map open, about 14 in combat Y).
3. **Below 360px wide, the same six-column grid with 44px buttons and 5px
   gaps** (roadmap question 8). Every button keeps its slot on every phone.
4. **A full-screen tip once per device**, worded for Android and for
   iPhone Safari (roadmap question 5).
5. **Desktop site: try to undo it (D), fall back to a tip (A)** if the
   owner's Pixel 8 shows D does not work (roadmap question 6).
6. **Approach 1: a phone's game column is one stack** laid out by the
   browser; the floating card stays only for column and slim.
7. **iPhone testing on a borrowed phone is optional**, if a houseguest
   lends one.

## The frame (plan 1a)

**The setting.** Settings gets a third section, **Touch controls**:
Automatic, Always, Never. Stored on the device (`localStorage`
`dogmud.touchControls`), like Sound and Appearance. Automatic resolves to
off until plan 2b, then to `pointer: coarse` (touch is the main input).
Always and Never work from day one.

**The phone stack.** In phone mode the game column is, top to bottom:
1. the session bar;
2. the game text, taking the height left and staying scrolled to the newest
   line (it already refits whenever its box changes size);
3. the **slot**, which holds one of three things:
   - nothing, at zero height (deck off and no tab open);
   - the **deck**: two rows of six 48px buttons with 8px gaps and 8px
     padding (about 122px with its border). Where the deck's box is
     narrower than 344px (328 for the buttons plus padding), the buttons
     are 44px with 5px gaps (289px), so it fits at 320 wide;
   - an **open tab**: at least the deck's height while the deck is on (a
     short tab never makes the game text jump), at most today's caps (30%
     of the screen, the Map 45%);
4. the divider (12px), only while the deck is on;
5. the command box;
6. the tab name strip (only while a tab is open) and the tab bar.

The open tab moves into the slot on phones only. Column and slim keep the
floating card, which sits beside or under the game there and covers no
game text. Moving the tab into the slot fixes the covered newest lines for
**every phone player, deck on or off**: the one change in 1a players see.

**Other devices** (only while the deck is on):

| Device | Deck |
|---|---|
| Upright tablet, open foldable | Two rows, as many 48px columns as fit (12 to 14), above the command box. Tabs as today |
| Sideways tablet | The same, above the command box; Map, Health and Conditions stay beside the game |
| Sideways phone | Session bar and tab bar fold away; the deck becomes a side column beside the game |
| Laptop, desktop | No deck unless set to Always, then the tablet shape |

**Empty slots.** In 1a the deck draws empty outlined slots. The client
cannot fill them: buttons only ever come from the server (the roadmap's
sync rule), starting with spec 2.

**Code.** New `static/js/touchdeck.js` (UMD like `layoutpick.js`, so
jstests load it): the setting, whether the deck shows, the column count and
button size for a width, and the slot decision. `dashboard.js` moves an
open tab into the slot in phone mode and keeps the card for column and
slim. `dashboard.css` replaces the phone's fixed offsets (`--cmdbox-h`,
`--tabbar-h` padding on `#dashboard`) with the stack. `webclient-pure.html`
gets the deck element, the divider and the Settings section. Static JS has
no `context.md`, so `touchdeck.js`'s header comment carries its
description, as `layoutpick.js` does.

## The keyboard (plan 1b)

Where the typing goes decides what steps aside:
- **In the command box:** the deck and any open tab step aside, and the
  slot empties so the game text gets the room. When the keyboard closes the
  same tab comes back, scrolled where it was (Comms back at its newest
  message, as today); with no tab open the deck comes back. If the player
  taps another tab meanwhile, the last tap wins.
- **In any other field** (today only the Automation dialog): nothing steps
  aside. The dialog stays over the page as now.
- **Comms reply** (spec 4a) is command-box typing, so it follows the first
  rule, as ruled on 2026-11-19.

The decision is a pure function in `touchdeck.js` (focused field, open tab,
keyboard state → what the slot shows) with jstests. On a phone, today's
"open card capped at 30% above the keyboard" goes, because the tab is
hidden while typing. The column layout keeps its own keyboard handling
unchanged.

## Phone polish (plan 1c)

1. **No Game title bar on phones** (32px, about 1.4 more rows). The sight
   colour is on the game window's border, so it stays.
2. **Sub-tabs in one sideways-scrolling row** (Inventory, Comms). The row
   fades at whichever edge has more to scroll, and the open sub-tab scrolls
   into view when the tab opens. The 11-07 comment in `dashboard.css` is
   rewritten to cite the roadmap ruling.
3. **The full-screen tip**, a toast once per device (`localStorage`), phones
   only:
   - Android, not in full screen: "Tap ⛶ for more room to play."
   - iPhone Safari, not opened from the Home Screen: "For more room, add
     this page to your Home Screen: tap Share, then Add to Home Screen."
   - None when running from the Home Screen, or when the player turned full
     screen off in the game.
4. **Desktop site.** Detected as a touch device whose page is laid out at
   least 15% wider than its screen. Any browser with a desktop-site switch
   is caught by the same check (Chromium browsers, Firefox for Android,
   Safari); only the ones we test count as verified.
   - **D (first):** the outer page rewrites its viewport tag to the
     screen's real width.
   - **A (if the owner's Pixel 8 shows D does not work):** a toast once
     per device: "This page is showing as a desktop site, so everything is
     small. Turn off Desktop site in your browser's menu."
   - When both tips apply, Desktop site comes first, and one visit never
     shows both.
5. **Brand doc** (`docs/guides/BRAND_STANDARDS.md`): touch targets 48px
   with 8px gaps (from 44px); the 44px and 5px exception below 360 wide;
   the 30px sub-tab and title-button exception kept; the deck, its divider,
   the button kinds and shapes, and the ◢ mark.

## Testing

**Automated (CI).**
- jstests for `touchdeck.js`: the setting (Automatic off until 2b, Always,
  Never), columns and button size per width (6 at 360, 6 at 320 with 44px,
  12 to 14 on tablets), the slot decision and every keyboard case.
- **Guard 6, touch targets** (roadmap): a jstest lays the deck out at the
  size matrix and fails on a slot under 48px or a gap under 8px, with the
  below-360 and 30px exceptions written into the guard. Can-fail case: a
  47px slot must fail.
- The existing guards keep running: `client_type_guard_test.go`, the
  wrap-width guards, `tools/jstest`, `tools/webclient-tests`.

**Headless (Claude, every plan).** `tools/webshot/live-shot.js` against a
local server on a private port, touch emulation on, deck on and off, at
375 x 548, 360 x 800, 412 x 915, 800 x 360, 820 x 1180 and 1180 x 820,
690 x 840, 1366 x 768, plus 320 wide. Keyboard states are approximated by a
shorter viewport. Numbers to meet (estimated from the S21's measured strips
until re-measured in the shots):
- S21 full screen: about 20 game rows with the deck (800 − 64 − 122 − 12
  − 62 − 64 = 476px), about 9 with the Map open.
- 360 x 550: deck and command box fit with no scrolling; at least about 9
  rows (about 226px of game).
- No sideways scroll at any size, 320 included.
- 1366 x 768 and a wide desktop match master in a before/after pair.

**By hand, every plan, on the owner's Pixel 8 and Malia's S21** (#834),
ordinary player steps only:
- 1a: open Comms, say something, and the newest line stays in view above
  the tab. Set Touch controls to Always: empty slots appear and a tab takes
  the deck's place.
- 1b: with Comms open, tap the command box; Comms steps aside and comes
  back when the keyboard closes.
- 1c: no Game title bar; Comms channels in one scrolling row; the
  full-screen tip once. On the Pixel 8, turn on Desktop site: this decides
  D or A.

**Optional: a borrowed iPhone** (owner, 2026-11-21). If a houseguest lends
one: the iPhone tip in Safari, the Home Screen app with no tip, the stack
at Safari's height, and Request Desktop Website. Without it, iPhone checks
are device-mode only and the plan says so. Tablets stay headless only.

**Unchanged, checked:** telnet and Mudlet (no server change), and desktop
layout D (the before/after pair).

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| The setting is device-wide (`dogmud.touchControls`), not per account | Sound and Appearance are device-wide; one phone, one hand | Key it by account name like the combat default |
| The setting lives in the Settings menu as a third section | Where the player already finds device settings | Its own button on the session bar |
| The 44px / 5px size applies when the deck's box is under 344px, not by screen width | The deck's own width is what has to fit | Switch by screen width under 360 |
| The divider shows only while the deck is on | With no deck there is nothing to divide from the command box | Always show it |
| Tablets use as many 48px columns as fit, two rows | The roadmap's 12 to 14 falls out of the width | Fix the count per device class |
| A desktop set to Always gets the tablet shape | It is the wide variant of the same deck | A phone-shaped deck on desktops |
| After the keyboard closes, the last tab tapped comes back, scrolled where it was | The player's latest choice wins | Always restore the tab open before typing |
| The Automation dialog counts as "another field" (nothing steps aside) | It is the only text field outside the command box today | Treat it like the command box |
| On phones the 30%-above-the-keyboard cap goes | The tab is hidden while typing in the command box | Keep a shrunken tab above the keyboard |
| The Game title bar goes on phones only, not slim or column | Only phones lack the room, and the bar holds nothing usable there | Drop it in slim too |
| The scrolling sub-tab row fades at the edge with more, and scrolls the open sub-tab into view | A scrolling row hides channels; the fade says so | Scroll buttons, or keep wrapping |
| Each tip shows once per device (`localStorage`), not per tab like the sideways note | A tip is advice, not a warning | Once per browser tab |
| No full-screen tip for a player who turned full screen off in the game | They chose | Tip them anyway |
| Desktop site is detected at 15% wider than the screen | Clear of scrollbars and rounding, far below the 2.4x seen on the Pixel 8 | Tune the threshold on the Pixel |
| Desktop site tip before the full-screen tip, never both in one visit | Full screen does not help a zoomed-out page | Show both |
| The tab-into-slot move lands in 1a (not a separate bug fix) | It is the stack itself | Ship it alone first |
| Row counts are estimates from the S21's measured strips | No device measurement of the new stack yet | Re-measure in each plan's shots |
