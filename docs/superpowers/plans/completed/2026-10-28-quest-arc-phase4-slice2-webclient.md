# #375 phase 4 slice 2: web client destination rendering, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The web map draws a quest's area as a brass wash with a dashed edge and its name, shows a
"Somewhere in <name>" banner when the player stands inside an area or zone destination, moves the
destination pin from teal to brass, and the quest panel shows the server's destination line.

**Architecture:** The server already sends, for the focused quest, `target_kind`, `target_label`,
`target_rooms`, `inside`, `guide` (slice 1, `modules/gmcp/gmcp.Char.go`). The page's `Char.Quests`
handler passes them to `RoomGridSVG.setQuestMarker`; the map draws the wash in a new group under the
connections, and the banner in its design-coordinate overlay. The panel shows `guide` where it showed
"→ head <dir>".

**Tech stack:** plain JavaScript in `_datafiles/html/public/static/js/gmcp.js` and
`_datafiles/html/public/webclient-pure.html`; zero-dependency node checks in `tools/webclient-tests/`.

**Spec:** `docs/superpowers/specs/2026-10-28-quest-arc-phase4-minimap-guidance-design.md`, rulings 8
and 9 and section 3 (owner approved 2026-10-28). Slice 2 of 3. Mockups: area render B (wash), zone
render A (banner).

---

## Ground rules

- Edit files with the Edit and Write tools only (no `sed -i`, no `>` into tracked files, no Python
  rewrites). `git add` named paths only; never `-A`, `.`, `--all`, `commit -a`, `--no-verify`.
- Commits via quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Every new check is seen failing before it passes.
- Colour: green against blue never carries meaning (owner is green/blue colourblind). Brass and
  brightness only.
- No em or en dashes in comments or copy.
- Work in a worktree `C:/tmp/dogmud-375-p4s2` on branch `feat/375-p4-webclient` from master; record
  the path in memory while it exists.

## Facts verified against source (master `f9d9608db`, 2026-10-28)

| Fact | Where |
|---|---|
| `Char.Quests` handler calls `gr.setQuestMarker({targetRoom, nextRoom, nextDir, name})` when the focused quest has `target_room` or `next_room`, else `setQuestMarker(null)` | `webclient-pure.html:832-843` |
| Panel mark: `◆ marked on map` when `target_room` is on `gr.rooms`, else `→ head <next_dir>`; only rendered when the focused quest has a `hint` | `webclient-pure.html:1017-1052` |
| `RoomGridSVG` constructor: `this._questMarker`, `this._questBrassId`, groups `surfaceGroup` (design), `worldSvg` (world coords) holding `connectionsGroup` then `roomsGroup`, `overlayGroup` (design coords, on top) | `static/js/gmcp.js:203-290` |
| World coordinates: a room's centre is `(room.x * spacing, room.y * spacing)`; `spacing` defaults to `roomSize * 2` = 48 | `gmcp.js:208-209, 504-505, 948-949` |
| `setZoneSnapshot` resets on a full map, zone or floor change, clearing `overlayGroup.innerHTML` and re-rendering the surface; it ends with `_drawQuestArrow(); _applyZoom();` | `gmcp.js:528-628` |
| `setQuestMarker(marker)` compares old and new by `targetRoom`, `nextRoom`, `nextDir`, `name`, rebuilds the old and new target tokens, redraws the arrow | `gmcp.js:636-651` |
| The pin is drawn in `_buildRoomTokenInGroup` for the target room: patina halo (`L.questPatina`, opacity 0.30), brass ring, patina core, brass flag, italic name in `L.questPatina` | `gmcp.js:1007-1022` |
| Legend "Quest destination" row draws a patina core | `gmcp.js:919-923` |
| Palette `RoomGridSVG.LEATHER`: `questGold: "#e8b84a"`, `questPatina: "#6bb0a0"`, `party: "#6bb0a0"`, `title: "#e8d2a0"`, `legendBg: "#241810"` | `gmcp.js:1398-1405` |
| `this._z` is the drawn floor; `this.rooms` maps room id to `{room, group}` with `room.x`, `room.y`, `room.z` | `gmcp.js:550-555` |
| Node checks evaluate the real `gmcp.js`, `GMCPUpdateHandlers`, `handleGMCP` and `initMap` with a DOM stub: see `tools/webclient-tests/zone-map-diff.js:1-190` (stub, `freshMap()`, `room(num, x, extra)`, `zoneMap(body)`, `check(name, cond, detail)`); CI runs every `tools/webclient-tests/*.js` | `zone-map-diff.js`; `.github/workflows/validate.yml:102` |
| Panel checks: `tools/webclient-tests/quest-panel-refresh.js` (records `gr.setQuestMarker` calls, `panelText()`) | that file |

