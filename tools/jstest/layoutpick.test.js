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
check('a monitor gets the full layout', LayoutPick.pick(1920, G, false), 'wide');
check('360px of room is the full layout', LayoutPick.pick(12 + G + 6 + 360, G, false), 'wide');
check('359px of room is one column', LayoutPick.pick(12 + G + 6 + 359, G, false), 'column');
check('200px of room is one column', LayoutPick.pick(12 + G + 6 + 200, G, false), 'column');
check('199px of room is the slim column', LayoutPick.pick(12 + G + 6 + 199, G, false), 'slim');
check('120px of room is the slim column', LayoutPick.pick(12 + G + 6 + 120, G, false), 'slim');
check('119px of room is the phone layout', LayoutPick.pick(12 + G + 6 + 119, G, false), 'phone');
check('narrower than the game is the phone layout', LayoutPick.pick(640, G, false), 'phone');
check('a phone is always the phone layout', LayoutPick.pick(1920, G, true), 'phone');
check('no game width yet keeps the phone layout', LayoutPick.pick(1920, null, false), 'phone');

process.exit(failures ? 1 : 0);
