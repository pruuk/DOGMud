# Item Behaviour Slice 3: Item Procs (Lighting 5e) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The four Pinnacle proc items (the Blackrazor, the Aegis of Mockery, the Thornwall Harness, the Staff of the Hollow Choir) fire their combat procs through their behaviour trees, with outcomes identical to today's, and the Pinnacle proc path (`dispatchItemProcs`, `ItemProc`, `procs:` keys, the builder's proc editor) is deleted. Answers #223 (proc cooldowns are per template).

**Architecture:** A new item-only action `proc(effect, ...)` in `internal/behaviortree` runs one of the four effects, moved unchanged from `internal/hooks/item_procs.go` (hooks imports behaviortree, so the effects must live on the behaviortree side). A proc branch sits under a proc event (`on_hit`, `on_spell_hit`, `on_block`, `on_grapple`, `on_kill`), with its chance as a `random` decorator (left out at 100%, so nothing is drawn) and its cooldown as a `cooldown` decorator. In an item tree the compiler turns a `cooldown` over a `proc` into a `ProcCooldownDecorator` that keeps the round in the holder's MiscData (per template, shared by copies, kept across relog), and a `random` over a `proc` into a `ProcRandomDecorator` that draws only while procs are on. `hooks.fireItemProc` replaces `dispatchItemProcs` at six call sites; a kill reaches on_kill procs through slice 2's existing `on_kill` event (ruling S4). A proc outcome record taken on today's path, under the seeded `util.Rand`, proves the tree path identical before the old path is deleted.

**Tech Stack:** Go 1.25, YAML world data, the existing behaviour-tree engine.

**Spec (binding):** `docs/superpowers/specs/2026-10-05-item-behaviour-foundation-design.md`, slice 3 (Rules 20 to 23, ruling R1, X19, X21, the "Slice 3" gates), plus owner ruling S4 of 2026-10-06 below.

**Branch:** implementation branch `feature/item-behaviour-slice3`, cut from master AFTER the docs branch `docs/item-behaviour-slice3-plan` (this plan) merges, in the worktree `C:/tmp/dogmud-itembeh3`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/item-behaviour-slice3 C:/tmp/dogmud-itembeh3 origin/master
```

(The docs branch uses the same path; remove that worktree once the plan PR merges, then cut this one.) All paths below are relative to the worktree; run Go commands from its root. Name `C:/tmp/dogmud-itembeh3` in the handoff memory while the branch is open; remove it with `git worktree remove C:/tmp/dogmud-itembeh3` once the PR merges. Throwaway output goes in the session scratchpad (`$TMP`), never `C:/tmp`. Edits use the Edit and Write tools only, never a Python read-modify-write. **An Edit whose old or new text ends in a space loses that space**: anchor such edits on whole lines and run `gofmt -l`.

Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that. Subagent model: sonnet for Tasks 1, 2, 5 and 6; opus for Tasks 3 and 4 (the switch-over and the retirement touch the most files).

**How the code blocks read.** "Create" blocks are the whole file. "Modify" blocks are unified diffs against the file as the previous task left it: apply each hunk with the Edit tool (the `-` lines are the old text, the `+` lines the new; context lines locate it). The spec check for each task is a byte comparison of the task's files against the dry-run checkpoint named in the task (`git diff <checkpoint> -- <paths>` from the implementation worktree), and must print nothing. Every worktree shares the main repository's object store, so the checkpoints are reachable by SHA with no fetch; `git cat-file -t 6f1ae983d` printing `commit` confirms it. Keep `C:/tmp/dogmud-procs-dry` registered until the branch merges (a `git worktree remove` there does not drop the commits, but `git gc` could prune unreachable ones; the worktree's HEAD keeps them reachable).

**Dry run (2026-10-06).** Every block below was applied, in this order, to a scratch worktree `C:/tmp/dogmud-procs-dry` detached at `origin/master` `de8cf323a`, one local checkpoint commit per task (never pushed): Task 1 `d0eda2ef8`, Task 2 `6036929bf`, Task 3 `264f7765a`, Task 4 `59af9ebd1`, Task 5 `d7d178073`, Task 6 `6f1ae983d`. The failing-first and null-probe steps were run and seen red as each step says. Found in the dry run and carried into the task that causes it: the events vocabulary test needs an `EventType: "..."` literal at a dispatch site for every event a shipped tree names, so the content and the dispatch land together (Task 3); `condition_apply_path_guard_test.go` keys sites by line number and Tasks 2, 3 and 4 each move lines it names (the numbers below were read from the checkpoints); `RemoveCondition` only expires a record until the prune pass, so the parity test resets a foe's condition set instead (Task 1). At the end: `gofmt -l internal/ modules/` clean; `go build ./...`; `go test ./... -count=1` with no `FAIL`; `golangci-lint run --new-from-merge-base=origin/master` `0 issues.` (one stale-cache warning naming another worktree's path); `python tools/context_md_audit.py` 16 phantom lines before and after, none in a touched package; no em or en dash in any new doc, patch note or player line (the effect comments moved verbatim from `item_procs.go` keep theirs); a boot on private ports through a scratch `CONFIG_PATH` overrides file (telnet 33334, local 9998, http 8091, AI port off) stayed up to the 150 s timeout (exit 124) with 0 panics and one `Server Ready`. The Docker race run is recorded under Task 6.

---

## Facts verified against source (2026-10-06, `origin/master` `de8cf323a`)

Every row was read at `de8cf323a`. **NEW** marks a fact the spec does not state, or states wrongly, that changes the plan.

| # | Fact | Where |
|---|---|---|
| F1 | `dispatchItemProcs(trigger, owner, other, room, damage)`: per item from `procBearingItems`, per proc from `spec.ProcsFor(trigger)`: `procGateOpen` (switch, cooldown, chance), the effect switch, `markProcCooldown` only when the effect executed | `internal/hooks/item_procs.go:98-131` |
| F2 | `procGateOpen` order: `ItemProcsEnabled`, then the cooldown (`now < until` fails), then `p.Chance < 100 && util.Rand(100) >= p.Chance`; at 100 nothing is drawn | `item_procs.go:49-67` |
| F3 | Cooldown key `pinnacle_proc_cd_<itemId>_<procIdx>` in the OWNER's MiscData, storing the round it may fire again (`now + CooldownRounds`) | `item_procs.go:33-35,70-75` |
| F4 | `procBearingItems`: on_hit, on_kill, on_spell_hit read the weapon; on_block the offhand; on_grapple the body | `item_procs.go:136-146` |
| F5 | The four effects `procLifesteal`, `procStealPool`, `procApplyCondition`, `procAoeStun` use only `characters`, `conditions`, `messaging`, `mobs`, `rooms`, `state` | `item_procs.go:77-285` |
| F6 | **NEW.** `behaviortree` already imports all six (and `hooks` imports `behaviortree`), so the effects can move into `behaviortree` unchanged; a `proc` node cannot call them in `hooks` | `go list -f '{{.Imports}}' ./internal/behaviortree` |
| F7 | Seven call sites: `MobDeath_ItemProcs.go:24` (on_kill), `NewRound_DoCombat_unified.go:166` (on_hit), `:180` (on_block), `Position_GrappleTick.go:408-409` (on_grapple, both sides), `spell_effects.go:244,357` (on_spell_hit) | grep |
| F8 | `MobDeathItemProcs` also fires slice 2's `on_kill` into every worn treed item (`fireWornItemEvent`); `fireItemEvent(event, it, slot, c, userId, mobInstanceId)` wraps `TryItemBehavior` | `MobDeath_ItemProcs.go:30`; `EquipmentChange_ItemEvents.go:65-80` |
| F9 | `CooldownDecorator` keeps the LAST-RUN round in item tree state and fails while `now - lastRun < Rounds`; a fresh state reads 0. `RandomDecorator` draws `util.Rand(100)` unconditionally | `internal/behaviortree/decorators.go:7-25,59-70` |
| F10 | `compileDecorator` builds `cooldown` with `StateKey: path + "_cooldown"` and `random` with `Percent`; item trees compile under root label `item` (`isItemTreePath`) | `loader.go:81,275-318` |
| F11 | Item tree state is per UUID, in memory, re-minted on every load (relog, restart): a tree-state cooldown cannot survive a relog or be shared by copies | `item_state.go:10-24` |
| F12 | `itemHolder(ctx)` resolves the holder through `users.GetByUserId` / `mobs.GetInstance` | `conditions_item.go:15-31` |
| F13 | `EventContext` has no field for an opponent, a room pointer or damage | `types.go:18-34` |
| F14 | **NEW.** `events_test.go` requires every event a live tree names to appear as an `EventType: "<name>"` literal in non-test Go, and every literal to be in `KnownBehaviorEvents` (which has `on_kill` but not `on_hit`, `on_block`, `on_grapple`, `on_spell_hit`) | `events_test.go:17-74`; `events.go:16-37` |
| F15 | `checkSpeakNodes` is the load-time param check pattern for item nodes; `loadItemTreeDef` calls it before `compileNode` | `item_voice.go:71-97`; `loader.go:102-117` |
| F16 | Item allowlists `itemSafeConditions`, `itemSafeActions` and `itemOnlyNodes` | `loader.go:131-165` |
| F17 | `ItemProc`, `validProcTriggers`, `validProcEffects`, `ItemSpec.Procs`, the proc loop in `Validate`, `ProcsFor`; `proc_accessors.go` (`ValidProcTriggers`, `ValidProcEffects`, `sortedBoolKeys`, used only there) | `internal/items/itemspec.go:255-271,285,779-789,822-833`; `proc_accessors.go` |
| F18 | Builder: `procRow`, `itemUpdateReq.Procs`, `itemDetail.ProcTriggers/ProcEffects`, `procTriggerIds`, `procEffectIds`, the `Procs` loops; `items.js` proc editor and its three DOM helpers `selBox`, `numBox`, `labelWrap` (used nowhere else) | `modules/gmcp/gmcp.Item.go:35-41,101,121-122,200-202,219-225,257-263`; `_datafiles/html/public/static/js/items.js:82-99,427,436,449,478-532,559` |
| F19 | Content: `procs:` on 40183 (on_hit lifesteal 100%, ratio 0.25), 40185 (on_block aoe_stun 10%, cd 20), 40186 (on_grapple bleed 50%, cd 5, duration 10, magnitude 7), 40189 (on_spell_hit steal_pool 100%, cd 3, pool 3, amount_pct 0.08). 40183 and 40185 have trees (`blackrazor`, `aegis`); 40186 and 40189 have none | `items/materials-40000/` |
| F20 | `hooks.readMiscRound` wraps `characters.MiscRound`; besides procs, `pinnacle_tick.go` calls it six times and `pinnacle_tick_test.go` three | grep |
| F21 | `condition_apply_path_guard_test.go` keys `internal/hooks/item_procs.go|256` (stun, mob holder) and `|206` (bleed) by line; `item_behaviour_guard_test.go` classifies `Procs`, maps `ProcsFor` to it and lists the read site `item_procs.go|Procs` | `:138,:295`; `:51,:90,:101` |
| F22 | `GamePlay.ItemProcsEnabled` ships `true` | `internal/configs/config.gameplay.go:30` |
| F23 | `util.SetRandForTest(seed)` swaps `util.Rand`'s source atomically (slice 2) | `internal/util/util.go:209-228` |
| F24 | **NEW.** `RemoveCondition` only expires a record (`expire()`); `HasCondition` reads it as held until the prune pass | `internal/conditions/conditions.go:131-137` |
| F25 | `*.golden` is `text eol=lf`, so a Windows checkout keeps the record byte-exact | `.gitattributes:22` |
| F26 | Rounds are 4 s (`Timing.RoundSeconds: 4`), so the Aegis's 20-round cooldown is 80 s | `_datafiles/config.yaml:193` |

## Owner ruling of 2026-10-06 (beyond the spec)

- **S4.** An `on_kill` proc reaches **every worn item with a tree**, the reach slice 2 gave kill lines (S3), not the weapon alone. `MobDeathItemProcs` stops dispatching a separate on_kill proc; its one `on_kill` event carries both. No shipped item has an on_kill proc, so nothing a player sees changes.

## Where the spec could not be implemented as written

1. **Rule 20 "wraps procLifesteal... unchanged".** The node lives in `behaviortree` and `hooks` imports `behaviortree` (F6), so the four effects MOVE into `internal/behaviortree/actions_item_proc.go` with their bodies unchanged (comments that named `dispatchItemProcs` reworded).
2. **Rule 21, the event's participants.** `EventContext` carries no opponent, room or damage (F13). It gains `Proc *ProcEvent{Other, Room, Damage}`, nil for every other event; `proc` reads a nil one as zero (a kill).
3. **Rule 21, "the seven call sites".** Six call `fireItemProc`; the seventh (on_kill) is deleted under S4, because the kill event already reaches every worn treed item.
4. **Rule 22, "a proc branch's cooldown decorator".** The tree `cooldown` keeps a last-run round in item state (F9, F11). In an item tree, a `cooldown` whose subtree names `proc` compiles to `ProcCooldownDecorator`, which keeps the ready-round in the holder's MiscData under `item_proc_cd_<itemId>_<compile path>`: today's semantics (F3), a new key (the old `pinnacle_proc_cd_*` keys are inert, so a cooldown running at deploy starts fresh once).
5. **"`ItemProcsEnabled` off stops every proc"** with no draw. `fireItemProc` reads the switch before running a tree; a kill does not pass through it, so a `random` over a `proc` compiles to `ProcRandomDecorator`, which reads the switch before drawing; `proc` reads it too.
6. **X19 enforced.** The loader refuses a `random` over a `proc` outside 1 to 99, so a 100% proc can only be written without a draw. It also refuses a `proc` outside a proc event, an unknown effect, and a param the effect does not read or that is not a number (the checks `ItemSpec.Validate` made on `procs:`).

## Player-visible lines that change

None: the parity record (Task 1) holds every outcome on every round. The builder's item editor loses its proc rows (X21; tree editing is #367); its Advanced heading becomes "Advanced: reserves, hunger, mutation".

## File map

| Task | Files |
|---|---|
| 1 | `internal/hooks/item_proc_parity_test.go` (new), `internal/hooks/testdata/item_proc_parity.golden` (new, recorded) |
| 2 | `internal/behaviortree/actions_item_proc.go` (new), `item_proc_test.go` (new), `actions.go`, `loader.go`, `types.go`; `condition_apply_path_guard_test.go` |
| 3 | `_datafiles/world/dogmud/behaviors/items/aegis.yaml`, `blackrazor.yaml`, `thornwall_harness.yaml` (new), `hollow_choir.yaml` (new); items 40186, 40189; `internal/behaviortree/events.go`, `loader.go`, `actions_item_proc.go`; `internal/hooks/item_proc_dispatch.go` (new), `NewRound_DoCombat_unified.go`, `Position_GrappleTick.go`, `spell_effects.go`, `MobDeath_ItemProcs.go`, `item_proc_parity_test.go`; `condition_apply_path_guard_test.go` |
| 4 | delete `internal/hooks/item_procs.go`, `item_procs_test.go`, `internal/items/proc_accessors.go`, `proc_accessors_test.go`; new `internal/behaviortree/item_proc_effects_test.go`, `internal/hooks/item_proc_dispatch_test.go`; `internal/hooks/item_proc_parity_test.go`, `pinnacle_tick.go`, `pinnacle_tick_test.go`; `internal/items/itemspec.go`, `itemspec_pinnacle_test.go`, `save_test.go`; `modules/gmcp/gmcp.Item.go`, `gmcp.Item_test.go`; `_datafiles/html/public/static/js/items.js`; items 40183, 40185, 40186, 40189; `condition_apply_path_guard_test.go`, `item_behaviour_guard_test.go` |
| 5 | `docs/PATCH_NOTES.md`, `docs/schemas/behavior.md`, `docs/schemas/pinnacle-items.md`, `internal/behaviortree/context.md`, `internal/hooks/context.md`, `internal/items/context.md`, `internal/characters/context.md` |
| 6 | `tools/playtest/scenarios/item-procs.yaml` (new), `tools/playtest/goals/scenarios/item-procs/{blade,shield,caster}.yaml` (new); gates |
| 7 | playtest, PR |

New files are listed in `docs/README.md` only when they are docs; this plan adds its own row (docs branch).

---

### Task 1: Record today's proc path (the parity record)

Spec "Slice 3" gates: before deleting, a proc outcome record for the four items under X19's seam, on both paths. This task writes the harness and records it on today's path. Checkpoint `d0eda2ef8`.

The record has five scenarios of 60 rounds from round 2000 under seed 7331: the Blackrazor on_hit with damage cycling 12, 0, 40, 3, 1 (the 0 rounds prove a no-op lifesteal); the Aegis on_block against two hostiles, with the room emptied for rounds 40 to 50 (a successful roll there must not burn the cooldown: in the dry run a roll landed at 46 with nobody to stun and the shield fired at 56); the Harness on_grapple from both sides of the hold in alternating order; the Staff on_spell_hit against a foe that runs dry and is refilled at round 40; and a kill every third round with three proc items worn. Each round records the bearer's health and conviction, each foe's health, conviction, stun and bleed stacks, and then a probe draw `util.Rand(1000)`, so a path that draws one number more or fewer moves every later line.

- [ ] **Step 1: Write the harness**

**Create `internal/hooks/item_proc_parity_test.go`:**

````go
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
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

// The proc parity record (item behaviour slice 3, spec "Slice 3" gates).
// testdata/item_proc_parity.golden was recorded on the Pinnacle proc path
// (dispatchItemProcs) BEFORE the procs moved into item trees, and is
// frozen: the tree path must reproduce it, the same outcomes on the same
// rounds, under one seeded random source (util.SetRandForTest). One
// scenario per shipped proc item and trigger, hits and misses, cooldown
// windows and rounds where the effect has nothing to act on, plus a kill
// scenario with every proc item worn. After each round a probe draw is
// recorded, so a path that draws more or fewer numbers moves the record.
//
// Set DOGMUD_RECORD_PROC_PARITY=1 to rewrite the record. It was written
// once, on the Pinnacle path; do not rewrite it on the tree path.

const (
	procParityUserId = 1
	procParityRoomId = 9972
	procParityFirst  = 2000 // the first scenario round
	procParityRounds = 60
	procParitySeed   = 7331
	procParityMobA   = 9801 // the bearer's foe
	procParityMobB   = 9802 // a second hostile in the room (aoe_stun)
)

// procDispatch fires a trigger's procs for owner against other: the
// dispatcher under test.
type procDispatch func(trigger string, owner, other *characters.Character, room *rooms.Room, damage int)

// procParityPaths are the dispatchers the record is checked against.
var procParityPaths = map[string]procDispatch{
	"pinnacle": dispatchItemProcs,
}

// loadProcParityWorld loads the shipped conditions and items, points the
// engine at the shipped item trees, enables procs and seeds one room.
func loadProcParityWorld(t *testing.T) *rooms.Room {
	t.Helper()
	// The overlay first: AddOverlayOverrides rebuilds the live config, so
	// it would drop a data path set before it.
	setItemProcsEnabled(t, true)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them too.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for id := range procParityItems {
		if spec := items.GetItemSpec(id); spec != nil && spec.Behavior != `` {
			name := spec.Behavior
			behaviortree.GetEngine().EvictItemTree(name)
			t.Cleanup(func() { behaviortree.GetEngine().EvictItemTree(name) })
		}
	}

	room := rooms.NewRoom("procparity")
	room.RoomId = procParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{procParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// setItemProcsEnabled sets ItemProcsEnabled for the test and puts it back.
func setItemProcsEnabled(t *testing.T, on bool) {
	t.Helper()
	prev := bool(configs.GetConfig().GamePlay.ItemProcsEnabled)
	if err := configs.AddOverlayOverrides(map[string]any{"GamePlay.ItemProcsEnabled": on}); err != nil {
		t.Fatalf("failed to set ItemProcsEnabled=%v: %v", on, err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"GamePlay.ItemProcsEnabled": prev})
	})
}

// procParityItems are the shipped proc items and the slot each is worn in.
var procParityItems = map[int]string{
	40183: "weapon",  // The Blackrazor: on_hit lifesteal
	40185: "offhand", // Aegis of Mockery: on_block aoe_stun
	40186: "body",    // Thornwall Harness: on_grapple bleed
	40189: "weapon",  // Staff of the Hollow Choir: on_spell_hit steal_pool
}

// procParityBearer seeds a fresh user 1 in room wearing a fresh instance of
// each of itemIds, with fresh item state.
func procParityBearer(t *testing.T, room *rooms.Room, itemIds ...int) *users.UserRecord {
	t.Helper()
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())
	u := users.NewTestUser(procParityUserId, "bearer", "Bearer", 0)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{procParityUserId: u}))
	room.AddPlayer(procParityUserId)
	for _, id := range itemIds {
		it := items.New(id)
		switch procParityItems[id] {
		case "weapon":
			u.Character.Equipment.Weapon = it
		case "offhand":
			u.Character.Equipment.Offhand = it
		case "body":
			u.Character.Equipment.Body = it
		default:
			t.Fatalf("procParityBearer: item %d has no slot", id)
		}
	}
	return u
}

// procParityMob registers a hostile mob instance in room.
func procParityMob(t *testing.T, room *rooms.Room, instanceId int) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		InstanceId: instanceId,
		HomeRoomId: room.RoomId,
		Character: characters.Character{
			Name:       "Parity Beast",
			RoomId:     room.RoomId,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 500
	m.Character.Health = 500
	m.Character.ConvictionMax.Value = 100
	m.Character.Conviction = 30
	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instanceId, nil) })
	room.AddMob(instanceId)
	return m
}

// procParityState is one round's observable outcome.
func procParityState(owner *characters.Character, foes ...*mobs.Mob) string {
	var b strings.Builder
	fmt.Fprintf(&b, "hp=%d conv=%d", owner.Health, owner.Conviction)
	for _, m := range foes {
		c := &m.Character
		fmt.Fprintf(&b, " | %s hp=%d conv=%d stun=%t", c.Name, c.Health, c.Conviction, c.HasCondition(84))
		for _, rec := range c.GetConditions(conditions.ConditionIdBleeding) {
			fmt.Fprintf(&b, " bleed=%d/%d/%d", len(rec.Stacks), rec.TriggersLeft, rec.TickAmount)
		}
	}
	return b.String()
}

// procParityScenario is one recorded run: run seeds its bearer and foes and
// returns their record, firing each round through fire.
type procParityScenario struct {
	name string
	run  func(t *testing.T, room *rooms.Room, fire procDispatch) string
}

// runProcRounds plays procParityRounds rounds under the seed, calling
// round(i) each round and recording the state after it, then a probe draw.
func runProcRounds(t *testing.T, round func(i int), state func() string) string {
	t.Helper()
	restoreRand := util.SetRandForTest(procParitySeed)
	defer restoreRand()
	defer util.ResetRoundCountForTest()
	var b strings.Builder
	for i := 0; i < procParityRounds; i++ {
		util.SetRoundCountForTest(uint64(procParityFirst + i))
		round(i)
		fmt.Fprintf(&b, "%d: %s probe=%d\n", i, state(), util.Rand(1000))
	}
	return b.String()
}

var procParityScenarios = []procParityScenario{
	{"blackrazor_on_hit", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40183)
		c := u.Character
		c.HealthMax.Value = 1000
		c.Health = 100
		foe := procParityMob(t, room, procParityMobA)
		damage := []int{12, 0, 40, 3, 1}
		return runProcRounds(t, func(i int) {
			fire("on_hit", c, &foe.Character, room, damage[i%len(damage)])
		}, func() string { return procParityState(c, foe) })
	}},
	{"aegis_on_block", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40185)
		c := u.Character
		foe := procParityMob(t, room, procParityMobA)
		other := procParityMob(t, room, procParityMobB)
		return runProcRounds(t, func(i int) {
			// No hostile in the room for rounds 40 to 50: a stun that
			// finds nobody does not burn the cooldown.
			switch i {
			case 40:
				room.RemoveMob(procParityMobA)
				room.RemoveMob(procParityMobB)
			case 51:
				room.AddMob(procParityMobA)
				room.AddMob(procParityMobB)
			}
			// A fresh condition set each round: RemoveCondition only
			// expires a record until the prune pass, so the stun would
			// read as still held.
			foe.Character.Conditions = conditions.New()
			other.Character.Conditions = conditions.New()
			fire("on_block", c, &foe.Character, room, 5)
		}, func() string { return procParityState(c, foe, other) })
	}},
	{"harness_on_grapple", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40186)
		c := u.Character
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			// Both sides of the hold, as Position_GrappleTick fires them;
			// the foe wears no harness.
			if i%2 == 0 {
				fire("on_grapple", c, &foe.Character, nil, 0)
				fire("on_grapple", &foe.Character, c, nil, 0)
			} else {
				fire("on_grapple", &foe.Character, c, nil, 0)
				fire("on_grapple", c, &foe.Character, nil, 0)
			}
		}, func() string { return procParityState(c, foe) })
	}},
	{"staff_on_spell_hit", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40189)
		c := u.Character
		c.ConvictionMax.Value = 1000
		c.Conviction = 0
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			// The foe runs dry, then is refilled at round 40.
			if i == 40 {
				foe.Character.Conviction = 30
			}
			fire("on_spell_hit", c, &foe.Character, nil, 20)
		}, func() string { return procParityState(c, foe) })
	}},
	{"all_on_kill", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40183, 40185, 40186)
		c := u.Character
		c.HealthMax.Value = 1000
		c.Health = 100
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			if i%3 == 0 {
				MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{u.UserId: 1}})
			}
		}, func() string { return procParityState(c, foe) })
	}},
}

// runProcParity plays every scenario on one dispatcher and returns the
// whole record.
func runProcParity(t *testing.T, fire procDispatch) string {
	t.Helper()
	var b strings.Builder
	for _, s := range procParityScenarios {
		var out string
		t.Run(s.name, func(t *testing.T) {
			room := loadProcParityWorld(t)
			out = s.run(t, room, fire)
		})
		fmt.Fprintf(&b, "## %s\n%s", s.name, out)
	}
	return b.String()
}

func TestItemProcParity(t *testing.T) {
	path := filepath.Join("testdata", "item_proc_parity.golden")
	if os.Getenv("DOGMUD_RECORD_PROC_PARITY") == "1" {
		if err := os.WriteFile(path, []byte(runProcParity(t, dispatchItemProcs)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("recorded " + path)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, fire := range procParityPaths {
		if got := runProcParity(t, fire); got != string(want) {
			t.Errorf("%s path moved off the record.\n--- want\n%s\n--- got\n%s", name, want, got)
		}
	}
}
````

- [ ] **Step 2: Record**

```bash
DOGMUD_RECORD_PROC_PARITY=1 go test ./internal/hooks -run TestItemProcParity -count=1
sha256sum internal/hooks/testdata/item_proc_parity.golden
```

Expected: `ok` (the test skips after writing), then `3b4491ed0fd583ece6d4aae15b83eece87a0405bef1c5f044a4cac11590d1a92`; the file is 305 lines. A different hash means the harness or the world differs from the dry run: stop and compare with `git diff d0eda2ef8 -- internal/hooks/`.

- [ ] **Step 3: The record holds**

```bash
go test ./internal/hooks -run TestItemProcParity -count=1
awk '/^## /{s=$2} /stun=true/{if(s=="aegis_on_block")a=a" "$1} END{print a}' internal/hooks/testdata/item_proc_parity.golden
```

Expected: `ok`, then ` 18: 56:` (the Aegis fires twice, the second after the empty-room roll at 46).

- [ ] **Step 4: Spec check and commit**

```bash
gofmt -l internal/hooks
git diff d0eda2ef8 -- internal/hooks/item_proc_parity_test.go internal/hooks/testdata/item_proc_parity.golden
git add internal/hooks/item_proc_parity_test.go internal/hooks/testdata/item_proc_parity.golden
git commit -m "test(hooks): record the item proc path before it moves into trees" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: The `proc` node and the holder-kept proc cooldown (Rules 20, 22; R1; X19)