## File map

| File | Change |
|---|---|
| `_datafiles/html/public/webclient-pure.html` | handler passes the new fields; panel shows `guide` |
| `_datafiles/html/public/static/js/gmcp.js` | marker fields and equality; `washGroup`; `_drawQuestWash`; `_drawQuestBanner`; brass pin and legend; drop `questPatina` |
| `tools/webclient-tests/quest-destination-map.js` (new) | wash, banner, pin checks |
| `tools/webclient-tests/quest-panel-refresh.js` | marker fields; guide line checks |
| `tools/webclient-tests/quest-destination-demo.html` (new) | a static page that draws the wash, banner and pin from fixtures, for eyes |
| `modules/gmcp/context.md`, `docs/PATCH_NOTES.md`, roadmap, `docs/README.md` | docs |

---

### Task 1: Marker carries the destination fields

**Files:** `webclient-pure.html` (handler), `gmcp.js` (`setQuestMarker`), `tools/webclient-tests/quest-panel-refresh.js`.

- [ ] **Step 1: Failing check.** In `quest-panel-refresh.js`, before the final summary line, add:

```js
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
```

  Run `node tools/webclient-tests/quest-panel-refresh.js`; expected: the new checks FAIL.

- [ ] **Step 2: Handler.** Replace the marker block in the `Char.Quests` handler (`webclient-pure.html:834-842`) with:

```js
                if (gr && gr.setQuestMarker) {
                    var data = (GMCPStructs.Char && GMCPStructs.Char.Quests) ? GMCPStructs.Char.Quests : [];
                    var f = Array.isArray(data) ? data.filter(function (q) { return q.focused; })[0] : null;
                    // A cross-zone destination has no room on this map (target_room 0) but still a
                    // next hop, and an area or zone the player stands in has neither but still draws
                    // its wash and banner (#375 p4).
                    if (f && (f.target_room || f.next_room || f.inside || (f.target_rooms && f.target_rooms.length))) {
                        gr.setQuestMarker({
                            targetRoom: f.target_room || 0, nextRoom: f.next_room || 0, nextDir: f.next_dir || "",
                            name: f.name || "", kind: f.target_kind || "", label: f.target_label || "",
                            targetRooms: Array.isArray(f.target_rooms) ? f.target_rooms : [], inside: !!f.inside
                        });
                    } else {
                        gr.setQuestMarker(null);
                    }
                }
```

- [ ] **Step 3: Equality.** In `setQuestMarker` (`gmcp.js:636-651`) widen `same` to include `kind`,
  `label`, `inside` and the room list:

```js
      const same = (!a && !b) || (!!a && !!b
          && a.targetRoom === b.targetRoom && a.nextRoom === b.nextRoom
          && a.nextDir === b.nextDir && a.name === b.name
          && a.kind === b.kind && a.label === b.label && !!a.inside === !!b.inside
          && String(a.targetRooms || []) === String(b.targetRooms || []));
```

  and after `this._drawQuestArrow();` add `this._drawQuestWash(); this._drawQuestBanner();` (defined in
  Tasks 2 and 3; until then add empty methods `_drawQuestWash() {}` and `_drawQuestBanner() {}` next to
  `_drawQuestArrow` so this task runs).

- [ ] **Step 4:** run the check (PASS) and every `node tools/webclient-tests/*.js` (PASS). Commit both
  files and the check: `feat(webclient): quest marker carries the destination fields (#375 p4)`.

---

### Task 2: The wash

