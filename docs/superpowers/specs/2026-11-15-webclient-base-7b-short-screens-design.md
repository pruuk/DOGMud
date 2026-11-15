# Web client base plan 7b: short screens (laptops)

Status: design approved by the owner 2026-11-15 in chat; this file awaits his review.
Parent: `docs/superpowers/specs/2026-11-14-webclient-base-7-panels-design.md` (plan 7, PR #870), under the touch epic #783. Built on `feature/webclient-base-7-panels`.
Trigger: Malia's two Mac laptops show a tiny Map with every panel open. Full screen (site header and footer hidden) makes the panels usable again (owner, 2026-11-15).

## Facts verified against source

Read 2026-11-15 from `feature/webclient-base-7-panels` at `5d53915c8`. `P` is `_datafiles/html/public`.

| Fact | Where |
|---|---|
| The ladder picks a mode from width alone: `pick(dashW, gameW, phone)`, `room = dashW - 2*PAD - gameW - GAP`; wide at 360px of room or more, column 200 to 359, slim 120 to 199, phone below | `P/static/js/layoutpick.js:24`, `:36-43` |
| Its one caller passes `#dashboard.clientWidth`, the measured game width (or 790) and the phone test | `P/static/js/dashboard.js:320-325` |
| The ladder re-runs on every window resize | `P/static/js/dashboard.js:186-187`, `:336-338` |
| Wide's Map is square at `--map-w: min(58%, calc((100vh - 192px) * 0.52))`, where 58% is of `#dash-panels`, whose width is the ladder's `room` | `P/static/css/dashboard.css:464` |
| Column's Map is `min(100%, max(120px, calc(100vh - 300px - var(--tabbar-h))))`; `--tabbar-h` is 60px | `P/static/css/dashboard.css:545`, `:712` |
| Column keeps Map, Health and Conditions open and puts Comms, Quests, Inventory and Automation on the tab bar | `P/static/js/dashboard.js:1-12`; `P/static/css/dashboard.css:504-559` |
| `isPhone(w, h)` is a short side under 600 | `P/static/js/viewportfit.js:18-20` |
| `wantFullScreen(pref, phone)`: a saved `'1'` or `'0'` wins, otherwise `phone` | `P/static/js/viewportfit.js:82-86` |
| `enterFullScreenOnConnect(phone, pref, browserCan, browserOn)` is `phone && pref !== '0' && browserCan && !browserOn` | `P/static/js/viewportfit.js:139-141` |
| The outer page hides `header` and `footer` at start when `wantFullScreen(readPref(), isPhone(screen))`; the choice is saved under `dogmud.fullscreen` only by the button | `P/webclient.html:57`, `:86-100`, `:107-110` |
| Enter Game asks the outer page `wantsFullScreenOnConnect()` and, if so, requests the browser's full screen inside that click | `P/webclient.html:115-119`; `P/webclient-pure.html:2738-2751`, `:2894-2897` |
| Leaving the browser's full screen with Esc writes no preference; the header stays as it was | `P/webclient.html:101-105` |
| Tests: `tools/jstest/layoutpick.test.js:37-45` (nine `pick` cases); `tools/jstest/viewportfit.test.js:76-79`, `:225-226` (full screen defaults) | |

Measured 2026-11-15 on the LAN test server (headless Edge; its game window measured 741px wide, a real Mac's may differ slightly). The outer page's header is 80px and its footer 43px, so the game frame is the window's height less 123px.

| Browser window | Frame height | Mode today | Map today | Column's Map |
|---|---|---|---|---|
| 1280×700 | 577 | wide | 200 | 217 |
| 1280×800 | 677 | wide | 252 | 317 |
| 1440×790 | 667 | wide | 247 | 307 |
| 1440×900 | 777 | wide | 304 | 417 |
| 1470×830 | 707 | wide | 268 | 347 |
| 1920×950 | 827 | wide | 330 | 467 |

## What players see

On a laptop the game opens without the site's header and footer, and Enter Game takes the browser full screen. There the Map is large enough for the wide layout and every panel stays open. A player who leaves full screen gets Map, Health and Conditions beside the game, with Comms, Quests, Inventory and Automation on the round buttons, and a Map larger than today's. A desktop monitor is unchanged.

## Design

### 1. Laptops open full screen

- New `ViewportFit.isShortScreen(w, h)`: the screen's short side is under 1000px and it is not a phone (`!isPhone(w, h)`).
- `wantFullScreen(pref, phone)` and `enterFullScreenOnConnect(phone, ...)` rename their `phone` parameter to `byDefault`. Their two callers in `webclient.html` pass `isPhone(...) || isShortScreen(...)`. Their bodies do not change, so a saved `'0'` still wins on a laptop as on a phone.
- Today's phone behaviour is unchanged.

### 2. The ladder checks the Map

- `pick(dashW, gameW, phone, viewH)` gains the frame's height (`window.innerHeight` in the frame). New `wideMapSize(room, viewH)` returns `min(0.58 * room, (viewH - 192) * 0.52)`, the JavaScript copy of `--map-w`.
- Wide when `wideMapSize(room, viewH) >= 300`. Otherwise column, slim and phone as today, by room. The `WIDE = 360` room test goes, because a 300px Map needs at least 518px of room, so it can no longer decide anything.
- Leaving or entering full screen resizes the frame, so the existing resize handler re-runs the ladder. Full screen gives wide, and leaving it gives column.
- A jstest reads `dashboard.css`, pulls the `--map-w` expression, and fails if its constants (58%, 192px, 0.52) differ from `layoutpick.js`'s, so the two cannot drift.

With the 300px floor and the header still showing, every row of the table above becomes column except the 1440×900 window (304) and the 1920×950 monitor. With the header hidden at load (section 1) a laptop's frame gains 123px, so its Map in wide grows too. On a 1440×900 Mac in browser full screen the frame is 900px: the Map is 368px and the layout is wide.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| 300px floor for wide's Map | Every laptop size measured is 200 to 304 today, and Malia called those tiny; the 1080p monitor at 330 stays wide | One constant in `layoutpick.js` |
| The 360px room boundary of the ladder ruling (2026-11-12) is replaced by the Map floor, so 360 to 517px of room now gets column | At that width wide's Map is 209 to 299px, the same complaint as a short window | Keep `room >= 360` as a second condition |
| Laptop means a screen short side under 1000px, not a phone | Covers every 13" and 14" MacBook (900 to 982) and 1366×768 or 1536×864 laptops; excludes 1080p and taller monitors | One constant in `viewportfit.js` |
| Laptops get the browser's full screen on Enter Game too, not only the hidden header | The browser's own bars are another 80 to 120px on a Mac; the owner asked for full screen by default | Pass `isPhone` alone to `enterFullScreenOnConnect` |
| Esc out of browser full screen still writes no preference, as on phones | Unchanged phone behaviour; the next Enter Game asks again | Write `'0'` on a `fullscreenchange` exit |
| No hysteresis at the floor | The ladder has none today; a resize across a boundary already switches mode | Add a few px of hysteresis |

## Out of scope

Health and Conditions floors in column, the tab bar's height, and slim. Desktop monitors' wide layout.

## Testing

- jstest `layoutpick`: the nine existing cases gain a tall `viewH` (1000) and keep their results except room 360 to 517, which now gives column (stated in the test). New cases: a 1440px dashboard with a 741px game is column at a 667px frame and wide at 900; the floor's edge at 299 and 300; `wideMapSize` against the CSS formula at three sizes; the CSS constant guard.
- jstest `viewportfit`: `isShortScreen` for 1440×900, 1512×982, 1536×864 (true), 1920×1080, 390×844 phone (false); the renamed defaults with a laptop and a saved `'0'`.
- Headless Edge on a private-port server: 1440×790 loads in column with the header hidden; after `setFullScreen(false)` the header shows; a 1440×900 page with the header hidden is wide.
- Real devices: Malia's two Macs, and the phones (unchanged).
- Gates: build, lint, `go test ./...`, jstests, boot.
