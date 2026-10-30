# #375 Phase 7 Coulee Plan 3b: Casting, Lore and Archery Lines, Quest 76, Coulee Dash Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the casting (44, 45, 46, 57), lore (47, 48, 49, 58) and archery (50, 51, 52, 59) lines to giver-to-giver loops of one action per step that end at hand-ins, add the bridges "Report to Toke (Casting/Lore/Archery)" (94 to 96), give quest 76 its interim disc home in Maren's Cottage with a hand-in at Brennan, fix the #689, #690 and #697 items these lines carry, add the skill-coverage info topics, and land a guard that holds every Coulee and antechamber quest, dialogue and conversation file free of dashes.

**Architecture:** Content YAML under `_datafiles/world/dogmud/` (quests, dialogue, rooms, one behaviour tree, one mob description), the two committed manifests and the loop backlog, one small engine change (`actions.SearchOptions.QuestStep`, so a `command: search` quest step is not contradicted by "You find nothing of interest."), rewritten shipped-content tests in `internal/questengine`, one behaviour data test, and one new guard in `internal/guards`.

**Tech Stack:** Go 1.x, YAML content under `_datafiles/world/dogmud/`

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md

**Branch / worktree:** branch `feat/375-p7-coulee-lines-b`, worktree `C:/tmp/dogmud-375-p7-coulee-lines-b`, off `master` once plans 1, 2 and 3a have merged.

---

## Spec deviations found (read before executing)

These come from reading source. None re-opens an owner ruling; each is flagged so the controller can reconcile sibling plans.

1. **Bridge quest file names.** The shared naming in the common brief gives `quests/90-report_to_toke_fighting.yaml`, but the loader demands `<id>-<ConvertForFilename(name)>.yaml` (`internal/quests/quests.go:326-329`, `internal/fileloader/fileloader.go:103` panics on a mismatch). `ConvertForFilename` (`internal/util/util.go:920-937`) lowercases, keeps a to z and 0 to 9, drops apostrophes and turns every other byte into `_`. So "Report to Toke (Casting)" must be filed as `94-report_to_toke__casting_.yaml` (two underscores before the line, one after). This plan uses `94-report_to_toke__casting_.yaml`, `95-report_to_toke__lore_.yaml`, `96-report_to_toke__archery_.yaml`. The coordinator confirmed this rule on 2026-10-30 (plan 3a's bridges are `90-report_to_toke__fighting_.yaml` to `93-report_to_toke__tracking_.yaml`).
2. **Search says "nothing" before the quest speaks.** `actions.Search` prints "You find nothing of interest." when the search found nothing (`internal/actions/search.go:316`), and `usercommands.Search` raises the quest `command` event only afterwards (`internal/usercommands/skill.search.go:52`). The only exception today is a `verb: search` room_interact spot. Moving quests 49, 58 and 76 onto `event: command, command: search` (spec "Typed nouns go") would print "You find nothing of interest." and then "Your fingers find the lifted edge of a flagstone...". Task 2 adds `SearchOptions.QuestStep` (set when a `command: search` trigger would fire for the player in this room) and skips the line then. Small engine change the spec did not list.
3. **Quest 76's "real actions".** The spec says the journal and attune steps stay at 4023 on real actions. `read`/`look journal` raise only `room_interact` (`internal/usercommands/read.go:54-57`); there is no `read` command event, and `use` notifies nothing. This plan makes the cottage `search` the journal step's way in (the search turns up the disc and the journal; its text is the reading), and `cast glow` at 4023 the attune step's way in (`command: cast, noun: chrysalis-glow`; the attune text already says the disc's lines "take the light and keep it"). Step ids stay `start`, `journal`, `attune`, `end`. If the owner prefers another verb, only Task 10 changes.
4. **Quest 49 still finds "a disc" in its story.** The spec removes the Reliquary's `disc` hidden noun and its floor hint. Quest 49's finale text (`quests/49-the_old_shrine.yaml:103-115`), Sere's lines and quest 58 tell of a disc under the floor that stays there. This plan keeps that story as text only (no item, no hidden noun, no `discover` action, no "look at the disc" invitation), and drops the two room lines that promise a find. The heaved-flag description stays, because quest 49 still sends the player to search that floor.
5. **Plain `search` now counts as fully as `search floor`.** The command event fires on every successful search (`skill.search.go:46-52`), so the phase 3a lesson "typing just search covers the whole room, slower and less sure" is no longer true for 49 and 58. Hints drop that sentence and keep `type search floor`.
6. **Garrow leaves by despawn.** `taunt` is a combat special move, so it always starts a real fight; Garrow's tree then `flee`s, which is a roll (`internal/behaviortree/actions_combat.go:116`, `mob.Command("flee")`) that can fail. Task 6 replaces `flee` with the mob command `despawn` (`internal/mobcommands/mobcommands.go:41`, `despawn.go`); the player's aggro on him is released next round because the target no longer exists (`internal/hooks/combat_retarget.go:35-40`), and room 5335's spawn list brings him back after 5 rounds with a message.
7. **Toke's three offer texts (plan 2's file).** Quests 44, 47 and 50 now begin "go and see Grieve / Wenna / Iden". Toke's `send_folding`, `send_folk` and `send_range` (`dialogue/pothole_coulee/9106.yaml:250-301` on master) still tell the player to cast, type help, or equip first. The brief says this plan only adds Toke's three hand-in topics. Task 12 checks what plan 2 left; if the texts are stale it holds for a controller decision and carries the exact replacement text.
8. **Manifestation topics are split, not duplicated:** Grieve teaches what manifestation is, conjuring and charming; Orrin teaches raising and `assess` (the grove fights leave corpses).
9. **Repeatables now end at hand-ins**, so 57, 58 and 59 add a walk back to the giver, and their reward lines move to the hand-in.
10. **Gate command (coordinator correction 2026-10-30).** `quest_loop_smoke_test.go:28-30` skips every loop sweep unless `DOGMUD_BOOT_SMOKE` is set (`boot_smoke_test.go:36`). Every loop or boot smoke run in this plan is `DOGMUD_BOOT_SMOKE=1 go test ... .` in Git Bash, and its expected output is `ok` with the tests run (`--- PASS` under `-v`), never `--- SKIP`.
11. **Content playtest.** `dogmud-authoring-content` asks every content plan to end with an adversarial playtest; the spec's Delivery table puts that playtest in PR 5 (the close). This plan defers to the spec and does not run a separate harness playtest.

## Interface with sibling plans

- **Plan 1 (engine):** nothing here depends on it. Task 2's search change touches `internal/actions/search.go` and `internal/usercommands/skill.search.go`; if plan 1 edited either, keep its edits.
- **Plan 2 (spine):** assumed done before this plan: Toke (9106) offers 44, 47 and 50 (`send_folding`, `send_folk`, `send_range`, gated on `31-end`); Grieve's `cast_offer`, Wenna's `words_offer` and Iden's `shoot_offer` are removed (and retired in `topic-keys.txt`) with breadcrumbs added; the pre-quest opener variants of Grieve, Wenna and Iden (`questRequired: [30-end]` with `questExcluded: [<Q1>-start, <Q1>-end]`) are plan 2's; backlog lines `chain 30 mob 9144 topic cast_offer`, `chain 30 mob 9153 topic words_offer`, `chain 30 mob 9161 topic shoot_offer` are deleted by plan 2. Task 1 verifies each.
- **Plan 3a:** owns quests 32 to 43 and 53 to 56, bridges 90 to 93 and their dashes. This plan's guard (Task 13) also reads their files; if one still carries a dash, stop and report it.
- **Plan 4:** owns the Shadows line (97 to 101), quest 89, and Toke and Esk flavour trees. Do not touch them. This plan adds only three hand-in topics to Toke.

## Backlog map (quests 28 to 59, 76, 88, 89)

Master `8432192f1` holds 95 such lines in `_datafiles/world/dogmud/quests/loop-backlog.txt`. Owner of each:

| Lines (master line numbers) | Owner |
|---|---|
| 7 `chain 30 mob 9100 topic road_esk` | plan 2 (deleted there: Hadwen takes quest 30's hand-in, so the offer passes `ruleChain`) |
| 8 to 24 (`chain 30` for road_toke, footing_offer, the seven Giver 1 offers, and `chain 32/34/35/37/38/40/41/43`) | plan 2 (lines 8 to 16) and plan 3a (17 to 24) |
| 25 to 30 `chain 44 mob 9145 topic grove_offer`, `chain 46 mob 9145 topic grove_loop_offer`, `chain 47 mob 9157 topic stones_offer`, `chain 48 mob 9160 topic shrine_offer`, `chain 50 mob 9162 topic canyon_offer`, `chain 52 mob 9162 topic range_loop_offer` | **this plan** (Tasks 4, 6, 7, 9) |
| 46 to 54 (`end 28/29/30`) | plan 2 |
| 55 to 64, 69 to 76 (`end 34/35/37/40/43/53/54/55/56`) | plan 3a |
| 65 to 68 (`end 46`, `end 52`), 77 to 82 (`end 57/58/59`) | **this plan** (Tasks 4, 7, 9) |
| 85, 86 (`end 76`) | **this plan** (Task 10) |
| 109 `end 88 ...` | plan 2 |
| 110 `end 89 quest 89 trigger room_enter room=468 grants=89-end` | plan 4 |
| 170, 171 `hint-length 76 step journal`, `hint-length 76 step start` | **this plan** (Task 10) |
| 191 to 199 (`one-action 28/32/34/35/38`) | plan 2 (28) and plan 3a (32 to 38) |
| 202 `one-action 50 ... grants=50-shoot:prev` | **this plan** (Task 8) |
| 233 to 235 `one-action 76 ...` (`:cond`, `:double`, `:prev`) | **this plan** (Task 10) |
| 244 to 248 (`start 28/29/30`) | plan 2 |
| 259 to 263 (`walk-dead-end 32/35/38`) | plan 3a |
| 266, 267 `walk-dead-end 50 ... grants=50-cover_shot`, `... grants=50-shoot` | **this plan** (Task 8) |
| 286 `walk-destination 28 step fled` | plan 2 |
| 287 `walk-destination 42 step hunt` | plan 3a |
| 295 `walk-destination 76 step attune` | **this plan** (Task 10) |

This plan deletes 27 lines. After plans 2, 3a and 3b, the only lines naming quests 28 to 59, 76, 88 or 89 must be line 110 (`end 89 ...`), plan 4's (plan 2 deletes line 7, `chain 30 mob 9100 topic road_esk`). Task 15 checks this.

## Facts verified against source (master `8432192f1`, 2026-10-30)

| Fact | Source |
|---|---|
| Backlog lines for this plan are master lines 25 to 30, 65 to 68, 77 to 82, 85, 86, 170, 171, 202, 233 to 235, 266, 267, 295 (27 lines) | `quests/loop-backlog.txt` |
| A listed finding is a warning; an unlisted one refuses boot and CI; a stale line fails CI | `loop-backlog.txt:1-6`; `internal/questloop/context.md` "Finding.Key and the backlog" |
| `ruleChain`: an offer gated on quest N's final token must sit on an NPC holding a hand-in for N | `internal/questloop/rules_more.go:14-52` |
| `one-action`: each way into step i must require step i-1's token; no item, flag or gold condition (`:cond`); one trigger grants at most one step (`:double`) | `internal/questloop/rule_oneaction.go:15-58` |
| Walk skip probe fires every later Way from the current step and reports a skip under the probed Way's name | `internal/questengine/walkthrough.go:338-358` |
| Quest 50's fire at 5351 gates on `50-start`, not `50-equip` (the `:prev` and both `walk-dead-end` lines) | `quests/50-first_shot.yaml:147-159` |
| Quests 46, 52, 57, 59 end on `mob_death`; 58 ends on a `verb: search` room_interact; none has a hand-in | quest files 46:64-74, 52:71-81, 57:48-58, 58:52-65, 59:72-81 |
| Quest 76 picks the disc up by a `disc` noun in 5343 and ends on a `room_interact` on `hearth-marks` that grants `76-attune` and `76-end` with `has_item` | `quests/76-the_disc.yaml:57-77,133-163` |
| Step ids are pinned by `quests/step-ids.txt`; it has no `# retired` section on master (only header comments, lines 1 to 8) | `quests/step-ids.txt`; `internal/guards/step_manifest_test.go` |
| Topic keys are pinned by `dialogue/topic-keys.txt`; a removed key goes under `# retired` (a key may stay listed and also be retired) | `topic-keys.txt:1-8`; `internal/manifest` `Parse`, `Diff` |
| Topic gates: `questRequired` all-of, `questExcluded` none-of; an offer hides once its quest is open | `internal/dialogue/engine.go:27`; spec facts |
| Offer and hand-in labels must equal the quest title; label at most 50 runes, playerLine at most 70; no semicolons in label, playerLine, text, breadcrumb | `internal/dialogue/world.go` (`validateTopicFile`); `internal/menu/lint.go:10-11` |
| `grantsQuest` must appear in `questExcluded` together with the quest's end token | `internal/dialogue/world.go` `gateRefProblems` |
| `topic_heard` trigger fields: `mob`, `topic`; the walk plays the topic then raises the event | `internal/questengine/walkthrough.go:866-875`; `quests/60-the_long_road.yaml:56-62` |
| `search` raises the quest `command` event (`command: search`, template room) for every successful search, plain or targeted | `internal/usercommands/skill.search.go:46-52`; `internal/questengine/command_event.go` `CommandDetails` |
| `actions.Search` prints "You find nothing of interest." unless something was found or a quest search spot was named | `internal/actions/search.go:316` |
| `SearchOptions` has `Feature`, `QuestFeatures` only | `internal/actions/search.go:93-105` |
| `Engine.WouldFire(event, details, player)` dry-runs field matches and conditions | `internal/questengine/dryrun.go:14-21` |
| `cast` raises `command` with `Noun` = canonical spell id; glow's id is `chrysalis-glow`, alias `glow`; it is a starter spell | `internal/usercommands/skill.cast.go:386`; `spells/chrysalis-glow.yaml:2-4`; `internal/characters/character.go:381-385` |
| `equip` raises `command equip` with the item id; equipping an item already in hand finds nothing in the backpack and refuses | `internal/usercommands/equip.go:96-102,212` |
| `read` and look-family verbs raise only `room_interact` | `internal/usercommands/read.go:54-57` |
| Repeatable completion clears the quest's token and stamps the cooldown | `internal/hooks/Quest_HandleQuestUpdate.go:345-347` |
| Quest file name is `<id>-<ConvertForFilename(name)>.yaml` | `internal/quests/quests.go:326-329`; `internal/util/util.go:920-937` |
| Practice Mote 9146 is `combat_passive` (fights back) and room 5305 respawns it with a message | `mobs/pothole_coulee/9146-practice_mote.yaml:3`; `behaviors/archetypes/combat_passive.yaml:3-5`; `rooms/pothole_coulee/5305.yaml:34-37` |
| Garrow's yield branch runs on `mob_combat_round`: say (with " -- "), emote, `flee` | `behaviors/pothole_coulee/9158-sullen_garrow.yaml` |
| BT `command` action issues a mob command from `cmd:` | `internal/behaviortree/actions_mob.go:169-180`; `behaviors/archetypes/boss_soren.yaml:30-32` |
| Room spawn entries take `message`, `narraterespawn`, `respawnrate` ("5 rounds") | `rooms/pothole_coulee/5351.yaml:36-40` |
| Grieve 9144 at 5303, Orrin 9145 at 5309, Wenna 9153 at 5324, Hale 9157 at 5333, Garrow 9158 and Ferd 9159 at 5335, Sere 9160 at 5341, Iden 9161 at 5350, Bryn 9162 at 5353, Toke 9106 at 5203, Brennan 9516 at greenford 6305 | room `spawninfo` |
| Route Hub 5203 to Grieve: west, north, north, north, up, up | rooms 5203, 5216, 5201, 5218, 5224, 5302 exits |
| Route Grieve 5303 to Orrin 5309: north, west, north, north, north | rooms 5303, 5305, 5306, 5307, 5308 |
| Route Orrin to Hub: south x3, east, south, down, down, south x3, east | rooms 5309 to 5216 |
| Route Thinnest Place 5320 to Orrin: west, south, east, south, down, east, south, south, east | rooms 5320, 5319, 5317, 5316, 5315, 5314, 5313, 5312, 5311 |
| Route Hub to Wenna 5324: east, then north six times | rooms 5208, 5202, 5217, 5225, 5322, 5323 |
| Route Wenna to Hale 5333: north, west, north, north, north (Outer Ring 5331), west | rooms 5327, 5328, 5329, 5330, 5331 |
| Route Hale to Sere 5341: north, north (Tall Stone 5337), west (5338), north, north | rooms 5335, 5337, 5338, 5339 |
| Route Sere to Reliquary 5343: west, north | rooms 5341, 5342 |
| Route Sere to Hub: S, S, E, S, S, E, S, S, S, E, then S six times, W, S | rooms 5339 to 5200 |
| Route Hub to Iden 5350: north, east, east, north, up, north, north, up | rooms 5200, 5202, 5209, 5226, 5347, 5348, 5349 |
| Route Iden to Bryn 5353: east, east, north | rooms 5351, 5352 |
| Route Overlook 5364 to Bryn: south x3, down, south x4 | rooms 5362, 5361, 5360, 5358, 5357, 5356, 5354 |
| Route Bryn to Hub: south, west, west, down, south, south, down, south, west, west, south | rooms 5352 to 5200 |
| Maren's Cottage 4023 is south of East Edge 4019; nouns `hearth`, `stone`, `journal`, `hearth marks` | `rooms/ashwick/4023.yaml` |
| Brennan offers 76 twice (`disc_quest` carriesItem 40167, `disc_tale` itemExcluded 40167); 76 opener variants name the Reliquary and `look` nouns | `dialogue/greenford/9516.yaml:26-67,131-183` |
| The Threshold Keeper's `crash` hint names the Reliquary | `dialogue/eastern_highlands/9553.yaml:56-61` |
| Reliquary 5343: hidden noun `disc` on `floor`; last description line and floor noun invite a find | `rooms/pothole_coulee/5343.yaml:15-27,32-50` |
| Only quest 45 carries an `area` destination; `TestShippedContent_EveryDestinationKindHasALiveStep` needs one | `internal/quests/live_destination_kinds_test.go:52-70`; grep `area:` in quests |
| Shipped-content tests pin these quests: `newbie_folding_steps_test.go` (45, 46, 57, Orrin), `newbie_lore_steps_test.go` (47, 48, 49, 58), `newbie_ranged_steps_test.go` (51, 52, 59, Bryn), `newbie_one_command_steps_test.go:415-486` (50) and `:367-371` (`lenientGrants` "50-shoot"), `newbie_topic_steps_test.go:291-301` (Iden) | those files |
| `TestNewbieQuestHintsNameOneCommand` holds every non-end step of every Coulee-offered quest to exactly one "type " | `newbie_one_command_steps_test.go:280-297` |
| Prose dash guard exempts whole-line comments only, by line prefix; `emOrEnDash` is a package var | `internal/guards/prose_dash_guard_test.go:40,65` |
| Guards run with the repo root as working directory; `repoRoot(t)` | `internal/guards/testmain_test.go` |
| Dash lines (non-comment) in this plan's quests on master: 44 (3), 45 (3), 46 (2), 47 (3), 48 (6), 50 (2), 51 (3, two of them trailing comments), 52 (4), 57 (1); conversations 9103_9105:21 and 9106_9107:16 | grep |
| Loop sweeps skip without `DOGMUD_BOOT_SMOKE` | `quest_loop_smoke_test.go:26-30`; `boot_smoke_test.go:36` |
| Help: spells are not taught by trainers; manifestation, conjure, charm, raise, assess (no `raise` command) | `templates/help/spells.template`, `manifestation.template`, `conjure-fire.template`, `charm.template`, `raise-skeleton.template`, `assess.template` |
| Rally steadies dodge, parry and block; warcry raises physical damage; both cost conviction, scale with rhetoric and charisma, can be used before a fight | `templates/help/rally.template`, `warcry.template` |
| `set linewidth <n>` exists | `internal/usercommands/set.go:60,531` |
| Bryn is "he" (test N13); Iden, Sere and Wenna are "she"; Grieve, Orrin, Hale, Brennan are "he" | `newbie_ranged_steps_test.go:165-167`; dialogue and quest text |

## File map

Create:
- `_datafiles/world/dogmud/quests/94-report_to_toke__casting_.yaml`: bridge, Orrin to Toke.
- `_datafiles/world/dogmud/quests/95-report_to_toke__lore_.yaml`: bridge, Sere to Toke.
- `_datafiles/world/dogmud/quests/96-report_to_toke__archery_.yaml`: bridge, Bryn to Toke.
- `internal/guards/coulee_dash_guard_test.go`: the Coulee dash guard and its can-fail sibling.
- `internal/questengine/coulee_bridges_test.go`: bridges 94 to 96, offer and hand-in.
- `internal/questengine/quest76_disc_steps_test.go`: quest 76's interim home.
- `internal/behaviortree/garrow_yield_test.go`: Garrow's yield branch and respawn.

Modify:
- `internal/actions/search.go`, `internal/actions/search_split_test.go`: `SearchOptions.QuestStep`.
- `internal/usercommands/skill.search.go`, `internal/usercommands/search_quest_feature_test.go`: set `QuestStep`.
- Quests 44, 45, 46, 47, 48, 49, 50, 51, 52, 57, 58, 59, 76: full replacements.
- Dialogue `pothole_coulee/9145.yaml`, `9157.yaml`, `9160.yaml`, `9162.yaml`: full replacements. `9144.yaml`, `9153.yaml`, `9161.yaml`: targeted edits. `9106.yaml`: three hand-in topics appended (plus the conditional Task 12). `greenford/9516.yaml`: 76 variants, offers, hand-in. `eastern_highlands/9553.yaml`: one hint.
- `behaviors/pothole_coulee/9158-sullen_garrow.yaml`: full replacement.
- Rooms `pothole_coulee/5343.yaml`, `5335.yaml`, `5213.yaml`, `5214.yaml`.
- `mobs/pothole_coulee/9146-practice_mote.yaml`: description.
- `conversations/pairs/9103_9105.yaml`, `9106_9107.yaml`: one line each, only if still dashed.
- `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`.
- Tests: `internal/questengine/newbie_folding_steps_test.go`, `newbie_lore_steps_test.go`, `newbie_ranged_steps_test.go` (full replacements), `newbie_one_command_steps_test.go`, `newbie_topic_steps_test.go` (targeted).
- Docs: `internal/questengine/context.md`, `internal/usercommands/context.md`, `internal/actions/context.md`, `internal/guards/context.md`, `.claude/skills/dogmud-authoring-quests/SKILL.md` (one line), `docs/PATCH_NOTES.md`, `docs/README.md` (only if no row for this plan).

## House rules for every executor

- Edit files only with the Edit and Write tools. Never write a repo file from the shell (`>`, `>>`, `sed -i`, Python writes); `guard.py` blocks it. Scratch files go in the session scratchpad by its full path.
- `git add` named paths only. Never `git add -A`, `.` or `--all`, never `git commit -a`, never `--no-verify`.
- No em dash, en dash or " -- " in any text you write, including YAML comments you add.
- Plain multi-line YAML scalars in dialogue must not contain ": " or " #"; where the text needs a colon, single-quote the whole scalar (double any `'` inside). Block scalars (`>-`, `|`) are safe.
- Every `gh` command carries `--repo pruuk/DOGMud`.
- `-race` cannot run on this machine; CI runs it.

---

### Task 1: Worktree and preconditions

**Executor:** Opus (judgement: reads what plans 2 and 3a left).

**Files:** none modified.

- [ ] **Step 1: Create the worktree off master**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/375-p7-coulee-lines-b C:/tmp/dogmud-375-p7-coulee-lines-b origin/master
cd C:/tmp/dogmud-375-p7-coulee-lines-b
git log --oneline -15
```
Expected: the log shows the merge commits of plans 1, 2 and 3a (PR titles naming #375 phase 7 engine, spine, and lines A). If any is missing, STOP and report: this plan runs only after all three.

- [ ] **Step 2: Check plan 2's hand-off**

Run from the worktree:
```bash
grep -n "key: cast_offer\|key: words_offer\|key: shoot_offer" _datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml
grep -n "key: send_folding\|key: send_folk\|key: send_range" _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml
grep -n "chain 30 mob 9144\|chain 30 mob 9153\|chain 30 mob 9161" _datafiles/world/dogmud/quests/loop-backlog.txt
grep -n "^# retired\|^# renamed" _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt
```
Expected: the first command prints nothing (the three Q1 offers are gone); the second prints three lines (Toke's offers exist); the third prints nothing; the fourth shows where any `# retired` section now starts (note the line numbers; later tasks append under it). If the first or third prints anything, STOP and report to the controller: plan 2 did not finish the Q1 hand-off this plan builds on.

- [ ] **Step 3: Read Toke's three offers and record their state**

Read `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` around `send_folding`, `send_folk`, `send_range`. Note whether each `hints` still tells the player to `type cast spike mote`, `Type help.` or `type equip sling` before reaching the giver. Task 12 needs this answer.

- [ ] **Step 4: Baseline the shipped-content tests**

```bash
go build ./...
go test ./internal/questengine/ ./internal/guards/ 2>&1 | tail -5
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)|quest loop:'
```
Expected: all `ok`, every `--- ` line `--- PASS` and none `--- SKIP` (the root sweeps skip without `DOGMUD_BOOT_SMOKE=1`, `quest_loop_smoke_test.go:28`). Record the `quest loop: N known backlog findings` line.

**Rule for every later task:** each `DOGMUD_BOOT_SMOKE=1 go test ... .` step in this plan must show `ok` with the tests run, not skipped. If unsure, rerun it with `-v` and check for `--- PASS` lines. Plan 2 makes quests 28 and 89 abandonable and drops wholly retired quests (29) at load; neither changes anything here, but do not "fix" those findings if `TestSmoke_AutoStartedQuestsNotAbandonable` mentions them. If anything is red before this plan's first edit, STOP and report.

---

### Task 2: Search does not say "nothing" before a quest step speaks

**Executor:** Sonnet.

**Files:**
- Modify: `internal/actions/search.go` (struct `SearchOptions`, the nothing-found line)
- Modify: `internal/actions/search_split_test.go` (new test at the end)
- Modify: `internal/usercommands/skill.search.go`
- Modify: `internal/usercommands/search_quest_feature_test.go` (two new tests at the end)

- [ ] **Step 1: Write the failing actions test**

Append to `internal/actions/search_split_test.go`:

```go
// #375 phase 7: a `command: search` quest step answers the search itself (the
// caller raises the command event right after), so a search that finds
// nothing does not first say so when such a step will fire. Without one, the
// line stays.
func TestSearch_QuestStepLeavesTheAnswerToTheQuest(t *testing.T) {
	pinConfigForTest(t)
	cases := []struct {
		name string
		room int
		uid  int
		step bool
	}{
		{"StepperYes", 9751, 7751, true},
		{"StepperNo", 9752, 7752, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actor := newSearchFakeActor(c.name, newSearchTestRoom(c.room), true, c.uid)
			stubBaubleSearch(t, false, actor)
			pinSearchRoll(t, false)
			Search(actor, SearchOptions{QuestStep: c.step})
			if said := searchSaid(actor, "You find nothing of interest."); said == c.step {
				t.Fatalf("QuestStep=%v: nothing-found line said=%v, messages %q", c.step, said, actor.sent)
			}
		})
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/actions/ -run TestSearch_QuestStepLeavesTheAnswerToTheQuest
```
Expected: build failure `unknown field QuestStep in struct literal of type SearchOptions`.

- [ ] **Step 3: Implement the option**

In `internal/actions/search.go`, replace

```go
	// QuestFeatures are the room's quest search spots a plain search may
	// uncover: the features of `verb: search` room_interact triggers that
	// would fire for this player now (the caller asks the quest engine). A
	// plain search rolls each at SearchRoomHiddenDifficulty; a feature
	// search ignores them (its trigger fires with no roll).
	QuestFeatures []string
}
```
with
```go
	// QuestFeatures are the room's quest search spots a plain search may
	// uncover: the features of `verb: search` room_interact triggers that
	// would fire for this player now (the caller asks the quest engine). A
	// plain search rolls each at SearchRoomHiddenDifficulty; a feature
	// search ignores them (its trigger fires with no roll).
	QuestFeatures []string

	// QuestStep is set when a `command: search` quest trigger in this room
	// will fire for this player now (the caller asks the quest engine). That
	// trigger's own text says what the search turned up, so a search that
	// finds nothing else does not first print "You find nothing of
	// interest." (#375 phase 7: quests 49 and 58 in the Reliquary, 76 in
	// Maren's Cottage).
	QuestStep bool
}
```
and replace
```go
	if actor.IsPlayer() && !result.FoundAnything() && !(hasFeature && isQuestSpot(feature.Name, opts.QuestFeatures)) {
```
with
```go
	if actor.IsPlayer() && !result.FoundAnything() && !opts.QuestStep && !(hasFeature && isQuestSpot(feature.Name, opts.QuestFeatures)) {
```

- [ ] **Step 4: Run the actions test and see it pass**

```bash
go test ./internal/actions/ -run 'TestSearch_' 
```
Expected: `ok`.

- [ ] **Step 5: Write the failing wiring tests**

Append to `internal/usercommands/search_quest_feature_test.go`:

```go
// #375 phase 7: quests 49, 58 and 76 listen for the search command itself
// (`event: command, command: search`). When such a step will fire, its text
// says what the search turned up, so the search does not first print "You
// find nothing of interest."
func TestSearch_ACommandSearchStepLeavesTheAnswerToTheQuest(t *testing.T) {
	alice, room := interactScene(t)
	searchEyes(t, alice, 1)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999490, "command", room.RoomId, questengine.TriggerDef{Command: "search"})
	events.DrainQueuedMessagesForTest(alice.UserId)

	_, err := TryCommand("search", "", alice.UserId, 0)
	require.NoError(t, err)
	require.True(t, fired(), "the search step fires")
	require.NotContains(t, strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n"), "You find nothing of interest.")
}

// When no search step would fire for this player, the line stays.
func TestSearch_NothingFoundStaysWhenNoSearchStepWouldFire(t *testing.T) {
	alice, room := interactScene(t)
	searchEyes(t, alice, 1)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999491, "command", room.RoomId, questengine.TriggerDef{
			Command:    "search",
			Conditions: questengine.Conditions{Has: []string{"999491-never"}},
		})
	events.DrainQueuedMessagesForTest(alice.UserId)

	_, err := TryCommand("search", "", alice.UserId, 0)
	require.NoError(t, err)
	require.False(t, fired())
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n"), "You find nothing of interest.")
}
```

- [ ] **Step 6: Run them and see the first fail**

```bash
go test ./internal/usercommands/ -run 'TestSearch_ACommandSearchStep|TestSearch_NothingFoundStays'
```
Expected: `TestSearch_ACommandSearchStepLeavesTheAnswerToTheQuest` FAILS (`should not contain "You find nothing of interest."`); the second passes.

- [ ] **Step 7: Wire the option in the command**

In `internal/usercommands/skill.search.go`, replace
```go
	result := actions.Search(actor, actions.SearchOptions{
		Feature:       rest,
		QuestFeatures: questSearchSpots(user, room),
	})
```
with
```go
	result := actions.Search(actor, actions.SearchOptions{
		Feature:       rest,
		QuestFeatures: questSearchSpots(user, room),
		QuestStep:     questSearchStep(user, room),
	})
```
replace
```go
	// Quest engine: command notification, so a quest can gate a step on the
	// `search` command anywhere in a room. Mirrors forage/drink/throw/cast.
	// No shipped quest uses it since quests 49 and 58 moved to a `verb:
	// search` room_interact on the Reliquary floor (#375 phase 3a); a quest
	// step tied to one feature belongs there.
	questengine.NotifyCommand(user, room.RoomId, "search")
```
with
```go
	// Quest engine: command notification, so a quest can gate a step on the
	// `search` command in a room (#375 phase 7: quests 49 and 58 in the
	// Reliquary, 76 in Maren's Cottage). Mirrors forage/drink/throw/cast.
	questengine.NotifyCommand(user, room.RoomId, "search")
```
and append at the end of the file:
```go

// questSearchStep reports whether a `command: search` quest trigger in this
// room would fire for this player now. Its own text says what the search
// turned up, so actions.Search must not first say it found nothing.
func questSearchStep(user *users.UserRecord, room *rooms.Room) bool {
	d := questengine.CommandDetails(user.UserId, room.RoomId, "search", questengine.NoTarget())
	return questengine.GetEngine().WouldFire("command", d, questengine.NewGameBridge(user, room.RoomId))
}
```

- [ ] **Step 8: Run both packages**

```bash
gofmt -l internal/actions internal/usercommands
go test ./internal/actions/ ./internal/usercommands/ 2>&1 | tail -3
```
Expected: `gofmt -l` prints nothing; both `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/actions/search.go internal/actions/search_split_test.go internal/usercommands/skill.search.go internal/usercommands/search_quest_feature_test.go
git commit -F - <<'EOF'
feat(search): a command-search quest step answers the search itself

actions.SearchOptions gains QuestStep. usercommands.Search sets it when a
`command: search` quest trigger in the room would fire for the player, and
the search then skips "You find nothing of interest." so the quest's own
line is not contradicted (#375 phase 7: quests 49, 58, 76).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Casting line quest 1, First Casting (44) and Grieve

**Executor:** Sonnet.

**Files:**
- Modify: `_datafiles/world/dogmud/quests/44-first_casting.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml` (targeted)
- Modify: `_datafiles/world/dogmud/mobs/pothole_coulee/9146-practice_mote.yaml` (description)
- Modify: `_datafiles/world/dogmud/quests/step-ids.txt`, `_datafiles/world/dogmud/dialogue/topic-keys.txt`
- Modify: `internal/questengine/newbie_topic_steps_test.go` (add `checkMeetStep`)
- Modify: `internal/questengine/newbie_folding_steps_test.go` (full replacement; this task writes the file, Task 4 finishes it)

- [ ] **Step 1: Add the meeting-step helper**

In `internal/questengine/newbie_topic_steps_test.go`, insert after the closing brace of `checkTopicStepsWith` (before `// checkHandout`):

```go
// checkMeetStep checks the first step of a giver-to-giver quest (#375 phase
// 7): go and see the giver. The step's hint names the walk and teaches `talk
// <npc>`; at that step the giver's opener names no command and points at the
// menu; the giver's meeting topic is on the menu, quest-aware, and names the
// next step's one command (want, which the next step's hint names too); and
// hearing that topic (the topic_heard trigger) moves the player to the next
// step. Another topic, or the same key on another NPC, does not.
func checkMeetStep(t *testing.T, npc, dfile, qfile string, id int, key, want string, others ...string) {
	t.Helper()
	q := loadShippedQuest(t, qfile, id)
	require.GreaterOrEqual(t, len(q.Steps), 3, "quest %d: a meeting step needs a step after it", id)
	next := q.Steps[1].Id
	require.Contains(t, questHintFor(t, q, "start"), "type talk "+npc,
		"quest %d: the meeting step's hint must teach talk %s", id, npc)
	require.Contains(t, questHintFor(t, q, next), want, "quest %d step %q hint changed; update this test", id, next)

	df := loadCouleeTopics(t, dfile)
	n := newNpcTalker(df, playerOn(q, "start", others...))
	_, opener := n.opener()
	require.Zero(t, typedCount(opener), "quest %d start: %s's opener names no command before the lesson: %q", id, npc, opener)
	require.Contains(t, squash(opener), "choose a line below", "quest %d start: %s's opener points at the menu", id, npc)
	tp := n.eligible(key)
	require.NotNil(t, tp, "quest %d start: %s topic %q is not on the menu", id, npc, key)
	require.Equal(t, dialogue.KindInfo, tp.Kind, "quest %d: %s is an info topic", id, key)
	require.True(t, gatesOnQuest(tp, id), "quest %d: %s must be quest-aware", id, key)
	require.Contains(t, squash(tp.Hints), want, "quest %d: %s names the next step's command", id, key)

	w := newQuestWalker(q).at("start")
	for _, tok := range others {
		w.p.fullMockPlayer.quests[tok] = true
	}
	w.notify("topic_heard", EventDetails{MobId: df.MobId, Topic: "not_" + key})
	require.Equal(t, "start", w.p.current, "quest %d: another topic must not count", id)
	w.notify("topic_heard", EventDetails{MobId: df.MobId + 1, Topic: key})
	require.Equal(t, "start", w.p.current, "quest %d: the same key on another NPC must not count", id)
	w.notify("topic_heard", EventDetails{MobId: df.MobId, Topic: key})
	require.Equal(t, next, w.p.current, "quest %d: hearing %s's %s moves the player on", id, npc, key)
}
```

- [ ] **Step 2: Write the folding test file (casting tests for 44 now, 45/46/57 kept as shipped for Task 4)**

Replace the whole of `internal/questengine/newbie_folding_steps_test.go` with:

```go
package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The casting line (Pothole Coulee quests 44, 45, 46 and the repeatable 57)
// under #375 phase 7: giver to giver. Toke offers First Casting and Grieve
// takes it; Grieve offers The Unquiet Grove and Orrin takes it; Orrin offers
// The Unfolded, takes its report, and keeps Tending the Grove and the bridge
// back to Toke. One command per step, true directions, and a giver who
// answers the words his own hints invite. Identifiers carry a `folding`
// prefix: sibling newbie test files share this package.

const (
	foldingHubSquare     = 5203
	foldingObservatory   = 5303
	foldingStarChamber   = 5305
	foldingOrrinRoom     = 5309
	foldingGroveEdge     = 5308
	foldingVeilHollow    = 5311
	foldingThinningWood  = 5312
	foldingGroveHeart    = 5313
	foldingThinnestPlace = 5320
	foldingGrieve        = 9144
	foldingOrrin         = 9145
	foldingPracticeMote  = 9146
	foldingDriftWisp     = 9147
	foldingGroveRevenant = 9148
	foldingTheUnfolded   = 9152
)

// foldingRoom is a coulee room's exits and spawns, read from the shipped YAML.
type foldingRoom struct {
	Title string `yaml:"title"`
	Exits map[string]struct {
		RoomId int `yaml:"roomid"`
	} `yaml:"exits"`
	SpawnInfo []struct {
		MobId int `yaml:"mobid"`
	} `yaml:"spawninfo"`
}

func foldingLoadRoom(t *testing.T, id int) foldingRoom {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "rooms", "pothole_coulee",
		strconv.Itoa(id)+".yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "room %d must exist", id)
	var r foldingRoom
	require.NoError(t, yaml.Unmarshal(data, &r))
	return r
}

// foldingWalk follows exits from a room and returns where the player ends up.
func foldingWalk(t *testing.T, from int, dirs ...string) int {
	t.Helper()
	at := from
	for _, d := range dirs {
		r := foldingLoadRoom(t, at)
		ex, ok := r.Exits[d]
		require.True(t, ok, "room %d (%s) has no %s exit", at, r.Title, d)
		at = ex.RoomId
	}
	return at
}

func foldingSpawns(t *testing.T, room int, mob int) bool {
	t.Helper()
	for _, s := range foldingLoadRoom(t, room).SpawnInfo {
		if s.MobId == mob {
			return true
		}
	}
	return false
}

// foldingInstallGrove installs the shipped unquiet_grove area (quest 45's
// cast step is scoped to it) over the coulee rooms it names.
func foldingInstallGrove(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "areas", "unquiet_grove.yaml"))
	require.NoError(t, err)
	var a areas.Area
	require.NoError(t, yaml.Unmarshal(data, &a))
	zones := map[int]string{}
	for _, id := range []int{foldingOrrinRoom, foldingGroveEdge, foldingVeilHollow, foldingThinningWood, foldingGroveHeart, foldingThinnestPlace} {
		zones[id] = "Pothole Coulee"
	}
	t.Cleanup(areas.InstallForTest([]areas.Area{a}, zones))
}

