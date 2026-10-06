# Rooms Package Context

## Overview
The `internal/rooms` package is the core world management system for GoMud, handling all aspects of game world rooms, zones, and spatial relationships. It provides comprehensive room functionality including dynamic loading/unloading, ephemeral room creation, biome management, and complex room state tracking.

## Key Components

### Core Room Structure (`rooms.go`)
- **Room struct**: The main room entity containing all room data and state
- **Room properties**: Title, description, exits, items, NPCs, players, and environmental settings
- **Special room types**: Banks, storage rooms, character creation rooms, PvP areas
- **Dynamic state**: Player/mob tracking, visitor history, temporary data storage
- **Room features**: Containers, signs, skill training areas, spawn points
- **Room text by channel**: `SendText` is the audio channel and is never
  sight-gated. `SendTextVisual` gates each recipient by
  `messaging.CanSeeClearly` / `CanSeeShapes` and anonymizes for infrared-only
  observers. `SendTextVisualWithAudio` gives the unsighted an audio variant.
  `VisualSnapshot()` returns a `VisualSnapshot` (user id to
  `messaging.SightDecision`) of what every player in the room can see now;
  `SendTextVisualToSnapshot(snap, cat, txt, names, excludeUserIds...)`
  delivers a line to exactly the players in the room now who were in the
  snapshot, each at their recorded decision, names hidden at shapes. Owner
  rule (2026-10-05): a line announcing a change to the room's light lands
  with the state everyone was in BEFORE the change, so take the snapshot
  first, make the change, then send. Lighting plan 5d uses it for a
  darkness's start line and its equip lines; #220 added `hood` and the end
  line of a light or darkness that runs out or is cancelled. Those end lines
  go out at the next prune, after the record has stopped counting, so the
  snapshot is kept in this package's end-line store
  (`end_line_snapshot.go`): `KeepEndLineSnapshot(rec, roomId, snap)` keeps
  one for a record (the hooks round ticks call it just before the Trigger the
  record runs out on, with `Room.EndLineRoundSnapshot()`, the one snapshot
  of that room per round, taken before any light or darkness in it has run
  out that round and shared by every record that runs out there that round),
  `KeepEndLineSnapshotsBeforeRemoval(c, conditionId)`
  snapshots the holder's room for a record about to be removed early
  (`usercommands` cancel calls it), `TakeEndLineSnapshot(rec, roomId)` hands
  it to the prune (nil when none was kept or the holder is now in another
  room), and `ClearEndLineSnapshots()` drops the rest (and the round's room
  snapshots) at the end of the prune. A watcher's decision is frozen from
  the snapshot until the line is sent. The store's doc comment lists the
  removal paths that keep no snapshot and why.
  The two as-lit senders this replaced, which judged every reader against a
  stand-in lit room, are deleted. All the
  visual senders share one per-recipient body, `deliverVisual`.
  `SendTextVisualHidingNames` is `SendTextVisual` for a line that names an
  event's parties: a shapes-only observer reads each name as "a figure". It is
  the observer half of `messaging.SendTrio`; `ParticipantSight(userId)` is the
  other half.
  `SendTextHidingNames(cat, txt, names, hide messaging.NameHider,
  excludeUserIds...)` is an AUDIO-channel sender for an authored line that
  names who made it (a rally, a howl, NPC speech): heard by everyone whatever
  they can see, with each of `names` rewritten per listener by `hide` at that
  listener's `messaging.ParticipantSight`. `hide` is `messaging.HideNames` for
  a sound ("Something lets out a roar!") and `messaging.HideSpeakerNames` for
  speech ("Someone says, ..."). There is no lit-room shortcut, which is how
  the deleted `mobcommands.sendAudioRoomText` named a speaker to a blinded
  listener; never deafen-filtered, because NPC lines are authored content
  (owner ruling 6, sight gates slice 5b).
  `SendCommunicationHidingNames(cat, txt, names, excludeUserIds...)` is a
  player's speech to the room: every listener hears the words, the speaker's
  name hidden by `messaging.HideSpeakerNames`, and every message marked
  `IsCommunication` so the Deafened moderation filter still applies.
  `SendVisualCommunicationHidingNames(cat, txt, names, excludeUserIds...)` is
  `SendTextVisualHidingNames` for a player's free-form text that is seen
  rather than heard (a free-form emote): the same sight gate and name hiding,
  also marked `IsCommunication` so a deafened player is still spared. Only
  the shared bodies in `internal/actions` call these three
  (`speech_wrapper_guard_test.go`, repo root).
  `SendTextCommunication` keeps two callers now: `actions.Shout`'s adjacent-
  room line (a player shouting next door) and the AI companion's `ask` line
  (`modules/aicompanion/listeners.go`); NPC speech never uses it, going
  through `SendTextHidingNames` unfiltered instead so a moderated player
  still hears quest content.
