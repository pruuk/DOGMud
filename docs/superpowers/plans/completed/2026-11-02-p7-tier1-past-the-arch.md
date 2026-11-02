# #375 phase 7 tier 1, past the arch: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clear all 39 backlog findings on quests 4 to 15, ship the three general hooks for an item a
player already holds, make the two ledgers and the purse come from somewhere sensible, give quest 14
a real strongbox, and add Warden Esk's two lines.

**Architecture:** Engine hooks first (Tasks 1 to 4), each tested alone, then content (Tasks 5 to 9).
Hook 1 mirrors `exploreAtStepStart`. Content moves through the existing ratchet: fixing a finding
makes its `loop-backlog.txt` line fail CI until the line is deleted.

**Tech stack:** Go, YAML world data, testify. Spec:
`docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier1-design.md`.

**Worktree:** `C:/tmp/dogmud-375-p7-tier1`, branch `feat/375-p7-tier1`, cut from
`docs/375-p7-tier1-spec`. All paths below are relative to it. Remove it when the PR merges.

**Brief for every subagent:**
- Edit files with the Edit and Write tools only. Never rewrite a repo file from the shell.
- `git add` named paths only. Never `--no-verify`.
- No em or en dashes in prose, including YAML comments. NPC `text` is first person. Quest hints are
  at most 200 characters.
- Test logs go in the session scratchpad, never `C:/tmp` or `$TMP`.
- Report back any fact below that does not match the source. Do not work around it.

## Facts verified against source (`45ebaf305` plus spec commits, 2026-11-02)

| Fact | Source |
|---|---|
| `Engine` has `exploreByArea`, `exploreByQuest`; `RegisterQuest` indexes triggers | `internal/questengine/engine.go:32-72` |
| `evaluate` calls `exploreAtStepStart` on `quest_granted` | `internal/questengine/engine.go:177-179` |
| `fire(eventType, t, details, player, ctx, guard, onFire)` is the one firing path | `internal/questengine/engine.go:189` |
| `ctx.Narrate(narration.Variants{Actor: textutil.Pool(line)})` narrates to the actor | `internal/questengine/explore.go:112` |
| Test helpers: `newFullMockPlayer(roomId)` with `.items`, `.quests`; `newFullMockActionContext(1, p)` with `.sentTexts` | `internal/questengine/engine_test.go:11-36`; `explore_test.go:126-143` |
| `TriggerDef.ExploredLine` is validated in `validateAreaCount` | `internal/quests/triggers.go:36`; `internal/quests/quests.go:146-148` |
| Builder lists: `EVENT_FILTERS.item_gain`, `FILTER_KINDS`, `FIELD_LABELS`; vocab row `item_gain`; kept in step by `TestQuestBuilderFiltersMatchTheVocab` | `_datafiles/html/public/static/js/quests.js:518-549`; `modules/gmcp/gmcp.Quest.go:163` |
| `Topic` struct; offer and info cases in the validator | `internal/dialogue/topic.go:64-80`; `internal/dialogue/world.go:138-165` |
| `cardBody(q, names, lastRun)`; `offerCard` builds the card and its callback calls `accept` then `openMenu` | `internal/conversation/card.go:34-56`; `internal/conversation/talk.go:333-349` |
| `GiveQuest` queues `events.Quest` at priority 0 | `internal/conversation/playerstate.go:60-65` |
| `TalkNextOffer`, priority 45, listener registered at `internal/hooks/hooks.go:109`; a ShowLast menu is priority 50 | `internal/events/eventtypes.go:105-118`; `internal/conversation/nextoffer_test.go` |
| `unlock.go` container path 39-42 (not locked), 53-60 (key ring), 61-84 (backpack key); exit path 95-98, 109-116, 117-140 | `internal/usercommands/unlock.go` |
| `notifyPicklockQuest` body | `internal/usercommands/picklock.go:405-416` |
| Lock quest test fixture to copy | `internal/usercommands/picklock_quest_test.go:31-74` |
| `HasKey` reads `key-<lockId>` from the key ring | `internal/characters/inventory.go:129-135` |
| Plain `item_gain` triggers: quests 4, 5, 7, 9, 14 (two), 15, 31, 35 (two), 38, 42, 79 to 87. None is on a repeatable quest | parse of `quests/*.yaml` |
| Backlog guard to copy | `internal/guards/coulee_backlog_guard_test.go` |
| Topic keys today: `83.help_quest`, `84.help_quest`, `86.help_quest`, `89.help_quest`, `250.quest_start`, `95.tithe_problem`, `99.tithe_ledger` | `dialogue/topic-keys.txt` |
| Quest 4: `investigate` granted by `behaviors/rooms/dustwalk_road/407.yaml` and `room_enter 408`; `4-investigate` also in Tessara's opener and `step-ids.txt:156` | as named |
| `internal/behaviortree/topics_test.go:100,119` use `4-investigate` as an inline fixture, not shipped content: leave them | as named |
| Room 408 already has a `goods` noun; a noun and a container may share a name, as on room 3040 | `rooms/dustwalk_road/408.yaml`; `rooms/ironwind_steppe/3040.yaml` |
| `step-ids.txt` has `# retired` and `# renamed` sections at the end, `old -> new` | `quests/step-ids.txt:385-392` |

Data paths below are under `_datafiles/world/dogmud/`.

---

### Task 1: `unlock` raises a quest event

**Files:**
- Modify: `internal/usercommands/unlock.go`, `internal/usercommands/picklock.go:405-416`
- Create: `internal/usercommands/unlock_quest_test.go`

- [ ] **Step 1: Write the failing test.** Create `internal/usercommands/unlock_quest_test.go`:

```go
package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 tier 1: unlocking is a quest step (quest 14's strongbox), so
// unlock raises a `command` event aimed at the container, as picklock does.
const (
	unlockProbeQuest = 99704
	unlockTestKey    = 999963
	unlockProbeToken = "99704-start"
)

func unlockQuestFixture(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	t.Cleanup(seedAllRegistries())

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	before := room.Containers
	room.Containers = map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 3}}}
	t.Cleanup(func() { room.Containers = before })

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		unlockTestKey: {ItemId: unlockTestKey, Name: "test key", Type: items.Key,
			KeyLockId: fmt.Sprintf("%d-lockbox", room.RoomId), Weight: 0.1},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.Stats.Strength.ValueAdj = 100

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: unlockProbeQuest,
		Name:    "Unlock Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "unlock",
			Target:  quests.TargetContainer,
			Noun:    "lockbox",
			Actions: []quests.ActionDef{{Grant: unlockProbeToken}},
		}},
	})
	return user, room
}

func TestUnlock_BackpackKeyRaisesTheQuestEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	require.True(t, acquire.AcquireItem(user, items.New(unlockTestKey), acquire.AcquireGive))

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.False(t, room.Containers["lockbox"].Lock.IsLocked(), "the key opened it")
	require.True(t, user.Character.HasQuest(unlockProbeToken))
}

func TestUnlock_KeyRingRaisesTheQuestEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	user.Character.SetKey(fmt.Sprintf("key-%d-lockbox", room.RoomId), fmt.Sprintf("%d", unlockTestKey))

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(unlockProbeToken))
}

func TestUnlock_NoKeyRaisesNothing(t *testing.T) {
	user, room := unlockQuestFixture(t)

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, room.Containers["lockbox"].Lock.IsLocked())
	require.False(t, user.Character.HasQuest(unlockProbeToken), "a refused unlock must raise nothing")
}

// The player typed the step's command and the box stands open, perhaps because
// another player opened it. Without this the step waits for the relock.
func TestUnlock_AlreadyOpenContainerRaisesTheEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	box := room.Containers["lockbox"]
	box.Lock.SetUnlocked()
	room.Containers["lockbox"] = box

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(unlockProbeToken))
}
```

- [ ] **Step 2: Run it and see it fail.**
`go test ./internal/usercommands/ -run 'TestUnlock_' -count=1`. Expected: the three raising tests
FAIL on `HasQuest`, `TestUnlock_NoKeyRaisesNothing` passes.

- [ ] **Step 3: Generalize the notify helper.** In `picklock.go`, replace the helper and its comment:

```go
// notifyLockQuest raises the `command` quest event for a lock just opened by
// command (picklock, unlock): a container lock is aimed at the container by
// name (`target: container`, `noun:` on a trigger); an exit lock is aimed at
// nothing, with the exit's name as its noun (#375 phase 7, quests 99 and 14).
func notifyLockQuest(user *users.UserRecord, room *rooms.Room, command string, containerName string, exitName string) {
	target := questengine.TargetContainer(containerName)
	if containerName == `` {
		target = questengine.NoTarget().WithNoun(exitName)
	}
	questengine.NotifyCommand(user, room.RoomId, command, target)
}
```

Update its two callers in `picklock.go` to `notifyLockQuest(user, room, "picklock", containerName, exitName)`,
keeping the variables they pass today. Run `grep -n notifyPicklockQuest internal -r` and confirm none remains.

- [ ] **Step 4: Raise it from `unlock.go`.** In the container path, change the not-locked branch:

```go
		if !container.Lock.IsLocked() {
			user.SendText(messaging.CategoryRefusal, "That's not locked.")
			notifyLockQuest(user, room, "unlock", containerName, ``)
			return true, nil
		}
```

Add `notifyLockQuest(user, room, "unlock", containerName, ``)` as the last line of the `if hasKey {` and
`else if hasBackpackKey {` bodies of the container path. Add `notifyLockQuest(user, room, "unlock", ``, exitName)`
as the last line of the same two bodies of the exit path. The exit's not-locked branch and both "You do
not have the key" branches stay silent.

- [ ] **Step 5: Run the tests.** `go test ./internal/usercommands/ -count=1`. Expected: PASS, including
`TestPicklock_*`.

- [ ] **Step 6: Commit.**

```bash
git add internal/usercommands/unlock.go internal/usercommands/picklock.go internal/usercommands/unlock_quest_test.go
git commit -m "feat(quests): unlock raises a command quest event"
```

### Task 2: Hook 1, a held item counts at step start

**Files:**
- Modify: `internal/quests/triggers.go`, `internal/quests/quests.go:146-148`,
  `internal/questengine/engine.go`, `_datafiles/html/public/static/js/quests.js:524-549`,
  `modules/gmcp/gmcp.Quest.go:163`
- Create: `internal/questengine/held.go`, `internal/questengine/held_test.go`

- [ ] **Step 1: Write the failing engine tests.** Create `internal/questengine/held_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// heldQuest: "begin" moves 341 to fetch; gaining the item ends it. Any filter
// the case sets goes on the item_gain trigger.
func heldQuest(mutate func(t *TriggerDef)) *QuestDef {
	gain := TriggerDef{
		Event: "item_gain", Item: 77,
		Conditions: Conditions{Has: []string{"341-fetch"}, Missing: []string{"341-end"}},
		Actions:    []ActionDef{{Grant: "341-end"}},
	}
	if mutate != nil {
		mutate(&gain)
	}
	return &QuestDef{
		QuestId: 341, Name: "Fetch",
		Steps: []QuestStep{{Id: "start"}, {Id: "fetch"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{Event: "command", Command: "begin",
				Conditions: Conditions{Has: []string{"341-start"}, Missing: []string{"341-fetch"}},
				Actions:    []ActionDef{{Grant: "341-fetch"}}},
			gain,
		},
	}
}

func runHeld(q *QuestDef, holds bool) (*fullMockPlayer, *fullMockActionContext) {
	e := NewEngine()
	e.RegisterQuest(q)
	p := newFullMockPlayer(100)
	p.quests["341-start"] = true
	if holds {
		p.items[77] = true
	}
	ctx := newFullMockActionContext(1, p)
	e.Notify("command", EventDetails{UserId: 1, Command: "begin", TargetKind: "none"}, p, ctx)
	return p, ctx
}

func TestHeld_AHeldItemFiresAtStepStart(t *testing.T) {
	p, ctx := runHeld(heldQuest(nil), true)
	assert.True(t, p.HasQuest("341-end"), "the held item satisfies the step as it starts")
	assert.Equal(t, []string{"You already have that."}, ctx.sentTexts,
		"with no item spec loaded the default line names no item")
}

func TestHeld_HeldLineOverridesTheDefault(t *testing.T) {
	p, ctx := runHeld(heldQuest(func(t *TriggerDef) { t.HeldLine = "You kept it safe." }), true)
	assert.True(t, p.HasQuest("341-end"))
	assert.Equal(t, []string{"You kept it safe."}, ctx.sentTexts)
}

func TestHeld_NotHeldDoesNotFire(t *testing.T) {
	p, ctx := runHeld(heldQuest(nil), false)
	assert.True(t, p.HasQuest("341-fetch"))
	assert.False(t, p.HasQuest("341-end"))
	assert.Empty(t, ctx.sentTexts)
}

// A filter asks how or where the item arrived, which a held item cannot answer.
func TestHeld_AFilteredTriggerDoesNotFire(t *testing.T) {
	for name, mutate := range map[string]func(t *TriggerDef){
		"reason": func(t *TriggerDef) { t.Reason = "steal" },
		"room":   func(t *TriggerDef) { t.Room = 5218 },
		"area":   func(t *TriggerDef) { t.Area = "grove" },
		"zone":   func(t *TriggerDef) { t.Zone = "Somewhere" },
		"count":  func(t *TriggerDef) { t.Count = 2 },
	} {
		p, _ := runHeld(heldQuest(mutate), true)
		assert.False(t, p.HasQuest("341-end"), name)
	}
}

// The trigger's own conditions still gate it: a held item with the step not
// reached fires nothing.
func TestHeld_ConditionsGate(t *testing.T) {
	q := heldQuest(func(t *TriggerDef) { t.Conditions.Has = []string{"341-other"} })
	p, _ := runHeld(q, true)
	assert.False(t, p.HasQuest("341-end"))
}
```

The bare type names are aliases from `internal/questengine/types.go`, as `explore_test.go:40-52` uses them.

- [ ] **Step 2: Add the failing validation test.** Create `internal/quests/held_line_test.go`,
`package quests`, importing `strings` and `testing`:

```go
func TestValidate_HeldLineNeedsAPlainItemGain(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    TriggerDef
		want string
	}{
		{"wrong event", TriggerDef{Event: "room_enter", HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"reason", TriggerDef{Event: "item_gain", Reason: "steal", HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"room", TriggerDef{Event: "item_gain", Room: 5, HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"count", TriggerDef{Event: "item_gain", Count: 2, HeldLine: "x"}, "held_line needs a plain item_gain"},
	} {
		err := validateAreaCount(tc.t)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if err := validateAreaCount(TriggerDef{Event: "item_gain", Item: 5, HeldLine: "x"}); err != nil {
		t.Errorf("a plain item_gain may carry held_line: %v", err)
	}
}
```

- [ ] **Step 3: Run them and see them fail** on the missing `HeldLine` field:
`go test ./internal/quests/ ./internal/questengine/ -run 'Held' -count=1`.

- [ ] **Step 4: Add the field and its rule.** In `internal/quests/triggers.go`, below `ExploredLine`:

```go
	HeldLine     string      `yaml:"held_line,omitempty" json:"held_line,omitempty"`         // plain item_gain: said when the step starts with the item already held
```

Add to `internal/quests/triggers.go`:

```go
// PlainItemGain reports whether t is an item_gain trigger that asks only which
// item arrived. A held item can stand in for one of these at step start; a
// reason, room, area, zone or count asks how or where it arrived, or how many.
func (t TriggerDef) PlainItemGain() bool {
	return t.Event == "item_gain" && t.Reason == "" && t.Room == 0 &&
		t.Area == "" && t.Zone == "" && t.Count == 0
}
```

In `validateAreaCount`, after the `ExploredLine` check:

```go
	if t.HeldLine != "" && !t.PlainItemGain() {
		return fmt.Errorf("held_line needs a plain item_gain (it is said when the step starts with the item already held; a reason, room, area, zone or count cannot be answered by a held item)")
	}
```

- [ ] **Step 5: Write `internal/questengine/held.go`:**

