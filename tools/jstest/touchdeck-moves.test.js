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

// --- cells: the deck laid out ---------------------------------------------

var PHONE = TD.shape(344, 600, true, false);      // 2 x 6 at 48px (S21)
var SIDE = TD.shape(0, 300, false, true);         // 4 x 3: a phone on its side
var TABLET = TD.shape(788, 1000, false, false);   // 2 x 13
function kinds(cs) { return cs.map(function (c) { return c.kind === 'move' ? c.dir : c.kind; }); }

var top = TD.cells(TD.moves(tower, ''), PHONE, 'top', 0);
check('frame 1: the T on the left, the exits button beside it, the rest empty', kinds(top),
    ['up', 'north', 'down', 'other', 'empty', 'empty', 'west', 'south', 'east', 'empty', 'empty', 'empty']);
check('frame 1: exits is live (two named exits)', top[3].lit, true);
check('up and down draw the stairs; compass shows an arrow and its letter',
    [top[0].icon, top[1].arrow + top[1].label, top[2].icon], ['stairs-up', '↑N', 'stairs-down']);
check('frame 4: exits dims when the room has none', TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0)[3].lit, false);
check('frame 6: combat reaches every movement cell',
    TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0).filter(function (c) { return c.combat; }).length, 7);
check('a wider deck keeps the T and the exits slot where they are',
    kinds(TD.cells(TD.moves(tower, ''), TABLET, 'top', 0)).slice(0, 4), ['up', 'north', 'down', 'other']);
check('frame 9: on its side, the exits button sits under the T',
    kinds(TD.cells(TD.moves(tower, ''), SIDE, 'top', 0)).slice(0, 7), ['up', 'north', 'down', 'west', 'south', 'east', 'other']);

var dk = TD.cells(TD.moves(dark, ''), PHONE, 'top', 0);
check('frame 5: dark: the exits slot is the Dark tile', dk[3].kind, 'dark');
check('frame 5: dark: every T cell is dark and keeps its id',
    dk.filter(function (c) { return c.kind === 'move' && c.dark && c.id; }).length, 6);
check('frame 5: the wall mark north, the padlock mark west', [dk[1].mark, dk[6].mark], ['wall', 'locked']);
check('dark: the list never opens', TD.cells(TD.moves(dark, ''), PHONE, 'exits', 0)[3].kind, 'dark');

var list = TD.cells(TD.moves(tower, ''), PHONE, 'exits', 0);
check('frame 2: the list replaces the deck: Back top-left, then the exits', kinds(list).slice(0, 4), ['back', 'exit', 'exit', 'empty']);
check('frame 2: labels in order', [list[1].label, list[2].label], ['ledge', 'shrine']);
check('frame 2: no More when they fit', list.some(function (c) { return c.kind === 'more'; }), false);
check('frame 3: diagonals in the list keep arrow and letters',
    TD.cells(TD.moves(junction, ''), PHONE, 'exits', 0).slice(1, 3).map(function (c) { return c.arrow + c.label; }), ['↗NE', '↙SW']);
check('no room has more than two today, so an empty list falls back to the T',
    TD.cells(TD.moves(barracks, ''), PHONE, 'exits', 0)[0].kind, 'move');

var many = {};
for (var i = 0; i < 15; i++) many['p' + (i < 10 ? '0' : '') + i] = ex('named', 'x' + (10000000 + i));
var big = TD.moves(info(2, many), '');
var page0 = TD.cells(big, PHONE, 'exits', 0);
check('More bottom-right only when the list does not fit', page0[11].kind, 'more');
check('page 1 holds ten (twelve slots less Back and More)', page0.filter(function (c) { return c.kind === 'exit'; }).length, 10);
var page1 = TD.cells(big, PHONE, 'exits', 1);
check('page 2 holds the other five', page1.filter(function (c) { return c.kind === 'exit'; }).map(function (c) { return c.label; }),
    ['p10', 'p11', 'p12', 'p13', 'p14']);