func foldingHint(q *QuestDef, id string) string {
	for _, s := range q.Steps {
		if s.Id == id {
			return s.Hint
		}
	}
	return ""
}

// foldingOffers maps each quest a Coulee dialogue file offers to its topic key.
func foldingOffers(t *testing.T, file string) map[int]string {
	t.Helper()
	out := map[int]string{}
	for _, tp := range loadCouleeTopics(t, file).Topics {
		if tp.Kind == dialogue.KindOffer {
			out[tp.Quest] = tp.Key
		}
	}
	return out
}

// First Casting: Toke sends the player to Grieve, Grieve's lesson opens the
// cast, and the cast is reported back to Grieve.
func TestQuest44FirstCasting_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "44-first_casting.yaml", 44)
	require.Equal(t, []string{"start", "met", "cast", "end"}, newLinearPlayer(q).steps,
		"start and cast keep their ids so a saved player resumes; met is the meeting with Grieve")
	turnIn := turnInRequires(t, "9144.yaml", "cast_turnin")
	require.Equal(t, []string{"44-cast"}, turnIn, "the turn-in gates on the last step")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber})
		require.Equal(t, "start", w.p.current, "a cast before Grieve's lesson must not count")
		w.notify("topic_heard", EventDetails{MobId: foldingGrieve, Topic: "cast_lesson"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingObservatory})
		require.Equal(t, "met", w.p.current, "the cast counts only in the Star Chamber")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber})
		require.Equal(t, "cast", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on cast can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("cast").p, turnIn))
	})

	// #697: the mote fights back and a second casting can scatter it, so no
	// line may promise it unharmed, and its own description says so.
	t.Run("nothing promises the mote is unharmed", func(t *testing.T) {
		for _, tr := range q.Triggers {
			for _, a := range tr.Actions {
				require.NotContains(t, strings.ToLower(a.SendText), "unharmed")
			}
		}
		mote := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9146-practice_mote.yaml")))
		require.NotContains(t, mote, "cannot hurt you")
		require.Contains(t, mote, "scatters it")
	})

	t.Run("directions are true", func(t *testing.T) {
		require.Contains(t, squash(foldingHint(q, "start")), "west, north, north, north, up and up to the observatory hall")
		require.Equal(t, foldingObservatory, foldingWalk(t, foldingHubSquare, "west", "north", "north", "north", "up", "up"))
		require.Contains(t, foldingHint(q, "met"), "Go north into the Star Chamber")
		require.Equal(t, foldingStarChamber, foldingWalk(t, foldingObservatory, "north"))
		require.True(t, foldingSpawns(t, foldingStarChamber, foldingPracticeMote))
	})
}

// Grieve's opener and topics follow each step of First Casting, and Grieve
// teaches where spells come from and what manifestation is.
func TestGrieve9144_TopicsNameEachStep(t *testing.T) {
	f := "44-first_casting.yaml"
	pre := []string{"30-end", "31-end"}
	checkMeetStep(t, "grieve", "9144.yaml", f, 44, "cast_lesson", "type cast spike mote", pre...)
	checkTopicSteps(t, "grieve", "9144.yaml", []topicStep{
		{f, 44, "met", "type cast spike mote", pre, "cast_progress", false},
		{f, 44, "cast", "", pre, "cast_turnin", true},
	})

	grieve := loadCouleeTopics(t, "9144.yaml")
	for key, help := range map[string]string{
		"new_spells":    "type help spells",
		"manifestation": "type help manifestation",
		"conjure":       "type help conjure-fire",
		"charm":         "type help charm",
	} {
		tp := grieve.FindTopic(key)
		require.NotNil(t, tp, "Grieve has no %s topic", key)
		require.Equal(t, dialogue.KindInfo, tp.Kind)
		require.Contains(t, squash(tp.Hints), help, "%s points at its help page", key)
	}
	require.Contains(t, strings.ToLower(grieve.FindTopic("new_spells").Text), "not from a trainer",
		"spells come from casting, not trainers (templates/help/spells.template)")
}
```

- [ ] **Step 3: Run the new tests and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest44|TestGrieve9144'
```
Expected: FAIL; `TestQuest44FirstCasting_GiverToGiver` reports the steps are `[start cast end]`, `TestGrieve9144_TopicsNameEachStep` reports quest 44 has no step "met" or Grieve has no `cast_lesson`.

- [ ] **Step 4: Replace quest 44**

Write `_datafiles/world/dogmud/quests/44-first_casting.yaml`:

```yaml
# Quest 44, First Casting (Pothole Coulee, casting line, quest 1 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   44-start: granted by Crier Toke's send_folding offer (9106), after 31-end.
#   44-met:   topic_heard on Adept Grieve's cast_lesson topic (9144): the
#             player went to Grieve and heard the lesson.
#   44-cast:  command trigger on `cast` in the Star Chamber (5305), where the
#             Practice Mote (9146) hangs as a target. Gated on 44-met.
#   44-end:   granted by Grieve's cast_turnin hand-in (needs 44-cast).
#
# The mote is combat_passive: it answers a cast feebly and a second casting
# can scatter it, and room 5305 respawns it. No line promises it unharmed
# (#697). New characters start with spellcasting rank 1 and Conviction Spike,
# so the beat is reachable cold.
#
# Saved characters: a player saved on start before this pass (the cast still
# to do) now talks to Grieve first; one saved on cast turns in as before.

questid: 44
name: First Casting
description: >-
  Adept Grieve keeps the ruined observatory where the Folding adepts once
  read the thin places. He teaches the first turning of the mind: how to
  cast a spell you already carry.
secret: false

steps:
  - id: start
    destination:
      mob: 9144
    description: >-
      Crier Toke has sent you to Adept Grieve, the caretaker of the old
      observatory. He teaches the first casting.
    hint: >-
      From Hub Square, go west, north, north, north, up and up to the
      Observatory Hall. Type talk grieve.
  - id: met
    destination:
      room: 5305
    description: >-
      Grieve has shown you the practice mote in the Star Chamber. Cast a
      spell you already know at it. Conviction Spike, the attack you woke
      with, will serve.
    hint: >-
      Go north into the Star Chamber and type cast spike mote.
  - id: cast
    destination:
      mob: 9144
    description: >-
      You shaped a spell and let it go at the mote. Return to Grieve in the
      Observatory Hall.
    hint: >-
      Go back south to Adept Grieve in the Observatory Hall and type talk
      grieve.
  - id: end
    description: >-
      Grieve has named you a caster of the first and smallest sort. When you
      are ready, he will send you north to the grove.

rewards:
  skillinfo: "spellcasting:1"
  itemid: 10018
  gold: 15
  actor: >-
    Grieve watches the mote take your spell, and something in his mismatched
    eyes settles. "There. You did not reach for it. You let it go, and that is
    the whole of it." He takes a long staff of pale oak from the wall and sets
    it in your hands. It is light, and the wood seems to lean toward the work.
    "A focus. It will not cast for you, but it will not fight you either.
    Remember the three channels: the body's force, the mind's will and the
    heart's conviction. You loosed conviction just now. Willpower is the stat
    that shapes a spell, and the more of it you carry, the cleaner the fold.
    When you are ready, I will send you north to the grove, where Orrin
    teaches the holding of a casting while the world tries to knock it loose."
  observer: "Grieve presses a pale oak staff into the newcomer's hands."

triggers:
  # MET: hearing Grieve's lesson (the topic the giver's hint invites).
  - event: topic_heard
    mob: 9144
    topic: cast_lesson
    conditions:
      has: ["44-start"]
      missing: ["44-met"]
    actions:
      - grant: "44-met"

  # CAST: loose a spell at the practice mote (Star Chamber, 5305).
  - event: command
    command: cast
    room: 5305
    conditions:
      has: ["44-met"]
      missing: ["44-cast"]
    actions:
      - grant: "44-cast"
      - actor: >-
          Your spell leaves you and breaks against the mote in a flare of
          folded light. That is casting: will shaped to a purpose and let go.
          The mote may flicker back at you, and a second casting can scatter
          it, but Grieve folds a new one out of the air for the next student.
          Go back south to Adept Grieve in the Observatory Hall and type talk
          grieve.
```

- [ ] **Step 5: Edit Grieve's dialogue (9144)**

Edit `_datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml`. If an `old_str` below no longer matches because plan 2 changed that block, STOP and report it.

Edit A, replace:
```yaml
  - questRequired:
    - 44-start
    questExcluded:
    - 44-cast
    - 44-end
    text: You have not loosed it yet. North, into the Star Chamber, where the practice
      mote hangs at the mark. Type cast spike mote and let the spell go at it. Then
      come back and tell me.
    hints: Go north into the Star Chamber and type cast spike mote.
```
with:
```yaml
  - questRequired:
    - 44-start
    questExcluded:
    - 44-met
    - 44-end
    text: Toke sent you up the stair, did he? Good. Then there is something to
      teach, and you have come to the right ruin for it.
    hints: Choose a line below to hear the first lesson.
  - questRequired:
    - 44-met
    questExcluded:
    - 44-cast
    - 44-end
    text: You have not loosed it yet. North, into the Star Chamber, where the practice
      mote hangs at the mark. Cast at it and let the spell go. Then come back and
      tell me.
    hints: Go north into the Star Chamber and type cast spike mote.
```

Edit B, replace:
```yaml
  - questRequired:
    - 44-end
    text: You can cast now. It is the first and smallest sort, but the door is open and
      it does not close again. North, through the ruin, the grove begins. Keeper Orrin
      tends it. He will teach you to hold a casting while something is trying to kill
      you, which is the part that matters.
    hints: Head north through the observatory into the grove and type talk orrin to
      find Keeper Orrin.
```
with:
```yaml
  - questRequired:
    - 44-end
    questExcluded:
    - 45-start
    - 45-end
    text: You can cast now. It is the first and smallest sort, but the door is open and
      it does not close again. North of this ruin the grove begins, and Keeper Orrin
      tends it. He teaches the part that matters, holding a casting while something
      is trying to kill you. When you are ready, I will send you to him.
    hints: Choose The Unquiet Grove below when you are ready for the grove.
  - questRequired:
    - 45-start
    questExcluded:
    - 45-met
    - 45-end
    text: Orrin is waiting for you in the Whispering Grove. Go gently. The grove is
      kind to slow feet.
    hints: From the Observatory Hall, go north, west, north, north and north to the
      Whispering Grove. Type talk orrin.
  - questRequired:
    - 44-end
    text: You can cast now. It is the first and smallest sort, but the door is open and
      it does not close again. Cast often. Every casting that holds is a chance to
      learn another.
    hints: Choose a line below to hear about spells, the Folding or manifestation.
```

Edit C, replace:
```yaml
    Take this staff. Oak, a focus, it will steady your castings without ever casting
    for you. North now, to the grove. Orrin will teach you to hold a working together
    while the world tries to break it.
  hints: Quest complete. Head north into the grove and type talk orrin to find Keeper
    Orrin.
```
with:
```yaml
    Take this staff. Oak, a focus, it will steady your castings without ever casting
    for you. When you are ready, I will send you north to the grove, where Orrin
    teaches the holding of a working while the world tries to break it.
  hints: Quest complete. Choose The Unquiet Grove below when you are ready for the
    grove.
```

Edit D, replace:
```yaml
    of it. The Awakening leaves everyone a little open to the Folding, which is why
    you woke with a spell. I only teach the aiming.
```
with:
```yaml
    of it. The Awakening leaves everyone a little open to the Folding, which is why
    you woke with a spell. I only teach the aiming.
- key: cast_lesson
  kind: info
  label: Toke sent me to learn casting.
  text: Then here is the first turning. North of here is the Star Chamber, and at
    its center a practice mark with a mote of folded light hanging over it. I conjure
    them for students. Stand at the mark and cast a spell you already know at the
    mote. The spike you woke with will do. Do not force it. Shape it, and let it go.
    The mote may flicker back at you, weakly, and a second casting can scatter it.
    I will fold another. Then come back to me.
  hints: Go north into the Star Chamber and type cast spike mote.
  questRequired:
  - 44-start
  questExcluded:
  - 44-met
  - 44-end
- key: cast_progress
  kind: info
  label: Where is the practice mote?
  text: North, in the Star Chamber, over the scribed mark. Stand in the circle and
    let the spike go at it. It may flicker back at you. Pay that no mind.
  hints: Go north into the Star Chamber and type cast spike mote.
  questRequired:
  - 44-met
  questExcluded:
  - 44-cast
  - 44-end
- key: new_spells
  kind: info
  label: How do I learn new spells?
  text: Not from a trainer. Nobody can hand you a spell. Each time you cast and the
    working holds, there is a chance a new pattern settles in your mind, a spell you
    did not know before. The more you cast, the more you learn. The small spells
    come first. The great ones wait until your skill can hold them.
  hints: To see the spells you know, type spells. For more, type help spells.
  marked: true
- key: manifestation
  kind: info
  label: What is manifestation?
  text: The art of binding a creature to your will. You can call one out of nothing,
    bend one that already lives, or raise one that has fallen. It draws on charisma
    more than willpower, and every creature you hold keeps a share of your conviction
    set aside for as long as it stays with you. Nobody starts with such a spell. If
    one comes to you through casting, you will know it. Orrin can tell you about
    raising the dead, if you have the stomach for it.
  hints: For more, type help manifestation.
  marked: true
- key: conjure
  kind: info
  label: How does conjuring work?
  text: A conjured creature is called out of nothing. An elemental of fire, air, earth
    or water, the same wherever you stand, and nothing is used up to make it. Cast
    its spell and it comes bound to you. A mightier one holds more of your will, so
    a conjurer chooses with care.
  hints: For more, type help conjure-fire.
- key: charm
  kind: info
  label: Can I make a creature serve me?
  text: For a time, with the charm spell. It bends a living creature's mind to yours,
    and it fights at your side. But the hold ends without warning, and if you are
    standing beside it when it does, it turns on you. A stubborn mind fights the charm
    hardest. A sleeping one is easiest of all.
  hints: For more, type help charm.
```

