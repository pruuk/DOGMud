// termfit.test.js: the game text keeps its size; the columns follow the panel,
// at most 80, and a fit keeps the scrollback.
//
// Run:  node tools/jstest/termfit.test.js
//
// Uses the real pinned xterm buffer, so reflow and trimming are the real thing.
// Only the page around it is faked: a letter is 0.6em wide and 1.2em tall, the
// fake _charSizeService holds the letter size xterm last measured (the height
// rounded up, as the page's own measure does), the fake renderer works its
// cell out with xterm 4.19's canvas arithmetic (_updateDimensions: device-pixel
// floor, and actualCellWidth = canvas width over the CURRENT column count),
// and the fake fit addon proposes with xterm-addon-fit's arithmetic (computed
// host width less padding and scrollbar, at least 2 columns and 1 row).
'use strict';

var path = require('path');
var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');

global.self = global;
global.window = global;
global.devicePixelRatio = 1;
global.document = {
    createElement: function () { return { getContext: function () { return null; }, style: {} }; },
    addEventListener: function () {},
    removeEventListener: function () {}
};
// Fake elements carry their computed style as a css map of functions.
global.getComputedStyle = function (el) {
    return {
        getPropertyValue: function (p) { return el.css && el.css[p] ? el.css[p]() : '0px'; }
    };
};
global.requestAnimationFrame = function (f) { return setTimeout(f, 0); };
global.cancelAnimationFrame = clearTimeout;

