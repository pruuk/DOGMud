# Screen reader opt-in, addendum: our own reading area (touch arc Task 0)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace xterm's screen reader mode with a visually hidden `role="log"` region the page fills with each piece of game text, so Narrator (and other screen readers) read each new piece once, never reread from the top, and never go silent.

**Architecture:** `static/js/screenreader.js` keeps its switch (phrase, device memory, account follow) and loses its timing machinery (pieces, quiet gaps, answer tracking, xterm private calls). Its `write(text)` turns server text into plain text and appends it to `#sr-log` as a new entry while the mode is on. The page writes server text to xterm as before and passes the same text to `screenReader.write`. The server does not change.

**Spec:** `docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md`, section 11 (owner ruling 2026-11-17, option 1 after the Narrator test). Branch `feature/screen-reader-task0`, worktree `C:/tmp/dogmud-sr0`. Follows plan `2026-11-17-screen-reader-task0.md` (Tasks 1 to 9).

## Facts verified against source (worktree at `03e3f7a99`)

| Fact | Where |
|---|---|
| The page writes each piece of server text at one place, through `screenReader.write(...)` today | `webclient-pure.html:3284` |
| `screenReader` is created with `term`, `termWrite`, `clearSpoken` (private `_accessibilityManager`) among its opts | `webclient-pure.html:553-597` |
| `screenReader.inputDone(d.seq)` in the `Char.Input.Done` handler; `inputSent(linesSent)` after the copyover token and in `SendData`; `keyTyped()` first in the command box keydown | `webclient-pure.html:1126`, `:3143`, `:3483`, `:3395` |
| The status region exists | `webclient-pure.html:251` |
| `.xterm .xterm-accessibility { pointer-events: none; }` exists only for xterm's overlay | `dashboard.css:1971` |
| `roomtitlebold.test.js` pins the server-text write line; `webclient-connect.test.js` stands in for `screenReader` with `isOn`, `loggedOut`, `inputSent`, `write` | `tools/jstest/roomtitlebold.test.js:34`; `tools/jstest/webclient-connect.test.js:88-90` |
| Each message is sent as column 0, erase line, text; the prompt is redrawn as its own frame with no line break | `internal/hooks/Message_SendMessages.go:40`; `internal/hooks/RedrawPrompt_SendRedraw.go` |
| xterm adds to its live region with `textContent +=` and empties it only on its own input's keys | `xterm.4.19.0.js` `_announceCharacters`, `_onKey` |

## Rules for every task

- Edit files with the Edit and Write tools only; never write a repo file from the shell.
- `git add` named paths only; never `--no-verify`; never stash. Commit messages through a quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes in anything written.

---

### Task 10: The module writes to a log

**Files:** `_datafiles/html/public/static/js/screenreader.js` (replace), `tools/jstest/screenreader.test.js` (replace).

- [ ] **Step 1: Replace the test** with exactly this content, and run it: it fails (the old module has no `plainText` or log).

