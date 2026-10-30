# #375 Phase 7 Coulee Plan 4: the Shadows Line and Leaving Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship PR 4 of the Pothole Coulee pass: the new Shadows line (Lamplighter Gethin, Cutpurse Dace, the Pocket Dummy, a practice lockbox, quests 97 to 101), quest 89 moved from Cleric Hadwen to Crier Toke behind all eight bridges, the Toke and Esk flavour lines, and the close-out of every Coulee line in `loop-backlog.txt`.

**Architecture:** Content is YAML under `_datafiles/world/dogmud/`. Three quest steps need actions the quest engine cannot hear today (`hood`, `picklock`, a strike from hiding), so each gets one `questengine.NotifyCommand` call at its success point, with a probe-quest test. The practice mark's purse is restocked by one new behaviour-tree action, `stock_item`. The flavour lines are per-mob behaviour trees (`player_enter`, `player_has_quest` x8, `random_chance`, a random-decorator cascade, `respond` with `user_text` only). A guard test keeps the zone's backlog lines at zero.

**Tech Stack:** Go 1.x, YAML content under `_datafiles/world/dogmud/`

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md

**Branch / worktree:** branch `feat/375-p7-coulee-shadows`, worktree `C:/tmp/dogmud-375-p7-coulee-shadows`, off `origin/master` after plans 1, 2, 3a and 3b have merged. Remove the worktree with `git worktree remove` once the PR merges.

---

## Facts verified against source (master `c4af0286b` tree, 2026-10-30)

| Fact | Source |
|---|---|
| `hood` raises no quest event; it sets `rec.Hooded` on the light slot's adjustable light records and ends with `lightnotice.Check` | `internal/usercommands/hood.go:42-74` |
| `picklock` raises no quest event. Two success paths: the keyring path (`sequenceMatches(keyring_sequence, sequence)`) and the pin game (`sequenceMatches(entered, sequence)`); a container lock id is `"%d-%s"` of room id and container name | `internal/usercommands/picklock.go:90,144-181,261-298` |
| Full list of `NotifyCommand` call sites: no `hood`, no `picklock`, no ambush | grep `NotifyCommand(` over `internal` (35 hits, includes `skill.skullduggery.sneak.go:125`) |
| `sneak` raises `command sneak` only when the player ends hidden (or already was) | `internal/usercommands/skill.skullduggery.sneak.go:53-58,99-126` |
| `steal` on a mob raises `command steal` (target mob, no item) BEFORE the contest, after the rank-2 check | `internal/actions/steal.go:258,266-272` |
| A successful theft from a mob moves one random non-`never_drops` carried item through `acquire.AcquireItemForChar(..., acquire.AcquireSteal)`, which raises `item_gain` only when it lands | `internal/actions/steal.go:330-332`; `internal/characters/inventory.go:165-176`; `internal/acquire/acquire.go` (`AcquireItemForChar`) |
| `plant` on a mob raises `command plant` (target mob, `WithItem`) BEFORE the contest | `internal/actions/plant.go:129-135` |
| No shipped quest listens for `command: steal`, `plant`, `picklock` or `hood` | grep `command: plant\|command: steal\|command: picklock\|command: hood` in `quests/` (0 hits) |
| Theft immunity: `CheckPlayerHarm` blocks non-combatants and `player_attack_immune` mobs; a caught thief is attacked only by a mob that is neither | `internal/mobs/harm_authorization.go:41-55`; `internal/actions/steal.go:654-661` |
| A caught theft is a crime only against a mob whose group is a faction; no faction file is named `construct` or `coulee_folk` | `internal/factions/factions.go:150-161`; `ls _datafiles/world/dogmud/factions` (22 files, grep for those names: 0) |
| A mob's carried items always drop on death (100%, per-item `dropchance` aside) | `internal/hooks/Death_MobLoot.go:41-50` |
| Nothing refills a room container's items | `internal/rooms/container.go:8-15` (no restock field) |
| A picked lock relocks by itself: `IsLocked` is true again once `relockinterval` has passed since `UnlockedRound`; the combination rotates only on `SetLocked` | `internal/gamelock/gamelock.go:24-55` |
| `relockinterval` accepts `N real minutes` | `internal/gametime/gametime.go:406,444-449,524-531`; shipped `thornwall_city/487.yaml:25` (`15 real minutes`) |
| A lock of difficulty 3 has 3 pins (`GetLockSequence` clamps to [2..32]) | `internal/util/util.go:494-501` |
| A surprise melee round carries `AttackResult.WasSurpriseAttack` into `applyCombatProgression` | `internal/combat/combat.go:396-397`; `internal/hooks/NewRound_DoCombat_unified.go:638-643` |
| A same-room shot from hiding sets `FireResult.Revealed`; `shoot.go` raises `fire` with `TargetActor(user, result.TargetUserId, result.TargetMobInstanceId)` | `internal/actions/combat_fire.go:38-42,374,426`; `internal/usercommands/shoot.go:252-253` |
| `asUser` panics on a non-player actor; `questengine` is not yet imported by `NewRound_DoCombat_unified.go` | `internal/hooks/NewRound_DoCombat_unified.go:3-26,1102-1107` |
| A per-mob tree attaches by path only: `behaviors/<zone folder>/<mobId>-<ConvertForFilename(name)>.yaml`; there is no mob YAML field. A per-mob tree that fails hands the event to the archetype | `internal/behaviortree/helpers.go:51-57,152-169,228-285` |
| `respond` sends `user_text` to the player and `room_text` to the room only when non-empty | `internal/behaviortree/actions_dialogue.go:18-54` |
| `random_chance {percent}` condition; `decorator mod: random percent:` runs its child on `Rand(100) < percent`; Barmaid Dal picks one of four lines with a 25 / 33 / 50 / last cascade | `internal/behaviortree/conditions_state.go:222-234`; `decorators.go:65-70`; `behaviors/thornwall_city/117-barmaid_dal.yaml:59-108` |
| `player_has_quest` is `Character.HasQuest(token)`, true for the current step and every earlier one | `internal/behaviortree/conditions_player.go:9-22`; `internal/characters/quests.go:67-89` |
| `noncombat_questgiver` archetype emotes on `player_enter`; Toke has no per-mob tree today; Esk's tree has one `player_topic` branch | `behaviors/archetypes/noncombat_questgiver.yaml:21-24`; `ls behaviors/pothole_coulee` (9100, 9107, 9158); `behaviors/pothole_coulee/9107-warden_esk.yaml:16-64` |
| No behaviour action gives a mob an item; `StoreItem` outside `internal/acquire` fails a guard | `internal/behaviortree/actions.go:26-133`; `internal/guards/acquire_seam_guard_test.go:44-77` |
| Quest file name must be `<id>-<ConvertForFilename(name)>.yaml` or boot panics | `internal/quests/quests.go:327-330`; `internal/fileloader/fileloader.go:102-103`; `internal/util/util.go:920-937` |
| Quest events: `room_enter, mob_death, command, item_gain, topic_heard` and the rest | `internal/quests/quests.go:198-201` |
| Offer/hand-in label must equal the quest title; offer grants `-start`, hand-in grants `-end`; a grant must sit in `questExcluded` with the end token | `internal/dialogue/world.go:151-174,262-268` |
| Labels max 50 characters, `playerLine` (menu Detail) max 70; no semicolons in spoken text | `internal/menu/lint.go:10-11`; `internal/dialogue/world.go:92-96,186-189` |
| Step hints max 200 runes | `internal/questloop/rules_loop.go:60` |
| Breadcrumbs are spoken after the opener for every gated offer the player does not hold | `internal/dialogue/topics.go:134-152` |
| `itemExcluded` hides a topic from a player carrying the item, worn included | `internal/dialogue/item_excluded_test.go:9-14` |
| `count:` needs a `has` on its own step, a `missing` it grants, a grant; `label` max 40 | `internal/quests/quests.go:85-140`; skill `dogmud-authoring-quests` |
| A mob's carried item is an item source for the walkthrough when a room spawns the mob | `internal/questworld/sources.go:111-136` |
| The walkthrough checks a `mob_death` mob only when `mob > 0` | `internal/questengine/walkthrough.go:925-941` |
| `mob_death` reaches every player with kill credit, matched in the template room | `internal/hooks/MobDeath_QuestNotify.go:15-31` |
| Every Coulee NPC (non-hostile and non_combatant, shop or dialogue) must wear a `never_drops` light in the light slot | `internal/guards/coulee_npc_light_guard_test.go:30-61`; Keeper's Lantern `items/armor-20000/light/20134-keepers_lantern.yaml` |
| Mine: 5255 Mine Head (cave, `skylight: 0.50`) spawns Mine Crawler 9120; 5256 Timbered Drift (cave) spawns 9120 and Tunnel Brute 9121; 5257 spawns 9121; 5259 and 5261 spawn nothing. 9120 statpool 60, 9121 statpool 90, both hostile | `rooms/pothole_coulee/5255.yaml:18-19,40-43`, `5256.yaml:16,88-94`, `5257.yaml:38-40`, `5259.yaml`, `5261.yaml`; `mobs/pothole_coulee/9120*`, `9121*` |
| Route Hub Square to Mine Head: north, west x5 (Smithy 5245), north (5247), west x5 (Mine Mouth 5254), down. 5249, 5250 spawn Scree Scavenger 9118 (statpool 18, hostile), 5253 spawns 9118 and Talus Lurker 9119 (40, hostile) | BFS over `rooms/pothole_coulee/*.yaml` exits (scratch script); mob files |
| Route Hub Square to Reed Jetty 5218: north, west, north. Back: south, east, south. Hub Square to Threshold House 5215: south, south | same BFS |
| Toke stands at Hub Square 5203 at every hour | `schedules/pothole_coulee/pothole_toke.yaml` |
| Trader Onna sells oil lantern 40038, torch 20096, hooded lantern 20097 | `mobs/pothole_coulee/9104-trader_onna.yaml:57-59` |
| Hooded Lantern 20097: light, value 20, worn condition 127; Iron Lockpicks 33: type lockpicks, value 5, uses 3 | `items/armor-20000/light/20097-hooded_lantern.yaml`; `items/other-0/33-iron_lockpicks.yaml` |
| Fence Dealer Siv (Thornwall) sells iron lockpicks 33 and basic disarm kit 36; Jeweler Vurl (9607) crafts `master-lockpicks` and `precision-disarm-kit` | `mobs/thornwall_city/104-fence_dealer_siv.yaml:55,59`; `mobs/pothole_coulee/9607-jeweler_vurl.yaml:24-37` |
| `shadow` and `defuse` need skullduggery rank 3 | `internal/usercommands/skill.skullduggery.shadow.go:31`; `internal/actions/defuse.go:65` |
| `plant <item> on <target>` parses | `internal/usercommands/skill.skullduggery.plant.go:59-82` |
| Repeatables 53 to 59: `cooldown_rounds: 300`, reward `gold: 6` plus one skill rank | `quests/53-*.yaml` to `59-*.yaml` (grep) |
| Line quest 1 rewards `gold: 15` plus a skill rank | `quests/32,35,38,41,44,50-*.yaml` (grep) |
| Quest 89: offered by Hadwen's `road_esk` (`dialogue/pothole_coulee/9100.yaml:117-134`); hand-in `road_esk_end` on Esk (`9107.yaml:30-43`); finisher `room_enter 468` (`quests/89-esks_road.yaml:39-47`); backlog lines `chain 30 mob 9100 topic road_esk` (line 7) and `end 89 quest 89 trigger room_enter room=468 grants=89-end` (line 110) | files named |
| Esk's `passage` topic needs only `30-end`, and so does the tree branch that runs the crossing | `dialogue/pothole_coulee/9107.yaml:46-53`; `behaviors/pothole_coulee/9107-warden_esk.yaml:21-29` |
| `TestHadwenMidRite` asserts Hadwen's menu contains `road_esk` after the rite | `internal/dialogue/hadwen_midrite_test.go:62` |
| `TestTokeGivesTheWay` checks Toke's route topics by key and hint | `internal/dialogue/toke_ways_test.go:18-40` |
| Neither manifest has a `# retired` section on master; the parser accepts keys, then `# retired`, then `# renamed` | `dialogue/topic-keys.txt`; `quests/step-ids.txt`; `internal/manifest/manifest.go:25-72` |
| Free ids: mobs 9125, 9126, 9127 (grep -w over all YAML and Go: 0 hits; control 9123: 3 hits); item 60 (no `itemid: 60`); quests 97 to 101; area key `coulee_mine` | `tools/id_inventory.py`; greps |
| Name checks: `Gethin`, `Dace`, `Pocket Dummy`, `Sand Purse` clash with no mob, item or novel name. Rejected: `Tove` (Tova exists), `Ilse` (Apothecary Ilsa), `token` (four other token items would catch `plant token`) | mob name list (642 names), `novel/`, `docs/world.md` (control: `Hob` found) |
| `steal_quest_event_test.go` uses quest id 98 as a probe in an isolated engine | `internal/actions/steal_quest_event_test.go:20-21` |

## Spec deviations found

1. **Bridge file name.** The common brief names bridge files `quests/90-report_to_toke_fighting.yaml`. The loader panics unless the file is `<id>-<ConvertForFilename(name)>.yaml`; "Report to Toke (Shadows)" gives `97-report_to_toke__shadows_.yaml`. Task 0 reads how plan 3 named 90 to 96 and this plan follows it.
2. **Engine changes ship in PR 4, not PR 1.** The `hood`, `picklock` and ambush quest events and the `stock_item` action are used only by this PR's content, so they ship with it (the brief asks for them here).
3. **The lantern.** The spec says the lamplighter "hands over a hooded lantern through givesItem"; the brief says "via the offer's givesItem", and the offer is Toke's. This plan has Gethin hand it over from a talk topic (`givesItem`) that is also the "go see the lamplighter" step (`topic_heard`). The lockpicks follow the spec literally: Gethin's offer of 99 carries `givesItem: 33`.
4. **Two extra steps.** 98 gains `equip` (hood refuses a lantern that is not in the light slot, `hood.go:20-23`). 100 gains `hidden` (sneak) before the strike, the one-action split quest 50 already uses.
5. **The lockbox holds no token.** Nothing refills a room container, so a token would be gone after the first student. The step completes on the pick.
6. **The practice mark restocks itself.** A theft never refills a mob and a mob restocks only by dying. The Pocket Dummy's tree calls the new `stock_item` action on idle.
7. **Steal and plant fire on the attempt.** Both raise their command event before the contest. The steal step listens on `item_gain` of the purse instead (success only). The plant step listens on `command: plant` and so counts a caught attempt; its copy says so. Owner ruling 2026-10-17 ("a step advances only when the action succeeds") was given for craft, sleep and search. **Decision for the owner at plan review:** keep this, or widen Task 1 to move all three `plant` notifies after success (three sites in `plant.go`, three tests in `steal_quest_event_test.go`).
8. **The ambush event fires on the opener, landed or not** (the opener is spent either way, `templates/help/ambush.template:20-23`). Gating it on a clean hit is a one-line change; same review.
9. **Mine spawns moved.** A hostile crawler at the Mine Head would attack students at Gethin's post, and a crawler plus brute would meet a quest-1 student in the first dark room. They move to 5259 and 5261; 5257's brute stays as quest 100's target. The road to the Mine Head still crosses three hostile talus rooms; the offer and step text warn, and the playtest (plan 5) judges it.
10. **Sneak step has no room filter.** Dace and the dummy watch 5218 and may spot a student, so the lesson lets them hide anywhere.
11. **Flavour lines:** four on Toke and four on Esk, one per Giver 2 across the pair, rather than all eight on each.
12. **Close-out is a guard test** (`internal/guards/coulee_backlog_guard_test.go`), not only a grep, so the ruling stays enforced.

## Dependencies on plans 1 to 3 (checked in Task 0)

- Plan 1: `StealMinRank` shipped at 1 (otherwise a rank-1 student is refused at `steal.go:258`).
- Plan 2: Toke is the only line starter; Hadwen has the "another way out" info topic; quest 29 retired.
- Plans 3a and 3b: quests 90 to 96 exist, Toke (9106) holds `report_<line>_in` hand-ins for them; the Coulee dash guard exists (every file this plan writes is dash-free, comments included).

## File map

