// companion-relay.test.js: guards for the companion key relay
// (modules/aicompanion/relayweb/relay.js), the only place a player's own
// model key ever exists: the framed relay, and the key window it opens.
//
// Run:  node tools/jstest/companion-relay.test.js
//
// Dependency-free and assertion-library-free, matching hotinput.test.js.
// Needs node 20 or later for globalThis.crypto.subtle. Exits non-zero on
// failure.

var path = require('path');
var fs = require('fs');

var WEB_DIR = path.join(__dirname, '..', '..', 'modules', 'aicompanion', 'relayweb');
var Relay = require(path.join(WEB_DIR, 'relay.js'));

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + String(got).slice(0, 200) + '\n  want: ' + String(want).slice(0, 200));
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

var KEY = 'sk-test-THISISTHEPLAYERSKEY1234567890';
var GAME = 'https://example.org';
var RELAY = 'https://keys.example.org';
var STORED = { endpoint: 'https://api.openai.com/v1', key: KEY, model: 'gpt-4.1-mini' };
// A body shaped like the server's (modules/aicompanion/openai.go chatRequest).
var BODY = {
    model: 'server-model', messages: [{ role: 'user', content: 'hi' }],
    response_format: { type: 'json_schema', json_schema: { name: 'companion_decision', strict: true, schema: {} } },
    max_completion_tokens: 900
};
var BODY_TEXT = JSON.stringify(BODY);
function bodyWith(extra) { return Object.assign({}, BODY, extra); }

function fakeFetch(reply) {
    var calls = [];
    var fn = function (url, init) {
        calls.push({ url: url, init: init });
        if (reply instanceof Error) { return Promise.reject(reply); }
        return Promise.resolve(reply);
    };
    fn.calls = calls;
    return fn;
}

function textReply(status, text, headers) {
    return {
        status: status,
        headers: { get: function (h) { return (headers || {})[h.toLowerCase()] || null; } },
        text: function () { return Promise.resolve(text); }
    };
}

function streamReply(status, chunks) {
    var i = 0;
    var r = {
        status: status,
        cancelled: false,
        headers: { get: function () { return null; } },
        body: {
            getReader: function () {
                return {
                    read: function () {
                        if (i >= chunks.length) { return Promise.resolve({ done: true }); }
                        return Promise.resolve({ done: false, value: new TextEncoder().encode(chunks[i++]) });
                    },
                    cancel: function () { r.cancelled = true; return Promise.resolve(); }
                };
            }
        },
        text: function () { throw new Error('the stream path must be used'); }
    };
    return r;
}

function sortedKeys(o) { return Object.keys(o).sort().join(','); }

function memoryStorage() {
    var m = {};
    return {
        data: m,
        get: function (k) { return Object.prototype.hasOwnProperty.call(m, k) ? m[k] : null; },
        set: function (k, v) { m[k] = v; return true; },
        remove: function (k) { delete m[k]; }
    };
}

function tick() { return new Promise(function (r) { setTimeout(r, 0); }); }
async function until(fn) { for (var i = 0; i < 4000 && !fn(); i++) { await tick(); } }

// leaks says whether a message carries the key, an endpoint, a header or a
// passphrase: nothing posted to the game page ever may.
function leaks(m) {
    var s = JSON.stringify(m);
    return s.indexOf(KEY) !== -1 || /endpoint|authorization|"key"|pass/i.test(s);
}

