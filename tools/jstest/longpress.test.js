// longpress.test.js: a long touch opens a menu on phones (web client base
// plan 7, owner P2; a stopgap until the touchscreen arc's contextual buttons).
// Run: node tools/jstest/longpress.test.js
'use strict';
var path = require('path');
var LP = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'longpress.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}
var now = 0, timers = [];
function setT(f, ms) { var t = { at: now + ms, f: f, live: true }; timers.push(t); return t; }
function clearT(t) { if (t) { t.live = false; } }
function advance(ms) { now += ms; timers.forEach(function (t) { if (t.live && t.at <= now) { t.live = false; t.f(); } }); }
var fired = [];
var p = LP.createPress({ setTimeout: setT, clearTimeout: clearT, fire: function (x, y) { fired.push([x, y]); } });

check('half a second', LP.DELAY_MS, 500);
check('ten pixels of give', LP.MOVE_PX, 10);
p.down(100, 200); advance(499);
check('not yet at 499ms', fired.length, 0);
advance(1);
check('fires at 500ms where the finger is', fired, [[100, 200]]);
check('the lift after it reports a long press (swallow the click)', p.up(), true);

p.down(10, 10); advance(200);
check('a short tap is not a long press', p.up(), false);
advance(1000);
check('and never fires', fired.length, 1);

p.down(10, 10); p.move(15, 18); advance(600);
check('a small wobble still fires', fired.length, 2);
p.up();

p.down(10, 10); p.move(25, 10); advance(600);
check('a move past 10px cancels', fired.length, 2);
check('and its lift is a plain lift', p.up(), false);

p.down(10, 10); p.cancel(); advance(600);
check('a cancel (scroll, second finger) stops it', fired.length, 2);

// Android Chrome may also send a native contextmenu on a long touch: the
// second open for the same element within the window is ignored.
var opens = 0, clock = 1000, el1 = {}, el2 = {};
function open() { opens++; }
function clk() { return clock; }
LP.once(el1, open, clk);
clock += 300; LP.once(el1, open, clk);
check('a second open of the same element within 700ms is ignored', opens, 1);
LP.once(el2, open, clk);
check('another element opens', opens, 2);
clock += 800; LP.once(el1, open, clk);
check('the same element opens again after the window', opens, 3);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