Checkpoint `6036929bf`. `actions_item_proc.go` holds the four effects moved unchanged from `hooks/item_procs.go` (which stays until Task 4, so for two tasks the bodies exist twice), `ProcEvent`, `ItemProcsOn`, `actProc`, `ProcCooldownDecorator`, `procCooldownKey`, `nodeDefNamesAction` and `checkProcNodes`. `EventContext` gains `Proc`. The compiler swaps in `ProcCooldownDecorator` for a `cooldown` over a `proc` in an item tree. The new effect bodies land at lines the condition-path guard reads, so it gets two keys now (moved again in Task 3).

- [ ] **Step 1: Write the tests first**

The `item_proc_test.go` block below. Run `go test ./internal/behaviortree -run TestProc -count=1`: it does not compile (`ProcEvent`, `procCooldownKey`, `loadItemTreeDef` refusing nothing). That is the failing state.

- [ ] **Step 2: Implement**

**Modify `condition_apply_path_guard_test.go`:**

````diff
@@ -134,9 +134,10 @@ var conditionApplyPathAllowlist = map[string]string{
 	"internal/usercommands/skill.disenchant.go|71": "former combat condition (withdrawal): the disenchant command narrates; must apply synchronously so Validate clamps the pool now",
 
 	// ── mob holders: no client, so no line could reach anyone ───────────────
-	"internal/usercommands/character.go|423":     "the holder is a MOB (m.Character), and condition 99 is a perma-gear pin, not something a player reads",
-	"internal/hooks/item_procs.go|256":           "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
-	"internal/hooks/manifester_companions.go|40": "the holder is a MOB (a summoned companion), not a player",
+	"internal/usercommands/character.go|423":         "the holder is a MOB (m.Character), and condition 99 is a perma-gear pin, not something a player reads",
+	"internal/hooks/item_procs.go|256":               "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
+	"internal/behaviortree/actions_item_proc.go|308": "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
+	"internal/hooks/manifester_companions.go|40":     "the holder is a MOB (a summoned companion), not a player",
 
 	// ── secret conditions: silence is the authored intent ───────────────────
 	"internal/hooks/Life_Cascades.go|131": "condition 81 Respawn Grace is secret:true, so StartUserNotice is empty by design and the event would narrate nothing anyway",
@@ -286,13 +287,14 @@ var conditionApplyPathAllowlist = map[string]string{
 	// ── former combat condition: Bleeding is now one stacking record (Task 9;
 	// re-keyed slice 1b; re-keyed again counters slice Task 3 when the
 	// drain-area counter field and its exit call were deleted) ───────────
-	"internal/actions/combat_drain.go|147":     "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/actions/combat_drain.go|314":     "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/actions/combat_hamstring.go|138": "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/actions/combat_maul.go|132":      "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/actions/combat_rake.go|132":      "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/actions/combat_throttle.go|146":  "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/hooks/item_procs.go|206":         "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_drain.go|147":           "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_drain.go|314":           "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_hamstring.go|138":       "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_maul.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_rake.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/actions/combat_throttle.go|146":        "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
+	"internal/hooks/item_procs.go|206":               "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
+	"internal/behaviortree/actions_item_proc.go|258": "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
 }
 
 // primitivePackages define Character.AddCondition / Conditions.AddCondition
````

**Modify `internal/behaviortree/actions.go`:**

````diff
@@ -129,6 +129,8 @@ func init() {
 	// Item voices (item behaviour slice 2)
 	actionRegistry["speak"] = actSpeak
 	actionRegistry["taunt_pull"] = actTauntPull
+	// Item procs (item behaviour slice 3)
+	actionRegistry["proc"] = actProc
 }
 
 // LookupAction returns the action function for the given name,
````

**Create `internal/behaviortree/actions_item_proc.go`:**

````go
package behaviortree

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The proc node (item behaviour slice 3, Rules 20 to 22). A proc is an item
// tree branch under one of the proc events, usually wrapped in a `random`
// decorator (its chance; omitted at 100% so no number is drawn) inside a
// `cooldown` decorator. `proc` runs one of four effects, written in Go
// because this is the combat hot path, and succeeds only when the effect
// did something, so the cooldown is armed exactly when it took effect.

// procEvents are the events a proc branch may sit under (spec P2).
var procEvents = map[string]bool{
	"on_hit": true, "on_kill": true, "on_block": true, "on_grapple": true, "on_spell_hit": true,
}

// procEffectParams are the effects and the numeric params each reads.
var procEffectParams = map[string]map[string]bool{
	"lifesteal":       {"ratio": true},
	"steal_pool":      {"pool": true, "amount_pct": true},
	"aoe_stun":        {"stun_rounds": true},
	"apply_condition": {"condition": true, "duration": true, "magnitude": true},
}

// ProcEvent carries a proc trigger's participants: the bearer's opponent
// (nil on a kill), the room (nil where the caller does not know it) and
// the damage the trigger dealt.
type ProcEvent struct {
	Other  *characters.Character
	Room   *rooms.Room
	Damage int
}

// ItemProcsOn reports the ItemProcsEnabled switch. The dispatcher reads it
// before running a tree, so a disabled proc draws nothing.
func ItemProcsOn() bool {
	return bool(configs.GetConfig().GamePlay.ItemProcsEnabled)
}

// actProc runs the effect named by `effect` for the item's holder. The
// other params are the effect's, as numbers.
func actProc(params map[string]any, ctx *EvalContext) Result {
	if !ItemProcsOn() {
		return Failure
	}
	owner := itemHolder(ctx)
	if owner == nil {
		return Failure
	}
	var ev ProcEvent
	if ctx.Event.Proc != nil {
		ev = *ctx.Event.Proc
	}
	effect := getStringParam(params, `effect`)
	p := make(map[string]float64, len(params))
	for k := range procEffectParams[effect] {
		if _, ok := params[k]; ok {
			p[k] = getFloatParam(params, k, 0)
		}
	}
	executed := false
	switch effect {
	case `lifesteal`:
		executed = procLifesteal(owner, ev.Damage, p) > 0
	case `steal_pool`:
		executed = procStealPool(owner, ev.Other, p)
	case `aoe_stun`:
		executed = procAoeStun(owner, ev.Room, p)
	case `apply_condition`:
		executed = procApplyCondition(ev.Other, p)
	}
	if executed {
		return Success
	}
	return Failure
}

// ProcCooldownDecorator is a `cooldown` decorator over a proc branch (Rule
// 22, ruling R1). Its state lives in the holder's MiscData, keyed by the
// item's template id and the branch's path, not in the item's tree state:
// two copies of one item share it and it survives a relog. It stores the
// round the branch may fire again.
type ProcCooldownDecorator struct {
	Rounds int
	Path   string // the decorator's compile path; names the branch
	Child  Node
}

// procCooldownKey is the holder's MiscData key for one proc branch.
func procCooldownKey(itemId int, path string) string {
	return fmt.Sprintf("item_proc_cd_%d_%s", itemId, path)
}

func (d *ProcCooldownDecorator) Evaluate(ctx *EvalContext) Result {
	c := itemHolder(ctx)
	if c == nil {
		return Failure
	}
	key := procCooldownKey(ctx.Item.ItemId, d.Path)
	now := util.GetRoundCount()
	if until, ok := characters.MiscRound(c.GetMiscData(key)); ok && now < until {
		return Failure
	}
	result := d.Child.Evaluate(ctx)
	if result == Success && d.Rounds > 0 {
		c.SetMiscData(key, now+uint64(d.Rounds))
	}
	return result
}

// nodeDefNamesAction reports whether a tree names the action anywhere.
func nodeDefNamesAction(def NodeDef, action string) bool {
	if def.Do == action {
		return true
	}
	for _, ch := range def.Children {
		if nodeDefNamesAction(ch, action) {
			return true
		}
	}
	return def.Child != nil && nodeDefNamesAction(*def.Child, action)
}

// checkProcNodes refuses a proc node that would never fire or fire wrongly:
// one not under a proc event, an unknown effect, a param its effect does
// not read or that is not a number, and a `random` decorator over a proc
// outside 1 to 99 (at 100 the branch omits it, so nothing is drawn; X19).
// event is the nearest enclosing event.
func checkProcNodes(def NodeDef, event, path string) error {
	if def.Event != `` {
		event = def.Event
	}
	if def.Type == `decorator` && def.Mod == `random` && nodeDefNamesAction(def, `proc`) {
		if pct := getIntParam(def.Params, `percent`); pct < 1 || pct > 99 {
			return fmt.Errorf("%s: random percent %d over a proc: want 1 to 99 (omit the decorator at 100)", path, pct)
		}
	}
	if def.Do == `proc` {
		if !procEvents[event] {
			return fmt.Errorf("%s: proc under event %q: want one of on_hit, on_kill, on_block, on_grapple, on_spell_hit", path, event)
		}
		effect := getStringParam(def.Params, `effect`)
		allowed, ok := procEffectParams[effect]
		if !ok {
			return fmt.Errorf("%s: proc effect %q: want lifesteal, steal_pool, aoe_stun or apply_condition", path, effect)
		}
		for k, v := range cleanParams(def) {
			if k == `effect` {
				continue
			}
			if !allowed[k] {
				return fmt.Errorf("%s: proc effect %s does not read %q", path, effect, k)
			}
			switch v.(type) {
			case int, float64:
			default:
				return fmt.Errorf("%s: proc param %s %v: want a number", path, k, v)
			}
		}
	}
	for i, ch := range def.Children {
		if err := checkProcNodes(ch, event, fmt.Sprintf("%s.%d", path, i)); err != nil {
			return err
		}
	}
	if def.Child != nil {
		return checkProcNodes(*def.Child, event, path+".child")
	}
	return nil
}

// procLifesteal heals the attacker for ratio*damage, clamped to HealthMax.
// Returns the amount actually healed.
func procLifesteal(attacker *characters.Character, damage int, params map[string]float64) int {
	if attacker == nil {
		return 0
	}
	ratio := params["ratio"]
	if ratio <= 0 || damage <= 0 {
		return 0
	}
	amt := int(float64(damage) * ratio)
	if amt < 1 {
		amt = 1
	}
	return attacker.Heal(amt)
}

// procStealPool drains a pool from the target into the owner. Params:
// pool (3=conviction; 1=health/2=stamina reserved, unimplemented — YAGNI
// until an item needs them), amount_pct (fraction of the TARGET's pool
// max, capped by what they actually have). Executes only when something
// was actually stolen (so an empty-pool target does not burn the cooldown).
func procStealPool(owner, other *characters.Character, params map[string]float64) bool {
	if owner == nil || other == nil {
		return false
	}
	pct := params["amount_pct"]
	if pct <= 0 {
		return false
	}
	switch int(params["pool"]) {
	case 3: // conviction
		amt := int(float64(other.ConvictionMax.Value) * pct)
		if amt < 1 {
			amt = 1
		}
		if amt > other.Conviction {
			amt = other.Conviction
		}
		if amt <= 0 {
			return false
		}
		// A TRANSFER, not a restore: the owner may only absorb what was
		// actually drained, or conviction would be created or destroyed. amt
		// is still pre-clamped to the target's pool above so the drain and the
		// gain stay equal today.
		ownerRef := state.ActorRef{UserId: owner.GetUserId(), MobInstanceId: owner.MobInstanceId}
		drained := other.ApplyHarm(characters.PoolConviction, amt, ownerRef)
		owner.ApplyRestore(characters.PoolConviction, drained)
		return true
	}
	return false
}

// procApplyCondition applies the Bleeding record to the target. Params:
// condition (1=bleeding — the switch is the extension point for future
// condition ids; only bleeding is wired here, YAGNI), duration (the stack's
// rounds, default 4 if unset/<1), magnitude (per-round health loss, default 2
// if unset/<1). Each proc that fires adds one stack; see the Stacking flag.
// Unknown condition ids do not execute (so the branch's cooldown isn't
// armed).
func procApplyCondition(target *characters.Character, params map[string]float64) bool {
	if target == nil {
		return false
	}
	dur := int(params["duration"])
	if dur < 1 {
		dur = 4
	}
	mag := params["magnitude"]
	if mag < 1 {
		mag = 2
	}
	switch int(params["condition"]) {
	case 1:
		return target.AddConditionMagnitude(conditions.ConditionIdBleeding, dur, -mag, "itemproc") == nil
	}
	return false
}

// procAoeStun applies the stagger-stun condition (84 — a 1-round Stunned) to every
// hostile, stun-eligible mob in the owner's room. Non-combatants,
// attack-immune, and charmed mobs are never targeted — stunning someone's
// companion or a town NPC would be a prod incident. Returns true if
// at least one target was stunned (only then is the branch's cooldown armed).
//
// Mob owners are a no-op: no Stage-2 mob wields an aoe_stun item, and "hostile
// to a mob" has no clean definition here, so we return false (cooldown
// unburned) rather than guess. owner.GetUserId() is 0 for mobs.
//
// The stun_rounds param is intentionally IGNORED: condition 84 is a fixed 1-round
// stagger (triggercount:1 in its YAML) and cannot be duration-scaled from data
// without hacking condition internals. The Aegis of Mockery tunes its
// strength through its branch's chance and cooldown instead.
func procAoeStun(owner *characters.Character, room *rooms.Room, params map[string]float64) bool {
	if owner == nil {
		return false
	}
	ownerUserId := owner.GetUserId()
	if ownerUserId <= 0 {
		// Mob (or unassigned) owner — no-op, see doc comment.
		return false
	}

	if room == nil {
		room = rooms.LoadRoom(owner.RoomId)
	}
	if room == nil {
		return false
	}

	stunned := 0
	for _, mobId := range room.GetMobs(rooms.FindAll) {
		m := mobs.GetInstance(mobId)
		if m == nil {
			continue
		}
		// Spare non-combatants, attack-immune mobs, and ALL charmed
		// companions whoever their master is — a non-party bystander's
		// companion caught in the shockwave would be a prod incident just as
		// surely as a party member's. mobs.CheckPlayerHarm is the same policy
		// the player-cast HarmArea path applies in resolveSpell.
		if mobs.CheckPlayerHarm(m).Blocked() {
			continue
		}
		_ = m.Character.AddCondition(84, false)
		stunned++
	}

	if stunned == 0 {
		return false
	}

	// Room-wide narration, no raw numbers (project rule). CategorySubmission
	// matches condition 84's own submission-stagger flavor.
	//
	// Observer-only SendTrio, not a raw SendTextVisual. The line names nobody,
	// so this is not a leak fix: it is what lets CategorySubmission join
	// sendTrioOnlyCategories, since this was the category's last raw sender.
	// Both names are NoName because there is no actor and no actee to hide.
	messaging.SendTrio(messaging.Trio{
		// Room flavour with no participants: the shockwave is the condition's,
		// not any character's. Both personal roles are NoLine so the silence
		// reads as considered rather than forgotten.
		Actor: messaging.NoLine,
		Actee: messaging.NoLine,
		Observer: messaging.Say(messaging.CategorySubmission,
			`<ansi fg="yellow">A jarring shockwave ripples outward, staggering the hostile creatures nearby!</ansi>`),
	}, messaging.Audience{
		ActorName: messaging.NoName,
		ActeeName: messaging.NoName,
		Room:      room,
	})
	return true
}
````

**Create `internal/behaviortree/item_proc_test.go`:**

````go
package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The proc node (item behaviour slice 3, Rules 20 to 22).

const (
	procProbeItemId = 90611
	procProbeRoom   = 90612
	procProbeUserId = 90613
	procProbeMobId  = 90614
)

// procProbeTree: a 100% lifesteal on a hit with a 5-round cooldown, and a
// 50% one on a block with none.
const procProbeTree = `
tree:
  type: selector
  children:
    - type: decorator
      event: on_hit
      mod: cooldown
      rounds: 5
      child:
        type: action
        do: proc
        effect: lifesteal
        ratio: 0.5
    - type: decorator
      event: on_block
      mod: random
      percent: 50
      child:
        type: action
        do: proc
        effect: lifesteal
        ratio: 0.5
`

// seedProcWorld: one room, a hurt bearer in it, the probe tree, procs on.
func seedProcWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.ItemProcsEnabled = true
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		procProbeItemId: {ItemId: procProbeItemId, Name: "Probe Blade", Type: items.Weapon, Hands: 1, Behavior: "proc_probe"},
	}))
	LoadItemTreeForTest(t, "proc_probe", procProbeTree)
	t.Cleanup(ResetItemBTreeStatesForTest())
	room := &rooms.Room{RoomId: procProbeRoom}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{procProbeRoom: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(procProbeUserId, "kesh", "Kesh", 0)
	u.Character.RoomId = procProbeRoom
	u.Character.HealthMax.Value = 1000
	u.Character.Health = 100
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{procProbeUserId: u}))
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	return u
}

// fireProbe runs one item instance's tree for a proc event.
func fireProbe(it items.Item, userId int, event string, damage int) bool {
	return TryItemBehavior(EventContext{EventType: event, Proc: &ProcEvent{Damage: damage}},
		ItemSubject{UUID: it.UUID, ItemId: it.ItemId, UserId: userId, Slot: "weapon"})
}

