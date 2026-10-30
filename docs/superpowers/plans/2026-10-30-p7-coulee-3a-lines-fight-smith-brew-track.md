# Coulee Lines A (Fighting, Smithing, Brewing, Tracking) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the fighting, smithing, brewing and tracking lines of Pothole Coulee to the giver-to-giver shape (Toke, Giver 1, Giver 2, back to Toke): every step one action with a typed-command hint, every finale and repeatable ending at a hand-in, bridges 90 to 93 back to Crier Toke, the skill-coverage additions for these lines, and 39 `loop-backlog.txt` lines cleared.

**Architecture:** Content only, plus one engine line: `eat` raises an `eat` command event (`questengine.NotifyCommand`) so quest 42 can gain an eat step. Quest YAML is replaced file by file; second-giver dialogue files are replaced whole; first-giver files and Toke's file get targeted Edit operations because plan 2 edits them too. Each line is one task whose commit boots clean on its own: the quest files, dialogue, manifests and backlog deletions for a line land together.

**Tech Stack:** Go 1.x, YAML content under `_datafiles/world/dogmud/`

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md

**Branch / worktree:** branch `feat/375-p7-coulee-lines-a`, worktree `C:/tmp/dogmud-375-p7-coulee-lines-a`, off `origin/master` after plans 1 (engine) and 2 (spine) have merged.

---

## Spec deviations found (read before starting)

1. **Bridge quest file names.** The shared naming gives `quests/90-report_to_toke_fighting.yaml`, but the loader refuses any quest file whose path does not end in `Filepath()` (`internal/fileloader/fileloader.go:102`), and `Quest.Filename()` is `<id>-ConvertForFilename(name).yaml` (`internal/quests/quests.go:327-330`). `ConvertForFilename` turns every character outside a-z and 0-9 (apostrophes aside) into `_` (`internal/util/util.go:920-937`), so "Report to Toke (Fighting)" must live in `90-report_to_toke__fighting_.yaml` (two underscores before the line, one after). This plan keeps the shared quest names and uses the converted file names. Plans 3b (94 to 96) and 4 (97) must follow the same rule.
2. **Tailsweep is a trip variant, not a kick variant.** `kick` becomes stomp (prone target) or knee (dominant grapple) (`internal/usercommands/kick.go:66-72`, `templates/help/kick.template`); tailsweep is `actions.TripTailsweep` (`internal/usercommands/trip.go:75`). Vorn's topic says so accurately.
3. **#697 kit salve.** A potion with `CraftedRound == 0` prints no age line at all: `hasAging := itemSpec.Aging.HasAging() && matchItem.CraftedRound > 0` (`internal/actions/drink.go:234`, plan 1 may have moved this into the `quaff` path), and nothing stamps a kit item (`tools/playtest/profiles/early.yaml:36-37`; `items.New` and `Item.Validate` never set it, `internal/items/items.go:79-100,219-236`; `internal/hooks/pinnacle_tick.go:90-96` documents purchased and spawned potions as never aging). The playtest seat's "freshly brewed" line came from the salve it brewed in quest 38, which is correct. No data change; Task 9 records this on #697.
4. **Item handouts stay on the quest 2 offers.** The firebomb (39) and salt pouch (42) move with their offers to Birna and Tarn instead of to Falv's and Delk's talk topics, because `TestOfferCardFirstStepDoesNotClaimTheItem` needs at least nine item-giving offers (`internal/guards/offer_card_possession_guard_test.go:106`, its list names 39 and 42) and `TestFalvFirebombHandoutGivesOnlyOne` counts Falv's handouts (`internal/questengine/newbie_alchemy_steps_test.go:160-177`).
5. **Toke's four quest 1 offers** (`send_drill`, `send_forge`, `send_brew`, `send_steppe`, `dialogue/pothole_coulee/9106.yaml:180-249`) tell the player to do the first action before meeting the giver ("type wield cudgel", "type list", "type track hare"). Controller ruling (2026-10-30): each lines plan owns Toke's offer `text` and `hints` for its own quest 1. Tasks 2, 4, 5 and 6 rewrite them to "go see the giver", with routes checked against room exits (Facts table). Apart from those and the four hand-ins, Toke's file is untouched.
6. **`32-consider` is renamed, not retired.** A retired current step is only logged on load (`internal/users/users.go:572-579`) and would strand a saved player; `32-consider -> 32-special` in `step-ids.txt`'s `# renamed` section moves them onto `special`, whose next step is `verbosity` (`internal/characters/quests.go:357-380`).
7. **Lenient grants end.** `35-craft` and `38-brew` gated on `-start` on purpose (`internal/questengine/newbie_one_command_steps_test.go:367-371`); that is exactly the `one-action ...:prev` and `walk-dead-end` finding. They now gate on the step before. A player saved on `35-start` or `38-start` with materials in hand talks to the giver and buys again.
8. **`eat` raises no quest event today** (`internal/usercommands/eat.go:19-96` has no `NotifyCommand`). Task 1 adds it unless plan 1 already did.
9. **Playtest.** `dogmud-authoring-content` asks every content plan to end with an adversarial playtest; the spec's Delivery table puts the one naive-newbie playtest across the whole flow in PR 5. This plan follows the spec and ends at the gate.
10. **Owner call surfaced, not taken:** at `skill_minimum: 1` the scab-hide jerkin becomes a random discovery for any tailor at rank 1 (`internal/crafting/crafting.go:603-626`), which thins quest 43's recipe reward. `learn_only: true` exists (`crafting.go:39`, `:613`). The spec ruling is `skill_minimum: 1`; this plan does only that.
11. **Help mismatch:** `templates/help/enchanting.template:72` says disenchant works on "an equipped item"; the code reads the backpack (`items.FindMatchIn(rest, user.Character.Items...)`, `internal/usercommands/skill.disenchant.go:33-41`, after the `enchanting_circle` check at `:28`) and `disenchant.template` agrees with the code. Controller ruling: Task 7 fixes the line next to Daff's disenchant topic.

## Facts verified against source (worktree base: master `c4af0286b`, 2026-10-30)

Line numbers are from `c4af0286b`; plans 1 and 2 shift some of them. Task 0 re-checks the ones this plan's edits anchor on.

| Fact | Source |
|---|---|
| Backlog lines for quests 32-43 and 53-56: 39 (8 `chain`, 18 `end`, 7 `one-action`, 5 `walk-dead-end`, 1 `walk-destination`) | `quests/loop-backlog.txt:17-24,55-76,193-199,259-263,287` |
| `chain 30 mob 9108 topic train_offer` and its 9116, 9128, 9136 siblings are plan 2's (Giver 1 duplicate offers) | `quests/loop-backlog.txt:10-13` |
| `ruleChain`: an offer gated on quest N's final token needs a hand-in for N on the same mob | `internal/questloop/rules_more.go:16-52` |
| `ruleEnd`: the final token only from hand-in topics; `ruleStart`: the first token only from offers | `internal/questloop/rules_loop.go:15-55` |
| `one-action`: each way requires the previous step's token; no item, flag or gold condition (`:cond`); no double grant | `internal/questloop/rule_oneaction.go:15-56` |
| Quest 34's kill triggers carry `has_flag` (the `:cond` finding) and grant `34-end` (the `end` finding) | `quests/34-take_the_tower.yaml:174-200` |
| Quest 32 keeps an ungranted `consider` step between `special` and `verbosity` (`one-action 32 step consider:none`, `:prev` on verbosity) | `quests/32-first_blood.yaml:60-69,164-172` |
| Quest 35's sell trigger grants `35-end` | `quests/35-first_heat.yaml:143-157` |
| Quests 37, 40, 43 end on a boss kill; 53, 54, 55, 56 end on a kill, craft, drink, forage | each quest's `triggers:` |
| Quest 42 `hunt` step has no destination and its trigger names no place | `quests/42-the_hunt.yaml:66-73,151-159`; `internal/questengine/destinations_rule.go:52-58` |
| Q2 offers sit on Giver 2 today: `wash_offer` 9112, `vein_offer` 9117, `marsh_offer` 9129, `hunt_offer` 9137 | the four dialogue files |
| Toke already offers 32, 35, 38, 41 (`send_drill` gives cudgel 10043) gated on `31-end` | `dialogue/pothole_coulee/9106.yaml:180-249` |
| `topic_heard` trigger: `mob` + `topic`, fires after the topic applied; topic gated by `questRequired` | `.claude/skills/dogmud-authoring-quests/SKILL.md` "Talk steps"; `quests/10-the_drowning_posts_debt.yaml:51-58` |
| The walkthrough plays a `topic_heard` trigger's topic through its gates | `internal/questengine/walkthrough.go:870-880` |
| `count: N` fires on the Nth match; needs `has` of its own step, `missing` and a grant; the walk reaches any count at once | `.claude/skills/dogmud-authoring-quests/SKILL.md` "Areas, counts"; `internal/questengine/counts.go:30-46`; `walkplayer.go:182` |
| `buy N <item>` parses a leading quantity; each purchase raises its own `ItemOwnership` gain | `internal/actions/buy.go:300-307,794`; `internal/acquire/acquire.go:62` |
| Healer's Root is item 40004 named `Healer's Root`; `buy healer's root` matches (#695) | `items/materials-40000/40004-healers_root.yaml`; issue #695 |
| Throw raises one command event per mob its blast reached; none when it reached nobody | `internal/usercommands/throw.go:488-494` |
| `target:` filter values `self`, `mob`, `player`, `item`, `container` on `command` | `.claude/skills/dogmud-authoring-quests/SKILL.md` "Command trigger filters" |
| A trigger's grants chain `quest_granted` triggers in the same Notify | `internal/questengine/engine.go:239-250` |
| A `quest_granted` trigger with no grant and only conditions and actions ships today | `quests/50-first_shot.yaml:124-129` |
| `train_stat` calls `IncreaseStat`; the test mock records `increasedStats` | `internal/questengine/actions.go:116-118`; `actions_test.go:82-84,209-215` |
| Grilled Meat 30022 is `type: food`, `subtype: edible`; recipe `grilled-meat` at `cooking_fire`, skill_minimum 0 | `items/consumables-30000/30022-grilled_meat.yaml`; `recipes/cooking/grilled-meat.yaml` |
| `eat` has no quest notify | `internal/usercommands/eat.go:19-96` |
| `NotifyCommand(user, roomId, command, targets...)` and `TargetItem(id)` | `internal/questengine/command_event.go:40,125` |
| Hint rule in tests: each non-end step hint names exactly one `\btype\s` after stripping the dark aside and "For more, type help X." | `internal/questengine/newbie_one_command_steps_test.go:166-178,280-298` |
| Dark aside string `If it is dark, type cast glow first.` | `newbie_one_command_steps_test.go:173` |
| Each trigger granting step i (i > 1) must gate on step i-1 unless in `lenientGrants` | `newbie_one_command_steps_test.go:317-365,367-378` |
| `checkTopicSteps`: opener hint names the step command once; a quest-aware info topic names it; on the last step the quest hint has `type talk <npc>` and the opener says `choose <hand-in label>` | `internal/questengine/newbie_topic_steps_test.go:138-195` |
| Offer card guard: step 0 description of an item-giving offer must not claim possession ("you carry", "you have a", "handed you"...) | `internal/guards/offer_card_possession_guard_test.go:19,38-110` |
| Dialogue validator: info labels unique per file and never a quest title; offer and hand-in labels are the quest title; no `;` in text; a grant's own token and the end token in `questExcluded` | `internal/dialogue/world.go:110-180,232-260` |
| `menu.LintLabels`: label at most 50 runes, player line at most 70, no em or en dash, no duplicate | `internal/menu/lint.go:22-59` |
| After a hand-in, exactly one newly open offer on the NPC is pitched; two re-show the menu | `internal/conversation/nextoffer.go:15-47` |
| Manifests: keys, then `# retired` and `# renamed` (`old -> new`); neither file has those sections at `c4af0286b` | `internal/manifest/manifest.go:22-68`; `quests/step-ids.txt:1-8`; `dialogue/topic-keys.txt:1-8` |
| Rename rewrites saved progress on load | `internal/characters/quests.go:357-380`; `internal/users/users.go:572-573` |
| Narration golden records every quest `actor`/`observer` line by trigger index; `-update` regenerates | `internal/narration/testdata/stores/quests.golden:86-184`; `internal/narration/snapshot_test.go:115,823-848` |
| Loop sweep and dialogue smoke tests need `DOGMUD_BOOT_SMOKE=1` | `quest_loop_smoke_test.go:22-25`; `boot_smoke_test.go:36` |
| Room ids: Drill Yard 5227, Squatter's Hollow 5234, Lower Wash 5232, Cracked Cistern 5235, Watch Room 5242, Coulee Smithy 5245, Lower Talus 5249, Scree Field 5250, Collapsed Cut 5251, Ore Pocket 5252, Den (mine) 5260, Sheltered Pool 5265, Drying Racks 5267, Reed Beds 5269, Sunken Path 5270, Cattail Stand 5271, Mudflat 5272, Black Pool 5273, Heart of the Mire 5280, Steppe Edge 5283, Open Steppe 5284, Sage Flat 5285, Thornbrake 5289, Hunter's Hollow 5290, Den (pack) 5298, Hub Square 5203 | `rooms/pothole_coulee/<id>.yaml` `title:` |
| Spawns: Toke 5203, Vorn 5227, Garve 5234, Rusk 5245, Ovell 5251, Birna 5265, Falv 5271, Tarn 5283, Delk 5290, Onna 5207 | room `spawninfo` |
| Routes from Hub Square 5203: north, east x4 = Drill Yard (5200, 5202, 5209, 5220, 5227); north, west x5 = Coulee Smithy (5200, 5201, 5210, 5221, 5244, 5245); south, east, east, south, south = Sheltered Pool (5204, 5207, 5222, 5264, 5265); west, west, south, south = Steppe Edge (5216, 5223, 5282, 5283) | exits of those rooms |
| Routes: Drill Yard north, east x3, south = Squatter's Hollow; Smithy north, west x2, south = Collapsed Cut; Sheltered Pool south x2, east = Cattail Stand; Steppe Edge south x4, west = Hunter's Hollow | exits of 5227, 5229-5232; 5245, 5247-5249; 5265, 5266, 5269; 5283-5288 |
| Crafter stations: Patch-Tent 6460 `loom` (Bex 9605, south of Coulee Provisions 5207), Warder's Dugout 6461 `enchanting_circle` (9606, down from 5207), Gem Stall 6462 `jeweler_bench` (Vurl 9607, east of Reedwash Mouth 5222), Cook-Fire 6463 `cooking_fire` (Hesk 9608, east of Strongbox House 5208) | `rooms/pothole_coulee/6460-6463.yaml` |
| Fresh-player recipes (skill_minimum 0): `linen-tunic`, `cloth-pants` (loom: 2 cloth strip + thread spool), `honed-edge` (enchanting_circle: binding paste + healer's root, enchants the wielded weapon), `copper-ring` (jeweler_bench: copper wire + gem dust), `trail-rations`, `grilled-meat` (cooking_fire: raw meat + salt pouch) | `recipes/*/<id>.yaml`; `templates/help/enchanting.template:39` |
| Sellers: Bex thread spool 40012, bone needle 40013; 9606 binding paste 40028; Vurl copper wire 40021, gem dust 40026; Hesk raw meat 40014, salt pouch 40017, healer's root 40004; no Coulee seller of cloth strip 40007 | `mobs/pothole_coulee/9605-9608*.yaml` `shop:`; `grep -rn 40007 mobs` |
| `scab-hide-jerkin`: skill_minimum 8, loom, 2 pack-hide + thread spool + bone needle; pack-hide pelt 40070 drops from 9140-9143 | `recipes/tailoring/scab-hide-jerkin.yaml:5-17`; `grep -rln 40070 mobs` |
| `disenchant`: needs an `enchanting_circle`, an enchanted item in the backpack, leaves Enchant Withdrawal | `internal/usercommands/skill.disenchant.go:20-90` |
| `identify` is a starter spell (cost conviction, no gold) | `internal/characters/character.go:381-385`; `spells/identify.yaml` |
| `appraise` costs a merchant fee, baubles free; `offer` asks a merchant's price; bartering is Charisma, better buy and sell prices, trained by trading | `templates/help/appraise.template`, `offer.template`, `bartering.template` |
| `rally` has no rank gate (no skill check in the file); quest 28 teaches `warcry` in the Proving | `internal/usercommands/rally.go` (125 lines, no `skill`); `quests/28-waking_to_gaius.yaml:111-115` |
| Warcry raises physical damage; rally improves dodge, parry and block | `templates/help/warcry.template:3-6`; `rally.template:3-6` |
| Defence: dodge (everyone), parry (weapon), block (shield); fists have short reach | `templates/help/combat.template:42-50`; `unarmed-combat.template:18-21` |
| `grapple <target>`, held foe cannot flee, kick becomes knee in a dominant hold | `templates/help/grapple.template:3-6,63-65`; `kick.template` |
| `bash` needs a shield, may knock prone, shares the special-move wait | `templates/help/bash.template` |
| Enchanter Skell is 9606; references: mob file and name, dialogue hint "what Skell sells", schedule description, a comment in Vurl's mob file. Goals files are gitignored. Porter Skell (9372) and Bowmaster Skell (9169) are other mobs | `mobs/pothole_coulee/9606-enchanter_skell.yaml:24`; `dialogue/pothole_coulee/9606.yaml:16`; `schedules/pothole_coulee/pothole_coulee_enchanter.yaml:2`; `mobs/pothole_coulee/9607-jeweler_vurl.yaml` shop comment; `.gitignore:44-45` |
| "Daff" appears nowhere in `_datafiles/world/dogmud` or `docs/world.md` (substring search; the same search finds "Mote" and "Wren", so it can find names) | `grep -rli daff` |
| Dash lines in scope: 23 player-facing in quests 33-43 and 54; 1 each in pairs `9103_9105` and `9106_9107`; `9101_9106` and the dialogue files have none | `grep -nP '(^\|\s)--(\s\|$)\|\x{2014}\|\x{2013}'` over the scope |
| Help files `tailoring`, `cooking`, `jewelcrafting`, `enchanting`, `combat`, `kick`, `grapple`, `bash`, `rally`, `appraise`, `offer`, `identify`, `bartering`, `eat` exist; `quaff` does not exist at `c4af0286b` (plan 1's) | `ls _datafiles/world/dogmud/templates/help` |

## File map

| Path | Change |
|---|---|
| `internal/usercommands/eat.go` | Modify: raise the `eat` command event after a successful eat |
| `internal/usercommands/eat_quest_event_test.go` | Create: the eat event test |
| `_datafiles/world/dogmud/quests/32-first_blood.yaml` ... `43-break_the_pack.yaml`, `53-cull_the_wash.yaml` ... `56-running_game.yaml` | Replace whole: new step shapes, hints, hand-ins, no dashes |
| `_datafiles/world/dogmud/quests/90-report_to_toke__fighting_.yaml`, `91-report_to_toke__smithing_.yaml`, `92-report_to_toke__brewing_.yaml`, `93-report_to_toke__tracking_.yaml` | Create: the four bridges |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml`, `9117.yaml`, `9129.yaml`, `9137.yaml` | Replace whole: second givers (arrive topics, hand-ins, bridge offers) |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`, `9116.yaml`, `9128.yaml`, `9136.yaml` | Edit: first givers (arrive topics, quest 2 offers, openers, Vorn's skill topics) |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` | Edit: add four `report_<line>_in` hand-ins; reword `text` and `hints` of `send_drill`, `send_forge`, `send_brew`, `send_steppe` (controller ruling) |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9104.yaml`, `9605.yaml`, `9606.yaml`, `9607.yaml`, `9608.yaml` | Edit: Onna and crafter station topics |
| `_datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_skell.yaml` | Rename to `9606-enchanter_daff.yaml`, name field |
| `_datafiles/world/dogmud/mobs/pothole_coulee/9607-jeweler_vurl.yaml` | Edit: one shop comment |
| `_datafiles/world/dogmud/schedules/pothole_coulee/pothole_coulee_enchanter.yaml` | Edit: description |
| `_datafiles/world/dogmud/recipes/tailoring/scab-hide-jerkin.yaml` | Edit: `skill_minimum: 1` |
| `_datafiles/world/dogmud/templates/help/enchanting.template` | Edit: line 72, disenchant takes an item from the pack |
| `_datafiles/world/dogmud/conversations/pairs/9103_9105.yaml`, `9106_9107.yaml` | Edit: one dash line each |
| `_datafiles/world/dogmud/quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt` | Edit: new ids and keys, renames, 39 lines deleted |
| `internal/questengine/newbie_martial_steps_test.go`, `newbie_forge_steps_test.go`, `newbie_alchemy_steps_test.go`, `newbie_wilderness_steps_test.go`, `first_heat_sellpath_test.go` | Replace whole: pin the new shapes |
| `internal/questengine/newbie_topic_steps_test.go`, `newbie_one_command_steps_test.go` | Edit: helpers, Birna/Falv cases, quest 41 test, lenient and retired maps |
| `internal/questengine/coulee_bridges_test.go`, `coulee_skill_topics_test.go` | Create: bridges and skill-topic pins |
| `internal/actions/sell_rusk_dagger_test.go` | Edit: a sale no longer completes quest 35 |
| `internal/narration/testdata/stores/quests.golden` | Regenerate with `-update` |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Edit: player entry; a row for this plan |

## House rules for every executor

- Edit files only with the Edit and Write tools. Never write a repo file from the shell (`>`, `>>`, `sed -i`, Python `open(..., 'w')`): `~/.claude/hooks/guard.py` blocks it. `git mv` is fine for the one rename.
- `git add` named paths only. Never `git add -A`, `.` or `-u`, never `git commit -a`, never `--no-verify`.
- No em dash, en dash or ` -- ` in anything you write, comments included. No `;` in dialogue text, labels or player lines (the validator refuses it).
- YAML: in a plain (unquoted) scalar never write `: ` or ` #`; when a dialogue line needs a colon, single-quote the whole value and double any apostrophe inside (`''`). Quest files use `>-` block scalars, where colons are safe.
- Run Go tests from the worktree root. `grep -c` exits 1 on zero matches: run any "expect zero" grep on its own line, never in an `&&` chain.
- Commit messages go through a quoted heredoc (`git commit -F - <<'EOF'`) and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Backlog map (every line this plan deletes, and the task that deletes it)

| Backlog line | Fixed by | Task |
|---|---|---|
| `chain 32 mob 9112 topic wash_offer` | quest 33's offer moves to Vorn (9108), who holds 32's hand-in | 2 |
| `chain 34 mob 9112 topic cull_offer` | Garve gains `tower_in`, the hand-in for 34 | 2 |
| `end 34 quest 34 trigger mob_death mob=9115 grants=34-end` | the kill grants `34-slain`; `tower_in` grants `34-end` | 2 |
| `end 34 quest 34 trigger mob_death mob=9115 grants=34-end #2` | same | 2 |
| `end 34 step end` | `tower_in` | 2 |
| `end 53 quest 53 trigger mob_death mob=9111 grants=53-end` | the squatter kill grants `53-culled`; `cull_in` ends it | 2 |
| `end 53 step end` | `cull_in` | 2 |
| `one-action 32 quest 32 trigger command command=combatverbosity grants=32-verbosity:prev` | step `consider` removed (renamed to `special`), so `special` is verbosity's previous step | 2 |
| `one-action 32 step consider:none` | step `consider` removed | 2 |
| `one-action 34 quest 34 trigger mob_death mob=9115 grants=34-end #2:cond` | the kill trigger carries no flag; the boon moves to two non-granting `quest_granted 34-slain` triggers | 2 |
| `one-action 34 quest 34 trigger mob_death mob=9115 grants=34-end:cond` | same | 2 |
| `walk-dead-end 32 quest 32 trigger command command=combatverbosity grants=32-verbosity` | same as its `:prev` line | 2 |
| `walk-dead-end 32 step consider` | step `consider` removed | 2 |
| `chain 35 mob 9117 topic vein_offer` | quest 36's offer moves to Rusk (9116) | 4 |
| `chain 37 mob 9117 topic forge_loop_offer` | Ovell gains `mine_in`, the hand-in for 37 | 4 |
| `end 35 quest 35 trigger command room=5245 mob=9116 item=10009 command=sell grants=35-end` | the sell trigger grants nothing (Rusk's line only) | 4 |
| `end 37 quest 37 trigger mob_death mob=9123 grants=37-end` | kill grants `37-slain`; `mine_in` ends it | 4 |
| `end 37 step end` | `mine_in` | 4 |
| `end 54 quest 54 trigger command room=5245 command=craft grants=54-end` | craft grants `54-crafted`; `forge_loop_in` ends it | 4 |
| `end 54 step end` | `forge_loop_in` | 4 |
| `one-action 35 quest 35 trigger command room=5245 command=craft grants=35-craft:prev` | craft gates on `35-strip` | 4 |
| `walk-dead-end 35 quest 35 trigger command room=5245 command=craft grants=35-craft` | same | 4 |
| `chain 38 mob 9129 topic marsh_offer` | quest 39's offer moves to Birna (9128) | 5 |
| `chain 40 mob 9129 topic brew_loop_offer` | Falv gains `mire_in`, the hand-in for 40 | 5 |
| `end 40 quest 40 trigger mob_death mob=9135 grants=40-end` | kill grants `40-slain`; `mire_in` ends it | 5 |
| `end 40 step end` | `mire_in` | 5 |
| `end 55 quest 55 trigger command command=drink grants=55-end` | `quaff` grants `55-quaffed`; `brew_loop_in` ends it | 5 |
| `end 55 step end` | `brew_loop_in` | 5 |
| `one-action 38 quest 38 trigger command room=5265 command=craft grants=38-brew:prev` | brew gates on `38-roots` | 5 |
| `one-action 38 quest 38 trigger command room=5267 command=craft grants=38-brew:prev` | same | 5 |
| `walk-dead-end 38 quest 38 trigger command room=5265 command=craft grants=38-brew` | same | 5 |
| `walk-dead-end 38 quest 38 trigger command room=5267 command=craft grants=38-brew` | same | 5 |
| `chain 41 mob 9137 topic hunt_offer` | quest 42's offer moves to Tarn (9136) | 6 |
| `chain 43 mob 9137 topic game_loop_offer` | Delk gains `pack_in`, the hand-in for 43 | 6 |
| `end 43 quest 43 trigger mob_death mob=9143 grants=43-end` | kill grants `43-slain`; `pack_in` ends it | 6 |
| `end 43 step end` | `pack_in` | 6 |
| `end 56 quest 56 trigger command room=5284 command=forage grants=56-end` | forage grants `56-foraged`; `game_loop_in` ends it | 6 |
| `end 56 step end` | `game_loop_in` | 6 |
| `walk-destination 42 step hunt` | `hunt` gets `destination: anywhere` (its trigger is a loot that happens where the corpse lies) | 6 |

Lines not cleared: none in scope. The four `chain 30 mob 9108|9116|9128|9136 topic ..._offer` lines are plan 2's and must already be gone (Task 0 checks).

---

## Task 0: Worktree and preconditions

Suggested executor: Sonnet.

**Files:** none changed.

- [ ] **Step 1: Create the worktree.**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add C:/tmp/dogmud-375-p7-coulee-lines-a -b feat/375-p7-coulee-lines-a origin/master
cd C:/tmp/dogmud-375-p7-coulee-lines-a
git log --oneline -3
```

Every later command runs in `C:/tmp/dogmud-375-p7-coulee-lines-a`.

- [ ] **Step 2: Plan 1 merged: `quaff` exists and notifies `quaff`.**

```bash
grep -rn '"quaff"' internal/actions internal/usercommands | grep -v _test
```

Expected: at least one `questengine.NotifyCommand(..., "quaff", ...)` line. If there is none, stop and report: plan 1 has not merged.

- [ ] **Step 3: Plan 2 merged: Giver 1 duplicate offers gone.** Run each on its own line (zero matches exit 1):

```bash
grep -c "key: train_offer" _datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml
grep -c "key: craft_offer" _datafiles/world/dogmud/dialogue/pothole_coulee/9116.yaml
grep -c "key: brew_offer" _datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml
grep -c "key: sign_offer" _datafiles/world/dogmud/dialogue/pothole_coulee/9136.yaml
grep -c "^chain 30 mob 91" _datafiles/world/dogmud/quests/loop-backlog.txt
```

Expected: `0` for each. Any other number: stop and report.

- [ ] **Step 4: Note whether `eat` already raises a quest event.**

```bash
grep -n "NotifyCommand" internal/usercommands/eat.go
```

Record the result for Task 1 (no output means Task 1 adds it).

- [ ] **Step 5: Check the anchors the Edit tasks rely on are unchanged since `c4af0286b`.**

```bash
git log --oneline c4af0286b..HEAD -- _datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9117.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9137.yaml
```

Expected: no output (the second givers are untouched, so their whole-file replacements in Tasks 2, 4, 5 and 6 lose nothing). If a commit is listed, read `git show <sha> -- <file>` and carry that commit's change into the replacement text in this plan before applying it; report what you carried.

```bash
git log --oneline c4af0286b..HEAD -- _datafiles/world/dogmud/quests/3[2-9]-*.yaml _datafiles/world/dogmud/quests/4[0-3]-*.yaml _datafiles/world/dogmud/quests/5[3-6]-*.yaml
```

Expected: at most plan 1's commit touching quests 38 and 55 (`drink` to `quaff`). The whole-file replacements below already use `quaff`. Any other commit: read it and carry its change, as above.

- [ ] **Step 6: Baseline.** Run and save the output to the session scratchpad (not `C:/tmp`):

```bash
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
go test -count=1 ./internal/questengine/ ./internal/guards/... 2>&1 | tail -5
```

Expected: both `ok`, and the smoke run lists `--- PASS` lines (`TestSmoke_ShippedQuestsPassTheLoop` among them), never `--- SKIP`; a skip means the variable was not set and the baseline proved nothing. If either fails on the untouched base, stop and report.

---

## Task 1: `eat` raises the `eat` command event

Suggested executor: Sonnet.

**Files:**
- Create: `internal/usercommands/eat_quest_event_test.go`
- Modify: `internal/usercommands/eat.go`

- [ ] **Step 1: Write the failing test.** Create `internal/usercommands/eat_quest_event_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

const eatProbeMealId = 999964

// Eating raises the eat command event, naming the food (#375 phase 7): the
// Hunt (quest 42) asks the player to eat the meal they cooked, and a step can
// only listen for an event the command raises.
func TestEat_RaisesTheEatCommandEvent(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		eatProbeMealId: {ItemId: eatProbeMealId, Name: "Probe Stew", NameSimple: "stew",
			Type: items.Food, Subtype: items.Edible, Uses: 1},
	}))
	user, room := getTestUserAndRoom(t)
	orig := user.Character.Items
	t.Cleanup(func() { user.Character.Items = orig })
	user.Character.Items = []items.Item{items.New(eatProbeMealId)}
	events.DrainQueuedMessagesForTest(user.UserId)

	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999303,
		Name:    "eat probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "command",
			Command: "eat",
			Item:    eatProbeMealId,
			Actions: []questengine.ActionDef{{Grant: "999303-start"}},
		}},
	})
	t.Cleanup(func() { user.Character.ClearQuestToken("999303-start") })

	Eat("stew", user, room, 0)

	require.Contains(t, sentTo(user), "You eat some of the", "the meal must actually be eaten for the probe to mean anything")
	require.True(t, user.Character.HasQuest("999303-start"), "eating must raise the eat command event naming the food")
}
```

If plan 1 renamed `Eat`'s first line ("You eat some of the") or changed `Eat`'s signature, use the line and signature `eat.go` has now; the assertion is that the meal was eaten.

- [ ] **Step 2: Run it and see it fail** (skip to Step 4 if Task 0 Step 4 found a `NotifyCommand` already).

```bash
go test -count=1 ./internal/usercommands/ -run TestEat_RaisesTheEatCommandEvent
```

Expected: `FAIL`, message `eating must raise the eat command event naming the food`.

- [ ] **Step 3: Implement.** In `internal/usercommands/eat.go` add the import `"github.com/GoMudEngine/GoMud/internal/questengine"` to the import block (keep it sorted), and, inside the success branch, directly after the loop that applies the food's conditions:

```go
		for _, conditionId := range itemSpec.ConditionIds {
			user.AddCondition(conditionId, `food`)
		}
```

add:

```go

		// Quest engine: command notification, after the eat lines and the
		// meal's effects, so a step line reads after "You eat" (the order
		// #697 asked of drink). Quest 42's eat step listens for it (#375
		// phase 7).
		questengine.NotifyCommand(user, room.RoomId, "eat", questengine.TargetItem(matchItem.ItemId))
```

If plan 1 restructured `Eat`, place the call at the same point: after the meal is consumed and its conditions applied, before the function returns from the success path.

- [ ] **Step 4: Run it and see it pass; prove it can fail.**

```bash
go test -count=1 ./internal/usercommands/ -run 'TestEat_'
```

Expected: `ok`. Then comment out the `NotifyCommand` line, rerun, confirm the failure names `eating must raise the eat command event`, and restore the line.

- [ ] **Step 5: Package tests and commit.**

```bash
gofmt -l internal/usercommands/
go test -count=1 ./internal/usercommands/ ./internal/guards/...
git add internal/usercommands/eat.go internal/usercommands/eat_quest_event_test.go
git commit -F - <<'EOF'
feat(usercommands): eat raises the eat command event

Quest 42 (The Hunt) gains an eat step in #375 phase 7, and a step can
only listen for an event a command raises. Eat now calls NotifyCommand
with the food it ate, after the eat lines and the meal's effects.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(If Task 0 found the call already present, commit only the test, with the subject `test(usercommands): pin the eat command event`.)

---

## Task 2: The fighting line (32, 33, 34, 53, bridge 90)

Suggested executor: Opus (player copy and dialogue judgement).

**Files:**
- Replace: `internal/questengine/newbie_martial_steps_test.go`
- Modify: `internal/questengine/newbie_topic_steps_test.go` (two helpers), `internal/questengine/newbie_one_command_steps_test.go` (`retiredSteps`)
- Create: `internal/questengine/coulee_bridges_test.go`
- Replace: `_datafiles/world/dogmud/quests/32-first_blood.yaml`, `33-hold_the_wash.yaml`, `34-take_the_tower.yaml`, `53-cull_the_wash.yaml`
- Create: `_datafiles/world/dogmud/quests/90-report_to_toke__fighting_.yaml`
- Replace: `_datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`, `9106.yaml`
- Modify: `_datafiles/world/dogmud/quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Regenerate: `internal/narration/testdata/stores/quests.golden`

- [ ] **Step 1: Add the two shared helpers.** In `internal/questengine/newbie_topic_steps_test.go`, directly after the closing `}` of `func checkHandout(...)`, add:

```go

// checkArriveTopic pins a "go see the giver" step (#375 phase 7): the
// quest's start step hint teaches `talk <npc>`, the giver's arrive topic is
// an info topic on the menu only while the player is on the start step, it
// grants nothing itself, and the quest's topic_heard trigger on it moves the
// player from start to met.
func checkArriveTopic(t *testing.T, npc, dfile string, q *QuestDef, mobId int, key string, pre []string) {
	t.Helper()
	require.Contains(t, questHintFor(t, q, "start"), "type talk "+npc,
		"quest %d: the start step sends the player to %s with talk", q.QuestId, npc)
	df := loadCouleeTopics(t, dfile)
	require.Equal(t, mobId, df.MobId)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %q", dfile, key)
	require.Equal(t, dialogue.KindInfo, tp.Kind, "%s is a talk step, an info topic", key)
	require.Empty(t, tp.GrantsQuest, "%s grants nothing itself; the quest's topic_heard trigger does", key)

	require.Nil(t, newNpcTalker(df, playerOn(q, "", pre...)).eligible(key),
		"%s must stay shut before quest %d is taken", key, q.QuestId)
	require.NotNil(t, newNpcTalker(df, playerOn(q, "start", pre...)).eligible(key),
		"%s must be on the menu on quest %d's start step", key, q.QuestId)
	require.Nil(t, newNpcTalker(df, playerOn(q, "met", pre...)).eligible(key),
		"%s must close once the player has met %s", key, npc)

	w := newQuestWalker(q).at("start")
	w.notify("topic_heard", EventDetails{MobId: mobId, Topic: key})
	require.Equal(t, "met", w.p.current, "hearing %s must move the player onto met", key)
}

