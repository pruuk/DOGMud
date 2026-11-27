// dashboard-deck.test.js: dashboard.js keeps the deck's movement state,
// draws it and sends a tap's id (touch arc plan 2b, #781).
//
// Run:  node tools/jstest/dashboard-deck.test.js
//
// Loads the real static/js/dashboard.js and touchdeck.js and runs the real
// applyDeck against a small fake DOM: the deck redraws only when a cell
// changes, takes the small and dark classes, and keeps the keyboard focus
// across a redraw. What each cell looks like is touchdeck.js render's job
// (touchdeck-moves.test.js). Dependency free, exits non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');
var vm = require('vm');

var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var TD = require(path.join(JS, 'touchdeck.js'));
var SRC = fs.readFileSync(path.join(JS, 'dashboard.js'), 'utf8');

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}

// --- a small fake DOM -------------------------------------------------------

var doc;
function FakeEl(tag, ns) {
    var self = this;
    this.tagName = tag; this.ns = ns || ''; this.children = []; this.attrs = {};
    this.dataset = {}; this.className = ''; this.textContent = ''; this.disabled = false; this.type = '';
    this.parentNode = null; this.hidden = false;
    var props = {};
    this.style = { setProperty: function (k, v) { props[k] = v; }, getPropertyValue: function (k) { return props[k] || ''; } };
    var cls = {};
    this.classList = {
        add: function (c) { cls[c] = true; }, remove: function (c) { delete cls[c]; },
        toggle: function (c, on) { if (on === undefined ? !cls[c] : on) cls[c] = true; else delete cls[c]; },
        contains: function (c) { return !!cls[c]; }
    };
    this.focus = function () { doc.activeElement = self; };
}
FakeEl.prototype.appendChild = function (c) { c.parentNode = this; this.children.push(c); return c; };
FakeEl.prototype.removeChild = function (c) {
    this.children.splice(this.children.indexOf(c), 1);
    c.parentNode = null;
    if (doc.activeElement === c || (doc.activeElement && c.contains(doc.activeElement))) doc.activeElement = doc.body;
    return c;
};
FakeEl.prototype.contains = function (el) { for (var e = el; e; e = e.parentNode) if (e === this) return true; return false; };
FakeEl.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
FakeEl.prototype.getAttribute = function (k) { return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null; };
FakeEl.prototype.addEventListener = function () {};
Object.defineProperty(FakeEl.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });
Object.defineProperty(FakeEl.prototype, 'childElementCount', { get: function () { return this.children.length; } });

var parent = new FakeEl('main');
var dash = new FakeEl('div'); dash.parentNode = parent;
var col = new FakeEl('div');
col.clientWidth = 346;   // 344 inside the slot's borders: six 48px columns
col.clientHeight = 600;
var other = new FakeEl('input');   // focus somewhere else on the page
doc = {
    body: new FakeEl('body'),
    activeElement: null,
    documentElement: new FakeEl('html'),
    getElementById: function (id) {
        if (id === 'dashboard') return dash;
        if (id === 'dash-col-center') return col;
        return null;
    },
    createElement: function (t) { return new FakeEl(t); },
    createElementNS: function (ns, t) { return new FakeEl(t, ns); },
    addEventListener: function () {},
    querySelectorAll: function () { return []; }
};
doc.activeElement = doc.body;

