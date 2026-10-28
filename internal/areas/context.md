# Areas Context

## Purpose

`internal/areas` holds the named sets of rooms that quest triggers refer to
(#375 phase 3b slice 4): "the unquiet grove", "the steppe". Each area is one
YAML file under `_datafiles/world/dogmud/areas/` listing whole zones and/or
single rooms. At boot every area is resolved to a set of template room ids,
with a reverse index from room to areas. It is a leaf package: it imports
only `configs`, `mudlog` and yaml, and is handed the room table by its
caller, so `rooms`, `quests` and `questengine` can all import it.

## Data format

```yaml
key: unquiet_grove          # must equal the file name without .yaml
name: the unquiet grove     # player-facing
zones: [Pothole Coulee]     # optional: zone names as rooms store them
rooms: [5311, 5312, 5313]   # optional: template room ids
```

Decoding is strict (unknown fields refuse). A file refuses when its key is
not its file name, it has no name, it names an unknown zone or room, or it
resolves to no rooms. A room already inside one of the area's zones is a
logged warning. Files that do not end in `.yaml` are ignored, so the
manifest `area-keys.txt` lives beside them.

`area-keys.txt` is a `internal/manifest` list of shipped area keys; the guard
`internal/guards/area_manifest_test.go` makes a key leave content only
through a `# retired` or `# renamed` line.

## Files

- **areas.go**: `Area{Key, Name, Zones, Rooms}`; the private `registry`
  (areas, room sets, reverse index) and `resolve`.
- **loader.go**: `DataPath()` (`<DataFiles>/areas`),
  `LoadDataFiles(roomZones map[int]string) error`, and the read API:
  `Get(key)`, `Contains(key, roomId)`, `AreasOf(roomId)` (sorted keys, shared
  slice), `Rooms(key)` (sorted ids), `Keys()` (sorted), and for quest
  destinations (#375 phase 4) `ZonesOf(key)` (the zones the area's rooms lie
  in, sorted) and `RoomsIn(key, zone)` (the area's template rooms in one zone,
  sorted, nil when it has none there). Both hand out shared slices: do not
  change them.
  `InstallForTest(areas, roomZones) (restore func())` installs areas for
  another package's tests.

## Use

- `main.go` calls `LoadDataFiles(templateRoomZones())` after
  `rooms.LoadDataFiles()` and before `questengine.LoadDataFiles()`. At boot an
  error panics; on a data reload the previous areas stay and the error is
  returned with the reload's result.
- The registry is swapped whole behind an atomic pointer, so the read API is
  safe from any goroutine. Room ids are template ids: map an instance room
  through `rooms.OriginalRoomId` before asking.
- A missing `areas/` directory loads no areas.
- Zone names in `zones:` are display names as rooms store them (`Pothole
  Coulee`), not file-style keys. The caller supplies them as
  `rooms.TemplateRoomZones()` (template room id to zone; see
  `internal/rooms/context.md`), so every template room is known at boot even
  though rooms load lazily.

## Consumers

- `internal/questengine`: `matchTriggerFields` tests a trigger's `area:` with
  `Contains(key, templateRoomId)`; `CheckExplore` finds the areas holding a
  newly mapped room with `AreasOf`; `StepProgress` and `exploreMet` read
  `Rooms(key)`; the explore line names the area through `Get(key).Name`; the
  walkthrough's `walkRoomIn` stands a synthetic event in the area's lowest room.
  For destinations, `ResolveDestination`'s area set tests membership with
  `Contains`, routes with `ZonesOf` and washes the map with `RoomsIn`.
- `internal/questworld`: `Validators()` wires `AreaExists`, `AreaSize` and
  `AreaGatedRoom` from `Get` and `Rooms`, and `addGateTriggerRooms` marks every
  room of a `room_gate` trigger's area as gated.
- `internal/guards/area_manifest_test.go`: the key manifest guard.

## Gotchas

- A trigger's `area:` is a key, never the player-facing `name`.
- Instance rooms are not in any area. Event sources already pass the template
  room (`questengine.EventAt`); do the same for any new reader.
- A new area file is a new key: add it to `area-keys.txt` or the manifest guard
  fails.
