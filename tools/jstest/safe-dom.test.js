// safe-dom.test.js — regression tests for the admin dashboards' HTML escaping
// (review finding 17, stored XSS through authored names and player names).
//
// Run:  node tools/jstest/safe-dom.test.js
//
// Deliberately dependency-free and assertion-library-free: the repo has no JS
// test runner yet (review finding 25 tracks adding JavaScript checks to CI).
// When that lands, this file is ready to be wired straight in — it exits
// non-zero on failure.

var path = require('path');

var SafeDom = require(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'admin', 'static', 'js', 'safe-dom.js'
));

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + got + '\n  want: ' + want);
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

var payload = '<img src=x onerror=alert(1)>';

// --- esc -------------------------------------------------------------------

check('esc neutralizes a script payload',
    SafeDom.esc(payload),
    '&lt;img src=x onerror=alert(1)&gt;');

check('esc handles both quote styles and ampersands',
    SafeDom.esc('a"b\'c&d'),
    'a&quot;b&#39;c&amp;d');

check('esc renders null as empty, not "null"', SafeDom.esc(null), '');
check('esc renders undefined as empty', SafeDom.esc(undefined), '');
check('esc leaves numbers alone', SafeDom.esc(42), '42');

// --- tr / td ---------------------------------------------------------------

// The finding's exact shape: a mob, shop, zone or character name carrying
// markup reaches an administrator's browser.
check('tr escapes a malicious name in a cell',
    SafeDom.tr([payload, 3]),
    '<tr><td>&lt;img src=x onerror=alert(1)&gt;</td><td>3</td></tr>');

check('raw() passes a trusted locally-built fragment through',
    SafeDom.tr([SafeDom.raw('<span class="badge">9</span>')]),
    '<tr><td><span class="badge">9</span></td></tr>');

check('cell attributes render in order',
    SafeDom.tr([{ v: 'x', colspan: 5, cls: 'text-muted', title: 'a"b', style: 'color:#fff' }]),
    '<tr><td colspan="5" class="text-muted" title="a&quot;b" style="color:#fff">x</td></tr>');

// Escaping the value but not the attribute would just move the hole.
check('attribute-breaking injection through title is neutralized',
    SafeDom.tr([{ v: 'x', title: '" onmouseover="alert(1)' }]),
    '<tr><td title="&quot; onmouseover=&quot;alert(1)">x</td></tr>');

// --- ths -------------------------------------------------------------------

check('ths escapes header text',
    SafeDom.ths(['A', payload]),
    '<th>A</th><th>&lt;img src=x onerror=alert(1)&gt;</th>');

// --- cell-spec detection ---------------------------------------------------

// An arbitrary object must not be mistaken for a { v: ... } cell descriptor
// and have its keys treated as attributes.
check('a non-spec object is stringified and escaped',
    SafeDom.tr([{ a: 1 }]),
    '<tr><td>[object Object]</td></tr>');

check('a cell spec holding a raw value stays trusted',
    SafeDom.tr([{ v: SafeDom.raw('<b>hi</b>'), cls: 'x' }]),
    '<tr><td class="x"><b>hi</b></td></tr>');

console.log(failures === 0
    ? '\nALL PASS (' + checks + ' checks)'
    : '\n' + failures + ' of ' + checks + ' checks FAILED');

process.exit(failures === 0 ? 0 : 1);
