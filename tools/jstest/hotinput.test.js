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

// --- phone keyboard after a send (plan 6) ------------------------------------
//
// docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md. On a
// touch-only phone, in game, after a send: chat keeps the keyboard (R1);
// anything else waits for the server's Char.Input.Done (R2, R5, R6). Login,
// password and menu answers keep it, as do tablets, desktops and a send that
// did not go out (owner ruling on #768, 2026-11-07).

var AFTER = { touchOnly: true, phoneWidth: true, inGame: true, menuOpen: false, password: false, sent: true, line: 'look' };
function after(overrides) {
    var s = {};
    Object.keys(AFTER).forEach(function (k) { s[k] = AFTER[k]; });
    Object.keys(overrides || {}).forEach(function (k) { s[k] = overrides[k]; });
    return HotInput.keyboardAfterSend(s);
}
check('phone, in game, look: waits for the game', after(), 'wait');
check('talk to an NPC: waits (its menu keeps the keyboard)', after({ line: 'talk hadwen' }), 'wait');
check('say: keeps the keyboard', after({ line: 'say hi' }), 'keep');
check('not touch-only (desktop, touch laptop): keeps the keyboard', after({ touchOnly: false }), 'keep');
check('tablet width: keeps the keyboard (arrow history, macros)', after({ phoneWidth: false }), 'keep');
check('at a login prompt: keeps the keyboard', after({ inGame: false }), 'keep');
check('answering a numbered game menu: keeps the keyboard', after({ menuOpen: true }), 'keep');
check('at a password prompt: keeps the keyboard', after({ password: true }), 'keep');
check('nothing was sent: keeps the keyboard', after({ sent: false }), 'keep');
check('no state at all: keeps the keyboard', HotInput.keyboardAfterSend(), 'keep');
check('the old name is gone', typeof HotInput.shouldHideKeyboardAfterSend, 'undefined');

// R1: chat words, as typed. Aliases and macros are out (ruling 13).
['say hi', 'whisper bob hi', 'reply ok', 'chat hello', 'newbie help?', 'trade wts rope',
 'shout hey', 'emote waves', 'yell hey', 'scream hey', 'holler hey', '/w bob hi',
 '. hi', '.hi', '` hi', '`hi', 'SAY hi', '  say hi  ', 'party chat hi', 'party say hi',
 'guild chat hi', 'guild say hi', 'guild gc hi', 'PARTY CHAT hi',
 'gc hi', 'pchat hi', 'psay hi'
].forEach(function (line) { check('chat line: ' + JSON.stringify(line), HotInput.isChatLine(line), true); });

// Guard against drift: every engine alias of a chat command is a chat line.
// Line-based read of the command-aliases block in keywords.yaml.
(function () {
    var yaml = require('fs').readFileSync(path.join(
        __dirname, '..', '..', '_datafiles', 'world', 'dogmud', 'keywords.yaml'), 'utf8').split(/\r?\n/);
    var CHAT_TARGETS = ['say', 'chat', 'shout', 'whisper', 'emote', 'reply', 'newbie', 'trade',
                        'party chat', 'party say', 'guild chat', 'guild say'];
    var inBlock = false, found = 0;
    yaml.forEach(function (ln) {
        if (/^command-aliases:/.test(ln)) { inBlock = true; return; }
        if (!inBlock) return;
        if (ln !== '' && !/^\s/.test(ln)) { inBlock = false; return; }
        var m = /^\s+(?:'([^']+)'|"([^"]+)"|([^:\s'"][^:]*?))\s*:\s*\[(.*)\]/.exec(ln);
        if (!m) return;
        var key = m[1] || m[2] || m[3];
        if (CHAT_TARGETS.indexOf(key) < 0) return;
        (m[4].match(/'([^']*)'|"([^"]*)"/g) || []).forEach(function (q) {
            var alias = q.slice(1, -1);
            found++;
            check('engine alias of ' + key + ': ' + JSON.stringify(alias),
                HotInput.isChatLine(alias + ' hi'), true);
        });
    });
    check('the alias scan found at least 8 aliases (found ' + found + ')', found >= 8, true);
})();
['look', 'talk hadwen', 'ask hadwen about rope', 'party invite bob', 'party', 'party loot ffa',
 'guild deposit 5', 'guild', 'sayings', 'emoted', '', '   ', null, undefined
].forEach(function (line) { check('not a chat line: ' + JSON.stringify(line), HotInput.isChatLine(line), false); });

