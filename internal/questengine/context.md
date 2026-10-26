# Quest Engine Context

## Purpose

`internal/questengine` is the **evaluation half** of the quest system: it takes
game events, matches them against indexed triggers, evaluates conditions, and
runs actions. It does not parse quest files.

The split matters. Since the 5c-pre unification, `internal/quests` is the single
owner of the quest file parse and of every definition type. `questengine`
re-exports those types as aliases (`types.go`) purely so its API and tests stay
source-compatible — **the data shapes live in `quests`, the logic lives here.**
When you change a quest YAML field, you edit `internal/quests`; when you change
what a trigger *does*, you edit this package.

## Files

- **engine.go** — `Engine`, the trigger index, `Notify`, the recursive
  evaluator, and `fieldMatches`, the match step it shares with `WouldFire`.
- **dryrun.go**: `WouldFire` (side-effect-free "would Notify fire?") and
  `SearchFeatures` (a room's quest search nouns).
- **types.go** — type aliases onto `internal/quests`, plus `EventDetails` and
  `NotifyResult` (which describe an engine *invocation*, so they stay here).
- **conditions.go** — `PlayerState` interface and `EvalConditions`.
- **actions.go** — `ActionContext` interface and `ExecuteAction`.
- **bridge.go** — `GameBridge`, the concrete implementation of `ActionContext`
  against real users/rooms/mobs. The biggest file in the package.
- **guards.go** — `EvalGuard`: depth limit, visit tracking, grant dedupe, trace.
- **loader.go** — `GetEngine` singleton, `LoadDataFiles`, `ValidateAllFlags`.
- **map_target.go** — `ResolveQuestTarget`, which feeds the map quest marker.
- **logging.go** — level-gated logging with per-player debug opt-in.
- **test_helpers.go** — `ResetEngineForTest() func()`.
- **walkthrough.go**: `Walk`, the completability walkthrough (see Walkthrough).
- **walkplayer.go**: `walkPlayer`, `walkCtx`, the dialogue `PlayerState` adapter.

## The Engine

```go
type Engine struct {
    quests       map[int]*QuestDef
    triggerIndex map[string][]*indexedTrigger
}

func NewEngine() *Engine
func GetEngine() *Engine                       // process singleton
func (e *Engine) RegisterQuest(q *QuestDef)
func (e *Engine) GetQuest(questId int) *QuestDef
func (e *Engine) AllQuests() []*QuestDef       // unordered
func (e *Engine) Notify(eventType string, details EventDetails, player PlayerState, ctx ActionContext) NotifyResult
func (e *Engine) WouldFire(eventType string, d EventDetails, player PlayerState) bool
func (e *Engine) SearchFeatures(roomId int) []string
```

`WouldFire` runs the same match step as `Notify` (`fieldMatches`, then
`EvalConditions` against the player's current state) and stops there: no
action runs, no token is granted, no text is sent, no `quest_granted` chain is
followed, no `EvalGuard` is touched. It is for gates that must ask before the
event happens (a quest gate on an exit). `SearchFeatures` returns the sorted,
distinct nouns of `room_interact` triggers with `verb: search` and `room:`
equal to the given id, regardless of conditions.

`RegisterQuest` flattens every trigger into `triggerIndex` keyed by event type,
so `Notify` never scans quests it cannot match. Each trigger gets an
engine-assigned `indexedTrigger{def, questId, trigId}` — an explicit wrapper
that replaced an older pattern of mutating unexported fields *on* the shared
definition struct. Definitions are immutable shared data owned by `quests`;
keep it that way.

## Evaluation flow

`Notify` → `evaluate` (recursive), per matching trigger:

1. **Field match** (`matchTriggerFields`) — mob, room, item, skill, command,
   topic, questToken, noun, verb. A trigger `noun:` matches in three tiers
   (`nounTier`, picked by `pickNounTier`): the typed noun exactly; else its
   singular/plural variant (`util.NounNumberVariants`); else
   `EventDetails.RoomNoun`, the room's own noun key the typed words name
   (`rooms.Room.NounNamedBy`: a data alias, a word of a multi-word name, a
   number variant), or that key's number variant. Only the narrowest tier
   that matches any trigger of the event is used, so two spellings of one
   thing never double-fire (quest 63's `board` and `floorboard` triggers:
   `look board` fires `board` only). So a trigger written with the room's
   noun key accepts every name the room accepts for it; a trigger noun that
   is not a room noun still matches the typed text. `NounNamedBy` refuses a
   phrase with a word that is no name of the noun, so `say i found the
   board` names nothing. The command handlers in `usercommands` fill
   `RoomNoun` on `room_interact` and `command_issued`. Author one spelling
   per noun (a guard test fails on a pair that differs only by number).
   A hyphen matches a space (`util.NounHyphenFold`) in `nounMatches` on the
   trigger noun, the typed noun and `RoomNoun`, so `look
   processional-reliefs` fires `noun: processional reliefs`.
   A trigger `verb:` names a `room_interact` verb family (#375 phase 3a):
   empty means `look` (`quests.InteractVerb`), so a look trigger never
   fires on search and a `verb: search` trigger never fires on look. The
   dispatcher raises `room_interact` with `Verb: look` for look (and its
   aliases l, examine, exam) and `read` raises it when nothing carried
   matches; `search <feature>` raises `Verb: search` after the search runs.
   Every raise matches the template room (`rooms.OriginalRoomId`). The
   walkthrough sends `quests.InteractVerb(t.Verb)`.
2. **Visit guard** — `MarkVisited(trigId)`; a trigger fires at most once per
   chain.
3. **Conditions** — `EvalConditions(t.Conditions, player)`.
4. **Actions** — `executeActions`, which returns the tokens granted.
5. **Chain** — for each granted token, recurse into `quest_granted` triggers.

That last step is what makes quest chains work: granting `10-end` can
immediately fire the trigger that grants `11-start`.

Step 1 is `fieldMatches`, computed once up front because it depends only on
the event. Steps 2 and 3 stay inside the loop: an earlier trigger's actions
and chains change the player state a later trigger's conditions read, and a
trigger is marked visited even when its conditions then fail.

## `EvalGuard`

```go
func NewEvalGuard(maxDepth int) *EvalGuard
func (g *EvalGuard) PushDepth() bool      // false once maxDepth is passed
func (g *EvalGuard) PopDepth()
func (g *EvalGuard) DepthExceeded() bool
func (g *EvalGuard) MarkVisited(trigId string) bool   // false if already seen
func (g *EvalGuard) MarkGranted(token string) bool    // false if already granted
func (g *EvalGuard) AddToTrace(desc string)
func (g *EvalGuard) GetTrace() []string
```

One guard per `Notify` call. It is the only thing standing between a circular
quest chain and an infinite recursion, and its trace is what gets logged when
the depth limit trips.

Config: `Balance.QuestChainDepthLimit` (max recursion) and
`Balance.QuestPerformanceWarnMs` (slow-evaluation warning threshold).

## `GameBridge`

`NewGameBridge(user *users.UserRecord, roomId int) *GameBridge` implements both
the read side (`HasQuest`, `HasItem`, `GetQuestFlag`, `GetGold`, `GetRoomId`,
`GetUserId`, `HasOwnMasterwork`) and the write side — grant/consume/give,
gold, text, mob and item spawning, spell teaching, skill training, stat
increases, recipe learning, conditions, teleport, exit locking, quest flags, faction
rep, mutation grants, NPC dialogue queueing, and timed sequences.

`ChargeGold(amount) error` takes the whole charge or nothing. A charge
larger than the player's gold takes nothing, sends no "You pay" line, logs a
warning, and its error makes `executeActions` abandon the rest of the
trigger. Gate any trigger that runs `charge_gold` on the price first (a
`has_gold` condition). A purchase made in dialogue should not use
`charge_gold` at all: put `goldRequired` and `chargesGold` on the node so the
check and the charge happen in the same step (see `internal/dialogue`).
Gold amounts in the pay and receive lines use `util.FormatNumber`, and the
learned-recipe line uses the recipe's display name.

This is the seam that keeps the evaluator testable: tests supply a fake
`ActionContext`/`PlayerState`, production supplies `GameBridge`.

`ActionContext.Narrate(v narration.Variants)` is the one door for a text action
(since 2026-09-12; it replaced `SendText` and `RoomText`). `ExecuteAction` calls
it with `a.Narration()`; `GameBridge.Narrate` renders with the triggering
player's tagged name as `{actor}`, sends the Actor line to the player and,
as of **M4d PR 3**, delivers the Observer line through `messaging.SendTrio`
rather than the earlier `room.SendTextVisual`. `SendTrio` both sight-gates
the line AND runs `messaging.HideNames` over it for a shapes-only reader;
the earlier `room.SendTextVisual` call only ever applied tag-based
`messaging.Anonymize`, which by its own docstring never touches a bare
(untagged) name. Every shipped trigger observer line is tag-wrapped
(`quests.RoomTextProblems` requires `{actor}` in every `observer` line), so
production content was incidentally safe before this migration; nothing
enforces that convention on the `Narrate` Go API itself, so the seam now
carries the plain `ActorName` through to `SendTrio` instead of depending on
authored text happening to be tagged. A quest trigger has one party (the
triggering player), so `ActeeName` is `messaging.NoName` and there is no
Actee line. The quest `actor` line is substituted too; no shipped line
carries a token. (The quest keys are `actor` and `observer` since M4b-1; they
were `send_text` and `room_text`, which the behaviour tree still uses for its
own, unrelated action param.)

## Gotchas

- **Zero and empty trigger fields are wildcards.** `matchTriggerFields` skips
  any filter where the value is `0` or `""`. A trigger with `mob: 0` matches
  *every* mob, not "no mob". Omitting a field widens the trigger.
- **A trigger's actions are applied as a unit.** Each runs under its own
  `recover()`, but a failure or panic **abandons the remaining actions of that
  trigger** (logged with how many were dropped). There is no rollback of what
  already ran, so a failure still leaves a partial effect — aborting only stops
  it compounding. Treat action errors as real bugs. A duplicate grant is a skip,
  not a failure, and does not abort.
- **Grant dedupe is per evaluation chain, not per player.** `MarkGranted` lives
  on the guard, which is created fresh in `Notify`. It stops a token being
  granted twice in one cascade; it does not stop re-granting across events.
  Re-grant prevention is a content concern — see the quest-token `questExcluded`
  SOP in `CLAUDE.md`.
- **`AllQuests` is sorted by quest id.** It iterates a map internally but sorts
  before returning, so audit and gate output is stable between runs.
- **`command_issued` and `command` are different events** — the first fires when
  a command is typed, the second only when it succeeded. `command_issued` fires
  on a refusal too, so a step whose command can be refused listens on
  `command` (quest 28 does for say, warcry, cast, trip and flee). Each handler
  sends its own `command`; a few cases worth knowing: `cast` carries the
  canonical spell id as `Noun`; `equip` (and its aliases wear, wield, hold)
  carries the item put on as `ItemId`, so a step filters it with `item:`
  (quests 31, 32 and 50 do); `trip` also sends it when the target is
  already on the ground, and `warcry` when one is already riding the player
  (the goal holds); `flee` is sent by the round hook once the player has
  arrived, from the room fled into. `forage` is sent only when the forage
  rolled (not on a wrong-ground or cooldown refusal; finding nothing still
  counts), `track` not for `track stop` or a cooldown refusal, and `sneak`
  only when the player ends up hidden (already hidden counts).
- **An `npc_say` line with `whisper: true` reaches the triggering player
  alone.** `npcCommand` turns it into the mob command `whisper @<userId>`,
  framed in asterisks so it stands out from combat text; the room hears
  nothing, so the line prints once. Quest 28 whispers Dewey's lines during the
  effigy fight. A line with `emote: true` ignores `whisper`.
- **A quest holds ONE current step, so objectives cannot run in any order.**
  `characters.HasQuest(t)` is true for the current step and every step before
  it, and a grant only moves forward. Two triggers that each gate only on
  `-start` let the later objective, done first, mark the earlier one done
  without doing it. Gate each trigger on the previous step's token. Newbie
  quests also ask for one command per step, each hint naming it plainly
  (owner ruling 2026-10-17; `newbie_one_command_steps_test.go` walks quests
  41 and 50 and checks the cert hints).
- **`room_gate` refuses a walk before it happens; `room_enter` fires after.**
  `usercommands.Go` notifies `room_gate` (filter `room`, the destination's
  template id) after the lock check and before the exit message, the move
  cost and `MoveToRoom`. A `Handled` result refuses the step: the trigger's
  actions narrate why and the player never moves, so no departure line and
  no arrival detection follow. The bridge is the player's CURRENT room, so
  `in_room` names the side the gate stands on (quest 14's cellar gate is
  `in_room: 485`, so a player inside the tunnels can always walk out). A
  room_enter trigger that teleports the player back is the old shape and
  printed the refusal, then "You head downward.", then the destination's
  detection line (#558); do not write a gate that way.
- **Ephemeral (instance) rooms match their TEMPLATE room id** in `room:`
  triggers, not their runtime id.
- **`craft`, `sleep` and `search` send `command` on success only** (owner,
  2026-10-17: a quest step advances only when the action succeeds). `craft`
  goes out when a craft COMPLETES and succeeded, never when it begins: a timed
  craft's contest runs at its last round, so the round hook
  (`hooks.tickUserCrafting`) sends it then, from the crafter's room, and a
  failed craft or an enchant that could not land sends nothing; an instant
  recipe sends it at once (`usercommands.notifyCraftQuest`). `sleep` is sent
  only when the player is asleep (not on a refusal in combat), `search` not on
  a cooldown refusal.
- **Every quest `observer` line must contain `{actor}`, and `quests.Quest.Validate`
  refuses a quest whose line does not**, including lines nested in a sequence's
  `on_complete`. `Validate` runs on every quest file parse at boot AND before an
  admin editor save, so a bad line is refused with a reply instead of being
  saved and left to fail the next boot. The room is watching the player act, so
  the line must say who. It also rejects `{actee}` and `{actee_plain}` (a
  quest has no actee) and `{actor_plain}` (an untagged name cannot be
  anonymized in the dark). The rules live in `quests.RoomTextProblems`, and
  `TestShippedQuestRoomTextFollowsConvention` in `internal/quests` checks the
  shipped files.
- **`ConsumeItem` is only honoured on `item_give`.** Setting it on any other
  event silently does nothing. Note also that `give.go` transfers the item to
  the mob *before* any handler runs, so consumption is a post-hoc correction —
  see the give.go gotcha in `CLAUDE.md`.

## Walkthrough

`Walk(w questloop.World, src questloop.Sources) []questloop.Finding` (quest arc
#375 phase 2b, spec section 7) drives every quest that has an `offer` topic
from its offer to its hand-in through the real `Engine.Notify` and the real
dialogue topics (`DialogueFile.Eligible` then `Apply`), with no game running.
It sorts the quests by id first, so the engine's trigger order does not depend
on the caller's order. It reads the Ways into each step from
`questloop.WaysInto`, which includes the world's external grants
(`World.External`).

- **`walkPlayer`** is the mock player. It has the game's real step order:
  `HasQuest(t)` holds for the current step and every earlier one, a grant only
  moves forward, and a quest is entered only at `start` (or the only step of a
  one-step quest), as `quests.IsTokenAfter` allows. It also holds items,
  flags, heard topics, gold and room, and `dialogueState()` wraps it as a
  `dialogue.PlayerState`.
- **`walkCtx`** is the `ActionContext`. Grants, items, gold and quest flags
  change the player; narration, spawns, teleports and the rest are no-ops.
  `QueueSequence` runs the sequence's `on_complete` actions at once, as the
  real bridge does after the lines finish (`GameBridge.QueueSequence`,
  `bridge.go`). `trailCtx` wraps it to note every grant.
- **`walkEnv`** is what every firing reads: the engine, the sources, the
  dialogue files by NPC, the topics some `heardRequired` names, and the flag
  setters.
- **Prerequisites.** `meetToken` holds a token a condition or gate names, but
  never a token of the walked quest itself: the walk is already on that quest's
  real progress, so a Way that needs a LATER step of its own quest cannot fire
  and is a dead end. Other quests' tokens and flags are held as prerequisites.
- **Own flags are never faked.** `walkEnv.meetFlag` gives a flag of the walked
  quest only when the walk sets it: a Way that sets it, or a "setter" (a
  trigger or topic that sets it and grants no step of the quest, such as quest
  72's `room_interact` `set_flag`) fired from the current state without moving
  the quest. Setters nest at most `maxSetterDepth` deep.
- **Heard topics.** `walkEnv.hear` marks as heard every info topic a
  `heardRequired` names that is open in the current state; applying an info
  topic marks it too. A topic whose heard topic never opens is a dead end.
- **Negative gates** (`missing`, `missing_item`, `missing_flag`,
  `questExcluded`, `itemExcluded`, `questFlagExcluded`) read the walk's real
  state.
- **Branches.** The walk carries every distinct successful state forward
  (`walkStates`, at most `maxWalkStates` = 16 per step, by `fingerprint`), so a
  later Way that needs one branch's flag is reached through that branch. A Way
  is reported as not reaching its step only when it works from no state; a
  skip from any state is reported.
- **`quest_granted` of the walked quest** is never fired directly. It fires
  only inside the chain of the grant that lands on its token, which every
  firing runs (the engine's chain, and `fireTopic` notifies `quest_granted`
  on a fresh dialogue grant as `Quest_HandleQuestUpdate.go` does). When the
  step it grants is dead, the Way is named too.
- **External grants.** A behaviour tree's `grant_quest` cannot be run, so the
  walk takes it from the step before it, once a topic its branch answers is
  open at the tree's NPC (or at once when the branch keys on no topic). The
  tree sets the token before the quest hook runs, so no `quest_granted` chain
  follows, in the game or the walk. An item `questtoken` fires once the walk
  holds the item or `Sources.ItemAvailable` says it can be had, and its chain
  runs. Tree grants are never fired by the skip probe.
- **Skip detection.** Two checks. (1) A `walkTrail` records every step one
  firing put the player on: a `quest_granted` chain through each step is fine,
  a missing step is a skip. (2) A probe fires each Way into a LATER step from
  each state resting on the current step. It gives an item only when the walk
  already holds it (an earlier step handed it over) or
  `Sources.ItemAvailable(item, quest, p.HasQuest)` says it can be had at this
  progress, and flags a Way that lands past the next step.
- **Source checks** run once for every Way into every step, so the steps after
  a dead step are still checked.

| Rule | Finding |
|---|---|
| `walk-dead-end` | A step no Way reaches, an offer that cannot be taken, a Way that does not reach its step, or a firing that skips a step |
| `walk-no-source` | A Way needs an item (`has_item`, `item_give`, `item_gain`, `requiresItem`, `carriesItem`, an item `questtoken`) that `Sources.ItemSources` says nothing provides |
| `walk-unspawned` | A Way needs a mob (`mob_death`, `item_give` target, the topic's NPC, a tree grant's NPC) that `Sources.MobSpawned` says nothing places |

Findings share `questloop`'s shape and its one backlog. The walk is CI only:
the sweep `TestSmoke_ShippedQuestsPassTheLoop` (`quest_loop_smoke_test.go`)
combines `questloop.Validate` and `Walk`; boot (`main_quests.go`) runs
`Validate` only and ignores the walk lines. Today's backlog holds 29 walk
lines, all `walk-dead-end`: 6 steps with no way in and 23 skips. Six of the
skips are hand-ins gated only on `start` plus an item an earlier step hands
over (quests 14, 63 twice, 65, 66, 67).

Deviation from spec 7.1: `walkPlayer` and `walkCtx` are package code, not the
`_test.go` doubles (`fullMockPlayer`, `mockActionContext`, `linearPlayer`),
because the root-package CI sweep calls `Walk` and cannot use test types. The
doubles stay where they are.

Known limits:

- Behaviour-tree mob spawns are not indexed by `questworld.Sources`, so a mob
  placed only that way reads as unspawned.
- A tree's own conditions (`player_has_quest`, state flags) are not read: a
  tree grant is taken from the step before it whenever its topic is open.
- A setter's other effects (items, gold) are not carried over; only the flag.
- A topic heard only at a step a `quest_granted` chain passes through without
  resting there is not marked.

## Dependencies

`internal/quests` (all definition types), `configs`, `users`, `mobs`, `items`,
`rooms`, `messaging`, `mudlog`, `dialogue`, `questloop` (the walkthrough).

## Consumers

`internal/hooks` (event dispatch), `internal/usercommands`, `internal/dialogue`
(quest grants from dialogue nodes), and `internal/mapper` consumers that render
the quest marker via `ResolveQuestTarget`.