```go
package questengine

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// heldAtStepStart checks the granted quest's plain item_gain triggers as a step
// starts: one whose conditions pass and whose item the player already holds
// fires at once, telling the player why (its held_line, else the default line
// naming the item). An item picked up before the step began raises no item_gain
// of its own, so without this the step could not finish (#375 phase 7 tier 1,
// owner ruling: a held item is handled, never dropped and picked up again). It
// runs inside evaluate beside exploreAtStepStart, so its chain is bounded by the
// guard like any other.
func (e *Engine) heldAtStepStart(d EventDetails, player PlayerState, ctx ActionContext, guard *EvalGuard) NotifyResult {
	questId, _ := quests.TokenToParts(d.QuestToken)
	result := NotifyResult{}
	for _, t := range e.heldByQuest[questId] {
		if !player.HasItem(t.def.Item) || !EvalConditions(t.def.Conditions, player) {
			continue
		}
		line := heldLine(t.def)
		result.merge(e.fire("held", t, d, player, ctx, guard, func() {
			ctx.Narrate(narration.Variants{Actor: textutil.Pool(line)})
		}))
	}
	return result
}

func heldLine(t *TriggerDef) string {
	if t.HeldLine != "" {
		return t.HeldLine
	}
	if spec := items.GetItemSpec(t.Item); spec != nil && spec.Name != "" {
		return fmt.Sprintf("You already have the %s.", spec.Name)
	}
	return "You already have that."
}
```

The default line uses the item's name as it stands: "You already have the Bandit's Purse." If that package
cannot import `items` without a cycle, report it; `bridge.go` is the place that already reads items.

- [ ] **Step 6: Index and call it.** In `internal/questengine/engine.go`:

```go
	exploreByArea  map[string][]*indexedTrigger
	exploreByQuest map[int][]*indexedTrigger
	// Plain item_gain triggers by quest id, read as a step starts
	// (heldAtStepStart). They are in triggerIndex too, where a real gain
	// still matches them.
	heldByQuest map[int][]*indexedTrigger
```

Initialize `heldByQuest: make(map[int][]*indexedTrigger),` in `NewEngine`. In `RegisterQuest`, after the
explore block:

```go
		if t.PlainItemGain() {
			e.heldByQuest[q.QuestId] = append(e.heldByQuest[q.QuestId], it)
		}
```

In `evaluate`:

```go
	if eventType == "quest_granted" && details.QuestToken != "" {
		result.merge(e.exploreAtStepStart(details, player, ctx, guard))
		result.merge(e.heldAtStepStart(details, player, ctx, guard))
	}
```

Check `QuestDef.Triggers` element type: `RegisterQuest` takes `t := &q.Triggers[i]`, and `t.def` is
`*quests.TriggerDef`, so a value-receiver `PlainItemGain` works on it.

- [ ] **Step 7: Builder lists.** In `quests.js`: add `"held_line"` to `EVENT_FILTERS.item_gain`, add
`held_line: "text"` to `FILTER_KINDS`, add `held_line: "Held line (said when the step starts with the item already held)"`
to `FIELD_LABELS`. In `gmcp.Quest.go:163`, end the `item_gain` text with `; count, label; held_line said when the item is already held)`.

- [ ] **Step 8: Run tests.** `go test ./internal/quests/ ./internal/questengine/ ./modules/gmcp/ -count=1`.
Expected: PASS, including `TestQuestBuilderFiltersMatchTheVocab` (it may need `held_line` in a list it
holds; read its failure if so).

- [ ] **Step 9: Run the whole suite** because this changes behaviour for 21 shipped triggers:
`go test ./... -count=1`. A walkthrough or Coulee test that changes is a real finding: report it, do
not edit the test to pass.

- [ ] **Step 10: Commit.**

```bash
git add internal/quests/triggers.go internal/quests/quests.go internal/quests/held_line_test.go internal/questengine/engine.go internal/questengine/held.go internal/questengine/held_test.go _datafiles/html/public/static/js/quests.js modules/gmcp/gmcp.Quest.go
git commit -m "feat(quests): a held item counts when its step starts"
```

### Task 3: Hook 2, an offer can state its own objective

**Files:**
- Modify: `internal/dialogue/topic.go:64-80`, `internal/dialogue/world.go:138-165`,
  `internal/conversation/card.go`, `internal/conversation/talk.go:333-338`
- Test: `internal/conversation/card_test.go`, `internal/dialogue/world_test.go`

- [ ] **Step 1: Write the failing card test** in `internal/conversation/card_test.go`:

```go
// An offer shown to a player already holding the item says what to do with it:
// its objective replaces the first step's description on the card.
func TestOfferCardBody_ObjectiveReplacesTheFirstLine(t *testing.T) {
	q := cullQuest()
	body := offerCardBody(&dialogue.Topic{Objective: "Give it back."}, q, quests.RewardNames{}, false)
	require.Equal(t, "Give it back.", body[0])
	require.Equal(t, cardBody(q, quests.RewardNames{}, false)[1:], body[1:], "the rest of the card is unchanged")
}

func TestOfferCardBody_NoObjectiveKeepsTheCard(t *testing.T) {
	q := cullQuest()
	require.Equal(t, cardBody(q, quests.RewardNames{}, false),
		offerCardBody(&dialogue.Topic{}, q, quests.RewardNames{}, false))
}

func TestOfferCardBody_ObjectiveOnAQuestWithNoDescription(t *testing.T) {
	q := &quests.Quest{QuestId: 1, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}}
	require.Equal(t, []string{"Give it back."},
		offerCardBody(&dialogue.Topic{Objective: "Give it back."}, q, quests.RewardNames{}, false))
}
```

Add the `dialogue` import to `card_test.go`.

