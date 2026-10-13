# Messaging M6 Row 7: Craft Room Lines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every craft, by a player or a mob, tells the room when it starts, succeeds and fails, judged per reader by sight, from per-skill pools in one new store.

**Architecture:** A new world file `crafting-messages.yaml` holds five pools per skill, loaded and validated by `internal/crafting` (`LoadCraftingMessages`, `CraftRoomLines`, `CraftHeardLine`). One sender, `actions.NarrateCraftPhase`, partitions the room by sight and sends the seen line (item named or "something"), or the heard line. All eight craft room-line sites call it; the mob-only Go lines are deleted.

**Tech Stack:** Go, yaml.v2, the `internal/narration` render core, `internal/messaging` sight predicates, `internal/rooms` senders.

**Spec:** `docs/superpowers/specs/2026-10-13-messaging-m6-row7-craft-room-lines-design.md` (owner approved 2026-10-13).

---

## Rules for every task (restate these in every subagent brief)

- **Edit tool only** for text edits. No `sed -i`, no heredoc appends, no Python read-modify-write, no `gofmt -w` on files you did not just write. Run `gofmt -l <file>` and fix by Edit.
- **Never `git add -A` / `git add .`**. Name paths.
- **No em or en dashes** in any Go comment, string, YAML or doc you write. No spaced ` - ` or `--` in prose either.
- **Every YAML line quoted** with double quotes. A `: ` inside an unquoted scalar turns a list item into a map.
- **After every task run `go test .` (root guards) as well as the package tests.** Several root guards key on line numbers.
- Work happens in the worktree `C:/tmp/dogmud-m6r7` on branch `feat/m6-row7-craft-room-lines` (created in Task 0).

## Facts verified against source (master `43d4eda93`, 2026-10-13)

| Fact | Where |
|---|---|
| `type Phase uint8`; `PhaseSuccess Phase = iota`, `PhaseFailure` | `internal/crafting/narration.go:14-18` |
| `RecipeSpec.Narration(p)` switch has only those two cases | `internal/crafting/narration.go:24-33` |
| `narration.Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles`; `Variants{Actor, Actee, Observer, ActeeObserver []string}` | `internal/narration/render.go:23,92` |
| `narration.Substitute` replaces any key in the map, so a store-local token rides in the same map | `internal/narration/render.go:146` |
| `narration.ValidateVariants(v, minVariants int, expected ...Role) error`; `RoleActor`, `RoleObserver` | `internal/narration/render.go:213` |
| `narration.Picker func(n int) int`, `DefaultPicker`, `SequencePicker()`, `FirstPicker` | `internal/narration/picker.go` |
| Store-specific event tokens are an established convention; `movenarration.TokenItem = "{item}"` | `internal/narration/render.go:124-127`, `internal/movenarration/store.go:32` |
| `textutil.TokenContext{ActorName, ActorPlainName, ActeeName, ActeePlainName}`, `.Tokens()` | `internal/textutil/tokens.go:14-30` |
| `LoadCastingMessages` pattern (read, yaml.v2 unmarshal, Validate, panic) | `internal/spells/casting_messages.go:114-133` |
| `main.go:1642` `crafting.LoadRecipeFiles()` | `main.go` |
| `crafting.GetAll()` returns the recipe map | used at `internal/narration/snapshot_test.go:1101` |
| The default world has no `recipes/`; `filepath.Walk` on a missing root errors, so `LoadRecipeFiles` already panics there. The default world is not bootable today; no default copy of the new file is shipped (spec corrected in Task 7) | `internal/crafting/crafting.go:92-96`, `internal/fileloader/fileloader.go:165` |
| `actions.Actor` interface; `*UserActor{User, Room}`, `*MobActor{Mob, Room}`; `NewUserActorInRoom`, `NewMobActorInRoom` | `internal/actions/actor.go:14`, `actor_user.go:30`, `actor_mob.go:30` |
| `UnseenBy(room, c) []int`, `MobSeenName(mob, room, viewingUserId int) string` | `internal/actions/perception.go:83,42` |
| `messaging.ReaderSight(c, room) SightDecision`; `SightFull`, `SightShapes`, `SightNone`; a sleeper is `SightNone` | `internal/messaging/predicates.go:225` |
| `room.SendTextVisualHidingNames(cat, txt, names []string, exclude ...int)` | `internal/rooms/rooms.go:288` |
| `room.SendTextUnsighted(cat, txt, exclude ...int)` reaches every `SightNone` reader, sleepers included | `internal/rooms/rooms.go:566` |
| `items.SpecDisplayName(itemId int) string`, "" when no spec; title cases ("iron dagger" renders "Iron Dagger") | `internal/items/items.go:502`, `internal/mobcommands/craft_begin_line_test.go:23` |
| Same-room sound wording precedent is "close by" (`You hear fighting close by.`, `cast_started_unseen`) | `internal/hooks/NewRound_DoCombat_helpers.go:465`, `_datafiles/world/dogmud/casting-messages.yaml:48-51` |
| Player craft room-line sites: tick success `:668-675`, tick failure `:705-712` | `internal/hooks/NewRound_UserRoundTick.go` |
| Player instant `craft.go:198-202`, start `:205-208`, `completeCraft` `:763-773`; `craftDeliver` `:33`, `craftDeliverInstant` `:55` | `internal/usercommands/craft.go` |
| Mob instant `:55-59`, start `:68-70`, `announceCraftBegin` `:81-99`, `craftBeginLine` `:105-114` | `internal/mobcommands/craft.go` |
| Mob tick success `:613-621`, failure `:622-634` | `internal/hooks/NewRound_MobRoundTick.go` |
| `RecipeSpec.MobRoomLine` | `internal/crafting/mob_room_line.go:13` |
| Tests on retired code: `TestMobRoomLine_AuthoredLineElseFallback` (`internal/crafting/narration_test.go:98`), `craft_begin_line_test.go` (2 tests), `craft_instant_room_hiding_test.go` (`:138`, `:204`, `:237` use an authored `success_observer`) | as named |
| Sender test helpers: `phaseScene`, `phaseHide`, `phaseMob`, `phaseLines`, `requireOneLine`, ids 7351-7353, mob 98351 | `internal/actions/spell_phase_lines_test.go`, `drink_room_line_sight_test.go:85` |
| Root guards that list bare world files: `proseDashFiles` | `prose_dash_guard_test.go:32` |
| Shipped narration data guard: per-store subtests (`casting` at `:127`) and `narrationStoreWalkRoots` (`:279`) | `shipped_narration_data_guard_test.go` |
| Snapshot harness: `setupRealStores` (`:138`), casting builder (`:714`), goldens under `internal/narration/testdata/stores/`, `-update` flag | `internal/narration/snapshot_test.go` |
| Viewpoint registry rows for the tick sites keyed `hooks/NewRound_UserRoundTick.go|<ansi fg="green">%s</ansi>` and `...red...` | `messaging_surface_guard_test.go:1282-1283` |
| Spell one-body guard pattern | `spell_phase_narration_guard_test.go` |

## File structure

| File | Responsibility |
|---|---|
| Create `internal/crafting/messages.go` | The store: types, load, validate, render, test seam |
| Create `internal/crafting/messages_test.go` | Store unit tests |
| Create `_datafiles/world/dogmud/crafting-messages.yaml` | The copy: six skills x five pools x three lines |
| Modify `internal/crafting/narration.go` | `PhaseStart` |
| Delete `internal/crafting/mob_room_line.go` | Retired |
| Create `internal/actions/craft_phase_lines.go` | `NarrateCraftPhase`, the one sender |
| Create `internal/actions/craft_phase_lines_test.go` | Sender tests |
| Modify `internal/usercommands/craft.go`, `internal/hooks/NewRound_UserRoundTick.go` | Player sites |
| Modify `internal/mobcommands/craft.go`, `internal/hooks/NewRound_MobRoundTick.go` | Mob sites |
| Create `craft_phase_narration_guard_test.go` (repo root) | One-sender guard |
| Modify `main.go`, `prose_dash_guard_test.go`, `shipped_narration_data_guard_test.go`, `internal/narration/snapshot_test.go`, `messaging_surface_guard_test.go` | Wiring and guards |
| Modify `internal/crafting/context.md`, `internal/actions/context.md` | Package docs |

---

### Task 0: Worktree

- [ ] **Step 1: Create the worktree from master**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin master
git worktree add -b feat/m6-row7-craft-room-lines C:/tmp/dogmud-m6r7 origin/master
```

- [ ] **Step 2: Baseline**

Run (in `C:/tmp/dogmud-m6r7`): `go build ./... && go test ./internal/crafting/ ./internal/actions/ .`
Expected: PASS. If anything fails on a clean master, stop and report.

---

### Task 1: The store (`internal/crafting/messages.go`)

**Files:**
- Modify: `internal/crafting/narration.go:14-18`
- Create: `internal/crafting/messages.go`
- Test: `internal/crafting/messages_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/crafting/messages_test.go`:

```go
package crafting

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func validPools() SkillPools {
	return SkillPools{
		StartObserver:   []string{"{actor} begins {item}.", "{actor} starts {item}.", "{actor} sets to {item}."},
		SuccessObserver: []string{"{actor} finishes {item}.", "{actor} completes {item}.", "{actor} is done with {item}."},
		FailureObserver: []string{"{actor} ruins it.", "{actor} spoils it.", "{actor} gives up."},
		StartHeard:      []string{"You hear work begin.", "You hear tools.", "You hear a clatter."},
		FinishHeard:     []string{"The work stops.", "The tools fall quiet.", "The clatter ends."},
	}
}

