# Screen reader opt-in, addendum 2: answers interrupt (touch arc Task 0)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The answer to a line the player sends interrupts whatever the screen reader is still reading, so a blind player moving quickly hears where they are now, not rooms ago. Everything else keeps queueing politely.

**Spec:** `docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md`, section 12 (owner ruling 2026-11-17). Follows `2026-11-17-screen-reader-task0-own-log.md`. Branch `feature/screen-reader-task0`, worktree `C:/tmp/dogmud-sr0`.

## Facts verified against source (worktree at `f8aa8d7cc`)

| Fact | Where |
|---|---|
| `#sr-log` sits before the main inline script, after `#dashboard` | `webclient-pure.html` (search `id="sr-log"`) |
| The switch is created with `log:` and `createEntry:` opts | `webclient-pure.html` (search `log: document.getElementById('sr-log')`) |
| Player-initiated sends: Enter `var sent = SendData(line);` (`:3451`), numpad `SendData(codeShortcuts[event.code]);` (`:3419`), F-key macro `SendData("=" + macroNum);` (`:3400`), chips `SendData("=" + num);` and `SendData(key);` (`:2311`, `:2313`), item menu `SendData(verb + " @" + handle);` (`:4233`), `SendData("remove @" + handle);` and `SendData("drop @" + handle);` (`:4279-4280`) | `webclient-pure.html` |
| Automation sends that must NOT count: `:1856`, `:1942`, `:2093`, `:2278`; also `quit` (`:3006`), the switch's `setCommand` (`:580`), and the panel's `set =` and `alias` writes (`:4685`, `:4716`, `:5154`, `:5156`) | `webclient-pure.html` |
| `hotinput.test.js` pins the Enter line text | `tools/jstest/hotinput.test.js:394` (`page.indexOf('var sent = SendData(line);')`) |
| The server redraws the prompt as its own unterminated frame after a command's output | `internal/hooks/RedrawPrompt_SendRedraw.go` |

## Rules

Edit/Write tools only; `git add` named paths; never `--no-verify`; never stash; heredoc commit messages ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; no em or en dashes.

---

### Task 14: Answers interrupt

**Files:** `_datafiles/html/public/static/js/screenreader.js` and `tools/jstest/screenreader.test.js` (replace both with the content below), `_datafiles/html/public/webclient-pure.html`, `tools/jstest/hotinput.test.js`.

- [ ] **Step 1: Replace the test** with exactly this content; run it: it fails (no `lineSent`, no `#sr-answers`).

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
check('the page has the assertive answers region, not atomic',
    /id="sr-answers"[^>]*role="log"[^>]*aria-live="assertive"[^>]*aria-atomic="false"/.test(page), '');
check('the page passes the answers region to the switch', page.indexOf("answers: document.getElementById('sr-answers')") >= 0, '');
check('the Enter key sends through playerSend', /var sent = playerSend\(line\);/.test(page), '');
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
        store: saved === undefined ? {} : { screenReader: saved }, inGame: false, log: fakeLog(), answers: fakeLog()
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
        answers: f.answers,
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

