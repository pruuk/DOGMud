# #375 phase 4: minimap guidance

Issue: #375 (quest and dialogue arc), roadmap `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`
"Phase 4: Minimap guidance (engine half)". Requirement source:
`docs/Quest_arc_screenshots/quest_thoughts.txt:10` ("Quest mini-map can point at a room, an npc,
a zone, or a defined area. It can also just say 'destination unknown', but that needs to be a rare
exception rather than the rule and create log warnings (not a crash)."). Brainstormed with the
owner 2026-10-28. One spec, three slices, each with its own plan, PR and playtest.

## Facts verified against source (master `591b17124`, 2026-10-28)

| Fact | Where |
|---|---|
| `QuestStep.MapTarget int` (`map_target`, 0 = infer, -1 = no marker) and `QuestStep.MapTargetMob int` (`map_target_mob`) | `internal/quests/quests.go:80-94` |
| `TriggerDef.Target string` (`target:`) is the command target kind filter: self, mob, player, item, container | `internal/quests/triggers.go:26` |
| Quests load with `gopkg.in/yaml.v2` (`fileloader`); `SaveQuest` marshals with yaml.v2; `TestRoundTrip_MarshalFixedPointEveryLiveFile` holds every shipped file to a marshal fixed point | `internal/fileloader/fileloader.go:18`; `internal/quests/save.go:16, 37`; `internal/quests/roundtrip_test.go:20` |
| `Quest.Validate` checks steps (empty and duplicate ids) and runs at load and at `SaveQuest` | `internal/quests/quests.go:193, 265-275`; `save.go:34` |
| `Engine.ResolveQuestTarget(questId, step) int`: -1 wins, then the mob's current room when exactly one live instance exists (`mobRoomLookup`), then `map_target`, then a `room_enter` trigger gated on the step, else 0 | `internal/questengine/map_target.go:19-98` |
| The only production caller is the `Char.Quests` build, for the focused quest only; it then calls `mapper.NextStep(user room, target)` | `modules/gmcp/gmcp.Char.go:721-730` |
| `Char.Quests` is re-pushed on every room change (`roomChangeHandler`) | `modules/gmcp/gmcp.Char.go:180-205` |
| `GMCPCharModule_Payload_Quest` carries `target_room`, `next_room`, `next_dir` (focused quest only) | `modules/gmcp/gmcp.Char.go:1183-1202` |
| The client sets a marker only when `target_room` is non-zero, so a payload with `next_dir` and no `target_room` draws no arrow | `_datafiles/html/public/webclient-pure.html:837-841` |
| `mapper.NextStep`: `GetPath` (A* in the start room's zone mapper, LRU-cached per pair) else `crossZoneHop` via `nextZoneHop` (BFS over a zone graph of one link per zone pair) | `internal/mapper/mapper.path.go:196-301`; `mapper.crosszone.go:186-250` |
| `findPath` costs every exit 1 (uniform), so a breadth-first search finds the same shortest distance | `internal/mapper/mapper.path.go:171` |
| `mapper.IsCompassDirection` (north ... southeast, up, down) | `internal/mapper/mapper.go:27-38, 177` |
| Web client: `setQuestMarker({targetRoom, nextRoom, nextDir, name})`, `_drawQuestArrow`, the pin drawn in the room token build (patina `#6bb0a0` halo + brass ring) | `_datafiles/html/public/static/js/gmcp.js:636-720, 1007-1022` |
| Map palette: `questPatina` and `party` are the same `#6bb0a0` | `gmcp.js:1402-1403` |
| Quest panel: "◆ marked on map" only when the target room is on the drawn map, else "→ head <dir>" | `webclient-pure.html:1018-1035` |
| `Zone.Map` carries only rooms of the zone the player stands in | `webclient-pure.html:1020-1023` (comment); #650 spec |
| The `quest` command renders `character/quests.template` with each quest's name, bar, description and progress line; no hint, destination or direction | `internal/usercommands/quests.go:19-28, 74-103`; `_datafiles/world/dogmud/templates/character/quests.template` |
| Validator: `map_target` room exists (and a warning when no trigger names it), `map_target_mob` mob exists, warnings for -1 with a mob and a mob with no room fallback | `internal/quests/validate_refs.go:304-331` |
| The builder's room and mob delete refusals come from `ValidateQuestRefs` errors, whose text names the field (`map_target`, `map_target_mob`) | `modules/gmcp/gmcp.Delete_refs.go:44-69`; `gmcp.Delete_refs_test.go:28-118` |
| `ValidateQuestRefs` warnings are shown only by the builder (`gmcp.Quest.go:245`); `questloop.ruleRefs` drops them (`rules_more.go:112`) | grep `ValidateQuestRefs(` |
| Walk findings (`questengine.Walk`) are ignored by boot and ratcheted with `questloop.Validate` by the CI sweep `TestSmoke_ShippedQuestsPassTheLoop` against `quests/loop-backlog.txt` (280 lines today) | `internal/questengine/walkthrough.go:25`; `quest_loop_smoke_test.go:53-80` |
| Boot door: `validateShippedQuests`, `questDoor`, called from `loadAllDataFiles` after `questengine.LoadDataFiles` | `main_quests.go:21, 33`; `main.go:1975-1982` |
| `TestSmoke_EveryQuestStepHasMarkerDecision` fails on a step with no marker unless `map_target: -1` | `boot_smoke_test.go:372-421` |
| Quest editor is off (`refuseQuestBuild`) until phase 6; its step form writes `map_target` | `modules/gmcp/gmcp.Build.go:77`; `_datafiles/html/public/static/js/quests.js:749-757` |
| `dialoguecheck` reads `MapTarget` / `MapTargetMob` to place quests in zones | `cmd/dialoguecheck/walk.go:661-664` |
| Areas: registry of `areas`, room sets and a room-to-areas index, swapped whole; `Get`, `Contains`, `AreasOf`, `Rooms(key)` (allocates a sorted copy per call), `Keys`, `InstallForTest`; one shipped area `unquiet_grove` | `internal/areas/areas.go:20-40`; `internal/areas/loader.go:96-150` |
| Template room to zone index from boot: `rooms.TemplateRoomZone(s)`, built in `loadAllRoomZones`, kept current by `addRoomToMemory`, `MoveToZone`, `RenameZone`, `DeleteRoomTemplate` | `internal/rooms/template_zones.go`; `save_and_load.go:417, 592`; `roommanager.go:719, 894` |
| `loadAllRoomZones` parses every room file at boot; 723 `spawninfo:` entries in room YAML; `SpawnInfo.MobId`; a builder save goes through `SaveRoomTemplate` | `save_and_load.go:268, 545`; `internal/rooms/spawninfo.go:10-11` |
| `questworld.Sources` builds a spawn set offline by loading every room (walkthrough only) | `internal/questworld/sources.go:60-80` |
| Hidden: `Character.Perceives(other)` (not hidden, or see-hidden from any source), the one rule for room listing and naming | `internal/characters/character.go:991` |
| Light: `messaging.ParticipantSight(observer, room)` returns SightNone / SightShapes / SightFull from blindness, room light, night vision, infravision; a typed-nil room panics | `internal/messaging/predicates.go:56-70` |
| `rooms.OriginalRoomId(id) (int, bool)` maps an instance room to its template | `internal/rooms/ephemeral.go:269` |
| `mobs.GetMobSpec(MobId) *Mob`; `mobs.GetAllMobInstanceIds`, `mobs.GetInstance` | `internal/mobs/mobs.go:807-830` |
| Trigger place fields: `Room`, `Mob`, `Area`, `Zone`, `Conditions.InRoom` | `internal/quests/triggers.go:18-31, 114` |
| `questengine` already imports `areas`, `messaging`, `mobs`, `rooms`, `users`; `mapper` does not import `questengine` | `go list` |

**Shipped step targets (228 non-secret, non-`end` steps):** 123 fixed room, 40 NPC with a room
fallback, 7 inferred from `room_enter`, 2 with nothing (the generic test quest 1000000), 56 set to
`-1`. Of the 56, 33 are "do it anywhere": 32 carry the comment `# action step` (quest 28: 20, quest
31: 2, quest 55: 1, the nine `secret_of_*` quests 79 to 87: 1 each) and quest 42's `sleep anywhere`.
The other 23 have a destination the old shape could not express: 10 return to the giver, 7 bundle
two rooms, 3 have "no single resolved destination", 2 branch (quest 11, quest 72), 1 is a kill in
one of two zones (quest 42). 220 steps across all quests (secret included) set `map_target` or
`map_target_mob`.

## Owner rulings (2026-10-28)

1. **`anywhere` is its own declared kind**, separate from "destination unknown". It logs nothing.
   Only a step that resolves to nothing is unknown, and only unknown warns.
2. **Content scope:** retag the "anywhere" steps, convert one live step to each new kind, leave the
   remaining gaps as warnings for phase 7.
3. **Several live copies of an NPC:** point at the nearest copy.
4. **Only copies the player perceives count**: a copy that is hidden from the player, or in a room
   too dark for them, is not a destination. When none is perceived the step falls back (rule 6).
5. **Perceived means full sight.** A copy the player would make out only as a shape does not count.
6. **Fallback** for a mob destination with no perceived copy: the step's declared fallback, else the
   mob's spawn rooms, else unknown.
7. **Inference reads every place field** of the triggers gated on the step; an explicit destination
   always wins.
8. **Area render:** a soft brass wash over the area's rooms with a dashed edge and the area name,
   pin on the nearest room (mockup B). Drawn inside and outside the area.
9. **Inside a zone or area destination:** a banner "Somewhere in <name>", no pin, no arrow (mockup
   A). The owner would prefer a drawn shader around the zone too; the map draws only the current
   zone, so inside the zone a shader would cover every room, and the banner carries it.
10. **YAML:** one step block replaces `map_target` and `map_target_mob`.
11. **Wandering mobs** (the oasis elementals): whole-zone by default is a coin flip, so it is not
    the default. The mob fallback accepts `zone:` so an author can choose it per step.
12. **The `quest` command gets a destination line** in phase 4, from the same resolver.
13. **Remaining unknown destinations are fixed in phase 7**, zone by zone; a zone's phase 7 pass is
    not done while it has `walk-destination` lines in the backlog.
14. **The block is named `destination:`**, not `target:`, because a trigger's `target:` already
    means the command target kind and both sit in one quest file.

## 1. The `destination:` block

```yaml
steps:
  - id: start
    destination:
      mob: 94          # the nearest copy the player perceives
      room: 473        # fallback (or area: / zone:)
  - id: hunt
    destination:
      area: unquiet_grove
  - id: scout
    destination:
      zone: Pothole Coulee
  - id: cast
    destination: anywhere
  - id: report        # no destination: inferred from triggers, else unknown
```

`quests.StepDestination{Anywhere bool; Room int; Mob int; Area string; Zone string}` on
`QuestStep.Destination` (a value, not a pointer; `IsZero` means no block), with `UnmarshalYAML`
taking the scalar `anywhere` or a mapping of the keys `room`, `mob`, `area`, `zone`, and
`MarshalYAML` writing them back the same way, so the round-trip test stays a fixed point. Allowed
shapes, checked in `Quest.Validate`: `anywhere`; `room`; `area`; `zone`; `mob` alone or with exactly
one of `room`, `area`, `zone` as its fallback. Any other scalar, an unknown key, or two of
room/area/zone is a load refusal. `ValidateQuestRefs` checks the room, mob, area key and zone name
exist, under the labels `destination room`, `destination mob`, `destination area`, `destination
zone`, which the builder's delete refusals then name. The warning "room referenced by no trigger"
stays (for `destination.room`); the two mob warnings go: the first shape cannot be written, and a
mob without a declared fallback falls back to its spawn rooms.

`MapTarget` and `MapTargetMob` are deleted; the compiler lists every reader (`map_target.go`,
`validate_refs.go`, `dialoguecheck/walk.go`, `boot_smoke_test.go`, tests). A guard test fails on any
quest file containing `map_target`. The phase 6 roadmap entry gains the `destination:` block for the
editor's step form (`quests.js:749-757`), which stays as it is while the editor is off.

**Conversion of the 220 steps:** `map_target: N` to `room: N`; `map_target_mob: M` plus
`map_target: N` to `mob: M` + `room: N`; the 33 "anywhere" `-1` steps to `destination: anywhere`;
the other 23 `-1` steps lose the field and go through inference. Every `-1` and its outcome is
listed in the slice 1 PR.

## 2. Resolution

`questengine.ResolveDestination(user, questId, step) Destination` replaces `ResolveQuestTarget`.

```go
type Destination struct {
    Kind      DestKind // "" (none: final step), anywhere, room, mob, area, zone, unknown
    Label     string   // area or zone name ("the unquiet grove"); "" otherwise
    Npc       string   // the NPC's name for a mob destination, also when it fell back
    ZoneRooms []int    // the set's template rooms in the player's zone, sorted (for the wash)
    Pin       int      // nearest set room reachable inside the player's zone; 0 if none
    Inside    bool     // the player's room is in the set
    NextRoom  int
    NextDir   string
    Inferred  bool
}
```

Order:

1. Final step (or no such quest or step): Kind "".
2. `anywhere`: Kind anywhere.
3. `mob`: the template rooms of every live copy the player perceives, judged as if the player stood
   in that copy's room: `user.Character.Perceives(&copy.Character)` and
   `messaging.ParticipantSight(&user.Character, copyRoom) == SightFull`, the sight judged once per
   room per call. A copy in an instance room maps through `rooms.OriginalRoomId`. No perceived
   copy: the declared fallback (resolved as rule 4, Kind of the fallback, Npc kept), else
   `rooms.SpawnRooms(mobId)` (Kind area, no Label, Npc kept), else unknown. This applies to a
   single-copy NPC too: a hidden or unlit quest giver is no longer tracked through walls.
4. `room` (that room), `area` (`areas.Contains`), `zone` (rooms whose template zone is that zone).
5. No block: the union of what every trigger gated on the step (`conditions.has` contains the
   step token) names: `room`, `conditions.in_room`, `area`, `zone`, `mob` (as rule 3, with no
   declared fallback). One kind keeps its kind and label; mixed kinds give Kind room over the union.
   `Inferred` is set.
6. Nothing: Kind unknown.

A set is held as a membership test plus the list of zones it touches, never as a full room list per
call: a zone is its name, an area gains `areas.ZonesOf(key)` and `areas.RoomsIn(key, zone)` computed
once at load, a mob set is the handful of perceived rooms.

**Nearest and direction.** New `mapper.NearestOf(fromRoomId int, in func(roomId int) bool, zones
[]string) (pin, nextRoom int, exit string, ok bool)`: a breadth-first search over the start room's
zone mapper (the same exits as `findPath`, exits taken in name order so ties are stable) that stops
at the first room `in` accepts. When no room of the player's zone qualifies, it takes the zone of
`zones` with the fewest zone hops (`nearestZoneHop`, the multi-target form of `nextZoneHop`, which
then calls it with one zone) and the same border step `crossZoneHop` takes; Pin is then 0 (the room
is not on the drawn map) and only the direction is given. Inside the set: no pin, no direction.

**Spawn index.** `rooms.SpawnRooms(mobId) []int` (sorted, shared), filled in the same
`loadAllRoomZones` pass from every template room's `SpawnInfo.MobId`, replaced for a room on
`SaveRoomTemplate` and dropped on `DeleteRoomTemplate`, behind its own lock like `templateZone`. A
move or rename does not change a room's spawns.

**Cost.** `ResolveDestination` runs per move per player with a focused quest. The mob scan already
walks every mob instance today; the perceive check adds one `Perceives` and at most one
`ParticipantSight` per distinct room. The nearest search is one breadth-first search inside one zone
instead of today's one cached A*.

## 3. GMCP and the web client

`GMCPCharModule_Payload_Quest` keeps `target_room` (now the Pin), `next_room`, `next_dir`, and adds,
for the focused quest only:

- `target_kind`: anywhere, room, mob, area, zone, unknown
- `target_label`: the area or zone name, else the NPC name
- `target_rooms`: `ZoneRooms`; empty for a zone destination and a single room
- `inside`
- `guide`: the destination line (section 4), so the web panel and the `quest` command say the same
  words

The handler calls `ResolveDestination` once; its own `mapper.NextStep` call goes. Slice 1 changes
the client's marker condition from `target_room` to `target_room || next_room`, so a cross-zone
destination (Pin 0) keeps today's off-map arrow stub.

Web client (slice 2):

- **Wash.** Each room of `target_rooms` gets a soft brass tile the size of the grid spacing, so
  neighbouring rooms merge into one region; a dashed brass edge on each tile side whose neighbour is
  not in the set; the label on the region. Rebuilt only when the set changes.
- **Pin and arrow:** as today, on `target_room` and `next_room`; the off-map stub stays. The pin's
  halo moves from patina to brass, so a quest marker no longer shares the party dot's colour.
- **Banner:** "Somewhere in <label>" across the map foot when `inside` is true for an area or zone.
- **Panel line:** "◆ marked on map" when the pin is on the drawn map, else `guide`.
- Shape and brightness carry meaning; no green against blue.

## 4. The destination line

`questengine.GuideLine(d Destination) string` (player copy rules: 80 columns, no numbers):

| Case | Line |
|---|---|
| route known, compass exit | `Head north.` |
| route known, any other exit | `Take the gate.` (the exit name, `command` colour) |
| inside, with a Label | `You are in the unquiet grove.` |
| inside a fallback set of a mob destination | `You are where <npc> is usually found.` |
| inside otherwise | `You are where you need to be.` |
| anywhere | `You can do this anywhere.` |
| destination known, no route from here | `You can't find a way there from here.` |
| unknown | `Destination unknown.` |
| Kind "" | "" (no line) |

The `quest` command shows it under each active quest's progress line (a `Guide` field on
`questRecord`, rendered by `character/quests.template`). The web panel shows it for the focused
quest.

## 5. Unknown destinations

`questengine.Destinations(w questloop.World) (findings []questloop.Finding, inferred []string)`
resolves each step of each non-secret quest statically, skipping the final step; a mob destination
counts as resolved when it has a declared fallback or spawn rooms. Rules:

- `walk-destination`, Where `step <id>`: the step resolves to unknown.
- `walk-destination-anywhere`, Where `step <id>`: an `anywhere` step whose gated triggers name a
  place (the guard against a mistagged gap).

Boot (`destinationDoor` in `main_quests.go`, after `questDoor`) logs each finding with `mudlog.Warn`
and the inferred count with `mudlog.Info`, and never fails. The CI sweep adds `Destinations` to
`Validate` plus `Walk`, so today's findings go into `loop-backlog.txt` and a new one fails CI until
the step gets a destination or `anywhere`. The sweep logs each inferred step for review.
`TestSmoke_EveryQuestStepHasMarkerDecision` is deleted: the rule replaces it.

## 6. Content

- The 220-step conversion (section 1), done by a subagent with Edit, never a script rewrite.
- After it, any `walk-destination-anywhere` finding drops that step's `anywhere` (its trigger names
  the place, so inference gives it a destination); the PR lists each.
