#!/usr/bin/env node
/*
 * Quest destination rendering on the web map (#375 phase 4 slice 2): EXECUTES
 * the real RoomGridSVG (static/js/gmcp.js) with a stub DOM.
 *
 *   node tools/webclient-tests/quest-destination-map.js
 *
 * Harness copied from zone-map-diff.js.
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
// The page's own map construction, so the test runs the page's wiring.
const initMapSrc = funcSrc(html, 'function initMap(');

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

// Client-to-server GMCP, recorded.
let sentGMCP = [];
global.SendGMCP = (pkg, obj) => { sentGMCP.push([pkg, obj]); return true; };
function resyncsSent() { return sentGMCP.filter(s => s[0] === 'Zone.Map.Resync'); }

// Manual timers: flushTimers() runs whatever is pending, as if time passed.
// timerDelays records the delay each setTimeout asked for.
let timers = new Map();
let timerSeq = 0;
let timerDelays = [];
global.setTimeout = (fn, ms) => { const id = ++timerSeq; timers.set(id, fn); timerDelays.push(ms); return id; };
global.clearTimeout = (id) => { timers.delete(id); };
function flushTimers() {
  const due = [...timers.values()];
  timers.clear();
  due.forEach(fn => fn());
}

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
  // eslint-disable-next-line no-eval
  (0, eval)('(' + initMapSrc + ')();'); // assigns the global gr, as the page does
  const g = global.gr;
  const real = g.setZoneSnapshot.bind(g);
  g.setZoneSnapshot = function () { snapshotCalls.push([...arguments]); return real(...arguments); };
  global.gr = g;
  globalThis.allRooms.currentZoneKey = '';
  globalThis.allRooms.roomZones = {};
  global.GMCPStructs = {};
  snapshotCalls = [];
  sentGMCP = [];
  timers.clear();
  timerDelays = [];
  return g;
}
function room(num, x, extra) {
  return Object.assign({ num, x, y: 0, z: 0, symbol: '.', biome: 'road', name: 'r' + num, exits: [] }, extra || {});
}
function zoneMap(body) { handleGMCP('Zone.Map', body); }
function roomInfo(num, x, z, zone) {
  handleGMCP('Room.Info', { num, coords: (zone || 'Thornwall') + ', ' + x + ', 0, ' + (z || 0), exitsv2: {}, details: [] });
}

// The wash group is the first child of worldSvg (under connections and rooms).
function washTiles(g) { return g.washGroup.children.filter(c => c.tag === 'rect'); }
function washEdges(g) { return g.washGroup.children.filter(c => c.tag === 'line'); }
function washLabel(g) { return g.washGroup.children.filter(c => c.tag === 'text').map(t => t.textContent).join('|'); }

console.log('SCENARIO 1: an area washes its rooms on this map, with a dashed outer edge and its name');
let g = freshMap();
// An L: 1(0,0) 2(1,0) 3(1,1) in the area; 4(2,0) not.
zoneMap({ zone: 'Grove', cz: 0, party: [], full: true, rooms: [
  room(1, 0), room(2, 1), room(3, 1, { y: 1 }), room(4, 2) ] });
g.setQuestMarker({ targetRoom: 1, nextRoom: 2, nextDir: 'west', name: 'Grove', kind: 'area',
  label: 'the unquiet grove', targetRooms: [1, 2, 3], inside: false });
check('one tile per area room on the map', washTiles(g).length === 3, washTiles(g).length);
// Outer sides of the L: room1 3 sides (top, left, bottom), room2 2 (top, right), room3 3 (left, right, bottom) = 8.
check('a dashed edge on every side not shared with another area room', washEdges(g).length === 8, washEdges(g).length);
check('every edge is dashed', washEdges(g).every(e => !!e.attrs['stroke-dasharray']), JSON.stringify(washEdges(g).map(e => e.attrs)));
check('the area name is drawn once', washLabel(g) === 'the unquiet grove', washLabel(g));
check('brass, never the party teal', washTiles(g).every(t => t.attrs.fill === RoomGridSVG.LEATHER.questGold), JSON.stringify(washTiles(g)[0] && washTiles(g)[0].attrs));

console.log('SCENARIO 2: rooms not on this map or floor get no tile; null clears');
g.setQuestMarker({ targetRoom: 1, nextRoom: 0, nextDir: '', name: 'Grove', kind: 'area',
  label: 'the unquiet grove', targetRooms: [1, 999], inside: false });
check('only rooms on the map are washed', washTiles(g).length === 1, washTiles(g).length);
g.setQuestMarker(null);
check('no marker, no wash', g.washGroup.children.length === 0, g.washGroup.children.length);

console.log('SCENARIO 3: a single room or a zone destination draws no wash');
g.setQuestMarker({ targetRoom: 1, nextRoom: 0, nextDir: '', name: 'Q', kind: 'room', label: '', targetRooms: [], inside: false });
check('room: no wash', g.washGroup.children.length === 0, g.washGroup.children.length);

console.log('SCENARIO 4: the wash survives a full map reset');
g.setQuestMarker({ targetRoom: 1, nextRoom: 2, nextDir: 'west', name: 'Grove', kind: 'area',
  label: 'the unquiet grove', targetRooms: [1, 2, 3], inside: false });
zoneMap({ zone: 'Grove', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1), room(3, 1, { y: 1 }), room(4, 2) ] });
check('redrawn after reset', washTiles(g).length === 3, washTiles(g).length);

function bannerText(g) {
  return g.overlayGroup.children.filter(c => c.attrs && c.attrs['data-quest-banner'] === '1')
    .map(c => c.textContent || '').join('|');
}
console.log('SCENARIO 5: inside an area or zone, a banner names it; outside, none');
g = freshMap();
zoneMap({ zone: 'Pothole Coulee', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1) ] });
g.setQuestMarker({ targetRoom: 0, nextRoom: 0, nextDir: '', name: 'Q', kind: 'zone', label: 'Pothole Coulee', targetRooms: [], inside: true });
check('banner says Somewhere in <label>', bannerText(g).indexOf('Somewhere in Pothole Coulee') >= 0, bannerText(g));
g.setQuestMarker({ targetRoom: 0, nextRoom: 2, nextDir: 'east', name: 'Q', kind: 'zone', label: 'Pothole Coulee', targetRooms: [], inside: false });
check('no banner outside', bannerText(g) === '', bannerText(g));
g.setQuestMarker({ targetRoom: 1, nextRoom: 0, nextDir: '', name: 'Q', kind: 'room', label: '', targetRooms: [], inside: true });
check('no banner for a room destination', bannerText(g) === '', bannerText(g));
g.setQuestMarker({ targetRoom: 0, nextRoom: 0, nextDir: '', name: 'Q', kind: 'area', label: 'the unquiet grove', targetRooms: [1, 2], inside: true });
zoneMap({ zone: 'Pothole Coulee', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1) ] });
check('banner survives a full map reset', bannerText(g).indexOf('Somewhere in the unquiet grove') >= 0, bannerText(g));

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
