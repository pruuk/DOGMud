// touchdeck.js: the touch controls deck (touch arc spec 1, plan 1a;
// docs/superpowers/specs/2026-11-21-touch-frame-design.md).
//
// The deck is a block of buttons above the command box. On a phone it sits
// in the slot under the game text, which holds the deck or the open tab
// (dashboard.js). Buttons only ever come from the server (the roadmap's sync
// rule), so in plan 1a the deck draws empty slots.
//
// Pure functions, loaded by the page and by node for the tests:
//   readPref / writePref  the Touch controls setting on this device
//   deckOn                whether the deck shows
//   shape                 rows, columns and sizes for the room it has
//   slotShows             what the slot under the game text holds
//   fieldOf               where the player is typing (plan 1b)
//   rememberScroll / restoreScroll  a stepped-aside tab's scroll place
// Tested by tools/jstest/touchdeck.test.js and touch-targets.test.js.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;       // node, for the tests
    } else {
        root.TouchDeck = api;       // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // The setting: Automatic, Always or Never, on this device (like Sound
    // and Appearance).
    var KEY = 'dogmud.touchControls';
    var PREFS = { auto: true, always: true, never: true };

    // Automatic shows the deck on a touch screen only once the deck has
    // buttons (owner ruling 2026-11-21). Plan 2b sets this to true.
    var AUTOMATIC_SHOWS = false;

    function readPref(storage) {
        try {
            var v = storage ? storage.getItem(KEY) : null;
            return PREFS[v] === true ? v : 'auto';
        } catch (e) {
            return 'auto';
        }
    }

    function writePref(storage, value) {
        var v = PREFS[value] === true ? value : 'auto';
        try { storage.setItem(KEY, v); } catch (e) { /* blocked: this visit only */ }
        return v;
    }

    // coarse: touch is the main input (pointer: coarse).
    function deckOn(pref, coarse) {
        if (pref === 'always') return true;
        if (pref === 'never') return false;
        return AUTOMATIC_SHOWS && !!coarse;
    }

    // Sizes (roadmap fat-finger rules; spec ruling 3 for the small size).
    var BTN = 48, GAP = 8, SMALL_BTN = 44, SMALL_GAP = 5, PAD = 8;
    var ROWS = 2, PHONE_COLS = 6, SIDE_ROWS = 4, SIDE_COLS = 3;

    // The length n buttons take in a line, with the deck's padding.
    function span(n, btn, gap) { return n * btn + (n - 1) * gap + 2 * PAD; }

    function make(rows, cols, btn, gap, side) {
        return {
            rows: rows, cols: cols, button: btn, gap: gap, pad: PAD, side: side,
            width: span(cols, btn, gap), height: span(rows, btn, gap)
        };
    }

    // The deck's shape for the room it has. boxW, boxH: that room in CSS px,
    // inside the slot's side borders. phone: an upright phone (six columns,
    // so a button keeps its slot on every phone). sideways: a phone on its
    // side (a side column beside the game, four rows of three).
    function shape(boxW, boxH, phone, sideways) {
        if (sideways) {
            var tall = span(SIDE_ROWS, BTN, GAP) <= boxH;
            return make(SIDE_ROWS, SIDE_COLS, tall ? BTN : SMALL_BTN, tall ? GAP : SMALL_GAP, true);
        }
        var fit = Math.floor((boxW - 2 * PAD + GAP) / (BTN + GAP));
        if (phone || fit < PHONE_COLS) {
            var wide = span(PHONE_COLS, BTN, GAP) <= boxW;
            return make(ROWS, PHONE_COLS, wide ? BTN : SMALL_BTN, wide ? GAP : SMALL_GAP, false);
        }
        return make(ROWS, fit, BTN, GAP, false);
    }

    // What the slot under the game text holds: 'tab', 'deck' or 'empty'.
    // o.phone: the phone layout, where an open tab lives in the slot.
    // o.tabOpen, o.deckOn, o.keyboard: the on-screen keyboard is up.
    // o.field: where the player is typing (fieldOf). Typing in the command
    // box sets the tab aside so the game text gets the room (plan 1b, ruled
    // 2026-11-19); typing in another field (the Automation dialog) moves
    // nothing. The keyboard covers the deck either way.
    function slotShows(o) {
        if (o.phone && o.tabOpen) {
            return o.keyboard && o.field === 'command' ? 'empty' : 'tab';
        }
        if (o.deckOn && !o.keyboard) return 'deck';
        return 'empty';
    }

    // Where the player is typing: 'command' for the command box, 'other' for
    // any other text field, null when nothing that takes text has focus.
    // activeId: document.activeElement's id. editable: ViewportFit.
    // editableFocused(document), the page's one test for a text field.
    var COMMAND_ID = 'command-input';
    function fieldOf(activeId, editable) {
        if (!editable) return null;
        return activeId === COMMAND_ID ? 'command' : 'other';
    }

    // A tab that steps aside is hidden, and a hidden box loses its scroll
    // place (dashboard.js _scrollChatToNewest). rememberScroll lists root and
    // every element under it that is scrolled down; restoreScroll puts each
    // back. root: anything with scrollTop and querySelectorAll.
    function rememberScroll(root) {
        var saved = [];
        if (!root) return saved;
        if (root.scrollTop > 0) saved.push({ el: root, top: root.scrollTop });
        var all = root.querySelectorAll ? root.querySelectorAll('*') : [];
        for (var i = 0; i < all.length; i++) {
            if (all[i].scrollTop > 0) saved.push({ el: all[i], top: all[i].scrollTop });
        }
        return saved;
    }

    function restoreScroll(saved) {
        if (!saved) return;
        for (var i = 0; i < saved.length; i++) saved[i].el.scrollTop = saved[i].top;
    }

    return {
        KEY: KEY, AUTOMATIC_SHOWS: AUTOMATIC_SHOWS,
        readPref: readPref, writePref: writePref, deckOn: deckOn,
        shape: shape, slotShows: slotShows, fieldOf: fieldOf,
        rememberScroll: rememberScroll, restoreScroll: restoreScroll,
        BTN: BTN, GAP: GAP, SMALL_BTN: SMALL_BTN, SMALL_GAP: SMALL_GAP, PAD: PAD
    };
}));