- **Naming a creature**: `FindByNameSeenBy(viewer, name, flags...)` skips
  every creature `viewer` does not perceive (`characters.Character.Perceives`)
  before matching, so a hidden creature cannot be named and does not count
  toward `2.name`. `FindByName` is the unfiltered form for staff tools and mob
  callers. The "Also here" listing (`roomdetails.go`) reads the same
  `Perceives` rule, with no pet requirement, and then the viewer's
  `messaging.ParticipantSight` in the room (lighting plan 5c): at shapes every
  entry, players, mobs and pet alike, is `messaging.UnseenFigure(SightShapes)`
  with no adjective, all in `VisiblePlayers` so the template's mob color
  cannot sort them; at none both lists are empty.

### Room Management System (`roommanager.go`)
- **RoomManager**: Singleton manager for all room operations and caching
- **Memory management**: Automatic loading/unloading of rooms based on occupancy
- **Zone management**: Organizing rooms into logical zones with metadata
- **File system integration**: Room data persistence and template loading
- **Cache optimization**: Room file path caching and efficient lookups

### Room Details and Presentation (`roomdetails.go`)
- **RoomTemplateDetails**: Rich room information for client rendering
- **Dynamic content**: Visible players, mobs, corpses, and exits
- **Environmental context**: Day/night cycles, lighting, biome effects
- **User-specific views**: Personalized room information based on character state
- **Room alerts**: Special notifications for banks, training, storage, etc.

### Biome System (`biomes.go`)
- **BiomeInfo**: Environmental definitions affecting room behavior
- **Lighting system**: Dark areas, lit areas, and visibility mechanics
- **Environmental effects**: Symbols, descriptions, and special properties
- **Item requirements**: Biomes that require specific items to navigate safely
- **Dynamic loading**: File-based biome definitions with validation

### Room Lighting (`lighting.go`, `light_trim.go`, graded scale, plans 1 through 5a of the lighting arc)

`Room.LightLevel() int` is the light accessor every consumer reads. It
reports light on a continuous -100 to 100 scale, composed from three kinds
of term on one logarithmic operator (`internal/lightscale.Combine`):

1. **The sky**: `internal/gametime.CelestialLight()` (sun plus moons, one
   value for the whole world per round), attenuated by this room's sky
   fraction and then by `mutatorSkyFilter()`, the product of every active
   mutator's `SkyLight` fraction (1 when clear; weather multiplies it down).
2. **The room's own lamp**, if it has one.
3. **Every light anyone present carries, one term each** (lighting plan 5a).
   `carriedTerms(exclude)` makes one pass over `r.mobs` and `r.players`,
   reading each bearer's `Conditions.LightAndDarknessSources()` through
   `Condition.LightNow`, so a hooded or trimmed-off source adds nothing and
   two torches are one doubling step brighter than one. Before 5a any light
   lifted the room to a flat `DimBelow`; the `FindHasLight` find flag that
   test used is DELETED.
4. **Every darkness anyone present carries** (lighting plan 5d), from the
   same pass. Darknesses combine among themselves by the same halving rule
   (two of 50 take 58) and the combined darkness is SUBTRACTED from the
   combined light (Absent light reads 0), so a room can read below 0, down
   to the -100 clamp. An unlit room with no darkness is still 0.

The composition is layered so a trim can leave one source out: `composeLight`
calls `composeLightExcluding(cfg, celestial, skyFilter, exclude)`, which calls
`composeWith(cfg, celestial, skyFilter, carried, dark)` (the pure core a test
can feed carried light and darkness terms without users or mobs).

**Trimming (`light_trim.go`, plan 5a; darkness plan 5d).** `(*Room).TrimLightFor(c)`
trims every adjustable, unhooded light AND darkness record `c` holds to `c`'s
own eyes. A light takes the least cut from full strength that keeps the room
under `messaging.LightTrimTarget(c.NightVisionStrength(), cfg.DazzleAbove)`,
solved by `lightscale.Trim` against `LightTerms.Light` with the target raised
by `LightTerms.Dark` (a light in a darkened room may run brighter before it
dazzles, ruling D3). A darkness takes the least cut that keeps the room at or
above `messaging.DarknessTrimTarget(strength, InfraReach(), cfg.BlindBelow)`,
solved by `lightscale.TrimDarkness` against the light and the other darkness
(ruling D2). A fresh cast or equip is full strength until the next move.
Sources trim in held order, whatever their kind, after a pre-pass sets them
all off. It is the ONLY
trim trigger: `MoveToRoom` (`roommanager.go`) and `AddMob` (`rooms.go`) call it
once the mover is in the room. Nobody already present re-trims when someone
arrives, and nothing re-trims on a round tick, so a room that brightens or
darkens around a standing bearer leaves their light as it was.

