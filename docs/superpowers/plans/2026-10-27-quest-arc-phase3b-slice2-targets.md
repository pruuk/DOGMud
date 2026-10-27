# #375 phase 3b slice 2: targets on command events, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every quest `command` event says what the command was aimed at (mob template, item,
player, self, container or nothing), through one helper that a guard holds every site to, so a
quest step can ask for "throw at the boar" rather than "throw".

**Architecture:** A new `questengine.NotifyCommand(user, roomId, command, targets...)` builds one
`EventDetails` per target (template room, `MobId`, `ItemId`, `Noun`, new `TargetKind`) and raises
them under one evaluation guard (`Engine.NotifyAll`), so a trigger fires at most once per command
however many targets it matches. A new trigger field `target:` filters on the kind; the existing
`mob:` and `item:` filters now see command targets. The 32 hand-built `Notify("command", ...)`
sites move onto the helper; steal and plant on a player, which raised nothing, gain the event.

**Tech Stack:** Go, `internal/quests` (trigger data and loader validation), `internal/questengine`
(matching, helper), `internal/questloop` (signatures), `internal/guards` (AST call guard).

**Spec:** `docs/superpowers/specs/2026-10-27-quest-arc-phase3b-trigger-kinds-design.md`, section 2.

**Rules for every implementer (global, enforced by hooks):** edit files only with the Edit and
Write tools, never with shell redirection, `sed -i` or Python file writes. `git add` named paths
only. Never `--no-verify`. No em or en dashes in prose or comments. Kill only processes you
started, by PID.

---

## Facts verified against source (master `906a97ae0` plus spec commit `a9ec734d3`, 2026-10-27)

| Fact | Where |
|---|---|
| 32 production `Notify("command", ...)` calls, listed in Task 6 to Task 9 with file and line | grep `Notify("command",` over `internal`, `modules` (0 in modules) |
| `Engine.Notify` builds one `EvalGuard` per call; `evaluate` marks a trigger visited before checking its conditions, so a shared guard lets each trigger fire once | `internal/questengine/engine.go:74-94, 99-112`; `guards.go:44` |
| `matchTriggerFields` already filters `mob` on `d.MobId` and `item` on `d.ItemId` for every event | `internal/questengine/engine.go:216-224` |
| `EventDetails` has no target kind field | `internal/questengine/types.go:39-50` |
| The loader refuses `verb` on anything but `room_interact`; the target check goes beside it | `internal/quests/quests.go:113-139` |
| The walkthrough builds a trigger's synthetic event from its fields | `internal/questengine/walkthrough.go:704-705` |
| `questloop` keys a trigger by `triggerSignature` | `internal/questloop/grants.go:99-118` |
| Shipped `command` triggers with `item:` or `mob:`: equip (quests 31, 32, 50), sell (quest 35, `mob: 9116`, `item: 10009`); every other shipped `command` trigger filters only `room`, `noun` or nothing | grep of `quests/*.yaml` |
| Equip already sends `ItemId: result.Item.ItemId`; sell sends `ItemId: item.ItemId` and `MobId: int(mob.MobId)` | `internal/usercommands/equip.go:213-218`; `internal/actions/sell.go:431-441` |
| Flee already matches on the template room (`rooms.OriginalRoomId`) with the bridge on the real id | `internal/hooks/NewRound_DoCombat_helpers.go:1069-1076` |
| `rooms.OriginalRoomId(id)` returns `(id, false)` for a normal room | `internal/rooms/ephemeral.go:269-274` |
| Combat move results carry `actions.AggroTarget{UserId, MobInstanceId}` (kick and trip `res.Target`) | `internal/actions/combat_helpers.go:12-18`; `usercommands/kick.go:52` |
| Taunt has `result.Target.UserId` / `.MobInstanceId`; consider has an `actions.Actor` with `GetUserId()` / `GetMobInstanceId()` | `usercommands/taunt.go:59-81`; `actions/actor.go:39,43` |
| Fire results carry `TargetUserId`, `TargetMobInstanceId`, `IsTargetMob` | `internal/actions/combat_fire.go:29-32` |
| Cast results carry `TargetUserIds []int`, `TargetMobInstanceIds []int`; the notify passes `Noun: spellInfo.SpellId` | `internal/actions/cast.go:59-61`; `usercommands/skill.cast.go:383-392` |
| Throw raises its event BEFORE the area loop (`throw.go:296-302`); the loop (`:355-493`) has no return, only `break` on a fumble (`:400`) | `internal/usercommands/throw.go` |
| Buy's notify is in `postSuccessBookkeeping(buyer, shopMob, shopUser, awardProgression)` (3 callers, `buy.go:414, 437, 449`), which has no item; `BuyResult` has no item field | `internal/actions/buy.go:44-50, 837-868` |
| Craft: `notifyCraftQuest(user, room)` (callers `usercommands/craft.go:195, 771`); the timed site (`hooks/NewRound_UserRoundTick.go:640-653`) follows an enchant branch (`targetItem`) and an output branch (`newItem := items.New(recipe.Output.ItemId)`) | as cited |
| `stealFromPlayer` (`actions/steal.go:423`) and `plantOnPlayer` (`actions/plant.go:261`) raise no command event; their mob and container siblings do | grep |
| Guards: `walkGoCalls(t, names)` returns `goCall{file, line, call *ast.CallExpr, name}` for every non-test call by name | `internal/guards/blank_line_guard_test.go:96-140` |
| Test mocks: `newFullMockPlayer`, `newFullMockActionContext`, `QuestDef`/`TriggerDef` aliases, `NewEngine().RegisterQuest` | `internal/questengine/engine_test.go:9-60, 408-433` |

