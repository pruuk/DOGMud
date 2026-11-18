// displayreport.js: tell the server the game window's columns and rows.
//
// The server wraps every line to the columns and pages long replies by the
// rows (Client.Display, handled in modules/gmcp/gmcp.ClientDisplay.go, which
// clamps cols to 30 to 80 and rows to 10 to 200; #842). A size is sent once
// it has held for 250ms, so a window being dragged does not send every step;
// the same size is not sent twice; and a new connection gets the size at
// once, because the server starts every connection at 80 columns.
//
// With the keepTallest option (the page passes it on touch screens only, a
// coarse pointer), the rows sent are the most seen at the current column
// count (owner ruling, #842 phone playtest: pages follow the window with the
// keyboard down). Without it, as on a desktop, the real rows are sent on every
// change, so a window dragged shorter or a zoom shrinks pages at once. With it,
// a phone keyboard coming up shrinks only the rows, so it sends nothing and
// pages stay the size of the open window; a height-only grow is sent. A
// column change (a phone turned, a different window width) or a new
// connection starts over from the rows the window has then. termfit.js
// reports the raw rows; the rule lives here because it is about the column
// count, which this file already tracks, and not about why the rows moved.

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
    // setTimeout/clearTimeout (the window's, or a test clock), and keepTallest
    // (boolean, default false: send the real rows on every change).
    function create(opts) {
        var keepTallest = opts.keepTallest === true;
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        // rows: the window's rows now. most: the most rows seen at these
        // columns, the count sent. restart: the columns changed, so most
        // starts over from the rows once the new size settles (a turn reports
        // the columns before the rows in the same fit).
        var cols = null, rows = null, most = null, restart = false, lastSent = null, timer = null;

        function flush() {
            timer = null;
            if (cols === null) return;
            if (restart || most === null) { most = rows; restart = false; }
            var key = cols + 'x' + most;
            if (key === lastSent) return;
            if (opts.send(cols, most) !== false) lastSent = key;
        }

        function settle() {
            if (timer) clearT(timer);
            timer = setT(flush, SETTLE_MS);
        }

        return {
            // The column count changed; the rows start over, and the size is
            // sent once it settles.
            columns: function (n) {
                if (n !== cols) restart = true;
                cols = n; settle();
            },
            // The row count changed; a new most is sent once it settles.
            rows: function (n) {
                rows = n;
                if (!keepTallest || (!restart && most !== null && n > most)) most = n;
                settle();
            },
            // A new connection: start over and send the current size now.
            connected: function (c, r) {
                if (timer) { clearT(timer); timer = null; }
                lastSent = null;
                cols = c; rows = r; most = r; restart = false;
                flush();
            }
        };
    }

    return { create: create };
}));