```js
// screenreader.test.js: the web client's screen reader switch (touch arc
// Task 0, docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md,
// section 11).
//
// Run:  node tools/jstest/screenreader.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var pub = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var SR = require(path.join(pub, 'static', 'js', 'screenreader.js'));
var page = fs.readFileSync(path.join(pub, 'webclient-pure.html'), 'utf8');

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// ---- the page --------------------------------------------------------------
// xterm's own screen reader mode is never used: its live region reread from
// the top under Narrator and fell silent when cleared (section 11).
check('the page never turns on xterm screenReaderMode', page.indexOf('screenReaderMode') < 0, '');
check('the page has the log region', /id="sr-log"[^>]*role="log"/.test(page), '');

// ---- matchPhrase -----------------------------------------------------------

check('on', SR.matchPhrase('screen reader on', false) === 'on', '');
check('off', SR.matchPhrase('screen reader off', false) === 'off', '');
check('case and spaces', SR.matchPhrase('  Screen   READER  On ', false) === 'on', '');
check('dictation full stop', SR.matchPhrase('Screen reader on.', false) === 'on', '');
check('one word', SR.matchPhrase('screenreader off', false) === 'off', '');
check('password box never matches', SR.matchPhrase('screen reader on', true) === null, '');
check('other text', SR.matchPhrase('screen reader', false) === null, '');
check('a command', SR.matchPhrase('look', false) === null, '');
check('not a string', SR.matchPhrase(undefined, false) === null, '');

// ---- plainText -------------------------------------------------------------

check('colour codes go', SR.plainText('\x1b[1;32mA rat\x1b[0m arrives.\r\n') === 'A rat arrives.', JSON.stringify(SR.plainText('\x1b[1;32mA rat\x1b[0m arrives.\r\n')));
check('the line erase before a message goes', SR.plainText('\x1b[1G\x1b[2KYou say hi.\r\n') === 'You say hi.', '');
check('blank lines at the ends go, inner ones stay', SR.plainText('\r\n\r\nOne\r\n\r\nTwo  \r\n\r\n') === 'One\n\nTwo', JSON.stringify(SR.plainText('\r\n\r\nOne\r\n\r\nTwo  \r\n\r\n')));

// ---- fakes -----------------------------------------------------------------

function fakeLog() {
    var log = { childNodes: [] };
    Object.defineProperty(log, 'firstChild', { get: function () { return log.childNodes[0] || null; } });
    log.appendChild = function (n) { log.childNodes.push(n); return n; };
    log.removeChild = function (n) { var i = log.childNodes.indexOf(n); if (i >= 0) log.childNodes.splice(i, 1); return n; };
    log.texts = function () { return log.childNodes.map(function (n) { return n.text; }); };
    return log;
}

function fakes(saved) {
    var f = {
        label: null, hint: null, status: '', lines: [], reports: [], setCommands: 0, setValues: [],
        waiting: ['Username (or "new"): '], setOk: true,
        store: saved === undefined ? {} : { screenReader: saved }, inGame: false, log: fakeLog()
    };
    f.opts = {
        storage: {
            getItem: function (k) { return Object.prototype.hasOwnProperty.call(f.store, k) ? f.store[k] : null; },
            setItem: function (k, v) { f.store[k] = v; }
        },
        input: { setAttribute: function (k, v) { if (k === 'aria-label') f.label = v; } },
        hint: {},
        status: {},
        log: f.log,
        createEntry: function (text) { return { text: text }; },
        report: function (on) { f.reports.push(on); return true; },
        setCommand: function (value) { f.setCommands++; f.setValues.push(value); return f.setOk; },
        writeLine: function (t) { f.lines.push(t); },
        currentLines: function () { return f.waiting.slice(); },
        inGame: function () { return f.inGame; }
    };
    Object.defineProperty(f.opts.hint, 'textContent', { set: function (v) { f.hint = v; } });
    Object.defineProperty(f.opts.status, 'textContent', { set: function (v) { f.status = v; }, get: function () { return f.status; } });
    return f;
}

function nLines(n, tag) {
    var s = '';
    for (var i = 1; i <= n; i++) s += tag + i + '\r\n';
    return s;
}

// ---- load and storage ------------------------------------------------------

var f = fakes();
var sr = SR.create(f.opts);
sr.load();
check('fresh device: off', !sr.isOn(), '');
check('fresh device: label names the phrase', f.label === SR.LABEL_OFF, f.label);
check('fresh device: hint names the phrase', f.hint === SR.HINT_OFF, f.hint);
check('fresh device: nothing reported or stored', f.reports.length === 0 && !('screenReader' in f.store), JSON.stringify(f.store));
sr.write('A rat arrives.\r\n');
check('off: nothing is added to the log', f.log.childNodes.length === 0, f.log.texts().join('|'));

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
check('remembered: on at load', sr.isOn(), '');
check('remembered: label shortened', f.label === SR.LABEL_ON, f.label);

f = fakes();
f.opts.storage = { getItem: function () { throw new Error('blocked'); }, setItem: function () { throw new Error('blocked'); } };
sr = SR.create(f.opts);
sr.load();
check('blocked storage: off, no throw', !sr.isOn(), '');
sr.phrase('on');
check('blocked storage: still turns on', sr.isOn(), '');

// ---- the log ---------------------------------------------------------------

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('\x1b[1G\x1b[2KA rat arrives.\r\n');
sr.write('You see a rat.\r\nIt looks hungry.\r\n');
check('each piece is one new entry, plain text', f.log.texts().join('|') === 'A rat arrives.|You see a rat.\nIt looks hungry.', f.log.texts().join('|'));
sr.write('\r\n');
sr.write('\x1b[1G\x1b[2K');
check('an empty piece adds nothing', f.log.childNodes.length === 2, f.log.childNodes.length);

// A long answer is one entry, never cut off.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
sr.write(nLines(60, 'L'));
check('a 60 line answer is one whole entry', f.log.childNodes.length === 1 && f.log.texts()[0].split('\n').length === 60, f.log.texts()[0] && f.log.texts()[0].split('\n').length);

// Entries are never cleared while the mode is on; only the oldest are trimmed.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
for (var i = 1; i <= SR.MAX_ENTRIES + 5; i++) sr.write('line ' + i + '\r\n');
check('the log keeps MAX_ENTRIES', f.log.childNodes.length === SR.MAX_ENTRIES, f.log.childNodes.length);
check('the oldest go first', f.log.texts()[0] === 'line 6' && f.log.texts()[SR.MAX_ENTRIES - 1] === 'line ' + (SR.MAX_ENTRIES + 5), f.log.texts()[0]);

// A prompt (a piece with no line break at its end) repeated unchanged is
// skipped; a changed prompt is announced.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
sr.write('A rat arrives.\r\n');
sr.write('[HP:20/20]: ');
sr.write('The rat squeaks.\r\n');
sr.write('[HP:20/20]: ');
sr.write('The rat bites you.\r\n');
sr.write('[HP:17/20]: ');
check('an unchanged prompt is not read again', f.log.texts().join('|') === 'A rat arrives.|[HP:20/20]:|The rat squeaks.|The rat bites you.|[HP:17/20]:', f.log.texts().join('|'));

// ---- the phrase ------------------------------------------------------------

// Before login: turn on, remember, report, then say so and repeat the
// waiting prompt, in the game window and in the log.
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.phrase('on');
check('phrase on: mode on', sr.isOn(), '');
check('phrase on: remembered', f.store.screenReader === 'true', JSON.stringify(f.store));
check('phrase on: reported', f.reports.join() === 'true', f.reports.join());
check('phrase on: confirmed, then the waiting prompt, in the window', f.lines.join('|') === SR.SAID_ON + '|' + f.waiting[0], f.lines.join('|'));
check('phrase on: and in the log', f.log.texts().join('|') === SR.SAID_ON + '|' + f.waiting[0].trim(), f.log.texts().join('|'));
check('phrase on: no set command before login', f.setCommands === 0, '');
sr.phrase('off');
check('phrase off: mode off and remembered', !sr.isOn() && f.store.screenReader === 'false', '');
check('phrase off: the log is emptied', f.log.childNodes.length === 0, f.log.texts().join('|'));
check('phrase off: spoken by the status region', f.status === SR.SAID_OFF, f.status);
check('phrase off: shown in the game window', f.lines[f.lines.length - 1] === SR.SAID_OFF, f.lines.join('|'));

// In game, account differs: send set screenreader once; the server's line speaks.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
check('in game, account off: set command sent', f.setCommands === 1, '');
check('in game: the command names the state (on), never a toggle', f.setValues.join() === 'true', f.setValues.join());
check('in game, account off: no local line', f.lines.length === 0, f.lines.join('|'));
sr.phrase('on');
check('in game: a second phrase does not toggle it back', f.setCommands === 1, 'set commands ' + f.setCommands);

// A repeated Char.Info value is not news.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
sr.account(false);   // stale: the same value as the last push
check('stale push: mode stays on', sr.isOn(), '');
sr.phrase('on');
check('stale push: no second set command', f.setCommands === 1, 'set commands ' + f.setCommands);
sr.account(true);    // the server's real answer
check('real push: mode on', sr.isOn(), '');
f.setOk = false;
sr.phrase('off');
check('local off: mode off', !sr.isOn(), '');
sr.loggedOut();
sr.account(true);
check('after logout: the same value applies again', sr.isOn(), '');

// In game, the set command could not be sent: confirm locally instead.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
f.setOk = false;
sr.account(false);
sr.phrase('on');
check('in game, send failed: local line', f.lines[0] === SR.SAID_ON, f.lines.join('|'));

// In game, account already matches: no set command, local line, no prompt repeat.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(true);
f.lines = [];
sr.phrase('on');
check('in game, account on: no set command', f.setCommands === 0, '');
check('in game, account on: local line only', f.lines.join('|') === SR.SAID_ON, f.lines.join('|'));

// The account's setting arrives: the mode follows it and is remembered.
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.account(true);
check('account on: mode on, remembered', sr.isOn() && f.store.screenReader === 'true', '');
sr.write('Hello.\r\n');
sr.account(false);
check('account off: mode off, remembered, log emptied', !sr.isOn() && f.store.screenReader === 'false' && f.log.childNodes.length === 0, '');

// Logged out: the account is forgotten. inGame stays true here so only
// loggedOut can explain the missing set command.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.loggedOut();
sr.phrase('on');
check('after logout: no set command', f.setCommands === 0, 'set commands ' + f.setCommands);
check('after logout: local line', f.lines[0] === SR.SAID_ON, f.lines.join('|'));

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
```

