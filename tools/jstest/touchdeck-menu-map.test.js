// touchdeck-menu-map.test.js: touch arc plan 2c (#781, #938). The sideways
// side column's Menu button, the Map tab's button shape, and the bars a
// sideways phone folds away. Pure functions of touchdeck.js.
//
// Run:  node tools/jstest/touchdeck-menu-map.test.js
'use strict';

var path = require('path');
var TD = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function e(kind, id) { return { num: 1, dx: 0, dy: 0, dz: 0, details: [], kind: kind, id: id }; }
function room(v2, extra) {
    var r = { num: 5243, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}
function kinds(list) { return list.map(function (c) { return c.kind === 'move' ? c.dir : c.kind; }); }
var tower = room({ west: e('compass', 'm4'), shrine: e('named', 'x1'), ledge: e('named', 'x2') });

// --- shapes ------------------------------------------------------------------

var SIDE = TD.shape(0, 300, true, true);
var PHONE = TD.shape(344, 600, true, false);
var SMALL = TD.shape(300, 600, true, false);
check('the side column carries Menu', SIDE.menu, true);
check('an upright deck does not', PHONE.menu, false);
check('a tablet deck does not', TD.shape(788, 1000, false, false).menu, false);

var mapUp = TD.mapShape(PHONE, false);
check('the Map tab upright: 3 rows of 3 at the deck size (frame 7, D1)',
    [mapUp.rows, mapUp.cols, mapUp.button, mapUp.gap, mapUp.menu], [3, 3, 48, 8, false]);
// The Map grid has no padding of its own (the Map card's body has its own),
// so its width is 3 buttons and 2 gaps: 160px, as frame 7.
check('the Map column is 160px wide at 48px, as frame 7', mapUp.width, 160);
check('the Map grid has no padding of its own', mapUp.pad, 0);
var mapSide = TD.mapShape(SIDE, true);
check('the Map tab sideways is 104px tall (2 x 48 + 8)', mapSide.height, 104);
check('the Map tab sideways: 2 rows of 4 (D1)', [mapSide.rows, mapSide.cols], [2, 4]);
check('the Map tab takes the small size with a small deck (D11)',
    [TD.mapShape(SMALL, false).button, TD.mapShape(SMALL, false).gap], [44, 5]);

// --- Menu in the side column (D3) ------------------------------------------

var top = TD.cells(TD.moves(tower, ''), SIDE, 'top', 0);
check('frame 9: the T, the exits button under it, Menu bottom right',
    kinds(top), ['up', 'north', 'down', 'west', 'south', 'east', 'other', 'empty', 'empty', 'empty', 'empty', 'menu']);
check('before the first Room.Info only Menu shows',
    kinds(TD.cells(TD.moves(null, ''), SIDE, 'top', 0)), ['empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'menu']);
check('in the dark Menu stays',
    kinds(TD.cells(TD.moves(room({}, { dark: true }), ''), SIDE, 'top', 0))[11], 'menu');
check('upright, no Menu anywhere', kinds(TD.cells(TD.moves(tower, ''), PHONE, 'top', 0)).indexOf('menu'), -1);

// The exits list in the side column: 11 slots, Back first, Menu kept last.
var many = {};
'abcdefghijkl'.split('').forEach(function (ch, i) { many['door' + ch] = e('named', 'x' + i); });
var big = TD.moves(room(many), '');
var page0 = kinds(TD.cells(big, SIDE, 'exits', 0));
check('a long list: Back, nine exits, More, Menu', page0,
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'more', 'menu']);
var page1 = kinds(TD.cells(big, SIDE, 'exits', 1));
check('page two: Back, the last three, More, Menu', page1,
    ['back', 'exit', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'more', 'menu']);
var ten = {};
'abcdefghij'.split('').forEach(function (ch, i) { ten['door' + ch] = e('named', 'x' + i); });
check('ten exits fit: no More', kinds(TD.cells(TD.moves(room(ten), ''), SIDE, 'exits', 0)),
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'menu']);
var eleven = {};
'abcdefghijk'.split('').forEach(function (ch, i) { eleven['door' + ch] = e('named', 'x' + i); });
var m11 = TD.moves(room(eleven), '');
check('eleven exits overflow the side column: Back, nine exits, More in slot 10, Menu in 11',
    kinds(TD.cells(m11, SIDE, 'exits', 0)),
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'more', 'menu']);
check('eleven exits, page two: the remaining two',
    kinds(TD.cells(m11, SIDE, 'exits', 1)),
    ['back', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty', 'more', 'menu']);
check('the upright deck keeps More in its last slot (plan 2b)',
    kinds(TD.cells(big, PHONE, 'exits', 0)).slice(-1), ['more']);

// --- the Map tab's cells -------------------------------------------------------

check('Map upright: the T, the exits button under it, two empty slots',
    kinds(TD.cells(TD.moves(tower, ''), mapUp, 'top', 0)),
    ['up', 'north', 'down', 'west', 'south', 'east', 'other', 'empty', 'empty']);
check('Map sideways: the exits button beside the T',
    kinds(TD.cells(TD.moves(tower, ''), mapSide, 'top', 0)),
    ['up', 'north', 'down', 'other', 'west', 'south', 'east', 'empty']);
check('Map upright, exits: Back and the list, the map stays (the list replaces only the buttons)',
    kinds(TD.cells(TD.moves(tower, ''), mapUp, 'exits', 0)),
    ['back', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'empty']);

// Map tab paging: the list replaces the buttons and pages like the deck's.
var nine = {};
'abcdefghi'.split('').forEach(function (ch, i) { nine['door' + ch] = e('named', 'x' + i); });
var m9 = TD.moves(room(nine), '');
check('Map upright, 9 exits, page one: Back, seven exits, More last',
    kinds(TD.cells(m9, mapUp, 'exits', 0)),
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'more']);
check('Map upright, 9 exits, page two: Back, the last two, More last',
    kinds(TD.cells(m9, mapUp, 'exits', 1)),
    ['back', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'empty', 'more']);
var eight = {};
'abcdefgh'.split('').forEach(function (ch, i) { eight['door' + ch] = e('named', 'x' + i); });
var m8 = TD.moves(room(eight), '');
check('Map sideways, 8 exits, page one: Back, six exits, More last',
    kinds(TD.cells(m8, mapSide, 'exits', 0)),
    ['back', 'exit', 'exit', 'exit', 'exit', 'exit', 'exit', 'more']);
check('Map sideways, 8 exits, page two: Back, the last two, More last',
    kinds(TD.cells(m8, mapSide, 'exits', 1)),
    ['back', 'exit', 'exit', 'empty', 'empty', 'empty', 'empty', 'more']);

// --- a Menu tap ----------------------------------------------------------------

var st = { level: 'top', page: 0, guardUntil: 0 };
var r = TD.tap(st, { kind: 'menu' }, 1000);
check('a Menu tap asks for the bars and sends nothing', [r.menu, r.send, r.typed, r.level], [true, '', '', 'top']);
check('a Menu tap sets no hold (D10)', r.guardUntil, 0);
check('any other tap does not ask for the bars', TD.tap(st, { kind: 'back' }, 1000).menu, false);
check('a Menu tap inside a hold is ignored like any tap', TD.tap({ level: 'top', page: 0, guardUntil: 2000 }, { kind: 'menu' }, 1000).menu, false);

// --- the bars --------------------------------------------------------------------

check('no fold: the bars as always', TD.barsState(false, false), '');
check('no fold even if asked', TD.barsState(false, true), '');
check('folded', TD.barsState(true, false), 'folded');
check('shown by Menu', TD.barsState(true, true), 'shown');
check('a pick folds them', TD.barsStayAfterPick(false, false), false);
check('Settings open: they stay (D5)', TD.barsStayAfterPick(true, false), true);
check('Leave Game asking: they stay (D5)', TD.barsStayAfterPick(false, true), true);

// --- drawing Menu ----------------------------------------------------------------

function El(tag) { this.tagName = tag; this.children = []; this.attrs = {}; this.className = ''; this.textContent = ''; this.disabled = false; }
El.prototype.appendChild = function (c) { this.children.push(c); return c; };
El.prototype.removeChild = function (c) { this.children.splice(this.children.indexOf(c), 1); return c; };
El.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
Object.defineProperty(El.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });
var doc = { createElement: function (t) { return new El(t); }, createElementNS: function (ns, t) { return new El(t); } };
var deck = new El('div');
TD.render(deck, [{ kind: 'menu', open: false }], doc);
var m = deck.children[0];
check('Menu is a brass button with its own class', m.className, 'deck-btn brass deck-menu');
check('Menu tells a screen reader what it opens and whether it is open',
    [m.attrs['aria-label'], m.attrs['aria-expanded'], m.attrs['aria-controls']], ['Menu', 'false', 'dash-session dash-tabbar']);
check('Menu draws a line icon and its word', [m.children[0].tagName, m.children[1].textContent], ['svg', 'Menu']);
TD.render(deck, [{ kind: 'menu', open: true }], doc);
check('open: aria-expanded true', deck.children[0].attrs['aria-expanded'], 'true');
check('Menu is never disabled', deck.children[0].disabled, false);

process.exit(failures ? 1 : 0);
