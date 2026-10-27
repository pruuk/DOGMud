# #375 phase 3b slice 4: areas, counts and explore, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Quest steps can be scoped to a named area or a zone ("kill wolves on the steppe", "find
the rare drop anywhere in the Grove"), can need several matches ("gather 5 roots"), and can ask the
player to explore an area fully, with explore progress read from the rooms the player has seen.

**Architecture:** A new leaf package `internal/areas` loads `_datafiles/world/dogmud/areas/*.yaml`
(each a key, a player-facing name, zones and/or rooms), resolves each to a set of template room ids
at boot and keeps a reverse index. Trigger fields `area:` and `zone:` filter every event by the
event's template room. `count: N` keeps a saved per-step counter on the character
(`QuestStepCounts`), bumped through an optional `StepCounter` interface on the action context, and
runs the trigger's actions only on the Nth match. `distinct: all|N` on a `room_enter` + `area`
trigger is not matched on ordinary room entry: it is evaluated when a room of the area is newly
mapped and when its step starts, and its progress is the number of the area's rooms on the
player's map (read through an optional `MapReader` interface). Quest 45's three copied cast triggers
become one with `area:`.

**Tech Stack:** Go (`internal/areas` new, `quests`, `questengine`, `questloop`, `questworld`,
`characters`, `actions`, `hooks`, `usercommands`, `modules/gmcp`, `guards`), YAML under
`_datafiles/world/dogmud/`.

