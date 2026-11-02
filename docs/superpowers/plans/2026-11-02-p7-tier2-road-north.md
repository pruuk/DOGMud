# #375 phase 7 tier 2, the road north and Stillwater: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clear all 28 backlog findings on quests 2, 16, 17, 18, 19, 20, 60 and 61; move quests 18 and
20 from typed nouns to `search`; give quest 61 a fixed order.

**Architecture:** Content only. No engine change. Fixes move through the existing ratchet: fixing a
finding makes its `loop-backlog.txt` line fail CI until the line is deleted.

**Tech stack:** YAML world data, Go tests with testify. Spec (owner approved):
`docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier2-design.md`. Its hint table and wording are
approved: use them verbatim.

**Worktree:** `C:/tmp/dogmud-375-p7-tier2`, branch `feat/375-p7-tier2`, cut from
`docs/375-p7-tier2-spec`. Paths are relative to it; data paths are under `_datafiles/world/dogmud/`.
Remove it when the PR merges.

**Brief for every subagent:**
- Edit files with Edit and Write only. Never rewrite a repo file from the shell.
- `git add` named paths only. Never `--no-verify`. Commit messages through a quoted heredoc, ending
  with a blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes, YAML comments included. Hints at most 200 characters. No `;` in spoken text. An
  unquoted `: ` in a YAML plain scalar breaks the loader.
- Do not start a server or push. Test logs go in the session scratchpad, never `C:/tmp` or `$TMP`.
- The sweep needs `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1` from the worktree root;
  without the variable it silently skips. Do not set it on `./internal/rooms`.
- `loop-backlog.txt`: delete a line when its finding is fixed. Never add a line.
- Report any fact below that does not match the source. Do not work around it.

## Facts verified against source (`d6fcb2b1b` plus the spec commit, 2026-11-02)

| Fact | Source |
|---|---|
| `event: command, command: search, room: N` is the shipped pattern | `quests/49-the_old_shrine.yaml:103-110` |
| `search` raises the event unless refused for cooldown | `internal/usercommands/skill.search.go:24-33,51` |
| Quest 20 has 17 `room_interact` triggers, quest 18 has 1 | `quests/20-*.yaml`, `quests/18-*.yaml` |
| "Try look" lines: 4138:34, 4124:77, 4130:71, 4140:92 | `rooms/stillwater/` |
| NPC lines: `337.yaml:52`, `344.yaml:26,44`, `347.yaml:74,113`, `355.yaml:39,48` | `dialogue/stillwater/` |
| Gates to change: `259.yaml:78-79` (`16-start`), `9208.yaml:43-44` (`60-start`), `347.yaml:90-91` (`20-kingfisher_found`), `355.yaml:68-69` (`20-brindle_confession`) | `dialogue/ashwick/`, `dialogue/new_plymouth_outskirts/`, `dialogue/stillwater/` |
| Flagged opener at `dialogue/north_road_north/9171.yaml:25-46` | as named |
| Test helpers: `newFullMockPlayer(roomId)`, `.quests`; `newFullMockActionContext(1, p)`, `.sentTexts`; `e.Notify("mob_death", EventDetails{UserId: 1, MobId: id, RoomId: 100}, p, ctx)` | `internal/questengine/engine_test.go`, `area_filter_test.go:44` |
| `QuestDef = quests.Quest`; shipped quests read by `yaml.Unmarshal`, as `tier1_undertow_test.go` does | `internal/questengine/types.go:11` |
| Backlog guard to copy | `internal/guards/tier1_backlog_guard_test.go` |
| `internal/questengine/walkplayer_test.go:38` uses `61-end`: read it if it fails | as named |

---

### Task 1: Quest 20 by `search`

**Files:** `quests/20-ullas_silence.yaml`; `rooms/stillwater/4138.yaml`, `4124.yaml`, `4130.yaml`,
`4140.yaml`; `dialogue/stillwater/337.yaml`, `344.yaml`, `347.yaml`, `355.yaml`; `quests/loop-backlog.txt`.
Test: create `internal/questengine/tier2_search_test.go`.