(Grieve's `grove_offer` is added in Task 4, with quest 45.)

- [ ] **Step 6: Fix the Practice Mote's description (#697)**

In `_datafiles/world/dogmud/mobs/pothole_coulee/9146-practice_mote.yaml`, replace:
```yaml
    open, a thing made of nothing but bent attention, durable enough to take
    a spell and shrug it off. When a cast strikes it the mote flares, draws
    inward, and reknits, absorbing the working the way still water absorbs
    a thrown stone. It cannot hurt you. It exists to be aimed at.
```
with:
```yaml
    open, a thing made of nothing but bent attention, sturdy enough to take
    a spell or two. When a cast strikes it the mote flares and draws inward.
    It answers a blow with a feeble flicker of its own, and a hard enough
    working scatters it, so Grieve folds a new one out of the air for the
    next student. It exists to be aimed at.
```

- [ ] **Step 7: Manifests**

In `_datafiles/world/dogmud/quests/step-ids.txt`, after the line `44-start`, add the line `44-met`.

In `_datafiles/world/dogmud/dialogue/topic-keys.txt`, replace the block
```
9144.cast_help
```
with
```
9144.cast_help
9144.cast_lesson
9144.cast_progress
9144.charm
9144.conjure
```
and after the line `9144.folding` add the lines
```
9144.manifestation
9144.new_spells
```
(If plan 2 put `9144.cast_offer` under `# retired`, leave it there.)

- [ ] **Step 8: Run the tests and see them pass**

```bash
go test ./internal/questengine/ -run 'TestQuest44|TestGrieve9144|TestNewbieQuest'
go test ./internal/guards/ -run 'TestStepIdManifest|TestTopicKeyManifest|TestDialogueHints'
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_AllDialogueFilesParse' .
```
Expected: all `ok`. The folding tests for 45, 46 and 57 are not in this file yet (Task 4 adds them); that is intended.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/quests/44-first_casting.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9146-practice_mote.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt internal/questengine/newbie_topic_steps_test.go internal/questengine/newbie_folding_steps_test.go
git commit -F - <<'EOF'
feat(coulee): First Casting starts with Grieve's lesson (#375 p7)

Quest 44 opens with a meeting step (topic_heard on Grieve's cast_lesson),
then the cast, then the hand-in. The cast line and the Practice Mote stop
promising the mote is unharmed (#697). Grieve teaches where new spells come
from and what manifestation, conjuring and charming are.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Casting line quests 45, 46, 57, bridge 94, Orrin, Toke hand-in

**Executor:** Sonnet.

**Files:**
- Modify: `quests/45-the_unquiet_grove.yaml`, `quests/46-the_unfolded.yaml`, `quests/57-tending_the_grove.yaml` (full replacements)
- Create: `quests/94-report_to_toke__casting_.yaml`
- Modify: `dialogue/pothole_coulee/9145.yaml` (full replacement), `9144.yaml` (add `grove_offer`), `9106.yaml` (append `report_casting_in`)
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Modify: `internal/questengine/newbie_folding_steps_test.go` (append tests)
- Create: `internal/questengine/coulee_bridges_test.go`

(All data paths are under `_datafiles/world/dogmud/`.)

- [ ] **Step 1: Append the casting tests**

Append to `internal/questengine/newbie_folding_steps_test.go`:

```go
// The Unquiet Grove: Grieve sends the player to Orrin; Orrin's lesson opens
// the grove fight; cast first, then finish the creature, then report.
func TestQuest45UnquietGrove_GiverToGiver(t *testing.T) {
	foldingInstallGrove(t)
	q := loadShippedQuest(t, "45-the_unquiet_grove.yaml", 45)
	require.Equal(t, []string{"start", "met", "cast", "kill", "end"}, newLinearPlayer(q).steps,
		"cast comes before kill; the old ids survive so a saved player resumes")
	turnIn := turnInRequires(t, "9145.yaml", "grove_turnin")
	require.Equal(t, []string{"45-kill"}, turnIn, "the turn-in gates on the last step")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingVeilHollow})
		require.Equal(t, "start", w.p.current, "a cast before Orrin's lesson must not count")
		w.notify("topic_heard", EventDetails{MobId: foldingOrrin, Topic: "grove_meet"})
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
		require.Equal(t, "met", w.p.current, "a kill before the cast must not count")
		w.notify("command", EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: foldingVeilHollow})
		require.Equal(t, "met", w.p.current, "lighting the grove is not the combat cast")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingOrrinRoom})
		require.Equal(t, "met", w.p.current, "the cast counts only in the grove's fight rooms")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingVeilHollow})
		require.Equal(t, "cast", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the kill")
		w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("a wisp in the Thinning Wood serves too", func(t *testing.T) {
		w := newQuestWalker(q).at("met")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingThinningWood})
		require.Equal(t, "cast", w.p.current)
		w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingThinningWood})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("player saved on cast resumes with the kill", func(t *testing.T) {
		w := newQuestWalker(q).at("cast")
		w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingGroveHeart})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("directions are true", func(t *testing.T) {
		require.Contains(t, squash(foldingHint(q, "start")), "north, west, north, north and north to the whispering grove")
		require.Equal(t, foldingOrrinRoom, foldingWalk(t, foldingObservatory, "north", "west", "north", "north", "north"))
		require.Contains(t, foldingHint(q, "met"), "Go west to the Veil Hollow")
		require.Contains(t, foldingHint(q, "met"), darkAside)
		require.Equal(t, foldingVeilHollow, foldingWalk(t, foldingOrrinRoom, "west"))
		require.True(t, foldingSpawns(t, foldingVeilHollow, foldingGroveRevenant), "the met hint names the Veil Hollow's revenant")
	})
}

// The Unfolded: one boss kill on a route that walks, then the report to Orrin.
func TestQuest46Unfolded_RouteKillAndReport(t *testing.T) {
	q := loadShippedQuest(t, "46-the_unfolded.yaml", 46)
	require.Equal(t, []string{"start", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9145.yaml", "scab_turnin")
	require.Equal(t, []string{"46-kill"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingGroveHeart})
	require.Equal(t, "start", w.p.current)
	w.notify("mob_death", EventDetails{MobId: foldingTheUnfolded, RoomId: foldingThinnestPlace})
	require.Equal(t, "kill", w.p.current, "the boss kill moves to the report; Orrin takes the hand-in")
	require.False(t, w.p.HasQuest("46-end"))
	require.True(t, holdsAll(w.p, turnIn))

	hint := foldingHint(q, "start")
	require.Contains(t, hint, darkAside)
	require.Contains(t, hint, "west, north and north to the Grove Heart")
	require.Equal(t, foldingGroveHeart, foldingWalk(t, foldingOrrinRoom, "west", "north", "north"))
	require.Contains(t, hint, "west, up, north, west, north and east to the Thinnest Place")
	require.Equal(t, foldingThinnestPlace, foldingWalk(t, foldingGroveHeart, "west", "up", "north", "west", "north", "east"))
	require.True(t, foldingSpawns(t, foldingThinnestPlace, foldingTheUnfolded))

	back := squash(foldingHint(q, "kill"))
	require.Contains(t, back, "west, south, east, south, down, east, south, south and east to keeper orrin")
	require.Contains(t, back, "type talk orrin")
	require.Equal(t, foldingOrrinRoom, foldingWalk(t, foldingThinnestPlace, "west", "south", "east", "south", "down", "east", "south", "south", "east"))
}

// Tending the Grove: put a wisp down, then report it to Orrin.
func TestQuest57TendingTheGrove_ReportsToOrrin(t *testing.T) {
	q := loadShippedQuest(t, "57-tending_the_grove.yaml", 57)
	require.True(t, q.Repeatable)
	require.Equal(t, []string{"start", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9145.yaml", "grove_loop_turnin")
	require.Equal(t, []string{"57-kill"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
	require.Equal(t, "start", w.p.current)
	w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingGroveEdge})
	require.Equal(t, "kill", w.p.current)
	require.True(t, holdsAll(w.p, turnIn))

	require.Contains(t, foldingHint(q, "start"), "Go south to the Grove Edge")
	require.Equal(t, foldingGroveEdge, foldingWalk(t, foldingOrrinRoom, "south"))
	require.True(t, foldingSpawns(t, foldingGroveEdge, foldingDriftWisp))
	require.Contains(t, squash(foldingHint(q, "kill")), "type talk orrin")
}

// Orrin greets each step with its one command, a quest-aware topic answers
// it, and every hand-in he takes completes its quest.
func TestOrrinMidQuestAnswers(t *testing.T) {
	f45, f46, f57 := "45-the_unquiet_grove.yaml", "46-the_unfolded.yaml", "57-tending_the_grove.yaml"
	checkMeetStep(t, "orrin", "9145.yaml", f45, 45, "grove_meet", "type cast spike revenant", "30-end", "44-end")
	checkTopicSteps(t, "orrin", "9145.yaml", []topicStep{
		{f45, 45, "met", "type cast spike revenant", []string{"44-end"}, "grove_mid_cast", false},
		{f45, 45, "cast", "type cast spike", []string{"44-end"}, "grove_mid_kill", false},
		{f45, 45, "kill", "", []string{"44-end"}, "grove_turnin", true},
		{f46, 46, "start", "west, up, north, west, north and east to the thinnest place",
			[]string{"44-end", "45-end"}, "scab_mid", false},
		{f46, 46, "kill", "", []string{"44-end", "45-end"}, "scab_turnin", true},
		{f57, 57, "start", "type cast spike wisp", []string{"44-end", "45-end", "46-end"}, "grove_loop_mid", false},
		{f57, 57, "kill", "", []string{"44-end", "45-end", "46-end"}, "grove_loop_turnin", true},
	})
}

// Every offer sits with the giver who took the previous hand-in (ruleChain):
// Grieve offers the grove; Orrin offers the Unfolded, the repeatable and the
// bridge, and no longer offers the grove. Orrin teaches raising and assess.
func TestCastingLine_OffersAndOrrinTopics(t *testing.T) {
	require.Equal(t, "grove_offer", foldingOffers(t, "9144.yaml")[45])
	orrin := foldingOffers(t, "9145.yaml")
	require.NotContains(t, orrin, 45, "Grieve offers The Unquiet Grove now")
	require.Equal(t, "scab_offer", orrin[46])
	require.Equal(t, "grove_loop_offer", orrin[57])
	require.Equal(t, "report_casting_offer", orrin[94])

	df := loadCouleeTopics(t, "9145.yaml")
	for key, help := range map[string]string{"raise": "type help raise-skeleton", "assess": "type assess corpse"} {
		tp := df.FindTopic(key)
		require.NotNil(t, tp, "Orrin has no %s topic", key)
		require.Equal(t, dialogue.KindInfo, tp.Kind)
		require.Contains(t, squash(tp.Hints), help)
	}
}
```

- [ ] **Step 2: Write the bridge test (casting case now; Tasks 7 and 9 add lore and archery)**

Create `internal/questengine/coulee_bridges_test.go`:

```go
package questengine

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The bridges back to Toke (#375 phase 7, spec "One line, giver to giver"):
// once Giver 2 takes the hand-in of the line's finale, Giver 2 offers
// "Report to Toke (<Line>)" and Crier Toke (9106) takes its hand-in. The
// offer is hidden until the finale is handed in, the first step teaches talk,
// and the way back to Hub Square walks.
type couleeBridge struct {
	file      string
	id        int
	line      string
	giverFile string
	offer     string
	finale    string
	handin    string
	from      int
	route     []string
}

var couleeBridges = []couleeBridge{
	{"94-report_to_toke__casting_.yaml", 94, "Casting", "9145.yaml", "report_casting_offer", "46-end", "report_casting_in",
		foldingOrrinRoom, []string{"south", "south", "south", "east", "south", "down", "down", "south", "south", "south", "east"}},
}

func TestCouleeBridges_GiverOffersTokeTakes(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	for _, c := range couleeBridges {
		t.Run(c.line, func(t *testing.T) {
			q := loadShippedQuest(t, c.file, c.id)
			require.Equal(t, fmt.Sprintf("Report to Toke (%s)", c.line), q.Name)
			require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
			require.Contains(t, questHintFor(t, q, "start"), "type talk toke")
			require.Equal(t, foldingHubSquare, foldingWalk(t, c.from, c.route...), "the way back to Hub Square walks")

			giver := loadCouleeTopics(t, c.giverFile)
			offer := giver.FindTopic(c.offer)
			require.NotNil(t, offer, "%s has no %s", c.giverFile, c.offer)
			require.Equal(t, dialogue.KindOffer, offer.Kind)
			require.Equal(t, c.id, offer.Quest)
			require.Equal(t, []string{c.finale}, offer.QuestRequired, "offered only once the finale is handed in")
			require.Nil(t, newNpcTalker(giver, playerOn(q, "")).eligible(c.offer), "hidden before the finale")

			p := playerOn(q, "", c.finale)
			newNpcTalker(giver, p).choose(t, c.offer)
			require.Equal(t, "start", p.current)

			in := toke.FindTopic(c.handin)
			require.NotNil(t, in, "Toke has no %s", c.handin)
			require.Equal(t, dialogue.KindHandIn, in.Kind)
			require.Equal(t, c.id, in.Quest)
			newNpcTalker(toke, p).choose(t, c.handin)
			require.True(t, p.HasQuest(fmt.Sprintf("%d-end", c.id)))
		})
	}
}
```

- [ ] **Step 3: Run them and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest45|TestQuest46|TestQuest57|TestOrrin|TestCastingLine|TestCouleeBridges'
```
Expected: FAIL (old step lists; missing `scab_turnin`, `grove_loop_turnin`, `grove_meet`; quest file 94 not found).

- [ ] **Step 4: Replace quest 45**

Write `_datafiles/world/dogmud/quests/45-the_unquiet_grove.yaml`:

```yaml
# Quest 45, The Unquiet Grove (Pothole Coulee, casting line, quest 2 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   45-start: granted by Adept Grieve's grove_offer (9144), after 44-end.
#   45-met:   topic_heard on Keeper Orrin's grove_meet topic (9145): the
#             player walked to the grove and heard the lesson.
#   45-cast:  command trigger on `cast conviction-spike` in the unquiet_grove
#             area (areas/unquiet_grove.yaml: rooms 5311, 5312, 5313), where
#             the creatures attack on sight: casting with a fight on you.
#   45-kill:  mob_death of a Drift-Wisp (9147) or a Grove Revenant (9148);
#             two triggers, either grants it. Gated on 45-cast.
#   45-end:   granted by Orrin's grove_turnin hand-in (needs 45-kill).
#
# The reward teaches sparks (Conviction Sparks), the first area spell. The
# met step keeps the area destination that the live-destination test needs.
#
# Saved characters: a player saved on start before this pass (the cast still
# to do) now hears Orrin first; one saved on cast or kill goes on as before.

questid: 45
name: The Unquiet Grove
description: >-
  Keeper Orrin tends the meditation grove, where the veil between the world
  and the Folding has worn thin and the things on the far side bleed through.
  He teaches casting with a fight on you: holding a thought steady while the
  world tries to knock it loose.
secret: false

steps:
  - id: start
    destination:
      mob: 9145
    description: >-
      Adept Grieve has sent you north to Keeper Orrin, who tends the grove
      beyond the observatory.
    hint: >-
      From the Observatory Hall, go north, west, north, north and north to
      the Whispering Grove. Type talk orrin.
  - id: met
    destination:
      area: unquiet_grove  # the cast counts in any grove fight room
    description: >-
      Orrin wants you to fight in the grove and cast while you do it. The
      creatures in the Veil Hollow, the Thinning Wood and the Grove Heart
      attack on sight. Loose a spell at one with the fight on you. A hit can
      break the casting, so push through it.
    hint: >-
      If it is dark, type cast glow first. Go west to the Veil Hollow and
      type cast spike revenant.
  - id: cast
    destination:
      mob: [9147, 9148]  # a drift-wisp or a grove revenant, either kill counts
    description: >-
      You held a casting together while the grove worked at you. Now bring
      the creature down. A drift-wisp in the Thinning Wood serves as well as
      the revenant.
    hint: >-
      Keep casting until it falls. Type cast spike again.
  - id: kill
    destination:
      mob: 9145
    description: >-
      You cast with a fight on you and put down a veil-warped creature.
      Report to Keeper Orrin in the Whispering Grove.
    hint: >-
      Go back to Keeper Orrin in the Whispering Grove and type talk orrin.
  - id: end
    description: >-
      Orrin has named your concentration sound. He has one more task, and it
      is the hard one: the thing that still casts in the thinnest place.

rewards:
  skillinfo: "spellcasting:1"
  spellid: sparks
  gold: 25
  actor: >-
    Orrin's late-arriving outline catches up to him as he goes still. "You
    were hit, and the fold held anyway. That is concentration: holding on
    through the disturbance." He turns a palm up, and a scatter of weightless
    sparks gathers above it and disperses. "Here is a wider working, then:
    sparks. It is the weakest of the area spells, which is to say it is the
    first one you can trust yourself with. It touches everything in front of
    you at once. You loosed it from the same conviction you woke with. When
    you are ready, I have one more task for you, and it is the hard one."
  observer: ""

triggers:
  # MET: hearing Orrin's lesson.
  - event: topic_heard
    mob: 9145
    topic: grove_meet
    conditions:
      has: ["45-start"]
      missing: ["45-met"]
    actions:
      - grant: "45-met"

  # CAST: conviction-spike mid-fight anywhere in the unquiet_grove area.
  - event: command
    command: cast
    noun: conviction-spike
    area: unquiet_grove
    conditions:
      has: ["45-met"]
      missing: ["45-cast"]
    actions:
      - grant: "45-cast"
      - actor: >-
          You shape the fold with the fight pressing on you, and it holds.
          That is concentration. Now bring the creature down: type cast
          spike again until it falls.

  # KILL: defeat a veil-warped grove foe after the cast.
  - event: mob_death
    mob: 9147
    conditions:
      has: ["45-cast"]
      missing: ["45-kill"]
    actions:
      - grant: "45-kill"
      - actor: >-
          The drift-wisp comes apart into the motes it was made of and does
          not gather again. Go back to Keeper Orrin in the Whispering Grove
          and type talk orrin.
  - event: mob_death
    mob: 9148
    conditions:
      has: ["45-cast"]
      missing: ["45-kill"]
    actions:
      - grant: "45-kill"
      - actor: >-
          The grove revenant folds down and is still. Go back to Keeper Orrin
          in the Whispering Grove and type talk orrin.
```

- [ ] **Step 5: Replace quest 46**

Write `_datafiles/world/dogmud/quests/46-the_unfolded.yaml`:

```yaml
# Quest 46, The Unfolded (Pothole Coulee, casting line, finale)
#
# Token flow (#375 phase 7, one action per step; the finale ends at a hand-in):
#   46-start: granted by Keeper Orrin's scab_offer (9145), after 45-end.
#             Route from Orrin (5309), walked against the room exits: west,
#             north, north to the Grove Heart (5313), then west (Veil Tear
#             5314), up, north (Folded Distance 5316), west, north (The
#             Unraveling 5319), east to the Thinnest Place (5320).
#   46-kill:  mob_death of The Unfolded (9152). Gated on 46-start.
#   46-end:   granted by Orrin's scab_turnin hand-in (needs 46-kill). The
#             rewards (willpower, the heal spell, gold) pay out there.
#
# Design: a solo spell duel against a long-dead caster's escaped folds
# (harm-only spellbook, so it duels rather than self-heal stalls). The
# takeaway is the player's first heal.

questid: 46
name: The Unfolded
description: >-
  Keeper Orrin says the grove's trouble has a source: out on the reality-thin
  scabland, in the thinnest place of all, something that was once a Folding
  adept still casts. It is called the Unfolded. Orrin says it cannot be
  reasoned with and cannot be left alone. It has to be out-cast.
secret: false

steps:
  - id: start
    destination:
      room: 5320
    description: >-
      Orrin has set you against the Unfolded. Climb out of the grove through
      the Veil Tear, cross the Folded Distance and the Unraveling, and defeat
      the Unfolded in the Thinnest Place. It casts at you from across the
      room, so cast back and keep casting until it falls.
    hint: >-
      If it is dark, type cast glow first. From Keeper Orrin, go west, north
      and north to the Grove Heart, then west, up, north, west, north and
      east to the Thinnest Place, and type cast spike unfolded.
  - id: kill
    destination:
      mob: 9145
    description: >-
      The Unfolded is undone, and the thinnest place is quiet for the first
      time in a long age. Go back to Keeper Orrin and tell him.
    hint: >-
      From the Thinnest Place, go west, south, east, south, down, east, south,
      south and east to Keeper Orrin. Type talk orrin.
  - id: end
    description: >-
      You crossed a scabland that did not want crossing, out-cast a thing
      made of nothing but casting, and came back able to mend your own wounds
      with a thought.

rewards:
  stat_info: "willpower:3"
  spellid: heal
  gold: 50
  actor: >-
    Orrin listens, and his late-arriving outline catches up to him as he goes
    still. "Undone. I felt the thinnest place go quiet from here." You feel
    the long strain of holding castings together in the open settle into you
    as a deeper steadiness of will. "The scabland gave you something more,
    whether you noticed it or not," he says, and he shows you how to turn a
    fold toward mending and lay it on your own wounds. You know how to heal
    now.
  observer: ""

triggers:
  # BOSS KILL: defeat the Unfolded, then report to Orrin.
  - event: mob_death
    mob: 9152
    conditions:
      has: ["46-start"]
      missing: ["46-kill"]
    actions:
      - grant: "46-kill"
      - actor: >-
          The Unfolded unravels a final time and does not refold. The
          thinnest place falls quiet around you. Go back to Keeper Orrin in
          the Whispering Grove and type talk orrin.
```

- [ ] **Step 6: Replace quest 57**

Write `_datafiles/world/dogmud/quests/57-tending_the_grove.yaml`:

```yaml
# Quest 57, Tending the Grove (Pothole Coulee, casting line repeatable)
#
# Token flow (#375 phase 7: the repeatable ends at a hand-in):
#   57-start: granted by Keeper Orrin's grove_loop_offer (9145), after 46-end.
#   57-kill:  mob_death of a Drift-Wisp (9147). The nearest is at the Grove
#             Edge (5308), south of Orrin (5309).
#   57-end:   granted by Orrin's grove_loop_turnin hand-in (needs 57-kill).
#             Completing clears the token; the offer returns after the
#             cooldown.

questid: 57
name: Tending the Grove
description: >-
  Orrin says the grove needs tending. Warped things drift back in whenever
  the watch slips. Clear one out and tell him, and the practice counts as
  much as the clearing.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5308  # the Grove Edge, where a drift-wisp spawns
    description: >-
      Orrin has asked you to go into the grove and put down one of the
      veil-warped things that wanders back in.
    hint: >-
      If it is dark, type cast glow first. Go south to the Grove Edge and
      type cast spike wisp until it falls.
  - id: kill
    destination:
      mob: 9145
    description: >-
      The drift-wisp is gone. Tell Orrin the grove is quiet again.
    hint: >-
      Go back north to Keeper Orrin in the Whispering Grove and type talk
      orrin.
  - id: end
    description: >-
      The grove is quiet again for now. Orrin will have more work when the
      watch slips.

rewards:
  gold: 6
  skillinfo: "spellcasting:1"
  actor: >-
    Orrin nods, and for a moment every edge of him arrives on time. "The fold
    held clean. They always drift back in. Come back when the watch slips
    again."
  observer: >-
    Keeper Orrin nods to {actor}, and the grove settles, as if it approves of
    the stillness.

triggers:
  - event: mob_death
    mob: 9147
    conditions:
      has: ["57-start"]
      missing: ["57-kill"]
    actions:
      - grant: "57-kill"
      - actor: >-
          The drift-wisp unravels and is gone. Go back north to Keeper Orrin
          and type talk orrin.
```

- [ ] **Step 7: Create bridge 94**

Write `_datafiles/world/dogmud/quests/94-report_to_toke__casting_.yaml`:

```yaml
# Quest 94, Report to Toke (Casting) (Pothole Coulee, bridge, #375 phase 7)
#
# The file name is <id>-<ConvertForFilename(name)>.yaml, which the loader
# enforces (internal/quests/quests.go Filename): the parentheses become
# underscores.
#
# Token flow:
#   94-start: granted by Keeper Orrin's report_casting_offer (9145), after
#             46-end (he takes 46's hand-in, so ruleChain holds).
#   94-end:   granted by Crier Toke's report_casting_in hand-in (9106).

questid: 94
name: Report to Toke (Casting)
description: >-
  Keeper Orrin has nothing left to teach you in the grove. Crier Toke in Hub
  Square will want to hear how the casting trail went.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # Crier Toke follows his schedule (pothole_toke)
      room: 5203  # fallback: Hub Square
    description: >-
      Go back to Crier Toke in Hub Square and tell him you finished the
      casting trail.
    hint: >-
      From the Whispering Grove, go south three times, east, south, down,
      down, south three times and east to Hub Square. Type talk toke.
  - id: end
    description: >-
      You told Toke how the casting trail went, and the hub will know it by
      supper.
```

- [ ] **Step 8: Add Grieve's grove offer**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml`, replace:
```yaml
- key: cast_lesson
  kind: info
  label: Toke sent me to learn casting.
```
with:
```yaml
- key: grove_offer
  kind: offer
  quest: 45
  label: The Unquiet Grove
  playerLine: Will you teach me to cast while I'm under attack?
  text: That lesson is not mine to give. It is Orrin's. He keeps the meditation grove
    north of this ruin, where the veil has worn thin and the things on the far side
    come through. Go to him. He will set you against them and teach you to hold a
    casting while you are being hit.
  hints: From the Observatory Hall, go north, west, north, north and north to the
    Whispering Grove. Type talk orrin.
  breadcrumb: Learn the first casting with me before the grove. The grove does not
    forgive a fumbled fold.
  questRequired:
  - 44-end
  questExcluded:
  - 45-start
  - 45-end
  grantsQuest: 45-start
- key: cast_lesson
  kind: info
  label: Toke sent me to learn casting.
```

- [ ] **Step 9: Replace Orrin's dialogue**

Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9145.yaml`:

```yaml
mobid: 9145
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: Come in slowly, if you would. The grove is kind to slow feet. My name is
    Orrin, and I look after it.
  hints: Choose a line below to hear about casting in combat or your task.
  variants:
  - questExcluded:
    - 44-end
    text: You have not learned to cast yet. There is no holding a fold under pressure
      before you can shape one at all. Go back down to Adept Grieve in the observatory
      and learn the first casting. Come to me when your hands know the shape of it.
    hints: Adept Grieve in the Observatory Hall teaches the first casting. Crier Toke
      in Hub Square can send you to him.
  - questRequired:
    - 44-end
    questExcluded:
    - 45-start
    - 45-end
    text: Grieve's mark is on you. Good. When he thinks you are ready for the harder
      half, he will send you up to me.
    hints: Talk to Adept Grieve in the Observatory Hall to take up the grove's lesson.
  - questRequired:
    - 45-start
    questExcluded:
    - 45-met
    - 45-end
    text: Grieve sent you up, then. Sit a moment, and I will tell you what the grove
      asks of you.
    hints: Choose a line below to hear the grove's lesson.
  - questRequired:
    - 45-met
    questExcluded:
    - 45-cast
    - 45-end
    text: Not yet. West of here, in the Veil Hollow, a revenant walks. It will come
      at you the moment you step in. Let it, and loose a spell at it while it does.
    hints: If it is dark, type cast glow first. Go west to the Veil Hollow and type
      cast spike revenant.
  - questRequired:
    - 45-cast
    questExcluded:
    - 45-kill
    - 45-end
    text: The fold held under fire. I felt it. Now finish what you started. Keep casting
      at the thing until it falls.
    hints: Go west to the Veil Hollow and type cast spike revenant until it falls.
  - questRequired:
    - 45-kill
    questExcluded:
    - 45-end
    text: You are back, and the grove is a little quieter for it. Tell me how it went.
    hints: Choose The Unquiet Grove below to report.
  - questRequired:
    - 45-end
    questExcluded:
    - 46-start
    - 46-end
    text: You hold a casting under fire now. That is the skill that keeps a caster
      breathing. There is one thing left, and it is the hard one. The Unfolded,
      out on the scabland. When your nerve is up, I will tell you where it waits.
    hints: Choose The Unfolded below when you are ready for the final task.
  - questRequired:
    - 46-start
    questExcluded:
    - 46-kill
    - 46-end
    text: It is still out there, in the thinnest place. Through the Grove Heart, up
      through the Veil Tear, across the folded ground and the unraveling. It casts
      at range, so cast back and keep casting. Do not let the strange ground turn
      you around. Put it down.
    hints: If it is dark, type cast glow first. Go west, north and north to the Grove
      Heart, then west, up, north, west, north and east to the Thinnest Place, and
      type cast spike unfolded.
  - questRequired:
    - 46-kill
    questExcluded:
    - 46-end
    text: I felt the thinnest place go quiet from here. Tell me it is done.
    hints: Choose The Unfolded below to report.
  - questRequired:
    - 57-start
    questExcluded:
    - 57-kill
    - 57-end
    text: A wisp has drifted back in at the Grove Edge, just south of here. Put it
      down, then come back and tell me.
    hints: If it is dark, type cast glow first. Go south to the Grove Edge and type
      cast spike wisp until it falls.
  - questRequired:
    - 57-kill
    questExcluded:
    - 57-end
    text: The grove told me already. Tell me yourself.
    hints: Choose Tending the Grove below to report.
  - questRequired:
    - 46-end
    questExcluded:
    - 94-start
    - 94-end
    text: You undid the Unfolded. The grove has been quieter since. The thinning
      is closing, slow, the way a wound closes. You came up here barely able to
      cast. You leave able to mend yourself. Toke will want to hear it.
    hints: Choose Report to Toke (Casting) below to carry the word back to Toke.
  - questRequired:
    - 46-end
    text: You undid the Unfolded. The grove has been quieter since. The thinning
      is closing, slow, the way a wound closes. I have kept this place a long time
      and never thought to see that. You came up here barely able to cast. You leave
      able to mend yourself. Go well.
    hints: You have finished the casting trail. For more work in the grove, choose
      Tending the Grove below.
topics:
- key: grove_meet
  kind: info
  label: Grieve sent me to learn the grove.
  text: 'Right. Here is the work. West of here the veil-warped things gather: drift-wisps,
    and the revenants of those who came before and never left. They come at anyone
    who walks in. Go to the Veil Hollow, and when the revenant there is on you, loose
    a spell at it. You will be hit mid-fold. The fold will shudder. Hold it through,
    and keep casting until it falls. Then come back to me.'
  hints: If it is dark, type cast glow first. Go west to the Veil Hollow and type
    cast spike revenant.
  questRequired:
  - 45-start
  questExcluded:
  - 45-met
  - 45-end
- key: grove_turnin
  kind: handin
  quest: 45
  label: The Unquiet Grove
  playerLine: I held the fold under fire, and the revenant fell.
  text: 'You were hit and the fold held. That is concentration, not calm, but holding-on.
    Here is a wider working for it: sparks. The weakest of the area spells, which
    is the same as saying the first you can trust yourself with. It touches everything
    before you at once. Now the last task, when you are ready. The Unfolded, on the
    scabland. I will tell you where it waits.'
  hints: Quest complete. You learned sparks. Choose The Unfolded below when you are
    ready for the final task.
  questRequired:
  - 45-kill
  questExcluded:
  - 45-end
  grantsQuest: 45-end
- key: scab_offer
  kind: offer
  quest: 46
  label: The Unfolded
  playerLine: Do you want the Unfolded put down?
  text: The Unfolded, then. Long ago an adept of Grieve's house reached too far into
    a thin place and did not come all the way back. What stayed is the shape the reaching
    left, a thing made of nothing but casting, and it has been casting at the world
    ever since. Go through the Grove Heart and up through the Veil Tear, then across
    the scabland to the thinnest place. It will throw foldings at you from across
    the room. Cast back, hold your concentration, and keep casting. Out-cast it. There
    is no other way past it. Then come back and tell me.
  hints: If it is dark, type cast glow first. Go west, north and north to the Grove
    Heart, then west, up, north, west, north and east to the Thinnest Place, and
    type cast spike unfolded.
  breadcrumb: Learn to hold a casting first. The scabland keeps worse than revenants,
    and it will wait for you.
  questRequired:
  - 45-end
  questExcluded:
  - 46-start
  - 46-end
  grantsQuest: 46-start
- key: scab_turnin
  kind: handin
  quest: 46
  label: The Unfolded
  playerLine: The Unfolded is undone. The thinnest place is quiet.
  text: I felt it go. The whole grove let out a breath it has held for longer than
    I have kept it. You came up here barely able to cast, and you out-cast a thing
    made of nothing but casting. Toke will want to hear that, and the grove will
    always have a little work for you.
  hints: Quest complete. Choose Report to Toke (Casting) below to carry the word back
    to Toke, or Tending the Grove for more work here.
  questRequired:
  - 46-kill
  questExcluded:
  - 46-end
  grantsQuest: 46-end
- key: grove_loop_offer
  kind: offer
  quest: 57
  label: Tending the Grove
  playerLine: Is there more work in the grove for me?
  text: The warped things drift back in. That is the nature of a thin place, and it
    keeps leaking. There is a wisp at the Grove Edge, just south of here. Put it down,
    then come back and tell me. The practice matters as much as the clearing.
  hints: If it is dark, type cast glow first. Go south to the Grove Edge and type
    cast spike wisp until it falls.
  questRequired:
  - 46-end
  questExcluded:
  - 57-start
  - 57-end
  grantsQuest: 57-start
- key: grove_loop_turnin
  kind: handin
  quest: 57
  label: Tending the Grove
  playerLine: The wisp at the Grove Edge is down.
  text: Good. I felt the grove settle a little. The fold held clean, too. They always
    drift back in, so come back when the watch slips again.
  hints: Quest complete. Orrin will have more grove work after a while.
  questRequired:
  - 57-kill
  questExcluded:
  - 57-end
  grantsQuest: 57-end
- key: report_casting_offer
  kind: offer
  quest: 94
  label: Report to Toke (Casting)
  playerLine: Should I tell Toke how the casting trail went?
  text: Do. Toke hears everything eventually, but he likes to hear it first, and from
    the one who did it. He is in Hub Square, south of the Awakening Pool, shouting
    as usual.
  hints: From the Whispering Grove, go south three times, east, south, down, down,
    south three times and east to Hub Square. Type talk toke.
  questRequired:
  - 46-end
  questExcluded:
  - 94-start
  - 94-end
  grantsQuest: 94-start
- key: grove_mid_cast
  kind: info
  label: Where do I find the revenant?
  text: West of here is the Veil Hollow, and a revenant walks it. It will come at
    you as soon as you step in. That is the point. With it on you, loose the spike
    you woke with. A hit can shake the fold. Hold it anyway.
  hints: If it is dark, type cast glow first. Go west to the Veil Hollow and type
    cast spike revenant.
  questRequired:
  - 45-met
  questExcluded:
  - 45-cast
  - 45-end
- key: grove_mid_kill
  kind: info
  label: The fold held. What now?
  text: You held the fold under fire. Now finish it. Keep casting at the thing until
    it falls. A drift-wisp in the Thinning Wood, north of the Veil Hollow, serves
    as well as the revenant.
  hints: Go west to the Veil Hollow and type cast spike revenant until it falls.
  questRequired:
  - 45-cast
  questExcluded:
  - 45-kill
  - 45-end
- key: scab_mid
  kind: info
  label: How do I reach the thinnest place?
  text: Through the Grove Heart, then west to the Veil Tear and up. The Riven Ground,
    then the Folded Distance, where the far looks near. West from there, then north
    into the Unraveling, then east. That is the thinnest place, and it is waiting
    in it. Cast at it and keep casting.
  hints: If it is dark, type cast glow first. Go west, north and north to the Grove
    Heart, then west, up, north, west, north and east to the Thinnest Place, and
    type cast spike unfolded.
  questRequired:
  - 46-start
  questExcluded:
  - 46-kill
  - 46-end
- key: grove_loop_mid
  kind: info
  label: Where is the wisp?
  text: The wisp is at the Grove Edge, just south of here. Put it down with a casting,
    then come back and tell me.
  hints: If it is dark, type cast glow first. Go south to the Grove Edge and type
    cast spike wisp until it falls.
  questRequired:
  - 57-start
  questExcluded:
  - 57-kill
  - 57-end
- key: grove_help
  kind: info
  label: How do I cast while I'm being hit?
  text: 'Anyone can cast standing still in a quiet room. The grove teaches the other
    thing: holding the fold together while you are being hit. Two things make a
    combat caster. First, cast while fighting: type cast and
    the spell, the same as standing still, but expect the blow to come mid-fold. Second,
    concentration: a hit can shake a casting loose, and the answer is to keep shaping
    it through the hit rather than flinch off it. Get struck, and hold the fold anyway.
    The grove will give you plenty of chances to practice being struck.'
  hints: While fighting in the grove, type cast spike. Push the casting
    through the hits you take.
  marked: true
- key: raise
  kind: info
  label: Can the dead be raised to serve me?
  text: Some casters can. Raising is necromancy, a spell cast at a corpse that stands
    the remains back up as a servant, from a rattling skeleton to far worse. Better
    remains make a better servant. There is no raising without the spell, and nobody
    starts with it. If it comes to you through casting, the grove leaves plenty of
    remains to practice on.
  hints: For more, type help raise-skeleton.
- key: assess
  kind: info
  label: How do I judge a corpse before raising it?
  text: Study it first. Assessing a corpse tells you what forms of undeath it can hold,
    and how much of your conviction each would keep while it serves. You need not
    assess before raising, but a caster who looks first wastes less.
  hints: Next to a corpse, type assess corpse. For more, type help assess.
- key: outline
  kind: info
  label: Why does your outline lag behind you?
  text: You have noticed the lag. The grove does it to everyone who stays. The edges
    of me arrive a beat behind the rest. The Awakening opened me to this place and
    the place keeps the difference. The trick is to move slow enough that the lag
    cannot matter.
```

- [ ] **Step 10: Toke's casting hand-in**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`, append after the last topic in the file (whichever it is after plans 2 and 3a; keep its indentation, two spaces less than its fields):

```yaml
- key: report_casting_in
  kind: handin
  quest: 94
  label: Report to Toke (Casting)
  playerLine: Keeper Orrin has nothing left to teach me in the grove.
  text: Nothing left? Then the grove made a caster out of you, and the whole hub will
    know it by supper. I will see to that myself. When you want another trail, pick
    a mouth and I will point you down it.
  hints: Quest complete. Choose a line below to take up another trail.
  questRequired:
  - 94-start
  questExcluded:
  - 94-end
  grantsQuest: 94-end
  moodChange: friendly
```

- [ ] **Step 11: Manifests and backlog**

`quests/step-ids.txt`: after `45-end` add `45-met`; after `46-end` add `46-kill`; after `57-end` add `57-kill`; after the last `9x-...` line block, add (in sorted position, between `89-start` and `9-deliver`):
```
94-end
94-start
```

`dialogue/topic-keys.txt`: in the `9145.` block add the lines
```
9145.assess
9145.grove_loop_turnin
9145.grove_meet
9145.raise
9145.report_casting_offer
9145.scab_turnin
```
in sorted position; add `9144.grove_offer` after `9144.folding`; add `9106.report_casting_in` in the `9106.` block (sorted). Under the `# retired` section (create it at the end of the file as a line `# retired` if it does not exist), add the line `9145.grove_offer`. Leave `9145.grove_offer` in the key list too, or remove it from there: either passes the guard (`internal/manifest` `Diff`).

`quests/loop-backlog.txt`: delete exactly these six lines:
```
chain 44 mob 9145 topic grove_offer
chain 46 mob 9145 topic grove_loop_offer
end 46 quest 46 trigger mob_death mob=9152 grants=46-end
end 46 step end
end 57 quest 57 trigger mob_death mob=9147 grants=57-end
end 57 step end
```

- [ ] **Step 12: Run and see it pass**

```bash
go test ./internal/questengine/ 2>&1 | tail -3
go test ./internal/guards/ -run 'TestStepIdManifest|TestTopicKeyManifest|TestDialogueHints|TestShippedContent' 2>&1 | tail -3
go test ./internal/quests/ -run TestShippedContent_EveryDestinationKindHasALiveStep
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_AllDialogueFilesParse' . 2>&1 | tail -5
```
Expected: all `ok`; the loop line reports 6 fewer known findings than Task 1's baseline and 0 new, 0 stale.

- [ ] **Step 13: Commit**

```bash
git add _datafiles/world/dogmud/quests/45-the_unquiet_grove.yaml _datafiles/world/dogmud/quests/46-the_unfolded.yaml _datafiles/world/dogmud/quests/57-tending_the_grove.yaml _datafiles/world/dogmud/quests/94-report_to_toke__casting_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9145.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9144.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/newbie_folding_steps_test.go internal/questengine/coulee_bridges_test.go
git commit -F - <<'EOF'
feat(coulee): casting line giver to giver, bridge 94 (#375 p7)

Grieve offers The Unquiet Grove (45), which opens with Orrin's lesson.
The Unfolded (46) and Tending the Grove (57) end at hand-ins on Orrin.
Orrin offers Report to Toke (Casting) (94); Toke takes it. Orrin teaches
raising and assess. Clears six backlog lines (two chain, four end).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Lore line quest 1, First Words (47) and Wenna

**Executor:** Sonnet.

**Files:**
- Modify: `_datafiles/world/dogmud/quests/47-first_words.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml` (targeted)
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`
- Modify: `internal/questengine/newbie_lore_steps_test.go` (full replacement; Tasks 6 and 7 fill in 48, 49, 58)

- [ ] **Step 1: Write the lore test file (47 now)**

Replace the whole of `internal/questengine/newbie_lore_steps_test.go` with:

```go
package questengine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// topicHints is the lowercased hint of one topic in a Pothole Coulee
// dialogue file.
func topicHints(t *testing.T, file, key string) string {
	t.Helper()
	tp := loadCouleeTopics(t, file).FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %s", file, key)
	return strings.ToLower(tp.Hints)
}

// roomDescription is a shipped room's description.
func roomDescription(t *testing.T, zoneFolder string, roomId int) string {
	t.Helper()
	var room struct {
		Description string `yaml:"description"`
	}
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", zoneFolder, fmt.Sprintf("%d.yaml", roomId)), &room))
	return room.Description
}

// The lore line (Pothole Coulee quests 47, 48, 49 and the repeatable 58)
// under #375 phase 7: giver to giver. Toke offers 47 and Wenna takes it;
// Wenna offers 48 and Hale takes it; Hale offers 49 and Sere takes it; Sere
// keeps the repeatable 58 and the bridge 95. One command per step; every
// quest after the first opens with the walk to the giver who takes it.

func loreStepHint(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return s.Hint
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

// loreOnlyFinalStepReports: the report back to the giver belongs to the last
// step before end. A meeting step (start, when the next step is met) also
// teaches talk, to the giver the player is walking to, and is left out.
func loreOnlyFinalStepReports(t *testing.T, q *QuestDef, giver string) {
	t.Helper()
	report := "type talk " + giver
	meeting := len(q.Steps) > 1 && q.Steps[1].Id == "met"
	last := q.Steps[len(q.Steps)-2].Id
	for _, s := range q.Steps {
		if s.Id == "end" || (meeting && s.Id == "start") {
			continue
		}
		has := strings.Contains(strings.ToLower(s.Hint), report)
		if s.Id == last {
			require.True(t, has, "quest %d last step %q must tell the player to type %s: %q", q.QuestId, s.Id, report, s.Hint)
		} else {
			require.False(t, has, "quest %d step %q previews the report; it belongs to the last step only: %q", q.QuestId, s.Id, s.Hint)
		}
	}
}

// loreOfferOn: the quest's offer sits on this lore giver and on no other.
func loreOfferOn(t *testing.T, id int, file string) {
	t.Helper()
	for _, f := range []string{"9153.yaml", "9157.yaml", "9160.yaml"} {
		has := false
		for _, tp := range loadCouleeTopics(t, f).Topics {
			if tp.Kind == dialogue.KindOffer && tp.Quest == id {
				has = true
			}
		}
		require.Equal(t, f == file, has, "quest %d's offer belongs on %s only (on %s: %v)", id, file, f, has)
	}
}

func TestQuest47FirstWords_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "47-first_words.yaml", 47)
	require.Equal(t, []string{"start", "met", "help", "end"}, newLinearPlayer(q).steps,
		"start and help keep their ids so a saved player resumes; met is the meeting with Wenna")
	turnIn := turnInRequires(t, "9153.yaml", "words_turnin")
	require.Equal(t, []string{"47-help"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "wenna")
	require.Equal(t, "Type help.", strings.TrimSpace(loreStepHint(t, q, "met")))
	require.Contains(t, squash(loreStepHint(t, q, "start")), "east, then north six times to the commons yard")
	require.Equal(t, 5324, foldingWalk(t, foldingHubSquare, "east", "north", "north", "north", "north", "north", "north"))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "start", w.p.current, "help before Wenna's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9153, Topic: "words_lesson"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "say", RoomId: 5324})
		require.Equal(t, "met", w.p.current, "another command must not finish the help lesson")
		// Help works anywhere, which is the lesson (owner 2026-10-17).
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "help", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on help can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("help").p, turnIn))
	})

	t.Run("Wenna's greeting and answers follow the step", func(t *testing.T) {
		f := "47-first_words.yaml"
		pre := []string{"30-end", "31-end"}
		checkMeetStep(t, "wenna", "9153.yaml", f, 47, "words_lesson", "type help", pre...)
		checkTopicSteps(t, "wenna", "9153.yaml", []topicStep{
			{f, 47, "met", "type help", pre, "words_progress", false},
			{f, 47, "help", "", pre, "words_turnin", true},
		})
	})

	// #690: nothing the player did was called "the reckoning"; the reckoning
	// is Hale's carved bone.
	t.Run("Wenna never calls the help the reckoning", func(t *testing.T) {
		raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9153.yaml")))
		require.NotContains(t, raw, "reckoning")
	})
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/questengine/ -run TestQuest47
```
Expected: FAIL (steps are `[start help end]`).

- [ ] **Step 3: Replace quest 47**

Write `_datafiles/world/dogmud/quests/47-first_words.yaml`:

```yaml
# Quest 47, First Words (Pothole Coulee, lore line, quest 1 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   47-start: granted by Crier Toke's send_folk offer (9106), after 31-end.
#   47-met:   topic_heard on Elder Wenna's words_lesson topic (9153).
#   47-help:  command trigger on `help`, anywhere: the lesson is that help
#             works anywhere (owner 2026-10-17). Gated on 47-met.
#   47-end:   granted by Wenna's words_turnin hand-in (needs 47-help).
#
# Saved characters: a player saved on start before this pass (help still to
# type) now hears Wenna first; one saved on help turns in as before.

questid: 47
name: First Words
description: >-
  Elder Wenna runs the outlying farmstead north of the coulee town, and the
  way of the folk waits with her. It is mostly the way of asking, of
  listening, and of knowing where to look a thing up.
secret: false

steps:
  - id: start
    destination:
      mob: 9153
    description: >-
      Crier Toke has sent you to Elder Wenna at the farmstead up the north
      shore. She teaches the way of the folk.
    hint: >-
      From Hub Square, go east, then north six times to the Commons Yard.
      Type talk wenna.
  - id: met
    destination: anywhere
    description: >-
      The way of the folk is mostly the way of asking, and of knowing where
      to look a thing up. Start with the in-game help. It works anywhere.
    hint: >-
      Type help.
  - id: help
    destination:
      mob: 9153
    description: >-
      You opened the help reference, your surest guide when memory fails.
      Return to Wenna.
    hint: >-
      Go back to Elder Wenna in the Commons Yard and type talk wenna.
  - id: end
    description: >-
      Wenna has named you a friend of the folk. When you are ready, she will
      send you on to the standing stones.

rewards:
  stat_info: "charisma:1"
  itemid: 20089
  gold: 15
  actor: >-
    Wenna watches you a moment with those flat gold eyes, then nods as though
    settling something. "You ask straight and you listen straight. That is most
    of getting on with people, and all of getting on with us." She knots a small
    charm of plaited grass and polished bone and presses it into your hand. "Wear
    it where it shows. The folk will know the sign and treat you the kindlier for
    it." A little of her unhurried certainty seems to stay with you as she turns
    back to the yard. "When you are ready, I will send you on to the stones. Hale
    keeps them, and he has been waiting a long time to tell their story to
    someone who will sit still for it."
  observer: "Wenna presses a small plaited charm into the newcomer's hand."

triggers:
  # MET: hearing Wenna's lesson.
  - event: topic_heard
    mob: 9153
    topic: words_lesson
    conditions:
      has: ["47-start"]
      missing: ["47-met"]
    actions:
      - grant: "47-met"

  # HELP: consult the in-game help reference, anywhere.
  - event: command
    command: help
    conditions:
      has: ["47-met"]
      missing: ["47-help"]
    actions:
      - grant: "47-help"
      - actor: >-
          That is the in-game help, your long-term reference, more patient
          than any person and always to hand. The folk set their lore in stone
          for the same reason: so it outlasts the remembering. Go back to
          Elder Wenna and type talk wenna.
```

- [ ] **Step 4: Edit Wenna's dialogue (9153)**

Edit `_datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml`. If an `old_str` no longer matches because plan 2 changed that block, STOP and report.

Edit A, replace:
```yaml
  - questRequired:
    - 47-start
    questExcluded:
    - 47-help
    - 47-end
    text: Still learning. Ask me about the folk, the ways, the stones, whatever you
      like. That is half the lesson. The other half is the in-game reckoning, the
      one that outlasts your memory. Open it.
    hints: Type help.
  - questRequired:
    - 47-help
    questExcluded:
    - 47-end
    text: You found the reckoning. Good. Tell me so properly.
    hints: Choose First Words below to report.
  - questRequired:
    - 47-end
    text: A friend of the folk, now, and wearing the sign of it. Good. The stones
      are west and north. Go and talk to Hale about them when you are ready. He keeps
      the deep part of us, the part even I only half recall.
    hints: Head to the standing stones (west and north) and type talk hale to find
      Loresinger Hale.
```
with:
```yaml
  - questRequired:
    - 47-start
    questExcluded:
    - 47-met
    - 47-end
    text: Toke sent you up the cartway? Sit, then. I have a lesson for you, and it
      is a short one.
    hints: Choose a line below to hear the first lesson.
  - questRequired:
    - 47-met
    questExcluded:
    - 47-help
    - 47-end
    text: Still learning. Ask me about the folk, the ways, the stones, whatever you
      like. That is half the lesson. The other half is the in-game help, the
      reference that outlasts your memory. Open it.
    hints: Type help.
  - questRequired:
    - 47-help
    questExcluded:
    - 47-end
    text: You opened the help. Good. Come and tell me about it.
    hints: Choose First Words below to report.
  - questRequired:
    - 47-end
    questExcluded:
    - 48-start
    - 48-end
    text: A friend of the folk, now, and wearing the sign of it. Good. When you want
      the deeper story, I will send you to the stones and to Hale, who keeps it.
    hints: Choose The Standing Stones below to go on to the stones.
  - questRequired:
    - 48-start
    questExcluded:
    - 48-met
    - 48-end
    text: Hale is waiting for you at the Stone Circle. Mind the green on your way.
      There is a quarrel parked on it.
    hints: From the Commons Yard, go north, west, north, north and north to the Outer
      Ring, then west into the Stone Circle. Type talk hale.
  - questRequired:
    - 47-end
    text: A friend of the folk, now, and wearing the sign of it. Good.
    hints: Choose a line below to hear about the folk and their ways.
```

Edit B, replace:
```yaml
    That is the whole way of the folk, and the rest is only more of it. Take this charm.
    Wear it where it shows, and ours will know you for a friend. Now the stones,
    west and north. Hale keeps the deep story. Go and let him tell it.
  hints: Quest complete. Head to the standing stones and type talk hale to find Loresinger
    Hale.
```
with:
```yaml
    That is the whole way of the folk, and the rest is only more of it. Take this charm.
    Wear it where it shows, and ours will know you for a friend. When you are ready,
    I will send you on to the stones, where Hale keeps the deep story.
  hints: Quest complete. Choose The Standing Stones below when you are ready for the
    stones.
```

Edit C, replace:
```yaml
  text: Not yet. You have done the asking part. Now open the in-game reckoning, the
    one that holds what memory drops. Then come back and tell me.
  hints: Type help.
  questRequired:
  - 47-start
```
with:
```yaml
  text: Not yet. You have done the asking part. Now open the in-game help, the one
    that holds what memory drops. Then come back and tell me.
  hints: Type help.
  questRequired:
  - 47-met
```

Edit D, replace:
```yaml
    type help and the in-game reckoning will hold it for you. It is the surest reference
```
with:
```yaml
    type help and the in-game help will hold it for you. It is the surest reference
```

Edit E, replace:
```yaml
    go and talk to him about the old tale. He has been waiting on a good listener a
    long while.
```
with:
```yaml
    go and talk to him about the old tale. He has been waiting on a good listener a
    long while.
- key: words_lesson
  kind: info
  label: Toke sent me to learn the ways of the folk.
  text: Then here is the first of it, and the whole of it. Learn to ask, and learn
    where to look. Ask me about the folk and our ways whenever you like, and listen
    to the answers. The other half is the in-game help, for when your own memory
    fails. It is more patient than I am and always to hand. Open it now, then come
    back to me.
  hints: Type help.
  questRequired:
  - 47-start
  questExcluded:
  - 47-met
  - 47-end
```
(Wenna's `stones_offer` is added in Task 6, with quest 48.)

- [ ] **Step 5: Manifests**

`quests/step-ids.txt`: after `47-help` add `47-met`. `dialogue/topic-keys.txt`: add `9153.words_lesson` in the `9153.` block (sorted, after `9153.stones`).

- [ ] **Step 6: Run and see it pass**

```bash
go test ./internal/questengine/ -run 'TestQuest47|TestNewbieQuest'
go test ./internal/guards/ -run 'TestStepIdManifest|TestTopicKeyManifest|TestDialogueHints'
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators' .
```
Expected: all `ok`.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/quests/47-first_words.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt internal/questengine/newbie_lore_steps_test.go
git commit -F - <<'EOF'
feat(coulee): First Words starts with Wenna's lesson (#375 p7)

Quest 47 opens with a meeting step (topic_heard on Wenna's words_lesson),
then help anywhere, then the hand-in. Wenna stops calling the help "the
reckoning" (#690).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Lore line quest 2, The Standing Stones (48), Hale, Garrow

**Executor:** Sonnet.

**Files:**
- Modify: `_datafiles/world/dogmud/quests/48-the_standing_stones.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9157.yaml` (full replacement), `9153.yaml` (add `stones_offer`)
- Modify: `_datafiles/world/dogmud/behaviors/pothole_coulee/9158-sullen_garrow.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/rooms/pothole_coulee/5335.yaml` (spawn entry)
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Modify: `internal/questengine/newbie_lore_steps_test.go` (append)
- Create: `internal/behaviortree/garrow_yield_test.go`

- [ ] **Step 1: Append the quest 48 test**

Append to `internal/questengine/newbie_lore_steps_test.go`:

```go
func TestQuest48StandingStones_GiverToGiver(t *testing.T) {
	const green, circle = 5335, 5333
	f := "48-the_standing_stones.yaml"
	q := loadShippedQuest(t, f, 48)
	require.Equal(t, []string{"start", "met", "taunt", "end"}, newLinearPlayer(q).steps,
		"start and taunt keep their ids so a saved player resumes; met is the meeting with Hale")
	turnIn := turnInRequires(t, "9157.yaml", "stones_turnin")
	require.Equal(t, []string{"48-taunt"}, turnIn)
	loreOnlyFinalStepReports(t, q, "hale")
	loreOfferOn(t, 48, "9153.yaml")
	require.Contains(t, squash(loreStepHint(t, q, "met")), "type taunt garrow")
	require.Contains(t, squash(loreStepHint(t, q, "start")),
		"north, west, north, north and north to the outer ring, then west into the stone circle")
	require.Equal(t, circle, foldingWalk(t, 5324, "north", "west", "north", "north", "north", "west"))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green})
		require.Equal(t, "start", w.p.current, "a taunt before Hale's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9157, Topic: "stones_meet"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "taunt", RoomId: circle})
		require.Equal(t, "met", w.p.current, "a taunt away from the green must not count")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green})
		require.Equal(t, "taunt", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Hale's greeting and answers follow the step", func(t *testing.T) {
		checkMeetStep(t, "hale", "9157.yaml", f, 48, "stones_meet", "type taunt garrow", "30-end", "47-end")
		checkTopicSteps(t, "hale", "9157.yaml", []topicStep{
			{f, 48, "met", "type taunt garrow", []string{"47-end"}, "stones_progress", false},
			{f, 48, "taunt", "", []string{"47-end"}, "stones_turnin", true},
		})
	})

	t.Run("Hale teaches rally and warcry", func(t *testing.T) {
		hale := loadCouleeTopics(t, "9157.yaml")
		for key, cmd := range map[string]string{"rally": "type rally", "warcry": "type warcry"} {
			tp := hale.FindTopic(key)
			require.NotNil(t, tp, "Hale has no %s topic", key)
			require.Equal(t, dialogue.KindInfo, tp.Kind)
			require.Contains(t, squash(tp.Hints), cmd)
		}
	})
}
```

- [ ] **Step 2: Write the Garrow behaviour test**

Create `internal/behaviortree/garrow_yield_test.go`:

```go
package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// #690: the quest 48 taunt line says Garrow quits the green. His tree's
// yield branch must make that true. A `flee` is a roll that can fail and
// leave a real fight running (the player could not walk south), so the
// branch speaks, emotes and despawns him; the player's aggro is released on
// the next round because the target is gone (hooks.ValidateAggro), and the
// Mustering Green's spawn list brings him back for the next student.
func TestGarrowYieldLeavesTheGreen(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	world := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	treePath := filepath.Join(world, "behaviors", "pothole_coulee", "9158-sullen_garrow.yaml")
	_, err := LoadTreeFromFile(treePath)
	require.NoError(t, err, "Garrow's tree must load")

	var doc struct {
		Tree struct {
			Children []struct {
				Event    string `yaml:"event"`
				Children []struct {
					Do   string `yaml:"do"`
					Cmd  string `yaml:"cmd"`
					Text string `yaml:"text"`
				} `yaml:"children"`
			} `yaml:"children"`
		} `yaml:"tree"`
	}
	data, err := os.ReadFile(treePath)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &doc))
	var steps []string
	found := false
	for _, br := range doc.Tree.Children {
		if br.Event != "mob_combat_round" {
			continue
		}
		found = true
		for _, a := range br.Children {
			step := a.Do
			if a.Cmd != "" {
				step += ":" + a.Cmd
			}
			steps = append(steps, step)
			require.NotContains(t, a.Text, "--", "no spaced dash in Garrow's lines")
		}
	}
	require.True(t, found, "Garrow's tree has a mob_combat_round yield branch")
	require.Equal(t, []string{"say", "emote", "command:despawn"}, steps, "he yields, leaves, and the green respawns him")

	var room struct {
		SpawnInfo []struct {
			MobId       int    `yaml:"mobid"`
			RespawnRate string `yaml:"respawnrate"`
			Message     string `yaml:"message"`
		} `yaml:"spawninfo"`
	}
	data, err = os.ReadFile(filepath.Join(world, "rooms", "pothole_coulee", "5335.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &room))
	seen := false
	for _, s := range room.SpawnInfo {
		if s.MobId == 9158 {
			seen = true
			require.NotEmpty(t, s.RespawnRate, "Garrow comes back for the next student")
			require.NotEmpty(t, s.Message, "his return is narrated")
		}
	}
	require.True(t, seen, "the Mustering Green spawns Garrow")
}
```

- [ ] **Step 3: Run them and see them fail**

```bash
go test ./internal/questengine/ -run TestQuest48
go test ./internal/behaviortree/ -run TestGarrowYieldLeavesTheGreen
```
Expected: both FAIL (old step list; the yield branch ends in `flee` and the say text has "--").

- [ ] **Step 4: Replace quest 48**

Write `_datafiles/world/dogmud/quests/48-the_standing_stones.yaml`:

```yaml
# Quest 48, The Standing Stones (Pothole Coulee, lore line, quest 2 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   48-start: granted by Elder Wenna's stones_offer (9153), after 47-end.
#   48-met:   topic_heard on Loresinger Hale's stones_meet topic (9157).
#   48-taunt: command trigger on `taunt` in the Mustering Green (5335), where
#             Sullen Garrow (9158) stands. Gated on 48-met. Garrow's tree
#             (behaviors/pothole_coulee/9158-sullen_garrow.yaml) makes him
#             yield and leave on the first round of the fight the taunt
#             starts, so the line below is true (#690); the green respawns
#             him for the next student.
#   48-end:   granted by Hale's stones_turnin hand-in (needs 48-taunt).
#
# Saved characters: a player saved on start before this pass (the taunt still
# to do) now hears Hale first; one saved on taunt turns in as before.

questid: 48
name: The Standing Stones
description: >-
  Loresinger Hale keeps the ring of standing stones and the deep story the folk
  carved into them. He has a use for someone with a steady tongue: a quarrel on
  the mustering green that gentle words have only made worse.
secret: false

steps:
  - id: start
    destination:
      mob: 9157
    description: >-
      Elder Wenna has sent you to Loresinger Hale, who keeps the ring of
      standing stones west and north of the farmstead.
    hint: >-
      From the Commons Yard, go north, west, north, north and north to the
      Outer Ring, then west into the Stone Circle. Type talk hale.
  - id: met
    destination:
      room: 5335
    description: >-
      Hale wants the standoff on the mustering green broken. Young Garrow has
      squared up against old Ferd and will not be reasoned down, so do not
      reason. Put a hard word on him: taunt him, and break his nerve.
    hint: >-
      Go north from the Stone Circle to the Mustering Green and type taunt
      garrow.
  - id: taunt
    destination:
      mob: 9157
    description: >-
      Your words did what reason could not. Garrow's bluster broke and he quit
      the green. Return to Hale.
    hint: >-
      Go south to Loresinger Hale at the Stone Circle and type talk hale.
  - id: end
    description: >-
      Hale has taken your measure and found it sound. When you are ready, he
      will send you on to the old shrine and the one who keeps it.

rewards:
  skillinfo: "rhetoric:1"
  itemid: 30066
  gold: 25
  actor: >-
    Hale's doubled voice settles to a single tone as he regards you. "I felt
    that from here, the moment his nerve went. That is rhetoric: the true word,
    landed where it weighs most. You have the beginnings of it." He lifts a flat
    strip of carved bone from among the stones and sets it in your hands. "A
    reckoning, the folk's whole story in the old script, small enough to carry.
    Read it when you have the patience. The oldest end is the part none of us
    can read any longer." He looks north, to where the ground rises past the
    ring. "There is a thing the bone only points at. When you are ready, I will
    send you to the one who keeps it."
  observer: ""

triggers:
  # MET: hearing Hale's task.
  - event: topic_heard
    mob: 9157
    topic: stones_meet
    conditions:
      has: ["48-start"]
      missing: ["48-met"]
    actions:
      - grant: "48-met"

  # TAUNT: face down the belligerent Garrow (Mustering Green, 5335).
  - event: command
    command: taunt
    room: 5335
    conditions:
      has: ["48-met"]
      missing: ["48-taunt"]
    actions:
      - grant: "48-taunt"
      - actor: >-
          Your words find the gap in his bluster and go straight through it. The
          fight goes out of Garrow all at once. He throws up his heavy hands,
          mutters a surrender and quits the green. Old Ferd lets out a breath he
          has been holding for days. Go south to Hale at the Stone Circle and
          type talk hale.
```

- [ ] **Step 5: Replace Hale's dialogue**

Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9157.yaml`:

```yaml
mobid: 9157
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: I sing the stones of this ring, so that what they carry outlasts the
    weather on the marks. Hale is my name.
  hints: Choose a line below to learn about the stones and the old story, or
    your task.
  variants:
  - questExcluded:
    - 47-end
    text: You came up here before you sat with Wenna, I think. The story starts at
      the farmstead, not the stones. Go and learn the way of the folk from her first,
      and come back when she names you a friend. The deep telling is not for strangers.
    hints: Elder Wenna at the farmstead teaches the way of the folk first. Crier Toke
      in Hub Square can send you to her.
  - questRequired:
    - 47-end
    questExcluded:
    - 48-start
    - 48-end
    text: Wenna's charm is on you, so you are one of ours now. When she sends you to
      me, I will have a use for you.
    hints: Talk to Elder Wenna in the Commons Yard to take up the stones' task.
  - questRequired:
    - 48-start
    questExcluded:
    - 48-met
    - 48-end
    text: Wenna sent you. Good. Then I can use you, and you can earn the deeper story
      in the doing.
    hints: Choose a line below to hear the task Wenna sent you for.
  - questRequired:
    - 48-met
    questExcluded:
    - 48-taunt
    - 48-end
    text: The green is still sour. Young Garrow will not be reasoned down, so do not
      reason. Go north to the green and put a hard true word on him, and break the
      standoff.
    hints: Go north to the Mustering Green and type taunt garrow.
  - questRequired:
    - 48-taunt
    questExcluded:
    - 48-end
    text: The green has gone quiet. I can hear it from here. Tell me how it went.
    hints: Choose The Standing Stones below to report.
  - questRequired:
    - 48-end
    questExcluded:
    - 49-start
    - 49-end
    text: You quieted the green with a handful of words. Ferd has not stopped thanking
      the stones for it. You have the reckoning-bone now. Read it when you can,
      though the oldest end will baffle you as it baffles me. There is a thing the
      bone only points at. When you are ready, I will send you to it.
    hints: Choose The Old Shrine below to go on to the shrine.
  - questRequired:
    - 49-start
    questExcluded:
    - 49-met
    - 49-end
    text: Sere is waiting at the old shrine. The way is not straight, so mind the
      turn at the Tall Stone.
    hints: From the Stone Circle, go north twice to the Tall Stone, then west, then
      north twice to the Old Shrine. Type talk sere.
  - questRequired:
    - 48-end
    text: You quieted the green with a handful of words. Ferd has not stopped thanking
      the stones for it.
    hints: Choose a line below to hear about the stones and the old story.
topics:
- key: stones_meet
  kind: info
  label: Wenna sent me to help with the stones.
  text: Then earn the telling. Just north of here, on the mustering green, young Garrow
    has squared up against old Ferd and dug in past all reason, a boundary quarrel
    swollen out of all sense. Gentle words have only fed it. So do not be gentle.
    Go and taunt him. Land one hard, true word where his bluster is thinnest and
    break his nerve. He will quit the green. Settle it, and the deep story is yours.
  hints: Go north to the Mustering Green and type taunt garrow.
  questRequired:
  - 48-start
  questExcluded:
  - 48-met
  - 48-end
- key: stones_turnin
  kind: handin
  quest: 48
  label: The Standing Stones
  playerLine: I broke Garrow's nerve. The green is quiet now.
  text: 'I felt it from here, the moment his nerve gave. That is the craft of the
    true word, and you have the beginnings of it. Take the reckoning-bone. It carries
    the whole story of us in the old script. And carry this with it: the oldest marks
    on it are not ours. The folk came from somewhere we no longer name. Sere keeps
    the proof of it at the old shrine. When you are ready, I will send you to her.'
  hints: Quest complete. You received the reckoning-bone. Choose The Old Shrine below
    when you are ready for the shrine.
  questRequired:
  - 48-taunt
  questExcluded:
  - 48-end
  grantsQuest: 48-end
- key: shrine_offer
  kind: offer
  quest: 49
  label: The Old Shrine
  playerLine: Will you send me to the old shrine?
  text: Sere keeps it, past the Tall Stone. She has felt something under its floor
    for thirty years, the same mark the bone carries at its oldest end. Go and see
    her. She will tell you what she needs.
  hints: From the Stone Circle, go north twice to the Tall Stone, then west, then
    north twice to the Old Shrine. Type talk sere.
  breadcrumb: Settle the green first. The shrine is not for the unready.
  questRequired:
  - 48-end
  questExcluded:
  - 49-start
  - 49-end
  grantsQuest: 49-start
- key: stones_progress
  kind: info
  label: How do I break Garrow's nerve?
  text: Not yet. The green is still sour. Go north to it and put one hard, true word
    on Garrow, where his bluster is thinnest. Words alone. No fists.
  hints: Go north to the Mustering Green and type taunt garrow.
  questRequired:
  - 48-met
  questExcluded:
  - 48-taunt
  - 48-end
# Restored from the old dispute patterns (dropped by the converter as
# covered), because the opener above invites the player to ask about it.
- key: dispute
  kind: info
  label: What is the quarrel on the green?
  text: There is a sourness on the mustering green just now. Young Garrow has squared
    up against old Ferd over a boundary stone, and neither will give. Soft words have
    only hardened them. It wants a harder word. If you have the nerve to be that word,
    that is the task I have for you.
  hints: Garrow stands on the Mustering Green, just north of the Stone Circle.
  questExcluded:
  - 48-taunt
- key: dispute_settled
  kind: info
  label: How is the green now?
  text: The green is quiet now. Ferd has his boundary stone back, and Garrow has his
    pride to nurse somewhere out of sight. That was your doing.
  questRequired:
  - 48-taunt
- key: rally
  kind: info
  label: How do I steady my allies in a fight?
  text: With a shout. A rally steadies you and everyone fighting beside you, so you
    all dodge, parry and block the better for a while. It costs conviction, and the
    better your rhetoric and charisma, the stronger it rings. You can sound it before
    a fight begins.
  hints: Type rally. For more, type help rally.
- key: warcry
  kind: info
  label: What does a war cry do?
  text: A war cry puts heart into the blows. You and everyone fighting beside you hit
    harder for a while. It costs conviction, it grows with your rhetoric and charisma,
    and you can sound it before a fight begins, the same as a rally.
  hints: Type warcry. For more, type help warcry.
- key: ring
  kind: info
  label: What story do these stones tell?
  text: Every stone in this ring is a page, and I am the one who still reads them.
    The story is long, and most of it is true. It is the whole story of the folk,
    and the oldest part is the strangest of all, which is where the folk came from.
- key: voice
  kind: info
  label: Why do you sing in two voices?
  text: You hear the second voice. The Awakening put it in me. When I sing the old
    lines they come out doubled, a lower tone under the first. I made it part of the
    craft. The stones were always meant to be sung, and now I sing them in two voices
    at once.
- key: origins
  kind: info
  label: Where did the folk come from?
  text: Where did the folk come from? The carvings say plainly that we came from somewhere,
    but they do not say where. The oldest marks, up where the weather has been at them
    longest, are not our script at all. Another hand. Something older and wider than
    the coulee, that we have only the edge of. I have given my life to those marks
    and I am no nearer the middle of them.
  marked: true
```

(Hale's `shrine_offer` is added here because quest 49 exists already; Task 7 converts 49 itself. Moving the offer now keeps the chain rule clean: Hale takes 48's hand-in. Sere's old `shrine_offer` is removed in Task 7; until then two NPCs offer 49, which no rule refuses.)

- [ ] **Step 6: Add Wenna's stones offer**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml`, replace:
```yaml
- key: words_lesson
  kind: info
  label: Toke sent me to learn the ways of the folk.
```
with:
```yaml
- key: stones_offer
  kind: offer
  quest: 48
  label: The Standing Stones
  playerLine: Will you send me to the standing stones?
  text: Hale keeps them, west and north of here, in their ring. He sings the story
    of us off the stones, and he has a use for someone with a steady tongue, a
    quarrel on the mustering green that soft words have only made worse. Go and see
    him. He will tell you the rest.
  hints: From the Commons Yard, go north, west, north, north and north to the Outer
    Ring, then west into the Stone Circle. Type talk hale.
  breadcrumb: Learn the first lesson with me before the stones. The deep telling is
    not for strangers.
  questRequired:
  - 47-end
  questExcluded:
  - 48-start
  - 48-end
  grantsQuest: 48-start
- key: words_lesson
  kind: info
  label: Toke sent me to learn the ways of the folk.
```

- [ ] **Step 7: Replace Garrow's tree**

Write `_datafiles/world/dogmud/behaviors/pothole_coulee/9158-sullen_garrow.yaml`:

```yaml
# Sullen Garrow (9158), the belligerent farmhand on the Mustering Green (5335).
#
# Owns the yield half of quest 48's taunt beat. Garrow is combat_passive (he
# never starts a fight). A taunt is a combat move, so it starts a real fight;
# on its first round his bluster collapses and he quits the green. The
# quest's command: taunt trigger (room 5335) narrates the beat; this tree
# makes it true.
#
# #690: he used to flee, a roll that can fail, which left a real fight
# running and the player unable to walk south. He now despawns. The player's
# aggro on him is released on the next combat round because the target is
# gone (hooks.ValidateAggro), and the Mustering Green's spawn list brings him
# back after a few rounds for the next student (rooms/pothole_coulee/5335.yaml).
# Test: internal/behaviortree/garrow_yield_test.go.

tree:
  type: selector
  children:

    # YIELD: the first round of a real confrontation breaks his nerve.
    - type: sequence
      event: mob_combat_round
      children:
        - type: action
          do: say
          text: All right! Enough. Have it your way.
        - type: action
          do: emote
          text: >-
            shoulders off the green without looking back, the fight gone out
            of him all at once.
        - type: action
          do: command
          cmd: despawn
```

- [ ] **Step 8: Respawn Garrow on the green**

In `_datafiles/world/dogmud/rooms/pothole_coulee/5335.yaml`, replace:
```yaml
spawninfo:
- mobid: 9158
- mobid: 9159
```
with:
```yaml
spawninfo:
- mobid: 9158
  message: Sullen Garrow stamps back onto the green, still sour.
  narraterespawn: true
  respawnrate: 5 rounds
- mobid: 9159
```

- [ ] **Step 9: Manifests and backlog**

`quests/step-ids.txt`: after `48-end` add `48-met`. `dialogue/topic-keys.txt`: add `9153.stones_offer`, `9157.rally`, `9157.shrine_offer`, `9157.stones_meet`, `9157.warcry` in sorted position; under `# retired` add `9157.stones_offer`.

`quests/loop-backlog.txt`: delete the line
```
chain 47 mob 9157 topic stones_offer
```

- [ ] **Step 10: Run and see it pass**

```bash
go test ./internal/questengine/ -run 'TestQuest47|TestQuest48|TestNewbieQuest'
go test ./internal/behaviortree/ -run TestGarrowYieldLeavesTheGreen
go test ./internal/guards/ 2>&1 | tail -3
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_ServerBootsCleanWithRealData' . 2>&1 | tail -5
```
Expected: all `ok`; the loop reports one fewer known finding than after Task 4. (`TestQuest49` in this file is still the old one until Task 7 replaces it; this file no longer has it, so nothing runs for 49 yet.)

- [ ] **Step 11: Commit**

```bash
git add _datafiles/world/dogmud/quests/48-the_standing_stones.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9157.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9153.yaml _datafiles/world/dogmud/behaviors/pothole_coulee/9158-sullen_garrow.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5335.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/newbie_lore_steps_test.go internal/behaviortree/garrow_yield_test.go
git commit -F - <<'EOF'
feat(coulee): Standing Stones from Wenna to Hale; Garrow quits the green (#375 p7)

Wenna offers quest 48, which opens with Hale's task. Hale offers The Old
Shrine and gives true directions to it (#689: north twice, west, north
twice). Garrow's yield despawns him instead of a flee roll that could fail,
and the green respawns him (#690). Hale teaches rally and warcry.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Lore line quests 49 and 58, Sere, the Reliquary, bridge 95

**Executor:** Sonnet.

**Files:**
- Modify: `quests/49-the_old_shrine.yaml`, `quests/58-reading_the_stones.yaml` (full replacements)
- Create: `quests/95-report_to_toke__lore_.yaml`
- Modify: `dialogue/pothole_coulee/9160.yaml` (full replacement), `9106.yaml` (append `report_lore_in`)
- Modify: `rooms/pothole_coulee/5343.yaml`
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Modify: `internal/questengine/newbie_lore_steps_test.go` (append), `internal/questengine/coulee_bridges_test.go` (add a case)

- [ ] **Step 1: Append the 49 and 58 tests**

Append to `internal/questengine/newbie_lore_steps_test.go`:

```go
// searchIn is the search command in a room: since #375 phase 7 quests 49 and
// 58 listen for the command itself, any search in the Reliquary.
func searchIn(roomId int) EventDetails {
	return EventDetails{Command: "search", RoomId: roomId}
}

func TestQuest49OldShrine_GiverToGiver(t *testing.T) {
	const reliquary, nave, shrine, circle = 5343, 5342, 5341, 5333
	f := "49-the_old_shrine.yaml"
	q := loadShippedQuest(t, f, 49)
	require.Equal(t, []string{"start", "met", "find", "end"}, newLinearPlayer(q).steps,
		"start and find keep their ids so a saved player resumes; met is the meeting with Sere")
	turnIn := turnInRequires(t, "9160.yaml", "shrine_turnin")
	require.Equal(t, []string{"49-find"}, turnIn)
	loreOnlyFinalStepReports(t, q, "sere")
	loreOfferOn(t, 49, "9157.yaml")

	// #689: the way from the Stone Circle is north twice, west, north twice.
	require.Contains(t, squash(loreStepHint(t, q, "start")),
		"north twice to the tall stone, then west, then north twice to the old shrine")
	require.Equal(t, shrine, foldingWalk(t, circle, "north", "north", "west", "north", "north"))

	met := strings.ToLower(loreStepHint(t, q, "met"))
	require.True(t, strings.HasPrefix(met, "from the old shrine"),
		"hint can be typed anywhere, so the directions start from where Sere gives the lesson")
	require.Contains(t, met, "type search floor")
	require.Contains(t, met, "type help search", "the lesson points at its help page")
	require.NotContains(t, met, "look at", "looking at the find is not a step")
	require.NotContains(t, met, "go up", "the Reliquary is west then north of the shrine, not up")
	require.Equal(t, reliquary, foldingWalk(t, shrine, "west", "north"))

	t.Run("the Reliquary hints at the floor, never instructs, and keeps no disc", func(t *testing.T) {
		desc := strings.ToLower(roomDescription(t, "pothole_coulee", reliquary))
		require.Contains(t, desc, "flagstone floor")
		require.NotContains(t, desc, "search", "a room hints at its secret; the step hint teaches the command (ruling 7)")
		room := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", "5343.yaml")))
		require.NotContains(t, room, "disc", "the disc hidden noun moved out with quest 76 (#625)")
	})

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", searchIn(reliquary))
		require.Equal(t, "start", w.p.current, "a search before Sere's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9160, Topic: "shrine_meet"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", searchIn(nave))
		require.Equal(t, "met", w.p.current, "a search outside the Reliquary must not count")
		w.notify("room_interact", EventDetails{RoomId: reliquary, Verb: "look", Noun: "floor", RoomNoun: "floor"})
		require.Equal(t, "met", w.p.current, "looking at the floor is not searching it")
		w.notify("command", searchIn(reliquary))
		require.Equal(t, "find", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
		require.Empty(t, w.ctx.discovered, "there is no hidden noun to discover any more")
	})

	t.Run("Sere's greeting and answers follow the step", func(t *testing.T) {
		checkMeetStep(t, "sere", "9160.yaml", f, 49, "shrine_meet", "type search floor", "30-end", "47-end", "48-end")
		checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
			{f, 49, "met", "type search floor", []string{"47-end", "48-end"}, "shrine_progress", false},
			{f, 49, "find", "", []string{"47-end", "48-end"}, "shrine_turnin", true},
		})
	})
}

func TestQuest58ReadingTheStones_ReportsToSere(t *testing.T) {
	const reliquary = 5343
	f := "58-reading_the_stones.yaml"
	q := loadShippedQuest(t, f, 58)
	require.True(t, q.Repeatable)
	require.Equal(t, []string{"start", "read", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9160.yaml", "stones_loop_turnin")
	require.Equal(t, []string{"58-read"}, turnIn)
	require.Contains(t, squash(loreStepHint(t, q, "start")), "type search floor")
	require.Contains(t, squash(loreStepHint(t, q, "read")), "type talk sere")

	w := newQuestWalker(q).at("start")
	w.notify("command", searchIn(5342))
	require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
	w.notify("command", searchIn(reliquary))
	require.Equal(t, "read", w.p.current)
	require.True(t, holdsAll(w.p, turnIn))

	// A real character on 49-end holds every earlier 48 and 49 token too.
	pre := []string{"47-end", "48-start", "48-met", "48-taunt", "48-end", "49-start", "49-met", "49-find", "49-end"}
	checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
		{f, 58, "start", "type search floor", pre, "stones_loop_progress", false},
		{f, 58, "read", "", pre, "stones_loop_turnin", true},
	})
}
```

In `internal/questengine/coulee_bridges_test.go`, replace:
```go
		foldingOrrinRoom, []string{"south", "south", "south", "east", "south", "down", "down", "south", "south", "south", "east"}},
}
```
with:
```go
		foldingOrrinRoom, []string{"south", "south", "south", "east", "south", "down", "down", "south", "south", "south", "east"}},
	{"95-report_to_toke__lore_.yaml", 95, "Lore", "9160.yaml", "report_lore_offer", "49-end", "report_lore_in",
		5341, []string{"south", "south", "east", "south", "south", "east", "south", "south", "south", "east",
			"south", "south", "south", "south", "south", "south", "west", "south"}},
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest49|TestQuest58|TestCouleeBridges'
```
Expected: FAIL (old step lists; no `shrine_meet`; file 95 missing).

- [ ] **Step 3: Replace quest 49**

Write `_datafiles/world/dogmud/quests/49-the_old_shrine.yaml`:

```yaml
# Quest 49, The Old Shrine (Pothole Coulee, lore line, finale)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   49-start: granted by Loresinger Hale's shrine_offer (9157), after 48-end.
#   49-met:   topic_heard on Hermit Sere's shrine_meet topic (9160).
#   49-find:  command trigger on `search` in the Reliquary (5343), any search
#             there (search floor, or plain search). The search command
#             raises the quest event after every successful search
#             (usercommands/skill.search.go); while this trigger would fire,
#             the search does not first say it found nothing (Task 2,
#             SearchOptions.QuestStep). Gated on 49-met.
#   49-end:   granted by Sere's shrine_turnin hand-in (needs 49-find).
#
# The disc under the floor is story only since quest 76 moved its item to
# Maren's Cottage (#625): no hidden noun, no discover action. The find text
# delivers the discovery, and the disc stays where it lies.
#
# Design: the quiet discovery capstone, no boss and no fight. Reward: a
# Charisma bump and the folk's recognition.

