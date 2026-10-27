# #375 phase 3b slice 3: effect spells (`spell_landed`), implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A quest step can finish when a spell actually lands ("mend the scout": any heal that
reaches mob 9112), and the walkthrough proves a player on that quest's path holds a spell that can
do it.

**Architecture:** A new quest event `spell_landed`, raised once per target a player's spell lands
on, from `resolveSpell`'s own per-target verdicts, through a helper beside slice 2's
`NotifyCommand` and under one evaluation guard. It carries the spell id (`Noun`), a new `Effect`
(the spell's `EffectType`) and slice 2's target fields. A new trigger filter `effect:` matches it;
`ValidateQuestRefs` checks the spell id and the effect name against the loaded spells. The
walkthrough tracks the spells a walking player holds (starters, `teach_spell` actions and
`spellid` rewards met on the path) and reports `walk-no-spell` for an effect step nobody on the
path can perform.

**Tech Stack:** Go: `internal/quests`, `internal/questengine`, `internal/questloop`,
`internal/questworld`, `internal/hooks`, `internal/guards`.

**Spec:** `docs/superpowers/specs/2026-10-27-quest-arc-phase3b-trigger-kinds-design.md`, section 3.
Slice 2 (PR #693, merged `684933c25`) is the base: read `internal/questengine/command_event.go`.

**Rules for every implementer (global, enforced by hooks):** edit files only with the Edit and
Write tools, never with shell redirection, `sed -i` or Python file writes. `git add` named paths
only. Never `--no-verify`. No em or en dashes in prose or comments. Kill only processes you
started, by PID.

---

## Facts verified against source (master `684933c25`, 2026-10-27)

| Fact | Where |
|---|---|
| Loader event list: room_enter, room_gate, item_give, skill_use, mob_death, command, item_gain, quest_granted, room_interact, command_issued; `target` allowed only on `command` | `internal/quests/quests.go:113-151` |
| `TriggerDef` has `Target`; no `Effect` | `internal/quests/triggers.go:11-23` |
| `EventDetails` has `TargetKind`; no `Effect` | `internal/questengine/types.go:39-51` |
| `NotifyCommand`, `CommandDetails`, `Target`, `TargetActor`, `TargetMob`, `NoTarget`, `TargetSelf`, `WithNoun` | `internal/questengine/command_event.go` |
| `Engine.NotifyAll` raises several details under one guard; a trigger is marked visited only when it fires | `internal/questengine/engine.go:74-140` |
| Guard `TestCommandEventsOnlyThroughNotifyCommand` with matcher `isCommandNotify` (literal `"command"`, `Notify`/`NotifyAll`), helper file `internal/questengine/command_event.go` | `internal/guards/command_event_guard_test.go` |
| `resolveSpell(user, cs, spellData, room) (anyLanded bool)`: identify returns early (`:84-88`); harm-area and help-area target fill; mob loop calls `resolveAgainstMob` and reads `landed` (`:121-136`); player loop calls `resolveAgainstPlayer` (`:139-166`); summon resolves after the loop when not fumbled; Go hooks fold-anchor, fold-recall, purge-affliction return true from a switch, purge with three branches (player, mob, self) | `internal/hooks/spell_resolution.go:78-300` |
| Self-cast reaches `resolveAgainstPlayer` with the caster in `TargetUserIds` | `spell_resolution.go:139-166`; slice 2 review |
| `runSpellChannelAttack` is a package var seam tests set to a literal contest outcome | `spell_resolution.go:281-291` |
| Mob casters resolve in `resolveMobSpell`, `resolveMobSpellAgainstMob`, `resolveMobSpellAgainstPlayer` | `spell_resolution.go:574, 785, 822` |
| Shipped `effect_type` values: charm 1, condition 20, damage 12, dot 2, drain_area 1, heal 6, identify 1, knockdown 1, none 13, purge 1, shield 3 | `_datafiles/world/dogmud/spells/*.yaml` |
| `QuestValidators` holds registry checks incl. `SpellExists`; filled by `questworld.Validators()`; trigger filters checked in `ValidateQuestRefs` (`:276-282`); test helpers `permissiveQuestValidators` (`internal/quests/validate_refs_test.go:8`) and `fillValidators` (`internal/questloop/rules_more_test.go:79`) | `internal/quests/validate_refs.go:16-33, 233-300`; `internal/questworld/questworld.go:28-60` |
| `Quest.Rewards.SpellId` (yaml `spellid`) teaches on completion; trigger action `TeachSpell` (yaml `teach_spell`) | `internal/quests/quests.go:46`; `internal/quests/triggers.go:92` |
| `characters.StarterSpells` = conviction-spike, chrysalis-glow, identify; `characters` does not depend on `questworld` | `internal/characters/character.go:377-381`; `go list -deps` |
| `questloop.Sources{ItemSources, MobSpawned, ItemAvailable}`, built by `questworld` | `internal/questloop/ways.go:70-88`; `internal/questworld/sources.go:167-173` |
| Walkthrough: `walkEnv` (`walkthrough.go:102`), `newWalkEnv` indexes triggers (`:132`), `walkQuest` (`:215`), `fireWay` switch on Trigger/Topic/External (`:475-489`), `fireTrigger` builds the synthetic event (`:666-717`); `meetToken` holds a foreign quest's token as a prerequisite (`:559-567`); `walkPlayer` fields (`walkplayer.go:20-29`), `clone` (`:91`), `fingerprint` (`:102`); `walkCtx.TeachSpell` is a no-op (`walkplayer.go:179`) | `internal/questengine/` |
| `questloop.Finding{Rule, QuestId, Where, Msg}`; walkthrough rules today: walk-dead-end, walk-no-source, walk-unspawned | `walkthrough.go:18, 219, 739-746` |
| Shipped-world walk smoke: `TestSmoke_ShippedQuestsPassTheLoop` (needs `DOGMUD_BOOT_SMOKE=1`), last run "274 known backlog findings, 0 new, 0 stale" | slice 2 Task 10 report |
| Quest tests install on the global engine with `questengine.GetEngine().RegisterQuest` (see `internal/hooks/craft_quest_event_test.go:31`) | grep |

## Spec amendments made by this plan (land in Task 8)

1. **Mob casters raise no `spell_landed`.** The event is matched against the caster's quests, and a
   mob has none (slice 2 found the same for `command`). "Be healed by the priest" (the player as
   the TARGET of a mob's spell) is a different event and out of scope.
2. **No effect-name manifest.** `ValidateQuestRefs` refuses an `effect:` no loaded spell has, which
   catches a renamed effect at boot, the job the manifest was for. One mechanism, not two.
3. **Spells with no creature target** (identify, a summon, fold-anchor, fold-recall) raise one event
   aimed at nothing (`none`) when they succeed, so `noun:`/`effect:` steps work for them too.

## File structure

| File | Change |
|---|---|
| `internal/quests/triggers.go` | `Effect` field |
| `internal/quests/quests.go` | `spell_landed` event; `effect` only on it; `target` on command and spell_landed |
| `internal/quests/spell_landed_test.go` | new: loader tests |
| `internal/quests/validate_refs.go` | `EffectExists` validator; spell and effect checks on `spell_landed` |
| `internal/quests/validate_refs_test.go`, `internal/questloop/rules_more_test.go` | test validators gain `EffectExists` |
| `internal/questworld/questworld.go` | fill `EffectExists` |
| `internal/questengine/types.go`, `engine.go`, `walkthrough.go` | `Effect` on events and matching |
| `internal/questloop/grants.go` | signature includes `effect` |
| `internal/questengine/command_event.go` | `SpellLandedDetails`, `NotifySpellLanded` |
| `internal/questengine/spell_landed_test.go` | new |
| `internal/guards/command_event_guard_test.go` | also guards `spell_landed` |
| `internal/hooks/spell_resolution.go` | raise the event |
| `internal/hooks/spell_landed_event_test.go` | new |
| `internal/questloop/ways.go`, `internal/questworld/sources.go` | `Sources.SpellEffect`, `Sources.StarterSpells` |
| `internal/questengine/walkthrough.go`, `walkplayer.go` | held spells, `walk-no-spell` |
| `internal/questengine/walkthrough_spell_test.go` | new |
| context.md files, authoring skill, GMCP trigger table, spec, README | docs |

---

### Task 0: Worktree

- [ ] **Step 1:**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/375-3b-spell-landed "C:/tmp/dogmud-375-3b3" origin/master
```

- [ ] **Step 2: Baseline.** In `C:/tmp/dogmud-375-3b3`: `go build ./... && go test ./internal/quests/ ./internal/questengine/ ./internal/questloop/ ./internal/questworld/ ./internal/hooks/ ./internal/guards/`. Expected: PASS. Stop and report on any baseline failure.

---

### Task 1: The `spell_landed` event and the `effect` field

**Files:** Modify `internal/quests/triggers.go`, `internal/quests/quests.go`; Create `internal/quests/spell_landed_test.go`.

- [ ] **Step 1: Failing test** `internal/quests/spell_landed_test.go`:

```go
package quests

import (
	"strings"
	"testing"
)

func TestValidateSpellLandedTriggers(t *testing.T) {
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Effect, q.Triggers[0].Target = "spell_landed", "heal", TargetMob
	if err := q.Validate(); err != nil {
		t.Fatalf("spell_landed with effect and target refused: %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Noun = "spell_landed", "heal"
	if err := q.Validate(); err != nil {
		t.Fatalf("spell_landed with only a spell noun refused: %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Effect = "heal" // the helper's trigger is a command trigger
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only spell_landed has an effect") {
		t.Fatalf("an effect on a command trigger must be refused, got %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Target = "mob_death", TargetMob
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only command and spell_landed have a target") {
		t.Fatalf("a target on mob_death must be refused, got %v", err)
	}
}
```

Also update `TestValidateCommandTargets` in `internal/quests/target_test.go`: its last case expects
the substring `"only command has a target"`; change it to `"only command and spell_landed have a target"`.

- [ ] **Step 2:** `go test ./internal/quests/ -run 'TestValidateSpellLanded|TestValidateCommandTargets'`. Expected: FAIL to compile (`Effect` undefined).

- [ ] **Step 3: Implement.** In `triggers.go`, add after `Target`:

```go
	Effect     string      `yaml:"effect,omitempty" json:"effect,omitempty"`           // spell_landed: the spell's effect_type (heal, damage, ...) (#375 3b)
```

and add `spell_landed` to the `Event` field comment. In `quests.go`, add `"spell_landed": true` to
`validEvents`, and replace the target block with:

```go
		// Only a command or a landed spell names a target (#375 phase 3b).
		if t.Target != "" {
			if t.Event != "command" && t.Event != "spell_landed" {
				return fmt.Errorf("quest %d (%s): trigger %d sets target %q on a %s trigger; only command and spell_landed have a target", r.QuestId, r.Name, i, t.Target, t.Event)
			}
			if !slices.Contains(TriggerTargetKinds, t.Target) {
				return fmt.Errorf("quest %d (%s): trigger %d has invalid target %q (use one of %s)", r.QuestId, r.Name, i, t.Target, strings.Join(TriggerTargetKinds, ", "))
			}
		}
		// Only a landed spell has an effect; the name is checked against the
		// loaded spells by ValidateQuestRefs.
		if t.Effect != "" && t.Event != "spell_landed" {
			return fmt.Errorf("quest %d (%s): trigger %d sets effect %q on a %s trigger; only spell_landed has an effect", r.QuestId, r.Name, i, t.Effect, t.Event)
		}
```

- [ ] **Step 4:** `go test ./internal/quests/`. Expected: PASS.

- [ ] **Step 5: Commit** `feat(quests): spell_landed event and effect filter (#375)` (heredoc, Co-Authored-By line).

---

### Task 2: Reference checks for spell and effect

**Files:** Modify `internal/quests/validate_refs.go`, `internal/questworld/questworld.go`, `internal/quests/validate_refs_test.go`, `internal/questloop/rules_more_test.go`.

- [ ] **Step 1: Failing test.** Append to `internal/quests/validate_refs_test.go` (read the file first; `permissiveQuestValidators()` returns validators where everything exists):

```go
func TestValidateRefsSpellLanded(t *testing.T) {
	v := permissiveQuestValidators()
	v.SpellExists = func(id string) bool { return id == "heal" }
	v.EffectExists = func(e string) bool { return e == "heal" }

	q := Quest{QuestId: 9002, Name: "Mend", Steps: []QuestStep{{Id: "start"}, {Id: "mended"}},
		Triggers: []TriggerDef{{Event: "spell_landed", Noun: "heal", Effect: "heal",
			Conditions: Conditions{Has: []string{"9002-start"}}, Actions: []ActionDef{{Grant: "9002-mended"}}}}}
	if errs, _ := ValidateQuestRefs(q, v); len(errs) > 0 {
		t.Fatalf("a known spell and effect refused: %v", errs)
	}

	q.Triggers[0].Noun, q.Triggers[0].Effect = "mend-everything", "mending"
	errs, _ := ValidateQuestRefs(q, v)
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, `spell "mend-everything" does not exist`) {
		t.Errorf("unknown spell noun not refused: %v", errs)
	}
	if !strings.Contains(joined, `effect "mending" is not the effect_type of any spell`) {
		t.Errorf("unknown effect not refused: %v", errs)
	}
}
```

Add `"strings"` to that file's imports if missing.

- [ ] **Step 2:** `go test ./internal/quests/ -run TestValidateRefsSpellLanded`. Expected: FAIL to compile (`EffectExists` undefined).

- [ ] **Step 3: Implement.** In `validate_refs.go` add to `QuestValidators` after `SpellExists`:

```go
	EffectExists    func(effect string) bool // some loaded spell has this effect_type (spell_landed effect filter)
```

In `ValidateQuestRefs`'s trigger loop, after the skill filter check:

```go
		if t.Event == "spell_landed" {
			if t.Noun != "" && !c.v.SpellExists(t.Noun) {
				c.errf("%s: spell %q does not exist", tw, t.Noun)
			}
			if t.Effect != "" && !c.v.EffectExists(t.Effect) {
				c.errf("%s: effect %q is not the effect_type of any spell", tw, t.Effect)
			}
		}
```

In `permissiveQuestValidators` (`validate_refs_test.go:8`) add
`EffectExists: func(string) bool { return true },`; in `fillValidators`
(`internal/questloop/rules_more_test.go:79`) add the same if that function fills every field (read
it; follow its pattern). In `questworld.Validators()` build the set once:

```go
	effects := map[string]bool{}
	for _, sp := range spells.GetAllSpells() {
		if sp.EffectType != "" {
			effects[sp.EffectType] = true
		}
	}
```

and add `EffectExists: func(e string) bool { return effects[e] },`. Check the real name of the
"all spells" accessor in `internal/spells/spells.go` (`grep -n "^func Get" internal/spells/spells.go`)
and use it; if it returns ids, look each up with `spells.GetSpell`.

- [ ] **Step 4:** `go build ./... && go test ./internal/quests/ ./internal/questloop/ ./internal/questworld/`. Expected: PASS. Any other test file that builds a `QuestValidators` literal and now panics on a nil `EffectExists` must get the field too: `grep -rn "QuestValidators{" --include=*_test.go internal modules`.

- [ ] **Step 5: Commit** `feat(quests): spell_landed refs check the spell and the effect (#375)`.

---

### Task 3: Match on the effect

**Files:** Modify `internal/questengine/types.go`, `engine.go`, `walkthrough.go`, `internal/questloop/grants.go`; Create `internal/questengine/spell_landed_test.go`.

- [ ] **Step 1: Failing test** `internal/questengine/spell_landed_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

// seedMendQuest: one step, any heal that lands on mob 9112.
func seedMendQuest() *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 310, Name: "Mend the Scout",
		Steps: []QuestStep{{Id: "310-start"}, {Id: "310-mended"}},
		Triggers: []TriggerDef{{
			Event: "spell_landed", Effect: "heal", Mob: 9112,
			Conditions: Conditions{Has: []string{"310-start"}, Missing: []string{"310-mended"}},
			Actions:    []ActionDef{{Grant: "310-mended"}},
		}},
	})
	return e
}

func TestSpellLandedNeedsTheEffectAndTheMob(t *testing.T) {
	e := seedMendQuest()
	p := newFullMockPlayer(100)
	p.quests["310-start"] = true

	// A damage spell on the scout does not mend it.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "conviction-spike", Effect: "damage", TargetKind: quests.TargetMob, MobId: 9112}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("310-mended"))
	// A heal on someone else does not either.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "heal", Effect: "heal", TargetKind: quests.TargetMob, MobId: 7}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("310-mended"))
	// Any heal on the scout does.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "mend-wounds", Effect: "heal", TargetKind: quests.TargetMob, MobId: 9112}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("310-mended"))
}
```

- [ ] **Step 2:** `go test ./internal/questengine/ -run TestSpellLanded`. Expected: FAIL to compile (`Effect` undefined on `EventDetails`).

- [ ] **Step 3: Implement.** `types.go` `EventDetails` after `TargetKind`:

```go
	Effect     string // spell_landed: the spell's effect_type (#375 3b)
```

`engine.go` `matchTriggerFields`, after the target check:

```go
	if t.Effect != "" && t.Effect != d.Effect {
		return false
	}
```

`walkthrough.go` synthetic event literal (around `:705`): add `Effect: t.Effect`.
`questloop/grants.go` `triggerSignature`: `addStr("effect", t.Effect)` after `target`.

- [ ] **Step 4:** `go test ./internal/questengine/ ./internal/questloop/`. Expected: PASS.

- [ ] **Step 5: Commit** `feat(questengine): match spell_landed on the effect (#375)`.

---

### Task 4: `NotifySpellLanded` and the guard

**Files:** Modify `internal/questengine/command_event.go`, `internal/guards/command_event_guard_test.go`; Test `internal/questengine/spell_landed_test.go`.

- [ ] **Step 1: Failing test** (append):

```go
func TestSpellLandedDetails(t *testing.T) {
	d := SpellLandedDetails(1, 5305, "heal", "heal", TargetActor(1, 1, 0))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Noun: "heal", Effect: "heal", TargetKind: quests.TargetSelf}, d)

	d = SpellLandedDetails(1, 5305, "identify", "identify", Target{})
	assert.Equal(t, quests.TargetNone, d.TargetKind)
	assert.Equal(t, "identify", d.Noun)
	assert.Empty(t, d.Command, "spell_landed is not a command event")
}
```

- [ ] **Step 2:** `go test ./internal/questengine/ -run TestSpellLandedDetails`. Expected: FAIL to compile.

- [ ] **Step 3: Implement** in `command_event.go`, below `NotifyCommand`:

```go
// SpellLandedDetails builds the spell_landed event for one target: Noun is
// the canonical spell id and Effect the spell's effect_type (#375 phase 3b
// slice 3). The target's own Noun is replaced by the spell id.
func SpellLandedDetails(userId, roomId int, spellId, effect string, t Target) EventDetails {
	d := CommandDetails(userId, roomId, "", t.WithNoun(spellId))
	d.Effect = effect
	return d
}

// NotifySpellLanded is THE spell_landed event: one detail per target the
// spell landed on, under one guard (NotifyAll), so a trigger fires at most
// once per cast. A cast that landed on nothing raises nothing; a spell with
// no creature target that succeeded passes NoTarget. Raised only for player
// casters: the event is matched against the caster's quests.
func NotifySpellLanded(user *users.UserRecord, roomId int, spellId, effect string, targets ...Target) NotifyResult {
	if user == nil || len(targets) == 0 {
		return NotifyResult{}
	}
	details := make([]EventDetails, 0, len(targets))
	for _, t := range targets {
		details = append(details, SpellLandedDetails(user.UserId, roomId, spellId, effect, t))
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("spell_landed", details, bridge, bridge)
}
```

- [ ] **Step 4: Guard.** In `internal/guards/command_event_guard_test.go`, make `isCommandNotify`
accept the literal `"spell_landed"` as well as `"command"` (rename the function to
`isGuardedNotify` and update its comment and the test's message to name both events); add two
cases to `TestCommandEventGuardCanFail`: `e.NotifyAll("spell_landed", ds, b, b)` true and
`questengine.GetEngine().Notify("spell_cast", d, b, b)` false. The helper file stays the only
allowed file.

- [ ] **Step 5:** `go test ./internal/questengine/ ./internal/guards/ -run 'SpellLanded|CommandEvent'`. Expected: PASS.

- [ ] **Step 6: Commit** `feat(questengine): NotifySpellLanded; the guard covers spell_landed (#375)`.

---

### Task 5: `resolveSpell` raises the event

**Files:** Modify `internal/hooks/spell_resolution.go`; Create `internal/hooks/spell_landed_event_test.go`.

- [ ] **Step 1: Read** `spell_resolution.go:78-300` and the existing tests that drive `resolveSpell`
through the `runSpellChannelAttack` seam (`grep -n "runSpellChannelAttack = " internal/hooks/*_test.go`;
`channel_defence_routing_test.go:413` is one). Reuse their fixture builders for a caster, a room and a
mob, and their way of forcing a contest result.

- [ ] **Step 2: Failing tests** in `internal/hooks/spell_landed_event_test.go`. Register a quest on
the global engine as `internal/hooks/craft_quest_event_test.go:31` does (and restore it the same
way), with one `spell_landed` trigger per case, and drive `resolveSpell` directly:
  1. A help spell with `EffectType: "heal"` cast at a mob of template T: a trigger `effect: heal, mob: T` fires.
  2. A harm spell at that mob with the seam forced to a defended/miss result: a trigger `noun: <spell id>` does NOT fire.
  3. The same harm spell with the seam forced to a hit: the trigger fires, once, even with two mobs of template T both hit (assert one grant and, if the action is `give_gold`, the gold once).
  4. A self-cast heal (`TargetUserIds: [caster]`): a trigger `target: self, effect: heal` fires.
  5. `identify` (`EffectType: "identify"`): a trigger `noun: identify` fires with no target filter.
Each assertion must fail before Step 3 (no event is raised today).

- [ ] **Step 3: Implement.** At the top of `resolveSpell`, after `magnitude := ...`:

```go
	// Quest engine: spell_landed, one target per creature the spell landed
	// on (#375 phase 3b slice 3). Collected below and raised once on every
	// return, so the identify and Go-hook early returns raise it too.
	var landedOn []questengine.Target
	defer func() {
		questengine.NotifySpellLanded(user, room.RoomId, cs.SpellId, spellData.EffectType, landedOn...)
	}()
```

Then append at each landing:
- identify branch, before `return true`: `landedOn = append(landedOn, questengine.NoTarget())`
- mob loop, after `resolveAgainstMob`: `if landed { landedOn = append(landedOn, questengine.TargetMob(mob)) }`
- player loop, after `resolveAgainstPlayer`: `if landed { landedOn = append(landedOn, questengine.TargetActor(user.UserId, targetUser.UserId, 0)) }`
- summon: after `resolveCompanionSummon(...)` inside its `if !castFumbled && ...` block: `landedOn = append(landedOn, questengine.NoTarget())`
- fold-anchor and fold-recall cases, before `return true`: `landedOn = append(landedOn, questengine.TargetSelf())`
- purge-affliction: inside each branch, where `resolvePurgeAffliction` is called (the player branch only when `t.stillPresent`, the mob branch likewise, the self branch always), append `questengine.TargetActor(user.UserId, targetUser.UserId, 0)`, `questengine.TargetMob(tMob)`, or `questengine.TargetSelf()` respectively.

Use `cs.SpellId` for the spell id only if it is the canonical id; confirm with
`grep -n "SpellId" internal/activity/*.go` and the cast site (`usercommands/skill.cast.go`, where
`SpellId: result.SpellInfo.SpellId`). If `spellData.SpellId` exists, prefer it.

Do not touch `resolveMobSpell*` (spec amendment 1).

- [ ] **Step 4:** `go test ./internal/hooks/ -count=1`. Expected: PASS, including every existing spell test.

- [ ] **Step 5: Commit** `feat(hooks): resolveSpell raises spell_landed per landed target (#375)`.

---

### Task 6: Walkthrough reachability

**Files:** Modify `internal/questloop/ways.go`, `internal/questworld/sources.go`, `internal/questengine/walkthrough.go`, `internal/questengine/walkplayer.go`; Create `internal/questengine/walkthrough_spell_test.go`.

- [ ] **Step 1: Sources.** In `questloop.Sources` add:

```go
	// SpellEffect is a spell's effect_type ("" for an unknown spell), for the
	// walk's spell_landed reachability check. Nil: the check is skipped.
	SpellEffect func(spellId string) string
	// StarterSpells are the spells every new character holds.
	StarterSpells []string
```

In `questworld/sources.go`, fill `SpellEffect` from `spells.GetSpell(id)` (`""` when nil) and
`StarterSpells: characters.StarterSpells` (confirm the import compiles without a cycle; if
`questworld` cannot import `characters`, stop and report).

- [ ] **Step 2: Failing tests** `internal/questengine/walkthrough_spell_test.go`. Read
`walkthrough_test.go` first and build fixtures the way it does (a quest with an offer topic on a
giver, and a hand-in), then three cases, each run through `Walk`:
  1. Quest A: offer, step `mended` reached by `spell_landed` with `effect: heal`, hand-in. Sources: `SpellEffect` maps `heal`→`heal`, `conviction-spike`→`damage`; `StarterSpells: [conviction-spike]`. Expect a finding `Rule: "walk-no-spell"` for A naming the step, and the quest not completing.
  2. Same, plus a `teach_spell: heal` action on the trigger that grants A's first step after the offer (or on a quest_granted trigger for `A-start`). Expect no `walk-no-spell`.
  3. Same as 1, but A's offer topic has `questRequired: ["B-end"]` and quest B has `rewards.spellid: heal`. Expect no `walk-no-spell`.
  Also: with `SpellEffect` nil, case 1 reports no `walk-no-spell` (the check is skipped, as other nil Sources funcs skip theirs).

- [ ] **Step 3:** `go test ./internal/questengine/ -run WalkSpell`. Expected: FAIL.

- [ ] **Step 4: Implement.**
  - `walkPlayer` gains `spells map[string]bool` (init in `newWalkPlayer`; copy in `clone`; include
    the sorted keys in `fingerprint`). `walkCtx.TeachSpell(id)` sets `c.p.spells[id] = true`.
  - `walkEnv` gains `teachers map[string][]string` (spell id to tokens whose holder knows the
    spell) and `noSpell []questloop.Finding`. In `newWalkEnv`, for each quest: for each trigger
    whose actions (walk the same way the setter index does, including sequence `OnComplete`) have
    `TeachSpell` S, add every token that trigger grants to `teachers[S]`; for `Rewards.SpellId` S,
    add the quest's final step token.
  - `func (env *walkEnv) holds(p *walkPlayer, spellId string) bool`: true when the id is in
    `env.src.StarterSpells`, in `p.spells`, or any `teachers[spellId]` token satisfies `p.HasQuest`.
  - `func (env *walkEnv) spellReachable(p *walkPlayer, t *quests.TriggerDef) bool`: true when
    `env.src.SpellEffect == nil`; otherwise when some held spell (iterate the union of starters,
    `p.spells` and `teachers` keys) satisfies `t.Noun == "" || id == t.Noun` and
    `t.Effect == "" || env.src.SpellEffect(id) == t.Effect`.
  - In `fireWay`'s `case wy.Trigger != nil:`, before `fireTrigger`:

```go
		if wy.Trigger.Event == "spell_landed" && !env.spellReachable(p, wy.Trigger) {
			env.noSpell = append(env.noSpell, questloop.Finding{Rule: "walk-no-spell", QuestId: own, Where: wy.Where,
				Msg: fmt.Sprintf("needs a spell that lands with effect %q / spell %q, and no spell a player holds on this path (starters, teach_spell, spellid rewards) does", wy.Trigger.Effect, wy.Trigger.Noun)})
			return trail
		}
```

  - At the end of `walkQuest`, `out = append(out, env.noSpell...)` and reset `env.noSpell = nil`;
    `dedupeFindings` collapses repeats. Add `walk-no-spell` to the `Walk` doc comment's rule list.
  - If the questloop backlog machinery needs rule names registered (`grep -rn "walk-unspawned" internal/questloop`), register `walk-no-spell` the same way.

- [ ] **Step 5:** `go test ./internal/questengine/ ./internal/questloop/ ./internal/questworld/ -count=1`, then `DOGMUD_BOOT_SMOKE=1 go test ./internal/... -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_WalkSourcesIndexShippedWorld' -count=1`. Expected: PASS with "0 new" (no shipped trigger uses `spell_landed`).

- [ ] **Step 6: Commit** `feat(questengine): walkthrough checks a spell_landed step is reachable (#375)`.

---

### Task 7: Review gate

Controller dispatches an Opus review of Tasks 1-6 before docs.

### Task 8: Docs

- [ ] `internal/questengine/context.md`: `spell_landed`, `EventDetails.Effect`, `SpellLandedDetails`, `NotifySpellLanded`, the walk's held spells and `walk-no-spell`. Verify every symbol with grep before naming it.
- [ ] `internal/quests/context.md`: the `effect` field, `spell_landed`, `target` on spell_landed, `EffectExists`.
- [ ] `internal/questloop/context.md`: `Sources.SpellEffect`, `Sources.StarterSpells`.
- [ ] `internal/hooks/context.md` (if it lists resolveSpell's duties): resolveSpell raises spell_landed.
- [ ] `.claude/skills/dogmud-authoring-quests/SKILL.md`: under "Command trigger filters", add a `spell_landed` section with the "mend the scout" example (`event: spell_landed`, `effect: heal`, `mob: 9112`), the reachability rule (teach the spell earlier, or gate the offer on a quest that teaches it and breadcrumb to the teacher), and that `command: cast` stays the attempt event. Add `spell_landed` to its event-name list.
- [ ] `modules/gmcp/gmcp.Quest.go` trigger table: a `spell_landed` row (filters: noun as spell id, effect, target, mob, item, room).
- [ ] Spec section 3: the three amendments at the top of this plan.
- [ ] `docs/README.md`: this plan's row is added with the plan commit; adjust if scope changed.
- [ ] Dash check over touched files. Commit `docs: spell_landed (#375)`.

### Task 9: Gate and PR

- [ ] `dogmud-shipping` pre-push gate: gofmt, build, `golangci-lint run --new-from-merge-base=origin/master` (0 issues), `go test ./... -count=1`, the detached-worktree boot check. PR with `--repo pruuk/DOGMud`, body "Part of #375", the amendments, the verification lines, the attribution line.

### Task 10: Playtest

- [ ] One-seat scenario (`dogmud-playtesting`; brief seats with the traps in memory
`project-375-phase3b2-playtest-harness-lessons-2026-10-27`): an `early` character in room 5305
with `set_quest_tokens: ["44-start"]` and `grant_spells: {conviction-spike: 1, heal: 1, chrysalis-glow: 1}`.
It casts Conviction Spike at the mote (quest 44's cast step still advances once), casts Mend Flesh
on itself, casts Chrysalis Glow with no target, and lets a cast miss or be interrupted if it can;
every cast must read normally and none may error. File findings as issues.
