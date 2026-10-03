// companion-relay-glue.test.js: guards for the game page's companion relay
// glue (_datafiles/html/public/static/js/companion-relay-glue.js). The glue
// passes a companion's model request from the server to the key relay frame
// and the frame's answer back. It must never carry a key, and it must build
// every message it sends field by field.
//
// Run:  node tools/jstest/companion-relay-glue.test.js
//
// Dependency-free and assertion-library-free, matching hotinput.test.js.
// Exits non-zero on failure.

var path = require('path');
var fs = require('fs');

var GLUE_PATH = path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'companion-relay-glue.js'
);
var PAGE_PATH = path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'
);
var Glue = require(GLUE_PATH);

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + String(got).slice(0, 300) + '\n  want: ' + String(want).slice(0, 300));
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

var RELAY = 'https://keys.example.org';
var KEY = 'sk-test-THISISTHEPLAYERSKEY1234567890';
var ID = '0123456789abcdef0123456789abcdef';

// harness builds a glue with recording fakes. frameWin stands in for the
// iframe's contentWindow; frameMsg builds an event as that window would
// send it.
function harness(opts) {
    opts = opts || {};
    var h = { sent: [], posted: [], states: [], hides: 0, frameWin: { name: 'frame' } };
    h.glue = Glue.createGlue({
        relayOrigin: opts.relayOrigin === undefined ? RELAY : opts.relayOrigin,
        sendGMCP: function (pkg, obj) { h.sent.push({ pkg: pkg, obj: obj }); return true; },
        postToFrame: function (msg, origin) { h.posted.push({ msg: msg, origin: origin }); },
        frameWindow: function () { return h.frameWin; },
        onState: function (s) { h.states.push(s); },
        onHide: function () { h.hides++; }
    });
    h.frameMsg = function (data, origin, source) {
        return { origin: origin === undefined ? RELAY : origin, source: source === undefined ? h.frameWin : source, data: data };
    };
    // up is a logged-in player whose relay frame has loaded and answered.
    h.up = function () {
        h.glue.frameLoaded();
        h.glue.hello('Meirok');
        h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: false, locked: false }));
        h.sent.length = 0;
        h.posted.length = 0;
    };
    return h;
}

function keys(o) { return Object.keys(o).sort().join(','); }

// forbidden reports any field, at any depth, whose name could carry a key.
function forbidden(v) {
    if (!v || typeof v !== 'object') { return ''; }
    var ks = Object.keys(v);
    for (var i = 0; i < ks.length; i++) {
        if (/^(key|apikey|api_key|endpoint|authorization|headers|url|pass|passphrase)$/i.test(ks[i])) { return ks[i]; }
        var inner = forbidden(v[ks[i]]);
        if (inner) { return inner; }
    }
    return '';
}

// --- relay origin ------------------------------------------------------------

check('an https origin is kept in its canonical form',
    Glue.canonicalRelayOrigin('https://Keys.Example.org:443/'), 'https://keys.example.org');
check('http is refused', Glue.canonicalRelayOrigin('http://keys.example.org'), '');
check('a path is refused', Glue.canonicalRelayOrigin('https://keys.example.org/x'), '');
check('user info is refused', Glue.canonicalRelayOrigin('https://a@keys.example.org'), '');
check('a query is refused', Glue.canonicalRelayOrigin('https://keys.example.org?x=1'), '');
check('empty is refused', Glue.canonicalRelayOrigin(''), '');
check('a non-string is refused', Glue.canonicalRelayOrigin({}), '');

// --- requests: server -> frame ------------------------------------------------

