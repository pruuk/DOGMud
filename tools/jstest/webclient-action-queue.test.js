// webclient-action-queue.test.js: a "queue at front" trigger really queues.
//
// Run:  node tools/jstest/webclient-action-queue.test.js
//
// Playtest 2026-10-17: "queue at back" worked and "queue at front" did not.
// A front entry is spliced in at the head of the queue. When that happened
// while another entry's Char.Action.Try was in flight (the round-start burst
// does exactly this: the per-round Commands.State push drains the head while
// the round's combat lines run the triggers), the Result handler retired the
// in-flight entry with actionQueue.shift(), which removed the FRONT entry
// instead. It vanished unsent, and the in-flight entry stayed queued and fired
// a second time. A back entry appends, so it never hit this.
//
// Parses the real queue code out of webclient-pure.html rather than copying
// it. Dependency-free and exits non-zero on failure.

'use strict';

var fs = require('fs');
var path = require('path');

var HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
var html = fs.readFileSync(HTML, 'utf8');

function extract(startMarker, endMarker) {
    var s = html.indexOf(startMarker);
    if (s < 0) throw new Error('marker not found in webclient-pure.html: ' + startMarker);
    var e = html.indexOf(endMarker, s + 1);
    if (e < 0) throw new Error('end marker not found in webclient-pure.html: ' + endMarker);
    return html.slice(s, e);
}

// The queue state and functions, and the Result handler's body. The handler
// is a property of the GMCPUpdateHandlers literal, so it is lifted out as a
// function of its own here; it reads the reply from GMCPStructs as the page
// does.
var queueCode = extract('var actionQueue = [];', 'function runTriggers(');
// The handler's closing line is the first "},"  at the literal's 12-space
// indent after its opening line.
var resultHandler = extract('"Char.Action.Result": function() {', '\n            },')
    .replace('"Char.Action.Result": function() {', 'function __onActionResult() {') + '\n}\n';

var sent = [];
global.GMCPStructs = {};
global.renderAutomation = function () {};
global.SendGMCP = function (pkg, body) { sent.push({ pkg: pkg, body: body }); };
(0, eval)(queueCode + '\n' + resultHandler);
(0, eval)('queueLoginSettled = true;');

var failures = 0, checks = 0;
function check(name, ok, detail) {
    checks++;
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}
function reset() {
    (0, eval)('clearActionQueue(); queueLoginSettled = true;');
    sent = [];
}
function names() { return (0, eval)('actionQueue').map(function (e) { return e.name; }); }
function reply(id, status) {
    GMCPStructs.Char = { Action: { Result: { id: id, status: status } } };
    __onActionResult(); // eslint-disable-line no-undef
}
function trig(id, name, mode) { return { id: id, name: name, queueMode: mode }; }
function newRound() { (0, eval)('queueFiredThisRound = false; drainQueue();'); }

// 1. A front entry arriving while a back entry is in flight is not lost.
reset();
enqueueTriggerAction(trig(1, 'B', 'back'), 'kick'); // eslint-disable-line no-undef
check('back entry is sent at once', sent.length === 1 && sent[0].body.id === 1, JSON.stringify(sent));
enqueueTriggerAction(trig(2, 'F', 'front'), 'bash'); // eslint-disable-line no-undef
check('front entry queues ahead of it', JSON.stringify(names()) === '["F","B"]', JSON.stringify(names()));
reply(1, 'fired');
check('the fired entry B leaves the queue, F stays', JSON.stringify(names()) === '["F"]', JSON.stringify(names()));
newRound();
check('next round sends F', sent.length === 2 && sent[1].body.id === 2 && sent[1].body.cmd === 'bash', JSON.stringify(sent));
reply(2, 'fired');
check('queue empties', names().length === 0, JSON.stringify(names()));

// 2. The same with a rejected in-flight entry.
reset();
enqueueTriggerAction(trig(1, 'B', 'back'), 'kick'); // eslint-disable-line no-undef
enqueueTriggerAction(trig(2, 'F', 'front'), 'bash'); // eslint-disable-line no-undef
reply(1, 'rejected');
check('rejected: F survives, B is gone', JSON.stringify(names()) === '["F"]', JSON.stringify(names()));

