// layoutpick.test.js: which layout the dashboard uses (owner ladder 2026-11-12).
//
// Run:  node tools/jstest/layoutpick.test.js
'use strict';

var path = require('path');
var LayoutPick = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'layoutpick.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + got + ', want ' + want);
}

// The game column is 785px (80 columns, border and slack); the room right of
// it is W - 2*PAD - 785 - GAP. PAD and GAP are read from dashboard.css, as
// viewportfit.test.js:110 reads its CSS, so the test fails if the stylesheet
// and layoutpick.js drift apart.
var fs = require('fs');
var css = fs.readFileSync(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'css', 'dashboard.css'), 'utf8');
var block = css.slice(css.indexOf('#dashboard {'), css.indexOf('}', css.indexOf('#dashboard {')));
var cssPad = parseFloat(/padding:\s*([\d.]+)px/.exec(block)[1]);
// The gap between the game and the panels is an explicit grid track in the
// wide grid (the 6px between the game column and the panels; the column gap
// itself is 0), read from the wide-mode grid-template-columns.
var wideStart = css.indexOf('#dashboard[data-mode="wide"] {');
var wide = css.slice(wideStart, css.indexOf('}', wideStart));
var cssGap = parseFloat(/var\(--game-w[^)]*\)\s*\)?\s*([\d.]+)px/.exec(wide)[1]);
check('layoutpick uses the stylesheet padding', LayoutPick.PAD, cssPad);
check('layoutpick uses the stylesheet gap', LayoutPick.GAP, cssGap);
// The game column before termfit measures: the stylesheet's fallback in the
// default grid and the dashboard's default must be the same width.
var cssDefault = parseFloat(/var\(--game-w,\s*([\d.]+)px\)/.exec(block)[1]);
check('the default game width matches the stylesheet fallback', LayoutPick.GAME_W_DEFAULT, cssDefault);
var G = 785;
// A frame tall enough that only width decides (the Map's height share is 420px).
var TALL = 1000;
check('a monitor gets the full layout', LayoutPick.pick(1920, G, false, TALL), 'wide');
// Owner 2026-11-15: wide only when its square Map is at least 300px. Width
// alone now needs 518px of room for that, so 360 to 517px of room, which was
// wide under the 2026-11-12 ladder, is one column.
check('518px of room is the full layout', LayoutPick.pick(12 + G + 6 + 518, G, false, TALL), 'wide');
check('517px of room is one column', LayoutPick.pick(12 + G + 6 + 517, G, false, TALL), 'column');
check('360px of room is one column now', LayoutPick.pick(12 + G + 6 + 360, G, false, TALL), 'column');
check('359px of room is one column', LayoutPick.pick(12 + G + 6 + 359, G, false, TALL), 'column');
check('200px of room is one column', LayoutPick.pick(12 + G + 6 + 200, G, false, TALL), 'column');
check('199px of room is the slim column', LayoutPick.pick(12 + G + 6 + 199, G, false, TALL), 'slim');
check('120px of room is the slim column', LayoutPick.pick(12 + G + 6 + 120, G, false, TALL), 'slim');
check('119px of room is the phone layout', LayoutPick.pick(12 + G + 6 + 119, G, false, TALL), 'phone');
check('narrower than the game is the phone layout', LayoutPick.pick(640, G, false, TALL), 'phone');
check('a phone is always the phone layout', LayoutPick.pick(1920, G, true, TALL), 'phone');
check('no game width yet keeps the phone layout', LayoutPick.pick(1920, null, false, TALL), 'phone');

// A short frame (a laptop with the site header showing) steps down to column.
check('a 769px frame keeps the full layout', LayoutPick.pick(1920, G, false, 769), 'wide');
check('a 768px frame is one column', LayoutPick.pick(1920, G, false, 768), 'column');
// Measured 2026-11-15: a 1440x790 window with the header showing has a 667px
// frame and a 741px game; a 1440x900 screen in browser full screen has 900.
check('a 1440 laptop with the header showing is one column', LayoutPick.pick(1440, 741, false, 667), 'column');
check('a 1440 laptop in full screen is the full layout', LayoutPick.pick(1440, 741, false, 900), 'wide');
// A short frame never pushes a narrow window below column.
check('a short frame does not change slim', LayoutPick.pick(12 + G + 6 + 150, G, false, 500), 'slim');

