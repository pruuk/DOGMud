// questtabs.test.js: the Quests panel's three tabs (web client base plan 7,
// layout spec rulings 18 and 19, owner P4).
// Run: node tools/jstest/questtabs.test.js
'use strict';
var path = require('path');
var QT = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'questtabs.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}
var rows = [
    { id: 1, name: 'The Truth', focused: true },
    { id: 2, name: 'Rope for Hadwen', repeatable: true },
    { id: 3, name: 'Night Watch', repeatable: true, status: 'ready' },
    { id: 4, name: 'Mill Wheel Grease', repeatable: true, status: 'waiting' },
    { id: 5, name: 'Rat Patrol', repeatable: true, status: 'ready' },
    { id: 6, name: 'Zebra Hunt', status: 'done' },
    { id: 7, name: 'Apple Picking', repeatable: true, status: 'done' }
];
var s = QT.split(rows);
check('active: rows with no status', s.active.map(function (q) { return q.id; }), [1, 2]);
check('repeat: ready first, then by name', s.repeat.map(function (q) { return q.id; }), [3, 5, 4]);
check('completed: by name', s.done.map(function (q) { return q.id; }), [7, 6]);
check('ready count', s.ready, 2);
check('label with ready', QT.repeatLabel(2), 'Repeat (2)');
check('label with none ready', QT.repeatLabel(0), 'Repeat');
check('empty lines', [QT.EMPTY.active, QT.EMPTY.repeat, QT.EMPTY.done],
    ['No active quests.', 'Nothing to repeat yet.', 'Nothing finished yet.']);
check('an unknown tab falls back to Active', QT.tabOrDefault('bogus'), 'active');
check('a known tab is kept', QT.tabOrDefault('done'), 'done');
check('no data: three empty tabs', QT.split(null), { active: [], repeat: [], done: [], ready: 0 });
check('an unknown status is not shown anywhere', QT.split([{ id: 9, name: 'x', status: 'odd' }]),
    { active: [], repeat: [], done: [], ready: 0 });
check('the input is not reordered', rows.map(function (q) { return q.id; }), [1, 2, 3, 4, 5, 6, 7]);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
