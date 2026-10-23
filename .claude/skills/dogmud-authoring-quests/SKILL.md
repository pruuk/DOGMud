---
name: dogmud-authoring-quests
description: Use when writing or editing a quest, a dialogue file, or any NPC text. Covers the two ways a dialogue file used to mute its NPC (both now refused at boot), the questExcluded end-token rule that stops a completed quest being re-offered, first-person NPC text versus narrator hints, quest flags and branching-quest gating, and the give.go trap where an item transfers before any handler can refuse it.
---

This skill covers authoring DOGMud quests, dialogue files, and NPC text.
Since quest arc #375 phase 1 a dialogue file is an opener plus a menu of
keyed topics that `talk <npc>` shows; the old keyword patterns and
conversation tree are deleted. The skill covers the two old silent-failure
modes (now refused at boot), quest re-grant prevention, discoverability,
voice, the give.go item-transfer trap, quest flags and branching, and the
quest engine's event-name vocabulary.

## Two ways a dialogue file used to mute an NPC

Both of these once failed silently, with no boot panic and the NPC simply
mute. Both are now refused at boot (the dialogue door in `main_dialogue.go`
panics on any problem) and by the CI sweep
`TestSmoke_ShippedDialoguePassesTheValidators`. Run the boot smoke tests
before pushing dialogue changes.

**Filename convention.** Dialogue YAML files must be named `<mobId>.yaml`
only, not `<mobId>-<name>.yaml`, and sit in the folder of the mob's zone.
Mob and behavior-tree files use the `<mobId>-<name>.yaml` convention, which
makes it intuitive (and wrong) to assume dialogue follows the same pattern.
Caught 2026-04-30 when all three foragers had dialogue named `371-vella.yaml`
etc and were mute. The loader now reports a file name that is not a mob id, a
`mobid:` that differs from the file name, and a file in the wrong zone
folder. [[feedback_dialogue_filename_convention]]

**Bare-scalar list fields.** A dialogue `[]string` field (`questRequired`,
`questExcluded`, `heardRequired`) written as a bare scalar instead of a
one-element list, for example `questRequired: "34-end"` instead of
`questRequired: ["34-end"]`, fails the whole file's decode. That once
silenced 7 quest-giver NPCs in production undetected. Decoding is now strict
(an unknown key fails too, so a typo or a leftover `patterns:` cannot be
ignored) and the boot door reports the file.
[[feedback_dialogue_bare_scalar_list_mutes_npc]]

## Gating fields at a glance

Three different things gate whether a topic is on the menu, and two of their
names differ by one word. Do not reach for one when you mean the other.

- `questExcluded`: a list of quest TOKENS, e.g. `["10-start", "10-end"]`.
  Hides a topic if the player holds one of these tokens (used for quest
  re-grant prevention, immediately below).
- `questFlagExcluded`: a map of flag KEY to VALUE, e.g.
  `{"11-branch": "sylara"}`. Hides a topic if a quest flag equals that value
  (used for branching-quest gating, in Quest flags and branching below).
- `heardRequired`: a list of info topic KEYS on the same NPC that the player
  must have heard first. It replaces the old tree's `requires`/`unlocks`, and
  what a player has heard is saved on the character, so it never expires.

## Re-grant prevention

### Quest Re-Grant Prevention SOP
Every topic with `grantsQuest` must include the quest's **end token** (e.g.,
`{questid}-end`) in `questExcluded`, not just the token being granted.
Without this, a player who completed the quest can get it re-offered.
Example: `grantsQuest: "10-start"` requires
`questExcluded: ["10-start", "10-end"]`. `dialogue.ValidateWorld` refuses
either omission, at boot and in CI.

The end-token requirement is the part that gets missed: it is easy to
exclude the token you just granted (`10-start`) and forget the token the
quest ends on (`10-end`). Both must be present in `questExcluded`, or a
player who finished the quest can walk back up and get offered it again.

## Discoverability

### Quest NPC Dialogue SOP
There are no trigger words to discover: `talk <npc>` lists every topic the
player may choose. A quest offer is a topic of `kind: offer` whose `label` is
the quest's title and whose `grantsQuest` is the quest's start token; a
hand-in is `kind: handin`, granting the end token. `ValidateWorld` checks
both. The roster marks an NPC with an open offer or hand-in.

### Dialogue Voice & Discoverability
- NPC `text` fields are spoken by the NPC, always first person ("I", "my", "me").
- `hints` are narrator text for the player, describe options from the player's
  perspective. **NEVER** write 3rd-person self-references like "Ask about why she
  left" when "she" is the speaking NPC. Write "You could ask why she left" or
  "You could ask about the marriage."