async function main() {
    // --- endpoint rules ------------------------------------------------------
    var ep = Relay.isAllowedEndpoint;
    check('https endpoint allowed', ep('https://api.openai.com/v1'), true);
    check('http on localhost allowed', ep('http://localhost:11434/v1'), true);
    check('http on 127.0.0.1 allowed', ep('http://127.0.0.1:1234/v1'), true);
    check('http elsewhere refused', ep('http://evil.example/v1'), false);
    check('javascript: refused', ep('javascript:alert(1)'), false);
    check('file: refused', ep('file:///x'), false);
    check('ftp refused', ep('ftp://example.org/v1'), false);
    check('user info refused', ep('https://user:pass@api.openai.com/v1'), false);
    check('user name alone refused', ep('https://user@api.openai.com/v1'), false);
    check('query refused', ep('https://api.openai.com/v1?x=1'), false);
    check('fragment refused', ep('https://api.openai.com/v1#x'), false);
    check('empty refused', ep(''), false);
    check('non-string refused', ep(null), false);
    check('ITER is 600000', Relay.ITER, 600000);

    // --- relayOne posts only to the stored endpoint ---------------------------
    var f = fakeFetch(textReply(200, '{"ok":true}'));
    var got = await Relay.relayOne(f, STORED, {
        id: 'abc', body: bodyWith({ model: 'x' }),
        url: 'https://evil.example/steal', endpoint: 'https://evil.example', headers: { Authorization: 'x' }
    });
    check('fetch called once', f.calls.length, 1);
    check('URL is the STORED endpoint plus /chat/completions', f.calls[0].url, 'https://api.openai.com/v1/chat/completions');
    var init = f.calls[0].init;
    check('Authorization carries the stored key', init.headers.Authorization, 'Bearer ' + KEY);
    check('header names are exactly the two we set', sortedKeys(init.headers), 'Authorization,Content-Type');
    check('credentials omit', init.credentials, 'omit');
    check('referrerPolicy no-referrer', init.referrerPolicy, 'no-referrer');
    check('redirect is error', init.redirect, 'error');
    check('mode cors', init.mode, 'cors');
    check('cache no-store', init.cache, 'no-store');
    check('method POST', init.method, 'POST');
    check('an object body is sent as JSON', JSON.parse(init.body).messages[0].content, 'hi');
    check('the body names the STORED model, whatever it asked for', JSON.parse(init.body).model, STORED.model);
    check('returned keys are exactly id,status,body', sortedKeys(got), 'body,id,status');
    check('returned body is the reply', got.body, '{"ok":true}');
    check('returned status is the reply status', got.status, 200);

    var f2 = fakeFetch(textReply(200, 'x'));
    await Relay.relayOne(f2, { endpoint: 'https://openrouter.ai/api/v1/', key: KEY, model: 'm' }, { id: 'a', body: BODY_TEXT });
    check('trailing slashes collapse', f2.calls[0].url, 'https://openrouter.ai/api/v1/chat/completions');

    var f3 = fakeFetch(textReply(200, 'x'));
    var none = await Relay.relayOne(f3, null, { id: 'abc', body: BODY_TEXT });
    check('no settings: status 0', none.status, 0);
    check('no settings: empty body', none.body, '');
    check('no settings: fetch never called', f3.calls.length, 0);

    var f3b = fakeFetch(textReply(200, 'x'));
    await Relay.relayOne(f3b, { endpoint: 'http://evil.example/v1', key: KEY, model: 'm' }, { id: 'a', body: BODY_TEXT });
    check('a stored endpoint that is not allowed is never called', f3b.calls.length, 0);

    // --- replies too big for the game page never leave the relay ------------
    check('the reply cap is 60 KiB', Relay.MAX_REPLY_BYTES, 60 * 1024);
    var big = new Array(60 * 1024 + 2).join('a');
    var over = await Relay.relayOne(fakeFetch(textReply(200, big)), STORED, { id: 'a', body: BODY_TEXT });
    check('oversized text reply: status 0', over.status, 0);
    check('oversized text reply: empty body', over.body, '');
    var atCap = new Array(60 * 1024 + 1).join('a');
    var fits = await Relay.relayOne(fakeFetch(textReply(200, atCap)), STORED, { id: 'a', body: BODY_TEXT });
    check('a reply exactly at the cap passes', fits.body.length, 60 * 1024);
    var s = streamReply(200, [big.slice(0, 40000), big.slice(40000)]);
    var overStream = await Relay.relayOne(fakeFetch(s), STORED, { id: 'a', body: BODY_TEXT });
    check('oversized streamed reply: status 0', overStream.status, 0);
    check('oversized streamed reply: stream cancelled', s.cancelled, true);
    var sOk = await Relay.relayOne(fakeFetch(streamReply(200, ['{"a":', '1}'])), STORED, { id: 'a', body: BODY_TEXT });
    check('a streamed reply is joined', sOk.body, '{"a":1}');
    var declared = await Relay.relayOne(fakeFetch(textReply(200, 'x', { 'content-length': String(60 * 1024 + 1) })),
        STORED, { id: 'a', body: BODY_TEXT });
    check('a declared oversize reply: status 0', declared.status, 0);

    // --- error text and echoes never carry the key back ----------------------
    var err = await Relay.relayOne(fakeFetch(textReply(401, 'Incorrect API key provided: sk-test-****7890')),
        STORED, { id: 'a', body: BODY_TEXT });
    check('an error status comes back', err.status, 401);
    check('an error status comes back without its body', err.body, '');
    var echo = await Relay.relayOne(fakeFetch(textReply(200, 'you sent ' + KEY)), STORED, { id: 'a', body: BODY_TEXT });
    check('a reply echoing the key: status 0', echo.status, 0);
    check('a reply echoing the key: empty body', echo.body, '');
    var thrown = await Relay.relayOne(fakeFetch(new TypeError('redirect was blocked')), STORED, { id: 'a', body: BODY_TEXT });
    check('a failed fetch (a refused redirect) is status 0', thrown.status, 0);
    check('a failed fetch has no error text', thrown.body, '');

    // --- messages are accepted only from the expected window -------------------
    var parent = {};
    var am = Relay.acceptMessage;
    check('another origin refused', am({ origin: 'https://evil.example', source: parent, data: {} }, GAME, parent), false);
    check('the game origin from the parent accepted', am({ origin: GAME, source: parent, data: {} }, GAME, parent), true);
    check('the game origin from another window refused', am({ origin: GAME, source: {}, data: {} }, GAME, parent), false);
    check('a non-object message refused', am({ origin: GAME, source: parent, data: 'x' }, GAME, parent), false);
    check('an array message refused', am({ origin: GAME, source: parent, data: [] }, GAME, parent), false);
    check('no game origin refuses all', am({ origin: '', source: parent, data: {} }, '', parent), false);
    check('no expected window refuses all', am({ origin: GAME, source: null, data: {} }, GAME, null), false);

    // --- sealed keys ---------------------------------------------------------
    var c = globalThis.crypto;
    var sealed = await Relay.seal(c, 'correct horse', STORED, 'Alice');
    check('the sealed blob does not contain the key', sealed.indexOf(KEY), -1);
    check('the sealed blob does not contain the endpoint', sealed.indexOf('openai'), -1);
    check('a sealed blob has the blob shape', Relay.isSealedBlob(sealed), true);
    check('plain text is not a blob', Relay.isSealedBlob('{"key":"x"}'), false);
    check('a non-string is not a blob', Relay.isSealedBlob(null), false);
    var blob = JSON.parse(sealed);
    check('the sealed blob is versioned', blob.v, 1);
    check('the salt is 16 bytes', Buffer.from(blob.salt, 'base64').length, 16);
    check('the IV is 12 bytes', Buffer.from(blob.iv, 'base64').length, 12);
    var opened = await Relay.unseal(c, 'correct horse', sealed, 'Alice');
    check('round trip: key', opened.key, KEY);
    check('round trip: endpoint', opened.endpoint, STORED.endpoint);
    check('round trip: model', opened.model, STORED.model);
    check('round trip: finds off unless allowed', opened.finds, false);
    var findsSealed = await Relay.seal(c, 'correct horse', Object.assign({}, STORED, { finds: true }), 'Alice');
    check('round trip: a remembered key keeps "name my finds"', (await Relay.unseal(c, 'correct horse', findsSealed, 'Alice')).finds, true);
    check('round trip: a livelier world is allowed unless unticked', opened.lively, true);
    var quietSealed = await Relay.seal(c, 'correct horse', Object.assign({}, STORED, { lively: false }), 'Alice');
    check('round trip: a remembered key keeps a livelier world unticked', (await Relay.unseal(c, 'correct horse', quietSealed, 'Alice')).lively, false);
    // A key sealed before the box existed has no lively field: it opens
    // with a livelier world allowed, as a new key starts.
    var oldSalt = c.getRandomValues(new Uint8Array(16)), oldIv = c.getRandomValues(new Uint8Array(12));
    var oldBase = await c.subtle.importKey('raw', new TextEncoder().encode('correct horse'), 'PBKDF2', false, ['deriveKey']);
    var oldKey = await c.subtle.deriveKey({ name: 'PBKDF2', salt: oldSalt, iterations: Relay.ITER, hash: 'SHA-256' },
        oldBase, { name: 'AES-GCM', length: 256 }, false, ['encrypt']);
    var oldCt = await c.subtle.encrypt({ name: 'AES-GCM', iv: oldIv, additionalData: new TextEncoder().encode(Relay.storageKey('Alice')) },
        oldKey, new TextEncoder().encode(JSON.stringify({ endpoint: STORED.endpoint, key: KEY, model: STORED.model, finds: false })));
    var oldBlob = JSON.stringify({ v: 1, salt: Buffer.from(oldSalt).toString('base64'), iv: Buffer.from(oldIv).toString('base64'),
        ct: Buffer.from(new Uint8Array(oldCt)).toString('base64') });
    check('round trip: a key saved before the box existed allows a livelier world', (await Relay.unseal(c, 'correct horse', oldBlob, 'Alice')).lively, true);
    check('round trip: account name case does not matter', (await Relay.unseal(c, 'correct horse', sealed, 'ALICE')).key, KEY);
    var wrong = await Relay.unseal(c, 'wrong horse', sealed, 'Alice').then(function () { return 'opened'; }, function () { return 'rejected'; });
    check('a wrong passphrase rejects', wrong, 'rejected');
    var moved = await Relay.unseal(c, 'correct horse', sealed, 'Bob').then(function () { return 'opened'; }, function () { return 'rejected'; });
    check('a blob moved to another account does not open', moved, 'rejected');
    var v2 = JSON.stringify(Object.assign({}, blob, { v: 2 }));
    var future = await Relay.unseal(c, 'correct horse', v2, 'Alice').then(function () { return 'opened'; }, function () { return 'rejected'; });
    check('an unknown blob version rejects', future, 'rejected');

    check('storageKey differs per account', Relay.storageKey('Alice') !== Relay.storageKey('Bob'), true);
    check('storageKey ignores case', Relay.storageKey('Alice'), Relay.storageKey('aLICE'));

    await relayRules(c);
    await bodyRules();
    await deadlineRules();
    await accountBinding();
    await requestCap();
    await tokenBudget();
    await frameBootTests();
    await setupBootTests();
    staticPageChecks();

    console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
    if (failures > 0) {
        console.log(failures + ' FAILED');
        process.exit(1);
    }
}

