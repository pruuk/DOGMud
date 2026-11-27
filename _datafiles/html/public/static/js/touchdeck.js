// touchdeck.js: the touch controls deck (touch arc spec 1, plan 1a;
// docs/superpowers/specs/completed/2026-11-21-touch-frame-design.md).
//
// The deck is a block of buttons above the command box. On a phone it sits
// in the slot under the game text, which holds the deck or the open tab
// (dashboard.js). Buttons only ever come from the server (the roadmap's sync
// rule): plan 2b draws the movement buttons from Room.Info and sends back
// only their ids (Char.Button.Press); the command slots stay empty until
// spec 3.
//
// Pure functions, loaded by the page and by node for the tests:
//   readPref / writePref  the Touch controls setting on this device
//   deckOn                whether the deck shows
//   shape                 rows, columns and sizes for the room it has
//   slotShows             what the slot under the game text holds
//   fieldOf               where the player is typing (plan 1b)
//   rowFade / scrollLeftToShow  a phone's sideways-scrolling sub-tab row (plan 1c)
//   moves / cells / levelAfter / tap / render / focusAfter  moving by touch (plan 2b)
//   lineIcon              a stroke-path icon (the deck and dashboard.js's tab bar)
//   mapShape / barsState / barsStayAfterPick  the Map tab's buttons and the sideways bars (plan 2c)
// Tested by tools/jstest/touchdeck.test.js, touchdeck-moves.test.js, touchdeck-menu-map.test.js and touch-targets.test.js.

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

    // Automatic shows the deck on a touch screen once the deck has buttons
    // (owner ruling 2026-11-21): plan 2b's movement buttons (spec 2,
    // "Turning it on").
    var AUTOMATIC_SHOWS = true;

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

    // pad: the grid's own padding, PAD unless given (the Map tab's grid sits
    // in a card body that has its own).
    function make(rows, cols, btn, gap, side, menu, pad) {
        var p = pad === undefined ? PAD : pad;
        return {
            rows: rows, cols: cols, button: btn, gap: gap, pad: p, side: side, menu: !!menu,
            width: cols * btn + (cols - 1) * gap + 2 * p, height: rows * btn + (rows - 1) * gap + 2 * p
        };
    }

    // The deck's shape for the room it has. boxW, boxH: that room in CSS px,
    // inside the slot's side borders. phone: an upright phone (six columns,
    // so a button keeps its slot on every phone). sideways: a phone on its
    // side (a side column beside the game, four rows of three, whose last
    // slot is Menu: the session bar and tab bar fold away there, #938).
    function shape(boxW, boxH, phone, sideways) {
        if (sideways) {
            var tall = span(SIDE_ROWS, BTN, GAP) <= boxH;
            return make(SIDE_ROWS, SIDE_COLS, tall ? BTN : SMALL_BTN, tall ? GAP : SMALL_GAP, true, true);
        }
        var fit = Math.floor((boxW - 2 * PAD + GAP) / (BTN + GAP));
        if (phone || fit < PHONE_COLS) {
            var wide = span(PHONE_COLS, BTN, GAP) <= boxW;
            return make(ROWS, PHONE_COLS, wide ? BTN : SMALL_BTN, wide ? GAP : SMALL_GAP, false, false);
        }
        return make(ROWS, fit, BTN, GAP, false, false);
    }

    // The Map tab's buttons beside the map (plan 2c, spec 2c, mockup frame
    // 7), at the deck's own button size (one size rule, guard 6). Upright:
    // three rows of three, the exits button under the T (160px wide at
    // 48px). Sideways: the Map card is too short for three rows, so two
    // rows of four, the exits button beside the T. No padding of its own:
    // the Map card's body has some (160px wide upright, 104px tall sideways).
    function mapShape(deck, sideways) {
        return sideways ? make(2, 4, deck.button, deck.gap, false, false, 0)
                        : make(3, 3, deck.button, deck.gap, false, false, 0);
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

    // The hold after a tap that changes what is under the finger (roadmap
    // fat-finger rules, plan 2b D11). A tap on the exits button, Back or
    // More, or on an exit that sends, redraws the deck or moves the player,
    // so a different button lands under the finger: every tap in the next
    // DOUBLE_TAP_MS is ignored. A T tap sets no hold, so a fast double tap
    // on the T walks twice, like typing (owner ruling 8).
    var DOUBLE_TAP_MS = 500;

    // The Dark tile types this, and the server answers with its own
    // no-sight line (actions.NoSightLine), worded and wrapped there (owner
    // ruling 2026-11-26: the client writes no text of its own for the deck).
    var DARK_COMMAND = 'look';

    // The deck's cells for one model, in slot order (row by row), for a
    // shape from shape(). level: 'top' (the T) or 'exits' (the other-exits
    // list, which replaces the whole deck: Back top-left, More in the last
    // slot only when the list does not fit; on the side column that is the
    // slot before Menu). page: which page of the list.
    // kind: 'empty', 'move', 'other', 'dark', 'back', 'exit', 'more' or
    // 'menu' (the side column's last slot, when s.menu).
    function cells(model, s, level, page) {
        var n = s.rows * s.cols, room = s.menu ? n - 1 : n, out = [], i;
        for (i = 0; i < n; i++) out.push({ kind: 'empty' });
        // Menu keeps the side column's last slot in every state (plan 2c
        // D3): Leave Game and Settings stay in reach before the first
        // Room.Info, in the dark and in the exits list.
        if (s.menu) out[n - 1] = { kind: 'menu' };
        if (!model || !model.ready) return out;
        if (level === 'exits' && !model.dark && model.others.length) {
            var more = model.others.length > room - 1;
            var per = more ? room - 2 : room - 1;
            var pages = Math.ceil(model.others.length / per);
            var p = (((page || 0) % pages) + pages) % pages;
            out[0] = { kind: 'back' };
            model.others.slice(p * per, p * per + per).forEach(function (o, k) {
                out[1 + k] = {
                    kind: 'exit', id: o.id, name: o.name, label: o.label, arrow: o.arrow,
                    locked: o.locked, own: o.own, combat: model.combat
                };
            });
            if (more) out[room - 1] = { kind: 'more' };
            return out;
        }
        T_DIRS.forEach(function (dir) {
            var t = model.t[dir], at = T_PLACE[dir], a = own(ARROWS, dir) ? ARROWS[dir] : null;
            out[at[0] * s.cols + at[1]] = {
                kind: 'move', dir: dir, id: t.id,
                arrow: a ? a[0] : '', label: a ? a[1] : '', icon: a ? '' : STAIRS[dir],
                lit: t.lit, locked: t.locked, mark: t.mark, dark: model.dark, combat: model.combat
            };
        });
        // The other-exits slot: beside the T on a two-row deck, under it in
        // a sideways phone's three-column side column (mockup frame 9).
        var slot = s.cols >= 4 ? 3 : 2 * s.cols;
        out[slot] = model.dark ? { kind: 'dark' }
            : { kind: 'other', lit: model.others.length > 0, combat: model.combat };
        return out;
    }

    // The level after a Room.Info: the T after a move (another room), in
    // the dark, or once the room has no other exits; otherwise as it was,
    // so a door unlocking while the list is open keeps it open.
    function levelAfter(level, lastNum, num, model) {
        if (level !== 'exits' || lastNum !== num) return 'top';
        if (!model.ready || model.dark || !model.others.length) return 'top';
        return 'exits';
    }

    // What a tap on cell c does. st: { level, page, guardUntil }; now in ms.
    // Returns the next state with send (an id for Char.Button.Press, or '')
    // typed (a command to send as if typed, or '') and menu (true: show or
    // fold the bars, #938). A tap before guardUntil is ignored
    // (DOUBLE_TAP_MS).
    function tap(st, c, now) {
        var r = { level: st.level, page: st.page, guardUntil: st.guardUntil || 0, send: '', typed: '', menu: false };
        if (!c || now < r.guardUntil) return r;
        switch (c.kind) {
        case 'move':
            if ((c.lit || c.dark) && c.id) r.send = c.id;
            break;
        case 'exit':
            if (!c.id) break;
            r.send = c.id;
            r.guardUntil = now + DOUBLE_TAP_MS;
            break;
        case 'other':
            if (c.lit) { r.level = 'exits'; r.page = 0; r.guardUntil = now + DOUBLE_TAP_MS; }
            break;
        case 'back':
            r.level = 'top';
            r.page = 0;
            r.guardUntil = now + DOUBLE_TAP_MS;
            break;
        case 'more':
            r.page = (st.page || 0) + 1;
            r.guardUntil = now + DOUBLE_TAP_MS;
            break;
        case 'menu':
            // The bars (#938): dashboard.js shows or folds them. No hold
            // (plan 2c D10): nothing new lands under the finger in the deck.
            r.menu = true;
            break;
        case 'dark':
            r.typed = DARK_COMMAND;
            break;
        }
        return r;
    }

    var SVG_NS = 'http://www.w3.org/2000/svg';

    // Line icons as stroke paths, like the tab bar's (dashboard.js
    // PANEL_TABS), never emoji (brand). Drawn from the approved mockup: the
    // ISO 7001 style stairs (ruling 10), the padlock (ruling 3) and the
    // closed eye of the Dark tile (ruling 11), and Menu's three lines (plan
    // 2c D12).
    var ICONS = {
        'stairs-up': { box: 26, paths: ['M3 22h5v-5h5v-5h5V7h5', 'M16 4h6v6'] },
        'stairs-down': { box: 26, paths: ['M3 7h5v5h5v5h5v5h5', 'M22 16v6h-6'] },
        lock: { box: 12, paths: ['M3 5.5h6a1 1 0 0 1 1 1v3.5a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1z',
                                 'M4 5.5V4a2 2 0 0 1 4 0v1.5'] },
        eye: { box: 26, paths: ['M3 12c3 4 7 6 10 6s7-2 10-6', 'M6 16l-2 3M10 18l-1 3.5M16 18l1 3.5M20 16l2 3'] },
        menu: { box: 26, paths: ['M5 8h16', 'M5 13h16', 'M5 18h16'] }
    };

    // One line icon: stroke paths in a box-wide square, hidden from screen
    // readers (its button carries the words). The client's one builder for
    // them: the deck's icons and the tab bar's round tabs (dashboard.js
    // makeTabBar) both draw through it. cls is optional.
    function lineIcon(doc, box, paths, cls) {
        var svg = doc.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 ' + box + ' ' + box);
        if (cls) svg.setAttribute('class', cls);
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        paths.forEach(function (p) {
            var path = doc.createElementNS(SVG_NS, 'path');
            path.setAttribute('d', p);
            svg.appendChild(path);
        });
        return svg;
    }

    function svgIcon(doc, name, cls) {
        var d = ICONS[name];
        return lineIcon(doc, d.box, d.paths, cls);
    }

    function textEl(doc, cls, text) {
        var e = doc.createElement('span');
        e.className = cls;
        e.textContent = text;
        return e;
    }

    // Kinds are told apart by shape, edge and marks (CSS), never by colour
    // alone. Every button but the pitch-dark ones wears the site's brass
    // (.brass, gomud.css; owner ruling R1 on plan 2b).
    function classOf(c) {
        var k = ['deck-btn'];
        if (!c.dark && c.kind !== 'dark') k.push('brass');
        if (c.kind === 'move') {
            k.push('deck-move');
            if (c.dark) k.push('deck-blind'); else if (!c.lit) k.push('deck-dim');
            if (c.mark === 'wall') k.push('deck-mark-wall');
            if (c.combat && !c.dark) k.push('deck-combat');
        } else if (c.kind === 'other') {
            k.push('deck-other');
            if (!c.lit) k.push('deck-dim');
            if (c.combat) k.push('deck-combat');
        } else if (c.kind === 'exit') {
            k.push('deck-exit');
            if (c.combat) k.push('deck-combat');
        } else if (c.kind === 'dark') {
            k.push('deck-darktile');
        } else if (c.kind === 'back') {
            k.push('deck-back');
        } else if (c.kind === 'more') {
            k.push('deck-more');
        } else if (c.kind === 'menu') {
            k.push('deck-menu');
        }
        return k.join(' ');
    }

    // A screen reader's words for a button, plain (dogmud-player-copy).
    function ariaOf(c) {
        switch (c.kind) {
        case 'move':
            if (!c.dark && !c.lit) return 'No way ' + c.dir;
            if (c.mark === 'wall') return 'Go ' + c.dir + ' (you hit a wall there)';
            if (c.locked || c.mark === 'locked') return 'Go ' + c.dir + ' (locked)';
            return 'Go ' + c.dir;
        case 'other': return c.lit ? 'Other exits' : 'No other exits';
        case 'exit': return 'Go ' + (c.arrow ? c.name : c.label) + (c.own ? ' (yours)' : '') + (c.locked ? ' (locked)' : '');
        case 'dark': return 'Too dark to see the exits';
        case 'back': return 'Back to the directions';
        case 'more': return 'More exits';
        case 'menu': return 'Menu';
        }
        return '';
    }

    // A lit room's arrow with no exit that way, an arrow with no id (a tap
    // would send nothing), and the exits button with no other exits take no
    // tap. In the dark all six arrows do (ruling 4).
    function disabledOf(c) {
        return (c.kind === 'move' && ((!c.lit && !c.dark) || !c.id)) || (c.kind === 'other' && !c.lit);
    }

    // Back's and More's words; touch-targets.test.js checks they fit their
    // heavy-edged buttons at both sizes.
    // No space beside the arrow (Malia, S21 check 2026-11-26): in the
    // monospaced deck font a space is a whole letter wide.
    var BACK_LABEL = '‹Back', MORE_LABEL = 'More›';

    // Draws cells into the deck element, replacing what it held. Text goes
    // in as textContent, never as markup (an exit name is server data). Each
    // button carries its cell's index in data-cell for the click handler
    // (dashboard.js).
    function render(deck, list, doc) {
        while (deck.firstChild) deck.removeChild(deck.firstChild);
        list.forEach(function (c, i) {
            if (c.kind === 'empty') {
                deck.appendChild(textEl(doc, 'deck-slot', ''));
                return;
            }
            var b = doc.createElement('button');
            b.type = 'button';
            b.className = classOf(c);
            b.setAttribute('data-cell', String(i));
            b.setAttribute('aria-label', ariaOf(c));
            if (disabledOf(c)) b.disabled = true;
            if (c.icon) b.appendChild(svgIcon(doc, c.icon, 'deck-icon'));
            if (c.arrow) {
                b.appendChild(textEl(doc, 'deck-arrow', c.arrow));
                b.appendChild(textEl(doc, 'deck-letter', c.label));
            } else if (c.kind === 'exit') {
                b.appendChild(textEl(doc, 'deck-name', c.label));
            }
            if (c.kind === 'other') {
                b.appendChild(textEl(doc, 'deck-name', '⋯'));
                b.appendChild(textEl(doc, 'deck-name', 'exits'));
            } else if (c.kind === 'dark') {
                b.appendChild(svgIcon(doc, 'eye', 'deck-icon'));
                b.appendChild(textEl(doc, 'deck-name', 'Dark'));
            } else if (c.kind === 'back') {
                b.appendChild(textEl(doc, 'deck-name', BACK_LABEL));
            } else if (c.kind === 'more') {
                b.appendChild(textEl(doc, 'deck-name', MORE_LABEL));
            } else if (c.kind === 'menu') {
                b.appendChild(svgIcon(doc, 'menu', 'deck-icon'));
                b.appendChild(textEl(doc, 'deck-name', 'Menu'));
                // What it opens. It is drawn closed; dashboard.js updates
                // aria-expanded in place on this button after drawing, so the
                // deck is not rebuilt (#938).
                b.setAttribute('aria-controls', 'dash-session dash-tabbar');
                b.setAttribute('aria-expanded', 'false');
            }
            if (c.locked || c.mark === 'locked') b.appendChild(svgIcon(doc, 'lock', 'deck-lock'));
            deck.appendChild(b);
        });
    }

    // Which cell takes the focus after a redraw, when the focus was on cell
    // `cell` inside the deck (render rebuilds every button, so the focused
    // one is gone; keyboard and screen reader users would land on the page
    // body). Opening the list focuses Back, closing it the exits button;
    // otherwise the same cell while it is still a live button, else the
    // first live button. -1 when the deck has none.
    function focusAfter(list, cell, levelWas, level) {
        function live(i) { var c = list[i]; return !!c && c.kind !== 'empty' && !disabledOf(c); }
        var i, want = cell;
        if (levelWas !== level) {
            var kind = level === 'exits' ? 'back' : 'other';
            for (i = 0; i < list.length; i++) if (list[i].kind === kind) { want = i; break; }
        }
        if (live(want)) return want;
        for (i = 0; i < list.length; i++) if (live(i)) return i;
        return -1;
    }

    // ---- The sideways bars (#938, plan 2c) --------------------------------
    // A sideways phone with the deck on folds the session bar and the tab
    // bar away; Menu brings both back over the game. folds: the deck shows
    // Menu now. shown: Menu brought them back. 'folded', 'shown', or '' for
    // the bars as always (dashboard.js sets it as #dashboard's data-bars).
    function barsState(folds, shown) {
        if (!folds) return '';
        return shown ? 'shown' : 'folded';
    }

    // After a click on a button in the bars: they fold again once the player
    // has picked something, unless the pick left one of the bars' own
    // dialogs open: the Settings menu, or Leave Game's question (plan 2c D5).
    function barsStayAfterPick(settingsOpen, confirmOpen) {
        return !!(settingsOpen || confirmOpen);
    }

    return {
        KEY: KEY, AUTOMATIC_SHOWS: AUTOMATIC_SHOWS,
        readPref: readPref, writePref: writePref, deckOn: deckOn,
        shape: shape, slotShows: slotShows, fieldOf: fieldOf,
        rowFade: rowFade, scrollLeftToShow: scrollLeftToShow,
        moves: moves, T_DIRS: T_DIRS, DIAGONALS: DIAGONALS, NAME_LETTERS: NAME_LETTERS,
        cells: cells, levelAfter: levelAfter, tap: tap, DOUBLE_TAP_MS: DOUBLE_TAP_MS,
        render: render, lineIcon: lineIcon, focusAfter: focusAfter, BACK_LABEL: BACK_LABEL, MORE_LABEL: MORE_LABEL,
        mapShape: mapShape, barsState: barsState, barsStayAfterPick: barsStayAfterPick,
        BTN: BTN, GAP: GAP, SMALL_BTN: SMALL_BTN, SMALL_GAP: SMALL_GAP, PAD: PAD
    };
}));