- A topic hidden behind gates the player cannot see should leave a trail:
  an offer can carry a `breadcrumb` line the NPC says while the offer is
  closed, and an opener variant or hint can point at what unlocks it.

Provenance note: an earlier implementation rendered hints via
`mob.Command("say ...")`, which caused exactly the NPC self-reference
problem the rule above forbids; hints now use a separate `SendText` path.
[[feedback_hint_voice]]

Discoverability extends past dialogue into room content: quest engine
`room_interact` triggers do an exact string match on the player's typed
noun, with no fuzzy matching or alias resolution (that forgiveness happens
in the room's `FindNoun`/`FindHiddenNoun`, which runs after the trigger has
already fired on the raw noun). Author hidden_noun keys as single intuitive
words (`carving`, not `bench-vise carving`), put a `` Try `look <key>`. ``
hint at the end of the hidden_description, and write one `room_interact`
trigger per plausible noun variant a player might type (`altar stone`,
`altar`, and `stone` as three separate triggers with the same conditions and
actions). Cross-reference room nouns against trigger nouns before shipping.
[[feedback_room_interact_noun_matching]]

There is no space/hyphen normalization on either side, which is the form
this bites in most often. `usercommands.go` notifies the engine with
`Noun: strings.ToLower(rest)` and `questengine/engine.go` compares
`t.Noun != d.Noun` as an exact string, and the shipped `noun:` spellings are
mixed: hyphenated (`old-guild-mark`, `hearth-marks`, `paper-scrap`) and
spaced (`gray material`, `buried marker`). Quest 68's trigger noun is
`loose-floorboard` while room 5720 lists the noun `loose floorboard`, so
`look loose floorboard` prints the room text and only `look loose-floorboard`
fires the trigger, which no player can guess. When a quest interaction "does
nothing," diff the room noun spelling against the trigger noun before
suspecting the step gate. Normalizing both sides in the engine is filed as an
owner call on the quest mechanisms arc, not a fix to make in passing.
[[project-quest-room-interact-noun-hyphen-trap]]

## Items

### Quest Item Delivery: give.go Gotcha
Since #375 (R7, R8) an item hand-in is a dialogue topic, never a quest
trigger: an `item_give` trigger may not carry a `grant` (the boot validators
refuse it). `give <item> <npc>` asks the NPC's topics first, BEFORE any
transfer:
- A topic wanting the item (`requiresItem`, taken; or `carriesItem`, shown
  and kept) that is eligible plays exactly as if chosen from the talk menu,
  and takes the instance the player named (`give 2.purse`).
