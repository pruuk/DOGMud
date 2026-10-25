# Quest Arc Phase 2b: The Completability Walkthrough (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A CI test drives every shipped quest headlessly through the real quest engine and the real dialogue topics, from its offer to its hand-in, and reports any step that cannot be reached, any item a step needs that nothing in the world provides, and any NPC a step needs that never spawns.

**Architecture:** Spec `docs/superpowers/specs/2026-10-24-quest-arc-phase2-loop-design.md` section 7 (rulings P1 to P7). The pure `internal/questloop` package exports its grant index (`WaysInto`) and a `Sources` callback struct. A new `internal/questengine/walkthrough.go` holds the driver: a world-aware mock player with the game's real step-order semantics, a no-op action context, and `Walk(world, sources) []questloop.Finding`. `internal/questworld/sources.go` builds `Sources` from the live registries. The CI sweep runs `Validate` and `Walk` together behind the one backlog ratchet. The boot door is unchanged (it runs `Validate` only).

**Tech Stack:** Go, the questengine trigger engine, `internal/dialogue` topics, testify `require`.

---

## Ground rules for every task

- **Edit files with the Edit and Write tools only.** No `sed -i`, no shell redirection into a
  tracked file, no Python read-modify-write. Generated files go to the scratchpad, then `cp`.
- **Stage by name:** `git add <paths>`. Never `git add -A`, `.`, `--all`, `git commit -a`, `--no-verify`.
- Commit messages through a quoted heredoc (`git commit -F - <<'EOF'`), ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Never kill a process you did not start; never by name or port. Never push.
- No em or en dashes in code comments or text.
- Read code before editing it. Read files over 400 lines in chunks of at most 400 lines.
- Worktree: `C:/tmp/dogmud-375-phase2b`, branch `feat/375-phase2b-walkthrough`, made from
  `feat/375-phase2a-loop` (PR #666, not merged). If #666 merges before this branch, rebase it
  onto master before the PR.

## Facts verified against source (`feat/375-phase2a-loop` at `f4464c2bf`, 2026-10-24)

| Fact | Source |
|---|---|
| `Engine.Notify(eventType string, details EventDetails, player PlayerState, ctx ActionContext) NotifyResult`; it chains `quest_granted` for every token granted | `internal/questengine/engine.go:73,154` |
| `NewEngine() *Engine`, `(*Engine).RegisterQuest(q *QuestDef)`; `QuestDef = quests.Quest` | `engine.go:30,38`, `types.go:11` |
| `EventDetails{UserId, RoomId, MobId, ItemId, Skill, Command, Noun, RoomNoun, Verb, QuestToken}` | `types.go:39` |
| `matchTriggerFields`: a trigger matches when each of its non-zero filters (`Mob`, `Room`, `Item`, `Skill`, `Command`, `QuestToken`, `Noun`, `Verb`) equals the event's field; `Noun` also matches singular/plural and room-noun forms | `engine.go:200-258` |
| `PlayerState` interface: `HasQuest(token) bool`, `HasItem(id) bool`, `GetRoomId() int`, `GetQuestFlag(key) string`, `GetGold() int`, `HasOwnMasterwork(skillMin int) bool` | `internal/questengine/conditions.go:5` |
| `EvalConditions` checks `Has`, `Missing`, `InRoom`, `HasItem`, `MissingItem`, `HasFlag`, `MissingFlag`, `HasGold`, `HasMasterwork` | `conditions.go:16` |
| `ActionContext` interface: `GrantQuest`, `ConsumeItem`, `GiveItem(int) error`, `GiveGold`, `ChargeGold(int) error`, `Narrate(narration.Variants)`, `SpawnMob(SpawnDef)`, `SpawnItem(SpawnDef)`, `TeachSpell`, `TrainSkill`, `IncreaseStat`, `LearnRecipe`, `ApplyStatusCondition(StatusConditionDef)`, `Teleport`, `LockExits(ExitLock)`, `UnlockExits(ExitLock)`, `QueueNpcSay(NpcSayDef)`, `QueueSequence(SequenceDef)`, `GiveMutation`, `SetQuestFlag`, `BumpRep`, `GetUserId` | `internal/questengine/actions.go:15-47` |
| Real step semantics: `HasQuest(t)` holds for the current step and every earlier one; a grant only moves forward, and a quest is entered only at its first step | `internal/characters/quests.go` (`HasQuest`, `GiveQuestToken`); the test double `linearPlayer` restates it, `internal/questengine/newbie_one_command_steps_test.go:25-115` |
| The questengine test doubles (`fullMockPlayer`, `mockActionContext`, `linearPlayer`) live in `_test.go` files, so non-test code cannot use them | `engine_test.go:11`, `actions_test.go:24`, `newbie_one_command_steps_test.go:30` |
| `questengine` imports `dialogue`, `quests`, `users`, `rooms` (it is heavy); `questloop` imports only `quests` and `dialogue` and must stay pure | `go list -deps`, 2026-10-24 |
| questloop grant index: unexported `way{where, trigger, owner, nested, topic, mobId, reward}`, `grants map[string][]way`, `indexGrants(w World) grants`; `where` is already the stable backlog key string (`quest 53 trigger mob_death mob=9110 grants=53-firstkill`, `mob 9112 topic cull_in`) | `internal/questloop/grants.go:13-62` |
| `questloop.World{Quests, Dialogue, Refs, Names}`, `Finding{Rule, QuestId, Where, Msg}`, `Validate`, `Ratchet`, `FormatBacklog` | `internal/questloop/questloop.go`, `backlog.go` |
| `dialogue.DialogueFile.Eligible(t, ps)` and `Apply(t, ps) bool`; `PlayerState` callbacks incl. `QuestOpen`, `HasHeard`, `MarkHeard` | `internal/dialogue/topics.go`, `types.go` |
| Topic gates `QuestRequired`, `QuestExcluded`, `QuestFlagRequired`, `QuestFlagExcluded`, `RequiresItem`, `ItemExcluded`, `CarriesItem`, `GoldRequired`, `MasterworkRequired`; `HeardRequired` keys | `internal/dialogue/topic.go` |
| Item sources: `mobs.AllMobTemplates() []*Mob`; `Mob.LootPool []int`; `Mob.Character.Items []items.Item`, `Mob.Character.Equipment.GetAllItems()`; `Mob.Character.Shop` is `[]ShopItem{MobId, ItemId, ...}` | `internal/mobs/mobs.go:86,233`, `internal/characters/character.go:131,144,148`, `internal/characters/worn.go:133`, `internal/characters/shop.go:16` |
| Room sources: `rooms.GetAllRoomIds() []int`, `rooms.LoadRoomTemplate(id) *Room`; `Room.SpawnInfo []SpawnInfo{MobId, ItemId, ...}`, `Room.Items []items.Item`, `Room.Containers map[string]Container{Items []items.Item, Recipes map[int][]int}` (a `Recipes` key is an item the container makes) | `internal/rooms/roommanager.go:264`, `save_and_load.go:248`, `rooms.go:108,114,118`, `container.go:8-15`, `spawninfo.go:11-14` |
| Forage sources: `forager.ForageYields`, `NightForageYields`, `ZoneForageYields`, `StormForageYields`, each `map[string][]int` | `internal/forager/forage_core.go:29,42,53,63` |
| Craft sources: `crafting.GetAll() map[string]*RecipeSpec`; `RecipeSpec.Output.ItemId` | `internal/crafting/crafting.go:26-43,134` |
| Quest and topic sources: trigger action `give_item` (`ActionDef.GiveItem`), `spawn_item` (`SpawnItem *SpawnDef`), rewards `ItemId` / `ItemInfo`, topic `givesItem`; mobs spawned by `spawn_mob` actions | `internal/quests/triggers.go:45-70`, `quests.go:36` |
| `forager` does not reach `conversation`, `questworld` or `questengine` | `go list -deps ./internal/forager`, 2026-10-24 |
| CI sweep `TestSmoke_ShippedQuestsPassTheLoop` (root, needs `DOGMUD_BOOT_SMOKE=1`, writes the backlog with `DOGMUD_LOOP_BACKLOG_WRITE=<path>`) | `quest_loop_smoke_test.go` |
| Boot door `questDoor` / `questDoorFrom` runs `Validate` only and ignores stale backlog lines | `main_quests.go` |

**Deviation from spec 7.1, decided here:** the spec says the mock players "move from `_test.go`
files into one shared test-helper file". The walkthrough is package code (the CI sweep in the
root package calls it), so it cannot use `_test.go` types. It gets its own `walkPlayer` in
`walkthrough.go`, built for many quests at once. The existing test doubles stay where they are.
Task 6 records this in the spec.