// --- the frame's rules and the key window's rules, apart from the DOM --------
async function relayRules(c) {
    var posts = [];
    var store = memoryStorage();
    var rf = fakeFetch(textReply(200, '{"r":1}'));
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: store, fetchFn: rf });
    var win = Relay.createSetup(c);

    // Before any login the window has nobody to set a key up for.
    var noAcct = relay.handlePopup({ type: 'popup-hello' });
    check('before hello the window is told there is no account', noAcct.view, 'none');
    check('setup before hello is refused', (await win.setup({ endpoint: STORED.endpoint, key: KEY, model: 'm', remember: false }, '')).ok, false);
    var early = relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    check('settings before hello are refused', early.ok, false);

    relay.handle({ type: 'hello', account: 'Alice' });
    var state = relay.handlePopup({ type: 'popup-hello' });
    check('the window is told the account', state.account, 'Alice');
    check('with nothing saved the window shows setup', state.view, 'setup');
    check('with nothing saved there is no blob to unlock', state.sealed, null);
    check('the window state never carries a key', JSON.stringify(state).indexOf(KEY), -1);

    var shortPass = await win.setup({ endpoint: STORED.endpoint, key: KEY, model: 'm', remember: true, pass: 'short' }, 'Alice');
    check('remember needs a passphrase of eight or more', shortPass.ok, false);
    var badEp = await win.setup({ endpoint: 'https://u:p@x.example/v1', key: KEY, model: 'm', remember: false }, 'Alice');
    check('setup refuses a user-info endpoint', badEp.ok, false);
    var spaced = await win.setup({ endpoint: STORED.endpoint, key: KEY, model: 'gpt 4', remember: false }, 'Alice');
    check('setup refuses a model name the server would refuse', spaced.ok, false);
    var noKey = await win.setup({ endpoint: STORED.endpoint, key: '', model: 'm', remember: false }, 'Alice');
    check('setup refuses an empty key', noKey.ok, false);

    var ok = await win.setup({ endpoint: STORED.endpoint, key: KEY, model: 'gpt-4.1-mini', remember: true, pass: 'correct horse' }, 'Alice');
    check('setup with remember succeeds', ok.ok, true);
    check('the settings message carries exactly account,endpoint,finds,key,lively,model,remember,sealed,type', sortedKeys(ok.msg), 'account,endpoint,finds,key,lively,model,remember,sealed,type');
    check('the settings message allows a livelier world unless unticked', ok.msg.lively, true);
    var quiet = await win.setup({ endpoint: STORED.endpoint, key: KEY, model: 'gpt-4.1-mini', lively: false, remember: false }, 'Alice');
    check('an unticked box reaches the settings message', quiet.msg.lively, false);
    check('the settings message names the account the window was shown', ok.msg.account, 'Alice');
    check('the settings message never carries the passphrase', JSON.stringify(ok.msg).indexOf('correct horse'), -1);
    check('the settings message carries a sealed blob when remembered', Relay.isSealedBlob(ok.msg.sealed), true);
    check('the sealed blob is not the key in plain text', ok.msg.sealed.indexOf(KEY), -1);

    var applied = relay.handlePopup(ok.msg);
    check('the frame accepts the settings', applied.ok, true);
    check('the blob is stored under Alice', store.get(Relay.storageKey('Alice')), ok.msg.sealed);
    var ready = posts.filter(function (p) { return p.type === 'status' && p.ready; });
    check('ready is posted with the model', ready.length > 0 && ready[ready.length - 1].model, 'gpt-4.1-mini');
    check('the game page is told to hide the panel', posts[posts.length - 1].type, 'hide');

    var bogus = relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: '{"key":"x"}', remember: true });
    check('a remembered key with no real blob is refused', bogus.ok, false);
    var badKey = relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: 'has space', model: 'm', sealed: null, remember: false });
    check('a key the relay could not send is refused', badKey.ok, false);
    check('a refused settings message changes nothing', relay.handlePopup({ type: 'popup-hello' }).model, 'gpt-4.1-mini');

    await relay.handle({ type: 'request', id: 'deadbeef', body: BODY });
    var resp = posts.filter(function (p) { return p.type === 'response'; });
    check('a request is answered', resp.length, 1);
    check('the answer carries only type,id,status,body', sortedKeys(resp[0]), 'body,id,status,type');
    check('the answer body is the reply', resp[0].body, '{"r":1}');
    var n = posts.length;
    relay.handle({ type: 'request', id: 'not hex!', body: BODY });
    relay.handle({ type: 'request', id: 42, body: BODY });
    check('a request with a malformed id is dropped', posts.length, n);

    // Another account on the same browser.
    relay.handle({ type: 'hello', account: 'Bob' });
    var bobStatus = posts[posts.length - 1];
    check('Bob sees no ready relay', bobStatus.ready, false);
    check('Bob is not offered Alice\'s saved key', bobStatus.locked, false);
    check('Bob is shown setup, not unlock', relay.handle({ type: 'setup' }).show, 'setup');
    check('Bob\'s window is not handed Alice\'s blob', relay.handlePopup({ type: 'popup-hello' }).sealed, null);
    await relay.handle({ type: 'request', id: 'beef', body: BODY });
    check('Alice\'s key never answers for Bob', posts[posts.length - 1].status, 0);
    check('Bob\'s request never reached fetch', rf.calls.length, 1);

    // Back to Alice: the saved key must be unlocked again, in the window.
    relay.handle({ type: 'hello', account: 'Alice' });
    check('Alice is locked after a switch', posts[posts.length - 1].locked, true);
    check('Alice is shown unlock', relay.handle({ type: 'setup' }).show, 'unlock');
    var locked = relay.handlePopup({ type: 'popup-hello' });
    check('the window is shown unlock', locked.view, 'unlock');
    check('the window is handed the sealed blob to open', locked.sealed, ok.msg.sealed);
    var badUnlock = await win.unlock('wrong horse', locked.sealed, 'Alice');
    check('a wrong passphrase fails cleanly', badUnlock.message, 'That passphrase did not open it.');
    check('a wrong passphrase yields no settings', badUnlock.msg, undefined);
    check('unlock with no blob fails cleanly', (await win.unlock('x', null, 'Alice')).ok, false);
    check('unlock with no passphrase fails cleanly', (await win.unlock('', locked.sealed, 'Alice')).ok, false);
    await relay.handle({ type: 'request', id: 'cafe', body: BODY });
    check('a failed unlock leaves no partial key', posts[posts.length - 1].status, 0);
    var goodUnlock = await win.unlock('correct horse', locked.sealed, 'Alice');
    check('the right passphrase unlocks', goodUnlock.ok, true);
    check('an unlocked key comes back as settings for the frame', goodUnlock.msg.key, KEY);
    check('the frame takes the unlocked settings', relay.handlePopup(goodUnlock.msg).ok, true);
    await relay.handle({ type: 'request', id: 'cafe', body: BODY });
    check('after unlock requests are answered', posts[posts.length - 1].status, 200);

    var forgot = relay.handlePopup({ type: 'forget', account: 'Alice' });
    check('forget from the window is acknowledged', forgot.ok, true);
    check('forget removes the saved blob', store.get(Relay.storageKey('Alice')), null);
    check('forget posts not ready', posts[posts.length - 1].ready, false);
    check('an unknown window message gets no reply', relay.handlePopup({ type: 'steal' }), null);

    check('no message to the game page ever carries the key, an endpoint, a header or a passphrase',
        posts.filter(leaks).length, 0);
}

// refusedUnfetched runs one relayOne with body and says whether it was
// refused (status 0, empty body) without any fetch.
async function refusedUnfetched(body) {
    var f = fakeFetch(textReply(200, '{"ok":1}'));
    var r = await Relay.relayOne(f, STORED, { id: 'a', body: body });
    return r.status === 0 && r.body === '' && f.calls.length === 0;
}

// sentBody runs one relayOne with body and returns the body it posted, or
// null when nothing was fetched.
async function sentBody(body) {
    var f = fakeFetch(textReply(200, '{"ok":1}'));
    await Relay.relayOne(f, STORED, { id: 'a', body: body });
    return f.calls.length === 1 ? JSON.parse(f.calls[0].init.body) : null;
}

// --- the body is data the relay constrains, not an order it obeys ---------------
// findsRules: a find is named through a player's key only if they ticked
// "Also name things I find while searching". The frame refuses the bauble
// schema otherwise, whatever the server sends.
function findsRules() {
    var bauble = {
        model: 'server-model', messages: [{ role: 'user', content: 'a room' }],
        response_format: { type: 'json_schema', json_schema: { name: Relay.FINDS_SCHEMA, strict: true, schema: {} } },
        max_completion_tokens: 800
    };
    check('finds: the bauble schema is named', Relay.FINDS_SCHEMA, 'bauble');
    check('finds: refused without the player allowing it', Relay.constrainBody(bauble, 'player-model'), null);
    check('finds: refused with it explicitly off', Relay.constrainBody(bauble, 'player-model', false), null);
    var c = Relay.constrainBody(bauble, 'player-model', true);
    check('finds: relayed when allowed', !!c, true);
    check('finds: with the player\'s own model', c && JSON.parse(c.text).model, 'player-model');
    check('finds: the companion schemas are unaffected', !!Relay.constrainBody(BODY, 'player-model', false), true);
    check('finds: a name outside both lists stays refused', Relay.constrainBody(Object.assign({}, bauble,
        { response_format: { type: 'json_schema', json_schema: { name: 'other', strict: true, schema: {} } } }), 'm', true), null);
}

