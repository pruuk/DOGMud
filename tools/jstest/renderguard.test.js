// renderguard.test.js — regression tests for skipping no-op DOM rebuilds in the
// web client (review finding 19, hot-path GMCP DOM rebuilds).
//
// Run:  node tools/jstest/renderguard.test.js
//
// Char.Conditions ships alongside Char.Vitals, so the status panel was being
// destroyed and recreated every round whether or not a condition had changed.
// Inventory is the largest subtree in the client and the one that changes least
// often, and it was rebuilt tile by tile on every push.
//
// Dependency-free and assertion-library-free, matching safe-dom.test.js.
// Exits non-zero on failure.

var path = require('path');

var RenderGuard = require(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'renderguard.js'
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

// A stand-in for a panel element. writeCount is what the finding is about: the
// number of times the DOM was actually replaced.
function fakeEl() {
    var el = { writeCount: 0 };
    var value = '';
    Object.defineProperty(el, 'innerHTML', {
        get: function () { return value; },
        set: function (v) { value = v; el.writeCount++; }
    });
    return el;
}

// --- setHTML ---------------------------------------------------------------

var status = fakeEl();

check('first render writes', RenderGuard.setHTML(status, '<b>poisoned</b>'), true);
check('  and the DOM was touched once', status.writeCount, 1);

check('identical markup does not write', RenderGuard.setHTML(status, '<b>poisoned</b>'), false);
check('  and the DOM was not touched again', status.writeCount, 1);

// The whole point: a panel pushed every round with unchanged content costs
// nothing after the first render.
RenderGuard.setHTML(status, '<b>poisoned</b>');
RenderGuard.setHTML(status, '<b>poisoned</b>');
RenderGuard.setHTML(status, '<b>poisoned</b>');
check('twenty unchanged rounds still cost one write', status.writeCount, 1);

check('changed markup writes', RenderGuard.setHTML(status, '<b>bleeding</b>'), true);
check('  and the DOM was touched again', status.writeCount, 2);
check('  and the content is correct', status.innerHTML, '<b>bleeding</b>');

// Reverting to earlier content must still write: the guard compares against the
// LAST value, not a set of everything ever seen.
check('reverting to earlier markup writes', RenderGuard.setHTML(status, '<b>poisoned</b>'), true);
check('  and the content is correct', status.innerHTML, '<b>poisoned</b>');

// Empty output is a real state ("No active effects."), not a reason to skip.
var empty = fakeEl();
check('writing empty markup counts as a write', RenderGuard.setHTML(empty, ''), true);
check('  repeated empty does not write', RenderGuard.setHTML(empty, ''), false);

// --- signatures are per element --------------------------------------------

var a = fakeEl();
var b = fakeEl();
RenderGuard.setHTML(a, '<i>same</i>');
check('a second element with the same markup still writes', RenderGuard.setHTML(b, '<i>same</i>'), true);
check('  each element tracks its own state', b.writeCount, 1);

// A panel torn down and recreated (window reopened, layout reset) arrives as a
// fresh object with no signature, so it renders without anything to invalidate
// by hand. This is why signatures hang off the element rather than a name.
var recreated = fakeEl();
check('a recreated panel renders again', RenderGuard.setHTML(recreated, '<i>same</i>'), true);

// --- changed ---------------------------------------------------------------

var grid = fakeEl();

check('first signature is a change', RenderGuard.changed(grid, 'worn|[]'), true);
check('same signature is not a change', RenderGuard.changed(grid, 'worn|[]'), false);

// The inventory bug this guards against: the tab is an input, so switching tabs
// with an unchanged payload MUST re-render.
check('switching tab is a change', RenderGuard.changed(grid, 'backpack|[]'), true);
check('switching back is a change', RenderGuard.changed(grid, 'worn|[]'), true);

// Payload change on the same tab.
check('a new item is a change', RenderGuard.changed(grid, 'worn|[{"id":1}]'), true);
check('the same item again is not', RenderGuard.changed(grid, 'worn|[{"id":1}]'), false);

// --- forget ----------------------------------------------------------------

RenderGuard.forget(grid);
check('forget forces the next render', RenderGuard.changed(grid, 'worn|[{"id":1}]'), true);

// --- defensive -------------------------------------------------------------

check('a missing element always renders (changed)', RenderGuard.changed(null, 'x'), true);
check('a missing element does not write (setHTML)', RenderGuard.setHTML(null, 'x'), false);
RenderGuard.forget(null); // must not throw
check('forget on null is safe', true, true);

// --- result ----------------------------------------------------------------

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) {
    console.log(failures + ' FAILED');
    process.exit(1);
}
