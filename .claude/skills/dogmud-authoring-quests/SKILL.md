---
name: dogmud-authoring-quests
description: Use when writing or editing a quest, a dialogue tree, or any NPC text. Covers the two ways a dialogue file silently mutes its NPC, the questExcluded end-token rule that stops a completed quest being re-offered, first-person NPC text versus narrator hints, quest flags and branching-quest gating, and the give.go trap where an item transfers before any handler can refuse it.
---

This skill covers authoring DOGMud quests, dialogue trees, and NPC text. It
leads with the two silent-failure modes that mute an NPC with no boot panic
and no error, because those are the most expensive mistakes in this document
to debug. It then covers quest re-grant prevention, discoverability, voice,
the give.go item-transfer trap, quest flags and branching, and the quest
engine's event-name vocabulary.

## Two ways to mute an NPC

Both of these fail silently: no boot panic, no runtime error, the NPC simply
stops responding to any keyword. The pre-push boot test cannot catch either
one because dialogue loads lazily, per mob and zone, on first interaction,
not at startup.

**Filename convention.** Dialogue YAML files must be named `<mobId>.yaml`
only, not `<mobId>-<name>.yaml`. Mob and behavior-tree files use the
`<mobId>-<name>.yaml` convention, which makes it intuitive (and wrong) to
assume dialogue follows the same pattern. A `<mobId>-<name>.yaml` dialogue
file loads with no panic and leaves the NPC silently mute. Caught 2026-04-30
when all three foragers had dialogue named `371-vella.yaml` etc: the loader
saw no dialogue file and every keyword got no response. Verification:
`ls _datafiles/world/dogmud/dialogue/<any>/`, every existing file is just
`<digits>.yaml`. [[feedback_dialogue_filename_convention]]

**Bare-scalar list fields.** A dialogue `[]string` field
(`questRequired`, `questExcluded`, `triggers`, `keywords`) written as a bare
scalar instead of a one-element list, for example `questRequired: "34-end"`
instead of `questRequired: ["34-end"]`, causes `yaml.v2` to error on
unmarshal. The loader treats any unmarshal error as fatal for the whole
file, logs it, and permanently caches nil for that mob. The NPC goes
completely mute, every node and pattern, not just the offending node. One
copy-paste slip silenced 7 quest-giver NPCs in production undetected. Sweep
grep before shipping dialogue changes:
`grep -rnE '^\s*(questRequired|questExcluded):\s*["'\'']' <dialogue-dir>`
(the list form has `[` after the colon, the bare-scalar form has a quote).
[[feedback_dialogue_bare_scalar_list_mutes_npc]]

## Gating fields at a glance

Three different things gate whether a dialogue node or pattern fires, and
two of their names differ by one word. Do not reach for one when you mean
the other.

- `questExcluded`: a list of quest TOKENS, e.g. `["10-start", "10-end"]`.
  Hides a node if the player holds one of these tokens (used for quest
  re-grant prevention, immediately below).
- `questFlagExcluded`: a map of flag KEY to VALUE, e.g.
  `{"11-branch": "sylara"}`. Hides a node if a quest flag equals that value
  (used for branching-quest gating, in Quest flags and branching below).
- `triggers` / `keywords`: the words a player must type to reach a node at
  all. This gates on player input, not on quest state (see Discoverability
  below).

## Re-grant prevention

### Quest Re-Grant Prevention SOP
Every dialogue node or pattern with `grantsQuest` must include the quest's
**end token** (e.g., `{questid}-end`) in `questExcluded`, not just the token
being granted. Without this, a player who completed the quest can get it
re-offered. Example: `grantsQuest: "10-start"` requires
`questExcluded: ["10-start", "10-end"]`. The dialogue loader logs a warning
at runtime if this exclusion is missing.

The end-token requirement is the part that gets missed: it is easy to
exclude the token you just granted (`10-start`) and forget the token the
quest ends on (`10-end`). Both must be present in `questExcluded`, or a
player who finished the quest can walk back up and get offered it again.

## Discoverability

### Quest NPC Dialogue SOP
Every quest-granting dialogue node (any tree node with `grantsQuest`) MUST include
`"quest"` and `"task"` in its `triggers` list. Similarly, quest-introducing
`patterns` entries must include `"quest"` and `"task"` in `keywords`. This ensures
`ask <npcname> quest` always works for discovering available quests.