## Spec amendments made by this plan (land in Task 11)

1. The helper is `NotifyCommand(user, roomId, command, targets ...Target)`, not `CommandEvent`; it
   raises the event as well as building it.
2. Kind `container` joins the kinds: steal and plant aimed at a room container name it in `Noun`.
3. A command with several targets raises one event per target under one guard: each trigger fires
   at most once per command.
4. Throw raises one event per mob its blast contested (hit or not), since the command event is the
   attempt; `spell_landed` (slice 3) is the "it took" event.
5. No mob-actor site raises a command event today, so nothing is added for mobs; the event shape
   is one.
6. Steal and plant on a player gain the event (they raised none).

## File structure

| File | Change |
|---|---|
| `internal/quests/triggers.go` | `Target` field; target kind constants |
| `internal/quests/quests.go` | loader check for `target` |
| `internal/quests/target_test.go` | new: loader tests |
| `internal/questengine/types.go` | `EventDetails.TargetKind` |
| `internal/questengine/engine.go` | `target` match; `NotifyAll`; `Notify` delegates |
| `internal/questengine/command_event.go` | new: `Target`, constructors, `CommandDetails`, `NotifyCommand` |
| `internal/questengine/command_event_test.go` | new |
| `internal/questengine/walkthrough.go` | synthetic event carries `TargetKind` |
| `internal/questloop/grants.go` | signature includes `target` |
| 21 production files with command sites | move onto `NotifyCommand` |
| `internal/guards/command_event_guard_test.go` | new: call guard with a must-fail case |
| `internal/questengine/context.md`, `internal/quests/context.md`, `.claude/skills/dogmud-authoring-quests/SKILL.md`, `modules/gmcp/gmcp.Quest.go`, the spec, `docs/README.md` | docs |

---

### Task 0: Worktree

- [ ] **Step 1: Create the worktree from the spec branch**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/375-3b-targets "C:/tmp/dogmud-375-3b2" docs/375-phase3b-spec
```

- [ ] **Step 2: Record it** in the session handoff notes as `C:/tmp/dogmud-375-3b2` (the tmp sweep keeps referenced paths). All later tasks run inside `C:/tmp/dogmud-375-3b2`.

- [ ] **Step 3: Baseline**

Run: `go build ./... && go test ./internal/quests/ ./internal/questengine/ ./internal/questloop/ ./internal/guards/`
Expected: PASS. If anything fails on the baseline, stop and report it; do not fix unrelated failures.

---

### Task 1: The `target` trigger field and its loader check

**Files:**
- Modify: `internal/quests/triggers.go:11-23`
- Modify: `internal/quests/quests.go` (after the verb block, about line 139)
- Create: `internal/quests/target_test.go`

- [ ] **Step 1: Write the failing test**

`internal/quests/target_test.go` (uses the package's existing `validQuest` helper, the same one
`narration_test.go:65` uses; its first trigger is a `command` trigger):

```go
package quests

import (
	"strings"
	"testing"
)

func TestValidateCommandTargets(t *testing.T) {
	for _, kind := range TriggerTargetKinds {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Target = kind
		if err := q.Validate(); err != nil {
			t.Fatalf("target %q refused: %v", kind, err)
		}
	}
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Target = TargetNone
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), `target "none"`) {
		t.Fatalf("target none is an event value, not a filter; got %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Target = "boar"
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), `target "boar"`) {
		t.Fatalf("target boar must be refused, got %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Target = "mob_death", TargetMob
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only command has a target") {
		t.Fatalf("a target on a mob_death trigger must be refused, got %v", err)
	}
}
```

(`validQuest`, `internal/quests/narration_test.go:40`, builds one `Event: "command"` trigger.)

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/quests/ -run TestValidateCommandTargets`
Expected: FAIL to compile (`Target`, `TriggerTargetKinds`, `TargetNone`, `TargetMob` undefined).

- [ ] **Step 3: Add the field and constants**

In `internal/quests/triggers.go`, add to `TriggerDef` after `Verb`:

```go
	Target     string      `yaml:"target,omitempty" json:"target,omitempty"`           // command target kind filter: self, mob, player, item, container (#375 3b)
```

and update the `Event` comment's list if it is stale. Below the `InteractVerb` function add:

```go
// The kinds of thing a command event names as its target (#375 phase 3b
// slice 2). Every command event carries one; TargetNone is an event value
// only, so a trigger that does not care about the target omits the filter.
const (
	TargetNone      = "none"
	TargetSelf      = "self"
	TargetMob       = "mob"
	TargetPlayer    = "player"
	TargetItem      = "item"
	TargetContainer = "container"
)

// TriggerTargetKinds are the values a trigger's target filter may take.
var TriggerTargetKinds = []string{TargetSelf, TargetMob, TargetPlayer, TargetItem, TargetContainer}
```

- [ ] **Step 4: Add the loader check**

In `internal/quests/quests.go`, directly after the closing brace of the `if t.Verb != "" {` block:

```go
		// Only a command event names a target (#375 phase 3b slice 2).
		if t.Target != "" {
			if t.Event != "command" {
				return fmt.Errorf("quest %d (%s): trigger %d sets target %q on a %s trigger; only command has a target", r.QuestId, r.Name, i, t.Target, t.Event)
			}
			if !slices.Contains(TriggerTargetKinds, t.Target) {
				return fmt.Errorf("quest %d (%s): trigger %d has invalid target %q (use one of %s)", r.QuestId, r.Name, i, t.Target, strings.Join(TriggerTargetKinds, ", "))
			}
		}
```

