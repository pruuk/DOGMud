# #375 phase 4 slice 1: destination engine, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `map_target` / `map_target_mob` with a `destination:` block and one resolver that
finds the nearest perceived copy of an NPC, an area, a zone or a room, infers a destination from a
step's triggers, says where to head in one shared line, and warns at boot about every step whose
destination is unknown.

**Architecture:** `quests.StepDestination` is the YAML type. `questengine.ResolveDestination`
builds a set (membership test plus the zones it touches) from the declared block or the step's
triggers, filters NPC copies by `Perceives` and full sight, and asks the new `mapper.NearestOf` for
the nearest room and the first hop. `questengine.GuideLine` turns the result into the line the GMCP
payload and the `quest` command share. `questengine.Destinations` is a static walk rule: boot logs
it, the CI sweep ratchets it against `loop-backlog.txt`.

**Tech stack:** Go, gopkg.in/yaml.v2 (quests), gopkg.in/yaml.v3 (areas), node for the web client
check.

**Spec:** `docs/superpowers/specs/2026-10-28-quest-arc-phase4-minimap-guidance-design.md` (owner
approved 2026-10-28; rulings 1 to 14). Slice 1 of 3.

---

## Ground rules for every task

- Edit files with the Edit and Write tools only. No `sed -i`, no `>` into a tracked file, no Python
  read-modify-write. Generated files are written to the session scratchpad and copied into place
  (Task 8). The hooks enforce this.
- `git add <named paths>` only. Never `-A`, `.`, `--all`, `commit -a`. Never `--no-verify`.
- Commit messages through a quoted heredoc, ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- A new test must be seen failing (or proven able to fail) before its green counts.
- Test binaries load Go config defaults, not `config.yaml`.
- Work in the worktree `C:/tmp/dogmud-375-p4s1` on branch `feat/375-p4-destination`, cut from
  master after the spec PR merges (see Task 0).

## Facts verified against source (master `591b17124`, 2026-10-28)

The spec's facts table holds. Plan-specific facts:

| Fact | Where |
|---|---|
| `QuestStep` fields: `Id`, `Description`, `Hint`, `MapTarget int`, `MapTargetMob int` | `internal/quests/quests.go:80-95` |
| Step ids are checked in `Quest.Validate`'s loop "Check for duplicate step IDs" | `internal/quests/quests.go:265-275` |
| `ValidateQuestRefs` step block (map_target checks and warnings) | `internal/quests/validate_refs.go:304-332` |
| `refsCtx.checkRoom`, `checkMob`; `QuestValidators.AreaExists`, `ZoneExists` | `validate_refs.go:111-129, 17-45` |
| `areas.registry{areas, rooms, byRoom}`, `newRegistry`, `add(a, set)` called by `loadDir` and `InstallForTest`; test `TestIndexAndContains` calls `add` with two args | `internal/areas/areas.go:20-37`; `loader.go:90, 140`; `areas_test.go:51-62` |
| `setTemplateZone` in `loadAllRoomZones` (`save_and_load.go:592`); `deleteTemplateZone` in `DeleteRoomTemplate` (`save_and_load.go:417`); `SaveRoomTemplate` calls `InvalidateTemplateCache(roomTpl.RoomId)` after writing (`save_and_load.go:310`) | `internal/rooms/` |
| `IsEphemeralRoomId` guards `setTemplateZone` | `internal/rooms/template_zones.go:18-21` |
| Test helpers: `setupPrepareRoom(t, id)` (temp DataFiles, a `preparezone` room), `restoreZone` | `internal/rooms/autosave_prepare_test.go:24`; `template_zones_test.go:40` |
| `mapper.findPath` (uniform cost 1 per exit), `buildTestMapper`, `buildCycleMapper`, `stubZoneGraph`, `npStub` | `internal/mapper/mapper.path.go:102-193`; `mapper.path_test.go:10-30`; `mapper.crosszone_test.go:7-34` |
| `nextZoneHop(fromZone, toZone)`, `crossZoneHop(fromRoomId, fromZone, toZone)` | `internal/mapper/mapper.crosszone.go:186-250` |
| `mapper.IsCompassDirection` | `internal/mapper/mapper.go:177` |
| `util.TrackTime(name, seconds)` used by `GetPath` | `internal/mapper/mapper.path.go:198-201` |
| `users.NewTestUser(userId, username, charName, connId)` (character in room 1) | `internal/users/test_helpers.go:56` |
| Hidden fixture: `characters.New()`, `Awareness = awareness.NewMachine()`, `TransitionToConcealing` then `ResolveConcealment(true, ...)` | `internal/characters/perceives_test.go:14-34` |
| `messaging.SightNone`, `SightShapes`, `SightFull`; `RoomVisibility{LightLevel() int}` | `internal/messaging/predicates.go:21-56` |
| GMCP focused-quest marker block | `modules/gmcp/gmcp.Char.go:721-730` (the file's only `mapper.` use is line 725) |
| GMCP payload struct | `modules/gmcp/gmcp.Char.go:1183-1202` |
| Client marker condition `if (f && f.target_room)` | `_datafiles/html/public/webclient-pure.html:837` |
| JS check `tools/webclient-tests/quest-panel-refresh.js` (records `gr.setQuestMarker` calls; ends with the `fails` summary); CI runs every `tools/webclient-tests/*.js` | `.github/workflows/validate.yml:102` |
| `questRecord` (no Guide), `activeQuestRecord`, template `character/quests.template` renders Progress | `internal/usercommands/quests.go:19-103`; `_datafiles/world/dogmud/templates/character/quests.template` |
| Quest log test pattern: `useDogmudTemplates(t)`, `activeQuestRecord`, `templates.Process("character/quests", ...)` | `internal/usercommands/quests_progress_test.go:29-50` |
| CI sweep `TestSmoke_ShippedQuestsPassTheLoop` builds `fs := append(questloop.Validate(w), questengine.Walk(w, questworld.Sources())...)` | `quest_loop_smoke_test.go:53-80` |
| `TestSmoke_EveryQuestStepHasMarkerDecision` | `boot_smoke_test.go:372-421` |
| `questDoor(isReload)` call in `loadAllDataFiles` | `main.go:1980` |
| Delete refusal tests name `map_target` and `map_target_mob` | `modules/gmcp/gmcp.Delete_refs_test.go:28-118` |
| `dialoguecheck` reads `st.MapTarget`, `st.MapTargetMob` | `cmd/dialoguecheck/walk.go:660-666` |
| Old resolver and its tests: `map_target.go`, `map_target_test.go`, `map_target_mob_test.go`, `map_target_data_test.go` | `internal/questengine/` |
| 260 `map_target`/`map_target_mob` lines in 79 quest files: 220 steps, every `map_target_mob` line directly after its `map_target` line, indents 2 or 4, no `map_target: 0`; one comment mentions `map_target` (`88-tokes_road.yaml:16`) | scan of `_datafiles/world/dogmud/quests/` |
| The 56 `-1` lines: 32 with `# action step`, quest 42 line 73 `sleep anywhere` (33 anywhere); 23 others listed in Task 8 | scan |
| Backlog regeneration: `DOGMUD_LOOP_BACKLOG_WRITE=<scratch path>` with `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_ShippedQuestsPassTheLoop` | `quest_loop_smoke_test.go:18-22, 59-65`; `internal/questloop/context.md` |
| Shipped quest smokes are `package main` at the repo root: `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke`; `./internal/...` does not run them | 10-27 handoff |

## File map

| File | Change |
|---|---|
| `internal/quests/destination.go` (new) | `StepDestination`, YAML (un)marshal, `Validate` |
| `internal/quests/destination_test.go` (new) | decode, marshal, shape tests |
| `internal/quests/quests.go` | `QuestStep.Destination`; step loop calls `Destination.Validate`; later delete `MapTarget*` |
| `internal/quests/validate_refs.go` | destination ref checks; later drop map_target checks |
| `internal/areas/areas.go`, `loader.go`, tests | `ZonesOf`, `RoomsIn` |
| `internal/rooms/spawn_rooms.go` (new), `spawn_rooms_test.go` (new), `save_and_load.go` | spawn index |
| `internal/mapper/mapper.nearest.go` (new), `mapper.nearest_test.go` (new), `mapper.crosszone.go` | `NearestOf`, `nearestZoneHop`, `hopToward` |
| `internal/questengine/destination.go` (new), `destination_test.go` (new) | resolver, places, sets |
| `internal/questengine/guide.go` (new), `guide_test.go` (new) | `GuideLine` |
| `internal/questengine/destinations_rule.go` (new), `destinations_rule_test.go` (new) | `Destinations` |
| `internal/questengine/destination_bench_test.go` (new), `internal/mapper/mapper.nearest_bench_test.go` (new) | benchmarks |
| `internal/questengine/map_target*.go` | deleted |
| `main_quests.go`, `main.go`, `main_quests_test.go` or `quest_loop_smoke_test.go` | `destinationDoor`, sweep |
| `boot_smoke_test.go` | delete the marker-decision smoke |
| `modules/gmcp/gmcp.Char.go`, `gmcp.Delete_refs_test.go` | payload fields, labels |
| `_datafiles/html/public/webclient-pure.html`, `tools/webclient-tests/quest-panel-refresh.js` | marker condition |
| `internal/usercommands/quests.go`, `quests_progress_test.go`, `_datafiles/world/dogmud/templates/character/quests.template` | Guide line |
| `cmd/dialoguecheck/walk.go` | read `Destination` |
| `internal/guards/quest_destination_field_test.go` (new) | no `map_target` key in quest files |
| `_datafiles/world/dogmud/quests/*.yaml` (79 files) | conversion |
| `_datafiles/world/dogmud/quests/loop-backlog.txt` | destination lines |
| docs (Task 13) | context.md x6, roadmap, skill, PATCH_NOTES, README |

---

### Task 0: Worktree

- [ ] **Step 1:** The spec branch `docs/375-phase4-spec` holds the spec and this plan. Push it and
  open a docs PR (`gh pr create --repo pruuk/DOGMud`), merge it once the owner has approved the
  plan, then:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud" && git fetch origin && git worktree add -b feat/375-p4-destination C:/tmp/dogmud-375-p4s1 origin/master
```

- [ ] **Step 2:** Record `C:/tmp/dogmud-375-p4s1` in the session handoff memory (the tmp sweep
  keeps only referenced paths). Baseline: `go build ./... && go test ./internal/quests/ ./internal/questengine/ ./internal/mapper/ ./internal/areas/ ./internal/rooms/` green.

---

### Task 1: `StepDestination`

**Files:** Create `internal/quests/destination.go`, `internal/quests/destination_test.go`; modify
`internal/quests/quests.go`.

- [ ] **Step 1: Failing tests** (`internal/quests/destination_test.go`):

```go
package quests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type destHolder struct {
	Destination StepDestination `yaml:"destination,omitempty"`
}

func decodeDest(t *testing.T, src string) (StepDestination, error) {
	t.Helper()
	var h destHolder
	err := yaml.Unmarshal([]byte(src), &h)
	return h.Destination, err
}

func TestStepDestination_Decodes(t *testing.T) {
	cases := []struct {
		src  string
		want StepDestination
	}{
		{"destination: anywhere\n", StepDestination{Anywhere: true}},
		{"destination:\n  room: 473\n", StepDestination{Room: 473}},
		{"destination:\n  area: unquiet_grove\n", StepDestination{Area: "unquiet_grove"}},
		{"destination:\n  zone: Pothole Coulee\n", StepDestination{Zone: "Pothole Coulee"}},
		{"destination:\n  mob: 94\n  room: 473\n", StepDestination{Mob: 94, Room: 473}},
		{"destination:\n  mob: 94\n", StepDestination{Mob: 94}},
		{"other: 1\n", StepDestination{}},
	}
	for _, c := range cases {
		got, err := decodeDest(t, c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %+v, want %+v", c.src, got, c.want)
		}
	}
}

func TestStepDestination_RefusesBadInput(t *testing.T) {
	for _, src := range []string{
		"destination: somewhere\n",
		"destination: 473\n",
		"destination:\n  roam: 473\n",
		"destination:\n  anywhere: true\n",
	} {
		if _, err := decodeDest(t, src); err == nil {
			t.Errorf("%q decoded; want a refusal", src)
		}
	}
}

func TestStepDestination_ValidateShapes(t *testing.T) {
	good := []StepDestination{
		{}, {Anywhere: true}, {Room: 1}, {Area: "a"}, {Zone: "z"},
		{Mob: 2}, {Mob: 2, Room: 1}, {Mob: 2, Area: "a"}, {Mob: 2, Zone: "z"},
	}
	for _, d := range good {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	bad := []StepDestination{
		{Room: 1, Area: "a"}, {Area: "a", Zone: "z"}, {Mob: 2, Room: 1, Zone: "z"},
		{Anywhere: true, Room: 1}, {Anywhere: true, Mob: 2}, {Room: -1}, {Mob: -2},
	}
	for _, d := range bad {
		if err := d.Validate(); err == nil {
			t.Errorf("%+v validated; want a refusal", d)
		}
	}
}

func TestStepDestination_MarshalRoundTrips(t *testing.T) {
	for _, d := range []StepDestination{{Anywhere: true}, {Room: 473}, {Mob: 94, Room: 473}, {Area: "g"}} {
		out, err := yaml.Marshal(destHolder{Destination: d})
		if err != nil {
			t.Fatal(err)
		}
		back, err := decodeDest(t, string(out))
		if err != nil || back != d {
			t.Errorf("%+v -> %q -> %+v (%v)", d, out, back, err)
		}
	}
	out, _ := yaml.Marshal(destHolder{})
	if strings.Contains(string(out), "destination") {
		t.Errorf("an empty destination must be omitted, got %q", out)
	}
	out, _ = yaml.Marshal(destHolder{Destination: StepDestination{Anywhere: true}})
	if string(out) != "destination: anywhere\n" {
		t.Errorf("anywhere must write as the scalar, got %q", out)
	}
}

func TestQuestValidate_RefusesABadDestination(t *testing.T) {
	q := Quest{QuestId: 1, Name: "x", Steps: []QuestStep{
		{Id: "start", Destination: StepDestination{Room: 1, Area: "a"}}, {Id: "end"},
	}}
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("want a destination refusal, got %v", err)
	}
}
```

- [ ] **Step 2:** `go test ./internal/quests/ -run 'StepDestination|BadDestination'`. Expected: build
  failure (`StepDestination` undefined).

- [ ] **Step 3: Implement** `internal/quests/destination.go`:

```go
package quests

import (
	"fmt"
	"slices"
)

// DestinationAnywhere is the scalar a step writes when it can be done
// anywhere (#375 phase 4): no marker, and no unknown-destination warning.
const DestinationAnywhere = "anywhere"

// StepDestination is where a quest step points the minimap and the quest
// log's destination line (#375 phase 4). The zero value means no block: the
// destination is inferred from the step's triggers. Shapes: anywhere; room;
// area; zone; or mob, alone or with one of room, area, zone as its fallback.
// questengine.ResolveDestination resolves it.
type StepDestination struct {
	Anywhere bool   `json:"anywhere,omitempty"`
	Room     int    `json:"room,omitempty"`
	Mob      int    `json:"mob,omitempty"`
	Area     string `json:"area,omitempty"`
	Zone     string `json:"zone,omitempty"`
}

// destinationFields is the mapping form on disk.
type destinationFields struct {
	Room int    `yaml:"room,omitempty"`
	Mob  int    `yaml:"mob,omitempty"`
	Area string `yaml:"area,omitempty"`
	Zone string `yaml:"zone,omitempty"`
}

var destinationKeys = []string{"room", "mob", "area", "zone"}

// IsZero reports whether the step declares no destination.
func (d StepDestination) IsZero() bool { return d == StepDestination{} }

// UnmarshalYAML takes the scalar anywhere or a mapping of room, mob, area and
// zone; any other word or key refuses the file.
func (d *StepDestination) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var word string
	if err := unmarshal(&word); err == nil {
		if word != DestinationAnywhere {
			return fmt.Errorf("destination %q: the only word a destination takes is %q; otherwise give room, mob, area or zone", word, DestinationAnywhere)
		}
		*d = StepDestination{Anywhere: true}
		return nil
	}
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("destination: want %q or a mapping of room, mob, area, zone: %w", DestinationAnywhere, err)
	}
	for k := range raw {
		if !slices.Contains(destinationKeys, k) {
			return fmt.Errorf("destination: unknown key %q (use room, mob, area or zone)", k)
		}
	}
	var f destinationFields
	if err := unmarshal(&f); err != nil {
		return err
	}
	*d = StepDestination{Room: f.Room, Mob: f.Mob, Area: f.Area, Zone: f.Zone}
	return nil
}

