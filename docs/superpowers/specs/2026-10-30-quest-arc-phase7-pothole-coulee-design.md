# #375 phase 7, zone pass 1: Pothole Coulee

**Status:** design approved in brainstorm 2026-10-30; awaiting owner review of this file.
**Arc:** #375 quest and dialogue rework, phase 7 (content migration, zone by zone). Roadmap:
`docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`; target page: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`.
**Issues closed by this pass:** #315, #689, #690, #694, #695, #697. **Touched, left open:** #625
(interim disc home only).
**Sequencing (owner 2026-10-30):** spec and plan are written, reviewed and parked as ready; #317
(ambient retune) ships next; then this plan is executed.

## Facts verified against source (master `8432192f1`, 2026-10-30)

| Fact | Source |
|---|---|
| Quests in this pass: 28 (antechamber), 29 to 59, 76, 88, 89 | `_datafiles/world/dogmud/quests/` |
| Free quest ids: 1, 3, 22 to 27, 64, and 90 up | `ls quests` (ids in use: 2, 4 to 21, 28 to 63, 65 to 89, 1000000) |
| Dialogue files in this pass: 29 in `dialogue/pothole_coulee/`, 1 in `dialogue/newcomer_antechamber/` (9491 Dewey) | `ls dialogue/...` |
| Coulee quests (29 to 59, 76, 88, 89) carry 88 backlog lines: 39 end, 24 chain, 11 one-action, 7 walk-dead-end, 3 start, 2 walk-destination, 2 hint-length. Quest 28 carries 7 more (2 start, 2 end, 2 one-action, 1 walk-destination) | `quests/loop-backlog.txt` |
| A backlog line with no matching finding fails CI, so clearing a finding forces deleting its line | header of `loop-backlog.txt`; `internal/questloop/backlog.go` |
| Walkthrough findings (`walk-dead-end`, `walk-destination`) share the same backlog | `internal/questengine/destinations_rule.go:15`, `walkthrough_test.go` |
| `ruleChain`: an offer gated on quest N's final token must sit on an NPC holding a hand-in topic for N | `internal/questloop/rules_more.go:14-52` |
| `ruleStart` / `ruleEnd`: only an offer topic may grant `-start`; only a hand-in topic may grant the final token | `internal/questloop/rules_loop.go:15-55` |
| Gates: `questRequired` is all-of, `questExcluded` is none-of; flags match an exact string; no any-of gate exists | `internal/dialogue/engine.go:27-75`, `topic.go:15-25` |
| Offer topics hide themselves once the quest is open (`QuestOpen`) | `internal/dialogue/topics.go:64-80` |
| Step ids are pinned by `quests/step-ids.txt`; removal goes through its `# retired` / `# renamed` sections, applied on load by `RetiredQuestSteps` / `RenameQuestSteps` | `step_manifest_test.go`, `internal/users/users.go:572-575`, `internal/quests/manifest.go:60` |
| Topic keys are pinned by `dialogue/topic-keys.txt` | `internal/guards/topic_manifest_test.go:40` |
| Every character starts with every skill at rank 1 (`initAllSkills`) | comment in `quests/50-first_shot.yaml:34` |
| `steal` command needs rank 1; the theft itself needs rank 2 at four sites | `internal/usercommands/skill.skullduggery.steal.go:33`; `internal/actions/steal.go:258,427,533,843` |
| `shadow` needs rank 3 (stays, owner 2026-10-30) | `internal/usercommands/skill.skullduggery.shadow.go:31` |
| `picklock` needs rank 1 and a lockpicks item in the pack; works on locked exits and locked room containers (`lock: {difficulty, trapconditionids}`) | `internal/usercommands/picklock.go:24,33-44` |
| Trader Onna (9104) sells a torch (20096) and a hooded lantern (20097) | `mobs/pothole_coulee/9104-trader_onna.yaml:58-59` |
| Cave biome has `skylight: 0.0`; Coulee cave rooms: mine 5255 to 5261 (5255 Mine Head overrides `skylight: 0.50`), Ancestral Cave 6468, 6469 | `biomes/cave.yaml`; room files |
| Behaviour trees offer `event: player_enter`, `player_has_quest`, `random_chance {percent}`, `respond {user_text, room_text}` | `internal/behaviortree/conditions*.go`, `actions.go:27`, `conditions.go:24` |
| Quest 30: `30-start` is auto-granted on entering 5200 and by the room 6467 tree; `30-rite` and `30-end` are granted by Hadwen's tree (`9100-cleric_hadwen.yaml:71,88,253`); 30-end is referenced in 49 non-save files, including Go (`behaviortree/actions_quest.go`, `characters/migrations.go`, `usercommands/start.go`) | quest 30 header; grep |
| Room 6467 tree grants `28-end` and `30-start` | `behaviors/rooms/newcomer_antechamber/6467.yaml:106-111` |
| Quest 89 is offered by Hadwen (topic `road_esk`) and ended by `room_enter 468`; quest 88 by Hadwen (`road_toke`), ended by `quest_granted 31-start`, with a hand-in topic at Toke | backlog lines; dialogue 9100, 9106 |
| Quest 76: the Inert Disc 40167 is picked up in the Reliquary (5343) by a typed `disc` noun; woken at Maren's Cottage (4023, Ashwick); the Attuned Disc 40168 gates the Threshold Keeper (9553); Brennan the Surveyor is 9516 (Greenford) | `quests/76-the_disc.yaml`; `dialogue/eastern_highlands/9553.yaml`; `dialogue/greenford/9516.yaml` |
| Quests 49 and 58 progress on a typed-noun `room_interact` (`search floor` in 5343) | `quests/49-the_old_shrine.yaml:90-93`, `58-reading_the_stones.yaml:53-56` |
| `drink` notifies the quest engine (line 317) before printing "You drink" (line 321) | `internal/actions/drink.go:317-321` |
| `hint` with no quest prints `talk &lt;name&gt;` inside an ansi tag | `internal/usercommands/hint.go:19` |
| `consider` pacing copy "beating you would take them an age" | `internal/actions/fight_read.go:192` |
| Dash lines in the pass's quest files: 168, of which 115 are YAML comments (53 player-facing). Coulee dialogue and quest 28 / Dewey: 0. Conversation pairs with Coulee NPCs (9101_9106, 9103_9105, 9106_9107): 2 | grep for ` -- `, em and en dashes |
| The global prose dash guard still exempts quests, dialogue and conversations until phase 7 closes | `internal/guards/prose_dash_guard_test.go:14-18` |