check('More after the last page goes back to the first', TD.cells(big, PHONE, 'exits', 2)[1].label, 'p00');

// --- levelAfter: the deck returns to the T after a move ---------------------

var mt = TD.moves(tower, '');
check('after a move the deck returns to the T', TD.levelAfter('exits', 5243, 5346, TD.moves(info(5346, {}), '')), 'top');
check('the same room again keeps the list open', TD.levelAfter('exits', 5243, 5243, mt), 'exits');
check('the room going dark closes the list', TD.levelAfter('exits', 5243, 5243, TD.moves(info(5243, {}, { dark: true }), '')), 'top');
check('no other exits left closes the list', TD.levelAfter('exits', 5243, 5243, TD.moves(info(5243, { west: ex('compass', 'm4') }), '')), 'top');
check('the T stays the T', TD.levelAfter('top', 5243, 5243, mt), 'top');

// --- tap ------------------------------------------------------------------

var st = { level: 'top', page: 0, lastId: '', lastAt: 0 };
check('a lit arrow sends its fixed id', TD.tap(st, top[6], 1000).send, 'm4');
check('a dim arrow sends nothing', TD.tap(st, top[1], 1000).send, '');
check('a dark arrow sends its fixed id (all six tappable)', TD.tap(st, dk[1], 1000).send, 'm1');
check('a combat-dimmed arrow still sends (the server says why)',
    TD.tap(st, TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0)[6], 1000).send, 'm4');
var r1 = TD.tap(st, top[6], 1000), r2 = TD.tap(r1, top[6], 1100);
check('ruling 8: a fast double tap on the T sends twice', [r1.send, r2.send], ['m4', 'm4']);
var opened = TD.tap(st, top[3], 1000);
check('the exits button opens the list at its first page', [opened.level, opened.page], ['exits', 0]);
check('a dim exits button does nothing', TD.tap(st, TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0)[3], 1000).level, 'top');
var e1 = TD.tap(opened, list[2], 2000);
check('an exit sends its per-room id', e1.send, 'xa1b2c3d4');
check('a second tap on it within half a second is ignored', TD.tap(e1, list[2], 2000 + TD.DOUBLE_TAP_MS - 1).send, '');
check('after half a second it counts again', TD.tap(e1, list[2], 2000 + TD.DOUBLE_TAP_MS).send, 'xa1b2c3d4');
check('Back returns to the T', TD.tap(opened, list[0], 3000).level, 'top');
check('More turns the page', TD.tap({ level: 'exits', page: 0, lastId: '', lastAt: 0 }, page0[11], 1000).page, 1);
check('the Dark tile says what to do', TD.tap(st, dk[3], 1000).say, 'It is too dark to see the way. Light a torch or cast glow.');
check('an empty slot does nothing', TD.tap(st, top[4], 1000), { level: 'top', page: 0, lastId: '', lastAt: 0, send: '', say: '' });

// --- render: the buttons drawn --------------------------------------------

function FakeEl(tag, ns) {
    this.tagName = tag; this.ns = ns || ''; this.children = []; this.attrs = {};
    this.className = ''; this.textContent = ''; this.disabled = false; this.type = '';
}
FakeEl.prototype.appendChild = function (c) { this.children.push(c); return c; };
FakeEl.prototype.removeChild = function (c) { this.children.splice(this.children.indexOf(c), 1); return c; };
Object.defineProperty(FakeEl.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });
FakeEl.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
var doc = {
    createElement: function (t) { return new FakeEl(t); },
    createElementNS: function (ns, t) { return new FakeEl(t, ns); }
};
function words(el) { return el.textContent + el.children.map(words).join(''); }
function has(el, cls) { return (' ' + el.className + ' ').indexOf(' ' + cls + ' ') !== -1; }
function svgs(el) { return el.children.filter(function (c) { return c.tagName === 'svg'; }).map(function (c) { return c.attrs['class']; }); }