// After the wait: close only if no menu, the box still has focus, and the
// player has not started the next command.
check('wait over, no menu, focused, empty: close', HotInput.blurAfterWait({ menu: false, focused: true, value: '' }), true);
check('a menu opened: keep', HotInput.blurAfterWait({ menu: true, focused: true, value: '' }), false);
check('focus moved away: leave it', HotInput.blurAfterWait({ menu: false, focused: false, value: '' }), false);
check('already typing the next command: keep', HotInput.blurAfterWait({ menu: false, focused: true, value: 'n' }), false);

// The wait matches the server's marker by line number (R7), with a ceiling.
(function () {
    var now = 0, timers = [], settled = [];
    function fakeSet(f, ms) { var t = { at: now + ms, f: f, live: true }; timers.push(t); return t; }
    function fakeClear(t) { if (t) t.live = false; }
    function advance(ms) {
        now += ms;
        timers.forEach(function (t) { if (t.live && t.at <= now) { t.live = false; t.f(); } });
    }
    var menuOpenNow = false;
    var w = HotInput.createSendWait({
        setTimeout: fakeSet, clearTimeout: fakeClear, ceilingMs: HotInput.MENU_WAIT_CEILING_MS,
        currentMenu: function () { return menuOpenNow; },
        settle: function (menu) { settled.push(menu); }
    });
    check('the ceiling is 2 seconds', HotInput.MENU_WAIT_CEILING_MS, 2000);

    w.start(5);
    w.done(4, true);
    check("an earlier line's marker does not end the wait", settled.length, 0);
    w.done('5', false);
    w.done(undefined, false);
    check('a non-number seq is ignored', settled.length, 0);
    w.done(5, true);
    check('its own marker ends it with that menu state', JSON.stringify(settled), '[true]');
    advance(3000);
    check('the ceiling does not fire after the marker', settled.length, 1);

    w.start(6);
    w.done(8, false);
    menuOpenNow = true;
    check('a later number means the line never reached the world: ends at once',
        JSON.stringify(settled), '[true,false]');

    w.start(9);
    menuOpenNow = true;
    advance(1999);
    check('no marker: still waiting at 1999ms', settled.length, 2);
    advance(1);
    check('no marker: the ceiling settles with the current menu state', JSON.stringify(settled), '[true,false,true]');

    w.start(10);
    w.start(11);
    w.done(10, false);
    check('a new wait replaces the old one', settled.length, 3);
    w.done(11, false);
    check('the new wait ends on its own marker', JSON.stringify(settled), '[true,false,true,false]');

    w.start(12);
    w.reset();
    advance(5000);
    w.done(12, true);
    check('a new socket clears the wait', settled.length, 4);
})();

// The page has to call it, after SendData and from its result, and the phone
// tap and xterm textarea guards must be in place.
var fs = require('fs');
var page = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'
), 'utf8');
var sendAt = page.indexOf('var sent = SendData(line);');
var decideAt = page.indexOf('window.HotInput.keyboardAfterSend({');
check('the command box asks HotInput after sending', sendAt >= 0 && decideAt > sendAt, true);
check('the decision is fed the send result and the typed line',
    /phoneWidth: phoneWidth\(\), sent: sent,[\s\S]{0,200}line: line/.test(page), true);
check('the decision is told about login, menus and password prompts',
    /inGame: inGame, menuOpen: menuOpen, password: textInput\.type === "password"/.test(page), true);
check('a wait starts with the line number SendData gave it',
    /=== 'wait'\) \{\s*sendWait\.start\(linesSent\);/.test(page), true);