**Spec:** `docs/superpowers/specs/2026-10-27-quest-arc-phase3b-trigger-kinds-design.md`, sections 4
and 5. Slices 1 to 3 are merged (#699, #698, #693).

**Rules for every implementer (global, enforced by hooks):** edit files only with the Edit and
Write tools, never with shell redirection, `sed -i` or Python file writes. `git add` named paths
only. Never `--no-verify`. No em or en dashes in prose or comments. Kill only processes you
started, by PID.

---

## Facts verified against source (master `a84384183`, 2026-10-27)

| Fact | Where |
|---|---|
| `TriggerDef` fields end with `Target`, `Effect`, `Topic`; no area, zone, count, distinct | `internal/quests/triggers.go` |
| `matchTriggerFields` checks mob, room, item, skill, command, quest_token, target, effect, topic, noun, verb | `internal/questengine/engine.go` |
| `room` filters match the event's `RoomId`; `NotifyCommand`/`NotifySpellLanded`/`NotifyTopicHeard` pass the template room (`rooms.OriginalRoomId`); `room_enter` from the RoomChange hook | `internal/questengine/command_event.go`; `internal/hooks/RoomChange_QuestRoomEnter.go:43` |
| `PlayerState` interface: HasQuest, HasItem, GetRoomId, GetQuestFlag, GetGold, HasOwnMasterwork; implemented by `GameBridge`, `walkPlayer`, test mocks and slice 1's `heldQuests` adapter | `internal/questengine/conditions.go:5-12` |
| `ActionContext` interface (GrantQuest, GiveItem, ...); implemented by `GameBridge`, `walkCtx`, test mocks | `internal/questengine/actions.go:15` |
| `evaluate` marks a trigger visited only when it fires (after conditions pass), then runs actions and the `quest_granted` chain | `internal/questengine/engine.go` |
| `Character.QuestProgress map[int]string`; `GiveQuestToken` moves progress forward | `internal/characters/character.go:320`; `internal/characters/quests.go:102` |
| `Character.VisitedRooms map[string][]int` (zone to template ids), `HasVisitedRoom` linear per zone; rooms are added only by `actions.MarkRoomMappedIfSeen`, which returns true only for a newly mapped room | `character.go:329, 680-700`; `internal/actions/map_visit.go:24` |
| `MarkRoomMappedIfSeen` callers: `usercommands/go.go:284`, `modules/gmcp/gmcp.Room.go:88` | grep |
| Quest log renders the current step's `Description` (`usercommands/quests.go:112-118`); the web tracker sends `Description` and `Hint` (`modules/gmcp/gmcp.Char.go:705-709`) | as cited |
| Quest 45 has three copies of one `command: cast, noun: conviction-spike` trigger for rooms 5311 (Veil Hollow), 5312 (Thinning Wood), 5313 (Grove Heart) | `quests/45-the_unquiet_grove.yaml:83-120` |
| Manifest guards: topic keys `internal/guards/topic_manifest_test.go:40,59` over `internal/manifest` | as cited |
| World data loads in `main.go` (spells at `:1711`, quests at `:1964`) | `main.go` |
| Shipped smokes: `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke` at the repo root | `quest_loop_smoke_test.go` |
| Rooms: exits can carry a `Lock` (`HasLock`, `SetExitLock`) | `internal/rooms/rooms.go:924-935, 1426-1436` |

## Spec amendments made by this plan (land in the docs task)

1. **Explore progress is read from the map, not a saved counter.** A `distinct` step's progress is
   the number of the area's rooms in `VisitedRooms`, computed when it is evaluated. That is the
   owner's ruling (rooms seen before the quest count; a room crossed in the dark does not) with no
   second copy of the same fact to drift. `QuestStepCounts` holds only `count:` progress.
2. **Explore is evaluated at two moments only:** when a room of the area is newly mapped
   (`MarkRoomMappedIfSeen` true) and when the step starts (the `quest_granted` chain). A `distinct`
   trigger is never matched on ordinary room entry. Cost: one O(area size) pass at each of those
   moments, zero on rooms already mapped.
3. **Optional interfaces.** Counting and map reading go through `StepCounter` (on the action
   context) and `MapReader` (on the player state), type-asserted, so existing implementations and
   test mocks need no change; a context without them treats `count` as reached on the first match
   and `distinct` as unreachable (the walk implements both).

## File structure

| File | Change |
|---|---|
| `internal/areas/areas.go`, `loader.go`, `context.md`, tests (new) | area data, loader, reverse index |
| `_datafiles/world/dogmud/areas/unquiet_grove.yaml`, `areas/area-keys.txt` (new) | first area, manifest |
| `internal/guards/area_manifest_test.go` (new) | area-key manifest guard |
| `main.go` | load areas after rooms |
| `internal/quests/triggers.go`, `quests.go`, `validate_refs.go`, tests | `Area`, `Zone`, `Count`, `Distinct`, `Label`, `ExploredLine`; loader and ref checks |
| `internal/questengine/engine.go`, `counts.go` (new), `explore.go` (new), tests | matching, counting, explore |
| `internal/characters/character.go`, `quests.go` | `QuestStepCounts`, cleared on step advance |
| `internal/questengine/bridge.go` | `StepCounter`, `MapReader` |
| `internal/actions/map_visit.go` | newly mapped room raises the explore check |
| `internal/usercommands/quests.go`, `modules/gmcp/gmcp.Char.go` | "label: N of M" progress line |
| `internal/questengine/walkthrough.go`, `walkplayer.go` | walk satisfies count and distinct |
| `internal/questworld/questworld.go` | `AreaExists`, `ZoneExists` validators |
| `quests/45-the_unquiet_grove.yaml`, `loop-backlog.txt` | three triggers become one |
| `internal/questengine/area_bench_test.go` (new) | the benchmark |

---

### Task 0: Worktree

The worktree exists: `C:/tmp/dogmud-375-3b4`, branch `feat/375-3b-areas`, from master `a84384183`.
Baseline: `go build ./... && go test ./internal/quests/ ./internal/questengine/ ./internal/questloop/ ./internal/questworld/ ./internal/characters/ ./internal/actions/ ./internal/guards/`. Stop and report on any failure.

### Task 1: `internal/areas`

- [ ] **Step 1: Failing tests** (`internal/areas/areas_test.go`), against an injected room table so no world is needed:

```go
package areas

import (
	"strings"
	"testing"
)

func testRooms() map[int]string { // room id -> zone
	return map[int]string{5311: "pothole_coulee", 5312: "pothole_coulee", 5313: "pothole_coulee", 100: "thornwall", 101: "thornwall"}
}

func TestResolveZonesAndRooms(t *testing.T) {
	a := Area{Key: "grove", Name: "the Grove", Zones: []string{"thornwall"}, Rooms: []int{5311, 5313}}
	set, warns, err := resolve(a, testRooms())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{100, 101, 5311, 5313} {
		if !set[id] {
			t.Errorf("room %d missing from %v", id, set)
		}
	}
	if set[5312] {
		t.Errorf("5312 is not in the area")
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings %v", warns)
	}
}

func TestResolveRefusesAndWarns(t *testing.T) {
	for _, c := range []struct {
		a    Area
		want string
	}{
		{Area{Key: "x", Name: "X", Zones: []string{"nowhere"}}, `unknown zone "nowhere"`},
		{Area{Key: "x", Name: "X", Rooms: []int{9}}, "unknown room 9"},
		{Area{Key: "x", Name: "X"}, "resolves to no rooms"},
		{Area{Key: "x"}, "has no name"},
	} {
		if _, _, err := resolve(c.a, testRooms()); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: want %q, got %v", c.a, c.want, err)
		}
	}
	_, warns, err := resolve(Area{Key: "x", Name: "X", Zones: []string{"thornwall"}, Rooms: []int{100}}, testRooms())
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "room 100 is already in zone thornwall") {
		t.Errorf("want one overlap warning, got %v %v", warns, err)
	}
}

func TestIndexAndContains(t *testing.T) {
	r := newRegistry()
	r.add(Area{Key: "grove", Name: "the Grove", Rooms: []int{5311, 5312}}, map[int]bool{5311: true, 5312: true})
	r.add(Area{Key: "wood", Name: "the Wood", Rooms: []int{5312}}, map[int]bool{5312: true})
	if !r.contains("grove", 5311) || r.contains("wood", 5311) {
		t.Fatal("contains wrong")
	}
	if got := r.areasOf(5312); len(got) != 2 {
		t.Fatalf("areasOf(5312) = %v, want both", got)
	}
}
```

- [ ] **Step 2:** `go test ./internal/areas/`: FAIL to compile.
- [ ] **Step 3: Implement** `internal/areas/areas.go`:

```go
// Package areas holds the named sets of rooms that quests and the minimap
// refer to (#375 phase 3b slice 4): one YAML file per area under
// _datafiles/world/dogmud/areas/, each listing whole zones and/or single
// rooms, resolved to template room ids at boot.
package areas

import (
	"fmt"
	"sort"
)

// Area is one area file.
type Area struct {
	Key   string   `yaml:"key"`
	Name  string   `yaml:"name"` // player-facing: "the Grove"
	Zones []string `yaml:"zones,omitempty"`
	Rooms []int    `yaml:"rooms,omitempty"`
}

type registry struct {
	areas  map[string]Area
	rooms  map[string]map[int]bool // key -> template room ids
	byRoom map[int][]string        // template room id -> area keys, sorted
}

func newRegistry() *registry {
	return &registry{areas: map[string]Area{}, rooms: map[string]map[int]bool{}, byRoom: map[int][]string{}}
}

func (r *registry) add(a Area, set map[int]bool) {
	r.areas[a.Key] = a
	r.rooms[a.Key] = set
	for id := range set {
		r.byRoom[id] = append(r.byRoom[id], a.Key)
		sort.Strings(r.byRoom[id])
	}
}

func (r *registry) contains(key string, roomId int) bool { return r.rooms[key][roomId] }
func (r *registry) areasOf(roomId int) []string           { return r.byRoom[roomId] }

// resolve turns an area into its room set. roomZones maps every loaded
// template room to its zone. It refuses an unnamed area, an unknown zone or
// room, and an area with no rooms; it warns on a room already inside one of
// the area's zones.
func resolve(a Area, roomZones map[int]string) (map[int]bool, []string, error) {
	if a.Name == "" {
		return nil, nil, fmt.Errorf("area %q has no name", a.Key)
	}
	zones := map[string]bool{}
	for _, z := range a.Zones {
		zones[z] = true
	}
	set := map[int]bool{}
	found := map[string]bool{}
	for id, z := range roomZones {
		if zones[z] {
			set[id] = true
			found[z] = true
		}
	}
	for _, z := range a.Zones {
		if !found[z] {
			return nil, nil, fmt.Errorf("area %q: unknown zone %q", a.Key, z)
		}
	}
	var warns []string
	for _, id := range a.Rooms {
		z, ok := roomZones[id]
		if !ok {
			return nil, nil, fmt.Errorf("area %q: unknown room %d", a.Key, id)
		}
		if zones[z] {
			warns = append(warns, fmt.Sprintf("area %q: room %d is already in zone %s", a.Key, id, z))
		}
		set[id] = true
	}
	if len(set) == 0 {
		return nil, nil, fmt.Errorf("area %q resolves to no rooms", a.Key)
	}
	return set, warns, nil
}
```

  Then `loader.go`: a package-level `reg *registry`; `LoadDataFiles(roomZones map[int]string) error`
  reading every `*.yaml` in `<DataFiles>/world/dogmud/areas/` the way other strict loaders do
  (find one with `grep -rln "KnownFields(true)" internal | head`, follow its file reading and
  `configs` path), refusing a key that is not the file name, collecting warnings to the log; and the
  public read API: `Get(key) (Area, bool)`, `Contains(key string, roomId int) bool`,
  `AreasOf(roomId int) []string`, `Rooms(key) []int` (sorted), `Keys() []string` (sorted), and
  `InstallForTest(areas []Area, roomZones map[int]string)` for other packages' tests. Add tests for
  `LoadDataFiles` over a temp directory (bad key, good file). Write `internal/areas/context.md`.
- [ ] **Step 4: Boot wiring.** In `main.go`, after rooms are loaded and before `questengine.LoadDataFiles()`, build `roomZones` from the loaded room templates (find the accessor: `grep -n "func GetAllRoomIds\|func LoadRoomTemplate\|func AllRoomTemplates" internal/rooms/*.go`) and call `areas.LoadDataFiles(roomZones)`; a load error stops the boot, as other world loaders do.
- [ ] **Step 5:** Create `_datafiles/world/dogmud/areas/unquiet_grove.yaml`:

```yaml
key: unquiet_grove
name: the unquiet grove
rooms: [5311, 5312, 5313]
```

  and `_datafiles/world/dogmud/areas/area-keys.txt` in the topic manifest's format (read
  `_datafiles/world/dogmud/dialogue/topic-keys.txt` and `internal/guards/topic_manifest_test.go`);
  add `internal/guards/area_manifest_test.go` with the guard and its `_GuardCanFail` twin.