Plan 4 deleted the old `-2..2` `LightMod` bridge entirely; a mutator now
only ever dims the sky, never adds a light of its own. Every weather
mutator is `outdooronly`, and `ActiveMutators` skips those in an indoor
biome, so weather never dims a roofed room. Of the indoor biomes only
`fort` and `interior` have any sky at all, plus two `dungeon` rooms that set
their own (`stillwater/5106` and `thornwall_city/5105`, `skylight: 0.1`), so
they are the only rooms this choice affects.

`Room.IsLit() bool` reports whether a normal observer can see anything at
all here (`LightLevel() >= cfg.BlindBelow`). It reads `configs.Lighting`
via `configs.GetLightingConfig()` rather than `GetBalanceConfig()` on
purpose: sixteen hand-rolled call sites used to copy the whole 424-field
`Balance` struct to check one threshold, and `IsLit` collapsed all of them
onto itself.

**`(*Room) LightTerms() LightTerms`**, added by lighting plan 3d, reports the
same light broken into the terms `LightLevel` combines, for a caller that
needs to know WHY the light is what it is: `Level` (identical to
`LightLevel()`, both come from the shared `composeLight`), `Sky` (the sky
term after fraction and the weather filter, `lightscale.Absent()` when the
room has no sky), `SkyFilter` (the product of the active mutators'
`SkyLight` fractions, 1 when clear), `Lamp`/`HasLamp`, `Carried`, and (plan
5a) `Raw`. Since lighting plan 5d `Raw` is the NET light `Level` rounds
(`Light` read as 0 when Absent, minus `Dark`), and three fields carry the
parts: `Light` (the combined light, `Absent` when nothing lights the room;
the light trim solves against it), `Dark` (the combined darkness, `Absent`
when nobody carries one) and `Darkened` (someone carries a darkness, as
`Carried` means someone carries a light).
`internal/lightnotice` is the one consumer: it compares two `LightTerms`
snapshots to name which term moved and so which cause to report for a band
change. `LightTerms` and `LightLevel` share one computation
(`composeLight`), so a caller reading both never risks the two disagreeing.

**Sky fraction and lamp are both `*float64`/`*int` POINTERS, on both
`Room` and `BiomeInfo`, because zero is meaningful for both.** A cave's sky
fraction is genuinely `0` (no sky reaches it at all) and must be
distinguishable from "unset," which reads as a fully open sky (`1.0`). The
YAML keys are `skylight` and `lamp`:

- `BiomeInfo.SkyLight *float64` / `BiomeInfo.Lamp *int` are the biome
  default, read through `SkyLightFraction()` / `LampValue()` (`biomes.go`).
- `Room.SkyLight *float64` / `Room.Lamp *int` (`rooms.go`) **override** the
  room's biome when set. Both carry `instance:"skip"`, matching `Biome`, so
  an ephemeral instance of a room inherits its template's lighting rather
  than persisting its own.

  Plan 3b gave the world a wider biome vocabulary instead of reaching for
  this override in most of the cases that once seemed to need it: a brick
  sewer vault, a wrecked ship's interior and a web-choked lair each got
  their own biome (`sewer`, `interior`, `spiderweb`) rather than a
  room-level number. **The `lamp` override ships on twenty-seven rooms**
  (grep `^lamp:` under `_datafiles/world/dogmud/rooms`): the eleven shop
  rooms of the merchants slice described below, and sixteen older ones.
  The sixteen older rooms are the three-room Planar
  Oasis (`instance_planar_oasis/500{3,4,5}.yaml`), which sets `lamp: 38`
  against `ether`'s biome lamp of `60` because its room text reads "Shapes
  move in the heat haze, some are mirages, some are not," so a fully lit
  oasis would contradict its own description; ten rooms from plan 3c-1
  carrying the same `lamp: 38` against a biome that otherwise ships none:
  seven `dungeon` rooms in New Plymouth's buried Old Quarter
  (`new_plymouth_old_quarter/60{21,26,27,30,31,33,35}.yaml`), one `sewer`
  room under the docks (`new_plymouth_docks/5509.yaml`, the chandlery's
  lamp-glow reaching down the stair), and two `road` rooms on the
  riverside track (`new_plymouth_outskirts/54{78,80}.yaml`, security lamps
  where the track meets the wall and the docks); and three more rooms from
  plan 3c-2, each carrying `lamp: 38` where its own text names something
  other than a burning lamp: 6250 The Greenford Road
  (`the_confluence/6250.yaml`, `road` biome, a security lamp just outside
  the Confluence's East Gate, the owner's wall-lamp ruling), 5000 The Rift
  Chamber (`thornwall_city/5000.yaml`, `dungeon` biome, pulsing wall
  runes) and 6443 The Chrysalis Workshop (`stillwater/6443.yaml`,
  `dungeon` biome, pale green shard glow), the last two by the owner's
  3c-2 ruling that a steady glow someone works by counts as light. Each is
  a room whose own text names a light source; every other room in the Old
  Quarter, the sewer pocket and the two new `dungeon` rooms' neighbours is
  dark (all three biomes ship `skylight: 0.0`). The riverside track is the
  exception: `road` and `river` both ship `skylight: 1.0`, so 5478 and
  5480 (the lamped rooms) and 5479 the Ford (plain `river`, no lamp) are
  all open to the sky by day; the lamp on 5478 and 5480 only matters after
  dark.

  **The `skylight` override ships on thirteen rooms** (grep `^skylight:`
  under `_datafiles/world/dogmud/rooms`). The first two, new in plan 3c-2,
  are the holding cells beneath Thornwall's guard barracks
  (`thornwall_city/5105.yaml`) and Stillwater's constabulary
  (`stillwater/5106.yaml`), both `dungeon` biome (`skylight: 0.0` by
  default) set to `skylight: 0.1`. Each cell's own text names a window slit
  admitting a bar of daylight; the owner's ruling was a room-level fraction
  rather than new code, since the existing sky-tracking math already reads
  it as shapes by day and dark at night.

  **Shop rooms inside buildings (merchants slice, 2026-09-30).** Eleven
  shops sit in inns, taverns, mills, a smithy, a chandlery and a trading
  post whose rooms are authored with an outdoor biome, so the lighting 5b
  shop sight gate refused their keepers at night. Each carries both
  overrides at the `interior` biome's values, `skylight: 0.15` and
  `lamp: 50`, with `biome:` left alone because the biome also drives
  weather, map symbol and movement cost: 423, 424 (`watchers_crossing`),
  4045 (`north_road`), 5245 (`pothole_coulee`), 5378 (`north_road_north`),
  5448, 5449 (`kingsbarrow_vale`), 6114, 6116, 6252 (`the_confluence`) and
  6280 (`greenford`). Street and stall keepers carry an Oil Lantern
  instead, which is mob content, not a room override. The root guard
  `shop_night_trade_guard_test.go` asserts every shop keeper can trade at
  night while awake, so a new dark shop fails the build.

  `fort`'s old granularity gap, sharing one sky fraction between
  an open yard and a buried vault, is gone: plan 3c-1 moved fort's open
  portions (the burst-open watch room, the roofless shrine, the cracked
  dome) out to `ruins`, so the eight rooms still biomed `fort` are all
  fully enclosed. One of them, 5245 The Coulee Smithy, carries the shop
  room override above, since `fort` ships no lamp.