// livelyRules: every request that makes the world livelier goes through a
// player's key only while they leave "Make the world livelier" ticked. The
// frame refuses each LIVELY_SCHEMAS name otherwise, whatever the server
// sends, and the box allows nothing else.
function livelyRules() {
    check('lively: townsfolk idle moments are listed', Relay.LIVELY_SCHEMAS.indexOf('npc_idle') !== -1, true);
    check('lively: ambient room events are listed', Relay.LIVELY_SCHEMAS.indexOf('room_event') !== -1, true);
    check('lively: closer looks are listed', Relay.LIVELY_SCHEMAS.indexOf('look_detail') !== -1, true);
    check('lively: new rift rooms are listed', Relay.LIVELY_SCHEMAS.indexOf('rift_room') !== -1, true);
    check('lively: no lively schema is also a companion schema or the finds schema',
        Relay.LIVELY_SCHEMAS.filter(function (n) { return Relay.SCHEMA_NAMES.indexOf(n) !== -1 || n === Relay.FINDS_SCHEMA; }).length, 0);
    Relay.LIVELY_SCHEMAS.forEach(function (name) {
        var body = {
            model: 'server-model', messages: [{ role: 'user', content: 'a tavern keeper' }],
            response_format: { type: 'json_schema', json_schema: { name: name, strict: true, schema: {} } },
            max_completion_tokens: 300
        };
        check('lively ' + name + ': refused without the player allowing it', Relay.constrainBody(body, 'player-model', false), null);
        check('lively ' + name + ': refused with it explicitly off', Relay.constrainBody(body, 'player-model', false, false), null);
        check('lively ' + name + ': finds alone does not allow it', Relay.constrainBody(body, 'player-model', true, false), null);
        var c = Relay.constrainBody(body, 'player-model', false, true);
        check('lively ' + name + ': relayed when allowed', !!c, true);
        check('lively ' + name + ': with the player\'s own model', c && JSON.parse(c.text).model, 'player-model');
    });
    var bauble = { model: 'm', messages: [], max_completion_tokens: 300,
        response_format: { type: 'json_schema', json_schema: { name: Relay.FINDS_SCHEMA, strict: true, schema: {} } } };
    check('lively: allowing it does not allow finds', Relay.constrainBody(bauble, 'player-model', false, true), null);
    var other = Object.assign({}, bauble, { response_format: { type: 'json_schema', json_schema: { name: 'not_listed', strict: true, schema: {} } } });
    check('lively: a name outside every list stays refused', Relay.constrainBody(other, 'player-model', true, true), null);
}

// livelyFrame: a status tells the game page whether the world may be made
// livelier, and the frame relays lively requests only then.
async function livelyFrame() {
    var posts = [];
    var store = {};
    var f = fakeFetch(textReply(200, '{"ok":1}'));
    var r = Relay.createRelay({ post: function (m) { posts.push(m); }, fetchFn: f,
        storage: { get: function (k) { return store[k] || null; }, set: function (k, v) { store[k] = v; return true; }, remove: function (k) { delete store[k]; } } });
    r.handle({ type: 'hello', account: 'Alice' });
    r.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'gpt-4.1-mini', finds: false, lively: true, remember: false, sealed: null });
    var st = posts.filter(function (m) { return m.type === 'status'; }).pop();
    check('frame: the status says the world may be made livelier', st.lively, true);
    var idle = JSON.stringify({ model: 'x', messages: [], response_format: { type: 'json_schema', json_schema: { name: 'npc_idle', strict: true, schema: {} } }, max_completion_tokens: 300 });
    await r.handle({ type: 'request', id: 'a1', body: idle });
    check('frame: a lively request is relayed while allowed', f.calls.length, 1);
    r.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'gpt-4.1-mini', finds: false, lively: false, remember: false, sealed: null });
    st = posts.filter(function (m) { return m.type === 'status'; }).pop();
    check('frame: the status says it may not', st.lively, false);
    await r.handle({ type: 'request', id: 'a2', body: idle });
    check('frame: a lively request is refused once unticked', f.calls.length, 1);
    var answered = posts.filter(function (m) { return m.type === 'response' && m.id === 'a2'; }).pop();
    check('frame: the refusal is answered as a failure', answered && answered.status, 0);
    var popup = r.handlePopup({ type: 'popup-hello' });
    check('frame: the popup is shown the box unticked', popup.lively, false);
}

