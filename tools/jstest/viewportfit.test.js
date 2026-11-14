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

check('upright phone', VF.isUprightPhone(390, 844, 'portrait-primary', true), true);
check('sideways phone', VF.isUprightPhone(844, 390, 'landscape-primary', true), false);
check('iOS sizes do not swap: the orientation wins', VF.isUprightPhone(390, 844, 'landscape-secondary', true), false);
check('no orientation API, upright', VF.isUprightPhone(390, 844, undefined, true), true);
check('no orientation API, sideways', VF.isUprightPhone(844, 390, undefined, true), false);
check('an upright tablet is not an upright phone', VF.isUprightPhone(768, 1024, 'portrait-primary', true), false);
// Plan 5 decision 13: a portrait screen under 600 wide without touch (a
// desktop monitor turned upright, or zoomed in) keeps the 16px text.
check('a portrait screen under 600 wide without touch is not an upright phone',
    VF.isUprightPhone(540, 960, 'portrait-primary', false), false);
(function () {
    var classes = {}, handlers = {};
    var win = {
        screen: { width: 390, height: 844, orientation: { type: 'portrait-primary' } },
        matchMedia: function (q) { return { matches: q === '(pointer: coarse)' }; },
        document: { documentElement: { classList: { toggle: function (n, on) { classes[n] = on; } } } },
        addEventListener: function (n, f) { handlers[n] = f; }
    };
    VF.watchUprightPhone(win);
    check('the class is set at once', classes['upright-phone'], true);
    win.screen.orientation.type = 'landscape-primary';
    handlers.orientationchange();
    check('turning the phone clears it', classes['upright-phone'], false);
    win.screen.orientation.type = 'portrait-primary';
    handlers.resize();
    check('a resize re-reads it', classes['upright-phone'], true);
}());

(function () {
    var classes = {};
    var win = {
        orientation: 90,
        screen: { width: 390, height: 844 },
        document: { documentElement: { classList: { toggle: function (n, on) { classes[n] = on; } } } },
        addEventListener: function () {}
    };
    VF.watchUprightPhone(win);
    check('old iOS held sideways: window.orientation wins', classes['upright-phone'], false);
}());

(function () {
    var classes = {};
    var win = {
        screen: { width: 540, height: 960, orientation: { type: 'portrait-primary' } },
        matchMedia: function () { return { matches: false }; },
        document: { documentElement: { classList: { toggle: function (n, on) { classes[n] = on; } } } },
        addEventListener: function () {}
    };
    VF.watchUprightPhone(win);
    check('a desktop with no touch screen never gets the upright phone class', classes['upright-phone'], false);
}());

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
check('every site page loads the module, before it paints',
    header.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1 &&
    header.indexOf('/static/js/viewportfit.js') < header.indexOf('/static/css/gomud.css'), true);
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

