# Zone.Map Diffs (#650) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The server sends the web client a full `Zone.Map` only on a new connection, a zone change or a floor change, and otherwise only the rooms that are new or changed (or nothing), cutting about 271 KB per move to a few hundred bytes or zero.

**Architecture:** Spec `docs/superpowers/specs/2026-10-24-zone-map-diffs-design.md`. A pure decision function in `modules/gmcp` compares the new snapshot with per-user sent state (connection id, zone, floor, party, a fingerprint per room) and returns what to send. `buildAndSend` uses it; `PlayerDespawn` clears the state. The payload gains `full`; the web client resets the floor on `full` and merges otherwise (it already merges).

**Tech Stack:** Go, the GMCP module, vanilla JS web client, Node zero-dependency client tests.

---

## Ground rules

Edit and Write tools only (no `sed -i`, no shell redirection into tracked files); `git add <paths>` only, never `-A`/`.`/`--all`/`commit -a`/`--no-verify`; commits through a quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; never push; never kill a process you did not start; no em or en dashes. Worktree `C:/tmp/dogmud-650`, branch `fix/650-zone-map-diffs` (from master `e049a5b51`).

## Facts (verified 2026-10-24)

The spec's facts table, plus:
- `GMCPZoneModule_Payload{Zone, CurrentZ "cz", Party "party,omitempty", Rooms "rooms", Zones, Biomes}` is shared with the admin builder's `Build.*` zone snapshot (`Zones`, `Biomes`) (`modules/gmcp/gmcp.Zone.go:27-34`).
- Existing tests: `modules/gmcp/gmcp.Zone_test.go`, `gmcp.ZoneSight_test.go`.

**Decision recorded here (refines the spec):** `party` is sent on every `Zone.Map` (no `omitempty`): it is a handful of ints, and an always-present list removes the "empty versus unchanged" ambiguity. A change in party alone still triggers a send. The spec's "party when it changed" line is updated in Task 3.

---

### Task 1: The pure diff function

**Files:** Create `modules/gmcp/gmcp.ZoneDiff.go`, `modules/gmcp/gmcp.ZoneDiff_test.go`

- [ ] **Step 1: Failing tests** (`gmcp.ZoneDiff_test.go`)

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/stretchr/testify/require"
)

func snap(ids ...int) []mapper.SnapshotRoom {
	out := []mapper.SnapshotRoom{}
	for _, id := range ids {
		out = append(out, mapper.SnapshotRoom{RoomId: id, X: id, Symbol: "o", Name: "r"})
	}
	return out
}

func TestZoneDiff_FirstSendIsFull(t *testing.T) {
	d := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.True(t, d.send)
	require.True(t, d.full)
	require.Len(t, d.rooms, 3)
}

func TestZoneDiff_RepeatStepSendsNothing(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2, 3)).next
	d := zoneDiff(st, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.False(t, d.send)
}

func TestZoneDiff_NewRoomSendsOnlyIt(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	d := zoneDiff(st, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.True(t, d.send)
	require.False(t, d.full)
	require.Len(t, d.rooms, 1)
	require.Equal(t, 3, d.rooms[0].RoomId)
}

func TestZoneDiff_ChangedRoomIsResent(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	s := snap(1, 2)
	s[1].Tags = []string{"shop"}
	d := zoneDiff(st, 7, "Thornwall", 0, nil, s)
	require.True(t, d.send)
	require.False(t, d.full)
	require.Len(t, d.rooms, 1)
	require.Equal(t, 2, d.rooms[0].RoomId)
}

func TestZoneDiff_ExitFlagChangeIsResent(t *testing.T) {
	s := snap(1)
	s[0].Exits = []mapper.SnapshotExit{{ToRoomId: 2, DX: 1}}
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, s).next
	s2 := snap(1)
	s2[0].Exits = []mapper.SnapshotExit{{ToRoomId: 2, DX: 1, Locked: true}}
	d := zoneDiff(st, 7, "Thornwall", 0, nil, s2)
	require.True(t, d.send)
	require.Len(t, d.rooms, 1)
}

