// webclient-connect.test.js: the web client holds one socket at a time.
//
// Run:  node tools/jstest/webclient-connect.test.js
//
// Runs the page's own connect handler and session state code against a fake
// WebSocket. The page connects by itself on load, so a player can press Enter
// Game while that first socket is still connecting.
'use strict';

var fs = require('fs');
var path = require('path');
var html = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');

function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}

function element() {
    var el = {
        style: {}, disabled: false, textContent: '', type: 'text', value: '',
        handlers: {}, children: {}, classList: { toggle: function () {} },
        // The session button holds its status circle, label and hidden status
        // words as child spans (PR #768); hand back one fake per selector.
        querySelector: function (sel) { return el.children[sel] || (el.children[sel] = element()); },
        setAttribute: function () {},
        addEventListener: function (ev, fn) { el.handlers[ev] = fn; },
        click: function () { if (!el.disabled && el.handlers.click) el.handlers.click(); },
        focus: function () {}
    };
    return el;
}

var sockets = [];
function FakeSocket() { this.readyState = FakeSocket.CONNECTING; sockets.push(this); }
FakeSocket.CONNECTING = 0; FakeSocket.OPEN = 1; FakeSocket.CLOSING = 2; FakeSocket.CLOSED = 3;
FakeSocket.prototype.send = function () {};
FakeSocket.prototype.close = function () { this.readyState = FakeSocket.CLOSING; };
FakeSocket.prototype.open = function () { this.readyState = FakeSocket.OPEN; this.onopen(); };
FakeSocket.prototype.closed = function () {
    this.readyState = FakeSocket.CLOSED;
    this.onclose({ wasClean: true, code: 1000, reason: '' });
};

var els = {};
global.document = { getElementById: function (id) { return els[id] || (els[id] = element()); } };
global.location = { protocol: 'http:', host: 'test' };
global.WebSocket = FakeSocket;
global.term = { writeln: function () {}, write: function () {}, clear: function () {} };
global.debugLog = function () {};
global.stopAutomation = function () {};
global.companionGlue = null;
global.textInput = element();
global.setTimeout = function () { return 0; };
global.clearTimeout = function () {};

var code = 'var socket = null, leavingGame = false, leaveFallback = null, inGame = false;\n'
    + 'var pendingReconnectToken = null, reconnectAttempts = 0;\n'
    + 'var connectButton = document.getElementById("connect-button");\n'
    + 'var sessionButton = document.getElementById("btn-session");\n'
    + 'var sessionConfirm = document.getElementById("sess-confirm");\n'
    + extract('var SESSION_STATES = {', "sessionButton.addEventListener('click'")
    + extract("connectButton.addEventListener('click'", "textInput.addEventListener('keydown'");
(0, eval)(code);

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

var connect = els['connect-button'], session = els['btn-session'];
// The status words ride inside the session button (PR #768): its hidden
// .sess-status span reads ", connecting", ", connected", ", not connected".
var label = session.querySelector('.sess-status');

// The page connects on load.
connect.click();
check('the first click opens a socket', sockets.length === 1, 'sockets ' + sockets.length);
check('the bar says it is connecting', label.textContent === ', connecting', 'label ' + label.textContent);
check('Enter Game is disabled while connecting', session.disabled === true, 'disabled ' + session.disabled);

// A programmatic click still reaches the handler; it must not open another.
connect.click();
check('a click while connecting opens no second socket', sockets.length === 1, 'sockets ' + sockets.length);

sockets[0].open();
check('the bar says connected once open', label.textContent === ', connected', 'label ' + label.textContent);
check('Enter Game comes back once open', session.disabled === false, 'disabled ' + session.disabled);

// The socket fails to connect, or closes: the player can try again.
connect.disabled = false;
sockets[0].close();
connect.click();
check('a click while closing opens no second socket', sockets.length === 1, 'sockets ' + sockets.length);
sockets[0].closed();
check('the bar says not connected after close', label.textContent === ', not connected', 'label ' + label.textContent);
check('Enter Game is enabled after close', session.disabled === false, 'disabled ' + session.disabled);
connect.click();
check('a click after close opens a new socket', sockets.length === 2, 'sockets ' + sockets.length);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