(function () {
    var h = harness();
    h.up();
    var body = { model: 'gpt-4.1-mini', messages: [{ role: 'user', content: 'hi' }] };
    h.glue.onGMCPRequest({ id: ID, body: body, key: KEY, url: 'https://evil.example' });
    check('a request is posted to the frame once', h.posted.length, 1);
    check('the request carries exactly type, id and body', keys(h.posted[0].msg), 'body,id,type');
    check('the request type is request', h.posted[0].msg.type, 'request');
    check('the request id is the server id', h.posted[0].msg.id, ID);
    check('the body is forwarded as-is, as the object it arrived as', h.posted[0].msg.body, body);
    check('the request is posted to the relay origin only', h.posted[0].origin, RELAY);
    check('nothing is sent to the server for a forwarded request', h.sent.length, 0);
}());

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {}, deadlineMs: 29950 });
    check('a request with a deadline carries it to the frame', keys(h.posted[0].msg), 'body,deadlineMs,id,type');
    check('the deadline is the server\'s', h.posted[0].msg.deadlineMs, 29950);
    ['30000', -5, 0, 1.5, null, {}].forEach(function (bad, i) {
        var id = 'ab' + i;
        h.glue.onGMCPRequest({ id: id, body: {}, deadlineMs: bad });
        var m = h.posted[h.posted.length - 1].msg;
        check('a deadline that is not a positive whole number is not passed on (' + JSON.stringify(bad) + ')',
            m.id === id && m.deadlineMs === undefined, true);
    });
}());

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: 'not hex!', body: {} });
    h.glue.onGMCPRequest(null);
    h.glue.onGMCPRequest({ id: ID });
    check('a request with a bad id, no object or no body is not forwarded', h.posted.length, 0);
}());

(function () {
    var h = harness();
    h.glue.hello('Meirok');
    h.glue.onGMCPRequest({ id: ID, body: {} });
    check('a request before the frame loads is not posted', h.posted.length, 0);
    check('a request before the frame loads fails at once', h.sent.length, 1);
    check('the failure is a Companion.Relay.Response', h.sent[0].pkg, 'Companion.Relay.Response');
    check('the failure is {id, status 0, empty body}',
        JSON.stringify(h.sent[0].obj), JSON.stringify({ id: ID, status: 0, body: '' }));
}());

// --- frame messages: who may speak -------------------------------------------

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {} });
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: 'x' }, 'https://evil.example'));
    check('a message from another origin is ignored', h.sent.length, 0);
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: 'x' }, RELAY, { name: 'other window' }));
    check('a message from another window on the relay origin is ignored', h.sent.length, 0);
    h.glue.onFrameMessage(h.frameMsg('{"type":"response"}'));
    h.glue.onFrameMessage(h.frameMsg(null));
    h.glue.onFrameMessage(null);
    check('a message that is not an object is ignored', h.sent.length, 0);
}());

// --- responses: frame -> server ----------------------------------------------

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {} });
    h.glue.onFrameMessage(h.frameMsg({
        type: 'response', id: ID, status: 200, body: '{"choices":[]}',
        key: KEY, endpoint: 'https://api.openai.com/v1', Authorization: 'Bearer ' + KEY, headers: { a: 1 }
    }));
    check('a response is sent to the server once', h.sent.length, 1);
    check('a response goes out as Companion.Relay.Response', h.sent[0].pkg, 'Companion.Relay.Response');
    check('a response carries exactly id, status and body', keys(h.sent[0].obj), 'body,id,status');
    check('a response keeps its status', h.sent[0].obj.status, 200);
    check('a response keeps its body', h.sent[0].obj.body, '{"choices":[]}');
    check('a response carries nothing key-like', forbidden(h.sent[0].obj), '');
    check('the key text appears nowhere in what was sent', JSON.stringify(h.sent).indexOf(KEY), -1);

    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: 'again' }));
    check('a second response to one id is dropped', h.sent.length, 1);
}());

(function () {
    var h = harness();
    h.up();
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: 'x' }));
    check('a response to an id never forwarded is dropped', h.sent.length, 0);
}());

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {} });
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: '200', body: { a: 1 } }));
    check('a response with a non-number status or non-string body is sent as a failure',
        JSON.stringify(h.sent[0].obj), JSON.stringify({ id: ID, status: 0, body: '' }));
}());

