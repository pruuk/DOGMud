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
['.auto-chip', '.auto-row', '#touch-deck'].forEach(function (sel) {
    check('dashboard.css greys ' + sel, css.indexOf('body.input-queue-full ' + sel) >= 0, sel);
});

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