// checkOfferMoved: a line's quest 2 is offered by the giver who took quest
// 1's hand-in (#375 phase 7, ruleChain), and the second giver no longer
// offers it.
func checkOfferMoved(t *testing.T, offerFile, key string, q *QuestDef, after, oldFile string) {
	t.Helper()
	df := loadCouleeTopics(t, offerFile)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %q", offerFile, key)
	require.Equal(t, dialogue.KindOffer, tp.Kind)
	require.Equal(t, q.QuestId, tp.Quest)
	require.Equal(t, fmt.Sprintf("%d-start", q.QuestId), tp.GrantsQuest)
	require.Equal(t, []string{after}, tp.QuestRequired, "%s opens on quest 1's hand-in", key)
	old := loadCouleeTopics(t, oldFile)
	for i := range old.Topics {
		o := &old.Topics[i]
		require.False(t, o.Kind == dialogue.KindOffer && o.Quest == q.QuestId,
			"%s still offers quest %d (topic %s); only the first giver offers it", oldFile, q.QuestId, o.Key)
	}
}
```

(`fmt`, `dialogue` and `require` are already imported in that file.)

- [ ] **Step 2: Retire the retired-steps entry.** In `internal/questengine/newbie_one_command_steps_test.go` replace:

```go
// retiredSteps are steps no trigger grants any more. Each stays in its
// quest's step list only so a character saved on it resumes, so the step
// gate skips over it when it looks for the step before a grant.
var retiredSteps = map[string]bool{
	"32-consider": true, // consider moved ahead of the fight as sized_up (owner 2026-10-19)
}
```

with:

```go
// retiredSteps are steps no trigger grants any more. Each stays in its
// quest's step list only so a character saved on it resumes, so the step
// gate skips over it when it looks for the step before a grant. None today:
// quest 32's old consider step was renamed to special in step-ids.txt
// (#375 phase 7), which moves a saved player on instead of keeping the step.
var retiredSteps = map[string]bool{}
```

- [ ] **Step 3: Replace the fighting tests.** Overwrite `internal/questengine/newbie_martial_steps_test.go` with:

```go
package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fighting line of Pothole Coulee (#375 phase 7): Crier Toke offers
// First Blood (32) and Drillmaster Vorn takes its hand-in; Vorn offers Hold
// the Wash (33) and Caravan Guard Garve takes it; Garve offers Take the
// Tower (34), then Report to Toke (Fighting) (90) and the repeatable Cull
// the Wash (53). One command per step, each trigger gated on the step before
// it (owner ruling 2026-10-17), and every quest ends at a hand-in.

const (
	martialDummy     = 9109
	martialScout     = 9110
	martialSquatter  = 9111
	martialCaptain   = 9115
	martialCudgel    = 10043
	martialDrillYard = 5227
	martialHollow    = 5234
	martialVorn      = 9108
	martialGarve     = 9112
)

func martialStepsKept(t *testing.T, q *QuestDef, old []string) []string {
	t.Helper()
	steps := newLinearPlayer(q).steps
	last := -1
	for _, s := range old {
		i := (&linearPlayer{steps: steps}).stepIndex(s)
		require.GreaterOrEqual(t, i, 0, "step %q must survive so a player saved on it resumes", s)
		require.Greater(t, i, last, "old step %q must keep its place in the order", s)
		last = i
	}
	return steps
}

func TestQuest32FirstBlood_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "32-first_blood.yaml", 32)
	steps := martialStepsKept(t, q, []string{"start", "wield", "strike", "special", "verbosity", "end"})
	require.Equal(t, []string{"start", "met", "wield", "sized_up", "strike", "special", "verbosity", "end"}, steps)
	turnIn := turnInRequires(t, "9108.yaml", "train_turnin")
	require.Equal(t, []string{"32-verbosity"}, turnIn, "the turn-in gates on the last step's token")

	cmd := func(c string) EventDetails { return EventDetails{Command: c, RoomId: martialDrillYard} }
	kill := EventDetails{MobId: martialDummy, RoomId: martialDrillYard}
	equipCudgel := EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: martialCudgel}
	heard := EventDetails{MobId: martialVorn, Topic: "drill_arrive", RoomId: martialDrillYard}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", equipCudgel)
		require.Equal(t, "start", w.p.current, "wielding before Vorn has spoken must not skip the talk")
		w.notify("topic_heard", EventDetails{MobId: martialGarve, Topic: "drill_arrive", RoomId: martialHollow})
		require.Equal(t, "start", w.p.current, "only Vorn's topic is the talk step")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", kill)
		require.Equal(t, "met", w.p.current, "a kill before wielding must not skip the wield step")
		w.notify("command", EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: 10038})
		require.Equal(t, "met", w.p.current, "wielding the sling must not finish the cudgel step")
		w.notify("command", equipCudgel)
		require.Equal(t, "wield", w.p.current)

		w.notify("mob_death", kill)
		require.Equal(t, "wield", w.p.current, "a kill before consider must not skip it")
		w.notify("command", cmd("kick"))
		require.Equal(t, "wield", w.p.current, "a kick before consider must not skip it")
		w.notify("command", cmd("consider"))
		require.Equal(t, "sized_up", w.p.current)

		w.notify("command", cmd("kick"))
		require.Equal(t, "sized_up", w.p.current, "a kick before the kill must not skip the fight")
		w.notify("mob_death", kill)
		require.Equal(t, "strike", w.p.current)

		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "strike", w.p.current, "verbosity before the special move must not skip it")
		w.notify("command", cmd("kick"))
		require.Equal(t, "special", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the verbosity step")

		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("trip counts as the special move", func(t *testing.T) {
		w := newQuestWalker(q).at("strike")
		w.notify("command", cmd("trip"))
		require.Equal(t, "special", w.p.current)
	})

	t.Run("a refused kick or consider does not count", func(t *testing.T) {
		w := newQuestWalker(q).at("strike")
		w.notify("command_issued", cmd("kick"))
		w.notify("command_issued", cmd("trip"))
		require.Equal(t, "strike", w.p.current, "command_issued fires on refusals too")
		w = newQuestWalker(q).at("wield")
		w.notify("command_issued", cmd("consider"))
		require.Equal(t, "wield", w.p.current)
	})

	t.Run("players saved on each step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current, "saved on start: the hint says talk to Vorn, and hearing him advances")

		w = newQuestWalker(q).at("met")
		w.notify("command", equipCudgel)
		require.Equal(t, "wield", w.p.current)

		w = newQuestWalker(q).at("wield")
		w.notify("command", cmd("consider"))
		require.Equal(t, "sized_up", w.p.current)

		w = newQuestWalker(q).at("strike")
		w.notify("command", cmd("kick"))
		require.Equal(t, "special", w.p.current)

		w = newQuestWalker(q).at("special")
		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)

		w = newQuestWalker(q).at("verbosity")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the old consider step moves saved players onto special", func(t *testing.T) {
		raw := string(repoFile(t, "_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
		require.Contains(t, raw, "32-consider -> 32-special")
	})

	t.Run("Vorn's talk step", func(t *testing.T) {
		checkArriveTopic(t, "vorn", "9108.yaml", q, martialVorn, "drill_arrive", []string{"30-end", "31-end"})
	})

	t.Run("Vorn's greeting names the step's one command", func(t *testing.T) {
		f := "32-first_blood.yaml"
		pre := []string{"30-end", "31-end"}
		carriesCudgel := func(p *linearPlayer) { p.items[martialCudgel] = true }
		checkTopicStepsWith(t, "vorn", "9108.yaml", carriesCudgel, []topicStep{
			{f, 32, "met", "type wield cudgel", pre, "progress_start", false},
			{f, 32, "wield", "type consider dummy", pre, "progress_wield", false},
			{f, 32, "sized_up", "type attack dummy", pre, "progress_sized_up", false},
			{f, 32, "strike", "type kick dummy", pre, "progress_strike", false},
			{f, 32, "special", "type set combatverbosity", pre, "progress_special", false},
			{f, 32, "verbosity", "", pre, "train_turnin", true},
		})
	})

	t.Run("a lost cudgel is replaced once", func(t *testing.T) {
		vorn := loadCouleeTopics(t, "9108.yaml")
		found := 0
		for i := range vorn.Topics {
			if tp := &vorn.Topics[i]; tp.GivesItem == martialCudgel && tp.GrantsQuest == "" {
				found++
			}
		}
		require.Equal(t, 1, found, "Vorn replaces a lost cudgel mid-quest through exactly one topic")
		n := newNpcTalker(vorn, playerOn(q, "start", "30-end"))
		checkHandout(t, n, "cudgel_replace", "cudgel_have", martialCudgel)
	})
}

func TestQuest33HoldTheWash_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "33-hold_the_wash.yaml", 33)
	steps := martialStepsKept(t, q, []string{"start", "scout", "squatter", "end"})
	require.Equal(t, []string{"start", "met", "scout", "squatter", "end"}, steps)
	turnIn := turnInRequires(t, "9112.yaml", "wash_turnin")
	require.Equal(t, []string{"33-squatter"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: martialGarve, Topic: "wash_arrive", RoomId: martialHollow}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "start", w.p.current, "a scout before Garve has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "met", w.p.current, "the squatter before the scout must not skip the scout")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "scout", w.p.current, "a second scout is not the squatter")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "squatter", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("players saved on each step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w = newQuestWalker(q).at("met")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)

		w = newQuestWalker(q).at("scout")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5236})
		require.Equal(t, "squatter", w.p.current)

		w = newQuestWalker(q).at("squatter")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
	})

	t.Run("Vorn offers it and Garve no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9108.yaml", "wash_offer", q, "32-end", "9112.yaml")
	})

	t.Run("Garve's talk step", func(t *testing.T) {
		checkArriveTopic(t, "garve", "9112.yaml", q, martialGarve, "wash_arrive", []string{"30-end", "32-end"})
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		f := "33-hold_the_wash.yaml"
		pre := []string{"30-end", "32-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 33, "met", "type attack scout", pre, "wash_progress_start", false},
			{f, 33, "scout", "type attack squatter", pre, "wash_progress_scout", false},
			{f, 33, "squatter", "", pre, "wash_turnin", true},
		})
	})
}

func TestQuest34TakeTheTower_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "34-take_the_tower.yaml", 34)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type attack captain")
	turnIn := turnInRequires(t, "9112.yaml", "tower_in")
	require.Equal(t, []string{"34-slain"}, turnIn)

	// The kill is one action with no flag condition; the boon the player
	// chose still lands at the kill, through the quest_granted 34-slain
	// triggers.
	for boon, stat := range map[string]string{"might": "strength", "finesse": "dexterity"} {
		w := newQuestWalker(q).at("start")
		w.p.flags["34-boon"] = boon
		w.notify("mob_death", EventDetails{MobId: martialCaptain, RoomId: 5242})
		require.Equal(t, "slain", w.p.current, "the %s path: the captain's death moves the player onto slain", boon)
		require.Equal(t, []StatIncreaseCall{{Stat: stat, Amount: 3}}, w.ctx.increasedStats,
			"the %s path trains %s once, at the kill", boon, stat)
		require.False(t, holdsAll(w.p, []string{"34-end"}), "the kill no longer ends the quest")
	}

	pre := []string{"30-end", "32-end", "33-end"}
	might := func(p *linearPlayer) { p.flags["34-boon"] = "might" }
	checkTopicStepsWith(t, "garve", "9112.yaml", might, []topicStep{
		{"34-take_the_tower.yaml", 34, "start", "type attack captain", pre, "tower_progress", false},
		{"34-take_the_tower.yaml", 34, "slain", "", pre, "tower_in", true},
	})
}

func TestQuest53CullTheWash_EndsAtGarve(t *testing.T) {
	q := loadShippedQuest(t, "53-cull_the_wash.yaml", 53)
	require.Equal(t, []string{"start", "firstkill", "culled", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[1].Hint), "squatter", "the second kill is the squatter")
	turnIn := turnInRequires(t, "9112.yaml", "cull_in")
	require.Equal(t, []string{"53-culled"}, turnIn)

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "start", w.p.current, "the squatter first must not count")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "firstkill", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "firstkill", w.p.current, "a second scout is not the squatter")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "culled", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
		require.False(t, holdsAll(w.p, []string{"53-end"}), "the kill no longer ends the run")
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		f := "53-cull_the_wash.yaml"
		pre := []string{"30-end", "32-end", "33-end", "34-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 53, "start", "type attack scout", pre, "cull_progress_start", false},
			{f, 53, "firstkill", "type attack squatter", pre, "cull_progress_firstkill", false},
			{f, 53, "culled", "", pre, "cull_in", true},
		})
	})
}
```

- [ ] **Step 4: Create the bridge test.** Create `internal/questengine/coulee_bridges_test.go`:

```go
package questengine

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The Report to Toke bridges (#375 phase 7): a line's second giver offers its
// bridge only once the finale is handed in, and Crier Toke (9106) takes it.
// Quest names follow the shared naming; the file name is the quest name
// through ConvertForFilename, as the loader demands.
func TestCouleeBridges_GiverTwoOffersTokeTakes(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	t.Run("Fighting", func(t *testing.T) {
		checkBridge(t, toke, 90, "Fighting", "90-report_to_toke__fighting_.yaml",
			"9112.yaml", "report_fighting_offer", "report_fighting_in", "34-end")
	})
}

// checkBridge walks one bridge: the quest's shape, the giver's offer gated
// on the finale's end token, and Toke's hand-in completing it.
func checkBridge(t *testing.T, toke *dialogue.DialogueFile, id int, line, file, giverFile, offerKey, handinKey, after string) {
	t.Helper()
	q := loadShippedQuest(t, file, id)
	require.Equal(t, "Report to Toke ("+line+")", q.Name)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "start"), "type talk toke")

	giver := loadCouleeTopics(t, giverFile)
	offer := giver.FindTopic(offerKey)
	require.NotNil(t, offer, "%s has no topic %q", giverFile, offerKey)
	require.Equal(t, dialogue.KindOffer, offer.Kind)
	require.Equal(t, id, offer.Quest)
	require.Equal(t, []string{after}, offer.QuestRequired, "the bridge opens on the finale's hand-in")
	require.Nil(t, newNpcTalker(giver, playerOn(q, "", "30-end")).eligible(offerKey),
		"no bridge before the finale is handed in")

	n := newNpcTalker(giver, playerOn(q, "", "30-end", after))
	n.choose(t, offerKey)
	require.Equal(t, "start", n.p.current)

	handin := toke.FindTopic(handinKey)
	require.NotNil(t, handin, "Toke has no topic %q", handinKey)
	require.Equal(t, dialogue.KindHandIn, handin.Kind)
	require.Equal(t, []string{fmt.Sprintf("%d-start", id)}, handin.QuestRequired)
	newNpcTalker(toke, n.p).choose(t, handinKey)
	require.True(t, n.p.HasQuest(fmt.Sprintf("%d-end", id)), "Toke's hand-in completes the bridge")
}
```

- [ ] **Step 4b: Pin Toke's quest 1 offers.** Append to `internal/questengine/coulee_bridges_test.go`:

```go

// Toke's quest 1 offers send the player to the line's first giver (#375
// phase 7, controller ruling): the hint names `talk <giver>` and no other
// command, because the quest now begins with that talk.
func TestTokeQuestOneOffers_SendToTheGiver(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	t.Run("Fighting", func(t *testing.T) { checkTokeSends(t, toke, "send_drill", 32, "vorn") })
}

func checkTokeSends(t *testing.T, toke *dialogue.DialogueFile, key string, quest int, npc string) {
	t.Helper()
	tp := toke.FindTopic(key)
	require.NotNil(t, tp, "Toke has no topic %q", key)
	require.Equal(t, dialogue.KindOffer, tp.Kind)
	require.Equal(t, quest, tp.Quest)
	require.Contains(t, squash(tp.Hints), "type talk "+npc, "%s sends the player to %s", key, npc)
	require.Equal(t, 1, typedCount(tp.Hints), "%s names one command: %q", key, tp.Hints)
}
```

- [ ] **Step 5: Run the new tests and see them fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestQuest32|TestQuest33|TestQuest34|TestQuest53|TestCouleeBridges|TestTokeQuestOneOffers'
```

