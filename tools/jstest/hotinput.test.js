// hotinput.test.js — regression tests for the web client's "keep the command
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
    return {
        tagName: tagName,
        isContentEditable: !!opts.isContentEditable,
        classList: {
            contains: function (c) {
                return (opts.classes || []).indexOf(c) !== -1;
            }
        },
        matches: function (selector) {
            for (var i = 0; i < descriptors.length; i++) {
                if (selector.indexOf(descriptors[i]) !== -1) return true;
            }
            return false;
        }
    };
}

var BUTTON = el('BUTTON', { descriptors: ['button'] });
var LINK = el('A', { descriptors: ['a[href]'] });
var TABBABLE = el('DIV', { descriptors: ['[tabindex]:not([tabindex="-1"])'] });
var ROLE_BUTTON = el('DIV', { descriptors: ['[role="button"]'] });
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

// Real fields keep their input.
check('typing in a text input is left alone', focuses(key('n'), TEXT_INPUT), false);
check('typing in the chat textarea is left alone', focuses(key('n'), CHAT_TEXTAREA), false);

// Modals own focus while open.
check('typing is left alone while a modal is open', focuses(key('n'), PLAIN_DIV, true), false);

// --- result ----------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
