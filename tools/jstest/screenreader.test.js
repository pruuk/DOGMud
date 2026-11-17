// screenreader.test.js: the web client's screen reader switch (touch arc
// Task 0, docs/superpowers/specs/2026-11-17-screen-reader-task0-design.md).
//
// Run:  node tools/jstest/screenreader.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var pub = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var SR = require(path.join(pub, 'static', 'js', 'screenreader.js'));

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// ---- the xterm internals the page relies on ---------------------------------
// The page resets xterm's spoken region through private names. An xterm
// upgrade that renames them must fail here, not go silent for blind players.
var xterm = fs.readFileSync(path.join(pub, 'static', 'js', 'xterm.4.19.0.js'), 'utf8');
check('xterm has _accessibilityManager', xterm.indexOf('this._accessibilityManager=new') >= 0, '');
check('xterm has _clearLiveRegion', xterm.indexOf('_clearLiveRegion=function') >= 0, '');
// On a Mac (iPadOS Safari reports MacIntel too) a reset takes the region out
// of the page; the page puts it back through these names.
check('xterm has _liveRegion and _accessibilityTreeRoot', xterm.indexOf('this._liveRegion') >= 0 && xterm.indexOf('_accessibilityTreeRoot.appendChild(') >= 0, '');
check('xterm announces up to 20 lines, then goes silent on the 21st (CHUNK_LINES must stay under 20)', xterm.indexOf('21===this._liveRegionLineCount') >= 0 && SR.CHUNK_LINES < 21, '');

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

// ---- fakes -----------------------------------------------------------------

// A fake clock: timers run only when the test advances time.
function clock() {
    var now = 0, timers = [];
    return {
        setTimeout: function (f, ms) { var t = { at: now + ms, f: f }; timers.push(t); return t; },
        clearTimeout: function (t) { timers = timers.filter(function (x) { return x !== t; }); },
        advance: function (ms) {
            now += ms;
            var due = timers.filter(function (t) { return t.at <= now; });
            timers = timers.filter(function (t) { return t.at > now; });
            due.forEach(function (t) { t.f(); });
        }
    };
}

