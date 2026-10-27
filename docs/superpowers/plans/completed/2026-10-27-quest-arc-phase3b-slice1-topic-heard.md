# #375 phase 3b slice 1: talk steps (`topic_heard`), implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The quest file owns every mid-quest talk step: a new `topic_heard` trigger (mob + topic
key) advances the step when the player hears that topic, the 32 topics that grant a mid-quest
token today move their grant onto such a trigger, and a topic that is the current step's objective
shows `[*]` even when already heard.

**Architecture:** `conversation.play` raises `topic_heard` after a topic's effects land, through a
helper beside slice 2's `NotifyCommand`. The trigger gains a `topic` filter; the loader and the
reference checks hold it to a real topic on a real NPC. `dialogue.PlayerState` gains an `Objective`
callback, wired to `Engine.WouldFire("topic_heard", ...)`, which `TopicMarker` reads. The
walkthrough hears a `topic_heard` trigger's topic (gates met, effects applied) before firing it. A
new `questloop` rule, `talk-on-quest`, reports a topic granting a token that is neither its quest's
first nor final step; after the migration it reports nothing, so the backlog ratchet keeps it at
zero.

**Tech Stack:** Go (`internal/quests`, `questengine`, `questloop`, `questworld`, `dialogue`,
`conversation`, `guards`), dialogue and quest YAML under `_datafiles/world/dogmud/`.

