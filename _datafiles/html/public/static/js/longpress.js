// longpress.js: a long touch opens a context menu on phones and tablets (web
// client base plan 7, owner P2). A stopgap: the touchscreen arc replaces it
// with contextual buttons (P3, #780). Mouse users keep right-click.
// createPress is the timing, pure and tested (tools/jstest/longpress.test.js);
// attach wires it to touch events for elements matching a selector.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.LongPress = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    var DELAY_MS = 500;   // common platform value (Android 400 to 500ms)
    var MOVE_PX = 10;     // a finger that strays further is scrolling

    // opts: { setTimeout, clearTimeout, fire(x, y) }
    function createPress(opts) {
        var timer = null, sx = 0, sy = 0, did = false;
        function clear() { if (timer) { opts.clearTimeout(timer); timer = null; } }
        return {
            down: function (x, y) {
                clear(); did = false; sx = x; sy = y;
                timer = opts.setTimeout(function () { timer = null; did = true; opts.fire(sx, sy); }, DELAY_MS);
            },
            move: function (x, y) {
                if (timer && (Math.abs(x - sx) > MOVE_PX || Math.abs(y - sy) > MOVE_PX)) { clear(); }
            },
            // true when this press fired: the caller cancels the click that follows.
            up: function () { clear(); var d = did; did = false; return d; },
            cancel: function () { clear(); did = false; }
        };
    }

    // A long touch on an element matching selector calls open(x, y, el).
    // opts (optional, for tests): { setTimeout, clearTimeout }.
    function attach(doc, selector, open, opts) {
        var el = null;
        var st = (opts && opts.setTimeout) || function (f, ms) { return setTimeout(f, ms); };
        var ct = (opts && opts.clearTimeout) || function (t) { clearTimeout(t); };
        var press = createPress({
            setTimeout: st,
            clearTimeout: ct,
            fire: function (x, y) { if (el) { open(x, y, el); } }
        });
        doc.addEventListener('touchstart', function (e) {
            if (!e.touches || e.touches.length !== 1) { press.cancel(); el = null; return; }
            el = e.target && e.target.closest ? e.target.closest(selector) : null;
            if (el) { press.down(e.touches[0].clientX, e.touches[0].clientY); }
        }, { passive: true });
        doc.addEventListener('touchmove', function (e) {
            if (e.touches && e.touches[0]) { press.move(e.touches[0].clientX, e.touches[0].clientY); }
        }, { passive: true });
        doc.addEventListener('touchend', function (e) {
            if (press.up() && el) { e.preventDefault(); }   // no click after a long press
            el = null;
        }, { passive: false });
        doc.addEventListener('touchcancel', function () { press.cancel(); el = null; });
        doc.addEventListener('scroll', function () { press.cancel(); }, true);
    }

    return { DELAY_MS: DELAY_MS, MOVE_PX: MOVE_PX, createPress: createPress, attach: attach };
}));
