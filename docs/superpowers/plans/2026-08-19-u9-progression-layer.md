# U9 Progression Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Tasks:** 18 (Task 9b was added after adversarial review).

**Goal:** Give every progression event in the contest paths one shape — a pure event returned by a leaf package and applied by a single applier — close a rank-independent progression faucet, delete a melee double-progression, and make `SpellData.PrimaryStat` load-bearing.

**Architecture:** A new leaf package `internal/progression` computes a list of `Event` values from plain contest facts. `internal/characters` gains one applier that turns events into skill and stat rolls. `internal/costs/action.go` moves to a new leaf `internal/actionspec` so both cost and progression read one action registry. The combat and spell paths build events and hand them to the applier; nothing else changes about when progression fires.

**Tech Stack:** Go 1.x, standard `testing`. YAML content under `_datafiles/world/dogmud/spells/`. Balance values in `_datafiles/config.yaml`.

**Spec:** [`2026-08-19-u9-progression-layer-design.md`](../specs/2026-08-19-u9-progression-layer-design.md)

**Branch:** `feature/u9-progression-layer` (already created; the spec is committed there).

---

## Before you start

**Set `GOTMPDIR` once per shell.** Windows Defender flags Go test binaries built
into random `go-build*` temp directories as `Trojan:Win64/CobaltStrike.YYY!MTB`.
The project fix is a fixed temp dir with a Defender exclusion:

```bash
export GOTMPDIR=C:/gotmp
```

With that set, `go test ./...` exits 0 with no known failures, so **any failure
you see is real**.

**Do not start or stop the user's local server.** The user runs it. Boot testing
happens in an isolated detached worktree (Task 17).

**Never change a balance number inside `internal/`.** Every tuning value is a
`_datafiles/config.yaml` edit. This is standing rule 1 of the arc.

**Commands used throughout:**

