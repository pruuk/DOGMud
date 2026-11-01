// motdrestyle.test.js — the web client shows the Message of the Day without
// its box (design 2026-11-01).
//
// Run:  node tools/jstest/motdrestyle.test.js
//
// The fixture copies the bytes the server sends for `motd` (captured from a
// local server 2026-11-01, internal/usercommands/motd.go motdBox): colour
// codes, the ╔ ╠ ╚ rules, side bars and right padding. If motdBox changes
// shape, the "unknown shape is left alone" checks are what keep the client
// safe; update this fixture to the new shape.
//
// Dependency-free and assertion-library-free, matching renderguard.test.js.
// Exits non-zero on failure.

var path = require('path');
var fs = require('fs');

var MotdRestyle = require(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'motdrestyle.js'
));

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + JSON.stringify(got) + '\n  want: ' + JSON.stringify(want));
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

var E = '\x1b';
var RULE = new Array(78).join('═'); // 77 rules, as motdBox draws at width 80
function row(text) { return ' ║ ' + text + new Array(77 - text.length).join(' ') + '║'; }

var box = [
    E + '[G' + E + '[2K' + E + '[38;5;75m' + E + '[49m' + E + '[38;5;3m' + E + '[49m ╔' + RULE + '╗',
    ' ║  ' + E + '[38;5;14m' + E + '[49m.:  M E S S A G E   O F   T H E   D A Y' + E + '[38;5;3m' + E + '[49m' + new Array(37).join(' ') + '║',
    ' ╠' + RULE + '╣',
    row('LIGHT AND DARK. Rooms are now bright, dim or pitch black.'),
    row('Full patch notes: https://example.org/PATCH_NOTES.md'),
    ' ╚' + RULE + '╝' + E + '[38;5;75m' + E + '[49m' + E + '[0m',
    ''
].join('\r\n');

var out = MotdRestyle.restyle(box);
var visible = out.replace(/\x1b\[[0-9;]*[A-Za-z]/g, '').split('\r\n');

check('no box characters left', /[╔╗╠╣╚╝║═]/.test(out), false);
check('three lines and the trailing newline', visible.length, 4);
check('title starts at the left edge', visible[0], '.:  M E S S A G E   O F   T H E   D A Y');
check('text starts at the left edge, no padding',
    visible[1], 'LIGHT AND DARK. Rooms are now bright, dim or pitch black.');
check('a link line keeps its text', visible[2], 'Full patch notes: https://example.org/PATCH_NOTES.md');
check('title keeps its own colour', out.indexOf(E + '[1;36m.:  M E S S A G E') !== -1, true);
check('text lines use the default colour, not the box yellow',
    out.split('\r\n')[1].indexOf(E + '[0mLIGHT AND DARK') === 0, true);
check('no yellow anywhere', out.indexOf('38;5;3m') === -1, true);
check('the line clear before the box is kept', out.indexOf(E + '[G' + E + '[2K') === 0, true);
check('ends on the default colour', /\x1b\[0m$/.test(out.split('\r\n')[2]), true);

// Text around the box, in the same message, is untouched.
var around = 'before\r\n' + box + 'after';
var aroundOut = MotdRestyle.restyle(around);
check('text before the box is kept', aroundOut.indexOf('before\r\n') === 0, true);
check('text after the box is kept', /after$/.test(aroundOut), true);

// Anything that is not the box we know comes back unchanged.
check('ordinary text unchanged', MotdRestyle.restyle('You see a lantern.\r\n'), 'You see a lantern.\r\n');
check('non-string unchanged', MotdRestyle.restyle(null), null);
var noBottom = box.replace(/ ╚═+╝/, '');
check('box without a bottom rule unchanged', MotdRestyle.restyle(noBottom), noBottom);
var otherBox = box.replace(' ║ LIGHT', ' | LIGHT');
check('a row of another shape leaves the box unchanged', MotdRestyle.restyle(otherBox), otherBox);
var mention = 'Type motd to read the M E S S A G E   O F   T H E   D A Y again.\r\n';
check('the title words outside a box unchanged', MotdRestyle.restyle(mention), mention);

// The page loads the script and passes incoming text through it.
var page = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');
check('webclient-pure.html loads motdrestyle.js', page.indexOf('static/js/motdrestyle.js') !== -1, true);
check('webclient-pure.html writes text through MotdRestyle.restyle',
    page.indexOf('MotdRestyle.restyle(event.data)') !== -1, true);

console.log(failures ? failures + ' of ' + checks + ' checks FAILED' : 'all ' + checks + ' checks passed');
process.exit(failures ? 1 : 0);
