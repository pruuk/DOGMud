// webclient-sound-off.test.js: the web client makes no sound until the player
// turns sound on, on every device, for everyone (owner, 2026-11-15).
//
// Run:  node tools/jstest/webclient-sound-off.test.js
//
// Starting an audio element at all, even at volume 0, can take a phone's
// audio away from the player's own music, so "off" means no element is ever
// started. Part 1 runs static/js/mp3.js against a fake Audio. Part 2 runs the
// page's own sound-settings load against a fake localStorage.
'use strict';

var fs = require('fs');
var path = require('path');
var root = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var mp3Src = fs.readFileSync(path.join(root, 'static', 'js', 'mp3.js'), 'utf8');
var html = fs.readFileSync(path.join(root, 'webclient-pure.html'), 'utf8');

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

// ---- Part 1: MP3Player ----------------------------------------------------

var started = [];
function FakeAudio() {
    this.src = ''; this.loop = false; this.volume = 1; this.paused = true; this.currentTime = 0;
}
FakeAudio.prototype.play = function () {
    this.paused = false; started.push(this.src);
    return { catch: function () {} };
};
FakeAudio.prototype.pause = function () { this.paused = true; };

var MP3Player = new Function('Audio', mp3Src + '\nreturn MP3Player;')(FakeAudio);
function playing(p) {
    var n = []; p.audioCache.forEach(function (a, u) { if (!a.paused) n.push(u); }); return n;
}

var music = new MP3Player(false);
started = [];
music.play('intro.mp3', true, 0.75);
check('a new player starts nothing: sound is off until turned on',
    started.length === 0, 'started ' + JSON.stringify(started));

music.setEnabled(true, 0.75);
check('turning sound on starts the track the server asked for',
    playing(music).join() === 'intro.mp3', 'playing ' + JSON.stringify(playing(music)));

music.setEnabled(false);
check('turning sound off stops it', playing(music).length === 0,
    'playing ' + JSON.stringify(playing(music)));

music.play('zone.mp3', true, 0.75);
check('while off, a new track does not start', playing(music).length === 0,
    'playing ' + JSON.stringify(playing(music)));
music.setEnabled(true, 0.75);
check('turning sound back on starts the latest track',
    playing(music).join() === 'zone.mp3', 'playing ' + JSON.stringify(playing(music)));

music.setGlobalVolume(0);
check('Music at 0 stops the track instead of playing it silently',
    playing(music).length === 0, 'playing ' + JSON.stringify(playing(music)));
started = [];
music.play('zone2.mp3', true, 0);
check('with Music at 0, a new track does not start', started.length === 0,
    'started ' + JSON.stringify(started));
music.setGlobalVolume(0.5);
check('raising Music starts the latest track',
    playing(music).join() === 'zone2.mp3' && music.audioCache.get('zone2.mp3').volume === 0.5,
    'playing ' + JSON.stringify(playing(music)));

music.stopAll();
started = [];
music.setGlobalVolume(0.6);
music.setEnabled(true, 0.6);
check('after the server stops music, nothing comes back on its own',
    started.length === 0, 'started ' + JSON.stringify(started));

var effects = new MP3Player(true);
started = [];
effects.play('step.mp3', false, 0.75);
check('a sound effect does not play while sound is off', started.length === 0,
    'started ' + JSON.stringify(started));
effects.setEnabled(true, 1);
check('turning sound on does not replay old sound effects', started.length === 0,
    'started ' + JSON.stringify(started));
effects.play('step.mp3', false, 0.75);
check('with sound on, a sound effect plays', started.join() === 'step.mp3',
    'started ' + JSON.stringify(started));
started = [];
effects.play('step.mp3', false, 0);
check('a sound effect at 0 is not started', started.length === 0,
    'started ' + JSON.stringify(started));

// ---- Part 2: the page's sound settings -------------------------------------

function extract(a, b) {
    var s = html.indexOf(a); if (s < 0) throw new Error('marker not found: ' + a);
    var e = html.indexOf(b, s + 1); if (e < 0) throw new Error('end marker not found: ' + b);
    return html.slice(s, e);
}
var defaultsSrc = extract('const defaultSliders = {', '};') + '};';
var loadSrc = extract('// Sound settings: load', '// Sound settings: end load');

function load(saved) {
    var store = {};
    Object.keys(saved || {}).forEach(function (k) { store[k] = JSON.stringify(saved[k]); });
    var localStorage = {
        getItem: function (k) { return k in store ? store[k] : null; },
        setItem: function (k, v) { store[k] = String(v); },
        removeItem: function (k) { delete store[k]; }
    };
    var fn = new Function('localStorage', defaultsSrc
        + '\nlet sliderValues = { ...defaultSliders };\nlet soundOn = false;\n' + loadSrc
        + '\nreturn { sliderValues: sliderValues, soundOn: soundOn };');
    var r = fn(localStorage);
    r.store = store;
    return r;
}

var fresh = load({});
check('a fresh browser has sound off', fresh.soundOn === false, 'soundOn ' + fresh.soundOn);
check('a fresh browser has every slider at 75, ready for when sound is turned on',
    Object.keys(fresh.sliderValues).every(function (k) { return fresh.sliderValues[k] === 75; }),
    JSON.stringify(fresh.sliderValues));

var before = load({ sliderValues: { music: 75, 'combat sounds': 40 }, muteAllSound: false });
check('a browser that used the old settings starts with sound off',
    before.soundOn === false, 'soundOn ' + before.soundOn);
check('it keeps its slider levels', before.sliderValues.music === 75
    && before.sliderValues['combat sounds'] === 40, JSON.stringify(before.sliderValues));
check('the old mute keys are cleared', !('muteAllSound' in before.store),
    JSON.stringify(Object.keys(before.store)));

var wasMuted = load({
    sliderValues: { music: 0, 'combat sounds': 0 },
    unmutedSliderValues: { music: 60, 'combat sounds': 30 },
    muteAllSound: true
});
check('an old "Mute all" browser gets its levels back from before it muted',
    wasMuted.sliderValues.music === 60 && wasMuted.sliderValues['combat sounds'] === 30,
    JSON.stringify(wasMuted.sliderValues));
check('and starts with sound off', wasMuted.soundOn === false, 'soundOn ' + wasMuted.soundOn);
check('and its old mute keys are cleared',
    !('muteAllSound' in wasMuted.store) && !('unmutedSliderValues' in wasMuted.store),
    JSON.stringify(Object.keys(wasMuted.store)));

var chose = load({ soundOn: true, sliderValues: { music: 20 } });
check('a player who turned sound on keeps it on', chose.soundOn === true,
    'soundOn ' + chose.soundOn);
check('and keeps their levels', chose.sliderValues.music === 20,
    JSON.stringify(chose.sliderValues));

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