## The player flow

### The opening (from landing to the first line)

1. **Antechamber, quest 28 (Waking to Gaius).** Dewey (9491) offers it with an offer topic; the
   `room_enter 6258` start goes. Its end becomes a hand-in at Dewey on his `ready` topic (the
   "I'm ready to go out into the world" line, with the #690 jargon removed). After the hand-in
   the existing 6467 handoff moves the player to the pool; the 6467 tree stops granting `28-end`
   and `30-start`. Dewey's `marks` topic is where the markers are first taught (requirements
   page, "Players are taught the system").
2. **The rite, quest 30.** Only Hadwen's `rite_offer` starts it: the `room_enter 5200` trigger and
   the 6467 grant are removed. A new step `woken` sits between `rite` and `end`: Hadwen's tree
   grants `30-woken` at the rite's close where it grants `30-end` today, and a new hand-in topic
   on Hadwen grants `30-end`. `30-end` keeps its meaning ("Opened"); it now lands when the player
   talks to Hadwen after the rite. Under phase 5 ruling 6, quest 30 becomes abandonable once
   nothing auto-grants it; that is harmless (Hadwen offers it again).
3. **The roads.** Hadwen's hand-in leads to his offer of **Toke's Road (88)**; its
   `quest_granted 31-start` finisher is removed so the hand-in at Toke is its only end. Toke
   then offers **Find Your Footing (31)**.