- [ ] **Step 2: Write the failing validator case** in `internal/dialogue/world_test.go`, following the
`breakFixture` table at line 104 (read the fixture to pick an info topic's anchor line):
`{"objective on info", breakFixture("<an info topic's unique line>", "<same line>\n  objective: x"), "objective belongs on offer topics only"}`
and the same for a hand-in topic.

- [ ] **Step 3: See them fail.** `go test ./internal/conversation/ ./internal/dialogue/ -run 'OfferCardBody|objective' -count=1`.

- [ ] **Step 4: Add the field.** In `Topic`, below `Breadcrumb`:

```go
	// Objective replaces the first line of the offer card (the quest's first
	// step description) when that line would be wrong for this offer, such as
	// one shown to a player already holding the item. Offers only.
	Objective string `yaml:"objective,omitempty" json:"objective,omitempty"`
```

- [ ] **Step 5: Validate it.** In `validateTopicFile`, beside each `breadcrumb belongs on offer topics only`
check (the `KindInfo` case, and the `KindHandIn` line in the offer and hand-in case):

```go
			if t.Objective != "" {
				add(where, "objective belongs on offer topics only")
			}
```

In the hand-in case guard it with `t.Kind == KindHandIn &&`.

- [ ] **Step 6: Add `offerCardBody`** to `internal/conversation/card.go`:

```go
// offerCardBody is cardBody for one offer: the offer's objective, when it has
// one, replaces the first step's description.
func offerCardBody(t *dialogue.Topic, q *quests.Quest, names quests.RewardNames, lastRun bool) []string {
	body := cardBody(q, names, lastRun)
	if t.Objective == "" {
		return body
	}
	if q != nil && len(q.Steps) > 0 && q.Steps[0].Description != "" && len(body) > 0 {
		body[0] = t.Objective
		return body
	}
	return append([]string{t.Objective}, body...)
}
```

Add the `dialogue` import. In `offerCard`, change the body line to:

```go
		card.Body = offerCardBody(t, q, cardNames(user.Character), user.Character.LastRepeat(q))
```

- [ ] **Step 7: Run.** `go test ./internal/conversation/ ./internal/dialogue/ -count=1`. Expected: PASS.

- [ ] **Step 8: Commit.**

```bash
git add internal/dialogue/topic.go internal/dialogue/world.go internal/dialogue/world_test.go internal/conversation/card.go internal/conversation/card_test.go internal/conversation/talk.go
git commit -m "feat(dialogue): an offer can state its own objective"
```

### Task 4: Hook 3, the menu comes back with the hand-in on it

**Files:**
- Modify: `internal/events/eventtypes.go`, `internal/conversation/nextoffer.go`,
  `internal/conversation/talk.go:333-349`, `internal/hooks/hooks.go:109`
- Test: `internal/conversation/nextoffer_test.go`

- [ ] **Step 1: Write the failing test** in `nextoffer_test.go`:

```go
// After accepting an offer for an item already held, the menu must wait for the
// quest's chain: the grant is a priority 0 Quest event, and the hand-in is not
// eligible until it and the step start have run.
func TestTalkMenuAgain_DrainsAfterTheQuestGrant(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.AddToQueue(events.TalkMenuAgain{UserId: 1, MobInstanceId: 2}, events.TalkNextOfferPriority)
	events.AddToQueue(events.Quest{UserId: 1, QuestToken: "9-start"})
	require.Equal(t, []string{"Quest", "TalkMenuAgain"}, events.QueuedTypesInOrderForTest())
}
```

- [ ] **Step 2: See it fail** on the undefined type. `go test ./internal/conversation/ -run TalkMenuAgain -count=1`.

- [ ] **Step 3: Add the event** to `internal/events/eventtypes.go`, after `TalkNextOfferPriority`:

```go
// TalkMenuAgain re-shows an NPC's talk menu after an offer for an item the
// player already holds is accepted (#375 phase 7 tier 1). It is queued at
// TalkNextOfferPriority so the grant and the step start have run first, and the
// hand-in they opened is on the menu.
type TalkMenuAgain struct {
	UserId        int
	MobInstanceId int
}

func (t TalkMenuAgain) Type() string { return `TalkMenuAgain` }
```

- [ ] **Step 4: Add the handler** to `internal/conversation/nextoffer.go`:

```go
// HandleTalkMenuAgain re-shows the talk menu queued by an accepted offer for a
// held item. While a menu gate holds or a prompt is open nothing opens, and the
// hand-in waits in the talk menu with its [?].
func HandleTalkMenuAgain(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.TalkMenuAgain)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	if reason, promptOpen := user.MenuBlockedReason(); reason != menu.CloseNone || promptOpen {
		return events.Continue
	}
	openMenu(user, evt.MobInstanceId, "")
	return events.Continue
}
```

Register it in `internal/hooks/hooks.go` beside line 109:
`events.RegisterListener(events.TalkMenuAgain{}, conversation.HandleTalkMenuAgain)`.

- [ ] **Step 5: Queue it from the card.** In `offerCard`'s callback:

```go
		granted, closes := "", false
		if e.Id == "yes" {
			granted, closes = accept(user, inst, key, prefer)
		}
		if closes {
			return
		}
		if granted != "" && t.CarriesItem > 0 {
			events.AddToQueue(events.TalkMenuAgain{UserId: user.UserId, MobInstanceId: inst}, events.TalkNextOfferPriority)
			return
		}
		openMenu(user, inst, granted)
```

Add the `events` import to `talk.go`.

- [ ] **Step 6: Run.** `go test ./internal/conversation/ ./internal/hooks/ -count=1`. Expected: PASS.
The live order (grant, step start, then menu) is confirmed by the playtest's Carrying seat. If the Quest
handler defers its chain past priority 45, the seat shows no `[?]`: stop and report it.

- [ ] **Step 7: Commit.**

```bash
git add internal/events/eventtypes.go internal/conversation/nextoffer.go internal/conversation/nextoffer_test.go internal/conversation/talk.go internal/hooks/hooks.go
git commit -m "feat(talk): the menu returns with the hand-in after a held item's offer"
```

### Task 5: Where the items come from (quests 4, 5, 9)

**Files:**
- Modify: `rooms/dustwalk_road/408.yaml`, `rooms/watchers_crossing/421.yaml`, `rooms/thornwall_city/476.yaml`,
  `behaviors/rooms/dustwalk_road/407.yaml`, `quests/4-the_wardens_report.yaml`,
  `quests/5-the_innkeepers_complaint.yaml`, `quests/9-the_temples_tithe_audit.yaml`, `quests/step-ids.txt`,
  `dialogue/dustwalk_road/83.yaml`, `dialogue/watchers_crossing/84.yaml`, `dialogue/watchers_crossing/86.yaml`,
  `dialogue/thornwall_city/95.yaml`, `dialogue/thornwall_city/99.yaml`, `dialogue/topic-keys.txt`,
  `quests/loop-backlog.txt`
- Create: `internal/dialogue/shipped_item_sources_test.go`

- [ ] **Step 1: Write the failing shipped-content test.**

```go
package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// Spec ruling 5: a quest item with a keeper beside it comes from the keeper.
func roomSpawns(t *testing.T, path string) []struct {
	ItemId    int    `yaml:"itemid"`
	Container string `yaml:"container"`
} {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/rooms/" + path)
	require.NoError(t, err)
	var room struct {
		SpawnInfo []struct {
			ItemId    int    `yaml:"itemid"`
			Container string `yaml:"container"`
		} `yaml:"spawninfo"`
	}
	require.NoError(t, yaml.Unmarshal(data, &room))
	return room.SpawnInfo
}

func TestTheLedgersComeFromTheirKeepers(t *testing.T) {
	for _, s := range roomSpawns(t, "watchers_crossing/421.yaml") {
		require.NotEqual(t, 21, s.ItemId, "the crossing ledger no longer lies in the Tollhouse")
	}
	for _, s := range roomSpawns(t, "thornwall_city/476.yaml") {
		require.NotEqual(t, 29, s.ItemId, "the tithe ledger no longer lies in the Records Office")
	}

	harn := loadShippedDialogue(t, "watchers_crossing/86.yaml")
	lend := harn.FindTopic("lend_ledger")
	require.NotNil(t, lend)
	require.Equal(t, 21, lend.GivesItem)
	require.Contains(t, menuKeys(harn, heldState("5-start")), "lend_ledger")
	require.NotContains(t, menuKeys(harn, heldState()), "lend_ledger", "only during quest 5")
	require.NotContains(t, menuKeys(harn, heldState("5-start", "5-end")), "lend_ledger")

	pell := loadShippedDialogue(t, "thornwall_city/99.yaml")
	copyTopic := pell.FindTopic("tithe_ledger")
	require.NotNil(t, copyTopic)
	require.Equal(t, 29, copyTopic.GivesItem)
	require.False(t, strings.Contains(copyTopic.Hints, "get ledger"))
}

func TestThePurseLiesInAContainer(t *testing.T) {
	found := false
	for _, s := range roomSpawns(t, "dustwalk_road/408.yaml") {
		if s.ItemId == 16 {
			found = true
			require.Equal(t, "goods", s.Container)
		}
	}
	require.True(t, found, "the Bandit Hollow still holds the purse")
}
```

Run `go test ./internal/dialogue/ -run 'TheLedgers|ThePurse' -count=1` and see it fail.

- [ ] **Step 2: Rooms.**
  - `rooms/watchers_crossing/421.yaml`: delete the `- itemid: 21` entry and its `respawnrate` line.
  - `rooms/thornwall_city/476.yaml`: delete the `- itemid: 29` entry and its `respawnrate` line.
  - `rooms/dustwalk_road/408.yaml`: change the purse spawn to the block below (the `message` goes), add
    the `containers` block before `nouns:`, and in `description` change "Stolen goods are stacked against
    the back wall." to `Stolen <ansi fg="itemname">goods</ansi> are stacked against the back wall.`

```yaml
- itemid: 16
  container: goods
  respawnrate: 10 real minutes
containers:
  goods: {}
```

- [ ] **Step 3: Quest 4 loses `investigate`.** In `quests/4-the_wardens_report.yaml`:
  - Replace the `start` step's `description` and `hint`, and the comment beside its `destination`:

```yaml
  - id: start
    destination:
      room: 408  # the Bandit Hollow, where the purse is
    description: "Road Warden Tessara at the Warden's Post on Dustwalk Road has asked\
      \ you to investigate a bandit camp in the gullies to the south. Search the\
      \ Bandit Hollow for evidence of their operations."
    hint: "From the Warden's Post, go south three times to the Bandit Hollow. Look\
      \ among the stolen goods. Type get purse from goods."
```

  - Delete the whole `investigate` step and the `room_enter 408` trigger.
  - Change the `item_gain 16` trigger's `has` to `["4-start"]`.
  - In `quests/step-ids.txt`, delete line `4-investigate` and add `4-investigate -> 4-start` in `# renamed`.

- [ ] **Step 4: Room 407's tree.** Replace `behaviors/rooms/dustwalk_road/407.yaml`'s children so it speaks and grants nothing:

```yaml
tree:
  type: sequence
  event: room_enter
  children:
    - type: condition
      check: player_has_quest
      quest: "4-start"
    - type: condition
      check: player_missing_quest
      quest: "4-evidence"
    - type: action
      do: send_user_text
      text: "The scattered debris and fresh boot prints confirm it: this is\
        \ where the bandits have been operating. There may be more evidence\
        \ deeper in the gully."
```

Read the file first: if it has more children than the five shown in the facts, keep them and report it.

- [ ] **Step 5: Tessara (`dialogue/dustwalk_road/83.yaml`).** Delete the opener variant gated on
`4-investigate`. In the next variant (`4-start`), change `questExcluded` from `4-investigate` to
`4-evidence` and its hint's "Type" line to name `get purse from goods` if it names a command. Update
the `help_quest` hint if it names `get purse`.

- [ ] **Step 6: Harn's `lend_ledger`.** In `dialogue/watchers_crossing/86.yaml`, after `show_ledger`, and add
`86.lend_ledger` to `dialogue/topic-keys.txt` (sorted):

```yaml
- key: lend_ledger
  kind: info
  label: May I see your ledger?
  text: "Tolva sent you, I expect. Take it. Read every page, and show it to whoever
    you like. Every coin is in there."
  hints: "Take the crossing ledger back to Innkeeper Tolva at the Crossing Inn."
  marked: true
  questRequired:
  - 5-start
  questExcluded:
  - 5-end
  itemExcluded: 21
  givesItem: 21
```

- [ ] **Step 7: Quest 5 and Tolva.** In `quests/5-the_innkeepers_complaint.yaml`, the `start` step:

```yaml
    description: "Innkeeper Tolva at the Crossing Inn has asked you to investigate the bridge toll. She believes it is unauthorized and wants proof. Ask Harn at the tollhouse for his ledger."
    hint: "From the Crossing Inn, go south to the West Approach, then east into the Tollhouse. Type talk harn and ask to see his ledger."
```

  Keep its `destination` (room 421) but make it `destination: {mob: 86, room: 421}`, since Harn moves by
  schedule. In `dialogue/watchers_crossing/84.yaml`, the `5-start` opener hint becomes "From the Crossing
  Inn, go south to the West Approach, then east into the Tollhouse. Type talk harn." In its
  `carriesItem: 21` variant the hint becomes "Choose a line below to hand Tolva the crossing ledger."

- [ ] **Step 8: Pell and Olen.** In `dialogue/thornwall_city/99.yaml`, replace the comment above `tithe_ledger`
and the topic's `text` and `hints`, keeping its gates and adding `givesItem: 29`:

```yaml
# Pell keeps the temple's copies and hands one over (spec ruling 5). Shown to
# anyone on quest 9 who is not carrying the ledger, so a lost one can be replaced.
- key: tithe_ledger
  kind: info
  label: I'm looking for the temple's tithe ledger.
  text: The tithe ledger? That is a temple document, not a municipal one. But yes,
    I have a copy. The temple files duplicates here as a matter of public record.
    Here. Whatever you find in there, remember that I only file the documents. I do
    not judge the contents.
  hints: Take the tithe ledger back to Temple Priest Olen and type talk olen.
```

In `dialogue/thornwall_city/95.yaml`: at lines 31, 41 and 58, change "Records Office. Type get ledger."
to "Records Office. Type talk pell." Reword the comment at lines 13-16 to say the hand-in gates on
`9-deliver`, and that `have_it` (Task 6) covers a save holding the ledger on `9-start`. The first
variant's hint becomes "Choose a line below to hand over the tithe ledger." In
`quests/9-the_temples_tithe_audit.yaml`, the `start` step:

```yaml
    description: "Temple Priest Olen has asked you to investigate discrepancies in
      the tithe records. He suspects merchants in the market are under-reporting
      their income to avoid full tithes. Ask Clerk Pell in the Records Office for
      the temple's copy of the tithe ledger."
    hint: "From the Temple Interior, go south twice, then east, then north
      into the Records Office. Type talk pell."
```

  Its `destination` becomes `destination: {mob: 99, room: 476}`.

- [ ] **Step 9: Run.** `go test ./internal/dialogue/ ./internal/questengine/ ./internal/behaviortree/ ./internal/guards/ ./internal/quests/ -count=1`
and `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedDialoguePassesTheValidators|TestSmoke' -count=1`.
Fix any backlog line that names a finding now gone by deleting that line from `quests/loop-backlog.txt`.
Never add a line.

- [ ] **Step 10: Commit** with named paths, message `fix(quests): the ledgers come from their keepers, the purse from a container`.

### Task 6: Hand-ins follow the last step, offers notice a held item (4, 5, 7, 9, 15)

**Files:**
- Modify: `dialogue/dustwalk_road/83.yaml`, `dialogue/watchers_crossing/84.yaml`, `dialogue/thornwall_outskirts/89.yaml`,
  `dialogue/thornwall_city/95.yaml`, `dialogue/marches_spur_road/250.yaml`, `dialogue/topic-keys.txt`,
  `quests/4-*.yaml`, `quests/5-*.yaml`, `quests/7-*.yaml`, `quests/9-*.yaml`, `quests/15-*.yaml`, `quests/loop-backlog.txt`
- Test: `internal/dialogue/shipped_quest_bugs_test.go`

- [ ] **Step 1: Write the failing test** in `shipped_quest_bugs_test.go`:

```go
// A held item on the first step: new players go through the carrying offer and
// Hook 1, and a save from before this shipped gets have_it (spec, "Old saves").
func TestGiversHandleAHeldItem(t *testing.T) {
	for _, c := range []struct {
		file, offer, carrying, handin string
		item                          int
		start, deliver, end           string
	}{
		{"dustwalk_road/83.yaml", "help_quest", "help_quest_carrying", "evidence_handoff", 16, "4-start", "4-evidence", "4-end"},
		{"thornwall_outskirts/89.yaml", "help_quest", "help_quest_carrying", "dornhandoff", 24, "7-start", "7-deliver", "7-end"},
		{"marches_spur_road/250.yaml", "quest_start", "quest_start_carrying", "quest_complete_dialogue", 40039, "15-start", "15-return", "15-end"},
	} {
		df := loadShippedDialogue(t, c.file)
		holding := heldState()
		holding.CarriesItem = func(id int) bool { return id == c.item }
		holding.HasItem = func(id int) bool { return id == c.item }

		keys := menuKeys(df, holding)
		require.Contains(t, keys, c.carrying, c.file)
		require.NotContains(t, keys, c.offer, c.file)
		keys = menuKeys(df, heldState())
		require.Contains(t, keys, c.offer, c.file)
		require.NotContains(t, keys, c.carrying, c.file)
		require.NotEmpty(t, df.FindTopic(c.carrying).Objective, c.file)

		// have_it shows only on the start step with the item.
		onStart := heldState(c.start)
		onStart.CarriesItem = holding.CarriesItem
		onStart.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, "the hand-in follows the last step: "+c.file)
		require.NotContains(t, menuKeys(df, heldState(c.start)), "have_it", c.file)

		delivered := heldState(c.start, c.deliver)
		delivered.CarriesItem = holding.CarriesItem
		delivered.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, delivered), c.handin, c.file)
		require.NotContains(t, menuKeys(df, delivered), "have_it", c.file)
	}

	// The ledgers cannot be held before their quests, so they have no second
	// offer, only have_it for an old save.
	for _, c := range []struct {
		file, handin        string
		item                int
		start, deliver      string
	}{
		{"watchers_crossing/84.yaml", "tolvahandoff", 21, "5-start", "5-deliver"},
		{"thornwall_city/95.yaml", "olenhandoff", 29, "9-start", "9-deliver"},
	} {
		df := loadShippedDialogue(t, c.file)
		onStart := heldState(c.start)
		onStart.CarriesItem = func(id int) bool { return id == c.item }
		onStart.HasItem = onStart.CarriesItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, c.file)
	}
}
```

Run `go test ./internal/dialogue/ -run TestGiversHandleAHeldItem -count=1` and see it fail.

- [ ] **Step 2: Hand-ins.** Change `questRequired` from the start token to the last step on these topics:
`evidence_handoff` (`4-evidence`), `tolvahandoff` (`5-deliver`), `dornhandoff` (`7-deliver`), `olenhandoff`
(`9-deliver`), `quest_complete_dialogue` (`15-return`). Reword any comment above them that says the
hand-in gates on the start token.

- [ ] **Step 3: Remove the hidden shortcuts.** Delete the `quest_granted` trigger with `has_item` from
`quests/7-*.yaml`, `quests/9-*.yaml` and `quests/15-*.yaml`.

- [ ] **Step 4: Add `have_it`** to each of the five files. Add `<mob>.have_it` to `topic-keys.txt`. The
template, with Tessara's values:

```yaml
# For a save already on the first step with the item in hand: nothing starts a
# step for it, so nothing counts the item (spec, "Old saves"). A new player
# never sees this; the step start moves them past it.
- key: have_it
  kind: info
  label: I have it here.
  text: Then let me see it.
  hints: Choose The Warden's Report below to hand it over.
  marked: true
  questRequired:
  - 4-start
  questExcluded:
  - 4-evidence
  - 4-end
  carriesItem: 16
```

Values for the rest: Tolva `5-start`, `5-deliver`, `5-end`, item 21, hint "Choose The Innkeeper's Complaint
below to hand it over."; Dorn `7-start`, `7-deliver`, `7-end`, item 24, "Choose The Fallow Field below to
hand it over."; Olen `9-start`, `9-deliver`, `9-end`, item 29, "Choose The Temple's Tithe Audit below to
hand it over."; Malk `15-start`, `15-return`, `15-end`, item 40039, "Choose The Peddler's Overdue Freight
below to hand it over."

Add a matching trigger to each quest file. Tessara's, with the mob and tokens changed for the others
(mobs: Tolva 84, Dorn 89, Olen 95, Malk 250):

```yaml
  # A save on the first step already holding the item (spec, "Old saves").
  - event: topic_heard
    mob: 83
    topic: have_it
    conditions:
      has: ["4-start"]
      missing: ["4-evidence"]
    actions:
      - grant: "4-evidence"
```

- [ ] **Step 5: Add the carrying offers** to Tessara, Dorn and Malk. Add `itemExcluded: <item>` to the existing
offer. Add `<mob>.<key>_carrying` to `topic-keys.txt`. Copy the offer's `quest`, `label`, `questExcluded`,
`grantsQuest` and `moodChange`, and use:

```yaml
- key: help_quest_carrying   # Tessara, dustwalk_road/83.yaml
  kind: offer
  quest: 4
  label: The Warden's Report
  playerLine: I took this purse off some bandits.
  text: "A purse with marks on it. You found their stash already, then. That is the
    very proof I was about to ask someone to go and get. Hand it over and it goes in
    my next report."
  objective: Give the bandit's purse to Road Warden Tessara.
  questExcluded:
  - 4-start
  - 4-end
  carriesItem: 16
  grantsQuest: 4-start
  moodChange: friendly
- key: help_quest_carrying   # Dorn, thornwall_outskirts/89.yaml
  kind: offer
  quest: 7
  label: The Fallow Field
  playerLine: I killed something in your fields and kept a piece.
  text: That is a pest from the south fields, and a big one. I have been meaning to
    ask someone to bring me a sample, so I can see what the fallow field is feeding.
    Let me look at it.
  objective: Give the pest sample to Farmer Dorn.
  questExcluded:
  - 7-start
  - 7-end
  carriesItem: 24
  grantsQuest: 7-start
  moodChange: friendly
- key: quest_start_carrying   # Malk, marches_spur_road/250.yaml
  kind: offer
  quest: 15
  label: The Peddler's Overdue Freight
  playerLine: I found a freight crate in a ruined barn.
  text: That is my freight. I know that forced lock. I have waited on it for days and
    feared the bandits had it for good. Set it down here and let me count what is left.
  objective: Give the freight crate to Peddler Malk.
  questExcluded:
  - 15-start
  - 15-end
  carriesItem: 40039
  grantsQuest: 15-start
  moodChange: friendly
```

Read each existing offer first: if it carries a field not shown (such as `hints`), keep it off the
carrying copy only if it describes finding the item.

- [ ] **Step 6: Openers.** Dorn's first variant (`7-start`, `carriesItem: 24`) and Tessara's and Malk's
matching variants keep their text; their hints become "Choose a line below to hand over the <item>."
Delete the "gates on 7-start plus the sample" comment in `89.yaml` and the "(batch 2 review B1)" lines in
`250.yaml`; replace with one line saying the hand-in follows the last step. Malk's comment at the top of
`250.yaml` (R15) still holds. Malk's first variant hint may stay as written, since it describes the same flow.

- [ ] **Step 7: Backlog and run.** Delete every line in `quests/loop-backlog.txt` for quests 4, 5, 7, 9, 15.
Run `go test ./internal/dialogue/ ./internal/questengine/ ./internal/questloop/ ./internal/guards/ -count=1`
and `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`. Expected: PASS. A new finding is fixed in
content, never listed.

- [ ] **Step 8: Commit** with named paths, message `fix(quests): hand-ins follow the last step, givers notice a held item`.

### Task 7: Quest 14's real strongbox

**Files:**
- Modify: `rooms/thornwall_city/498.yaml`, `items/*/40034-strongbox_key.yaml`, `quests/14-the_undertow.yaml`,
  `quests/step-ids.txt`, `quests/loop-backlog.txt`
- Test: `internal/questengine/` (new file `tier1_undertow_test.go`)

- [ ] **Step 1: Write the failing shipped-content test.** Create `internal/questengine/tier1_undertow_test.go`,
`package questengine`, importing `os`, `path/filepath`, `testing`, `quests`, `require` and `yaml "gopkg.in/yaml.v2"`:

```go
func TestUndertowHasARealStrongbox(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/14-the_undertow.yaml")
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	steps := []string{}
	for _, s := range q.Steps {
		steps = append(steps, s.Id)
	}
	require.Equal(t, []string{"start", "explore", "confront", "loot", "unlock", "opened", "evidence", "end"}, steps)
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
	}
	data, err = os.ReadFile("../../_datafiles/world/dogmud/rooms/thornwall_city/498.yaml")
	require.NoError(t, err)
	require.Contains(t, string(data), "strongbox:\n    lock:")
	require.Contains(t, string(data), "container: strongbox")
	key, err := filepath.Glob("../../_datafiles/world/dogmud/items/*/40034-*.yaml")
	require.NoError(t, err)
	require.Len(t, key, 1)
	data, err = os.ReadFile(key[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "keylockid: 498-strongbox")
}
```

See it fail.

- [ ] **Step 2: Room 498.** Add below `spawninfo` and before `nouns:`, and keep the noun:

```yaml
spawninfo:
  - mobid: 249
  - itemid: 40036
    container: strongbox
    respawnrate: 5 real minutes
containers:
  strongbox:
    lock:
      difficulty: 20
      relockinterval: 5 real minutes
```

- [ ] **Step 3: The key.** Add `keylockid: 498-strongbox` below `namesimple`, and change the description's
"a crude shipping mark -- a circle" to "a crude shipping mark, a circle".

- [ ] **Step 4: Steps.** In `quests/14-the_undertow.yaml`, delete step `report`. Change `loot`'s and
`unlock`'s hints, and add `opened` between `unlock` and `evidence`:

```yaml
  - id: loot
    destination:
      room: 498  # Operations Room, where Torvan falls
    description: "Torvan is down. His strongbox key is on his body."
    hint: "Type loot torvan to take the key."
  - id: unlock
    destination:
      room: 498  # Operations Room, where the strongbox is
    description: "You have the key to Torvan's strongbox."
    hint: "With the key in hand, type unlock strongbox."
  - id: opened
    destination:
      room: 498  # Operations Room, where the strongbox is
    description: "Torvan's strongbox is open. Whatever proof he kept is inside."
    hint: "Type get ledger from strongbox."
```

Update `confront`'s description if it says the box needs anything other than the key. In
`quests/step-ids.txt`, add `14-opened` (sorted) and move `14-report` to `# retired`, with a comment:
`# Quest 14's report step: nothing ever granted it (#375 phase 7 tier 1).`

- [ ] **Step 5: Triggers.** Delete all five `room_interact` triggers on `strongbox` and the second `room_enter 498`
trigger (the one with `has_item: 40034`). Keep `room_gate 486`, `item_gain 40035`, and the first `room_enter 498`.
Replace the rest:

```yaml
  - event: mob_death
    mob: 249
    conditions:
      has: ["14-confront"]
      missing: ["14-loot"]
    actions:
      - grant: "14-loot"
      - actor: "Torvan Cresk falls. The strongbox key is still on his belt."

  - event: item_gain
    item: 40034
    conditions:
      has: ["14-loot"]
      missing: ["14-unlock"]
    actions:
      - grant: "14-unlock"

  - event: command
    command: unlock
    target: container
    noun: strongbox
    room: 498
    conditions:
      has: ["14-unlock"]
      missing: ["14-opened"]
    actions:
      - grant: "14-opened"

  # A skilled hand can pick it instead of using the key.
  - event: command
    command: picklock
    target: container
    noun: strongbox
    room: 498
    conditions:
      has: ["14-unlock"]
      missing: ["14-opened"]
    actions:
      - grant: "14-opened"

  - event: item_gain
    item: 40036
    conditions:
      has: ["14-opened"]
      missing: ["14-evidence"]
    actions:
      - grant: "14-evidence"
      - actor: "A slim leather journal, a bribe ledger, filled with names, dates, and
          amounts paid to corrupt city officials. This is the evidence you need."
```

- [ ] **Step 6: Backlog and run.** Delete the 13 quest 14 lines from `quests/loop-backlog.txt`. Run
`go test ./internal/questengine/ ./internal/quests/ ./internal/guards/ ./internal/questloop/ -count=1` and
`DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`. Expected: PASS. The walkthrough must accept the
locked container as the ledger's source; if it reports `walk-no-source`, report it rather than listing it.

- [ ] **Step 7: Commit** with named paths, message `fix(quests): quest 14's strongbox is a real locked container`.

### Task 8: Quests 6, 11, 12, 13: hints, map targets, recovery

**Files:**
- Modify: `quests/6-*.yaml`, `quests/11-*.yaml`, `quests/12-*.yaml`, `quests/13-*.yaml`,
  `dialogue/ironwind_steppe/241.yaml`, `dialogue/ironwind_steppe/242.yaml`, `dialogue/topic-keys.txt`,
  `quests/loop-backlog.txt`
- Test: `internal/dialogue/shipped_quest_bugs_test.go`

- [ ] **Step 1: Extend the failing test.** Add a case in `TestQuest11CanBeFinishedAfterTalkingToKaelFirst`:

```go
			// A save from before the sided step: the flag is set, 11-sided is not.
			reaffirm := "reaffirm_" + s.value
			flagged := heldState("11-start")
			flagged.GetQuestFlag = sided.GetQuestFlag
			flagged.HasItem = sided.HasItem
			flagged.CarriesItem = sided.CarriesItem
			require.Contains(t, menuKeys(df, flagged), reaffirm)
			require.NotContains(t, menuKeys(df, flagged), s.handin, "the hand-in follows sided")
			require.NotContains(t, menuKeys(df, sided), reaffirm, "a new player never sees it")
			require.NotContains(t, menuKeys(df, unsided), reaffirm)
```

See it fail.

- [ ] **Step 2: Hand-ins.** `totem_return` and `sample_return`: `questRequired` becomes `11-sided`.

- [ ] **Step 3: Recovery topics.** Add to `241.yaml` (and `242.yaml` with `rhett`), with keys in `topic-keys.txt`:

```yaml
# A save that chose a side before the sided step existed holds the flag without
# 11-sided, and may never have heard the terms. A new player never sees this:
# the stand topic grants 11-sided as it sets the flag.
- key: reaffirm_sylara
  kind: info
  label: I stand by what I chose.
  text: Then I will hold you to it. Bring me what I asked for.
  hints: "Sylara wants her carved wolf totem from the Kill Ground, deep in the wolf
    runs to the south."
  marked: true
  questRequired:
  - 11-start
  questExcluded:
  - 11-sided
  - 11-end
  questFlagRequired:
    11-branch: sylara
```

Rhett's: `reaffirm_rhett`, text "Good. Then bring me the samples.", hint "Rhett wants windstone samples from the
veins on the Rocky Shelf, just south of his camp.", flag value `rhett`.

Add to `quests/11-*.yaml`, one per side (mob 241 `reaffirm_sylara`, mob 242 `reaffirm_rhett`):

```yaml
  # A save holding the side flag without this step (spec, quest 11).
  - event: topic_heard
    mob: 241
    topic: reaffirm_sylara
    conditions:
      has: ["11-start"]
      missing: ["11-sided"]
    actions:
      - grant: "11-sided"
```

Reword the comment in the `sided` step that says the hand-ins gate on `11-start`.

- [ ] **Step 4: Hints.** Replace these exactly:

| Step | Hint |
|---|---|
| 6 `start` | Take the report to Clerk Pell in Thornwall. From the Tollhouse, go east ten times to the end of Main Street, then north to the Records Office. Type talk pell. |
| 6 `report` | Return to Toll Collector Harn. From the Records Office, go south, then west ten times to the Tollhouse. In the evening he drinks at the Crossing Inn. Type talk harn. |
| 11 `start` | Rhett camps at the Ridge Approach, seven rooms east of Thornwall's Old Citadel Plaza. Sylara keeps the Stone Circle, south twice from there, then east. Type talk rhett. |
| 11 `sided` | Sylara wants her carved wolf totem from the Kill Ground, deep in the wolf runs to the south. Rhett wants windstone samples from the Rocky Shelf, south of his camp. |
| 12 `start` | Tell Sylara at the Stone Circle you are ready for the ritual. From Thornwall's Old Citadel Plaza, go east seven times, south twice, then east. Type talk sylara. |

- [ ] **Step 5: Map targets.** Add to step 11 `sided`: `destination: {zone: Ironwind Steppe}`. Step 12
`ritual`: `destination: {room: 3115}`. Step 13 `extraction`: `destination: {room: 3030}`.

- [ ] **Step 6: Backlog and run.** Delete every line for quests 6, 11, 12, 13. Run
`go test ./internal/dialogue/ ./internal/questengine/ ./internal/questloop/ ./internal/guards/ -count=1` and
`DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`. Expected: PASS.

- [ ] **Step 7: Add the tier 1 guard.** Create `internal/guards/tier1_backlog_guard_test.go`:

```go
package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
)

// The #375 phase 7 tier 1 pass (spec
// docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier1-design.md) cleared
// every loop finding on quests 4 to 15. A finding on one of them is fixed in
// content, never listed.
func tier1BacklogLines(t *testing.T, data []byte) []string {
	t.Helper()
	b, err := questloop.ParseBacklog(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range b {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if id, err := strconv.Atoi(fields[1]); err == nil && id >= 4 && id <= 15 {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier1QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier1BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 1 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier1QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\none-action 14 step report:none\nhint-length 6 step start\nchain 73 mob 9465 topic q74_grant\n")
	if got := tier1BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 1 lines, want 2: %v", len(got), got)
	}
}
```

Run `go test ./internal/guards/ -run Tier1 -count=1`. Expected: PASS.

- [ ] **Step 8: Commit** with named paths, message `fix(quests): steppe and toll hints, map targets and the side recovery`.

### Task 9: Esk's lines

**Files:**
- Modify: `dialogue/pothole_coulee/9107.yaml`, `dialogue/topic-keys.txt`
- Test: `internal/dialogue/esk_road_test.go`

- [ ] **Step 1: Write the failing test** in `esk_road_test.go`:

```go
func TestEskSpeaksOfWhatWaitsOutThere(t *testing.T) {
	esk := loadCouleeDialogue(t, "9107.yaml")

	require.NotContains(t, menuKeys(esk, heldState()), "out_there", "for the Opened only")

	opened := menuKeys(esk, heldState("30-end"))
	require.Contains(t, opened, "out_there")
	require.NotContains(t, opened, "what_it_does", "it opens once the first is heard")

	heard := heldState("30-end")
	heard.HasHeard = func(key string) bool { return key == HeardKey(9107, "out_there") }
	require.Contains(t, menuKeys(esk, heard), "what_it_does")

	require.True(t, esk.FindTopic("out_there").Marked)
	require.Contains(t, esk.FindTopic("passage").Text, "The rest is out there")
}
```

See it fail. `heardRequired` reads `HeardKey(df.MobId, key)` (`internal/dialogue/topics.go:75`), so
`loadCouleeDialogue` returns Esk with `MobId` 9107 and the key above matches.

- [ ] **Step 2: Add the topics** after `eyes`, and add `9107.out_there` and `9107.what_it_does` to `topic-keys.txt`:

```yaml
# Owner 2026-11-02: past the arch nobody walks the player, and Esk says so.
# These lines stay subtle and generic. No place, directions or NPC names, and
# nothing about the crash, the ships or what the Chrysalis actually is.
- key: out_there
  kind: info
  label: What is waiting out there?
  text: Nobody can tell you that. The Chrysalis has had a long time to work on the
    people out there, and no two of them have come out the same. You will see things
    you have no name for. Ask about them. Most folk will tell you more than they
    meant to.
  hints: Past the arch, nobody sends you to your next quest. Talk to people and look
    closely at what is around you.
  marked: true
  questRequired:
  - 30-end
- key: what_it_does
  kind: info
  label: What will it do to me?
  text: These eyes came the winter I decided to keep this door. I never asked for
    them. I meant it, and I kept on meaning it, and that is as much as I understand.
    Whatever you hold to hardest, the Chrysalis seems to hear. Be careful what you are
    certain of.
  heardRequired:
  - out_there
  questRequired:
  - 30-end
```

- [ ] **Step 3: The crossing line.** `passage`'s `text` becomes:

```yaml
  text: So. The coulee has taught you what it can, for now. The rest is out there,
    for whoever goes looking.
```

- [ ] **Step 4: Run.** `go test ./internal/dialogue/ ./internal/guards/ -count=1` and
`DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`. Expected: PASS. If `TestEskPassage*` or a
Coulee copy test pins the old line, update the pin to the approved line.

- [ ] **Step 5: Commit.**

```bash
git add _datafiles/world/dogmud/dialogue/pothole_coulee/9107.yaml _datafiles/world/dogmud/dialogue/topic-keys.txt internal/dialogue/esk_road_test.go
git commit -m "content(esk): what waits past the arch, in the Chrysalis's terms"
```

### Task 10: Dashes, docs, gates

**Files:**
- Modify: comments in `quests/4-*`, `5-*`, `6-*`, `7-*`, `8-*`, `9-*`, `10-*`, `14-*`, `15-*`;
  `docs/PATCH_NOTES.md`, `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/README.md`,
  `internal/questengine/context.md`, `internal/quests/context.md`, `internal/dialogue/context.md`,
  `internal/conversation/context.md`, `.claude/skills/dogmud-authoring-quests/SKILL.md`

- [ ] **Step 1: Dashes.** `grep -nP '[\x{2014}\x{2013}]' _datafiles/world/dogmud/quests/{4,5,6,7,8,9,10,11,12,13,14,15}-*.yaml`.
Reword each comment with a comma or colon. Run it again and confirm no output (the command exits 1 on none; that is the pass).

- [ ] **Step 2: Context files.** `questengine/context.md`: beside the explore section, describe `heldAtStepStart`:
plain `item_gain` triggers, `held_line`, default line, runs on `quest_granted`; and name `unlock` beside
`picklock` as a command that raises `command` on opening. `quests/context.md`: add `held_line` and
`PlainItemGain` to the trigger fields. `dialogue/context.md`: `objective`, offers only.
`conversation/context.md`: `offerCardBody`, `TalkMenuAgain`.

- [ ] **Step 3: Skill.** In `dogmud-authoring-quests`, add a section "An item the player already holds" under
"Item gains: `reason`": a plain `item_gain` counts a held item when its step starts; give a quest whose
first step is getting a findable item a second offer with `carriesItem`, an `objective`, and `itemExcluded`
on the first, as Tessara's `help_quest_carrying`; a quest item whose keeper stands beside it comes from the
keeper as a `givesItem` topic, as Harn's `lend_ledger`; never ask a player to drop and pick up.

- [ ] **Step 4: Roadmap.** In the phase 7 status: tier 1 done, naming this spec and the PR, with "The other
zones are not started" changed to name tier 2 next. Mark the spec's rulings 2 and 5 under Approach or the
phase 7 section so tier 2 reads them.

- [ ] **Step 5: Patch notes.** Add under today's date in `docs/PATCH_NOTES.md`, no raw numbers:

```markdown
## 2026-11-02: Past the arch

- Warden Esk has something to say about what waits on the far side of the arch.
- If you already have what a quest asks for, the quest counts it, and the person
  asking may well notice.
- Harn now lends you his ledger himself, and Clerk Pell hands over the temple's
  copy of the tithe ledger. Neither is lying around any more.
- The purse in the Bandit Hollow is among the stolen goods.
- Torvan's strongbox in The Undertow is a real lockbox: unlock it, or pick it if
  you have the hands for it, then take what is inside.
- Quests around Thornwall, the steppe and the Crossing now ask for one thing at a
  time, and the map points the way on every step.
```

Add `internal/questengine/held.go` to any file list in `questengine/context.md`, and add the plan and the new
guard to `docs/README.md`.

- [ ] **Step 6: Gates.** Run, in order, and read each result:

```bash
gofmt -l internal/ modules/
go build ./...
go test ./... -count=1
DOGMUD_BOOT_SMOKE=1 go test . -count=1
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `gofmt` prints nothing; tests pass; lint `0 issues`. `DOGMUD_BOOT_SMOKE=1` on `./internal/rooms`
fails locally on Windows for two known zone tests: run it on the root package only.

- [ ] **Step 7: Isolated boot** per the `dogmud-shipping` skill, with the scratch override in the session
scratchpad. Expected: `Server Ready` 1, panic count 0, exit 124.

- [ ] **Step 8: Commit** with named paths, message `docs: tier 1 context, skill, roadmap and patch notes`.

### Task 11: Adversarial playtest (controller)

**Use the `dogmud-playtesting` skill.** One subagent per seat, ports private, a fresh instance, a goals file in
the scratchpad.

| Seat | Must show |
|---|---|
| Quest 14 | Whole line with the key; again by picking the lock; a second seat takes the ledger first and the first seat's step survives it |
| Carrying | Takes Malk's crate first: Malk's line, the card's objective, Accept, "You already have...", `[?]` straight after, hand-in. Then Dorn's sample, drops it with the card open, accepts: the step does not advance |
| Ledgers | Quest 5 from Harn at the Tollhouse and in the evening at the inn; quest 9 from Pell; loses each and asks again; quests 5 and 6 on one character |
| Steppe | Kael first, then a side, then the hand-in; 12 or 13 offered at hand-in. The dim-room lanterns from #758 seen live: `talk kael` at midnight |
| Old save | Quest 11 with the flag set and no `11-sided`; quest 9 on `9-start` holding the ledger; `4-investigate` loads as `4-start` |
| Crossing | A newcomer with no coaching reads Esk's lines and crosses. Reports in their own words what the lines made them want to do |

- [ ] **Step 1:** Run the seats. Check claims against raw events, not only the seat's report.
- [ ] **Step 2:** Fix findings that belong to this pass on the branch, with a test. File the rest on `pruuk/DOGMud`, searching first.
- [ ] **Step 3:** Save harness lessons to memory.
- [ ] **Step 4:** Push, open the PR with `--repo pruuk/DOGMud --base master`, watch CI, merge with `--merge` when green. Then remove the worktree, delete `docs/375-p7-tier1-spec`, and put the main checkout back on master.
