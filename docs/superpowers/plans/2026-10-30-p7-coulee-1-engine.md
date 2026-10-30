# #375 Phase 7 Coulee PR 1: Engine and Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the engine and config half of the Pothole Coulee pass: the `StealMinRank` knob (shipped at 1), `quaff` for potions with `eat` and `drink` as one food command, cooking drinks retyped, the #697 narration order, the #690 engine nits (escaped `<name>`, `consider` copy, the Straw Effigy), and the menu-close notice (owner ruling (a)).

**Architecture:** The potion body `actions.Drink` is renamed `actions.Quaff` and reached by a new `quaff` command (player, mob, AI companion); its quest notify becomes command `quaff` and moves after the "You drink" line. `eat` and `drink` both run one food body (`consumeFood` in `internal/usercommands/eat.go`) that takes subtype `edible`, which every food and every cooking drink carries; a potion aimed at it consumes nothing and gets one refusal line naming `quaff`. Steal's four hardcoded rank-2 gates read `Balance.StealMinRank` through the existing `theftAttempt`. The menu close for a typed command speaks through the existing `menu.InterruptLine`.

**Tech Stack:** Go 1.x, YAML content under `_datafiles/world/dogmud/`, one inline JS edit in `_datafiles/html/public/webclient-pure.html`.

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md

**Branch / worktree:** branch `feat/375-p7-coulee-engine`, worktree `C:/tmp/dogmud-375-p7-coulee-engine`, off `origin/master` (verified at `8432192f1`). Remove the worktree with `git worktree remove` once the PR merges.

---

## Owner check before execution (one item)

**Lowering the steal gate to rank 1 turns on mob theft that is dormant today.** `Character.Validate` raises every skill to at least rank 1 for players and mobs (`internal/characters/validate.go:761`, `ensureAllSkills`; mobs call it at `internal/mobs/mobs.go:658`). The five mobs on the `thief` behaviour archetype (`behaviors/archetypes/thief.yaml:88` runs `try_steal`) sit at skullduggery rank 1: Market Cutpurse 9190 (Hartcharn), Dock Thug 9318 (New Plymouth docks), A Cutpurse 9210 and Quill the Tout 9211 (New Plymouth outskirts), Thornwall Highwayman 90. Under today's rank-2 gate every one of their thefts is refused ("not advanced enough", silently, since a mob's `SendText` is a no-op). With `StealMinRank: 1`, shared with mobs as the spec rules, they start lifting gold from players. The spec rules the knob is shared; it does not say this effect was intended. This plan ships the shared knob as ruled and says so in the patch notes (Task 14). If the owner wants mob thieves to stay dormant, the clean option is to raise those five mobs to rank 2 or remove their theft branch, in a follow-up content change; it does not change this plan.

## Spec deviations found

1. **Backlog key renames with the trigger.** Quest 55's end trigger is a listed finding: `end 55 quest 55 trigger command command=drink grants=55-end` (`quests/loop-backlog.txt:73`). Renaming its command to `quaff` changes the finding key, so the same PR must rewrite that line to `command=quaff` or boot refuses. This renames an existing finding; it adds none. Plan 3 deletes the line when it gives quest 55 its hand-in.
2. **Herbal Tea's toxicity.** Herbal Tea (30024) does not rely on aging or magnitude (no `aging:` block, no `magnitude:`), but it carries `toxicity: 3`, which only the potion body charges (`internal/actions/drink.go:309-311`); the food body applies `ConditionIds` only (`internal/usercommands/eat.go:89-91`). Retyping it drops that cost. This plan removes `toxicity: 3` from the file so the data says what the game does (no food or cooking drink carries toxicity: Spiced Wine 30025 and Antidote Broth 30027 have none). Owner may overrule by keeping the line and teaching the food body toxicity.
3. **The purge stops stripping Herbal Tea's condition.** Condition 44 (Clear Mind) is granted only by Herbal Tea (grep of every item file), so while the tea is `type: potion` it is in `items.PotionEffectConditionIds` and the Purging Draught strips it. Once the tea is a drink it leaves that set, like every meal's condition. Two tests pin 44 in the set (`internal/guards/potion_conditions_guard_test.go:63`, `internal/actions/drink_purge_shipped_test.go:57`); Task 6 moves 44 to their "not stripped" side.
4. **Cooking drinks take subtype `edible`, not `drinkable`.** The spec moves them to type `drink` and says `eat` and `drink` accept both `food` and `drink`. Subtype `drinkable` is what auto-routes an item into a potion bandolier (`internal/characters/inventory.go:213,647`) and what `quaff`, the companion and the bandolier lookup key on, so the three cooking drinks become `type: drink`, `subtype: edible`. The food body keeps keying on `edible`. A Herbal Tea a player already holds in a bandolier stays findable: the food body looks there second (Task 5).
5. **#690's escaped brackets are six files, not one.** `&lt;name&gt;` reaches the player literally on every port: the parser recognises only `<ansi ...>` and `</ansi>` and passes every other byte through (`internal/util/ansiescape.go:18-23`; `ansitags@v1.1.0/ansitags.go` has no entity decoding), and both telnet and the web client render through it (`internal/hooks/Message_SendMessages.go:33`, `internal/templates/templates.go:453-466`). `internal/usercommands/refusal_room_container_test.go:45-56` already pins raw brackets for the storage hint for exactly this reason. The same bug sits in `admin.ai.go:23`, `channel.go:31,81`, `guild.go:480`, `set.go:436,475,485,501` and `shoot.go:142`. Task 9 fixes all eleven literals and adds a guard.
6. **The `consider` verdict line has the same pronoun problem.** "They won't lay a finger on you." (`internal/actions/fight_read.go:164`) is what a straw dummy usually reads (untouched). Task 10 rewrites it with the pace lines.
7. **The Straw Effigy is neither a label bug nor a crit bug.** The crit line is chosen by the swing's quality: `pctDamage` is the hit against the attacker's own mean damage (`internal/combat/combat_helpers.go:1832-1834`), and a crit is forced to 101 so it reads from the critical pool (`combat_helpers.go:1791-1797`). The fight prompt label is the target's health share, `healthy` at 80% and above (`internal/users/userrecord.prompt.go:226-246`). The effigy carries vitality 100200 by design so a new player cannot drop it (`mobs/newcomer_antechamber/9614-straw_effigy.yaml:27-36`). Both readouts are true. The fix sits at the effigy: a per-mob behaviour tree that, on a share of hits (`mob_hurt`, fired per hit at `internal/hooks/NewRound_DoCombat_unified.go:856-871`), emotes that it settles back into shape, no worse for the blow. No engine change.
8. **Menu close line only for menus with a topic.** The ruling says the line names the conversation. Only talk menus set `Session.Topic` (`internal/conversation/talk.go:208`); a topic-less menu (a confirm, a builder list) has no conversation to name and stays silent. Typing `talk <same npc>` while that NPC's menu is open now prints the break-off line before the new menu opens; accepted.

## Facts verified against source (master `8432192f1`, 2026-10-30)

| Fact | Source |
|---|---|
| Four theft rank gates read `t.rank < 2`: creature, player, container, household bauble | `internal/actions/steal.go:258,427,533,843` |
| `theftAttempt` holds `char, rank, points, period`; built once in `newTheftAttempt(char, room, cfg)`; comment at 172 says "rank below 2"; comment at 826 says "rank 2 is required" | `internal/actions/steal.go:176-206,172,826` |
| Mob thieves reach the same body: `mobcommands.Steal` and the btree `try_steal` both call `actions.Steal` | `internal/mobcommands/steal.go:17-20`; `internal/behaviortree/actions_skullduggery.go:41-54` |
| Player `steal` and `plant` commands need rank 1 to be visible; `actions.Plant` has no rank gate | `internal/usercommands/skill.skullduggery.steal.go:33`; `skill.skullduggery.plant.go:25`; no `rank` in `internal/actions/plant.go` |
| Every character, player or mob, holds every skill at rank 1 or more after `Validate` | `internal/characters/character.go:459-480`, `validate.go:761`, `internal/mobs/mobs.go:658` |
| Skullduggery knobs are declared in `config.balance.go` 292-304; steal pocket bounds validated in `validateCombatSteal`, called from `validateCombat` | `internal/configs/config.balance.go:292-304`; `config.balance.combat.go:40-60,297` |
| `config.yaml` skullduggery block ends `StealHiddenBonus: 25 ...` at line 959; the block's header warns absent keys read as zero | `_datafiles/config.yaml:938-959` |
| Shipped-config test pattern: unmarshal `config.yaml` into `Config` with `gopkg.in/yaml.v2` | `internal/configs/config.balance.baubles_test.go:274-307` |
| Tests that assert a rank-1 refusal: four subtests and one household test | `internal/actions/steal_cooldown_test.go:46-83`; `household_bauble_test.go:170-174` |
| Steal help line 22 and plant help line 20 both say rank 2; steal line 5 and plant line 5 carry " -- " | `templates/help/steal.template:5,22`; `plant.template:5,20` |
| `actions.Drink(actor DrinkActor, rest string) DrinkResult` is the one potion body; bare prompt 185, busy 190, bandolier then drinkable-first backpack 204-215, non-drinkable refusal 224-229, quest notify 316-318 BEFORE "You drink the" at 320-321 | `internal/actions/drink.go:162-474` |
| Wrappers: `usercommands.Drink` and `mobcommands.Drink` call `actions.Drink` | `internal/usercommands/drink.go:12-15`; `internal/mobcommands/drink.go:11-14` |
| Registry: `drink` and `eat` both `{.., true, true, false}` at 119-120; `quests` at 187 | `internal/usercommands/usercommands.go:119-120,185-187` |
| Mob registry `"drink"` 42, `"eat"` 44, `"put"` 69 | `internal/mobcommands/mobcommands.go:42,44,69` |
| Player `Eat` body: edible-first, ingredient hint, spoiled refusal, narration "You eat some of the ...", conditions source `food` | `internal/usercommands/eat.go:19-96` |
| Mob `Eat`: first match, silent unless edible, room line "eats some" | `internal/mobcommands/eat.go:13-46` |
| Guard: drink wrappers must not apply rules; reads the two `drink.go` paths | `internal/guards/drink_wrapper_guard_test.go:14-25` |
| Guard: line-keyed allowlist entry `"internal/actions/drink.go|359"` (the `AddConditionMagnitude` call) | `internal/guards/condition_apply_path_guard_test.go:236`; `drink.go:359` |
| Guard: narration registry key `usercommands/eat.go|You eat some of the <ansi fg=\"itemname\">%s</ansi>.`; keys are file plus first literal capped at 80 runes | `internal/guards/messaging_surface_guard_test.go:1366,744-765` |
| Item types `Potion`, `Food`, `Drink` at 140-142; subtypes `Drinkable`, `Edible` at 159-160; subtype descriptions at 90-91 | `internal/items/itemspec.go:90-91,140-142,159-160` |
| Look text: "You could probably drink this." for drinkable, "eat" for edible | `internal/items/items.go:255-263` |
| Consumables census: 8 `food/edible`, 35 `potion/drinkable`, no `type: drink` item | grep of `items/` for `type:`/`subtype:` |
| Herbal Tea 30024 `potion/drinkable`, conditionids `[44]`, `toxicity: 3`, no aging, no magnitude; Spiced Wine 30025 and Antidote Broth 30027 `food/edible`, no toxicity | `items/consumables-30000/30024-herbal_tea.yaml:11-20`, `30025-spiced_wine.yaml:11-12`, `30027-antidote_broth.yaml:11-12` |
| Cooking recipe outputs: 30027, 30026, 30022, 30023, 30024, 40224 (Reduction Base, `object/mundane`), 30025, 30061, 30021; the drinks among them are 30024, 30025, 30027 | `recipes/cooking/*.yaml` output blocks |
| Condition 44 is named only by 30024 | grep `^\s*-\s*44\s*$` over `items/` (same pattern finds 42 in three files) |
| Bandolier auto-route keys on `Type == Potion` or `Subtype == Drinkable`; `SortPotionItems` keys on `Drinkable`/`Throwable` | `internal/characters/inventory.go:213,647` |
| `FindInPotions(name)` (oldest first) and `UseItemFromPotions(item) int` | `internal/characters/inventory.go:319-361` |
| Quest triggers on `command: drink`: quest 38 (176), quest 55 (76) | `quests/38-first_brew.yaml:174-185`; `quests/55-another_brew.yaml:75-84` |
| "Type drink healing salve" instructions: quest 38 at 79, 161, 172; quest 55 at 37, 62, 73; dialogue 9128 at 67, 173; dialogue 9129 at 89, 282 (10 lines) | grep |
| Loop backlog line for 55's drink end | `quests/loop-backlog.txt:73` |
| Shipped-YAML tests on those: `Command: "drink"` in `newbie_alchemy_steps_test.go:48,59,81,144,151`; topic hints `"type drink healing salve"` at `newbie_topic_steps_test.go:274,286` | `internal/questengine/` |
| Help: `drink.template`, `eat.template`, `cooking.template:22-27,41`, `alchemy.template:34-37,54`, `equipment.template:49`, `toxicity.template:60`; `help potions` aliases to `alchemy` | `templates/help/`; `keywords.yaml:267` |
| Help index `items:` list (drink, drop, eat, ... put, read ...) at 129-147 | `keywords.yaml:129-147` |
| Every registered command needs a help file and an index entry | `internal/usercommands/helpfile_completeness_test.go:83,235` |
| Other Go callers that drink a potion: survival planner `"drink " + potion`; companion reflex `act("drink " + ...)`; companion action `drink` issues `drink `+target for `Drinkable` | `internal/planners/survival.go:51`; `modules/aicompanion/combat.go:469-470`; `modules/aicompanion/actions.go:434-438` |
| Tab completion: `eat` searches `Edible`, `drink` searches `Drinkable` | `world.go:486-494` |
| Web client: `consumeVerb` maps potion and drink to `drink`, food to `eat`; tick placeholder "e.g. drink health;drink mana" | `_datafiles/html/public/webclient-pure.html:3567-3571,4070` |
| No mob idle command, schedule or behaviour tree issues `drink` or `eat` | grep of `behaviors/`, `mobs/`, `schedules/` |
| `hintNoQuestText` holds `talk &lt;name&gt;` in a command tag; the test asserts the escaped form | `internal/usercommands/hint.go:19`; `hint_test.go:22` |
| `consider` pace tables and `paceText` | `internal/actions/fight_read.go:163-211` |
| Pace copy test (80 columns, no digits); companion test pins "They won't lay a finger on you." | `internal/actions/fight_read_test.go:103-130`; `modules/aicompanion/aicompanion_test.go:506` |
| Straw Effigy 9614: `behavior_archetype: combat_passive`, vitality base 100200; no per-mob tree; Dewey's antechamber tree sits at `behaviors/newcomer_antechamber/9491-dewey.yaml` | mob file; `ls behaviors/newcomer_antechamber` |
| Per-mob tree path is `behaviors/<zone folder>/<mobid>-<name>.yaml`; per-mob first, archetype on Failure | `internal/behaviortree/helpers.go:51-57,152-169,228-283` |
| Tree test pattern: override `actionRegistry[...]`, `LoadTree`, `EvaluateEvent`, `EvictTree` | `internal/behaviortree/hadwen_rite_guard_test.go:29-36,64-72` |
| `InterruptLine` silent for `CloseOtherCommand`; `RouteMenuInput` closes with `announce=false` at 383 | `internal/menu/lines.go:34-54`; `internal/users/userrecord.menu.go:382-384` |
| Tests pinning the silence: `lines_test.go:23`; `userrecord_menu_test.go:73-83` checks the reason only | `internal/menu/`, `internal/users/` |
| Talk menus set `Topic: "talk with " + name` | `internal/conversation/talk.go:208` |
| Refusal helpers live in `internal/refusal`; `hint(cmd)` appends " Try <command>." and `echo` escapes typed text | `internal/refusal/refusal.go:30-59`; `carrying.go` |

## File map