### The shipped biome vocabulary (plan 3b)

Every room's `biome:` field names one of the biomes below (`biomes.go`
loads each `_datafiles/world/dogmud/biomes/*.yaml` file). `skylight` is the
sky attenuation fraction `SkyLightFraction()` applies (`0.0` blocks the sky
entirely, `1.0` lets it straight through); `lamp`, where a biome sets one,
is a flat light floor from `LampValue()`, independent of the sky. A biome
with no listed lamp has none: its light is sky alone (plus whatever a
carried light or the room adds).

| Biome | skylight | lamp | Meaning |
|---|---|---|---|
| `sewer` | `0.0` | none | A brick vault under a city; no sky and no fixture of its own. Ships with New Plymouth Sewers |
| `interior` | `0.15` | `50` | A built structure with its own light: a house, a wrecked ship's cabins, a temple's interior rooms, a crafting hall, an arena's vaulted chambers |
| `dense_forest` | `0.25` | none | Canopy thick enough to matter, split out of `forest`'s rooms |
| `plains` | `1.0` | none | Open grassland, fully open sky |
| `river` | `1.0` | none | Flowing water, fully open sky |
| `ether` | `0.0` | `60` | Outside the world; time-invariant. Character creation, the shadow realm, the planar oasis |
| `spiderweb` | `0.0` | `45` | A web-choked lair. Declared before plan 3b but held zero rooms until this plan gave it the Foldweave |
| `city_thoroughfare` | `0.95` | `52` | A city's main streets, squares, markets and gates. Lamps hold it in the faces band at any hour, day or night. Added by plan 3c-1 |
| `city_backstreet` | `0.95` | `35` | A city's lanes, alleys, courts and yards off the main ways. No lamp reaches them, so a normal eye reads shapes, not faces, after dark. Added by plan 3c-1 |
| `ruins` | `0.75` | none | A roofless building: takes weather and sky like open ground, a little shaded by whatever walls still stand, dark at night. `movementcost: 1.0` for the rubble underfoot. Added by plan 3c-1 |

