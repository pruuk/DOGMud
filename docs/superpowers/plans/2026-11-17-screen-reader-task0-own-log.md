# Screen reader opt-in, addendum: our own reading area (touch arc Task 0)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace xterm's screen reader mode with a visually hidden `role="log"` region the page fills with each piece of game text and each of its own status lines, so screen readers read each piece once, never reread from the top, and never go silent.

**Architecture:** `static/js/screenreader.js` keeps its switch (phrase, device memory, account follow) and loses its timing machinery (pieces, quiet gaps, answer tracking, xterm private calls). Its `write(text)` turns terminal text into plain words (no escapes, no drawing symbols) and appends it to `#sr-log` as a new entry while the mode is on. The page writes server text to xterm as before and passes the same text to `screenReader.write`; its own status lines go through one helper, `gameLine`, that does both. The server sends a screen reader account a plain MOTD.

**Spec:** `docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md`, section 11 (owner ruling 2026-11-17, option 1 after the Narrator test). Branch `feature/screen-reader-task0`, worktree `C:/tmp/dogmud-sr0`. Follows plan `2026-11-17-screen-reader-task0.md` (Tasks 1 to 9). Revised after a blind adversarial review (two reviewers, 2026-11-17).

## Facts verified against source (worktree at `a9e43c7e0`)