| Path | Action | Responsibility |
|---|---|---|
| `internal/configs/config.balance.go` | modify | declare `StealMinRank` |
| `internal/configs/config.balance.combat.go` | modify | `defaultStealMinRank`, validate below 1 |
| `internal/configs/steal_min_rank_test.go` | create | shipped value, absent and bad values, help matches config |
| `_datafiles/config.yaml` | modify | ship `StealMinRank: 1` with comment |
| `internal/actions/steal.go` | modify | four gates read the knob |
| `internal/actions/steal_cooldown_test.go`, `household_bauble_test.go` | modify | pin min rank 2 where a refusal is the subject |
| `internal/actions/steal_min_rank_test.go` | create | the knob drives the gate, player and mob |
| `templates/help/steal.template`, `plant.template` | modify | real ranks, dashes out |
| `internal/usercommands/skill.skullduggery.steal.go` | modify | comments only |
| `internal/refusal/carrying.go`, `internal/refusal/potion_test.go` | modify, create | `QuaffPotion`, `NotAPotion` |
| `internal/actions/drink.go` | modify | `Drink` becomes `Quaff`; verb; edible refusal; notify after narration |
| `internal/actions/quaff_quest_order_test.go` | create | #697 order |
| `internal/actions/drink_*_test.go` (4 files) | modify | call `Quaff` |
| `internal/usercommands/quaff.go` | create | player `quaff` |
| `internal/usercommands/drink.go`, `eat.go` | modify | one food body for both verbs |
| `internal/usercommands/eat_drink_quaff_test.go` | create | the three verbs |
| `internal/usercommands/usercommands.go` | modify | register `quaff` |
| `internal/mobcommands/quaff.go` | create | mob `quaff` |
| `internal/mobcommands/drink.go`, `eat.go`, `mobcommands.go` | modify | mob food alias, verb by type, register |
| `internal/mobcommands/drink_wrapper_test.go` | rename to `quaff_wrapper_test.go` | call `Quaff` |
| `internal/guards/drink_wrapper_guard_test.go` | modify | guard the `quaff.go` wrappers |
| `internal/guards/condition_apply_path_guard_test.go`, `messaging_surface_guard_test.go` | modify | re-key moved entries |
| `items/consumables-30000/30024-herbal_tea.yaml`, `30025-spiced_wine.yaml`, `30027-antidote_broth.yaml` | modify | `type: drink`, `subtype: edible` |
| `internal/guards/potion_conditions_guard_test.go`, `internal/actions/drink_purge_shipped_test.go` | modify | 44 leaves the purge set |
| `internal/items/items.go`, `itemspec.go`, `items/longdesc_verb_test.go` | modify, create | look text names quaff, drink or eat |
| `internal/planners/survival.go`, `modules/aicompanion/combat.go`, `actions.go`, `aicompanion_test.go` | modify | companion and planner quaff |
| `world.go` | modify | tab completion |
| `quests/38-first_brew.yaml`, `55-another_brew.yaml`, `loop-backlog.txt` | modify | `command: quaff`, hints, backlog key |
| `dialogue/pothole_coulee/9128.yaml`, `9129.yaml` | modify | four hints |
| `internal/questengine/newbie_alchemy_steps_test.go`, `newbie_topic_steps_test.go` | modify | quaff |
| `templates/help/quaff.template` | create | help for quaff |
| `templates/help/drink.template`, `eat.template`, `cooking.template`, `alchemy.template`, `equipment.template`, `toxicity.template`, `keywords.yaml` | modify | verbs and index |
| `_datafiles/html/public/webclient-pure.html` | modify | right-click verb, tick placeholder |
| `internal/usercommands/hint.go`, `hint_test.go`, `admin.ai.go`, `channel.go`, `guild.go`, `set.go`, `shoot.go` | modify | raw angle brackets |
| `internal/guards/html_entity_guard_test.go` | create | no `&lt;`/`&gt;` in Go player text |
| `internal/actions/fight_read.go`, `fight_read_test.go` | modify | pronoun-free, no repeated pace |
| `behaviors/newcomer_antechamber/9614-straw_effigy.yaml`, `internal/behaviortree/effigy_tree_test.go` | create | the effigy rights itself |
| `internal/menu/lines.go`, `lines_test.go`, `internal/users/userrecord.menu.go`, `userrecord_menu_test.go` | modify | menu-close notice |
| `context.md` in actions, usercommands, mobcommands, items, configs, refusal, menu, users, planners, guards; `modules/aicompanion/context.md` | modify | API changes |
| `internal/guards/copy_no_dash_test.go` | modify | widen to the files this PR rewrote |
| `docs/PATCH_NOTES.md` | modify | player-facing entry |

All `_datafiles` content paths above are under `_datafiles/world/dogmud/` unless they start with `_datafiles/html` or `_datafiles/config.yaml`.

## House rules for every executor

- Edit files only with the Edit and Write tools. Never write a repo file from the shell (no `>`, `>>`, `sed -i`, Python `open(..., 'w')`); `guard.py` blocks it. `git mv` is fine.
- `git add` named paths only. Never `git add -A`, `.` or `--all`; never `git commit -a`; never `--no-verify`.
- Commit messages go through a quoted heredoc and end with the attribution line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes and no " -- " in any text you write, comments included.
- Every `gh` command carries `--repo pruuk/DOGMud`.
- Test binaries load Go defaults, never `config.yaml`. Pin a knob in a test when the test depends on it.
- Run commands from the worktree root `C:/tmp/dogmud-375-p7-coulee-engine` (Git Bash: `cd /c/tmp/dogmud-375-p7-coulee-engine`).

---

## Task 0: Worktree

**Executor:** Sonnet

- [ ] **Step 1: Create the worktree off fresh master**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/375-p7-coulee-engine C:/tmp/dogmud-375-p7-coulee-engine origin/master
cd /c/tmp/dogmud-375-p7-coulee-engine && git log --oneline -1
```

Expected: a commit at or after `8432192f1`. If master moved, re-run the grep in each task's first step before editing; line numbers below are from `8432192f1`.

- [ ] **Step 2: Baseline**

```bash
go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/menu/ ./internal/users/ ./internal/guards/ . 2>&1 | tail -20
```

Expected: all `ok`. Note any pre-existing failure before touching code.

---

## Task 1: `StealMinRank` knob

**Executor:** Sonnet

**Files:**
- Create: `internal/configs/steal_min_rank_test.go`
- Modify: `internal/configs/config.balance.go:295`, `internal/configs/config.balance.combat.go:40-60`, `_datafiles/config.yaml:959`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/steal_min_rank_test.go`:

```go
package configs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// repoFile anchors a path on this test file, not the process CWD.
func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(append([]string{filepath.Dir(here), "..", ".."}, parts...)...)
}

// shippedBalance reads the Balance block of the shipped config.yaml.
func shippedBalance(t *testing.T) Balance {
	t.Helper()
	data, err := os.ReadFile(repoFile(t, "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	return cfg.Balance
}

// StealMinRank (#375 phase 7) is the skullduggery rank every theft needs,
// players and mobs alike. It ships at 1: every character starts at rank 1.
// Absent (zero) or below 1 reads as 1, since rank 0 is no rank at all.
func TestStealMinRank_ShippedAndDefaulted(t *testing.T) {
	if got := shippedBalance(t).StealMinRank; got != 1 {
		t.Fatalf("shipped StealMinRank is %d, want 1", got)
	}
	if defaultStealMinRank != 1 {
		t.Fatalf("defaultStealMinRank is %d, want 1", defaultStealMinRank)
	}
	var absent Balance
	absent.validateCombatSteal()
	if absent.StealMinRank != 1 {
		t.Fatalf("an absent key reads %d, want 1", absent.StealMinRank)
	}
	negative := Balance{StealMinRank: -3}
	negative.validateCombatSteal()
	if negative.StealMinRank != 1 {
		t.Fatalf("a negative value reads %d, want 1", negative.StealMinRank)
	}
	kept := Balance{StealMinRank: 4}
	kept.validateCombatSteal()
	if kept.StealMinRank != 4 {
		t.Fatalf("a set value must be kept: %d", kept.StealMinRank)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/configs/ -run TestStealMinRank 2>&1 | tail -5
```

Expected: build failure `undefined: defaultStealMinRank` and `unknown field StealMinRank`.

- [ ] **Step 3: Declare the knob**

In `internal/configs/config.balance.go`, Edit:

old:
```go
	StealCooldown               ConfigInt   `yaml:"StealCooldown"`               // Steal/plant cooldown in real seconds (default 60)
```
new:
```go
	StealCooldown               ConfigInt   `yaml:"StealCooldown"`               // Steal/plant cooldown in real seconds (default 60)
	StealMinRank                ConfigInt   `yaml:"StealMinRank"`                // Skullduggery rank every theft needs, players and mobs alike (default 1; below 1 reads as 1)
```

- [ ] **Step 4: Default and validate it**

In `internal/configs/config.balance.combat.go`, Edit:

old:
```go
// validateCombatSteal bounds the pickpocket pause. Zero is an absent key,
// so it takes the default (a pause of nothing would give the roll away at
// once). Written `!(x > 0)` so a NaN from YAML `.nan` is caught too (see the
// warning on ContestGapSaturation below).
func (b *Balance) validateCombatSteal() {
```
new:
```go
// defaultStealMinRank is the skullduggery rank a theft needs (#375 phase 7,
// a hardcoded 2 until then). Every character starts at rank 1.
const defaultStealMinRank = 1

// validateCombatSteal bounds the theft rank gate and the pickpocket pause.
// StealMinRank below 1 (an absent key reads 0) takes the default: rank 0 is
// no skill at all. For the pause, zero is an absent key, so it takes the
// default (a pause of nothing would give the roll away at once). Written
// `!(x > 0)` so a NaN from YAML `.nan` is caught too (see the warning on
// ContestGapSaturation below).
func (b *Balance) validateCombatSteal() {
	if b.StealMinRank < 1 {
		b.StealMinRank = defaultStealMinRank
	}
```

- [ ] **Step 5: Ship it in config.yaml**

In `_datafiles/config.yaml`, Edit:

old:
```yaml
  StealHiddenBonus: 25   # Score added to steal AND plant attempts made while hidden
```
new:
```yaml
  StealHiddenBonus: 25   # Score added to steal AND plant attempts made while hidden
  # StealMinRank: the skullduggery rank every theft needs (a creature, a
  #   player, a room container, a household's find), for players and mobs
  #   alike. Every character starts at rank 1, so 1 lets anyone try; a mob
  #   thief's rank comes from its own file. Below 1 reads as 1. A hardcoded 2
  #   until #375 phase 7. templates/help/steal.template names this value and
  #   internal/configs/steal_min_rank_test.go fails if the two disagree.
  StealMinRank: 1
```

- [ ] **Step 6: Run and pass**

```bash
gofmt -l internal/configs/ && go test ./internal/configs/ 2>&1 | tail -5
```

Expected: `gofmt` prints nothing; `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go internal/configs/steal_min_rank_test.go _datafiles/config.yaml
git commit -F - <<'EOF'
feat(config): StealMinRank knob, shipped at 1 (#375 phase 7)

Declares Balance.StealMinRank, validated so an absent or sub-1 value
reads 1, and ships it at 1 in config.yaml. Not yet read by steal.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 2: Steal reads `StealMinRank`; help states the real ranks

**Executor:** Sonnet

**Files:**
- Create: `internal/actions/steal_min_rank_test.go`
- Modify: `internal/actions/steal.go`, `internal/actions/steal_cooldown_test.go`, `internal/actions/household_bauble_test.go`, `internal/usercommands/skill.skullduggery.steal.go`, `internal/configs/steal_min_rank_test.go`, `_datafiles/world/dogmud/templates/help/steal.template`, `_datafiles/world/dogmud/templates/help/plant.template`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/steal_min_rank_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// pinStealMinRank sets Balance.StealMinRank for one test. Call it after any
// other config pin (pinStealCooldown, setupHousehold): it copies the config
// as it stands.
func pinStealMinRank(t *testing.T, rank int) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.StealMinRank = configs.ConfigInt(rank)
	configs.SetConfigForTest(t, c)
}

// The theft rank gate is Balance.StealMinRank (#375 phase 7), the same for a
// player and a mob thief. At the shipped 1 a fresh character can try; at 2
// the same character is refused.
func TestSteal_MinRankComesFromConfig(t *testing.T) {
	t.Run("player at the default", func(t *testing.T) {
		actor := newStealPlayerActor(100, 1)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.NotEqual(t, "not advanced enough", res.Reason, "rank 1 passes the shipped gate: %+v", res)
	})
	t.Run("player below a raised gate", func(t *testing.T) {
		pinStealMinRank(t, 2)
		actor := newStealPlayerActor(100, 1)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.Equal(t, "not advanced enough", res.Reason)
	})
	t.Run("mob thief at the default", func(t *testing.T) {
		mark := users.NewTestUser(3021, "mark", "Mark", 0)
		mark.Character.Gold = 50
		defer users.SeedUsersForTest(map[int]*users.UserRecord{3021: mark})()
		actor := newStealMobActor(100, 1)
		res := Steal(actor, StealOptions{TargetUserId: 3021})
		require.NotEqual(t, "not advanced enough", res.Reason, "a rank-1 mob passes the shipped gate: %+v", res)
	})
	t.Run("mob thief below a raised gate", func(t *testing.T) {
		pinStealMinRank(t, 3)
		mark := users.NewTestUser(3022, "mark", "Mark", 0)
		mark.Character.Gold = 50
		defer users.SeedUsersForTest(map[int]*users.UserRecord{3022: mark})()
		actor := newStealMobActor(100, 2)
		res := Steal(actor, StealOptions{TargetUserId: 3022})
		require.Equal(t, "not advanced enough", res.Reason)
	})
}
```

Append to `internal/configs/steal_min_rank_test.go` (add `"fmt"` and `"strings"` to its imports):

```go
// The steal help names the rank the game ships, so a retune that forgets
// the help fails here.
func TestStealHelpNamesTheShippedRank(t *testing.T) {
	rank := shippedBalance(t).StealMinRank
	help, err := os.ReadFile(repoFile(t, "_datafiles", "world", "dogmud", "templates", "help", "steal.template"))
	if err != nil {
		t.Fatalf("read steal help: %v", err)
	}
	want := fmt.Sprintf("</ansi> rank %d.", rank)
	if !strings.Contains(string(help), want) {
		t.Fatalf("steal.template must say %q (StealMinRank ships at %d)", want, rank)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/actions/ -run TestSteal_MinRankComesFromConfig 2>&1 | tail -8
go test ./internal/configs/ -run TestStealHelpNamesTheShippedRank 2>&1 | tail -4
```

Expected: the two "at the default" subtests fail with Reason `not advanced enough`; the help test fails asking for `"</ansi> rank 1."`.

- [ ] **Step 3: Carry the knob on `theftAttempt`**

In `internal/actions/steal.go`, Edit:

old:
```go
// only then asks for the score to roll with. score() is the only way to
// read it, and it arms the cooldown as it hands it over, so no refusal can
// spend the cooldown and no contest can run without spending it.
type theftAttempt struct {
	char   *characters.Character
	rank   int
	points float64
	period string
}
```
new:
```go
// only then asks for the score to roll with. score() is the only way to
// read it, and it arms the cooldown as it hands it over, so no refusal can
// spend the cooldown and no contest can run without spending it.
type theftAttempt struct {
	char    *characters.Character
	rank    int
	minRank int
	points  float64
	period  string
}

// belowMinRank is every theft's rank gate: the thief's skullduggery rank
// under Balance.StealMinRank (#375 phase 7; a hardcoded 2 until then).
// Players and mobs share it.
func (t theftAttempt) belowMinRank() bool {
	return t.rank < t.minRank
}
```

Edit:

old:
```go
	return theftAttempt{
		char:   char,
		rank:   rank,
		points: points,
		period: fmt.Sprintf(`%d real seconds`, int(cfg.StealCooldown)),
	}
```
new:
```go
	return theftAttempt{
		char:    char,
		rank:    rank,
		minRank: int(cfg.StealMinRank),
		points:  points,
		period:  fmt.Sprintf(`%d real seconds`, int(cfg.StealCooldown)),
	}
```

Edit (comment at 171-172):

old:
```go
// only CHECK the cooldown; each path then runs its own refusals (target
// gone, a companion, an immune mob, rank below 2, an empty container) and
```
new:
```go
// only CHECK the cooldown; each path then runs its own refusals (target
// gone, a companion, an immune mob, rank below StealMinRank, an empty container) and
```

If that old_string does not match, run `grep -n "rank below 2" internal/actions/steal.go` and edit that line the same way.

- [ ] **Step 4: Replace the four gates**