| Path | Create / Modify | Responsibility |
|---|---|---|
| `internal/usercommands/hood.go` | Modify | Raise `command hood` (item = the lantern) after a hood closes |
| `internal/usercommands/picklock.go` | Modify | Raise `command picklock` on both success paths |
| `internal/usercommands/shoot.go` | Modify | Raise `command ambush` after a shot from cover |
| `internal/hooks/NewRound_DoCombat_unified.go` | Modify | Raise `command ambush` for a player's surprise melee round |
| `internal/behaviortree/actions_quest.go`, `actions.go` | Modify | New `stock_item` action and its registration |
| `internal/usercommands/hood_quest_test.go`, `picklock_quest_test.go`, `shoot_ambush_test.go` | Create | Probe-quest tests for the three events |
| `internal/hooks/ambush_quest_notify_test.go` | Create | Melee ambush event test |
| `internal/behaviortree/actions_stock_test.go`, `coulee_flavour_trees_test.go` | Create | `stock_item` tests; Toke and Esk tree tests |
| `internal/dialogue/esk_road_test.go` | Create | Esk's passage with and without 89; Toke's gate on 89 |
| `internal/guards/coulee_backlog_guard_test.go` | Create | No backlog line names a Coulee quest |
| `_datafiles/world/dogmud/items/other-0/60-sand_purse.yaml` | Create | The practice purse |
| `_datafiles/world/dogmud/areas/coulee_mine.yaml`, `areas/area-keys.txt` | Create / Modify | The mine as an area |
| `_datafiles/world/dogmud/mobs/pothole_coulee/9125-lamplighter_gethin.yaml`, `9126-cutpurse_dace.yaml`, `9127-pocket_dummy.yaml` | Create | Giver 1, Giver 2, practice mark |
| `_datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml` | Create | Purse restock on idle |
| `_datafiles/world/dogmud/behaviors/pothole_coulee/9106-crier_toke.yaml` | Create | Toke's flavour lines |
| `_datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml` | Modify | Esk's flavour branch |
| `_datafiles/world/dogmud/rooms/pothole_coulee/5218.yaml`, `5255.yaml`, `5256.yaml`, `5259.yaml`, `5261.yaml` | Modify | Jetty spawns and lockbox; mine spawns |
| `_datafiles/world/dogmud/quests/97-report_to_toke__shadows_.yaml`, `98-first_light.yaml`, `99-quiet_hands.yaml`, `100-from_the_dark.yaml`, `101-quick_fingers.yaml` | Create | The Shadows line |
| `_datafiles/world/dogmud/quests/89-esks_road.yaml` | Modify (rewrite) | Toke offers, Esk takes the hand-in, no arch finisher |
| `_datafiles/world/dogmud/quests/step-ids.txt`, `loop-backlog.txt` | Modify | New step ids; delete cleared lines |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9125.yaml`, `9126.yaml` | Create | Gethin's and Dace's topics |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`, `9100.yaml`, `9107.yaml`, `dialogue/topic-keys.txt` | Modify | Toke's new topics; Hadwen loses `road_esk`; Esk's hand-in copy; manifest |
| `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml` | Modify (if needed) | Drop "either road" wording |
| `internal/dialogue/hadwen_midrite_test.go`, `toke_ways_test.go` | Modify | Follow the moved offer; the new route topic |
| `internal/questengine/context.md`, `internal/behaviortree/context.md`, `internal/hooks/context.md` | Modify | Document the events and the action |
| `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/PATCH_NOTES.md`, `docs/README.md` | Modify | Status, patch notes, plan row |

---

### Task 0: Preflight (Sonnet)

**Files:** none changed.

- [ ] **Step 1: Create the worktree**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/375-p7-coulee-shadows C:/tmp/dogmud-375-p7-coulee-shadows origin/master
cd C:/tmp/dogmud-375-p7-coulee-shadows
```

- [ ] **Step 2: Confirm plans 1 to 3 merged.** Run each; every one must print what is described, or STOP and report to the controller.

```bash
grep -n "StealMinRank" internal/configs/config.balance.go          # a field line
grep -n -i "stealminrank" _datafiles/config.yaml                   # a line ending in 1
ls _datafiles/world/dogmud/quests/ | grep -E '^9[0-6]-'            # seven bridge files
grep -n "key: report_.*_in" _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml   # seven hand-ins
grep -n "key: road_esk" _datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml       # still present
```

- [ ] **Step 3: Read the bridge naming plan 3 used.** `head -20 _datafiles/world/dogmud/quests/90-*.yaml`. Note the `name:` form and the file name. If the name is `Report to Toke (Fighting)` and the file is `90-report_to_toke__fighting_.yaml`, use `Report to Toke (Shadows)` and `97-report_to_toke__shadows_.yaml` as this plan does. If plan 3 used another form, apply the same form to 97 everywhere this plan writes the title (quest `name:`, Dace's `report_shadows_offer` label, Toke's `report_shadows_in` label) and recompute the file name as `97-` + ConvertForFilename(name) (lowercase; keep a to z and 0 to 9; drop apostrophes; every other character becomes `_`).

- [ ] **Step 4: Confirm ids are still free.** Each must print 0 (and the control must not):

```bash
for id in 9125 9126 9127; do grep -rlw "$id" _datafiles/world/dogmud --include=*.yaml | wc -l; done
grep -rlw 9123 _datafiles/world/dogmud --include=*.yaml | wc -l          # control: not 0
grep -rlE '^itemid: 60$' _datafiles/world/dogmud/items | wc -l
ls _datafiles/world/dogmud/quests | grep -cE '^(97|98|99|100|101)-'
python tools/id_inventory.py --zone pothole_coulee
```

If an id is taken, pick the next free one with `id_inventory.py` and replace it in every block of this plan before starting (mob ids appear in mob, room, dialogue, quest, tree and test blocks).

- [ ] **Step 5: Read the files this plan edits as they stand now** (plans 2 and 3 changed them): `dialogue/pothole_coulee/9100.yaml`, `9106.yaml`, `9107.yaml`, `behaviors/pothole_coulee/9107-warden_esk.yaml`, `9100-cleric_hadwen.yaml`, `quests/89-esks_road.yaml`, `quests/loop-backlog.txt` (lines naming 30, 88, 89), the tails of `quests/step-ids.txt` and `dialogue/topic-keys.txt`, and `internal/guards/esk_return_portal_test.go`. Read in chunks of at most 400 lines. Nothing to commit.

---

### Task 1: `hood` and `picklock` raise quest events (Opus)

**Files:**
- Modify: `internal/usercommands/hood.go`, `internal/usercommands/picklock.go`
- Create: `internal/usercommands/hood_quest_test.go`, `internal/usercommands/picklock_quest_test.go`

- [ ] **Step 1: Write the failing hood test**

`internal/usercommands/hood_quest_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 98, First Light): closing a lantern's hood is a quest
// step, so hood raises a `command` event naming the lantern as its item.
// A probe quest in an isolated engine holds the only trigger.
const hoodProbeQuest = 99702

func registerHoodProbe(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: hoodProbeQuest,
		Name:    "Hood Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "hood",
			Item:    hoodTestLanternItem,
			Actions: []quests.ActionDef{{Grant: "99702-start"}},
		}},
	})
}

func TestHood_RaisesTheHoodQuestEvent(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	registerHoodProbe(t)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest("99702-start"),
		"closing the hood must raise command hood with the lantern as its item")
}

func TestHood_RefusalRaisesNoQuestEvent(t *testing.T) {
	user, room := hoodFixture(t)
	registerHoodProbe(t)

	_, err := Hood("", user, room, 0) // no light worn: refused
	require.NoError(t, err)

	require.False(t, user.Character.HasQuest("99702-start"),
		"a refused hood must raise nothing")
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/usercommands/ -run 'TestHood_RaisesTheHoodQuestEvent|TestHood_RefusalRaisesNoQuestEvent' -count=1`
Expected: `TestHood_RaisesTheHoodQuestEvent` FAILS with "closing the hood must raise command hood"; the refusal test passes.

- [ ] **Step 3: Implement the hood notify.** In `internal/usercommands/hood.go` add `"github.com/GoMudEngine/GoMud/internal/questengine"` to the imports (alphabetical, after `messaging`). In `Hood`, replace:

```go
	lightnotice.Check(user, lightnotice.TriggerCommand)
	return true, nil
}

// Unhood opens the hood at full strength.
```

with:

```go
	lightnotice.Check(user, lightnotice.TriggerCommand)
	// #375 phase 7 (quest 98): a closed hood is a quest step. Sent after the
	// hood line and the band notice, so the step's line reads last.
	if room != nil {
		questengine.NotifyCommand(user, room.RoomId, "hood",
			questengine.TargetItem(user.Character.Equipment.Light.ItemId))
	}
	return true, nil
}

// Unhood opens the hood at full strength.
```

(The comment line `// Unhood opens the hood at full strength.` is quoted only to make the match unique; keep it and the line after it unchanged.)

- [ ] **Step 4: Run the hood tests, then prove the test can fail.** Run the Step 2 command: both PASS. Then comment out the `questengine.NotifyCommand(...)` call, rerun, confirm `TestHood_RaisesTheHoodQuestEvent` fails naming "closing the hood", restore the call, rerun green.

- [ ] **Step 5: Write the failing picklock tests**

`internal/usercommands/picklock_quest_test.go`:

```go
package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 99, Quiet Hands): picking a lock is a quest step, so
// picklock raises a `command` event when a lock opens, aimed at the room
// container by name. The keyring path is driven here; the pin game reaches
// the same helper.
const (
	picklockProbeQuest = 99703
	picklockTestPicks  = 999962
)

func picklockFixture(t *testing.T) (*users.UserRecord, *rooms.Room, string) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		picklockTestPicks: {ItemId: picklockTestPicks, Name: "test lockpicks", Type: items.Lockpicks, Uses: 3, Weight: 0.1},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.Stats.Strength.ValueAdj = 100
	_ = user.Character.GetSkillLevel(skills.Skullduggery) // allocates Skills
	user.Character.Skills[string(skills.Skullduggery)] = 1
	require.True(t, acquire.AcquireItem(user, items.New(picklockTestPicks), acquire.AcquireGive))

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	before := room.Containers
	room.Containers = map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 3}}}
	t.Cleanup(func() { room.Containers = before })

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: picklockProbeQuest,
		Name:    "Picklock Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "picklock",
			Target:  quests.TargetContainer,
			Noun:    "lockbox",
			Actions: []quests.ActionDef{{Grant: "99703-start"}},
		}},
	})
	return user, room, fmt.Sprintf("%d-%s", room.RoomId, "lockbox")
}

func TestPicklock_OpenedLockRaisesTheQuestEvent(t *testing.T) {
	user, room, lockId := picklockFixture(t)
	user.Character.SetKey(lockId, util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0))

	_, err := Picklock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest("99703-start"),
		"an opened container lock must raise command picklock aimed at the container")
}

func TestPicklock_UnfinishedPickRaisesNoQuestEvent(t *testing.T) {
	user, room, _ := picklockFixture(t)

	_, err := Picklock("lockbox", user, room, 0) // no keyring: the pin game only starts
	require.NoError(t, err)
	user.ClearPrompt()

	require.False(t, user.Character.HasQuest("99703-start"),
		"a pick that has not opened the lock must raise nothing")
}
```

- [ ] **Step 6: Run and see the first fail.** `go test ./internal/usercommands/ -run 'TestPicklock_' -count=1`. Expected: `TestPicklock_OpenedLockRaisesTheQuestEvent` FAILS ("an opened container lock must raise"); the second passes.

- [ ] **Step 7: Implement the picklock notify.** In `internal/usercommands/picklock.go` add `"github.com/GoMudEngine/GoMud/internal/questengine"` to the imports (after `messaging`). Add this function after `sequenceMatches` at the end of the file:

```go
// notifyPicklockQuest raises the `command` quest event for a lock just
// opened (#375 phase 7, quest 99): a container lock is aimed at the
// container by name (`target: container`, `noun:` on a trigger); an exit
// lock is aimed at nothing, with the exit's name as its noun. Called from
// both success paths, the keyring and the pin game, after the lock opened.
func notifyPicklockQuest(user *users.UserRecord, room *rooms.Room, containerName string, exitName string) {
	target := questengine.TargetContainer(containerName)
	if containerName == `` {
		target = questengine.NoTarget().WithNoun(exitName)
	}
	questengine.NotifyCommand(user, room.RoomId, "picklock", target)
}
```

Then in the keyring path replace:

```go
			room.SetExitLock(exitName, false)

		}
		return true, nil
	}
```

with:

```go
			room.SetExitLock(exitName, false)

		}
		notifyPicklockQuest(user, room, containerName, exitName)
		return true, nil
	}
```

and in the pin-game path replace:

```go
			room.SetExitLock(exitName, false)
		}

		user.ClearPrompt()
```

with:

```go
			room.SetExitLock(exitName, false)
		}
		notifyPicklockQuest(user, room, containerName, exitName)

		user.ClearPrompt()
```

- [ ] **Step 8: Run green, then prove it can fail.** Rerun Step 6: both PASS. Comment out the keyring-path `notifyPicklockQuest` call, confirm the first test fails naming "an opened container lock", restore, rerun green.

- [ ] **Step 9: Package tests and commit**

```bash
gofmt -l internal/usercommands/
go test ./internal/usercommands/ -count=1
git add internal/usercommands/hood.go internal/usercommands/picklock.go internal/usercommands/hood_quest_test.go internal/usercommands/picklock_quest_test.go
git commit -F - <<'EOF'
feat(quests): hood and picklock raise quest command events

#375 phase 7, Pothole Coulee plan 4. Quest 98 asks the player to close a
lantern's hood and quest 99 to pick a lock, and neither command told the
quest engine anything. hood now raises command hood with the lantern as its
item after the hood closes; picklock raises command picklock on both success
paths, aimed at the container by name (an exit lock carries the exit name
as its noun). Refusals and unfinished picks raise nothing. Probe-quest tests
for each, proven red with the call removed.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: a strike from hiding raises `command ambush` (Opus)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go`, `internal/usercommands/shoot.go`
- Create: `internal/hooks/ambush_quest_notify_test.go`, `internal/usercommands/shoot_ambush_test.go`

- [ ] **Step 1: Write the failing melee test**

`internal/hooks/ambush_quest_notify_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 100, From the Dark): a player's opening strike from
// hiding is a quest step. The round carries the fact on
// AttackResult.WasSurpriseAttack (see surpriseCandidate), so the melee path
// raises `command ambush` from applyCombatProgression, aimed at the defender.
const ambushProbeQuest = 99701

func registerAmbushProbe(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: ambushProbeQuest,
		Name:    "Ambush Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "ambush",
			Target:  quests.TargetMob,
			Actions: []quests.ActionDef{{Grant: "99701-start"}},
		}},
	})
}

func TestSurpriseRound_RaisesTheAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.True(t, u.Character.HasQuest("99701-start"),
		"a player's surprise round must raise command ambush aimed at the mob")
}

func TestOrdinaryRound_RaisesNoAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.False(t, u.Character.HasQuest("99701-start"),
		"an ordinary round is no ambush")
}
```

- [ ] **Step 2: Run it and see it fail.** `go test ./internal/hooks/ -run 'TestSurpriseRound_RaisesTheAmbushQuestEvent|TestOrdinaryRound_RaisesNoAmbushQuestEvent' -count=1`. Expected: the first FAILS ("a player's surprise round must raise"); the second passes.

- [ ] **Step 3: Implement the melee notify.** In `internal/hooks/NewRound_DoCombat_unified.go` add `"github.com/GoMudEngine/GoMud/internal/questengine"` to the imports (after `progression`). In `applyCombatProgression` replace:

```go
	atkUid := atk.GetUserId()
	defUid := def.GetUserId()

	// Cancel sleeping / cancel-on-damage conditions for any defender that took
```

with:

```go
	atkUid := atk.GetUserId()
	defUid := def.GetUserId()

	// #375 phase 7 (quest 100, From the Dark): a player's opening strike from
	// hiding is the `ambush` quest event, landed or not (the opener is spent
	// either way). WasSurpriseAttack is the only place the fact survives to
	// this phase; see surpriseCandidate.
	if atk.IsPlayer() && res.WasSurpriseAttack {
		notifyAmbushQuest(atk, def)
	}

	// Cancel sleeping / cancel-on-damage conditions for any defender that took
```

and add after the `asUser` function:

```go
// notifyAmbushQuest raises the `command ambush` quest event for a player's
// opening strike from hiding, aimed at the defender (#375 phase 7). Call it
// only for a player attacker.
func notifyAmbushQuest(atk, def actions.Actor) {
	room := atk.GetRoom()
	if room == nil {
		return
	}
	questengine.NotifyCommand(asUser(atk), room.RoomId, "ambush",
		questengine.TargetActor(atk.GetUserId(), def.GetUserId(), def.GetMobInstanceId()))
}
```

- [ ] **Step 4: Run green, then prove it can fail.** Rerun Step 2: both PASS. Remove `&& res.WasSurpriseAttack`, confirm `TestOrdinaryRound_RaisesNoAmbushQuestEvent` fails; restore. Comment out the `notifyAmbushQuest` call, confirm the first test fails; restore; rerun green.

