// viewportfit.js: phone detection, the full screen default, and the height of
// the game frame with the on-screen keyboard open.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.ViewportFit = api;    // browser
        api.watchUprightPhone(root);
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // Judged from the screen, so rotating, the keyboard and the iframe cannot
    // change the answer. A phone's shorter side is under 600 CSS px; a tablet's
    // is not.
    function isPhone(screenW, screenH) {
        return Math.min(screenW, screenH) < 600;
    }

    // A laptop (owner 2026-11-15): a screen whose short side is under 1000px
    // and that is not a phone. It opens full screen as a phone does, so the
    // site's header and footer leave the game their height.
    function isShortScreen(screenW, screenH) {
        return !isPhone(screenW, screenH) && Math.min(screenW, screenH) < 1000;
    }

    // An upright phone reads 14/18 instead of 16/20 (gomud.css). Judged from the
    // screen, like isPhone. iOS does not swap screen.width and height when the
    // phone turns, so the orientation API comes first. coarse: the screen is
    // a touch screen (pointer: coarse). A desktop monitor turned upright, or
    // zoomed in, can have a short side under 600 too; it keeps 16px, as the
    // layout ladder treats it as a desktop (plan 5, decision 13).
    function isUprightPhone(screenW, screenH, orientationType, coarse) {
        var landscape = orientationType ? orientationType.indexOf('landscape') === 0 : screenW >= screenH;
        return !!coarse && isPhone(screenW, screenH) && !landscape;
    }

    // A phone held sideways (plan 6, ruling 15): the mirror of isUprightPhone.
    // Judged from the screen, so the on-screen keyboard cannot trigger it.
    function isSidewaysPhone(screenW, screenH, orientationType, coarse) {
        var landscape = orientationType ? orientationType.indexOf('landscape') === 0 : screenW >= screenH;
        return !!coarse && isPhone(screenW, screenH) && landscape;
    }

    // The screen's orientation type, or undefined where the browser gives no
    // signal. iOS before 16.4 has no screen.orientation; window.orientation is
    // its only true signal.
    function orientationType(win) {
        var s = win.screen || {};
        var type = s.orientation && s.orientation.type;
        if (!type && typeof win.orientation === 'number') {
            type = Math.abs(win.orientation) === 90 ? 'landscape-primary' : 'portrait-primary';
        }
        return type;
    }

    function markUprightPhone(win) {
        var s = win.screen;
        var coarse = !!(win.matchMedia && win.matchMedia('(pointer: coarse)').matches);
        win.document.documentElement.classList.toggle('upright-phone',
            isUprightPhone(s.width, s.height, orientationType(win), coarse));
    }

    // The sideways note (layout spec ruling 15), once per browser tab:
    // sessionStorage survives a reload but not a new tab. memo stops a repeat
    // on this page where storage is missing or blocked.
    var SIDEWAYS_TOAST = 'This game is easier to play with your phone held upright.';
    var SIDEWAYS_KEY = 'dogmud.sidewaysToast';
    function takeSidewaysToast(storage, memo) {
        if (memo.shown) return false;
        memo.shown = true;
        try {
            if (storage.getItem(SIDEWAYS_KEY) === '1') return false;
            storage.setItem(SIDEWAYS_KEY, '1');
        } catch (e) { /* missing or blocked storage: memo alone */ }
        return true;
    }

    function watchUprightPhone(win) {
        var mark = function () { markUprightPhone(win); };
        mark();
        win.addEventListener('resize', mark);
        win.addEventListener('orientationchange', mark);
    }

    // pref is the remembered choice: '1', '0', or null for none yet.
    // byDefault: a phone or a laptop (isPhone or isShortScreen).
    function wantFullScreen(pref, byDefault) {
        if (pref === '1') return true;
        if (pref === '0') return false;
        return byDefault;
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
    // The on-screen keyboard can only be open while an editable field has focus
    // (editableFocused): a page that grew wider than the screen at load also
    // has a layout viewport taller than the visible one, with no keyboard.
    function keyboardOpen(layoutH, visibleH, editableFocused) {
        return !!editableFocused && typeof visibleH === 'number' && layoutH - visibleH > 120;
    }

    // True when the document's focused element takes typed text: a text-like
    // input, a textarea, or anything contenteditable.
    var NON_TEXT_INPUTS = { button: 1, checkbox: 1, radio: 1, submit: 1, reset: 1, range: 1, color: 1, file: 1, image: 1 };
    function editableFocused(doc) {
        var el = doc && doc.activeElement;
        if (!el) return false;
        var tag = String(el.tagName || '').toUpperCase();
        if (tag === 'TEXTAREA') return true;
        if (tag === 'INPUT') return !NON_TEXT_INPUTS[String(el.type || 'text').toLowerCase()];
        return !!el.isContentEditable;
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

    // Whether tapping Enter Game should also take a phone or a laptop full screen
    // (Calabe 2026-11-07 for phones, 2026-11-15 for laptops). The browser only
    // allows it from a tap, so it cannot happen at load. A player who chose to
    // leave full screen keeps that.
    function enterFullScreenOnConnect(byDefault, pref, browserCan, browserOn) {
        return byDefault && pref !== '0' && browserCan && !browserOn;
    }

    return {
        keyboardOpen: keyboardOpen,
        editableFocused: editableFocused,
        isPhone: isPhone,
        isShortScreen: isShortScreen,
        isUprightPhone: isUprightPhone,
        isSidewaysPhone: isSidewaysPhone,
        orientationType: orientationType,
        takeSidewaysToast: takeSidewaysToast,
        SIDEWAYS_TOAST: SIDEWAYS_TOAST,
        watchUprightPhone: watchUprightPhone,
        wantFullScreen: wantFullScreen,
        fullScreenShown: fullScreenShown,
        enterFullScreenOnConnect: enterFullScreenOnConnect,
        frameHeight: frameHeight,
        visibleHeight: visibleHeight
    };
}));