- [ ] **Step 1: Write the failing test.**

```go
package questengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadTier2Quest(t *testing.T, id string) *quests.Quest {
	t.Helper()
	paths, err := filepath.Glob("../../_datafiles/world/dogmud/quests/" + id + "-*.yaml")
	require.NoError(t, err)
	require.Len(t, paths, 1)
	data, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	return &q
}

// Rule 14: a step completes on something the player does, never a typed noun.
// Tier 2 ruling: each quest 20 site is found with a real search in that room.
func TestQuest20SitesAreFoundBySearch(t *testing.T) {
	q := loadTier2Quest(t, "20")
	sites := map[int]string{
		4138: "20-workshop_carving",
		4122: "20-temple_pillars",
		4124: "20-garden_marker",
		4144: "20-chapel_altar",
		4145: "20-wardstone_altar",
		4130: "20-bone_shoals_cache",
		4140: "20-kingfisher_found",
	}
	found := map[int]bool{}
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
		if tr.Event != "command" || tr.Command != "search" {
			continue
		}
		grant := ""
		for _, a := range tr.Actions {
			if a.Grant != "" {
				grant = a.Grant
			}
		}
		if grant == "" {
			continue // the lost kingfisher's replacement grants nothing
		}
		require.Equal(t, sites[tr.Room], grant, "room %d", tr.Room)
		found[tr.Room] = true
	}
	require.Len(t, found, len(sites))
}

// A lost kingfisher is replaced by searching at the grave again.
func TestQuest20LostKingfisherComesBackBySearch(t *testing.T) {
	q := loadTier2Quest(t, "20")
	for _, tr := range q.Triggers {
		if tr.Event == "command" && tr.Room == 4140 && tr.Conditions.MissingItem == 40060 {
			require.Contains(t, tr.Conditions.Has, "20-kingfisher_found")
			return
		}
	}
	t.Fatal("no replacement trigger for a lost kingfisher")
}
```

Run `go test ./internal/questengine/ -run TestQuest20 -count=1`. See it fail on `room_interact`.

- [ ] **Step 2: Replace the triggers.** In `quests/20-ullas_silence.yaml`, each site's group of
`room_interact` triggers becomes one trigger below, keeping the group's `actor` text word for word.
Keep both `topic_heard` triggers.

| `room` | `has` | `missing` | `grant` |
|---|---|---|---|
| 4138 | `20-start` | `20-workshop_carving` | `20-workshop_carving` |
| 4122 | `20-workshop_carving` | `20-temple_pillars` | `20-temple_pillars` |
| 4124 | `20-temple_pillars` | `20-garden_marker` | `20-garden_marker` |
| 4144 | `20-garden_marker` | `20-chapel_altar` | `20-chapel_altar` |
| 4145 | `20-chapel_altar` | `20-wardstone_altar` | `20-wardstone_altar` |
| 4130 | `20-wardstone_altar` | `20-bone_shoals_cache` | `20-bone_shoals_cache` |
| 4140 | `20-brindle_confession` | `20-kingfisher_found` | `20-kingfisher_found` (keeps `give_item: 40060`) |

```yaml
  - event: command
    command: search
    room: 4138
    conditions:
      has: ["20-start"]
      missing: ["20-workshop_carving"]
    actions:
      - grant: "20-workshop_carving"
      - actor: "A long tight spiral inside an outer ring, scratched
          into the iron with a patient hand. The cuts are dark with
          age: old work, set where it would not be noticed by anyone
          who did not know to look. Not a Chrysalis sigil and not a
          Stillwater fish-mark. The same spiral is cut into the two
          stone pillars on the Temple Approach in town."
```

The replacement for a lost kingfisher, keeping its text:

```yaml
  - event: command
    command: search
    room: 4140
    conditions:
      has: ["20-kingfisher_found"]
      missing: ["20-vella_journal"]
      missing_item: 40060
    actions:
      - give_item: 40060
      - actor: "There was another small carving wrapped beneath
          the first. You can take this one to Vella in place of
          the one you lost."
```

