// hotinput.test.js: regression tests for the web client's "keep the command
// bar hot" focus rule (review finding 18, keyboard accessibility).
//
// Run:  node tools/jstest/hotinput.test.js
//
// The client used to redirect focus to the command input on every keydown
// without a Ctrl/Meta/Alt modifier. That made the interface impossible to
// operate without a mouse: Tab could not traverse, Space and Enter could not
// activate a focused button, and arrow keys never reached what was focused.
//
// Dependency-free and assertion-library-free, matching safe-dom.test.js: the
// repo has no JS test runner. Exits non-zero on failure.

var path = require('path');

var HotInput = require(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'hotinput.js'
));

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + got + '\n  want: ' + want);
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

// --- element stubs ---------------------------------------------------------
//
// matches() is implemented by checking the stub's declared descriptors against
// the selector text. Crude on purpose: the point is to exercise the DECISION,
// which is where the bug was, not to reimplement a selector engine.

function el(tagName, opts) {
    opts = opts || {};
    var descriptors = opts.descriptors || [];
    var node = {
        tagName: tagName,
        isContentEditable: !!opts.isContentEditable,
        focusVisible: false,
        classList: {
            contains: function (c) {
                return (opts.classes || []).indexOf(c) !== -1;
            }
        },
        matches: function (selector) {
            // Modelled on Chrome and Edge (checked in both, 2026-11-07): a
            // clicked button is not :focus-visible, but becomes it as soon
            // as any key is pressed, before the page's keydown listener runs.
            if (selector === ':focus-visible') return node.focusVisible;
            for (var i = 0; i < descriptors.length; i++) {
                if (selector.indexOf(descriptors[i]) !== -1) return true;
            }
            return false;
        }
    };
    return node;
}

var BUTTON = el('BUTTON', { descriptors: ['button'] });
var LINK = el('A', { descriptors: ['a[href]'] });
var TABBABLE = el('DIV', { descriptors: ['[tabindex]:not([tabindex="-1"])'] });
var ROLE_BUTTON = el('DIV', { descriptors: ['[role="button"]'] });
var ROLE_TAB = el('DIV', { descriptors: ['[role="tab"]'] });
var PLAIN_DIV = el('DIV', {});
var TEXT_INPUT = el('INPUT', {});
var SELECT = el('SELECT', {});
var CONTENT_EDITABLE = el('DIV', { isContentEditable: true });
var CHAT_TEXTAREA = el('TEXTAREA', {});
var XTERM_TEXTAREA = el('TEXTAREA', { classes: ['xterm-helper-textarea'] });

function key(k, mods) {
    mods = mods || {};
    return {
        key: k,
        ctrlKey: !!mods.ctrl,
        metaKey: !!mods.meta,
        altKey: !!mods.alt
    };
}

function focuses(k, activeElement, modalOpen) {
    return HotInput.shouldFocusCommandBar(k, {
        activeElement: activeElement,
        modalOpen: !!modalOpen,
        document: { body: PLAIN_DIV }
    });
}

// --- isTypingKey -----------------------------------------------------------

check('printable letter is typing', HotInput.isTypingKey(key('n')), true);
check('printable digit is typing', HotInput.isTypingKey(key('5')), true);
check('space IS a printable character', HotInput.isTypingKey(key(' ')), true);

// The keys whose theft broke the interface.
check('Tab is not typing', HotInput.isTypingKey(key('Tab')), false);
check('Enter is not typing', HotInput.isTypingKey(key('Enter')), false);
check('Escape is not typing', HotInput.isTypingKey(key('Escape')), false);
check('ArrowUp is not typing', HotInput.isTypingKey(key('ArrowUp')), false);
check('F1 is not typing', HotInput.isTypingKey(key('F1')), false);

// Bare modifiers used to need a hand-kept list; length !== 1 covers them.
check('Shift is not typing', HotInput.isTypingKey(key('Shift')), false);
check('Control is not typing', HotInput.isTypingKey(key('Control')), false);
check('Meta is not typing', HotInput.isTypingKey(key('Meta')), false);

// Shortcut combos belong to the browser or the app, not the command bar.
check('Ctrl+a is not typing', HotInput.isTypingKey(key('a', { ctrl: true })), false);
check('Meta+v is not typing', HotInput.isTypingKey(key('v', { meta: true })), false);
check('Alt+f is not typing', HotInput.isTypingKey(key('f', { alt: true })), false);

