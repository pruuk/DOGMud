# Item Behaviour Slice 2: Item Voices (Lighting 5e) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A sentient item's voice moves from the Pinnacle tick and `internal/itemvoices` into its behaviour tree: a `speak` node with quiet, normal and chatty pacing and a per-listener ambient cap, the Blackrazor and the Aegis of Mockery rebuilt as trees with their lines unchanged, and the dead `on_equip` / `on_unequip` lines firing at last (fixes #222).

**Architecture:** An item tree's YAML gains `speech:` (pools of lines) and `chatter:` (a pacing level); the engine stores that voice beside the compiled tree. Four item-only nodes in `internal/behaviortree`: `chatter_ready` (ambient gate: cooldown, listener cap, chance), `speak` (send and arm the cooldown), `taunt_pull` (the Aegis's pull) and `hunger_overdue` (the Blackrazor's warning state). `internal/hooks` fires `on_equip` / `on_unequip` from an `EquipmentChange` listener, `on_kill` from `MobDeathItemProcs`, `on_hunger_feeding` from `tickHunger`; the item tick already fires `item_idle`. A 200-round record of today's voice path, taken under a new seeded `util.Rand`, proves the tree path reproduces it before the old path, `itemvoices`, `voice_id`, `taunt_pull` and the `SentientChatter*` knobs are deleted.

**Tech Stack:** Go 1.25, YAML world data, the existing behaviour-tree engine.

**Spec (binding):** `docs/superpowers/specs/2026-10-05-item-behaviour-foundation-design.md`, slice 2 (Rules 16 to 19, X14 to X17, X19 to X21, rulings R5, R6, R10), plus three owner rulings of 2026-10-06 recorded below as S1 to S3.

**Branch:** implementation branch `feature/item-behaviour-slice2`, cut from master AFTER the docs branch `docs/item-behaviour-slice2-plan` (this plan) merges, in the worktree `C:/tmp/dogmud-itembeh2`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/item-behaviour-slice2 C:/tmp/dogmud-itembeh2 origin/master
```

(The docs branch uses the same path; remove that worktree once the plan PR merges, then cut this one.) All paths below are relative to the worktree. Run Go commands from its root. Name `C:/tmp/dogmud-itembeh2` in the handoff memory while the branch is open; remove it with `git worktree remove C:/tmp/dogmud-itembeh2` once the PR merges. Throwaway output goes in the session scratchpad (`$TMP`), never `C:/tmp`. Edits use the Edit and Write tools only, never a Python read-modify-write. **An Edit whose old or new text ends in a space loses that space** (seen twice in the dry run, both times a gofmt alignment); anchor such edits on whole lines. A fresh worktree's `_datafiles/config.yaml` is the committed blob (no skip-worktree), so editing it on disk is safe there; check `git diff _datafiles/config.yaml` shows only the intended lines.

Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that. Subagent model: sonnet for Tasks 1 to 6 and 8 to 9, opus for Task 7 (the switch-over has the most moving parts).

**Dry run (2026-10-06).** Every code and content block below was applied, in this order, to a scratch worktree `C:/tmp/dogmud-itembeh2-dry` detached at `origin/master` `66777d507`, with a local checkpoint commit per task (never pushed). Each failing-first step was run and failed as its step says; the "shown to fail against today's sender" probe in Task 5 was run, seen red and restored. The dry run met three things this plan now carries into the task that causes them, rather than discovering them later: `condition_apply_path_guard_test.go` keys sites by line number, and Tasks 5, 6 and 7 each shift lines it names (the numbers below were read from the checkpoint commits); `items.LoadDataFiles` in a test also replaces the combat and defence message stores, which leaked into a later hooks test until the loader snapshot both (Task 2); and an Edit ending in a space drops it (above). At the end: `gofmt -l internal/ modules/ .` clean; `go vet ./...` clean; `go build ./...`; `go test ./... -count=1` with no `FAIL`; `golangci-lint run --new-from-merge-base=origin/master` `0 issues.` (one stale-cache warning naming another worktree's path, not an issue); `python tools/context_md_audit.py` identical before and after except `packages checked` 143 to 142 (`itemvoices` gone); every added line free of em and en dashes; a boot on private ports (telnet 33357, local 9957, http 8057, through a scratch `CONFIG_PATH` overrides file) stayed up to the 180 s timeout (exit 124) with 0 panics, one `Server Ready` and `mapper.ValidateZoneConsi errors=0 warnings=0 mode="panic"`. The Docker race run result is recorded under Task 9.

---

## Facts verified against source (2026-10-06, `origin/master` `66777d507`)

The spec's facts were read at `7e0748145`; slice 1 (#410) has merged since. Every row below was read at `66777d507`. **NEW** marks a fact the spec does not state, or states wrongly, that changes the plan.

| # | Fact | Where |
|---|---|---|
| F1 | `ItemSubject{UUID, ItemId, UserId, MobInstanceId, RoomId, Slot, OnFloor}`; `TryItemBehavior(event EventContext, subject ItemSubject) (handled bool)` resolves the template's tree by `spec.Behavior`, resolves the holder, runs under a recover, returns true on Success | `internal/behaviortree/item_engine.go:26-122` |
| F2 | `ValidateItemBehaviors()` loads every named tree through `Engine.LoadItemTree(name, path)`, which calls `LoadItemTreeFromFile(path) (Node, error)`; main.go panics on its error | `item_engine.go:129-178`; `engine.go:210-223`; `loader.go:75-93`; `main.go:1676` |
| F3 | The engine caches `itemTrees map[string]Node` and `noItemTree`; `LoadItemTreeForTest(t, name, yaml)` installs a compiled node directly | `engine.go:23-24,213-254`; `test_export.go:89-117` |
| F4 | `TreeDef{Notes, Tree, GoalWeights, DefaultGoals}`; `LoadTreeFromBytes` (mob and room trees) and `LoadArchetypeYAMLFromFile` parse a `TreeDef` | `types.go:109-114`; `loader.go:36-70` |
| F5 | The item allowlist is `itemSafeConditions` / `itemSafeActions`, and `itemOnlyNodes` refuses item nodes in mob and room trees | `loader.go:101-151` |
| F6 | `KnownBehaviorEvents` lists 16 events including `item_idle`; `events_test.go` requires every listed event to appear as an `EventType: "..."` literal in non-test Go under `internal` or `modules`, and every such literal to be listed | `events.go:16-33`; `events_test.go:17-63` |
| F7 | `ItemRoundTick` (a `NewRound` listener registered after `UserRoundTick` and `DoCombat`) fires `item_idle` for every treed worn and backpack item of every online player through `tickHeldItems(c, userId, mobInstanceId) bool`, then indexed mobs and rooms | `internal/hooks/NewRound_ItemRoundTick.go:25-84`; `hooks.go:53,59,70` |
| F8 | `condInCombat`, `condWorn`, `condHolderAsleep` and the helper `itemHolder(ctx)` exist | `conditions_item.go` |
| F9 | `pinnacleUserTick` calls `tickHunger(c, user, now)` then `tickVoices(user, room, worn, now)`; `tickVoices` holds one per-bearer cooldown in MiscData `pinnacle_voice_next_round`, picks the event with `pickVoiceEvent` (combat taunt, hunger past 3/4 by integer `HungerRounds*3/4`, else idle), checks the pool is non-empty, rolls `util.Rand(100) >= SentientChatterChancePct` once, emits through `emitVoiceLine`, and calls `applyTauntPull` after an Aegis taunt | `internal/hooks/pinnacle_tick.go:44-57,397-491` |
| F10 | **NEW.** Today's kill line is `tryEmitVoice(user, nil, wspec, "on_kill")` with `room == nil`: the BEARER ALONE hears it, and only the weapon slot is asked, so the Aegis's five authored kill lines never fire (ruled S2, S3) | `MobDeath_ItemProcs.go:26-38`; `itemvoices/aegis.yaml` |
| F11 | **NEW.** Today's feeding line is `emitVoiceLine(user, nil, spec, "on_hunger_feeding", fallback)`: bearer alone, paced only by MiscData `pinnacle_hunger_msg_next_round` armed with `SentientChatterCooldownRounds`, and it neither reads nor arms the voice cooldown. Spec Rule 17 would put it behind the item cooldown; ruled S1 to keep today's pacing | `pinnacle_tick.go:181-191` |
| F12 | `emitVoiceLine` sends the holder `<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"` and the room `<ansi fg="username">%s</ansi>'s ... mutters, ...` through `room.SendTextVisual` excluding the holder, both `messaging.CategorySystem` | `pinnacle_tick.go:513-535` |
| F13 | `Room.SendTextHidingNames(cat, txt, names, hide, exclude...)` sends to every player in the room on the audio channel with each name hidden at that listener's `ParticipantSight`; `messaging.HideSpeakerNames` replaces a whole identity tag (`username` / `mobname`) with "a figure" or "someone", capitalised at a sentence start | `internal/rooms/rooms.go:290-343`; `internal/messaging/hidenames.go:94`; `hidenames_tagged.go:10,49` |
| F14 | `narration.Render(Variants{Actor: lines}, nil, nil)` picks through `DefaultPicker`, which is `util.Rand(n)` | `internal/narration/render.go:92`; `picker.go:14-19` |
| F15 | `util.Rand` is `rand.Intn` on the global source; no seam exists. `util.SetRoundCountForTest` and `ResetRoundCountForTest` exist | `internal/util/util.go:148-157,204-209` |
| F16 | `hooks.readMiscRound(v any) (uint64, bool)` is the only MiscData round reader; behaviortree has none | `internal/hooks/item_procs.go:37-52` |
| F17 | `actions.equipItem` queues `EquipmentChange{UserId, MobInstanceId, ItemsWorn: [item], ItemsRemoved: displaced}`; `removeWorn` queues `ItemsRemoved: [item]`; both for players and mobs. Its listeners today: `onEquipmentChangeForAwareness` (a stub) and GMCP | `internal/actions/remove_equip.go:92-97,178-182`; `hooks/Awareness_LightChange.go:36`; `modules/gmcp/gmcp.Char.go:56` |
| F18 | `Worn.AllSlots()` returns `[]WornSlot{Key, Label, Item *items.Item}`, weapon first, offhand second | `internal/characters/worn.go:53-69` |
| F19 | Knobs: `SentientChatterCooldownRounds` and `SentientChatterChancePct` (Go default 20 / 15, shipped 20 / 15); `TauntHoldRounds` exists. **NEW:** a `< 0` coercion cannot repair an absent key (0), the trap noted at `KnockdownFrequencyScale`; the spec's "zero is legal" for the new chatter knobs would let a deleted line silence every item, so the new knobs coerce `<= 0` (cooldowns, cap) and `< 1 \|\| > 100` (chances) to their defaults, and `PinnacleItemsEnabled` stays the off switch | `config.balance.go:1110-1113`; `config.balance.misc.go:248-260,480-490`; blob `:2103-2104` |
| F20 | **NEW.** `items.LoadDataFiles` replaces the item specs AND the combat and defence message stores; `SeedItemsForTest` restores only the specs. `SeedAttackMessagesForTest` / `SeedDefenseMessagesForTest` restore the other two | `internal/items/itemspec.go:906-942`; `test_helpers_combat.go:18,32` |
| F21 | **NEW.** `configs.AddOverlayOverrides` (used by `setPinnacleEnabled`) rebuilds the live config, so a `SetConfigForTest` made before it is lost; set the overlay first | `internal/hooks/pinnacle_tick_test.go:22-29`; measured |
| F22 | Repo-root guards that name the voice surface: `item_behaviour_guard_test.go` (`VoiceId`, `TauntPull` classified, three read sites, the scan's sanity probe reads `VoiceId`); `narration_render_callers_guard_test.go` (registers `internal/itemvoices/itemvoices.go`); `messaging_surface_guard_test.go` (`lines`, `on_taunt`, `voice_id`, `voiceid`, `taunt_pull` registry rows; the viewpoint registry row `hooks/pinnacle_tick.go|<ansi fg="item">%s</ansi> says, ...`); `shipped_narration_data_guard_test.go` (an `itemvoices` subtest and walk root); `condition_apply_path_guard_test.go` (line-keyed `item_procs.go|215`, `|265`, `pinnacle_tick.go|335`, `|349`) | files named |
| F23 | Other references: `main.go:49,1670-1672` (load), `modules/gmcp/gmcp.Item.go:14,106-107,126,201,224,231,260` and `gmcp.Item_test.go` (builder), `_datafiles/html/public/static/js/items.js:6,427-429,457-459,563` (builder form), `internal/narration/snapshot_test.go` and `testdata/stores/itemvoices.golden`, `internal/configs/configs_pinnacle_test.go:11`, comments in `combat/taunt_messages.go:198`, `narration/render.go:12`, `narration/picker.go:48`, `modules/weather/content/arch_test.go:18`, `narration/context.md:97,118,239`, `behaviortree/context.md:1015`, `behaviortree/item_engine.go:127`, `docs/schemas/pinnacle-items.md` (sections 5, 6, 9, 10, Stage 2 tables) | grep |
| F24 | **NEW.** The builder's save starts from the loaded spec (`reqToSpec(base, req)`), so an item's `behavior:` survives a builder save without a form field | `modules/gmcp/gmcp.Item.go:233-262` |
| F25 | Content: 40183 carries `voice_id: blackrazor` (`:37`), `hunger_rounds: 50`, `hunger_drain_pct: 0.01`; 40185 carries `voice_id: aegis` (`:25`), `taunt_pull: true` (`:26`); `itemvoices/` holds `aegis.yaml`, `blackrazor.yaml`, `.gitkeep` | `items/materials-40000/`; `itemvoices/` |
| F26 | Help files for the two items describe recipes only, nothing about speech; no help change is needed | `templates/help/assemble-*.template` |
| F27 | `behaviortree/shipped_item_trees_test.go`'s `loadShippedItemWorld` (slice 1) calls `items.LoadDataFiles` with only `SeedItemsForTest` restoring: the F20 leak, a sibling fixed in Task 2 | `shipped_item_trees_test.go:21-35` |

## Owner rulings of 2026-10-06 (beyond the spec)

| # | Question | Ruling |
|---|---|---|
| S1 | The feeding line (F11): today's pacing, or Rule 17's item cooldown | **Today's.** Bearer only, paced only by `HungerFeedingLineCooldownRounds`, neither reading nor arming the item cooldown. The parity record stays exact |
| S2 | Kill lines (F10): bearer only, or the room too | **The room too** (Rule 16 as written) |
| S3 | Which worn items hear a kill | **Every worn voiced item**, so the Aegis's kill lines fire |

## Where the spec could not be implemented as written

1. **Pacing is two nodes, not one.** Rule 16 puts cooldown, chance and cap inside `speak`. A tree's selector falls through a failed `speak` to the next branch, so a failed taunt roll would fall through to an idle line and draw a second number, which today's tick never does. `chatter_ready` gates the ambient branch (cooldown, listeners and cap, then the chance, drawn last so a closed round draws nothing), and `speak` re-checks and arms the cooldown. Event branches speak without `chatter_ready`. The draw order matches today's (`chance` then `pick`), which the parity record proves.
2. **`speak` takes `to: holder` and `paced: false`.** Ruling S1 needs a holder-only, unpaced feeding line; Rule 16 has neither.
3. **`taunt_pull` always succeeds,** so a taunt branch with nothing to pull never falls through to an idle line.
4. **An item with nobody in its room does not speak** (`chatter_ready` needs at least one listener), so a mob-held item in an empty room draws nothing.
5. **The new chatter knobs coerce zero to the default** (F19). The spec's "zero is legal" is replaced by the house rule; `PinnacleItemsEnabled` is the off switch.
6. **The pools are proved byte-identical to the voice files, not to `itemvoices.golden`.** That golden pins only the first line of each pool. Task 7 compares every pool line for line with `itemvoices/*.yaml` while both exist (`TestItemTreePoolsMatchTheVoiceFiles`), and Task 8 deletes that test with `itemvoices`.
7. **The Aegis leaves the record at the kill.** Ruling S3 gives the shield a kill line the record (today's path) does not have, and the extra draw shifts what follows, so `TestItemVoiceParity` compares the Aegis up to the kill round and then asserts its kill line. The Blackrazor matches all 200 rounds.
8. **Two items worn together each keep their own cooldown.** Today one bearer cooldown covered both; per item, two sentient items can each speak, bounded by the listener cap (accepted in the spec's design A7).
9. **`hunger_overdue` compares as a float** (`elapsed > HungerRounds * fraction`); for whole rounds this equals today's integer `elapsed > HungerRounds*3/4` (50 gives 37.5 against 37: both fire at 38).
10. **One MiscData round reader.** `characters.MiscRound` takes `hooks.readMiscRound`'s body; the hooks function delegates, so its call sites are unchanged.
11. **The hunger fallback line loses its em dash** ("The blade feeds on you, a cold pull beneath your grip."). The line is rewritten in Task 7, so it falls under the no-dash rule; no shipped item reaches it any more (the Blackrazor's tree speaks).
12. **The viewpoint audit doc gets a retirement row** (Task 7), as `TestNarrationSitesMatchViewpointAudit` requires before its registry entry goes.
13. **`behaviors/items` joins the shipped narration walk** (Task 8), so the legacy role-key checks that covered `itemvoices/` still cover item speech.

## Player-visible lines that change (everything else stays byte-identical)

| Line | Before | After |
|---|---|---|
| Equip / remove the Blackrazor or the Aegis | Nothing (#222) | One of the authored `on_equip` / `on_unequip` lines, to the bearer and the room |
| A kill with the Aegis worn | Nothing from the shield | One of its five kill lines |
| A kill line | The bearer alone | The bearer, and the room hears it muttered (S2) |
| The room line of any item speech | Sight-gated, raw bearer name; nothing in the dark | Heard by everyone; "Someone's ..." / "A figure's ..." for a listener who cannot see the bearer |
| A mob holding a voiced item | Silent | The room line (R6) |
| The hunger fallback (no tree) | "The blade feeds on you — a cold pull beneath your grip." | "The blade feeds on you, a cold pull beneath your grip." |
| Two voiced items worn together | At most one line per 20 rounds between them | Each paced on its own; ambient lines bounded by the listener cap |

Idle, taunt, hunger-warning and feeding lines to the bearer are unchanged (the parity record).

## File map

| File | Task | Change |
|---|---|---|
| `internal/util/util.go`, `rand_seam_test.go` (new) | 1 | `SetRandForTest(seed)` |
| `internal/hooks/item_voice_parity_test.go` (new), `testdata/item_voice_parity.golden` (new) | 2, 7, 8 | The record; switched to the tree path; the pool proof added then removed |
| `internal/behaviortree/shipped_item_trees_test.go` | 2 | Store snapshot (F27) |
| `internal/configs/config.balance.go`, `config.balance.misc.go`, `configs_item_chatter_test.go` (new), `_datafiles/config.yaml`, `internal/hooks/pinnacle_tick.go` | 3 | Eight knobs; feeding pace reads the new one |
| `internal/behaviortree/types.go`, `item_voice.go` (new), `item_voice_test.go` (new), `loader.go`, `engine.go`, `test_export.go` | 4 | `speech:` / `chatter:`, the voice store |
| `internal/behaviortree/actions_item_voice.go` (new), `item_voice_nodes_test.go` (new), `conditions.go`, `actions.go`, `loader.go`, `item_voice.go`; `internal/characters/character.go`; `internal/hooks/item_procs.go`; `condition_apply_path_guard_test.go` | 5 | The four nodes, `MiscRound` |
| `internal/hooks/EquipmentChange_ItemEvents.go` (new), `item_voice_events_test.go` (new), `MobDeath_ItemProcs.go`, `pinnacle_tick.go`, `hooks.go`; `internal/behaviortree/events.go`; `condition_apply_path_guard_test.go` | 6 | The four events |
| `behaviors/items/blackrazor.yaml`, `aegis.yaml` (new); 40183, 40185; `pinnacle_tick.go`, `pinnacle_tick_test.go`, `MobDeath_ItemProcs.go`; four root guards; the viewpoint audit doc | 7 | The switch-over |
| `internal/itemvoices/` (deleted), `itemvoices/` data (deleted), `itemvoices.golden` (deleted); `main.go`; `itemspec.go`; gmcp builder and `items.js`; knobs; three root guards; narration snapshot; comments | 8 | Retire |
| `context.md` in `behaviortree`, `hooks`, `characters`, `util`, `narration`; `docs/schemas/pinnacle-items.md`, `behavior.md`; `docs/PATCH_NOTES.md` | 9 | Docs and gates |

---

### Task 1: A seeded `util.Rand` for tests (spec X19)

**Model:** sonnet.

**Files:**
- Create: `internal/util/rand_seam_test.go`
- Modify: `internal/util/util.go:204-209`

- [ ] **Step 1: Write the failing test**

Create `internal/util/rand_seam_test.go`:

```go
package util

import "testing"

// SetRandForTest replays one seeded sequence, so a golden can pin every
// random draw a run makes (item behaviour slice 2, spec X19).
func TestSetRandForTestReplaysASeededSequence(t *testing.T) {
	draw := func() []int {
		out := make([]int, 20)
		for i := range out {
			out[i] = Rand(100)
		}
		return out
	}

	restore := SetRandForTest(42)
	first := draw()
	restore()

	restore = SetRandForTest(42)
	second := draw()
	restore()

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("draw %d: %d then %d; the same seed must replay the same sequence", i, first[i], second[i])
		}
	}
	if Rand(0) != 0 {
		t.Fatal("Rand(0) must stay 0 whatever the source")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/util/ -run TestSetRandForTest -count=1`
Expected: build failure, `undefined: SetRandForTest`.

- [ ] **Step 3: Implement**

In `internal/util/util.go`, replace

```go
func Rand(maxInt int) int {
	if maxInt < 1 {
		return 0
	}
	return rand.Intn(maxInt)
}
```

with

```go
// randSource is where Rand draws: the math/rand global, unless a test has
// swapped in a seeded source with SetRandForTest. Atomic so a -race run
// sees no data race on the swap.
var randSource atomic.Pointer[func(int) int]

func Rand(maxInt int) int {
	if maxInt < 1 {
		return 0
	}
	if src := randSource.Load(); src != nil {
		return (*src)(maxInt)
	}
	return rand.Intn(maxInt)
}

// SetRandForTest makes Rand draw from a source seeded with seed, so a test
// can replay every random draw a run makes (item behaviour slice 2, spec
// X19). Returns a restore func. The seeded source is not safe for
// concurrent draws: use it only in a test that draws from one goroutine.
func SetRandForTest(seed int64) func() {
	r := rand.New(rand.NewSource(seed))
	draw := func(n int) int { return r.Intn(n) }
	prev := randSource.Swap(&draw)
	return func() { randSource.Store(prev) }
}
```

(`sync/atomic` and `math/rand` are already imported.)

- [ ] **Step 4: Run the package**

Run: `go test ./internal/util/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/util/util.go internal/util/rand_seam_test.go
git commit -m "feat(util): seeded Rand for tests (item behaviour slice 2)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Record today's voice path (the parity record)

**Model:** sonnet.

**Files:**
- Create: `internal/hooks/item_voice_parity_test.go`, `internal/hooks/testdata/item_voice_parity.golden`
- Modify: `internal/behaviortree/shipped_item_trees_test.go:28` (sibling fix, F27)

- [ ] **Step 1: Write the harness**

Create `internal/hooks/item_voice_parity_test.go`:

```go
package hooks

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/itemvoices"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The voice parity record (item behaviour slice 2, spec "Slice 2" gates).
// testdata/item_voice_parity.golden was recorded on the Pinnacle tick's
// voice path (tickVoices, tryEmitVoice, emitVoiceLine) BEFORE the voices
// moved into item trees, and the tree path must reproduce it: the same
// lines on the same rounds to the bearer, under one seeded random source
// (util.SetRandForTest) that both the chance roll and the line pick draw
// from. Two scenarios, one per shipped voiced item, each 200 rounds:
// idle, a hunger warning (the Blackrazor only), combat taunts, hungry
// feeding, a kill, idle again.
//
// Only the bearer's own lines are recorded: the room line changed on
// purpose (spec X17, it is heard and hides the bearer's name).
var updateVoiceParity = flag.Bool("update-voices", false, "rewrite testdata/item_voice_parity.golden")

const (
	voiceParityUserId   = 1
	voiceParityRoomId   = 9971
	voiceParityFirst    = 1000 // the first scenario round
	voiceParityRounds   = 200
	voiceParityFightAt  = 60  // combat from this round offset
	voiceParityFightEnd = 110 // to this one
	voiceParityKillAt   = 165 // the kill: both items' chatter cooldowns are open here under the seed
	voiceParitySeed     = 5150
)

// voiceParityScenarios are the shipped voiced items and the slot each is
// worn in.
var voiceParityScenarios = []struct {
	name   string
	itemId int
	slot   string
}{
	{"blackrazor", 40183, "weapon"},
	{"aegis", 40185, "offhand"},
}

// loadVoiceParityWorld loads the shipped conditions, items and voices once
// for the test, and seeds one room.
func loadVoiceParityWorld(t *testing.T) *rooms.Room {
	t.Helper()
	// The overlay first: AddOverlayOverrides rebuilds the live config, so
	// it would drop a data path set before it.
	setPinnacleEnabled(t, true)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them too, or later tests in the package read the
	// shipped pools instead of their seeds.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	t.Cleanup(itemvoices.SeedVoicesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	itemvoices.LoadDataFiles()

	room := rooms.NewRoom("voiceparity")
	room.RoomId = voiceParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// newParityBearer seeds a fresh user 1 in room, wearing a fresh instance of
// itemId in slot.
func newParityBearer(t *testing.T, room *rooms.Room, itemId int, slot string) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(voiceParityUserId, "bearer", "Bearer", 0)
	u.Character.RoomId = room.RoomId
	u.Character.HealthMax.Value = 100000 // the hunger drain never reaches its floor
	u.Character.Health = 100000
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{voiceParityUserId: u}))
	room.AddPlayer(voiceParityUserId)

	it := items.New(itemId)
	switch slot {
	case "weapon":
		u.Character.Equipment.Weapon = it
	case "offhand":
		u.Character.Equipment.Offhand = it
	default:
		t.Fatalf("newParityBearer: unknown slot %q", slot)
	}
	return u
}

// voiceParityRound runs one round of the voice machinery for the bearer:
// the per-round tick, then a kill when kill is set.
func voiceParityRound(u *users.UserRecord, room *rooms.Room, kill bool) {
	pinnacleUserTick(u, room)
	if kill {
		MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{u.UserId: 1}})
	}
}

// runVoiceParity plays one scenario and returns its record: one line per
// round that sent the bearer anything.
func runVoiceParity(t *testing.T, room *rooms.Room, itemId int, slot string) string {
	t.Helper()
	u := newParityBearer(t, room, itemId, slot)
	c := u.Character

	restoreRand := util.SetRandForTest(voiceParitySeed)
	defer restoreRand()
	defer util.ResetRoundCountForTest()
	_ = events.DrainQueuedMessagesForTest(u.UserId)

	var b strings.Builder
	for i := 0; i < voiceParityRounds; i++ {
		util.SetRoundCountForTest(uint64(voiceParityFirst + i))
		switch i {
		case voiceParityFightAt:
			c.SetAggro(0, 424242, characters.DefaultAttack)
		case voiceParityFightEnd:
			c.EndAggro()
		}
		voiceParityRound(u, room, i == voiceParityKillAt)
		msgs := events.DrainQueuedMessagesForTest(u.UserId)
		for j := range msgs {
			msgs[j] = strings.TrimRight(msgs[j], "\n")
		}
		if len(msgs) > 0 {
			fmt.Fprintf(&b, "%d: %s\n", i, strings.Join(msgs, " | "))
		}
	}
	return b.String()
}

func TestItemVoiceParity(t *testing.T) {
	room := loadVoiceParityWorld(t)
	var b strings.Builder
	for _, s := range voiceParityScenarios {
		fmt.Fprintf(&b, "## %s (%d, %s)\n", s.name, s.itemId, s.slot)
		b.WriteString(runVoiceParity(t, room, s.itemId, s.slot))
	}
	got := b.String()

	path := filepath.Join("testdata", "item_voice_parity.golden")
	if *updateVoiceParity {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (record it with -update-voices): %v", path, err)
	}
	if got != string(want) {
		t.Errorf("the voice record moved.\n--- want\n%s\n--- got\n%s", want, got)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/hooks/ -run TestItemVoiceParity -count=1`
Expected: FAIL, `read testdata\item_voice_parity.golden (record it with -update-voices)`.

- [ ] **Step 3: Record it**

Run: `go test ./internal/hooks/ -run TestItemVoiceParity -count=1 -update-voices`
Expected: `ok`, and `internal/hooks/testdata/item_voice_parity.golden` holds 21 lines: `## blackrazor (40183, weapon)`, lines at round offsets 2, 25, 45, 51, 69, 71, 91, 103, 111, 138, 165, 197, then `## aegis (40185, offhand)`, lines at 2, 25, 45, 70, 106, 142, 183. Check these points (each line starts `<offset>: <ansi fg="system"><ansi fg="item">`):

- Blackrazor 45 `I begin to consider the nearest throat. Yours is nearest.` (a hunger warning) and 51 `I would rather it were someone else's. But you were here.` (a feeding line six rounds after a chatter line: ruling S1's separate pacing)
- Blackrazor 69 `Yes! Closer! Let me taste what it is so proud of.` (a taunt) and 165 `Yes... YES. Another.` (the kill)
- Aegis 70 `You reek of defeat and, I regret to add, of yesterday's fish!` (a taunt) and nothing at 165 (today's kill line is the weapon's only)

If any differs, stop: the seed, the round offsets or the load order differ from this plan.

- [ ] **Step 4: Replay it**

Run: `go test ./internal/hooks/ -run TestItemVoiceParity -count=3`
Expected: `ok`.

- [ ] **Step 5: The same store leak in slice 1's loader (F27)**

In `internal/behaviortree/shipped_item_trees_test.go`, replace

```go
	t.Cleanup(items.SeedItemsForTest(nil))
	conditions.LoadDataFiles()
```

with

```go
	t.Cleanup(items.SeedItemsForTest(nil))
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them so later tests read their own seeds.
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
```

- [ ] **Step 6: Run both packages**

Run: `go test ./internal/hooks/ ./internal/behaviortree/ -count=1`
Expected: both `ok`. (Without the two snapshots in Step 1, `TestSpellDefence_DefenderInTheDarkIsToldWithTheAttackerHidden` later in `internal/hooks` fails on the shipped defence pools: F20.)

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/item_voice_parity_test.go internal/hooks/testdata/item_voice_parity.golden internal/behaviortree/shipped_item_trees_test.go
git commit -m "test(hooks): record the sentient voice path before it moves into trees" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The item chatter knobs (spec "Config knobs", X16)

**Model:** sonnet.

**Files:**
- Create: `internal/configs/configs_item_chatter_test.go`
- Modify: `internal/configs/config.balance.go:1113`, `internal/configs/config.balance.misc.go:490`, `_datafiles/config.yaml:2104`, `internal/hooks/pinnacle_tick.go:181-189`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/configs_item_chatter_test.go`:

```go
package configs

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// The item chatter knobs (item behaviour slice 2, spec "Config knobs").
// An absent key reads 0, and 0 is not an off switch: it reads as unset and
// takes the default, the KnockdownFrequencyScale rule
// (config.balance.misc.go), so deleting a line cannot silence every item.
// PinnacleItemsEnabled is the off switch.
func TestItemChatterKnobDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	cases := []struct {
		name      string
		got, want int
	}{
		{"ItemChatterQuietCooldownRounds", int(b.ItemChatterQuietCooldownRounds), 40},
		{"ItemChatterQuietChancePct", int(b.ItemChatterQuietChancePct), 10},
		{"ItemChatterNormalCooldownRounds", int(b.ItemChatterNormalCooldownRounds), 20},
		{"ItemChatterNormalChancePct", int(b.ItemChatterNormalChancePct), 15},
		{"ItemChatterChattyCooldownRounds", int(b.ItemChatterChattyCooldownRounds), 10},
		{"ItemChatterChattyChancePct", int(b.ItemChatterChattyChancePct), 25},
		{"ItemChatterListenerCapRounds", int(b.ItemChatterListenerCapRounds), 10},
		{"HungerFeedingLineCooldownRounds", int(b.HungerFeedingLineCooldownRounds), 20},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	b = Balance{ItemChatterNormalChancePct: 101, ItemChatterChattyChancePct: -5, ItemChatterQuietCooldownRounds: -1}
	b.Validate()
	if b.ItemChatterNormalChancePct != 15 || b.ItemChatterChattyChancePct != 25 || b.ItemChatterQuietCooldownRounds != 40 {
		t.Errorf("out-of-range values must coerce to the defaults, got normal %d%%, chatty %d%%, quiet %d rounds",
			int(b.ItemChatterNormalChancePct), int(b.ItemChatterChattyChancePct), int(b.ItemChatterQuietCooldownRounds))
	}
}

// The shipped config.yaml and the Go defaults agree, or a server started
// without the block behaves differently from one started with it (the
// TestBaubleShippedConfigMatchesDefaults precedent).
func TestItemChatterShippedConfigMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	s := cfg.Balance
	var d Balance
	d.Validate()
	if s.ItemChatterQuietCooldownRounds != d.ItemChatterQuietCooldownRounds ||
		s.ItemChatterQuietChancePct != d.ItemChatterQuietChancePct ||
		s.ItemChatterNormalCooldownRounds != d.ItemChatterNormalCooldownRounds ||
		s.ItemChatterNormalChancePct != d.ItemChatterNormalChancePct ||
		s.ItemChatterChattyCooldownRounds != d.ItemChatterChattyCooldownRounds ||
		s.ItemChatterChattyChancePct != d.ItemChatterChattyChancePct ||
		s.ItemChatterListenerCapRounds != d.ItemChatterListenerCapRounds ||
		s.HungerFeedingLineCooldownRounds != d.HungerFeedingLineCooldownRounds {
		t.Errorf("shipped item chatter knobs differ from the Go defaults: shipped quiet %d/%d normal %d/%d chatty %d/%d cap %d feeding %d",
			int(s.ItemChatterQuietCooldownRounds), int(s.ItemChatterQuietChancePct),
			int(s.ItemChatterNormalCooldownRounds), int(s.ItemChatterNormalChancePct),
			int(s.ItemChatterChattyCooldownRounds), int(s.ItemChatterChattyChancePct),
			int(s.ItemChatterListenerCapRounds), int(s.HungerFeedingLineCooldownRounds))
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/configs/ -run ItemChatter -count=1`
Expected: build failure, `b.ItemChatterQuietCooldownRounds undefined` (and more).

- [ ] **Step 3: Declare the knobs**

In `internal/configs/config.balance.go`, replace

```go
	SentientChatterChancePct      ConfigInt `yaml:"SentientChatterChancePct"`      // Percent chance per eligible round that a sentient item speaks (default 15)
```

with

```go
	SentientChatterChancePct      ConfigInt `yaml:"SentientChatterChancePct"`      // Percent chance per eligible round that a sentient item speaks (default 15)

	// Item tree chatter (item behaviour slice 2): a tree's `chatter:` level
	// picks its cooldown and chance for an ambient (item_idle) line; event
	// lines need the cooldown only. The listener cap is one ambient item
	// line per listener per that many rounds, across every item.
	ItemChatterQuietCooldownRounds  ConfigInt `yaml:"ItemChatterQuietCooldownRounds"`  // Rounds between a quiet item's lines (default 40)
	ItemChatterQuietChancePct       ConfigInt `yaml:"ItemChatterQuietChancePct"`       // Percent chance a quiet item speaks on an open round (default 10)
	ItemChatterNormalCooldownRounds ConfigInt `yaml:"ItemChatterNormalCooldownRounds"` // Rounds between a normal item's lines (default 20)
	ItemChatterNormalChancePct      ConfigInt `yaml:"ItemChatterNormalChancePct"`      // Percent chance a normal item speaks on an open round (default 15)
	ItemChatterChattyCooldownRounds ConfigInt `yaml:"ItemChatterChattyCooldownRounds"` // Rounds between a chatty item's lines (default 10)
	ItemChatterChattyChancePct      ConfigInt `yaml:"ItemChatterChattyChancePct"`      // Percent chance a chatty item speaks on an open round (default 25)
	ItemChatterListenerCapRounds    ConfigInt `yaml:"ItemChatterListenerCapRounds"`    // A listener hears at most one ambient item line per this many rounds (default 10)
	HungerFeedingLineCooldownRounds ConfigInt `yaml:"HungerFeedingLineCooldownRounds"` // Rounds between a hungry weapon's feeding lines; the drain itself runs every round (default 20)
```

In `internal/configs/config.balance.misc.go`, replace

```go
	if b.SentientChatterChancePct < 1 || b.SentientChatterChancePct > 100 {
		b.SentientChatterChancePct = 15
	}
}
```

with

```go
	if b.SentientChatterChancePct < 1 || b.SentientChatterChancePct > 100 {
		b.SentientChatterChancePct = 15
	}
	// Item tree chatter (item behaviour slice 2). `<= 0` and `< 1`, not
	// `< 0`: an absent key reads 0, and a `< 0` check cannot repair it (the
	// trap noted at KnockdownFrequencyScale). So 0 is not an off switch for
	// these; PinnacleItemsEnabled is.
	if b.ItemChatterQuietCooldownRounds <= 0 {
		b.ItemChatterQuietCooldownRounds = 40
	}
	if b.ItemChatterQuietChancePct < 1 || b.ItemChatterQuietChancePct > 100 {
		b.ItemChatterQuietChancePct = 10
	}
	if b.ItemChatterNormalCooldownRounds <= 0 {
		b.ItemChatterNormalCooldownRounds = 20
	}
	if b.ItemChatterNormalChancePct < 1 || b.ItemChatterNormalChancePct > 100 {
		b.ItemChatterNormalChancePct = 15
	}
	if b.ItemChatterChattyCooldownRounds <= 0 {
		b.ItemChatterChattyCooldownRounds = 10
	}
	if b.ItemChatterChattyChancePct < 1 || b.ItemChatterChattyChancePct > 100 {
		b.ItemChatterChattyChancePct = 25
	}
	if b.ItemChatterListenerCapRounds <= 0 {
		b.ItemChatterListenerCapRounds = 10
	}
	if b.HungerFeedingLineCooldownRounds <= 0 {
		b.HungerFeedingLineCooldownRounds = 20
	}
}
```

- [ ] **Step 4: Ship them in `config.yaml`**

In `_datafiles/config.yaml` (the worktree copy is the committed blob; see Branch), replace

```yaml
  SentientChatterChancePct: 15        # Percent chance per eligible round that a sentient item speaks
```

with

```yaml
  SentientChatterChancePct: 15        # Percent chance per eligible round that a sentient item speaks
  # Item tree chatter: a tree's chatter level (quiet, normal, chatty) sets
  # the rounds between its lines and its chance on an open round. A
  # listener hears at most one ambient item line per ItemChatterListenerCapRounds.
  # 0 reads as unset and takes the default; PinnacleItemsEnabled is the off switch.
  ItemChatterQuietCooldownRounds: 40
  ItemChatterQuietChancePct: 10
  ItemChatterNormalCooldownRounds: 20
  ItemChatterNormalChancePct: 15
  ItemChatterChattyCooldownRounds: 10
  ItemChatterChattyChancePct: 25
  ItemChatterListenerCapRounds: 10
  HungerFeedingLineCooldownRounds: 20 # Rounds between a hungry weapon's feeding lines (the drain runs every round)
```

Run: `git diff --stat _datafiles/config.yaml`
Expected: `1 file changed, 12 insertions(+)`.

- [ ] **Step 5: The feeding line reads its own knob (X16)**

In `internal/hooks/pinnacle_tick.go`, replace

```go
	// The drain repeats every overdue round, but the feeding LINE is paced by
	// its own cooldown (reusing the chatter knob) so an ignored hunger debt
	// doesn't spam the player every round.
```

with

```go
	// The drain repeats every overdue round, but the feeding LINE is paced by
	// its own cooldown (HungerFeedingLineCooldownRounds) so an ignored hunger
	// debt doesn't spam the player every round.
```

and replace

```go
				now+uint64(configs.GetBalanceConfig().SentientChatterCooldownRounds))
		}
	}
}
```

(the one inside `tickHunger`, after `pinnacle_hunger_msg_next_round`) with

```go
				now+uint64(configs.GetBalanceConfig().HungerFeedingLineCooldownRounds))
		}
	}
}
```

- [ ] **Step 6: Run the packages and the root guards**

Run: `go test ./internal/configs/ -count=1 && go test ./internal/hooks/ -run 'TestItemVoiceParity|TestPinnacle' -count=1 && go test . -count=1 -run 'Config|Knob|Balance'`
Expected: three `ok`. The parity record does not move (both knobs are 20).

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.misc.go internal/configs/configs_item_chatter_test.go _datafiles/config.yaml internal/hooks/pinnacle_tick.go
git commit -m "feat(configs): item chatter knobs; the feeding line gets its own pace" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: An item tree's voice: `speech:` and `chatter:` (Rule 16)

**Model:** sonnet.

**Files:**
- Create: `internal/behaviortree/item_voice.go`, `internal/behaviortree/item_voice_test.go`
- Modify: `internal/behaviortree/types.go:113`, `loader.go:41-93`, `engine.go:24,46,213-230,249-254`, `test_export.go:93-117`

- [ ] **Step 1: Write the failing test**

Create `internal/behaviortree/item_voice_test.go`:

```go
package behaviortree