func TestZoneDiff_ZoneFloorOrConnectionChangeIsFull(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	require.True(t, zoneDiff(st, 7, "Stillwater", 0, nil, snap(1, 2)).full, "zone change")
	require.True(t, zoneDiff(st, 7, "Thornwall", 1, nil, snap(1, 2)).full, "floor change")
	require.True(t, zoneDiff(st, 8, "Thornwall", 0, nil, snap(1, 2)).full, "new connection")
}

func TestZoneDiff_PartyChangeAloneSends(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, []int{5}, snap(1)).next
	d := zoneDiff(st, 7, "Thornwall", 0, []int{6}, snap(1))
	require.True(t, d.send)
	require.False(t, d.full)
	require.Empty(t, d.rooms)
	require.Equal(t, []int{6}, d.party)
	require.False(t, zoneDiff(d.next, 7, "Thornwall", 0, []int{6}, snap(1)).send)
}

func TestZoneDiff_PartyOrderDoesNotMatter(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, []int{5, 6}, snap(1)).next
	require.False(t, zoneDiff(st, 7, "Thornwall", 0, []int{6, 5}, snap(1)).send)
}
```

- [ ] **Step 2:** `go test ./modules/gmcp/ -run TestZoneDiff -v` fails to compile (`undefined: zoneDiff`).

- [ ] **Step 3: Implement** `gmcp.ZoneDiff.go`

```go
package gmcp

import (
	"encoding/json"
	"hash/fnv"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/mapper"
)

// zoneSentState is what the server last sent one player's web client as a
// Zone.Map (#650): the connection it was built for, the zone, the floor, the
// party positions and a fingerprint of every room.
type zoneSentState struct {
	conn  uint64
	zone  string
	cz    int
	party []int
	rooms map[int]uint64 // room id -> fingerprint
}

// zoneDiffResult says what to send: nothing (send false), the whole map
// (full), or only rooms that are new or changed. next replaces the state.
type zoneDiffResult struct {
	send  bool
	full  bool
	rooms []mapper.SnapshotRoom
	party []int
	next  *zoneSentState
}

// zoneDiff compares a fresh snapshot with what was last sent. A new
// connection, a zone change or a floor change sends the full map; otherwise
// only new or changed rooms, and a send at all only when a room or the party
// changed. Rooms are never removed: the visited set only grows.
func zoneDiff(prev *zoneSentState, conn uint64, zone string, cz int, party []int, snap []mapper.SnapshotRoom) zoneDiffResult {
	sortedParty := slices.Clone(party)
	slices.Sort(sortedParty)
	next := &zoneSentState{conn: conn, zone: zone, cz: cz, party: sortedParty, rooms: make(map[int]uint64, len(snap))}
	for _, r := range snap {
		next.rooms[r.RoomId] = roomFingerprint(r)
	}
	if prev == nil || prev.conn != conn || prev.zone != zone || prev.cz != cz {
		return zoneDiffResult{send: true, full: true, rooms: snap, party: sortedParty, next: next}
	}
	var changed []mapper.SnapshotRoom
	for _, r := range snap {
		if old, ok := prev.rooms[r.RoomId]; !ok || old != next.rooms[r.RoomId] {
			changed = append(changed, r)
		}
	}
	partyChanged := !slices.Equal(prev.party, sortedParty)
	if len(changed) == 0 && !partyChanged {
		return zoneDiffResult{next: next}
	}
	return zoneDiffResult{send: true, rooms: changed, party: sortedParty, next: next}
}

