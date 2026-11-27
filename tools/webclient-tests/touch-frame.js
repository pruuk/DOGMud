#!/usr/bin/env node
/*
 * Guards the touch frame's page wiring and stylesheet (touch frame plan 1a,
 * docs/superpowers/specs/completed/2026-11-21-touch-frame-design.md).
 *
 *   node tools/webclient-tests/touch-frame.js
 *
 * Zero dependencies. Static analysis of webclient-pure.html, dashboard.css
 * and dashboard.js; it does not run the UI.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
const html = fs.readFileSync(path.join(root, 'webclient-pure.html'), 'utf8');
const css = fs.readFileSync(path.join(root, 'static', 'css', 'dashboard.css'), 'utf8');
const dash = fs.readFileSync(path.join(root, 'static', 'js', 'dashboard.js'), 'utf8');

let fails = 0;
function check(ok, what) {
  console.log((ok ? '  OK        ' : '  FAIL      ') + what);
  if (!ok) fails++;
}

// The page.
const feedAt = html.indexOf('id="panel-feed"');
const slotAt = html.indexOf('id="touch-slot"');
const deckAt = html.indexOf('id="touch-deck"');
const cmdAt = html.indexOf('id="panel-cmd"');
check(feedAt > 0 && slotAt > feedAt && cmdAt > slotAt, 'the slot sits between the game text and the command box');
check(deckAt > slotAt && deckAt < cmdAt, 'the deck is inside the slot');
check(/<div id="touch-slot" hidden>/.test(html), 'the slot starts hidden');
check(/<div id="touch-deck" class="deck-grid" role="group" aria-label="Movement"><\/div>/.test(html), 'the deck starts empty and is one labelled group of buttons (plan 2b), styled as a deck grid (plan 2c)');
check(/<div id="map-deck" class="deck-grid" role="group" aria-label="Movement" hidden><\/div>/.test(html), 'the Map tab has its own movement buttons, hidden until the Map tab opens on a phone (plan 2c)');
check(html.indexOf('id="map-render"') >= 0 && html.indexOf('id="map-deck"') > html.indexOf('id="map-render"') &&
      html.indexOf('id="map-deck"') < html.indexOf('id="map-controls"'), 'the Map tab buttons sit right after the map');
const scriptAt = html.indexOf('/static/js/touchdeck.js');
const wiringAt = html.indexOf("querySelectorAll('input[name=\"touch-controls\"]')");
check(scriptAt > 0 && wiringAt > scriptAt, 'touchdeck.js loads before the Settings code reads it');
check(/<fieldset id="settings-touch" aria-describedby="touch-note">/.test(html), 'Touch controls is its own Settings section');
['auto', 'always', 'never'].forEach(function (v) {
  check(html.indexOf('<input type="radio" name="touch-controls" value="' + v + '">') > 0, 'Settings offers ' + v);
});

// The stylesheet.
check(/#panel-feed:has\(~ #panel-cmd\)/.test(css), 'the game window joins the command box with the slot between them');
check(!/#panel-feed(\.sight-\w+)?\s*\+\s*#panel-cmd/.test(css), 'no rule still needs the command box right after the game window');
['dark', 'shapes', 'faces', 'dazzled'].forEach(function (b) {
  check(new RegExp('#panel-feed\\.sight-' + b + '\\s*~ #touch-slot \\{ border-color: var\\(--sight-' + b + '\\); \\}').test(css),
    'the slot takes the ' + b + ' sight colour');
});
check(/#touch-slot\[hidden\] \{ display: none; \}/.test(css), 'an empty slot takes no room');
check(/#touch-slot #dash-drawer \{[^}]*position: static;/.test(css), 'an open tab in the slot is part of the stack, not floating');
check(/#touch-slot #dash-drawer:not\(\.open\) \{ display: none; \}/.test(css), 'a closed tab takes no room in the slot');
check(/#touch-slot\[data-shows="tab"\] #touch-deck \{ display: none; \}/.test(css), 'an open tab takes the deck\'s place');
check(/#touch-slot\.deck-on #dash-drawer\.open \{ min-height: var\(--deck-h\); \}/.test(css), 'a short tab keeps the deck\'s height');

// dashboard.js.
check(/_placeDrawer\(true\)/.test(dash) && /_placeDrawer\(false\)/.test(dash), 'a phone moves its tab card into the slot and back out');
check(/applyDeck: function/.test(dash), 'dashboard.js applies the deck');

// Plan 1b: the keyboard rule.
check(/slotShows\(\{[\s\S]{0,300}field:/.test(dash), 'applyDeck passes the field being typed in to slotShows');
check(/TD\.fieldOf\(/.test(dash) && /editableFocused\(document\)/.test(dash), 'the field comes from fieldOf and ViewportFit.editableFocused');
// The page-wide watch: the tab bar's own focusin/focusout listeners (the
// roving tab stop) must not satisfy this check.
check(/document\.addEventListener\("focusin", onFocusMove\)/.test(dash) && /document\.addEventListener\("focusout", onFocusMove\)/.test(dash), 'a focus watch reruns applyDeck when focus moves');
check(/if \(shows === "tab" && was !== "tab"\) \{\s*this\._scrollChatToNewest\(/.test(dash), 'a tab back in the slot puts Comms at its newest message');

// The Automation dialog with a phone keyboard up (found in the plan 1b
// device check, 2026-11-21): it never grows past the page, its body scrolls,
// so Save stays on screen; and its fields take commands, so no capitals.
check(/\.auto-modal \{[^}]*max-height: calc\(100% - 16px\)/.test(css), 'the Automation dialog is never taller than the page');
check(/\.auto-modal-body \{[^}]*overflow-y: auto/.test(css) && /\.auto-modal-body \{[^}]*min-height: 0/.test(css), 'the Automation dialog body scrolls, so Cancel and Save stay on screen');
check(/window\.openAutoModal\s*=\s*function[\s\S]{0,900}setAttribute\("autocapitalize", "off"\)/.test(html), 'the Automation dialog turns off automatic capitals on its fields');

// Plan 1c: phone polish.
const outer = fs.readFileSync(path.join(root, 'webclient.html'), 'utf8');
// Phones only: the rule must sit inside the phone media query, because an
// upright tablet is in phone mode too and keeps its title bar.
const phoneBlock = (css.match(/@media \(pointer: coarse\) and \(max-width: 599px\), \(pointer: coarse\) and \(max-height: 500px\) \{[\s\S]*?\n\}/) || [''])[0];
check(/#dashboard\[data-mode="phone"\] #panel-feed > \.dash-panel-head \{ display: none; \}/.test(phoneBlock), 'no Game title bar on a phone, inside the phone media query');
check(!/^#dashboard\[data-mode="phone"\] #panel-feed > \.dash-panel-head/m.test(css), 'the title bar rule is not outside the phone media query (tablets keep it)');
check(/\.subtabs \{[^}]*flex-wrap: nowrap;[^}]*overflow-x: auto;/.test(css), 'a phone keeps sub-tabs in one sideways-scrolling row');
check(/\.subtabs\.fade-left/.test(css) && /\.subtabs\.fade-right/.test(css), 'the row fades at the edge with more to scroll');
check(/TD\.rowFade\(/.test(dash) && /TD\.scrollLeftToShow\(/.test(dash), 'dashboard.js wires the row fades and scrolls the open sub-tab into view');
check(/maybePhoneTip: function/.test(dash) && /VF\.takeOnce\(/.test(dash) && /VF\.phoneTip\(/.test(dash), 'dashboard.js shows a phone tip once per device');
check(/Dashboard\.maybePhoneTip\(\)/.test(html), 'entering the world asks for the phone tip');
check(/ViewportFit\.isDesktopSite\(/.test(outer) && /setAttribute\('content', 'width=' \+ screen\.width/.test(outer), 'the outer page undoes Desktop site where it can');
check(/if \(!ViewportFit\.isPhone\(screen\.width, screen\.height\)\) return;\s*if \(!ViewportFit\.isDesktopSite\(/.test(outer), 'only a phone undoes Desktop site (tablets keep today\'s layout)');
check(/desktopSite: function/.test(outer) && /turnedOff: function/.test(outer) && /canFullScreen: browserCan/.test(outer), 'the outer page tells the game what the tip needs');
check(!/so every\s+Chat channel stays in sight on a phone/.test(css), 'the 2026-11-07 wrap comment is rewritten');

// Plan 2b: moving by touch.
const roomAt = html.indexOf('"Room": function() {');
const movesAt = html.indexOf('Dashboard.setMoves(obj.Info)');
const coordsAt = html.indexOf('var coords = obj.Info.coords.split(",");');
check(roomAt > 0 && movesAt > roomAt && movesAt < coordsAt, 'Room.Info reaches the deck before the map code runs');
check(/Dashboard\.setDeckSender\(sendButtonPress, playerSend\)/.test(html), 'a deck button sends through sendButtonPress (its id as Char.Button.Press; webclient-input-queue.test.js runs it), and the Dark tile types look through playerSend, the typed command path');
check(/function sendButtonPress\(id\) \{[\s\S]{0,400}SendGMCP\("Char\.Button\.Press", \{ id: id \}\)/.test(html), 'sendButtonPress sends only the id');
check(/"Commands": function\(\) \{[^}]*syncDeckMode\(\);/.test(html) && /"Commands\.State": function\(\) \{\s*checkDeathClearsQueue\(\);\s*syncDeckMode\(\);/.test(html), 'Commands.State\'s mode reaches the deck');
check(/socket\.onclose = function\(event\) \{[\s\S]{0,300}if \(window\.Dashboard && Dashboard\.setMoves\) Dashboard\.setMoves\(null\);/.test(html), 'a closed connection clears the buttons');
check(/TD\.cells\(model, s, this\._deckState\.level, this\._deckState\.page\)/.test(dash) && /TD\.render\(deck, cells, document\)/.test(dash), 'applyDeck draws the cells touchdeck.js lays out');
check(/closest\("button\[data-cell\]"\)/.test(dash), 'one click handler serves every deck button');
check(dash.indexOf('deck-slot') === -1, 'dashboard.js draws no empty slots of its own (touchdeck.js render draws every cell)');
check(/\.deck-btn \{[^}]*touch-action: manipulation;/.test(css), 'a quick second tap is a tap, not a zoom');
const tdScriptAt = html.indexOf('/static/js/touchdeck.js');
const dashScriptAt = html.indexOf('/static/js/dashboard.js');
check(tdScriptAt > 0 && dashScriptAt > tdScriptAt, 'touchdeck.js loads before dashboard.js (the tab bar draws its icons with TouchDeck.lineIcon)');

check(html.indexOf('<p class="settings-note" id="touch-note">Buttons above the command box for walking. Automatic shows them on phones and tablets. Always shows them on every screen. Never hides them.</p>') > 0, 'the Touch controls note says what each choice does now (plan 2b)');

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails === 0 ? 0 : 1);
