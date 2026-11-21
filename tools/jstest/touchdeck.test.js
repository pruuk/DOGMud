// touchdeck.test.js: the touch controls deck's pure functions (touch frame
// plan 1a, docs/superpowers/specs/2026-11-21-touch-frame-design.md).
//
// Run:  node tools/jstest/touchdeck.test.js
'use strict';

var path = require('path');
var TD = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}
function store(init) {
    var m = Object.assign({}, init);
    return {
        getItem: function (k) { return Object.prototype.hasOwnProperty.call(m, k) ? m[k] : null; },
        setItem: function (k, v) { m[k] = String(v); },
        saved: m
    };
}
var blocked = {
    getItem: function () { throw new Error('blocked'); },
    setItem: function () { throw new Error('blocked'); }
};

// The setting, on this device.
check('the storage key', TD.KEY, 'dogmud.touchControls');
check('no choice yet reads Automatic', TD.readPref(store()), 'auto');
check('a saved Always reads back', TD.readPref(store({ 'dogmud.touchControls': 'always' })), 'always');
check('a saved Never reads back', TD.readPref(store({ 'dogmud.touchControls': 'never' })), 'never');
check('junk reads Automatic', TD.readPref(store({ 'dogmud.touchControls': 'sometimes' })), 'auto');
check('blocked storage reads Automatic', TD.readPref(blocked), 'auto');
check('no storage reads Automatic', TD.readPref(null), 'auto');
var s = store();
TD.writePref(s, 'never');
check('a choice is saved', s.saved['dogmud.touchControls'], 'never');
check('junk is saved as Automatic', TD.writePref(store(), 'x'), 'auto');
check('blocked storage does not throw on save', TD.writePref(blocked, 'always'), 'always');

// Whether the deck shows. Owner 2026-11-21: Automatic is off until plan 2b
// puts movement in the deck.
check('the switch plan 2b turns on is still off', TD.AUTOMATIC_SHOWS, false);
check('Automatic on a touch screen: off until plan 2b', TD.deckOn('auto', true), false);
check('Automatic with a mouse: off', TD.deckOn('auto', false), false);
check('Always with a mouse: on', TD.deckOn('always', false), true);
check('Always on touch: on', TD.deckOn('always', true), true);
check('Never on touch: off', TD.deckOn('never', true), false);

// The deck's shape: shape(boxW, boxH, phone, sideways). boxW and boxH are
// the room the deck may fill, inside the slot's side borders.
var p360 = TD.shape(346, 600, true, false);
check('a 360px phone: two rows of six', [p360.rows, p360.cols], [2, 6]);
check('a 360px phone: 48px buttons, 8px gaps', [p360.button, p360.gap], [48, 8]);
check('a 360px phone: 344 x 120', [p360.width, p360.height], [344, 120]);
check('a Pixel 8 keeps six columns (the same slot on every phone)', TD.shape(398, 700, true, false).cols, 6);
var p320 = TD.shape(306, 500, true, false);
check('a 320px phone: six columns of 44px with 5px gaps', [p320.cols, p320.button, p320.gap], [6, 44, 5]);
check('a 320px phone: fits its 306px box', p320.width <= 306, true);
check('a 343px box is too narrow for 48px', TD.shape(343, 600, true, false).button, 44);
check('a 344px box takes 48px', TD.shape(344, 600, true, false).button, 48);
var ipad = TD.shape(806, 1000, false, false);
check('an upright iPad: two rows of as many 48px columns as fit (14)', [ipad.rows, ipad.cols, ipad.button], [2, 14, 48]);
check('an open foldable: 11 columns', TD.shape(676, 800, false, false).cols, 11);
check('a laptop game column: 13 columns', TD.shape(788, 600, false, false).cols, 13);
var narrow = TD.shape(320, 600, false, false);
check('a narrow box that is not a phone falls back to the phone grid', [narrow.cols, narrow.button], [6, 44]);
var side = TD.shape(0, 232, true, true);
check('a phone on its side: a side column, four rows of three', [side.rows, side.cols, side.side], [4, 3, true]);
check('a phone on its side with 232px: 48px', side.button, 48);
var sideShort = TD.shape(0, 220, true, true);
check('a phone on its side with 220px: 44px and it fits', [sideShort.button, sideShort.height <= 220], [44, true]);
check('an upright deck is not a side column', p360.side, false);

// What the slot under the game text shows.
function shows(phone, tabOpen, deckOn, keyboard, field) {
    return TD.slotShows({ phone: phone, tabOpen: tabOpen, deckOn: deckOn, keyboard: keyboard, field: field });
}
check('phone, a tab open: the tab', shows(true, true, true, false, null), 'tab');
check('phone, a tab open, deck off: the tab', shows(true, true, false, false, null), 'tab');
// Plan 1b: typing in the command box sets the tab aside; the game text gets the room.
check('phone, a tab open, typing in the command box: empty', shows(true, true, true, true, 'command'), 'empty');
check('phone, a tab open, deck off, typing in the command box: empty', shows(true, true, false, true, 'command'), 'empty');
check('phone, a tab open, typing in another field: the tab stays', shows(true, true, true, true, 'other'), 'tab');
check('phone, a tab open, command box focused but no keyboard (a hardware keyboard): the tab', shows(true, true, true, false, 'command'), 'tab');
check('deck on, nothing open: the deck', shows(true, false, true, false, null), 'deck');
check('deck on, keyboard up: empty (the keyboard covers the deck)', shows(true, false, true, true, 'command'), 'empty');
check('deck on, keyboard up for another field: empty', shows(true, false, true, true, 'other'), 'empty');
check('deck off, nothing open: empty', shows(true, false, false, false, null), 'empty');
check('not a phone: a tab is a card elsewhere, the slot keeps the deck', shows(false, true, true, false, null), 'deck');
check('not a phone, typing in the command box: empty (the keyboard covers the deck)', shows(false, true, true, true, 'command'), 'empty');
check('not a phone, deck off: empty', shows(false, true, false, false, null), 'empty');

// Which field the player is typing in (plan 1b).
check('nothing editable has focus: no field', TD.fieldOf('command-input', false), null);
check('the command box', TD.fieldOf('command-input', true), 'command');
check('another text field', TD.fieldOf('auto-modal-name', true), 'other');
check('a text field with no id', TD.fieldOf('', true), 'other');

// Plan 1c: a sub-tab row that scrolls sideways fades at the edge with more.
check('all in view: no fade', TD.rowFade(0, 300, 300), { left: false, right: false });
check('more to the right', TD.rowFade(0, 300, 520), { left: false, right: true });
check('scrolled to the end: more to the left', TD.rowFade(220, 300, 520), { left: true, right: false });
check('in the middle: both', TD.rowFade(100, 300, 520), { left: true, right: true });
check('a pixel of rounding is not more', TD.rowFade(219.5, 300, 520), { left: true, right: false });

// Plan 1c: the open sub-tab scrolls into view.
check('already in view: no scroll', TD.scrollLeftToShow(50, 60, 0, 300), 0);
check('off to the right: scroll until its right edge shows', TD.scrollLeftToShow(400, 80, 0, 300), 180);
check('off to the left: scroll to its left edge', TD.scrollLeftToShow(20, 80, 150, 300), 20);
check('wider than the row: show its start', TD.scrollLeftToShow(400, 350, 0, 300), 400);

process.exit(failures ? 1 : 0);
