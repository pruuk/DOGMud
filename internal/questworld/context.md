# questworld: Package Documentation

## Purpose

`internal/questworld` builds the quest validators' inputs from the live
registries (quest arc #375 phase 2). `internal/quests` and
`internal/questloop` stay pure and take these inputs as arguments; this
package is the one place that reaches the item, recipe, spell, faction, skill,
condition, mob and room registries to fill them.

## Files

| File | What it holds |
|---|---|
| `names.go` | `Names() quests.RewardNames`: item, recipe, spell and faction display names, "" for an unknown id |
| `sources.go` | `Sources() questloop.Sources`: the walkthrough's item-source and spawn index, and its external grants || `questworld.go` | `Validators()`, `Registry()`, `RegistryWith(v)`, `WithQuest(w, q)` with its step and flag overlays, `BacklogPath()`, `LoadBacklog()` |

## API

- `Names()`: backs the offer card's reward line (`quests.RewardSummary`) and
  the `reward` loop rule.
- `Validators() quests.QuestValidators`: the existence checks
  `quests.ValidateQuestRefs` runs. Moved here from `modules/gmcp`, which now
  calls it for the editor's reference check.
- `Registry() questloop.World`: every loaded quest, every dialogue file
  (`dialogue.LoadAllFiles`; their own problems are the dialogue door's), the
  live `Validators()` and `Names()`, and `External`: every behaviour tree
  `grant_quest` action (mob, room, archetype and item trees, via
  `behaviortree.ListTreeFiles` and `behaviortree.QuestGrantsInFile`, named
  "mob 9100 tree grant_quest 30-rite") and every item `questtoken` ("item
  40035 questtoken"), sorted by name (`externalGrants` in `sources.go`). It is
  `RegistryWith(Validators())`. `quest_loop_smoke_test.go`
  (`TestSmoke_WalkSourcesIndexShippedWorld`) checks `External` against
  shipped data.
- `RegistryWith(v)`: the same world with reference checks the caller already
  built; the quest editor passes its own so they are built once per save.
- `WithQuest(w, q)`: `w` with `q` swapped in for the quest that has its id, or
  added. The proposed world an editor save is checked against. `w` is not
  changed; its dialogue, names and `External` carry over. Its `Refs` is a copy whose `StepExists` and `FlagDeclared` answer
  for `q`'s own tokens and flags (`"<q.QuestId>-..."`) from `q` itself, the
  way `quests.GetQuest` and `quests.ValidateFlag` would after the save, and
  pass everything else to `w`'s checks. Without the overlay the refs rule
  judges other quests against the loaded steps, so a step rename that breaks
  another quest's trigger saves clean and the next boot refuses.
- `Sources() questloop.Sources`: indexes once, from the loaded registries, where
  each item can come from and which mobs are placed (spec 7.2). Item sources:
  room spawninfo, floor items, containers and their recipes; quest `give_item`,
  `spawn_item` and rewards; topics' `givesItem`; forage tables; crafting
  outputs; and, for a SPAWNED mob only, its carried and worn items, loot pool
  and shop stock. A mob is spawned when a room's spawninfo or a quest's
  `spawn_mob` action places it. Each source records the quests that own it
  (`owner`: their own grants, a topic of or gated on the quest, a mob only
  that quest spawns), which backs `ItemAvailable`. A topic's owner entry
  carries the tokens of that quest its `questRequired` names and the token it
  grants (`topicOwners`): the source opens to that quest once the player holds
  them all (a topic of quest 70 gated on 63-start is open to quest 63 from
  start on; quest 20's `vellahandoff`, which grants 20-vella_journal with the
  journal, is no source before that step). Behaviour-tree mob spawns are not
  indexed. The external grants are on the loop world (`Registry`), not here.
  Call it after the registries are loaded; `quest_loop_smoke_test.go`
  (`TestSmoke_WalkSourcesIndexShippedWorld`) checks it against shipped data.
- `BacklogPath()` and `LoadBacklog()`: the shipped `quests/loop-backlog.txt`; a
  missing file is an empty backlog.

## Callers

- `internal/conversation` (`offerCard` in `talk.go`, for the card body): `Names()`.
- `main_quests.go`: `Registry()`, `LoadBacklog()`, `BacklogPath()`.
- `modules/gmcp/gmcp.Quest.go`: `Validators()`, `RegistryWith`, `WithQuest`,
  `LoadBacklog`. Its save refuses only the findings the edit adds (see
  `internal/questloop/context.md`, Doors).
- `quest_loop_smoke_test.go`: the CI sweep.

## Gotchas

- Registry reads only: load quests, dialogue, mobs, items, spells, recipes and
  factions first, or the world is empty and every rule passes for the wrong
  reason.
- Import direction: this package imports `questloop`, never the reverse, and
  nothing it imports reaches `conversation`.
- A new `QuestValidators` callback that reads quest data (steps, flags) needs
  the same overlay in `WithQuest`, or the editor judges against stale data.
  Today only `StepExists` and `FlagDeclared` read quests; `DialogueGrants`
  and `MobHasDialogue` read dialogue, which an editor save does not change.
