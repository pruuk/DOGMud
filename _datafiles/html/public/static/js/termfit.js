// termfit.js: fit the game text to its panel.
//
// All MUD output is fixed-80 (room text, boxes, tables, the inline minimap), so
// the terminal always keeps at least 80 columns and never wraps them. Wide
// panels grow the font (capped); narrow ones (phones) shrink it to fit exactly
// 80. If that would go below the browser's minimum font size, draw at the
// minimum and scale the whole terminal down instead, so what xterm measures is
// what it draws. Only the game text scales; the command box keeps its own size.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.TermFit = api;        // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var TARGET = 80, MAX_FS = 20, STEP = 0.25;

    // opts: term, fitAddon, host (the element xterm opened in), minFont (the
    // smallest size the browser renders), setScale(s) (told the scale in use).
    function create(opts) {
        var term = opts.term, fitAddon = opts.fitAddon, host = opts.host;
        var minFont = opts.minFont, setScale = opts.setScale;

        var lastW = 0, lastH = 0, lastDpr = 0, lastScale = 1, forcePending = false;

        // Columns at the current font size, without touching the terminal.
        // Null when it cannot measure yet (no cell size).
        function colsAtFont() {
            var d = fitAddon.proposeDimensions();
            return d && !isNaN(d.cols) ? d.cols : null;
        }

        function remember(W, H, scale) {
            lastW = W; lastH = H; lastDpr = window.devicePixelRatio; lastScale = scale;
            forcePending = false;
        }

        // The terminal itself is resized only once, after the font is chosen,
        // and never below 80 columns. Trying a size by resizing to it rewraps
        // every line, and the scrollback limit then drops the oldest for good.
        // force: measure again though the panel is the same size (a font load).
        function fit(force) {
            if (force) forcePending = true;
            var body = host && host.parentElement;
            // While the page loads the frame can be a few pixels wide; fitting
            // 80 columns into that made a terminal tens of thousands of rows
            // tall and crashed the tab. Wait for a real size.
            if (!body || body.clientWidth < 160 || body.clientHeight < 80) return;
            // The display scale changes the letter widths at the same panel size.
            if (!forcePending && body.clientWidth === lastW && body.clientHeight === lastH &&
                window.devicePixelRatio === lastDpr) return;
            try {
                // Only the height changed (the on-screen keyboard): the font
                // depends on the width, so it stays and only the rows change.
                if (!forcePending && body.clientWidth === lastW && window.devicePixelRatio === lastDpr) {
                    if (lastScale !== 1) host.style.height = (body.clientHeight / lastScale) + 'px';
                    fitAddon.fit();
                    remember(lastW, body.clientHeight, lastScale);
                    return;
                }

                // Start from the panel's own size, unscaled.
                host.style.transform = '';
                setScale(1);
                host.style.position = '';
                host.style.width = '';
                host.style.height = '';
                var W = body.clientWidth, H = body.clientHeight;
                var fs = MAX_FS;
                term.setOption('fontSize', fs);
                var cols = colsAtFont();
                if (cols === null) return;
                if (cols < TARGET) {
                    // Cell width grows with font size, so one measurement gives a
                    // close estimate; then step down until 80 columns fit, but
                    // never below the browser minimum.
                    fs = Math.max(minFont,
                        Math.floor((MAX_FS * cols / TARGET) / STEP) * STEP);
                    term.setOption('fontSize', fs);
                    cols = colsAtFont();
                    var guard = 0;
                    while (cols < TARGET && fs - STEP >= minFont && guard++ < 40) {
                        fs -= STEP;
                        term.setOption('fontSize', fs);
                        cols = colsAtFont();
                    }
                }
                if (cols >= TARGET) {
                    fitAddon.fit();
                    remember(W, H, 1);
                    return;
                }

                // Still short of 80 at the browser minimum: lay the terminal out
                // 80 columns wide at that size, then scale it down to the panel.
                var dims = term._core._renderService.dimensions;
                var cellW = dims.actualCellWidth, cellH = dims.actualCellHeight;
                if (!cellW || !cellH) return;
                var extra = W - cols * cellW; // scrollbar and padding fitAddon keeps
                var naturalW = TARGET * cellW + Math.max(0, extra) + 1;
                var s = W / naturalW;
                // Out of the layout flow: the unscaled 80-column box is wider
                // than the panel, and in flow it widened the phone page.
                host.style.position = 'absolute';
                host.style.left = '0';
                host.style.top = '0';
                host.style.width = naturalW + 'px';
                host.style.height = (H / s) + 'px';
                host.style.transformOrigin = 'top left';
                host.style.transform = 'scale(' + s + ')';
                setScale(s);
                fitAddon.fit();
                remember(W, H, s);
            } catch (e) {}
        }

        return { fit: fit };
    }

    return { create: create };
}));
