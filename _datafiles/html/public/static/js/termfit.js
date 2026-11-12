// termfit.js: fit the game window to its panel.
//
// The game text is one size: the --text token (16px, 14px on an upright
// phone), never below the smallest size the browser renders, and nothing
// here ever changes it to make text fit (owner ruling 2026-11-12). The
// layout gives the game window the width of 80 columns (widthFor80) when
// the browser has room, so the terminal has 80 columns; only when the
// browser is narrower than that does the panel, and so the column count,
// follow it. The server wraps to the column count (displayreport.js). A
// height-only change, such as the on-screen keyboard, changes only the rows.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.TermFit = api;        // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var MAX_COLS = 80;

    // opts: term, fitAddon, host (the element xterm opened in), minFont (the
    // smallest size the browser renders), fontSize() (the --text token in px),
    // onCols(cols) (told when the column count changes), onGameWidth(px)
    // (told when the width 80 columns need changes: a font load, zoom,
    // another monitor, an upright phone).
    function create(opts) {
        var term = opts.term, fitAddon = opts.fitAddon, host = opts.host;
        var lastW = 0, lastH = 0, lastDpr = 0, lastFont = 0, lastW80 = 0, forcePending = false;

        // The terminal is resized at most once per fit, straight to the
        // answer. Trying a size by resizing to it rewraps every line, and the
        // scrollback limit then drops the oldest for good.
        // force: measure again though the panel is the same size (a font load).
        function fit(force) {
            if (force) forcePending = true;
            var body = host && host.parentElement;
            // While the page loads the frame can be a few pixels wide; wait
            // for a real size (a tiny frame once made a terminal tens of
            // thousands of rows tall and crashed the tab).
            // A panel already fitted at this width may go much shorter: a
            // phone on its side with the keyboard up.
            if (!body || body.clientWidth < 160 || body.clientHeight < 1 ||
                (body.clientHeight < 80 && body.clientWidth !== lastW)) return;
            var colsChanged = false, cols;
            try {
                var font = Math.max(opts.fontSize(), opts.minFont);
                if (!forcePending && body.clientWidth === lastW && body.clientHeight === lastH &&
                    window.devicePixelRatio === lastDpr && font === lastFont) return;
                if (font !== lastFont) term.setOption('fontSize', font);
                // Measure the letters again unless only the height changed
                // (the keyboard). A font load and a display-scale change (zoom,
                // another monitor) change the cell size at the same panel size,
                // and xterm's own update can arrive after this fit; a stale cell
                // gave too many columns, or rows hidden under the panel after a
                // font load (2026-11-07). Private xterm 4.19 internals.
                var heightOnly = !forcePending && body.clientWidth === lastW &&
                    window.devicePixelRatio === lastDpr && font === lastFont;
                if (!heightOnly && term._core && term._core._charSizeService) {
                    term._core._charSizeService.measure();
                }
                // A zoom or another monitor changes the display scale. The
                // canvas renderer works its cell size out from the scale and
                // updates it only from its own watcher, which can run after
                // this fit; update it now so the fit addon counts the rows
                // against the new cell. Private xterm 4.19 internals.
                if (window.devicePixelRatio !== lastDpr && term._core && term._core._renderService &&
                    term._core._renderService.onDevicePixelRatioChange) {
                    term._core._renderService.onDevicePixelRatioChange();
                }
                // Tell the layout the width 80 columns now need BEFORE the
                // columns are counted, so it can widen the game window first.
                // The count reads the host's computed width, which lays the
                // page out again, so it sees the new width in this same fit:
                // a font load never shows fewer than 80 columns for a moment.
                // On its own, so a listener that throws cannot stop the fit.
                try {
                    var w80 = widthFor80();
                    if (w80 && w80 !== lastW80) {
                        lastW80 = w80;
                        if (opts.onGameWidth) opts.onGameWidth(w80);   // the page keeps the last one
                    }
                } catch (e) {}
                // A terminal that cannot measure yet is not fitted, and the
                // fit is not remembered, so the next one tries again.
                // The fit addon gives the rows. Its columns are not used: it
                // divides by xterm's cell width, which is the canvas width
                // over the CURRENT column count, so its answer drifts with
                // that count: a window exactly 80 columns wide fitted from 41
                // columns got 79, and a keyboard move at one width could
                // flip between two counts (review 2026-11-12).
                var d = fitAddon.proposeDimensions();
                if (!d || isNaN(d.rows)) return;
                if (heightOnly) {
                    cols = term.cols;            // the keyboard changes only the rows
                } else {
                    cols = colsThatFit();
                    if (!cols) return;
                }
                colsChanged = cols !== term.cols;
                if (colsChanged || d.rows !== term.rows) {
                    term._core._renderService.clear();
                    term.resize(cols, d.rows);
                }
                lastW = body.clientWidth; lastH = body.clientHeight;
                lastDpr = window.devicePixelRatio; lastFont = font; forcePending = false;
            } catch (e) { return; }
            // The fit is remembered; the column listener is told on its own,
            // so it cannot undo the fit by throwing.
            try {
                if (colsChanged && opts.onCols) opts.onCols(cols);
            } catch (e) {}
        }

        // The columns that fit the host, at most 80, with the same arithmetic
        // as widthFor80, so a host widthFor80 wide holds exactly 80. The
        // width is the one xterm-addon-fit measures: the computed width of
        // the element xterm opened in, less the terminal element's side
        // padding and the scrollbar (none without scrollback). Null until
        // xterm has measured its letters. Private xterm 4.19 fields.
        function colsThatFit() {
            var cell = cellWidth();
            var el = term.element, parent = el && el.parentElement;
            if (!cell || !parent) return null;
            var ps = window.getComputedStyle(parent), es = window.getComputedStyle(el);
            var width = Math.max(0, parseInt(ps.getPropertyValue('width'))) -
                (parseInt(es.getPropertyValue('padding-right')) + parseInt(es.getPropertyValue('padding-left'))) -
                scrollBarWidth();
            if (isNaN(width)) return null;
            // The small allowance keeps a width of exactly 80 cells from
            // counting 79.99999 in floating point.
            return Math.min(MAX_COLS, Math.max(2, Math.floor(width / cell + 1e-6)));
        }

        // One cell in CSS pixels, the way the canvas renderer sizes it:
        // floor(letter * display scale) device pixels. Null before xterm has
        // measured its letters.
        function cellWidth() {
            var cs = term._core && term._core._charSizeService;
            if (!cs || !cs.width) return null;
            var dpr = window.devicePixelRatio || 1;
            var scaled = Math.floor(cs.width * dpr);
            return scaled ? scaled / dpr : null;
        }

        // xterm-addon-fit leaves no room for a scrollbar when there is no
        // scrollback.
        function scrollBarWidth() {
            var core = term._core;
            if (term.options && term.options.scrollback === 0) return 0;
            return core && core.viewport ? core.viewport.scrollBarWidth : 0;
        }

        // The pixel width the terminal needs for exactly 80 columns at the
        // current font: 80 real cells plus xterm's scrollbar. The layout gives
        // the game window this width (plus its own border) whenever the
        // browser has room, so the terminal there always has 80 columns.
        // Null until xterm has measured its cells. Private xterm 4.19 fields,
        // the same ones xterm-addon-fit reads.
        // Worked from the letter width and the display scale, the way the
        // canvas renderer sizes its cells (scaled cell = floor(letter * dpr)),
        // not from dimensions.actualCellWidth, which is canvasWidth / cols and
        // drifts by a pixel when the terminal has fewer than 80 columns.
        function widthFor80() {
            var cs = term._core && term._core._charSizeService;
            if (!cs || !cs.width) return null;
            var dpr = window.devicePixelRatio || 1;
            return Math.ceil(Math.floor(cs.width * dpr) * MAX_COLS / dpr + scrollBarWidth());
        }

        return { fit: fit, cols: function () { return term.cols; }, widthFor80: widthFor80 };
    }

    return { create: create };
}));