// roomFingerprint hashes everything the client draws for a room, so any
// visible change re-sends it.
func roomFingerprint(r mapper.SnapshotRoom) uint64 {
	b, _ := json.Marshal(r)
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}
```

Before relying on `json.Marshal` as the fingerprint, check the snapshot is deterministic: read `mapper.Snapshot` and confirm `Tags` and `Exits` are built in a fixed order (the exits loop must not range over a map). If either comes from map iteration, sort it inside `roomFingerprint` (on a copy) or fix the order in `Snapshot`, and add a test that two snapshots of the same room fingerprint equal.

- [ ] **Step 4:** `go test ./modules/gmcp/ -run TestZoneDiff -v` passes.
- [ ] **Step 5: Commit** `modules/gmcp/gmcp.ZoneDiff.go`, `modules/gmcp/gmcp.ZoneDiff_test.go`: `feat(gmcp): Zone.Map diff decision (#650)`.

---

### Task 2: Wire it into `buildAndSend`, add `full`, clear on despawn

**Files:** Modify `modules/gmcp/gmcp.Zone.go`; test `modules/gmcp/gmcp.Zone_test.go`

- [ ] **Step 1: Read** `gmcp.Zone.go` in full, `gmcp.Zone_test.go` and `gmcp.ZoneSight_test.go` in full, to see how tests build a user, a room and capture `GMCPOut` (`events.DrainQueued...ForTest` helpers or a fake). Check every other producer of `GMCPZoneModule_Payload` (grep) so the builder's payload is unaffected.

- [ ] **Step 2: Failing integration tests** in `gmcp.Zone_test.go`, using that setup:
  - two `GMCPZoneUpdate`s for the same user in the same room: the first queues a `Zone.Map` with `full: true`; the second queues nothing;
  - after moving the user to an unvisited neighbouring room in the same zone, the next update queues a `Zone.Map` with `full: false` and exactly that room;
  - after a `PlayerDespawn` for the user, the next update is `full: true` again.

- [ ] **Step 3: Implement.**
  - Payload: add `Full bool \`json:"full"\`` and make `Party` `\`json:"party"\`` (no `omitempty`; nil becomes `[]` via `party := []int{}` as today). The builder producer sets `Full: true` (grep its construction and set it explicitly), so the builder client is unaffected.
  - Module state: `zoneSent map[int]*zoneSentState` on `GMCPZoneModule` (initialise in `init`), guarded by a `sync.Mutex` (all handlers run on the event loop today; the lock is cheap insurance).
  - In `buildAndSend`, after building `party` and `rooms := m.Snapshot(visited)`:

```go
	g.zoneMu.Lock()
	d := zoneDiff(g.zoneSent[evt.UserId], uint64(user.ConnectionId()), room.Zone, cz, party, rooms)
	g.zoneSent[evt.UserId] = d.next
	g.zoneMu.Unlock()
	if !d.send {
		return events.Continue
	}
	payload := GMCPZoneModule_Payload{
		Zone:     room.Zone,
		CurrentZ: cz,
		Party:    d.party,
		Rooms:    d.rooms,
		Full:     d.full,
	}
```

  Keep `Rooms` non-nil (`d.rooms` may be nil on a party-only send: use `[]mapper.SnapshotRoom{}`), and keep the debug log with `full` and the room count. Check the `user.ConnectionId()` type and convert it correctly.
  - Register `events.RegisterListener(events.PlayerDespawn{}, g.zoneDespawnHandler)`, which deletes `g.zoneSent[evt.UserId]` under the lock.
  - Since `g` is registered by value in `init` today (`g := GMCPZoneModule{...}` then method values), make sure the map and mutex are shared by every handler: use a pointer receiver or put the state in a package-level struct; read how `init` registers handlers and keep one instance.

- [ ] **Step 4:** `go test ./modules/gmcp/` passes (the existing Zone and ZoneSight tests too; if one asserted a full map on every move, update it to the new contract and say so).
- [ ] **Step 5: Commit** `modules/gmcp/gmcp.Zone.go`, `modules/gmcp/gmcp.Zone_test.go`: `feat(gmcp): send Zone.Map diffs, full only on connect, zone or floor change (#650)`.

---

### Task 3: Web client honours `full`, docs

**Files:** Modify `_datafiles/html/public/webclient-pure.html` (Zone.Map handler, about line 789), `_datafiles/html/public/static/js/gmcp.js` (`setZoneSnapshot`, about line 465); create `tools/webclient-tests/zone-map-diff.js`; docs.

- [ ] **Step 1: Failing client test** `tools/webclient-tests/zone-map-diff.js`, built like `quest-panel-refresh.js` (read it in full; it extracts real functions from the page and stubs the DOM): extract the `GMCPUpdateHandlers` map and `handleGMCP`, stub `gr.setZoneSnapshot` to record its arguments, then:
  - a `Zone.Map` with `full: true` calls it with `full === true`;
  - a `Zone.Map` with `full: false` calls it with `full === false`;
  - a `Zone.Map` with no `full` field calls it with `full === true` (safe default for an older server).
  Exit non-zero on failure, print `ALL CHECKS PASSED` on success.
- [ ] **Step 2:** `node tools/webclient-tests/zone-map-diff.js` fails (the argument is missing).
- [ ] **Step 3: Implement.**
  - Handler: `gr.setZoneSnapshot(obj.Map.zone, obj.Map.rooms || [], obj.Map.cz || 0, obj.Map.party || [], obj.Map.full !== false);`
  - `setZoneSnapshot(zone, snapshotRooms, currentZ, party, full)`: change the reset condition to `if (full !== false || this._zone !== zone || this._z !== currentZ)` only when the call is a full send; a partial send must never reset. Concretely: `const isFull = full !== false; if (isFull || this._zone !== zone || this._z !== currentZ) { ...existing reset block... }`. Keep the party rebuild as it is (party is always present now). Read the function to the end and confirm nothing else assumes every room arrives on each call (bounds, centring, quest arrow): `_updateBounds` must account for existing rooms, not only the new ones; if it is computed from the passed rooms alone, compute it from all placed rooms.
- [ ] **Step 4:** `node tools/webclient-tests/zone-map-diff.js` and `node tools/webclient-tests/quest-panel-refresh.js` and every `tools/jstest/*.test.js` pass.
- [ ] **Step 5: Docs.** `modules/gmcp/context.md` (Zone.Map now diffs; the `full` flag; state per user, cleared on despawn); the spec's party line ("party is sent on every Zone.Map"); `docs/README.md` rows for the spec and this plan (follow the table style).
- [ ] **Step 6: Commit** the client files, the new test, the docs: `feat(webclient): Zone.Map full flag; partial maps merge (#650)`.

---

### Task 4: Measure, gate, PR

- [ ] **Step 1: End to end.** Build master `e049a5b51` and this branch on private ports (read `.claude/skills/dogmud-shipping/SKILL.md`; no popup consoles; stop only your PIDs). Using the scratchpad clients from the 10-24 investigation (`C:/Users/CALABE~1/AppData/Local/Temp/claude/C--Users-Calabe-Davis-workspace-DOGMud/c844ec13-7024-41d5-8994-4fd7aea69d1e/scratchpad/lag/client` and `.../e2e/`), with the fully explored test character, run the 60-move Thornwall route over the web socket and record `Zone.Map` bytes per move (median, total) on both. Expected on the branch: one full map at the start, then small or no `Zone.Map` messages. Also load the real web client page in a browser if practical, walk a few rooms, change floor and zone, and confirm the map draws correctly; if not practical, say so.
- [ ] **Step 2: Gate.** `go build ./... && go vet ./... && go test ./...`; `golangci-lint run --new-from-merge-base=origin/master` (binary `~/go/bin/golangci-lint`) 0 issues; `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_`; the JS tests.
- [ ] **Step 3: PR** (controller does this): push and `gh pr create --repo pruuk/DOGMud`, body with the before/after bytes; "Closes #650" is correct here.
