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

var outer = fs.readFileSync(path.join(PUBLIC, 'webclient.html'), 'utf8');
var header = fs.readFileSync(path.join(PUBLIC, '_header.html'), 'utf8');
check('the outer page loads the module',
    outer.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('the full screen code exists before the game frame starts loading',
    outer.indexOf('window.WebclientFrame') !== -1 &&
    outer.indexOf('window.WebclientFrame') < outer.indexOf('<iframe'), true);
check('the frame height follows the visual viewport',
    outer.indexOf('ViewportFit.frameHeight(') !== -1, true);
check('the header link uses the same full screen code',
    header.indexOf('WebclientFrame.setFullScreen(true)') !== -1, true);
check('the header script checks the link exists', /if \(!link\) return;/.test(header), true);

var game = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
check('the game page has a viewport tag',
    game.indexOf('<meta name="viewport" content="width=device-width, initial-scale=1.0"') !== -1, true);
check('the game page loads the module',
    game.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('the phone class is set before the terminal reads its colours',
    game.indexOf("classList.add('phone')") !== -1 &&
    game.indexOf("classList.add('phone')") < game.indexOf('const rootStyle ='), true);
check('the session bar has the full screen toggle', game.indexOf('id="btn-fullscreen"') !== -1, true);
check('the toggle talks to the outer page', game.indexOf('window.parent.WebclientFrame') !== -1, true);

// Zooming in also shrinks the visual viewport, and is not the keyboard.
check('no visual viewport has no visible height', VF.visibleHeight(null), undefined);
check('at normal zoom the visible height counts', VF.visibleHeight({ height: 400, scale: 1 }), 400);
check('zoomed in with no keyboard, the page is still all visible', VF.visibleHeight({ height: 370, scale: 2 }), 740);
check('zoomed in with the keyboard, the keyboard still counts', VF.visibleHeight({ height: 220, scale: 2 }), 440);

check('no visual viewport: no keyboard', VF.keyboardOpen(740, undefined), false);
check('an address bar hiding is not a keyboard', VF.keyboardOpen(740, 680), false);
check('a keyboard is a keyboard', VF.keyboardOpen(740, 440), true);

var dashCss = fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8');
check('the phone tab bar steps aside for the keyboard',
    /html\.keyboard-open #dash-tabbar\s*\{\s*display:\s*none !important;/.test(dashCss), true);
check('the game gives its room back when the tab bar goes',
    /html\.keyboard-open #dashboard\[data-mode="phone"\]\s*\{\s*padding-bottom:\s*6px;/.test(dashCss), true);
check('the outer page marks the game when the keyboard is open',
    /classList\.toggle\('keyboard-open'/.test(outer), true);
check('the bare page marks itself when the keyboard is open',
    /classList\.toggle\('keyboard-open'/.test(game), true);
check('the full screen button is shaped like the sound button',
    /#menu-icon,\s*#btn-fullscreen \{/.test(game) && game.indexOf('class="brass" id="btn-fullscreen"') === -1, true);

check('the keyboard path never resets the frame to nothing',
    /addEventListener\('resize', function \(\) \{\s*holdPageAtTop\(\);\s*fitFrameHeight\(\);/.test(outer), true);
check('a zoomed-in player can pan freely',
    /function holdPageAtTop\(\) \{\s*if \(window\.visualViewport\.scale <= 1\.01\)/.test(outer), true);
check('the bare page lets go of its minimum height', /document\.body\.style\.minHeight = '0'/.test(game), true);
check('phones leave the layout reset out of the session bar',
    /#dashboard\[data-mode="phone"\] #btn-reset-layout\s*\{\s*display:\s*none;/.test(dashCss), true);
check('a very short screen gives the game the session bar and panel title',
    /@media \(max-height: 320px\) \{\s*html\.keyboard-open #dash-session,\s*html\.keyboard-open #panel-feed \.dash-panel-head\s*\{\s*display:\s*none;/.test(dashCss), true);
check('the frame height ignores zoom',
    outer.indexOf('ViewportFit.visibleHeight(window.visualViewport)') !== -1 &&
    outer.indexOf('vv.height') === -1, true);
check('the bare page ignores zoom', /ViewportFit\.visibleHeight\(vv\)/.test(game), true);
check('a page framed by another site cannot break the toggle code',
    /try \{[^}]*window\.parent\.WebclientFrame[^}]*\} catch/.test(game), true);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