Change the `topic_heard` for `vellahandoff`'s `has` to `["20-kingfisher_found"]`.

- [ ] **Step 3: Hints.** Replace these steps' `hint` with the spec's table, verbatim: `start`,
`workshop_carving`, `temple_pillars`, `garden_marker`, `chapel_altar`, `wardstone_altar`,
`bone_shoals_cache`, `brindle_confession`, `kingfisher_found`. Quote any hint with `&` or `: `.

- [ ] **Step 4: Rooms.** Only the "Try" sentence changes.
  - `4138.yaml:33-34`: `carving scratched into the underside. Try` / `` `look carving`. `` becomes `carving scratched into the underside. Type search.`
  - `4124.yaml:77`: `of the crab-apple tree. Try <ansi fg="command">search</ansi>.`
  - `4130.yaml:71`: `depression in the floor below. Try <ansi fg="command">search</ansi>.`
  - `4140.yaml:92`: `disturbed and patted back not long ago. Try <ansi fg="command">search</ansi>.`

- [ ] **Step 5: NPC lines.**
  - `337.yaml:52`: `grave, type search.`
  - `344.yaml:26`: `...south of this room. Type search there.`
  - `344.yaml:44`: `the crab-apple tree there. Type search.`
  - `347.yaml:74`: `At the foot of Elgar's grave, type search.`
  - `347.yaml:113`: `handle of the vise on Elgar's bench. Type search.`
  - `355.yaml:39`: `Elgar's grave in the cemetery, type search.`
  - `355.yaml:48`: `type search.`
  - In `347.yaml`, `ullahandoff`'s `questRequired` becomes `20-vella_journal`.
  - In `355.yaml`, `vellahandoff`'s `questRequired` becomes `20-kingfisher_found`.

Then `grep -rnE 'look (carving|pillars?|marker|altar|slab|cache|disturbance)' _datafiles/world/dogmud/quests/20-* _datafiles/world/dogmud/dialogue/stillwater _datafiles/world/dogmud/rooms/stillwater _datafiles/world/dogmud/behaviors/stillwater`. It should find nothing that tells a player to type it (exit 1 is the pass). `344.yaml` has a line in the plain sense of "look at the foot of it": leave prose that is not a command.

- [ ] **Step 6: Backlog and run.** Delete the 7 quest 20 lines from `quests/loop-backlog.txt`. Run
`go test ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/narration/ ./internal/guards/ -count=1`
and the sweep. `internal/narration`'s golden changes only by the removed duplicate triggers: read the diff, then regenerate with `go test ./internal/narration/ -update -count=1`. A new finding is fixed in content.

- [ ] **Step 7: Commit** with named paths, message `fix(quests): Ulla's Silence is found by search`.

### Task 2: Quest 18's bedroll

**Files:** `quests/18-bettas_silence.yaml`, `quests/loop-backlog.txt`. Test: `internal/questengine/tier2_search_test.go`.

- [ ] **Step 1: Failing test,** appended:

```go
func TestQuest18BedrollIsFoundBySearch(t *testing.T) {
	q := loadTier2Quest(t, "18")
	found := false
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
		if tr.Event == "command" && tr.Command == "search" && tr.Room == 4050 {
			found = true
			require.Contains(t, tr.Conditions.Missing, "18-start")
		}
	}
	require.True(t, found)
}
```

- [ ] **Step 2: Trigger.** Change `event: room_interact` and `noun: bedroll` to the block below; the
conditions, `set_flag` and `actor` stay. Step `start`'s hint comes from the spec's table.

```yaml
  - event: command
    command: search
    room: 4050
```

- [ ] **Step 3:** Delete the `hint-length 18 step start` line. Run the Task 1 test command and the sweep.

- [ ] **Step 4: Commit** with named paths, message `fix(quests): Betta's bedroll is found by search`.

### Task 3: Quest 61's fixed order