Edit (creature, line 254-258):

old:
```go
	// Skill rank 2 required for the actual steal mechanic. Checked
	// AFTER the immune gate so a low-rank thief targeting an immune mob
	// gets the immune rebuff (which doesn't imply skill would help)
	// rather than the skill rebuff (which does).
	if t.rank < 2 {
```
new:
```go
	// The StealMinRank gate on the actual steal mechanic. Checked
	// AFTER the immune gate so a low-rank thief targeting an immune mob
	// gets the immune rebuff (which doesn't imply skill would help)
	// rather than the skill rebuff (which does).
	if t.belowMinRank() {
```

Edit (player, line 426-427):

old:
```go
	// Skill rank 2 required.
	if t.rank < 2 {
```
new:
```go
	// The StealMinRank gate.
	if t.belowMinRank() {
```

Edit (container, line 530-533):

old:
```go
	// Skill rank 2 required for the actual steal mechanic. Mirrors the
	// gate in stealFromMob; checked AFTER target validation so the rebuff
	// order is consistent.
	if t.rank < 2 {
```
new:
```go
	// The StealMinRank gate on the actual steal mechanic. Mirrors the
	// gate in stealFromMob; checked AFTER target validation so the rebuff
	// order is consistent.
	if t.belowMinRank() {
```

Edit (household, line 826 and 843):

old:
```go
// when the contest runs (theftAttempt.score); skullduggery rank 2 is required, as for
```
new:
```go
// when the contest runs (theftAttempt.score); the StealMinRank gate holds, as for
```

old:
```go
		return StealResult{Reason: "not found"}
	}
	if t.rank < 2 {
```
new:
```go
		return StealResult{Reason: "not found"}
	}
	if t.belowMinRank() {
```

Verify: `grep -n "rank < 2\|rank 2" internal/actions/steal.go` prints nothing.

- [ ] **Step 5: Re-pin the refusal tests at min rank 2**

In `internal/actions/steal_cooldown_test.go`, Edit the header comment:

old:
```go
// The skullduggery cooldown is spent by an attempt, never by a refusal. It
// used to be armed in Steal and Plant before any target was resolved, so a
// fresh character (Skullduggery rank 1 passes the command's visibility gate
// but fails every theft's rank-2 gate) was refused on the first try and then
// locked out of trying again for the whole cooldown.
```
new:
```go
// The skullduggery cooldown is spent by an attempt, never by a refusal. It
// used to be armed in Steal and Plant before any target was resolved, so a
// character below the theft rank gate (Balance.StealMinRank, a hardcoded 2
// then) was refused on the first try and then locked out of trying again
// for the whole cooldown. The refusal tests pin the gate at 2 so a rank-1
// thief is below it.
```

Edit:

old:
```go
func TestStealCooldown_RankOneRefusalLeavesItUnarmed(t *testing.T) {
	pinStealCooldown(t)
```
new:
```go
func TestStealCooldown_BelowMinRankRefusalLeavesItUnarmed(t *testing.T) {
	pinStealCooldown(t)
	pinStealMinRank(t, 2)
```

Edit (the household subtest re-pins the cooldown, which copies the config, so it must re-pin the rank after it):

old:
```go
		h := setupHousehold(t, 99301, 1, nil, false)
		pinStealCooldown(t)
		res := Steal(h.thief, StealOptions{HouseholdItem: h.itm})
```
new:
```go
		h := setupHousehold(t, 99301, 1, nil, false)
		pinStealCooldown(t)
		pinStealMinRank(t, 2)
		res := Steal(h.thief, StealOptions{HouseholdItem: h.itm})
```

In `internal/actions/household_bauble_test.go`, Edit:

old:
```go
// The steal checks: skullduggery rank 2, and the steal cooldown.
func TestStealHousehold_UsesTheStealChecks(t *testing.T) {
	h := setupHousehold(t, 9704, 1, nil, false)
	if res := h.steal(); res.Succeeded || res.Reason != "not advanced enough" || h.inPack() {
		t.Fatalf("rank 1 cannot steal: %+v", res)
	}
```
new:
```go
// The steal checks: the StealMinRank gate, and the steal cooldown.
func TestStealHousehold_UsesTheStealChecks(t *testing.T) {
	h := setupHousehold(t, 9704, 1, nil, false)
	pinStealMinRank(t, 2)
	if res := h.steal(); res.Succeeded || res.Reason != "not advanced enough" || h.inPack() {
		t.Fatalf("rank 1 cannot steal under a rank-2 gate: %+v", res)
	}
```

- [ ] **Step 6: Comments in the player command**

In `internal/usercommands/skill.skullduggery.steal.go`, Edit:

old:
```go
/*
Skullduggery Skill
Level 2 - Steal: attempt to steal from a mob or room container using
an opposed Dex+skill vs Perception roll.
*/
```
new:
```go
/*
Skullduggery Skill
Steal: attempt to steal from a mob or room container using an opposed
Dex+skill vs Perception roll. The theft needs Balance.StealMinRank.
*/
```

Edit:

old:
```go
	// Requires skullduggery rank 1 to even see the command. Rank 2
	// is required for the actual steal — that gate lives inside
	// actions.Steal AFTER target validation, so immune-mob targets get
```
new:
```go
	// Requires skullduggery rank 1 to even see the command.
	// Balance.StealMinRank is required for the actual steal: that gate lives inside
	// actions.Steal AFTER target validation, so immune-mob targets get
```

- [ ] **Step 7: Help states the real ranks**

In `_datafiles/world/dogmud/templates/help/steal.template`, Edit:

old:
```
a container without being noticed. You cannot choose what
you steal -- you grab whatever you can get your hands on.
```
new:
```
a container without being noticed. You cannot choose what
you steal: you grab whatever you can get your hands on.
```

Edit:

old:
```
Requires <ansi fg="skill">skullduggery</ansi> rank 2.
```
new:
```
Requires <ansi fg="skill">skullduggery</ansi> rank 1. Every character starts at
rank 1, so anyone can try.
```

In `_datafiles/world/dogmud/templates/help/plant.template`, Edit:

old:
```
or a container without being noticed. The last word is
treated as the target -- everything before it is the item.
```
new:
```
or a container without being noticed. The last word is
the target, and everything before it is the item.
```

Edit:

old:
```
Requires <ansi fg="skill">skullduggery</ansi> rank 2.
```
new:
```
Requires <ansi fg="skill">skullduggery</ansi> rank 1. Every character starts at
rank 1, so anyone can try.
```

- [ ] **Step 8: Run and pass, and prove the gate test can fail**

```bash
gofmt -l internal/ && go test ./internal/actions/ ./internal/configs/ ./internal/usercommands/ ./internal/behaviortree/ ./internal/mobcommands/ 2>&1 | tail -10
```

Expected: all `ok`. Null probe: temporarily change `return t.rank < t.minRank` to `return t.rank < 2`, rerun `go test ./internal/actions/ -run TestSteal_MinRankComesFromConfig`, see the two "at the default" subtests fail, then restore the line and see green.

- [ ] **Step 9: Commit**

```bash
git add internal/actions/steal.go internal/actions/steal_min_rank_test.go internal/actions/steal_cooldown_test.go internal/actions/household_bauble_test.go internal/usercommands/skill.skullduggery.steal.go internal/configs/steal_min_rank_test.go _datafiles/world/dogmud/templates/help/steal.template _datafiles/world/dogmud/templates/help/plant.template
git commit -F - <<'EOF'
feat(steal): theft rank gate reads StealMinRank (#375 phase 7)

The four hardcoded rank-2 gates (creature, player, container, household
find) now read Balance.StealMinRank through theftAttempt, shared by player
and mob thieves. steal and plant help say rank 1; a test ties the steal
help to the shipped value.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 3: Refusal helpers for the wrong drinking verb

**Executor:** Sonnet

**Files:**
- Modify: `internal/refusal/carrying.go`
- Create: `internal/refusal/potion_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/refusal/potion_test.go`:

```go
package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

// drink or eat aimed at a potion consumes nothing and names quaff; quaff
// aimed at food or a cooked drink names the verb that takes it (#375 phase 7).
func TestQuaffPotionAndNotAPotion(t *testing.T) {
	f := &fakeRecipient{}
	QuaffPotion(f, "healing salve")
	assert.Equal(t, `That is a potion, and potions are drunk with quaff. Try <ansi fg="command">quaff healing salve</ansi>.`,
		one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	NotAPotion(f, "tea", "drink")
	assert.Equal(t, `That is not a potion. Try <ansi fg="command">drink tea</ansi>.`,
		one(t, f, messaging.CategoryRefusal))

	// Typed text is echoed as every refusal echoes it: tags cannot style it.
	assert.NotContains(t, QuaffPotionText(`</ansi><ansi fg="red">x`), `<ansi fg="red">`)
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/refusal/ -run TestQuaffPotionAndNotAPotion 2>&1 | tail -4
```

Expected: `undefined: QuaffPotion`.

- [ ] **Step 3: Implement**

Append to `internal/refusal/carrying.go`:

```go

// QuaffPotionText answers drink or eat aimed at a potion: nothing is
// consumed, and the line names quaff (#375 phase 7, owner ruling
// 2026-10-30). It echoes typed, the player's own item.
func QuaffPotionText(typed string) string {
	return `That is a potion, and potions are drunk with quaff.` + hint(`quaff `+echo(typed))
}

func QuaffPotion(to Recipient, typed string) { send(to, QuaffPotionText(typed)) }

// NotAPotionText answers quaff aimed at food or a cooked drink, naming the
// verb that takes it ("eat" or "drink").
func NotAPotionText(typed, verb string) string {
	return `That is not a potion.` + hint(verb+` `+echo(typed))
}

func NotAPotion(to Recipient, typed, verb string) { send(to, NotAPotionText(typed, verb)) }
```

- [ ] **Step 4: Run and pass**

```bash
go test ./internal/refusal/ ./internal/guards/ -run 'TestQuaffPotion|TestRefusal' 2>&1 | tail -4
```

Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add internal/refusal/carrying.go internal/refusal/potion_test.go
git commit -F - <<'EOF'
feat(refusal): QuaffPotion and NotAPotion lines (#375 phase 7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 4: `actions.Quaff`, and the drink line before the quest line (#697)

**Executor:** Sonnet

**Files:**
- Create: `internal/actions/quaff_quest_order_test.go`
- Modify: `internal/actions/drink.go`, `internal/actions/drink_magnitude_test.go`, `drink_parity_test.go`, `drink_room_line_sight_test.go`, `drink_scour_test.go`, `internal/guards/condition_apply_path_guard_test.go:236`

- [ ] **Step 1: Write the failing test**

Create `internal/actions/quaff_quest_order_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #697: the quest engine hears a quaff only after the drinker has read "You
// drink the ...", so a step's line ("The salve spreads warm through you")
// never arrives before the drink it reports. The quest engine hears the
// command as quaff (#375 phase 7).
func TestQuaff_DrinkLineComesBeforeTheQuestLine(t *testing.T) {
	const userId, tonicId = 7311, 39311
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tonicId: {ItemId: tonicId, Name: "test tonic", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
	}))
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 98,
		Name:    "Quaff Order Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "quaff",
			Actions: []quests.ActionDef{{Grant: "98-start"}, {SendText: "QUEST LINE"}},
		}},
	})
	c := characters.New()
	c.Name = "Quaffer"
	c.RoomId = 99999
	u := &users.UserRecord{UserId: userId, Character: c}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	require.True(t, c.StoreItem(items.New(tonicId)))
	events.DrainQueuedMessagesForTest(userId)

	res := Quaff(NewUserActorInRoom(u, newEmptyTestRoom(t)).(DrinkActor), "test tonic")
	require.True(t, res.Drank, "the tonic must be drunk: %+v", res)

	msgs := events.DrainQueuedMessagesForTest(userId)
	drank, quest := -1, -1
	for i, m := range msgs {
		if drank < 0 && strings.Contains(m, "You drink the") {
			drank = i
		}
		if quest < 0 && strings.Contains(m, "QUEST LINE") {
			quest = i
		}
	}
	require.GreaterOrEqual(t, drank, 0, "no drink line: %q", msgs)
	require.GreaterOrEqual(t, quest, 0, "the quaff did not reach the quest engine as quaff: %q", msgs)
	require.Less(t, drank, quest, "the quest line came before the drink: %q", msgs)
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/actions/ -run TestQuaff_DrinkLineComesBeforeTheQuestLine 2>&1 | tail -4
```

Expected: `undefined: Quaff`.

- [ ] **Step 3: Rename and re-verb the body**

In `internal/actions/drink.go`, Edit:

old:
```go
// Drink is the one drink body for players and mobs, the AI companion included
// (drink path unification). It was usercommands.Drink; the mob command was a
// separate 49-line copy with no toxicity, no aging or crafter scaling and no
// special potions. Every special potion applies fully to a mob (owner ruling,
// 2026-09-28). SendText is a no-op for a mob, so a mob drinks silently apart
// from the room line.
func Drink(actor DrinkActor, rest string) DrinkResult {
```
new:
```go
// Quaff is the one potion body for players and mobs, the AI companion
// included (drink path unification). It was usercommands.Drink; the mob
// command was a separate 49-line copy with no toxicity, no aging or crafter
// scaling and no special potions. Every special potion applies fully to a mob
// (owner ruling, 2026-09-28). SendText is a no-op for a mob, so a mob drinks
// silently apart from the room line. The command is quaff since #375 phase 7
// (owner ruling 2026-10-30); drink and eat are the food command
// (usercommands/eat.go). The narration still reads "You drink the ...".
func Quaff(actor DrinkActor, rest string) DrinkResult {
```

Edit:

old:
```go
		refusal.Prompt(actor, `drink`, refusal.What, `drink <item>`)
```
new:
```go
		refusal.Prompt(actor, `quaff`, refusal.What, `quaff <item>`)
```

Edit:

old:
```go
		refusal.WhileBusy(actor, `drink`)
```
new:
```go
		refusal.WhileBusy(actor, `quaff`)
```

Edit:

old:
```go
	if itemSpec.Subtype != items.Drinkable {
		actor.SendText(messaging.CategoryRefusal,
			fmt.Sprintf(`You can't drink <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()),
		)
		return DrinkResult{Refusal: DrinkRefuseNotDrinkable, ItemId: matchItem.ItemId}
	}
```
new:
```go
	if itemSpec.Subtype != items.Drinkable {
		if itemSpec.Subtype == items.Edible {
			// Food or a cooked drink: name the verb that takes it.
			verb := `eat`
			if itemSpec.Type == items.Drink {
				verb = `drink`
			}
			refusal.NotAPotion(actor, rest, verb)
		} else {
			actor.SendText(messaging.CategoryRefusal,
				fmt.Sprintf(`You can't quaff <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()),
			)
		}
		return DrinkResult{Refusal: DrinkRefuseNotDrinkable, ItemId: matchItem.ItemId}
	}
```

- [ ] **Step 4: Narrate first, then notify (#697)**

Edit:

old:
```go
	// Quest engine: command notification. A successful drink advances
	// "drink a potion" quest steps (e.g. the Spoke C alchemy cert). Only a
	// player has quests.
	if ua, ok := actor.(*UserActor); ok && room != nil {
		questengine.NotifyCommand(ua.User, room.RoomId, "drink", questengine.TargetItem(matchItem.ItemId))
	}

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You drink the <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()))
	if room != nil {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi> drinks <ansi fg="itemname">%s</ansi>.`,
			nameColor, char.Name, matchItem.DisplayName()), []string{char.Name}, roomExclude...)
	}
```
new:
```go
	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You drink the <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()))
	if room != nil {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi> drinks <ansi fg="itemname">%s</ansi>.`,
			nameColor, char.Name, matchItem.DisplayName()), []string{char.Name}, roomExclude...)
	}

	// Quest engine: command notification, AFTER the drink line so a step's
	// own line reads after the drink it reports (#697). A successful quaff
	// advances "drink a potion" quest steps (the Spoke C alchemy cert). Only
	// a player has quests.
	if ua, ok := actor.(*UserActor); ok && room != nil {
		questengine.NotifyCommand(ua.User, room.RoomId, "quaff", questengine.TargetItem(matchItem.ItemId))
	}
