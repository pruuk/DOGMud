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
| `questworld.go` | `Validators()`, `Registry()`, `WithQuest(w, q)`, `BacklogPath()`, `LoadBacklog()` |

## API

- `Names()`: backs the offer card's reward line (`quests.RewardSummary`) and
  the `reward` loop rule.
- `Validators() quests.QuestValidators`: the existence checks
  `quests.ValidateQuestRefs` runs. Moved here from `modules/gmcp`, which now
  calls it for the editor's reference check.
- `Registry() questloop.World`: every loaded quest, every dialogue file
  (`dialogue.LoadAllFiles`; their own problems are the dialogue door's), the
  live `Validators()` and `Names()`.
- `WithQuest(w, q)`: `w` with `q` swapped in for the quest that has its id, or
  added. The proposed world an editor save is checked against.
- `BacklogPath()` and `LoadBacklog()`: the shipped `quests/loop-backlog.txt`; a
  missing file is an empty backlog.

## Callers

- `internal/conversation` (`offerCard` in `talk.go`, for the card body): `Names()`.
- `main_quests.go`: `Registry()`, `LoadBacklog()`, `BacklogPath()`.
- `modules/gmcp/gmcp.Quest.go`: `Validators()`, `WithQuest`, `Registry()`,
  `LoadBacklog`.
- `quest_loop_smoke_test.go`: the CI sweep.

## Gotchas

- Registry reads only: load quests, dialogue, mobs, items, spells, recipes and
  factions first, or the world is empty and every rule passes for the wrong
  reason.
- Import direction: this package imports `questloop`, never the reverse, and
  nothing it imports reaches `conversation`.