Add `"slices"` and `"strings"` to the imports if the file lacks them.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/quests/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/quests/triggers.go internal/quests/quests.go internal/quests/target_test.go
git commit -F - <<'EOF'
feat(quests): target filter on command triggers (#375)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Match on the target kind; walkthrough and signature carry it

**Files:**
- Modify: `internal/questengine/types.go:39-50`
- Modify: `internal/questengine/engine.go:216-250`
- Modify: `internal/questengine/walkthrough.go:704-705`
- Modify: `internal/questloop/grants.go:118`
- Test: `internal/questengine/command_event_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/questengine/command_event_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

// seedTargetQuest: step 1 needs a throw at mob 9112, step 2 any throw.
func seedTargetQuest() *Engine {
	q := &QuestDef{
		QuestId: 300,
		Name:    "Target Test Quest",
		Steps: []QuestStep{
			{Id: "300-start", Description: "Begin"},
			{Id: "300-boar", Description: "Throw at the boar"},
			{Id: "300-any", Description: "Throw at anything"},
		},
		Triggers: []TriggerDef{
			{
				Event: "command", Command: "throw", Target: quests.TargetMob, Mob: 9112,
				Conditions: Conditions{Has: []string{"300-start"}, Missing: []string{"300-boar"}},
				Actions:    []ActionDef{{Grant: "300-boar"}},
			},
			{
				Event: "command", Command: "throw",
				Conditions: Conditions{Has: []string{"300-boar"}, Missing: []string{"300-any"}},
				Actions:    []ActionDef{{Grant: "300-any"}},
			},
		},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	return e
}

func TestTargetFilterNeedsTheNamedMob(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// A throw at another mob does not count.
	r := e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 7}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"), "wrong mob advanced the step: %+v", r)

	// A throw at nothing does not count.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetNone}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))

	// A throw at the boar does.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"))
}

func TestTargetFilterKindMustMatch(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// MobId set but the kind is item (sell's shopkeeper rides along on an item target).
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetItem, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/questengine/ -run TestTargetFilter`
Expected: FAIL to compile (`TargetKind` undefined).

- [ ] **Step 3: Add the field and the match**

In `internal/questengine/types.go`, add to `EventDetails` after `Verb`:

```go
	TargetKind string // command events: what the command was aimed at, a quests.Target* kind (#375 3b)
```

In `internal/questengine/engine.go` `matchTriggerFields`, after the `QuestToken` check:

```go
	if t.Target != "" && t.Target != d.TargetKind {
		return false
	}
```

In `internal/questengine/walkthrough.go:704-705`, add `TargetKind: t.Target` to the literal:

```go
	d := EventDetails{RoomId: t.Room, MobId: t.Mob, ItemId: t.Item, Skill: t.Skill,
		Command: t.Command, Noun: t.Noun, QuestToken: t.QuestToken, TargetKind: t.Target}
```

In `internal/questloop/grants.go` `triggerSignature`, after `addStr("verb", t.Verb)`:

```go
	addStr("target", t.Target)
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/questengine/ ./internal/questloop/`
Expected: PASS (the walkthrough and questloop tests over shipped quests stay green: no shipped
trigger sets `target`).

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/types.go internal/questengine/engine.go internal/questengine/walkthrough.go internal/questloop/grants.go internal/questengine/command_event_test.go
git commit -F - <<'EOF'
feat(questengine): match command triggers on the target kind (#375)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `NotifyAll`, one guard over several details

**Files:**
- Modify: `internal/questengine/engine.go:70-94`
- Test: `internal/questengine/command_event_test.go`

- [ ] **Step 1: Write the failing test** (append to `command_event_test.go`)

```go
func TestNotifyAllFiresEachTriggerOnce(t *testing.T) {
	q := &QuestDef{
		QuestId: 301,
		Name:    "Once Per Command",
		Steps:   []QuestStep{{Id: "301-start"}, {Id: "301-thrown"}},
		Triggers: []TriggerDef{{
			Event: "command", Command: "throw",
			Conditions: Conditions{Has: []string{"301-start"}},
			Actions:    []ActionDef{{GiveGold: 5}},
		}},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	player := newFullMockPlayer(100)
	player.quests["301-start"] = true
	ctx := newFullMockActionContext(1, player)

	three := []EventDetails{
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 1},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 2},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 3},
	}
	r := e.NotifyAll("command", three, player, ctx)
	assert.True(t, r.Handled)
	assert.Equal(t, 5, ctx.givenGold, "an untargeted trigger must fire once per command, not once per target")
}

func TestNotifyAllMatchesALaterTarget(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true
	details := []EventDetails{
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 7},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 9112},
	}
	e.NotifyAll("command", details, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"), "the boar was the second target")
}
```

(`mockActionContext.GiveGold` accumulates into `givenGold`, `internal/questengine/actions_test.go:60`;
`fullMockActionContext` embeds it.)

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/questengine/ -run TestNotifyAll`
Expected: FAIL to compile (`NotifyAll` undefined).

- [ ] **Step 3: Implement**

Replace `Notify` in `internal/questengine/engine.go` (lines 70-94) with:

```go
// Notify is the main entry point. It evaluates all triggers matching the given
// event type and returns a result indicating whether anything fired and whether
// an item should be consumed.
func (e *Engine) Notify(eventType string, details EventDetails, player PlayerState, ctx ActionContext) NotifyResult {
	return e.NotifyAll(eventType, []EventDetails{details}, player, ctx)
}

// NotifyAll raises one event for several details under ONE guard, so each
// trigger fires at most once however many of the details it matches: a
// command aimed at three mobs advances an untargeted step once, and a step
// that names one of them still fires on that one (#375 phase 3b slice 2).
func (e *Engine) NotifyAll(eventType string, details []EventDetails, player PlayerState, ctx ActionContext) NotifyResult {
	bal := configs.GetBalanceConfig()
	maxDepth := int(bal.QuestChainDepthLimit)
	warnMs := int64(bal.QuestPerformanceWarnMs)

	guard := NewEvalGuard(maxDepth)
	start := time.Now()

	result := NotifyResult{}
	userId := 0
	for _, d := range details {
		userId = d.UserId
		r := e.evaluate(eventType, d, player, ctx, guard)
		result.Handled = result.Handled || r.Handled
		result.ConsumeItem = result.ConsumeItem || r.ConsumeItem
	}

	elapsed := time.Since(start).Milliseconds()
	if elapsed > warnMs {
		LogWarn("Notify(%s) for user %d took %dms (threshold %dms), trace: %v",
			eventType, userId, elapsed, warnMs, guard.GetTrace())
	}
	if guard.DepthExceeded() {
		LogError("depth limit %d exceeded for user %d event %s, trace: %v",
			maxDepth, userId, eventType, guard.GetTrace())
	}

	return result
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/questengine/`
Expected: PASS, including every existing test (Notify's behaviour for one detail is unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/engine.go internal/questengine/command_event_test.go
git commit -F - <<'EOF'
feat(questengine): NotifyAll raises several details under one guard (#375)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: The helper, `NotifyCommand`

**Files:**
- Create: `internal/questengine/command_event.go`
- Test: `internal/questengine/command_event_test.go`

- [ ] **Step 1: Write the failing tests** (append)

```go
func TestCommandDetailsCarryTheTarget(t *testing.T) {
	d := CommandDetails(1, 5305, "cast", NoTarget().WithNoun("heal"))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Command: "cast", Noun: "heal", TargetKind: quests.TargetNone}, d)

	d = CommandDetails(1, 5305, "throw", Target{})
	assert.Equal(t, quests.TargetNone, d.TargetKind, "the zero Target reads as none")

	d = CommandDetails(1, 5305, "sell", TargetItem(10009).WithMob(&mobs.Mob{MobId: 9116}))
	assert.Equal(t, quests.TargetItem, d.TargetKind)
	assert.Equal(t, 10009, d.ItemId)
	assert.Equal(t, 9116, d.MobId)

	d = CommandDetails(1, 5305, "plant", TargetContainer("crate").WithItem(4))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Command: "plant", Noun: "crate", ItemId: 4, TargetKind: quests.TargetContainer}, d)
}