**Files:** `quests/61-hedgerow_toll.yaml`, `dialogue/north_road_north/9171.yaml`, `quests/loop-backlog.txt`.
Test: create `internal/questengine/tier2_order_test.go`.

- [ ] **Step 1: Failing test.**

```go
package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func runKills(t *testing.T, questId string, start string, mobs ...int) (*fullMockPlayer, *fullMockActionContext) {
	t.Helper()
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, questId))
	p := newFullMockPlayer(100)
	p.quests[start] = true
	ctx := newFullMockActionContext(1, p)
	for _, m := range mobs {
		e.Notify("mob_death", EventDetails{UserId: 1, MobId: m, RoomId: 100}, p, ctx)
	}
	return p, ctx
}

// Tier 2 ruling: the hedgerow bandit first, then the Reach-Touched.
func TestQuest61InOrder(t *testing.T) {
	p, _ := runKills(t, "61", "61-start", 9173, 9178)
	assert.True(t, p.HasQuest("61-bandit"))
	assert.True(t, p.HasQuest("61-reach"))
}

func TestQuest61ReachFirstGrantsNothingAndSaysSo(t *testing.T) {
	p, ctx := runKills(t, "61", "61-start", 9178)
	assert.False(t, p.HasQuest("61-bandit"))
	assert.False(t, p.HasQuest("61-reach"))
	assert.Len(t, ctx.sentTexts, 1)
	assert.Contains(t, ctx.sentTexts[0], "Pemberton wanted the bandit")

	// The bandit, then the Reach-Touched again, still finishes.
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, "61"))
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9173, RoomId: 100}, p, ctx)
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9178, RoomId: 100}, p, ctx)
	assert.True(t, p.HasQuest("61-reach"))
}

// Quest 19: a hunter killed before a swarm grants nothing.
func TestQuest19HunterAfterShrimp(t *testing.T) {
	p, _ := runKills(t, "19", "19-start", 331)
	assert.False(t, p.HasQuest("19-hunter"))
	p, _ = runKills(t, "19", "19-start", 330, 331)
	assert.True(t, p.HasQuest("19-hunter"))
}
```

See the 61 tests fail.

- [ ] **Step 2: Triggers.** In `quests/61-hedgerow_toll.yaml`, delete the `flags:` block. The triggers become:

```yaml
triggers:
  - event: mob_death
    mob: 9173
    conditions:
      has: ["61-start"]
      missing: ["61-bandit"]
    actions:
      - grant: "61-bandit"
      - actor: >-
          The hedgerow bandit folds into the ditch he ambushed from and
          does not get up. One of the road's two is done. The
          Reach-Touched still haunts the Empty Reach to the north.

  - event: mob_death
    mob: 9178
    conditions:
      has: ["61-bandit"]
      missing: ["61-reach"]
    actions:
      - grant: "61-reach"
      - actor: >-
          The Reach-Touched shudders, goes still, and is just a body on
          the stones. The road north is clear of both now. Carry the
          word back to Pemberton at the Lake & Ladle.

  - event: mob_death
    mob: 9178
    conditions:
      has: ["61-start"]
      missing: ["61-bandit"]
    actions:
      - actor: >-
          The Reach-Touched shudders, goes still, and is just a body on
          the stones. It will not stay down long. Pemberton wanted the
          bandit in the reeds dealt with first.
```

Step `start`: the description ends "Clear them in that order." and `hint` comes from the spec's table.

- [ ] **Step 3: Pemberton.** In `9171.yaml`: delete the flagged opener variant and reword the comment above
the `61-bandit` variant to "The bandit down, the Reach-Touched still out there." In `quest_start`, the
text's ending "Put them both down and the road is walkable again, and you will have my thanks and a fair purse for it." becomes "Put the brute down first, then the thing in the Reach, and the road is walkable again. You will have my thanks and a fair purse for it." The topic's `hints` becomes "Pemberton wants the hedgerow bandit at the Mire Cutoff dead first, and then the Reach-Touched in the Empty Reach." Read the `61-start` opener variant: if it says either order, say the bandit first.