```

- [ ] **Step 5: Let the compiler list the test callers**

```bash
go vet ./internal/actions/ 2>&1 | grep "undefined: Drink" | head
```

In each listed file (expected: `drink_magnitude_test.go`, `drink_parity_test.go`, `drink_room_line_sight_test.go`, `drink_scour_test.go`), use Edit to change every call `Drink(` to `Quaff(` and every comment naming `actions.Drink` to `actions.Quaff`. Do not rename the types `DrinkActor`, `DrinkResult` or `DrinkRefusal`, and do not change the condition source string `drink`. Re-run until `go vet ./internal/actions/` is clean.

- [ ] **Step 6: Re-key the line allowlist entry**

```bash
grep -n "actor.AddConditionMagnitude(conditionId, trig, mag" internal/actions/drink.go
```

Take that line number N (it moved from 359 because Step 3 added lines). In `internal/guards/condition_apply_path_guard_test.go`, Edit the key `"internal/actions/drink.go|359":` to `"internal/actions/drink.go|N":` with your N. Leave the reason text as it is.

- [ ] **Step 7: Run and pass, and prove the order test can fail**

```bash
gofmt -l internal/ && go test ./internal/actions/ 2>&1 | tail -5
```

Expected: `ok`. `./internal/usercommands`, `./internal/mobcommands` and `./internal/guards` (which imports both) do not build until Task 5; the guard re-key is checked in Task 5 Step 11. Null probe: move the `questengine.NotifyCommand` block back above the `You drink the` line, rerun `go test ./internal/actions/ -run TestQuaff_DrinkLineComesBeforeTheQuestLine`, see `the quest line came before the drink`, then restore and see green.

- [ ] **Step 8: Do not commit yet**

The tree does not build until the callers move to `actions.Quaff`. Task 5 Step 12 commits this task's files together with its own, so every commit on the branch builds.

---

## Task 5: The `quaff` command; `eat` and `drink` as one food command

**Executor:** Opus

**Files:**
- Create: `internal/usercommands/quaff.go`, `internal/mobcommands/quaff.go`, `internal/usercommands/eat_drink_quaff_test.go`
- Modify: `internal/usercommands/drink.go`, `eat.go`, `usercommands.go`, `busy_refuse_test.go`, `bare_item_verb_prompt_test.go`, `usercommands_test.go`; `internal/mobcommands/drink.go`, `eat.go`, `mobcommands.go`, `mobcommands_test.go`; `internal/guards/drink_wrapper_guard_test.go`, `messaging_surface_guard_test.go`
- Rename: `internal/mobcommands/drink_wrapper_test.go` to `internal/mobcommands/quaff_wrapper_test.go`

- [ ] **Step 1: Write the failing player test**

Create `internal/usercommands/eat_drink_quaff_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	edqBreadId = 39321 // food
	edqTeaId   = 39322 // a cooked drink
	edqTonicId = 39323 // a potion
)

// edqScene is user 1 in a lit room, with the three test items registered and
// an empty pack and bandolier.
func edqScene(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := aimScene(t, aimFull)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		edqBreadId: {ItemId: edqBreadId, Name: "test bread", Type: items.Food, Subtype: items.Edible, Uses: 1},
		edqTeaId:   {ItemId: edqTeaId, Name: "test tea", Type: items.Drink, Subtype: items.Edible, Uses: 1},
		edqTonicId: {ItemId: edqTonicId, Name: "test tonic", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
	}))
	user.Character.Items = nil
	user.Character.PotionItems = nil
	return user, room
}

func edqCarries(user *users.UserRecord, itemId int) bool {
	for _, it := range append(append([]items.Item{}, user.Character.Items...), user.Character.PotionItems...) {
		if it.ItemId == itemId {
			return true
		}
	}
	return false
}

// eat and drink are one command (#375 phase 7, owner ruling 2026-10-30):
// both take food and cooked drinks, and the line says eat or drink by the
// item, not by the word typed.
func TestEatAndDrinkTakeFoodAndCookedDrinks(t *testing.T) {
	user, room := edqScene(t)
	for _, c := range []struct {
		verb   string
		itemId int
		typed  string
		line   string
	}{
		{"eat", edqBreadId, "test bread", "You eat some of the"},
		{"drink", edqBreadId, "test bread", "You eat some of the"},
		{"drink", edqTeaId, "test tea", "You drink some of the"},
		{"eat", edqTeaId, "test tea", "You drink some of the"},
	} {
		require.True(t, user.Character.StoreItem(items.New(c.itemId)), c.verb)
		events.DrainQueuedMessagesForTest(1)
		handled, err := userCommands[c.verb].Func(c.typed, user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		told := aimTold(1)
		assert.Contains(t, told, c.line, "%s %s", c.verb, c.typed)
		assert.False(t, edqCarries(user, c.itemId), "%s %s must consume it", c.verb, c.typed)
	}
}

// drink or eat on a potion consumes nothing and names quaff.
func TestEatAndDrinkLeaveAPotion(t *testing.T) {
	user, room := edqScene(t)
	require.True(t, user.Character.StoreItem(items.New(edqTonicId)))
	for _, verb := range []string{"drink", "eat"} {
		events.DrainQueuedMessagesForTest(1)
		_, err := userCommands[verb].Func("test tonic", user, room, 0)
		require.NoError(t, err)
		told := aimTold(1)
		assert.Contains(t, told, "potions are drunk with quaff", verb)
		assert.Contains(t, told, "quaff test tonic", verb)
		assert.True(t, edqCarries(user, edqTonicId), "%s must not consume a potion", verb)
	}
}

// quaff drinks a potion and refuses a cooked drink by naming drink.
func TestQuaffTakesPotionsOnly(t *testing.T) {
	user, room := edqScene(t)
	require.True(t, user.Character.StoreItem(items.New(edqTonicId)))
	require.True(t, user.Character.StoreItem(items.New(edqTeaId)))

	events.DrainQueuedMessagesForTest(1)
	_, err := Quaff("test tea", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.Contains(t, told, "That is not a potion.")
	assert.Contains(t, told, "drink test tea")
	assert.True(t, edqCarries(user, edqTeaId))

	events.DrainQueuedMessagesForTest(1)
	_, err = Quaff("test tonic", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, aimTold(1), "You drink the")
	assert.False(t, edqCarries(user, edqTonicId))
}

// Herbal Tea was a potion until #375 phase 7, so a player may hold one in a
// bandolier. drink still finds it there.
func TestDrinkFindsACookedDrinkInTheBandolier(t *testing.T) {
	user, room := edqScene(t)
	user.Character.PotionItems = []items.Item{items.New(edqTeaId)}
	events.DrainQueuedMessagesForTest(1)
	_, err := Drink("test tea", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, aimTold(1), "You drink some of the")
	assert.Empty(t, user.Character.PotionItems)
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/usercommands/ -run 'TestEatAndDrink|TestQuaffTakes|TestDrinkFinds' 2>&1 | tail -6
```

Expected: build failure (`undefined: Quaff`, and the `actions.Drink` callers from Task 4).

- [ ] **Step 3: The player `quaff` command**

Create `internal/usercommands/quaff.go`:

```go
package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Quaff is the player command for potions (#375 phase 7, owner ruling
// 2026-10-30); the rules live in actions.Quaff (drink path unification),
// shared with mobs and the AI companion. Food and cooked drinks go through
// eat and drink (eat.go).
func Quaff(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actions.Quaff(actions.NewUserActorInRoom(user, room).(actions.DrinkActor), rest)
	return true, nil
}
```

- [ ] **Step 4: `drink` becomes the food command**

Replace the whole of `internal/usercommands/drink.go` with Write:

```go
package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Drink is eat under its other name (#375 phase 7, owner ruling 2026-10-30):
// both take food and cooked drinks (consumeFood in eat.go). Potions are
// drunk with quaff (quaff.go, actions.Quaff).
func Drink(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return consumeFood(`drink`, rest, user, room)
}
```

- [ ] **Step 5: One food body**

Replace the whole of `internal/usercommands/eat.go` with Write:

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Eat is the food command. eat and drink are one command (#375 phase 7,
// owner ruling 2026-10-30): both take food (type food) and cooked drinks
// (type drink), every one of which is subtype edible. Potions are drunk with
// quaff (actions.Quaff); eat or drink aimed at one consumes nothing and
// names quaff.
func Eat(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return consumeFood(`eat`, rest, user, room)
}

// consumeFood is the body of eat and drink. verb is the word the player
// typed; it words the prompt and the refusals. The success line says eat or
// drink by the item's type, whichever word was typed.
func consumeFood(verb string, rest string, user *users.UserRecord, room *rooms.Room) (bool, error) {
	if strings.TrimSpace(rest) == `` {
		refusal.Prompt(user, verb, refusal.What, verb+` <item>`)
		return true, nil
	}

	if refuseWhileBusy(user, verb) {
		return true, nil
	}

	// Chunk 4e: can't eat or drink while grappled, both hands committed.
	if user.Character.Position != nil && user.Character.Position.IsGrappling() {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="red">Your hands are committed to the grapple, so you can't reach for that.</ansi>`)
		return true, nil
	}

	// Edible-first: skip same-noun inedibles. Next the bandolier, where a
	// Herbal Tea bought while it was a potion may still sit (it was retyped a
	// drink in #375 phase 7). Then the unfiltered match, so the refusals
	// below still fire when nothing edible matches.
	fromBandolier := false
	matchItem, found := user.Character.FindInBackpackWhere(rest, func(it items.Item) bool {
		return it.GetSpec().Subtype == items.Edible
	})
	if !found {
		if it, ok := user.Character.FindInPotions(rest); ok && it.GetSpec().Subtype == items.Edible {
			matchItem, found, fromBandolier = it, true, true
		}
	}
	if !found {
		matchItem, found = user.Character.FindInBackpack(rest)
	}
	if !found {
		// A potion carried only in the bandolier gets the quaff line too.
		if it, ok := user.Character.FindInPotions(rest); ok && it.GetSpec().Subtype == items.Drinkable {
			refusal.QuaffPotion(user, rest)
			return true, nil
		}
		refusal.NotCarrying(user, rest)
		return true, nil
	}

	itemSpec := matchItem.GetSpec()

	if itemSpec.Subtype != items.Edible {
		// A potion: nothing is consumed, and the line names quaff.
		if itemSpec.Subtype == items.Drinkable {
			refusal.QuaffPotion(user, rest)
			return true, nil
		}
		// A cooking ingredient (raw hare meat) says what to do with it
		// (2026-10-17 playtest: the refusal gave no pointer to cooking).
		if crafting.IsIngredientForSkill(matchItem, string(skills.Cooking)) {
			user.SendText(messaging.CategoryRefusal,
				fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi> as it is. Cook it first. Try <ansi fg="command">help cooking</ansi>.`, verb, matchItem.DisplayName()),
			)
			return true, nil
		}
		user.SendText(messaging.CategoryRefusal,
			fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi>.`, verb, matchItem.DisplayName()),
		)
		return true, nil
	}

	// Food that has gone bad is refused; mob eat asks the same (#277).
	if matchItem.IsSpoiledFood(util.GetRoundCount()) {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi>`)
		return true, nil
	}

	user.Character.CancelConditionsWithFlag(conditions.Hidden)

	you, they := `eat`, `eats`
	if itemSpec.Type == items.Drink {
		you, they = `drink`, `drinks`
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`You %s some of the <ansi fg="itemname">%s</ansi>.`, you, matchItem.DisplayName()))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> %s some <ansi fg="itemname">%s</ansi>.`, user.Character.Name, they, matchItem.DisplayName()), user.UserId)

	// If no more uses, will be lost, so trigger event
	usesLeft := 0
	if fromBandolier {
		usesLeft = user.Character.UseItemFromPotions(matchItem)
	} else {
		usesLeft = user.Character.UseItem(matchItem)
	}
	if usesLeft < 1 {
		events.AddToQueue(events.ItemOwnership{
			UserId: user.UserId,
			Item:   matchItem,
			Gained: false,
		})
	}

	for _, conditionId := range itemSpec.ConditionIds {
		user.AddCondition(conditionId, `food`)
	}

	return true, nil
}
```

- [ ] **Step 6: Register `quaff`**

In `internal/usercommands/usercommands.go`, Edit:

old:
```go
		`quests`:          {Quests, true, true, false},
```
new:
```go
		`quaff`:           {Quaff, true, true, false},
		`quests`:          {Quests, true, true, false},
```

- [ ] **Step 7: The mob side**

Create `internal/mobcommands/quaff.go`:

```go
package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Quaff is the mob command for potions; the rules live in actions.Quaff,
// the same body a player quaffs through (#375 phase 7).
func Quaff(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actions.Quaff(actions.NewMobActorInRoom(mob, room).(actions.DrinkActor), rest)
	return true, nil
}
```

Replace the whole of `internal/mobcommands/drink.go` with Write:

```go
package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Drink is eat under its other name for a mob, as for a player
// (usercommands.Drink): it takes food and cooked drinks. A mob drinks a
// potion with quaff (quaff.go).
func Drink(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	return Eat(rest, mob, room)
}
```

In `internal/mobcommands/eat.go`, Edit:

old:
```go
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> eats some <ansi fg="itemname">%s</ansi>.`, mob.Character.Name, matchItem.DisplayName()))
```
new:
```go
		verb := `eats`
		if itemSpec.Type == items.Drink {
			verb = `drinks`
		}
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> %s some <ansi fg="itemname">%s</ansi>.`, mob.Character.Name, verb, matchItem.DisplayName()))
```

In `internal/mobcommands/mobcommands.go`, Edit:

old:
```go
		"put":            {Put, false},
```
new:
```go
		"put":            {Put, false},
		"quaff":          {Quaff, false},
```

- [ ] **Step 8: Tests that named the old split**

`git mv internal/mobcommands/drink_wrapper_test.go internal/mobcommands/quaff_wrapper_test.go`, then in it Edit `TestMobDrinkRunsTheSharedBody` to `TestMobQuaffRunsTheSharedBody`, both `Drink(` calls to `Quaff(`, and the comment's `actions.Drink` to `actions.Quaff`.

In `internal/mobcommands/mobcommands_test.go`, Edit:

old:
```go
		{"drink", ""},
		{"eat", ""},
```
new:
```go
		{"drink", ""},
		{"eat", ""},
		{"quaff", ""},
```

In `internal/usercommands/busy_refuse_test.go`, Edit:

old:
```go
		{"drink", "drink", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Drink("water", u, r, 0) }},
```
new:
```go
		{"drink", "drink", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Drink("water", u, r, 0) }},
		{"quaff", "quaff", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Quaff("salve", u, r, 0) }},