- [ ] **Step 2: Replace the module** with exactly this content.

```js
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

    // plainText is what a screen reader should hear of a piece of terminal
    // text: no escape sequences (colour, cursor moves, the line erase before
    // each message), no carriage returns, no trailing spaces on a line, and no
    // blank lines at either end.
    function plainText(s) {
        return String(s)
            .replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '')
            .replace(/\x1b\][^\x07]*\x07/g, '')
            .replace(/\r/g, '')
            .split('\n').map(function (l) { return l.replace(/\s+$/, ''); }).join('\n')
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
    // character is logged in).
    function create(opts) {
        var on = false;
        var accountSetting = null; // the account's setting as this page believes it
        var lastPushed = null;     // the last Char.Info.screenreader value seen
        var lastPrompt = null;     // the last unterminated piece announced

        function remember(value) {
            try { opts.storage.setItem(STORAGE_KEY, JSON.stringify(value)); } catch (e) { /* storage blocked: this visit only */ }
        }

        function clearLog() {
            while (opts.log.firstChild) opts.log.removeChild(opts.log.firstChild);
            lastPrompt = null;
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
        // with no line break at its end is a prompt; one identical to the
        // last prompt announced is skipped, so a prompt redrawn unchanged
        // after every message is not read again (a changed one is).
        function speak(raw) {
            var text = plainText(raw);
            if (!text.trim()) return;
            var unterminated = !/\n\s*$/.test(String(raw).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, ''));
            if (unterminated) {
                if (text === lastPrompt) return;
                lastPrompt = text;
            }
            opts.log.appendChild(opts.createEntry(text));
            while (opts.log.childNodes.length > MAX_ENTRIES) opts.log.removeChild(opts.log.firstChild);
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
                }
            },

            // The connection closed: the next login names the account again.
            loggedOut: function () {
                accountSetting = null;
                lastPushed = null;
            }
        };
    }

    return {
        STORAGE_KEY: STORAGE_KEY, MAX_ENTRIES: MAX_ENTRIES,
        LABEL_OFF: LABEL_OFF, LABEL_ON: LABEL_ON,
        HINT_OFF: HINT_OFF, HINT_ON: HINT_ON,
        SAID_ON: SAID_ON, SAID_OFF: SAID_OFF,
        matchPhrase: matchPhrase,
        plainText: plainText,
        create: create
    };
}));
```