func TestProcLifestealHealsTheHolder(t *testing.T) {
	u := seedProcWorld(t)
	if !fireProbe(items.New(procProbeItemId), u.UserId, "on_hit", 40) {
		t.Fatal("a 100% lifesteal on a 40-damage hit should succeed")
	}
	if u.Character.Health != 120 {
		t.Errorf("health %d, want 120 (half of 40 healed)", u.Character.Health)
	}
}

// A mob holder procs too: today's dispatcher fired a mob's weapon.
func TestProcFiresForAMobHolder(t *testing.T) {
	seedProcWorld(t)
	m := &mobs.Mob{InstanceId: procProbeMobId, Character: characters.Character{
		Name: "Probe Brute", RoomId: procProbeRoom, Conditions: conditions.New(),
	}}
	m.Character.HealthMax.Value = 1000
	m.Character.Health = 100
	mobs.SetInstanceForTest(procProbeMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(procProbeMobId, nil) })
	it := items.New(procProbeItemId)
	ok := TryItemBehavior(EventContext{EventType: "on_hit", Proc: &ProcEvent{Damage: 40}},
		ItemSubject{UUID: it.UUID, ItemId: it.ItemId, MobInstanceId: procProbeMobId, Slot: "weapon"})
	if !ok || m.Character.Health <= 100 {
		t.Errorf("mob holder: handled %v, health %d; want a heal", ok, m.Character.Health)
	}
}

// Ruling R1: the cooldown is per template and lives on the holder, so a
// second copy of the item waits it out, and so does a fresh instance after a
// relog (a new UUID, an empty item state, MiscData back from YAML as a
// float).
func TestProcCooldownIsSharedAndSurvivesARelog(t *testing.T) {
	u := seedProcWorld(t)
	first, second := items.New(procProbeItemId), items.New(procProbeItemId)
	if !fireProbe(first, u.UserId, "on_hit", 40) {
		t.Fatal("the first hit should proc")
	}
	key := procCooldownKey(procProbeItemId, "item.selector[0]")
	until, ok := characters.MiscRound(u.Character.GetMiscData(key))
	if !ok || until != 7005 {
		t.Fatalf("holder MiscData %s = %v, want 7005", key, u.Character.GetMiscData(key))
	}
	if fireProbe(second, u.UserId, "on_hit", 40) {
		t.Error("a second copy of the item fired inside the first's cooldown")
	}

	// The relog: item state gone, the round stored as YAML reads it back.
	ResetItemBTreeStatesForTest()
	u.Character.SetMiscData(key, float64(until))
	relogged := items.New(procProbeItemId)
	util.SetRoundCountForTest(7004)
	if fireProbe(relogged, u.UserId, "on_hit", 40) {
		t.Error("a relogged item fired inside the cooldown")
	}
	util.SetRoundCountForTest(7005)
	if !fireProbe(relogged, u.UserId, "on_hit", 40) {
		t.Error("the cooldown should be open at its stored round")
	}
}

// An effect that does nothing (no damage to drain) does not arm the
// cooldown, as markProcCooldown did not.
func TestProcNoOpDoesNotArmTheCooldown(t *testing.T) {
	u := seedProcWorld(t)
	it := items.New(procProbeItemId)
	if fireProbe(it, u.UserId, "on_hit", 0) {
		t.Fatal("a 0-damage lifesteal should not succeed")
	}
	if v := u.Character.GetMiscData(procCooldownKey(procProbeItemId, "item.selector[0]")); v != nil {
		t.Errorf("a no-op armed the cooldown: %v", v)
	}
	if !fireProbe(it, u.UserId, "on_hit", 40) {
		t.Error("the hit after a no-op should proc: the no-op armed a cooldown")
	}
}

// X19: a branch without a random decorator draws no number; one with draws
// exactly one.
func TestProcDrawsOnlyThroughItsRandomDecorator(t *testing.T) {
	u := seedProcWorld(t)
	draws := func(event string) int {
		restore := util.SetRandForTest(99)
		var want []int
		for i := 0; i < 4; i++ {
			want = append(want, util.Rand(1000))
		}
		restore()
		restore = util.SetRandForTest(99)
		defer restore()
		fireProbe(items.New(procProbeItemId), u.UserId, event, 40)
		got := util.Rand(1000)
		for i, w := range want {
			if got == w {
				return i
			}
		}
		t.Fatalf("%s: probe %d matches no early draw", event, got)
		return -1
	}
	if n := draws("on_hit"); n != 0 {
		t.Errorf("a 100%% proc drew %d numbers, want 0", n)
	}
	if n := draws("on_block"); n != 1 {
		t.Errorf("a 50%% proc drew %d numbers, want 1", n)
	}
}

func TestProcOffDoesNothing(t *testing.T) {
	u := seedProcWorld(t)
	cfg := configs.GetConfig()
	cfg.GamePlay.ItemProcsEnabled = false
	configs.SetConfigForTest(t, cfg)
	if fireProbe(items.New(procProbeItemId), u.UserId, "on_hit", 40) || u.Character.Health != 100 {
		t.Errorf("ItemProcsEnabled off: a proc fired (health %d)", u.Character.Health)
	}
}