```

In `internal/usercommands/bare_item_verb_prompt_test.go`, Edit:

old:
```go
	for _, verb := range []string{"drink", "eat", "read", "use", "remove", "stash"} {
```
new:
```go
	for _, verb := range []string{"drink", "eat", "quaff", "read", "use", "remove", "stash"} {
```

In `internal/usercommands/usercommands_test.go`, Edit:

old:
```go
	t.Run("drink_healing_potion", func(t *testing.T) {
		user.Character.StoreItem(items.Item{ItemId: 30001})
		handled, err := Drink("healing potion", user, room, 0)
```
new:
```go
	t.Run("quaff_healing_potion", func(t *testing.T) {
		user.Character.StoreItem(items.Item{ItemId: 30001})
		handled, err := Quaff("healing potion", user, room, 0)
```

- [ ] **Step 9: Guard the new wrappers**

Replace the whole of `internal/guards/drink_wrapper_guard_test.go` with Write:

```go
package guards

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestQuaffWrappersDoNotReFork: the potion rules live in actions.Quaff. If a
// command wrapper applies a condition, toxicity, an item use, or a mutation
// change itself, the player and mob paths have forked again (the mob path
// was a 49-line copy missing toxicity, potency and every special potion
// until 2026-09-28). The wrappers are quaff.go since #375 phase 7; drink.go
// is the food command now (usercommands/eat.go).
func TestQuaffWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`AddCondition|AddToxicity|UseItem|ScourMutations|AddBloomAddiction|GrantRandomMutation|BloomSeedNewMutation|BloomAdvanceMutation|SetTickAmount`)
	for _, path := range []string{"internal/usercommands/quaff.go", "internal/mobcommands/quaff.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(b), "actions.Quaff(") {
			t.Errorf("%s does not call actions.Quaff", path)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s applies potion rules itself (%q); call actions.Quaff instead", path, b[loc[0]:loc[1]])
		}
	}
}
```

- [ ] **Step 10: Re-key the narration registry entry**

In `internal/guards/messaging_surface_guard_test.go`, Edit:

old:
```go
	"usercommands/eat.go|You eat some of the <ansi fg=\"itemname\">%s</ansi>.":
```
new:
```go
	"usercommands/eat.go|You %s some of the <ansi fg=\"itemname\">%s</ansi>.":
```

and in the same entry's reason string, after `usercommands/eat.go:67)` append `; the verb follows the item's type since #375 phase 7 (eat or drink)` before the closing quote.

- [ ] **Step 11: Build, run, pass**

```bash
gofmt -l internal/ modules/ && go build ./... && go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ ./internal/refusal/ 2>&1 | tail -8
go test ./internal/guards/ 2>&1 | tail -8
```

Expected: all `ok`. If `TestNarrationSitesMatchViewpointAudit` names a different key for `usercommands/eat.go`, use the key it prints as unregistered. `TestHelpFileCompleteness_Commands` and `..._IndexListsEveryCommand` fail for `quaff` until it has a help file and an index entry: do Task 8 Step 6's `quaff.template` and `keywords.yaml` edits now, and commit them in Step 12. Null probe: add `char.UseItem` text to `internal/usercommands/quaff.go` as a comment line `// UseItem`, see `TestQuaffWrappersDoNotReFork` fail, remove it.

- [ ] **Step 12: Commit**

```bash
git add internal/actions/drink.go internal/actions/quaff_quest_order_test.go internal/actions/drink_magnitude_test.go internal/actions/drink_parity_test.go internal/actions/drink_room_line_sight_test.go internal/actions/drink_scour_test.go internal/guards/condition_apply_path_guard_test.go internal/usercommands/quaff.go internal/usercommands/drink.go internal/usercommands/eat.go internal/usercommands/usercommands.go internal/usercommands/eat_drink_quaff_test.go internal/usercommands/busy_refuse_test.go internal/usercommands/bare_item_verb_prompt_test.go internal/usercommands/usercommands_test.go internal/mobcommands/quaff.go internal/mobcommands/drink.go internal/mobcommands/eat.go internal/mobcommands/mobcommands.go internal/mobcommands/mobcommands_test.go internal/mobcommands/quaff_wrapper_test.go internal/mobcommands/drink_wrapper_test.go internal/guards/drink_wrapper_guard_test.go internal/guards/messaging_surface_guard_test.go
git status --short
git commit -F - <<'EOF'
feat: quaff for potions; eat and drink are one food command

actions.Drink becomes actions.Quaff (#375 phase 7, owner ruling
2026-10-30), run by a new quaff command for players and mobs; its
prompt and refusals say quaff, and the quest engine hears quaff AFTER
the "You drink the" line (#697). eat and drink share consumeFood: both
take food and cooked drinks (subtype edible), the line says eat or drink
by the item, a potion is left in the pack with a line naming quaff, and
a cooked drink left in a bandolier is still found.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Also add the two help files made in Step 11 to this `git add`: `_datafiles/world/dogmud/templates/help/quaff.template` and `_datafiles/world/dogmud/keywords.yaml`. Task 8 then skips creating them and makes its other help edits.

---

## Task 6: Retype the cooking drinks; look text names the verb

**Executor:** Sonnet

**Files:**
- Modify: `_datafiles/world/dogmud/items/consumables-30000/30024-herbal_tea.yaml`, `30025-spiced_wine.yaml`, `30027-antidote_broth.yaml`; `internal/guards/potion_conditions_guard_test.go`; `internal/actions/drink_purge_shipped_test.go`; `internal/items/items.go`; `internal/items/itemspec.go`
- Create: `internal/items/longdesc_verb_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/items/longdesc_verb_test.go`:

```go
package items

import (
	"strings"
	"testing"
)

// Look text names the command that takes the item (#375 phase 7): quaff a
// potion, drink a cooked drink, eat food.
func TestLongDescriptionNamesTheVerb(t *testing.T) {
	var i Item
	for _, c := range []struct {
		spec ItemSpec
		want string
	}{
		{ItemSpec{Type: Potion, Subtype: Drinkable}, `<ansi fg="command">quaff</ansi>`},
		{ItemSpec{Type: Drink, Subtype: Edible}, `<ansi fg="command">drink</ansi>`},
		{ItemSpec{Type: Food, Subtype: Edible}, `<ansi fg="command">eat</ansi>`},
	} {
		if got := i.longDescriptionFrom(c.spec); !strings.Contains(got, c.want) {
			t.Errorf("%s/%s: %q, want it to name %s", c.spec.Type, c.spec.Subtype, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/items/ -run TestLongDescriptionNamesTheVerb 2>&1 | tail -4
```

Expected: the potion and drink rows fail.

- [ ] **Step 3: Look text and subtype descriptions**

In `internal/items/items.go`, Edit:

old:
```go
	} else if iSpec.Subtype == Drinkable {

		longDesc.WriteString("\n")
		longDesc.WriteString(` - You could probably <ansi fg="command">drink</ansi> this.`)

	} else if iSpec.Subtype == Edible {

		longDesc.WriteString("\n")
		longDesc.WriteString(` - You could probably <ansi fg="command">eat</ansi> this.`)
```
new:
```go
	} else if iSpec.Subtype == Drinkable {

		longDesc.WriteString("\n")
		longDesc.WriteString(` - You could probably <ansi fg="command">quaff</ansi> this.`)

	} else if iSpec.Subtype == Edible {

		longDesc.WriteString("\n")
		if iSpec.Type == Drink {
			longDesc.WriteString(` - You could probably <ansi fg="command">drink</ansi> this.`)
		} else {
			longDesc.WriteString(` - You could probably <ansi fg="command">eat</ansi> this.`)
		}
```

In `internal/items/itemspec.go`, Edit:

old:
```go
		{string(Drinkable), `Can be targetted with the drink command.`, 0, 0, 0},
		{string(Edible), `Can be targetted with the eat command.`, 0, 0, 0},
```
new:
```go
		{string(Drinkable), `Can be targetted with the quaff command (potions).`, 0, 0, 0},
		{string(Edible), `Can be targetted with the eat or drink command (food and cooked drinks).`, 0, 0, 0},
```

- [ ] **Step 4: Retype the three cooking drinks**

Replace the whole of `_datafiles/world/dogmud/items/consumables-30000/30024-herbal_tea.yaml` with Write:

```yaml
itemid: 30024
name: Herbal Tea
vendor_categories:
- cooking
namesimple: tea
description: |
  A small clay cup of pale green tea brewed from steeped healer's root.
  The liquid is warm and faintly bitter, with an aftertaste of something
  floral. Herbalists swear by it for settling the mind after hard work
  or long travel.
type: drink
subtype: edible
uses: 1
conditionids:
  - 44
value: 4
rarity_tier: 50
weight: 0.3
```

In `30025-spiced_wine.yaml`, Edit:

old:
```yaml
type: food
subtype: edible
uses: 1
conditionids:
  - 46
```
new:
```yaml
type: drink
subtype: edible
uses: 1
conditionids:
  - 46
```

In `30027-antidote_broth.yaml`, Edit:

old:
```yaml
type: food
subtype: edible
uses: 1
conditionids:
  - 42
value: 6
rarity_tier: 50
weight: 0.4
```
new:
```yaml
type: drink
subtype: edible
uses: 1
conditionids:
  - 42
value: 6
rarity_tier: 50
weight: 0.4
```

- [ ] **Step 5: Condition 44 leaves the purge set**

In `internal/guards/potion_conditions_guard_test.go`, Edit:

old:
```go
	for _, id := range []int{7, 44, 47, 48, 49, 51, 82, 130} {
		if !set[id] {
			t.Errorf("condition %d is granted only by a potion; it must be in the set", id)
		}
	}
```
new:
```go
	for _, id := range []int{7, 47, 48, 49, 51, 82, 130} {
		if !set[id] {
			t.Errorf("condition %d is granted only by a potion; it must be in the set", id)
		}
	}
	// 44 is Herbal Tea's. The tea is a cooked drink since #375 phase 7, and
	// the purge never strips a meal's condition.
	if set[44] {
		t.Error("condition 44 is Herbal Tea's, a cooked drink; it must not be in the set")
	}
```

In `internal/actions/drink_purge_shipped_test.go`, Edit:

old:
```go
	for _, id := range []int{7, 44, 47, 48, 49, 51, 82, 130} {
```
new:
```go
	for _, id := range []int{7, 47, 48, 49, 51, 82, 130} {
```

and Edit:

old:
```go
		5:  "also granted by food and bandages; stripping it would undo a meal",
```
new:
```go
		5:  "also granted by food and bandages; stripping it would undo a meal",
		44: "Herbal Tea's, a cooked drink since #375 phase 7; the purge never strips a meal's",
```

- [ ] **Step 6: Run and pass**

```bash
gofmt -l internal/ && go test ./internal/items/ ./internal/actions/ ./internal/guards/ ./internal/usercommands/ 2>&1 | tail -8
```

Expected: all `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/items/items.go internal/items/itemspec.go internal/items/longdesc_verb_test.go internal/guards/potion_conditions_guard_test.go internal/actions/drink_purge_shipped_test.go _datafiles/world/dogmud/items/consumables-30000/30024-herbal_tea.yaml _datafiles/world/dogmud/items/consumables-30000/30025-spiced_wine.yaml _datafiles/world/dogmud/items/consumables-30000/30027-antidote_broth.yaml
git commit -F - <<'EOF'
feat(items): cooking drinks are type drink; look text names the verb

Herbal Tea, Spiced Wine and Antidote Broth become type drink, subtype
edible, so eat and drink take them and the bandolier does not. The tea
loses its potion toxicity, and its condition leaves the purge's
potion-only set. Look text says quaff, drink or eat. (#375 phase 7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 7: Planner, AI companion and tab completion quaff potions

**Executor:** Sonnet

**Files:**
- Modify: `internal/planners/survival.go:48-52`, `modules/aicompanion/combat.go:467-472`, `modules/aicompanion/actions.go:434-438`, `modules/aicompanion/aicompanion_test.go`, `world.go:486-494`

- [ ] **Step 1: Write the failing companion test**

Append to `modules/aicompanion/aicompanion_test.go`:

```go
// A badly hurt companion tends itself with a potion through quaff, the
// potion command since #375 phase 7.
func TestReflexPotion_Quaffs(t *testing.T) {
	const tonicId = 39331
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tonicId: {ItemId: tonicId, Name: "Test Tonic", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
	}))
	m := &AICompanionModule{}
	mob := reflexFleeMob(t, 9104)
	mob.Character.Items = []items.Item{items.New(tonicId)}
	c := &controller{profile: &Profile{}, mind: &Mind{}, fight: &fightState{}}
	m.reflex(c, mob, nil, nil, 100, 30, 100, false)
	queued := events.DrainQueuedInputsForTest(mob.InstanceId)
	found := false
	for _, cmd := range queued {
		if cmd == `quaff test tonic` {
			found = true
		}
	}
	if !found {
		t.Fatalf("a hurt companion must quaff its potion; queued %v", queued)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./modules/aicompanion/ -run TestReflexPotion_Quaffs 2>&1 | tail -4
```

Expected: fails with `queued [drink test tonic]`.

- [ ] **Step 3: Implement**

In `modules/aicompanion/combat.go`, Edit:

old:
```go
			act(`drink ` + util.EscapeAnsiTags(strings.ToLower(name)))
```
new:
```go
			act(`quaff ` + util.EscapeAnsiTags(strings.ToLower(name)))
```

In `modules/aicompanion/actions.go`, Edit:

old:
```go
	case `drink`:
		if t.Kind != `carried` || t.Item.GetSpec().Subtype != items.Drinkable {
			return actionOutcome{Refused: `not something to drink`}
		}
		return m.issue(c, mob, `drink`, `drink `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)
```
new:
```go
	case `drink`:
		if t.Kind != `carried` {
			return actionOutcome{Refused: `not something to drink`}
		}
		switch t.Item.GetSpec().Subtype {
		case items.Drinkable:
			// A potion is drunk with quaff (#375 phase 7).
			return m.issue(c, mob, `drink`, `quaff `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)
		case items.Edible:
			// Food or a cooked drink: drink is eat under its other name.
			return m.issue(c, mob, `drink`, `drink `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)
		}
		return actionOutcome{Refused: `not something to drink`}
```

In `internal/planners/survival.go`, Edit:

old:
```go
	// Low HP, not in combat → drink a healing potion if one is available.
	if hpPct < fleePct {
		if potion := pickHealingPotion(mob); potion != "" {
			return PlanResult{Command: "drink " + potion, Status: StatusRunning}
```
new:
```go
	// Low HP, not in combat → quaff a healing potion if one is available.
	if hpPct < fleePct {
		if potion := pickHealingPotion(mob); potion != "" {
			return PlanResult{Command: "quaff " + potion, Status: StatusRunning}
```

In `world.go`, Edit:

old:
```go
		} else if cmd == `eat` {

			itemList = user.Character.GetAllBackpackItems()
			itemSubtypeSearch = append(itemSubtypeSearch, items.Edible)

		} else if cmd == `drink` {

			itemList = user.Character.GetAllBackpackItems()
			itemSubtypeSearch = append(itemSubtypeSearch, items.Drinkable)
```
new:
```go
		} else if cmd == `eat` || cmd == `drink` {

			itemList = user.Character.GetAllBackpackItems()
			itemSubtypeSearch = append(itemSubtypeSearch, items.Edible)

		} else if cmd == `quaff` {

			itemList = user.Character.GetAllBackpackItems()
			itemSubtypeSearch = append(itemSubtypeSearch, items.Drinkable)
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/ && go build ./... && go test ./modules/aicompanion/ ./internal/planners/ . 2>&1 | tail -6
```

Expected: `ok`. Quest content is untouched until Task 8, so the root package's shipped-data tests still see quest 55's `command=drink` finding and its matching backlog line.

- [ ] **Step 5: Commit**

```bash
git add internal/planners/survival.go modules/aicompanion/combat.go modules/aicompanion/actions.go modules/aicompanion/aicompanion_test.go world.go
git commit -F - <<'EOF'
feat(quaff): companion, survival planner and tab completion quaff potions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 8: Content and help say quaff for potions

**Executor:** Sonnet

**Files:**
- Modify: `_datafiles/world/dogmud/quests/38-first_brew.yaml`, `55-another_brew.yaml`, `loop-backlog.txt`; `_datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml`, `9129.yaml`; `internal/questengine/newbie_alchemy_steps_test.go`, `newbie_topic_steps_test.go`; `_datafiles/world/dogmud/templates/help/drink.template`, `eat.template`, `cooking.template`, `alchemy.template`, `equipment.template`, `toxicity.template`; `_datafiles/world/dogmud/keywords.yaml`; `_datafiles/html/public/webclient-pure.html`
- Create: `_datafiles/world/dogmud/templates/help/quaff.template`

Scope rule: change only the trigger command and the "Type drink ..." instructions for potions. Every other edit to quests 38, 39, 55 and dialogues 9128, 9129 belongs to plan 3.

- [ ] **Step 1: Tests first**

In `internal/questengine/newbie_alchemy_steps_test.go`, Edit each of the five `Command: "drink"` occurrences to `Command: "quaff"` (lines 48, 59, 81, 144, 151 on master), and the two messages `"a drink before the brew must not count"` to `"a quaff before the brew must not count"` and `"a drink before the brew must not finish the quest"` to `"a quaff before the brew must not finish the quest"`. Leave step ids `"drink"` (`require.Equal(t, "drink", w.p.current)`, `.at("drink")`, the `[]string{"start", "brew", "drink", "end"}` list) unchanged: they are saved step ids.

In `internal/questengine/newbie_topic_steps_test.go`, Edit:

old:
```go
		{f, 38, "brew", "type drink healing salve", pre, "brew_step_drink", false},
```
new:
```go
		{f, 38, "brew", "type quaff healing salve", pre, "brew_step_drink", false},
```

old:
```go
		{"55-another_brew.yaml", 55, "brew", "type drink healing salve", []string{"38-end", "39-end", "40-end"}, "brew_loop_step_drink", false},
```
new:
```go
		{"55-another_brew.yaml", 55, "brew", "type quaff healing salve", []string{"38-end", "39-end", "40-end"}, "brew_loop_step_drink", false},
```

- [ ] **Step 2: See them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest38|TestQuest55|TestBirna9128|TestFalv9129' 2>&1 | tail -8
```

Expected: failures on quests 38 and 55 (the trigger still says drink) and on the two topic hints.

- [ ] **Step 3: Quest 38**

In `quests/38-first_brew.yaml`, Edit:

old:
```yaml
#   38-drink: command trigger on drink (anywhere). Teaches the drink command.
```
new:
```yaml
#   38-drink: command trigger on quaff (anywhere). Teaches the quaff command.
```

old:
```yaml
      You brewed a healing salve. Now drink it. A brew is only worth what
      it does.
    hint: >-
      Type drink healing salve.
```
new:
```yaml
      You brewed a healing salve. Now drink it. A brew is only worth what
      it does.
    hint: >-
      Type quaff healing salve.
```

The next two edits share text; make each with enough context to be unique (include the `room: 5265` or `room: 5267` line above). For both craft triggers:

old (each):
```yaml
          healing salve is sealed in the vial. Type drink healing salve.
```
new (each):
```yaml
          healing salve is sealed in the vial. Type quaff healing salve.
```

old:
```yaml
  # ── DRINK: drink the salve (anywhere) ───────────────────────────────
  - event: command
    command: drink
```
new:
```yaml
  # ── DRINK: quaff the salve (anywhere) ───────────────────────────────
  - event: command
    command: quaff
```

- [ ] **Step 4: Quest 55 and its backlog line**

In `quests/55-another_brew.yaml`, Edit:

old:
```yaml
#   55-end:  command trigger on drink (anywhere, no room), once brewed.
```
new:
```yaml
#   55-end:  command trigger on quaff (anywhere, no room), once brewed.
```

old:
```yaml
    hint: "Type drink healing salve."
```
new:
```yaml
    hint: "Type quaff healing salve."
```

For both craft triggers (each with its `room:` line for uniqueness):

old (each):
```yaml
          Type drink healing salve, or drink whatever you made.
```
new (each):
```yaml
          Type quaff healing salve, or quaff whatever you made.
```

old:
```yaml
  - event: command
    command: drink
    conditions:
      has: ["55-brew"]
```
new:
```yaml
  - event: command
    command: quaff
    conditions:
      has: ["55-brew"]
```

In `quests/loop-backlog.txt`, Edit:

old:
```
end 55 quest 55 trigger command command=drink grants=55-end
```
new:
```
end 55 quest 55 trigger command command=quaff grants=55-end
```

- [ ] **Step 5: Dialogue hints**

In `dialogue/pothole_coulee/9128.yaml`, Edit:

old:
```yaml
    text: You made the salve. A brew is only worth what it does. Drink it down and
      feel it work.
    hints: Type drink healing salve.
```
new:
```yaml
    text: You made the salve. A brew is only worth what it does. Drink it down and
      feel it work.
    hints: Type quaff healing salve.
```

old:
```yaml
  text: The salve is made. Drink it. You will feel it mend you, slow and warm.
  hints: Type drink healing salve.
```
new:
```yaml
  text: The salve is made. Drink it. You will feel it mend you, slow and warm.
  hints: Type quaff healing salve.
```

In `dialogue/pothole_coulee/9129.yaml`, Edit:

old:
```yaml
    text: You brewed. Now drink it. That is all I need to know.
    hints: Type drink healing salve.
```
new:
```yaml
    text: You brewed. Now drink it. That is all I need to know.
    hints: Type quaff healing salve.
```

old:
```yaml
    tell me.
  hints: Type drink healing salve.
```
new:
```yaml
    tell me.
  hints: Type quaff healing salve.
```

Verify: `grep -rn "Type drink" _datafiles/world/dogmud/quests _datafiles/world/dogmud/dialogue` prints nothing, and the same grep for `Type quaff` prints 10 lines.

- [ ] **Step 6: Help files**

Task 5 Step 11 makes the `quaff.template` file and the `keywords.yaml` entry below and commits them; if they exist, skip those two edits here and leave both paths out of this task's `git add`.

Create `_datafiles/world/dogmud/templates/help/quaff.template` with Write:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">quaff</ansi>

The <ansi fg="command">quaff</ansi> command drinks a potion you carry.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">quaff healing salve</ansi>
  This would drink your healing salve.

<ansi fg="yellow">Good to know:</ansi>

  - If you wear a potion bandolier, <ansi fg="command">quaff</ansi> draws from it first
    (oldest potion first), then from your backpack.
  - Potions age: a brew grows stronger as it ferments toward its
    peak, then declines. A spoiled potion will make you ill, so
    <ansi fg="command">look</ansi> at it first if it has been sitting a while.
  - Potions carry toxicity. Quaff too many too fast and the poison
    builds up in you; see <ansi fg="command">help toxicity</ansi>.
  - You cannot quaff while grappled, because both hands are
    committed. Quaffing also waits while you are focused on
    crafting, salvaging, or casting.
  - Food and cooked drinks such as tea or wine are not potions.
    Use <ansi fg="command">eat</ansi> or <ansi fg="command">drink</ansi> for those.

Find out more about referring to items by name by typing <ansi fg="command">help item-names</ansi>.
```

Replace the whole of `templates/help/drink.template` with Write:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">drink</ansi>

The <ansi fg="command">drink</ansi> command drinks a cooked drink you carry, such as tea,
wine or broth. It is the same command as <ansi fg="command">eat</ansi>: either word
takes food or a cooked drink.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">drink tea</ansi>
  This would drink your tea.

Potions are different. Drink a potion with <ansi fg="command">quaff</ansi>
(see <ansi fg="command">help quaff</ansi>).

You cannot drink while grappled, because both hands are committed.
Drinking also waits while you are focused on crafting, salvaging, or
casting.

If something else in your pack shares the name, <ansi fg="command">drink</ansi> reaches
for the food or drink first.

Find out more about referring to items by name by typing
<ansi fg="command">help item-names</ansi>.
```

Replace the whole of `templates/help/eat.template` with Write:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">eat</ansi>

The <ansi fg="command">eat</ansi> command eats food you carry. It is the same command as
<ansi fg="command">drink</ansi>: either word takes food or a cooked drink. Potions are
drunk with <ansi fg="command">quaff</ansi>.

Food that has been left too long may spoil and become inedible.
If the food has gone bad, you will be unable to eat it and must
discard it.

You cannot eat while grappled, because both hands are committed.
Free yourself first. Eating also has to wait while you are focused
on crafting, salvaging, or casting.

If something else in your pack shares the food's name, <ansi fg="command">eat</ansi>
reaches for the food first.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">eat sandwich</ansi>
  This would eat your sandwich.

Find out more about referring to items by name by typing
<ansi fg="command">help item-names</ansi>.
```

In `templates/help/cooking.template`, Edit:

old:
```
Crafted meals are used with the <ansi fg="command">eat</ansi> command:

  <ansi fg="command">eat meal</ansi>
```
new:
```
Crafted meals are used with the <ansi fg="command">eat</ansi> command, and cooked drinks
such as tea with <ansi fg="command">drink</ansi>. Either word works for both:

  <ansi fg="command">eat meal</ansi>
  <ansi fg="command">drink tea</ansi>
```

Edit:

old:
```
  <ansi fg="command">eat <item></ansi>         Consume a cooked meal.
```
new:
```
  <ansi fg="command">eat <item></ansi>         Consume a cooked meal or drink.
```

In `templates/help/alchemy.template`, Edit:

old:
```
Crafted potions and poultices are used with the <ansi fg="command">drink</ansi> command:

  <ansi fg="command">drink poultice</ansi>
  <ansi fg="command">drink draught</ansi>
```
new:
```
Crafted potions and poultices are drunk with the <ansi fg="command">quaff</ansi> command:

  <ansi fg="command">quaff poultice</ansi>
  <ansi fg="command">quaff draught</ansi>
```

old:
```
  <ansi fg="command">drink <item></ansi>        Consume a crafted potion or poultice.
```
new:
```
  <ansi fg="command">quaff <item></ansi>        Consume a crafted potion or poultice.
```

In `templates/help/equipment.template`, Edit:

old:
```
stored in the bandolier. When you <ansi fg="command">drink</ansi> a potion,
```
new:
```
stored in the bandolier. When you <ansi fg="command">quaff</ansi> a potion,
```

In `templates/help/toxicity.template`, Edit:

old:
```
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help alchemy</ansi>, <ansi fg="command">help drink</ansi>, <ansi fg="command">help craft</ansi>,
```
new:
```
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help alchemy</ansi>, <ansi fg="command">help quaff</ansi>, <ansi fg="command">help craft</ansi>,
```

In `keywords.yaml`, Edit:

old:
```yaml
      - put
      - read
      - remove
```
new:
```yaml
      - put
      - quaff
      - read
      - remove
```

- [ ] **Step 7: Web client**

In `_datafiles/html/public/webclient-pure.html`, Edit:

old:
```js
                if (type === "potion" || type === "drink" || subtype === "potion" || subtype === "drink") return "drink";
                if (type === "food"   || subtype === "food")   return "eat";
```
new:
```js
                // Potions are quaffed; cooked drinks are drunk and food is
                // eaten (eat and drink are one command, #375 phase 7).
                if (type === "potion" || subtype === "drinkable") return "quaff";
                if (type === "drink") return "drink";
                if (type === "food"   || subtype === "edible") return "eat";
```

Edit:

old:
```js
                cmdsInput.placeholder = "e.g. drink health;drink mana";
```
new:
```js
                cmdsInput.placeholder = "e.g. quaff health;quaff mana";
```

The comment at line 1729 lists ordinary commands ("drink, wear, sneak"); `drink` is still a command, so leave it.

- [ ] **Step 8: Run and pass**

```bash
go test ./internal/questengine/ ./internal/usercommands/ ./internal/guards/ 2>&1 | tail -8
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)' | tail -12
```

Expected: all `ok`, and the `--- PASS` lines show the root tests ran (without `DOGMUD_BOOT_SMOKE=1` they SKIP and prove nothing). The root run proves the backlog line matches the renamed finding: a mismatch reads as a listed line with no finding plus an unlisted finding.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/quests/38-first_brew.yaml _datafiles/world/dogmud/quests/55-another_brew.yaml _datafiles/world/dogmud/quests/loop-backlog.txt _datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml internal/questengine/newbie_alchemy_steps_test.go internal/questengine/newbie_topic_steps_test.go _datafiles/world/dogmud/templates/help/quaff.template _datafiles/world/dogmud/templates/help/drink.template _datafiles/world/dogmud/templates/help/eat.template _datafiles/world/dogmud/templates/help/cooking.template _datafiles/world/dogmud/templates/help/alchemy.template _datafiles/world/dogmud/templates/help/equipment.template _datafiles/world/dogmud/templates/help/toxicity.template _datafiles/world/dogmud/keywords.yaml _datafiles/html/public/webclient-pure.html
git commit -F - <<'EOF'
content: potion steps and help say quaff (#375 phase 7)

Quests 38 and 55 trigger on quaff and their ten "Type drink healing
salve" instructions say quaff; quest 55's backlog key follows the
renamed trigger. New help quaff; drink, eat, cooking, alchemy,
equipment and toxicity help match. Web client right-click quaffs a
potion.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 9: Raw angle brackets in player text (#690)

**Executor:** Sonnet

**Files:**
- Create: `internal/guards/html_entity_guard_test.go`
- Modify: `internal/usercommands/hint.go:19`, `hint_test.go`, `admin.ai.go:23`, `channel.go:31,81`, `guild.go:480`, `set.go:436,475,485,501`, `shoot.go:142`

- [ ] **Step 1: Write the guard and the wire test**

Create `internal/guards/html_entity_guard_test.go`:

```go
package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Player text reaches telnet and the web client through ansitags, which
// recognises only <ansi ...> and </ansi> and passes every other byte through
// as written (internal/util/ansiescape.go). Nothing on the way decodes an
// HTML entity, so "talk &lt;name&gt;" reached players literally (#690).
// Write the angle brackets themselves: "talk <name>".
var htmlEntityInGo = regexp.MustCompile(`&(lt|gt);`)

// htmlEntityExempt lists files whose literal is HTML, where the entity is right.
var htmlEntityExempt = map[string]string{
	"internal/playtestenv/report.go": "writes an HTML report",
}

func TestPlayerTextHasNoHTMLEntities(t *testing.T) {
	if !htmlEntityInGo.MatchString(`talk &lt;name&gt;`) {
		t.Fatal("probe: the pattern cannot match an entity")
	}
	root := repoRoot(t)
	fset := token.NewFileSet()
	scanned := 0
	for _, dir := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
			if _, ok := htmlEntityExempt[rel]; ok {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			scanned++
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING && htmlEntityInGo.MatchString(lit.Value) {
					t.Errorf("%s: an HTML entity reaches the player literally; write < and >: %s",
						fset.Position(lit.Pos()), lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d Go files; the walk is not reaching the code", scanned)
	}
}
```

In `internal/usercommands/hint_test.go`, Edit:

old:
```go
	// The angle brackets travel escaped, as in every other usage line.
	assert.Contains(t, told, "talk &lt;name&gt;")
```
new:
```go
	// told has its tags stripped (aimTold), which also strips "<name>";
	// TestHint_NoQuestLineShowsTalkNameOnTheWire checks the brackets.
	assert.Contains(t, told, "Type talk")
	assert.NotContains(t, told, "&lt;")
```

Append to `internal/usercommands/hint_test.go` (add imports `strings` and `github.com/GoMudEngine/ansitags`):

```go
// #690: the hint reaches a player through ansitags, which passes "<name>"
// through and does not decode "&lt;", so the line holds raw brackets.
func TestHint_NoQuestLineShowsTalkNameOnTheWire(t *testing.T) {
	user, room := aimScene(t, aimFull)
	user.Character.QuestProgress = nil
	events.DrainQueuedMessagesForTest(1)
	_, err := Hint("", user, room, events.EventFlag(0))
	require.NoError(t, err)
	wire := ansitags.Parse(strings.Join(events.DrainQueuedMessagesForTest(1), ""))
	assert.Contains(t, wire, "talk <name>")
	assert.NotContains(t, wire, "&lt;")
}
```

- [ ] **Step 2: See them fail**

```bash
go test ./internal/guards/ -run TestPlayerTextHasNoHTMLEntities 2>&1 | tail -15
go test ./internal/usercommands/ -run TestHint_ 2>&1 | tail -6
```

Expected: the guard names eleven literals in `admin.ai.go`, `channel.go` (2), `guild.go`, `hint.go`, `set.go` (4), `shoot.go`; the wire test fails on `talk <name>`. This run is the guard's null probe.

- [ ] **Step 3: Fix each literal**

In each named literal, replace `&lt;` with `<` and `&gt;` with `>` using Edit, character for character. The exact current literals (master):

- `hint.go:19`: `talk &lt;name&gt;` becomes `talk <name>`
- `admin.ai.go:23`: `ai-flag &lt;username&gt;` becomes `ai-flag <username>`
- `channel.go:31`: `</ansi> &lt;message&gt;` becomes `</ansi> <message>`
- `channel.go:81`: `(channels &lt;name&gt; to toggle):` becomes `(channels <name> to toggle):`
- `guild.go:480`: `guild chat &lt;message&gt;` and `gc &lt;message&gt;` become `guild chat <message>` and `gc <message>`
- `set.go:436`: `set submission &lt;mercy|subdue|cripple|lethal&gt;` becomes `set submission <mercy|subdue|cripple|lethal>`
- `set.go:475`: `set surrender &lt;never|always|auto-tap-below &lt;N&gt;&gt;` becomes `set surrender <never|always|auto-tap-below <N>>`
- `set.go:485`: `auto-tap-below &lt;1-100&gt;.` becomes `auto-tap-below <1-100>.`
- `set.go:501`: `set arrest &lt;surrender|resist&gt;` becomes `set arrest <surrender|resist>`
- `shoot.go:142`: `shoot &lt;target&gt; [direction]` becomes `shoot <target> [direction]`

None of the new strings starts a `<ansi` or `</ansi`, so ansitags passes them through.

- [ ] **Step 4: Run and pass**

```bash
go test ./internal/guards/ -run TestPlayerTextHasNoHTMLEntities 2>&1 | tail -4
go test ./internal/usercommands/ 2>&1 | tail -4
```

Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add internal/guards/html_entity_guard_test.go internal/usercommands/hint.go internal/usercommands/hint_test.go internal/usercommands/admin.ai.go internal/usercommands/channel.go internal/usercommands/guild.go internal/usercommands/set.go internal/usercommands/shoot.go
git commit -F - <<'EOF'
fix(text): write raw angle brackets in usage lines (#690)

ansitags passes "<name>" through and decodes no HTML entity, so
"talk &lt;name&gt;" reached players literally on telnet and the web
client. Fixes hint and the ten other usage literals that had the same
escape, and adds a guard over Go string literals.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 10: `consider` copy without "them" and without a repeated pace (#690)

**Executor:** Sonnet

**Files:**
- Modify: `internal/actions/fight_read.go:163-211`, `internal/actions/fight_read_test.go`, `modules/aicompanion/aicompanion_test.go:506`

- [ ] **Step 1: Write the failing test**

Append to `internal/actions/fight_read_test.go`:

```go
// #690: a read against a straw dummy said "them" for it and "an age" twice.
// The copy names no pronoun for the target, and a pace never repeats in one
// line: equal paces read as one sentence.
func TestFightReadCopy_NoTargetPronounAndNoRepeat(t *testing.T) {
	pronoun := regexp.MustCompile(`(?i)\b(them|they|they'd|their)\b`)
	for v := VerdictUntouched; v <= VerdictStalemate; v++ {
		if pronoun.MatchString(verdictText[v]) {
			t.Errorf("verdict %v names the target by pronoun: %q", v, verdictText[v])
		}
	}
	for yp := PaceMoments; yp <= PaceNever; yp++ {
		for tp := PaceMoments; tp <= PaceNever; tp++ {
			l := paceText(FightRead{YourPace: yp, TheirPace: tp})
			if pronoun.MatchString(l) {
				t.Errorf("pace %v/%v names the target by pronoun: %q", yp, tp, l)
			}
			for _, p := range []string{"an age", "moments", "a short fight", "a long fight"} {
				if strings.Count(l, p) > 1 {
					t.Errorf("pace %v/%v repeats %q: %q", yp, tp, p, l)
				}
			}
		}
	}
	if got, want := paceText(FightRead{YourPace: PaceAge, TheirPace: PaceNever}), "Ending it would take you an age; you could never fall."; got != want {
		t.Errorf("effigy pace: %q, want %q", got, want)
	}
	if got, want := paceText(FightRead{YourPace: PaceAge, TheirPace: PaceAge}), "Either of you would need an age to end it."; got != want {
		t.Errorf("equal paces: %q, want %q", got, want)
	}
}
```

Add `"regexp"` to the file's imports if absent.

- [ ] **Step 2: See it fail**

```bash
go test ./internal/actions/ -run TestFightReadCopy 2>&1 | tail -8
```

Expected: pronoun and repeat errors.

- [ ] **Step 3: Rewrite the tables**

In `internal/actions/fight_read.go`, Edit:

old:
```go
	VerdictUntouched:  `They won't lay a finger on you.`,
```
new:
```go
	VerdictUntouched:  `You won't take a scratch.`,
```

Edit:

old:
```go
var yourPaceText = map[Pace]string{
	PaceMoments: `you could beat them in moments`,
	PaceShort:   `you could beat them in a short fight`,
	PaceLong:    `beating them would be a long fight`,
	PaceAge:     `beating them would take an age`,
	PaceNever:   `you could never beat them`,
}

var theirPaceText = map[Pace]string{
	PaceMoments: `they could beat you in moments`,
	PaceShort:   `they could beat you in a short fight`,
	PaceLong:    `they'd need a long fight to beat you`,
	PaceAge:     `beating you would take them an age`,
	PaceNever:   `they could never beat you`,
}
```
new:
```go
// The pace copy names no pronoun for the target, which may be a person, a
// beast or a straw dummy (#690): "it" is the fight, and the second clause is
// how long the reader holds out.
var yourPaceText = map[Pace]string{
	PaceMoments: `you could end it in moments`,
	PaceShort:   `you could end it in a short fight`,
	PaceLong:    `ending it would take you a long fight`,
	PaceAge:     `ending it would take you an age`,
	PaceNever:   `you could never end it`,
}

var theirPaceText = map[Pace]string{
	PaceMoments: `you could fall in moments`,
	PaceShort:   `you could fall in a short fight`,
	PaceLong:    `you could hold out through a long fight`,
	PaceAge:     `you could hold out for an age`,
	PaceNever:   `you could never fall`,
}

// samePaceText is the one sentence when both paces fall in the same band,
// so a band is never said twice in one line (#690).
var samePaceText = map[Pace]string{
	PaceMoments: `Either of you could end it in moments.`,
	PaceShort:   `Either of you could end it in a short fight.`,
	PaceLong:    `Either of you would need a long fight to end it.`,
	PaceAge:     `Either of you would need an age to end it.`,
	PaceNever:   `Neither of you could ever end it.`,
}
```

Edit:

old:
```go
// paceText joins the two paces into one sentence.
func paceText(r FightRead) string {
	s := yourPaceText[r.YourPace] + `; ` + theirPaceText[r.TheirPace] + `.`
```
new:
```go
// paceText joins the two paces into one sentence, or says the shared band
// once when they match.
func paceText(r FightRead) string {
	if r.YourPace == r.TheirPace {
		return samePaceText[r.YourPace]
	}
	s := yourPaceText[r.YourPace] + `; ` + theirPaceText[r.TheirPace] + `.`
```

Edit the verdict comment at line 20:

old:
```go
	VerdictUntouched  Verdict = iota // they won't lay a finger on you
```
new:
```go
	VerdictUntouched  Verdict = iota // you won't take a scratch
```

In `modules/aicompanion/aicompanion_test.go`, Edit:

old:
```go
!strings.HasPrefix(got, `They won't lay a finger on you.`) {
```
new:
```go
!strings.HasPrefix(got, `You won't take a scratch.`) {
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ && go test ./internal/actions/ ./modules/aicompanion/ ./internal/guards/ 2>&1 | tail -6
```

Expected: all `ok`, including `TestFightReadCopy_FitsAndHasNoNumbers` (80 columns, no digits).

- [ ] **Step 5: Commit**

```bash
git add internal/actions/fight_read.go internal/actions/fight_read_test.go modules/aicompanion/aicompanion_test.go
git commit -F - <<'EOF'
fix(consider): no "them" for the target, no pace said twice (#690)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 11: The Straw Effigy shows it shrugs off the blow (#690)

**Executor:** Sonnet

**Files:**
- Create: `_datafiles/world/dogmud/behaviors/newcomer_antechamber/9614-straw_effigy.yaml`, `internal/behaviortree/effigy_tree_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/behaviortree/effigy_tree_test.go`:

```go
package behaviortree

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// #690: the Straw Effigy's vitality is set very high so a new player cannot
// drop it, so the fight prompt reads [healthy] however hard it is hit, while
// a critical swing still reads DEVASTATING (combat words a crit by the
// swing, not by the target's health). Its tree says why: now and then, when
// struck, it settles back into shape.
func TestStrawEffigySettlesWhenHurt(t *testing.T) {
	var emoted []string
	chance := Success
	origEmote, origChance := actionRegistry["emote"], conditionRegistry["random_chance"]
	actionRegistry["emote"] = func(params map[string]any, ctx *EvalContext) Result {
		emoted = append(emoted, getStringParam(params, "text"))
		return Success
	}
	conditionRegistry["random_chance"] = func(map[string]any, *EvalContext) Result { return chance }
	t.Cleanup(func() {
		actionRegistry["emote"] = origEmote
		conditionRegistry["random_chance"] = origChance
	})

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud",
		"behaviors", "newcomer_antechamber", "9614-straw_effigy.yaml")
	require.NoError(t, GetEngine().LoadTree(9614, tree))
	t.Cleanup(func() { GetEngine().EvictTree(9614) })

	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_hurt"}, NewBehaviorState())
	require.Len(t, emoted, 1, "a blow that passes the chance sets it settling")
	require.Contains(t, emoted[0], "no worse for the blow")

	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_idle"}, NewBehaviorState())
	require.Len(t, emoted, 1, "only a blow sets it settling")

	chance = Failure
	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_hurt"}, NewBehaviorState())
	require.Len(t, emoted, 1, "the line is paced by its chance, not every hit")
}
```

- [ ] **Step 2: See it fail**

```bash
go test ./internal/behaviortree/ -run TestStrawEffigySettlesWhenHurt 2>&1 | tail -4
```

Expected: `LoadTree` fails, file not found.

- [ ] **Step 3: The tree**

Create `_datafiles/world/dogmud/behaviors/newcomer_antechamber/9614-straw_effigy.yaml` with Write:

```yaml
# Straw Effigy (9614), the practice target in the Newcomer Antechamber.
#
# #690: its vitality is set very high so a new player cannot drop it during
# quest 28's lesson, so the fight prompt reads [healthy] however hard it is
# hit, while a critical swing still reads DEVASTATING (combat words a crit by
# the swing, not by the target's health). Now and then, when struck, it says
# why: it settles back into shape, no worse for the blow. Per-mob first; the
# combat_passive archetype has no mob_hurt branch, so nothing is lost when
# this one fires or falls through.

tree:
  type: selector
  children:
    - type: sequence
      event: mob_hurt
      children:
        - type: condition
          check: random_chance
          percent: 30
        - type: action
          do: emote
          text: rustles and settles back into shape, no worse for the blow.
```

- [ ] **Step 4: Run and pass**

```bash
go test ./internal/behaviortree/ 2>&1 | tail -4
```

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/behaviors/newcomer_antechamber/9614-straw_effigy.yaml internal/behaviortree/effigy_tree_test.go
git commit -F - <<'EOF'
content: the Straw Effigy shows it shrugs off a blow (#690)

A crit reads DEVASTATING by the swing while the prompt reads [healthy]
by the effigy's very high vitality; both are true. A per-mob tree now
emotes, on a share of hits, that it settles back into shape.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 12: A typed command that closes a talk menu says so (owner ruling (a))

**Executor:** Sonnet

**Files:**
- Modify: `internal/menu/lines.go:34-54`, `internal/menu/lines_test.go`, `internal/users/userrecord.menu.go:343-346,382-384`, `internal/users/userrecord_menu_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/menu/lines_test.go`, Edit:

old:
```go
	for _, silent := range []CloseReason{CloseNone, CloseChosen, CloseLeft, CloseOtherCommand, CloseReplaced, CloseDead, CloseFollow} {
```
new:
```go
	// A typed command that closes a talk menu names the conversation (owner
	// ruling 2026-10-30, #690); a menu with no topic has none to name.
	if got := InterruptLine(CloseOtherCommand, "talk with Kael"); got != "You break off your talk with Kael." {
		t.Errorf("other command, topic: %q", got)
	}
	if got := InterruptLine(CloseOtherCommand, ""); got != "" {
		t.Errorf("other command, no topic, should be silent: %q", got)
	}
	for _, silent := range []CloseReason{CloseNone, CloseChosen, CloseLeft, CloseReplaced, CloseDead, CloseFollow} {
```

In `internal/users/userrecord_menu_test.go`, Edit:

old:
```go
func TestRouteMenuInput_OtherCommandClosesAndPassesThrough(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if u.RouteMenuInput("north", true) {
		t.Fatal("north was consumed")
	}
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseOtherCommand {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}
```
new:
```go
func TestRouteMenuInput_OtherCommandClosesAndPassesThrough(t *testing.T) {
	u, r := menuUser(), &rec{}
	s := r.session()
	s.Topic = "talk with Kael"
	u.OpenMenu(testMenu(), s)
	events.DrainQueuedMessagesForTest(u.UserId)
	if u.RouteMenuInput("north", true) {
		t.Fatal("north was consumed")
	}
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseOtherCommand {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
	// Owner ruling 2026-10-30 (#690): the player is told which conversation
	// the command ended, before the command's own output.
	out := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	if !strings.Contains(out, "You break off your talk with Kael.") {
		t.Fatalf("no break-off line: %q", out)
	}
}

// A menu with no topic (a confirm, a builder list) closes silently.
func TestRouteMenuInput_OtherCommandClosesATopiclessMenuSilently(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	events.DrainQueuedMessagesForTest(u.UserId)
	u.RouteMenuInput("north", true)
	if out := events.DrainQueuedMessagesForTest(u.UserId); len(out) != 0 {
		t.Fatalf("a topic-less menu spoke on close: %q", out)
	}
}
```

Add imports `"strings"` and `"github.com/GoMudEngine/GoMud/internal/events"` to that file.

- [ ] **Step 2: See them fail**

```bash
go test ./internal/menu/ ./internal/users/ -run 'TestInterruptLine|TestRouteMenuInput_OtherCommand' 2>&1 | tail -8
```

Expected: the menu test fails on the topic line; the users test fails with `no break-off line`.

- [ ] **Step 3: Implement**

In `internal/menu/lines.go`, Edit:

old:
```go
// InterruptLine is the one line the player reads when an interrupt closes
// their menu, or "" when the close is silent (their own choice, a command
// whose output speaks for itself, death, a follow pull).
func InterruptLine(r CloseReason, topic string) string {
	if r == ClosePartnerLeft {
```
new:
```go
// InterruptLine is the one line the player reads when an interrupt closes
// their menu, or "" when the close is silent (their own choice, death, a
// follow pull, a typed command closing a menu with no topic).
func InterruptLine(r CloseReason, topic string) string {
	if r == CloseOtherCommand {
		// A typed command closed the menu (owner ruling 2026-10-30, #690):
		// talk menus number lines by position, so name the conversation the
		// command ended. A menu with no topic has none to name.
		if topic == "" {
			return ""
		}
		return "You break off your " + topic + "."
	}
	if r == ClosePartnerLeft {
```

In `internal/users/userrecord.menu.go`, Edit:

old:
```go
// RouteMenuInput offers a line to the open game menu. It returns true when
// the menu consumed the line. A line the menu does not recognise closes it
// (menu.CloseOtherCommand) and returns false so the caller runs it as a
// command. Only typed lines may answer.
```
new:
```go
// RouteMenuInput offers a line to the open game menu. It returns true when
// the menu consumed the line. A line the menu does not recognise closes it
// (menu.CloseOtherCommand), naming the conversation it ended when the menu
// has a topic, and returns false so the caller runs it as a command. Only
// typed lines may answer.
```

Edit:

old:
```go
	default:
		u.closeMenuWith(om, menu.CloseOtherCommand, false)
		return false
```
new:
```go
	default:
		// Announced (owner ruling 2026-10-30, #690): menu.InterruptLine names
		// the conversation, or stays silent for a menu with no topic. The
		// line is queued before the command runs, so it reads first.
		u.closeMenuWith(om, menu.CloseOtherCommand, true)
		return false
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ && go test ./internal/menu/ ./internal/users/ ./internal/conversation/ ./internal/usercommands/ ./internal/hooks/ 2>&1 | tail -8
```

Expected: all `ok`. If a conversation or usercommands test compares exact output after typing a command over an open talk menu, add the expected `You break off your talk with <Name>.` line to it; do not silence the close.

- [ ] **Step 5: Commit**

```bash
git add internal/menu/lines.go internal/menu/lines_test.go internal/users/userrecord.menu.go internal/users/userrecord_menu_test.go
git commit -F - <<'EOF'
feat(menu): a typed command closing a talk menu names it (#690)

Owner ruling (a), 2026-10-30: numbering stays; a command typed over an
open talk menu prints "You break off your talk with <name>." before its
own output. Menus with no topic still close silently.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 13: `context.md` and the copy guard

**Executor:** Sonnet

**Files:**
- Modify: `internal/actions/context.md`, `internal/usercommands/context.md`, `internal/mobcommands/context.md`, `internal/items/context.md`, `internal/configs/context.md`, `internal/refusal/context.md`, `internal/menu/context.md`, `internal/users/context.md`, `internal/planners/context.md`, `modules/aicompanion/context.md`, `internal/guards/copy_no_dash_test.go`

Before each edit, `grep -n` the old text to confirm it is still there.

- [ ] **Step 1: actions**

In `internal/actions/context.md`, replace the section heading and first paragraph:

old:
```
## Drink (`drink.go`, drink path unification 2026-09-28)

`Drink(actor DrinkActor, rest string) DrinkResult` is the ONE drink body for
players, mobs and the AI companion. It was `usercommands.Drink`; the mob
command was a separate copy with no toxicity, no aging or crafter scaling and
no special potions. `usercommands.Drink` and `mobcommands.Drink` are now thin
wrappers, and `internal/guards/drink_wrapper_guard_test.go`
(`TestDrinkWrappersDoNotReFork`) fails if either grows drink rules again.
A bare `drink` (nothing named) gets the prompt "Drink what? Try **drink
&lt;item&gt;**." and `DrinkRefuseNotFound`, before any other check.
```
new:
```
## Quaff (`drink.go`, drink path unification 2026-09-28; quaff since #375 phase 7)

`Quaff(actor DrinkActor, rest string) DrinkResult` is the ONE potion body for
players, mobs and the AI companion. It was `actions.Drink` until #375 phase 7
(owner ruling 2026-10-30: potions are quaffed; `eat` and `drink` are one food
command in `usercommands/eat.go`). `usercommands.Quaff` and `mobcommands.Quaff`
(`quaff.go`) are thin wrappers, and `internal/guards/drink_wrapper_guard_test.go`
(`TestQuaffWrappersDoNotReFork`) fails if either grows potion rules. A bare
`quaff` gets the prompt "Quaff what? Try **quaff &lt;item&gt;**." and
`DrinkRefuseNotFound`, before any other check. Food or a cooked drink (subtype
`edible`) is refused with `refusal.NotAPotion`, naming `eat` or `drink`. The
narration still reads "You drink the ...". The quest engine hears command
`quaff`, notified AFTER the drink line so a step's line follows the drink it
reports (#697, `quaff_quest_order_test.go`). The types keep their names
(`DrinkActor`, `DrinkResult`, `DrinkRefusal`), and so does the condition
source `drink`.
```

Edit the other two mentions: `| Drink | actions | self | DrinkResult | both | none |` becomes `| Quaff | actions | self | DrinkResult | both | none |`, and in the paragraph starting "`Drink` no longer snapshots", change the leading `` `Drink` `` to `` `Quaff` ``.

Add a short section after the Quaff section:

```
## Theft rank gate (`steal.go`, #375 phase 7)

Every theft (a creature, a player, a room container, a household's find)
refuses "not advanced enough" when the thief's skullduggery rank is under
`Balance.StealMinRank` (`theftAttempt.belowMinRank`). It was a hardcoded 2.
Players and mob thieves (`mobcommands.Steal`, the btree `try_steal`) share it.
`actions.Plant` has no rank gate.

## Fight read copy (`fight_read.go`, #690)

The verdict and pace lines name no pronoun for the target, which may be a
straw dummy: "it" is the fight. When both paces fall in one band,
`samePaceText` says it once ("Either of you would need an age to end it.").
```

- [ ] **Step 2: usercommands and mobcommands**

In `internal/usercommands/context.md`, replace:

old:
```
- **Drink** (`drink.go`, drink path unification 2026-09-28): `Drink` is a
  wrapper over `actions.Drink`, which holds every drink rule (toxicity, aging,
  crafter scaling, the special potions) for players and mobs alike. Do not add
  a rule here: `internal/guards/drink_wrapper_guard_test.go` fails if this file
  or `internal/mobcommands/drink.go` applies drink rules itself.
```
new:
```
- **Quaff, eat and drink** (#375 phase 7, owner ruling 2026-10-30): `Quaff`
  (`quaff.go`) is a wrapper over `actions.Quaff`, which holds every potion rule
  (toxicity, aging, crafter scaling, the special potions) for players and mobs
  alike. Do not add a rule there: `internal/guards/drink_wrapper_guard_test.go`
  fails if `quaff.go` here or in `internal/mobcommands` applies one. `Eat` and
  `Drink` (`eat.go`, `drink.go`) are one command, `consumeFood(verb, ...)`:
  both take subtype `edible` (food and cooked drinks), look in the backpack,
  then the bandolier, then refuse; the line says eat or drink by the item's
  type; a potion is left in the pack with `refusal.QuaffPotion`.
- **Usage lines** write raw angle brackets (`talk <name>`); ansitags decodes
  no HTML entity (`internal/guards/html_entity_guard_test.go`, #690).
```

In `internal/mobcommands/context.md`, replace:

old:
```
- **Resource consumption**: `eat`, `drink` - Survival behaviors. `Drink`
  (`drink.go`) is a wrapper over `actions.Drink`, the same body a player drinks
  through (drink path unification 2026-09-28), so a mob pays toxicity, reads
  potion freshness and crafter skill, and gets every special potion's effect.
  The guard `internal/guards/drink_wrapper_guard_test.go` fails if the wrapper grows drink
  rules again.
```
new:
```
- **Resource consumption**: `eat`, `drink`, `quaff`. `Quaff` (`quaff.go`) is a
  wrapper over `actions.Quaff`, the same body a player quaffs through (drink
  path unification 2026-09-28; quaff since #375 phase 7), so a mob pays
  toxicity, reads potion freshness and crafter skill, and gets every special
  potion's effect. The guard `internal/guards/drink_wrapper_guard_test.go`
  fails if the wrapper grows potion rules. `Drink` (`drink.go`) is `Eat` under
  its other name: food and cooked drinks, with "drinks" in the room line for
  a cooked drink.
```

- [ ] **Step 3: items, configs, refusal, planners, aicompanion**

In `internal/items/context.md`:
- Under `### Item Subtypes`, after the `Drinkable`/`Edible` lines' code block, add: `Potions are `potion`/`drinkable` and are quaffed. Food is `food`/`edible`; cooked drinks (Herbal Tea 30024, Spiced Wine 30025, Antidote Broth 30027) are `drink`/`edible` since #375 phase 7, so `eat` and `drink` take both and the bandolier does not. Look text names the verb: quaff, drink or eat (`items.go`).`
- In `### Potion Bandolier`, change "`drink` consumes the oldest match first" to "`quaff` consumes the oldest match first".

In `internal/configs/context.md`, after the `### Mitigation curve knobs (#465)` section add:

```
### Theft rank knob (#375 phase 7)
`StealMinRank` is the skullduggery rank every theft needs, players and mobs
alike (`actions.theftAttempt.belowMinRank`). It ships at 1 (every character
starts at rank 1); below 1, including an absent key, reads as 1
(`validateCombatSteal`, `defaultStealMinRank`). `steal_min_rank_test.go` pins
the shipped value and fails if `templates/help/steal.template` names another
rank.
```

In `internal/refusal/context.md`, add two rows to the `## Helpers` table after the `NotCarrying(u, "")` row:

```
| `QuaffPotion(u, "salve")` / `QuaffPotionText` | That is a potion, and potions are drunk with quaff. Try **quaff salve**. |
| `NotAPotion(u, "tea", "drink")` / `NotAPotionText` | That is not a potion. Try **drink tea**. |
```

In `internal/planners/context.md`, change `Flee combat, drink heal potion, or rest` to `Flee combat, quaff a heal potion, or rest`.

In `modules/aicompanion/context.md`, change `` `fire #id`, `taunt`, `flee`, `drink` and `aid @id`. `` to `` `fire #id`, `taunt`, `flee`, `quaff` and `aid @id`. `` and add after that bullet: `- The `drink` action verb issues `quaff` for a potion and `drink` for food or a cooked drink (#375 phase 7).`

- [ ] **Step 4: menu and users**

In `internal/menu/context.md`, replace:

old:
```
- `InterruptLine(reason, topic)` is the one line the player reads when an
  interrupt closes their menu ("You break off your talk with Kael as the fight
  begins."; for `ClosePartnerLeft`, "Your talk with Kael ends because they
  left."), or `""` when the close is silent (their own choice, a command
  whose output speaks for itself, death, a follow pull).
```
new:
```
- `InterruptLine(reason, topic)` is the one line the player reads when an
  interrupt closes their menu ("You break off your talk with Kael as the fight
  begins."; for `ClosePartnerLeft`, "Your talk with Kael ends because they
  left."; for `CloseOtherCommand`, "You break off your talk with Kael.", owner
  ruling 2026-10-30, #690), or `""` when the close is silent (their own
  choice, death, a follow pull, a typed command over a menu with no topic).
```

In `internal/users/context.md`, replace:

old:
```
  `menuGate` and `Session.StillValid` first; a line that is not an answer
  closes the menu (`CloseOtherCommand`) and returns false so the caller runs it
  as a command.
```
new:
```
  `menuGate` and `Session.StillValid` first; a line that is not an answer
  closes the menu (`CloseOtherCommand`), announced, so a talk menu says
  "You break off your talk with <name>." before the command's output (owner
  ruling 2026-10-30, #690), and returns false so the caller runs it as a
  command.
```

In `internal/guards/context.md`, Edit:

old:
```
- **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
  `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
  `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`.
```
new:
```
- **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
  `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
  `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`,
  `html_entity_guard_test.go` (no `&lt;`/`&gt;` in Go player text: ansitags
  decodes no entity, #690).
```

- [ ] **Step 5: Widen the copy guard**

In `internal/guards/copy_no_dash_test.go`, Edit:

old:
```go
	// #287, the flee refusals and the AI rate-limit notice.
	"internal/usercommands/flee.go",
	"main.go",
}
```
new:
```go
	// #287, the flee refusals and the AI rate-limit notice.
	"internal/usercommands/flee.go",
	"main.go",
	// #375 phase 7 PR 1: quaff, the food command, the consider copy, the
	// menu-close line and the hint line.
	"internal/usercommands/quaff.go",
	"internal/usercommands/drink.go",
	"internal/actions/drink.go",
	"internal/actions/fight_read.go",
	"internal/menu/lines.go",
	"internal/usercommands/hint.go",
	"internal/refusal/carrying.go",
}
```

`internal/usercommands/drink.go` holds the literals `drink` only; the guard requires at least one string literal per file, which `` `drink` `` satisfies.

- [ ] **Step 6: Audit and run**

```bash
python tools/context_md_audit.py 2>&1 | tail -10
go test ./internal/guards/ -run TestCopyFilesHaveNoDashesInStringLiterals 2>&1 | tail -4
```

Expected: the audit lists no phantom symbol in the edited files (it may list pre-existing ones elsewhere; compare with a run on master if unsure); the guard `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/context.md internal/usercommands/context.md internal/mobcommands/context.md internal/items/context.md internal/configs/context.md internal/refusal/context.md internal/menu/context.md internal/users/context.md internal/planners/context.md modules/aicompanion/context.md internal/guards/context.md internal/guards/copy_no_dash_test.go
git commit -F - <<'EOF'
docs(context): quaff, the food command, StealMinRank, menu close (#375 phase 7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 14: Patch notes

**Executor:** Sonnet

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Add the entry at the top**

In `docs/PATCH_NOTES.md`, Edit:

old:
```
# DOGMud Patch Notes

## 2026-10-28: Set a quest aside, or follow another
```
new:
```
# DOGMud Patch Notes

## 2026-10-30: Potions are quaffed, and thieves are about

- Drink a potion with `quaff`, for example `quaff healing salve`. Typing
  `drink` or `eat` at a potion now leaves it in your pack and tells you to
  quaff it.
- `eat` and `drink` now do the same thing, and both take food and cooked
  drinks. Herbal Tea, Spiced Wine and Antidote Broth are drinks now, so
  `drink tea` works, and so does `eat tea`.
- If a tick or macro of yours drinks potions, change `drink` to `quaff`
  in it.
- Herbal Tea no longer adds to your toxicity. No cooked dish does.
- Stealing needs only the skullduggery every character starts with, so
  anyone can try `steal`. The same goes for thieves in the world: watch
  your purse in Hartcharn's market, on the New Plymouth docks and roads,
  and outside Thornwall.
- Typing a command while a conversation menu is open now tells you which
  conversation it ended.
- `hint`, when you have no quest, shows `talk <name>` correctly instead of
  odd symbols. So do the usage lines for channels, guild chat, `set`
  and `shoot`.
- `consider` reads more naturally against things that are not people,
  and no longer says the same length of fight twice.
- The Straw Effigy now shows that even your best blows leave it no worse.

## 2026-10-28: Set a quest aside, or follow another
```

If the owner overrules the mob-theft effect (see "Owner check" above), delete the second sentence of the stealing bullet before merging.

- [ ] **Step 2: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs(patch-notes): quaff, StealMinRank, #690 nits (#375 phase 7 PR 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 15: Gate

**Executor:** Sonnet (Opus to triage any failure)

- [ ] **Step 1: Leftovers sweep**

```bash
grep -rn "actions\.Drink\b\|actions\.Drink(" --include=*.go internal modules *.go
grep -rn "rank < 2" internal/actions/steal.go
grep -rn "command: drink" _datafiles/world/dogmud/quests
grep -rn "Type drink" _datafiles/world/dogmud/quests _datafiles/world/dogmud/dialogue
grep -rn '&lt;\|&gt;' --include=*.go internal modules | grep -v _test | grep -v playtestenv/report.go
```

Expected: every command prints nothing. Prove the first pattern can match: `grep -rn "actions\.Quaff(" --include=*.go internal | head -2` prints lines.

- [ ] **Step 2: Format, build, tests**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/configs/ ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/refusal/ ./internal/items/ ./internal/menu/ ./internal/users/ ./internal/conversation/ ./internal/behaviortree/ ./internal/questengine/ ./internal/planners/ ./internal/hooks/ ./modules/aicompanion/ 2>&1 | tail -20
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)' | tail -12
go test ./internal/guards/... 2>&1 | tail -10
DOGMUD_BOOT_SMOKE=1 go test . 2>&1 | tail -5
```

Expected: `gofmt` prints nothing; build clean; every package `ok`; the root tests show `--- PASS`, not `SKIP` (they skip without `DOGMUD_BOOT_SMOKE=1`). `-race` cannot run on this machine (no C compiler); CI runs it.

- [ ] **Step 3: Lint**

```bash
golangci-lint run --new-from-merge-base=origin/master 2>&1 | tail -5
```

Expected: `0 issues.`

- [ ] **Step 4: Isolated boot check**

```bash
S="<your session scratchpad directory, full path>"
cd /c/tmp/dogmud-375-p7-coulee-engine
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
grep -n "9614-straw_effigy\|loop-backlog\|quest 55" "$S/boot.log" | head
```

Expected: `exit 124` (the server stayed up), `0` panics, `1` Server Ready, and no error naming the effigy tree. Quest 55's backlog line shows as a warning, not a refusal. Clean up: `cd /c/tmp/dogmud-375-p7-coulee-engine && git worktree remove --force C:/tmp/dogmud-boot-check` (if Windows holds a lock, PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check` then `git worktree prune`).

- [ ] **Step 5: Push and open the PR**

```bash
cd /c/tmp/dogmud-375-p7-coulee-engine
git push -u origin feat/375-p7-coulee-engine
gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-coulee-engine --title "#375 phase 7 Coulee PR 1: quaff, StealMinRank, #690 engine nits, menu close" --body-file - <<'EOF'
First of five PRs for the #375 phase 7 Pothole Coulee pass (spec: docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md; plan: docs/superpowers/plans/2026-10-30-p7-coulee-1-engine.md).

- `StealMinRank` knob, shipped at 1, replaces the four hardcoded rank-2 theft gates; players and mob thieves share it. Note: this wakes the five thief-archetype mobs, which were refused at rank 1 before.
- `quaff` drinks potions (player, mob, AI companion, survival planner). `eat` and `drink` are one food command for food and cooked drinks; a potion is left in the pack with a line naming quaff. Herbal Tea, Spiced Wine and Antidote Broth are type drink.
- The quest engine hears `quaff` after the "You drink the" line (part of #697). Quests 38 and 55 trigger on quaff; quest 55's backlog key follows the rename.
- #690 engine items: raw `<name>` in hint and ten other usage lines, with a guard; `consider` copy without "them" or a repeated pace; the Straw Effigy emotes that it shrugs off blows.
- Owner ruling (a): a typed command that closes a talk menu prints "You break off your talk with <name>."

Refs #690 #697 #375. Does not close them; later PRs in the pass finish both.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. The body uses "Refs", never a closing keyword. Then `gh pr checks <n> --repo pruuk/DOGMud --watch`, and confirm each expected workflow ran with `gh run list --repo pruuk/DOGMud --branch feat/375-p7-coulee-engine`.