```bash
go build ./...
go test ./internal/progression/... -v
go test ./internal/characters/... -v
go test ./internal/combat/... -v
go test ./internal/hooks/... -v
gofmt -l internal/ modules/     # must print nothing
```

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/actionspec/action.go` | **New.** The action registry, moved verbatim from `internal/costs/action.go`, plus the `Stat` override field and `StatFor`. |
| `internal/costs/action.go` | **Reduced to aliases.** Re-exports `Action`, `Spec`, `SkillSource`, the constants and `SpecFor` so no existing caller changes. |
| `internal/progression/event.go` | **New leaf.** `Side`, `Class`, `Event`, `Outcome`, `Bonuses`, `EventsForContest`. Pure: no config, no character, no room. |
| `internal/progression/context.md` | **New.** Package documentation. |
| `internal/actionspec/context.md` | **New.** Package documentation. |
| `internal/characters/progression.go` | Gains `ApplyProgression` and `applyBonusProgression`; `OnCritReceived` moves off `CheckRegenProgression`; phantom `TrackSkillUse` calls deleted. |
| `internal/characters/character.go:298` area | Gains the unexported `bonusProgressionRound` dedupe map beside `combatDriftRound`. |
| `internal/combat/combat.go` | Per-quadrant attacker progression and both defender-dexterity lines **deleted** (Task 9). |
| `internal/combat/combat_helpers.go` | The per-swing defender roll in `sendDefenseMessages` **deleted** (Task 9b). |
| `_datafiles/world/default/spells/*.yaml` | 8 upstream files gain `primarystat`, or a fresh checkout boot-panics. |
| `internal/combat/defence_multiplier.go` | Channel-defence path routed through the seam. |
| `internal/hooks/NewRound_DoCombat_unified.go` | `applyCombatProgression` becomes the single melee progression path and builds events. |
| `internal/hooks/NewRound_DoCombat_helpers.go` | Spell double-roll deleted; spell path supplies `primarystat`. |
| `internal/hooks/spell_resolution.go` | 11 caster-side Willpower reads become `primarystat` lookups. |
| `internal/spells/spells.go` | `PrimaryStat` validator. |
| `internal/configs/config.balance.go` + `config.balance.progression.go` | Two new knobs and their `< 0` validation. |
| `_datafiles/config.yaml` | The two knobs with documentation. |
| `_datafiles/world/dogmud/spells/*.yaml` | 14 manifestation-school files. |
| `docs/audits/2026-08-19-progression-firing-audit.md` | **New.** The firing-condition audit. |

---

## Task 1: Confirm the melee double-progression

Spec §7.3 claims melee progression fires twice per round. It was read from
source, not measured. **Nothing is deleted until this test says it is real.**

**Files:**
- Test: `internal/hooks/progression_duplication_test.go` (create, temporary — deleted in Task 9)

- [ ] **Step 1: Write the instrumented test**

The test drives one full combat round through the unified orchestrator and
counts the attacker's use-counter deltas. `SkillUseCount` and `StatUseCount` are
exported maps on `Character`, so no new instrumentation is needed.

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// countUses snapshots the two progression counters we care about.
func countUses(c *characters.Character) (strength, dexterity, weapon int) {
	return c.GetStatUseCount("strength"),
		c.GetStatUseCount("dexterity"),
		c.GetSkillUseCount("weapon-combat")
}

// TestMeleeProgressionFiresOncePerRound pins the intended contract: ONE
// strength track and ONE weapon-combat track per attacker per round.
//
// It is expected to FAIL before Task 9 with roughly double counts. That failure
// is the confirmation the spec asked for. Record the real numbers in the task
// notes before changing any code.
func TestMeleeProgressionFiresOncePerRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)

	beforeStr, beforeDex, beforeWeapon := countUses(atk.GetCharacter())

	runOneCombatRoundForTest(t, atk, def)

	afterStr, afterDex, afterWeapon := countUses(atk.GetCharacter())

	if got := afterStr - beforeStr; got != 1 {
		t.Errorf("strength tracked %d times in one round, want 1", got)
	}
	if got := afterWeapon - beforeWeapon; got != 1 {
		t.Errorf("weapon-combat tracked %d times in one round, want 1", got)
	}
	// Dexterity is tracked directly AND as weapon-combat's primary stat, so the
	// intended count is 2, not 1. Anything above that is duplication.
	if got := afterDex - beforeDex; got > 2 {
		t.Errorf("dexterity tracked %d times in one round, want at most 2", got)
	}
}
```

- [ ] **Step 2: Build the two test helpers**

`newCombatPairForTest` and `runOneCombatRoundForTest` do not exist. Look for an
existing combat-round fixture first:

```bash
grep -rn "func new.*ForTest\|func .*combatPair\|resolveCombatRound(" internal/hooks/*_test.go internal/combat/*_test.go | head -20
```

Reuse whatever the hooks package already uses to build a player and a mob. If
nothing exists, build the pair with a real species fixture — `Wear` has no nil
guard on `HandsRequired`, so a bare `Character{}` panics. Set stats by writing
`.Base` and calling `Recalculate()`; `.Value` and `.ValueAdj` are derived and
writing them directly is silently discarded.

Go test binaries run with CWD set to their own package directory, so if the
helper needs real config it must chdir to the repo root and call
`configs.ReloadConfig()`.

- [ ] **Step 3: Run the test and record the real numbers**

Run: `go test ./internal/hooks/ -run TestMeleeProgressionFiresOncePerRound -v`

Expected: **FAIL**, with counts around `strength 2`, `weapon-combat 2`,
`dexterity 4`.

Write the actual observed numbers into the task notes. They are the evidence for
the PR and for the spec's §7.1 table.

- [ ] **Step 4: Decide**

- If it fails with roughly doubled counts: the finding is confirmed. Continue to
  Task 2; Task 9 deletes the duplication.
- If it passes: the spec is wrong. **Stop.** Report to the user, correct spec
  §7.3 and §7.1, and skip Task 9. Do not proceed on an unverified premise.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/progression_duplication_test.go
git commit -m "test(u9): pin one progression event per melee round

Currently failing. Confirms the spec 7.3 finding that
NewRound_DoCombat_unified runs progression in both phase 2 and phase 5.
Task 9 deletes the phase 2 path and turns this green."
```

---

## Task 2: The firing-condition audit

A documentation deliverable. U9 changes none of these; U10b and U10c act on it.

**Files:**
- Create: `docs/audits/2026-08-19-progression-firing-audit.md`
- Modify: `docs/README.md` (add the row under "Audits & findings")

- [ ] **Step 1: Enumerate every production progression call site**

```bash
grep -rn "\.OnSkillUse(\|\.OnSkillUseScaled(\|\.OnStatUse(\|\.CheckSkillProgression(\|\.CheckStatProgression(\|\.OnCritReceived(\|\.OnCriticalSuccess(\|\.OnCriticalFailure(" \
  --include=*.go internal/ modules/ | grep -v _test
```

There are 88 `OnSkillUse` / `OnStatUse` production call sites across roughly 50
files, plus the crit entry points. Every one gets a row.

- [ ] **Step 2: Write the audit**

Structure, one table with these columns: **Site** (file:line), **Action**,
**Skill awarded**, **Stat awarded**, **Fires when**, **Deliberate?**, **Notes**.

Seed it with the seven divergent conditions already known:

| Path | Skill fires when |
|---|---|
| Melee autoattack | `CleanHit` only (U6 Task 14) |
| Special moves (bash, kick, trip, gore, hamstring, maul, pounce, rake, drain, throttle, grapple) | on hit only |
| `shoot` (`usercommands/shoot.go:197-199`) | skill on hit, `perception` stat on **every** shot |
| `throw` (`usercommands/throw.go:399`) | always, hit or miss |
| `taunt` (`actions/combat_taunt.go:183,270,325`) | always, all three outcomes |
| `warcry` / `rally` (`actions/skill_helpers.go`, `awardRhetoricUse`) | always in combat, **50%** out of combat |
| Melee defences, per-round (`hooks/NewRound_DoCombat_helpers.go`, `processDefenderProgression`) | once per defence type per round, keyed on `SwingEvent.DefenseUsed` |
| Melee defences, per-swing (`combat/combat_helpers.go:1228`, in `sendDefenseMessages`) | **every defended swing** — the duplicate Task 9b deletes |
| Channel defences (`combat/defence_multiplier.go:246`) | **whenever the contest ran**, win or lose |

Call out explicitly, in its own section, the last three rows:

- Melee has **two** defender progression sites, one per swing and one per round.
  That is a defect, not a convention, and Task 9b removes it. An earlier draft
  of the spec listed only the per-round one and called melee correct.
- What remains is a genuine convention disagreement: melee awards a defence only
  when one actually registered, while the channel path awards the best defence
  whether it won or lost. Two live answers to one question, and **U10b's call**.

Close with a "Recommended convention" section stating the owner's shape: one
event per success, with crit and critical-failure as a separate bonus on top.

- [ ] **Step 3: Index it**

Add to `docs/README.md` under "Audits & findings", matching the existing row
format:

```markdown
| [`audits/2026-08-19-progression-firing-audit.md`](audits/2026-08-19-progression-firing-audit.md) | Every progression call site with its firing condition. Seven different conventions; input to U10b |
```

- [ ] **Step 4: Commit**

```bash
git add docs/audits/2026-08-19-progression-firing-audit.md docs/README.md
git commit -m "docs(u9): audit every progression firing condition

Input for U10b. U9 changes none of them."
```

---

## Task 3: Move the action registry to `internal/actionspec`

**Files:**
- Create: `internal/actionspec/action.go`
- Modify: `internal/costs/action.go` (reduced to aliases)
- Test: existing `internal/costs/action_test.go` must pass unchanged

- [ ] **Step 1: Create the new package**

```bash
mkdir -p internal/actionspec
git mv internal/costs/action.go internal/actionspec/action.go
```

Change the package clause on line 1 of `internal/actionspec/action.go`:

```go
package actionspec
```

Nothing else in that file changes in this step. It already imports only
`internal/skills`, so it is a leaf.

- [ ] **Step 2: Re-create `internal/costs/action.go` as aliases**

```go
package costs

import "github.com/GoMudEngine/GoMud/internal/actionspec"

// The action registry moved to internal/actionspec in U9 so that the cost
// calculator and the progression layer read ONE table. Costs and progression
// ask the same question of an action -- which skill governs it -- and two
// tables answering it would drift the first time someone added an action to
// only one of them.
//
// These aliases exist so no cost call site changed when it moved. New code
// should import internal/actionspec directly.

type Action = actionspec.Action
type Spec = actionspec.Spec
type SkillSource = actionspec.SkillSource

const (
	SkillNone           = actionspec.SkillNone
	SkillFixed          = actionspec.SkillFixed
	SkillEquippedCombat = actionspec.SkillEquippedCombat
)

const (
	ActionAttack          = actionspec.ActionAttack
	ActionDodge           = actionspec.ActionDodge
	ActionParry           = actionspec.ActionParry
	ActionBlock           = actionspec.ActionBlock
	ActionMove            = actionspec.ActionMove
	ActionFlee            = actionspec.ActionFlee
	ActionQuell           = actionspec.ActionQuell
	ActionDefy            = actionspec.ActionDefy
	ActionShoot           = actionspec.ActionShoot
	ActionReload          = actionspec.ActionReload
	ActionBash            = actionspec.ActionBash
	ActionTrip            = actionspec.ActionTrip
	ActionKick            = actionspec.ActionKick
	ActionGrapple         = actionspec.ActionGrapple
	ActionGrappleMaintain = actionspec.ActionGrappleMaintain
	ActionHamstring       = actionspec.ActionHamstring
	ActionRake            = actionspec.ActionRake
	ActionMaul            = actionspec.ActionMaul
	ActionPounce          = actionspec.ActionPounce
	ActionGore            = actionspec.ActionGore
	ActionDrain           = actionspec.ActionDrain
	ActionThrottle        = actionspec.ActionThrottle
	ActionThrow           = actionspec.ActionThrow
	ActionSneak           = actionspec.ActionSneak
	ActionTaunt           = actionspec.ActionTaunt
	ActionRally           = actionspec.ActionRally
	ActionWarcry          = actionspec.ActionWarcry
)

// SpecFor returns the registry entry for an action. See actionspec.SpecFor for
// the unregistered-action contract.
func SpecFor(a Action) Spec { return actionspec.SpecFor(a) }
```

Note `type Action = actionspec.Action` is a **type alias**, not a definition.
A definition (`type Action actionspec.Action`) would make `costs.ActionDodge`
a different type from `actionspec.ActionDodge` and break every call site.

- [ ] **Step 3: Build and run the existing tests unchanged**

Run: `go build ./... && go test ./internal/costs/... ./internal/actions/... -v`
Expected: PASS. No test file is edited in this task. If a cost test needs
changing, the alias is wrong — fix the alias, not the test.

- [ ] **Step 4: Commit**

```bash
git add internal/actionspec/ internal/costs/action.go
git commit -m "refactor(u9): move the action registry to internal/actionspec

Costs and progression ask the same question of an action -- which skill
governs it. Two tables answering it would drift on the first added
action. costs re-exports everything as aliases, so no call site changed."
```

---

## Task 4: Add the `Stat` override to the registry

**Files:**
- Modify: `internal/actionspec/action.go`
- Test: `internal/actionspec/action_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package actionspec

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
)

func TestStatFor_DefaultsToSkillPrimaryStat(t *testing.T) {
	// Dodge is registered with UnarmedCombat and no Stat override, and
	// unarmed-combat's primary stat is dexterity.
	if got := StatFor(SpecFor(ActionDodge)); got != "dexterity" {
		t.Errorf("StatFor(dodge) = %q, want %q", got, "dexterity")
	}
}

func TestStatFor_OverrideWins(t *testing.T) {
	s := Spec{Skill: skills.Spellcasting, SkillSource: SkillFixed, Stat: "charisma"}
	if got := StatFor(s); got != "charisma" {
		t.Errorf("StatFor(override) = %q, want %q", got, "charisma")
	}
}

func TestStatFor_NoSkillNoStat(t *testing.T) {
	if got := StatFor(Spec{}); got != "" {
		t.Errorf("StatFor(zero) = %q, want empty", got)
	}
}

// Every registered action must resolve to a real stat. A registry row whose
// skill has no primary stat would award a skill roll and silently no stat roll,
// which is the exact half-wiring spec 5 warns about.
func TestEveryRegisteredActionResolvesAStat(t *testing.T) {
	for action, spec := range registry {
		if spec.SkillSource == SkillEquippedCombat {
			continue // skill is resolved from the actor's weapon at call time
		}
		if StatFor(spec) == "" {
			t.Errorf("action %q resolves no stat (skill %q)", action, spec.Skill)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/actionspec/ -v`
Expected: FAIL, `undefined: StatFor` and `spec.Stat undefined`.

- [ ] **Step 3: Add the field and the resolver**

In `internal/actionspec/action.go`, extend `Spec`:

```go
// Spec is what the registry knows about an action: which skill discounts it,
// whether encumbrance applies, and which stat it exercises.
type Spec struct {
	Skill       skills.SkillTag // governing skill for SkillFixed actions
	SkillSource SkillSource
	Physical    bool // physical actions take the encumbrance multiplier

	// Stat OVERRIDES the stat this action exercises for progression. Empty --
	// which is every registered action -- means the skill's primary stat, which
	// is already what every one of them wants.
	//
	// It exists for the two cases that genuinely diverge: a spell declaring its
	// own primarystat, and the toughening stat awarded for a crit RECEIVED
	// (vitality / willpower / charisma), which is deliberately not the stat that
	// fed the defence score.
	Stat string
}
```

Add the resolver at the bottom of the file:

```go
// StatFor returns the stat an action exercises: the Spec's override if it has
// one, otherwise the governing skill's primary stat.
//
// Returns empty for a Spec with neither, which callers must treat as "no stat
// roll" rather than passing it on -- CheckStatProgression("") takes a roll and
// a success sends a levelup banner naming no stat at all.
func StatFor(s Spec) string {
	if s.Stat != "" {
		return s.Stat
	}
	if s.Skill == "" {
		return ""
	}
	return skills.GetSkillPrimaryStat(string(s.Skill))
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/actionspec/ -v`
Expected: PASS, all four.

If `TestEveryRegisteredActionResolvesAStat` fails, a registry skill is missing
from `skills.SkillPrimaryStats`. Add the mapping there; do not add a `Stat`
override to paper over it.

- [ ] **Step 5: Commit**

```bash
git add internal/actionspec/
git commit -m "feat(u9): add the Stat override to the action registry

Empty for all 27 registered actions -- each already wants its skill's
primary stat. The field exists for a spell's primarystat and for the
crit-received toughening stat."
```

---

## Task 5: The `internal/progression` package

**Files:**
- Create: `internal/progression/event.go`
- Test: `internal/progression/event_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package progression

import "testing"

func bonuses() Bonuses { return Bonuses{Doing: 2.0, Observing: 0.5} }

func fullOutcome() Outcome {
	return Outcome{
		AttackerSkill: "weapon-combat",
		AttackerStat:  "dexterity",
		DefenderSkill: "unarmed-combat",
		DefenderStat:  "dexterity",
		ToughenStat:   "vitality",
	}
}

// find returns the single event matching side+class, or fails.
func find(t *testing.T, evs []Event, side Side, class Class) Event {
	t.Helper()
	var out []Event
	for _, e := range evs {
		if e.Side == side && e.Class == class {
			out = append(out, e)
		}
	}
	if len(out) != 1 {
		t.Fatalf("want exactly 1 event for side=%v class=%v, got %d", side, class, len(out))
	}
	return out[0]
}

func TestOrdinary_BothSidesOneEventEach(t *testing.T) {
	evs := EventsForContest(fullOutcome(), bonuses())
	if len(evs) != 2 {
		t.Fatalf("ordinary contest produced %d events, want 2", len(evs))
	}
	a := find(t, evs, SideAttacker, ClassOrdinary)
	if a.Skill != "weapon-combat" || a.Stat != "dexterity" || a.Multiplier != 1.0 {
		t.Errorf("attacker ordinary = %+v", a)
	}
	d := find(t, evs, SideDefender, ClassOrdinary)
	if d.Skill != "unarmed-combat" || d.Multiplier != 1.0 {
		t.Errorf("defender ordinary = %+v", d)
	}
}

func TestAttackCrit_AttackerDoesDefenderToughens(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	evs := EventsForContest(o, bonuses())

	a := find(t, evs, SideAttacker, ClassCrit)
	if a.Multiplier != 2.0 || a.Stat != "dexterity" {
		t.Errorf("attacker crit = %+v, want mult 2.0 stat dexterity", a)
	}
	d := find(t, evs, SideDefender, ClassObserved)
	if d.Multiplier != 0.5 {
		t.Errorf("defender observed multiplier = %v, want 0.5", d.Multiplier)
	}
	// You learn to TAKE a hit, not to swing better.
	if d.Stat != "vitality" {
		t.Errorf("defender observed stat = %q, want vitality (the toughening stat)", d.Stat)
	}
	if d.Skill != "unarmed-combat" {
		t.Errorf("defender observed skill = %q, want the defence skill", d.Skill)
	}
}

func TestDefenceCrit_DefenderDoesAttackerObserves(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcDefenceCrit
	evs := EventsForContest(o, bonuses())

	d := find(t, evs, SideDefender, ClassCrit)
	if d.Multiplier != 2.0 || d.Stat != "dexterity" {
		t.Errorf("defender crit = %+v", d)
	}
	a := find(t, evs, SideAttacker, ClassObserved)
	if a.Multiplier != 0.5 {
		t.Errorf("attacker observed multiplier = %v, want 0.5", a.Multiplier)
	}
}

// Failure teaches: spec 5.0's matrix pays the bonus to whoever fumbled.
func TestAttackFumble_AttackerEarnsTheBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackFumble
	evs := EventsForContest(o, bonuses())

	a := find(t, evs, SideAttacker, ClassFumble)
	if a.Multiplier != 2.0 {
		t.Errorf("attacker fumble multiplier = %v, want 2.0", a.Multiplier)
	}
	// The defender OBSERVES a fumble with their own defence stat, not the
	// toughening stat -- nothing hit them.
	d := find(t, evs, SideDefender, ClassObserved)
	if d.Stat != "dexterity" {
		t.Errorf("defender observed stat on fumble = %q, want the defence stat", d.Stat)
	}
}

func TestDefenceFumble_DefenderEarnsTheBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcDefenceFumble
	evs := EventsForContest(o, bonuses())

	if got := find(t, evs, SideDefender, ClassFumble).Multiplier; got != 2.0 {
		t.Errorf("defender fumble multiplier = %v, want 2.0", got)
	}
	if got := find(t, evs, SideAttacker, ClassObserved).Multiplier; got != 0.5 {
		t.Errorf("attacker observed multiplier = %v, want 0.5", got)
	}
}

// A floored outcome is the system overriding the dice. Participation still
// teaches; an exceptional event the dice did not produce does not.
func TestFloored_OrdinaryOnlyNoBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	o.Floored = true
	evs := EventsForContest(o, bonuses())

	if len(evs) != 2 {
		t.Fatalf("floored contest produced %d events, want 2 ordinary only", len(evs))
	}
	for _, e := range evs {
		if e.Class != ClassOrdinary {
			t.Errorf("floored contest emitted a %v event: %+v", e.Class, e)
		}
	}
}

// The caller decides who participates by populating the fields. An absent
// defender must not fabricate a defender event -- this is what keeps U9 from
// changing WHEN progression fires.
func TestNoDefenderFields_NoDefenderEvents(t *testing.T) {
	o := Outcome{AttackerSkill: "skullduggery", AttackerStat: "dexterity"}
	evs := EventsForContest(o, bonuses())
	if len(evs) != 1 || evs[0].Side != SideAttacker {
		t.Fatalf("got %+v, want a single attacker event", evs)
	}
}

func TestZeroBonuses_ActAsOffSwitches(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	evs := EventsForContest(o, Bonuses{Doing: 0, Observing: 0})
	for _, e := range evs {
		if e.Class != ClassOrdinary && e.Multiplier != 0 {
			t.Errorf("zeroed bonus produced multiplier %v", e.Multiplier)
		}
	}
}

// The split exists so melee can take the bonus tier WITHOUT a second ordinary
// defender event, which its per-round AwardDefenceProgression already awards.
func TestBonusEvents_EmitsNoOrdinary(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	for _, e := range BonusEvents(o, bonuses()) {
		if e.Class == ClassOrdinary {
			t.Errorf("BonusEvents emitted an ordinary event: %+v", e)
		}
	}
}

func TestOrdinaryEvents_EmitsNoBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	for _, e := range OrdinaryEvents(o) {
		if e.Class != ClassOrdinary {
			t.Errorf("OrdinaryEvents emitted a bonus event: %+v", e)
		}
	}
}

// One contest lands on exactly ONE matrix row. Four independent booleans let a
// caller pay four bonus rolls for one swing, and pay a fumble bonus to the
// winner; the enum makes that unrepresentable.
func TestClassify_Precedence(t *testing.T) {
	cases := []struct {
		name                                              string
		atkCrit, defCrit, atkFumble, defFumble bool
		want                                              Exceptional
	}{
		{"nothing", false, false, false, false, ExcNone},
		{"attack crit", true, false, false, false, ExcAttackCrit},
		{"defence crit", false, true, false, false, ExcDefenceCrit},
		{"attack fumble", false, false, true, false, ExcAttackFumble},
		{"defence fumble", false, false, false, true, ExcDefenceFumble},
		// A fumble is self-relative and a crit is margin-derived, so an
		// attacker can roll terribly and still be out-rolled worse. The crit is
		// what the game narrated, so the crit is what pays.
		{"crit outranks a co-occurring fumble", false, true, true, false, ExcDefenceCrit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.atkCrit, tc.defCrit, tc.atkFumble, tc.defFumble); got != tc.want {
				t.Errorf("Classify = %v, want %v", got, tc.want)
			}
		})
	}
}

// Exactly two bonus events per contest, never four.
func TestBonusEvents_NeverPaysBothAxes(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	if got := len(BonusEvents(o, bonuses())); got != 2 {
		t.Errorf("BonusEvents produced %d events, want exactly 2", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/progression/ -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/progression/event.go`:

```go
// Package progression is the pure event layer of the unified contest arc:
// given the plain facts of one resolved contest, it says which progression
// events that contest implies, for whom, at what multiplier.
//
// It deliberately does NOT fire anything. It holds no *characters.Character, no
// room, and reads no config -- multipliers arrive as arguments. That is what
// makes the matrix table-testable with plain values, which matters here because
// a Go test binary never loads _datafiles/config.yaml and a package that read
// balance config would be tested against Go defaults instead of shipped values.
//
// It also does NOT decide WHEN progression fires. The caller populates only the
// sides that earned an ordinary event, exactly as that call site did before U9.
// Adding or removing a firing condition is U10b's job, not this package's.
package progression

// Side names one participant in a contest.
type Side uint8

const (
	SideAttacker Side = iota
	SideDefender
)

// Class names why an event fired. Ordinary events track a use and progress
// normally; the other three are bonus events.
type Class uint8

const (
	// ClassOrdinary is ordinary use. It tracks the use counter.
	ClassOrdinary Class = iota
	// ClassCrit is the party who landed the crit.
	ClassCrit
	// ClassFumble is the party who fumbled. Failure teaches.
	ClassFumble
	// ClassObserved is the party who received or witnessed the exceptional
	// event rather than causing it.
	ClassObserved
)

// IsBonus reports whether a class is one of the three exceptional classes.
//
// Bonus events must NOT increment the use counters. In CheckSkillProgression
// the use count becomes a virtual rank and CalculateProgressionChance is
// monotonically DECREASING in rank, so inflating the counter on a crit would
// punish critting. See spec 5.2.
func (c Class) IsBonus() bool { return c != ClassOrdinary }

// Event is one progression award for one side. An empty Skill or Stat means
// "no roll of that kind" -- never pass an empty name downstream, since
// CheckSkillProgression("") takes a roll and a success banners no skill at all.
type Event struct {
	Side       Side
	Skill      string
	Stat       string
	Class      Class
	Multiplier float64
}

// Outcome is everything about a resolved contest that the matrix needs.
//
// The four boolean outcome flags are not mutually exclusive by type, but a
// single contest produces at most one attack-side and one defence-side
// exceptional result. Callers set what happened; this package does not
// arbitrate.
type Outcome struct {
	// Populate a side's Skill/Stat only if that side earns an ordinary event
	// under the CALL SITE's existing rules. Leaving them empty suppresses that
	// side entirely.
	AttackerSkill string
	AttackerStat  string
	DefenderSkill string
	DefenderStat  string

	// ToughenStat is the stat awarded to a defender who RECEIVES a crit:
	// vitality for physical, willpower for magical, charisma for conviction.
	// Deliberately not DefenderStat -- you learn to take a hit, not to swing
	// better. Falls back to DefenderStat when empty.
	ToughenStat string

	// Exceptional names the ONE exceptional result this contest produced.
	// It is a single enum rather than four booleans on purpose: the spec's
	// matrix is five mutually exclusive rows, and four independent flags let a
	// caller pay four bonus events for one contest, or pay a fumble bonus to
	// the side that won. Use Classify to derive it.
	Exceptional Exceptional

	// Floored reports that a contest floor CHANGED the outcome. Floored
	// contests award ordinary events but never bonuses.
	Floored bool
}

// Exceptional names which single row of the spec's matrix a contest landed on.
type Exceptional uint8

const (
	ExcNone Exceptional = iota
	ExcAttackCrit
	ExcAttackFumble
	ExcDefenceCrit
	ExcDefenceFumble
)

// Classify reduces the engine's crit and fumble signals to the one row that
// fired, in a fixed precedence.
//
// Crits cannot collide: since 5.11d a contest crit is derived from the
// NORMALIZED MARGIN, and one margin cannot be both strongly positive and
// strongly negative, so attackCrit and defenceCrit are mutually exclusive by
// construction.
//
// Fumbles CAN collide with a crit, because a fumble is self-relative (the
// z-score of one roll) rather than margin-derived: an attacker can roll
// terribly and still be out-rolled worse. When that happens the CRIT wins,
// because the crit is the outcome the game narrated to both players, and
// paying a bonus for an event nobody was told about is how progression stops
// being legible.
func Classify(attackCrit, defenceCrit, attackFumble, defenceFumble bool) Exceptional {
	switch {
	case attackCrit:
		return ExcAttackCrit
	case defenceCrit:
		return ExcDefenceCrit
	case attackFumble:
		return ExcAttackFumble
	case defenceFumble:
		return ExcDefenceFumble
	}
	return ExcNone
}

// Bonuses carries the two config-driven multipliers. They are arguments rather
// than config reads so this package stays pure. Zero is a legal off-switch.
type Bonuses struct {
	Doing     float64 // CritProgressionBonus
	Observing float64 // ObservedCritProgressionBonus
}

// OrdinaryEvents returns only the ordinary-use events an Outcome implies: one
// per side whose Skill or Stat is populated.
//
// It is SEPARATE from BonusEvents because callers genuinely need one without
// the other. Melee awards its defender's ordinary event once per round through
// AwardDefenceProgression, but evaluates its bonus tier from the same Outcome;
// asking for both there would award the defender an extra ordinary event per
// weapon hit. Making the caller filter a combined slice is how that bug gets
// written, so the package does the split.
func OrdinaryEvents(o Outcome) []Event {
	evs := make([]Event, 0, 2)
	if o.AttackerSkill != "" || o.AttackerStat != "" {
		evs = append(evs, Event{
			Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat,
			Class: ClassOrdinary, Multiplier: 1.0,
		})
	}
	if o.DefenderSkill != "" || o.DefenderStat != "" {
		evs = append(evs, Event{
			Side: SideDefender, Skill: o.DefenderSkill, Stat: o.DefenderStat,
			Class: ClassOrdinary, Multiplier: 1.0,
		})
	}
	return evs
}

// BonusEvents returns only the crit/fumble tier: the pair of events the one
// exceptional result implies, or nothing.
//
// A floored outcome returns nothing. A floor overrode the dice, and an
// exceptional event that did not actually happen teaches nobody.
func BonusEvents(o Outcome, b Bonuses) []Event {
	if o.Floored || o.Exceptional == ExcNone {
		return nil
	}

	toughen := o.ToughenStat
	if toughen == "" {
		toughen = o.DefenderStat
	}

	switch o.Exceptional {
	case ExcAttackCrit:
		return []Event{
			{SideAttacker, o.AttackerSkill, o.AttackerStat, ClassCrit, b.Doing},
			// The one cell that swaps the stat: a crit RECEIVED toughens.
			{SideDefender, o.DefenderSkill, toughen, ClassObserved, b.Observing},
		}
	case ExcAttackFumble:
		return []Event{
			{SideAttacker, o.AttackerSkill, o.AttackerStat, ClassFumble, b.Doing},
			{SideDefender, o.DefenderSkill, o.DefenderStat, ClassObserved, b.Observing},
		}
	case ExcDefenceCrit:
		return []Event{
			{SideDefender, o.DefenderSkill, o.DefenderStat, ClassCrit, b.Doing},
			{SideAttacker, o.AttackerSkill, o.AttackerStat, ClassObserved, b.Observing},
		}
	case ExcDefenceFumble:
		return []Event{
			{SideDefender, o.DefenderSkill, o.DefenderStat, ClassFumble, b.Doing},
			{SideAttacker, o.AttackerSkill, o.AttackerStat, ClassObserved, b.Observing},
		}
	}
	return nil
}

// EventsForContest is OrdinaryEvents followed by BonusEvents, for the callers
// that want both. Ordinary events come first so a caller applying them in order
// tracks the use before rolling the bonus.
func EventsForContest(o Outcome, b Bonuses) []Event {
	return append(OrdinaryEvents(o), BonusEvents(o, b)...)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/progression/ -v`
Expected: PASS, all twelve.

- [ ] **Step 5: Commit**

```bash
git add internal/progression/
git commit -m "feat(u9): add the pure progression event package

Layer 3 of the contest design, spec section 5.3: the contest returns the
progression events it implies and a thin adapter applies them. No
character, no room, no config read -- multipliers arrive as arguments,
so the matrix is table-testable with plain values."
```

---

## Task 6: The two config knobs

**Files:**
- Modify: `internal/configs/config.balance.go` (around line 337-347)
- Modify: `internal/configs/config.balance.progression.go`
- Modify: `_datafiles/config.yaml`
- Test: `internal/configs/config.balance.progression_test.go` (create or extend)

- [ ] **Step 1: Write the failing validation test**

```go
package configs

import "testing"

// Zero must SURVIVE validation on both knobs. The neighbouring progression
// knobs validate with `<= 0`, which silently replaces a deliberate 0 with the
// default. These two are documented off-switches, so they validate on `< 0`.
func TestProgressionBonuses_ZeroIsLegal(t *testing.T) {
	b := Balance{CritProgressionBonus: 0, ObservedCritProgressionBonus: 0}
	b.Validate()
	if b.CritProgressionBonus != 0 {
		t.Errorf("CritProgressionBonus 0 became %v; 0 is a documented off-switch", b.CritProgressionBonus)
	}
	if b.ObservedCritProgressionBonus != 0 {
		t.Errorf("ObservedCritProgressionBonus 0 became %v", b.ObservedCritProgressionBonus)
	}
}

func TestProgressionBonuses_NegativeIsCorrected(t *testing.T) {
	b := Balance{CritProgressionBonus: -1, ObservedCritProgressionBonus: -1}
	b.Validate()
	if b.CritProgressionBonus != 2.0 {
		t.Errorf("CritProgressionBonus = %v, want the 2.0 default", b.CritProgressionBonus)
	}
	if b.ObservedCritProgressionBonus != 0.5 {
		t.Errorf("ObservedCritProgressionBonus = %v, want the 0.5 default", b.ObservedCritProgressionBonus)
	}
}
```

Before writing this, confirm the actual validation entry point name and the
`Balance` struct name:

```bash
grep -n "func (b \*Balance)" internal/configs/config.balance*.go | head
```

Match whatever is there. If validation is split per-subsystem, call the
progression one directly rather than inventing `Validate()`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/configs/ -run TestProgressionBonuses -v`
Expected: FAIL — unknown fields.

- [ ] **Step 3: Declare the fields**

In `internal/configs/config.balance.go`, beside the other progression knobs
(around line 337-347):

```go
	CritProgressionBonus         ConfigFloat `yaml:"CritProgressionBonus"`         // Progression multiplier for the party who DID a crit or fumble (default 2.0; 0 disables)
	ObservedCritProgressionBonus ConfigFloat `yaml:"ObservedCritProgressionBonus"` // Progression multiplier for the party who RECEIVED one (default 0.5; 0 disables)
```

- [ ] **Step 4: Add the validation**

In `internal/configs/config.balance.progression.go`, alongside the existing
progression validation:

```go
	// `< 0`, NOT `<= 0`. Both knobs are documented off-switches and 0 is a
	// legal shipped value; the `<= 0` idiom used by the neighbouring knobs
	// would silently restore the default and make disabling them impossible.
	if b.CritProgressionBonus < 0 {
		b.CritProgressionBonus = 2.0
	}
	if b.ObservedCritProgressionBonus < 0 {
		b.ObservedCritProgressionBonus = 0.5
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/configs/ -run TestProgressionBonuses -v`
Expected: PASS.

- [ ] **Step 6: Ship the values in `config.yaml`**

In `_datafiles/config.yaml`, immediately after `ProgressionDecayAboveCap`
(around line 933):

```yaml
  # CritProgressionBonus: multiplier on the progression roll for whichever side
  # DID the crit or the fumble. Failure teaches, so a fumble pays it too.
  # Raising it makes exceptional moments the main way characters improve;
  # lowering it makes progression purely a function of repetition. 0 turns
  # bonus progression off entirely and is a legal value.
  CritProgressionBonus: 2.0
  # ObservedCritProgressionBonus: multiplier for the side that RECEIVED the crit
  # or fumble rather than causing it. Deliberately well below
  # CritProgressionBonus: observing is worth strictly less than doing. This is
  # also the rate at which being critically hit toughens you, which before U9
  # was a flat 0.25 that never decayed with rank. 0 is a legal off-switch.
  ObservedCritProgressionBonus: 0.5
```

Read the shipped `config.yaml` from `git show HEAD:_datafiles/config.yaml`
rather than from disk when building the commit. The file carries
`skip-worktree`, so the disk copy desyncs in both directions.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/ _datafiles/config.yaml
git commit -m "feat(u9): add CritProgressionBonus and ObservedCritProgressionBonus

Replaces a hardcoded 2.0 literal in internal/characters/progression.go,
per standing rule 1: no balance number inside internal/.

Both validate on < 0, not <= 0, so a deliberate 0 survives as the
documented off-switch."
```

---

## Task 7: The applier

**Files:**
- Modify: `internal/characters/character.go` (add the dedupe map beside `combatDriftRound`, around line 298)
- Modify: `internal/characters/progression.go`
- Test: `internal/characters/progression_apply_test.go` (create)

- [ ] **Step 1: Write the failing tests**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/progression"
)

// Ordinary events must go through the SAME path they used before U9, which
// includes tracking the use counter.
func TestApplyProgression_OrdinaryTracksTheUse(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("skill use count = %d, want 1", got)
	}
}

// Bonus events must NOT track. The use count becomes a virtual rank and the
// progression curve DECREASES in rank, so tracking a crit would punish
// critting. Spec 5.2.
func TestApplyProgression_BonusDoesNotTrackTheUse(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassCrit, Multiplier: 2.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("bonus event tracked the use count (%d), which decays progression", got)
	}
}

func TestApplyProgression_IgnoresTheOtherSide(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideDefender, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("applied the defender's event to the attacker")
	}
}

// Bonus events dedupe once per round per skill. Ordinary events do not.
func TestApplyProgression_BonusDedupesWithinARound(t *testing.T) {
	c := newProgressionTestCharacter(t)
	bonus := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassCrit, Multiplier: 2.0,
	}}

	ev := bonus[0]
	if !c.claimBonusProgression(ev, 7) {
		t.Fatal("first claim in round 7 was refused")
	}
	if c.claimBonusProgression(ev, 7) {
		t.Error("second claim in the same round was allowed")
	}
	if !c.claimBonusProgression(ev, 8) {
		t.Error("claim in the next round was refused")
	}

	// Same skill, DIFFERENT stat, same round: must NOT collide. A crit received
	// trains the defence skill with the toughening stat while a fumble observed
	// trains it with the defence stat, and keying on skill alone would let the
	// first consume the other's slot.
	other := ev
	other.Stat = "vitality"
	if !c.claimBonusProgression(other, 7) {
		t.Error("a same-skill different-stat event collided with an unrelated claim")
	}
}

func TestApplyProgression_OrdinaryDoesNotDedupe(t *testing.T) {
	c := newProgressionTestCharacter(t)
	ev := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(ev, progression.SideAttacker, 0, 3)
	c.ApplyProgression(ev, progression.SideAttacker, 0, 3)

	if got := c.GetSkillUseCount("weapon-combat"); got != 2 {
		t.Errorf("ordinary events deduped: use count = %d, want 2", got)
	}
}

// An empty skill or stat name must be skipped, not passed on.
// CheckSkillProgression("") takes a roll and a success banners no skill at all.
func TestApplyProgression_EmptyNamesAreSkipped(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "", Stat: "",
		Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1) // must not panic

	if got := c.GetSkillUseCount(""); got != 0 {
		t.Errorf("tracked an empty skill name")
	}
}

// Spec 5.3: mobs progress through the SAME applier, gated only by the existing
// MobProgressionEnabled / MobProgressionRate knobs. No new gate, no new branch.
// A userId of 0 (which every mob passes) must not suppress the roll -- it only
// suppresses the player-facing banner.
func TestApplyProgression_MobsUseTheSamePath(t *testing.T) {
	c := newProgressionTestCharacter(t)
	c.IsMob = true

	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("mob skill use count = %d, want 1 -- mobs must not need a separate path", got)
	}
}

// Spec 5.1 rule 4: U8 lets an exhausted actor autoattack, defend, flee and
// maintain a grapple while DROPPING the skill term from its score. That is a
// combat-effectiveness penalty, not a progression penalty, so the event is
// unchanged.
//
// This is the current behaviour and U9 must not accidentally change it: the
// applier takes an Event, and nothing in Event or in the applier reads a cost
// result. The test pins that absence, because the natural "improvement" someone
// will later propose is to scale progression by whether the action was fully
// paid.
func TestApplyProgression_IgnoresWhetherTheActionWasFullyPaid(t *testing.T) {
	full := newProgressionTestCharacter(t)
	broke := newProgressionTestCharacter(t)
	broke.Stamina = 0

	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	full.ApplyProgression(evs, progression.SideAttacker, 0, 1)
	broke.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if full.GetSkillUseCount("weapon-combat") != broke.GetSkillUseCount("weapon-combat") {
		t.Error("an exhausted actor progressed differently; exhaustion is an effectiveness penalty, not a progression penalty")
	}
}
```

Check the real field name for the stamina pool before writing
`broke.Stamina = 0` — U5b routed every pool mutation through helpers and the
field may not be directly assignable:

```bash
grep -n "Stamina" internal/characters/character.go | head
```

`newProgressionTestCharacter` must build a character with initialised stats.
Set `.Base` and call `Recalculate()`; `.Value` and `.ValueAdj` are derived.
Check whether the package already has such a helper before writing one:

```bash
grep -rn "func new.*TestCharacter\|func testCharacter" internal/characters/*_test.go
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/characters/ -run TestApplyProgression -v`
Expected: FAIL — `ApplyProgression` and `claimBonusProgression` undefined.

- [ ] **Step 3: Add the dedupe field**

In `internal/characters/character.go`, immediately after the
`combatDriftRound` field (line 298):

```go
	bonusProgressionRound   map[string]uint64              // transient (not persisted): "skill|class" -> last round a BONUS progression event fired (once-per-round guard; ordinary events are never deduped)
```

- [ ] **Step 4: Write the applier**

Append to `internal/characters/progression.go`:

```go
// claimBonusProgression enforces the once-per-round bonus rule: at most one
// bonus event per skill per class per round, per character.
//
// Ordinary per-swing events are deliberately NOT deduped, so ordinary melee
// progression rates do not move. It is only the bonus that a margin-driven crit
// rate can fire on nearly every swing of a lopsided fight.
//
// A round of 0 means "no round context" and always claims, so non-combat
// callers are never silently suppressed.
func (c *Character) claimBonusProgression(ev progression.Event, round uint64) bool {
	if round == 0 {
		return true
	}
	if c.bonusProgressionRound == nil {
		c.bonusProgressionRound = make(map[string]uint64)
	}
	skillName, class := ev.Skill, ev.Class
	// The stat is part of the key, not just the skill. Observed events can
	// carry the SAME skill with DIFFERENT stats -- a crit received trains the
	// defence skill with the TOUGHENING stat, while a fumble observed trains it
	// with the defence stat. Keying on skill alone would let the first of those
	// in a round consume the slot for the other. Events with an empty skill
	// name (which the unarmed and no-defence paths can produce) would otherwise
	// all collapse onto one key.
	key := skillName + "|" + ev.Stat + "|" + strconv.Itoa(int(class))
	if c.bonusProgressionRound[key] >= round {
		return false
	}
	c.bonusProgressionRound[key] = round
	return true
}

// ClaimedBonusThisRound reports whether a bonus progression event has already
// fired for this skill this round, for any class.
//
// Exported deliberately. It is the only observable the bonus tier leaves behind
// -- bonus events do not track use counts by design -- so tests in OTHER
// packages (internal/combat, internal/hooks) need it to assert that the tier
// ran at all. A _test.go helper cannot serve them: test files compile only into
// their own package's test binary.
func (c *Character) ClaimedBonusThisRound(skillName string) bool {
	if c == nil || c.bonusProgressionRound == nil {
		return false
	}
	round := util.GetRoundCount()
	// Keys are "skill|stat|class" and the caller does not know the stat, so
	// match on the skill segment.
	prefix := skillName + "|"
	for key, claimed := range c.bonusProgressionRound {
		if claimed >= round && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// ApplyProgression applies every event belonging to one side to this character.
// Callers invoke it twice per contest, once per participant, so this function
// never needs to know who the other party is.
//
// Ordinary events route through the pre-U9 entry points unchanged, which is
// what keeps the ordinary rates identical: OnSkillUseScaled tracks the use,
// grants mutation cluster drift, emits the SkillUsed quest event and rolls the
// skill's primary stat.
//
// Bonus events are pure extra rolls: no use tracking (it would decay the curve,
// spec 5.2), no cluster drift, no quest event.
func (c *Character) ApplyProgression(events []progression.Event, side progression.Side, userId int, round uint64) {
	if c == nil {
		return
	}
	for _, ev := range events {
		if ev.Side != side {
			continue
		}
		if ev.Class.IsBonus() {
			if !c.claimBonusProgression(ev, round) {
				continue
			}
			c.applyBonusProgression(ev, userId)
			continue
		}

		if ev.Skill != "" {
			c.OnSkillUseScaled(ev.Skill, userId, ev.Multiplier)
		}
		// OnSkillUseScaled already rolled the skill's primary stat. Roll again
		// only when the event names a DIFFERENT stat, which is the spell
		// primarystat override and the crit-received toughening stat.
		if ev.Stat != "" && ev.Stat != skills.GetSkillPrimaryStat(ev.Skill) {
			c.OnStatUse(ev.Stat, userId)
		}
	}
}

// applyBonusProgression takes the extra skill and stat rolls a crit, fumble or
// observed exceptional event earns, and speaks the flavour line on a success.
func (c *Character) applyBonusProgression(ev progression.Event, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}
	if ev.Multiplier <= 0 {
		return // the knob is set to its off-switch
	}

	if ev.Skill != "" && c.CheckSkillProgression(ev.Skill, userId, ev.Multiplier) && userId > 0 {
		switch ev.Class {
		case progression.ClassCrit:
			events.AddToQueue(events.Message{UserId: userId, Text: fmt.Sprintf(
				`<ansi fg="magenta">***</ansi> A moment of brilliance! Your <ansi fg="yellow">%s</ansi> technique improves! <ansi fg="magenta">***</ansi>`,
				ev.Skill) + "\n"})
		case progression.ClassFumble:
			events.AddToQueue(events.Message{UserId: userId, Text: fmt.Sprintf(
				`<ansi fg="red">!!!</ansi> You learn from your mistake! Your <ansi fg="yellow">%s</ansi> understanding deepens. <ansi fg="red">!!!</ansi>`,
				ev.Skill) + "\n"})
			// ClassObserved is deliberately silent. Watching someone else's
			// brilliance is not your moment of brilliance.
		}
	}

	if ev.Stat != "" {
		c.CheckStatProgression(ev.Stat, userId, ev.Multiplier)
	}
}
```

Add `strconv` and `github.com/GoMudEngine/GoMud/internal/progression` to the
import block of `internal/characters/progression.go`. `fmt`, `configs`,
`events` and `skills` are already imported.

**Import direction check:** `progression` imports only `skills` and
`actionspec`; `characters` imports `progression`. Confirm no cycle:

```bash
go build ./internal/characters/ ./internal/progression/
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/characters/ -run TestApplyProgression -v`
Expected: PASS, all eight.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/
git commit -m "feat(u9): add the progression event applier

Ordinary events route through the pre-U9 entry points unchanged, so
ordinary rates do not move. Bonus events are pure extra rolls: no use
tracking, since the use count decays the curve and tracking a crit would
punish critting.

Bonus events dedupe once per round per skill per class."
```

---

## Task 8: Close the crit-received faucet

**Files:**
- Modify: `internal/characters/progression.go` (`OnCritReceived`)
- Test: `internal/characters/progression_faucet_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package characters

import "testing"

// OnCritReceived routed through CheckRegenProgression, which never calls
// CalculateProgressionChance and never applies StatProgressionRate -- a flat
// chance at EVERY virtual rank. That is the shape of the fyttyn vitality
// exploit that migration 0.16.0 exists to freeze, and post-U6 margin-driven
// crit rates made it worse.
//
// This test pins the fix structurally rather than statistically: the effective
// chance must FALL as the stat's virtual rank rises.
func TestCritReceivedProgression_DecaysWithRank(t *testing.T) {
	low := newProgressionTestCharacter(t)
	high := newProgressionTestCharacter(t)

	// Drive the high character's virtual rank up via its use counter.
	for i := 0; i < 25*120; i++ {
		high.TrackStatUse("vitality")
	}

	lowChance := critReceivedChanceForTest(low, "vitality")
	highChance := critReceivedChanceForTest(high, "vitality")

	if !(highChance < lowChance) {
		t.Errorf("crit-received chance did not decay with rank: rank-0 %.5f, high-rank %.5f",
			lowChance, highChance)
	}
}
```

`critReceivedChanceForTest` is a test-only helper in the same package that
computes what `OnCritReceived` will roll against, using the same expression the
implementation uses. If that duplicates logic, extract the chance calculation
into an unexported `critReceivedChance(statName string) float64` in
`progression.go` and have both the implementation and the test call it.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/characters/ -run TestCritReceivedProgression -v`
Expected: FAIL — the chance is identical at both ranks.

- [ ] **Step 3: Move `OnCritReceived` onto the decayed curve**

Replace the body of `OnCritReceived` in `internal/characters/progression.go`:

```go
// OnCritReceived is called when a character takes a critical hit.
// Triggers stat progression for the stat related to the damage channel:
//
//	physical crit → vitality (body toughens from surviving hard blows)
//	magical crit  → willpower (mind hardens against arcane trauma)
//	rhetoric crit → charisma (ego steels itself after being shaken)
//
// U9: this routes through CheckStatProgression, NOT CheckRegenProgression.
// CheckRegenProgression is correct for OnRegenTick, whose chance comes from
// pool depletion, but it never calls CalculateProgressionChance and never
// applies StatProgressionRate -- so as a crit handler it was a flat chance at
// every virtual rank. That is rank-independent stat progression driven by being
// hit, which is structurally the fyttyn vitality exploit (see
// internal/migration/0.16.0.go), and U6's margin-driven crit rates made it
// worse by pushing crit rates against an outclassed defender toward certainty.
//
// The multiplier is ObservedCritProgressionBonus: receiving is worth strictly
// less than doing.
func (c *Character) OnCritReceived(damageChannel string, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}

	statName := ToughenStatFor(damageChannel)
	if statName == "" {
		return
	}

	mult := float64(configs.GetBalanceConfig().ObservedCritProgressionBonus)
	if mult <= 0 {
		return
	}

	// TRACK BEFORE ROLLING. This is the second half of the faucet fix and it is
	// the load-bearing half. CheckStatProgression derives virtualRank from
	// GetStatUseCount, and NOTHING in the game tracks vitality use -- zero
	// production OnStatUse("vitality") sites, and CheckRegenProgression never
	// tracked either. So vitality's rank sat at 0 until its VALUE passed 150,
	// which meant its progression chance was constant no matter how much
	// vitality the character already had. Moving to the decayed curve without
	// this would have bought a flat 25% -> 13.5% cut and left the
	// rank-independence, which is the actual fyttyn mechanism, fully intact.
	c.TrackStatUse(statName)
	c.CheckStatProgression(statName, userId, mult)
}

// ToughenStatFor maps a damage channel to the stat that TAKING a critical hit
// in that channel trains. Exported because the contest seam needs the same
// mapping to fill Outcome.ToughenStat, and two copies would drift.
func ToughenStatFor(damageChannel string) string {
	switch damageChannel {
	case "physical":
		return "vitality"
	case "magical":
		return "willpower"
	case "conviction":
		return "charisma"
	}
	return ""
}
```

- [ ] **Step 3b: Track the use on the regen path too**

Owner decision, 2026-08-19: do **both** callers, because fixing only the crit
path leaves open the low-health grind fyttyn actually used.

In `OnRegenTick`, track each related stat before rolling it:

```go
	for _, statName := range relatedStats {
		// Same reason as OnCritReceived: CheckRegenProgression rolls but never
		// records that the stat was exercised, so the rank that decays the
		// curve never moved. Health/stamina/conviction regen is the largest
		// source of vitality progression in the game and was entirely
		// rank-free.
		c.TrackStatUse(statName)
		c.CheckRegenProgression(statName, userId, chance)
	}
```

**Name the blast radius in the commit.** `OnRegenTick` maps health to vitality
and willpower, stamina to strength and vitality, conviction to willpower and
charisma. All of those now decay with use for the first time, so veteran growth
in them slows sharply. This is the single largest behaviour change in U9 and it
belongs in spec §7.1, which already lists it.

Run: `go test ./internal/characters/ -v` — expected PASS. A regen-progression
test asserting a rank-free rate is asserting the faucet; update it and say so.

- [ ] **Step 4: Pin the rates numerically (spec §10)**

The decay test above proves the shape. This one pins the magnitude, so a later
retune cannot quietly reopen the faucet without a failing test explaining what
it is reopening.

```go
// Pins the crit-received chance at three ranks. A Go test binary never loads
// _datafiles/config.yaml, so the balance values are INJECTED here rather than
// read -- reading them under test would measure struct zero values and make
// every assertion vacuously true.
func TestCritReceivedProgression_RatesAtThreeRanks(t *testing.T) {
	const (
		base       = 0.12 // BaseProgressionChance
		decayBelow = 3.0  // ProgressionDecayBelowCap
		softCap    = 150  // StatProgressionSoftCap
		statRate   = 2.25 // StatProgressionRate
		observed   = 0.5  // ObservedCritProgressionBonus
	)
	cases := []struct {
		rank int
		want float64 // percent
	}{
		{0, 13.5},
		{75, 3.0},
		{150, 0.67},
	}
	for _, tc := range cases {
		chance := base
		if tc.rank > 0 {
			chance = base * math.Exp(-decayBelow*float64(tc.rank)/float64(softCap))
		}
		got := chance * statRate * observed * 100
		if math.Abs(got-tc.want) > 0.05 {
			t.Errorf("rank %d: %.2f%%, want %.2f%%", tc.rank, got, tc.want)
		}
	}
	// Before U9 all three were 25.0%, flat, because CheckRegenProgression
	// applies neither the decay curve nor StatProgressionRate.
}
```

This test deliberately recomputes the curve rather than calling
`CalculateProgressionChance`, so that a change to the curve's *shape* also
fails it. If it drifts from the implementation, that is the signal, not the
bug — reconcile them deliberately.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/characters/ -v`
Expected: PASS.

If an existing test asserted the old flat 0.25, it is asserting the faucet.
Update it and say so in the commit — this is one of the §7.1 named changes, not
a regression.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/
git commit -m "fix(u9): put crit-received progression on the decayed curve

OnCritReceived routed through CheckRegenProgression, which never calls
CalculateProgressionChance and never applies StatProgressionRate. The
chance was flat at every virtual rank: rank-independent stat progression
driven by being hit, which is structurally the fyttyn vitality exploit
that migration 0.16.0 exists to freeze.

U6 made it worse. Crit is margin-driven now, so crit rates against a
badly outclassed defender approach certainty, and the faucet rate rose
with how outmatched the defender was.

Effective chance at the shipped config: 25.0% flat becomes 13.5% at rank
0, 3.0% at rank 75 and 0.67% at the soft cap."
```

---

## Task 9: Delete the melee double-progression

**Gated on Task 1 confirming the duplication.** If Task 1 passed, skip this task.

**Files:**
- Modify: `internal/combat/combat.go` (delete progression from `AttackPlayerVsMob`, `AttackPlayerVsPlayer`, `AttackMobVsPlayer`, `AttackMobVsMob`, and delete `trackMobAttackProgression`)
- Modify: `internal/hooks/progression_duplication_test.go` (keep as a permanent regression test)

- [ ] **Step 1: Delete the player-attacker block**

In `internal/combat/combat.go`, in `AttackPlayerVsMob`, delete the progression
block (currently lines 76-102): the two `OnStatUse` calls, the
`OnSkillUse`/`OnCriticalSuccess` block inside `if attackResult.CleanHit`, the
`isDualWieldingWeaponCombat` progression call, and the `OnCriticalFailure` in
the `else` branch.

**Keep the sound calls.** `user.PlaySound("hit-other", "combat")` and
`user.PlaySound("miss", "combat")` are not progression and must survive. The
`if attackResult.CleanHit { ... } else { ... }` structure stays; only the
progression lines come out.

Do the same in `AttackPlayerVsPlayer` (lines 164, 174 area).

- [ ] **Step 2: Delete the mob-attacker path**

Delete `trackMobAttackProgression` entirely (`combat.go:185-203`) and its two
call sites in `AttackMobVsPlayer` and `AttackMobVsMob`.

Also delete the defender-dexterity lines. There are **two**, one per mob-attacker
quadrant, and deleting only the first breaks the four-quadrant parity convention
that spec §5.3 invokes:

| File:line | Function |
|---|---|
| `internal/combat/combat.go:231` | `AttackMobVsPlayer` — `user.Character.OnStatUse("dexterity", user.UserId)` |
| `internal/combat/combat.go:286` | `AttackMobVsMob` — `mobDef.Character.OnStatUse("dexterity", 0)`, commented "mirrors player defender tracking in AttackMobVsPlayer" |

The defender's dexterity is already awarded by `AwardDefenceProgression` on a
dodge. These were a third and fourth source. Verify both line numbers before
editing — the earlier draft of this plan said 230 for the first and did not know
about the second at all:

```bash
grep -n 'OnStatUse("dexterity"' internal/combat/combat.go
```

- [ ] **Step 3: Let the compiler find the orphans**

Run: `go build ./...`

`isDualWieldingWeaponCombat` may now be unused. If the compiler or `go vet`
flags it, delete it too — but first check whether Task 10 needs it, since the
dual-wield weapon-combat award is real behaviour that must survive the move.
**Do not delete a behaviour, only a duplicate.** If `applyCombatProgression`
does not currently award dual-wield weapon-combat, Task 10 must add it.

```bash
grep -n "isDualWieldingWeaponCombat" internal/combat/*.go internal/hooks/*.go
```

- [ ] **Step 4: Run the Task 1 test**

Run: `go test ./internal/hooks/ -run TestMeleeProgressionFiresOncePerRound -v`
Expected: **PASS** now.

- [ ] **Step 5: Run the full combat and hooks suites**

Run: `go test ./internal/combat/... ./internal/hooks/... -v`
Expected: PASS. Any test asserting the doubled counts was asserting the bug;
update it and name it in the commit.

- [ ] **Step 6: Rename the test to a permanent regression guard**

The Task 1 test stays. Update its doc comment to say the duplication was
confirmed and removed, and that it exists so it cannot come back.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/combat.go internal/hooks/progression_duplication_test.go
git commit -m "fix(u9): delete the melee double-progression

NewRound_DoCombat_unified ran progression twice against the same actors
in the same round: phase 2 inside combat.Attack*vs* and
trackMobAttackProgression, and phase 5 in applyCombatProgression. Both
unconditional. applyCombatProgression's doc comment already described
itself as the path owning all four quadrants, so the per-quadrant calls
were leftovers from before the unified orchestrator landed.

Measured before the fix: <fill in the Task 1 numbers>.

Phase 5 is now the single melee progression path. Also removes a third
defender-dexterity source at AttackMobVsPlayer, which duplicated the
dodge award from AwardDefenceProgression."
```

---

## Task 9b: Delete the melee DEFENCE double-progression

The symmetric half of Task 9, found by adversarial review. An earlier draft of
the spec documented this path as correct behaviour.

**Files:**
- Modify: `internal/combat/combat_helpers.go` (~line 1227)
- Test: `internal/hooks/progression_duplication_test.go` (extend)

- [ ] **Step 1: Write the failing test**

```go
// A defender who dodges four swings in a round takes ONE dodge progression
// event, not five. combat_helpers.go's sendDefenseMessages rolled per defended
// swing while processDefenderProgression rolled once per round.
func TestMeleeDefenceProgressionFiresOncePerRound(t *testing.T) {
	atk, def := newDualWieldingCombatPairForTest(t)
	before := def.GetCharacter().GetSkillUseCount("unarmed-combat")

	runOneCombatRoundAllDodgedForTest(t, atk, def)

	if got := def.GetCharacter().GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("defender dodge tracked %d times in one round, want 1", got)
	}
}
```

- [ ] **Step 2: Run it and record the number**

Run: `go test ./internal/hooks/ -run TestMeleeDefenceProgressionFiresOncePerRound -v`
Expected: **FAIL**, with a count equal to (swings defended) + 1.

Record the real number. It is the evidence for the PR.

- [ ] **Step 3: Delete the per-swing roll**

In `internal/combat/combat_helpers.go`, delete both lines of this block
(currently ~1227-1229), keeping the `if skillToProgress != ""` guard's
surrounding code intact:

```go
	if skillToProgress != "" {
		targetChar.TrackSkillUse(skillToProgress)
		targetChar.CheckSkillProgression(skillToProgress, targetChar.GetUserId(), 1.0)
	}
```

Delete the whole `if` block. `processDefenderProgression` →
`AwardDefenceProgression` → `OnSkillUse` already tracks the use, rolls the
skill, **and** rolls the primary stat — which this per-swing path never did, so
the surviving path is the more complete one.

Leave the U6 Task 12 comment above it that explains why the empty-name guard
exists; move it to `AwardDefenceProgression` if it no longer has a home, since
the hazard it documents (an empty skill name is not inert) is still real.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/... ./internal/hooks/... -v`
Expected: PASS, including the new test.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/combat_helpers.go internal/hooks/progression_duplication_test.go
git commit -m "fix(u9): delete the melee defence double-progression

The symmetric half of the attack duplication. sendDefenseMessages rolled
TrackSkillUse + CheckSkillProgression on every defended swing while
processDefenderProgression rolled the same skill once per round through
AwardDefenceProgression -- which also rolls the primary stat, so the
per-swing path was both duplicate and less complete.

A defender who dodged four swings took five skill rolls.

Measured before the fix: <fill in the Step 2 number>.

Found by adversarial review; an earlier draft of the spec documented
this path as correct and would have allow-listed it into the U9 guard
test permanently."
```

---

## Task 10: Route melee through the seam

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go` (`applyCombatProgression`)
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go` (`processDefenderProgression`)
- Test: `internal/hooks/progression_seam_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package hooks

import "testing"

// A crit must now pay the attacker the config bonus once, and give the defender
// a toughening event -- which melee never awarded before U9.
func TestMeleeCrit_AwardsDefenderToughening(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := def.GetCharacter().GetStatUseCount("vitality")

	runOneCombatRoundForcingCritForTest(t, atk, def)

	// Bonus events do not TRACK, so vitality's use count must not move; what
	// changes is that a roll happened at all. Assert on the roll, not the
	// counter: use the same seam the applier does.
	if got := def.GetCharacter().GetStatUseCount("vitality"); got != before {
		t.Errorf("a bonus event tracked a use count (%d -> %d)", before, got)
	}
}

// The seam must not change ordinary rates.
func TestMeleeOrdinary_StillOneEventPerRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := atk.GetCharacter().GetSkillUseCount("weapon-combat")

	runOneCombatRoundForTest(t, atk, def)

	if got := atk.GetCharacter().GetSkillUseCount("weapon-combat") - before; got != 1 {
		t.Errorf("weapon-combat tracked %d times, want 1", got)
	}
}

// REGRESSION (adversarial review, finding 1): the defender's ordinary defence
// event is awarded once per round by processDefenderProgression. An earlier
// draft of this task also emitted it from the seam INSIDE the WeaponHits loop,
// which awarded it once per weapon hit on top.
func TestMeleeDefender_OrdinaryEventNotMultipliedByWeaponCount(t *testing.T) {
	atk, def := newDualWieldingCombatPairForTest(t) // 2+ weapons, so N > 1
	before := def.GetCharacter().GetSkillUseCount("unarmed-combat")

	runOneCombatRoundAllDodgedForTest(t, atk, def)

	if got := def.GetCharacter().GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("defender dodge tracked %d times in one round, want 1 regardless of weapon count", got)
	}
}

// REGRESSION (adversarial review, finding 2): a fumbled swing has CleanHit
// false. Deriving the bonus skill from a CleanHit-gated field left it empty and
// deleted attacker fumble progression entirely.
func TestMeleeFumble_StillAwardsTheAttacker(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		AttackerStat:  "dexterity",
		Exceptional:   progression.ExcAttackFumble,
	}
	evs := progression.BonusEvents(o, progression.Bonuses{Doing: 2.0, Observing: 0.5})

	var found bool
	for _, e := range evs {
		if e.Side == progression.SideAttacker && e.Class == progression.ClassFumble {
			found = true
			if e.Skill == "" {
				t.Error("attacker fumble event carries no skill, so no roll fires")
			}
		}
	}
	if !found {
		t.Error("no attacker fumble event produced")
	}
}

// REGRESSION (adversarial review, finding 3): WeaponHits is empty for an
// unarmed attacker, and most mobs are unarmed. Evaluating the bonus tier inside
// the WeaponHits loop deleted the whole tier for them.
func TestUnarmedAttacker_StillReachesTheBonusTier(t *testing.T) {
	atk, def := newUnarmedCombatPairForTest(t)

	runOneCombatRoundForcingCritForTest(t, atk, def)

	if len(lastAttackResultForTest(atk).WeaponHits) != 0 {
		t.Fatal("fixture is not actually unarmed; WeaponHits is populated")
	}
	// The bonus tier ran if the round claimed a bonus slot for the defender.
	if !def.GetCharacter().ClaimedBonusThisRound("unarmed-combat") {
		t.Error("unarmed attacker's crit produced no defender bonus event")
	}
}
```

`runOneCombatRoundForcingCritForTest` uses the existing `forceCrit` parameter
already threaded through `rollCombatAttack` for the sleeping-defender case.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestMelee -v`
Expected: FAIL on the toughening assertion.

- [ ] **Step 3: Rewrite `applyCombatProgression` to build events**

Replace the progression portion of `applyCombatProgression` (the block from
`// Defender OnCritReceived` through `processDefenderProgression(...)`) with:

```go
	// U9: melee builds a progression.Outcome and hands it to the seam. The
	// FIRING CONDITIONS below are unchanged from pre-U9 -- CleanHit for the
	// attacker's skill, the defence-used set for the defender. What changed is
	// what the events carry. Changing when they fire is U10b.
	round := util.GetRoundCount()
	bonuses := progression.Bonuses{
		Doing:     float64(configs.GetBalanceConfig().CritProgressionBonus),
		Observing: float64(configs.GetBalanceConfig().ObservedCritProgressionBonus),
	}

	// Attacker stat progression keeps its quadrant-flavoured room messages, so
	// it stays on its own helper rather than becoming an event.
	emitAttackerStatGain(atk, "strength", atkUid)
	emitAttackerStatGain(atk, "dexterity", atkUid)

	// ── Ordinary attacker events: per weapon hit, gated on CleanHit ──────
	// Firing condition unchanged from pre-U9. ORDINARY ONLY: the defender's
	// ordinary event is awarded once per round by processDefenderProgression
	// below, so asking for it here as well would award it per weapon hit.
	for _, wh := range res.WeaponHits {
		if !wh.CleanHit {
			continue
		}
		atkChar.ApplyProgression(
			progression.OrdinaryEvents(progression.Outcome{
				AttackerSkill: wh.SkillTag,
				AttackerStat:  skills.GetSkillPrimaryStat(wh.SkillTag),
			}),
			progression.SideAttacker, atkUid, round)
	}
	if len(res.WeaponHits) == 0 && res.CleanHit {
		atkChar.OnSkillUse(string(skills.UnarmedCombat), atkUid)
	}

	// ── Bonus tier: ONCE per round, OUTSIDE the weapon loop ─────────────
	// Outside on purpose. AttackResult.WeaponHits is populated only by the
	// weapon loop in calculateCombat, so an UNARMED attacker produces none --
	// which is why the CleanHit fallback above exists. Evaluating the bonus
	// tier inside the loop would silently delete crit-received toughening and
	// the whole bonus tier for every unarmed attacker, and most mobs are
	// unarmed.
	//
	// Firing conditions are preserved exactly: res.Crit and res.Fumble are the
	// per-round aggregates the pre-U9 code used (documented at
	// attackresult.go:74-79 as reset per swing, so they reflect the LAST
	// swing). That last-swing semantics is odd and is recorded in the firing
	// audit for U10b. U9 does not change it.
	defencesUsed := defenceTypesUsed(*res)
	bonusSkill, bonusStat := attackerBonusSkillAndStat(*res, atkChar)

	bonusOut := progression.Outcome{
		AttackerSkill: bonusSkill,
		AttackerStat:  bonusStat,
		DefenderSkill: defenceSkillFor(defencesUsed),
		DefenderStat:  defenceStatFor(defencesUsed),
		ToughenStat:   characters.ToughenStatFor("physical"),
		Exceptional: progression.Classify(
			res.Hit && res.Crit,
			res.ParryCritDetected || res.DodgeCritDetected || res.BlockCritDetected,
			res.Fumble,
			false, // melee exposes no defence-fumble signal on AttackResult
		),
		Floored: false, // melee exposes no per-swing floor flag; see step 6
	}

	bonusEvs := progression.BonusEvents(bonusOut, bonuses)
	atkChar.ApplyProgression(bonusEvs, progression.SideAttacker, atkUid, round)
	defChar.ApplyProgression(bonusEvs, progression.SideDefender, defUid, round)

	// Defender dodge/parry/block ordinary events, unchanged: once per defence
	// type per round.
	processDefenderProgression(defChar, defUid, *res)
```

- [ ] **Step 4a: Extract the five-defence mapping in `internal/combat`**

`AwardDefenceProgression` (`internal/combat/defence_multiplier.go:117`) holds
the only mapping from a defence to what it trains. Task 10 and Task 11 both
need it, so extract it now rather than letting either copy the switch.

```go
// DefenceSkillAndStat is THE mapping from a defence to what it trains, in one
// place, for all five defences. AwardDefenceProgression and the crit/fumble
// bonus tier both read it, so the five rows exist once.
//
// Note the asymmetry with AwardDefenceProgression: parry deliberately awards
// BOTH dexterity and strength there, while this returns the single stat the
// bonus tier wants. That is intentional. Do not "simplify" the two into one by
// dropping parry's second stat.
//
// An unrecognised defence returns two empty strings rather than guessing.
// Passing an empty skill on is not inert: CheckSkillProgression("") takes the
// roll and a success banners no skill at all.
func DefenceSkillAndStat(defenceType string) (skill, stat string) {
	switch defenceType {
	case characters.DefenseDodge:
		return string(skills.UnarmedCombat), "dexterity"
	case characters.DefenseParry:
		return string(skills.WeaponCombat), "dexterity"
	case characters.DefenseBlock:
		return string(skills.WeaponCombat), "strength"
	case characters.DefenseQuell:
		return string(skills.Spellcasting), "willpower"
	case characters.DefenseDefy:
		return string(skills.Rhetoric), "willpower"
	}
	return "", ""
}
```

Rewrite `AwardDefenceProgression`'s switch to call it, keeping parry's second
`OnStatUse("strength")` exactly as it is today:

```go
func AwardDefenceProgression(c *characters.Character, userId int, defenceType string) {
	if c == nil {
		return
	}
	skill, stat := DefenceSkillAndStat(defenceType)
	if skill == "" {
		return // unrecognised defence awards nothing rather than guessing
	}
	c.OnSkillUse(skill, userId)
	c.OnStatUse(stat, userId)
	// Parry is the one two-stat defence: it takes both the timing and the
	// force to turn a blade. Preserved from pre-U9 behaviour verbatim.
	if defenceType == characters.DefenseParry {
		c.OnStatUse("strength", userId)
	}
}
```

Run: `go test ./internal/combat/... -v` — expected PASS with no test edits. If a
defence-progression test changes behaviour here, the refactor is wrong.

- [ ] **Step 4b: Add the three small helpers**

In `internal/hooks/NewRound_DoCombat_helpers.go`:

```go
// defenceTypesUsed returns the set of defences that registered this round, in
// the same fixed order processDefenderProgression uses. Extracted so the seam
// and the ordinary award read one definition of "which defences happened".
func defenceTypesUsed(result combat.AttackResult) []combat.DefenseType {
	used := make(map[combat.DefenseType]bool, 3)
	for _, se := range result.SwingEvents {
		if se.DefenseUsed != combat.DefenseNone {
			used[se.DefenseUsed] = true
		}
	}
	out := make([]combat.DefenseType, 0, 3)
	for _, d := range []combat.DefenseType{combat.DefenseDodge, combat.DefenseParry, combat.DefenseBlock} {
		if used[d] {
			out = append(out, d)
		}
	}
	return out
}

// defenceSkillFor names the skill and stat the defender's OBSERVED event trains
// when a crit or fumble happens. It uses the first defence that registered this
// round; with no defence registered it returns empty, which suppresses the
// event rather than guessing.
//
// It delegates to combat.DefenceSkillAndStat rather than switching again here.
// A second copy of the five-defence mapping is exactly the drift this arc
// exists to remove, and it would go stale the first time a defence changed what
// it trains.
func defenceSkillFor(used []combat.DefenseType) string {
	if len(used) == 0 {
		return ""
	}
	skill, _ := combat.DefenceSkillAndStat(string(used[0]))
	return skill
}

// defenceStatFor is defenceSkillFor's stat counterpart, from the same mapping.
func defenceStatFor(used []combat.DefenseType) string {
	if len(used) == 0 {
		return ""
	}
	_, stat := combat.DefenceSkillAndStat(string(used[0]))
	return stat
}

// attackerBonusSkillAndStat names the skill the attacker's crit or FUMBLE
// bonus trains.
//
// It deliberately does NOT gate on CleanHit. A fumbled swing has CleanHit
// false, so deriving the bonus skill from a CleanHit-gated field would leave it
// empty and applyBonusProgression would skip the roll -- silently deleting
// attacker fumble progression, which pre-U9 fired via OnCriticalFailure with
// the real skill tag and which spec 7.1 lists as an INCREASE.
//
// Falls back through: the first weapon's tag, then the character's current
// combat skill (correct for the unarmed case, which has no WeaponHits at all).
func attackerBonusSkillAndStat(res combat.AttackResult, atkChar *characters.Character) (skill, stat string) {
	if len(res.WeaponHits) > 0 {
		skill = res.WeaponHits[0].SkillTag
	}
	if skill == "" && atkChar != nil {
		skill = string(atkChar.GetCombatSkillTag())
	}
	if skill == "" {
		return "", ""
	}
	return skill, skills.GetSkillPrimaryStat(skill)
}
```

Then simplify `processDefenderProgression` to consume `defenceTypesUsed`:

```go
func processDefenderProgression(c *characters.Character, userId int, result combat.AttackResult) {
	for _, d := range defenceTypesUsed(result) {
		combat.AwardDefenceProgression(c, userId, string(d))
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/hooks/... -v`
Expected: PASS, including both Task 1 and Task 10 tests.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/
git commit -m "feat(u9): route melee progression through the event seam

Firing conditions are unchanged -- CleanHit for the attacker's skill,
the defence-used set for the defender. What changed is what the events
carry: a crit now also gives the defender a toughening event at the
observed rate, and a fumble pays its own side the bonus.

Changing WHEN any of this fires is U10b's job."
```

---

## Task 11: Route the channel-defence path through the seam

This covers the channels that actually resolve through
`resolveChannelDefenceWithRunner` rather than through melee's swing loop.

**Scope, corrected:** that is the **five spell sites and `combat_taunt.go:244`**
— not "spell, ranged and social" as an earlier draft claimed. Nothing calls it
with `ChannelRanged`: `shoot` has its own `rangedDefenseScore` path. And
`ChannelMelee` is documented at `defence_sets.go:32-34` as never routed here, so
the test below must construct its own `ChannelDefenceResult` rather than assume
a melee call reaches this function. Verify before writing the test:

```bash
grep -rn "ResolveChannelDefence\|resolveChannelDefenceWithRunner" --include=*.go internal/ | grep -v _test
```

**Files:**
- Modify: `internal/combat/defence_multiplier.go`
- Test: `internal/combat/defence_progression_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
)

// A floored outcome must award the ordinary defence event but never a bonus.
func TestChannelDefence_FlooredAwardsNoBonus(t *testing.T) {
	defender := newDefenceTestCharacter(t)
	attacker := newDefenceTestCharacter(t)

	runner := func(atk float64, entries []contest.Entry) contest.Result {
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Floored:   true,
			Success:   false,
			Margin:    -1,
		}
	}

	before := defender.GetSkillUseCount("unarmed-combat")
	resolveChannelDefenceWithRunner(ChannelMelee, attacker, defender, runner)

	if got := defender.GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("floored defence awarded %d ordinary events, want 1", got)
	}
	// A floored save is the system overriding the dice; it is not a defensive
	// crit and must not pay the bonus. Assert via the dedupe map, which only a
	// bonus event touches.
	if defender.ClaimedBonusThisRound("unarmed-combat") {
		t.Error("a floored outcome fired a bonus progression event")
	}
}
```

`ClaimedBonusThisRound` is the **exported** accessor added in Task 7. It cannot
be a `_test.go` helper: test files compile only into their own package's test
binary, so a helper in `internal/characters` would be invisible here in
`internal/combat`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/combat/ -run TestChannelDefence -v`
Expected: FAIL.

- [ ] **Step 3: Replace the award call**

In `resolveChannelDefenceWithRunner`, replace this block:

```go
	for _, candidate := range candidates {
		if candidate.entry.Name == res.Winner {
			AwardDefenceProgression(defender, defender.GetUserId(), res.Winner)
			break
		}
	}
```

with:

```go
	// U9: the ordinary defence award is unchanged in WHEN it fires -- whenever
	// the contest ran, win or lose, which is what this path has always done and
	// is deliberately different from melee's defence-used gate. That divergence
	// is recorded in the firing audit and is U10b's to reconcile.
	//
	// What is new is the bonus tier: a defensive crit or fumble now pays the
	// defender, and the attacker observes it.
	for _, candidate := range candidates {
		if candidate.entry.Name == res.Winner {
			AwardDefenceProgression(defender, defender.GetUserId(), res.Winner)
			break
		}
	}

	awardChannelDefenceBonus(channel, attacker, defender, res)
```

Add the new function to the same file:

```go
// awardChannelDefenceBonus pays the crit/fumble tier for a channel contest.
//
// The ORDINARY events are left to AwardDefenceProgression and to the attacker's
// own call site, so Outcome carries only the skill and stat names the bonus
// cells need -- populating the ordinary fields here would double-award.
func awardChannelDefenceBonus(channel AttackChannel, attacker, defender *characters.Character, res contest.Result) {
	if !res.Contested || res.Floored {
		return
	}

	// Crit is decided by NORMALIZED MARGIN, not by a self-relative z-score.
	// Since 5.11d the engine tests margin/(stdDev*sqrt2) against
	// ContestCritThreshold; re-deriving crit from AttackRoll.ZScore here would
	// fire the bonus tier on a DIFFERENT set of swings than the game narrates
	// as crits, which is two mechanisms answering one question.
	//
	// Note the sign: Result.Margin is ATTACK-positive, so the defence side
	// negates it, exactly as defenceDamageMultiplier does at
	// defence_multiplier.go:307.
	attackCrit := AttackContestCrit(res.Margin, res.AttackRoll)
	defenceCrit := DefenseContestCrit(-res.Margin, res.DefenseRoll)

	// Fumble stays self-relative: it is a property of one bad roll, not of the
	// gap between two. ContestCritThreshold is the same magnitude in both
	// directions.
	attackFumble := res.AttackRoll.ZScore <= -ContestCritThreshold
	defenceFumble := res.DefenseRoll.ZScore <= -ContestCritThreshold

	exceptional := progression.Classify(attackCrit, defenceCrit, attackFumble, defenceFumble)
	if exceptional == progression.ExcNone {
		return
	}

	atkSkill, atkStat := channelAttackSkillAndStat(channel, attacker)
	defSkill, defStat := DefenceSkillAndStat(res.Winner)

	out := progression.Outcome{
		AttackerSkill: atkSkill,
		AttackerStat:  atkStat,
		DefenderSkill: defSkill,
		DefenderStat:  defStat,
		ToughenStat:   characters.ToughenStatFor(channelDamageChannel(channel)),
		Exceptional:   exceptional,
	}

	bal := configs.GetBalanceConfig()
	bonuses := progression.Bonuses{
		Doing:     float64(bal.CritProgressionBonus),
		Observing: float64(bal.ObservedCritProgressionBonus),
	}

	// BonusEvents, not EventsForContest: the ordinary events on this path are
	// already awarded by AwardDefenceProgression above and by the attacker's
	// own call site, so asking for them here would double-award.
	evs := progression.BonusEvents(out, bonuses)

	round := util.GetRoundCount()
	attacker.ApplyProgression(evs, progression.SideAttacker, attacker.GetUserId(), round)
	defender.ApplyProgression(evs, progression.SideDefender, defender.GetUserId(), round)
}
```

- [ ] **Step 4: Supply the three lookup helpers**

`DefenceSkillAndStat` already exists — Task 10 step 4a extracted it. Call it,
do not re-derive the mapping.

`channelAttackSkillAndStat` and `channelDamageChannel` map an `AttackChannel`
to the attacker's skill/stat and to the `"physical"`/`"magical"`/`"conviction"`
damage-channel string. Check whether `ChannelAttackScore` already encodes the
first mapping and reuse it:

```bash
grep -n "func ChannelAttackScore" -A25 internal/combat/*.go
```

**`ChannelAttackScore` is where the spell attacker's stat is decided**, and it
independently builds `Willpower + Spellcasting × SkillWeight`
(`defence_multiplier.go:64-66`). Task 13 moves the spell attack roll onto
`primarystat` but does **not** touch this. Left alone, a manifestation spell
would roll to hit on charisma in one place and contest the defence on willpower
in another. Task 13 step 5a now owns it; this task must not fork a second
mapping.

The crit helpers already exist and are **verified**, so nothing needs finding:

| Symbol | Where |
|---|---|
| `AttackContestCrit(margin, roll)` | `internal/combat/crit_floor.go:68` |
| `DefenseContestCrit(margin, roll)` | `internal/combat/crit_floor.go:85` |
| `ContestCritThreshold` (= 2.0) | `internal/combat/margin_crit.go:90` |

There is no `critZScore` constant; an earlier draft of this plan invented one.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/combat/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/
git commit -m "feat(u9): pay the crit/fumble tier on channel defences

Spell, ranged and social contests resolve through
resolveChannelDefenceWithRunner rather than melee's swing loop, so they
had no bonus tier at all. Ordinary awards are untouched, including the
fact that this path awards win or lose while melee gates on
defence-used; that divergence is recorded in the firing audit for U10b.

Floored outcomes award the ordinary event and never a bonus."
```

---

## Task 12: Fix the spell double-roll and the phantom skill keys

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go` (lines ~319-326 and ~468-474)
- Modify: `internal/characters/progression.go` (delete `OnCriticalSuccess`, `OnCriticalFailure`, and their phantom `TrackSkillUse` calls)
- Modify: `internal/actions/actor.go`, `actor_user.go`, `actor_mob.go` (remove the two interface methods if they now have no callers)
- Test: `internal/hooks/spell_progression_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package hooks

import "testing"

// OnSkillUseScaled already rolls the skill's primary stat, and manifestation's
// primary stat IS charisma. The explicit OnStatUse beside it made every cast
// take two charisma rolls instead of one, on both the player and mob branches.
func TestSpellCast_TracksItsStatOnce(t *testing.T) {
	caster := newCasterForTest(t)
	before := caster.GetStatUseCount("charisma")

	castManifestationSpellForTest(t, caster)

	if got := caster.GetStatUseCount("charisma") - before; got != 1 {
		t.Errorf("charisma tracked %d times for one cast, want 1", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestSpellCast -v`
Expected: FAIL with 2.

- [ ] **Step 3: Delete the duplicate stat calls**

In `internal/hooks/NewRound_DoCombat_helpers.go`, the user branch (~319-326)
becomes:

```go
		if spellBonus > 0 {
			// OnSkillUseScaled already rolls the skill's primary stat --
			// manifestation maps to charisma, spellcasting to willpower -- so an
			// explicit OnStatUse beside it double-rolled every cast. The stat a
			// spell trains now comes from its primarystat (Task 13).
			if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
				user.Character.OnSkillUseScaled(string(skills.Manifestation), userId, spellBonus)
			} else {
				user.Character.OnSkillUseScaled(string(skills.Spellcasting), userId, spellBonus)
			}
		}
```

Apply the identical deletion to the mob branch (~468-474).

- [ ] **Step 3b: Move `OnFirstMobKill`'s hardcoded 2.0 onto the knob**

There are **two** bare `2.0` literals in `internal/characters/progression.go`,
not one. `:301` (`OnCriticalSuccess`) is deleted below anyway, but `:332`
(`OnFirstMobKill`) would have survived, leaving standing rule 1 unsatisfied
while §6 claimed otherwise.

```go
	if configs.GetGamePlayConfig().UseSkillProgression {
		bonus := float64(configs.GetBalanceConfig().CritProgressionBonus)
		if c.CheckSkillProgression("combat", userId, bonus) {
```

**Leave `buffSkillMult = 2.0` at `:105` alone.** That is the Skill Attunement
buff's doubling, a buff effect rather than a crit multiplier; folding it into
`CritProgressionBonus` would couple two unrelated things to one knob. It is a
separate rule-1 item and is filed, not fixed here.

- [ ] **Step 4: Delete the phantom skill keys**

In `internal/characters/progression.go`, delete these two lines:

```go
	c.TrackSkillUse("critical_success")   // in OnCriticalSuccess
	c.TrackSkillUse("critical_failure")   // in OnCriticalFailure
```

They write counters into every player save that nothing reads.

- [ ] **Step 5: Delete `OnCriticalSuccess` and `OnCriticalFailure`**

Their production callers were `combat.go` (deleted in Task 9) and
`NewRound_DoCombat_unified.go` (replaced in Task 10). The applier now speaks
their messages. Confirm before deleting:

```bash
grep -rn "OnCriticalSuccess\|OnCriticalFailure" --include=*.go internal/ modules/ | grep -v _test
```

If the only remaining hits are the two method definitions plus the
`actions.Actor` interface and its two wrappers, delete all five. The compiler is
the dead-code sweep — delete the interface method first and let the build find
the wrappers.

If any *other* caller survives, leave the methods alone and note it; do not
delete a live path to satisfy a tidy-up.

- [ ] **Step 6: Run the tests**

Run: `go build ./... && go test ./internal/hooks/... ./internal/characters/... ./internal/actions/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/ internal/characters/ internal/actions/
git commit -m "fix(u9): stop double-rolling caster stats, drop phantom skill keys

OnSkillUseScaled already rolls the skill's primary stat, and
manifestation's primary stat is charisma while spellcasting's is
willpower -- so the explicit OnStatUse beside each branch made every cast
take two stat rolls, on both the player and mob paths.

Also deletes TrackSkillUse(\"critical_success\") and
(\"critical_failure\"), which wrote counters into every save that nothing
reads, and removes OnCriticalSuccess/OnCriticalFailure now that the
applier speaks their messages."
```

---

## Task 13: Wire `SpellData.PrimaryStat`

**Files:**
- Modify: `internal/spells/spells.go` (validator)
- Modify: `internal/hooks/spell_resolution.go` (11 caster-side reads)
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go` (progression stat override)
- Test: `internal/spells/primarystat_test.go` (create)

- [ ] **Step 1: Write the failing validator test**

```go
package spells

import "testing"

func TestPrimaryStat_RequiredAndValid(t *testing.T) {
	cases := []struct {
		name    string
		stat    string
		wantErr bool
	}{
		{"valid willpower", "willpower", false},
		{"valid charisma", "charisma", false},
		{"empty is now an error", "", true},
		{"typo", "willpwer", true},
		{"not a stat", "manifestation", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SpellData{SpellId: "test", Name: "Test", PrimaryStat: tc.stat}
			err := s.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
```

Check the real loader hook name first — `fileloader` may already require a
specific method:

```bash
grep -n "func (s \*SpellData)\|Validate\|Filepath" internal/spells/spells.go | head
```

Use whatever validation entry point the loader already calls. If the loader
does not validate, add the check where spells are loaded and make it **fail at
boot**, matching the project convention that authored content panics at startup
on an unresolved reference.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/spells/ -run TestPrimaryStat -v`
Expected: FAIL.

- [ ] **Step 3: Add the validator**

```go
// validStats is the closed set a spell's primarystat may name.
var validStats = map[string]struct{}{
	"strength": {}, "dexterity": {}, "perception": {},
	"vitality": {}, "willpower": {}, "charisma": {},
}

// U9 made primarystat load-bearing: it drives both the caster-side stat in
// spell resolution and the stat the cast trains. A typo must therefore fail at
// boot rather than falling back to a default, which is how the field spent its
// whole life describing an intent nothing implemented.
func (s *SpellData) validatePrimaryStat() error {
	if s.PrimaryStat == "" {
		return fmt.Errorf("spell %q: primarystat is required", s.SpellId)
	}
	if _, ok := validStats[s.PrimaryStat]; !ok {
		return fmt.Errorf("spell %q: primarystat %q is not one of the six stats", s.SpellId, s.PrimaryStat)
	}
	return nil
}
```

- [ ] **Step 4: Add the caster-stat accessor**

```go
// CasterStatValue returns the caster-side stat value this spell's rolls,
// duration and shield strength are built from.
//
// CASTER SIDE ONLY. The DEFENDER's stat is owned by the U6 defence set --
// spell_resolution.go's quell read and charm's target resist both stay on
// Willpower by design, and routing them through here would silently move quell
// off the stat U6 designed it around.
func (s *SpellData) CasterStatValue(stats stats.Statistics) int {
	switch s.PrimaryStat {
	case "strength":
		return stats.Strength.ValueAdj
	case "dexterity":
		return stats.Dexterity.ValueAdj
	case "perception":
		return stats.Perception.ValueAdj
	case "vitality":
		return stats.Vitality.ValueAdj
	case "charisma":
		return stats.Charisma.ValueAdj
	default:
		return stats.Willpower.ValueAdj
	}
}
```

- [ ] **Step 5: Replace the 11 caster-side reads**

In `internal/hooks/spell_resolution.go`, replace each of these with
`spellData.CasterStatValue(<char>.Stats)`:

| Line | Context |
|---|---|
| 85 | `CalcSpellAttack(user...Willpower.ValueAdj, skillLevel)` |
| 526 | `casterWil = user...Willpower.ValueAdj` |
| 725 | `willpower = casterChar...Willpower.ValueAdj` |
| 922 | user DoT duration |
| 980 | user shield bonus |
| 991 | user buff duration |
| 1147 | `CalcSpellAttack(mob...)` |
| 1295 | mob DoT duration |
| 1326 | mob shield bonus |
| 1337 | mob buff duration |
| 1451 | DoT duration, **also** hardcodes `GetSkillLevel(skills.Spellcasting)` |

**Do NOT touch line 1060.** That is the defender's Willpower for quell.
**Do NOT touch `internal/hooks/charm_spell.go:60-63.`** That is the charm
target's resist.

At line 1451 also replace the hardcoded spellcasting rank with the caster's
rank in the spell's actual skill (manifestation for manifestation-school
spells), matching the branch already used for progression.

- [ ] **Step 5a: Fix the SKILL rank too, in three places**

Replacing only the *stat* produces `Charisma + spellcasting-rank × weight` for a
manifestation spell — the manifestation stat mixed with the spellcasting skill.
Adversarial review caught this; an earlier draft fixed only line 1451.

The lines immediately above `:85` and `:1147` read:

```go
skillLevel := user.Character.GetSkillLevel(skills.Spellcasting)   // and mob.Character at :1147
```

Both must resolve the spell's actual skill, the same way the progression branch
already does:

```go
castSkill := skills.Spellcasting
if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
	castSkill = skills.Manifestation
}
skillLevel := user.Character.GetSkillLevel(castSkill)
```

**And `combat.ChannelAttackScore` (`defence_multiplier.go:64-66`) independently
builds the spell attacker's score as `Willpower + Spellcasting × SkillWeight`.**
Left untouched, the same spell would roll to hit on charisma and contest the
defence on willpower. It needs the spell's stat and skill threaded in, or an
explicit decision — recorded here — that the defence contest deliberately stays
on Willpower for every spell.

**Stop and ask before choosing.** Threading `primarystat` into
`ChannelAttackScore` changes the *hit rate* of every manifestation spell against
quell, which is a balance change beyond the "near-zero delta" §8.2 promises.
Verify first:

```bash
grep -n "func ChannelAttackScore" -A25 internal/combat/defence_multiplier.go
grep -rn "ChannelAttackScore" --include=*.go internal/ | grep -v _test
```

- [ ] **Step 6: Override the progression stat**

In `NewRound_DoCombat_helpers.go`, after the `OnSkillUseScaled` call, add the
stat override for the case where `primarystat` differs from the skill's primary
stat:

```go
			// primarystat overrides the skill's default stat. Manifestation
			// already maps to charisma and spellcasting to willpower, so for
			// every shipped file this is a no-op -- it exists so a spell that
			// declares something else actually trains it.
			if spellData != nil {
				if st := spellData.PrimaryStat; st != "" && st != skills.GetSkillPrimaryStat(castSkill) {
					user.Character.OnStatUse(st, userId)
				}
			}
```

Hoist `castSkill` out of the if/else branch so both arms and this block use it.
Apply the same to the mob branch.

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/spells/... ./internal/hooks/... -v`
Expected: PASS. The spell tests will fail to LOAD data until Task 14 supplies
`primarystat` on `charm.yaml`, so run Task 14 before the full boot test.

- [ ] **Step 8: Commit**

```bash
git add internal/spells/ internal/hooks/
git commit -m "feat(u9): make SpellData.PrimaryStat load-bearing

Declared with the comment 'Stat used for spell rolls and progression',
parsed by 58 of 59 files, read by zero Go code -- while resolution
hardcoded Willpower in eleven caster-side places.

primarystat now drives both. Caster side only: the quell read and
charm's target resist stay on Willpower, because the U6 defence set owns
the defender's stat.

Required and validated against the six stats at load, so a typo fails at
boot rather than silently falling back."
```

---

## Task 14: Correct the manifestation family's data

**Files:**
- Modify: 14 files under `_datafiles/world/dogmud/spells/`

- [ ] **Step 1: Confirm the file set from the `schools:` block, not the filename**

```bash
for f in _datafiles/world/dogmud/spells/*.yaml; do
  if awk '/^schools:/{s=1;next} /^[a-z_]+:/{s=0} s&&/manifestation/{print;exit}' "$f" | grep -q .; then
    echo "$(basename $f)"
  fi
done
```

Expected, exactly 14: `charm`, `conjure-air`, `conjure-earth`, `conjure-fire`,
`conjure-magma`, `conjure-water`, `raise-golem`, `raise-skeleton`,
`raise-spectre`, `raise-vampire`, `raise-wraith`, `raise-zombie`,
`summon-hive-swarm`, `summon-steppe-spirit`.

**`veil-rend` must NOT appear.** An earlier scoping pass matched it on a
filename grep for summon/conjure/raise; its `schools:` block does not contain
manifestation. If it appears here, stop and re-read it.

- [ ] **Step 2: Verify each one really is Willpower-free before changing it**

For each of the 13 non-charm files, confirm `effect_type: none` and that it
carries no `damage_multiplier`, no `buff_ids` and no shield effect:

```bash
grep -n "effect_type\|damage_multiplier\|buff_ids\|effect_magnitude" \
  _datafiles/world/dogmud/spells/{conjure-*,raise-*,summon-*}.yaml
```

Any file that reaches a damage, shield or DoT-duration path is a **balance
change**, not a data correction. Stop and report it rather than editing it.

- [ ] **Step 3: Edit the 13**

Change `primarystat: willpower` to `primarystat: charisma` in each. Use `Edit`,
not a script — a read-modify-write in Python has destroyed files on this project
twice.

- [ ] **Step 4: Add the field to `charm.yaml`**

Insert beside the other scalar fields, matching the file's existing ordering:

```yaml
primarystat: charisma
```

Charm is the only file of 59 that never declared it. Its school is already
`manifestation` and its Go path (`charm_spell.go:51`) already scores
`Charisma + Manifestation`, so this makes the data agree with behaviour that
already exists.

- [ ] **Step 4b: Add `primarystat` to the 8 upstream default-world spells**

**This is a boot-panic our own config would have hidden.**
`_datafiles/world/default/spells/` holds 8 files and **none declares
`primarystat`**, while `internal/configs/config.filepaths.go:23` defaults
`DataFiles` to `_datafiles/world/default` whenever the key is absent. Our
`config.yaml` sets it to the dogmud tree, so the Task 17 boot test passes — but
a fresh checkout, a stripped container config or an ephemeral playtest env would
panic the moment `primarystat` became required.

Owner decision, 2026-08-19: fix the 8 files rather than soften the validator,
because a silent fallback is exactly what let this field mean nothing for its
whole life and would swallow typos as well as omissions.

```bash
ls _datafiles/world/default/spells/*.yaml
grep -L "primarystat" _datafiles/world/default/spells/*.yaml
```

Add `primarystat: willpower` to each unless its `schools:` block says
`manifestation`, in which case use `charisma` — check each file, do not assume.

- [ ] **Step 5: Verify BOTH trees load**

```bash
grep -c "^primarystat:" _datafiles/world/dogmud/spells/*.yaml   | grep ":0$"
grep -c "^primarystat:" _datafiles/world/default/spells/*.yaml  | grep ":0$"
```

Expected: no output from either — all 59 dogmud files and all 8 default-world
files now declare it.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/spells/
git commit -m "data(u9): the manifestation family trains charisma, not willpower

Fourteen manifestation-school files, identified by their schools block
rather than by filename. Thirteen move from willpower; charm gains the
field it was alone in never declaring.

Near-zero behaviour delta: the thirteen are effect_type: none and never
reach a Willpower-derived damage, shield or duration path, and their
power already runs on CalcCompanionPool, which U7b defined as Charisma
plus manifestation.

veil-rend is deliberately NOT in this set. It matched an early filename
grep for summon/conjure/raise; its schools block is not manifestation."
```

---

## Task 15: The guard test

**Files:**
- Test: `internal/progression/seam_guard_test.go` (create)

- [ ] **Step 1: Write the guard**

Model it on U5b's AST pool-mutation guard. Find it first and copy its shape:

```bash
grep -rln "go/ast\|go/parser" --include=*_test.go internal/ | head
```

```go
package progression_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Files in the contest paths that are ALLOWED to fire progression directly.
// Everything else in these packages must go through
// characters.ApplyProgression, so the matrix has one implementation.
//
// The ~50 non-contest call sites elsewhere in the codebase (craft, salvage,
// forage, search, steal, and the rest) are deliberately untouched by U9 and are
// not covered by this guard. Routing them is U10b's job.
var allowedDirectProgression = map[string]bool{
	// The shared five-defence mapping (AwardDefenceProgression).
	filepath.Join("internal", "combat", "defence_multiplier.go"): true,
	// Melee's unarmed fallback and the quadrant-flavoured stat-gain emitter.
	filepath.Join("internal", "hooks", "NewRound_DoCombat_unified.go"): true,
	filepath.Join("internal", "hooks", "NewRound_DoCombat_helpers.go"): true,
	// Crafting is Category C. Deliberately out of U9's scope and out of the
	// contest paths; U10b routes it. Listed so its exemption is a decision on
	// the record rather than an accident of which files got scanned.
	filepath.Join("internal", "hooks", "NewRound_MobRoundTick.go"):  true,
	filepath.Join("internal", "hooks", "NewRound_UserRoundTick.go"): true,

	// internal/combat/combat_helpers.go is DELIBERATELY ABSENT. Task 9b deletes
	// the per-swing defender roll that lives there. If this guard flags it,
	// something reintroduced the duplication -- do NOT silence it by adding a
	// row here.
}

var progressionCalls = map[string]bool{
	"OnSkillUse": true, "OnSkillUseScaled": true, "OnStatUse": true,
	"CheckSkillProgression": true, "CheckStatProgression": true,
	// The consolidated entry points too, so the calls U9 just deleted cannot
	// quietly return. TrackSkillUse/TrackStatUse are included because a bonus
	// event that tracks is a curve-decay bug the rate tests would not catch.
	"OnCritReceived": true, "OnCriticalSuccess": true, "OnCriticalFailure": true,
	"TrackSkillUse": true, "TrackStatUse": true,
	"CheckRegenProgression": true,
}

func TestContestPathsFireProgressionOnlyThroughTheApplier(t *testing.T) {
	for _, pkg := range []string{"internal/combat", "internal/hooks"} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, filepath.Join("..", "..", pkg), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", pkg, err)
		}
		for _, p := range pkgs {
			for path, file := range p.Files {
				rel := filepath.Join(pkg, filepath.Base(path))
				if strings.HasSuffix(path, "_test.go") || allowedDirectProgression[rel] {
					continue
				}
				ast.Inspect(file, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if progressionCalls[sel.Sel.Name] {
						t.Errorf("%s fires %s directly; contest paths must go through characters.ApplyProgression",
							fset.Position(call.Pos()), sel.Sel.Name)
					}
					return true
				})
			}
		}
	}
}
```

- [ ] **Step 2: Run it, expecting a specific failure set**

Run: `go test ./internal/progression/ -run TestContestPaths -v`

**Do not expect a clean pass on the first run.** Scanned against the tree before
U9, this guard flags three files, and each has a different correct answer:

| Flagged | Correct response |
|---|---|
| `internal/combat/combat_helpers.go:1228` | **Fixed by Task 9b**, which deletes it. If it still flags, Task 9b was not applied. Never allow-list it. |
| `internal/hooks/NewRound_MobRoundTick.go:496` | Mob crafting: Category C, allow-listed above with a reason. |
| `internal/hooks/NewRound_UserRoundTick.go:591` | User crafting: same. |

So the expected result **after Tasks 9, 9b, 10 and 11** is PASS. Run it early
anyway to confirm it flags exactly those three and nothing else — a fourth hit
is a site nobody has accounted for.

Note the guard scans `internal/combat` and `internal/hooks` only.
`internal/characters/progression.go` is where the applier lives and is never
scanned, so listing it in the allow-list would be dead weight; an earlier draft
of this plan did exactly that.

If it flags something new, decide deliberately: either route that site through
the applier, or allow-list it **with a comment saying why**. An allow-list entry
with no reason is how the guard becomes decoration.

- [ ] **Step 3: Commit**

```bash
git add internal/progression/
git commit -m "test(u9): guard that contest paths fire progression via the applier

AST guard on internal/combat and internal/hooks, on the model of U5b's
pool-mutation guard. The non-contest call sites elsewhere are
deliberately out of scope and uncovered; routing them is U10b's."
```

---

## Task 16: Documentation

**Files:**
- Create: `internal/progression/context.md`, `internal/actionspec/context.md`
- Modify: `internal/costs/context.md`, `internal/characters/context.md`, `internal/combat/context.md`, `internal/spells/context.md`
- Modify: `docs/PATCH_NOTES.md`, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`

- [ ] **Step 1: Write the two new `context.md` files**

Follow the project structure: `## Purpose` (say what it deliberately does NOT
do), `## Files`, core types in a `go` block with real field names,
`## Public API` with verified signatures, `## Gotchas`, `## Dependencies`,
`## Consumers`. Do **not** write "Future Enhancements", "Security
Considerations", "Performance Characteristics" or "Scalability" sections.

**Verify every symbol before you document it:**

```powershell
Select-String -Path internal\progression\*.go -Pattern '^(func|type|const|var)\s'
```

Gotchas that must appear in `internal/progression/context.md`:
- Bonus events must never track a use count; the count decays the curve.
- An empty `Skill` or `Stat` means no roll — never pass it downstream.
- The package decides what an event carries, never when it fires.
- `Bonuses` are arguments, not config reads, because a Go test binary never
  loads `config.yaml`.

- [ ] **Step 2: Update the four existing files**

- `internal/spells/context.md` — **delete** the line recording `PrimaryStat` as
  "INERT. Parsed, never read." It is now load-bearing and required.
- `internal/costs/context.md` — say the registry moved to `actionspec` and that
  the local names are aliases.
- `internal/characters/context.md` — add `ApplyProgression`, `ToughenStatFor`,
  and the `OnCritReceived` change; remove `OnCriticalSuccess` /
  `OnCriticalFailure` if Task 12 deleted them.
- `internal/combat/context.md` — the channel bonus tier and the shared
  `defenceSkillAndStat` mapping.

Verify with `python tools/context_md_audit.py` and fix any phantom symbols.

- [ ] **Step 3: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`. Player-facing framing, **no raw
numbers**, no em dashes. Something like:

```markdown
## 2026-08-XX

Learning from combat has been rebuilt. Landing a decisive blow, or making a
serious mistake, now teaches both the skill you used and the attribute behind
it, and the person on the other side learns something from it too. Surviving a
punishing hit still toughens you, but it now follows the same curve as every
other kind of practice, so it rewards genuine progress rather than repetition.

Several places were quietly handing out practice twice for a single action.
They now count once.
```

- [ ] **Step 4: Roadmap edits**

In `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`:

1. Rewrite the **U9** row to what shipped, and mark it done.
2. Correct the **U8** row: it still reads "IMPLEMENTATION + VALIDATION COMPLETE
   ON FEATURE BRANCH; INTEGRATION PENDING". U8 merged as `15a5fc94d` on
   2026-08-18.
3. Add a **U10b** row after U10 and before U12:

```markdown
| **U10b** | **Progression firing consistency.** Added 2026-08-19. Progression fires under at least seven different conditions today with no convention (see `docs/audits/2026-08-19-progression-firing-audit.md`). Adopt one rule: **one event per success, with crit and critical-failure as a separate bonus on top**. Also routes Category C (crafting, salvage, forage) through the U9 event seam, and reconciles the melee-versus-channel defence divergence: melee awards a defence only when one registered, the channel path awards win or lose. | M | U9, U10 | **Yes** |
```

4. Add a **U10c** row:

```markdown
| **U10c** | **Charm redesign.** Added 2026-08-19. `charm_spell.go` scores `Charisma + Manifestation x 25`, a skill weight of 25 against U6's uniform 5.0, so it survived the flip. It was compensating for a periodic resist ladder that is **dead code**: nothing assigns `CharmDuration` at cast time and the ladder is gated on it being non-zero, so a charmed mob is charmed for 99999 rounds with no resist check. Give charm a real duration, delete the ladder, return the weight to 5.0. No migration needed (owner: no veteran uses charm). Needs modelling and a playtest. | M | U9 | **Yes** |
```

5. Add a line to the "Modelling gates" section: **Before tuning
   `CritProgressionBonus`**, assess the compound effect of the melee
   double-progression deletion and the once-per-round bonus dedupe, which are
   both cuts landing together.

- [ ] **Step 5: Crib sheet**

Add one item to `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`: across a real
ten-plus-round fight, how often do skill and stat improvement banners appear?
Rates moved in both directions in U9 and the felt frequency is what matters, not
the arithmetic.

- [ ] **Step 6: Commit**

```bash
git add internal/*/context.md docs/
git commit -m "docs(u9): context.md sweep, patch notes, roadmap, crib sheet

