# Quest and Dialogue Arc: Roadmap

> Living document for epic #375. The target is `QUEST_DIALOGUE_REQUIREMENTS.md`; this page is
> the route: steps, ordering and approach. Update each phase's **Status** as work lands.
> Each phase gets its own spec and plan in `docs/superpowers/specs/` and
> `docs/superpowers/plans/` when it is picked up. Brainstormed with the owner 2026-10-20 from
> `docs/Quest_arc_screenshots/`.

## Facts verified against source (master `a3b82e5ac`, 2026-10-20)

Every phase spec re-verifies the facts it builds on; this table is the starting point.

| Fact | Where |
|---|---|
| `talk <npc>` only greets (root/greeting line plus a bracketed hint); topics need `ask <npc> <word>` | `internal/usercommands/talk.go:24`, `ask.go:72` |
| `talk` has an LLM greeting path, off unless `LLMConfig.Enabled` and the mob has an `LLMProfile` | `talk.go:69` |
| A dialogue file mixes stateless `patterns` and a stateful `tree` | `internal/dialogue/context.md` |
| `Pattern`, `TreeNode` and `QuestGreeting` share one set of gating fields, checked by `checkQuestGate` and applied by `applyQuestEffects` | `internal/dialogue/context.md`, `engine.go` |
| 302 dialogue files in 34 zone folders; 79 quest files | `_datafiles/world/dogmud/dialogue/`, `quests/` |
| Dialogue memory is in-memory only, keyed by mob INSTANCE and user, lost on respawn or restart | `internal/dialogue/memory.go:18` |
| Quest progress is saved per character as `QuestProgress map[int]string` | `internal/characters/character.go:319` |
| A quest step has `id`, `description`, `hint` (optional), `map_target`, `map_target_mob` | `internal/quests/quests.go:72` |
| 12 quest files carry no `hint:` at all | grep of `quests/` |
| Valid trigger events: room_enter, room_gate, item_give, skill_use, mob_death, command, item_gain, dialogue, quest_granted, room_interact, command_issued | `internal/quests/quests.go:109` |
| Shipped trigger counts: room_interact 64, command 49, item_give 32, mob_death 27, quest_granted 21, item_gain 20, command_issued 14, room_enter 11, room_gate 1, dialogue 1 | grep of `quests/` |
| Mid-quest rewards exist: trigger actions `give_item` 27, `learn_recipe` 26, `give_gold` 6 | grep of `quests/` |
| Minimap target today: one room, or a unique NPC's current room; no zone or area target | `internal/questengine/map_target.go` |
| Repeatable quests exist: `repeatable` plus `cooldown_rounds`, cooldown saved in MiscData; 7 shipped (53 to 59). No limit on repeat count | `quests.go:69`, `internal/characters/quests.go:192`, `internal/hooks/Quest_HandleQuestUpdate.go:207,442` |
| No quest abandon command exists | grep of `usercommands`, `quests`, `questengine`, `characters` |
| 5 quests start from a trigger grant instead of an NPC: 15, 16, 28, 30, 31 | grep `grant: N-start` |
| Quest 21 (Newcomer's Path) is a secret quest with no triggers, used only as a marker | `quests/21-newcomers_path.yaml` |
| The nine pinnacle recipe quests are 79 to 87 ("Secret of the ...") | `quests/79-*.yaml` to `87-*.yaml` |
| Existing menu code: `internal/prompt` (per-user Q&A session, input taken before commands) and `tables/numbered-list` templates over `templates.NameDescription`, admin builders only | `internal/prompt/prompt.go`, `internal/users/userrecord.go:784`, `internal/templates/name_description.go:4` |
| The admin builders copy one "render list, Atoi, range check, index, fall back to name" block: 6 menu-index parses (`admin.item.go:201,356`, `admin.room.go:423,592,779`, `admin.zone.go:149`) behind 19 `numbered-list` renders. (The other 12 `Atoi(...Response)` sites parse typed numbers, not menu choices.) | grep of `internal`, `modules` |
| Choice prompts with fixed options: 27 `prompt.Ask` calls with a literal option list (yes/no, y/n, UP/DOWN/quit), 13 of them in admin builders | grep of `internal`, `modules` |
| Only `presence` and `perception` expose `Subscribe` on their machine wrappers; `combatphase`, `activity`, `life`, `position`, `control` keep `inner *state.Machine` private | `internal/state/*/*.go` |
| Rounds are 4 s (`RoundSeconds: 4`) | `_datafiles/config.yaml:193` |
| No `[!]`, `[?]`, `[*]`, `[i]` or `[+]` marker is in use in templates or Go | grep |
| The web client quest tracker gets its data over GMCP | `modules/gmcp/gmcp.Quest.go`, `gmcp.Quest_refs.go` |
| Dash count waiting on this arc: dialogue 75 files / 430 lines, quests 43 / 104, conversations 6 / 6 | #375 comment, 2026-10-17 |

## Approach

- **Engine first, then content zone by zone.** Phases 0 to 5 build mechanisms and keep every
  existing NPC and quest working. Phase 7 then ports content one zone at a time, applying
  everything at once, so each zone is touched once and playtested once.
- **Extend, don't replace.** The topic structure wraps today's dialogue entries and reuses
  their gating fields. Existing NPCs appear in the menu from phase 1 with fallback labels;
  the zone pass gives them real ones.
- **Reuse before inventing.** The menu primitive absorbs `prompt` and the numbered-list
  templates; quest state stays in quest tokens; repeat frequency stays on `cooldown_rounds`.
- **Validators enforce the rules.** Each requirement that can be checked at boot is a
  validator. A validator that the old content would fail starts as a warning list and becomes
  a hard boot failure once phase 7 has cleared that list.
- **Players and mobs share rules** (standing owner rule): nothing here special-cases players
  where a mob path exists.
- **Validators take a proposed world, not just one file.** From phase 1 on, the quest and
  dialogue validators accept the loaded world plus pending edits, so the phase 6 builder can
  check a save against the world as it would be after the save and publish a quest with its
  giver's topics as one unit. Designing this in now is cheaper than retrofitting it.
- **Steps stay an ordered list; progress stays one step id per quest.** A character saves only
  its current step id (`QuestProgress map[int]string`), compared by list position
  (`IsTokenAfter`). Chains are declared quest to quest (a "next quest" field); steps keep their
  order inside each quest. Renaming or removing a shipped step id strands players on it, so
  step ids are covered by the stable-id manifest guard.
- **Docs ship with each phase.** Every phase updates the `context.md` of each package it
  touches (new packages ship one) and the player help it changes (`templates/help/talk`,
  `ask`, `quests`). The Pothole Coulee pass teaches the markers and the menu in play.
- **Every zone pass ends in an adversarial playtest** (`dogmud-playtesting`), and its findings
  are filed as issues before the pass closes.

## Topic structure (shared by every topic)

```
key        stable id, unique within the NPC ("directions")
kind       info | quest
label      menu text in natural language, no "Ask:"/"Task:" prefix
           ("How do I reach the steppe?")
text       what the NPC says (quest: the pitch)
marked     info only: lights [*] until heard
gates      the existing gating fields (questRequired, questExcluded, quest flags,
           itemExcluded, goldRequired, masterworkRequired...) plus new skill/stat gates
quest      quest only: quest id; offer and hand-in state come from tokens
breadcrumb shown while a quest topic is hidden for ineligibility
source     authored | llm
```

- Heard info is saved on the character as a set of `<mob template id>.<key>` strings. Menu
  numbers are display order only.
- Quest topics derive their marker from tokens: `[!]` eligible and not started, `[?]` final
  step reached and ready to hand in. No second record to drift.
- LLM topics get a generated key when stored and are then indistinguishable from authored ones.

## Build order

```
0 Menu primitive
1 One talk command          (needs 0)
2 Offer, hand-in, loop      (needs 1)
3 Action triggers  ┐
4 Minimap targets  ├ engine halves, any order after 2
5 Abandon / focus  ┘
7 Content, zone by zone     (needs 2 to 5; Pothole Coulee first)
6 Web builder               (last; the owner is the only admin until then)
```

## Phase 0: Menu primitive

**Goal.** One menu package every numbered choice in the game uses.

**Approach.**
- New package (name settled in the spec) owning: entries (number, marker, label, optional
  second line, id, enabled or dimmed), one-column rendering at the 80-character wrap,
  choice resolution by number or by name, Enter to leave, and an invalid-choice reply.
- No clicking (owner 2026-10-20): menus are typed in every client. DOGMud's terminal is
  xterm.js 4.19 with no link provider, and typing a number is as fast as a click.
- Game menus are non-modal: a number, an option name or Enter goes to the menu; any other
  input closes it and runs as a normal command. Admin builder menus stay modal (their steps
  mix menus with free-text answers) and ignore game interrupts unless they opt in.
- Interrupts (owner 2026-10-20): a game menu cannot open, and an open one closes, while the
  player is in combat, casting, crafting or salvaging, dead or respawning, asleep, off their
  feet or controlled, or quitting; any room change closes it (walked, fled, or pulled by a
  leader). Closing listens to the `internal/state` machines' observers and the condition
  path, and every choice re-checks the whole rule set before acting. The gate is built from
  the same sources as the future shared busy check (#640) and moves onto it when it lands.
- Talk menus also close when the NPC leaves, dies or stops being visible by name. A leader
  pulling a follower away tells the follower ("You break off your talk with Kael to follow
  Brannoc."), the leader ("Mira breaks off a talk with Kael to follow you.") and the room
  left behind ("Mira breaks off a talk with Kael and follows Brannoc."), all through the
  sight rules. Other interrupts give one line naming the cause.
- Input session: built on `UserRecord.StartPrompt`; callers never touch `prompt` directly.
- Migrate every fixed choice onto it (owner 2026-10-20): the admin builders' 6 copied menu-index
  parse blocks and 19 `numbered-list` renders, the 27 fixed-option prompts (yes/no confirms
  in character, guild, mail, rename, auctions, start and the builders), the `character` menu
  and picklock's UP/DOWN/quit. A confirm
  becomes `1. Yes` / `2. No` and still accepts `yes` or `y` by name. Free-text questions
  (mail body, passwords) stay on `prompt`.

**Exit.** No `Atoi`-and-index menu block remains outside the package; admin builders behave
as before; tests cover number, name, Enter, out-of-range and a stale menu.

**Status:** not started.

## Phase 1: One talk command

**Goal.** `talk <npc>` is the only way to converse, through the menu.

**Approach.**
- Introduce the topic structure. Adapt today's patterns and tree nodes into topics at load
  time; a missing label falls back to the first trigger word, a missing key to a derived one
  (the validator lists every fallback so phase 7 can clear them).
- Menu content: eligible topics in authored order, quest topics first. Hidden quest topics
  show their breadcrumb if they have one.
- Heard set on the character (new saved field, through `dogmud-persistence` rules). Writes
  on hearing a `marked` info topic.
- Markers: computed per viewer, shown in the room's mob list and on menu lines; only when the
  NPC is visible by name.
- Observers: "X asks Kael about <label>" plus the NPC's spoken line; accept, decline and
  reward lines are private.
- `give` of an asked-for item resolves as choosing that topic, with the same output.
- `ask` becomes a redirect line. The tree's free-text triggers stop being the entry point.
- LLM path: generated replies are stored as `source: llm` topics on that NPC and then served
  like any other.
- Dialogue validators take a proposed world (loaded world plus pending edits), per the
  approach section, so phase 6's whole-world save check needs no rework.
- Start the topic-key manifest and its guard test here.

**Exit.** Every NPC with dialogue answers `talk` with a menu; heard state survives logout and
restart; markers clear and return correctly; `ask` redirects.

**Spec questions.** Where LLM topics are stored; how tree nodes that unlock other nodes
(`UnlockedNodes`) map onto topics.

**Status:** not started.

## Phase 2: Quest offer, hand-in and the closed loop

**Goal.** Every quest is offered, accepted, finished and chained through the menu.

**Approach.**
- Offer card: pitch, objective (current step's description), reward summary in words where
  the player-copy rules require it, then Accept / Leave.
- Finish line on the giver for a quest at its hand-in step; completion pays out as today and
  then offers the chain's next quest in the same menu.
- Chain declaration on the quest (next quest id), so the hand-in offer and the validator
  share one source.
- Eligibility: gates hide the quest topic; breadcrumb shows instead.
- Validators (warning list now, hard failure after phase 7): start and end at an NPC topic;
  every step has a hint; every step has exactly one progression trigger, optionally preceded
  by a location requirement; no trigger grants a quest.
- Repeatable: add a repeat limit and a saved completion count next to the existing
  `cooldown_rounds` cooldown.
- Quest validators take a proposed world too, and check the loop across files: the quest and
  its givers' offer and hand-in topics validate as one unit (phase 6's linked save).
- Start the step-id manifest and its guard test here, and the headless completability
  walkthrough (warning list until phase 7 clears it).

**Exit.** A test quest goes offer, accept, steps, finish, next offer through the menu; the
validators report the current backlog; repeat limit enforced.

**Spec questions.** Whether repeat frequency stays in rounds or moves to game days or real
time; exactly what counts as "one action" for the validator.

**Status:** not started.

## Phase 3: Action triggers (engine half)

**Goal.** Steps progress on what the player does, not what they type.

**Approach.**
- New or widened events: cast (spell, target), explore (room or area entered), acquire
  (item gained by any route), talk (topic heard), observe (a world event fired in view),
  use (command or skill used, with its target). Map each onto the existing events where they
  already fire (`skill_use`, `item_gain`, `room_enter`, `command_issued`) before adding new
  ones.
- Every movement path notifies the engine, including flee (#588).
- Keep `room_interact` loading until phase 7 removes its last use, then delete it.

**Exit.** Each new event has a test proving it fires on the real action and not on a
look-alike; #588 closed.

**Status:** not started.

## Phase 4: Minimap guidance (engine half)

**Goal.** The map always points somewhere useful.

**Approach.**
- Target kinds on a step: room, NPC (current room, as today), zone, area (a named set of
  rooms). Areas are defined once and reused across quests.
- Zone and area targets: the arrow paths to the nearest room of the set; the map highlights
  the set or labels "somewhere in this zone".
- "Destination unknown": explicit, logged as a warning at boot, listed by the validator.
- GMCP and the web client minimap carry the new target shapes.

**Exit.** Each target kind renders in the web client; boot lists every unknown destination.

**Status:** not started.

## Phase 5: Quest log actions (engine half)

**Goal.** Players control their log.

**Approach.**
- `quest abandon <quest>` and `quest focus <quest>` commands, and right-click Abandon / Focus
  in the web client quest panel. Reuse what exists: a left-click on a quest row already sends
  `Char.Quests.Focus` (`webclient-pure.html:986`, handled in `modules/gmcp/gmcp.CharOp.go:129`),
  and the inventory and automation panels already have right-click menus
  (`webclient-pure.html:3514,3672`) to copy for the quest panel. This panel is the only
  clicking the arc adds.
- Abandon hides the quest from the active log, keeps its progress token, and restores the
  giver's `[!]`; re-accepting resumes at the saved step. Mid-quest rewards already granted are
  not granted again.
- Focus sets which quest the minimap and tracker follow.

**Exit.** Abandon then re-accept resumes at the same step with no repeat rewards; focus drives
the minimap.

**Status:** not started.

## Phase 7: Content migration, zone by zone

**Goal.** Every zone meets the requirements page.

**Each zone pass.**
1. Port dialogue to keyed, labelled topics; decide `[*]` flags; add breadcrumbs.
2. Rewrite quests to atomic, hinted steps on action triggers, with map targets; chain them;
   add bridge quests between givers.
3. Move any trigger-started quest to an NPC start.
4. Clear dashes from the zone's dialogue and quest text.
5. Clear the zone from every validator warning list.
6. Adversarial playtest; file findings.

**Zone order.** Pothole Coulee first (new players land there). The rest is set in the phase 7
spec.

**Named items.**
- Pothole Coulee: one quest at a time, chained, heavy hand-holding; the light and
  skullduggery arm (#315); the portal quest becomes an info topic.
- Pinnacle quests 79 to 87 become recipe sales gated on crafting threshold; Veyra's gold check
  (quest 80) goes with them.
- Quest 76 redesign: the disc leaves the newbie zone (#625).
- Quests 15, 16, 28, 30, 31 move to NPC starts; quest 21 becomes a flag.
- Bugs: #613 (quest 11 soft-lock, live on prod), #616 (Kael), #290 (Marek's duplicate
  notice), the #611 grant-first ordering findings, Soren's unreachable `give_crate`, the six
  17-letter gates in Ashwick 259/260.

**Closing step.** Validators become hard boot failures; `room_interact` is deleted; the dash
guards stop exempting dialogue, quests and conversations.

**Status:** not started.

## Phase 6: Web builder (last)

**Goal.** An admin can author to these rules without reading this page, and the builder
cannot save a malformed quest or dialogue.

**Already in place (verified 2026-10-20).** Both editors validate server-side before saving:
dialogue runs `ValidateDialogueFile` with `RegistryValidators()`
(`modules/gmcp/gmcp.Dialogue.go:76`); quests run `Validate()` and `ValidateQuestRefs`
(`modules/gmcp/gmcp.Quest.go:223,226`). Errors block the save; warnings are shown.

**Approach: cannot save it malformed.**
1. Every rule lives in the shared validators, never only in client JavaScript. The server is
   the authority.
2. Whole-world validation on save: the edit is checked against the world as it would be after
   the save, because a closed loop spans a quest file and its givers' dialogue files.
3. Drafts: a failing quest or topic can be saved as a draft that never loads into play; only a
   passing draft can be published.
4. Linked saves: a quest and its giver's offer and hand-in topics publish together or not at
   all.
5. Topic keys and step ids lock after first publish; changing one needs an explicit migration,
   which the stable-id manifest guard checks.
6. Deleting anything referenced is refused, with the list of what references it.
7. Version history and undo on every save.

**Approach: helped not to build it wrong.**
8. Pickers for mobs, items, rooms, quests, spells and areas; a dropdown for trigger events; no
   raw YAML editing.
9. A "New quest" wizard that creates the giver's offer topic, a first step with its hint slot,
   and the hand-in topic, so the loop exists from the first click.
10. A live rule checklist beside the editor (starts and ends at an NPC, every step hinted, one
    action per step, chain target exists).
11. A "talk as player" preview of the menu and markers for a chosen player state: new,
    ineligible, mid-quest, ready to hand in.
12. The completability walkthrough on demand, the same code as the CI test.
13. Mouseover hints on every field, plus click-through cross-references between quests,
    rooms, zones, items and mobs, and a topic editor that makes `key`, `label`, `marked` and
    breadcrumbs obvious.

**Exit.** Every malformed fixture in the requirements' fixture suite is refused by the builder;
a builder round trip loads unchanged at boot; a new admin can build a chained quest with the
wizard and pass the checklist.

**Status:** not started.