Expected: `FAIL`. Among the messages: `Not equal` on the step lists (`start, met, wield...` against today's `start, wield, sized_up...`), `9108.yaml has no topic "drill_arrive"`, and for the bridge `must be able to read the real quest YAML 90-report_to_toke__fighting_.yaml`.

- [ ] **Step 6: Replace `_datafiles/world/dogmud/quests/32-first_blood.yaml`** with:

```yaml
# Quest 32, First Blood (Pothole Coulee, fighting line, quest 1).
#
# Crier Toke offers it (topic send_drill, dialogue 9106) and hands over the
# Drillyard Cudgel (10043). Drillmaster Vorn (9108) takes the hand-in
# (train_turnin) and then offers Hold the Wash (33).
#
# One command per step, each trigger gated on the step before it (owner
# ruling 2026-10-17):
#   start      go to the Drill Yard and talk to Vorn
#   met        topic_heard on Vorn's drill_arrive topic
#   wield      equip the cudgel (the equip event names its item)
#   sized_up   consider the training dummy, before the fight
#   strike     kill the training dummy (9109)
#   special    kick or trip it
#   verbosity  set combatverbosity
#   end        Vorn's hand-in
# The old consider step (after special) is renamed to special in
# step-ids.txt (#375 phase 7), so a player saved on it resumes at verbosity.

questid: 32
name: First Blood
description: A cudgel, a training dummy, and Drillmaster Vorn's Drill Yard.
  Learn the basics of combat there before you take your bruises somewhere
  they can kill you.
secret: false

steps:
  - id: start
    destination:
      mob: 9108
      room: 5227
    description: >-
      Crier Toke hands you a cudgel when you take this on and sends you to
      Drillmaster Vorn in the Drill Yard. From Hub Square, go north to the
      Awakening Pool, then east four times to the Drill Yard.
    hint: >-
      From Hub Square, go north, then east four times to the Drill Yard.
      Type talk vorn.
  - id: met
    destination:
      room: 5227
    description: >-
      Vorn has told you how his yard works. First the cudgel goes in your
      hand. If you already hold it, remove it and take it up again so he
      sees you do it.
    hint: >-
      Type wield cudgel.
  - id: wield
    destination:
      room: 5227
    description: >-
      The cudgel is in your hand. Before you fight, size up the training
      dummy in the Drill Yard.
    hint: >-
      Consider tells you how hard a foe is before you fight it. Type
      consider dummy.
  - id: sized_up
    destination:
      room: 5227
    description: >-
      You sized up the training dummy. Consider tells you how dangerous a
      foe is before you start a fight. Now fight the dummy until it falls.
    hint: >-
      Type attack dummy.
  - id: strike
    destination:
      room: 5227
    description: >-
      You beat the training dummy. A new dummy stands up in the yard after
      a short wait. Now try a special move on it. A kick hits hard, and a
      trip puts it on the ground. Either one counts.
    hint: >-
      Type kick dummy.
  - id: special
    destination:
      room: 5227
    description: >-
      You used a special move. Kicks and trips cost stamina, so use them
      when they help most. Next, choose how much combat detail you see.
      Full shows every swing, medium shows only the hits, and light shows
      one line a round.
    hint: >-
      Combat verbosity sets how much fight text you see. Type set
      combatverbosity full. For more, type help combatverbosity.
  - id: verbosity
    destination:
      mob: 9108
      room: 5227
    description: >-
      You chose your combat detail. You can change it at any time. The
      Drill Yard has taught you what it can. Tell Vorn.
    hint: >-
      Drillmaster Vorn waits in the Drill Yard. Type talk vorn.
  - id: end
    description: >-
      Drillmaster Vorn has signed off on your first fight, and he has work
      for you out in the wash.

rewards:
  gold: 15
  skillinfo: "weapon-combat:1"
  actor: >-
    Vorn turns your performance over in his head for a long moment,
    the way a smith eyes a finished blade for flaws. Then he nods
    once, short, like it costs him something. "Passable. You did not
    freeze, you did not panic, and you remembered to think between
    swings. That is more than most manage on the first day." He
    presses a few coins into your hand. "There is real work out in
    the wash. Ask me about it when you are ready."
  observer: "Vorn nods at {actor} with a gruff, brief approval."

triggers:
  # met: hearing Vorn's drill_arrive topic.
  - event: topic_heard
    mob: 9108
    topic: drill_arrive
    conditions:
      has: ["32-start"]
      missing: ["32-met"]
    actions:
      - grant: "32-met"
      - actor: "Vorn has your measure now. Take up the cudgel: type wield
          cudgel."

  # wield: the cudgel (10043); the equip event names its item.
  - event: command
    command: equip
    item: 10043
    conditions:
      has: ["32-met"]
      missing: ["32-wield"]
    actions:
      - grant: "32-wield"
      - actor: "The cudgel sits in your hand. Before you fight, size up
          the training dummy: type consider dummy."

  # sized_up: consider, before the fight. The command event goes out only
  # once its target resolves, so consider while the dummy is down does not
  # count.
  - event: command
    command: consider
    conditions:
      has: ["32-wield"]
      missing: ["32-sized_up"]
    actions:
      - grant: "32-sized_up"
      - actor: "Now you know what you face. Fight it: type attack dummy."

  # strike: the training dummy (9109) falls.
  - event: mob_death
    mob: 9109
    conditions:
      has: ["32-sized_up"]
      missing: ["32-strike"]
    actions:
      - grant: "32-strike"
      - actor: "The training dummy shudders and goes still. Your arms
          remember the rhythm of it: weight, angle, follow through. A new
          dummy stands up soon. When it does, type kick dummy."

  # special: kick or trip. Both send the command event only once the move
  # resolves against a target, so a refused move does not count.
  - event: command
    command: kick
    conditions:
      has: ["32-strike"]
      missing: ["32-special"]
    actions:
      - grant: "32-special"
      - actor: "That is a special move. Now choose how much combat detail
          you see: type set combatverbosity full."

  - event: command
    command: trip
    conditions:
      has: ["32-strike"]
      missing: ["32-special"]
    actions:
      - grant: "32-special"
      - actor: "That is a special move. Now choose how much combat detail
          you see: type set combatverbosity full."

  # verbosity: set combatverbosity fires a logical "combatverbosity" command
  # event (set.go).
  - event: command
    command: combatverbosity
    conditions:
      has: ["32-special"]
      missing: ["32-verbosity"]
    actions:
      - grant: "32-verbosity"
      - actor: "Your combat detail is set. Report to Drillmaster Vorn: type
          talk vorn."
```

- [ ] **Step 7: Replace `_datafiles/world/dogmud/quests/33-hold_the_wash.yaml`** with:

```yaml
# Quest 33, Hold the Wash (Pothole Coulee, fighting line, quest 2).
#
# Drillmaster Vorn (9108) offers it (wash_offer) once First Blood (32) is
# handed in. Caravan Guard Garve (9112) takes the hand-in (wash_turnin) and
# then offers Take the Tower (34).
#   start     go to Squatter's Hollow and talk to Garve
#   met       topic_heard on Garve's wash_arrive topic
#   scout     kill a bandit scout (9110)
#   squatter  kill a bandit squatter (9111), after the scout
#   end       Garve's hand-in
#
# Design intent: the wash can kill a careless newbie. Garve's arrive topic
# names flee and the Mending Hut so the death loop is a taught mechanic.

questid: 33
name: Hold the Wash
description: >-
  Caravan Guard Garve's convoy was ambushed in the wash, and he holds
  Squatter's Hollow with a wounded sword arm. Drillmaster Vorn has sent you
  to help him drive out a bandit scout and then a bandit squatter.
secret: false

steps:
  - id: start
    destination:
      mob: 9112
      room: 5234
    description: >-
      Drillmaster Vorn has sent you to Caravan Guard Garve, who holds
      Squatter's Hollow in the wash. From the Drill Yard, go north to the
      Sparring Circle, east three times to the Lower Wash, then south.
    hint: >-
      From the Drill Yard, go north, east three times, then south to
      Squatter's Hollow. Type talk garve.
  - id: met
    destination:
      mob: 9110  # the nearest Bandit Scout the player can see; else the rooms that spawn one
    description: >-
      Garve needs the wash cleared: a bandit scout first, then a bandit
      squatter. The nearest scout is in the Lower Wash, just north of
      Squatter's Hollow. If a fight goes badly, type flee to get away. If
      you fall, you wake at the Mending Hut, not in a grave, and you can
      walk back and try again.
    hint: >-
      Go north from Squatter's Hollow to the Lower Wash and type attack
      scout. If it is dark, type cast glow first.
  - id: scout
    destination:
      room: 5235  # Cracked Cistern: Bandit Squatter (mob 9111)
    description: >-
      One bandit scout is down. The squatter camps in the Cracked
      Cistern, east of Squatter's Hollow. Type flee if the fight turns
      against you. When a bandit falls, its gear stays on the corpse:
      type loot corpse to take it.
    hint: >-
      From Squatter's Hollow, go east to the Cracked Cistern and type
      attack squatter.
  - id: squatter
    destination:
      mob: 9112
      room: 5234
    description: >-
      The bandit squatter is down too. Garve waits in Squatter's Hollow,
      west of the Cracked Cistern.
    hint: >-
      Go back to Garve in Squatter's Hollow and type talk garve.
  - id: end
    description: >-
      Garve can hold the wash now. He has one more job for you, the bandit
      tower further up.

rewards:
  skillinfo: "weapon-combat:1"
  itemid: 20087
  gold: 25
  actor: >-
    Garve presses a purse into your hand and holds your gaze a moment
    longer than is strictly necessary, like someone revising his estimate
    upward. The leggings he slides from the pack beside him are well made,
    cut for movement, and they fit.

triggers:
  # met: hearing Garve's wash_arrive topic.
  - event: topic_heard
    mob: 9112
    topic: wash_arrive
    conditions:
      has: ["33-start"]
      missing: ["33-met"]
    actions:
      - grant: "33-met"
      - actor: >-
          Garve has told you what the wash needs. Start with the scout. Go
          north to the Lower Wash and type attack scout.

  # scout: a bandit scout (9110).
  - event: mob_death
    mob: 9110
    conditions:
      has: ["33-met"]
      missing: ["33-scout"]
    actions:
      - grant: "33-scout"
      - actor: >-
          The bandit scout goes down. His gear stays on the body until
          you take it: type loot corpse to strip it. Next is the
          squatter. From Squatter's Hollow, go east to the Cracked
          Cistern and type attack squatter.

  # squatter: a bandit squatter (9111), after the scout.
  - event: mob_death
    mob: 9111
    conditions:
      has: ["33-scout"]
      missing: ["33-squatter"]
    actions:
      - grant: "33-squatter"
      - actor: >-
          The bandit squatter drops, and he was wearing something worth
          having. Loot the corpse before you go. Then go back to Garve
          in Squatter's Hollow and type talk garve.
```

- [ ] **Step 8: Replace `_datafiles/world/dogmud/quests/34-take_the_tower.yaml`** with:

```yaml
# Quest 34, Take the Tower (Pothole Coulee, fighting line finale).
#
# Caravan Guard Garve (9112) offers it after Hold the Wash (33), through
# tower_might or tower_finesse, which also set the 34-boon flag:
#   might    a permanent Strength gain at the kill
#   finesse  a permanent Dexterity gain at the kill
#
# Steps (#375 phase 7, one action each):
#   start  kill the bandit captain (9115) in the Watch Room (5242)
#   slain  report to Garve
#   end    Garve's hand-in tower_in, which pays the rewards
# The kill grants 34-slain with no flag condition, so it is one action. The
# boon rides on the two quest_granted 34-slain triggers below, which grant
# nothing, so the stat still lands at the kill.

questid: 34
name: Take the Tower
description: >-
  The bandits hold a ruined watchtower at the head of the wash, and their
  captain holds the watch room at the top of it. Choose how Garve hones you,
  climb the tower, put the captain down, and tell Garve.
secret: false

flags:
  - key: boon
    values: [might, finesse]
    description: "Capstone boon path chosen at Garve: might raises Strength, finesse raises Dexterity."

steps:
  - id: start
    destination:
      room: 5242
    description: >-
      Garve has set you on the tower. From Squatter's Hollow, go east to
      the Cracked Cistern, northeast to the Upper Wash, then east twice to
      the Broken Gate. Go north to the Tower Base and up twice to the Watch
      Room, where the bandit captain waits. A bruiser holds the Broken
      Gate, and a lieutenant waits in the barracks east of it. Type consider
      on a foe first to size it up, and type flee if a fight turns against
      you.
    hint: >-
      In the Watch Room at the top of the tower, type attack captain.
      If it is dark, type cast glow first.
  - id: slain
    destination:
      mob: 9112
      room: 5234
    description: >-
      The captain has fallen and the tower is yours. Garve waits in
      Squatter's Hollow to hear it from you.
    hint: >-
      Go back down to Caravan Guard Garve in Squatter's Hollow. Type talk
      garve.
  - id: end
    description: >-
      The captain has fallen and the tower is yours. The coulee is a harder
      place for bandits than it was this morning.

rewards:
  itemid: 10044
  skillinfo: "weapon-combat:1,unarmed-combat:1"
  gold: 50
  actor: >-
    Garve hears you out without a word. Then he takes the captain's blade
    from you, turns it once in his good hand, and gives it back. "You took
    it, you keep it. It has changed owners by violence before, and it found
    a better one this time." He counts coin into your palm. "Whatever she
    was guarding is the coulee's question now, not hers."
  observer: "Garve hands {actor} back a well-kept blade and a purse of coin."

triggers:
  # slain: the bandit captain (9115) falls. No flag condition: one action.
  - event: mob_death
    mob: 9115
    conditions:
      has: ["34-start"]
      missing: ["34-slain"]
    actions:
      - grant: "34-slain"
      - actor: >-
          The captain falls, and the watch room goes quiet. Her blade lies
          where she dropped it, the one well-kept thing in this ruin. Take
          the news down to Garve: type talk garve when you reach him.

  # The might boon, at the kill: a permanent Strength gain. Grants nothing.
  - event: quest_granted
    quest_token: 34-slain
    conditions:
      has_flag: {"34-boon": "might"}
    actions:
      - train_stat:
          stat: strength
          amount: 3
      - actor: >-
          The fight leaves something behind in your arms and back: a
          settled, lasting strength, earned the hard way at the top of a
          tower.

  # The finesse boon, at the kill: a permanent Dexterity gain. Grants nothing.
  - event: quest_granted
    quest_token: 34-slain
    conditions:
      has_flag: {"34-boon": "finesse"}
    actions:
      - train_stat:
          stat: dexterity
          amount: 3
      - actor: >-
          The fight leaves something behind in your hands and feet: a
          settled, lasting quickness, earned the hard way at the top of a
          tower.
```

- [ ] **Step 9: Replace `_datafiles/world/dogmud/quests/53-cull_the_wash.yaml`** with:

```yaml
# Quest 53, Cull the Wash (Pothole Coulee, fighting line repeatable).
#
# Caravan Guard Garve (9112) offers it (cull_offer) once Take the Tower (34)
# is handed in. Repeatable with a cooldown; completion clears progress.
#   start      kill a bandit scout (9110)
#   firstkill  kill a bandit squatter (9111), after the scout
#   culled     report to Garve
#   end        Garve's hand-in cull_in (#375 phase 7: every run ends at Garve)

questid: 53
name: Cull the Wash
description: >-
  Garve always has more work clearing bandits out of the wash. Kill a
  bandit scout, then a bandit squatter, and go back to Garve for the pay.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5232  # Lower Wash, north of Squatter's Hollow: Bandit Scout (mob 9110)
    description: >-
      Garve has asked you to thin out the bandits in the wash again: a
      bandit scout first, then a bandit squatter. The nearest scout is in
      the Lower Wash, just north of Squatter's Hollow.
    hint: >-
      Go north from Squatter's Hollow to the Lower Wash and type attack
      scout. If it is dark, type cast glow first.
  - id: firstkill
    destination:
      room: 5235  # Cracked Cistern: Bandit Squatter (mob 9111)
    description: >-
      One bandit scout is down. Now the bandit squatter, in the Cracked
      Cistern east of Squatter's Hollow.
    hint: >-
      From Squatter's Hollow, go east to the Cracked Cistern and type
      attack squatter.
  - id: culled
    destination:
      mob: 9112
      room: 5234
    description: >-
      The scout and the squatter are down. Garve pays in Squatter's Hollow.
    hint: >-
      Go back to Caravan Guard Garve in Squatter's Hollow. Type talk garve.
  - id: end
    description: >-
      You thinned the wash again. Garve will have more work after a while.

rewards:
  gold: 6
  skillinfo: "weapon-combat:1"
  actor: >-
    Garve counts a few coins into your palm with his good hand. "The wash
    is quiet again. Come back when it is not. It never stays quiet long."
  observer: ""

triggers:
  - event: mob_death
    mob: 9110
    conditions:
      has: ["53-start"]
      missing: ["53-firstkill"]
    actions:
      - grant: "53-firstkill"
      - actor: >-
          One scout down. Now the squatter: from Squatter's Hollow, go
          east to the Cracked Cistern and type attack squatter.

  - event: mob_death
    mob: 9111
    conditions:
      has: ["53-firstkill"]
      missing: ["53-culled"]
    actions:
      - grant: "53-culled"
      - actor: >-
          The squatter drops. Go back to Garve in Squatter's Hollow and
          type talk garve for your pay.
```

- [ ] **Step 10: Create `_datafiles/world/dogmud/quests/90-report_to_toke__fighting_.yaml`:**

```yaml
# Quest 90, Report to Toke (Fighting) (Pothole Coulee, fighting line bridge).
#
# Caravan Guard Garve (9112) offers it (report_fighting_offer) once Take the
# Tower (34) is handed in. Crier Toke (9106) takes the hand-in
# (report_fighting_in). Bridges 90 to 97 share this shape, one per line, and
# the completer (quest 89) gates on all eight hand-ins. The file name is the
# quest name through ConvertForFilename, as the loader requires.

questid: 90
name: Report to Toke (Fighting)
description: >-
  Caravan Guard Garve has asked you to carry the news of the bandit tower to
  Crier Toke in Hub Square.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # follow Crier Toke (schedule pothole_toke)
      room: 5203  # fallback when Crier Toke is not spawned
    description: >-
      The wash is held and the bandit tower has fallen. Crier Toke, who sent
      you to Drillmaster Vorn, waits in Hub Square for the news.
    hint: >-
      Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - id: end
    description: >-
      Toke has your news, and the whole coulee will have it by nightfall.
```

- [ ] **Step 11: Replace `_datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml`** (Caravan Guard Garve) with:

```yaml
mobid: 9112
zone: Pothole Coulee
defaultMood: neutral
greetings:
- text: You are not one of theirs. Good. Sit if you need to. I am not going anywhere
    on this arm.
  moods:
  - friendly
  - neutral
opener:
  text: Garve. Road guard. Was road guard, until the ambush. My convoy made Squatter's
    Hollow two nights back and the bandits were waiting in the wash. Most of the caravan
    pushed through. I did not. This arm says I hold the hollow. It does not say I
    clear the wash alone.
  hints: Choose a line below if you want to hear about the wash, the ambush, or
    what he needs. His eyes catch the light strangely, and one line below is
    about them.
  variants:
  - questExcluded:
    - 30-end
    text: You have the look of someone who has not been through the pool yet. I will
      talk to anyone who sits still, but the folk in this coulee will take you more
      seriously once you are Opened. Cleric Hadwen, at the water, you cannot miss
      it. Come back after.
    hints: Find Cleric Hadwen at the reflecting pool and complete the Awakening before
      pushing out into the wash.
  - questRequired:
    - 30-end
    questExcluded:
    - 32-end
    text: Opened, good. But you are still carrying yourself like someone who has not
      put that to work yet. The wash is not a forgiving place to find out for the
      first time. There is a drillmaster, Vorn, in the Drill Yard, back west along
      the wash. He turns raw into ready. Come back when you have worked with him.
    hints: 'Find Drillmaster Vorn in the Drill Yard: go north to the Lower Wash, west
      three times to the Sparring Circle, then south. Train with him before you fight
      in the wash.'
  - questRequired:
    - 32-end
    questExcluded:
    - 33-start
    - 33-end
    text: Trained and still standing, by the look of you. Vorn sends me the ones he
      trusts with the wash. If he has not sent you yet, ask him about it.
    hints: 'Go back to Drillmaster Vorn in the Drill Yard: north to the Lower Wash,
      west three times to the Sparring Circle, then south. Type talk vorn.'
  - questRequired:
    - 33-start
    questExcluded:
    - 33-met
    - 33-end
    text: Vorn sent you? Good. Sit a moment and I will tell you what the wash is and
      what I need from it.
    hints: Choose Drillmaster Vorn sent me to help you below.
  - questRequired:
    - 33-met
    questExcluded:
    - 33-scout
    - 33-end
    text: Still in the field. Good. Scout first. There is one in the Lower Wash, just
      north of here. If a fight turns on you, flee. I mean it, not as advice but as
      the correct move. Fall and you wake at the Mending Hut. That is how the coulee
      works, not the end of anything.
    hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  - questRequired:
    - 33-scout
    questExcluded:
    - 33-squatter
    - 33-end
    text: 'The scout is down. Good. Now the squatter. He has dug into the old cistern,
      east of this hollow. Same rule as before: if it turns on you, flee.'
    hints: From Squatter's Hollow, go east to the Cracked Cistern and type attack
      squatter.
  - questRequired:
    - 33-squatter
    questExcluded:
    - 33-end
    text: You have the look of someone who has been busy. Tell me how it went out
      there.
    hints: Choose Hold the Wash below to report.
  - questRequired:
    - 33-end
    questExcluded:
    - 34-start
    - 34-end
    text: 'You held the wash. I will not forget that. Now I will ask for the hard
      one. The bandits have a tower at the head of the wash, past the cistern (old
      stone, not their work), and a captain holding the watch room at the top. She
      is the reason the rest of them have spine. Break her and you break the whole
      nest. Before you go, tell me how you fight, and I will drill you toward it while
      you climb: toward might, or toward finesse.'
    hints: Garve is offering the tower. Choose How do I take the tower? below, then
      choose Take the Tower with your edge, might (Strength) or finesse (quickness).
  - questRequired:
    - 34-start
    questExcluded:
    - 34-slain
    - 34-end
    text: Still down here? The captain is still up there. Go east to the cistern,
      northeast to the Upper Wash, and east twice to the Broken Gate. The tower is
      north of the gate. Climb the stairs to the watch room and put her down. A bruiser
      holds the gate, and a lieutenant sits in the barracks east of it. Flee and come
      back if you have to. The Mending Hut still has your name.
    hints: In the Watch Room at the top of the tower, type attack captain.
  - questRequired:
    - 34-slain
    questExcluded:
    - 34-end
    text: You came down from that tower on your own feet. Tell me how it went up
      there.
    hints: Choose Take the Tower below to report.
  - questRequired:
    - 53-start
    questExcluded:
    - 53-firstkill
    - 53-end
    text: Scout first, same as before. There is usually one in the Lower Wash, just
      north of here.
    hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  - questRequired:
    - 53-firstkill
    questExcluded:
    - 53-culled
    - 53-end
    text: One scout down. Now the squatter in the old cistern, east of here. Then
      come back and tell me.
    hints: From Squatter's Hollow, go east to the Cracked Cistern and type attack
      squatter.
  - questRequired:
    - 53-culled
    questExcluded:
    - 53-end
    text: The wash sounds quieter. Tell me what you put down out there.
    hints: Choose Cull the Wash below to report.
  - questRequired:
    - 34-end
    questExcluded:
    - 90-start
    - 90-end
    text: You took the tower. I heard it go quiet up there and I knew. Whatever the
      captain was guarding is the coulee's problem now, not just mine, and that is a
      better class of problem. Crier Toke will want the news from your own mouth. He
      sent you down this road in the first place.
    hints: Choose Report to Toke (Fighting) below to carry the news to Toke.
  - questRequired:
    - 90-start
    questExcluded:
    - 90-end
    text: Still here? Toke is waiting on that news in Hub Square. Go and tell him.
    hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - questRequired:
    - 34-end
    text: You came in raw. You are not raw anymore. The road out runs east when you
      want it. Until then, the coulee is glad to have you, and the wash always has
      more bandits.
    hints: You have finished the fighting trail. When you want more work in the wash,
      choose Cull the Wash below.
topics:
- key: wash_arrive
  kind: info
  label: Drillmaster Vorn sent me to help you.
  text: 'Here is what I am asking. There is a scout in the Lower Wash, just north
    of here, and more of them up the bend. East of this hollow, a squatter has claimed
    the old cistern. Between them they own the lower wash and I cannot push them without
    this arm. You can. Scout first, then the squatter, then come back to me. One piece
    of advice that is not optional: if a fight turns against you, use flee. The wash
    will kill a careless fighter, and dead is not what either of us needs right now.
    And if you do go down, if the worst happens, you wake at the Mending Hut, back
    in the coulee, two rooms west of the Drill Yard. Not in a ditch. Not gone. You
    walk back and try again. That is how the coulee works. One more thing. Strip what
    they carry while you are at it. Gear does a dead bandit no good, and it will do
    you plenty.'
  hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  questRequired:
  - 33-start
  questExcluded:
  - 33-met
  - 33-end
  moodChange: neutral
- key: wash_turnin
  kind: handin
  quest: 33
  label: Hold the Wash
  playerLine: The scout and the squatter are both down.
  text: Both of them. You actually did it. I watched the bend all morning and nothing
    moved on it. I just did not let myself believe yet. The hollow is holdable now.
    Take this, it came off the last guard the convoy lost and it fits better on
    legs that are still moving. The coin is mine. I can spare it. The tower further
    up the wash still stands. I will have a clearer read on it once I can think past
    this arm. Come back when you want to look in that direction.
  hints: Quest complete. You receive leggings and a sharper edge in a fight. The bandit
    tower further up the wash may have more work waiting. Come back to Garve when
    you are ready.
  questRequired:
  - 33-squatter
  questExcluded:
  - 33-end
  grantsQuest: 33-end
  moodChange: friendly
- key: wash_progress_start
  kind: info
  label: Where do I find the scout?
  text: Scout first. There is one in the Lower Wash, just north of here. If the fight
    turns on you, flee. If you fall, you wake at the Mending Hut and walk back. Then
    the squatter in the cistern, east of here.
  hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  questRequired:
  - 33-met
  questExcluded:
  - 33-scout
  - 33-end
- key: wash_progress_scout
  kind: info
  label: The scout is down. Where is the squatter?
  text: The scout is down. Now the squatter. He holds the old cistern, east of this
    hollow. If the fight turns on you, flee.
  hints: From Squatter's Hollow, go east to the Cracked Cistern and type attack squatter.
  questRequired:
  - 33-scout
  questExcluded:
  - 33-squatter
  - 33-end
# The tower pitch comes first: the two Take the Tower offers (might and
# finesse) need it heard, so the player hears what the edges mean before
# the two same-titled offers appear, told apart by their player lines.
- key: tower_offer
  kind: info
  label: How do I take the tower?
  text: 'The tower is the last of it. Stone walls at the head of the wash, a crew
    dug in (a bruiser, a lieutenant) and above them a captain who has held the
    watch room long enough to call it hers. Take her down and the rest scatter. But
    go in honed. Tell me your edge: might, if you mean to overpower her, or finesse,
    if you mean to outpace her. Choose, and I will drill it into you on your way up.'
  hints: 'Choose Take the Tower below with the edge you want: might hones Strength,
    finesse hones quickness. The choice holds for this climb.'
  marked: true
  questRequired:
  - 33-end
  questExcluded:
  - 34-start
  - 34-end
  moodChange: neutral
- key: tower_might
  kind: offer
  quest: 34
  label: Take the Tower
  playerLine: I'll take her with might. Hone my strength.
  text: 'Might, then? Then we build the swing: shoulder and back and the will to close
    the distance and end it. Go east to the cistern, northeast to the Upper Wash,
    and east twice to the Broken Gate. The tower is north of the gate. Take the stairs
    up and break her in the watch room. Come back stronger or do not come back small.'
  hints: This is the path of might (Strength). In the Watch Room at the top of the
    tower, type attack captain.
  heardRequired:
  - tower_offer
  questRequired:
  - 33-end
  questExcluded:
  - 34-start
  - 34-end
  grantsQuest: 34-start
  setsQuestFlag:
    key: 34-boon
    value: might
  moodChange: neutral
- key: tower_finesse
  kind: offer
  quest: 34
  label: Take the Tower
  playerLine: I'll take her with finesse. Hone my speed.
  text: 'Finesse, then? Good for fighting someone bigger than you, and she will be.
    We sharpen the feet and the timing: be where the blow is not, and put yours where
    she is. Go east to the cistern, northeast to the Upper Wash, and east twice to
    the Broken Gate. The tower is north of the gate. Climb to the watch room and outpace
    her. Then come tell me how it went.'
  hints: This is the path of finesse (Dexterity). In the Watch Room at the top of
    the tower, type attack captain.
  heardRequired:
  - tower_offer
  questRequired:
  - 33-end
  questExcluded:
  - 34-start
  - 34-end
  grantsQuest: 34-start
  setsQuestFlag:
    key: 34-boon
    value: finesse
  moodChange: neutral
- key: tower_progress
  kind: info
  label: Where is the captain?
  text: The captain holds the watch room at the top of the tower. Go east to the cistern,
    northeast to the Upper Wash, and east twice to the Broken Gate. The tower is north
    of the gate, and the stairs go up from there. Flee if you have to and come back.
  hints: In the Watch Room at the top of the tower, type attack captain.
  questRequired:
  - 34-start
  questExcluded:
  - 34-slain
  - 34-end
- key: tower_in
  kind: handin
  quest: 34
  label: Take the Tower
  playerLine: The captain is dead and the tower is quiet.
  text: Dead. Good. I heard the watch room go quiet and did not trust my own ears.
    You took the hard one, and you came back down to tell it. Whatever she was guarding
    is the coulee's problem now, and that is a better class of problem than bandits
    in my wash. Crier Toke will want to hear this from you.
  hints: Quest complete. Choose Report to Toke (Fighting) below to carry the news to
    Toke.
  questRequired:
  - 34-slain
  questExcluded:
  - 34-end
  grantsQuest: 34-end
  moodChange: friendly
- key: arm_wound
  kind: info
  label: What happened to your arm?
  text: Clean cut, deep enough to count. One of theirs got the angle on me when the
    wagon block went wrong. It will hold, since Sala at the Mending Hut wrapped it right,
    but sword work is out until the sinew knits. A guard without a sword arm is
    a lookout, and I am a very good lookout from this alcove.
  hints: Choose a line below if you would like to hear about the ambush, the
    wash, or his eyes.
- key: ambush_lore
  kind: info
  label: How did the ambush happen?
  text: Two nights back. We were moving through after dark (foolish, but the schedule
    was tight) and the bandits had the bend already. Four wagons. Three pushed through.
    One did not. I was riding the tailgate of the third when the block happened. I
    covered the gap. The wagons made it. I did not make it far.
  hints: Choose a line below to hear about the wash, the arm, or his eyes.
- key: eyes_lambent
  kind: info
  label: Why do your eyes catch the light?
  text: The road. Years of it, out past the settlements, past the lamplight, where
    the dark is serious about itself. Things shift in people who spend enough time
    in the real dark. My eyes started catching the light differently about four years
    in. I stopped mentioning it when I stopped being surprised by it.
  hints: Choose a line below to learn about the convoy or the wash.
# Until the wash is held; after that the tower pitch above takes over.
- key: tower_hint
  kind: info
  label: What is that stone tower up the wash?
  text: There is a structure up the wash, past the cistern. Stone walls, old work,
    not something the bandits built. They are using it, which means they did not just
    camp here by chance. What they are waiting for, or who they answer to, I cannot
    tell you yet. Once this arm lets me think about something other than this arm,
    I will have a better answer.
  hints: Clear the wash first. Garve may have more to offer about the tower when you
    come back.
  questExcluded:
  - 33-end
# The bridge comes before the repeatable, so the menu lists it first.
- key: report_fighting_offer
  kind: offer
  quest: 90
  label: Report to Toke (Fighting)
  playerLine: Should I take the news of the tower to Toke?
  text: Do that. Toke sent you down this road, and he likes his news firsthand. He
    keeps to Hub Square, south of the Awakening Pool, and he shouts the day's word
    to anyone who stands still. Tell him the wash is held and the tower is ours.
  hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk toke.
  questRequired:
  - 34-end
  questExcluded:
  - 90-start
  - 90-end
  grantsQuest: 90-start
  moodChange: friendly
- key: cull_offer
  kind: offer
  quest: 53
  label: Cull the Wash
  playerLine: Do you want the wash thinned out again?
  text: There is always more work in the wash. Thin them out again, a scout first,
    then the squatter, same as before. Then come back to me and the coin is yours.
  hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  questRequired:
  - 34-end
  questExcluded:
  - 53-start
  - 53-firstkill
  - 53-end
  grantsQuest: 53-start
- key: cull_progress_start
  kind: info
  label: Where do I start the cull?
  text: Scout first, same as before. There is usually one in the Lower Wash, just
    north of here. Then the squatter in the cistern. Then come back to me for the
    pay.
  hints: Go north from Squatter's Hollow to the Lower Wash and type attack scout.
  questRequired:
  - 53-start
  questExcluded:
  - 53-firstkill
  - 53-end
- key: cull_progress_firstkill
  kind: info
  label: One scout is down. What is next?
  text: One scout down. Now the squatter in the old cistern, east of here. Then come
    back to me.
  hints: From Squatter's Hollow, go east to the Cracked Cistern and type attack squatter.
  questRequired:
  - 53-firstkill
  questExcluded:
  - 53-culled
  - 53-end
- key: cull_in
  kind: handin
  quest: 53
  label: Cull the Wash
  playerLine: The scout and the squatter are down again.
  text: Good. That buys the hollow a few quiet days. Here is your pay. Come back when
    the wash fills up again, and it will.
  hints: Quest complete. Garve will have more work in the wash after a while.
  questRequired:
  - 53-culled
  questExcluded:
  - 53-end
  grantsQuest: 53-end
  moodChange: friendly
```

- [ ] **Step 12: Edit Vorn (`_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`).** Four Edit calls. Do not touch the opener variant gated `questRequired: [30-end]` / `questExcluded: [32-start, 32-end]` (plan 2's). If an `old_string` below no longer matches, stop and report which one.

Edit 12a. Replace:

```yaml
  - questRequired:
    - 32-start
    questExcluded:
    - 32-wield
    - 32-end
    text: You are not done. That cudgel goes in your hand, not your pack. If it is
      already in your hand, put it away and take it up again so I can see you do it.
    hints: Type wield cudgel.
```

with:

```yaml
  - questRequired:
    - 32-start
    questExcluded:
    - 32-met
    - 32-end
    text: Toke sent you, with a cudgel by the look of it. Good. Tell me why you came,
      and I will tell you how this yard works.
    hints: Choose Crier Toke sent me to learn to fight below.
  - questRequired:
    - 32-met
    questExcluded:
    - 32-wield
    - 32-end
    text: You are not done. That cudgel goes in your hand, not your pack. If it is
      already in your hand, put it away and take it up again so I can see you do it.
    hints: Type wield cudgel.
```

Edit 12b. Replace:

```yaml
  - questRequired:
    - 32-end
    text: You are through here. Garve is running caravan guard work out in the wash.
      Go north to the Sparring Circle and follow the wash east. He waits in Squatter's
      Hollow, south of the Lower Wash. That is where the real schooling starts. Set
      your home first. You will want a fallback point.
    hints: Type sethome coulee first to make this your home, where you wake if you
      die.
topics:
```

with:

```yaml
  - questRequired:
    - 32-end
    questExcluded:
    - 33-start
    - 33-end
    text: You passed my yard. Garve, a caravan guard, holds a hollow out in the wash
      on one good arm, and he needs a fighter. Say the word and I will send you.
    hints: Choose Hold the Wash below to take the job.
  - questRequired:
    - 33-start
    questExcluded:
    - 33-met
    - 33-end
    text: Garve is waiting. North to the Sparring Circle, east three times along the
      wash, then south into Squatter's Hollow.
    hints: From the Drill Yard, go north, east three times, then south to Squatter's
      Hollow. Type talk garve.
  - questRequired:
    - 32-end
    text: You are through here. The wash is Garve's business now. Set your home first
      if you have not. You will want a fallback point.
    hints: Type sethome coulee to make this your home, where you wake if you die.
topics:
```

Edit 12c. Replace the whole `train_turnin` topic (from `- key: train_turnin` through its `grantsQuest: 32-end` line) with the hand-in below, followed by the two new topics:

```yaml
- key: train_turnin
  kind: handin
  quest: 32
  label: First Blood
  playerLine: I've worked through your whole checklist.
  text: Good. You hit the dummy, used your feet, read your opponent, and tuned your
    output. That is the checklist. You are not a soldier yet, but you are not dead
    weight either. Take the coin. And if you want work that matters, there is a caravan
    guard out in the wash who could use someone who knows which end of a cudgel to
    hold. Ask me about it.
  hints: Quest complete. Choose Hold the Wash below to hear what Caravan Guard Garve
    needs.
  questRequired:
  - 32-verbosity
  questExcluded:
  - 32-end
  grantsQuest: 32-end
- key: drill_arrive
  kind: info
  label: Crier Toke sent me to learn to fight.
  text: Then stand straight and listen. That cudgel Toke gave you is nothing fancy,
    nothing that will embarrass you in a real fight. First, take it in your hand. Then
    consider the training dummy in this yard, to see what you are walking into. Then
    hit it until it stops hitting back. After that, kick it or trip it once, to learn
    that you have options beyond the basic swing. Last, set combatverbosity to your
    taste. It controls how much combat detail you see. Then come back to me and report.
  hints: Type wield cudgel.
  questRequired:
  - 32-start
  questExcluded:
  - 32-met
  - 32-end
- key: wash_offer
  kind: offer
  quest: 33
  label: Hold the Wash
  playerLine: Do you want me to help Garve hold the wash?
  text: Garve rode caravan guard until the bandits caught his convoy in the wash. He
    holds Squatter's Hollow now on one good arm, with a scout watching him from the
    north and a squatter dug in to the east. Go and see him. North to the Sparring
    Circle, east three times along the wash, then south into the hollow. If the wash
    kills you, you wake at the Mending Hut. Walk back and try again.
  hints: From the Drill Yard, go north, east three times, then south to Squatter's
    Hollow. Type talk garve.
  questRequired:
  - 32-end
  questExcluded:
  - 33-start
  - 33-end
  grantsQuest: 33-start
  moodChange: neutral
```

Edit 12d. In the `progress_start` topic replace:

```yaml
  questRequired:
  - 32-start
  questExcluded:
  - 32-wield
  - 32-end
- key: progress_wield
```

with:

```yaml
  questRequired:
  - 32-met
  questExcluded:
  - 32-wield
  - 32-end
- key: progress_wield
```

- [ ] **Step 13: Add Toke's hand-in (`_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml`).** Toke's file is shared with plans 3b and 4: this plan touches only its four hand-ins and its four quest-1 offers (Step 13b and the matching steps in Tasks 4 to 6). Replace:

```yaml
  grantsQuest: 50-start
  givesItem: 10038
```

with:

```yaml
  grantsQuest: 50-start
  givesItem: 10038
# #375 phase 7: each line's last giver sends the player back to Toke with a
# Report to Toke bridge (quests 90 to 97). Toke takes every bridge hand-in.
- key: report_fighting_in
  kind: handin
  quest: 90
  label: Report to Toke (Fighting)
  playerLine: Garve sends word. The bandit tower has fallen.
  text: Fallen? Hoy! The low mouth will have that across the square before you finish
    your sentence. Garve's tower, taken by someone who could not hold a cudgel a few
    days ago. That is the kind of news I like best. The fighting trail is yours, and
    any mouth you have not walked yet is still open to you.
  hints: Quest complete. Toke can still send you down any trail you have not walked.
  questRequired:
  - 90-start
  questExcluded:
  - 90-end
  grantsQuest: 90-end
  moodChange: friendly
```

(Later tasks insert their hand-ins directly below this one.)

- [ ] **Step 13b: Toke's First Blood offer sends the player to Vorn** (controller ruling: this plan owns the `text` and `hints` of Toke's offers for its lines; route checked: Hub Square 5203 north 5200, east 5202, 5209, 5220, 5227). In `9106.yaml`, inside `send_drill`, replace:

```yaml
  text: Drillmaster Vorn, at the Drill Yard. He turns raw into ready, and you will
    hear him before you see him. I have a cudgel for you, to hold in your hand on
    the way. From this square go north to the pool, then east four times, past the
    shore path, the Mending Hut and the dry coulee mouth, to the yard itself.
  hints: Accept, and Toke hands you a cudgel. Then type wield cudgel, and from Hub
    Square go north, then east four times to the Drill Yard.
```

with:

```yaml
  text: Drillmaster Vorn, at the Drill Yard. He turns raw into ready, and you will
    hear him before you see him. Take this cudgel to him. He will tell you when to
    pick it up. From this square go north to the pool, then east four times, past
    the shore path, the Mending Hut and the dry coulee mouth, to the yard itself.
  hints: Accept, and Toke hands you a cudgel. From Hub Square, go north, then east
    four times to the Drill Yard. Type talk vorn.
```

If the old text no longer matches (plan 2 may have reworded it), replace whatever `text:` and `hints:` the topic has with the new pair above and report it.

- [ ] **Step 14: Step-id manifest (`_datafiles/world/dogmud/quests/step-ids.txt`).** With Edit:
  - Insert `32-met` between `32-end` and `32-sized_up`; insert `33-met` between `33-end` and `33-scout`; insert `34-slain` between `34-end` and `34-start`; insert `53-culled` directly before `53-end`; insert `90-end` and `90-start` after the line `9-start` (the last key line; keys sort as text, so `90-...` follows `9-start`).
  - Delete the line `32-consider` from the key list.
  - If the file has no line `# renamed`, append at the end of the file:

```text

# renamed
32-consider -> 32-special
```

  If `# renamed` already exists, add `32-consider -> 32-special` as a new line at the end of that section.

- [ ] **Step 15: Topic-key manifest (`_datafiles/world/dogmud/dialogue/topic-keys.txt`).** With Edit, add each key in its sorted place among the existing lines: `9106.report_fighting_in`, `9108.drill_arrive`, `9108.wash_offer`, `9112.cull_in`, `9112.report_fighting_offer`, `9112.tower_in`, `9112.wash_arrive`. Delete the line `9112.wash_offer`. Then add the rename (create the section at the end of the file if there is no `# renamed` line yet, else append to it):

```text
# renamed
9112.wash_offer -> 9108.wash_offer
```

- [ ] **Step 16: Delete the fighting backlog lines** from `_datafiles/world/dogmud/quests/loop-backlog.txt` with Edit (one line each, exact text):

```text
chain 32 mob 9112 topic wash_offer
chain 34 mob 9112 topic cull_offer
end 34 quest 34 trigger mob_death mob=9115 grants=34-end
end 34 quest 34 trigger mob_death mob=9115 grants=34-end #2
end 34 step end
end 53 quest 53 trigger mob_death mob=9111 grants=53-end
end 53 step end
one-action 32 quest 32 trigger command command=combatverbosity grants=32-verbosity:prev
one-action 32 step consider:none
one-action 34 quest 34 trigger mob_death mob=9115 grants=34-end #2:cond
one-action 34 quest 34 trigger mob_death mob=9115 grants=34-end:cond
walk-dead-end 32 quest 32 trigger command command=combatverbosity grants=32-verbosity
walk-dead-end 32 step consider
```

Then check none is left (run on its own; expect no output and exit 1):

```bash
grep -nE '^\S+ (32|33|34|53|90) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

- [ ] **Step 17: Run the line's tests and see them pass.**

```bash
gofmt -l internal/
go test -count=1 ./internal/questengine/ -run 'TestQuest32|TestQuest33|TestQuest34|TestQuest53|TestCouleeBridges|TestTokeQuestOneOffers|TestNewbieQuest'
go test -count=1 ./internal/guards/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: `gofmt` prints nothing; every run `ok`; the verbose smoke run lists `--- PASS: TestSmoke_ShippedQuestsPassTheLoop` (and the other smoke tests) as PASS, never `--- SKIP` (a skip means `DOGMUD_BOOT_SMOKE=1` was not set and the run proved nothing). If the loop sweep reports `not in loop-backlog.txt: ...`, the content raised a new finding: fix the content (never add a backlog line). If it reports `matches no finding: delete it`, delete that line only if it is one of the 13 above.

- [ ] **Step 18: Prove the new tests can fail.** Temporarily change `has: ["32-met"]` in the equip trigger of `32-first_blood.yaml` to `has: ["32-start"]`, run `go test -count=1 ./internal/questengine/ -run TestQuest32`, confirm the failure says `wielding before Vorn has spoken must not skip the talk`, then restore the line and rerun to green.

- [ ] **Step 19: Regenerate the narration golden and read the diff.**

```bash
go test -count=1 ./internal/narration/ -run TestSnapshotStores -update
git diff --stat internal/narration/testdata/stores/
git diff internal/narration/testdata/stores/quests.golden | grep -E '^[+-]quest\|' | cut -c1-60
go test -count=1 ./internal/narration/
```

Expected: only `quests.golden` changed, and every changed line starts `quest|32|`, `quest|33|`, `quest|34|`, `quest|53|` or `quest|90|`. Any other changed line: stop and report.

- [ ] **Step 20: Commit.**

```bash
git add internal/questengine/newbie_martial_steps_test.go internal/questengine/newbie_topic_steps_test.go internal/questengine/newbie_one_command_steps_test.go internal/questengine/coulee_bridges_test.go _datafiles/world/dogmud/quests/32-first_blood.yaml _datafiles/world/dogmud/quests/33-hold_the_wash.yaml _datafiles/world/dogmud/quests/34-take_the_tower.yaml _datafiles/world/dogmud/quests/53-cull_the_wash.yaml _datafiles/world/dogmud/quests/90-report_to_toke__fighting_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/narration/testdata/stores/quests.golden
git status --short
git commit -F - <<'EOF'
feat(quests): fighting line runs Toke, Vorn, Garve, Toke (#375 phase 7)

First Blood (32) now starts with a talk to Vorn; Vorn offers Hold the
Wash (33), which starts with a talk to Garve; Take the Tower (34) and
Cull the Wash (53) end at Garve's new hand-ins; Garve offers Report to
Toke (Fighting) (90), handed in at Toke. The tower kill no longer
carries a flag condition: the boon rides on quest_granted 34-slain.
Quest 32's ungranted consider step is renamed to special. Clears 13
loop-backlog lines; dashes gone from these quests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

`git status --short` before the commit must show no other staged path.

---

## Task 3: Vorn's skill topics and the shouts correction

Suggested executor: Sonnet (all text given).

**Files:**
- Create: `internal/questengine/coulee_skill_topics_test.go`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml`, `_datafiles/world/dogmud/dialogue/topic-keys.txt`

- [ ] **Step 1: Write the failing test.** Create `internal/questengine/coulee_skill_topics_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Skill coverage in Pothole Coulee (#375 phase 7 spec, "Skill coverage"): the
// NPCs who teach by talk name each skill's command, and the corrections hold.

// openInfo returns an info topic's text and hints, squashed, and requires it
// to be open to every player (no quest gate).
func openInfo(t *testing.T, dfile, key string) (text, hints string) {
	t.Helper()
	df := loadCouleeTopics(t, dfile)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %q", dfile, key)
	require.Equal(t, dialogue.KindInfo, tp.Kind, "%s topic %s", dfile, key)
	require.Empty(t, tp.QuestRequired, "%s topic %s is open to everyone", dfile, key)
	return squash(tp.Text), squash(tp.Hints)
}

func TestVorn9108_TeachesTheFightBeyondTheSwing(t *testing.T) {
	cases := map[string][]string{
		"grapple_help": {"type grapple", "help grapple", "cannot flee"},
		"bash_help":    {"type bash", "help bash", "shield"},
		"kick_help":    {"stomp", "knee", "tailsweep", "help kick"},
		"defence_help": {"dodge", "parry", "block", "help combat"},
	}
	for key, wants := range cases {
		text, hints := openInfo(t, "9108.yaml", key)
		for _, w := range wants {
			require.Contains(t, text+" "+hints, w, "Vorn's %s must say %q", key, w)
		}
	}
}

// Shouts do not come later: rally has no rank gate, and quest 28 already
// teaches warcry (spec "Corrections").
func TestVorn9108_ShoutsAreNotForLater(t *testing.T) {
	text, hints := openInfo(t, "9108.yaml", "shouts_help")
	require.NotContains(t, text+" "+hints, "come to you later")
	require.NotContains(t, text+" "+hints, "come later")
	require.Contains(t, text, "warcry")
	require.Contains(t, hints, "type rally")
}
```

- [ ] **Step 2: Run it and see it fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestVorn9108'
```

Expected: `FAIL` with `9108.yaml has no topic "grapple_help"` and `should not contain "come to you later"`.

- [ ] **Step 3: Replace `shouts_help` and add four topics.** In `9108.yaml` replace the whole `shouts_help` topic (from `- key: shouts_help` to the end of the file) with:

```yaml
- key: shouts_help
  kind: info
  label: What are battle-shouts?
  text: There is more to a fight than the swing. A battle-shout can turn one. You
    already know warcry. You shouted it in the practice ring before you ever reached
    the pool, and it makes your blows land harder, yours and your allies'. Rally is
    the other one. It steadies your guard and the guard of anyone fighting beside you,
    so dodges, parries and blocks come easier. Both lean on a silver tongue, not a
    strong arm, and they share the wait with kicks and trips.
  hints: Rally is a shout that steadies your defences and your allies'. Type rally.
    For more, type help rally.
- key: grapple_help
  kind: info
  label: How does grappling work?
  text: Grab a foe and hold on, and they cannot flee. Each round you both struggle,
    and the stronger, better trained wrestler gains ground, on top, behind, or pinned
    underneath. From a strong hold your kick turns into a knee. A long weapon is clumsy
    that close, so if you mean to wrestle, carry a short blade into the fight or go
    in bare-handed.
  hints: Grapple locks a foe in a hold so they cannot flee. Type grapple and the foe's
    name in a fight. For more, type help grapple.
  marked: true
- key: bash_help
  kind: info
  label: How do I fight with a shield?
  text: With a shield on your arm you can bash. You slam it into your foe. It hits
    less hard than a swing, but it can knock them to the ground, and a foe on the ground
    fights badly. It costs stamina and shares the wait with kick and trip. No shield,
    no bash, so strap one on before the fight if you want it.
  hints: Bash hits with your shield and may knock a foe down. Type bash in a fight.
    For more, type help bash.
- key: kick_help
  kind: info
  label: Why did my kick come out as a stomp?
  text: Your kick reads the fight for you. Kick a foe who is standing and it is a
    kick. Kick one already on the ground and it comes out as a stomp, which hits hard
    and keeps them down. Kick from on top in a grapple and it is a knee. Trips work
    the same way. Anyone born with a tail sweeps with it, and that is a tailsweep. You
    type the same word, and your body picks the move.
  hints: Kick changes with the fight, a stomp on a foe who is down and a knee in a
    grapple. For more, type help kick.
- key: defence_help
  kind: info
  label: How do I avoid getting hit?
  text: You defend without thinking about it. Every blow aimed at you meets a dodge,
    a parry with your weapon, or a block with your shield, whichever you are best at.
    Bare hands can only dodge. A weapon lets you parry. A shield lets you block. Choose
    your gear before the fight with that in mind. Some weapons reach further than others,
    and that matters more the closer a fight gets.
  hints: Dodge, parry and block happen on their own. For more, type help combat.
```

- [ ] **Step 4: Topic keys.** Add `9108.bash_help`, `9108.defence_help`, `9108.grapple_help`, `9108.kick_help` to `dialogue/topic-keys.txt` in sorted position.

- [ ] **Step 5: Run and pass.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestVorn9108|TestQuest32'
go test -count=1 ./internal/guards/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke_AllDialogueFilesParse' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: `ok` for each, and the verbose run lists `--- PASS` for both smoke tests (not `--- SKIP`).

- [ ] **Step 6: Commit.**

```bash
git add internal/questengine/coulee_skill_topics_test.go _datafiles/world/dogmud/dialogue/pothole_coulee/9108.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt
git commit -F - <<'EOF'
feat(dialogue): Vorn teaches grapple, bash, kick variants and defence

Skill coverage for #375 phase 7: four info topics on Drillmaster Vorn,
and his battle-shouts topic no longer says shouts come later (rally has
no rank gate and the antechamber already teaches warcry).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 4: The smithing line (35, 36, 37, 54, bridge 91)

Suggested executor: Opus.

**Files:**
- Replace: `internal/questengine/newbie_forge_steps_test.go`, `internal/questengine/first_heat_sellpath_test.go`
- Modify: `internal/actions/sell_rusk_dagger_test.go`, `internal/questengine/newbie_one_command_steps_test.go` (`lenientGrants`), `internal/questengine/coulee_bridges_test.go`
- Replace: `_datafiles/world/dogmud/quests/35-first_heat.yaml`, `36-pick_the_vein.yaml`, `37-into_the_vein.yaml`, `54-back_to_the_forge.yaml`
- Create: `_datafiles/world/dogmud/quests/91-report_to_toke__smithing_.yaml`
- Replace: `_datafiles/world/dogmud/dialogue/pothole_coulee/9117.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9116.yaml`, `9106.yaml`, `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Regenerate: `internal/narration/testdata/stores/quests.golden`

`internal/questengine/first_heat_givepath_test.go` needs no change: the `craft` step, the `craft_turnin` gate and the `strip` step it uses keep their meaning.

- [ ] **Step 1: Replace `internal/questengine/newbie_forge_steps_test.go`** with:

```go
package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The smithing line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Heat (35) and Smith Rusk takes it; Rusk offers Pick the Vein (36) and
// Survivor Ovell takes it; Ovell offers Into the Vein (37), then Report to
// Toke (Smithing) (91) and the repeatable Back to the Forge (54). One command
// per step, each trigger gated on the step before it, every quest ends at a
// hand-in. The forge prefix keeps identifiers apart from the sibling
// newbie_*_steps_test.go files in this package.

const (
	forgeSmithy     = 5245 // The Coulee Smithy: Rusk and the forge
	forgeOreStall   = 5246 // Ore Stall: no merchant, no forge
	forgeLowerTalus = 5249
	forgeScreeField = 5250
	forgeCollapsed  = 5251 // Collapsed Cut: Ovell
	forgeOrePocket  = 5252
	forgeUpperTalus = 5253
	forgeLowerDrift = 5258
	forgeDen        = 5260

	forgeIronIngot    = 40001
	forgeLeatherStrip = 40002
	forgeIronDagger   = 10009

	forgeSmithRusk  = 9116
	forgeOvell      = 9117
	forgeScavenger  = 9118
	forgeLurker     = 9119
	forgeDriftMob   = 9122
	forgeStoneBeast = 9123
)

func TestRusk9116_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f := "35-first_heat.yaml"
	pre := []string{"30-end", "31-end"}
	checkTopicSteps(t, "rusk", "9116.yaml", []topicStep{
		{f, 35, "met", "type list", pre, "heat_list_help", false},
		{f, 35, "list", "type buy iron ingot", pre, "heat_ingot_help", false},
		{f, 35, "ingot", "type buy leather strip", pre, "heat_strip_help", false},
		{f, 35, "strip", "type craft iron dagger", pre, "heat_craft_help", false},
		{f, 35, "craft", "", pre, "craft_turnin_told", true},
	})
}

func TestOvell9117_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f36 := "36-pick_the_vein.yaml"
	pre := []string{"30-end", "35-end"}
	checkTopicSteps(t, "ovell", "9117.yaml", []topicStep{
		{f36, 36, "met", "type attack scavenger", pre, "vein_scavenger_help", false},
		{f36, 36, "scavenger", "type attack lurker", pre, "vein_lurker_help", false},
		{f36, 36, "lurker", "type forage", pre, "vein_forage_help", false},
		{f36, 36, "ore", "", pre, "vein_turnin", true},
		{"37-into_the_vein.yaml", 37, "start", "type attack beast", []string{"30-end", "35-end", "36-end"}, "beast_help", false},
		{"37-into_the_vein.yaml", 37, "slain", "", []string{"30-end", "35-end", "36-end"}, "mine_in", true},
		{"54-back_to_the_forge.yaml", 54, "start", "type craft iron dagger", []string{"30-end", "35-end", "36-end", "37-end"}, "forge_loop_help", false},
		{"54-back_to_the_forge.yaml", 54, "crafted", "", []string{"30-end", "35-end", "36-end", "37-end"}, "forge_loop_in", true},
	})
}

func TestQuest35FirstHeat_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)
	require.Equal(t, []string{"start", "met", "list", "ingot", "strip", "craft", "end"}, newLinearPlayer(q).steps,
		"start, list, ingot, strip and craft are ids a saved character may hold; met is inserted after start")
	turnIn := turnInRequires(t, "9116.yaml", "craft_turnin")
	require.Equal(t, []string{"35-craft"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: forgeSmithRusk, Topic: "heat_arrive", RoomId: forgeSmithy}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "list", RoomId: forgeSmithy})
		require.Equal(t, "start", w.p.current, "listing before Rusk has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "met", w.p.current, "a purchase out of order must not skip the list step")
		w.notify("command", EventDetails{Command: "list", RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current, "the strip out of order must not skip the ingot step")
		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "ingot", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "ingot", w.p.current, "a craft before the strip must not skip it (no lenient grant)")
		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "strip", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the craft")

		w.notify("command", EventDetails{Command: "craft", RoomId: forgeOreStall})
		require.Equal(t, "strip", w.p.current, "crafting away from the forge is not the forge lesson")
		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "craft", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the craft")
	})

	t.Run("the craft line names the sell route and the talk", func(t *testing.T) {
		found := false
		for _, tr := range q.Triggers {
			if tr.Command != "craft" {
				continue
			}
			for _, a := range tr.Actions {
				if a.SendText == "" {
					continue
				}
				found = true
				low := squash(a.SendText)
				require.Contains(t, low, "sell", "#697: the post-craft line mentions the sell route")
				require.Contains(t, low, "type talk rusk")
			}
		}
		require.True(t, found, "the craft trigger has a line")
	})

	t.Run("Rusk's talk step", func(t *testing.T) {
		checkArriveTopic(t, "rusk", "9116.yaml", q, forgeSmithRusk, "heat_arrive", []string{"30-end", "31-end"})
	})

	rusk := loadCouleeTopics(t, "9116.yaml")
	require.Equal(t, forgeSmithRusk, rusk.MobId)

	t.Run("player saved on craft can turn in by handing over the blade", func(t *testing.T) {
		w := newQuestWalker(q).at("craft")
		w.p.fullMockPlayer.quests["30-end"] = true
		w.p.items[forgeIronDagger] = true
		require.True(t, holdsAll(w.p, turnIn))
		n := newNpcTalker(rusk, w.p)
		tp, wanted := rusk.ItemTopic(forgeIronDagger, n.state())
		require.True(t, wanted, "Rusk wants the dagger")
		require.NotNil(t, tp, "giving the dagger on the craft step must play a hand-in")
		require.Equal(t, "craft_turnin", tp.Key)
		require.True(t, rusk.Apply(tp, n.state()))
		require.Equal(t, "end", w.p.current)
		require.True(t, w.p.items[forgeIronDagger], "Rusk looks the dagger over and hands it back")
	})

	t.Run("handing over a blade before the craft does not finish the quest", func(t *testing.T) {
		w := newQuestWalker(q).at("strip")
		w.p.fullMockPlayer.quests["30-end"] = true
		w.p.items[forgeIronDagger] = true
		tp, wanted := rusk.ItemTopic(forgeIronDagger, newNpcTalker(rusk, w.p).state())
		require.True(t, wanted, "Rusk wants the dagger, so give hands it back instead of keeping it")
		require.Nil(t, tp, "no hand-in is open before the craft")
		require.Equal(t, "strip", w.p.current)
	})
}

