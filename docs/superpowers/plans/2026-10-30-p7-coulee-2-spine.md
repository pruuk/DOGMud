# #375 Phase 7 Pothole Coulee, Plan 2: The Spine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A brand-new character is walked from waking to the seven trades one NPC at a time: Dewey offers and takes Waking to Gaius (28), Cleric Hadwen offers and takes The Awakening (30), Hadwen offers Toke's Road (88), Crier Toke takes it and offers Find Your Footing (31), and Toke is then the only starter of the seven trades. Nothing grants a quest by itself, quest 29 is retired, and 28 backlog lines go.

**Architecture:** Content-first. Quest 28 starts at a new Dewey offer topic and ends at his `ready` topic turned hand-in (`behavior: true`), whose tree branch plays the farewell and moves the player to the pool. Quest 30 loses its two automatic starts and gains a `woken` step: Hadwen's rite tree grants `30-woken` where it granted `30-end`, and a new Hadwen hand-in grants `30-end`. Quest 88 loses its trigger finisher; Toke's footing offer gates on `88-end`. The seven Giver 1 offers go, replaced by opener breadcrumbs to Toke. Quest 29's file goes, its step ids are retired, and a small load-time drop clears a retired-whole quest from saves (the existing deleted-quest migration only runs when the user index is missing).

**Tech Stack:** Go 1.x, YAML content under `_datafiles/world/dogmud/`

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md (sections "The opening", "Toke is the hub", Delivery row 2)

**Branch / worktree:** `feat/375-p7-coulee-spine`, worktree `C:/tmp/dogmud-375-p7-coulee-spine` off master **after plan 1 (engine) has merged**. Every path below is relative to that worktree unless it starts with `C:/`.

---

## Spec deviations found

