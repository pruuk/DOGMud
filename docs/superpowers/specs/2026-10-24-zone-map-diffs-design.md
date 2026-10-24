# Zone.Map Diffs (Design, #650)

> Owner approved approach A on 2026-10-24 ("good enough for now"). Follows PR #671, which
> moved socket writes off the game loop; this cuts the bytes themselves.

## Facts verified against source (master `e049a5b51`, 2026-10-24)

| Fact | Source |
|---|---|
| Every room change queues `GMCPZoneUpdate` (`roomChangeHandler`); a sight change does too, now only when the map really changed (#671) | `modules/gmcp/gmcp.Zone.go:44-55`, `modules/gmcp/gmcp.Room.go:88-89` |
| `buildAndSend` sends `Zone.Map` with `{zone, cz, party, rooms}`, `rooms` being `mapper.Snapshot(visited)` of every visited room the zone crawl reaches | `gmcp.Zone.go:57-120` |
| Measured payload: about 271 KB (1,209 rooms) per move for a fully explored character; 4 to 5 KB for a fresh one | 10-24 A/B investigation |
| `SnapshotRoom{num, x, y, z, plane, symbol, biome, name, tags, exits[], foreign, ozone}`; exits carry `to, dx, dy, dz, kind, locked, secret, oneway, gate, stub, tozone`; `tags` (bank, storage, shop) come from live room state | `internal/mapper/mapper.snapshot.go` |
| Web client `Zone.Map` handler calls `gr.setZoneSnapshot(zone, rooms, cz, party)` | `_datafiles/html/public/webclient-pure.html:789-799` |
| `setZoneSnapshot` resets the map only when zone or floor changes; otherwise `addRoom` places or updates each room it is given and keeps every other room. So a payload with only some rooms already merges | `_datafiles/html/public/static/js/gmcp.js:461-520` |
| Login fires `events.PlayerSpawn`; logout `events.PlayerDespawn` | `modules/gmcp/gmcp.Char.go:59,80` |

## Design

The server remembers, per player connection, what it last sent and sends only what changed. The
message stays `Zone.Map`.

- **State:** a map in the GMCP zone module keyed by user id, holding the connection id it was
  built for, the zone, the floor (`cz`), the party list, and a fingerprint per room id. Cleared on
  `PlayerDespawn`. A different connection id (a reconnect or a second login) counts as no state.
- **Full send** (`full: true`, every room): no state for this connection, a zone change, or a floor
  change. The state is replaced.
- **Partial send** (`full: false`): only rooms whose fingerprint is new or different. `party` is
  sent on every Zone.Map (`[]` when none; refined in the plan), and a change in party alone sends a
  partial with no rooms. Zone and `cz` are always included so the client keys correctly.
- **No send:** nothing changed (the usual step into a known room).
- **Fingerprint:** a hash of everything the client draws for a room: position, plane, symbol,
  biome, name, tags, foreign, owner zone and every exit field. Any visible change re-sends that room.
  Exits are hashed in a sorted order, because `mapper.Snapshot` builds them from a Go map
  (`mapNode.Exits`) and their order differs between calls.
- **Client:** `full: true` (or a missing flag, for safety with an older server) clears the current
  floor's rooms before placing; `full: false` merges as today. The client stays tolerant of an
  absent `party` and `rooms`. Three things relied on a full map after every step and are handled
  in the client instead: party markers on rooms a partial did not re-send are rebuilt; moving the
  centre re-fogs tokens whose distance band changed; and the `Room` handler no longer overwrites a
  room a snapshot drew with its own shape (no name, tags or exit metadata).
- **Builder page:** `build.html` also receives the play client's `Zone.Map` and replaces its canvas
  wholesale, so it ignores a `full: false` map. The builder's own snapshot always sets `full: true`.
- **Client-requested resync** (owner-approved direction, added during the build): if the client's
  map is wiped by anything other than a `full: true` map, it sends GMCP `Zone.Map.Resync` (empty
  payload) and the server forgets that user's sent state and queues an update, so the next map is
  full. The two wipe paths are the `Room` handler (its `Room.Info` zone or floor key changed, as
  when a room with no map coordinates reports z 999... while the server's `cz` stays put) and a
  partial map whose zone or floor differs from what is drawn. The `Room` handler asks only if no
  full map arrives within a 2 s grace, since an ordinary zone or floor change brings one; a
  resetting partial asks at once. A full map never asks, which rules out a loop, and one wipe asks
  at most once. Server side, `HandleIAC` queues the request to MainWorker, like `Char.Quests.Focus`.
- Rooms are never removed (the visited set only grows), so no delete message is needed.
- The snapshot is still built each move (about 3 ms locally); trimming that is out of scope.

## Tests

- Server: full send on first build for a connection, on zone change and on floor change; no send on
  a repeat step into a known room; exactly one room for a newly visited room and for a tag change
  (a merchant arriving); party-only send when party positions change; a new connection id gets a
  full send; despawn clears the state; `Zone.Map.Resync` is queued from the connection goroutine
  and its handler clears the state so the next map is full.
- Client (`tools/webclient-tests/quest-panel-refresh.js` style, a new zone-map test file): a partial
  message merges into the existing rooms; a `full` message resets the floor; a message with no
  `party` leaves party markers as they were; a wipe by anything but a full map sends exactly one
  `Zone.Map.Resync`, and a full map (even one that resets) sends none.
- End to end: re-run the 10-24 two-client A/B (scratchpad `lag/client`, `e2e/`) and report bytes per
  move before and after.

## Out of scope

Builder `Build.*` zone snapshots (unfogged, admin), and the per-move snapshot build cost.
