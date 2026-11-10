// roomtitlebold.test.js: text in the room title's colour turns bold in the web
// client's game window, and nothing else does.
//
// Run:  node tools/jstest/roomtitlebold.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var Bold = require(path.join(PUBLIC, 'static', 'js', 'roomtitlebold.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want));
}

// Captured from ansitags.Parse of room-title.template's tags.
var title = '\x1b[38;5;8m\x1b[49m.:\x1b[0m \x1b[38;5;170m\x1b[49mThe Square\x1b[0m \x1b[38;5;167m\x1b[49m[Town]\x1b[0m';
var bold = '\x1b[38;5;8m\x1b[49m.:\x1b[0m \x1b[1;38;5;170m\x1b[49mThe Square\x1b[0m \x1b[38;5;167m\x1b[49m[Town]\x1b[0m';
check('the room title turns bold', Bold.apply(title), bold);
check('the close still resets everything', Bold.apply(title).indexOf('The Square\x1b[0m') !== -1, true);
check('two titles in one message', Bold.apply('\x1b[38;5;170mA\x1b[0m\x1b[38;5;170mB\x1b[0m'), '\x1b[1;38;5;170mA\x1b[0m\x1b[1;38;5;170mB\x1b[0m');
check('other colours are left alone', Bold.apply('\x1b[38;5;17m\x1b[49mx\x1b[0m'), '\x1b[38;5;17m\x1b[49mx\x1b[0m');
check('a background of that colour is left alone', Bold.apply('\x1b[48;5;170mx'), '\x1b[48;5;170mx');
check('plain text is unchanged', Bold.apply('You see a lantern.\r\n'), 'You see a lantern.\r\n');
check('a non-string is unchanged', Bold.apply(null), null);

var game = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
check('the game page loads it',
    game.indexOf('/static/js/roomtitlebold.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('server text goes through it on the way to the terminal',
    /term\.write\(window\.RoomTitleBold \? window\.RoomTitleBold\.apply\(shown\) : shown\);/.test(game), true);

if (failures) { console.log(failures + ' failed'); process.exit(1); }
console.log('all passed');