func TestTargetConstructors(t *testing.T) {
	assert.Equal(t, quests.TargetMob, TargetMob(&mobs.Mob{MobId: 9112}).Kind)
	assert.Equal(t, 9112, TargetMob(&mobs.Mob{MobId: 9112}).MobId)
	assert.Equal(t, quests.TargetNone, TargetMob(nil).Kind, "a vanished mob is no target")
	assert.Equal(t, quests.TargetNone, TargetItem(0).Kind)
	assert.Equal(t, quests.TargetSelf, TargetActor(1, 1, 0).Kind)
	assert.Equal(t, quests.TargetPlayer, TargetActor(1, 2, 0).Kind)
	assert.Equal(t, quests.TargetNone, TargetActor(1, 0, 0).Kind)
	// A mob instance id that resolves to nothing (no world loaded here) is no target.
	assert.Equal(t, quests.TargetNone, TargetActor(1, 0, 999999).Kind)
}
```

Add `"github.com/GoMudEngine/GoMud/internal/mobs"` to the test imports. Check the type of
`mobs.Mob.MobId` (`internal/mobs/mobs.go`); if it is a named type, write `MobId: 9116` as a typed
constant the compiler accepts (an untyped constant works for a named int type).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/questengine/ -run 'TestCommandDetails|TestTargetConstructors'`
Expected: FAIL to compile.

- [ ] **Step 3: Implement** `internal/questengine/command_event.go`