func TestQuest36PickTheVein_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "36-pick_the_vein.yaml", 36)
	require.Equal(t, []string{"start", "met", "scavenger", "lurker", "ore", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9117.yaml", "vein_turnin")
	require.Equal(t, []string{"36-ore"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: forgeOvell, Topic: "vein_arrive", RoomId: forgeCollapsed}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: forgeScavenger, RoomId: forgeLowerTalus})
		require.Equal(t, "start", w.p.current, "a kill before Ovell has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "met", w.p.current, "a forage first must not skip both fights")
		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeOrePocket})
		require.Equal(t, "met", w.p.current, "the lurker first must not skip the scavenger")

		w.notify("mob_death", EventDetails{MobId: forgeScavenger, RoomId: forgeLowerTalus})
		require.Equal(t, "scavenger", w.p.current)
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "scavenger", w.p.current, "a forage must not skip the lurker")

		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeUpperTalus})
		require.Equal(t, "lurker", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeLowerTalus})
		require.Equal(t, "lurker", w.p.current, "foraging off the vein does not count")
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeOrePocket})
		require.Equal(t, "ore", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the forage")
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
	})

	t.Run("forage lines do not claim ore was found", func(t *testing.T) {
		for _, tr := range q.Triggers {
			if tr.Command != "forage" {
				continue
			}
			for _, a := range tr.Actions {
				low := strings.ToLower(a.SendText)
				for _, claim := range []string{"you pry", "you work a chunk", "pull ore", "you pulled"} {
					require.NotContains(t, low, claim,
						"forage can roll and find nothing, so its line must not claim ore: %q", a.SendText)
				}
			}
		}
	})

	t.Run("Rusk offers it and Ovell no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9116.yaml", "vein_offer", q, "35-end", "9117.yaml")
	})

	t.Run("Ovell's talk step", func(t *testing.T) {
		checkArriveTopic(t, "ovell", "9117.yaml", q, forgeOvell, "vein_arrive", []string{"30-end", "35-end"})
	})
}

func TestQuest37IntoTheVein_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "37-into_the_vein.yaml", 37)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside, "the mine is pitch black: the start hint carries the dark aside")
	turnIn := turnInRequires(t, "9117.yaml", "mine_in")
	require.Equal(t, []string{"37-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: forgeDriftMob, RoomId: forgeLowerDrift})
	require.Equal(t, "start", w.p.current, "a lesser mine mob does not count")
	w.notify("mob_death", EventDetails{MobId: forgeStoneBeast, RoomId: forgeDen})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"37-end"}), "the kill no longer ends the quest")
}

func TestQuest54BackToTheForge_EndsAtOvell(t *testing.T) {
	q := loadShippedQuest(t, "54-back_to_the_forge.yaml", 54)
	require.Equal(t, []string{"start", "crafted", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type craft iron dagger", "the hint names the recipe to craft")
	require.Contains(t, questHintFor(t, q, "crafted"), "type talk ovell")
	turnIn := turnInRequires(t, "9117.yaml", "forge_loop_in")
	require.Equal(t, []string{"54-crafted"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeOrePocket})
	require.Equal(t, "start", w.p.current, "crafting away from the forge does not count")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
	require.Equal(t, "crafted", w.p.current)
	require.False(t, holdsAll(w.p, []string{"54-end"}), "the craft no longer ends the run")
}
```

- [ ] **Step 2: Replace `internal/questengine/first_heat_sellpath_test.go`** with:

```go
package questengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// R22 (#375) let a sale of the crafted iron dagger to Smith Rusk complete
// First Heat (35). #375 phase 7 ends every quest at a hand-in (ruleEnd), so
// the sale now only draws Rusk's line, and the player tells her how the work
// went (craft_turnin_told, pinned in newbie_topic_steps_test.go). These load
// the REAL quest 35 file and fire the command event the sell action raises.

const (
	sellPathRusk   = 9116
	sellPathDagger = 10009
	sellPathForge  = 5245
)

func loadQuest35ForSell(t *testing.T) (*Engine, *QuestDef) {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/35-first_heat.yaml")
	require.NoError(t, err, "must be able to read the real quest 35 YAML")
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, 35, q.QuestId)
	e := NewEngine()
	e.RegisterQuest(&q)
	return e, &q
}

func crafted35Player() *fullMockPlayer {
	p := newFullMockPlayer(sellPathForge)
	p.quests["35-start"] = true
	p.quests["35-craft"] = true
	return p
}

func TestQuest35_NoTriggerGrantsTheEnd(t *testing.T) {
	_, q := loadQuest35ForSell(t)
	for i, tr := range q.Triggers {
		for _, a := range tr.Actions {
			require.NotEqual(t, "35-end", a.Grant, "trigger %d grants 35-end; only Rusk's hand-in may", i)
		}
	}
}

func TestQuest35_SellingDaggerToRuskPointsToTheHandIn(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	result := e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.True(t, result.Handled, "selling the dagger to Rusk draws her line")
	assert.False(t, player.HasQuest("35-end"), "the sale alone does not complete First Heat")
	require.Len(t, ctx.npcSays, 1, "Rusk must answer the sale")
	assert.Equal(t, sellPathRusk, ctx.npcSays[0].Mob)
	require.Len(t, ctx.npcSays[0].Lines, 1)
	assert.Equal(t, "I love the entrepreneurial spirit. Now tell me how the work went.",
		ctx.npcSays[0].Lines[0].Text)
}

func TestQuest35_SellingOtherItemToRuskSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: 40001, MobId: sellPathRusk,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "selling an iron ingot is not the dagger")
}

func TestQuest35_SellingDaggerToAnotherShopSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: 9117,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "only Rusk's shop answers")
}