**`city` is gone from the dogmud world.** Plan 3c-1 split New Plymouth's
156 `city` rooms and plan 3c-2 (in two PRs, 3c-2a and 3c-2b) sorted the
remaining 268 across `the_confluence`, `greenford`, `thornwall_city`,
`stillwater`, `hartcharn`, `kilnreach_works` and `pothole_coulee` into
`city_thoroughfare`, `city_backstreet`, `interior`, `dungeon`, `road` or
`ruins`. `city_thoroughfare` and `city_backstreet` are the vocabulary now:
a thoroughfare's lamp holds it in the faces band at any hour, a backstreet
has none and reads shapes after dark. `biomes/city.yaml`,
`weather/climate/city.yaml` and the `city:` emote pool key are all deleted
from `_datafiles/world/dogmud`, and all 12 zone-configs that defaulted an
unauthored room to `city` now default to `city_backstreet`.
`internal/rooms/no_city_biome_test.go` guards the deletion: it fails if
any dogmud room file still reads `biome: city` or any zone-config still
reads `defaultbiome: city`. The upstream `default` world is untouched and
keeps its own `city` biome and climate key; `default` ships no weather
data folder of its own, so it falls back to the built-in `"city"` profile
in `modules/weather/sim/climate.go`, which plan 3c-2 deliberately left in
place for exactly that reason.

**`house` is deleted.** Plan 3b folded its rooms into `interior` along with
every other room that was really an indoor space wearing an outdoor biome:
the Crash Site's interior rooms (previously `cave`), the temple's interior
rooms and `new_plymouth_crafting`'s rooms (both previously `city`), and
`instance_arena`'s rooms (previously no biome at all, falling through to
the synthetic default). `house.yaml` and its climate file no longer exist;
a reference to either describes deleted content.

**`GetVisibility()` and `legacyVisibility()` are GONE.** Plan 1 shipped
`GetVisibility` as a call-through to the old three-value (0/1/2) model, kept
as `legacyVisibility` while `LightLevel` bridged onto it with three fixed
constants (`LightDark`/`LightRoomOnly`/`LightFull`). Plan 3a deleted all of
it: `LightLevel` now composes light from the sky, lamp and carried
sources directly, with no bridge and no three-value model underneath. If
you find a reference to `GetVisibility`, `legacyVisibility`,
`LightDark`, `LightRoomOnly` or `LightFull`, it describes deleted code.
Every consumer, including `internal/messaging`'s `RoomVisibility`
interface, reads `LightLevel()` (or `IsLit()` for a plain lit/dark
question).

### Spawn Management (`spawninfo.go`)
- **SpawnInfo**: Comprehensive mob and item spawning system
- **Spawn configuration**: Mob templates, items, gold, and containers
- **Respawn mechanics**: Time-based respawning with configurable rates
- **Spawn customization**: Level modifications, hostility, scripting overrides
- **Quest integration**: Quest flags and condition assignments for spawned entities

### Container System (`container.go`)
- **Container**: In-room storage with locking mechanisms
- **Item management**: Adding, removing, and searching container contents
- **Lock system**: Difficulty-based locks requiring skills to open
- **Recipe system**: Crafting recipes that trigger when ingredients are present
- **Temporary containers**: Time-limited containers that despawn automatically
- **Hidden containers**: Optional `Hidden` bool field hides container until discovered via search
- **Discovery tracking**: Per-player tracking of which containers have been discovered

### Ephemeral Rooms (`ephemeral.go`)
- **Dynamic room creation**: Runtime creation of temporary room copies
- **Chunk management**: Efficient allocation of ephemeral room ID ranges
- **Memory optimization**: Automatic cleanup when rooms are no longer needed
- **Zone duplication**: Creating temporary copies of entire zones
- **ID mapping**: Tracking relationships between original and ephemeral rooms

### Zone Instance Options (`instances.go`)

`CreateZoneInstance` (legacy, existing callers unchanged) now delegates to:

```go
func CreateZoneInstanceWithOpts(
    zoneName      string,
    goldPaid      int,
    ownerUserId   int,
    authorizedUsers []int,
    overworldRoomId int,
    opts          ZoneInstanceOpts,
) (int, bool)
```

- Returns the ephemeral entry room ID and `true` on success.
- `ZoneInstanceOpts{SuppressReturnPortal bool}` — when `true`, the
  auto-added "return portal" exit is **not** created. Use this for
  confinement instances (e.g. instanced jail cells) where the occupant
  must not be able to walk out; lifetime is controlled by explicit
  teardown rather than portal expiry.
- `CreateZoneInstance(zoneName, goldPaid, ownerUserId, overworldRoomId)`
  passes a zero-value `ZoneInstanceOpts` (i.e. portal created as before),
  so all existing callers are unaffected.