**Spec:** `docs/superpowers/specs/2026-10-27-quest-arc-phase3b-trigger-kinds-design.md`, section 1.
Slices 2 (PR #693) and 3 (PR #698) are merged: read `internal/questengine/command_event.go`.

**Rules for every implementer (global, enforced by hooks):** edit files only with the Edit and
Write tools, never with shell redirection, `sed -i` or Python file writes. `git add` named paths
only. Never `--no-verify`. No em or en dashes in prose or comments. Kill only processes you
started, by PID.

---

## Facts verified against source (master `e061dd69d`, 2026-10-27)

| Fact | Where |
|---|---|
| Loader `validEvents` include `command`, `spell_landed`; `target` allowed on both; `effect` only on `spell_landed` | `internal/quests/quests.go` (Validate) |
| `TriggerDef` fields: event, room, mob, item, skill, command, quest_token, noun, verb, target, effect, conditions, actions | `internal/quests/triggers.go:11-25` |
| `EventDetails` fields include `TargetKind`, `Effect`; no topic field | `internal/questengine/types.go:39-52` |
| `NotifyCommand`, `NotifySpellLanded` and their `*Details` builders; guard `isGuardedNotify` allows only `command_event.go` to raise `command` and `spell_landed` | `internal/questengine/command_event.go`; `internal/guards/command_event_guard_test.go` |
| `Engine.WouldFire(eventType, d, player)` dry run over `fieldMatches` + `EvalConditions` | `internal/questengine/dryrun.go:14` |
| `conversation.play` runs `applyWithHandover` and, only on success, the NPC line, hints and `after` (markers, mood, behavior topic) | `internal/conversation/talk.go:277-322` |
| `DialogueFile.Apply` returns false when a charge or paid effect refuses; marks an info topic heard | `internal/dialogue/topics.go:172-187` |
| `TopicMarker` returns `[*]` only when `t.Marked && !heard`; `Marker` builds the NPC's roster marker from eligible topics | `internal/dialogue/topics.go:48-80` |
| `dialogue.PlayerState` is a struct of callbacks (HasQuest, HasHeard, MarkHeard, QuestOpen, ...); a nil interrogative callback fails closed | `internal/dialogue/types.go:6-50`; `internal/dialogue/context.md` |
| `conversation.PlayerState(user ...)` builds it (`HasHeard: user.Character.HasHeard`, `QuestOpen: ...`) | `internal/conversation/playerstate.go:57-135` |
| `internal/conversation` already depends on `internal/questengine` | `go list -deps` (slice spec facts) |
| `GameBridge.GrantQuest` grants synchronously (`GiveQuestToken`) | `internal/questengine/bridge.go:89-92` |
| 187 topic `grantsQuest`: 87 `-start`, 68 `-end`, 32 other, all 32 on `kind: info`; the 32 are listed in Task 8 | grep of `dialogue/**` |
| `QuestValidators.MobHasDialogue`; `questworld.mobHasDialogueFile` loads `dialogue.Load(mobId, zone)` | `internal/quests/validate_refs.go:30`; `internal/questworld/questworld.go:88-94` |
| `questloop.Validate` runs ruleStart, ruleEnd, ruleHint, ruleHintLength, ruleOneAction, ruleRepeat, ruleReward, ruleRefs per quest, then ruleChain; findings keyed `rule questId where`; topic ways read `where: "mob <id> topic <key>"` | `internal/questloop/questloop.go:61-80`; `grants.go:60-66` |
| The backlog ratchet: `_datafiles/world/dogmud/quests/loop-backlog.txt`; the shipped smoke fails on new findings and reports stale ones | `internal/questloop/backlog.go`; `quest_loop_smoke_test.go` (repo root, `package main`) |
| Shipped smokes run with `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedQuestsPassTheLoop\|TestSmoke_WalkSourcesIndexShippedWorld'` at the repo root (not `./internal/...`) | slice 3 Task 6 report |
| Walkthrough: topic Ways fire through `fireTopic` (Eligible, then Apply, then a `quest_granted` chain); trigger Ways through `fireTrigger`, which builds the event from the trigger's fields; `env.files` maps mob id to dialogue file; `env.meetTopic` meets a topic's gates | `internal/questengine/walkthrough.go` |

## File structure

| File | Change |
|---|---|
| `internal/quests/triggers.go`, `quests.go`, `topic_heard_test.go` (new) | `Topic` field, `topic_heard` event, loader rules |
| `internal/quests/validate_refs.go` (+ test validators) , `internal/questworld/questworld.go` | `TopicExists` |
| `internal/questengine/types.go`, `engine.go`, `walkthrough.go`, `internal/questloop/grants.go` | `Topic` on events and matching |
| `internal/questengine/command_event.go`, `topic_heard_test.go` (new) | `TopicHeardDetails`, `NotifyTopicHeard` |
| `internal/guards/command_event_guard_test.go` | guard covers `topic_heard` |
| `internal/conversation/talk.go`, test | raise the event |
| `internal/dialogue/types.go`, `topics.go`, test; `internal/conversation/playerstate.go` | `Objective` marker |
| `internal/questengine/walkthrough.go`, `walkthrough_topic_test.go` (new) | walk hears `topic_heard` topics; `fireTopic` raises it |
| `internal/questloop/rule_talk.go` (new), `questloop.go`, test | `talk-on-quest` |
| 27 dialogue files, the quest files they serve, `quests/loop-backlog.txt` | the migration |
| context.md files, authoring skill, GMCP vocab, builder JS, spec, README | docs |

---

### Task 0: Worktree

The worktree exists: `C:/tmp/dogmud-375-3b1`, branch `feat/375-3b-topic-heard`, from master
`e061dd69d`. Baseline: `go build ./... && go test ./internal/quests/ ./internal/questengine/ ./internal/questloop/ ./internal/questworld/ ./internal/dialogue/ ./internal/conversation/ ./internal/guards/`. Expected PASS; stop and report otherwise.

---

### Task 1: `topic_heard` and the `topic` field

**Files:** `internal/quests/triggers.go`, `internal/quests/quests.go`; Create `internal/quests/topic_heard_test.go`.

- [ ] **Step 1: Failing test**

```go
package quests

import (
	"strings"
	"testing"
)

func TestValidateTopicHeardTriggers(t *testing.T) {
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Mob, q.Triggers[0].Topic = "topic_heard", 9112, "directions"
	if err := q.Validate(); err != nil {
		t.Fatalf("topic_heard with mob and topic refused: %v", err)
	}
	for _, c := range []struct {
		mob   int
		topic string
		want  string
	}{
		{0, "directions", "topic_heard needs a mob"},
		{9112, "", "topic_heard needs a topic"},
	} {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].Mob, q.Triggers[0].Topic = "topic_heard", c.mob, c.topic
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("mob %d topic %q: want %q, got %v", c.mob, c.topic, c.want, err)
		}
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Topic = "directions" // a command trigger
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only topic_heard has a topic") {
		t.Fatalf("a topic on a command trigger must be refused, got %v", err)
	}
}
```

- [ ] **Step 2:** `go test ./internal/quests/ -run TestValidateTopicHeard`. Expected: FAIL to compile (`Topic` undefined).

- [ ] **Step 3: Implement.** `triggers.go`, after `Effect`:

```go
	Topic      string      `yaml:"topic,omitempty" json:"topic,omitempty"`             // topic_heard: the NPC's topic key (#375 3b)
```

(and add `topic_heard` to the `Event` comment). `quests.go`: add `"topic_heard": true` to
`validEvents`; after the effect check add:

```go
		// A topic_heard trigger names one NPC's topic; the topic must exist on
		// that NPC (ValidateQuestRefs checks it against the dialogue files).
		if t.Event == "topic_heard" {
			if t.Mob <= 0 {
				return fmt.Errorf("quest %d (%s): trigger %d: topic_heard needs a mob (the NPC whose topic it is)", r.QuestId, r.Name, i)
			}
			if t.Topic == "" {
				return fmt.Errorf("quest %d (%s): trigger %d: topic_heard needs a topic (the topic key)", r.QuestId, r.Name, i)
			}
		} else if t.Topic != "" {
			return fmt.Errorf("quest %d (%s): trigger %d sets topic %q on a %s trigger; only topic_heard has a topic", r.QuestId, r.Name, i, t.Topic, t.Event)
		}
```

- [ ] **Step 4:** `go test ./internal/quests/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(quests): topic_heard event and topic filter (#375)` (quoted heredoc, Co-Authored-By line).

---

### Task 2: The topic must exist

**Files:** `internal/quests/validate_refs.go`, its test, `internal/questworld/questworld.go`, every test file with a full `QuestValidators` literal (`grep -rn "QuestValidators{" --include=*_test.go internal modules`).

- [ ] **Step 1: Failing test** (append to `internal/quests/validate_refs_test.go`):

```go
func TestValidateRefsTopicHeard(t *testing.T) {
	v := permissiveQuestValidators()
	v.TopicExists = func(mobId int, key string) bool { return mobId == 9112 && key == "directions" }
	q := Quest{QuestId: 9003, Name: "Ask", Steps: []QuestStep{{Id: "start"}, {Id: "asked"}},
		Triggers: []TriggerDef{{Event: "topic_heard", Mob: 9112, Topic: "directions",
			Conditions: Conditions{Has: []string{"9003-start"}}, Actions: []ActionDef{{Grant: "9003-asked"}}}}}
	if errs, _ := ValidateQuestRefs(q, v); len(errs) > 0 {
		t.Fatalf("a real topic refused: %v", errs)
	}
	q.Triggers[0].Topic = "rumours"
	errs, _ := ValidateQuestRefs(q, v)
	if !strings.Contains(strings.Join(errs, "\n"), `mob 9112 has no topic "rumours"`) {
		t.Fatalf("unknown topic not refused: %v", errs)
	}
}
```

- [ ] **Step 2:** run it; expect FAIL to compile (`TopicExists` undefined).
- [ ] **Step 3: Implement.** `QuestValidators` gains `TopicExists func(mobId int, key string) bool // the NPC's dialogue file has this topic key`. In `ValidateQuestRefs`'s trigger loop:

```go
		if t.Event == "topic_heard" && t.Mob > 0 && t.Topic != "" && c.v.MobExists(t.Mob) && !c.v.TopicExists(t.Mob, t.Topic) {
			c.errf("%s: mob %d has no topic %q", tw, t.Mob, t.Topic)
		}
```

`questworld.Validators()`: `TopicExists` loads the mob's dialogue file the way `mobHasDialogueFile` does and returns `df != nil && df.FindTopic(key) != nil`. Add `TopicExists: func(int, string) bool { return true }` to every full test literal you found.

- [ ] **Step 4:** `go build ./... && go test ./internal/quests/ ./internal/questloop/ ./internal/questworld/ ./modules/gmcp/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(quests): a topic_heard trigger's topic must exist (#375)`.

---

### Task 3: Match on the topic; the helper; the guard

**Files:** `internal/questengine/types.go`, `engine.go`, `walkthrough.go`, `command_event.go`; `internal/questloop/grants.go`; `internal/guards/command_event_guard_test.go`; Create `internal/questengine/topic_heard_test.go`.

- [ ] **Step 1: Failing test** `internal/questengine/topic_heard_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopicHeardNeedsTheTopicAndTheNPC(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 320, Name: "Ask Fred",
		Steps: []QuestStep{{Id: "320-start"}, {Id: "320-asked"}},
		Triggers: []TriggerDef{{
			Event: "topic_heard", Mob: 9112, Topic: "directions",
			Conditions: Conditions{Has: []string{"320-start"}, Missing: []string{"320-asked"}},
			Actions:    []ActionDef{{Grant: "320-asked"}},
		}},
	})
	p := newFullMockPlayer(100)
	p.quests["320-start"] = true

	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 9112, Topic: "rumours"}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("320-asked"), "another topic of the same NPC")
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 7, Topic: "directions"}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("320-asked"), "the same key on another NPC")
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 9112, Topic: "directions"}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("320-asked"))
}

func TestTopicHeardDetails(t *testing.T) {
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5200, MobId: 9112, Topic: "directions"},
		TopicHeardDetails(1, 5200, 9112, "directions"))
}
```

- [ ] **Step 2:** run; expect FAIL to compile.
- [ ] **Step 3: Implement.**
  - `EventDetails` gains `Topic string // topic_heard: the topic key (#375 3b)`.
  - `matchTriggerFields`: `if t.Topic != "" && t.Topic != d.Topic { return false }`.
  - `walkthrough.go` synthetic event literal: add `Topic: t.Topic`.
  - `questloop/grants.go` `triggerSignature`: `addStr("topic", t.Topic)`.
  - `command_event.go`, below `NotifySpellLanded`:

```go
// TopicHeardDetails builds the topic_heard event: the NPC's template id and
// the topic key, in the template room (#375 phase 3b slice 1).
func TopicHeardDetails(userId, roomId, mobId int, topic string) EventDetails {
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	return EventDetails{UserId: userId, RoomId: matchRoom, MobId: mobId, Topic: topic}
}

// NotifyTopicHeard is THE topic_heard event, raised by conversation.play once
// a chosen topic's effects have landed (a refused charge raises nothing).
func NotifyTopicHeard(user *users.UserRecord, roomId, mobId int, topic string) NotifyResult {
	if user == nil || mobId <= 0 || topic == "" {
		return NotifyResult{}
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("topic_heard", []EventDetails{TopicHeardDetails(user.UserId, roomId, mobId, topic)}, bridge, bridge)
}
```

  - Guard: `isGuardedNotify` also accepts `"topic_heard"`; add the two matching `CanFail` cases (`e.NotifyAll("topic_heard", ds, b, b)` true; `Notify("topic", ...)` false); update the message to name all three events.

- [ ] **Step 4:** `go test ./internal/questengine/ ./internal/questloop/ ./internal/guards/ -run 'TopicHeard|CommandEvent' -count=1`, then the full packages. Expected: PASS.
- [ ] **Step 5: Commit** `feat(questengine): topic_heard matching and NotifyTopicHeard (#375)`.

---

### Task 4: `play` raises the event

**Files:** `internal/conversation/talk.go`; test in `internal/conversation/` (read the existing talk tests first: `ls internal/conversation/*_test.go`, and find one that drives `play` or `choose` with a dialogue file and a user).

- [ ] **Step 1: Failing test.** Register a `topic_heard` trigger on the global engine (save and restore it as `internal/hooks/craft_quest_event_test.go:31` does, or use `questengine.ResetEngineForTest` if that is the helper), drive a topic choice through the existing test harness, and assert: the trigger fires for the chosen topic of that NPC; a topic whose `chargesGold` the player cannot pay raises nothing (the trigger does not fire). Run red.
- [ ] **Step 2: Implement.** In `play`, after `applyWithHandover` succeeds and before `npcSay`:

```go
	// Quest engine: topic_heard, once the topic's own effects have landed
	// (#375 phase 3b slice 1). A refused charge returned above.
	questengine.NotifyTopicHeard(user, room.RoomId, int(mob.MobId), t.Key)
```

Check `mob.MobId`'s type and the existing imports. If the existing harness cannot drive `play`
without a live world, say so and test `play` through the narrowest seam the package already
uses; do not build a large fixture.

- [ ] **Step 3:** `go test ./internal/conversation/ -count=1`. Expected: PASS.
- [ ] **Step 4: Commit** `feat(conversation): a played topic raises topic_heard (#375)`.

---

### Task 5: The objective marker

**Files:** `internal/dialogue/types.go`, `topics.go`, a dialogue test; `internal/conversation/playerstate.go`.

- [ ] **Step 1: Failing test** in `internal/dialogue/` (find the `TopicMarker` tests: `grep -rn "TopicMarker" internal/dialogue/*_test.go`): a `KindInfo` topic, heard, not `Marked`, with `ps.Objective` returning true for `(df.MobId, t.Key)`, gets `menu.MarkerInfo`; with `Objective` nil or false it gets `""`; the NPC's `Marker` shows the info marker when one eligible topic is an objective.
- [ ] **Step 2: Implement.** `PlayerState` gains:

```go
	// Objective reports whether hearing this NPC's topic would advance one of
	// the player's quests now (a topic_heard trigger would fire). Such a
	// topic shows [*] even when heard (#375 phase 3b slice 1). Nil: false.
	Objective func(mobId int, key string) bool
```

`TopicMarker`, before the `Marked && !heard` check:

```go
	if t.Kind == KindInfo && ps != nil && ps.Objective != nil && ps.Objective(df.MobId, t.Key) {
		return menu.MarkerInfo
	}
```

Read `Marker` and make sure it reaches the info marker through `TopicMarker` (if it duplicates the
`Marked && !heard` test, make it call `TopicMarker`). `conversation.PlayerState`:

```go
		Objective: func(mobId int, key string) bool {
			bridge := questengine.NewGameBridge(user, user.Character.RoomId)
			return questengine.GetEngine().WouldFire("topic_heard",
				questengine.TopicHeardDetails(user.UserId, user.Character.RoomId, mobId, key), bridge)
		},
```

- [ ] **Step 3:** `go test ./internal/dialogue/ ./internal/conversation/ -count=1`. Expected: PASS.
- [ ] **Step 4: Commit** `feat(dialogue): an objective topic shows [*] even when heard (#375)`.

---

### Task 6: The walkthrough hears `topic_heard` topics

**Files:** `internal/questengine/walkthrough.go`; Create `internal/questengine/walkthrough_topic_test.go`.

Read `walkthrough.go` fully (chunks of at most 400 lines) and `walkthrough_test.go`'s fixture style first.

- [ ] **Step 1: Failing tests.** A quest: offer topic on giver G grants `Q-start`; step `asked` reached by a `topic_heard` trigger (`mob: N`, `topic: rumours`, `has: [Q-start]`); hand-in on G. NPC N's file has info topic `rumours` gated `questRequired: [Q-start]`, with `setsQuestFlag` `Q-route=north`; a later step needs `has_flag: {Q-route: north}`. Expect: the quest walks to the end with no findings. Variant: `rumours` gated on a token nothing grants: expect a `walk-dead-end` at the `asked` step (the topic can never be heard). Run red.
- [ ] **Step 2: Implement.**
  - In `fireTrigger`, for `t.Event == "topic_heard"`: find `df := env.files[t.Mob]` and `tp := df.FindTopic(t.Topic)`; if either is nil, return false. `env.meetTopic(p, df, tp, own)`; `env.hear(p)`; build the dialogue state the way `fireTopic` does; if `!df.Eligible(tp, ps)` return false; if `!df.Apply(tp, ps)` return false; then send the event as now (the synthetic details carry `MobId` and `Topic`).
  - In `fireTopic`, after a successful `Apply`, also send `topic_heard` for `wy.MobId`/`wy.Topic.Key` through `eng.Notify` with the same `trailCtx` (the game raises it for every played topic; a trigger may listen to an offer or hand-in topic).
- [ ] **Step 3:** `go test ./internal/questengine/ -count=1`, then the shipped smokes at the repo root (`DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_WalkSourcesIndexShippedWorld' -count=1`). Expected: PASS, 0 new.
- [ ] **Step 4: Commit** `feat(questengine): the walk hears a topic_heard trigger's topic (#375)`.

---

### Task 7: `talk-on-quest`

**Files:** Create `internal/questloop/rule_talk.go` and its test; modify `internal/questloop/questloop.go`.

- [ ] **Step 1: Failing test:** a quest with steps start, asked, end; a topic (on any mob) granting `Q-asked` gives one finding `Rule: "talk-on-quest"` with `Where: "mob <id> topic <key>"`; a topic granting `Q-start` or `Q-end` gives none; a negative-token topic (`-Q-asked`, a revoke) gives none.
- [ ] **Step 2: Implement** `ruleTalkOnQuest(q, g)` over the `grants` index the other rules use (topics carry `w.topic != nil`): for each step index 1..last-1, each topic way granting that token yields a finding, `Msg: "a topic grants a mid-quest step; give the quest a topic_heard trigger for it instead"`. Add it to `Validate` after `ruleOneAction`.
- [ ] **Step 3:** `go test ./internal/questloop/ -count=1`. Then the shipped smoke: expect it to FAIL with 32 new `talk-on-quest` findings (the migration in Task 8 clears them). Save the list to the session scratchpad; it must match Task 8's table. Do not add them to the backlog.
- [ ] **Step 4: Commit** `feat(questloop): talk-on-quest reports a topic granting a mid-quest step (#375)` (the smoke is red until Task 8; say so in the commit body).

---

### Task 8: The migration

**Files:** the 27 dialogue files and their quests below; `_datafiles/world/dogmud/quests/loop-backlog.txt`.

| Dialogue file (mob id = stem) | Topic key | Token it grants today | Other effects (stay on the topic) |
|---|---|---|---|
| `ashwick/262.yaml` | negotiate | 16-forager | |
| `greenford/9516.yaml` | turnin | 75-intro | |
| `hartcharn/9182.yaml` | pellhandoff | 60-waybill | givesItem |
| `ironwind_steppe/241.yaml` | stand_with_sylara | 11-sided | setsQuestFlag |
| `ironwind_steppe/241.yaml` | ritual_begin | 12-ritual | |
| `ironwind_steppe/242.yaml` | stand_with_rhett | 11-sided | setsQuestFlag |
| `ironwind_steppe/242.yaml` | extraction_begin | 13-extraction | |
| `kingsbarrow_vale/9198.yaml` | verroldhandoff | 60-manifest | givesItem |
| `newcomer_antechamber/9491.yaml` | asked_28 | 28-asked | |
| `new_plymouth_crafting/9332.yaml` | q68_choice | 68-choice | |
| `new_plymouth_crafting/9338.yaml` | q68_toby | 68-evidence | givesItem |
| `new_plymouth_docks/9305.yaml` | bloom_undercover | 67-front | setsQuestFlag |
| `new_plymouth_docks/9305.yaml` | bloom_evidence | 67-front | setsQuestFlag |
| `new_plymouth_merchant/9344.yaml` | q71_horst | 71-horst | |
| `new_plymouth_merchant/9345.yaml` | bloom_lintel | 67-lintel | |
| `new_plymouth_merchant/9349.yaml` | q71_vell | 71-vell | |
| `new_plymouth_noble/9369.yaml` | bloom_delivery | 67-delivery | |
| `new_plymouth_noble/9371.yaml` | q69_lysha | 69-reading | givesItem |
| `new_plymouth_old_quarter/9379.yaml` | bloom_source | 67-source | |
| `new_plymouth_old_quarter/9380.yaml` | bloom_witness | 67-witness | givesItem |
| `new_plymouth_temple/9360.yaml` | q69_gallery | 69-gallery | |
| `north_road/287.yaml` | say_nothing | 18-answered | setsQuestFlag |
| `north_road/287.yaml` | misdirect | 18-answered | setsQuestFlag |
| `north_road/287.yaml` | tell_truth | 18-answered | setsQuestFlag |
| `stillwater/335.yaml` | sign_bounty | 19-signed | |
| `stillwater/337.yaml` | confession | 20-brindle_confession | |
| `stillwater/344.yaml` | acknowledge_spiral | 20-temple_pillars | |
| `stillwater/355.yaml` | vellahandoff | 20-vella_journal | givesItem |
| `the_confluence/9465.yaml` | q74_turnin | 74-descent | |
| `thornwall_city/107.yaml` | reassure | 8-found | |
| `thornwall_city/94.yaml` | protection_report | 10-report | |
| `thornwall_city/99.yaml` | bridge_report | 6-report | |

For each row:
- [ ] In the dialogue file, delete the topic's `grantsQuest:` line only. Keep every gate (`questRequired`, `questExcluded`, flag gates, `heardRequired`) and every other effect: the topic must still show only during the step, and its item or flag still lands before the trigger runs.
- [ ] In the quest file of the token's quest (`quests/<id>-*.yaml`), add one trigger, placed with the quest's other triggers in step order, with a comment naming the topic:

```yaml
  # <step>: hearing <NPC name>'s "<topic key>" topic (moved from the
  # topic's grantsQuest, #375 phase 3b).
  - event: topic_heard
    mob: <mob id>
    topic: <topic key>
    conditions:
      has: [<the topic's questRequired tokens that belong to this quest>]
      missing: ["<token>"]
    actions:
      - grant: "<token>"
```

  `has` lists exactly the topic's `questRequired` tokens of the same quest (the step the topic is
  gated on). If the topic's `questRequired` names no token of this quest, stop and report the row:
  the conversion needs a decision. Rows that grant the same token from several topics (11-sided,
  67-front, 18-answered) get one trigger per topic.
