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

    // The height left by the keyboard, in page pixels; undefined where the browser
    // has no visual viewport. Zooming in shrinks the visual viewport by its scale,
    // so that is multiplied back out: phone players zoom to read, and that must
    // not squash the game. A keyboard still counts while zoomed.
    function visibleHeight(vv) {
        if (!vv) return undefined;
        return vv.height * (vv.scale || 1);
    }

    // True when the visible area is well short of the page: more than an address
    // bar hiding can account for.
    function keyboardOpen(layoutH, visibleH) {
        return typeof visibleH === 'number' && layoutH - visibleH > 120;
    }

    // Whether the Full screen button should read as on. Where the browser can
    // go full screen, that means it really is: the site header hidden AND the
    // browser's own bars gone. A phone starts with the header hidden but its
    // address bar showing, so it reads "Full screen" until the player taps it
    // (Calabe 2026-11-07). Where the browser cannot (iPhone Safari), hiding
    // the header is all full screen can do.
    function fullScreenShown(layoutOn, browserCan, browserOn) {
        return browserCan ? (layoutOn && browserOn) : layoutOn;
    }

    // Whether tapping Enter Game should also take a phone full screen
    // (Calabe 2026-11-07). The browser only allows it from a tap, so it cannot
    // happen at load. A player who chose to leave full screen keeps that.
    function enterFullScreenOnConnect(phone, pref, browserCan, browserOn) {
        return phone && pref !== '0' && browserCan && !browserOn;
    }

    return {
        keyboardOpen: keyboardOpen,
        isPhone: isPhone,
        wantFullScreen: wantFullScreen,
        fullScreenShown: fullScreenShown,
        enterFullScreenOnConnect: enterFullScreenOnConnect,
        frameHeight: frameHeight,
        visibleHeight: visibleHeight
    };
}));