**Confinement zones and `CheckPortalTimers`:**
A zone whose YAML sets `portal_duration: none` is **skipped** by
`CheckPortalTimers` — no TTL eviction and no "portal collapsing" warning
messages. This is the explicit "no-TTL" sentinel. Note that an **empty**
`portal_duration` field is auto-filled to `"30 real minutes"` by
`ZoneConfig.Validate()`, so omitting the field does NOT disable TTL —
you must set it to `none` explicitly. Such zones must be torn down
explicitly (e.g. via `TryEphemeralCleanup` called from the owning
subsystem's release/despawn path). The `instance_jail_cell` zone template
uses this pattern: its lifetime is owned entirely by `internal/justice`
(arrest creates, release or player-despawn destroys).

## Hidden Object Discovery System

### Overview
Rooms can contain hidden objects (containers, nouns) that are invisible until players discover them via the `search` skill. This system supports world-building secrets, optional exploration, and discovery-based puzzle elements.

### HiddenNoun Structure
```go
type HiddenNoun struct {
    Description       string // What `look <noun>` shows after discovery
    HiddenDescription string // Text appended to room description for discoverers
}
```

**Field meanings:**
- `Description` — Rich text shown when player runs `look <noun>` after finding it
- `HiddenDescription` — Flavor text appended to the room description when a discoverer enters the room (e.g., "You notice strange scratchmarks on the wall here.")
- No formal parent link; references to parent objects are pure prose (e.g., "gouged into the wooden wall")
- Hidden nouns are stored on `Room.HiddenNouns map[string]*HiddenNoun`
- Marked `instance:"skip"` in the YAML schema — always loaded from template, never overridden by instance saves

### Container Hidden Field
```go
type Container struct {
    // ... other fields ...
    Hidden bool // If true, container is invisible until discovered
}
```

**Behavior:**
- When `Hidden` is `true`, the container doesn't appear in room descriptions or `look` output
- After discovery via `search`, all players see the container in room details
- Discovery is tracked per-player in the character's discovery map
- The container noun in the room description should be subtly highlighted with `<ansi fg="itemname">noun</ansi>` for discoverability hints

### Authoring Convention
When writing hidden noun descriptions:
1. Reference parent objects in prose, not via formal links — e.g., "You see strange markings gouged into the **wooden wall**" rather than `parent: wall`
2. Keep `description` focused on the hidden object itself
3. Keep `hidden_description` brief (1-2 sentences) for room ambiance
4. Template-driven design: hidden nouns come from `rooms/zone/roomid.yaml`, not instance saves

## Key Features

### Dynamic World Management
- **Memory efficiency**: Rooms load/unload based on player presence
- **Visitor tracking**: History of who has visited rooms and when
- **State persistence**: Automatic saving of room changes and contents
- **Zone organization**: Logical grouping of related rooms

### Environmental Systems
- **Biome integration**: Environmental effects on room behavior and appearance
- **Day/night cycles**: Time-based lighting and atmospheric changes
- **Weather integration**: Biome-based weather effects and descriptions
- **Lighting mechanics**: Dark rooms, light sources, and visibility

### Interactive Elements
- **Containers**: Lockable storage with crafting recipe support
- **Signs**: Player-created messages and room annotations
- **Skill training**: Designated areas for character skill development
- **Special services**: Banking, storage, and character management rooms

### Spawn and Population
- **Flexible spawning**: Mobs, items, and gold with complex configuration
- **Respawn timing**: Configurable respawn rates and conditions
- **Population limits**: Preventing overcrowding through spawn management
- **Per-mob stat pools**: Mob difficulty set via `statpool` in mob YAML or per-spawn `statpool`/`statpoolmod` in room spawn info (zone-level autoscaling was removed in Phase 21)

### Performance Optimization
- **Chunk-based ephemeral rooms**: Efficient temporary room management
- **Lazy loading**: Rooms load only when needed
- **Memory cleanup**: Automatic removal of unused rooms and data
- **Cache management**: File path caching and lookup optimization

## Files

| File | Purpose |
|------|---------|
| `rooms.go` | The `Room` type and its core behaviour |
| `roommanager.go` | The room registry, load/unload, and lookup |
| `end_line_snapshot.go` | The end-line snapshot store (#220): a `VisualSnapshot` kept per light or darkness record that ran out or was cancelled, for its end line at the prune |
| `lighting.go` | `Room.LightLevel()`, `Room.IsLit()`, `Room.LightTerms()` (plan 3d), and the sky/lamp/mutator/carried-light and carried-darkness composition (`composeLight`, `composeLightExcluding`, `composeWith`, `carriedTerms`; darkness plan 5d) |
| `light_trim.go` | `Room.TrimLightFor` (lighting plan 5a, darkness 5d): trims an arrival's adjustable lights and darknesses to their eyes; called from `MoveToRoom` and `AddMob` only |
| `save_and_load.go` | Room YAML + instance-save persistence, `restoreSkipTaggedFields` |
| `prose_wrap.go` | Re-folds long prose into wrapped `>` block scalars on template save |
| `roomdetails.go` | Assembled per-look detail payload |
| `zoneconfig.go` | Per-zone `zone-config.yaml` (including `non_cartesian`) |
| `zone_lifecycle.go` | Zone create/delete lifecycle |
| `zone_rename.go` | Zone renaming and the reference rewrites it implies |
| `zone_activity.go` | Per-zone activity/occupancy tracking |
| `planes.go` / `instance_planes.go` | Plane coordinate handling |
| `placement.go` | Coordinate placement helpers |
| `adjacency.go` | Room adjacency queries |
| `biomes.go` | Biome definitions and lookup |
| `exit`-adjacent: `sign.go` | Room signs |
| `container.go` | Room containers |
| `corpse.go` / `corpse_roundrobin.go` | Corpses and fair-share corpse looting |
| `baubles_untaken.go` | Found baubles left lying untaken for `BaubleUntakenHours` (24) vanish: `removeUntakenBaubles`, run from `RoundTick` (rooms with players) and `Prepare` (before a visitor sees the floor); records `baubles.MarkVanished` |
| `spawninfo.go` / `spawninfo_validate.go` | Room spawn lists and their validation |
| `instances.go` / `ephemeral.go` | Instanced and ephemeral rooms |
| `cubegen.go` | Generated cube/maze room structures |
| `memory.go` | Memory reporting for the admin report |
| `test_helpers.go` | Test fixtures |

### An indoor biome must be classified for weather prose

`BiomeInfo.Indoor` (`biomes.go`) is read by the weather module to pick a prose
class, not just to gate outdoor-only mutators. A biome with `indoor: true`
must appear in exactly one of the two classification maps in
`modules/weather/content/emotes.go` (`undergroundBiomes` or
`surfaceIndoorBiomes`), or it silently serves prose about roofs and
windowpanes inside a cave. `modules/weather/content/biome_coupling_test.go`
fails the build on a miss, but it lives in that package, so a rooms-only test
run will not catch it. See `modules/weather/content/context.md` for the full
three-class design.

### Prose folding on template save

`SaveRoomTemplate` marshals through `marshalRoomTemplate` (`prose_wrap.go`), not
`yaml.Marshal` directly. Authored rooms wrap prose in folded (`>`) block
scalars; loading one joins its lines with spaces, so the in-memory value is a
single long line and yaml.v2 re-emits it as a literal (`|`) block holding one
enormous line. `marshalRoomTemplate` re-folds `description`, `nouns`,
`hidden_nouns` and `idlemessages` at 78 columns.

The binding constraint is round-trip fidelity: `load(save(room))` must return
the prose byte for byte. Only a folded scalar does that, and only with the right
chomping (`>` when the value carries a trailing newline, `>-` when it does not).
The code refuses to fold anything it cannot prove safe (interior newlines, runs
of spaces, tabs, leading/trailing spaces, tokens wider than the line) and
verifies the finished document by parsing it back, falling through to plain
yaml.v2 output on any doubt. It deliberately does NOT switch the whole marshal
to yaml.v3, which would re-indent every sequence in all 1386 room files.

### Instance saves vs. `instance:"skip"`

Room instance data in `rooms.instances/` **overlays** the YAML template on
load, so a stale instance save shadows template edits. The exception is fields
tagged `instance:"skip"`: `SaveRoomInstance` does not write them, and
`restoreSkipTaggedFields` (`save_and_load.go`) copies them back from the
template after the overlay is applied.

**`Room.SpawnInfo` is in that skip category** — a spawn-list edit takes effect
on the next room load with no wipe needed. Check the struct tag before assuming
a field is shadowed.

### Instance saves are two-phase (chunk 3.6b-1)

`SaveRoomInstance` no longer writes the file itself. It is now the synchronous
composition of two halves, and the split is what lets autosave stop freezing the
world:

- **`PrepareInstanceWrite(r Room) (savequeue.PendingWrite, error)`** does
  everything that reads live state: load the template, run the reflection diff,
  marshal. Returns immutable BYTES. Caller must hold the world lock.
- **`savequeue.Commit(p)`** does the durable write, or the delete when
  `p.Data == nil`. Touches nothing but the bytes and the path, so it can happen
  later, on a different tick.

3.6a measured the write as 95.3% of a dirty room's save cost, which is why the
split is worth having: only prepare stays under the lock. Measured after:
1000 fully-dirty rooms went from 5980 ms to 120 ms of lock-held cost.

`SaveRoomInstance` still exists and still means "save this room NOW and tell me
if it failed", because unload, the builder, shutdown and copyover all need that.

**A prepared write must be CANCELLED by anything that takes ownership of the
file** (guard G2), and that set is larger than "everything that calls
`SaveRoomInstance`". These three delete room data directly and each cancels:

| Path | Why it bypasses the save function |
|---|---|
| `ClearRoomCache` | drops the room from memory with no save at all |
| `DeleteRoomTemplate` | `os.Remove`s the template |
| `DeleteZone` | `os.RemoveAll`s `rooms.instances/<zone>/` wholesale |

`DeleteZone` cancels from its `doomed` list BEFORE the removal, for the same
reason that list exists: once the directories are gone, `LoadRoomTemplate` reads
from disk and returns nil for every one of those rooms.

**`LoadRoomTemplate` is uncached** — a disk read and YAML parse per call, and
`PrepareInstanceWrite` calls it per room per autosave cycle. That is roughly
64% of what prepare costs. Template caching is a known separate slice; until it
lands, prepare cost tracks file I/O rather than CPU.

### `Room.DefusedExits` — persisting a disarm without shadowing

`Room.Exits` is skip-tagged, which meant a player disarming an exit lock trap
saw the trap return on the next restart (`Room.Containers` is *not*
skip-tagged, so the container branch of the same `defuse` command persisted
correctly — two branches of one command with opposite guarantees).

`DefusedExits []string` is the narrow escape hatch: a list of exit **names**
only, not skip-tagged, so it round-trips through the instance save.
`(*Room).MarkExitTrapDefused(name)` clears the live trap and records the name;
`(*Room).applyDefusedExits()` re-clears them in `LoadRoomInstance` **after**
`restoreSkipTaggedFields` has rebuilt `Exits` from the template.

This cannot reintroduce shadowing: every exit property — destination, lock
difficulty, exit message, oneway/secret — is still sourced wholly from the
template on each load. The instance file cannot add, remove or redirect an
exit; its only power is to clear `Lock.TrapConditionIds` on an exit the template
already defines, and a name that no longer matches an authored exit is a
silent no-op. Do **not** "simplify" this by removing the `instance:"skip"` tag
from `Exits`.

## Dependencies
- `internal/characters`: Character and mob management
- `internal/items`: Item system integration
- `internal/mobs`: NPC spawning and management
- `internal/exit`: Room connection and movement system
- `internal/gametime`: Time-based mechanics and scheduling
- `internal/mutators`: Room effect modifiers
- `internal/conditions`: Status effects in rooms
- `internal/configs`: Configuration management
- `internal/lightscale`: The graded light scale's combine/attenuate arithmetic
- `internal/fileloader`: Data file loading system
- `internal/baubles`: `UntakenLimit` and `MarkVanished` for the untaken sweep (baubles imports only configs, items, mudlog and util, so there is no cycle)

## Usage Patterns
- Room loading through manager functions with automatic caching
- Player/mob tracking through room occupancy methods
- Dynamic content through spawn system and container management
- Environmental effects through biome integration
- Temporary content through ephemeral room system

## Testing
Comprehensive test coverage in `*_test.go` files covering:
- Room loading and caching mechanisms
- Spawn system functionality
- Container and lock mechanics
- Ephemeral room creation and cleanup
- Visitor tracking and room state management

## Special Considerations
- **Memory management**: Rooms automatically unload when empty to conserve memory
- **Ephemeral limits**: Maximum of 100 chunks with 250 rooms each for temporary content
- **Thread safety**: Room operations are designed for concurrent access
- **Data persistence**: Room changes are automatically saved to maintain world state

This package serves as the foundation for the entire game world, providing a rich and dynamic environment system that supports complex gameplay mechanics while maintaining optimal performance through intelligent memory management.

## WalkItems, LoadedRooms, and untaken finds on load

`(*Room).WalkItems` (walk_items.go) walks the floor, the stash, every
container, every corpse (the dead character's gear and its loot) and the
sealed crate. `LoadedRooms()` returns every room in memory, ephemeral ones
included; the caller holds the mud lock. `LoadRoomInstance` removes finds
left untaken past `baubles.UntakenLimit()` from the floor as soon as a room
is loaded from its instance file, because the bauble sweep no longer counts
floor finds that old and may prune their records. At boot,
`factions.ValidateHoldingCells` loads some rooms before `baubles.Load`; a
find removed then is not marked vanished (no record is loaded yet) and the
sweep prunes its record later as lost.
## Fixtures and the item index (lighting 5e)

- **Composition.** `composeLightExcluding` reads the room's fixture outputs
  from `internal/itemlight` (`Terms(r.RoomId)`) and composes through
  `composeWithFixtures(cfg, celestial, skyFilter, carried, dark,
  fixtureLight, fixtureDark)`; `composeWith` is the same with no fixtures,
  so every older test reads the same terms. A lit light fixture is one term
  in the light combine, a darkness fixture one in the darkness combine.
  Fixtures never trim; a carried adjustable light trims against them.
- **`LightTerms.Fixture`** is the combine of the lit light fixtures (Absent
  when none) and never sets `Carried`; **`LightTerms.CarriedLight`** is the
  combine of carried light alone (Absent when none). `Darkened` still means a
  CARRIED darkness; a darkness fixture moves `Dark`. `internal/lightnotice`
  reads all three to name a cause.
- **The item index.** A room joins `items`' holder index when a treed item
  lands on its floor (`AddItem`, Prepare's spawn append), when it loads into
  memory holding one (`addRoomToMemory`), and at boot through
  `IndexTreedFloors()` for rooms loaded before item specs. It leaves on
  unload (`removeRoomFromMemory`, which also clears its fixture outputs).
  `RemoveItem` clears the removed item's fixture output. Stashed items are
  never visited and never index a room.
