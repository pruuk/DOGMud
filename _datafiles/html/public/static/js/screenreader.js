// screenreader.js: the web client's screen reader switch (touch arc Task 0,
// docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md).
//
// A blind player hears nothing of the game until xterm's screenReaderMode
// is on, and a browser cannot tell that a screen reader is running. So the
// command box's spoken label, and a hidden line at the top of the page, tell
// screen reader users to type "screen reader on". The page passes that
// phrase here instead of sending it. The choice is remembered on this
// device, reported to the server (Client.Display), and follows the account's
// setting (Char.Info.screenreader) once a character is logged in. Sighted
// players never see any of it: with the mode off, write() hands text
// straight to xterm.
//
// xterm 4.19 speaks up to 20 lines of a burst; on the 21st it appends "Too
// much output to announce" and goes silent until its spoken region is reset.
// It resets it only on a key in its own hidden input, which players never
// use. So this module resets it (owner ruling 2026-11-17):
//   - an answer to a line the page sent (from inputSent(seq) until the
//     server's Char.Input.Done for that seq or a later one), and everything
//     before login (the connect splash and the prompts), is written in
//     pieces of CHUNK_LINES, with a reset after each piece is parsed, so it
//     is read in full;
//   - other output is written whole, with a reset after QUIET_MS of quiet,
//     so each unprompted burst gets its own 20 spoken lines.
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
    var QUIET_MS = 2000;        // under a 4 second combat round
    var CHUNK_LINES = 18;       // under xterm's 20 spoken lines per burst
    var ANSWER_MAX_MS = 15000;  // a lost Char.Input.Done ends the answer
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

    // splitLines cuts text into pieces of at most max newline-ended lines;
    // the last piece may end without a newline (a prompt).
    function splitLines(text, max) {
        var pieces = [], start = 0, count = 0;
        for (var i = 0; i < text.length; i++) {
            if (text.charAt(i) !== '\n') continue;
            count++;
            if (count === max) {
                pieces.push(text.slice(start, i + 1));
                start = i + 1;
                count = 0;
            }
        }
        if (start < text.length) pieces.push(text.slice(start));
        return pieces;
    }

    // opts: term (xterm, for setOption); termWrite(data, cb) (xterm's write,
    // cb once data is parsed); storage (localStorage, any call may throw);
    // input (the command box); hint (the hidden first paragraph); status (a
    // visually hidden role="status" element); report(on) (sends
    // Client.Display); setCommand(on) (sends "set screenreader on" or "off",
    // a state and never a toggle, false when it could not); writeLine(text)
    // (writes a local line into the game window); currentLines() (the
    // waiting prompt, as lines); clearSpoken()
    // (resets xterm's spoken region); inGame() (true while a character is
    // logged in); setTimeout/clearTimeout (the window's, or a test clock).
    function create(opts) {
        var setT = opts.setTimeout || setTimeout, clearT = opts.clearTimeout || clearTimeout;
        var on = false;
        var accountSetting = null; // the account's setting as this page believes it
        var lastPushed = null; // the last Char.Info.screenreader value seen
        var quietTimer = null;
        var answering = false; // a sent line's answer is still arriving
        var lastSeq = 0;       // the number of the last line sent
        var answerTimer = null;
        var settling = false;  // its Done arrived; waiting for output to go quiet
        var settleTimer = null;
        var queue = [];        // pieces waiting for the one being parsed
        var parsing = false;

        function remember(value) {
            try { opts.storage.setItem(STORAGE_KEY, JSON.stringify(value)); } catch (e) { /* storage blocked: this visit only */ }
        }

        function show(value) {
            on = value;
            if (!value && quietTimer) { clearT(quietTimer); quietTimer = null; }
            opts.term.setOption('screenReaderMode', value);
            opts.input.setAttribute('aria-label', value ? LABEL_ON : LABEL_OFF);
            opts.hint.textContent = value ? HINT_ON : HINT_OFF;
        }

        // say writes the line into our own status region: xterm's region is
        // gone once the mode is off.
        function say(text) {
            opts.status.textContent = '';
            opts.status.textContent = text;
        }

        function endAnswer() {
            answering = false;
            settling = false;
            if (answerTimer) { clearT(answerTimer); answerTimer = null; }
            if (settleTimer) { clearT(settleTimer); settleTimer = null; }
        }

        // armSettle (re)starts the quiet wait that ends an answer whose
        // Char.Input.Done has arrived.
        function armSettle() {
            if (settleTimer) clearT(settleTimer);
            settleTimer = setT(endAnswer, QUIET_MS);
        }

        function armQuiet() {
            if (quietTimer) clearT(quietTimer);
            quietTimer = setT(function () { quietTimer = null; if (on) opts.clearSpoken(); }, QUIET_MS);
        }

        // pump writes the next queued piece; a piece that needs a reset after
        // it holds the queue until xterm has parsed it.
        function pump() {
            while (!parsing && queue.length) {
                var item = queue.shift();
                if (item.resetAfter) {
                    parsing = true;
                    // termWrite must always call its callback (xterm's write
                    // does, once the data is parsed). If it never did, the
                    // queue would stall and later text would not reach the
                    // terminal.
                    opts.termWrite(item.text, function () {
                        parsing = false;
                        if (on) opts.clearSpoken();
                        pump();
                    });
                } else {
                    opts.termWrite(item.text);
                    if (on) armQuiet();
                }
            }
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

            // write is the page's only way to put server text in the game
            // window.
            write: function (text) {
                if (settling) armSettle();
                if (!on && !parsing && !queue.length) {
                    opts.termWrite(text);
                    return;
                }
                if (on && (answering || !opts.inGame())) {
                    splitLines(text, CHUNK_LINES).forEach(function (piece) {
                        queue.push({ text: piece, resetAfter: true });
                    });
                } else {
                    queue.push({ text: text, resetAfter: false });
                }
                pump();
            },

            // The page sent line number seq to the server (any line: typed,
            // a macro, a numpad move, automation). Reset now, and read its
            // answer in full. Char.Input.Done comes only after login. A line
            // sent before login is answered by the login burst, read in full
            // once it ends on a quiet gap; before login everything is read
            // in full anyway.
            inputSent: function (seq) {
                if (on) opts.clearSpoken();
                lastSeq = seq;
                answering = true;
                if (answerTimer) clearT(answerTimer);
                answerTimer = setT(endAnswer, ANSWER_MAX_MS);
                if (!opts.inGame()) {
                    // A login line (such as the creation confirm) gets no
                    // Char.Input.Done, and the welcome burst arrives after
                    // Char, so a quiet gap ends this answer instead.
                    settling = true;
                    armSettle();
                } else {
                    settling = false;
                    if (settleTimer) { clearT(settleTimer); settleTimer = null; }
                }
            },

            // The player is typing, as xterm clears for a key in its own
            // hidden input, so the screen reader has no stale text to return
            // to.
            keyTyped: function () { if (on) opts.clearSpoken(); },

            // Char.Input.Done for line seq: the answer is over once the server
            // has finished the last line sent. A Done for an earlier line
            // changes nothing.
            inputDone: function (seq) {
                if (!answering || seq < lastSeq) return;
                // Not over yet: a macro's commands and anything run through
                // user.Command() print on later turns, after this Done. The
                // answer ends once output has been quiet for QUIET_MS.
                settling = true;
                armSettle();
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
                opts.writeLine(value ? SAID_ON : SAID_OFF);
                if (value) {
                    waiting.forEach(function (line) { opts.writeLine(line); });
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
                }
            },

            // The connection closed: the next login names the account again,
            // and no answer is still running.
            loggedOut: function () {
                accountSetting = null;
                lastPushed = null;
                endAnswer();
            }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY, QUIET_MS: QUIET_MS,
        CHUNK_LINES: CHUNK_LINES, ANSWER_MAX_MS: ANSWER_MAX_MS,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        create: create
    };
}));