```go
package questengine

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Target is what a command was aimed at, for the command event's target
// filters (#375 phase 3b slice 2). Build it with the constructors below; the
// zero value reads as NoTarget.
type Target struct {
	Kind   string // a quests.Target* kind; "" reads as quests.TargetNone
	MobId  int    // mob template id: the mob aimed at, or the counterparty (sell's shopkeeper)
	ItemId int    // item template id: the item used, aimed at or traded
	Noun   string // the command's own noun: cast's canonical spell id, a container's name
}

// NoTarget is a command aimed at nothing (help, sleep, a grenade that hit no one).
func NoTarget() Target { return Target{Kind: quests.TargetNone} }

// TargetSelf is a command aimed at its own actor (a self-cast).
func TargetSelf() Target { return Target{Kind: quests.TargetSelf} }

// TargetPlayer is a command aimed at another player.
func TargetPlayer() Target { return Target{Kind: quests.TargetPlayer} }

// TargetMob is a command aimed at a mob, by its template id. A nil mob (it
// died or left before the event) is no target.
func TargetMob(m *mobs.Mob) Target {
	if m == nil {
		return NoTarget()
	}
	return Target{Kind: quests.TargetMob, MobId: int(m.MobId)}
}

// TargetItem is a command aimed at, using or trading an item, by its
// template id. Zero (a condition sale, no item) is no target.
func TargetItem(itemId int) Target {
	if itemId <= 0 {
		return NoTarget()
	}
	return Target{Kind: quests.TargetItem, ItemId: itemId}
}

// TargetContainer is a command aimed at a room container, by its name.
func TargetContainer(name string) Target {
	return Target{Kind: quests.TargetContainer, Noun: name}
}

// TargetActor resolves the {userId, mobInstanceId} pair a combat or move
// result carries: the actor's own id is self, another user is a player, a
// live mob instance is that mob's template, and nothing is no target.
func TargetActor(actorUserId, userId, mobInstanceId int) Target {
	switch {
	case userId > 0 && userId == actorUserId:
		return TargetSelf()
	case userId > 0:
		return TargetPlayer()
	case mobInstanceId > 0:
		return TargetMob(mobs.GetInstance(mobInstanceId))
	}
	return NoTarget()
}

// WithMob adds a counterparty mob (sell's shopkeeper) to a target.
func (t Target) WithMob(m *mobs.Mob) Target {
	if m != nil {
		t.MobId = int(m.MobId)
	}
	return t
}

// WithItem adds the item a command used on its target (the item planted).
func (t Target) WithItem(itemId int) Target {
	if itemId > 0 {
		t.ItemId = itemId
	}
	return t
}

// WithNoun sets the command's own noun (cast's canonical spell id).
func (t Target) WithNoun(noun string) Target {
	t.Noun = noun
	return t
}

// CommandDetails builds the command event for one target. RoomId is the
// template room (rooms.OriginalRoomId), as command_issued and flee already
// match, so room-scoped triggers work in ephemeral instances.
func CommandDetails(userId, roomId int, command string, t Target) EventDetails {
	kind := t.Kind
	if kind == "" {
		kind = quests.TargetNone
	}
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	return EventDetails{
		UserId:     userId,
		RoomId:     matchRoom,
		Command:    command,
		MobId:      t.MobId,
		ItemId:     t.ItemId,
		Noun:       t.Noun,
		TargetKind: kind,
	}
}

// NotifyCommand is THE command event: every production `command` site calls
// it (internal/guards/command_event_guard_test.go holds them to it). It
// raises one event per target under one guard (NotifyAll), so a trigger
// fires at most once per command; no targets means one event aimed at
// nothing. roomId is the room the command happened in; the bridge keeps the
// real id for mob and item resolution.
func NotifyCommand(user *users.UserRecord, roomId int, command string, targets ...Target) NotifyResult {
	if user == nil {
		return NotifyResult{}
	}
	if len(targets) == 0 {
		targets = []Target{NoTarget()}
	}
	details := make([]EventDetails, 0, len(targets))
	for _, t := range targets {
		details = append(details, CommandDetails(user.UserId, roomId, command, t))
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("command", details, bridge, bridge)
}
```

- [ ] **Step 4: Run the tests**

Run: `go build ./... && go test ./internal/questengine/`
Expected: PASS. (If the build reports an import cycle, stop and report it: `questengine` already
imports `mobs`, `rooms`, `users`, so none is expected.)

- [ ] **Step 5: Commit**

```bash
git add internal/questengine/command_event.go internal/questengine/command_event_test.go
git commit -F - <<'EOF'
feat(questengine): NotifyCommand, the one command event helper (#375)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: The guard (written now, red until Task 9)

**Files:**
- Create: `internal/guards/command_event_guard_test.go`

- [ ] **Step 1: Write the guard**

```go
package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// commandEventFile is the one file allowed to raise the quest "command" event
// by hand: questengine.NotifyCommand builds every command event's target
// fields (#375 phase 3b slice 2). A hand-built Notify("command", ...) skips
// them, and a quest step that names a target would never fire there.
const commandEventFile = "internal/questengine/command_event.go"

// isCommandNotify reports whether call is X.Notify("command", ...) or
// X.NotifyAll("command", ...) with the literal event name.
func isCommandNotify(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Notify" && sel.Sel.Name != "NotifyAll") || len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `"command"`
}

func TestCommandEventsOnlyThroughNotifyCommand(t *testing.T) {
	calls, _ := walkGoCalls(t, map[string]bool{"Notify": true, "NotifyAll": true})
	seenHelper := false
	var violations []string
	for _, c := range calls {
		if !isCommandNotify(c.call) {
			continue
		}
		if c.file == commandEventFile {
			seenHelper = true
			continue
		}
		violations = append(violations, c.file+":"+strconv.Itoa(c.line))
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("quest command event raised by hand (use questengine.NotifyCommand with the command's target):\n  %s",
			strings.Join(violations, "\n  "))
	}
	if !seenHelper {
		t.Fatalf("the scan never saw %s raise the command event: the guard cannot find calls", commandEventFile)
	}
}