## File map

| File | Change |
|---|---|
| `internal/questloop/ways.go` (new) | exported `Way`, `WaysInto`, `Sources` |
| `internal/questengine/walkplayer.go` (new) | `walkPlayer` (PlayerState with real step order), `walkCtx` (no-op ActionContext), the dialogue `PlayerState` adapter |
| `internal/questengine/walkthrough.go` (new) | `Walk(w questloop.World, src questloop.Sources) []questloop.Finding` |
| `internal/questengine/walkthrough_test.go` (new) | fixture worlds, one per rule, each proven to fail |
| `internal/questworld/sources.go` (new) | `Sources() questloop.Sources` over the registries |
| `quest_loop_smoke_test.go` | runs `Validate` plus `Walk` |
| `_datafiles/world/dogmud/quests/loop-backlog.txt` | regenerated with the walk findings |
| docs | `context.md` of questloop, questengine, questworld; spec 7; roadmap; `docs/README.md` |

---

### Task 0: Worktree check

- [ ] **Step 1:** `git -C C:/tmp/dogmud-375-phase2b log --oneline -1` shows `f4464c2bf` or later
  on `feat/375-phase2b-walkthrough`. `go build ./... && go test ./internal/questloop/ ./internal/questengine/`
  passes. If not, stop and report.

---

### Task 1: Export the ways and the sources contract from `questloop`

**Files:**
- Create: `internal/questloop/ways.go`, `internal/questloop/ways_test.go`

- [ ] **Step 1: Write the failing test** `ways_test.go`

```go
package questloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaysInto_ExposesTheIndex(t *testing.T) {
	w := goodWorld()
	ways := WaysInto(w)

	require.Len(t, ways["53-start"], 1)
	require.NotNil(t, ways["53-start"][0].Topic)
	require.Equal(t, 9112, ways["53-start"][0].MobId)
	require.Equal(t, "mob 9112 topic cull_offer", ways["53-start"][0].Where)

	require.Len(t, ways["53-firstkill"], 1)
	require.NotNil(t, ways["53-firstkill"][0].Trigger)
	require.Equal(t, 53, ways["53-firstkill"][0].Owner)
	require.Contains(t, ways["53-firstkill"][0].Where, "quest 53 trigger mob_death")

	require.Len(t, ways["53-end"], 1)
	require.Equal(t, "cull_in", ways["53-end"][0].Topic.Key)
}
```

(`goodWorld()` is the fixture in `questloop_test.go`.)

- [ ] **Step 2: Run it**

Run: `go test ./internal/questloop/ -run TestWaysInto -v`
Expected: FAIL to compile (`undefined: WaysInto`).

- [ ] **Step 3: Implement** `ways.go`

```go
package questloop

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Way is one thing in the world that grants a quest token, as the rules see
// it: a trigger, a topic, or a completion reward. Where is its stable backlog
// name. The walkthrough (questengine.Walk) drives each Way.
type Way struct {
	Where   string
	Trigger *quests.TriggerDef // set for a trigger
	Owner   int                // the quest whose file holds the trigger
	Nested  bool               // granted inside a sequence's on_complete
	Topic   *dialogue.Topic    // set for a topic
	MobId   int                // the topic's NPC
	File    *dialogue.DialogueFile
	Reward  bool // rewards.questid
}

// WaysInto maps each token ("53-firstkill") to every Way that grants it, the
// same index every rule reads.
func WaysInto(w World) map[string][]Way {
	out := map[string][]Way{}
	files := map[int]*dialogue.DialogueFile{}
	for _, df := range w.Dialogue {
		if df != nil {
			files[df.MobId] = df
		}
	}
	for tok, ws := range indexGrants(w) {
		for _, x := range ws {
			out[tok] = append(out[tok], Way{
				Where: x.where, Trigger: x.trigger, Owner: x.owner, Nested: x.nested,
				Topic: x.topic, MobId: x.mobId, File: files[x.mobId], Reward: x.reward,
			})
		}
	}
	return out
}

// Sources answers the walkthrough's world questions. Each func may be nil,
// which skips that check (a fixture world that only tests step order).
type Sources struct {
	// ItemSources names where an item can come from ("mob 9110 loot",
	// "shop 9123", "room 5203 floor", "forage marsh", "recipe iron-dagger",
	// "quest 32 give_item", "mob 9106 topic blood_offer"). Empty: no source.
	ItemSources func(itemId int) []string
	// MobSpawned reports whether a mob template is placed by some room's
	// spawninfo or a quest spawn_mob action.
	MobSpawned func(mobId int) bool
}
```

