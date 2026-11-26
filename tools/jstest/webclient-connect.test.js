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

// The Enter Game button's child spans, read from its real markup, so a
// selector the page asks for that the button does not have finds nothing.
var sessionMarkup = /<button[^>]*id="btn-session"[^>]*>([\s\S]*?)<\/button>/.exec(html);
if (!sessionMarkup) throw new Error('btn-session markup not found');
var SESSION_SPANS = [];
sessionMarkup[1].replace(/<span class="([^"]+)"/g, function (m, cls) {
    SESSION_SPANS.push('.' + cls.split(/\s+/)[0]);
});

function element(childSelectors) {
    var el = {
        style: {}, disabled: false, textContent: '', type: 'text', value: '',
        handlers: {}, children: {}, classList: { toggle: function () {} },
        // Only the child spans this element really has; anything else is
        // null, as in a browser, so a renamed span fails the test.
        querySelector: function (sel) {
            if ((childSelectors || []).indexOf(sel) < 0) return null;
            return el.children[sel] || (el.children[sel] = element());
        },
        setAttribute: function () {},
        addEventListener: function (ev, fn) { el.handlers[ev] = fn; },
        click: function () { if (!el.disabled && el.handlers.click) el.handlers.click(); },
        focus: function () {}
    };
    return el;
}

var sockets = [];
function FakeSocket(url) { this.url = url; this.readyState = FakeSocket.CONNECTING; sockets.push(this); }
FakeSocket.CONNECTING = 0; FakeSocket.OPEN = 1; FakeSocket.CLOSING = 2; FakeSocket.CLOSED = 3;
FakeSocket.prototype.send = function () {};
FakeSocket.prototype.close = function () { this.readyState = FakeSocket.CLOSING; };
FakeSocket.prototype.open = function () { this.readyState = FakeSocket.OPEN; this.onopen(); };
FakeSocket.prototype.closed = function () {
    this.readyState = FakeSocket.CLOSED;
    this.onclose({ wasClean: true, code: 1000, reason: '' });
};

var els = {};
global.document = { getElementById: function (id) {
    return els[id] || (els[id] = element(id === 'btn-session' ? SESSION_SPANS : []));
} };
global.location = { protocol: 'http:', host: 'test' };
global.WebSocket = FakeSocket;
global.term = { cols: 57, writeln: function () {}, write: function () {}, clear: function () {} };
// The page's width reporter (static/js/displayreport.js): onopen reports the
// game window's columns, because the server starts every connection at 80.
var reported = [];
global.displayReport = { connected: function (cols) { reported.push(cols); }, columns: function () {} };
global.debugLog = function () {};
global.stopAutomation = function () {};
// onclose clears the full command queue (webclient-input-queue.test.js).
global.setInputQueueFull = function () {};
global.companionGlue = null;
var musicStops = 0;
global.MusicPlayer = { stopAll: function () { musicStops++; } };
global.textInput = element();
// The page's phone-keyboard wait (plan 6) sits just above the keydown marker
// and reads window.HotInput; with none, the page makes no wait. gameLine
// wraps with the real termfit.js.
global.window = { TermFit: require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'termfit.js')) };
global.setTimeout = function () { return 0; };
global.clearTimeout = function () {};
global.__srWrites = [];

var code = 'var socket = null, leavingGame = false, leaveFallback = null, inGame = false;\n'
    + 'var pendingReconnectToken = null, reconnectAttempts = 0;\n'
    + 'var connectButton = document.getElementById("connect-button");\n'
    + 'var sessionButton = document.getElementById("btn-session");\n'
    + 'var sessionConfirm = document.getElementById("sess-confirm");\n'
    // The screen reader switch (static/js/screenreader.js): the connect
    // handler reads isOn() for the address, onopen calls loggedOut(), and
    // onmessage and gameLine pass text to write(), recorded here.
    + 'var screenReader = { isOn: function () { return global.__srOn === true; }, loggedOut: function () {}, write: function (t) { global.__srWrites.push(t); } };\n'
    // onopen forgets the last account's Char.Info.
    + 'var GMCPStructs = {};\n'
    // onopen forgets the room the map last centred on (#781).
    + 'var allRooms = { lastRoomId: 5 }; global.__allRooms = allRooms;\n'
    // From gameLine, which sits just above markInGame, so the page's own
    // lines (onclose and the rest) reach the screen reader log here too.
    + extract('function gameLine(text) {', "sessionButton.addEventListener('click'")
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
check('the Enter Game button has the dot, label and status spans the page writes',
    ['.sess-dot', '.sess-label', '.sess-status'].every(function (s) { return SESSION_SPANS.indexOf(s) >= 0; }),
    'spans in the markup: ' + SESSION_SPANS.join(' '));
if (!label) {
    console.log('FAIL the Enter Game button has no .sess-status span');
    process.exit(1);
}