// Count real renders: the deck redraws only when a cell changes.
var renders = 0;
var TDwin = Object.create(TD);
TDwin.render = function (deck, cells, d) { renders++; return TD.render(deck, cells, d); };
var win = {
    TouchDeck: TDwin,
    matchMedia: function () { return { matches: true }; },          // a touch screen
    localStorage: { getItem: function () { return 'always'; }, setItem: function () {} },
    innerWidth: 360, innerHeight: 800
};
var sandbox = { window: win, document: doc, console: console };
vm.createContext(sandbox);
vm.runInContext(SRC, sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

var deck = new FakeEl('div');
var slot = new FakeEl('div');
slot.appendChild(deck);
D._slot = slot;
D._deck = deck;
var mapDeck = new FakeEl('div');
mapDeck.hidden = true;
D._mapDeck = mapDeck;

var pressed = [], typed = [];
D.setDeckSender(function (id) { pressed.push(id); }, function (line) { typed.push(line); });

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function room(num, v2, extra) {
    var r = { num: num, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}
function e(kind, id) { return { num: 1, dx: 0, dy: 0, dz: 0, details: [], kind: kind, id: id }; }
function at(kind, pred) {
    return D._deckMemo.cells.findIndex(function (c) { return c.kind === kind && (!pred || pred(c)); });
}
function focused() { return deck.contains(doc.activeElement) ? doc.activeElement.getAttribute('data-cell') : 'outside'; }
var tower = room(5243, { west: e('compass', 'm4'), shrine: e('named', 'xaaaa0001'), ledge: e('named', 'xaaaa0002') });
var WEST = function (c) { return c.dir === 'west'; };

// --- the real applyDeck ------------------------------------------------------

D.setMoves(tower);
check('the deck shows and draws one child per cell', [slot.dataset.shows, deck.children.length, renders], ['deck', 12, 1]);
check('a 48px deck is not small, a lit room not dark', [deck.classList.contains('deck-small'), deck.classList.contains('deck-dark')], [false, false]);
D.applyDeck();
check('nothing changed: no redraw', renders, 1);
col.clientWidth = 330;   // too narrow for six 48px buttons
D.applyDeck();
check('44px buttons: the deck takes deck-small', deck.classList.contains('deck-small'), true);
col.clientWidth = 346;
D.applyDeck();
check('48px again: deck-small goes', deck.classList.contains('deck-small'), false);

// --- taps and the level ------------------------------------------------------

D._deckTap(at('move', WEST));
D._deckTap(at('move', WEST));
check('a lit arrow sends its fixed id, and a quick second tap sends again (ruling 8)', pressed, ['m4', 'm4']);

// Focus survives a redraw (keyboard and screen reader users).
deck.children[at('move', WEST)].focus();
D.setDeckMode('combat');
check('a redraw keeps the focus on the same button', focused(), String(at('move', WEST)));
D.setDeckMode('');
doc.activeElement = other;
D.setDeckMode('combat');
check('a redraw takes no focus from outside the deck', doc.activeElement === other, true);
D.setDeckMode('');

deck.children[at('other')].focus();
D._deckTap(at('other'));
check('the exits button opens the list', D._deckState.level, 'exits');
check('opening the list focuses Back', focused(), String(at('back')));
D._deckState.guardUntil = 0;   // the list's hold (D11) is touchdeck-moves.test.js's; tap the list now
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
check('an exit sends its id once for a quick double tap', pressed.slice(2), ['xaaaa0001']);
D.setMoves(tower);
check('Room.Info again in the same room keeps the list open', D._deckState.level, 'exits');
D.setMoves(room(5346, { north: e('compass', 'm1'), gate: e('named', 'xbbbb0001') }));
check('after a move the deck returns to the T', D._deckState.level, 'top');
check('returning to the T focuses the exits button', focused(), String(at('other')));
D._deckTap(at('move', function (c) { return c.dir === 'north'; }));
check('the exit\'s hold survives the move: a second tap now walks nowhere (plan 2b D11)', pressed.slice(3), []);
D._deckState.guardUntil = 0;
D._deckTap(at('move', function (c) { return c.dir === 'north'; }));
check('once the hold ends the arrow walks', pressed.slice(3), ['m1']);

deck.children[at('move', function (c) { return c.dir === 'north'; })].focus();
D.setMoves(room(5347, { up: e('vertical', 'm5') }));
check('a button that is gone hands the focus to the first button', focused(), String(at('move', function (c) { return c.dir === 'up'; })));

D.setDeckMode('combat');
check('combat reaches the cells', D._deckMemo.cells[1].combat, true);
D.setMoves(room(6418, {}, { dark: true, darkmarks: { north: 'wall' } }));
check('a pitch-dark room: the deck takes deck-dark', deck.classList.contains('deck-dark'), true);
D._deckTap(at('dark'));
check('the Dark tile types look through the page, and presses nothing', [typed, pressed.length], [['look'], 4]);
D.setMoves(null);
check('a closed connection leaves only empty slots', D._deckMemo.cells.every(function (c) { return c.kind === 'empty'; }), true);
check('and the deck is light again', deck.classList.contains('deck-dark'), false);

// --- the Map tab's buttons (plan 2c) -------------------------------------------

D.mode = 'phone';   // the Map tab's buttons are a phone's (D9)
D.setDeckMode('');
D.setMoves(tower);
check('Map tab closed: its buttons stay hidden', mapDeck.hidden, true);
D._drawerActive = 'map';
D.applyDeck();
check('Map tab open on a phone: its buttons show, 3 by 3 at the deck size (D1, D11)',
    [mapDeck.hidden, mapDeck.children.length, mapDeck.style.getPropertyValue('--deck-cols'), mapDeck.style.getPropertyValue('--deck-btn')],
    [false, 9, '3', '48px']);
function atMap(kind, pred) {
    return D._mapMemo.cells.findIndex(function (c) { return c.kind === kind && (!pred || pred(c)); });
}
var sent = pressed.length;
D._deckTap(atMap('move', WEST), 'map');
check('a Map tab arrow sends the same fixed id as the deck', pressed.slice(sent), ['m4']);
D._deckTap(atMap('other'), 'map');
check('the Map tab exits button opens the list, shared with the deck (D2)', [D._deckState.level, D._mapMemo.cells[0].kind], ['exits', 'back']);
check('and the deck draws the same list, Back first', D._deckMemo.cells[0].kind, 'back');
D._deckState.guardUntil = 0;
D._deckTap(atMap('back'), 'map');
check('Back returns both to the T', [D._deckState.level, D._deckMemo.cells[0].kind, D._mapMemo.cells[0].kind, atMap('other') >= 0, at('other') >= 0],
    ['top', 'move', 'move', true, true]);

// Different pages per grid (review point 1): the Map tab holds fewer exits per
// page, so its page number means nothing to the deck.
var many = {};
for (var n = 0; n < 15; n++) many['x' + n] = e('named', 'xcccc00' + (10 + n));
D.setMoves(room(7001, many));
D._deckState.guardUntil = 0;
D._deckTap(atMap('other'), 'map');
D._deckState.guardUntil = 0;
D._deckTap(atMap('more'), 'map');
check('the Map tab is on page 1 of a long list', D._deckState.page, 1);
D._drawerActive = null;
D.applyDeck();
check('closing the tab: the deck shows page 0, not the map\'s page', D._deckState.page, 0);
check('and the level is kept (D2)', D._deckState.level, 'exits');
D._deckState.guardUntil = 0;
D._deckTap(at('back'));
D._drawerActive = 'map';
D.applyDeck();
D._deckState.guardUntil = 0;

// The keyboard (review point 2): the deck would not show, so neither does the Map grid.
doc.documentElement.classList.add('keyboard-open');
D.applyDeck();
check('keyboard up for another field: the Map tab buttons hide', mapDeck.hidden, true);
doc.documentElement.classList.remove('keyboard-open');
D.applyDeck();
check('keyboard down: they show again', mapDeck.hidden, false);
D.setMoves(tower);
D.setMoves(room(6418, {}, { dark: true }));
check('a pitch-dark room: the Map tab buttons go dark too', mapDeck.classList.contains('deck-dark'), true);
D.mode = 'column';
D.applyDeck();
check('outside phone mode the Map tab buttons hide (D9)', mapDeck.hidden, true);
D.mode = 'phone';
D._drawerActive = null;
D.applyDeck();
check('Map tab closed again: hidden', mapDeck.hidden, true);
D.setMoves(null);

// --- the sideways bars (#938, plan 2c) -------------------------------------------

// A phone on its side: viewportfit's isSidewaysPhone reads the screen.
win.screen = { width: 800, height: 360 };
win.ViewportFit = {
    isPhone: function () { return true; },
    isSidewaysPhone: function () { return true; },
    orientationType: function () { return 'landscape-primary'; },
    editableFocused: function () { return false; }
};
sandbox.screen = win.screen;
D._drawerActive = null;
D.setMoves(tower);
check('sideways with the deck on: the bars fold', dash.dataset.bars, 'folded');
var menuAt = D._deckMemo.cells.findIndex(function (c) { return c.kind === 'menu'; });
check('Menu is the side column\'s last slot', menuAt, 11);
D._deckState.guardUntil = 0;   // Back's hold from the Map section above
D._deckTap(menuAt);
check('Menu shows the bars', [dash.dataset.bars, D._deckMemo.cells[menuAt].open], ['shown', true]);
D._deckTap(menuAt);
check('Menu again folds them', [dash.dataset.bars, D._deckMemo.cells[menuAt].open], ['folded', false]);
D._showBars(true);
win.localStorage = { getItem: function () { return 'never'; }, setItem: function () {} };
D.applyDeck();
check('Touch controls on Never: no deck, no Menu, the bars as always (D4)', [dash.dataset.bars, D._barsShown], [undefined, false]);
win.localStorage = { getItem: function () { return 'always'; }, setItem: function () {} };
win.screen = { width: 360, height: 800 };
sandbox.screen = win.screen;
win.ViewportFit.isSidewaysPhone = function () { return false; };
D.applyDeck();
check('upright again: the bars as always', dash.dataset.bars, undefined);
delete win.ViewportFit;
win.screen = undefined;
D.setMoves(null);

// --- the tab bar draws its icons with touchdeck.js lineIcon -----------------
// One stroke-path icon builder for the client (unify rule): the phone tab
// bar's round tabs (makeTabBar) and the deck's icons share TouchDeck.lineIcon.

D._buildPhone();
var bar = D._phoneTabs.el;
var tabs = bar.children.filter(function (c) { return c.tagName === 'button'; });
// The tab icons as dashboard.js PANEL_TABS writes them.
var want = {};
SRC.replace(/^\s*(\w+):\s*\{ label: "[^"]+",\s*icon: \[([^\]]*)\] \}/mg, function (_, name, list) {
    want[name] = list.split(',').map(function (s) { return s.trim().replace(/^"|"$/g, ''); });
});
check('the tab bar has a tab per panel (PANEL_TABS read)', tabs.map(function (t) { return t.dataset.panel; }), Object.keys(want));
check('PANEL_TABS was read', Object.keys(want).length > 0, true);
check('every tab draws one line icon: a 24 box SVG, hidden from screen readers, its own paths',
    tabs.map(function (t) {
        var svg = t.children[0];
        return [t.children.length, svg.tagName, svg.ns, svg.attrs.viewBox, svg.attrs['aria-hidden'], svg.attrs.focusable,
            svg.attrs['class'] === undefined, svg.children.map(function (p) { return p.attrs.d; })];
    }),
    tabs.map(function (t) {
        return [1, 'svg', 'http://www.w3.org/2000/svg', '0 0 24 24', 'true', 'false', true, want[t.dataset.panel]];
    }));
check('dashboard.js keeps no stroke-path loop of its own', /createElementNS/.test(SRC), false);

process.exit(failures ? 1 : 0);