questid: 49
name: The Old Shrine
description: >-
  Hermit Sere keeps a shrine older than the standing stones, older than the folk
  who half-remember it. Something lies under the cracked floor of its innermost
  chamber. The reckoning-bone points at it, and Sere has waited a long time for
  a patient hand to bring it to light.
secret: false

steps:
  - id: start
    destination:
      mob: 9160
    description: >-
      Loresinger Hale has sent you to Hermit Sere, who keeps the old shrine
      past the Tall Stone.
    hint: >-
      From the Stone Circle, go north twice to the Tall Stone, then west, then
      north twice to the Old Shrine. Type talk sere.
  - id: met
    destination:
      room: 5343
    description: >-
      Sere wants you to find what lies beneath the shrine. In the Reliquary,
      the deepest back-chamber, the flagstone floor has cracked and heaved over
      something. Search it out.
    hint: >-
      From the Old Shrine, go west, then north into the Reliquary. Type search
      floor. For more, type help search.
  - id: find
    destination:
      mob: 9160
    description: >-
      You worked a disc of pale, impossible metal free of the floor, looked on
      it, and set it back where the light can reach it: a thing older than the
      folk, marked with a sign even the loresingers cannot read. Bring word to
      Sere.
    hint: >-
      Go south and east back to Hermit Sere in the Old Shrine and type talk
      sere.
  - id: end
    description: >-
      You have seen the thing under the shrine and brought word of it back. You
      do not understand it. Neither does Sere, nor Hale, nor the oldest reckoning
      the folk keep. But you have seen it, and the coulee folk will not forget
      that you were the one who did.