func TestQuest35_SellingDaggerBeforeCraftingSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := newFullMockPlayer(sellPathForge)
	player.quests["35-start"] = true
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "the line waits for 35-craft")
	assert.False(t, player.HasQuest("35-end"))
}
```

- [ ] **Step 3: Edit `internal/actions/sell_rusk_dagger_test.go`.** Replace the header comment's first paragraph:

```go
// R22 (#375): a completed sale of the crafted Iron Dagger (item 10009) to Smith
// Rusk (mob 9116) turns in quest 35 "First Heat". That only works if Rusk
// actually BUYS the dagger on a fresh world, where her mob template says
// `gold: 0`. This test runs the real Sell path against the REAL item, mob and
// quest files, with a fresh (empty) shop store.
```

with:

```go
// R22 (#375): a player may sell the crafted Iron Dagger (item 10009) to Smith
// Rusk (mob 9116) instead of showing it to her. Since #375 phase 7 the sale no
// longer completes quest 35 "First Heat" (every quest ends at a hand-in): the
// player tells Rusk afterwards. The sale itself must still go through on a
// fresh world, where her mob template says `gold: 0`. This test runs the real
// Sell path against the REAL item, mob and quest files, with a fresh (empty)
// shop store.
```

Rename `func TestSell_RuskBuysTheCraftedDaggerOnAFreshWorldAndFirstHeatTurnsIn(` to `func TestSell_RuskBuysTheCraftedDaggerOnAFreshWorld(`, and replace:

```go
	assert.True(t, u.Character.HasQuest("35-end"), "the completed sale must turn in First Heat")
```

with:

```go
	assert.False(t, u.Character.HasQuest("35-end"), "a sale no longer turns First Heat in; Rusk's hand-in does")
```

- [ ] **Step 4: End the two lenient grants.** In `internal/questengine/newbie_one_command_steps_test.go` replace:

```go
var lenientGrants = map[string]bool{
	"50-shoot": true, // fire needs the sling readied (the equip step)
	"35-craft": true, // crafting the dagger needs the bought ingot and strip
	"38-brew":  true, // brewing the salve needs the bought vial and roots
}
```

with:

```go
var lenientGrants = map[string]bool{
	"50-shoot": true, // fire needs the sling readied (the equip step)
}
```

(`38-brew` leaves here too: Task 5 makes it strict, and a stale entry only skips a check.)

- [ ] **Step 5: Add the smithing bridge case.** In `internal/questengine/coulee_bridges_test.go`, after the `t.Run("Fighting", ...)` block add:

```go
	t.Run("Smithing", func(t *testing.T) {
		checkBridge(t, toke, 91, "Smithing", "91-report_to_toke__smithing_.yaml",
			"9117.yaml", "report_smithing_offer", "report_smithing_in", "37-end")
	})
```

- [ ] **Step 6: Run and see them fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestRusk9116|TestOvell9117|TestQuest35|TestQuest36|TestQuest37|TestQuest54|TestCouleeBridges'
go test -count=1 ./internal/actions/ -run TestSell_RuskBuysTheCraftedDagger
```

Expected: `FAIL` (step lists differ, `no topic "heat_arrive"`, `trigger 4 grants 35-end`, the 91 file missing, and the actions test `a sale no longer turns First Heat in`).

- [ ] **Step 7: Replace `_datafiles/world/dogmud/quests/35-first_heat.yaml`** with:

```yaml
# Quest 35, First Heat (Pothole Coulee, smithing line, quest 1).
#
# Crier Toke offers it (send_forge, dialogue 9106). Smith Rusk (9116) takes
# the hand-in (craft_turnin when the dagger is shown, craft_turnin_told when
# it was sold, salvaged or lost) and then offers Pick the Vein (36).
#   start  go to the Coulee Smithy and talk to Rusk
#   met    topic_heard on Rusk's heat_arrive topic
#   list   list beside Rusk in the smithy (5245)
#   ingot  item_gain of an iron ingot (40001)
#   strip  item_gain of a leather strip (40002)
#   craft  craft at the forge (5245), after the strip
#   end    Rusk's hand-in
# Selling the dagger to Rusk (R22) is still welcome but no longer ends the
# quest (#375 phase 7, every quest ends at a hand-in): the sell trigger only
# plays Rusk's line, and the player tells her how the work went.

questid: 35
name: First Heat
description: >-
  The coulee smithy's forge, and Smith Rusk to judge your work. Buy an iron
  ingot and a leather strip from her stall, work them at the forge into an
  iron dagger, and show her the finished blade.
secret: false

steps:
  - id: start
    destination:
      mob: 9116
      room: 5245
    description: >-
      Crier Toke has sent you to Smith Rusk at the Coulee Smithy. From Hub
      Square, go north to the Awakening Pool, then west five times to the
      smithy.
    hint: >-
      From Hub Square, go north, then west five times to the Coulee Smithy.
      Type talk rusk.
  - id: met
    destination:
      room: 5245
    description: >-
      Rusk has told you how the forge works. First see what she sells. Her
      goods are on her stall beside the forge.
    hint: >-
      In the Coulee Smithy, next to Rusk, type list.
  - id: list
    destination:
      room: 5245
    description: >-
      You have seen Rusk's goods. The dagger needs an iron ingot and a
      leather strip. Buy the ingot first.
    hint: >-
      Type buy iron ingot.
  - id: ingot
    destination:
      room: 5245
    description: >-
      You have an iron ingot. Now buy the leather strip for the grip.
    hint: >-
      Type buy leather strip.
  - id: strip
    destination:
      room: 5245
    description: >-
      You have both materials. Stand at the forge in the Coulee Smithy and
      make the dagger. The forge takes a few moments to finish the work.
    hint: >-
      At the forge in the Coulee Smithy, type craft iron dagger. A craft can
      fail. If it does, buy an iron ingot and a leather strip again and
      retry.
  - id: craft
    destination:
      mob: 9116
      room: 5245
    description: >-
      You worked iron at the forge for the first time and turned raw
      materials into something that did not exist before. Show the dagger
      to Rusk. If you would rather have the coin, sell it to her first, then
      tell her how the work went.
    hint: >-
      Go to Smith Rusk at the forge. Type talk rusk.
  - id: end
    description: >-
      Rusk is satisfied with your first work at the forge, and she knows a
      man on the talus slope who needs a hand.

rewards:
  skillinfo: "blacksmithing:1"
  gold: 15
  # Neutral on purpose: Rusk's two hand-in topics (the dagger shown, or
  # her taking the player's word) both end here.
  actor: >-
    Rusk looks at you, not at the forge, with the measuring expression
    of a smith deciding whether a piece of iron is worth tempering.
    Then she nods once and presses a few coins into your hand with the
    finality of a door closing.
  observer: "Rusk nods once and presses a few coins into {actor}'s
    hand."

triggers:
  # met: hearing Rusk's heat_arrive topic.
  - event: topic_heard
    mob: 9116
    topic: heat_arrive
    conditions:
      has: ["35-start"]
      missing: ["35-met"]
    actions:
      - grant: "35-met"
      - actor: "Rusk has shown you the shape of it. Start with her stall:
          type list."

  # list: Rusk's goods (she trades in the smithy, room 5245).
  - event: command
    command: list
    room: 5245
    conditions:
      has: ["35-met"]
      missing: ["35-list"]
    actions:
      - grant: "35-list"
      - actor: "Those are Rusk's goods. The dagger needs an iron
          ingot and a leather strip. Type buy iron ingot."

  # ingot: an iron ingot (40001), after the list.
  - event: item_gain
    item: 40001
    conditions:
      has: ["35-list"]
      missing: ["35-ingot"]
    actions:
      - grant: "35-ingot"
      - actor: "You have an iron ingot. Now the grip. Type buy
          leather strip."

  # strip: a leather strip (40002), after the ingot.
  - event: item_gain
    item: 40002
    conditions:
      has: ["35-ingot"]
      missing: ["35-strip"]
    actions:
      - grant: "35-strip"
      - actor: "You have the leather strip. That is everything the
          dagger needs. At the forge, type craft iron dagger."

  # craft: at the forge (room 5245), after the strip. The craft quest event
  # fires only when a craft succeeds, so the dagger exists when this plays.
  - event: command
    command: craft
    room: 5245
    conditions:
      has: ["35-strip"]
      missing: ["35-craft"]
    actions:
      - grant: "35-craft"
      - actor: "The iron dagger is yours, made at the forge. Show it to
          Rusk, or sell it to her if you would rather have the coin. Either
          way, type talk rusk after."

  # A sale of the finished blade to Rusk's shop draws her line and grants
  # nothing. The hand-in that takes the player's word (craft_turnin_told)
  # finishes the quest.
  - event: command
    command: sell
    room: 5245
    mob: 9116
    item: 10009
    conditions:
      has: ["35-craft"]
      missing: ["35-end"]
    actions:
      - npc_say:
          mob: 9116
          lines:
            - {delay: 1, text: "I love the entrepreneurial spirit. Now tell me
                how the work went."}
```

- [ ] **Step 8: Replace `_datafiles/world/dogmud/quests/36-pick_the_vein.yaml`** with:

```yaml
# Quest 36, Pick the Vein (Pothole Coulee, smithing line, quest 2).
#
# Smith Rusk (9116) offers it (vein_offer) once First Heat (35) is handed
# in. Survivor Ovell (9117) takes the hand-in (vein_turnin) and then offers
# Into the Vein (37).
#   start      go to the Collapsed Cut and talk to Ovell
#   met        topic_heard on Ovell's vein_arrive topic
#   scavenger  kill a scree scavenger (9118)
#   lurker     kill a talus lurker (9119), after the scavenger
#   ore        forage the vein (rooms 5250 or 5252), after the lurker; a
#              forage that finds nothing still counts
#   end        Ovell's hand-in, which gifts the Component Pouch (30033)

questid: 36
name: Pick the Vein
description: >-
  Survivor Ovell pulled himself out of a cave-in on the talus slope. His leg
  keeps him off the high scree, but the basalt-iron vein is still there and
  so are the scavengers and lurkers that claimed it. Smith Rusk has sent you
  to clear them out and work the vein yourself.
secret: false

steps:
  - id: start
    destination:
      mob: 9117
      room: 5251
    description: >-
      Smith Rusk has sent you to Survivor Ovell, who pulled himself out of a
      cave-in on the talus slope. From the Coulee Smithy, go north to the
      Quench Shed, west twice to the Lower Talus, then south into the
      Collapsed Cut.
    hint: >-
      From the Coulee Smithy, go north, west twice, then south into the
      Collapsed Cut. Type talk ovell.
  - id: met
    destination:
      room: 5249
    description: >-
      Ovell needs the slope cleared, starting with a scree scavenger. They
      crawl the Lower Talus just north of his cut, and the Scree Field and
      Upper Talus beyond it. If a fight turns against you, flee. If you
      die, you wake at the Mending Hut, not in the rock.
    hint: >-
      Go north from the Collapsed Cut to the Lower Talus and type attack
      scavenger. If it is dark, type cast glow first.
  - id: scavenger
    destination:
      room: 5252
    description: >-
      One scree scavenger is down. Next is a talus lurker. One waits in the
      Ore Pocket, west of Ovell's cut and south of the Scree Field, and
      another on the Upper Talus. Lurkers are tough, so flee if the fight
      turns.
    hint: >-
      In the Ore Pocket, west of the Collapsed Cut, type attack lurker.
  - id: lurker
    destination:
      room: 5250
    description: >-
      The talus lurker is down. Now work the basalt-iron vein. It shows in
      the bluff above the Scree Field and runs into the rubble of the Ore
      Pocket below it.
    hint: >-
      In the Ore Pocket or on the Scree Field just north of it, type forage.
  - id: ore
    destination:
      mob: 9117
      room: 5251
    description: >-
      You have worked the basalt-iron vein, and the slope is cleared.
      Report back to Ovell in the Collapsed Cut.
    hint: >-
      Go back to Survivor Ovell in the Collapsed Cut, east of the Ore
      Pocket, and type talk ovell.
  - id: end
    description: >-
      The slope is clear and you have worked the vein. Ovell pressed a
      component pouch into your hands. Once you wear it, your crafting
      materials go into it on their own. The mine below still holds
      something he would not name.

rewards:
  skillinfo: "blacksmithing:1"
  itemid: 30033
  gold: 25
  actor: >-
    Ovell digs a worn leather pouch from his coat with the care of
    someone parting with something that has been useful. The quartz
    eye tracks you steadily while the other squints in the slope
    light. "You cleared it. Worked the vein. I did not know if you
    would come back." He presses the component pouch into your hands
    and curls your fingers around the strap. "Materials go in on
    their own once you wear it. Sort if you have loose ones already.
    And if you ever want to break a piece of kit back down,
    salvage it. The slope gives back what you put into it, mostly."

triggers:
  # met: hearing Ovell's vein_arrive topic.
  - event: topic_heard
    mob: 9117
    topic: vein_arrive
    conditions:
      has: ["36-start"]
      missing: ["36-met"]
    actions:
      - grant: "36-met"
      - actor: >-
          Ovell has told you what the slope needs. The scavenger first. Go
          north to the Lower Talus and type attack scavenger.

  # scavenger: a scree scavenger (9118).
  - event: mob_death
    mob: 9118
    conditions:
      has: ["36-met"]
      missing: ["36-scavenger"]
    actions:
      - grant: "36-scavenger"
      - actor: >-
          The scree scavenger goes still on the rock. Next is the
          talus lurker. In the Ore Pocket, type attack lurker.

  # lurker: a talus lurker (9119), after the scavenger.
  - event: mob_death
    mob: 9119
    conditions:
      has: ["36-scavenger"]
      missing: ["36-lurker"]
    actions:
      - grant: "36-lurker"
      - actor: >-
          The talus lurker drops back into the rock and does not
          rise. Now work the vein. In the Ore Pocket or on the Scree
          Field, type forage.

  # ore: forage the vein on the Scree Field (5250). A forage can find
  # nothing; it still counts, so the line claims no ore.
  - event: command
    command: forage
    room: 5250
    conditions:
      has: ["36-lurker"]
      missing: ["36-ore"]
    actions:
      - grant: "36-ore"
      - actor: >-
          You work the exposed vein for whatever it will give. The
          seam is open to anyone now. Go back to Ovell in the
          Collapsed Cut and type talk ovell.

  # ore: forage the vein in the Ore Pocket (5252).
  - event: command
    command: forage
    room: 5252
    conditions:
      has: ["36-lurker"]
      missing: ["36-ore"]
    actions:
      - grant: "36-ore"
      - actor: >-
          You pick through the rubble where the vein crosses the ore
          pocket. The seam is open to anyone now. Go back to Ovell,
          just east of here, and type talk ovell.
```

- [ ] **Step 9: Replace `_datafiles/world/dogmud/quests/37-into_the_vein.yaml`** with:

```yaml
# Quest 37, Into the Vein (Pothole Coulee, smithing line finale).
#
# Survivor Ovell (9117) offers it (mine_offer) after Pick the Vein (36). The
# player descends the dark basalt mine (cast glow first) and kills the
# Stone-Blooded Beast (9123) in the Den (5260).
# Route: Upper Talus 5253 west to the Mine Mouth 5254, down to the Mine Head
# 5255, west to the Timbered Drift 5256, down to the Lower Drift 5258, west to
# the Stone Gallery 5259, west to the Den 5260.
#   start  kill the beast
#   slain  report to Ovell
#   end    Ovell's hand-in mine_in, which pays the forged blade (10045), a
#          blacksmithing rank, a Strength gain and the basalt-iron recipe

questid: 37
name: Into the Vein
description: >-
  A stone-blooded beast has denned at the bottom of the basalt mine below the
  slope. The mine is pitch dark, so cast your glow before you descend, and the
  beast is the worst thing in it. Take it, then tell Ovell, and the deep vein
  is yours.
secret: false

steps:
  - id: start
    destination:
      room: 5260
    description: >-
      Survivor Ovell has set you on the mine. The Mine Mouth is west of the
      Upper Talus. From there the way to the Den is down, west, down, then
      west twice. The mine is pitch black, so type cast glow before you go
      down. Other things live in the workings. Type consider on a foe to
      size it up, and flee if a fight turns.
    hint: >-
      In the Den at the bottom of the mine, type attack beast. If it is
      dark, type cast glow first.
  - id: slain
    destination:
      mob: 9117
      room: 5251
    description: >-
      The stone-blooded beast is dead and the deep vein is open. Climb out
      of the mine and tell Ovell.
    hint: >-
      Go back up to Survivor Ovell in the Collapsed Cut. Type talk ovell.
  - id: end
    description: >-
      The beast is dead and the deep vein is open. You came in not knowing
      which end of a hammer to hold. You leave a smith.

rewards:
  itemid: 10045
  skillinfo: "blacksmithing:1"
  stat_info: "strength:3"
  recipe_info: "basalt-iron-blade"
  gold: 50
  actor: >-
    Ovell listens with his quartz eye half closed, as if he can hear the
    deep mine through the rock. When you finish he nods slowly. "Then the
    vein is yours." He talks you through how the dark ore folds into an
    edge until the shape of the recipe settles in your head, and he waves
    at the blade you carried up. "Keep that. The first of many." The long
    work of the slope has settled into your arms as a lasting strength.

triggers:
  # slain: the Stone-Blooded Beast (9123) falls.
  - event: mob_death
    mob: 9123
    conditions:
      has: ["37-start"]
      missing: ["37-slain"]
    actions:
      - grant: "37-slain"
      - actor: >-
          The stone-blooded beast shudders, its fused hide cracking like
          cooling slag, and goes down for the last time. The deep mine is
          quiet, and a blade lies on the floor of the den. Climb back up to
          Ovell and type talk ovell.
```

- [ ] **Step 10: Replace `_datafiles/world/dogmud/quests/54-back_to_the_forge.yaml`** with:

```yaml
# Quest 54, Back to the Forge (Pothole Coulee, smithing line repeatable).
#
# Survivor Ovell (9117) offers it (forge_loop_offer) once Into the Vein (37)
# is handed in. Repeatable with a cooldown; completion clears progress.
#   start    craft anything at the forge in the Coulee Smithy (5245)
#   crafted  report to Ovell
#   end      Ovell's hand-in forge_loop_in (#375 phase 7)

questid: 54
name: Back to the Forge
description: >-
  Ovell wants to see you keep your hand in at the forge. Head back to the
  smith's fire, turn out another piece of work, and tell him how it rang.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5245
    description: >-
      Ovell has asked you to put your blacksmithing to work once more. Any
      recipe made at the forge in the Coulee Smithy counts. An iron dagger
      needs an iron ingot and a leather strip, and Smith Rusk sells both.
    hint: >-
      At the forge in the Coulee Smithy, type craft iron dagger. A craft can
      fail. If it does, buy an iron ingot and a leather strip again and
      retry.
  - id: crafted
    destination:
      mob: 9117
      room: 5251
    description: >-
      You put good work through the forge again. Ovell waits in the
      Collapsed Cut to hear how it went.
    hint: >-
      Go back to Survivor Ovell in the Collapsed Cut. Type talk ovell.
  - id: end
    description: >-
      You put good work through the forge again. Ovell will have another
      reason to send you back before long.

rewards:
  gold: 6
  skillinfo: "blacksmithing:1"
  actor: >-
    Ovell turns a coin over his knuckles before he hands it to you. "Still
    sharp. The iron knows a practiced hand. Come back when you want to keep
    it that way."
  observer: "Ovell hands {actor} a few coins with a slow, approving nod."

triggers:
  - event: command
    command: craft
    room: 5245
    conditions:
      has: ["54-start"]
      missing: ["54-crafted"]
    actions:
      - grant: "54-crafted"
      - actor: >-
          The work rings clean off the anvil. Go back to Ovell in the
          Collapsed Cut and type talk ovell.
```

- [ ] **Step 11: Create `_datafiles/world/dogmud/quests/91-report_to_toke__smithing_.yaml`:**

```yaml
# Quest 91, Report to Toke (Smithing) (Pothole Coulee, smithing line bridge).
#
# Survivor Ovell (9117) offers it (report_smithing_offer) once Into the Vein
# (37) is handed in. Crier Toke (9106) takes the hand-in
# (report_smithing_in). Same shape as quest 90.

questid: 91
name: Report to Toke (Smithing)
description: >-
  Survivor Ovell has asked you to carry the news of the deep mine to Crier
  Toke in Hub Square.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # follow Crier Toke (schedule pothole_toke)
      room: 5203  # fallback when Crier Toke is not spawned
    description: >-
      The stone-blooded beast is dead and the deep vein is open. Crier Toke,
      who sent you to Smith Rusk, waits in Hub Square for the news.
    hint: >-
      Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - id: end
    description: >-
      Toke has your news, and every smith in Gaius will hear it.
```

- [ ] **Step 12: Replace `_datafiles/world/dogmud/dialogue/pothole_coulee/9117.yaml`** (Survivor Ovell) with:

```yaml
mobid: 9117
zone: Pothole Coulee
defaultMood: neutral
greetings:
- text: Hey. Careful on the scree. It is loose up here. I am not going anywhere fast,
    so there is no rush.
  moods:
  - friendly
  - neutral
opener:
  text: Ovell. Prospector, or I was, anyway. Cave-in caught me three days back, up
    where the ore pocket narrows. I dug myself far enough out to breathe and then
    the leg gave and that was that. The slope is still good rock, basalt-iron in
    a good seam, but the scavengers moved in after the noise of the fall and the
    lurker claimed the pocket. I can see the vein from here with this eye and I
    cannot reach it.
  hints: Each line below is something Ovell can tell you about. Type its number.
  variants:
  - questExcluded:
    - 30-end
    text: You have not been through the pool yet. I can tell by the eyes. Cleric
      Hadwen at the water will sort that. Come back after. The slope will keep and
      so will I.
    hints: Find Cleric Hadwen at the pool in Pothole Coulee and go through the Awakening
      Rite before heading up the slope.
  - questRequired:
    - 30-end
    questExcluded:
    - 35-end
    text: Opened, good. But the slope is not a kind place to find your footing for
      the first time. There is a smith, Rusk, at the Coulee Smithy who can put some
      edge on your hands before you come up here. From here it is north, east, east,
      then south. Work with her first. Then come back.
    hints: Find Smith Rusk at the Coulee Smithy and complete her work before tackling
      the talus slope. From the Collapsed Cut go north, east, east, then south. From
      the Awakening Pool, follow the path west five times.
  - questRequired:
    - 35-end
    questExcluded:
    - 36-start
    - 36-end
    text: Forged something already. Good. Rusk sends me the ones with steady hands.
      If she has not sent you up yet, ask her about the slope.
    hints: From the Collapsed Cut, go north, east twice, then south to the Coulee Smithy.
      Type talk rusk.
  - questRequired:
    - 36-start
    questExcluded:
    - 36-met
    - 36-end
    text: Rusk sent you up? Good. Sit on the rock there and I will tell you what the
      slope needs.
    hints: Choose Smith Rusk sent me to clear the slope below.
  - questRequired:
    - 36-met
    questExcluded:
    - 36-scavenger
    - 36-end
    text: The scavenger first. One works the Lower Talus, just north of me. Put it
      down. If it turns against you, flee and come back. If you die, you wake at
      the Mending Hut.
    hints: Go north to the Lower Talus and type attack scavenger.
  - questRequired:
    - 36-scavenger
    questExcluded:
    - 36-lurker
    - 36-end
    text: Good. Now the lurker. It holds the Ore Pocket, just west of me, and another
      ranges the Upper Talus. It is tougher than the scavenger. Flee if you must.
    hints: Go west into the Ore Pocket and type attack lurker.
  - questRequired:
    - 36-lurker
    questExcluded:
    - 36-ore
    - 36-end
    text: The way is open. Now work the vein yourself. It runs through the rubble
      in the Ore Pocket and shows in the bluff above the Scree Field. Forage it, same
      as anything else on the slope.
    hints: In the Ore Pocket or on the Scree Field, type forage.
  - questRequired:
    - 36-ore
    questExcluded:
    - 36-end
    text: You are back, and the slope sounds different. Tell me how it went.
    hints: Choose Pick the Vein below to report.
  - questRequired:
    - 36-end
    questExcluded:
    - 37-start
    - 37-end
    text: 'You cleared the slope and worked the vein. You are not the green thing
      that came up here three days ago. So I will say the rest of it. Below us, where
      the old claim runs deep, a beast has denned in the dark. It is stone-blooded.
      Its hide has fused with the rock until you cannot tell where one ends, and it
      has eaten men who went down armed and certain. It is the reason the deep vein
      has sat untouched. Put it down and the mine is yours. But hear me on the dark
      first, every time: cast your glow before you descend. Go blind down there and
      you will not come back up.'
    hints: Ovell is offering the mine. Choose Into the Vein below.
  - questRequired:
    - 37-start
    questExcluded:
    - 37-slain
    - 37-end
    text: The beast is still down there in the deep dark. The Mine Mouth is west of
      the Upper Talus. From there it is down, west, down, then west twice to the Den.
      Before you go down, type cast glow. Then put the thing down in its den. Flee
      back up if you must. The Mending Hut holds, same as ever.
    hints: In the Den at the bottom of the mine, type attack beast. If it is dark,
      type cast glow first.
  - questRequired:
    - 37-slain
    questExcluded:
    - 37-end
    text: You came back up out of the dark. I felt the deep go quiet through the rock.
      Tell me.
    hints: Choose Into the Vein below to report.
  - questRequired:
    - 54-start
    questExcluded:
    - 54-crafted
    - 54-end
    text: Go on, then. The forge is north, east twice, then south from here. Put something
      through the fire. The iron dagger will do. Rusk sells the ingot and the strip.
      A craft can fail. If it does, buy the ingot and the strip again and retry.
    hints: At the forge in the Coulee Smithy, type craft iron dagger. A craft can
      fail. If it does, buy an iron ingot and a leather strip again and retry.
  - questRequired:
    - 54-crafted
    questExcluded:
    - 54-end
    text: I heard the anvil ring from here. Tell me how it went.
    hints: Choose Back to the Forge below to report.
  - questRequired:
    - 37-end
    questExcluded:
    - 91-start
    - 91-end
    text: You took the mine. This eye reads more than dark, and the deep has gone
      quiet. Crier Toke sent you to Rusk, and Rusk sent you to me. Toke will want
      to hear the end of it from you.
    hints: Choose Report to Toke (Smithing) below to carry the news to Toke.
  - questRequired:
    - 91-start
    questExcluded:
    - 91-end
    text: Toke is waiting on that news in Hub Square. Go on. I will still be here.
    hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - questRequired:
    - 37-end
    text: You walked out of the dark with the deep vein at your back. You came up
      here not knowing a hammer from your hand. Look at you.
    hints: You have finished the forge trail. When there is more forge work, choose
      Back to the Forge below.
topics:
- key: vein_arrive
  kind: info
  label: Smith Rusk sent me to clear the slope.
  text: Right. Here is what I am asking. There is a scree scavenger working the Lower
    Talus, just north of me. You will hear it before you see it. More of them crawl
    the Scree Field and the Upper Talus beyond. West of me, in the Ore Pocket, a talus
    lurker has settled in. Another ranges the high shoulder, up on the Upper Talus.
    Both kinds need to go before anyone can work that seam safely. The scavenger first,
    then the lurker. And then I want you to work the vein yourself, not just clear
    the way. Forage in the Ore Pocket or on the Scree Field. The rock will give up
    what it has. If a fight turns against you up there, flee. I mean it. If you die,
    you wake at the Mending Hut. The slope does not get to keep you. Come back,
    try again. The rock is patient. Check what they drop and put it on if it fits.
    Armor is armor up here.
  hints: Go north to the Lower Talus and type attack scavenger.
  questRequired:
  - 36-start
  questExcluded:
  - 36-met
  - 36-end
  moodChange: neutral
- key: vein_turnin
  kind: handin
  quest: 36
  label: Pick the Vein
  playerLine: The slope is clear, and I worked the vein.
  text: 'You actually did it. The pocket is clear. I can hear the difference in
    how the wind moves up there. And you worked the vein. Whatever it gave you is
    better than it looks raw. Basalt-iron forges dark and holds an edge longer than
    standard iron. Take this pouch. I carried it out of three claim sites and it has
    never been empty long. Wear it, and any crafting material you pick up goes into
    it on its own. If you have loose materials already, type sort and it gathers
    them in. When it fills, take it to the Strongbox House, just east of Hub Square,
    and type stow. Every material you carry goes into storage there in one step.
    And if you find yourself with worn gear you cannot use, salvage it. Type salvage
    and the name of the piece. The slope gives back what went into making it, roughly.
    That is the work, more or less.'
  hints: Quest complete. You receive a Component Pouch, a bag for crafting materials.
    To wear it, type equip component pouch. For more, type help component-bag.
  questRequired:
  - 36-ore
  questExcluded:
  - 36-end
  grantsQuest: 36-end
  moodChange: friendly
- key: mine_offer
  kind: offer
  quest: 37
  label: Into the Vein
  playerLine: Do you want the beast in the mine put down?
  text: Then here it is. The mine mouth is at the head of the slope, west of the upper
    talus, a black adit cut straight into the basalt. From the mouth it is down,
    west, down, then west twice to the bottom. Below the first landing there is no
    light at all, so type cast glow before you go down and keep it up. The workings
    run deep and there are things in them that have never seen the sun, but the one
    that matters is at the bottom, in the Den. The stone-blooded beast. End it.
    Then come back up and tell me, and bring the blade off the floor of that den
    if it is worth carrying. The deep vein is the reward, and it is a good one.
  hints: In the Den at the bottom of the mine, type attack beast. If it is dark,
    type cast glow first.
  breadcrumb: Clear the slope first. There is worse work below it, and I will not
    send you down there green.
  questRequired:
  - 36-end
  questExcluded:
  - 37-start
  - 37-end
  grantsQuest: 37-start
  moodChange: neutral
- key: mine_in
  kind: handin
  quest: 37
  label: Into the Vein
  playerLine: The stone-blooded beast is dead.
  text: Dead. I felt it go through the rock, the way you feel a storm break. The deep
    vein has sat untouched since before my cave-in, and now it is open. You did that.
    Keep the blade you carried up. And here is how to fold the dark ore into an edge
    of your own.
  hints: Quest complete. Crier Toke will want the news. Choose Report to Toke (Smithing)
    below.
  questRequired:
  - 37-slain
  questExcluded:
  - 37-end
  grantsQuest: 37-end
  moodChange: friendly
# The bridge comes before the repeatable, so the menu lists it first.
- key: report_smithing_offer
  kind: offer
  quest: 91
  label: Report to Toke (Smithing)
  playerLine: Should I take the news of the mine to Toke?
  text: Do. Toke sent you to Rusk, and Rusk sent you to me, and the man likes his
    news firsthand. He keeps to Hub Square, south of the Awakening Pool. Tell him
    the deep vein is open.
  hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk toke.
  questRequired:
  - 37-end
  questExcluded:
  - 91-start
  - 91-end
  grantsQuest: 91-start
  moodChange: friendly
- key: forge_loop_offer
  kind: offer
  quest: 54
  label: Back to the Forge
  playerLine: Do you have more forge work for me?
  text: The vein does not stop giving just because you stopped swinging. Go back to
    the forge and put something through the fire. I want to know the skill is still
    sharp in you. The iron dagger will do, and Rusk sells what it takes. A craft can
    fail. If it does, buy the ingot and the strip again and retry. Then come and tell
    me how it rang.
  hints: At the forge in the Coulee Smithy, type craft iron dagger. A craft can fail.
    If it does, buy an iron ingot and a leather strip again and retry.
  questRequired:
  - 37-end
  questExcluded:
  - 54-start
  - 54-end
  grantsQuest: 54-start
- key: forge_loop_in
  kind: handin
  quest: 54
  label: Back to the Forge
  playerLine: I put another piece through the forge.
  text: I heard it ring. Still sharp, then. The iron knows a practiced hand. Here,
    for your trouble. Come back when you want to keep it that way.
  hints: Quest complete. Ovell will have more forge work after a while.
  questRequired:
  - 54-crafted
  questExcluded:
  - 54-end
  grantsQuest: 54-end
  moodChange: friendly
- key: vein_scavenger_help
  kind: info
  label: Where is the scavenger?
  text: The scavenger first. One works the Lower Talus, just north of me. The lurker
    and the vein can wait until it is down.
  hints: Go north to the Lower Talus and type attack scavenger.
  questRequired:
  - 36-met
  questExcluded:
  - 36-scavenger
  - 36-end
- key: vein_lurker_help
  kind: info
  label: The scavenger is down. Where is the lurker?
  text: The scavenger is done. The lurker next. It holds the Ore Pocket, just west
    of me. Leave the vein until it is down.
  hints: Go west into the Ore Pocket and type attack lurker.
  questRequired:
  - 36-scavenger
  questExcluded:
  - 36-lurker
  - 36-end
- key: vein_forage_help
  kind: info
  label: Where do I work the vein?
  text: Both are down. Now the vein. It runs through the rubble in the Ore Pocket
    and shows in the bluff above the Scree Field. Forage it. Some days the rock gives
    a lot and some days a little, but working it is what I asked for.
  hints: In the Ore Pocket or on the Scree Field, type forage.
  questRequired:
  - 36-lurker
  questExcluded:
  - 36-ore
  - 36-end
# Shown once the pouch is in hand: quest 36's reward (itemid 30033) is given
# when the hand-in above grants 36-end.
- key: bag_help
  kind: info
  label: How does a component pouch work?
  text: The pouch does the sorting for you once you wear it. Type equip component
    pouch and it hangs at your belt. After that, any crafting material you pick up
    or buy goes straight in. If you have loose materials already sitting in your
    pack, type sort and the pouch gathers them. What is inside weighs less, so it
    carries lighter than it looks. When it fills up, go to the Strongbox House,
    just east of Hub Square, and type stow. Every crafting material you carry, in
    the pouch and in your pack, goes into storage there in one step. If you want
    to choose what goes in and what stays out, use storage instead. Salvage is the
    other half. Type salvage and the name of whatever you want to break down, like
    worn armor or a blade you cannot use, and the components come back out. You
    will not get everything, but you get enough to matter. The better your skill,
    the more comes back.
  hints: To wear the pouch, type equip component pouch. For more, type help
    component-bag.
  questRequired:
  - 36-end
- key: beast_help
  kind: info
  label: How do I reach the beast's den?
  text: The beast waits in the Den at the bottom of the mine. The mouth is west of
    the Upper Talus. From there it is down, west, down, then west twice. Type cast
    glow before you go down. The things in the workings are only in the way. The
    beast is the one that matters.
  hints: In the Den at the bottom of the mine, type attack beast. If it is dark,
    type cast glow first.
  questRequired:
  - 37-start
  questExcluded:
  - 37-slain
  - 37-end
# Gated on 36-end, when Ovell first names the mine (the 36-end opener above).
- key: dark_help
  kind: info
  label: How dark is it down in the mine?
  text: I am not joking about the dark down there. I spent six hours in it during
    the cave-in before the eye adjusted, and I was lucky to have the eye. You will
    not be. The mine below the slope runs deep and there is no light at all past
    the first landing. None. You need to make your own before you go down. Type
    cast glow before you take the descent. The glow lasts a while, then fades.
    When it fades, cast it again. Go in without light and you will be blind within
    a few steps, and whatever is in there will know it before you do.
  hints: When it is too dark to see, type cast glow. For more, type help
    chrysalis-glow.
  marked: true
  questRequired:
  - 36-end
- key: forge_loop_help
  kind: info
  label: Which forge should I use?
  text: The forge is north, east twice, then south from here. Put something through
    the fire there. The iron dagger will do, and Rusk sells the ingot and the strip.
    A craft can fail. If it does, buy the ingot and the strip again and retry. Then
    come back and tell me how it went.
  hints: At the forge in the Coulee Smithy, type craft iron dagger. A craft can fail.
    If it does, buy an iron ingot and a leather strip again and retry.
  questRequired:
  - 54-start
  questExcluded:
  - 54-crafted
  - 54-end
```

- [ ] **Step 13: Edit Rusk (`_datafiles/world/dogmud/dialogue/pothole_coulee/9116.yaml`).** Four Edit calls. Do not touch the opener variant gated `questRequired: [30-end]` / `questExcluded: [35-start, 35-end]` (plan 2's). If an `old_string` no longer matches, stop and report.

Edit 13a. Replace:

```yaml
  - questRequired:
    - 35-start
    questExcluded:
    - 35-list
    - 35-end
    text: First see what I stock.
```

with:

```yaml
  - questRequired:
    - 35-start
    questExcluded:
    - 35-met
    - 35-end
    text: Toke sent you. Good. I teach anyone who watches more than they talk. Ask
      me why you came, and I will tell you how the forge works.
    hints: Choose Crier Toke sent me to learn the forge below.
  - questRequired:
    - 35-met
    questExcluded:
    - 35-list
    - 35-end
    text: First see what I stock.
```

Edit 13b. Replace from the variant gated `questRequired: [35-craft]` through the line `topics:`, that is:

```yaml
  - questRequired:
    - 35-craft
    questExcluded:
    - 35-end
    text: You made something at the forge. Show it to me, or sell it to me if you
      would rather have the coin. Lost it? Then tell me how the work went.
    hints: Choose First Heat below to report to Rusk. If you still carry the dagger,
      she looks it over. If not, she takes your word. Selling her the dagger with
      sell iron dagger also counts.
  - questRequired:
    - 35-end
    text: You are through here. North through the Quench Shed, then west down to the
      talus. Ovell is in the Collapsed Cut, south of the Lower Talus. The man pulled
      himself out of a cave-in and has been sitting there since. He could use a hand.
      If things go wrong out there and you die, you wake at the Mending Hut.
    hints: Go north to the Quench Shed, west to the Lower Talus and south into the
      Collapsed Cut, and type talk ovell to find out what the cave-in survivor needs.
topics:
```

with:

```yaml
  - questRequired:
    - 35-craft
    questExcluded:
    - 35-end
    text: You made something at the forge. Show it to me. If you would rather have
      the coin, sell it to me first, then tell me how the work went. Lost it? Then
      tell me anyway.
    hints: Choose First Heat below to report to Rusk. If you still carry the dagger,
      she looks it over. If you sold or lost it, she takes your word.
  - questRequired:
    - 35-end
    questExcluded:
    - 36-start
    - 36-end
    text: You understand the loop now. There is a man on the talus slope, Ovell, who
      pulled himself out of a cave-in and has been sitting there since. He could use
      a hand that knows iron. Say the word and I will send you.
    hints: Choose Pick the Vein below to hear what Ovell needs.
  - questRequired:
    - 36-start
    questExcluded:
    - 36-met
    - 36-end
    text: Ovell is waiting. North through the Quench Shed, west twice to the Lower
      Talus, then south into the Collapsed Cut.
    hints: From the Coulee Smithy, go north, west twice, then south into the Collapsed
      Cut. Type talk ovell.
  - questRequired:
    - 35-end
    text: You are through here. The slope is Ovell's business now. If things go wrong
      out there and you die, you wake at the Mending Hut.
    hints: When you want more forge work, Survivor Ovell in the Collapsed Cut may
      have some.
topics:
```

Edit 13c. Replace the two hand-ins (from `- key: craft_turnin` through the `grantsQuest: 35-end` line of `craft_turnin_told`) with:

```yaml
- key: craft_turnin
  kind: handin
  quest: 35
  label: First Heat
  playerLine: I made a dagger at your forge. Here, see for yourself.
  text: Let me see it. Straight seam. Weight is forward where it belongs. You did
    not rush the quench. This is a working blade. Not fancy. Does not need to be.
    Keep it. You made it. You understand the loop now. Raw stock in, finished piece
    out. Apply that to anything the forge can make. And when you want more than the
    forge, I know a man on the talus slope who needs a hand.
  hints: Quest complete. Choose Pick the Vein below to hear what Ovell needs.
  questRequired:
  - 35-craft
  questExcluded:
  - 35-end
  carriesItem: 10009
  grantsQuest: 35-end
- key: craft_turnin_told
  kind: handin
  quest: 35
  label: First Heat
  playerLine: I made a dagger at your forge, but I don't have it now.
  text: Gone already? Then I take your word. The forge does not lie, and neither do
    you, I think. You understand the loop now. Raw stock in, finished piece out. Apply
    that to anything the forge can make. And when you want more than the forge, I
    know a man on the talus slope who needs a hand.
  hints: Quest complete. Choose Pick the Vein below to hear what Ovell needs.
  questRequired:
  - 35-craft
  questExcluded:
  - 35-end
  itemExcluded: 10009
  grantsQuest: 35-end
- key: heat_arrive
  kind: info
  label: Crier Toke sent me to learn the forge.
  text: Right. The forge teaches faster than I do, but here is the shape of it. Buy
    an iron ingot and a leather strip off my stall. List shows you what I stock and
    what it costs. Take them to the forge here in the smithy and type craft iron dagger.
    The iron and the leather disappear and a blade appears. That is the whole trick
    the first time. Once you have a feel for it, type craft list to see everything
    you know how to make. Show me what you made when you are done. Start with the
    list.
  hints: Here beside Rusk, type list.
  questRequired:
  - 35-start
  questExcluded:
  - 35-met
  - 35-end
- key: vein_offer
  kind: offer
  quest: 36
  label: Pick the Vein
  playerLine: Do you want me to help Ovell on the slope?
  text: Go and see him. Ovell was a prospector until the slope came down on him. He
    dug himself out of a cave-in and has been sitting in the Collapsed Cut since,
    watching scavengers walk over a good seam of basalt-iron. North through the Quench
    Shed, west twice to the Lower Talus, then south into the cut. If you die out there,
    you wake at the Mending Hut.
  hints: From the Coulee Smithy, go north, west twice, then south into the Collapsed
    Cut. Type talk ovell.
  questRequired:
  - 35-end
  questExcluded:
  - 36-start
  - 36-end
  grantsQuest: 36-start
```

Edit 13d. In `heat_list_help` replace:

```yaml
  questRequired:
  - 35-start
  questExcluded:
  - 35-list
  - 35-end
- key: heat_ingot_help
```

with:

```yaml
  questRequired:
  - 35-met
  questExcluded:
  - 35-list
  - 35-end
- key: heat_ingot_help
```

- [ ] **Step 14: Toke's smithing hand-in.** In `9106.yaml`, directly after the `report_fighting_in` topic (after its `moodChange: friendly` line), insert:

```yaml
- key: report_smithing_in
  kind: handin
  quest: 91
  label: Report to Toke (Smithing)
  playerLine: Ovell sends word. The beast in the mine is dead.
  text: The stone-blooded beast? Dead? Then the deep vein is open, and every smith in
    Gaius will want to know who opened it. Leave that to me. The forge trail is yours,
    and any mouth you have not walked yet is still open to you.
  hints: Quest complete. Toke can still send you down any trail you have not walked.
  questRequired:
  - 91-start
  questExcluded:
  - 91-end
  grantsQuest: 91-end
  moodChange: friendly
```

- [ ] **Step 14b: Toke's First Heat offer sends the player to Rusk** (route checked: Hub Square 5203 north 5200, west 5201, 5210, 5221, 5244, 5245). In `9106.yaml`, inside `send_forge`, replace:

```yaml
  text: Smith Rusk, at the Coulee Smithy. Few words and good iron. She will show you
    how raw stock becomes a blade. From this square go north to the pool, then follow
    the path west five times to the smithy. Start by looking over her stall.
  hints: From Hub Square go north, then west five times to the Coulee Smithy. Next
    to Rusk, type list.
```

with:

```yaml
  text: Smith Rusk, at the Coulee Smithy. Few words and good iron. She will show you
    how raw stock becomes a blade. From this square go north to the pool, then follow
    the path west five times to the smithy. Talk to her before you touch her stall.
  hints: From Hub Square, go north, then west five times to the Coulee Smithy. Type
    talk rusk.
```

Add to `TestTokeQuestOneOffers_SendToTheGiver` in `coulee_bridges_test.go`:

```go
	t.Run("Smithing", func(t *testing.T) { checkTokeSends(t, toke, "send_forge", 35, "rusk") })
```

- [ ] **Step 15: Manifests.** In `quests/step-ids.txt` insert `35-met` (between `35-list` and `35-start`), `36-met` (between `36-lurker` and `36-ore`), `37-slain` (after `37-end`), `54-crafted` (before `54-end`), and `91-end`, `91-start` (after `90-start`). In `dialogue/topic-keys.txt` add `9106.report_smithing_in`, `9116.heat_arrive`, `9116.vein_offer`, `9117.forge_loop_in`, `9117.mine_in`, `9117.report_smithing_offer`, `9117.vein_arrive` in sorted position, delete `9117.vein_offer`, and add under `# renamed`:

```text
9117.vein_offer -> 9116.vein_offer
```

- [ ] **Step 16: Delete the smithing backlog lines** from `quests/loop-backlog.txt`:

```text
chain 35 mob 9117 topic vein_offer
chain 37 mob 9117 topic forge_loop_offer
end 35 quest 35 trigger command room=5245 mob=9116 item=10009 command=sell grants=35-end
end 37 quest 37 trigger mob_death mob=9123 grants=37-end
end 37 step end
end 54 quest 54 trigger command room=5245 command=craft grants=54-end
end 54 step end
one-action 35 quest 35 trigger command room=5245 command=craft grants=35-craft:prev
walk-dead-end 35 quest 35 trigger command room=5245 command=craft grants=35-craft
```

Check (own line, expect no output):

```bash
grep -nE '^\S+ (35|36|37|54|91) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

- [ ] **Step 17: Run and pass; prove a test can fail.**

```bash
gofmt -l internal/
go test -count=1 ./internal/questengine/ ./internal/actions/ ./internal/guards/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: no gofmt output; all `ok`; the smoke tests listed as `--- PASS`, none `--- SKIP`. Then change the craft trigger's `has: ["35-strip"]` to `has: ["35-start"]`, run `go test -count=1 ./internal/questengine/ -run TestQuest35`, confirm `a craft before the strip must not skip it`, and restore.

- [ ] **Step 18: Narration golden.**

```bash
go test -count=1 ./internal/narration/ -run TestSnapshotStores -update
git diff --stat internal/narration/testdata/stores/
git diff internal/narration/testdata/stores/quests.golden | grep -E '^[+-]quest\|' | cut -c1-60
go test -count=1 ./internal/narration/
```

Expected: only `quests.golden`, only lines `quest|35|`, `quest|36|`, `quest|37|`, `quest|54|`, `quest|91|`.

- [ ] **Step 19: Commit.**

```bash
git add internal/questengine/newbie_forge_steps_test.go internal/questengine/first_heat_sellpath_test.go internal/questengine/newbie_one_command_steps_test.go internal/questengine/coulee_bridges_test.go internal/actions/sell_rusk_dagger_test.go _datafiles/world/dogmud/quests/35-first_heat.yaml _datafiles/world/dogmud/quests/36-pick_the_vein.yaml _datafiles/world/dogmud/quests/37-into_the_vein.yaml _datafiles/world/dogmud/quests/54-back_to_the_forge.yaml _datafiles/world/dogmud/quests/91-report_to_toke__smithing_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9117.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9116.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/narration/testdata/stores/quests.golden
git status --short
git commit -F - <<'EOF'
feat(quests): smithing line runs Toke, Rusk, Ovell, Toke (#375 phase 7)

First Heat (35) starts with a talk to Rusk and its craft gates on the
strip; selling the dagger draws Rusk's line and names the talk that
finishes the quest (#697). Rusk offers Pick the Vein (36), which starts
with a talk to Ovell. Into the Vein (37) and Back to the Forge (54) end
at Ovell's new hand-ins; Ovell offers Report to Toke (Smithing) (91).
Clears 9 loop-backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 5: The brewing line (38, 39, 40, 55, bridge 92)

Suggested executor: Opus.

**Files:**
- Replace: `internal/questengine/newbie_alchemy_steps_test.go`
- Modify: `internal/questengine/newbie_topic_steps_test.go` (Birna and Falv cases), `internal/questengine/coulee_bridges_test.go`
- Replace: `_datafiles/world/dogmud/quests/38-first_brew.yaml`, `39-wade_the_marsh.yaml`, `40-heart_of_the_mire.yaml`, `55-another_brew.yaml`
- Create: `_datafiles/world/dogmud/quests/92-report_to_toke__brewing_.yaml`
- Replace: `_datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml`, `9106.yaml`, `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Regenerate: `internal/narration/testdata/stores/quests.golden`

This task carries plan 1's bandolier wording too: potions are taken with `quaff`, so "when you drink, the oldest one goes first" (quest 39 reward, Falv's `marsh_turnin` and `bandolier`) becomes `quaff` in the text below.

- [ ] **Step 1: Replace `internal/questengine/newbie_alchemy_steps_test.go`** with:

```go
package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The brewing line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Brew (38) and Herbalist Birna takes it; Birna offers Wade the Marsh (39)
// and Fenwalker Falv takes it; Falv offers Heart of the Mire (40), then
// Report to Toke (Brewing) (92) and the repeatable Another Brew (55). Potions
// are taken with quaff (plan 1). One command per step, every quest ends at a
// hand-in.

const (
	glassVial    = 40006
	healersRoot  = 40004
	firebombItem = 30058
	marshLeech   = 9130
	reedLurker   = 9131
	birnaMob     = 9128
	falvMob      = 9129
	sheltered    = 5265 // Birna's pool, an alchemy bench
	dryingRacks  = 5267 // the second alchemy bench
	reedBeds     = 5269
	sunkenPath   = 5270
	cattailStand = 5271 // Falv's camp
	mudflat      = 5272
	blackPool    = 5273
)

// countingLinearCtx keeps count: progress, as the game's bridge does, so a
// count: N trigger needs N matches (the plain walker context reaches any
// count at once).
type countingLinearCtx struct {
	*linearCtx
	counts map[string]int
}

func (c *countingLinearCtx) BumpStepCount(questId int, step, key string) (int, bool) {
	c.counts[key]++
	return c.counts[key], true
}

func notifyCounting(w *questWalker, ctx *countingLinearCtx, event string, d EventDetails) {
	d.UserId = 1
	w.p.roomId = d.RoomId
	w.e.Notify(event, d, w.p, ctx)
}

func TestQuest38FirstBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "38-first_brew.yaml", 38)
	require.Equal(t, []string{"start", "met", "list", "vial", "roots", "brew", "drink", "end"}, newLinearPlayer(q).steps,
		"the old ids keep their places; met is inserted after start")
	turnIn := turnInRequires(t, "9128.yaml", "brew_turnin")
	require.Equal(t, []string{"38-drink"}, turnIn)
	heard := EventDetails{MobId: birnaMob, Topic: "brew_arrive", RoomId: sheltered}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		ctx := &countingLinearCtx{linearCtx: w.ctx, counts: map[string]int{}}

		notifyCounting(w, ctx, "command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "start", w.p.current, "listing before Birna has spoken must not skip the talk")
		notifyCounting(w, ctx, "topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "met", w.p.current, "a vial bought before listing must not skip the list step")
		notifyCounting(w, ctx, "command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "list", w.p.current, "a root before the vial must not skip the vial step")
		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current)

		notifyCounting(w, ctx, "command", EventDetails{Command: "quaff", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a quaff before the brew must not count")
		notifyCounting(w, ctx, "command", EventDetails{Command: "craft", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a craft before the roots must not count (no lenient grant)")

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "#695: one root is not the salve's two")
		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "roots", w.p.current, "the second root finishes the step")
		require.False(t, holdsAll(w.p, turnIn))

		notifyCounting(w, ctx, "command", EventDetails{Command: "craft", RoomId: dryingRacks})
		require.Equal(t, "brew", w.p.current, "the Drying Racks bench counts")
		notifyCounting(w, ctx, "command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "brew", w.p.current, "drink is for food and drink now; a potion is taken with quaff")
		notifyCounting(w, ctx, "command", EventDetails{Command: "quaff", RoomId: sheltered})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("the vial hint names a buy command that works", func(t *testing.T) {
		require.Contains(t, questHintFor(t, q, "vial"), "type buy 2 healer's root", "#695")
	})

	t.Run("Birna's talk step", func(t *testing.T) {
		checkArriveTopic(t, "birna", "9128.yaml", q, birnaMob, "brew_arrive", []string{"30-end", "31-end"})
	})
}

func TestQuest39WadeTheMarsh_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	require.Equal(t, []string{"start", "met", "leech", "throw", "forage", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9129.yaml", "marsh_turnin")
	require.Equal(t, []string{"39-forage"}, turnIn)
	heard := EventDetails{MobId: falvMob, Topic: "marsh_arrive", RoomId: cattailStand}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: reedBeds})
		require.Equal(t, "start", w.p.current, "a leech before Falv has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "met", w.p.current, "a forage before the leech must not skip ahead")
		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: reedBeds})
		require.Equal(t, "leech", w.p.current)

		// #694: a throw that reached nobody does not count.
		w.notify("command", EventDetails{Command: "throw", RoomId: sunkenPath, TargetKind: "none"})
		require.Equal(t, "leech", w.p.current, "a throw at nobody must not finish the throw step")
		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "leech", w.p.current, "a forage before the throw must not skip it")
		w.notify("command", EventDetails{Command: "throw", RoomId: mudflat, TargetKind: "mob", MobId: reedLurker})
		require.Equal(t, "throw", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
	})

	t.Run("Birna offers it with the firebomb and Falv no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9128.yaml", "marsh_offer", q, "38-end", "9129.yaml")
		require.Equal(t, firebombItem, loadCouleeTopics(t, "9128.yaml").FindTopic("marsh_offer").GivesItem)
	})

	t.Run("Falv's talk step", func(t *testing.T) {
		checkArriveTopic(t, "falv", "9129.yaml", q, falvMob, "marsh_arrive", []string{"30-end", "38-end"})
	})
}

func TestQuest40HeartOfTheMire_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "40-heart_of_the_mire.yaml", 40)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	turnIn := turnInRequires(t, "9129.yaml", "mire_in")
	require.Equal(t, []string{"40-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: blackPool})
	require.Equal(t, "start", w.p.current, "a leech is not the spirit")
	w.notify("mob_death", EventDetails{MobId: 9135, RoomId: 5280})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"40-end"}), "the kill no longer ends the quest")
}

func TestQuest55AnotherBrew_EndsAtFalv(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	require.Equal(t, []string{"start", "brew", "quaffed", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "brew"), "type quaff healing salve")
	require.Contains(t, questHintFor(t, q, "quaffed"), "type talk falv")
	turnIn := turnInRequires(t, "9129.yaml", "brew_loop_in")
	require.Equal(t, []string{"55-quaffed"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "quaff", RoomId: sheltered})
	require.Equal(t, "start", w.p.current, "a quaff before the brew must not count")
	w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
	require.Equal(t, "brew", w.p.current, "a craft at the Drying Racks bench counts")
	w.notify("command", EventDetails{Command: "quaff", RoomId: 5271})
	require.Equal(t, "quaffed", w.p.current)
	require.False(t, holdsAll(w.p, []string{"55-end"}), "the quaff no longer ends the run")
}

// Falv hands out a replacement firebomb while quest 39 is open. A player who
// still carries one must not get a second (S8): the handout topic carries
// itemExcluded, and its reminder (carriesItem, same gates) hands nothing out.
// Every firebomb handout on Falv is checked, so a new one cannot skip the
// rule. Birna's offer hands over the first firebomb.
func TestFalvFirebombHandoutGivesOnlyOne(t *testing.T) {
	falv := loadCouleeTopics(t, "9129.yaml")
	found := 0
	for i := range falv.Topics {
		tp := &falv.Topics[i]
		if tp.GivesItem != firebombItem || tp.GrantsQuest != "" {
			continue
		}
		found++
		require.Equal(t, firebombItem, tp.ItemExcluded, "topic %s hands a firebomb to a player who already carries one", tp.Key)
	}
	require.Equal(t, 1, found, "expected exactly one firebomb handout topic; the check below names it")

	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	n := newNpcTalker(falv, playerOn(q, "start", "38-end"))
	checkHandout(t, n, "firebomb_recovery", "firebomb_already_have", firebombItem)
	require.Contains(t, squash(falv.FindTopic("firebomb_already_have").Text), "already have one")
}
```

The const block mixes trailing comments: run `gofmt -l internal/questengine/` after writing; if it lists the file, align the `=` and the comments in the block by hand with Edit until it prints nothing.

- [ ] **Step 2: Replace the Birna and Falv topic cases.** In `internal/questengine/newbie_topic_steps_test.go` replace the bodies of `TestBirna9128_TopicsNameEachStep` and `TestFalv9129_TopicsNameEachStep` (both whole functions) with:

```go
func TestBirna9128_TopicsNameEachStep(t *testing.T) {
	f := "38-first_brew.yaml"
	pre := []string{"30-end", "31-end"}
	checkTopicSteps(t, "birna", "9128.yaml", []topicStep{
		{f, 38, "met", "type list", pre, "brew_step_list", false},
		{f, 38, "list", "type buy glass vial", pre, "brew_step_vial", false},
		{f, 38, "vial", "type buy 2 healer's root", pre, "brew_step_roots", false},
		{f, 38, "roots", "type craft healing salve", pre, "brew_step_craft", false},
		{f, 38, "brew", "type quaff healing salve", pre, "brew_step_drink", false},
		{f, 38, "drink", "", pre, "brew_turnin", true},
	})
}

func TestFalv9129_TopicsNameEachStep(t *testing.T) {
	f39 := "39-wade_the_marsh.yaml"
	pre39 := []string{"30-end", "38-end"}
	pre40 := []string{"30-end", "38-end", "39-end"}
	pre55 := []string{"30-end", "38-end", "39-end", "40-end"}
	checkTopicSteps(t, "falv", "9129.yaml", []topicStep{
		{f39, 39, "met", "type attack leech", pre39, "marsh_step_leech", false},
		{f39, 39, "leech", "type throw firebomb", pre39, "marsh_step_throw", false},
		{f39, 39, "throw", "type forage", pre39, "marsh_step_forage", false},
		{f39, 39, "forage", "", pre39, "marsh_turnin", true},
		{"40-heart_of_the_mire.yaml", 40, "start", "type attack spirit", pre40, "mire_step_help", false},
		{"40-heart_of_the_mire.yaml", 40, "slain", "", pre40, "mire_in", true},
		{"55-another_brew.yaml", 55, "start", "type craft healing salve", pre55, "brew_loop_step_craft", false},
		{"55-another_brew.yaml", 55, "brew", "type quaff healing salve", pre55, "brew_loop_step_drink", false},
		{"55-another_brew.yaml", 55, "quaffed", "", pre55, "brew_loop_in", true},
	})
}
```

- [ ] **Step 3: Add the brewing bridge case** to `coulee_bridges_test.go`, after the `Smithing` block:

```go
	t.Run("Brewing", func(t *testing.T) {
		checkBridge(t, toke, 92, "Brewing", "92-report_to_toke__brewing_.yaml",
			"9129.yaml", "report_brewing_offer", "report_brewing_in", "40-end")
	})
```

- [ ] **Step 4: Run and see them fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestQuest38|TestQuest39|TestQuest40|TestQuest55|TestBirna9128|TestFalv9129|TestCouleeBridges'
```

Expected: `FAIL` (step lists differ, `no topic "brew_arrive"`, the 92 file missing).

- [ ] **Step 5: Replace `_datafiles/world/dogmud/quests/38-first_brew.yaml`** with:

```yaml
# Quest 38, First Brew (Pothole Coulee, brewing line, quest 1).
#
# Crier Toke offers it (send_brew, dialogue 9106). Herbalist Birna (9128)
# takes the hand-in (brew_turnin) and then offers Wade the Marsh (39).
#   start  go to the Sheltered Pool and talk to Birna
#   met    topic_heard on Birna's brew_arrive topic
#   list   list at Birna's pool (5265)
#   vial   item_gain of a Glass Vial (40006)
#   roots  item_gain of a Healer's Root (40004), count 2 (#695); foraged
#          roots count too
#   brew   craft at an alchemy bench (5265 or 5267), after the roots
#   drink  quaff (potions are taken with quaff since plan 1); the step id
#          stays "drink" because saved characters hold it
#   end    Birna's hand-in
# Healing salve is a skill_minimum 0 recipe, so the lesson is the loop, not
# the recipe. The reward grants the minor-antidote recipe and two vials.

questid: 38
name: First Brew
description: >-
  Herbalist Birna keeps the drying-racks at the sheltered pool, and the
  first brew waits there. Buy a glass vial and a pair of healer's roots
  from her stall, grind and steep them at the alchemy bench into a healing
  salve, then quaff it down.
secret: false

steps:
  - id: start
    destination:
      mob: 9128
      room: 5265
    description: >-
      Crier Toke has sent you to Herbalist Birna at the Sheltered Pool. From
      Hub Square, go south to Market Row, east twice to the Reedwash Mouth,
      then south twice to her pool.
    hint: >-
      From Hub Square, go south, east, east, south, south to the Sheltered
      Pool. Type talk birna.
  - id: met
    destination:
      room: 5265
    description: >-
      Birna has told you how a brew is made. First see what she sells. Her
      racks behind her are her stall.
    hint: >-
      At Birna's pool, type list.
  - id: list
    destination:
      room: 5265
    description: >-
      You have seen Birna's stock. A healing salve needs one glass vial and
      two healer's roots. Start with the vial.
    hint: >-
      Type buy glass vial.
  - id: vial
    destination:
      room: 5265
    description: >-
      You have a glass vial. Now the roots. A healing salve takes two.
      Foraged roots work just as well as bought ones.
    hint: >-
      A healing salve takes two roots. Type buy 2 healer's root.
  - id: roots
    destination:
      room: 5265
    description: >-
      You have the glass vial and the roots. The bench at Birna's pool is an
      alchemy bench, and so is the one on the Drying Racks just west of it.
      Type craft list at a bench to see everything you can make.
    hint: >-
      At the alchemy bench, type craft healing salve. A craft can fail. If
      it does, buy a glass vial and two healer's roots again and retry.
  - id: brew
    destination: anywhere
    description: >-
      You brewed a healing salve. Now take it. A potion is taken with
      quaff, and a brew is only worth what it does.
    hint: >-
      Type quaff healing salve.
  - id: drink
    destination:
      mob: 9128
      room: 5265
    description: >-
      The salve went to work, warm and steady. Tell Birna how it felt.
    hint: >-
      Go back to Herbalist Birna at the Sheltered Pool and type talk birna.
  - id: end
    description: >-
      Birna has named you brewer enough to leave the pool, and she knows a
      fenwalker in the marsh who will teach you the rest.

rewards:
  skillinfo: "alchemy:1"
  recipe_info: "minor-antidote"
  item_info: "40006:2"
  gold: 15
  actor: >-
    Birna takes the empty vial back and turns it to the light, checking
    the film the salve left behind. The green-gold veining in her hands
    catches the same light. "Clean steep. You did not scorch the root.
    Most do, the first time." She tucks a pair of empty vials into your
    pack and presses a folded scrap of waxed paper after them. "The
    antidote formula. You will want it before the marsh teaches you why.
    Set your home here first, so you have somewhere to wake."
  observer: "Birna inspects the empty vial, nods, and tucks a few
    things into the newcomer's pack."

triggers:
  # met: hearing Birna's brew_arrive topic.
  - event: topic_heard
    mob: 9128
    topic: brew_arrive
    conditions:
      has: ["38-start"]
      missing: ["38-met"]
    actions:
      - grant: "38-met"
      - actor: >-
          Birna has shown you the shape of it. Start with her racks. Type
          list.

  # list: Birna's stall (room 5265).
  - event: command
    command: list
    room: 5265
    conditions:
      has: ["38-met"]
      missing: ["38-list"]
    actions:
      - grant: "38-list"
      - actor: >-
          That is Birna's stock. A healing salve needs one glass vial and
          two healer's roots. Type buy glass vial.

  # vial: a glass vial in hand, after the list.
  - event: item_gain
    item: 40006
    conditions:
      has: ["38-list"]
      missing: ["38-vial"]
    actions:
      - grant: "38-vial"
      - actor: >-
          The glass vial will hold the salve. Now the roots, two of them.
          Type buy 2 healer's root.

  # roots: two healer's roots in hand, after the vial (#695).
  - event: item_gain
    item: 40004
    count: 2
    label: Healer's roots
    conditions:
      has: ["38-vial"]
      missing: ["38-roots"]
    actions:
      - grant: "38-roots"
      - actor: >-
          Two healer's roots, pale and bitter-smelling. With the vial, you
          have a salve's worth. At the alchemy bench, type craft healing
          salve.

  # brew: craft at an alchemy bench, after the roots.
  - event: command
    command: craft
    room: 5265
    conditions:
      has: ["38-roots"]
      missing: ["38-brew"]
    actions:
      - grant: "38-brew"
      - actor: >-
          The root ground to paste, the water drew it green, and the
          healing salve is sealed in the vial. Type quaff healing salve.
  - event: command
    command: craft
    room: 5267
    conditions:
      has: ["38-roots"]
      missing: ["38-brew"]
    actions:
      - grant: "38-brew"
      - actor: >-
          The root ground to paste, the water drew it green, and the
          healing salve is sealed in the vial. Type quaff healing salve.

  # drink: quaff the salve (anywhere).
  - event: command
    command: quaff
    conditions:
      has: ["38-brew"]
      missing: ["38-drink"]
    actions:
      - grant: "38-drink"
      - actor: >-
          The salve spreads warm through you, a slow steady mending.
          That is what a brew is for. Go back to Birna and type talk
          birna.
```

- [ ] **Step 6: Replace `_datafiles/world/dogmud/quests/39-wade_the_marsh.yaml`** with:

```yaml
# Quest 39, Wade the Marsh (Pothole Coulee, brewing line, quest 2).
#
# Herbalist Birna (9128) offers it (marsh_offer) once First Brew (38) is
# handed in, and hands over one of Falv's firebombs (givesItem 30058).
# Fenwalker Falv (9129) takes the hand-in (marsh_turnin) and then offers
# Heart of the Mire (40). Falv replaces a lost firebomb (firebomb_recovery).
#   start   go to the Cattail Stand and talk to Falv
#   met     topic_heard on Falv's marsh_arrive topic
#   leech   kill a marsh leech (9130)
#   throw   a throw that reaches a creature (target: mob, #694)
#   forage  forage the marsh (rooms 5270 or 5273); a forage that finds
#           nothing still counts
#   end     Falv's hand-in
# A thrown firebomb, a cast spell and a special move share one cooldown.

questid: 39
name: Wade the Marsh
description: >-
  Fenwalker Falv keeps a camp on a hummock in the reeds. The marsh past his
  fire is no sanctuary, with leeches in the water and a serpent lurking in
  the cattails. Herbalist Birna has sent you to him with a firebomb, to blood
  yourself on the marsh: put a leech down, learn to throw the firebomb in a
  fight, and forage the good herbs while you are at it.
secret: false

steps:
  - id: start
    destination:
      mob: 9129
      room: 5271
    description: >-
      Herbalist Birna hands you a firebomb when you take this on and sends
      you to Fenwalker Falv at his camp in the marsh. From the Sheltered
      Pool, go south twice to the Reed Beds, then east to the Cattail Stand.
    hint: >-
      From the Sheltered Pool, go south twice to the Reed Beds, then east to
      the Cattail Stand. Type talk falv.
  - id: met
    destination:
      room: 5269
    description: >-
      Falv has set you loose in the marsh. First put down a marsh leech.
      Keep the firebomb for after the leech is down. If a fight turns bad,
      type flee. If you fall, you wake at the Mending Hut, not in the mud.
    hint: >-
      Go west from Falv's camp to the Reed Beds and type attack leech. If
      it is dark, type cast glow first.
  - id: leech
    destination:
      room: 5272
    description: >-
      One marsh leech is down. Now the firebomb. Throw it where the blast
      reaches a creature, and the fight starts with a burst of flame. A
      throw at empty ground does not count. A thrown firebomb, a cast spell
      and a special move all share one cooldown, so you get one such trick
      every few breaths. Another leech waits at the Black Pool, and the
      reed lurker hunts the Mudflat south of Falv's camp.
    hint: >-
      With a creature in the room, type throw firebomb.
  - id: throw
    destination:
      room: 5270
    description: >-
      The firebomb burst where you wanted it. Now forage the marsh herbs
      Falv asked for. The Sunken Path and the Black Pool south of it grow
      the best of them. Some days your hands come up empty, and the search
      still counts.
    hint: >-
      On the Sunken Path or at the Black Pool, type forage.
  - id: forage
    destination:
      mob: 9129
      room: 5271
    description: >-
      You have blooded yourself, thrown the firebomb, and worked the marsh
      for herbs. Report back to Falv at his camp.
    hint: >-
      Go back to Fenwalker Falv at the Cattail Stand and type talk falv.
  - id: end
    description: >-
      Falv has named you marsh-worthy and given you a bandolier for your
      brews. The poison swamp lies south past the miasma edge, and something
      old has gathered at its heart.

rewards:
  skillinfo: "alchemy:1"
  itemid: 20059
  recipe_info: "ironhide-brew,firebomb"
  gold: 25
  actor: >-
    Falv works a leather bandolier off a peg by his fire and tosses it
    to you underhand, the loops already supple from years of use. The
    shed-proof skin of his hands pales over the knuckles as he grips.
    "Wear it. Potions you carry drop into it on their own, and when you
    quaff, the oldest one goes first, so your brews never sit long enough
    to turn. You threw clean. You did not blow your own hand off. That is
    more than most." He nods south, where the reeds go grey. "Past the
    miasma the water is poison and something rules it. The firebomb
    formula is yours now, and a tougher brew besides. Make your own pots.
    You will need them down there."

triggers:
  # met: hearing Falv's marsh_arrive topic.
  - event: topic_heard
    mob: 9129
    topic: marsh_arrive
    conditions:
      has: ["39-start"]
      missing: ["39-met"]
    actions:
      - grant: "39-met"
      - actor: >-
          Falv has told you what the marsh will try. The leech first. Go
          west to the Reed Beds and type attack leech.

  # leech: a marsh leech (9130).
  - event: mob_death
    mob: 9130
    conditions:
      has: ["39-met"]
      missing: ["39-leech"]
    actions:
      - grant: "39-leech"
      - actor: >-
          The marsh leech goes limp in the water. First blood in the
          marsh. Now the firebomb. With a creature in the room, type throw
          firebomb.

  # throw: the blast must reach a creature (#694). A throw at nobody raises
  # its event with no target, which target: mob does not match.
  - event: command
    command: throw
    target: mob
    conditions:
      has: ["39-leech"]
      missing: ["39-throw"]
    actions:
      - grant: "39-throw"
      - actor: >-
          The firebomb bursts in a gout of flame right where you aimed it.
          Now you feel the cost: no second trick for a few breaths. That
          cooldown is shared by spells and special moves too. Next, on the
          Sunken Path or at the Black Pool, type forage.

  # forage: the marsh (rooms 5270 / 5273), after the throw. A forage that
  # turns up nothing still counts, so these lines claim no find.
  - event: command
    command: forage
    room: 5270
    conditions:
      has: ["39-throw"]
      missing: ["39-forage"]
    actions:
      - grant: "39-forage"
      - actor: >-
          That is foraging the marsh: working the firm ground between the
          cattails for what grows there. Some days it gives little, and
          that is part of it. Go back to Falv and type talk falv.
  - event: command
    command: forage
    room: 5273
    conditions:
      has: ["39-throw"]
      missing: ["39-forage"]
    actions:
      - grant: "39-forage"
      - actor: >-
          The black pool's waterline is the richest ground in the marsh,
          even on a day it gives up little. Go back to Falv and type talk
          falv.
```

- [ ] **Step 7: Replace `_datafiles/world/dogmud/quests/40-heart_of_the_mire.yaml`** with:

```yaml
# Quest 40, Heart of the Mire (Pothole Coulee, brewing line finale).
#
# Fenwalker Falv (9129) offers it (mire_offer) after Wade the Marsh (39).
# The player descends past the miasma edge into the sunken mire and kills
# the Spirit of the Swamp (9135) at the Heart of the Mire (5280).
#   start  kill the spirit
#   slain  report to Falv
#   end    Falv's hand-in mire_in, which pays a Vitality gain, a potion
#          stockpile, the Mire Renewal Draught recipe and gold

questid: 40
name: Heart of the Mire
description: >-
  Past the miasma edge the marsh turns to poison swamp, and the ground sinks
  into a sunken mire where the water itself seems to watch. Falv says
  something old has gathered at the heart of it. Brew your kit, fill your
  bandolier, put it down, and tell Falv.
secret: false

steps:
  - id: start
    destination:
      room: 5280
    description: >-
      Falv has set you on the mire. The spirit of the swamp is a hard fight,
      so brew healing salves and carry firebombs before you go. From the
      Black Pool, go south past the Miasma Edge and the Fenrot Approach to
      the Sulphur Flats. The sinkhole there drops down into the Sunken
      Hollow, and the Heart of the Mire lies two rooms south of it, past the
      Black Mere. Consider a foe to size it up, and flee if a fight turns
      against you.
    hint: >-
      Go down the sinkhole at the Sulphur Flats, then south to the Heart of
      the Mire, and type attack spirit. If it is dark, type cast glow first.
  - id: slain
    destination:
      mob: 9129
      room: 5271
    description: >-
      The spirit of the swamp is broken and the mire runs clear. Falv waits
      at the Cattail Stand to hear it.
    hint: >-
      Climb back out of the mire to Fenwalker Falv at the Cattail Stand. Type
      talk falv.
  - id: end
    description: >-
      The spirit of the swamp is broken and the mire runs clear. You came to
      the pool not knowing one herb from another. You leave a brewer, with
      the marsh's own renewal recipe in your head.

rewards:
  stat_info: "vitality:3"
  item_info: "30036:3,30028:2,30058:2"
  recipe_info: "mire-renewal-draught"
  gold: 50
  actor: >-
    Falv hears you out with his hands held over the fire, water running
    off them in threads. "Gone, then. I felt the water change." He fills
    your pack with a brewer's stock from his own crates and tells you the
    mire's own renewal, willow-bark and mint and root, until the recipe
    settles in your head. The long labor of the marsh has settled into
    your body as a lasting toughness.
  observer: ""

triggers:
  # slain: the Spirit of the Swamp (9135) falls.
  - event: mob_death
    mob: 9135
    conditions:
      has: ["40-start"]
      missing: ["40-slain"]
    actions:
      - grant: "40-slain"
      - actor: >-
          The spirit comes apart all at once, mud and black water and rot
          falling still and ordinary. A spring breaks clear through the
          rock behind it, and the poison drains out of the air. Climb back
          out to Falv at the Cattail Stand and type talk falv.
```

- [ ] **Step 8: Replace `_datafiles/world/dogmud/quests/55-another_brew.yaml`** with:

```yaml
# Quest 55, Another Brew (Pothole Coulee, brewing line repeatable).
#
# Fenwalker Falv (9129) offers it (brew_loop_offer) once Heart of the Mire
# (40) is handed in. Repeatable with a cooldown; completion clears progress.
#   start    craft at an alchemy bench (5265 or 5267)
#   brew     quaff the brew (anywhere)
#   quaffed  report to Falv
#   end      Falv's hand-in brew_loop_in (#375 phase 7)

questid: 55
name: Another Brew
description: >-
  Falv figures anyone worth keeping alive stays current with their kit. Brew
  something at the alchemy bench, quaff it down, and tell Falv it held.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5265
    description: >-
      Falv wants you to brew at the alchemy bench and take what you make.
      The benches are at Birna's Sheltered Pool and on the Drying Racks west
      of it. Any recipe counts. A healing salve takes two healer's roots and
      a glass vial.
    hint: >-
      At the alchemy bench, type craft healing salve. A craft can fail. If
      it does, buy the ingredients again and retry.
  - id: brew
    destination: anywhere
    description: >-
      You made the brew. Now take it. A potion is taken with quaff.
    hint: >-
      Type quaff healing salve.
  - id: quaffed
    destination:
      mob: 9129
      room: 5271
    description: >-
      The brew held. Falv waits at the Cattail Stand to hear it.
    hint: >-
      Go back to Fenwalker Falv at the Cattail Stand. Type talk falv.
  - id: end
    description: >-
      The brew held. Falv will send you back to the bench before long.

rewards:
  gold: 6
  skillinfo: "alchemy:1"
  actor: >-
    Falv sniffs the empty bottle and nods. "Fresh. The marsh does not care
    how long ago you learned, only whether your salve is fresh." He presses
    a few coins into your hand.
  observer: "Falv sniffs an empty bottle and nods at {actor}."

triggers:
  - event: command
    command: craft
    room: 5265
    conditions:
      has: ["55-start"]
      missing: ["55-brew"]
    actions:
      - grant: "55-brew"
      - actor: >-
          The brew came off the bench clean, steeped to the right color.
          Type quaff healing salve, or quaff whatever you made.
  - event: command
    command: craft
    room: 5267
    conditions:
      has: ["55-start"]
      missing: ["55-brew"]
    actions:
      - grant: "55-brew"
      - actor: >-
          The brew came off the bench clean, steeped to the right color.
          Type quaff healing salve, or quaff whatever you made.

  - event: command
    command: quaff
    conditions:
      has: ["55-brew"]
      missing: ["55-quaffed"]
    actions:
      - grant: "55-quaffed"
      - actor: >-
          The brew does what it promised. Go back to Falv at the Cattail
          Stand and type talk falv.
```

- [ ] **Step 9: Create `_datafiles/world/dogmud/quests/92-report_to_toke__brewing_.yaml`:**

```yaml
# Quest 92, Report to Toke (Brewing) (Pothole Coulee, brewing line bridge).
#
# Fenwalker Falv (9129) offers it (report_brewing_offer) once Heart of the
# Mire (40) is handed in. Crier Toke (9106) takes the hand-in
# (report_brewing_in). Same shape as quest 90.

questid: 92
name: Report to Toke (Brewing)
description: >-
  Fenwalker Falv has asked you to carry the news of the mire to Crier Toke
  in Hub Square.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # follow Crier Toke (schedule pothole_toke)
      room: 5203  # fallback when Crier Toke is not spawned
    description: >-
      The spirit of the swamp is gone and the mire runs clear. Crier Toke,
      who sent you to Herbalist Birna, waits in Hub Square for the news.
    hint: >-
      Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - id: end
    description: >-
      Toke has your news, and the marsh folk will be drinking to it tonight.
```

- [ ] **Step 10: Replace `_datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml`** (Fenwalker Falv) with:

```yaml
mobid: 9129
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: Falv. Watch the water side.
  hints: Choose a line below to learn about the marsh or your task.
  variants:
  - questExcluded:
    - 38-end
    text: You are not ready for the marsh. Go back to Birna at the pool and learn
      to brew first. Out here a salve in your pack is the difference between walking
      home and not. Come find me when you can make one.
    hints: Go west to the Reed Beds, then north twice to Herbalist Birna at the Sheltered
      Pool, and finish her brewing lesson before coming back to Falv.
  - questRequired:
    - 38-end
    questExcluded:
    - 39-start
    - 39-end
    text: Birna's brewer, then. She sends me the ones worth teaching. If she has not
      sent you out yet, ask her about the marsh.
    hints: Go west to the Reed Beds, then north twice to the Sheltered Pool. Type talk
      birna.
  - questRequired:
    - 39-start
    questExcluded:
    - 39-met
    - 39-end
    text: Birna sent you. Good. Come to the fire and I will tell you what the marsh
      will try to do to you.
    hints: Choose Herbalist Birna sent me to learn the marsh below.
  - questRequired:
    - 39-met
    questExcluded:
    - 39-leech
    - 39-end
    text: Still dry, I see. First the water. Leeches sit in the Reed Beds just west
      of my fire. Put one down. Keep the firebomb for after.
    hints: Go west to the Reed Beds and type attack leech.
  - questRequired:
    - 39-leech
    questExcluded:
    - 39-throw
    - 39-end
    text: One leech down. Now the firebomb. Find something else that wants to bite
      you, another leech at the Black Pool or the lurker on the Mudflat south of here,
      and throw it so the blast reaches it. One trick every few breaths, remember.
    hints: With a creature in the room, type throw firebomb.
  - questRequired:
    - 39-throw
    questExcluded:
    - 39-forage
    - 39-end
    text: You threw clean. Now the herbs. The Sunken Path and the Black Pool south
      of it grow the best of them. Some days your hands come up empty. That still
      counts.
    hints: On the Sunken Path or at the Black Pool, type forage.
  - questRequired:
    - 39-forage
    questExcluded:
    - 39-end
    text: You have the look of the marsh on you. Tell me how it went.
    hints: Choose Wade the Marsh below to report.
  - questRequired:
    - 39-end
    questExcluded:
    - 40-start
    - 40-end
    text: You wear the bandolier well enough. There is one thing left, and it is the
      bad one. Past the miasma edge the water turns poison, and something old has
      settled at the heart of it. When you are ready to finish this, I will tell you
      where to find it.
    hints: Choose Heart of the Mire below when you are ready for the final task.
  - questRequired:
    - 40-start
    questExcluded:
    - 40-slain
    - 40-end
    text: It is still down there. Brew your kit, fill the bandolier, and go put the
      spirit of the swamp down. Past the Black Pool and the Miasma Edge, keep south
      to the Sulphur Flats. The sinkhole there takes you under.
    hints: Go down the sinkhole at the Sulphur Flats, then south to the Heart of the
      Mire, and type attack spirit.
  - questRequired:
    - 40-slain
    questExcluded:
    - 40-end
    text: You came back out of the mire, and the water smells different. Tell me.
    hints: Choose Heart of the Mire below to report.
  - questRequired:
    - 55-start
    questExcluded:
    - 55-brew
    - 55-end
    text: Back to the bench with you. Make something. A healing salve will do. A craft
      can fail. If yours does, buy the ingredients again and go at it once more.
    hints: At the alchemy bench, type craft healing salve. A craft can fail. If it
      does, buy the ingredients again and retry.
  - questRequired:
    - 55-brew
    questExcluded:
    - 55-quaffed
    - 55-end
    text: You brewed. Now take it. A potion goes down with quaff.
    hints: Type quaff healing salve.
  - questRequired:
    - 55-quaffed
    questExcluded:
    - 55-end
    text: Brewed and taken, by the look of you. Tell me how it held.
    hints: Choose Another Brew below to report.
  - questRequired:
    - 40-end
    questExcluded:
    - 92-start
    - 92-end
    text: You broke it. The water runs clean off the spring now, first time in a long
      time. Toke sent you down this road. He will want the news from you.
    hints: Choose Report to Toke (Brewing) below to carry the news to Toke.
  - questRequired:
    - 92-start
    questExcluded:
    - 92-end
    text: Toke is waiting on that news in Hub Square. Go on. The marsh will keep.
    hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - questRequired:
    - 40-end
    text: You came in a pool-brewer and you walk out something the marsh respects.
      That is rarer than you know. The world is bigger than this mud.
    hints: You have finished the brewing trail. When there is more work at the bench,
      choose Another Brew below.
topics:
- key: marsh_arrive
  kind: info
  label: Herbalist Birna sent me to learn the marsh.
  text: Right. Leeches in the water, a serpent in the cattails. The marsh will blood
    you if you let it. That firebomb Birna gave you bursts into flame where it lands.
    You throw it in a fight, with a creature in front of you. But mind this. A thrown
    firebomb, a cast spell and a special move all pull from the same well. Use one
    and you wait a few breaths before the next. So make it count. Then you wade in,
    in this order. Put a marsh leech down in the Reed Beds west of here to learn the
    water. Throw the firebomb at the next thing that wants to bite you. Forage the
    herbs on the Sunken Path or at the Black Pool. Type flee if a fight turns. If you
    fall, you wake at the Mending Hut, not in the mud. Lose the firebomb before you
    throw it, and I will give you another.
  hints: Go west to the Reed Beds and type attack leech.
  questRequired:
  - 39-start
  questExcluded:
  - 39-met
  - 39-end
# The replacement and the reminder never show together: itemExcluded and
# carriesItem on the same item are exact opposites.
- key: firebomb_recovery
  kind: info
  label: I've lost my firebomb.
  text: 'Lost it already? Here. Last one I am giving free. Throw this one: type throw
    firebomb with a foe in front of you. Do not just carry it around.'
  hints: You received another firebomb. With a foe in the room, type throw firebomb.
  questRequired:
  - 39-start
  questExcluded:
  - 39-throw
  itemExcluded: 30058
  givesItem: 30058
- key: firebomb_already_have
  kind: info
  label: Can I have a second firebomb?
  text: You already have one. I can see it on you. Put a leech down first, then throw
    that one at the next thing that comes at you. I do not hand out a second while
    the first is still in your pack.
  hints: You still carry the firebomb Birna gave you.
  questRequired:
  - 39-start
  questExcluded:
  - 39-throw
  carriesItem: 30058
- key: marsh_turnin
  kind: handin
  quest: 39
  label: Wade the Marsh
  playerLine: The leech is down, the firebomb thrown, and the herbs found.
  text: Leech down, firebomb thrown, herbs in your pack. That is the marsh, near enough.
    Here, a bandolier. Your potions drop into it on their own and the oldest goes
    first when you quaff, so nothing sits long enough to turn. You will want it for
    what is next. Past the miasma edge the water goes poison, and something rules it.
    When your nerve is up, I will tell you about the mire.
  hints: Quest complete. Choose Heart of the Mire below when you are ready for the
    final task.
  questRequired:
  - 39-forage
  questExcluded:
  - 39-end
  grantsQuest: 39-end
- key: marsh_step_leech
  kind: info
  label: Where do I find a leech?
  text: Leech first. They sit in the Reed Beds, just west of my fire, fat and slow.
    Put one down and the marsh will know you. Keep the firebomb until after.
  hints: Go west to the Reed Beds and type attack leech.
  questRequired:
  - 39-met
  questExcluded:
  - 39-leech
  - 39-end
- key: marsh_step_throw
  kind: info
  label: The leech is down. What now?
  text: 'Now the firebomb. Find a foe: another leech at the Black Pool, or the lurker
    out on the Mudflat south of here. Throw it so the blast reaches it, and the fight
    starts in flame. A throw at empty ground is wasted. Then you wait a few breaths
    before your next trick. Spells and special moves share that wait.'
  hints: With a creature in the room, type throw firebomb.
  questRequired:
  - 39-leech
  questExcluded:
  - 39-throw
  - 39-end
- key: marsh_step_forage
  kind: info
  label: Where do the best herbs grow?
  text: 'Herbs. The Sunken Path and the Black Pool just south of it grow the best
    in the marsh: cattail, willow-bark, mint. You will not always find something.
    The looking is the lesson.'
  hints: On the Sunken Path or at the Black Pool, type forage.
  questRequired:
  - 39-throw
  questExcluded:
  - 39-forage
  - 39-end
- key: mire_offer
  kind: offer
  quest: 40
  label: Heart of the Mire
  playerLine: Do you want the thing in the mire put down?
  text: The bad one, then. Past the miasma edge the marsh turns to poison swamp.
    The air alone will turn your stomach. The ground sinks down into a sunken mire,
    and at the heart of it the water has gathered into something with a will. A spirit,
    an aberration, I do not have the word. It is the worst thing in this country.
    Brew your salves, fill the bandolier, carry firebombs. From the Black Pool go
    south past the Miasma Edge to the Sulphur Flats, take the sinkhole down, and put
    it down. Then come back and tell me. Do not go in light.
  hints: Go down the sinkhole at the Sulphur Flats, then south to the Heart of the
    Mire, and type attack spirit.
  breadcrumb: Learn the marsh first. There is something worse past the miasma, and
    it will keep until you are ready.
  questRequired:
  - 39-end
  questExcluded:
  - 40-start
  - 40-end
  grantsQuest: 40-start
- key: mire_step_help
  kind: info
  label: How do I reach the heart of the mire?
  text: From the Black Pool, south past the Miasma Edge and the Fenrot Approach to
    the Sulphur Flats. There is a sinkhole in the crust. Drop down it into the hollow,
    then go south past the Black Mere. The spirit waits at the heart. Bring salves.
  hints: Go down the sinkhole at the Sulphur Flats, then south to the Heart of the
    Mire, and type attack spirit.
  questRequired:
  - 40-start
  questExcluded:
  - 40-slain
  - 40-end
- key: mire_in
  kind: handin
  quest: 40
  label: Heart of the Mire
  playerLine: The spirit of the swamp is gone.
  text: Gone. I felt the water change before you came in sight. That thing sat at
    the heart of my marsh longer than I have camped here, and you put it down. Take
    a brewer's stock from my crates, and the mire's own renewal recipe. You will make
    it better than I can.
  hints: Quest complete. Crier Toke will want the news. Choose Report to Toke (Brewing)
    below.
  questRequired:
  - 40-slain
  questExcluded:
  - 40-end
  grantsQuest: 40-end
  moodChange: friendly
- key: cooldown_help
  kind: info
  label: How does throwing a firebomb work?
  text: Throwing a firebomb is simple. Type throw firebomb in a fight. The catch
    is the well it draws from. A thrown firebomb, a cast spell, a special move like
    a kick or a taunt, they all share one timer. Spend it and you wait a few breaths
    before you can spend it again. Out here that pause can get you killed, so do not
    waste a trick on the wrong moment.
  hints: Type throw firebomb in combat. Throwing, casting, and special moves share
    one cooldown. You get one such action every few breaths.
  marked: true
# The bridge comes before the repeatable, so the menu lists it first.
- key: report_brewing_offer
  kind: offer
  quest: 92
  label: Report to Toke (Brewing)
  playerLine: Should I take the news of the mire to Toke?
  text: Go on, then. Toke sent you to Birna, and Birna sent you to me. He keeps to
    Hub Square, south of the Awakening Pool. Tell him the mire runs clean.
  hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk toke.
  questRequired:
  - 40-end
  questExcluded:
  - 92-start
  - 92-end
  grantsQuest: 92-start
- key: brew_loop_offer
  kind: offer
  quest: 55
  label: Another Brew
  playerLine: Do you have more bench work for me?
  text: The marsh does not care when you last brewed. Get back to the bench at Birna's
    pool, make something, quaff it. A healing salve will do. A craft can fail. If
    yours does, buy the ingredients again and go at it once more. Then come and tell
    me how it held.
  hints: At the alchemy bench, type craft healing salve. A craft can fail. If it does,
    buy the ingredients again and retry.
  questRequired:
  - 40-end
  questExcluded:
  - 55-start
  - 55-brew
  - 55-end
  grantsQuest: 55-start
- key: brew_loop_step_craft
  kind: info
  label: Which bench should I use?
  text: Bench first. The one at Birna's pool, or the one on her Drying Racks. Make
    something. A healing salve will do. A craft can fail. If yours does, buy the ingredients
    again and go at it once more.
  hints: At the alchemy bench, type craft healing salve. A craft can fail. If it does,
    buy the ingredients again and retry.
  questRequired:
  - 55-start
  questExcluded:
  - 55-brew
  - 55-end
- key: brew_loop_step_drink
  kind: info
  label: I've brewed. What now?
  text: You brewed. Now quaff it down, then come and tell me how it held.
  hints: Type quaff healing salve.
  questRequired:
  - 55-brew
  questExcluded:
  - 55-quaffed
  - 55-end
- key: brew_loop_in
  kind: handin
  quest: 55
  label: Another Brew
  playerLine: I brewed something and quaffed it down.
  text: Fresh, and it held. Good. The marsh does not care how long ago you learned,
    only whether your salve is fresh. Here, for the trouble.
  hints: Quest complete. Falv will have more bench work after a while.
  questRequired:
  - 55-quaffed
  questExcluded:
  - 55-end
  grantsQuest: 55-end
- key: hands
  kind: info
  label: Why does water run off your hands?
  text: The Awakening took to my hands. Water runs off them like off a duck's back
    now. Useful, out here. I do not talk about it and neither should you.
- key: bandolier
  kind: info
  label: What is a bandolier for?
  text: A bandolier holds your potions ready to hand. They drop into it on their own,
    and the oldest goes first when you quaff. Clear the marsh and one is yours.
```

- [ ] **Step 11: Edit Birna (`_datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml`).** Five Edit calls. Do not touch the variant gated `questRequired: [30-end]` / `questExcluded: [38-start, 38-end]` (plan 2's). If an `old_string` no longer matches, stop and report.

Edit 11a. Replace:

```yaml
  - questRequired:
    - 38-start
    questExcluded:
    - 38-list
    - 38-end
```

with:

```yaml
  - questRequired:
    - 38-start
    questExcluded:
    - 38-met
    - 38-end
    text: Toke sent you to the pool? Good. Ask me why you came, and I will tell you
      how a brew is made.
    hints: Choose Crier Toke sent me to learn to brew below.
  - questRequired:
    - 38-met
    questExcluded:
    - 38-list
    - 38-end
```

(The `text:` and `hints:` lines that followed the old block stay where they are and now belong to the `38-met` variant.)

Edit 11b. Replace from the variant gated `questRequired: [38-vial]` through `topics:`:

```yaml
  - questRequired:
    - 38-vial
    questExcluded:
    - 38-roots
    - 38-end
    text: 'You have the vial. Now the roots: two healer''s roots for one salve. Buy
      them from me, or forage your own if you know where they grow.'
    hints: Type buy 2 healers root.
  - questRequired:
    - 38-roots
    questExcluded:
    - 38-brew
    - 38-end
    text: Vial and roots. That is a salve's worth. The bench is right here at the
      water's edge, and there is a second one on the Drying Racks, west of the pool.
      A craft can fail. If it does, buy a glass vial and two roots again and try once
      more.
    hints: At the alchemy bench, type craft healing salve. A craft can fail. If it
      does, buy a glass vial and two healer's roots again and retry.
  - questRequired:
    - 38-brew
    questExcluded:
    - 38-drink
    - 38-end
    text: You made the salve. A brew is only worth what it does. Drink it down and
      feel it work.
    hints: Type drink healing salve.
  - questRequired:
    - 38-drink
    questExcluded:
    - 38-end
    text: You have brewed and you have drunk. Tell me how it went.
    hints: Choose First Brew below to report.
  - questRequired:
    - 38-end
    text: You are brewer enough to leave the pool. South, past the verge, the marsh
      begins, and Falv keeps a camp in the reeds. He is rough company but he knows
      the water. Set your home here first if you have not.
    hints: 'Type sethome coulee to make this your home, where you wake if you die.
      Fenwalker Falv camps on
      the Cattail Stand: south twice to the Reed Beds, then east.'
topics:
```

with:

```yaml
  - questRequired:
    - 38-vial
    questExcluded:
    - 38-roots
    - 38-end
    text: You have the vial. Now the roots. Two healer's roots for one salve. Buy
      both from me in one go, or forage your own if you know where they grow.
    hints: Type buy 2 healer's root.
  - questRequired:
    - 38-roots
    questExcluded:
    - 38-brew
    - 38-end
    text: Vial and roots. That is a salve's worth. The bench is right here at the
      water's edge, and there is a second one on the Drying Racks, west of the pool.
      A craft can fail. If it does, buy a glass vial and two roots again and try once
      more.
    hints: At the alchemy bench, type craft healing salve. A craft can fail. If it
      does, buy a glass vial and two healer's roots again and retry.
  - questRequired:
    - 38-brew
    questExcluded:
    - 38-drink
    - 38-end
    text: You made the salve. A brew is only worth what it does. A potion goes down
      with quaff. Take it and feel it work.
    hints: Type quaff healing salve.
  - questRequired:
    - 38-drink
    questExcluded:
    - 38-end
    text: You have brewed and you have taken it. Tell me how it went.
    hints: Choose First Brew below to report.
  - questRequired:
    - 38-end
    questExcluded:
    - 39-start
    - 39-end
    text: You are brewer enough to leave the pool. South past the verge the marsh
      begins, and Falv keeps a camp in the reeds. He is rough company, but he knows
      the water, and he will teach you the rest if I send you. Say the word.
    hints: Choose Wade the Marsh below to hear about the marsh.
  - questRequired:
    - 39-start
    questExcluded:
    - 39-met
    - 39-end
    text: Falv is waiting at the Cattail Stand. South twice to the Reed Beds, then
      east.
    hints: From the Sheltered Pool, go south twice to the Reed Beds, then east to the
      Cattail Stand. Type talk falv.
  - questRequired:
    - 38-end
    text: Set your home here first if you have not. The marsh is Falv's country now.
    hints: Type sethome coulee to make this your home, where you wake if you die.
topics:
```

Edit 11c. Replace the whole `brew_turnin` topic (from `- key: brew_turnin` through its `grantsQuest: 38-end` line) with:

```yaml
- key: brew_turnin
  kind: handin
  quest: 38
  label: First Brew
  playerLine: I brewed a healing salve and quaffed it.
  text: Warm and steady, was it, not a jolt, a mending. That is a salve doing its
    work. You understand the loop now. Gather or buy, work the bench, take the result.
    Apply it to anything you learn to make. Here, the antidote formula, and a couple
    of empty vials to keep you brewing. When you want to learn the marsh, ask me
    about it. Set your home here first. You will want somewhere to wake.
  hints: Quest complete. Type sethome coulee to make this your home, where you wake
    if you die. Choose Wade the Marsh below when you are ready.
  questRequired:
  - 38-drink
  questExcluded:
  - 38-end
  grantsQuest: 38-end
- key: brew_arrive
  kind: info
  label: Crier Toke sent me to learn to brew.
  text: Right. Brewing is patience and clean hands, and the bench does the rest. Here
    is the shape of it. A healing salve takes two healer's roots and one glass vial,
    and both are on my racks. First see what I stock and what it costs. Then buy the
    vial, then the roots. Take them to the alchemy bench at the water's edge and craft
    a healing salve. The roots and the water become the brew, sealed in the vial.
    Then quaff it and feel what you made. A brew you have not tasted is just a guess.
    Come back when you have brewed and taken it.
  hints: At Birna's pool, type list.
  questRequired:
  - 38-start
  questExcluded:
  - 38-met
  - 38-end
- key: marsh_offer
  kind: offer
  quest: 39
  label: Wade the Marsh
  playerLine: Do you want me to learn the marsh from Falv?
  text: Then take this to him. It is one of Falv's firebombs. He leaves a few with
    me for the brewers I send him. Ceramic, packed tight, and it bursts into flame
    where it lands, so do not drop it. Falv camps on the Cattail Stand, south twice
    to the Reed Beds and then east. He is rough, but he knows the water.
  hints: Accept, and Birna hands you a firebomb. From the Sheltered Pool, go south
    twice to the Reed Beds, then east. Type talk falv.
  questRequired:
  - 38-end
  questExcluded:
  - 39-start
  - 39-end
  grantsQuest: 39-start
  givesItem: 30058
```

Edit 11d. In `brew_step_list` replace:

```yaml
  questRequired:
  - 38-start
  questExcluded:
  - 38-list
  - 38-end
- key: brew_step_vial
```

with:

```yaml
  questRequired:
  - 38-met
  questExcluded:
  - 38-list
  - 38-end
- key: brew_step_vial
```

Edit 11e. Replace the `brew_step_roots` text and hints and the `brew_step_drink` topic:

```yaml
  text: You have your vial. Now two healer's roots. Pale, bitter things. Buy them
    from me, or forage your own if you know where they grow. Either kind works.
  hints: Type buy 2 healers root.
```

with:

```yaml
  text: You have your vial. Now two healer's roots. Pale, bitter things. Buy both
    from me in one go, or forage your own if you know where they grow. Either kind
    works.
  hints: Type buy 2 healer's root.
```

and:

```yaml
  text: The salve is made. Drink it. You will feel it mend you, slow and warm.
  hints: Type drink healing salve.
```

with:

```yaml
  text: The salve is made. Quaff it. You will feel it mend you, slow and warm.
  hints: Type quaff healing salve.
```

- [ ] **Step 12: Toke's brewing hand-in.** In `9106.yaml`, directly after the `report_smithing_in` topic, insert:

```yaml
- key: report_brewing_in
  kind: handin
  quest: 92
  label: Report to Toke (Brewing)
  playerLine: Falv sends word. The spirit of the swamp is gone.
  text: Gone? The thing at the heart of the mire? Hoy! Birna will brew a toast and
    Falv will pretend he did not want one. The brewing trail is yours, and any mouth
    you have not walked yet is still open to you.
  hints: Quest complete. Toke can still send you down any trail you have not walked.
  questRequired:
  - 92-start
  questExcluded:
  - 92-end
  grantsQuest: 92-end
  moodChange: friendly
```

- [ ] **Step 12b: Toke's First Brew offer sends the player to Birna** (route checked: Hub Square 5203 south 5204, east 5207, east 5222, south 5264, south 5265). The `text` already only gives the way; replace the `hints` of `send_brew` in `9106.yaml`:

```yaml
  hints: From Hub Square go south, east, east, south, south to the Sheltered Pool.
    At Birna's pool, type list.
```

with:

```yaml
  hints: From Hub Square, go south, east, east, south, south to the Sheltered Pool.
    Type talk birna.
```

Add to `TestTokeQuestOneOffers_SendToTheGiver` in `coulee_bridges_test.go`:

```go
	t.Run("Brewing", func(t *testing.T) { checkTokeSends(t, toke, "send_brew", 38, "birna") })
```

- [ ] **Step 13: Manifests.** In `quests/step-ids.txt` insert `38-met` (between `38-list` and `38-roots`), `39-met` (between `39-leech` and `39-start`), `40-slain` (between `40-end` and `40-start`), `55-quaffed` (between `55-end` and `55-start`), and `92-end`, `92-start` (after `91-start`). In `dialogue/topic-keys.txt` add `9106.report_brewing_in`, `9128.brew_arrive`, `9128.marsh_offer`, `9129.brew_loop_in`, `9129.marsh_arrive`, `9129.mire_in`, `9129.report_brewing_offer` in sorted position, delete `9129.marsh_offer`, and add under `# renamed`:

```text
9129.marsh_offer -> 9128.marsh_offer
```

- [ ] **Step 14: Delete the brewing backlog lines** from `quests/loop-backlog.txt`:

```text
chain 38 mob 9129 topic marsh_offer
chain 40 mob 9129 topic brew_loop_offer
end 40 quest 40 trigger mob_death mob=9135 grants=40-end
end 40 step end
end 55 quest 55 trigger command command=drink grants=55-end
end 55 step end
one-action 38 quest 38 trigger command room=5265 command=craft grants=38-brew:prev
one-action 38 quest 38 trigger command room=5267 command=craft grants=38-brew:prev
walk-dead-end 38 quest 38 trigger command room=5265 command=craft grants=38-brew
walk-dead-end 38 quest 38 trigger command room=5267 command=craft grants=38-brew
```

If plan 1 already rewrote quest 55's drink trigger to `quaff`, the `end 55 quest 55 trigger command command=drink ...` line will have become `... command=quaff ...` in the backlog; delete whichever spelling is there. Check (own line, expect no output):

```bash
grep -nE '^\S+ (38|39|40|55|92) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

- [ ] **Step 15: Run and pass; prove a test can fail.**

```bash
gofmt -l internal/
go test -count=1 ./internal/questengine/ ./internal/guards/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: no gofmt output; all `ok`; smoke tests `--- PASS`, none `--- SKIP`. Then remove `target: mob` from quest 39's throw trigger, rerun `go test -count=1 ./internal/questengine/ -run TestQuest39`, confirm `a throw at nobody must not finish the throw step`, and restore.

- [ ] **Step 16: Narration golden.**

```bash
go test -count=1 ./internal/narration/ -run TestSnapshotStores -update
git diff --stat internal/narration/testdata/stores/
git diff internal/narration/testdata/stores/quests.golden | grep -E '^[+-]quest\|' | cut -c1-60
go test -count=1 ./internal/narration/
```

Expected: only `quests.golden`, only lines `quest|38|`, `quest|39|`, `quest|40|`, `quest|55|`, `quest|92|`.

- [ ] **Step 17: Commit.**

```bash
git add internal/questengine/newbie_alchemy_steps_test.go internal/questengine/newbie_topic_steps_test.go internal/questengine/coulee_bridges_test.go _datafiles/world/dogmud/quests/38-first_brew.yaml _datafiles/world/dogmud/quests/39-wade_the_marsh.yaml _datafiles/world/dogmud/quests/40-heart_of_the_mire.yaml _datafiles/world/dogmud/quests/55-another_brew.yaml _datafiles/world/dogmud/quests/92-report_to_toke__brewing_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9128.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/narration/testdata/stores/quests.golden
git status --short
git commit -F - <<'EOF'
feat(quests): brewing line runs Toke, Birna, Falv, Toke (#375 phase 7)

First Brew (38) starts with a talk to Birna, counts two healer's roots
with a buy command that works (#695), brews only after the roots, and
is taken with quaff. Birna offers Wade the Marsh (39) with the firebomb;
its throw step needs a throw that reaches a creature (#694). Heart of
the Mire (40) and Another Brew (55) end at Falv's new hand-ins; Falv
offers Report to Toke (Brewing) (92). Bandolier lines say quaff.
Clears 10 loop-backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 6: The tracking line (41, 42, 43, 56, bridge 93) and the jerkin

Suggested executor: Opus.

**Files:**
- Replace: `internal/questengine/newbie_wilderness_steps_test.go`
- Modify: `internal/questengine/newbie_one_command_steps_test.go` (`TestQuest41FirstSign_OneCommandPerStep`), `internal/questengine/coulee_bridges_test.go`
- Replace: `_datafiles/world/dogmud/quests/41-first_sign.yaml`, `42-the_hunt.yaml`, `43-break_the_pack.yaml`, `56-running_game.yaml`
- Create: `_datafiles/world/dogmud/quests/93-report_to_toke__tracking_.yaml`
- Replace: `_datafiles/world/dogmud/dialogue/pothole_coulee/9137.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9136.yaml`, `9106.yaml`, `_datafiles/world/dogmud/recipes/tailoring/scab-hide-jerkin.yaml`, `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`
- Regenerate: `internal/narration/testdata/stores/quests.golden`

Task 1 must be merged into the branch first (quest 42's eat step listens for the `eat` event).

- [ ] **Step 1: Replace `internal/questengine/newbie_wilderness_steps_test.go`** with:

```go
package questengine

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tracking line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Sign (41) and Scout Tarn (9136) takes it; Tarn offers The Hunt (42) with a
// salt pouch and Hunter Delk (9137) takes it; Delk offers Break the Pack (43),
// then Report to Toke (Tracking) (93) and the repeatable Running Game (56).
// One command per step, every quest ends at a hand-in.

const (
	wildPronghorn  = 9139
	wildAlpha      = 9143
	wildRawMeat    = 40014
	wildSaltPouch  = 40017
	wildDelk       = 9137
	wildTarn       = 9136
	wildDelkCamp   = 5290 // Hunter's Hollow, the cooking_fire station
	wildOpenSteppe = 5284
	wildSteppeEdge = 5283
	wildThornbrake = 5289
)

func wildOneTypedCommand(t *testing.T, hint string) {
	t.Helper()
	n := len(typedCommand.FindAllString(strings.ReplaceAll(hint, darkAside, ""), -1))
	require.Equal(t, 1, n, "a hint names exactly one command: %q", hint)
}

func TestQuest42TheHunt_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	require.Equal(t, []string{"start", "met", "track", "hunt", "meat", "rest", "cook", "eaten", "end"}, newLinearPlayer(q).steps,
		"talk, track, kill, take the meat, sleep, cook, eat, report; the old ids keep their places")
	turnIn := turnInRequires(t, "9137.yaml", "hunt_turnin")
	require.Equal(t, []string{"42-eaten"}, turnIn)
	heard := EventDetails{MobId: wildDelk, Topic: "hunt_arrive", RoomId: wildDelkCamp}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "track", RoomId: wildThornbrake})
		require.Equal(t, "start", w.p.current, "tracking before Delk has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildDelkCamp})
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		w.notify("command", EventDetails{Command: "eat", RoomId: wildDelkCamp})
		require.Equal(t, "met", w.p.current, "only the track starts the hunt")

		w.notify("command", EventDetails{Command: "track", RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current)
		w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current, "only a pronghorn is the kill")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: wildSaltPouch, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current, "only the meat is the meat")
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		require.Equal(t, "meat", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "meat", w.p.current, "cooking before the rest must not skip it")
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildThornbrake})
		require.Equal(t, "rest", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: wildThornbrake})
		require.Equal(t, "rest", w.p.current, "a craft away from Delk's fire is not the cook")
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "cook", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the meal to be eaten")

		w.notify("command", EventDetails{Command: "eat", RoomId: wildDelkCamp, ItemId: 30022})
		require.Equal(t, "eaten", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("each hint names its step's command", func(t *testing.T) {
		want := map[string]string{
			"start": "type talk delk",
			"met":   "type track pronghorn",
			"track": "type attack pronghorn",
			"hunt":  "type loot corpse",
			"meat":  "type sleep",
			"rest":  "type craft grilled meat",
			"cook":  "type eat grilled meat",
			"eaten": "type talk delk",
		}
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			wildOneTypedCommand(t, s.Hint)
			require.Contains(t, strings.ToLower(s.Hint), want[s.Id], "step %s", s.Id)
		}
		require.Contains(t, q.Steps[1].Hint, darkAside, "the hunt is a kill quest in rooms that go dark")
	})

	t.Run("Tarn offers it with the salt and Delk no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9136.yaml", "hunt_offer", q, "41-end", "9137.yaml")
		require.Equal(t, wildSaltPouch, loadCouleeTopics(t, "9136.yaml").FindTopic("hunt_offer").GivesItem)
	})

	t.Run("Delk's talk step", func(t *testing.T) {
		checkArriveTopic(t, "delk", "9137.yaml", q, wildDelk, "hunt_arrive", []string{"30-end", "41-end"})
	})
}