4. **Skipping the Coulee** is an info topic on Hadwen (working label "Is there another way
   out?") that points to Esk. Esk's existing `passage` topic does the crossing. Hadwen's
   `road_esk` offer is removed. This is the requirements page's "portal quest is an info topic".
5. **Quest 29 (Two Roads)** is retired: `29-start` and `29-end` move to the `# retired` section
   of `step-ids.txt`, and the quest file is deleted.

### Toke is the hub

Once 31 is handed in, Toke shows the start of every line the player has not taken. The player
takes as many as they like (owner: "they can choose to take all, but don't have to").

### One line, giver to giver

Each line follows the same pattern, and every offer sits on the NPC who took the previous
hand-in, which is exactly what `ruleChain` asks for:

1. Toke offers quest 1. Its first step is "go see Giver 1"; Giver 1 takes the hand-in.
2. Giver 1 offers quest 2. Its first step is "go see Giver 2"; Giver 2 takes the hand-in.
3. Giver 2 offers quest 3, the finale. Kill, craft, drink, forage or search finales gain a closing
   "report back" step; Giver 2 takes the hand-in.
4. Only now does Giver 2 offer the line's **Report to Toke** bridge, plus the optional repeatable.
   Toke takes the bridge's hand-in.

Givers drop their duplicate offers of a line's quest 1 (Toke is the only starter) and get a
breadcrumb instead ("Toke sends new folk my way").

| Line | Quest 1 (Toke offers) | Giver 1 | Quest 2 | Quest 3 (finale) | Giver 2 | Repeatable | Bridge |
|---|---|---|---|---|---|---|---|
| Fighting | 32 First Blood | Vorn 9108 | 33 Hold the Wash | 34 Take the Tower | Garve 9112 | 53 | 90 |
| Smithing | 35 First Heat | Rusk 9116 | 36 Pick the Vein | 37 Into the Vein | Ovell 9117 | 54 | 91 |
| Brewing | 38 First Brew | Birna 9128 | 39 Wade the Marsh | 40 Heart of the Mire | Falv 9129 | 55 | 92 |
| Tracking | 41 First Sign | Tarn 9136 | 42 The Hunt | 43 Break the Pack | Delk 9137 | 56 | 93 |
| Casting | 44 First Casting | Grieve 9144 | 45 The Unquiet Grove | 46 The Unfolded | Orrin 9145 | 57 | 94 |
| Lore | 47 First Words | Wenna 9153 | 48 The Standing Stones (Hale 9157) | 49 The Old Shrine | Sere 9160 | 58 | 95 |
| Archery | 50 First Shot | Iden 9161 | 51 Across the Canyon | 52 The Overlook | Bryn 9162 | 59 | 96 |
| Shadows (new) | 98 First Light | Lamplighter (new) | 99 Quiet Hands | 100 From the Dark | Cutpurse (new) | 101 | 97 |

The lore line has three givers: Wenna offers 48 (go see Hale), Hale offers 49 (go see Sere), and
Sere offers the repeatable and the bridge.

### Leaving

- **Quest 89 (Esk's Road) is reused as the completer.** Toke offers it once he holds all eight
  bridge hand-ins (`questRequired: [90-end ... 97-end]`, all-of); Esk takes the hand-in, and his
  `passage` topic does the crossing. Its step ids (`start`, `end`) stay, so a save holding 89
  still finishes at Esk. Its `room_enter 468` finisher is removed.
- **Repeatable flavour lines (owner 2026-10-30).** A new tree for Toke
  (`behaviors/pothole_coulee/9106-crier_toke.yaml`) and a new branch in Esk's: `event:
  player_enter`, `player_has_quest` for each of `90-end` to `97-end`, `random_chance` at 25,
  then `respond` with `user_text` only, so only that player hears it. A handful of lines, one
  picked at random, each naming a giver and their repeatable work.

## The Shadows line (#315)

New content, built to the same shape. NPC names are placeholders until the plan picks them;
the plan checks whether an existing townsperson (Onna, Wicker, Croup, Garrow) can fill a role
before adding a mob, and runs `tools/id_inventory.py` before choosing any id.

| Quest | Giver | Steps (one action each) | Reward |
|---|---|---|---|
| 98 First Light | Lamplighter, posted at the Mine Head (5255) | go see the lamplighter (who hands over a hooded lantern, 20097, through `givesItem`); walk into the dark drift (5256) carrying it; hood it (`hood`); report back | 15 gold; the lamplighter's info topics name Onna as the seller of torches and lanterns |
| 99 Quiet Hands | Cutpurse | go see the cutpurse; sneak (the `command` event fires only when the player ends hidden); steal the token from the practice mark; plant it back; pick the practice lockbox (lockpicks handed over by the offer's `givesItem`); report back | skullduggery:1, 15 gold |
| 100 From the Dark | Cutpurse | ambush a target in the dark mine (a surprise attack from hiding); kill it; report back | 15 gold |
| 101 (repeatable) | Cutpurse | lift a set number of tokens from the marks; report back | as the other repeatables, with `cooldown_rounds` |
| 97 Report to Toke | offered by the cutpurse | talk to Toke | none |

- **Info topics (`[*]`).** Lamplighter: night, infravision, magical darkness, dazzle as a cost.
  Cutpurse: how shadowing works (rank 3, earned later), how the surprise strike works, how
  defuse works. None of these is a quest step (owner 2026-10-30).
- **Practice mark:** a new mob in the style of the training dummy (9109) and the practice butt
  (9163), carrying a stealable token and respawning quickly. Townspeople stay immune to theft.
- **Practice lockbox:** a locked room container in the cutpurse's room, low difficulty, no trap.
  The plan checks how a picked lock re-locks, so the next student is not left an open box.
- **Rank change:** a new config knob `StealMinRank` (shipped at 1, declared in
  `internal/configs/config.balance*.go`, surfaced in `_datafiles/config.yaml`) replaces the four
  hardcoded rank-2 checks in `internal/actions/steal.go`. Players and mobs share it.
- **Overlap with archery:** First Shot (50) already teaches one sneak and a shot from cover. The
  Shadows line does not depend on archery being done first.

## Converting the existing lines

- **One action per step, each hinted with a command the player can type.** The 11 `one-action`
  and 7 `walk-dead-end` findings are split or re-pointed; the 2 `hint-length` findings rewritten.
- **Every finale ends at a hand-in** (the `end` findings), as above. Quests 35, 54, 55, 56 and
  58 end on a sell, craft, drink, forage or search today; each gets the same report-back step.
- **Typed nouns go** (requirements: zero typed-noun `room_interact`). Quests 49 and 58 progress
  on the real `search` command in room 5343 (`event: command, command: search, room: 5343`).
- **Walk destinations:** 42 `hunt` and 28 `fled` get a destination the walk can reach.
- **Issue fixes:**
  - #694: quest 39's throw trigger gains `target: mob`; recheck the step hint.
  - #695: quest 38's roots step uses `count: 2`; the hint becomes `buy healer's root`.
  - #697: the First Casting line stops calling the mote unharmed; the potion path (now `quaff`)
    prints its "You drink" line before notifying the quest engine; Rusk's post-craft line mentions the sell route; the
    `early` kit salve's starting age is checked and fixed if it reads "freshly brewed" wrongly.
  - #689: the lore line starts from Toke like every line; Hale's directions read west, then
    north; the School Hall notice board names a command that works (`talk`, not `ask`).
  - #690: every checklist item. The engine ones: `hint.go:19` escaping (confirm on the telnet
    port first), `consider` copy at `fight_read.go:192` ("them" and "an age" twice), the Straw
    Effigy "DEVASTATING BLOW" while it stays healthy, and Garrow's taunt (quest 48) promising
    he leaves when a real fight starts. Menu renumbering: see the owner ruling below.
- **Dashes:** every player-facing dash in the pass's quests, dialogue and the three conversation
  pairs is rewritten as a sentence without one. A new guard test holds the Coulee and antechamber
  quest, dialogue and conversation files dash-free; the global guard's exemption is untouched
  until phase 7 closes.

## Skill coverage (audit and owner rulings, 2026-10-30)

An Opus audit compared every skill's help page with what the zone teaches (103 files read). The
claims below were rechecked against source before this section was written.

**Rulings**
- **Potions are drunk with `quaff`; `eat` and `drink` become aliases for each other** and consume
  cooking output (food and drinks). Today Herbal Tea (30024) is typed `potion`, while Spiced Wine
  (30025) and Antidote Broth (30027) are typed `food` and need `eat`. Cooking drinks move to the
  existing but unused item type `drink` (`internal/items/itemspec.go:142`); `eat` and `drink`
  accept both `food` and `drink`. `quaff` takes over today's potion path (`internal/actions/drink.go`,
  including its quest notify, renamed to `quaff`). Blast radius measured: 2 quest triggers
  (quests 38, 55), 10 player-facing "drink" lines in YAML and help, 17 Go references, 3 web-client
  references. `drink` or `eat` on a potion consumes nothing and answers with one line naming
  `quaff` (Claude's call; owner may overrule). The plan checks whether Herbal Tea relies on the
  potion pipeline (magnitude, aging) before retyping it: `eat` applies `ConditionIds` only
  (`internal/usercommands/eat.go:89`). Ships in PR 1.
- **Manifestation is taught in dialogue on the casting line** (Grieve 9144, Orrin 9145): what
  manifestation is, conjure, raise, charm, and `assess` on a corpse before raising it. No quest
  step: there is no starter summon spell.
- **Passive defences** (dodge, parry, block, reach) get a breadcrumb to `help combat`, on Vorn.
- **The scab-hide jerkin** (quest 43's recipe reward) drops to `skill_minimum: 1`
  (`recipes/tailoring/scab-hide-jerkin.yaml:5`, today 8), so a fresh player can make it.
- **Enchanter Skell (9606) is renamed.** Bowmaster Skell (9169) keeps the name; he is quest 52's
  boss. The plan picks the new name and updates every reference.

**Additions**

| Kind | Where | Addition |
|---|---|---|
| Quest step | Quest 42 | `eat` the cooked meal, after the cook step (quest 43 then sends the player into the pack fight carrying food; nothing in the zone says `eat` today) |
| Info topics | Vorn 9108 | grapple; shield `bash`; why a kick can read as stomp, knee or tailsweep; passive defences pointing to `help combat` |
| Info topics | Hale 9157 | rally and warcry |
| Info topics | Grieve 9144 | new spells come from casting, not trainers; manifestation topics as above (shared with Orrin) |
| Info topics | Onna 9104 | `appraise`, `offer`, the free `identify` spell, bartering skill and prices |
| Info topics | Bex 9605, Vurl 9607, Skell 9606, Hesk 9608 | how to craft at their station, naming a recipe a fresh player can make; Skell also covers `disenchant` |
| Info topics | Cutpurse (new) | getting caught, hot goods and fences; where lockpicks and disarm kits come from; defuse framed as a rank 3 skill learned later (`internal/actions/defuse.go:65`) |
| Corrections | Vorn's `shouts_help` (`9108.yaml:211-219`) | Shouts do not come later: rally has no rank gate, and quest 28 already teaches warcry |
| Corrections | `templates/help/steal.template:22`, `plant.template:20` | Both say rank 2; plant's code checks rank 1, and steal follows `StealMinRank` |
| Left out | | player `trade`; salvage (already taught by Rusk and Ovell) |

## Quest 76, interim disc home (owner option A)

- The Inert Disc (40167) is found at Maren's Cottage (4023) by a real action (`search` there),
  not in the Reliquary. The journal and attune steps stay at 4023.
- The Reliquary (5343) loses the `disc` hidden noun and its floor text stops hinting at it.
- 76's 8 backlog lines are cleared: a hand-in at Brennan (9516) ends it, and the
  `room_interact` triggers become real actions.
- #625 stays open for the full endgame redesign.

## Delivery

| PR | Contents |
|---|---|
| 1. Engine and config | `StealMinRank`; `quaff` and the `eat`/`drink` aliases, with item retyping; `drink.go` ordering; the #690 engine nits; the menu-close notice; their tests |
| 2. Spine | Quests 28, 30, 88; 29 retired; Toke as the only line starter; Hadwen's skip info topic |
| 3. Seven lines | Conversions, issue fixes, dashes, typed nouns, bridges 90 to 96, quest 76's interim home |
| 4. Shadows line and leaving | New NPCs, practice mark, lockbox, quests 97 to 101, info topics; quest 89 moves to Toke; Toke and Esk flavour trees |
| 5. Close | Adversarial playtest with naive-newbie seats across the whole flow; findings filed |

Each PR passes the pre-push gate (gofmt, build, tests, lint, boot). A finding not listed in
`loop-backlog.txt` refuses boot, and every merge to master can be deployed, so each PR must boot
clean on its own: a change that would raise a new finding ships in the same PR as its fix. That
is why the bridges ship with the line conversions (a bridge offered before Giver 2 takes quest
3's hand-in is a new `chain` finding), and why quest 89 keeps Hadwen's offer until PR 4, when
all eight bridges exist for Toke's offer to gate on.

## Done when

- No line in `loop-backlog.txt` names quest 28 to 59, 76, 88 or 89 (CI enforces the deletion).
- `step-ids.txt` and `topic-keys.txt` carry every new id and key; 29's ids are retired.
- The walkthrough reaches every step of every quest in the pass.
- The Coulee dash guard passes.
- The server boots with no warnings for the pass's quests.
- The playtest has run and its findings are filed as issues.
- #315, #689, #690, #694, #695 and #697 are closed; #625 has a comment recording the interim home.
- `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md` phase 7 records this zone as done; `context.md`
  files for any touched package are updated; `PATCH_NOTES.md` has a player-facing entry.

## Owner ruling: menu renumbering (#690, 2026-10-30)

A talk menu numbers lines by position (quest lines first, then info lines in file order,
ineligible lines skipped: `internal/dialogue/topics.go:157`), so a newly unlocked line or a
quest appearing shifts the numbers. Ruled **(a)**: the numbering stays, and a typed command that
closes an open menu now prints one line naming the conversation it ended. This reverses phase
0's silent close for `CloseOtherCommand` (`internal/menu/lines.go:35`). Rejected: (b) appending
newly eligible lines at the bottom (writers lose control of menu order, and quest lines still
shift everything). Ships in PR 1.