- [ ] **Step 5: Write the failing ranged test**

`internal/usercommands/shoot_ambush_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 100): a same-room shot from hiding is an ambush
// (help ambush: "a hidden shot is always an ambush"), and ExecuteFire marks
// it FireResult.Revealed. ambushQuestTargets turns that into the targets of
// the `command ambush` event; any other shot raises none.
func TestAmbushQuestTargets_OnlyAShotFromCover(t *testing.T) {
	const instId = 99704
	mobs.SetInstanceForTest(instId, &mobs.Mob{MobId: 9121, InstanceId: instId})
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	targets, ok := ambushQuestTargets(1, actions.FireResult{Revealed: true, TargetMobInstanceId: instId})
	require.True(t, ok, "a shot from cover is an ambush")
	require.Len(t, targets, 1)
	require.Equal(t, quests.TargetMob, targets[0].Kind)
	require.Equal(t, 9121, targets[0].MobId)

	_, ok = ambushQuestTargets(1, actions.FireResult{TargetMobInstanceId: instId})
	require.False(t, ok, "an ordinary shot is no ambush")
}
```

- [ ] **Step 6: Run it and see it fail to compile.** `go test ./internal/usercommands/ -run TestAmbushQuestTargets -count=1`. Expected: `undefined: ambushQuestTargets`.

- [ ] **Step 7: Implement the ranged notify.** In `internal/usercommands/shoot.go` add after the `questNotifyCommand` const block:

```go
// ambushQuestTargets is the `command ambush` quest event a shot raises (#375
// phase 7, quest 100): a same-room shot from hiding (FireResult.Revealed) is
// an ambush aimed at what it was fired at. Any other shot raises none.
func ambushQuestTargets(userId int, result actions.FireResult) ([]questengine.Target, bool) {
	if !result.Revealed {
		return nil, false
	}
	return []questengine.Target{questengine.TargetActor(userId, result.TargetUserId, result.TargetMobInstanceId)}, true
}
```

and replace:

```go
	questengine.NotifyCommand(user, room.RoomId, questNotifyCommand,
		questengine.TargetActor(user.UserId, result.TargetUserId, result.TargetMobInstanceId))
```

with:

```go
	questengine.NotifyCommand(user, room.RoomId, questNotifyCommand,
		questengine.TargetActor(user.UserId, result.TargetUserId, result.TargetMobInstanceId))
	if targets, ok := ambushQuestTargets(user.UserId, result); ok {
		questengine.NotifyCommand(user, room.RoomId, "ambush", targets...)
	}
```

- [ ] **Step 8: Run green, prove it can fail, run packages, commit.** Rerun Step 6: PASS. Change `if !result.Revealed` to `if result.Revealed`, confirm both assertions fail, restore.

```bash
gofmt -l internal/hooks/ internal/usercommands/
go test ./internal/hooks/ ./internal/usercommands/ -count=1
go test . ./internal/guards/ -count=1
git add internal/hooks/NewRound_DoCombat_unified.go internal/hooks/ambush_quest_notify_test.go internal/usercommands/shoot.go internal/usercommands/shoot_ambush_test.go
git commit -F - <<'EOF'
feat(quests): a strike from hiding raises command ambush

#375 phase 7, Pothole Coulee plan 4. Quest 100 asks for an ambush, and no
event proved one. A player's surprise melee round (AttackResult.
WasSurpriseAttack in applyCombatProgression) and a same-room shot from cover
(FireResult.Revealed) now raise command ambush aimed at the target, once,
landed or not, since the opener is spent either way. Tests drive the melee
progression phase and the shot helper, each proven red.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: the `stock_item` behaviour action (Opus)

**Files:**
- Modify: `internal/behaviortree/actions_quest.go`, `internal/behaviortree/actions.go`
- Create: `internal/behaviortree/actions_stock_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/behaviortree/actions_stock_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// stock_item keeps the acting mob carrying an item (#375 phase 7): the
// Pothole Coulee's Pocket Dummy refills the practice purse a student lifted.
const (
	stockItemId     = 975501
	stockTemplateId = 975502
	stockInstanceId = 975503
	stockRoomId     = 975590
)

func seedStockMob(t *testing.T) *mobs.Mob {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		stockItemId: {ItemId: stockItemId, Name: "sand purse", Type: items.Object, Weight: 0.1},
	}))
	t.Cleanup(seedTestRoom(t, stockRoomId, "Testzone"))
	t.Cleanup(seedTestMob(t, stockTemplateId, stockInstanceId, stockRoomId, "pocket dummy"))
	m := mobs.GetInstance(stockInstanceId)
	require.NotNil(t, m)
	m.Character.Stats.Strength.ValueAdj = 100 // carry capacity for the purse
	return m
}

func carried(m *mobs.Mob, itemId int) int {
	n := 0
	for _, it := range m.Character.Items {
		if it.ItemId == itemId {
			n++
		}
	}
	return n
}

func TestActStockItem_GivesOneToAnEmptyPocket(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}

	require.Equal(t, Success, actStockItem(map[string]any{"item_id": stockItemId}, ctx))
	require.Equal(t, 1, carried(m, stockItemId))
}

func TestActStockItem_FailsWhenAlreadyStocked(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}

	require.Equal(t, Success, actStockItem(map[string]any{"item_id": stockItemId}, ctx))
	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId}, ctx),
		"a full pocket fails, so a sequence after it says nothing")
	require.Equal(t, 1, carried(m, stockItemId))
}

func TestActStockItem_MaxAllowsMore(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}
	params := map[string]any{"item_id": stockItemId, "max": 2}

	require.Equal(t, Success, actStockItem(params, ctx))
	require.Equal(t, Success, actStockItem(params, ctx))
	require.Equal(t, Failure, actStockItem(params, ctx))
	require.Equal(t, 2, carried(m, stockItemId))
}

func TestActStockItem_NoMobOrNoItemFails(t *testing.T) {
	seedStockMob(t)
	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId}, &EvalContext{}))
	require.Equal(t, Failure, actStockItem(map[string]any{}, &EvalContext{InstanceId: stockInstanceId}))
}
```

- [ ] **Step 2: Run and see it fail to compile.** `go test ./internal/behaviortree/ -run TestActStockItem -count=1`. Expected: `undefined: actStockItem`.

- [ ] **Step 3: Implement.** In `internal/behaviortree/actions_quest.go` add after `actTakeItem` (the file already imports `acquire`, `items`; add `"github.com/GoMudEngine/GoMud/internal/mobs"` if it is not imported):

```go
// actStockItem keeps the acting mob carrying an item (#375 phase 7): when it
// holds fewer than max copies of item_id (default 1) it gains one through
// the acquire seam and succeeds; otherwise it fails and nothing changes, so
// a line after it in a sequence is said only when a copy was added. The
// Pothole Coulee's Pocket Dummy (9127) refills the practice purse with it.
// params: item_id (int), max (int, default 1).
func actStockItem(params map[string]any, ctx *EvalContext) Result {
	itemId := getIntParam(params, "item_id")
	if itemId <= 0 || ctx == nil {
		return Failure
	}
	maxCount := getIntParam(params, "max")
	if maxCount <= 0 {
		maxCount = 1
	}
	m := mobs.GetInstance(ctx.InstanceId)
	if m == nil {
		return Failure
	}
	held := 0
	for _, it := range m.Character.Items {
		if it.ItemId == itemId {
			held++
		}
	}
	if held >= maxCount {
		return Failure
	}
	if !acquire.AcquireItemForChar(&m.Character, 0, m.InstanceId, items.New(itemId), acquire.AcquireTreeGive) {
		return Failure
	}
	return Success
}
```

In `internal/behaviortree/actions.go` `init()`, replace `	actionRegistry["give_item_multiple"] = actGiveItemMultiple` with:

```go
	actionRegistry["give_item_multiple"] = actGiveItemMultiple
	actionRegistry["stock_item"] = actStockItem
```

- [ ] **Step 4: Run green, prove it can fail, commit.** Rerun Step 2: all four PASS. Change `held >= maxCount` to `held > maxCount`, confirm `TestActStockItem_FailsWhenAlreadyStocked` fails, restore.

```bash
gofmt -l internal/behaviortree/
go test ./internal/behaviortree/ -count=1
go test ./internal/guards/ -count=1
git add internal/behaviortree/actions_quest.go internal/behaviortree/actions.go internal/behaviortree/actions_stock_test.go
git commit -F - <<'EOF'
feat(behaviortree): stock_item keeps a mob carrying an item

#375 phase 7, Pothole Coulee plan 4. A theft never refills a mob and a mob
restocks only by dying, so the Coulee's practice dummy would be left empty
after the first student. stock_item gives the acting mob one copy of an item
through the acquire seam while it holds fewer than max (default 1), and
fails otherwise, so a line after it speaks only on a refill.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: item, area, mobs, the dummy's tree and the rooms (Sonnet)

**Files:**
- Create: `_datafiles/world/dogmud/items/other-0/60-sand_purse.yaml`, `_datafiles/world/dogmud/areas/coulee_mine.yaml`, `_datafiles/world/dogmud/mobs/pothole_coulee/9125-lamplighter_gethin.yaml`, `9126-cutpurse_dace.yaml`, `9127-pocket_dummy.yaml`, `_datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml`
- Modify: `_datafiles/world/dogmud/areas/area-keys.txt`, `rooms/pothole_coulee/5218.yaml`, `5255.yaml`, `5256.yaml`, `5259.yaml`, `5261.yaml`

Rules for every YAML written: file name matches the `name:` field through ConvertForFilename; no colon followed by a space inside unquoted text; no em or en dash and no `--` anywhere, comments included.

- [ ] **Step 1: The purse.** Write `_datafiles/world/dogmud/items/other-0/60-sand_purse.yaml`:

```yaml
itemid: 60
name: Sand Purse
namesimple: purse
description: >-
  A small leather purse sewn shut around a fistful of sand. It hangs and
  swings like a full purse and is worth nothing at all. Cutpurse Dace keeps
  the pocket dummy on the Reed Jetty stocked with them for her students.
type: object
subtype: holdable
uses: 0
weight: 0.1
value: 0
not_salable: true
```

- [ ] **Step 2: The mine area.** Write `_datafiles/world/dogmud/areas/coulee_mine.yaml`:

```yaml
key: coulee_mine
name: the old mine
rooms: [5255, 5256, 5257, 5258, 5259, 5260, 5261]
```

Append the line `coulee_mine` to the key list in `_datafiles/world/dogmud/areas/area-keys.txt` (after `unquiet_grove`, before any `# retired` section).

- [ ] **Step 3: Lamplighter Gethin.** Write `_datafiles/world/dogmud/mobs/pothole_coulee/9125-lamplighter_gethin.yaml`:

```yaml
mobid: 9125
zone: Pothole Coulee
behavior_archetype: noncombat_questgiver
statpool: 40
hostile: false
non_combatant: true
charm_immune: true
maxwander: 0
groups:
  - humanoid
  - coulee_folk
activitylevel: 10
idlecommands:
  - 'emote trims the wick of a lamp on the shored timber and watches the flame settle'
  - ''
  - 'emote holds a lamp out toward the dark of the drift and counts under his breath'
  - ''
  - 'emote wipes soot from a lamp glass with the cuff of his sleeve'
  - ''
character:
  name: Lamplighter Gethin
  description: |
    Gethin keeps the lamps at the head of the old mine, a row of soot-black
    lanterns hung from the shoring so that whoever goes down into the
    workings can find the way back up. He is a lean, quiet man with ash on
    his fingers and pupils that open wide as a cat's in the gloom and pinch
    to slits when he lifts a lamp, so that the light itself seems to tell
    you where he is looking. He talks about the dark the way a farmer talks
    about weather: plainly, without fear, as a thing to be measured and
    prepared for. Newcomers sent down to him learn what a light can do, and
    what the dark can do back.
  speciesid: 1
  level: 1
  gold: 0
  equipment:
    light:
      itemid: 20134
  stats:
    perception:
      base: 106
    willpower:
      base: 104
```

- [ ] **Step 4: Cutpurse Dace.** Write `_datafiles/world/dogmud/mobs/pothole_coulee/9126-cutpurse_dace.yaml`:

```yaml
mobid: 9126
zone: Pothole Coulee
behavior_archetype: noncombat_questgiver
statpool: 40
hostile: false
non_combatant: true
charm_immune: true
maxwander: 0
groups:
  - humanoid
  - coulee_folk
activitylevel: 10
idlecommands:
  - 'emote walks a coin across her knuckles and back without looking at it'
  - ''
  - 'emote straightens the coat on the practice dummy and pats its empty pocket'
  - ''
  - 'emote sits on the end of the jetty with her boots over the water, watching everyone'
  - ''
character:
  name: Cutpurse Dace
  description: |
    Dace sits at the end of the Reed Jetty as if she owns it, a small, sharp
    woman in a patched coat with more pockets than any honest coat needs.
    Her fingers are long, each with one joint more than it should have, and
    they are never still: a coin walks across her knuckles, a knot ties and
    unties itself, a button vanishes from your sleeve and comes back before
    you notice it went. She was a cutpurse once, and some in the coulee say
    she still is. The town lets her teach quiet hands, so long as the lessons
    stay on her practice dummy and off the neighbors.
  speciesid: 1
  level: 1
  gold: 0
  equipment:
    light:
      itemid: 20134
  stats:
    dexterity:
      base: 108
    charisma:
      base: 104
```

- [ ] **Step 5: The Pocket Dummy.** Write `_datafiles/world/dogmud/mobs/pothole_coulee/9127-pocket_dummy.yaml`:

```yaml
mobid: 9127
zone: Pothole Coulee
behavior_archetype: combat_passive
statpool: 3
hostile: false
maxwander: 0
carry_capacity: 5
groups:
  - construct
activitylevel: 5
idlecommands:
  - 'emote turns a slow half circle on its peg, and a little bell at its cuff rings once'
  - ''
  - ''
  - ''
character:
  name: Pocket Dummy
  description: |
    A straw dummy on a peg at the root of the jetty, dressed in a coat so
    patched it is more patch than coat. Tiny brass bells are sewn along the
    cuffs and the edges of every pocket, so a clumsy hand rings them. One
    pocket sags with the weight of a purse. Dace's students practice on it:
    in, out, and away before the bells give them up. It is built to be
    robbed all day and stand for the next student.
  speciesid: 19
  level: 1
  gold: 0
  items:
    - itemid: 60
  stats:
    # Species 19 (dummy) has base vitality 200 and base damage 0. Dropped
    # far down like the training dummy (9109) and practice butt (9163), so a
    # student who is caught and swung at can knock it down in a round or two
    # and a fresh one is set out. Low perception makes it a fair first mark.
    vitality:
      base: 12
    strength:
      base: 7
    dexterity:
      base: 7
    perception:
      base: 5
```

- [ ] **Step 6: The dummy's tree.** Write `_datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml`:

```yaml
# Pocket Dummy (9127), Reed Jetty (5218): the Shadows line's practice mark
# (#375 phase 7, Pothole Coulee pass; quests 99 Quiet Hands and 101 Quick
# Fingers).
#
# The dummy carries one Sand Purse (item 60). A theft never gives it back and
# a mob restocks only by dying, so on an idle tick the dummy refills its own
# pocket: one chance in five a tick, and only while it holds no purse.
# stock_item fails on a full pocket, so the line after it is said only on a
# refill. A student who planted the purse back has already refilled it.
#
# Every other event falls through to its combat_passive archetype.

tree:
  type: selector
  children:
    - type: decorator
      event: mob_idle
      mod: random
      percent: 20
      child:
        type: sequence
        children:
          - type: action
            do: stock_item
            item_id: 60
            max: 1
          - type: action
            do: emote
            text: >-
              sways on its peg, and a fresh purse shows at the edge of its coat
              pocket.
```

- [ ] **Step 7: The Reed Jetty (5218).** In `rooms/pothole_coulee/5218.yaml`:

Replace:

```yaml
  afternoon go by. The reeds whisper around you, hiding the path back
  south.
```

with:

```yaml
  afternoon go by. The reeds whisper around you, hiding the path back
  south. A small iron <ansi fg="itemname">lockbox</ansi> is chained to the
  post nearest the shore, its keyhole scratched bright by practice picks.
```

Replace:

```yaml
  reeds: The reeds crowd close on both sides of the jetty, tall and green
    and thick enough to hide a wading body. They whisper and lean when the
    wind comes down the coulee. Dragonflies rest on the upper stalks
    between their darting runs over the open water.
idlemessages:
```

with:

```yaml
  reeds: The reeds crowd close on both sides of the jetty, tall and green
    and thick enough to hide a wading body. They whisper and lean when the
    wind comes down the coulee. Dragonflies rest on the upper stalks
    between their darting runs over the open water.
  lockbox: A small iron lockbox chained to the post nearest the shore. It
    holds nothing worth taking. Its keyhole is scratched bright from years
    of practice picks, and it locks itself again a little while after
    someone opens it.
spawninfo:
- mobid: 9126
- mobid: 9127
  message: A fresh straw dummy in a patched coat is propped up at the root of the jetty.
  narraterespawn: true
  respawnrate: 5 rounds
containers:
  lockbox:
    lock:
      difficulty: 3
      relockinterval: 2 real minutes
idlemessages:
```

- [ ] **Step 8: The Mine Head (5255).** In `rooms/pothole_coulee/5255.yaml` replace:

```yaml
spawninfo:
- mobid: 9120
  message: A pale mine crawler pours out of a crack in the wall, too many legs finding purchase.
  respawnrate: 5 real minutes
```

with:

```yaml
spawninfo:
- mobid: 9125
```

- [ ] **Step 9: The Timbered Drift (5256).** In `rooms/pothole_coulee/5256.yaml` delete these lines entirely:

```yaml
spawninfo:
- mobid: 9120
  message: A mine crawler skitters along the shoring overhead.
  respawnrate: 5 real minutes
- mobid: 9121
  message: A tunnel brute hauls itself upright in the drift, stone-knuckled and slow.
  respawnrate: 5 real minutes
```

- [ ] **Step 10: The Stone Gallery (5259) and Deep Vein (5261).** In `5259.yaml` replace the line `idlemessages:` (it occurs once) with:

```yaml
spawninfo:
- mobid: 9120
  message: A pale mine crawler pours out of a crack in the gallery wall, too many legs finding purchase.
  respawnrate: 5 real minutes
- mobid: 9121
  message: A tunnel brute hauls itself upright in the gallery, stone-knuckled and slow.
  respawnrate: 5 real minutes
idlemessages:
```

In `5261.yaml` replace the line `idlemessages:` with:

```yaml
spawninfo:
- mobid: 9120
  message: A mine crawler skitters out along the seam.
  respawnrate: 5 real minutes
idlemessages:
```

- [ ] **Step 11: Check and commit.** Room spawn lists are `instance:"skip"`, so no instance wipe is needed for them.

```bash
grep -rnE ' -- |—|–' _datafiles/world/dogmud/items/other-0/60-sand_purse.yaml _datafiles/world/dogmud/areas/coulee_mine.yaml _datafiles/world/dogmud/mobs/pothole_coulee/912[5-7]-*.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml
```

Expected: no output (run on its own; grep exits 1 when it finds nothing).

```bash
go test ./internal/guards/ -run 'TestAreaKeyManifest|TestCouleeNPCsCarryALightAndAreSeenByFace' -count=1 -v
```

Expected: PASS (the light guard now covers 9125 and 9126). If the area guard's test name differs, run `go test ./internal/guards/ -count=1`.

```bash
git add _datafiles/world/dogmud/items/other-0/60-sand_purse.yaml _datafiles/world/dogmud/areas/coulee_mine.yaml _datafiles/world/dogmud/areas/area-keys.txt _datafiles/world/dogmud/mobs/pothole_coulee/9125-lamplighter_gethin.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9126-cutpurse_dace.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9127-pocket_dummy.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5218.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5255.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5256.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5259.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5261.yaml
git commit -F - <<'EOF'
feat(coulee): Shadows line NPCs, practice dummy, lockbox and mine area

#375 phase 7, Pothole Coulee plan 4. Lamplighter Gethin (9125) at the Mine
Head and Cutpurse Dace (9126) on the Reed Jetty, both carrying the Keeper's
Lantern the coulee light guard asks for. The Pocket Dummy (9127) carries a
Sand Purse (item 60) and refills it on idle through stock_item. The jetty
gets a trap-free difficulty-3 lockbox that relocks after two real minutes,
and holds nothing, since no container restocks. The mine becomes the
coulee_mine area. The crawler at the Mine Head and the crawler and brute in
the Timbered Drift move to the Stone Gallery and the Deep Vein, so the
lamplighter's post and the first dark room are quiet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: quests 97 to 101 and their step ids (Opus)

**Files:**
- Create: `_datafiles/world/dogmud/quests/97-report_to_toke__shadows_.yaml` (or plan 3's form, Task 0 Step 3), `98-first_light.yaml`, `99-quiet_hands.yaml`, `100-from_the_dark.yaml`, `101-quick_fingers.yaml`
- Modify: `_datafiles/world/dogmud/quests/step-ids.txt`

The loop validator cannot pass until Task 6 adds the offers and hand-ins, so this task runs only the manifest and parse checks; the loop sweep runs at the end of Task 7.

- [ ] **Step 1: Quest 97.** Write `_datafiles/world/dogmud/quests/97-report_to_toke__shadows_.yaml`:

```yaml
# Quest 97: Report to Toke (Shadows). Pothole Coulee, the Shadows line's
# bridge back to the hub (#375 phase 7, Pothole Coulee pass; #315).
#
# Token flow:
#   97-start: granted on Accept of Cutpurse Dace's offer report_shadows_offer
#             (dialogue/pothole_coulee/9126.yaml), open once From the Dark
#             (100) is handed in.
#   97-end:   granted by Crier Toke's hand-in report_shadows_in
#             (dialogue/pothole_coulee/9106.yaml).
#
# Toke offers Esk's Road (89) once all eight bridges, 90 to 97, are in.

questid: 97
name: Report to Toke (Shadows)
description: >-
  Cutpurse Dace has taught you what she can for now. Crier Toke likes to hear
  how each line ends, from the one who walked it.
secret: false

steps:
  - id: start
    destination:
      mob: 9106
      room: 5203
    description: >-
      Go back to Crier Toke in Hub Square and tell him the Shadows line is
      done.
    hint: >-
      From the Reed Jetty go south, east to the Awakening Pool, then south to
      Hub Square. Type talk toke.
  - id: end
    description: >-
      You told Toke how the Shadows line ended.
```

- [ ] **Step 2: Quest 98.** Write `_datafiles/world/dogmud/quests/98-first_light.yaml`:

```yaml
# Quest 98: First Light. Pothole Coulee, the Shadows line, quest 1 of 3
# (#375 phase 7, Pothole Coulee pass; #315).
#
# Token flow (one action per step, each trigger gated on the step before):
#   98-start:   granted on Accept of Crier Toke's offer send_light
#               (dialogue/pothole_coulee/9106.yaml), open after 31-end.
#   98-lantern: topic_heard on Lamplighter Gethin (9125), topic
#               first_light_lantern, which hands over a hooded lantern
#               (item 20097) through givesItem.
#   98-equip:   command equip of the hooded lantern (item 20097). hood refuses
#               a lantern that is not in the light slot, so this step exists.
#   98-drift:   room_enter of the Timbered Drift (5256), cave biome with no
#               skylight, the first room of the mine with no light of its own.
#   98-hooded:  command hood of the hooded lantern in the Timbered Drift
#               (internal/usercommands/hood.go raises it since this pass).
#   98-end:     granted by Gethin's hand-in first_light_in.
#
# Gethin's lantern_spare topic replaces a lost lantern while it is needed.

questid: 98
name: First Light
description: >-
  Lamplighter Gethin keeps the lamps at the Mine Head, where the coulee's old
  mine goes down into the dark. He teaches newcomers what a light can do, and
  what the dark can do back.
secret: false

steps:
  - id: start
    destination:
      mob: 9125
      room: 5255
    description: >-
      Crier Toke has sent you to Lamplighter Gethin at the Mine Head. The way
      runs past the Coulee Smithy and up over the talus, where scree
      scavengers and talus lurkers hunt. Type consider on a creature to size
      it up, and flee if a fight turns.
    hint: >-
      From Hub Square go north, west five times to the Coulee Smithy, north,
      west five times to the Mine Mouth, then down. Type talk gethin.
  - id: lantern
    destination: anywhere
    description: >-
      Gethin has given you a hooded lantern. A light does nothing in your
      pack. Put it in your light slot.
    hint: >-
      Type equip lantern.
  - id: equip
    description: >-
      Your lantern is lit. West of the Mine Head the old drift has no light
      at all except the one you carry. Walk in.
    hint: >-
      From the Mine Head, go west into the Timbered Drift. Walking in is
      enough.
  - id: drift
    description: >-
      You stand in the drift by your own light. Now close the lantern's hood
      and find out what the dark is like.
    hint: >-
      Type hood.
  - id: hooded
    destination:
      mob: 9125
      room: 5255
    description: >-
      The dark closed in the moment the hood came down. Go back to Gethin at
      the Mine Head and tell him. Type unhood whenever you want your light
      back.
    hint: >-
      Go east to the Mine Head. Type talk gethin.
  - id: end
    description: >-
      Gethin has shown you how to carry a light, and how to put it out.

rewards:
  gold: 15
  actor: >-
    Gethin nods slowly. "Now you know what your light is for, and what the
    hood is for. A light lets you see. A closed hood lets you stay unseen.
    Keep the lantern. It is yours."
  observer: ""

triggers:
  # ── THE LANTERN: hear Gethin's first lesson, which hands it over ──
  - event: topic_heard
    mob: 9125
    topic: first_light_lantern
    conditions:
      has: ["98-start"]
      missing: ["98-lantern"]
    actions:
      - grant: "98-lantern"

  # ── EQUIP: the hooded lantern into the light slot ──
  - event: command
    command: equip
    item: 20097
    conditions:
      has: ["98-lantern"]
      missing: ["98-equip"]
    actions:
      - grant: "98-equip"
      - actor: >-
          The lantern settles in your light slot and its glow trims itself to
          your eyes. Now go west from the Mine Head, into the drift.

  # ── THE DRIFT: walk into the dark by your own light ──
  - event: room_enter
    room: 5256
    conditions:
      has: ["98-equip"]
      missing: ["98-drift"]
    actions:
      - grant: "98-drift"
      - actor: >-
          Your lantern is the only light in the drift. The old timbers stand
          out of the black around you. Type hood to close it.

  # ── HOOD: close it, in the drift ──
  - event: command
    command: hood
    item: 20097
    room: 5256
    conditions:
      has: ["98-drift"]
      missing: ["98-hooded"]
    actions:
      - grant: "98-hooded"
      - actor: >-
          The dark is total. You can hear the timbers settle and nothing else.
          Type unhood when you want your light back, then go east to Gethin.
```

- [ ] **Step 3: Quest 99.** Write `_datafiles/world/dogmud/quests/99-quiet_hands.yaml`:

```yaml
# Quest 99: Quiet Hands. Pothole Coulee, the Shadows line, quest 2 of 3
# (#375 phase 7, Pothole Coulee pass; #315).
#
# Token flow (one action per step, each trigger gated on the step before):
#   99-start:   granted on Accept of Lamplighter Gethin's offer
#               quiet_hands_offer (dialogue/pothole_coulee/9125.yaml), open
#               after 98-end. The offer hands over Iron Lockpicks (item 33).
#   99-lesson:  topic_heard on Cutpurse Dace (9126), topic quiet_hands_lesson.
#   99-hidden:  command sneak, anywhere (sneak raises it only when the player
#               ends up hidden). Dace and the dummy watch the jetty and may
#               spot a student, so the step does not insist on the room.
#   99-lifted:  item_gain of the Sand Purse (item 60) on the Reed Jetty. steal
#               raises its own command event before the contest, so item_gain
#               is what proves the theft landed.
#   99-planted: command plant aimed at the Pocket Dummy (9127) with the purse.
#               plant raises it on the attempt, before the contest, so a caught
#               plant counts; the line says so.
#   99-picked:  command picklock aimed at the lockbox container on the jetty
#               (internal/usercommands/picklock.go raises it since this pass).
#   99-end:     granted by Dace's hand-in quiet_hands_in.
#
# Dace's lockpicks_spare topic replaces broken picks while they are needed.

questid: 99
name: Quiet Hands
description: >-
  Cutpurse Dace teaches quiet hands on the Reed Jetty, on a straw dummy in an
  old coat and a little lockbox chained to a post. Hide, lift the purse, put
  it back, and pick the lock.
secret: false

steps:
  - id: start
    destination:
      mob: 9126
      room: 5218
    description: >-
      Lamplighter Gethin has sent you to Cutpurse Dace on the Reed Jetty, with
      a set of lockpicks of your own.
    hint: >-
      From Hub Square go north, west, then north to the Reed Jetty. Type talk
      dace.
  - id: lesson
    destination: anywhere
    description: >-
      Dace wants you out of sight first. If someone spots you, step somewhere
      quieter and try again.
    hint: >-
      Type sneak.
  - id: hidden
    description: >-
      You are hidden. The pocket dummy on the Reed Jetty has a purse in its
      coat. Lift it. If the dummy catches you it swings back, harmlessly.
      Knock it down and a fresh one is set out.
    hint: >-
      On the Reed Jetty, type steal dummy.
  - id: lifted
    description: >-
      The purse is yours. Now put it back in the dummy's coat without being
      caught.
    hint: >-
      Type plant purse on dummy.
  - id: planted
    description: >-
      Last, pick the lockbox chained to the jetty post. If it is already open,
      wait a little. It locks itself again.
    hint: >-
      Type picklock lockbox. Then choose up or down for each pin.
  - id: picked
    destination:
      mob: 9126
      room: 5218
    description: >-
      The lock gave. Tell Dace you have done all four.
    hint: >-
      Type talk dace.
  - id: end
    description: >-
      Dace has taught you quiet hands: how to hide, to lift, to plant, and to
      pick a lock.

rewards:
  skillinfo: "skullduggery:1"
  gold: 15
  actor: >-
    Dace flips a coin across her knuckles and catches it without looking.
    "Your hands are quieter than they were this morning. Keep the picks.
    Picks break, so buy more before you need them."
  observer: ""

triggers:
  # ── THE LESSON: hear Dace out ──
  - event: topic_heard
    mob: 9126
    topic: quiet_hands_lesson
    conditions:
      has: ["99-start"]
      missing: ["99-lesson"]
    actions:
      - grant: "99-lesson"

  # ── SNEAK: out of sight, anywhere ──
  - event: command
    command: sneak
    conditions:
      has: ["99-lesson"]
      missing: ["99-hidden"]
    actions:
      - grant: "99-hidden"
      - actor: >-
          You are out of sight. Now lift the purse from the pocket dummy's
          coat. On the Reed Jetty, type steal dummy.

  # ── LIFT: the purse lands in your pack, on the jetty ──
  - event: item_gain
    item: 60
    room: 5218
    conditions:
      has: ["99-hidden"]
      missing: ["99-lifted"]
    actions:
      - grant: "99-lifted"
      - actor: >-
          The purse is in your hand. Now put it back where it came from. Type
          plant purse on dummy.

  # ── PLANT: slip it back into the dummy's coat ──
  - event: command
    command: plant
    target: mob
    mob: 9127
    item: 60
    conditions:
      has: ["99-lifted"]
      missing: ["99-planted"]
    actions:
      - grant: "99-planted"
      - actor: >-
          That is the plant. If the dummy felt it, the purse is still in your
          pack, and you can try again for practice. Now the lockbox on the
          post. Type picklock lockbox.

  # ── PICK: open the lockbox on the jetty ──
  - event: command
    command: picklock
    target: container
    noun: lockbox
    room: 5218
    conditions:
      has: ["99-planted"]
      missing: ["99-picked"]
    actions:
      - grant: "99-picked"
      - actor: >-
          The last pin lifts and the lid comes free. Go and tell Dace. Type
          talk dace.
```

- [ ] **Step 4: Quest 100.** Write `_datafiles/world/dogmud/quests/100-from_the_dark.yaml`:

```yaml
# Quest 100: From the Dark. Pothole Coulee, the Shadows line, quest 3 of 3
# (#375 phase 7, Pothole Coulee pass; #315).
#
# Token flow (one action per step, each trigger gated on the step before):
#   100-start:  granted on Accept of Cutpurse Dace's offer from_the_dark_offer
#               (dialogue/pothole_coulee/9126.yaml), open after 99-end.
#   100-hidden: command sneak in the coulee_mine area.
#   100-ambush: command ambush aimed at a mob in the mine. A player's opening
#               strike from hiding raises it, melee or a shot from cover,
#               landed or not (hooks.applyCombatProgression, usercommands
#               shoot.go, since this pass).
#   100-kill:   mob_death of any creature in the mine the player has kill
#               credit on.
#   100-end:    granted by Dace's hand-in from_the_dark_in.
#
# The Timbered Drift (5256) has no spawns since this pass, so a student can
# hide there unseen; the Tunnel Brute (9121) in the Flooded Sump (5257), one
# room west, is the target the text names.

