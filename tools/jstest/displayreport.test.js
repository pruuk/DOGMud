// displayreport.test.js: the web client tells the server its columns and rows
// once it settles (spec section 1: 250ms, only on a change, again on connect).
// The rows are the most seen at the current columns: a height-only shrink
// (the phone keyboard) is not sent; a column change or a connect starts over.
//
// Run:  node tools/jstest/displayreport.test.js
'use strict';

var path = require('path');
var DisplayReport = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'displayreport.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// A fake clock: timers run only when the test advances time.
function clock() {
    var now = 0, timers = [];
    return {
        setTimeout: function (f, ms) { var t = { at: now + ms, f: f }; timers.push(t); return t; },
        clearTimeout: function (t) { timers = timers.filter(function (x) { return x !== t; }); },
        advance: function (ms) {
            now += ms;
            var due = timers.filter(function (t) { return t.at <= now; });
            timers = timers.filter(function (t) { return t.at > now; });
            due.forEach(function (t) { t.f(); });
        }
    };
}

var c = clock(), sent = [], open = true;
var r = DisplayReport.create({
    send: function (cols, rows) { if (!open) return false; sent.push(cols + 'x' + rows); return true; },
    setTimeout: c.setTimeout, clearTimeout: c.clearTimeout,
    keepTallest: true
});

r.connected(80, 30);
check('connect sends the size at once', sent.join() === '80x30', sent.join());

r.columns(60); c.advance(100); r.columns(50); c.advance(100); r.columns(42);
c.advance(249);
check('nothing is sent while the size is still moving', sent.join() === '80x30', sent.join());
c.advance(1);
check('the settled width is sent once', sent.join() === '80x30,42x30', sent.join());

r.columns(42); c.advance(300);
check('the same size is not sent again', sent.join() === '80x30,42x30', sent.join());

r.rows(18); c.advance(300);
check('a height-only shrink is not sent', sent.join() === '80x30,42x30', sent.join());
r.rows(34); c.advance(300);
check('a height-only grow is sent', sent.join() === '80x30,42x30,42x34', sent.join());

open = false; r.columns(36); c.advance(300);
check('a closed socket sends nothing', sent.join() === '80x30,42x30,42x34', sent.join());
open = true; r.connected(36, 18);
check('a reconnect sends the size though it was sent before', sent.join() === '80x30,42x30,42x34,36x18', sent.join());

// A phone (owner ruling, #842 playtest): pages follow the window with the
// keyboard down, so the keyboard coming up (rows only) sends nothing.
var pc = clock(), ps = [];
var phone = DisplayReport.create({
    send: function (cols, rows) { ps.push(cols + 'x' + rows); return true; },
    setTimeout: pc.setTimeout, clearTimeout: pc.clearTimeout,
    keepTallest: true
});
phone.connected(36, 16);
phone.rows(6); pc.advance(300);
check('the keyboard opening sends nothing new', ps.join() === '36x16', ps.join());
phone.rows(16); pc.advance(300);
check('the keyboard closing again sends nothing new', ps.join() === '36x16', ps.join());
// Turned on its side: termfit reports the columns, then the rows, in one fit.
phone.columns(60); phone.rows(9); pc.advance(300);
check('a column change starts over, so fewer rows at the new columns are sent', ps.join() === '36x16,60x9', ps.join());
phone.rows(5); pc.advance(300);
check('the keyboard on the side view sends nothing new', ps.join() === '36x16,60x9', ps.join());
phone.connected(60, 5);
check('a reconnect sends the current size', ps.join() === '36x16,60x9,60x5', ps.join());
phone.rows(9); pc.advance(300);
check('after a reconnect a grow is sent', ps.join() === '36x16,60x9,60x5,60x9', ps.join());

// A desktop (keepTallest false, the default): the real rows are sent on every
// change, so a window dragged shorter, zoom or devtools shrinks pages at once.
var dc = clock(), ds = [];
var desk = DisplayReport.create({
    send: function (cols, rows) { ds.push(cols + 'x' + rows); return true; },
    setTimeout: dc.setTimeout, clearTimeout: dc.clearTimeout,
    keepTallest: false
});
desk.connected(100, 40);
desk.rows(20); dc.advance(300);
check('a desktop sends a height-only shrink', ds.join() === '100x40,100x20', ds.join());
desk.rows(30); dc.advance(300);
check('a desktop sends a height-only grow', ds.join() === '100x40,100x20,100x30', ds.join());
desk.columns(90); desk.rows(25); dc.advance(300);
check('a desktop sends a column and row change', ds.join() === '100x40,100x20,100x30,90x25', ds.join());
var dd = clock(), dds = [];
var deflt = DisplayReport.create({
    send: function (cols, rows) { dds.push(cols + 'x' + rows); return true; },
    setTimeout: dd.setTimeout, clearTimeout: dd.clearTimeout
});
deflt.connected(100, 40); deflt.rows(20); dd.advance(300);
check('keepTallest defaults to false', dds.join() === '100x40,100x20', dds.join());

process.exit(failures ? 1 : 0);
