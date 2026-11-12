// dashboard-popout-layout.test.js: a popped-out panel keeps its place in its
// column and its window keeps its size and place (#768 review item 14), on
// the layout ladder (plan 5, Task 7).
//
// Run:  node tools/jstest/dashboard-popout-layout.test.js
//
// The bug: pop out Map, narrow the window until pop-outs dock, widen again,
// close the window. Map landed under Inventory and stayed there after a
// reload, and every crossing reset the window to 360x300. Since the ladder
// (owner 2026-11-12) panels never move between columns, so a popped panel
// docks back before its anchor, its place in the page's order; the saved
// layout (v2) keeps only folded and popped panels and window places.
//
// Loads the real static/js/dashboard.js, with the real layoutpick.js and
// viewportfit.js, against a small fake DOM and a fake WinBox, and drives
// Dashboard's own setGameWidth, _apply, popout, redock, saveLayout and
// restoreLayout. Only the tab bar modes' enter and exit are stubbed: moving
// panels into the drawer is not part of the bug.
//
// Dependency-free and assertion-library-free, matching the other jstest
// files. Exits non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');
var vm = require('vm');

var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var SRC = fs.readFileSync(path.join(JS, 'dashboard.js'), 'utf8');

var failures = 0;
var checks = 0;
function check(name, got, want) {
    checks++;
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g !== w) {
        console.log('FAIL ' + name + '\n  got:  ' + g + '\n  want: ' + w);
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

// --- a small fake DOM --------------------------------------------------------
//
// Enough for the code paths under test: ids, classes, data-*, parent and
// children, and querySelector(All) for ".a", ".a.b", "#id .a" and lists.

function Style() {
    var props = {};
    this.getPropertyValue = function (k) { return props[k] || ''; };
    this.setProperty = function (k, v) { props[k] = String(v); };
    this.removeProperty = function (k) { delete props[k]; };
}

function Node(tag, attrs) {
    attrs = attrs || {};
    this.tagName = tag.toUpperCase();
    this.id = attrs.id || '';
    this.classes = (attrs.className || '').split(/\s+/).filter(Boolean);
    this.dataset = {};
    this.children = [];
    this.parentNode = null;
    this.style = new Style();
    this.textContent = attrs.text || '';
    this.scrollTop = 0;
    this.scrollHeight = 0;
    this.clientWidth = 0;
    var self = this;
    this.classList = {
        contains: function (c) { return self.classes.indexOf(c) >= 0; },
        add: function (c) { if (self.classes.indexOf(c) < 0) self.classes.push(c); },
        remove: function (c) { self.classes = self.classes.filter(function (x) { return x !== c; }); },
        toggle: function (c, on) {
            var want = on === undefined ? !this.contains(c) : !!on;
            if (want) this.add(c); else this.remove(c);
            return want;
        }
    };
}
Node.prototype.appendChild = function (n) { return this.insertBefore(n, null); };
Node.prototype.insertBefore = function (n, ref) {
    if (n.parentNode) n.parentNode.removeChild(n);
    var at = ref ? this.children.indexOf(ref) : -1;
    if (at < 0) this.children.push(n); else this.children.splice(at, 0, n);
    n.parentNode = this;
    return n;
};
Node.prototype.removeChild = function (n) {
    var at = this.children.indexOf(n);
    if (at >= 0) this.children.splice(at, 1);
    n.parentNode = null;
    return n;
};
Object.defineProperty(Node.prototype, 'nextElementSibling', {
    get: function () {
        if (!this.parentNode) return null;
        var sib = this.parentNode.children;
        return sib[sib.indexOf(this) + 1] || null;
    }
});
Node.prototype.removeAttribute = function (name) {
    if (name === 'style') this.style = new Style();
    if (name.indexOf('data-') === 0) {
        var key = name.slice(5).replace(/-([a-z])/g, function (m, c) { return c.toUpperCase(); });
        delete this.dataset[key];
    }
};
Node.prototype.setAttribute = function () {};
Node.prototype.addEventListener = function () {};
Node.prototype.getBoundingClientRect = function () { return { width: this.clientWidth }; };
Node.prototype.contains = function (n) {
    for (; n; n = n.parentNode) if (n === this) return true;
    return false;
};
function simpleMatch(n, sel) {
    var m = sel.match(/^(#[\w-]+)?((?:\.[\w-]+)*)$/);
    if (!m) throw new Error('fake DOM cannot match ' + sel);
    if (m[1] && n.id !== m[1].slice(1)) return false;
    var cls = m[2] ? m[2].split('.').filter(Boolean) : [];
    return cls.every(function (c) { return n.classList.contains(c); });
}
function descendants(root) {
    var out = [];
    (function walk(n) { n.children.forEach(function (c) { out.push(c); walk(c); }); })(root);
    return out;
}
Node.prototype.matches = function (sel) {
    var self = this;
    return sel.split(',').some(function (part) {
        var steps = part.trim().split(/\s+/);
        if (!simpleMatch(self, steps[steps.length - 1])) return false;
        var n = self.parentNode;
        for (var i = steps.length - 2; i >= 0; i--) {
            while (n && !simpleMatch(n, steps[i])) n = n.parentNode;
            if (!n) return false;
            n = n.parentNode;
        }
        return true;
    });
};
Node.prototype.querySelectorAll = function (sel) {
    return descendants(this).filter(function (n) { return n.matches(sel); });
};
Node.prototype.querySelector = function (sel) { return this.querySelectorAll(sel)[0] || null; };
Node.prototype.closest = function (sel) {
    for (var n = this; n; n = n.parentNode) if (n.matches && n.matches(sel)) return n;
    return null;
};

var htmlEl = new Node('html');
var docRoot = new Node('body');
var document = {
    documentElement: htmlEl,
    body: docRoot,
    getElementById: function (id) {
        return descendants(docRoot).filter(function (n) { return n.id === id; })[0] || null;
    },
    querySelectorAll: function (sel) { return docRoot.querySelectorAll(sel); },
    querySelector: function (sel) { return docRoot.querySelector(sel); },
    createElement: function (tag) { return new Node(tag); },
    addEventListener: function () {}
};

// The real page's columns and panels, in the page's own order: the game
// column first, then the panel area holding the left and right columns.
var COLS = {
    'dash-col-left': ['map', 'quests', 'vitals', 'art'],
    'dash-col-center': ['feed', 'cmd'],
    'dash-col-right': ['chat', 'status', 'trig']
};
var dashboard = new Node('div', { id: 'dashboard' });
dashboard.clientWidth = 1400;
docRoot.appendChild(dashboard);
var panelArea = new Node('div', { id: 'dash-panels' });
function makeCol(colId, parent) {
    var col = new Node('div', { id: colId, className: 'dash-col' });
    parent.appendChild(col);
    COLS[colId].forEach(function (name) {
        var p = new Node('section', { id: 'panel-' + name, className: 'dash-panel' });
        p.dataset.panel = name;
        p.dataset.homeCol = colId;
        p.appendChild(new Node('span', { className: 'ph-title', text: name }));
        col.appendChild(p);
    });
}
makeCol('dash-col-center', dashboard);
dashboard.appendChild(panelArea);
makeCol('dash-col-left', panelArea);
makeCol('dash-col-right', panelArea);
// Somewhere off the columns for a popped panel to live, as WinBox does.
var floating = new Node('div', { id: 'winbox-layer' });
docRoot.appendChild(floating);

function column(colId) {
    return document.getElementById(colId).children.map(function (p) { return p.dataset.panel; });
}

// --- fake WinBox -------------------------------------------------------------

var boxes = [];
function WinBox(opts) {
    this.opts = opts;
    this.x = opts.x === undefined ? 100 : opts.x;
    this.y = opts.y === undefined ? 100 : opts.y;
    this.width = opts.width;
    this.height = opts.height;
    this.onclose = opts.onclose;
    this.onmove = opts.onmove;
    this.onresize = opts.onresize;
    floating.appendChild(opts.mount);
    boxes.push(this);
}
WinBox.prototype.close = function () {
    if (this.onclose && this.onclose()) return true;
};
// The player drags and resizes the window: WinBox calls its callbacks with
// the window as this.
WinBox.prototype.playerMoves = function (x, y, w, h) {
    this.x = x; this.y = y; if (this.onmove) this.onmove.call(this, x, y);
    this.width = w; this.height = h; if (this.onresize) this.onresize.call(this, w, h);
};

// --- load layoutpick.js, viewportfit.js and dashboard.js ---------------------

var store = {};
var localStorage = {
    getItem: function (k) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem: function (k, v) { store[k] = String(v); },
    removeItem: function (k) { delete store[k]; }
};
var timers = [];
var win = {
    innerWidth: 1400,
    innerHeight: 900,
    addEventListener: function () {}
};
var coarse = false;
var sandbox = {
    window: win,
    document: document,
    localStorage: localStorage,
    WinBox: WinBox,
    // A desktop monitor with a mouse, until a check says otherwise.
    screen: { width: 1920, height: 1080 },
    matchMedia: function (q) { return { matches: q === '(pointer: coarse)' && coarse }; },
    addEventListener: function () {},
    setTimeout: function (fn) { timers.push(fn); return timers.length; },
    clearTimeout: function () {},
    requestAnimationFrame: function () {},
    location: { reload: function () {} },
    console: console
};
vm.createContext(sandbox);
['layoutpick.js', 'viewportfit.js'].forEach(function (f) {
    vm.runInContext(fs.readFileSync(path.join(JS, f), 'utf8'), sandbox, { filename: f });
});
vm.runInContext(SRC, sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

// Stand-ins for the tab bar modes; the panel moves under test live in
// _apply, popout, redock, _enterWide, saveLayout and restoreLayout.
var entered = [];
['Column', 'Slim', 'Phone'].forEach(function (m) {
    D['_enter' + m] = function () { entered.push(m.toLowerCase()); };
    D['_exit' + m] = function () {};
});

// termfit reports 780px for 80 columns: the game column is 786.
D.setGameWidth(780);
check('the game column is the 80-column width plus border and slack',
    htmlEl.style.getPropertyValue('--game-w'), '786px');
check('a 1400px dashboard is the full layout', D.mode, 'wide');
D.initPopout();

// Widths the ladder maps to each mode, with the 786px game column:
// room = width - 12 - 786 - 6.
var WIDE = 1400, COLUMN = 1100, SLIM = 1000, PHONE = 900;
function resize(width) {
    dashboard.clientWidth = width;
    win.innerWidth = width;
    D._apply();
}
var KEY = 'dogmud.dashboard.layout.v2';
function saved() { return JSON.parse(store[KEY] || '{}'); }
function anchorOf(name) {
    var a = D._dockAnchor[name];
    return a && a.dataset ? a.dataset.panel : null;
}

// --- the ladder --------------------------------------------------------------

resize(COLUMN);
check('296px of room is the one-column step', D.mode, 'column');
check('the root is marked for the column CSS', htmlEl.classList.contains('layout-column'), true);
resize(SLIM);
check('196px of room is the slim column', D.mode, 'slim');
check('the root is marked for the slim CSS', [htmlEl.classList.contains('layout-slim'), htmlEl.classList.contains('layout-column')], [true, false]);
resize(PHONE);
check('96px of room is the phone layout', D.mode, 'phone');
check('the phone layout has no column or slim mark',
    [htmlEl.classList.contains('layout-slim'), htmlEl.classList.contains('layout-column')], [false, false]);
check('each tab bar mode was entered', entered, ['column', 'slim', 'phone']);
sandbox.screen = { width: 390, height: 844 };
coarse = true;
resize(WIDE);
check('a phone gets the phone layout however wide it reports', D.mode, 'phone');
coarse = false;
resize(WIDE);
check('a small screen without touch (a zoomed desktop) stays on the ladder', D.mode, 'wide');
sandbox.screen = { width: 1920, height: 1080 };

// --- before termfit has measured ---------------------------------------------

// No reported width yet (the page's first fit returned early): a desktop
// still starts in Layout D, at the default game column, and only a phone
// gets the phone layout.
dashboard.clientWidth = 1920;
D.setGameWidth(null);
check('a 1920 desktop with no reported width starts in the full layout', D.mode, 'wide');
check('with no reported width the game column is the default',
    htmlEl.style.getPropertyValue('--game-w'), '786px');
sandbox.screen = { width: 390, height: 844 };
coarse = true;
D.setGameWidth(null);
check('a touch phone with no reported width is the phone layout', D.mode, 'phone');
check('the game column stays set in the phone layout too',
    htmlEl.style.getPropertyValue('--game-w'), '786px');
sandbox.screen = { width: 1920, height: 1080 };
coarse = false;
dashboard.clientWidth = WIDE;
D.setGameWidth(800);
check('a reported width replaces the default', [D.mode, htmlEl.style.getPropertyValue('--game-w')], ['wide', '806px']);
D.setGameWidth(780);

// --- the round trip ----------------------------------------------------------

var LEFT = ['map', 'quests', 'vitals', 'art'];
check('starts in the page order', column('dash-col-left'), LEFT);

D.popout('map');
check('popping out Map takes it out of its column', column('dash-col-left'), ['quests', 'vitals', 'art']);
check('Map will dock back before Quests', anchorOf('map'), 'quests');
check('the saved layout keeps Map popped and no arrangement',
    [saved().popped, saved().arrange, saved().cols], [['map'], undefined, undefined]);
check('a first pop-out opens at the default size', [boxes[0].width, boxes[0].height], [360, 300]);

boxes[0].playerMoves(500, 120, 480, 420);
timers.splice(0).forEach(function (fn) { fn(); });
check('moving and resizing the window is saved',
    (saved().boxes || {}).map, { x: 500, y: 120, width: 480, height: 420 });

resize(COLUMN);
check('narrowing to the one-column step docks Map back in its place', column('dash-col-left'), LEFT);
check('a docked-for-narrow Map is still saved as popped', saved().popped, ['map']);
resize(WIDE);
check('widening again pops Map out again', boxes.length, 2);
check('the window comes back at its size and place',
    [boxes[1].x, boxes[1].y, boxes[1].width, boxes[1].height], [500, 120, 480, 420]);
check('Map will still dock back before Quests', anchorOf('map'), 'quests');

resize(PHONE);
check('narrowing to the phone layout docks Map back in its place', column('dash-col-left'), LEFT);
resize(WIDE);
check('widening from the phone layout pops Map out again', boxes.length, 3);

boxes[2].close();
check('closing the window docks Map in its place, not under Inventory',
    column('dash-col-left'), LEFT);
check('the saved layout after the close has nothing popped', saved().popped, []);

// --- a reload while popped ---------------------------------------------------

D.popout('map');
boxes[3].playerMoves(40, 60, 300, 260);
timers.splice(0).forEach(function (fn) { fn(); });
var beforeReload = store[KEY];

// A fresh page: panels in the page's own order, then the saved layout.
D.redock('map');
store[KEY] = beforeReload;
D._popped = {};
D._dockAnchor = {};
D._boxGeom = null;
D.restoreLayout();
check('after a reload Map pops out again', !!D._popped.map, true);
check('after a reload the window keeps its size and place',
    [boxes[4].x, boxes[4].y, boxes[4].width, boxes[4].height], [40, 60, 300, 260]);
boxes[4].close();
check('after a reload, closing docks Map in its place', column('dash-col-left'), LEFT);

// --- two popped panels, one anchored on the other ----------------------------

// Map pops first, so its anchor is Quests, which then pops too.
D.popout('map');
var mapBox = boxes[boxes.length - 1];
D.popout('quests');
var questsBox = boxes[boxes.length - 1];
check('Map and Quests both popped: Map anchored on Quests, Quests on Health',
    [anchorOf('map'), anchorOf('quests')], ['quests', 'vitals']);
mapBox.close();
check('Map closed while Quests is still out docks at the top, not the bottom',
    column('dash-col-left'), ['map', 'vitals', 'art']);
questsBox.close();
check('both docked back in place', column('dash-col-left'), LEFT);

// --- the Game window, in the centre column -----------------------------------

D.popout('feed');
check('the Game window will dock above the command box', anchorOf('feed'), 'cmd');
resize(PHONE);
check('the phone layout docks the Game window above the command box', column('dash-col-center'), ['feed', 'cmd']);
resize(WIDE);
boxes[boxes.length - 1].close();
check('the Game window docks above the command box', column('dash-col-center'), ['feed', 'cmd']);

// --- a remembered window that no longer fits stays on screen -----------------

D._boxGeom = { art: { x: 1300, y: 800, width: 600, height: 400 } };
win.innerWidth = 1200;
win.innerHeight = 700;
D.popout('art');
var b = boxes[boxes.length - 1];
check('a window too far right or down is pulled back on screen',
    [b.x, b.y, b.width, b.height], [600, 300, 600, 400]);
b.close();

// --- reset forgets window sizes, in whatever mode it is pressed --------------

D._toast = function () {};
D.resetLayout();
check('Reset layout forgets window sizes', D._boxGeom, null);
check('Reset layout forgets the saved layout', store[KEY], undefined);
check('Reset layout in Layout D keeps every panel in its page order',
    [column('dash-col-left'), column('dash-col-right')], [LEFT, ['chat', 'status', 'trig']]);
resize(SLIM);
entered = [];
D.resetLayout();
check('Reset layout in the slim column re-enters the slim column', [D.mode, entered], ['slim', ['slim']]);
resize(WIDE);

// --- an old v1 saved layout carries over once --------------------------------

delete store[KEY];
store['dogmud.dashboard.layout.v1'] = JSON.stringify({
    cols: { l: '300px', r: '280px' },
    arrange: { 'dash-col-left': ['art', 'map', 'quests', 'vitals'] },
    collapsed: ['quests'],
    popped: ['map'],
    boxes: { map: { x: 10, y: 20, width: 330, height: 310 } }
});
var carried = D._readLayout();
check('v1 folded, popped and window places carry over',
    carried, { collapsed: ['quests'], popped: ['map'], boxes: { map: { x: 10, y: 20, width: 330, height: 310 } } });
check('the carried layout is saved as v2, without the arrangement or widths',
    JSON.parse(store[KEY]), { collapsed: ['quests'], popped: ['map'], boxes: { map: { x: 10, y: 20, width: 330, height: 310 } } });
check('v1 is removed after the carry-over', store['dogmud.dashboard.layout.v1'], undefined);
store['dogmud.dashboard.layout.v1'] = JSON.stringify({ collapsed: ['chat'] });
check('an existing v2 wins over a stray v1', D._readLayout().collapsed, ['quests']);
delete store['dogmud.dashboard.layout.v1'];
delete store[KEY];
store['dogmud.dashboard.layout.v1'] = '{not json';
check('a broken v1 is dropped, not carried', [D._readLayout(), store['dogmud.dashboard.layout.v1']], [null, undefined]);
// A full or blocked storage: the v2 write fails, so v1 stays for next time.
var V1 = JSON.stringify({ collapsed: ['chat'], popped: [] });
store['dogmud.dashboard.layout.v1'] = V1;
var realSetItem = localStorage.setItem;
localStorage.setItem = function () { throw new Error('QuotaExceededError'); };
D._readLayout();
localStorage.setItem = realSetItem;
check('when the v2 write fails, v1 survives and no v2 is written',
    [store['dogmud.dashboard.layout.v1'], store[KEY]], [V1, undefined]);
check('the next load carries it over', D._readLayout().collapsed, ['chat']);
check('and then v1 is gone', store['dogmud.dashboard.layout.v1'], undefined);

// --- Chat unfolds at the newest message --------------------------------------
//
// The fold button's handler is wired in initCollapse; read its effect
// through the shared helper and the handler's source.

var chat = document.getElementById('panel-chat');
var chatWin = new Node('div', { className: 'chat-window active' });
chatWin.scrollHeight = 900;
chat.appendChild(chatWin);
D._scrollChatToNewest(chat);
check('the Chat helper scrolls to the newest message', chatWin.scrollTop, 900);
check('unfolding a panel scrolls its Chat to the newest message',
    /panel\.classList\.toggle\("collapsed"\);\s*\/\/[^\n]*\n\s*if \(!panel\.classList\.contains\("collapsed"\)\) self\._scrollChatToNewest\(panel\);/.test(SRC), true);
check('the fold handler lives in initCollapse, which init calls',
    /initCollapse: function \(\) \{[\s\S]*?panel\.classList\.toggle\("collapsed"\)/.test(SRC) &&
    /this\.initCollapse\s*&& this\.initCollapse\(\);/.test(SRC), true);
check('opening the phone card uses the same helper',
    /this\._scrollChatToNewest\(panel\);\s*this\._updateTabbarActive\(name\);/.test(SRC), true);

// --- result ------------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