questid: 100
name: From the Dark
description: >-
  Cutpurse Dace wants you to learn the quiet strike. Go down into the old
  mine, get out of sight, and hit something before it knows you are there.
secret: false

steps:
  - id: start
    description: >-
      Go down into the old mine below the Mine Mouth. Where nothing can see
      you, slip out of sight. The Timbered Drift, west of the Mine Head, is
      usually empty.
    hint: >-
      From the Mine Head go west into the Timbered Drift. Type sneak.
  - id: hidden
    description: >-
      You are hidden. West of the drift, in the Flooded Sump, a tunnel brute
      guards the water. Close your lantern's hood before you walk in, so your
      light does not give you away. Beside it, open the hood and strike at
      once. Type consider on it first if you want to size it up.
    hint: >-
      Go west into the Flooded Sump and type attack brute.
  - id: ambush
    description: >-
      Your strike came out of the dark. Now it is an ordinary fight. Finish
      it, or flee if it turns against you.
    hint: >-
      Keep fighting until it falls. Type flee if the fight turns.
  - id: kill
    destination:
      mob: 9126
      room: 5218
    description: >-
      It is dead. Climb back out of the mine and tell Dace on the Reed Jetty.
    hint: >-
      Go up from the Mine Head, east five times, south to the Smithy, east
      four times, then north to the Reed Jetty. Type talk dace.
  - id: end
    description: >-
      You struck from the dark and finished the fight.

rewards:
  gold: 15
  actor: >-
    Dace looks you over for blood, finds less than she expected, and nods.
    "One blow from the dark is worth three in the open. Remember it opens the
    fight, and you still have to finish it."
  observer: ""

triggers:
  # ── HIDE: out of sight somewhere in the mine ──
  - event: command
    command: sneak
    area: coulee_mine
    conditions:
      has: ["100-start"]
      missing: ["100-hidden"]
    actions:
      - grant: "100-hidden"
      - actor: >-
          You are out of sight in the dark. West lies the Flooded Sump and the
          brute that guards it. Walk in quietly, open your hood beside it, and
          type attack brute.

  # ── STRIKE: the opening blow from hiding ──
  - event: command
    command: ambush
    target: mob
    area: coulee_mine
    conditions:
      has: ["100-hidden"]
      missing: ["100-ambush"]
    actions:
      - grant: "100-ambush"
      - actor: >-
          Your first blow comes out of the dark. It knows where you are now.
          Finish the fight.

  # ── FINISH: it falls ──
  - event: mob_death
    area: coulee_mine
    conditions:
      has: ["100-ambush"]
      missing: ["100-kill"]
    actions:
      - grant: "100-kill"
      - actor: >-
          It falls. Climb back out of the mine and tell Dace on the Reed
          Jetty.
```

- [ ] **Step 5: Quest 101.** Write `_datafiles/world/dogmud/quests/101-quick_fingers.yaml`:

```yaml
# Quest 101: Quick Fingers. Pothole Coulee, the Shadows line repeatable
# (#375 phase 7, Pothole Coulee pass; #315).
#
# Token flow:
#   101-start:  granted on Accept of Cutpurse Dace's offer quick_fingers_offer
#               (dialogue/pothole_coulee/9126.yaml), open after 100-end.
#               Repeatable with the cooldown the other Coulee repeatables use.
#   101-lifted: item_gain of the Sand Purse (item 60) on the Reed Jetty,
#               counted to 3. The Pocket Dummy refills its pocket on idle
#               (behaviors/pothole_coulee/9127-pocket_dummy.yaml).
#   101-end:    granted by Dace's hand-in quick_fingers_in.

questid: 101
name: Quick Fingers
description: >-
  Dace keeps the pocket dummy stocked for anyone who wants to keep their
  fingers quick. Lift purses from its coat until she is satisfied, then tell
  her.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5218
    description: >-
      Lift three purses from the pocket dummy on the Reed Jetty. It refills
      its own coat if you give it a moment between tries.
    hint: >-
      On the Reed Jetty, type steal dummy.
  - id: lifted
    destination:
      mob: 9126
      room: 5218
    description: >-
      Three purses lifted. Tell Dace.
    hint: >-
      Type talk dace.
  - id: end
    description: >-
      Your fingers are a little quicker. Dace will have the dummy stocked
      again after a while.

rewards:
  gold: 6
  skillinfo: "skullduggery:1"
  actor: >-
    Dace counts the purses without looking at them. "Quicker. Come back when
    your fingers start to forget."
  observer: ""

triggers:
  - event: item_gain
    item: 60
    room: 5218
    count: 3
    label: Purses lifted
    conditions:
      has: ["101-start"]
      missing: ["101-lifted"]
    actions:
      - grant: "101-lifted"
      - actor: >-
          That is three. Go and tell Dace. Type talk dace.
```

- [ ] **Step 6: Step ids.** Add these lines to the key list of `_datafiles/world/dogmud/quests/step-ids.txt` in sorted position (before any `# retired` section; the parser sorts, so position is tidiness only):

```
97-end
97-start
98-drift
98-end
98-equip
98-hooded
98-lantern
98-start
99-end
99-hidden
99-lesson
99-lifted
99-picked
99-planted
99-start
100-ambush
100-end
100-hidden
100-kill
100-start
101-end
101-lifted
101-start
```

- [ ] **Step 7: Check and commit**

```bash
grep -rnE ' -- |—|–' _datafiles/world/dogmud/quests/97-*.yaml _datafiles/world/dogmud/quests/98-*.yaml _datafiles/world/dogmud/quests/99-*.yaml _datafiles/world/dogmud/quests/100-*.yaml _datafiles/world/dogmud/quests/101-*.yaml
```

Expected: no output.

```bash
go test ./internal/guards/ -run 'TestStepIdManifest' -count=1 -v
go test ./internal/quests/ -count=1
```

Expected: PASS (`TestShippedQuestRoomTextFollowsConvention` and the parse tests read the new files).

```bash
git add _datafiles/world/dogmud/quests/97-report_to_toke__shadows_.yaml _datafiles/world/dogmud/quests/98-first_light.yaml _datafiles/world/dogmud/quests/99-quiet_hands.yaml _datafiles/world/dogmud/quests/100-from_the_dark.yaml _datafiles/world/dogmud/quests/101-quick_fingers.yaml _datafiles/world/dogmud/quests/step-ids.txt
git commit -F - <<'EOF'
feat(coulee): quests 97 to 101, the Shadows line

#375 phase 7, Pothole Coulee plan 4; part of #315. 98 First Light (Toke
starts it; Gethin hands over a hooded lantern; equip, walk into the dark
drift, hood it, report), 99 Quiet Hands (Dace's lesson, sneak, lift the
purse, plant it back, pick the lockbox, report), 100 From the Dark (hide in
the mine, strike from hiding, finish it, report), 101 Quick Fingers
(repeatable: lift three purses, report) and the bridge 97 Report to Toke
(Shadows). One action per step, each hinted with a command. Offers and
hand-ins follow in the next commit.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Gethin's and Dace's dialogue, Toke's new topics (Opus)

**Files:**
- Create: `_datafiles/world/dogmud/dialogue/pothole_coulee/9125.yaml`, `9126.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`, `_datafiles/world/dogmud/dialogue/topic-keys.txt`, `internal/dialogue/toke_ways_test.go`

Facts the info topics use (do not add others): `templates/help/light.template`, `darkness.template`, `steal.template:18-22,50-63`, `ambush.template`, `defuse.template`, `shadow.template`; Onna's shop (`9104-trader_onna.yaml:57-59`); Siv's shop (`104-fence_dealer_siv.yaml:55,59`); Vurl's crafter recipes (`9607-jeweler_vurl.yaml:31,34`). No durations in numbers.

- [ ] **Step 1: Gethin.** Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9125.yaml`:

```yaml
mobid: 9125
zone: Pothole Coulee
defaultMood: friendly
greetings:
- text: Mind your feet. The floor in here has opinions.
  moods:
  - friendly
  - neutral
opener:
  text: Gethin. I keep the lamps at the Mine Head, so whoever goes down into
    the old workings can find the way back up. Most folk fear the dark. I
    measure it.
  hints: Choose a line below to hear about the night, the dark, or lights.
  variants:
  - questExcluded:
    - 98-start
    text: Toke sends new folk my way, when they are ready for the dark. Go and
      find him in Hub Square first. Ask him who can teach you about the dark,
      and he will send you back down here.
    hints: Crier Toke is in Hub Square. Type talk toke there. You can still
      choose a line below to hear about the night, the dark, or lights.
  - questRequired:
    - 98-start
    questExcluded:
    - 98-lantern
    text: Toke sent you? Good. Then the first thing you need is a light of your
      own.
    hints: Choose the line about learning the dark, below.
  - questRequired:
    - 98-lantern
    questExcluded:
    - 98-equip
    text: A lantern in your pack lights nothing. Put it on.
    hints: Type equip lantern.
  - questRequired:
    - 98-equip
    questExcluded:
    - 98-drift
    text: West of here the drift has no light but the one you bring. Walk in.
    hints: Go west into the Timbered Drift. Walking in is enough.
  - questRequired:
    - 98-drift
    questExcluded:
    - 98-hooded
    text: You have stood in the dark by your own light. Now close the hood, in
      there.
    hints: Go west into the Timbered Drift and type hood.
  - questRequired:
    - 98-hooded
    questExcluded:
    - 98-end
    text: You came back up. Tell me what the dark was like.
    hints: Choose First Light below to finish.
topics:
- key: first_light_lantern
  kind: info
  label: Toke sent me to learn about the dark.
  text: Then we start with a light. Here, a hooded lantern. It trims itself to
    your eyes each time you walk somewhere new, so it never dazzles you. And it
    has a hood, which is the part that matters down here. Put it on, walk west
    into the drift, and close the hood. Then come and tell me what you saw.
  hints: Type equip lantern.
  questRequired:
  - 98-start
  questExcluded:
  - 98-lantern
  - 98-end
  givesItem: 20097
- key: lantern_spare
  kind: info
  label: I've lost my lantern.
  text: Lost it already? Here is another. Keep this one closer.
  hints: Type equip lantern.
  questRequired:
  - 98-lantern
  questExcluded:
  - 98-hooded
  - 98-end
  itemExcluded: 20097
  givesItem: 20097
- key: first_light_in
  kind: handin
  quest: 98
  label: First Light
  playerLine: I walked into the drift and closed the hood.
  text: And the dark came down all at once, did it not? That is the whole
    lesson. Your light lets you see. Your light also lets everything else see
    you. Close the hood and you are only another piece of the dark. Remember
    that when you go hunting.
  questRequired:
  - 98-hooded
  questExcluded:
  - 98-end
  grantsQuest: 98-end
- key: quiet_hands_offer
  kind: offer
  quest: 99
  label: Quiet Hands
  playerLine: Who can teach me to move without being seen?
  text: Dace, on the Reed Jetty. She was a cutpurse once, and some say she
    still is. She teaches quiet hands, so long as the lessons stay on her
    practice dummy. Take these lockpicks to her. She will want you carrying
    your own.
  hints: From Hub Square go north, west, then north to the Reed Jetty. Type
    talk dace.
  questRequired:
  - 98-end
  questExcluded:
  - 99-start
  - 99-end
  grantsQuest: 99-start
  givesItem: 33
- key: night
  kind: info
  label: What changes when night falls?
  text: The sun goes down and its light goes with it. Most nights a moon is up,
    so outside you can still make out shapes. Under trees, roofs or rock the
    night is darker still. In the mine it makes no difference at all, because
    no sky reaches down here. Day or night, the only light in the mine is the
    light you bring. If you go out after dark, carry one.
  hints: Choose a line below to hear about seeing in the dark, or about lights.
  marked: true
- key: infravision
  kind: info
  label: Can anyone see in the dark?
  text: Some can, after a fashion. Night vision lets your eyes use fainter
    light than mine can, but the same eyes are dazzled sooner by a bright one.
    Infravision is different. It sees the warmth of living things, not light,
    so it shows you a shape in any darkness it can reach. A shape, mind, never
    a face. The Night Vision and Heat Sight spells give them, and so do the
    Cat's Eye Draught and the Pitsense Tincture.
  hints: Choose a line below to hear about made darkness, or about lights.
  marked: true
- key: made_dark
  kind: info
  label: Can darkness be made?
  text: It can, and it is worse than this mine. The Chrysalis Pall spell
    gathers spores that drink the light around their target, and the rare
    Umbral Lantern does the same from a light slot. A made darkness darkens a
    place for everyone in it, whoever brought it. Night vision helps only a
    little. Infravision is the answer, because warmth is not light. And a
    darkness never gives you away when you sneak.
  hints: Choose a line below to hear about seeing in the dark, or about lights.
  marked: true
- key: dazzle
  kind: info
  label: Can a light be too bright?
  text: Yes, and it costs you when it is. Too much light dazzles you. Your aim,
    your guard and your eye for detail all suffer, and the brighter it is the
    worse it gets. A plain lantern or a torch always burns at full strength,
    which is too much under open sky near a summer noon. A hooded lantern or
    the Chrysalis Glow trims itself to your eyes each time you walk somewhere
    new. And a strong light can dazzle the things that live in the dark, which
    is worth knowing.
  hints: Type help light to read more.
  marked: true
- key: buy_light
  kind: info
  label: Where can I buy a light?
  text: Trader Onna, at Coulee Provisions off Market Row. She sells torches,
    oil lanterns, and hooded lanterns with a hood you can close. A torch is
    cheap and bright. A hooded lantern costs more, but you can put it out
    without taking it off.
  hints: From Hub Square go south to Market Row, then east into Coulee
    Provisions. Type list there.
  marked: true
```

- [ ] **Step 2: Dace.** Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9126.yaml`:

```yaml
mobid: 9126
zone: Pothole Coulee
defaultMood: neutral
greetings:
- text: Keep your hands where I can see them. Habit. Mine, not yours.
  moods:
  - friendly
  - neutral
opener:
  text: Dace. I sit on this jetty and teach quiet hands to anyone Gethin sends
    me. The coulee lets me, so long as the lessons stay on the dummy and off
    the neighbors.
  hints: Choose a line below to hear about getting caught, stolen goods, or
    the tools of the trade.
  variants:
  - questExcluded:
    - 99-start
    text: I don't take students off the street. Gethin, the lamplighter at the
      Mine Head, sends me the ones worth teaching. See him first.
    hints: Lamplighter Gethin keeps the Mine Head. Crier Toke in Hub Square can
      start you on that road. You can still choose a line below.
  - questRequired:
    - 99-start
    questExcluded:
    - 99-lesson
    text: Gethin sent you? Then sit, and I will tell you how this goes.
    hints: Choose the line about learning quiet hands, below.
  - questRequired:
    - 99-lesson
    questExcluded:
    - 99-hidden
    text: First, out of sight. If I spot you, step away from me and try again.
    hints: Type sneak.
  - questRequired:
    - 99-hidden
    questExcluded:
    - 99-lifted
    text: Now the purse in the dummy's coat. Light fingers.
    hints: Type steal dummy.
  - questRequired:
    - 99-lifted
    questExcluded:
    - 99-planted
    text: Now put it back where you found it. That is the harder trick.
    hints: Type plant purse on dummy.
  - questRequired:
    - 99-planted
    questExcluded:
    - 99-picked
    text: Last, the lockbox on the post. Your picks, the lock, and patience.
    hints: Type picklock lockbox.
  - questRequired:
    - 99-picked
    questExcluded:
    - 99-end
    text: Done? Come and tell me, then.
    hints: Choose Quiet Hands below to finish.
  - questRequired:
    - 100-start
    questExcluded:
    - 100-kill
    text: The mine is where this lesson is. Not here.
    hints: In the old mine below the Mine Mouth, where nothing can see you, type
      sneak. Then attack from hiding.
  - questRequired:
    - 100-kill
    questExcluded:
    - 100-end
    text: You came back up, so it went well enough. Tell me about it.
    hints: Choose From the Dark below to finish.
topics:
- key: quiet_hands_lesson
  kind: info
  label: Gethin sent me to learn quiet hands.
  text: Four things, then. First you slip out of sight. Then you lift the purse
    out of that dummy's coat. Then you put it back, which is harder than taking
    it. Last, you pick the little lockbox on the post with the picks Gethin
    gave you. If the dummy catches you it will swing at you. It hits like a
    pillow. Knock it down and another is set out. Start by hiding, and if I
    spot you, step away from me and try again.
  hints: Type sneak.
  questRequired:
  - 99-start
  questExcluded:
  - 99-lesson
  - 99-end
