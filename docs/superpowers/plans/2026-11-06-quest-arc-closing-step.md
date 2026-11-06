# Quest arc (#375) closing step Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every quest loop and destination finding refuses boot with no backlog to park it on, `room_interact` becomes `look_at` and `search_at`, and the prose dash guard covers quests, dialogue and conversations.

**Architecture:** Three independent parts in one PR, one task each, then docs and verification. Part 1 deletes the backlog ratchet and makes both boot doors refuse. Part 2 renames one quest event into two and deletes the `verb` filter. Part 3 is copy edits plus a guard list.

**Tech Stack:** Go, YAML content under `_datafiles/world/dogmud/`, one JS file for the (disabled) quest editor.

**Spec:** `docs/superpowers/specs/2026-11-06-quest-arc-closing-step-design.md` (its facts table is the source for every line number below; master `a130651d9`).

---

## Rules for every worker

- **Edit files with the Edit and Write tools only.** No `sed -i`, no Python read-modify-write, no `>` into a tracked file. The hooks block those.
- **`git add <named paths>` only.** Never `-A`, `.` or `commit -a`. Never `--no-verify`.
- Commit messages go through a quoted heredoc (`git commit -F - <<'EOF'`), ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **No em or en dashes** in anything you write: comments, YAML, docs, commit messages.
- Run `go` commands from the worktree root. A red test stops the chain: never `&&` a commit after a test run whose exit code you have not read.
- Never kill a process you did not start; never by name.

## Setup (controller, once)

- [ ] Main checkout back to master: `git -C "C:/Users/Calabe Davis/workspace/DOGMud" switch master`.
- [ ] Worktree: `git -C "C:/Users/Calabe Davis/workspace/DOGMud" worktree add C:/tmp/dogmud-375-closing feat/375-closing-step`. Every task runs in `C:/tmp/dogmud-375-closing`.
- [ ] Baseline: `go build ./... && go test ./internal/quests/ ./internal/questloop/ ./internal/questengine/ ./internal/guards/ ./modules/gmcp/ ./internal/usercommands/ . -count=1` passes before any change. Record the output in the scratchpad.

## File map

| File | Task | Change |
|------|------|--------|
| `_datafiles/world/dogmud/quests/31-find_your_footing.yaml`, `75-the_surveyors_report.yaml` | 1 | dash comments |
| `_datafiles/world/dogmud/conversations/pairs/116_117.yaml`, `pairs/259_262.yaml`, `types/{employee,employer,family,friend}.yaml` | 1 | dash lines |
| `internal/guards/prose_dash_guard_test.go` | 1 | three dirs added |
| `internal/guards/coulee_dash_guard_test.go` | 1 | header comment only |
| `internal/questloop/backlog.go`, `backlog_test.go` | 2 | deleted |
| `internal/questloop/keys_test.go` | 2 | first test rewritten |
| `internal/questworld/questworld.go` | 2 | `BacklogPath`, `LoadBacklog` deleted |
| `main_quests.go`, `main.go` | 2 | both doors refuse |
| `quest_loop_smoke_test.go` | 2 | no ratchet |
| `modules/gmcp/gmcp.Quest.go`, `gmcp.Delete_refs.go`, `gmcp.Quest_loop_test.go`, `gmcp.Delete_refs_test.go` | 2 | backlog parameter gone |
| `internal/guards/{coulee,tier1,tier2,tier3,tier4,tier5}_backlog_guard_test.go` | 2 | deleted |
| `_datafiles/world/dogmud/quests/loop-backlog.txt` | 2 | deleted |
| `internal/questengine/destinations_rule.go` | 2 | comment |
| `internal/guards/closing_step_guard_test.go` | 2, 3 | new |
| `internal/quests/triggers.go`, `quests.go`, `samecommand.go` | 3 | events, loader |
| `internal/questengine/types.go`, `engine.go`, `dryrun.go`, `walkthrough.go` | 3 | `Verb` gone |
| `internal/questloop/grants.go` | 3 | `verb` signature part gone |
| `internal/usercommands/usercommands.go`, `read.go`, `skill.search.go`; `internal/actions/search.go` | 3 | raise sites |
| `modules/gmcp/gmcp.Quest.go`, `_datafiles/html/public/static/js/quests.js` | 3 | editor vocab |
| quests 28, 72, 75, 76, 77 YAML | 3 | 8 triggers |
| 25 test files naming `room_interact` | 3 | renamed |
| `.claude/skills/dogmud-authoring-quests/SKILL.md`, `.claude/commands/sketch-quest.md`, four `context.md`, roadmap, `docs/README.md` | 4 | docs |

---

### Task 1: Dash guards cover quests, dialogue and conversations

**Files:** listed in the file map under Task 1.

- [ ] **Step 1: Widen the guard (the failing test).** In `internal/guards/prose_dash_guard_test.go`, replace the comment and list head:

```go
// House style (dogmud-player-copy): no em dash, no en dash, and no spaced
// hyphen or spaced double hyphen (" - ", " -- ") standing in for one, in
// prose a player reads. M6 slice 2 (ledger row 55); quests, dialogue and
// conversations joined at the #375 closing step.
var proseDashDirs = []string{
	"combat-messages",
	"defense-messages",
	"narration",
	"messaging",
	"rooms",
	"mobs",
	"taunt-messages",
	"spells",
	"quests",
	"dialogue",
	"conversations",
}
```

(The old text from `// House style` through the closing `}` of `proseDashDirs`, lines 11 to 28, is replaced whole.)

