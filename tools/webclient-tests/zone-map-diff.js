#!/usr/bin/env node
/*
 * Zone.Map diffs (#650): EXECUTES the real client against the message shapes
 * the server now sends.
 *
 *   node tools/webclient-tests/zone-map-diff.js
 *
 * The server used to send every visited room on every step (about 271 KB for
 * an explored character). It now sends the whole map with full:true only on a
 * new connection, a zone change or a floor change; otherwise full:false with
 * only the new or changed rooms, or no Zone.Map at all.
 *
 * Zero dependencies. Evaluates the real RoomGridSVG (static/js/gmcp.js) and
 * the real GMCPUpdateHandlers map and handleGMCP (webclient-pure.html), with a
 * stub DOM, then checks:
 *   1. the Zone.Map handler passes the full flag (a missing flag means full,
 *      for an older server);
 *   2. a partial map merges into the rooms already drawn;
 *   3. a full map on the same zone and floor replaces them;
 *   4. a party-only partial moves the party markers;
 *   5. Room.Info for a room the map already has does not strip that room's
 *      name and service tags (no Zone.Map follows to put them back);
 *   6. moving to a known room re-fogs the rooms around the old and new centre
 *      (no Zone.Map follows to rebuild every token).
 */
'use strict';

const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
const html = fs.readFileSync(path.join(ROOT, 'webclient-pure.html'), 'utf8');
const js = fs.readFileSync(path.join(ROOT, 'static', 'js', 'gmcp.js'), 'utf8');

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
const handleGMCPSrc = funcSrc(html, 'function handleGMCP(');

// ---------------------------------------------------------------- DOM stub
function El(tag) {
  this.tag = tag; this.children = []; this.attrs = {}; this.style = {}; this.dataset = {};
  this._html = ''; this.textContent = '';
  Object.defineProperty(this, 'innerHTML', {
    get() { return this._html; }, set(v) { this._html = v; this.children = []; }
  });
  Object.defineProperty(this, 'firstChild', { get() { return this.children[0] || null; } });
  Object.defineProperty(this, 'parentNode', { get() { return this._parent || null; } });
}
El.prototype.setAttribute = function (k, v) { this.attrs[k] = v; };
El.prototype.getAttribute = function (k) { return this.attrs[k]; };
El.prototype.removeAttribute = function (k) { delete this.attrs[k]; };
El.prototype.appendChild = function (c) { c._parent = this; this.children.push(c); return c; };
El.prototype.removeChild = function (c) {
  const i = this.children.indexOf(c); if (i >= 0) this.children.splice(i, 1); return c;
};
El.prototype.append = function () {
  for (const c of arguments) { if (c && typeof c === 'object') this.appendChild(c); }
};
El.prototype.insertBefore = function (n) { return this.appendChild(n); };
El.prototype.remove = function () { if (this._parent) this._parent.removeChild(this); };
El.prototype.addEventListener = function () {};
El.prototype.querySelector = function () { return null; };
El.prototype.querySelectorAll = function () { return []; };
El.prototype.getBoundingClientRect = function () { return { width: 400, height: 400 }; };
Object.defineProperty(El.prototype, 'clientWidth', { get() { return 400; } });
Object.defineProperty(El.prototype, 'clientHeight', { get() { return 400; } });

global.document = {
  createElement: (t) => new El(t),
  createElementNS: (ns, t) => new El(t),
  getElementById: () => new El('div'),
  querySelector: () => new El('div'),
  querySelectorAll: () => [],
  addEventListener: () => {},
};
global.window = { addEventListener: () => {}, devicePixelRatio: 1 };
global.requestAnimationFrame = (fn) => fn();
global.GMCPStructs = {};
global.companionGlue = null;

const noop = () => {};
for (const name of [
  'updateStatusPanel', 'renderInventory', 'renderCommsTabs', 'updateVitalsWindow',
  'renderAutomation', 'rebuildTickTimers', 'renderRoom', 'setConnected',
  'renderMutationToast', 'onActionResult', 'onActionInterrupted', 'renderCommands',
  'SendGMCP', 'appendGameText', 'updateMapPanelTitle', 'renderParty',
  'drainQueue', 'updateActionBar', 'setCooldowns', 'refreshCommandList',
  'charDisplayName', 'handleActionResult', 'renderQuests',
]) {
  if (!(name in global)) global[name] = noop;
}

