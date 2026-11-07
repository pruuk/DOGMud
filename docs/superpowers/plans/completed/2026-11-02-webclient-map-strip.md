# Map strip: legend and zoom buttons under the map

> Spec: [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md)
> ("Map strip"). Status: BAR done (findings below, fixed in the tasks);
> Malia chose the simpler version; built 2026-11-02 and checked by her
> (localhost and phone). Part 1 (the
> legend) was built on 2026-11-02 before this plan existed; this plan covers
> both parts (the simpler version kept part 1's `<details>`).

**Branch:** `feat/game-settings` (PR #768). Files: `static/js/gmcp.js`,
`webclient-pure.html`, `static/css/dashboard.css` (all under
`_datafiles/html/public/`). No server work.

## Goal
Nothing drawn over the map's rooms except the map itself and its compass.
The legend and the four zoom buttons sit in one strip under the map, at
reading size, in every place the Map panel appears (wide column, rail tab,
phone drawer, pop-out window).

```
[ Fit ] [ Center ] [ − ] [ + ]
▾ Map legend
Party member · Bank / Shop / Store · Stairs ▲▼ · Road / trail / water ·
Quest destination · Next step                   (while the legend is open)
```

**Malia's choice (2026-11-02, after the BAR):** the simpler version. The
buttons get their own row; the legend keeps the native `<details>` from
part 1, which gives the fold, keyboard and state handling for free. Tasks 3,
4 and 6 below are written for this version.

## What the code shows
- `RoomGridSVG` (`gmcp.js`) draws into `#map-render`. The legend card was
  part of its SVG overlay (`overlayGroup`, design space 320 x 300); part 1
  deleted that block and pointed comments at the new strip.
- `_createHTMLControls()` (~1340) injects a `<style id="rgsvg-ctl-style">`
  for `.rgsvg-brass` (24px) and `.rgsvg-controls`, then appends a
  `div.rgsvg-controls` to `this.container` with inline
  `position:absolute; bottom/left: controlsMargin`. Buttons: `fit`, `ctr`,
  `−`, `+`, each with `aria-label` and `title`; the group has
  `role="group"`, `aria-label="Map controls"`.
- `_applyControlsMode()` plus a `ResizeObserver` switch the group to
  `compact` (32% opacity, 80% scale) on a small map
  (`RoomGridSVG.controlsModeForSize`).
- The map is created once: `new RoomGridSVG('#map-render', mapOptions)`
  (`webclient-pure.html` ~621, options ~617). `this.container.style.position`
  is set to `relative` (~247), so absolute children anchor to it.
- No `tools/jstest` file loads `gmcp.js`, but CI also runs
  `tools/webclient-tests/*.js` (three of them evaluate the real `gmcp.js`
  and build a `RoomGridSVG`: zone-map-diff, zone-crossing-center,
  quest-destination-map) and `tools/test_rgsvg_controls.mjs` tests
  `controlsModeForSize`. `tools/webclient-tests/quest-destination-demo.html`
  builds a map with no host.
- `#panel-map` has `min-height: 150px` (header included); the pop-out window
  is 360 x 300. The injected `rgsvg-ctl-style` loads after `dashboard.css`,
  so equal-specificity rules there lose to it.
- Part 1 markup: `.dash-panel-body.map-body` holds `#map-render` and
  `details.map-legend` (summary "Map legend", a `ul` of six items with
  inline SVG symbols). CSS: `#panel-map .map-body` is a flex column,
  `#map-render` flexes, `.map-legend` styles with measured contrast.

## Tasks
1. **`controlsHost` option** (`gmcp.js` constructor ~220 and
   `_createHTMLControls`): `this.controlsHost = options.controlsHost || null`
   (element or selector, resolved with `document.querySelector` when a
   string). When set and found: append the group there, no inline absolute
   positioning, no `ResizeObserver`, and `_applyControlsMode()` returns early
   when hosted (it is also called directly, and would overwrite the class);
   the class is `rgsvg-controls hosted`. Given but not found: `console.warn`
   and fall back to today's overlay. Unset: today's placement (the labels
   in Task 2 change for every instance). Keep the injected stylesheet; add
   `.rgsvg-controls.hosted .rgsvg-brass{min-width:32px;height:32px;font-size:14px;}`
   to it.
2. **Labels:** `fit` → `Fit`, `ctr` → `Center`; aria-label/title
   "Fit map to view", "Center on your room", "Zoom out", "Zoom in". Update
   stale comments: the tiny-glyph comment in `_createHTMLControls`, the
   `_drawQuestBanner` comment (~711, "buttons sit bottom left"), the
   `controlsModeForSize` comment (~1465), and part 1's comments naming
   `.map-legend` / `<details>` (gmcp.js ~971, webclient-pure.html ~283,
   dashboard.css ~213).
3. **Markup** (`webclient-pure.html`, Map panel body): add
   `div#map-controls.map-controls` between `#map-render` and the existing
   `details.map-legend` (kept as built in part 1).
4. **No toggle script:** `<details>` handles fold, keyboard and state.
5. **Pass the host:** `mapOptions.controlsHost = '#map-controls'`. The host
   exists before `new RoomGridSVG` runs (it is in the static markup).
6. **CSS** (`dashboard.css`, beside the `.map-legend` block):
   `.map-controls` flex row, gap 5px, top border `var(--panel-border)`,
   padding 6px 10px; the legend below keeps its own border. Touch sizes use
   `#map-controls .rgsvg-brass` inside `@media (pointer: coarse)` (the ID
   out-ranks the injected stylesheet, which loads later): at least 44px.
   Measure the button label `#3b2a10` on the brass middle stop `#cb9f42`
   and write it in the comment.
7. **Keep the map usable when small:** `#panel-map` min-height 150px → 230px,
   and `#map-render` min-height 120px, so header + strip + an open legend
   still leave a map, including in the 360 x 300 pop-out.
8. **Pop-out and drawer:** no markup change; the strip moves with the one
   `#panel-map` node (WinBox `mount: panel`, rail and drawer `appendChild`).
   Check both, including sizes.

## Definition of done
- No HTML control or legend over the rooms at any size.
- Fit, Center, −, + work from the strip (the map visibly fits, centres,
  zooms); keyboard reaches them in order, then the legend summary.
- The legend folds and unfolds by click, Enter and Space.
- Text 4.5 or more, including the button labels, numbers in the CSS
  comment; 44px targets on touch.
- The map keeps at least 120px of height at the panel's minimum and in the
  pop-out window.
- A `RoomGridSVG` without `controlsHost` keeps its overlay placement and
  compact mode (only the labels change).
- `node --check gmcp.js`; all `tools/jstest`, all `tools/webclient-tests`
  (as CI runs them: `node <file>` for each) and
  `tools/test_rgsvg_controls.mjs` pass; open
  `tools/webclient-tests/quest-destination-demo.html` and check it still
  draws.

## BAR (2026-11-02)
A blind adversarial review of this plan found: `hidden` would not hide a
`display:flex` list; the touch-size rule would lose to the injected
stylesheet; `_applyControlsMode()` would overwrite the hosted class; the map
could shrink to 20 to 40px at the panel minimum and in the pop-out; the CI
`tools/webclient-tests` and `test_rgsvg_controls.mjs` were missing; the list
layout was ambiguous and the ▾ glyph would be read aloud; stale comments;
button label contrast unmeasured. All are fixed in the tasks above. It also
confirmed the host exists before the map is built, nothing else depends on
the controls being inside the map, and the panel node is moved, never
cloned. Its simpler alternative (keep `<details>`, buttons on a row above) was
taken (Malia, 2026-11-02), which also removes the `hidden` and toggle
findings.

## Verification
Fresh headless Chrome profile (the frame caches CSS); rooms fed with
`handleGMCP('Room.Info', ...)` in the game frame; `/webclient` at 1366, 820
(Map rail tab) and 390 (Map drawer), plus a popped-out Map at 1366; click
each button and compare the map's transform before and after; Tab through
the strip; fold and unfold the legend and see the list hide; measure the
map's height at the panel minimum and in the pop-out. Malia reviews on localhost; then one commit (legend + buttons)
pushed to PR #768.