import (
	"strings"
	"testing"
)

// An item tree carries its voice: line pools by name and a chatter level
// (item behaviour slice 2, Rule 16).
func TestItemTreeCarriesItsVoice(t *testing.T) {
	LoadItemTreeForTest(t, "voice_probe", `
chatter: chatty
speech:
  on_idle:
    - "Hm."
    - "Hmm."
tree:
  type: condition
  check: worn
`)
	v := GetEngine().GetItemVoice("voice_probe")
	if v == nil {
		t.Fatal("GetItemVoice(voice_probe) = nil, want the tree's voice")
	}
	if v.Chatter != ChatterChatty {
		t.Errorf("Chatter = %q, want %q", v.Chatter, ChatterChatty)
	}
	if got := v.Speech["on_idle"]; len(got) != 2 || got[1] != "Hmm." {
		t.Errorf("Speech[on_idle] = %q, want the two authored lines", got)
	}

	LoadItemTreeForTest(t, "voice_default", "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: worn\n")
	if v := GetEngine().GetItemVoice("voice_default"); v == nil || v.Chatter != ChatterNormal {
		t.Errorf("a tree with no chatter: level must default to %q, got %+v", ChatterNormal, v)
	}

	LoadItemTreeForTest(t, "voice_none", "tree:\n  type: condition\n  check: worn\n")
	if v := GetEngine().GetItemVoice("voice_none"); v != nil {
		t.Errorf("a tree with no speech has no voice, got %+v", v)
	}
}