- [ ] **Step 3: Run** `node --check _datafiles/html/public/static/js/screenreader.js`. The module checks of `node tools/jstest/screenreader.test.js` pass; its two page checks ("the page never turns on xterm screenReaderMode", "the page has the log region") fail until Task 11. Commit both files together with Task 11 (one commit), so no commit leaves the test red.

### Task 11: The page feeds the log

**Files:** `_datafiles/html/public/webclient-pure.html`, `_datafiles/html/public/static/css/dashboard.css`, `tools/jstest/roomtitlebold.test.js`, `tools/jstest/webclient-connect.test.js`.

- [ ] **Step 1: The log region.** After the `#sr-status` line (`:251`), add:

```html
    <!-- Game text for screen readers, one entry per piece, seen by no one
         (static/js/screenreader.js; spec section 11). -->
    <div id="sr-log" class="visually-hidden" role="log" aria-live="polite" aria-relevant="additions" aria-label="Game text"></div>
```

- [ ] **Step 2: The opts.** In the `window.ScreenReader.create({...})` call, remove the `term`, `termWrite` and `clearSpoken` entries (with `clearSpoken`'s comment), and add:

```js
            log: document.getElementById('sr-log'),
            createEntry: function (text) {
                var entry = document.createElement('div');
                entry.textContent = text;
                return entry;
            },
```

Keep `storage`, `input`, `hint`, `status`, `report`, `setCommand`, `writeLine`, `currentLines`, `inGame`, and `screenReader.load();`.

- [ ] **Step 3: Server text.** Replace

```js
                // Through the screen reader switch (static/js/screenreader.js):
                // straight to xterm when the mode is off.
                screenReader.write(window.RoomTitleBold ? window.RoomTitleBold.apply(shown) : shown);
```

with

```js
                term.write(window.RoomTitleBold ? window.RoomTitleBold.apply(shown) : shown);
                // The same text, as plain words, into the screen reader log
                // (static/js/screenreader.js); nothing when the mode is off.
                screenReader.write(shown);
```

- [ ] **Step 4: Remove the answer tracking calls**, each with its comment: `screenReader.inputDone(d.seq);` in the `Char.Input.Done` handler; `screenReader.inputSent(linesSent);` after the copyover token's `socket.send(t);` (rewrite that block's comment to drop the answer wording, keep `linesSent++;` directly followed by `socket.send(t);`); `screenReader.inputSent(linesSent);` in `SendData`; and the `keyTyped` line at the top of the command box keydown handler. `grep -n "inputSent\|inputDone\|keyTyped\|clearSpoken\|termWrite\|screenReaderMode\|_accessibilityManager" _datafiles/html/public/webclient-pure.html` must print nothing.