// 3. A deferred in-flight entry stays queued, behind the front entry.
reset();
enqueueTriggerAction(trig(1, 'B', 'back'), 'kick'); // eslint-disable-line no-undef
enqueueTriggerAction(trig(2, 'F', 'front'), 'bash'); // eslint-disable-line no-undef
reply(1, 'deferred');
check('deferred: both stay, front first', JSON.stringify(names()) === '["F","B"]', JSON.stringify(names()));
newRound();
check('deferred: next round sends F, not B', sent[sent.length - 1].body.id === 2, JSON.stringify(sent));

// 4. Back mode is unchanged: FIFO.
reset();
enqueueTriggerAction(trig(1, 'B1', 'back'), 'kick'); // eslint-disable-line no-undef
enqueueTriggerAction(trig(3, 'B2', 'back'), 'trip'); // eslint-disable-line no-undef
reply(1, 'fired');
check('back: B2 remains after B1 fires', JSON.stringify(names()) === '["B2"]', JSON.stringify(names()));

// ---- Audit (#375 phase 0): every queue behaviour the pause will rely on ----
// Cases are labelled A2.. to avoid clashing with the numbered cases above.

// A2. Front after front: both ahead of back, in arrival order.
reset();
enqueueTriggerAction(trig(10, 'B', 'back'), 'kick');   // in flight
enqueueTriggerAction(trig(11, 'F1', 'front'), 'bash');
enqueueTriggerAction(trig(12, 'F2', 'front'), 'trip');
check('A2 front entries keep arrival order ahead of back', JSON.stringify(names()) === '["F1","F2","B"]', JSON.stringify(names()));

// A3. Front arriving while a FRONT entry is in flight.
reset();
enqueueTriggerAction(trig(20, 'F1', 'front'), 'bash'); // in flight
enqueueTriggerAction(trig(21, 'F2', 'front'), 'trip');
reply(20, 'fired');
check('A3 in-flight front retires by identity, F2 stays', JSON.stringify(names()) === '["F2"]', JSON.stringify(names()));

// A4. Dedup: one entry per trigger.
reset();
enqueueTriggerAction(trig(30, 'B', 'back'), 'kick');
enqueueTriggerAction(trig(30, 'B', 'back'), 'kick');
check('A4 a trigger queues once', names().length === 1, JSON.stringify(names()));

// A5. Cap of 10, quiet drop.
reset();
for (var i = 0; i < 12; i++) { enqueueTriggerAction(trig(100 + i, 'Q' + i, 'back'), 'kick'); }
check('A5 queue caps at 10', names().length === 10, String(names().length));

// A6. Rejected leaves the queue.
reset();
enqueueTriggerAction(trig(40, 'B', 'back'), 'kick');
reply(40, 'rejected');
check('A6 rejected entry leaves', names().length === 0, JSON.stringify(names()));

// A7. Deferred retries next round and drops after the staleness window.
reset();
enqueueTriggerAction(trig(50, 'B', 'back'), 'kick');
for (var r = 0; r < 6; r++) { reply(50, 'deferred'); newRound(); }
check('A7 deferred entry ages out after 6 rounds', names().length === 0, JSON.stringify(names()));

// A8. One release per round.
reset();
enqueueTriggerAction(trig(60, 'A', 'back'), 'kick');
enqueueTriggerAction(trig(61, 'B', 'back'), 'bash');
reply(60, 'fired');
check('A8 second entry waits for the next round', sent.length === 1, JSON.stringify(sent));
newRound();
check('A8 next round releases the second', sent.length === 2 && sent[1].body.id === 61, JSON.stringify(sent));

// A9. Login hold: nothing sends before settle. drainQueue arms a real 2.5 s
// settle timer while held; stub setTimeout for these cases so no stray timer
// keeps node alive or fires after the summary.
var realSetTimeout = global.setTimeout;
var armed = [];
global.setTimeout = function (fn, ms) { armed.push({ fn: fn, ms: ms }); return armed.length; };
function releaseLoginHold() {
    (0, eval)('queueLoginSettled = true; queueSettleTimer = null; drainQueue();');
}
reset();
(0, eval)('queueLoginSettled = false; queueSettleTimer = null;');
enqueueTriggerAction(trig(70, 'B', 'back'), 'kick');
check('A9 nothing sends during the login hold', sent.length === 0, JSON.stringify(sent));
check('A9 the hold arms the 2.5 s fallback timer', armed.length === 1 && armed[0].ms === 2500, JSON.stringify(armed.map(function (a) { return a.ms; })));
releaseLoginHold();
check('A9 settle releases it', sent.length === 1, JSON.stringify(sent));