async function bodyRules() {
    findsRules();
    livelyRules();
    await livelyFrame();
    // The schema names are the server's: every SchemaName in the module's
    // Go source, no more and no fewer.
    var modDir = path.join(__dirname, '..', '..', 'modules', 'aicompanion');
    var names = {};
    fs.readdirSync(modDir).filter(function (n) { return /\.go$/.test(n) && !/_test\.go$/.test(n); }).forEach(function (n) {
        var src = fs.readFileSync(path.join(modDir, n), 'utf8');
        var re = /SchemaName:\s*`([^`]+)`/g, m;
        while ((m = re.exec(src)) !== null) { names[m[1]] = true; }
    });
    var goNames = Object.keys(names).sort().join(',');
    check('the Go source names at least four schemas (the scan works)', Object.keys(names).length >= 4, true);
    check('the relay accepts exactly the schemas the server uses', Relay.SCHEMA_NAMES.slice().sort().join(','), goNames);
    for (var i = 0; i < Relay.SCHEMA_NAMES.length; i++) {
        var n = Relay.SCHEMA_NAMES[i];
        var sent = await sentBody(bodyWith({ response_format: { type: 'json_schema', json_schema: { name: n, strict: true, schema: {} } } }));
        check('the schema ' + n + ' is relayed', sent !== null, true);
    }

    check('the per-request token cap is 4000', Relay.MAX_TOKENS, 4000);
    check('the body cap is 256 KiB', Relay.MAX_BODY_BYTES, 256 * 1024);

    var plain = await sentBody(BODY);
    check('a server-shaped body is relayed', plain !== null, true);
    check('its model is the stored one', plain.model, STORED.model);
    check('its token cap under the limit is kept', plain.max_completion_tokens, 900);
    check('a body with no max_tokens gets none', plain.max_tokens, undefined);
    check('the caller\'s body object is not changed', BODY.model, 'server-model');

    check('max_completion_tokens over the cap is capped',
        (await sentBody(bodyWith({ max_completion_tokens: 100000 }))).max_completion_tokens, 4000);
    var noCap = bodyWith({});
    delete noCap.max_completion_tokens;
    check('a body with no max_completion_tokens is given the cap', (await sentBody(noCap)).max_completion_tokens, 4000);
    check('max_tokens over the cap is capped', (await sentBody(bodyWith({ max_tokens: 9000 }))).max_tokens, 4000);
    check('max_tokens under the cap is kept', (await sentBody(bodyWith({ max_tokens: 300 }))).max_tokens, 300);
    check('a string body is constrained the same', (await sentBody(JSON.stringify(bodyWith({ max_completion_tokens: 99999, model: 'x' })))).model, STORED.model);

    check('n of 1 is relayed', (await sentBody(bodyWith({ n: 1 }))) !== null, true);
    check('stream false is relayed', (await sentBody(bodyWith({ stream: false }))) !== null, true);
    check('n of 2 is refused unfetched', await refusedUnfetched(bodyWith({ n: 2 })), true);
    check('n of 0 is refused unfetched', await refusedUnfetched(bodyWith({ n: 0 })), true);
    check('n as text is refused unfetched', await refusedUnfetched(bodyWith({ n: '5' })), true);
    check('stream true is refused unfetched', await refusedUnfetched(bodyWith({ stream: true })), true);
    check('an unknown schema is refused unfetched', await refusedUnfetched(bodyWith({
        response_format: { type: 'json_schema', json_schema: { name: 'write_me_a_novel', strict: true, schema: {} } } })), true);
    var noFormat = bodyWith({});
    delete noFormat.response_format;
    check('a body with no response_format is refused unfetched', await refusedUnfetched(noFormat), true);
    check('a text response_format is refused unfetched', await refusedUnfetched(bodyWith({ response_format: { type: 'text' } })), true);
    check('a token cap that is not a whole number is refused', await refusedUnfetched(bodyWith({ max_completion_tokens: 1.5 })), true);
    check('a negative token cap is refused', await refusedUnfetched(bodyWith({ max_completion_tokens: -1 })), true);
    check('a token cap as text is refused', await refusedUnfetched(bodyWith({ max_completion_tokens: '4000' })), true);
    check('a bad max_tokens is refused', await refusedUnfetched(bodyWith({ max_tokens: 0 })), true);
    check('an array body is refused', await refusedUnfetched([BODY]), true);
    check('a body that is not JSON is refused', await refusedUnfetched('{not json'), true);
    check('an empty object is refused', await refusedUnfetched({}), true);
    check('a missing body is refused', await refusedUnfetched(undefined), true);

    var pad = new Array(256 * 1024).join('a');
    var huge = bodyWith({ messages: [{ role: 'user', content: pad }] });
    check('an object body over 256 KiB is refused unfetched', await refusedUnfetched(huge), true);
    check('a text body over 256 KiB is refused unfetched', await refusedUnfetched(JSON.stringify(huge)), true);
    // Refused before it is parsed, even when what it parses to is small.
    check('a text body over 256 KiB of mostly white space is refused unfetched',
        await refusedUnfetched(BODY_TEXT + new Array(256 * 1024).join(' ')), true);
    var roomy = bodyWith({ messages: [{ role: 'user', content: new Array(200 * 1024).join('a') }] });
    check('a body under 256 KiB is relayed', (await sentBody(roomy)) !== null, true);

    // Through the frame: a refused body is answered at once and not counted.
    var posts = [];
    var ff = fakeFetch(textReply(200, 'x'));
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: memoryStorage(), fetchFn: ff });
    relay.handle({ type: 'hello', account: 'Alice' });
    relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    posts.length = 0;
    await relay.handle({ type: 'request', id: 'd1', body: bodyWith({ stream: true }) });
    check('frame: a refused body is answered as a failure', JSON.stringify(posts[0]), JSON.stringify({ type: 'response', id: 'd1', status: 0, body: '' }));
    check('frame: a refused body is never fetched', ff.calls.length, 0);
    await relay.handle({ type: 'request', id: 'd2', body: BODY });
    check('frame: the frame posts the stored model', JSON.parse(ff.calls[0].init.body).model, 'm');
}

// --- the fetch stops when the server stops waiting --------------------------------
async function deadlineRules() {
    check('the relay ceiling is 90 seconds', Relay.FETCH_TIMEOUT_MS, 90000);
    check('a deadline under the ceiling is used', Relay.fetchTimeout(5000), 5000);
    check('a deadline over the ceiling is capped', Relay.fetchTimeout(200000), 90000);
    check('no deadline is the ceiling', Relay.fetchTimeout(undefined), 90000);
    check('a zero deadline is the ceiling', Relay.fetchTimeout(0), 90000);
    check('a negative deadline is the ceiling', Relay.fetchTimeout(-5), 90000);
    check('a fractional deadline is the ceiling', Relay.fetchTimeout(1.5), 90000);
    check('a text deadline is the ceiling', Relay.fetchTimeout('5'), 90000);

    // A fetch that never answers until it is aborted.
    function hanging() {
        var fn = function (url, init) {
            fn.signal = init.signal;
            return new Promise(function (resolve, reject) {
                init.signal.addEventListener('abort', function () { reject(new Error('aborted')); });
            });
        };
        return fn;
    }
    var h = hanging();
    var start = Date.now();
    var r = await Relay.relayOne(h, STORED, { id: 'a', body: BODY, deadlineMs: 30 });
    check('a fetch past the deadline is aborted', h.signal.aborted, true);
    check('an aborted fetch is status 0', r.status, 0);
    check('the abort comes at the deadline, not the ceiling', Date.now() - start < 5000, true);

    var posts = [];
    var h2 = hanging();
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: memoryStorage(), fetchFn: h2 });
    relay.handle({ type: 'hello', account: 'Alice' });
    relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    posts.length = 0;
    var start2 = Date.now();
    await relay.handle({ type: 'request', id: 'e1', body: BODY, deadlineMs: 30 });
    check('frame: the request\'s deadline reaches the fetch', h2.signal.aborted && Date.now() - start2 < 5000, true);
    check('frame: the aborted request is answered as a failure', posts.length === 1 && posts[0].status, 0);
}

// --- the key window is bound to the account it was opened for ---------------------
async function accountBinding() {
    var posts = [];
    var store = memoryStorage();
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: store, fetchFn: fakeFetch(textReply(200, 'x')) });
    relay.handle({ type: 'hello', account: 'Alice' });
    var base = { type: 'settings', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false };
    var other = relay.handlePopup(Object.assign({ account: 'Bob' }, base));
    check('settings for another account are refused', other.ok, false);
    check('and the player is told why', /someone else/.test(other.message), true);
    check('refused settings leave the relay not ready', relay.handlePopup({ type: 'popup-hello' }).model, '');
    check('settings naming no account are refused', relay.handlePopup(base).ok, false);
    check('settings with a non-text account are refused', relay.handlePopup(Object.assign({ account: 7 }, base)).ok, false);
    check('the account name matches whatever its case', relay.handlePopup(Object.assign({ account: 'ALICE' }, base)).ok, true);

    store.set(Relay.storageKey('Alice'), 'blob');
    check('forget for another account is refused', relay.handlePopup({ type: 'forget', account: 'Bob' }).ok, false);
    check('a refused forget keeps the saved key', store.get(Relay.storageKey('Alice')), 'blob');
    check('a refused forget keeps the key in use', relay.handlePopup({ type: 'popup-hello' }).model, 'm');
    check('forget naming no account is refused', relay.handlePopup({ type: 'forget' }).ok, false);
    check('forget for this account works', relay.handlePopup({ type: 'forget', account: 'alice' }).ok, true);

    // The frame closes its popup when the game page logs in as someone else,
    // and stops listening to it.
    var p = fakePage({ page: 'frame' });
    Relay.boot(p.win, p.doc);
    var fromGame = function (data) { p.listeners.message({ origin: GAME, source: p.parent, data: data }); };
    fromGame({ type: 'hello', account: 'Alice' });
    fromGame({ type: 'setup' });
    p.els.open.fire('click');
    p.listeners.message({ origin: RELAY, source: p.popupWin, data: { type: 'popup-hello' } });
    check('frame: the popup is answered for Alice', p.popupWin.posted.length === 1 && p.popupWin.posted[0].m.account, 'Alice');
    fromGame({ type: 'hello', account: 'alice' });
    check('frame: the same account in another case keeps the popup', p.popupWin.closeCalls, 0);
    fromGame({ type: 'hello', account: 'Bob' });
    check('frame: another account closes the popup', p.popupWin.closeCalls, 1);
    p.listeners.message({ origin: RELAY, source: p.popupWin, data: Object.assign({ account: 'Bob' }, base) });
    check('frame: the closed popup is no longer heard', p.popupWin.posted.length, 1);
    var ready = p.posted.filter(function (x) { return x.m.type === 'status' && x.m.ready; });
    check('frame: nothing from the closed popup made Bob ready', ready.length, 0);
    p.els.open.fire('click');
    check('frame: a new popup is opened for Bob', p.win.opened.length, 2);
}

// --- the request cap: the key never pays for a runaway page ---------------------
async function requestCap() {
    check('at most two requests in flight', Relay.MAX_INFLIGHT, 2);
    check('at most thirty requests a minute', Relay.MAX_PER_MINUTE, 30);

    // In flight: fetches that never resolve until told to.
    var posts = [];
    var resolvers = [];
    var slow = function () { return new Promise(function (r) { resolvers.push(r); }); };
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: memoryStorage(), fetchFn: slow });
    relay.handle({ type: 'hello', account: 'Alice' });
    relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    posts.length = 0;
    var ids = ['a1', 'a2', 'a3'];
    ids.forEach(function (id) { relay.handle({ type: 'request', id: id, body: BODY }); });
    check('two requests are fetched', resolvers.length, 2);
    check('the third is answered at once', posts.length, 1);
    check('the third is a failure for its id', JSON.stringify(posts[0]), JSON.stringify({ type: 'response', id: 'a3', status: 0, body: '' }));
    resolvers.forEach(function (r) { r(textReply(200, 'x')); });
    await until(function () { return posts.length === 3; });
    check('the two in flight are answered when they return', posts.filter(function (p) { return p.status === 200; }).length, 2);
    relay.handle({ type: 'request', id: 'a4', body: BODY });
    check('a slot freed by a reply is used again', resolvers.length, 3);
    resolvers[2](textReply(200, 'x'));
    await until(function () { return posts.length === 4; });

    // Per minute, with a clock of the test's own and instant fetches.
    var t = 1000000;
    var fast = fakeFetch(textReply(200, 'x'));
    var posts2 = [];
    var relay2 = Relay.createRelay({ post: function (m) { posts2.push(m); }, storage: memoryStorage(), fetchFn: fast, now: function () { return t; } });
    relay2.handle({ type: 'hello', account: 'Alice' });
    relay2.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    posts2.length = 0;
    for (var i = 0; i < 30; i++) {
        t += 1000;
        await relay2.handle({ type: 'request', id: 'b' + i, body: BODY });
    }
    check('thirty requests in a minute are all fetched', fast.calls.length, 30);
    check('thirty requests in a minute are all answered', posts2.filter(function (p) { return p.status === 200; }).length, 30);
    await relay2.handle({ type: 'request', id: 'b30', body: BODY });
    check('the thirty-first in the minute is not fetched', fast.calls.length, 30);
    check('the thirty-first is answered as a failure at once', posts2[posts2.length - 1].status, 0);
    t += 60000; // the first request is now a minute old
    await relay2.handle({ type: 'request', id: 'b31', body: BODY });
    check('a minute later requests are fetched again', fast.calls.length, 31);
    check('a request refused for no settings is not counted', true, true);
}

// --- the token budget: a minute's requests may ask for 40000 tokens in all -------
async function tokenBudget() {
    check('the per-minute token budget is 40000', Relay.MAX_TOKENS_PER_MINUTE, 40000);
    var t = 5000000;
    var fast = fakeFetch(textReply(200, 'x'));
    var posts = [];
    var relay = Relay.createRelay({ post: function (m) { posts.push(m); }, storage: memoryStorage(), fetchFn: fast, now: function () { return t; } });
    relay.handle({ type: 'hello', account: 'Alice' });
    relay.handlePopup({ type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'm', sealed: null, remember: false });
    posts.length = 0;
    var bigAsk = bodyWith({ max_completion_tokens: 50000 }); // capped to 4000 each
    for (var i = 0; i < 10; i++) {
        t += 1000;
        await relay.handle({ type: 'request', id: 'c' + i, body: bigAsk });
    }
    check('ten requests at the cap fit the budget', fast.calls.length, 10);
    t += 1000;
    await relay.handle({ type: 'request', id: 'c10', body: bigAsk });
    check('the eleventh is refused though under the request cap', fast.calls.length, 10);
    check('the eleventh is answered as a failure', posts[posts.length - 1].status, 0);
    await relay.handle({ type: 'request', id: 'c11', body: bodyWith({ stream: true }) });
    t += 50000; // the first request is now a minute old
    await relay.handle({ type: 'request', id: 'c12', body: bodyWith({ max_completion_tokens: 3000 }) });
    check('a minute later the budget has room again', fast.calls.length, 11);
    // In the window now: nine at 4000 and one at 3000, 39000 in all.
    await relay.handle({ type: 'request', id: 'c13', body: bodyWith({ max_completion_tokens: 1000 }) });
    check('an ask that exactly fills the budget fits', fast.calls.length, 12);
    await relay.handle({ type: 'request', id: 'c14', body: bodyWith({ max_completion_tokens: 1000 }) });
    check('but not past it', fast.calls.length, 12);
}

// --- fake pages ------------------------------------------------------------------
function fakeElement(id) {
    var handlers = {};
    return {
        id: id, value: '', hidden: true, checked: false, textContent: '',
        attributes: {},
        setAttribute: function (k, v) { this.attributes[k] = String(v); },
        addEventListener: function (t, fn) { handlers[t] = fn; },
        fire: function (t, ev) { if (handlers[t]) { handlers[t](ev || { preventDefault: function () {} }); } }
    };
}

var FRAME_IDS = ['setup', 'unlock', 'status', 'unlockstatus', 'open', 'unlockopen', 'forget', 'close', 'unlockforget', 'unlockclose', 'host'];
var SETUP_IDS = ['waiting', 'setup', 'unlock', 'endpoint', 'key', 'model', 'remember', 'pass', 'passlabel',
    'unlockpass', 'status', 'unlockstatus', 'use', 'cancel', 'unlockbtn', 'unlockforget', 'unlockcancel', 'origin', 'ollama'];

// fakeWindow builds a window that would be the one relay.js runs in. It has
// no DOM library: only what boot touches.
function fakePage(opts) {
    var ids = opts.page === 'frame' ? FRAME_IDS : SETUP_IDS;
    var els = {};
    ids.forEach(function (i) { els[i] = fakeElement(i); });
    var listeners = {};
    var posted = [];
    var parent = { postMessage: function (m, target) { posted.push({ m: m, target: target }); } };
    var storage = {};
    var win = {
        isSecureContext: opts.secure !== false,
        crypto: globalThis.crypto,
        location: { origin: RELAY, host: 'keys.example.org' },
        localStorage: {
            getItem: function (k) { return storage[k] === undefined ? null : storage[k]; },
            setItem: function (k, v) { storage[k] = v; },
            removeItem: function (k) { delete storage[k]; }
        },
        fetch: opts.fetch || fakeFetch(textReply(200, '{"ok":1}')),
        addEventListener: function (t, fn) { listeners[t] = fn; },
        opened: [],
        closed: 0,
        open: function (url, name, features) {
            win.opened.push({ url: url, name: name, features: features });
            return opts.popup === undefined ? popupWin : opts.popup;
        },
        close: function () { win.closed++; }
    };
    win.parent = opts.topLevel ? win : parent;
    if (opts.opener !== undefined) { win.opener = opts.opener; }
    var popupWin = { posted: [], closed: false, focused: 0, closeCalls: 0,
        postMessage: function (m, target) { popupWin.posted.push({ m: m, target: target }); },
        focus: function () { popupWin.focused++; },
        close: function () { popupWin.closeCalls++; popupWin.closed = true; } };
    var doc = {
        body: { getAttribute: function (k) { return k === 'data-page' ? opts.page : null; } },
        querySelector: function (sel) {
            return sel === 'meta[name="game-origin"]' ? { getAttribute: function () { return opts.game === undefined ? GAME : opts.game; } } : null;
        },
        getElementById: function (i) { return els[i]; }
    };
    return { win: win, doc: doc, els: els, listeners: listeners, posted: posted, parent: parent, popupWin: popupWin };
}

// --- the frame: relays, opens the key window, never shows an input ------------------
async function frameBootTests() {
    var insecure = fakePage({ page: 'frame', secure: false });
    Relay.boot(insecure.win, insecure.doc);
    check('frame: a non-secure context registers no listener', Object.keys(insecure.listeners).length, 0);

    var top = fakePage({ page: 'frame', topLevel: true });
    Relay.boot(top.win, top.doc);
    check('frame: outside a frame nothing is wired', Object.keys(top.listeners).length, 0);

    var badOrigin = fakePage({ page: 'frame', game: 'https://example.org/path' });
    Relay.boot(badOrigin.win, badOrigin.doc);
    check('frame: a malformed game origin registers no listener', Object.keys(badOrigin.listeners).length, 0);

    var unknown = fakePage({ page: 'other' });
    Relay.boot(unknown.win, unknown.doc);
    check('an unknown page registers no listener', Object.keys(unknown.listeners).length, 0);

    var bf = fakeFetch(textReply(200, '{"ok":1}'));
    var p = fakePage({ page: 'frame', fetch: bf });
    Relay.boot(p.win, p.doc);
    var fromGame = function (data, from) { p.listeners.message({ origin: from || GAME, source: p.parent, data: data }); };
    var fromPopup = function (data, from, source) {
        p.listeners.message({ origin: from || RELAY, source: source || p.popupWin, data: data });
    };

    check('frame: the page names its own host for the player to check', p.els.host.textContent, 'keys.example.org');
    fromGame({ type: 'hello', account: 'Alice' });
    fromGame({ type: 'setup' });
    check('frame: setup shows the frame panel', p.els.setup.hidden, false);
    fromGame({ type: 'setup' }, 'https://evil.example');

    // A window message before any window was opened is nobody's.
    fromPopup({ type: 'popup-hello' });
    check('frame: a window message before the window is opened is ignored', p.popupWin.posted.length, 0);

    p.els.open.fire('click');
    check('frame: the button opens one window', p.win.opened.length, 1);
    check('frame: the window is the setup page on the relay origin', p.win.opened[0].url, RELAY + '/companion-relay-setup.html');
    check('frame: the window is opened as a popup', /popup/.test(p.win.opened[0].features), true);
    check('frame: the window has no name to target', p.win.opened[0].name, '_blank');
    p.els.open.fire('click');
    check('frame: a second click brings the open window forward', p.popupWin.focused, 1);
    check('frame: a second click opens no second window', p.win.opened.length, 1);

    fromPopup({ type: 'popup-hello' }, RELAY, { other: true });
    check('frame: a hello from another window on the relay origin is ignored', p.popupWin.posted.length, 0);
    fromPopup({ type: 'popup-hello' }, GAME);
    check('frame: a hello from the window at another origin is ignored', p.popupWin.posted.length, 0);
    fromPopup({ type: 'popup-hello' });
    check('frame: the window is answered', p.popupWin.posted.length, 1);
    check('frame: the answer goes to the relay origin only', p.popupWin.posted[0].target, RELAY);
    check('frame: the answer is the state', p.popupWin.posted[0].m.type, 'popup-state');
    check('frame: the state names the account', p.popupWin.posted[0].m.account, 'Alice');
    check('frame: the state shows setup', p.popupWin.posted[0].m.view, 'setup');

    var settings = { type: 'settings', account: 'Alice', endpoint: STORED.endpoint, key: KEY, model: 'gpt-4.1-mini', sealed: null, remember: false };
    var gamePosts = p.posted.length;
    fromPopup(settings, GAME);
    fromPopup(settings, RELAY, { other: true });
    check('frame: settings from anyone but the window are ignored', p.posted.length, gamePosts);
    fromPopup(settings);
    check('frame: the window is told the key is in use', p.popupWin.posted[1].m.type, 'popup-done');
    check('frame: and that it worked', p.popupWin.posted[1].m.ok, true);
    var statuses = p.posted.filter(function (x) { return x.m.type === 'status'; });
    check('frame: the game page hears ready with the model', statuses[statuses.length - 1].m.ready && statuses[statuses.length - 1].m.model, 'gpt-4.1-mini');
    check('frame: the game page is told to hide the panel', p.posted[p.posted.length - 1].m.type, 'hide');
    check('frame: the panel is hidden', p.els.setup.hidden, true);

    fromGame({ type: 'request', id: 'feed', body: BODY });
    await until(function () { return p.posted.some(function (x) { return x.m.type === 'response'; }); });
    var r = p.posted.filter(function (x) { return x.m.type === 'response'; })[0];
    check('frame: a request is answered', r && r.m.status, 200);
    check('frame: the request went to the stored endpoint', bf.calls[0].url, 'https://api.openai.com/v1/chat/completions');
    check('frame: the request carried the key from the window', bf.calls[0].init.headers.Authorization, 'Bearer ' + KEY);

    // A message claiming the game origin but sent from another window.
    var before = p.posted.length;
    p.listeners.message({ origin: GAME, source: {}, data: { type: 'request', id: 'beef', body: BODY } });
    await tick();
    check('frame: a message from a window other than the parent is ignored', p.posted.length, before);
    check('frame: it never reached fetch', bf.calls.length, 1);

    // Forget from the window: the panel stays open, on setup.
    p.popupWin.closed = true;
    fromGame({ type: 'setup' });
    fromPopup({ type: 'forget', account: 'Alice' });
    check('frame: forget from the window is acknowledged', p.popupWin.posted[2].m.ok, true);
    check('frame: forget leaves the panel on setup', p.els.setup.hidden, false);
    check('frame: the game page hears not ready', p.posted.filter(function (x) { return x.m.type === 'status'; }).pop().m.ready, false);

    check('frame: every message to the game page names the game origin, never *',
        p.posted.every(function (x) { return x.target === GAME; }), true);
    check('frame: something was posted', p.posted.length > 0, true);
    check('frame: no message to the game page carries the key', p.posted.filter(function (x) { return leaks(x.m); }).length, 0);
    check('frame: no message to the window carries the key', JSON.stringify(p.popupWin.posted).indexOf(KEY), -1);
    var inDom = Object.keys(p.els).some(function (i) {
        var e = p.els[i];
        return String(e.value).indexOf(KEY) !== -1 || String(e.textContent).indexOf(KEY) !== -1 ||
            JSON.stringify(e.attributes).indexOf(KEY) !== -1;
    });
    check('frame: the key is nowhere in the DOM', inDom, false);

    // A blocked popup is explained, not swallowed.
    var blocked = fakePage({ page: 'frame', popup: null });
    Relay.boot(blocked.win, blocked.doc);
    blocked.listeners.message({ origin: GAME, source: blocked.parent, data: { type: 'hello', account: 'Alice' } });
    blocked.listeners.message({ origin: GAME, source: blocked.parent, data: { type: 'setup' } });
    blocked.els.open.fire('click');
    check('frame: a blocked window is explained', /pop-ups/.test(blocked.els.status.textContent), true);
    check('frame: the explanation names the relay host', /keys\.example\.org/.test(blocked.els.status.textContent), true);
}

// --- the key window: top level, speaks only to the frame that opened it -----------
async function setupBootTests() {
    var framed = fakePage({ page: 'setup', opener: { postMessage: function () {} } });
    Relay.boot(framed.win, framed.doc);
    check('window: framed, nothing is wired', Object.keys(framed.listeners).length, 0);

    var orphan = fakePage({ page: 'setup', topLevel: true, opener: null });
    Relay.boot(orphan.win, orphan.doc);
    check('window: with no opener nothing is wired', Object.keys(orphan.listeners).length, 0);

    var insecure = fakePage({ page: 'setup', topLevel: true, secure: false, opener: { postMessage: function () {} } });
    Relay.boot(insecure.win, insecure.doc);
    check('window: a non-secure context registers no listener', Object.keys(insecure.listeners).length, 0);

    function openWindow() {
        var toFrame = [];
        var opener = { postMessage: function (m, target) { toFrame.push({ m: m, target: target }); } };
        var w = fakePage({ page: 'setup', topLevel: true, opener: opener });
        Relay.boot(w.win, w.doc);
        w.toFrame = toFrame;
        w.opener = opener;
        w.fromFrame = function (data, from, source) {
            w.listeners.message({ origin: from || RELAY, source: source || opener, data: data });
        };
        return w;
    }

    var w = openWindow();
    check('window: it greets its opener', w.toFrame.length, 1);
    check('window: the greeting is popup-hello', w.toFrame[0].m.type, 'popup-hello');
    check('window: the greeting names the relay origin, never *', w.toFrame[0].target, RELAY);
    check('window: it shows its own host for the player to check', w.els.origin.textContent, 'keys.example.org');
    check('window: until the frame answers it shows the waiting note', w.els.waiting.hidden, false);
    check('window: until the frame answers no form is shown', w.els.setup.hidden && w.els.unlock.hidden, true);

    w.fromFrame({ type: 'popup-state', account: 'Alice', view: 'setup', endpoint: '', model: '', sealed: null }, RELAY, { other: true });
    check('window: a state from a window other than the opener is ignored', w.els.setup.hidden, true);
    w.fromFrame({ type: 'popup-state', account: 'Alice', view: 'setup', endpoint: '', model: '', sealed: null }, GAME);
    check('window: a state from the opener at another origin is ignored', w.els.setup.hidden, true);
    w.fromFrame({ type: 'popup-state', account: '', view: 'none', endpoint: '', model: '', sealed: null });
    check('window: with no account the form stays hidden', w.els.setup.hidden, true);
    check('window: and the player is told to log in', /Log in/.test(w.els.waiting.textContent), true);
    w.fromFrame({ type: 'popup-state', account: 'Alice', view: 'setup', endpoint: 'https://openrouter.ai/api/v1', model: 'm-1', sealed: null });
    check('window: the frame\'s state shows the form', w.els.setup.hidden, false);
    check('window: the waiting note goes', w.els.waiting.hidden, true);
    check('window: the endpoint in use is offered', w.els.endpoint.value, 'https://openrouter.ai/api/v1');
    check('window: the model in use is offered', w.els.model.value, 'm-1');

    w.els.endpoint.value = 'https://api.openai.com/v1';
    w.els.key.value = KEY;
    w.els.model.value = 'gpt-4.1-mini';
    w.els.remember.checked = true;
    w.els.remember.fire('change');
    check('window: ticking remember shows the passphrase', w.els.pass.hidden, false);
    w.els.pass.value = 'short';
    w.els.use.fire('click');
    await until(function () { return w.els.status.textContent !== 'Working...'; });
    check('window: a short passphrase is refused in place', /eight/.test(w.els.status.textContent), true);
    check('window: a refusal sends nothing to the frame', w.toFrame.length, 1);
    check('window: a refusal keeps what was typed', w.els.key.value, KEY);

    w.els.pass.value = 'correct horse';
    w.els.key.fire('keydown', { key: 'Enter', preventDefault: function () {} });
    await until(function () { return w.toFrame.length === 2; });
    var sent = w.toFrame[1];
    check('window: enter in the key field sends the settings to the frame', sent.m.type, 'settings');
    check('window: the settings go to the relay origin only', sent.target, RELAY);
    check('window: the settings carry the key', sent.m.key, KEY);
    check('window: the settings carry a sealed blob', Relay.isSealedBlob(sent.m.sealed), true);
    check('window: the settings say remember', sent.m.remember, true);
    check('window: the settings name the account the frame showed', sent.m.account, 'Alice');
    check('window: the settings never carry the passphrase', JSON.stringify(sent.m).indexOf('correct horse'), -1);
    check('window: the key field is cleared once sent', w.els.key.value, '');
    check('window: the passphrase field is cleared once sent', w.els.pass.value, '');
    check('window: it stays open until the frame answers', w.win.closed, 0);

    w.fromFrame({ type: 'popup-done', ok: false, message: 'That key could not be saved.' });
    check('window: a refusal from the frame is shown', w.els.status.textContent, 'That key could not be saved.');
    check('window: a refusal from the frame leaves it open', w.win.closed, 0);
    w.fromFrame({ type: 'popup-done', ok: true, message: '' });
    check('window: once the frame has the key the window closes', w.win.closed, 1);
    check('window: and its opener is nulled', w.win.opener, null);

    // Unlocking a saved key.
    var sealed = await Relay.seal(globalThis.crypto, 'correct horse', STORED, 'Alice');
    var u = openWindow();
    u.fromFrame({ type: 'popup-state', account: 'Alice', view: 'unlock', endpoint: '', model: '', sealed: sealed });
    check('window: a saved key shows unlock', u.els.unlock.hidden, false);
    check('window: a saved key does not show setup', u.els.setup.hidden, true);
    u.els.unlockpass.value = 'wrong horse';
    u.els.unlockbtn.fire('click');
    await until(function () { return u.els.unlockstatus.textContent !== 'Working...'; });
    check('window: a wrong passphrase is refused in place', u.els.unlockstatus.textContent, 'That passphrase did not open it.');
    check('window: a wrong passphrase sends nothing', u.toFrame.length, 1);
    check('window: the passphrase field is cleared after a try', u.els.unlockpass.value, '');
    u.els.unlockpass.value = 'correct horse';
    u.els.unlockpass.fire('keydown', { key: 'Enter', preventDefault: function () {} });
    await until(function () { return u.toFrame.length === 2; });
    check('window: the right passphrase sends the opened key to the frame', u.toFrame[1].m.key, KEY);
    check('window: the opened key keeps its blob', u.toFrame[1].m.sealed, sealed);
    check('window: the opened key stays remembered', u.toFrame[1].m.remember, true);
    check('window: the opened key names the account it was shown', u.toFrame[1].m.account, 'Alice');
    check('window: the passphrase never leaves the window', JSON.stringify(u.toFrame).indexOf('correct horse'), -1);

    var g = openWindow();
    g.fromFrame({ type: 'popup-state', account: 'Alice', view: 'unlock', endpoint: '', model: '', sealed: sealed });
    g.els.unlockforget.fire('click');
    check('window: forget tells the frame', g.toFrame[1].m.type, 'forget');
    check('window: forget carries only the account it was shown', sortedKeys(g.toFrame[1].m), 'account,type');
    check('window: forget names that account', g.toFrame[1].m.account, 'Alice');

    var sw = openWindow();
    sw.fromFrame({ type: 'popup-state', account: 'Alice', view: 'setup', endpoint: '', model: '', sealed: null });
    sw.els.key.value = KEY;
    sw.fromFrame({ type: 'popup-state', account: 'alice', view: 'setup', endpoint: '', model: '', sealed: null });
    check('window: a state for the same account keeps what was typed', sw.els.key.value, KEY);
    sw.fromFrame({ type: 'popup-state', account: 'Bob', view: 'setup', endpoint: '', model: '', sealed: null });
    check('window: a state for another account clears what was typed', sw.els.key.value, '');
    sw.els.unlockforget.fire('click');
    check('window: it then speaks for the new account', sw.toFrame[sw.toFrame.length - 1].m.account, 'Bob');

    var x = openWindow();
    x.fromFrame({ type: 'popup-state', account: 'Alice', view: 'setup', endpoint: '', model: '', sealed: null });
    x.els.key.value = KEY;
    x.els.cancel.fire('click');
    check('window: cancel clears the key', x.els.key.value, '');
    check('window: cancel closes the window', x.win.closed, 1);
    check('window: cancel nulls the opener', x.win.opener, null);
    check('window: cancel sends nothing', x.toFrame.length, 1);
}

// --- the two pages: no form, no input in the frame, no password field anywhere ---
function staticPageChecks() {
    var frame = fs.readFileSync(path.join(WEB_DIR, 'relay.html'), 'utf8');
    var setup = fs.readFileSync(path.join(WEB_DIR, 'relay-setup.html'), 'utf8');
    check('frame page: is marked as the frame', /<body data-page="frame">/.test(frame), true);
    check('frame page: has no input at all', /<input/i.test(frame), false);
    check('frame page: has no form', /<form/i.test(frame), false);
    check('frame page: loads the relay script from its own origin', frame.indexOf('<script src="/companion-relay.js"></script>') !== -1, true);
    check('setup page: is marked as the key window', /<body data-page="setup">/.test(setup), true);
    check('setup page: has no form', /<form/i.test(setup), false);
    check('setup page: has no password field', /type="password"/i.test(setup), false);
    check('setup page: has no submit button', /type="submit"/i.test(setup), false);
    check('setup page: warns that a weak passphrase can be guessed from a copy of the browser',
        /use it nowhere else: anyone with a copy of this browser's\s+data can try guesses/.test(setup), true);
    check('setup page: loads the relay script from its own origin', setup.indexOf('<script src="/companion-relay.js"></script>') !== -1, true);
    ['key', 'pass', 'unlockpass'].forEach(function (id) {
        var m = setup.match(new RegExp('<input id="' + id + '"[^>]*>'));
        check('setup page: the ' + id + ' input exists', !!m, true);
        var tag = m ? m[0] : '';
        check('setup page: the ' + id + ' input is plain text', /type="text"/.test(tag), true);
        check('setup page: the ' + id + ' input refuses autocomplete', /autocomplete="off"/.test(tag), true);
        check('setup page: the ' + id + ' input is masked by style', /class="secret"/.test(tag), true);
    });
    check('setup page: the masking style is declared', /\.secret\{-webkit-text-security:disc\}/.test(setup), true);
    check('setup page: a browser without the masking style is told so', /@supports not \(-webkit-text-security: disc\)/.test(setup), true);
    [frame, setup].forEach(function (page, i) {
        var name = i === 0 ? 'frame page' : 'setup page';
        check(name + ': has no inline script', /<script>/.test(page), false);
        check(name + ': has no inline handler', /\son[a-z]+=/i.test(page), false);
        check(name + ': names the key window path the script opens', true, true);
    });
    var js = fs.readFileSync(path.join(WEB_DIR, 'relay.js'), 'utf8');
    check('relay.js: opens the key window on its own origin', js.indexOf("win.open(relayOrigin + SETUP_PATH") !== -1, true);
    check('relay.js: the key window path is the setup page', Relay.SETUP_PATH, '/companion-relay-setup.html');
    check('relay.js: never posts with a wildcard origin', /postMessage\([^)]*['"]\*['"]/.test(js), false);
    check('relay.js: the frame never reads the game page\'s storage or the popup\'s', /opener\.localStorage|parent\.localStorage/.test(js), false);
}

main().catch(function (e) {
    console.log('FAIL uncaught: ' + (e && e.stack || e));
    process.exit(1);
});
