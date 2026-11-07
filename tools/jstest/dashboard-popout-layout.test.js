// dashboard-popout-layout.test.js: a popped-out panel keeps its place in its
// column and its window keeps its size and place (#768 review item 14).
//
// Run:  node tools/jstest/dashboard-popout-layout.test.js
//
// The bug: pop out Map, shrink the window below 1100 wide (pop-outs dock),
// widen again, close the window. Map landed under Inventory and stayed there
// after a reload, and every crossing reset the window to 360x300. The saved
// arrangement left the popped panel out, so the way back to wide appended it
// at the end of its column, and every pop-out started at a fixed size.
//
// Loads the real static/js/dashboard.js against a small fake DOM and a fake
// WinBox, and drives Dashboard's own _apply, popout, redock, saveLayout and
// restoreLayout. Only the rail and phone chrome is stubbed: moving panels
// into the rail is not part of the bug.
//
// Dependency-free and assertion-library-free, matching the other jstest
// files. Exits non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');
var vm = require('vm');

var SRC = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'dashboard.js'
), 'utf8');

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

function Node(tag, attrs) {
    attrs = attrs || {};
    this.tagName = tag.toUpperCase();
    this.id = attrs.id || '';
    this.classes = (attrs.className || '').split(/\s+/).filter(Boolean);
    this.dataset = {};
    this.children = [];
    this.parentNode = null;
    this.style = { getPropertyValue: function () { return ''; }, setProperty: function () {} };
    this.textContent = attrs.text || '';
    this.scrollTop = 0;
    this.scrollHeight = 0;
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
    if (name === 'style') this.style = { getPropertyValue: function () { return ''; }, setProperty: function () {} };
    if (name.indexOf('data-') === 0) {
        var key = name.slice(5).replace(/-([a-z])/g, function (m, c) { return c.toUpperCase(); });
        delete this.dataset[key];
    }
};
Node.prototype.setAttribute = function () {};
Node.prototype.addEventListener = function () {};
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

var docRoot = new Node('body');
var document = {
    body: docRoot,
    getElementById: function (id) {
        return descendants(docRoot).filter(function (n) { return n.id === id; })[0] || null;
    },
    querySelectorAll: function (sel) { return docRoot.querySelectorAll(sel); },
    querySelector: function (sel) { return docRoot.querySelector(sel); },
    createElement: function (tag) { return new Node(tag); },
    addEventListener: function () {}
};

// The real page's columns and panels, in the page's own order.
var COLS = {
    'dash-col-left': ['map', 'quests', 'vitals', 'art'],
    'dash-col-center': ['feed', 'cmd'],
    'dash-col-right': ['chat', 'status', 'trig']
};
var dashboard = new Node('div', { id: 'dashboard' });
dashboard.clientWidth = 1400;
docRoot.appendChild(dashboard);
Object.keys(COLS).forEach(function (colId) {
    var col = new Node('div', { id: colId, className: 'dash-col' });
    dashboard.appendChild(col);
    COLS[colId].forEach(function (name) {
        var p = new Node('section', { id: 'panel-' + name, className: 'dash-panel' });
        p.dataset.panel = name;
        p.dataset.homeCol = colId;
        p.appendChild(new Node('span', { className: 'ph-title', text: name }));
        col.appendChild(p);
    });
});
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