// MarshalYAML writes anywhere as the scalar and anything else as the mapping.
func (d StepDestination) MarshalYAML() (interface{}, error) {
	if d.Anywhere {
		return DestinationAnywhere, nil
	}
	return destinationFields{Room: d.Room, Mob: d.Mob, Area: d.Area, Zone: d.Zone}, nil
}

// Validate checks the shape. Existence of the room, mob, area and zone is
// ValidateQuestRefs' job.
func (d StepDestination) Validate() error {
	places := 0
	for _, set := range []bool{d.Room != 0, d.Area != "", d.Zone != ""} {
		if set {
			places++
		}
	}
	switch {
	case d.Anywhere && (places > 0 || d.Mob != 0):
		return fmt.Errorf("anywhere takes no room, mob, area or zone")
	case d.Room < 0 || d.Mob < 0:
		return fmt.Errorf("room and mob must be positive ids")
	case places > 1:
		return fmt.Errorf("names %d of room, area and zone; give one (a mob may add one as its fallback)", places)
	}
	return nil
}
```

In `internal/quests/quests.go` add the field after `Hint` (keep `MapTarget`/`MapTargetMob` until
Task 9):

```go
	// Destination is where the minimap and the quest log point during this
	// step (#375 phase 4); see StepDestination. Replaces MapTarget and
	// MapTargetMob, which go once every quest file is converted.
	Destination StepDestination `yaml:"destination,omitempty" json:"destination,omitempty"`
```

and in `Validate`'s "Check for duplicate step IDs" loop, after `seen[s.Id] = true`:

```go
		if err := s.Destination.Validate(); err != nil {
			return fmt.Errorf("quest %d (%s): step %q destination: %w", r.QuestId, r.Name, s.Id, err)
		}