**Files:** `gmcp.js`; create `tools/webclient-tests/quest-destination-map.js`.

- [ ] **Step 1: Failing check.** Create `quest-destination-map.js`. Copy the extraction helpers, the DOM
  stub, the globals, the timer stubs, the `noop` list, the `eval` block and the helpers `check`,
  `freshMap`, `room`, `zoneMap`, `roomInfo` from `tools/webclient-tests/zone-map-diff.js` lines 1-190
  verbatim (change only the header comment), then add:

```js
// The wash group is the first child of worldSvg (under connections and rooms).
function washTiles(g) { return g.washGroup.children.filter(c => c.tag === 'rect'); }
function washEdges(g) { return g.washGroup.children.filter(c => c.tag === 'line'); }
function washLabel(g) { return g.washGroup.children.filter(c => c.tag === 'text').map(t => t.textContent).join('|'); }

console.log('SCENARIO 1: an area washes its rooms on this map, with a dashed outer edge and its name');
let g = freshMap();
// An L: 1(0,0) 2(1,0) 3(1,1) in the area; 4(2,0) not.
zoneMap({ zone: 'Grove', cz: 0, party: [], full: true, rooms: [
  room(1, 0), room(2, 1), room(3, 1, { y: 1 }), room(4, 2) ] });
roomInfo(4, 2, 0, 'Grove');
g.setQuestMarker({ targetRoom: 1, nextRoom: 2, nextDir: 'west', name: 'Grove', kind: 'area',
  label: 'the unquiet grove', targetRooms: [1, 2, 3], inside: false });
check('one tile per area room on the map', washTiles(g).length === 3, washTiles(g).length);
// Outer sides of the L: room1 3 sides (top, left, bottom), room2 2 (top, right), room3 3 (left, right, bottom) = 8.
check('a dashed edge on every side not shared with another area room', washEdges(g).length === 8, washEdges(g).length);
check('every edge is dashed', washEdges(g).every(e => !!e.attrs['stroke-dasharray']), JSON.stringify(washEdges(g).map(e => e.attrs)));
check('the area name is drawn once', washLabel(g) === 'the unquiet grove', washLabel(g));
check('brass, never the party teal', washTiles(g).every(t => t.attrs.fill === RoomGridSVG.LEATHER.questGold), JSON.stringify(washTiles(g)[0].attrs));

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

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
```

  If `room(num, x, extra)` or `roomInfo` differ from what this needs (for example `y` handling), adapt
  the fixture, not the check's intent. Run it: expected FAIL (`washGroup` undefined).

- [ ] **Step 2: Implement.** In the constructor, create the wash group before `connectionsGroup`:

```js
      // Quest destination wash (#375 p4): under connections and rooms, world coords.
      this.washGroup = document.createElementNS(LEATHER_NS, 'g');
      this.worldSvg.appendChild(this.washGroup);
```

  (insert immediately before `this.connectionsGroup = ...`). Add the method next to `_drawQuestArrow`:

```js
  /**
   * Brass wash over the focused quest's destination rooms on this floor (#375 phase 4, spec ruling 8):
   * one tile per room, a dashed brass edge on every side that does not border another washed room,
   * and the area's name above the topmost tile. Only an area, or an NPC or room set of two or more,
   * is washed; a single room has its pin and a zone has its banner.
   */
  _drawQuestWash() {
      const g = this.washGroup;
      while (g.firstChild) g.removeChild(g.firstChild);
      const qm = this._questMarker;
      if (!qm || !Array.isArray(qm.targetRooms) || qm.targetRooms.length === 0) { return; }
      if (qm.kind !== 'area' && qm.targetRooms.length < 2) { return; }
      const L = RoomGridSVG.LEATHER, sp = this.spacing, half = sp / 2;
      const cells = new Map(); // "x,y" -> room
      qm.targetRooms.forEach(id => {
          const e = this.rooms.get(id) || this.rooms.get(String(id));
          if (!e || (e.room.z || 0) !== (this._z || 0)) { return; }
          cells.set(e.room.x + ',' + e.room.y, e.room);
      });
      if (cells.size === 0) { return; }
      let top = null;
      cells.forEach(r => {
          const x = r.x * sp, y = r.y * sp;
          g.appendChild(lEl('rect', { x: x - half, y: y - half, width: sp, height: sp, fill: L.questGold, opacity: '0.13' }));
          const sides = [[0, -1, x - half, y - half, x + half, y - half], [0, 1, x - half, y + half, x + half, y + half],
                         [-1, 0, x - half, y - half, x - half, y + half], [1, 0, x + half, y - half, x + half, y + half]];
          sides.forEach(s => {
              if (cells.has((r.x + s[0]) + ',' + (r.y + s[1]))) { return; }
              g.appendChild(lEl('line', { x1: s[2], y1: s[3], x2: s[4], y2: s[5], stroke: L.questGold,
                  'stroke-width': 1.2, 'stroke-dasharray': '4 3', opacity: '0.85' }));
          });
          if (!top || r.y < top.y || (r.y === top.y && r.x < top.x)) { top = r; }
      });
      if (qm.label) {
          g.appendChild(lTxt({ x: top.x * sp, y: top.y * sp - half - 3, 'text-anchor': 'middle',
              'font-family': 'Georgia,serif', 'font-size': '9', 'font-style': 'italic', fill: L.title }, qm.label));
      }
  }
```

  Call `this._drawQuestWash();` also at the end of `setZoneSnapshot` next to its `_drawQuestArrow()`
  call, and wherever `reset()` clears the world groups make sure `washGroup` is emptied too (read
  `reset()`; if it rebuilds `worldSvg` children, recreate `washGroup` first in it). Use the file's
  own element helpers (`lEl`, `lTxt`) as above; confirm their names by grep.

- [ ] **Step 3:** run `quest-destination-map.js` (PASS) and every other `tools/webclient-tests/*.js`
  (PASS). Commit `gmcp.js` and the new check: `feat(webclient): brass wash over a quest's area (#375 p4)`.

---

### Task 3: The banner

**Files:** `gmcp.js`; `tools/webclient-tests/quest-destination-map.js`.

- [ ] **Step 1: Failing check.** Append before the summary in `quest-destination-map.js`:

```js
function bannerText(g) {
  return g.overlayGroup.children.filter(c => c.attrs && c.attrs['data-quest-banner'] === '1')
    .map(c => c.textContent || '').join('|');
}
console.log('SCENARIO 5: inside an area or zone, a banner names it; outside, none');
g = freshMap();
zoneMap({ zone: 'Pothole Coulee', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1) ] });
roomInfo(1, 0, 0, 'Pothole Coulee');
g.setQuestMarker({ targetRoom: 0, nextRoom: 0, nextDir: '', name: 'Q', kind: 'zone', label: 'Pothole Coulee', targetRooms: [], inside: true });
check('banner says Somewhere in <label>', bannerText(g).indexOf('Somewhere in Pothole Coulee') >= 0, bannerText(g));
g.setQuestMarker({ targetRoom: 0, nextRoom: 2, nextDir: 'east', name: 'Q', kind: 'zone', label: 'Pothole Coulee', targetRooms: [], inside: false });
check('no banner outside', bannerText(g) === '', bannerText(g));
g.setQuestMarker({ targetRoom: 1, nextRoom: 0, nextDir: '', name: 'Q', kind: 'room', label: '', targetRooms: [], inside: true });
check('no banner for a room destination', bannerText(g) === '', bannerText(g));
g.setQuestMarker({ targetRoom: 0, nextRoom: 0, nextDir: '', name: 'Q', kind: 'area', label: 'the unquiet grove', targetRooms: [1, 2], inside: true });
zoneMap({ zone: 'Pothole Coulee', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1) ] });
check('banner survives a full map reset', bannerText(g).indexOf('Somewhere in the unquiet grove') >= 0, bannerText(g));
```

  Run: expected FAIL.

- [ ] **Step 2: Implement** next to `_drawQuestWash`:

