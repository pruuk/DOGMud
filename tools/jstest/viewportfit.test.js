// viewportfit.test.js: what counts as a phone, when to open full screen, and
// how tall the game frame may be with the on-screen keyboard open.
//
// Run:  node tools/jstest/viewportfit.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var VF = require(path.join(PUBLIC, 'static', 'js', 'viewportfit.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + got + ', want ' + want);
}

check('a phone held upright is a phone', VF.isPhone(390, 844), true);
check('a phone on its side is a phone', VF.isPhone(844, 390), true);
check('a small tablet is not a phone', VF.isPhone(600, 960), false);
check('a laptop is not a phone', VF.isPhone(1920, 1080), false);

check('a phone opens full screen', VF.wantFullScreen(null, true), true);
check('a larger screen does not', VF.wantFullScreen(null, false), false);
check('a phone player who turned it off stays off', VF.wantFullScreen('0', true), false);
check('a desktop player who turned it on stays on', VF.wantFullScreen('1', false), true);

check('no keyboard: the frame fills its space', VF.frameHeight(700, 700, 0), 700);
check('keyboard open: the frame fits above it', VF.frameHeight(700, 400, 0), 400);
check('a header above the frame counts', VF.frameHeight(600, 400, 100), 300);
check('a fractional height rounds down, never under the keyboard', VF.frameHeight(700, 400.6, 0), 400);
check('no visual viewport: the frame fills its space', VF.frameHeight(700, undefined, 0), 700);
check('never negative', VF.frameHeight(700, 50, 100), 0);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