- [ ] **Step 6:** `go build ./... && go test ./internal/areas/ ./internal/guards/ -count=1` and the boot smokes at the repo root. Expected: PASS.
- [ ] **Step 7: Commit** `feat(areas): named room sets loaded from areas/ (#375)` (quoted heredoc, Co-Authored-By line).

### Task 2: Trigger fields and their checks

- [ ] **Step 1: Failing tests** in `internal/quests/areas_counts_test.go` (use `validQuest`):
  - `area` and `zone` are accepted on any event; both on one trigger is refused ("set area or zone, not both").
  - `count: 1` is refused ("count must be 2 or more"); `count: 0` means unset; `count: 3` accepted; `count` with `distinct` refused.
  - `distinct` accepts `all` or a whole number of 1 or more, only on `room_enter` with `area`; refused otherwise with messages naming the rule.
  - `label` only with `count` or `distinct`; `explored_line` only with `distinct`.
  - In `validate_refs_test.go`: unknown `area` and unknown `zone` are reference errors (new validators `AreaExists func(string) bool`, `ZoneExists func(string) bool`); `distinct: all` on an area larger than 40 rooms is a warning; `distinct: all` on an area containing a room that is the destination of a locked exit or the room of a `room_gate` trigger is an error ("explore-all area has a gated room <id>"; add `AreaGatedRoom func(key string) (int, bool)`).
