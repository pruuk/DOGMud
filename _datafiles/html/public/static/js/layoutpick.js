// layoutpick.js: which layout the dashboard uses (owner ruling 2026-11-12).
//
// The game window is on the left, exactly as wide as 80 columns of the
// game text (termfit.js widthFor80, plus its border). The panels take the
// room right of it, and only they change as the browser narrows:
//   wide    Layout D, every panel open (spec ruling 9), only when its square
//           Map would be at least 300px (owner 2026-11-15); width alone needs
//           518px of room for that, and a short frame needs more
//   column  200px or more otherwise: Map, Health and Conditions; the other four on buttons
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
    var COLUMN = 200, SLIM = 120;
    // Wide only when its square Map is at least this big (owner 2026-11-15).
    var MAP_FLOOR = 300;
    // Wide's Map, as dashboard.css --map-w: min(58%, calc((100vh - 192px) * 0.52)),
    // where 58% is of the panel area, whose width is the room. A jstest holds
    // the two together.
    var MAP_SHARE = 0.58, MAP_H_TAKEN = 192, MAP_H_SHARE = 0.52;

    // room: the panel area's width. viewH: the frame's height (100vh here).
    function wideMapSize(room, viewH) {
        return Math.min(MAP_SHARE * room, (viewH - MAP_H_TAKEN) * MAP_H_SHARE);
    }
    // The game column before termfit has measured the letters: 80 columns
    // at 16px (768px), the scrollbar's room (up to 17px) and the border,
    // rounded up so the widest real case (a 125% display, 785px for 80
    // columns plus the border) still gets 80, as in dashboard.css
    // var(--game-w, 790px). The dashboard uses it so a desktop never gets
    // the phone layout only for lack of a measurement.
    var GAME_W_DEFAULT = 790;

    // dashW: #dashboard's width in CSS px. gameW: the game window's width at
    // 80 columns, or null before it is measured. phone: a touch screen whose
    // short side is under 600px (viewportfit.isPhone and pointer: coarse).
    // viewH: the frame's height in CSS px (window.innerHeight in the frame).
    function pick(dashW, gameW, phone, viewH) {
        if (phone || !gameW) return 'phone';
        var room = dashW - 2 * PAD - gameW - GAP;
        if (room >= COLUMN && wideMapSize(room, viewH) >= MAP_FLOOR) return 'wide';
        if (room >= COLUMN) return 'column';
        if (room >= SLIM) return 'slim';
        return 'phone';
    }

    return {
        pick: pick, wideMapSize: wideMapSize,
        PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT,
        MAP_FLOOR: MAP_FLOOR, MAP_SHARE: MAP_SHARE, MAP_H_TAKEN: MAP_H_TAKEN, MAP_H_SHARE: MAP_H_SHARE
    };
}));
