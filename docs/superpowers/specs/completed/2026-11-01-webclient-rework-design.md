# Web client and website rework (rounds 1 and 2): design

**Status:** written 2026-11-02 *after* most of this work was built (PR #768),
to put the skipped spec step back into the repo (Calabe, 2026-11-02). The
decisions below are Malia's, made while reviewing on localhost, dated.
**Owner:** Malia (look and feel). **Calabe's share:** review; one small
server change (asset version tags).
**Arc:** the touch web client rework, epic #783 (its requirements doc was
never merged; the epic is the pointer). **Colours:**
[`2026-11-01-brand-palette-design.md`](2026-11-01-brand-palette-design.md).
**Brand rules:** [`../../../guides/BRAND_STANDARDS.md`](../../../guides/BRAND_STANDARDS.md).

## Goal
Younger players on any device: keep the reading, cut the repetitive typing,
and let a player tell the website from the game at a glance. One component
per job, the same on phone (< 700px), tablet rail (700 to 1100px) and wide
screens.

## Decisions

### Surfaces
- **Teal means website, umber and gold mean game** (2026-11-01). Site bar
  and footer teal `#085044`, 92% opaque; nav text `#dfcc94` (5.89).
- **Footer:** full-width teal bar; website pages repeat the menu above the
  credit line; the game page has the credit line only (2026-11-01, built
  2026-11-02). Credit in nav gold (olive grey fails on teal, 4.46).