1. **The pool move cannot stay in The Landing's tree.** Spec "The opening" item 1 says "the existing 6467 handoff moves the player to the pool". Room trees never hear a topic: `player_topic` is a mob event (`internal/behaviortree/events.go:36`, "a player chose a behavior talk topic on this mob"), and `room_idle` carries no player for `move_player` (`internal/behaviortree/actions_room.go:102-105` fails on `UserId <= 0`). So the farewell and the move go to Dewey's own tree, fired by the hand-in's `behavior: true` (`internal/conversation/talk.go:316-327`, `after`). The 6467 tree keeps its arrival prompt, re-prompt and walk block, stops intercepting `talk`, grants nothing, and gains a rescue branch for a player who holds `28-end` but is still in the room.
2. **The brief's loop-sweep command skips silently.** `go test -run 'TestSmoke_|...' .` skips every shipped-data sweep unless `DOGMUD_BOOT_SMOKE=1` is set (`quest_loop_smoke_test.go:28-30`, `boot_smoke_test.go:36`). Every command below sets it.
3. **Retiring quest 29 needs a load-time drop.** `users.DoQuestStepMigration` drops a deleted quest from saves (`internal/users/migration.go:66-80`) but runs only when the user index file is missing (`main.go:423-427`). `LoadUser` only logs a retired current step (`internal/users/users.go:573-576`). A save left on `29-start` would keep a deleted quest as the lowest-id active quest, which `GetFocusQuestId` picks (`internal/characters/quests.go:449-475`), so a bare `hint` would answer "You don't have any current quests" while the player is on quest 31 (`internal/usercommands/hint.go:83-87`). Task 1 adds a manifest-driven drop: a quest whose every step id sits under `# retired` in `step-ids.txt` is cleared from progress and the shelf at load.
4. **Spec facts row on Hadwen's tree lines.** The spec cites `9100-cleric_hadwen.yaml:71,88,253` as grants. Line 71 is a `player_missing_quest "30-end"` condition; the grants are lines 86-88 (`30-rite`) and 251-254 (`30-end`). Lines 277-278 and 298-299 also read `30-end` (greeting and ambient). All are handled in Task 4.
5. **Giver breadcrumbs are opener variants.** `breadcrumb` is allowed on offer topics only (`internal/dialogue/world.go:139-160`), and the Giver 1 offers are being removed. Each giver's existing "Opened, not on this quest" opener variant is rewritten to send the player to Toke.
6. **Side effects the spec does not name, all intended by the rules:** quest 28 becomes abandonable as well as 30 (its only start is now Dewey's offer, `internal/dialogue/offers.go:48-59`); quest 89 becomes abandonable because its only offer no longer excludes another quest's token once `29-end` goes (quest 88 stays unabandonable through `31-start`); the pool's movement gate (`behaviors/rooms/pothole_coulee/5200.yaml`) must name the offer, the rite and the hand-in, since `30-start` no longer arrives by itself.
7. **Quest 89 keeps Hadwen's offer** (`road_esk`) in this PR, as the spec's Delivery section requires; plan 4 removes it. The new `another_way` info topic ships beside it.

## Facts verified against source (master `c4af0286b`, 2026-10-30)

| Fact | Source |
|---|---|
| Quest 28 is started by `room_enter 6258` (grant `28-start` plus 4 Dewey lines, delays 1+2+2+2) | `quests/28-waking_to_gaius.yaml:172-190` |
| Quest 28's first `look guide` trigger grants both `28-looked` and `28-examined`; the second grants `28-examined` only; the bare `look` trigger matches `look guide` too and comes last | `quests/28-waking_to_gaius.yaml:205-260` |
| Quest 28 `fled` step has no `destination:` | `quests/28-waking_to_gaius.yaml:157-161` |
| Room 6467's tree intercepts `talk`/`ask`/`say` naming Dewey, moves to 5200 at 4.0 s, grants `28-end` and `30-start` at 4.5 s | `behaviors/rooms/newcomer_antechamber/6467.yaml:29-114` |
| The Landing (6467) has `exits: {}` and spawns Dewey 9491 | `rooms/newcomer_antechamber/6467.yaml:10-12` |
| Dewey 9491 spawns in all six antechamber rooms | `rooms/newcomer_antechamber/6258.yaml:17`, `6259.yaml:15`, `6260.yaml:14`, `6261.yaml:14`, `6262.yaml:14`, `6467.yaml:12` |
| Dewey's tree: flee re-nag (dash at line 57), way-north nag, goal planner, silent `player_enter` | `behaviors/newcomer_antechamber/9491-dewey.yaml:31-95` |
| Dewey dialogue: no offer or hand-in; `ready` is info with the #690 line "when you break away from that fight you reach the landing"; `marks` gated `28-asked`; `quest` ungated; no dash in the file | `dialogue/newcomer_antechamber/9491.yaml:131-143,184-201` |
| The new-to-MUDs route moves the player into an ephemeral copy of 6258 and fires no mob `player_enter`; the quest engine's `room_enter` comes from `hooks.QuestRoomEnter` | `internal/usercommands/start.go:210-248`; `internal/hooks/RoomChange_QuestRoomEnter.go:30-48` |
| The mud-vet route moves to 5200 and fires `player_enter` on its mobs; the veteran route queues `30-end` | `internal/usercommands/start.go:180-204,278-283` |
| `move_player` fires no mob `player_enter` (only the quest engine `room_enter`) | `internal/behaviortree/actions_room.go:102-127`; `player_enter` dispatch sites: `go.go:448`, `start.go:197`, `PlayerSpawn_HandleJoin.go:233` |
| A behavior hand-in plays (grant, text, hints), then fires the mob tree as `player_topic` and closes the menu | `internal/conversation/talk.go:255-268,279-293,316-327` |
| `ValidateWorld`: a `behavior: true` topic needs a tree `topic_is` for its key; offer/hand-in label must be the quest title; every gate token (variants included) must exist | `internal/dialogue/world.go:101-107,164-174,205-216,270-277` |
| `Breadcrumbs`: offers only, said while the offer is closed and its start not held | `internal/dialogue/topics.go:137-152`; `world.go:139-160` |
| After a hand-in, exactly one newly opened offer on that NPC is pitched automatically, else the menu returns | `internal/conversation/nextoffer.go:15-47` |
| Quest 30 is started by `room_enter 5200` and by 6467's tree; Hadwen also offers `rite_offer` | `quests/30-the_awakening.yaml:52-62`; `6467.yaml:109-112`; `dialogue/pothole_coulee/9100.yaml:153-168` |
| Hadwen's tree grants `30-rite` (line 86-88) and `30-end` (251-254); reads `30-end` at 69-71, 276-278, 297-299; dash at 129 | `behaviors/pothole_coulee/9100-cleric_hadwen.yaml` |
| The pool (5200) blocks every walk while `30-end` is missing and says "choose to begin the rite" | `behaviors/rooms/pothole_coulee/5200.yaml:23-42` |
| Hadwen's dialogue reads `29-end` in an opener variant and in `road_toke`/`road_esk` | `dialogue/pothole_coulee/9100.yaml:30-38,115,133` |
| Quest 88 is also ended by a `quest_granted 31-start` trigger | `quests/88-tokes_road.yaml:40-48` |
| Toke's `footing_offer` gates on `30-end`; `road_toke_end` excludes `31-start`; the seven `send_*` offers gate on `31-end` and Toke holds `footing_turnin` | `dialogue/pothole_coulee/9106.yaml:85-100,123-144,156-175,180-301` |
| Giver 1 offers gated on `30-end`: Vorn `train_offer` (9108:79-97), Rusk `craft_offer` (9116:74-92), Birna `brew_offer` (9128:83-101), Tarn `sign_offer` (9136:54-69), Grieve `cast_offer` (9144:50-66), Wenna `words_offer` (9153:50-66), Iden `shoot_offer` (9161:73-89); each hand-in stays | `dialogue/pothole_coulee/*.yaml` |
| Each giver's second opener variant (`30-end`, missing `N-start`/`N-end`) says "Choose <quest> below" | 9108:16-23, 9116:16-23, 9128:16-24, 9136:15-23, 9144:17-25, 9153:18-26, 9161:16-24 |
| Quest 29: 6 backlog lines (5 `end`, 1 `start`); step ids `29-end`, `29-start` | `quests/loop-backlog.txt:48-52,246`; `quests/step-ids.txt:89-90` |
| Neither manifest has a `# retired` section yet; both are CRLF on this machine (`core.autocrlf=true`), and parsing trims each line | `quests/step-ids.txt`; `dialogue/topic-keys.txt`; `internal/manifest/manifest.go:25-72` |
| A manifest key may leave content only through `# retired` / `# renamed` | `internal/guards/step_manifest_test.go:39-55`; `internal/guards/topic_manifest_test.go:40-55` |
| `ruleChain`: an offer gated on N's final token needs a hand-in for N on the same NPC; findings keyed `chain <N> mob <id> topic <key>` | `internal/questloop/rules_more.go:14-52` |
| `ruleStart`/`ruleEnd`: an external (tree) grant of a start or end token is a finding | `internal/questloop/rules_loop.go:15-55`; `grants.go:13-31` |
| `AutoStarts`: a trigger grant or a non-item external grant of `N-start`; abandon needs a resuming offer and no auto start | `internal/questloop/autostarts.go:16-34`; `internal/usercommands/questlog.go:104-110`; `internal/dialogue/offers.go:48-59` |
| A quest file with no `triggers:` is valid (quests 2, 17, 66, 1000000) | `quests/*.yaml` |
| A trigger needs at least one action; no grant is required | `internal/quests/quests.go:211-213` |
| A trigger's step for destinations is the latest step its `has` names; a trigger with no `has` belongs to no step | `internal/quests/triggers.go:43-51`; `internal/questengine/destination.go:250-273` |
| One-command rule over coulee quests (those a `pothole_coulee` dialogue or tree starts): every non-end step hint has exactly one "type " | `internal/questengine/newbie_one_command_steps_test.go:184-297` |
| Quest 28 pacing: each non-cooldown trigger's npc_say delays sum to 8 s or less; the two cooldown prompts sum to 17-20 s | `internal/questengine/quest28_pacing_test.go:27-92` |
| 7 backlog lines for quest 28; 10 `chain 30` lines; 2 `end 30`; 2 `start 30`; 1 `end 88` | `quests/loop-backlog.txt:7-16,46-47,53-54,109,191-192,244-245,247-248,286` |
| After this PR, 67 lines name quests 32 to 59, 76 or 89 (66 for plans 3a/3b, `end 89 ...room=468...` for plan 4); 95 = 28 cleared + 67 left, matching the spec's 88 + 7 | grep of `loop-backlog.txt` |
| `trip` on a prone target refuses but notifies the quest engine as a trip | `internal/usercommands/trip.go:57-67` |
| Tests pinned to the old flow: `TestEskPassageClosesTwoRoads` (loads quest 29), `TestQuest29TwoRoads_BothRoadsStillClose`, `TestLandingHandoffNeedsARealTalk`, `TestQuest28_LookLessonRewardsACloseLook` (expects 2 guide triggers), Hadwen grant lists `[30-rite 30-end]`, `TestSmoke_AutoStartedQuestsNotAbandonable` (expects 30 unabandonable) | `internal/guards/esk_return_portal_test.go:123-232`; `internal/questengine/newbie_hub_steps_test.go:104-115`; `internal/behaviortree/landing_handoff_test.go`; `internal/questengine/quest28_teaching_steps_test.go:283`; `internal/behaviortree/hadwen_rite_guard_test.go:146-147`; `hadwen_midrite_test.go:156`; `main_quests_offer_test.go:101-133` |
| Shipped-data smoke tests skip without `DOGMUD_BOOT_SMOKE=1` | `quest_loop_smoke_test.go:26-31` |

## File map

| Action | Path | Responsibility |
|---|---|---|
| Modify | `internal/quests/manifest.go` | `RetiredQuests()`: quests retired whole in `step-ids.txt` |
| Modify | `internal/quests/manifest_test.go` | test for it |
| Modify | `internal/characters/quests.go` | `DropRetiredQuests(retired)` |
| Modify | `internal/characters/migrate_newbie_test.go` | test for it |
| Modify | `internal/users/users.go` | call the drop in `LoadUser` |
| Delete | `_datafiles/world/dogmud/quests/29-two_roads.yaml` | quest 29 retired |
| Modify | `_datafiles/world/dogmud/quests/step-ids.txt` | retire `29-*`; add `30-woken` |
| Modify | `_datafiles/world/dogmud/quests/loop-backlog.txt` | delete the 28 cleared lines |
| Modify | `_datafiles/world/dogmud/dialogue/topic-keys.txt` | 3 new keys; 7 retired |
| Modify | `_datafiles/world/dogmud/quests/28-waking_to_gaius.yaml` | greeting without grant, offer-started, one-action look lesson, fled destination, #690 lines |
| Modify | `_datafiles/world/dogmud/dialogue/newcomer_antechamber/9491.yaml` | offer, hand-in, first-menu opener, marks open early |
| Modify | `_datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml` | send-off branch; dash fix |
| Modify | `_datafiles/world/dogmud/behaviors/rooms/newcomer_antechamber/6467.yaml` | no talk interception, no grants; rescue |
| Modify | `_datafiles/world/dogmud/quests/30-the_awakening.yaml` | no auto start; `woken` step |
| Modify | `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml` | `30-woken`; greeting; dash fix |
| Modify | `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml` | hand-in, skip-path info topic, no `29-end` |
| Modify | `_datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5200.yaml` | gate names the next step in each state |
| Modify | `_datafiles/world/dogmud/quests/88-tokes_road.yaml` | trigger finisher removed |
| Modify | `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` | footing gated on `88-end`; hub explanation |
| Modify | `_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`, `9116.yaml`, `9128.yaml`, `9136.yaml`, `9144.yaml`, `9153.yaml`, `9161.yaml` | offer removed; breadcrumb to Toke |
| Modify | `_datafiles/world/dogmud/quests/31-*.yaml`, `32-*.yaml`, `35-*.yaml`, `38-*.yaml`, `41-*.yaml`, `44-*.yaml`, `47-*.yaml`, `50-*.yaml` | header token-flow comments; 32 and 50 "Toke or" descriptions |
| Modify | `internal/usercommands/start.go`, `internal/behaviortree/actions_quest.go`, `main_quests.go`, `internal/dialogue/offers.go`, `internal/characters/quests.go` | comments only |
| Modify | tests listed per task | follow the new flow |
| Create | `coulee_spine_smoke_test.go` (repo root) | headless hand-off walk of the whole spine on shipped data |
| Modify | `internal/quests/context.md`, `internal/characters/context.md`, `internal/dialogue/context.md`, `internal/questloop/context.md`, `internal/usercommands/context.md` | API and example updates |
| Modify | `.claude/skills/dogmud-authoring-quests/SKILL.md` | abandon rules 3 and 5 examples |
| Modify | `docs/README.md` | esk test row; new smoke test row |
| Modify | `docs/PATCH_NOTES.md` | player-facing entry |

## Rules every task follows

- Edit files only with the Edit and Write tools. No `>`, `>>`, `sed -i` or Python writes into a repo file (a hook blocks them). Deleting a file uses `git rm <path>`.
- `git add` only the paths the task names. Never `git add -A` or `.`; never `--no-verify`.
- No em dash, en dash or ` -- ` in anything you write (prose, YAML text, comments).
- Every commit message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- When a task fixes a finding, the same task deletes its `loop-backlog.txt` line. Never add a line.
- Test commands run from the worktree root in Git Bash. `-race` cannot run on this machine; CI runs it.

---

## Task 0: Worktree and baseline

**Executor:** Sonnet

**Files:** none

- [ ] **Step 1: Create the worktree off merged master**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git log --oneline -1 origin/master   # must include plan 1's merge (grep the log for "quaff")
git log --oneline origin/master | grep -i quaff | head -3
git worktree add -b feat/375-p7-coulee-spine C:/tmp/dogmud-375-p7-coulee-spine origin/master
cd C:/tmp/dogmud-375-p7-coulee-spine
```

Expected: the grep prints at least one plan 1 commit. If it prints nothing, stop: plan 1 has not merged.

- [ ] **Step 2: Record the baseline**

```bash
cd C:/tmp/dogmud-375-p7-coulee-spine
wc -l _datafiles/world/dogmud/quests/loop-backlog.txt
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop' -timeout 600s . 2>&1 | tail -5
```

Expected: `296` lines (if plan 1 changed the backlog, note the new count; every later count is relative to it), and `ok`. If the sweep fails on master, stop and report.

---

## Task 1: Drop a quest retired whole from saves at load

**Executor:** Opus

**Files:**
- Modify: `internal/quests/manifest.go`
- Modify: `internal/quests/manifest_test.go`
- Modify: `internal/characters/quests.go`
- Modify: `internal/characters/migrate_newbie_test.go`
- Modify: `internal/users/users.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/quests/manifest_test.go`:

```go
// A quest is retired whole when every step id it ever had sits under
// "# retired" and none is still a key (#375 phase 7: quest 29 Two Roads).
// One retired step of a live quest does not retire the quest.
func TestRetiredQuests_OnlyWholeQuests(t *testing.T) {
	m, err := ParseStepManifest([]byte("30-start\n30-end\n# retired\n29-start\n29-end\n30-old\n"))
	require.NoError(t, err)
	require.Equal(t, map[int]bool{29: true}, retiredQuests(m))

	m, err = ParseStepManifest([]byte("30-start\n30-end\n"))
	require.NoError(t, err)
	require.Nil(t, retiredQuests(m))
}
```

Append to `internal/characters/migrate_newbie_test.go`:

```go
// A quest retired whole (#375 phase 7: quest 29 Two Roads) is dropped from
// progress, the shelf and the focus, so hint and the quest panel never point
// at a quest that no longer exists. Other quests are untouched, and a second
// run drops nothing.
func TestDropRetiredQuests(t *testing.T) {
	c := &Character{
		QuestProgress: map[int]string{29: "start", 30: "end", 31: "start"},
		ShelvedQuests: map[int]string{29: "start"},
		LastQuestId:   29,
	}
	got := c.DropRetiredQuests(map[int]bool{29: true, 77: true})
	if len(got) != 1 || got[0] != 29 {
		t.Fatalf("dropped %v, want [29]", got)
	}
	if _, ok := c.QuestProgress[29]; ok {
		t.Errorf("quest 29 is still in progress: %v", c.QuestProgress)
	}
	if c.IsShelved(29) {
		t.Errorf("quest 29 is still on the shelf: %v", c.ShelvedQuests)
	}
	if c.LastQuestId != 0 {
		t.Errorf("focus still on %d, want 0", c.LastQuestId)
	}
	if c.QuestProgress[30] != "end" || c.QuestProgress[31] != "start" {
		t.Errorf("other quests changed: %v", c.QuestProgress)
	}
	if again := c.DropRetiredQuests(map[int]bool{29: true}); len(again) != 0 {
		t.Errorf("second run dropped %v, want nothing", again)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/quests/ -run TestRetiredQuests_OnlyWholeQuests
go test ./internal/characters/ -run TestDropRetiredQuests
```

Expected: both fail to compile: `undefined: retiredQuests` and `c.DropRetiredQuests undefined`.

- [ ] **Step 3: Implement**

In `internal/quests/manifest.go`, after `StepRetired`, add:

```go
// RetiredQuests reads the shipped manifest for quests retired whole: every
// step id of the quest sits under "# retired" and none is still a key. A
// save holding such a quest has nothing left to resume, so users.LoadUser
// drops it (#375 phase 7: quest 29 Two Roads). A missing or unreadable
// manifest retires nothing.
func RetiredQuests() map[int]bool {
	m, ok := shippedStepManifest("quests.RetiredQuests()")
	if !ok {
		return nil
	}
	return retiredQuests(m)
}

func retiredQuests(m manifest.Manifest) map[int]bool {
	live := map[int]bool{}
	for _, k := range m.Keys {
		if id, _ := TokenToParts(k); id > 0 {
			live[id] = true
		}
	}
	out := map[int]bool{}
	for _, k := range m.Retired {
		if id, _ := TokenToParts(k); id > 0 && !live[id] {
			out[id] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
```

In `internal/characters/quests.go`, after `retiredQuestSteps`, add:

```go
// DropRetiredQuests clears every quest in retired (quests.RetiredQuests:
// retired whole in the step-id manifest) from progress and the shelf, through
// ClearQuestToken, which also moves the focus off it. It returns the ids it
// dropped, sorted. Idempotent, so it runs on every load with no marker.
func (c *Character) DropRetiredQuests(retired map[int]bool) []int {
	var out []int
	for qid := range retired {
		_, held := c.QuestProgress[qid]
		_, shelved := c.ShelvedQuests[qid]
		if !held && !shelved {
			continue
		}
		c.ClearQuestToken(quests.PartsToToken(qid, `start`))
		out = append(out, qid)
	}
	sort.Ints(out)
	return out
}
```

In `internal/users/users.go`, replace:

```go
	// A renamed quest step keeps the player on it (#375 phase 2): renames
	// come from the step-id manifest, quests/step-ids.txt.
	loadedUser.Character.RenameQuestSteps(quests.StepRenames())
```

with:

```go
	// A quest retired whole (every step id under "# retired" in
	// quests/step-ids.txt) has nothing left to resume, so it is dropped
	// before the step checks below (#375 phase 7: quest 29 Two Roads).
	for _, qid := range loadedUser.Character.DropRetiredQuests(quests.RetiredQuests()) {
		mudlog.Info("LoadUser", "user", loadedUser.Username, "dropped retired quest", qid)
	}
	// A renamed quest step keeps the player on it (#375 phase 2): renames
	// come from the step-id manifest, quests/step-ids.txt.
	loadedUser.Character.RenameQuestSteps(quests.StepRenames())
```

- [ ] **Step 4: Run them and see them pass; prove each can fail**

```bash
go test ./internal/quests/ -run TestRetiredQuests_OnlyWholeQuests
go test ./internal/characters/ -run TestDropRetiredQuests
go build ./...
```

Expected: `ok` twice; the build prints nothing. Null probe: temporarily change `!live[id]` to `true` in `retiredQuests`, rerun the quests test and see it fail naming `map[int]bool{29:true}` against `{29:true, 30:true}`; restore and rerun green.

- [ ] **Step 5: Commit**

```bash
git add internal/quests/manifest.go internal/quests/manifest_test.go internal/characters/quests.go internal/characters/migrate_newbie_test.go internal/users/users.go
git commit -F - <<'EOF'
feat(quests): drop a quest retired whole from saves at load

A quest whose every step id is under "# retired" in step-ids.txt has
nothing left to resume. LoadUser now clears it from progress, the shelf
and the focus, so hint never points at a deleted quest. The existing
deleted-quest migration only runs when the user index is missing.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 2: Retire quest 29 (Two Roads)

**Executor:** Sonnet

**Files:**
- Delete: `_datafiles/world/dogmud/quests/29-two_roads.yaml`
- Modify: `_datafiles/world/dogmud/quests/step-ids.txt`
- Modify: `_datafiles/world/dogmud/quests/loop-backlog.txt`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml`
- Modify: `internal/guards/esk_return_portal_test.go`
- Modify: `internal/questengine/newbie_hub_steps_test.go`
- Modify: `internal/behaviortree/actions_move_player_delayed_test.go`
- Modify: `internal/characters/quests.go` (one comment)
- Modify: `docs/README.md`

- [ ] **Step 1: Delete the quest file and see the guards fail**

```bash
git rm _datafiles/world/dogmud/quests/29-two_roads.yaml
go test ./internal/guards/ -run TestStepIdManifest
```

Expected: FAIL with `step 29-end is gone from quest content` and `step 29-start is gone`.

- [ ] **Step 2: Retire the step ids**

In `_datafiles/world/dogmud/quests/step-ids.txt` delete the two lines `29-end` and `29-start` (between `28-warcried` and `30-end`), then append at the very end of the file:

```
# retired
# Quest 29 Two Roads (#375 phase 7): no road grants it; users.LoadUser drops
# it from saves (quests.RetiredQuests).
29-end
29-start
```

- [ ] **Step 3: Remove every `29-` token from Hadwen's dialogue**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml`:

Delete this opener variant (with its comment line):

```yaml
  # No road open: a 29-end holder (old signpost), or both roads taken.
  - questRequired:
    - 30-end
    - 29-end
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. You have already set out from
      here, so the pool has done its part for you.
    hints: Choose a line below to hear about mutations, seeing in the dark, or
      your name.
```

and put this comment line in its place, directly above the variant that requires `30-end`, `88-start`, `89-start`:

```yaml
  # Both roads taken.
```

In `road_toke`, delete the line `  - 29-end` (under `- 31-start`). In `road_esk`, delete the line `  - 29-end` (under `- 89-end`). Replace the topics comment block

```yaml
# R17: the two roads that replace quest 29 "Two Roads". Take either, both or
# neither. Toke's road is hidden once footing has started (31-start), since
# the player is already on it. Both are hidden from 29-end holders, who chose
# a road under the old signpost (quest 29, retired). An old-save 29-start
# holder still sees both; taking one grants 29-end through quest 29's
# triggers, which hides the other, and both NPCs stay open to them anyway.
```

with

```yaml
# R17: the two roads from the pool. Take either, both or neither. Toke's
# road is hidden once footing has started (31-start), since the player is
# already on it. Quest 29 (Two Roads) is retired (#375 phase 7).
```

(Task 4 rewrites this file whole; this step only keeps each commit bootable.)

- [ ] **Step 4: Remove the tests that load quest 29**

In `internal/questengine/newbie_hub_steps_test.go` delete `TestQuest29TwoRoads_BothRoadsStillClose` and its two-line comment (the last 13 lines of the file, from `// Two Roads (quest 29): the hint now names one road's command, but both roads`).

In `internal/guards/esk_return_portal_test.go` delete `TestEskPassageClosesTwoRoads` and its comment (from `// TestEskPassageClosesTwoRoads: quest 29 "Two Roads" closes when the player` through the closing `}` before `// movePlayerDestinations`). Then make the import block exactly:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	yaml "gopkg.in/yaml.v2"
)
```

(`hooks.QuestRoomEnter` on a teleport keeps its own test, `TestQuestRoomEnter` in `internal/hooks/RoomChange_QuestRoomEnter_test.go`.)

In `internal/behaviortree/actions_move_player_delayed_test.go`:
- replace `// (hooks.QuestRoomEnter), which closes quest 29 on Esk's road (#619).` with `// (hooks.QuestRoomEnter), which closes quest 89 on Esk's road (#619).`
- replace `	u.Character.QuestProgress = map[int]string{30: "end", 29: "start"}` with `	u.Character.QuestProgress = map[int]string{30: "end", 89: "start"}`
- replace `// Quest 29 closes through its own room_enter trigger on 468, not a grant in` with `// Quest 89 closes through its own room_enter trigger on 468, not a grant in`
- replace `(room_enter on %d closes quest 29)` with `(room_enter on %d closes quest 89)`

In `internal/characters/quests.go` replace `	// Two Roads bridge completes into Find Your Footing.` with `	// Toke's Road hand-in leads into Find Your Footing.`

In both `_datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml` and `_datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5215.yaml` replace the two comment lines

```yaml
        # Quest 29 "Two Roads" closes on arrival through its own room_enter
        # trigger on 468, which a move_player teleport raises (#619).
```

with

```yaml
        # Quest 89 (Esk's Road) closes on arrival through its own room_enter
        # trigger on 468, which a move_player teleport raises (#619).
```

(and add both files to this task's `git add` below).

In `docs/README.md`, in the `esk_return_portal_test.go` row, delete the sentence ` Also pins that both arch transits close quest 29 "Two Roads" (a teleport raises no `room_enter`), once only, and only for a player still on its start step` so the row ends `...finds no error in either zone. |`.

- [ ] **Step 5: Delete quest 29's backlog lines**

In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete exactly these 6 lines:

```
end 29 quest 29 trigger quest_granted quest_token=31-start grants=29-end
end 29 quest 29 trigger quest_granted quest_token=88-start grants=29-end
end 29 quest 29 trigger quest_granted quest_token=89-start grants=29-end
end 29 quest 29 trigger room_enter room=468 grants=29-end
end 29 step end
start 29 step start
```

- [ ] **Step 6: Run and pass**

```bash
go build ./...
go test ./internal/guards/ ./internal/questengine/ ./internal/behaviortree/ ./internal/characters/
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators' -timeout 600s .
git grep -nE '\b29-(start|end)\b' -- '_datafiles/world/dogmud/quests/*.yaml' '_datafiles/world/dogmud/dialogue' '_datafiles/world/dogmud/behaviors'
```

Expected: `ok` for every package and the root; the final grep prints nothing (prove it can find something first: `git grep -n '30-end' -- _datafiles/world/dogmud/behaviors | head -1` prints a line).

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/quests/29-two_roads.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/quests/loop-backlog.txt _datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml _datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5215.yaml internal/guards/esk_return_portal_test.go internal/questengine/newbie_hub_steps_test.go internal/behaviortree/actions_move_player_delayed_test.go internal/characters/quests.go docs/README.md
git commit -F - <<'EOF'
content(quests): retire quest 29 Two Roads

The file goes, 29-start and 29-end move to the retired section of
step-ids.txt, its 6 backlog lines go, and Hadwen's dialogue stops
reading 29-end. Saves holding it are cleared at load (previous commit).

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 3: Quest 28 starts and ends at Dewey

**Executor:** Opus

**Files:**
- Modify: `_datafiles/world/dogmud/quests/28-waking_to_gaius.yaml`
- Modify (whole file): `_datafiles/world/dogmud/dialogue/newcomer_antechamber/9491.yaml`
- Modify (whole file): `_datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml`
- Modify (whole file): `_datafiles/world/dogmud/behaviors/rooms/newcomer_antechamber/6467.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/topic-keys.txt`
- Modify: `_datafiles/world/dogmud/quests/loop-backlog.txt`
- Modify: `internal/questengine/quest28_teaching_steps_test.go`
- Modify (whole file): `internal/behaviortree/landing_handoff_test.go`
- Modify: `internal/usercommands/start.go` (comment)

The brand-new character's path this task builds: `startInAntechamber` drops the player in a private copy of 6258 and prints the room; the roster shows `[!] Dewey` (his offer is open). `hooks.QuestRoomEnter` raises `room_enter 6258`: Dewey says what a command is, what `[!]` and a quest are, and ends "Type talk dewey." `talk dewey` shows an opener naming the `[!]` line, the offer "Waking to Gaius", and the marks question `[*]` that explains `[!]`, `[?]` and `[*]`. Accept grants `28-start`; its `quest_granted` trigger has Dewey name `hint`. The lessons run as before. After the flee, the arrival lines point at `[?]`; `talk dewey` offers the hand-in; choosing it grants `28-end`, Dewey speaks the farewell, and his tree narrates, moves the player to 5200 and names Hadwen's `[!]`.

- [ ] **Step 1: Write the failing tests**

In `internal/questengine/quest28_teaching_steps_test.go`, change the import block to:

```go
import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)
```

Replace

```go
	require.Equal(t, 2, guideLines, "both guide-lesson triggers carry the line")
```

with

```go
	require.Equal(t, 1, guideLines, "one guide-lesson trigger carries the line (#375 phase 7: one action per step)")
```

Append to the same file:

```go
// TestQuest28_StartsAndEndsAtDewey (#375 phase 7): no trigger grants 28-start
// or 28-end. Dewey's offer starts the quest, his [!] and the marks question
// greet a brand-new character, and his hand-in, open only after the flee
// lesson, ends it and fires his tree for the send-off.
func TestQuest28_StartsAndEndsAtDewey(t *testing.T) {
	q := loadQuest28(t)
	for i, tr := range q.Triggers {
		for _, a := range tr.Actions {
			require.NotEqual(t, "28-start", a.Grant, "trigger %d grants 28-start: only Dewey's offer may", i)
			require.NotEqual(t, "28-end", a.Grant, "trigger %d grants 28-end: only Dewey's hand-in may", i)
		}
	}

	df := loadShippedDialogue(t, "newcomer_antechamber", "9491.yaml")
	held := map[string]bool{}
	ps := &dialogue.PlayerState{
		QuestOpen: func(int) bool { return true },
		HasQuest:  func(tok string) bool { return held[tok] },
		GiveQuest: func(tok string) { held[tok] = true },
		HasHeard:  func(string) bool { return false },
		MarkHeard: func(string) {},
	}

	offer := df.FindTopic("waking_offer")
	require.NotNil(t, offer, "Dewey's offer of quest 28")
	require.Equal(t, dialogue.KindOffer, offer.Kind)
	require.Equal(t, "28-start", offer.GrantsQuest)
	require.True(t, df.Eligible(offer, ps), "a brand-new character is offered quest 28")
	require.Equal(t, menu.MarkerQuest, df.Marker(ps), "Dewey shows [!] before the quest is taken")
	require.True(t, df.Eligible(df.FindTopic("marks"), ps), "the marks question is in the first menu")
	require.False(t, df.Eligible(df.FindTopic("quest"), ps), "'What should I be doing' waits until the quest is taken")
	text, _ := df.OpenerFor(ps)
	require.Contains(t, text, "[!]", "the first opener names the mark")

	ready := df.FindTopic("ready")
	require.NotNil(t, ready)
	require.Equal(t, dialogue.KindHandIn, ready.Kind)
	require.Equal(t, "28-end", ready.GrantsQuest)
	require.True(t, ready.Behavior, "Dewey's tree plays the send-off")
	require.NotContains(t, squash(ready.Text+" "+ready.Hints), "break away", "#690: no fight jargon in the farewell")
	held["28-start"], held["28-tripped"] = true, true
	require.False(t, df.Eligible(ready, ps), "no hand-in before the flee lesson")
	held["28-fled"] = true
	require.True(t, df.Eligible(ready, ps))
	require.Equal(t, menu.MarkerHandIn, df.Marker(ps), "Dewey shows [?] once the quest can be finished")
}

// TestQuest28_WalksFromTheOfferToTheLanding drives the real quest 28 YAML
// through every step (#375 phase 7): arriving greets a newcomer and grants
// nothing, Dewey's offer starts the quest (here, the quest_granted event its
// grant raises), and each lesson's own event moves it one step, to fled.
// Dewey's hand-in ends it (TestQuest28_StartsAndEndsAtDewey).
func TestQuest28_WalksFromTheOfferToTheLanding(t *testing.T) {
	e, p, ctx := newQuest28Walk(t, "", 6258)
	notify := func(event string, d EventDetails) {
		t.Helper()
		d.UserId = 1
		p.roomId = d.RoomId
		e.Notify(event, d, p, ctx)
	}

	notify("room_enter", EventDetails{RoomId: 6258})
	require.Equal(t, "", p.step(), "arriving grants nothing: only Dewey's offer starts the quest")
	require.Len(t, ctx.npcSays, 1, "Dewey greets the newcomer")
	greet := ctx.npcSays[0].Lines
	require.True(t, strings.HasSuffix(squash(greet[len(greet)-1].Text), "type talk dewey."), // squash lowercases
		"the greeting ends on talk: %q", greet[len(greet)-1].Text)

	p.current = "start" // Dewey's offer granted 28-start
	notify("quest_granted", EventDetails{RoomId: 6258, QuestToken: "28-start"})
	require.Equal(t, "start", p.step())
	require.Len(t, ctx.npcSays, 2, "Dewey names the first command once the quest is taken")
	first := ctx.npcSays[1].Lines
	require.True(t, strings.HasSuffix(squash(first[len(first)-1].Text), "type hint."), "%q", first[len(first)-1].Text)

	walk := []struct {
		event string
		d     EventDetails
		want  string
	}{
		{"command_issued", EventDetails{RoomId: 6258, Command: "hint"}, "hinted"},
		{"command_issued", EventDetails{RoomId: 6258, Command: "look"}, "looked"},
		{"command_issued", EventDetails{RoomId: 6258, Command: "look", Noun: "guide"}, "examined"},
		{"room_enter", EventDetails{RoomId: 6259}, "status_prompt"},
		{"command_issued", EventDetails{RoomId: 6259, Command: "status"}, "statused"},
		{"command_issued", EventDetails{RoomId: 6259, Command: "skills"}, "skilled"},
		{"command_issued", EventDetails{RoomId: 6259, Command: "spells"}, "spelled"},
		{"command", EventDetails{RoomId: 6259, Command: "cast", Noun: "chrysalis-glow"}, "glowed"},
		{"room_enter", EventDetails{RoomId: 6260}, "carry_prompt"},
		{"command_issued", EventDetails{RoomId: 6260, Command: "get", Noun: "token"}, "got"},
		{"command_issued", EventDetails{RoomId: 6260, Command: "inventory"}, "invd"},
		{"command_issued", EventDetails{RoomId: 6260, Command: "help"}, "helped"},
		{"room_enter", EventDetails{RoomId: 6261}, "speaks_prompt"},
		{"command", EventDetails{RoomId: 6261, Command: "say"}, "said"},
		{"topic_heard", EventDetails{RoomId: 6261, MobId: 9491, Topic: "asked_28"}, "asked"},
		{"room_enter", EventDetails{RoomId: 6262}, "proving_prompt"},
		{"command", EventDetails{RoomId: 6262, Command: "warcry"}, "warcried"},
		{"command_issued", EventDetails{RoomId: 6262, Command: "conditions"}, "saw_conditions"},
		{"command_issued", EventDetails{RoomId: 6262, Command: "cooldowns"}, "checked_cooldowns"},
		{"command_issued", EventDetails{RoomId: 6262, Command: "consider", Noun: "effigy"}, "considered"},
		{"command_issued", EventDetails{RoomId: 6262, Command: "attack", Noun: "effigy"}, "attacked"},
		{"command", EventDetails{RoomId: 6262, Command: "cast", Noun: "conviction-spike"}, "cast_spike"},
		{"command", EventDetails{RoomId: 6262, Command: "trip"}, "tripped"},
		{"command", EventDetails{RoomId: 6467, Command: "flee"}, "fled"},
	}
	for _, s := range walk {
		notify(s.event, s.d)
		require.Equal(t, s.want, p.step(), "after %s %s %s", s.event, s.d.Command, s.d.Noun)
	}
}
```

Replace the whole of `internal/behaviortree/landing_handoff_test.go` with:

```go
package behaviortree

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The Landing (room 6467) since #375 phase 7: quest 28 ends at Dewey's
// hand-in topic, so the room no longer takes `talk dewey` itself. A talk,
// say or ask goes on to the command (and Dewey's talk menu); a walk is still
// refused while the quest is open, since the room has no exits. A player who
// already holds 28-end but is still here (the send-off's move did not
// happen) is sent on by any talk or walk.
func TestLandingLeavesTalkToDewey(t *testing.T) {
	m, room := sightScene(t, "city")
	m.MobId = 9491
	m.Character.Name = "Dewey"
	room.AddMob(m.InstanceId)

	u := users.NewTestUser(8141, "verenna", "Verenna", 98141)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8141: u}))
	room.AddPlayer(8141)

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "rooms", "newcomer_antechamber", "6467.yaml")
	require.NoError(t, GetEngine().LoadRoomTree(room.RoomId, tree))
	t.Cleanup(func() { clearEngineRoomState(t, room.RoomId); clearRoomStates(t, room.RoomId) })

	try := func(cmd, rest string) bool {
		clearRoomStates(t, room.RoomId)
		return TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8141, RoomId: room.RoomId, Command: cmd, Rest: rest,
		})
	}

	u.Character.QuestProgress = map[int]string{28: "fled"}
	for _, c := range []struct{ cmd, rest string }{
		{"talk", "dewey"}, {"talk", "to dewey"}, {"say", "hello"}, {"ask", "dewey quest"},
	} {
		require.False(t, try(c.cmd, c.rest), "%s %q must reach the command, which opens Dewey's talk menu", c.cmd, c.rest)
	}
	require.True(t, try("north", ""), "a walk is refused while quest 28 is open")

	u.Character.QuestProgress = map[int]string{28: "end"}
	require.True(t, try("talk", "dewey"), "a player who finished but is still here is sent on")
	DrainAllDelayedActionsForTest(t)
}

// Dewey's send-off (#375 phase 7): choosing the `ready` hand-in fires his
// tree, which moves the player to the Awakening Pool (5200). The hand-in
// grants 28-end itself, so the tree grants nothing. A second choice in the
// moment before 28-end lands does not replay it, and a player who left
// before the move is not moved.
func TestDeweySendOffMovesThePlayerToThePool(t *testing.T) {
	const (
		landing   = 6467
		pool      = 5200
		elsewhere = 6262
		deweyInst = 94910
	)
	run := func(t *testing.T, leave bool) int {
		DrainAllDelayedActionsForTest(t)
		events.DrainQueuedQuestsForTest()
		t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
			landing:   {RoomId: landing, Zone: "TestZone", Title: "The Landing", Exits: map[string]exit.RoomExit{}},
			pool:      {RoomId: pool, Zone: "TestZone", Title: "The Awakening Pool", Exits: map[string]exit.RoomExit{}},
			elsewhere: {RoomId: elsewhere, Zone: "TestZone", Title: "The Proving", Exits: map[string]exit.RoomExit{}},
		}, map[string]*rooms.ZoneConfig{}))
		t.Cleanup(seedTestMob(t, 9491, deweyInst, landing, "Dewey"))
		t.Cleanup(seedTestUser(t, 1, "verenna", "Verenna", landing))
		u := requireUser(t, 1)

		tree, err := LoadTreeFromFile("../../_datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml")
		require.NoError(t, err)
		state := NewBehaviorState()
		ctx := func() *EvalContext {
			return &EvalContext{
				Event:    EventContext{EventType: "player_topic", UserId: 1, Text: "ready", RoomId: landing},
				MobState: state, MobId: 9491, InstanceId: deweyInst, RoomId: landing, MobName: "Dewey",
			}
		}
		require.Equal(t, Success, tree.Evaluate(ctx()), "the ready topic must reach the send-off branch")
		require.NotEqual(t, Success, tree.Evaluate(ctx()), "a second choice must not replay the send-off")
		if leave {
			u.Character.RoomId = elsewhere
		}
		DrainAllDelayedActionsForTest(t)
		require.Empty(t, questGrantsFor(1), "the hand-in grants 28-end; the tree grants nothing")
		return u.Character.RoomId
	}

	t.Run("stayed", func(t *testing.T) {
		require.Equal(t, pool, run(t, false), "the player is moved to the Awakening Pool")
	})
	t.Run("left", func(t *testing.T) {
		require.Equal(t, elsewhere, run(t, true), "a player who left is not moved")
	})
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest28_'
go test ./internal/behaviortree/ -run 'TestLandingLeavesTalkToDewey|TestDeweySendOffMovesThePlayerToThePool'
```

Expected: `TestQuest28_StartsAndEndsAtDewey` fails (`trigger 0 grants 28-start`), `TestQuest28_WalksFromTheOfferToTheLanding` fails (`arriving grants nothing`), `TestQuest28_LookLessonRewardsACloseLook` fails (`expected: 1 actual: 2`); `TestLandingLeavesTalkToDewey` fails (`talk "dewey" must reach the command`); `TestDeweySendOffMovesThePlayerToThePool` fails (`the ready topic must reach the send-off branch`).

- [ ] **Step 3: Edit quest 28**

In `_datafiles/world/dogmud/quests/28-waking_to_gaius.yaml`:

(a) Replace the `fled` step

```yaml
  - id: fled
    # destination inferred from triggers (#375 p4); was: no single resolved destination; Phase 2 review
    description: "You broke away. Running is how you live to win the next one."
    hint: "Dewey is waiting at the Landing for one last word. Type talk
      dewey."
```

with

```yaml
  - id: fled
    # Dewey takes the hand-in (#375 phase 7). He stands in every antechamber
    # room, and the map follows the copy the player can see.
    destination:
      mob: 9491
    description: "You broke away. Running is how you live to win the next one."
    hint: "Dewey is waiting at the Landing to send you on. Type talk dewey,
      then choose Waking to Gaius."
```

(b) Replace the `cast_spike` step hint

```yaml
    hint: "A trip knocks your foe to the ground. Once your spike has rested,
      type trip. For more, type help trip."
```

with

```yaml
    hint: "A trip knocks your foe to the ground. Once your spike has rested,
      type trip, even if the effigy is down already. For more, type help
      trip."
```

(c) Replace the rewards block

```yaml
rewards:
  actor: >-
    Dewey's voice stays with you a moment after the grey lets go: steady,
    warm, already a little proud. You are awake now, and the world is real.
```

with (the banner now lands at the hand-in, before the move)

```yaml
rewards:
  actor: >-
    Dewey's hand is steady on your shoulder, and his voice is warm, already
    a little proud. You are awake now, and the world is real.
```

(d) Replace the first trigger, lines 170-190 on master (from `triggers:` and `  # ── 6258 The Threshold: greet, hint, look, look guide, north ──` through Dewey's fourth line ending `step, any time you are not sure. Type hint."}`), with:

```yaml
triggers:
  # ── 6258 The Threshold: greet, hint, look, look guide, north ──
  # The greeting grants nothing (#375 phase 7): quest 28 starts at Dewey's
  # offer topic waking_offer (dialogue/newcomer_antechamber/9491.yaml). A
  # newcomer has never played a MUD (R24, owner 2026-10-21), so Dewey says
  # what a command and a quest are, and teaches talk, before the first menu.
  # HandleJoin raises room_enter on every login too; the missing gate keeps
  # the greeting to a player who has not taken the quest.
  - event: room_enter
    room: 6258
    conditions:
      missing: [28-start, 28-end]
    actions:
      - npc_say:
          mob: 9491
          lines:
            - {delay: 1, text: "There you are, waking up at last. Easy, now.
                Nothing here can harm you. I am Dewey, your guide."}
            - {delay: 2, text: "You act here by typing. The first word you type
                is the command. Type it, press Enter, and the world answers."}
            - {delay: 2, text: "See the [!] in front of my name? It means I
                have a quest for you. A quest is a task, done one step at a
                time."}
            - {delay: 2, text: "Talk starts a conversation with someone, and
                you choose what to say from a list. Type talk dewey."}

  # Accepting Dewey's offer grants 28-start, which raises quest_granted, and
  # Dewey names the first command. It grants nothing; the missing gate keeps
  # it to a fresh start, so a resume past the first step stays quiet.
  - event: quest_granted
    quest_token: 28-start
    conditions:
      missing: [28-hinted]
    actions:
      - npc_say:
          mob: 9491
          lines:
            - {delay: 1, text: "Good. Your quest has begun. Your first command
                is hint. It names your next step, any time you are not sure.
                Type hint."}
```

(e) Delete the shortcut `look guide` trigger and its comments. Replace this block

```yaml
  # A player who types look guide before a bare look has done both lessons.
  # First, so the bare-look trigger below (which also matches look guide)
  # does not say "Type look guide" in the same command (#495).
  #
  # The room-noun lesson (owner ruling 2026-10-17): "guide" is a room noun
  # of 6258 and its description names it, so it shows highlighted.
  - event: command_issued
    command: look
    noun: guide
    conditions:
      has: [28-hinted]
      missing: [28-looked]
    actions:
      - grant: 28-looked
      - grant: 28-examined
      - npc_say:
          mob: 9491
          lines:
            - {delay: 0, text: "That's the whole trick of this world. The
                ones who read what is in front of them live longest. Any
                word shown in a different color is worth a look, and a close
                look often shows you more than the room did. Try the stone
                under your feet. Type look stone."}

  - event: command_issued
```

with

```yaml
  # The room-noun lesson (owner ruling 2026-10-17): "guide" is a room noun
  # of 6258 and its description names it, so it shows highlighted. One
  # action per step (#375 phase 7): a player who types look guide before a
  # bare look is taught look first by the bare-look trigger below (which
  # matches look guide too), then looks at the guide again. This trigger
  # comes first, so the bare look's grant cannot fire it in the same
  # command (#495).
  - event: command_issued
```

(f) In the `say` trigger, replace

```yaml
            - {delay: 0, text: "Ha, and the world will answer, out there. Talk
                starts a conversation with one person. A numbered list of
                questions comes up, and you type a number to ask one. Type
                talk dewey."}
```

with

```yaml
            - {delay: 0, text: "Ha, and the world will answer, out there. You
                know talk already. This time, ask me a question. A numbered
                list comes up, and you type a number to ask one. Type talk
                dewey."}
```

(g) #690 "Type trip" when the effigy is already down. In the cast trigger replace

```yaml
            - {delay: 9, whisper: true, text: "Rested. Now knock the effigy
                down. A trip sweeps a foe off its feet. Type trip."}
```

with

```yaml
            - {delay: 9, whisper: true, text: "Rested. Now your body. A trip
                sweeps a foe off its feet. If the effigy is down already, trip
                it anyway, and it counts. Type trip."}
```

and in the trip trigger replace

```yaml
            - {delay: 1, whisper: true, text: "That is the body's way. When a
                trip lands, the foe goes down wearing a condition of its
                own. Voice, belief, and body: all yours."}
```

with

```yaml
            - {delay: 1, whisper: true, text: "That is the body's way. A foe on
                the ground wears a condition of its own until it stands.
                Voice, belief, and body: all yours."}
```

(h) Replace the closing comment

```yaml
  # ── 6467 The Landing: the `talk dewey` handoff (farewell + teleport to the
  #    Awakening Pool + grant of quest 30 + completion of 28-end) is handled by
  #    the room's behavior tree, which INTERCEPTS the talk command so Dewey's
  #    generic dialogue hint does not also fire. See
  #    behaviors/rooms/newcomer_antechamber/6467.yaml.
```

with

```yaml
  # ── 6467 The Landing: quest 28 ends at Dewey's hand-in topic `ready`
  #    (dialogue/newcomer_antechamber/9491.yaml), which grants 28-end. His
  #    tree (behaviors/newcomer_antechamber/9491-dewey.yaml) plays the
  #    farewell and moves the player to the Awakening Pool, where Cleric
  #    Hadwen offers quest 30. The room's tree
  #    (behaviors/rooms/newcomer_antechamber/6467.yaml) prompts, blocks a
  #    walk and grants nothing (#375 phase 7).
```

- [ ] **Step 4: Replace Dewey's dialogue file**

Write `_datafiles/world/dogmud/dialogue/newcomer_antechamber/9491.yaml` with exactly:

```yaml
mobid: 9491
zone: Newcomer Antechamber
defaultMood: friendly
opener:
  # Promises only what every player can ask from the first room on: the
  # prompt and fight questions open after the talk lesson (28-asked).
  text: Ask me anything you like. When you reach the landing at the end, talk to
    me there and I'll send you through to the world.
  hints: Each line below is a question. Type the number of a question, then
    press Enter. Type a number again any time to hear more.
  variants:
  # The first talk, before quest 28 is taken (#375 phase 7): Dewey's offer is
  # the first [!] a newcomer meets, so the opener says what it is and how to
  # take it. The marks question explains every mark.
  - questExcluded:
    - 28-start
    - 28-end
    text: There you are. I'm Dewey, and I'll be your guide here. I have a quest for
      you, the first of many. See the [!] in front of it in the list below? That
      mark means a quest.
    hints: Each line below is a numbered choice. Type the number of the line
      marked [!], then press Enter. Then choose Accept.
  # The hand-in at the landing (#375 phase 7): after the flee lesson the [?]
  # line finishes quest 28, and Dewey sends the player up to the pool.
  - questRequired:
    - 28-fled
    questExcluded:
    - 28-end
    text: You made it across, and now you know how to run. One thing is left. Tell
      me you're ready, and I'll send you up into the world.
    hints: The line marked [?] finishes your quest. Type its number, then press
      Enter.
  # Quest 28's talk lesson in room 6261: the player has said hello and now
  # chooses a question from Dewey's menu (R20). Its world question is the
  # lesson's [*], so the opener says what that mark means (owner 2026-10-21,
  # R24).
  - questRequired:
    - 28-said
    questExcluded:
    - 28-asked
    text: Ask me something. Anything at all, but the world is a fine place to start.
      See the [*] beside it? That mark means I think you should hear it.
    hints: Each line below is a question. Type the number of the question about
      the world, then press Enter.
  # Opener variants carry gates only, not heardRequired, so this cannot tell
  # which lessons are heard. It points at the [*] questions; the way north
  # comes from quest 28's `asked` step hint and from Dewey's mob_idle line
  # in room 6261 (behaviors/newcomer_antechamber/9491-dewey.yaml). Topic
  # hints cannot say north: these topics stay open in 6262 and 6467, where
  # north is refused.
  - questRequired:
    - 28-asked
    questExcluded:
    - 28-proving_prompt
    text: You asked, and you listened. Half of what matters out there is learned
      that way. Hear the questions marked [*] before you go. After that, the way
      north is open.
    hints: The questions marked [*] are worth hearing before you go on. Type the
      number of one.
topics:
# Quest 28 starts here (#375 phase 7): the room_enter greeting on 6258 grants
# nothing, so this offer is the quest's only start.
- key: waking_offer
  kind: offer
  quest: 28
  label: Waking to Gaius
  playerLine: Will you show me how to live here?
  text: Gladly. That's what I'm here for. Your quest is the waking itself, a handful
    of small lessons, one at a time, at your own pace. I'll be beside you in every
    room.
  hints: Choose Accept to take the quest.
  questExcluded:
  - 28-start
  - 28-end
  grantsQuest: 28-start
# The lesson topic (R20, quest 28 trigger folded in): hearing it grants
# 28-asked, through quest 28's topic_heard trigger. The world topic gated on
# 28-asked that answered the same question is folded into this text. Its hint
# steers the player to the lessons it opens (prompt, command, fight, and
# marks if still unheard), all marked [*], before the walk north (review M1,
# 2026-10-21): one command, the number.
- key: asked_28
  kind: info
  label: What is the world of Gaius like?
  text: 'Gaius. A whole living world: a city, roads, people who lie and help and
    everything between. It changes while you''re not looking. That''s how you''ll
    learn half of what matters out there, by asking. So ask me more. The new
    questions marked [*] are worth hearing before you go north.'
  hints: New questions are marked [*]. Hear them before you go on. Type the
    number of one.
  marked: true
  questRequired:
  - 28-said
  questExcluded:
  - 28-asked
  - 28-end
# Hidden from the lesson on, so the lesson's question is the only world
# question in the menu when it is taught.
- key: world_2
  kind: info
  label: What is Gaius like?
  text: Gaius. A whole living world. A city, roads, people who lie and help and
    everything between. It changes while you're not looking. You'll love it and it'll
    scare you, both. That's right and proper.
  questExcluded:
  - 28-said
# The newcomer lessons (R24, owner 2026-10-21): each one is a question about
# something on the player's screen, marked [*] once, and opened after the talk
# lesson. Facts are cited in the Task 15 batch 1 report (prompt: config.yaml
# TextFormats.Prompt and FightPrompt, users/userrecord.prompt.go
# renderVitalBar; reservation: characters/validate.go GetPoolReservation and
# help/reservation.template).
- key: prompt
  kind: info
  label: What is the HP, SP, CP line on my screen?
  text: That line is your prompt. The game prints it again after everything you
    do, so you always know how you stand. HP is your health. If it runs out, you
    die. SP is your stamina. It pays for swings, dodges and other moves of the
    body. CP is your conviction. It pays for spells and shouts. Those three are
    your pools. In each bar, the lit blocks are what you have left and the dark
    blocks are what you have spent. All three fill again with time and rest. The
    sun or moon at the front tells you if it is day or night. While you fight,
    the line grows longer. It also shows your foe and how hurt it is.
  hints: To read your pools in words, type status. For more about this line, type
    help set-prompt.
  marked: true
  questRequired:
  - 28-asked
# Review M3 (2026-10-21): spent blocks are the dark ░ right after the lit
# ones (fg 238); a reserved band is ▒ in a lighter grey (fg 244) at the very
# end, drawn only when something reserves (userrecord.prompt.go
# renderVitalBar). A newcomer has nothing reserved, so the answer says so
# first and keeps spent and held apart.
- key: reserve
  kind: info
  label: Why would a bar not fill all the way?
  text: You have none of this yet, so your bars always fill. The dark blocks after
    your lit ones are what you have spent, and rest fills them again. Later, some
    things you keep will hold part of a pool back for as long as you keep them. A
    piece of enchanted gear does it, and so does a companion who walks with you.
    That is called reservation. The held part shows as a lighter grey band at the
    very end of the bar, and rest does not fill it, because you never spent it.
    Take the gear off or send the companion away, and it comes straight back.
  hints: To see how much of each pool is held, type status. For more, type help
    reservation.
  heardRequired:
  - prompt
- key: command
  kind: info
  label: What is a command?
  text: Everything you do here, you do by typing. A command is a word you type to
    act, and then you press Enter. Some commands work alone. Look is one. Many
    take a thing to act on, the way a sentence has a verb and an object. Look guide
    looks at the guide. Get token picks up the token. You have typed several
    already, and every one of them was a command.
  hints: To see every command the game knows, type help.
  marked: true
  questRequired:
  - 28-asked
- key: help
  kind: info
  label: What if I forget a command?
  text: Forget a command and the game remembers for you. Type <ansi fg="command">help</ansi>
    for the whole list, or <ansi fg="command">help</ansi> and a topic for one thing,
    like help look. No shame in it. I still look things up.
  heardRequired:
  - command
# The markers are taught here first (#375 phase 7; requirements page,
# "Players are taught the system"): open from the very first menu, where
# Dewey's offer is the newcomer's first [!].
- key: marks
  kind: info
  label: What do the marks next to names mean?
  text: A mark next to a name means that person has something for you. A [!] means
    they have a quest to offer. A quest is a task, done one step at a time. Talk
    to them, choose it from the list, then choose Accept. A [?] means you can
    finish a quest with them now. A [*] means they have something worth hearing.
    The mark goes away once you take the quest, finish it, or hear what they had
    to say.
  hints: To start a conversation with someone, type talk and their name.
  marked: true
# Review M2 (2026-10-21): `stand` is the command flee's prone refusal asks
# for (usercommands/flee.go FleeRefuseProne); death sends a new character to
# the Mending Hut (characters/respawn_home.go, HomeLocations "default" 5209)
# with every pool nearly empty (help death).
- key: fight
  kind: info
  label: How do I fight?
  text: First, size them up. Type consider and their name, and your instincts tell
    you how a fight would go. If the fight looks fair, type attack and their name.
    From then on you keep swinging on your own, round after round, until one of
    you dies or runs. While you swing, you can add a spell, like cast spike, or a
    trip. If it goes badly, type flee to break away and run. You cannot run from
    the ground, so if you are knocked down, type stand first. If something holds
    you, you cannot run until it lets go. Running isn't losing. If you die, you
    wake in a safe place, the Mending Hut in Pothole Coulee, the first town you
    will see. You wake tired, with every pool nearly empty, so rest before you go
    back out.
  hints: For more about fighting, type help combat.
  marked: true
  questRequired:
  - 28-asked
- key: look
  kind: info
  label: How do I look at things?
  text: Looking is the first skill, and you will use it every day here. Type <ansi
    fg="command">look</ansi> in any room. Type <ansi fg="command">look</ansi> and
    a thing's name to look at that thing closely. This world hides a lot in its
    rooms, so look at anything that catches your eye.
- key: condition
  kind: info
  label: What are conditions?
  text: Conditions are effects on you that last a while. A war cry steadying your
    hand, a poison in your blood, a leg swept out from under you. Some are good
    and some are bad. Type <ansi fg="command">conditions</ansi> to see yours.
- key: grow
  kind: info
  label: How will I grow stronger?
  text: You grow by doing. Swing a blade and you sharpen. Cast and you deepen. Shout
    and you steady. Over time the way you fight even draws changes into your body,
    and those are called mutations. You never pick what you become. You earn it.
# Opens once the quest is taken (#375 phase 7): before that, "you are on a
# quest" would not be true.
- key: quest
  kind: info
  label: What should I be doing right now?
  text: You are on a quest right now. A quest is a task, done one step at a time,
    and yours is the waking itself. Type <ansi fg="command">hint</ansi> any time
    and it'll name your next step. And when you reach the landing at the end, type
    <ansi fg="command">talk dewey</ansi> and I'll send you through to the world.
  questRequired:
  - 28-start
# Quest 28 ends here (#375 phase 7): the hand-in grants 28-end, and its tree
# branch (behaviors/newcomer_antechamber/9491-dewey.yaml, topic_is ready)
# plays the farewell, moves the player to the Awakening Pool (5200) and names
# Cleric Hadwen's [!] there. Open only after the flee lesson, so its words
# never run ahead of what the player has done (#690). The key stays `ready`,
# so a player who heard the old info line keeps it heard.
- key: ready
  kind: handin
  quest: 28
  label: Waking to Gaius
  playerLine: I'm ready to go out into the world.
  text: That's the last of it. Steady, now. This part feels strange the first time,
    but it's gentle. I've got you.
  behavior: true
  questRequired:
  - 28-fled
  questExcluded:
  - 28-end
  grantsQuest: 28-end
```

- [ ] **Step 5: Replace Dewey's tree**

Write `_datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml` with exactly:

```yaml
# Dewey (9491) per-mob behavior tree.
#
# It overrides the noncombat_passive archetype's player_enter reaction. That
# archetype emotes "pays you little mind." on every player entry, which
# flatly contradicts Dewey's warm-mentor persona and fires right on top of his
# arrival lines (2026-07-17 playtest finding). Dewey's teaching lines live in
# quest 28 (npc_say) and the room trees (behaviors/rooms/newcomer_antechamber/).
#
# Composition (2026-08-15): a per-mob tree only owns an event it explicitly
# handles a matching branch for. So this file claims player_enter itself with
# a silent always-Success branch (set_state is the established no-op action)
# rather than simply omitting the event, or the archetype's dismissive emote
# falls straight through.
#
# The send-off (#375 phase 7): quest 28 ends at Dewey's hand-in topic `ready`
# (dialogue/newcomer_antechamber/9491.yaml, behavior: true), which grants
# 28-end and fires this tree as player_topic. The branch plays the farewell,
# moves the player to the Awakening Pool (5200) and names Cleric Hadwen's
# [!] there, since a move_player fires no player_enter and Hadwen's greeting
# would not play. A room tree never hears a topic (player_topic is a mob
# event, internal/behaviortree/events.go), so this cannot live in The
# Landing's tree. The per-player `leaving` flag stops a second choice in the
# moment before 28-end lands from replaying it. The antechamber is a solo
# instance the player never re-enters, so the flag is never cleared. If the
# player is gone when the move fires, move_player drops the rest of the run,
# and The Landing's tree sends them on at their next talk or walk.
#
# The mob_idle flee re-nag: quest 28's flee prompt is one delayed line that
# can scroll away under combat text (Malia, 2026-08-02), and a player who
# misses it grinds the unkillable effigy forever. While a player mid-lesson
# (has 28-tripped, missing 28-fled) shares Dewey's room, he repeats the prompt
# every few rounds. ANDing the in-room has/missing conditions is safe here
# ONLY because the antechamber is a solo ephemeral instance.
#
# The mob_idle way-north line (#375 phase 1, R24): in The World Speaks (6261),
# once the talk lesson is done (has 28-asked) and before the player walks on
# (missing 28-proving_prompt, granted on entering 6262), Dewey says to type
# north every few rounds. His lesson topics cannot carry it: their hints are
# static and the topics stay open in 6262 and 6467, where north is refused.
# This pair of tokens holds only while the player stands in 6261.

tree:
  type: selector
  children:
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topic: ready
        - type: condition
          check: state_equals
          key: leaving
          per_player: true
          value: ""
        - type: action
          do: set_state
          key: leaving
          per_player: true
          value: "1"
        - type: action
          do: send_user_text
          delay: 1.5
          text: >-
            Dewey rests a hand on your shoulder, and the grey begins to thin
            around you, unspooling like thread pulled loose, brightening from
            the edges inward.
        - type: action
          do: send_user_text
          delay: 3.0
          text: >-
            The pale blue-green light rises up to meet you. For a breath there
            is only warmth and a weightless sense of falling upward into it.
            Then the world takes shape around you, solid and real and loud
            with life.
        - type: action
          do: move_player
          room_id: 5200
          delay: 4.0
        - type: action
          do: send_user_text
          delay: 5.5
          text: >-
            Cleric Hadwen keeps this pool. The [!] beside his name means he
            has a quest for you. Type <ansi fg="command">talk hadwen</ansi>
            to begin.
    - type: sequence
      event: mob_idle
      children:
        - type: condition
          check: player_in_room_has_quest
          quest: "28-tripped"
        - type: condition
          check: player_in_room_missing_quest
          quest: "28-fled"
        # delay(3) holds the first nag until after quest 28's own delayed
        # flee line (8s post-trip) so Dewey does not talk over himself;
        # cooldown(6) paces the repeats.
        - type: decorator
          mod: delay
          rounds: 3
          child:
            type: decorator
            mod: cooldown
            rounds: 6
            child:
              type: action
              do: say
              text: >-
                No shame in stopping. Straw does not tire, and you cannot
                break it. Type flee to break away and live.
    - type: sequence
      event: mob_idle
      children:
        - type: condition
          check: player_in_room_has_quest
          quest: "28-asked"
        - type: condition
          check: player_in_room_missing_quest
          quest: "28-proving_prompt"
        # delay(5) leaves the player about 20 seconds with the world answer
        # and the lesson menu first; cooldown(8) paces the repeats at about
        # 32 seconds (Timing.RoundSeconds 4).
        - type: decorator
          mod: delay
          rounds: 5
          child:
            type: decorator
            mod: cooldown
            rounds: 8
            child:
              type: action
              do: say
              text: >-
                Ask me all you like. When you are ready to go on, type north.
    - type: action
      event: mob_idle
      do: try_goal_planner

    # Silent player_enter no-op: claims the event so composition never
    # falls through to noncombat_passive's "pays you little mind." emote.
    - type: sequence
      event: player_enter
      children:
        - type: action
          do: set_state
          key: dewey_player_enter_suppressed
          value: "true"
```

- [ ] **Step 6: Replace The Landing's tree**

Write `_datafiles/world/dogmud/behaviors/rooms/newcomer_antechamber/6467.yaml` with exactly:

```yaml
# 6467 The Landing: the end of the tutorial. Since #375 phase 7 quest 28 ends
# at Dewey's hand-in topic (`ready`, dialogue/newcomer_antechamber/9491.yaml),
# and Dewey's own tree (behaviors/newcomer_antechamber/9491-dewey.yaml) plays
# the farewell and moves the player to the Awakening Pool. This tree no longer
# intercepts talk and grants nothing. Its jobs:
#   1. Prompt on ARRIVAL. The player reaches this room by fleeing, so a
#      room_enter fires here (NewRound_DoCombat_helpers.go fires entry hooks
#      for a flee, not just a walk). Without it the room is silent.
#   2. Re-prompt if they sit here a while: the first prompt is easy to lose
#      in combat scrollback.
#   3. Block a plain walk (the room has no exits) and point at the hand-in.
#   4. Rescue: a player who holds 28-end but is still here (the send-off's
#      move did not happen, a logout mid-way) is sent to the pool by any talk
#      or walk.
tree:
  type: selector
  children:

    # ── rescue: finished, but still here ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [talk, say, ask, north, n, go, south, s, east, e, west, w, up, u, down, d]
        - type: condition
          check: player_has_quest
          quest: "28-end"
        - type: action
          do: send_user_text
          text: >-
            The grey thins around you, and the pale light rises to meet you.
        - type: action
          do: move_player
          room_id: 5200
          delay: 0.5
        - type: action
          do: intercept

    # ── block a plain walk; point at the hand-in ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [north, n, go, south, s, east, e, west, w, up, u, down, d]
        - type: condition
          check: player_missing_quest
          quest: "28-end"
        - type: action
          do: mob_say
          mob_id: 9491
          text: >-
            One step left, and it is not a walk. Type
            <ansi fg="command">talk dewey</ansi> and choose the line
            marked [?].
        - type: action
          do: intercept

    # ── arrival: name the command, because nothing else will ──
    # Fires on the flee that brings the player here. The delays let the room
    # description and the arrival text land first.
    - type: sequence
      event: room_enter
      children:
        - type: condition
          check: player_missing_quest
          quest: "28-end"
        # Reset the nudge timer so the re-prompt is measured from arrival.
        - type: action
          do: set_state
          key: nudge_rounds
          value: 0
        - type: action
          do: mob_say
          mob_id: 9491
          delay: 1.5
          text: >-
            There you are, safe across. Running worked, and now you know it
            for yourself.
        - type: action
          do: mob_say
          mob_id: 9491
          delay: 4.0
          text: >-
            One thing left. The [?] beside my name means you can finish your
            quest with me. Type <ansi fg="command">talk dewey</ansi> and
            choose it.

    # ── re-prompt: the first line is easy to lose in the combat text ──
    # room_idle fires once per round (4s) while a player is present. The counter
    # increments every round and the say fires on the 7th, so roughly every 28
    # seconds, measured from arrival.
    - type: sequence
      event: room_idle
      children:
        - type: condition
          check: player_in_room_missing_quest
          quest: "28-end"
        - type: action
          do: increment_state
          key: nudge_rounds
        - type: condition
          check: state_greater_than
          key: nudge_rounds
          value: 6
        - type: action
          do: set_state
          key: nudge_rounds
          value: 0
        - type: action
          do: mob_say
          mob_id: 9491
          text: >-
            Take your time. When you are ready, type
            <ansi fg="command">talk dewey</ansi> and choose the line
            marked [?].
```

- [ ] **Step 7: Manifest, backlog and the start.go comment**

In `_datafiles/world/dogmud/dialogue/topic-keys.txt`, insert `9491.waking_offer` on its own line between `9491.reserve` and `9491.world_2`.

In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete exactly these 8 lines (the last one clears because 6467's tree no longer grants `30-start`):

```
end 28 room 6467 tree grant_quest 28-end
end 28 step end
one-action 28 quest 28 trigger command_issued command=look noun=guide grants=28-examined,28-looked:double
one-action 28 quest 28 trigger command_issued command=look noun=guide grants=28-examined,28-looked:prev
start 28 quest 28 trigger room_enter room=6258 grants=28-start
start 28 step start
walk-destination 28 step fled
start 30 room 6467 tree grant_quest 30-start
```

In `internal/usercommands/start.go` replace

```go
	// Quest 28's 28-start and Dewey's opening line come from its room_enter
	// trigger, raised by hooks.QuestRoomEnter for this move once this command
	// returns, so his line lands beneath the arrival look.
```

with

```go
	// Dewey's greeting (ending on `talk dewey`) comes from quest 28's
	// room_enter trigger, raised by hooks.QuestRoomEnter for this move once
	// this command returns, so his lines land beneath the arrival look. The
	// quest itself starts at Dewey's offer (#375 phase 7).
```

- [ ] **Step 8: Run and pass**

```bash
gofmt -l internal/
go build ./...
go test ./internal/questengine/ ./internal/behaviortree/ ./internal/guards/ ./internal/usercommands/
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_ServerBootsCleanWithRealData' -timeout 600s .
```

Expected: `gofmt` prints nothing; every package `ok` (this includes `TestQuest28_NpcLinesFinishWithinAFewSeconds`: greeting 7 s, quest_granted 1 s; `TestQuest28_CooldownStepsWaitOutTheSharedCooldown`: unchanged delays; `TestAntechamberBtreeYAMLsLoad`: the two rewritten trees parse); the root sweep `ok` with no "not in loop-backlog.txt" and no "matches no finding". If the sweep reports a fresh `walk-*` finding for quest 28 (quest 28 is walked for the first time now that it has an offer), fix the content it names; never add a backlog line.

Null probes: (1) temporarily delete `behavior: true` from `ready` in 9491.yaml, rerun `go test ./internal/questengine/ -run TestQuest28_StartsAndEndsAtDewey`, see `Dewey's tree plays the send-off` fail, restore. (2) temporarily change Dewey's tree `topic: ready` to `topic: readyx`, rerun `go test ./internal/behaviortree/ -run TestDeweySendOff`, see it fail, restore.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/quests/28-waking_to_gaius.yaml _datafiles/world/dogmud/dialogue/newcomer_antechamber/9491.yaml _datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml _datafiles/world/dogmud/behaviors/rooms/newcomer_antechamber/6467.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/quest28_teaching_steps_test.go internal/behaviortree/landing_handoff_test.go internal/usercommands/start.go
git commit -F - <<'EOF'
content(quest 28): Dewey offers and takes Waking to Gaius

The 6258 greeting grants nothing and teaches talk and [!]; Dewey's new
offer is the only start, and his `ready` topic is the hand-in, open only
after the flee lesson (#690 jargon gone). His tree plays the farewell
and moves the player to the pool; The Landing's tree stops intercepting
talk and grants nothing. The look-guide shortcut that granted two steps
goes, the fled step points at Dewey, and the trip prompt reads right
when the effigy is already down (#690). Clears 8 backlog lines.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 4: Quest 30 ends at a Hadwen hand-in

**Executor:** Opus

**Files:**
- Modify (whole file): `_datafiles/world/dogmud/quests/30-the_awakening.yaml`
- Modify: `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml`
- Modify (whole file): `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml`
- Modify (whole file): `_datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5200.yaml`
- Modify: `_datafiles/world/dogmud/quests/step-ids.txt`
- Modify: `_datafiles/world/dogmud/dialogue/topic-keys.txt`
- Modify: `_datafiles/world/dogmud/quests/loop-backlog.txt`
- Modify: `internal/behaviortree/hadwen_rite_guard_test.go`
- Modify: `internal/behaviortree/hadwen_midrite_test.go`
- Modify: `internal/dialogue/hadwen_midrite_test.go`
- Modify: `internal/dialogue/velk_opener_test.go`
- Modify: `main_quests_offer_test.go`
- Modify: `main_quests.go`, `internal/usercommands/start.go`, `internal/behaviortree/actions_quest.go` (comments)

Readers of `30-end` and `30-rite` (49 non-save files reference `30-end`). `30-end` keeps its meaning, "Opened", so every gate on it stays: the coulee NPC openers and topics, Esk's `passage` and tree, the arch room 5215, `characters.MigrateNewbieAwakening` and `usercommands.autoAwaken` (both set `end`, still the final step). Only these change, because they mean "the rite has not finished" and must not fire for a player at `30-woken`: Hadwen's rite guard (`player_missing_quest 30-end`, tree line 71), his greeting and ambient lines (277-278, 298-299), his mid-rite opener and `rite_wait` (`questExcluded 30-end`), and the pool's movement gate text. A player at `30-woken` cannot leave the pool (5200 gate) before talking to Hadwen, so no other NPC sees that state.

- [ ] **Step 1: Write the failing tests**

In `internal/behaviortree/hadwen_rite_guard_test.go`:
- replace the file comment's `actions about 37 seconds long, and 30-end is granted only at its close. The` with `actions about 41 seconds long, and 30-woken is granted only at its close. The`, and `passed every gate (the asker still lacks 30-end) and queued a second full` with `passed every gate (the asker still lacks 30-woken) and queued a second full`;
- replace

```go
	// 30-rite as the rite starts, then 30-end once: the two roads are offer
	// topics in his dialogue now, so the rite no longer grants quest 29
	// (#375 R17).
	require.Equal(t, []string{"30-rite", "30-end"}, granted[8142], "the first asker must be Opened exactly once")
	require.Equal(t, []string{"30-rite", "30-end"}, granted[8143], "the second asker must be Opened exactly once")
```

with

```go
	// 30-rite as the rite starts, then 30-woken once at its close. 30-end
	// comes from his hand-in topic when the player tells him the rite is
	// done (#375 phase 7).
	require.Equal(t, []string{"30-rite", "30-woken"}, granted[8142], "the first asker's rite must close exactly once")
	require.Equal(t, []string{"30-rite", "30-woken"}, granted[8143], "the second asker's rite must close exactly once")
```

In `internal/behaviortree/hadwen_midrite_test.go`:
- replace `		Steps: []quests.QuestStep{{Id: "start"}, {Id: "rite"}, {Id: "end"}},` with `		Steps: []quests.QuestStep{{Id: "start"}, {Id: "rite"}, {Id: "woken"}, {Id: "end"}},`
- replace `	require.Equal(t, []string{"30-rite", "30-end"}, granted[8142])` with `	require.Equal(t, []string{"30-rite", "30-woken"}, granted[8142])`
- append:

```go
// The pool's movement gate (room 5200) holds every player who is not yet
// Opened and names the next step for where they are (#375 phase 7): the
// offer before the quest, the begin line once it is taken, patience while
// the rite runs, and the hand-in once it is over. An Opened player walks.
func TestPoolGateNamesTheNextStep(t *testing.T) {
	seedQuest30(t)
	said := []string{}
	orig := actionRegistry["send_user_text"]
	actionRegistry["send_user_text"] = func(params map[string]any, ctx *EvalContext) Result {
		said = append(said, getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["send_user_text"] = orig })

	_, room := sightScene(t, "city")
	u := users.NewTestUser(8146, "brann", "Brann", 98146)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8146: u}))
	room.AddPlayer(8146)
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "rooms", "pothole_coulee", "5200.yaml")
	require.NoError(t, GetEngine().LoadRoomTree(room.RoomId, tree))
	t.Cleanup(func() { clearEngineRoomState(t, room.RoomId); clearRoomStates(t, room.RoomId) })

	walk := func(progress map[int]string) (bool, string) {
		said = said[:0]
		u.Character.QuestProgress = progress
		held := TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8146, RoomId: room.RoomId, Command: "south",
		})
		return held, strings.Join(said, " ")
	}
	for _, c := range []struct {
		name     string
		progress map[int]string
		want     string
	}{
		{"no quest", map[int]string{}, "[!]"},
		{"taken", map[int]string{30: "start"}, "begin the rite"},
		{"mid-rite", map[int]string{30: "rite"}, "under way"},
		{"woken", map[int]string{30: "woken"}, "tell him the rite is done"},
	} {
		held, text := walk(c.progress)
		require.True(t, held, "%s: the pool must hold the player", c.name)
		require.Contains(t, text, c.want, "%s", c.name)
	}
	held, _ := walk(map[int]string{30: "end"})
	require.False(t, held, "an Opened player walks on")
}
```

and add `"path/filepath"` and `"runtime"` and `"github.com/GoMudEngine/GoMud/internal/users"` to that file's imports (it already imports `strings`, `testing`, `quests`, `require`).

In `internal/dialogue/velk_opener_test.go` replace `	"30": {"start", "rite", "end"},` with `	"30": {"start", "rite", "woken", "end"},`.

In `internal/dialogue/hadwen_midrite_test.go`, inside `t.Run("during the rite", ...)` add before its closing `})`:

```go
		require.NotContains(t, ids, "rite_report", "the hand-in waits for the rite's close")
```

then replace the whole `t.Run("after the rite", ...)` block with:

```go
	t.Run("rite done, not yet told", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "woken"})
		text, _ := df.OpenerFor(ps)
		require.Contains(t, text, "Come, tell me how it went")
		ids := strings.Fields(menuIds(ps))
		require.Contains(t, ids, "rite_report", "the hand-in is open once the rite is over (#375 phase 7)")
		require.NotContains(t, ids, "rite_wait", "the rite is over")
		require.NotContains(t, ids, "begin")
		require.NotContains(t, ids, "road_toke", "the roads wait for the hand-in")
		require.NotContains(t, ids, "another_way")
	})
	t.Run("after the hand-in", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "end"})
		text, _ := df.OpenerFor(ps)
		require.Contains(t, text, "How does it sit")
		ids := strings.Fields(menuIds(ps))
		require.NotContains(t, ids, "begin")
		require.NotContains(t, ids, "rite_wait")
		require.NotContains(t, ids, "rite_report")
		require.Contains(t, ids, "road_toke")
		require.Contains(t, ids, "road_esk")
		require.Contains(t, ids, "another_way", "the skip path is an info topic")
	})
```

In `main_quests_offer_test.go` replace the whole `TestSmoke_AutoStartedQuestsNotAbandonable` (comment included) with:

```go
// On shipped data, quests 28 (Waking to Gaius) and 30 (The Awakening) are
// abandonable since #375 phase 7: only Dewey's and Hadwen's offers start
// them, so nothing would take a shelved one back up by itself. Quest 42 (The
// Hunt) still is. The test logs every quest the rule still takes out of
// abandonable (offered with a resuming offer, but auto-started), so the
// owner sees the full effect.
func TestSmoke_AutoStartedQuestsNotAbandonable(t *testing.T) {
	loadShippedQuestWorld(t)
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	t.Cleanup(usercommands.SetAutoStartedForTest(nil))
	offerIndexDoor()

	c := characters.New()
	c.QuestProgress = map[int]string{28: "start", 30: "start", 42: "start"}
	for _, id := range []int{28, 30, 42} {
		if !usercommands.Abandonable(c, id) {
			t.Errorf("quest %d is not abandonable; it has a clean offer and no automatic start", id)
		}
	}

	w := questworldForOfferDoor()
	auto := questloop.AutoStarts(w)
	for _, id := range []int{28, 30} {
		if len(auto[id]) > 0 {
			t.Errorf("quest %d still starts by itself: %v", id, auto[id])
		}
	}
	ids := make([]int, 0, len(auto))
	for id := range auto {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	t.Logf("%d quests have an automatic start: %v", len(ids), ids)
	for _, id := range ids {
		if dialogue.Resumable(id) {
			name := ""
			if q := quests.GetQuestById(id); q != nil {
				name = q.Name
			}
			t.Logf("ruling 6 removes quest %d (%s): %v", id, name, auto[id])
		}
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/behaviortree/ -run 'TestHadwen|TestPoolGateNamesTheNextStep'
go test ./internal/dialogue/ -run TestHadwenMidRite
DOGMUD_BOOT_SMOKE=1 go test -run TestSmoke_AutoStartedQuestsNotAbandonable -timeout 600s .
```

Expected: the Hadwen grant tests fail (`[30-rite 30-end]` against `[30-rite 30-woken]`); `TestPoolGateNamesTheNextStep` fails on `no quest` (text lacks `[!]`); `TestHadwenMidRite/rite_done,_not_yet_told` fails (no `rite_report`); the smoke test fails with `quest 30 is not abandonable`.

- [ ] **Step 3: Replace quest 30**

Write `_datafiles/world/dogmud/quests/30-the_awakening.yaml` with exactly:

```yaml
# Quest 30: The Awakening (Pothole Coulee hub, the mandatory beat)
#
# Token flow (#375 phase 7, Pothole Coulee pass):
#   30-start: granted only by Cleric Hadwen's offer topic rite_offer
#             (dialogue/pothole_coulee/9100.yaml). Nothing grants it by
#             itself any more (the room_enter 5200 trigger and room 6467's
#             grant are gone), so the quest can be set aside and taken up
#             again from his offer (phase 5, owner ruling 6).
#   30-rite:  granted by Hadwen's behavior tree as the rite starts
#             (behaviors/pothole_coulee/9100-cleric_hadwen.yaml).
#   30-woken: granted by the same tree at the close of the rite sequence.
#   30-end:   granted by Hadwen's hand-in topic rite_report when the player
#             tells him the rite is done. It keeps its meaning, "Opened":
#             every gate in the coulee reads it, and the pool's movement gate
#             (behaviors/rooms/pothole_coulee/5200.yaml) holds the player
#             until it lands.
# Blank-slate Awakening: the rite grants NO starter mutation; mutations are
# earned afterward through use-based drift.

questid: 30
name: The Awakening
description: Undergo the Awakening Rite at the pool in Pothole Coulee.
secret: false

steps:
  - id: start
    destination:
      mob: 9100  # follow Cleric Hadwen (schedule pothole_hadwen); a fixed room is wrong for part of the day
      room: 5200  # fallback when Cleric Hadwen is not spawned
    description: "Cleric Hadwen will open the pool for you when you are ready. Talk to him at the pool and choose to begin the rite."
    hint: "Stand at the Awakening Pool and type talk hadwen. When you are ready, choose to begin the rite."
  # Granted by Hadwen's tree the moment the rite starts, so his dialogue can
  # tell a player mid-rite from one still waiting (#375 phase 1 playtest).
  # A rite cut short by a logout or a restart leaves a player here; his
  # rite_wait topic then starts the rite again.
  - id: rite
    destination:
      mob: 9100
      room: 5200
    description: "The rite has begun. Stay by the Awakening Pool until Cleric Hadwen is done."
    hint: "Stay by the Awakening Pool until the rite is done. If nothing is happening, type talk hadwen and choose Is the rite still going?"
  # Granted by Hadwen's tree at the close of the rite (#375 phase 7). The
  # player then tells him it is done, through his hand-in rite_report, which
  # grants 30-end.
  - id: woken
    destination:
      mob: 9100
      room: 5200
    description: "The rite is done, and something in you is newly awake. Cleric Hadwen wants a word before you go."
    hint: "Cleric Hadwen waits at the Awakening Pool to hear how the rite went. Type talk hadwen, then choose The Awakening."
  - id: end
    description: "The pool has opened you. Two roads from here: learn the town with Crier Toke, or leave through Warden Esk's arch. Cleric Hadwen can point you down either one."
    # An end hint is never displayed (the quest is complete). What a new
    # Opened needs to hear lives where it is shown: Hadwen's road offers and
    # the rite's closing lines name the next road, and the reward line below
    # names the light spell.
    hint: "Talk to Cleric Hadwen to choose a road on from here."

rewards:
  actor: >-
    Something under your skin settles into place, the way silt settles
    in still water. You are not quite who you were when you arrived.
    A small light waits in you now too: in any dark place, type cast
    glow.
```

- [ ] **Step 4: Edit Hadwen's tree**

In `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml`:

(a) Replace lines 1-23 (from the first line, which begins `# Cleric Hadwen (9100)` and names the rite-keeper, through `# its 30-end gate, so the player cannot choose it again.`) with:

```yaml
# Cleric Hadwen (9100), rite-keeper at the Awakening Pool (room 5200).
#
# Owns the Awakening Rite beat of quest 30 (#375 phase 7 token flow):
#   - his dialogue (dialogue/pothole_coulee/9100.yaml) grants 30-start from
#     the offer rite_offer, and 30-end from the hand-in rite_report;
#   - this tree answers the player choosing to begin the rite (talk hadwen,
#     topic `begin`, #375 R17; the topic's text is empty, so this tree
#     speaks every line) while 30-start is held, grants 30-rite as the rite
#     starts, plays the paced rite sequence, and grants 30-woken at its
#     close. Blank-slate Awakening: the rite grants NO starter mutation;
#     mutations are earned afterward through use-based drift.
# Single-shot: 30-woken is granted at the CLOSE of the rite (a delayed
# action), so for the ~41 s the rite runs the asker still lacks 30-woken,
# and token gating alone would let a second begin queue a second rite
# (2026-10-17 audit S6). The `rite_running` state flag closes that window:
# set instantly when the rite starts, checked by the rite and by the
# mid-rite answer below. Hadwen serves every newcomer, so his clear is a
# delayed action queued with the rite itself. Queued closures run whether
# or not the asker is still online, and BehaviorState is not saved
# (mobs.Mob.BTreeState is yaml:"-"), so a logout or a reboot mid-rite
# cannot leave the flag stuck. After the rite, the begin topic is hidden by
# its 30-rite gate and rite_wait by its 30-woken gate.
```

(b) Replace

```yaml
        - type: condition
          check: player_has_quest
          quest: "30-start"
        - type: condition
          check: player_missing_quest
          quest: "30-end"
```

with

```yaml
        - type: condition
          check: player_has_quest
          quest: "30-start"
        - type: condition
          check: player_missing_quest
          quest: "30-woken"
```

(c) Replace `        # begin. 30-end at the close supersedes it.` with `        # begin. 30-woken at the close supersedes it.`

(d) #690 dash. Replace

```yaml
            not. The body answers effort, conflict, fear, wanting -- it
            changes. We call the changes mutations, and none of them are
```

with

```yaml
            not. The body answers effort, conflict, fear and wanting. It
            changes. We call the changes mutations, and none of them are
```

(e) Replace

```yaml
          text: >-
            Talk to me again when you are ready, with
            <ansi fg="command">talk hadwen</ansi>, and I will set you
            on either road, or both. The coulee will still be here
            either way.
          delay: 40.0
```

with

```yaml
          text: >-
            When you are ready, type
            <ansi fg="command">talk hadwen</ansi> and choose The
            Awakening to tell me the rite is done. Then I will set you
            on your road.
          delay: 40.0
```

(f) Replace everything from `        # Blank-slate Awakening: no starter mutation is granted during the rite.` to the end of the file with:

```yaml
        # Blank-slate Awakening: no starter mutation is granted during the rite.
        # The pool opens the capacity; every mutation is earned afterward through
        # use-based drift (see the mutation-graph acquisition design). The rite's
        # close is the FINAL beat, after the last line at 40.0, so the step
        # lands as the rite ends. 30-end, with its reward line, comes from his
        # hand-in topic rite_report when the player tells him it is done (#375
        # phase 7). This action honors the static `delay` param
        # (behaviortree/actions.go).
        - type: action
          do: grant_quest
          quest: "30-woken"
          delay: 40.5
        # The rite is over: clear the flag so the next newcomer can begin.
        # Queued after the 30-woken grant (the queue runs in order), so by the
        # time a second begin can pass the flag the asker already holds
        # 30-woken.
        - type: action
          do: set_state
          key: rite_running
          per_player: true
          value: ""
          delay: 41.0

    # ── GREETING: a player who has not finished the rite arrives (the
    #    mud-vet start fires player_enter here, internal/usercommands/start.go;
    #    a walk-in does too). Since #375 phase 7 nothing grants quest 30 by
    #    itself, so the line names the offer. Dewey's send-off names it for
    #    the newcomer route, whose move_player fires no player_enter. ──
    - type: sequence
      event: player_enter
      children:
        - type: condition
          check: player_missing_quest
          quest: "30-woken"
        - type: action
          do: say
          text: >-
            New to Gaius? Welcome. This is Pothole Coulee, and I keep
            its pool, which is waiting for you. Type
            <ansi fg="command">talk hadwen</ansi> and choose The
            Awakening, the line marked [!], when you are ready.
          delay: 2.0

    # ── AMBIENT BACKSTOP: every so often, repeat the invitation aloud.
    #    Gated on a player who has not finished the rite actually being
    #    present (custom condition: mob_idle has no triggering player, so
    #    plain player_missing_quest can't apply here). ──
    - type: sequence
      event: mob_idle
      children:
        - type: condition
          check: player_in_room_missing_quest
          quest: "30-woken"
        - type: condition
          check: random_chance
          percent: 4
        - type: action
          do: say
          text: >-
            If you are new to Gaius and not yet Opened, type
            <ansi fg="command">talk hadwen</ansi> and choose The
            Awakening. The pool is patient.
```

- [ ] **Step 5: Replace Hadwen's dialogue**

Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml` with exactly:

```yaml
mobid: 9100
zone: Pothole Coulee
defaultMood: friendly
opener:
  text: Welcome, and be easy. My name is Hadwen, and this is the Awakening Pool,
    which I keep. I have opened more people at this water than I can count, and
    I have never once been bored by it.
  # The root is for a player who has not taken The Awakening (#375 phase 7:
  # nothing grants it by itself), so the hint names the offer and its mark.
  hints: Choose The Awakening, the line marked [!], to ask for the rite. Other
    lines tell you about the Chrysalis.
  variants:
  # Mid-rite: the tree grants 30-rite as the rite starts (the dialogue cannot
  # read its rite_running state). The rite's close grants 30-woken.
  - questRequired:
    - 30-rite
    questExcluded:
    - 30-woken
    text: Quietly, now. The rite is under way, and the pool is still speaking to
      you. Stay close to the water and let it finish.
    hints: The rite takes a little while. Stay by the pool until it is done.
  # The rite is over (30-woken) and the player has not told him yet: his
  # hand-in rite_report grants 30-end (#375 phase 7).
  - questRequired:
    - 30-woken
    questExcluded:
    - 30-end
    text: There you are, still standing, with something new under your skin. The
      pool is finished with you. Come, tell me how it went.
    hints: The line marked [?] finishes The Awakening. Type its number.
  - questRequired:
    - 30-start
    questExcluded:
    - 30-rite
    - 30-end
    text: The pool is ready when you are. There is no hurry in it, but there is
      also nothing left to wait for.
    hints: When you are ready, choose to begin the rite.
  # N2: the road hint names only the roads this player is actually offered.
  # First match wins, so the no-road states come first. HasQuest(88-start)
  # stays true at 88-end, so a -start token covers "taken" either way.
  # Both roads taken.
  - questRequired:
    - 30-end
    - 88-start
    - 89-start
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. You have walked both roads from
      here, so the pool has done its part for you.
    hints: Choose a line below to hear about mutations, seeing in the dark, or
      your name.
  # Toke's road is hidden (footing started) and Esk's is taken.
  - questRequired:
    - 30-end
    - 31-start
    - 89-start
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. You have walked both roads from
      here, so the pool has done its part for you.
    hints: Choose a line below to hear about mutations, seeing in the dark, or
      your name.
  # One road left: Toke's (Esk's taken).
  - questRequired:
    - 30-end
    - 89-start
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. One road from here is still open
      to you, if you want it.
    hints: Choose Toke's Road below to learn the town. Other lines below tell you
      about mutations, seeing in the dark, or your name.
  # One road left: Esk's (Toke's taken, or footing already started).
  - questRequired:
    - 30-end
    - 88-start
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. One road from here is still open
      to you, if you want it.
    hints: Choose Esk's Road below to leave the coulee. Other lines below tell you
      about mutations, seeing in the dark, or your name.
  - questRequired:
    - 30-end
    - 31-start
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. One road from here is still open
      to you, if you want it.
    hints: Choose Esk's Road below to leave the coulee. Other lines below tell you
      about mutations, seeing in the dark, or your name.
  # Both roads open.
  - questRequired:
    - 30-end
    text: How does it sit, the new thing under your skin? Settling, I hope. The first
      one always argues a little before it agrees. Two roads lead on from here, and
      I can point you down either one.
    hints: Choose Toke's Road or Esk's Road below to set out. Other lines below
      tell you about mutations, seeing in the dark, or your name.
topics:
# R17: the two roads from the pool. Take either, both or neither. Both are
# gated on 30-end, which Hadwen's own hand-in rite_report grants, so each
# sits on the NPC who took the previous hand-in (the chain rule, #375 phase
# 7). Toke's road is hidden once footing has started (31-start), since the
# player is already on it. Quest 29 (Two Roads) is retired. Plan 4 of the
# phase 7 pass moves Esk's Road (89) to Crier Toke and removes road_esk;
# until then it stays, beside the another_way info topic.
- key: road_toke
  kind: offer
  quest: 88
  label: Toke's Road
  playerLine: How do I learn my way around the town?
  text: Go south to the square and find Crier Toke at the notice post. He knows every
    door in the coulee, and he will walk you through the town properly, where to
    sleep, where to buy, where to learn. If Gaius is new to you, that is the road
    I would take.
  hints: From the Awakening Pool, go south to Hub Square and type talk toke.
  questRequired:
  - 30-end
  questExcluded:
  - 88-start
  - 88-end
  - 31-start
  grantsQuest: 88-start
- key: road_esk
  kind: offer
  quest: 89
  label: Esk's Road
  playerLine: How do I leave the coulee for the wider world?
  text: Warden Esk keeps the arch in the Threshold House, south through the market.
    The arch opens on Thornwall, and from Thornwall every road in the Marches. If
    you already know this world and only needed the pool, go to him. The coulee will
    still be here either way.
  hints: From the Awakening Pool, go south three times, through Hub Square and Market
    Row, to the Threshold House. Then type talk esk.
  questRequired:
  - 30-end
  questExcluded:
  - 89-start
  - 89-end
  grantsQuest: 89-start
# The skip path (#375 phase 7; requirements page: "the Pothole Coulee portal
# quest is an info topic"). Warden Esk's passage topic does the crossing.
- key: another_way
  kind: info
  label: Is there another way out?
  text: There is. Warden Esk keeps the old arch in the Threshold House, south
    through the market. It opens on Thornwall and every road beyond it. If you
    already know a world like this one, he will let you through. The coulee will
    still be here if you ever want it.
  hints: From the Awakening Pool, go south three times, through Hub Square and Market
    Row, to the Threshold House. Type talk esk, then choose to open the arch.
  questRequired:
  - 30-end
- key: newcomer_mutations
  kind: info
  label: How will I grow from here?
  text: 'The pool woke something in you: a capacity, not yet a shape. As you grow,
    the chrysalis changes you, and each change it makes is called a mutation. In
    time it will change you. Type mutations to see what has changed in you so far.
    And hear this, for it is the law of Gaius: you grow by doing, and only by doing.
    Swing a blade and your blade-arm sharpens. Study, and your mind does. Type skills
    to see what you can do, and status to see your attributes. Both climb as you
    use them. Go and use them.'
  hints: You are newly Opened. Type <ansi fg="command">mutations</ansi> to watch the
    changes arrive. For more, type help mutations.
  questRequired:
  - 30-end
  - 21-start
  questExcluded:
  - 21-end
  moodChange: friendly
- key: rite_offer
  kind: offer
  quest: 30
  label: The Awakening
  playerLine: Will you open the pool for me?
  text: Everyone who stays in Gaius is Opened, sooner or later. That is our word
    for it, once the Chrysalis takes hold in you. The world insists. The rite only
    makes the first change a kindness instead of a surprise. You stand at the edge,
    I do the small part I do, and the pool does the rest. It does not hurt. It has
    never once hurt. Say yes, and we begin whenever you are ready.
  hints: Accept, then choose to begin the rite when you are ready. There is no hurry.
  questExcluded:
  - 30-start
  - 30-end
  grantsQuest: 30-start
  moodChange: friendly
# The end of The Awakening (#375 phase 7): the rite's close grants 30-woken,
# and telling Hadwen it is done grants 30-end, the "Opened" token every gate
# in the coulee reads. The quest's reward line plays here, and his road
# offers open on 30-end.
- key: rite_report
  kind: handin
  quest: 30
  label: The Awakening
  playerLine: The rite is done. I can feel something new in me.
  text: Then the pool has done its part, and you are one of the Opened. Welcome.
    It will go on settling for a while. That is normal. When you are ready, I can
    point you down a road.
  hints: Quest complete. Choose Toke's Road below to learn the town.
  questRequired:
  - 30-woken
  questExcluded:
  - 30-end
  grantsQuest: 30-end
  moodChange: friendly
- key: rite_waiting
  kind: info
  label: What happens in the rite?
  text: Everyone who stays in Gaius is Opened, sooner or later. That is our word
    for it, once the Chrysalis takes hold in you. The world insists. The rite only
    makes the first change a kindness instead of a surprise. You stand at the edge,
    I do the small part I do, and the pool does the rest. It does not hurt. It has
    never once hurt. Tell me when you are ready and we will begin.
  hints: When you are ready, choose to begin the rite. There is no hurry.
  questRequired:
  - 30-start
  questExcluded:
  - 30-rite
  - 30-end
# R17: one behavior topic. The text is empty on purpose: the tree speaks in
# both cases. Before the rite its branch says "Then we begin. Step to the
# edge with me." and plays the rite; while the rite is already running (the
# per-player rite_running state) its mid-rite branch says "Hush, now...".
# A topic text here would be spoken in both cases and contradict the hush.
- key: begin
  kind: info
  label: I'm ready. Let us begin the rite.
  text: ""
  hints: The rite takes a little while. Stay by the pool until it is done.
  behavior: true
  questRequired:
  - 30-start
  questExcluded:
  - 30-rite
  - 30-end
# Mid-rite (30-rite held), in place of begin. The tree answers: the hush line
# while the rite runs, or, when a logout or a restart cut the rite short, it
# says so and starts the rite again. Text empty for the same reason as begin.
# Hidden once the rite's close grants 30-woken.
- key: rite_wait
  kind: info
  label: Is the rite still going?
  text: ""
  hints: The rite takes a little while. Stay by the pool until it is done.
  behavior: true
  questRequired:
  - 30-rite
  questExcluded:
  - 30-woken
- key: rite_done
  kind: info
  label: What happens after the pool?
  text: The pool opens a person once. After that, the opening is yours to continue
    through effort, conflict and wanting. The body remembers all of it, and in time it answers.
    You do not need me for the second change, or the tenth.
  hints: Choose a line below to hear about mutations or the Chrysalis.
  questRequired:
  - 30-end
- key: mutations
  kind: info
  label: What is a mutation?
  text: A mutation is a change the chrysalis makes in you as you grow. Type <ansi
    fg="command">mutations</ansi> and the truth of yourself is listed plainly, which
    is more than most worlds offer. Each one woke because of something you did or
    survived. Read them now and then. They are a better diary than most.
  hints: Choose a line below to hear about your name, seeing in the dark, or the
    Chrysalis.
  questRequired:
  - 30-end
- key: glow
  kind: info
  label: How do I see in the dark?
  text: With the Chrysalis Glow. It is a spell, and every soul wakes knowing it.
    Type cast glow and hold still a moment while it gathers. The spores around you
    give off a soft light, and it stays with you a while. Then it fades, and you
    cast it again. Each casting spends a little of your conviction, the CP bar on
    your prompt. The world
    keeps dark places, caves and deep mines and shuttered rooms, and the dark in
    them is no metaphor. It will blind you until you make your own light.
  hints: When it is too dark to see, type cast glow. For more, type help
    chrysalis-glow.
  marked: true
  questRequired:
  - 30-end
  moodChange: friendly
- key: rename
  kind: info
  label: What if my name no longer fits me?
  text: Some people walk out of the pool and their old name still fits. Some find
    it hangs on them like a borrowed coat. If yours no longer fits the shape of you,
    type <ansi fg="command">rename</ansi> followed by the name you choose. The coulee
    will learn the new one fast enough.
  hints: Choose a line below to hear about mutations or the Chrysalis.
  questRequired:
  - 30-end
- key: chrysalis
  kind: info
  label: What is the Chrysalis?
  text: The Chrysalis is the form between what was and what will be. Gaius is made
    of belief. The land believes itself into shape, and so does everything living
    on it. We are not a faith of dogma. We are a faith of becoming, and the pool is
    the oldest becoming in this valley.
  hints: Choose a line below to hear about the rite. For the old way, talk to
    Granny Wicker.
- key: help_literacy
  kind: info
  label: What do I do when I don't know something?
  text: When I do not know a thing, I ask the water. When you do not know a thing,
    type <ansi fg="command">help</ansi>. It is patient, thorough, and never tired
    of the question. Add a word to narrow it, like <ansi fg="command">help sleep</ansi>.
  hints: Choose a line below to hear about the rite or the Chrysalis.
```

- [ ] **Step 6: Replace the pool's gate**

Write `_datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5200.yaml` with exactly:

```yaml
# The Awakening Pool (5200): movement gate for the un-Opened.
#
# A player without quest token 30-end cannot leave this room: every
# movement command is intercepted and answers with the exact next step.
# This is the hard floor of the newbie funnel: the only way forward IS the
# taught command. Opened players (30-end) pass through untouched.
#
# Since #375 phase 7 nothing grants quest 30 by itself: the player takes it
# from Cleric Hadwen's offer (The Awakening, marked [!]), does the rite, and
# then tells him it is done (his hand-in, marked [?]), which grants 30-end.
# So the answer names the step for each state, first match wins: the rite
# done, the rite under way, the quest taken, and no quest yet.
#
# Narrator text (send_user_text), not mob_say: the block must hold even in
# the freak case Hadwen is absent, and the pool itself doing the holding is
# true to the room. Characters from before the Pothole Coulee were given
# 30-end by characters.MigrateNewbieAwakening.

tree:
  type: selector
  children:

    # ── the rite is done, but Hadwen has not heard it ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [north, south, east, west, n, s, e, w, up, down, u, d, go]
        - type: condition
          check: player_has_quest
          quest: "30-woken"
        - type: condition
          check: player_missing_quest
          quest: "30-end"
        - type: action
          do: send_user_text
          text: >-
            The pool's stillness holds you a moment longer, the way a hand
            on a shoulder holds. Cleric Hadwen wants a word first. Type
            <ansi fg="command">talk hadwen</ansi> and choose The Awakening
            to tell him the rite is done.
        - type: action
          do: intercept

    # ── the rite is under way ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [north, south, east, west, n, s, e, w, up, down, u, d, go]
        - type: condition
          check: player_has_quest
          quest: "30-rite"
        - type: condition
          check: player_missing_quest
          quest: "30-woken"
        - type: action
          do: send_user_text
          text: >-
            The pool's stillness holds you where you stand. The rite is
            still under way. Stay by the water until Cleric Hadwen is done.
        - type: action
          do: intercept

    # ── the quest is taken, the rite not begun ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [north, south, east, west, n, s, e, w, up, down, u, d, go]
        - type: condition
          check: player_has_quest
          quest: "30-start"
        - type: condition
          check: player_missing_quest
          quest: "30-rite"
        - type: action
          do: send_user_text
          text: >-
            The pool's stillness holds you where you stand, not unkindly.
            The rite comes first. Type
            <ansi fg="command">talk hadwen</ansi> and choose to begin the
            rite when you are ready.
        - type: action
          do: intercept

    # ── no quest yet: the un-Opened stay at the pool ──
    - type: sequence
      event: room_command
      children:
        - type: condition
          check: command_matches
          commands: [north, south, east, west, n, s, e, w, up, down, u, d, go]
        - type: condition
          check: player_missing_quest
          quest: "30-end"
        - type: action
          do: send_user_text
          text: >-
            The pool's stillness holds you where you stand, not
            unkindly, the way a hand on a shoulder holds. The rite
            comes first. Type <ansi fg="command">talk hadwen</ansi>
            and choose The Awakening, the line marked [!], and the
            whole coulee opens with you.
        - type: action
          do: intercept
```

- [ ] **Step 7: Manifests, backlog and comments**

In `_datafiles/world/dogmud/quests/step-ids.txt` insert `30-woken` on its own line directly after `30-start`.

In `_datafiles/world/dogmud/dialogue/topic-keys.txt` insert `9100.another_way` on its own line directly before `9100.begin`, and `9100.rite_report` directly after `9100.rite_offer`.

In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete exactly these 5 lines:

```
chain 30 mob 9100 topic road_esk
chain 30 mob 9100 topic road_toke
end 30 mob 9100 tree grant_quest 30-end
end 30 step end
start 30 quest 30 trigger room_enter room=5200 grants=30-start
```

In `internal/usercommands/start.go` replace

```go
		// The Awakening's 30-start comes from its room_enter trigger on 5200,
		// raised by hooks.QuestRoomEnter for this move like any other.
```

with

```go
		// Nothing grants The Awakening by itself (#375 phase 7): Hadwen's
		// greeting above names his offer, and the pool's movement gate
		// (behaviors/rooms/pothole_coulee/5200.yaml) holds the player until
		// the rite is done and handed in.
```

In `internal/behaviortree/actions_quest.go` replace `	// Without this, behavior-tree grants (e.g. Hadwen granting 30-end at the` with `	// Without this, behavior-tree grants (e.g. Hadwen granting 30-woken at the`.

In `main_quests.go` replace `// back up without the player choosing to (owner ruling 6, quest 30). That is` with `// back up without the player choosing to (owner ruling 6). That is`.

- [ ] **Step 8: Run and pass**

```bash
gofmt -l internal/
go build ./...
go test ./internal/behaviortree/ ./internal/dialogue/ ./internal/questengine/ ./internal/questloop/ ./internal/usercommands/ ./internal/guards/
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop|TestOfferIndexDoor' -timeout 900s .
```

Expected: every package `ok` (`TestNewbieQuestHintsNameOneCommand` passes: the `woken` hint names one command); the root `ok`, with the sweep reporting no fresh and no stale line. Null probe: temporarily put `- 30-end` back as the only `questExcluded` entry of `rite_wait`, rerun `go test ./internal/dialogue/ -run TestHadwenMidRite`, see `rite done, not yet told` fail on `rite_wait`, restore.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/quests/30-the_awakening.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml _datafiles/world/dogmud/behaviors/rooms/pothole_coulee/5200.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/behaviortree/hadwen_rite_guard_test.go internal/behaviortree/hadwen_midrite_test.go internal/dialogue/hadwen_midrite_test.go internal/dialogue/velk_opener_test.go main_quests_offer_test.go main_quests.go internal/usercommands/start.go internal/behaviortree/actions_quest.go
git commit -F - <<'EOF'
content(quest 30): The Awakening ends at a Hadwen hand-in

Only Hadwen's offer starts quest 30 now: the room_enter 5200 trigger
goes (6467's grant went in the quest 28 commit). A new step `woken`
sits between rite and end: the rite's close grants it, and his new
hand-in rite_report grants 30-end, so his road offers chain from his
own hand-in. Quest 30 (and 28) become abandonable with no code change.
Adds the skip-path info topic "Is there another way out?"; road_esk
stays until plan 4. The pool gate names the next step in every state.
Clears 5 backlog lines.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 5: Toke's Road ends only at Toke; footing gates on it

**Executor:** Sonnet

**Files:**
- Modify (whole file): `_datafiles/world/dogmud/quests/88-tokes_road.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`
- Modify: `_datafiles/world/dogmud/quests/31-find_your_footing.yaml` (comment)
- Modify: `_datafiles/world/dogmud/quests/loop-backlog.txt`

- [ ] **Step 1: See the findings this task clears**

```bash
grep -nE '^(chain 30 mob 9106|end 88 )' _datafiles/world/dogmud/quests/loop-backlog.txt
```

Expected: two lines, `chain 30 mob 9106 topic footing_offer` and `end 88 quest 88 trigger quest_granted quest_token=31-start grants=88-end`.

- [ ] **Step 2: Replace quest 88**

Write `_datafiles/world/dogmud/quests/88-tokes_road.yaml` with exactly:

```yaml
# Quest 88: Toke's Road (Pothole Coulee, post-rite bridge)
#
# Cleric Hadwen offers it once the player has told him the rite is done
# (30-end, his hand-in rite_report), and Crier Toke takes the hand-in, so the
# chain runs Hadwen, then Toke (owner ruling R17; #375 phase 7). Toke's
# hand-in unlocks Find Your Footing (31), which he offers next.
#
# Token flow:
#   88-start: granted on Accept of Hadwen's offer topic road_toke
#             (dialogue/pothole_coulee/9100.yaml). Hidden once footing has
#             started (31-start), for a save that took footing first.
#   88-end:   granted only by Crier Toke's hand-in topic road_toke_end
#             (dialogue/pothole_coulee/9106.yaml). The quest_granted 31-start
#             trigger that also ended it is gone (#375 phase 7), so the
#             hand-in is the quest's one end.
#
# The destination follows Toke on his schedule (pothole_toke); room 5203
# (Hub Square) is the fallback.

questid: 88
name: Toke's Road
description: >-
  Cleric Hadwen has pointed you to Crier Toke, who knows every door in
  Pothole Coulee and will walk you through the town.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # follow Crier Toke (schedule pothole_toke)
      room: 5203  # fallback when Crier Toke is not spawned
    description: >-
      Crier Toke waits in Hub Square, just south of the Awakening Pool. He
      will teach you the town.
    hint: >-
      From the Awakening Pool, go south to Hub Square. Type talk toke.
  - id: end
    description: >-
      You found Crier Toke, and he will walk you through the town.
```

- [ ] **Step 3: Edit Toke's dialogue**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`:

(a) Replace the 31-end opener variant

```yaml
  - questRequired:
    - 31-end
    text: The town knows your face now. I made sure of it. When you want more than
      the hub, pick a mouth and I will send you down it. The coulee has seven of them,
      and every one leads somewhere that will teach you something.
    hints: Choose a line below to hear about the seven mouths or the day's news,
      or take one of the seven trails.
```

with

```yaml
  # The hub (#375 phase 7): Toke is the only starter of the seven trades, and
  # a player new to MUDs is told so in plain words. Each trade's first quest
  # is one [!] line on his menu; take one, several or all seven.
  - questRequired:
    - 31-end
    text: 'The town knows your face now. I made sure of it. Here is the rest of
      it. I can start you on any trade the coulee teaches: fighting, the forge,
      brewing, the wilds, casting, the lore of the folk and the long shot. Each
      one has its own teacher, and I send you to the first. Take one, take
      several, or take all seven.'
    hints: Each line marked [!] below starts one trade. Type the number of one to
      hear who teaches it and where. You can take as many as you like.
```

(b) Replace

```yaml
# R17: the end of Toke's Road (quest 88), offered by Cleric Hadwen after the
# rite. Quest 88 also closes when footing starts (its 31-start trigger), so a
# player who takes footing first is never left holding it.
```

with

```yaml
# R17: the end of Toke's Road (quest 88), offered by Cleric Hadwen after the
# rite. This hand-in is the quest's only end (#375 phase 7), and it unlocks
# Find Your Footing below, which is pitched right after it.
```

(c) In `road_toke_end` replace

```yaml
  hints: Toke offers Find Your Footing, a walk around the town. Choose it below.
  questRequired:
  - 88-start
  questExcluded:
  - 88-end
  - 31-start
  grantsQuest: 88-end
```

with

```yaml
  hints: Quest complete. Next, Toke offers Find Your Footing, a walk around the
    town.
  questRequired:
  - 88-start
  questExcluded:
  - 88-end
  grantsQuest: 88-end
```

(d) In `footing_offer` replace

```yaml
  hints: Go north past the Awakening Pool and keep north up the basalt stair to the
    Chrysalis School Hall. Walking in is enough.
  questRequired:
  - 30-end
  questExcluded:
  - 31-start
  - 31-end
  grantsQuest: 31-start
```

with

```yaml
  hints: Go north past the Awakening Pool and keep north up the basalt stair to the
    Chrysalis School Hall. Walking in is enough.
  # Gated on Toke's Road's end, which Toke's own hand-in grants (the chain
  # rule, #375 phase 7).
  breadcrumb: Cleric Hadwen at the pool sends new folk to me. Once he has, I will
    walk you round the whole town.
  questRequired:
  - 88-end
  questExcluded:
  - 31-start
  - 31-end
  grantsQuest: 31-start
```

(e) Replace

```yaml
# R21: the seven trails, each its own offer after Find Your Footing. Each
# starter item rides on the offer of the quest it belongs to. Every trail's
# own trailhead offer stays on its teacher. Quest 50's pouch of shot comes
# from its own quest_granted 50-start trigger (missing_item 30064).
```

with

```yaml
# R21: the seven trails, each its own offer after Find Your Footing. Each
# starter item rides on the offer of the quest it belongs to. Toke is the
# only starter of the seven (#375 phase 7); each first teacher takes the
# hand-in and points a newcomer back here. Quest 50's pouch of shot comes
# from its own quest_granted 50-start trigger (missing_item 30064).
```

- [ ] **Step 4: Quest 31's header and the backlog**

In `_datafiles/world/dogmud/quests/31-find_your_footing.yaml` replace the header line that begins `#   31-start` (it says Toke's dialogue grants it, requiring 30-end) with `#   31-start: granted by Crier Toke's footing_offer (requires 88-end, his own hand-in of Toke's Road)`.

In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete exactly:

```
chain 30 mob 9106 topic footing_offer
end 88 quest 88 trigger quest_granted quest_token=31-start grants=88-end
```

- [ ] **Step 5: Run and pass**

```bash
go test ./internal/questengine/ ./internal/dialogue/ ./internal/usercommands/ ./internal/guards/
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_AutoStartedQuestsNotAbandonable' -timeout 600s .
```

Expected: all `ok` (`TestShippedOfferIndex_Quest88NotAbandonable` still passes: `road_toke` excludes `31-start`; `TestTokeGivesTheWay` unaffected).

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/quests/88-tokes_road.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/31-find_your_footing.yaml _datafiles/world/dogmud/quests/loop-backlog.txt
git commit -F - <<'EOF'
content(coulee): Toke's Road ends only at Toke, and Toke explains the hub

Quest 88 loses its quest_granted 31-start finisher, so Toke's hand-in
is its one end, and Find Your Footing gates on 88-end with a
breadcrumb back to Hadwen. After footing, Toke's opener says in plain
words that he starts every trade and the player may take any number.
Clears 2 backlog lines.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 6: Toke is the only starter of the seven trades

**Executor:** Sonnet

**Files:**
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`, `9116.yaml`, `9128.yaml`, `9136.yaml`, `9144.yaml`, `9153.yaml`, `9161.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/topic-keys.txt`
- Modify: `_datafiles/world/dogmud/quests/loop-backlog.txt`
- Modify: `_datafiles/world/dogmud/quests/32-first_blood.yaml`, `35-first_heat.yaml`, `38-first_brew.yaml`, `41-first_sign.yaml`, `44-first_casting.yaml`, `47-first_words.yaml`, `50-first_shot.yaml`

Each Giver 1's quest-1 hand-in stays exactly where it is (`train_turnin`, `craft_turnin` and `craft_turnin_told`, `brew_turnin`, `sign_turnin`, `cast_turnin`, `words_turnin`, `shoot_turnin`); plans 3a and 3b reshape the lines.

- [ ] **Step 1: Remove the seven offers**

Delete each whole topic block below (from its `- key:` line through its last field, `grantsQuest: ...` or `givesItem: ...`), leaving the next topic's `- key:` line in place:

| File | Topic to delete | Last line of the block |
|---|---|---|
| `9108.yaml` | `train_offer` | `  givesItem: 10043` (the one directly before `- key: train_turnin`) |
| `9116.yaml` | `craft_offer` | `  grantsQuest: 35-start` |
| `9128.yaml` | `brew_offer` | `  grantsQuest: 38-start` |
| `9136.yaml` | `sign_offer` | `  grantsQuest: 41-start` |
| `9144.yaml` | `cast_offer` | `  grantsQuest: 44-start` |
| `9153.yaml` | `words_offer` | `  grantsQuest: 47-start` |
| `9161.yaml` | `shoot_offer` | `  givesItem: 10038` (the one directly before the comment `# Each replacement and its reminder never show together:`) |

- [ ] **Step 2: Breadcrumb each giver to Toke**

Each giver's player who is Opened but has not taken the trade now hears to go to Toke. Replace each block on the left with the block on the right. The hints line is the same in all seven:

```yaml
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9108.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 32-start
    - 32-end
    text: You have been through the rite. Good. Now you learn to use what the Awakening
      gave you. I run combat training in this yard. If you want it, say so.
    hints: Choose First Blood below to begin combat training.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Vorn takes First Blood's
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 32-start
    - 32-end
    text: You have been through the rite. Good. I train fighters, but Crier Toke
      sends new folk to me. Find him in Hub Square and ask him who can teach you to
      fight. Then come back, and we begin.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9116.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 35-start
    - 35-end
    text: You have been through the rite. Good. I teach the forge if you want it,
      iron work, and how raw materials become things you can use.
    hints: Choose First Heat below to learn the forge.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Rusk takes First Heat's
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 35-start
    - 35-end
    text: You have been through the rite. Good. I teach the forge, but Crier Toke
      sends me my students. Ask him in Hub Square who can teach you to work iron,
      then come back to my stall.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9128.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 38-start
    - 38-end
    text: 'You have been through the rite. Good. Steady hands are wasted on the
      unawakened. I teach the brew if you want it: buy a vial and a root, work the
      bench, drink what you make. Say the word.'
    hints: Choose First Brew below to learn the first brew.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Birna takes First Brew's
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 38-start
    - 38-end
    text: You have been through the rite. Good. I teach the brew, but Crier Toke
      sends me my students. Ask him in Hub Square who can teach you to brew, and I
      will be here at the water.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9136.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 41-start
    - 41-end
    text: You have the rite on you. Good. I teach the ground, how to cut a trail
      and follow it, what the scrub will feed you. If you mean to learn, I will show
      you your first sign.
    hints: Choose First Sign below to learn to read the ground.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Tarn takes First Sign's
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 41-start
    - 41-end
    text: You have the rite on you. Good. I teach the ground, but Crier Toke sends
      me the ones who want it. Ask him in Hub Square who can teach you to read the
      wild. Then come find me.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9144.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 44-start
    - 44-end
    text: 'The rite is on you. Good. Then there is something to teach. I show the
      first turning of the mind: how to shape a spell you already carry and let it
      go at a mark. Your first casting is waiting when you mean to begin.'
    hints: Choose First Casting below to learn the first casting.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Grieve takes First
  # Casting's hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 44-start
    - 44-end
    text: The rite is on you. Good. Then there is something to teach, but Crier
      Toke sends me my students. Ask him in Hub Square who can teach you to cast
      spells, and come back up the stair.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9153.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 47-start
    - 47-end
    text: 'The rite is on you. Good. Then there is something here worth your while:
      the way of the folk, which is the way of asking and of knowing where to look
      a thing up. If you mean to learn, I will give you your first words.'
    hints: Choose First Words below to learn the way of the folk.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Wenna takes First Words'
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 47-start
    - 47-end
    text: The rite is on you. Good. There is something here worth your while, the
      way of the folk, but Crier Toke sends me the ones who want it. Ask him in Hub
      Square who can teach you the ways of the folk, then come and sit with me.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

`9161.yaml`, replace

```yaml
  - questRequired:
    - 30-end
    questExcluded:
    - 50-start
    - 50-end
    text: Opened, and steady enough. Good. The terraces are where the coulee learns
      to reach past arm's length. When you are ready to put a stone in the straw,
      I have a sling for you.
    hints: Choose First Shot below to take up the sling and the lesson.
```

with

```yaml
  # Crier Toke starts every trade (#375 phase 7): Iden takes First Shot's
  # hand-in but no longer offers it.
  - questRequired:
    - 30-end
    questExcluded:
    - 50-start
    - 50-end
    text: Opened, and steady enough. Good. The terraces are where the coulee learns
      to reach past arm's length, but Crier Toke sends me my students. Ask him in
      Hub Square who can teach you to shoot. He has a sling for you.
    hints: Crier Toke starts every trade in the coulee. In Hub Square, type talk
      toke.
```

- [ ] **Step 3: See the manifest guard fail, then retire the keys**

```bash
go test ./internal/guards/ -run TestTopicKeyManifest
```

Expected: FAIL naming the 7 keys `9108.train_offer` ... `9161.shoot_offer` as gone from content.

In `_datafiles/world/dogmud/dialogue/topic-keys.txt` delete the 7 lines `9108.train_offer`, `9116.craft_offer`, `9128.brew_offer`, `9136.sign_offer`, `9144.cast_offer`, `9153.words_offer`, `9161.shoot_offer`, then append at the very end of the file:

```
# retired
# The seven Giver 1 offers (#375 phase 7): Crier Toke is the only starter of
# the trades.
9108.train_offer
9116.craft_offer
9128.brew_offer
9136.sign_offer
9144.cast_offer
9153.words_offer
9161.shoot_offer
```

- [ ] **Step 4: Quest headers and the two "Toke or" descriptions**

- `quests/32-first_blood.yaml`: replace `    description: "Crier Toke or Drillmaster Vorn hands you a cudgel when you` with `    description: "Crier Toke hands you a cudgel when you`.
- `quests/50-first_shot.yaml`: replace `      Crier Toke or Marksman Iden hands you a sling when you take this on,` with `      Crier Toke hands you a sling when you take this on,`; replace the four header lines 4-7 (the first begins `#   50-start` and names Iden's shoot_offer; the fourth ends `A quest_granted trigger`) with

```yaml
#   50-start: granted by Crier Toke's send_range offer after quest 31 (R21;
#                   the only starter since #375 phase 7), which hands the
#                   player a sling (givesItem 10038). A quest_granted trigger
```

  and replace `  # Fires on the grant from either road in (Iden's offer or Toke's send_range` with `  # Fires on the grant from Toke's send_range` and the next line `  # offer). missing_item keeps it to one pouch for a player who already` with `  # offer. missing_item keeps it to one pouch for a player who already`.
- `quests/35-first_heat.yaml`: replace `#   35-start  granted by Rusk's craft_offer node (9116.yaml).` with `#   35-start  granted by Crier Toke's send_forge offer (9106.yaml, #375 phase 7).`
- `quests/38-first_brew.yaml`: replace the two lines `#   38-start: granted by Herbalist Birna's brew_offer dialogue node. Requires` and `#             30-end (Awakening).` with `#   38-start: granted by Crier Toke's send_brew offer (9106.yaml, requires` and `#             31-end; #375 phase 7).`
- `quests/41-first_sign.yaml`: replace header line 4 (it begins `#   41-start` and names Tarn's sign_offer) with `#   41-start: granted by Crier Toke's send_steppe offer (9106.yaml, requires 31-end).`
- `quests/44-first_casting.yaml`: replace header line 4 (it begins `#   44-start` and names Grieve's cast_offer) with `#   44-start: granted by Crier Toke's send_folding offer (9106.yaml, requires 31-end).`
- `quests/47-first_words.yaml`: replace header line 4 (it begins `#   47-start` and names Wenna's words_offer) with `#   47-start: granted by Crier Toke's send_folk offer (9106.yaml, requires 31-end).`

For `32-first_blood.yaml`, read lines 1-20 first; if a header comment names `train_offer`, replace that line the same way with `send_drill`. If none does, change nothing else.

- [ ] **Step 5: Backlog**

In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete exactly:

```
chain 30 mob 9108 topic train_offer
chain 30 mob 9116 topic craft_offer
chain 30 mob 9128 topic brew_offer
chain 30 mob 9136 topic sign_offer
chain 30 mob 9144 topic cast_offer
chain 30 mob 9153 topic words_offer
chain 30 mob 9161 topic shoot_offer
```

- [ ] **Step 6: Run and pass**

```bash
go test ./internal/guards/ ./internal/questengine/ ./internal/dialogue/
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators' -timeout 600s .
grep -c '^chain 30 ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

Expected: `ok` everywhere (the giver progress-topic tests in `newbie_*_steps_test.go` use players already on the quest, so the rewritten variant never matches them). Run the last `grep -c` on its own: it prints `0` and exits 1, which is the success case here.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9116.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9136.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt _datafiles/world/dogmud/quests/32-first_blood.yaml _datafiles/world/dogmud/quests/35-first_heat.yaml _datafiles/world/dogmud/quests/38-first_brew.yaml _datafiles/world/dogmud/quests/41-first_sign.yaml _datafiles/world/dogmud/quests/44-first_casting.yaml _datafiles/world/dogmud/quests/47-first_words.yaml _datafiles/world/dogmud/quests/50-first_shot.yaml
git commit -F - <<'EOF'
content(coulee): Crier Toke is the only starter of the seven trades

Vorn, Rusk, Birna, Tarn, Grieve, Wenna and Iden drop their duplicate
offers of their trade's first quest (keys retired in topic-keys.txt)
and send a newcomer to Toke instead; each keeps its hand-in. Clears
the last 7 chain 30 backlog lines.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 7: Headless walk of the whole spine

**Executor:** Opus

**Files:**
- Create: `coulee_spine_smoke_test.go` (repo root, package `main`)
- Modify: `docs/README.md`

The walkthrough (`questengine.Walk`, swept by `TestSmoke_ShippedQuestsPassTheLoop`) drives each quest from its offer to its hand-in on its own; `TestQuest28_WalksFromTheOfferToTheLanding` drives every quest 28 step through the engine. Neither chains quests. This test walks one brand-new character across every hand-off on shipped data, with the real dialogue gates and the real quest step order (`Character.HasQuest`), and stands in for each quest's own steps with `GiveQuestToken`.

- [ ] **Step 1: Write the test**

Create `coulee_spine_smoke_test.go`:

```go
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// TestSmoke_NewCharacterSpine walks a brand-new character through the
// Pothole Coulee opening on shipped data (#375 phase 7, plan 2): every quest
// is offered by the NPC who took the previous hand-in, nothing is granted by
// itself, and Crier Toke ends up the only starter of the seven trades. The
// steps inside each quest are the walkthrough's job
// (TestSmoke_ShippedQuestsPassTheLoop); here GiveQuestToken stands in for
// them, and this test holds the hand-offs between quests.
func TestSmoke_NewCharacterSpine(t *testing.T) {
	loadShippedQuestWorld(t)
	files, problems := dialogue.LoadAllFiles()
	if len(problems) > 0 {
		t.Fatalf("dialogue problems: %v", problems)
	}
	byMob := map[int]*dialogue.DialogueFile{}
	for _, df := range files {
		byMob[df.MobId] = df
	}

	c := characters.New()
	c.QuestProgress = map[int]string{}
	ps := &dialogue.PlayerState{
		HasQuest:     c.HasQuest,
		GiveQuest:    func(tok string) { c.GiveQuestToken(tok) },
		HasItem:      func(int) bool { return false },
		CarriesItem:  func(int) bool { return false },
		GiveItem:     func(int) bool { return true },
		GetQuestFlag: c.GetQuestFlag,
		SetQuestFlag: c.SetQuestFlag,
		HasHeard:     c.HasHeard,
		MarkHeard:    c.MarkHeard,
		QuestOpen:    func(id int) bool { return c.QuestOpen(quests.GetQuestById(id)) },
		Shelved:      c.IsShelved,
	}
	npc := func(mobId int) *dialogue.DialogueFile {
		t.Helper()
		df := byMob[mobId]
		if df == nil {
			t.Fatalf("no dialogue for mob %d", mobId)
		}
		return df
	}
	open := func(mobId int, key string) bool {
		t.Helper()
		df := npc(mobId)
		tp := df.FindTopic(key)
		if tp == nil {
			t.Fatalf("mob %d has no topic %s", mobId, key)
		}
		return df.Eligible(tp, ps)
	}
	take := func(mobId int, key, want string) {
		t.Helper()
		df := npc(mobId)
		tp := df.FindTopic(key)
		if tp == nil || !df.Eligible(tp, ps) {
			t.Fatalf("mob %d topic %s is not open at %v", mobId, key, c.QuestProgress)
		}
		if !df.Apply(tp, ps) {
			t.Fatalf("mob %d topic %s did not apply", mobId, key)
		}
		if !c.HasQuest(want) {
			t.Fatalf("mob %d topic %s: want %s, progress %v", mobId, key, want, c.QuestProgress)
		}
	}
	advance := func(tok string) {
		t.Helper()
		if !c.GiveQuestToken(tok) {
			t.Fatalf("could not move to %s from %v", tok, c.QuestProgress)
		}
	}
	marker := func(mobId int, want string) {
		t.Helper()
		if got := npc(mobId).Marker(ps); got != want {
			t.Fatalf("mob %d shows %q, want %q at %v", mobId, got, want, c.QuestProgress)
		}
	}

	const dewey, hadwen, toke = 9491, 9100, 9106

	// Waking to Gaius: Dewey offers it and takes it.
	marker(dewey, menu.MarkerQuest)
	take(dewey, "waking_offer", "28-start")
	advance("28-tripped")
	if open(dewey, "ready") {
		t.Fatal("Dewey's hand-in is open before the flee lesson")
	}
	advance("28-fled")
	marker(dewey, menu.MarkerHandIn)
	take(dewey, "ready", "28-end")

	// The Awakening: Hadwen offers it, the rite's tree grants rite and
	// woken, and Hadwen takes it.
	if open(hadwen, "road_toke") {
		t.Fatal("Toke's Road is offered before the rite")
	}
	marker(hadwen, menu.MarkerQuest)
	take(hadwen, "rite_offer", "30-start")
	advance("30-rite")
	advance("30-woken")
	if open(hadwen, "road_toke") {
		t.Fatal("Toke's Road is offered before the hand-in")
	}
	marker(hadwen, menu.MarkerHandIn)
	take(hadwen, "rite_report", "30-end")
	for _, key := range []string{"road_toke", "road_esk", "another_way"} {
		if !open(hadwen, key) {
			t.Fatalf("Hadwen's %s is not open after the hand-in", key)
		}
	}

	// Toke's Road, then Find Your Footing, both at Toke.
	if open(toke, "footing_offer") {
		t.Fatal("Find Your Footing is offered before Toke's Road is handed in")
	}
	crumbs := strings.Join(npc(toke).Breadcrumbs(ps), " ")
	if !strings.Contains(crumbs, "Hadwen") {
		t.Fatalf("Toke's breadcrumbs do not point at Hadwen: %q", crumbs)
	}
	take(hadwen, "road_toke", "88-start")
	take(toke, "road_toke_end", "88-end")
	take(toke, "footing_offer", "31-start")
	advance("31-lantern")
	take(toke, "footing_turnin", "31-end")

	// The seven trades: Toke offers each first quest; its first teacher
	// offers none and points back to Toke.
	trades := []struct {
		offer   string
		quest   int
		teacher int
	}{
		{"send_drill", 32, 9108}, {"send_forge", 35, 9116}, {"send_brew", 38, 9128},
		{"send_steppe", 41, 9136}, {"send_folding", 44, 9144}, {"send_folk", 47, 9153},
		{"send_range", 50, 9161},
	}
	marker(toke, menu.MarkerQuest)
	for _, tr := range trades {
		if !open(toke, tr.offer) {
			t.Errorf("Toke's %s is not open after footing", tr.offer)
		}
		start := fmt.Sprintf("%d-start", tr.quest)
		for _, tp := range npc(tr.teacher).Topics {
			if tp.GrantsQuest == start {
				t.Errorf("mob %d topic %s still grants %s: Toke is the only starter", tr.teacher, tp.Key, start)
			}
		}
		if text, _ := npc(tr.teacher).OpenerFor(ps); !strings.Contains(text, "Toke") {
			t.Errorf("mob %d's opener for an Opened newcomer does not send them to Toke: %q", tr.teacher, text)
		}
	}
}
```

- [ ] **Step 2: Run it and see it pass; prove it can fail**

```bash
DOGMUD_BOOT_SMOKE=1 go test -run TestSmoke_NewCharacterSpine -v -timeout 600s . 2>&1 | tail -5
```

Expected: `--- PASS: TestSmoke_NewCharacterSpine` and `ok`. Null probe: temporarily set Toke's `footing_offer` `questRequired` back to `- 30-end`, rerun, and see `Find Your Footing is offered before Toke's Road is handed in`; restore and rerun green.

- [ ] **Step 3: README row**

In `docs/README.md`, directly after the row whose text ends with `` `main_quests.go` is the boot door and `quest_loop_smoke_test.go` the CI sweep | ``, insert this row:

```
| [`../coulee_spine_smoke_test.go`](../coulee_spine_smoke_test.go) | Shipped-data smoke test (#375 phase 7, plan 2; runs with `DOGMUD_BOOT_SMOKE=1`): walks one brand-new character across every hand-off of the Pothole Coulee opening (Dewey's offer and hand-in of quest 28, Hadwen's of quest 30, Toke's Road, Find Your Footing) with the real dialogue gates and quest step order, then checks that Crier Toke is the only starter of the seven trades and each first teacher points back to him |
```

- [ ] **Step 4: Commit**

```bash
git add coulee_spine_smoke_test.go docs/README.md
git commit -F - <<'EOF'
test(coulee): walk a new character across every hand-off of the spine

One shipped-data smoke test takes a fresh character from Dewey's offer
to Toke's seven trade offers, checking each offer opens only after the
previous hand-in on that NPC and each first teacher points to Toke.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 8: Docs, skill and patch notes

**Executor:** Sonnet

**Files:**
- Modify: `internal/quests/context.md`, `internal/characters/context.md`, `internal/dialogue/context.md`, `internal/questloop/context.md`, `internal/usercommands/context.md`
- Modify: `internal/dialogue/offers.go` (comment)
- Modify: `internal/usercommands/questlog_test.go` (comment)
- Modify: `.claude/skills/dogmud-authoring-quests/SKILL.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: context.md updates**

`internal/quests/context.md`:
- in the `manifest.go` paragraph (it ends `` `# renamed` line. ``), append after that last sentence: `` `RetiredQuests()` reads the quests retired whole (every step id under `# retired`, none still a key); `users.LoadUser` hands them to `Character.DropRetiredQuests`, which clears them from progress and the shelf (#375 phase 7: quest 29 Two Roads). ``
- in the file table, replace `` | `manifest.go` | `ParseStepManifest`, `StepTokens`, `StepRenames`: the step-id manifest (`quests/step-ids.txt`) | `` with `` | `manifest.go` | `ParseStepManifest`, `StepTokens`, `StepRenames`, `StepRetired`, `RetiredQuests`: the step-id manifest (`quests/step-ids.txt`) | ``.

`internal/characters/context.md`: after the bullet that begins `` - `RenameQuestSteps(renames)` runs at load `` (ending `It renames `ShelvedQuests` the same way.`), add:

```markdown
- `DropRetiredQuests(retired) []int` runs at load (`users.LoadUser`, with
  `quests.RetiredQuests()`): a quest retired whole in `quests/step-ids.txt`
  is cleared from progress and the shelf through `ClearQuestToken`, which
  also moves the focus off it. Idempotent, no marker, so alts are safe
  (#375 phase 7: quest 29 Two Roads).
```

`internal/dialogue/context.md`: replace `` not a fault). Shipped 2026-10-28: 13 (excludes `12-start`), 88 (excludes `` and the next line `` `31-start`, `29-end`), 89 (excludes `29-end`). `` with `` not a fault). Shipped after #375 phase 7 plan 2: 13 (excludes `12-start`), 88 (excludes `` and `` `31-start`). Quest 29 is retired, so 89's offer now resumes. ``

`internal/questloop/context.md`: replace `` `end` finding (room 6467's tree grants `30-start` and `28-end`; Hadwen's `` and the next line `` grants `30-end`). One-action's `:prev`, `:cond`, `:seq` and `:double` are `` with `` `end` finding (before #375 phase 7, room 6467's tree granted `30-start` and `28-end`, and Hadwen's `30-end`; now his tree grants `30-rite` and `30-woken`, mid steps). One-action's `:prev`, `:cond`, `:seq` and `:double` are ``.

`internal/usercommands/context.md`: replace `` choosing to; quest 30, granted by `room_enter 5200` and room 6467's tree). `` with `` choosing to; quest 30 was the instance until #375 phase 7 removed its automatic starts). ``

`internal/dialogue/offers.go`: replace `// excludes 31-start and 29-end) the offer hides for good.` with `// excludes 31-start) the offer hides for good.`

`internal/usercommands/questlog_test.go`: replace `// On shipped data, quest 88's only offer (road_toke) excludes 31-start and` and the next line `// 29-end, so 88 is not abandonable; quest 42's offer excludes only its own` with `// On shipped data, quest 88's only offer (road_toke) excludes 31-start,` and `// so 88 is not abandonable; quest 42's offer excludes only its own`.

- [ ] **Step 2: Skill**

In `.claude/skills/dogmud-authoring-quests/SKILL.md`, rule 3 under "Abandon and resume": replace

```markdown
3. **Abandon needs an NPC offer.** A quest is abandonable only when some topic
   grants its `-start`. A quest started by a trigger (quest 28, started by
   `room_enter`) cannot be abandoned.
```

with

```markdown
3. **Abandon needs an NPC offer.** A quest is abandonable only when some topic
   grants its `-start`. A quest started only by a trigger cannot be
   abandoned. (Quest 28 was one until #375 phase 7 gave it Dewey's offer.)
```

and in rule 5 replace

```markdown
   unabandonable (owner ruling 6). Quest 30 is the instance: `room_enter 5200`
   and room 6467's antechamber tree grant `30-start`, though Hadwen offers it.
```

with

```markdown
   unabandonable (owner ruling 6). Quest 30 was the instance until #375 phase
   7 removed its `room_enter 5200` and room 6467 grants, leaving Hadwen's offer.
```

- [ ] **Step 3: Patch notes**

Run `date +%F` and use its output as `<DATE>`. Insert directly under `# DOGMud Patch Notes` (with one blank line before and after):

```markdown
## <DATE>: A clearer start in Pothole Coulee

- When you wake, Dewey offers your first quest, Waking to Gaius. Talk to
  him and accept it. At the end of the tutorial, talk to him again and
  choose it to finish, and he sends you up to the Awakening Pool.
- Cleric Hadwen offers The Awakening. When the rite is over, talk to him
  once more to finish it.
- Crier Toke now starts every trade in the coulee. Once you have walked
  the town with him, he can start you on fighting, smithing, brewing,
  tracking, casting, the lore of the folk and archery. Take as many as
  you like.
- The trade teachers send new folk to Toke.
- Ask Cleric Hadwen "Is there another way out?" to hear how to leave the
  coulee through Warden Esk's arch.
- Two Roads, the old signpost quest, is gone. If you still had it, it is
  cleared from your quest log.
- Waking to Gaius and The Awakening can now be set aside like any other
  quest.
```

- [ ] **Step 4: Check and commit**

```bash
go build ./...
go test ./internal/dialogue/ ./internal/usercommands/
git grep -nP '\x{2014}|\x{2013}| -- ' -- docs/PATCH_NOTES.md | head -3
```

Expected: build clean, `ok`; the grep prints only lines from older entries (none from the new entry; compare line numbers with the inserted block).

```bash
git add internal/quests/context.md internal/characters/context.md internal/dialogue/context.md internal/questloop/context.md internal/usercommands/context.md internal/dialogue/offers.go internal/usercommands/questlog_test.go .claude/skills/dogmud-authoring-quests/SKILL.md docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs(coulee): context, skill and patch notes for the spine

Package context.md files, the authoring-quests abandon examples and a
player-facing patch note follow the new opening: offers and hand-ins
at Dewey, Hadwen and Toke, quest 29 retired, quests 28 and 30
abandonable.

Part of #375 phase 7 (Pothole Coulee, plan 2).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 9: Gate

**Executor:** Sonnet (stop and report any failure; do not work around it)

**Files:** none (fixes go back to the task that owns the file)

- [ ] **Step 1: Format, build, packages**

```bash
cd C:/tmp/dogmud-375-p7-coulee-spine
gofmt -l internal/ modules/
go build ./...
go test ./internal/questengine/ ./internal/behaviortree/ ./internal/dialogue/ ./internal/questloop/ ./internal/characters/ ./internal/quests/ ./internal/users/ ./internal/usercommands/ ./internal/hooks/ ./internal/guards/...
```

Expected: `gofmt` prints nothing; build prints nothing; every package `ok`.

- [ ] **Step 2: Shipped-data loop and smoke tests**

```bash
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop|TestOfferIndexDoor' -timeout 900s .
```

Expected: `ok`. (Without `DOGMUD_BOOT_SMOKE=1` the sweeps skip and prove nothing.)

- [ ] **Step 3: Backlog arithmetic**

```bash
wc -l _datafiles/world/dogmud/quests/loop-backlog.txt
grep -E '^[a-z-]+ (2[89]|30|31|88) ' _datafiles/world/dogmud/quests/loop-backlog.txt
grep -E '^chain 30 ' _datafiles/world/dogmud/quests/loop-backlog.txt
grep -cE '^[a-z-]+ (3[2-9]|4[0-9]|5[0-9]|76|89) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

Run each `grep` on its own (an empty `grep` exits 1). Expected: the line count is the Task 0 baseline minus 28 (`268` from a 296 baseline); the next two greps print nothing; the count is `67`. Of those 67, `end 89 quest 89 trigger room_enter room=468 grants=89-end` is plan 4's; the other 66 (including quest 76's 8) are plans 3a and 3b's.

- [ ] **Step 4: Lint**

```bash
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `0 issues.`

- [ ] **Step 5: Isolated boot check**

```bash
S="C:/Users/CALABE~1/AppData/Local/Temp/claude/<your session scratchpad>"   # the Scratchpad directory your system prompt names, never $TMP
cd C:/tmp/dogmud-375-p7-coulee-spine
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > boot.log 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log
grep -c "Server Ready" boot.log
grep -nE "fresh|not in loop-backlog|offered but not abandonable" boot.log | head
```

Expected: `exit 124` (the timeout fired because the server stayed up); first count `0` (exits 1, which is fine); `Server Ready` count `1`; the "offered but not abandonable" log line, if present, no longer lists 30 among the auto-started quests. Then clean up:

```bash
cd C:/tmp/dogmud-375-p7-coulee-spine
git worktree remove --force C:/tmp/dogmud-boot-check
```

(If Windows holds a lock, use PowerShell `Remove-Item -Recurse -Force C:/tmp/dogmud-boot-check`, then `git worktree prune`.)

- [ ] **Step 6: Push and open the PR**

```bash
cd C:/tmp/dogmud-375-p7-coulee-spine
git status --short
git push -u origin feat/375-p7-coulee-spine
gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-coulee-spine --title "#375 phase 7 Coulee plan 2: the spine (Dewey, Hadwen, Toke)" --body-file - <<'EOF'
Plan 2 of the #375 phase 7 Pothole Coulee pass
(docs/superpowers/plans/2026-10-30-p7-coulee-2-spine.md). It runs after
plan 1 (engine).

What changes for a new player
- Dewey offers Waking to Gaius (28) and takes its hand-in; his tree
  sends the player to the pool. The 6258 greeting teaches talk and [!].
- Cleric Hadwen offers The Awakening (30) and takes its hand-in after the
  rite (new step `woken`). Nothing grants 28 or 30 by itself, so both can
  be set aside and resumed (phase 5 ruling 6, no code change).
- Toke's Road (88) ends only at Toke; Find Your Footing (31) gates on it.
- Toke is the only starter of the seven trades; each first teacher sends
  newcomers to him. Toke explains the hub in plain words.
- Hadwen's "Is there another way out?" points to Esk. road_esk stays until
  plan 4.
- Quest 29 (Two Roads) is retired; saves holding it are cleared at load.

#690 items fixed here: Dewey's farewell jargon, the trip prompt on a
prone effigy, and the double-hyphen lines in Dewey's and Hadwen's trees.

Backlog: 28 lines cleared (296 to 268). 67 lines remain for quests 32 to
59, 76 and 89: 66 for plans 3a and 3b, `end 89 ...room=468` for plan 4.

Checks run: gofmt, build, package tests, DOGMUD_BOOT_SMOKE=1 root sweeps
(including the new TestSmoke_NewCharacterSpine), golangci-lint 0 issues,
isolated boot check (exit 124, Server Ready).

Part of #375.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

Expected: `git status --short` shows nothing uncommitted; `gh` prints a URL that starts `https://github.com/pruuk/DOGMud/pull/`. If the URL names `GoMudEngine`, stop and report at once. The body uses no "closes", "fixes" or "resolves" before an issue number, so no issue auto-closes.

- [ ] **Step 7: Watch CI**

```bash
gh pr checks <number> --repo pruuk/DOGMud --watch
gh run list --repo pruuk/DOGMud --branch feat/375-p7-coulee-spine --limit 5
```

Expected: every check green, and `gh run list` shows the test workflow ran (not only the fast checks). Report the PR link; the owner merges and deploys.
