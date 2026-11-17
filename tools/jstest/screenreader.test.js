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
check('every send tells the switch, with the password box state', page.indexOf("screenReader.sent(dataToSend, textInput.type === 'password');") >= 0, '');
check('playerSend marks the line as the player\'s', /if \(sent\) screenReader\.lineSent\(\);/.test(page), '');
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

// The server echoes every sent line as its own unterminated piece. The page
// calls sent(line) for every line, and lineSent() after it for a player's.
function playerLine(sr, line) { sr.sent(line); sr.lineSent(); }
function allTexts(f) { return f.log.texts().concat(f.answers.texts()); }

f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
playerLine(sr, 'look');
sr.write('look');
sr.write('\r\r\n');
sr.write('The Void\r\n');
check('the echo of the sent line is skipped and does not end the answer', f.answers.texts().join('|') === 'The Void' && f.log.childNodes.length === 0, 'answers ' + f.answers.texts().join('|') + ' log ' + f.log.texts().join('|'));
sr.write('[HP:unhurt]: ');
check('the status after an echoed command is read at once', f.log.texts().join('|') === '[HP:unhurt]:', f.log.texts().join('|'));
// A prompt with no echo owed ends the answer.
sr.lineSent();
sr.write('[HP:hurt]: ');
sr.write('A rat arrives.\r\n');
check('a prompt ends an answer that owes no echo', f.log.texts().slice(-1)[0] === 'A rat arrives.', f.log.texts().join('|'));

// In a fight: a round line and its prompt are already on the way when the
// player sends. The prompt before the echo must not end the answer, and the
// echo that comes after it is not read as the status.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
sr.write('[HP:unhurt]: ');
f.clock.advance(SR.PROMPT_GAP_MS);
playerLine(sr, 'look');
sr.write('The rat bites you.\r\n');
sr.write('[HP:hurt]: ');
sr.write('look');
sr.write('\r\r\n');
sr.write('The Void\r\n');
check('an echo after a round prompt is never read', allTexts(f).indexOf('look') < 0, allTexts(f).join('|'));
check('the answer still interrupts after a round prompt', f.answers.texts().slice(-1)[0] === 'The Void', 'answers ' + f.answers.texts().join('|'));
sr.write('[HP:hurt]: ');
sr.write('The rat squeaks.\r\n');
check('the prompt after the answer ends it', f.log.texts().slice(-1)[0] === 'The rat squeaks.', f.log.texts().join('|'));

// Moving quickly: two lines sent before either answer comes. Both answers
// interrupt.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
playerLine(sr, 'n');
playerLine(sr, 'n');
sr.write('n'); sr.write('\r\r\n'); sr.write('Room One\r\n'); sr.write('[HP:unhurt]: ');
sr.write('n'); sr.write('\r\r\n'); sr.write('Room Two\r\n'); sr.write('[HP:unhurt]: ');
check('two quick moves: both rooms interrupt', f.answers.texts().join('|') === 'Room One|Room Two', 'answers ' + f.answers.texts().join('|') + ' log ' + f.log.texts().join('|'));
check('two quick moves: no echo is read', allTexts(f).indexOf('n') < 0, allTexts(f).join('|'));

// The page's own lines (automation, its own commands) are echoed too: the
// echo is skipped and does not end an answer, and their answers queue.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
playerLine(sr, 'look');
sr.write('look');
sr.sent('kill rat');
sr.write('kill rat');
sr.write('The Void\r\n');
check('an automation echo is skipped and does not end an answer', f.answers.texts().join('|') === 'The Void' && allTexts(f).indexOf('kill rat') < 0, allTexts(f).join('|'));
sr.write('[HP:unhurt]: ');
sr.sent('set screenreader on');
sr.write('set screenreader on');
sr.write('ScreenReader mode toggled.\r\n');
check('the page\'s own command echo is skipped; its answer queues', f.log.texts().slice(-1)[0] === 'ScreenReader mode toggled.' && allTexts(f).indexOf('set screenreader on') < 0, f.log.texts().join('|'));

// What the server echoes: * for a password, nothing for an item @handle line.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
sr.sent('Secret-1', true);
sr.write('********');
check('a masked echo is skipped', allTexts(f).length === 0, allTexts(f).join('|'));
sr.write('*');
check('a lone * is never read', allTexts(f).length === 0, allTexts(f).join('|'));
f.inGame = true;
playerLine(sr, 'look @1-000000000000000-01-00000000000000');
sr.write('A plain knife.\r\n');
sr.write('[HP:unhurt]: ');
sr.write('A rat arrives.\r\n');
check('an item handle line owes no echo; its prompt ends the answer', f.answers.texts().join('|') === 'A plain knife.' && f.log.texts().slice(-1)[0] === 'A rat arrives.', 'answers ' + f.answers.texts().join('|') + ' log ' + f.log.texts().join('|'));

// A lost echo cannot keep the answer open.
f = fakes('true');
sr = SR.create(f.opts);
sr.load();
f.inGame = true;
playerLine(sr, 'look');
f.clock.advance(SR.ECHO_WAIT_MS);
sr.write('[HP:unhurt]: ');
sr.write('A rat arrives.\r\n');
check('an echo owed past the wait is taken as lost', f.log.texts().slice(-1)[0] === 'A rat arrives.', 'answers ' + f.answers.texts().join('|'));
for (var e = 0; e < SR.MAX_ECHOES + 5; e++) sr.sent('say ' + e);
f.clock.advance(SR.PROMPT_GAP_MS);
sr.write('say 0');
check('echoes owed are capped; the oldest is dropped', f.log.texts().slice(-1)[0] === 'say 0', f.log.texts().join('|'));
// Mode off: nothing is owed.
sr.phrase('off');
sr.sent('look');
sr.phrase('on');
f.clock.advance(SR.PROMPT_GAP_MS);
sr.write('look');
check('a line sent while off owes no echo', f.log.texts().slice(-1)[0] === 'look', f.log.texts().join('|'));

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
