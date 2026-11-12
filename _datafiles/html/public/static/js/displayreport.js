// displayreport.js: tell the server how many columns the game window has.
//
// The server wraps every line to this (Client.Display, handled in
// modules/gmcp/gmcp.ClientDisplay.go, which clamps it to 30 to 80). A width
// is sent once it has held for 250ms, so a window being dragged does not
// send every step; the same width is not sent twice; and a new connection
// gets the width at once, because the server starts every connection at 80.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;          // node, for the tests
    } else {
        root.DisplayReport = api;      // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var SETTLE_MS = 250;

    // opts: send(cols) (returns false when it could not send), and
    // setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var lastSent = null, pending = null, timer = null;

        function flush() {
            timer = null;
            if (pending === null || pending === lastSent) return;
            if (opts.send(pending) !== false) lastSent = pending;
        }

        return {
            // The column count changed; send it once it settles.
            columns: function (cols) {
                pending = cols;
                if (timer) clearT(timer);
                timer = setT(flush, SETTLE_MS);
            },
            // A new connection: send the current width now.
            connected: function (cols) {
                if (timer) { clearT(timer); timer = null; }
                lastSent = null;
                pending = cols;
                flush();
            }
        };
    }

    return { create: create };
}));