check('a missing key is not typing', HotInput.isTypingKey({}), false);
check('a null event is not typing', HotInput.isTypingKey(null), false);

// --- isEditableField -------------------------------------------------------

check('input is an editable field', HotInput.isEditableField(TEXT_INPUT), true);
check('select is an editable field', HotInput.isEditableField(SELECT), true);
check('contenteditable is an editable field', HotInput.isEditableField(CONTENT_EDITABLE), true);
check('chat textarea is an editable field', HotInput.isEditableField(CHAT_TEXTAREA), true);

// The terminal keeps this invisible textarea focused to receive input; the user
// never chose it, so typing there SHOULD be pulled into the command bar.
check('xterm helper textarea is NOT a field', HotInput.isEditableField(XTERM_TEXTAREA), false);

check('a plain div is not a field', HotInput.isEditableField(PLAIN_DIV), false);
check('null is not a field', HotInput.isEditableField(null), false);

// --- the decision ----------------------------------------------------------

// The behaviour worth keeping: typing after clicking the map still reaches the
// command bar.
check('typing on a plain div focuses the command bar', focuses(key('n'), PLAIN_DIV), true);
check('typing in the xterm textarea focuses the command bar', focuses(key('n'), XTERM_TEXTAREA), true);
check('typing with nothing focused focuses the command bar', focuses(key('n'), null), true);

// The bug. Each of these was previously stolen.
check('Tab is left alone on a button', focuses(key('Tab'), BUTTON), false);
check('Tab is left alone on a plain div', focuses(key('Tab'), PLAIN_DIV), false);
check('Enter is left alone on a button', focuses(key('Enter'), BUTTON), false);
check('Space is left alone on a button', focuses(key(' '), BUTTON), false);
check('Space is left alone on a link', focuses(key(' '), LINK), false);
check('Space is left alone on role=button', focuses(key(' '), ROLE_BUTTON), false);
check('Space is left alone on a tabindex element', focuses(key(' '), TABBABLE), false);
check('ArrowDown is left alone on a button', focuses(key('ArrowDown'), BUTTON), false);
check('Escape is left alone on a button', focuses(key('Escape'), BUTTON), false);

// Even a printable character is left alone on a control the user tabbed to.
check('typing on a focused button is left alone', focuses(key('n'), BUTTON), false);

// --- where focus came from: real event sequences ---------------------------
//
// A button focused by a mouse click or tap (an Automation or Inventory tab, a
// Chat channel, the map Legend) does not own the keys: typing goes to the
// command box instead of vanishing (#768 review item 6). A button reached
// with the keyboard keeps them. HotInput.watch() listens on the document in
// the capture phase; this fake document replays what a browser sends, in the
// browser's order, and the decision is asked the way the page's own keydown
// listener asks it, after the capture listeners have run.

var listeners = {};
var page = {
    body: PLAIN_DIV,
    activeElement: PLAIN_DIV,
    addEventListener: function (type, fn, capture) {
        if (capture !== true) throw new Error('watch() must listen in the capture phase: ' + type);
        (listeners[type] = listeners[type] || []).push(fn);
    }
};
function dispatch(type, target, extra) {
    var e = { type: type, target: target };
    Object.keys(extra || {}).forEach(function (k) { e[k] = extra[k]; });
    (listeners[type] || []).forEach(function (fn) { fn(e); });
    return e;
}
function focusOn(target, fromKeyboard) {
    page.activeElement = target;
    target.focusVisible = !!fromKeyboard;
    dispatch('focusin', target);
}
function click(target) {
    dispatch('pointerdown', target);
    dispatch('mousedown', target);
    focusOn(target, false);
    dispatch('click', target);
}
function tabTo(target) {
    dispatch('keydown', page.activeElement, key('Tab'));
    focusOn(target, true);
}
// press returns whether the page's keydown listener would move the key to
// the command bar.
function press(k) {
    var target = page.activeElement;
    var e = dispatch('keydown', target, key(k));
    // Chrome and Edge: any key makes the focused element :focus-visible
    // before the page listener runs.
    target.focusVisible = true;
    return HotInput.shouldFocusCommandBar(e, {
        activeElement: page.activeElement,
        modalOpen: false,
        document: page
    });
}
HotInput.watch(page);

click(BUTTON);
check('click a button, type n: n reaches the command bar', press('n'), true);
click(ROLE_TAB);
check('click a role=tab, type n: n reaches the command bar', press('n'), true);
click(BUTTON);
press('n');
check('click a button, type two letters: the second also reaches the command bar', press('o'), true);
click(BUTTON);
check('click a button, press Tab: Tab is left alone', press('Tab'), false);
click(BUTTON);
check('click a button, press Enter: Enter is left alone', press('Enter'), false);