var X = require(path.join(JS, 'xterm.4.19.0.js'));
var TermFit = require(path.join(JS, 'termfit.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// A page: a panel of W x H holding the terminal host.
// opts.text is the --text token (page.text, changeable), opts.minFont the
// browser minimum, opts.onCols the column-count listener. page.glyph is the
// letter width in em; changing it stands for a font load (the letters change,
// xterm has not measured them yet). opts.cols starts the terminal at that
// many columns; opts.unmeasured starts it before xterm could measure.
var LINE_HEIGHT = 1.2;        // the page's lineHeight option
var SCROLLBAR = 15;
function makePage(W, H, opts) {
    opts = opts || {};
    var real = new X.Terminal({ cols: opts.cols || 80, rows: 40, scrollback: 1000 });
    var page = {
        real: real, text: opts.text, fontSize: 20, glyph: 0.6, unmeasured: !!opts.unmeasured,
        narrowest: real.cols, resizes: 0, measures: 0, dprChanges: 0, addonFits: 0,
        gameWidths: []
    };
    // The host fills the panel body, as #terminal does (width/height 100%).
    var panel = { clientWidth: W, clientHeight: H };
    var host = {
        style: {}, parentElement: panel,
        css: {
            width: function () { return panel.clientWidth + 'px'; },
            height: function () { return panel.clientHeight + 'px'; }
        }
    };
    page.host = host;
    page.panel = panel;

    // xterm's CharSizeService: the letter size as last measured. xterm
    // measures on open and on its own when the fontSize option changes; those
    // are not counted. page.measures counts only measure() calls from termfit.
    var charSize = {
        _w: 0, _h: 0,
        get width() { return page.unmeasured ? 0 : this._w; },
        get height() { return page.unmeasured ? 0 : this._h; },
        measure: function () { page.measures++; take(); }
    };
    // The canvas renderer's dimensions (xterm 4.19 _updateDimensions, with
    // letterSpacing 0). Updated when the letters are measured, on a resize
    // and on the renderer's display-scale update, never on its own.
    var dims = { actualCellWidth: 0, actualCellHeight: 0 };
    function updateDims() {
        if (!charSize.width || !charSize.height) return;   // !hasValidSize: left as they were
        var dpr = window.devicePixelRatio;
        var scaledCharWidth = Math.floor(charSize.width * dpr);
        var scaledCharHeight = Math.ceil(charSize.height * dpr);
        var scaledCellHeight = Math.floor(scaledCharHeight * LINE_HEIGHT);
        var canvasWidth = Math.round(real.cols * scaledCharWidth / dpr);
        var canvasHeight = Math.round(real.rows * scaledCellHeight / dpr);
        dims.actualCellWidth = canvasWidth / real.cols;
        dims.actualCellHeight = canvasHeight / real.rows;
    }
    function take() {
        charSize._w = page.fontSize * page.glyph;
        charSize._h = Math.ceil(page.fontSize * 1.2 - 0.01);   // the page's measure rounds up
        updateDims();
    }
    take();
    page.charSize = charSize;
    page.dims = dims;

    var element = { parentElement: host, css: {} };   // .xterm, no padding
    var term = {
        element: element,
        options: { scrollback: 1000 },
        get cols() { return real.cols; },
        get rows() { return real.rows; },
        setOption: function (k, v) { if (k === 'fontSize') { page.fontSize = v; take(); } },
        resize: function (c, r) {
            page.resizes++;
            page.narrowest = Math.min(page.narrowest, c);
            real.resize(c, r);
            updateDims();
        },
        _core: {
            _charSizeService: charSize,
            viewport: { scrollBarWidth: SCROLLBAR },
            _renderService: {
                dimensions: dims,
                clear: function () {},
                onDevicePixelRatioChange: function () { page.dprChanges++; take(); }
            }
        }
    };
    // xterm-addon-fit.js proposeDimensions, line for line.
    var fitAddon = {
        proposeDimensions: function () {
            if (!term.element || !term.element.parentElement) return undefined;
            var core = term._core, t = core._renderService.dimensions;
            if (t.actualCellWidth === 0 || t.actualCellHeight === 0) return undefined;
            var r = term.options.scrollback === 0 ? 0 : core.viewport.scrollBarWidth;
            var i = window.getComputedStyle(term.element.parentElement);
            var o = parseInt(i.getPropertyValue('height'));
            var s = Math.max(0, parseInt(i.getPropertyValue('width')));
            var n = window.getComputedStyle(term.element);
            var l = o - (parseInt(n.getPropertyValue('padding-top')) + parseInt(n.getPropertyValue('padding-bottom')));
            var a = s - (parseInt(n.getPropertyValue('padding-right')) + parseInt(n.getPropertyValue('padding-left'))) - r;
            return {
                cols: Math.max(2, Math.floor(a / t.actualCellWidth)),
                rows: Math.max(1, Math.floor(l / t.actualCellHeight))
            };
        },
        // Must not be used: it resizes to the whole panel, over 80 on a desktop.
        fit: function () { page.addonFits++; }
    };
    page.fitter = TermFit.create({
        term: term, fitAddon: fitAddon, host: host, minFont: opts.minFont,
        fontSize: opts.fontSize || function () { return page.text; },
        onCols: opts.onCols,
        onGameWidth: opts.onGameWidth || function (px) { page.gameWidths.push(px); }
    });
    return page;
}

function fill(page, then) {
    var s = '';
    for (var i = 0; i < 1100; i++) {
        s += ('L' + String(i).padStart(4, '0') + ' ').padEnd(78, 'x') + '\r\n';
    }
    page.real.write(s, then);
}

function nothingScaled(page) {
    var st = page.host.style;
    return !st.transform && !st.position && !st.width && !st.height;
}

// What widthFor80 must be: 80 cells the way the canvas renderer sizes them
// (floor(letter * dpr) device pixels each), back in CSS pixels, plus the
// scrollbar.
function expected80(letter, dpr) {
    return Math.ceil(Math.floor(letter * dpr) * 80 / dpr + SCROLLBAR);
}

// The columns a W-wide panel must get: whole renderer cells after the
// scrollbar, at most 80, at least 2.
function fits(W, letter, dpr) {
    var cell = Math.floor(letter * (dpr || 1)) / (dpr || 1);
    return Math.min(80, Math.max(2, Math.floor((W - SCROLLBAR) / cell + 1e-6)));
}

function syncChecks() {
    // A phone: the font stays the token size and the columns follow the panel.
    var phone = makePage(360, 500, { text: 14, minFont: 1 });
    phone.fitter.fit();
    check('a phone keeps the 14px text', phone.fontSize === 14, 'font ' + phone.fontSize);
    check('a phone gets the columns that fit', phone.real.cols === fits(360, 14 * 0.6), 'cols ' + phone.real.cols);
    // Never resized to a trial width: one fit, one resize, straight to the answer.
    check('a phone fit resizes exactly once', phone.resizes === 1, 'resizes ' + phone.resizes);
    check('a phone fit never lays out narrower than its answer, even for a moment',
        phone.narrowest === phone.real.cols, 'narrowest ' + phone.narrowest + ' cols ' + phone.real.cols);
    check('a phone is not scaled', nothingScaled(phone), 'style ' + JSON.stringify(phone.host.style));

    // A desktop: 16px, never more than 80 columns.
    var wide = makePage(1600, 900, { text: 16, minFont: 1 });
    wide.fitter.fit();
    check('a wide panel keeps the 16px text', wide.fontSize === 16, 'font ' + wide.fontSize);
    check('a wide panel stops at 80 columns', wide.real.cols === 80, 'cols ' + wide.real.cols);
    check('a wide panel is not scaled', nothingScaled(wide), 'style ' + JSON.stringify(wide.host.style));
    check('the fit addon never sizes the terminal', wide.addonFits === 0 && phone.addonFits === 0,
        'addon fits ' + wide.addonFits + ' ' + phone.addonFits);

    // A raised browser minimum wins over the token.
    var raised = makePage(360, 500, { text: 14, minFont: 20 });
    raised.fitter.fit();
    check('a raised minimum is used', raised.fontSize === 20, 'font ' + raised.fontSize);
    check('a raised minimum gets the columns that fit at that size',
        raised.real.cols === fits(360, 20 * 0.6), 'cols ' + raised.real.cols);
    check('a raised minimum never lays out narrower than its answer',
        raised.narrowest === raised.real.cols, 'narrowest ' + raised.narrowest);
    check('nothing is scaled', !raised.host.style.transform, 'transform ' + raised.host.style.transform);
    check('a raised minimum leaves the host its own size', nothingScaled(raised),
        'style ' + JSON.stringify(raised.host.style));

    // Back to a panel with room: 80 columns at the same size, still unscaled.
    raised.panel.clientWidth = 1600;
    raised.fitter.fit();
    check('a widened panel goes back to 80 columns', raised.real.cols === 80 && raised.fontSize === 20,
        'cols ' + raised.real.cols + ' font ' + raised.fontSize);
    check('a widened panel is not scaled', nothingScaled(raised), 'style ' + JSON.stringify(raised.host.style));

    // While the page loads the frame can be a few pixels wide.
    var tiny = makePage(20, 20, { text: 16, minFont: 1 });
    tiny.fitter.fit();
    check('a tiny frame is left alone', tiny.resizes === 0, 'resizes ' + tiny.resizes);

    // The keyboard: a height-only change keeps the columns and reports nothing.
    var reported = [];
    var kb = makePage(360, 500, { text: 14, minFont: 1, onCols: function (c) { reported.push(c); } });
    kb.fitter.fit();
    check('the first fit reports the columns', reported.length === 1 && reported[0] === kb.real.cols,
        'reports ' + reported.join(','));
    var cols = kb.real.cols, n = reported.length, rows = kb.real.rows;
    var measures = kb.measures, widths = kb.gameWidths.length, resizes = kb.resizes;
    kb.panel.clientHeight = 300; kb.fitter.fit();
    check('the keyboard keeps the columns', kb.real.cols === cols, 'cols ' + kb.real.cols);
    check('the keyboard reports nothing', reported.length === n, 'reports ' + reported.join(','));
    check('the keyboard still fits the rows', kb.real.rows < rows, 'rows ' + kb.real.rows + ' was ' + rows);
    check('the keyboard resizes exactly once', kb.resizes === resizes + 1, 'resizes ' + kb.resizes);
    check('the keyboard does not measure the letters', kb.measures === measures, 'measures ' + kb.measures);
    check('the keyboard keeps the font', kb.fontSize === 14, 'font ' + kb.fontSize);
    check('the keyboard does not report a game width', kb.gameWidths.length === widths,
        'widths ' + kb.gameWidths.join(','));

    // A height-only refit leaves the columns alone, even if the letters
    // would measure differently by now (a font load not yet noticed).
    var held = makePage(360, 700, { text: 14, minFont: 1 });
    held.fitter.fit();
    var heldCols = held.real.cols;
    held.glyph = 0.5;
    held.panel.clientHeight = 500;
    held.fitter.fit();
    check('a height-only refit keeps the columns', held.real.cols === heldCols, 'cols ' + held.real.cols + ' was ' + heldCols);
    check('a height-only refit leaves the host its own size', nothingScaled(held),
        'style ' + JSON.stringify(held.host.style));

    // Landscape with the keyboard up leaves a very short panel; the
    // terminal must still shrink or the newest lines hang off the bottom.
    var flat = makePage(740, 300, { text: 16, minFont: 1 });
    flat.fitter.fit();
    flat.panel.clientHeight = 40;
    flat.fitter.fit();
    check('a very short panel still fits its rows', flat.real.rows <= 3, 'rows ' + flat.real.rows);
    // A panel under 80px tall at a width not yet fitted is a page still
    // laying out: leave it alone.
    var flatResizes = flat.resizes, flatCols = flat.real.cols;
    flat.panel.clientWidth = 500;
    flat.fitter.fit();
    check('a very short panel at a new width is not fitted',
        flat.resizes === flatResizes && flat.real.cols === flatCols,
        'resizes ' + flat.resizes + ' cols ' + flat.real.cols);

    // A same-size refit does no work; a forced one (font load) measures again.
    var same = makePage(360, 500, { text: 14, minFont: 1 });
    same.fitter.fit();
    var sr = same.resizes, sm = same.measures, sd = same.dprChanges;
    same.fitter.fit();
    check('a same-size refit resizes nothing', same.resizes === sr, 'resizes ' + same.resizes);
    check('a same-size refit measures nothing', same.measures === sm, 'measures ' + same.measures);
    check('a same-size refit keeps the font', same.fontSize === 14, 'font ' + same.fontSize);
    check('a same-size refit leaves the display scale alone', same.dprChanges === sd, 'dpr changes ' + same.dprChanges);
    same.glyph = 0.5;                      // the web font arrives, narrower letters
    same.fitter.fit(true);
    check('a forced refit measures again', same.measures === sm + 1, 'measures ' + same.measures);
    check('a forced refit counts the new letters', same.real.cols === fits(360, 14 * 0.5),
        'cols ' + same.real.cols);
    check('a forced refit keeps the font', same.fontSize === 14, 'font ' + same.fontSize);

    // A forced refit asked for while the panel is hidden still happens.
    var hidden = makePage(1600, 900, { text: 16, minFont: 1 });
    hidden.fitter.fit();
    var hm = hidden.measures, hw = hidden.gameWidths.length;
    hidden.glyph = 0.5;
    hidden.panel.clientWidth = 0;
    hidden.fitter.fit(true);
    check('a hidden panel is not fitted', hidden.measures === hm, 'measures ' + hidden.measures);
    hidden.panel.clientWidth = 1600;
    hidden.fitter.fit();
    check('a forced refit survives a hidden panel', hidden.measures === hm + 1, 'measures ' + hidden.measures);
    check('the font load reports the new game width',
        hidden.gameWidths.length === hw + 1 && hidden.gameWidths[hw] === expected80(16 * 0.5, 1),
        'widths ' + hidden.gameWidths.join(','));

    // A terminal that cannot measure yet has not been fitted.
    var unmeasured = makePage(1600, 900, { text: 16, minFont: 1, unmeasured: true });
    check('widthFor80 is null before xterm has measured', unmeasured.fitter.widthFor80() === null,
        'width ' + unmeasured.fitter.widthFor80());
    unmeasured.fitter.fit();
    check('an unmeasured fit reports no game width', unmeasured.gameWidths.length === 0,
        'widths ' + unmeasured.gameWidths.join(','));
    unmeasured.unmeasured = false;
    var um = unmeasured.measures;
    unmeasured.fitter.fit();
    check('a fit with no measurement is not remembered',
        unmeasured.measures === um + 1 && unmeasured.real.cols === 80 && unmeasured.gameWidths.length === 1,
        'measures ' + unmeasured.measures + ' cols ' + unmeasured.real.cols + ' widths ' + unmeasured.gameWidths.join(','));

    // A move to a monitor with other scaling changes the letter widths.
    var moved = makePage(1600, 900, { text: 16, minFont: 1 });
    global.devicePixelRatio = 1;
    moved.fitter.fit();
    var mm = moved.measures, md = moved.dprChanges;
    moved.glyph = 0.5;
    global.devicePixelRatio = 2;
    moved.fitter.fit();
    check('a new display scale measures again', moved.measures === mm + 1, 'measures ' + moved.measures);
    check('a new display scale updates the renderer', moved.dprChanges === md + 1, 'dpr changes ' + moved.dprChanges);
    global.devicePixelRatio = 1;

    // widthFor80: 80 cells as the renderer sizes them, plus the scrollbar.
    [1, 1.25, 2].forEach(function (dpr) {
        global.devicePixelRatio = dpr;
        var p = makePage(1600, 900, { text: 16, minFont: 1 });
        p.fitter.fit();
        var want = expected80(16 * 0.6, dpr);
        check('widthFor80 at display scale ' + dpr + ' is ' + want, p.fitter.widthFor80() === want,
            'width ' + p.fitter.widthFor80());
    });
    global.devicePixelRatio = 1;

    // The same with 42 columns as with 80: the layout must not chase the column count.
    var at80 = makePage(1600, 900, { text: 16, minFont: 1 });
    at80.fitter.fit();
    // At display scale 1 a 9.6px letter is drawn in a 9px cell.
    var at42 = makePage(42 * Math.floor(16 * 0.6) + SCROLLBAR, 900, { text: 16, minFont: 1 });
    at42.fitter.fit();
    check('the 42-column page really has 42 columns', at42.real.cols === 42, 'cols ' + at42.real.cols);
    check('widthFor80 is the same at 42 columns as at 80',
        at42.fitter.widthFor80() === at80.fitter.widthFor80() && at80.fitter.widthFor80() === expected80(16 * 0.6, 1),
        'at 42 ' + at42.fitter.widthFor80() + ' at 80 ' + at80.fitter.widthFor80());
    check('cols() is the terminal\'s column count', at42.fitter.cols() === 42, 'cols ' + at42.fitter.cols());

    // A display-scale change calls the renderer's own update.
    var zoom = makePage(1600, 900, { text: 16, minFont: 1 });
    zoom.fitter.fit();
    var zd = zoom.dprChanges, zw = zoom.gameWidths.length;
    check('the first fit reports the game width once',
        zw === 1 && zoom.gameWidths[0] === expected80(16 * 0.6, 1), 'widths ' + zoom.gameWidths.join(','));
    zoom.fitter.fit();
    zoom.panel.clientHeight = 700; zoom.fitter.fit();
    check('an unchanged game width is not reported again', zoom.gameWidths.length === zw,
        'widths ' + zoom.gameWidths.join(','));
    global.devicePixelRatio = 1.25;
    zoom.fitter.fit();
    check('a display-scale change calls onDevicePixelRatioChange', zoom.dprChanges === zd + 1,
        'dpr changes ' + zoom.dprChanges);
    check('a display-scale change reports the new game width',
        zoom.gameWidths.length === zw + 1 && zoom.gameWidths[zw] === expected80(16 * 0.6, 1.25),
        'widths ' + zoom.gameWidths.join(','));
    zoom.fitter.fit(true);
    check('a forced refit with the same width reports nothing new', zoom.gameWidths.length === zw + 1,
        'widths ' + zoom.gameWidths.join(','));
    global.devicePixelRatio = 1;

    driftChecks();
    listenerChecks();
}

// xterm's own cell width is the canvas width over the CURRENT column count,
// so its proposal drifts with that count (review 2026-11-12). The columns
// must come from the letters alone.
function driftChecks() {
    // A browser widened to exactly widthFor80, from any narrower count: 80.
    // At 16px these displays once gave 79 from some counts (41, 43, ...).
    [1.25, 1.5, 2, 0.8, 0.9].forEach(function (dpr) {
        global.devicePixelRatio = dpr;
        var W = expected80(16 * 0.6, dpr), bad = [];
        for (var from = 30; from < 80; from++) {
            var p = makePage(W, 900, { text: 16, minFont: 1, cols: from });
            p.fitter.fit();
            if (p.real.cols !== 80) bad.push(from + '->' + p.real.cols);
        }
        check('widening to widthFor80 at display scale ' + dpr + ' gives 80 from any count',
            bad.length === 0, 'host ' + W + ': ' + bad.join(' '));
    });

    // The keyboard at a width where xterm's proposal flips between two counts.
    global.devicePixelRatio = 2.625;
    var reported = [];
    var kb = makePage(401, 700, { text: 14, minFont: 1, onCols: function (c) { reported.push(c); } });
    kb.fitter.fit();
    var cols = kb.real.cols, n = reported.length;
    check('the flip page gets the columns that fit', cols === fits(401, 14 * 0.6, 2.625), 'cols ' + cols);
    var seen = [];
    for (var h = 650; h >= 450; h -= 50) {
        kb.panel.clientHeight = h; kb.fitter.fit();
        seen.push(kb.real.cols);
    }
    check('a height-only fit at display scale 2.625, 8.4px letters, 401px keeps its columns',
        seen.every(function (c) { return c === cols; }) && reported.length === n,
        'cols ' + cols + ' then ' + seen.join(',') + ' reports ' + reported.join(','));

    // The sweep: every width from 250 to 800 at five display scales and both
    // letter sizes. The first fit gets the columns that fit; forced refits
    // (a font load) and keyboard moves at the same width never change them.
    var bad = [], pages = 0;
    [1, 1.25, 1.5, 2, 2.625].forEach(function (dpr) {
        global.devicePixelRatio = dpr;
        [14, 16].forEach(function (text) {
            var letter = text * 0.6;
            for (var W = 250; W <= 800; W++) {
                var p = makePage(W, 600, { text: text, minFont: 1 });
                pages++;
                p.fitter.fit();
                var want = fits(W, letter, dpr), seq = [p.real.cols];
                p.fitter.fit(true); seq.push(p.real.cols);
                p.fitter.fit(true); seq.push(p.real.cols);
                p.panel.clientHeight = 500; p.fitter.fit(); seq.push(p.real.cols);
                p.panel.clientHeight = 400; p.fitter.fit(); seq.push(p.real.cols);
                p.fitter.fit(true); seq.push(p.real.cols);
                if (seq.some(function (c) { return c !== want; })) {
                    bad.push('dpr ' + dpr + ' letter ' + letter + ' W ' + W + ' want ' + want + ' got ' + seq.join(','));
                }
            }
        });
    });
    check('sweep: ' + pages + ' pages keep the columns that fit through refits and keyboard moves',
        bad.length === 0, bad.length + ' bad, e.g.\n  ' + bad.slice(0, 8).join('\n  '));
    global.devicePixelRatio = 1;
}

// A listener that throws cannot keep the other from hearing, and a font
// lookup that throws does not escape the fit.
function listenerChecks() {
    var widths = [];
    var a = makePage(1600, 900, { text: 16, minFont: 1, cols: 40,
        onCols: function () { throw new Error('onCols'); },
        onGameWidth: function (px) { widths.push(px); } });
    a.fitter.fit();
    check('a throwing onCols does not silence onGameWidth', widths.length === 1, 'widths ' + widths.join(','));

    var cols = [];
    var b = makePage(1600, 900, { text: 16, minFont: 1, cols: 40,
        onCols: function (c) { cols.push(c); },
        onGameWidth: function () { throw new Error('onGameWidth'); } });
    b.fitter.fit();
    check('a throwing onGameWidth does not silence onCols', cols.length === 1 && cols[0] === 80,
        'cols ' + cols.join(','));

    // The layout hears the new game width before the columns are counted.
    // The page gives the game window exactly widthFor80 (dashboard.js
    // setGameWidth); here onGameWidth sets the host to it, as the page's
    // layout does. The fallback font is narrower (0.5em, 8px cells); when
    // the web font lands (0.6em, 9px cells) the same fit must widen the
    // window first and end at 80 columns, never counting the old width.
    var lay;
    lay = makePage(16 * 0.5 * 80 + SCROLLBAR, 900, { text: 16, minFont: 1,
        onGameWidth: function (px) { lay.panel.clientWidth = px; } });
    lay.glyph = 0.5;
    lay.fitter.fit(true);
    check('the fallback font fills its window at 80 columns', lay.real.cols === 80, 'cols ' + lay.real.cols);
    lay.narrowest = lay.real.cols;
    lay.glyph = 0.6;                       // the web font arrives, wider letters
    lay.fitter.fit(true);
    check('a font load widens the game window before counting: 80 columns',
        lay.real.cols === 80 && lay.panel.clientWidth === expected80(16 * 0.6, 1),
        'cols ' + lay.real.cols + ' host ' + lay.panel.clientWidth);
    check('a font load never resizes the game window below 80, even for a moment',
        lay.narrowest === 80, 'narrowest ' + lay.narrowest);

    // Before the web font loads the page gives the layout a predicted width.
    // Once a 0.6em font is measured, widthFor80 must equal it, so the game
    // window does not move when the font lands.
    [1, 1.25, 2].forEach(function (dpr) {
        [16, 14].forEach(function (text) {
            global.devicePixelRatio = dpr;
            var pr = makePage(1600, 900, { text: text, minFont: 1 });
            pr.glyph = 0.5;                    // the fallback font is in
            pr.fitter.fit(true);
            var predicted = pr.fitter.predictWidthFor80(text);
            pr.glyph = 0.6;                    // JetBrains Mono lands
            pr.fitter.fit(true);
            check('the predicted game width is the measured one (dpr ' + dpr + ', ' + text + 'px)',
                predicted === pr.fitter.widthFor80() && predicted === expected80(text * 0.6, dpr),
                'predicted ' + predicted + ' measured ' + pr.fitter.widthFor80());

            // The page connects on load, often before the font, and puts the
            // columns on /ws. A host exactly the predicted width holds 80 in
            // the fallback font and in the real one.
            var conn = makePage(predicted, 900, { text: text, minFont: 1 });
            conn.glyph = 0.5;
            conn.fitter.fit(true);
            var before = conn.real.cols;
            conn.glyph = 0.6;
            conn.fitter.fit(true);
            check('a desktop connects at 80 columns in the predicted window, either font (dpr ' + dpr + ', ' + text + 'px)',
                before === 80 && conn.real.cols === 80 && conn.narrowest === 80,
                'fallback ' + before + ' real ' + conn.real.cols + ' narrowest ' + conn.narrowest);
        });
    });
    global.devicePixelRatio = 1;

    var c = makePage(1600, 900, { text: 16, minFont: 1,
        fontSize: function () { throw new Error('fontSize'); } });
    var threw = false;
    try { c.fitter.fit(); } catch (e) { threw = true; }
    check('a throwing font lookup does not escape the fit', !threw, 'threw');
}

function run() {
    syncChecks();

    // A phone panel full of scrollback: the case that lost lines.
    var phone = makePage(360, 500, { text: 14, minFont: 1 });
    fill(phone, function () {
        check('the fixture holds a full scrollback', phone.real.buffer.active.length > 1000,
            'length ' + phone.real.buffer.active.length);
        phone.fitter.fit();                       // the one narrowing (decision 6)
        var before = phone.real.buffer.active.length, cols = phone.real.cols, resizes = phone.resizes;
        for (var i = 0; i < 5; i++) {
            phone.panel.clientHeight = 500 - i * 40;  // a keyboard or toolbar moving
            phone.fitter.fit();
        }
        check('keyboard moves never change the columns, even for a moment',
            phone.narrowest === cols && phone.real.cols === cols, 'narrowest ' + phone.narrowest + ' cols ' + phone.real.cols);
        check('each keyboard move resizes at most once', phone.resizes - resizes <= 4,
            'resizes ' + (phone.resizes - resizes));
        var lost = before - phone.real.buffer.active.length;
        // Fewer rows means a few lines move into scrollback and the oldest
        // fall off the limit; only that small amount may go.
        check('five refits keep the scrollback', lost <= 40, 'lost ' + lost + ' of ' + before);

        // Turning upright: the token changes to 14 and the panel narrows; one report.
        // Narrowing keeps the newest lines (decision 6).
        var reported = [];
        var turn = makePage(800, 400, { text: 16, minFont: 1, onCols: function (c) { reported.push(c); } });
        fill(turn, function () {
            turn.fitter.fit();
            check('the sideways page has 80 columns', turn.real.cols === 80, 'cols ' + turn.real.cols);
            var buf = turn.real.buffer.active;
            var last = buf.getLine(buf.length - 2).translateToString(true);
            var n = reported.length, r = turn.resizes;
            turn.text = 14; turn.panel.clientWidth = 360; turn.fitter.fit();
            buf = turn.real.buffer.active;
            var tail = '';
            for (var i = buf.length - 6; i < buf.length; i++) {
                tail += buf.getLine(i).translateToString(true);
            }
            check('turning upright uses the 14px text', turn.fontSize === 14, 'font ' + turn.fontSize);
            check('turning upright gets the columns that fit', turn.real.cols === fits(360, 14 * 0.6),
                'cols ' + turn.real.cols);
            check('turning upright reports one column count',
                reported.length === n + 1 && reported[n] === turn.real.cols, 'reports ' + reported.join(','));
            check('turning upright resizes exactly once', turn.resizes === r + 1, 'resizes ' + (turn.resizes - r));
            check('narrowing keeps the newest line', last.length > 10 && tail.indexOf(last.slice(0, 10)) >= 0,
                'last ' + last + ' tail ' + tail);

            console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
            process.exit(failures ? 1 : 0);
        });
    });
}

run();