var deck = new FakeEl('div');
deck.appendChild(new FakeEl('span'));            // the old empty slots go
TD.render(deck, top, doc);
check('one child per cell', deck.children.length, 12);
check('an empty cell is a slot, not a button', [deck.children[4].tagName, deck.children[4].className], ['span', 'deck-slot']);
var west = deck.children[6], north = deck.children[1], up = deck.children[0], other = deck.children[3];
check('a lit arrow is a button with its cell index', [west.tagName, west.type, west.attrs['data-cell'], west.disabled], ['button', 'button', '6', false]);
check('it shows the arrow and its letter', words(west), '←W');
check('it reads plainly to a screen reader', west.attrs['aria-label'], 'Go west');
check('a movement button is square-edged by class', has(west, 'deck-move'), true);
check('R1: lit, dim, exits buttons wear the site brass', [has(west, 'brass'), has(north, 'brass'), has(other, 'brass')], [true, true, true]);
check('a dim arrow is disabled and says so', [north.disabled, has(north, 'deck-dim'), north.attrs['aria-label']], [true, true, 'No way north']);
check('up draws the stairs pictogram, not a letter', [svgs(up), words(up)], [['deck-icon'], '']);
check('the exits button: dashed by class, live, its words', [has(other, 'deck-other'), other.disabled, words(other)], [true, false, '⋯exits']);

TD.render(deck, TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0), doc);
var down = deck.children[2];
check('frame 4: a locked door carries the padlock', svgs(down), ['deck-icon', 'deck-lock']);
check('frame 4: and says it is locked', down.attrs['aria-label'], 'Go down (locked)');
check('frame 4: no other exits: the exits button is dim and disabled', [deck.children[3].disabled, has(deck.children[3], 'deck-dim')], [true, true]);

TD.render(deck, dk, doc);
check('frame 5: dark arrows are tappable in the dark style',
    [deck.children[7].disabled, has(deck.children[7], 'deck-blind')], [false, true]);
check('frame 5: the wall mark is a class (drawn by CSS)', has(deck.children[1], 'deck-mark-wall'), true);
check('frame 5: a locked door felt in the dark carries the padlock', svgs(deck.children[6]), ['deck-lock']);
check('frame 5: the Dark tile: closed eye and its word', [svgs(deck.children[3]), words(deck.children[3])], [['deck-icon'], 'Dark']);
check('R1: the pitch-dark buttons are olive grey, not brass', [has(deck.children[7], 'brass'), has(deck.children[3], 'brass')], [false, false]);

TD.render(deck, TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0), doc);
check('frame 6: combat dims by class and keeps the button live',
    [has(deck.children[6], 'deck-combat'), deck.children[6].disabled], [true, false]);

TD.render(deck, list, doc);
check('frame 2: Back and the names', [words(deck.children[0]), words(deck.children[1]), words(deck.children[2])], ['‹ Back', 'ledge', 'shrine']);
check('frame 2: a name is a deck-name span (12px in CSS)', deck.children[1].children[0].className, 'deck-name');
check('an exit name reaches the page as text, never markup', (function () {
    var evil = TD.cells(TD.moves(info(3, { '<b>x': ex('named', 'x12345678') }), ''), PHONE, 'exits', 0);
    TD.render(deck, evil, doc);
    return deck.children[1].children[0].textContent;
}()), '<b>x');
TD.render(deck, page0, doc);
check('More reads More', words(deck.children[11]), 'More ›');
check('Back and More wear the brass too', [has(deck.children[0], 'brass'), has(deck.children[11], 'brass')], [true, true]);
TD.render(deck, TD.cells(TD.moves(portals, ''), PHONE, 'exits', 0), doc);
check('two portals to one zone: the same label, the player\'s own first and named so for a screen reader',
    [words(deck.children[1]), deck.children[1].attrs['aria-label'], words(deck.children[2]), deck.children[2].attrs['aria-label']],
    ['arena', 'Go arena (yours)', 'arena', 'Go arena']);

process.exit(failures ? 1 : 0);