### Game tabs (round 1b)
- **Names** (Calabe OK'd in person, 2026-11-01): Map, Quests, Health
  (was Vitals), Inventory (not "Bag": bag items exist), Chat (panel was
  Comms), Status (was Status & Conditions), Automation (was Trig). Client
  labels only.
- **Round icon tabs, Version B** (2026-11-01): one component for the rail
  and the phone bar, line icons (no emoji), `aria-pressed`. One name pill
  per bar resting on the open tab, sliding to the hovered or focused circle;
  touch moves it on tap only; reduced motion means no slide.
- **Bar:** dark umber `#19140c` with a teal rim `#2e6e60` (rim chosen over a
  teal gradient, 2026-11-02).
- **Circles** (2026-11-02): shiny brass with a dark icon; open = pressed
  into dark brass with a cream icon.

### Brass and sub-tabs
- **Open/closed rule** (2026-11-02): closed is shiny brass with dark text;
  open is dark pressed brass with cream text. Dark text on darkened brass
  fails (1.56 to 2.74). No coloured underline (an orange one was tried and
  rejected).
- **Sub-tabs** (Chat channels, Inventory, Automation): one `.subtab` style,
  Inventory's button shape at Chat's reading size (15px), nothing dimmed,
  rows wrap (no hidden channels), 44px on touch.
- **Game brass** for other buttons: even gradient, no shine spot (the
  shine made text 2.29 at the edge); no text glow; regular weight. Session
  bar buttons 15px with 12px side padding; no gear icon on wide screens
  (phones keep icon squares). Drawer close is a square ✕.

### Panels
- Gold outline on every card (parchment cards were red, which read as a
  warning). One teal line under every panel title. Orange-red was tried
  under the open Automation tab, then dropped with the brass rule.
- Pop-out windows are wide-screen only: docked on tablet and phone, popped
  again when wide; the panel header is hidden inside the window.

### Light mode (round 2, 2026-11-02)
- Website pages: existing light theme. Game panels: olive grey title bars
  `#b4b4a3`, bodies `#cec6aa`, umber text (Malia's sample
  `mapGtabFull.jpg`). Tab strips sit on the body colour (no grey behind the
  buttons). The game text window and command box stay dark; the Map body
  stays dark until round 3.

### Logo and icons (2026-11-02)
- Full logo (Malia's art) in the home page hero; a light version for Light
  mode (`#6f5418` / `#473019`). The D.O.G. mark in the top bar: the stacked
  logo is unreadable at bar height.
- Logo golds `#d9ac46` and `#c3932d` joined the palette as their own role.
- Favicons from Malia's Moons of Gaius on the panel brown `#201913` (Calabe's
  `#1e1915` is within ΔE 1.6).

### Other pages
- Architecture diagram cards lose their outlines; umber fill `#252015` plus
  a soft shadow so they still separate from the page card.

### Shipping
- Static CSS and JS links carry master's `?v={{ .ASSET_VERSION }}`, a hash of
  `static/css` and `static/js` worked out once per boot, so each deploy
  reaches browsers. Malia chose automatic tags on 2026-11-02; this PR's own
  start-time tag was dropped for master's in the 2026-11-03 merge. Images
  are not hashed, so a changed image gets a new file name.

### Map strip (2026-11-02, in progress)
- The legend card leaves the map: a "Map legend" strip under the map at
  reading size, foldable.
- The zoom buttons leave the map too, into the same strip: Fit, Center
  (US spelling), −, + on the left, "Legend ▾" on the right; 32px, 44px on
  touch.

### Phone reading (2026-11-02, spec for Malia's review)
Malia's phone screenshot (`~/Downloads/ssphone.jpg`, Android Chrome): the
game text is cut off on both sides and the keyboard covers half the screen
after a command.

**Why it happens.** The server hard-wraps text at 80 characters (room text is
authored that way, and `Display.ScreenWidth`, parsed from the client's
screen-size report in `internal/inputhandlers/term_ansi.go` and
`term_iac.go`, is read by only three admin and map commands). The web client
keeps all 80 columns on a phone by shrinking the font, then scaling the
terminal down (`resizeTerminal` in `webclient-pure.html`): about 7px text on a
390px screen. Players pinch-zoom to read it, and the text runs off both
edges.

**Slice A, client only (Malia, 2026-11-02: "readable now, server later").**
*Later the same day Malia dropped the readable-text part: "it can be fixed
later server side". Only the keyboard part below was built
(`plans/2026-11-02-webclient-phone-keyboard.md`); readable phone text waits
for slice B.*
- **Readable text on phones** (phone mode, under 700px wide): the game text
  window uses a readable font size (about 13 to 14px, the exact size chosen
  in the plan by measuring) and as many columns as fit; the 80-column
  shrink-and-scale applies only on wider screens. Lines longer than the
  screen wrap onto the next row (xterm's own wrapping), so nothing runs off
  the edge and no zooming is needed.
- **Known cost, accepted:** an 80-character line becomes two rows, so some
  lines end early, and wide layouts (tables, the DOGMud logo art in the
  welcome screen) can break. Slice B fixes ordinary text properly.
- **Keyboard hides after each send** on touch screens (Malia, 2026-11-02):
  after Enter in the command box, the box loses focus so the phone keyboard
  closes and the whole screen shows the result to read; tapping the box
  opens it again. Desktop and physical keyboards keep focus as today. The
  plan checks whether the Chat box should do the same, and how this meets
  `static/js/hotinput.js`, which may move focus back on key presses.
- **Done when:** on a 360 to 412px phone, game text needs no zooming, nothing
  is cut off, and after sending a command the keyboard is down and the
  latest output is in view; tablets and desktops unchanged.

**Slice B, server wraps to the screen (later; Calabe's work).**
- The web client reports its column count (the server already parses
  screen-size reports into `Display.ScreenWidth`), and the server wraps
  outgoing text to that width instead of a fixed 80, so phone lines end at
  word boundaries.
- Needs Calabe to scope it: where text is wrapped today, authored hard wraps
  in room text, and the `term_ansi.go` branch that saves the width inside an
  `if err != nil` (so ANSI screen-size reports are never stored; the telnet
  NAWS path in `term_iac.go` is separate).
- Its own spec and plan when Calabe takes it up.

### Session bar status (2026-11-02)
- The status text ("○ Not connected", "● Connected", "↻ Reconnecting…") read
  smaller than the 15px buttons beside it. Malia's fix: the status circle
  moves **into** the Enter Game / Leave Game button, circle only (○ red when
  not connected, ● green when connected, ↻ amber reconnecting, ● amber
  "Leaving…"); its colour and shape show the status; screen readers and the
  tooltip say the words. Plan: `plans/2026-11-02-webclient-session-status.md`.

### Automation after the #768 review (2026-11-05)
- Text on the dark Ticks and Triggers rows and the queue bar keeps the Dark
  mode light golds in Light mode too (Malia; review item 2).
- On/off toggles: on is the lamp green `#70e010` with an umber rim and knob,
  off keeps its dark track; new entries start on; red never means on (Malia). Check
  boxes are not toggles and are unchanged.
  Plan: `plans/2026-11-05-webclient-768-review-fixes.md`.

### Chat and the phone tabs (#768 item 1, Malia 2026-11-06)
- The Chat tab keeps its text channel buttons and loses its own typing box.
  Players chat from the command box (`say hello`).
- On a phone every open tab fits its content, up to half the screen, and
  stops above the command box, so the game text and the command box stay in
  view. The Map and Chat tabs keep the full half (the map needs a set space;
  Chat scrolls to its newest message).
- On a laptop the command box stays at the bottom of the middle column even
  when the Game window is popped out (it had jumped to the top of the page).
- Title bar buttons: laptops and tablets keep ▾ (fold) and ⧉ (pop out).
  Phones show ▾ and ✕ (close, the size of the Settings square) in the open
  tab's own title bar, ✕ at the far right with a 16px gap against mis-taps;
  the strip that held only the ✕ is gone, giving the game that room. Every
  tab's ▾ folds it on every screen (Map, Chat and Automation had not).
- Repeatable quests (review item 3): the "(repeatable)" tag, the
  "Repeatable work" heading and the finished rows are quieted with the quest
  hint colour (`--quest-progress`, 4.95 Dark, 4.97 Light), not with opacity.
  Opacity had faded them to about 2 in Dark and 1.9 in Light, and the review
  found 1.01 in Light because they had no colour at all (Malia approved).
- Settings and typing (review item 6): closing Settings with a mouse,
  trackpad or finger leaves nothing focused, so typing reaches the command
  box; a keyboard close (Esc, or Enter/Space on ✕ or Settings) still returns
  focus to the Settings button. Malia reproduced the bug on her laptop; the
  Automation tabs did not lose typing in her test, so they are unchanged.
- Map (Malia 2026-11-06): an even 10px gap above and below the leather, the
  map box in the leather's own shape so Fit / Center sits right under it.
  "Legend ▾" (gold words, not brass) moves into the Fit / Center / − / + row
  at its right end and opens the key below; the key starts folded on windows
  under 800px tall. In a very narrow column the row wraps rather than cut
  off a button.
- Laptop page scroll (review item 5): the Map's minimum height is back to
  master's 150px (this PR had 230). Measured with five quests, logged out:
  no whole-page scroll from 1440x900 down to 1100x540. A logged-in
  character with a party or a long gear list adds height; check that by
  hand.
- Code is written so a person can maintain it: edit what exists first, and
  give every change a plain-English comment (Malia).
- Still to decide: a description of each Chat button (hover on a laptop; a
  phone needs another way), and whether picking a channel fills the command
  box with its command. The laptop right column (Chat, Status, Automation in
  equal thirds) is unchanged.

## Rules that hold everywhere
Text 4.5:1 or more, icons and focus 3:1, numbers written beside the colour in
the CSS; light mode text darker than its surface, dark mode lighter; red only
for warnings; phone, tablet and desktop checked in both modes before review.

## Open items
- Round 3: light Map (room ink and biome tints): not planned until Malia has
  opinions from other human testers (Malia, 2026-11-02). Issue #812.
- Listing-site images still show the old emblem (keep their names).
- The session bar's "Connected" text was smaller than its 15px buttons:
  Done: the status is a circle in the button ("Session bar status"). Yes and
  Stay keep their bold labels (Malia, 2026-11-02).
- Hide the Light option until round 3? No: Light stays visible (Calabe,
  2026-11-07).
- Readable game text on phones: left to slice B (server wraps to the
  screen's width; Calabe's work), Malia 2026-11-02. Issue #776.
- Small map issues Malia noticed after the map strip: dropped from the open
  list (Malia, 2026-11-02); raise again if they come back.
- **The map's level line is hard to read** (Malia, 2026-11-03). On the map,
  under the place name (for example "Newcomer Antechamber"), the small line
  that says which floor you are on ("~ Level 0 ~") is too small to read
  comfortably on laptops, tablets and phones. It should be bigger and
  clearer on every screen, while staying smaller than the place name above
  it. Her screenshot: `~/Desktop/dogmudRedo/levelSZIncrease.jpg`. Ours to
  fix in the web client (no server work); it goes through the usual spec
  note, plan, review and Malia's approval when picked up. Issue #811.

## Slices and plans
| Slice | Plan |
|---|---|
| Round 1b: game tabs, brass, panels | `plans/2026-11-01-webclient-round-1b-game-tabs.md` |
| Logo, light logo, favicons | `plans/2026-11-02-webclient-logo-and-favicons.md` |
| Round 1a: footer, Architecture cards | `plans/2026-11-02-webclient-round-1a-footer-architecture.md` |
| Round 2: Light mode game panels | `plans/2026-11-02-webclient-round-2-light-panels.md` |
| Asset version tags | `plans/2026-11-02-web-asset-version-tags.md` |
| Map strip | `plans/2026-11-02-webclient-map-strip.md` |
| Phone reading, slice A (client): keyboard | `plans/2026-11-02-webclient-phone-keyboard.md` (readable text dropped, left to slice B) |
| Phone reading, slice B (server wrap) | Calabe to scope; own spec and plan |