- key: lockpicks_spare
  kind: info
  label: My lockpicks broke.
  text: They do that. A wrong guess snaps a pick. Here, another set. A wrong pin
    tells you which way that pin goes, so this set should last.
  hints: Type picklock lockbox.
  questRequired:
  - 99-lesson
  questExcluded:
  - 99-picked
  - 99-end
  itemExcluded: 33
  givesItem: 33
- key: quiet_hands_in
  kind: handin
  quest: 99
  label: Quiet Hands
  playerLine: I hid, lifted the purse, put it back, and picked the box.
  text: All four. Not bad for a first sitting. Quiet hands are mostly patience
    and nerve, and you have some of both.
  questRequired:
  - 99-picked
  questExcluded:
  - 99-end
  grantsQuest: 99-end
- key: from_the_dark_offer
  kind: offer
  quest: 100
  label: From the Dark
  playerLine: What comes after quiet hands?
  text: A quiet strike. Go down into the old mine below the Mine Mouth. Find
    something that lives there, get out of its sight, then hit it before it
    knows you are there. The first blow from hiding lands like nothing else.
    After that it is an ordinary fight, so be ready to finish it.
  hints: Go to the Mine Head and west into the Timbered Drift. Type sneak there.
  questRequired:
  - 99-end
  questExcluded:
  - 100-start
  - 100-end
  grantsQuest: 100-start
- key: from_the_dark_in
  kind: handin
  quest: 100
  label: From the Dark
  playerLine: I struck from the dark and finished the fight.
  text: And walked back up on your own feet. Good. Now you know why a thief
    keeps her hood closed and her first blow ready.
  questRequired:
  - 100-kill
  questExcluded:
  - 100-end
  grantsQuest: 100-end
- key: quick_fingers_offer
  kind: offer
  quest: 101
  label: Quick Fingers
  playerLine: Can I practice on the dummy again?
  text: Always. Lift three purses from its coat and come back. It refills its
    own pocket if you give it a moment between tries.
  hints: On the Reed Jetty, type steal dummy.
  questRequired:
  - 100-end
  questExcluded:
  - 101-start
  - 101-lifted
  - 101-end
  grantsQuest: 101-start
- key: quick_fingers_in
  kind: handin
  quest: 101
  label: Quick Fingers
  playerLine: I lifted three purses from the dummy.
  text: Three, and the bells barely spoke. Your fingers are getting quick.
  questRequired:
  - 101-lifted
  questExcluded:
  - 101-end
  grantsQuest: 101-end
- key: report_shadows_offer
  kind: offer
  quest: 97
  label: Report to Toke (Shadows)
  playerLine: Should I tell Toke the Shadows line is done?
  text: Go and tell him. Toke likes to hear how each line ends, and he will
    hear it from somebody, so it might as well be you.
  hints: From the Reed Jetty go south, east to the Awakening Pool, then south
    to Hub Square. Type talk toke.
  questRequired:
  - 100-end
  questExcluded:
  - 97-start
  - 97-end
  grantsQuest: 97-start
- key: shadowing
  kind: info
  label: How do I follow someone without being seen?
  text: That is shadowing, and it comes later, once your quiet hands have more
    practice behind them. You hide first, then shadow your mark, and you follow
    wherever they go. Every doorway is a chance for someone to spot you, and
    the mark may feel eyes on their back.
  hints: Type help shadow to read more.
  marked: true
- key: surprise_strike
  kind: info
  label: What is a strike from hiding?
  text: You hide, then you attack or shoot, and your first blow becomes the
    ambush. It is one blow, not a whole round, and it is spent even if it
    misses. A ready mark can still dodge it, parry it or block it. If it lands
    clean it strikes like a critical hit, and the better your skullduggery the
    deeper it bites. Either way it gives your place away, so plan the rest of
    the fight as an open one.
  hints: Type help ambush to read more.
  marked: true
- key: defuse_lesson
  kind: info
  label: What if a lock has a trap on it?
  text: Then a wrong pick sets it off. Taking a trap apart with a disarm kit is
    a skill for later, when your hands are steadier than they are today. The
    kit is used up whether you win or lose. My lockbox has no trap. I am not
    that cruel.
  hints: Type help defuse to read more.
  marked: true
- key: getting_caught
  kind: info
  label: What happens if I get caught?
  text: Your cover is gone at once, and the one you tried to rob may come at
    you. Folk remember a thief, and so do the people who keep the peace. The
    good folk of this coulee you cannot rob at all, and I would not let you
    try. Practice on the dummy.
  hints: Type help steal to read more.
  marked: true
- key: hot_goods
  kind: info
  label: What happens to the things I steal?
  text: A stolen trinket stays hot for a few days in the town where you took
    it. No honest merchant there will buy it, no storage will hold it, and its
    owner may know it on sight if your paths cross. Give it back and it cools.
    Or take it to a fence. A fence buys hot goods anywhere, and pays well for
    them.
  hints: Type help steal to read more.
  marked: true
- key: tools
  kind: info
  label: Where do lockpicks come from?
  text: Fence Dealer Siv, in Thornwall, sells plain iron picks and basic
    disarm kits. Jeweler Vurl, here in the coulee, makes master lockpicks and
    precision disarm kits now and then. A smith with enough skill can forge
    steel or master picks of their own. Picks break, so carry a spare.
  hints: Type help picklock to read more.
  marked: true
```

- [ ] **Step 3: Toke's new topics.** Append this block at the end of `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` (the `topics:` list is the last key; menu order puts quest lines first whatever their place in the file):

```yaml
# #375 phase 7: the Shadows line starts here like every line (quest 98), its
# bridge (97) is handed in here, and once all eight bridges are in, Toke
# offers Esk's Road (89), which Cleric Hadwen offered before this pass.
- key: send_light
  kind: offer
  quest: 98
  label: First Light
  playerLine: Who can teach me about the dark?
  text: Lamplighter Gethin, at the Mine Head. He keeps the lamps for the old
    mine, and he knows more about light and dark than anyone in the coulee. It
    is a long walk. Past the Coulee Smithy, up over the talus where the
    scavengers hunt, and down into the mine mouth. Keep your eyes open on the
    slope.
  hints: From Hub Square go north, west five times to the Coulee Smithy, north,
    west five times to the Mine Mouth, then down. Type talk gethin.
  questRequired:
  - 31-end
  questExcluded:
  - 98-start
  - 98-end
  grantsQuest: 98-start
- key: report_shadows_in
  kind: handin
  quest: 97
  label: Report to Toke (Shadows)
  playerLine: Dace says the Shadows line is done.
  text: Done, is it? Gethin's lantern and Dace's quiet hands. I hear you went
    down into the mine and came back out of it, which is the part most folk
    skip. Every teacher who sends you back to me is one road fewer for me to
    worry about.
  hints: When every teacher has sent you back, Toke has one more road for you.
  questRequired:
  - 97-start
  questExcluded:
  - 97-end
  grantsQuest: 97-end
- key: road_esk
  kind: offer
  quest: 89
  label: Esk's Road
  playerLine: Is there anything left for me to learn here?
  text: Not from me, and not from anyone I know. Every teacher in the coulee
    has sent you back to me, all eight of them. The rest of Gaius is waiting.
    Warden Esk keeps the arch in the Threshold House. Go and tell him you are
    ready.
  hints: From Hub Square go south twice, through Market Row, to the Threshold
    House. Type talk esk.
  questRequired:
  - 90-end
  - 91-end
  - 92-end
  - 93-end
  - 94-end
  - 95-end
  - 96-end
  - 97-end
  questExcluded:
  - 89-start
  - 89-end
  grantsQuest: 89-start
- key: esk_road_when
  kind: info
  label: When will you send me out into the world?
  text: When every teacher in the coulee has sent you back to me, all eight of
    them, I have one last road for you. But nobody is kept here. If you want to
    go sooner, Warden Esk will open the arch for you now.
  hints: Warden Esk keeps the arch in the Threshold House. From Hub Square go
    south twice.
  questRequired:
  - 31-end
  questExcluded:
  - 89-start
- key: way_mine_head
  kind: info
  label: How do I get to the Mine Head?
  text: The long way round, I am afraid. North to the pool, then west along the
    shore and the forge path to the Coulee Smithy. North into the quench shed,
    then west up the talus to the Mine Mouth, and down. Mind the scavengers on
    the slope. Lamplighter Gethin keeps the lamps down there.
  hints: 'From Hub Square: north, west five times, north, west five times, down.'
```

- [ ] **Step 4: Topic keys.** In `_datafiles/world/dogmud/dialogue/topic-keys.txt` add, in sorted position in the key list (9106 keys among the other `9106.` lines; 9125 and 9126 between the `9117.` and `9128.` lines):

```
9106.esk_road_when
9106.report_shadows_in
9106.road_esk
9106.send_light
9106.way_mine_head
9125.buy_light
9125.dazzle
9125.first_light_in
9125.first_light_lantern
9125.infravision
9125.lantern_spare
9125.made_dark
9125.night
9125.quiet_hands_offer
9126.defuse_lesson
9126.from_the_dark_in
9126.from_the_dark_offer
9126.getting_caught
9126.hot_goods
9126.lockpicks_spare
9126.quick_fingers_in
9126.quick_fingers_offer
9126.quiet_hands_in
9126.quiet_hands_lesson
9126.report_shadows_offer
9126.shadowing
9126.surprise_strike
9126.tools
```

- [ ] **Step 5: Extend `TestTokeGivesTheWay`.** In `internal/dialogue/toke_ways_test.go` replace:

```go
		{"way_scout_tarn", "From Hub Square: west, west, south, south to the Steppe Edge."},
```

with:

```go
		{"way_scout_tarn", "From Hub Square: west, west, south, south to the Steppe Edge."},
		{"way_mine_head", "From Hub Square: north, west five times, north, west five times, down."},
```

- [ ] **Step 6: Validate.**

```bash
grep -rnE ' -- |—|–|;' _datafiles/world/dogmud/dialogue/pothole_coulee/9125.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9126.yaml
```

Expected: no output.

```bash
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones pothole_coulee
go test ./internal/guards/ -run 'TestTopicKeyManifest' -count=1 -v
go test ./internal/dialogue/ -count=1
```

Expected: dialoguecheck reports no problem for 9106, 9125, 9126 (fix any it names, for example a label over 50 characters or a playerLine over 70); the manifest test and the dialogue package PASS.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/dialogue/pothole_coulee/9125.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9126.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt internal/dialogue/toke_ways_test.go
git commit -F - <<'EOF'
feat(coulee): Gethin, Dace and Toke's Shadows topics

#375 phase 7, Pothole Coulee plan 4; part of #315. Gethin hands over the
hooded lantern, takes First Light's hand-in, offers Quiet Hands with a set
of lockpicks, and teaches night, infravision, made darkness, dazzle and
where to buy a light. Dace runs Quiet Hands, offers From the Dark, Quick
Fingers and the bridge, and teaches shadowing, the strike from hiding,
traps, getting caught, hot goods and where picks come from. Toke starts
First Light, takes the Shadows bridge, offers Esk's Road behind all eight
bridges, and gains a route topic to the Mine Head. Both new givers have
replacement topics for a lost lantern and broken picks.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Leaving: quest 89 moves to Toke (Opus)

**Files:**
- Modify (rewrite): `_datafiles/world/dogmud/quests/89-esks_road.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml`, `9107.yaml`, `_datafiles/world/dogmud/dialogue/topic-keys.txt`, `_datafiles/world/dogmud/quests/loop-backlog.txt`, `internal/dialogue/hadwen_midrite_test.go`, possibly `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml` and `internal/guards/esk_return_portal_test.go`
- Create: `internal/dialogue/esk_road_test.go`

- [ ] **Step 1: Write the failing dialogue test**

`internal/dialogue/esk_road_test.go`:

```go
package dialogue

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// #375 phase 7: Esk's Road (89) is offered by Crier Toke once all eight
// "Report to Toke" bridges are in (90-end to 97-end), no longer by Cleric
// Hadwen; Warden Esk takes the hand-in, and his passage topic opens the arch
// for any Opened player, holding quest 89 or not.

