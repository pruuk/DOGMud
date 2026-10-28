# Mapping System Context

## Purpose

`internal/mapper` turns the room graph into geometry. It crawls exits outward
from a root room, assigns each reachable room an (x, y, z) coordinate, and
serves three consumers from that one structure: the ASCII `map` command,
A\* pathfinding, and the web client's leather map snapshot.

One `mapper` exists per zone, cached and keyed by room id. Everything below —
consistency checking, fog-of-war snapshots, cross-zone routing — is built on
the same crawl.

**A zone's map stops at its zone's border** (#375 phase 4, spec ruling 18).
`discover` and `startByCrawl` do not queue an exit whose destination is in
another zone (zone from `rooms.TemplateRoomZone`, falling back to `LoadRoom`
for an instance room id, in `roomZone`). The exit stays on its node and
records the far room's zone in `nodeExit.ToZone`. `buildMapper` (which
`GetMapper` calls) claims only rooms of the map's own zone, so every room belongs to its own zone's map
whichever map is built first. On shipped data (2026-10-28) that is 50 maps
over 1,305 rooms, the largest 175 rooms (Pothole Coulee), no map holding two
zones; Ferries alone has 3 maps, its rooms being unconnected. Before, one map
crawled 1,209 rooms across 38 zones.

## Files

- **mapper.go** — direction/delta tables, the `mapper` type, the crawl, the
  per-zone cache, and the render entry points.
- **mapper.node.go** — `mapNode` and `nodeExit`, the crawl's internal graph.
- **mapper.map.go** — `mapRender` and legend assembly.
- **maptag.go**: `LegendSlug` and `ColorizeLegendLine`, the one colour-tag
  builder for a rendered map line (minimap, `map`, legend template).