- [ ] **Step 5: CSS.** Remove the `.xterm .xterm-accessibility { pointer-events: none; }` rule and its comment from `dashboard.css` (`:1968-1971`); xterm's overlay is never created now.

- [ ] **Step 6: Tests.** In `roomtitlebold.test.js`, put the server-text regex back to `/term\.write\(window\.RoomTitleBold \? window\.RoomTitleBold\.apply\(shown\) : shown\);/`. In `webclient-connect.test.js`, drop `inputSent` from the `screenReader` stand-in and its comment, and make its `write` a no-op (`write: function () {}`), since the page now writes to `term` itself.

- [ ] **Step 7: Gates and commit.** Run `node tools/jstest/screenreader.test.js` (all passed), then every `tools/jstest/*.test.js` and `tools/webclient-tests/*.js`, and `node --check` on every non-vendored `.js`; no FAIL. Commit Tasks 10 and 11 as one commit: "feat(webclient): screen readers read a log of game text, not xterm's region (#733)".

### Task 12: Docs

- [ ] `internal/` and `modules/` docs need no change (the server is unchanged). Update `docs/README.md`'s `screenreader.js` row: "the web client's screen reader switch: the spoken opt-in phrase, device memory, the account's setting, and a hidden log of game text that screen readers read (spec section 11)". In `docs/PATCH_NOTES.md`'s screen reader entry, nothing names xterm; leave it. Commit "docs: screen reader log (#733)".

### Task 13: Checks and the owner's retest (controller)

- [ ] Restart the test server on the new head (kill only its own PID).
- [ ] Rewrite the headless check's speech assertions to read `#sr-log` entries instead of xterm's region: the overlay `.xterm-accessibility` never exists; after `screen reader on` the log holds "Screen reader mode is on." and the waiting prompt; typing at a prompt removes no entry; the login welcome is in the log with no "Too much output"; `help set` is one entry ending with its last line; a returning visit's log holds the words-only splash and the name prompt; `set screenreader off` empties the log.
- [ ] Full Go suite, lint, JS gate.
- [ ] Owner retests with Narrator, the same steps as before.
