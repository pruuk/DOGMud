// layoutpick.js: which layout the dashboard uses (owner ruling 2026-11-12).
//
// The game window is on the left, exactly as wide as 80 columns of the
// game text (termfit.js widthFor80, plus its border). The panels take the
// room right of it, and only they change as the browser narrows:
//   wide    360px or more: Layout D, every panel open (spec ruling 9)
//   column  200 to 359:    Map, Health and Conditions; the other four on buttons
//   slim    120 to 199:    Health over Conditions; the rest on the tab bar
//   phone   under 120, a browser narrower than the game, or a phone: every
//           panel on the tab bar under the game
// The game window and its text never change between wide, column and slim.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;       // node, for the tests
    } else {
        root.LayoutPick = api;      // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var PAD = 6, GAP = 6;                 // #dashboard padding and the game-to-panels track (dashboard.css)
    var WIDE = 360, COLUMN = 200, SLIM = 120;
    // The game column before termfit has measured the letters: 80 columns
    // at 16px (768px), the scrollbar's room and the border, as in
    // dashboard.css var(--game-w, 786px). The dashboard uses it so a
    // desktop never gets the phone layout only for lack of a measurement.
    var GAME_W_DEFAULT = 786;

    // dashW: #dashboard's width in CSS px. gameW: the game window's width at
    // 80 columns, or null before it is measured. phone: a touch screen whose
    // short side is under 600px (viewportfit.isPhone and pointer: coarse).
    function pick(dashW, gameW, phone) {
        if (phone || !gameW) return 'phone';
        var room = dashW - 2 * PAD - gameW - GAP;
        if (room >= WIDE) return 'wide';
        if (room >= COLUMN) return 'column';
        if (room >= SLIM) return 'slim';
        return 'phone';
    }

    return { pick: pick, PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT };
}));