check('no visual viewport: no keyboard', VF.keyboardOpen(740, undefined, true), false);
check('an address bar hiding is not a keyboard', VF.keyboardOpen(740, 680, true), false);
check('a keyboard is a keyboard', VF.keyboardOpen(740, 440, true), true);
// Plan 6 check: a keyboard needs a focused text field.
check('200px short with no field focused: not a keyboard', VF.keyboardOpen(740, 540, false), false);
check('200px short, third argument missing: not a keyboard', VF.keyboardOpen(740, 540), false);
check('200px short with a field focused: a keyboard', VF.keyboardOpen(740, 540, true), true);
check('50px short with a field focused: not a keyboard', VF.keyboardOpen(740, 690, true), false);
function docWith(el) { return { activeElement: el }; }
check('editable: input text', VF.editableFocused(docWith({ tagName: 'INPUT', type: 'text' })), true);
check('editable: input with no type', VF.editableFocused(docWith({ tagName: 'INPUT' })), true);
check('editable: input checkbox', VF.editableFocused(docWith({ tagName: 'INPUT', type: 'checkbox' })), false);
check('editable: input button', VF.editableFocused(docWith({ tagName: 'INPUT', type: 'button' })), false);
check('editable: textarea', VF.editableFocused(docWith({ tagName: 'TEXTAREA' })), true);
check('editable: contenteditable', VF.editableFocused(docWith({ tagName: 'DIV', isContentEditable: true })), true);
check('editable: a button', VF.editableFocused(docWith({ tagName: 'BUTTON' })), false);
check('editable: null activeElement', VF.editableFocused(docWith(null)), false);
check('editable: no document', VF.editableFocused(null), false);
check('the page and the outer page both pass editableFocused to keyboardOpen',
    /keyboardOpen\(window\.innerHeight, visible,\s*window\.ViewportFit\.editableFocused\(document\)\)/.test(fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8'))
    && /keyboardOpen\(window\.innerHeight, visible,\s*ViewportFit\.editableFocused\(frame\.contentDocument\)\)/.test(fs.readFileSync(path.join(PUBLIC, 'webclient.html'), 'utf8')), true);
check('the page on its own re-runs on focusin and focusout',
    /document\.addEventListener\('focusin', fitOwnPage\);\s*document\.addEventListener\('focusout', fitOwnPage\);/.test(fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8')), true);
// webclient.html behaviourally: its script runs at parse time, before the
// frame exists. Run it in a fake page and fire DOMContentLoaded and load.
(function () {
    var vm = require('vm');
    var html = fs.readFileSync(path.join(PUBLIC, 'webclient.html'), 'utf8');
    var m = /<script type="text\/javascript">([\s\S]*?)<\/script>/.exec(html);
    check('webclient.html has one inline script with fitFrameHeight', !!m && /function fitFrameHeight/.test(m[1]), true);
    if (!m) return;
    var winL = {}, docL = {}, frameL = {}, frameDocL = {};
    function recorder(store) {
        return function (name, fn) { (store[name] = store[name] || []).push(fn); };
    }
    var frameDoc = { addEventListener: recorder(frameDocL), documentElement: { classList: { toggle: function () {} }, style: { setProperty: function () {} } } };
    var frame = { addEventListener: recorder(frameL), contentDocument: frameDoc, style: {} };
    var domReady = false;
    var el = { style: {}, clientWidth: 300, clientHeight: 500, getBoundingClientRect: function () { return { top: 0 }; } };
    var fakeDoc = {
        addEventListener: recorder(docL),
        getElementById: function (id) { return id === 'webclient-frame' ? (domReady ? frame : null) : el; },
        getElementsByTagName: function () { return []; }
    };
    var fakeWin = { addEventListener: recorder(winL), screen: { width: 390, height: 844 }, innerHeight: 800, scrollTo: function () {} };
    // As in a browser, the window is the global object: window.X is a global X.
    var sandbox = fakeWin;
    fakeWin.document = fakeDoc;
    fakeWin.window = fakeWin;
    fakeWin.ViewportFit = VF;
    var threw = null;
    try { vm.runInNewContext(m[1], sandbox); } catch (e) { threw = e; }
    check('webclient.html script runs at parse time without throwing', threw === null, true);
    check('beforeunload is registered at parse time', (winL.beforeunload || []).length, 1);
    check('no frame yet: no focus listeners', Object.keys(frameDocL).length, 0);
    domReady = true;
    try { (docL.DOMContentLoaded || []).forEach(function (f) { f(); }); } catch (e) { threw = e; }
    if (threw) console.log('threw: ' + threw.stack.split('\n').slice(0, 3).join(' | '));
    check('DOMContentLoaded handlers do not throw', threw === null, true);
    (frameL.load || []).forEach(function (f) { f(); });
    check('the frame document watches focusin', (frameDocL.focusin || []).length >= 1, true);
    check('the frame document watches focusout', (frameDocL.focusout || []).length >= 1, true);
})();
check('Reset layout is hidden on a phone before a mode is applied',
    /html\.phone #dashboard:not\(\[data-mode\]\) #btn-reset-layout \{ display: none; \}/.test(fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8')), true);
check('the command input can shrink beside the X',
    /#command-input \{\s*flex: 1;[^}]*min-width: 0;/.test(fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8')), true);

var dashCss = fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8');
check('the phone tab bar steps aside for the keyboard',
    /html\.keyboard-open #dash-tabbar\s*\{\s*display:\s*none !important;/.test(dashCss), true);
check('the game gives its room back when the tab bar goes',
    /html\.keyboard-open #dashboard\[data-mode="phone"\]\s*\{\s*padding-bottom:\s*6px;/.test(dashCss), true);
check('the outer page marks the game when the keyboard is open',
    /classList\.toggle\('keyboard-open'/.test(outer), true);
check('the bare page marks itself when the keyboard is open',
    /classList\.toggle\('keyboard-open'/.test(game), true);
// PR #768 replaced the round sound button with the Settings menu, so the full
// screen button matches the session bar's other brass buttons instead.
check('the full screen button matches the session bar buttons',
    game.indexOf('class="brass" id="btn-fullscreen"') !== -1 && !/#menu-icon,\s*#btn-fullscreen \{/.test(game), true);

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

// Full screen means the browser's bars are gone too, where the browser can
// do that (Calabe 2026-11-07).
check('header hidden but address bar showing reads as not full screen', VF.fullScreenShown(true, true, false), false);
check('header hidden and browser full screen reads as full screen', VF.fullScreenShown(true, true, true), true);
check('browser full screen with the header showing is not our full screen', VF.fullScreenShown(false, true, true), false);
check('a browser that cannot go full screen: hiding the header is full screen', VF.fullScreenShown(true, false, false), true);
check('a browser that cannot: header showing is not', VF.fullScreenShown(false, false, false), false);

check('Enter Game on a phone goes full screen', VF.enterFullScreenOnConnect(true, null, true, false), true);
check('a player who left full screen keeps that', VF.enterFullScreenOnConnect(true, '0', true, false), false);
check('not on a laptop or tablet', VF.enterFullScreenOnConnect(false, null, true, false), false);
check('not where the browser cannot', VF.enterFullScreenOnConnect(true, null, false, false), false);
check('not when already full screen', VF.enterFullScreenOnConnect(true, '1', true, true), false);

check('the game frame may go full screen',
    /<iframe id="webclient-frame"[^>]*allow="fullscreen"[^>]*allowfullscreen/.test(outer), true);
check('the game frame asks for full screen itself, inside the tap',
    /function requestGameFullScreen\(\) \{\s*var root = document\.documentElement;\s*var req = root\.requestFullscreen \|\| root\.webkitRequestFullscreen;/.test(game), true);
check('a refused full screen is reported in the game text',
    /Your browser did not allow full screen/.test(game), true);
check('the Full screen button asks for it',
    /if \(next\) requestGameFullScreen\(\);\s*outer\.setFullScreen\(next\);/.test(game), true);
check('the outer page follows the back gesture leaving full screen',
    /'fullscreenchange', 'webkitfullscreenchange'/.test(outer), true);
check('the button reads the real state', /isFullScreen: shown/.test(outer), true);
var sessionAt = game.indexOf("sessionButton.addEventListener('click'");
var sessionHandler = game.slice(sessionAt, game.indexOf('connectButton.click();', sessionAt));
check('every Enter Game tap on a phone can go full screen, before the connected check returns',
    /wantsFullScreenOnConnect\(\)\) \{\s*requestGameFullScreen\(\);/.test(sessionHandler) &&
    sessionHandler.indexOf('wantsFullScreenOnConnect') < sessionHandler.indexOf('if (open) {'), true);
function mediaBlock(head) {
    var at = dashCss.indexOf(head + ' {');
    return at === -1 ? '' : dashCss.slice(at, dashCss.indexOf('\n}', at));
}
var phones = mediaBlock('@media (pointer: coarse) and (max-width: 599px), (pointer: coarse) and (max-height: 500px)');
var sideways = mediaBlock('@media (pointer: coarse) and (orientation: landscape) and (max-height: 500px)');
check('a phone either way up gets 30px sub-tabs', /\.subtab \{ min-height: 30px;/.test(phones), true);
check('a phone either way up gets 30px title buttons', /\.dash-panel-head \.ph-btn \{ width: 32px; height: 30px;/.test(phones), true);
check('a phone held sideways hides the tab name pill', /\.gtab-pill \{ display: none; \}/.test(sideways), true);
check('a phone held sideways out of full screen says Full screen gives Chat room',
    /html:not\(\.fullscreen-on\) \.chat-fs-hint \{ display: block; \}/.test(sideways) &&
    game.indexOf('<p class="chat-fs-hint">Turn on Full screen for more room to read Chat.</p>') !== -1, true);
check('the frame knows when full screen is on',
    /classList\.toggle\('fullscreen-on', on\)/.test(game), true);

var manifest = JSON.parse(fs.readFileSync(path.join(PUBLIC, 'static', 'manifest.json'), 'utf8'));
check('home screen icon opens without the address bar', manifest.display, 'standalone');
check('home screen icon opens the game', manifest.start_url, '/webclient');

// 2026-11-07 hotfix: the terminal came out three rows taller than the Game
// panel (exits and prompt hidden). It refits once the font's real size is in,
// and when the panel body (what termfit measures) changes size.
// webFontIn (plan 5) lets the layout take the real game width, then refits.
check('webFontIn refits the terminal',
    /function webFontIn\(\) \{\s*window\.dogmudFontReady = true;\s*resizeTerminal\(true\);/.test(game), true);
check('the terminal refits after the font swap, on the next frame',
    /term\.setOption\('fontFamily', termFont\);(\s*\/\/[^\n]*)*\s*webFontIn\(\);[\s\S]{0,600}requestAnimationFrame\(function \(\) \{ resizeTerminal\(true\); \}\);/.test(game), true);
check('the terminal refits once every font has loaded',
    /document\.fonts\.ready\.then\(function \(\) \{ webFontIn\(\); \}\);/.test(game), true);
check('the Game panel body is observed, not only the panel',
    /_feedBody = _feedPanel\.querySelector\('\.dash-panel-body'\);\s*if \(_feedBody\) _feedObserver\.observe\(_feedBody\);/.test(game), true);
check('a tab that wraps alone keeps its own size',
    /\.subtabs::after \{\s*content: "";\s*flex: 999 1 auto;/.test(dashCss), true);

// Plan 6, ruling 15: a phone turned sideways, judged from the screen so the
// keyboard cannot trigger it.
check('sideways phone', VF.isSidewaysPhone(844, 390, 'landscape-primary', true), true);
check('upright phone is not sideways', VF.isSidewaysPhone(390, 844, 'portrait-primary', true), false);
check('iOS sizes do not swap: the orientation wins', VF.isSidewaysPhone(390, 844, 'landscape-secondary', true), true);
check('no orientation API, sideways', VF.isSidewaysPhone(844, 390, undefined, true), true);
check('a sideways tablet is not a sideways phone', VF.isSidewaysPhone(1024, 768, 'landscape-primary', true), false);
check('a desktop is not a sideways phone', VF.isSidewaysPhone(1920, 1080, 'landscape-primary', false), false);

check('orientation from the API',
    VF.orientationType({ screen: { orientation: { type: 'landscape-primary' } } }), 'landscape-primary');
check('iOS before 16.4: window.orientation 90 is landscape',
    VF.orientationType({ screen: {}, orientation: 90 }), 'landscape-primary');
check('iOS before 16.4: window.orientation 0 is portrait',
    VF.orientationType({ screen: {}, orientation: 0 }), 'portrait-primary');
check('no signal at all', VF.orientationType({ screen: {} }), undefined);

// Once per browser tab: sessionStorage survives a reload but not a new tab.
(function () {
    var store = {};
    var ss = { getItem: function (k) { return k in store ? store[k] : null; }, setItem: function (k, v) { store[k] = String(v); } };
    var memo = {};
    check('first time in a tab: show it', VF.takeSidewaysToast(ss, memo), true);
    check('again on this page: not again', VF.takeSidewaysToast(ss, memo), false);
    check('after a reload in the same tab: not again', VF.takeSidewaysToast(ss, {}), false);
    check('a new tab: show it', VF.takeSidewaysToast({ getItem: function () { return null; }, setItem: function () {} }, {}), true);
    var blocked = { getItem: function () { throw new Error('blocked'); }, setItem: function () { throw new Error('blocked'); } };
    var m2 = {};
    check('storage blocked: show it once', VF.takeSidewaysToast(blocked, m2), true);
    check('storage blocked: not twice on one page', VF.takeSidewaysToast(blocked, m2), false);
    check('no storage at all: show it once', VF.takeSidewaysToast(null, {}), true);
})();
check('the toast words', VF.SIDEWAYS_TOAST, 'This game is easier to play with your phone held upright.');

// The dashboard wires it, and its toast can be lifted and held longer.
var dashSrc = fs.readFileSync(path.join(PUBLIC, 'static', 'js', 'dashboard.js'), 'utf8');
check('the dashboard watches for a sideways phone',
    /this\._watchSideways && this\._watchSideways\(\);/.test(dashSrc)
    && /VF\.takeSidewaysToast\(store, self\._sidewaysMemo\)/.test(dashSrc), true);
check('the sideways toast is lifted above the command box and shown 4s',
    /self\._toast\(VF\.SIDEWAYS_TOAST, \{ ms: 4000, lifted: true \}\)/.test(dashSrc), true);
check('a lifted toast sits above the command box and tab bar',
    /\.dash-toast\.lifted \{ bottom: calc\(var\(--cmdbox-h, 64px\) \+ var\(--tabbar-h, 64px\) \+ 12px\); \}/.test(dashCss), true);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
