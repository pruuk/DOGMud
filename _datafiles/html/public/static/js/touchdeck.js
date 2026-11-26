// touchdeck.js: the touch controls deck (touch arc spec 1, plan 1a;
// docs/superpowers/specs/completed/2026-11-21-touch-frame-design.md).
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
//   rowFade / scrollLeftToShow  a phone's sideways-scrolling sub-tab row (plan 1c)
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

    // A sub-tab row that scrolls sideways on a phone (plan 1c) fades at each
    // edge with more to scroll. Measures from the row element; a pixel of
    // rounding is not more.
    function rowFade(scrollLeft, clientWidth, scrollWidth) {
        return {
            left: scrollLeft > 1,
            right: scrollLeft + clientWidth < scrollWidth - 1
        };
    }

    // The row's scrollLeft that shows an item: itemLeft and itemWidth in the
    // row's scroll coordinates. Unchanged when it is already in view; an item
    // wider than the row shows its start.
    function scrollLeftToShow(itemLeft, itemWidth, scrollLeft, rowWidth) {
        if (itemLeft < scrollLeft || itemWidth >= rowWidth) return itemLeft;
        if (itemLeft + itemWidth > scrollLeft + rowWidth) return itemLeft + itemWidth - rowWidth;
        return scrollLeft;
    }

    // ---- Moving by touch (plan 2b, #781) ---------------------------------
    // Spec docs/superpowers/specs/2026-11-22-moving-by-touch-design.md;
    // mockup docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html. The
    // server builds every button (Room.Info exitsv2, moves, darkmarks and
    // dark, plan 2a); the client lays them out and sends back only an id
    // (Char.Button.Press).

    function own(o, k) { return !!o && Object.prototype.hasOwnProperty.call(o, k); }

    // The inverted T: each direction's row and column in the T block.
    var T_DIRS = ['up', 'north', 'down', 'west', 'south', 'east'];
    var T_PLACE = { up: [0, 0], north: [0, 1], down: [0, 2], west: [1, 0], south: [1, 1], east: [1, 2] };

    // Ruling 10: an arrow with its letter for compass and diagonal exits;
    // up and down draw the stairs pictogram (ICONS).
    var ARROWS = {
        north: ['↑', 'N'], south: ['↓', 'S'], east: ['→', 'E'], west: ['←', 'W'],
        northeast: ['↗', 'NE'], southeast: ['↘', 'SE'],
        southwest: ['↙', 'SW'], northwest: ['↖', 'NW']
    };
    var STAIRS = { up: 'stairs-up', down: 'stairs-down' };

    // Other exits come in a fixed order: the diagonals clockwise from
    // northeast, then every other exit A to Z by its label. The server sorts
    // the same way (rooms.exitOrder, internal/rooms/known_exits.go), but
    // Room.Info sends exitsv2 as a JSON object, which keeps no order, so the
    // client sorts again; touchdeck-moves.test.js fails if the two part.
    var DIAGONALS = ['northeast', 'southeast', 'southwest', 'northwest'];

    // The longest exit name, exit.MaxNameLetters (owner ruling 18);
    // internal/guards checks the two agree, and touch-targets.test.js that a
    // name this long fits a button at both sizes.
    var NAME_LETTERS = 6;

    function hasDetail(e, d) { return !!(e && e.details && e.details.indexOf(d) !== -1); }

    function diagonalRank(name) {
        var i = DIAGONALS.indexOf(name);
        return i < 0 ? DIAGONALS.length : i;
    }
    function otherCompare(a, b) {
        var r = diagonalRank(a.name) - diagonalRank(b.name);
        if (r) return r;
        var la = a.label.toLowerCase(), lb = b.label.toLowerCase();
        if (la !== lb) return la < lb ? -1 : 1;
        // Two portals to one zone share a label: the player's own first
        // (plan 2b D3), then by key.
        if (a.own !== b.own) return a.own ? -1 : 1;
        return a.name < b.name ? -1 : (a.name > b.name ? 1 : 0);
    }

    // The deck's view of one Room.Info (null before the first). mode:
    // Commands.State's mode ('exploration', 'combat', 'dead').
    //   ready   a Room.Info has come
    //   dark    the player sees nothing here: all six T buttons tappable
    //   combat  movement dims (the server refuses it anyway)
    //   t       per T direction: id (its fixed id), lit (an exit you know
    //           that way), locked (locked now), mark ('wall' or 'locked',
    //           in the dark only)
    //   others  the other exits that have a button, in order: name (what
    //           the server runs), id, label, arrow (diagonals), locked,
    //           own (the player's own instance portal, detail own_portal)
    function moves(info, mode) {
        var m = { ready: !!info, dark: !!(info && info.dark), combat: mode === 'combat', t: {}, others: [] };
        var v2 = (info && info.exitsv2) || {}, ids = (info && info.moves) || {}, marks = (info && info.darkmarks) || {};
        T_DIRS.forEach(function (dir) {
            var e = own(v2, dir) ? v2[dir] : null;
            m.t[dir] = {
                id: own(ids, dir) ? ids[dir] : '',
                lit: !m.dark && !!e,
                locked: !m.dark && hasDetail(e, 'locked'),
                mark: m.dark && own(marks, dir) ? marks[dir] : ''
            };
        });
        if (m.dark) return m;
        Object.keys(v2).forEach(function (name) {
            var e = v2[name];
            if (own(T_PLACE, name) || !e || !e.id) return;
            var a = own(ARROWS, name) ? ARROWS[name] : null;
            m.others.push({
                name: name,
                id: e.id,
                label: a ? a[1] : (e.label || name),
                arrow: a ? a[0] : '',
                locked: hasDetail(e, 'locked'),
                own: hasDetail(e, 'own_portal')
            });
        });
        m.others.sort(otherCompare);
        return m;
    }

    return {
        KEY: KEY, AUTOMATIC_SHOWS: AUTOMATIC_SHOWS,
        readPref: readPref, writePref: writePref, deckOn: deckOn,
        shape: shape, slotShows: slotShows, fieldOf: fieldOf,
        rowFade: rowFade, scrollLeftToShow: scrollLeftToShow,
        moves: moves, T_DIRS: T_DIRS, DIAGONALS: DIAGONALS, NAME_LETTERS: NAME_LETTERS,
        BTN: BTN, GAP: GAP, SMALL_BTN: SMALL_BTN, SMALL_GAP: SMALL_GAP, PAD: PAD
    };
}));