- [ ] **Step 2:** run red.
- [ ] **Step 3: Implement.** `TriggerDef` gains (yaml snake case):

```go
	Area         string `yaml:"area,omitempty" json:"area,omitempty"`                   // the event's room is in this area (#375 3b)
	Zone         string `yaml:"zone,omitempty" json:"zone,omitempty"`                   // the event's room is in this zone
	Count        int    `yaml:"count,omitempty" json:"count,omitempty"`                 // fire on the Nth match in this step (N >= 2)
	Distinct     string `yaml:"distinct,omitempty" json:"distinct,omitempty"`           // room_enter + area: "all" or N rooms of the area on the player's map
	Label        string `yaml:"label,omitempty" json:"label,omitempty"`                 // progress line label for count/distinct
	ExploredLine string `yaml:"explored_line,omitempty" json:"explored_line,omitempty"` // distinct: said when the area is already explored at step start
```

  and the loader and reference checks above; `questworld.Validators()` fills the new validators
  from `internal/areas` and the loaded rooms and quests (room_gate triggers' rooms; locked exits:
  read how exits store locks in `internal/rooms` and check every room's exits for a lock whose
  destination is in the area). Add the fields to every full test validator literal
  (`grep -rn "QuestValidators{" --include=*_test.go internal modules`).