func TestQuest42_DelkMatchesEachStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	f := "42-the_hunt.yaml"
	pre := []string{"30-end", "41-end"}
	carriesSalt := func(p *linearPlayer) { p.items[wildSaltPouch] = true }
	checkTopicStepsWith(t, "delk", "9137.yaml", carriesSalt, []topicStep{
		{f, 42, "met", "type track pronghorn", pre, "hunt_progress_start", false},
		{f, 42, "track", "type attack pronghorn", pre, "hunt_progress_track", false},
		{f, 42, "hunt", "type loot corpse", pre, "hunt_progress_hunt", false},
		{f, 42, "meat", "type sleep", pre, "hunt_progress_meat", false},
		{f, 42, "rest", "type craft grilled meat", pre, "hunt_progress_rest", false},
		{f, 42, "cook", "type eat grilled meat", pre, "hunt_progress_cook", false},
		{f, 42, "eaten", "", pre, "hunt_turnin", true},
	})

	t.Run("the salt is handed out once", func(t *testing.T) {
		n := newNpcTalker(loadCouleeTopics(t, "9137.yaml"), playerOn(q, "hunt", "41-end"))
		checkHandout(t, n, "salt_recovery", "salt_have", wildSaltPouch)
	})

	t.Run("Delk's field help teaches eating", func(t *testing.T) {
		tp := loadCouleeTopics(t, "9137.yaml").FindTopic("hunt_help")
		require.NotNil(t, tp)
		require.Contains(t, squash(tp.Text), "type eat grilled meat")
	})
}