func loadCouleeDialogue(t *testing.T, file string) *DialogueFile {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/pothole_coulee/" + file)
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// heldState is a player holding exactly the listed tokens.
func heldState(held ...string) *PlayerState {
	set := map[string]bool{}
	for _, h := range held {
		set[h] = true
	}
	return &PlayerState{
		QuestOpen: func(id int) bool {
			return set[fmt.Sprintf("%d-start", id)] && !set[fmt.Sprintf("%d-end", id)]
		},
		HasQuest:     func(tok string) bool { return set[tok] },
		HasItem:      func(int) bool { return false },
		CarriesItem:  func(int) bool { return false },
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}
}

func menuKeys(df *DialogueFile, ps *PlayerState) []string {
	var ids []string
	for _, e := range df.Menu("npc", ps).Entries {
		ids = append(ids, e.Id)
	}
	return ids
}

func TestEskPassageWorksWithAndWithoutEsksRoad(t *testing.T) {
	esk := loadCouleeDialogue(t, "9107.yaml")

	without := menuKeys(esk, heldState("30-start", "30-end"))
	require.Contains(t, without, "passage")
	require.NotContains(t, without, "road_esk_end")

	with := menuKeys(esk, heldState("30-start", "30-end", "89-start"))
	require.Contains(t, with, "passage")
	require.Contains(t, with, "road_esk_end")
}

func TestTokeOffersEsksRoadOnlyAfterEveryBridge(t *testing.T) {
	toke := loadCouleeDialogue(t, "9106.yaml")
	held := []string{"30-start", "30-end", "31-start", "31-end"}
	for q := 90; q <= 97; q++ {
		held = append(held, fmt.Sprintf("%d-start", q), fmt.Sprintf("%d-end", q))
	}
	require.Contains(t, menuKeys(toke, heldState(held...)), "road_esk")
	require.NotContains(t, menuKeys(toke, heldState(held[:len(held)-1]...)), "road_esk",
		"without 97-end the offer stays hidden")

	hadwen := loadCouleeDialogue(t, "9100.yaml")
	require.Nil(t, hadwen.FindTopic("road_esk"), "Hadwen no longer offers Esk's Road")
}
```

- [ ] **Step 2: Run and see it fail.** `go test ./internal/dialogue/ -run 'TestEskPassage|TestTokeOffersEsksRoad' -count=1`. Expected: `TestTokeOffersEsksRoadOnlyAfterEveryBridge` FAILS on "Hadwen no longer offers Esk's Road" (Toke's offer already exists from Task 6). If `FindTopic` returns a value type, compare with `require.Equal(t, "", hadwen.FindTopic("road_esk").Key)` instead; read `internal/dialogue/topics.go` for its signature.

- [ ] **Step 3: Remove Hadwen's offer.** In `_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml` delete the whole `road_esk` topic, from `- key: road_esk` through its `grantsQuest: 89-start` line. On master it reads:

```yaml
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
  - 29-end
  grantsQuest: 89-start
```

Plan 2 keeps this topic but has already dropped its `29-end` exclusion when it retired quest 29, so the block you find ends `- 89-end` then `grantsQuest: 89-start`. Delete the topic as it stands now. Then fix the opener and the comments:

- Every opener variant whose `questRequired` lists `89-start`: delete the variant (it described a road taken from Hadwen).
- Every opener `hints` that names "Esk's Road": remove that road from the sentence. On master the three cases become `Choose Toke's Road below to learn the town. Other lines below tell you about mutations, seeing in the dark, or your name.` (for the "Toke's Road or Esk's Road" variant) and, for the two variants that offered only Esk's Road, delete the variant: with the offer gone they would point at nothing.
- The comment above the roads (`# R17: the two roads ...`) becomes `# R17 (phase 7 update): Toke's Road is Hadwen's only road. Esk's Road (89) is offered by Crier Toke once all eight bridges are in; leaving early is his info topic that points to Esk.`

End state, checked:

```bash
grep -n "89-\|Esk's Road\|road_esk" _datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml
```

Expected: no output (run on its own).

- [ ] **Step 4: Hadwen's rite line and tree comment.** In `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml`, if this text is still present:

```yaml
            <ansi fg="command">talk hadwen</ansi>, and I will set you
            on either road, or both. The coulee will still be here
```

replace it with:

```yaml
            <ansi fg="command">talk hadwen</ansi>, and I will set you
            on your road. The coulee will still be here
```

and if the comment `# road_toke and road_esk offer topics in his dialogue, which the` is present, replace `road_toke and road_esk offer topics` with `road_toke offer topic`. Read the lines around each match before editing; if plan 2 already reworded them, leave them.

- [ ] **Step 5: Retire the key.** In `_datafiles/world/dogmud/dialogue/topic-keys.txt` delete the line `9100.road_esk` from the key list and add it under `# retired` at the end of the file. If there is no `# retired` section, append:

```
# retired
9100.road_esk
```

(If a `# renamed` section exists, put `# retired` above it; the parser reads keys, then `# retired`, then `# renamed`.)

- [ ] **Step 6: Rewrite quest 89.** Replace the whole of `_datafiles/world/dogmud/quests/89-esks_road.yaml` with:

```yaml
# Quest 89: Esk's Road. Pothole Coulee, the way out of the coulee
# (#375 phase 7, Pothole Coulee pass).
#
# Crier Toke offers it once all eight "Report to Toke" bridges are handed in
# (90-end to 97-end), so it closes the coulee's teaching. Before this pass
# Cleric Hadwen offered it after the rite (owner ruling R17). A save that took
# it from him still holds 89-start and finishes at Esk the same way.
#
# Token flow:
#   89-start: granted on Accept of Crier Toke's offer road_esk
#             (dialogue/pothole_coulee/9106.yaml).
#   89-end:   granted by Warden Esk's hand-in road_esk_end
#             (dialogue/pothole_coulee/9107.yaml). Crossing the arch no longer
#             ends it: the hand-in is the only end.
#
# Leaving early needs no quest. Cleric Hadwen's info topic and Toke's
# esk_road_when topic point to Esk, and Esk's passage topic opens the arch for
# any Opened player.

questid: 89
name: Esk's Road
description: >-
  Every teacher in the coulee has sent you back to Crier Toke. He has pointed
  you to Warden Esk, who keeps the arch out of Pothole Coulee to Thornwall and
  the wider world.
secret: false

steps:
  - id: start
    destination:
      mob: 9107  # follow Warden Esk (schedule pothole_esk)
      room: 5215  # fallback when Warden Esk is not spawned
    description: >-
      Warden Esk keeps the arch in the Threshold House, south of Hub Square
      through Market Row. The arch opens on Thornwall.
    hint: >-
      From Hub Square go south twice, through Market Row, to the Threshold
      House. Type talk esk.
  - id: end
    description: >-
      You reached Warden Esk and the arch out of the coulee.
```

- [ ] **Step 7: Esk's hand-in copy.** In `_datafiles/world/dogmud/dialogue/pothole_coulee/9107.yaml` replace:

```yaml
# R17: the end of Esk's Road (quest 89), offered by Cleric Hadwen after the
# rite. Crossing the arch (room 468) also closes it, through the quest's own
# room_enter trigger, so a player who crosses first is never left holding it.
- key: road_esk_end
  kind: handin
  quest: 89
  label: Esk's Road
  playerLine: Cleric Hadwen sent me. I'm ready to see the far side.
  text: Hadwen sends me the ones who are ready. The far side is Thornwall, temple
    stone and city noise, and every road in the Marches starts there. When you want
    to cross, tell me you are ready, and the arch will do the rest.
```

with:

```yaml
# The end of Esk's Road (quest 89). Crier Toke offers it once all eight
# bridges are in (#375 phase 7); an older save took it from Cleric Hadwen.
# The hand-in is its only end. Crossing the arch does not close it.
- key: road_esk_end
  kind: handin
  quest: 89
  label: Esk's Road
  playerLine: I was sent to you. I'm ready to see the far side.
  text: Word of you reached me before you did. The far side is Thornwall,
    temple stone and city noise, and every road in the Marches starts there.
    When you want to cross, tell me you are ready, and the arch will do the
    rest.
```

- [ ] **Step 8: Tests that pinned the old offer.** In `internal/dialogue/hadwen_midrite_test.go`, if the line `require.Contains(t, ids, "road_esk")` is present, replace it with:

```go
		require.NotContains(t, ids, "road_esk", "Esk's Road moved to Crier Toke (#375 phase 7)")
```

Then grep the Go tests for anything else pinned to the old wiring and fix each to the new behaviour:

```bash
grep -rn "89-end\|89-start\|road_esk\|room=468\|Esk's Road" --include=*_test.go internal . | grep -v esk_road_test.go
```

In particular, if `internal/guards/esk_return_portal_test.go` still has a test (plan 2 may have rewritten `TestEskPassageClosesTwoRoads`) asserting that crossing the arch grants a quest-89 token, change its expectation: a player holding `89-start` who crosses keeps `89-start` and gains no `89-end`, because the hand-in is the only end. Read the test in full before editing it.

- [ ] **Step 9: Backlog lines.** In `_datafiles/world/dogmud/quests/loop-backlog.txt` delete these lines if present:

```
chain 30 mob 9100 topic road_esk
end 89 quest 89 trigger room_enter room=468 grants=89-end
```

- [ ] **Step 10: Run green and run the loop sweep for the first time**

```bash
go test ./internal/dialogue/ -count=1
go test ./internal/guards/ -count=1
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . -count=1 -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL|SKIP)|ok|FAIL)'
```

Expected: all PASS, and the `-v` lines show `--- PASS: TestSmoke_ShippedQuestsPassTheLoop` (and the other sweeps) with no `--- SKIP`. The sweeps skip unless `DOGMUD_BOOT_SMOKE=1` is set (`quest_loop_smoke_test.go:28`), so a SKIP means the run proved nothing: rerun from Git Bash with the variable set. The loop sweep reports any finding on quests 89 and 97 to 101 as fresh (not listed), and any backlog line no finding matches as stale. Fix fresh findings in content (never add a backlog line). For a stale line, confirm its finding is gone and delete the line. If a `walk-destination` finding names a step of 98 to 101, give that step an explicit `destination:` block.

- [ ] **Step 11: Commit**

```bash
git add _datafiles/world/dogmud/quests/89-esks_road.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9107.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/dialogue/esk_road_test.go internal/dialogue/hadwen_midrite_test.go
git status --short
```

Add, by name, any of these that Steps 4 and 8 changed: `_datafiles/world/dogmud/behaviors/pothole_coulee/9100-cleric_hadwen.yaml`, `internal/guards/esk_return_portal_test.go`, and any test file Step 8 fixed. Then:

```bash
git commit -F - <<'EOF'
feat(coulee): Esk's Road moves to Toke behind all eight bridges

#375 phase 7, Pothole Coulee plan 4. Crier Toke offers quest 89 once every
Report to Toke bridge (90 to 97) is handed in; Cleric Hadwen's road_esk
offer is retired (topic key moved to # retired) and his opener no longer
names it. Warden Esk's hand-in is the quest's only end: the room_enter 468
finisher is gone, with its backlog line, and the chain 30 line Hadwen's
offer raised. Esk's passage topic still opens the arch for any Opened
player, with or without quest 89; a test holds both cases and Toke's gate.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Toke and Esk flavour lines (Opus)

**Files:**
- Create: `_datafiles/world/dogmud/behaviors/pothole_coulee/9106-crier_toke.yaml`, `internal/behaviortree/coulee_flavour_trees_test.go`
- Modify: `_datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml`

- [ ] **Step 1: Write the failing test**

`internal/behaviortree/coulee_flavour_trees_test.go`:

```go
package behaviortree

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (Pothole Coulee pass, owner ruling 2026-10-30): once a player
// holds all eight "Report to Toke" bridges (90-end to 97-end), Crier Toke and
// Warden Esk greet them on arrival, one time in four, with a line about one
// teacher's repeatable work. The line goes to that player alone (respond
// with user_text only), and a player missing any bridge hears nothing.

type flavourCall struct{ user, room string }

func recordFlavour(t *testing.T) *[]flavourCall {
	t.Helper()
	var calls []flavourCall
	orig := actionRegistry["respond"]
	actionRegistry["respond"] = func(params map[string]any, ctx *EvalContext) Result {
		calls = append(calls, flavourCall{getStringParam(params, "user_text"), getStringParam(params, "room_text")})
		return Success
	}
	t.Cleanup(func() { actionRegistry["respond"] = orig })
	return &calls
}

func loadCouleeTree(t *testing.T, mobId int, file string) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "pothole_coulee", file)
	require.NoError(t, GetEngine().LoadTree(mobId, path))
	t.Cleanup(func() { GetEngine().EvictTree(mobId) })
}

func seedBridgeUser(t *testing.T, userId int, lastBridge int) {
	t.Helper()
	u := users.NewTestUser(userId, "bridge", "Bridge", 0)
	u.Character.QuestProgress = map[int]string{}
	for q := 90; q <= lastBridge; q++ {
		u.Character.QuestProgress[q] = "end"
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
}

func TestCouleeFlavourTrees_SpeakOnlyToAPlayerWithEveryBridge(t *testing.T) {
	for _, tc := range []struct {
		mobId  int
		file   string
		givers []string
	}{
		{9106, "9106-crier_toke.yaml", []string{"Garve", "Falv", "Orrin", "Dace"}},
		{9107, "9107-warden_esk.yaml", []string{"Ovell", "Delk", "Sere", "Bryn"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			calls := recordFlavour(t) // before the load, so the tree's nodes call it
			loadCouleeTree(t, tc.mobId, tc.file)
			DrainAllDelayedActionsForTest(t)
			seedBridgeUser(t, 8601, 97) // all eight
			seedBridgeUser(t, 8602, 96) // missing 97-end
			state := NewBehaviorState()
			enter := func(userId int) {
				GetEngine().EvaluateEvent(tc.mobId, tc.mobId, EventContext{EventType: "player_enter", UserId: userId}, state)
			}

			for i := 0; i < 400; i++ {
				enter(8602)
			}
			DrainAllDelayedActionsForTest(t)
			require.Empty(t, *calls, "a player missing a bridge hears nothing")

			for i := 0; i < 400; i++ {
				enter(8601)
			}
			DrainAllDelayedActionsForTest(t)
			require.NotEmpty(t, *calls, "one arrival in four speaks")
			require.Less(t, len(*calls), 400, "not every arrival speaks")

			heard := map[string]bool{}
			for _, c := range *calls {
				require.Empty(t, c.room, "the line goes to the player alone")
				named := ""
				for _, g := range tc.givers {
					if strings.Contains(c.user, g) {
						named = g
					}
				}
				require.NotEmpty(t, named, "each line names a teacher: %q", c.user)
				heard[named] = true
			}
			require.Len(t, heard, len(tc.givers), "every line can be picked")
		})
	}
}
```

- [ ] **Step 2: Run and see it fail.** `go test ./internal/behaviortree/ -run TestCouleeFlavourTrees -count=1`. Expected: the Toke subtest FAILS loading `9106-crier_toke.yaml` (no such file); the Esk subtest FAILS on "one arrival in four speaks".

- [ ] **Step 3: Toke's tree.** Write `_datafiles/world/dogmud/behaviors/pothole_coulee/9106-crier_toke.yaml`:

```yaml
# Crier Toke (9106), Hub Square (5203).
#
# #375 phase 7 (Pothole Coulee pass, owner ruling 2026-10-30): once a player
# has handed in all eight "Report to Toke" bridges (quests 90 to 97), Toke
# now and then greets them on arrival with a line about one teacher's
# standing work. One arrival in four. The line goes to that player alone:
# respond with user_text only, and an empty room_text sends nothing to the
# room (actRespond, internal/behaviortree/actions_dialogue.go). One line is
# picked by the random-decorator cascade Barmaid Dal's tree uses (25, then
# 33, then 50, then the last), which gives each line an even share. Esk's
# tree names the other four teachers.
#
# A per-mob tree that fails hands the event on, so every other arrival and
# every other event still reaches his noncombat_questgiver archetype.

tree:
  type: selector
  children:

    # ── FLAVOUR: a teacher's standing work, for a player with every bridge ──
    - type: sequence
      event: player_enter
      children:
        - type: condition
          check: player_has_quest
          quest: "90-end"
        - type: condition
          check: player_has_quest
          quest: "91-end"
        - type: condition
          check: player_has_quest
          quest: "92-end"
        - type: condition
          check: player_has_quest
          quest: "93-end"
        - type: condition
          check: player_has_quest
          quest: "94-end"
        - type: condition
          check: player_has_quest
          quest: "95-end"
        - type: condition
          check: player_has_quest
          quest: "96-end"
        - type: condition
          check: player_has_quest
          quest: "97-end"
        - type: condition
          check: random_chance
          percent: 25
        - type: selector
          children:
            - type: decorator
              mod: random
              percent: 25
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} catches your eye as you come in. "Garve was asking
                  after you. The wash has filled up with bandits again, and he
                  pays whoever thins them out."
            - type: decorator
              mod: random
              percent: 33
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} calls to you with the low mouth. "Falv says the fen
                  kit is running low. Go and see him if you have a brew in
                  you."
            - type: decorator
              mod: random
              percent: 50
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} leans in. "Keeper Orrin says the grove has drifted
                  again. Warped things creep back whenever nobody keeps
                  watch."
            - type: action
              do: respond
              user_text: >-
                {actor} grins at you with both mouths. "Dace has a fresh purse
                in that dummy's coat, she tells me. Your fingers could use the
                practice."
```

- [ ] **Step 4: Esk's branch.** Append this block to the end of `_datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml` (after any trailing comment; the four-space indent makes it a new child of `tree.children`):

```yaml

    # ── FLAVOUR (#375 phase 7, owner ruling 2026-10-30): a player who holds
    #    all eight "Report to Toke" bridges (90-end to 97-end) is greeted, one
    #    arrival in four, with a line about one teacher's standing work, to
    #    that player alone (respond with user_text only). The cascade (25,
    #    33, 50, last) gives each line an even share. Toke's tree
    #    (9106-crier_toke.yaml) names the other four teachers. ──
    - type: sequence
      event: player_enter
      children:
        - type: condition
          check: player_has_quest
          quest: "90-end"
        - type: condition
          check: player_has_quest
          quest: "91-end"
        - type: condition
          check: player_has_quest
          quest: "92-end"
        - type: condition
          check: player_has_quest
          quest: "93-end"
        - type: condition
          check: player_has_quest
          quest: "94-end"
        - type: condition
          check: player_has_quest
          quest: "95-end"
        - type: condition
          check: player_has_quest
          quest: "96-end"
        - type: condition
          check: player_has_quest
          quest: "97-end"
        - type: condition
          check: random_chance
          percent: 25
        - type: selector
          children:
            - type: decorator
              mod: random
              percent: 25
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} looks up from his whittling. "Ovell asked after you.
                  The forge on the slope still wants a steady hand."
            - type: decorator
              mod: random
              percent: 33
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} nods to you. "Delk was through. Pronghorn are running
                  on the fringe again, and he wants someone to bring one down."
            - type: decorator
              mod: random
              percent: 50
              child:
                type: action
                do: respond
                user_text: >-
                  {actor} turns his ember eyes your way. "Sere says the stones
                  in the Reliquary have more to tell. She waits for whoever
                  will listen."
            - type: action
              do: respond
              user_text: >-
                {actor} speaks without rising. "Bryn is drilling at the canyon
                mouth again. The eye drifts if you stop using it, he says."
```

If the file still ends with the comment `# Quest 29 "Two Roads" closes on arrival through its own room_enter` and its next line, replace those two lines with `# No quest closes on arrival in 468: Esk's Road (89) ends at Esk's hand-in.`

- [ ] **Step 5: Run green, prove it can fail, run the Esk tree's other tests, commit.** Rerun Step 2: PASS. Change Toke's `percent: 25` on the `random_chance` to `100`, confirm "not every arrival speaks" fails, restore. Delete the `97-end` condition from Toke's tree, confirm "a player missing a bridge hears nothing" fails, restore.

```bash
grep -nE ' -- |—|–' _datafiles/world/dogmud/behaviors/pothole_coulee/9106-crier_toke.yaml
```

Expected: no output.

```bash
go test ./internal/behaviortree/ -count=1
go test ./internal/guards/ -count=1
git add _datafiles/world/dogmud/behaviors/pothole_coulee/9106-crier_toke.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9107-warden_esk.yaml internal/behaviortree/coulee_flavour_trees_test.go
git commit -F - <<'EOF'
feat(coulee): Toke and Esk point finished students at standing work

#375 phase 7, Pothole Coulee plan 4 (owner ruling 2026-10-30). A new tree
for Crier Toke and a new branch in Warden Esk's: on player_enter, a player
holding all eight Report to Toke bridges hears, one arrival in four, a line
about one teacher's repeatable work, to them alone (respond, user_text
only). Four lines each, picked evenly by the random-decorator cascade
Barmaid Dal uses; between them they name all eight teachers. Other events
still reach the questgiver archetype. Test proven red on the chance and on
a missing bridge.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: close out the zone's backlog (Sonnet)

**Files:**
- Create: `internal/guards/coulee_backlog_guard_test.go`
- Modify (only if the guard names a line): `_datafiles/world/dogmud/quests/loop-backlog.txt`

- [ ] **Step 1: Write the guard**

`internal/guards/coulee_backlog_guard_test.go`:

```go
package guards

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
)