```js
  /**
   * "Somewhere in <name>" across the foot of the map while the player stands inside an area or
   * zone destination (#375 phase 4, spec ruling 9). Drawn in the design-coordinate overlay so zoom
   * and pan never move it. The elements carry data-quest-banner so a redraw removes only them.
   */
  _drawQuestBanner() {
      const ov = this.overlayGroup;
      ov.children && ov.children.slice
          ? ov.children.slice().forEach(c => { if (c.getAttribute && c.getAttribute('data-quest-banner') === '1') ov.removeChild(c); })
          : Array.from(ov.childNodes || []).forEach(c => { if (c.getAttribute && c.getAttribute('data-quest-banner') === '1') ov.removeChild(c); });
      const qm = this._questMarker;
      if (!qm || !qm.inside || !qm.label || (qm.kind !== 'area' && qm.kind !== 'zone')) { return; }
      const L = RoomGridSVG.LEATHER;
      const text = 'Somewhere in ' + qm.label;
      const w = Math.min(240, 40 + text.length * 4.6);
      const rect = lEl('rect', { x: 160 - w / 2, y: 262, width: w, height: 14, rx: 3, fill: L.legendBg,
          stroke: L.questGold, 'stroke-width': 0.8, 'data-quest-banner': '1' });
      const label = lTxt({ x: 160, y: 272, 'text-anchor': 'middle', 'font-family': 'Georgia,serif',
          'font-size': '7.5', 'font-style': 'italic', fill: L.title, 'data-quest-banner': '1' }, text);
      ov.appendChild(rect);
      ov.appendChild(label);
  }
```

  Simplify the child-removal to whichever form the real DOM and the test stub both support (real SVG
  elements have `childNodes`/`children` as live collections without `slice`; the stub's `children` is
  an array): `Array.from(ov.children || ov.childNodes || [])` works for both. Call
  `this._drawQuestBanner()` also at the end of `setZoneSnapshot` (the reset empties `overlayGroup`).
  Check the banner does not overlap the legend or compass in the real overlay layout (read where the
  legend is drawn, `gmcp.js` ~880-935, and move the banner's `y` if they collide; say which `y` you
  chose and why).

- [ ] **Step 3:** run all `tools/webclient-tests/*.js` (PASS). Commit: `feat(webclient): banner inside an area or zone destination (#375 p4)`.

---

### Task 4: Panel line and brass pin

**Files:** `webclient-pure.html` (panel mark), `gmcp.js` (pin, legend, palette),
`tools/webclient-tests/quest-panel-refresh.js`, `tools/webclient-tests/quest-destination-map.js`.

- [ ] **Step 1: Failing checks.** In `quest-panel-refresh.js` add:

```js
console.log('SCENARIO 11: the panel shows the server destination line when the pin is not on this map');
reset();
mapRooms.clear();
handleGMCP('Char.Quests', [{ id: 9, name: 'Far Off', completion: 10, focused: true, hint: 'Go far.',
  target_room: 0, next_room: 420, next_dir: 'west', target_kind: 'area', guide: 'Head west.' }]);
check('guide shown', panelText().includes('Head west.'), panelText().slice(0, 240));
check('the old arrow text is not doubled', !panelText().includes('→ head west'), panelText().slice(0, 240));
reset();
mapRooms.clear();
handleGMCP('Char.Quests', [{ id: 9, name: 'Anywhere', completion: 10, focused: true, hint: 'Cast.',
  target_kind: 'anywhere', guide: 'You can do this anywhere.' }]);
check('anywhere shown', panelText().includes('You can do this anywhere.'), panelText().slice(0, 240));
reset();
mapRooms.clear();
mapRooms.set(301, { room: { RoomId: 301 } });
handleGMCP('Char.Quests', [{ id: 2, name: 'Here', completion: 50, focused: true, hint: 'Talk.',
  target_room: 301, next_room: 300, next_dir: 'north', guide: 'Head north.' }]);
check('on-map pin still says marked on map', panelText().includes('marked on map'), panelText().slice(0, 240));
```

  In `quest-destination-map.js` add:

