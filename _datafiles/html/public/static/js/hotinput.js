// hotinput.js: decides whether a keystroke should be pulled into the command
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
        if (root.document) api.watch(root.document);
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

    // Where the current focus came from. A mouse click or tap also focuses a
    // button (the Automation and Inventory tabs, the Chat channels, the map
    // Legend), and a player who clicks one and then types expects the letters
    // in the command box, not lost on the button (#768 review item 6).
    //
    // :focus-visible cannot tell us: Chrome and Edge turn it on for a clicked
    // button as soon as any key is pressed, before the page's keydown listener
    // runs, so the first typed letter always looked like keyboard focus. So
    // watch() records the origin itself. The last input before a focus change
    // decides it: a pointer press (mouse, pen, finger) or a key (Tab, arrows,
    // a script answering a key). Typing onto a clicked button does not move
    // focus, so it does not change the origin.
    var lastInput = null;       // 'pointer' | 'keyboard' | null
    var focused = null;         // { el: Element, pointer: bool }

    function onPointer() { lastInput = 'pointer'; }
    function onKey() { lastInput = 'keyboard'; }
    function onFocusIn(e) {
        focused = { el: e && e.target, pointer: lastInput === 'pointer' };
    }

    // watch installs the focus-origin listeners on a document, in the capture
    // phase so they run before any page handler. The browser build calls it
    // on load; the tests call it on a fake document.
    function watch(doc) {
        lastInput = null;
        focused = null;
        if (!doc || typeof doc.addEventListener !== 'function') return;
        doc.addEventListener('pointerdown', onPointer, true);
        doc.addEventListener('mousedown', onPointer, true);   // no pointer events
        doc.addEventListener('keydown', onKey, true);
        doc.addEventListener('focusin', onFocusIn, true);
    }

    // isPointerFocused reports whether the element got its focus from a click
    // or tap. Focus with no recorded origin (watch() not running, a script
    // focusing on load) counts as keyboard focus: the older, safer rule that a
    // focused control owns its keys.
    function isPointerFocused(el) {
        return !!(el && focused && focused.el === el && focused.pointer);
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
        if (isKeyboardControl(el, opts.document) && !isPointerFocused(el)) return false;

        return true;
    }

    // --- The phone keyboard after a send (plan 6) --------------------------
    // docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md.
    //
    // R1: the keyboard stays up when the player's next act is to type again,
    // that is chat to other players. These words are today's data under that
    // principle, not a closed list. Only what was literally typed counts:
    // aliases and macros are out (layout spec ruling 13). party and guild are
    // mostly management, so they count only when talking.
    var CHAT_WORDS = ['say', 'whisper', 'reply', 'chat', 'newbie', 'trade', 'shout',
                      'emote', 'yell', 'scream', 'holler', '/w', 'gc', 'pchat', 'psay'];
    var PARTY_TALK = ['chat', 'say'];
    var GUILD_TALK = ['chat', 'say', 'gc'];
    // R2, R5, R6: anything else waits for the server's Char.Input.Done for
    // that line; this long at most if the marker never comes.
    var MENU_WAIT_CEILING_MS = 2000;

    function isChatLine(line) {
        var t = String(line == null ? '' : line).trim();
        if (t === '') return false;
        // The server reads a leading . or backtick as say or chat with or
        // without a space (usercommands.SplitInput).
        var c = t.charAt(0);
        if (c === '.' || c === '`') return true;
        var words = t.toLowerCase().split(/\s+/);
        if (CHAT_WORDS.indexOf(words[0]) >= 0) return true;
        if (words[0] === 'party') return PARTY_TALK.indexOf(words[1]) >= 0;
        if (words[0] === 'guild') return GUILD_TALK.indexOf(words[1]) >= 0;
        return false;
    }

    // What the phone keyboard does after a send: 'keep' it, or 'wait' for the
    // game's answer. Only a touch-only phone-width screen, in game, with no
    // menu open and not at a password prompt ever waits (owner ruling on #768,
    // 2026-11-07, which these checks keep). Tablets keep the keyboard, since a
    // blur there would also stop arrow history and macros working.
    //
    // s: { touchOnly, phoneWidth, sent, inGame, menuOpen, password, line }
    function keyboardAfterSend(s) {
        s = s || {};
        if (!(s.touchOnly && s.phoneWidth && s.sent)) return 'keep';
        if (!s.inGame || s.menuOpen || s.password) return 'keep';
        if (isChatLine(s.line)) return 'keep';
        return 'wait';
    }

    // Once the wait is over: close the keyboard only if no menu or prompt is
    // open, the command box still has focus, and the player has not started
    // typing the next command.
    function blurAfterWait(s) {
        s = s || {};
        return !s.menu && !!s.focused && (s.value || '') === '';
    }

    // One wait at a time for the marker of line `seq` (the client's count of
    // text frames; the server counts the same, R7). opts: { setTimeout,
    // clearTimeout, ceilingMs, currentMenu() -> bool, settle(menu) }.
    function createSendWait(opts) {
        var pending = null;   // { seq, timer }
        function finish(menu) {
            if (!pending) return;
            opts.clearTimeout(pending.timer);
            pending = null;
            opts.settle(menu);
        }
        return {
            start: function (seq) {
                if (pending) opts.clearTimeout(pending.timer);
                var p = { seq: seq };
                p.timer = opts.setTimeout(function () {
                    if (pending === p) finish(opts.currentMenu());
                }, opts.ceilingMs);
                pending = p;
            },
            // The server's marker for line `seq`. A higher number means the
            // waiting line never reached the world (an admin system command, a
            // dropped line): settle now with the current menu state.
            done: function (seq, menu) {
                if (!pending || typeof seq !== 'number') return;
                if (seq === pending.seq) finish(menu === true);
                else if (seq > pending.seq) finish(opts.currentMenu());
            },
            // A new socket: its count starts again, so an old wait is void.
            reset: function () {
                if (pending) opts.clearTimeout(pending.timer);
                pending = null;
            }
        };
    }

    return {
        FOCUSABLE_CONTROL_SELECTOR: FOCUSABLE_CONTROL_SELECTOR,
        MENU_WAIT_CEILING_MS: MENU_WAIT_CEILING_MS,
        isTypingKey: isTypingKey,
        isEditableField: isEditableField,
        isKeyboardControl: isKeyboardControl,
        isPointerFocused: isPointerFocused,
        watch: watch,
        shouldFocusCommandBar: shouldFocusCommandBar,
        isChatLine: isChatLine,
        keyboardAfterSend: keyboardAfterSend,
        blurAfterWait: blurAfterWait,
        createSendWait: createSendWait
    };
}));