(function () {
    // The server closes a websocket on a frame over 64 KiB, so anything that
    // would reach 60 KiB is replaced by a failure.
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {} });
    var overhead = Glue.frameBytes('Companion.Relay.Response', { id: ID, status: 200, body: '' });
    var fits = new Array(Glue.MAX_FRAME_BYTES - overhead + 1).join('a');
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: fits }));
    check('a reply exactly at the cap is sent whole', h.sent[0].obj.body.length, fits.length);
    check('the frame at the cap measures the cap',
        Glue.frameBytes('Companion.Relay.Response', h.sent[0].obj), Glue.MAX_FRAME_BYTES);

    var id2 = 'ab' + ID.slice(2);
    h.glue.onGMCPRequest({ id: id2, body: {} });
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: id2, status: 200, body: fits + 'a' }));
    check('a reply one byte over the cap becomes a failure',
        JSON.stringify(h.sent[1].obj), JSON.stringify({ id: id2, status: 0, body: '' }));

    // JSON escaping and UTF-8 both grow a body: measure the encoded frame,
    // not the string's length.
    var id3 = 'cd' + ID.slice(2);
    h.glue.onGMCPRequest({ id: id3, body: {} });
    var quotes = new Array(Math.floor(Glue.MAX_FRAME_BYTES / 2)).join('"');
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: id3, status: 200, body: quotes }));
    check('a body that grows past the cap once JSON-escaped becomes a failure', h.sent[2].obj.body, '');
    var id4 = 'ef' + ID.slice(2);
    h.glue.onGMCPRequest({ id: id4, body: {} });
    var wide = new Array(Math.floor(Glue.MAX_FRAME_BYTES / 2)).join('é');
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: id4, status: 200, body: wide }));
    check('a body that grows past the cap once UTF-8 encoded becomes a failure', h.sent[3].obj.body, '');
    check('the cap is 60 KiB', Glue.MAX_FRAME_BYTES, 60 * 1024);
}());

// --- status: frame -> server -------------------------------------------------

(function () {
    var h = harness();
    h.glue.frameLoaded();
    h.glue.hello('Meirok');
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini', locked: false, key: KEY }));
    check('a ready status becomes Companion.Relay.Ready', h.sent[0].pkg, 'Companion.Relay.Ready');
    check('Ready carries only the model and the finds and lively choices', JSON.stringify(h.sent[0].obj), JSON.stringify({ model: 'gpt-4.1-mini', finds: false, lively: false }));
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini', finds: true, locked: false }));
    check('the player allowing their key to name finds reaches the server', JSON.stringify(h.sent[1].obj), JSON.stringify({ model: 'gpt-4.1-mini', finds: true, lively: false }));
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini', finds: 'yes', locked: false }));
    check('finds is true only when it is exactly true', JSON.stringify(h.sent[2].obj), JSON.stringify({ model: 'gpt-4.1-mini', finds: false, lively: false }));
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini', lively: true, locked: false }));
    check('the player allowing a livelier world reaches the server', JSON.stringify(h.sent[3].obj), JSON.stringify({ model: 'gpt-4.1-mini', finds: false, lively: true }));
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini', lively: 'yes', locked: false }));
    check('lively is true only when it is exactly true', JSON.stringify(h.sent[4].obj), JSON.stringify({ model: 'gpt-4.1-mini', finds: false, lively: false }));
    h.sent.splice(1, 4);

    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: false, locked: true }));
    check('a not-ready status becomes Companion.Relay.Gone', h.sent[1].pkg, 'Companion.Relay.Gone');
    check('Gone carries nothing', JSON.stringify(h.sent[1].obj), '{}');
    check('a locked relay is reported to the page', h.states[h.states.length - 1].locked, true);

    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'has space' }));
    check('a ready status with an unusable model is sent as Gone', h.sent[2].pkg, 'Companion.Relay.Gone');

    var all = JSON.stringify(h.sent);
    check('no status sent carries a key-like field', h.sent.map(function (s) { return forbidden(s.obj); }).join(''), '');
    check('no status sent carries the key text', all.indexOf(KEY), -1);
}());

(function () {
    var h = harness();
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: true, model: 'gpt-4.1-mini' }));
    check('a status before any hello is ignored', h.sent.length, 0);
}());

// --- hello and the logged-in state -------------------------------------------

(function () {
    var h = harness();
    h.glue.hello('Meirok');
    check('hello waits for the frame to load', h.posted.length, 0);
    h.glue.frameLoaded();
    check('hello is posted once the frame loads', h.posted.length, 1);
    check('hello carries exactly type and account', keys(h.posted[0].msg), 'account,type');
    check('hello names the account', h.posted[0].msg.account, 'Meirok');
    h.glue.hello('Meirok');
    check('the same account is not announced twice', h.posted.length, 1);
    h.glue.frameLoaded();
    check('a reloaded frame is told the account again', h.posted.length, 2);
    h.glue.hello('Other');
    check('another account is announced', h.posted[2].msg.account, 'Other');
    h.glue.hello('');
    h.glue.hello(42);
    check('an empty or non-string account is not announced', h.posted.length, 3);
}());