```js
console.log('SCENARIO 6: the pin is brass, never the party teal');
g = freshMap();
zoneMap({ zone: 'T', cz: 0, party: [], full: true, rooms: [ room(1, 0), room(2, 1) ] });
roomInfo(2, 1, 0, 'T');
g.setQuestMarker({ targetRoom: 1, nextRoom: 1, nextDir: 'west', name: 'Velk', kind: 'mob', label: 'Velk', targetRooms: [1], inside: false });
const pinFills = g.rooms.get(1).group.children.map(c => c.attrs && c.attrs.fill).filter(Boolean);
check('no teal in the pin', pinFills.indexOf('#6bb0a0') < 0, JSON.stringify(pinFills));
```

  Run both: expected FAIL.

- [ ] **Step 2: Panel.** In `renderQuests` (`webclient-pure.html` ~1027-1036) change the mark:

```js
                    var mark = '';
                    if (onMap) {
                        mark = ' <span class="qonmap">◆ marked on map</span>';
                    } else if (q.guide) {
                        // The server's destination line (#375 phase 4, questengine.GuideLine), the same
                        // words the `quest` command prints.
                        mark = ' <span class="qonmap">' + escapeHTML(q.guide) + '</span>';
                    } else if (q.next_dir) {
                        mark = ' <span class="qonmap">→ head ' + escapeHTML(q.next_dir) + '</span>';
                    }
```

- [ ] **Step 3: Pin and legend.** In `_buildRoomTokenInGroup`'s pin block (`gmcp.js` ~1011-1020): halo
  `fill: L.questGold` (opacity stays 0.30), core `fill: L.questGold`, name `fill: L.title`. In the legend
  "quest" row (~923) the core `fill: L.questGold`. Remove `questPatina` from `RoomGridSVG.LEATHER`
  after `grep -n questPatina` shows no other user (update any it finds the same way).

- [ ] **Step 4:** run every `tools/webclient-tests/*.js` (PASS). Commit: `feat(webclient): panel shows the destination line; brass pin (#375 p4)`.

---

### Task 5: Demo page, docs, gate

- [ ] **Step 1: Demo page.** Create `tools/webclient-tests/quest-destination-demo.html`: a standalone page
  that loads `../../_datafiles/html/public/static/js/gmcp.js` with a relative `<script src>`, creates
  three `RoomGridSVG` maps side by side (`new RoomGridSVG('#m1')` etc., read the constructor and the
  page's `initMap` for required options), feeds each a fixture through `setZoneSnapshot` and
  `centerOnRoom`, and sets a marker: (1) an L-shaped 6-room area outside the player, (2) inside a zone
  (banner), (3) a single-room pin. It must open from disk in a browser with no server. Add a short
  comment block at the top saying what it is for. Take a screenshot of it in a browser if one is
  available to this session (save as `quest-destination-demo.png` in the repo root, the owner's
  screenshot folder, and do not commit it); otherwise say the owner can open the page.
- [ ] **Step 2: Docs.** `modules/gmcp/context.md` (the client reads the five fields); the web-client
  section of any context.md that describes `setQuestMarker` (grep); `docs/PATCH_NOTES.md` new top entry
  for 2026-10-28 or later: "The web map shades a quest's area and says when you are inside it; the quest
  panel shows the same where-to-go line as the quest log" (plain, no numbers, 80 columns); the roadmap
  phase 4 status (slice 2 merged with PR number); `docs/README.md` (this plan, the demo page).
- [ ] **Step 3: Gate** per `dogmud-shipping`: `gofmt` (no Go change expected), every
  `node tools/webclient-tests/*.js`, `go test ./modules/gmcp/`, `golangci-lint run
  --new-from-merge-base=origin/master`, isolated boot check. Commit docs.
- [ ] **Step 4:** Open the PR (`gh pr create --repo pruuk/DOGMud`), body with the screenshot description
  or the demo page path, the checks, and "Part of #375" (no closing keyword).

## Self-review

- Spec section 3 slice 2 items: wash (Task 2), banner (Task 3), panel line (Task 4), pin colour
  (Task 4), green/blue rule (brass only, the pin's teal removed).
- Names used throughout: `setQuestMarker({targetRoom, nextRoom, nextDir, name, kind, label,
  targetRooms, inside})`, `washGroup`, `_drawQuestWash`, `_drawQuestBanner`, `data-quest-banner`.
