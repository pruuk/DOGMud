// viewportfit.js: phone detection, the full screen default, and the height of
// the game frame with the on-screen keyboard open.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.ViewportFit = api;    // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // Judged from the screen, so rotating, the keyboard and the iframe cannot
    // change the answer. A phone's shorter side is under 600 CSS px; a tablet's
    // is not.
    function isPhone(screenW, screenH) {
        return Math.min(screenW, screenH) < 600;
    }

    // pref is the remembered choice: '1', '0', or null for none yet.
    function wantFullScreen(pref, phone) {
        if (pref === '1') return true;
        if (pref === '0') return false;
        return phone;
    }

    // layoutH: the height the page gives the frame. visibleH: the visual
    // viewport's height, which excludes the keyboard; undefined where the
    // browser has none. top: the frame's top edge within the visible area.
    function frameHeight(layoutH, visibleH, top) {
        if (typeof visibleH !== 'number') return layoutH;
        return Math.max(0, Math.min(layoutH, Math.floor(visibleH) - top));
    }

    // The visual viewport's height, or undefined when it says nothing about the
    // keyboard: the browser has none, or the player has zoomed in, which shrinks
    // it too. Phone players zoom to read, and that must not squash the game.
    function visibleHeight(vv) {
        if (!vv || vv.scale > 1.01) return undefined;
        return vv.height;
    }

    // True when the visible area is well short of the page: more than an address
    // bar hiding can account for.
    function keyboardOpen(layoutH, visibleH) {
        return typeof visibleH === 'number' && layoutH - visibleH > 120;
    }

    return {
        keyboardOpen: keyboardOpen,
        isPhone: isPhone,
        wantFullScreen: wantFullScreen,
        frameHeight: frameHeight,
        visibleHeight: visibleHeight
    };
}));