// A malformed voice refuses at load, and only an item tree may carry one.
func TestItemVoiceRefusesAtLoad(t *testing.T) {
	bad := map[string]string{
		"unknown chatter": "chatter: loud\nspeech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: worn\n",
		"empty pool":      "speech:\n  on_idle: []\ntree:\n  type: condition\n  check: worn\n",
		"blank line":      "speech:\n  on_idle: [\"  \"]\ntree:\n  type: condition\n  check: worn\n",
		"chatter alone":   "chatter: quiet\ntree:\n  type: condition\n  check: worn\n",
	}
	for name, src := range bad {
		if _, _, err := loadItemTreeDef([]byte(src)); err == nil {
			t.Errorf("%s: loaded, want a refusal", name)
		}
	}

	mob := "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: random_chance\n  percent: 50\n"
	if _, err := LoadTreeFromBytes([]byte(mob)); err == nil || !strings.Contains(err.Error(), "item tree") {
		t.Errorf("a mob or room tree with speech: err = %v, want a refusal naming item trees", err)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/behaviortree/ -run 'ItemTreeCarriesItsVoice|ItemVoiceRefuses' -count=1`
Expected: build failure, `GetEngine().GetItemVoice undefined` and `undefined: loadItemTreeDef`.

- [ ] **Step 3: `TreeDef` gains the voice**

In `internal/behaviortree/types.go`, replace

```go
	DefaultGoals []GoalDefault      `yaml:"default_goals,omitempty" json:"default_goals,omitempty"` // chunk 4.3
}
```

with

```go
	DefaultGoals []GoalDefault      `yaml:"default_goals,omitempty" json:"default_goals,omitempty"` // chunk 4.3

	// Speech and Chatter are an item tree's voice (item behaviour slice 2,
	// Rule 16): line pools by name, read by `speak`, and the pacing level
	// of its ambient lines. Only an item tree may carry them.
	Speech  map[string][]string `yaml:"speech,omitempty" json:"speech,omitempty"`
	Chatter string              `yaml:"chatter,omitempty" json:"chatter,omitempty"`
}
```

- [ ] **Step 4: `item_voice.go`**

Create `internal/behaviortree/item_voice.go`:

```go
package behaviortree

import (
	"fmt"
	"sort"
	"strings"
)

// An item tree's voice (item behaviour slice 2, Rule 16): the line pools
// its `speak` nodes draw from and the chatter level that paces its ambient
// lines. The pools live in the tree file, beside the tree that speaks them,
// so the two cannot drift apart the way an item and a separate voice file
// could.

// The chatter levels. Each names a cooldown and a chance in config.yaml
// (ItemChatter*); a tree that names none is normal.
const (
	ChatterQuiet  = "quiet"
	ChatterNormal = "normal"
	ChatterChatty = "chatty"
)

// ItemVoice is an item tree's voice.
type ItemVoice struct {
	Speech  map[string][]string // pool name to its lines
	Chatter string              // ChatterQuiet, ChatterNormal or ChatterChatty
}

// itemVoiceFrom checks a tree's speech and chatter and returns its voice:
// nil for a tree with no speech. A chatter level with no speech, an unknown
// level, an empty pool or a blank line is an error.
func itemVoiceFrom(def TreeDef) (*ItemVoice, error) {
	if len(def.Speech) == 0 {
		if def.Chatter != `` {
			return nil, fmt.Errorf("chatter %q: a tree with no speech has nothing to pace", def.Chatter)
		}
		return nil, nil
	}
	chatter := def.Chatter
	switch chatter {
	case ``:
		chatter = ChatterNormal
	case ChatterQuiet, ChatterNormal, ChatterChatty:
	default:
		return nil, fmt.Errorf("chatter %q: want %s, %s or %s", def.Chatter, ChatterQuiet, ChatterNormal, ChatterChatty)
	}
	pools := make([]string, 0, len(def.Speech))
	for name := range def.Speech {
		pools = append(pools, name)
	}
	sort.Strings(pools)
	for _, name := range pools {
		lines := def.Speech[name]
		if len(lines) == 0 {
			return nil, fmt.Errorf("speech pool %q has no lines", name)
		}
		for i, line := range lines {
			if strings.TrimSpace(line) == `` {
				return nil, fmt.Errorf("speech pool %q line %d is blank", name, i+1)
			}
		}
	}
	return &ItemVoice{Speech: def.Speech, Chatter: chatter}, nil
}

// refuseItemVoice is the mob, room and archetype loaders' check: only an
// item tree has a voice.
func refuseItemVoice(def TreeDef) error {
	if len(def.Speech) > 0 || def.Chatter != `` {
		return fmt.Errorf("speech and chatter belong only in an item tree (behaviors/items/)")
	}
	return nil
}
```

- [ ] **Step 5: The loaders**

In `internal/behaviortree/loader.go`, in `LoadArchetypeYAMLFromFile` replace

```go
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, nil, fmt.Errorf("parse error: %w", err)
	}
	// Archetype trees compile
```

with

```go
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, nil, fmt.Errorf("parse error: %w", err)
	}
	if err := refuseItemVoice(def); err != nil {
		return nil, nil, nil, err
	}
	// Archetype trees compile
```

replace

```go
func LoadTreeFromBytes(data []byte) (Node, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return compileNode(def.Tree, "root")
}
```

with

```go
func LoadTreeFromBytes(data []byte) (Node, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if err := refuseItemVoice(def); err != nil {
		return nil, err
	}
	return compileNode(def.Tree, "root")
}
```

and replace

```go
// LoadItemTreeFromFile reads an item tree YAML file and compiles it.
func LoadItemTreeFromFile(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadItemTreeFromBytes(data)
}

// LoadItemTreeFromBytes parses an item tree and compiles it under the item
// root label, which holds every node to the item-safe allowlist (Rule 8).
func LoadItemTreeFromBytes(data []byte) (Node, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return compileNode(def.Tree, itemRootLabel)
}
```

with

```go
// LoadItemTreeFromFile reads an item tree YAML file and compiles it,
// returning its voice too (nil when it has no speech).
func LoadItemTreeFromFile(path string) (Node, *ItemVoice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return loadItemTreeDef(data)
}

// LoadItemTreeFromBytes parses an item tree and compiles it under the item
// root label, which holds every node to the item-safe allowlist (Rule 8).
func LoadItemTreeFromBytes(data []byte) (Node, error) {
	node, _, err := loadItemTreeDef(data)
	return node, err
}

// loadItemTreeDef parses an item tree, checks its voice (item behaviour
// slice 2) and compiles its tree.
func loadItemTreeDef(data []byte) (Node, *ItemVoice, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}
	voice, err := itemVoiceFrom(def)
	if err != nil {
		return nil, nil, err
	}
	node, err := compileNode(def.Tree, itemRootLabel)
	if err != nil {
		return nil, nil, err
	}
	return node, voice, nil
}
```

- [ ] **Step 6: The engine keeps the voice**

In `internal/behaviortree/engine.go`, replace the whole line

```go
	noItemTree            map[string]bool               // item tree name → its file failed to load
```

with the two lines

```go
	noItemTree            map[string]bool               // item tree name → its file failed to load
	itemVoices            map[string]*ItemVoice         // item tree name → its voice, when it has speech (item behaviour slice 2)
```

(do NOT include the following `queue` line in the edit: it is aligned with trailing spaces an edit would drop), replace

```go
		noItemTree:            make(map[string]bool),
	}
```

with

```go
		noItemTree:            make(map[string]bool),
		itemVoices:            make(map[string]*ItemVoice),
	}
```

replace

```go
func (e *Engine) LoadItemTree(name string, path string) error {
	node, err := LoadItemTreeFromFile(path)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.itemTrees[name] = node
	delete(e.noItemTree, name)
	e.mu.Unlock()
	return nil
}

// GetItemTree returns the cached item tree by name, or nil.
func (e *Engine) GetItemTree(name string) Node {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.itemTrees[name]
}
```

with

```go
func (e *Engine) LoadItemTree(name string, path string) error {
	node, voice, err := LoadItemTreeFromFile(path)
	if err != nil {
		return err
	}
	e.installItemTree(name, node, voice)
	return nil
}

// installItemTree caches a compiled item tree and its voice (nil when it
// has no speech) and clears any negative entry.
func (e *Engine) installItemTree(name string, node Node, voice *ItemVoice) {
	e.mu.Lock()
	e.itemTrees[name] = node
	if voice != nil {
		e.itemVoices[name] = voice
	} else {
		delete(e.itemVoices, name)
	}
	delete(e.noItemTree, name)
	e.mu.Unlock()
}

// GetItemTree returns the cached item tree by name, or nil.
func (e *Engine) GetItemTree(name string) Node {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.itemTrees[name]
}

// GetItemVoice returns the named item tree's voice, or nil when the tree is
// not loaded or has no speech (item behaviour slice 2).
func (e *Engine) GetItemVoice(name string) *ItemVoice {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.itemVoices[name]
}
```

and in `EvictItemTree` replace

```go
	delete(e.itemTrees, name)
	delete(e.noItemTree, name)
```

with

```go
	delete(e.itemTrees, name)
	delete(e.itemVoices, name)
	delete(e.noItemTree, name)
```

- [ ] **Step 7: The test loader installs the voice too**

In `internal/behaviortree/test_export.go`, in `LoadItemTreeForTest` replace

```go
	node, err := LoadItemTreeFromBytes([]byte(yamlText))
	if err != nil {
		t.Fatalf("LoadItemTreeForTest(%s): %v", name, err)
	}
	e := GetEngine()
	e.mu.Lock()
	prev, hadPrev := e.itemTrees[name]
	e.itemTrees[name] = node
	delete(e.noItemTree, name)
	e.mu.Unlock()
	t.Cleanup(func() {
		e.mu.Lock()
		if hadPrev {
			e.itemTrees[name] = prev
		} else {
			delete(e.itemTrees, name)
		}
		e.mu.Unlock()
	})
}
```

with

```go
	node, voice, err := loadItemTreeDef([]byte(yamlText))
	if err != nil {
		t.Fatalf("LoadItemTreeForTest(%s): %v", name, err)
	}
	e := GetEngine()
	e.mu.Lock()
	prev, hadPrev := e.itemTrees[name]
	prevVoice, hadVoice := e.itemVoices[name]
	e.mu.Unlock()
	e.installItemTree(name, node, voice)
	t.Cleanup(func() {
		e.mu.Lock()
		if hadPrev {
			e.itemTrees[name] = prev
		} else {
			delete(e.itemTrees, name)
		}
		if hadVoice {
			e.itemVoices[name] = prevVoice
		} else {
			delete(e.itemVoices, name)
		}
		e.mu.Unlock()
	})
}
```

- [ ] **Step 8: Run the package**

Run: `gofmt -l internal/behaviortree/ && go build ./... && go test ./internal/behaviortree/ -count=1`
Expected: gofmt prints nothing; `ok` (including `TestRoundTrip_MarshalFixedPointEveryLiveBehaviorFile`, which marshals every live tree through the widened `TreeDef`).

- [ ] **Step 9: Commit**

```bash
git add internal/behaviortree/types.go internal/behaviortree/item_voice.go internal/behaviortree/item_voice_test.go internal/behaviortree/loader.go internal/behaviortree/engine.go internal/behaviortree/test_export.go
git commit -m "feat(behaviortree): an item tree carries its voice (speech, chatter)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: The voice nodes: `chatter_ready`, `speak`, `taunt_pull`, `hunger_overdue` (Rules 16 to 18; X14, X15, X17; R5, R6, S1)

**Model:** sonnet.

**Files:**
- Create: `internal/behaviortree/actions_item_voice.go`, `internal/behaviortree/item_voice_nodes_test.go`
- Modify: `internal/behaviortree/conditions.go:53`, `actions.go:129`, `loader.go:106-130,92` (allowlist; speak check), `item_voice.go` (speak check); `internal/characters/character.go:608`; `internal/hooks/item_procs.go:37-52`; `condition_apply_path_guard_test.go:138,295`

- [ ] **Step 1: Write the failing tests**

Create `internal/behaviortree/item_voice_nodes_test.go`:

```go
package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The voice nodes (item behaviour slice 2, Rules 16 to 18).

const (
	voiceProbeItemId   = 99640
	voiceProbeItemId2  = 99641
	voiceBearerId      = 71
	voiceListenerId    = 72
	voiceLitRoom       = 9961
	voiceDarkRoom      = 9962
	voiceMobTemplateId = 9963
	voiceMobInstanceId = 9964
)

// voiceProbeTree speaks its idle pool through chatter_ready, its event
// pools straight, and its feeding pool to the holder unpaced.
const voiceProbeTree = `
speech:
  on_idle: ["Hm."]
  on_equip: ["Up we go."]
  on_hunger_feeding: ["A sip."]
tree:
  type: selector
  children:
    - type: sequence
      event: item_idle
      children:
        - type: condition
          check: chatter_ready
        - type: action
          do: speak
          pool: on_idle
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_hunger_feeding
      do: speak
      pool: on_hunger_feeding
      to: holder
      paced: false
`

// seedVoiceWorld: a lit and a dark room, the bearer and a listener in the
// lit one, two probe items sharing the probe tree, the Pinnacle switch on
// and the normal chatter level at 100% so no roll can fail.
func seedVoiceWorld(t *testing.T) (bearer, listener *users.UserRecord) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = true
	cfg.Balance.ItemChatterNormalChancePct = 100
	cfg.Balance.ItemChatterNormalCooldownRounds = 20
	cfg.Balance.ItemChatterListenerCapRounds = 10
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		voiceProbeItemId:  {ItemId: voiceProbeItemId, Name: "Probe Shield", Type: items.Offhand, Behavior: "voice_nodes_probe"},
		voiceProbeItemId2: {ItemId: voiceProbeItemId2, Name: "Probe Blade", Type: items.Weapon, Hands: 1, Behavior: "voice_nodes_probe"},
	}))
	LoadItemTreeForTest(t, "voice_nodes_probe", voiceProbeTree)
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(ResetItemListenerCapsForTest())

	lit := &rooms.Room{RoomId: voiceLitRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	dark := &rooms.Room{RoomId: voiceDarkRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceLitRoom: lit, voiceDarkRoom: dark}, map[string]*rooms.ZoneConfig{}))

	bearer = users.NewTestUser(voiceBearerId, "kesh", "Kesh", 0)
	listener = users.NewTestUser(voiceListenerId, "clara", "Clara", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{voiceBearerId: bearer, voiceListenerId: listener}))
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = voiceLitRoom
		lit.AddPlayer(u.UserId)
	}
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(5000)
	_ = events.DrainQueuedMessagesForTest(voiceBearerId)
	_ = events.DrainQueuedMessagesForTest(voiceListenerId)
	return bearer, listener
}

func wornProbe(itemId int, id byte) ItemSubject {
	return ItemSubject{UUID: uuid.UUID{id}, ItemId: itemId, UserId: voiceBearerId, Slot: "offhand"}
}

func voiceEvent(name string) EventContext { return EventContext{EventType: name} }

func drained(userId int) string {
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

// Rule 16: the holder reads "<Item> says", the room "<Name>'s <Item>
// mutters", heard by everyone.
func TestSpeakSendsTheHolderAndTheRoom(t *testing.T) {
	seedVoiceWorld(t)
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("an open cooldown, a clear cap and a 100% chance: the idle line must go out")
	}
	if got := drained(voiceBearerId); !strings.Contains(got, `Probe Shield</ansi> says, "<ansi fg="yellow">Hm.`) {
		t.Errorf("the holder reads %q, want the item's says line", got)
	}
	if got := drained(voiceListenerId); !strings.Contains(got, `Kesh</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters, "<ansi fg="yellow">Hm.`) {
		t.Errorf("the listener reads %q, want the bearer's item muttering", got)
	}
}

// Spec X17: the room line is heard, and the bearer's name is hidden by each
// listener's sight. Shown to fail against the Pinnacle tick's sender
// (room.SendTextVisual), which sends a listener in the dark nothing.
func TestSpeakHidesTheBearerFromAListenerInTheDark(t *testing.T) {
	bearer, listener := seedVoiceWorld(t)
	dark := rooms.LoadRoom(voiceDarkRoom)
	rooms.LoadRoom(voiceLitRoom).RemovePlayer(voiceBearerId)
	rooms.LoadRoom(voiceLitRoom).RemovePlayer(voiceListenerId)
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = voiceDarkRoom
		dark.AddPlayer(u.UserId)
	}
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("the idle line must go out in the dark too")
	}
	got := drained(voiceListenerId)
	if !strings.Contains(got, `Someone</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters`) {
		t.Errorf("a listener in the dark reads %q, want the words with the bearer hidden as someone", got)
	}
	if strings.Contains(got, "Kesh") {
		t.Errorf("a listener in the dark reads the bearer's name: %q", got)
	}
}

// Rule 17: an ambient line needs the item's cooldown, and every listener
// past their cap; hearing one starts the cap. An event line bypasses the
// cap but needs the cooldown.
func TestSpeakPacing(t *testing.T) {
	seedVoiceWorld(t)
	shield, blade := wornProbe(voiceProbeItemId, 1), wornProbe(voiceProbeItemId2, 2)

	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Fatal("the first idle line must go out")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)

	util.SetRoundCountForTest(5001)
	if TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("the shield's own cooldown is closed: no second idle line")
	}
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("the blade's cooldown is open but both listeners are inside their cap: no ambient line")
	}
	if !TryItemBehavior(voiceEvent("on_equip"), blade) {
		t.Error("an event line bypasses the listener cap")
	}
	if TryItemBehavior(voiceEvent("on_equip"), shield) {
		t.Error("an event line still needs the item's own cooldown")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)

	util.SetRoundCountForTest(5010)
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("the blade's event line at 5001 armed its cooldown: no idle line at 5010")
	}
	util.SetRoundCountForTest(5021)
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("past both the cap and the blade's cooldown: the idle line goes out")
	}
}