rewards:
  stat_info: "charisma:3"
  gold: 50
  actor: >-
    Sere listens to your account with the stillness of someone who has waited a
    long time to hear it, the pale shape of her skull faintly lit from within.
    "So it is real," she says at last, very quietly. "I have felt it under the
    floor for thirty years and never had the hands or the right to lift it. You
    did." She does not ask you to bring it to her. She seems to understand it is
    not a thing to be owned. "Leave it where the light can reach it. Some things
    are meant to be found more than once." Then, with a gravity that settles over
    you like a mantle: "You came to us a stranger. You leave a friend of the
    coulee folk, and the only living soul who has looked on the thing beneath
    the shrine and carried the wondering back out. Carry it well. One day,
    somewhere, it may come to mean more than either of us can see from here."
  observer: ""

triggers:
  # MET: hearing Sere's lesson.
  - event: topic_heard
    mob: 9160
    topic: shrine_meet
    conditions:
      has: ["49-start"]
      missing: ["49-met"]
    actions:
      - grant: "49-met"

  # FIND: search the Reliquary (5343).
  - event: command
    command: search
    room: 5343
    conditions:
      has: ["49-met"]
      missing: ["49-find"]
    actions:
      - grant: "49-find"
      - actor: >-
          Your fingers find the lifted edge of a flagstone, and under it
          something that is not stone: cool, smooth, regular. You work it free,
          a disc of pale grey metal the size of a spread hand, and nothing about
          it belongs here. It is neither cast nor forged nor any process the folk
          have a word for, its face made smooth the way still water is smooth.
          Fine raised lines branch and rejoin across it with a cold regularity,
          threading inward toward a single mark pressed deep at the center: a
          ring with a line struck through it, the very mark the reckoning-bone
          carries at its oldest, deepest end. It stays cool in your warm hands.
          You do not understand it, and you are fairly certain no one in the
          coulee does. It feels less like something the folk made and lost than
          like something that was already here when they came, waiting under the
          floor for a hand to lift it. You set it back in its bed of stone, where
          the light can reach it.
      - actor: "Go back to Sere in the Old Shrine and type talk sere."
```

- [ ] **Step 4: Replace quest 58**

Write `_datafiles/world/dogmud/quests/58-reading_the_stones.yaml`:

```yaml
# Quest 58, Reading the Stones (Pothole Coulee, lore line repeatable)
#
# Token flow (#375 phase 7: the repeatable ends at a hand-in):
#   58-start: granted by Hermit Sere's stones_loop_offer (9160), after 49-end.
#   58-read:  command trigger on `search` in the Reliquary (5343), any search
#             there. Gated on 58-start.
#   58-end:   granted by Sere's stones_loop_turnin hand-in (needs 58-read).
#             Completing clears the token; the offer returns after the
#             cooldown.

questid: 58
name: Reading the Stones
description: >-
  Sere believes the shrine still has things to teach anyone patient enough to
  listen. She wants you to go back to the Reliquary, read what the stones have
  to say, and tell her.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5343
    description: >-
      Sere has asked you to go back to the Reliquary and search the old
      stones again.
    hint: >-
      From the Old Shrine, go west into the Collapsed Nave and north into the
      Reliquary, then type search floor.
  - id: read
    destination:
      mob: 9160
    description: >-
      The stones gave up a little more than before. Tell Sere what they said.
    hint: >-
      Go south and east back to Hermit Sere in the Old Shrine and type talk
      sere.
  - id: end
    description: >-
      You read the stones again. Sere will have more for you when the shrine
      is ready to give it.

rewards:
  gold: 6
  skillinfo: "rhetoric:1"
  actor: >-
    Sere listens with her eyes closed. "Not quite what they said last time.
    Very old places keep speaking, and the words shift with the listener. Come
    again when you have the patience for it."
  observer: >-
    {actor} tells Hermit Sere what the stones said, and she listens with her
    eyes closed.

triggers:
  - event: command
    command: search
    room: 5343
    conditions:
      has: ["58-start"]
      missing: ["58-read"]
    actions:
      - grant: "58-read"
      - actor: >-
          The stones give up a little more than they did before. It is not
          much, but it is something. The mark at the center of the disc catches
          your eye again, unchanged and patient. Go back to Hermit Sere in the
          Old Shrine and type talk sere.
```

- [ ] **Step 5: Create bridge 95**

Write `_datafiles/world/dogmud/quests/95-report_to_toke__lore_.yaml`:

```yaml
# Quest 95, Report to Toke (Lore) (Pothole Coulee, bridge, #375 phase 7)
#
# The file name is <id>-<ConvertForFilename(name)>.yaml, which the loader
# enforces: the parentheses become underscores.
#
# Token flow:
#   95-start: granted by Hermit Sere's report_lore_offer (9160), after 49-end
#             (she takes 49's hand-in, so ruleChain holds).
#   95-end:   granted by Crier Toke's report_lore_in hand-in (9106).

questid: 95
name: Report to Toke (Lore)
description: >-
  You have looked on the thing beneath the old shrine. Crier Toke in Hub
  Square will want to hear how the lore trail ended.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # Crier Toke follows his schedule (pothole_toke)
      room: 5203  # fallback: Hub Square
    description: >-
      Go back to Crier Toke in Hub Square and tell him you finished the lore
      trail.
    hint: >-
      Go back past the stones and Elder Wenna's farmstead, south down the
      cartway to the shore, west to the Awakening Pool, then south to Hub
      Square. Type talk toke.
  - id: end
    description: >-
      You told Toke how the lore trail ended, and he will make a story of it.
```

- [ ] **Step 6: Replace Sere's dialogue**

Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9160.yaml`:

```yaml
mobid: 9160
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: Sere. Keeper of the old shrine, or its longest guest.
  hints: Choose a line below if you would like to hear about the shrine, or
    about what lies beneath it, or your task.
  variants:
  - questExcluded:
    - 48-end
    text: You have come up out of order. The stones come before the shrine. Go down
      to Hale, learn the folk's story and settle what needs settling, and come back
      when he has named you sound. What lies here is not for the unready, and I have
      waited this long. I can wait a little more.
    hints: Loresinger Hale at the Stone Circle has a task for you first.
  - questRequired:
    - 48-end
    questExcluded:
    - 49-start
    - 49-end
    text: Hale has named you sound, I hear. When he thinks you are ready, he will
      send you up to me.
    hints: Talk to Loresinger Hale at the Stone Circle to take up the shrine's task.
  - questRequired:
    - 49-start
    questExcluded:
    - 49-met
    - 49-end
    text: Hale sent you, and the reckoning-bone is on you, so you have seen the
      mark it carries at its oldest end. I have felt that same mark under my floor
      for thirty years.
    hints: Choose a line below to hear what lies beneath the shrine.
  - questRequired:
    - 49-met
    questExcluded:
    - 49-find
    - 49-end
    text: It is still down there, under the Reliquary floor. West into the nave, then
      north, to the deepest chamber, where the flags have cracked and heaved. Search
      it out with your hands.
    hints: Go west into the Collapsed Nave and north into the Reliquary, then type
      search floor.
  - questRequired:
    - 49-find
    questExcluded:
    - 49-end
    text: You have the look of someone who has touched it. Tell me. Is it real?
    hints: Choose The Old Shrine below to report.
  - questRequired:
    - 58-start
    questExcluded:
    - 58-read
    - 58-end
    text: The stones are waiting for you in the Reliquary. Go and listen to them,
      then come and tell me what they said.
    hints: Go west into the Collapsed Nave and north into the Reliquary, then type
      search floor.
  - questRequired:
    - 58-read
    questExcluded:
    - 58-end
    text: You have been listening to them. I can see it on you. Tell me.
    hints: Choose Reading the Stones below to report.
  - questRequired:
    - 49-end
    questExcluded:
    - 95-start
    - 95-end
    text: You looked on it, and you came back, and you carry the wondering the way
      I have carried it lightly, because there is nothing else to do with it. You
      are a friend of the folk now. Toke will want to hear of it, down in the town.
    hints: Choose Report to Toke (Lore) below to carry the word back to Toke.
  - questRequired:
    - 49-end
    text: You looked on it, and you came back, and you carry the wondering the way
      I have carried it lightly, because there is nothing else to do with it. You
      are a friend of the folk now, and the only soul living who has seen the thing
      beneath the shrine. That is no small thing to be. Go well, and come back if
      the world ever tells you what it was.
    hints: You have finished the lore trail. When the stones have more to tell,
      choose Reading the Stones below.
topics:
- key: shrine_meet
  kind: info
  label: Hale sent me to look beneath the shrine.
  text: Then I will send you where I cannot go on my own account. The Reliquary is
    the deepest chamber of the shrine, west into the nave from here, then north. Its
    flagstone floor has cracked and lifted over something that has wanted lifting
    a long time. Search the floor where it has lifted, and bring out what is under
    it. Look on it well. Then come and tell me what you saw.
  hints: Go west into the Collapsed Nave, then north into the Reliquary, and type
    search floor.
  questRequired:
  - 49-start
  questExcluded:
  - 49-met
  - 49-end
- key: shrine_turnin
  kind: handin
  quest: 49
  label: The Old Shrine
  playerLine: I found it under the Reliquary floor. It is real.
  text: So it is real. I have felt it under the floor for thirty years and never had
    the hands to lift it, and you did, in an afternoon. Leave it where the light
    can reach it. Some things are meant to be found more than once. You came to us
    a stranger, and you leave the one soul living who has looked on the thing beneath
    the shrine and carried the wondering back out. Carry it well. The world may tell
    you what it was, one day, somewhere far from here.
  hints: Quest complete. Choose Report to Toke (Lore) below to carry the word back
    to Toke.
  questRequired:
  - 49-find
  questExcluded:
  - 49-end
  grantsQuest: 49-end
- key: stones_loop_offer
  kind: offer
  quest: 58
  label: Reading the Stones
  playerLine: Is there more for me to do at the shrine?
  text: The shrine speaks differently each time. If you have the patience, go back
    to the Reliquary and search the floor again. The stones will tell you what they
    have to say. Then come and tell me what they said.
  hints: Go west into the Collapsed Nave and north into the Reliquary, then type
    search floor.
  questRequired:
  - 49-end
  questExcluded:
  - 58-start
  - 58-end
  grantsQuest: 58-start
- key: stones_loop_turnin
  kind: handin
  quest: 58
  label: Reading the Stones
  playerLine: I listened to the stones again.
  text: And they said something new to you. No, keep it. The words are yours this
    time. Come again when you have the patience for it.
  hints: Quest complete. Sere will have more for you after a while.
  questRequired:
  - 58-read
  questExcluded:
  - 58-end
  grantsQuest: 58-end
- key: report_lore_offer
  kind: offer
  quest: 95
  label: Report to Toke (Lore)
  playerLine: Should I tell Toke what I found under the shrine?
  text: Tell him you saw it, if you like. Toke will make a story of it either way,
    but it may as well be a true one. He is in Hub Square, south of the Awakening
    Pool.
  hints: Go back past the stones and Elder Wenna's farmstead, south down the cartway
    to the shore, west to the Awakening Pool, then south to Hub Square. Type talk toke.
  questRequired:
  - 49-end
  questExcluded:
  - 95-start
  - 95-end
  grantsQuest: 95-start
- key: shrine_progress
  kind: info
  label: Where is the thing beneath the floor?
  text: Not yet. It is still under the Reliquary floor. West into the nave from here,
    then north. Search the cracked flags with your hands.
  hints: Go west into the Collapsed Nave and north into the Reliquary, then type
    search floor.
  questRequired:
  - 49-met
  questExcluded:
  - 49-find
  - 49-end
- key: stones_loop_progress
  kind: info
  label: Where do I read the stones?
  text: The stones are waiting for you in the Reliquary, not here with me. Go and
    search them, then come back and tell me.
  hints: Go west into the Collapsed Nave and north into the Reliquary, then type
    search floor.
  questRequired:
  - 58-start
  questExcluded:
  - 58-read
  - 58-end
# Restored from the old "beneath" pattern gated on 49-end (dropped by the
# converter as covered).
- key: disc
  kind: info
  label: Is the disc still down there?
  text: The disc is still down there under the Reliquary floor, where the light can
    reach it. I go and sit with it some evenings. It does not seem to mind.
  questRequired:
  - 49-end
- key: shrine
  kind: info
  label: How old is this shrine?
  text: Older than the stones Hale sings, older than the folk who built the ring
    below. Someone raised it before any of us, and left, and left something behind.
    I have tended this place a long time on the strength of a feeling. Something
    lies under the floor of the Reliquary, the deepest room. I have felt it there
    for thirty years, under the cracked flags, and never had the hands or the right
    to lift it.
  questExcluded:
  - 49-find
- key: skull
  kind: info
  label: Why does your skull show at the temples?
  text: The thinning shows through me, yes. The bone of my skull is pale under the
    skin at the temples. The old place does it to whoever stays in it. I stayed longest.
    I have made my peace. The shrine and I have worn into the same shape.
```

- [ ] **Step 7: Toke's lore hand-in**

Append to the end of `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` (after `report_casting_in`):

```yaml
- key: report_lore_in
  kind: handin
  quest: 95
  label: Report to Toke (Lore)
  playerLine: I looked on the thing beneath the old shrine.
  text: Beneath the shrine? Sere let you lift it? Then you know more than half the
    coulee and all of the council. I will tell it carefully, mind. Some stories get
    bigger every time they cross the square, and that one starts big.
  hints: Quest complete. Choose a line below to take up another trail.
  questRequired:
  - 95-start
  questExcluded:
  - 95-end
  grantsQuest: 95-end
  moodChange: friendly
```

