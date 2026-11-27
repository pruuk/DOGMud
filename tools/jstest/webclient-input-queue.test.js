// webclient-input-queue.test.js: while the server holds as many of the
// player's commands as it will (GMCP Char.Input.Queue full), the controls that
// send commands grey out and their clicks send nothing; a free frame or a new
// connection undoes it (owner ruling 2026-11-25).
//
// Run:  node tools/jstest/webclient-input-queue.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var ROOT = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var html = fs.readFileSync(path.join(ROOT, 'webclient-pure.html'), 'utf8');
var css = fs.readFileSync(path.join(ROOT, 'static', 'css', 'dashboard.css'), 'utf8');

function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}

var failures = 0;
function check(name, ok, detail) { if (ok) { console.log('ok   ' + name); } else { failures++; console.log('FAIL ' + name + '\n  ' + detail); } }

// A body whose classList records toggles.
var classes = {};
global.document = { body: { classList: { toggle: function (c, on) { if (on) classes[c] = true; else delete classes[c]; } } } };
global.GMCPStructs = { Char: {} };

// Markers avoid line endings, which differ by checkout.
var SET_END = 'classList.toggle("input-queue-full", inputQueueFull);';
var setter = extract('function setInputQueueFull(', SET_END) + SET_END + '\n}\n';
var HANDLER_END = 'setInputQueueFull(q.full === true);';
var handler = extract('"Char.Input.Queue": function() {', HANDLER_END)
    .replace('"Char.Input.Queue": function() {', 'function __Char_Input_Queue() {') + HANDLER_END + '\n}\n';
(0, eval)('var inputQueueFull = false;\n' + setter + handler);

function frame(q) { GMCPStructs.Char.Input = { Queue: q }; __Char_Input_Queue(); }
function full() { return (0, eval)('inputQueueFull'); }

frame({ depth: 10, full: true });
check('a full frame marks the queue full', full() === true, String(full()));
check('a full frame greys the controls', classes['input-queue-full'] === true, JSON.stringify(classes));

frame({ depth: 9, full: false });
check('a free frame clears it', full() === false && !classes['input-queue-full'], JSON.stringify(classes));

frame({ depth: 10, full: true });
frame({});
check('a frame without full reads as not full', full() === false, String(full()));

// The click handler on the automation list refuses rows and chips while
// full, before any of them fires, but leaves the on/off switches working.
var clicks = extract('document.getElementById("auto-list").addEventListener("click"', '// ── Tick enable toggle');
check('rows and chips send nothing while full',
    /if \(inputQueueFull && e\.target\.closest\("\.auto-row, \.auto-chip"\) && !e\.target\.closest\("\.auto-tog"\)\)/.test(clicks),
    clicks);

// A dropped connection starts the next one empty.
var onclose = extract('socket.onclose = function(event) {', 'if (pendingReconnectToken)');
check('a closed socket clears the full state', onclose.indexOf('setInputQueueFull(false)') >= 0, onclose);

// The grey-out covers the touch deck, the trigger and tick rows and the
// macro and alias chips.
['.auto-chip', '.auto-row', '.deck-grid'].forEach(function (sel) {
    check('dashboard.css greys ' + sel, css.indexOf('body.input-queue-full ' + sel) >= 0, sel);
});

// Menu keeps its slot in every state and sends nothing (D3), so the grey-out
// and the click block apply to the other cells, never to the grid itself (a
// child cannot undo its parent's opacity or filter).
var gridRules = css.split('}').filter(function (r) { return /body\.input-queue-full \.deck-grid/.test(r); }).join('}');
check('the deck grey-out skips Menu', /\.deck-grid\s*>\s*:not\(\.deck-menu\)/.test(gridRules), gridRules);
check('no rule greys the whole grid', !/body\.input-queue-full \.deck-grid\s*[,{]/.test(gridRules), gridRules);

// A touch deck press (plan 2b) is a player action like a typed command: it
// sends nothing while the queue is full (the deck's grey-out is only CSS, so
// Enter, Space or a screen reader would still activate a button), and once
// sent it leaves "Leaving..." and snaps the view to the newest output, the
// same code SendData runs (playerActed).
var ACTED_END = 'function SendData(';
var acted = extract('function playerActed(', ACTED_END);
var PRESS_END = 'playerActed(false);';
var press = extract('function sendButtonPress(', PRESS_END) + PRESS_END + '\n    return true;\n}\n';
var log = [];
var probe = {
    leavingGame: true,
    setSessionState: function (s) { log.push('state ' + s); probe.leavingGame = false; },
    term: { scrollToBottom: function () { log.push('scroll'); } },
    SendGMCP: function (pkg, obj) { log.push('gmcp ' + pkg + ' ' + JSON.stringify(obj)); return probe.open; },
    open: true
};
global.__probe = probe;
(0, eval)('var __pressScope = (function () {\n' +
    'var p = __probe;\n' +
    'function setSessionState(s) { p.setSessionState(s); }\n' +
    'function SendGMCP(a, b) { return p.SendGMCP(a, b); }\n' +
    'var term = p.term;\n' +
    acted.replace(/\bleavingGame\b/g, 'p.leavingGame') + press +
    'return { press: sendButtonPress };\n}).call({});');
var sendPress = (0, eval)('__pressScope.press');
frame({ depth: 10, full: true });
log = [];
check('a deck press while the queue is full sends nothing', sendPress('m1') === false && log.length === 0, JSON.stringify(log));
frame({ depth: 0, full: false });
log = [];
var sent = sendPress('m1');
check('a deck press sends only its id as Char.Button.Press, then leaves "Leaving..." and scrolls to the newest output',
    sent === true && JSON.stringify(log) === JSON.stringify(['gmcp Char.Button.Press {"id":"m1"}', 'state connected', 'scroll']), JSON.stringify(log));
probe.open = false; probe.leavingGame = true; log = [];
check('a press on a closed socket changes nothing on the page', sendPress('m1') === false && JSON.stringify(log) === JSON.stringify(['gmcp Char.Button.Press {"id":"m1"}']), JSON.stringify(log));
var sendData = extract('function SendData(', 'function SendGMCP(');
check('SendData runs the same after-send code', /playerActed\(dataToSend === 'quit'\);/.test(sendData) && !/scrollToBottom/.test(sendData) && !/setSessionState/.test(sendData), sendData);

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