func TestProcLoadChecks(t *testing.T) {
	branch := func(event, body string) string {
		return "tree:\n  type: selector\n  children:\n    - type: action\n      event: " + event + "\n      do: proc\n" + body
	}
	cases := []struct {
		name, yaml, want string
	}{
		{"idle event", branch("item_idle", "      effect: lifesteal\n"), `proc under event "item_idle"`},
		{"unknown effect", branch("on_hit", "      effect: explode\n"), `proc effect "explode"`},
		{"foreign param", branch("on_hit", "      effect: lifesteal\n      pool: 3\n"), `does not read "pool"`},
		{"non-number", branch("on_hit", "      effect: lifesteal\n      ratio: lots\n"), `want a number`},
		{"random 100", "tree:\n  type: decorator\n  event: on_hit\n  mod: random\n  percent: 100\n  child:\n    type: action\n    do: proc\n    effect: lifesteal\n", `random percent 100`},
	}
	for _, c := range cases {
		_, _, err := loadItemTreeDef([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
	if _, _, err := loadItemTreeDef([]byte(branch("on_kill", "      effect: lifesteal\n      ratio: 0.5\n"))); err != nil {
		t.Errorf("a well-formed on_kill proc was refused: %v", err)
	}
}

func TestProcRefusedOutsideAnItemTree(t *testing.T) {
	_, err := LoadTreeFromBytes([]byte("tree:\n  type: action\n  do: proc\n  effect: lifesteal\n"))
	if err == nil || !strings.Contains(err.Error(), "allowed only in an item tree") {
		t.Errorf("a mob tree named proc: err %v", err)
	}
}
````

**Modify `internal/behaviortree/loader.go`:**

````diff
@@ -111,6 +111,9 @@ func loadItemTreeDef(data []byte) (Node, *ItemVoice, error) {
 	if err := checkSpeakNodes(def.Tree, voice, itemRootLabel); err != nil {
 		return nil, nil, err
 	}
+	if err := checkProcNodes(def.Tree, ``, itemRootLabel); err != nil {
+		return nil, nil, err
+	}
 	node, err := compileNode(def.Tree, itemRootLabel)
 	if err != nil {
 		return nil, nil, err
@@ -148,6 +151,7 @@ var (
 		"pulse_light":     true,
 		"speak":           true,
 		"taunt_pull":      true,
+		"proc":            true,
 	}
 	// itemOnlyNodes need an item subject, so a mob or room tree may not
 	// name them.
@@ -161,6 +165,7 @@ var (
 		"hunger_overdue": true,
 		"speak":          true,
 		"taunt_pull":     true,
+		"proc":           true,
 	}
 )
 
@@ -287,6 +292,15 @@ func compileDecorator(def NodeDef, path string) (Node, error) {
 
 	switch def.Mod {
 	case "cooldown":
+		// A proc branch's cooldown lives on the holder, not the item's
+		// tree state (item behaviour slice 3, Rule 22).
+		if isItemTreePath(path) && nodeDefNamesAction(*def.Child, "proc") {
+			return &ProcCooldownDecorator{
+				Rounds: getIntParam(params, "rounds"),
+				Path:   path,
+				Child:  child,
+			}, nil
+		}
 		return &CooldownDecorator{
 			Rounds:   getIntParam(params, "rounds"),
 			StateKey: path + "_cooldown",
````

**Modify `internal/behaviortree/types.go`:**

````diff
@@ -31,6 +31,9 @@ type EventContext struct {
 	Command   string         // Command name for room command interception
 	Rest      string         // Command arguments
 	Direction string         // Direction for movement events
+	// Proc carries a proc event's participants into an item tree (item
+	// behaviour slice 3). Nil for every other event.
+	Proc *ProcEvent
 }
 
 // Node is the interface all behavior tree nodes implement.
````

- [ ] **Step 3: Tests pass, and the shared-cooldown test can fail**

```bash
go test ./internal/behaviortree -run TestProc -count=1 -v 2>&1 | grep -c "^--- PASS"
```

Expected: `8`. Null probe: change `if isItemTreePath(path) && nodeDefNamesAction(*def.Child, "proc") {` in `compileDecorator`'s `cooldown` case to `if false && ...`, rerun `-run TestProcCooldown`: it fails with `holder MiscData item_proc_cd_90611_item.selector[0] = <nil>, want 7005`. Restore the line and check `git diff internal/behaviortree/loader.go` shows only the intended hunks.

- [ ] **Step 4: Packages and root guards**

```bash
gofmt -l internal/behaviortree
go build ./...
go test ./internal/behaviortree -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: all `ok`. Without the two new `condition_apply_path_guard_test.go` keys, `TestPlayerConditionsTravelTheEventPath` names `actions_item_proc.go|258` and `|308`.

- [ ] **Step 5: Spec check and commit**

```bash
git diff 6036929bf -- internal/behaviortree/actions_item_proc.go internal/behaviortree/item_proc_test.go internal/behaviortree/actions.go internal/behaviortree/loader.go internal/behaviortree/types.go condition_apply_path_guard_test.go
git add internal/behaviortree/actions_item_proc.go internal/behaviortree/item_proc_test.go internal/behaviortree/actions.go internal/behaviortree/loader.go internal/behaviortree/types.go condition_apply_path_guard_test.go
git commit -m "feat(behaviortree): the proc node; a proc branch's cooldown lives on the holder" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The four items proc through their trees (Rule 21; S4)

Checkpoint `264f7765a`. Content and dispatch land together: the events vocabulary test (F14) fails as soon as a shipped tree names `on_hit` with no `EventType: "on_hit"` literal in Go, and the literals belong at the call sites. The proc branches: the Blackrazor's on_hit lifesteal (no `random`: 100%), the Aegis's on_block stun (`cooldown` 20 over `random` 10), two new trees for the Harness (`cooldown` 5 over `random` 50) and the Staff (`cooldown` 3, no `random`). The items keep their `procs:` keys until Task 4, so both paths can run against the record here. `fireItemProc` folds `procBearingItems` in and reads `ItemProcsEnabled` first; the six call sites switch to it with literals; the on_kill dispatch line goes (S4). `ProcRandomDecorator` joins `actions_item_proc.go` and the compiler, so a kill with procs off draws nothing. The parity test gains its tree path.

- [ ] **Step 1: Apply**

Edits to the `behaviors/items/*.yaml` and item files keep their CRLF working copies consistent if made with the Edit tool; `core.autocrlf` stores LF either way.

**Modify `_datafiles/world/dogmud/behaviors/items/aegis.yaml`:**

````diff
@@ -78,3 +78,15 @@ tree:
       event: on_kill
       do: speak
       pool: on_kill
+    - type: decorator
+      event: on_block
+      mod: cooldown
+      rounds: 20
+      child:
+        type: decorator
+        mod: random
+        percent: 10
+        child:
+          type: action
+          do: proc
+          effect: aoe_stun
````

**Modify `_datafiles/world/dogmud/behaviors/items/blackrazor.yaml`:**

````diff
@@ -103,3 +103,8 @@ tree:
       pool: on_hunger_feeding
       to: holder
       paced: false
+    - type: action
+      event: on_hit
+      do: proc
+      effect: lifesteal
+      ratio: 0.25
````

**Create `_datafiles/world/dogmud/behaviors/items/hollow_choir.yaml`:**

````yaml
# hollow_choir: the Staff of the Hollow Choir's hunger for conviction
# (40189; item behaviour slice 3, spec Rule 20). Every spell that lands
# drains a share of the target's conviction into the caster, at most once
# every 3 rounds. Always on a hit, so no random decorator: nothing is drawn
# (X19). The cooldown is the caster's, shared by every copy and kept across
# a relog (ruling R1).
notes: A choir of stolen will.
tree:
  type: decorator
  event: on_spell_hit
  mod: cooldown
  rounds: 3
  child:
    type: action
    do: proc
    effect: steal_pool
    pool: 3
    amount_pct: 0.08
````

**Create `_datafiles/world/dogmud/behaviors/items/thornwall_harness.yaml`:**

````yaml
# thornwall_harness: the Thornwall Harness's barbs (40186; item behaviour
# slice 3, spec Rule 20). In a grapple, whichever side the wearer is on, the
# barbs may open a bleeding wound on the other: half the time, at most once
# every 5 rounds. The cooldown is the wearer's, shared by every copy and kept
# across a relog (ruling R1).
notes: Barbs that punish anything that closes in.
tree:
  type: decorator
  event: on_grapple
  mod: cooldown
  rounds: 5
  child:
    type: decorator
    mod: random
    percent: 50
    child:
      type: action
      do: proc
      effect: apply_condition
      condition: 1
      duration: 10
      magnitude: 7
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40186-thornwall_harness.yaml`:**

````diff
@@ -27,6 +27,7 @@ procs:
       condition: 1
       duration: 10
       magnitude: 7
+behavior: thornwall_harness
 weight: 11.0
 rarity_tier: 82
 value: 30000
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40189-staff_of_the_hollow_choir.yaml`:**

````diff
@@ -33,6 +33,7 @@ procs:
     params:
       pool: 3
       amount_pct: 0.08
+behavior: hollow_choir
 weight: 4.0
 rarity_tier: 82
 value: 45000
````

**Modify `condition_apply_path_guard_test.go`:**

````diff
@@ -136,7 +136,7 @@ var conditionApplyPathAllowlist = map[string]string{
 	// ── mob holders: no client, so no line could reach anyone ───────────────
 	"internal/usercommands/character.go|423":         "the holder is a MOB (m.Character), and condition 99 is a perma-gear pin, not something a player reads",
 	"internal/hooks/item_procs.go|256":               "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
-	"internal/behaviortree/actions_item_proc.go|308": "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
+	"internal/behaviortree/actions_item_proc.go|324": "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
 	"internal/hooks/manifester_companions.go|40":     "the holder is a MOB (a summoned companion), not a player",
 
 	// ── secret conditions: silence is the authored intent ───────────────────
@@ -248,7 +248,7 @@ var conditionApplyPathAllowlist = map[string]string{
 	// case, and again Task 3 when it gained its heal case, and again Task 4
 	// when it gained its shield case, and again Task 5 when its doc comment
 	// grew and the per-pairing fallthrough became the default arm) ──────
-	"internal/hooks/spell_effects.go|321": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",
+	"internal/hooks/spell_effects.go|322": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",
 
 	// ── light spells (lighting plan 5a): an EVENT door, not the silent
 	// character door. applySpellCondition's target is a spellConditionTarget,
@@ -294,7 +294,7 @@ var conditionApplyPathAllowlist = map[string]string{
 	"internal/actions/combat_rake.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
 	"internal/actions/combat_throttle.go|146":        "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
 	"internal/hooks/item_procs.go|206":               "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
-	"internal/behaviortree/actions_item_proc.go|258": "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
+	"internal/behaviortree/actions_item_proc.go|274": "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
 }
 
 // primitivePackages define Character.AddCondition / Conditions.AddCondition
````

**Modify `internal/behaviortree/actions_item_proc.go`:**

````diff
@@ -119,6 +119,22 @@ func (d *ProcCooldownDecorator) Evaluate(ctx *EvalContext) Result {
 	return result
 }
 
+// ProcRandomDecorator is a `random` decorator over a proc branch: the
+// proc's chance. It draws only while ItemProcsEnabled is on, as the old
+// gate did, because a kill reaches a proc branch without the dispatcher
+// reading the switch first.
+type ProcRandomDecorator struct {
+	Percent int
+	Child   Node
+}
+
+func (d *ProcRandomDecorator) Evaluate(ctx *EvalContext) Result {
+	if !ItemProcsOn() || util.Rand(100) >= d.Percent {
+		return Failure
+	}
+	return d.Child.Evaluate(ctx)
+}
+
 // nodeDefNamesAction reports whether a tree names the action anywhere.
 func nodeDefNamesAction(def NodeDef, action string) bool {
 	if def.Do == action {
````

**Modify `internal/behaviortree/events.go`:**

````diff
@@ -21,9 +21,13 @@ var KnownBehaviorEvents = map[string]bool{
 	"mob_flee":               true, // the mob is fleeing
 	"mob_hurt":               true, // the mob took damage
 	"mob_idle":               true, // idle tick (out of combat)
+	"on_block":               true, // item trees: the bearer blocked with the offhand item (item behaviour slice 3)
 	"on_equip":               true, // item trees: the item was put on (item behaviour slice 2)
+	"on_grapple":             true, // item trees: a grapple round, either side of the hold, into the body armour
+	"on_hit":                 true, // item trees: the bearer's weapon hit
 	"on_hunger_feeding":      true, // item trees: a hungry weapon fed on its bearer, paced by HungerFeedingLineCooldownRounds
 	"on_kill":                true, // item trees: the bearer had a hand in a kill (every worn treed item)
+	"on_spell_hit":           true, // item trees: the bearer's spell hit, into the weapon
 	"on_unequip":             true, // item trees: the item was taken off
 	"packmate_hurt":          true, // a same-room routine-matched packmate was attacked
 	"player_ask":             true, // a player asked this mob about a topic
````

**Modify `internal/behaviortree/loader.go`:**

````diff
@@ -316,6 +316,14 @@ func compileDecorator(def NodeDef, path string) (Node, error) {
 			Child: child,
 		}, nil
 	case "random":
+		// A proc branch's chance draws nothing while procs are off; a kill
+		// reaches a proc branch without the dispatcher's gate (Rule 21).
+		if isItemTreePath(path) && nodeDefNamesAction(*def.Child, "proc") {
+			return &ProcRandomDecorator{
+				Percent: getIntParam(params, "percent"),
+				Child:   child,
+			}, nil
+		}
 		return &RandomDecorator{
 			Percent: getIntParam(params, "percent"),
 			Child:   child,
````

**Modify `internal/hooks/MobDeath_ItemProcs.go`:**

````diff
@@ -7,9 +7,10 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/util"
 )
 
-// MobDeathItemProcs fires on_kill procs, records the last-kill round (the
-// Blackrazor hunger anchor, Task 11) and fires on_kill into every worn
-// treed item, for every player with damage attribution on the kill.
+// MobDeathItemProcs records the last-kill round (the Blackrazor hunger
+// anchor, Task 11) and fires on_kill into every worn treed item, for every
+// player with damage attribution on the kill. The one event carries both
+// kill lines and on_kill procs (ruling S4).
 func MobDeathItemProcs(e events.Event) events.ListenerReturn {
 	evt, typeOk := e.(events.MobDeath)
 	if !typeOk {
@@ -21,12 +22,13 @@ func MobDeathItemProcs(e events.Event) events.ListenerReturn {
 			continue
 		}
 		user.Character.SetMiscData("pinnacle_last_kill_round", util.GetRoundCount())
-		dispatchItemProcs("on_kill", user.Character, nil, nil, 0)
 
 		// Every worn treed item hears of the kill (item behaviour slice 2,
 		// ruling S3: the shield's kill lines too, not the weapon's alone).
 		// Its speak node is paced by the item's cooldown, so a multi-kill
-		// round does not spam, and gated by PinnacleItemsEnabled.
+		// round does not spam, and gated by PinnacleItemsEnabled. An
+		// on_kill proc branch rides the same event, gated by
+		// ItemProcsEnabled (ruling S4: any worn item, not the weapon alone).
 		fireWornItemEvent(behaviortree.EventContext{EventType: "on_kill"}, user.Character, uid, 0)
 	}
 	return events.Continue
````

**Modify `internal/hooks/NewRound_DoCombat_unified.go`:**

````diff
@@ -158,12 +158,12 @@ func handleCombatRound(
 
 	// Pinnacle item procs: attacker's weapon on_hit. Fires for all four
 	// quadrants (player and mob attackers) — the point of hooking the unified
-	// orchestrator. Gated internally by ItemProcsEnabled + the per-proc
-	// chance/cooldown; a no-op when the attacker carries no proc weapon.
+	// orchestrator. Gated by ItemProcsEnabled, then the weapon's tree (its
+	// proc branch's chance and cooldown); a no-op when the weapon has no tree.
 	// Decision (U6 Task 14): res.Hit, not CleanHit, on purpose — on-hit procs
 	// read the damage actually dealt, and a deflected swing deals real damage.
 	if res.Hit {
-		dispatchItemProcs("on_hit", atk.GetCharacter(), def.GetCharacter(), atk.GetRoom(), res.DamageToTarget)
+		fireItemProc(behaviortree.EventContext{EventType: "on_hit"}, atk.GetCharacter(), def.GetCharacter(), atk.GetRoom(), res.DamageToTarget)
 	}
 
 	// Pinnacle item procs: defender's shield on_block. A "successful block" in
@@ -177,7 +177,7 @@ func handleCombatRound(
 	// CRITS only. rollCombatAttack has already resolved defense into res by
 	// this point, so DefenseUsed is populated.
 	if res.DefenseUsed == combatvocab.DefenceBlock {
-		dispatchItemProcs("on_block", def.GetCharacter(), atk.GetCharacter(), atk.GetRoom(), onBlockProcDamage(res))
+		fireItemProc(behaviortree.EventContext{EventType: "on_block"}, def.GetCharacter(), atk.GetCharacter(), atk.GetRoom(), onBlockProcDamage(res))
 	}
 
 	// Combat analytics (shared across all four quadrants).
````

**Modify `internal/hooks/Position_GrappleTick.go`:**

````diff
@@ -26,6 +26,7 @@ import (
 	"math"
 	"sync"
 
+	"github.com/GoMudEngine/GoMud/internal/behaviortree"
 	"github.com/GoMudEngine/GoMud/internal/characters"
 	"github.com/GoMudEngine/GoMud/internal/combat"
 	"github.com/GoMudEngine/GoMud/internal/configs"
@@ -405,8 +406,8 @@ func processGrapplePairWithContest(
 	// directly off outcome.Kind instead of re-deriving "did they stay
 	// grappling" from the pre-roll state.
 	if outcome.Kind != position.OutcomeEscape {
-		dispatchItemProcs("on_grapple", controller, controlled, nil, 0)
-		dispatchItemProcs("on_grapple", controlled, controller, nil, 0)
+		fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, controller, controlled, nil, 0)
+		fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, controlled, controller, nil, 0)
 	}
 
 	fireStaminaWarningIfLow(controller)
````

**Create `internal/hooks/item_proc_dispatch.go`:**

````go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// fireItemProc runs a proc event into the one worn item it reaches (item
// behaviour slice 3, Rule 21): a hit or a spell hit reaches the weapon, a
// block the offhand, a grapple the body armour (spec P3). owner is whoever
// the event belongs to, other the opponent, room nil where the caller does
// not know it. A kill does not come through here: it reaches every worn
// treed item through MobDeathItemProcs (ruling S4).
//
// ItemProcsEnabled is read first, so a disabled proc draws no number, and
// an item without a tree costs one spec lookup.
func fireItemProc(event behaviortree.EventContext, owner, other *characters.Character, room *rooms.Room, damage int) {
	if owner == nil || !behaviortree.ItemProcsOn() {
		return
	}
	var it items.Item
	var slot string
	switch event.EventType {
	case "on_hit", "on_spell_hit":
		it, slot = owner.Equipment.Weapon, "weapon"
	case "on_block":
		it, slot = owner.Equipment.Offhand, "offhand"
	case "on_grapple":
		it, slot = owner.Equipment.Body, "body"
	default:
		return
	}
	if it.ItemId <= 0 || !it.HasBehavior() {
		return
	}
	event.Proc = &behaviortree.ProcEvent{Other: other, Room: room, Damage: damage}
	fireItemEvent(event, it, slot, owner, owner.GetUserId(), owner.MobInstanceId)
}
````

**Modify `internal/hooks/item_proc_parity_test.go`:**

````diff
@@ -49,6 +49,9 @@ type procDispatch func(trigger string, owner, other *characters.Character, room
 // procParityPaths are the dispatchers the record is checked against.
 var procParityPaths = map[string]procDispatch{
 	"pinnacle": dispatchItemProcs,
+	"tree": func(trigger string, owner, other *characters.Character, room *rooms.Room, damage int) {
+		fireItemProc(behaviortree.EventContext{EventType: trigger}, owner, other, room, damage)
+	},
 }
 
 // loadProcParityWorld loads the shipped conditions and items, points the
````

**Modify `internal/hooks/spell_effects.go`:**

````diff
@@ -4,6 +4,7 @@ import (
 	"fmt"
 
 	"github.com/GoMudEngine/GoMud/internal/actions"
+	"github.com/GoMudEngine/GoMud/internal/behaviortree"
 	"github.com/GoMudEngine/GoMud/internal/characters"
 	"github.com/GoMudEngine/GoMud/internal/combat"
 	"github.com/GoMudEngine/GoMud/internal/conditions"
@@ -241,7 +242,7 @@ func applySpellDamage(c spellEffectCtx) int {
 		// on_spell_hit item procs fire only on a harm hit that dealt damage;
 		// the proc's own chance and cooldown pace an area cast.
 		if dmg > 0 {
-			dispatchItemProcs("on_spell_hit", c.casterChar, tc, nil, dmg)
+			fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, c.casterChar, tc, nil, dmg)
 		}
 	}
 	commitHarmfulSpellAggro(c, fresh)
@@ -354,7 +355,7 @@ func applySpellKnockdown(c spellEffectCtx) int {
 		tc.ApplyHarm(characters.PoolHealth, dmg, c.casterRef())
 		cancelDamageConditions(tc)
 		if dmg > 0 {
-			dispatchItemProcs("on_spell_hit", c.casterChar, tc, nil, dmg)
+			fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, c.casterChar, tc, nil, dmg)
 		}
 	}
 	// Spell knockdowns put the target on its back (Supine); a target already
````

- [ ] **Step 2: Both paths match the record, and the record can catch the tree**

```bash
go build ./...
go test ./internal/hooks -run TestItemProcParity -count=1
```

Expected: `ok` (both `pinnacle` and `tree` paths). Null probe: in `hollow_choir.yaml` change `  rounds: 3` to `  rounds: 2`, rerun: `tree path moved off the record.`; restore.

- [ ] **Step 3: Packages and root guards**

```bash
gofmt -l internal/
go vet ./internal/hooks ./internal/behaviortree
go test ./internal/behaviortree ./internal/hooks ./internal/items -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: all `ok`. The guard keys move with this task's edits: `actions_item_proc.go|258` to `|274`, `|308` to `|324` (`ProcRandomDecorator` sits above them), `spell_effects.go|321` to `|322` (its new import).

- [ ] **Step 4: Spec check and commit**

```bash
P="_datafiles/world/dogmud/behaviors/items/aegis.yaml _datafiles/world/dogmud/behaviors/items/blackrazor.yaml _datafiles/world/dogmud/behaviors/items/thornwall_harness.yaml _datafiles/world/dogmud/behaviors/items/hollow_choir.yaml _datafiles/world/dogmud/items/materials-40000/40186-thornwall_harness.yaml _datafiles/world/dogmud/items/materials-40000/40189-staff_of_the_hollow_choir.yaml internal/behaviortree/events.go internal/behaviortree/loader.go internal/behaviortree/actions_item_proc.go internal/hooks/item_proc_dispatch.go internal/hooks/NewRound_DoCombat_unified.go internal/hooks/Position_GrappleTick.go internal/hooks/spell_effects.go internal/hooks/MobDeath_ItemProcs.go internal/hooks/item_proc_parity_test.go condition_apply_path_guard_test.go"
git diff 264f7765a -- $P
git add $P
git commit -m "feat: the four proc items proc through their item trees" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Retire the Pinnacle proc path (Rule 23; X21)

Checkpoint `59af9ebd1`. Delete `item_procs.go` and its test; the effect tests move to `behaviortree/item_proc_effects_test.go` (same assertions, own fixtures), the dispatch tests become `hooks/item_proc_dispatch_test.go` driven through `fireItemProc` with a probe tree, which also proves the switch-off draws nothing on any event including a kill, and that a kill reaches an offhand on_kill proc (S4). `readMiscRound` callers use `characters.MiscRound`. The parity test drops the old path and its record switch (the record is frozen). `ItemProc`, `Procs`, its validation, `ProcsFor` and `proc_accessors.go` go; delete the field first and let `go build` and `go vet` list the consumers (they are exactly the files below). The builder loses its proc rows, enums and DOM helpers. The `procs:` blocks leave the four item files. The guards drop `item_procs.go` keys and the `Procs` classification. The two `send_trio_only_guard_test.go` comments naming `item_procs.go:275` are dated survey records that say they stay as written: leave them.

- [ ] **Step 1: Apply**

**Modify `_datafiles/html/public/static/js/items.js`:**

````diff
@@ -3,8 +3,9 @@
  * items.js — the item-template editor (admin web-building 2), a second mode of
  * the /build page. Consumes Build.Items (list) + Build.Item (detail) GMCP and
  * drives Build.Item.Create/Update/Delete. The form morphs by item type; fields
- * the form doesn't cover (procs, hunger, the behaviour tree) round-trip untouched
- * because the server rebuilds the spec from the loaded copy.
+ * the form doesn't cover (the behaviour tree, which carries an item's voice and
+ * procs) round-trip untouched because the server rebuilds the spec from the
+ * loaded copy.
  */
 (function () {
   var ARMOR_SLOTS = ["offhand", "head", "neck", "body", "belt", "gloves", "ring",
@@ -79,25 +80,6 @@
   function gmcp(pkg, obj) { if (window.Builder && window.Builder.sendGMCP) window.Builder.sendGMCP(pkg, obj); }
   function toast(m, e) { if (window.Builder && window.Builder.toast) window.Builder.toast(m, e); }
 
-  // Bare DOM builders used by the proc-row editor (which lives outside the
-  // renderForm closure that owns numField/selectField).
-  function selBox(opts, val) {
-    var s = document.createElement("select");
-    opts.forEach(function (o) {
-      var op = document.createElement("option"); op.value = o; op.textContent = o === "" ? "(none)" : o;
-      if (o === val) op.selected = true; s.appendChild(op);
-    });
-    return s;
-  }
-  function numBox(val, step) {
-    var i = document.createElement("input"); i.type = "number"; i.step = step || "1";
-    i.value = (val === 0 ? "0" : (val || "")); return i;
-  }
-  function labelWrap(text, input) {
-    var d = document.createElement("div"); d.style.flex = "1 1 auto"; d.style.minWidth = "0";
-    var l = document.createElement("label"); l.textContent = text; d.appendChild(l); d.appendChild(input); return d;
-  }
-
   var Panel = {
     rows: [],
     search: "",
@@ -330,7 +312,8 @@
     function rerenderTypeSections(type) { self.buildTypeSections(self.typeSections, type, detail, F, markDirty, field, numField, textField, checkField, selectField, hintFor); }
     rerenderTypeSections(detail.type);
 
-    // Advanced (sentient & procs) — collapsible, auto-open when populated.
+    // Advanced (reserves, hunger, mutation drip, worn conditions): collapsible,
+    // auto-open when populated.
     this.buildAdvancedSection(insp, detail, F, markDirty, field, numField, textField, checkField, selectField, hintFor);
 
     // Save + delete row
@@ -424,8 +407,7 @@
 
   Panel.buildAdvancedSection = function (insp, detail, F, markDirty, field, numField, textField, checkField, selectField, hintFor) {
     var self = this;
-    var hasAdv = (detail.procs && detail.procs.length) ||
-      detail.reserveHealthPct || detail.reserveStaminaPct || detail.reserveConvictionPct ||
+    var hasAdv = detail.reserveHealthPct || detail.reserveStaminaPct || detail.reserveConvictionPct ||
       detail.hungerRounds || detail.hungerDrainPct ||
       detail.mutationTickInterval || detail.mutationTickChance || detail.mutationRarityFloor ||
       (detail.wornConditionIds && detail.wornConditionIds.length);
@@ -433,7 +415,7 @@
     // author's toggle across same-item re-renders (e.g. the post-save re-Get).
     if (detail.itemId !== this._advItemId) { this.advancedOpen = !!hasAdv; this._advItemId = detail.itemId; }
 
-    function headText() { return (self.advancedOpen ? "▾ " : "▸ ") + "Advanced — sentient & procs"; }
+    function headText() { return (self.advancedOpen ? "▾ " : "▸ ") + "Advanced: reserves, hunger, mutation"; }
     var head = ce("h3", { text: headText() });
     head.style.cursor = "pointer";
     var body = ce("div", {});
@@ -446,8 +428,8 @@
     insp.appendChild(head);
     insp.appendChild(body);
 
-    this.buildProcEditor(body, detail, F, markDirty);
-
+    // An item's procs live in its behaviour tree (behaviors/items/) since item
+    // behaviour slice 3; tree editing is #367.
     body.appendChild(sectionTitle("Reserves"));
     body.appendChild(ce("div", { "class": "row" }, [
       numField("Reserve HP", "reserveHealthPct", detail.reserveHealthPct, "0.05"),
@@ -475,63 +457,6 @@
     body.appendChild(field("Worn condition ids", wb, hintFor("wornConditionIds", false)));
   };
 
-  Panel.buildProcEditor = function (body, detail, F, markDirty) {
-    body.appendChild(sectionTitle("Procs"));
-    var procBox = ce("div", {});
-    body.appendChild(procBox);
-    var procRows = [];
-
-    function addProc(p) {
-      p = p || { trigger: "", effect: "", chance: 100, cooldownRounds: 0, params: {} };
-      var trig = selBox([""].concat(detail.procTriggers || []), p.trigger);
-      var eff = selBox([""].concat(detail.procEffects || []), p.effect);
-      var chance = numBox(p.chance, "1"); chance.style.width = "60px";
-      var cd = numBox(p.cooldownRounds, "1"); cd.style.width = "60px";
-      var rm = ce("button", { "class": "mini rm", text: "✕ proc" });
-
-      var paramBox = ce("div", { style: "margin:3px 0 3px 10px;" });
-      var paramRows = [];
-      function addParam(k, v) {
-        var name = ce("input", { type: "text", placeholder: "param" }); name.value = k || ""; name.style.flex = "1";
-        var val = ce("input", { type: "number", step: "0.05" }); val.value = (v === 0 ? "0" : (v || "")); val.style.width = "70px";
-        var prm = ce("button", { "class": "mini rm", text: "✕" });
-        var prow = ce("div", { "class": "kv" }, [name, val, prm]);
-        name.addEventListener("input", markDirty); val.addEventListener("input", markDirty);
-        prm.addEventListener("click", function () { paramBox.removeChild(prow); paramRows.splice(paramRows.indexOf(prow), 1); markDirty(); });
-        prow._name = name; prow._val = val; paramRows.push(prow); paramBox.appendChild(prow);
-      }
-      Object.keys(p.params || {}).forEach(function (k) { addParam(k, p.params[k]); });
-      var addParamBtn = ce("button", { "class": "mini", text: "+ param" });
-      addParamBtn.addEventListener("click", function () { addParam("", 0); markDirty(); });
-
-      var row = ce("div", { style: "border:1px solid var(--tooled);border-radius:4px;padding:6px;margin:4px 0;" }, [
-        ce("div", { "class": "row" }, [labelWrap("Trigger", trig), labelWrap("Effect", eff)]),
-        ce("div", { "class": "row" }, [labelWrap("Chance", chance), labelWrap("Cooldown", cd)]),
-        ce("div", {}, [ce("label", { text: "Params (e.g. ratio 0.25)" }), paramBox, addParamBtn]),
-        rm
-      ]);
-      trig.addEventListener("change", markDirty); eff.addEventListener("change", markDirty);
-      chance.addEventListener("input", markDirty); cd.addEventListener("input", markDirty);
-      rm.addEventListener("click", function () { procBox.removeChild(row); procRows.splice(procRows.indexOf(row), 1); markDirty(); });
-      row._get = function () {
-        var params = {};
-        paramRows.forEach(function (pr) { var n = pr._name.value.trim(); if (n) params[n] = parseFloat(pr._val.value) || 0; });
-        return { trigger: trig.value, effect: eff.value, chance: parseInt(chance.value, 10) || 0, cooldownRounds: parseInt(cd.value, 10) || 0, params: params };
-      };
-      procRows.push(row); procBox.appendChild(row);
-    }
-
-    (detail.procs || []).forEach(addProc);
-    var addBtn = ce("button", { "class": "mini", text: "+ proc" });
-    addBtn.addEventListener("click", function () { addProc(); markDirty(); });
-    body.appendChild(addBtn);
-
-    F.procs = function () {
-      return procRows.map(function (r) { return r._get(); })
-        .filter(function (p) { return p.trigger || p.effect; });
-    };
-  };
-
   // ---- mutations ----
   Panel.gather = function () {
     var F = this.fields || {};
@@ -556,7 +481,6 @@
       bottleAgingMultiplier: g("bottleAgingMultiplier", 0), isBandolier: g("isBandolier", false), bandolierCapacity: g("bandolierCapacity", 0),
       isComponent: g("isComponent", false), componentTag: g("componentTag", ""), weightReduction: g("weightReduction", 0),
       bagCapacity: g("bagCapacity", 0), salvageReturns: g("salvageReturns", []), keyLockId: g("keyLockId", ""),
-      procs: g("procs", []),
       reserveHealthPct: g("reserveHealthPct", 0), reserveStaminaPct: g("reserveStaminaPct", 0), reserveConvictionPct: g("reserveConvictionPct", 0),
       hungerRounds: g("hungerRounds", 0), hungerDrainPct: g("hungerDrainPct", 0),
       mutationTickInterval: g("mutationTickInterval", 0), mutationTickChance: g("mutationTickChance", 0), mutationRarityFloor: g("mutationRarityFloor", 0),
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml`:**

````diff
@@ -28,12 +28,6 @@ staminacost: 9
 reserve_health_pct: 0.25
 hunger_rounds: 50
 hunger_drain_pct: 0.01
-procs:
-  - trigger: on_hit
-    chance: 100
-    effect: lifesteal
-    params:
-      ratio: 0.25
 behavior: blackrazor
 statmods:
   strength: 6
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40185-aegis_of_mockery.yaml`:**

````diff
@@ -17,11 +17,6 @@ blockrating: 30
 physical_mitigation: 14
 magical_mitigation: 10
 conviction_mitigation: 8
-procs:
-  - trigger: on_block
-    chance: 10
-    cooldown_rounds: 20
-    effect: aoe_stun
 behavior: aegis
 weight: 8.0
 rarity_tier: 82
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40186-thornwall_harness.yaml`:**

````diff
@@ -18,15 +18,6 @@ conviction_mitigation: 6
 statmods:
   strength: 6
   vitality: 6
-procs:
-  - trigger: on_grapple
-    chance: 50
-    cooldown_rounds: 5
-    effect: apply_condition
-    params:
-      condition: 1
-      duration: 10
-      magnitude: 7
 behavior: thornwall_harness
 weight: 11.0
 rarity_tier: 82
````

**Modify `_datafiles/world/dogmud/items/materials-40000/40189-staff_of_the_hollow_choir.yaml`:**

````diff
@@ -25,14 +25,6 @@ statmods:
   casting: 10
   spellcasting: 5
   manifestation: 5
-procs:
-  - trigger: on_spell_hit
-    chance: 100
-    cooldown_rounds: 3
-    effect: steal_pool
-    params:
-      pool: 3
-      amount_pct: 0.08
 behavior: hollow_choir
 weight: 4.0
 rarity_tier: 82
````

**Modify `condition_apply_path_guard_test.go`:**

````diff
@@ -135,7 +135,6 @@ var conditionApplyPathAllowlist = map[string]string{
 
 	// ── mob holders: no client, so no line could reach anyone ───────────────
 	"internal/usercommands/character.go|423":         "the holder is a MOB (m.Character), and condition 99 is a perma-gear pin, not something a player reads",
-	"internal/hooks/item_procs.go|256":               "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
 	"internal/behaviortree/actions_item_proc.go|324": "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
 	"internal/hooks/manifester_companions.go|40":     "the holder is a MOB (a summoned companion), not a player",
 
@@ -293,7 +292,6 @@ var conditionApplyPathAllowlist = map[string]string{
 	"internal/actions/combat_maul.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
 	"internal/actions/combat_rake.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
 	"internal/actions/combat_throttle.go|146":        "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
-	"internal/hooks/item_procs.go|206":               "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
 	"internal/behaviortree/actions_item_proc.go|274": "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
 }
 
````

**Create `internal/behaviortree/item_proc_effects_test.go`:**

````go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The four proc effects (item behaviour slice 3, Rule 20), moved unchanged
// from internal/hooks/item_procs.go with their tests.

const (
	effectRoom      = 90621
	effectEmptyRoom = 90622
)

// seedStunWorld registers condition 84 (the 1-round stagger-Stun) and two
// rooms, effectRoom and an empty effectEmptyRoom.
func seedStunWorld(t *testing.T) *rooms.Room {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		84: {
			ConditionId:   84,
			Name:          "Stunned",
			Description:   "Reeling — no meaningful attack or defense this round.",
			RoundInterval: 1,
			TriggerCount:  1,
		},
	}))
	room := &rooms.Room{RoomId: effectRoom}
	empty := &rooms.Room{RoomId: effectEmptyRoom}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{effectRoom: room, effectEmptyRoom: empty}, map[string]*rooms.ZoneConfig{}))
	return room
}

// addEffectMob registers a mob instance in room for aoe_stun targeting.
func addEffectMob(t *testing.T, room *rooms.Room, instanceId int, nonCombatant bool) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		InstanceId: instanceId,
		HomeRoomId: room.RoomId,
		Character: characters.Character{
			Name:         "Test Beast",
			RoomId:       room.RoomId,
			NonCombatant: nonCombatant,
			Conditions:   conditions.New(),
			Cooldowns:    map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 50
	m.Character.Health = 50
	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instanceId, nil) })
	room.AddMob(instanceId)
	return m
}

// effectOwner is a player character (user id 1) in room.
func effectOwner(room int) *characters.Character {
	c := characters.New()
	c.SetUserId(1)
	c.RoomId = room
	return c
}

func TestProcAoeStun_StunsHostilesSkipsProtected(t *testing.T) {
	room := seedStunWorld(t)
	hostileA := addEffectMob(t, room, 90631, false)
	hostileB := addEffectMob(t, room, 90632, false)
	nonCombatant := addEffectMob(t, room, 90633, true)

	if ok := procAoeStun(effectOwner(effectRoom), room, map[string]float64{}); !ok {
		t.Fatal("aoe_stun should execute (true) with hostile mobs present")
	}
	if !hostileA.Character.HasCondition(84) || !hostileB.Character.HasCondition(84) {
		t.Error("both hostile mobs should be stunned (condition 84)")
	}
	if nonCombatant.Character.HasCondition(84) {
		t.Error("the non-combatant must NOT be stunned")
	}
}

// Charmed mobs are spared whoever their master is: the owner's AND a
// bystander's (player-cast HarmArea parity).
func TestProcAoeStun_SkipsCharmedCompanions(t *testing.T) {
	room := seedStunWorld(t)
	hostile := addEffectMob(t, room, 90631, false)
	ownerCompanion := addEffectMob(t, room, 90632, false)
	ownerCompanion.Character.Charm(1, characters.CharmPermanent, "")
	bystanderCompanion := addEffectMob(t, room, 90633, false)
	bystanderCompanion.Character.Charm(2, characters.CharmPermanent, "")

	if ok := procAoeStun(effectOwner(effectRoom), room, map[string]float64{}); !ok {
		t.Fatal("aoe_stun should execute with a hostile present")
	}
	if !hostile.Character.HasCondition(84) {
		t.Error("the hostile mob should be stunned")
	}
	if ownerCompanion.Character.HasCondition(84) || bystanderCompanion.Character.HasCondition(84) {
		t.Error("charmed companions must NOT be stunned")
	}
}

// An empty room does not execute, so the branch's cooldown is not armed.
func TestProcAoeStun_EmptyRoomReturnsFalse(t *testing.T) {
	seedStunWorld(t)
	if procAoeStun(effectOwner(effectEmptyRoom), nil, map[string]float64{}) {
		t.Fatal("aoe_stun in an empty room should return false")
	}
}

// A mob owner (GetUserId() == 0) stuns nothing and returns false.
func TestProcAoeStun_MobOwnerIsNoOp(t *testing.T) {
	room := seedStunWorld(t)
	hostile := addEffectMob(t, room, 90631, false)
	if procAoeStun(characters.New(), room, map[string]float64{}) {
		t.Fatal("aoe_stun from a mob owner should return false")
	}
	if hostile.Character.HasCondition(84) {
		t.Error("mob-owner aoe_stun must not stun anything")
	}
}

func TestProcLifesteal(t *testing.T) {
	attacker := characters.New()
	attacker.HealthMax.Value = 200
	attacker.Health = 100
	if healed := procLifesteal(attacker, 80, map[string]float64{"ratio": 0.25}); healed != 20 {
		t.Fatalf("expected 20 healed (25%% of 80), got %d", healed)
	}
	if attacker.Health != 120 {
		t.Fatalf("expected health 120, got %d", attacker.Health)
	}
	attacker.Health = 195
	procLifesteal(attacker, 80, map[string]float64{"ratio": 0.25})
	if attacker.Health != 200 {
		t.Fatalf("expected clamp at 200, got %d", attacker.Health)
	}
}

func TestProcApplyCondition_Bleed(t *testing.T) {
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	target := characters.New()
	if !procApplyCondition(target, map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("apply_condition should execute")
	}
	if got := target.Conditions.TriggersLeft(conditions.ConditionIdBleeding); got != 6 {
		t.Fatalf("expected 6: duration is the stack's rounds and the record ticks every round, got %d", got)
	}
	held := target.GetConditions(conditions.ConditionIdBleeding)
	if len(held) != 1 || held[0].Magnitude != -12 {
		t.Fatalf("expected one Bleeding record at magnitude -12, got %+v", held)
	}
	if len(held[0].Stacks) != 1 || held[0].Stacks[0].RoundsLeft != 6 || held[0].Stacks[0].Amount != -12 {
		t.Fatalf("expected one stack of 6 rounds at -12, got %+v", held[0].Stacks)
	}
}

// With the Bleeding spec absent the add fails, and procApplyCondition must
// report it rather than claim success, or the branch's cooldown would be
// armed for a bleed that never landed. Null probe: making case 1 return
// true unconditionally turns this red.
func TestProcApplyCondition_BleedSpecMissing_ReturnsFalse(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	if procApplyCondition(characters.New(), map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("procApplyCondition must return false when the Bleeding spec is missing")
	}
}

func TestProcApplyCondition_NilAndUnknown(t *testing.T) {
	if procApplyCondition(nil, map[string]float64{"condition": 1}) {
		t.Fatal("nil target must not execute")
	}
	if procApplyCondition(characters.New(), map[string]float64{"condition": 99}) {
		t.Fatal("unknown condition id must not execute")
	}
}

func TestProcStealPool_Conviction(t *testing.T) {
	caster := characters.New()
	caster.ConvictionMax.Value = 100
	caster.Conviction = 40
	target := characters.New()
	target.ConvictionMax.Value = 100
	target.Conviction = 50
	if !procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.10}) {
		t.Fatal("steal_pool should execute")
	}
	if target.Conviction != 40 || caster.Conviction != 50 {
		t.Fatalf("10%% of the target's max moves over: target=%d caster=%d, want 40 and 50", target.Conviction, caster.Conviction)
	}
	// clamps: target at 0, caster at max
	target.Conviction = 3
	caster.Conviction = 95
	procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.10})
	if target.Conviction != 0 || caster.Conviction != 98 {
		t.Fatalf("clamps wrong: target=%d caster=%d", target.Conviction, caster.Conviction)
	}
}

func TestProcStealPool_Guards(t *testing.T) {
	caster := characters.New()
	target := characters.New()
	if procStealPool(nil, target, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("nil owner must not execute")
	}
	if procStealPool(caster, nil, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("nil target must not execute")
	}
	if procStealPool(caster, target, map[string]float64{"pool": 3}) {
		t.Fatal("missing amount_pct must not execute")
	}
	target.Conviction = 0
	target.ConvictionMax.Value = 100
	if procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("empty target pool must not execute")
	}
	if procStealPool(caster, target, map[string]float64{"pool": 2, "amount_pct": 0.1}) {
		t.Fatal("unimplemented pool ids must not execute")
	}
}
````

**Create `internal/hooks/item_proc_dispatch_test.go`:**

````go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Proc dispatch (item behaviour slice 3, Rule 21): which worn item each
// proc event reaches, the ItemProcsEnabled gate, and the kill (ruling S4).

const (
	dispatchProbeWeapon  = 999920
	dispatchProbeOffhand = 999921
	dispatchProbeBody    = 999922
)

// dispatchProbeTree procs on every proc event; the kill branch has a chance,
// so a draw can be seen.
const dispatchProbeTree = `
tree:
  type: selector
  children:
    - type: action
      event: on_hit
      do: proc
      effect: lifesteal
      ratio: 0.5
    - type: action
      event: on_block
      do: proc
      effect: lifesteal
      ratio: 0.5
    - type: action
      event: on_grapple
      do: proc
      effect: apply_condition
      condition: 1
      duration: 6
      magnitude: 12
    - type: action
      event: on_spell_hit
      do: proc
      effect: steal_pool
      pool: 3
      amount_pct: 0.10
    - type: decorator
      event: on_kill
      mod: random
      percent: 50
      child:
        type: action
        do: proc
        effect: aoe_stun
`

// seedDispatchProbe seeds the registries (users 1 and 2 and hostile mob 100
// in room 1), three probe items sharing the probe tree, procs on.
func seedDispatchProbe(t *testing.T) (alice, bob *characters.Character) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	setItemProcsEnabled(t, true)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		dispatchProbeWeapon:  {ItemId: dispatchProbeWeapon, Name: "probe blade", Type: items.Weapon, Hands: 1, Behavior: "dispatch_probe"},
		dispatchProbeOffhand: {ItemId: dispatchProbeOffhand, Name: "probe shield", Type: items.Offhand, Behavior: "dispatch_probe"},
		dispatchProbeBody:    {ItemId: dispatchProbeBody, Name: "probe harness", Type: items.Body, Behavior: "dispatch_probe"},
	}))
	behaviortree.LoadItemTreeForTest(t, "dispatch_probe", dispatchProbeTree)
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	alice, bob = users.GetByUserId(1).Character, users.GetByUserId(2).Character
	for _, c := range []*characters.Character{alice, bob} {
		c.HealthMax.Value = 200
		c.Health = 100
		c.ConvictionMax.Value = 100
		c.Conviction = 50
	}
	return alice, bob
}

func TestFireItemProc_HitAndBlockReachTheirSlot(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)

	// A block reaches the offhand, which is empty: nothing heals.
	fireItemProc(behaviortree.EventContext{EventType: "on_block"}, alice, bob, nil, 80)
	if alice.Health != 100 {
		t.Fatalf("a block reached the weapon: health %d", alice.Health)
	}
	fireItemProc(behaviortree.EventContext{EventType: "on_hit"}, alice, bob, nil, 80)
	if alice.Health != 140 {
		t.Fatalf("on_hit lifesteal: health %d, want 140", alice.Health)
	}
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	fireItemProc(behaviortree.EventContext{EventType: "on_block"}, alice, bob, nil, 20)
	if alice.Health != 150 {
		t.Fatalf("on_block lifesteal from the offhand: health %d, want 150", alice.Health)
	}
}

// A grapple fires for both sides; each reaches its own body armour and
// wounds the other.
func TestFireItemProc_GrappleReachesTheBody(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	alice.Equipment.Body = items.New(dispatchProbeBody)

	fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, alice, bob, nil, 0)
	fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, bob, alice, nil, 0)
	if !bob.HasCondition(conditions.ConditionIdBleeding) {
		t.Error("the harness wearer's opponent should be bleeding")
	}
	if alice.HasCondition(conditions.ConditionIdBleeding) {
		t.Error("the opponent wears no harness; the wearer should not bleed")
	}
}

func TestFireItemProc_SpellHitReachesTheWeapon(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)
	fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, alice, bob, nil, 25)
	if bob.Conviction != 40 || alice.Conviction != 60 {
		t.Fatalf("steal_pool: target %d caster %d, want 40 and 60", bob.Conviction, alice.Conviction)
	}
}

// ItemProcsEnabled off: no proc fires and no number is drawn, on the
// dispatcher's events and on a kill.
func TestItemProcsOffStopsEveryProcAndDrawsNothing(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	alice.Equipment.Body = items.New(dispatchProbeBody)
	setItemProcsEnabled(t, false)

	restore := util.SetRandForTest(42)
	want := util.Rand(1000)
	restore()
	restore = util.SetRandForTest(42)
	defer restore()
	for _, ev := range []string{"on_hit", "on_block", "on_grapple", "on_spell_hit"} {
		fireItemProc(behaviortree.EventContext{EventType: ev}, alice, bob, nil, 80)
	}
	MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 1}})
	if got := util.Rand(1000); got != want {
		t.Errorf("a disabled proc drew a number (probe %d, want %d)", got, want)
	}
	if alice.Health != 100 || bob.Conviction != 50 || bob.HasCondition(conditions.ConditionIdBleeding) {
		t.Errorf("a disabled proc fired: health %d, target conviction %d", alice.Health, bob.Conviction)
	}
}

// Ruling S4: a kill reaches an on_kill proc on any worn item, here the
// offhand, not the weapon alone.
func TestKillReachesAnOnKillProcOnTheOffhand(t *testing.T) {
	alice, _ := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		84: {ConditionId: 84, Name: "Stunned", RoundInterval: 1, TriggerCount: 1},
	}))
	alice.SetUserId(1)
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	hostile := mobs.GetInstance(100)
	if hostile == nil {
		t.Fatal("expected seeded hostile mob instance 100")
	}
	restore := util.SetRandForTest(1)
	defer restore()
	for i := 0; i < 20 && !hostile.Character.HasCondition(84); i++ {
		MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 1}})
	}
	if !hostile.Character.HasCondition(84) {
		t.Error("twenty kills never fired the offhand's on_kill proc")
	}
}

// The on_kill hook stamps the hunger-anchor round for every player with
// damage attribution.
func TestMobDeathItemProcs_RecordsLastKill(t *testing.T) {
	seedDispatchProbe(t)
	u := users.GetByUserId(1)
	if ret := MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 50}}); ret != events.Continue {
		t.Fatalf("expected events.Continue, got %v", ret)
	}
	got, ok := characters.MiscRound(u.Character.GetMiscData("pinnacle_last_kill_round"))
	if !ok || got != util.GetRoundCount() {
		t.Fatalf("expected last-kill round %d, got %v (ok=%v)", util.GetRoundCount(), got, ok)
	}
}
````

**Modify `internal/hooks/item_proc_parity_test.go`:**

````diff
@@ -20,17 +20,16 @@ import (
 )
 
 // The proc parity record (item behaviour slice 3, spec "Slice 3" gates).
-// testdata/item_proc_parity.golden was recorded on the Pinnacle proc path
-// (dispatchItemProcs) BEFORE the procs moved into item trees, and is
-// frozen: the tree path must reproduce it, the same outcomes on the same
+// testdata/item_proc_parity.golden was recorded on the retired Pinnacle
+// proc path (dispatchItemProcs) BEFORE the procs moved into item trees, and
+// is frozen: the tree path must reproduce it, the same outcomes on the same
 // rounds, under one seeded random source (util.SetRandForTest). One
 // scenario per shipped proc item and trigger, hits and misses, cooldown
 // windows and rounds where the effect has nothing to act on, plus a kill
 // scenario with every proc item worn. After each round a probe draw is
 // recorded, so a path that draws more or fewer numbers moves the record.
-//
-// Set DOGMUD_RECORD_PROC_PARITY=1 to rewrite the record. It was written
-// once, on the Pinnacle path; do not rewrite it on the tree path.
+// Both paths matched it before the old one was deleted; do not re-record
+// it on the tree path.
 
 const (
 	procParityUserId = 1
@@ -42,16 +41,12 @@ const (
 	procParityMobB   = 9802 // a second hostile in the room (aoe_stun)
 )
 
-// procDispatch fires a trigger's procs for owner against other: the
-// dispatcher under test.
+// procDispatch fires a trigger's procs for owner against other.
 type procDispatch func(trigger string, owner, other *characters.Character, room *rooms.Room, damage int)
 
-// procParityPaths are the dispatchers the record is checked against.
-var procParityPaths = map[string]procDispatch{
-	"pinnacle": dispatchItemProcs,
-	"tree": func(trigger string, owner, other *characters.Character, room *rooms.Room, damage int) {
-		fireItemProc(behaviortree.EventContext{EventType: trigger}, owner, other, room, damage)
-	},
+// treeProcDispatch fires a trigger through the item trees.
+func treeProcDispatch(trigger string, owner, other *characters.Character, room *rooms.Room, damage int) {
+	fireItemProc(behaviortree.EventContext{EventType: trigger}, owner, other, room, damage)
 }
 
 // loadProcParityWorld loads the shipped conditions and items, points the
@@ -290,20 +285,11 @@ func runProcParity(t *testing.T, fire procDispatch) string {
 }
 
 func TestItemProcParity(t *testing.T) {
-	path := filepath.Join("testdata", "item_proc_parity.golden")
-	if os.Getenv("DOGMUD_RECORD_PROC_PARITY") == "1" {
-		if err := os.WriteFile(path, []byte(runProcParity(t, dispatchItemProcs)), 0o644); err != nil {
-			t.Fatal(err)
-		}
-		t.Skip("recorded " + path)
-	}
-	want, err := os.ReadFile(path)
+	want, err := os.ReadFile(filepath.Join("testdata", "item_proc_parity.golden"))
 	if err != nil {
 		t.Fatal(err)
 	}
-	for name, fire := range procParityPaths {
-		if got := runProcParity(t, fire); got != string(want) {
-			t.Errorf("%s path moved off the record.\n--- want\n%s\n--- got\n%s", name, want, got)
-		}
+	if got := runProcParity(t, treeProcDispatch); got != string(want) {
+		t.Errorf("the tree path moved off the record.\n--- want\n%s\n--- got\n%s", want, got)
 	}
 }
````

**Delete `internal/hooks/item_procs.go`** (`git rm internal/hooks/item_procs.go`).

**Delete `internal/hooks/item_procs_test.go`** (`git rm internal/hooks/item_procs_test.go`).

**Modify `internal/hooks/pinnacle_tick.go`:**

````diff
@@ -19,7 +19,7 @@ import (
 )
 
 // pinnacle_tick.go — the always-on per-round layer for pinnacle items
-// (Stage 1, Task 11). Procs (item_procs.go) are event-driven off combat
+// (Stage 1, Task 11). Procs (item trees, item_proc_dispatch.go) fire off combat
 // chokepoints; THIS file is the passive upkeep that runs once per player per
 // round from UserRoundTick: hunger drain, ambient-potion conditions, aging freeze
 // and mutation drip. Sentient voices are item trees (the item tick) since slice 2.
@@ -55,7 +55,7 @@ func pinnacleUserTick(user *users.UserRecord, room *rooms.Room) {
 	// tick (ItemRoundTick) speaks their ambient lines.
 }
 
-// readMiscIntSlice tolerantly reads a []int from MiscData. Like readMiscRound,
+// readMiscIntSlice tolerantly reads a []int from MiscData. Like characters.MiscRound,
 // it copes with yaml round-tripping: a persisted []int comes back as []any of
 // int/int64/float64. Returns nil for absent/other values.
 func readMiscIntSlice(v any) []int {
@@ -65,7 +65,7 @@ func readMiscIntSlice(v any) []int {
 	case []any:
 		out := make([]int, 0, len(s))
 		for _, e := range s {
-			if n, ok := readMiscRound(e); ok {
+			if n, ok := characters.MiscRound(e); ok {
 				out = append(out, int(n))
 			}
 		}
@@ -137,14 +137,14 @@ func tickHunger(c *characters.Character, user *users.UserRecord, now uint64) {
 	if spec.HungerRounds <= 0 || spec.HungerDrainPct <= 0 {
 		return
 	}
-	anchor, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_anchor"))
+	anchor, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_anchor"))
 	if !ok {
 		// First tick wielding it — the hunger clock starts now.
 		c.SetMiscData("pinnacle_hunger_anchor", now)
 		return
 	}
 	// A kill since the anchor resets the clock (the blade was recently fed).
-	if kill, ok := readMiscRound(c.GetMiscData("pinnacle_last_kill_round")); ok && kill > anchor {
+	if kill, ok := characters.MiscRound(c.GetMiscData("pinnacle_last_kill_round")); ok && kill > anchor {
 		anchor = kill
 		c.SetMiscData("pinnacle_hunger_anchor", kill)
 	}
@@ -180,7 +180,7 @@ func tickHunger(c *characters.Character, user *users.UserRecord, now uint64) {
 	// its own cooldown (HungerFeedingLineCooldownRounds) so an ignored hunger
 	// debt doesn't spam the player every round.
 	if user != nil {
-		if next, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok || now >= next {
+		if next, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok || now >= next {
 			// The weapon's tree speaks the line when it has one (item
 			// behaviour slice 2); otherwise the plain fallback goes out.
 			if !fireItemEvent(behaviortree.EventContext{EventType: "on_hunger_feeding"},
@@ -333,7 +333,7 @@ func tickAmbientPotions(user *users.UserRecord, now uint64) {
 
 	// Contents unchanged. While a new potion is still attuning, keep the
 	// already-active conditions refreshed but hold the pending one back.
-	if attune, ok := readMiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok && now < attune {
+	if attune, ok := characters.MiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok && now < attune {
 		for _, id := range applied {
 			if desired[id] && !c.Conditions.HasCondition(id) {
 				_ = c.AddConditionScaled(id, 1.30)
@@ -353,7 +353,7 @@ func tickAmbientPotions(user *users.UserRecord, now uint64) {
 			_ = c.AddConditionScaled(id, 1.30)
 		}
 	}
-	if _, ok := readMiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok {
+	if _, ok := characters.MiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok {
 		if newlyApplied {
 			user.SendText(messaging.CategorySystem, fmt.Sprintf(
 				`<ansi fg="magenta">The %s settles into resonance — its stored virtues suffuse you.</ansi>`,
````

**Modify `internal/hooks/pinnacle_tick_test.go`:**

````diff
@@ -16,7 +16,7 @@ import (
 
 // setPinnacleEnabled flips the PinnacleItemsEnabled master toggle in-memory for
 // the test process (AddOverlayOverrides, the same file-free mechanism
-// enableItemProcs uses). Overlay overrides persist across tests, so the value
+// setItemProcsEnabled uses). Overlay overrides persist across tests, so the value
 // the test found is put back when it ends; a test that flips it twice ends
 // with both cleanups run, last first, back at that value.
 func setPinnacleEnabled(t *testing.T, on bool) {
@@ -53,7 +53,7 @@ func TestPinnacleHunger_DrainAndClock(t *testing.T) {
 	if c.Health != 100 {
 		t.Fatalf("first tick should not drain, health=%d", c.Health)
 	}
-	if a, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_anchor")); !ok || a != 50 {
+	if a, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_anchor")); !ok || a != 50 {
 		t.Fatalf("first tick should set anchor to 50, got %v (ok=%v)", a, ok)
 	}
 
@@ -100,7 +100,7 @@ func TestPinnacleHunger_NeverLethalAndKillReset(t *testing.T) {
 	if c.Health != 100 {
 		t.Fatalf("recent kill should reset the hunger clock, health=%d", c.Health)
 	}
-	if a, _ := readMiscRound(c.GetMiscData("pinnacle_hunger_anchor")); a != 60 {
+	if a, _ := characters.MiscRound(c.GetMiscData("pinnacle_hunger_anchor")); a != 60 {
 		t.Fatalf("kill should advance anchor to 60, got %d", a)
 	}
 }
@@ -132,7 +132,7 @@ func TestPinnacleHunger_FeedingLineCooldown(t *testing.T) {
 	if msgs := events.DrainQueuedMessagesForTest(708); len(msgs) == 0 {
 		t.Fatal("first overdue tick should emit the feeding line")
 	}
-	if _, ok := readMiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok {
+	if _, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok {
 		t.Fatal("feeding line should arm pinnacle_hunger_msg_next_round")
 	}
 
````

**Modify `internal/items/itemspec.go`:**

````diff
@@ -252,24 +252,6 @@ type AttackMessageOptions []ItemMessage
 type AttackEffects map[Intensity]AttackMessageOptions
 type AttackMessages map[ItemSubType]AttackEffects
 
-// ItemProc is a data-driven proc an item fires from a combat/round trigger.
-// Dispatch lives in internal/hooks (import direction: hooks → items).
-type ItemProc struct {
-	Trigger        string             `yaml:"trigger"`                   // on_hit | on_kill | on_block | on_grapple | on_spell_hit
-	Chance         int                `yaml:"chance"`                    // percent per trigger event (1-100)
-	CooldownRounds int                `yaml:"cooldown_rounds,omitempty"` // internal cooldown, 0 = none
-	Effect         string             `yaml:"effect"`                    // lifesteal | steal_pool | aoe_stun | apply_condition
-	Params         map[string]float64 `yaml:"params,omitempty"`
-}
-
-var validProcTriggers = map[string]bool{
-	"on_hit": true, "on_kill": true, "on_block": true, "on_grapple": true, "on_spell_hit": true,
-}
-
-var validProcEffects = map[string]bool{
-	"lifesteal": true, "steal_pool": true, "aoe_stun": true, "apply_condition": true,
-}
-
 // The blueprint for an item
 type ItemSpec struct {
 	ItemId           int
@@ -281,28 +263,28 @@ type ItemSpec struct {
 	// in the item's description the way a room's nouns are (lighting plan 5a:
 	// the hooded lantern's hood).
 	Nouns map[string]string `yaml:"nouns,omitempty"`
-	// ── Pinnacle Stage 1: procs, reserves, bandolier, mutation drip, hunger, voice ──
-	Procs                 []ItemProc `yaml:"procs,omitempty"`                   // data-driven combat procs
-	ReserveHealthPct      float64    `yaml:"reserve_health_pct,omitempty"`      // 0-1 fraction of HealthMax reserved while equipped
-	ReserveStaminaPct     float64    `yaml:"reserve_stamina_pct,omitempty"`     // 0-1 fraction of StaminaMax reserved while equipped
-	ReserveConvictionPct  float64    `yaml:"reserve_conviction_pct,omitempty"`  // 0-1 fraction of ConvictionMax reserved while equipped
-	PreservesContents     bool       `yaml:"preserves_contents,omitempty"`      // bandolier: contents never age
-	AmbientPotions        bool       `yaml:"ambient_potions,omitempty"`         // bandolier: slotted potion conditions always-on at Peak
-	MutationTickInterval  int        `yaml:"mutation_tick_interval,omitempty"`  // rounds between mutation rolls while worn (0 = never)
-	MutationTickChance    int        `yaml:"mutation_tick_chance,omitempty"`    // percent chance per roll
-	MutationRarityFloor   int        `yaml:"mutation_rarity_floor,omitempty"`   // min mutation rarity in the pool (0 = no floor)
-	HungerRounds          int        `yaml:"hunger_rounds,omitempty"`           // rounds without a kill before the item feeds on the wielder (0 = never)
-	HungerDrainPct        float64    `yaml:"hunger_drain_pct,omitempty"`        // fraction of HealthMax drained per hungry round
-	PhysicalMitigation    int        `yaml:"physical_mitigation,omitempty"`     // % physical damage reduction (Stage 34)
-	MagicalMitigation     int        `yaml:"magical_mitigation,omitempty"`      // % magical damage reduction (Stage 34)
-	ConvictionMitigation  int        `yaml:"conviction_mitigation,omitempty"`   // % conviction damage reduction (Stage 34)
-	DamageMultiplier      float64    `yaml:"damage_multiplier,omitempty"`       // Weapon damage multiplier for new pipeline (Stage 34)
-	SpellDamageMultiplier float64    `yaml:"spell_damage_multiplier,omitempty"` // Spell damage multiplier for caster weapons (wand/sceptre/staff)
-	ParryRating           int        `yaml:"parryrating,omitempty"`             // Weapon parry bonus (Stage 7.1)
-	BlockRating           int        `yaml:"blockrating,omitempty"`             // Shield block bonus (Stage 7.1)
-	AmmoTag               string     `yaml:"ammo_tag,omitempty"`                // Ranged weapons: ammo type required (arrows/bolts/shot). Ammo items: type provided.
-	MinStrength           int        `yaml:"min_strength,omitempty"`            // Minimum Strength to wield (heavy bows/arbalest)
-	WaitRounds            int        `yaml:"waitrounds,omitempty"`              // How many extra rounds each combat requires
+	// ── Pinnacle Stage 1: reserves, bandolier, mutation drip, hunger. Procs
+	// and voices live in the item's behaviour tree (item behaviour slices 2, 3).
+	ReserveHealthPct      float64 `yaml:"reserve_health_pct,omitempty"`      // 0-1 fraction of HealthMax reserved while equipped
+	ReserveStaminaPct     float64 `yaml:"reserve_stamina_pct,omitempty"`     // 0-1 fraction of StaminaMax reserved while equipped
+	ReserveConvictionPct  float64 `yaml:"reserve_conviction_pct,omitempty"`  // 0-1 fraction of ConvictionMax reserved while equipped
+	PreservesContents     bool    `yaml:"preserves_contents,omitempty"`      // bandolier: contents never age
+	AmbientPotions        bool    `yaml:"ambient_potions,omitempty"`         // bandolier: slotted potion conditions always-on at Peak
+	MutationTickInterval  int     `yaml:"mutation_tick_interval,omitempty"`  // rounds between mutation rolls while worn (0 = never)
+	MutationTickChance    int     `yaml:"mutation_tick_chance,omitempty"`    // percent chance per roll
+	MutationRarityFloor   int     `yaml:"mutation_rarity_floor,omitempty"`   // min mutation rarity in the pool (0 = no floor)
+	HungerRounds          int     `yaml:"hunger_rounds,omitempty"`           // rounds without a kill before the item feeds on the wielder (0 = never)
+	HungerDrainPct        float64 `yaml:"hunger_drain_pct,omitempty"`        // fraction of HealthMax drained per hungry round
+	PhysicalMitigation    int     `yaml:"physical_mitigation,omitempty"`     // % physical damage reduction (Stage 34)
+	MagicalMitigation     int     `yaml:"magical_mitigation,omitempty"`      // % magical damage reduction (Stage 34)
+	ConvictionMitigation  int     `yaml:"conviction_mitigation,omitempty"`   // % conviction damage reduction (Stage 34)
+	DamageMultiplier      float64 `yaml:"damage_multiplier,omitempty"`       // Weapon damage multiplier for new pipeline (Stage 34)
+	SpellDamageMultiplier float64 `yaml:"spell_damage_multiplier,omitempty"` // Spell damage multiplier for caster weapons (wand/sceptre/staff)
+	ParryRating           int     `yaml:"parryrating,omitempty"`             // Weapon parry bonus (Stage 7.1)
+	BlockRating           int     `yaml:"blockrating,omitempty"`             // Shield block bonus (Stage 7.1)
+	AmmoTag               string  `yaml:"ammo_tag,omitempty"`                // Ranged weapons: ammo type required (arrows/bolts/shot). Ammo items: type provided.
+	MinStrength           int     `yaml:"min_strength,omitempty"`            // Minimum Strength to wield (heavy bows/arbalest)
+	WaitRounds            int     `yaml:"waitrounds,omitempty"`              // How many extra rounds each combat requires
 	// StaminaCost is DEPRECATED and no longer read for cost. U7 Task 7 replaced
 	// the per-weapon attack charge with a config base (AttackBaseStaminaCost)
 	// times the encumbrance multiplier, charged per swing. A heavy weapon already
@@ -776,17 +758,6 @@ func (i *ItemSpec) Validate() error {
 		i.AutoCalculateValue()
 	}
 
-	for idx, p := range i.Procs {
-		if !validProcTriggers[p.Trigger] {
-			return fmt.Errorf("item %d proc %d: invalid trigger %q", i.ItemId, idx, p.Trigger)
-		}
-		if !validProcEffects[p.Effect] {
-			return fmt.Errorf("item %d proc %d: invalid effect %q", i.ItemId, idx, p.Effect)
-		}
-		if p.Chance < 1 || p.Chance > 100 {
-			return fmt.Errorf("item %d proc %d: chance must be 1-100, got %d", i.ItemId, idx, p.Chance)
-		}
-	}
 	for name, v := range map[string]float64{
 		"reserve_health_pct": i.ReserveHealthPct, "reserve_stamina_pct": i.ReserveStaminaPct, "reserve_conviction_pct": i.ReserveConvictionPct,
 		"hunger_drain_pct": i.HungerDrainPct,
@@ -819,20 +790,6 @@ func (i *ItemSpec) Validate() error {
 	return nil
 }
 
-// ProcsFor returns the procs matching a trigger. Cheap; no allocation when empty.
-func (i *ItemSpec) ProcsFor(trigger string) []ItemProc {
-	if len(i.Procs) == 0 {
-		return nil
-	}
-	var out []ItemProc
-	for _, p := range i.Procs {
-		if p.Trigger == trigger {
-			out = append(out, p)
-		}
-	}
-	return out
-}
-
 func (i *ItemSpec) Filename() string {
 
 	filename := util.ConvertForFilename(i.Name)
````

**Modify `internal/items/itemspec_pinnacle_test.go`:**

````diff
@@ -2,31 +2,9 @@ package items
 
 import "testing"
 
-func TestItemProcValidation(t *testing.T) {
-	spec := &ItemSpec{
-		ItemId: 999901, Name: "test proc item", Type: Weapon,
-		Procs: []ItemProc{{Trigger: "on_hit", Chance: 25, Effect: "lifesteal", Params: map[string]float64{"ratio": 0.25}}},
-	}
-	if err := spec.Validate(); err != nil {
-		t.Fatalf("valid proc rejected: %v", err)
-	}
-
-	bad := &ItemSpec{
-		ItemId: 999902, Name: "bad trigger", Type: Weapon,
-		Procs: []ItemProc{{Trigger: "on_sneeze", Chance: 25, Effect: "lifesteal"}},
-	}
-	if err := bad.Validate(); err == nil {
-		t.Fatal("invalid trigger accepted")
-	}
-
-	badEffect := &ItemSpec{
-		ItemId: 999903, Name: "bad effect", Type: Weapon,
-		Procs: []ItemProc{{Trigger: "on_hit", Chance: 25, Effect: "explode"}},
-	}
-	if err := badEffect.Validate(); err == nil {
-		t.Fatal("invalid effect accepted")
-	}
-
+// Procs are validated with the item's tree now (behaviortree checkProcNodes,
+// item behaviour slice 3).
+func TestItemReserveValidation(t *testing.T) {
 	badReserve := &ItemSpec{ItemId: 999904, Name: "bad reserve", Type: Weapon, ReserveHealthPct: 1.5}
 	if err := badReserve.Validate(); err == nil {
 		t.Fatal("reserve pct > 1 accepted")
@@ -45,19 +23,6 @@ func TestItemSpecBoundsValidation(t *testing.T) {
 		mut     func(*ItemSpec)
 		wantErr bool
 	}{
-		// Proc chance boundaries
-		{"chance 1 valid", func(s *ItemSpec) {
-			s.Procs = []ItemProc{{Trigger: "on_hit", Chance: 1, Effect: "lifesteal"}}
-		}, false},
-		{"chance 100 valid", func(s *ItemSpec) {
-			s.Procs = []ItemProc{{Trigger: "on_hit", Chance: 100, Effect: "lifesteal"}}
-		}, false},
-		{"chance 0 invalid", func(s *ItemSpec) {
-			s.Procs = []ItemProc{{Trigger: "on_hit", Chance: 0, Effect: "lifesteal"}}
-		}, true},
-		{"chance 101 invalid", func(s *ItemSpec) {
-			s.Procs = []ItemProc{{Trigger: "on_hit", Chance: 101, Effect: "lifesteal"}}
-		}, true},
 		// Reserve pct boundaries
 		{"reserve 0 valid", func(s *ItemSpec) { s.ReserveHealthPct = 0 }, false},
 		{"reserve 0.99 valid", func(s *ItemSpec) { s.ReserveHealthPct = 0.99 }, false},
@@ -98,16 +63,3 @@ func TestItemSpecBoundsValidation(t *testing.T) {
 		})
 	}
 }
-
-func TestProcsFor(t *testing.T) {
-	spec := &ItemSpec{Procs: []ItemProc{
-		{Trigger: "on_hit", Chance: 100, Effect: "lifesteal"},
-		{Trigger: "on_block", Chance: 10, Effect: "aoe_stun"},
-	}}
-	if got := spec.ProcsFor("on_hit"); len(got) != 1 || got[0].Effect != "lifesteal" {
-		t.Fatalf("ProcsFor(on_hit) = %+v", got)
-	}
-	if got := spec.ProcsFor("on_kill"); len(got) != 0 {
-		t.Fatalf("expected empty, got %+v", got)
-	}
-}
````

**Delete `internal/items/proc_accessors.go`** (`git rm internal/items/proc_accessors.go`).

**Delete `internal/items/proc_accessors_test.go`** (`git rm internal/items/proc_accessors_test.go`).

**Modify `internal/items/save_test.go`:**

````diff
@@ -95,20 +95,20 @@ func TestCanonicalizeItemNames_PreservesMinorWords(t *testing.T) {
 	}
 }
 
-// SaveItemSpec calls Validate(), which rejects an invalid proc — so a bad proc
-// from the web editor returns an error (red toast) instead of persisting. If
-// this ever fails, SaveItemSpec has stopped validating.
-func TestSaveItemSpec_RejectsInvalidProc(t *testing.T) {
+// SaveItemSpec calls Validate(), which rejects an invalid spec — so a bad
+// value from the web editor returns an error (red toast) instead of
+// persisting. If this ever fails, SaveItemSpec has stopped validating.
+func TestSaveItemSpec_RejectsInvalidSpec(t *testing.T) {
 	dir := t.TempDir()
 	pointItemsAt(t, dir)
 	spec := ItemSpec{ItemId: 10009, Name: "Bad Blade", Type: Weapon, Description: "d", Hands: 1,
 		NotSalable: true, DamageMultiplier: 1.0,
-		Procs: []ItemProc{{Trigger: "on_wobble", Effect: "lifesteal", Chance: 50}}, // bad trigger
+		ReserveHealthPct: 1.5, // out of [0,1)
 	}
 	items[spec.ItemId] = &spec
 	t.Cleanup(func() { delete(items, 10009) })
 	if err := SaveItemSpec(spec); err == nil {
-		t.Fatal("expected SaveItemSpec to reject an invalid proc trigger")
+		t.Fatal("expected SaveItemSpec to reject an out-of-range reserve")
 	}
 }
 
````

**Modify `item_behaviour_guard_test.go`:**

````diff
@@ -48,7 +48,6 @@ import (
 // field is classified exactly once, so a new field fails until someone says
 // which it is.
 var itemSpecBehaviourFields = map[string]string{
-	"Procs":                "combat procs; slice 3 moves them into proc nodes",
 	"ReserveHealthPct":     "Pinnacle reserve held while worn",
 	"ReserveStaminaPct":    "Pinnacle reserve held while worn",
 	"ReserveConvictionPct": "Pinnacle reserve held while worn",
@@ -85,20 +84,18 @@ var itemSpecDataFields = []string{
 }
 
 // itemSpecBehaviourMethods maps an ItemSpec method to the behaviour field it
-// reads, so calling it counts as reading that field.
-var itemSpecBehaviourMethods = map[string]string{
-	"ProcsFor": "Procs",
-}
+// reads, so calling it counts as reading that field. Empty since slice 3
+// retired ProcsFor with the Procs field.
+var itemSpecBehaviourMethods = map[string]string{}
 
 // itemSpecBehaviourReadSites are the only places non-test internal/hooks
 // reads a behaviour field: "file|field". These are today's Pinnacle
 // mechanics. Slice 2 retired the VoiceId and TauntPull sites (voices are
-// item trees), slice 3 retires the Procs ones; an entry nothing reads any
-// more fails, so the list only shrinks. A new site fails: put the
-// behaviour in a tree.
+// item trees), slice 3 the Procs one (procs are item trees); an entry
+// nothing reads any more fails, so the list only shrinks. A new site
+// fails: put the behaviour in a tree.
 var itemSpecBehaviourReadSites = map[string]bool{
 	"PlayerSpawn_HandleJoin.go|PreservesContents": true,
-	"item_procs.go|Procs":                         true,
 	"pinnacle_tick.go|AmbientPotions":             true,
 	"pinnacle_tick.go|HungerDrainPct":             true,
 	"pinnacle_tick.go|HungerRounds":               true,
````

**Modify `modules/gmcp/gmcp.Item.go`:**

````diff
@@ -32,14 +32,6 @@ type itemSalvageRow struct {
 	Quantity int    `json:"quantity"`
 }
 
-type procRow struct {
-	Trigger        string             `json:"trigger"`
-	Effect         string             `json:"effect"`
-	Chance         int                `json:"chance"`
-	CooldownRounds int                `json:"cooldownRounds"`
-	Params         map[string]float64 `json:"params"`
-}
-
 // itemUpdateReq is both the Save payload and (embedded) the echoed detail.
 type itemUpdateReq struct {
 	ItemId           int            `json:"itemId"`
@@ -97,17 +89,16 @@ type itemUpdateReq struct {
 	SalvageReturns  []itemSalvageRow `json:"salvageReturns"`
 	// key
 	KeyLockId string `json:"keyLockId"`
-	// advanced / pinnacle
-	Procs                []procRow `json:"procs"`
-	ReserveHealthPct     float64   `json:"reserveHealthPct"`
-	ReserveStaminaPct    float64   `json:"reserveStaminaPct"`
-	ReserveConvictionPct float64   `json:"reserveConvictionPct"`
-	HungerRounds         int       `json:"hungerRounds"`
-	HungerDrainPct       float64   `json:"hungerDrainPct"`
-	MutationTickInterval int       `json:"mutationTickInterval"`
-	MutationTickChance   int       `json:"mutationTickChance"`
-	MutationRarityFloor  int       `json:"mutationRarityFloor"`
-	WornConditionIds     []int     `json:"wornConditionIds"`
+	// advanced / pinnacle (procs live in the item's behaviour tree, slice 3)
+	ReserveHealthPct     float64 `json:"reserveHealthPct"`
+	ReserveStaminaPct    float64 `json:"reserveStaminaPct"`
+	ReserveConvictionPct float64 `json:"reserveConvictionPct"`
+	HungerRounds         int     `json:"hungerRounds"`
+	HungerDrainPct       float64 `json:"hungerDrainPct"`
+	MutationTickInterval int     `json:"mutationTickInterval"`
+	MutationTickChance   int     `json:"mutationTickChance"`
+	MutationRarityFloor  int     `json:"mutationRarityFloor"`
+	WornConditionIds     []int   `json:"wornConditionIds"`
 }
 
 // ---- server -> client detail (Build.Item) ----
@@ -118,8 +109,6 @@ type itemDetail struct {
 	Elements      []string              `json:"elements"`
 	Stats         []string              `json:"stats"`
 	VendorCats    []string              `json:"vendorCats"`       // valid vendor categories for the checkboxes
-	ProcTriggers  []string              `json:"procTriggers"`     // valid proc trigger ids for the dropdown
-	ProcEffects   []string              `json:"procEffects"`      // valid proc effect ids for the dropdown
 	Ranges        map[string][2]float64 `json:"ranges,omitempty"` // observed min–max per numeric field, across items of this type
 }
 
@@ -197,9 +186,6 @@ func specToReq(s *items.ItemSpec) itemUpdateReq {
 	req.HungerRounds, req.HungerDrainPct = s.HungerRounds, s.HungerDrainPct
 	req.MutationTickInterval, req.MutationTickChance, req.MutationRarityFloor = s.MutationTickInterval, s.MutationTickChance, s.MutationRarityFloor
 	req.WornConditionIds = s.WornConditionIds
-	for _, p := range s.Procs {
-		req.Procs = append(req.Procs, procRow{Trigger: p.Trigger, Effect: p.Effect, Chance: p.Chance, CooldownRounds: p.CooldownRounds, Params: p.Params})
-	}
 	return req
 }
 
@@ -215,17 +201,14 @@ func buildItemGet(d itemDeps, itemId int) (itemDetail, bool) {
 	return itemDetail{
 		itemUpdateReq: specToReq(s),
 		Types:         itemTypeIds(), Subtypes: itemSubtypeIds(), Elements: itemElementIds(), Stats: statModNames(),
-		VendorCats:   shops.ValidVendorCategories,
-		ProcTriggers: procTriggerIds(), ProcEffects: procEffectIds(),
-		Ranges: ranges,
+		VendorCats: shops.ValidVendorCategories,
+		Ranges:     ranges,
 	}, true
 }
 
-func procTriggerIds() []string { return items.ValidProcTriggers() }
-func procEffectIds() []string  { return items.ValidProcEffects() }
-
 // reqToSpec starts from the loaded spec so fields the form does NOT cover
-// (procs, reserves, worn-conditions, mutation drip, etc.) survive a Save untouched.
+// (the behaviour tree, reserves, worn-conditions, mutation drip, etc.)
+// survive a Save untouched.
 func reqToSpec(base *items.ItemSpec, req itemUpdateReq) items.ItemSpec {
 	s := *base
 	s.Name, s.DisplayName, s.NameSimple, s.Description = req.Name, req.DisplayName, req.NameSimple, req.Description
@@ -254,13 +237,6 @@ func reqToSpec(base *items.ItemSpec, req itemUpdateReq) items.ItemSpec {
 	s.HungerRounds, s.HungerDrainPct = req.HungerRounds, req.HungerDrainPct
 	s.MutationTickInterval, s.MutationTickChance, s.MutationRarityFloor = req.MutationTickInterval, req.MutationTickChance, req.MutationRarityFloor
 	s.WornConditionIds = req.WornConditionIds
-	s.Procs = nil
-	for _, p := range req.Procs {
-		if p.Trigger == "" && p.Effect == "" {
-			continue // skip blank rows the form may emit
-		}
-		s.Procs = append(s.Procs, items.ItemProc{Trigger: p.Trigger, Chance: p.Chance, CooldownRounds: p.CooldownRounds, Effect: p.Effect, Params: p.Params})
-	}
 	return s
 }
 
````

**Modify `modules/gmcp/gmcp.Item_test.go`:**

````diff
@@ -6,18 +6,6 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/items"
 )
 
-func TestBuildItemGet_ShipsAdvancedEnums(t *testing.T) {
-	w := newFakeItemWorld()
-	w.specs[10005] = &items.ItemSpec{ItemId: 10005, Name: "Sword", Type: items.Weapon}
-	d, ok := buildItemGet(w.deps(), 10005)
-	if !ok {
-		t.Fatal("expected found")
-	}
-	if len(d.ProcTriggers) == 0 || len(d.ProcEffects) == 0 {
-		t.Error("detail must ship proc trigger/effect enums")
-	}
-}
-
 // fakeItemWorld is an in-memory stand-in for the items package.
 type fakeItemWorld struct {
 	specs   map[int]*items.ItemSpec
@@ -81,8 +69,6 @@ func TestBuildItemUpdate_RoundTripsAdvancedFields(t *testing.T) {
 	w.specs[10001] = &items.ItemSpec{ItemId: 10001, Name: "Old", Type: items.Weapon, Hands: 2, Behavior: "blackrazor"}
 	res := buildItemUpdate(w.deps(), itemUpdateReq{
 		ItemId: 10001, Name: "Blackrazor", Type: "weapon", Description: "d", NotSalable: true,
-		Procs: []procRow{{Trigger: "on_hit", Effect: "lifesteal", Chance: 100, CooldownRounds: 2,
-			Params: map[string]float64{"ratio": 0.25}}},
 		ReserveHealthPct: 0.25,
 		HungerRounds:     50, HungerDrainPct: 0.01,
 		MutationTickInterval: 10, MutationTickChance: 5, MutationRarityFloor: 3,
@@ -92,12 +78,9 @@ func TestBuildItemUpdate_RoundTripsAdvancedFields(t *testing.T) {
 		t.Fatalf("update should succeed, got %+v", res)
 	}
 	got := w.saved[0]
-	if len(got.Procs) != 1 || got.Procs[0].Trigger != "on_hit" || got.Procs[0].Effect != "lifesteal" ||
-		got.Procs[0].Chance != 100 || got.Procs[0].CooldownRounds != 2 || got.Procs[0].Params["ratio"] != 0.25 {
-		t.Errorf("proc not round-tripped: %+v", got.Procs)
-	}
 	// The form has no tree field (editing trees is #367), so a save keeps
-	// the item's behavior: from the loaded spec.
+	// the item's behavior: from the loaded spec, and with it the item's
+	// voice and procs (item behaviour slices 2, 3).
 	if got.Behavior != "blackrazor" {
 		t.Errorf("a save dropped the item's behavior tree: got %q", got.Behavior)
 	}
````

- [ ] **Step 2: Nothing names the old path**

```bash
grep -rn --exclude-dir=vendor --exclude-dir=.git -I "dispatchItemProcs\|procGateOpen\|markProcCooldown\|procBearingItems\|items\.ItemProc\|ProcsFor\|ValidProcTriggers\|ValidProcEffects\|procTriggers\|procEffects" internal modules _datafiles *.go | grep -v "_test.go"
grep -rln "^procs:" _datafiles/world
```

Expected: both print nothing. (Test files are excluded: the parity test's header names the retired `dispatchItemProcs` on purpose.) Docs are Task 5.

- [ ] **Step 3: Tests, and the switch-off test can fail**

```bash
gofmt -l internal/ modules/
go build ./...
go vet ./internal/items ./internal/hooks ./internal/behaviortree ./modules/gmcp
node --check _datafiles/html/public/static/js/items.js
go test ./internal/behaviortree ./internal/hooks ./internal/items ./modules/gmcp -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: clean, then all `ok`. Null probe: in `ProcRandomDecorator.Evaluate` change `if !ItemProcsOn() || util.Rand(100) >= d.Percent {` to `if util.Rand(100) >= d.Percent {`, run `go test ./internal/hooks -run TestItemProcsOff -count=1`: `a disabled proc drew a number`. Restore.

- [ ] **Step 4: Spec check and commit**

```bash
P="internal/behaviortree/item_proc_effects_test.go internal/hooks/item_proc_dispatch_test.go internal/hooks/item_proc_parity_test.go internal/hooks/pinnacle_tick.go internal/hooks/pinnacle_tick_test.go internal/items/itemspec.go internal/items/itemspec_pinnacle_test.go internal/items/save_test.go modules/gmcp/gmcp.Item.go modules/gmcp/gmcp.Item_test.go _datafiles/html/public/static/js/items.js _datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml _datafiles/world/dogmud/items/materials-40000/40185-aegis_of_mockery.yaml _datafiles/world/dogmud/items/materials-40000/40186-thornwall_harness.yaml _datafiles/world/dogmud/items/materials-40000/40189-staff_of_the_hollow_choir.yaml condition_apply_path_guard_test.go item_behaviour_guard_test.go"
git rm -q internal/hooks/item_procs.go internal/hooks/item_procs_test.go internal/items/proc_accessors.go internal/items/proc_accessors_test.go
git diff 59af9ebd1 -- $P internal/hooks/item_procs.go internal/items/proc_accessors.go
git add $P
git commit -m "refactor: retire the Pinnacle proc path, ItemProc, procs: keys and the builder proc editor" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Docs and patch notes (spec "Every slice")

Checkpoint `d7d178073`. `context.md` for `behaviortree` (events table, allowlist, files table, a new "Item procs" section), `hooks` (helper list, an "Item procs" paragraph), `items` (the `proc_accessors.go` row goes), `characters` (the `Heal` caller moved to `actions_item_proc.go:213`); `docs/schemas/pinnacle-items.md` §1 rewritten for tree branches, the config, MiscData and shipped-item rows; `docs/schemas/behavior.md` (allowlist, the `proc` row, an "Item procs" subsection); a patch notes entry.

- [ ] **Step 1: Apply**

**Modify `docs/PATCH_NOTES.md`:**

````diff
@@ -1,5 +1,15 @@
 # DOGMud Patch Notes
 
+## 2026-10-06: Gear that strikes back
+
+- The Blackrazor's life drain, the Aegis of Mockery's shockwave, the
+  Thornwall Harness's barbs and the Staff of the Hollow Choir's hunger for
+  conviction now run on the same system as talking gear. They work
+  exactly as before.
+- When one of these effects has to wait before it can strike again,
+  logging out does not end the wait, and neither does swapping to another
+  copy of the same item.
+
 ## 2026-10-06: Small fixes
 
 - A disarmed weapon now always goes back into your pack, even when your
````

**Modify `docs/schemas/behavior.md`:**

````diff
@@ -549,7 +549,7 @@ An item tree may name only these nodes (anything else refuses at load):
 | Kind | Nodes |
 |---|---|
 | Conditions | `time_of_day`, `round_mod`, `random_chance`, `state_equals`, `state_greater_than`, `holder_asleep`, `worn`, `in_combat`, `chatter_ready`, `hunger_overdue` |
-| Actions | `set_state`, `increment_state`, `decrement_state`, `set_light`, `pulse_light`, `speak`, `taunt_pull` |
+| Actions | `set_state`, `increment_state`, `decrement_state`, `set_light`, `pulse_light`, `speak`, `taunt_pull`, `proc` |
 | Decorators | all |
 
 | Node | Params | Description |
@@ -563,9 +563,10 @@ An item tree may name only these nodes (anything else refuses at load):
 | `hunger_overdue` | `fraction` (default 0.75) | The holder's hunger anchor is more than that fraction of the item's `hunger_rounds` behind. |
 | `speak` | `pool`; `to`: `all` (default) or `holder`; `paced`: true (default) or false | Says a line from the tree's `speech:` pool: the holder reads "<Item> says", the room hears "<Name>'s <Item> mutters" with the holder's name hidden by sight. Paced: refuses while the item's cooldown is closed, then arms it. |
 | `taunt_pull` | none | The holder's foe, a mob fighting someone else, turns on the holder. Always succeeds. |
+| `proc` | `effect`: `lifesteal`, `steal_pool`, `aoe_stun` or `apply_condition`, plus that effect's number params | Runs a combat effect for the holder against the event's opponent. Succeeds only when the effect did something. Only under a proc event. |
 
 The item-only nodes (`holder_asleep`, `worn`, `in_combat`, `chatter_ready`,
-`hunger_overdue`, `set_light`, `pulse_light`, `speak`, `taunt_pull`) refuse in a mob or room tree. A tree that writes light may
+`hunger_overdue`, `set_light`, `pulse_light`, `speak`, `taunt_pull`, `proc`) refuse in a mob or room tree. A tree that writes light may
 not sit on an item whose worn light is `adjustable` (the trim owns it). A
 fixture (`fixture: light` or `darkness` on the item) cannot be taken off the
 floor; a pulsing fixture must stay inside one light band
@@ -584,3 +585,19 @@ by `HungerFeedingLineCooldownRounds`). Gate ambient lines with
 `chatter_ready`; an event branch speaks straight, skipping the listener
 cap and the chance. `behaviors/items/blackrazor.yaml` and `aegis.yaml` are
 the worked examples.
+
+### Item procs (item behaviour slice 3)
+
+A proc is a branch under one of the proc events: `on_hit` and
+`on_spell_hit` reach the weapon, `on_block` the offhand, `on_grapple` the
+body armour (both sides of a hold fire), and `on_kill` every worn item.
+Wrap the chance in a `random` decorator (1 to 99; leave it out at 100, so
+nothing is drawn) and the cooldown in a `cooldown` decorator, then end in
+`proc`. A `cooldown` over a `proc` is kept on the bearer, per item
+template, so two copies share it and it survives a relog; every other
+item cooldown is the item's own. `GamePlay.ItemProcsEnabled` off stops
+every proc and draws nothing. The loader refuses a `proc` outside a proc
+event, an unknown effect, a param its effect does not read or that is not
+a number, and a `random` over a proc outside 1 to 99. The effects and
+their params are in `docs/schemas/pinnacle-items.md`; `aegis.yaml`,
+`thornwall_harness.yaml` and `hollow_choir.yaml` are the worked examples.
````

**Modify `docs/schemas/pinnacle-items.md`:**

````diff
@@ -12,76 +12,91 @@ spec), on branch `feature/pinnacle-stage1-engine-primitives`.
 All fields live on `ItemSpec` (`internal/items/itemspec.go`) unless
 noted otherwise.
 
-## 1. Procs (`procs:`)
+## 1. Procs (item tree `proc` branches)
+
+Since item behaviour slice 3 a proc is a branch of the item's behaviour
+tree (`behaviors/items/<name>.yaml`, named by `behavior:` on the item), not
+a `procs:` list on the item. The branch sits under a proc event, wraps the
+chance in a `random` decorator and the cooldown in a `cooldown` decorator,
+and ends in a `proc` action naming the effect and its params:
 
 ```yaml
-procs:
-  - trigger: on_hit           # on_hit | on_kill | on_block | on_grapple | on_spell_hit
-    chance: 25                # 1-100, percent per trigger event
-    cooldown_rounds: 0        # 0 = no cooldown
-    effect: lifesteal         # lifesteal | steal_pool | aoe_stun | apply_condition
-    params:
-      ratio: 0.25
+tree:
+  type: decorator
+  event: on_block          # on_hit | on_kill | on_block | on_grapple | on_spell_hit
+  mod: cooldown
+  rounds: 20               # the proc's cooldown; leave the decorator out for none
+  child:
+    type: decorator
+    mod: random
+    percent: 10            # 1-99; leave the decorator out at 100 (nothing is drawn)
+    child:
+      type: action
+      do: proc
+      effect: aoe_stun     # lifesteal | steal_pool | aoe_stun | apply_condition
 ```
 
-`Validate()` rejects unknown `trigger`/`effect` values and requires
-`chance` in 1-100 — bad data panics at boot, not at first swing.
+The tree loader refuses (and the boot panics on) a `proc` outside a proc
+event, an unknown effect, a param its effect does not read or that is not
+a number, and a `random` over a proc outside 1 to 99.
 
-**Which equipment slot is consulted, per trigger**
-(`procBearingItems` in `internal/hooks/item_procs.go`):
+**Which equipment slot each event reaches** (`fireItemProc` in
+`internal/hooks/item_proc_dispatch.go`):
 
-| Trigger        | Slot consulted |
-|----------------|----------------|
+| Event          | Slot reached |
+|----------------|--------------|
 | `on_hit`       | Weapon |
-| `on_kill`      | Weapon |
 | `on_spell_hit` | Weapon |
 | `on_block`     | Offhand |
-| `on_grapple`   | Body |
+| `on_grapple`   | Body (both sides of the hold fire, each into its own body armour) |
+| `on_kill`      | Every worn item with a tree (owner ruling S4, the reach kill lines have) |
+
+A proc authored under an event its item's slot never hears simply never
+fires.
 
-Only one item per trigger is ever consulted (the cost discipline is
-1-2 spec lookups per swing) — a proc authored on the wrong slot for
-its trigger simply never fires.
+**Gate order.** `GamePlay.ItemProcsEnabled` first (`fireItemProc` reads it
+before running the tree, and a `random` over a proc reads it again, so a
+disabled proc draws no number on any event), then the branch's cooldown,
+then its chance. A `proc` succeeds only when the effect did something, and
+only then is the cooldown armed (a `lifesteal` on a 0-damage hit does not
+burn it).
 
-**Gate order** (`procGateOpen`): `GamePlay.ItemProcsEnabled` kill
-switch → per-(item, proc-index) cooldown check → chance roll. The
-cooldown is marked **only when the effect actually executed** (e.g. a
-`lifesteal` proc that rolls on a 0-damage hit does not burn its
-cooldown) — see `dispatchItemProcs`'s `executed` flag.
+**The cooldown is per template, on the bearer** (ruling R1, #223). A
+`cooldown` decorator over a proc keeps its round in the holder's MiscData
+under `item_proc_cd_<itemId>_<branch path>` (`ProcCooldownDecorator`), not
+in the item's tree state, so two copies of one item share it and it
+survives a relog. Every other item cooldown stays in tree state.
 
 **`on_kill` fires once per player with damage attribution on the
 kill**, not just the killing blow (`MobDeathItemProcs` iterates
 `evt.PlayerDamage`). This is a deliberate party-friendly design
-decision, not an oversight — it also resets every such player's
+decision, not an oversight. It also resets every such player's
 Blackrazor-style hunger anchor (`pinnacle_last_kill_round`).
 
-### Per-effect `params`
-
-- **`lifesteal`** — `{ratio: <fraction>}`. Heals the attacker
-  `ratio * damage` (floored, minimum 1 if ratio*damage rounds to 0),
-  clamped to `HealthMax` by `Character.Heal`.
-- **`steal_pool`** — `{pool: 3, amount_pct: <fraction>}`. Only
-  `pool: 3` (conviction) is wired; `1` (health) and `2` (stamina) are
-  reserved but **unimplemented** (YAGNI until an item needs them —
-  they silently no-op). `amount_pct` is a fraction of the **target's**
-  pool max, capped by what the target actually has; drains the target
-  and adds to the owner (clamped to the owner's max).
-- **`aoe_stun`** — `{}` (no params consumed; `stun_rounds` is
-  intentionally ignored — see below). Applies condition **84** (a fixed
-  1-round stagger/stun) to every hostile mob in the owner's room.
-  Non-combatants, `PlayerAttackImmune` mobs, and **any** charmed mob
-  (not just the owner's own charm) are always skipped — sparing
-  bystanders' companions, matching the `HarmArea` precedent. Mob
-  owners (no `GetUserId()`) are a no-op — no Stage-2 mob wields one of
-  these. `stun_rounds` is ignored by design: condition 84 is a fixed
-  1-round stagger baked into its own YAML (`triggercount: 1`) and
-  cannot be duration-scaled from proc params without hacking condition
-  internals; tune an aoe_stun item's strength via `chance`/
-  `cooldown_rounds` instead.
-- **`apply_condition`** — `{condition: 1, duration: <rounds>,
-  magnitude: <per-tick>}`. Only `condition: 1` (bleeding) is wired.
-  `duration` defaults to 4 rounds, `magnitude` defaults to 2 per tick,
-  when unset or < 1. Unknown condition ids no-op (cooldown not
-  burned).
+### Per-effect params
+
+The effects are `internal/behaviortree/actions_item_proc.go`'s
+`procLifesteal`, `procStealPool`, `procAoeStun` and `procApplyCondition`,
+moved unchanged from the retired `internal/hooks/item_procs.go`.
+
+- **`lifesteal`**: `ratio`. Heals the bearer `ratio * damage` (floored,
+  minimum 1 if it rounds to 0), clamped to `HealthMax` by `Character.Heal`.
+- **`steal_pool`**: `pool: 3`, `amount_pct`. Only `pool: 3` (conviction)
+  is wired; `1` (health) and `2` (stamina) are reserved but
+  **unimplemented** (they no-op). `amount_pct` is a fraction of the
+  **target's** pool max, capped by what the target has; it drains the
+  target and adds exactly the drained amount to the bearer.
+- **`aoe_stun`**: no params consumed (`stun_rounds` is accepted and
+  ignored). Applies condition **84** (a fixed 1-round stagger) to every
+  hostile mob in the bearer's room. Non-combatants, `PlayerAttackImmune`
+  mobs and **any** charmed mob are skipped, matching the `HarmArea`
+  precedent. A mob bearer is a no-op. Condition 84 is fixed at one round
+  in its own YAML (`triggercount: 1`), so tune an aoe_stun item through its
+  branch's chance and cooldown instead.
+- **`apply_condition`**: `condition: 1`, `duration`, `magnitude`. Only
+  `condition: 1` (bleeding) is wired; each firing adds a stack. `duration`
+  defaults to 4 rounds and `magnitude` to 2 per tick when unset or below
+  1. Unknown condition ids no-op (the cooldown is not armed).
 
 ## 2. Pool reservations
 
@@ -277,7 +292,7 @@ special-case branch never fires. Drinking it:
 | Knob | Location | Default | Purpose |
 |------|----------|---------|---------|
 | `GamePlay.PinnacleItemsEnabled` | config.gameplay.go | `true` | Master toggle for the whole per-round pinnacle tick (hunger/ambient/mutation-drip); `pinnacleUserTick` early-returns entirely when off. It also silences item tree voices: `chatter_ready`, `speak` and `taunt_pull` each check it. |
-| `GamePlay.ItemProcsEnabled` | config.gameplay.go | `true` | Kill switch for proc firing (`on_hit`/`on_block`/etc.); checked in `procGateOpen`. |
+| `GamePlay.ItemProcsEnabled` | config.gameplay.go | `true` | Kill switch for proc firing (`on_hit`/`on_block`/etc.); read by `fireItemProc`, `proc` and a `random` over a proc, so a disabled proc draws nothing. |
 | `Balance.BandolierAttuneRounds` | config.balance.go | `100` | Re-attunement cooldown length after bandolier contents change. |
 | `Balance.ItemChatterQuietCooldownRounds` / `...ChancePct` | config.balance.go | `40` / `10` | A quiet item tree's rounds between lines and chance on an open round. |
 | `Balance.ItemChatterNormalCooldownRounds` / `...ChancePct` | config.balance.go | `20` / `15` | The same for a normal tree (the default level; the shipped voices). |
@@ -296,7 +311,8 @@ YAML (numeric values round-trip through YAML as `int`/`int64`/
 
 | Key | Set by | Meaning |
 |-----|--------|---------|
-| `pinnacle_proc_cd_<itemId>_<procIdx>` | `markProcCooldown` | Round at which this item's Nth proc may fire again. |
+| `item_proc_cd_<itemId>_<branch path>` | `ProcCooldownDecorator` (behaviortree) | Round at which this item template's proc branch may fire again (ruling R1: per template, shared by copies, kept across relog). |
+| `pinnacle_proc_cd_<itemId>_<procIdx>` | (retired) | Inert in old saves: the proc cooldown moved to `item_proc_cd_*` in item behaviour slice 3, so a cooldown running at that deploy starts fresh once. |
 | `pinnacle_last_kill_round` | `MobDeathItemProcs` | Last round this player got damage-attribution credit on a kill (drives hunger reset). |
 | `pinnacle_hunger_anchor` | `tickHunger` | Round the current hunger weapon's clock is anchored to. |
 | `pinnacle_hunger_msg_next_round` | `tickHunger` | Cooldown gate for the repeated feeding message. |
@@ -305,9 +321,9 @@ YAML (numeric values round-trip through YAML as `int`/`int64`/
 | `pinnacle_bandolier_fingerprint` | `tickAmbientPotions` | Last-seen `beltId:potionId,potionId,...` fingerprint, used to detect any content change. |
 | `pinnacle_voice_next_round` | (retired) | Inert in old saves: the voice cooldown is item tree state since item behaviour slice 2. |
 
-Cooldown keys (`pinnacle_proc_cd_*` in particular) are **intentionally
+Cooldown keys (`item_proc_cd_*` in particular) are **intentionally
 never pruned** when an item is unequipped or lost — the key space is
-bounded (per item id x proc index) and a stale cooldown is harmless
+bounded (per item id x proc branch) and a stale cooldown is harmless
 (it just means that exact item, if re-equipped, resumes mid-cooldown
 rather than fresh). This is a deliberate simplicity trade-off, not an
 oversight.
@@ -333,13 +349,13 @@ starting values; combat/economy tuning is a later stage.
 |----|------|-------------|----------------------------------------|-----------------|
 | 40181 | Phial of Second Birth | consumable (potion) | `40181-phial_of_second_birth.yaml` | remort (`drink.go` hardcodes id 40181 → `ScourMutations` + rarity-floored grant) |
 | 40182 | Vitalis Bandolier | belt | `40182-vitalis_bandolier.yaml` | `preserves_contents`, `ambient_potions`, `is_bandolier`/`bandolier_capacity` |
-| 40183 | The Blackrazor | weapon (2H slashing) | `40183-the_blackrazor.yaml` | `reserve_health_pct`, `hunger_rounds`/`hunger_drain_pct`, `procs` (on_hit lifesteal), `behavior: blackrazor` (voice, slice 2) |
+| 40183 | The Blackrazor | weapon (2H slashing) | `40183-the_blackrazor.yaml` | `reserve_health_pct`, `hunger_rounds`/`hunger_drain_pct`, `behavior: blackrazor` (voice, slice 2; on_hit lifesteal proc, slice 3) |
 | 40184 | Wayfarer's Bottomless Pack | back | `40184-wayfarers_bottomless_pack.yaml` | `weight_reduction` (0.99) |
-| 40185 | Aegis of Mockery | offhand (shield) | `40185-aegis_of_mockery.yaml` | `procs` (on_block aoe_stun), `behavior: aegis` (voice and taunt pull, slice 2) |
-| 40186 | Thornwall Harness | body | `40186-thornwall_harness.yaml` | `procs` (on_grapple apply_condition bleed) |
+| 40185 | Aegis of Mockery | offhand (shield) | `40185-aegis_of_mockery.yaml` | `behavior: aegis` (voice and taunt pull, slice 2; on_block aoe_stun proc, slice 3) |
+| 40186 | Thornwall Harness | body | `40186-thornwall_harness.yaml` | `behavior: thornwall_harness` (on_grapple bleed proc, slice 3) |
 | 40187 | Seething Prism | neck | `40187-seething_prism.yaml` | `reserve_*_pct` (all three pools), `mutation_tick_interval`/`_chance`/`_rarity_floor` |
 | 40188 | Zephyr Treads | feet | `40188-zephyr_treads.yaml` | `wornconditionids: [98]`, `staminamax` statmod |
-| 40189 | Staff of the Hollow Choir | weapon (2H staff) | `40189-staff_of_the_hollow_choir.yaml` | `spell_damage_multiplier`, `procs` (on_spell_hit steal_pool), `casting`/`manifestation` statmods |
+| 40189 | Staff of the Hollow Choir | weapon (2H staff) | `40189-staff_of_the_hollow_choir.yaml` | `spell_damage_multiplier`, `behavior: hollow_choir` (on_spell_hit steal_pool proc, slice 3), `casting`/`manifestation` statmods |
 
 **Condition (worn):**
 
````

**Modify `internal/behaviortree/context.md`:**

````diff
@@ -180,7 +180,10 @@ tree:
 | `player_enter` | A player enters the mob's room | `UserId` = player |
 | `item_idle` | Item trees only: once a round from the item tick (`hooks.ItemRoundTick`, lighting 5e) | `RoomId` = the item's room; the subject is `ctx.Item` |
 | `on_equip` / `on_unequip` | Item trees only: the item was put on / taken off (`hooks.ItemEquipEvents`, item behaviour slice 2) | `ctx.Item.Slot` = its slot on equip, "" on removal |
-| `on_kill` | Item trees only: the holder had a hand in a kill; every worn treed item (`hooks.MobDeathItemProcs`) | `ctx.Item.Slot` = its slot |
+| `on_kill` | Item trees only: the holder had a hand in a kill; every worn treed item (`hooks.MobDeathItemProcs`); kill lines and `on_kill` procs (ruling S4) | `ctx.Item.Slot` = its slot; `Proc` nil |
+| `on_hit` / `on_spell_hit` | Item trees only: the holder's weapon hit, or its spell landed (`hooks.fireItemProc`, item behaviour slice 3) | `ctx.Item.Slot` = `weapon`; `Proc` = opponent, room, damage |
+| `on_block` | Item trees only: the holder blocked (`hooks.fireItemProc`) | `ctx.Item.Slot` = `offhand`; `Proc` |
+| `on_grapple` | Item trees only: a grapple round, each side of the hold (`hooks.fireItemProc`) | `ctx.Item.Slot` = `body`; `Proc` |
 | `on_hunger_feeding` | Item trees only: a hungry weapon fed on its holder (`hooks.tickHunger`), paced by `HungerFeedingLineCooldownRounds` | `ctx.Item.Slot` = `weapon` |
 
 Any node may include `event: <type>` to skip that branch when the event
@@ -992,7 +995,7 @@ Spec: `docs/superpowers/specs/completed/2026-05-12-mob-aliveness-2.6-sunset-tact
 | Actions — other | `actions_dialogue.go`, `actions_quest.go`, `actions_progression.go`, `actions_mutation.go`, `actions_scout.go`, `actions_skullduggery.go` |
 | Conditions | `conditions.go`, `conditions_combat.go`, `conditions_mob.go`, `conditions_player.go`, `conditions_party.go`, `conditions_position.go`, `conditions_room.go`, `conditions_state.go`, `conditions_scout.go`, `conditions_forager.go`, `conditions_skullduggery.go`, `conditions_submission.go` |
 | Misc | `archetype_shift.go`, `room_state.go` |
-| Items (lighting 5e) | `item_engine.go`, `item_state.go`, `conditions_item.go`, `actions_item_light.go`, `item_voice.go`, `actions_item_voice.go` |
+| Items (lighting 5e) | `item_engine.go`, `item_state.go`, `conditions_item.go`, `actions_item_light.go`, `item_voice.go`, `actions_item_voice.go`, `actions_item_proc.go` |
 
 The `actions_*` / `conditions_*` split is the whole architecture: a tree is
 authored data, and extending the engine means adding a named action or
@@ -1024,7 +1027,7 @@ Items are the third subject of the engine, beside mobs and rooms (spec
   `loader.go`): `time_of_day`, `round_mod`, `random_chance`, `state_equals`,
   `state_greater_than`, `holder_asleep`, `worn`, `in_combat`, `chatter_ready`,
   `hunger_overdue`; `set_state`, `increment_state`, `decrement_state`,
-  `set_light`, `pulse_light`, `speak`, `taunt_pull`; every
+  `set_light`, `pulse_light`, `speak`, `taunt_pull`, `proc`; every
   decorator and composite. Anything else refuses with its path. The item-only
   nodes (`itemOnlyNodes`) refuse in a mob or room tree.
 - **Subject.** `EvalContext.Item *ItemSubject{UUID, ItemId, UserId,
@@ -1106,3 +1109,41 @@ the `SentientChatter*` knobs are retired.
 - **Shipped trees.** `blackrazor.yaml` and `aegis.yaml`: worn, then
   `chatter_ready`, then taunt in combat (the Aegis pulls), a hunger warning
   past 0.75 (the Blackrazor), else idle; the event branches beside it.
+
+## Item procs (item behaviour slice 3)
+
+A combat proc is an item tree branch (`actions_item_proc.go`); the
+Pinnacle proc path (`hooks/item_procs.go`, `items.ItemProc`, `procs:`,
+`ProcsFor`, the builder's proc editor) is retired.
+
+- **`proc(effect, ...)`** runs `procLifesteal`, `procStealPool`,
+  `procAoeStun` or `procApplyCondition` (moved unchanged from hooks) for
+  `itemHolder(ctx)` against `ctx.Event.Proc` (`ProcEvent{Other, Room,
+  Damage}`; nil on a kill, read as zero). The other params are the
+  effect's numbers. It returns Success only when the effect did something
+  and refuses while `ItemProcsOn()` (`GamePlay.ItemProcsEnabled`) is off.
+- **Compile.** `checkProcNodes` refuses a `proc` whose nearest `event:` is
+  not `on_hit`, `on_kill`, `on_block`, `on_grapple` or `on_spell_hit`, an
+  unknown effect, a param its effect does not read (`procEffectParams`) or
+  that is not a number, and a `random` over a proc outside 1 to 99 (X19:
+  at 100 the branch omits it, so nothing is drawn). In an item tree,
+  `compileDecorator` turns a `cooldown` whose subtree names `proc` into a
+  `ProcCooldownDecorator` and such a `random` into a `ProcRandomDecorator`.
+- **`ProcCooldownDecorator`** (Rule 22, ruling R1) keeps the round the
+  branch may fire again in the HOLDER's MiscData,
+  `item_proc_cd_<itemId>_<compile path>` (`procCooldownKey`), read through
+  `characters.MiscRound`: per template, shared by two copies, kept across a
+  relog. It arms only on Success. Every other item cooldown stays in tree
+  state.
+- **`ProcRandomDecorator`** draws only while procs are on: a kill reaches a
+  proc branch through `fireWornItemEvent`, without the dispatcher's gate.
+- **Events.** `hooks.fireItemProc` fires `on_hit`, `on_spell_hit` (weapon),
+  `on_block` (offhand) and `on_grapple` (body, both sides) into the one
+  item the event reaches, after reading `ItemProcsOn()`; `on_kill` procs
+  ride `MobDeathItemProcs`' event into every worn treed item (ruling S4).
+- **Shipped trees.** `blackrazor.yaml` (on_hit lifesteal 0.25),
+  `aegis.yaml` (on_block aoe_stun, 10%, cooldown 20),
+  `thornwall_harness.yaml` (on_grapple bleed, 50%, cooldown 5),
+  `hollow_choir.yaml` (on_spell_hit conviction steal 0.08, cooldown 3).
+  `hooks/testdata/item_proc_parity.golden` is the retired path's record;
+  `TestItemProcParity` holds the trees to it.
````

**Modify `internal/characters/context.md`:**

````diff
@@ -496,7 +496,8 @@ and `applyVitalChange` (the single signed pipeline behind harm and restore).
 - **A green pool-mutation guard does not mean every pool write is routed.**
   `resources.go` is exempt as a FILE, so `Heal()`'s writes are invisible and so
   are its three production callers (`actions/combat_drain.go:126`, `:281`,
-  `hooks/item_procs.go:99`). They retire with `Heal` in U5c.
+  `behaviortree/actions_item_proc.go:213`, the lifesteal proc). They retire
+  with `Heal` in U5c.
 - **`CostCommitResult.Short()` strips only the governing skill term** for a
   partially paid autoattack, winning defence, flee, or grapple participant. The
   action still resolves and does not inherit unpaid debt.
````

**Modify `internal/hooks/context.md`:**

````diff
@@ -2209,7 +2209,7 @@ here. There is no `Input_*` or `Combat_*` prefix.
 
 The remaining 39 files are shared helpers rather than handlers and carry no
 prefix at all; they are the lowercase-named ones, for example
-`combat_shared_helpers.go`, `spell_resolution.go`, `item_procs.go`,
+`combat_shared_helpers.go`, `spell_resolution.go`, `item_proc_dispatch.go`,
 `machine_resolver.go`, `tick_cause.go` (the death-cause tag a damaging
 health tick stamps; see "The damaging condition tick" above), and
 `light_spell.go` (see "Vision-scaled spells" below). `hooks.go` is in that
@@ -2259,3 +2259,22 @@ you" fallback when no tree handles it. `pinnacle_voice_next_round` is inert
 in old saves. `testdata/item_voice_parity.golden` is the Pinnacle voice
 path's 200-round record, frozen before the move; `TestItemVoiceParity`
 holds the tree path to it.
+
+**Item procs (item behaviour slice 3).** `item_procs.go` is deleted
+(`dispatchItemProcs`, `procGateOpen`, `markProcCooldown`,
+`procBearingItems`, `readMiscRound`; the four effects moved to
+`behaviortree/actions_item_proc.go`). `fireItemProc(event, owner, other,
+room, damage)` (`item_proc_dispatch.go`) reads `ItemProcsEnabled` first,
+picks the one item the event reaches (`on_hit` and `on_spell_hit` the
+weapon, `on_block` the offhand, `on_grapple` the body) and runs its tree
+with `EventContext.Proc`. Its six call sites spell their `EventType`
+literal: `NewRound_DoCombat_unified.go` (on_hit, on_block),
+`Position_GrappleTick.go` (on_grapple, both sides),
+`spell_effects.go` (on_spell_hit, two). `MobDeathItemProcs` no longer
+dispatches a separate on_kill proc: its one `on_kill` event carries kill
+lines and procs to every worn treed item (ruling S4). The Pinnacle tick
+reads MiscData rounds through `characters.MiscRound`. The proc cooldown
+key `pinnacle_proc_cd_*` is inert in old saves.
+`testdata/item_proc_parity.golden` is the retired path's record (four
+items, five events, hits, misses, cooldown windows and a probe draw each
+round); `TestItemProcParity` holds the tree path to it.
````

**Modify `internal/items/context.md`:**

````diff
@@ -1357,7 +1357,6 @@ and `TestPreDetuneBowTable_MatchesTheRealTemplates` both fail otherwise.
 | `bauble_placement.go` | Where a found bauble lies: `LeaveBaubleAt`, `ClearBaublePlacement`, `BaubleBelongsTo`, `BaubleUntakenFor`, `BaubleSpotSuffix` |
 | `spec_baseline.go` | `SpecBaseline`: pre-enchant numeric snapshot, so a tier re-apply cannot wipe affix scaling |
 | `detune_migration.go` | U10d ranged-weapon rescale (`MigrateDetunedBow`); idempotent by value threshold, no run-once marker |
-| `proc_accessors.go` | On-hit proc access |
 | `reach.go` | Weapon reach data |
 | `attack_messages.go` / `defensive_messages.go` | Combat message pools. Both render a coordinated triad through `narration.Render`; see below |
 | `memory.go` | Memory reporting |
````

- [ ] **Step 2: Checks**

```bash
python tools/context_md_audit.py 2>&1 | grep -A6 "^internal/\(behaviortree\|hooks\|items\|characters\)\b\|^modules/gmcp"
git diff origin/master -- docs internal/*/context.md | grep "^+" | grep -c "—\|–"
```

Expected: no output, then `0` (docs only; moved Go comments are not counted). (`grep -c` exits 1 on a zero count: run it on its own line, not in an `&&` chain.)

- [ ] **Step 3: Spec check and commit**

```bash
P="docs/PATCH_NOTES.md docs/schemas/behavior.md docs/schemas/pinnacle-items.md internal/behaviortree/context.md internal/hooks/context.md internal/items/context.md internal/characters/context.md"
git diff d7d178073 -- $P
git add $P
git commit -m "docs: item procs (item behaviour slice 3)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: The playtest scenario, and the gates

Checkpoint `6f1ae983d`. Three actors, each in a private arena from Sable in the Rift Chamber (5000): the Blackrazor wielder reports health after hits; the Aegis-and-Harness bearer reports the shockwave line, relogs right after one and reports the next (not inside 80 s), then grapples and reports the bleed; the caster with the Staff and `conviction-spike` reports conviction after each landed spell.

- [ ] **Step 1: Write the scenario**

**Create `tools/playtest/goals/scenarios/item-procs/blade.yaml`:**

````yaml
# Item procs: the BLACKRAZOR wielder. You are Sil Vantage, starting in the
# Rift Chamber (5000). The Blackrazor, a two-handed sword that drinks its
# victims' life, is in your pack. Report your health VERBATIM as the game
# shows it, with the round or time.
ephemeral:
  profile: m2-actor
  start_room: 5000
  overlays:
    grant_items: [40183]
    set_gold: 500
  budgets:
    wall_clock: 25m

goals:
  - >-
    `look`, `inventory`, `equipment`. Then `equip blackrazor`. Report your
    current and maximum health.
  - >-
    `ask sable arena 100` and `attack` the arena mob. After every round,
    report your health and whether your swing landed. Health should rise a
    little after a hit that lands (the blade heals you by a share of the
    damage), never after a miss, and never above your maximum.
  - >-
    Fight until the mob dies or you are below a third of your health (then
    `flee`). Run a second arena fight if the first was short.
  - >-
    reads-well: report any raw number in a line about your blade, any line
    over 80 columns, any em dash, and any line that appeared twice.
````

**Create `tools/playtest/goals/scenarios/item-procs/caster.yaml`:**

````yaml
# Item procs: the HOLLOW CHOIR caster. You are Fold Adept, starting in the
# Rift Chamber (5000). The Staff of the Hollow Choir, which drinks the will
# of whatever your spells strike, is in your pack. Report your conviction
# VERBATIM as the game shows it, with the round or time.
ephemeral:
  profile: specialist-caster
  start_room: 5000
  overlays:
    grant_items: [40189]
    grant_spells:
      conviction-spike: 1
    set_gold: 500
  budgets:
    wall_clock: 25m

goals:
  - >-
    `look`, `inventory`, `equipment`. Then `equip staff`. Report your
    current and maximum conviction.
  - >-
    `ask sable arena 100`. In the fight, `cast conviction-spike` at the
    arena mob every round you can. After each cast report your conviction
    and whether the spell landed. Conviction should rise a little after a
    spell lands (the staff drains the target), but not after every single
    landed spell: it rests a few rounds between drains.
  - >-
    Fight until the mob dies or you run low; `flee` if you are losing. Run a
    second fight if the first was short.
  - >-
    reads-well: report any raw number in a line about your staff, any line
    over 80 columns, any em dash, and any line that appeared twice.
````

**Create `tools/playtest/goals/scenarios/item-procs/shield.yaml`:**

````yaml
# Item procs: the AEGIS and HARNESS bearer. You are Veteran Pathfinder,
# starting in the Rift Chamber (5000). The Aegis of Mockery (a shield) and
# the Thornwall Harness (barbed body armour) are in your pack. Quote every
# line about your gear VERBATIM, with the round or the clock time.
ephemeral:
  profile: veteran
  start_room: 5000
  overlays:
    grant_items: [40185, 40186]
    set_gold: 1000
  budgets:
    wall_clock: 25m

goals:
  - >-
    `look`, `inventory`, `equipment`. `equip aegis` and `equip harness`
    (remove whatever is in those slots first). Report your equipment.
  - >-
    SHOCKWAVE. `ask sable arena 100` and fight the arena mob, letting it
    attack you. Quote every "A jarring shockwave ripples outward ..." line
    with its clock time, and say whether the mob seemed staggered after it.
    It comes about one block in ten, then not again for about 80 seconds.
  - >-
    RELOG. Right after a shockwave, note the clock time, type `quit`, log
    back in at once, go back to Sable and start another arena fight. Report
    the time of the next shockwave. It must be at least 80 seconds after
    the one before you quit; sooner is the defect to report.
  - >-
    BLEED. In an arena fight, `grapple` the mob and hold it for several
    rounds. Quote every line saying the mob bleeds or is wounded by your
    barbs, with the round. If `grapple` is refused, quote the refusal.
  - >-
    reads-well: report any raw number in a line about your gear, any line
    over 80 columns, any em dash, and any line that appeared twice.
````

**Create `tools/playtest/scenarios/item-procs.yaml`:**

````yaml
# Item behaviour slice 3: item procs. The four proc items fire through their
# behaviour trees now, with the same numbers as before: the Blackrazor
# (40183) heals its wielder by a quarter of each hit's damage; the Aegis of
# Mockery (40185) staggers every hostile in the room on a block, one time in
# ten, then rests 20 rounds (about 80 seconds); the Thornwall Harness
# (40186) opens a bleeding wound on whatever grapples its wearer, half the
# time, at most once every 5 rounds; the Staff of the Hollow Choir (40189)
# drains a share of a spell target's conviction into its caster, at most
# once every 3 rounds. A proc's rest now lives on the character, so it
# survives a relog (ruling R1): the shield bearer relogs inside the Aegis's
# rest to show it.
#
# Fixtures (verified against room YAML 2026-10-06):
#   5000 The Rift Chamber (dungeon, room lamp 38 plus the Rift Stone). Sable
#     is here: `ask sable arena <gold>` (at least 100) opens a lit arena
#     fight. Each asker gets a PRIVATE arena, so the three actors fight
#     apart; no actor can see another's fight.
#   Rounds are 4 seconds (Timing.RoundSeconds).
name: item-procs
mode: party
summary: >-
  A Blackrazor wielder, an Aegis-and-Harness bearer and a Hollow Choir caster
  each fight in their own arena and report what their gear does; the shield
  bearer relogs inside the Aegis's rest.
on_actor_stop: continue
budgets:
  wall_clock: 25m
requires:
  max_connections: 20
roster:
  - id: blade
    personality: bug-finder
    goals: goals/scenarios/item-procs/blade.yaml
  - id: shield
    personality: bug-finder
    goals: goals/scenarios/item-procs/shield.yaml
  - id: caster
    personality: bug-finder
    goals: goals/scenarios/item-procs/caster.yaml

group_goals:
  - id: lifesteal
    do: The blade bearer fights an arena mob with the Blackrazor.
    verify: >-
      The wielder's health rises a little after hits that land, and never
      above its maximum.
  - id: shockwave
    do: The shield bearer blocks an arena mob's attacks with the Aegis.
    verify: >-
      Now and then "A jarring shockwave ripples outward, staggering the
      hostile creatures nearby!" appears, never twice within about 80
      seconds.
  - id: relog-rest
    do: The shield bearer quits and logs back in right after a shockwave.
    verify: >-
      No second shockwave comes within about 80 seconds of the first, the
      relog included.
  - id: bleed
    do: The shield bearer grapples an arena mob while wearing the Harness.
    verify: The mob starts bleeding during the hold.
  - id: conviction-drain
    do: The caster lands damaging spells while wielding the Staff.
    verify: >-
      The caster's conviction rises a little after a spell lands, and the
      target's falls, not more often than every few rounds.
  - id: reads-well
    do: Everyone judges every gear line as prose.
    verify: No raw numbers, no em dashes, nothing over 80 columns, no doubled line.
````

- [ ] **Step 2: Commit**

```bash
P="tools/playtest/scenarios/item-procs.yaml tools/playtest/goals/scenarios/item-procs/blade.yaml tools/playtest/goals/scenarios/item-procs/shield.yaml tools/playtest/goals/scenarios/item-procs/caster.yaml"
git diff 6f1ae983d -- $P
git add $P
git commit -m "test(playtest): item procs scenario (item behaviour slice 3)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 3: Gates**

```bash
gofmt -l internal/ modules/
go vet ./...
go build ./...
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
golangci-lint run --new-from-merge-base=origin/master
```

Expected: gofmt, vet and the test grep print nothing; lint `0 issues.` Known flake: `internal/playtestrun` can hang under load; re-run it alone before calling it a failure.

- [ ] **Step 4: Race run in Docker** (no gcc on this box, so `go test -race` cannot build locally)

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-5e3-test . > "$TMP/5e3-build.log" 2>&1
docker run --rm dogmud-5e3-test > "$TMP/5e3-race.log" 2>&1
grep -c "WARNING: DATA RACE" "$TMP/5e3-race.log"
grep -E '^(--- FAIL|FAIL)' "$TMP/5e3-race.log"
docker rmi dogmud-5e3-test
```

Expected: `0` races; the only failures are the two tests that shell out to `git`, which has no repository inside the image (`TestNoStringOrDataSaysBuff` in the root package, `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`), with their `FAIL` package lines. The dry run (2026-10-06) gave exactly that, 128 packages `ok`. Proc cooldowns are written to MiscData from the same call sites the old `markProcCooldown` wrote from, and `util.Rand`'s seam is atomic; a race here is a real one.

- [ ] **Step 5: Boot check on private ports**

```bash
O="$TMP/boot-overrides.yaml"
printf 'Network.TelnetPort: [33334]\nNetwork.LocalPort: 9998\nNetwork.HttpPort: 8091\nNetwork.HttpsPort: 0\nNetwork.AIPort: 0\n' > "$O"
go build -o boot-check.exe .
CONFIG_PATH="$O" LOG_NOCOLOR=1 timeout 150 ./boot-check.exe > "$TMP/boot.log" 2>&1; echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$TMP/boot.log"
grep -c "Server Ready" "$TMP/boot.log"
rm boot-check.exe
```

Expected: `exit=124` (it stayed up), `0`, `1`. Never kill a server by name or port: the owner runs their own.

---

### Task 7: Playtest and PR

- [ ] **Step 1: Playtest** with `playtest-scenario` and `tools/playtest/scenarios/item-procs.yaml`, run the way slice 2 ran (handoff 2026-10-06): build `playtestrun.exe`, launch each mudagent with a scratch launcher reading `control/creds.json` by `actor_id` (never echo a password), one sonnet driver agent per actor on a shared clock. Afterwards stop the run and kill orphaned `tail -f` feeders BY PID (their command line contains the run id). File findings as GitHub issues on `pruuk/DOGMud` (search first); fix in-scope defects on the branch in their own commits.
- [ ] **Step 2: Whole-branch review** (opus), findings fixed in their own commits, then push and open the PR:

```bash
git push -u origin feature/item-behaviour-slice3
gh pr create --repo pruuk/DOGMud --base master --head feature/item-behaviour-slice3 --title "Item behaviour slice 3: item procs (lighting 5e)" --body-file "$TMP/pr-body.md"
```

The body names #223 as answered with "Answers #223", and #365 as "Part of #365": no close, fix or resolve keyword before an issue number unless that issue should close at merge. Read back the URL `gh` prints: it must say `pruuk/DOGMud`. Merge with `gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch` after checks are green and confirmed with `gh run list --repo pruuk/DOGMud`. The owner deploys.

---

## Self-review

- Spec coverage: Rule 20 (Task 2), Rule 21 (Task 3), Rule 22 and R1 (Tasks 2, 4: shared, relog), Rule 23 and X21 (Task 4), X19 (Tasks 2, 4: no draw at 100, none when off), the "Slice 3" gates (record on both paths before deleting: Tasks 1, 3; 100% draws nothing: Task 2; switch-off: Tasks 2, 4; relog and two copies: Task 2; playtest: Tasks 6, 7), "Every slice" docs and gates (Tasks 5, 6). S4 (Tasks 3, 4).
- No knob added or retired (spec: slice 3 adds none); `config.yaml` is untouched.
- Every symbol named in the plan's prose exists at its checkpoint; the facts table was read at `de8cf323a`.