function fakes(saved) {
    var c = clock();
    var f = {
        mode: null, label: null, hint: null, status: '', lines: [], reports: [], setCommands: 0, setValues: [],
        cleared: 0, waiting: ['Username (or "new"): '], setOk: true,
        written: [],        // every piece handed to xterm, in order
        events: [],         // 'w:<piece>' and 'clear', interleaved, in order
        callbacks: [],      // write callbacks not yet run (a piece being parsed)
        store: saved === undefined ? {} : { screenReader: saved }, inGame: false, clock: c
    };
    f.parse = function () { var cb = f.callbacks.shift(); if (cb) cb(); };
    f.parseAll = function () { while (f.callbacks.length) f.parse(); };
    f.opts = {
        term: { setOption: function (k, v) { if (k === 'screenReaderMode') f.mode = v; } },
        termWrite: function (data, cb) {
            f.written.push(data); f.events.push('w:' + data);
            if (cb) f.callbacks.push(cb);
        },
        storage: {
            getItem: function (k) { return Object.prototype.hasOwnProperty.call(f.store, k) ? f.store[k] : null; },
            setItem: function (k, v) { f.store[k] = v; }
        },
        input: { setAttribute: function (k, v) { if (k === 'aria-label') f.label = v; } },
        hint: {},
        status: {},
        report: function (on) { f.reports.push(on); return true; },
        setCommand: function (value) { f.setCommands++; f.setValues.push(value); return f.setOk; },
        writeLine: function (t) { f.lines.push(t); },
        currentLines: function () { return f.waiting.slice(); },
        clearSpoken: function () { f.cleared++; f.events.push('clear'); },
        inGame: function () { return f.inGame; },
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
function countNewlines(s) { return (s.match(/\n/g) || []).length; }

// ---- load and storage ------------------------------------------------------

var f = fakes();
var sr = SR.create(f.opts);
sr.load();
check('fresh device: off', f.mode === false && !sr.isOn(), 'mode ' + f.mode);
check('fresh device: label names the phrase', f.label === SR.LABEL_OFF, f.label);
check('fresh device: hint names the phrase', f.hint === SR.HINT_OFF, f.hint);
check('fresh device: nothing reported or stored', f.reports.length === 0 && !('screenReader' in f.store), JSON.stringify(f.store));

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
check('remembered: on at load', f.mode === true && sr.isOn(), 'mode ' + f.mode);
check('remembered: label shortened', f.label === SR.LABEL_ON, f.label);

f = fakes();
f.opts.storage = { getItem: function () { throw new Error('blocked'); }, setItem: function () { throw new Error('blocked'); } };
sr = SR.create(f.opts);
sr.load();
check('blocked storage: off, no throw', f.mode === false, '');
sr.phrase('on');
check('blocked storage: still turns on', f.mode === true, '');

// ---- writing: off means untouched ------------------------------------------

f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
var big = nLines(50, 'L');
sr.inputSent(1);
sr.write(big);
check('off: written whole, once, no callback', f.written.length === 1 && f.written[0] === big && f.callbacks.length === 0, JSON.stringify(f.written.length));
f.clock.advance(SR.QUIET_MS * 3);
check('off: never resets a region that is off', f.cleared === 0, 'cleared ' + f.cleared);
f.inGame = false;
sr.write(big);
check('off before login: written whole too', f.written.length === 2 && f.callbacks.length === 0, f.written.length);

// ---- writing: an answer is read in full ------------------------------------

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
check('sending a line resets the region', f.cleared === 1, 'cleared ' + f.cleared);
sr.write(big);
f.parseAll();
check('answer: every line written, in order', f.written.join('') === big, '');
check('answer: no piece over CHUNK_LINES', f.written.every(function (p) { return countNewlines(p) <= SR.CHUNK_LINES; }), f.written.map(countNewlines).join());
check('answer: more than one piece for 50 lines', f.written.length >= 3, f.written.length);
// After every piece is parsed, the region is reset before the next piece.
var ok = true, sinceClear = 0;
f.events.forEach(function (e) {
    if (e === 'clear') { sinceClear = 0; return; }
    sinceClear += countNewlines(e.slice(2));
    if (sinceClear > SR.CHUNK_LINES) ok = false;
});
check('answer: never more than CHUNK_LINES between resets', ok, f.events.join(' | ').slice(0, 300));

// Order holds when more text arrives while a piece is still being parsed.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.write(nLines(30, 'A'));
sr.write('after\r\n');
check('queued: later text waits for the piece being parsed', f.written.join('').indexOf('after') < 0, f.written.join(''));
f.parseAll();
check('queued: everything written, in order', f.written.join('') === nLines(30, 'A') + 'after\r\n', f.written.join(''));

// The mode goes off while pieces are queued: every line is still written, in
// order, and no reset is made once the mode is off.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(true);
sr.inputSent(1);
sr.write(big);
f.setOk = false;
var clearedBefore = f.cleared;
sr.phrase('off');
check('mode off locally while queued', f.mode === false, 'mode ' + f.mode);
f.parseAll();
check('mode off mid-queue: every line still written, in order', f.written.join('') === big, f.written.length);
check('mode off mid-queue: no reset after the mode went off', f.cleared === clearedBefore, 'cleared ' + f.cleared + ' vs ' + clearedBefore);

// The answer ends QUIET_MS after its Char.Input.Done. Output in that gap is
// still part of it: a macro's commands and anything run through
// user.Command() are queued for later turns, so they print after the Done.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.write('reply\r\n');
f.parseAll();
sr.inputDone(1);
f.written = [];
sr.write(big);
f.parseAll();
check('just after Done (a macro\'s output): still read in full', f.written.length >= 3, f.written.length);
f.clock.advance(SR.QUIET_MS - 1);
f.written = [];
sr.write(big);
f.parseAll();
check('output keeps the answer open', f.written.length >= 3, f.written.length);
f.clock.advance(1); // QUIET_MS since the Done, but output came 1ms ago
f.written = [];
sr.write(big);
f.parseAll();
check('still open QUIET_MS after the Done, because output kept coming', f.written.length >= 3, f.written.length);
f.clock.advance(SR.QUIET_MS);
f.written = []; f.events = [];
sr.write(big);
check('quiet after Done: unprompted text written whole', f.written.length === 1 && f.written[0] === big, f.written.length);
f.parseAll();

// Lines are matched by number: a Done for an earlier line (an automation
// command, a numpad move) does not end a later line's answer.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
sr.inputSent(2);
sr.inputDone(1);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('Done for line 1 while line 2 is out: still answering', f.written.length >= 3, f.written.length);
sr.inputDone(2);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('Done for the last line sent, then quiet: answer over', f.written.length === 1, f.written.length);

// A Done numbered past the last line sent also ends it (the server counted a
// line this page did not, such as a reconnect token).
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(4);
sr.inputDone(5);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('a later Done ends it', f.written.length === 1, f.written.length);

// A line sent before login (the creation confirm "y") opens an answer: the
// welcome burst after Char arrives is read in full. No Char.Input.Done comes
// for it, so a quiet gap ends it.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = false;
sr.inputSent(1);
f.inGame = true;
f.written = [];
sr.write(big);
f.parseAll();
check('a login line opens an answer: the welcome burst is pieced', f.written.length >= 3, f.written.length);
f.clock.advance(SR.QUIET_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('a login line answer ends after a quiet gap', f.written.length === 1, f.written.length);

// Typing clears the spoken region, as xterm does for a key in its own input.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
sr.keyTyped();
check('mode on: a key clears the spoken region', f.cleared === 1, 'cleared ' + f.cleared);
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.keyTyped();
check('mode off: a key clears nothing', f.cleared === 0, 'cleared ' + f.cleared);

// A lost Done does not leave answers on forever.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.inputSent(1);
f.clock.advance(SR.ANSWER_MAX_MS);
f.written = [];
sr.write(big);
f.parseAll();
check('lost Done: answer ends after ANSWER_MAX_MS', f.written.length === 1, f.written.length);

// Before login everything is read in full: the connect splash and the
// prompts must reach a returning blind player, and there is no Done yet.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = false;
f.written = [];
sr.write(big);
f.parseAll();
check('before login: read in full', f.written.length >= 3 && f.written.join('') === big, f.written.length);

// ---- writing: unprompted output resets after a quiet gap -------------------

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('round 1\r\n');
f.clock.advance(SR.QUIET_MS - 1);
sr.write('round 1 more\r\n');
f.clock.advance(SR.QUIET_MS - 1);
check('output still arriving: no reset', f.cleared === 0, 'cleared ' + f.cleared);
f.clock.advance(1);
check('output quiet: reset once', f.cleared === 1, 'cleared ' + f.cleared);
check('QUIET_MS is under a 4 second combat round', SR.QUIET_MS < 4000, SR.QUIET_MS);

// ---- the phrase ------------------------------------------------------------

// Before login: turn on, remember, report, then say so and repeat the
// waiting prompt (a prompt already on screen is never read by xterm).
f = fakes();
sr = SR.create(f.opts);
sr.load();
sr.phrase('on');
check('phrase on: mode on', f.mode === true, '');
check('phrase on: remembered', f.store.screenReader === 'true', JSON.stringify(f.store));
check('phrase on: reported', f.reports.join() === 'true', f.reports.join());
check('phrase on: confirmed, then the waiting prompt', f.lines.join('|') === SR.SAID_ON + '|' + f.waiting[0], f.lines.join('|'));
check('phrase on: no set command before login', f.setCommands === 0, '');
sr.phrase('off');
check('phrase off: mode off and remembered', f.mode === false && f.store.screenReader === 'false', '');
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

// A repeated Char.Info value is not news: the page's generic Char handler
// re-reads the last Info for Char.Enemies, Char.Stats and the like, and that
// stale value must not undo the phrase before the server answers.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.phrase('on');
sr.account(false);   // stale: the same value as the last push
check('stale push: mode stays on', f.mode === true, 'mode ' + f.mode);
sr.phrase('on');
check('stale push: no second set command', f.setCommands === 1, 'set commands ' + f.setCommands);
sr.account(true);    // the server's real answer
check('real push: mode on', f.mode === true, '');
// The mode goes off on this device only (the set command could not be
// sent), then a new login pushes the SAME value as before: it must apply.
f.setOk = false;
sr.phrase('off');
check('local off: mode off', f.mode === false, 'mode ' + f.mode);
sr.loggedOut();
sr.account(true);
check('after logout: the same value applies again', f.mode === true, 'mode ' + f.mode);

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
check('account on: mode on, remembered', f.mode === true && f.store.screenReader === 'true', '');
sr.account(false);
check('account off: mode off, remembered', f.mode === false && f.store.screenReader === 'false', '');

// Logged out: the account and any open answer are forgotten. inGame stays
// true here so only loggedOut can explain the missing set command.
f = fakes();
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.account(false);
sr.inputSent(1);
sr.loggedOut();
sr.phrase('on');
check('after logout: no set command', f.setCommands === 0, 'set commands ' + f.setCommands);
check('after logout: local line', f.lines[0] === SR.SAID_ON, f.lines.join('|'));
f.written = [];
sr.write(big);
f.parseAll();
check('after logout: no answer left open', f.written.length === 1, f.written.length);

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