```

- [ ] **Step 4:** `go test ./internal/quests/`. Expected: PASS, including
  `TestRoundTrip_MarshalFixedPointEveryLiveFile`. If `omitempty` does not drop the zero value
  (yaml.v2 reads `IsZero`), the round-trip test fails on every file; fix before going on.
- [ ] **Step 5:** Commit `internal/quests/destination.go internal/quests/destination_test.go internal/quests/quests.go`:
  `feat(quests): destination block on quest steps (#375 p4)`.

---

### Task 2: `areas.ZonesOf` and `areas.RoomsIn`

**Files:** `internal/areas/areas.go`, `internal/areas/loader.go`, `internal/areas/areas_test.go`,
`internal/areas/loader_test.go`.

- [ ] **Step 1: Failing test** appended to `internal/areas/loader_test.go`:

```go
func TestZonesOfAndRoomsIn(t *testing.T) {
	restore := InstallForTest([]Area{
		{Key: "grove", Name: "the Grove", Zones: []string{"thornwall"}, Rooms: []int{5313, 5311}},
	}, testRooms())
	defer restore()
	if got := ZonesOf("grove"); !reflect.DeepEqual(got, []string{"pothole_coulee", "thornwall"}) {
		t.Errorf("ZonesOf(grove) = %v", got)
	}
	if got := RoomsIn("grove", "pothole_coulee"); !reflect.DeepEqual(got, []int{5311, 5313}) {
		t.Errorf("RoomsIn(grove, pothole_coulee) = %v", got)
	}
	if got := RoomsIn("grove", "nowhere"); got != nil {
		t.Errorf("RoomsIn on a zone the area misses = %v, want nil", got)
	}
	if ZonesOf("nope") != nil || RoomsIn("nope", "thornwall") != nil {
		t.Error("an unknown key has no zones and no rooms")
	}
}
```

- [ ] **Step 2:** `go test ./internal/areas/ -run ZonesOfAndRoomsIn`. Expected: build failure.
- [ ] **Step 3: Implement.** In `areas.go` the registry gains two maps and `add` takes the room
  zones:

```go
type registry struct {
	areas  map[string]Area
	rooms  map[string]map[int]bool    // key -> template room ids
	byRoom map[int][]string           // template room id -> area keys, sorted
	zones  map[string][]string        // key -> zones its rooms lie in, sorted (#375 phase 4)
	inZone map[string]map[string][]int // key -> zone -> its rooms there, sorted
}

func newRegistry() *registry {
	return &registry{areas: map[string]Area{}, rooms: map[string]map[int]bool{}, byRoom: map[int][]string{},
		zones: map[string][]string{}, inZone: map[string]map[string][]int{}}
}

func (r *registry) add(a Area, set map[int]bool, roomZones map[int]string) {
	r.areas[a.Key] = a
	r.rooms[a.Key] = set
	byZone := map[string][]int{}
	for id := range set {
		r.byRoom[id] = append(r.byRoom[id], a.Key)
		sort.Strings(r.byRoom[id])
		z := roomZones[id]
		byZone[z] = append(byZone[z], id)
	}
	zs := make([]string, 0, len(byZone))
	for z, ids := range byZone {
		sort.Ints(ids)
		zs = append(zs, z)
	}
	sort.Strings(zs)
	r.zones[a.Key] = zs
	r.inZone[a.Key] = byZone
}
```

`loadDir` calls `r.add(a, set, roomZones)`; `InstallForTest` calls `r.add(a, set, roomZones)`;
`TestIndexAndContains` passes `testRooms()` as the third argument. In `loader.go`, after `Rooms`:

```go
// ZonesOf lists the zones the area's rooms lie in, sorted (#375 phase 4: the
// minimap routes to the nearest of them). Shared: do not change it.
func ZonesOf(key string) []string { return current().zones[key] }

// RoomsIn lists the area's template rooms in zone, sorted, for the minimap's
// wash. Shared: do not change it. Nil when the area has none there.
func RoomsIn(key, zone string) []int { return current().inZone[key][zone] }
```

- [ ] **Step 4:** `go test ./internal/areas/`. Expected: PASS.
- [ ] **Step 5:** Commit the four files: `feat(areas): zones and per-zone rooms of an area (#375 p4)`.

---

### Task 3: `rooms.SpawnRooms`

**Files:** Create `internal/rooms/spawn_rooms.go`, `internal/rooms/spawn_rooms_test.go`; modify
`internal/rooms/save_and_load.go`.

- [ ] **Step 1: Failing tests** (`internal/rooms/spawn_rooms_test.go`):

```go
package rooms

import (
	"reflect"
	"testing"
)

func TestSpawnRooms_IndexReplaceAndDrop(t *testing.T) {
	t.Cleanup(func() {
		deleteTemplateSpawns(920001)
		deleteTemplateSpawns(920002)
	})
	setTemplateSpawns(920001, []SpawnInfo{{MobId: 9901}, {MobId: 9902}, {MobId: 9901}, {ItemId: 5}})
	setTemplateSpawns(920002, []SpawnInfo{{MobId: 9901}})
	if got := SpawnRooms(9901); !reflect.DeepEqual(got, []int{920001, 920002}) {
		t.Fatalf("SpawnRooms(9901) = %v", got)
	}
	shared := SpawnRooms(9901)

	// A builder save replaces the room's spawns.
	setTemplateSpawns(920001, []SpawnInfo{{MobId: 9902}})
	if got := SpawnRooms(9901); !reflect.DeepEqual(got, []int{920002}) {
		t.Errorf("after the replace SpawnRooms(9901) = %v", got)
	}
	if !reflect.DeepEqual(shared, []int{920001, 920002}) {
		t.Errorf("a slice handed out earlier changed under its reader: %v", shared)
	}

	deleteTemplateSpawns(920002)
	if got := SpawnRooms(9901); got != nil {
		t.Errorf("after the delete SpawnRooms(9901) = %v, want nil", got)
	}
	if got := SpawnRooms(9902); !reflect.DeepEqual(got, []int{920001}) {
		t.Errorf("SpawnRooms(9902) = %v", got)
	}
}

func TestSpawnRooms_SkipsEphemeralRooms(t *testing.T) {
	id := ephemeralRoomIdMinimum + 7
	setTemplateSpawns(id, []SpawnInfo{{MobId: 9903}})
	t.Cleanup(func() { deleteTemplateSpawns(id) })
	if got := SpawnRooms(9903); got != nil {
		t.Errorf("an ephemeral room was indexed: %v", got)
	}
}

func TestSpawnRooms_FollowsDeleteRoomTemplate(t *testing.T) {
	const roomId = 920003
	restore := setupPrepareRoom(t, roomId)
	defer restore()
	setTemplateZone(roomId, "preparezone")
	setTemplateSpawns(roomId, []SpawnInfo{{MobId: 9904}})
	t.Cleanup(func() { deleteTemplateZone(roomId); deleteTemplateSpawns(roomId) })

	if err := DeleteRoomTemplate(roomId); err != nil {
		t.Fatalf("DeleteRoomTemplate: %v", err)
	}
	if got := SpawnRooms(9904); got != nil {
		t.Errorf("a deleted room still spawns: %v", got)
	}
}
```

- [ ] **Step 2:** `go test ./internal/rooms/ -run SpawnRooms`. Expected: build failure.
- [ ] **Step 3: Implement** `internal/rooms/spawn_rooms.go`:

```go
package rooms

import (
	"slices"
	"sync"
)

// The mob spawn index (#375 phase 4): which template rooms' spawninfo place
// each mob template, so a quest destination can fall back to where an NPC
// lives. Built in loadAllRoomZones, replaced for a room by SaveRoomTemplate,
// dropped by DeleteRoomTemplate. A move or rename does not change a room's
// spawns. Slices are replaced, never changed in place, so a slice handed to
// a reader stays valid.
var (
	spawnMu    sync.RWMutex
	spawnRooms = map[int][]int{} // mob template id -> template room ids, sorted
	roomSpawns = map[int][]int{} // template room id -> mob template ids it places
)

func setTemplateSpawns(roomId int, spawns []SpawnInfo) {
	if IsEphemeralRoomId(roomId) {
		return
	}
	spawnMu.Lock()
	defer spawnMu.Unlock()
	clearRoomSpawnsLocked(roomId)
	var here []int
	for _, si := range spawns {
		if si.MobId <= 0 || slices.Contains(here, si.MobId) {
			continue
		}
		here = append(here, si.MobId)
		ids := append(slices.Clone(spawnRooms[si.MobId]), roomId)
		slices.Sort(ids)
		spawnRooms[si.MobId] = ids
	}
	if len(here) > 0 {
		roomSpawns[roomId] = here
	}
}

func deleteTemplateSpawns(roomId int) {
	spawnMu.Lock()
	defer spawnMu.Unlock()
	clearRoomSpawnsLocked(roomId)
}

func clearRoomSpawnsLocked(roomId int) {
	for _, mobId := range roomSpawns[roomId] {
		ids := slices.DeleteFunc(slices.Clone(spawnRooms[mobId]), func(id int) bool { return id == roomId })
		if len(ids) == 0 {
			delete(spawnRooms, mobId)
		} else {
			spawnRooms[mobId] = ids
		}
	}
	delete(roomSpawns, roomId)
}

// SpawnRooms lists the template rooms whose spawninfo place mobId, sorted;
// nil when none does. The slice is shared: do not change it.
func SpawnRooms(mobId int) []int {
	spawnMu.RLock()
	defer spawnMu.RUnlock()
	return spawnRooms[mobId]
}
```

Wire it in `save_and_load.go`:
- `loadAllRoomZones`, after `setTemplateZone(loadedRoom.RoomId, loadedRoom.Zone)`:
  `setTemplateSpawns(loadedRoom.RoomId, loadedRoom.SpawnInfo)`
- `SaveRoomTemplate`, after `InvalidateTemplateCache(roomTpl.RoomId)`:
  `setTemplateSpawns(roomTpl.RoomId, roomTpl.SpawnInfo)`
- `DeleteRoomTemplate`, after `deleteTemplateZone(roomId)`: `deleteTemplateSpawns(roomId)`

- [ ] **Step 4:** `go test ./internal/rooms/`. Expected: PASS. Prove the delete test can fail:
  comment out the `deleteTemplateSpawns(roomId)` line in `DeleteRoomTemplate`, run, see
  `TestSpawnRooms_FollowsDeleteRoomTemplate` fail, restore the line.
- [ ] **Step 5:** Commit the three files: `feat(rooms): mob spawn index (#375 p4)`.

---

### Task 4: `mapper.NearestOf`

**Files:** Create `internal/mapper/mapper.nearest.go`, `internal/mapper/mapper.nearest_test.go`;
modify `internal/mapper/mapper.crosszone.go`.

- [ ] **Step 1: Failing tests** (`internal/mapper/mapper.nearest_test.go`):

```go
package mapper

import "testing"

// A 3x3 grid, ids 1..9 row by row, every neighbour linked both ways:
//
//	1 2 3
//	4 5 6
//	7 8 9
func buildGridMapper() *mapper {
	m := NewMapper(1)
	m.crawledRooms = map[int]*mapNode{}
	for id := 1; id <= 9; id++ {
		x, y := (id-1)%3, (id-1)/3
		exits := map[string]nodeExit{}
		if x > 0 {
			exits["west"] = nodeExit{RoomId: id - 1}
		}
		if x < 2 {
			exits["east"] = nodeExit{RoomId: id + 1}
		}
		if y > 0 {
			exits["north"] = nodeExit{RoomId: id - 3}
		}
		if y < 2 {
			exits["south"] = nodeExit{RoomId: id + 3}
		}
		m.crawledRooms[id] = &mapNode{RoomId: id, Pos: positionDelta{x: x, y: y}, Exits: exits}
	}
	return m
}

func inSet(ids ...int) func(int) bool {
	s := map[int]bool{}
	for _, id := range ids {
		s[id] = true
	}
	return func(id int) bool { return s[id] }
}

func TestNearestIn_PicksTheClosestMember(t *testing.T) {
	m := buildGridMapper()
	pin, next, exit, ok := m.nearestIn(1, inSet(9, 3))
	if !ok || pin != 3 || next != 2 || exit != "east" {
		t.Fatalf("from 1 toward {3,9}: got pin %d next %d %q ok %v; want 3 via 2 east", pin, next, exit, ok)
	}
}

func TestNearestIn_MatchesFindPathDistance(t *testing.T) {
	m := buildGridMapper()
	for goal := 2; goal <= 9; goal++ {
		path, err := m.findPath(1, goal)
		if err != nil {
			t.Fatal(err)
		}
		pin, _, _, ok := m.nearestIn(1, inSet(goal))
		if !ok || pin != goal {
			t.Fatalf("goal %d: nearestIn found %d", goal, pin)
		}
		if got := m.distanceForTest(1, goal); got != len(path) {
			t.Errorf("goal %d: breadth-first distance %d, A* path %d", goal, got, len(path))
		}
	}
}

func TestNearestIn_TiesResolveTheSameWay(t *testing.T) {
	m := buildGridMapper()
	// 2 and 4 are both one step from 1; exits are taken in name order, east before south.
	for i := 0; i < 20; i++ {
		if pin, _, exit, _ := m.nearestIn(1, inSet(2, 4)); pin != 2 || exit != "east" {
			t.Fatalf("tie went to %d via %q", pin, exit)
		}
	}
}

func TestNearestIn_NoneAndSelf(t *testing.T) {
	m := buildGridMapper()
	if _, _, _, ok := m.nearestIn(1, inSet(42)); ok {
		t.Error("a set with no reachable room must not resolve")
	}
	if pin, _, _, ok := m.nearestIn(1, inSet(1, 5)); !ok || pin != 5 {
		t.Errorf("the start room is never the answer: got %d %v", pin, ok)
	}
	if _, _, _, ok := m.nearestIn(77, inSet(5)); ok {
		t.Error("a start room the map lacks must not resolve")
	}
}

func TestNearestZoneHop_FewestZones(t *testing.T) {
	npStub(t)
	l, ok := nearestZoneHop("Docks", []string{"Noble", "Old Quarter"})
	if !ok || l.toZone != "Old Quarter" || l.exitRoom != 5520 {
		t.Fatalf("want the Old Quarter border, got %+v %v", l, ok)
	}
	if _, ok := nearestZoneHop("Docks", []string{"Docks"}); ok {
		t.Error("the start zone is not a hop")
	}
	if _, ok := nearestZoneHop("Docks", nil); ok {
		t.Error("no zones, no hop")
	}
}
```

and a test helper in the same file:

```go
// distanceForTest is the breadth-first step count from a to b (-1 if none).
func (r *mapper) distanceForTest(a, b int) int {
	dist := map[int]int{a: 0}
	queue := []int{a}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == b {
			return dist[cur]
		}
		for _, ex := range r.crawledRooms[cur].Exits {
			if _, seen := dist[ex.RoomId]; !seen {
				dist[ex.RoomId] = dist[cur] + 1
				queue = append(queue, ex.RoomId)
			}
		}
	}
	return -1
}
```

- [ ] **Step 2:** `go test ./internal/mapper/ -run 'NearestIn|NearestZoneHop'`. Expected: build failure.
- [ ] **Step 3: Implement** `internal/mapper/mapper.nearest.go`:

```go
package mapper

import (
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// nearestIn is a breadth-first search from fromRoomId over this map's rooms
// that stops at the first room in accepts (#375 phase 4). Every exit costs 1,
// as in findPath, so the first room found is a nearest one; exits are taken
// in name order so a tie always goes the same way. It returns that room, the
// first room on the way and the exit to it. fromRoomId itself is never the
// answer, and an exit to a room this map lacks (another zone) is not taken.
func (r *mapper) nearestIn(fromRoomId int, in func(roomId int) bool) (pin, nextRoomId int, exitName string, found bool) {
	if _, ok := r.crawledRooms[fromRoomId]; !ok {
		return 0, 0, "", false
	}
	type hop struct {
		room, first int
		exit        string
	}
	seen := map[int]bool{fromRoomId: true}
	var queue []hop
	expand := func(at hop) {
		node := r.crawledRooms[at.room]
		names := make([]string, 0, len(node.Exits))
		for name := range node.Exits {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			next := node.Exits[name].RoomId
			if seen[next] {
				continue
			}
			if _, ok := r.crawledRooms[next]; !ok {
				continue
			}
			seen[next] = true
			h := hop{room: next, first: at.first, exit: at.exit}
			if at.room == fromRoomId {
				h.first, h.exit = next, name
			}
			queue = append(queue, h)
		}
	}
	expand(hop{room: fromRoomId})
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if in(h.room) {
			return h.room, h.first, h.exit, true
		}
		expand(h)
	}
	return 0, 0, "", false
}

// NearestOf is where to head from fromRoomId toward the nearest room in
// accepts (#375 phase 4: quest destinations). Inside the start room's zone it
// searches the map and returns that room as pin with the first hop. When no
// room of that zone qualifies, it heads for the zone in zones with the fewest
// zone crossings, the way crossZoneHop does, and pin is 0: the room is not on
// the player's drawn map. found is false when neither works.
func NearestOf(fromRoomId int, in func(roomId int) bool, zones []string) (pin, nextRoomId int, exitName string, found bool) {
	start := time.Now()
	defer func() { util.TrackTime(`mapper.NearestOf()`, time.Since(start).Seconds()) }()

	fromRoom := rooms.LoadRoom(fromRoomId)
	if fromRoom == nil {
		return 0, 0, "", false
	}
	if m := GetMapper(fromRoomId); m != nil {
		if pin, next, exit, ok := m.nearestIn(fromRoomId, in); ok {
			return pin, next, exit, true
		}
	}
	link, ok := nearestZoneHop(fromRoom.Zone, zones)
	if !ok {
		return 0, 0, "", false
	}
	next, exit, ok := hopToward(fromRoomId, link)
	return 0, next, exit, ok
}
```

In `mapper.crosszone.go` replace the body of `nextZoneHop` and `crossZoneHop` (keep their doc
comments) and add `nearestZoneHop` and `hopToward`:

```go
func nextZoneHop(fromZone, toZone string) (zoneLink, bool) {
	if toZone == "" {
		return zoneLink{}, false
	}
	return nearestZoneHop(fromZone, []string{toZone})
}

// nearestZoneHop is nextZoneHop toward whichever of toZones is the fewest
// zone crossings away (#375 phase 4: an area or an NPC's rooms can span
// zones). fromZone itself and "" are never targets.
func nearestZoneHop(fromZone string, toZones []string) (zoneLink, bool) {
	want := map[string]bool{}
	for _, z := range toZones {
		if z != "" && z != fromZone {
			want[z] = true
		}
	}
	if fromZone == "" || len(want) == 0 {
		return zoneLink{}, false
	}
	ensureZoneGraph()

	zoneGraphMu.RLock()
	defer zoneGraphMu.RUnlock()

	// BFS over zones, remembering which first-leg link each frontier came from.
	type node struct {
		zone  string
		first zoneLink
	}
	queue := make([]node, 0, len(zoneGraph[fromZone]))
	seen := map[string]bool{fromZone: true}
	for _, l := range zoneGraph[fromZone] {
		if want[l.toZone] {
			return l, true // direct neighbour
		}
		if !seen[l.toZone] {
			seen[l.toZone] = true
			queue = append(queue, node{zone: l.toZone, first: l})
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, l := range zoneGraph[n.zone] {
			if want[l.toZone] {
				return n.first, true // keep the FIRST leg, not this one
			}
			if !seen[l.toZone] {
				seen[l.toZone] = true
				queue = append(queue, node{zone: l.toZone, first: n.first})
			}
		}
	}
	return zoneLink{}, false
}

func crossZoneHop(fromRoomId int, fromZone, toZone string) (nextRoomId int, exitName string, found bool) {
	link, ok := nextZoneHop(fromZone, toZone)
	if !ok {
		return 0, "", false
	}
	return hopToward(fromRoomId, link)
}

// hopToward is the first step from fromRoomId to the border room of link, or
// the crossing itself when standing on it.
func hopToward(fromRoomId int, link zoneLink) (nextRoomId int, exitName string, found bool) {
	if fromRoomId == link.exitRoom {
		return link.destRoom, link.exitName, true
	}
	path, err := GetPath(fromRoomId, link.exitRoom) // same zone, GetPath can do this
	if err != nil {
		return 0, "", false
	}
	return firstHop(path)
}
```

Note: the old `nextZoneHop` refused `fromZone == toZone` and `fromZone == ""`; `nearestZoneHop`
keeps both refusals, so `TestNextZoneHop_SameZoneAndUnreachable` stays green.

- [ ] **Step 4:** `go test ./internal/mapper/`. Expected: PASS (new and existing crosszone tests).
- [ ] **Step 5:** Commit the three files: `feat(mapper): NearestOf, nearest room of a set (#375 p4)`.

---

### Task 5: `ResolveDestination`

**Files:** Create `internal/questengine/destination.go`, `internal/questengine/destination_test.go`.

- [ ] **Step 1: Failing tests** (`internal/questengine/destination_test.go`):

```go
package questengine

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// destWorld stubs every seam ResolveDestination reads: room zones, live
// copies, the sight in each room, spawn rooms, names and the nearest search.
type destWorld struct {
	zones  map[int]string
	copies map[int][]liveCopy
	dark   map[int]messaging.SightDecision // room -> sight; absent = full
	spawns map[int][]int
	names  map[int]string
	near   func(from int, in func(int) bool, zones []string) (int, int, string, bool)
}

func installDestWorld(t *testing.T, w destWorld) {
	t.Helper()
	pz, pc, ps, psp, pn, pnear, ptpl := roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom
	roomZone = func(id int) string { return w.zones[id] }
	liveCopies = func(mobId int) []liveCopy { return w.copies[mobId] }
	sightIn = func(_ *characters.Character, roomId int) messaging.SightDecision {
		if s, ok := w.dark[roomId]; ok {
			return s
		}
		return messaging.SightFull
	}
	spawnRoomsOf = func(mobId int) []int { return w.spawns[mobId] }
	npcName = func(mobId int) string { return w.names[mobId] }
	templateRoom = func(id int) int { return id }
	nearestOf = w.near
	if nearestOf == nil {
		nearestOf = func(from int, in func(int) bool, zones []string) (int, int, string, bool) {
			for id := 1; id < 100000; id++ { // the lowest member, one hop "north"
				if id != from && in(id) {
					return id, id, "north", true
				}
			}
			return 0, 0, "", false
		}
	}
	t.Cleanup(func() {
		roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom = pz, pc, ps, psp, pn, pnear, ptpl
	})
}

func destCopy(t *testing.T, roomId int, hidden bool) liveCopy {
	t.Helper()
	c := characters.New()
	c.Awareness = awareness.NewMachine()
	if hidden {
		r := state.TransitionReason{Trigger: "destination_test"}
		if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r); err != nil {
			t.Fatal(err)
		}
		c.Awareness.ResolveConcealment(true, r)
	}
	return liveCopy{roomId: roomId, char: c}
}

func destEngine(steps []quests.QuestStep, triggers ...quests.TriggerDef) *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{QuestId: 700, Name: "Dest", Steps: append(steps, QuestStep{Id: "end"}), Triggers: triggers})
	return e
}

func destUser(roomId int) *users.UserRecord {
	u := users.NewTestUser(97001, "dest", "Dest", 97001)
	u.Character.RoomId = roomId
	return u
}

func gated(step string, t quests.TriggerDef) quests.TriggerDef {
	t.Conditions.Has = append(t.Conditions.Has, "700-"+step)
	t.Actions = []quests.ActionDef{{Grant: "700-end"}}
	return t
}

func TestResolveDestination_FinalStepAndUnknownQuest(t *testing.T) {
	installDestWorld(t, destWorld{})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Room: 5}}})
	if d := e.ResolveDestination(destUser(1), 700, "end"); d.Kind != DestNone {
		t.Errorf("final step: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 999, "start"); d.Kind != DestNone {
		t.Errorf("unknown quest: %+v", d)
	}
}

func TestResolveDestination_Anywhere(t *testing.T) {
	installDestWorld(t, destWorld{})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Anywhere: true}}},
		gated("start", quests.TriggerDef{Event: "room_enter", Room: 5}))
	if d := e.ResolveDestination(destUser(1), 700, "start"); d.Kind != DestAnywhere || d.Pin != 0 {
		t.Errorf("anywhere must win over inference and mark nothing: %+v", d)
	}
}

func TestResolveDestination_Room(t *testing.T) {
	installDestWorld(t, destWorld{zones: map[int]string{1: "z", 5: "z"}})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Room: 5}}})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestRoom || d.Pin != 5 || d.NextDir != "north" || d.Inside || d.ZoneRooms != nil {
		t.Errorf("room: %+v", d)
	}
	if d := e.ResolveDestination(destUser(5), 700, "start"); !d.Inside || d.Pin != 0 || d.NextDir != "" {
		t.Errorf("standing in the room: %+v", d)
	}
}

func TestResolveDestination_AreaAndZone(t *testing.T) {
	zones := map[int]string{1: "town", 20: "grove", 21: "grove", 22: "grove", 30: "moor"}
	installDestWorld(t, destWorld{zones: zones})
	t.Cleanup(areas.InstallForTest([]areas.Area{{Key: "g", Name: "the grove", Rooms: []int{20, 21, 30}}}, zones))

	e := destEngine([]QuestStep{
		{Id: "start", Destination: quests.StepDestination{Area: "g"}},
		{Id: "zone", Destination: quests.StepDestination{Zone: "grove"}},
	})
	d := e.ResolveDestination(destUser(21), 700, "start")
	if d.Kind != DestArea || d.Label != "the grove" || !d.Inside || !reflect.DeepEqual(d.ZoneRooms, []int{20, 21}) {
		t.Errorf("inside the area: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "start")
	if d.Inside || d.Pin != 20 || d.ZoneRooms != nil {
		t.Errorf("outside the area (town has none of it): %+v", d)
	}
	d = e.ResolveDestination(destUser(22), 700, "zone")
	if d.Kind != DestZone || d.Label != "grove" || !d.Inside || d.ZoneRooms != nil {
		t.Errorf("inside the zone: %+v", d)
	}
}

func TestResolveDestination_NearestPerceivedCopy(t *testing.T) {
	w := destWorld{
		zones:  map[int]string{1: "z", 4: "z", 6: "z", 8: "z", 9: "z"},
		copies: map[int][]liveCopy{},
		dark:   map[int]messaging.SightDecision{6: messaging.SightNone, 8: messaging.SightShapes},
		names:  map[int]string{50: "a wolf"},
	}
	w.copies[50] = []liveCopy{destCopy(t, 4, true), destCopy(t, 6, false), destCopy(t, 8, false), destCopy(t, 9, false)}
	installDestWorld(t, w)
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Mob: 50}}})
	d := e.ResolveDestination(destUser(1), 700, "start")
	// 4 is hidden, 6 is dark, 8 is shapes only: only 9 counts.
	if d.Kind != DestMob || d.Pin != 9 || d.Npc != "a wolf" || d.Fallback {
		t.Errorf("want the one perceived copy in 9: %+v", d)
	}
}

func TestResolveDestination_MobFallsBack(t *testing.T) {
	zones := map[int]string{1: "z", 4: "z", 7: "z", 12: "z", 13: "z"}
	w := destWorld{zones: zones, copies: map[int][]liveCopy{}, names: map[int]string{50: "Velk"},
		spawns: map[int][]int{50: {12, 13}}}
	w.copies[50] = []liveCopy{destCopy(t, 4, true)} // one copy, hidden
	installDestWorld(t, w)

	e := destEngine([]QuestStep{
		{Id: "start", Destination: quests.StepDestination{Mob: 50, Room: 7}},
		{Id: "spawn", Destination: quests.StepDestination{Mob: 50}},
		{Id: "none", Destination: quests.StepDestination{Mob: 51}},
	})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestRoom || d.Pin != 7 || d.Npc != "Velk" || !d.Fallback {
		t.Errorf("a hidden single NPC falls back to its room: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "spawn")
	if d.Kind != DestArea || d.Pin != 12 || !d.Fallback || !reflect.DeepEqual(d.ZoneRooms, []int{12, 13}) {
		t.Errorf("no declared fallback: the spawn rooms: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "none"); d.Kind != DestUnknown {
		t.Errorf("no copy, no fallback, no spawns: %+v", d)
	}
}

func TestResolveDestination_Inference(t *testing.T) {
	zones := map[int]string{1: "z", 5: "z", 6: "z", 9: "z"}
	installDestWorld(t, destWorld{zones: zones, spawns: map[int][]int{60: {9}}})
	e := destEngine([]QuestStep{{Id: "one"}, {Id: "mixed"}, {Id: "bare"}},
		gated("one", quests.TriggerDef{Event: "room_enter", Room: 5}),
		gated("one", quests.TriggerDef{Event: "command", Conditions: quests.Conditions{InRoom: 6}}),
		gated("mixed", quests.TriggerDef{Event: "room_enter", Room: 5}),
		gated("mixed", quests.TriggerDef{Event: "mob_death", Mob: 60}),
		gated("bare", quests.TriggerDef{Event: "command", Command: "sleep"}),
	)
	d := e.ResolveDestination(destUser(1), 700, "one")
	if d.Kind != DestRoom || !d.Inferred || d.Pin != 5 || !reflect.DeepEqual(d.ZoneRooms, []int{5, 6}) {
		t.Errorf("two rooms infer one room set: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "mixed")
	if d.Kind != DestRoom || d.Label != "" || d.Pin != 5 {
		t.Errorf("a room and a mob's spawn rooms: mixed kinds give a room set: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "bare"); d.Kind != DestUnknown || !d.Inferred {
		t.Errorf("no place anywhere: %+v", d)
	}
}
```

- [ ] **Step 2:** `go test ./internal/questengine/ -run ResolveDestination`. Expected: build failure.
- [ ] **Step 3: Implement** `internal/questengine/destination.go`:

```go
package questengine

import (
	"slices"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// DestKind is what a quest step's destination resolved to (#375 phase 4).
type DestKind string

const (
	DestNone     DestKind = ""         // the final step, or no such quest or step
	DestAnywhere DestKind = "anywhere" // the step can be done anywhere
	DestRoom     DestKind = "room"
	DestMob      DestKind = "mob" // live copies of an NPC the player perceives
	DestArea     DestKind = "area"
	DestZone     DestKind = "zone"
	DestUnknown  DestKind = "unknown" // nothing resolved
)

// Destination is where a quest step points for one player right now.
type Destination struct {
	Kind      DestKind
	Label     string // area or zone name; "" otherwise
	Npc       string // the NPC's name for a mob destination, also when it fell back
	Fallback  bool   // a mob destination showing its fallback set (no copy perceived)
	ZoneRooms []int  // the set's template rooms in the player's zone, for the map wash
	Pin       int    // nearest set room inside the player's zone; 0 if none
	Inside    bool   // the player's room is in the set
	NextRoom  int
	NextDir   string
	Inferred  bool // read from the step's triggers, not a destination block
}

// liveCopy is one live instance of a mob template.
type liveCopy struct {
	roomId int // the room it stands in (an instance room id when instanced)
	char   *characters.Character
}

// The world seams ResolveDestination reads; package vars so tests stub them.
var (
	liveCopies = func(mobId int) []liveCopy {
		var out []liveCopy
		for _, instId := range mobs.GetAllMobInstanceIds() {
			m := mobs.GetInstance(instId)
			if m == nil || int(m.MobId) != mobId || m.Character.RoomId < 1 {
				continue
			}
			out = append(out, liveCopy{roomId: m.Character.RoomId, char: &m.Character})
		}
		return out
	}
	// sightIn is what viewer would make out standing in roomId.
	sightIn = func(viewer *characters.Character, roomId int) messaging.SightDecision {
		r := rooms.LoadRoom(roomId)
		if r == nil {
			return messaging.SightNone
		}
		return messaging.ParticipantSight(viewer, r)
	}
	templateRoom = func(roomId int) int { id, _ := rooms.OriginalRoomId(roomId); return id }
	roomZone     = rooms.TemplateRoomZone
	spawnRoomsOf = rooms.SpawnRooms
	npcName      = func(mobId int) string {
		if m := mobs.GetMobSpec(mobs.MobId(mobId)); m != nil {
			return m.Character.Name
		}
		return ""
	}
	nearestOf = mapper.NearestOf
)

// destSet is a destination's rooms: a membership test on template room ids
// and the zones they touch, never a full room list per call.
type destSet struct {
	kind      DestKind
	label     string
	npc       string
	fallback  bool
	in        func(templateRoomId int) bool
	zones     []string
	zoneRooms func(zone string) []int // nil: no wash (a zone, a single room)
}

func roomSet(id int) destSet {
	return destSet{kind: DestRoom, in: func(r int) bool { return r == id }, zones: []string{roomZone(id)}}
}

func areaSet(key string) destSet {
	a, _ := areas.Get(key)
	return destSet{kind: DestArea, label: a.Name, in: func(r int) bool { return areas.Contains(key, r) },
		zones: areas.ZonesOf(key), zoneRooms: func(z string) []int { return areas.RoomsIn(key, z) }}
}

func zoneSet(zone string) destSet {
	return destSet{kind: DestZone, label: zone, in: func(r int) bool { return roomZone(r) == zone }, zones: []string{zone}}
}

// roomsSet is a set of listed rooms: an NPC's rooms, spawn rooms, an inferred union.
func roomsSet(kind DestKind, ids []int) destSet {
	member := make(map[int]bool, len(ids))
	var zones []string
	for _, id := range ids {
		member[id] = true
		if z := roomZone(id); z != "" && !slices.Contains(zones, z) {
			zones = append(zones, z)
		}
	}
	sort.Strings(zones)
	return destSet{kind: kind, in: func(r int) bool { return member[r] }, zones: zones,
		zoneRooms: func(z string) []int {
			var out []int
			for _, id := range ids {
				if roomZone(id) == z {
					out = append(out, id)
				}
			}
			return out
		}}
}

// perceivedRooms lists the template rooms of the live copies of mobId the
// viewer perceives at full sight, judged as if standing in each copy's room:
// Character.Perceives (hidden) and ParticipantSight == SightFull (owner
// rulings 4 and 5). The sight is judged once per room.
func perceivedRooms(viewer *characters.Character, mobId int) []int {
	sight := map[int]bool{}
	seen := map[int]bool{}
	var out []int
	for _, c := range liveCopies(mobId) {
		if !viewer.Perceives(c.char) {
			continue
		}
		ok, judged := sight[c.roomId]
		if !judged {
			ok = sightIn(viewer, c.roomId) == messaging.SightFull
			sight[c.roomId] = ok
		}
		if !ok {
			continue
		}
		if tpl := templateRoom(c.roomId); !seen[tpl] {
			seen[tpl] = true
			out = append(out, tpl)
		}
	}
	sort.Ints(out)
	return out
}

// mobSet is a mob destination: its perceived copies, else the fallback the
// step declares (room, area or zone), else its spawn rooms (owner ruling 6).
func mobSet(viewer *characters.Character, mobId int, fallback quests.StepDestination) (destSet, bool) {
	npc := npcName(mobId)
	if ids := perceivedRooms(viewer, mobId); len(ids) > 0 {
		s := roomsSet(DestMob, ids)
		s.npc = npc
		return s, true
	}
	var s destSet
	switch {
	case fallback.Room > 0:
		s = roomSet(fallback.Room)
	case fallback.Area != "":
		s = areaSet(fallback.Area)
	case fallback.Zone != "":
		s = zoneSet(fallback.Zone)
	default:
		ids := spawnRoomsOf(mobId)
		if len(ids) == 0 {
			return destSet{}, false
		}
		s = roomsSet(DestArea, ids)
	}
	s.npc, s.fallback = npc, true
	return s, true
}

func declaredSet(viewer *characters.Character, d quests.StepDestination) (destSet, bool) {
	switch {
	case d.Mob > 0:
		return mobSet(viewer, d.Mob, d)
	case d.Room > 0:
		return roomSet(d.Room), true
	case d.Area != "":
		return areaSet(d.Area), true
	case d.Zone != "":
		return zoneSet(d.Zone), true
	}
	return destSet{}, false
}

// stepPlaces is what the triggers gated on a step name as places, the
// inference input (owner ruling 7).
type stepPlaces struct {
	rooms []int
	mobs  []int
	areas []string
	zones []string
}

func (p stepPlaces) empty() bool {
	return len(p.rooms)+len(p.mobs)+len(p.areas)+len(p.zones) == 0
}

func placesOf(q *quests.Quest, step string) stepPlaces {
	token := quests.PartsToToken(q.QuestId, step)
	var p stepPlaces
	for i := range q.Triggers {
		t := &q.Triggers[i]
		if !slices.Contains(t.Conditions.Has, token) {
			continue
		}
		for _, r := range []int{t.Room, t.Conditions.InRoom} {
			if r > 0 && !slices.Contains(p.rooms, r) {
				p.rooms = append(p.rooms, r)
			}
		}
		if t.Mob > 0 && !slices.Contains(p.mobs, t.Mob) {
			p.mobs = append(p.mobs, t.Mob)
		}
		if t.Area != "" && !slices.Contains(p.areas, t.Area) {
			p.areas = append(p.areas, t.Area)
		}
		if t.Zone != "" && !slices.Contains(p.zones, t.Zone) {
			p.zones = append(p.zones, t.Zone)
		}
	}
	return p
}

// inferredSet joins every place the step's triggers name. One kind keeps its
// kind (and its label when all parts share it); mixed kinds give a room set.
func inferredSet(viewer *characters.Character, p stepPlaces) (destSet, bool) {
	var parts []destSet
	if len(p.rooms) > 0 {
		parts = append(parts, roomsSet(DestRoom, p.rooms))
	}
	for _, m := range p.mobs {
		if s, ok := mobSet(viewer, m, quests.StepDestination{}); ok {
			parts = append(parts, s)
		}
	}
	for _, a := range p.areas {
		parts = append(parts, areaSet(a))
	}
	for _, z := range p.zones {
		parts = append(parts, zoneSet(z))
	}
	switch len(parts) {
	case 0:
		return destSet{}, false
	case 1:
		return parts[0], true
	}
	u := destSet{kind: parts[0].kind, label: parts[0].label, npc: parts[0].npc, fallback: parts[0].fallback}
	for _, s := range parts[1:] {
		if s.kind != u.kind {
			u.kind = DestRoom
		}
		if s.label != u.label {
			u.label = ""
		}
		if s.npc != u.npc {
			u.npc = ""
		}
		u.fallback = u.fallback && s.fallback
	}
	if u.kind == DestRoom {
		u.label, u.npc, u.fallback = "", "", false
	}
	u.in = func(r int) bool {
		for _, s := range parts {
			if s.in(r) {
				return true
			}
		}
		return false
	}
	u.zoneRooms = func(z string) []int {
		var out []int
		for _, s := range parts {
			if s.zoneRooms != nil { // a zone part has none: the banner carries it
				out = append(out, s.zoneRooms(z)...)
			}
		}
		slices.Sort(out)
		return slices.Compact(out)
	}
	for _, s := range parts {
		for _, z := range s.zones {
			if !slices.Contains(u.zones, z) {
				u.zones = append(u.zones, z)
			}
		}
	}
	sort.Strings(u.zones)
	return u, true
}

// ResolveDestination is where questId's step points user right now (#375
// phase 4): the declared destination block, else what the step's triggers
// name, else unknown. Spec section 2.
func (e *Engine) ResolveDestination(user *users.UserRecord, questId int, step string) Destination {
	q := e.quests[questId]
	if q == nil || user == nil {
		return Destination{}
	}
	idx := slices.IndexFunc(q.Steps, func(s quests.QuestStep) bool { return s.Id == step })
	if idx < 0 || idx == len(q.Steps)-1 {
		return Destination{}
	}
	s := q.Steps[idx]
	if s.Destination.Anywhere {
		return Destination{Kind: DestAnywhere}
	}
	var set destSet
	var ok bool
	inferred := s.Destination.IsZero()
	if inferred {
		set, ok = inferredSet(&user.Character, placesOf(q, s.Id))
	} else {
		set, ok = declaredSet(&user.Character, s.Destination)
	}
	if !ok {
		return Destination{Kind: DestUnknown, Inferred: inferred}
	}
	return finishDestination(set, user.Character.RoomId, inferred)
}

func finishDestination(set destSet, playerRoom int, inferred bool) Destination {
	from := templateRoom(playerRoom)
	d := Destination{Kind: set.kind, Label: set.label, Npc: set.npc, Fallback: set.fallback, Inferred: inferred}
	if set.zoneRooms != nil {
		if zr := set.zoneRooms(roomZone(from)); len(zr) > 1 || (len(zr) == 1 && set.kind == DestArea) {
			d.ZoneRooms = zr
		}
	}
	if set.in(from) {
		d.Inside = true
		return d
	}
	in := func(r int) bool { return set.in(templateRoom(r)) }
	if pin, next, exit, ok := nearestOf(playerRoom, in, set.zones); ok {
		d.Pin, d.NextRoom, d.NextDir = templateRoom(pin), next, exit
	}
	return d
}
```

Inferred rooms always go through `roomsSet`, so a lone inferred room still gets no wash:
`finishDestination` sends `ZoneRooms` only for two or more rooms, or for an area.

- [ ] **Step 4:** `go test ./internal/questengine/ -run ResolveDestination`. Expected: PASS.
  Prove the perceive filter can fail: in `perceivedRooms` change `== messaging.SightFull` to
  `!= messaging.SightNone`, run, see `NearestPerceivedCopy` fail (pin 8), restore.
- [ ] **Step 5:** `go build ./... && go test ./internal/questengine/`. Expected: PASS (the old
  resolver still exists alongside).
- [ ] **Step 6:** Commit both files: `feat(questengine): ResolveDestination (#375 p4)`.

---

### Task 6: `GuideLine`

**Files:** Create `internal/questengine/guide.go`, `internal/questengine/guide_test.go`.

- [ ] **Step 1: Failing test:**

```go
package questengine

import "testing"

func TestGuideLine_EveryCase(t *testing.T) {
	cases := []struct {
		d    Destination
		want string
	}{
		{Destination{}, ""},
		{Destination{Kind: DestAnywhere}, "You can do this anywhere."},
		{Destination{Kind: DestUnknown}, "Destination unknown."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "north"}, "Head north."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "up"}, "Head up."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "gate"}, "Take the gate."},
		{Destination{Kind: DestArea, Label: "the unquiet grove", Inside: true}, "You are in the unquiet grove."},
		{Destination{Kind: DestZone, Label: "Pothole Coulee", Inside: true}, "You are in Pothole Coulee."},
		{Destination{Kind: DestArea, Npc: "a wolf", Fallback: true, Inside: true}, "You are where a wolf is usually found."},
		{Destination{Kind: DestMob, Npc: "Velk", Inside: true}, "You are where you need to be."},
		{Destination{Kind: DestRoom, Inside: true}, "You are where you need to be."},
		{Destination{Kind: DestRoom}, "You can't find a way there from here."},
	}
	for _, c := range cases {
		if got := GuideLine(c.d); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.d, got, c.want)
		}
	}
}
```

- [ ] **Step 2:** run it; expected: build failure.
- [ ] **Step 3: Implement** `guide.go`:

```go
package questengine

import "github.com/GoMudEngine/GoMud/internal/mapper"

// GuideLine is the one destination line (#375 phase 4, spec section 4): the
// `quest` command prints it and the web quest panel shows it, so both say the
// same words. Plain text: the web panel escapes markup.
func GuideLine(d Destination) string {
	switch d.Kind {
	case DestNone:
		return ""
	case DestAnywhere:
		return "You can do this anywhere."
	case DestUnknown:
		return "Destination unknown."
	}
	if d.Inside {
		switch {
		case d.Label != "":
			return "You are in " + d.Label + "."
		case d.Fallback && d.Npc != "":
			return "You are where " + d.Npc + " is usually found."
		}
		return "You are where you need to be."
	}
	if d.NextDir == "" {
		return "You can't find a way there from here."
	}
	if mapper.IsCompassDirection(d.NextDir) {
		return "Head " + d.NextDir + "."
	}
	return "Take the " + d.NextDir + "."
}
```

- [ ] **Step 4:** run; expected PASS. **Step 5:** commit both files:
  `feat(questengine): GuideLine, the shared destination line (#375 p4)`.

---

### Task 7: `Destinations` rule, boot door, CI sweep

**Files:** Create `internal/questengine/destinations_rule.go`,
`internal/questengine/destinations_rule_test.go`; modify `main_quests.go`, `main.go`,
`quest_loop_smoke_test.go`.

- [ ] **Step 1: Failing test** (`destinations_rule_test.go`):

```go
package questengine

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

func TestDestinations_Findings(t *testing.T) {
	installDestWorld(t, destWorld{spawns: map[int][]int{60: {9}}})
	q := &quests.Quest{QuestId: 701, Name: "Rule", Steps: []quests.QuestStep{
		{Id: "start", Destination: quests.StepDestination{Room: 5}},
		{Id: "lost"},                                                  // unknown
		{Id: "here", Destination: quests.StepDestination{Anywhere: true}}, // mistagged
		{Id: "wolf", Destination: quests.StepDestination{Mob: 60}},        // spawn rooms: fine
		{Id: "ghost", Destination: quests.StepDestination{Mob: 61}},       // no fallback, no spawns
		{Id: "guess"},                                                 // inferred
		{Id: "end"},                                                   // final: skipped
	}, Triggers: []quests.TriggerDef{
		{Event: "room_enter", Room: 7, Conditions: quests.Conditions{Has: []string{"701-here"}}},
		{Event: "mob_death", Mob: 60, Conditions: quests.Conditions{Has: []string{"701-guess"}}},
	}}
	secret := &quests.Quest{QuestId: 702, Name: "S", Secret: true, Steps: []quests.QuestStep{{Id: "x"}, {Id: "end"}}}

	fs, inferred := Destinations([]*quests.Quest{secret, q})
	var keys []string
	for _, f := range fs {
		keys = append(keys, f.Key())
	}
	want := []string{
		"walk-destination 701 step ghost",
		"walk-destination 701 step lost",
		"walk-destination-anywhere 701 step here",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("findings %v, want %v", keys, want)
	}
	if !reflect.DeepEqual(inferred, []string{"quest 701 step guess"}) {
		t.Errorf("inferred %v", inferred)
	}
}
```

- [ ] **Step 2:** run; expected build failure.
- [ ] **Step 3: Implement** `destinations_rule.go`:

```go
package questengine

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Destinations is the static destination rule (#375 phase 4, spec section 5),
// a walk rule: boot logs its findings as warnings and never refuses; the CI
// sweep ratchets them against loop-backlog.txt. Secret quests and each
// quest's final step are skipped. It returns the findings sorted by key, and
// "quest <id> step <step>" for each step whose destination is inferred.
//
//	walk-destination           the step resolves to unknown
//	walk-destination-anywhere  an anywhere step whose triggers name a place
func Destinations(qs []*quests.Quest) (findings []questloop.Finding, inferred []string) {
	sorted := slices.Clone(qs)
	slices.SortFunc(sorted, func(a, b *quests.Quest) int { return cmp.Compare(a.QuestId, b.QuestId) })
	for _, q := range sorted {
		if q == nil || q.Secret {
			continue
		}
		for i, s := range q.Steps {
			if i == len(q.Steps)-1 {
				continue
			}
			where := "step " + s.Id
			p := placesOf(q, s.Id)
			d := s.Destination
			switch {
			case d.Anywhere:
				if !p.empty() {
					findings = append(findings, questloop.Finding{Rule: "walk-destination-anywhere", QuestId: q.QuestId, Where: where,
						Msg: "marked anywhere, but a trigger on this step names a place; drop anywhere so the destination is inferred"})
				}
			case !d.IsZero():
				if d.Mob > 0 && d.Room == 0 && d.Area == "" && d.Zone == "" && len(spawnRoomsOf(d.Mob)) == 0 {
					findings = append(findings, questloop.Finding{Rule: "walk-destination", QuestId: q.QuestId, Where: where,
						Msg: fmt.Sprintf("mob %d has no fallback and no room spawns it; add room, area or zone", d.Mob)})
				}
			default:
				if staticInferable(p) {
					inferred = append(inferred, fmt.Sprintf("quest %d step %s", q.QuestId, s.Id))
				} else {
					findings = append(findings, questloop.Finding{Rule: "walk-destination", QuestId: q.QuestId, Where: where,
						Msg: "no destination and no trigger on this step names a place; give it destination: (room, mob, area or zone) or destination: anywhere"})
				}
			}
		}
	}
	slices.SortFunc(findings, func(a, b questloop.Finding) int { return cmp.Compare(a.Key(), b.Key()) })
	return findings, inferred
}

// staticInferable reports whether a step's trigger places resolve without a
// live world: any room, area or zone, or a mob some room spawns.
func staticInferable(p stepPlaces) bool {
	if len(p.rooms)+len(p.areas)+len(p.zones) > 0 {
		return true
	}
	for _, m := range p.mobs {
		if len(spawnRoomsOf(m)) > 0 {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4:** run; expected PASS.
- [ ] **Step 5: Boot door.** In `main_quests.go` add:

```go
// destinationDoor logs every quest step whose minimap destination is unknown
// or mistagged (#375 phase 4) as a warning, and the inferred count. It never
// refuses: the CI sweep ratchets these findings against the backlog.
func destinationDoor() int {
	return destinationDoorFrom(func() ([]questloop.Finding, []string) {
		all := quests.GetAllQuests()
		qs := make([]*quests.Quest, 0, len(all))
		for i := range all {
			qs = append(qs, &all[i])
		}
		return questengine.Destinations(qs)
	})
}

// destinationDoorFrom is destinationDoor over any source, for tests. It
// returns how many findings it logged.
func destinationDoorFrom(src func() ([]questloop.Finding, []string)) int {
	fs, inferred := src()
	for _, f := range fs {
		mudlog.Warn("quests", "destination", f.String())
	}
	mudlog.Info("quests", "inferred destinations", len(inferred))
	return len(fs)
}
```

(imports: `questengine`, `quests`). In `main.go` after the `questDoor` block (line 1980-1982) add
`destinationDoor()`. Add to `quest_loop_smoke_test.go`:

```go
func TestDestinationDoor_WarnsAndNeverRefuses(t *testing.T) {
	n := destinationDoorFrom(func() ([]questloop.Finding, []string) {
		return []questloop.Finding{{Rule: "walk-destination", QuestId: 1, Where: "step a", Msg: "m"}}, nil
	})
	if n != 1 {
		t.Fatalf("logged %d findings, want 1", n)
	}
}
```

and in `TestSmoke_ShippedQuestsPassTheLoop` replace the `fs :=` line with:

```go
	fs := append(questloop.Validate(w), questengine.Walk(w, questworld.Sources())...)
	dest, inferred := questengine.Destinations(w.Quests)
	fs = append(fs, dest...)
	for _, s := range inferred {
		t.Logf("destination inferred: %s", s)
	}
```

- [ ] **Step 6:** `go build . && go test . -run DestinationDoor`. Expected PASS. (The sweep itself
  is run in Task 10 once content is converted.)
- [ ] **Step 7:** Commit `internal/questengine/destinations_rule.go internal/questengine/destinations_rule_test.go main_quests.go main.go quest_loop_smoke_test.go`:
  `feat(quests): destination walk rule, boot warnings (#375 p4)`.

---

### Task 8: Convert the 220 steps

**Files:** `_datafiles/world/dogmud/quests/*.yaml` (79 files). Converter script in the session
scratchpad, never in the repo.

The 23 `-1` steps that lose the field (inference), by file and line on master `591b17124`:
`11:14, 11:28, 12:18, 13:16, 14:26, 14:53, 19:46, 28:158, 42:65, 61:22, 63:22, 63:32, 65:37,
66:32, 66:41, 67:103, 68:54, 72:32, 74:62, 75:41, 76:28, 76:37, 78:10`. Every other `-1` (32 with
`# action step`, and `42:73` `sleep anywhere`) becomes `destination: anywhere`.

- [ ] **Step 0: Guard test first.** Create `internal/guards/quest_destination_field_test.go`:

```go
package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// #375 phase 4: map_target and map_target_mob became the destination block.
// A quest file still carrying either key would be read as an unknown key and
// silently lose its minimap destination.
func TestQuestFilesCarryNoMapTargetKey(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "quests", "*.yaml"))
	if err != nil || len(files) < 70 {
		t.Fatalf("found %d quest files (%v); the guard would pass vacuously", len(files), err)
	}
	key := regexp.MustCompile(`(?m)^\s*map_target(_mob)?\s*:`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if key.Match(b) {
			t.Errorf("%s still has a map_target key; use destination:", filepath.Base(f))
		}
	}
}
```

  Run `go test ./internal/guards/ -run QuestFilesCarryNoMapTargetKey`. Expected: FAIL naming 79
  files (the probe can fail). It passes after Step 4.

- [ ] **Step 1:** Write `<scratchpad>/convert_dest.py` (run with `python -I`, reading from the
  repo, writing only under `<scratchpad>/converted/`):

```python
import glob, os, re, sys
src_dir, out_dir = sys.argv[1], sys.argv[2]
PAT = re.compile(r'^(\s*)(map_target|map_target_mob):\s*(-?\d+)\s*(#.*)?$')
INFER = {('11',14),('11',28),('12',18),('13',16),('14',26),('14',53),('19',46),('28',158),('42',65),
         ('61',22),('63',22),('63',32),('65',37),('66',32),('66',41),('67',103),('68',54),('72',32),
         ('74',62),('75',41),('76',28),('76',37),('78',10)}
def tidy(c):  # keep an author's reason, drop the old field's own name
    c = (c or '').strip()
    return ('  ' + c) if c else ''
os.makedirs(out_dir, exist_ok=True)
stats = {'files': 0, 'room': 0, 'mob': 0, 'anywhere': 0, 'inferred': 0}
for path in sorted(glob.glob(os.path.join(src_dir, '*.yaml'))):
    name = os.path.basename(path); qid = name.split('-')[0]
    text = open(path, encoding='utf-8', newline='').read()
    eol = '\r\n' if '\r\n' in text else '\n'  # keep each file's own line endings
    lines = text.split(eol)
    out, i, changed = [], 0, False
    while i < len(lines):
        m = PAT.match(lines[i])
        if not m:
            out.append(lines[i].replace('# map_target ', '# destination room ')); changed |= out[-1] != lines[i]; i += 1; continue
        assert m.group(2) == 'map_target', (name, i + 1)
        ind, val, com = m.group(1), int(m.group(3)), m.group(4)
        mob = PAT.match(lines[i + 1]) if i + 1 < len(lines) else None
        mob = mob if mob and mob.group(2) == 'map_target_mob' else None
        changed = True
        if val == -1:
            assert mob is None, (name, i + 1)
            if (qid, i + 1) in INFER:
                reason = (com or '').lstrip('#').strip().removeprefix('-1:').strip()
                out.append(f'{ind}# destination inferred from triggers (#375 p4); was: {reason}')
                stats['inferred'] += 1
            else:
                out.append(f'{ind}destination: anywhere'); stats['anywhere'] += 1
            i += 1; continue
        out.append(f'{ind}destination:')
        if mob:
            out.append(f'{ind}  mob: {mob.group(3)}{tidy(mob.group(4))}')
            out.append(f'{ind}  room: {val}{tidy(com)}')
            stats['mob'] += 1; i += 2
        else:
            out.append(f'{ind}  room: {val}{tidy(com)}')
            stats['room'] += 1; i += 1
    if changed:
        stats['files'] += 1
        open(os.path.join(out_dir, name), 'w', encoding='utf-8', newline='').write(eol.join(out))
print(stats)
```

- [ ] **Step 2:** Run it:
  `python -I <scratchpad>/convert_dest.py _datafiles/world/dogmud/quests <scratchpad>/converted`.
  Expected: `{'files': 79, 'room': 124, 'mob': 40, 'anywhere': 33, 'inferred': 23}` (220 steps).
  Any other count: stop and find out why before going on.
- [ ] **Step 3:** Review the diff of every converted file, one at a time (a whole-folder diff also
  lists the non-YAML files the converter does not copy, such as `step-ids.txt`, as deleted):
  `for f in <scratchpad>/converted/*.yaml; do git diff --no-index "_datafiles/world/dogmud/quests/$(basename "$f")" "$f"; done`
  (exit 1 is normal for a diff). A dry run on master `591b17124` gave exactly the Step 2 counts. Check: only the step lines changed; line endings match the
  original (the script splits and joins on each file's own ending); quest 88's comment now says
  `destination room 5203`.
- [ ] **Step 4:** Copy into place: `cp <scratchpad>/converted/*.yaml _datafiles/world/dogmud/quests/`.
- [ ] **Step 5:** `go test ./internal/guards/ -run QuestFilesCarryNoMapTargetKey ./internal/quests/ ./internal/questengine/`
  and `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_ServerBootsCleanWithRealData`. Expected: PASS
  (the old fields are still declared, so a converted file loads; `destination` validates). The old
  `map_target` resolver tests that read shipped files may now fail; they are deleted in Task 9, so
  note which and go on.
- [ ] **Step 6:** Commit the guard and the 79 files (`git add internal/guards/quest_destination_field_test.go _datafiles/world/dogmud/quests/<each>.yaml`;
  list them with `git diff --name-only`): `content(quests): map_target becomes destination (#375 p4)`.

---

### Task 9: Delete `MapTarget` and `MapTargetMob`

**Files:** `internal/quests/quests.go`, `internal/quests/validate_refs.go`,
`internal/quests/validate_refs_test.go` (if it names the fields), `internal/questengine/map_target*.go`
(delete 4 files), `cmd/dialoguecheck/walk.go`, `boot_smoke_test.go`,
`modules/gmcp/gmcp.Delete_refs_test.go`, `internal/questloop/keys_test.go`,
`internal/rooms/roommanager.go` (comment).

- [ ] **Step 2:** Delete the fields from `QuestStep` (and their doc comment), then `go build ./...`
  and `go vet ./...`; fix every reader the compiler lists:
  - `git rm internal/questengine/map_target.go internal/questengine/map_target_test.go internal/questengine/map_target_mob_test.go internal/questengine/map_target_data_test.go`
    (the data test's room-exists check is now `ValidateQuestRefs`; the same-zone probe is
    replaced by cross-zone routing).
  - `validate_refs.go`: replace the step block (lines 304-332) with:

```go
	// Steps: the destination's room, mob, area and zone must exist (#375 phase 4).
	roomFiltered := map[int]bool{}
	hasRoomFilters := false
	for _, t := range q.Triggers {
		if t.Room > 0 {
			roomFiltered[t.Room] = true
			hasRoomFilters = true
		}
	}
	for i, s := range q.Steps {
		d := s.Destination
		where := fmt.Sprintf("step %d (%s) destination", i, s.Id)
		c.checkRoom(where+" room", d.Room)
		c.checkMob(where+" mob", d.Mob)
		if d.Area != "" && !c.v.AreaExists(d.Area) {
			c.errf("%s area: area %q does not exist", where, d.Area)
		}
		if d.Zone != "" && !c.v.ZoneExists(d.Zone) {
			c.errf("%s zone: zone %q does not exist", where, d.Zone)
		}
		if d.Room > 0 && d.Mob == 0 && hasRoomFilters && !roomFiltered[d.Room] && c.v.RoomExists(d.Room) {
			c.warnf("step %d (%s): destination room %d is referenced by no trigger; double-check it is the intended destination", i, s.Id, d.Room)
		}
	}
```

    (`checkRoom`/`checkMob` ignore id 0.) Check `AreaExists`/`ZoneExists` are non-nil wherever
    `ValidateQuestRefs` runs with partial validators in tests; follow how `checkArea` guards them
    and match it.
  - `cmd/dialoguecheck/walk.go:661-666`: `st.MapTarget` to `st.Destination.Room`,
    `st.MapTargetMob` to `st.Destination.Mob`; update the comment above to "destinations (room or NPC)".
  - `boot_smoke_test.go`: delete `TestSmoke_EveryQuestStepHasMarkerDecision` (lines 372-421; the
    `walk-destination` rule replaces it) and any import it alone used.
  - `modules/gmcp/gmcp.Delete_refs_test.go`: `q.Steps[1].MapTarget = 9001` to
    `q.Steps[1].Destination.Room = 9001`; `q.Steps[1].MapTargetMob = 9300` to
    `q.Steps[1].Destination = quests.StepDestination{Mob: 9300}`; the wanted strings `map_target`
    to `destination room`, `map_target_mob` to `destination mob`; the refusal fixture string at
    line 140 to `step firstkill destination room: room 2 does not exist`; rename
    `TestBuildMobDelete_RefusedWhileAStepMapTargetMobNamesIt` to
    `TestBuildMobDelete_RefusedWhileAStepDestinationNamesIt`.
  - `internal/questloop/keys_test.go:102-103`: the expected text becomes the new label.
  - `internal/rooms/roommanager.go:418` comment: "matches no quest destination".
- [ ] **Step 3:** `go build ./... && go vet ./... && go test ./internal/... ./modules/... ./cmd/...`.
  Expected: PASS.
- [ ] **Step 4:** Commit the changed and deleted paths by name:
  `refactor(quests): retire map_target and map_target_mob (#375 p4)`.

---

### Task 10: GMCP payload and the client marker

**Files:** `modules/gmcp/gmcp.Char.go`, `_datafiles/html/public/webclient-pure.html`,
`tools/webclient-tests/quest-panel-refresh.js`.

- [ ] **Step 1: Failing JS check.** In `quest-panel-refresh.js`, before the final summary line, add:

```js
console.log('SCENARIO 9: a cross-zone destination sends next_room but no target_room');
reset();
handleGMCP('Char.Quests', [{ id: 9, name: 'Far Off', completion: 10, focused: true,
  hint: 'Go far.', target_room: 0, next_room: 420, next_dir: 'west', target_kind: 'area' }]);
check('the off-map arrow still gets a marker', markerCalls.length > 0 && markerCalls[markerCalls.length - 1] &&
  markerCalls[markerCalls.length - 1].nextRoom === 420, JSON.stringify(markerCalls));
```

  Run `node tools/webclient-tests/quest-panel-refresh.js`; expected: the new check FAILS.
- [ ] **Step 2:** `webclient-pure.html:837`: `if (f && f.target_room) {` becomes
  `if (f && (f.target_room || f.next_room)) {` with a comment
  `// A cross-zone destination has no room on this map (target_room 0) but still a next hop (#375 p4).`
  Rerun; expected: ALL CHECKS PASSED.
- [ ] **Step 3:** In `gmcp.Char.go` add to `GMCPCharModule_Payload_Quest` after `NextDir`:

```go
	// #375 phase 4 (focused quest only). TargetRoom is the nearest room of the
	// destination on the player's map (0 when it lies in another zone).
	TargetKind  string `json:"target_kind,omitempty"`  // anywhere, room, mob, area, zone, unknown
	TargetLabel string `json:"target_label,omitempty"` // area or zone name, else the NPC's name
	TargetRooms []int  `json:"target_rooms,omitempty"` // the destination's rooms in this zone (map wash)
	Inside      bool   `json:"inside,omitempty"`
	Guide       string `json:"guide,omitempty"` // questengine.GuideLine, the quest log's line
```

  and replace the marker block (lines 721-730) with:

```go
			// Destination data is computed for the focused quest only (#375 phase 4).
			if questPayload.Focused {
				if d := engine.ResolveDestination(user, questId, questStep); d.Kind != questengine.DestNone {
					questPayload.TargetKind = string(d.Kind)
					questPayload.TargetLabel = d.Label
					if questPayload.TargetLabel == "" {
						questPayload.TargetLabel = d.Npc
					}
					questPayload.TargetRooms = d.ZoneRooms
					questPayload.Inside = d.Inside
					questPayload.TargetRoom = d.Pin
					questPayload.NextRoom = d.NextRoom
					questPayload.NextDir = d.NextDir
					questPayload.Guide = questengine.GuideLine(d)
				}
			}
```

  Drop the now-unused `mapper` import if the build says so; update the `roomChangeHandler` comment
  (line 183) from `mapper.NextStep in the Char.Quests build` to `ResolveDestination in the
  Char.Quests build`.
- [ ] **Step 4:** `go build ./... && go test ./modules/gmcp/`. Expected PASS.
- [ ] **Step 5:** Commit the three files: `feat(gmcp): destination fields on the focused quest (#375 p4)`.

---

### Task 11: The `quest` command line

**Files:** `internal/usercommands/quests.go`, `internal/usercommands/quests_progress_test.go`,
`_datafiles/world/dogmud/templates/character/quests.template`.

- [ ] **Step 1: Failing test** appended to `quests_progress_test.go`:

```go
// #375 phase 4: each active quest shows where to head under its progress.
func TestQuestLog_ShowsTheDestinationLine(t *testing.T) {
	useDogmudTemplates(t)
	t.Cleanup(questengine.ResetEngineForTest())
	q := &quests.Quest{QuestId: 900055, Name: "Anywhere Quest", Steps: []quests.QuestStep{
		{Id: "start", Description: "Practise.", Destination: quests.StepDestination{Anywhere: true}},
		{Id: "end", Description: "Done."},
	}}
	questengine.GetEngine().RegisterQuest(q)
	u := users.NewTestUser(9763, "questdest", "Questdest", 97630)

	rec, _ := activeQuestRecord(u, q, "start")
	assert.Equal(t, "You can do this anywhere.", rec.Guide)
	out, err := templates.Process("character/quests", map[string]any{
		"QuestsFound": 1, "QuestsTotal": 1, "Records": []questRecord{rec},
	}, 0)
	require.NoError(t, err)
	assert.Contains(t, out, "You can do this anywhere.")

	rec, _ = activeQuestRecord(u, q, "end")
	assert.Empty(t, rec.Guide, "the final step has no destination line")
}
```

- [ ] **Step 2:** run `go test ./internal/usercommands/ -run ShowsTheDestinationLine`; expected:
  build failure (`Guide`).
- [ ] **Step 3:** Add `Guide string // the destination line (questengine.GuideLine), else ""` to
  `questRecord`; in `activeQuestRecord`, inside `if questStep != "all+" {` after the progress line:

```go
		guide = questengine.GuideLine(questengine.GetEngine().ResolveDestination(user, questInfo.QuestId, questStep))
```

  (declare `guide := ""` beside `progress := ""`, set `Guide: guide` in the returned record). In
  `quests.template`, after the Progress clause `{{ end }}` add:

```
{{ if $qInfo.Guide }}{{ "\n" }}   <ansi fg="240">{{ $qInfo.Guide }}</ansi>{{ end }}
```

  on the same line, before `{{ if lt $idx $nlLen }}`.
- [ ] **Step 4:** run; expected PASS. Run `go test ./internal/usercommands/`; expected PASS.
- [ ] **Step 5:** Commit the three files: `feat(quests): the quest log shows where to head (#375 p4)`.

---

### Task 12: Backlog and the mistag check

- [ ] **Step 1:** Generate today's findings into the scratchpad:
  `DOGMUD_BOOT_SMOKE=1 DOGMUD_LOOP_BACKLOG_WRITE=<scratchpad>/backlog.txt go test . -run TestSmoke_ShippedQuestsPassTheLoop -v`.
- [ ] **Step 2:** Compare with the shipped file: every non-destination line must be identical
  (`diff` the two with destination lines filtered out of the new one; expected no other change).
  Count the `walk-destination` and `walk-destination-anywhere` lines and the logged inferred steps.
- [ ] **Step 3:** For each `walk-destination-anywhere` finding, open the step: if its trigger's
  place is where the player goes, replace `destination: anywhere` with nothing (inference) using
  Edit; if the place is incidental, leave it and say why in the PR. Re-run Step 1 until no
  `walk-destination-anywhere` line remains.
- [ ] **Step 4:** Read the inferred list: each inferred step's destination should be where its hint
  sends the player. Fix any that is not with an explicit `destination:` (Edit) and note it.
- [ ] **Step 5:** Copy the scratch backlog into place:
  `cp <scratchpad>/backlog.txt _datafiles/world/dogmud/quests/loop-backlog.txt`, then
  `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_ShippedQuestsPassTheLoop`. Expected PASS.
- [ ] **Step 6:** Commit the backlog and any quest files changed in Steps 3-4:
  `content(quests): destination backlog, inferred steps checked (#375 p4)`. The PR body lists
  every `-1` step and what it became, the unknown count, and the inferred list.

---

### Task 13: Benchmarks and docs

- [ ] **Step 1: Mapper benchmark** `internal/mapper/mapper.nearest_bench_test.go`: build a 20x20
  grid mapper (same shape as `buildGridMapper`), `BenchmarkNearestIn400` (from corner to a
  3-room set in the far corner) and `BenchmarkFindPath400` (same start and the nearest of those
  rooms, uncached `findPath`). Run `go test ./internal/mapper/ -run x -bench 'NearestIn400|FindPath400' -count=5`.
- [ ] **Step 2: Resolver benchmark** `internal/questengine/destination_bench_test.go`:
  `BenchmarkResolveDestinationMob30` with `installDestWorld`-style stubs (30 copies over 30 rooms,
  half hidden, `sightIn` calling the real `messaging.ParticipantSight` against a fixed-light room,
  `nearestOf` returning at once), and `BenchmarkResolveDestinationArea40` (a 40-room area via
  `areas.InstallForTest`). Run with `-count=5`. Put both tables in the PR. If the mob case is
  above 50 µs/op, profile before shipping and bring the numbers to the owner.
- [ ] **Step 3: Docs** (Edit only), each naming only symbols that exist (check with
  `codegraph_search` or `Select-String`):
  - `internal/quests/context.md`: `StepDestination`, the `destination:` block and its shapes; drop
    `map_target`.
  - `internal/questengine/context.md`: `ResolveDestination`, `Destination`, `DestKind`,
    `GuideLine`, `Destinations`; drop `ResolveQuestTarget`, `mobRoomLookup`.
  - `internal/mapper/context.md`: `NearestOf`, `nearestZoneHop`.
  - `internal/rooms/context.md`: `SpawnRooms`.
  - `internal/areas/context.md`: `ZonesOf`, `RoomsIn`; questengine as a consumer.
  - `modules/gmcp/context.md`: the five payload fields.
  - `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`: phase 4 status (slice 1 merged, PR number), the
    facts row "Minimap target today" updated, phase 6 editor list gains the `destination:` block,
    phase 7 exit gains "no `walk-destination` lines left for the zone".
  - `.claude/skills/dogmud-authoring-quests/SKILL.md`: the `destination:` block, `anywhere`, that
    a trigger's `target:` is a different thing, and that a step with no block is inferred.
  - `PATCH_NOTES.md`: one player-facing entry: "The quest log now says where to head next" (no
    numbers, 80 columns).
  - `docs/README.md`: this plan.
  - Run `python tools/context_md_audit.py`; expected: no phantom symbols in the six files.
- [ ] **Step 4:** Commit benchmarks and docs by name: `docs(375): phase 4 slice 1 docs and benchmarks`.

---

### Task 14: Gate, playtest, PR

- [ ] **Step 1:** Full gate per `dogmud-shipping` (pre-push order): `go build ./...`, `go vet ./...`,
  `go test ./...`, `go test -race` on `internal/questengine internal/mapper internal/rooms internal/areas`,
  `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke`, every `node tools/webclient-tests/*.js`, and a
  detached-worktree boot check. Paste the tail of each into the PR.
- [ ] **Step 2:** Playtest per `dogmud-playtesting` (private port, kill by PID, `docker rm -f` the
  container after): one seat walks quest 10 (Velk by NPC destination: the `quest` line says
  where to head and changes as the seat walks; in the browser the arrow points the right way),
  one walks a quest with an `anywhere` step (quest 31), one stands in the unquiet grove on quest
  45's step. Record findings as GitHub issues (`--repo pruuk/DOGMud`).
- [ ] **Step 3:** Open the PR (`gh pr create --repo pruuk/DOGMud`), body: summary, the `-1` table,
  the unknown and inferred counts, benchmarks, gate output, playtest results, deploy notes (no save
  changes; quest files changed; `destination` replaces `map_target`; the off-map arrow condition in
  the web client). End with the generated-with line. No "closes #375".

---

## Self-review

- Spec coverage: section 1 (Tasks 1, 8, 9), section 2 (Tasks 3, 4, 5), section 3 slice 1 part
  (Task 10; wash, banner, panel line and pin colour are slice 2), section 4 (Tasks 6, 11), section 5
  (Tasks 7, 12), section 6 conversion and mistag fix (Tasks 8, 12; live examples are slice 3),
  section 7 (tests in each task, Task 13 benchmarks, Task 14 playtest), section 9 (Task 13).
- Types: `StepDestination`, `Destination`, `DestKind`, `liveCopy`, `destSet`, `stepPlaces`,
  `NearestOf(from, in, zones)`, `nearestZoneHop`, `hopToward`, `SpawnRooms`, `ZonesOf`, `RoomsIn`,
  `GuideLine`, `Destinations(qs)`, `destinationDoor`, `destinationDoorFrom` are used with one
  signature throughout.
