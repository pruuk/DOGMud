#!/usr/bin/env node
/*
 * EXECUTES the real client dispatch to prove the Quests panel refreshes on a
 * full "Char" push — not merely that the delegation line exists.
 *
 *   node tools/webclient-tests/quest-panel-refresh.js
 *
 * Zero dependencies. Parses the real GMCPUpdateHandlers map, handleGMCP and
 * renderQuests out of webclient-pure.html, stubs just enough DOM, then feeds
 * the two payload shapes the server actually sends:
 *
 *   "Char.Quests"  — sent ONLY when a quest is granted or advances
 *   "Char"         — sent on login / refresh / reconnect, and contains the
 *                    same quest data (GetCharNode: all = gmcpModule == "Char")
 *
 * Before the 2026-07-30 fix the second case rendered nothing, because
 * handleGMCP stops at the first matching handler and the generic "Char" one
 * had no Quests branch. Players saw "No active quests" for everything they
 * already had.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
const html = fs.readFileSync(HTML, 'utf8');

// ---------------------------------------------------------------- extraction
function sliceBalanced(src, startIdx, open, close) {
  const s = src.indexOf(open, startIdx);
  let depth = 0;
  for (let i = s; i < src.length; i++) {
    if (src[i] === open) depth++;
    else if (src[i] === close) {
      depth--;
      if (depth === 0) return src.slice(s, i + 1);
    }
  }
  throw new Error('unbalanced ' + open + close);
}
function funcSrc(src, signature) {
  const i = src.indexOf(signature);
  if (i < 0) throw new Error('not found: ' + signature);
  return src.slice(i, i + sliceBalanced(src, i, '{', '}').length + (src.indexOf('{', i) - i));
}

const handlersMap = sliceBalanced(html, html.indexOf('let GMCPUpdateHandlers ='), '{', '}');
const renderQuestsSrc = funcSrc(html, 'function renderQuests(');
const handleGMCPSrc = funcSrc(html, 'function handleGMCP(');
const escapeHTMLSrc = funcSrc(html, 'function escapeHTML(');
const writeQuestsHTMLSrc = funcSrc(html, 'function writeQuestsHTML(');

// ---------------------------------------------------------------- DOM stub
function El(tag) {
  this.tag = tag; this.className = ''; this.dataset = {}; this.children = [];
  this._html = ''; this._text = ''; this.style = {};
  Object.defineProperty(this, 'innerHTML', {
    get() { return this._html; },
    set(v) { this._html = v; this.children = []; }
  });
  Object.defineProperty(this, 'textContent', {
    get() { return this._text; }, set(v) { this._text = v; }
  });
}
El.prototype.appendChild = function (c) { c.parent = this; this.children.push(c); this._html += (c._html || c._text || ''); return c; };
El.prototype.insertBefore = function (n) { return this.appendChild(n); };
El.prototype.remove = function () {};
El.prototype.querySelector = function () { return null; };
El.prototype.querySelectorAll = function () { return []; };
El.prototype.addEventListener = function () {};
El.prototype.setAttribute = function () {};

const els = {};
function el(id) { if (!els[id]) els[id] = new El('div'); return els[id]; }

global.document = {
  createElement: (t) => new El(t),
  getElementById: (id) => el(id),
  querySelector: () => null,
  querySelectorAll: () => [],
  addEventListener: () => {},
};
global.window = { addEventListener: () => {} };
global.GMCPStructs = {};
global.console_ = console;

// Globals the handler map touches that we do not care about here.
const noop = () => {};
for (const name of [
  'updateStatusPanel', 'renderInventory', 'renderCommsTabs', 'updateVitalsWindow',
  'renderAutomation', 'rebuildTickTimers', 'renderRoom', 'setConnected',
  'renderMutationToast', 'onActionResult', 'onActionInterrupted', 'renderCommands',
  'SendGMCP', 'appendGameText', 'updateMapPanelTitle', 'renderParty',
  'drainQueue', 'updateActionBar', 'setCooldowns', 'refreshCommandList',
  'charDisplayName', 'handleActionResult',
]) {
  if (!(name in global)) global[name] = noop;
}
// The map renderer: record marker calls instead of drawing. `rooms` mirrors
// RoomGridSVG's Map of the rooms currently ON the map — Zone.Map only carries
// the zone you are standing in, so a cross-zone target is legitimately absent.
const markerCalls = [];
const mapRooms = new Map();
// The page's companion key relay glue; absent here, as when no relay is offered.
global.companionGlue = null;
global.gr = { setQuestMarker: (m) => markerCalls.push(m), rooms: mapRooms };

// eslint-disable-next-line no-eval
(0, eval)([
  escapeHTMLSrc,
  renderQuestsSrc,
  writeQuestsHTMLSrc,
  handlersMap.replace(/^\{/, 'globalThis.GMCPUpdateHandlers = {') + ';',
  handleGMCPSrc,
].join('\n'));

// ---------------------------------------------------------------- helpers
const QUESTS = [
  { id: 2, name: 'The Warren Compact', completion: 50, focused: true,
    hint: 'Bring a healing salve to the shaman.', target_room: 301, next_room: 300, next_dir: 'north' },
  { id: 6, name: "The Collector's Burden", completion: 33, focused: false, hint: 'Deliver to Clerk Pell.' },
];
function panelText() { return el('quests-list').innerHTML || ''; }

// The panel ships with this markup in webclient-pure.html (the #quests-list
// div). Seeding it matters: if the stub started empty, "does not contain
// 'No active quests'" would pass VACUOUSLY on a panel nothing ever touched —
// which is exactly the bug being tested for.
const PANEL_INITIAL = '<div class="quests-empty">No active quests.</div>';
function reset() {
  const e = new El('div');
  e.innerHTML = PANEL_INITIAL;
  els['quests-list'] = e;
  markerCalls.length = 0;
  global.GMCPStructs = {};
}

let fails = 0;
function check(name, cond, detail) {
  if (cond) console.log('  PASS  ' + name);
  else { fails++; console.log('  FAIL  ' + name + '\n          ' + detail); }
}

// ---------------------------------------------------------------- scenarios
console.log('SCENARIO 1 — "Char.Quests" push (a quest was granted or advanced)');
reset();
handleGMCP('Char.Quests', QUESTS);
console.log('   panel: ' + JSON.stringify(panelText().slice(0, 70)));
check('panel is not the empty state', !panelText().includes('No active quests'), panelText().slice(0, 120));
check('a quest name rendered', panelText().includes('Warren Compact'), panelText().slice(0, 120));
check('map marker set from the focused quest', markerCalls.length > 0 && markerCalls[markerCalls.length - 1] &&
  markerCalls[markerCalls.length - 1].targetRoom === 301, JSON.stringify(markerCalls));

console.log('SCENARIO 2 — full "Char" push (login / refresh / reconnect) — THE BUG');
reset();
handleGMCP('Char', {
  Info: { Name: 'Meirok' },
  Vitals: { hp: 100, hp_max: 100 },
  Quests: QUESTS,
});
console.log('   panel: ' + JSON.stringify(panelText().slice(0, 70)));
check('panel refreshed from the full Char payload', !panelText().includes('No active quests'),
  'panel still shows the empty state: ' + JSON.stringify(panelText().slice(0, 120)));
check('a quest name rendered', panelText().includes('Warren Compact'), panelText().slice(0, 120));
check('map marker set on login too', markerCalls.length > 0 && markerCalls[markerCalls.length - 1] &&
  markerCalls[markerCalls.length - 1].targetRoom === 301, JSON.stringify(markerCalls));

console.log('SCENARIO 3 — full "Char" push with NO quests clears the panel and marker');
reset();
handleGMCP('Char', { Info: { Name: 'Meirok' }, Quests: [] });
check('empty state shown', panelText().includes('No active quests'), JSON.stringify(panelText().slice(0, 120)));

console.log('SCENARIO 4 — SAME-zone target: the destination really is on the map');
reset();
mapRooms.clear();
mapRooms.set(301, { room: { RoomId: 301 } }); // target is in this zone's snapshot
handleGMCP('Char.Quests', QUESTS);
console.log('   label: ' + JSON.stringify((panelText().match(/qonmap">([^<]*)/) || [])[1] || '(none)'));
check('claims "marked on map"', panelText().includes('marked on map'), panelText().slice(0, 200));

console.log('SCENARIO 5 — CROSS-zone target: room 301 is NOT on the Watchers Crossing map');
reset();
mapRooms.clear();
mapRooms.set(420, { room: { RoomId: 420 } }); // only local rooms are on the map
handleGMCP('Char.Quests', QUESTS);
console.log('   label: ' + JSON.stringify((panelText().match(/qonmap">([^<]*)/) || [])[1] || '(none)'));
check('does NOT claim a marker that cannot be drawn',
  !panelText().includes('marked on map'),
  'panel promises a dot for an off-map room: ' + panelText().slice(0, 220));
check('shows the next-step direction instead', panelText().includes('head north'),
  panelText().slice(0, 220));

console.log('SCENARIO 6: repeatables are tagged and finished ones are listed in words');
reset();
handleGMCP('Char.Quests', [
  { id: 53, name: 'Cull the Wash', completion: 33, focused: true, repeatable: true, hint: 'Type attack scout.' },
  { id: 7, name: 'The Last Delivery', completion: 50, focused: false },
  { id: 54, name: 'Back to the Forge', repeatable: true, status: 'waiting' },
  { id: 58, name: 'Reading the Stones', repeatable: true, status: 'ready' },
]);
var p6 = panelText();
check('active repeatable shows "(repeatable)"', /Cull the Wash<\/span><span class="qrepeat">\(repeatable\)/.test(p6), p6.slice(0, 300));
check('the one-off quest has no tag', !/Last Delivery<\/span><span class="qrepeat">/.test(p6), p6.slice(0, 300));
check('a "Repeatable work" group follows the active quests', p6.indexOf('Repeatable work') > p6.indexOf('Last Delivery'), p6);
check('waiting status in words', p6.includes('Back to the Forge') && p6.includes('Not offered again yet.'), p6);
check('ready status in words', p6.includes('Reading the Stones') && p6.includes('Ready to take on again.'), p6);
check('finished rows are not focusable rows', !/class="qrow[^"]*" data-quest-id="54"/.test(p6), p6);

console.log('SCENARIO 7: only finished repeatables, nothing active');
reset();
handleGMCP('Char.Quests', [{ id: 54, name: 'Back to the Forge', repeatable: true, status: 'ready' }]);
var p7 = panelText();
check('empty state still says no active quests', p7.includes('No active quests'), p7);
check('and the repeatable work is listed', p7.includes('Back to the Forge') && p7.includes('Ready to take on again.'), p7);

console.log('SCENARIO 8: a long hint is clamped with a "more" toggle; the direction stays outside the clamp (#375 P8)');
// The cap is 200 characters after whitespace collapses (questloop.HintMaxRunes).
const LONG_HINT = 'Walk the old road east past the ruined mill. '.repeat(5) + '  \n  Then type talk pell.'; // over 200
const AT_CAP = 'x'.repeat(200);
function longQuests(hint) {
  return [{ id: 76, name: 'The Long Road', completion: 10, focused: true, hint: hint, next_dir: 'east' }];
}
// The stub's querySelectorAll returns nothing, so to drive the toggle's
// click handler this scenario hands back one fake button for ".qhint-more"
// and records the listener the page binds to it.
function withToggleCapture() {
  const host = el('quests-list');
  const bound = [];
  host.querySelectorAll = (sel) => {
    if (sel !== '.qhint-more') return [];
    const m = /class="qhint-more"[^>]*data-quest-id="(\d+)"/.exec(host.innerHTML);
    if (!m) return [];
    return [{
      getAttribute: (a) => (a === 'data-quest-id' ? m[1] : null),
      addEventListener: (ev, fn) => bound.push({ ev, fn }),
    }];
  };
  return bound;
}
reset();
mapRooms.clear();
let bound = withToggleCapture();
handleGMCP('Char.Quests', longQuests(LONG_HINT));
let p8 = panelText();
check('a long hint renders the clamp class', /class="qhint-text qhint-clamp"/.test(p8), p8);
check('a long hint renders a toggle labelled "more"', /<button type="button" class="qhint-more"[^>]*>more<\/button>/.test(p8), p8);
// The clamped span holds only escaped text, so its first "</span>" ends it.
const clampStart = p8.indexOf('class="qhint-text');
const clampEnd = clampStart < 0 ? -1 : p8.indexOf('</span>', clampStart);
check('the direction suffix is outside the clamped element',
  clampEnd > -1 && p8.indexOf('head east') > clampEnd && !p8.slice(clampStart, clampEnd).includes('qonmap'), p8);
check('the toggle is outside the clamped element', clampEnd > -1 && p8.indexOf('qhint-more') > clampEnd, p8);
check('one click handler bound to the toggle', bound.length === 1 && bound[0].ev === 'click', JSON.stringify(bound));
const noClick = [{ fn: () => {} }];
if (!bound.length) bound = noClick;
let stopped = false;
bound[0].fn({ stopPropagation: () => { stopped = true; }, preventDefault: () => {} });
p8 = panelText();
check('the click does not reach the row focus handler (stopPropagation)', stopped, 'stopPropagation not called');
check('after a click the clamp is lifted and the toggle says "less"',
  /class="qhint-text"/.test(p8) && !/qhint-clamp/.test(p8) && />less<\/button>/.test(p8), p8);
bound = withToggleCapture();
handleGMCP('Char.Quests', longQuests(LONG_HINT + ' Mind the dogs.'));
p8 = panelText();
check('a re-render from a new push keeps the hint expanded', />less<\/button>/.test(p8) && !/qhint-clamp/.test(p8), p8);
if (!bound.length) bound = noClick;
bound[0].fn({ stopPropagation: () => {}, preventDefault: () => {} });
check('a second click clamps it again', /qhint-clamp/.test(panelText()) && />more<\/button>/.test(panelText()), panelText());

reset();
handleGMCP('Char.Quests', longQuests(AT_CAP));
let p8s = panelText();
check('a hint of exactly 200 characters renders no clamp', !/qhint-clamp/.test(p8s), p8s);
check('and no toggle', !/qhint-more/.test(p8s), p8s);
reset();
handleGMCP('Char.Quests', longQuests('Type talk pell.'));
p8s = panelText();
check('a short hint renders neither clamp nor toggle', !/qhint-clamp/.test(p8s) && !/qhint-more/.test(p8s), p8s);
check('a short hint still shows the direction', p8s.includes('head east'), p8s);
reset();
handleGMCP('Char.Quests', longQuests('a ' + ' \n\t '.repeat(300) + 'b'));
p8s = panelText();
check('whitespace runs collapse before counting', !/qhint-more/.test(p8s), p8s);

console.log('SCENARIO 9: a cross-zone destination sends next_room but no target_room');
reset();
handleGMCP('Char.Quests', [{ id: 9, name: 'Far Off', completion: 10, focused: true,
  hint: 'Go far.', target_room: 0, next_room: 420, next_dir: 'west', target_kind: 'area' }]);
check('the off-map arrow still gets a marker', markerCalls.length > 0 && markerCalls[markerCalls.length - 1] &&
  markerCalls[markerCalls.length - 1].nextRoom === 420, JSON.stringify(markerCalls));

console.log('SCENARIO 10: the marker carries the destination fields');
reset();
handleGMCP('Char.Quests', [{ id: 45, name: 'The Unquiet Grove', completion: 40, focused: true, hint: 'Cast in the grove.',
  target_room: 5311, next_room: 5310, next_dir: 'north', target_kind: 'area', target_label: 'the unquiet grove',
  target_rooms: [5311, 5312, 5313], inside: false, guide: 'Head north.' }]);
const m10 = markerCalls[markerCalls.length - 1] || {};
check('kind passed', m10.kind === 'area', JSON.stringify(m10));
check('label passed', m10.label === 'the unquiet grove', JSON.stringify(m10));
check('rooms passed', JSON.stringify(m10.targetRooms) === '[5311,5312,5313]', JSON.stringify(m10));
check('inside passed', m10.inside === false, JSON.stringify(m10));
reset();
handleGMCP('Char.Quests', [{ id: 45, name: 'The Unquiet Grove', completion: 40, focused: true, hint: 'Cast.',
  target_kind: 'zone', target_label: 'Pothole Coulee', inside: true, guide: 'You are in Pothole Coulee.' }]);
const m10b = markerCalls[markerCalls.length - 1];
check('an inside destination with no pin or hop still reaches the map', !!m10b && m10b.inside === true && m10b.kind === 'zone',
  JSON.stringify(markerCalls));

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
