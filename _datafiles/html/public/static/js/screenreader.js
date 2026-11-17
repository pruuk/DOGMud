// screenreader.js: the web client's screen reader switch (touch arc Task 0,
// docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md, section 11).
//
// A browser cannot tell that a screen reader is running. So the command box's
// spoken label, and a hidden line at the top of the page, tell screen reader
// users to type "screen reader on". The page passes that phrase here instead
// of sending it. The choice is remembered on this device, reported to the
// server (Client.Display), and follows the account's setting
// (Char.Info.screenreader) once a character is logged in.
//
// While the mode is on, each piece of game text the page writes is also added
// to a visually hidden role="log" region as its own new entry. A screen
// reader announces each new entry once. Nothing is cleared while it might
// still be read (old entries are trimmed past MAX_ENTRIES, and a removal is
// not announced), so there is no rereading from the top, no line limit and no
// race with the screen reader (owner ruling 2026-11-17, after Narrator went
// silent with xterm's own screen reader mode). xterm's screenReaderMode is
// not used. Sighted players never see any of it: with the mode off nothing is
// added.
//
// tools/jstest/screenreader.test.js is the regression guard.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;          // node, for the tests
    } else {
        root.ScreenReader = api;       // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var STORAGE_KEY = 'screenReader';
    var MAX_ENTRIES = 100;      // entries kept for reading back; older ones go
    // PROMPT_GAP_MS is the least time between two status prompts read in
    // game: one combat round (Timing.RoundSeconds, 4 in config.yaml), so the
    // status is heard at most once a round (owner, 2026-11-17).
    var PROMPT_GAP_MS = 4000;
    var LABEL_OFF = 'Game commands. Screen reader users: type screen reader on, then press Enter.';
    var LABEL_ON = 'Game commands.';
    var HINT_OFF = 'Screen reader users: type screen reader on, then press Enter.';
    var HINT_ON = 'Screen reader mode is on. Type screen reader off to turn it off.';
    var SAID_ON = 'Screen reader mode is on.';
    var SAID_OFF = 'Screen reader mode is off.';

    // matchPhrase returns 'on', 'off' or null for a typed line. Case,
    // spacing and a trailing full stop (phone dictation adds one) do not
    // matter, and "screenreader" works as one word. A password box never
    // matches, so a password is never taken for the phrase.
    function matchPhrase(line, isPassword) {
        if (isPassword || typeof line !== 'string') return null;
        var s = line.toLowerCase().replace(/[.!?,\s]+$/, '').trim().replace(/\s+/g, ' ');
        if (s === 'screen reader on' || s === 'screenreader on') return 'on';
        if (s === 'screen reader off' || s === 'screenreader off') return 'off';
        return null;
    }

    // ESCAPES matches a terminal escape sequence: CSI (colour, cursor moves,
    // the line erase before each message), OSC ended by BEL or ESC \, and
    // the two-character forms (charset selection and the like).
    var ESCAPES = /\x1b(\[[0-?]*[ -\/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[()%][0-9A-Za-z@]|.)/g;
    // SYMBOLS are characters a screen reader reads out by name: box drawing
    // and block elements (U+2500 to U+259F: the MOTD box, the health bars in
    // the prompt) and the prompt's sun, moon and arrow marks.
    var SYMBOLS = /[─-▟☀☾»]/g;

    // plainText is what a screen reader should hear of a piece of terminal
    // text: no escape sequences, no control characters other than line
    // breaks, no drawing symbols, single spaces, no spaces at either end of
    // a line (indentation means nothing aloud), and no blank lines at either
    // end.
    function plainText(s) {
        return String(s)
            .replace(ESCAPES, '')
            .replace(/\r/g, '')
            .replace(/[\x00-\x09\x0B-\x1F\x7F]/g, '')
            .replace(SYMBOLS, ' ')
            .split('\n').map(function (l) { return l.replace(/ {2,}/g, ' ').trim(); }).join('\n')
            .replace(/^\n+|\n+$/g, '');
    }

    // opts: storage (localStorage, any call may throw); input (the command
    // box); hint (the hidden first paragraph); status (a visually hidden
    // role="status" element, for "off"); log (the visually hidden role="log"
    // element); createEntry(text) (a new element holding text, for the log);
    // report(on) (sends Client.Display); setCommand(on) (sends "set
    // screenreader on" or "off", a state and never a toggle, false when it
    // could not); writeLine(text) (writes a local line into the game window);
    // currentLines() (the waiting prompt, as lines); inGame() (true while a
    // character is logged in); now(), setTimeout and clearTimeout (the
    // window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var now = opts.now || Date.now;
        var on = false;
        var accountSetting = null; // the account's setting as this page believes it
        var lastPushed = null;     // the last Char.Info.screenreader value seen
        var lastPrompt = null;     // the last prompt announced
        var lastPromptAt = null;   // when it was announced
        var pendingPrompt = null;  // a changed prompt held until a round has passed
        var promptTimer = null;

        function remember(value) {
            try { opts.storage.setItem(STORAGE_KEY, JSON.stringify(value)); } catch (e) { /* storage blocked: this visit only */ }
        }

        // forgetPrompt makes the next prompt news, with no gap to wait out.
        function forgetPrompt() {
            lastPrompt = null;
            lastPromptAt = null;
            pendingPrompt = null;
            if (promptTimer) { clearT(promptTimer); promptTimer = null; }
        }

        function clearLog() {
            forgetPrompt();
            if (!opts.log) return;
            while (opts.log.firstChild) opts.log.removeChild(opts.log.firstChild);
        }

        function append(text) {
            opts.log.appendChild(opts.createEntry(text));
            while (opts.log.childNodes.length > MAX_ENTRIES) opts.log.removeChild(opts.log.firstChild);
        }

        function announcePrompt(text) {
            lastPrompt = text;
            lastPromptAt = now();
            append(text);
        }

        // flushPrompt reads the held prompt once the gap is over, if it still
        // differs from the last one read.
        function flushPrompt() {
            promptTimer = null;
            var text = pendingPrompt;
            pendingPrompt = null;
            if (on && text !== null && text !== lastPrompt) announcePrompt(text);
        }

        function show(value) {
            on = value;
            if (!value) clearLog();
            opts.input.setAttribute('aria-label', value ? LABEL_ON : LABEL_OFF);
            opts.hint.textContent = value ? HINT_ON : HINT_OFF;
        }

        // say writes the line into the status region, for "off": the log is
        // emptied when the mode goes off.
        function say(text) {
            opts.status.textContent = '';
            opts.status.textContent = text;
        }

        // speak adds a piece of text to the log as one new entry. A piece
        // with no line break at its end is a prompt. A prompt identical to the
        // last one read is skipped (the server redraws it after every
        // message). In game, a changed prompt is read at most once a round:
        // one arriving within PROMPT_GAP_MS of the last is held, and read when
        // the gap is over only if it still differs (owner, 2026-11-17).
        // Login prompts are read at once.
        function speak(raw) {
            if (!opts.log) return;
            var text = plainText(raw);
            if (!text.trim()) return;
            var unterminated = !/\n\s*$/.test(String(raw).replace(ESCAPES, ''));
            if (!unterminated) { append(text); return; }
            if (text === lastPrompt) { pendingPrompt = null; return; }
            var since = lastPromptAt === null ? Infinity : now() - lastPromptAt;
            if (opts.inGame() && since < PROMPT_GAP_MS) {
                pendingPrompt = text;
                if (!promptTimer) promptTimer = setT(flushPrompt, PROMPT_GAP_MS - since);
                return;
            }
            announcePrompt(text);
        }

        // local writes a line of the page's own into the game window and,
        // while the mode is on, into the log.
        function local(text) {
            opts.writeLine(text);
            if (on) speak(text + '\n');
        }

        return {
            isOn: function () { return on; },

            // Page load: put the labels in place, and turn the mode on when
            // this device remembers it, before the connection opens.
            load: function () {
                var saved = false;
                try { saved = JSON.parse(opts.storage.getItem(STORAGE_KEY) || 'false') === true; } catch (e) { saved = false; }
                show(saved);
            },

            // write is called with each piece of server text the page has
            // written into the game window; while the mode is on it is added
            // to the log.
            write: function (text) {
                if (on) speak(text);
            },

            // The player typed the phrase: which is 'on' or 'off'.
            phrase: function (which) {
                var value = which === 'on';
                // Read the waiting prompt before anything is written over it.
                var waiting = value && !opts.inGame() ? opts.currentLines() : [];
                show(value);
                remember(value);
                opts.report(value);
                if (opts.inGame() && accountSetting !== null && accountSetting !== value && opts.setCommand(value) !== false) {
                    // The server's "ScreenReader mode toggled" line confirms it.
                    // Count it as done now, so a second phrase before Char.Info
                    // returns does not toggle it back.
                    accountSetting = value;
                    if (!value) say(SAID_OFF);
                    return;
                }
                local(value ? SAID_ON : SAID_OFF);
                if (value) {
                    waiting.forEach(function (line) { local(line); });
                } else {
                    say(SAID_OFF);
                }
            },

            // Char.Info named the account's setting. The page's generic Char
            // handler re-reads the last Info for other Char packages, so a
            // value equal to the last one seen is not news and is ignored: it
            // must not undo a phrase the server has not answered yet.
            account: function (value) {
                if (value === lastPushed) return;
                lastPushed = value;
                accountSetting = value;
                if (value !== on) {
                    show(value);
                    remember(value);
                    // Turned off from elsewhere (set screenreader off, or another
                    // device): the log was just emptied, so say it here.
                    if (!value) say(SAID_OFF);
                }
            },

            // The connection closed: the next login names the account again,
            // and the next prompt is news even if it reads the same.
            loggedOut: function () {
                accountSetting = null;
                lastPushed = null;
                forgetPrompt();
            }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY, MAX_ENTRIES: MAX_ENTRIES, PROMPT_GAP_MS: PROMPT_GAP_MS,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        plainText: plainText,
        create: create
    };
}));
