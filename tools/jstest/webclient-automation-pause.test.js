// webclient-automation-pause.test.js: ticks and triggers queue while a menu
// is open, release paced after it closes, and never fire during login (#375).
//
// Run:  node tools/jstest/webclient-automation-pause.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
var html = fs.readFileSync(HTML, 'utf8');
function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}

var BANNER = '/////////////////////////////////////////////';
var queueCode = extract('var actionQueue = [];', 'function runTriggers(');
var triggerCode = extract('function runTriggers(', '// Left-click on an auto-chip');
// stripAnsi through fireTriggerCommands, including the page's own
// triggerLastFire and TRIGGER_MIN_INTERVAL_MS.
var helpers = extract('function stripAnsi(', BANNER);
var tickCode = extract('function rebuildTickTimers() {', BANNER);
var renderCode = extract('function renderAutomation() {', 'function rebuildTickTimers() {')
    .replace('function renderAutomation() {', 'function __renderAutomationReal() {');
function lift(name) {
    return extract('"' + name + '": function() {', '\n            },')
        .replace('"' + name + '": function() {', 'function __' + name.replace(/\./g, '_') + '() {') + '\n}\n';
}

// Interval stub: clearInterval nulls the timer's slot, so the disconnect
// checks can see directly that every timer was cleared. The tick callback's
// own automationReady check is proven separately, by calling a callback
// captured before the disconnect.
var sentData = [], sentGMCP = [], intervals = [];
global.GMCPStructs = { Char: {} };
global.renderAutomation = function () {};
global.SendData = function (d) { sentData.push(d); return true; };
global.SendGMCP = function (pkg, body) { sentGMCP.push({ pkg: pkg, body: body }); };
global.setInterval = function (fn, ms) { intervals.push(fn); return intervals.length; };
global.clearInterval = function (id) { if (id >= 1 && id <= intervals.length) intervals[id - 1] = null; };
global.tickTimers = {}; global.tickTimersSig = '';
global.autoActiveTab = 'triggers';
(0, eval)(helpers + queueCode + triggerCode + tickCode + renderCode
    + lift('Char.Menu') + lift('Char.Automation') + lift('Char.Action.Result'));

var failures = 0;
function check(name, ok, detail) { if (ok) { console.log('ok   ' + name); } else { failures++; console.log('FAIL ' + name + '\n  ' + detail); } }
function setMenu(open) { GMCPStructs.Char.Menu = { open: open }; __Char_Menu(); }
function automation(a) { GMCPStructs.Char.Automation = a; __Char_Automation(); }
function fireTicks() { intervals.forEach(function (fn) { if (fn) fn(); }); }
function newRound() { (0, eval)('queueFiredThisRound = false; drainQueue();'); }
function q() { return (0, eval)('actionQueue'); }
function qnames() { return q().map(function (e) { return e.name; }); }
function reply(id, status) { GMCPStructs.Char.Action = { Result: { id: id, status: status } }; __Char_Action_Result(); }
var MIN_GAP = (0, eval)('TRIGGER_MIN_INTERVAL_MS');
var clock = Date.now();
function later() { clock += MIN_GAP + 1; return clock; }

// Login: nothing before the first Char.Automation.
(0, eval)('queueLoginSettled = true;');
GMCPStructs.Char.Automation = { ticks: [], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] };
runTriggers('You are hungry.', later());
check('no trigger fires before the session automation push', sentData.length === 0, JSON.stringify(sentData));