// TestCommandEventGuardCanFail proves the matcher flags a hand-built call and
// ignores other events.
func TestCommandEventGuardCanFail(t *testing.T) {
	for src, want := range map[string]bool{
		`questengine.GetEngine().Notify("command", d, b, b)`:        true,
		`e.NotifyAll("command", ds, b, b)`:                          true,
		`questengine.GetEngine().Notify("command_issued", d, b, b)`: false,
		`questengine.GetEngine().Notify("room_enter", d, b, b)`:     false,
	} {
		expr, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		if got := isCommandNotify(expr.(*ast.CallExpr)); got != want {
			t.Errorf("isCommandNotify(%s) = %v, want %v", src, got, want)
		}
	}
}
```

`NotifyCommand` passes `"command"` as a literal to `NotifyAll`, which is how the guard sees the
helper file. Keep it a literal.

- [ ] **Step 2: Run it**

Run: `go test ./internal/guards/ -run 'TestCommandEvent'`
Expected: `TestCommandEventGuardCanFail` PASS; `TestCommandEventsOnlyThroughNotifyCommand` FAIL
listing the 32 sites. Save that list to the session scratchpad; Tasks 6 to 9 must empty it.

- [ ] **Step 3: Commit** (the red guard lands with the conversions; commit the file now so each
conversion task can re-run it)

```bash
git add internal/guards/command_event_guard_test.go
git commit -F - <<'EOF'
test(guards): command events only through NotifyCommand (#375)

Red until every site moves; the site tasks follow.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Site conversion pattern (Tasks 6 to 9)

Every site has the shape

```go
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "kick",
	}, bridge, bridge)
```

and becomes one line:

```go
	questengine.NotifyCommand(user, room.RoomId, "kick", <target>)
```

Remove the `bridge` variable only when nothing else in the block uses it. Keep each site's
existing comment and its surrounding guards (`if actor.IsPlayer()`, `users.GetByUserId`) exactly;
only the event construction changes. In the `actions` package the user is the local `u`; use
`room.RoomId` from whichever `room` the old code used. Leave a `NoTarget` site's argument list
empty (`NotifyCommand(user, room.RoomId, "help")`): the helper raises one event aimed at nothing.

After each task: `go build ./... && go test ./internal/questengine/ ./internal/guards/ -run
TestCommandEvent` and confirm the guard's list shrank by exactly that task's sites.

### Task 6: Sites with no target, plus track (11 sites)

**Files:** `internal/usercommands/help.go:105`, `list.go:129`, `say.go:52`, `set.go:566`,
`sleep.go:27`, `skill.skullduggery.sneak.go:126`, `skill.forage.go:29`, `skill.search.go:53`,
`warcry.go:127`, `skill.track.go:36`; `internal/hooks/NewRound_DoCombat_helpers.go:1071`.

- [ ] **Step 1: Convert** each to `questengine.NotifyCommand(user, <roomId>, "<command>")` with
the room id the old code passed: `room.RoomId`, except `set.go` (`user.Character.RoomId`, command
`"combatverbosity"`) and flee:

```go
func notifyFleeSucceeded(user *users.UserRecord, arrivedRoomId int) {
	questengine.NotifyCommand(user, arrivedRoomId, "flee")
}
```

Keep `notifyFleeSucceeded`'s doc comment; replace its sentence about `rooms.OriginalRoomId` with
"NotifyCommand matches on the template id, as command_issued does, so room-scoped triggers work in
ephemeral instances; the bridge keeps the real id." Remove the now-unused `rooms` import there
only if the file no longer uses `rooms` (it does elsewhere; check with the build).

Track is the exception that has a target: `actions.Track` returns `result.ActiveTargetUserId` and
`result.ActiveTargetMobInstId` in active-track mode. Use
`questengine.TargetActor(user.UserId, result.ActiveTargetUserId, result.ActiveTargetMobInstId)`
(it reads as no target in trail-scan mode, where both are 0). Eleven sites in all: ten with no
target, plus track.

- [ ] **Step 2: Build, test, guard count** as in the pattern. Then
`go test ./internal/usercommands/ ./internal/hooks/` Expected: PASS.

- [ ] **Step 3: Commit** the named files with message `refactor: no-target command events through NotifyCommand (#375)` and the Co-Authored-By line, via `git commit -F - <<'EOF'`.

### Task 7: Creature targets (10 sites plus 2 new)

- [ ] **Step 1: Convert**

| Site | Target |
|---|---|
| `usercommands/kick.go:150` | `questengine.TargetActor(user.UserId, target.UserId, target.MobInstanceId)` (`target := res.Target`, line 52) |
| `usercommands/trip.go:65` (target already on the floor) | `questengine.TargetActor(user.UserId, res.Target.UserId, res.Target.MobInstanceId)` |
| `usercommands/trip.go:167` | same as above (confirm the local is `res` or `target` at that line) |
| `usercommands/taunt.go:140` | `questengine.TargetActor(user.UserId, result.Target.UserId, result.Target.MobInstanceId)` |
| `usercommands/consider.go:61` | `questengine.TargetActor(user.UserId, target.GetUserId(), target.GetMobInstanceId())` |
| `usercommands/shoot.go:253` | `questengine.TargetActor(user.UserId, result.TargetUserId, result.TargetMobInstanceId)` (open `actions/combat_fire.go` and confirm `TargetUserId` is set for a player target; if it is only set when `!IsTargetMob`, the call is still right) |
| `actions/shadow.go:124` (mob) | `questengine.TargetMob(m)` |
| `actions/shadow.go:165` (player) | `questengine.TargetPlayer()` |
| `actions/steal.go:271` (`stealFromMob`) | `questengine.TargetMob(m)` |
| `actions/plant.go:134` (`plantOnMob`) | `questengine.TargetMob(m).WithItem(plantItem.ItemId)` |

- [ ] **Step 2: Add the two missing siblings.** `stealFromPlayer` (`actions/steal.go:423`) and
`plantOnPlayer` (`actions/plant.go:261`) raise no event. Add, at the same point in the flow as the
mob sibling (after the target is found and the rank check passes, before the contest):

```go
	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			questengine.NotifyCommand(u, actor.GetRoom().RoomId, "steal", questengine.TargetPlayer())
		}
	}
```

and for plant the same block with `"plant"` and `questengine.TargetPlayer().WithItem(plantItem.ItemId)`.
Read each function first and place the block after the same refusals the mob sibling's block
follows; if the player function's order differs, mirror the mob sibling's position relative to the
contest, and say so in the commit message.

- [ ] **Step 3: Tests for the new siblings.** Find the existing steal and plant tests
(`grep -rln "stealFromPlayer\|plantOnPlayer\|func TestSteal\|func TestPlant" internal/actions`). If a
test already drives a player-target steal with a quest engine installed, extend it to assert the
event; otherwise add `internal/actions/steal_quest_event_test.go` that registers a one-trigger quest
(`Event: "command", Command: "steal", Target: quests.TargetPlayer`) on `questengine.GetEngine()`
(save and restore the global the way existing `actions` quest tests do: find one with
`grep -rln "questengine.GetEngine().RegisterQuest" internal/actions`) and asserts it fires on a
player-target steal and not on a mob-target one. Do the same for plant. If no fixture exists to
drive a steal against a player in a unit test, stop and report rather than inventing one.

- [ ] **Step 4: Build, test, guard count.** `go test ./internal/actions/ ./internal/usercommands/`
Expected: PASS.

- [ ] **Step 5: Commit** with message `feat: creature targets on command events; steal and plant on a player raise them (#375)`.

### Task 8: Item and container targets (9 sites)

- [ ] **Step 1: Convert**

| Site | Target |
|---|---|
| `usercommands/equip.go:213` | `questengine.TargetItem(result.Item.ItemId)` |
| `actions/sell.go:441` | `questengine.TargetItem(item.ItemId).WithMob(mob)` (remove the `details` variable) |
| `actions/drink.go:318` | `questengine.TargetItem(matchItem.ItemId)` (user is `ua.User`) |
| `actions/steal.go:546` (`stealFromContainer`) | `questengine.TargetContainer(containerName)` |
| `actions/steal.go:857` (`stealHouseholdBauble`) | `questengine.TargetItem(itm.ItemId)` |
| `actions/plant.go:350` (`plantInContainer`) | `questengine.TargetContainer(containerName).WithItem(plantItem.ItemId)` |
| `usercommands/craft.go:435` `notifyCraftQuest` | add a parameter: `func notifyCraftQuest(user *users.UserRecord, room *rooms.Room, itemId int)`, body `questengine.NotifyCommand(user, room.RoomId, "craft", questengine.TargetItem(itemId))`; callers `:195` pass `result.Recipe.Output.ItemId`, `:771` pass `recipe.Output.ItemId` |
| `hooks/NewRound_UserRoundTick.go:648` | declare `craftedItemId := 0` before the enchant/output `if`; in the enchant branch set `craftedItemId = targetItem.ItemId` (inside `if eDef != nil`), in the output branch `craftedItemId = recipe.Output.ItemId`; notify with `questengine.TargetItem(craftedItemId)` |
| `actions/buy.go:861` | see Step 2 |

- [ ] **Step 2: Buy carries its item.** Add `ItemId int // template id of the item bought; 0 for a condition sale` to `BuyResult` (`actions/buy.go:44`). In `tryPurchaseFromInventory`, `tryPurchaseLegacy` and the player-shop path that reaches `postSuccessBookkeeping` at `:414`, set `ItemId` on the success result to the bought item's template id (the `newItm`/`bought`/`matched.entry.ItemId` the function already builds; read each function and use the id of the item that went to the buyer). Change `postSuccessBookkeeping(buyer, shopMob, shopUser, awardProgression)` to take `itemId int` after `shopUser`, pass `r.ItemId` from the three callers, and notify with:

```go
			questengine.NotifyCommand(u, buyer.GetRoom().RoomId, "buy", questengine.TargetItem(itemId).WithMob(shopMob))
```

Add a unit test in the existing buy test file (`grep -ln "func TestBuy\|tryPurchaseFromInventory" internal/actions/*_test.go`) asserting `BuyResult.ItemId` on a successful inventory purchase.

- [ ] **Step 3: Equivalence check for shipped triggers.** Quests 31, 32, 50 (equip, `item:`) and 35
(sell, `mob: 9116`, `item: 10009`) must still fire. Run
`go test ./internal/questengine/ -run 'Walkthrough|FirstHeat|Newbie'` and the full
`go test ./internal/questengine/`. Expected: PASS.

- [ ] **Step 4: Build, test, guard count.** `go test ./internal/actions/ ./internal/usercommands/ ./internal/hooks/` Expected: PASS.

- [ ] **Step 5: Commit** with message `feat: item and container targets on command events (#375)`.

### Task 9: Multi-target commands, cast and throw (2 sites)

- [ ] **Step 1: Cast.** In `usercommands/skill.cast.go:383-392`, replace the notify with:

```go
	questengine.NotifyCommand(user, room.RoomId, "cast", castTargets(user.UserId, result, spellInfo.SpellId)...)
```

and add below the function:

```go
// castTargets is one command-event target per creature the cast was aimed
// at, each carrying the canonical spell id, so `noun: <spell>` and a target
// filter both work; a cast at no creature (an area with nobody in it, a
// summon) is one target of none.
func castTargets(casterUserId int, result actions.CastResult, spellId string) []questengine.Target {
	var out []questengine.Target
	for _, uid := range result.TargetUserIds {
		out = append(out, questengine.TargetActor(casterUserId, uid, 0).WithNoun(spellId))
	}
	for _, mid := range result.TargetMobInstanceIds {
		out = append(out, questengine.TargetActor(casterUserId, 0, mid).WithNoun(spellId))
	}
	if len(out) == 0 {
		out = append(out, questengine.NoTarget().WithNoun(spellId))
	}
	return out
}
```

Confirm the result type's name (`grep -n "type .*Result struct" internal/actions/cast.go`; the
fields at `cast.go:59-61` belong to it) and the local's name at the notify site; adjust both.

