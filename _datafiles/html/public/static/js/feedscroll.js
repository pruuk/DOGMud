// feedscroll.js: keeps the Game window's text from sliding out of view
// sideways.
//
// Playtest 2026-10-17: once in a while the middle panel's text disappeared,
// and the owner suspected a sideways mouse scroll. Reading the page bears
// that out as a likely cause, though nobody has reproduced it live:
//
//   * xterm's own wheel handler only acts on vertical motion. A wheel event
//     that is purely sideways (a tilt wheel, a trackpad swipe, or Shift+wheel,
//     which Chrome on Windows turns into deltaX) is not cancelled, so the
//     browser scrolls the nearest container that can move sideways.
//   * .xterm-viewport is overflow-y: scroll, which makes its overflow-x
//     compute to auto, so it can take a sideways scroll.
//   * The feed's panel and panel body are overflow: hidden. A hidden-overflow
//     box cannot be scrolled by the wheel, but the browser still scrolls it
//     to reveal a focused descendant (xterm's helper textarea follows the
//     cursor) or while a drag-select runs past the edge. Once shifted it can
//     never be scrolled back by hand, so the text stays off-screen.
//
// The guard does two things: it cancels sideways wheel motion over the feed,
// and it snaps any container that does get shifted back to where it belongs.
// Neither touches vertical scrolling of the scrollback, which xterm owns.
//
// tools/jstest/feedscroll.test.js is the regression guard; the page requires
// this file and calls FeedScroll.install after term.open.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.FeedScroll = api;     // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {

    // A wheel event whose motion is mostly sideways. Pure vertical motion,
    // and a diagonal that is mostly vertical, are left to xterm.
    function isSidewaysWheel(e) {
        if (!e) return false;
        var dx = Math.abs(e.deltaX || 0);
        var dy = Math.abs(e.deltaY || 0);
        return dx > 0 && dx > dy;
    }

    // Put a shifted container back. bothAxes is for hidden-overflow boxes,
    // which have no legitimate scroll offset at all; the xterm viewport may
    // scroll vertically, so only its horizontal offset is reset.
    function pin(el, bothAxes) {
        if (!el) return;
        if (el.scrollLeft) el.scrollLeft = 0;
        if (bothAxes && el.scrollTop) el.scrollTop = 0;
    }

    // els: { terminal, viewport, body, panel }. Any may be missing.
    function install(els) {
        els = els || {};
        if (els.terminal && els.terminal.addEventListener) {
            els.terminal.addEventListener('wheel', function (e) {
                if (isSidewaysWheel(e) && e.preventDefault) e.preventDefault();
            }, { passive: false });
        }
        var watch = [
            [els.panel, true],
            [els.body, true],
            [els.terminal, true],
            [els.viewport, false]
        ];
        watch.forEach(function (w) {
            var el = w[0], bothAxes = w[1];
            if (!el || !el.addEventListener) return;
            el.addEventListener('scroll', function () { pin(el, bothAxes); });
            pin(el, bothAxes);
        });
    }

    return {
        isSidewaysWheel: isSidewaysWheel,
        pin: pin,
        install: install
    };
}));