func TestCraftingMessagesValidate_AcceptsAWellFormedStore(t *testing.T) {
	cm := CraftingMessages{"cooking": validPools()}
	if err := cm.Validate([]string{"cooking"}); err != nil {
		t.Fatalf("valid store refused: %v", err)
	}
}

func TestCraftingMessagesValidate_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(cm CraftingMessages)
		skills []string
		want   string
	}{
		{"missing skill block", func(cm CraftingMessages) {}, []string{"cooking", "tailoring"}, `no block for skill "tailoring"`},
		{"unknown skill block", func(cm CraftingMessages) { cm["cookery"] = validPools() }, []string{"cooking"}, `skill "cookery" has no recipes`},
		{"shallow pool", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.StartHeard = p.StartHeard[:2]
			cm["cooking"] = p
		}, []string{"cooking"}, "start_heard"},
		{"seen line without actor", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.SuccessObserver = []string{"A stew is done.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "success_observer must name the crafter with {actor}"},
		{"item in a failure line", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.FailureObserver = []string{"{actor} burns {item}.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "failure_observer may not use {item}"},
		{"token in a heard line", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.FinishHeard = []string{"{actor} stops.", "b.", "c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "finish_heard may not use {actor}"},
		{"unknown token", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.StartObserver = []string{"{actor} begins {itme}.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "start_observer may not use {itme}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cm := CraftingMessages{"cooking": validPools()}
			tc.mutate(cm)
			err := cm.Validate(tc.skills)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestCraftRoomLines_FullNamesTheItemVagueSaysSomething(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}

	full, vague := CraftRoomLines("cooking", PhaseStart, ctx, "Stew", narration.FirstPicker)
	if full != "Kael begins Stew." || vague != "Kael begins something." {
		t.Fatalf("got full %q vague %q", full, vague)
	}
	full, vague = CraftRoomLines("cooking", PhaseSuccess, ctx, "", narration.FirstPicker)
	if full != "Kael finishes something." || vague != "Kael finishes something." {
		t.Fatalf("no item: got full %q vague %q", full, vague)
	}
	full, _ = CraftRoomLines("cooking", PhaseFailure, ctx, "Stew", narration.FirstPicker)
	if full != "Kael ruins it." {
		t.Fatalf("failure: got %q", full)
	}
}

// One index for both renders: the vague line must be the same variant as the
// full one, or a shapes reader and a full reader would describe different
// moments.
func TestCraftRoomLines_FullAndVagueShareOneVariant(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}
	pick := narration.SequencePicker()
	for i := 0; i < 3; i++ {
		full, vague := CraftRoomLines("cooking", PhaseStart, ctx, "Stew", pick)
		if strings.Replace(full, "Stew", "something", 1) != vague {
			t.Fatalf("round %d: full %q and vague %q are different variants", i, full, vague)
		}
	}
}

func TestCraftHeardLine_StartAndFinishPools(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	if got := CraftHeardLine("cooking", PhaseStart, narration.FirstPicker); got != "You hear work begin." {
		t.Fatalf("start: %q", got)
	}
	for _, p := range []Phase{PhaseSuccess, PhaseFailure} {
		if got := CraftHeardLine("cooking", p, narration.FirstPicker); got != "The work stops." {
			t.Fatalf("phase %d: %q, want the finish pool for success and failure alike", p, got)
		}
	}
}

func TestCraftLines_UnknownSkillOrUnloadedStoreRendersNothing(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(nil))
	ctx := textutil.TokenContext{ActorName: "Kael"}
	if f, v := CraftRoomLines("cooking", PhaseStart, ctx, "Stew"); f != "" || v != "" {
		t.Fatalf("unloaded store rendered %q / %q", f, v)
	}
	if h := CraftHeardLine("cooking", PhaseStart); h != "" {
		t.Fatalf("unloaded store rendered %q", h)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/crafting/ -run 'CraftingMessages|CraftRoomLines|CraftHeardLine|CraftLines'`
Expected: FAIL to compile (`undefined: SkillPools`, `PhaseStart`).

- [ ] **Step 3: Add `PhaseStart`**

In `internal/crafting/narration.go`, replace the const block:

```go
const (
	PhaseSuccess Phase = iota
	PhaseFailure
	// PhaseStart is a craft beginning (messaging M6 row 7). It has no recipe
	// text: Narration returns empty variants for it, and the room's start line
	// comes from the crafting-messages store.
	PhaseStart
)
```

`PhaseStart` goes LAST so the existing values do not shift.

- [ ] **Step 4: Write the store**

Create `internal/crafting/messages.go`:

```go
package crafting

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// TokenItem names the thing being made in a crafting-messages line. It is
// this store's own event token, like movenarration's {item}: the store puts it
// in the token map it hands narration.Render, and the shared vocabulary never
// sees it.
const TokenItem = "{item}"

// unknownItem is what {item} reads when the reader cannot make the item out,
// or the recipe makes no item (an enchant), the word craftBeginLine used.
const unknownItem = "something"

// minCraftingVariants is the floor for every pool, casting's floor.
const minCraftingVariants = 3

// SkillPools is one skill's room lines (messaging M6 row 7, owner rulings
// R1 to R3). The *_observer pools are seen lines naming the crafter with
// {actor}; the *_heard pools are what a reader who cannot see the crafter
// hears, and name nobody. finish_heard serves success and failure alike so
// a sound never gives the outcome away.
type SkillPools struct {
	StartObserver   []string `yaml:"start_observer"`
	SuccessObserver []string `yaml:"success_observer"`
	FailureObserver []string `yaml:"failure_observer"`
	StartHeard      []string `yaml:"start_heard"`
	FinishHeard     []string `yaml:"finish_heard"`
}

// CraftingMessages is the store: recipe skill to that skill's pools.
type CraftingMessages map[string]SkillPools

var craftingMessages CraftingMessages

var craftTokenRE = regexp.MustCompile(`\{[a-z_]+\}`)

// craftPool is one pool with the rules its lines obey.
type craftPool struct {
	key     string
	lines   []string
	seen    bool            // validates as Observer and must name {actor}
	allowed map[string]bool // tokens a line may use
}

func (p SkillPools) pools() []craftPool {
	actorOnly := map[string]bool{narration.TokenActor: true, narration.TokenActorPlain: true}
	withItem := map[string]bool{narration.TokenActor: true, narration.TokenActorPlain: true, TokenItem: true}
	return []craftPool{
		{"start_observer", p.StartObserver, true, withItem},
		{"success_observer", p.SuccessObserver, true, withItem},
		{"failure_observer", p.FailureObserver, true, actorOnly},
		{"start_heard", p.StartHeard, false, nil},
		{"finish_heard", p.FinishHeard, false, nil},
	}
}

// Validate checks every pool of every block, and that the blocks match the
// recipe skills exactly: a recipe skill with no block would craft in
// silence, and a block no recipe uses is most likely a misspelling.
func (cm CraftingMessages) Validate(recipeSkills []string) error {
	want := map[string]bool{}
	for _, s := range recipeSkills {
		want[s] = true
		if _, ok := cm[s]; !ok {
			return fmt.Errorf("crafting-messages: no block for skill %q", s)
		}
	}
	names := make([]string, 0, len(cm))
	for s := range cm {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, skill := range names {
		if !want[skill] {
			return fmt.Errorf("crafting-messages: skill %q has no recipes (misspelled?)", skill)
		}
		for _, p := range cm[skill].pools() {
			v, role := narration.Variants{Actor: p.lines}, narration.RoleActor
			if p.seen {
				v, role = narration.Variants{Observer: p.lines}, narration.RoleObserver
			}
			if err := narration.ValidateVariants(v, minCraftingVariants, role); err != nil {
				return fmt.Errorf("crafting-messages %s %s: %w", skill, p.key, err)
			}
			for i, line := range p.lines {
				if p.seen && !strings.Contains(line, narration.TokenActor) {
					return fmt.Errorf("crafting-messages %s: %s must name the crafter with %s (variant %d)", skill, p.key, narration.TokenActor, i)
				}
				for _, tok := range craftTokenRE.FindAllString(line, -1) {
					if !p.allowed[tok] {
						return fmt.Errorf("crafting-messages %s: %s may not use %s (variant %d)", skill, p.key, tok, i)
					}
				}
			}
		}
	}
	return nil
}

// LoadCraftingMessages reads and validates the store against the loaded
// recipes. Called once at startup from main.go, after LoadRecipeFiles. It
// panics on a missing, unparseable or invalid file, as LoadCastingMessages
// does: no Go fallback text shadows shipped data.
func LoadCraftingMessages() {
	start := time.Now()

	path := string(configs.GetFilePathsConfig().DataFiles) + `/crafting-messages.yaml`
	data, err := os.ReadFile(path)
	if err != nil {
		panic(errors.Wrap(err, "reading "+path))
	}
	var cm CraftingMessages
	if err := yaml.Unmarshal(data, &cm); err != nil {
		panic(errors.Wrap(err, "parsing "+path))
	}
	if err := cm.Validate(RecipeSkills()); err != nil {
		panic(errors.Wrap(err, "validating "+path))
	}
	craftingMessages = cm
	mudlog.Info("crafting.LoadCraftingMessages()", "skills", len(cm), "Time Taken", time.Since(start))
}

// RecipeSkills lists the distinct skills of the loaded recipes, sorted.
func RecipeSkills() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range allRecipes {
		if !seen[r.Skill] {
			seen[r.Skill] = true
			out = append(out, r.Skill)
		}
	}
	sort.Strings(out)
	return out
}

// SeedCraftingMessagesForTest installs cm (nil unloads the store) and
// returns the restore func for t.Cleanup. It does not validate.
func SeedCraftingMessagesForTest(cm CraftingMessages) func() {
	prev := craftingMessages
	craftingMessages = cm
	return func() { craftingMessages = prev }
}

func pickerOf(picker []narration.Picker) narration.Picker {
	if len(picker) > 0 && picker[0] != nil {
		return picker[0]
	}
	return narration.DefaultPicker
}

// CraftRoomLines renders the seen line for one moment of a skill's craft,
// twice from ONE variant: full names the item (item, already formatted;
// "" for none), vague says "something" for a reader who makes out only
// shapes. Both are "" when the store has no pool for skill.
func CraftRoomLines(skill string, p Phase, ctx textutil.TokenContext, item string, picker ...narration.Picker) (full, vague string) {
	pools, ok := craftingMessages[skill]
	if !ok {
		return "", ""
	}
	var pool []string
	switch p {
	case PhaseStart:
		pool = pools.StartObserver
	case PhaseSuccess:
		pool = pools.SuccessObserver
	case PhaseFailure:
		pool = pools.FailureObserver
	}
	if len(pool) == 0 {
		return "", ""
	}
	index := pickerOf(picker)(len(pool))
	v := narration.Variants{Observer: pool}
	tokens := ctx.Tokens()
	if item == "" {
		item = unknownItem
	}
	// The index is already drawn; FirstPicker keeps Render from drawing a
	// second, unused random number before the override applies.
	tokens[TokenItem] = item
	full = narration.Render(v, tokens, narration.FirstPicker, index).Observer
	tokens[TokenItem] = unknownItem
	vague = narration.Render(v, tokens, narration.FirstPicker, index).Observer
	return full, vague
}

// CraftHeardLine renders what a reader who cannot see the crafter hears:
// the start pool at PhaseStart, the finish pool at success and at failure.
// "" when the store has no pool for skill.
func CraftHeardLine(skill string, p Phase, picker ...narration.Picker) string {
	pools, ok := craftingMessages[skill]
	if !ok {
		return ""
	}
	pool := pools.FinishHeard
	if p == PhaseStart {
		pool = pools.StartHeard
	}
	if len(pool) == 0 {
		return ""
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pickerOf(picker)).Actor
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/crafting/`
Expected: PASS. `gofmt -l internal/crafting/` prints nothing.

- [ ] **Step 6: Commit**

```bash
git add internal/crafting/narration.go internal/crafting/messages.go internal/crafting/messages_test.go
git commit -m "feat(crafting): crafting-messages store with per-skill room pools"
```

---

### Task 2: The copy, wiring and data guards

**Files:**
- Create: `_datafiles/world/dogmud/crafting-messages.yaml`
- Modify: `main.go` (after line 1642), `prose_dash_guard_test.go:32-34`, `shipped_narration_data_guard_test.go` (after the `casting` subtest at `:127-141`, and `narrationStoreWalkRoots` at `:279-294`), `internal/narration/snapshot_test.go` (`setupRealStores` at `:166`, a new builder, a new golden subtest near `:864`)
- Test: the shipped data guard subtest

- [ ] **Step 1: Add the failing shipped-data subtest**

In `shipped_narration_data_guard_test.go`, directly after the `t.Run("casting", ...)` block, add:

```go
	t.Run("crafting-messages", func(t *testing.T) {
		path := shippedWorldRoot + "/crafting-messages.yaml"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("crafting-messages: %v", err)
		}
		// yaml.v2, matching internal/crafting/messages.go.
		var cm crafting.CraftingMessages
		if err := yamlv2.Unmarshal(data, &cm); err != nil {
			t.Fatalf("crafting-messages: parse %s: %v", path, err)
		}
		skills := []string{"alchemy", "blacksmithing", "cooking", "enchanting", "jewelcrafting", "tailoring"}
		if err := cm.Validate(skills); err != nil {
			t.Errorf("crafting-messages: %v", err)
		}
	})
```

Add `"github.com/GoMudEngine/GoMud/internal/crafting"` to its imports if it is not already there. Add `shippedWorldRoot + "/crafting-messages.yaml",` to `narrationStoreWalkRoots` directly after the `casting-messages.yaml` entry. Add `"crafting-messages.yaml",` to `proseDashFiles` in `prose_dash_guard_test.go`.

Before trusting the hardcoded skill list, confirm it against disk: `ls _datafiles/world/dogmud/recipes/` must print exactly those six directories.

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'ShippedNarration|ProseDash'`
Expected: FAIL, `crafting-messages: open ... no such file`.

- [ ] **Step 3: Write the copy**

Create `_datafiles/world/dogmud/crafting-messages.yaml` with exactly this content:

```yaml
# Room lines for crafting (messaging M6 row 7, owner rulings R1 to R3,
# 2026-10-13). One block per recipe skill; every recipe of a skill uses its
# block. internal/crafting/messages.go loads and validates this file at boot.
#
#   start_observer, success_observer, failure_observer: what a reader who can
#     see the crafter reads. {actor} is the crafter (required). {item} is the
#     thing being made, or "something" to a reader who makes out only shapes
#     and for a recipe that makes no item. Failure lines may not use {item}.
#   start_heard, finish_heard: what a reader who cannot see the crafter hears.
#     No tokens: a sound names nobody. finish_heard serves success and
#     failure alike, so a sound never tells the outcome.
#
# Quote every line. No dashes.

alchemy:
  start_observer:
    - "{actor} sets out vials and begins working on {item}."
    - "{actor} measures reagents into a flask to make {item}."
    - "{actor} warms a flask over a small flame, starting on {item}."
  success_observer:
    - "{actor} stoppers the last vial and sets aside {item}."
    - "{actor} holds a flask up to the light, satisfied with {item}."
    - "{actor} pours off the last drops, finishing {item}."
  failure_observer:
    - "{actor}'s flask clouds over with a sour smell. The batch is ruined."
    - "{actor} winces as the mixture curdles, and pours it away."
    - "{actor} frowns at a murky flask and sets it aside, spoiled."
  start_heard:
    - "You hear glass clinking softly close by."
    - "You hear liquid poured carefully from one vessel to another."
    - "Something close by begins to bubble quietly."
  finish_heard:
    - "The bubbling close by dies away."
    - "You hear a stopper pushed into glass, then quiet."
    - "The clink of glass close by falls still."

blacksmithing:
  start_observer:
    - "{actor} heats metal in the forge and begins working on {item}."
    - "{actor} sets a bar of metal on the anvil to make {item}."
    - "{actor} works the bellows, starting on {item}."
  success_observer:
    - "{actor} quenches the metal with a hiss, finishing {item}."
    - "{actor} gives a last few taps and sets down {item}, finished."
    - "{actor} looks over {item} one last time, satisfied."
  failure_observer:
    - "{actor} curses as the metal cracks under the hammer. The work is ruined."
    - "{actor} pulls the metal from the fire too late, and it is spoiled."
    - "{actor} shakes their head at a warped piece of metal and tosses it aside."
  start_heard:
    - "You hear the ring of a hammer on metal close by."
    - "You hear the roar of bellows and the hiss of a forge."
    - "Close by, a hammer begins to strike metal in a steady rhythm."
  finish_heard:
    - "The hammering close by falls silent."
    - "You hear a sharp hiss of steam, then quiet."
    - "The ring of metal close by stops."

cooking:
  start_observer:
    - "{actor} sets a pot over the fire and begins cooking {item}."
    - "{actor} chops ingredients and starts making {item}."
    - "{actor} stirs a pot over the flames, cooking {item}."
  success_observer:
    - "{actor} ladles out a finished serving of {item}."
    - "{actor} takes the pot off the fire, finishing {item}."
    - "{actor} tastes the pot, nods, and sets {item} aside."
  failure_observer:
    - "{actor}'s pot boils over and the food burns. The meal is ruined."
    - "{actor} tastes the pot, grimaces, and dumps it out."
    - "{actor} scrapes a burnt mess from the pot and throws it away."
  start_heard:
    - "You hear a pot bubbling over a fire close by."
    - "You hear the steady sound of chopping close by."
    - "You hear something sizzling close by."
  finish_heard:
    - "The bubbling of the pot close by stops."
    - "You hear a pot lifted off the fire."
    - "The sizzling close by dies down."

enchanting:
  start_observer:
    - "{actor} traces a glowing pattern in the air over {item}."
    - "{actor} lays out runes and begins enchanting {item}."
    - "{actor} murmurs over {item} as a faint light gathers."
  success_observer:
    - "{actor} completes the pattern over {item}, and the glow settles."
    - "{actor} lowers their hands, and the glow sinks into {item}."
    - "A soft light flares over {actor}'s work on {item} and fades."
  failure_observer:
    - "{actor}'s pattern flickers and collapses. The enchantment fails."
    - "The light over {actor}'s work sputters out, and the materials crumble."
    - "{actor} flinches as the runes go dark and the work comes undone."
  start_heard:
    - "You hear a low, steady murmur close by."
    - "A faint hum rises somewhere close by."
    - "You hear soft chanting close by."
  finish_heard:
    - "The humming close by fades away."
    - "The murmuring close by stops."
    - "The faint hum close by falls silent."

jewelcrafting:
  start_observer:
    - "{actor} sets a gem in a small vise and begins working on {item}."
    - "{actor} bends fine wire with pliers, starting on {item}."
    - "{actor} picks up a jeweler's file and begins shaping {item}."
  success_observer:
    - "{actor} gives {item} a last polish with a soft cloth."
    - "{actor} sets the last stone in {item} and holds the work to the light."
    - "{actor} turns {item} in the light, satisfied with the work."
  failure_observer:
    - "{actor} winces as a stone cracks in its setting. The piece is ruined."
    - "{actor}'s file slips and gouges the metal. The work is spoiled."
    - "{actor} sighs and sweeps the ruined pieces aside."
  start_heard:
    - "You hear the fine scrape of a file close by."
    - "You hear tiny taps of metal on metal close by."
    - "You hear the faint grinding of a polishing wheel close by."
  finish_heard:
    - "The fine scraping close by stops."
    - "The tiny tapping close by falls quiet."
    - "The grinding close by slows and stops."

tailoring:
  start_observer:
    - "{actor} spreads out cloth and begins cutting a pattern for {item}."
    - "{actor} threads a needle and starts sewing {item}."
    - "{actor} measures out a length of material for {item}."
  success_observer:
    - "{actor} ties off the last stitch in {item}."
    - "{actor} shakes out {item} and looks the work over, satisfied."
    - "{actor} bites through the thread and holds up {item}."
  failure_observer:
    - "{actor}'s seam tears apart in their hands. The work is ruined."
    - "{actor} frowns at a crooked seam and pulls the work apart."
    - "{actor} cuts the cloth wrong and tosses the scraps aside."
  start_heard:
    - "You hear the snip of shears close by."
    - "You hear cloth rustling and a needle drawn through it."
    - "You hear fabric torn into strips close by."
  finish_heard:
    - "The rustle of cloth close by goes quiet."
    - "You hear a final snip of thread, then quiet."
    - "The snipping close by stops."
```

- [ ] **Step 4: Wire the loader**

In `main.go`, directly after `crafting.LoadRecipeFiles()` (line 1642), add:

```go
	crafting.LoadCraftingMessages()
```

- [ ] **Step 5: Snapshot the store**

In `internal/narration/snapshot_test.go`:

1. In `setupRealStores`, directly after `crafting.LoadRecipeFiles()`, add `crafting.LoadCraftingMessages()`.
2. After `buildCraftingGolden`, add:

```go
// Store 11b: crafting-messages.yaml (internal/crafting, messaging M6 row 7).
// Every pool of every skill, each variant in order through SequencePicker,
// with a named crafter and item for the seen lines.
func buildCraftingMessagesGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# crafting-messages.yaml store snapshot (internal/crafting)\n")
	fmt.Fprintf(&b, "# dimensions: skill x phase x variant; seen lines full|vague, then heard\n\n")

	skills := crafting.RecipeSkills()
	if len(skills) == 0 {
		t.Fatal("no recipes loaded; setupRealStores must call crafting.LoadRecipeFiles()")
	}
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}
	phases := []struct {
		name string
		p    crafting.Phase
	}{{"start", crafting.PhaseStart}, {"success", crafting.PhaseSuccess}, {"failure", crafting.PhaseFailure}}
	for _, skill := range skills {
		for _, ph := range phases {
			pick := narration.SequencePicker()
			for i := 0; i < 3; i++ {
				full, vague := crafting.CraftRoomLines(skill, ph.p, ctx, "Iron Dagger", pick)
				fmt.Fprintf(&b, "craft|%s|%s|%d|full => %s\n", skill, ph.name, i, full)
				fmt.Fprintf(&b, "craft|%s|%s|%d|vague => %s\n", skill, ph.name, i, vague)
			}
		}
		for _, ph := range []struct {
			name string
			p    crafting.Phase
		}{{"start_heard", crafting.PhaseStart}, {"finish_heard", crafting.PhaseSuccess}} {
			pick := narration.SequencePicker()
			for i := 0; i < 3; i++ {
				fmt.Fprintf(&b, "craft|%s|%s|%d => %s\n", skill, ph.name, i, crafting.CraftHeardLine(skill, ph.p, pick))
			}
		}
	}
	return b.String()
}
```

3. Next to the `t.Run("casting_messages", ...)` subtest, add:

```go
	t.Run("crafting_messages", func(t *testing.T) {
		checkGolden(t, "crafting_messages.golden", buildCraftingMessagesGolden(t))
	})
```

(`SequencePicker` walks 0, 1, 2 and wraps; verified at `internal/narration/picker.go:28-37`.)

- [ ] **Step 6: Record the golden and run everything touched**

Run: `go test ./internal/narration/ -run Snapshot -update` then `go test ./internal/narration/ ./internal/crafting/ . && go build ./...`
Expected: PASS. Open `internal/narration/testdata/stores/crafting_messages.golden` and read it: 6 skills, every full line names "Iron Dagger", every vague line says "something", heard lines carry no name. `crafting.golden` must be UNCHANGED (`git diff --stat internal/narration/testdata/` lists only the new file).

- [ ] **Step 7: YAML shape check**

Run: `python -c "import yaml,sys; d=yaml.safe_load(open('_datafiles/world/dogmud/crafting-messages.yaml',encoding='utf-8')); assert all(isinstance(l,str) for s in d.values() for p in s.values() for l in p), 'non-string line'; print(len(d),'skills ok')"`
Expected: `6 skills ok`.

- [ ] **Step 8: Commit**

```bash
git add _datafiles/world/dogmud/crafting-messages.yaml main.go prose_dash_guard_test.go shipped_narration_data_guard_test.go internal/narration/snapshot_test.go internal/narration/testdata/stores/crafting_messages.golden
git commit -m "feat(crafting): ship crafting-messages.yaml and load it at boot"
```

---

### Task 3: The sender (`actions.NarrateCraftPhase`)

**Files:**
- Create: `internal/actions/craft_phase_lines.go`
- Test: `internal/actions/craft_phase_lines_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/craft_phase_lines_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 row 7: every craft's room lines go through one body.
// phaseScene stands Kael (7351, the crafter), Mira (7352) and Tob (7353).

const (
	craftItemId  = 8301
	craftSleepId = 7363
)

func craftScene(t *testing.T, biome string, infrared bool) *rooms.Room {
	t.Helper()
	room := phaseScene(t, biome, infrared)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		craftItemId: {ItemId: craftItemId, Name: "iron dagger"},
	}))
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"blacksmithing": {
		StartObserver:   []string{"{actor} begins forging {item}."},
		SuccessObserver: []string{"{actor} finishes {item}."},
		FailureObserver: []string{"{actor} ruins the work."},
		StartHeard:      []string{"You hear hammering close by."},
		FinishHeard:     []string{"The hammering stops."},
	}}))
	return room
}

func craftRecipe() *crafting.RecipeSpec {
	return &crafting.RecipeSpec{RecipeId: "test-dagger", Name: "Test Dagger", Skill: "blacksmithing",
		Output: crafting.RecipeOutput{ItemId: craftItemId, Quantity: 1}}
}

func craftBy(u *users.UserRecord, room *rooms.Room, r *crafting.RecipeSpec, p crafting.Phase) {
	NarrateCraftPhase(NewUserActorInRoom(u, room), r, p, room)
}

func TestCraftPhase_FullSightReadsTheItemAndTheCrafterReadsNothing(t *testing.T) {
	room := craftScene(t, "city", false)
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseStart)

	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael begins forging Iron Dagger.")
	if got := phaseLines(phaseCasterId); len(got) != 0 {
		t.Fatalf("the crafter read a room line: %q", got)
	}
}

func TestCraftPhase_ShapesReadsAFigureAndSomething(t *testing.T) {
	room := craftScene(t, "cave", true)
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseSuccess)
	requireOneLine(t, "shapes bystander", phaseLines(phaseBystanderId), "A figure finishes something.")
}

func TestCraftPhase_NoSightHearsTheStartAndTheSameFinishEitherWay(t *testing.T) {
	room := craftScene(t, "cave", false)
	crafter := users.GetByUserId(phaseCasterId)

	craftBy(crafter, room, craftRecipe(), crafting.PhaseStart)
	requireOneLine(t, "dark bystander, start", phaseLines(phaseBystanderId), "You hear hammering close by.")
	craftBy(crafter, room, craftRecipe(), crafting.PhaseSuccess)
	requireOneLine(t, "dark bystander, success", phaseLines(phaseBystanderId), "The hammering stops.")
	craftBy(crafter, room, craftRecipe(), crafting.PhaseFailure)
	requireOneLine(t, "dark bystander, failure", phaseLines(phaseBystanderId), "The hammering stops.")
}

func TestCraftPhase_SleeperHearsTheCraft(t *testing.T) {
	room := craftScene(t, "city", false)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		craftSleepId: {ConditionId: craftSleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	if !users.GetByUserId(phaseBystanderId).Character.Conditions.AddCondition(craftSleepId, true) {
		t.Fatal("precondition: asleep")
	}
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseStart)
	requireOneLine(t, "sleeping bystander", phaseLines(phaseBystanderId), "You hear hammering close by.")
	requireOneLine(t, "awake bystander", phaseLines(phaseTargetId), "Kael begins forging Iron Dagger.")
}

// R3 as for spells: a reader who does not perceive a hidden crafter hears the
// craft and never reads a name, even in a lit room.
func TestCraftPhase_HiddenCrafterIsHeardNotSeen(t *testing.T) {
	room := craftScene(t, "city", false)
	crafter := users.GetByUserId(phaseCasterId)
	phaseHide(t, crafter.Character)

	craftBy(crafter, room, craftRecipe(), crafting.PhaseStart)
	for _, id := range []int{phaseTargetId, phaseBystanderId} {
		requireOneLine(t, "lit non-perceiver", phaseLines(id), "You hear hammering close by.")
	}
}

func TestCraftPhase_SeeHiddenReaderReadsAHiddenMobCrafter(t *testing.T) {
	room := craftScene(t, "city", false)
	m := phaseMob(t, room, true)
	if !users.GetByUserId(phaseTargetId).Character.Conditions.AddCondition(phaseSeeHiddenId, true) {
		t.Fatal("precondition: see hidden")
	}

	NarrateCraftPhase(NewMobActorInRoom(m, room), craftRecipe(), crafting.PhaseSuccess, room)

	got := phaseLines(phaseTargetId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "finishes Iron Dagger.") {
		t.Fatalf("a perceiving reader reads the named line, got %q", got)
	}
	requireOneLine(t, "non-perceiver", phaseLines(phaseBystanderId), "The hammering stops.")
}

func TestCraftPhase_RecipeWithNoOutputItemReadsSomething(t *testing.T) {
	room := craftScene(t, "city", false)
	r := craftRecipe()
	r.Output.ItemId, r.EnchantType = 0, "test-enchant"
	craftBy(users.GetByUserId(phaseCasterId), room, r, crafting.PhaseStart)
	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael begins forging something.")
}

func TestCraftPhase_SkillWithNoPoolsSendsNothing(t *testing.T) {
	room := craftScene(t, "city", false)
	r := craftRecipe()
	r.Skill = "tailoring"
	craftBy(users.GetByUserId(phaseCasterId), room, r, crafting.PhaseStart)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("no pools for the skill, yet the room read %q", got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/actions/ -run CraftPhase`
Expected: FAIL to compile, `undefined: NarrateCraftPhase`.

- [ ] **Step 3: Write the sender**

Create `internal/actions/craft_phase_lines.go`:

```go
package actions

import (
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// NarrateCraftPhase sends the room's lines for one moment of a craft, start,
// success or failure, for a player or a mob crafter (messaging M6 row 7,
// owner rulings R1 to R3). It sends ROOM lines only: the crafter's own lines
// stay with the sites, from the recipe file.
//
// Each player in the room but the crafter reads one line, by sight:
//   - full sight: the skill's seen line, crafter named, {item} the output's
//     name;
//   - shapes only: the same variant, crafter hidden, {item} "something";
//   - no sight (dark, blinded, asleep): the skill's heard line, the start
//     sound at start and the finish sound at success and failure alike;
//   - a reader who does not perceive a hidden crafter: the heard line,
//     whatever their sight (R3 and R5 of M6 slice 4, as for spells).
func NarrateCraftPhase(crafter Actor, recipe *crafting.RecipeSpec, phase crafting.Phase, room *rooms.Room) {
	if crafter == nil || recipe == nil || room == nil {
		return
	}
	cc := crafter.GetCharacter()
	if cc == nil {
		return
	}
	actorName, actorPlain := cc.GetCharacterName(true), cc.GetCharacterName(false)
	cat := messaging.CategoryEmote
	crafterUserId := 0
	switch a := crafter.(type) {
	case *UserActor:
		if a.User == nil {
			return
		}
		crafterUserId = a.User.UserId
	case *MobActor:
		if a.Mob == nil {
			return
		}
		actorName = MobSeenName(a.Mob, room, 0)
		cat = messaging.CategoryMobIdle
	}

	item := ""
	if name := items.SpecDisplayName(recipe.Output.ItemId); name != "" {
		item = fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, name)
	}
	full, vague := crafting.CraftRoomLines(recipe.Skill, phase,
		textutil.TokenContext{ActorName: actorName, ActorPlainName: actorPlain}, item)
	heard := crafting.CraftHeardLine(recipe.Skill, phase)

	unseenBy := UnseenBy(room, cc)
	// notFull and notShapes are the exclusion lists that narrow each visual
	// send to its own band: everyone outside the band, the crafter and the
	// non-perceivers included.
	var notFull, notShapes []int
	for _, uid := range room.GetPlayers() {
		u := users.GetByUserId(uid)
		sight := messaging.SightNone
		if u != nil && uid != crafterUserId && !slices.Contains(unseenBy, uid) {
			sight = messaging.ReaderSight(u.Character, room)
		}
		if sight != messaging.SightFull {
			notFull = append(notFull, uid)
		}
		if sight != messaging.SightShapes {
			notShapes = append(notShapes, uid)
		}
	}
	names := []string{actorName}
	if full != "" {
		room.SendTextVisualHidingNames(cat, full, names, notFull...)
	}
	if vague != "" {
		room.SendTextVisualHidingNames(cat, vague, names, notShapes...)
	}

	if heard == "" {
		return
	}
	// The sound channel reaches every reader who sees nothing; the
	// non-perceivers are sent their line one by one below, so the channel
	// skips them to send each exactly one.
	room.SendTextUnsighted(cat, heard, append(slices.Clone(unseenBy), crafterUserId)...)
	for _, uid := range unseenBy {
		if uid == crafterUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil {
			u.SendText(cat, heard)
		}
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/actions/ -run CraftPhase -v`
Expected: PASS, all eight. If `TestCraftPhase_SleeperHearsTheCraft` fails because the audio render drops sleepers, STOP and report: the spec's correction ("a sleeper hears the hammer") would be wrong and needs an owner call, not a workaround.

- [ ] **Step 5: Prove two tests can fail**

Temporarily change `if sight != messaging.SightShapes {` to `if sight != messaging.SightFull {` and run `go test ./internal/actions/ -run 'CraftPhase_Shapes'`: it must FAIL. Revert with Edit. Temporarily delete the `for _, uid := range unseenBy` loop and run `-run CraftPhase_Hidden`: it must FAIL. Revert with Edit. Run the full `-run CraftPhase` again: PASS.

- [ ] **Step 6: Package and root tests, commit**

Run: `go test ./internal/actions/ . && gofmt -l internal/actions/`
Expected: PASS, nothing listed.

```bash
git add internal/actions/craft_phase_lines.go internal/actions/craft_phase_lines_test.go
git commit -m "feat(actions): NarrateCraftPhase, one sender for craft room lines"
```

---

### Task 4: Player sites onto the sender

**Files:**
- Modify: `internal/usercommands/craft.go` (`craftDeliverInstant` `:46-67`, ImmediateComplete `:198-202`, Initiated `:205-208`, `completeCraft` `:763-773`)
- Modify: `internal/hooks/NewRound_UserRoundTick.go:668-675`, `:705-712`
- Test: `internal/usercommands/craft_instant_room_hiding_test.go`

- [ ] **Step 1: Read the existing hiding test file in full**

Read `internal/usercommands/craft_instant_room_hiding_test.go`. Its three tests at `:138`, `:204`, `:237` assert that a shapes-only third party reads "A figure" for a recipe's authored `success_observer`. After this task that slot is unread; the room line comes from the store.

- [ ] **Step 2: Rewrite those tests against the store**

For `TestCompleteCraft_RealBranch_ShapesOnlyThirdPartyReadsAFigure` and `TestCraftImmediateComplete_RealBranch_ShapesOnlyThirdPartyReadsAFigure`: keep each test's scene setup; add, after the existing seeding,

```go
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"<the recipe's skill>": {
		StartObserver:   []string{"{actor} begins {item}."},
		SuccessObserver: []string{"{actor} finishes {item}."},
		FailureObserver: []string{"{actor} ruins it."},
		StartHeard:      []string{"You hear work close by."},
		FinishHeard:     []string{"The work stops."},
	}}))
```

using the skill `craftInstantHidingRecipe` sets (read it at `:178`); remove the recipe's `SuccessRoomMessage` from `craftInstantHidingRecipe`; and change each test's expectation to: the shapes-only third party reads exactly two lines, `A figure begins something.` then `A figure finishes something.`, and no line contains the crafter's name. Keep the existing crafter-name absence assertions (`craftCountContaining`).

Delete `TestCraftInstantRoomLine_SendTrioHidesABareNameSendTextVisualDoesNot` (`:138`) and its header comment: it pins the `craftDeliverInstant` observer path this task removes, and the sender tests in Task 3 own name hiding now. Remove any helper only it used (check with Grep before deleting).

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/usercommands/ -run 'RealBranch'`
Expected: FAIL (the room reads nothing: the sites still read the empty recipe slot).

- [ ] **Step 4: Change `craftDeliverInstant` to the crafter's line only**

Replace `craftDeliverInstant` and its comment (`craft.go:46-67`) with:

```go
// craftDeliverInstant sends an instant-complete craft's lines: the crafter's
// own success line here, and the room's start and success lines through
// actions.NarrateCraftPhase (messaging M6 row 7). An instant craft tells the
// room both moments a timed craft does. Shared by both instant-complete
// sites: case result.ImmediateComplete in Craft(), and completeCraft (the
// enchanting instant path).
func craftDeliverInstant(user *users.UserRecord, room *rooms.Room, recipe *crafting.RecipeSpec, roles narration.Roles) {
	craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(`<ansi fg="green">%s</ansi>`, roles.Actor))
	actor := actions.NewUserActorInRoom(user, room)
	actions.NarrateCraftPhase(actor, recipe, crafting.PhaseStart, room)
	actions.NarrateCraftPhase(actor, recipe, crafting.PhaseSuccess, room)
}
```

Update both callers: `craftDeliverInstant(user, room, result.Recipe, roles)` at the ImmediateComplete case and `craftDeliverInstant(user, room, recipe, roles)` in `completeCraft`.

- [ ] **Step 5: Announce the timed start**

In the `case result.Initiated:` block, directly after the existing `craftDeliver(... "You begin crafting %s... (%s)" ...)` call, add:

```go
		actions.NarrateCraftPhase(actions.NewUserActorInRoom(user, room), result.Recipe, crafting.PhaseStart, room)
```

Confirm `result.Recipe` is set on the Initiated result: read the result type in `internal/actions/craft.go` (Grep `Initiated` there). If it is nil on that path, resolve the recipe the way the ImmediateComplete case does and say so in the commit message.

- [ ] **Step 6: The tick sites**

In `internal/hooks/NewRound_UserRoundTick.go`, replace

```go
									if successRoles.Observer != "" {
										sendVisualRoomText(room, messaging.CategoryEmote, successRoles.Observer, user.UserId)
									}
```

with

```go
									actions.NarrateCraftPhase(actions.NewUserActorInRoom(user, room), recipe, crafting.PhaseSuccess, room)
```

and replace

```go
									if failureRoles.Observer != "" {
										sendVisualRoomText(room, messaging.CategoryEmote, failureRoles.Observer, user.UserId)
									}
```

with

```go
									actions.NarrateCraftPhase(actions.NewUserActorInRoom(user, room), recipe, crafting.PhaseFailure, room)
```

The `TokenContext` passed to `recipe.Narrate` stays as is (the crafter's own line still uses it).

- [ ] **Step 7: Run and fix the registry**

Run: `go build ./... && go test ./internal/usercommands/ ./internal/hooks/ .`
Expected: the package tests PASS. `TestNarrationSitesMatchViewpointAudit` may now report the two rows `hooks/NewRound_UserRoundTick.go|<ansi fg="green">%s</ansi>` and `...red...` (`messaging_surface_guard_test.go:1282-1283`) as STALE, because those sites no longer send a room line in the shape the walk tracks. If it does, delete the two rows and leave a comment in their place in the style of the craft.go note at `:1341-1351`:

```go
	// hooks/NewRound_UserRoundTick.go's multi-round craft success and failure
	// lines (`<ansi fg="green">%s</ansi>` / red) were here (M3 item 6,
	// verdictCorrect). Messaging M6 row 7 (2026-10-13) moved their room half
	// to actions.NarrateCraftPhase, which this walk does not track, so the
	// sites left the walk. Removed rather than re-keyed: no gap was fixed.
```

If it reports something else (an unregistered row), read the reported site against source and register it with an accurate Reason. Re-run `go test .`: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/usercommands/craft.go internal/usercommands/craft_instant_room_hiding_test.go internal/hooks/NewRound_UserRoundTick.go messaging_surface_guard_test.go
git commit -m "feat(craft): player crafts narrate start, success and failure to the room"
```

---

### Task 5: Mob sites onto the sender; delete the mob-only lines

**Files:**
- Modify: `internal/mobcommands/craft.go` (`:55-59`, `:68-70`; delete `:76-114`)
- Modify: `internal/hooks/NewRound_MobRoundTick.go:613-634`
- Delete: `internal/crafting/mob_room_line.go`
- Modify: `internal/crafting/narration_test.go` (delete `TestMobRoomLine_AuthoredLineElseFallback`, `:98-113`)
- Modify: `internal/mobcommands/craft_begin_line_test.go`

- [ ] **Step 1: Rewrite the mob test first**

Replace the whole of `internal/mobcommands/craft_begin_line_test.go` with a test of the real `craft` command path. Keep `TestAnnounceCraftBeginJudgesEachReader`'s table and scene setup (cave biome, lamp for lit, infrared condition 9001 for shapes) but:

- rename it `TestMobCraftStartJudgesEachReader`;
- seed the store and the recipe:

```go
			t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"blacksmithing": {
				StartObserver:   []string{"{actor} begins working on {item}."},
				SuccessObserver: []string{"{actor} finishes {item}."},
				FailureObserver: []string{"{actor} ruins the work."},
				StartHeard:      []string{"You hear hammering close by."},
				FinishHeard:     []string{"The hammering stops."},
			}}))
```

- call `actions.NarrateCraftPhase(actions.NewMobActorInRoom(mob, room), &crafting.RecipeSpec{Skill: "blacksmithing", Output: crafting.RecipeOutput{ItemId: 8301}}, crafting.PhaseStart, room)` instead of `announceCraftBegin`;
- change the "no sight" row's want from `nil` to `[]string{"You hear hammering close by."}` (R3).

The full and shapes rows keep their wants (`Skeleton begins working on Iron Dagger.`, `A figure begins working on something.`). Delete `TestCraftBeginLineNamesTheItem` (its subject is deleted). Add the `actions` import. If `mobSpeechHeard` strips or filters in a way that hides an audio line, read it (`speech_sight_test.go:26`) and use the drain helper that returns every queued line instead.

- [ ] **Step 2: Run it**

Run: `go test ./internal/mobcommands/ -run MobCraftStart`
Expected: PASS already, because it calls the Task 3 sender directly. That is intended: it pins the reader rules for a mob crafter. The failing-first proof that the mob SITES are wired is Step 5.

- [ ] **Step 3: Move the mob sites**

In `internal/mobcommands/craft.go`:

ImmediateComplete, replace

```go
		room.SendTextVisual(messaging.CategoryMobIdle, result.Recipe.MobRoomLine(
			crafting.PhaseSuccess, mob.Character.Name, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> works quickly and produces something.`,
				mob.Character.Name)))
```

with

```go
		actor := actions.NewMobActorInRoom(mob, room)
		actions.NarrateCraftPhase(actor, result.Recipe, crafting.PhaseStart, room)
		actions.NarrateCraftPhase(actor, result.Recipe, crafting.PhaseSuccess, room)
```

Initiated, replace `announceCraftBegin(room, mob.Character.Name, result.Recipe)` with

```go
		actions.NarrateCraftPhase(actions.NewMobActorInRoom(mob, room), result.Recipe, crafting.PhaseStart, room)
```

Delete `announceCraftBegin` and `craftBeginLine` with their comments. Remove imports the file no longer uses (`go build` will name them).

In `internal/hooks/NewRound_MobRoundTick.go`, replace the success block

```go
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			sendVisualRoomText(room, messaging.CategoryMobIdle, recipe.MobRoomLine(
				crafting.PhaseSuccess, mob.Character.Name, fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi> finishes their work.`,
					mob.Character.Name)))
		}
```

with

```go
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			actions.NarrateCraftPhase(actions.NewMobActorInRoom(mob, room), recipe, crafting.PhaseSuccess, room)
		}
```

and the failure block

```go
		// A failed mob craft is silent unless the recipe authors a failure room line.
		if line := recipe.MobRoomLine(crafting.PhaseFailure, mob.Character.Name, ""); line != "" {
			if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
				sendVisualRoomText(room, messaging.CategoryMobIdle, line)
			}
		}
```

with

```go
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			actions.NarrateCraftPhase(actions.NewMobActorInRoom(mob, room), recipe, crafting.PhaseFailure, room)
		}
```

Add the `actions` import to `NewRound_MobRoundTick.go` (package `hooks` already imports it elsewhere, so no cycle). Remove unused imports.

- [ ] **Step 4: Delete `MobRoomLine`**

Delete `internal/crafting/mob_room_line.go` (`git rm`). Delete `TestMobRoomLine_AuthoredLineElseFallback` from `internal/crafting/narration_test.go`. Grep the repo (excluding `.claude/worktrees`) for `MobRoomLine`, `announceCraftBegin`, `craftBeginLine`, `finishes their work`, `produces something`: only docs under `docs/superpowers/specs/completed` or `plans/completed` may still mention them. Fix any live code, test or `context.md` hit.

- [ ] **Step 5: Prove a mob site is wired**

Temporarily comment out the `actions.NarrateCraftPhase(... PhaseStart ...)` line in the mobcommands Initiated case and run the mobcommands craft tests that drive the real `craft` command (Grep `mobcommands` tests for `Craft(`). If no test drives the real command, add one to `craft_begin_line_test.go`: seed a recipe the test mob can start (read `mobcommands_test.go` `seedAllRegistries` and `getTestMobAndRoom` for what is available), run `Craft("<recipe id>", mob, room, 0)`, and assert a full-sight reader reads the start line. Confirm it FAILS with the line commented out, restore it with Edit, confirm PASS.

- [ ] **Step 6: Run, commit**

Run: `go build ./... && go test ./internal/mobcommands/ ./internal/hooks/ ./internal/crafting/ . && gofmt -l internal/`
Expected: PASS, nothing listed. Fix any stale viewpoint-registry row exactly as in Task 4 Step 7.

```bash
git rm internal/crafting/mob_room_line.go
git add internal/mobcommands/craft.go internal/mobcommands/craft_begin_line_test.go internal/hooks/NewRound_MobRoundTick.go internal/crafting/narration_test.go messaging_surface_guard_test.go
git commit -m "feat(craft): mob crafts use the shared sender; retire the mob-only lines"
```

(Leave `messaging_surface_guard_test.go` out of `git add` if Step 6 did not change it.)

---

### Task 6: The one-sender root guard

**Files:**
- Create: `craft_phase_narration_guard_test.go` (repo root)

- [ ] **Step 1: Write the guard**

```go
package main

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// craftRoomRenderRE matches a call to the crafting-messages room renderers.
var craftRoomRenderRE = regexp.MustCompile(`crafting\.Craft(RoomLines|HeardLine)\(`)

// craftObserverReadRE matches reading the Observer of a narrated craft; the
// recipe's *_observer slot is unread since messaging M6 row 7.
var craftObserverReadRE = regexp.MustCompile(`(?i)roles\.Observer\b`)

// TestCraftRoomLinesLiveInOneBody keeps every craft's room lines in
// actions.NarrateCraftPhase (messaging M6 row 7). Eight sites once sent them
// under three rules; a new site must call the body.
func TestCraftRoomLinesLiveInOneBody(t *testing.T) {
	const home = "internal/actions/craft_phase_lines.go"
	inside := 0
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") ||
				strings.HasPrefix(rel, "internal/crafting/") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			usesCraftPhase := bytes.Contains(src, []byte("crafting.Phase"))
			sc := bufio.NewScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if craftRoomRenderRE.MatchString(line) {
					if rel == home {
						inside++
						continue
					}
					t.Errorf("%s:%d renders a craft room line itself: %s\n  call actions.NarrateCraftPhase instead",
						rel, n, strings.TrimSpace(line))
				}
				if usesCraftPhase && rel != home && craftObserverReadRE.MatchString(line) {
					t.Errorf("%s:%d reads a craft's Observer: %s\n  the room line comes from actions.NarrateCraftPhase",
						rel, n, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	// Proves the walk can find a match at all: the body renders both kinds.
	if inside < 2 {
		t.Fatalf("found %d renderer calls in %s, want at least 2; the walk or the regex is broken", inside, home)
	}
}
```

Check: `messagingSurfaceGoRoots` is the `[]string{"internal", "modules"}` var at `messaging_surface_guard_test.go:427`, already in package `main`.

- [ ] **Step 2: Run it, then prove it can fail**

Run: `go test . -run CraftRoomLinesLiveInOneBody`
Expected: PASS. Then temporarily add the line `_ = successRoles.Observer` in `NewRound_UserRoundTick.go` directly after the `successRoles := recipe.Narrate(...)` statement and re-run: FAIL naming that line. Remove it with Edit; PASS.

- [ ] **Step 3: Commit**

```bash
git add craft_phase_narration_guard_test.go
git commit -m "test: root guard keeps craft room lines in NarrateCraftPhase"
```

---

### Task 7: Docs, spec correction, full gate

**Files:**
- Modify: `internal/crafting/context.md`, `internal/actions/context.md`
- Modify: `docs/superpowers/specs/2026-10-13-messaging-m6-row7-craft-room-lines-design.md`
- Modify: `docs/superpowers/audits/messaging-m6-content-ledger.md` (row 7)
- Modify: `docs/README.md` (plan entry)

- [ ] **Step 1: `internal/crafting/context.md`**

Add a section "Room lines (crafting-messages.yaml)" naming, verified with `codegraph_search` or Grep: `SkillPools`, `CraftingMessages`, `TokenItem`, `LoadCraftingMessages`, `RecipeSkills`, `CraftRoomLines`, `CraftHeardLine`, `SeedCraftingMessagesForTest`, `PhaseStart`; say the per-recipe `success_observer` / `failure_observer` are unread since M6 row 7 and the room line comes from `actions.NarrateCraftPhase`. Remove every mention of `MobRoomLine`. Run `python tools/context_md_audit.py` and fix what it reports for these two packages.

- [ ] **Step 2: `internal/actions/context.md`**

Add `NarrateCraftPhase` beside `NarrateSpellPhase`, with the four-band reader rule from its doc comment and the root guard's file name.

- [ ] **Step 3: Spec corrections**

In the spec, by Edit: (a) replace the bullet starting "The `default` world gets its own copy" with: "No default-world copy: the default world has no `recipes/`, and `LoadRecipeFiles` already panics on the missing directory, so it does not boot today (found while planning)."; (b) in Testing, remove "each rendered line fits 80 columns with a long name" (the renderer wraps per reader; the store has no fixed width to check) and change "two boots (dogmud and default world)" to "two boots of the dogmud world".

- [ ] **Step 4: Ledger row 7**

Change row 7's status cell to: `done: per-skill crafting-messages.yaml + actions.NarrateCraftPhase (PR #<n>); per-recipe copy deferred to the crafting rework (owner R1, 2026-10-13)`. Fill `<n>` when the PR exists (Task 8).

- [ ] **Step 5: docs/README.md**

Add a row for this plan after the spec's row, in the same table style: path `superpowers/plans/2026-10-13-messaging-m6-row7-craft-room-lines.md`, one sentence: "Plan for the row 7 spec: the crafting-messages store and copy, the one sender, eight sites moved, the mob-only lines retired, a root guard, and a playtest."

- [ ] **Step 6: Full gate**

Run, each standalone:

```bash
go build ./...
go vet ./...
go test ./...
golangci-lint run --new-from-merge-base=origin/master ./...
```

Expected: all PASS / no new findings. Then boot the dogmud world once from the worktree on a free dev port per `dogmud-shipping` (detached-worktree boot check), confirm the log line `crafting.LoadCraftingMessages()` with `skills=6`, and stop that server BY ITS PID.

- [ ] **Step 7: Commit**

```bash
git add internal/crafting/context.md internal/actions/context.md docs/superpowers/specs/2026-10-13-messaging-m6-row7-craft-room-lines-design.md docs/superpowers/audits/messaging-m6-content-ledger.md docs/README.md
git commit -m "docs: M6 row 7 context, spec corrections, ledger"
```

---

### Task 8: Review, PR, playtest

- [ ] **Step 1: Whole-branch review** with `superpowers:requesting-code-review` against the spec. Fix findings in place.

- [ ] **Step 2: Push and open the PR** per `dogmud-shipping`: `gh pr create --repo pruuk/DOGMud --base master`. Body: what changed, the owner rulings R1 to R3, the two spec corrections, "Part of #370" (never "closes"), and the attribution line. Fill the PR number into ledger row 7 and push that commit.

- [ ] **Step 3: Playtest** with the `dogmud-playtesting` harness (ephemeral goals file, `--checkout` this branch). Scenes:
  1. Lit room, a station for one skill (e.g. a cooking fire): player A crafts a timed recipe and an instant one; watcher B reads start then success (or failure), item named.
  2. Same in a dim room where B reads only shapes: "A figure ... something".
  3. Same in a dark room: B reads `start_heard` then `finish_heard`, no name.
  4. A hidden A crafts in a lit room: B hears only.
  5. A mob made to run `craft` by admin command (no shipped mob is scripted to craft; `crafter: true` mobs restock without it): B reads the mob lines by the same rules.
  File any finding as a GitHub issue on `pruuk/DOGMud` (search first). Report the run ids.

- [ ] **Step 4: Merge** after the owner's go-ahead, per `dogmud-shipping`. The owner deploys; do not deploy.

---

### Task 9: Cancelled crafts tell the room (R4, amendment 2026-10-13)

Spec section "Cancelled crafts (R4, amendment 2026-10-13)" holds the verified facts. Same rules as every task above.

**Files:**
- Modify: `internal/state/activity/activity.go` (Machine field), `internal/state/activity/rules.go` (`TransitionToFree`, `ForceFree`), and add `OnCraftEnded`
- Test: `internal/state/activity/craft_ended_test.go` (new)
- Modify: `internal/crafting/narration.go` (`PhaseCancel`), `internal/crafting/messages.go` (pool, validation, render), `internal/crafting/messages_test.go`
- Modify: `_datafiles/world/dogmud/crafting-messages.yaml`, `internal/narration/snapshot_test.go` + golden
- Create: `internal/hooks/Activity_CraftEnded.go`, `internal/hooks/activity_craft_ended_test.go`
- Modify: `internal/state/activity/context.md`, `internal/crafting/context.md`, `internal/hooks/context.md` (if it lists cascade files), `docs/PATCH_NOTES.md`

- [ ] **Step 1: Engine callback, test first.** In `internal/state/activity/craft_ended_test.go`, test that a handler registered with `m.OnCraftEnded("t", fn)`:
  - fires exactly once with the `CraftingData` that was set (`RecipeId: "r1"`) and the reason's `Trigger` after `TransitionToCrafting` then `TransitionToFree(state.TransitionReason{Trigger: TriggerCraftCancel})`;
  - fires once after `TransitionToCrafting` then `ForceFree(...)`;
  - does not fire for `TransitionToCasting` then `TransitionToFree`, nor for salvaging;
  - does not fire when `TransitionToFree` is called while already Free (it returns an error or no-ops; read `rules.go` and assert whatever the real behaviour is, with no handler call).
  Read `internal/state/activity/*_test.go` first for how existing tests build data and reasons. Run: FAIL to compile.

- [ ] **Step 2: Implement.** In `activity.go` add to `Machine` a field `craftEnded []craftEndedEntry` with `type craftEndedEntry struct { name string; fn func(CraftingData, state.TransitionReason) }`, and:

```go
// OnCraftEnded registers a handler called after the machine leaves Crafting,
// with the data of the craft that ended and the reason it ended (messaging M6
// row 7, R4). It exists because an AfterTransition cascade runs once the
// state already reads Free, when CraftingData no longer answers.
func (m *Machine) OnCraftEnded(name string, fn func(CraftingData, state.TransitionReason)) {
	m.craftEnded = append(m.craftEnded, craftEndedEntry{name: name, fn: fn})
}

// fireCraftEnded calls the craft-ended handlers for a craft that just ended.
func (m *Machine) fireCraftEnded(ended *CraftingData, r state.TransitionReason) {
	if ended == nil {
		return
	}
	for _, e := range m.craftEnded {
		e.fn(*ended, r)
	}
}
```

  In `rules.go`, `TransitionToFree`: capture `ended := m.crafting` and `wasCrafting := m.State() == Crafting` before `m.inner.TransitionTo`; after the existing clearing, `if wasCrafting { m.fireCraftEnded(ended, r) }`. `ForceFree`: same capture after its early `Free` return, fire after clearing. If `Machine` uses a mutex around these fields, follow it. Run the Step 1 tests: PASS. `go test ./internal/state/...`: PASS.

- [ ] **Step 3: Store pool, test first.** In `messages_test.go`: extend `validPools()` with `CancelObserver: []string{"{actor} stops.", "{actor} sets it down.", "{actor} gives up for now."}`; add refusal cases "cancel line without actor" (expect `cancel_observer must name the crafter with {actor}`) and "item in a cancel line" (`cancel_observer may not use {item}`); add a render test: `CraftRoomLines("cooking", PhaseCancel, ctx, "Stew", narration.FirstPicker)` gives `"Kael stops."` for both full and vague, and `CraftHeardLine("cooking", PhaseCancel, narration.FirstPicker)` gives `"The work stops."` (the finish pool). Run: FAIL.

- [ ] **Step 4: Implement the pool.** `narration.go`: add `PhaseCancel` AFTER `PhaseStart` in the const block, with a one-line comment (a craft that ended without finishing, R4). `messages.go`: `SkillPools` gains `CancelObserver []string \`yaml:"cancel_observer"\``; `pools()` gains `{"cancel_observer", p.CancelObserver, true, actorOnly}`; `CraftRoomLines` switch gains `case PhaseCancel: pool = pools.CancelObserver`; `CraftHeardLine` already returns `FinishHeard` for any phase but `PhaseStart`; add a comment saying a cancel deliberately sounds like a finish (R3, R4). Update the SkillPools doc comment. Run: PASS.

- [ ] **Step 5: The copy.** Add to each skill block in `_datafiles/world/dogmud/crafting-messages.yaml`, after `failure_observer`, exactly:

```yaml
  # alchemy
  cancel_observer:
    - "{actor} stops working and pushes the vials aside, the batch unfinished."
    - "{actor} corks a half-made mixture and sets it down."
    - "{actor} pinches out the small flame and leaves the work for now."
  # blacksmithing
  cancel_observer:
    - "{actor} sets down the hammer, the work unfinished."
    - "{actor} pulls the metal from the forge and stops."
    - "{actor} leaves the work cooling on the anvil, unfinished."
  # cooking
  cancel_observer:
    - "{actor} pulls the pot off the fire, the meal unfinished."
    - "{actor} stops cooking and sets the pot aside."
    - "{actor} leaves the half-made meal where it sits."
  # enchanting
  cancel_observer:
    - "{actor} lets the glowing pattern fade, the work unfinished."
    - "{actor} breaks off the enchantment, and the gathered light dies away."
    - "{actor} stops murmuring, and the faint light fades."
  # jewelcrafting
  cancel_observer:
    - "{actor} sets down the file, the piece unfinished."
    - "{actor} loosens the vise and puts the work away for now."
    - "{actor} stops working and lays the tools aside."
  # tailoring
  cancel_observer:
    - "{actor} sets the needle down, the work unfinished."
    - "{actor} folds away the half-sewn cloth."
    - "{actor} stops sewing and puts the work aside."
```

  (The `# skill` comments mark which block each goes in; do not paste them.) Update the file's header comment to list `cancel_observer` and say a cancel is heard as `finish_heard`. Snapshot: in `buildCraftingMessagesGolden` add `{"cancel", crafting.PhaseCancel}` to the seen phases; re-record with `go test ./internal/narration/ -run TestSnapshotStores -update`; only `crafting_messages.golden` may change. Shape check with the Python one-liner from Task 2 Step 7. Run `go test ./internal/crafting/ ./internal/narration/ .`: PASS.

- [ ] **Step 6: The hook, test first.** Create `internal/hooks/activity_craft_ended_test.go`. Read how hooks tests build a lit room with two players and seed stores (grep `SeedUsersForTest`, `SeedBiomesForTest` in `internal/hooks/*_test.go`; reuse an existing scene helper if one fits). Seed `crafting.SeedCraftingMessagesForTest` with a blacksmithing block whose `CancelObserver` is `[]string{"{actor} stops forging."}` and `FinishHeard` `[]string{"The hammering stops."}` (other pools one line each), and register a recipe with `crafting.RegisterRecipeForTest` (skill blacksmithing; cleanup `UnregisterRecipeForTest`). For a crafter player whose `Character` has passed through `Validate()` or `characters.New()` (so `OnCharacterCreated` wired the hook; confirm by reading how the fixture builds characters), call `c.Activity.TransitionToCrafting(activity.CraftingData{RecipeId: <id>, RoundsTotal: 5}, reason)` then `c.Activity.TransitionToFree(state.TransitionReason{Trigger: <t>})` and assert the lit bystander's lines:
  - `activity.TriggerCraftCancel`, `TriggerMovementInterrupt`, `TriggerDamageInterrupt`: exactly one line, `"<Crafter> stops forging."` (plain text after stripping tags);
  - `activity.TriggerCraftComplete`, `activity.TriggerDeath`: no line.
  If wiring through `OnCharacterCreated` is impractical in the test fixture, export nothing new: call the hook's wiring function directly on the test character (it is in the same package) and say so in the report. Run: FAIL (no hook yet).

- [ ] **Step 7: Implement the hook.** Create `internal/hooks/Activity_CraftEnded.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// wireCraftEndedNarration tells the room when a craft ends without finishing
// (messaging M6 row 7, owner ruling R4): cancel, walking away, damage, or any
// later path that leaves Crafting. Completion narrates its own outcome at the
// round tick, and death has its own narration, so both are skipped. One hook
// rather than a call at each cancel site, so a new cancel path cannot be
// silent.
func wireCraftEndedNarration(c *characters.Character) {
	if c.Activity == nil {
		return
	}
	c.Activity.OnCraftEnded("craft_ended_room_line", func(d activity.CraftingData, r state.TransitionReason) {
		if r.Trigger == activity.TriggerCraftComplete || r.Trigger == activity.TriggerDeath {
			return
		}
		recipe := crafting.GetRecipe(d.RecipeId)
		room := rooms.LoadRoom(c.RoomId)
		if recipe == nil || room == nil {
			return
		}
		var crafter actions.Actor
		ref := c.ActorRef()
		switch {
		case ref.UserId > 0:
			if u := users.GetByUserId(ref.UserId); u != nil {
				crafter = actions.NewUserActorInRoom(u, room)
			}
		case ref.MobInstanceId > 0:
			if m := mobs.GetInstance(ref.MobInstanceId); m != nil {
				crafter = actions.NewMobActorInRoom(m, room)
			}
		}
		if crafter == nil {
			return
		}
		actions.NarrateCraftPhase(crafter, recipe, crafting.PhaseCancel, room)
	})
}

func init() {
	characters.OnCharacterCreated(wireCraftEndedNarration)
}
```

  Check: `crafting.GetRecipe` returns the recipe by id (`crafting.go:126`); if `CraftingData.RecipeId` for an enchant holds something else, read `usercommands/craft.go` `craftEnchanting` and handle it. Check the import graph builds (`hooks` already imports all of these). Run Step 6 tests: PASS. Then prove one bites: change the skip line to also skip `TriggerCraftCancel`, confirm the cancel case FAILS, revert by Edit.

- [ ] **Step 8: Docs and gate.** `internal/state/activity/context.md`: document `OnCraftEnded` and why it exists. `internal/crafting/context.md`: `PhaseCancel`, `cancel_observer`, cancel heard as `finish_heard`. `internal/hooks/context.md`: add `Activity_CraftEnded.go` beside `Activity_Cascades.go` if that file lists cascade hooks. `docs/PATCH_NOTES.md`: extend the crafting bullet with one sentence: "If someone stops partway, the room sees them set the work aside." Spec already carries R4. Full gate, each standalone: `gofmt -l internal/ modules/ *.go`, `go build ./...`, `go vet ./...`, `go test ./...`, `golangci-lint run --new-from-merge-base=origin/master ./...`. Commit per step or in two commits (engine + store; hook + copy + docs), named paths only.

- [ ] **Step 9: File the mob-movement issue.** Search first (`gh issue list --repo pruuk/DOGMud --search "mob craft move"`); if none, `gh issue create --repo pruuk/DOGMud` titled "A mob that moves mid-craft keeps crafting and finishes in its new room", body: mob movement never cancels mob crafting (players' `go` does, `usercommands/go.go:90-99`), so the new room hears a finish with no start; found in the M6 row 7 final review (PR #508); behaviour question, not a narration one.