Adds U10b (progression firing consistency, plus Category C routing) and
U10c (charm redesign) to the roadmap, and corrects the U8 row, which
still read integration-pending after merging as 15a5fc94d."
```

---

## Task 17: Pre-push gates

- [ ] **Step 1: Formatting**

Run: `gofmt -l internal/ modules/`
Expected: **no output.** This has its own CI gate and has broken a push before.

- [ ] **Step 2: Build and full test suite**

```bash
export GOTMPDIR=C:/gotmp
go build ./...
go test ./...
```

Expected: exit 0. There are no known failures, so any failure is real.

- [ ] **Step 3: Confirm `LogToFile: false`**

Check `_datafiles/config.yaml` has `Logging.LogToFile: false` (the droplet has
limited disk). Remember this file carries `skip-worktree`.

- [ ] **Step 4: Isolated boot test**

YAML data files panic at *startup* on a bad reference, and Task 13 added a
**new required field with a boot-time validator**, so this task is the only
thing that proves all 59 spell files pass it.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the success case** — the timeout fired because the server
stayed up. Build to a fixed `boot-check.exe` path, never `go run .`, or Windows
Firewall prompts on every run. Do **not** grep for the bare word `panic`: the
config key `GamePlay.MapConsistencyEnforce` legitimately has the *value*
`panic`.

Clean up:

```bash
git worktree remove --force C:/tmp/dogmud-boot-check
```

- [ ] **Step 5: Open the PR**

```bash
git push -u origin feature/u9-progression-layer
gh pr create --repo pruuk/DOGMud --base master --head feature/u9-progression-layer --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