- [ ] **Step 4:** Delete the 5 quest 61 lines. Confirm with `grep -rn 'reach-first' _datafiles/world/dogmud` (exit 1 is the pass). Run `go test ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/guards/ -count=1` and the sweep. Regenerate the narration golden if only quest 61's lines differ.

- [ ] **Step 5: Commit** with named paths, message `fix(quests): Hedgerow Toll runs in one order`.

### Task 4: Quests 2, 16, 17, 19, 60

**Files:** `quests/2-*.yaml`, `quests/16-*.yaml`, `quests/17-*.yaml`, `quests/19-*.yaml`, `quests/60-*.yaml`,
`dialogue/ashwick/259.yaml`, `dialogue/new_plymouth_outskirts/9208.yaml`, `quests/loop-backlog.txt`.
Test: `internal/dialogue/shipped_quest_bugs_test.go`.

- [ ] **Step 1: Failing test,** appended:

```go
// Tier 2: a hand-in follows the quest's last step.
func TestTier2HandInsFollowTheLastStep(t *testing.T) {
	for _, c := range []struct {
		file, topic, onPrev, onLast string
		prevExtra                   []string
		item                        int
	}{
		{"ashwick/259.yaml", "quest_complete_herbs", "16-start", "16-forager", nil, 0},
		{"new_plymouth_outskirts/9208.yaml", "guardhandoff", "60-waybill", "60-manifest", nil, 40073},
		{"stillwater/347.yaml", "ullahandoff", "20-kingfisher_found", "20-vella_journal", nil, 40061},
		{"stillwater/355.yaml", "vellahandoff", "20-brindle_confession", "20-kingfisher_found", nil, 40060},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(tok string) *PlayerState {
			ps := heldState(tok)
			ps.HasItem = func(id int) bool { return id == c.item }
			ps.CarriesItem = ps.HasItem
			return ps
		}
		require.NotContains(t, menuKeys(df, state(c.onPrev)), c.topic, c.file)
		require.Contains(t, menuKeys(df, state(c.onLast)), c.topic, c.file)
	}
}
```

Remove the unused `prevExtra` field if lint flags it. See it fail on the unchanged gates. `heldState` holds exact tokens, and `quest_complete_herbs` may also gate on other things: read each topic and adapt its fixture, without loosening the check.

- [ ] **Step 2: Gates.** `quest_complete_herbs`: `questRequired` `16-forager`. `guardhandoff`: `questRequired` `60-manifest`.

- [ ] **Step 3: Quest 19.** The `mob_death 331` trigger's `has` becomes `["19-shrimp"]`. The `sign_bounty` trigger's `has` becomes `["19-hunter"]`. Step `signed` gains:

```yaml
    destination:
      mob: 335  # follow Constable Drunn (maxwander 2)
      room: 4110  # fallback when Constable Drunn is not spawned
```

- [ ] **Step 4: Hints.** The spec's table, verbatim, for quest 2 `start`; 16 `start`, `forager`; 17 `start`; 19 `start`, `hunter`, `signed`; 60 `start`, `waybill`.

- [ ] **Step 5:** Delete every remaining line for quests 2, 16, 17, 19, 60. Then `grep -E '^[a-z-]+ (2|16|17|18|19|20|60|61) ' _datafiles/world/dogmud/quests/loop-backlog.txt` (exit 1 is the pass) and `grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .` must print 128 (156 minus 28). Run `go test ./internal/dialogue/ ./internal/questengine/ ./internal/quests/ ./internal/questloop/ ./internal/guards/ -count=1` and the sweep.

- [ ] **Step 6: Commit** with named paths, message `fix(quests): tier 2 hand-ins, the bounty's order and hints`.

### Task 5: Guard, docs, gates

**Files:** create `internal/guards/tier2_backlog_guard_test.go`; `docs/PATCH_NOTES.md`,
`docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/README.md`.

- [ ] **Step 1: The guard.**