func TestQuest43BreakThePack_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "43-break_the_pack.yaml", 43)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	start := q.Steps[0]
	wildOneTypedCommand(t, start.Hint)
	require.Contains(t, strings.ToLower(start.Hint), "type attack alpha")
	require.Contains(t, start.Hint, darkAside)
	turnIn := turnInRequires(t, "9137.yaml", "pack_in")
	require.Equal(t, []string{"43-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: 9140, RoomId: 5298})
	require.Equal(t, "start", w.p.current, "a hound is not the alpha")
	w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: 5298})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"43-end"}), "the kill no longer ends the quest")

	pre := []string{"30-end", "41-end", "42-end"}
	checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
		{"43-break_the_pack.yaml", 43, "start", "type attack alpha", pre, "pack_progress", false},
		{"43-break_the_pack.yaml", 43, "slain", "", pre, "pack_in", true},
	})
}

// The jerkin (quest 43's recipe reward) is sewable by a fresh player once
// known (spec "Skill coverage" ruling).
func TestScabHideJerkin_SkillMinimumOne(t *testing.T) {
	raw := string(repoFile(t, "_datafiles", "world", "dogmud", "recipes", "tailoring", "scab-hide-jerkin.yaml"))
	require.Regexp(t, regexp.MustCompile(`(?m)^skill_minimum: 1\r?$`), raw)
}

func TestQuest56RunningGame_EndsAtDelk(t *testing.T) {
	q := loadShippedQuest(t, "56-running_game.yaml", 56)
	require.Equal(t, []string{"start", "hunt", "foraged", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9137.yaml", "game_loop_in")
	require.Equal(t, []string{"56-foraged"}, turnIn)

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "start", w.p.current, "forage before the kill must not count")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)
		w.notify("command", EventDetails{Command: "forage", RoomId: wildDelkCamp})
		require.Equal(t, "hunt", w.p.current, "the forage that counts is on the Open Steppe")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "foraged", w.p.current)
		require.False(t, holdsAll(w.p, []string{"56-end"}), "the forage no longer ends the run")
	})

	t.Run("hints say what completes each step", func(t *testing.T) {
		start, hunt, foraged := q.Steps[0], q.Steps[1], q.Steps[2]
		wildOneTypedCommand(t, start.Hint)
		wildOneTypedCommand(t, hunt.Hint)
		wildOneTypedCommand(t, foraged.Hint)
		require.Contains(t, strings.ToLower(start.Hint), "type attack pronghorn")
		require.Contains(t, start.Hint, darkAside)
		require.Contains(t, strings.ToLower(hunt.Hint), "type forage")
		require.Contains(t, hunt.Hint, "Open Steppe")
		require.Contains(t, strings.ToLower(foraged.Hint), "type talk delk")
	})

	t.Run("Delk greets each step", func(t *testing.T) {
		f := "56-running_game.yaml"
		pre := []string{"30-end", "41-end", "42-end", "43-end"}
		checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
			{f, 56, "start", "type attack pronghorn", pre, "game_progress_start", false},
			{f, 56, "hunt", "type forage", pre, "game_progress_hunt", false},
			{f, 56, "foraged", "", pre, "game_loop_in", true},
		})
	})
}

func TestQuest41_TrackStepCarriesDarkAside(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Contains(t, q.Steps[1].Hint, darkAside)
}

func TestQuest41_TarnAnswersQuestWords(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	f := "41-first_sign.yaml"
	pre := []string{"30-end", "31-end"}
	checkTopicSteps(t, "tarn", "9136.yaml", []topicStep{
		{f, 41, "met", "type track hare", pre, "sign_progress_start", false},
		{f, 41, "track", "type forage", pre, "sign_progress_track", false},
		{f, 41, "forage", "", pre, "sign_turnin", true},
	})
	checkArriveTopic(t, "tarn", "9136.yaml", q, wildTarn, "sign_arrive", pre)
}
```

Run `gofmt -l internal/questengine/` after writing; if it lists the file, fix the const block alignment by hand with Edit.

- [ ] **Step 2: Replace the quest 41 walk test.** In `internal/questengine/newbie_one_command_steps_test.go` replace the whole function `TestQuest41FirstSign_OneCommandPerStep` (from its `func` line through its closing `}`, before `func TestQuest50FirstShot_OneCommandPerStep`) with:

```go
func TestQuest41FirstSign_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Equal(t, []string{"start", "met", "track", "forage", "end"}, newLinearPlayer(q).steps,
		"start, track and forage are what a saved character stores; met is inserted after start")
	turnIn := turnInRequires(t, "9136.yaml", "sign_turnin")
	heard := EventDetails{Command: "", MobId: 9136, Topic: "sign_arrive", RoomId: 5283}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "track", RoomId: 5284})
		require.Equal(t, "start", w.p.current, "tracking before Tarn has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: 5284})
		require.Equal(t, "met", w.p.current, "forage out of order must not skip the track step")
		require.False(t, holdsAll(w.p, turnIn), "turn-in must not open early")

		w.notify("command", EventDetails{Command: "track", RoomId: 5284})
		require.Equal(t, "track", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: 5285})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on track resumes", func(t *testing.T) {
		w := newQuestWalker(q).at("track")
		w.notify("command", EventDetails{Command: "forage", RoomId: 5284})
		require.Equal(t, "forage", w.p.current)
	})

	t.Run("player saved on forage can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("forage")
		require.True(t, holdsAll(w.p, turnIn))
	})
}
```

- [ ] **Step 3: Add the tracking bridge case** to `coulee_bridges_test.go`, after the `Brewing` block:

```go
	t.Run("Tracking", func(t *testing.T) {
		checkBridge(t, toke, 93, "Tracking", "93-report_to_toke__tracking_.yaml",
			"9137.yaml", "report_tracking_offer", "report_tracking_in", "43-end")
	})
```

- [ ] **Step 4: Run and see them fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestQuest41|TestQuest42|TestQuest43|TestQuest56|TestScabHideJerkin|TestCouleeBridges'
```

Expected: `FAIL` (step lists differ, `no topic "hunt_arrive"`, `skill_minimum: 1` not found, the 93 file missing).

- [ ] **Step 5: Replace `_datafiles/world/dogmud/quests/41-first_sign.yaml`** with:

```yaml
# Quest 41, First Sign (Pothole Coulee, tracking line, quest 1).
#
# Crier Toke offers it (send_steppe, dialogue 9106). Scout Tarn (9136) takes
# the hand-in (sign_turnin) and then offers The Hunt (42).
#   start   go to the Steppe Edge and talk to Tarn
#   met     topic_heard on Tarn's sign_arrive topic
#   track   track on the Open Steppe (5284)
#   forage  forage on the steppe (5284 or 5285), after the track
#   end     Tarn's hand-in
# Both commands send the command event only when they ran (not on a
# cooldown, a wrong-ground refusal, or track stop).

questid: 41
name: First Sign
description: >-
  Scout Tarn keeps watch over the scrub steppe south of the coulee, and the
  ground there is a book if you can read it. Pick up the trail of a steppe
  hare and track it, then forage the steppe for what it offers.
secret: false

steps:
  - id: start
    destination:
      mob: 9136
      room: 5283
    description: >-
      Crier Toke has sent you to Scout Tarn at the Steppe Edge. From Hub
      Square, go west twice along the stilt-house walk and into the scrub
      draw, then south twice through the scrub mouth.
    hint: >-
      From Hub Square, go west twice, then south twice to the Steppe Edge.
      Type talk tarn.
  - id: met
    destination:
      room: 5284
    description: >-
      The steppe is your first lesson in reading the ground. Start with the
      tracks. A steppe hare has crossed the Open Steppe, just south of Scout
      Tarn.
    hint: >-
      Go south to the Open Steppe and type track hare. If it is dark, type
      cast glow first.
  - id: track
    destination:
      room: 5284
    description: >-
      You read the ground for the hare's trail. Next, see what the scrub
      will give you.
    hint: >-
      Type forage.
  - id: forage
    destination:
      mob: 9136
      room: 5283
    description: >-
      You tracked and you foraged. Scout Tarn is waiting at the Steppe Edge,
      north of the Open Steppe.
    hint: >-
      Go back to Scout Tarn on the Steppe Edge and type talk tarn.
  - id: end
    description: >-
      Tarn has named you fit to read the ground, and she knows a hunter in
      the fringe who will teach you the rest.

rewards:
  skillinfo: "search:1"
  itemid: 20024
  gold: 15
  actor: >-
    Tarn watches you come back with the unhurried attention of someone
    measuring whether you wasted her time. The amber of her eyes catches
    a hare's bolt half a field off and dismisses it. "You followed the
    line instead of staring at your feet. Most never learn that." She
    works a rolled cloak off her pack and tosses it to you. "Steppe wind
    cuts after dark."
  observer: "Tarn looks the newcomer over, nods once, and hands across
    a rolled traveling cloak."

triggers:
  # met: hearing Tarn's sign_arrive topic.
  - event: topic_heard
    mob: 9136
    topic: sign_arrive
    conditions:
      has: ["41-start"]
      missing: ["41-met"]
    actions:
      - grant: "41-met"
      - actor: >-
          Tarn has told you what to look for. Go south to the Open Steppe
          and type track hare.

  # track: read the ground for the hare (room 5284, Open Steppe).
  - event: command
    command: track
    room: 5284
    conditions:
      has: ["41-met"]
      missing: ["41-track"]
    actions:
      - grant: "41-track"
      - actor: >-
          That is tracking: reading the ground for what passed and where it
          went. The skill gets better each time you use it. Next, type
          forage.

  # forage: gather on the steppe (5284 / 5285), after the track.
  - event: command
    command: forage
    room: 5284
    conditions:
      has: ["41-track"]
      missing: ["41-forage"]
    actions:
      - grant: "41-forage"
      - actor: >-
          That is foraging: searching the scrub for what grows there. Some
          days your hands come up empty, and that is part of it. Go north to
          Scout Tarn and type talk tarn.
  - event: command
    command: forage
    room: 5285
    conditions:
      has: ["41-track"]
      missing: ["41-forage"]
    actions:
      - grant: "41-forage"
      - actor: >-
          The sage flat rewards a patient hand, even on a day it gives up
          little. Go back to Scout Tarn on the Steppe Edge and type talk
          tarn.
```

- [ ] **Step 6: Replace `_datafiles/world/dogmud/quests/42-the_hunt.yaml`** with:

```yaml
# Quest 42, The Hunt (Pothole Coulee, tracking line, quest 2).
#
# Scout Tarn (9136) offers it (hunt_offer) once First Sign (41) is handed in,
# and hands over a salt pouch (givesItem 40017) so the player can cook. Hunter
# Delk (9137) takes the hand-in (hunt_turnin), replaces lost salt
# (salt_recovery) and then offers Break the Pack (43).
#   start  go to Hunter's Hollow and talk to Delk
#   met    topic_heard on Delk's hunt_arrive topic
#   track  track (any room)
#   hunt   kill a pronghorn (9139); the kill leaves raw meat on the corpse
#   meat   item_gain of raw meat (40014): loot the corpse, where it fell
#   rest   sleep
#   cook   craft at Delk's cooking fire (5290): grilled meat (born-known)
#   eaten  eat (the eat command event, #375 phase 7 skill coverage)
#   end    Delk's hand-in
# Sleeping carries cancel-on-action, so the craft command itself wakes the
# player and runs.

questid: 42
name: The Hunt
description: >-
  Hunter Delk works the predator fringe out of a rough camp. Scout Tarn has
  sent you to him with a salt pouch to learn the hunter's loop: bring down a
  pronghorn for its meat, rest rough in the field to keep your strength,
  cook what you killed over his fire, and eat it.
secret: false

steps:
  - id: start
    destination:
      mob: 9137
      room: 5290
    description: >-
      Scout Tarn hands you a salt pouch when you take this on and sends you
      to Hunter Delk, who hunts the fringe out of Hunter's Hollow. From the
      Steppe Edge, go south four times to the Game Trail, then west.
    hint: >-
      From the Steppe Edge, go south four times to the Game Trail, then west
      into Hunter's Hollow. Type talk delk.
  - id: met
    destination:
      room: 5289
    description: >-
      Delk has set you hunting. Pronghorn graze the Thornbrake, north of his
      camp, and the Carrion Flat, south of the Game Trail. First read the
      ground for one.
    hint: >-
      In the Thornbrake, north of Delk's camp, type track pronghorn. If it
      is dark, type cast glow first.
  - id: track
    destination:
      room: 5289
    description: >-
      You have the pronghorn's line. Now bring one down. A pronghorn does not
      start a fight, so you choose the moment. Consider it first if you want
      to size it up.
    hint: >-
      Type attack pronghorn.
  - id: hunt
    destination: anywhere  # the loot happens where the pronghorn fell
    description: >-
      The pronghorn is down and its meat is on the carcass. If you already
      took the meat from an earlier kill, bring down another pronghorn and
      loot that one.
    hint: >-
      Type loot corpse.
  - id: meat
    destination: anywhere
    description: >-
      You have the meat. Before you cook, rest rough the way a hunter does.
      Sleep where nothing hostile is near. Any harm wakes you, and so does
      your next command.
    hint: >-
      Type sleep.
  - id: rest
    destination:
      room: 5290
    description: >-
      You slept rough and woke stronger. Now cook the kill at Delk's fire in
      Hunter's Hollow. You need the raw meat and the salt pouch. No meat?
      Bring down a pronghorn and loot it. No salt? Talk to Delk for more.
    hint: >-
      At Delk's camp fire in Hunter's Hollow, type craft grilled meat. A
      craft can fail. If it does, hunt another pronghorn for meat and retry.
  - id: cook
    destination: anywhere
    description: >-
      You cooked the kill over the fire. A hot meal is for eating, not for
      carrying. Eat what you cooked.
    hint: >-
      Type eat grilled meat.
  - id: eaten
    destination:
      mob: 9137
      room: 5290
    description: >-
      You ate what you killed. Delk is at his camp in Hunter's Hollow.
    hint: >-
      Go back to Hunter Delk in Hunter's Hollow and type talk delk.
  - id: end
    description: >-
      Delk has named you a hunter, near enough. Deeper south the pack rules
      its own country, and an alpha rules the pack.

rewards:
  skillinfo: "search:1"
  itemid: 20052
  recipe_info: "hearty-stew"
  gold: 25
  actor: >-
    Delk hears you out, then grunts, which from him is praise. The
    shed-proof skin of his hands shows pale as he rubs the cold from them.
    "Killed clean, rested rough, ate what you took. That is the whole of
    it." He pushes a pair of worn field boots across the stone and a scrap
    of waxed paper after them. "Stew recipe. A hot meal does more than cold
    meat. Now the bad work. South, the pack. They run that country in
    numbers and the alpha runs them. Come back about the pack when your
    nerve is up."
  observer: "Delk grunts at the newcomer and hands across a pair of field
    boots."

triggers:
  # met: hearing Delk's hunt_arrive topic.
  - event: topic_heard
    mob: 9137
    topic: hunt_arrive
    conditions:
      has: ["42-start"]
      missing: ["42-met"]
    actions:
      - grant: "42-met"
      - actor: >-
          Delk has set you to it. In the Thornbrake, north of his camp, type
          track pronghorn.

  # track: read the ground for a pronghorn. The track command sends no
  # target noun, so any track that ran counts, in any room.
  - event: command
    command: track
    conditions:
      has: ["42-met"]
      missing: ["42-track"]
    actions:
      - grant: "42-track"
      - actor: >-
          You read the ground for a pronghorn's line. Next, type attack
          pronghorn.

  # hunt: a pronghorn (9139), after the track.
  - event: mob_death
    mob: 9139
    conditions:
      has: ["42-track"]
      missing: ["42-hunt"]
    actions:
      - grant: "42-hunt"
      - actor: >-
          The pronghorn drops. Its meat is on the carcass. Type loot corpse.

  # meat: raw meat (40014), after the kill. item_gain fires from loot, get
  # and buy alike.
  - event: item_gain
    item: 40014
    conditions:
      has: ["42-hunt"]
      missing: ["42-meat"]
    actions:
      - grant: "42-meat"
      - actor: >-
          You have the meat. Now rest rough before you cook. Type sleep.

  # rest: sleep in the field, after the meat.
  - event: command
    command: sleep
    conditions:
      has: ["42-meat"]
      missing: ["42-rest"]
    actions:
      - grant: "42-rest"
      - actor: >-
          You bed down rough and the field does its slow work. Your next
          command wakes you. At Delk's camp fire, type craft grilled meat.

  # cook: craft at the camp fire (room 5290).
  - event: command
    command: craft
    room: 5290
    conditions:
      has: ["42-rest"]
      missing: ["42-cook"]
    actions:
      - grant: "42-cook"
      - actor: >-
          The meat sears over the fire, salt and smoke and fat. Now eat it
          while it is hot. Type eat grilled meat.

  # eaten: eat the meal, after the cook. Any eat counts; the hint names the
  # grilled meat.
  - event: command
    command: eat
    conditions:
      has: ["42-cook"]
      missing: ["42-eaten"]
    actions:
      - grant: "42-eaten"
      - actor: >-
          A hot meal in the field is its own small victory. Go back to Delk
          in Hunter's Hollow and type talk delk.
```

- [ ] **Step 7: Replace `_datafiles/world/dogmud/quests/43-break_the_pack.yaml`** with:

```yaml
# Quest 43, Break the Pack (Pothole Coulee, tracking line finale).
#
# Hunter Delk (9137) offers it (pack_offer) after The Hunt (42). The player
# fights through the pack (two and three predators to a room) and kills the
# Alpha Pack-Leader (9143) in the Den (5298).
#   start  kill the alpha
#   slain  report to Delk
#   end    Delk's hand-in pack_in, which pays a Perception gain, the
#          Scab-Hide Jerkin (20088), a hunting kit, the jerkin's tailoring
#          recipe and gold

questid: 43
name: Break the Pack
description: >-
  The predator pack runs the country south of the fringe and the alpha runs
  the pack. Hunter Delk says there is no living alongside it. The pack has
  to be broken at its head. Fight through the hounds, put the alpha down,
  and tell Delk.
secret: false

steps:
  - id: start
    destination:
      room: 5298
    description: >-
      Delk has set you on the pack. Cook a meal and carry it before you go,
      because the pack comes at you in numbers, two and three to a room. From
      the Game Trail, east of Delk's camp, the way runs south through the
      Killing Ground and the Bone Field to the Den. Consider a foe to size it
      up, and flee if a fight turns against you.
    hint: >-
      In the Den, south of the Bone Field, type attack alpha. If it is dark,
      type cast glow first.
  - id: slain
    destination:
      mob: 9137
      room: 5290
    description: >-
      The alpha is dead and the pack is broken. Delk waits in Hunter's
      Hollow to hear it.
    hint: >-
      Go back to Hunter Delk in Hunter's Hollow. Type talk delk.
  - id: end
    description: >-
      The alpha is dead and the pack is broken. You came to the steppe not
      knowing a hare's track from a hound's. You leave a hunter, wearing the
      pack that tried to take you.

rewards:
  stat_info: "perception:3"
  itemid: 20088
  item_info: "30021:3,30020:2,30058:1"
  recipe_info: "scab-hide-jerkin"
  gold: 50
  actor: >-
    Delk hears you out by the fire, then reaches behind him for a bundle of
    cured hide. "From the pack that tried to take you." The jerkin fits. He
    pushes a kit of rations, bandages and a firebomb after it, and talks you
    through the cut of the hide until you could sew another at any loom. The
    long work of the steppe has settled into you as a hunter's sharpness,
    eyes and instinct quicker than they were.
  observer: ""

triggers:
  # slain: the Alpha Pack-Leader (9143) falls.
  - event: mob_death
    mob: 9143
    conditions:
      has: ["43-start"]
      missing: ["43-slain"]
    actions:
      - grant: "43-slain"
      - actor: >-
          The alpha pack-leader shudders and goes down, and the den falls
          silent. The hounds that survive scatter and do not re-form. Go
          back to Delk in Hunter's Hollow and type talk delk.
```

- [ ] **Step 8: Replace `_datafiles/world/dogmud/quests/56-running_game.yaml`** with:

```yaml
# Quest 56, Running Game (Pothole Coulee, tracking line repeatable).
#
# Hunter Delk (9137) offers it (game_loop_offer) once Break the Pack (43) is
# handed in. Repeatable with a cooldown; completion clears progress.
#   start    kill a pronghorn (9139)
#   hunt     forage on the Open Steppe (5284)
#   foraged  report to Delk
#   end      Delk's hand-in game_loop_in (#375 phase 7)

questid: 56
name: Running Game
description: >-
  Delk keeps a close eye on what moves through the fringe. He wants you to
  bring down a pronghorn, forage the Open Steppe, and tell him what the
  country gave you.
secret: false
repeatable: true
cooldown_rounds: 300

steps:
  - id: start
    destination:
      room: 5289
    description: >-
      Delk wants you to bring down a pronghorn and then forage the Open
      Steppe before you lose the feel for the country. Pronghorn graze the
      Thornbrake, north of his camp, and the Carrion Flat, south of the Game
      Trail.
    hint: >-
      In the Thornbrake, north of Delk's camp, type attack pronghorn. If it
      is dark, type cast glow first.
  - id: hunt
    destination:
      room: 5284
    description: >-
      One pronghorn down. Forage the Open Steppe, north of the Steppe Fringe.
    hint: >-
      On the Open Steppe, type forage.
  - id: foraged
    destination:
      mob: 9137
      room: 5290
    description: >-
      You took game and worked the land. Delk waits in Hunter's Hollow.
    hint: >-
      Go back to Hunter Delk in Hunter's Hollow. Type talk delk.
  - id: end
    description: >-
      You took game and worked the land. Delk will have another run waiting
      before long.

rewards:
  gold: 6
  skillinfo: "search:1"
  actor: >-
    Delk listens to the run and grunts. "Clean kill, eyes open, and the land
    worked after it." He hands over a few coins. "The fringe teaches you
    different things each pass."
  observer: ""

triggers:
  - event: mob_death
    mob: 9139
    conditions:
      has: ["56-start"]
      missing: ["56-hunt"]
    actions:
      - grant: "56-hunt"
      - actor: >-
          The pronghorn goes down clean. Now, on the Open Steppe, type
          forage.

  - event: command
    command: forage
    room: 5284
    conditions:
      has: ["56-hunt"]
      missing: ["56-foraged"]
    actions:
      - grant: "56-foraged"
      - actor: >-
          You work the steppe for what it gives. Some days that is little,
          and the run still counts. Go back to Delk and type talk delk.
```

- [ ] **Step 9: Create `_datafiles/world/dogmud/quests/93-report_to_toke__tracking_.yaml`:**

```yaml
# Quest 93, Report to Toke (Tracking) (Pothole Coulee, tracking line bridge).
#
# Hunter Delk (9137) offers it (report_tracking_offer) once Break the Pack
# (43) is handed in. Crier Toke (9106) takes the hand-in
# (report_tracking_in). Same shape as quest 90.

questid: 93
name: Report to Toke (Tracking)
description: >-
  Hunter Delk has asked you to carry the news of the pack to Crier Toke in
  Hub Square.
secret: false

steps:
  - id: start
    destination:
      mob: 9106  # follow Crier Toke (schedule pothole_toke)
      room: 5203  # fallback when Crier Toke is not spawned
    description: >-
      The alpha is dead and the pack is broken. Crier Toke, who sent you to
      Scout Tarn, waits in Hub Square for the news.
    hint: >-
      Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - id: end
    description: >-
      Toke has your news, and the steppe folk will sleep easier for it.
```

- [ ] **Step 10: The jerkin recipe.** In `_datafiles/world/dogmud/recipes/tailoring/scab-hide-jerkin.yaml` replace `skill_minimum: 8` with `skill_minimum: 1`.

- [ ] **Step 11: Replace `_datafiles/world/dogmud/dialogue/pothole_coulee/9137.yaml`** (Hunter Delk) with:

```yaml
mobid: 9137
zone: Pothole Coulee
defaultMood: neutral
opener:
  text: The name is Delk, and the fringe is where I hunt. Ask me straight and I
    will answer you straight, once.
  hints: Choose a line below to hear about hunting or your task.
  variants:
  - questExcluded:
    - 41-end
    text: You are not ready for the fringe. Go back to Tarn on the steppe and learn
      to read ground first. Out here, not seeing the sign is how you become the
      kill. Come find me when she says you can track.
    hints: Return north to Scout Tarn and finish her tracking lesson before coming
      to Hunter Delk.
  - questRequired:
    - 41-end
    questExcluded:
    - 42-start
    - 42-end
    text: Tarn's eyes, then. She sends me the ones who can read ground. If she has
      not sent you down yet, ask her about the hunt.
    hints: From Hunter's Hollow, go east to the Game Trail, then north four times to
      the Steppe Edge. Type talk tarn.
  - questRequired:
    - 42-start
    questExcluded:
    - 42-met
    - 42-end
    text: Tarn sent you. Good. Come to the fire and I will tell you how a hunter eats.
    hints: Choose Scout Tarn sent me to learn the hunt below.
  - questRequired:
    - 42-met
    questExcluded:
    - 42-track
    - 42-end
    text: Still at it. First the line. Pronghorn graze the Thornbrake, just north
      of my camp. Read the ground for one before you go at it.
    hints: In the Thornbrake, north of Delk's camp, type track pronghorn.
  - questRequired:
    - 42-track
    questExcluded:
    - 42-hunt
    - 42-end
    text: You have its line. Now bring it down. A pronghorn will not start the fight,
      so you pick the moment.
    hints: Type attack pronghorn.
  - questRequired:
    - 42-hunt
    questExcluded:
    - 42-meat
    - 42-end
    text: You made the kill. The meat stays on the carcass until you take it. Take
      it, or you killed for nothing.
    hints: Type loot corpse.
  - questRequired:
    - 42-meat
    questExcluded:
    - 42-rest
    - 42-end
    text: You have meat. Before you cook, rest. Bed down where nothing hostile is
      near and let the field mend you.
    hints: Type sleep.
  - questRequired:
    - 42-rest
    questExcluded:
    - 42-cook
    - 42-end
    text: Rested. Now the fire, here in my camp. The meat and the salt become a meal.
      Burn it and the meat is gone, so bring down another pronghorn and try again.
      Lost the salt? Ask me for salt.
    hints: At Delk's camp fire in Hunter's Hollow, type craft grilled meat.
  - questRequired:
    - 42-cook
    questExcluded:
    - 42-eaten
    - 42-end
    text: You cooked it. A hot meal is for eating, not for carrying. Eat it.
    hints: Type eat grilled meat.
  - questRequired:
    - 42-eaten
    questExcluded:
    - 42-end
    text: You have the look of someone who ate what they killed. Tell me how it went.
    hints: Choose The Hunt below to report.
  - questRequired:
    - 42-end
    questExcluded:
    - 43-start
    - 43-end
    text: You hunt well enough to feed yourself. Now the hard work. The pack, south,
      and the alpha at the head of it. When your nerve is up, I will send you after
      them.
    hints: Choose Break the Pack below when you are ready for the final task.
  - questRequired:
    - 43-start
    questExcluded:
    - 43-slain
    - 43-end
    text: It is still out there. Carry food, fight through the hounds, and take the
      alpha in its den. Do not let them get behind you. From the Game Trail, go south
      through the Killing Ground and the Bone Field.
    hints: In the Den, south of the Bone Field, type attack alpha.
  - questRequired:
    - 43-slain
    questExcluded:
    - 43-end
    text: You came back from the den. I can see it on you. Tell me.
    hints: Choose Break the Pack below to report.
  - questRequired:
    - 56-start
    questExcluded:
    - 56-hunt
    - 56-end
    text: The run is on. Bring a pronghorn down first. The Thornbrake, just north
      of my camp, usually has one.
    hints: In the Thornbrake, north of Delk's camp, type attack pronghorn.
  - questRequired:
    - 56-hunt
    questExcluded:
    - 56-foraged
    - 56-end
    text: One down. Now work the Open Steppe for what it gives. Then come back and
      tell me.
    hints: On the Open Steppe, type forage.
  - questRequired:
    - 56-foraged
    questExcluded:
    - 56-end
    text: Back from the run. Tell me what the fringe gave you.
    hints: Choose Running Game below to report.
  - questRequired:
    - 43-end
    questExcluded:
    - 93-start
    - 93-end
    text: You broke the pack. I have hunted this fringe for years and never managed
      that. Toke sent you to Tarn, and Tarn to me. He will want this news from you.
    hints: Choose Report to Toke (Tracking) below to carry the news to Toke.
  - questRequired:
    - 93-start
    questExcluded:
    - 93-end
    text: Toke is waiting on that news in Hub Square. The fringe will keep.
    hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk
      toke.
  - questRequired:
    - 43-end
    text: The country south is quieter now, and you walk it wearing the hide of the
      thing that ruled it. Go on. You have the steppe in you.
    hints: You have finished the wilderness trail. When you want more hunting, choose
      Running Game below.
topics:
- key: hunt_arrive
  kind: info
  label: Scout Tarn sent me to learn the hunt.
  text: Right. Hunting is patience and one clean kill. Keep that salt Tarn gave you.
    You will want it for the fire. The loop is this. First, take meat. Track a pronghorn
    in the Thornbrake, just north of here, bring it down, and take the meat off the
    carcass. Second, rest rough. Sleep where it is safe and the field mends you faster
    than walking does. Third, cook it. At my fire here, the meat and the salt become
    grilled meat. Last, eat what you cooked. Do all of it and come back.
  hints: In the Thornbrake, north of Delk's camp, type track pronghorn.
  questRequired:
  - 42-start
  questExcluded:
  - 42-met
  - 42-end
# The replacement and the reminder never show together: itemExcluded and
# carriesItem on the same item are exact opposites.
- key: salt_recovery
  kind: info
  label: I've lost my salt.
  text: Lost your salt? Here. Last pinch I am sparing. You need it and raw meat to
    cook grilled meat at the fire. Do not lose this one.
  hints: You received a salt-pouch. With raw meat, type craft grilled meat at the
    camp fire.
  questRequired:
  - 42-start
  questExcluded:
  - 42-cook
  itemExcluded: 40017
  givesItem: 40017
- key: salt_have
  kind: info
  label: Do I need more salt?
  text: You already have salt. Check your pack and your pouch. One pinch is all a
    cut of meat needs.
  hints: Type inventory to see what you carry.
  questRequired:
  - 42-start
  questExcluded:
  - 42-cook
  carriesItem: 40017
- key: hunt_turnin
  kind: handin
  quest: 42
  label: The Hunt
  playerLine: I made a kill, rested, cooked the meat and ate it.
  text: Killed clean, rested rough, ate what you took. That is the whole of being
    a hunter, and you did it without me holding your hand. Take these boots (broken
    in, good for long ground) and the stew recipe. A hot meal does more than fill
    you. Now the bad work, when you are ready. The pack. I will send you after it.
  hints: Quest complete. Choose Break the Pack below when you are ready for the final
    task.
  questRequired:
  - 42-eaten
  questExcluded:
  - 42-end
  grantsQuest: 42-end
- key: pack_offer
  kind: offer
  quest: 43
  label: Break the Pack
  playerLine: Do you want the pack broken?
  text: The pack, then. South of the fringe is their country, and they do not share
    it. They will come at you two and three at a time. That is how a pack kills,
    and it is how this one will try to kill you. Cook a meal and carry it. Fight through
    the hounds, keep your back clear, and do not stop until you reach the den. From
    the Game Trail, east of my camp, it is south all the way, through the Killing
    Ground and the Bone Field. The alpha is in the den. Put it down, then come back
    and tell me. Go.
  hints: In the Den, south of the Bone Field, type attack alpha.
  breadcrumb: Learn the hunt first. The pack to the south is the hard work, and it
    will wait for you.
  questRequired:
  - 42-end
  questExcluded:
  - 43-start
  - 43-end
  grantsQuest: 43-start
- key: pack_in
  kind: handin
  quest: 43
  label: Break the Pack
  playerLine: The alpha is dead, and the pack has scattered.
  text: Scattered. I heard the howling stop and did not dare believe it. You broke
    them at the head, the way it had to be done. Here. A jerkin cut from their own
    hides, a kit to keep you in the field, and the cut of the hide so you can sew more.
    Stitcher Bex has a loom in the Patch-Tent in town, south of Coulee Provisions,
    and it will take that hide.
  hints: Quest complete. Crier Toke will want the news. Choose Report to Toke (Tracking)
    below.
  questRequired:
  - 43-slain
  questExcluded:
  - 43-end
  grantsQuest: 43-end
  moodChange: friendly
- key: hunt_progress_start
  kind: info
  label: How do I start the hunt?
  text: 'Not yet. First the line: read the ground for a pronghorn. The Thornbrake,
    just north of my camp, is where they graze.'
  hints: In the Thornbrake, north of Delk's camp, type track pronghorn.
  questRequired:
  - 42-met
  questExcluded:
  - 42-track
  - 42-end
- key: hunt_progress_track
  kind: info
  label: I have its line. What now?
  text: Not yet. You have its line. Now bring it down.
  hints: Type attack pronghorn.
  questRequired:
  - 42-track
  questExcluded:
  - 42-hunt
  - 42-end
- key: hunt_progress_hunt
  kind: info
  label: The pronghorn is down. What next?
  text: Not yet. The meat is still on the carcass. Take it. If you took it from an
    earlier kill, bring down another and take that one.
  hints: Type loot corpse.
  questRequired:
  - 42-hunt
  questExcluded:
  - 42-meat
  - 42-end
- key: hunt_progress_meat
  kind: info
  label: I have the meat. Can I cook it now?
  text: Not yet. You have meat. Rest before you cook it.
  hints: Type sleep.
  questRequired:
  - 42-meat
  questExcluded:
  - 42-rest
  - 42-end
- key: hunt_progress_rest
  kind: info
  label: I've rested. What now?
  text: 'Not yet. You rested. Now cook the kill at my fire. If it burns, the meat
    is lost: bring down another pronghorn and cook that one.'
  hints: At Delk's camp fire in Hunter's Hollow, type craft grilled meat.
  questRequired:
  - 42-rest
  questExcluded:
  - 42-cook
  - 42-end
- key: hunt_progress_cook
  kind: info
  label: I cooked the meat. What now?
  text: Not yet. You cooked it. Now eat it. A hot meal does more than cold meat.
  hints: Type eat grilled meat.
  questRequired:
  - 42-cook
  questExcluded:
  - 42-eaten
  - 42-end
- key: hunt_prey
  kind: info
  label: Where do pronghorn graze?
  text: 'Pronghorn graze the Thornbrake, just north of my camp, and the Carrion Flat,
    south of the Game Trail. Track one to read its line: type track and the beast''s
    name. Then bring it down. It will not start the fight, so you choose the moment.'
  hints: In the Thornbrake, north of Delk's camp, type track pronghorn.
  questRequired:
  - 42-met
  questExcluded:
  - 42-end
- key: hunt_meat
  kind: info
  label: How do I take the meat?
  text: A pronghorn always leaves meat on the carcass, and it stays there until you
    take it. Loot the corpse where it fell. Raw meat and salt, at my fire, make grilled
    meat.
  hints: Where the pronghorn fell, type loot corpse.
  questRequired:
  - 42-met
  questExcluded:
  - 42-end
- key: hunt_help
  kind: info
  label: How do I rest and cook in the field?
  text: Three field skills. Rest. Type sleep where nothing hostile is near and your
    body mends fast. The next thing you do wakes you, and any harm wakes you the hard
    way. Cooking. Stand at my fire and type craft followed by the meal, so craft grilled
    meat with raw meat and salt. Eating. A meal in your pack feeds nobody. Type eat
    grilled meat when it is cooked. A rested, fed hunter outlasts a stronger one who
    is neither.
  hints: With raw meat and salt in your pack, stand at a cooking fire and type craft
    grilled meat. For more, type help craft.
  marked: true
- key: pack_progress
  kind: info
  label: Where is the alpha's den?
  text: The alpha is in the den. From the Game Trail, east of my camp, go south through
    the Killing Ground and the Bone Field. Carry a cooked meal, and if a fight turns
    against you, flee and come back stronger.
  hints: In the Den, south of the Bone Field, type attack alpha.
  questRequired:
  - 43-start
  questExcluded:
  - 43-slain
  - 43-end
# The bridge comes before the repeatable, so the menu lists it first.
- key: report_tracking_offer
  kind: offer
  quest: 93
  label: Report to Toke (Tracking)
  playerLine: Should I take the news of the pack to Toke?
  text: Do that. Toke sent you to Tarn, and Tarn sent you to me. He keeps to Hub
    Square, south of the Awakening Pool. Tell him the pack is broken.
  hints: Crier Toke stands in Hub Square, south of the Awakening Pool. Type talk toke.
  questRequired:
  - 43-end
  questExcluded:
  - 93-start
  - 93-end
  grantsQuest: 93-start
- key: game_loop_offer
  kind: offer
  quest: 56
  label: Running Game
  playerLine: Do you have more hunting work for me?
  text: The fringe is always running game. Go out and bring one down, then forage
    the Open Steppe. Keep your hand in. Then come back and tell me what it gave you.
  hints: In the Thornbrake, north of Delk's camp, type attack pronghorn.
  questRequired:
  - 43-end
  questExcluded:
  - 56-start
  - 56-hunt
  - 56-end
  grantsQuest: 56-start
- key: game_progress_start
  kind: info
  label: Where should I start the run?
  text: The run is on. Bring a pronghorn down first. The Thornbrake, just north of
    my camp, usually has one.
  hints: In the Thornbrake, north of Delk's camp, type attack pronghorn.
  questRequired:
  - 56-start
  questExcluded:
  - 56-hunt
  - 56-end
- key: game_progress_hunt
  kind: info
  label: One beast is down. What now?
  text: One down. Now work the Open Steppe for what it gives. Then come back to me.
  hints: On the Open Steppe, type forage.
  questRequired:
  - 56-hunt
  questExcluded:
  - 56-foraged
  - 56-end
- key: game_loop_in
  kind: handin
  quest: 56
  label: Running Game
  playerLine: I took a pronghorn and worked the steppe.
  text: Clean kill, eyes open, and the land worked after it. The fringe teaches you
    different things each pass. Here, for the run.
  hints: Quest complete. Delk will have another run waiting after a while.
  questRequired:
  - 56-foraged
  questExcluded:
  - 56-end
  grantsQuest: 56-end
  moodChange: friendly
- key: hands
  kind: info
  label: What happened to your hands?
  text: The Awakening toughened my hands, skin like waxed bark now, turns cold and
    thorn and wet. Out here it is a gift. I do not dwell on it.
```