- [ ] **Step 4: Run**

Run: `go test ./internal/questloop/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/questloop/ways.go internal/questloop/ways_test.go
git commit -F - <<'EOF'
feat(questloop): export the grant index and the walkthrough sources contract (#375 phase 2b)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: The walk player and the no-op action context

**Files:**
- Create: `internal/questengine/walkplayer.go`, `internal/questengine/walkplayer_test.go`

- [ ] **Step 1: Write the failing tests** `walkplayer_test.go`

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func walkQuests() map[int][]string {
	return map[int][]string{53: {"start", "firstkill", "end"}, 34: {"start", "end"}}
}

func TestWalkPlayer_StepOrderSemantics(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	require.False(t, p.HasQuest("53-start"))

	p.grant("53-firstkill")
	require.False(t, p.HasQuest("53-start"), "a quest is entered only at its first step")

	p.grant("53-start")
	require.True(t, p.HasQuest("53-start"))
	require.False(t, p.HasQuest("53-firstkill"))

	p.grant("53-end")
	require.True(t, p.HasQuest("53-firstkill"), "a later step implies the earlier ones")
	require.Equal(t, "end", p.progress[53])

	p.grant("53-start")
	require.Equal(t, "end", p.progress[53], "a grant never moves back")
}

func TestWalkPlayer_SetHeldAndClone(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	p.hold("34-end")
	p.items[7] = true
	c := p.clone()
	c.grant("53-start")
	c.items[8] = true
	require.True(t, c.HasQuest("34-start"))
	require.False(t, p.HasQuest("53-start"), "a clone does not share progress")
	require.False(t, p.items[8], "a clone does not share items")
}

func TestWalkCtx_GrantGiveConsume(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	ctx := &walkCtx{p: p}
	ctx.GrantQuest("53-start")
	require.NoError(t, ctx.GiveItem(5))
	ctx.ConsumeItem(5)
	ctx.SetQuestFlag("53-x", "y")
	require.True(t, p.HasQuest("53-start"))
	require.False(t, p.items[5])
	require.Equal(t, "y", p.GetQuestFlag("53-x"))
	var _ ActionContext = ctx // compile-time: walkCtx satisfies the interface
	var _ PlayerState = p
	_ = quests.Quest{}
}

func TestDialogueState_RoutesToThePlayer(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	ps := p.dialogueState()
	ps.GiveQuest("53-start")
	require.True(t, ps.HasQuest("53-start"))
	require.True(t, ps.GiveItem(9))
	require.True(t, ps.HasItem(9))
	require.True(t, ps.RemoveItem(9))
	require.False(t, ps.HasItem(9))
	require.True(t, ps.HasHeard("anything"), "the walk counts every info topic as heard")
	require.True(t, ps.QuestOpen(53))
}
```

- [ ] **Step 2: Run them**

Run: `go test ./internal/questengine/ -run 'TestWalkPlayer|TestWalkCtx|TestDialogueState' -v`
Expected: FAIL to compile (`undefined: newWalkPlayer`).

- [ ] **Step 3: Implement** `walkplayer.go`