Test (`internal/usercommands/cast_targets_test.go`): self-cast ids `[caster]` gives one `self`
target with the noun; `TargetUserIds: [2]` gives `player`; empty gives one `none` with the noun.

- [ ] **Step 2: Throw.** In `usercommands/throw.go`, delete the notify at `:296-302` (keep its
comment for the new site). Before the loop at `:355`, add `var thrownAt []questengine.Target`. Inside
the loop, directly after `if mob == nil { continue }` and before `combat.ResolveChannelAttack`, add
`thrownAt = append(thrownAt, questengine.TargetMob(mob))`. After the loop's closing brace (`:493`),
add:

```go
	// Quest engine: command notification, one target per mob the blast
	// reached (hit or not); a throw that reached no one is aimed at nothing.
	// It goes out after the blast so a step that names a mob sees the throw
	// that reached it (#375 phase 3b slice 2).
	questengine.NotifyCommand(user, room.RoomId, "throw", thrownAt...)
```

The spoiled-item return (`:290-294`) still returns before any event, as today.

- [ ] **Step 3: Throw test.** Find the existing throw test that drives a grenade into a room of mobs
(`grep -ln "func TestThrow" internal/usercommands/*_test.go`). Add a case that registers a quest with
`Event: "command", Command: "throw", Target: quests.TargetMob, Mob: <a mob template in that fixture>`
and asserts it fires, plus a case with no mob in the room asserting a `Target: none`-agnostic
trigger (`Command: "throw"` only) still fires once. If no such fixture exists, stop and report.