// eslint-disable-next-line no-eval
(0, eval)([
  js,
  'globalThis.RoomGridSVG = RoomGridSVG;',
  'globalThis.allRooms = { currentZoneKey: "", roomZones: {} };',
  handlersMap.replace(/^\{/, 'globalThis.GMCPUpdateHandlers = {') + ';',
  handleGMCPSrc,
].join('\n'));

// ---------------------------------------------------------------- helpers
let fails = 0;
function check(name, cond, detail) {
  if (cond) console.log('  PASS  ' + name);
  else { fails++; console.log('  FAIL  ' + name + '\n          ' + detail); }
}

// A fresh real map as the page's `gr`, with setZoneSnapshot's arguments recorded.
let snapshotCalls = [];
function freshMap() {
  const g = new globalThis.RoomGridSVG(new El('div'), {});
  const real = g.setZoneSnapshot.bind(g);
  g.setZoneSnapshot = function () { snapshotCalls.push([...arguments]); return real(...arguments); };
  global.gr = g;
  globalThis.allRooms.currentZoneKey = '';
  globalThis.allRooms.roomZones = {};
  global.GMCPStructs = {};
  snapshotCalls = [];
  return g;
}
function room(num, x, extra) {
  return Object.assign({ num, x, y: 0, z: 0, symbol: '.', biome: 'road', name: 'r' + num, exits: [] }, extra || {});
}
function zoneMap(body) { handleGMCP('Zone.Map', body); }
function roomInfo(num, x) {
  handleGMCP('Room.Info', { num, coords: 'Thornwall, ' + x + ', 0, 0', exitsv2: {}, details: [] });
}
function ids(g) { return [...g.rooms.keys()].sort((a, b) => a - b).join(','); }
// The party marker is the only circle filled #1c2e2a (see _buildRoomTokenInGroup).
function hasPartyMarker(g, id) {
  const e = g.rooms.get(id);
  return !!e && e.group.children.some(c => c.attrs && c.attrs.fill === '#1c2e2a');
}
function opacity(g, id) { return g.rooms.get(id).group.attrs.opacity; }

// ---------------------------------------------------------------- scenarios
console.log('SCENARIO 1: the Zone.Map handler passes the full flag');
freshMap();
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [room(1, 0)], full: true });
check('full:true reaches setZoneSnapshot as true', snapshotCalls[0] && snapshotCalls[0][4] === true,
  JSON.stringify(snapshotCalls[0] && snapshotCalls[0][4]));
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [], full: false });
check('full:false reaches setZoneSnapshot as false', snapshotCalls[1] && snapshotCalls[1][4] === false,
  JSON.stringify(snapshotCalls[1] && snapshotCalls[1][4]));
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [room(1, 0)] });
check('a missing flag (older server) means full', snapshotCalls[2] && snapshotCalls[2][4] === true,
  JSON.stringify(snapshotCalls[2] && snapshotCalls[2][4]));

console.log('SCENARIO 2: a partial map merges into the rooms already drawn');
let g = freshMap();
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [room(1, 0), room(2, 1)], full: true });
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [room(3, 2)], full: false });
check('rooms 1, 2 and 3 are all on the map', ids(g) === '1,2,3', 'rooms=' + ids(g));

console.log('SCENARIO 3: a full map on the same zone and floor replaces the rooms');
zoneMap({ zone: 'Thornwall', cz: 0, party: [], rooms: [room(1, 0), room(2, 1)], full: true });
check('only rooms 1 and 2 remain', ids(g) === '1,2', 'rooms=' + ids(g));

console.log('SCENARIO 4: a party-only partial moves the party marker');
g = freshMap();
zoneMap({ zone: 'Thornwall', cz: 0, party: [2], rooms: [room(1, 0), room(2, 1)], full: true });
check('marker starts in room 2', hasPartyMarker(g, 2) && !hasPartyMarker(g, 1),
  'r1=' + hasPartyMarker(g, 1) + ' r2=' + hasPartyMarker(g, 2));
zoneMap({ zone: 'Thornwall', cz: 0, party: [1], rooms: [], full: false });
check('marker moved to room 1', hasPartyMarker(g, 1), 'room 1 has no marker');
check('room 2 lost its marker', !hasPartyMarker(g, 2), 'room 2 still shows the party member');

console.log('SCENARIO 5: Room.Info into a known room keeps its name and tags');
g = freshMap();
roomInfo(1, 0); // login: Room.Info first, then the full map
zoneMap({ zone: 'Thornwall', cz: 0, party: [],
  rooms: [room(1, 0, { name: 'Bank Hall', tags: ['bank'] }), room(2, 1)], full: true });
roomInfo(2, 1); // a step into a known room: no Zone.Map follows
roomInfo(1, 0); // and back
const r1 = g.rooms.get(1).room;
check('room 1 keeps its name', r1.name === 'Bank Hall', 'name=' + JSON.stringify(r1.name));
check('room 1 keeps its bank tag', Array.isArray(r1.tags) && r1.tags.includes('bank'), 'tags=' + JSON.stringify(r1.tags));
check('room 1 keeps its exits metadata', Array.isArray(r1.ExitsMeta), 'ExitsMeta=' + JSON.stringify(r1.ExitsMeta));
check('the current room is room 1', g.currentCenterId === 1, 'centre=' + g.currentCenterId);

console.log('SCENARIO 6: moving to a known room re-fogs the neighbourhood');
g = freshMap();
roomInfo(1, 0);
zoneMap({ zone: 'Thornwall', cz: 0, party: [],
  rooms: [room(1, 0), room(2, 1), room(3, 2), room(4, 3), room(5, 4)], full: true });
check('next to the start, room 2 is lightly fogged', String(opacity(g, 2)) === '0.92', 'opacity=' + opacity(g, 2));
check('far from the start, room 5 is fogged', String(opacity(g, 5)) === '0.74', 'opacity=' + opacity(g, 5));
roomInfo(4, 3); // known room, no Zone.Map follows
check('room 5, now adjacent, is lightly fogged', String(opacity(g, 5)) === '0.92', 'opacity=' + opacity(g, 5));
check('room 2, now distant, is fogged', String(opacity(g, 2)) === '0.74', 'opacity=' + opacity(g, 2));
check('room 3, adjacent to the new centre, is lightly fogged', String(opacity(g, 3)) === '0.92', 'opacity=' + opacity(g, 3));

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
