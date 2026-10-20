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

var queueCode = extract('var actionQueue = [];', 'function runTriggers(');
var triggerCode = extract('function runTriggers(', '// Left-click on an auto-chip');
var helpers = extract('function stripAnsi(', 'var triggerLastFire = {};') + 'var triggerLastFire = {}; var TRIGGER_MIN_INTERVAL_MS = 1000;\n'
    + extract('function fireTriggerCommands(', '/////////////////////////////////////////////');
var tickCode = extract('function rebuildTickTimers() {', '/////////////////////////////////////////////');
function lift(name) {
    return extract('"' + name + '": function() {', '\n            },')
        .replace('"' + name + '": function() {', 'function __' + name.replace('.', '_') + '() {') + '\n}\n';
}

var sentData = [], sentGMCP = [], intervals = [];
global.GMCPStructs = { Char: {} };
global.renderAutomation = function () {};
global.SendData = function (d) { sentData.push(d); return true; };
global.SendGMCP = function (pkg, body) { sentGMCP.push({ pkg: pkg, body: body }); };
global.setInterval = function (fn, ms) { intervals.push(fn); return intervals.length; };
global.clearInterval = function () {};
global.tickTimers = {}; global.tickTimersSig = '';
(0, eval)(helpers + queueCode + triggerCode + tickCode + lift('Char.Menu') + lift('Char.Automation'));

var failures = 0;
function check(name, ok, detail) { if (ok) { console.log('ok   ' + name); } else { failures++; console.log('FAIL ' + name + '\n  ' + detail); } }
function setMenu(open) { GMCPStructs.Char.Menu = { open: open }; __Char_Menu(); }
function automation(a) { GMCPStructs.Char.Automation = a; __Char_Automation(); }
function fireTicks() { intervals.forEach(function (fn) { fn(); }); }
function newRound() { (0, eval)('queueFiredThisRound = false; drainQueue();'); }

// Login: nothing before the first Char.Automation.
(0, eval)('queueLoginSettled = true;');
GMCPStructs.Char.Automation = { ticks: [], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] };
runTriggers('You are hungry.', Date.now());
check('no trigger fires before the session automation push', sentData.length === 0, JSON.stringify(sentData));

automation({ ticks: [{ id: 5, enabled: true, intervalSec: 10, commands: 'score;look' }], triggers: [{ id: 1, enabled: true, pattern: 'You are hungry*', thenCmds: 'eat bread' }] });
fireTicks();
check('ticks send after the automation push', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
sentData = [];

// Menu open: ticks and immediate triggers queue.
setMenu(true);
fireTicks();
runTriggers('You are hungry.', Date.now() + 5000);
check('nothing is sent while a menu is open', sentData.length === 0 && sentGMCP.length === 0, JSON.stringify([sentData, sentGMCP]));
var q = (0, eval)('actionQueue');
check('the tick and the trigger are queued', q.length === 2, JSON.stringify(q));
fireTicks();
check('a tick queues once (dedup)', (0, eval)('actionQueue').length === 2, String((0, eval)('actionQueue').length));

// Close: paced release, one per round, every command of the entry.
setMenu(false);
check('first entry released on close', JSON.stringify(sentData) === '["score","look"]', JSON.stringify(sentData));
newRound();
check('second entry released next round', JSON.stringify(sentData) === '["score","look","eat bread"]', JSON.stringify(sentData));

// Socket close: everything stops and resets.
sentData = [];
(0, eval)('stopAutomation();');
fireTicks();
runTriggers('You are hungry.', Date.now() + 10000);
check('nothing fires after a disconnect', sentData.length === 0, JSON.stringify(sentData));
check('queue emptied on disconnect', (0, eval)('actionQueue').length === 0, '');

console.log(failures ? failures + ' FAILED' : 'all ok');
process.exit(failures ? 1 : 0);
