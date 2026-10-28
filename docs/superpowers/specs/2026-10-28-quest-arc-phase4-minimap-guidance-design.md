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
| `Engine.ResolveQuestTarget(questId, step) int`: -1 wins, then the mob's current room when exactly one live instance exists (`mobRoomLookup`), then `map_target`, then a `room_enter` trigger gated on the step, else 0 | `internal/questengine/map_target.go:19-98` |
| The only production caller is the `Char.Quests` build, for the focused quest only; it then calls `mapper.NextStep(user room, target)` | `modules/gmcp/gmcp.Char.go:721-730` |
| `Char.Quests` is re-pushed on every room change (`roomChangeHandler`) | `modules/gmcp/gmcp.Char.go:180-205` |
| `GMCPCharModule_Payload_Quest` carries `target_room`, `next_room`, `next_dir` (focused quest only) | `modules/gmcp/gmcp.Char.go:1183-1202` |
| `mapper.NextStep`: `GetPath` (A* in the start room's zone mapper, LRU-cached per pair) else `crossZoneHop` via `nextZoneHop` | `internal/mapper/mapper.path.go:196-301`; `mapper.crosszone.go:237-250` |
| Web client: `setQuestMarker({targetRoom, nextRoom, nextDir, name})`, `_drawQuestArrow`, the pin drawn in the room token build (patina `#6bb0a0` halo + brass ring) | `_datafiles/html/public/static/js/gmcp.js:636-720, 1007-1022` |
| Map palette: `questPatina` and `party` are the same `#6bb0a0` | `gmcp.js:1402-1403` |
| Quest panel: "◆ marked on map" only when the target room is on the drawn map, else "→ head <dir>" | `_datafiles/html/public/webclient-pure.html:1018-1035` |
| `Zone.Map` carries only rooms of the zone the player stands in | `webclient-pure.html:1020-1023` (comment); #650 spec |
| The `quest` command (`usercommands.Quests`) shows no destination or direction | `internal/usercommands/quests.go:104` |
| Validator: `map_target` room exists, `map_target_mob` mob exists, warnings for -1 with a mob and a mob with no room fallback | `internal/quests/validate_refs.go:314-331` |
| `ValidateQuestRefs` warnings are shown only by the builder (`gmcp.Quest.go:245`); `questloop.ruleRefs` drops them (`rules_more.go:112`) | grep `ValidateQuestRefs(` |
| Walk findings (`questengine.Walk`) are ignored by boot and ratcheted by the CI sweep against `quests/loop-backlog.txt` (280 lines today) | `internal/questengine/walkthrough.go:25`; `internal/questloop/context.md` "Doors" |
| Boot door: `validateShippedQuests`, `questDoor` | `main_quests.go:21, 33` |
| Quest editor is off (`refuseQuestBuild`) until phase 6; its step form writes `map_target` | `modules/gmcp/gmcp.Build.go:77`; `_datafiles/html/public/static/js/quests.js:749-757` |
| `dialoguecheck` reads `MapTarget` / `MapTargetMob` to place quests in zones | `cmd/dialoguecheck/walk.go:661-664` |
| Areas: `areas.Get`, `Rooms(key)` (sorted template ids), `Contains`, `AreasOf`; one shipped area `unquiet_grove` | `internal/areas/loader.go`; `_datafiles/world/dogmud/areas/` |
| Template room to zone index from boot: `rooms.TemplateRoomZone(s)`, built in `loadAllRoomZones`, kept current by builder save, move, rename, delete | `internal/rooms/template_zones.go`; `internal/rooms/save_and_load.go:514` |
| `loadAllRoomZones` parses every room file at boot; 723 `spawninfo:` entries in room YAML; `SpawnInfo.MobId` | `save_and_load.go:545`; `internal/rooms/spawninfo.go:10-11` |
| Hidden: `Character.Perceives(other)` (not hidden, or see-hidden from any source), the one rule for room listing and naming | `internal/characters/character.go:991` |
| Light: `messaging.ParticipantSight(observer, room)` returns SightNone / SightShapes / SightFull from blindness, room light, night vision, infravision | `internal/messaging/predicates.go:56` |
| Trigger place fields: `Room`, `Mob`, `Area`, `Zone`, `Conditions.InRoom` | `internal/quests/triggers.go:18-31, 114` |

**Shipped step targets (228 non-secret, non-`end` steps):** 123 fixed room, 40 NPC with a room
fallback, 7 inferred from `room_enter`, 2 with nothing (the generic test quest 1000000), 56 set to
`-1`. Of the 56: about 33 are "do it anywhere" action steps (cast, skill, sleep; quest 28, quest 31,
the nine `secret_of_*` quests), about 23 have a destination the old shape could not express
(return to giver, two bundled rooms, a branch, "no single destination"). 220 steps across all quests
(secret included) set `map_target` or `map_target_mob`.

## Owner rulings (2026-10-28)

1. **`anywhere` is its own declared kind**, separate from "destination unknown". It logs nothing.
   Only a step that resolves to nothing is unknown, and only unknown warns.
2. **Content scope:** retag the "anywhere" steps, convert one live step to each new kind, leave the
   remaining gaps as warnings for phase 7.
3. **Several live copies of an NPC:** point at the nearest copy.
4. **Only copies the player perceives count**: a copy that is hidden
   from the player, or in a room too dark for them, is not a target. When none is perceived the step
   falls back (rule 6).
5. **Perceived means full sight.** A copy the player would make out only as a shape does not count.
6. **Fallback** for a mob target with no perceived copy: the step's declared fallback, else the
   mob's spawn rooms, else unknown.
7. **Inference reads every place field** of the triggers gated on the step; an explicit target
   always wins.
8. **Area render:** a soft brass wash over the area's rooms with a dashed edge and the area name,
   pin on the nearest room (mockup B). Drawn inside and outside the area.
9. **Inside a zone or area target:** a banner "Somewhere in <name>", no pin, no arrow (mockup A).
   The owner would prefer a drawn shader around the zone too; the map draws only the current zone,
   so inside the zone a shader would cover every room, and the banner carries it.
10. **YAML:** one `target:` block replaces `map_target` and `map_target_mob`.
11. **Wandering mobs** (the oasis elementals): whole-zone by default is a coin flip, so it is not
    the default. The mob fallback accepts `zone:` so an author can choose it per step.
12. **The `quest` command gets a destination line** in phase 4, from the same resolver.
13. **Remaining unknown destinations are fixed in phase 7**, zone by zone; a zone's phase 7 pass is
    not done while it has `destination` lines in the backlog.

## 1. The `target:` block

```yaml
steps:
  - id: start
    target:
      mob: 94          # the nearest copy the player perceives
      room: 473        # fallback (or area: / zone:)
  - id: hunt
    target:
      area: unquiet_grove
  - id: scout
    target:
      zone: Pothole Coulee
  - id: cast
    target: anywhere
  - id: report        # no target: inferred from triggers, else unknown
```

`quests.StepTarget{Anywhere bool; Room int; Mob int; Area string; Zone string}` on
`QuestStep.Target`, with an `UnmarshalYAML` that takes the scalar `anywhere` or the mapping.
Allowed shapes: `anywhere`; `room`; `area`; `zone`; `mob` alone or with exactly one of `room`,
`area`, `zone` as its fallback. Anything else (two of room/area/zone without a mob, `anywhere` in a
mapping, an unknown key) is a load refusal. `validate_refs` checks the room, mob, area key and zone
name exist. The two old warnings (-1 with a mob, a mob with no room fallback) go: the first shape
cannot be written, and a mob without a declared fallback now falls back to its spawn rooms.

`MapTarget` and `MapTargetMob` are deleted; the compiler lists every reader
(`map_target.go`, `validate_refs.go`, `dialoguecheck/walk.go`, tests). A guard test fails on any
quest file containing `map_target`. The phase 6 roadmap entry gains the target block for the
editor's step form (`quests.js:749-757`), which stays as it is while the editor is off.

**Conversion of the 220 steps:** `map_target: N` to `room: N`; `map_target_mob: M` plus
`map_target: N` to `mob: M` + `room: N`; `-1` on an action step (the comment says "action step", or
the step is cast, skill or sleep) to `target: anywhere`; every other `-1` loses its target and goes
through inference. The list of each `-1` and its outcome is in the slice 1 PR.

## 2. Resolution

`questengine.ResolveTarget(user, questId, step) Target` replaces `ResolveQuestTarget`.

```go
type Target struct {
    Kind     TargetKind // anywhere, room, mob, area, zone, unknown
    Label    string     // NPC, area or zone name; "" for a room or a mixed set
    Rooms    []int      // the target set (template ids), sorted
    Pin      int        // nearest room of the set reachable from the player; 0 if none
    Inside   bool       // the player's room is in the set
    NextRoom int
    NextDir  string
    Inferred bool
}
```

Order:

1. `anywhere`: Kind anywhere, nothing else.
2. `mob`: the rooms of every live copy the player perceives, judged as if the player stood in that
   copy's room: `user.Character.Perceives(&mob.Character)` and
   `messaging.ParticipantSight(&user.Character, mobRoom) == SightFull`. A copy in an instance room
   is mapped through `rooms.OriginalRoomId` for the set. No perceived copy: the declared fallback
   (room, area or zone, resolved as below, Kind of the fallback, Label still the NPC), else
   `rooms.SpawnRooms(mobId)` (Kind area, Label the NPC), else unknown. This applies to a single-copy
   NPC too: a hidden or unlit quest giver is no longer tracked through walls.
3. `room`, `area` (`areas.Rooms`), `zone` (rooms of that zone from the template-zone index): that
   set.
4. No block: the union of what every trigger gated on the step (`conditions.has` contains the
   step token) names: `room`, `conditions.in_room`, `area`, `zone`, `mob` (as in rule 2, with no
   declared fallback). One kind keeps its kind and label; mixed kinds give Kind room with the union
   as the set and no label. `Inferred` is set.
5. Nothing: Kind unknown.

**Nearest and direction.** New `mapper.NearestOf(fromRoomId int, set map[int]bool) (pin, nextRoom
int, exit string, ok bool)`: one search over the start room's zone mapper that stops at the first
room in the set, using the same exits and costs as `findPath`. A set with no reachable room in the
player's zone picks the set's zone with the fewest zone hops (`nextZoneHop`) and takes
`crossZoneHop`; Pin is then 0 (the room is not on the drawn map) and only the direction is given.
Inside the set: no pin, no direction.

**Spawn index.** `rooms.SpawnRooms(mobId) []int`, built in the same `loadAllRoomZones` pass from
every room's `SpawnInfo.MobId` and kept current at the same points as the template-zone index
(builder save, move, delete), behind its own lock like `templateZone`.

**Cost.** `ResolveTarget` runs per move per player with a focused quest. The mob scan already walks
every mob instance today; the perceive check adds two calls per matching copy. The nearest search is
one bounded search per move instead of today's one cached A*.

## 3. GMCP and the web client

`GMCPCharModule_Payload_Quest` keeps `target_room` (now the Pin), `next_room`, `next_dir`, and adds,
for the focused quest only:

- `target_kind`: anywhere, room, mob, area, zone, unknown
- `target_label`
- `target_rooms`: the set's rooms in the player's current zone; empty for a zone target and a
  single room
- `inside`
- `guide`: the destination line (section 4), so the web panel and the `quest` command say the same
  words

The handler calls `ResolveTarget` once; its own `mapper.NextStep` call goes.

Web client:

- **Wash.** Each room of `target_rooms` gets a soft brass tile the size of the grid spacing, so
  neighbouring rooms merge into one region; a dashed brass edge on each tile side whose neighbour is
  not in the set; the label on the region. Rebuilt only when the set changes.
- **Pin and arrow:** as today, on `target_room` and `next_room`; the off-map stub stays. The pin's
  halo moves from patina to brass, so a quest marker no longer shares the party dot's colour.
- **Banner:** "Somewhere in <label>" across the map foot when `inside` is true for an area or zone.
- **Panel line:** "◆ marked on map" when the pin is on the drawn map, else `guide`.
- Shape and brightness carry meaning; no green against blue.

## 4. The destination line

One function builds `guide` from a `Target` (player copy rules: 80 columns, no numbers):

| Case | Line |
|---|---|
| route known | `Head <dir>.` |
| inside an area or zone | `You are in <label>.` |
| inside a room or at the NPC | `You are where you need to be.` |
| anywhere | `You can do this anywhere.` |
| set known, no route from here | `You can't find a way there from here.` |
| unknown | `Destination unknown.` |

The `quest` command shows it under each active quest's hint. The web panel shows it for the focused
quest.

## 5. Unknown destinations

A new walk rule in `questengine` (beside `Walk`), key `walk-destination <questId> step <id>`, resolves
each step statically (a mob target counts as resolved when it has a declared fallback or spawn
rooms): one finding per step that is unknown, and one per `anywhere` step whose gated triggers name
a place (`walk-destination-anywhere`, the guard against a mistagged gap). Boot logs each finding
with `mudlog.Warn` and never fails on it. The CI sweep ratchets it like every walk rule: today's
unknown steps go into `loop-backlog.txt`, a new one fails CI until it gets a target or `anywhere`.
The same report lists inferred steps for review (an info line, not a finding).

## 6. Content

- The 220-step conversion (section 1), done by a subagent with Edit, never a script rewrite.
- One live step of each new kind: quest 45 resolves to `unquiet_grove` by inference; one zone step
  and one multi-copy hunting step (a `mob_death` step on a template with several spawns), chosen in
  the slice 3 plan.
- Phase 7's exit gains: no `walk-destination` lines left for the zone.

## 7. Testing and measurement

- `StepTarget` decode: the scalar, each allowed shape, each refused shape.
- `ResolveTarget`: one test per rule; perceive cases (hidden copy without see-hidden, copy in a dark
  room, shapes-only copy, one visible copy among hidden ones, single hidden NPC falling back);
  inference single kind and mixed; unknown.
- `NearestOf` against `GetPath` on the same zone; cross-zone falls to `crossZoneHop`.
- `SpawnRooms` across a builder save and delete.
- The walk rule: an unknown step, a mistagged `anywhere`, backlog ratchet.
- `guide` lines, the `quest` command line.
- Web client: cases in `tools/webclient-tests/` for the wash edges, the banner and each panel line.
- Every probe shown able to fail before its green counts.
- Benchmarks against master's `ResolveQuestTarget` + `NextStep`: room target, 40-room area, mob with
  30 live instances, cross-zone. Numbers in the slice 1 PR.
- Playtest per slice; slice 3 walks one quest of each kind and captures a screenshot of each in the
  browser into the repo root.

## 8. Slices

1. **Engine:** `StepTarget`, conversion, `ResolveTarget`, `SpawnRooms`, `NearestOf`, the destination
   walk rule and boot warnings, `guide`, the `quest` command line, GMCP fields filled (the client
   ignores the new ones until slice 2).
2. **Web client:** wash, banner, panel line, pin colour.
3. **Content and playtest:** the live examples, the backlog lines, the walk of each kind.

## 9. Docs

`context.md` for `questengine`, `quests`, `mapper`, `rooms`, `modules/gmcp`; the roadmap (phase 4
status, phase 6 editor form, phase 7 exit); the `dogmud-authoring-quests` skill (the target block);
PATCH_NOTES; `docs/README.md` for this spec and each plan.