```go
package questengine

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// walkPlayer is the walkthrough's player: quest progress with the game's real
// step order (HasQuest holds for the current step and every earlier one; a
// grant only moves forward; a quest is entered only at its first step, as
// characters.GiveQuestToken does), plus items, flags, gold and room. It
// satisfies PlayerState for the trigger engine and backs a dialogue
// PlayerState for topics.
type walkPlayer struct {
	steps      map[int][]string // quest id -> step ids in order
	progress   map[int]string   // quest id -> current step id
	items      map[int]bool
	flags      map[string]string
	gold       int
	roomId     int
	masterwork bool
}

func newWalkPlayer(steps map[int][]string) *walkPlayer {
	return &walkPlayer{steps: steps, progress: map[int]string{}, items: map[int]bool{}, flags: map[string]string{}}
}

func splitToken(token string) (int, string) {
	id, step, ok := strings.Cut(token, "-")
	if !ok {
		return 0, ""
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0, ""
	}
	return n, step
}

func (p *walkPlayer) index(questId int, step string) int {
	return slices.Index(p.steps[questId], step)
}

func (p *walkPlayer) HasQuest(token string) bool {
	id, step := splitToken(token)
	cur, ok := p.progress[id]
	if !ok {
		return false
	}
	want := p.index(id, step)
	return want >= 0 && want <= p.index(id, cur)
}

// grant moves a quest forward the way GiveQuestToken does.
func (p *walkPlayer) grant(token string) {
	id, step := splitToken(token)
	next := p.index(id, step)
	if next < 0 {
		return
	}
	cur, ok := p.progress[id]
	if !ok {
		if next == 0 {
			p.progress[id] = step
		}
		return
	}
	if next > p.index(id, cur) {
		p.progress[id] = step
	}
}

// hold puts the player on a token's step directly, for prerequisites.
func (p *walkPlayer) hold(token string) {
	id, step := splitToken(token)
	if p.index(id, step) >= 0 {
		p.progress[id] = step
	}
}

func (p *walkPlayer) clone() *walkPlayer {
	c := *p
	c.progress = maps.Clone(p.progress)
	c.items = maps.Clone(p.items)
	c.flags = maps.Clone(p.flags)
	return &c
}

func (p *walkPlayer) HasItem(itemId int) bool          { return p.items[itemId] }
func (p *walkPlayer) GetRoomId() int                   { return p.roomId }
func (p *walkPlayer) GetQuestFlag(key string) string   { return p.flags[key] }
func (p *walkPlayer) GetGold() int                     { return p.gold }
func (p *walkPlayer) HasOwnMasterwork(int) bool        { return p.masterwork }

// dialogueState is a dialogue PlayerState over p. Every info topic counts as
// heard and every quest as open: the walk tests reachability, not pacing.
func (p *walkPlayer) dialogueState() *dialogue.PlayerState {
	return &dialogue.PlayerState{
		HasQuest:    p.HasQuest,
		HasItem:     p.HasItem,
		CarriesItem: p.HasItem,
		RemoveItem: func(id int) bool {
			if !p.items[id] {
				return false
			}
			delete(p.items, id)
			return true
		},
		GiveQuest:        p.grant,
		GiveItem:         func(id int) bool { p.items[id] = true; return true },
		GetQuestFlag:     p.GetQuestFlag,
		SetQuestFlag:     func(k, v string) { p.flags[k] = v },
		BumpRep:          func(string, int) {},
		GiveGold:         func(n int) { p.gold += n },
		HasOwnMasterwork: p.HasOwnMasterwork,
		HasGold:          func(n int) bool { return p.gold >= n },
		ChargeGold: func(n int) bool {
			if p.gold < n {
				return false
			}
			p.gold -= n
			return true
		},
		HasHeard:  func(string) bool { return true },
		MarkHeard: func(string) {},
		QuestOpen: func(int) bool { return true },
	}
}

// walkCtx is the walkthrough's ActionContext: grants, items, gold and flags
// change the walk player; everything else (narration, spawns, teleports,
// timed scenes) is a no-op, because the walk follows quest state only.
type walkCtx struct{ p *walkPlayer }

func (c *walkCtx) GrantQuest(token string)               { c.p.grant(token) }
func (c *walkCtx) ConsumeItem(itemId int)                { delete(c.p.items, itemId) }
func (c *walkCtx) GiveItem(itemId int) error             { c.p.items[itemId] = true; return nil }
func (c *walkCtx) GiveGold(amount int)                   { c.p.gold += amount }
func (c *walkCtx) ChargeGold(amount int) error           { c.p.gold -= amount; return nil }
func (c *walkCtx) Narrate(narration.Variants)            {}
func (c *walkCtx) SpawnMob(SpawnDef)                     {}
func (c *walkCtx) SpawnItem(SpawnDef)                    {}
func (c *walkCtx) TeachSpell(string)                     {}
func (c *walkCtx) TrainSkill(string, int)                {}
func (c *walkCtx) IncreaseStat(string, int)              {}
func (c *walkCtx) LearnRecipe(string)                    {}
func (c *walkCtx) ApplyStatusCondition(StatusConditionDef) {}
func (c *walkCtx) Teleport(int)                          {}
func (c *walkCtx) LockExits(ExitLock)                    {}
func (c *walkCtx) UnlockExits(ExitLock)                  {}
func (c *walkCtx) QueueNpcSay(NpcSayDef)                 {}
func (c *walkCtx) QueueSequence(SequenceDef)             {}
func (c *walkCtx) GiveMutation()                         {}
func (c *walkCtx) SetQuestFlag(key, value string)        { c.p.flags[key] = value }
func (c *walkCtx) BumpRep(string, int)                   {}
func (c *walkCtx) GetUserId() int                        { return 0 }
```

Two checks while writing it, both by reading source, not guessing:
- `QueueSequence`: the real bridge runs a sequence's `on_complete` actions after its lines.
  Read the real implementation (`grep -n "func (b \*GameBridge) QueueSequence" -A30 internal/questengine/bridge.go`).
  If `on_complete` grants are run through the engine there, `walkCtx.QueueSequence` must run
  them too (call `ExecuteAction` for each `s.OnComplete` action with `c`), or a quest whose
  step is granted inside a sequence would read as a dead end that the real game can finish.
  The one-action rule already flags such steps; the walk must not add a second, false finding.
- Method set: compare with `actions.go:15-47`; `go build` fails if one is missing or misspelled.

`gofmt` will align the one-line methods; run it.

- [ ] **Step 4: Run**

Run: `go test ./internal/questengine/ -run 'TestWalkPlayer|TestWalkCtx|TestDialogueState' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/walkplayer.go internal/questengine/walkplayer_test.go
git commit -F - <<'EOF'
feat(questengine): walkthrough player with real step order and a no-op action context (#375 phase 2b)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `Walk`: drive every quest from offer to hand-in

**Files:**
- Create: `internal/questengine/walkthrough.go`, `internal/questengine/walkthrough_test.go`

The algorithm, per quest `q` with at least one step (spec 7.1):

1. **Offer.** Find the `offer` Ways into `q`'s first token (`WaysInto(w)[first]` where
   `Topic.Kind == offer`). None: skip the quest (the `start` rule already reports it).
2. **Base player.** For the first offer: hold every `QuestRequired` token, give
   `RequiresItem` and `CarriesItem`, set `QuestFlagRequired`, set gold to `GoldRequired`,
   set masterwork when `MasterworkRequired > 0`. Then `Eligible` must be true and `Apply` must
   put `q` on its first step. If not: finding `walk-dead-end` at the offer's Where.
3. **Steps.** For each step `i` from 1 to the last, take `WaysInto(w)[token(i)]`. For each Way,
   from a fresh `clone()` of the state at step `i-1`:
   - **Trigger:** satisfy its conditions (hold every `Has` token, give `HasItem`, set
     `HasFlag`, gold `HasGold`, masterwork when `HasMasterwork > 0`, room `InRoom`); give the
     trigger's `Item` filter for `item_give` and `item_gain` events. Build `EventDetails` from
     its filters (`Room`, `Mob`, `Item`, `Skill`, `Command`, `QuestToken`, `Noun`, `Verb`;
     `RoomId` also from `InRoom` when the trigger has no `Room`). For a `quest_granted`
     trigger, the event is the token its `QuestToken` filter names. Call `Notify` on an engine
     holding every quest.
   - **Topic:** satisfy its gates (same as the offer) on the clone, then `Eligible` and `Apply`.
     Topics outside `q`'s own steps are not walked.
   - **Reward Way:** skip (the one-action rule reports it).
   - **Result:** the step is reached when `progress[q] == step i`. If it jumped past step `i`,
     finding `walk-dead-end` "skips to <step>". If it did not move, finding `walk-dead-end`
     "does not reach <token>". Each failing Way is reported by its Where.
   - Continue from the clone of the first Way that reached step `i`. If none did, add a
     finding at `step <id>` ("no way in reaches it") and stop walking `q`.
4. **Sources** (Task 4) are checked per Way, before firing it.

- [ ] **Step 1: Write the failing tests** `walkthrough_test.go`

```go
package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// walkWorld is Cull the Wash as one clean loop: Garve offers it, a kill moves
// it on, Garve takes the hand-in.
func walkWorld() questloop.World {
	q := &quests.Quest{
		QuestId: 53, Name: "Cull the Wash",
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "firstkill", Hint: "h"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{{
			Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-firstkill"}},
			Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
		}},
	}
	df := &dialogue.DialogueFile{MobId: 9112, Topics: []dialogue.Topic{
		{Key: "cull_offer", Kind: dialogue.KindOffer, Quest: 53, Label: "Cull", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "53-start"}},
		{Key: "cull_in", Kind: dialogue.KindHandIn, Quest: 53, Label: "Cull", Text: "t",
			Gates: dialogue.Gates{QuestRequired: []string{"53-firstkill"}}, Effects: dialogue.Effects{GrantsQuest: "53-end"}},
	}}
	return questloop.World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{df}}
}

