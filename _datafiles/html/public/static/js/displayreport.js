// displayreport.js: tell the server the game window's columns and rows.
//
// The server wraps every line to the columns and pages long replies by the
// rows (Client.Display, handled in modules/gmcp/gmcp.ClientDisplay.go, which
// clamps cols to 30 to 80 and rows to 10 to 200; #842). A size is sent once
// it has held for 250ms, so a window being dragged does not send every step;
// the same size is not sent twice; and a new connection gets the size at
// once, because the server starts every connection at 80 columns.

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

    // opts: send(cols, rows) (returns false when it could not send), and
    // setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var cols = null, rows = null, lastSent = null, timer = null;

        function flush() {
            timer = null;
            if (cols === null) return;
            var key = cols + 'x' + rows;
            if (key === lastSent) return;
            if (opts.send(cols, rows) !== false) lastSent = key;
        }

        function settle() {
            if (timer) clearT(timer);
            timer = setT(flush, SETTLE_MS);
        }

        return {
            // The column count changed; send it once it settles.
            columns: function (n) { cols = n; settle(); },
            // The row count changed; send it once it settles.
            rows: function (n) { rows = n; settle(); },
            // A new connection: send the current size now.
            connected: function (c, r) {
                if (timer) { clearT(timer); timer = null; }
                lastSent = null;
                cols = c; rows = r;
                flush();
            }
        };
    }

    return { create: create };
}));