(function () {
    var h = harness();
    check('the button stays hidden before login', h.glue.state().visible, false);
    h.up();
    check('the button shows once logged in and the relay answered', h.glue.state().visible, true);
    h.glue.disconnected();
    check('the button hides on disconnect', h.glue.state().visible, false);
    check('the setup view is hidden on disconnect', h.hides > 0, true);
    h.glue.onGMCPRequest({ id: ID, body: {} });
    check('a request after disconnect is not forwarded', h.posted.length, 0);
    h.glue.hello('Meirok');
    check('after a reconnect the same account is announced again', h.posted.length, 1);
    check('and it is a hello', h.posted[0].msg.type, 'hello');
}());

(function () {
    var h = harness();
    h.up();
    h.glue.onGMCPRequest({ id: ID, body: {} });
    check('the request was forwarded before the disconnect', h.posted.length, 1);
    h.glue.disconnected();
    h.glue.hello('Meirok');
    h.glue.onFrameMessage(h.frameMsg({ type: 'response', id: ID, status: 200, body: 'x' }));
    check('a response to a request from before a disconnect is dropped',
        h.sent.filter(function (s) { return s.pkg === 'Companion.Relay.Response' && s.obj.status === 200; }).length, 0);
}());

// --- setup ---------------------------------------------------------------------

(function () {
    var h = harness();
    h.glue.frameLoaded();
    check('openSetup before hello is refused', h.glue.openSetup(), false);
    check('openSetup before hello posts nothing', h.posted.length, 0);
    h.glue.hello('Meirok');
    h.posted.length = 0;
    check('openSetup before the relay answers is refused', h.glue.openSetup(), false);
    h.glue.onFrameMessage(h.frameMsg({ type: 'status', ready: false, locked: false }));
    check('openSetup after hello is allowed', h.glue.openSetup(), true);
    check('openSetup posts exactly {type:setup}', JSON.stringify(h.posted[0].msg), JSON.stringify({ type: 'setup' }));
    check('openSetup shows the frame', h.glue.state().open, true);
    h.glue.onFrameMessage(h.frameMsg({ type: 'hide' }));
    check('a hide from the frame closes it', h.glue.state().open, false);
    h.glue.onFrameMessage(h.frameMsg({ type: 'hide' }, 'https://evil.example'));
    check('a hide from elsewhere does nothing', h.hides, 1);
}());

(function () {
    var h = harness({ relayOrigin: 'http://keys.example.org' });
    h.glue.frameLoaded();
    h.glue.hello('Meirok');
    check('a glue with no usable relay origin never posts', h.posted.length, 0);
    check('a glue with no usable relay origin never opens', h.glue.openSetup(), false);
}());

// --- boot: the frame it builds ----------------------------------------------------

function fakeDom() {
    function El(tag) {
        this.tagName = tag.toUpperCase(); this.attrs = {}; this.style = {}; this.children = [];
        this.listeners = {}; this.textContent = '';
    }
    El.prototype.setAttribute = function (k, v) { this.attrs[k.toLowerCase()] = String(v); };
    El.prototype.appendChild = function (c) { this.children.push(c); return c; };
    El.prototype.addEventListener = function (t, f) { (this.listeners[t] = this.listeners[t] || []).push(f); };
    var dom = { made: [], body: new El('body') };
    dom.doc = {
        body: dom.body,
        createElement: function (t) { var e = new El(t); dom.made.push(e); return e; }
    };
    dom.win = {
        listeners: {},
        addEventListener: function (t, f) { (this.listeners[t] = this.listeners[t] || []).push(f); }
    };
    dom.button = new El('button');
    dom.button.style.display = 'none';
    return dom;
}

