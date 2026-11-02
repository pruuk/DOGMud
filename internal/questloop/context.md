# questloop: Package Documentation

## Purpose

`internal/questloop` holds the quest loop rules (quest arc #375 phase 2, spec
section 5): the checks that every quest is a closed loop, offered by an NPC,
walked one action per step, and handed in. It is pure. It reads a proposed
`World` (quests, dialogue files, reference callbacks, reward names, external
grants) and
returns `Finding`s; it opens no registry itself. `internal/questworld` builds
the world from the live registries, and three doors run it.

## Files

| File | What it holds |
|---|---|
| `questloop.go` | `World`, `Finding` (`Key`, `String`), `Validate(w)`: every rule, findings sorted by key |
| `grants.go` | The grant index: which topics, triggers, completion rewards and external grants (`World.External`) grant each step token, as `way`s (`indexGrants`, `walkGrants`, `ownGrants`), and the stable trigger names (`triggerWheres`, `triggerSignature`) |
| `rules_loop.go` | `ruleStart`, `ruleEnd`, `ruleHint`, `ruleHintLength` and its cap `HintMaxRunes` |
| `rule_oneaction.go` | `ruleOneAction` and `dedupe` |
| `rule_talk.go` | `ruleTalkOnQuest` (rule `talk-on-quest`) |
| `rules_more.go` | `ruleChain`, `ruleRepeat`, `ruleReward`, `ruleRefs`, `refsText` |
| `ways.go` | `Way`, `WaysInto`, `Sources`: the walkthrough's inputs |
| `autostarts.go` | `AutoStarts(w) map[int][]string`: quests whose `-start` a trigger (sequence `on_complete` included) or a behaviour tree `grant_quest` grants by itself, to those grants' `Where`s, sorted, read off `indexGrants`. Offers, `rewards.questid` and item questtokens do not count. The offer index door (`main_quests.go`) hands them to `usercommands.SetAutoStarted`: such a quest cannot be abandoned (#375 phase 5, owner ruling 6) |
| `backlog.go` | `Backlog`, `ParseBacklog`, `FormatBacklog`, `Ratchet` |

## Rules

| Rule id | Check |
|---|---|
| `start` | The first step's token is granted by at least one `offer` topic, and by nothing else |
| `end` | The final step's token is granted only by `handin` topics, and by at least one |
| `hint` | Every step except the final one has a non-empty `hint` |
| `hint-length` | No step's `hint` is over `HintMaxRunes` (200, owner ruling P8): whitespace runs collapse to one space, then runes are counted. Where is `step <id>`. A step with no hint is the `hint` rule's finding. The web quest panel's "more" toggle uses the same 200 |
| `one-action` | Every non-start step has at least one way in; each way is one trigger or one topic that requires the previous step's token, with no item, flag or gold condition and no `sequence`; a trigger grants at most one step of its own quest |
| `talk-on-quest` | No dialogue topic grants a step between a quest's first and final (#375 phase 3b slice 1): a mid-quest talk step is a `topic_heard` trigger in the quest file. Where is the topic's way (`mob <id> topic <key>`). The 32 shipped cases moved, so the backlog holds none |
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
| `quest 53 trigger mob_death mob=9110 grants=53-firstkill` | a trigger, by signature: the event, its non-zero filters (room, mob, item, skill, command, quest_token, noun, verb, target, effect, topic, reason, area, zone, count, distinct in that order) and the tokens it grants, sorted, sequences included; triggers of one quest that share a signature take ` #2`, ` #3` in file order |
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

## The walkthrough contract (plan 2b)

The completability walkthrough (spec section 7) is `questengine.Walk`, because
it needs the trigger engine; this package only exports what it reads
(`ways.go`):

- `Way`: one thing that grants a token (a trigger, a topic, a completion
  reward or an external grant), with its stable backlog name `Where`, the
  `Owner` quest, `Nested` for a grant inside a sequence's `on_complete`, the
  topic's (or mob tree's) `MobId`, the topic's `File`, `Reward`, and
  `External` for an external grant.
- `WaysInto(w World) map[string][]Way`: token to every Way that grants it, the
  same index every rule reads, external grants included.
- `ExternalGrant` (`World.External`): a grant outside triggers and topics, a
  behaviour tree's `grant_quest` (with the tree's `MobId` and the `Topics` its
  branch answers) or an item's `questtoken` (`ItemId`).
  `questworld.Registry` fills them, so boot, CI and the editor door all see
  them. In `Validate` an external grant is a way in: a step only a tree
  grants is not `one-action ...:none` (Hadwen's `30-rite`). It is neither an
  offer nor a hand-in, so a grant of a start or end token is a `start` or
  `end` finding (before #375 phase 7, room 6467's tree granted `30-start`
  and `28-end`, and Hadwen's `30-end`; now his tree grants `30-rite` and
  `30-woken`, mid steps). One-action's `:prev`, `:cond`, `:seq` and `:double` are
  not checked for it: it has no trigger conditions, and its gate (the topic
  that fires the tree, or getting the item) is the walk's to check. The
  rule-by-rule choice is commented on `way.external` in `grants.go`.
- `Sources`: the world questions the walk asks. `ItemSources(itemId)` names
  where an item comes from (empty: none); `MobSpawned(mobId)` says a room or a
  quest spawns it; `ItemAvailable(itemId, questId, has)` says a player on that
  quest with progress `has` can obtain the item without the quest's later
  steps (a source the quest does not own, or one it owns through gates `has`
  already holds). A nil func skips that
  check. `SpellEffect(spellId)` is a spell's `effect_type` ("" for an unknown
  spell) and `StarterSpells` the spells every new character holds, for the
  `spell_landed` reachability check; a nil `SpellEffect` skips it.
  `questworld.Sources()` fills it from the registries.

The walk's rules (`walk-dead-end`, `walk-no-source`, `walk-unspawned`,
`walk-no-spell`: a `spell_landed` Way no spell on the path can land) share
the one backlog with the rules above. They are produced by `Walk`, not
`Validate`: boot runs `Validate` only and ignores the walk lines, and the CI
sweep ratchets both together.

## Gotchas

- Never import `conversation` or `questworld` here: the package must stay
  importable from both.
- A nil callback in `World.Refs` or `World.Names` skips that check; production
  worlds always fill them.