check('every string sent on the socket is counted (both ends count text frames, R7)',
    (page.match(/socket\.send\(/g) || []).length === 3
    && /linesSent\+\+;\s*socket\.send\(t\);/.test(page)
    && /linesSent\+\+;\s*socket\.send\(dataToSend\);/.test(page), true);
check('a new socket starts the count again',
    /socket\.onopen = function\(\) \{\s*linesSent = 0;\s*if \(sendWait\) sendWait\.reset\(\);/.test(page), true);
check('Char.Input.Done ends the wait by number',
    /"Char\.Input\.Done": function\(\) \{[\s\S]{0,400}sendWait\.done\(d\.seq, d\.menu === true\)/.test(page), true);
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
// The key starts folded on every screen (owner 2026-11-13).
check('the map legend starts folded in the markup',
    /<div id="map-legend" class="map-legend" hidden>/.test(page)
    && /id="map-legend-toggle"[^>]*aria-expanded="false"/.test(page), true);
check('the map legend starts folded at any window height',
    /initMapLegend[\s\S]*?show\(false\);/.test(page)
    && !/show\(window\.innerHeight/.test(page), true);
// Touch screens have no map button row (Calabe 2026-11-15), so the key opens
// from a Legend button in the Map's title bar and lies over the map, taking
// no height from it (owner 2026-11-15, after the phone lost the key).
check('the Map title bar has a Legend button for the key',
    /id="panel-map"[\s\S]*?<span class="ph-btns"><button type="button" class="brass map-legend-head" aria-controls="map-legend" aria-expanded="false">Legend<\/button>/.test(page), true);
check('both Legend buttons open the same key',
    /initMapLegend[\s\S]*?querySelector\('#panel-map \.map-legend-head'\)/.test(page), true);
check('a tap on the key laid over the map folds it',
    /initMapLegend[\s\S]*?legend\.addEventListener\('click'/.test(page), true);

// The Chat tab has no typing box of its own: players chat from the command
// box (Malia 2026-11-06).
check('the Chat tab has no typing box', page.indexOf('chat-input') === -1, true);

// On a phone, an open tab must not cover the command box, and on a laptop
// the command box stays at the bottom even when the Game window is popped out.
var css = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'css', 'dashboard.css'
), 'utf8');
// The touch Legend (owner 2026-11-15; markup checks above, with the key's).
check('touch screens hide the map button row but keep the key',
    /@media \(pointer: coarse\) \{\s*#panel-map \.map-controls \{ display: none; \}/.test(css)
    && !/#panel-map \.map-legend \{ display: none; \}/.test(css), true);
check('the title bar Legend button shows on touch screens only',
    /\.map-legend-head \{ display: none; \}/.test(css)
    && /@media \(pointer: coarse\) \{[\s\S]*?#panel-map \.map-legend-head \{\s*display: inline-flex;/.test(css), true);
check('on touch the key lies over the map',
    /@media \(pointer: coarse\) \{[\s\S]*?#panel-map \.map-legend \{\s*position: absolute;/.test(css), true);
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
// Plan 7, rulings 18 and 19, P4: Quests has Active, Repeat and Completed tabs.
check('the page loads questtabs.js', /static\/js\/questtabs\.js\?v=/.test(page), true);
check('the Quests panel has its three tabs in the existing subtab pattern',
    /<div id="quest-tabs" class="subtabs" role="group" aria-label="Quests">\s*<button type="button" class="subtab active" aria-pressed="true" data-qtab="active">Active<\/button>\s*<button type="button" class="subtab" aria-pressed="false" data-qtab="repeat">Repeat<\/button>\s*<button type="button" class="subtab" aria-pressed="false" data-qtab="done">Completed<\/button>\s*<\/div>/.test(page), true);
check('the old "Repeatable work" heading is gone', /Repeatable work/.test(page), false);
check('Repeat rows end "Ready now" or "Not yet"', /'Ready now' : 'Not yet'/.test(page), true);
check('the quest list uses the game window line spacing',
    /\.qrow \.qname \{[^}]*line-height:\s*1\.2;/.test(css) && /\.qhint \{[^}]*line-height:\s*1\.2;/.test(css) && /\.qprogress \{[^}]*line-height:\s*1\.2;/.test(css), true);
check('a not-ready Repeat row and a Completed row are dimmed by colour, never see-through',
    /\.qrepeatrow:not\(\.ready\)[^{]*\{[^}]*color: var\(--quest-dim\)/.test(css) && /\.qdonerow[^{]*\{[^}]*color: var\(--quest-dim\)/.test(css)
    && !/\.(qrepeatrow|qdonerow)[^{]*\{[^}]*opacity/.test(css), true);
check('a not-ready Repeat row and a Completed row are quieter by weight (Light mode dim is darker than the name)',
    /\.qrepeatrow:not\(\.ready\) \.qname, \.qdonerow \.qname \{[^}]*font-weight: 400;/.test(css), true);
// The Map may shrink to 150px (master's value) so a short laptop window
// with a few quests does not scroll the whole page (#768 review item 5).
check('the Map can shrink to 150px', /#panel-map\s+\{ flex: 1 1 0; min-height: 150px; \}/.test(css), true);
check('a folded tab drops its own height',
    /min-height: 0 !important;\s*height: auto !important;/.test(css), true);

// Plan 6 R3: an open tab is at most 30% of the visible height where it
// covers game text (phone, slim); column keeps its older cap.
check('phone and slim tabs are 30% of the screen',
    /--drawer-cap: max\(80px, 30vh\);/.test(css) && /--drawer-cap: max\(80px, 30dvh\);/.test(css), true);
check('with the keyboard up, 30% of what is visible',
    /html\.keyboard-open #dash-drawer \{[^}]*--drawer-cap: max\(60px, calc\(0\.3 \* var\(--visible-h, 100dvh\)\)\);/.test(css), true);
check('column keeps its cap, keyboard down',
    /html\.layout-column #dash-drawer \{\s*--drawer-cap: max\(80px, min\(50vh,/.test(css), true);
check('column keeps its cap, keyboard up',
    /html\.layout-column\.keyboard-open #dash-drawer \{\s*--drawer-cap: max\(60px, min\(50vh,/.test(css), true);
check('the page opened on its own sets --visible-h',
    /setProperty\('--visible-h', Math\.floor\(visible\) \+ 'px'\)/.test(page), true);
var outer = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient.html'
), 'utf8');
check('the framed page gets --visible-h from webclient.html, the frame\'s own height',
    /setProperty\('--visible-h', Math\.floor\(frameH\) \+ 'px'\)/.test(outer)
    && /frame\.height = frameH;/.test(outer), true);
check('column keeps its dvh cap, keyboard down',
    /@supports \(height: 1dvh\) \{\s*html\.layout-column #dash-drawer \{\s*--drawer-cap: max\(80px, min\(50dvh,/.test(css), true);
check('column keeps its dvh cap, keyboard up',
    /@supports \(height: 1dvh\) \{\s*html\.layout-column\.keyboard-open #dash-drawer \{\s*--drawer-cap: max\(60px, min\(50dvh,/.test(css), true);

// Plan 6, ruling 14: a Close keyboard X at the end of the command box, shown
// only while the on-screen keyboard is open, at least 44px.
var cmdAt = page.indexOf('<section class="dash-panel" id="panel-cmd"');
var xAt = page.indexOf('id="cmd-close-kbd"');
check('the X is inside the command box, after the input',
    cmdAt >= 0 && xAt > page.indexOf('id="command-input"', cmdAt) && xAt < page.indexOf('</section>', cmdAt), true);
check('the X is a real button named for screen readers',
    /<button type="button" id="cmd-close-kbd" class="brass" aria-label="Close keyboard"/.test(page), true);
check('the X is hidden unless the keyboard is open',
    /#cmd-close-kbd \{[^}]*display: none;/.test(css) && /html\.keyboard-open #cmd-close-kbd \{ display: flex; \}/.test(css), true);
check('the X is 48px', /#cmd-close-kbd \{[^}]*width: 48px;[^}]*height: 48px;/.test(css), true);
check('the X closes the keyboard without taking focus first',
    /closeKbd\.addEventListener\('pointerdown', function \(e\) \{ e\.preventDefault\(\); \}\);/.test(page)
    && /closeKbd\.addEventListener\('mousedown', function \(e\) \{ e\.preventDefault\(\); \}\);/.test(page)
    && /closeKbd\.addEventListener\('click', function \(\) \{ textInput\.blur\(\); \}\);/.test(page), true);

// Plan 7, ruling 16: the panel is Comms; the Chat channel keeps its name.
check('the Comms panel title', /<span class="ph-title">Comms<\/span>/.test(page), true);
check('no panel says Chat any more', /<span class="ph-title">Chat<\/span>|aria-label="Chat channels"|read Chat\./.test(page), false);
check('the Comms channels are named for screen readers', /aria-label="Comms channels"/.test(page), true);
check('the Chat channel keeps its name', /id="comm-tab-chat"[^>]*data-label="Chat"/.test(page), true);
check('the phone tab is Comms', /chat:\s*\{ label: "Comms",/.test(dash), true);
// Plan 7, ruling 20: one menu look for items, Automation and Quests.
['inv-ctx', 'auto-ctx', 'quest-ctx'].forEach(function (cls) {
    check(cls + ' is built as a shared ctx-menu', new RegExp('el\\.className = "' + cls + ' ctx-menu";').test(page), true);
});
check('the shared menu is a fixed box above the page',
    /\.ctx-menu \{[^}]*position: fixed;[^}]*z-index: 9000;/.test(css), true);
check('the menu header is the name at the title size, boxed, regular weight, in gold',
    /\.ctx-menu \.ctx-header \{[^}]*margin: 6px;[^}]*font-size: var\(--title\);[^}]*font-weight: 400;[^}]*color: var\(--title-gold\);[^}]*border: 1px solid var\(--gold-rule\);/.test(css), true);
check('a dimmed menu row does not use --text-secondary (4.40 on the menu)',
    /\.ctx-menu \.ctx-row\.ctx-disabled,\s*\.ctx-menu \.ctx-row\.ctx-disabled:hover \{[^}]*color: var\(--toast-desc\);/.test(css), true);
check('the quest menu skips the header when the row has no name',
    /if \(qName\) \{\s*var hdr = document\.createElement\("div"\);/.test(page), true);
check('menu rows are indented under the name and tall enough to tap',
    /\.ctx-menu \.ctx-row \{[^}]*padding: 8px 10px 8px 24px;/.test(css), true);
check('no per-menu copies of the menu rules are left',
    /\.(inv|auto|quest)-ctx\b/.test(css), false);
check('a dimmed menu row keeps its look under the shared menu',
    /\.ctx-menu \.ctx-row\.ctx-disabled:hover \{[^}]*background: none;/.test(css), true);
check('the unused empty-tile rule is gone', /\.inv-tile\.empty\b/.test(css), false);
// Plan 7, P2: a long-press opens each menu on touch; a tap on an item tile
// opens its menu; other taps keep their jobs.
check('the page loads longpress.js', /static\/js\/longpress\.js\?v=/.test(page), true);
['.inv-tile[data-handle]', '.auto-chip[data-auto-type], .auto-row[data-auto-type]', '.qrow[data-quest-id]'].forEach(function (sel) {
    check('long-press opens the menu on ' + sel,
        page.indexOf('LongPress.attach(document, "' + sel + '"') >= 0, true);
});
check('a tap or click on an item tile opens its menu at the tile',
    /addEventListener\("click", function\(e\) \{\s*var tile = e\.target\.closest\("\.inv-tile\[data-handle\]"\);/.test(page), true);
check('a long touch on these does not select text or raise the iOS callout',
    /@media \(pointer: coarse\) \{\s*\.inv-tile, \.qrow, \.auto-chip, \.auto-row \{[^}]*-webkit-touch-callout: none;[^}]*user-select: none;/.test(css), true);
// --- result ----------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