// wideMapSize is the JavaScript copy of dashboard.css --map-w.
function near(a, b) { return Math.abs(a - b) < 0.01; }
check('wideMapSize is width bound when tall', near(LayoutPick.wideMapSize(500, 1200), 290), true);
check('wideMapSize is height bound when short', near(LayoutPick.wideMapSize(1000, 692), 260), true);
check('wideMapSize at the measured 1440x790 laptop', near(LayoutPick.wideMapSize(681, 667), 247), true);
check('the Map floor is 300px', LayoutPick.MAP_FLOOR, 300);

// The CSS formula and layoutpick.js must not drift apart.
var mapW = /--map-w:\s*min\(([\d.]+)%,\s*calc\(\(100vh - ([\d.]+)px\) \* ([\d.]+)\)\);/.exec(css);
check('the stylesheet still has the --map-w formula', !!mapW, true);
if (mapW) {
    check('the Map width share matches the stylesheet', parseFloat(mapW[1]) / 100, LayoutPick.MAP_SHARE);
    check('the height the Map leaves matches the stylesheet', parseFloat(mapW[2]), LayoutPick.MAP_H_TAKEN);
    check('the Map height share matches the stylesheet', parseFloat(mapW[3]), LayoutPick.MAP_H_SHARE);
}

// The column layout's arrangements (plan 7c, owner 2026-11-15): Conditions
// beside a larger Map with Health below (side), Health beside it with
// Conditions below once Health passes a third of the column (swapped), or
// today's stack where that gives the larger Map.
function arr(w, h, v) { var r = LayoutPick.columnArrangement(w, h, v); return r.arrangement + ' ' + r.map; }
check('a 1440 laptop puts Conditions beside the Map', arr(681, 568, 92), 'side 470');
check('a 1280 laptop does too, width bound', arr(521, 568, 92), 'side 355');
check('a narrow column stays stacked', arr(250, 568, 92), 'stacked 250');
check('Health at a third stays the strip', arr(681, 600, 200), 'side 394');
check('Health past a third swaps with Conditions', arr(681, 600, 201), 'swapped 498');
check('a tall Health in a short column swaps', arr(681, 400, 300), 'swapped 298');
check('a very tall column keeps the full-width stacked Map', arr(700, 2000, 92), 'stacked 700');
check('columnMap gives the kept arrangement its size', LayoutPick.columnMap('side', 681, 400, 92), 302);
check('columnMap for stacked is the stylesheet formula', LayoutPick.columnMap('stacked', 681, 300, 92), 120);

// The floors and gap columnArrangement assumes are the stylesheet's.
var colVitals = /#dashboard\[data-mode="column"\] #panel-vitals \{ grid-area: vitals; min-height: ([\d.]+)px; \}/.exec(css);
var colStatus = /#dashboard\[data-mode="column"\] #panel-status \{ grid-area: status; min-height: ([\d.]+)px; \}/.exec(css);
var colMapMin = /width: min\(100%, max\(([\d.]+)px, calc\(100vh - 300px - var\(--tabbar-h\)\)\)\);/.exec(css);
var colStrip = /grid-template-rows: var\(--col-map\) ([\d.]+)px;/.exec(css);
check('the Health floor matches the stylesheet', colVitals && parseFloat(colVitals[1]), LayoutPick.VITALS_FLOOR);
check('the Conditions floor matches the stylesheet', colStatus && parseFloat(colStatus[1]), LayoutPick.STATUS_FLOOR);
check('the stacked Map minimum matches the stylesheet', colMapMin && parseFloat(colMapMin[1]), LayoutPick.STACKED_MAP_MIN);
check('the swapped Conditions strip is the Conditions floor', colStrip && parseFloat(colStrip[1]), LayoutPick.STATUS_FLOOR);
check('the column gap matches the stylesheet', /gap: var\(--dash-gap, 6px\)/.test(css) && LayoutPick.COL_GAP, 6);

process.exit(failures ? 1 : 0);
