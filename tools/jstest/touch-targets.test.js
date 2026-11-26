// touch-targets.test.js: guard 6 of the touch arc roadmap
// (docs/roadmaps/TOUCH_ARC_ROADMAP.md, "Tests and guards"). At every size in
// the test matrix, every deck slot is at least 48px with 8px gaps. The only
// exception is 44px with 5px gaps where 48px cannot fit (spec ruling 3,
// 2026-11-21). The deck must also fit its box (no sideways scroll), and the
// stylesheet must size it from touchdeck.js, never from numbers of its own.
//
// Run:  node tools/jstest/touch-targets.test.js
'use strict';

var path = require('path');
var fs = require('fs');
var js = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var TD = require(path.join(js, 'touchdeck.js'));
var LayoutPick = require(path.join(js, 'layoutpick.js'));
var css = fs.readFileSync(path.join(js, '..', 'css', 'dashboard.css'), 'utf8');

var failures = 0;
function check(name, ok) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name);
}

var PAD = LayoutPick.PAD;          // #dashboard padding (layoutpick.test.js ties it to the CSS)
var BORDER = 2;                    // the slot's side borders (dashboard.css #touch-slot)
var SESSION = 64, TABBAR = 64;     // the session bar with its gap; the tab bar's room on touch (--tabbar-h)
var GAME_W = LayoutPick.GAME_W_DEFAULT;   // the game column outside phone mode

function span(n, b, g) { return n * b + (n - 1) * g + 2 * TD.PAD; }

// What is wrong with one deck shape in its box: a list, empty when it passes.
function problems(s, boxW, boxH) {
    var out = [];
    var big = s.button >= TD.BTN && s.gap >= TD.GAP;
    var small = s.button >= TD.SMALL_BTN && s.gap >= TD.SMALL_GAP;
    var bigFits = s.side ? span(s.rows, TD.BTN, TD.GAP) <= boxH : span(s.cols, TD.BTN, TD.GAP) <= boxW;
    if (!big && !(small && !bigFits)) out.push(s.button + 'px buttons with ' + s.gap + 'px gaps');
    if (!s.side && s.width > boxW) out.push(s.width + 'px wide in a ' + boxW + 'px box');
    if (s.side && s.height > boxH) out.push(s.height + 'px tall in a ' + boxH + 'px box');
    return out;
}

// The size matrix (roadmap "How we test"), plus the WCAG reflow width.
var MATRIX = [
    ['small iPhone in Safari', 375, 548],
    ['Galaxy S21', 360, 800],
    ['Pixel 8', 412, 915],
    ['WCAG reflow width', 320, 640],
    ['phone on its side', 800, 360],
    ['iPad upright', 820, 1180],
    ['iPad sideways', 1180, 820],
    ['open foldable', 690, 840],
    ['laptop', 1366, 768]
];
MATRIX.forEach(function (m) {
    var w = m[1], h = m[2];
    var phone = Math.min(w, h) < 600;
    var sideways = phone && w > h;
    var mode = LayoutPick.pick(w, GAME_W, phone, h);
    var colW = mode === 'phone' ? w - 2 * PAD : GAME_W;
    var boxW = colW - BORDER;
    var boxH = h - 2 * PAD - SESSION - TABBAR;
    var s = TD.shape(boxW, boxH, phone && !sideways, sideways);
    var p = problems(s, boxW, boxH);
    check(m[0] + ' ' + w + 'x' + h + ': ' + s.rows + 'x' + s.cols + ' at ' + s.button + 'px' +
          (p.length ? ' (' + p.join('; ') + ')' : ''), p.length === 0);
});

// The guard can fail.
check('the guard fails a 47px slot',
    problems({ rows: 2, cols: 6, button: 47, gap: 8, side: false, width: 338, height: 118 }, 346, 600).length > 0);
check('the guard fails 44px where 48px fits',
    problems({ rows: 2, cols: 6, button: 44, gap: 5, side: false, width: 305, height: 109 }, 346, 600).length > 0);
check('the guard fails a deck wider than its box',
    problems({ rows: 2, cols: 6, button: 48, gap: 8, side: false, width: 344, height: 120 }, 300, 600).length > 0);

// The stylesheet sizes the deck from touchdeck.js (dashboard.js sets the
// variables), never with numbers of its own.
var deckRule = (/#touch-deck \{[^}]*\}/.exec(css) || [''])[0];
check('the deck grid takes its button size from --deck-btn', /var\(--deck-btn/.test(deckRule));
check('the deck grid takes its gap from --deck-gap', /var\(--deck-gap/.test(deckRule));
var slotRule = (/\.deck-slot \{[^}]*\}/.exec(css) || [''])[0];
check('a deck slot is styled', slotRule !== '');
check('a deck slot sets no size of its own', !/(^|[;{\s])(width|height|min-width|min-height)\s*:/.test(slotRule));

// Plan 2b: a deck button fills its grid cell and sets no size of its own.
var btnRule = (/\.deck-btn \{[^}]*\}/.exec(css) || [''])[0];
check('a deck button is styled', btnRule !== '');
check('a deck button sets no size of its own', btnRule !== '' && !/(^|[;{\s])(width|height|min-width|min-height)\s*:/.test(btnRule));

// Owner ruling 18: the longest exit name fits its button at both sizes.
// JetBrains Mono's letters are 0.6em wide; a named button has a 1px edge
// each side. Task 11 of plan 2b measures it in the browser too.
var gomud = fs.readFileSync(path.join(js, '..', 'css', 'gomud.css'), 'utf8');
var labelPx = Number((/--deck-label:\s*(\d+)px;/.exec(gomud) || [])[1]);
var tight = Number((/#touch-deck\.deck-small \.deck-name \{[^}]*letter-spacing:\s*(-?[\d.]+)px/.exec(css) || [])[1]);
var LETTER = 0.6 * labelPx, EDGES = 2;
check('the deck name size is read from --deck-label', labelPx > 0);
check('a ' + TD.NAME_LETTERS + '-letter name fits a ' + TD.BTN + 'px button', TD.NAME_LETTERS * LETTER <= TD.BTN - EDGES);
check('a ' + TD.NAME_LETTERS + '-letter name fits a ' + TD.SMALL_BTN + 'px button with the small spacing',
    !isNaN(tight) && TD.NAME_LETTERS * (LETTER + tight) <= TD.SMALL_BTN - EDGES);
check('the fit check can fail: seven letters do not fit at the small size', 7 * (LETTER + tight) > TD.SMALL_BTN - EDGES);

process.exit(failures ? 1 : 0);