- [ ] **Step 2: Run it and read the failures.**
Run: `go test ./internal/guards/ -run TestPlayerProseHasNoDashes -count=1`
Expected: FAIL. At least these lines are named: `quests/31-find_your_footing.yaml:28`, `:49`, `quests/75-the_surveyors_report.yaml:23`, `conversations/pairs/116_117.yaml:12`, `conversations/pairs/259_262.yaml:28`, and the four `conversations/types/*.yaml` description lines. If it names a line not in this plan (a spaced hyphen the grep missed), fix it the same way (rewrite with a comma, colon or full stop) and note it in the commit message.

- [ ] **Step 3: Fix the lines.** Each edit replaces the dash only; keep the rest of the line byte for byte.

`quests/31-find_your_footing.yaml` (fix all eight, comment lines too, so the folder holds no em or en dash):
- `:1` `# Quest 31: Find Your Footing (Pothole Coulee hub, optional orientation)`
- `:5` `#   31-school: room_enter trigger, Chrysalis School Hall (5213)`
- `:9` `#   31-shop: item_gain trigger, buying a cotton shirt (20008) at Coulee`
- `:10`, `:12`: replace ` — ` with `; ` (the backticked `` `buy` `` and `` `equip` `` stay).
- `:13` `#   31-lantern: room_enter trigger, The Drowned Lantern (5205)`
- `:28` the trailing comment becomes `# follow Trader Onna (schedule pothole_onna); a fixed room is wrong for part of the day`
- `:49` the trailing comment becomes `# follow Crier Toke (schedule pothole_toke); a fixed room is wrong for part of the day`
- `:94`, `:110`: replace ` — teaches` with `, teaches` (the `──` rule characters and backticks stay).

(The `──` box-drawing characters are U+2500, not dashes; leave them.)

`quests/75-the_surveyors_report.yaml:23`: trailing comment becomes `# follow Brennan the Surveyor (schedule gf_brennan); a fixed room is wrong for part of the day`.

Conversations:
- `pairs/116_117.yaml:12` `        text: "Mind the spill on table three. I'll get to it."`
- `pairs/259_262.yaml:28` `        text: "Back to the wall, corner seat, same spot every time."`
- `types/employee.yaml:4` `  another: deference, gentle complaint, shared routine.`
- `types/employer.yaml:4` `  under them: work talk, small instructions, dry approval.`
- `types/family.yaml:3` `  Generic banter between kin: easy familiarity, old griefs left`
- `types/friend.yaml:3` `  Generic friend banter: small talk, gossip, weather, drink,`

- [ ] **Step 4: Prove the folders are clean.**
Run each on its own (grep exits 1 on no match; that is the pass):
`grep -rn "[—–]" _datafiles/world/dogmud/quests _datafiles/world/dogmud/dialogue _datafiles/world/dogmud/conversations`
Expected: no output, exit 1.
Run: `go test ./internal/guards/ -run 'TestPlayerProseHasNoDashes|Coulee' -count=1`
Expected: PASS.

- [ ] **Step 5: Reword the Coulee guard's header.** In `internal/guards/coulee_dash_guard_test.go`, replace the comment sentence that begins `The global prose guard (prose_dash_guard_test.go)` and ends `(couleeDashWorldDirs).` with:

```go
// standing in for one. The global prose guard (prose_dash_guard_test.go)
// reads quests, dialogue and conversations line by line since the #375
// closing step; this guard stays because it also reads the two zones'
// behaviour trees and schedules, which the global guard does not, and it
// reads parsed YAML, so a trailing comment is never mistaken for prose
// (couleeDashWorldDirs).
```

Make sure the line before it still reads `// (dogmud-player-copy) bars em dashes, en dashes and a spaced double hyphen` so the sentence joins. Run `go vet ./internal/guards/`. Expected: no output.

- [ ] **Step 6: Commit.**

```bash
git add internal/guards/prose_dash_guard_test.go internal/guards/coulee_dash_guard_test.go \
  _datafiles/world/dogmud/quests/31-find_your_footing.yaml _datafiles/world/dogmud/quests/75-the_surveyors_report.yaml \
  _datafiles/world/dogmud/conversations/pairs/116_117.yaml _datafiles/world/dogmud/conversations/pairs/259_262.yaml \
  _datafiles/world/dogmud/conversations/types/employee.yaml _datafiles/world/dogmud/conversations/types/employer.yaml \
  _datafiles/world/dogmud/conversations/types/family.yaml _datafiles/world/dogmud/conversations/types/friend.yaml
git commit -F - <<'EOF'
fix(guards): the dash guard reads quests, dialogue and conversations

#375 closing step. The last 17 dash lines go, comments included, and
the three folders join proseDashDirs. The Coulee guard stays: it also
reads that zone's behaviour trees and schedules.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Retire the loop backlog; both doors refuse boot

**Files:** listed in the file map under Task 2.

- [ ] **Step 1: The guard first (failing).** Create `internal/guards/closing_step_guard_test.go`:

```go
package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The #375 closing step's guards. The quest loop backlog is retired: every
// loop and destination finding refuses boot, so there is no list to park a
// finding on, and nothing may bring one back.

const closingStepGuardFile = "closing_step_guard_test.go"

var backlogRevivers = regexp.MustCompile(`\b(Ratchet|ParseBacklog|FormatBacklog|LoadBacklog|BacklogPath)\b|loop-backlog\.txt`)

