# questloop: Package Documentation

## Purpose

`internal/questloop` holds the quest loop rules (quest arc #375 phase 2, spec
section 5): the checks that every quest is a closed loop, offered by an NPC,
walked one action per step, and handed in. It is pure. It reads a proposed
`World` (quests, dialogue files, reference callbacks, reward names) and
returns `Finding`s; it opens no registry itself. `internal/questworld` builds
the world from the live registries, and three doors run it.

## Files

| File | What it holds |
|---|---|
| `questloop.go` | `World`, `Finding` (`Key`, `String`), `Validate(w)`: every rule, findings sorted by key |
| `grants.go` | The grant index: which topics and triggers grant each step token, as `way`s (`indexGrants`, `walkGrants`, `ownGrants`) |
| `rules_loop.go` | `ruleStart`, `ruleEnd`, `ruleHint` |
| `rule_oneaction.go` | `ruleOneAction` and `dedupe` |
| `rules_more.go` | `ruleChain`, `ruleRepeat`, `ruleReward`, `ruleRefs` |
| `backlog.go` | `Backlog`, `ParseBacklog`, `FormatBacklog`, `Ratchet` |

## Rules

| Rule id | Check |
|---|---|
| `start` | The first step's token is granted by at least one `offer` topic, and by nothing else |
| `end` | The final step's token is granted only by `handin` topics, and by at least one |
| `hint` | Every step except the final one has a non-empty `hint` |
| `one-action` | Every non-start step has at least one way in; each way is one trigger or one topic that requires the previous step's token, with no item, flag or gold condition and no `sequence`; a trigger grants at most one step of its own quest |
| `chain` | An offer gated on quest N's final token sits on a mob that has a `handin` topic for N |
| `repeat` | A repeatable has `cooldown_rounds` above zero; `repeat_limit` is refused on a non-repeatable and when negative |
| `reward` | An authored `rewards.summary` names every item, recipe and spell paid, unless `rewards.surprise` is set; `surprise` with no summary to show is also a finding |
| `refs` | Every refusal from `quests.ValidateQuestRefs`; its warnings stay warnings |

A one-step quest has one token that is both first and final, so it cannot pass
both `start` and `end`. That is intended: a closed loop needs an offer and a
separate hand-in.

## Finding.Key and the backlog

`Finding.Key()` is `"rule questId where"` and is the backlog line. `Msg` is
free to reword without touching the backlog. The committed backlog is
`_datafiles/world/dogmud/quests/loop-backlog.txt` (`ParseBacklog` reads it,
`FormatBacklog` writes it, sorted, with the header comment).

`Ratchet(findings, backlog)` returns `fresh`, `known` and `stale`:

- **fresh** (not listed) refuses: boot fails, the CI sweep fails, the editor
  save is refused.
- **known** (listed) is a warning.
- **stale** (a listed line no finding matched) fails the CI sweep, so the list
  only shrinks. Phase 7 deletes lines as each zone pass clears them; an empty
  list means every rule is hard.

## Doors

| Door | Where |
|---|---|
| Boot | `validateShippedQuests` and `questDoor` in `main_quests.go` (panics at boot, returns an error on a reload; stale lines are CI's job) |
| CI | `TestSmoke_ShippedQuestsPassTheLoop` in `quest_loop_smoke_test.go`; setting `DOGMUD_LOOP_BACKLOG_WRITE` rewrites the backlog from today's findings |
| Editor save | `buildQuestUpdate` in `modules/gmcp/gmcp.Quest.go` validates `questworld.WithQuest(questworld.Registry(), q)` and ratchets it |

## Still to come

Plan 2b adds the completability walkthrough (spec section 7), a headless walk
of each quest from a fresh character with item-source checks. It is not
written yet; its rules will join this package.

## Gotchas

- Never import `conversation` or `questworld` here: the package must stay
  importable from both.
- A nil callback in `World.Refs` or `World.Names` skips that check; production
  worlds always fill them.