(function () {
    var dom = fakeDom();
    dom.win.COMPANION_RELAY_ORIGIN = '';
    check('boot does nothing with no relay origin', Glue.boot(dom.win, dom.doc, { button: dom.button }), null);
    check('boot with no relay origin builds no frame', dom.made.length, 0);
    dom.win.COMPANION_RELAY_ORIGIN = 'http://keys.example.org';
    check('boot does nothing with a non-https relay origin', Glue.boot(dom.win, dom.doc, { button: dom.button }), null);
}());

(function () {
    var dom = fakeDom();
    var sent = [];
    dom.win.COMPANION_RELAY_ORIGIN = 'https://keys.example.org';
    var glue = Glue.boot(dom.win, dom.doc, {
        button: dom.button, sendGMCP: function (p, o) { sent.push({ pkg: p, obj: o }); return true; }
    });
    var frame = dom.made.filter(function (e) { return e.tagName === 'IFRAME'; })[0];
    check('boot builds one frame', !!frame, true);
    check('the frame loads the relay page on the relay origin', frame.src, RELAY + '/companion-relay.html');
    check('the frame is sandboxed to scripts, its own origin and opening the key window, with no forms',
        frame.attrs.sandbox, 'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox');
    check('the frame sends no referrer', frame.attrs.referrerpolicy, 'no-referrer');
    check('the frame is given no browser features', frame.attrs.allow, undefined);
    check('the frame has no name to target', frame.attrs.name === undefined && frame.name === undefined, true);
    check('the button stays hidden after boot', dom.button.style.display, 'none');

    var posted = [];
    frame.contentWindow = { postMessage: function (m, o) { posted.push({ msg: m, origin: o }); } };
    frame.listeners.load.forEach(function (f) { f(); });
    glue.hello('Meirok');
    check('boot posts hello to the relay origin only', posted[0] && posted[0].origin, RELAY);
    var onMessage = dom.win.listeners.message[0];
    onMessage({ origin: RELAY, source: { other: true }, data: { type: 'status', ready: false, locked: false } });
    check('boot ignores a message from a window other than its frame', dom.button.style.display, 'none');
    onMessage({ origin: RELAY, source: frame.contentWindow, data: { type: 'status', ready: false, locked: true } });
    check('the button shows once the relay answers', dom.button.style.display, '');
    check('a locked key offers to unlock', dom.button.textContent, 'Unlock companion key');
    dom.button.listeners.click.forEach(function (f) { f(); });
    check('the button opens setup', JSON.stringify(posted[posted.length - 1].msg), JSON.stringify({ type: 'setup' }));
    var overlay = dom.body.children[0];
    check('setup shows the frame', overlay.style.display, 'flex');
    onMessage({ origin: RELAY, source: frame.contentWindow, data: { type: 'hide' } });
    check('the relay hides it again', overlay.style.display, 'none');
    glue.disconnected();
    check('the button hides on disconnect', dom.button.style.display, 'none');
}());

// --- the page wiring -------------------------------------------------------------

(function () {
    var page = fs.readFileSync(PAGE_PATH, 'utf8');
    check('the page takes the relay origin as an unquoted JSON literal',
        page.indexOf('window.COMPANION_RELAY_ORIGIN = {{ .COMPANION_RELAY_ORIGIN_JSON }};') !== -1, true);
    check('the page includes the glue script',
        page.indexOf('/static/js/companion-relay-glue.js"></script>') !== -1, true);
    check('the page handles Companion.Relay.Request',
        /"Companion\.Relay\.Request"\s*:\s*function/.test(page), true);
    // An attribute glued to the closing quote of the one before it
    // (id="x"onclick=...) parses, but only by accident of the HTML parser.
    check('no event attribute is glued to the attribute before it', /"on[a-z]+=/.test(page), false);

    var src = fs.readFileSync(GLUE_PATH, 'utf8');
    check('the glue never posts to the frame with a wildcard origin', /postMessage\([^)]*['"]\*['"]/.test(src), false);
    check('the glue sets the frame sandbox',
        src.indexOf("'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox'") !== -1, true);
    check('the glue never gives the frame allow-forms', /setAttribute\('sandbox', '[^']*allow-forms/.test(src), false);
    check('the glue sets no referrer on the frame', src.indexOf("'no-referrer'") !== -1, true);
    check('the glue never spreads a frame message into what it sends',
        /Object\.assign|\.\.\.\s*(m|msg|data|ev)/.test(src), false);
}());

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) { process.exit(1); }