// The page connects on load.
connect.click();
check('the first click opens a socket', sockets.length === 1, 'sockets ' + sockets.length);
// The server sends the connect splash the moment the socket opens, before
// the Client.Display report arrives, so the address carries the columns.
check('the address carries the game window\'s columns', sockets[0].url === 'ws://test/ws?cols=57',
    'url ' + sockets[0].url);
check('the bar says it is connecting', label.textContent === ', connecting', 'label ' + label.textContent);
check('Enter Game is disabled while connecting', session.disabled === true, 'disabled ' + session.disabled);

// A programmatic click still reaches the handler; it must not open another.
connect.click();
check('a click while connecting opens no second socket', sockets.length === 1, 'sockets ' + sockets.length);

sockets[0].open();
check('opening forgets the room the map last centred on', global.__allRooms.lastRoomId === null,
    'lastRoomId ' + global.__allRooms.lastRoomId);
check('opening tells the server the game window\'s columns', reported.length === 1 && reported[0] === 57,
    'reported ' + JSON.stringify(reported));
// Open but at the login prompt: amber, not green (#818).
var dot = session.querySelector('.sess-dot');
check('the bar says not logged in at the login prompt', label.textContent === ', not logged in', 'label ' + label.textContent);
check('the light is amber at the login prompt', dot.style.color === 'var(--conn-reconnecting)' && !dot.style.background,
    'color ' + dot.style.color + ', background ' + dot.style.background);
check('Enter Game comes back once open', session.disabled === false, 'disabled ' + session.disabled);

// The first Char push means a character is in the world: green.
markInGame();
check('the bar says connected once in the game', label.textContent === ', connected', 'label ' + label.textContent);
check('the light is green once in the game', dot.style.background === 'var(--lamp-green)', 'background ' + dot.style.background);

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

// A copyover reconnect: the token goes first, then the width.
var order = [];
sockets[1].send = function (d) { order.push('send ' + d); };
displayReport.connected = function (cols) { order.push('width ' + cols); };
pendingReconnectToken = 'TOKEN';
sockets[1].open();
check('a reconnect sends the token before the width', order.join(', ') === 'send TOKEN, width 57',
    'order ' + order.join(', '));
// onopen resets the count to 0, then the token frame is counted: 1.
check('the token frame is counted after onopen', linesSent === 1, 'linesSent ' + linesSent);

// Music (#838): it stops when the player is really disconnected, and keeps
// playing while a live restart reconnects by itself.
// sockets[1] is open and carries no token: a plain drop, Leave Game, or the
// player pressing Enter Game while connected all end in onclose.
musicStops = 0;
pendingReconnectToken = null;
sockets[1].closed();
check('music stops when the connection closes', musicStops === 1, 'stops ' + musicStops);

// A copyover: the server sent a token, the client retries by itself.
connect.disabled = false;
connect.click();
var s = sockets[sockets.length - 1];
s.open();
musicStops = 0;
pendingReconnectToken = 'TOKEN';
reconnectAttempts = 0;
s.closed();
check('music keeps playing while a restart reconnects', musicStops === 0, 'stops ' + musicStops);
check('the client is retrying', label.textContent === ', reconnecting', 'label ' + label.textContent);

// The server sends !!MUSIC(Off) at login to stop the intro track. MP3Player.stop
// with no URL does nothing, so the page has to call stopAll. This runs the
// page's real onmessage handler.
connect.disabled = false;
connect.click();
s = sockets[sockets.length - 1];
s.open();
global.payloadsReceived = 0; global.totalBytesReceived = 0;
global.handleWebclientCommand = function () { return false; };
global.handleGMCP = function () {};
global.stripAnsi = function (t) { return t; };
global.runTriggers = function () {};
var stopCalls = [];
MusicPlayer.stop = function (u) { stopCalls.push(u); };
musicStops = 0;
s.onmessage({ data: '!!MUSIC(Off)' });
check('the login !!MUSIC(Off) stops the intro music', musicStops === 1,
    'stopAll calls ' + musicStops + ', stop(url) calls ' + stopCalls.length);
musicStops = 0;
s.onmessage({ data: '!!MUSIC(Off U=http://x/m/)' });
check('!!MUSIC(Off U=...) only sets the base URL', musicStops === 0, 'stopAll calls ' + musicStops);

// The reconnect attempts run out.
connect.disabled = false;
connect.click();
s = sockets[sockets.length - 1];
pendingReconnectToken = 'TOKEN';
reconnectAttempts = 90;
s.closed();
check('music stops when reconnecting gives up', musicStops === 1, 'stops ' + musicStops);

