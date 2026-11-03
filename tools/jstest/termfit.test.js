// termfit.test.js: fitting the game text to its panel keeps the scrollback.
//
// Run:  node tools/jstest/termfit.test.js
//
// Uses the real pinned xterm buffer, so reflow and trimming are the real thing.
// Only the page around it is faked: a cell is 0.6em wide and 1.2em tall, and
// the fit addon proposes what fits the host at the current font size.
'use strict';

var path = require('path');
var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');

global.self = global;
global.window = global;
global.document = {
    createElement: function () { return { getContext: function () { return null; }, style: {} }; },
    addEventListener: function () {},
    removeEventListener: function () {}
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
function makePage(W, H, minFont) {
    var real = new X.Terminal({ cols: 80, rows: 40, scrollback: 1000 });
    var page = { real: real, fontSize: 20, narrowest: 80, resizes: 0, scale: 1 };
    var host = { style: {}, parentElement: { clientWidth: W, clientHeight: H } };
    page.host = host;
    page.panel = host.parentElement;

    function cellW() { return page.fontSize * 0.6; }
    function cellH() { return page.fontSize * 1.2; }
    function hostW() { return parseFloat(host.style.width) || page.panel.clientWidth; }
    function hostH() { return parseFloat(host.style.height) || page.panel.clientHeight; }

    var term = {
        get cols() { return real.cols; },
        get rows() { return real.rows; },
        setOption: function (k, v) { if (k === 'fontSize') page.fontSize = v; },
        _core: { _renderService: { get dimensions() {
            return { actualCellWidth: cellW(), actualCellHeight: cellH() };
        } } }
    };
    var fitAddon = {
        proposeDimensions: function () {
            if (page.unmeasured) return undefined;
            return { cols: Math.floor(hostW() / cellW()), rows: Math.floor(hostH() / cellH()) };
        },
        fit: function () {
            var d = this.proposeDimensions();
            if (!d) return;
            if (d.cols === real.cols && d.rows === real.rows) return;
            page.resizes++;
            page.narrowest = Math.min(page.narrowest, d.cols);
            real.resize(d.cols, d.rows);
        }
    };
    page.fitter = TermFit.create({
        term: term, fitAddon: fitAddon, host: host, minFont: minFont,
        setScale: function (s) { page.scale = s; }
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

function run() {
    // A phone panel, 80 columns only fit below 20px: the case that lost lines.
    var phone = makePage(360, 500, 1);
    fill(phone, function () {
        var before = phone.real.buffer.active.length;
        check('the fixture holds a full scrollback', before > 1000, 'length ' + before);

        for (var i = 0; i < 5; i++) {
            phone.panel.clientHeight = 500 - i * 40;  // a keyboard or toolbar moving
            phone.fitter.fit();
        }
        check('never narrower than 80 columns on a phone, even for a moment',
            phone.narrowest >= 80, 'narrowest ' + phone.narrowest);
        var lost = before - phone.real.buffer.active.length;
        // Fewer rows means a few lines move into scrollback and the oldest
        // fall off the limit; only that small amount may go.
        check('five refits keep the scrollback', lost <= 40, 'lost ' + lost + ' of ' + before);

        // An unchanged panel does no work at all.
        var resizes = phone.resizes, font = phone.fontSize;
        phone.fitter.fit();
        check('a same-size refit resizes nothing', phone.resizes === resizes, 'resizes ' + phone.resizes);
        check('a same-size refit keeps the font', phone.fontSize === font, 'font ' + phone.fontSize);

        // The font load changes the letter widths with the panel unchanged.
        phone.fontSize = 3;
        phone.fitter.fit(true);
        check('a forced refit measures again', phone.fontSize !== 3, 'font ' + phone.fontSize);

        // A wide panel grows the font to the cap and keeps at least 80 columns.
        var wide = makePage(1600, 900, 1);
        wide.fitter.fit();
        check('a wide panel uses the largest font', wide.fontSize === 20, 'font ' + wide.fontSize);
        check('a wide panel has at least 80 columns', wide.real.cols >= 80, 'cols ' + wide.real.cols);
        check('a wide panel is not scaled', wide.scale === 1, 'scale ' + wide.scale);

        // A raised browser minimum: draw at the minimum and scale down instead.
        var raised = makePage(360, 500, 10);
        raised.fitter.fit();
        check('a raised minimum holds the font', raised.fontSize === 10, 'font ' + raised.fontSize);
        check('a raised minimum still gives 80 columns', raised.real.cols >= 80, 'cols ' + raised.real.cols);
        check('a raised minimum never lays out narrower than 80',
            raised.narrowest >= 80, 'narrowest ' + raised.narrowest);
        check('a raised minimum scales the terminal down', raised.scale < 1, 'scale ' + raised.scale);
        check('the scale is on the host', /^scale\(/.test(raised.host.style.transform || ''),
            'transform ' + raised.host.style.transform);

        // Back to a panel that fits: the scale comes off.
        raised.panel.clientWidth = 1600;
        raised.fitter.fit();
        check('a wide panel removes the scale', raised.scale === 1 && !raised.host.style.transform,
            'scale ' + raised.scale + ' transform ' + raised.host.style.transform);

        // While the page loads the frame can be a few pixels wide.
        var tiny = makePage(20, 20, 1);
        tiny.fitter.fit();
        check('a tiny frame is left alone', tiny.resizes === 0, 'resizes ' + tiny.resizes);

        // A forced refit asked for while the panel is hidden still happens.
        var hidden = makePage(1600, 900, 1);
        hidden.fitter.fit();
        hidden.fontSize = 3;
        hidden.panel.clientWidth = 0;
        hidden.fitter.fit(true);
        hidden.panel.clientWidth = 1600;
        hidden.fitter.fit();
        check('a forced refit survives a hidden panel', hidden.fontSize === 20, 'font ' + hidden.fontSize);

        // A terminal that cannot measure yet has not been fitted.
        var unmeasured = makePage(1600, 900, 1);
        unmeasured.unmeasured = true;
        unmeasured.fitter.fit();
        unmeasured.unmeasured = false;
        unmeasured.fontSize = 3;
        unmeasured.fitter.fit();
        check('a fit with no measurement is not remembered', unmeasured.fontSize === 20, 'font ' + unmeasured.fontSize);

        // A move to a monitor with other scaling changes the letter widths.
        var moved = makePage(1600, 900, 1);
        global.devicePixelRatio = 1;
        moved.fitter.fit();
        moved.fontSize = 3;
        global.devicePixelRatio = 2;
        moved.fitter.fit();
        check('a new display scale measures again', moved.fontSize === 20, 'font ' + moved.fontSize);

        console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
        process.exit(failures ? 1 : 0);
    });
}

run();
