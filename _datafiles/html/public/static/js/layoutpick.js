// layoutpick.js: which layout the dashboard uses (owner ruling 2026-11-12).
//
// The game window is on the left, exactly as wide as 80 columns of the
// game text (termfit.js widthFor80, plus its border). The panels take the
// room right of it, and only they change as the browser narrows:
//   wide    Layout D, every panel open (spec ruling 9), only when its square
//           Map would be at least 300px (owner 2026-11-15); width alone needs
//           518px of room for that, and a short frame needs more
//   column  200px or more otherwise: Map, Health and Conditions; the other four on buttons
//           Inside column, columnArrangement lays Map, Health and Conditions
//           out stacked, side by side or swapped (plan 7c)
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

    // The column layout's arrangements (plan 7c, owner 2026-11-15). Column
    // keeps Map, Health and Conditions open; it lays them out one of three
    // ways, whichever gives the larger Map:
    //   stacked : Map, Health, Conditions one above the other (dashboard.css)
    //   side    : Conditions beside the Map, Health in a strip below
    //   swapped : Health beside the Map, Conditions in a strip below, once
    //             Health is taller than a third of the column
    // The floors, the gap and the stacked Map's minimum are dashboard.css's
    // (a jstest holds them together).
    var COL_GAP = 6, VITALS_FLOOR = 120, STATUS_FLOOR = 96, STACKED_MAP_MIN = 120;
    // The panel beside the Map is at least this wide.
    var SIDE_MIN = 160;
    // Health beside the Map once it is taller than this share of the column.
    var SWAP_SHARE = 1 / 3;

    // The Map's size in one arrangement. w, h: the panel area's content box
    // (h is the height above the tab bar). vitalsH: Health's natural height.
    function columnMap(arrangement, w, h, vitalsH) {
        if (arrangement === 'stacked') {
            return Math.floor(Math.min(w, Math.max(STACKED_MAP_MIN, h - 2 * COL_GAP - VITALS_FLOOR - STATUS_FLOOR)));
        }
        var under = arrangement === 'swapped' ? STATUS_FLOOR : vitalsH;
        return Math.floor(Math.min(w - SIDE_MIN - COL_GAP, h - COL_GAP - under));
    }

    // Which arrangement, and its Map size: { arrangement, map }.
    function columnArrangement(w, h, vitalsH) {
        var beside = vitalsH > h * SWAP_SHARE ? 'swapped' : 'side';
        var besideMap = columnMap(beside, w, h, vitalsH);
        var stackedMap = columnMap('stacked', w, h, vitalsH);
        if (besideMap > stackedMap) return { arrangement: beside, map: besideMap };
        return { arrangement: 'stacked', map: stackedMap };
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
        columnArrangement: columnArrangement, columnMap: columnMap,
        PAD: PAD, GAP: GAP, GAME_W_DEFAULT: GAME_W_DEFAULT,
        MAP_FLOOR: MAP_FLOOR, MAP_SHARE: MAP_SHARE, MAP_H_TAKEN: MAP_H_TAKEN, MAP_H_SHARE: MAP_H_SHARE,
        COL_GAP: COL_GAP, VITALS_FLOOR: VITALS_FLOOR, STATUS_FLOOR: STATUS_FLOOR,
        STACKED_MAP_MIN: STACKED_MAP_MIN, SIDE_MIN: SIDE_MIN, SWAP_SHARE: SWAP_SHARE
    };
}));
