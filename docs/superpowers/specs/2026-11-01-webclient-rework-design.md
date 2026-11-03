# Web client and website rework (rounds 1 and 2): design

**Status:** written 2026-11-02 *after* most of this work was built (PR #768),
to put the skipped spec step back into the repo (Calabe, 2026-11-02). The
decisions below are Malia's, made while reviewing on localhost, dated.
**Owner:** Malia (look and feel). **Calabe's share:** review; one small
server change (asset version tags).
**Arc:** the touch web client rework
(`docs/superpowers/specs/2026-10-31-touch-web-client-requirements.md`, on
branch `docs/touch-web-client-requirements`). **Colours:**
[`completed/2026-11-01-brand-palette-design.md`](completed/2026-11-01-brand-palette-design.md).
**Brand rules:** [`../../guides/BRAND_STANDARDS.md`](../../guides/BRAND_STANDARDS.md).

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
- Static CSS, JS and icon links carry `?v={{ .ASSETV }}` (server start time),
  so each deploy reaches browsers (2026-11-02; Malia chose automatic over a
  manual date tag knowing it is a small server change).

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

## Rules that hold everywhere
Text 4.5:1 or more, icons and focus 3:1, numbers written beside the colour in
the CSS; light mode text darker than its surface, dark mode lighter; red only
for warnings; phone, tablet and desktop checked in both modes before review.

## Open items
- Round 3: light Map (room ink and biome tints), on hold.
- Listing-site images still show the old emblem (keep their names).
- The session bar's "Connected" text is smaller than its 15px buttons.
- Hide the Light option until round 3? (asked, unanswered)

## Slices and plans
| Slice | Plan |
|---|---|
| Round 1b: game tabs, brass, panels | `plans/2026-11-01-webclient-round-1b-game-tabs.md` |
| Logo, light logo, favicons | `plans/2026-11-02-webclient-logo-and-favicons.md` |
| Round 1a: footer, Architecture cards | `plans/2026-11-02-webclient-round-1a-footer-architecture.md` |
| Round 2: Light mode game panels | `plans/2026-11-02-webclient-round-2-light-panels.md` |
| Asset version tags | `plans/2026-11-02-web-asset-version-tags.md` |
| Map strip | `plans/2026-11-02-webclient-map-strip.md` |
| Phone reading, slice A (client) | plan to follow once this spec section is approved |
| Phone reading, slice B (server wrap) | Calabe to scope; own spec and plan |