| Fact | Where |
|---|---|
| The page writes each piece of server text at one place, through `screenReader.write(...)` today; `RELOGTKN:`, `TEXTMASK:` and `!!` frames return before it | `webclient-pure.html:3284`, `:3170` |
| `screenReader` is created with `term`, `termWrite`, `clearSpoken` (private `_accessibilityManager`) among its opts, before `init()` connects | `webclient-pure.html:553-597` |
| `inputDone(d.seq)`, `inputSent(linesSent)` after the copyover token and in `SendData`, and `keyTyped()` are the module calls this addendum removes | `webclient-pure.html:1126`, `:3143`, `:3483`, `:3395` |
| The page's own lines go straight to xterm: `onclose` (connection died, closed cleanly, left the game, server restarting, could not reconnect), "Not connected…", the automation login message, the name-prompt nudge, the full-screen refusal | `webclient-pure.html:3310-3340`, `:3454`, `:2920`, `:2990`, `:2824` |
| `webclient-connect.test.js` evaluates the page from `function markInGame() {` to the command box keydown listener, with a `screenReader` stand-in | `tools/jstest/webclient-connect.test.js:82-90` |
| `.visually-hidden` (with `white-space: nowrap`) ends at `dashboard.css:1968`; the overlay rule and its comment are `:1969-1971` | `dashboard.css` |
| The default prompt draws 10 block characters per bar and a sun or moon (`renderVitalBar`, `{t}`); the prompt redraw is its own frame with no line break and is not screen-reader stripped | `config.yaml:2489`; `internal/users/userrecord.prompt.go:217-236`; `internal/hooks/RedrawPrompt_SendRedraw.go` |
| The MOTD is a `║ ╠ ╣` box with a letter-spaced title, sent by the `motd` command, which login runs | `internal/usercommands/motd.go:26-100`; `config.yaml:76` |
| Each `SendTo` is one websocket frame; a message ends with a line break | `internal/connections` send queue; `userrecord.go:528` |
| Narrator reads a live region whose atomic property is not exposed as atomic, that is, whole | [Mozilla bug 1887790](https://bugzilla.mozilla.org/show_bug.cgi?id=1887790) |

## Rules for every task

- Edit files with the Edit and Write tools only; never write a repo file from the shell.
- `git add` named paths only; never `--no-verify`; never stash. Commit messages through a quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Doc comments on new or edited Go declarations. No em or en dashes in anything written.

---

### Task 10: The module writes to a log

**Files:** `_datafiles/html/public/static/js/screenreader.js` (replace), `tools/jstest/screenreader.test.js` (replace).

- [ ] **Step 1: Replace the test** with exactly this content and run it: it fails (the old module has no `plainText`, and the page checks need Task 11).

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
check('the page has the log region, not atomic (Narrator reads a region it takes as atomic whole)',
    /id="sr-log"[^>]*role="log"[^>]*aria-atomic="false"/.test(page), '');
check('the page passes the log to the switch', page.indexOf("log: document.getElementById('sr-log')") >= 0, '');
check('the page feeds server text to the log', page.indexOf('screenReader.write(shown);') >= 0, '');
var moduleSrc = fs.readFileSync(path.join(pub, 'static', 'js', 'screenreader.js'), 'utf8');
check('the module never reaches into xterm', !/setOption|_accessibilityManager|_clearLiveRegion|screenReaderMode/.test(moduleSrc.replace(/^\s*\/\/.*$/gm, '')), '');

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
// The real prompt (config.yaml Prompt, userrecord.prompt.go renderVitalBar):
// the bars and the sun or moon would be read out as symbol names.
var prompt = '\x1b[1G\x1b[2K[☀ HP:███████░░░ SP:██████████ CP:▒▒▒▒▒▒▒▒▒▒]:';
check('prompt bars and marks go', SR.plainText(prompt) === '[ HP: SP: CP: ]:', JSON.stringify(SR.plainText(prompt)));
check('MOTD box sides go', SR.plainText(' ║ Welcome back.        ║\r\n') === 'Welcome back.', JSON.stringify(SR.plainText(' ║ Welcome back.        ║\r\n')));
check('an OSC ended by ESC backslash goes, text after it stays', SR.plainText('\x1b]0;x\x1b\\A\r\nB\x07C\r\n') === 'A\nBC', JSON.stringify(SR.plainText('\x1b]0;x\x1b\\A\r\nB\x07C\r\n')));
check('charset escapes and control characters go', SR.plainText('\x1b(BHi\x07 there\r\n') === 'Hi there', JSON.stringify(SR.plainText('\x1b(BHi\x07 there\r\n')));

// ---- fakes -----------------------------------------------------------------

function fakeLog() {
    var log = { childNodes: [] };
    Object.defineProperty(log, 'firstChild', { get: function () { return log.childNodes[0] || null; } });
    log.appendChild = function (n) { log.childNodes.push(n); return n; };
    log.removeChild = function (n) { var i = log.childNodes.indexOf(n); if (i >= 0) log.childNodes.splice(i, 1); return n; };
    log.texts = function () { return log.childNodes.map(function (n) { return n.text; }); };
    return log;
}

// A fake clock: timers run only when the test advances time.
function clock() {
    var t = 0, timers = [];
    return {
        now: function () { return t; },
        setTimeout: function (fn, ms) { var x = { at: t + ms, fn: fn }; timers.push(x); return x; },
        clearTimeout: function (x) { timers = timers.filter(function (y) { return y !== x; }); },
        advance: function (ms) {
            t += ms;
            var due = timers.filter(function (x) { return x.at <= t; });
            timers = timers.filter(function (x) { return x.at > t; });
            due.forEach(function (x) { x.fn(); });
        }
    };
}

function fakes(saved) {
    var c = clock();
    var f = {
        clock: c,
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
        inGame: function () { return f.inGame; },
        now: c.now,
        setTimeout: c.setTimeout,
        clearTimeout: c.clearTimeout
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
// A health bar change is not news once the bars are gone: same words.
sr.write(prompt);
sr.write('You rest.\r\n');
sr.write(prompt.replace('█░', '██'));
check('a bar-only prompt change is not read again', f.log.texts().slice(-2).join('|') === '[ HP: SP: CP: ]:|You rest.', f.log.texts().slice(-2).join('|'));
// After a reconnect the same login prompt is asked again and must be read.
sr.write('Username (or "new"): ');
sr.loggedOut();
sr.write('Username (or "new"): ');
check('after a reconnect the same prompt is read again', f.log.texts().slice(-2).join('|') === 'Username (or "new"):|Username (or "new"):', f.log.texts().slice(-2).join('|'));

// In game the status is read at most once a round, and only when it changed.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('[HP:unhurt]: ');
f.clock.advance(1000);
sr.write('The rat bites you.\r\n');
sr.write('[HP:hurt]: ');
check('a changed status within a round is held; messages are not', f.log.texts().join('|') === '[HP:unhurt]:|The rat bites you.', f.log.texts().join('|'));
f.clock.advance(SR.PROMPT_GAP_MS - 1000);
check('it is read when the round is over', f.log.texts().join('|') === '[HP:unhurt]:|The rat bites you.|[HP:hurt]:', f.log.texts().join('|'));
// Changed and changed back within one round: nothing to say.
sr.write('[HP:badly hurt]: ');
sr.write('[HP:hurt]: ');
f.clock.advance(SR.PROMPT_GAP_MS);
check('a change undone within the round is not read', f.log.texts().slice(-1)[0] === '[HP:hurt]:' && f.log.childNodes.length === 3, f.log.texts().join('|'));
// Several changes in one round: only the latest is read.
f.clock.advance(SR.PROMPT_GAP_MS);
sr.write('[HP:badly hurt]: ');
f.clock.advance(500);
sr.write('[HP:near death]: ');
sr.write('[HP:badly hurt]: ');
sr.write('[HP:near death]: ');
f.clock.advance(SR.PROMPT_GAP_MS);
check('only the latest status of a round is read', f.log.texts().slice(-2).join('|') === '[HP:badly hurt]:|[HP:near death]:', f.log.texts().slice(-3).join('|'));
// Mode off while one is held: it is never read.
sr.write('[HP:hurt]: ');
f.clock.advance(100);
sr.write('[HP:unhurt]: ');
sr.phrase('off');
f.clock.advance(SR.PROMPT_GAP_MS);
check('a held status is dropped when the mode goes off', f.log.childNodes.length === 0, f.log.texts().join('|'));
// Before login, prompts are read at once.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
sr.write('Username (or "new"): ');
sr.write('Password: ');
check('login prompts are not held', f.log.texts().join('|') === 'Username (or "new"):|Password:', f.log.texts().join('|'));

// No log element: nothing throws, for sighted players too.
f = fakes('true');
f.opts.log = undefined;
sr = SR.create(f.opts);
var threw = false;
try { sr.load(); sr.write('Hi.\r\n'); sr.phrase('off'); } catch (e) { threw = e.message; }
check('a missing log never throws', threw === false, threw);

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
check('account off: spoken by the status region', f.status === SR.SAID_OFF, f.status);

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
```

- [ ] **Step 3:** `node --check` the module. Its own checks pass; the four page checks pass after Task 11. Commit Tasks 10 and 11 together (Task 11 Step 9), so no commit leaves the test red.

### Task 11: The page feeds the log

**Files:** `_datafiles/html/public/webclient-pure.html`, `_datafiles/html/public/static/css/dashboard.css`, `tools/jstest/roomtitlebold.test.js`, `tools/jstest/webclient-connect.test.js`.

- [ ] **Step 1: The log region.** Immediately before the main inline `<script>` that follows `#dashboard` (`:458`, after the `</div>` that closes `#dashboard`), add the region, so a phone screen reader swiping from the top meets the game before up to 100 log entries. `#sr-hint` stays the first element of `body`.

```html
    <!-- Game text for screen readers, one entry per piece, seen by no one
         (static/js/screenreader.js; spec section 11). aria-atomic="false"
         is explicit because Narrator reads a region it cannot see as
         non-atomic as a whole. -->
    <div id="sr-log" class="visually-hidden" role="log" aria-live="polite" aria-relevant="additions" aria-atomic="false" aria-label="Game text"></div>
```

- [ ] **Step 2: The opts.** In `window.ScreenReader.create({...})`, remove `term`, `termWrite` and `clearSpoken` (with its comment), and add:

```js
            log: document.getElementById('sr-log'),
            createEntry: function (text) {
                // One child per line, so line breaks survive in the
                // accessibility tree (.visually-hidden sets nowrap).
                var entry = document.createElement('div');
                text.split('\n').forEach(function (line) {
                    var row = document.createElement('div');
                    row.textContent = line;
                    entry.appendChild(row);
                });
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

- [ ] **Step 4: The page's own lines.** Immediately above `function markInGame() {` add (here, so `webclient-connect.test.js`'s extract includes it):

```js
        // gameLine writes one of the page's own lines into the game window
        // and, while screen reader mode is on, into the screen reader log,
        // so a blind player hears it (a dropped connection above all).
        function gameLine(text) {
            term.writeln(text);
            screenReader.write(text + '\r\n');
        }
```

Then change these `term.writeln(...)` (or `term.write('\r\n' + X + '\r\n')`) calls to `gameLine(...)` with the same text: every message in `socket.onclose` (connection died, closed cleanly, left the game, "Server is restarting…", "Could not reconnect…"), "Not connected to the server. Click Enter Game to play.", the automation login message, "Type your character name in the box below…", and the full-screen refusal. Leave the `term.writeln("")` blank lines (or fold them into the text), the reconnect heartbeat `term.write(".")`, and "Connected to the server!" (cleared at once) as they are. `writeLine` in the opts stays `term.write('\r\n' + text)` (the module speaks those lines itself).

- [ ] **Step 5: Remove the answer tracking calls**, each with its comment: `screenReader.inputDone(d.seq);`; `screenReader.inputSent(linesSent);` after the copyover token's `socket.send(t);` (keep `linesSent++;` directly followed by `socket.send(t);`) and in `SendData`; and the `keyTyped` line at the top of the command box keydown handler. Rewrite the stale comments at `:246-249` ("speaks what xterm's own region cannot") and `:3114-3115` ("before the token below opens an answer") to match. `grep -n "inputSent\|inputDone\|keyTyped\|clearSpoken\|termWrite\|screenReaderMode\|_accessibilityManager" _datafiles/html/public/webclient-pure.html` prints nothing.

- [ ] **Step 6: CSS.** Remove from `dashboard.css` the rule `.xterm .xterm-accessibility { pointer-events: none; }` and the comment above it (`:1969-1971`). Do not touch `.visually-hidden` (`:1959-1968`).

- [ ] **Step 7: Tests.** `roomtitlebold.test.js`: put the server-text regex back to `/term\.write\(window\.RoomTitleBold \? window\.RoomTitleBold\.apply\(shown\) : shown\);/`. `webclient-connect.test.js`: the `screenReader` stand-in drops `inputSent` and records writes, `write: function (t) { global.__srWrites.push(t); }` with `global.__srWrites = []` set before each case that checks it. Add checks, following the file's own patterns for driving the socket: a plain server frame reaches `__srWrites`; a `!!MUSIC(` frame and a `RELOGTKN:` frame do not; closing the socket puts its message ("Connection died" or whichever the case triggers) into `__srWrites`.

- [ ] **Step 8: Gates.** `node tools/jstest/screenreader.test.js` (all passed), every `tools/jstest/*.test.js` and `tools/webclient-tests/*.js`, and `node --check` on every non-vendored `.js`; no FAIL.

- [ ] **Step 9: Commit** Tasks 10 and 11 together: "feat(webclient): screen readers read a log of game text, not xterm's region (#733)".

### Task 11a: A plain MOTD for screen reader accounts (server)

**Files:** `internal/usercommands/motd.go`, a test beside the existing MOTD tests (`grep -ln "motdBox" internal/usercommands/*_test.go`).

- [ ] **Step 1: Test first.** Add `motdFor(text string, lw int, screenReader bool) string` tests: with `screenReader` true at 80 and at 42, the output contains the MOTD text, the words `Message of the day`, and none of `║ ╠ ╣ ═ ╔ ╗ ╚ ╝` and not `M E S S A G E`; with false it equals `motdBox(text, lw)`. Run: fails (undefined).

- [ ] **Step 2: Implement.**

```go
// motdFor is the MOTD as one reader gets it: the box (motdBox) for most,
// and plain words for a screen reader account, which would otherwise hear
// the box sides by name and the spaced title letter by letter (spec 2026-11-17,
// section 11).
func motdFor(text string, lw int, screenReader bool) string {
	if !screenReader {
		return motdBox(text, lw)
	}
	return "Message of the day.\n" + messaging.WrapAnsi(text, lw) + "\n"
}
```

and in `Motd`, send `motdFor(text, user.WrapWidth(), user.ScreenReader)` instead of `motdBox(...)`. Run the test: passes. Run `go test ./internal/usercommands/ ./internal/guards/ -count=1`. Commit "feat(motd): a plain MOTD for screen reader accounts (#733)".

### Task 11b: The status in words for screen reader accounts (server)

Owner ruling 2026-11-17: a screen reader account hears its status as words in four buckets, read at most once a round and only when it changed (the once-a-round part is the module's, Task 10). The bars would otherwise be read as block symbols, or, with symbols dropped, say nothing about health.

| Pool | 75% and up | 50 to 74% | 25 to 49% | below 25% |
|---|---|---|---|---|
| HP (`{hpbar}`, and the tank's `{tankbar}`) | unhurt | hurt | badly hurt | near death |
| SP (`{stbar}`) | fresh | tired | very tired | exhausted |
| CP (`{cvbar}`) | strong | steady | shaken | broken |

The percentage is measured against the reachable pool (the pool less its reservation), as `renderVitalBar` measures its colour (`internal/users/userrecord.prompt.go:165-210`).

**Files:** `internal/users/userrecord.prompt.go`, a test in `internal/users/` beside `prompt_tank_test.go`.

- [ ] **Step 1: Tests first.** A table test of `vitalWord` for each pool at 100, 75, 74, 50, 49, 25, 24 and 0 percent, with and without a reservation (e.g. health 60 of 100 with 20 reserved is 75% of the reachable 80: "unhurt"); `max <= 0` and `current < 0` do not panic. Then a prompt test built the way `prompt_tank_test.go` builds its user (read it and reuse its fixture; do not invent one): with `user.ScreenReader = true`, `ProcessPromptString("{hpbar} {stbar} {cvbar}")` at full pools gives `unhurt fresh strong` and contains no `█`, `░` or `▒`; with it false the output still contains `█`. If `prompt_tank_test.go` covers `{tankbar}`, add the same check for the tank. Run: fails (undefined).

- [ ] **Step 2: Implement** in `internal/users/userrecord.prompt.go`, beside `renderVitalBar`:

```go
// vitalWords are the four words a screen reader account hears for a pool in
// place of its bar, best first, for 75% and up, 50 to 74, 25 to 49 and below
// 25 (owner, 2026-11-17).
var vitalWords = map[string][4]string{
	`hp`: {`unhurt`, `hurt`, `badly hurt`, `near death`},
	`sp`: {`fresh`, `tired`, `very tired`, `exhausted`},
	`cp`: {`strong`, `steady`, `shaken`, `broken`},
}

// vitalWord is a pool's bucket word, measured against the reachable pool
// (maxValue less reserved) as renderVitalBar measures its colour.
func vitalWord(pool string, current, maxValue, reserved int) string {
	words := vitalWords[pool]
	effectiveMax := max(maxValue-max(reserved, 0), 1)
	current = min(max(current, 0), effectiveMax)
	pct := float64(current) / float64(effectiveMax) * 100.0
	switch {
	case pct >= 75:
		return words[0]
	case pct >= 50:
		return words[1]
	case pct >= 25:
		return words[2]
	default:
		return words[3]
	}
}
```

and in `ProcessPromptString`, at `{hpbar}`, `{stbar}`, `{cvbar}` and the bar inside `{tankbar}`, write `vitalWord(...)` with the same arguments when `u.ScreenReader` is true, and `renderVitalBar(...)` otherwise. Keep each case's other text (`{tankbar}`'s `Thp:` label) as it is. Run the tests, `go test ./internal/users/ -count=1`, gofmt, lint. Commit "feat(prompt): the status in words for screen reader accounts (#733)".

### Task 12: Docs

- [ ] **Spec** (`docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md`): mark as superseded by section 11, keeping their history, every place that still states xterm's mode as current: section 2 (`term.setOption('screenReaderMode', true)`), sections 3 and 5 ("xterm's mode"), ruling 1 ("xterm's screen reader mode stays off unless…": now "nothing is added to the log unless…"), ruling 5's reason (the 20-line limit no longer applies; light combat text stays because long bursts are slow to hear, and the owner should be told the stated reason changed), the general decisions on xterm's mode, the 2 second reset and the private xterm calls, and the Testing section's `.xterm-accessibility` line. In section 11 add: "Read in full" holds for the log; a screen reader may still cut a long entry short when the player types (NVDA interrupts speech on typed characters and Enter by default), and the entries stay in the log to read back line by line.- [ ] **docs/README.md**: the `tools/jstest/` row's screen reader note (`:106`, "resets of xterm's spoken region") and the spec row (`:319`, "turns on xterm's `screenReaderMode` at runtime") describe the log instead; the `screenreader.js` row reads "the web client's screen reader switch: the spoken opt-in phrase, device memory, the account's setting, and a hidden log of game text that screen readers read (spec section 11)".
- [ ] `internal/usercommands/context.md`: one line for `motdFor`. `internal/users/context.md`: one line for `vitalWord` and `vitalWords`. `docs/PATCH_NOTES.md` screen reader entry: add "With screen reader mode on, your status is spoken in words (for example unhurt, tired, steady), at most once a round and only when it changes."
- [ ] Commit "docs: screen reader log (#733)".

### Task 13: Checks and the owner's retest (controller)

- [ ] Restart the test server on the new head (kill only its own PID).
- [ ] Headless check, rewritten to read `#sr-log` (entries' `textContent`) and `#sr-status`:
  - sighted: with the mode off, `#sr-log` has no children after login, `look` and `help set`; `.xterm-accessibility` never exists;
  - `#sr-hint` is the first element of `body`; `page.getByRole('log', { name: 'Game text' })` is found (in the accessibility tree);
  - after `screen reader on`: entries "Screen reader mode is on." and the waiting prompt;
  - typing at a prompt removes no entry (regression check);
  - the creation burst after "y" is in the log, ending with the prompt;
  - `help set` is one entry whose last line names `help submission`;
  - after `look`, an unchanged prompt adds no entry; no entry contains `\x1b`, `<ansi`, a box character or a block character;
  - the in-game prompt entry reads the status in words (`HP:unhurt SP:fresh CP:strong` at full pools);
  - the MOTD entry reads "Message of the day." with no box characters;
  - a returning visit: the words-only splash and the name prompt are entries;
  - stopping the server's socket (close the page's socket from the test) adds the onclose message as an entry;
  - `screen reader off`: the log is empty and `#sr-status` says "Screen reader mode is off.".
- [ ] Full Go suite, lint, JS gate, telnet check (unchanged server paths plus the MOTD).
- [ ] Owner retests with Narrator. Pass criteria: no rereading from the top after a pause in typing; game text heard after typing; two messages a few seconds apart are each heard once (not the whole log again); `help set` heard to its end when not typing; a dropped connection heard; the MOTD heard as words; in a fight, the status heard in words, at most once a round and not when unchanged.