// A9b. The fallback timer itself settles the hold and sends.
reset();
armed = [];
(0, eval)('queueLoginSettled = false; queueSettleTimer = null;');
enqueueTriggerAction(trig(71, 'B', 'back'), 'kick');
armed[0].fn();
check('A9b fallback timer settles the hold and sends', sent.length === 1 && sent[0].body.id === 71, JSON.stringify(sent));

// A9c. Front and back entries enqueued during the hold: front goes first,
// whichever arrived first.
reset();
(0, eval)('queueLoginSettled = false; queueSettleTimer = null;');
enqueueTriggerAction(trig(72, 'B', 'back'), 'kick');
enqueueTriggerAction(trig(73, 'F', 'front'), 'bash');
check('A9c held queue orders front ahead of back', JSON.stringify(names()) === '["F","B"]' && sent.length === 0, JSON.stringify(names()) + JSON.stringify(sent));
releaseLoginHold();
check('A9c release sends the front entry first', sent.length === 1 && sent[0].body.id === 73, JSON.stringify(sent));
reply(73, 'fired');
newRound();
check('A9c then the back entry', sent.length === 2 && sent[1].body.id === 72, JSON.stringify(sent));

reset();
(0, eval)('queueLoginSettled = false; queueSettleTimer = null;');
enqueueTriggerAction(trig(74, 'F', 'front'), 'bash');
enqueueTriggerAction(trig(75, 'B', 'back'), 'kick');
releaseLoginHold();
check('A9c front first when it arrived first too', sent.length === 1 && sent[0].body.id === 74 && JSON.stringify(names()) === '["F","B"]', JSON.stringify(sent));

global.setTimeout = realSetTimeout;
reset();

// A10. Death clears.
reset();
enqueueTriggerAction(trig(80, 'B', 'back'), 'kick');
GMCPStructs.Commands = { State: { mode: 'dead' } };
(0, eval)('checkDeathClearsQueue();');
check('A10 death clears the queue', names().length === 0, JSON.stringify(names()));
GMCPStructs.Commands = { State: { mode: '' } };
(0, eval)('checkDeathClearsQueue();');

// A11. In-flight timeout: a Try that never gets a reply is retried after
// QUEUE_INFLIGHT_TIMEOUT_MS, counting an attempt each time, and dropped after
// QUEUE_STALENESS_ROUNDS timeouts. Date.now is stubbed to a test clock.
var realNow = Date.now;
var clock = realNow();
Date.now = function () { return clock; };
var TIMEOUT = (0, eval)('QUEUE_INFLIGHT_TIMEOUT_MS');
var STALE = (0, eval)('QUEUE_STALENESS_ROUNDS');
var realLog = console.log;
reset();
enqueueTriggerAction(trig(90, 'T', 'back'), 'kick');
var head = (0, eval)('actionQueue[0]');
check('A11 the Try is sent', sent.length === 1 && sent[0].body.id === 90, JSON.stringify(sent));
clock += TIMEOUT - 1;
(0, eval)('drainQueue();');
check('A11 no retry before the timeout', sent.length === 1 && head.attempts === 0, JSON.stringify(sent) + ' attempts=' + head.attempts);
clock += 2;
console.log = function () {};
(0, eval)('drainQueue();');
console.log = realLog;
check('A11 timeout counts an attempt and retries', sent.length === 2 && sent[1].body.id === 90 && head.attempts === 1, JSON.stringify(sent) + ' attempts=' + head.attempts);
console.log = function () {};
for (var k = 1; k < STALE; k++) { clock += TIMEOUT + 1; (0, eval)('drainQueue();'); }
console.log = realLog;
check('A11 dropped after ' + STALE + ' timeouts', names().length === 0 && (0, eval)('queueInFlight') === null, JSON.stringify(names()));
check('A11 sent once plus ' + (STALE - 1) + ' retries', sent.length === STALE, String(sent.length));
Date.now = realNow;
reset();

console.log((failures ? failures + ' of ' : 'all ') + checks + ' checks ' + (failures ? 'FAILED' : 'passed'));
process.exit(failures ? 1 : 0);
