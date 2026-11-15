# Web client base plan 7c: a bigger Map in the laptop column

Status: design approved by the owner 2026-11-15 in chat (option A of the mocks, then the Health and Conditions swap); this file awaits his review.
Parent: `docs/superpowers/specs/2026-11-15-webclient-base-7b-short-screens-design.md` (plan 7b, PR #870). Built on `feature/webclient-base-7-panels`.
Mocks: `laptop-mock-1440x700-{now,A,B}.png` and `laptop-mock-1280x700-{now,A,B}.png` in the main checkout's root (gitignored), rendered in the real client with fake Health rows and Conditions.

## Facts verified against source

Read 2026-11-15 from `feature/webclient-base-7-panels` at `53512c451`. `P` is `_datafiles/html/public`.

| Fact | Where |
|---|---|
| Column stacks Map, Health and Conditions in one grid column: `grid-template-rows: auto auto minmax(96px, 1fr)`, areas `map / vitals / status` | `P/static/css/dashboard.css:507-522` |
| Column's Map is `min(100%, max(120px, calc(100vh - 300px - var(--tabbar-h))))`, which is the panel area's height less 228px (Health floor 120, Conditions floor 96, two 6px gaps) | `P/static/css/dashboard.css:529-548` |
| Health has `min-height: 120px`, Conditions `min-height: 96px` in column | `P/static/css/dashboard.css:551-552` |
| The panel area's bottom padding is `6px + --tabbar-h`, more while the tab name pill shows, 0 while the keyboard is up; `--tabbar-h` is 60px, 64px on touch | `P/static/css/dashboard.css:514`, `:712`, `:810-812`, `:816-822`, `:830` |
| `_measurePanels` sets `--panels-w`; `_apply` runs it before and after a mode change, and on every resize | `P/static/js/dashboard.js:337-342`, `:344-370`, `:186-187` |
| `_apply` returns early when the mode is unchanged (after measuring) | `P/static/js/dashboard.js:346-347` |
| Health is `#vitals-bars`, one `.vitals-row` per member (26px, 2px gap), self and then companions or party members; its height grows with each member | `P/webclient-pure.html:373`, `:2505-2520`, `:3809-3822` |
| Conditions is `#status-conditions`, a flex column of `.status-line` lines (6px gap) | `P/webclient-pure.html:2675`, `:3974-3984` |
| `LayoutPick` holds the ladder's pure logic and its CSS guard (`wideMapSize`, `MAP_FLOOR`) | `P/static/js/layoutpick.js:28-58`; `tools/jstest/layoutpick.test.js` |
| While the keyboard is up the ladder keeps the frame height from before (plan 7b review fix) | `P/static/js/dashboard.js` `_computeMode` |

Measured in the mocks (1440×700 window, header hidden; panel area 681×568 above the tab bar; Health with two rows about 92px): today's Map 340px; Conditions beside the Map with Health below, 477px; 1280×700 (panel area 521 wide): 340px today, 355px side by side.

## What players see

On a laptop in the column layout the Map is as large as the space allows. Conditions sits to its right, as tall as the Map, and Health runs across the full width below both, above the round buttons. A party or companions make Health taller and the Map gives way. When Health would take more than a third of the panel column, Health and Conditions swap: Health moves beside the Map, where its rows stack, and Conditions becomes the strip, its lines side by side. A narrow column, where Conditions would squeeze the Map, keeps today's stacked layout.

## Design

### 1. Three arrangements inside column

The ladder's modes do not change: column keeps its tab bar and the same four panels behind buttons. Inside column, `#dashboard` carries `data-column`:

- `stacked`: today's (Map, Health, Conditions in one column).
- `side`: grid areas `"map status" "vitals vitals"`. Map square, Conditions the rest of the width at the Map's height, Health below at its own height.
- `swapped`: grid areas `"map vitals" "status status"`. Health beside the Map at the Map's height (it scrolls if it has more rows than fit), Conditions below in a 96px strip whose lines flow into columns (`column-width`), scrolling if they overflow.

In `side` and `swapped` the grid's first column is the Map's size (`--col-map`) and the side panel takes the rest, at least 160px.

### 2. Picking one

A pure function in `layoutpick.js`, `columnArrangement(w, h, vitalsH)`, where `w` and `h` are the panel area's content box (width, and height above the tab bar) and `vitalsH` is Health's natural height. It returns `{ arrangement, map }`:

- `stackedMap = min(w, max(120, h - 2*6 - 120 - 96))` (today's formula, restated).
- Health is the strip while `vitalsH <= h / 3`, so the candidate is `side` with `map = min(w - 166, h - 6 - vitalsH)`; otherwise `swapped` with `map = min(w - 166, h - 6 - 96)`. 166 is Conditions' 160px floor plus the gap.
- The candidate wins when its Map is larger than `stackedMap`; otherwise `stacked`.

`dashboard.js` measures the panel area and Health and calls it whenever the mode is column: after `_apply` (also when the mode did not change, since a resize within column changes the room), and from a `ResizeObserver` on `#vitals-bars` (a member joins or leaves). It sets `data-column` and `--col-map`. Health's natural height (`vitalsH`) is its title bar plus `#vitals-bars`' own height, which is the rows' height in every arrangement (the panel body scrolls when the cell is smaller), so the swap compares the panel's content, not the space it was given. While the keyboard is up the arrangement is kept (the ladder's 7b rule) and only `--col-map` is recomputed.

No oscillation: the arrangement changes the panels' widths, not Health's row heights (names do not wrap), so `vitalsH` does not move when it swaps.

### 3. Leaving column

Any other mode removes `data-column` and `--col-map`, so wide, slim and phone are untouched.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| Conditions (or Health when swapped) is at least 160px wide | Keeps "Slightly wet" and Health's 9ch name plus a bar readable | One constant (`SIDE_MIN`) |
| The swap point is a third of the panel column's height | About the title bar and five rows on a 1440×700 laptop: you and four companions keep layout A, a party of six or more swaps | One constant |
| The Conditions strip is 96px, its existing floor, with lines in columns | Reuses the floor stacked already gives it; columns fit several conditions in a short strip | Change the strip height |
| Stacked versus side is decided by the bigger Map | The owner's goal is the Map; a rule by size cannot pick a layout that shrinks it | Prefer side above some room width |
| The wide floor stays at 300px | At 1440×790 side would give a 515px Map against wide's 311px, but wide keeps every panel open, which the owner valued; only the column layout changes here | Raise `MAP_FLOOR`, or compare wide against side |
| This is plan 7c on #870 | Same PR as 7 and 7b, not yet merged | Its own PR |

## Out of scope

Wide, slim and phone. Making Conditions narrower than 160px to win more Map at 1280 wide (the owner chose A knowing 1280 gains little).

## Testing

- jstest `layoutpick`: `columnArrangement` at the measured 1440 (681×568, Health 92: side, about 477) and 1280 (521×568: side, 355) laptops; a narrow column (250 wide: stacked); Health at a third and just over (side, then swapped); a tall Health in a short column; the Map never above `w - 166` beside a panel; guards that the 120 / 96 floors and the 6px gap match `dashboard.css`.
- jstest source guards: `dashboard.js` sets `data-column` and `--col-map` from `columnArrangement`, observes `#vitals-bars`, and clears both outside column.
- Headless Edge on a private-port server: 1440×700 laptop with one, two and seven fake Health rows (side, side, swapped); 1280×700 (side); a 1100×700 window (stacked); screenshots for the owner.
- Gates: jstests, lint, `go test ./...`, boot.
