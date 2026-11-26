// touchdeck-moves.test.js: moving by touch on the deck (touch arc plan 2b,
// docs/superpowers/specs/2026-11-22-moving-by-touch-design.md, mockup
// docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html). The Room.Info
// shapes below are plan 2a's (modules/gmcp/gmcp.Room.go).
//
// Run:  node tools/jstest/touchdeck-moves.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var ROOT = path.join(__dirname, '..', '..');
var TD = require(path.join(ROOT, '_datafiles', 'html', 'public', 'static', 'js', 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function ex(kind, id, details, extra) {
    var e = { num: 1, dx: 0, dy: 0, dz: 0, details: details || [], kind: kind, id: id };
    if (extra) Object.keys(extra).forEach(function (k) { e[k] = extra[k]; });
    return e;
}
function info(num, v2, extra) {
    var r = { num: num, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}

// Mockup frames 1 and 2: Tower Top, west plus two named exits.
var tower = info(5243, {
    west: ex('compass', 'm4'),
    shrine: ex('named', 'xa1b2c3d4'),
    ledge: ex('named', 'x0f0f0f0f')
});
// Frame 3: diagonals only.
var junction = info(6408, { southwest: ex('diagonal', 'x22222222'), northeast: ex('diagonal', 'x11111111') });
// Frame 4: down locked now.
var barracks = info(473, { down: ex('vertical', 'm6', ['locked']), south: ex('compass', 'm2'), up: ex('vertical', 'm5') });
// Frame 5: pitch dark after a bump north and a locked door felt west.
var dark = info(6418, {}, { dark: true, darkmarks: { north: 'wall', west: 'locked' } });
// Ruling 18 and R2: the viewer (43) may enter a party member's portal
// (arena-42) and their own (arena-43, marked own_portal); both read
// "arena". A portal they may not enter (tower-7) comes with no id.
var portals = info(900, {
    'arena-42': ex('temporary', 'x42424242', [], { label: 'arena' }),
    'arena-43': ex('temporary', 'x43434343', ['own_portal'], { label: 'arena' }),
    'tower-7': ex('temporary', '', [], { label: 'tower' }),
    plank: ex('temporary', 'x99999999')
});
// The Planar Oasis threshold: a temporary exit named north carries m1.
var oasis = info(901, { north: ex('temporary', 'm1') });

// --- moves: the model ------------------------------------------------------

check('no Room.Info yet: not ready', TD.moves(null, '').ready, false);
var m = TD.moves(tower, 'exploration');
check('Tower Top: only west is lit on the T',
    TD.T_DIRS.filter(function (d) { return m.t[d].lit; }), ['west']);
check('every T direction carries its fixed id', TD.T_DIRS.map(function (d) { return m.t[d].id; }),
    ['m5', 'm1', 'm6', 'm4', 'm2', 'm3']);
check('other exits A to Z by label (spec: named and temporary A to Z)',
    m.others.map(function (o) { return o.label; }), ['ledge', 'shrine']);
check('an other exit keeps its id and the name the server runs', m.others[1], { name: 'shrine', id: 'xa1b2c3d4', label: 'shrine', arrow: '', locked: false, own: false });
check('out of combat', m.combat, false);
check('combat comes from Commands.State', TD.moves(tower, 'combat').combat, true);
check('dead does not dim', TD.moves(tower, 'dead').combat, false);

var j = TD.moves(junction, '');
check('diagonals clockwise from northeast, with an arrow and their letters',
    j.others.map(function (o) { return o.arrow + o.label; }), ['↗NE', '↙SW']);

var b = TD.moves(barracks, '');
check('a door locked now carries the padlock', b.t.down.locked, true);
check('an open way does not', b.t.up.locked, false);

var d = TD.moves(dark, '');
check('dark: nothing is lit', TD.T_DIRS.some(function (k) { return d.t[k].lit; }), false);
check('dark: the marks ride on their directions', [d.t.north.mark, d.t.west.mark, d.t.east.mark], ['wall', 'locked', '']);
check('dark: no other exits', d.others.length, 0);
check('dark: every T id is still there (all six tappable)', TD.T_DIRS.every(function (k) { return d.t[k].id !== ''; }), true);
check('lit: no marks even if a stale one came', TD.moves(info(1, {}, { darkmarks: { north: 'wall' } }), '').t.north.mark, '');

var p = TD.moves(portals, '');
check('portals show their zone, the player\'s own first; one with no id gets no button',
    p.others.map(function (o) { return o.label + '=' + o.name; }), ['arena=arena-43', 'arena=arena-42', 'plank=plank']);
check('a temporary exit named north sits on the T, not in the list',
    [TD.moves(oasis, '').t.north.lit, TD.moves(oasis, '').others.length], [true, 0]);

// The server sorts known exits the same way (rooms.exitOrder); Room.Info's
// exitsv2 is a JSON object and loses it, so the client sorts again. Fails
// if the two orders part.
var goSrc = fs.readFileSync(path.join(ROOT, 'internal', 'rooms', 'known_exits.go'), 'utf8');
var order = /var exitOrder = map\[string\]int\{([\s\S]*?)\}/.exec(goSrc);
var ranks = {};
(order ? order[1] : '').replace(/"(\w+)":\s*(\d+)/g, function (_, k, v) { ranks[k] = Number(v); });
var serverDiagonals = Object.keys(ranks).filter(function (k) { return TD.DIAGONALS.indexOf(k) !== -1 || /^(north|south)(east|west)$/.test(k); })
    .sort(function (a, b) { return ranks[a] - ranks[b]; });
check('the client\'s diagonal order is the server\'s exitOrder', TD.DIAGONALS, serverDiagonals);

process.exit(failures ? 1 : 0);
