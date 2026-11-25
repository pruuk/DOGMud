// commandchain.test.js: one limit for every command chain (a macro, a tick,
// a trigger's then or else commands), taken from the server's
// Char.Automation maxCommands; the editors count against it live and refuse
// to save past it (owner follow-up 2026-11-25).
//
// Run:  node tools/jstest/commandchain.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var ROOT = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var CC = require(path.join(ROOT, 'static', 'js', 'commandchain.js'));
var html = fs.readFileSync(path.join(ROOT, 'webclient-pure.html'), 'utf8');

var failures = 0;
function check(name, ok, detail) { if (ok) { console.log('ok   ' + name); } else { failures++; console.log('FAIL ' + name + '\n  ' + detail); } }

// ── The pure rules ─────────────────────────────────────────────────────
check('blank pieces are not commands', CC.count('n;e; ;s;') === 3, CC.count('n;e; ;s;'));
check('an empty chain is none', CC.count('') === 0 && CC.count(null) === 0, '');
check('the limit comes from the server', CC.limit({ maxCommands: 7 }) === 7, CC.limit({ maxCommands: 7 }));
check('a missing limit falls back to the shipped 10', CC.limit({}) === 10 && CC.limit(null) === 10 && CC.limit({ maxCommands: 0 }) === 10, '');
check('the count reads "3 of 10 commands"', CC.label(3, 10) === '3 of 10 commands', CC.label(3, 10));
check('the warning names the limit and suggests a split',
    CC.warning(10) === 'Too many commands: a chain can hold at most 10. Split a long route into two.', CC.warning(10));
check('ten fit and eleven do not', CC.fits('n;n;n;n;n;n;n;n;n;n', 10) && !CC.fits('n;n;n;n;n;n;n;n;n;n;n', 10), '');

// ── The page's counter, with a fake DOM ──────────────────────────────────
function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}
function el() {
    var cls = {};
    return { hidden: false, textContent: '', style: {}, attrs: {},
        setAttribute: function (k, v) { this.attrs[k] = v; },
        classList: { toggle: function (c, on) { if (on) cls[c] = true; else delete cls[c]; }, has: function (c) { return !!cls[c]; } } };
}
global.window = { CommandChain: CC };
global.CommandChain = CC;
global.document = { createElement: function () { return el(); } };
global.GMCPStructs = { Char: { Automation: { maxCommands: 3 } } };
(0, eval)(extract('function chainLimit() {', '// Get the next free macro number'));

function field(value) {
    var f = el(); f.value = value; f.listeners = {};
    f.insertAdjacentElement = function (where, node) { f.note = node; };
    f.addEventListener = function (ev, fn) { f.listeners[ev] = fn; };
    f.type = function (v) { f.value = v; f.listeners.input(); };
    return f;
}

var f = field('n;e');
var c = attachChainCount(f, '');
check('the count shows under the field', f.note.textContent === '2 of 3 commands', f.note.textContent);
f.type('n;e;s;w');
check('over the limit the note says so in words', f.note.textContent.indexOf('Too many commands') === 0 && f.note.classList.has('over'), f.note.textContent);
check('over the limit it does not fit', c.fits() === false, '');
f.type('n;e;s');
check('back within, it fits and the warning goes', c.fits() === true && !f.note.classList.has('over'), f.note.textContent);

var old = field('n;e;s;w;n');
var oc = attachChainCount(old, 'n;e;s;w;n');
check('a chain saved before the limit, left as it was, still saves', oc.fits() === true, '');
check('...though it shows the warning', old.note.classList.has('over'), old.note.textContent);
old.type('n;e;s;w;n;e');
check('...and once edited it must fit', oc.fits() === false, '');

// The trigger editor moves its then commands between the simple field and
// the if/else Then field. The field it lands in started empty, but compares
// with the trigger's saved chain, so an unchanged old chain still saves.
var moved = field('');
var mc = attachChainCount(moved, 'n;e;s;w;n');
moved.type('n;e;s;w;n');
check('an unchanged old chain moved to the other layout still saves', mc.fits() === true, '');
moved.type(' n;e;s;w;n ');
check('...compared trimmed, as the editor sends it', mc.fits() === true, '');

var hid = field('n'); hid.style.display = 'none';
attachChainCount(hid, '');
check('a hidden field hides its count', hid.note.hidden === true, '');

// ── The editors use it, and Save stays open on false ────────────────────
check('Save keeps the editor open when onSave returns false',
    /if \(typeof _onSave === "function" && _onSave\(\) === false\) return;/.test(html), '');
['if (!macroChain.fits()) return false;', 'if (!tickChain.fits()) return false;',
 '!simpleChain.fits() : (!thenChain.fits() || !elseChain.fits())) return false;'].forEach(function (s) {
    check('an editor refuses to save past the limit: ' + s, html.indexOf(s) >= 0, s);
});
// Each trigger field compares with the chain it holds: both then fields with
// the saved then commands, the else field with the saved else commands. So
// renaming an old long trigger, in either layout, still saves.
['attachChainCount(simpleThenArea, prefillThenCmds)', 'attachChainCount(thenArea, prefillThenCmds)',
 'attachChainCount(elseArea, prefillElseCmds)', 'attachChainCount(cmdsInput, prefillCmds)'].forEach(function (s) {
    check('the editor passes the saved chain: ' + s, html.indexOf(s) >= 0, s);
});
check('the page loads commandchain.js', html.indexOf('/static/js/commandchain.js') >= 0, '');

if (failures) { console.log(failures + ' failure(s)'); process.exit(1); }
console.log('all passed');