- [ ] **Step 8: The Reliquary loses its disc (#625)**

In `_datafiles/world/dogmud/rooms/pothole_coulee/5343.yaml`:

Replace:
```yaml
  recently, or often, as though someone has been checking it.
  Whatever pushed those flags up may still be lying under them.
biome: fort
```
with:
```yaml
  recently, or often, as though someone has been checking it.
biome: fort
```

Replace:
```yaml
    cracked and heaved and been scratched clean as though by repeated
    examination. The gap between two of the lifted flags is just wide
    enough to admit a finger.
```
with:
```yaml
    cracked and heaved and been scratched clean as though by repeated
    examination.
```

Delete the whole `hidden_nouns:` block, from the line `hidden_nouns:` through the last line of the disc's `hidden_description` (`      A disc of pale metal lies worked free of the cracked floor, too smooth` and `      and too regular to be anything the folk made.`), so `niches:` is followed directly by `idlemessages:`.

- [ ] **Step 9: Manifests and backlog**

`quests/step-ids.txt`: after `49-find` add `49-met`; after `58-end` add `58-read`; add `95-end` and `95-start` after `94-start`.

`dialogue/topic-keys.txt`: add `9160.report_lore_offer`, `9160.shrine_meet`, `9160.stones_loop_turnin`, `9106.report_lore_in` in sorted position; under `# retired` add `9160.shrine_offer`.

`quests/loop-backlog.txt`: delete exactly these three lines:
```
chain 48 mob 9160 topic shrine_offer
end 58 quest 58 trigger room_interact room=5343 noun=floor verb=search grants=58-end
end 58 step end
```

- [ ] **Step 10: Run and see it pass**

```bash
go test ./internal/questengine/ 2>&1 | tail -3
go test ./internal/guards/ 2>&1 | tail -3
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_ServerBootsCleanWithRealData' . 2>&1 | tail -5
```
Expected: all `ok`; the loop reports 3 fewer known findings, 0 new, 0 stale. `TestSearchTriggerNounsNameARoomNoun` still passes because the cottage's `stone` trigger is still a `verb: search` trigger.

- [ ] **Step 11: Commit**

```bash
git add _datafiles/world/dogmud/quests/49-the_old_shrine.yaml _datafiles/world/dogmud/quests/58-reading_the_stones.yaml _datafiles/world/dogmud/quests/95-report_to_toke__lore_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9160.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5343.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/newbie_lore_steps_test.go internal/questengine/coulee_bridges_test.go
git commit -F - <<'EOF'
feat(coulee): Old Shrine from Hale to Sere, real search, bridge 95 (#375 p7)

Quest 49 opens with Sere's lesson and finds on the search command in the
Reliquary; Reading the Stones (58) ends at a hand-in on Sere. The
Reliquary loses its disc hidden noun and the lines promising a find
(#625). Sere offers Report to Toke (Lore) (95); Toke takes it. Clears
three backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Archery line quest 1, First Shot (50) and Iden

**Executor:** Sonnet.

**Files:**
- Modify: `_datafiles/world/dogmud/quests/50-first_shot.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml` (targeted)
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Modify: `internal/questengine/newbie_one_command_steps_test.go` (TestQuest50, `lenientGrants`)
- Modify: `internal/questengine/newbie_topic_steps_test.go` (TestIden9161)

- [ ] **Step 1: Rewrite the quest 50 tests**

In `internal/questengine/newbie_one_command_steps_test.go`, replace
```go
	"50-shoot": true, // fire needs the sling readied (the equip step)
```
with nothing (delete that line; keep the other entries plan 3a left).

Replace the whole function `TestQuest50FirstShot_OneCommandPerStep` (from `func TestQuest50FirstShot_OneCommandPerStep(t *testing.T) {` to its closing brace at the end of the file) with:

```go
func TestQuest50FirstShot_OneCommandPerStep(t *testing.T) {
	const terrace, rangeRoom = 5351, 5350
	q := loadShippedQuest(t, "50-first_shot.yaml", 50)
	require.Equal(t, []string{"start", "met", "equip", "shoot", "ambush", "cover_shot", "end"}, newLinearPlayer(q).steps,
		"the old ids survive so a player mid-quest resumes; met is the meeting with Iden (#375 phase 7)")
	turnIn := turnInRequires(t, "9161.yaml", "shoot_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "equip", RoomId: rangeRoom, ItemId: 10038})
		require.Equal(t, "start", w.p.current, "the sling step waits for Iden's lesson")
		w.notify("topic_heard", EventDetails{MobId: 9161, Topic: "shoot_lesson"})
		require.Equal(t, "met", w.p.current)

		// The equip event names its item (2026-10-17 newbie-duo playtest:
		// `wield cudgel` printed "The sling settles in your hand").
		w.notify("command", EventDetails{Command: "equip", RoomId: rangeRoom, ItemId: 10043})
		require.Equal(t, "met", w.p.current, "wielding the cudgel must not finish the sling step")
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "met", w.p.current, "a shot before the sling step does not count")
		w.notify("command", EventDetails{Command: "equip", RoomId: rangeRoom, ItemId: 10038})
		require.Equal(t, "equip", w.p.current)

		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "shoot", w.p.current)
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "shoot", w.p.current, "a second shot in the open is not the shot from cover")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("command", EventDetails{Command: "sneak", RoomId: terrace})
		require.Equal(t, "ambush", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the shot from cover")
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "cover_shot", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the shot from cover")
	})

	// Shot is not a step of its own: the quest hands a pouch over when it
	// starts, unless the player already carries one.
	t.Run("the grant hands over shot once", func(t *testing.T) {
		const shotPouch = 30064
		w := newQuestWalker(q).at("start")
		w.notify("quest_granted", EventDetails{QuestToken: "50-start", RoomId: rangeRoom})
		require.Equal(t, []int{shotPouch}, w.ctx.givenItems, "a player with no shot gets a pouch")

		w = newQuestWalker(q).at("start")
		w.p.items[shotPouch] = true
		w.notify("quest_granted", EventDetails{QuestToken: "50-start", RoomId: rangeRoom})
		require.Empty(t, w.ctx.givenItems, "a player who already carries shot gets no second pouch")
	})

	t.Run("player saved on shoot resumes with sneak", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("command", EventDetails{Command: "sneak", RoomId: terrace})
		require.Equal(t, "ambush", w.p.current)
	})

	t.Run("player saved on ambush resumes with the shot", func(t *testing.T) {
		w := newQuestWalker(q).at("ambush")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "cover_shot", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("directions are true", func(t *testing.T) {
		require.Contains(t, questHintFor(t, q, "start"), "north, east, east, north, up, north, north and up to the firing range")
		require.Equal(t, rangeRoom, foldingWalk(t, foldingHubSquare, "north", "east", "east", "north", "up", "north", "north", "up"))
		require.Equal(t, terrace, foldingWalk(t, rangeRoom, "east"))
	})
}
```

In `internal/questengine/newbie_topic_steps_test.go`, replace the whole function `TestIden9161_TopicsNameEachStep` with:

```go
func TestIden9161_TopicsNameEachStep(t *testing.T) {
	f := "50-first_shot.yaml"
	pre := []string{"30-end", "31-end"}
	checkMeetStep(t, "iden", "9161.yaml", f, 50, "shoot_lesson", "type equip sling", pre...)
	checkTopicSteps(t, "iden", "9161.yaml", []topicStep{
		{f, 50, "met", "type equip sling", pre, "coach_start", false},
		{f, 50, "equip", "type fire butt", pre, "coach_equip", false},
		{f, 50, "shoot", "type sneak", pre, "coach_shoot", false},
		{f, 50, "ambush", "type fire butt", pre, "coach_ambush", false},
		{f, 50, "cover_shot", "", pre, "shoot_turnin", true},
	})
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest50|TestIden9161|TestNewbieQuestTriggersGateOnThePreviousStep'
```
Expected: FAIL (`TestQuest50` sees the old steps; `TestNewbieQuestTriggersGateOnThePreviousStep` reports the `50-shoot` trigger must gate on `50-equip`).

- [ ] **Step 3: Replace quest 50**

Write `_datafiles/world/dogmud/quests/50-first_shot.yaml`:

```yaml
# Quest 50, First Shot (Pothole Coulee, archery line, quest 1 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   50-start:      granted by Crier Toke's send_range offer (9106), after
#                  31-end; the offer hands over a sling (givesItem 10038). A
#                  quest_granted trigger below hands over a pouch of shot
#                  (30064) unless the player already carries one.
#   50-met:        topic_heard on Marksman Iden's shoot_lesson topic (9161).
#   50-equip:      command trigger on `equip` of the sling (item 10038).
#   50-shoot:      command trigger on `fire` at the Long Terrace (5351): a
#                  stone into the practice butt (9163). Gated on 50-equip, so
#                  the steps run in order (the one-action rule; the old gate
#                  on 50-start let a fire skip the equip step).
#   50-ambush:     command trigger on `sneak` at the Long Terrace: take cover.
#                  Sneak raises its event only when the player ends up hidden.
#   50-cover_shot: command trigger on `fire` at the Long Terrace once hidden.
#   50-end:        granted by Iden's shoot_turnin hand-in (needs 50-cover_shot).
#
# A player who already holds the sling in hand at the equip step takes it
# off (`remove sling`) and readies it again; Iden's coaching says so.
#
# Every character can sneak from creation (initAllSkills seeds every skill at
# rank 1). The practice butt respawns after 5 rounds (room 5351 spawninfo).
#
# Saved characters: start, equip, shoot, ambush and cover_shot keep their
# ids. A player saved on start before this pass now hears Iden first.

questid: 50
name: First Shot
description: >-
  Marksman Iden keeps the shooting terraces above the coulee, where the
  bluff-folk learn to reach out and touch a thing from across a distance.
  Ready your sling and put a stone into the straw butt at the end of the
  range.
secret: false

steps:
  - id: start
    destination:
      mob: 9161
    description: >-
      Crier Toke handed you a sling and sent you to Marksman Iden, who keeps
      the shooting terraces up the bluff.
    hint: >-
      From Hub Square, go north, east, east, north, up, north, north and up
      to the Firing Range. Type talk iden.
  - id: met
    destination: anywhere
    description: >-
      Iden wants the sling ready in your hand before anything else. If it is
      already in your hand, take it off and ready it again.
    hint: >-
      Type equip sling.
  - id: equip
    destination:
      room: 5351
    description: >-
      The sling is in your hand. The practice butt stands down-range on the
      Long Terrace, east of Marksman Iden on the Firing Range.
    hint: >-
      Go east from the Firing Range to the Long Terrace and type fire butt.
  - id: shoot
    destination:
      room: 5351
    description: >-
      You put a stone into the butt, and the sling readied itself without
      being told. Iden wants you to try it once more from cover.
    hint: >-
      Type sneak.
  - id: ambush
    destination:
      room: 5351
    description: >-
      You are out of sight. A shot taken from cover lands far harder than one
      the target saw coming, and it gives your place away when it lands.
    hint: >-
      Type fire butt.
  - id: cover_shot
    destination:
      mob: 9161
    description: >-
      You shot from cover and felt the difference. Marksman Iden is waiting
      on the Firing Range, west of the Long Terrace.
    hint: >-
      Go west to Marksman Iden and type talk iden.
  - id: end
    description: >-
      Iden has named your eye sound. When you are ready, she will send you on
      to Scout Bryn at the Canyon Mouth.

rewards:
  skillinfo: "ranged-combat:1"
  gold: 15
  actor: >-
    Iden watches the stone bury itself in the straw and gives a single nod.
    "There. A sling readies itself while your eye is still on the mark, so the
    only thing you have to think about is the mark. And you have the other
    half already: a shot from cover lands like nothing else, and it works
    once, because afterward they know where you are. The eye is the thing that
    matters now, the eye and never the arm. Keep the sling and the shot. They
    are yours." She tips her head north, toward the canyon mouth. "When you are
    ready, I will send you up to Bryn. He will show you how to put a shot into
    something a whole room away, before it ever knows you are there."
  observer: "Marksman Iden nods as the newcomer's shot finds the straw."

triggers:
  # SHOT: a pouch to feed the sling, unless the player already has one.
  - event: quest_granted
    quest_token: 50-start
    conditions:
      missing_item: 30064
    actions:
      - give_item: 30064

  # MET: hearing Iden's lesson.
  - event: topic_heard
    mob: 9161
    topic: shoot_lesson
    conditions:
      has: ["50-start"]
      missing: ["50-met"]
    actions:
      - grant: "50-met"

  # EQUIP: ready the sling (only the sling, item 10038, counts).
  - event: command
    command: equip
    item: 10038
    conditions:
      has: ["50-met"]
      missing: ["50-equip"]
    actions:
      - grant: "50-equip"
      - actor: >-
          The sling settles in your hand, a stone already in its cradle. Go
          to the Long Terrace, east of the Firing Range, and type fire butt.

  # FIRST SHOT: loose at the practice butt down-range (5351).
  - event: command
    command: fire
    room: 5351
    conditions:
      has: ["50-equip"]
      missing: ["50-shoot"]
    actions:
      - grant: "50-shoot"
      - actor: >-
          The stone leaves the sling and strikes the straw with a solid thud.
          Your hand has already found another and settled it in the cradle.
          Iden calls down the range: "Now again, from where it cannot see you
          coming. Once the butt is down, type sneak."

  # AMBUSH: take cover.
  - event: command
    command: sneak
    room: 5351
    conditions:
      has: ["50-shoot"]
      missing: ["50-ambush"]
    actions:
      - grant: "50-ambush"
      - actor: >-
          You settle out of sight behind the butts. The sling is already
          loaded, because it loaded itself. Type fire butt and see what a
          shot is worth when nothing is expecting it.

  # COVER SHOT: the shot that follows the sneak.
  - event: command
    command: fire
    room: 5351
    conditions:
      has: ["50-ambush"]
      missing: ["50-cover_shot"]
    actions:
      - grant: "50-cover_shot"
      - actor: >-
          The stone takes the butt dead center, harder than any shot before
          it. Go west to Marksman Iden and type talk iden.
```

- [ ] **Step 4: Edit Iden's dialogue (9161)**

Edit `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml`. If an `old_str` no longer matches because plan 2 changed that block, STOP and report.

Edit A, replace:
```yaml
  - questRequired:
    - 50-start
    questExcluded:
    - 50-equip
    - 50-shoot
    - 50-end
    text: Still at it. A sling does nothing in your pack. Ready it first.
    hints: Type equip sling.
```
with:
```yaml
  - questRequired:
    - 50-start
    questExcluded:
    - 50-met
    - 50-end
    text: Toke sent you up with a sling, I see. Good. Hold still and I will walk you
      through it one thing at a time.
    hints: Choose a line below to hear the first lesson.
  - questRequired:
    - 50-met
    questExcluded:
    - 50-equip
    - 50-end
    text: Still at it. A sling does nothing in your pack. Ready it first.
    hints: Type equip sling.
```

Edit B, replace:
```yaml
  - questRequired:
    - 50-end
    text: 'You have the eye. I knew it when your first stone found the straw and you
      did not flinch at the noise. East along the terraces and north into the canyon.
      Bryn holds the slot, and he will teach you the part I cannot from a safe
      terrace: shooting a thing that shoots back.'
    hints: Go east twice to the High Terrace, then north to the Canyon Mouth, where
      Scout Bryn waits.
```
with:
```yaml
  - questRequired:
    - 50-end
    questExcluded:
    - 51-start
    - 51-end
    text: 'You have the eye. I knew it when your first stone found the straw and you
      did not flinch at the noise. Bryn holds the canyon slot, and he will teach you
      the part I cannot from a safe terrace: shooting a thing that shoots back. When
      you are ready, I will send you to him.'
    hints: Choose Across the Canyon below to go on to the canyon.
  - questRequired:
    - 51-start
    questExcluded:
    - 51-met
    - 51-end
    text: Bryn is waiting at the Canyon Mouth. Keep your voice low when you get there.
      Sound carries in that canyon.
    hints: From the Firing Range, go east twice to the High Terrace, then north to the
      Canyon Mouth. Type talk bryn.
  - questRequired:
    - 50-end
    text: You have the eye. I knew it when your first stone found the straw and you
      did not flinch at the noise.
    hints: Choose a line below to hear about shooting.
```

Edit C, replace:
```yaml
  text: Not yet. A sling does nothing in your pack. Ready it first.
  hints: Type equip sling.
  questRequired:
  - 50-start
  questExcluded:
  - 50-equip
  - 50-end
```
with:
```yaml
  text: Not yet. A sling does nothing in your pack. Ready it first. If it is already
    in your hand, take it off and ready it again, so I can see you do it.
  hints: Type equip sling. If the sling is already in your hand, type remove sling
    first.
  questRequired:
  - 50-met
  questExcluded:
  - 50-equip
  - 50-end
```

Edit D, replace:
```yaml
    types down the road. The sling and the shot are yours. The
    eye is yours too, though it will want feeding. East along the terraces and north
    to the canyon when you are ready. Bryn will teach you the rest.
  hints: Quest complete. Go east twice to the High Terrace, then north to the Canyon
    Mouth, where Scout Bryn waits.
```
with:
```yaml
    types down the road. The sling and the shot are yours. The
    eye is yours too, though it will want feeding. When you are ready for the canyon,
    I will send you to Bryn. He will teach you the rest.
  hints: Quest complete. Choose Across the Canyon below when you are ready for the
    canyon.
```

Edit E, replace:
```yaml
    the read of wind and distance. Shoot enough and it sharpens on its own.
```
with:
```yaml
    the read of wind and distance. Shoot enough and it sharpens on its own.
- key: shoot_lesson
  kind: info
  label: Toke sent me to learn to shoot.
  text: Then you have the sling, plaited cord and a leather cradle, cheap and honest,
    and a pouch of shot to feed it. Both are yours to keep. Ready the sling first.
    The cradle fills itself while your eye is still on the mark, so there is no second
    motion to learn. I will walk you through the rest one thing at a time. If the
    sling is already in your hand, take it off and ready it again, so I can see you
    do it.
  hints: Type equip sling.
  questRequired:
  - 50-start
  questExcluded:
  - 50-met
  - 50-end
```
(Iden's `canyon_offer` is added in Task 9, with quest 51.)

- [ ] **Step 5: Manifests and backlog**

`quests/step-ids.txt`: after `50-equip` add `50-met`. `dialogue/topic-keys.txt`: add `9161.shoot_lesson` (sorted, after `9161.shoot_help`).

`quests/loop-backlog.txt`: delete exactly these three lines:
```
one-action 50 quest 50 trigger command room=5351 command=fire grants=50-shoot:prev
walk-dead-end 50 quest 50 trigger command room=5351 command=fire grants=50-cover_shot
walk-dead-end 50 quest 50 trigger command room=5351 command=fire grants=50-shoot
```

- [ ] **Step 6: Run and see it pass**

```bash
go test ./internal/questengine/ 2>&1 | tail -3
go test ./internal/guards/ -run 'TestStepIdManifest|TestTopicKeyManifest|TestDialogueHints'
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators' . 2>&1 | tail -5
```
Expected: all `ok`; 3 fewer known findings.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/quests/50-first_shot.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/newbie_one_command_steps_test.go internal/questengine/newbie_topic_steps_test.go
git commit -F - <<'EOF'
feat(coulee): First Shot starts with Iden's lesson, steps in order (#375 p7)

Quest 50 opens with a meeting step (topic_heard on Iden's shoot_lesson);
the first shot now gates on the equip step, so a fire can no longer skip
it. Clears the one-action and two walk-dead-end backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Archery line quests 51, 52, 59, Bryn, bridge 96

**Executor:** Sonnet.

**Files:**
- Modify: `quests/51-across_the_canyon.yaml`, `quests/52-the_overlook.yaml`, `quests/59-range_practice.yaml` (full replacements)
- Create: `quests/96-report_to_toke__archery_.yaml`
- Modify: `dialogue/pothole_coulee/9162.yaml` (full replacement), `9161.yaml` (add `canyon_offer`), `9106.yaml` (append `report_archery_in`)
- Modify: `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Modify: `internal/questengine/newbie_ranged_steps_test.go` (full replacement), `internal/questengine/coulee_bridges_test.go` (add a case)

- [ ] **Step 1: Rewrite the ranged tests**

Replace the whole of `internal/questengine/newbie_ranged_steps_test.go` with:

```go
package questengine

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The archery line under #375 phase 7: Toke offers First Shot (50) and Iden
// takes it; Iden offers Across the Canyon (51) and Bryn takes it; Bryn offers
// The Overlook (52), takes its report, and keeps Range Practice (59) and the
// bridge back to Toke. One command per step, in order; handouts give only to
// a player who does not already carry one.

const (
	canyonMouth    = 5353
	canyonNarrows  = 5354 // Echoing Narrows: the shot is taken FROM here
	soundingFloor  = 5356 // The Sounding Floor: where the scout stands
	overlookRoom   = 5364
	canyonScout    = 9165
	bowmasterSkell = 9169
	pouchOfShot    = 30064
	caseOfBolts    = 30063
	slingItem      = 10038
)

// squash lowercases and collapses whitespace, so a folded YAML line compares
// the way the player reads it.
func squash(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func readQuestFile(t *testing.T, file string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", file))
	require.NoError(t, err)
	return string(raw)
}

// playerOn builds a linear player for quest q at step, holding the given
// tokens from other quests (the prerequisites a real character would carry).
func playerOn(q *QuestDef, step string, other ...string) *linearPlayer {
	p := newLinearPlayer(q)
	p.current = step
	for _, tok := range other {
		p.fullMockPlayer.quests[tok] = true
	}
	return p
}

// questHintFor returns the step's own hint, squashed.
func questHintFor(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return squash(s.Hint)
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

func TestQuest51AcrossTheCanyon_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	require.Equal(t, []string{"start", "met", "shoot", "kill", "end"}, newLinearPlayer(q).steps,
		"the old ids survive so a player mid-quest resumes; met is the meeting with Bryn")
	turnIn := turnInRequires(t, "9162.yaml", "canyon_turnin")
	require.Equal(t, []string{"51-kill"}, turnIn)
	require.Equal(t, "canyon_offer", foldingOffers(t, "9161.yaml")[51], "Iden offers the canyon")
	require.NotContains(t, foldingOffers(t, "9162.yaml"), 51, "Bryn no longer offers it")

	require.Contains(t, questHintFor(t, q, "start"), "east twice to the high terrace, then north to the canyon mouth")
	require.Contains(t, questHintFor(t, q, "start"), "type talk bryn")
	require.Equal(t, canyonMouth, foldingWalk(t, 5350, "east", "east", "north"))
	require.Contains(t, questHintFor(t, q, "met"), "type fire scout north")
	require.Contains(t, q.Steps[1].Hint, darkAside)
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")
	require.Contains(t, questHintFor(t, q, "kill"), "type talk bryn")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
		require.Equal(t, "start", w.p.current, "a shot before Bryn's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9162, Topic: "canyon_meet"})
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
		require.Equal(t, "met", w.p.current, "a kill before the cross-room shot must not skip the shot")
		w.notify("command", EventDetails{Command: "fire", RoomId: soundingFloor})
		require.Equal(t, "met", w.p.current, "only a shot from the Echoing Narrows counts")
		w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
		require.Equal(t, "shoot", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the kill")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on shoot resumes with the kill", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("player saved on kill can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("kill").p, turnIn))
	})
}

func TestQuest52TheOverlook_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	require.Equal(t, []string{"start", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9162.yaml", "overlook_turnin")
	require.Equal(t, []string{"52-kill"}, turnIn)
	require.Contains(t, questHintFor(t, q, "start"), "type fire skell")
	require.Contains(t, q.Steps[0].Hint, darkAside, "the Overlook goes dark at night; the start hint carries the dark aside")
	require.NotContains(t, questHintFor(t, q, "start"), "ammo",
		"ammo advice is not the step's command; it belongs in the description")
	require.Contains(t, squash(q.Steps[0].Description), "ammo")

	back := questHintFor(t, q, "kill")
	require.Contains(t, back, "south three times, down, then south four times to the canyon mouth")
	require.Contains(t, back, "type talk bryn")
	require.Equal(t, canyonMouth, foldingWalk(t, overlookRoom, "south", "south", "south", "down", "south", "south", "south", "south"))

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
	require.Equal(t, "start", w.p.current, "only Skell's death moves the Overlook on")
	w.notify("mob_death", EventDetails{MobId: bowmasterSkell, RoomId: overlookRoom})
	require.Equal(t, "kill", w.p.current, "the boss kill moves to the report; Bryn takes the hand-in")
	require.False(t, w.p.HasQuest("52-end"))
	require.True(t, holdsAll(w.p, turnIn))
}

func TestQuest59RangePractice_ReportsToBryn(t *testing.T) {
	q := loadShippedQuest(t, "59-range_practice.yaml", 59)
	require.True(t, q.Repeatable)
	require.Equal(t, []string{"start", "shoot", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9162.yaml", "range_loop_turnin")
	require.Equal(t, []string{"59-kill"}, turnIn)
	require.Contains(t, questHintFor(t, q, "start"), "type fire scout north")
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")
	require.Contains(t, questHintFor(t, q, "kill"), "type talk bryn")

	// N13: Bryn is "he" (mobs/pothole_coulee/9162-scout_bryn.yaml).
	require.NotRegexp(t, regexp.MustCompile(`(?i)\b(she|her|hers|herself)\b`), readQuestFile(t, "59-range_practice.yaml"),
		"Bryn is a he")

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
	require.Equal(t, "start", w.p.current, "a kill before the shot does not count")
	w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
	require.Equal(t, "shoot", w.p.current)
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
	require.Equal(t, "kill", w.p.current)
	require.True(t, holdsAll(w.p, turnIn))
}

// Bryn greets each step with its command, a quest-aware topic answers it, and
// every hand-in he takes completes its quest.
func TestBryn9162_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f51, f52, f59 := "51-across_the_canyon.yaml", "52-the_overlook.yaml", "59-range_practice.yaml"
	checkMeetStep(t, "bryn", "9162.yaml", f51, 51, "canyon_meet", "type fire scout north", "30-end", "50-end")
	all := []string{"50-end", "51-end", "52-end"}
	checkTopicSteps(t, "bryn", "9162.yaml", []topicStep{
		{f51, 51, "met", "type fire scout north", []string{"50-end"}, "coach_51_start", false},
		{f51, 51, "shoot", "type fire scout", []string{"50-end"}, "coach_51_shoot", false},
		{f51, 51, "kill", "", []string{"50-end"}, "canyon_turnin", true},
		{f52, 52, "start", "type fire skell", []string{"50-end", "51-end"}, "coach_52_start", false},
		{f52, 52, "kill", "", []string{"50-end", "51-end"}, "overlook_turnin", true},
		{f59, 59, "start", "type fire scout north", all, "coach_59_start", false},
		{f59, 59, "shoot", "type fire scout", all, "coach_59_shoot", false},
		{f59, 59, "kill", "", all, "range_loop_turnin", true},
	})
}

// S8: a handout topic gives its item once. A player who already carries one
// sees the reminder instead, and gets nothing.
func TestRangedGivers_HandoutsGiveOnlyWhenTheItemIsMissing(t *testing.T) {
	q50 := loadShippedQuest(t, "50-first_shot.yaml", 50)
	q51 := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	q52 := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	iden := loadCouleeTopics(t, "9161.yaml")
	bryn := loadCouleeTopics(t, "9162.yaml")

	cases := []struct {
		name         string
		df           *dialogue.DialogueFile
		p            func() *linearPlayer
		supply, have string
		item         int
	}{
		{"iden shot", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "shot_supply", "shot_have", pouchOfShot},
		{"iden sling", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "sling_recovery", "sling_have", slingItem},
		{"bryn shot during 51", bryn, func() *linearPlayer { return playerOn(q51, "start", "50-end") }, "canyon_shot_supply", "canyon_shot_have", pouchOfShot},
		{"bryn bolts during 52", bryn, func() *linearPlayer { return playerOn(q52, "start", "50-end", "51-end") }, "ammo_resupply", "ammo_have", caseOfBolts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkHandout(t, newNpcTalker(c.df, c.p()), c.supply, c.have, c.item)
		})
	}
}
```

In `internal/questengine/coulee_bridges_test.go`, replace:
```go
			"south", "south", "south", "south", "south", "south", "west", "south"}},
}
```
with:
```go
			"south", "south", "south", "south", "south", "south", "west", "south"}},
	{"96-report_to_toke__archery_.yaml", 96, "Archery", "9162.yaml", "report_archery_offer", "52-end", "report_archery_in",
		canyonMouth, []string{"south", "west", "west", "down", "south", "south", "down", "south", "west", "west", "south"}},
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/questengine/ -run 'TestQuest51|TestQuest52|TestQuest59|TestBryn9162|TestCouleeBridges'
```
Expected: FAIL (old step lists, missing topics, missing file 96).

- [ ] **Step 3: Replace quest 51**

Write `_datafiles/world/dogmud/quests/51-across_the_canyon.yaml`:

```yaml
# Quest 51, Across the Canyon (Pothole Coulee, archery line, quest 2 of 3)
#
# Token flow (#375 phase 7, one action per step, giver to giver):
#   51-start: granted by Marksman Iden's canyon_offer (9161), after 50-end.
#   51-met:   topic_heard on Scout Bryn's canyon_meet topic (9162).
#   51-shoot: command trigger on `fire` in the Echoing Narrows (5354): the
#             cross-room shot. The player stands in 5354 and shoots the
#             Canyon Scout (9165) on the Sounding Floor (5356) with `fire
#             scout north`. The fire event carries the shooter's room, so the
#             trigger names 5354. Gated on 51-met.
#   51-kill:  mob_death of the Canyon Scout, who charges into 5354 when shot
#             (the retaliation lesson). Gated on 51-shoot.
#   51-end:   granted by Bryn's canyon_turnin hand-in (needs 51-kill).
#
# Saved characters: start, shoot and kill keep their ids. A player saved on
# start before this pass now hears Bryn first.

questid: 51
name: Across the Canyon
description: >-
  Scout Bryn holds the echoing box canyon below the raider bluffs, where
  sound and shot carry from wall to wall. He will teach you the trick the
  raiders use against trespassers: to put a shot into something a whole
  chamber away, before it can close the distance.
secret: false

steps:
  - id: start
    destination:
      mob: 9162
    description: >-
      Marksman Iden has sent you to Scout Bryn, who holds the canyon slot
      below the raider bluffs.
    hint: >-
      From the Firing Range, go east twice to the High Terrace, then north to
      the Canyon Mouth. Type talk bryn.
  - id: met
    destination:
      room: 5354  # Echoing Narrows: the shot is taken from here
    description: >-
      Bryn wants you to take the canyon scout posted on the Sounding Floor,
      and to do it the hard way: with a shot from the chamber below, before
      it ever reaches you. Stand in the Echoing Narrows and shoot across the
      exit into the chamber to the north. When it charges, finish it.
    hint: >-
      Go north to the Echoing Narrows and type fire scout north. If it is
      dark, type cast glow first.
  - id: shoot
    destination:
      room: 5354  # hold this ground; the scout charges to you
    description: >-
      Your shot crossed the gap and bit. The scout is coming for you. Hold
      your ground in the Narrows and put it down when it arrives.
    hint: >-
      When the scout reaches you, type fire scout until it falls.
  - id: kill
    destination:
      mob: 9162
    description: >-
      The scout is down. You reached across the canyon and stood your ground
      when the fight came to you. Scout Bryn waits in the Canyon Mouth.
    hint: >-
      Go back to Scout Bryn in the Canyon Mouth, south of the Narrows, and
      type talk bryn.
  - id: end
    description: >-
      Bryn has seen you reach across the canyon and connect. Up the chimney
      at the canyon's head the raider overlook waits, and the bowmaster who
      holds it, the first foe that fights the way you now can.

rewards:
  skillinfo: "ranged-combat:1"
  itemid: 10039
  item_info: "30063:1"
  gold: 25
  actor: >-
    Bryn's outline doubles for a moment as he turns to you, then settles.
    "You felt it, then. The shot lands, and the thing it lands on comes
    straight up the slot at you, so the cross-room shot is never free. You
    buy a hit, and you pay for it with a fight. Worth it, when you choose the
    ground." He works a compact crossbow loose from the rock behind him and
    hands it over with a case of bolts. "Heavier than a sling, and it hits
    like it. Above us is the overlook, and on it a bowmaster named Skell, who
    has made this whole canyon a killing field from up there. When you have
    the nerve, the chimney at the canyon's head goes up to him."
  observer: "Scout Bryn passes the newcomer a compact crossbow and a case of bolts."

triggers:
  # MET: hearing Bryn's lesson.
  - event: topic_heard
    mob: 9162
    topic: canyon_meet
    conditions:
      has: ["51-start"]
      missing: ["51-met"]
    actions:
      - grant: "51-met"

  # CROSS-ROOM SHOT: fire on the scout in 5356 from the Narrows (5354).
  - event: command
    command: fire
    room: 5354
    conditions:
      has: ["51-met"]
      missing: ["51-shoot"]
    actions:
      - grant: "51-shoot"
      - actor: >-
          Your shot streaks north out of the Narrows and finds the scout on
          the open floor beyond. It is hit, and it is coming. Hold your
          ground. When it reaches you, type fire scout until it falls.

  # KILL: the Canyon Scout charges in and is put down (9165).
  - event: mob_death
    mob: 9165
    conditions:
      has: ["51-shoot"]
      missing: ["51-kill"]
    actions:
      - grant: "51-kill"
      - actor: >-
          The canyon scout drops, run up the slot by your own shot. That is
          the whole of it: reach out, then stand your ground. Go back to
          Scout Bryn in the Canyon Mouth and type talk bryn.
```

- [ ] **Step 4: Replace quest 52**

Write `_datafiles/world/dogmud/quests/52-the_overlook.yaml`:

```yaml
# Quest 52, The Overlook (Pothole Coulee, archery line, finale)
#
# Token flow (#375 phase 7, one action per step; the finale ends at a hand-in):
#   52-start: granted by Scout Bryn's overlook_offer (9162), after 51-end.
#             Route: Canyon Head (5358) up to Overlook Rise (5360), then north
#             through Raider Perch (5361) and The High Stand (5362) to The
#             Overlook (5364), where Skell spawns.
#   52-kill:  mob_death of Bowmaster Skell (9169). Gated on 52-start.
#   52-end:   granted by Bryn's overlook_turnin hand-in (needs 52-kill). The
#             rewards (perception, a hunting bow, ammunition, gold) pay out
#             there.
#
# Design: the archery capstone. Skell is a kiting archer (the `archer`
# archetype): out-shoot him or close the gap faster than he opens it.

questid: 52
name: The Overlook
description: >-
  Bowmaster Skell holds the raider overlook above the canyon, and through it
  the whole climb. Scout Bryn says there is no passing the bluffs while Skell
  shoots from the high ground. He is the reason the band can tax everything
  that moves below. He fights the way you have just learned to: at range, on
  the move. Bring him down.
secret: false

steps:
  - id: start
    destination:
      room: 5364
    description: >-
      Bryn has set you against Bowmaster Skell. Climb from the Canyon Head to
      the raider overlook, fight up past the raider shooters, and bring down
      the bowmaster on the Overlook itself. Carry more ammo than you think you
      need before you climb: Skell gives ground and shoots, so the fight runs
      long. Out-shoot him or close fast. If you run low, Scout Bryn will spare
      bolts before you go up.
    hint: >-
      From the Canyon Head, go up, then keep north to the Overlook and type
      fire skell. If it is dark, type cast glow first.
  - id: kill
    destination:
      mob: 9162
    description: >-
      Skell is down and the overlook is taken. Go back down to Scout Bryn and
      tell him.
    hint: >-
      From the Overlook, go south three times, down, then south four times to
      the Canyon Mouth. Type talk bryn.
  - id: end
    description: >-
      Skell is down and the overlook is taken. The bluff band has lost the
      high ground that let it rule the canyon, and you have learned the last
      tool the coulee had to teach you. The country beyond the rim is open
      now, and you are ready for it.

rewards:
  stat_info: "perception:3"
  itemid: 10041
  item_info: "30062:3,30063:2,30064:2"
  gold: 50
  actor: >-
    Bryn hears you out, and his doubled outline settles. "Off the high stand.
    Then the canyon is ours, and the band has lost the ground that let it rule
    us." From the raider cache his scouts carried down, he hands you a full
    hunting bow and ammunition enough to keep it fed. You feel the long work
    of the terraces and the canyon settle into you as a keenness of eye.
    Distances come quicker now, and the right instant to loose is just there
    when you reach for it. The whole coulee lies open behind you, every spoke
    of it walked and won, and past the rim the wider scablands wait.
  observer: "Scout Bryn hands the newcomer a hunting bow from the raider cache."

triggers:
  # BOSS KILL: defeat Bowmaster Skell, then report to Bryn.
  - event: mob_death
    mob: 9169
    conditions:
      has: ["52-start"]
      missing: ["52-kill"]
    actions:
      - grant: "52-kill"
      - actor: >-
          Bowmaster Skell goes off the overlook and does not rise. The high
          ground is yours, and the canyon below falls quiet. Go back down to
          Scout Bryn at the Canyon Mouth and type talk bryn.
```

- [ ] **Step 5: Replace quest 59**

Write `_datafiles/world/dogmud/quests/59-range_practice.yaml`:

```yaml
# Quest 59, Range Practice (Pothole Coulee, archery line repeatable)
#
# Token flow (#375 phase 7: the repeatable ends at a hand-in):
#   59-start: granted by Scout Bryn's range_loop_offer (9162), after 52-end.
#   59-shoot: command trigger on `fire` in the Echoing Narrows (5354), the
#             cross-room shot at the Canyon Scout on the Sounding Floor
#             (5356). Gated on 59-start.
#   59-kill:  mob_death of the Canyon Scout (9165). Gated on 59-shoot.
#   59-end:   granted by Bryn's range_loop_turnin hand-in (needs 59-kill).
#             Completing clears the token; the offer returns after the
#             cooldown.

questid: 59
name: Range Practice
description: >-
  Bryn never stops drilling. He wants you to put a shot through the canyon
  slot, finish what charges you, and come back to tell him: the same lesson,
  kept sharp.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5354
    description: >-
      Bryn has sent you to the Echoing Narrows to put a shot across the exit
      at the canyon scout on the Sounding Floor, then bring it down when it
      charges.
    hint: >-
      Go north to the Echoing Narrows and type fire scout north. If it is
      dark, type cast glow first.
  - id: shoot
    destination:
      room: 5354
    description: >-
      The shot crossed the slot. Stand your ground in the Narrows and finish
      the scout when it reaches you.
    hint: >-
      When the scout reaches you, type fire scout until it falls.
  - id: kill
    destination:
      mob: 9162
    description: >-
      Shot placed, target finished. Tell Bryn the drill is done.
    hint: >-
      Go back south to Scout Bryn in the Canyon Mouth and type talk bryn.
  - id: end
    description: >-
      Shot placed, target finished. Bryn will have another run in the canyon
      ready when you are.

rewards:
  gold: 6
  skillinfo: "ranged-combat:1"
  actor: >-
    Bryn gives a single nod. "Good reach. Stood when it came. Come and find me
    when the eye starts to drift. It always drifts."
  observer: "Scout Bryn gives the newcomer a single nod."

triggers:
  - event: command
    command: fire
    room: 5354
    conditions:
      has: ["59-start"]
      missing: ["59-shoot"]
    actions:
      - grant: "59-shoot"
      - actor: >-
          The shot crosses the slot and bites. Now it will come at you.
          When it reaches you, type fire scout until it falls.

  - event: mob_death
    mob: 9165
    conditions:
      has: ["59-shoot"]
      missing: ["59-kill"]
    actions:
      - grant: "59-kill"
      - actor: >-
          The scout drops. Shot placed, fight finished. Go back south to
          Scout Bryn in the Canyon Mouth and type talk bryn.
```

- [ ] **Step 6: Create bridge 96**

Write `_datafiles/world/dogmud/quests/96-report_to_toke__archery_.yaml`:

```yaml
# Quest 96, Report to Toke (Archery) (Pothole Coulee, bridge, #375 phase 7)
#
# The file name is <id>-<ConvertForFilename(name)>.yaml, which the loader
# enforces: the parentheses become underscores.
#
# Token flow:
#   96-start: granted by Scout Bryn's report_archery_offer (9162), after
#             52-end (he takes 52's hand-in, so ruleChain holds).
#   96-end:   granted by Crier Toke's report_archery_in hand-in (9106).

questid: 96
name: Report to Toke (Archery)
description: >-
  Bowmaster Skell is off the overlook. Crier Toke in Hub Square will want to
  hear it from you before he shouts it across the square.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # Crier Toke follows his schedule (pothole_toke)
      room: 5203  # fallback: Hub Square
    description: >-
      Go back to Crier Toke in Hub Square and tell him you finished the
      archery trail.
    hint: >-
      From the Canyon Mouth, go south, west, west, down, south, south, down,
      south, west, west and south to Hub Square. Type talk toke.
  - id: end
    description: >-
      You told Toke the overlook is taken, and the whole square heard it a
      breath later.
```

- [ ] **Step 7: Replace Bryn's dialogue**

Write `_datafiles/world/dogmud/dialogue/pothole_coulee/9162.yaml`:

```yaml
mobid: 9162
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: Keep it low. Sound carries in this canyon. I'm Bryn, and I hold it.
  hints: Choose a line below to hear about the canyon or your task.
  variants:
  - questExcluded:
    - 50-end
    text: You have not been to the terraces yet. You are carrying that sling like
      it might bite you, if you are carrying one at all. Go up to Iden first and learn
      which end the stone comes out of. The canyon is no place to learn the basics.
    hints: Marksman Iden on the terraces teaches the sling first. Crier Toke in Hub
      Square can send you to her.
  - questRequired:
    - 50-end
    questExcluded:
    - 51-start
    - 51-end
    text: Iden's eye, then. When she thinks you are ready for the slot, she will send
      you down to me.
    hints: Talk to Marksman Iden on the Firing Range to take up the canyon lesson.
  - questRequired:
    - 51-start
    questExcluded:
    - 51-met
    - 51-end
    text: Iden sent you. Good enough for the slot. Here you learn the thing the raiders
      use on us.
    hints: Choose a line below to hear the canyon lesson.
  - questRequired:
    - 51-met
    questExcluded:
    - 51-shoot
    - 51-end
    text: Still at it. The scout holds the sounding floor, one chamber north of the
      Narrows. Stand in the Narrows and put a shot across the exit before it ever
      sees you.
    hints: Go north to the Echoing Narrows and type fire scout north. If it is dark,
      type cast glow first.
  - questRequired:
    - 51-shoot
    questExcluded:
    - 51-kill
    - 51-end
    text: You hit it, and now it is coming for you. Hold the Narrows and finish it
      when it arrives.
    hints: When the scout reaches you, type fire scout until it falls.
  - questRequired:
    - 51-kill
    questExcluded:
    - 51-end
    text: I heard it fall. Tell me how it went.
    hints: Choose Across the Canyon below to report.
  - questRequired:
    - 51-end
    questExcluded:
    - 52-start
    - 52-end
    text: You can reach across the canyon now and stand when the fight comes to you.
      That is everything but the last of it. The last of it is Skell, on the overlook.
      When your nerve is up, I will send you to him. Carry ammo.
    hints: Choose The Overlook below when you are ready for the final task.
  - questRequired:
    - 52-start
    questExcluded:
    - 52-kill
    - 52-end
    text: He is still up there. Up the chimney at the canyon's head, past the perch
      and the high stand, to the Overlook itself. Skell gives ground and shoots. Out-shoot
      him or close fast, and do not run dry. Out of ammo? Come to me before you climb.
    hints: From the Canyon Head, go up, then keep north to the Overlook and type fire
      skell. If it is dark, type cast glow first.
  - questRequired:
    - 52-kill
    questExcluded:
    - 52-end
    text: I saw him go off the high stand from down here. Tell me.
    hints: Choose The Overlook below to report.
  - questRequired:
    - 59-start
    questExcluded:
    - 59-shoot
    - 59-end
    text: The drill, then. The scout is back on the sounding floor. Shoot it from
      the Narrows, across the exit.
    hints: Go north to the Echoing Narrows and type fire scout north. If it is dark,
      type cast glow first.
  - questRequired:
    - 59-shoot
    questExcluded:
    - 59-kill
    - 59-end
    text: It is coming for you. Stand in the Narrows and finish it, then come back
      to me.
    hints: When the scout reaches you, type fire scout until it falls.
  - questRequired:
    - 59-kill
    questExcluded:
    - 59-end
    text: I heard it fall. Tell me how the drill went.
    hints: Choose Range Practice below to report.
  - questRequired:
    - 52-end
    questExcluded:
    - 96-start
    - 96-end
    text: Skell is off the overlook and the high ground is ours. I have watched that
      band tax this canyon for years and never had the hand to break it. You did.
      Toke will want to hear it, down in the town.
    hints: Choose Report to Toke (Archery) below to carry the word back to Toke.
  - questRequired:
    - 52-end
    text: Skell is off the overlook and the high ground is ours. I have watched that
      band tax this canyon for years and never had the hand to break it. You did.
      The coulee has nothing left to teach you. Past the rim is the wider country,
      and you are ready for it. Go well.
    hints: You have finished the archery trail. For more practice, choose Range
      Practice below.
topics:
- key: canyon_meet
  kind: info
  label: Iden sent me to learn the canyon shot.
  text: 'The canyon scout, then. It holds the sounding floor, the open stretch one
    chamber north of the Echoing Narrows. Here is the lesson: do not walk up and trade
    blows. Stand in the Narrows and shoot it across the exit with fire scout north.
    The shot will cross the gap and bite, and then it will come straight up the slot
    at you, because a shot thing always does. Stand your ground and finish it. That
    is the cross-canyon shot, and the price of it. Go.'
  hints: Go north to the Echoing Narrows and type fire scout north.
  questRequired:
  - 51-start
  questExcluded:
  - 51-met
  - 51-end
- key: canyon_turnin
  kind: handin
  quest: 51
  label: Across the Canyon
  playerLine: I shot the scout from the Narrows and held my ground.
  text: Reached across the slot and stood when it came. That is the canyon, learned.
    Take this hand crossbow and a case of bolts. Heavier than the sling and it
    hits like it. You will want it above. Skell is on the overlook, and he is the
    last of it. When your nerve is up, I will send you to the overlook.
  hints: Quest complete. Choose The Overlook below when you are ready for the final
    task.
  questRequired:
  - 51-kill
  questExcluded:
  - 51-end
  grantsQuest: 51-end
- key: overlook_offer
  kind: offer
  quest: 52
  label: The Overlook
  playerLine: Do you want Skell taken off the overlook?
  text: Skell, then. He holds the overlook above the canyon's head, and from up there
    he has made the whole slot a killing field. He fights the way you have just learned
    to, at range, on the move, giving ground a step at a time and making you pay
    for every yard you take back. Climb the chimney at the Canyon Head, fight up past
    his shooters, and bring him down on the Overlook, past the high stand. Carry all
    the ammo you can, because up there you will spend it. Then come back and tell me.
  hints: From the Canyon Head, go up, then keep north to the Overlook and type fire
    skell.
  breadcrumb: Learn the slot first. Skell holds the overlook above us, and he is the
    last of it.
  questRequired:
  - 51-end
  questExcluded:
  - 52-start
  - 52-end
  grantsQuest: 52-start
- key: overlook_turnin
  kind: handin
  quest: 52
  label: The Overlook
  playerLine: Skell is down, and the overlook is ours.
  text: I watched him go off the high stand from down here, and I did not believe it
    until he stopped moving. The band has lost the ground that let it rule this
    canyon. Here, the raider cache is yours by right.
  hints: Quest complete. Choose Report to Toke (Archery) below to carry the word back
    to Toke, or Range Practice for more drills.
  questRequired:
  - 52-kill
  questExcluded:
  - 52-end
  grantsQuest: 52-end
- key: range_loop_offer
  kind: offer
  quest: 59
  label: Range Practice
  playerLine: Can I run the canyon drill again?
  text: The eye drifts if you stop using it. Put a shot through the slot from the
    Narrows and finish what charges you. Same drill. Come back and tell me when the
    scout falls.
  hints: Go north to the Echoing Narrows and type fire scout north.
  questRequired:
  - 52-end
  questExcluded:
  - 59-start
  - 59-shoot
  - 59-end
  grantsQuest: 59-start
- key: range_loop_turnin
  kind: handin
  quest: 59
  label: Range Practice
  playerLine: I shot the scout from the Narrows and finished it.
  text: Good reach. Stood when it came. That is the whole drill, and it is done.
  hints: Quest complete. Bryn will have another run ready after a while.
  questRequired:
  - 59-kill
  questExcluded:
  - 59-end
  grantsQuest: 59-end
- key: report_archery_offer
  kind: offer
  quest: 96
  label: Report to Toke (Archery)
  playerLine: Should I tell Toke the overlook is taken?
  text: Tell him. He will have it shouted across the square before you finish the
    sentence, but he likes to hear it first. He is in Hub Square, south of the Awakening
    Pool.
  hints: From the Canyon Mouth, go south, west, west, down, south, south, down, south,
    west, west and south to Hub Square. Type talk toke.
  questRequired:
  - 52-end
  questExcluded:
  - 96-start
  - 96-end
  grantsQuest: 96-start
# Each replacement and its reminder never show together: itemExcluded and
# carriesItem on the same item are exact opposites.
- key: canyon_shot_supply
  kind: info
  label: I'm out of shot.
  text: Out of stones? Here, a pouch of shot. The scout will not wait while you go
    looking for more.
  hints: You received a pouch of shot.
  questRequired:
  - 51-start
  questExcluded:
  - 51-end
  itemExcluded: 30064
  givesItem: 30064
- key: canyon_shot_have
  kind: info
  label: Can I have more shot?
  text: You still have a pouch of shot on you. Spend it first. When the last stone
    is gone, ask me again.
  hints: You already carry a pouch of shot. Choose a line below to get more once
    it is empty.
  questRequired:
  - 51-start
  questExcluded:
  - 51-end
  carriesItem: 30064
- key: ammo_resupply
  kind: info
  label: I need bolts for the climb.
  text: A case of bolts. Take it, since a kiter makes a long fight, and an empty weapon
    on the overlook is a dead climber. Run dry up there and you will wish you had
    asked twice. Come back for more if you need it.
  hints: You received a case of bolts. Carry plenty against Skell. Choose a line
    below to get more ammo if you run dry.
  questRequired:
  - 51-end
  questExcluded:
  - 52-end
  itemExcluded: 30063
  givesItem: 30063
- key: ammo_have
  kind: info
  label: Can I have more bolts?
  text: You still have bolts in that case. Spend them first. When the case is empty,
    come and ask me again, before you climb.
  hints: You already carry a case of bolts. Choose a line below to get more once
    it is empty.
  questRequired:
  - 51-end
  questExcluded:
  - 52-end
  carriesItem: 30063
- key: cross_help
  kind: info
  label: How do I shoot into the next room?
  text: 'Two things the canyon teaches. The cross-shot: stand in one chamber and type
    fire, the target, and the direction (fire scout north) to shoot through the
    exit into the next room. And kiting: a foe that shoots and backs away, opening
    the range as you close. Beat it by out-shooting it, or by closing faster than
    it can retreat. Mind your ammo. The weapon feeds itself, but the pouch does not.'
  hints: 'Cross-room shot: fire <target> <direction>. Against a kiter, out-shoot it
    or close fast, and keep ammo on you.'
  marked: true
- key: coach_51_start
  kind: info
  label: Where is the canyon scout?
  text: Not yet. The scout holds the sounding floor, one chamber north of the Narrows.
    Shoot it from the Narrows, across the exit.
  hints: Go north to the Echoing Narrows and type fire scout north.
  questRequired:
  - 51-met
  questExcluded:
  - 51-shoot
  - 51-end
- key: coach_51_shoot
  kind: info
  label: I hit the scout. What now?
  text: Not yet. You hit it, and it is coming for you. Hold the Narrows and finish
    it.
  hints: When the scout reaches you, type fire scout until it falls.
  questRequired:
  - 51-shoot
  questExcluded:
  - 51-kill
  - 51-end
- key: coach_52_start
  kind: info
  label: Where is Skell?
  text: Not yet. He is still up there. Up the chimney at the canyon's head, past the
    perch and the high stand, to the Overlook itself.
  hints: From the Canyon Head, go up, then keep north to the Overlook and type fire
    skell.
  questRequired:
  - 52-start
  questExcluded:
  - 52-kill
  - 52-end
- key: coach_59_start
  kind: info
  label: Where do I run the drill?
  text: Same drill. The scout is on the sounding floor. Shoot it from the Narrows,
    across the exit.
  hints: Go north to the Echoing Narrows and type fire scout north.
  questRequired:
  - 59-start
  questExcluded:
  - 59-shoot
  - 59-end
- key: coach_59_shoot
  kind: info
  label: The scout is charging me. What now?
  text: It is coming for you. Finish it, then come back to me.
  hints: When the scout reaches you, type fire scout until it falls.
  questRequired:
  - 59-shoot
  questExcluded:
  - 59-kill
  - 59-end
- key: outline
  kind: info
  label: Why does your outline double?
  text: The Awakening left me a half-step behind myself. Turn quick and my outline
    doubles, a second me still catching up. It has fooled more than one raider's aim.
    I do not complain.
```

- [ ] **Step 8: Add Iden's canyon offer**

In `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml`, replace:
```yaml
- key: shoot_lesson
  kind: info
  label: Toke sent me to learn to shoot.
```
with:
```yaml
- key: canyon_offer
  kind: offer
  quest: 51
  label: Across the Canyon
  playerLine: Will you send me on to the canyon?
  text: Bryn holds the slot below the raider bluffs, east along the terraces and north.
    He teaches the trick the raiders use on us, the shot that crosses a whole chamber
    before the target knows you are there. Go and see him.
  hints: From the Firing Range, go east twice to the High Terrace, then north to the
    Canyon Mouth. Type talk bryn.
  breadcrumb: Finish the terraces with me first. The canyon is no place to learn the
    basics.
  questRequired:
  - 50-end
  questExcluded:
  - 51-start
  - 51-end
  grantsQuest: 51-start
- key: shoot_lesson
  kind: info
  label: Toke sent me to learn to shoot.
```

- [ ] **Step 9: Toke's archery hand-in**

Append to the end of `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` (after `report_lore_in`):

```yaml
- key: report_archery_in
  kind: handin
  quest: 96
  label: Report to Toke (Archery)
  playerLine: Skell is off the overlook, and the canyon is ours.
  text: Off the overlook! The low mouth has been aching to shout that for years. Stand
    back a pace, friend. This one carries.
  hints: Quest complete. Choose a line below to take up another trail.
  questRequired:
  - 96-start
  questExcluded:
  - 96-end
  grantsQuest: 96-end
  moodChange: friendly
```

- [ ] **Step 10: Manifests and backlog**

`quests/step-ids.txt`: after `51-kill` add `51-met`; after `52-end` add `52-kill`; after `59-end` add `59-kill`; add `96-end` and `96-start` after `95-start`.

`dialogue/topic-keys.txt`: add `9161.canyon_offer`, `9162.canyon_meet`, `9162.overlook_turnin`, `9162.range_loop_turnin`, `9162.report_archery_offer`, `9106.report_archery_in` in sorted position; under `# retired` add `9162.canyon_offer`.

`quests/loop-backlog.txt`: delete exactly these six lines:
```
chain 50 mob 9162 topic canyon_offer
chain 52 mob 9162 topic range_loop_offer
end 52 quest 52 trigger mob_death mob=9169 grants=52-end
end 52 step end
end 59 quest 59 trigger mob_death mob=9165 grants=59-end
end 59 step end
```

- [ ] **Step 11: Run and see it pass**

```bash
go test ./internal/questengine/ 2>&1 | tail -3
go test ./internal/guards/ 2>&1 | tail -3
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators' . 2>&1 | tail -5
```
Expected: all `ok`; 6 fewer known findings.

- [ ] **Step 12: Commit**

```bash
git add _datafiles/world/dogmud/quests/51-across_the_canyon.yaml _datafiles/world/dogmud/quests/52-the_overlook.yaml _datafiles/world/dogmud/quests/59-range_practice.yaml _datafiles/world/dogmud/quests/96-report_to_toke__archery_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9162.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/newbie_ranged_steps_test.go internal/questengine/coulee_bridges_test.go
git commit -F - <<'EOF'
feat(coulee): archery line giver to giver, bridge 96 (#375 p7)

Iden offers Across the Canyon (51), which opens with Bryn's lesson. The
Overlook (52) and Range Practice (59) end at hand-ins on Bryn. Bryn offers
Report to Toke (Archery) (96); Toke takes it. Clears six backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Quest 76, the interim disc home (owner option A)

**Executor:** Opus (touches three zones and a recovery path).

**Files:**
- Modify: `_datafiles/world/dogmud/quests/76-the_disc.yaml` (full replacement)
- Modify: `_datafiles/world/dogmud/dialogue/greenford/9516.yaml` (targeted)
- Modify: `_datafiles/world/dogmud/dialogue/eastern_highlands/9553.yaml` (one hint)
- Modify: `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Create: `internal/questengine/quest76_disc_steps_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/questengine/quest76_disc_steps_test.go`:

```go
package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Quest 76, The Disc, interim home (#375 phase 7, owner option A, #625): the
// Inert Disc (40167) is found by a real search in Maren's Cottage (4023); the
// same search reads the journal; casting Chrysalis Glow there wakes the disc
// (40167 for 40168); Brennan (9516) takes the hand-in. The Reliquary keeps no
// disc. A finished player who lost the woken disc can search and wake another.
func TestQuest76TheDisc_InterimHome(t *testing.T) {
	const cottage, reliquary, inert, woken = 4023, 5343, 40167, 40168
	q := loadShippedQuest(t, "76-the_disc.yaml", 76)
	require.Equal(t, []string{"start", "journal", "attune", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; they stay")

	brennan := loadShippedDialogue(t, "greenford", "9516.yaml")
	in := brennan.FindTopic("disc_in")
	require.NotNil(t, in, "Brennan takes the hand-in")
	require.Equal(t, dialogue.KindHandIn, in.Kind)
	require.Equal(t, []string{"76-attune"}, in.QuestRequired)
	require.Equal(t, "76-end", in.GrantsQuest)

	search := func(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }
	glow := func(room int) EventDetails { return EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: room} }

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", glow(cottage))
		require.Equal(t, "start", w.p.current, "the hearth waits for the journal")
		w.notify("command", search(reliquary))
		require.Equal(t, "start", w.p.current, "the Reliquary holds no disc any more")
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Equal(t, []int{inert}, w.ctx.givenItems, "the search turns up the disc")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: cottage})
		require.Equal(t, "journal", w.p.current, "only the glow wakes it")
		w.notify("command", glow(cottage))
		require.Equal(t, "attune", w.p.current)
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems)
		require.Contains(t, w.ctx.consumedItems, inert)
		require.False(t, w.p.HasQuest("76-end"), "Brennan takes the hand-in")
	})

	t.Run("a player who already carries a disc gets no second one", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.p.items[inert] = true
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Empty(t, w.ctx.givenItems)
	})

	t.Run("a finished player who lost the woken disc can wake another", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.notify("command", search(cottage))
		require.Equal(t, []int{inert}, w.ctx.givenItems)
		w.p.items[inert] = true
		w.notify("command", glow(cottage))
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems)
	})

	t.Run("a finished player who still holds the woken disc finds nothing more", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.p.items[woken] = true
		w.notify("command", search(cottage))
		require.Empty(t, w.ctx.givenItems)
	})

	t.Run("no step listens for a typed noun", func(t *testing.T) {
		for _, tr := range q.Triggers {
			if tr.Event != "room_interact" {
				continue
			}
			require.Equal(t, "stone", tr.Noun, "only Maren's letter, which is no step, still listens for a noun")
			for _, a := range tr.Actions {
				require.Empty(t, a.Grant, "the letter trigger grants no step")
			}
		}
	})

	t.Run("hints fit the cap and name one command", func(t *testing.T) {
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			require.LessOrEqual(t, len([]rune(squash(s.Hint))), 200, "step %s hint is over the cap", s.Id)
			require.Equal(t, 1, typedCount(s.Hint), "step %s hint names one command: %q", s.Id, s.Hint)
		}
	})

	t.Run("nothing still sends the player to the Reliquary for the disc", func(t *testing.T) {
		for _, f := range [][]string{{"greenford", "9516.yaml"}, {"eastern_highlands", "9553.yaml"}} {
			raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", f[0], f[1])))
			require.NotContains(t, raw, "reliquary", "%s/%s", f[0], f[1])
			require.NotContains(t, raw, "look disc", "%s/%s", f[0], f[1])
		}
		require.NotContains(t, strings.ToLower(readQuestFile(t, "76-the_disc.yaml")), "reliquary")
	})
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/questengine/ -run TestQuest76
```
Expected: FAIL (no `disc_in`; the search gives nothing; Brennan's file names the Reliquary).

- [ ] **Step 3: Replace quest 76**

Write `_datafiles/world/dogmud/quests/76-the_disc.yaml`:

```yaml
# Quest 76, The Disc (Greenford and Ashwick). Interim home, #375 phase 7,
# owner option A (#625 stays open for the full endgame redesign).
#
# Token flow:
#   76-start:   granted by Brennan the Surveyor's disc_quest or disc_tale
#               offer (dialogue/greenford/9516.yaml), after 75-end.
#   76-journal: command trigger on `search` in Maren's Cottage (4023, Ashwick).
#               The search turns up the journal and, pressed in its pages, the
#               Inert Disc (40167), given by the trigger just above it unless
#               the player already carries one. The journal text is the reading.
#   76-attune:  command trigger on `cast chrysalis-glow` (cast glow) in 4023:
#               the method wakes the disc with light at the marked hearth.
#               Consumes 40167, gives the Attuned Disc (40168).
#   76-end:     granted by Brennan's disc_in hand-in (needs 76-attune).
#
# Real actions only: no step listens for a typed noun. The Reliquary (5343)
# no longer holds a disc; quest 49 keeps one under its floor as story only.
#
# Recovery: a player who woke a disc and lost it (the Threshold Keeper 9553
# opens the way only for 40168) searches the cottage for another grey disc
# and casts glow there again. Those two triggers grant nothing.
#
# While any search trigger here would fire for the player, the search does
# not first say it found nothing (actions.SearchOptions.QuestStep).
questid: 76
name: The Disc
description: >-
  The inert disc is a key that has never been woken. Brennan knows of the
  man who studied how, one who went east from Ashwick years ago and did
  not come back. His research waits in the cottage he left behind.
secret: false
steps:
  - id: start
    destination:
      room: 4023
    description: >-
      Brennan believes the disc is a key that must be woken, and that the man
      who knew how left his work, and perhaps a disc of his own, in an Ashwick
      cottage. From Brennan's Office, go north, east and north to the
      University Gate, then down the University Stair and north through the
      town to the Market Square, down the Town Stair, and north to the
      Greenford Gate. Then go north over the bridge and west along the road to
      the Confluence. In the city, cross the Brenn Bridge west to Tri-Cross
      Square and go north through the quays to the River Gate. Then take the
      River Road north, through Amber Valley and up the South Road to the
      Ashwick Crossroads. From the crossroads, go east five times to the East
      Edge of the village, then south into the cottage.
    hint: >-
      Maren's Cottage is south of the East Edge of Ashwick, a long road north
      of Greenford. The route is in the quest log. In the cottage, type
      search.
  - id: journal
    destination:
      room: 4023
    description: >-
      You searched the cottage and read the father's journal: a life spent
      chasing a buried thing in the east, and the method he worked out for
      waking the disc with light at the marked hearth.
    hint: >-
      His method wakes the disc with light, at the marked hearth in the
      cottage. Type cast glow there.
  - id: attune
    destination:
      mob: 9516
    description: >-
      You followed his method at the hearth, and the disc woke. The lines
      hold warmth now, and the mark keeps the light. Brennan will want to
      hear it.
    hint: >-
      Go back to Brennan's Office in Greenford and type talk brennan.
  - id: end
    description: >-
      You carry the woken disc, a key for the door in the east, and the last
      of a lost man's work, done at last. Reth's route ends at that door, in a
      buried face in the Eastern Highlands.
rewards:
  actor: >-
    Brennan turns the woken disc to the light and watches the lines keep it.
    "A father's work, finished by a stranger, in the cottage he left to chase
    it." He gives it back. What it opens waits east, in the broken country,
    behind a door in the buried face.
  observer: "Brennan turns a grey disc to the light and hands it back."
  gold: 0
triggers:
  # Maren's letter (item 40041), for any player, on or off this quest: the
  # loose stone by the hearth hides it. Moved here from the room's behaviour
  # tree, which answered push, move, pull, shift and press (#375 phase 3a,
  # owner ruling 5: look and search are the only interaction verbs). Being a
  # `verb: search` trigger, `search stone` runs the search (and its cooldown)
  # first, and a plain search can find it at the room difficulty. Carrying
  # the letter, the stone holds nothing more, and the search says so. Story
  # first, so a full pack hears what is there before the refusal. It grants
  # no step.
  - event: room_interact
    verb: search
    room: 4023
    noun: stone
    conditions:
      missing_item: 40041
    actions:
      - actor: >-
          You work the loose stone free of its neighbors. It grinds aside and
          shows a small cavity in the wall behind it. Inside, a folded letter
          lies against the back of the hollow, hidden there on purpose.
      - give_item: 40041

  # RECOVERY, search: a player who woke a disc and lost it finds another.
  # Grants nothing. Before the journal trigger, so it is judged first.
  - event: command
    command: search
    room: 4023
    conditions:
      has: ["76-attune"]
      missing_item: 40168
    actions:
      - give_item: 40167
      - actor: >-
          Between the last pages of the journal lies another grey disc, cool
          and dead in the hand, the twin of the one you woke. You take it.

  # DISC: the search turns up the disc, unless the player carries one.
  # Grants nothing; the trigger below grants the step.
  - event: command
    command: search
    room: 4023
    conditions:
      has: ["76-start"]
      missing: ["76-attune"]
      missing_item: 40167
    actions:
      - give_item: 40167
      - actor: >-
          Pressed between the last pages of the journal on the table lies a
          disc of pale grey metal: cool, too smooth, heavier than its size.
          Fine raised lines thread across its face to a ring struck through
          with a line. You take it.
      - observer: "{actor} lifts something grey from between the pages of a journal and pockets it."

  # JOURNAL: the search reads the man's journal.
  - event: command
    command: search
    room: 4023
    conditions:
      has: ["76-start"]
      missing: ["76-journal"]
    actions:
      - grant: "76-journal"
      - actor: |
          You sit at the cold table and read the man's life backward.
          Years of it: a hand that starts careful and turns driven, the
          same questions put a hundred ways to one buried thing far to
          the east. Something made, he is certain, and old past any
          reckoning, and wrong in a way he never quite sets down in
          words. He calls it only the thing. The ring struck through with
          a line is inked in every margin, worried at like a sore tooth.
          Then, near the end, the part you came for without knowing it:
          a method, worked out slow and sure across many pages, for
          waking the grey disc that is its key. Light, held over it at
          the marked hearth. The last line is not about the thing at
          all. It is a name, set down once and gently, as if to keep one
          soft thing safe. Maren. He went east to finish this. He did not
          come back to her. At the marked hearth, type cast glow.
      - observer: "{actor} reads a long while at the cottage table."

  # RECOVERY, wake: a finished player with a grey disc and no woken one.
  # Grants nothing. Before the attune trigger, so it is judged first.
  - event: command
    command: cast
    noun: chrysalis-glow
    room: 4023
    conditions:
      has: ["76-attune"]
      has_item: 40167
      missing_item: 40168
    actions:
      - consume_item: 40167
      - give_item: 40168
      - actor: >-
          You set the grey disc into the cut center of the hearthstone and
          hold the glow over it, as his method says. The lines warm, and the
          mark at the center wakes.

  # ATTUNE: light at the marked hearth wakes the disc.
  - event: command
    command: cast
    noun: chrysalis-glow
    room: 4023
    conditions:
      has: ["76-journal"]
      missing: ["76-attune"]
    actions:
      - grant: "76-attune"
      - consume_item: 40167
      - give_item: 40168
      - actor: |
          You do as the journal says. You set the disc into the cut
          center of the hearthstone, where the marks draw inward to a
          single point, and you hold the soft glow over it, patient and
          exact, the way a driven man learned alone and wrote down so a
          stranger might one day do this right. For a long moment,
          nothing. Then the grey face warms under your fingers, not with
          your warmth but its own. The fine raised lines drink the light
          and keep it, holding a glow a half-breath past when it should
          fade, and the ring-struck mark at the center wakes and steadies,
          sure of itself now, sure of where it belongs. The disc is a key,
          and it knows its door: east, in the broken country, in the
          buried face he gave his years to. His work, done at last, by a
          stranger's hand in the cottage he left to chase it. And a name
          comes to you off the last page, unbidden. Maren. He kept her,
          all the way to the end of himself. You hope, wherever she is,
          she never learns how far he went. Brennan will want to hear of
          this. Go back to his office in Greenford and type talk brennan.
      - observer: "{actor} holds a soft glow over the marked hearth, and the air goes briefly strange."
```

- [ ] **Step 4: Edit Brennan's dialogue (9516)**

Edit `_datafiles/world/dogmud/dialogue/greenford/9516.yaml`.

Edit A, replace (the four 76 variants, lines 26 to 67 on master):
```yaml
  - questRequired:
    - 76-attune
    itemExcluded: 40168
    text: You woke it, and now it is gone from your hand. The Reliquary floor gave
      up one grey disc. It may give another, and the hearth knows the method.
    hints: A grey disc still lies in the cracked floor of the old Reliquary at
      Pothole Coulee, behind the old shrine north of the stone circle. The hearth in
      the Ashwick cottage will wake it. At the Reliquary, type look disc.
  - questRequired:
    - 76-attune
    text: You woke it. I can see the light it keeps along its lines. Then it is a
      key, and somewhere east there is a door that fits it. Reth's route will take
      you there.
    hints: Reth's route runs north from Hartcharn to Kingsbarrow, then east over the
      Cascade Pass Road into the Eastern Highlands, to a door in a buried face.
  - questRequired:
    - 76-journal
    questExcluded:
    - 76-attune
    text: You read it. Then you know the method, and the hearth is where he meant
      it done. Take the disc there.
    hints: If you do not carry the disc yet, it lies in the cracked floor of the old
      Reliquary at Pothole Coulee, behind the old shrine north of the stone circle.
      In the cottage, type look hearth marks.
  - questRequired:
    - 76-start
    questExcluded:
    - 76-journal
    - 76-attune
    text: The cottage in Ashwick. Read what he left behind. You will need the disc
      there.
    hints: Ashwick is a long road from here. From Brennan's Office, go north, east
      and north to the University Gate, then down the University Stair and north
      through the town to the Market Square, down the Town Stair, and north to the
      Greenford Gate. Then go north over the bridge and west along the road to the
      Confluence. In the city, cross the Brenn Bridge west to Tri-Cross Square and
      go north through the quays to the River Gate. Then take the River Road north,
      through Amber Valley and up the South Road to the Ashwick Crossroads. From
      there go east five times to the East Edge of the village, then south into the
      cottage. If you do not carry the disc yet, it lies in the cracked floor of the
      old Reliquary at Pothole Coulee, behind the old shrine north of the stone
      circle. Type look journal.
```
with:
```yaml
  - questRequired:
    - 76-attune
    itemExcluded: 40168
    text: You woke it, and now it is gone from your hand. The cottage gave up one grey
      disc. It may give another, and the hearth knows the method.
    hints: In Maren's Cottage in Ashwick, type search, then cast glow there to wake
      the disc again.
  - questRequired:
    - 76-attune
    questExcluded:
    - 76-end
    text: You have the look of someone who has watched a thing wake. Tell me.
    hints: Choose The Disc below to report.
  - questRequired:
    - 76-attune
    text: You woke it. I can see the light it keeps along its lines. Then it is a
      key, and somewhere east there is a door that fits it. Reth's route will take
      you there.
    hints: Reth's route runs north from Hartcharn to Kingsbarrow, then east over the
      Cascade Pass Road into the Eastern Highlands, to a door in a buried face.
  - questRequired:
    - 76-journal
    questExcluded:
    - 76-attune
    text: You read it. Then you know the method, and the hearth is where he meant
      it done.
    hints: At the marked hearth in Maren's Cottage, type cast glow.
  - questRequired:
    - 76-start
    questExcluded:
    - 76-journal
    - 76-attune
    text: The cottage in Ashwick. Search it, and read what he left behind.
    hints: Ashwick is a long road from here. From Brennan's Office, go north, east
      and north to the University Gate, then down the University Stair and north
      through the town to the Market Square, down the Town Stair, and north to the
      Greenford Gate. Then go north over the bridge and west along the road to the
      Confluence. In the city, cross the Brenn Bridge west to Tri-Cross Square and
      go north through the quays to the River Gate. Then take the River Road north,
      through Amber Valley and up the South Road to the Ashwick Crossroads. From
      there go east five times to the East Edge of the village, then south into the
      cottage. Type search.
```

Edit B, replace:
```yaml
  hints: Accept, then take the disc to the abandoned cottage in Ashwick.
```
with:
```yaml
  hints: Accept, then take the disc to the abandoned cottage in Ashwick and search
    it.
```

Edit C, replace the whole `disc_tale` topic's `text` and `hints` (from `  text: |` under `key: disc_tale` through its `hints:` block, which ends `to the abandoned cottage in Ashwick. At the Reliquary, type look disc.`) with:
```yaml
  text: |
    The mark on my wall, the nested rings. One margin, older than
    the rest, draws it with a line struck through the ring, and a
    note beside it: the same mark, cut in grey metal, is a key. A
    dead one, until someone wakes it, and I do not know how. A man
    did. He kept a cottage in Ashwick and gave his whole life to
    that thing in the east. They say he had a disc of his own, and
    studied how to wake it, then went east himself and never came
    back. His work should still be in the house he left, and the
    disc with it, if no one has been there before you. Will you go
    to Ashwick, search the abandoned cottage, and read what he left
    behind?
  hints: Accept, then go to the abandoned cottage in Ashwick and search it.
```

Edit D, insert after the `disc_tale` topic (before `- key: knower`):
```yaml
- key: disc_in
  kind: handin
  quest: 76
  label: The Disc
  playerLine: I woke the disc at the cottage hearth.
  text: |
    Let me see it. Yes. The lines hold the light now, the way the
    margins always said they would. A key, then, and somewhere
    east there is a door that fits it. Reth's route will take you
    there. Keep it close, and keep it quiet.
  hints: Quest complete. Reth's route runs north from Hartcharn to Kingsbarrow, then
    east over the Cascade Pass Road into the Eastern Highlands, to a door in a buried
    face.
  questRequired:
  - 76-attune
  questExcluded:
  - 76-end
  grantsQuest: 76-end
```

- [ ] **Step 5: The Threshold Keeper's hint**

In `_datafiles/world/dogmud/dialogue/eastern_highlands/9553.yaml`, replace:
```yaml
  hints: If you woke a disc before and lost it, a grey disc still lies in the
    cracked floor of the old Reliquary at Pothole Coulee, and the hearth in Ashwick
    where you woke the first will wake it.
```
with:
```yaml
  hints: If you woke a disc before and lost it, search Maren's Cottage in Ashwick for
    another grey disc, then cast glow there to wake it.
```

- [ ] **Step 6: Manifests and backlog**

`dialogue/topic-keys.txt`: add `9516.disc_in` in the `9516.` block (sorted, after `9516.disc_quest`... place it before `9516.disc_quest` alphabetically: `9516.disc_in` sorts before `9516.disc_quest`). No step ids change.

`quests/loop-backlog.txt`: delete exactly these eight lines:
```
end 76 quest 76 trigger room_interact room=4023 noun=hearth-marks grants=76-attune,76-end
end 76 step end
hint-length 76 step journal
hint-length 76 step start
one-action 76 quest 76 trigger room_interact room=4023 noun=hearth-marks grants=76-attune,76-end:cond
one-action 76 quest 76 trigger room_interact room=4023 noun=hearth-marks grants=76-attune,76-end:double
one-action 76 quest 76 trigger room_interact room=4023 noun=hearth-marks grants=76-attune,76-end:prev
walk-destination 76 step attune
```

- [ ] **Step 7: Run and see it pass**

```bash
go test ./internal/questengine/ 2>&1 | tail -3
go test ./internal/guards/ 2>&1 | tail -3
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_ServerBootsCleanWithRealData' . 2>&1 | tail -5
```
Expected: all `ok` (including `TestMarenLetter_SearchTheStone`, which still finds the letter trigger); 8 fewer known findings, 0 new, 0 stale.

- [ ] **Step 8: Commit**

```bash
git add _datafiles/world/dogmud/quests/76-the_disc.yaml _datafiles/world/dogmud/dialogue/greenford/9516.yaml _datafiles/world/dogmud/dialogue/eastern_highlands/9553.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/questengine/quest76_disc_steps_test.go
git commit -F - <<'EOF'
feat(quests): quest 76 interim disc home in Maren's Cottage (#375 p7, #625)

The Inert Disc is found by a real search in Maren's Cottage, which also
reads the journal; casting glow at the hearth wakes it; Brennan takes the
hand-in. The Reliquary keeps no disc. A player who lost the woken disc can
search and wake another. Clears quest 76's eight backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: Coulee rooms for #689 and #690

**Executor:** Sonnet.

**Files:**
- Modify: `_datafiles/world/dogmud/rooms/pothole_coulee/5213.yaml` (notice board)
- Modify: `_datafiles/world/dogmud/rooms/pothole_coulee/5214.yaml` (desk noun, rubbings note)

- [ ] **Step 1: Write the failing check**

Append to `internal/questengine/newbie_lore_steps_test.go`:

```go
// #689 and #690: the School Hall notice board names commands a player can
// type (talk, help, set linewidth), and the Cleric's Study has the desk its
// description names, and its rubbings point toward the shrine.
func TestCouleeRooms_SignpostsAndNouns(t *testing.T) {
	var room struct {
		Nouns map[string]string `yaml:"nouns"`
	}
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", "5213.yaml"), &room))
	board := squash(room.Nouns["notice board"])
	require.Contains(t, board, "type talk")
	require.Contains(t, board, "type help")
	require.Contains(t, board, "type set linewidth")
	require.NotContains(t, board, "put the question plainly to the world")

	room.Nouns = nil
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", "5214.yaml"), &room))
	require.NotEmpty(t, room.Nouns["desk"], "the Cleric's Study names a desk, so look desk must find it")
	require.Contains(t, squash(room.Nouns["rubbings"]), "the old shrine")
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/questengine/ -run TestCouleeRooms_SignpostsAndNouns
```
Expected: FAIL (`"type talk"` not found).

- [ ] **Step 3: The notice board**

In `_datafiles/world/dogmud/rooms/pothole_coulee/5213.yaml`, replace the whole `notice board:` value (from `    A broad notice board fills the wall beside the door, shingled deep` through `    again.`) with:
```yaml
    A broad notice board fills the wall beside the door, shingled deep
    with pinned notes, lesson-lists, and curling hand-bills. Pride of
    place goes to a single weathered card lettered in the schoolmaster's
    careful hand. It says that no question is too small or too strange.
    To learn how a thing is done, type help and the name of it, and the
    world will answer what it knows. To learn about a place or a story,
    find someone who keeps it and type talk and their name. Elder Wenna
    at the farmstead up the north shore, it adds, knows where the old
    stones stand. Below it, smaller and kindlier, a line for tired eyes.
    Should the written word ever sit crooked on the page, spilling past
    its edges or crowding itself too thin to read, type set linewidth
    and a number, and it will lay itself out clean and square again.
```

- [ ] **Step 4: The Cleric's Study**

In `_datafiles/world/dogmud/rooms/pothole_coulee/5214.yaml`, replace:
```yaml
    explained none of them. They simply hang there, patient and faintly
    unsettling, like a question someone left half-asked on purpose.
```
with:
```yaml
    explained none of them. They simply hang there, patient and faintly
    unsettling, like a question someone left half-asked on purpose.
    Beneath the last of them a note is pinned in the cleric's cramped
    hand. It reads only this. The old shrine, past the standing stones.
  desk:
    A broad oak desk lost under its own drifts of paper, splayed books
    holding a dozen places at once, a cracked inkwell and a sheaf of
    half-finished notes in the cleric's cramped hand. A few of the notes
    copy the rings from the rubbings on the wall, worried over and
    crossed out, as though he keeps trying to finish a drawing that will
    not close.
```

- [ ] **Step 5: Run and see it pass**

```bash
go test ./internal/questengine/ -run TestCouleeRooms_SignpostsAndNouns
go test ./internal/guards/ -run 'TestPlayerProseHasNoDashes|TestHiddenNoun'
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ServerBootsCleanWithRealData' .
```
Expected: all `ok`.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/rooms/pothole_coulee/5213.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5214.yaml internal/questengine/newbie_lore_steps_test.go
git commit -F - <<'EOF'
fix(coulee): notice board names real commands; Cleric's Study desk (#689, #690)

The School Hall card names talk, help and set linewidth, and points at
Elder Wenna for the old stones. The Cleric's Study gains the desk its
description names, and its rubbings carry a note toward the shrine.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: Toke's three offer texts (conditional)

**Executor:** Opus.

**Controller ruling (2026-10-30):** each lines plan owns Toke's offer `text` and `hints` for its
own lines' quest 1. If Step 1 finds them stale, apply the replacement below directly (Step 2 no
longer waits for approval), then verify the route against room exits before committing.

**Files:** possibly `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` (`text` and `hints` of `send_folding`, `send_folk`, `send_range` only).

- [ ] **Step 1: Decide whether anything is stale**

Use Task 1 Step 3's notes. If plan 2 already rewrote all three offers so their `hints` send the player to talk to Grieve, Wenna and Iden (no `type cast spike mote`, no `Type help.`, no `type equip sling` before the giver), mark this task done and skip to Task 13.

- [ ] **Step 2: If stale, apply this replacement**

For `send_folding` (keep every other field):
```yaml
  text: Adept Grieve, in the old observatory past the reed jetty. Quiet, and stranger
    than he lets on, but he knows the folding art. From this square go west, then
    north three times up the shore to the Stargazer Cut, and climb up twice to the
    Observatory Hall. Talk to him there.
  hints: From Hub Square go west, north, north, north, up, up to the Observatory Hall.
    Type talk grieve.
```
For `send_folk`:
```yaml
  text: Elder Wenna, at the farmstead up the north shore. She teaches the ways of the
    folk, and her first lesson is a short one. From this square go east, then north
    six times, up the east shore and the cartway to the Commons Yard. Talk to her
    there.
  hints: From Hub Square go east, then north six times to the Commons Yard. Type talk
    wenna.
```
For `send_range`:
```yaml
  text: Marksman Iden, on the terraces up the bluff. She teaches the long shot. I have
    a sling for you to take along. From this square go north to the pool, east twice
    to the Mending Hut, then north to the Bluff Steps. Climb up, go north twice, and
    up once more to the Firing Range. Talk to her there.
  hints: Accept, and Toke hands you a sling. From Hub Square go north, east, east, north,
    up, north, north, up to the Firing Range. Type talk iden.
```

- [ ] **Step 3: If approved, apply and commit**

Apply the three replacements with Edit (each replaces only that topic's `text:` and `hints:` blocks), then:
```bash
go test ./internal/questengine/ ./internal/dialogue/ 2>&1 | tail -3
DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_ShippedDialoguePassesTheValidators' .
git add _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml
git commit -F - <<'EOF'
fix(coulee): Toke's casting, lore and archery offers send you to the giver (#375 p7)

Quests 44, 47 and 50 now begin with a talk to Grieve, Wenna and Iden, so
Toke's offers point there instead of at the old first action.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: The Coulee dash guard

**Executor:** Sonnet.

**Files:**
- Create: `internal/guards/coulee_dash_guard_test.go`
- Modify (only if still dashed): `_datafiles/world/dogmud/conversations/pairs/9103_9105.yaml`, `9106_9107.yaml`

- [ ] **Step 1: Write the guard and its can-fail sibling**

Create `internal/guards/coulee_dash_guard_test.go`:

```go
package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// The Pothole Coulee dash guard (#375 phase 7, first zone pass). House style
// (dogmud-player-copy) bars em dashes, en dashes and a spaced double hyphen
// standing in for one. The global prose guard (prose_dash_guard_test.go)
// still exempts quests, dialogue and conversations until phase 7 closes, so
// this guard holds the files the Coulee pass cleared: the pass's quests,
// every dialogue file of the Coulee and the Newcomer Antechamber, and every
// conversation pair with a Coulee NPC in it.
//
// It reads the parsed YAML, not lines: every scalar value is checked and no
// comment is, whole-line or trailing (a comment after a value on the same
// line is still a comment, which a line scan cannot tell from prose). Keys
// are not checked; they are never player text.

// couleeDashDoubleHyphen is a double hyphen standing alone: between spaces,
// at the start or at the end of a value. A folded scalar joins its lines
// with a space, so a line that ended in " --" reads " -- " here. "--help"
// and "well-made" do not match.
var couleeDashDoubleHyphen = regexp.MustCompile(`(^|\s)--(\s|$)`)

// couleeDashQuest reports whether a quest id is in the Coulee pass: 28 (the
// antechamber), 30 to 59, 76, 88, 89 and the bridge and Shadows quests 90 to
// 101. Quest 29 is retired.
func couleeDashQuest(id int) bool {
	return id == 28 || (id >= 30 && id <= 59) || id == 76 || id == 88 || id == 89 || (id >= 90 && id <= 101)
}

// couleeDashNPC reports whether a mob id is a Coulee or antechamber NPC: 9100
// to 9169, Dewey (9491) and the four crafters 9605 to 9608.
func couleeDashNPC(id int) bool {
	return (id >= 9100 && id <= 9169) || id == 9491 || (id >= 9605 && id <= 9608)
}

// couleeDashDialogueDirs are the dialogue folders checked whole.
var couleeDashDialogueDirs = []string{"pothole_coulee", "newcomer_antechamber"}

// couleeDashFiles lists the files the guard reads, by kind.
func couleeDashFiles(t *testing.T, world string) (quests, dialogue, pairs []string) {
	t.Helper()
	qs, err := filepath.Glob(filepath.Join(world, "quests", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range qs {
		idText, _, ok := strings.Cut(filepath.Base(p), "-")
		if id, err := strconv.Atoi(idText); ok && err == nil && couleeDashQuest(id) {
			quests = append(quests, p)
		}
	}
	for _, d := range couleeDashDialogueDirs {
		ds, err := filepath.Glob(filepath.Join(world, "dialogue", d, "*.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		dialogue = append(dialogue, ds...)
	}
	ps, err := filepath.Glob(filepath.Join(world, "conversations", "pairs", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		for _, part := range strings.Split(strings.TrimSuffix(filepath.Base(p), ".yaml"), "_") {
			if id, err := strconv.Atoi(part); err == nil && couleeDashNPC(id) {
				pairs = append(pairs, p)
				break
			}
		}
	}
	return quests, dialogue, pairs
}

// couleeDashFindings parses one YAML file and returns one line per scalar
// value that carries a dash, with the number of scalars read.
func couleeDashFindings(name string, data []byte) ([]string, int, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", name, err)
	}
	var out []string
	scalars := 0
	var walk func(n *yaml.Node, path string)
	walk = func(n *yaml.Node, path string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], path+"."+n.Content[i].Value)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				walk(c, fmt.Sprintf("%s[%d]", path, i))
			}
		case yaml.ScalarNode:
			scalars++
			if emOrEnDash.MatchString(n.Value) || couleeDashDoubleHyphen.MatchString(n.Value) {
				out = append(out, fmt.Sprintf("%s:%d %s uses a dash: %q", name, n.Line, strings.TrimPrefix(path, "."), n.Value))
			}
		}
	}
	walk(&doc, "")
	return out, scalars, nil
}

func TestCouleeTextHasNoDashes(t *testing.T) {
	root := repoRoot(t)
	world := filepath.Join(root, "_datafiles", "world", "dogmud")
	quests, dialogue, pairs := couleeDashFiles(t, world)
	// Floors: before any bridge exists the pass holds 34 quest files, 30
	// dialogue files and 3 conversation pairs (spec facts table). Fewer means
	// a path or a pattern broke, not that the text is clean.
	if len(quests) < 34 || len(dialogue) < 30 || len(pairs) < 3 {
		t.Fatalf("found %d quest, %d dialogue and %d conversation files; the guard would check too little",
			len(quests), len(dialogue), len(pairs))
	}
	scalars := 0
	files := append(append(append([]string{}, quests...), dialogue...), pairs...)
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, p)
		found, n, err := couleeDashFindings(filepath.ToSlash(rel), data)
		if err != nil {
			t.Fatal(err)
		}
		scalars += n
		for _, f := range found {
			t.Error(f)
		}
	}
	if scalars == 0 {
		t.Fatal("read no scalar values")
	}
}

// TestCouleeTextHasNoDashes_GuardCanFail proves the guard sees a planted em
// dash, en dash and spaced double hyphen (one across a folded line), skips
// comments whole-line and trailing, passes clean text, and picks the right
// quests and NPCs.
func TestCouleeTextHasNoDashes_GuardCanFail(t *testing.T) {
	bad := []byte("# a comment may say -- this \u2014 freely\n" +
		"name: First Casting\n" +
		"description: >-\n  You let it go --\n  that is the whole of it.\n" +
		"steps:\n  - id: start\n    hint: Type cast spike mote \u2014 then wait.\n" +
		"  - id: end\n    description: The mote reknits \u2013 unharmed.\n" +
		"room: 5354  # Echoing Narrows \u2014 a trailing comment\n")
	found, n, err := couleeDashFindings("bad.yaml", bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("guard saw %d of 3 planted dashes in %d scalars: %v", len(found), n, found)
	}
	good := []byte("name: First Casting\nhint: Type cast spike-mote. A --help flag is no dash.\n")
	if found, _, err := couleeDashFindings("good.yaml", good); err != nil || len(found) != 0 {
		t.Fatalf("guard flagged clean text (err %v): %v", err, found)
	}
	for id, want := range map[int]bool{28: true, 29: false, 30: true, 59: true, 60: false, 76: true, 88: true, 89: true, 90: true, 101: true, 102: false} {
		if couleeDashQuest(id) != want {
			t.Errorf("quest %d: in the pass = %v, want %v", id, !want, want)
		}
	}
	for id, want := range map[int]bool{9099: false, 9100: true, 9169: true, 9170: false, 9491: true, 9604: false, 9605: true, 9608: true, 9609: false} {
		if couleeDashNPC(id) != want {
			t.Errorf("mob %d: Coulee NPC = %v, want %v", id, !want, want)
		}
	}
}
```

- [ ] **Step 2: Prove the can-fail test fails when the guard is blind**

Temporarily change `couleeDashDoubleHyphen` in the new file to `regexp.MustCompile("$^")` (matches nothing), then run:
```bash
go test ./internal/guards/ -run TestCouleeTextHasNoDashes_GuardCanFail
```
Expected: FAIL `guard saw 2 of 3 planted dashes`. Restore the original pattern with Edit and rerun: PASS.

- [ ] **Step 3: Run the guard over shipped text**

```bash
go test ./internal/guards/ -run TestCouleeTextHasNoDashes -v 2>&1 | tail -20
```
Expected: PASS. If it fails:
- A finding in `conversations/pairs/9103_9105.yaml` line `"The young ones passing through -- they spend before they think."`: replace that `text:` value with `"The young ones passing through spend before they think."`.
- A finding in `conversations/pairs/9106_9107.yaml` line `"That one with the long coat -- you recognize the face?"`: replace with `"That one with the long coat. You recognize the face?"`.
- A finding in any quest 44 to 52, 57 to 59, 76, 94 to 96 or the dialogue files this plan wrote: fix the sentence there (it is this plan's text).
- A finding in any other file (quests 28, 30 to 43, 53 to 56, 88, 89, 90 to 93, other Coulee dialogue): STOP and report the file and line to the controller; it belongs to plan 2, 3a or 4.

Rerun until PASS.

- [ ] **Step 4: Commit**

```bash
gofmt -l internal/guards
git add internal/guards/coulee_dash_guard_test.go
git status --short _datafiles/world/dogmud/conversations/pairs
# add each conversation file only if Step 3 changed it:
# git add _datafiles/world/dogmud/conversations/pairs/9103_9105.yaml _datafiles/world/dogmud/conversations/pairs/9106_9107.yaml
git commit -F - <<'EOF'
test(guards): Coulee dash guard for quests, dialogue and conversations (#375 p7)

Walks the parsed YAML of the Coulee pass's quest files (28, 30 to 59,
76, 88, 89, 90 to 101), every Coulee and antechamber dialogue file and
every conversation pair with a Coulee NPC, and fails on an em dash, an
en dash or a standalone double hyphen. Comments are never read. A
can-fail sibling plants each dash and checks the file selection.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Docs, context files, patch notes

**Executor:** Sonnet.

**Files:**
- Modify: `internal/questengine/context.md`, `internal/usercommands/context.md`, `internal/actions/context.md`, `internal/guards/context.md`
- Modify: `.claude/skills/dogmud-authoring-quests/SKILL.md` (one line)
- Modify: `docs/PATCH_NOTES.md`; `docs/README.md` (only if it has no row for this plan)

- [ ] **Step 1: questengine context**

In `internal/questengine/context.md`, replace
```
Quest 49's floor search discovers `disc` in the Reliquary (5343).
```
with
```
No shipped quest uses `discover` since quest 76's disc moved to Maren's
Cottage and the Reliquary (5343) lost its `disc` hidden noun (#375 phase 7).
```

- [ ] **Step 2: usercommands context**

In `internal/usercommands/context.md`, replace
```
window, never the room's, and no contested tier. The quest `command`
notification fires for every form of `search`, as before (quests 49 and 58
gate on it in room 5343). See "Targeted search" in
`internal/actions/context.md`.
```
with
```
window, never the room's, and no contested tier. The quest `command`
notification fires for every form of `search` (quests 49 and 58 gate on it
in room 5343, quest 76 in room 4023). Before searching, `questSearchStep`
asks the engine whether a `command: search` trigger would fire for the
player in this room and passes it as `actions.SearchOptions.QuestStep`, so
the search does not print "You find nothing of interest." ahead of the
quest's own line (#375 phase 7). See "Targeted search" in
`internal/actions/context.md`.
```

- [ ] **Step 3: actions context**

In `internal/actions/context.md`, replace
```
lists the quest search spots a whole-room search turned up. After the roll,
`usercommands/skill.search.go` raises `room_interact` with `Verb: search`
for the feature typed.
```
with
```
lists the quest search spots a whole-room search turned up. After the roll,
`usercommands/skill.search.go` raises `room_interact` with `Verb: search`
for the feature typed. `SearchOptions.QuestStep` (set by the caller when a
`command: search` quest trigger in the room would fire for the player)
skips "You find nothing of interest.", since that trigger's own text says
what the search turned up (#375 phase 7).
```

- [ ] **Step 4: guards context**

In `internal/guards/context.md`, replace
```
- **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
  `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
  `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`.
```
with
```
- **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
  `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
  `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`, and
  `coulee_dash_guard_test.go` (#375 phase 7: the Coulee and antechamber
  quest, dialogue and conversation files, read as parsed YAML so comments
  are never checked).
```

- [ ] **Step 5: The authoring skill's live example**

In `.claude/skills/dogmud-authoring-quests/SKILL.md`, replace
```
  (`internal/quests/live_destination_kinds_test.go`): area, quest 45 `start`;
```
with
```
  (`internal/quests/live_destination_kinds_test.go`): area, quest 45 `met`;
```

- [ ] **Step 6: Patch notes**

In `docs/PATCH_NOTES.md`, insert after the line `# DOGMud Patch Notes` and its blank line:
```markdown
## 2026-10-30: Pothole Coulee, casting, lore and archery trails

- Each of these three trails now starts with Crier Toke, who sends you to
  your first teacher. Every quest begins by finding the person who gives
  it, and ends when you go back and tell them.
- Each teacher sends you on to the next: Grieve to Orrin, Wenna to Hale
  and then to Sere, Iden to Bryn. When you finish a trail, your last
  teacher asks you to report to Toke.
- The repeatable tasks (Tending the Grove, Reading the Stones, Range
  Practice) now end when you tell the giver.
- Grieve explains where new spells come from, and what manifestation,
  conjuring and charming are. Orrin explains raising the fallen and how
  to judge a corpse first. Hale explains rally and warcry.
- Searching for a quest no longer tells you it found nothing just before
  the quest tells you what you found.
- Hale's directions to the old shrine are right now, and the School
  Hall notice board names commands you can actually type.
- Garrow really does leave the green when you taunt him.
- The grey disc is no longer in Pothole Coulee. Brennan's quest now sends
  you to search Maren's Cottage in Ashwick, and you report back to him
  when the disc wakes.
```

- [ ] **Step 7: README row (only if missing)**

```bash
grep -n "2026-10-30-p7-coulee-3b" docs/README.md
```
If it prints nothing, insert after the row for `superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md`:
```markdown
| [`superpowers/plans/2026-10-30-p7-coulee-3b-lines-cast-lore-archery-disc.md`](superpowers/plans/2026-10-30-p7-coulee-3b-lines-cast-lore-archery-disc.md) | Plan 3b of the phase 7 Pothole Coulee pass: the casting, lore and archery lines giver to giver (quests 44 to 52 and 57 to 59), bridges 94 to 96 back to Toke, quest 76's interim disc home in Maren's Cottage, `SearchOptions.QuestStep`, Garrow's yield, and the Coulee dash guard |
```

- [ ] **Step 8: Commit**

```bash
git add internal/questengine/context.md internal/usercommands/context.md internal/actions/context.md internal/guards/context.md .claude/skills/dogmud-authoring-quests/SKILL.md docs/PATCH_NOTES.md
git status --short docs/README.md   # add it only if Step 7 changed it
git commit -F - <<'EOF'
docs: casting, lore and archery trails, quest 76, Coulee dash guard (#375 p7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 15: Gate, PR, #625 comment

**Executor:** Opus.

**Files:** none modified (unless a gate fails).

- [ ] **Step 1: Formatting and build**

```bash
cd C:/tmp/dogmud-375-p7-coulee-lines-b
gofmt -l internal/ modules/
go build ./...
```
Expected: `gofmt -l` prints nothing; build succeeds.

- [ ] **Step 2: Touched packages and guards**

```bash
go test ./internal/questengine/ ./internal/questloop/ ./internal/quests/ ./internal/dialogue/ ./internal/behaviortree/ ./internal/actions/ ./internal/usercommands/ ./internal/guards/... 2>&1 | tail -12
```
Expected: every line `ok`.

- [ ] **Step 3: Shipped-data loop and boot sweeps**

```bash
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)|quest loop:'
```
Expected: `ok`, every `--- ` line is `--- PASS` (none `--- SKIP`; a SKIP means `DOGMUD_BOOT_SMOKE` was not set and nothing was proven), and the `quest loop:` line reads 27 fewer known findings than Task 1's baseline, `0 new, 0 stale`.

- [ ] **Step 4: The backlog for this pass's quests**

```bash
grep -nE '^\S+ (2[89]|3[0-9]|4[0-9]|5[0-9]|76|88|89) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```
Expected, exactly one line (plan 4's). Plan 2 already deleted `chain 30 mob 9100 topic road_esk`:
once Hadwen takes quest 30's hand-in, his `road_esk` offer passes `ruleChain`.
```
end 89 quest 89 trigger room_enter room=468 grants=89-end
```
Any other line: name its quest and owner per the Backlog map and report it to the controller (if it is one of this plan's 27, the task that should have deleted it missed it: fix and rerun Step 3).

- [ ] **Step 5: Lint**

```bash
golangci-lint run --new-from-merge-base=origin/master
```
Expected: `0 issues.`

- [ ] **Step 6: Isolated boot check**

```bash
S="<your session scratchpad directory, full path>"
cd C:/tmp/dogmud-375-p7-coulee-lines-b
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
grep -E "quest loop|offered but not abandonable" "$S/boot.log" | tail -5
cd C:/tmp/dogmud-375-p7-coulee-lines-b && git worktree remove --force C:/tmp/dogmud-boot-check
```
Expected: `exit 124`; `0`; `1`. The quest loop warning count matches Step 3. If `git worktree remove` fails on a locked exe, use PowerShell `Remove-Item -Recurse -Force C:/tmp/dogmud-boot-check` and `git worktree prune`.

- [ ] **Step 7: Push and open the PR**

Write the PR body to `<scratchpad>/pr-3b.md` with the Write tool:
```markdown
#375 phase 7, Pothole Coulee plan 3b: the casting, lore and archery lines giver to giver, bridges 94 to 96, quest 76's interim disc home, and the Coulee dash guard.

- Quests 44 to 52 and 57 to 59: one action per step; every quest opens by meeting its giver and ends at a hand-in; each offer sits on the NPC who took the previous hand-in.
- Bridges "Report to Toke (Casting/Lore/Archery)" (94 to 96), offered by Orrin, Sere and Bryn, handed in to Toke.
- Quests 49 and 58 progress on the real `search` command; `actions.SearchOptions.QuestStep` keeps the search from saying "You find nothing of interest." before the quest speaks.
- Quest 76: the disc is found by searching Maren's Cottage, woken by casting glow at the hearth, handed in to Brennan. The Reliquary lost its disc hidden noun.
- Garrow's taunt beat despawns him (the green respawns him) instead of a flee that could fail.
- Skill coverage topics on Grieve, Orrin and Hale. #689 directions and notice board, #690 Wenna, Garrow, Cleric's Study desk, #697 mote line.
- New guard `internal/guards/coulee_dash_guard_test.go`.
- Clears 27 lines from `loop-backlog.txt`.

Refs #375, #689, #690, #697, #625. (Not closed here: the remaining checklist items and the playtest belong to other phase 7 PRs.)

Gates run locally: gofmt, build, touched packages, guards, `DOGMUD_BOOT_SMOKE=1` loop and boot sweeps, golangci-lint (0 issues), isolated boot (exit 124, Server Ready). `-race` runs in CI only.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```
Then:
```bash
git push -u origin feat/375-p7-coulee-lines-b
gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-coulee-lines-b --title "feat(coulee): casting, lore and archery lines, quest 76 interim home, Coulee dash guard (#375 p7)" --body-file "<scratchpad>/pr-3b.md"
```
Expected: the URL printed names `pruuk/DOGMud`. The body uses "Refs", never "closes" (a closing keyword auto-closes the issue at merge).

- [ ] **Step 8: Watch the checks**

```bash
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run list --repo pruuk/DOGMud --branch feat/375-p7-coulee-lines-b --limit 5
```
Expected: all green, and the expected workflows actually ran. On a red check, read `gh run view <id> --repo pruuk/DOGMud --log-failed` before acting.

- [ ] **Step 9: Comment on #625**

Write `<scratchpad>/625-comment.md` with the Write tool (replace `<n>` with the PR number):
```markdown
Interim home shipped in #375 phase 7, Pothole Coulee plan 3b (PR #<n>), owner option A:

- The Inert Disc (40167) is no longer in Pothole Coulee. It is found in Maren's Cottage (room 4023, Ashwick) by the real `search` command, given only to a player on quest 76, or to one who finished it and lost the woken disc.
- The same search reads the journal (step `journal`). Casting Chrysalis Glow at the hearth there (`cast glow`, step `attune`) consumes 40167 and gives the Attuned Disc (40168).
- Quest 76 now ends with a hand-in at Brennan the Surveyor (9516, topic `disc_in`). Its eight loop-backlog lines are gone.
- The Reliquary (5343) lost its `disc` hidden noun and the lines that promised a find. Quest 49 keeps the disc under its floor as story only.
- Players who already hold 40167 or 40168 keep them. The Threshold Keeper (9553) still opens the way for 40168.

This issue stays open for the full endgame redesign: where the disc lives, how it is found, and whether the cottage and Brennan stay.
```
Then:
```bash
gh issue comment 625 --repo pruuk/DOGMud --body-file "<scratchpad>/625-comment.md"
```
Expected: the printed URL is on `pruuk/DOGMud`.

- [ ] **Step 10: Hand back**

Report to the controller: the PR URL, the check status, the loop count change (27 lines cleared), whether Task 12 applied or is waiting, and any guard finding Task 13 handed back. Do not merge or deploy.
