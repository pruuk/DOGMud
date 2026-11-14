// webclient-music-default.test.js: music is off for a browser with no saved
// settings, and saved settings are kept (#838).
//
// Run:  node tools/jstest/webclient-music-default.test.js
//
// Runs the page's own defaultSliders and its load-from-localStorage code
// against a fake localStorage. The sound effect sliders stay at 75. A browser
// that already has sliderValues keeps them: a saved 75 cannot be told apart
// from a player who chose 75.
'use strict';

var fs = require('fs');
var path = require('path');
var html = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}

var defaultsSrc = extract('const defaultSliders = {', '};') + '};';
var loadSrc = extract("const savedValues = localStorage.getItem('sliderValues');",
    '// 2) Check if we have a "muteAllSound"');

function load(saved) {
    var store = {};
    if (saved !== undefined) store.sliderValues = JSON.stringify(saved);
    var localStorage = {
        getItem: function (k) { return k in store ? store[k] : null; },
        setItem: function (k, v) { store[k] = String(v); }
    };
    var fn = new Function('localStorage', defaultsSrc
        + '\nlet sliderValues = { ...defaultSliders };\n' + loadSrc
        + '\nreturn sliderValues;');
    return { sliderValues: fn(localStorage), store: store };
}

var fresh = load(undefined);
check('a fresh browser starts with music at 0', fresh.sliderValues.music === 0,
    'music ' + fresh.sliderValues.music);
check('a fresh browser keeps the sound effect sliders at 75',
    ['combat sounds', 'movement sounds', 'environment sounds', 'other sounds']
        .every(function (k) { return fresh.sliderValues[k] === 75; }),
    JSON.stringify(fresh.sliderValues));
check('a fresh browser saves its defaults',
    JSON.parse(fresh.store.sliderValues).music === 0, fresh.store.sliderValues);

var old = load({ "music": 75, "combat sounds": 40 });
check('saved settings keep their music level', old.sliderValues.music === 75,
    'music ' + old.sliderValues.music);
check('saved settings keep their other levels', old.sliderValues['combat sounds'] === 40,
    'combat ' + old.sliderValues['combat sounds']);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