- A topic wants it but none is eligible: the item never leaves the player.
  The mob's tree may say why: a `player_give` branch with an `item_matches`
  condition for that item, using only speech and `return_item`, plays as a
  hand-back (Arn's "See Drunn first, then come back."). It can credit
  nothing. Otherwise the player reads "<npc> looks at the <item> and hands
  it back."
- No topic wants it: the old give. `give.go` transfers the item to the mob
  BEFORE the tree's `player_give` fires, so a handler can only react after
  the fact, typically by giving it back with `return_item`.
- Quest givers who hand out physical items via `givesItem` must also have a
  recovery topic that gives a replacement if the player lost the item

### Dialogue Engine: givesItem
Topics support `givesItem: <itemId>`. When a topic with `givesItem` is
chosen, the player receives the item and sees "You receive a <itemname>."
Use this for NPCs handing quest items to the player during dialogue. If the
item cannot be carried, nothing else the topic does applies, so it can be
chosen again once the player makes room.

For an item no topic wants this is a hard constraint: by the time any
handler runs, the item is already gone from the player. A handler cannot
inspect the item and decide to refuse it, it can only react after the fact
(typically by giving it back with `return_item`). To refuse a quest item
before it moves, make a topic want it.

Two more item rules for quests:

- Quest delivery items (anything referenced by `requiresItem` in dialogue,
  or checked by an `item_give` trigger) must never have `is_component: true`.
  That flag auto-routes the item to the component pouch on pickup, and both
  the `give` command and dialogue `requiresItem` check only search the
  backpack, not the pouch, breaking the delivery.
  [[feedback_quest_items_not_components]]
- Loot must never be placed directly on the ground via `spawninfo`. Use mob
  drops (primary), locked chests or containers (secondary, gated by keys or
  lockpicking), or quest rewards (for special/unique items). Flavor nouns
  that aren't items are fine. [[feedback_loot_placement]]

## Quest flags and branching

### Quest Flags System
Quest flags store arbitrary metadata about quest choices. Primary use case:
tracking which branch a player took in an opposed/branching quest.

#### Flag Declaration (Quest YAML)
Quests declare expected flags with allowed values. **Undeclared flag
references cause a server panic at startup**, catching typos before
they reach production.

```yaml
flags:
  - key: branch
    values: [sylara, rhett]
    description: "Which NPC the player sided with"
```

Flag key convention: `"{questId}-{flagName}"` (e.g., `"11-branch"`).

#### Dialogue Integration
- `setsQuestFlag: {key: "11-branch", value: "rhett"}`: set a flag when
  the topic is chosen
- `questFlagRequired: {"11-branch": "rhett"}`: gate on flag value
- `questFlagExcluded: {"11-branch": "sylara"}`: hide if flag matches

#### Quest Engine Integration
- Conditions: `has_flag: {"11-branch": "rhett"}`, `missing_flag: ...`
- Action: `set_flag: {key: "11-branch", value: "rhett"}`

#### Admin/Scripting
- `questtoken flags`: show all flags on your character
- `questtoken flag <key> [value]`: view or set a flag
- JS scripting: `user.GetQuestFlag(key)`, `user.SetQuestFlag(key, value)`,
  `user.HasQuestFlag(key)`

#### Branching Quest SOP
Every branching quest MUST have:
1. Flag declaration in quest YAML with all valid values
2. `setsQuestFlag` on each branch NPC's quest-start topic
3. `questFlagRequired` on followup quest offers to gate by branch
4. `questFlagExcluded` (or `questFlagRequired`) on every branch topic, so a
   wrong-path player's menu never shows the other branch. The menu lists
   only eligible topics, so no ordering or dismissal topic is needed.
5. Opener variants with `questFlagRequired` for path-specific openers
6. Mid-quest opener variants for cross-NPC visits during the OTHER quest

### Quest reward YAML keys

Quest `rewards:` fields (`itemid`, `skillinfo`, `conditionid`, `roomid`,
`spellid`, `questid`) load via a tag-less struct
that binds on the lowercased field name with no underscore handling, so
`itemid` is correct and `item_id` silently fails to load with no panic and
no warning. This is scoped to the rewards block only: trigger
`actions:`/`conditions:` and the dialogue fields used elsewhere in this
document (`grantsQuest`, `setsQuestFlag`, `questExcluded`, `givesItem`) are
properly snake_case-tagged structs and are correct as written throughout
this skill; only the rewards block is the no-underscore exception.
[[reference_quest_reward_yaml_key_gotcha]]

The reward block's two narrated lines are NOT in that no-underscore set: they
are `actor` (the completing player's line) and `observer` (the room's), both
explicitly tagged. M4b-1 renamed them from `playermessage` and `roommessage`,
and renamed the two narrating trigger actions from `send_text` and `room_text`
to the same two words, so one vocabulary covers both blocks.

## Verify event names against the loader

Quest YAML triggers panic at startup if the `event:` name is not in the
quest engine's hardcoded `validEvents` map (`internal/questengine/loader.go`,
around line 39-43). The behavior tree event vocabulary
(`mob_die`, `mob_hurt`, `mob_idle`, `player_topic`, `player_give`,
`player_enter`, `mob_flee`) is a separate system and does not overlap with
quest engine event names. Do not guess an event name from the behavior tree
vocabulary or from what sounds plausible: `mob_killed` and `mob_die` both
sound right for a mob-death trigger, and the actual quest engine event is
`mob_death`. Before writing a trigger, grep `validEvents` in
`internal/questengine/loader.go` for the current canonical list, or copy the
spelling from an existing quest YAML that uses the event you want.
[[feedback_quest_engine_event_names]]

## Sources

Lifted verbatim from CLAUDE.md:
- "Quest Re-Grant Prevention SOP" (lines 786-793)
- "Quest NPC Dialogue SOP" (lines 794-799)
- "Dialogue Voice & Trigger Discoverability" (lines 800-813)
- "Quest Item Delivery: give.go Gotcha" (lines 814-825)
- "Dialogue Engine: givesItem" (lines 826-830)
- "Quest Flags System" (lines 831-875)

Folded memory files:
- [[feedback_dialogue_filename_convention]]
- [[feedback_dialogue_bare_scalar_list_mutes_npc]]
- [[feedback_hint_voice]]
- [[feedback_quest_engine_event_names]]
- [[feedback_quest_items_not_components]]
- [[feedback_loot_placement]]
- [[feedback_room_interact_noun_matching]]
- [[project-quest-room-interact-noun-hyphen-trap]] (the space/hyphen half of
  the same exact-match rule; its engine-normalization proposal belongs to
  [[project-quest-mechanisms-arc]])
- [[reference_quest_reward_yaml_key_gotcha]]
