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
// The answer to a line the player sends (from lineSent until the server's
// done marker for it, or before login the next prompt) goes into a second,
// assertive log instead, so it cuts
// off whatever the screen reader is still reading: each command drops the
// backlog, as desktop MUD clients do for blind players (owner ruling
// 2026-11-17, after NVDA kept up in order but fell rooms behind). Everything
// else queues politely in the first log.
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

    // MAX_ECHOES bounds the echoes waited for, so a lost one cannot pile up.
    var MAX_ECHOES = 20;
    // ECHO_WAIT_MS: an echo or a done marker still owed after this long is
    // taken as lost, so one the server never sent cannot keep an answer open.
    var ECHO_WAIT_MS = 3000;

    // echoOf is what the server sends back for a line the page sent
    // (internal/inputhandlers/echo.go): the line itself (the websocket
    // cleanser has made each tab a space), one * per character when the box is
    // a password box, and nothing at all for an item panel line that names an
    // item by its @handle (lineTargetsItemHandle: an @ token followed by 35
    // lowercase hex digits and hyphens).
    function echoOf(line, masked) {
        if (typeof line !== 'string') return null;
        if (/(^|\s)@[0-9a-f-]{35}(\s|$)/.test(line)) return null;
        if (masked) return new Array(Array.from(line).length + 1).join('*');
        return plainText(line.replace(/\t/g, ' '));
    }

    // opts: storage (localStorage, any call may throw); input (the command
    // box); hint (the hidden first paragraph); status (a visually hidden
    // role="status" element, for "off"); log (the visually hidden polite
    // role="log" element); answers (the visually hidden assertive role="log"
    // element, for the answer to a line the player sent); createEntry(text)
    // (a new element holding text, for either log); report(on) (sends
    // Client.Display); setCommand(on) (sends "set
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
        // answering: the player sent a line and its answer has not ended. In
        // game the server marks the end of each typed line's answer with
        // Char.Input.Done {seq}, after the line's text (world.go
        // processInput), so the answer ends when every player line's marker
        // has come: a fight's prompt arriving before then does not end it,
        // and a fight's text after it queues. Before login there is no
        // marker, and the answer ends at the first real prompt once no player
        // line's echo is still owed.
        var answering = false;
        // echoes the server still owes, oldest first: {text, player, at}.
        // player: the player asked for that line, so its answer interrupts.
        var echoes = [];
        var lastSent = null;       // the echo owed for the last line sent, if any
        var dones = [];            // done markers owed for player lines: {seq, at}

        // answerOwed: a player line's echo or done marker is still owed (and
        // not lost).
        function answerOwed() {
            var t = now();
            return echoes.some(function (e) { return e.player && t - e.at < ECHO_WAIT_MS; }) ||
                dones.some(function (d) { return t - d.at < ECHO_WAIT_MS; });
        }

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

        function empty(region) {
            if (!region) return;
            while (region.firstChild) region.removeChild(region.firstChild);
        }

        // forgetSends drops any answer in progress and every echo still due.
        function forgetSends() {
            answering = false;
            echoes = [];
            lastSent = null;
            dones = [];
        }

        function clearLog() {
            forgetPrompt();
            forgetSends();
            empty(opts.log);
            empty(opts.answers);
        }

        // append adds an entry to region (the polite log unless given),
        // keeping at most MAX_ENTRIES.
        function append(text, region) {
            region = region || opts.log;
            region.appendChild(opts.createEntry(text));
            while (region.childNodes.length > MAX_ENTRIES) region.removeChild(region.firstChild);
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
            if (!unterminated) {
                // The answer to the player's line interrupts; anything else queues.
                append(text, answering && opts.answers ? opts.answers : opts.log);
                return;
            }
            // The server echoes every line the page sends, as its own piece,
            // possibly after other text already on its way. An echo is not news
            // (the screen reader echoed the typing), not a prompt, and does not
            // end an answer. A masked echo (only *) is never read.
            for (var i = 0; i < echoes.length; i++) {
                if (echoes[i].text === text) { echoes.splice(i, 1); return; }
            }
            if (/^\*+$/.test(text)) return;
            // A real prompt ends the answer, once nothing is owed for a player
            // line (a prompt before its echo or done marker belongs to an
            // earlier answer or to a fight).
            if (!answerOwed()) answering = false;
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

            // The page sent a line to the server (any line: typed, automation,
            // its own commands). The server will echo it; the echo is skipped
            // when it comes. masked: the line was typed into a password box.
            sent: function (line, masked) {
                lastSent = null;
                if (!on) return;
                var echo = echoOf(line, masked);
                if (echo === null || !echo.trim()) return;
                lastSent = { text: echo, player: false, at: now() };
                echoes.push(lastSent);
                if (echoes.length > MAX_ECHOES) echoes.shift();
            },

            // The player asked for the line just sent (typed, the numpad, a
            // macro key, a chip, an item menu, a tick's fire now; never
            // automation): its answer interrupts whatever is still being read.
            // Called after sent(). seq: the line's number, which the server's
            // done marker carries back (only once a character is in game).
            lineSent: function (seq) {
                if (!on) return;
                answering = true;
                if (lastSent) lastSent.player = true;
                if (typeof seq === 'number' && opts.inGame()) {
                    dones.push({ seq: seq, at: now() });
                    if (dones.length > MAX_ECHOES) dones.shift();
                }
            },

            // Char.Input.Done: the server has finished the line numbered seq,
            // after all of its text. Lines finish in order, so earlier markers
            // are no longer owed either. With nothing else owed, the answer
            // ends here.
            done: function (seq) {
                if (typeof seq !== 'number') return;
                dones = dones.filter(function (d) { return d.seq > seq; });
                if (answering && !answerOwed()) answering = false;
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
                forgetSends();
                forgetPrompt();
            }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY, MAX_ENTRIES: MAX_ENTRIES, PROMPT_GAP_MS: PROMPT_GAP_MS,
        MAX_ECHOES: MAX_ECHOES, ECHO_WAIT_MS: ECHO_WAIT_MS,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        plainText: plainText,
        create: create
    };
}));