// The #375 phase 7 Pothole Coulee pass (spec
// docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md,
// "Done when") cleared every loop finding on its quests. This guard keeps it
// that way: no loop-backlog.txt line may name quest 28 to 59, 76, 88, 89 or
// 90 to 101. A finding on one of them is fixed in content, never listed.

func couleeQuest(id int) bool {
	return (id >= 28 && id <= 59) || id == 76 || id == 88 || id == 89 || (id >= 90 && id <= 101)
}

// couleeBacklogLines returns the backlog lines ("rule questId where") that
// name a Coulee quest.
func couleeBacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && couleeQuest(id) {
			out = append(out, line)
		}
	}
	return out
}

func TestCouleeQuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := couleeBacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a Pothole Coulee quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

// TestCouleeQuestsHaveNoBacklogLines_GuardCanFail proves the guard sees a
// Coulee line and ignores another zone's.
func TestCouleeQuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\n" +
		"end 89 quest 89 trigger room_enter room=468 grants=89-end\n" +
		"walk-dead-end 101 step lifted\n" +
		"chain 73 mob 9465 topic q74_grant\n" +
		"one-action 7 mob 89 topic dornhandoff:prev\n")
	if got := couleeBacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d Coulee lines, want 2 (quests 89 and 101): %v", len(got), got)
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/guards/ -run TestCouleeQuestsHaveNoBacklogLines -count=1 -v`. Expected: `_GuardCanFail` PASSES. `TestCouleeQuestsHaveNoBacklogLines` passes if plans 2 and 3 and Task 7 cleared every line; if it lists survivors, each one is a finding still standing or a stale line:
  - stale (the loop sweep from Task 7 Step 10 reported it stale): delete the line;
  - a standing finding on a quest this plan owns (89, 97 to 101): fix the content;
  - a standing finding on a quest plans 2 or 3 owned (28 to 59, 76, 88): STOP and report to the controller with the line and the sweep's message. Do not edit their quests from this plan without the controller's go-ahead.

- [ ] **Step 3: Cross-check with grep (the brief's loud check).** Run on its own:

```bash
grep -nE '^[a-z-]+ (2[89]|3[0-9]|4[0-9]|5[0-9]|76|88|89|9[0-9]|100|101) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

Expected: no output. Prove the pattern can match: `echo "end 89 quest 89 x" | grep -nE '^[a-z-]+ (2[89]|3[0-9]|4[0-9]|5[0-9]|76|88|89|9[0-9]|100|101) '` prints the line.

- [ ] **Step 4: Prove the guard red on real data, then commit.** Temporarily append `end 89 quest 89 probe` to `loop-backlog.txt` with the Edit tool, run the guard and see it fail naming that line, remove it with the Edit tool, rerun green.

```bash
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop' . -count=1 -v 2>&1 | grep -E '^--- |^ok|^FAIL'
```

Expected: `--- PASS: TestSmoke_ShippedQuestsPassTheLoop` and `ok`; a `--- SKIP` means the variable was not set and the run proved nothing.

```bash
git add internal/guards/coulee_backlog_guard_test.go
git status --short _datafiles/world/dogmud/quests/loop-backlog.txt
```

If the backlog changed in Step 2, add it by name too. Then:

```bash
git commit -F - <<'EOF'
test(guards): no backlog line may name a Pothole Coulee quest

#375 phase 7, Pothole Coulee plan 4. The pass cleared every loop finding on
quests 28 to 59, 76, 88, 89 and 90 to 101; this guard fails if a line naming
one of them comes back, so the zone stays closed as later passes edit the
shared backlog. A fixture proves it sees a Coulee line and ignores another
zone's.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: docs (Sonnet)

**Files:**
- Modify: `internal/questengine/context.md`, `internal/behaviortree/context.md`, `internal/hooks/context.md`, `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/PATCH_NOTES.md`, `docs/README.md`

- [ ] **Step 1: `internal/questengine/context.md`.** Replace:

```
  only when the player ends up hidden (already hidden counts).
```

with:

```
  only when the player ends up hidden (already hidden counts). `hood`
  names the lantern as `ItemId` and is sent only when the hood closed;
  `picklock` is sent when a lock opens, aimed at the room container by name
  (`target: container`, `noun:`) or, for an exit, at nothing with the exit's
  name as `Noun`; `ambush` is sent once for a player's opening strike from
  hiding, melee (`hooks.applyCombatProgression` on
  `AttackResult.WasSurpriseAttack`) or a same-room shot from cover
  (`FireResult.Revealed` in `usercommands/shoot.go`), aimed at the target,
  landed or not (#375 phase 7). `steal` and `plant` are sent on the attempt,
  before the contest: a step that needs the theft to land listens on
  `item_gain` of the item (quest 99).
```

- [ ] **Step 2: `internal/behaviortree/context.md`.** Replace:

```
| `give_item_multiple` | `item_id` (int), `count` (int, default 1) | Gives N copies to player. |
```

with:

```
| `give_item_multiple` | `item_id` (int), `count` (int, default 1) | Gives N copies to player. |
| `stock_item` | `item_id` (int), `max` (int, default 1) | The acting mob gains one copy through the acquire seam while it carries fewer than `max`; fails, changing nothing, otherwise, so a line after it speaks only on a refill (#375 phase 7: the Pocket Dummy 9127 refills its purse). |
```

and replace:

```
| `give_item_multiple` | No |
```

with:

```
| `give_item_multiple` | No |
| `stock_item` | No |
```

- [ ] **Step 3: `internal/hooks/context.md`.** Insert before the line `### Resumed quests (`Quest_HandleQuestUpdate.go`, #375 phase 5)`:

```
### The ambush quest event (`NewRound_DoCombat_unified.go`, #375 phase 7)

`applyCombatProgression` raises `command ambush` (`notifyAmbushQuest`) when
a player's round carries `AttackResult.WasSurpriseAttack`, aimed at the
defender, landed or not. It is the only place the fact survives to the
progression phase (see `surpriseCandidate`). A shot from cover raises the
same event from `usercommands/shoot.go`. Quest 100 (From the Dark) listens
for it.

```

- [ ] **Step 4: Roadmap.** In `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, in the `## Phase 7: Content migration, zone by zone` section, replace its `**Status:**` line (on master `**Status:** not started.`; plans 1 to 3 may have changed it) with:

```
**Status:** in progress. Pothole Coulee (with the Newcomer Antechamber) is
built across PRs 1 to 4 of
`docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md`
(engine and config, spine, the seven lines, the Shadows line and leaving)
and is done pending its adversarial playtest (plan 5). No `loop-backlog.txt`
line names a Coulee quest (`internal/guards/coulee_backlog_guard_test.go`).
The other zones are not started.
```

- [ ] **Step 5: Patch notes.** Read the top of `docs/PATCH_NOTES.md`. Insert a new entry above the newest one, dated the day the PR is opened (PowerShell `Get-Date -Format yyyy-MM-dd`). Leave entries PRs 1 to 3 already wrote untouched; drop a bullet below if one of those entries already says the same thing, and drop any bullet whose change did not ship (check `git log --oneline origin/master` and the merged files).

```
## <YYYY-MM-DD>: Pothole Coulee, one teacher at a time

- Every line of lessons in Pothole Coulee now starts with Crier Toke in Hub
  Square. Walk the town with him first, and he can send you to any of eight
  teachers.
- Each line runs from one teacher to the next, one quest at a time. When a
  line is done, its last teacher sends you back to tell Toke.
- New: the Shadows line. Lamplighter Gethin, at the Mine Head, teaches you
  to carry a hooded lantern and to close its hood. Cutpurse Dace, on the
  Reed Jetty, teaches you to hide, to lift a purse and slip it back, to pick
  a lock, and to strike from hiding. Her practice dummy is there for anyone
  who wants to keep their fingers quick.
- When all eight teachers have sent you back, Toke points you to Warden Esk
  and the arch out of the coulee. You can still leave whenever you like:
  ask Cleric Hadwen whether there is another way out.
- The creatures of the old mine now keep to the deeper workings, so the
  Mine Head and the first dark drift are quiet.
```

- [ ] **Step 6: README row.** If `docs/README.md` has no row for this plan, add one after the row for `superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md` (or after the last `2026-10-30-p7-coulee-` plan row, if plans 1 to 3 added theirs):

```
| [`superpowers/plans/2026-10-30-p7-coulee-4-shadows-and-leaving.md`](superpowers/plans/2026-10-30-p7-coulee-4-shadows-and-leaving.md) | Plan 4 of the Coulee pass in 13 tasks: quest events for `hood`, `picklock` and a strike from hiding; the `stock_item` action; Gethin, Dace, the Pocket Dummy and the jetty lockbox; quests 97 to 101; Esk's Road moved to Toke behind all eight bridges; Toke and Esk flavour trees; a guard that keeps the zone's backlog empty; docs, playtest, gate |
```

- [ ] **Step 7: Check and commit**

```bash
grep -nE '—|–| -- ' docs/PATCH_NOTES.md | head -5
```

Expected: no line from the new entry.

```bash
git add internal/questengine/context.md internal/behaviortree/context.md internal/hooks/context.md docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md docs/PATCH_NOTES.md docs/README.md
git commit -F - <<'EOF'
docs: Coulee plan 4 events, stock_item, roadmap and patch notes

#375 phase 7, Pothole Coulee plan 4. questengine context lists the hood,
picklock and ambush command events and the attempt-time steal and plant;
behaviortree context documents stock_item; hooks context notes where the
ambush event is raised. The roadmap records Pothole Coulee as done pending
its playtest, and the patch notes carry the player-facing entry.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: focused playtest of the Shadows line and the way out (Opus)

**Files:** a goals file in the session scratchpad (not committed); fixes, if any, to files this plan created.

The full adversarial playtest of the whole flow is plan 5. This one checks this PR's content before it goes up.

- [ ] **Step 1: Load the `dogmud-playtesting` skill** and follow it: restore the harness if missing, use private ports, kill only the server you start, by PID, never the owner's.

- [ ] **Step 2: Write an ephemeral goals file** in the session scratchpad with `--checkout C:/tmp/dogmud-375-p7-coulee-shadows` and two naive-newbie seats, each a fresh character set up with the harness's quest-token fixture:
  - Seat A, tokens `30-end`, `31-end`: talk to Toke, take First Light, walk to the Mine Head, and play 98, 99, 100, 101 and 97 to the end using only the hints. Read every line. Report any step whose hint does not get a new player through it, any refusal the hint did not prepare them for (dark aiming, being spotted, the lockbox already open, broken picks), and the danger of the talus walk and the brute fight.
  - Seat B, tokens `30-end`, `31-end` and `90-end` to `97-end`: enter Hub Square and the Threshold House at least twelve times each and record every flavour line; take Esk's Road from Toke, hand it in at Esk, cross with `passage`. Then, on a second character with `30-end` only, confirm Esk's passage still works and Toke shows no Esk's Road.

- [ ] **Step 3: Run, read the report, act.** Fix every defect in this plan's own content in the branch (rerun the affected task's checks and commit by name with a `fix(coulee):` message and the attribution line). File anything outside this plan's files as a GitHub issue with `gh issue create --repo pruuk/DOGMud`, searching first with `gh issue list --repo pruuk/DOGMud --search "<words>"`. Reports are gitignored; the issues are the record.

---

### Task 12: Gate (Opus)

**Files:** none new.

- [ ] **Step 1: Format, build, tests**

```bash
cd C:/tmp/dogmud-375-p7-coulee-shadows
gofmt -l internal/ modules/
```

Expected: no output.

```bash
go build ./...
go test ./internal/usercommands/ ./internal/hooks/ ./internal/behaviortree/ ./internal/dialogue/ ./internal/questengine/ ./internal/questloop/ ./internal/quests/ ./internal/actions/ -count=1
go test ./internal/guards/... -count=1
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . -count=1 -v 2>&1 | grep -E '^--- |^ok|^FAIL'
go test . -count=1
```

Expected: all PASS. The boot-smoke run must list each sweep as `--- PASS` (for example `--- PASS: TestSmoke_ShippedQuestsPassTheLoop`, `--- PASS: TestSmoke_ShippedDialoguePassesTheValidators`) and end in `ok`; any `--- SKIP` means `DOGMUD_BOOT_SMOKE=1` did not reach the test (Git Bash syntax) and proves nothing. (`-race` cannot run on this machine, which has no C compiler; CI runs it. `internal/rooms` zone-lifecycle tests may fail locally under `DOGMUD_BOOT_SMOKE`; they are known Windows-only failures, `dogmud-writing-tests`.)

- [ ] **Step 2: Lint**

```bash
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `0 issues.`

- [ ] **Step 3: Isolated boot check** (from `dogmud-shipping`; never `go run`; never the owner's ports). `S` is the session scratchpad directory, full path.

```bash
S="<session scratchpad directory, full path>"
git worktree list | grep -q dogmud-boot-check && git worktree remove --force C:/tmp/dogmud-boot-check
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
grep -n "Pothole Coulee\|quest 9[7-9]\|quest 10[01]\|quest 89" "$S/boot.log" | grep -i "warn" | head
cd C:/tmp/dogmud-375-p7-coulee-shadows && git worktree remove --force C:/tmp/dogmud-boot-check
```

Expected: `exit 124` (the timeout fired because the server stayed up), `0` panics, `1` Server Ready, and no warning naming quests 89 or 97 to 101. If `git worktree remove` fails on a Windows lock, use PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check` and then `git worktree prune`.

- [ ] **Step 4: Push and open the PR.** Every `gh` command carries `--repo pruuk/DOGMud`. Write the body to the scratchpad with the Write tool, then:

```bash
git push -u origin feat/375-p7-coulee-shadows
gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-coulee-shadows --title "feat(coulee): #375 phase 7 Coulee plan 4, the Shadows line and leaving" --body-file "<scratchpad>/pr-body.md"
```

Confirm the URL `gh` prints says `pruuk/DOGMud`. The body (no closing keyword anywhere; "Part of" only):

```
## Summary

#375 phase 7, Pothole Coulee pass, PR 4 of 5: the Shadows line and leaving.

- **Shadows line (new).** Lamplighter Gethin (9125) at the Mine Head and Cutpurse Dace (9126) on the Reed Jetty. Quests 98 First Light, 99 Quiet Hands, 100 From the Dark, 101 Quick Fingers (repeatable) and the bridge 97 Report to Toke (Shadows), one action per step.
- **Practice targets.** The Pocket Dummy (9127) carries a Sand Purse (item 60) and refills it through a new `stock_item` behaviour action; a trap-free lockbox on the jetty relocks after a short while.
- **Engine.** `hood`, `picklock` and a strike from hiding (melee and a shot from cover) now raise quest command events, each with a probe-quest test proven red.
- **Mine.** The crawler and brute spawns at the Mine Head and in the Timbered Drift moved deeper, so Gethin's post and the first dark room are quiet.
- **Leaving.** Quest 89 Esk's Road moves from Cleric Hadwen to Crier Toke, gated on all eight bridges (90 to 97). Esk's hand-in is its only end; the arch-crossing finisher and its backlog line are gone. Esk's passage still opens the arch for any Opened player.
- **Flavour.** Toke (new tree) and Esk (new branch) greet a player who holds all eight bridges with a line about one teacher's repeatable work, to that player alone.
- **Close-out.** No `loop-backlog.txt` line names quests 28 to 59, 76, 88, 89 or 90 to 101, held by a new guard test.
- **Docs.** context.md for questengine, behaviortree and hooks; roadmap status; patch notes.

Decisions recorded in the plan for review: the plant step counts a caught attempt (plant raises its event before the contest), and the ambush event fires on the opener whether or not it lands.

Part of #315. Part of #375. The whole-flow adversarial playtest is PR 5.

## Test plan

- [x] gofmt clean, go build, package tests (usercommands, hooks, behaviortree, dialogue, questengine, questloop, quests, actions)
- [x] go test ./internal/guards/...
- [x] DOGMUD_BOOT_SMOKE=1 quest loop, door and dialogue sweeps
- [x] golangci-lint --new-from-merge-base=origin/master: 0 issues
- [x] isolated boot check: Server Ready, no panic, no warning for quests 89, 97 to 101
- [x] focused two-seat playtest of 97 to 101 and Esk's Road; findings fixed or filed

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

- [ ] **Step 5: Watch CI.** `gh pr checks <n> --repo pruuk/DOGMud --watch`, then `gh run list --repo pruuk/DOGMud --branch feat/375-p7-coulee-shadows` to confirm every expected workflow ran; read failures with `gh run view <id> --repo pruuk/DOGMud --log-failed`. Report the PR link to the controller. The owner deploys; do not deploy. After merge, remove the worktree: `git worktree remove C:/tmp/dogmud-375-p7-coulee-shadows`.