- [ ] **Step 4: Guard green.** `go test ./internal/guards/ -run TestCommandEvent` Expected: PASS
(the list is empty). Then `go build ./... && go test ./internal/...` Expected: PASS.

- [ ] **Step 5: Commit** with message `feat: cast and throw raise one command event per target (#375)`.

---

### Task 10: Content proof, quest 39's throw

The one shipped throw step (quest 39, `wade_the_marsh`, trigger `command: throw`, token `39-throw`)
keeps working unchanged: it has no target filter. Do not change its YAML in this slice (content
ports are phase 7). Run `go test ./internal/questengine/ -run Walkthrough` and confirm quest 39 walks.
Expected: PASS. No commit.

### Task 11: Docs

**Files:** `internal/questengine/context.md`, `internal/quests/context.md`,
`.claude/skills/dogmud-authoring-quests/SKILL.md`, `modules/gmcp/gmcp.Quest.go:157`,
`docs/superpowers/specs/2026-10-27-quest-arc-phase3b-trigger-kinds-design.md`, `docs/README.md`.

- [ ] **Step 1: `internal/questengine/context.md`**: add `command_event.go` to the file list
(`Target`, `NoTarget`, `TargetSelf`, `TargetPlayer`, `TargetMob`, `TargetItem`, `TargetContainer`,
`TargetActor`, `WithMob`, `WithItem`, `WithNoun`, `CommandDetails`, `NotifyCommand`), `NotifyAll`
beside `Notify`, `EventDetails.TargetKind`, and the rule "every command event goes through
NotifyCommand (guard: `internal/guards/command_event_guard_test.go`)". Verify each named symbol
exists with `grep -n "^func \|^type " internal/questengine/command_event.go` before writing it.

- [ ] **Step 2: `internal/quests/context.md`**: the `target` trigger field, its values, and the loader rule.

- [ ] **Step 3: Authoring skill**: in `.claude/skills/dogmud-authoring-quests/SKILL.md`, under the
trigger filters, document `target:` and that `mob:`/`item:` now work on `command` triggers, with the
example:

```yaml
  - event: command
    command: throw
    target: mob
    mob: 9112
    conditions:
      has: ["39-leech"]
      missing: ["39-boar"]
    actions:
      - grant: "39-boar"
```

and the at-most-once-per-command rule.

- [ ] **Step 4: GMCP trigger table** (`modules/gmcp/gmcp.Quest.go:157` area): update the `command`
row's filter list to include `target, mob, item`.

- [ ] **Step 5: Spec amendments**: in the spec's section 2, apply the six amendments listed at the
top of this plan (helper name and signature, `container` kind, once per command, throw per contested
mob, no mob sites, steal and plant on a player).

- [ ] **Step 6: README**: this plan's row is already in `docs/README.md`; update its wording if a task changed scope.

- [ ] **Step 7: Dash check**: `grep -n "—\|–"` over every file this task touched. Expected: no output.

- [ ] **Step 8: Commit** with message `docs: command event targets (#375)`.

### Task 12: Gate, PR

- [ ] **Step 1:** Load the `dogmud-shipping` skill and run its pre-push gate in its order (build,
vet, the full test suite with `-race` where it says, the detached-worktree boot check).
Expected: all green; paste the summary lines into the PR body.

- [ ] **Step 2:** Push and open the PR **with `--repo pruuk/DOGMud`**:

```bash
git push -u origin feat/375-3b-targets
gh pr create --repo pruuk/DOGMud --base master --head feat/375-3b-targets \
  --title "#375 phase 3b slice 2: targets on command events" --body-file <scratchpad>/pr-body.md
```

The body says "Part of #375" (never "closes #375"), lists the spec and plan, the six spec
amendments, the guard, the two new sibling events, and ends with the Claude Code attribution line.

### Task 13: Playtest

- [ ] **Step 1:** Load `dogmud-playtesting`. Goals file (ephemeral, `--checkout C:/tmp/dogmud-375-3b2`):
a seat that (a) throws a grenade in Pothole Coulee's marsh (quest 39 room 5270/5273) with and without a
mob present and confirms quest 39's throw step still advances once; (b) casts Conviction Spike at
the practice mark for quest 44 (room 5305) and confirms the cast step advances; (c) equips the sling
for quest 50 and confirms the step; (d) sells the quest 35 item to mob 9116 and confirms the step;
(e) steals from and plants on another seat's character with a debug log line confirming one command
event with kind `player` each. Each seat checks that no step advances twice.

- [ ] **Step 2:** File every finding as a GitHub issue on `pruuk/DOGMud` (search first). Record
harness lessons in memory per the playtesting skill.

- [ ] **Step 3:** Report to the controller: the PR link, gate summary, playtest verdict per goal,
issues filed.
