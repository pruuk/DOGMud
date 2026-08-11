// hotinput.js — decides whether a keystroke should be pulled into the command
// bar, for the web client's "keep the command bar hot" behaviour.
//
// Review finding 18. The client used to redirect focus to the command input on
// EVERY keydown that had no Ctrl/Meta/Alt modifier. The intent was good: if you
// click the map and then start typing a movement command, the keystrokes should
// not be swallowed. The effect was that the interface could not be operated
// without a mouse at all.
//
//   Tab    moved focus to the command bar instead of to the next control, so no
//          other control could ever be reached.
//   Space  and Enter were stolen before the focused button could act on them,
//          so no other control could ever be operated.
//   Arrows never reached whatever the user had focused.
//
// The logic lives here rather than inline in the page so it can be tested.
// tools/jstest/hotinput.test.js is the regression guard; the page requires this
// file and calls shouldFocusCommandBar.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.HotInput = api;       // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {

    // Controls a user can deliberately move focus to. Typing while one of these
    // is focused is far more likely to be a mistake than an attempt to send a
    // command, and stealing the keystroke is what makes focus feel like it is
    // fighting back.
    var FOCUSABLE_CONTROL_SELECTOR =
        'button, a[href], [tabindex]:not([tabindex="-1"]), summary, details, ' +
        '[role="button"], [role="link"], [role="menuitem"], [role="tab"]';

    // isTypingKey reports whether an event represents a printable character.
    //
    // e.key.length === 1 is the standard test: "a" and "5" are a single code
    // unit, while "Tab", "Enter", "ArrowUp", "F1", "Escape" and the bare
    // modifier names are all longer. That one check replaces the old hand-kept
    // list of modifier key names, which is exactly the kind of list that goes
    // stale.
    function isTypingKey(e) {
        if (!e) return false;
        if (e.ctrlKey || e.metaKey || e.altKey) return false;
        return typeof e.key === 'string' && e.key.length === 1;
    }

    // isEditableField reports whether the element is somewhere the user is
    // genuinely typing already.
    //
    // The xterm helper textarea is excluded on purpose: it is an invisible
    // element the terminal keeps focused to receive input, not a field the user
    // chose, so keystrokes landing there SHOULD be pulled into the command bar.
    function isEditableField(el) {
        if (!el) return false;
        if (el.tagName === 'INPUT' || el.tagName === 'SELECT') return true;
        if (el.isContentEditable) return true;
        if (el.tagName === 'TEXTAREA') {
            var cl = el.classList;
            return !(cl && typeof cl.contains === 'function' &&
                     cl.contains('xterm-helper-textarea'));
        }
        return false;
    }

    // isKeyboardControl reports whether the element is a control the user
    // tabbed to and expects to keep operating.
    function isKeyboardControl(el, doc) {
        if (!el) return false;
        if (doc && el === doc.body) return false;
        if (typeof el.matches !== 'function') return false;
        return el.matches(FOCUSABLE_CONTROL_SELECTOR);
    }

    // shouldFocusCommandBar is the whole decision.
    //
    // opts: { modalOpen: bool, activeElement: Element|null, document: Document }
    function shouldFocusCommandBar(e, opts) {
        opts = opts || {};

        if (!isTypingKey(e)) return false;

        // Never fight an open modal for focus.
        if (opts.modalOpen) return false;

        var el = opts.activeElement;
        if (isEditableField(el)) return false;
        if (isKeyboardControl(el, opts.document)) return false;

        return true;
    }

    return {
        FOCUSABLE_CONTROL_SELECTOR: FOCUSABLE_CONTROL_SELECTOR,
        isTypingKey: isTypingKey,
        isEditableField: isEditableField,
        isKeyboardControl: isKeyboardControl,
        shouldFocusCommandBar: shouldFocusCommandBar
    };
}));