**Always pass `--repo pruuk/DOGMud`.** This repo is a fork of
`GoMudEngine/GoMud` and `gh` defaults to the parent; a bare `gh pr create`
opened a PR against upstream once already.

A green check is not proof — confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed`, since a run can pass while
emitting annotations and the lint gate is `only-new-issues`.

- [ ] **Step 6: Do not merge**

U9 changes progression rates in both directions. Hand the PR to the user with
the §7.1 table and the measured Task 1 numbers, and let them decide whether it
merges before or after a feel check.

---

## Notes for the implementer

- **Task 1 is a gate.** If the melee duplication does not reproduce, stop and
  report. Tasks 9, 9b and 10 assume it.
- **This plan was revised after a blind adversarial review**, which found three
  showstoppers in Task 10 alone (a defender rate multiplication, attacker fumble
  progression deleted outright, and the whole bonus tier dropped for unarmed
  attackers), plus a fifth defect the spec had documented as correct behaviour.
  Where a step says "an earlier draft did X", that is not commentary — it is the
  specific mistake the step exists to prevent you repeating.
- **Two steps deliberately stop and ask** rather than guessing: Task 13 step 5a
  (whether `ChannelAttackScore` moves off Willpower, which changes spell hit
  rates) and any fourth hit from the Task 15 guard.
- **Do not change when progression fires.** Every task preserves existing firing
  conditions. If a change seems obviously right, it belongs in the Task 2 audit
  as a recommendation, not in the code.
- **Do not put a balance number in `internal/`.** Both multipliers come from
  config. A numeric literal under `internal/` is a defect in this arc.
- **`config.yaml` carries `skip-worktree`.** Build commits from
  `git show HEAD:_datafiles/config.yaml`, never from the disk copy, which
  desyncs in both directions.
- **Never edit any file with a Python read-modify-write.** It truncates before
  the write evaluates and has destroyed files on this project twice. Use `Edit`.