tabTo(BUTTON);
check('Tab to a button, press Space: the button keeps it', press(' '), false);
tabTo(BUTTON);
check('Tab to a button, press Enter: the button keeps it', press('Enter'), false);
tabTo(BUTTON);
check('Tab to a button, type n: the button keeps it', press('n'), false);

// Click one control, then Tab on to the next: the new one came from the keyboard.
click(BUTTON);
tabTo(LINK);
check('click, then Tab to a link, press Space: the link keeps it', press(' '), false);
// Tab to a control, then click a different one: the click wins.
tabTo(LINK);
click(ROLE_BUTTON);
check('Tab to a link, then click a role=button, type n: n reaches the command bar', press('n'), true);

// A control focused with no recorded input (a script on load): the older,
// safer rule, the control keeps its keys.
page.activeElement = TABBABLE;
check('a control focused with no input owns its keys', press('n'), false);

// Clicking a plain area still lets typing through, as before.
click(PLAIN_DIV);
check('click the map area, type n: n reaches the command bar', press('n'), true);

// The origin belongs to the element it was recorded for.
click(BUTTON);
page.activeElement = LINK;   // focus moved by something with no focusin seen
check('a recorded click on one control does not free another', press('n'), false);

// Real fields keep their input.
check('typing in a text input is left alone', focuses(key('n'), TEXT_INPUT), false);
check('typing in the chat textarea is left alone', focuses(key('n'), CHAT_TEXTAREA), false);

// Modals own focus while open.
check('typing is left alone while a modal is open', focuses(key('n'), PLAIN_DIV, true), false);

// --- phone keyboard hides after an in-game send ----------------------------
//
// docs/superpowers/plans/2026-11-02-webclient-phone-keyboard.md. True only
// on a touch-only phone-width screen when an in-game command was sent (owner
// ruling on #768, 2026-11-07). Login prompts, password prompts and numbered
// menu answers keep the keyboard, as do tablets, desktops, and a send that
// did not go out.

var HIDE = { touchOnly: true, phoneWidth: true, inGame: true, menuOpen: false, password: false, sent: true };
function hide(overrides) {
    var s = {};
    Object.keys(HIDE).forEach(function (k) { s[k] = HIDE[k]; });
    Object.keys(overrides || {}).forEach(function (k) { s[k] = overrides[k]; });
    return HotInput.shouldHideKeyboardAfterSend(s);
}
check('phone, in game, sent: keyboard hides', hide(), true);
check('not touch-only (desktop, touch laptop): keeps the keyboard', hide({ touchOnly: false }), false);
check('tablet width: keeps the keyboard (arrow history, macros)', hide({ phoneWidth: false }), false);
check('at a login prompt: keeps the keyboard', hide({ inGame: false }), false);
check('answering a numbered game menu: keeps the keyboard', hide({ menuOpen: true }), false);
check('at a password prompt: keeps the keyboard', hide({ password: true }), false);
check('a password prompt at login: keeps the keyboard', hide({ inGame: false, password: true }), false);
check('an old call site with no in-game state: keeps the keyboard',
    HotInput.shouldHideKeyboardAfterSend({ touchOnly: true, phoneWidth: true, sent: true }), false);
check('nothing was sent: keeps the keyboard', hide({ sent: false }), false);
check('no state at all: keeps the keyboard', HotInput.shouldHideKeyboardAfterSend(), false);

// The page has to call it, after SendData and from its result, and the phone
// tap and xterm textarea guards must be in place.
var fs = require('fs');
var page = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'
), 'utf8');
var sendAt = page.indexOf('var sent = SendData(event.target.value);');
var decideAt = page.indexOf('window.HotInput.shouldHideKeyboardAfterSend({');
check('the command box asks HotInput after sending', sendAt >= 0 && decideAt > sendAt, true);
check('the decision is fed the send result', /phoneWidth: phoneWidth\(\), sent: sent/.test(page), true);
check('the decision is told about login, menus and password prompts',
    /inGame: inGame, menuOpen: menuOpen, password: textInput\.type === "password"/.test(page), true);
check('a phone tap on the game text does not refocus the command box',
    /!isDragging && !selectedText && !\(phoneTouchOnly\(\) && phoneWidth\(\)\)/.test(page), true);