// Ruling S1: the feeding line goes to the holder alone, ignores the
// item's cooldown and does not arm it.
func TestSpeakToTheHolderUnpaced(t *testing.T) {
	seedVoiceWorld(t)
	blade := wornProbe(voiceProbeItemId2, 2)
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Fatal("the idle line must go out")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)
	if !TryItemBehavior(voiceEvent("on_hunger_feeding"), blade) {
		t.Fatal("an unpaced line goes out with the cooldown closed")
	}
	if got := drained(voiceBearerId); !strings.Contains(got, "A sip.") {
		t.Errorf("the holder reads %q, want the feeding line", got)
	}
	if got := drained(voiceListenerId); got != "" {
		t.Errorf("to: holder sends the room nothing, got %q", got)
	}
}

// Ruling R6: a mob holder's item speaks the room line only, the mob's name
// hidden like a player's.
func TestSpeakForAMobHolder(t *testing.T) {
	seedVoiceWorld(t)
	t.Cleanup(seedTestMob(t, voiceMobTemplateId, voiceMobInstanceId, voiceLitRoom, "ogre"))
	rooms.LoadRoom(voiceLitRoom).AddMob(voiceMobInstanceId)
	held := ItemSubject{UUID: uuid.UUID{3}, ItemId: voiceProbeItemId, MobInstanceId: voiceMobInstanceId, Slot: "offhand"}
	if !TryItemBehavior(voiceEvent("item_idle"), held) {
		t.Fatal("a mob-held voiced item speaks")
	}
	// The renderer capitalises the name at the sentence start.
	if got := drained(voiceListenerId); !strings.Contains(got, `<ansi fg="mobname">Ogre</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters`) {
		t.Errorf("the room reads %q, want the mob's item muttering", got)
	}
}

// The Pinnacle switch silences every voice node.
func TestVoiceNodesHonourThePinnacleSwitch(t *testing.T) {
	seedVoiceWorld(t)
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = false
	configs.SetConfigForTest(t, cfg)
	if TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) ||
		TryItemBehavior(voiceEvent("on_equip"), wornProbe(voiceProbeItemId, 1)) {
		t.Error("PinnacleItemsEnabled off: no item speaks")
	}
	if got := drained(voiceBearerId) + drained(voiceListenerId); got != "" {
		t.Errorf("PinnacleItemsEnabled off sent %q", got)
	}
}

// Spec X14: hunger_overdue reads the holder's hunger anchor against the
// item's hunger window, as the Pinnacle tick's pickVoiceEvent did.
func TestHungerOverdue(t *testing.T) {
	bearer, _ := seedVoiceWorld(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		voiceProbeItemId2: {ItemId: voiceProbeItemId2, Name: "Probe Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01},
	}))
	LoadItemTreeForTest(t, "hunger_probe", "tree:\n  type: condition\n  check: hunger_overdue\n  fraction: 0.75\n")
	items.GetItemSpec(voiceProbeItemId2).Behavior = "hunger_probe"
	blade := ItemSubject{UUID: uuid.UUID{2}, ItemId: voiceProbeItemId2, UserId: voiceBearerId, Slot: "weapon"}

	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("no anchor yet: not overdue")
	}
	bearer.Character.SetMiscData("pinnacle_hunger_anchor", uint64(5000-37))
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("37 of 50 rounds: not past three quarters")
	}
	bearer.Character.SetMiscData("pinnacle_hunger_anchor", uint64(5000-38))
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("38 of 50 rounds: past three quarters")
	}
}

// Spec X15, ruling R5: taunt_pull moves the bearer's foe onto the bearer
// and always succeeds, so a failed pull never falls through to another
// line.
func TestTauntPull(t *testing.T) {
	bearer, _ := seedVoiceWorld(t)
	LoadItemTreeForTest(t, "taunt_probe", "tree:\n  type: action\n  do: taunt_pull\n")
	items.GetItemSpec(voiceProbeItemId).Behavior = "taunt_probe"
	t.Cleanup(func() { items.GetItemSpec(voiceProbeItemId).Behavior = "voice_nodes_probe" })
	t.Cleanup(seedTestMob(t, voiceMobTemplateId, voiceMobInstanceId, voiceLitRoom, "ogre"))
	foe := mobs.GetInstance(voiceMobInstanceId)
	shield := wornProbe(voiceProbeItemId, 1)

	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull with nothing to pull still succeeds")
	}
	bearer.Character.SetAggro(0, voiceMobInstanceId, characters.DefaultAttack)
	foe.Character.SetAggro(voiceListenerId, 0, characters.DefaultAttack)

	foe.Character.NonCombatant = true
	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull on a non-combatant still succeeds")
	}
	if got := foe.Character.CurrentCombatTarget().UserId; got != voiceListenerId {
		t.Errorf("a non-combatant is never pulled: it fights user %d, want %d", got, voiceListenerId)
	}

	foe.Character.NonCombatant = false
	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull succeeds")
	}
	if got := foe.Character.CurrentCombatTarget().UserId; got != voiceBearerId {
		t.Errorf("the foe fights user %d, want the bearer %d", got, voiceBearerId)
	}

	// A foe already on the bearer is left as it is.
	already := foe.Character.CurrentCombatTarget()
	TryItemBehavior(voiceEvent("item_idle"), shield)
	if foe.Character.CurrentCombatTarget() != already {
		t.Errorf("a foe already on the bearer was re-forced: %+v", foe.Character.CurrentCombatTarget())
	}
}

// A speak node names a pool its tree has, and an audience it knows.
func TestSpeakRefusesAtLoad(t *testing.T) {
	bad := map[string]string{
		"unknown pool":     "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: action\n  do: speak\n  pool: on_kill\n",
		"no speech":        "tree:\n  type: action\n  do: speak\n  pool: on_idle\n",
		"unknown audience": "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: action\n  do: speak\n  pool: on_idle\n  to: everyone\n",
	}
	for name, src := range bad {
		if _, _, err := loadItemTreeDef([]byte(src)); err == nil {
			t.Errorf("%s: loaded, want a refusal", name)
		}
	}
	for _, node := range []string{"  type: action\n  do: speak\n  pool: on_idle\n", "  type: condition\n  check: chatter_ready\n"} {
		if _, err := LoadTreeFromBytes([]byte("tree:\n" + node)); err == nil || !strings.Contains(err.Error(), "item") {
			t.Errorf("a mob tree naming an item voice node: err = %v, want a refusal", err)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go vet ./internal/behaviortree/`
Expected: `undefined: ResetItemListenerCapsForTest`.

- [ ] **Step 3: One MiscData round reader**

In `internal/characters/character.go`, replace

```go
func (c *Character) GetMiscDataKeys(prefixMatch ...string) []string {
```

with

```go
// MiscRound reads a round number stored in MiscData. A value saved as a
// uint64 comes back from YAML as an int, int64 or float64, so all four are
// read; anything else, nil included, is not a round.
func MiscRound(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case int:
		return uint64(n), true
	case int64:
		return uint64(n), true
	case float64:
		return uint64(n), true
	}
	return 0, false
}

func (c *Character) GetMiscDataKeys(prefixMatch ...string) []string {
```

In `internal/hooks/item_procs.go`, replace

```go
// readMiscRound tolerates int/uint64/float64 from yaml round-tripping —
// MiscData persists to player YAML and numeric types are not stable across a
// save/load cycle.
func readMiscRound(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case int:
		return uint64(n), true
	case int64:
		return uint64(n), true
	case float64:
		return uint64(n), true
	}
	return 0, false
}
```

with

```go
// readMiscRound tolerates int/uint64/float64 from yaml round-tripping —
// MiscData persists to player YAML and numeric types are not stable across a
// save/load cycle. The one reader is characters.MiscRound, which the item
// tree nodes read too.
func readMiscRound(v any) (uint64, bool) {
	return characters.MiscRound(v)
}
```

(the first comment line keeps its existing dash: an unchanged line). That shortens `item_procs.go` by nine lines, so in `condition_apply_path_guard_test.go` replace the key `"internal/hooks/item_procs.go|265":` with `"internal/hooks/item_procs.go|256":` and the key `"internal/hooks/item_procs.go|215":` with `"internal/hooks/item_procs.go|206":` (keys only; the reasons stay). Confirm with `grep -n "AddConditionMagnitude(conditions.ConditionIdBleeding\|AddCondition(84" internal/hooks/item_procs.go`: lines 206 and 256.

- [ ] **Step 4: The nodes**

Create `internal/behaviortree/actions_item_voice.go`:

```go
package behaviortree

import (
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The voice nodes (item behaviour slice 2, Rules 16 to 18). All four need
// an item subject (Rule 8), and all four are the Pinnacle feature:
// PinnacleItemsEnabled off silences them.
//
// Pacing is split across two nodes so a tree reads the way the Pinnacle
// tick behaved. `chatter_ready` gates an AMBIENT line: the item's cooldown
// open, someone to hear it and every listener past their cap, then the
// level's chance, drawn last so a closed round draws nothing. `speak` sends
// a line and arms the item's cooldown, and for an item_idle line starts
// every listener's cap. An event branch (on_equip, on_kill...) speaks
// without chatter_ready: it bypasses the cap and the chance, and speak
// still refuses while the item's cooldown is closed.

// speakNextRoundKey is the item state key holding the round its cooldown
// opens again.
const speakNextRoundKey = `speak_next_round`

// The speak audiences: the holder and the room (default), or the holder
// alone.
const (
	speakToAll    = `all`
	speakToHolder = `holder`
)

var (
	listenerCapMu sync.Mutex
	// listenerCapNext is the round each listening player may next hear an
	// ambient item line, across every item (Rule 17). In memory only.
	listenerCapNext = map[int]uint64{}
)

// ResetItemListenerCapsForTest empties the listener caps and returns a
// restore func.
func ResetItemListenerCapsForTest() func() {
	listenerCapMu.Lock()
	orig := listenerCapNext
	listenerCapNext = map[int]uint64{}
	listenerCapMu.Unlock()
	return func() {
		listenerCapMu.Lock()
		listenerCapNext = orig
		listenerCapMu.Unlock()
	}
}

func pinnacleItemsOn() bool {
	return bool(configs.GetConfig().GamePlay.PinnacleItemsEnabled)
}

// itemVoiceOf is the subject item's voice, through its template's tree.
func itemVoiceOf(ctx *EvalContext) *ItemVoice {
	if ctx == nil || ctx.Item == nil {
		return nil
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	if spec == nil || spec.Behavior == `` {
		return nil
	}
	return GetEngine().GetItemVoice(spec.Behavior)
}

// chatterPacing is a chatter level's cooldown in rounds and chance in
// percent, from config.yaml.
func chatterPacing(level string) (uint64, int) {
	b := configs.GetBalanceConfig()
	switch level {
	case ChatterQuiet:
		return uint64(b.ItemChatterQuietCooldownRounds), int(b.ItemChatterQuietChancePct)
	case ChatterChatty:
		return uint64(b.ItemChatterChattyCooldownRounds), int(b.ItemChatterChattyChancePct)
	}
	return uint64(b.ItemChatterNormalCooldownRounds), int(b.ItemChatterNormalChancePct)
}

func itemCooldownOpen(ctx *EvalContext, now uint64) bool {
	next, ok := characters.MiscRound(ctx.MobState.Get(speakNextRoundKey))
	return !ok || now >= next
}

// condChatterReady: an ambient line may go out this round.
func condChatterReady(_ map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() {
		return Failure
	}
	voice := itemVoiceOf(ctx)
	if voice == nil {
		return Failure
	}
	now := util.GetRoundCount()
	if !itemCooldownOpen(ctx, now) {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	listeners := room.GetPlayers()
	if len(listeners) == 0 {
		return Failure
	}
	listenerCapMu.Lock()
	for _, uid := range listeners {
		if now < listenerCapNext[uid] {
			listenerCapMu.Unlock()
			return Failure
		}
	}
	listenerCapMu.Unlock()
	_, chance := chatterPacing(voice.Chatter)
	if util.Rand(100) >= chance {
		return Failure
	}
	return Success
}

// condHungerOverdue: the holder's hunger anchor (the round the item last
// fed, pinnacle_hunger_anchor) is more than `fraction` of the item's
// hunger window behind. Spec X14: the Pinnacle tick's warning state.
func condHungerOverdue(params map[string]any, ctx *EvalContext) Result {
	c := itemHolder(ctx)
	if c == nil {
		return Failure
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	if spec == nil || spec.HungerRounds <= 0 {
		return Failure
	}
	anchor, ok := characters.MiscRound(c.GetMiscData(`pinnacle_hunger_anchor`))
	if !ok {
		return Failure
	}
	now := util.GetRoundCount()
	fraction := getFloatParam(params, `fraction`, 0.75)
	if now > anchor && float64(now-anchor) > float64(spec.HungerRounds)*fraction {
		return Success
	}
	return Failure
}

// actSpeak: `speak` with `pool` (a pool of the tree's speech), `to: all |
// holder` (default all) and `paced: true | false` (default true). It picks
// a line through the narration core (the picker seam, so a seeded
// util.Rand replays it), sends the holder "<Item> says" and, for `all`,
// the room "<Name>'s <Item> mutters" heard by everyone with the holder's
// name hidden at each listener's sight (spec X17; never deafen-filtered,
// owner ruling 6). A mob holder gets the room line only (ruling R6).
// Paced: refuses while the item's cooldown is closed, and arms it.
func actSpeak(params map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() {
		return Failure
	}
	voice := itemVoiceOf(ctx)
	if voice == nil {
		return Failure
	}
	lines := voice.Speech[getStringParam(params, `pool`)]
	if len(lines) == 0 {
		return Failure
	}
	paced := true
	if _, ok := params[`paced`]; ok {
		paced = getBoolParam(params, `paced`)
	}
	now := util.GetRoundCount()
	if paced && !itemCooldownOpen(ctx, now) {
		return Failure
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	line := narration.Render(narration.Variants{Actor: lines}, nil, nil).Actor
	if spec == nil || line == `` {
		return Failure
	}

	var holderName, nameTag string
	var holderUser *users.UserRecord
	switch {
	case ctx.Item.UserId > 0:
		holderUser = users.GetByUserId(ctx.Item.UserId)
		if holderUser == nil || holderUser.Character == nil {
			return Failure
		}
		holderName, nameTag = holderUser.Character.Name, `username`
	case ctx.Item.MobInstanceId > 0:
		m := mobs.GetInstance(ctx.Item.MobInstanceId)
		if m == nil {
			return Failure
		}
		holderName, nameTag = m.Character.Name, `mobname`
	default:
		return Failure // a floor item has no holder to speak through
	}

	if holderUser != nil {
		holderUser.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"`, spec.Name, line))
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if getStringParam(params, `to`) != speakToHolder && room != nil {
		exclude := []int{}
		if holderUser != nil {
			exclude = append(exclude, holderUser.UserId)
		}
		room.SendTextHidingNames(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi>'s <ansi fg="item">%s</ansi> mutters, "<ansi fg="yellow">%s</ansi>"`,
			nameTag, holderName, spec.Name, line),
			[]string{holderName}, messaging.HideSpeakerNames, exclude...)
	}

	if paced {
		cooldown, _ := chatterPacing(voice.Chatter)
		ctx.MobState.Set(speakNextRoundKey, now+cooldown)
	}
	if ctx.Event.EventType == `item_idle` && room != nil {
		capRounds := uint64(configs.GetBalanceConfig().ItemChatterListenerCapRounds)
		listenerCapMu.Lock()
		for _, uid := range room.GetPlayers() {
			listenerCapNext[uid] = now + capRounds
		}
		listenerCapMu.Unlock()
	}
	return Success
}

// actTauntPull: the Aegis's tank loop (spec X15, ruling R5). A player
// holder's current foe, a mob fighting someone else, is made to fight the
// holder, through the taunt hold so the per-round re-aggro cannot flip it
// straight back. A mob holder, a holder at peace, a non-combatant foe or a
// foe already on the holder: nothing to do. Always Success, so a tree's
// taunt branch never falls through to another line.
func actTauntPull(_ map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() || ctx == nil || ctx.Item == nil || ctx.Item.UserId <= 0 {
		return Success
	}
	u := users.GetByUserId(ctx.Item.UserId)
	if u == nil || u.Character == nil {
		return Success
	}
	target := u.Character.CurrentCombatTarget()
	if target.MobInstanceId <= 0 {
		return Success
	}
	foe := mobs.GetInstance(target.MobInstanceId)
	if foe == nil || foe.IsNonCombatant() {
		return Success
	}
	if foe.Character.CurrentCombatTarget().UserId == u.UserId {
		return Success
	}
	holdRounds := int(configs.GetBalanceConfig().TauntHoldRounds)
	targeting.CommitTaunt(&foe.Character, state.ActorRef{UserId: u.UserId}, holdRounds)
	return Success
}
```

- [ ] **Step 5: Register them and allow them in item trees only**

In `internal/behaviortree/conditions.go`, replace

```go
	conditionRegistry["in_combat"] = condInCombat
}
```

with

```go
	conditionRegistry["in_combat"] = condInCombat
	// Item voices (item behaviour slice 2)
	conditionRegistry["chatter_ready"] = condChatterReady
	conditionRegistry["hunger_overdue"] = condHungerOverdue
}
```

In `internal/behaviortree/actions.go`, replace

```go
	actionRegistry["pulse_light"] = actPulseLight
}
```

with

```go
	actionRegistry["pulse_light"] = actPulseLight
	// Item voices (item behaviour slice 2)
	actionRegistry["speak"] = actSpeak
	actionRegistry["taunt_pull"] = actTauntPull
}
```

In `internal/behaviortree/loader.go`, replace

```go
		"worn":               true,
		"in_combat":          true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
		"set_light":       true,
		"pulse_light":     true,
	}
	// itemOnlyNodes need an item subject, so a mob or room tree may not
	// name them.
	itemOnlyNodes = map[string]bool{
		"holder_asleep": true,
		"worn":          true,
		"in_combat":     true,
		"set_light":     true,
		"pulse_light":   true,
	}
```

with

```go
		"worn":               true,
		"in_combat":          true,
		"chatter_ready":      true,
		"hunger_overdue":     true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
		"set_light":       true,
		"pulse_light":     true,
		"speak":           true,
		"taunt_pull":      true,
	}
	// itemOnlyNodes need an item subject, so a mob or room tree may not
	// name them.
	itemOnlyNodes = map[string]bool{
		"holder_asleep":  true,
		"worn":           true,
		"in_combat":      true,
		"set_light":      true,
		"pulse_light":    true,
		"chatter_ready":  true,
		"hunger_overdue": true,
		"speak":          true,
		"taunt_pull":     true,
	}
```

- [ ] **Step 6: A `speak` names a pool its tree has**

In `internal/behaviortree/item_voice.go`, replace

```go
// refuseItemVoice is the mob, room and archetype loaders' check: only an
// item tree has a voice.
```

with

```go
// checkSpeakNodes refuses a speak node that names a pool the tree's speech
// lacks (voice nil: no speech at all) or an audience other than all or
// holder, wherever the node sits.
func checkSpeakNodes(def NodeDef, voice *ItemVoice, path string) error {
	if def.Do == `speak` {
		pool := getStringParam(def.Params, `pool`)
		if voice == nil || len(voice.Speech[pool]) == 0 {
			return fmt.Errorf("%s: speak pool %q is not in the tree's speech", path, pool)
		}
		switch getStringParam(def.Params, `to`) {
		case ``, speakToAll, speakToHolder:
		default:
			return fmt.Errorf("%s: speak to %q: want %s or %s", path, getStringParam(def.Params, `to`), speakToAll, speakToHolder)
		}
	}
	for i, ch := range def.Children {
		if err := checkSpeakNodes(ch, voice, fmt.Sprintf("%s.%d", path, i)); err != nil {
			return err
		}
	}
	if def.Child != nil {
		return checkSpeakNodes(*def.Child, voice, path+".child")
	}
	return nil
}

// refuseItemVoice is the mob, room and archetype loaders' check: only an
// item tree has a voice.
```

In `internal/behaviortree/loader.go`, in `loadItemTreeDef` replace

```go
	voice, err := itemVoiceFrom(def)
	if err != nil {
		return nil, nil, err
	}
	node, err := compileNode(def.Tree, itemRootLabel)
```

with

```go
	voice, err := itemVoiceFrom(def)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSpeakNodes(def.Tree, voice, itemRootLabel); err != nil {
		return nil, nil, err
	}
	node, err := compileNode(def.Tree, itemRootLabel)
```

- [ ] **Step 7: Run the package**

Run: `gofmt -l internal/ && go test ./internal/behaviortree/ -count=1`
Expected: gofmt prints nothing; `ok`.

- [ ] **Step 8: Show the dark test fails against today's sender**

In `actions_item_voice.go`, temporarily replace the `room.SendTextHidingNames(...)` call (four lines) with today's sender:

```go
		room.SendTextVisual(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi>'s <ansi fg="item">%s</ansi> mutters, "<ansi fg="yellow">%s</ansi>"`,
			nameTag, holderName, spec.Name, line), exclude...)
```

Run: `go test ./internal/behaviortree/ -run TestSpeakHidesTheBearer -count=1`
Expected: FAIL, `a listener in the dark reads "", want the words with the bearer hidden as someone`. Restore the `SendTextHidingNames` call exactly as in Step 4 and re-run: `ok`.

- [ ] **Step 9: Run the touched packages and the root guards**

Run: `go test ./internal/behaviortree/ ./internal/hooks/ ./internal/characters/ -count=1 && go test . -count=1`
Expected: four `ok` (the root run takes about 20 s). If `TestPlayerConditionsTravelTheEventPath` names `item_procs.go`, the Step 3 key edit is missing.

- [ ] **Step 10: Commit**

```bash
git add internal/behaviortree/actions_item_voice.go internal/behaviortree/item_voice_nodes_test.go internal/behaviortree/item_voice.go internal/behaviortree/loader.go internal/behaviortree/conditions.go internal/behaviortree/actions.go internal/characters/character.go internal/hooks/item_procs.go condition_apply_path_guard_test.go
git commit -m "feat(behaviortree): item voice nodes: chatter_ready, speak, taunt_pull, hunger_overdue" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Fire the voice events: `on_equip`, `on_unequip`, `on_kill`, `on_hunger_feeding` (Rule 6, X18, S1, S3)

**Model:** sonnet.

**Files:**
- Create: `internal/hooks/EquipmentChange_ItemEvents.go`, `internal/hooks/item_voice_events_test.go`
- Modify: `internal/hooks/MobDeath_ItemProcs.go`, `internal/hooks/pinnacle_tick.go:3-22,181-185`, `internal/hooks/hooks.go:71`, `internal/behaviortree/events.go:24`, `condition_apply_path_guard_test.go:148-149`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/item_voice_events_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The item voice events (item behaviour slice 2, Rule 6, spec X18): equip
// and remove fire on_equip and on_unequip (closes #222), a kill fires
// on_kill into every worn treed item (ruling S3), and the hunger tick
// fires on_hunger_feeding into the weapon's tree (ruling S1).

const (
	eventsShieldId     = 99650
	eventsBladeId      = 99651
	eventsPlainBladeId = 99652
	eventsBearerId     = 81
	eventsListenerId   = 82
	eventsRoomId       = 9981
	eventsMobInstance  = 9982
)

const eventsProbeTree = `
speech:
  on_equip: ["Up we go."]
  on_unequip: ["Down we go."]
  on_kill: ["Another."]
  on_hunger_feeding: ["A sip."]
tree:
  type: selector
  children:
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_unequip
      do: speak
      pool: on_unequip
    - type: action
      event: on_kill
      do: speak
      pool: on_kill
    - type: action
      event: on_hunger_feeding
      do: speak
      pool: on_hunger_feeding
      to: holder
      paced: false
`

func seedItemEventsWorld(t *testing.T) (bearer *users.UserRecord, room *rooms.Room) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = true
	cfg.Balance.ItemChatterNormalCooldownRounds = 20
	cfg.Balance.HungerFeedingLineCooldownRounds = 20
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		eventsShieldId: {ItemId: eventsShieldId, Name: "Probe Shield", Type: items.Offhand, Behavior: "item_events_probe"},
		eventsBladeId: {ItemId: eventsBladeId, Name: "Probe Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01, Behavior: "item_events_probe"},
		eventsPlainBladeId: {ItemId: eventsPlainBladeId, Name: "Plain Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01},
	}))
	behaviortree.LoadItemTreeForTest(t, "item_events_probe", eventsProbeTree)
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())

	room = &rooms.Room{RoomId: eventsRoomId, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{eventsRoomId: room}, map[string]*rooms.ZoneConfig{}))

	bearer = users.NewTestUser(eventsBearerId, "kesh", "Kesh", 0)
	listener := users.NewTestUser(eventsListenerId, "clara", "Clara", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{eventsBearerId: bearer, eventsListenerId: listener}))
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = eventsRoomId
		room.AddPlayer(u.UserId)
	}
	bearer.Character.HealthMax.Value = 1000
	bearer.Character.Health = 1000

	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	_ = events.DrainQueuedMessagesForTest(eventsBearerId)
	_ = events.DrainQueuedMessagesForTest(eventsListenerId)
	return bearer, room
}

