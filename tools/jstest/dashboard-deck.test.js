// dashboard-deck.test.js: dashboard.js keeps the deck's movement state and
// sends a tap's id (touch arc plan 2b, #781).
//
// Run:  node tools/jstest/dashboard-deck.test.js
//
// Loads the real static/js/dashboard.js and touchdeck.js. applyDeck is
// replaced by one that lays the cells out at the S21's shape, since drawing
// is touchdeck.js render's job (touchdeck-moves.test.js). Dependency free,
// exits non-zero on failure.
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

var win = { TouchDeck: TD };
var sandbox = {
    window: win,
    document: {
        getElementById: function () { return null; },
        addEventListener: function () {},
        querySelectorAll: function () { return []; }
    },
    console: console
};
vm.createContext(sandbox);
vm.runInContext(SRC, sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

var SHAPE = TD.shape(344, 600, true, false);
D.applyDeck = function () {
    D._deckCells = TD.cells(TD.moves(D._moves, D._deckMode), SHAPE, D._deckState.level, D._deckState.page);
};
var pressed = [], said = [];
D.setDeckSender(function (id) { pressed.push(id); }, function (t) { said.push(t); });

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function room(num, v2, extra) {
    var r = { num: num, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}
function e(kind, id) { return { num: 1, dx: 0, dy: 0, dz: 0, details: [], kind: kind, id: id }; }
function at(kind, pred) {
    return D._deckCells.findIndex(function (c) { return c.kind === kind && (!pred || pred(c)); });
}
var tower = room(5243, { west: e('compass', 'm4'), shrine: e('named', 'xaaaa0001'), ledge: e('named', 'xaaaa0002') });

D.setMoves(tower);
D._deckTap(at('move', function (c) { return c.dir === 'west'; }));
D._deckTap(at('move', function (c) { return c.dir === 'west'; }));
check('a lit arrow sends its fixed id, and a quick second tap sends again (ruling 8)', pressed, ['m4', 'm4']);
D._deckTap(at('other'));
check('the exits button opens the list', D._deckState.level, 'exits');
D._deckState.guardUntil = 0;   // the list's hold (D11) is touchdeck-moves.test.js's; tap the list now
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
check('an exit sends its id once for a quick double tap', pressed.slice(2), ['xaaaa0001']);
D.setMoves(tower);
check('Room.Info again in the same room keeps the list open', D._deckState.level, 'exits');
check('the tap hold survives a Room.Info (plan 2b D11)', D._deckState.guardUntil > 0, true);
D.setMoves(room(5346, { north: e('compass', 'm1') }));
check('after a move the deck returns to the T', D._deckState.level, 'top');
check('the hold survives the move too, so the second tap cannot walk again', D._deckState.guardUntil > 0, true);
D._deckState.guardUntil = 0;
D.setDeckMode('combat');
check('combat reaches the cells', D._deckCells[1].combat, true);
D.setMoves(room(6418, {}, { dark: true, darkmarks: { north: 'wall' } }));
D._deckTap(at('dark'));
check('the Dark tile writes its line through the page', said, [TD.DARK_LINE]);
D.setMoves(null);
check('a closed connection leaves only empty slots', D._deckCells.every(function (c) { return c.kind === 'empty'; }), true);

// --- the tab bar draws its icons with touchdeck.js lineIcon -----------------
// One stroke-path icon builder for the client (unify rule): the phone tab
// bar's round tabs (makeTabBar) and the deck's icons share TouchDeck.lineIcon.

function FakeEl(tag, ns) {
    this.tagName = tag; this.ns = ns || ''; this.children = []; this.attrs = {};
    this.dataset = {}; this.style = {}; this.className = ''; this.textContent = '';
    var cls = {};
    this.classList = {
        add: function (c) { cls[c] = true; }, remove: function (c) { delete cls[c]; },
        toggle: function (c, on) { if (on === undefined ? !cls[c] : on) cls[c] = true; else delete cls[c]; },
        contains: function (c) { return !!cls[c]; }
    };
}
FakeEl.prototype.appendChild = function (c) { this.children.push(c); return c; };
FakeEl.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
FakeEl.prototype.addEventListener = function () {};
var parent = new FakeEl('body');
sandbox.document = {
    getElementById: function (id) { return id === 'dashboard' ? { parentNode: parent } : null; },
    createElement: function (t) { return new FakeEl(t); },
    createElementNS: function (ns, t) { return new FakeEl(t, ns); },
    addEventListener: function () {},
    querySelectorAll: function () { return []; }
};
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