check("xterm's textarea cannot raise the phone keyboard",
    /ta\.setAttribute\('inputmode', 'none'\)/.test(page) && /window\.addEventListener\('resize', syncTermKeyboard\)/.test(page), true);
// Closing Settings with a mouse, trackpad or finger leaves no button focused,
// so typing goes to the command box; only a keyboard close returns focus to
// the Settings button (#768 review item 6, reproduced by Malia 2026-11-06).
check('Settings closes back to its button only from the keyboard',
    /function fromKeyboard\(e\) \{ return e\.detail === 0; \}/.test(page)
    && /closeSettings\(fromKeyboard\(e\)\)/.test(page)
    && !/getElementById\('settings-close'\)\.addEventListener\('click', function \(\) \{\s*closeSettings\(true\)/.test(page), true);

// The map's "Legend" button sits in the Fit / Center / zoom row and opens
// the key below it; the old full-width "Map legend" line is gone (Malia
// 2026-11-06).
var controlsAt = page.indexOf('<div id="map-controls"');
var toggleAt = page.indexOf('id="map-legend-toggle"');
check('the Legend button is inside the map button row',
    controlsAt >= 0 && toggleAt > controlsAt && toggleAt < page.indexOf('</div>', controlsAt), true);
check('the old Map legend line is gone', /<summary>Map legend<\/summary>/.test(page), false);

// The Chat tab has no typing box of its own: players chat from the command
// box (Malia 2026-11-06).
check('the Chat tab has no typing box', page.indexOf('chat-input') === -1, true);

// On a phone, an open tab must not cover the command box, and on a laptop
// the command box stays at the bottom even when the Game window is popped out.
var css = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'css', 'dashboard.css'
), 'utf8');
check('an open phone tab is not a fixed 62% of the screen', /height:\s*62vh/.test(css), false);
check('an open phone tab stops above the command box',
    /bottom: calc\(var\(--cmdbox-h\) \+ var\(--tabbar-h\) \+ var\(--tabname-h\)\)/.test(css), true);
check('the command box stays at the bottom of its column',
    /#panel-cmd \{ flex: 0 0 auto; margin-top: auto; \}/.test(css), true);
// The ▾ button folds every tab: the fold must beat each tab's own rules
// (Map, Chat and Automation each had one that kept them open).
check('a folded tab hides its insides on every tab',
    /\.dash-panel\.collapsed \.dash-panel-body \{ display: none !important; \}/.test(css), true);
// On a phone the Automation rows and Inventory items must not squeeze to
// nothing now that phone tabs fit their content.
check('phone tabs keep their Automation rows and Inventory items',
    /#dash-drawer \.inv-grid,\s*#dash-drawer \.auto-list \{\s*flex: 0 1 auto;/.test(css), true);
// The phone ✕ lives in the tab's title bar (no strip of its own), shows on
// phones only, and keeps a wide gap from ▾ against mis-taps (Malia 2026-11-06).
var dash = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'dashboard.js'
), 'utf8');
check('the phone card has no strip just for its close button', /drawer-head/.test(css + dash), false);
check('the close button shows on phones only',
    /\.drawer-close \{\s*display: none;/.test(css) && /#dash-drawer \.drawer-close \{ display: inline-block; \}/.test(css), true);
check('a wide gap keeps the fold and close buttons apart',
    /#dash-drawer \.ph-btns \{ gap: 16px;/.test(css), true);
// Repeatable quests are quieted by colour, never see-through: see-through
// faded them to about 2:1 in Light and Dark mode (#768 review item 3).
var repeatRules = (css.match(/\.(qrepeat|qgroup|qrepeatrow) \{[^}]*\}/g) || []).join('\n');
check('the repeatable quest styles are found', repeatRules.split('\n').length, 3);
check('repeatable quests are not see-through', /opacity/.test(repeatRules), false);
check('the repeatable heading and tag use the quest hint colour',
    /\.qrepeat \{[^}]*color: var\(--quest-progress\)/.test(css) && /\.qgroup \{[^}]*color: var\(--quest-progress\)/.test(css), true);
// The Map may shrink to 150px (master's value) so a short laptop window
// with a few quests does not scroll the whole page (#768 review item 5).
check('the Map can shrink to 150px', /#panel-map\s+\{ flex: 1 1 0; min-height: 150px; \}/.test(css), true);
check('a folded tab drops its own height',
    /min-height: 0 !important;\s*height: auto !important;/.test(css), true);

// --- result ----------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