- **mapper.ascii.go**: `AsciiMapSymbol`, `mapRender.AsciiCells` and
  `AsciiLegend`, the map's own ASCII cell forms for an ASCII-mode player
  (`map` and the `look` minimap call them after `GetLegend`). The
  `asciiMapSymbols` table holds only glyphs whose shared `ConvertToAscii`
  form would read the same as another legend entry: Deep Water `≈` is `D`
  (Shore is `~`), the town and road biomes' `•` is `o` (Desert is `*`),
  Snow `❄` is `n` and Cliffs `▼` is `c` (the down-exit arrow is `v`, #593).
  UTF-8 players and every other use of those runes are untouched.
  `usercommands`' `TestMapLegendAsciiFormsAreDistinct` scans every biome,
  room mapsymbol, marker and exit arrow (`ExitArrow`) and fails on a
  collision; `TestMapLegendOverridesNameDrawnSymbols` fails on a
  keywords.yaml `legend-overrides` entry for a symbol nothing draws or a
  zone that does not exist.
- **mapper.config.go** — `Config` (render options) and `SymbolOverride`.
- **mapper.path.go** — A\* pathfinding, the path cache, and `GetPath`/`NextStep`.
- **mapper.consistency.go** — the Cartesian consistency engine (see below).
- **mapper.snapshot.go** — the fog-of-war snapshot for the web map.
- **mapper.crosszone.go** — the coarse zone graph and cross-zone hop routing.
- **mapper.nearest.go**: `NearestOf`, the nearest-of-a-set search for quest
  destinations (see Nearest of a set below).

## Coordinates: authored first, crawl second

`(*mapper).discover()` picks one of two placement strategies:

- **`placeByAuthoredCoords()`** when `authoredLayoutClean()` passes — rooms
  carry authored x/y/z/plane, and those are used directly.
- **`startByCrawl()`** otherwise — coordinates are derived by walking exits and
  accumulating deltas from the root room.

This is why the world must stay Cartesian-consistent: under the crawl path, two
rooms whose exit deltas lead to the same cell genuinely collide, and the map
silently loses one of them.

## Public API

### Direction helpers

```go
func GetDelta(exitName string) (x, y, z int)
func GetReciprocalExit(exitDirection string) string
func IsValidExitDirection(exitName string) bool
func IsCompassDirection(exitName string) bool
func IsValidDirection(directionName string) bool
func GetDirectionDeltaNames() []string
func ExitArrow(directionName string) (rune, bool)
func AdjustExitName(exitName string) (newExitName, newExitDirection string, err error)
```

### Mapper lifecycle

```go
func NewMapper(rootRoomId int) *mapper
func GetMapper(roomId int, forceRefresh ...bool) *mapper
func GetMapperIfExists(roomId int) *mapper
func AddMapper(m *mapper, lookupRoomId int)
func PreCacheMaps()
func ClearCache()

func (r *mapper) Start()
func (r *mapper) RootRoomId() int
func (r *mapper) CrawledRoomIds() []int
func (m *mapper) GetCopy() *mapper
func (m *mapper) OverrideRoomIds(replacements map[int]int)
```

`PreCacheMaps` builds every zone's mapper at boot, then warms the cross-zone
graph and runs `ValidateZoneConsistency`.

**Refreshing the caches** (#375 phase 4 Task 21). `ClearCache` drops every
map, the path cache and the zone graph, then rebuilds every zone's map as
`PreCacheMaps` does (`buildAllMaps`): the web map, call for help and
adjacency reach a joined zone only through its cached map, so a cache refilled
one zone at a time would stop them at the border. It runs only on admin
`reload mapcache` and a builder's zone rename or delete. A rebuild drops the
map it replaces from the cache by key, so no stale second map keeps holding
its rooms. `GetMapper(id, true)` rebuilds the
room's map and, through `rebuildJoinedMaps`, every map its border exits reach
(directly or through other zones, each from its own root), then drops the
path cache and the zone graph. Before zone maps stopped at their border, a
forced rebuild re-crawled the one map of the connected world, so the builder's
zone view (`sendZoneMapFull`, which forces its zone's map) and the web map of
a joined zone (`WebMap`, which reads neighbouring maps from the cache) showed
an edit made next door; rebuilding the joined maps keeps that, at master's
cost (the same rooms crawled). Before Task 21 neither refresh touched the path
cache, so a cached failure outlived the edit that opened the route.
`TestCacheRefreshDropsPathsAndZoneGraph` and
`TestForcedRebuildShowsAnEditInAJoinedZone` pin both. `GetCopy`/`OverrideRoomIds` exist for
ephemeral instance rooms, which reuse a template zone's geometry under
different room ids.

### Lookup

```go
func (r *mapper) HasRoom(roomId int) bool
func (r *mapper) GetRoomId(x, y, z int) (roomId int, err error)
func (r *mapper) GetCoordinates(roomId int) (x, y, z int, err error)
func (r *mapper) FindRoomsInDistance(centerRoomId int, xyRadius int, zRadiusOpt ...int) []int
func (r *mapper) FindAdjacentRoom(centerRoomId int, direction string, limitDistance ...int) (roomId, distance int)
```

`FindRoomsInDistance` (call for help, `mobcommands/callforhelp.go`) returns
every room other than the centre within `xyRadius` on each axis whose signed
offset sum `|dx + dy|` is at most `xyRadius`, and within the z radius (0 when
omitted). That is master's range, kept as shipped: it takes a room at
(+2, -2) at radius 2 (sum 0) and leaves out (-1, -1) at radius 1. It is not
Manhattan distance; changing it is a balance call (#707), and
`TestFindRoomsInDistanceKeepsMastersSignedSum` pins master's result. Since
maps stop at their border it also searches, when this map sits at authored
coordinates (`mapper.authored`, set by `placeByAuthoredCoords`), the cached
maps of every zone the zone graph joins to this one, keeping rooms on the
centre's plane; an instance room or instance map searches only its own map
(#375 phase 4 Task 19). Fixed with it: no z radius indexed an empty slice
(panic). Measured against the pre-split tree on shipped data with the
Manhattan form then in place (every room, radius 1 to 4): every room within
range was still found but 22, which sit on another plane (The Foldweave,
plane 5, over plane 0 rooms); none was new.

`FindAdjacentRoom` (`build room <dir>`, `teleport <dir>`) walks cells in a
direction to the first room. Since maps stop at their border it reads a cell
this map lacks from the maps of `WebMap` (`adjacentInOthers`: a room on the
centre's plane first, else one on another plane) and stops outside all of
them, as it stopped outside the one map that held them all (#375 phase 4
Task 20). Compared with 8534cf6a1 on shipped data (every room, ten
directions, one step and unbounded, 13,050 lookups): identical. That
includes a quirk kept on purpose: the one map kept one room per cell
whatever its plane, so from Dustwalk Road 400 `down` finds The Foldweave's
6427 (plane 5), and `build room down` there would join the two planes.

### Rendering

```go
func (r *mapper) GetLimitedMap(centerRoomId int, c Config) mapRender
func (r *mapper) GetFullMap(centerRoomId int, c Config) mapRender
func (m *mapRender) GetLegend(overrides map[rune]string) map[rune]string
func (c *Config) OverrideSymbol(roomId int, symbol rune, legend string)
func (c *Config) SymbolOverrideAt(roomId int) (SymbolOverride, bool)
func LegendSlug(name string) string
func ColorizeLegendLine(line string, legend map[rune]string) string
```

A legend name becomes a colour tag through `LegendSlug` only: lower case,
spaces as hyphens (`map-deep-water`). The colour aliases are keyed by that
slug; with ansitags v1.1.0 a spaced `fg="map-deep water"` matches no alias
and the tile only loses its colour. The tags that printed as raw text in the
#455 playtest came from room noun highlighting rewriting a word inside a map
tag's attribute (`fg="map-deep-<ansi fg="noun">water</ansi>"`);
`rooms.highlightNouns` now matches visible text only and runs before the
minimap joins the line. `ColorizeLegendLine` walks a line
once, rune by rune, so a symbol inside a tag it already wrote is never
rewritten. `usercommands/look.go` (minimap), `usercommands/skill.map.go` and
the `mapslug` template function use it; `maps/map.template`,
`descriptions/room-title.template` and `admincommands/ingame/roominfo.template`
call `mapslug` in both worlds, and `TestMapTagTemplatesUseMapslug` fails any
template that builds a `map-` tag with `lowercase`. A new multi-word legend
needs a `map-<slug>` alias in each world's `ansi-aliases.yaml`: a biome name
(`TestMultiWordBiomesHaveSlugAliases`), or a room `maplegend` or a name the
code writes such as "Party Member" (`TestEveryMultiWordLegendHasSlugAlias`).

`GetLimitedMap` is the player-facing one — the visible radius scales with
Perception. `GetFullMap` is unbounded and used by admin tooling.

`GetLimitedMap` walks every room reachable from the player, across zone
borders, as it did when one map held the connected world. It reads a room
through `drawNode`: this map, else the room's own zone's map, else the room
itself, so with every zone's map built (`PreCacheMaps`) it loads no room
(`TestGetLimitedMapLoadsNoRoomWhenMapsAreBuilt`); between the split and #375
phase 4 Task 19 it reloaded every cold room across a border on each look.
`TestGetLimitedMapGolden` pins a two-zone render. **The walk is not bounded
by the canvas, on purpose:** a room on the canvas can be reachable only by a
path that leaves it (the east side of Stillwater from room 4100 comes round
by the farm road). Measured on shipped data (1,305 rooms, 6 configs, 2026-10-28),
a walk stopped at the canvas plus 3 rooms changed 898 of 7,830 renders, one
stopped at twice the canvas still changed 4 in Cartesian zones; the unbounded
walk matches the pre-change renders but for 1 tie in the toroidal Endless
Trashheap, whose tie order varies run to run on either tree.

### Pathfinding

```go
func GetPath(startRoomId int, endRoomId ...int) ([]pathStep, error)
func NextStep(fromRoomId, toRoomId int) (nextRoomId int, exitName string, found bool)

func (p pathStep) ExitName() string
func (p pathStep) RoomId() int
func (p pathStep) Waypoint() bool
```

A\* over the crawled graph (`heuristic` + a `priorityQueue` of `nodeRecord`),
with results memoised in a path cache keyed by start/end. `GetPath` **searches
across zone maps**: `findPath` reads each room from its own zone's map
(`nodeFor`: this map, else `GetMapperIfExists`, else `GetMapper`), so a route
crosses borders exactly as it did when one map crawled the whole connected
world, and a waypoint in another zone is accepted. Scores start empty and grow
as rooms are reached, so the cost is the rooms the search touches, not a map's
size. The heuristic is the Manhattan distance between node positions (the
authored coordinates, unless a zone fell back to the exit-delta crawl).
`TestShippedRoutesSurviveZoneBoundedMaps` (repo root, `DOGMUD_BOOT_SMOKE`)
holds the (from, to, length) triples of
`testdata/mapper_route_lengths.golden`, recorded from master's mapper, as a
regression baseline: each route may be shorter than recorded or equal, never
longer, and a recorded failure must still fail. A triple whose room no longer
exists is skipped and counted. It also checks the 13 cross-zone patrol and
schedule legs. `NextStep` still falls back to `crossZoneHop` when `GetPath`
fails across zones; see Cross-Zone Routing below.

### Nearest of a set (`mapper.nearest.go`, #375 phase 4)

```go
type SecretExitKnown func(roomId int, exitName string, toRoomId int) bool
func NearestOf(fromRoomId int, in func(roomId int) bool, zones []string, known SecretExitKnown) (pin, nextRoomId int, exitName string, found bool)
```

Where to head toward the nearest room of a set (a quest destination: an
area, an NPC's rooms, a zone). `zones` lists every zone the set touches.

**Secret exits.** Guidance takes a secret exit only when `known` lets it
through (nil: never), in the zone (`nearestIn`) and across a border (the zone
graph marks a secret link, `zoneLink.secret`; `nearestZoneHop` and
`hopToward` skip one `known` refuses). The quest engine passes the player's
own knowledge (`questengine.secretExitsKnownTo`): `search` found the exit, or
the room beyond is on their map, or they were there recently, which is when
the game already discloses that exit to them. Mob routing (`GetPath`,
`pathto`, `NextStep` and its `crossZoneHop` fallback) takes every secret exit,
as before. On shipped data, from room 6323 (The Hedge-Gap Trail, root of
Cascade Pass Road) 1,179 of 1,228 one-room destinations used to route first
through its secret west exit; with the exit unknown none does, and only the
49 other rooms of Cascade Pass Road (20 rooms) and the Eastern Highlands (30)
beyond it get a direction: that pocket joins the rest of the world only
through this secret passage (5441 east in Kingsbarrow Vale, 6323 west back).
A player who walked in through it has 5441 on their map, so their guidance
uses it.

- **Start zone.** Only when the start room's zone is in `zones`
  (`searchesHere`), it runs `nearestIn` on that zone's mapper: a
  breadth-first search that stops at the first member, every exit costing 1
  as in `findPath`, exits taken in name order so a tie always goes the same
  way. The start room is never the answer, and the search never enters a room
  of another zone: not one the map lacks, nor one it holds from another zone
  (#375 phase 4 Task 17). Its cost is bounded by the zone;
  `TestNearestIn_CostDoesNotGrowOutsideTheZone` doubles the rooms outside the
  zone and holds the visit count. It returns that room as `pin` with the first
  hop.
- **Zone hop.** When the start zone is not in `zones`, or no member there is
  reachable, `nearestZoneHop` (`mapper.crosszone.go`) picks the next zone
  toward the zone of `zones` with the fewest zone crossings, and `hopToward`
  runs the same in-zone `nearestIn` to the NEAREST border room among every
  link from the start zone to that next zone (or, standing on one, returns its
  crossing). `pin` is then 0: the room is not on the player's drawn map.
- `found` is false when neither works.

**Cost** (`mapper.nearest_bench_test.go`, a 20x20 grid from one corner to
three rooms in the far corner, Ryzen 9 5900X, 2026-10-28):
`BenchmarkNearestIn400` about 126 us/op, `BenchmarkFindPath400` (uncached A*
to the same room) about 108 us/op. The web quest panel re-resolves the focused
quest on every move, so `searchesHere` skips that crawl whenever the player's
zone holds no member of the set.

On shipped zones (`destination_nearest_bench_test.go`, repo root,
`DOGMUD_BOOT_SMOKE=1 go test . -run x -bench DestinationNearest -count=5`,
medians, one machine, 2026-10-28), master `8534cf6a1` cold `NextStep` against
this tree, largest zone / Greenford: master `NextStep` 90.8 / 82.2 us; new
`GetPath` 13.1 / 6.1 us; `NearestOf` one room 22.7 / 7.3 us, 40 rooms
1.5 / 0.45 us.

## Gotchas

- **Never compare `positionDelta` values with `==`.** Use `samePos`, which
  compares only x/y/z — the struct also carries an `arrow` display field that
  will produce false mismatches.
- **`crawledRooms` is the authoritative node set, not `RoomGrid`.** `RoomGrid`
  is a 3D slice, so when two rooms land on one cell only the last writer
  survives; a collision is invisible there and visible in `crawledRooms`.
- **`ClearCache` is a package-level name that also exists in several other
  packages.** Confirm which one you are calling.
- **A map holds only its own zone's rooms.** Anything that reads
  `crawledRooms` for a room across a border (an exit's far room, a radius
  search, a render) finds nothing in this map; look the room up in its own
  zone's map (`GetMapperIfExists` / `GetMapper`), as `findPath` does. The
  destination search (Task 17), the ASCII map and `FindRoomsInDistance`
  (Task 19), the web map (`WebMap`, Task 18), the consistency checks, the
  builder's zone view and `FindAdjacentRoom` (Task 20) follow that rule.
- **Fixture room ids in tests must not repeat across tests.** The template
  zone index (`rooms.TemplateRoomZone`), which the crawl reads for an exit's
  zone, outlives a test, so a reused id keeps the zone an earlier test gave
  it and the crawl stops where it should not.

## Dependencies

`rooms`, `exit`, `configs`, `mudlog`, `util`, `gametime` (for biome/light
context during render), plus `container/heap` for the A\* queue.

## Consumers

`internal/usercommands` (`map`, `pathto`, `cartcheck`), `internal/mobs`
(schedule and patrol movement), `internal/questengine` (`NearestOf` for quest
destinations),
`internal/rooms`, and `modules/gmcp` (the `Zone.Map` payload).

## Cartesian Consistency Engine

The mapper enforces geometric coherence between room coordinates and
declared exits. The entry point is `ValidateZoneConsistency()`, called
at the tail of `PreCacheMaps()` and gated by the config knob
`GamePlay.MapConsistencyEnforce` (`off` | `warn` (default) | `panic`).
The same logic is exposed on demand via the `cartcheck [zone]` admin
command (`internal/usercommands/admin.cartcheck.go`).

### Core Method

```go
(*mapper).CheckConsistency(zone string, nonCartesian bool) []Finding
func CheckZone(zone string) []Finding
```

`CheckZone` (boot and `cartcheck`) runs `CheckConsistency` over every map of
the zone (`zoneMaps`: a room its root reaches only through another zone has
a map of its own) and scopes the findings to the zone.

Scans `crawledRooms` (the `map[int]*mapNode` populated during BFS
crawl) for the zone and returns a slice of findings. `RoomGrid` is a
separate struct over a `[][][]*mapNode` 3D slice used for rendering, not
for consistency checks. The `nonCartesian` flag is sourced from the
zone's `zone-config.yaml` field `non_cartesian: true`; when set, only
`longcrossing` warnings are emitted (the hard checks are skipped).

### Exit Kind Classification

```go
func classifyKind(nominal, actual positionDelta) ExitKind
```

Exit-kind classifier used by the map snapshot/render layer (consumed in
later tasks). Compares the nominal compass delta (what the exit direction
implies) to the actual coordinate delta (the difference between the two
rooms' positions). Returns one of four `ExitKind` values:

**Note:** `CheckConsistency` does NOT call `classifyKind`. Detection of
`deltamismatch` and `longcrossing` is performed via inline delta
comparisons (`samePos`) directly inside `CheckConsistency`.

| Kind       | Meaning                                                     |
|------------|-------------------------------------------------------------|
| `normal`   | Nominal == actual (standard single-cell step)               |
| `long`     | Same direction, magnitude > 1 (multi-cell connector)        |
| `vertical` | One axis is Z only (up/down exits)                          |
| `wrap`     | Nominal and actual differ — toroidal or maze-style crossing |

**Important:** `classifyKind` uses the helper `samePos(a, b positionDelta)`,
which compares only the `x`, `y`, `z` fields. Never compare
`positionDelta` values with `==` directly — the struct also carries an
`arrow` display field that will cause false mismatches.

### Finding Kinds

| Kind            | Severity | Description                                              |
|-----------------|----------|----------------------------------------------------------|
| `collision`     | error    | Two distinct rooms share the same (x,y,z) coordinate.   |
|                 |          | Detected by grouping `crawledRooms` nodes by their       |
|                 |          | `(x,y,z)` Pos. Scanning `crawledRooms` (not `RoomGrid`)  |
|                 |          | is required because `RoomGrid` is a 3D slice: when two   |
|                 |          | rooms land on the same cell, the slice assignment keeps  |
|                 |          | only the last writer, making the collision invisible     |
|                 |          | there — every room is present in `crawledRooms`.         |
| `noreciprocal`  | error    | A spatial exit has no matching return exit in the        |
|                 |          | opposite direction, and the exit is not marked           |
|                 |          | `oneway: true`.                                          |
| `deltamismatch` | error    | Exit's compass direction does not match the actual       |
|                 |          | coordinate delta between the two rooms — a wrap exit     |
|                 |          | inside a Cartesian zone.                                 |
| `longcrossing`  | warning  | A long-connector exit's straight span passes through     |
|                 |          | another room's occupied cell. Always emitted regardless  |
|                 |          | of `non_cartesian` setting.                              |

### Exemptions

The engine automatically skips:
- **Non-compass exits** (portals, named exits): filtered via the
  `getMapNode` `mapdirection→name→skip` rule; they are not spatial edges.
- **Ephemeral/instance rooms**: checked via `rooms.IsEphemeralRoomId`.
- **`oneway: true` exits**: exempt from `noreciprocal`; still
  collision-checked.
- **`non_cartesian: true` zones**: exempt from `collision`,
  `noreciprocal`, and `deltamismatch`; their wrap exits render as
  edge stubs in the web mapper.

### Authoring Primitives

- **`oneway: true`** on an exit YAML field — marks an intentional
  one-way passage; suppresses the reciprocity check for that exit.
- **`non_cartesian: true`** in a zone's `zone-config.yaml` — marks the
  entire zone as toroidal/maze geometry; skips the three hard checks
  zone-wide.

### Cross-Zone Exits and Collisions

Since the crawl stops at the zone border (#375 phase 4), a border exit's
far room is not in the map. `CheckConsistency` reads it from its own zone's
map (`farNode`); reciprocity and the plane check always run, the delta and
long-crossing checks only when both maps sit at authored coordinates (one
frame). Collisions with other zones come from `joinedCells`: the rooms of
the maps of `WebMap` by (plane, x, y, z), one finding per cell under the
zone of its lowest room id, as one map of the connected world reported a
collision group. It is NOT every map: the jail cell 5107 (no exit reaches
it, unplaced at the origin) shares Dustwalk Road 400's cell, which master
never checked and which, in boot's panic mode, would stop the server.
`TestShippedRoutesSurviveZoneBoundedMaps` (repo root, `DOGMUD_BOOT_SMOKE`)
fails on any shipped finding; master and this tree both have 0 (2026-10-28).
`mapper.consistency_border_test.go` pins a cross-zone missing return,
delta mismatch and collision (each also reported on 8534cf6a1, none on the
tree before Task 20). `FilterFindingsToZone` still drops any finding whose
room's owning `zone:` is not the zone being checked.

## Fog-of-War Snapshot (Web Map)

### File

`mapper.snapshot.go`

### Types

```go
type SnapshotRoom struct {
    RoomId int            `json:"num"`
    X      int            `json:"x"`
    Y      int            `json:"y"`
    Z      int            `json:"z"`
    Symbol string         `json:"symbol"`
    Biome  string         `json:"biome"`
    Exits  []SnapshotExit `json:"exits"`
}

type SnapshotExit struct {
    ToRoomId int      `json:"to"`
    DX       int      `json:"dx"`
    DY       int      `json:"dy"`
    DZ       int      `json:"dz"`
    Kind     ExitKind `json:"kind"`
}
```

### Method

```go
func (r *mapper) Snapshot(visited map[int]struct{}) []SnapshotRoom
func (r *mapper) WebMap() *WebMap
func (w *WebMap) HasRoom(roomId int) bool
func (w *WebMap) Snapshot(visited map[int]struct{}) []SnapshotRoom
```

**The play client draws `WebMap`, not one zone's `Snapshot`** (#375 phase 4
Task 18). The owner's 2026-07-30 rule is that the web map shows every room
the player has explored whatever zone it is in. Before ruling 18 one map
crawled every room a compass exit reached, so its snapshot did that; now
`WebMap` gathers the same maps: the zone's map, then breadth-first the map of
each room in a map's `borderRooms` (the far rooms of its border exits,
recorded by the crawl), found with `GetMapperIfExists`, so it builds no map.
A map at crawl positions or an instance copy has its own frame: as the root
it draws alone, as a joined map it is walked through but not drawn. Its
`Snapshot` draws each visited room from its own map (once, should a stale map
also hold it) and makes an exit a full edge when the far room is a visited
room of any drawn map, as the one shared map did. Compared on shipped data
against 8534cf6a1 (every room as the current room, a fully and a seeded half
explored character): all 2,610 payloads identical, including `cz`. Cost about
master's (0.8 to 0.9 ms for the 1,209-room overworld view, both trees, one
machine, one run). The builder's zone view (`sendZoneMapFull` in
`gmcp.Build.go`) also reads `WebMap` (`RoomIds`, `Snapshot`): the zone's
rooms its root reaches through another zone, and the neighbours it dims.

Iterates `crawledRooms` and returns only rooms whose ID is present in
`visited`. For each included room, exits are also filtered: an exit is
included only if its destination room is in `visited` (fog of war — the
client never learns about rooms the player hasn't been to). Each exit's
`Kind` is set by `classifyKind(nominal, actual)` — the same classifier
used by the consistency engine.

Through `WebMap.Snapshot` this is the output consumed by the `gmcp.Zone`
module (`modules/gmcp/gmcp.Zone.go`), which builds the `Zone.Map` GMCP payload
and sends it to the web client on every room change. The web client
renderer (`RoomGridSVG` in `gmcp.js`) uses the `kind` field to route
each exit to its correct visual treatment: connector line (`normal`/
`long`), teal edge-stub with chevron (`wrap`), or ▲/▼ tick (`vertical`).

**Snapshot never loads a room.** It runs on the event loop on every room
change, over every visited room in the crawl (790 to 1,209 on the
overworld). A loaded room (`rooms.IsRoomLoaded`) is read live, including
the live-only `shop` tag. An unloaded room reads the template fields
`getMapNode` copied onto its `mapNode` at crawl time (`Title`, `Zone`,
`Biome`, `IsBank`, `IsStorage`, all `instance:"skip"` on `rooms.Room`);
its biome name still resolves through `Room.GetBiome`. Exit `ToZone`
comes from `snapshotZone`, the same live-or-node rule, then the zone the
exit itself recorded at crawl time (`nodeExit.ToZone`): a border exit's far
room is in another zone's map, so only the exit names its zone. A border
exit is a `Stub` in one zone's own `Snapshot`, and a full edge in
`WebMap.Snapshot` when the far room is visited. Before this, an
idle-unloaded room was reparsed from disk per snapshot (~180 ms for 960
cold rooms locally). `mapper.snapshot_cold_test.go` pins both the
no-load rule and the payload against a golden from the loading version.

### SnapshotExit Extended Fields

`SnapshotExit` carries additional flags that inform per-exit visual
styling on the client:

```go
type SnapshotExit struct {
    ToRoomId int      `json:"to"`
    DX       int      `json:"dx"`
    DY       int      `json:"dy"`
    DZ       int      `json:"dz"`
    Kind     ExitKind `json:"kind"`
    Locked   bool     `json:"locked,omitempty"`
    Secret   bool     `json:"secret,omitempty"`
    OneWay   bool     `json:"oneway,omitempty"`
    Gate     bool     `json:"gate,omitempty"`
    Stub     bool     `json:"stub,omitempty"`
    ToZone   string   `json:"tozone,omitempty"`
}
```

- `Locked` — exit has a key requirement (room has a lock or door key set).
- `Secret` — exit is normally hidden from plain sight.
- `OneWay` — exit is flagged `oneway: true`; no return exit expected.
- `Gate` — set when the exit's `ExitMessage != ""`; indicates a barrier
  or door with flavor text (portcullises, heavy doors, etc.).
- `Stub` — the destination room is **not** in the visited set (unvisited)
  or is in a different zone. Stub exits are now emitted by `Snapshot`
  instead of being dropped, so the client can render a visual hint that
  a passage continues beyond the fog boundary.
- `ToZone` — populated on cross-zone stub exits; contains the destination
  zone name. Allows the client to label or style zone-boundary exits
  distinctly.

**Prior behavior:** `Snapshot` dropped exits whose destination was not in
the visited set. After this change it emits them as `Stub: true` entries,
giving the client enough information to draw a "passage continues"
indicator without revealing the destination room's details.

### nodeExit.Gate

`nodeExit` (the internal per-exit node built during BFS crawl) gained a
`Gate bool` field set from `exit.ExitMessage != ""`. This propagates to
`SnapshotExit.Gate` during snapshot construction.

## Map Consumers

The mapper data is consumed by two independent rendering paths:

### (a) In-Game ASCII `map` Command

`internal/usercommands/skill.map.go` calls `GetLimitedMap` and
`GetLegend` to render a terminal-width ASCII map scaled by the player's
Perception skill. Symbol legend:

| Symbol | ASCII | Meaning |
|--------|-------|---------|
| `@`    | `@`   | You (current room) |
| `☺`    | `P`   | Party member; from map level 4, a player or a peaceful NPC |
| `☠`    | `!`   | Hostile or fighting mob, from map level 4 (`mobMapSymbol`) |
| `☹`    | `&`   | Friend (a party member's charmed mob) |
| `⚷`    | `K`   | Locked exit (`LockedSymbol`) |
| *(biome/mapsymbol)* | one letter | Room terrain glyph |

Every glyph a map draws must convert to one ASCII character
(`TestMapGlyphsConvertToOneAsciiCharacter`); the ASCII forms live in
`util.unicodeToAscii`, except the mob cell, which `mobMapSymbol` picks at
the source because `☠` is dropped elsewhere.

`map wide` (a screen-sized map) is also level 4 and up
(`mapSizeFor`, `wideMapLevel`).

Detail level (visible radius, secret/locked display) scales with
Perception. This path is text-only and has no awareness of the
`SnapshotExit` extended flags or `Zone.Map.Party`.

### (b) Web Leather Map (GMCP `Zone.Map`)

The `Zone.Map` GMCP snapshot (`Snapshot`) feeds the browser client.
The `Zone.Map` payload now includes a `Party []int` field — a list of
room IDs currently occupied by party members — enabling the client to
render party-member position markers on the map.

The web renderer (`RoomGridSVG` in
`_datafiles/html/public/static/js/gmcp.js`) presents an **antique
tooled-leather** themed map: a fixed leather-textured SVG surface holds
a nested pannable `worldSvg` containing the room grid. Connection
styling is per-exit-type:

- **Biome roads/trails/water** — line color/style derived from room biome.
- **Locked** — distinctive styling for keyed doors.
- **Secret** — rendered as a dimmed or dashed connector.
- **One-way** — directional arrow or asymmetric line weight.
- **Gate** — styled to suggest a barrier (portcullis texture or color).
- **Stairs** — ▲/▼ ticks on the room node for `vertical` exits.
- **Cross-zone stubs** — short labeled stubs at the zone boundary.
- **Fog stubs** — dim stubs for unvisited exits (stub exits in the
  snapshot).

Party markers are small figures drawn on the room node for each room ID
in `Zone.Map.party`. The current player's room is rendered with a raised
(drop-shadow) treatment to distinguish it from adjacent rooms.

Visual source of truth: `docs/superpowers/specs/2026-06-06-mapper-leather-mockups/`.

Connection-type styling and party markers are **web-only** — the ASCII
`map` command does not reflect these.
## Cross-Zone Routing (`mapper.crosszone.go`)

History: until #375 phase 4, `GetPath` resolved its mapper from
`mapperZoneCache[zone]` keyed off the **start** room and rejected a target the
start map did not hold. It now searches across zone maps (see Pathfinding), so
the fallback below fires only when `GetPath` fails. Before 2026-07-30 that made every cross-zone quest
marker inert: no destination dot (the room isn't in the current zone's
snapshot) and no next-step arrow. Six quests were affected; quest 67 crosses
four New Plymouth districts, so its guidance went dark exactly when a player
was trying to find the right one.

`NextStep` now falls back to `crossZoneHop` when `GetPath` fails and the two
rooms are in different zones. The approach is deliberately **not** a global
room-level pathfinder:

1. A coarse zone graph (`zoneGraph`) records, per zone, every outbound border
   link (#375 phase 4 Task 17; before, one per neighbouring zone):
   `zoneLink{toZone, exitRoom, exitName, destRoom, secret}`, sorted by `toZone`,
   `exitRoom`, `exitName` so the pick is the same on every boot.
2. `nextZoneHop` / `nearestZoneHop` BFS zone-to-zone and return the **first**
   leg, whose `toZone` is the next zone out of the player's current zone, even
   when the target is several zones off.
3. `hopToward` then runs the in-zone `nearestIn` to the nearest border room
   among every link from the current zone to that next zone (or returns the
   crossing exit itself if the player is already standing on one). A border
   the player cannot reach inside the zone is passed over for another.

The hot path therefore stays on the cached per-zone mapper, bounded by the
zone. The graph is built once and warmed in `PreCacheMaps` (every room is
already loaded there); `ClearCache` and a forced `GetMapper` rebuild call
`InvalidateZoneGraph()`, so the next query rebuilds it from the edited rooms
(#375 phase 4 Task 21).

Real-world shape (shipped data, re-measured 2026-10-28 by the same scan as
`buildZoneGraph`): **39 zones with outbound links, 83 links**, one per
ordered zone pair, so keeping every link (Task 17) adds none today.

**Limitation:** the in-zone leg does not leave the zone, so a border room
reachable only through another zone gives no arrow.

**The web marker still needs the room to be in the current zone's snapshot**, so
cross-zone journeys get the *arrow* but not the destination dot until the player
reaches that zone. The arrow is the part that guides.

### Why there are two zone-adjacency crawls

`modules/weather/crawler/build.go` builds its own zone graph for weather
fronts. Unification was designed and **rejected** on 2026-07-30 — `sim.Edge` is
undirected and discards the border room/exit that routing needs; `internal/`
never imports `modules/`; and the weather packages are vendored from a
standalone repo with arch tests (`TestCrawlerPackageStaysPure`) that forbid
engine imports. The full reasoning, the known behavioural differences, and the
triggers that would reopen the decision are in the header comment of
`mapper.crosszone.go`. Read that before attempting to merge them.
