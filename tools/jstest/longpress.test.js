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

// attach, with a fake document and the fake timers above.
var handlers = {};
var doc = { addEventListener: function (type, f) { handlers[type] = f; } };
var opened = [];
LP.attach(doc, '.hit', function (x, y, el) { opened.push([x, y, el.id]); }, { setTimeout: setT, clearTimeout: clearT });
var hitEl = { id: 'hit1' };
function target(matching) { return { closest: function () { return matching ? hitEl : null; } }; }
function touch(x, y) { return { clientX: x, clientY: y }; }
function ev(matching, touches) {
    var e = { target: target(matching), touches: touches, prevented: 0 };
    e.preventDefault = function () { e.prevented++; };
    return e;
}

var s = ev(true, [touch(40, 50)]); handlers.touchstart(s); advance(500);
check('attach: a still 500ms touch on a match opens at the finger', opened, [[40, 50, 'hit1']]);
var u = ev(true, []); handlers.touchend(u);
check('attach: its touchend prevents the click', u.prevented, 1);

s = ev(true, [touch(1, 1)]); handlers.touchstart(s); advance(200);
u = ev(true, []); handlers.touchend(u); advance(1000);
check('attach: a short touch does not open', opened.length, 1);
check('attach: and its touchend is not prevented', u.prevented, 0);

handlers.touchstart(ev(true, [touch(1, 1)]));
handlers.touchstart(ev(true, [touch(1, 1), touch(9, 9)])); advance(600);
check('attach: a second finger cancels', opened.length, 1);
handlers.touchend(ev(true, []));

handlers.touchstart(ev(true, [touch(1, 1)]));
handlers.scroll({}); advance(600);
check('attach: a scroll cancels', opened.length, 1);
handlers.touchend(ev(true, []));

handlers.touchstart(ev(false, [touch(1, 1)])); advance(600);
u = ev(false, []); handlers.touchend(u);
check('attach: a touch off the selector does nothing', [opened.length, u.prevented], [1, 0]);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
