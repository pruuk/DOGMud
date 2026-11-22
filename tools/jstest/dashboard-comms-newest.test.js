// dashboard-comms-newest.test.js: the Comms panel opens each channel at its
// newest message (#942, group playtest 2026-11-21).
//
// Run:  node tools/jstest/dashboard-comms-newest.test.js
//
// The bug: with the Comms panel small, a channel showed its first (oldest)
// message. Two paths did it: lines that arrived on a channel while another
// one showed landed in a hidden box that kept its scroll place at the top,
// and choosing that channel never scrolled; and a panel made smaller kept
// its scroll place, so the newest lines slid under the bottom edge.
//
// Loads the real static/js/dashboard.js and drives Dashboard._watchComms
// against a small fake Comms panel and a fake ResizeObserver. Dependency
// free, exits non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');
var vm = require('vm');

var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var SRC = fs.readFileSync(path.join(JS, 'dashboard.js'), 'utf8');

var failures = 0;
function check(name, got, want) {
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + got + '\n  want: ' + want);
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

// --- a fake Comms panel ------------------------------------------------------

function El(classes) {
    this.classes = classes.slice();
    var self = this;
    this.classList = {
        contains: function (c) { return self.classes.indexOf(c) >= 0; },
        add: function (c) { if (!this.contains(c)) self.classes.push(c); },
        remove: function (c) { self.classes = self.classes.filter(function (x) { return x !== c; }); }
    };
}
El.prototype.closest = function (sel) {
    return this.classList.contains(sel.replace('.', '')) ? this : null;
};

// A channel window: 100 lines of 20px in a box clientHeight high.
function Win(name, active) {
    El.call(this, ['chat-window', 'tab-content'].concat(active ? ['active'] : []));
    this.name = name;
    this.scrollHeight = 2000;
    this.clientHeight = 300;
    this.scrollTop = 0;
}
Win.prototype = Object.create(El.prototype);

var say = new Win('say', true);
var chat = new Win('chat', false);
var chatButton = new El(['tab-button', 'subtab']);

var listeners = { click: [], scroll: [] };
var out = {
    addEventListener: function (type, fn) { listeners[type].push(fn); },
    querySelectorAll: function () { return [say, chat]; },
    querySelector: function (sel) {
        if (sel !== '.chat-window.active') throw new Error('unexpected selector ' + sel);
        return [say, chat].filter(function (w) { return w.classList.contains('active'); })[0] || null;
    }
};

var observed = [];
var resizeCallback = null;
function FakeResizeObserver(cb) { resizeCallback = cb; }
FakeResizeObserver.prototype.observe = function (el) { observed.push(el); };

var win = { ResizeObserver: FakeResizeObserver };
var sandbox = {
    window: win,
    ResizeObserver: FakeResizeObserver,
    document: {
        getElementById: function (id) { return id === 'comm-output' ? out : null; },
        addEventListener: function () {},
        querySelectorAll: function () { return []; }
    },
    console: console
};
vm.createContext(sandbox);
vm.runInContext(SRC, sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

D._watchComms();

function fire(type, target) {
    listeners[type].forEach(function (fn) { fn({ target: target }); });
}
function atNewest(w) { return w.scrollTop === w.scrollHeight; }

check('every channel window is watched for size', observed.length, 2);

// Path 1: lines arrived on Chat while Say showed. Chat is hidden, so its
// scroll place stayed at the top. The page's own button handler swaps
// active first, then the click bubbles to #comm-output.
say.classList.remove('active');
chat.classList.add('active');
fire('click', chatButton);
check('choosing a channel shows its newest message', atNewest(chat), true);

// Path 2: the panel shrinks while the reader is at the newest line.
chat.scrollTop = chat.scrollHeight - chat.clientHeight;
fire('scroll', chat);
chat.clientHeight = 120;
resizeCallback();
check('a smaller panel keeps the newest message in view', atNewest(chat), true);

// A reader who scrolled up to read old lines is left there on a resize.
chat.scrollTop = 400;
fire('scroll', chat);
chat.clientHeight = 200;
resizeCallback();
check('a reader scrolled up stays where they are', chat.scrollTop, 400);

// A scroll in a hidden channel does not change the active one's state.
chat.scrollTop = chat.scrollHeight - chat.clientHeight;
fire('scroll', chat);
say.scrollTop = 0;
fire('scroll', say);
resizeCallback();
check('a hidden channel scroll is ignored', atNewest(chat), true);

// A click that is not on a channel button does nothing.
chat.scrollTop = 50;
fire('click', new El(['chat-hint']));
check('a click off the buttons does not scroll', chat.scrollTop, 50);

if (failures) {
    console.log(failures + ' failure(s)');
    process.exit(1);
}
console.log('all passed');