// --- load dashboard.js -------------------------------------------------------

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
var sandbox = {
    window: win,
    document: document,
    localStorage: localStorage,
    WinBox: WinBox,
    setTimeout: function (fn) { timers.push(fn); return timers.length; },
    clearTimeout: function () {},
    requestAnimationFrame: function () {},
    location: { reload: function () {} },
    console: console
};
vm.createContext(sandbox);
vm.runInContext(SRC, sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

// Stand-ins for the rail and phone chrome; the panel moves under test live in
// _apply, popout, redock, _enterWide, saveLayout and restoreLayout.
D._enterRail = D._exitRail = D._enterPhone = D._exitPhone = function () {};
D.mode = 'wide';
D.initPopout();

function resize(width) {
    dashboard.clientWidth = width;
    win.innerWidth = width;
    D._apply();
}
function saved() { return JSON.parse(store['dogmud.dashboard.layout.v1'] || '{}'); }

// --- the round trip ----------------------------------------------------------

var LEFT = ['map', 'quests', 'vitals', 'art'];
check('starts in the page order', column('dash-col-left'), LEFT);

D.popout('map');
check('popping out Map takes it out of its column', column('dash-col-left'), ['quests', 'vitals', 'art']);
check('the saved arrangement keeps Map in its place while popped',
    saved().arrange['dash-col-left'], LEFT);
check('a first pop-out opens at the default size', [boxes[0].width, boxes[0].height], [360, 300]);

boxes[0].playerMoves(500, 120, 480, 420);
timers.splice(0).forEach(function (fn) { fn(); });
check('moving and resizing the window is saved',
    (saved().boxes || {}).map, { x: 500, y: 120, width: 480, height: 420 });

resize(900);
check('narrowing docks Map back in its place', column('dash-col-left'), LEFT);
resize(1400);
check('widening again pops Map out again', boxes.length, 2);
check('the window comes back at its size and place',
    [boxes[1].x, boxes[1].y, boxes[1].width, boxes[1].height], [500, 120, 480, 420]);
check('Map is still saved in its place', saved().arrange['dash-col-left'], LEFT);

boxes[1].close();
check('closing the window docks Map in its place, not under Inventory',
    column('dash-col-left'), LEFT);
check('the saved arrangement after the close keeps Map first', saved().arrange['dash-col-left'], LEFT);

// --- a reload while popped ---------------------------------------------------

D.popout('map');
boxes[2].playerMoves(40, 60, 300, 260);
timers.splice(0).forEach(function (fn) { fn(); });
var beforeReload = store['dogmud.dashboard.layout.v1'];

// A fresh page: panels in the page's own order, then the saved layout.
D.redock('map');
store['dogmud.dashboard.layout.v1'] = beforeReload;
D._popped = {};
D._dockAnchor = {};
D._boxGeom = null;
D._savedArrange = null;
D.restoreLayout();
check('after a reload Map pops out again', !!D._popped.map, true);
check('after a reload the window keeps its size and place',
    [boxes[3].x, boxes[3].y, boxes[3].width, boxes[3].height], [40, 60, 300, 260]);
boxes[3].close();
check('after a reload, closing docks Map in its place', column('dash-col-left'), LEFT);

// --- two popped panels, one anchored on the other ----------------------------

// Map pops first, so its anchor is Quests, which then pops too.
D.popout('map');
var mapBox = boxes[boxes.length - 1];
D.popout('quests');
var questsBox = boxes[boxes.length - 1];
check('Map and Quests both popped: both saved in place',
    saved().arrange['dash-col-left'], LEFT);
mapBox.close();
check('Map closed while Quests is still out docks at the top, not the bottom',
    column('dash-col-left'), ['map', 'vitals', 'art']);
questsBox.close();
check('both docked back in place', column('dash-col-left'), LEFT);

// --- the Game window, in the centre column -----------------------------------

D.popout('feed');
check('the Game window is saved above the command box while popped',
    saved().arrange['dash-col-center'], ['feed', 'cmd']);
resize(900);
resize(1400);
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

// --- reset forgets window sizes ----------------------------------------------

D._toast = function () {};
D.resetLayout();
check('Reset layout forgets window sizes', D._boxGeom, null);

// --- Chat unfolds at the newest message --------------------------------------
//
// The fold button's handler is wired in initRearrange; read its effect
// through the shared helper and the handler's source.

var chat = document.getElementById('panel-chat');
var chatWin = new Node('div', { className: 'chat-window active' });
chatWin.scrollHeight = 900;
chat.appendChild(chatWin);
D._scrollChatToNewest(chat);
check('the Chat helper scrolls to the newest message', chatWin.scrollTop, 900);
check('unfolding a panel scrolls its Chat to the newest message',
    /panel\.classList\.toggle\("collapsed"\);\s*\/\/[^\n]*\n\s*if \(!panel\.classList\.contains\("collapsed"\)\) self\._scrollChatToNewest\(panel\);/.test(SRC), true);
check('opening the phone card uses the same helper',
    /this\._scrollChatToNewest\(panel\);\s*this\._updateTabbarActive\(name\);/.test(SRC), true);

// --- result ------------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