- One live step of each new kind: quest 45 resolves to `unquiet_grove` by inference; one zone step
  and one multi-copy hunting step (a `mob_death` step on a template with several spawns), chosen in
  the slice 3 plan.
- Phase 7's exit gains: no `walk-destination` lines left for the zone.

## 7. Testing and measurement

- `StepDestination` decode and marshal: the scalar, each allowed shape, each refused shape; the
  round-trip test over shipped files.
- `ResolveDestination`: one test per rule; perceive cases (hidden copy without see-hidden, copy in
  a dark room, shapes-only copy, one visible copy among hidden ones, single hidden NPC falling back
  to its room, then to spawn rooms); inference single kind and mixed; unknown; final step.
- `NearestOf` against `findPath` distances on the same map; cross-zone takes `nearestZoneHop`.
- `SpawnRooms` at boot, across `SaveRoomTemplate` and `DeleteRoomTemplate`.
- `areas.ZonesOf`, `areas.RoomsIn`.
- `Destinations`: an unknown step, a mistagged `anywhere`, a mob with only spawn rooms, the final
  step skipped, a secret quest skipped; the backlog ratchet.
- `GuideLine` for every row of section 4; the `quest` command line.
- Web client: cases in `tools/webclient-tests/` for the slice 1 marker condition, and in slice 2 for
  the wash edges, the banner and each panel line.
- Every probe shown able to fail before its green counts.
- Benchmarks against master's `ResolveQuestTarget` + `NextStep`: room destination, 40-room area,
  mob with 30 live instances, cross-zone. Numbers in the slice 1 PR.
- Playtest per slice; slice 3 walks one quest of each kind and captures a screenshot of each in the
  browser into the repo root.

## 8. Slices

1. **Engine:** `StepDestination`, conversion, `ResolveDestination`, `SpawnRooms`, `areas.ZonesOf`
   and `RoomsIn`, `NearestOf`, `Destinations` and the boot warnings, `GuideLine`, the `quest`
   command line, GMCP fields filled, the client marker condition.
2. **Web client:** wash, banner, panel line, pin colour.
3. **Content and playtest:** the live examples, the backlog lines, the walk of each kind.

## 9. Docs

`context.md` for `questengine`, `quests`, `mapper`, `rooms`, `areas`, `modules/gmcp`; the roadmap
(phase 4 status, phase 6 editor form, phase 7 exit); the `dogmud-authoring-quests` skill (the
`destination:` block); PATCH_NOTES; `docs/README.md` for this spec and each plan.