- [ ] **Step 4:** `go build ./... && go test ./internal/quests/ ./internal/questloop/ ./internal/questworld/ ./modules/gmcp/ -count=1`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(quests): area, zone, count and distinct trigger fields (#375)`.

### Task 3: `area` and `zone` matching

- [ ] **Step 1: Failing test** (`internal/questengine/area_filter_test.go`): with `areas.InstallForTest` (rooms 5311-5313 in `grove`, room 100 in zone `thornwall`), a `mob_death` trigger with `area: grove` fires for `RoomId: 5312` and not for `RoomId: 100`; a `zone: thornwall` trigger the reverse. Read how the engine looks up a room's zone: use `rooms.LoadRoomTemplate(id).Zone` behind a package var seam `roomZoneOf = func(id int) string` so the test can stub it.
- [ ] **Step 2:** red. **Step 3:** in `matchTriggerFields`, after the room check:

```go
	if t.Area != "" && !areas.Contains(t.Area, d.RoomId) {
		return false
	}
	if t.Zone != "" && roomZoneOf(d.RoomId) != t.Zone {
		return false
	}
```

  Check that every event source passes the template room (`room_enter` in `RoomChange_QuestRoomEnter.go`, `mob_death`, `item_gain`, `item_give`, `skill_use`, `command_issued`, `room_interact`, `room_gate`, `quest_granted`): where one passes an instance id, map it with `rooms.OriginalRoomId` at that source and add a test. `walkthrough.go`'s synthetic event: set `RoomId` to some room of `t.Area` (the first of `areas.Rooms`) when the trigger names an area and no room. `questloop/grants.go` signature: add `area`, `zone`, `count`, `distinct`.
- [ ] **Step 4:** tests green. **Step 5: Commit** `feat(questengine): area and zone filter every event (#375)`.

### Task 4: `count`

- [ ] **Step 1: Failing tests** (`internal/questengine/count_test.go`): a `mob_death` trigger with `count: 3` grants its token only on the third matching event; a non-matching mob does not count; conditions that fail do not count; a context without `StepCounter` fires on the first match. In `internal/characters`: `BumpQuestStepCount("53-hunt")` returns 1, 2, 3; `GiveQuestToken` advancing quest 53 clears its counts; counts survive a YAML round trip (follow an existing save round-trip test in `internal/characters`).
- [ ] **Step 2:** red. **Step 3: Implement.**
  - `characters.Character` gains `QuestStepCounts map[string]int \`yaml:"queststepcounts,omitempty"\`` (keyed `"<questId>-<stepId>"`, the step the player is on), `BumpQuestStepCount(key string) int`, `QuestStepCount(key string) int`; `GiveQuestToken` deletes every key with prefix `"<questId>-"` when it moves that quest's progress. Abandon (phase 5) does not exist yet; note in a comment that it must keep them.
  - `internal/questengine/counts.go`:

```go
// StepCounter is implemented by action contexts that keep count: progress
// (#375 phase 3b slice 4). A context without it treats a count as reached on
// the first match.
type StepCounter interface {
	BumpStepCount(questId int) int // bumps the player's current step of questId, returns the new count
}
```

  - In `evaluate`, after conditions pass and before `MarkVisited`: if `t.def.Count > 1`, and `ctx` implements `StepCounter`, `n := sc.BumpStepCount(t.questId)`; if `n < t.def.Count`, mark the trigger visited (one bump per event) and `continue` without running actions; mark `result.Handled = true` so callers see the event was used. Confirm `indexedTrigger` carries the quest id; if not, add it at registration.
  - `GameBridge.BumpStepCount(questId)`: key from the user's current step of that quest; returns `Character.BumpQuestStepCount(key)`.
  - `walkCtx.BumpStepCount` returns a large number (the walk proves reachability, not repetition).
- [ ] **Step 4:** green, including `go test ./internal/characters/ -count=1`. **Step 5: Commit** `feat(questengine): count: N fires a trigger on its Nth match (#375)`.

### Task 5: `distinct` (explore)

- [ ] **Step 1: Failing tests** (`internal/questengine/explore_test.go`), with `areas.InstallForTest` and a player state that implements `MapReader`:
  - A `room_enter` trigger `area: grove, distinct: all` does not fire on ordinary `room_enter` events.
  - `CheckExplore(player, ctx, roomId)` with the third of three rooms newly mapped fires it; with two of three it does not.
  - `distinct: "2"` fires at two.
  - At step start (the engine's `quest_granted` chain granting the step the trigger's `has` names), an already fully mapped area fires the trigger at once and sends the trigger's `explored_line`, or "You already know every corner of the unquiet grove." (area name) when it has none.
- [ ] **Step 2:** red. **Step 3: Implement** `internal/questengine/explore.go`:

```go
// MapReader is implemented by player states that can read the player's map
// (#375 phase 3b slice 4): how many of these template rooms the player has
// mapped. Explore steps need it; a player state without it never satisfies
// one.
type MapReader interface {
	MappedCount(roomIds []int) int
}
```

  - `matchTriggerFields`: a trigger with `Distinct != ""` never matches an ordinary event (return false).
  - `func (e *Engine) CheckExplore(roomId int, player PlayerState, ctx ActionContext) NotifyResult`: for each area containing `roomId`, each `room_enter` trigger with that `Area` and a `Distinct`, whose conditions pass, compute `have := mr.MappedCount(areas.Rooms(key))` and `need` (`len(rooms)` for `all`, else the number); when `have >= need`, run it through the same guarded path `evaluate` uses (one guard; mark visited; execute actions; follow the `quest_granted` chain). Keep an index `exploreByArea map[string][]*indexedTrigger` built in `RegisterQuest`.
  - At step start: in `evaluate`'s `quest_granted` chain handling, after a token is granted, check the explore triggers of that quest (index `exploreByQuest`) whose conditions now pass; if met, fire them and send the explored line through the action context's narration (find how `actor` text is sent: `ctx.Narrate` with a `narration.Variants` actor line).
  - `GameBridge.MappedCount(roomIds)`: build a set per zone from `Character.VisitedRooms` once per call and count.
  - `actions.MarkRoomMappedIfSeen`: when it returns true for a player character, call a hook that raises `questengine.GetEngine().CheckExplore(...)` for that user. `actions` already imports `questengine`; find the user for the character (the callers have the user: change the two callers in `go.go:284` and `gmcp.Room.go:88` to call a new `questengine.NotifyRoomMapped(user, roomId)` when the function returns true, rather than threading the user into `actions`). Add `NotifyRoomMapped` to `command_event.go` and to the guard's event list if it raises a literal event name.
  - `walkPlayer`: implement `MapReader` returning the full count (the walk proves reachability).
- [ ] **Step 4:** green. **Step 5: Commit** `feat(questengine): distinct explore steps read the player's map (#375)`.

### Task 6: Progress line

- [ ] **Step 1: Failing tests:** the quest log (`usercommands/quests.go`) and the web tracker payload (`gmcp.Char.go` around `:705`) show, for a current step reached by a `count` or `distinct` trigger, the step description followed by `"<label>: <have> of <need>"` (label defaults to "Progress"). Find existing tests of either and extend them.
- [ ] **Step 2:** red. **Step 3: Implement** a helper in `questengine`: `StepProgress(questId int, step string, player PlayerState, counts func(key string) int) (label string, have, need int, ok bool)`, used by both renderers; the tracker payload gains `Progress string \`json:"progress,omitempty"\``; the web client renders it if its quest panel shows `Hint` (read `webclient-pure.html` around the quest panel; add the line under the description, plain text, no colour meaning).
- [ ] **Step 4:** green. **Step 5: Commit** `feat: count and explore progress in the quest log and tracker (#375)`.

### Task 7: Quest 45 and the walk

- [ ] Replace quest 45's three cast triggers (`quests/45-the_unquiet_grove.yaml:83-120`) with one trigger keeping the same conditions, actions and text, with `area: unquiet_grove` in place of `room:`. Update its header comment.
- [ ] Walkthrough: confirm a count trigger and a distinct trigger walk (tests in `walkthrough_test.go` style); the shipped smokes at the repo root report 0 new findings (the three quest 45 ways becoming one may move backlog keys: confirm each moved key is the same finding and replace it in `loop-backlog.txt`; anything genuinely new: stop and report).
- [ ] Commit `content: quest 45's grove cast step uses the unquiet_grove area (#375)`.

### Task 8: Benchmark

- [ ] `internal/questengine/area_bench_test.go`: `BenchmarkNotifyRoomEnter` registers the shipped quests (load them the way the walkthrough smoke does, or a fixed set of 80 synthetic quests with mixed room_enter triggers) plus 50 synthetic `room_enter` triggers with `area:` over a 30-room area, and times `Notify("room_enter", ...)` for a room outside every area and one inside. Run `go test ./internal/questengine/ -run x -bench NotifyRoomEnter -count=5` on this branch and on `origin/master` (master lacks the area triggers: run the same benchmark with the area triggers omitted, via a flag in the benchmark), and put both outputs in the PR. The spec's bar: the per-call cost of `Notify("room_enter")` for shipped content rises no more than 10%.
- [ ] Commit `test(questengine): room_enter benchmark with area triggers (#375)`.

### Task 9: Review gate

Controller dispatches an Opus review of Tasks 1-8.

### Task 10: Docs

- [ ] context.md: `internal/areas` (new), `questengine` (area/zone, count, explore, StepCounter, MapReader, NotifyRoomMapped, StepProgress), `quests`, `questworld`, `characters` (`QuestStepCounts`). Verify symbols by grep.
- [ ] `.claude/skills/dogmud-authoring-quests/SKILL.md`: areas (file format, where they live, the manifest), `area:`/`zone:`, `count:` with `label:`, `distinct:` with `explored_line:`, the seven quest shapes table from the spec section 4.
- [ ] GMCP `questEventVocab` filters and the builder JS (`EVENT_FILTERS`, `FILTER_KINDS`) gain `area`, `zone`, `count`, `distinct`, `label`, `explored_line` where they apply; `node --check`; builder Go test counts.
- [ ] Spec sections 4 and 5: the three amendments; the benchmark result.
- [ ] `docs/README.md`: rows for `internal/areas/` and `_datafiles/world/dogmud/areas/`.
- [ ] Dash check. Commit `docs: areas, counts and explore (#375)`.

### Task 11: Gate and PR

- [ ] `dogmud-shipping` gate (gofmt, build, lint, full tests, shipped smokes at the repo root, boot check). PR "Part of #375" with the amendments, the benchmark numbers and verification lines.

### Task 12: Playtest

- [ ] Two seats: (1) quest 45 seat at the grove with `set_quest_tokens: ["45-start"]` casts Conviction Spike in a fight in each of 5311 and 5313 on separate runs or after a relog, and confirms the step advances once from any grove room; (2) an explore check: there is no shipped explore quest, so the seat walks the grove rooms with a light and confirms `quests` and the map behave as before (no new lines, no lag). Record CPU (`docker stats --no-stream` on the container every few minutes) and put it in the PR. File findings as issues.
