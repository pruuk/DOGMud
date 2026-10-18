// feedscroll.test.js: the Game window's text cannot be scrolled out of view
// sideways (playtest 2026-10-17, "the middle panel's text disappears").
//
// Run:  node tools/jstest/feedscroll.test.js
//
// Covers static/js/feedscroll.js and checks the page and stylesheet still
// wire it up. Dependency-free and exits non-zero on failure.

'use strict';

var fs = require('fs');
var path = require('path');

var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var FeedScroll = require(path.join(PUBLIC, 'static', 'js', 'feedscroll.js'));

var failures = 0, checks = 0;
function check(name, ok, detail) {
    checks++;
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + (detail || ''));
}

// --- element stub ----------------------------------------------------------
function el() {
    var listeners = {};
    return {
        scrollLeft: 0, scrollTop: 0, listeners: listeners,
        addEventListener: function (type, fn, opts) {
            (listeners[type] = listeners[type] || []).push({ fn: fn, opts: opts });
        },
        fire: function (type, evt) {
            (listeners[type] || []).forEach(function (l) { l.fn(evt || {}); });
        }
    };
}
function wheel(dx, dy) {
    return { deltaX: dx, deltaY: dy, prevented: false,
             preventDefault: function () { this.prevented = true; } };
}

// --- isSidewaysWheel -------------------------------------------------------
check('vertical wheel is not sideways', !FeedScroll.isSidewaysWheel(wheel(0, 120)));
check('sideways wheel is sideways', FeedScroll.isSidewaysWheel(wheel(120, 0)));
check('mostly-vertical diagonal is left alone', !FeedScroll.isSidewaysWheel(wheel(10, 100)));
check('mostly-sideways diagonal is sideways', FeedScroll.isSidewaysWheel(wheel(-100, 10)));
check('no motion is not sideways', !FeedScroll.isSidewaysWheel(wheel(0, 0)));

// --- install ---------------------------------------------------------------
var terminal = el(), viewport = el(), body = el(), panel = el();
body.scrollLeft = 40; // already shifted before install
FeedScroll.install({ terminal: terminal, viewport: viewport, body: body, panel: panel });

check('install snaps an already-shifted body back', body.scrollLeft === 0, 'scrollLeft ' + body.scrollLeft);

var wl = (terminal.listeners.wheel || [])[0];
check('wheel listener is registered non-passive', !!wl && wl.opts && wl.opts.passive === false,
      JSON.stringify(wl && wl.opts));
var side = wheel(120, 0), down = wheel(0, 120);
terminal.fire('wheel', side);
terminal.fire('wheel', down);
check('a sideways wheel over the feed is cancelled', side.prevented);
check('a vertical wheel is passed through to xterm', !down.prevented);

body.scrollLeft = 300; body.scrollTop = 25; body.fire('scroll');
check('a hidden-overflow body shifted sideways snaps back', body.scrollLeft === 0, 'scrollLeft ' + body.scrollLeft);
check('a hidden-overflow body shifted down snaps back', body.scrollTop === 0, 'scrollTop ' + body.scrollTop);

panel.scrollLeft = 12; panel.fire('scroll');
check('the panel itself snaps back', panel.scrollLeft === 0);

viewport.scrollLeft = 50; viewport.scrollTop = 900; viewport.fire('scroll');
check('the xterm viewport loses its sideways offset', viewport.scrollLeft === 0);
check('the xterm viewport keeps its scrollback position', viewport.scrollTop === 900, 'scrollTop ' + viewport.scrollTop);

check('install tolerates missing elements', (function () {
    try { FeedScroll.install({}); FeedScroll.install(); return true; } catch (e) { return false; }
}()));

// --- wiring ----------------------------------------------------------------
var page = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
var css = fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8');
check('the page loads feedscroll.js', page.indexOf('/static/js/feedscroll.js"') >= 0);
var openAt = page.indexOf("term.open(document.getElementById('terminal'));");
var installAt = page.indexOf('FeedScroll.install(');
check('the page installs the guard after term.open', openAt >= 0 && installAt > openAt,
      'term.open at ' + openAt + ', install at ' + installAt);
check('the stylesheet stops the viewport scrolling sideways',
      /#panel-feed #terminal \.xterm-viewport\s*\{\s*overflow-x:\s*hidden;/.test(css));

console.log((failures ? failures + ' of ' : 'all ') + checks + ' checks ' + (failures ? 'FAILED' : 'passed'));
process.exit(failures ? 1 : 0);
