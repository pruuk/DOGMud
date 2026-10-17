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

console.log((failures ? failures + ' of ' : 'all ') + checks + ' checks ' + (failures ? 'FAILED' : 'passed'));
process.exit(failures ? 1 : 0);