func keys(fs []questloop.Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key())
	}
	return out
}

func TestWalk_CleanQuestFinishes(t *testing.T) {
	require.Empty(t, Walk(walkWorld(), questloop.Sources{}))
}

func TestWalk_UnreachableStepIsADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.Has = []string{"53-end"} // can never hold end before firstkill
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step firstkill")
}

func TestWalk_SkippingAStepIsReported(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Grant: "53-end"}}
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "skill_use", Skill: "brawling",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	})
	fs := Walk(w, questloop.Sources{})
	require.NotEmpty(t, fs)
	require.True(t, strings.Contains(fs[0].Msg, "skips"), fs[0].String())
}

func TestWalk_BrokenAlternativeIsNamed(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "room_enter", Room: 5,
		Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-start"}}, // impossible
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	})
	got := keys(Walk(w, questloop.Sources{}))
	require.Len(t, got, 1, "only the broken alternative; the kill still works")
	require.Contains(t, got[0], "walk-dead-end 53 quest 53 trigger room_enter")
}

func TestWalk_HandInGateTheOfferCannotMeet(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[1].QuestExcluded = []string{"53-firstkill"} // hand-in hides itself
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step end")
}

func TestWalk_GuardCanFail(t *testing.T) {
	// The clean world must stay clean, and every mutation above must be
	// caught: this is the proof the walk is capable of failing.
	require.Empty(t, Walk(walkWorld(), questloop.Sources{}))
	w := walkWorld()
	w.Quests[0].Triggers = nil
	require.NotEmpty(t, Walk(w, questloop.Sources{}))
}
```

`Walk` reports a failing Way under its own Where and a step with no working Way under
`step <id>`. In `TestWalk_UnreachableStepIsADeadEnd` the one Way fails, so both the Way key
and `walk-dead-end 53 step firstkill` appear; the test checks the step key.

- [ ] **Step 2: Run them**

Run: `go test ./internal/questengine/ -run TestWalk -v`
Expected: FAIL to compile (`undefined: Walk`).

- [ ] **Step 3: Implement** `walkthrough.go`

```go
package questengine

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Walk drives every quest in w through the real trigger engine and the real
// dialogue topics, from its offer to its hand-in (spec 7), and reports what
// cannot be completed: walk-dead-end, walk-no-source, walk-unspawned. Findings
// share questloop's shape and backlog. A quest with no offer is skipped: the
// start rule reports it.
func Walk(w questloop.World, src questloop.Sources) []questloop.Finding {
	eng := NewEngine()
	steps := map[int][]string{}
	for _, q := range w.Quests {
		if q == nil {
			continue
		}
		eng.RegisterQuest(q)
		for _, s := range q.Steps {
			steps[q.QuestId] = append(steps[q.QuestId], s.Id)
		}
	}
	ways := questloop.WaysInto(w)
	var out []questloop.Finding
	for _, q := range w.Quests {
		if q == nil || len(q.Steps) == 0 {
			continue
		}
		out = append(out, walkQuest(eng, q, steps, ways, src)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return dedupeFindings(out)
}

func stepToken(q *quests.Quest, i int) string { return fmt.Sprintf("%d-%s", q.QuestId, q.Steps[i].Id) }

func walkQuest(eng *Engine, q *quests.Quest, steps map[int][]string, ways map[string][]questloop.Way, src questloop.Sources) []questloop.Finding {
	var out []questloop.Finding
	dead := func(where, msg string) {
		out = append(out, questloop.Finding{Rule: "walk-dead-end", QuestId: q.QuestId, Where: where, Msg: msg})
	}

	var offer *questloop.Way
	for _, wy := range ways[stepToken(q, 0)] {
		if wy.Topic != nil && wy.Topic.Kind == dialogue.KindOffer {
			wy := wy
			offer = &wy
			break
		}
	}
	if offer == nil {
		return nil
	}

	p := newWalkPlayer(steps)
	out = append(out, checkWay(q, *offer, src)...)
	meetTopic(p, offer.Topic)
	if !fireTopic(p, *offer) || p.progress[q.QuestId] != q.Steps[0].Id {
		dead(offer.Where, "the offer cannot be taken")
		return out
	}

	for i := 1; i < len(q.Steps); i++ {
		tok, want := stepToken(q, i), q.Steps[i].Id
		var next *walkPlayer
		for _, wy := range ways[tok] {
			if wy.Reward {
				continue
			}
			out = append(out, checkWay(q, wy, src)...)
			c := p.clone()
			ok := false
			switch {
			case wy.Trigger != nil:
				ok = fireTrigger(eng, c, wy.Trigger)
			case wy.Topic != nil:
				meetTopic(c, wy.Topic)
				ok = fireTopic(c, wy)
			}
			got := c.progress[q.QuestId]
			switch {
			case !ok || got == q.Steps[i-1].Id:
				dead(wy.Where, "does not reach "+tok)
			case got != want:
				dead(wy.Where, fmt.Sprintf("skips to %d-%s", q.QuestId, got))
			default:
				if next == nil {
					next = c
				}
			}
		}
		if next == nil {
			dead("step "+want, "no way in reaches "+tok)
			return out
		}
		p = next
	}
	return out
}

// meetTopic gives p what a topic's gates ask for.
func meetTopic(p *walkPlayer, t *dialogue.Topic) {
	for _, tok := range t.QuestRequired {
		if !p.HasQuest(tok) {
			p.hold(tok)
		}
	}
	for k, v := range t.QuestFlagRequired {
		p.flags[k] = v
	}
	if t.RequiresItem > 0 {
		p.items[t.RequiresItem] = true
	}
	if t.CarriesItem > 0 {
		p.items[t.CarriesItem] = true
	}
	if t.GoldRequired > p.gold {
		p.gold = t.GoldRequired
	}
	if t.MasterworkRequired > 0 {
		p.masterwork = true
	}
}

func fireTopic(p *walkPlayer, wy questloop.Way) bool {
	ps := p.dialogueState()
	if wy.File == nil || !wy.File.Eligible(wy.Topic, ps) {
		return false
	}
	return wy.File.Apply(wy.Topic, ps)
}

// fireTrigger meets the trigger's conditions on p, then sends the event its
// filters describe through the engine.
func fireTrigger(eng *Engine, p *walkPlayer, t *quests.TriggerDef) bool {
	c := t.Conditions
	for _, tok := range c.Has {
		if !p.HasQuest(tok) {
			p.hold(tok)
		}
	}
	if c.HasItem > 0 {
		p.items[c.HasItem] = true
	}
	for k, v := range c.HasFlag {
		p.flags[k] = v
	}
	if c.HasGold > p.gold {
		p.gold = c.HasGold
	}
	if c.HasMasterwork > 0 {
		p.masterwork = true
	}
	if c.InRoom > 0 {
		p.roomId = c.InRoom
	}
	if t.Item > 0 && (t.Event == "item_give" || t.Event == "item_gain") {
		p.items[t.Item] = true
	}
	d := EventDetails{RoomId: t.Room, MobId: t.Mob, ItemId: t.Item, Skill: t.Skill,
		Command: t.Command, Noun: t.Noun, Verb: t.Verb, QuestToken: t.QuestToken}
	if d.RoomId == 0 {
		d.RoomId = c.InRoom
	}
	if d.RoomId != 0 {
		p.roomId = d.RoomId
	}
	return eng.Notify(t.Event, d, p, &walkCtx{p: p}).Handled
}

func dedupeFindings(fs []questloop.Finding) []questloop.Finding {
	seen := map[string]bool{}
	var out []questloop.Finding
	for _, f := range fs {
		if !seen[f.Key()] {
			seen[f.Key()] = true
			out = append(out, f)
		}
	}
	return out
}
```

`checkWay` arrives in Task 4. For this task add it as a stub at the bottom of
`walkthrough.go`: `func checkWay(*quests.Quest, questloop.Way, questloop.Sources) []questloop.Finding { return nil }`.

Notes the implementer must verify against source, not assume:
- `fireTopic` for a hand-in topic with `RequiresItem`: `Apply` takes the item through
  `RemoveItem`; `meetTopic` gave it first. Confirm `Apply` calls `RemoveItem` (read
  `internal/dialogue/engine.go` `applyQuestEffects`).
- A `quest_granted` trigger chained off this quest's own earlier grant has already fired
  inside `Notify` or `Apply`'s grant: the walk's `walkCtx.GrantQuest` and `dialogueState`'s
  `GiveQuest` call `grant`, but the dialogue path does not chain `quest_granted` (in the game
  the hook does it, `Quest_HandleQuestUpdate.go`). So after `fireTopic` succeeds, also run
  `eng.Notify("quest_granted", EventDetails{QuestToken: topic.GrantsQuest}, p, &walkCtx{p: p})`
  so steps that a `quest_granted` trigger finishes are walked as the game runs them. Add it
  inside `fireTopic` after a successful `Apply` when `GrantsQuest != ""`, and a test:
  a quest whose `firstkill` step is granted by a `quest_granted` trigger on `53-start`
  finishes cleanly.

- [ ] **Step 4: Run**

Run: `go test ./internal/questengine/ -run TestWalk -v` then `go test ./internal/questengine/`
Expected: PASS, then `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/walkthrough.go internal/questengine/walkthrough_test.go
git commit -F - <<'EOF'
feat(questengine): headless walkthrough drives each quest from offer to hand-in (#375 phase 2b)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Item-source and spawn checks in the walk

**Files:**
- Modify: `internal/questengine/walkthrough.go` (replace the `checkWay` stub)
- Modify: `internal/questengine/walkthrough_test.go`

- [ ] **Step 1: Write the failing tests** (append)

```go
func srcWith(items map[int][]string, spawned map[int]bool) questloop.Sources {
	return questloop.Sources{
		ItemSources: func(id int) []string { return items[id] },
		MobSpawned:  func(id int) bool { return spawned[id] },
	}
}

func TestWalk_ItemWithNoSource(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.HasItem = 777
	got := keys(Walk(w, srcWith(nil, map[int]bool{9110: true, 9112: true})))
	require.Contains(t, got, "walk-no-source 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill")
}

func TestWalk_ItemWithASourcePasses(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.HasItem = 777
	require.Empty(t, Walk(w, srcWith(map[int][]string{777: {"shop 9123"}}, map[int]bool{9110: true, 9112: true})))
}

func TestWalk_UnspawnedMobAndNPC(t *testing.T) {
	got := keys(Walk(walkWorld(), srcWith(nil, map[int]bool{})))
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill")
	require.Contains(t, got, "walk-unspawned 53 mob 9112 topic cull_offer")
}

func TestWalk_HandInItemNeedsASource(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[1].RequiresItem = 4242
	got := keys(Walk(w, srcWith(nil, map[int]bool{9110: true, 9112: true})))
	require.Contains(t, got, "walk-no-source 53 mob 9112 topic cull_in")
}
```

The trigger Where strings must match `questloop`'s exactly; read
`internal/questloop/grants.go` `triggerSignature` and adjust the literals to what it produces
for this trigger before running (run `go test` once and read the actual key from the failure).

- [ ] **Step 2: Run them**

Run: `go test ./internal/questengine/ -run 'TestWalk_Item|TestWalk_Unspawned|TestWalk_HandIn' -v`
Expected: FAIL (the stub reports nothing).

- [ ] **Step 3: Implement** `checkWay` (replace the stub)

```go
// checkWay reports a Way's needs the world cannot meet (spec 7.2): an item
// nothing provides, a mob or NPC nothing spawns. A nil Sources func skips
// that check.
func checkWay(q *quests.Quest, wy questloop.Way, src questloop.Sources) []questloop.Finding {
	var out []questloop.Finding
	needItem := func(id int, what string) {
		if id > 0 && src.ItemSources != nil && len(src.ItemSources(id)) == 0 {
			out = append(out, questloop.Finding{Rule: "walk-no-source", QuestId: q.QuestId, Where: wy.Where,
				Msg: fmt.Sprintf("needs item %d (%s), which no mob, shop, room, forage table, recipe, quest or topic provides", id, what)})
		}
	}
	needMob := func(id int, what string) {
		if id > 0 && src.MobSpawned != nil && !src.MobSpawned(id) {
			out = append(out, questloop.Finding{Rule: "walk-unspawned", QuestId: q.QuestId, Where: wy.Where,
				Msg: fmt.Sprintf("needs mob %d (%s), which no room spawns and no quest spawns", id, what)})
		}
	}
	switch {
	case wy.Trigger != nil:
		t := wy.Trigger
		needItem(t.Conditions.HasItem, "has_item")
		if t.Event == "item_give" || t.Event == "item_gain" {
			needItem(t.Item, t.Event)
		}
		if t.Event == "mob_death" || t.Event == "item_give" {
			needMob(t.Mob, t.Event)
		}
	case wy.Topic != nil:
		needItem(wy.Topic.RequiresItem, "requiresItem")
		needItem(wy.Topic.CarriesItem, "carriesItem")
		needMob(wy.MobId, "the NPC who has this topic")
	}
	return out
}
```

- [ ] **Step 4: Run**

Run: `go test ./internal/questengine/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/walkthrough.go internal/questengine/walkthrough_test.go
git commit -F - <<'EOF'
feat(questengine): walkthrough checks item sources and spawns (#375 phase 2b)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: `questworld.Sources()` over the live registries

**Files:**
- Create: `internal/questworld/sources.go`, `internal/questworld/sources_test.go`

- [ ] **Step 1: Confirm the imports make no cycle**

Run: `go list -deps ./internal/forager ./internal/crafting ./internal/mobs ./internal/rooms | grep -E "internal/(questworld|questengine|conversation)$"`
Expected: nothing printed.

- [ ] **Step 2: Implement** `sources.go`

```go
package questworld

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Sources indexes, once, where each item can come from and which mobs are
// placed in the world, from the loaded registries (spec 7.2). A mob's items,
// equipment, loot pool and shop stock count only when that mob is spawned.
func Sources() questloop.Sources {
	itemSrc := map[int][]string{}
	add := func(id int, where string) {
		if id > 0 {
			itemSrc[id] = append(itemSrc[id], where)
		}
	}
	spawned := map[int]bool{}

	for _, roomId := range rooms.GetAllRoomIds() {
		r := rooms.LoadRoomTemplate(roomId)
		if r == nil {
			continue
		}
		for _, si := range r.SpawnInfo {
			if si.MobId > 0 {
				spawned[si.MobId] = true
			}
			add(si.ItemId, fmt.Sprintf("room %d spawn", roomId))
		}
		for _, it := range r.Items {
			add(it.ItemId, fmt.Sprintf("room %d floor", roomId))
		}
		for name, c := range r.Containers {
			for _, it := range c.Items {
				add(it.ItemId, fmt.Sprintf("room %d %s", roomId, name))
			}
			for made := range c.Recipes {
				add(made, fmt.Sprintf("room %d %s recipe", roomId, name))
			}
		}
	}

	for _, q := range quests.GetAllQuests() {
		walkActions(q, func(a quests.ActionDef) {
			add(a.GiveItem, fmt.Sprintf("quest %d give_item", q.QuestId))
			if a.SpawnItem != nil {
				add(a.SpawnItem.Id, fmt.Sprintf("quest %d spawn_item", q.QuestId))
			}
			if a.SpawnMob != nil && a.SpawnMob.Id > 0 {
				spawned[a.SpawnMob.Id] = true
			}
		})
		add(q.Rewards.ItemId, fmt.Sprintf("quest %d reward", q.QuestId))
		for _, g := range quests.ParseItemGrants(q.Rewards.ItemInfo) {
			add(g.ItemId, fmt.Sprintf("quest %d reward", q.QuestId))
		}
	}

	for _, m := range mobs.AllMobTemplates() {
		if m == nil || !spawned[int(m.MobId)] {
			continue
		}
		who := fmt.Sprintf("mob %d", m.MobId)
		for _, it := range m.Character.Items {
			add(it.ItemId, who+" carried")
		}
		for _, it := range m.Character.Equipment.GetAllItems() {
			add(it.ItemId, who+" worn")
		}
		for _, id := range m.LootPool {
			add(id, who+" loot")
		}
		for _, s := range m.Character.Shop {
			add(s.ItemId, fmt.Sprintf("shop %d", m.MobId))
		}
	}

	files, _ := dialogue.LoadAllFiles()
	for _, df := range files {
		for _, t := range df.Topics {
			add(t.GivesItem, fmt.Sprintf("mob %d topic %s", df.MobId, t.Key))
		}
	}

	for _, table := range []map[string][]int{forager.ForageYields, forager.NightForageYields, forager.ZoneForageYields, forager.StormForageYields} {
		for place, ids := range table {
			for _, id := range ids {
				add(id, "forage "+place)
			}
		}
	}
	for id, r := range crafting.GetAll() {
		add(r.Output.ItemId, "recipe "+id)
	}

	for id := range itemSrc {
		sort.Strings(itemSrc[id])
	}
	return questloop.Sources{
		ItemSources: func(id int) []string { return itemSrc[id] },
		MobSpawned:  func(id int) bool { return spawned[id] },
	}
}

// walkActions visits every action of q's triggers, sequence on_complete
// actions included.
func walkActions(q quests.Quest, fn func(quests.ActionDef)) {
	var walk func([]quests.ActionDef)
	walk = func(as []quests.ActionDef) {
		for _, a := range as {
			fn(a)
			if a.Sequence != nil {
				walk(a.Sequence.OnComplete)
			}
		}
	}
	for _, t := range q.Triggers {
		walk(t.Actions)
	}
}
```

Verify each field and type by reading source before building: `m.MobId`'s type (`mobs.MobId`,
converted with `int(...)`), `GetAllItems` returning `[]items.Item`, `Character.Shop` element
`ItemId`, the four forager maps' types, `crafting.GetAll` returning a map keyed by recipe id.
If a mob placed by another mechanism (a `systemNPCAnchorRooms` force spawn in `main.go`, a
caravan, a schedule) is reported unspawned in Task 6, read how it is placed and add that
source here with a comment, rather than allow-listing the mob.

- [ ] **Step 3: Test** `sources_test.go`: the registries are global, so this test loads the
  shipped world like the smoke tests do. Put it in the root smoke file instead if
  `questworld` cannot load data in its own test binary (check how
  `quest_loop_smoke_test.go` calls `loadAllDataFiles`, a `package main` function). Preferred:
  a root test `TestSmoke_WalkSourcesIndexShippedWorld` in `quest_loop_smoke_test.go`, gated
  on `DOGMUD_BOOT_SMOKE`, that asserts:
  - `Sources().MobSpawned(9112)` (Caravan Guard Garve) is true;
  - a known shop item has a `shop` source (pick one from a shipped shop mob's YAML by reading
    it, and name the file in a comment);
  - a known recipe output has a `recipe` source;
  - a known forage item has a `forage` source (read `forage_core.go` for one id);
  - item id `999999999` has no source (proves an empty answer is possible).

- [ ] **Step 4: Run**

Run: `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_WalkSourcesIndexShippedWorld -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/questworld/sources.go quest_loop_smoke_test.go
git commit -F - <<'EOF'
feat(questworld): item-source and spawn index for the walkthrough (#375 phase 2b)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Run the walk in CI and regenerate the backlog

**Files:**
- Modify: `quest_loop_smoke_test.go`, `_datafiles/world/dogmud/quests/loop-backlog.txt`

- [ ] **Step 1: Combine the findings.** In `TestSmoke_ShippedQuestsPassTheLoop`, the findings
  become `Validate(Registry())` plus `questengine.Walk(Registry(), questworld.Sources())`
  (build the registry world once). Both the backlog writer branch and the ratchet branch use
  the combined list. Ratchet against the backlog with the combined list, so walk lines are
  never stale in CI. The boot door (`main_quests.go`) stays on `Validate` only, and it already
  ignores stale lines, so walk lines in the backlog do not affect boot. Add a comment saying
  so in both files.

- [ ] **Step 2: First run, read before writing.** Generate into the scratchpad:

```bash
DOGMUD_BOOT_SMOKE=1 DOGMUD_LOOP_BACKLOG_WRITE="<scratchpad>/loop-backlog.txt" go test . -run TestSmoke_ShippedQuestsPassTheLoop -v 2>&1 | tail -3
grep -v "^#" "<scratchpad>/loop-backlog.txt" | cut -d' ' -f1 | sort | uniq -c
```

Cross-check against what is already known before copying anything (a large mismatch means a
walk bug, not content: stop and report with samples):
- `walk-dead-end` should include the 10 steps the spec census found with no way in
  (14-report, 28-end, 30-rite, 30-end, 32-consider, 63-report, 65-report, 66-escort,
  66-detox, 67-report), unless a quest has no offer (then it is skipped, and its `start`
  finding stands). Quests the `end` rule flags for finishing on a trigger must NOT be dead
  ends: a trigger finish still finishes.
- `walk-unspawned`: read every line. Each must be a mob no room spawns; check two by grepping
  `mobid: <id>` across `_datafiles/world/dogmud/rooms`. A system NPC placed another way is a
  Sources gap (Task 5 note), not content.
- `walk-no-source`: read every line; check two by grepping the item id across the world data.
- Spot-check three quests with NO walk finding (for example 53, 62, 72) are genuinely
  completable by reading their files.

- [ ] **Step 3: Copy in and run the real sweep**

```bash
cp "<scratchpad>/loop-backlog.txt" _datafiles/world/dogmud/quests/loop-backlog.txt
DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_' -v 2>&1 | grep -E "quest loop|PASS|FAIL" | tail -8
```

Expected: PASS, `0 new, 0 stale`, with the known count now covering the walk lines.

- [ ] **Step 4: Boot check.** Boot from a detached worktree on a private port as
  `dogmud-shipping` describes, confirm the backlog warning and no panic (walk lines are
  ignored at boot), stop only your own PID.

- [ ] **Step 5: Commit**

```bash
git add quest_loop_smoke_test.go _datafiles/world/dogmud/quests/loop-backlog.txt
git commit -F - <<'EOF'
feat(quests): the CI sweep walks every quest; backlog gains the walk findings (#375 phase 2b)

Walk findings: <per-rule counts from Step 2>.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Docs

- [ ] **Step 1:** `internal/questengine/context.md`: a "Walkthrough" section: `Walk`,
  `walkPlayer` (real step order), `walkCtx` (no-op except grants, items, gold, flags, and
  sequence `on_complete` if Task 2 found the bridge runs them), the three rules, that it is
  CI-only (not boot), and the deviation from spec 7.1 (own player type, not the `_test.go`
  doubles).
- [ ] **Step 2:** `internal/questloop/context.md`: `Way`, `WaysInto`, `Sources`; the walk
  rules share the backlog; boot ignores walk lines.
- [ ] **Step 3:** `internal/questworld/context.md`: `Sources()` and what it counts.
- [ ] **Step 4:** Spec section 7: note the player-type deviation and the CI-only scope in
  the live text.
- [ ] **Step 5:** Roadmap phase 2: plan 2b line points at this plan; Status "2b implemented on
  `feat/375-phase2b-walkthrough` (not yet merged)". `docs/README.md`: a row for this plan.
- [ ] **Step 6:** Verify every symbol named exists (`grep`), run `python tools/context_md_audit.py`
  and expect nothing for the three packages, then commit by explicit paths.

---

### Task 8: Gate and PR

- [ ] **Step 1:** `go build ./... && go vet ./... && go test ./... 2>&1 | grep -v "^ok\|no test files"`,
  `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_'`, `golangci-lint run --new-from-merge-base=origin/master`.
  All green; report any failure with output.
- [ ] **Step 2:** An Opus review over the branch diff against spec 7 and this plan's facts
  table; fix what it confirms, with tests.
- [ ] **Step 3:** No playtest: the walkthrough changes nothing a player sees. Say so in the PR.
- [ ] **Step 4:** If #666 has merged, rebase onto master first. Push and open the PR with
  `gh pr create --repo pruuk/DOGMud`, base `master` if #666 has merged, otherwise base
  `feat/375-phase2a-loop` (a stacked PR) and say so in the body. Body: what the walk checks,
  the walk-finding counts, the deviation, deploy notes ("none: CI only"). No closing keyword
  for #375. Do not merge.