// The answer to a line the player sent interrupts: it goes to the assertive
// region until the prompt; anything else stays in the polite log.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('A rat squeaks.\r\n');
check('without a sent line, text queues in the log', f.log.texts().join('|') === 'A rat squeaks.' && f.answers.childNodes.length === 0, f.answers.texts().join('|'));
sr.lineSent();
sr.write('Trail North\r\nA narrow trail climbs between red rocks.\r\n');
sr.write('You see a rat here.\r\n');
check('the answer goes to the interrupting region', f.answers.texts().join('|') === 'Trail North\nA narrow trail climbs between red rocks.|You see a rat here.', f.answers.texts().join('|'));
sr.write('[HP:unhurt]: ');
sr.write('The rat bites you.\r\n');
check('the prompt ends the answer; later text queues again', f.log.texts().slice(-1)[0] === 'The rat bites you.' && f.answers.childNodes.length === 2, f.log.texts().join('|'));
sr.lineSent();
sr.loggedOut();
sr.write('Connected.\r\n');
check('logging out ends an answer', f.log.texts().slice(-1)[0] === 'Connected.', f.log.texts().join('|'));
sr.lineSent();
sr.write('Your answer.\r\n');
sr.phrase('off');
check('off empties both regions', f.log.childNodes.length === 0 && f.answers.childNodes.length === 0, '');
// Mode off: a sent line does not open an answer for later.
sr.lineSent();
sr.phrase('on');
sr.write('Plain text.\r\n');
check('a line sent while off opens no answer', f.log.texts().slice(-1)[0] === 'Plain text.' && f.answers.childNodes.length === 0, f.answers.texts().join('|'));

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
// The answer to a line the player sends (from lineSent until the game
// redraws the prompt) goes into a second, assertive log instead, so it cuts
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

    // opts: storage (localStorage, any call may throw); input (the command
    // box); hint (the hidden first paragraph); status (a visually hidden
    // role="status" element, for "off"); log (the visually hidden polite
    // role="log" element); answers (the visually hidden assertive role="log"
    // element, for the answer to a line the player sent); createEntry(text) (a new element holding text, for the log);
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
        var answering = false;     // the player sent a line; its answer has not ended

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

        function clearLog() {
            forgetPrompt();
            answering = false;
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
            // A prompt ends the answer to the line the player sent.
            answering = false;
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

            // The player sent a line (typed, the numpad, a macro key, a chip,
            // an item menu; never automation): its answer, up to the next
            // prompt, interrupts whatever is still being read.
            lineSent: function () {
                if (on) answering = true;
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
                answering = false;
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

- [ ] **Step 3: The answers region.** Directly after the `#sr-log` element, add:

```html
    <!-- The answer to a line the player sent, read at once, cutting off
         whatever is still being read (static/js/screenreader.js; spec
         section 12). -->
    <div id="sr-answers" class="visually-hidden" role="log" aria-live="assertive" aria-relevant="additions" aria-atomic="false" aria-label="Answers"></div>
```

- [ ] **Step 4: The opt.** In `window.ScreenReader.create({...})`, after `log: document.getElementById('sr-log'),` add `answers: document.getElementById('sr-answers'),`.

- [ ] **Step 5: `playerSend`.** Directly after the `gameLine` function, add:

```js
        // playerSend sends a line the player asked for (typed, the numpad, a
        // macro key, a chip, an item menu) and marks its answer for the
        // screen reader, so it interrupts what is still being read (spec
        // section 12). Automation and the page's own commands use SendData.
        function playerSend(line) {
            var sent = SendData(line);
            if (sent) screenReader.lineSent();
            return sent;
        }
```

Then change exactly these calls from `SendData(` to `playerSend(`, keeping their arguments: the Enter line (`var sent = SendData(line);` becomes `var sent = playerSend(line);`), the numpad shortcut, the F-key macro, both chip sends, the item menu `send(verb)`, and the item menu `remove` and `drop` pair. Change no other `SendData` call (the automation, `quit`, `setCommand` and panel writes stay).

- [ ] **Step 6: `hotinput.test.js:394`**: change the anchor string to `'var sent = playerSend(line);'`. If `webclient-connect.test.js` or another test evaluates code that now calls `playerSend`, give it what it needs following its own pattern.

- [ ] **Step 7: Gates.** `node tools/jstest/screenreader.test.js` (all passed), every `tools/jstest/*.test.js` and `tools/webclient-tests/*.js`, `node --check` on every non-vendored `.js`; no FAIL. `grep -n "playerSend(" _datafiles/html/public/webclient-pure.html` lists the definition and exactly 8 calls.

- [ ] **Step 8: Docs.** `docs/PATCH_NOTES.md`, screen reader entry: add "When you send a command, its answer is read at once, cutting off anything still being read; other text waits its turn. Press Ctrl to skip what is waiting. The web client reads best with NVDA or JAWS; Windows Narrator may skip or jumble text when a lot arrives at once." `docs/README.md`: the `screenreader.js` row also names the answers region. Commit everything as one commit: "feat(webclient): the answer to a command interrupts the screen reader backlog (#733)".

### Task 15: Checks and the owner's NVDA retest (controller)

- [ ] Restart the test server on the new head (kill only its own PID). Extend the headless check: after `look`, its room text is in `#sr-answers`, not `#sr-log`; text that arrives with no command sent is in `#sr-log`; the prompt is in `#sr-log`; `screen reader off` empties both.
- [ ] Full Go suite, lint, JS gate.
- [ ] Owner retests under NVDA: move quickly through several rooms; each new room is heard at once, without the backlog of earlier rooms; a fight's lines still come in order; Ctrl skips a queue; the Pothole Coulee shop `list` is read.
