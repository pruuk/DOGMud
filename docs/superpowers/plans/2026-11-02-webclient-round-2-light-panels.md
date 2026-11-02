# Web client round 2: Light mode game panels (retroactive record)

> **Written after the work** (2026-11-02), to put the skipped plan step back
> into the repo. No BAR was run before execution; a BAR of the shipped diff
> follows. Spec:
> [`../specs/2026-11-01-webclient-rework-design.md`](../specs/2026-11-01-webclient-rework-design.md).

**Commit:** `ec67ca4c9` on `feat/game-settings` (PR #768).

## Asked
Malia: Light mode should reach the game after Play in Browser, per her sample
(`mapGtabFull.jpg`): olive grey title bars, light panel bodies, umber text.
Then: no grey behind the tab buttons.

## What the code showed
- The game colours are named tokens (about 390 `var()` uses, about 20 fixed
  values), so one scoped token block can swap them.
- Several tokens do two jobs: `--ink-deep` is both a dark bar and the text on
  the parchment cards; `--vellum` is both a bright text and the parchment
  card fill. Flipping them globally would break those cards.
- The test browser kept the game frame's old stylesheet even with cache
  disabled; a fresh profile was needed (led to the asset version tags).

## What changed
`static/css/dashboard.css`, one block:
`:root[data-theme="light"][data-client] .dash-panel:not(#panel-feed):not(#panel-cmd):not(#panel-map)`
sets light values for panel, text, quest, chat, chip, input and focus
tokens; `.dash-panel-head` gets `#b4b4a3` with `#252015` titles; `.subtabs`
use the panel body. `--ink-deep` and `--vellum` are left alone, so the
parchment cards keep dark text.

## Definition of done
Every text pair 4.5 or more on `#cec6aa` (and 7.71 for titles on the bar);
the Game text window, command box and Map body unchanged; Dark mode
unchanged.

## Verified
Sample chat fed through `handleGMCP('Comm.Channel', ...)`; screenshots at
390, 820 and 1366 in Light and Dark; measured: text 9.22, secondary 4.97,
headings 5.83, links 5.15, mob names 5.36, player names 5.83, broadcast
4.60, focus 7.20; JS tests pass.

## Review
BAR of the shipped diff (2026-11-02): the definition of done above was **not**
fully met. Found and fixed on the branch:
- Mutation slabs (`.inv-slab.mut`, `--copper`) were 3.49: `--copper` is now
  `#085044` in the block (5.48).
- The focused-quest marker (`--brass-mid`) was 1.99: umber gold `#5e4712`
  (5.15). `--brass-mid` itself is left alone, since the sub-tab brass uses it.
- The chat box focus edge was 2.48 and its resting edge 2.90: the edge token
  `--panel-border` is now `#736133` (3.52 on the body, 4.39 on the box) and
  focus uses `--focus-ring` (8.98).
- Flipping `--ink-gold` sank the charge meters on the dark inventory tiles
  to about 2: they keep `#b5a26c` (6.34 on the tile).
- The Automation switch's off knob was 2.13 on its track: light knob
  `#e3dcc4` (4.39).
Rule learned: before flipping a token in a scoped block, list every rule that
reads it, not only the text ones.