- [ ] Update the quest file's header comment where it says the token is granted by a topic.

Then:
- [ ] `go test ./internal/... -count=1` and the shipped smokes. `talk-on-quest` must report 0. Ways whose `Where` changed from `mob <id> topic <key>` to the new trigger's `quest <id> trigger topic_heard ...` may show as new plus stale backlog lines: for each, confirm it is the same finding under its new key (same quest, same step, same message), and replace the stale line in `loop-backlog.txt` with the new key. Any finding that is genuinely new is a defect: stop and report it.
- [ ] Commit `content: talk steps move onto topic_heard triggers (#375)`, listing the 32 rows in the body.

---

### Task 9: Review gate

Controller dispatches an Opus review of Tasks 1-8 (code, then the migration row by row against the
table and each topic's gates).

### Task 10: Docs

- [ ] `internal/questengine/context.md`, `internal/quests/context.md`, `internal/questloop/context.md` (`talk-on-quest`), `internal/dialogue/context.md` (`PlayerState.Objective`, the marker rule), `internal/conversation/context.md` (play raises topic_heard; Objective wiring). Verify every symbol by grep before naming it.
- [ ] `.claude/skills/dogmud-authoring-quests/SKILL.md`: a `topic_heard` section: the YAML shape, that a mid-quest talk step is a quest trigger (the topic keeps its gates and other effects, never `grantsQuest` for a mid step), that `[*]` shows on the objective topic, and that a topic heard before the step does not count (the player picks it again). Add `topic_heard` to the event list.
- [ ] `modules/gmcp/gmcp.Quest.go` `questEventVocab`: a `topic_heard` row (filters `mob`, `topic`). `_datafiles/html/public/static/js/quests.js`: `EVENT_FILTERS.topic_heard = ["mob","topic"]` and a `topic` text kind in `FILTER_KINDS`; `node --check` it. Update the builder Go test's event count.
- [ ] Spec section 1: describe what was built.
- [ ] `docs/README.md`: rows for new non-test files (`internal/questloop/rule_talk.go`).
- [ ] Dash check on added lines. Commit `docs: topic_heard (#375)`.

### Task 11: Gate and PR

- [ ] `dogmud-shipping` gate: gofmt, build, lint 0 issues, `go test ./... -count=1`, the shipped smokes at the repo root, the boot check. PR `--repo pruuk/DOGMud`, "Part of #375", the 32 rows, verification lines, attribution line.

### Task 12: Playtest

- [ ] Scenario with four seats (`dogmud-playtesting`; brief from memory `project-375-phase3b2-playtest-harness-lessons-2026-10-27`; only the walked quest's own step tokens in `set_quest_tokens`):
  1. Quest 18 (North Road, mob 287): from the step before `answered`, pick one of the three answers; the step advances once, the flag matches the answer, the other answers vanish.
  2. Quest 28 (newcomer antechamber, Dewey 9491): the `asked_28` step advances when the topic is heard; `[*]` shows on that topic before it is picked.
  3. Quest 60 (Hartcharn 9182 `pellhandoff`): the item arrives and the step advances once.
  4. Quest 67 (New Plymouth Docks 9305): `bloom_undercover` or `bloom_evidence` sets its flag and advances `67-front` once.
  Each seat also re-opens the talk menu after the step and confirms the topic no longer advances anything. File findings as issues.