// Screen reader mode (static/js/screenreader.js): the address tells the
// server before the connect splash, so the splash comes as words only.
check('with the mode off the address has no sr', !/[?&]sr=/.test(sockets[0].url), 'url ' + sockets[0].url);
global.__srOn = true;
connect.disabled = false;
connect.click();
s = sockets[sockets.length - 1];
check('with the mode on the address ends in &sr=1', s.url === 'ws://test/ws?cols=57&sr=1', 'url ' + s.url);
global.__srOn = false;

// The screen reader log (static/js/screenreader.js, spec section 11): server
// text reaches it; music and the reconnect token do not; the page's own
// close messages do, so a blind player hears a dropped connection.
s.open();
global.__srWrites = [];
s.onmessage({ data: 'A rat arrives.\r\n' });
check('a plain server frame reaches the screen reader log',
    __srWrites.length === 1 && __srWrites[0] === 'A rat arrives.\r\n', JSON.stringify(__srWrites));
global.__srWrites = [];
s.onmessage({ data: '!!MUSIC(Off)' });
s.onmessage({ data: 'RELOGTKN:abc123\r\n' });
check('music and reconnect token frames never reach the log', __srWrites.length === 0, JSON.stringify(__srWrites));
pendingReconnectToken = null;
global.__srWrites = [];
s.closed();
check('a clean close says so in the log',
    __srWrites.join('|').indexOf('Connection closed cleanly') >= 0, JSON.stringify(__srWrites));
connect.disabled = false;
connect.click();
s = sockets[sockets.length - 1];
s.open();
global.__srWrites = [];
s.readyState = FakeSocket.CLOSED;
s.onclose({ wasClean: false, code: 1006, reason: '' });
check('a dropped connection says so in the log', __srWrites.join('|').indexOf('Connection died') >= 0, JSON.stringify(__srWrites));

// #922: the server clears a wrapped prompt by climbing its rows. A line the
// page wrote itself (gameLine) sits where the server does not know, so the
// next climbing clear is cut back to one row; later ones climb again.
connect.disabled = false;
connect.click();
s = sockets[sockets.length - 1];
s.open();
var termWrites = [];
var oldWrite = term.write;
term.write = function (t) { termWrites.push(t); };
var CLIMB = '\x1b[G\x1b[2K\x1b[A\x1b[2K';
var ONE = '\x1b[G\x1b[2K';
// The close messages above were the page's own lines; a server clear ends that.
s.onmessage({ data: ONE + 'Welcome back.\r\n' });
s.onmessage({ data: CLIMB + 'A rat arrives.\r\n' });
check('a wrapped prompt clear climbs its rows', termWrites[1] === CLIMB + 'A rat arrives.\r\n', JSON.stringify(termWrites[1]));
gameLine('Your browser did not allow full screen.');
s.onmessage({ data: CLIMB + 'A bat arrives.\r\n' });
check('after a line the page wrote, the clear is one row', termWrites[2] === ONE + 'A bat arrives.\r\n', JSON.stringify(termWrites[2]));
s.onmessage({ data: CLIMB + 'A cat arrives.\r\n' });
check('the next clear climbs again', termWrites[3] === CLIMB + 'A cat arrives.\r\n', JSON.stringify(termWrites[3]));
term.write = oldWrite;

// A page line wraps at word boundaries to the game window's columns, and one
// the player's tap prints (the touch deck's Dark tile, plan 2b) starts on a
// line of its own, not glued to the prompt (Task 11, 360x800 and 320x640).
var lines = [], oldWriteln = term.writeln, oldCols = term.cols;
term.writeln = function (t) { lines.push(t); };
term.cols = 24;
gameLineAfterPrompt('It is too dark to see the way. Light a torch or cast glow.');
check('a page line after the prompt starts a new line and wraps between words',
    JSON.stringify(lines) === JSON.stringify(['\r\nIt is too dark to see\r\nthe way. Light a torch\r\nor cast glow.']), JSON.stringify(lines));
check('the screen reader hears it unwrapped',
    __srWrites[__srWrites.length - 1] === '\r\nIt is too dark to see the way. Light a torch or cast glow.\r\n', JSON.stringify(__srWrites[__srWrites.length - 1]));
term.writeln = oldWriteln;
term.cols = oldCols;

// Every local write marks it, not only gameLine: the page wraps term.write and
// term.writeln where term is made, server text bypasses the wrapper, and a
// change of width marks it too (a reflowed prompt, second review of #924).
check('the page wraps term.write and term.writeln to mark local writes',
    /term\.write = function \(data, cb\) \{ localLineSincePrompt = true;/.test(html) &&
    /term\.writeln = function \(data, cb\) \{ localLineSincePrompt = true;/.test(html), 'wrapper not found');
check('server text bypasses the wrapper',
    /\(term\.writeFromServer \|\| term\.write\)\.call\(term,/.test(html), 'server write not found');
check('a change of width marks it',
    /onCols: function \(cols\) \{ localLineSincePrompt = true;/.test(html), 'onCols not found');

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