automation({ ticks: [{ id: 5, enabled: true, intervalSec: 10, commands: 'score;look' }], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] });
fireTicks();
check('ticks send after the automation push', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
sentData = [];

// Menu open: ticks and immediate triggers queue.
setMenu(true);
fireTicks();
runTriggers('You are hungry.', later());
check('nothing is sent while a menu is open', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
check('the tick and the trigger are queued', q().length === 2, JSON.stringify(q()));
fireTicks();
check('a tick queues once (dedup)', q().length === 2, String(q().length));

// The drain itself is gated: a queue-mode trigger and a direct drain call
// send nothing while the menu is open.
GMCPStructs.Char.Automation.triggers.push({ id: 2, enabled: true, pattern: 'You feel weak*', thenCmds: 'kick', queueMode: 'back' });
runTriggers('You feel weak.', later());
(0, eval)('drainQueue();');
check('drainQueue sends nothing while a menu is open', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
check('the back-mode trigger queues behind the direct entries', q().length === 3 && q()[2].triggerId === 2 && !q()[2].direct, JSON.stringify(q()));

// Close: paced release, one per round, every command of the entry.
setMenu(false);
check('first entry released on close', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
(0, eval)('drainQueue();');
check('a second drain in the same round sends nothing more', JSON.stringify(sentData) === '["score","look"]' && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
newRound();
check('second entry released next round', JSON.stringify(sentData) === '["score","look","eat bread"]', JSON.stringify(sentData));
newRound();
check('the back-mode entry goes out as a Try the round after', sentGMCP.length === 1 && sentGMCP[0].pkg === 'Char.Action.Try' && sentGMCP[0].body.id === 2 && sentGMCP[0].body.cmd === 'kick', JSON.stringify(sentGMCP));
reply(2, 'fired');

// Socket close: everything stops and resets.
sentData = []; sentGMCP = [];
setMenu(true);
var liveTicks = intervals.filter(function (fn) { return fn; });
var capturedTick = liveTicks[liveTicks.length - 1];
check('a tick timer is running before the disconnect', liveTicks.length >= 1, String(liveTicks.length));
(0, eval)('stopAutomation();');
check('disconnect clears every tick timer', intervals.every(function (fn) { return fn === null; }), JSON.stringify(intervals.map(function (fn) { return fn ? 'live' : null; })));
capturedTick();
check('a tick callback that outlives the disconnect sends nothing', sentData.length === 0 && q().length === 0, JSON.stringify([sentData, q()]));
fireTicks();
runTriggers('You are hungry.', later());
check('nothing fires after a disconnect', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
check('queue emptied on disconnect', q().length === 0, JSON.stringify(q()));
check('menuOpen reset on disconnect', (0, eval)('menuOpen') === false, String((0, eval)('menuOpen')));
check('queueLoginSettled reset on disconnect', (0, eval)('queueLoginSettled') === false, String((0, eval)('queueLoginSettled')));

// Source check: socket.onclose calls stopAutomation() as its first statement.
var onclose = extract('socket.onclose = function(event) {', '\n            };');
var firstStmt = onclose.split('\n')[1].trim();
check('socket.onclose calls stopAutomation() first', /^stopAutomation\(\);/.test(firstStmt), firstStmt);

// ---- Mixed queue: direct entries and Try entries share one pacing ----
function freshSession() {
    (0, eval)('stopAutomation(); queueLoginSettled = true;');
    automation({ ticks: [{ id: 5, enabled: true, intervalSec: 10, commands: 'score;look' }], triggers: [] });
    sentData = []; sentGMCP = [];
}

// M1. A direct entry behind an in-flight Try waits for the reply AND the next round.
freshSession();
enqueueTriggerAction({ id: 7, name: 'B', queueMode: 'back' }, 'kick');
check('M1 the Try entry is in flight', sentGMCP.length === 1 && sentGMCP[0].body.id === 7, JSON.stringify(sentGMCP));
setMenu(true);
fireTicks();
setMenu(false);
check('M1 the direct entry waits while the Try is in flight', sentData.length === 0, JSON.stringify(sentData));
reply(7, 'fired');
check('M1 the direct entry waits for the next round after the reply', sentData.length === 0, JSON.stringify(sentData));
newRound();
check('M1 the next round sends the direct entry', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));

// M2. A front Try entry arriving while a direct entry is queued goes ahead of it.
freshSession();
setMenu(true);
fireTicks();
enqueueTriggerAction({ id: 8, name: 'F', queueMode: 'front' }, 'bash');
check('M2 the front entry sits ahead of the direct entry', JSON.stringify(qnames()) === '["F","tick: 5"]', JSON.stringify(qnames()));
check('M2 nothing sent while the menu is open', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
setMenu(false);
check('M2 close sends the front Try first', sentGMCP.length === 1 && sentGMCP[0].body.id === 8 && sentData.length === 0, JSON.stringify([sentData, sentGMCP]));
reply(8, 'fired');
newRound();
check('M2 then the direct entry next round', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));

// ---- Display: a queued tick renders under its own name, unwired ----
function fakeEl(tag) {
    return {
        tagName: tag, className: '', textContent: '', dataset: {}, children: [],
        get firstChild() { return this.children[0] || null; },
        appendChild: function (c) { this.children.push(c); return c; },
        removeChild: function (c) { this.children.splice(this.children.indexOf(c), 1); return c; },
        addEventListener: function () {}
    };
}
var autoList = fakeEl('div');
global.document = { getElementById: function (id) { return id === 'auto-list' ? autoList : null; }, createElement: fakeEl };
function rowText(row) { return row.children.map(function (c) { return c.textContent; }).join(' | '); }

// D1. With a trigger configured.
freshSession();
GMCPStructs.Char.Automation.triggers = [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread', name: 'Hunger' }];
setMenu(true);
fireTicks();
__renderAutomationReal();
var rows = autoList.children.filter(function (c) { return /auto-row/.test(c.className); });
var tickRow = rows[0];
check('D1 the queued tick row shows its own name', !!tickRow && tickRow.children.some(function (c) { return c.className === 'name' && c.textContent === 'tick: 5'; }), tickRow ? rowText(tickRow) : 'no row');
check('D1 the queued tick row is unwired', !!tickRow && tickRow.dataset.autoType === undefined && tickRow.dataset.autoId === undefined && tickRow.dataset.autoKey === undefined, tickRow ? JSON.stringify(tickRow.dataset) : 'no row');
check('D1 the configured trigger still renders as a trigger row', rows.length === 2 && rows[1].dataset.autoType === 'trigger' && rows[1].dataset.autoId === '1', JSON.stringify(rows.map(function (r) { return r.dataset; })));

// D2. With no triggers configured, the queued tick still shows.
GMCPStructs.Char.Automation.triggers = [];
__renderAutomationReal();
var rows2 = autoList.children.filter(function (c) { return /auto-row/.test(c.className); });
var empty2 = autoList.children.filter(function (c) { return c.className === 'auto-empty'; });
check('D2 queued tick shows when no triggers are configured', rows2.length === 1 && empty2.length === 1, autoList.children.map(function (c) { return c.className; }).join(','));

(0, eval)('stopAutomation();');
console.log(failures ? failures + ' FAILED' : 'all ok');
process.exit(failures ? 1 : 0);