func eventsText(userId int) string {
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

// #222: wearing and removing a voiced item fires its equip and unequip
// lines, for a player and for a mob.
func TestEquipmentChangeFiresOnEquipAndOnUnequip(t *testing.T) {
	bearer, room := seedItemEventsWorld(t)
	shield := items.New(eventsShieldId)
	bearer.Character.Equipment.Offhand = shield

	ItemEquipEvents(events.EquipmentChange{UserId: eventsBearerId, ItemsWorn: []items.Item{shield}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Up we go.") {
		t.Errorf("on equip the bearer reads %q, want the equip line", got)
	}
	if got := eventsText(eventsListenerId); !strings.Contains(got, "mutters") {
		t.Errorf("on equip the room reads %q, want the item muttering", got)
	}

	util.SetRoundCountForTest(7020)
	bearer.Character.Equipment.Offhand = items.Item{}
	bearer.Character.StoreItem(shield)
	ItemEquipEvents(events.EquipmentChange{UserId: eventsBearerId, ItemsRemoved: []items.Item{shield}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Down we go.") {
		t.Errorf("on removal the bearer reads %q, want the unequip line", got)
	}

	ogre := &mobs.Mob{InstanceId: eventsMobInstance, HomeRoomId: eventsRoomId,
		Character: characters.Character{Name: "ogre", RoomId: eventsRoomId, Conditions: conditions.New()}}
	mobs.SetInstanceForTest(eventsMobInstance, ogre)
	t.Cleanup(func() { mobs.SetInstanceForTest(eventsMobInstance, nil) })
	room.AddMob(eventsMobInstance)
	mobShield := items.New(eventsShieldId)
	ogre.Character.Equipment.Offhand = mobShield
	ItemEquipEvents(events.EquipmentChange{MobInstanceId: eventsMobInstance, ItemsWorn: []items.Item{mobShield}})
	if got := eventsText(eventsListenerId); !strings.Contains(got, `Ogre</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters, "<ansi fg="yellow">Up we go.`) {
		t.Errorf("a mob's equip: the room reads %q, want the mob's item muttering", got)
	}
}

// Ruling S3: a kill reaches every worn treed item, not only the weapon,
// and the room hears it (ruling S2).
func TestKillFiresOnKillIntoEveryWornVoicedItem(t *testing.T) {
	bearer, _ := seedItemEventsWorld(t)
	bearer.Character.Equipment.Offhand = items.New(eventsShieldId)

	MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{eventsBearerId: 1}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Another.") {
		t.Errorf("the bearer reads %q, want the shield's kill line", got)
	}
	if got := eventsText(eventsListenerId); !strings.Contains(got, "Another.") {
		t.Errorf("the room reads %q, want the kill line muttered", got)
	}
}

// Ruling S1: the feeding line goes to the bearer alone, through the
// weapon's tree, paced only by HungerFeedingLineCooldownRounds; a hungry
// weapon with no tree sends the plain fallback.
func TestHungerFeedingSpeaksThroughTheWeaponsTree(t *testing.T) {
	bearer, _ := seedItemEventsWorld(t)
	c := bearer.Character
	c.Equipment.Weapon = items.New(eventsBladeId)
	c.SetMiscData("pinnacle_hunger_anchor", uint64(7000-60))

	tickHunger(c, bearer, 7000)
	if got := eventsText(eventsBearerId); !strings.Contains(got, "A sip.") {
		t.Errorf("the bearer reads %q, want the tree's feeding line", got)
	}
	if got := eventsText(eventsListenerId); got != "" {
		t.Errorf("the feeding line is the bearer's alone, the room read %q", got)
	}
	health := c.Health
	tickHunger(c, bearer, 7001)
	if got := eventsText(eventsBearerId); got != "" {
		t.Errorf("one round later the feeding line is paced out, got %q", got)
	}
	if c.Health >= health {
		t.Error("the drain itself runs every overdue round")
	}

	c.Equipment.Weapon = items.New(eventsPlainBladeId)
	c.SetMiscData("pinnacle_hunger_msg_next_round", nil)
	tickHunger(c, bearer, 7002)
	if got := eventsText(eventsBearerId); !strings.Contains(got, "The blade feeds on you") {
		t.Errorf("a hungry weapon with no tree: the bearer reads %q, want the fallback line", got)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go vet ./internal/hooks/`
Expected: `undefined: ItemEquipEvents`.

- [ ] **Step 3: The equip listener and the shared dispatch**

Create `internal/hooks/EquipmentChange_ItemEvents.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ItemEquipEvents fires on_equip into the tree of every treed item an
// equipment change put on, and on_unequip into every one it took off
// (item behaviour slice 2, spec X18, closes #222). Players and mobs alike:
// actions.equipItem and actions.removeWorn queue the one EquipmentChange
// both share, and internal/actions cannot reach a tree itself. An item put
// on is found in its slot by identity; an item taken off is in the
// backpack (or spilled to the floor), so it fires with no slot.
func ItemEquipEvents(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.EquipmentChange)
	if !ok || (len(evt.ItemsWorn) == 0 && len(evt.ItemsRemoved) == 0) {
		return events.Continue
	}
	c := equipmentChangeHolder(evt)
	if c == nil {
		return events.Continue
	}
	for _, it := range evt.ItemsWorn {
		if !it.HasBehavior() {
			continue
		}
		for _, s := range c.Equipment.AllSlots() {
			if s.Item.ItemId == it.ItemId && s.Item.UUID == it.UUID {
				fireItemEvent(behaviortree.EventContext{EventType: "on_equip"}, *s.Item, s.Key, c, evt.UserId, evt.MobInstanceId)
				break
			}
		}
	}
	for _, it := range evt.ItemsRemoved {
		if it.HasBehavior() {
			fireItemEvent(behaviortree.EventContext{EventType: "on_unequip"}, it, ``, c, evt.UserId, evt.MobInstanceId)
		}
	}
	return events.Continue
}

// equipmentChangeHolder is the character an equipment change happened to.
func equipmentChangeHolder(evt events.EquipmentChange) *characters.Character {
	if evt.UserId > 0 {
		if u := users.GetByUserId(evt.UserId); u != nil {
			return u.Character
		}
		return nil
	}
	if evt.MobInstanceId > 0 {
		if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
			return &m.Character
		}
	}
	return nil
}

// fireWornItemEvent fires an event into the tree of every treed item c
// wears, in slot order.
func fireWornItemEvent(event behaviortree.EventContext, c *characters.Character, userId, mobInstanceId int) {
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId > 0 && s.Item.HasBehavior() {
			fireItemEvent(event, *s.Item, s.Key, c, userId, mobInstanceId)
		}
	}
}

// fireItemEvent runs one item's tree for an event and reports whether the
// tree handled it.
func fireItemEvent(event behaviortree.EventContext, it items.Item, slot string, c *characters.Character, userId, mobInstanceId int) bool {
	return behaviortree.TryItemBehavior(event, behaviortree.ItemSubject{
		UUID: it.UUID, ItemId: it.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
		RoomId: c.RoomId, Slot: slot,
	})
}
```

Each call site spells its event as an `EventType: "..."` literal: `behaviortree/events_test.go` finds fired events that way (F6).

- [ ] **Step 4: The kill and the feeding line**

In `internal/hooks/MobDeath_ItemProcs.go`, replace

```go
import (
	"github.com/GoMudEngine/GoMud/internal/configs"
```

with

```go
import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

and replace

```go
		user.Character.SetMiscData("pinnacle_last_kill_round", util.GetRoundCount())
		dispatchItemProcs("on_kill", user.Character, nil, nil, 0)
```

with

```go
		user.Character.SetMiscData("pinnacle_last_kill_round", util.GetRoundCount())
		dispatchItemProcs("on_kill", user.Character, nil, nil, 0)

		// Every worn treed item hears of the kill (item behaviour slice 2,
		// ruling S3: the shield's kill lines too, not the weapon's alone).
		fireWornItemEvent(behaviortree.EventContext{EventType: "on_kill"}, user.Character, uid, 0)
```

(the old voice block below it stays until Task 7; no shipped item has a tree yet, so nothing speaks twice).

In `internal/hooks/pinnacle_tick.go`, replace

```go
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

with

```go
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

and replace

```go
		if next, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok || now >= next {
			emitVoiceLine(user, nil, spec, "on_hunger_feeding",
				`<ansi fg="red">The blade feeds on you — a cold pull beneath your grip.</ansi>`)
```

with

```go
		if next, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok || now >= next {
			// The weapon's tree speaks the line when it has one (item
			// behaviour slice 2); otherwise the plain fallback goes out.
			if !fireItemEvent(behaviortree.EventContext{EventType: "on_hunger_feeding"},
				c.Equipment.Weapon, "weapon", c, user.UserId, 0) {
				emitVoiceLine(user, nil, spec, "on_hunger_feeding",
					`<ansi fg="red">The blade feeds on you — a cold pull beneath your grip.</ansi>`)
			}
```

(the fallback line is unchanged here; Task 7 rewrites it). Those two edits move the bandolier lines `condition_apply_path_guard_test.go` names: replace the key `"internal/hooks/pinnacle_tick.go|335":` with `"internal/hooks/pinnacle_tick.go|341":` and `"internal/hooks/pinnacle_tick.go|349":` with `"internal/hooks/pinnacle_tick.go|355":`. Confirm with `grep -n "AddConditionScaled(id" internal/hooks/pinnacle_tick.go`: 341 and 355.

- [ ] **Step 5: Register the listener and the events**

In `internal/hooks/hooks.go`, replace

```go
	items.OnRoomHolderIndexed = EvaluateRoomFixtures
```

with

```go
	items.OnRoomHolderIndexed = EvaluateRoomFixtures
	// Item behaviour slice 2: equip and remove fire on_equip / on_unequip.
	events.RegisterListener(events.EquipmentChange{}, ItemEquipEvents)
```

In `internal/behaviortree/events.go`, replace

```go
	"mob_idle":               true, // idle tick (out of combat)
```

with

```go
	"mob_idle":               true, // idle tick (out of combat)
	"on_equip":               true, // item trees: the item was put on (item behaviour slice 2)
	"on_hunger_feeding":      true, // item trees: a hungry weapon fed on its bearer, paced by HungerFeedingLineCooldownRounds
	"on_kill":                true, // item trees: the bearer had a hand in a kill (every worn treed item)
	"on_unequip":             true, // item trees: the item was taken off
```

- [ ] **Step 6: Run**

Run: `gofmt -l internal/ && go test ./internal/hooks/ ./internal/behaviortree/ -count=1 && go test . -count=1`
Expected: gofmt prints nothing; three `ok`. `TestItemVoiceParity` still passes: the shipped items have no tree yet, so the feeding line takes the fallback branch, which is today's `emitVoiceLine`.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/EquipmentChange_ItemEvents.go internal/hooks/item_voice_events_test.go internal/hooks/MobDeath_ItemProcs.go internal/hooks/pinnacle_tick.go internal/hooks/hooks.go internal/behaviortree/events.go condition_apply_path_guard_test.go
git commit -m "feat(hooks): fire on_equip, on_unequip, on_kill and on_hunger_feeding into item trees" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: The switch-over: the Blackrazor and the Aegis become trees (Rules 18, 19; R5; S1 to S3)

**Model:** opus.

**Files:**
- Create: `_datafiles/world/dogmud/behaviors/items/blackrazor.yaml`, `_datafiles/world/dogmud/behaviors/items/aegis.yaml`
- Modify: `_datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml:37`, `40185-aegis_of_mockery.yaml:26`; `internal/hooks/item_voice_parity_test.go` (rewritten); `internal/hooks/pinnacle_tick.go`; `internal/hooks/MobDeath_ItemProcs.go`; `internal/hooks/pinnacle_tick_test.go:278-492`; `condition_apply_path_guard_test.go:148-149`; `messaging_surface_guard_test.go:1296`; `narration_render_callers_guard_test.go:19`; `item_behaviour_guard_test.go:95-113,168-170`; `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md` (append)

- [ ] **Step 1: Point the parity test at the tree path (it fails until the trees exist)**

Replace the whole of `internal/hooks/item_voice_parity_test.go` with:

```go
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/itemvoices"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The voice parity record (item behaviour slice 2, spec "Slice 2" gates).
// testdata/item_voice_parity.golden was recorded on the Pinnacle tick's
// voice path (tickVoices, tryEmitVoice, emitVoiceLine) BEFORE the voices
// moved into item trees, and is frozen: the tree path must reproduce it,
// the same lines on the same rounds to the bearer, under one seeded random
// source (util.SetRandForTest) that the chance roll and the line pick both
// draw from. Two scenarios, one per shipped voiced item, each 200 rounds:
// idle, a hunger warning (the Blackrazor only), combat taunts, hungry
// feeding, a kill, idle again.
//
// Only the bearer's own lines are recorded: the room line changed on
// purpose (spec X17, it is heard and hides the bearer's name).

const (
	voiceParityUserId   = 1
	voiceParityRoomId   = 9971
	voiceParityFirst    = 1000 // the first scenario round
	voiceParityRounds   = 200
	voiceParityFightAt  = 60  // combat from this round offset
	voiceParityFightEnd = 110 // to this one
	voiceParityKillAt   = 165 // the kill: both items' chatter cooldowns are open here under the seed
	voiceParitySeed     = 5150
)

// voiceParityScenarios are the shipped voiced items, the slot each is worn
// in, and the round offset from which the tree path may differ from the
// record on purpose (-1: never). The Aegis's kill lines were dead on the
// Pinnacle path, which voiced only the weapon's; owner ruling S3 gives every
// worn voiced item the kill, so from the kill on the Aegis speaks and draws
// where the record has nothing.
var voiceParityScenarios = []struct {
	name     string
	itemId   int
	slot     string
	divertAt int
}{
	{"blackrazor", 40183, "weapon", -1},
	{"aegis", 40185, "offhand", voiceParityKillAt},
}

// loadVoiceParityWorld loads the shipped conditions and items once for the
// test, points the engine at the shipped item trees, and seeds one room.
func loadVoiceParityWorld(t *testing.T) *rooms.Room {
	t.Helper()
	// The overlay first: AddOverlayOverrides rebuilds the live config, so
	// it would drop a data path set before it.
	setPinnacleEnabled(t, true)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them too, or later tests in the package read the
	// shipped pools instead of their seeds.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for _, name := range []string{"blackrazor", "aegis"} {
		behaviortree.GetEngine().EvictItemTree(name)
		t.Cleanup(func() { behaviortree.GetEngine().EvictItemTree(name) })
	}

	room := rooms.NewRoom("voiceparity")
	room.RoomId = voiceParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// newParityBearer seeds a fresh user 1 in room, wearing a fresh instance of
// itemId in slot, with fresh item state and listener caps.
func newParityBearer(t *testing.T, room *rooms.Room, itemId int, slot string) *users.UserRecord {
	t.Helper()
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())
	u := users.NewTestUser(voiceParityUserId, "bearer", "Bearer", 0)
	u.Character.RoomId = room.RoomId
	u.Character.HealthMax.Value = 100000 // the hunger drain never reaches its floor
	u.Character.Health = 100000
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{voiceParityUserId: u}))
	room.AddPlayer(voiceParityUserId)

	it := items.New(itemId)
	switch slot {
	case "weapon":
		u.Character.Equipment.Weapon = it
	case "offhand":
		u.Character.Equipment.Offhand = it
	default:
		t.Fatalf("newParityBearer: unknown slot %q", slot)
	}
	return u
}

// voiceParityRound runs one round of the voice machinery for the bearer in
// the server's order: the Pinnacle tick (hunger and its feeding line), the
// item tick's visit to the bearer (ambient lines), then a kill when kill
// is set.
func voiceParityRound(u *users.UserRecord, room *rooms.Room, kill bool) {
	pinnacleUserTick(u, room)
	tickHeldItems(u.Character, u.UserId, 0)
	if kill {
		MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{u.UserId: 1}})
	}
}

// runVoiceParity plays one scenario and returns its record: one line per
// round that sent the bearer anything.
func runVoiceParity(t *testing.T, room *rooms.Room, itemId int, slot string) string {
	t.Helper()
	u := newParityBearer(t, room, itemId, slot)
	c := u.Character

	restoreRand := util.SetRandForTest(voiceParitySeed)
	defer restoreRand()
	defer util.ResetRoundCountForTest()
	_ = events.DrainQueuedMessagesForTest(u.UserId)

	var b strings.Builder
	for i := 0; i < voiceParityRounds; i++ {
		util.SetRoundCountForTest(uint64(voiceParityFirst + i))
		switch i {
		case voiceParityFightAt:
			c.SetAggro(0, 424242, characters.DefaultAttack)
		case voiceParityFightEnd:
			c.EndAggro()
		}
		voiceParityRound(u, room, i == voiceParityKillAt)
		msgs := events.DrainQueuedMessagesForTest(u.UserId)
		for j := range msgs {
			msgs[j] = strings.TrimRight(msgs[j], "\n")
		}
		if len(msgs) > 0 {
			fmt.Fprintf(&b, "%d: %s\n", i, strings.Join(msgs, " | "))
		}
	}
	return b.String()
}

// voiceRecordSections splits the record into its scenarios' lines.
func voiceRecordSections(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "item_voice_parity.golden"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	name := ``
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "## ") {
			name = strings.Fields(line)[1]
			continue
		}
		out[name] = append(out[name], line)
	}
	return out
}

// recordLinesBefore keeps the record lines whose round offset is below
// cutoff (all of them when cutoff is negative).
func recordLinesBefore(lines []string, cutoff int) []string {
	if cutoff < 0 {
		return lines
	}
	var kept []string
	for _, line := range lines {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		round, err := strconv.Atoi(line[:colon])
		if err == nil && round < cutoff {
			kept = append(kept, line)
		}
	}
	return kept
}

func TestItemVoiceParity(t *testing.T) {
	room := loadVoiceParityWorld(t)
	record := voiceRecordSections(t)
	for _, s := range voiceParityScenarios {
		got := strings.Split(strings.TrimRight(runVoiceParity(t, room, s.itemId, s.slot), "\n"), "\n")
		want := record[s.name]
		if len(want) == 0 {
			t.Fatalf("the record has no %s scenario", s.name)
		}
		if g, w := recordLinesBefore(got, s.divertAt), recordLinesBefore(want, s.divertAt); !reflect.DeepEqual(g, w) {
			t.Errorf("%s: the tree path moved off the record.\n--- want\n%s\n--- got\n%s",
				s.name, strings.Join(w, "\n"), strings.Join(g, "\n"))
		}
		if s.divertAt >= 0 {
			// Ruling S3: where the record is silent, the shield now
			// speaks one of its kill lines.
			prefix := strconv.Itoa(s.divertAt) + ": "
			found := false
			for _, line := range got {
				if strings.HasPrefix(line, prefix) && strings.Contains(line, "Aegis of Mockery</ansi> says") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no kill line on the kill round %d", s.name, s.divertAt)
			}
		}
	}
}

// The tree pools carry the voice files' lines byte for byte (spec Rule
// 18), checked while both exist. The retire task deletes this test with
// internal/itemvoices.
func TestItemTreePoolsMatchTheVoiceFiles(t *testing.T) {
	loadVoiceParityWorld(t)
	t.Cleanup(itemvoices.SeedVoicesForTest(nil))
	itemvoices.LoadDataFiles()
	for _, id := range itemvoices.AllVoiceIds() {
		if err := behaviortree.GetEngine().LoadItemTree(id, behaviortree.GetItemTreePath(id)); err != nil {
			t.Fatalf("item tree %s: %v", id, err)
		}
		tree := behaviortree.GetEngine().GetItemVoice(id)
		if tree == nil {
			t.Fatalf("item tree %s has no speech", id)
		}
		if !reflect.DeepEqual(tree.Speech, itemvoices.GetVoice(id).Lines) {
			t.Errorf("item tree %s's pools differ from itemvoices/%s.yaml:\n tree:  %q\n voice: %q",
				id, id, tree.Speech, itemvoices.GetVoice(id).Lines)
		}
	}
}
```

The record is frozen from here on: the `-update-voices` flag is gone, so the tree path can only be held to it.

Run: `go test ./internal/hooks/ -run 'TestItemVoiceParity|TestItemTreePools' -count=1`
Expected: FAIL. The new loader no longer loads `itemvoices` and no item names a tree yet, so nothing speaks: `blackrazor: the tree path moved off the record` with an empty `got`, `aegis` likewise, and `TestItemTreePoolsMatchTheVoiceFiles` fails `item tree aegis: open ...behaviors\items\aegis.yaml`.

- [ ] **Step 2: The two trees**

Create `_datafiles/world/dogmud/behaviors/items/blackrazor.yaml`:

```yaml
# blackrazor: the Blackrazor's voice (40183; item behaviour slice 2, spec
# Rule 18). Ambient lines while worn: a taunt in combat, a hunger warning
# past three quarters of its hunger window, else idle chatter, paced by
# chatter_ready (the normal level's cooldown and chance, and the listener
# cap). Event lines on equip, removal and a kill need only the cooldown.
# The feeding line goes to the bearer alone, paced by the hunger tick
# (HungerFeedingLineCooldownRounds), not by the cooldown (owner ruling S1).
notes: Ancient, vain, starving aristocrat.
chatter: normal
speech:
  on_equip:
    - "Ahhhh. A hand again. Do keep it moving."
    - "So. They send me another footman. Grip tighter, boy."
    - "Warm palms. How thoughtful. I do so hate the cold."
    - "Lift me properly. I was a king's blade before you were a rumor."
    - "You'll do, I suppose. The last one did not do for long."
    - "Mind the edge. I am told I bite the hand that neglects me."
  on_unequip:
    - "Setting me down already? You wound me more than I wound you."
    - "Go on, then. Cold and idle. See how you fare without me."
    - "You will come back. They always come back, hungrier than I."
    - "Ungrateful. I have kept better company in shallower graves."
    - "Leave me if you like. I have all the patience of stone."
  on_kill:
    - "Yes... YES. Another."
    - "It drinks well tonight."
    - "Ahh. Still warm. My favorite vintage."
    - "There. Was that so hard? You are learning to serve."
    - "Good. Good. I could almost grow fond of you."
    - "Another for the tally. Do not stop on my account."
  on_idle:
    - "Mm. Hmm. Dull. Everything here is so dreadfully dull."
    - "I have cut through walls more interesting than this room."
    - "Are we waiting for something? I do detest waiting."
    - "Hmmmm... hmmm... such a small little life you lead."
    - "I remember banquets. I remember screaming. This is neither."
    - "Do something. Anything. I rust with boredom, not with air."
  on_hunger_warning:
    - "I hunger, bearer."
    - "It has been too long. Feed me. I ask you as a courtesy."
    - "My patience is a fine thing, and it is nearly spent."
    - "That warmth in your hands? That is me, asking politely."
    - "I begin to consider the nearest throat. Yours is nearest."
    - "Feed me soon, or I shall make do with the hand that holds me."
  on_hunger_feeding:
    - "You left me no choice. I take only a little. This time."
    - "Hold still. It is easier for us both if you hold still."
    - "There. A sip. I did warn you, and warning is more than most get."
    - "Nothing personal, bearer. A blade must eat. Even from the hand."
    - "I would rather it were someone else's. But you were here."
  on_taunt:
    - "Yes! Closer! Let me taste what it is so proud of."
    - "Swing, you sluggard! There is a living thing in reach!"
    - "Oh, I like this one. Do let me finish it slowly."
    - "More! Harder! I did not wake for a fair fight."
    - "Open it up. I want to see the color it keeps inside."
    - "Don't you dare miss. I am so very, very hungry."
tree:
  type: selector
  children:
    - type: sequence
      event: item_idle
      children:
        - type: condition
          check: worn
        - type: condition
          check: chatter_ready
        - type: selector
          children:
            - type: sequence
              children:
                - type: condition
                  check: in_combat
                - type: action
                  do: speak
                  pool: on_taunt
            - type: sequence
              children:
                - type: condition
                  check: hunger_overdue
                  fraction: 0.75
                - type: action
                  do: speak
                  pool: on_hunger_warning
            - type: action
              do: speak
              pool: on_idle
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_unequip
      do: speak
      pool: on_unequip
    - type: action
      event: on_kill
      do: speak
      pool: on_kill
    - type: action
      event: on_hunger_feeding
      do: speak
      pool: on_hunger_feeding
      to: holder
      paced: false
```

Create `_datafiles/world/dogmud/behaviors/items/aegis.yaml`:

```yaml
# aegis: the Aegis of Mockery's voice (40185; item behaviour slice 2, spec
# Rule 18). Ambient lines while worn: a taunt in combat, which also pulls
# the bearer's foe onto the bearer (taunt_pull, owner ruling R5), else idle
# heckling, paced by chatter_ready (the normal level's cooldown and chance,
# and the listener cap). Event lines on equip, removal and a kill need only
# the cooldown; a kill reaches the shield as well as the weapon (owner
# ruling S3).
notes: Period insult-comic.
chatter: normal
speech:
  on_equip:
    - "Ah, a proper arm at last! Yes, flex it, let them see. Grand."
    - "There you are, hero. Now stand still and let me do the talking."
    - "Up we go! Together we are unbearable, and I mean that kindly."
    - "Good hand, good shoulder, splendid glower. We shall be a menace."
    - "At last, someone with a face worth hiding behind. Onward!"
  on_unequip:
    - "Off already? But I was just warming to my material!"
    - "Fine, set me down. Deprive the world of its finest heckler."
    - "You wound me, sir. And I so rarely take a hit sitting down."
    - "Go on, abandon me. I shall insult the wall until you return."
    - "Retreat, is it? I shall pretend it was my idea, and loudly."
  on_kill:
    - "Ha! Down he goes, and it was my finest line that did it!"
    - "Note that, everyone: I talked him into the ground. Applause."
    - "One fewer critic. A shame, he was warming to my act."
    - "You struck the blow, I struck the mood. We split the credit."
    - "Timber! Another one who could not take a joke, poor soul."
  on_idle:
    - "That fellow yonder walks like a duck apologizing to a puddle."
    - "Do you see this one's stance? A scarecrow would be ashamed."
    - "I could do things with that man's nose. Given a moment. And time."
    - "Ahem. Testing. Is this thing pointed at anyone I may abuse?"
    - "Dull crowd. Not one of them worth a good insult. Tragic."
  on_taunt:
    - "Your mother smells of elderberries; your footwork shames her further!"
    - "Call that a guard? My bearer's grandmother threw a stouter one!"
    - "You swing like a man beating dust from a very old rug, sir!"
    - "I have met chair legs with a keener sense of where to stand!"
    - "Your father was a turnip and your posture confirms the union!"
    - "Come closer, coward, so the whole field may see you tremble!"
    - "You reek of defeat and, I regret to add, of yesterday's fish!"
    - "That footwork! Do you fight me or apologize to the ground?"
    - "A whole clan of you, and not one taught the boy to duck!"
tree:
  type: selector
  children:
    - type: sequence
      event: item_idle
      children:
        - type: condition
          check: worn
        - type: condition
          check: chatter_ready
        - type: selector
          children:
            - type: sequence
              children:
                - type: condition
                  check: in_combat
                - type: action
                  do: speak
                  pool: on_taunt
                - type: action
                  do: taunt_pull
            - type: action
              do: speak
              pool: on_idle
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_unequip
      do: speak
      pool: on_unequip
    - type: action
      event: on_kill
      do: speak
      pool: on_kill
```

The pool lines are the `itemvoices/*.yaml` lines verbatim, semicolon included (existing authored lines; the no-semicolon rule is for new speech). `TestItemTreePoolsMatchTheVoiceFiles` proves it.

- [ ] **Step 3: The items name their trees**

In `_datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml`, replace `voice_id: blackrazor` with the two lines `voice_id: blackrazor` and `behavior: blackrazor`. In `40185-aegis_of_mockery.yaml`, replace the two lines `voice_id: aegis` / `taunt_pull: true` with those two lines followed by `behavior: aegis`. (`voice_id` and `taunt_pull` go in Task 8 with their fields.)

- [ ] **Step 4: The Pinnacle tick stops speaking**

In `internal/hooks/pinnacle_tick.go`:

Replace

```go
	tickMutationItems(user, worn, now)
	tickVoices(user, room, worn, now)
}
```

with

```go
	tickMutationItems(user, worn, now)
	// Sentient voices are item trees since item behaviour slice 2: the item
	// tick (ItemRoundTick) speaks their ambient lines.
}
```

Replace

```go
				emitVoiceLine(user, nil, spec, "on_hunger_feeding",
					`<ansi fg="red">The blade feeds on you — a cold pull beneath your grip.</ansi>`)
```

with

```go
				user.SendText(messaging.CategorySystem,
					`<ansi fg="red">The blade feeds on you, a cold pull beneath your grip.</ansi>`)
```

(the line is rewritten, so it loses its dash: "Where the spec could not be implemented" item 11).

Delete everything from the blank line before `// ── Sentient chatter (Blackrazor / Aegis voices) ──` to the end of the file: `pickVoiceEvent`, `tickVoices`, `applyTauntPull`, `tryEmitVoice` and `emitVoiceLine` (about 145 lines; the file then ends with `revokeAmbient`'s closing brace). Remove the now-unused imports `internal/itemvoices`, `internal/mobs` and `internal/targeting` (`go build ./internal/hooks/` names them).

- [ ] **Step 5: The kill block goes**

In `internal/hooks/MobDeath_ItemProcs.go`, replace

```go
		fireWornItemEvent(behaviortree.EventContext{EventType: "on_kill"}, user.Character, uid, 0)

		// Sentient weapons savor the kill (paced by the shared chatter cooldown,
		// so this doesn't spam on multi-kill rounds). Sentient chatter is the
		// PinnacleItemsEnabled toggle's domain — same gate pinnacleUserTick reads.
		if bool(configs.GetConfig().GamePlay.PinnacleItemsEnabled) &&
			user.Character.Equipment.Weapon.ItemId > 0 {
			if wspec := user.Character.Equipment.Weapon.GetSpec(); wspec.VoiceId != "" {
				tryEmitVoice(user, nil, wspec, "on_kill")
			}
		}
	}
```

with

```go
		// Its speak node is paced by the item's cooldown, so a multi-kill
		// round does not spam, and gated by PinnacleItemsEnabled.
		fireWornItemEvent(behaviortree.EventContext{EventType: "on_kill"}, user.Character, uid, 0)
	}
```

remove the `internal/configs` import, and replace the function comment

```go
// MobDeathItemProcs fires on_kill procs and records the last-kill round
// (the Blackrazor hunger anchor — Task 11) for every player with damage
// attribution on the kill.
```

with

```go
// MobDeathItemProcs fires on_kill procs, records the last-kill round (the
// Blackrazor hunger anchor, Task 11) and fires on_kill into every worn
// treed item, for every player with damage attribution on the kill.
```

- [ ] **Step 6: The old voice tests go (each has a replacement)**

In `internal/hooks/pinnacle_tick_test.go`, delete from `// ─── Voices ───` through the end of `TestMobDeathItemProcs_OnKillVoice` (five tests: `TestPinnaclePickVoiceEvent`, `TestPinnacleVoiceCooldownGates`, `TestPinnacleEmitVoiceLine`, `TestApplyTauntPull`, `TestMobDeathItemProcs_OnKillVoice`), keeping `containsLine` (`pinnacle_ambient_smart_test.go` uses it), and put in their place:

```go
// Voices moved into item trees (item behaviour slice 2): the event pick,
// pacing, sender, taunt pull and kill line are tested in
// internal/behaviortree/item_voice_nodes_test.go and
// item_voice_events_test.go, and the old path is pinned by
// item_voice_parity_test.go.
```

Remove the `internal/itemvoices` import. Replacements: `TestHungerOverdue` and the trees (event pick), `TestSpeakPacing` (cooldown), `TestSpeakSendsTheHolderAndTheRoom` / `TestSpeakToTheHolderUnpaced` (the sender), `TestTauntPull` (all four of the old pull cases), `TestKillFiresOnKillIntoEveryWornVoicedItem` and `TestVoiceNodesHonourThePinnacleSwitch` (the kill line and its switch).

- [ ] **Step 7: Run the parity record**

Run: `go vet ./internal/hooks/ && go test ./internal/hooks/ ./internal/behaviortree/ -count=1`
Expected: both `ok`. `TestItemVoiceParity` holds the Blackrazor to all 200 recorded rounds and the Aegis up to round 165, where it now speaks a kill line; `TestItemTreePoolsMatchTheVoiceFiles` passes. If the Blackrazor moves, do NOT re-record: the tree or the node order differs from this plan.

- [ ] **Step 8: The root guards**

Run: `go test . -count=1`
Expected: FAIL in four guards. Fix each:

1. `TestPlayerConditionsTravelTheEventPath`: in `condition_apply_path_guard_test.go` replace the key `"internal/hooks/pinnacle_tick.go|341":` with `"internal/hooks/pinnacle_tick.go|339":` and `"internal/hooks/pinnacle_tick.go|355":` with `"internal/hooks/pinnacle_tick.go|353":` (the three removed imports). Confirm with `grep -n "AddConditionScaled(id" internal/hooks/pinnacle_tick.go`: 339 and 353.
2. `TestNarrationSitesMatchViewpointAudit` reports the registry entry `hooks/pinnacle_tick.go|<ansi fg="item">%s</ansi> says, ...` stale (the sender moved). The guard asks for the audit doc first. Append to `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md`, after the last table row of "Retirement from the M4e-1 darkness fixes, 2026-09-21":

```markdown

### Retirement from item behaviour slice 2, 2026-10-06

| Retired entry | Why |
|---|---|
| `hooks/pinnacle_tick.go` `<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"` (verdictCorrect, actor+observer) | Sentient item chatter moved into item trees: `emitVoiceLine` is deleted and the line is sent by the `speak` node (`behaviortree/actions_item_voice.go`), which tells the bearer and sends the room line through `SendTextHidingNames`, heard by everyone with the bearer's name hidden by sight. The verdict holds: an item's speech has no actee. The row for `hooks/pinnacle_tick.go:527` above is the record of the site as audited. |
```

Then delete the line in `messaging_surface_guard_test.go` that starts `"hooks/pinnacle_tick.go|<ansi fg=\"item\">%s</ansi> says,`.
3. `TestNarrationRenderIsCalledOnlyByRegisteredStores` names `internal/behaviortree/actions_item_voice.go`. In `narration_render_callers_guard_test.go`, add as the first entry of `narrationRenderCallers`:

```go
	"internal/behaviortree/actions_item_voice.go": "Kind A: item tree speech pools (item behaviour slice 2), single role (the item speaks); the default picker is deliberate, so a seeded util.Rand replays it",
```

then run `gofmt -w narration_render_callers_guard_test.go` (the longer key realigns the map).
4. `TestHooksReadItemBehaviourFieldsOnlyAtAllowlistedSites` fails on its sanity probe (`the scan found no VoiceId read`). In `item_behaviour_guard_test.go`, replace

```go
// reads a behaviour field: "file|field". These are today's Pinnacle
// mechanics. Slice 2 retires the VoiceId and TauntPull sites, slice 3 the
// Procs ones; an entry nothing reads any more fails, so the list only
// shrinks. A new site fails: put the behaviour in a tree.
var itemSpecBehaviourReadSites = map[string]bool{
	"MobDeath_ItemProcs.go|VoiceId":               true,
	"PlayerSpawn_HandleJoin.go|PreservesContents": true,
```

with

```go
// reads a behaviour field: "file|field". These are today's Pinnacle
// mechanics. Slice 2 retired the VoiceId and TauntPull sites (voices are
// item trees), slice 3 retires the Procs ones; an entry nothing reads any
// more fails, so the list only shrinks. A new site fails: put the
// behaviour in a tree.
var itemSpecBehaviourReadSites = map[string]bool{
	"PlayerSpawn_HandleJoin.go|PreservesContents": true,
```

replace

```go
	"pinnacle_tick.go|PreservesContents":          true,
	"pinnacle_tick.go|TauntPull":                  true,
	"pinnacle_tick.go|VoiceId":                    true,
}
```

with

```go
	"pinnacle_tick.go|PreservesContents":          true,
}
```

and replace

```go
	if len(reads["VoiceId"]) == 0 {
		t.Fatalf("the scan found no VoiceId read: it is not seeing internal/hooks (got %v)", reads)
	}
```

with

```go
	if len(reads["HungerRounds"]) == 0 {
		t.Fatalf("the scan found no HungerRounds read: it is not seeing internal/hooks (got %v)", reads)
	}
```

Run: `gofmt -l . internal modules && go test . -count=1`
Expected: gofmt prints nothing; `ok`.

- [ ] **Step 9: The whole suite**

Run: `go build ./... && go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"`
Expected: no output.

- [ ] **Step 10: Commit**

```bash
git add _datafiles/world/dogmud/behaviors/items/blackrazor.yaml _datafiles/world/dogmud/behaviors/items/aegis.yaml _datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml _datafiles/world/dogmud/items/materials-40000/40185-aegis_of_mockery.yaml internal/hooks/item_voice_parity_test.go internal/hooks/pinnacle_tick.go internal/hooks/pinnacle_tick_test.go internal/hooks/MobDeath_ItemProcs.go condition_apply_path_guard_test.go messaging_surface_guard_test.go narration_render_callers_guard_test.go item_behaviour_guard_test.go docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md
git commit -m "feat: the Blackrazor and the Aegis speak through item trees" -m "The Pinnacle voice sub-tick is gone; the frozen record proves the tree path matches it (the Aegis up to the kill it now voices, ruling S3)." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Retire the old voice surface (Rule 19, X21)

**Model:** sonnet.

**Files:**
- Delete: `internal/itemvoices/` (4 files), `_datafiles/world/dogmud/itemvoices/` (3 files), `internal/narration/testdata/stores/itemvoices.golden`
- Modify: `main.go:49,1670-1675`; `internal/items/itemspec.go:294,297`; 40183, 40185; `modules/gmcp/gmcp.Item.go`, `gmcp.Item_test.go`; `_datafiles/html/public/static/js/items.js`; `internal/configs/config.balance.go`, `config.balance.misc.go`, `configs_pinnacle_test.go`, `_datafiles/config.yaml`; `internal/hooks/item_voice_parity_test.go`, `pinnacle_tick_test.go:105`; `internal/narration/snapshot_test.go`; `item_behaviour_guard_test.go`, `messaging_surface_guard_test.go`, `narration_render_callers_guard_test.go`, `shipped_narration_data_guard_test.go`; comments in `internal/combat/taunt_messages.go`, `internal/narration/picker.go`, `render.go`, `context.md`, `modules/weather/content/arch_test.go`, `internal/behaviortree/item_engine.go`

- [ ] **Step 1: Delete the package, its data and its golden**

```bash
git rm -q -r internal/itemvoices _datafiles/world/dogmud/itemvoices internal/narration/testdata/stores/itemvoices.golden
```

- [ ] **Step 2: main.go**

Replace

```go
	baubles.LoadCorpus()
	// Pinnacle Stage 1: sentient item voices. Must load AFTER items so the
	// voice_id cross-validation can see every item's ItemSpec.
	itemvoices.LoadDataFiles()
	// Lighting 5e (item behaviour slice 1): every item's behavior: must name
	// a tree under behaviors/items/ that loads and compiles, or the boot fails
	// here rather than leave the item silently inert (spec X12).
```

with

```go
	baubles.LoadCorpus()
	// Lighting 5e (item behaviour slice 1): every item's behavior: must name
	// a tree under behaviors/items/ that loads and compiles, or the boot fails
	// here rather than leave the item silently inert (spec X12). A sentient
	// item's voice lives in its tree since slice 2.
```

and remove the `internal/itemvoices` import.

- [ ] **Step 3: The fields and their keys (keys first: the boot's strict probe refuses unknown keys, I9)**

In 40183 delete the line `voice_id: blackrazor`; in 40185 delete `voice_id: aegis` and `taunt_pull: true`. Each file keeps its `behavior:` line. In `internal/items/itemspec.go` delete the two field lines

```go
	VoiceId               string     `yaml:"voice_id,omitempty"`                // sentient item voice file id (itemvoices/)
```

```go
	TauntPull             bool       `yaml:"taunt_pull,omitempty"`              // sentient chatter on_taunt also pulls the bearer's target's aggro (Aegis)
```

- [ ] **Step 4: The web builder (X21)**

In `modules/gmcp/gmcp.Item.go`: delete the fields `VoiceId string \`json:"voiceId"\`` and `TauntPull bool \`json:"tauntPull"\`` from `itemUpdateReq`, the field `Voices []string \`json:"voices"\`` from `itemDetail`, the lines `req.VoiceId, req.TauntPull = s.VoiceId, s.TauntPull` and `s.VoiceId, s.TauntPull = req.VoiceId, req.TauntPull`, the function `itemVoiceIds`, `, Voices: itemVoiceIds()` from `buildItemGet`, and the `internal/itemvoices` import. Then run `gofmt -w modules/gmcp/gmcp.Item.go` (two struct blocks realign; deleting a field with an edit that ends in a space misaligns them, which is how the dry run met it).

In `modules/gmcp/gmcp.Item_test.go`: remove the `internal/itemvoices` import and the two lines seeding voices at the top of `TestBuildItemGet_ShipsAdvancedEnums`, and its `d.Voices` check:

```go
	if len(d.Voices) == 0 || d.Voices[0] != "blackrazor" {
		t.Errorf("detail must ship the voice list, got %v", d.Voices)
	}
```

In `TestBuildItemUpdate_RoundTripsAdvancedFields`, give the loaded spec a tree and drop the voice fields: replace

```go
	w.specs[10001] = &items.ItemSpec{ItemId: 10001, Name: "Old", Type: items.Weapon, Hands: 2}
```

with

```go
	w.specs[10001] = &items.ItemSpec{ItemId: 10001, Name: "Old", Type: items.Weapon, Hands: 2, Behavior: "blackrazor"}
```

replace `ReserveHealthPct: 0.25, VoiceId: "blackrazor", TauntPull: true,` with `ReserveHealthPct: 0.25,` (gofmt realigns the next line), and replace

```go
	if got.ReserveHealthPct != 0.25 || got.VoiceId != "blackrazor" || !got.TauntPull ||
		got.HungerRounds != 50
```

with

```go
	// The form has no tree field (editing trees is #367), so a save keeps
	// the item's behavior: from the loaded spec.
	if got.Behavior != "blackrazor" {
		t.Errorf("a save dropped the item's behavior tree: got %q", got.Behavior)
	}
	if got.ReserveHealthPct != 0.25 ||
		got.HungerRounds != 50
```

In `_datafiles/html/public/static/js/items.js`: in `buildAdvancedSection` drop `|| detail.voiceId` and `|| detail.tauntPull` from `hasAdv` (keep the line breaks valid), replace the three lines

```js
    body.appendChild(sectionTitle("Sentient"));
    body.appendChild(selectField("Voice", "voiceId", detail.voiceId, [""].concat(detail.voices || [])));
    body.appendChild(ce("div", { "class": "flags" }, [checkField("taunt-pull", "tauntPull", detail.tauntPull)]));
```

with

```js
    // A sentient item's voice and taunt pull live in its behaviour tree
    // (behaviors/items/) since item behaviour slice 2; tree editing is #367.
```

delete the payload line `voiceId: g("voiceId", ""), tauntPull: g("tauntPull", false),`, and in the header comment replace `(procs, sentient/voice, hunger)` with `(procs, hunger, the behaviour tree)`. Check: `node --check _datafiles/html/public/static/js/items.js` prints nothing and `grep -n "voice\|taunt" _datafiles/html/public/static/js/items.js` finds only the new comment.

- [ ] **Step 5: The retired knobs**

In `internal/configs/config.balance.go` replace

```go
	BandolierAttuneRounds         ConfigInt `yaml:"BandolierAttuneRounds"`         // Rounds of re-attunement after bandolier contents change (default 100)
	SentientChatterCooldownRounds ConfigInt `yaml:"SentientChatterCooldownRounds"` // Min rounds between sentient item lines (default 20)
	SentientChatterChancePct      ConfigInt `yaml:"SentientChatterChancePct"`      // Percent chance per eligible round that a sentient item speaks (default 15)
```

with

```go
	BandolierAttuneRounds ConfigInt `yaml:"BandolierAttuneRounds"` // Rounds of re-attunement after bandolier contents change (default 100)
```

In `config.balance.misc.go` delete the two `SentientChatter*` checks (six lines) above `// Item tree chatter`. In `_datafiles/config.yaml` delete the lines `SentientChatterCooldownRounds: 20 ...` and `SentientChatterChancePct: 15 ...`. In `configs_pinnacle_test.go` delete the `SentientChatterCooldownRounds` check (three lines). In `internal/hooks/pinnacle_tick_test.go` replace `// (SentientChatterCooldownRounds): two consecutive overdue ticks both drain,` with `// (HungerFeedingLineCooldownRounds): two consecutive overdue ticks both drain,`. `pinnacle_voice_next_round` stays inert in old saves (the Task 4 precedent).

- [ ] **Step 6: The one-shot pool proof goes with `itemvoices`**

In `internal/hooks/item_voice_parity_test.go`, delete `TestItemTreePoolsMatchTheVoiceFiles` (with its comment) and the `internal/itemvoices` import.

- [ ] **Step 7: The narration snapshot**

In `internal/narration/snapshot_test.go`: replace the header line

```go
//   - _datafiles/world/dogmud/itemvoices/            2 files (sentient item voices)
```

with

```go
//   (itemvoices/ is retired since item behaviour slice 2, 2026-10-06: sentient
//   item speech lives in behaviors/items/ trees, pinned by
//   internal/hooks/testdata/item_voice_parity.golden.)
```

delete the six-line `//   - itemvoices: voiceid x the full 8-event valid set...` bullet, the `internal/itemvoices` import, the `itemvoices.LoadDataFiles()` call, the `t.Run("itemvoices", ...)` block (three lines) and the whole `// Store 6: itemvoices` section with `buildItemVoicesGolden`.

- [ ] **Step 8: The guards**

`item_behaviour_guard_test.go`: delete the `"VoiceId":` and `"TauntPull":` lines from `itemSpecBehaviourFields`.

`messaging_surface_guard_test.go`: replace the block from `// -- Sentient item voice narration: internal/itemvoices/itemvoices.go` through the `"taunt_pull": {config, ...}` line (the `lines` and `on_taunt` rows, the `voice_id` / `voiceid` comment and rows, the `taunt_pull` comment and row) with

```go
	// -- Sentient item speech: an item tree's speech: map
	// (behaviors/items/<tree>.yaml, internal/behaviortree TreeDef.Speech),
	// spoken by the speak node. Item behaviour slice 2 (2026-10-06) moved it
	// there from internal/itemvoices and retired voice_id, voiceid and
	// taunt_pull. --
	"lines":    {narration, "Overloaded but every schema hit is narration: quests/triggers.go NpcSayDef.Lines (npc_say scripted speech) and conversations/conversation.go ConversationDef.Lines (ambient NPC-NPC exchange, see CLAUDE.md NPC<->NPC Conversations). A handful of room `nouns:` children (e.g. \"flood lines\") coincidentally reuse this spelling as author content and are a known false positive of the 2-file heuristic -- see washing lines below for the same pattern."},
	"on_taunt": {narration, "a pool name under an item tree's speech: map (behaviors/items/aegis.yaml, blackrazor.yaml), the lines a sentient item speaks while its bearer fights, named by a speak node's pool param. A selector key like optionid, not prose itself, but part of the same narration shape."},
```

(`on_taunt` still appears in two files, the two trees, so it stays registered.)

`narration_render_callers_guard_test.go`: delete the `"internal/itemvoices/itemvoices.go":` line.

`shipped_narration_data_guard_test.go`: replace the `t.Run("itemvoices", ...)` block (seven lines) with

```go
	// Sentient item speech lives in item trees since item behaviour slice 2
	// (behaviors/items/<tree>.yaml speech:). ValidateItemBehaviors checks
	// every pool at boot, and TestEveryShippedItemBehaviorResolves runs it
	// over the shipped world.

```

replace `shippedWorldRoot + "/itemvoices",` in `narrationStoreWalkRoots` with `shippedWorldRoot + "/behaviors/items", // sentient item speech (item behaviour slice 2)`, append to the comment above it (after `which cannot rot the way a path exemption can.`)

```go
// behaviors/items is the one
// part of behaviors/ that is a narration store: since item behaviour slice 2
// it holds the sentient item speech pools itemvoices/ used to.
```

(rewrap to the comment's width), and remove the `internal/itemvoices` import.

- [ ] **Step 9: Comments that still say `itemvoices` is there**

- `internal/combat/taunt_messages.go`: replace `// returning a restore func for the caller to defer. Mirrors` / `// itemvoices.SeedVoicesForTest.` with `// returning a restore func for the caller to defer.`
- `internal/narration/picker.go`: replace `itemvoices` / `never validates its pool sizes, so a one-line voice pool is legitimate and` / `its draw count must not change either.` (the end of the `FirstPicker` comment) with `an item` / `tree's speech pool may legitimately hold one line, and its draw count must` / `not change either (a seeded run replays those draws).`
- `internal/narration/render.go`: `(itemvoices' on_equip, casting's` becomes `(an item tree's on_equip, casting's`.
- `modules/weather/content/arch_test.go`: `the same way internal/itemvoices and` / `internal/items already do, so this is a deliberate widening of the` becomes `the same way internal/items already does` / `(and internal/itemvoices did until item behaviour slice 2 moved its` / `pools into item trees), so this is a deliberate widening of the`.
- `internal/narration/context.md`: in Consumers, `` `internal/itemvoices`, `` becomes `` `internal/behaviortree` (an item tree's speech pools, through the `speak` node; item behaviour slice 2 retired `internal/itemvoices`), ``; in the boot-panic members, `itemvoices,` becomes `` item tree speech (`behaviortree.ValidateItemBehaviors`), ``; the `n == 1` paragraph's `because itemvoices never validates its pool sizes and legitimately holds one-line pools whose draw count must not change.` becomes `because an item tree's speech pool may legitimately hold one line, and its draw count must not change (a seeded run replays it).`
- `internal/behaviortree/item_engine.go`: the `ValidateItemBehaviors` comment's `after items load (X12, the voice_id precedent): a missing or broken tree` / `fails the boot instead of leaving an item silently inert.` becomes `after items load (X12, the precedent the retired voice_id check set): a` / `missing or broken tree, or a malformed voice (slice 2), fails the boot` / `instead of leaving an item silently inert.`

Then: `git grep -n -i "itemvoices\|voice_id\|VoiceSpec\|SeedVoicesForTest\|SentientChatter" -- '*.go' '*.js' '*.yaml'` finds only the engine's `itemVoices` map (`engine.go`, `test_export.go`), `item_engine.go:127`'s "the retired voice_id check", `snapshot_test.go:18` (the retirement note) and `:1208` (history, "the way itemvoices did"), `messaging_surface_guard_test.go:184`, `arch_test.go:19` and `shipped_narration_data_guard_test.go:278` (15 lines).

- [ ] **Step 10: Run everything**

Run: `gofmt -l internal/ modules/ . && go vet ./... && go build ./... && go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"`
Expected: no output at all.

- [ ] **Step 11: Commit**

```bash
git add main.go internal/items/itemspec.go _datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml _datafiles/world/dogmud/items/materials-40000/40185-aegis_of_mockery.yaml modules/gmcp/gmcp.Item.go modules/gmcp/gmcp.Item_test.go _datafiles/html/public/static/js/items.js internal/configs/config.balance.go internal/configs/config.balance.misc.go internal/configs/configs_pinnacle_test.go _datafiles/config.yaml internal/hooks/item_voice_parity_test.go internal/hooks/pinnacle_tick_test.go internal/narration/snapshot_test.go item_behaviour_guard_test.go messaging_surface_guard_test.go narration_render_callers_guard_test.go shipped_narration_data_guard_test.go internal/combat/taunt_messages.go internal/narration/picker.go internal/narration/render.go internal/narration/context.md modules/weather/content/arch_test.go internal/behaviortree/item_engine.go
git commit -m "refactor: retire itemvoices, voice_id, taunt_pull and the SentientChatter knobs" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(Step 1's `git rm` already staged the deletions; `git status --short` should show them as `D` before the commit.)

---

### Task 9: Docs, patch notes, gates (spec "Every slice")

**Model:** sonnet.

**Files:**
- Modify: `internal/behaviortree/context.md`, `internal/hooks/context.md`, `internal/characters/context.md`, `internal/util/context.md`; `docs/schemas/pinnacle-items.md`, `docs/schemas/behavior.md`; `docs/PATCH_NOTES.md:1-3`

- [ ] **Step 1: Verify every symbol before naming it**

Run (PowerShell): `Select-String -Path internal\behaviortree\item_voice.go,internal\behaviortree\actions_item_voice.go,internal\behaviortree\engine.go,internal\behaviortree\loader.go,internal\hooks\EquipmentChange_ItemEvents.go,internal\characters\character.go,internal\util\util.go -Pattern '^(func|type|const|var)\s'`
Expected to include: `type ItemVoice struct {`, `func itemVoiceFrom(def TreeDef) (*ItemVoice, error)`, `func checkSpeakNodes(`, `func refuseItemVoice(def TreeDef) error`, `func ResetItemListenerCapsForTest() func()`, `func condChatterReady(`, `func condHungerOverdue(`, `func actSpeak(`, `func actTauntPull(`, `func (e *Engine) GetItemVoice(name string) *ItemVoice`, `func (e *Engine) installItemTree(`, `func loadItemTreeDef(data []byte) (Node, *ItemVoice, error)`, `func LoadItemTreeFromFile(path string) (Node, *ItemVoice, error)`, `func ItemEquipEvents(e events.Event) events.ListenerReturn`, `func fireWornItemEvent(`, `func fireItemEvent(`, `func MiscRound(v any) (uint64, bool)`, `func SetRandForTest(seed int64) func()`. Name nothing that is not in that output.

- [ ] **Step 2: `internal/behaviortree/context.md`**

In the Event Types table, after the `item_idle` row add:

```markdown
| `on_equip` / `on_unequip` | Item trees only: the item was put on / taken off (`hooks.ItemEquipEvents`, item behaviour slice 2) | `ctx.Item.Slot` = its slot on equip, "" on removal |
| `on_kill` | Item trees only: the holder had a hand in a kill; every worn treed item (`hooks.MobDeathItemProcs`) | `ctx.Item.Slot` = its slot |
| `on_hunger_feeding` | Item trees only: a hungry weapon fed on its holder (`hooks.tickHunger`), paced by `HungerFeedingLineCooldownRounds` | `ctx.Item.Slot` = `weapon` |
```

In the Files table, the Items row gains `` `item_voice.go`, `actions_item_voice.go` ``. In "Item behaviour trees (lighting 5e, item behaviour slice 1)": the Boot bullet's `tree is missing, does not compile, or writes light on an adjustable worn` / `condition; main.go panics on it (X12, the `voice_id` precedent).` becomes `tree is missing, does not compile (a malformed voice included), or writes` / `light on an adjustable worn condition; main.go panics on it (X12, the` / `precedent the retired `voice_id` check set).`; the Compile bullet's allowlist gains `` `chatter_ready`, `hunger_overdue` `` among the conditions and `` `speak`, `taunt_pull` `` among the actions; the Tests bullet becomes `` `LoadItemTreeForTest` (installs the tree and its voice), `ItemBTreeStateForTest`, `ResetItemBTreeStatesForTest` and `ResetItemListenerCapsForTest` let other packages drive the engine. `` Append:

```markdown

## Item voices (item behaviour slice 2)

A sentient item's voice lives in its tree file (`item_voice.go`,
`actions_item_voice.go`); `internal/itemvoices`, `voice_id`, `taunt_pull` and
the `SentientChatter*` knobs are retired.

- **Data.** `TreeDef.Speech map[pool][]string` and `TreeDef.Chatter`
  (`quiet`, `normal` default, `chatty`). `loadItemTreeDef` turns them into an
  `ItemVoice` and refuses an unknown level, a level with no speech, an empty
  pool, a blank line, a `speak` naming a pool the tree lacks, or a `speak`
  `to:` other than `all` / `holder`. `refuseItemVoice` stops a mob, room or
  archetype tree carrying either. The engine keeps the voice beside the tree
  (`GetItemVoice(name)`); `speak` finds it through the item's template, so
  `EvalContext` needs no new field.
- **Pacing (Rule 17).** `chatter_ready` gates an AMBIENT line: Pinnacle on,
  the item's cooldown open (item state `speak_next_round`), at least one
  player in the room and every one past their listener cap
  (`listenerCapNext`, in memory, across every item), then the level's
  chance, drawn LAST so a closed round draws nothing. `speak` re-checks and
  arms the item's cooldown (`ItemChatter<Level>CooldownRounds`) and, for an
  `item_idle` line, starts every listener's cap
  (`ItemChatterListenerCapRounds`). An event branch (`on_equip`,
  `on_unequip`, `on_kill`) speaks without `chatter_ready`: no cap, no chance,
  cooldown still. `paced: false` skips the cooldown entirely (the feeding
  line, owner ruling S1).
- **`speak(pool, to, paced)`** picks through `narration.Render` with the
  default picker (a seeded `util.SetRandForTest` replays it), sends a player
  holder `<Item> says, "..."`, and with `to: all` (default) the room
  `<Name>'s <Item> mutters, "..."` through `SendTextHidingNames` /
  `HideSpeakerNames`: heard by everyone, the holder's name hidden at each
  listener's sight, never deafen-filtered (spec X17, ruling 6). A mob holder
  gets the room line only (ruling R6); a floor item has no voice.
- **`taunt_pull`** (ruling R5) makes a player holder's foe, a mob fighting
  someone else, fight the holder through `targeting.CommitTaunt`. It always
  returns Success, so a taunt branch never falls through to another line.
- **`hunger_overdue(fraction)`** is the Pinnacle tick's old warning state:
  the holder's `pinnacle_hunger_anchor` more than `fraction` of the item's
  `HungerRounds` behind.
- **Events** fired into item trees: `item_idle` (the item tick), `on_equip` /
  `on_unequip` (`hooks.ItemEquipEvents`, players and mobs), `on_kill`
  (`hooks.MobDeathItemProcs`, every worn treed item, ruling S3),
  `on_hunger_feeding` (`hooks.tickHunger`, the weapon, paced by
  `HungerFeedingLineCooldownRounds`).
- **Shipped trees.** `blackrazor.yaml` and `aegis.yaml`: worn, then
  `chatter_ready`, then taunt in combat (the Aegis pulls), a hunger warning
  past 0.75 (the Blackrazor), else idle; the event branches beside it.
```

- [ ] **Step 3: `internal/hooks/context.md`**

After the item tick section's last paragraph (ending `` `EquipBestFloorItem` skips fixtures. ``) add:

```markdown

**Item voices (item behaviour slice 2).** The tick's `item_idle` speaks a
sentient item's ambient lines; the Pinnacle tick no longer has a voice
sub-tick (`tickVoices`, `pickVoiceEvent`, `tryEmitVoice`, `emitVoiceLine`,
`applyTauntPull` are deleted). Event lines fire from three sites, each
spelling its `EventType` literal so `behaviortree`'s vocabulary test sees
it: `ItemEquipEvents` (`EquipmentChange_ItemEvents.go`, an
`events.EquipmentChange` listener) fires `on_equip` into every treed item put
on, found in its slot by UUID, and `on_unequip` into every one taken off,
players and mobs alike; `MobDeathItemProcs` fires `on_kill` into every worn
treed item of each player with damage on the kill (`fireWornItemEvent`);
`tickHunger` fires `on_hunger_feeding` into the weapon's tree when
`HungerFeedingLineCooldownRounds` allows a line (MiscData
`pinnacle_hunger_msg_next_round`) and sends the plain "The blade feeds on
you" fallback when no tree handles it. `pinnacle_voice_next_round` is inert
in old saves. `testdata/item_voice_parity.golden` is the Pinnacle voice
path's 200-round record, frozen before the move; `TestItemVoiceParity`
holds the tree path to it.
```

- [ ] **Step 4: `internal/characters/context.md` and `internal/util/context.md`**

Append to `internal/characters/context.md`:

```markdown

## `MiscRound` (item behaviour slice 2)

`MiscRound(v any) (uint64, bool)` reads a round number stored in MiscData. A
uint64 saved to a player file comes back from YAML as an int, int64 or
float64, so all four read; anything else (nil included) is not a round. It is
the one reader: `hooks.readMiscRound` delegates to it, and the item tree node
`hunger_overdue` reads the `pinnacle_hunger_anchor` through it.
```

In `internal/util/context.md`'s Dice block, after `func Rand(maxInt int) int` add the line `func SetRandForTest(seed int64) func() // tests only: Rand draws from a seeded source until restore`.

- [ ] **Step 5: `docs/schemas/pinnacle-items.md`**

Section 5 (hunger): replace `attrition, not an attack. The feeding message is cooldown-gated` / `(reuses `Balance.SentientChatterCooldownRounds`) so an ignored hunger` / `debt doesn't spam a line every single overdue round.` with `attrition, not an attack. The feeding message is cooldown-gated` / `` (`Balance.HungerFeedingLineCooldownRounds`, default 20) so an ignored `` / `hunger debt doesn't spam a line every single overdue round. When the` / `` line is due, `tickHunger` fires `on_hunger_feeding` into the weapon's `` / `item tree, which speaks to the bearer alone; a hunger weapon with no` / `tree sends the plain "The blade feeds on you" line instead.`

Section 6: replace the whole of `## 6. Voices (`voice_id:`)` (through the paragraph ending `that round.`) with:

````markdown
## 6. Voices (an item tree's `speech:`)

Since item behaviour slice 2 (2026-10-06) a sentient item speaks
through its behaviour tree; `voice_id:`, `taunt_pull:`, the
`itemvoices/` folder and the `SentientChatter*` knobs are gone. The
item names its tree:

```yaml
behavior: blackrazor
```

and `_datafiles/world/dogmud/behaviors/items/blackrazor.yaml` carries
the pools beside the tree that speaks them:

```yaml
chatter: normal          # quiet | normal (default) | chatty
speech:
  on_idle: ["..."]
  on_taunt: ["..."]
  on_kill: ["..."]
tree:
  type: selector
  children:
    - type: sequence
      event: item_idle
      children:
        - {type: condition, check: worn}
        - {type: condition, check: chatter_ready}
        - type: action
          do: speak
          pool: on_idle
    - type: action
      event: on_kill
      do: speak
      pool: on_kill
```

Pool names are free; every pool needs at least one non-blank line, and
every `speak` must name a pool its tree has, or the boot panics
(`behaviortree.ValidateItemBehaviors`). The shipped trees keep the old
event names as pool names. See `internal/behaviortree/context.md`,
"Item voices", for the nodes.

**Pacing**: an ambient line (the `item_idle` branch) passes
`chatter_ready`: the item's own cooldown open, everyone in the room
past their listener cap, then one roll at the level's chance. The
levels are `Balance.ItemChatter{Quiet,Normal,Chatty}CooldownRounds` and
`...ChancePct` (40/10, 20/15, 10/25); a listener hears at most one
ambient item line per `Balance.ItemChatterListenerCapRounds` (10),
across every item. Event lines (`on_equip`, `on_unequip`, `on_kill`)
skip the cap and the roll but still need the item's cooldown; the
feeding line ignores it (§5). Each item keeps its own cooldown, so a
player wearing two sentient items can hear both, bounded by the cap.

**Who hears it**: the bearer reads `<Item> says, "..."`; everyone else
in the room hears `<Name>'s <Item> mutters, "..."`, the bearer's name
hidden by each listener's sight ("Someone's Aegis of Mockery mutters"
in the dark). A mob holding a sentient item speaks the room line. A
kill reaches every worn sentient item, the shield's too. The Aegis's
taunt line is followed by `taunt_pull`, which pulls the bearer's foe
onto the bearer.
````

Section 9: replace the two `Balance.SentientChatter*` rows with

```markdown
| `Balance.ItemChatterQuietCooldownRounds` / `...ChancePct` | config.balance.go | `40` / `10` | A quiet item tree's rounds between lines and chance on an open round. |
| `Balance.ItemChatterNormalCooldownRounds` / `...ChancePct` | config.balance.go | `20` / `15` | The same for a normal tree (the default level; the shipped voices). |
| `Balance.ItemChatterChattyCooldownRounds` / `...ChancePct` | config.balance.go | `10` / `25` | The same for a chatty tree. |
| `Balance.ItemChatterListenerCapRounds` | config.balance.go | `10` | A listener hears at most one ambient item line per this many rounds, across every item. |
| `Balance.HungerFeedingLineCooldownRounds` | config.balance.go | `20` | Rounds between a hungry weapon's feeding lines; the drain runs every overdue round. |

The item chatter knobs read 0 as unset (the default applies), so 0 is
not an off switch for them; `GamePlay.PinnacleItemsEnabled` is.
```

Section 10: the `pinnacle_voice_next_round` row becomes `` | `pinnacle_voice_next_round` | (retired) | Inert in old saves: the voice cooldown is item tree state since item behaviour slice 2. | ``. Stage 2 table: 40183's cell `` `voice_id: blackrazor` `` becomes `` `behavior: blackrazor` (voice, slice 2) ``; 40185's `` `taunt_pull`, `voice_id: aegis` `` becomes `` `behavior: aegis` (voice and taunt pull, slice 2) ``. Replace the "Sentient item voices" table with:

```markdown
**Sentient item voices** (item trees, `behaviors/items/<tree>.yaml`, since
item behaviour slice 2):

| Tree | File | Item | Character |
|------|------|------|-----------|
| blackrazor | `behaviors/items/blackrazor.yaml` | The Blackrazor (40183) | Ancient, vain, starving aristocrat |
| aegis | `behaviors/items/aegis.yaml` | Aegis of Mockery (40185) | Period insult-comic |
```

(The Stage 2 boot record's `itemvoices loadedCount=2` is history and stays.)

- [ ] **Step 6: `docs/schemas/behavior.md`**

In "Item Behavior Trees (lighting 5e)": the allowlist table's Conditions cell gains `` `chatter_ready`, `hunger_overdue` `` and its Actions cell `` `speak`, `taunt_pull` ``. After the `pulse_light` row of the node table add:

```markdown
| `chatter_ready` | none | An ambient line may go out: the item's cooldown is open, everyone in the room is past their listener cap, and the tree's chatter chance rolls true (drawn last). |
| `hunger_overdue` | `fraction` (default 0.75) | The holder's hunger anchor is more than that fraction of the item's `hunger_rounds` behind. |
| `speak` | `pool`; `to`: `all` (default) or `holder`; `paced`: true (default) or false | Says a line from the tree's `speech:` pool: the holder reads "<Item> says", the room hears "<Name>'s <Item> mutters" with the holder's name hidden by sight. Paced: refuses while the item's cooldown is closed, then arms it. |
| `taunt_pull` | none | The holder's foe, a mob fighting someone else, turns on the holder. Always succeeds. |
```

In the following paragraph, the item-only list becomes `` (`holder_asleep`, `worn`, `in_combat`, `chatter_ready`, `hunger_overdue`, `set_light`, `pulse_light`, `speak`, `taunt_pull`) ``. Append:

```markdown

### Item voices (item behaviour slice 2)

An item tree may carry a voice: `speech:` maps pool names to lines, and
`chatter:` sets how often its ambient lines come (`quiet`, `normal` by
default, `chatty`; the cooldown and chance per level are the
`ItemChatter*` knobs in `config.yaml`). Only an item tree may carry them.
Besides `item_idle`, item trees hear `on_equip` and `on_unequip` (put on,
taken off), `on_kill` (the holder had a hand in a kill; every worn item
hears it) and `on_hunger_feeding` (a hungry weapon fed on its holder, paced
by `HungerFeedingLineCooldownRounds`). Gate ambient lines with
`chatter_ready`; an event branch speaks straight, skipping the listener
cap and the chance. `behaviors/items/blackrazor.yaml` and `aegis.yaml` are
the worked examples.
```

- [ ] **Step 7: Patch notes**

At the top of `docs/PATCH_NOTES.md`, above the newest entry, add (use the merge date if it is not 2026-10-06):

```markdown
## 2026-10-06: Talking gear

- The Blackrazor and the Aegis of Mockery now speak up when you put them
  on and when you take them off.
- The Aegis now has something to say about a kill too, not only the
  Blackrazor.
- Everyone nearby hears a talking item, even in the dark. Someone who
  cannot see you hears whose item it is as "Someone's".
- A monster carrying a talking item talks too.
- If several talking items share a room, you hear at most one idle remark
  every little while, so the chatter never piles up.
```

- [ ] **Step 8: Gates**

```bash
gofmt -l internal/ modules/ .
go vet ./...
go build ./...
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
golangci-lint run --new-from-merge-base=origin/master
python tools/context_md_audit.py > "$TMP/ctx_after.txt"
git diff origin/master -- . | grep "^+" | grep -c "—\|–"
```

Expected: gofmt, vet and the test grep print nothing; lint `0 issues.`; the dash count `0`. For the audit, run it on master too (`git worktree add --detach C:/tmp/dogmud-itembeh2-base origin/master`, run it there into `$TMP/ctx_before.txt`, then `git worktree remove C:/tmp/dogmud-itembeh2-base`) and diff: the only difference is `packages checked` one lower (`itemvoices` is gone). Known flake: `internal/playtestrun` can hang under load; re-run it alone before calling it a failure.

- [ ] **Step 9: Race run in Docker**

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-5e2-test . > "$TMP/5e2-build.log" 2>&1
docker run --rm dogmud-5e2-test > "$TMP/5e2-race.log" 2>&1
grep -c "WARNING: DATA RACE" "$TMP/5e2-race.log"
grep -E '^(--- FAIL|FAIL)' "$TMP/5e2-race.log"
docker rmi dogmud-5e2-test
```

Expected: `0` races; the only failures are the two tests that shell out to `git`, which has no repository inside the image (`TestNoStringOrDataSaysBuff` in the root package, `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`), with their `FAIL` package lines. The dry run's race run (2026-10-06) gave exactly that, 128 packages `ok`. The listener caps and item state are behind their own mutexes and `util.Rand`'s seam is atomic; a race here is a real one.

- [ ] **Step 10: Boot check on private ports**

```bash
printf 'Network:\n  TelnetPort: [33357]\n  LocalPort: 9957\n  HttpPort: 8057\n' > "$TMP/itembeh2-boot-overrides.yaml"
go build -o boot-check.exe .
CONFIG_PATH="$TMP/itembeh2-boot-overrides.yaml" timeout 180 ./boot-check.exe > "$TMP/itembeh2-boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$TMP/itembeh2-boot.log"
grep -c "Server Ready" "$TMP/itembeh2-boot.log"
grep "ValidateZoneConsi" "$TMP/itembeh2-boot.log"
rm -f boot-check.exe
```

Expected: `exit 124` (the timeout fired because the server stayed up), `0`, `1`, `errors=0 warnings=0 mode="panic"`. The server stops with the timeout; it never touches the user's ports.

- [ ] **Step 11: Commit**

```bash
git add internal/behaviortree/context.md internal/hooks/context.md internal/characters/context.md internal/util/context.md docs/schemas/pinnacle-items.md docs/schemas/behavior.md docs/PATCH_NOTES.md
git commit -m "docs: item voices (item behaviour slice 2)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Playtest and PR

**Model:** opus (the playtest is judgement work).

- [ ] **Step 1: Adversarial playtest (`playtest-scenario`, spec "Slice 2" playtest)**

Load the `dogmud-playtesting` skill first (harness location, ephemeral goals file, `--checkout`, never kill the user's server). Three agents on this branch's checkout: A wears the Blackrazor, B the Aegis of Mockery (admin-spawn both: `spawn item 40183`, `spawn item 40185`), C carries neither. Run it in a lit room and then in a dark one (a cave, or `server night` outdoors on a moonless night). Each agent reports exactly what it reads.

- A and B equip and remove their items: each reads its own equip and unequip line; the others read "<Name>'s <Item> mutters"; in the dark, "Someone's ..." (or "A figure's ..." at shapes), never the name.
- Idle a few minutes: lines come, never two ambient lines to C within the listener cap, and A's and B's items do not both fire at once.
- Fight a mob that will last several rounds (the combat fixture rule in the playtesting skill): taunts; with B in the fight and the mob on A, B's taunt pulls the mob onto B.
- A goes hungry (no kills for a few minutes): a hunger warning, then feeding lines to A alone, C reads none; a kill resets it and A's and B's items each savour it, the room hearing both.
- Every agent tries to make an item speak twice in a row by re-equipping: the cooldown holds.

File every finding as a GitHub issue (`--repo pruuk/DOGMud`, under #365 or the right epic) or fix it on this branch if it is this slice's bug; reports are gitignored.

- [ ] **Step 2: Push and open the PR**

Load the `dogmud-shipping` skill. Then:

```bash
git push -u origin feature/item-behaviour-slice2
gh pr create --repo pruuk/DOGMud --base master --head feature/item-behaviour-slice2 --title "Item behaviour slice 2: item voices (lighting 5e)" --body-file "$TMP/5e2-pr-body.md"
```

Check the URL `gh` prints says `pruuk/DOGMud`. Body (`$TMP/5e2-pr-body.md`): what ships, one line each (`speech:` / `chatter:`, the four nodes, the four events, the two trees, the retired surface); the owner rulings it implements (R5, R6, S1 to S3) and the knob coercion (F19); the thirteen items under "Where the spec could not be implemented as written"; the "Player-visible lines that change" table; the parity record (Blackrazor 200 of 200 rounds, Aegis to the kill, then its kill line); gate results with counts, the race run, the boot check, the playtest outcome; the spec and plan paths. Close #222 with the exact line `Closes #222` on its own line, and reference #365 without a closing keyword ("Part of #365": slice 3 remains). Never write "closes" or "fixes" next to any other issue number. End with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

- [ ] **Step 3: Watch the checks and merge**

```bash
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

If GitHub Actions runners stall (jobs cancelled with no steps run), the owner's standing call is to merge on the green local gate. After merging, confirm #222 closed and #365 is still open (`gh issue view 222 --repo pruuk/DOGMud`, `gh issue view 365 --repo pruuk/DOGMud`). The owner deploys; do not.

---

## Self-review

**Spec coverage.** Rule 16 (`speak`, `speech`, `chatter`, the senders, mob holders, the Pinnacle gate): Tasks 4, 5. Rule 17 (cooldown, chance, listener cap, event lines bypass the cap): Task 5 (`chatter_ready`, `speak`, `TestSpeakPacing`). Rule 18 (the two trees, pools byte for byte, taunt then `taunt_pull`, `hunger_overdue 0.75`, the event branches, `tickHunger` firing `on_hunger_feeding` with the knob-paced fallback): Tasks 6, 7. Rule 19 (retire `tickVoices`, `pickVoiceEvent`, `tryEmitVoice`, `emitVoiceLine`, the package, its data and golden, `voice_id`, `taunt_pull`, the builder fields, the two knobs; the MiscData key inert; hunger, drip, reserves and bandolier untouched; `on_grudge` dropped): Tasks 7, 8. Rule 6's slice 2 events: Task 6. X14 to X17: Tasks 3, 5, 7. X19 (the seam and the line goldens): Tasks 1, 2, 7. X21: Task 8. R5, R6: Tasks 5, 7. S1 to S3: Tasks 5 to 7. The spec's slice 2 tests: pools byte-identical (Task 7), a 200-round golden on both paths (Tasks 2, 7), the listener cap across two items with event lines through (`TestSpeakPacing`), the name hidden in the dark shown to fail against today's sender (Task 5 Step 8), equip events for a player and a mob (Task 6), `TauntPull` only with a taunt line (the Aegis tree puts it after `speak`; `TestTauntPull`), the feeding fallback paced by its knob (`TestHungerFeedingSpeaksThroughTheWeaponsTree`, with the knob set through `SetConfigForTest`), and the playtest (Task 10). "Every slice": `context.md`, patch notes, the full gate, race, boot (Task 9).

**Placeholder scan.** Every code step shows the code. Task 7 Step 4 and Task 8 Steps 4, 7 describe two deletions by their bounds (a 145-line tail of `pinnacle_tick.go`, the snapshot's builder) rather than reprinting deleted code; both bounds are unique strings and the compiler confirms them.

**Consistency.** `chatter_ready`, `hunger_overdue`, `speak`, `taunt_pull`, `ItemVoice`, `GetItemVoice`, `installItemTree`, `loadItemTreeDef`, `checkSpeakNodes`, `refuseItemVoice`, `ResetItemListenerCapsForTest`, `ItemEquipEvents`, `fireWornItemEvent`, `fireItemEvent`, `MiscRound`, `SetRandForTest` and the eight knob names are spelled the same in every task that uses them, and the line-keyed guard numbers follow each task's own edit (206/256 after Task 5; 341/355 after Task 6; 339/353 after Task 7).
