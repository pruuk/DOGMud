#!/usr/bin/env node
/*
 * Guards the touch frame's page wiring and stylesheet (touch frame plan 1a,
 * docs/superpowers/specs/2026-11-21-touch-frame-design.md).
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
check(/<div id="touch-deck" aria-hidden="true"><\/div>/.test(html), 'the deck starts empty and hidden from screen readers');
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

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails === 0 ? 0 : 1);