### Dialogue Voice & Trigger Discoverability
- NPC `text` fields are spoken by the NPC, always first person ("I", "my", "me").
- `hints` are narrator text for the player, describe options from the player's
  perspective. **NEVER** write 3rd-person self-references like "Ask about why she
  left" when "she" is the speaking NPC. Write "You could ask why she left" or
  "You could ask about the marriage."
- Every trigger word MUST be discoverable, it must appear in a hint, NPC text,
  room description, or quest log. Undiscoverable triggers are broken triggers.
- Prefer `questRequired` over `requires` for quest-gated nodes. `requires` depends
  on per-player memory that can expire and brick quests.
- `expiryPeriod` should almost never be set. The ONLY valid use is quests
  where urgency is the design intent (e.g., timed delivery before an attack).
  For all other NPCs, leave it empty or omit entirely.

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

## Items

### Quest Item Delivery: give.go Gotcha
**CRITICAL:** `give.go` transfers the item from the player to the mob BEFORE
any handler fires. The handler cannot prevent or undo the transfer.
Consequences:
- Quest item delivery is handled by the quest engine's `item_give` triggers
  (in quest YAML) and/or behavior tree `player_give` handlers on the mob
- NPCs that should NOT keep the item (e.g., the quest giver who handed it
  out) need a behavior tree `player_give` handler that uses the `return_item`
  action to give the item back
- Quest givers who hand out physical items via `givesItem` must also have a
  recovery dialogue node that gives a replacement if the player lost the item

### Dialogue Engine: givesItem
Tree nodes and patterns support `givesItem: <itemId>`. When a node fires with
`givesItem` set, the player receives the item and sees "You receive a <itemname>."
Use this for NPCs handing quest items to the player during dialogue.

This is a hard constraint, not advice: by the time any handler runs, the
item is already gone from the player. A handler cannot inspect the item and
decide to refuse it, it can only react after the fact (typically by giving
it back with `return_item`).

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
- `setsQuestFlag: {key: "11-branch", value: "rhett"}`: set a flag on
  node match
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
2. `setsQuestFlag` on each branch NPC's quest-start dialogue node
3. `questFlagRequired` on followup quest offers to gate by branch
4. **Dismissal nodes** at the TOP of each NPC's tree nodes list for
   wrong-path players. Without these, keyword patterns fire and
   players think there's a hidden quest
5. Root variants with `questFlagRequired` for path-specific greetings
6. Mid-quest root variants for cross-NPC visits during the OTHER quest

### Quest reward YAML keys

Quest `rewards:` fields (`itemid`, `skillinfo`, `buffid`, `playermessage`,
`roommessage`, `roomid`, `spellid`, `questid`) load via a tag-less struct
that binds on the lowercased field name with no underscore handling, so
`itemid` is correct and `item_id` silently fails to load with no panic and
no warning. This is scoped to the rewards block only: trigger
`actions:`/`conditions:` and the dialogue fields used elsewhere in this
document (`grantsQuest`, `setsQuestFlag`, `questExcluded`, `givesItem`) are
properly snake_case-tagged structs and are correct as written throughout
this skill; only the rewards block is the no-underscore exception.
[[reference_quest_reward_yaml_key_gotcha]]

## Verify event names against the loader

Quest YAML triggers panic at startup if the `event:` name is not in the
quest engine's hardcoded `validEvents` map (`internal/questengine/loader.go`,
around line 39-43). The behavior tree event vocabulary
(`mob_die`, `mob_hurt`, `mob_idle`, `player_ask`, `player_give`,
`player_enter`, `mob_flee`) is a separate system and does not overlap with
quest engine event names. Do not guess an event name from the behavior tree
vocabulary or from what sounds plausible: `mob_killed` and `mob_die` both
sound right for a mob-death trigger, and the actual quest engine event is
`mob_death`. Before writing a trigger, grep `validEvents` in
`internal/questengine/loader.go` for the current canonical list, or copy the
spelling from an existing quest YAML that uses the event you want.
[[feedback_quest_engine_event_names]]

## Sources

- [[feedback_dialogue_filename_convention]]
- [[feedback_dialogue_bare_scalar_list_mutes_npc]]
- [[feedback_hint_voice]]
- [[feedback_quest_engine_event_names]]
- [[feedback_quest_items_not_components]]
- [[feedback_loot_placement]]
- [[feedback_room_interact_noun_matching]]
- [[reference_quest_reward_yaml_key_gotcha]]