// matchingLines returns "line N: text" for each line of src that re matches.
func matchingLines(re *regexp.Regexp, src string) []string {
	var out []string
	for i, line := range strings.Split(src, "\n") {
		if re.MatchString(line) {
			out = append(out, fmt.Sprintf("line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	return out
}

// repoGoFiles lists every Go file at the repo root and under internal,
// modules, cmd and tools, except this guard file. It fails if it finds too
// few to mean anything.
func repoGoFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"internal", "modules", "cmd", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, d), func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !de.IsDir() && filepath.Ext(path) == ".go" && de.Name() != closingStepGuardFile {
				files = append(files, path)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if len(files) < 500 {
		t.Fatalf("found only %d Go files; the guard would check almost nothing", len(files))
	}
	return files
}

func TestLoopBacklogStaysRetired(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt")); err == nil {
		t.Error("quests/loop-backlog.txt is back; every loop finding refuses boot, so fix the content instead")
	}
	for _, path := range repoGoFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matchingLines(backlogRevivers, string(data)) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s %s: the loop backlog is retired (#375 closing step)", rel, m)
		}
	}
}

func TestLoopBacklogStaysRetired_GuardCanFail(t *testing.T) {
	src := "package x\n\nfunc f() { questloop.Ratchet(nil, nil) }\n// reads quests/loop-backlog.txt\nvar ok = 1\n"
	if got := matchingLines(backlogRevivers, src); len(got) != 2 {
		t.Fatalf("guard saw %d lines, want 2: %v", len(got), got)
	}
}
```

Run: `go test ./internal/guards/ -run TestLoopBacklogStaysRetired -count=1`
Expected: `TestLoopBacklogStaysRetired` FAILS, naming the backlog file and lines in `backlog.go`, `questworld.go`, `main_quests.go`, `quest_loop_smoke_test.go`, `gmcp.Quest.go`, `gmcp.Delete_refs.go`, `keys_test.go`, `destinations_rule.go` and the six backlog guards. `_GuardCanFail` PASSES.

- [ ] **Step 2: Delete the backlog.**

```bash
git rm -q internal/questloop/backlog.go internal/questloop/backlog_test.go \
  _datafiles/world/dogmud/quests/loop-backlog.txt \
  internal/guards/coulee_backlog_guard_test.go internal/guards/tier1_backlog_guard_test.go \
  internal/guards/tier2_backlog_guard_test.go internal/guards/tier3_backlog_guard_test.go \
  internal/guards/tier4_backlog_guard_test.go internal/guards/tier5_backlog_guard_test.go
```

In `internal/questworld/questworld.go` delete `BacklogPath` and `LoadBacklog` (the two functions and their comments, lines 321 to 336). Then `go build ./internal/questworld/` and drop any import it reports unused (`errors`, `os` or `util` only if the compiler names them).

- [ ] **Step 3: Rewrite the boot doors.** In `main_quests.go`, replace everything from `// destinationDoor logs every quest step` to the end of the file with:

```go
// destinationDoor refuses on any quest step whose minimap destination is
// unknown or mistagged (#375 phase 4; a refusal since the closing step): a
// panic at boot, an error on a reload. It logs the inferred count.
func destinationDoor(isReload bool) error {
	return destinationDoorFrom(isReload, func() ([]questloop.Finding, []string) {
		all := quests.GetAllQuests()
		qs := make([]*quests.Quest, 0, len(all))
		for i := range all {
			qs = append(qs, &all[i])
		}
		return questengine.Destinations(qs)
	})
}

// destinationDoorFrom is destinationDoor over any source, for tests.
func destinationDoorFrom(isReload bool, src func() ([]questloop.Finding, []string)) error {
	fs, inferred := src()
	mudlog.Info("quests", "inferred destinations", len(inferred))
	return refuseFindings(isReload, "quest destination", fs)
}

// questDoor is the boot door of the quest loop validators (#375 phase 2):
// every rule over the loaded quests and dialogue. Any finding refuses (the
// backlog that once parked known findings is retired). Load quests,
// dialogue, mobs, items, spells, recipes and factions first.
//
// Boot runs Validate only, never the walkthrough (questengine.Walk), which
// is the CI sweep's job (quest_loop_smoke_test.go).
func questDoor(isReload bool) error {
	return questDoorFrom(isReload, func() []questloop.Finding {
		return questloop.Validate(questworld.Registry())
	})
}

// questDoorFrom is questDoor over any finding source, so its refusals are
// testable without the shipped world.
func questDoorFrom(isReload bool, src func() []questloop.Finding) error {
	return refuseFindings(isReload, "quest loop", src())
}

// refuseFindings logs each finding and refuses: a panic at boot, an error
// on a reload (the dialogueDoor rule). No findings, no refusal.
func refuseFindings(isReload bool, what string, fs []questloop.Finding) error {
	if len(fs) == 0 {
		return nil
	}
	for _, f := range fs {
		mudlog.Error("quests", what, f.String())
	}
	msg := fmt.Sprintf("%d %s problems; see the log above", len(fs), what)
	if !isReload {
		panic(msg)
	}
	return errors.New(msg)
}
```

In `main.go` (around line 1981), replace

```go
	if err := questDoor(isReload); err != nil && reloadErr == nil {
		reloadErr = err
	}
	destinationDoor()
```

with

```go
	if err := questDoor(isReload); err != nil && reloadErr == nil {
		reloadErr = err
	}
	if err := destinationDoor(isReload); err != nil && reloadErr == nil {
		reloadErr = err
	}
```

- [ ] **Step 4: The CI sweep and the door tests.** In `quest_loop_smoke_test.go`:
  - Delete the `loopBacklogWriteEnv` const and its comment (lines 18 to 22).
  - Replace `TestSmoke_ShippedQuestsPassTheLoop` (comment and body) with:

```go
// TestSmoke_ShippedQuestsPassTheLoop runs the quest loop rules, the
// completability walkthrough and the destination rule over shipped content:
// any finding fails. Boot runs Validate and Destinations; only this sweep
// runs the walk.
func TestSmoke_ShippedQuestsPassTheLoop(t *testing.T) {
	loadShippedQuestWorld(t)

	w := questworld.Registry()
	fs := append(questloop.Validate(w), questengine.Walk(w, questworld.Sources())...)
	dest, inferred := questengine.Destinations(w.Quests)
	fs = append(fs, dest...)
	for _, s := range inferred {
		t.Logf("destination inferred: %s", s)
	}
	for _, f := range fs {
		t.Errorf("quest loop: %s", f.String())
	}
	t.Logf("quest loop: %d findings", len(fs))
}
```

  - Replace `TestQuestDoor_RefusesAFreshFinding`, `TestDestinationDoor_WarnsAndNeverRefuses` and `TestQuestLoop_GuardCanFail` (from the comment above the first to the end of the file) with:

```go
// TestQuestDoor_RefusesAnyFinding: a finding panics at boot and is an error
// on a reload; no findings pass.
func TestQuestDoor_RefusesAnyFinding(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	one := func() []questloop.Finding {
		return []questloop.Finding{{Rule: "hint", QuestId: 1, Where: "step start", Msg: "no hint"}}
	}
	none := func() []questloop.Finding { return nil }
	if err := questDoorFrom(true, one); err == nil {
		t.Error("a reload must return an error")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("boot must panic")
			}
		}()
		_ = questDoorFrom(false, one)
	}()
	if err := questDoorFrom(false, none); err != nil {
		t.Errorf("no findings must not refuse boot: %v", err)
	}
}

// TestDestinationDoor_RefusesAnyFinding: the same refusal for a destination
// finding; inferred destinations alone pass.
func TestDestinationDoor_RefusesAnyFinding(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	one := func() ([]questloop.Finding, []string) {
		return []questloop.Finding{{Rule: "walk-destination", QuestId: 1, Where: "step a", Msg: "m"}}, nil
	}
	inferredOnly := func() ([]questloop.Finding, []string) { return nil, []string{"quest 1 step a"} }
	if err := destinationDoorFrom(true, one); err == nil {
		t.Error("a reload must return an error")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("boot must panic")
			}
		}()
		_ = destinationDoorFrom(false, one)
	}()
	if err := destinationDoorFrom(false, inferredOnly); err != nil {
		t.Errorf("inferred destinations must not refuse boot: %v", err)
	}
}
```

  - Remove the `errors` import if the compiler reports it unused.

- [ ] **Step 5: The quest editor's door.** In `modules/gmcp/gmcp.Quest.go`:
  - In `questDeps`, delete the `backlog` field and reword its comment's last sentence to: `(#375 phase 2 editor door). Nil skips the loop rules.` In `realQuestDeps`, delete `backlog:    questworld.LoadBacklog,`.
  - Replace `loopVerdict` (comment and body) with:

```go
// loopVerdict runs the loop rules over the world before and after the edit.
// It refuses only what the edit adds: a finding in the proposed world that
// the current world does not have. It warns about this quest's findings
// that were already there. Boot refuses every finding, so in play the
// current world has none; the comparison keeps a save from being blamed for
// another quest's problem. Findings on other quests that the edit did not
// cause are not this save's business.
func loopVerdict(world questloop.World, q quests.Quest) (refuse, warn []string) {
	before := map[string]bool{}
	for _, f := range questloop.Validate(world) {
		before[f.Key()] = true
	}
	for _, f := range questloop.Validate(questworld.WithQuest(world, q)) {
		k := f.Key()
		switch {
		case before[k] && f.QuestId == q.QuestId:
			warn = append(warn, "known (already in the world): "+f.String())
		case !before[k]:
			refuse = append(refuse, f.String())
		}
	}
	return refuse, warn
}
```

  - In `buildQuestUpdate` (around line 248), replace the block from `if d.loopWorld != nil && d.backlog != nil {` to its closing `}` with:

```go
	if d.loopWorld != nil {
		refuse, known := loopVerdict(d.loopWorld(d.validators), q)
		if len(refuse) > 0 {
			return buildErr("quest %d refused:\n%s", q.QuestId, strings.Join(refuse, "\n"))
		}
		warns = append(warns, known...)
	}
```

  In `modules/gmcp/gmcp.Delete_refs.go`:
  - Header comment: replace `a finding the world already has, or one the loop backlog lists, is not the delete's doing.` with `a finding the world already has is not the delete's doing.`
  - `questRefsWithout`, `questRefsWithoutRoom`, `questRefsWithoutItem`, `questRefsWithoutMob`: drop the `b questloop.Backlog` parameter and its pass-through; in `questRefsWithout` the condition becomes `if f.Rule == "refs" && !before[f.Key()] {`.
  - Delete `loopWorldAndBacklog`. Each caller (`mobQuestRefs`, `roomQuestRefs`, `itemQuestRefs`) replaces `w, b := loopWorldAndBacklog()` with `w := questworld.Registry()` and drops `b` from its call.

  Tests:
  - `modules/gmcp/gmcp.Delete_refs_test.go`: delete every `questloop.Backlog{}, ` argument (seven calls). Drop the `questloop` import only if the compiler says it is unused.
  - `modules/gmcp/gmcp.Quest_loop_test.go`: `loopFixture` loses its `backlog questloop.Backlog` parameter and the `d.backlog = ...` line; every call drops its last argument. Delete `TestBuildQuestUpdate_RealRules_BacklogFindingSavesWithAWarning` whole (the behaviour it tested is gone). In `TestBuildQuestUpdate_RealRules_OwnStandingFindingWarns`, if it asserts the old warning text `"known (already in the world, not in the loop backlog)"`, change it to `"known (already in the world)"`.

- [ ] **Step 6: The key test.** In `internal/questloop/keys_test.go`, replace `TestKeys_RefsErrorsDoNotCollide` (comment and body) with:

```go
// Two different refs errors on one quest have two keys, so fixing one can
// never be mistaken for fixing the other.
func TestKeys_RefsErrorsDoNotCollide(t *testing.T) {
	v := quests.QuestValidators{MobExists: func(id int) bool { return id < 424242 }}
	fillValidators(&v)

	w := goodWorld()
	w.Refs = &v
	w.Quests[0].Triggers[0].Mob = 424242
	w.Quests[0].Rewards.ItemId = 0
	w.Quests[0].Steps[0].Destination = quests.StepDestination{Mobs: []int{424243}, Room: 5}
	keys := map[string]bool{}
	n := 0
	for _, f := range Validate(w) {
		if f.Rule == "refs" {
			keys[f.Key()] = true
			n++
		}
	}
	require.Equal(t, 2, n, "two refs findings")
	require.Len(t, keys, 2, "each refs error has its own key")
}
```

- [ ] **Step 7: Comments.** `internal/questengine/destinations_rule.go:13-16`: the doc comment becomes

```go
// Destinations is the static destination rule (#375 phase 4, spec section 5).
// Boot and the CI sweep refuse on any of its findings (since the #375
// closing step). Secret quests and each quest's final step are skipped. It
// returns the findings sorted by key, and "quest <id> step <step>" for each
// step whose destination is inferred.
```

and at `:64` replace `A single id keeps the message the backlog` with `A single id keeps the message the old backlog` only if the guard flags that line; otherwise leave it (the guard regexp does not match the word "backlog" alone).

- [ ] **Step 8: Run.**
Run: `go build ./... && go vet ./ ./internal/questloop/ ./internal/questworld/ ./modules/gmcp/`
Expected: no output.
Run: `go test ./ ./internal/questloop/ ./internal/questworld/ ./internal/questengine/ ./modules/gmcp/ ./internal/guards/ -count=1`
Expected: PASS, including `TestLoopBacklogStaysRetired`.
Run: `DOGMUD_BOOT_SMOKE=1 go test ./ -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_Boot' -count=1 -v 2>&1 | tail -20`
Expected: PASS and `quest loop: 0 findings`.
Then: `grep -rn "Backlog\|backlog" --include=*.go . | grep -v _archive` and read every hit; a hit outside a comment is a leftover to fix, a comment that describes the old backlog as current is reworded.

- [ ] **Step 9: Commit.** Stage by name: the guard file, `main_quests.go`, `main.go`, `quest_loop_smoke_test.go`, `internal/questworld/questworld.go`, `internal/questloop/keys_test.go`, `internal/questengine/destinations_rule.go`, the four `modules/gmcp` files (the `git rm` paths are already staged).

```bash
git commit -F - <<'EOF'
feat(quests): retire the loop backlog; every finding refuses boot

#375 closing step. The ratchet, loop-backlog.txt and the six backlog
guards go. Boot refuses any loop finding and now any destination
finding too; the CI sweep fails on any finding from Validate, the walk
or Destinations. A guard keeps the backlog from coming back.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `room_interact` becomes `look_at` and `search_at`

**Files:** listed in the file map under Task 3. The compiler enumerates most consumers once `Verb` and `InteractVerb` are gone; follow it (dogmud-refactoring: delete first, let the compiler list the rest).

- [ ] **Step 1: The guard (failing).** Append to `internal/guards/closing_step_guard_test.go`:

```go
// room_interact was split into look_at and search_at at the #375 closing
// step. The name may appear only in the loader's refusal (the
// retiredInteractEvent constant and the lines that use it).
var roomInteractRevivers = regexp.MustCompile(`room_interact|RoomInteract`)

func roomInteractLines(src string) []string {
	var out []string
	for _, m := range matchingLines(roomInteractRevivers, src) {
		if !strings.Contains(m, "retiredInteractEvent") {
			out = append(out, m)
		}
	}
	return out
}

func TestRoomInteractStaysRetired(t *testing.T) {
	root := repoRoot(t)
	files := repoGoFiles(t)
	for _, d := range []string{
		filepath.Join(root, "_datafiles", "world"),
		filepath.Join(root, "_datafiles", "html"),
	} {
		err := filepath.WalkDir(d, func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ext := filepath.Ext(path); !de.IsDir() && (ext == ".yaml" || ext == ".js") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range roomInteractLines(string(data)) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s %s: use look_at or search_at (#375 closing step)", rel, m)
		}
	}
}

func TestRoomInteractStaysRetired_GuardCanFail(t *testing.T) {
	src := "  - event: room_interact\n    noun: stone\nnotifyRoomInteract(u)\nif t.Event == retiredInteractEvent {\n"
	if got := roomInteractLines(src); len(got) != 2 {
		t.Fatalf("guard saw %d lines, want 2: %v", len(got), got)
	}
}
```

Run: `go test ./internal/guards/ -run TestRoomInteractStaysRetired -count=1`
Expected: the main test FAILS listing the 8 triggers, 2 quest 28 comments and the Go sites; `_GuardCanFail` PASSES.

- [ ] **Step 2: The quests package.** `internal/quests/triggers.go`:
  - The `Event` field's comment: replace `room_interact` with `look_at, search_at`.
  - Replace `Verb string ...` with:

```go
	RetiredVerb string `yaml:"verb,omitempty" json:"-"` // retired at the #375 closing step (the event names look or search); read only so the loader can refuse it
```

  - Replace the block from `// The two room_interact verb families` through the end of `InteractVerb` with:

```go
// The room feature events (#375 phase 3a, owner ruling 5: look and search
// are the only interaction verbs; one event each since the closing step).
// look_at: look, l, examine, exam or read names a room feature. search_at:
// search names it (after the search runs, no roll), or a plain search
// uncovers it at the room difficulty.
const (
	EventLookAt   = "look_at"
	EventSearchAt = "search_at"
)
```

  `internal/quests/quests.go`:
  - `:93-96` becomes:

```go
// countSwallowsCommand are the events whose callers read a handled result as
// "the quest took this command" (room_gate, look_at, search_at, item_give),
// so they take no count.
var countSwallowsCommand = map[string]bool{"room_gate": true, EventLookAt: true, EventSearchAt: true, "item_give": true}

// retiredInteractEvent is the event look_at and search_at replaced at the
// #375 closing step; the loader names them when it refuses it.
const retiredInteractEvent = "room_interact"
```

  - `:141` message: `(count is refused on room_gate, look_at, search_at and item_give)`.
  - `validEvents`: replace `"room_interact": true,` with `EventLookAt: true, EventSearchAt: true,`.
  - Directly after the `if t.Event == "" {...}` block, insert:

```go
		if t.Event == retiredInteractEvent {
			return fmt.Errorf("quest %d (%s): trigger %d uses event %s, which is retired: use %s (look, examine, read) or %s (search)", r.QuestId, r.Name, i, retiredInteractEvent, EventLookAt, EventSearchAt)
		}
```

  - Replace the whole `if t.Verb != "" {...}` block and its two comment lines with:

```go
		// The event names the verb family since the #375 closing step.
		if t.RetiredVerb != "" {
			return fmt.Errorf("quest %d (%s): trigger %d sets verb %q; verb is retired: use event %s for look and %s for search", r.QuestId, r.Name, i, t.RetiredVerb, EventLookAt, EventSearchAt)
		}
```

  `internal/quests/samecommand.go`: delete the three lines starting `// An empty room_interact verb means look` through the closing `}` of that `if`. The caller already skips pairs whose events differ (`a.Event != b.Event`), which is what kept look and search apart.

  `internal/questloop/grants.go:118`: delete `addStr("verb", t.Verb)`.

- [ ] **Step 3: The engine.** `internal/questengine/types.go:48`: delete `Verb       string`.
  `internal/questengine/engine.go`: delete the comment `// Only room_interact carries a verb...` and the `if t.Verb != "" || d.Verb != "" {...}` block (lines 379 to 385).
  `internal/questengine/dryrun.go`: replace `SearchFeatures` comment and loop head:

```go
// SearchFeatures returns the sorted, distinct nouns of search_at triggers
// whose Room is roomId. These are the things a player can search in that
// room for a quest, whatever the triggers' conditions.
func (e *Engine) SearchFeatures(roomId int) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range e.triggerIndex[quests.EventSearchAt] {
		if t.def.Room != roomId || t.def.Noun == "" {
			continue
		}
```

  and change the import to `import (` `"sort"` and `"github.com/GoMudEngine/GoMud/internal/quests"` `)`.
  `internal/questengine/walkthrough.go`: delete the `if t.Event == "room_interact" {...}` block (`:847-851`, four lines and the closing brace); at `:684` replace `quest 72's room_interact set_flag` with `quest 72's look_at set_flag`.

- [ ] **Step 4: The raise sites.** `internal/usercommands/usercommands.go`:
  - Replace the block from `// Quest engine: room_interact for the look family only` through its closing `}` with:

```go
		// Quest engine: look_at for the look family (#375 phase 3a, owner
		// ruling 5: look and search are the only interaction verbs; l,
		// examine and exam expand to look before this runs). read raises it
		// itself when nothing carried matches, and search raises search_at
		// after the search runs (skill.search.go), so a cooldown refusal
		// sends nothing.
		if !skipScript && alias == `look` {
			if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
				if notifyRoomFeature(user, room, quests.EventLookAt, rest, room.NounNamedBy(rest)) {
					return true, nil
				}
			}
		}
```

  - Replace `interactionFamily` and `notifyRoomInteract` (comments and bodies, `:586-611`) with:

```go
// notifyRoomFeature raises a room feature event (quests.EventLookAt or
// quests.EventSearchAt), matched on the template room
// (rooms.OriginalRoomId, as command_issued is) so a trigger fires inside an
// instance; the bridge keeps the real room. It reports whether a trigger
// handled the event.
func notifyRoomFeature(user *users.UserRecord, room *rooms.Room, event, typed, roomNoun string) bool {
	matchRoom, _ := rooms.OriginalRoomId(room.RoomId)
	bridge := questengine.NewGameBridge(user, room.RoomId)
	return questengine.GetEngine().Notify(event, questengine.EventDetails{
		UserId:   user.UserId,
		RoomId:   matchRoom,
		Noun:     strings.ToLower(strings.TrimSpace(typed)),
		RoomNoun: roomNoun,
	}, bridge, bridge).Handled
}
```

  (Check the variable is named `alias` at that point, as `interactionFamily(alias)` used it.)
  - `internal/usercommands/read.go:62`: `notifyRoomFeature(user, room, quests.EventLookAt, rest, room.NounNamedBy(rest))`.
  - `internal/usercommands/skill.search.go`: the comment at `:72` becomes `// #375 phase 3a: search_at fires after the search has run, so a trigger on`, then `// that feature fires with no roll.` and keep the rest of that comment; `:78` and `:81` call `notifyRoomFeature(user, room, quests.EventSearchAt, ...)` with the same last three arguments; `questSearchSpots`' comment says `search_at triggers` for `` `verb: search` room_interact triggers ``; its `WouldFire` call becomes `engine.WouldFire(quests.EventSearchAt, questengine.EventDetails{UserId: user.UserId, RoomId: matchRoom, Noun: spot, RoomNoun: spot}, bridge)`.
  - `internal/actions/search.go:100,133,599`: reword `` `verb: search` room_interact triggers `` to `search_at triggers` and `raises room_interact` to `raises search_at`.

- [ ] **Step 5: The editor's vocabulary** (the editor is off; keep it true). `modules/gmcp/gmcp.Quest.go`: the comment at `:150` names `room_gate, look_at, search_at and item_give`; replace the `room_interact` vocab row with:

```go
	{"look_at", "player looked at a room feature: look, l, examine, exam or read naming it (filters: room, noun, area, zone; no count)"},
	{"search_at", "player searched a room feature: search naming it, after the search runs and with no roll, or a plain search that uncovered it (filters: room, noun, area, zone; no count)"},
```

  `_datafiles/html/public/static/js/quests.js`: the comment at `:519` names `room_gate, look_at, search_at and item_give`; replace `room_interact: ["room", "noun", "verb", "area", "zone"],` with

```js
    look_at: ["room", "noun", "area", "zone"],
    search_at: ["room", "noun", "area", "zone"],
```

  and delete `verb: "text", ` from `FILTER_KINDS`. Run `node tools/jstest` if the repo has it (`ls tools | grep -i jstest`), else `go test ./modules/gmcp/ -run TestQuestBuilderFiltersMatchTheVocab -count=1`.

- [ ] **Step 6: Content.** In each file, `event: room_interact` becomes `event: look_at`, except quest 76:
  - `quests/28-waking_to_gaius.yaml:281`, `:297`; `quests/72-the_water_dispute.yaml:81`, `:104`; `quests/75-the_surveyors_report.yaml:62`, `:81`; `quests/77-the_truth.yaml:109`: `look_at`.
  - `quests/76-the_disc.yaml:156-157`: `event: search_at` and delete the `verb: search` line.
  - `quests/28-waking_to_gaius.yaml:271`: `look_at trigger, so look, l, examine, exam` (replacing `room_interact, so look, l, examine, exam`); `:274`: `look_at skips command_issued, so an earlier` (replacing `room_interact skips command_issued, so an earlier`). Re-read both comments whole after the edit so they still read as sentences.
  - Then `grep -rn "verb:" _datafiles/world/dogmud/quests` must print nothing (exit 1).

- [ ] **Step 7: Tests.** `go build ./... && go vet ./...` lists every test that still uses `Verb`, `InteractVerb`, `VerbLook`, `VerbSearch`, `interactionFamily` or `notifyRoomInteract`; `grep -rln "room_interact\|RoomInteract" --include=*_test.go .` lists the rest (25 files at the start). Convert each by these rules, and read each test's intent before editing:
  1. A trigger `{Event: "room_interact", Verb: "search", ...}` becomes `{Event: "search_at", ...}`. `Verb: ""` or `Verb: "look"` becomes `Event: "look_at"` with no verb.
  2. An `EventDetails{..., Verb: "search", ...}` sent or dry-run on `"room_interact"` is sent on `"search_at"` without `Verb`; `Verb: "look"` goes to `"look_at"`.
  3. A test that sent a verb other than look or search (`noun_room_test.go:110` uses `"get"`; `noun_pairs_guard_test.go:66` uses `"touch"`) was proving that another verb does not fire a look trigger. Convert it to prove a `search_at` event does not fire a `look_at` trigger (and the reverse where the test had both), which is the boundary the split now holds.
  4. `noun_pairs_guard_test.go:83` compares `a.Verb != b.Verb`: drop that term (the event comparison covers it).
  5. `internal/quests/narration_test.go` `TestValidateRoomInteractVerbs` becomes `TestValidateRetiredInteract`: a trigger with `Event: retiredInteractEvent` is refused and the error names `look_at` and `search_at`; a trigger with `RetiredVerb: "search"` on `look_at` is refused and names both; `look_at` and `search_at` with no verb load; `Count: 2` on either is refused.
  6. `internal/usercommands/room_interact_family_test.go`: rename it with `git mv` to `look_at_family_test.go`; tests named `TestRoomInteract_*` become `TestLookAt_*` (or `TestSearchAt_*` where they test search); the look-family loop drops its inner verb loop (`"", "look"`) and runs once per command on `look_at`.
  7. Guards: `quest_search_light_guard_test.go:57` becomes `isSpot := tr.Event == quests.EventSearchAt`; `search_trigger_noun_guard_test.go:26` `if tr.Event != quests.EventSearchAt || tr.Noun == "" {`, `:51` `if tr.Event == quests.EventSearchAt {`, `:115` `{Event: quests.EventSearchAt, Room: room, Noun: noun}`, `:130` `{Event: quests.EventLookAt, Room: 1, Noun: "chandelier"}`; reword their comments from `` `verb: search` room_interact `` to `search_at`.
  8. Comments in tests that say `room_interact` are reworded to the event they now use.

  Run: `go test ./internal/quests/ ./internal/questloop/ ./internal/questengine/ ./internal/usercommands/ ./internal/guards/ ./modules/gmcp/ ./ -count=1`
  Expected: PASS, including `TestRoomInteractStaysRetired`.
  Run: `DOGMUD_BOOT_SMOKE=1 go test ./ -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_Boot' -count=1 -v 2>&1 | tail -20`
  Expected: PASS, `quest loop: 0 findings`.
  Narration goldens: if `internal/narration` has a snapshot that prints trigger signatures, run `go test ./internal/narration/ -count=1`; a diff may show only `room_interact` becoming `look_at`/`search_at` and `verb=search` disappearing. Anything else stops the task.

- [ ] **Step 8: Commit.** Stage every file changed in Steps 1 to 7 by name (`git status --short` lists them; check `git diff --cached --stat` before committing).

```bash
git commit -F - <<'EOF'
feat(quests): room_interact becomes look_at and search_at

#375 closing step. The event names the verb family, so the verb field,
InteractVerb and EventDetails.Verb go. The loader refuses room_interact
and verb: with a message naming the new events. Eight triggers in quests
28, 72, 75, 76 and 77 move; behaviour is unchanged. A guard keeps the old
name out of Go, world YAML and the editor's JS.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Docs

- [ ] **Step 1: Authoring docs.** `.claude/skills/dogmud-authoring-quests/SKILL.md:156-165` and `:550`, and `.claude/commands/sketch-quest.md:135`, `:344-346`: every `room_interact` becomes `look_at` (or `search_at` where the text is about searching), and any mention of `verb: search` becomes `search_at`. Keep each rule; change only the event names. Add one line under the trigger list in the skill: `` `look_at` and `search_at` replaced `room_interact` at the #375 closing step; the loader refuses the old name and any `verb:` key. ``
- [ ] **Step 2: `context.md`.** For `internal/questloop`, `internal/quests`, `internal/questengine`, `internal/questworld`: remove `Backlog`, `ParseBacklog`, `FormatBacklog`, `Ratchet`, `LoadBacklog`, `BacklogPath`, `InteractVerb`, `VerbLook`, `VerbSearch`, `EventDetails.Verb`; add `EventLookAt`, `EventSearchAt`, `RetiredVerb`; say the loop rules and destinations refuse boot. Run `python tools/context_md_audit.py` and fix what it names in these four.
- [ ] **Step 3: Roadmap.** `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`:
  - `:24-25`, the event list and counts: replace `room_interact` with `look_at, search_at` in the list; the counts row gets a note `(2026-10-17 counts; room_interact split into look_at and search_at at the closing step)`.
  - `:246`: `Keep room_interact loading until phase 7 removes its last use, then delete it.` becomes `room_interact was split into look_at and search_at at the closing step (2026-11-06).`
  - The closing step paragraph (`:353-354`) gets a status line after it: `**Done 2026-11-06** (spec and plan 2026-11-06-quest-arc-closing-step): every loop and destination finding refuses boot and the backlog is gone; room_interact is look_at and search_at; the dash guard reads quests, dialogue and conversations.`
  - The tier 5 paragraph's last sentence (`:390-391`) becomes `The closing step followed on 2026-11-06.`
  - Do not touch `QUEST_DIALOGUE_REQUIREMENTS.md`: its boxes are the owner's.
- [ ] **Step 4: `docs/README.md`.** The spec and plan rows were added at planning. Search the README for rows describing the six deleted backlog guards or `loop-backlog.txt`; reword each to say it was retired at the closing step (do not delete history rows).
- [ ] **Step 5:** `grep -rn "room_interact\|loop-backlog" docs/roadmaps docs/guides .claude` and read each hit: live text is updated, history (dated entries) is left.
- [ ] **Step 6: Commit** the named files: `docs(quests): closing step docs, skill and context.md`.

---

### Task 5: Verify (controller)

- [ ] **Step 1:** `go build ./... && go vet ./...` then `go test ./... -count=1` (read the exit code; a failure stops here). Then `DOGMUD_BOOT_SMOKE=1 go test ./ -count=1`.
- [ ] **Step 2: Negative boots.** Prove each refusal can fire, in a scratch copy only (never edit shipped data for this): a quest fixture with `event: room_interact` fails to load with the naming message (covered by Task 3's loader test; confirm it ran); `questDoorFrom` and `destinationDoorFrom` panic tests ran.
- [ ] **Step 3: Two boots** from a detached worktree at the branch head, per `dogmud-shipping` (private port, kill by PID only). Both reach the listening line with no `quests` error lines.
- [ ] **Step 4: Playtest** per `dogmud-playtesting`, one seat, local, private port: Dewey's threshold stone (`look stone` in room 6258 after `28-examined`: Dewey speaks once, names repeat on a second look), the disc's `search stone` in room 4023 (quest 76, the trigger's text appears with no roll), a plain `search` in a room with a quest search spot uncovering it, and `examine` and `read` on a feature firing the look trigger. File findings as GitHub issues (`--repo pruuk/DOGMud`).
- [ ] **Step 5:** `git log --oneline master..HEAD`, `git status` clean, PR with `gh pr create --repo pruuk/DOGMud`. The body does not use a closing keyword for #375 (the arc stays open for the owner's boxes).