- [ ] **Step 12: Edit Tarn (`_datafiles/world/dogmud/dialogue/pothole_coulee/9136.yaml`).** Four Edit calls. Do not touch the variant gated `questRequired: [30-end]` / `questExcluded: [41-start, 41-end]` (plan 2's). If an `old_string` no longer matches, stop and report.

Edit 12a. Replace:

```yaml
  - questRequired:
    - 41-start
    questExcluded:
    - 41-track
    - 41-end
```

with:

```yaml
  - questRequired:
    - 41-start
    questExcluded:
    - 41-met
    - 41-end
    text: Toke sent you out to me? Good. Ask me why you came, and I will tell you
      what the ground is saying.
    hints: Choose Crier Toke sent me to learn the ground below.
  - questRequired:
    - 41-met
    questExcluded:
    - 41-track
    - 41-end
```

(The following `text:` and `hints:` lines stay and now belong to the `41-met` variant.)

Edit 12b. Replace:

```yaml
  - questRequired:
    - 41-end
    text: You can read ground now. That will keep you alive longer than a sharp knife
      will. South, then, to the fringe, where the predators range. Hunter Delk keeps
      a camp out there. Tell him Tarn sent you.
    hints: Head south into the predator fringe and type talk delk to find Hunter Delk.
topics:
```

with:

```yaml
  - questRequired:
    - 41-end
    questExcluded:
    - 42-start
    - 42-end
    text: You can read ground now. That will keep you alive longer than a sharp knife
      will. South, in the fringe, Hunter Delk keeps a camp. He will teach you the rest
      if I send you. Say the word.
    hints: Choose The Hunt below to hear what Delk teaches.
  - questRequired:
    - 42-start
    questExcluded:
    - 42-met
    - 42-end
    text: Delk is waiting in Hunter's Hollow. South four times to the Game Trail, then
      west.
    hints: From the Steppe Edge, go south four times to the Game Trail, then west into
      Hunter's Hollow. Type talk delk.
  - questRequired:
    - 41-end
    text: You can read ground now. The fringe is Delk's country. Keep your eyes on
      the line.
    hints: When you want more of the wild, Hunter Delk in Hunter's Hollow may have
      work.
topics:
```

Edit 12c. Replace the whole `sign_turnin` topic (from `- key: sign_turnin` through its `grantsQuest: 41-end` line) with:

```yaml
- key: sign_turnin
  kind: handin
  quest: 41
  label: First Sign
  playerLine: I tracked the hare and worked the scrub.
  text: You followed the line and you worked the scrub. That is the whole foundation,
    and everything else out here builds on reading ground. Take this cloak. The wind
    has teeth after dark. When you want to learn the hunt, ask me, and I will send
    you to Delk in the fringe.
  hints: Quest complete. Choose The Hunt below to hear what Delk teaches.
  questRequired:
  - 41-forage
  questExcluded:
  - 41-end
  grantsQuest: 41-end
- key: sign_arrive
  kind: info
  label: Crier Toke sent me to learn the ground.
  text: Right. Here is the first of it. Out on the Open Steppe, just south of here,
    a hare has left a trail. Read the ground for it. That is tracking. You read what
    passed and where it went. When you have, I will show you what the scrub can feed
    you.
  hints: Go south to the Open Steppe and type track hare.
  questRequired:
  - 41-start
  questExcluded:
  - 41-met
  - 41-end
- key: hunt_offer
  kind: offer
  quest: 42
  label: The Hunt
  playerLine: Do you want me to learn the hunt from Delk?
  text: Then go south to him. Delk hunts the fringe out of a rough camp in Hunter's
    Hollow. Take this salt with you. He will want you cooking what you kill. From
    here go south four times to the Game Trail, then west into the hollow. Tell him
    Tarn says you can read ground.
  hints: Accept, and Tarn hands you a salt pouch. From the Steppe Edge, go south four
    times to the Game Trail, then west. Type talk delk.
  questRequired:
  - 41-end
  questExcluded:
  - 42-start
  - 42-end
  grantsQuest: 42-start
  givesItem: 40017
```

Edit 12d. In `sign_progress_start` replace:

```yaml
  questRequired:
  - 41-start
  questExcluded:
  - 41-track
  - 41-end
- key: sign_progress_track
```

with:

```yaml
  questRequired:
  - 41-met
  questExcluded:
  - 41-track
  - 41-end
- key: sign_progress_track
```

- [ ] **Step 13: Toke's tracking hand-in.** In `9106.yaml`, directly after the `report_brewing_in` topic, insert:

```yaml
- key: report_tracking_in
  kind: handin
  quest: 93
  label: Report to Toke (Tracking)
  playerLine: Delk sends word. The pack is broken.
  text: Broken? The alpha too? Hoy! The steppe folk will sleep with their doors open
    tonight, and I will tell them whose doing it was. The wilderness trail is yours,
    and any mouth you have not walked yet is still open to you.
  hints: Quest complete. Toke can still send you down any trail you have not walked.
  questRequired:
  - 93-start
  questExcluded:
  - 93-end
  grantsQuest: 93-end
  moodChange: friendly
```

- [ ] **Step 13b: Toke's First Sign offer sends the player to Tarn** (route checked: Hub Square 5203 west 5216, west 5223, south 5282, south 5283). In `9106.yaml`, inside `send_steppe`, replace:

```yaml
  text: Scout Tarn, out at the Steppe Edge. She reads ground the way Croup reads
    a ledger. From this square go west twice, along the stilt-house walk and into
    the scrub draw, then south twice through the scrub mouth. A hare has crossed
    the Open Steppe just past her. Find its trail.
  hints: 'From Hub Square: west, west, south, south to the Steppe Edge, then south
    once more. On the Open Steppe, type track hare.'
```

with:

```yaml
  text: Scout Tarn, out at the Steppe Edge. She reads ground the way Croup reads
    a ledger. From this square go west twice, along the stilt-house walk and into
    the scrub draw, then south twice through the scrub mouth. She will show you your
    first sign.
  hints: From Hub Square, go west twice, then south twice to the Steppe Edge. Type
    talk tarn.
```

Add to `TestTokeQuestOneOffers_SendToTheGiver` in `coulee_bridges_test.go`:

```go
	t.Run("Tracking", func(t *testing.T) { checkTokeSends(t, toke, "send_steppe", 41, "tarn") })
```

- [ ] **Step 14: Manifests.** In `quests/step-ids.txt` insert `41-met` (between `41-forage` and `41-start`), `42-eaten` (between `42-cook` and `42-end`), `42-met` (between `42-meat` and `42-rest`), `43-slain` (between `43-end` and `43-start`), `56-foraged` (between `56-end` and `56-hunt`), and `93-end`, `93-start` (after `92-start`). In `dialogue/topic-keys.txt` add `9106.report_tracking_in`, `9136.hunt_offer`, `9136.sign_arrive`, `9137.game_loop_in`, `9137.hunt_arrive`, `9137.hunt_progress_cook`, `9137.pack_in`, `9137.report_tracking_offer` in sorted position, delete `9137.hunt_offer`, and add under `# renamed`:

```text
9137.hunt_offer -> 9136.hunt_offer
```

- [ ] **Step 15: Delete the tracking backlog lines** from `quests/loop-backlog.txt`:

```text
chain 41 mob 9137 topic hunt_offer
chain 43 mob 9137 topic game_loop_offer
end 43 quest 43 trigger mob_death mob=9143 grants=43-end
end 43 step end
end 56 quest 56 trigger command room=5284 command=forage grants=56-end
end 56 step end
walk-destination 42 step hunt
```

Check (each on its own line, expect no output):

```bash
grep -nE '^\S+ (41|42|43|56|93) ' _datafiles/world/dogmud/quests/loop-backlog.txt
grep -nE '^\S+ (3[2-9]|4[0-3]|5[3-6]) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

- [ ] **Step 16: Run and pass; prove a test can fail.**

```bash
gofmt -l internal/
go test -count=1 ./internal/questengine/ ./internal/guards/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: no gofmt output; all `ok`; smoke tests `--- PASS`, none `--- SKIP`. Then change the eat trigger's `has: ["42-cook"]` to `has: ["42-rest"]`, rerun `go test -count=1 ./internal/questengine/ -run TestQuest42TheHunt`, confirm a failure in "walk in order", and restore.

- [ ] **Step 17: Narration golden.**

```bash
go test -count=1 ./internal/narration/ -run TestSnapshotStores -update
git diff --stat internal/narration/testdata/stores/
git diff internal/narration/testdata/stores/quests.golden | grep -E '^[+-]quest\|' | cut -c1-60
go test -count=1 ./internal/narration/
```

Expected: only `quests.golden`, only lines `quest|41|`, `quest|42|`, `quest|43|`, `quest|56|`, `quest|93|`.

- [ ] **Step 18: Commit.**

```bash
git add internal/questengine/newbie_wilderness_steps_test.go internal/questengine/newbie_one_command_steps_test.go internal/questengine/coulee_bridges_test.go _datafiles/world/dogmud/quests/41-first_sign.yaml _datafiles/world/dogmud/quests/42-the_hunt.yaml _datafiles/world/dogmud/quests/43-break_the_pack.yaml _datafiles/world/dogmud/quests/56-running_game.yaml _datafiles/world/dogmud/quests/93-report_to_toke__tracking_.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9137.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9136.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml _datafiles/world/dogmud/recipes/tailoring/scab-hide-jerkin.yaml _datafiles/world/dogmud/quests/step-ids.txt _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/quests/loop-backlog.txt internal/narration/testdata/stores/quests.golden
git status --short
git commit -F - <<'EOF'
feat(quests): tracking line runs Toke, Tarn, Delk, Toke (#375 phase 7)

First Sign (41) starts with a talk to Tarn. Tarn offers The Hunt (42)
with the salt pouch; it starts with a talk to Delk and gains an eat
step after the cook. Break the Pack (43) and Running Game (56) end at
Delk's new hand-ins; Delk offers Report to Toke (Tracking) (93). The
scab-hide jerkin recipe drops to skill_minimum 1. Clears 7
loop-backlog lines.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 7: Onna's trading topics, the crafters' station topics, Enchanter Daff

Suggested executor: Sonnet (all text given).

**Files:**
- Modify: `internal/questengine/coulee_skill_topics_test.go`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9104.yaml`, `9605.yaml`, `9606.yaml`, `9607.yaml`, `9608.yaml`
- Rename: `_datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_skell.yaml` to `9606-enchanter_daff.yaml`
- Modify: `_datafiles/world/dogmud/mobs/pothole_coulee/9607-jeweler_vurl.yaml`, `_datafiles/world/dogmud/schedules/pothole_coulee/pothole_coulee_enchanter.yaml`, `_datafiles/world/dogmud/dialogue/topic-keys.txt`, `_datafiles/world/dogmud/templates/help/enchanting.template`

The name "Daff" was checked against every mob, dialogue, conversation, quest and room file and `docs/world.md` (no match, including as a substring). The novel manuscript named in `dogmud-authoring-content` is not in the repo; the controller should ask the owner if it must be checked too.

- [ ] **Step 1: Write the failing tests.** Append to `internal/questengine/coulee_skill_topics_test.go`:

```go

func TestOnna9104_TeachesTrading(t *testing.T) {
	cases := map[string]string{
		"appraise_help": "type appraise",
		"offer_help":    "type offer",
		"identify_help": "type cast identify",
		"barter_help":   "help bartering",
	}
	for key, want := range cases {
		_, hints := openInfo(t, "9104.yaml", key)
		require.Contains(t, hints, want, "Onna's %s hint names %q", key, want)
	}
	text, _ := openInfo(t, "9104.yaml", "identify_help")
	require.Contains(t, text, "no coin", "the identify spell is free of gold")
}

func TestCrafters_NameAStationRecipe(t *testing.T) {
	cases := []struct{ file, recipe string }{
		{"9605.yaml", "linen tunic"},
		{"9605.yaml", "scab-hide jerkin"},
		{"9606.yaml", "honed-edge"},
		{"9607.yaml", "copper ring"},
		{"9608.yaml", "trail rations"},
	}
	for _, c := range cases {
		text, hints := openInfo(t, c.file, "station_help")
		require.Contains(t, text+" "+hints, c.recipe, "%s names %s", c.file, c.recipe)
		require.Contains(t, hints, "type craft", "%s hint names the craft command", c.file)
	}
	text, _ := openInfo(t, "9606.yaml", "station_help")
	require.Contains(t, text, "disenchant")
	require.Contains(t, text, "in your pack", "disenchant reads the backpack (skill.disenchant.go)")

	// The help page agrees with the code and with Daff: disenchant takes an
	// item from the backpack, not an equipped one.
	help := string(repoFile(t, "_datafiles", "world", "dogmud", "templates", "help", "enchanting.template"))
	require.NotContains(t, help, "from an equipped item")
	require.Contains(t, help, "Remove an enchantment from an item in your pack.")
}

// Enchanter Skell (9606) is renamed so Bowmaster Skell (9169), quest 52's
// boss, keeps the name (spec "Skill coverage" ruling).
func TestEnchanter9606_IsDaff(t *testing.T) {
	mob := string(repoFile(t, "_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9606-enchanter_daff.yaml"))
	require.Contains(t, mob, "  name: Enchanter Daff\n")
	for _, rel := range [][]string{
		{"_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9606.yaml"},
		{"_datafiles", "world", "dogmud", "schedules", "pothole_coulee", "pothole_coulee_enchanter.yaml"},
		{"_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9607-jeweler_vurl.yaml"},
	} {
		require.NotContains(t, string(repoFile(t, rel...)), "Skell", "%v still names Skell", rel)
	}
}
```

(The one-line struct `struct{ file, recipe string }` is how gofmt writes a one-group struct on one line.)

- [ ] **Step 2: Run and see them fail.**

```bash
go test -count=1 ./internal/questengine/ -run 'TestOnna9104|TestCrafters|TestEnchanter9606'
```

Expected: `FAIL` (`9104.yaml has no topic "appraise_help"`, `no topic "station_help"`, `9606-enchanter_daff.yaml` unreadable).

- [ ] **Step 3: Onna's topics.** In `9104.yaml`, after the last topic (`remembers`, whose last line is `  hints: Choose a line below to learn about her wares or the sling.`), append:

```yaml
- key: appraise_help
  kind: info
  label: Can you tell me what an item really is?
  text: I can, for a small fee. Stand by me, or any shopkeeper, and type appraise
    and the item's name, and I will tell you what it does and what it is worth. Small
    trinkets you find on the road I look over for free. It is the least I can do for
    the chance to buy them.
  hints: A merchant reads an item for a small fee. Type appraise and the item's name.
    For more, type help appraise.
  marked: true
- key: offer_help
  kind: info
  label: How do I know what you would pay?
  text: Ask before you sell. Type offer and the item's name, and I will tell you what
    I would give for it. You keep it until you say sell. A wise seller asks two shops
    before he picks one.
  hints: Offer asks a merchant's price without selling. Type offer and the item's name.
    For more, type help offer.
  marked: true
- key: identify_help
  kind: info
  label: Can I learn what an item does myself?
  text: You already can. Everyone who wakes at the pool knows the identify spell.
    Cast it on something you carry or wear and it tells you what the thing does. It
    costs you a little of your conviction and no coin at all. Save my fee for the
    things you cannot read.
  hints: Identify is a spell you already know. Type cast identify and the item's name.
    For more, type help identify.
  marked: true
- key: barter_help
  kind: info
  label: Why do your prices change?
  text: Because my shelves change. Plenty of a thing and I sell it cheap. The last
    one in the coulee costs what it costs. The other half is you. A trader with a
    good tongue pays less and is paid more, and the knack grows every time you buy,
    sell or appraise. Charm counts for more than muscle at a counter.
  hints: Bartering makes buying cheaper and selling dearer, and trading trains it.
    For more, type help bartering.
```

- [ ] **Step 4: Crafter station topics.** Append each block to the end of its file.

`9605.yaml` (Stitcher Bex, after the `backpack` topic):

```yaml
- key: station_help
  kind: info
  label: What can I make at your loom?
  text: The loom is where cloth and hide become something you can wear, and anyone
    may use it. Everyone knows the linen tunic and the cloth pants. Each takes two
    cloth strips and a thread spool. I sell the thread. Cloth strip I do not stock,
    it comes up the road from the city drapers. Hunter Delk teaches the scab-hide jerkin
    to whoever breaks the pack south of his camp, and it is made right here at this
    loom, from two pack-hide pelts off the hounds, a thread spool and a bone needle.
    The spool and the needle you can buy from me.
  hints: The loom is a crafting station. Type craft list beside it to see what you
    can make. For more, type help tailoring.
  marked: true
```

`9606.yaml` (the enchanter). First change the `ward` topic's hint line `  hints: To see what Skell sells and what it costs, type list. For more, type help` to `  hints: To see what Daff sells and what it costs, type list. For more, type help` (the next line, `    buy.`, stays). Then append:

```yaml
- key: station_help
  kind: info
  label: What can I do at your circle?
  text: The ring in this floor is an enchanting circle. Stand inside it to work. The
    first working anyone can manage is honed-edge. One binding paste, which I sell,
    and one healer's root, from Birna or the cook. Hold the weapon you mean to sharpen
    and work the paste into it, and the edge cuts cleaner. If you ever want an enchantment
    out again, bring the item here in your pack, not your hand, and disenchant it.
    Mind that it pulls at you. The weakness passes, in time.
  hints: The circle is an enchanting station. Type craft honed-edge while you hold
    a weapon. For more, type help enchanting.
  marked: true
```

`9607.yaml` (Jeweler Vurl, after the `gem` topic):

```yaml
- key: station_help
  kind: info
  label: What can I make at your bench?
  text: This is a jeweler's bench, and you may work it yourself. Start with a copper
    ring. One copper wire and one pinch of gem dust, and I sell both. Stand here and
    twist the wire round. It will not be pretty the first time. Nothing is.
  hints: The bench is a jewelcrafting station. Type craft copper ring beside it. For
    more, type help jewelcrafting.
  marked: true
```

`9608.yaml` (Cook Hesk, after the `stew` topic):

```yaml
- key: station_help
  kind: info
  label: Can I cook at your fire?
  text: Pull up a stone. This is a cooking fire, and anyone can use it. The easiest
    thing is trail rations. One raw meat and a salt pouch, and I sell you both. Dry
    the meat over the coals and it keeps for the road. Grilled meat takes the same
    two and wants eating hot. When it is done, eat it, or carry it.
  hints: The fire is a cooking station. Type craft trail rations beside it. For more,
    type help cooking.
  marked: true
```

- [ ] **Step 4b: Fix the enchanting help line.** Check the code first:

```bash
sed -n 26,41p internal/usercommands/skill.disenchant.go
```

Expected: the `enchanting_circle` station check, then `items.FindMatchIn(rest, user.Character.Items...)` (the backpack; equipment is not searched). Then in `_datafiles/world/dogmud/templates/help/enchanting.template` replace the line:

```text
  <ansi fg="command">disenchant <item></ansi>   Remove an enchantment from an equipped item.
```

with:

```text
  <ansi fg="command">disenchant <item></ansi>   Remove an enchantment from an item in your pack.
```

(Leave the line's other bytes, including the alignment spaces, exactly as they are.)

- [ ] **Step 5: Rename Enchanter Skell to Enchanter Daff.**

```bash
git mv _datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_skell.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_daff.yaml
```

Then with Edit:
- In `9606-enchanter_daff.yaml` replace `  name: Enchanter Skell` with `  name: Enchanter Daff`, and the shop comment `    # Thornwall chrysalis (in-shop crafted) — Skell's specialty` with `    # Thornwall chrysalis (in-shop crafted), Daff's specialty`.
- In `9607-jeweler_vurl.yaml` replace `    # Thornwall chrysalis (in-shop / Skell overlap)` with `    # Thornwall chrysalis (in-shop, Daff overlap)`.
- In `schedules/pothole_coulee/pothole_coulee_enchanter.yaml` replace `description: "Enchanter Skell's daily routine: the wards by day, the dugout floor by night."` with `description: "Enchanter Daff's daily routine: the wards by day, the dugout floor by night."`.

Verify nothing live still names the enchanter Skell (run on its own; the only hits allowed are Bowmaster Skell 9169, Porter Skell 9372, quests 51 and 52, dialogue 9162, the newbie ranged test, the quests golden, completed docs and the phase 7 spec):

```bash
grep -rln "Skell" _datafiles internal modules *.go | grep -v economy/snapshots
```

The goals file `goals/9606-enchanter_skell.yaml` is gitignored living state (`.gitignore:44`); the game seeds `9606-enchanter_daff.yaml` on its own. Leave the old local file alone.

- [ ] **Step 6: Topic keys.** Add to `dialogue/topic-keys.txt` in sorted position: `9104.appraise_help`, `9104.barter_help`, `9104.identify_help`, `9104.offer_help`, `9605.station_help`, `9606.station_help`, `9607.station_help`, `9608.station_help`.

- [ ] **Step 7: Run and pass.**

```bash
gofmt -l internal/
go test -count=1 ./internal/questengine/ -run 'TestOnna9104|TestCrafters|TestEnchanter9606|TestVorn9108' 
go test -count=1 ./internal/guards/... ./internal/mobs/...
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: all `ok`, smoke tests `--- PASS`, none `--- SKIP`. The boot smoke test loads every mob, so a file name that does not match `Enchanter Daff` would fail it.

- [ ] **Step 8: Commit.**

```bash
git add internal/questengine/coulee_skill_topics_test.go _datafiles/world/dogmud/dialogue/pothole_coulee/9104.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9605.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9606.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9607.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/9608.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_daff.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9606-enchanter_skell.yaml _datafiles/world/dogmud/mobs/pothole_coulee/9607-jeweler_vurl.yaml _datafiles/world/dogmud/schedules/pothole_coulee/pothole_coulee_enchanter.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt _datafiles/world/dogmud/templates/help/enchanting.template
git status --short
git commit -F - <<'EOF'
feat(dialogue): Onna teaches trading, crafters teach their stations

Skill coverage for #375 phase 7: Trader Onna explains appraise, offer,
the free identify spell and bartering; Stitcher Bex, Jeweler Vurl, Cook
Hesk and the enchanter each name a recipe a fresh player can make at
their station, and the enchanter covers disenchant. Enchanter Skell is
renamed Enchanter Daff so Bowmaster Skell keeps the name. The
enchanting help page now says disenchant takes an item from the pack,
as the code does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 8: Dashes in the two conversation pairs

Suggested executor: Sonnet.

**Files:** Modify `_datafiles/world/dogmud/conversations/pairs/9103_9105.yaml`, `_datafiles/world/dogmud/conversations/pairs/9106_9107.yaml`. (`9101_9106.yaml` has no dash; the dialogue files of these NPCs had none; the quest files lost theirs in Tasks 2, 4, 5 and 6.)

- [ ] **Step 1: Confirm the two lines.**

```bash
grep -nP '(^|\s)--(\s|$)|\x{2014}|\x{2013}' _datafiles/world/dogmud/conversations/pairs/9101_9106.yaml _datafiles/world/dogmud/conversations/pairs/9103_9105.yaml _datafiles/world/dogmud/conversations/pairs/9106_9107.yaml
```

Expected: exactly `9103_9105.yaml:21` and `9106_9107.yaml:16`.

- [ ] **Step 2: Rewrite them.** In `9103_9105.yaml` replace

```yaml
        text: "The young ones passing through -- they spend before they think."
```

with

```yaml
        text: "The young ones passing through spend before they think."
```

In `9106_9107.yaml` replace

```yaml
        text: "That one with the long coat -- you recognize the face?"
```

with

```yaml
        text: "That one with the long coat. Do you know the face?"
```

- [ ] **Step 3: Check the whole scope is dash-free.** Run on its own (expect no output):

```bash
grep -nP '(^|\s)--(\s|$)|\x{2014}|\x{2013}' _datafiles/world/dogmud/quests/3[2-9]-*.yaml _datafiles/world/dogmud/quests/4[0-3]-*.yaml _datafiles/world/dogmud/quests/5[3-6]-*.yaml _datafiles/world/dogmud/quests/9[0-3]-*.yaml _datafiles/world/dogmud/dialogue/pothole_coulee/{9104,9106,9108,9112,9116,9117,9128,9129,9136,9137,9605,9606,9607,9608}.yaml _datafiles/world/dogmud/conversations/pairs/9101_9106.yaml _datafiles/world/dogmud/conversations/pairs/9103_9105.yaml _datafiles/world/dogmud/conversations/pairs/9106_9107.yaml
```

If `9106.yaml` shows a hit, it comes from a plan 3b or 4 topic, not this plan: report it, do not edit it.

- [ ] **Step 4: Run and commit.**

```bash
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_ServerBootsCleanWithRealData' . 2>&1 | grep -E '^(--- |ok|FAIL)'
git add _datafiles/world/dogmud/conversations/pairs/9103_9105.yaml _datafiles/world/dogmud/conversations/pairs/9106_9107.yaml
git commit -F - <<'EOF'
chore(conversations): drop the dashes from two Coulee pairs (#690)

Croup and Wicker, Toke and Esk: one line each rewritten without a dash
(#375 phase 7).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: `--- PASS: TestSmoke_ServerBootsCleanWithRealData`.

---

## Task 9: #697 kit salve record, patch notes, docs index

Suggested executor: Sonnet.

**Files:** Modify `docs/PATCH_NOTES.md`, `docs/README.md`.

- [ ] **Step 1: Re-verify the kit salve finding on the merged code.**

```bash
grep -rn "CraftedRound > 0" internal/actions/ | grep -v _test
grep -n "itemid" tools/playtest/profiles/early.yaml
grep -rn "CraftedRound = " internal --include=*.go | grep -v _test
```

Expected: the potion path (plan 1's `quaff`, or `drink.go`) gates its age line on `CraftedRound > 0`; the early kit lists item 30036 with no `crafted_round`; the only `CraftedRound =` writers are the craft tick, the alchemy migration (old item ids only) and the pinnacle tick. If the gate is gone, stop and report: the finding no longer holds.

- [ ] **Step 2: Patch notes.** Insert at the top of `docs/PATCH_NOTES.md`, directly under `# DOGMud Patch Notes` and its blank line, with today's date from `date +%F`:

```markdown
## <today's date>: Pothole Coulee trails run teacher to teacher

- Crier Toke starts the fighting, smithing, brewing and tracking trails.
  Each first job begins by talking to the teacher Toke names, and each
  teacher sends you on to the next one.
- Every job on these trails now ends by talking to the person who gave
  it, the big fights and the repeat jobs included. The last teacher on
  each trail sends you back to Toke with the news.
- First Brew counts two healer's roots, and its hint names a buy
  command that works.
- In Wade the Marsh, the firebomb only counts when the blast reaches a
  creature.
- The Hunt now asks you to eat the meal you cooked.
- Selling your first dagger to Smith Rusk is still fine. Talk to her
  afterwards to finish First Heat.
- Drillmaster Vorn explains grappling, shield bashes, kicks and how you
  defend. Trader Onna explains appraise, offer, the identify spell and
  how prices move. The crafters in town each name something you can
  make at their station.
- The enchanter in the Warder's Dugout is now Enchanter Daff.
- Anyone who learns the scab-hide jerkin can sew it.
```

- [ ] **Step 3: Docs index.** In `docs/README.md`, directly below the row for `superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md`, add:

```markdown
| [`superpowers/plans/2026-10-30-p7-coulee-3a-lines-fight-smith-brew-track.md`](superpowers/plans/2026-10-30-p7-coulee-3a-lines-fight-smith-brew-track.md) | Plan 3a of the #375 phase 7 Coulee spec: the fighting, smithing, brewing and tracking lines run Toke, giver, giver, with every finale and repeatable ending at a hand-in and bridges 90 to 93 back to Toke; the `eat` command event and quest 42's eat step; Vorn, Onna and crafter skill topics; Enchanter Skell renamed Enchanter Daff; the jerkin at rank 1; 39 backlog lines cleared. Records that bridge file names follow `ConvertForFilename` (`90-report_to_toke__fighting_.yaml`) |
```

- [ ] **Step 4: Commit.**

```bash
git add docs/PATCH_NOTES.md docs/README.md
git commit -F - <<'EOF'
docs: patch notes and index row for the Coulee lines A pass

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

- [ ] **Step 5: Record the #697 finding** (after the PR exists, Task 10 Step 7). Write the comment to the session scratchpad with the Write tool, then post it with `gh issue comment 697 --repo pruuk/DOGMud --body-file <scratchpad path>`:

```markdown
Kit salve item (#375 phase 7, PR <number>): no change needed, and here is why.

- The potion path prints an age line only when `CraftedRound > 0` (the gate in the quaff path, formerly `internal/actions/drink.go:234`).
- The `early` playtest kit (`tools/playtest/profiles/early.yaml`) gives item 30036 with no `crafted_round`, and nothing stamps one: `items.New` and `Item.Validate` never set it, and `internal/hooks/pinnacle_tick.go` documents purchased and spawned potions as never aging.
- So the kit salve prints no age line at all. The "The potion is freshly brewed." line in the 3b slice 2 playtest came from the salve the seat brewed itself in quest 38, which is correct.

The other #697 items: the post-craft line in First Heat now names the sell route and the talk that finishes the quest (this PR); the drink/quaff ordering and the mote line are plans 1 and 3b.
```

---

## Task 10: Gate and PR

Suggested executor: Sonnet; Opus if anything is red.

**Files:** none new.

- [ ] **Step 1: Format and build.**

```bash
gofmt -l internal/ modules/
go build ./...
```

Expected: no gofmt output; build succeeds.

- [ ] **Step 2: Touched packages and guards.**

```bash
go test -count=1 ./internal/questengine/ ./internal/usercommands/ ./internal/actions/ ./internal/dialogue/ ./internal/narration/ ./internal/questloop/ ./internal/quests/ ./internal/mobs/
go test -count=1 ./internal/guards/...
```

Expected: every line `ok`. (`internal/rooms` zone-lifecycle tests fail on Windows only under `DOGMUD_BOOT_SMOKE=1`; they are not run here.)

- [ ] **Step 3: The shipped-data loop and boot smoke tests (they skip without the variable).**

```bash
DOGMUD_BOOT_SMOKE=1 go test -count=1 -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' . 2>&1 | grep -E '^(--- |ok|FAIL|=== RUN)' | grep -v '=== RUN'
```

Expected: `ok`, and `--- PASS` lines for `TestSmoke_ShippedQuestsPassTheLoop`, `TestSmoke_ShippedDialoguePassesTheValidators`, `TestSmoke_ServerBootsCleanWithRealData`, `TestQuestDoor_RefusesAFreshFinding`, `TestDestinationDoor_WarnsAndNeverRefuses`, `TestQuestLoop_GuardCanFail`. Any `--- SKIP` means the variable did not reach the test: rerun from Git Bash exactly as written.

- [ ] **Step 4: Backlog done-when for this plan's quests** (run each on its own; expect no output):

```bash
grep -nE '^\S+ (3[2-9]|4[0-3]|5[3-6]|9[0-3]) ' _datafiles/world/dogmud/quests/loop-backlog.txt
grep -nE '^chain (3[2-9]|4[0-3]) ' _datafiles/world/dogmud/quests/loop-backlog.txt
```

- [ ] **Step 5: Lint.**

```bash
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `0 issues.`

- [ ] **Step 6: Isolated boot check** (`dogmud-shipping`; never `go run`; never touch the owner's server).

```bash
S="C:/Users/CALABE~1/AppData/Local/Temp/claude/<session>/scratchpad"   # your session scratchpad, full path
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
grep -nE "quest (3[2-9]|4[0-3]|5[3-6]|9[0-3])\b" "$S/boot.log" | head
cd C:/tmp/dogmud-375-p7-coulee-lines-a
git worktree remove --force C:/tmp/dogmud-boot-check
```

Expected: `exit 124` (the server stayed up until the timeout), `0` panics, `1` `Server Ready`, and no warning line naming quests 32-43, 53-56 or 90-93. If `git worktree remove` fails on a locked exe, use PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check` then `git worktree prune`.

- [ ] **Step 7: Push and open the PR** (always `--repo pruuk/DOGMud`; the repo is a fork and `gh` defaults to upstream). Write the body to the scratchpad with the Write tool first:

```markdown
## Summary

#375 phase 7, Pothole Coulee, plan 3a: the fighting, smithing, brewing and tracking lines.

- Each line runs Toke, Giver 1, Giver 2, Toke. Quest 1 starts with a talk to Giver 1, quest 2 (now offered by Giver 1) with a talk to Giver 2.
- Every finale (34, 37, 40, 43) and repeatable (53, 54, 55, 56) ends at a hand-in on Giver 2. Bridges 90 to 93 (Report to Toke) are offered by Giver 2 and handed in at Toke.
- Issue fixes: #694 (throw must reach a creature), #695 (two roots, `buy 2 healer's root`), #697 (First Heat names the sell route; kit salve finding recorded on the issue), #690 (dashes in these quests and two conversation pairs).
- Skill coverage: `eat` raises a command event and The Hunt gains an eat step; Vorn, Onna and the crafters gain teaching topics; Vorn's shouts topic corrected; Enchanter Skell renamed Enchanter Daff; the scab-hide jerkin at skill_minimum 1.
- 39 loop-backlog lines cleared. No line names quests 32-43, 53-56 or 90-93.

## Notes for review

- Bridge files are named through `ConvertForFilename` (`90-report_to_toke__fighting_.yaml`), which the loader requires.
- `32-consider` is renamed to `32-special` in `step-ids.txt`, so a player saved on it moves on.
- In Toke's shared file this PR touches only his four hand-ins and the text and hints of his four quest-1 offers (now "go see the giver", routes checked against room exits).
- `enchanting.template` now says disenchant takes an item from the pack, matching `skill.disenchant.go`.
- No playtest here: the spec puts one naive-newbie playtest across the whole flow in PR 5.

## Verification

- gofmt clean, build, touched packages, guards: ok
- `DOGMUD_BOOT_SMOKE=1` loop and boot smoke tests: PASS (not skipped)
- golangci-lint: 0 issues
- isolated boot: exit 124, 0 panics, Server Ready

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

```bash
git push -u origin feat/375-p7-coulee-lines-a
gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-coulee-lines-a --title "feat(quests): Coulee lines A, fighting to tracking, giver to giver (#375 phase 7)" --body-file "<scratchpad path>/pr-body.md"
```

Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Then run Task 9 Step 5 with the PR number. Do not merge: the controller and the owner decide.