```go
package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
)

// The #375 phase 7 tier 2 pass (spec
// docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier2-design.md) cleared
// every loop finding on these quests. A finding on one is fixed in content,
// never listed.
var tier2Quests = map[int]bool{2: true, 16: true, 17: true, 18: true, 19: true, 20: true, 60: true, 61: true}

func tier2BacklogLines(t *testing.T, data []byte) []string {
	t.Helper()
	b, err := questloop.ParseBacklog(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range b {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if id, err := strconv.Atoi(fields[1]); err == nil && tier2Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier2QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier2BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 2 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier2QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 20 step start\none-action 61 step reach:none\nchain 73 mob 9465 topic q74_grant\n")
	if got := tier2BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 2 lines, want 2: %v", len(got), got)
	}
}
```

- [ ] **Step 2: Patch notes,** under a new dated heading at the top of `docs/PATCH_NOTES.md`:

```markdown
## 2026-11-02: The road north and Stillwater

- Ulla's Silence is now followed by searching: at each place the trail leads,
  type search to find what Elgar left.
- In the barn, a search shows you what Betta's guest left behind.
- Goodwife Pemberton now wants the hedgerow bandit dealt with before the thing in
  the Reach. If you meet the Reach-Touched first, it will still be there.
- The lake-caves bounty counts a drowned hunter once the shrimp are cleared.
- Quests around Ashwick, the north road and Stillwater have shorter hints, and the
  map points the way.
```

- [ ] **Step 3: Docs.** Add the guard and this plan to `docs/README.md`. Roadmap: tier 2's list line reads "Built, playtest pending." The phase 7 status names tier 2 built, tier 3 (The Confluence) next. The controller finishes the line after the playtest.

- [ ] **Step 4: Gates.** Run in order and read each:

```bash
gofmt -l internal/ modules/
go build ./...
go test ./... -count=1
DOGMUD_BOOT_SMOKE=1 go test . -count=1
golangci-lint run --new-from-merge-base=origin/master
```

Then the isolated boot per the `dogmud-shipping` skill, built in this worktree, with the override in the
scratchpad. Expect exit 124, `Server Ready` 1, panic count 0. Delete `boot-check.exe`.

- [ ] **Step 5: Commit** with named paths, message `docs: tier 2 guard, patch notes and roadmap`.

### Task 6: Adversarial playtest (controller)

**Use the `dogmud-playtesting` skill.** Write `tools/playtest/scenarios/375-p7-tier2.yaml` and goals under
`tools/playtest/goals/scenarios/375-p7-tier2/`, committed. One subagent per seat, with helper scripts named
after the seat. Profile `mid`: the veteran has finished 2 and 17.

| Seat | Fixture | Must show |
|---|---|---|
| `mystery` | `mid`, room 4137, grant a light | Quest 20 from Ulla to the hook-spear, every site by `search`, `look <noun>` at each saying nothing false, the kingfisher lost and replaced. Notes where the short hints were not enough |
| `bounties` | `mid`, room 4110, then the Lake & Ladle | Quest 19 with a hunter first, then in order; the tooth or the partial close. Quest 61 with the Reach-Touched first, then in order |
| `roads` | `mid`, tokens `20-start` to `20-garden_marker` and flag `61-reach-first` for old saves | Quests 2, 16, 17 and 18 with the bedroll found first; quest 60's first leg; the old save reaches the chapel by search; the flagged save finishes quest 61 |

- [ ] **Step 1:** Run the seats. Check claims against raw `events.jsonl`.
- [ ] **Step 2:** Fix findings that belong to this pass, with a test. File the rest on `pruuk/DOGMud`, searching first.
- [ ] **Step 3:** Stop the environment, `docker rm -f` the container, stop seat `tail`s by PID (find by command line). Save harness lessons to memory.
- [ ] **Step 4:** Finish the roadmap line. Push, open the PR with `--repo pruuk/DOGMud --base master`, watch CI, merge with `--merge` when green. Then remove the worktree, delete `docs/375-p7-tier2-spec`, and put the main checkout on master.
