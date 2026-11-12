// displayreport.test.js: the web client tells the server its column count
// once it settles (spec section 1: 250ms, only on a change, again on connect).
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
    send: function (cols) { if (!open) return false; sent.push(cols); return true; },
    setTimeout: c.setTimeout, clearTimeout: c.clearTimeout
});

r.connected(80);
check('connect sends the width at once', sent.join() === '80', sent.join());

r.columns(60); c.advance(100); r.columns(50); c.advance(100); r.columns(42);
c.advance(249);
check('nothing is sent while the size is still moving', sent.join() === '80', sent.join());
c.advance(1);
check('the settled width is sent once', sent.join() === '80,42', sent.join());

r.columns(42); c.advance(300);
check('the same width is not sent again', sent.join() === '80,42', sent.join());

open = false; r.columns(36); c.advance(300);
check('a closed socket sends nothing', sent.join() === '80,42', sent.join());
open = true; r.connected(36);
check('a reconnect sends the width though it was sent before', sent.join() === '80,42,36', sent.join());

process.exit(failures ? 1 : 0);
