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
| `grants.go` | The grant index: which topics and triggers grant each step token, as `way`s (`indexGrants`, `walkGrants`, `ownGrants`), and the stable trigger names (`triggerWheres`, `triggerSignature`) |
| `rules_loop.go` | `ruleStart`, `ruleEnd`, `ruleHint`, `ruleHintLength` and its cap `HintMaxRunes` |
| `rule_oneaction.go` | `ruleOneAction` and `dedupe` |
| `rules_more.go` | `ruleChain`, `ruleRepeat`, `ruleReward`, `ruleRefs`, `refsText` |
| `backlog.go` | `Backlog`, `ParseBacklog`, `FormatBacklog`, `Ratchet` |

## Rules

| Rule id | Check |
|---|---|
| `start` | The first step's token is granted by at least one `offer` topic, and by nothing else |
| `end` | The final step's token is granted only by `handin` topics, and by at least one |
| `hint` | Every step except the final one has a non-empty `hint` |
| `hint-length` | No step's `hint` is over `HintMaxRunes` (200, owner ruling P8): whitespace runs collapse to one space, then runes are counted. Where is `step <id>`. A step with no hint is the `hint` rule's finding. The web quest panel's "more" toggle uses the same 200 |
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
free to reword without touching the backlog (refs aside: its key carries the
refusal text). The committed backlog is
`_datafiles/world/dogmud/quests/loop-backlog.txt` (`ParseBacklog` reads it,
`FormatBacklog` writes it, sorted, with the header comment).

`Where` follows two rules. It holds no positions, so inserting a step or a
trigger moves no other key. And it names the defect class, so two different
defects never share a key and a listed one cannot hide a new one.

| Where | Names |
|---|---|
| `step firstkill` | a step, by id |
| `quest 53 trigger mob_death mob=9110 grants=53-firstkill` | a trigger, by signature: the event, its non-zero filters (room, mob, item, skill, command, quest_token, noun, verb in that order) and the tokens it grants, sorted, sequences included; triggers of one quest that share a signature take ` #2`, ` #3` in file order |
| `quest 53 rewards questid` | the completion reward's grant |
| `quest 53 rewards summary` | the reward override (`reward` rule) |
| `mob 9112 topic cull_in` | a dialogue topic |
| `quest 53:cooldown`, `:negative`, `:notrepeatable` | the three `repeat` sub-rules |
| `quest 53 refs: <refusal>` | one `refs` refusal, trimmed to one line, its leading `trigger N` rewritten to the trigger's signature and `step N (id)` to `step id` (`refsText`); action indexes inside a trigger stay |

`one-action` appends a reason to the way it names: `:none` (nothing moves a
player onto the step), `:reward`, `:prev` (the previous step is not
required), `:cond` (an item, flag or gold condition), `:seq` (granted inside a
sequence) or `:double` (one trigger grants two steps of its quest). One way
can carry several reasons, each its own key. `dedupe` drops only repeats of
one key.

Changing a key format changes every backlog line it covers: regenerate the
backlog with `DOGMUD_LOOP_BACKLOG_WRITE` into a scratch path, check the
per-rule counts against the old file, then copy it into place.

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
| Boot | `validateShippedQuests` and `questDoor` (over `questDoorFrom`, which takes the finding source so tests can feed it) in `main_quests.go`: panics at boot, returns an error on a reload; stale lines are CI's job |
| CI | `TestSmoke_ShippedQuestsPassTheLoop` in `quest_loop_smoke_test.go`; setting `DOGMUD_LOOP_BACKLOG_WRITE` writes today's findings as backlog text to that path |
| Editor save | `buildQuestUpdate` in `modules/gmcp/gmcp.Quest.go` (`loopVerdict`) builds the registry world once (`questworld.RegistryWith(d.validators)`), validates it before and after `questworld.WithQuest(world, q)`, and refuses only findings the edit adds: in the after world, not in the before world, not in the backlog. It warns about the edited quest's own other findings (listed, or already there) and says nothing about other quests' standing findings |

## Still to come

Plan 2b adds the completability walkthrough (spec section 7), a headless walk
of each quest from a fresh character with item-source checks. It is not
written yet; its rules will join this package.

## Gotchas

- Never import `conversation` or `questworld` here: the package must stay
  importable from both.
- A nil callback in `World.Refs` or `World.Names` skips that check; production
  worlds always fill them.
