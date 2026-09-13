# Conditions unification, slice 1: one model, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the ten-value combat condition enum by making each condition a buff record with a per-instance magnitude and a closed effect vocabulary that combat reads through one door, number-identical to today except for the changes the spec states.

**Architecture:** `Buff` gains `Magnitude`; `BuffSpec` gains `effects` (seven closed keys, literal or `magnitude`) and `tick_from_magnitude`; `Buffs.Effect(kind)` / `HasEffect(kind)` is the reader door and `AddBuffMagnitude(id, rounds, magnitude, source)` the writer door at the collection, character, user and event. Nine live conditions migrate one per task (producer, consumers, tests together, so every commit is green and number-identical); the tenth has no producer and is deleted. Then the enum, its tick, the shield decay helpers and `condition-mirror` are deleted, display and GMCP collapse to one list, guards pin the shape.

**Tech Stack:** Go 1.25, `go test`, yaml.v2 for specs, the buff notice / flag / apply-path root guards, the narration goldens, the playtest harness.

Spec: `docs/superpowers/specs/2026-09-12-conditions-unification-slice-1-model-design.md`. Read it first. Branch `feature/conditions-unification-slice-1-model` (exists; spec committed).

## Rules for this plan

- **One implementer at a time in the checkout.** Reviewers only in parallel. No agent runs `git reset`. (A commit race in 5b swept one agent's staged files into another's commit.)
- **Edit tool only.** Never a Python read-modify-write (it truncated and converted line endings in 5b). All `.go` files are LF; if `gofmt -l` flags a whole file, look for CRLF.
- **Run every test command standalone**, never `| tail`; `grep -c` exits 1 on zero matches and breaks an `&&` chain, so run "expect zero" greps alone.
- **`buffs.golden` is re-recorded ONCE, in Task 3, then `-update` is forbidden.** The other nine goldens must never change.
- **Numbers do not change.** Every magnitude, duration, floor and multiplier in the migration tasks is copied from the producer it replaces. A review that finds a different number is a must-fix.
- Named paths only in `git add`; commit messages end with a separate final line `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`; every `gh` command carries `--repo pruuk/DOGMud`.
- Buff YAML is CRLF: grep with `\s*$`, never a bare `$`.
- New files: state the full path in the report and add docs to `docs/README.md` (Task 14).

## File map

| File | Responsibility |
|---|---|
| `internal/buffs/ids.go` (new) | the record ids the engine names |
| `internal/buffs/effects.go` (new) | `EffectKind`, `EffectValue`, `AllEffectKinds`, `Effect`, `HasEffect`, effects validation |
| `internal/buffs/effects_test.go` (new) | door tests, YAML round trip |
| `internal/buffs/buffs.go` | `Buff.Magnitude`, `AddBuffMagnitude`, tick snapshot from magnitude |
| `internal/buffs/buffspec.go` | `Effects`, `TickFromMagnitude`, flags `bleeding` and `quiet`, validation |
| `internal/buffs/notice.go` | `quiet` silences both notices |
| `internal/characters/buffs.go` | `Character.AddBuffMagnitude` |
| `internal/events/eventtypes.go` | `events.Buff.Magnitude` |
| `internal/users/userrecord.go` | `UserRecord.AddBuffMagnitude` |
| `internal/hooks/Buff_ApplyBuffs.go` | passes the magnitude through |
| `internal/hooks/NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go` | damaging ticks call the two cancel helpers; `TickConditions` calls removed |
| `_datafiles/world/dogmud/buffs/79-warcry.yaml`, `80-rally.yaml` | gain `effects`, lose `condition-mirror` |
| `_datafiles/world/dogmud/buffs/117-off_balance.yaml` ... `123-enchant_withdrawal.yaml` (new) | the seven new records |
| `internal/narration/testdata/stores/buffs.golden` | re-recorded once |
| per-condition producer and consumer files | Tasks 4 to 10 |
| `internal/characters/conditions.go`, `conditions_test.go`, `conditions_immunity_test.go` | DELETED in Task 11 |
| `internal/usercommands/conditions.go`, `modules/gmcp/gmcp.Char.go`, `_datafiles/html/public/webclient-pure.html` | one list, one payload |
| `timed_state_guard_test.go` (new, repo root) | no second timed-state collection |
| six `context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`, the spec | Task 14 |
| `tools/playtest/scenarios/conditions-slice-1.yaml` + goals (new) | Task 15 |

---

### Task 0: Pin today's numbers before anything moves

The pins use today's `AddCondition` API and literal expectations. Every migration task rewrites the pin's SETUP to the record API and must keep every literal. A reviewer who sees a literal change has found a number change.

**Files:**
- Create: `internal/characters/conditions_pin_test.go`
- Create: `internal/hooks/conditions_pin_test.go`

- [ ] **Step 1: The characters pins**

Create `internal/characters/conditions_pin_test.go`:

```go
package characters

import (
	"math"
	"testing"
)

// Pins for the conditions unification (slice 1). Each literal below is what
// the OLD enum path produces at master 230041292; the migration tasks change
// only the setup lines and must leave every literal untouched.

func pinCharacter() *Character {
	c := &Character{}
	c.Buffs.Validate(true)
	c.Name = "Pin"
	c.HealthMax.Value = 200
	c.Health = 200
	c.StaminaMax.Value = 100
	c.Stamina = 100
	c.ConvictionMax.Value = 80
	c.Conviction = 80
	return c
}

func TestPin_ShieldAddsFlatPhysicalMitigation(t *testing.T) {
	c := pinCharacter()
	before := c.GetPhysicalMitigation()
	c.AddCondition(ConditionShield, 10, 12, "pin") // SETUP: migrates in Task 6
	got := c.GetPhysicalMitigation() - before
	if math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("shield 12 must add exactly 0.12 mitigation, got %v", got)
	}
}

func TestPin_WithdrawalCutsThePoolMaximumByTheFraction(t *testing.T) {
	c := pinCharacter()
	c.AddCondition(ConditionEnchantWithdrawal, 50, 0.25, "health") // SETUP: migrates in Task 10
	c.Validate()
	if c.HealthMax.Value != 150 {
		t.Fatalf("health max 200 with a 0.25 withdrawal must read 150, got %d", c.HealthMax.Value)
	}
	if c.StaminaMax.Value != 100 {
		t.Fatalf("a health withdrawal must not touch stamina, got %d", c.StaminaMax.Value)
	}
}

func TestPin_WithdrawalOnStaminaAndConviction(t *testing.T) {
	c := pinCharacter()
	c.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "stamina") // SETUP: Task 10
	c.Validate()
	if c.StaminaMax.Value != 50 {
		t.Fatalf("stamina max 100 with a 0.5 withdrawal must read 50, got %d", c.StaminaMax.Value)
	}
	c2 := pinCharacter()
	c2.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "conviction") // SETUP: Task 10
	c2.Validate()
	if c2.ConvictionMax.Value != 40 {
		t.Fatalf("conviction max 80 with a 0.5 withdrawal must read 40, got %d", c2.ConvictionMax.Value)
	}
}
```

`conditions_test.go` builds characters as `&Character{}`; `Validate()` may need `c.Stats` populated, in which case copy the fixture from `validate_test.go` in the same package. The literals stay.

- [ ] **Step 2: Run, expect green (these pin TODAY)**

Run: `go test ./internal/characters/ -run 'TestPin_' -count=1`
Expected: `ok`. If a pin is red, the literal is wrong about today; fix the literal by reading the consumer (`combat.go:185`, `validate.go:187-221`), never the setup.

- [ ] **Step 3: The hooks pin: poison kills on the round it reaches zero, with the cause "poison"**

Read `internal/hooks/hooks_test.go` around `TestAutoHeal_PoisonDamage` (line ~909) for the fixture (`seedAllRegistries`, `users.GetByUserId(1)`), and `internal/hooks/Death_PlayerAnnouncement.go:118-132` for how the cause is derived. Create `internal/hooks/conditions_pin_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Pin: a poisoned player at 1 health dies to the poison tick and the death
// cause reads "poison". Setup migrates in Task 8; the assertions do not.
func TestPin_PoisonTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	u := users.GetByUserId(1)
	u.Character.Health = 1
	u.Character.AddCondition(characters.ConditionPoisoned, 10, 5.0, "pin") // SETUP: Task 8

	AutoHeal(events.NewRound{RoundNumber: 30}) // SETUP: Task 8 runs UserRoundTick instead
	require.LessOrEqual(t, u.Character.Health, 0, "the poison tick must take the last point")
	require.True(t, u.Character.HasCondition(characters.ConditionPoisoned)) // SETUP: Task 8 tests the poison flag
}
```

Then read `Death_PlayerAnnouncement.go` to find the function that computes the cause string and, if it is callable from a test with a character (it takes `c`), add a second test asserting it returns `"poison"` for a poisoned character not in combat, and `"bleeding out"` for a bleeding one. If the cause is computed inline inside the listener, assert instead on the announcement text the listener emits (drain the user's queued messages with `drainPlain(u.UserId)` after firing the death event as the existing death tests do; find one with `grep -n "Death" internal/hooks/hooks_test.go`).

Run: `go test ./internal/hooks/ -run 'TestPin_' -count=1` → `ok`.

- [ ] **Step 4: Commit**

```bash
git add internal/characters/conditions_pin_test.go internal/hooks/conditions_pin_test.go
git commit -m "test(conditions): pin today's shield, withdrawal and poison numbers before the model changes"
```

---

### Task 1: The record: magnitude, effects, the door, the writer, two flags

**Files:**
- Create: `internal/buffs/ids.go`, `internal/buffs/effects.go`, `internal/buffs/effects_test.go`
- Modify: `internal/buffs/buffs.go` (`Buff` struct, new `AddBuffMagnitude`), `internal/buffs/buffspec.go` (spec fields, flags, `AllFlags`, `Validate`), `internal/buffs/notice.go`

- [ ] **Step 1: Failing tests**

Create `internal/buffs/effects_test.go`:

```go
package buffs

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func withSpecs(t *testing.T, specs ...*BuffSpec) {
	t.Helper()
	m := map[int]*BuffSpec{}
	for _, s := range specs {
		m[s.BuffId] = s
	}
	t.Cleanup(SeedBuffsForTest(m))
}

func TestEffectValueParsesANumberOrTheWordMagnitude(t *testing.T) {
	var s BuffSpec
	err := yaml.Unmarshal([]byte("buffid: 900\nname: Probe\neffects:\n  damage_mult: magnitude\n  defense_mult: 0.85\n"), &s)
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Effects[EffectDamageMult]; !v.UsesMagnitude {
		t.Fatalf("damage_mult should use the magnitude, got %+v", v)
	}
	if v := s.Effects[EffectDefenseMult]; v.UsesMagnitude || v.Literal != 0.85 {
		t.Fatalf("defense_mult should be the literal 0.85, got %+v", v)
	}
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back BuffSpec
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Effects[EffectDamageMult] != s.Effects[EffectDamageMult] || back.Effects[EffectDefenseMult] != s.Effects[EffectDefenseMult] {
		t.Fatalf("effects must round-trip through yaml, got %+v", back.Effects)
	}
}

func TestValidateRefusesAnUnknownEffectKey(t *testing.T) {
	var s BuffSpec
	if err := yaml.Unmarshal([]byte("buffid: 901\nname: Probe\neffects:\n  damage_multt: 1\n"), &s); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("an unknown effect key must be refused at load")
	}
}

func TestValidateRefusesTickFromMagnitudeWithoutAPool(t *testing.T) {
	s := &BuffSpec{BuffId: 902, Name: "Probe", TickFromMagnitude: true}
	if err := s.Validate(); err == nil {
		t.Fatal("tick_from_magnitude needs tick_pool")
	}
	s2 := &BuffSpec{BuffId: 903, Name: "Probe", TickFromMagnitude: true, TickPool: "health", TickPercent: -0.1, TriggerRate: "1 round", TriggerCount: 3}
	if err := s2.Validate(); err == nil {
		t.Fatal("tick_from_magnitude and tick_percent cannot both be set")
	}
}

func TestEffectMultipliesFlatsSumCapsTakeTheMinimum(t *testing.T) {
	withSpecs(t,
		&BuffSpec{BuffId: 910, Name: "Shout", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}, EffectDefenseMult: {UsesMagnitude: true}}},
		&BuffSpec{BuffId: 911, Name: "Exposed", TriggerRate: "1 round", TriggerCount: 1, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}, EffectAttacksCap: {Literal: 1}}},
		&BuffSpec{BuffId: 912, Name: "Ward", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}},
		&BuffSpec{BuffId: 913, Name: "Ward2", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}, EffectAttacksCap: {Literal: 3}}},
	)
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(910, 5, 1.12)
	bs.AddBuffMagnitude(911, 1, 0)
	bs.AddBuffMagnitude(912, 5, 12)
	bs.AddBuffMagnitude(913, 5, 5)

	if got := bs.Effect(EffectDamageMult); got != 1.12 {
		t.Fatalf("damage mult: got %v", got)
	}
	if got := bs.Effect(EffectDefenseMult); got != 1.12*0.85 {
		t.Fatalf("defense mult must multiply across records: got %v", got)
	}
	if got := bs.Effect(EffectMitigationFlat); got != 17 {
		t.Fatalf("flats must sum: got %v", got)
	}
	if got := bs.Effect(EffectAttacksCap); got != 1 {
		t.Fatalf("caps take the minimum: got %v", got)
	}
	if got := bs.Effect(EffectRegenMult); got != 1 {
		t.Fatalf("an absent multiplier reads 1: got %v", got)
	}
	if got := bs.Effect(EffectPoolMaxPct); got != 0 {
		t.Fatalf("an absent flat reads 0: got %v", got)
	}
	if !bs.HasEffect(EffectMitigationFlat) || bs.HasEffect(EffectRegenMult) {
		t.Fatal("HasEffect must report declared kinds only")
	}
}

func TestMagnitudeZeroOnAMultiplierContributesNothing(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 914, Name: "Hollow", TriggerRate: "1 round", TriggerCount: 5, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(914, 5, 0)
	if got := bs.Effect(EffectDamageMult); got != 1 {
		t.Fatalf("a zero magnitude must not zero the product: got %v", got)
	}
}

func TestExpiredRecordsDoNotContribute(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 915, Name: "Brief", TriggerRate: "1 round", TriggerCount: 1, RoundInterval: 1, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(915, 1, 1)
	if got := bs.Effect(EffectAttacksCap); got != 1 {
		t.Fatalf("held: %v", got)
	}
	bs.Trigger()
	if got := bs.Effect(EffectAttacksCap); got != 0 {
		t.Fatalf("expired by its own trigger, the cap must be gone: %v", got)
	}
}

func TestAddBuffMagnitudeSetsTheSnapshotForATickRecord(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 916, Name: "Venomed", TriggerRate: "1 round", TriggerCount: 4, TickPool: "health", TickFromMagnitude: true})
	bs := Buffs{}
	bs.Validate(true)
	if !bs.AddBuffMagnitude(916, 6, -5) {
		t.Fatal("add refused")
	}
	b := bs.GetBuffs(916)[0]
	if b.Magnitude != -5 || b.TickAmount != -5 || b.TriggersLeft != 6 {
		t.Fatalf("magnitude -5 for 6 rounds must become tick snapshot -5 with 6 triggers, got %+v", *b)
	}
	bs.AddBuffMagnitude(916, 3, -9)
	b = bs.GetBuffs(916)[0]
	if b.Magnitude != -9 || b.TickAmount != -9 || b.TriggersLeft != 3 {
		t.Fatalf("a re-add overwrites magnitude, snapshot and duration: %+v", *b)
	}
}

func TestAddBuffMagnitudeZeroRoundsMeansTheSpecDefault(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 920, Name: "Default", TriggerRate: "1 round", TriggerCount: 7, Effects: map[EffectKind]EffectValue{EffectDamageMult: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuffMagnitude(920, 0, 1.5)
	if got := bs.GetBuffs(920)[0].TriggersLeft; got != 7 {
		t.Fatalf("rounds 0 must take the spec's triggercount, got %d", got)
	}
}

// Exact rounds, not a multiplier: AddBuffScaled truncates float64(count) *
// mult, and 3.3 * 10 is 32.999... in binary, which would have shortened a
// 33-round ward to 32. Every former condition passes the integer it computed.
func TestAddBuffMagnitudeRoundsAreExact(t *testing.T) {
	withSpecs(t, &BuffSpec{BuffId: 921, Name: "Exact", TriggerRate: "1 round", TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: {UsesMagnitude: true}}})
	bs := Buffs{}
	bs.Validate(true)
	for _, rounds := range []int{1, 3, 33, 37, 250} {
		bs.AddBuffMagnitude(921, rounds, 1)
		if got := bs.GetBuffs(921)[0].TriggersLeft; got != rounds {
			t.Fatalf("rounds %d became %d", rounds, got)
		}
	}
}

func TestAddBuffMagnitudeRefusesPoisonUnderImmunity(t *testing.T) {
	withSpecs(t,
		&BuffSpec{BuffId: 917, Name: "Stone", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{PoisonImmunity}},
		&BuffSpec{BuffId: 918, Name: "Toxin", TriggerRate: "1 round", TriggerCount: 5, Flags: []Flag{Poison}, TickPool: "health", TickFromMagnitude: true},
	)
	bs := Buffs{}
	bs.Validate(true)
	bs.AddBuff(917, false)
	if bs.AddBuffMagnitude(918, 5, -3) {
		t.Fatal("a poison record must be refused under poison immunity")
	}
}

func TestQuietSilencesBothNotices(t *testing.T) {
	s := &BuffSpec{BuffId: 919, Name: "Off Balance", Flags: []Flag{Quiet}, StartUserText: "x", EndUserText: "y"}
	if s.StartUserNotice() != "" || s.EndUserNotice() != "" {
		t.Fatal("a quiet record sends no line at either end")
	}
}
```

Run: `go test ./internal/buffs/ -run 'TestEffect|TestValidateRefusesAnUnknownEffect|TestValidateRefusesTickFrom|TestMagnitudeZero|TestExpiredRecords|TestAddBuffMagnitude|TestQuiet' -count=1` → FAIL to compile.

- [ ] **Step 2: `ids.go`**

```go
package buffs

// Record ids the engine names in code. The YAML under
// _datafiles/world/dogmud/buffs/ is the definition; these constants exist so
// a producer or reader never spells a bare number. Slice 1 of the conditions
// unification (2026-09-12) added 117 to 123 when the ten combat conditions
// became records.
const (
	BuffIdWarcry            = 79
	BuffIdRally             = 80
	BuffIdOffBalance        = 117 // failed grapple exposure, one round
	BuffIdRecovering        = 118 // prone recovery penalty, one round
	BuffIdMinorShield       = 119
	BuffIdRegenerating      = 120
	BuffIdPoisoned          = 121 // the spell dot; Venom (39) and Spore Toxin (40) are their own records
	BuffIdBleeding          = 122
	BuffIdEnchantWithdrawal = 123
)
```

Before committing, confirm none of 117 to 123 exists: `ls _datafiles/world/dogmud/buffs | grep -E '^(117|118|119|120|121|122|123)-'` (run standalone; empty is right) and `python tools/id_inventory.py` (read its buff section).

- [ ] **Step 3: `effects.go`**

```go
package buffs

import (
	"fmt"
	"sort"
	"strconv"
)

// EffectKind is one of the closed set of mechanical effects a record may
// declare. Combat reads them through Buffs.Effect. The set is closed on
// purpose: a new kind is a code change with a reader, never a data change.
type EffectKind string

const (
	EffectDamageMult     EffectKind = "damage_mult"     // physical damage multiplier (warcry)
	EffectDefenseMult    EffectKind = "defense_mult"    // defense score multiplier (rally, grapple exposure)
	EffectDodgeMult      EffectKind = "dodge_mult"      // dodge score multiplier (no producer today; kept for parity with the reader)
	EffectRegenMult      EffectKind = "regen_mult"      // multiplier on base health regen (heal spells, corpse feeding)
	EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
	EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Buff.Source
	EffectAttacksCap     EffectKind = "attacks_cap"     // upper bound on swings per round
)

// AllEffectKinds is the closed set, for validation and docs.
var AllEffectKinds = []EffectKind{
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
}

func (k EffectKind) isMultiplier() bool {
	return k == EffectDamageMult || k == EffectDefenseMult || k == EffectDodgeMult || k == EffectRegenMult
}

func (k EffectKind) isCap() bool { return k == EffectAttacksCap }

// EffectValue is either a literal number or the word "magnitude", meaning the
// instance's own Magnitude, which the applier set.
type EffectValue struct {
	Literal       float64
	UsesMagnitude bool
}

func (v *EffectValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		if s == "magnitude" {
			*v = EffectValue{UsesMagnitude: true}
			return nil
		}
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("effect value %q is neither a number nor the word magnitude", s)
		}
		*v = EffectValue{Literal: f}
		return nil
	}
	var f float64
	if err := unmarshal(&f); err != nil {
		return fmt.Errorf("effect value must be a number or the word magnitude: %w", err)
	}
	*v = EffectValue{Literal: f}
	return nil
}

func (v EffectValue) MarshalYAML() (interface{}, error) {
	if v.UsesMagnitude {
		return "magnitude", nil
	}
	return v.Literal, nil
}

// validateEffects refuses an unknown key and a magnitude-bound tick without a
// pool. It is called from BuffSpec.Validate.
func (b *BuffSpec) validateEffects() error {
	keys := make([]string, 0, len(b.Effects))
	for k := range b.Effects {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		known := false
		for _, ak := range AllEffectKinds {
			if EffectKind(k) == ak {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("buffId %d (%s) declares unknown effect %q; see buffs.AllEffectKinds", b.BuffId, b.Name, k)
		}
	}
	if b.TickFromMagnitude {
		if b.TickPool == "" {
			return fmt.Errorf("buffId %d (%s) sets tick_from_magnitude without tick_pool", b.BuffId, b.Name)
		}
		if b.TickPercent != 0 {
			return fmt.Errorf("buffId %d (%s) sets both tick_from_magnitude and tick_percent; the applier's magnitude IS the per-round amount", b.BuffId, b.Name)
		}
	}
	return nil
}

// Effect combines every held, unexpired record's contribution for one kind:
// multipliers multiply (identity 1, a zero magnitude contributes nothing),
// flats and pool fractions sum (identity 0), attacks_cap takes the minimum
// (0 meaning no cap). This is the ONE door combat reads timed state through.
// It never calls HasFlag with expire=true, which mutates.
func (bs *Buffs) Effect(kind EffectKind) float64 {
	product := 1.0
	sum := 0.0
	capValue := 0.0
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetBuffSpec(b.BuffId)
		if spec == nil {
			continue
		}
		v, ok := spec.Effects[kind]
		if !ok {
			continue
		}
		val := v.Literal
		if v.UsesMagnitude {
			val = b.Magnitude
		}
		switch {
		case kind.isMultiplier():
			if val != 0 {
				product *= val
			}
		case kind.isCap():
			if val > 0 && (capValue == 0 || val < capValue) {
				capValue = val
			}
		default:
			sum += val
		}
	}
	switch {
	case kind.isMultiplier():
		return product
	case kind.isCap():
		return capValue
	default:
		return sum
	}
}

// HasEffect reports whether any held, unexpired record declares the kind.
func (bs *Buffs) HasEffect(kind EffectKind) bool {
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetBuffSpec(b.BuffId); spec != nil {
			if _, ok := spec.Effects[kind]; ok {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: `buffs.go`**

Add to the `Buff` struct, after `TickAmount`:

```go
	// Magnitude is the per-instance strength the applier set. A spec effect
	// whose value is the word "magnitude" reads it; a tick_from_magnitude
	// record snapshots it into TickAmount. Zero means "no effect" for a
	// multiplier and nothing for a flat; it is not a valid poison amount.
	Magnitude float64 `yaml:"magnitude,omitempty"`
```

Add after `AddBuffScaled`:

```go
// AddBuffMagnitude applies a record for an EXACT number of rounds with a
// per-instance magnitude. It is the writer door for every record that used to
// be a combat condition. rounds 0 means the spec's own triggercount. A held
// record of the same id is refreshed and its magnitude, rounds and tick
// snapshot overwritten, which is what AddCondition did. Returns false when
// refused (poison immunity) or unknown.
//
// Rounds are an int on purpose: AddBuffScaled truncates float64(count) * mult,
// and 3.3 * 10 is 32.999... in binary, so a multiplier would shorten some
// durations by a round. The former conditions all computed an integer.
func (bs *Buffs) AddBuffMagnitude(buffId int, rounds int, magnitude float64) bool {
	if !bs.AddBuffScaled(buffId, 1.0) {
		return false
	}
	idx, ok := bs.buffIds[buffId]
	if !ok {
		return false
	}
	if rounds > 0 {
		bs.List[idx].TriggersLeft = rounds
	}
	bs.List[idx].Magnitude = magnitude
	if spec := GetBuffSpec(buffId); spec != nil && spec.TickFromMagnitude {
		// The magnitude IS the signed per-round amount: negative harms.
		bs.List[idx].TickAmount = int(magnitude)
	}
	return true
}
```

Also add to `internal/buffs/test_helpers.go` the ADDITIVE seeding helper every migrated test in a package that does not load the world will call (the hooks fixture `seedAllRegistries` seeds its own map by replacement, so the records must be added on top):

```go
// SeedConditionRecordsForTest adds the condition records (79, 80, 117 to 123)
// to whatever spec map is current, with exactly the shipped mechanical shape
// (text omitted), and returns a cleanup that removes them again. Additive on
// purpose: a package fixture that already seeded its own buffs keeps them.
func SeedConditionRecordsForTest() func() {
	mag := EffectValue{UsesMagnitude: true}
	records := []*BuffSpec{
		{BuffId: BuffIdWarcry, Name: "Warcry", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDamageMult: mag}},
		{BuffId: BuffIdRally, Name: "Rally", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: mag}},
		{BuffId: BuffIdOffBalance, Name: "Off Balance", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}}},
		{BuffId: BuffIdRecovering, Name: "Recovering", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}},
		{BuffId: BuffIdMinorShield, Name: "Minor Shield", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: mag}},
		{BuffId: BuffIdRegenerating, Name: "Regenerating", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectRegenMult: mag}},
		{BuffId: BuffIdPoisoned, Name: "Poisoned", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{Poison, SilentStart}, TickPool: "health", TickFromMagnitude: true},
		{BuffId: BuffIdBleeding, Name: "Bleeding", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{Bleeding, SilentStart}, TickPool: "health", TickFromMagnitude: true},
		{BuffId: BuffIdEnchantWithdrawal, Name: "Enchant Withdrawal", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectPoolMaxPct: mag}},
	}
	if buffs == nil {
		buffs = map[int]*BuffSpec{}
	}
	replaced := map[int]*BuffSpec{}
	for _, r := range records {
		if old, ok := buffs[r.BuffId]; ok {
			replaced[r.BuffId] = old
		}
		buffs[r.BuffId] = r
	}
	return func() {
		for _, r := range records {
			delete(buffs, r.BuffId)
		}
		for id, old := range replaced {
			buffs[id] = old
		}
	}
}
```
(`RoundInterval` is what `Trigger` reads; confirm how `LoadDataFiles` derives it from `triggerrate` (`Validate` computes it) and set it explicitly here so the seeded records tick. If a shipped record's tick behaviour differs from this helper, the helper is wrong, not the record: keep them identical.)

- [ ] **Step 5: `buffspec.go`**

Add the two spec fields after `StartRemoveBuffs`:

```go
	// Effects is the closed mechanical vocabulary combat reads through
	// Buffs.Effect. See effects.go. A value is a number or the word
	// "magnitude".
	Effects map[EffectKind]EffectValue `yaml:"effects,omitempty"`
	// TickFromMagnitude marks a tick record whose per-round amount is the
	// applier's magnitude rather than tick_percent of a pool: the spell dot
	// and bleed records. Requires tick_pool; forbids tick_percent.
	TickFromMagnitude bool `yaml:"tick_from_magnitude,omitempty"`
```

Add two flags next to `Poison`/`SilentStart` and to `AllFlags`:

```go
	// Bleeding marks a bleed record: death cause reads it, as it reads Poison.
	Bleeding Flag = `bleeding`
	// Quiet marks a record that is listed but sends no start or end line,
	// because it is reapplied every round it persists (prone recovery, the
	// grapple exposure) and any line would repeat each round.
	Quiet Flag = `quiet`
```

In `Validate`, directly after the `validateNarration` call added in 5b, add:

```go
	if err := b.validateEffects(); err != nil {
		return err
	}
```

Also relax the existing `tick_pool set but tick_percent is 0` warning so it does not fire when `b.TickFromMagnitude` is true (wrap that `mudlog.Warn` in `if !b.TickFromMagnitude`).

- [ ] **Step 6: `notice.go`**

In both `StartUserNotice` and `EndUserNotice`, directly after the `Secret` check, add:

```go
	if b.hasFlag(Quiet) {
		return ""
	}
```

- [ ] **Step 7: Run, then the whole package and the guards that read flags**

Run: `go test ./internal/buffs/ -count=1` → `ok` (includes `TestAllFlagsNamesEveryDeclaredConstant`, which now sees the two new constants in `AllFlags`).
Run: `go test . -run 'TestEveryDogmudBuffFlagIsDeclared|TestEveryDogmudBuffHasAuthoredNotices' -count=1` → `ok` (no data uses the new flags yet).
Run: `go test ./internal/narration/ -count=1` → `ok`; `git status --short internal/narration/testdata/` empty (no records changed yet).

- [ ] **Step 8: Commit**

```bash
git add internal/buffs/ids.go internal/buffs/effects.go internal/buffs/effects_test.go internal/buffs/buffs.go internal/buffs/buffspec.go internal/buffs/notice.go
git commit -m "feat(buffs): per-instance magnitude, the closed effects vocabulary, the Effect door and AddBuffMagnitude"
```

---

### Task 2: The writer door at the character, the user, the mob, the event; the guards learn it

**Files:**
- Modify: `internal/characters/buffs.go`, `internal/events/eventtypes.go`, `internal/users/userrecord.go`, `internal/mobs/mobs.go`, `internal/hooks/Buff_ApplyBuffs.go`
- Modify: `buff_apply_path_guard_test.go`, `buff_notice_guard_test.go`

- [ ] **Step 1: `Character.AddBuffMagnitude`**

In `internal/characters/buffs.go` after `AddBuffScaled`:

```go
// AddBuffMagnitude applies a record synchronously with a per-instance
// magnitude. It is what every former AddCondition site calls: those effects
// must be in place within the same round tick (a shout, a ward, a bleed) and
// their appliers narrate the moment themselves, so the record is silent-start
// or quiet and the event path's start notice is not wanted. The prune pass
// still narrates the end.
func (c *Character) AddBuffMagnitude(buffId int, rounds int, magnitude float64, source string) error {
	buffId = int(math.Abs(float64(buffId)))
	if !c.Buffs.AddBuffMagnitude(buffId, rounds, magnitude) {
		return fmt.Errorf(`failed to add buff. target: "%s" buffId: %d`, c.Name, buffId)
	}
	for _, b := range c.Buffs.GetBuffs(buffId) {
		b.Source = source
	}
	c.Validate()
	return nil
}
```

(`Validate` is what applies pool-max effects, so it must run, as it does in `AddBuffScaled`.)

- [ ] **Step 2: The event and the user door**

`internal/events/eventtypes.go`, in `type Buff struct` after `DurationMult`:

```go
	// Magnitude is the per-instance strength for a record whose effects read
	// it, and Rounds the exact duration; either non-zero routes the event
	// through AddBuffMagnitude. Zero both means the DurationMult path.
	Magnitude float64
	Rounds    int
```

`internal/users/userrecord.go`, after `AddBuffScaled`:

```go
// AddBuffMagnitude queues a record with an exact duration and a per-instance
// magnitude through the event path, so the holder reads the start notice.
// Former conditions apply synchronously through Character.AddBuffMagnitude
// instead; this door is for a spell or item that wants the notice too.
func (u *UserRecord) AddBuffMagnitude(buffId int, rounds int, magnitude float64, source string) {
	events.AddToQueue(events.Buff{
		UserId:    u.UserId,
		BuffId:    buffId,
		Source:    source,
		Rounds:    rounds,
		Magnitude: magnitude,
	})
}
```

`internal/hooks/Buff_ApplyBuffs.go`: where it chooses between `AddBuffScaled` and `AddBuff` (lines ~83-88), add a first branch:

```go
	if evt.Magnitude != 0 || evt.Rounds > 0 {
		addErr = targetChar.AddBuffMagnitude(evt.BuffId, evt.Rounds, evt.Magnitude, evt.Source)
	} else if evt.DurationMult > 0 && evt.DurationMult != 1.0 {
```

No mob-level door is needed: mob producers call `mob.Character.AddBuffMagnitude` directly, as they called `AddCondition`.

- [ ] **Step 3: The apply-path guard learns the new method**

In `buff_apply_path_guard_test.go`: change `buffAddCallPattern` to

```go
var buffAddCallPattern = regexp.MustCompile(`\.(AddBuff(?:Scaled|Magnitude)?)\(`)
```

and in `isEventPathCall`, when the matched name is `AddBuffMagnitude`, return `eventPath = false, parsed = true` unconditionally: the character door and the user door have the same four-argument shape ending in a source string, so arity cannot tell them apart, and treating every call as a direct add is the safe reading. The producer sites in Tasks 4 to 10 are allowlisted with the reason "former combat condition; applies synchronously; record is silent-start or quiet". Extend the doc comment above the allowlist with one paragraph saying so. Add the compile-pinned signature lines:

```go
	_ func(int, int, float64, string) error = (*characters.Character)(nil).AddBuffMagnitude
	_ func(int, int, float64, string)       = (*users.UserRecord)(nil).AddBuffMagnitude
```

- [ ] **Step 4: The notice guard learns `quiet`**

In `buff_notice_guard_test.go`, next to `silentStart` and `hidden`:

```go
		quiet := slices.Contains(b.Flags, "quiet")
```
and make both text checks `&& !silentStart && !quiet` / `&& !hidden && !quiet`. Add a sentence to the file's header comment: "quiet: listed but never announced, for a record reapplied every round it persists."

- [ ] **Step 5: Verify and commit**

Run: `go build ./... && go vet ./internal/characters/ ./internal/users/ ./internal/hooks/ ./internal/events/` → clean.
Run: `go test ./internal/characters/ ./internal/users/ ./internal/hooks/ ./internal/buffs/ -count=1` → all `ok`.
Run: `go test . -run 'TestPlayerBuffsTravelTheEventPath|TestEveryDogmudBuffHasAuthoredNotices' -count=1` → `ok` (no producer uses the method yet).

```bash
git add internal/characters/buffs.go internal/events/eventtypes.go internal/users/userrecord.go internal/hooks/Buff_ApplyBuffs.go buff_apply_path_guard_test.go buff_notice_guard_test.go
git commit -m "feat(conditions): the AddBuffMagnitude writer door at the character, the user and the event; guards learn it and quiet"
```

---

### Task 3: The records, the tick path's cancel helpers, and the one golden re-record

**Files:**
- Modify: `_datafiles/world/dogmud/buffs/79-warcry.yaml`, `80-rally.yaml`
- Create: `117-off_balance.yaml`, `118-recovering.yaml`, `119-minor_shield.yaml`, `120-regenerating.yaml`, `121-poisoned.yaml`, `122-bleeding.yaml`, `123-enchant_withdrawal.yaml` under `_datafiles/world/dogmud/buffs/`
- Modify: `internal/hooks/NewRound_UserRoundTick.go`, `internal/hooks/NewRound_MobRoundTick.go` (damaging ticks call the cancel helpers)
- Modify: `internal/narration/testdata/stores/buffs.golden` (the one re-record)

The filename must match the `name` field through `util.ConvertForFilename` (a mismatch panics at boot): lowercase, spaces to underscores. Lines wrap at 80. No em or en dashes. No raw numbers in player text. Write the files with LF endings (the loader accepts either; keep new files consistent with the Go tree).

- [ ] **Step 1: The two shouts**

`79-warcry.yaml`: remove `  - condition-mirror` and add, after `triggercount: 25`:
```yaml
effects:
  damage_mult: magnitude
```
`80-rally.yaml`: remove `  - condition-mirror` and add:
```yaml
effects:
  defense_mult: magnitude
```
(the applier sets the magnitude to 1 + bonus, Task 4.)

- [ ] **Step 2: The seven new records**

`117-off_balance.yaml`:
```yaml
buffid: 117
name: Off Balance
description: A failed grapple left you exposed; your defenses are weakened this round.
triggerrate: 1 round
triggercount: 1
effects:
  defense_mult: 0.85
flags:
  - quiet
```

`118-recovering.yaml`:
```yaml
buffid: 118
name: Recovering
description: Still finding your feet; you can manage only a single attack this round.
triggerrate: 1 round
triggercount: 1
effects:
  attacks_cap: 1
flags:
  - quiet
```

`119-minor_shield.yaml`:
```yaml
buffid: 119
name: Minor Shield
description: A magical barrier turns aside some of the blows that reach you.
triggerrate: 1 round
triggercount: 10
end_user_text: "Your Minor Shield dissipates."
end_room_text: "{source}'s Minor Shield dissipates."
effects:
  mitigation_flat: magnitude
flags:
  - silent-start
```
(`triggercount` is only the default for a rounds of 0; the ward spell passes its computed `duration` as exact rounds. See Task 6.)

`120-regenerating.yaml`:
```yaml
buffid: 120
name: Regenerating
description: Healing magic is knitting your wounds closed a little faster each round.
triggerrate: 1 round
triggercount: 10
end_user_text: "The healing magic in your wounds runs its course."
effects:
  regen_mult: magnitude
flags:
  - silent-start
```

`121-poisoned.yaml`:
```yaml
buffid: 121
name: Poisoned
description: Toxins coursing through your body, dealing damage over time.
triggerrate: 1 round
triggercount: 10
tick_pool: health
tick_from_magnitude: true
trigger_user_text: '<ansi fg="green">The poison burns through your veins!</ansi>'
end_user_text: "The poison in your veins finally burns itself out."
flags:
  - poison
  - silent-start
```

`122-bleeding.yaml`:
```yaml
buffid: 122
name: Bleeding
description: Wounds seeping blood, taking damage over time.
triggerrate: 1 round
triggercount: 10
tick_pool: health
tick_from_magnitude: true
trigger_user_text: '<ansi fg="red">Blood seeps from your wounds!</ansi>'
end_user_text: "Your wounds stop bleeding."
flags:
  - bleeding
  - silent-start
```

`123-enchant_withdrawal.yaml`:
```yaml
buffid: 123
name: Enchant Withdrawal
description: Weakened from severing a Chrysalis bond; one of your reserves runs shallow.
triggerrate: 1 round
triggercount: 10
start_user_text: "The severed bond leaves a hollow in you that will take time to fill."
end_user_text: "The hollow the severed bond left in you has finally closed."
effects:
  pool_max_pct: magnitude
```

Durations: every applier passes the exact integer of rounds it computed today as `AddBuffMagnitude`'s `rounds`; the record's `triggercount` is only the default for a rounds of 0. No multiplier arithmetic anywhere (`TestAddBuffMagnitudeRoundsAreExact` in Task 1 is why).

- [ ] **Step 3: The tick path calls the cancel helpers on a damaging health tick**

In `internal/hooks/NewRound_UserRoundTick.go`, in the tick application `case "health":` branch, after `user.Character.ApplyHarm(characters.PoolHealth, -tickAmt, state.ActorRef{})` add:

```go
									// Damage is damage: wake a sleeper and drop
									// cancel-on-damage records, as the poison and
									// bleed hook always did (slice 1, change 5).
									cancelCraftOrSalvageOnDamage(user.Character)
									cancelDamageBuffs(user.Character)
```
In `NewRound_MobRoundTick.go`, the mirror branch after the mob's `ApplyHarm(characters.PoolHealth, ...)`:
```go
						cancelCraftOrSalvageOnDamage(&mob.Character)
						cancelDamageBuffs(&mob.Character)
```

- [ ] **Step 4: Boot-load the records through the real loader, then re-record the golden ONCE**

Run: `go test ./internal/buffs/ -count=1` and `go test . -run 'TestEveryDogmudBuffFlagIsDeclared|TestEveryDogmudBuffHasAuthoredNotices' -count=1` → `ok` (the notice guard accepts `quiet` and `silent-start` records; 121 and 122 carry end lines; 123 both).
Run: `go test ./internal/narration/ -run TestSnapshotStores -count=1` → FAIL on `buffs.golden` (new rows). Confirm the failure names a `buff|117|...` or `buff|79|...` row, nothing else.
Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1` → `ok`. Then `git diff --stat internal/narration/testdata/stores/` must list ONLY `buffs.golden`, and `git diff internal/narration/testdata/stores/buffs.golden` must show only ADDED rows for 117 to 123 (79 and 80 gained no text, so their rows are unchanged). Paste the diff stat in the report. This is the last `-update` in this plan.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/buffs/79-warcry.yaml _datafiles/world/dogmud/buffs/80-rally.yaml _datafiles/world/dogmud/buffs/117-off_balance.yaml _datafiles/world/dogmud/buffs/118-recovering.yaml _datafiles/world/dogmud/buffs/119-minor_shield.yaml _datafiles/world/dogmud/buffs/120-regenerating.yaml _datafiles/world/dogmud/buffs/121-poisoned.yaml _datafiles/world/dogmud/buffs/122-bleeding.yaml _datafiles/world/dogmud/buffs/123-enchant_withdrawal.yaml internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewRound_MobRoundTick.go internal/narration/testdata/stores/buffs.golden
git commit -m "feat(conditions): the seven condition records and the shouts' effects; damaging ticks cancel like the poison hook did"
```

---

### Task 4: Warcry and rally become one record each

**Files:**
- Modify: `internal/actions/combat_warcry.go:117-118`, `internal/actions/combat_rally.go:115-116`, `internal/usercommands/warcry.go:56-57,90-91,120-121`, `internal/usercommands/rally.go:56-57,86-87,116-117`
- Modify: `internal/combat/combat_helpers.go:491-494,745-747`
- Modify: `internal/actions/rhetoric_progression_test.go`; `buff_apply_path_guard_test.go` allowlist

- [ ] **Step 1: The two appliers**

`combat_warcry.go:117-118`, replace
```go
	char.AddCondition(characters.ConditionWarcry, duration, bonus, "warcry")
	char.AddBuff(79, false)
```
with
```go
	// One record carries both the bookkeeping and the magnitude; the damage
	// multiplier the reader wants is 1 + bonus. duration is the exact integer
	// the shout computed (25 scaled by shout amp).
	_ = char.AddBuffMagnitude(buffs.BuffIdWarcry, duration, 1.0+bonus, "warcry")
```
`combat_rally.go:115-116` likewise with `buffs.BuffIdRally` and `"rally"`. Add the `buffs` import to both.

`usercommands/warcry.go`: line 56-57 (party member) becomes
```go
					_ = memberUser.Character.AddBuffMagnitude(buffs.BuffIdWarcry, result.Duration, 1.0+result.Bonus, "warcry")
```
Lines 90-91 (rally stacked on warcry) → `buffs.BuffIdRally`, `rd`, `1.0+rb`, `"rally"`. Lines 120-121 (companions) → `buffs.BuffIdWarcry`, `duration`, `1.0+bonus`. `usercommands/rally.go` the mirror three sites. Remove the `characters` import from any file where it becomes unused.

- [ ] **Step 2: A test that the record carries the shout's own numbers**

Add to `internal/actions/rhetoric_progression_test.go` (or a new `shout_record_test.go` in that package) one test: build a character with rhetoric 30 and charisma 100 the way that file's fixtures do, call `ApplyWarcryEffect(char)`, and assert `char.Buffs.TriggersLeft(buffs.BuffIdWarcry) == 25`, `char.Buffs.Effect(buffs.EffectDamageMult) == 1.0 + bonus` where `bonus` is the function's returned value, and that `bonus` is within [0.05, 0.20]. Mirror it for rally and `EffectDefenseMult`.

- [ ] **Step 3: The two readers**

`internal/combat/combat_helpers.go:491-494`:
```go
	// Warcry record: the damage multiplier is the record's magnitude (1 + bonus).
	if warcryMult := sourceChar.Buffs.Effect(buffs.EffectDamageMult); warcryMult != 1.0 {
		dmgMean *= warcryMult
		rawDmgForCrit *= warcryMult
	}
```
`:745-747`:
```go
		// Rally record: defense score multiplier from the rhetoric shout. The
		// same door folds the grapple exposure (Task 5) and any other defense
		// multiplier.
		defenseScore *= targetChar.Buffs.Effect(buffs.EffectDefenseMult)
```
🔴 Leave `:755-757` (the `ConditionDefensePenalty` block) in place until Task 5, or the exposure would apply zero times; Task 5 deletes it. Because the door now multiplies rally and exposure together at line 746, and 755 still multiplies exposure, exposure would apply TWICE between Task 4 and Task 5 only if a record with `defense_mult` existed; none does until Task 5 migrates the producer. State this in the commit message.

- [ ] **Step 4: Tests and the allowlist**

`rhetoric_progression_test.go`: the fixture's `selfCondition characters.ConditionType` field and the `HasCondition(result.selfCondition)` assertions (lines 37, 65, 74, 178, 227, 258, 499) become assertions on the record: replace the field with `selfBuffID` (already present) and each `HasCondition(result.selfCondition)` with `char.HasBuff(result.selfBuffID)`; where the test asserts the condition is absent, assert `!char.HasBuff(...)`. Add one assertion where it asserts presence: `require.Equal(t, 1.0+expectedBonus, char.Buffs.Effect(buffs.EffectDamageMult))` for warcry (compute `expectedBonus` the way `ApplyWarcryEffect` does, or read `char.GetBuffs(79)[0].Magnitude` and assert it is within [1.05, 1.20]).

Run `go test . -run TestPlayerBuffsTravelTheEventPath -count=1`: it now reports the six `AddBuffMagnitude` sites (and the two in actions). Add each to `buffApplyPathAllowlist` keyed `file|line` with the reason `"former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it"`.

- [ ] **Step 5: Verify and commit**

Run: `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/combat/ ./internal/characters/ -count=1` → `ok`.
Run: `go test . -run 'TestPlayerBuffsTravelTheEventPath|TestEveryDogmudBuffHasAuthoredNotices' -count=1` → `ok`.
Run: `go test ./internal/narration/ -count=1` → `ok`, testdata untouched.
Grep standalone: `grep -rn "ConditionWarcry\|ConditionRally\|AddBuff(79\|AddBuff(80" --include=*.go internal/ modules/ | grep -v _test.go` → only `conditions.go` (deleted in Task 11).

```bash
git add internal/actions/combat_warcry.go internal/actions/combat_rally.go internal/usercommands/warcry.go internal/usercommands/rally.go internal/combat/combat_helpers.go internal/actions/rhetoric_progression_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): warcry and rally are one record each; damage and defense read the door"
```

---

### Task 5: Grapple exposure and prone recovery

**Files:**
- Modify: `internal/combat/grapple_move.go:56`, `internal/characters/skills.go:73,96,100`, `internal/combat/combat_helpers.go:232,755-757`
- Modify: `internal/combat/hitroll_test.go:129`; `buff_apply_path_guard_test.go`

- [ ] **Step 1: Producers**

`grapple_move.go:56`: `attacker.AddCondition(characters.ConditionDefensePenalty, 1, 0.85, "failed grapple")` → `_ = attacker.AddBuffMagnitude(buffs.BuffIdOffBalance, 1, 0, "failed grapple")` (one round; the record's literal 0.85 is the effect; magnitude unused).
`skills.go:73,96,100`: `c.AddCondition(ConditionRecoveryPenalty, 1, 1.0, "prone recovery")` → `_ = c.AddBuffMagnitude(buffs.BuffIdRecovering, 1, 0, "prone recovery")` (`characters` already imports `buffs`).

- [ ] **Step 2: Readers**

`combat_helpers.go:232`:
```go
	// Recovering record: caps swings (1 today; the record's literal).
	if cap := sourceChar.Buffs.Effect(buffs.EffectAttacksCap); cap > 0 && result > int(cap) {
		result = int(cap)
	}
```
`combat_helpers.go:755-757`: delete the `ConditionDefensePenalty` block; Task 4's door line already multiplies the 0.85.

- [ ] **Step 3: The faithful-inertness test, and the pin**

The spec records that the recovery penalty never reaches combat today because the round tick removes it before `DoCombat`. The record reproduces that: `AddBuffMagnitude(118)` gives `TriggersLeft` 1, and the same tick's `Buffs.Trigger()` expires it. Add to `internal/combat/hitroll_test.go` next to line 129's test a sibling:

```go
// Faithful to the enum (spec finding): a Recovering record applied and then
// ticked in the same round contributes nothing to the swing count. Making the
// cap bite is a filed owner call, not this slice.
func TestRecoveringRecordExpiredByItsOwnTickCapsNothing(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()
	ch := &characters.Character{}
	ch.Stats.Dexterity.ValueAdj = 200
	ch.StaminaMax.Value = 100
	ch.Stamina = 100
	setCombatPositionParallel(ch, position.Standing)
	ch.Buffs.Validate(true)
	_ = ch.AddBuffMagnitude(buffs.BuffIdRecovering, 1, 0, "test")
	assert.Equal(t, 1, calcSwingCount(ch, items.Item{}, 1.4, 0, false), "held: the cap applies")
	ch.Buffs.Trigger()
	assert.Greater(t, calcSwingCount(ch, items.Item{}, 1.4, 0, false), 1, "expired by its own tick: no cap")
}
```
and change `TestCalcSwingCount_RecoveryForcesOne` (line 123-133): add `defer buffs.SeedConditionRecordsForTest()()` and `ch.Buffs.Validate(true)` after the fixture lines, and replace `ch.AddCondition(characters.ConditionRecoveryPenalty, 1, 1.0, "test")` with `_ = ch.AddBuffMagnitude(buffs.BuffIdRecovering, 1, 0, "test")`; its assertion (`1`) stays.

- [ ] **Step 4: Allowlist, verify, commit**

Allowlist the four new sites with `"former combat condition (one-round penalty): quiet record; must apply synchronously inside the round tick"`.

Run: `go build ./... && go test ./internal/combat/ ./internal/characters/ ./internal/hooks/ -count=1 && go test . -run 'TestPlayerBuffsTravelTheEventPath' -count=1` (run the last standalone if the chain stops) → all `ok`.
Grep standalone: `grep -rn "ConditionDefensePenalty\|ConditionRecoveryPenalty" --include=*.go internal/ modules/ | grep -v _test.go` → only `conditions.go`.

```bash
git add internal/combat/grapple_move.go internal/characters/skills.go internal/combat/combat_helpers.go internal/combat/hitroll_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): grapple exposure and prone recovery are quiet one-round records"
```

---

### Task 6: Minor Shield

**Files:**
- Modify: `internal/hooks/spell_resolution.go:1165,1521`, `internal/characters/combat.go:185`, `internal/combat/ai.go:741`, `internal/behaviortree/action_cast_best_in_category.go:224`
- Modify: `internal/hooks/NewRound_DoCombat.go:120,308-316`, `internal/hooks/NewRound_DoCombat_helpers.go:507-517` (delete the decay helpers)
- Modify tests: `internal/hooks/hooks_test.go:1085-1112,1668-1680`, `internal/combat/combat_helpers_test.go:126`, `internal/hooks/NewRound_DoCombat_parity_test.go:263`, `internal/behaviortree/pure_caster_archetype_integration_test.go:162,194`, `internal/characters/conditions_pin_test.go`

- [ ] **Step 1: Producers**

`spell_resolution.go:1165` (player target): `target.Character.AddCondition(characters.ConditionShield, duration, float64(shieldBonus), "spell")` → `_ = target.Character.AddBuffMagnitude(buffs.BuffIdMinorShield, duration, float64(shieldBonus), "spell")`. `:1521` (mob target) likewise. `buffs` is already imported in this file.

- [ ] **Step 2: Readers**

`characters/combat.go:185`: `nonGearMit := int(c.GetConditionMagnitude(ConditionShield))` → `nonGearMit := int(c.Buffs.Effect(buffs.EffectMitigationFlat))`.
`combat/ai.go:741`: `!mob.Character.HasCondition(characters.ConditionShield)` → `!mob.Character.Buffs.HasEffect(buffs.EffectMitigationFlat)`.
`behaviortree/action_cast_best_in_category.go:224`: `char.HasCondition(characters.ConditionShield)` → `char.Buffs.HasEffect(buffs.EffectMitigationFlat)`; update its doc comment (lines 210-215) to name the record and the door.

- [ ] **Step 3: Delete the double decay**

Delete `handlePlayerShieldDecay` (`NewRound_DoCombat_helpers.go:507-517`) and its call at `NewRound_DoCombat.go:120`; delete the mob branch at `NewRound_DoCombat.go:308-316` (from `// Mob shield decay` through the closing brace of that `if`). The record now decays once per round through `Buffs.Trigger` and narrates its end through the prune pass (`end_user_text` and `end_room_text` in the record).

- [ ] **Step 4: Tests**

Every migrated test in a package whose fixture does not load the world's buff files (all of them: `seedAllRegistries` in hooks seeds its own map, `&characters.Character{}` fixtures seed nothing) adds `defer buffs.SeedConditionRecordsForTest()()` AFTER its own fixture setup (it is additive), and `c.Buffs.Validate(true)` on a bare character before the first add.

- `hooks_test.go:1085-1112`: the three `TestHandlePlayerShieldDecay_*` tests are deleted (the helper is gone). Replace with one test that a Minor Shield record held for 1 round by user 1 (`_ = u.Character.AddBuffMagnitude(buffs.BuffIdMinorShield, 1, 10.0, "test")`) is gone after `UserRoundTick` then `PruneBuffs` run (read how `buff_room_text_test.go`'s expiry tests fire the round and the turn) and that the user read "Your Minor Shield dissipates." (`drainPlain(1)` contains it).
- `hooks_test.go:1668-1680` `TestMobRoundTick_TickConditions`: setup becomes `_ = mob.Character.AddBuffMagnitude(buffs.BuffIdMinorShield, 3, 10.0, "test")`, assertion becomes `assert.True(t, mob.Character.HasBuff(buffs.BuffIdMinorShield))`.
- `combat_helpers_test.go:126`: `c.AddCondition(characters.ConditionShield, 100, float64(mitigationPct), "test")` → `_ = c.AddBuffMagnitude(buffs.BuffIdMinorShield, 100, float64(mitigationPct), "test")`.
- `NewRound_DoCombat_parity_test.go:263`: `defUser.Character.AddCondition(characters.ConditionShield, 10, magnitude, "test")` → `_ = defUser.Character.AddBuffMagnitude(buffs.BuffIdMinorShield, 10, magnitude, "test")`; update the comment at 226-262 to say "Minor Shield record".
- `pure_caster_archetype_integration_test.go:162,194`: `mob.Character.AddCondition(characters.ConditionShield, 20, 75, "test")` → `_ = mob.Character.AddBuffMagnitude(buffs.BuffIdMinorShield, 20, 75, "test")`.
- `conditions_pin_test.go` `TestPin_ShieldAddsFlatPhysicalMitigation`: setup → `_ = c.AddBuffMagnitude(buffs.BuffIdMinorShield, 10, 12, "pin")`; the literal `0.12` stays.

- [ ] **Step 5: Allowlist, verify, commit**

Allowlist the two producer sites: `"former combat condition (ward): silent-start record, the spell narrates; must apply synchronously so the same resolution pass sees it"`.

Run: `go build ./... && go test ./internal/hooks/ ./internal/characters/ ./internal/combat/ ./internal/behaviortree/ -count=1` → `ok` (run separately if the chain stops). Guard: `go test . -run TestPlayerBuffsTravelTheEventPath -count=1`.
Grep standalone: `grep -rn "ConditionShield\|handlePlayerShieldDecay" --include=*.go internal/ modules/ | grep -v _test.go` → only `conditions.go`.

```bash
git add internal/hooks/spell_resolution.go internal/characters/combat.go internal/combat/ai.go internal/behaviortree/action_cast_best_in_category.go internal/hooks/NewRound_DoCombat.go internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/hooks_test.go internal/combat/combat_helpers_test.go internal/hooks/NewRound_DoCombat_parity_test.go internal/behaviortree/pure_caster_archetype_integration_test.go internal/characters/conditions_pin_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): Minor Shield is a record; it decays once per round and narrates its end everywhere"
```

---

### Task 7: Regenerating

**Files:**
- Modify: `internal/hooks/spell_resolution.go:841,1062,1482`, `internal/mobcommands/consume.go:46,55`
- Modify: `internal/hooks/NewRound_AutoHeal.go:191-196,206-216,335-340,349-354`
- Modify tests: `internal/mobcommands/predator_test.go:78,103-105,135,157`

- [ ] **Step 1: Producers**

Each `AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")` → `_ = X.Character.AddBuffMagnitude(buffs.BuffIdRegenerating, durationRounds, regenMult, "heal spell")`. `consume.go:46`: `_ = mob.Character.AddBuffMagnitude(buffs.BuffIdRegenerating, 10, 3.0, "grafted corpse")`; `:55`: `_ = mob.Character.AddBuffMagnitude(buffs.BuffIdRegenerating, 6, 2.0, "consumed corpse")`.

- [ ] **Step 2: The regen hook, four branches, same arithmetic**

Player out of combat (`:191-196`):
```go
				// Regenerating record from a heal spell: multiplier on base regen
				if regenMult := user.Character.Buffs.Effect(buffs.EffectRegenMult); regenMult > 1.0 {
					healthRegen *= regenMult
				}
```
The tick-feedback line (`:206-210`) gates on `user.Character.Buffs.HasEffect(buffs.EffectRegenMult)`.
Player in combat (`:212-216`): `if user.Character.Buffs.HasEffect(buffs.EffectRegenMult) { regenMult := user.Character.Buffs.Effect(buffs.EffectRegenMult); ... }` with the same formula.
Mob branches (`:335-340`, `:349-354`) likewise.

- [ ] **Step 3: Tests**

`predator_test.go`: `HasCondition(characters.ConditionRegen)` → `HasBuff(buffs.BuffIdRegenerating)`; `GetConditionMagnitude(...)` → `mob.Character.Buffs.Effect(buffs.EffectRegenMult)` asserting `2.0`; `GetConditionDuration(...)` asserting `6` → `mob.Character.Buffs.TriggersLeft(buffs.BuffIdRegenerating)` asserting `6`.

- [ ] **Step 4: Allowlist, verify, commit**

Allowlist the five sites: `"former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously"`.
Run: `go build ./... && go test ./internal/hooks/ ./internal/mobcommands/ -count=1` → `ok`; the guard; grep standalone for `ConditionRegen` → only `conditions.go`.

```bash
git add internal/hooks/spell_resolution.go internal/mobcommands/consume.go internal/hooks/NewRound_AutoHeal.go internal/mobcommands/predator_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): Regenerating is a record; the regen hook reads the door"
```

---

### Task 8: Poisoned (the spell dot)

**Files:**
- Modify: `internal/hooks/spell_resolution.go:637,1653`, `internal/hooks/NewRound_AutoHeal.go:229-239,399-407` (delete the poison blocks), `internal/hooks/Death_PlayerAnnouncement.go:126`, `internal/hooks/spell_purgeaffliction.go:102`
- Modify tests: `internal/hooks/hooks_test.go:909-921,2407`, `internal/hooks/purge_target_test.go:55,68`, `internal/hooks/selfcast_wording_test.go:31`, `internal/hooks/buff_notice_test.go:151-175`, `internal/hooks/conditions_pin_test.go`, `internal/characters/conditions_immunity_test.go` (DELETED; its case moves to `internal/buffs/effects_test.go` `TestAddBuffMagnitudeRefusesPoisonUnderImmunity`, already present)

- [ ] **Step 1: Producers**

`spell_resolution.go:637`: `afflicted := mob.Character.AddCondition(characters.ConditionPoisoned, dotDuration, float64(magnitude), "spell")` → 
```go
	dotAmount := magnitude
	if dotAmount < 1 {
		dotAmount = 1
	}
	afflicted := mob.Character.AddBuffMagnitude(buffs.BuffIdPoisoned, dotDuration, -float64(dotAmount), "spell") == nil
```
(the hook applied `int(magnitude)` min 1 per round; the record's negative snapshot is the harm). `:1653` the player-target mirror: `if target.Character.AddBuffMagnitude(...) == nil {`.

- [ ] **Step 2: Delete the hook's poison blocks; flag tests elsewhere**

`NewRound_AutoHeal.go:229-239` and `:399-407`: delete both `ConditionPoisoned` blocks (the record ticks in the round tick, which now calls the same two cancel helpers; the line is `trigger_user_text`).
`Death_PlayerAnnouncement.go:126`: `c.HasCondition(characters.ConditionPoisoned)` → `c.HasBuffFlag(buffs.Poison)`.
`spell_purgeaffliction.go:102`: delete `target.char.RemoveCondition(characters.ConditionPoisoned)`; the `CancelBuffsWithFlag(buffs.Poison)` on the line above already clears the record.

- [ ] **Step 3: Tests**

- `hooks_test.go:909-921` `TestAutoHeal_PoisonDamage`: becomes `TestRoundTick_PoisonDamage`: after `seedAllRegistries()` add `defer buffs.SeedConditionRecordsForTest()()`; setup `_ = u1.Character.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "test")`, fire `UserRoundTick(evt)` instead of `AutoHeal`, assert `u1.Character.Health == 75` (no regen runs in the round tick, so the number is exact now; the old test could only assert "changed").
- `:2407` and `selfcast_wording_test.go:31`: setup → `AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "test")` with the seeding helper.
- `purge_target_test.go:55,68`: setup → the record; assertion → `!mob.Character.HasBuff(buffs.BuffIdPoisoned)`.
- `buff_notice_test.go:151-175`: rename to the record, assert `!target.Character.HasBuff(buffs.BuffIdPoisoned)`.
- `conditions_pin_test.go`: setup → the record with `-5`; fire `UserRoundTick` instead of `AutoHeal`; `HasCondition` assertion → `u.Character.HasBuffFlag(buffs.Poison)`; the death-cause assertion stays.
- Delete `internal/characters/conditions_immunity_test.go`.

- [ ] **Step 4: Allowlist, verify, commit**

Allowlist the two producer sites: `"former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator"`.
Run: `go build ./... && go test ./internal/hooks/ ./internal/characters/ -count=1` → `ok`; the guard; grep standalone for `ConditionPoisoned` → only `conditions.go`.

```bash
git add internal/hooks/spell_resolution.go internal/hooks/NewRound_AutoHeal.go internal/hooks/Death_PlayerAnnouncement.go internal/hooks/spell_purgeaffliction.go internal/hooks/hooks_test.go internal/hooks/purge_target_test.go internal/hooks/selfcast_wording_test.go internal/hooks/buff_notice_test.go internal/hooks/conditions_pin_test.go buff_apply_path_guard_test.go
git rm -q internal/characters/conditions_immunity_test.go
git commit -m "refactor(conditions): the spell dot is a Poisoned tick record; death cause and purge read the flag"
```

---

### Task 9: Bleeding

**Files:**
- Modify: `internal/actions/combat_drain.go:146,316`, `combat_hamstring.go:134`, `combat_maul.go:131`, `combat_rake.go:131`, `combat_throttle.go:143`, `internal/hooks/item_procs.go:213`
- Modify: `internal/hooks/NewRound_AutoHeal.go` (delete both bleed blocks), `internal/hooks/Death_PlayerAnnouncement.go:128`
- Modify tests: `internal/hooks/predator_hooks_test.go`, `internal/actions/combat_drain_test.go`, `combat_throttle_test.go:154`, `internal/hooks/item_procs_test.go:247-253,287`, `internal/hooks/spell_drainarea_test.go:133,146,167`, `internal/actions/command_readiness_drift_test.go:169-192`

- [ ] **Step 1: Producers**

Every `X.AddCondition(characters.ConditionBleeding, rounds, float64(mag), "<move>")` → `_ = X.AddBuffMagnitude(buffs.BuffIdBleeding, rounds, -float64(mag), "<move>")`, keeping each move's `rounds` literal and `mag` expression exactly (drain 4 and strength/12 min 2, both sites; hamstring 5 and `bleedDmg`; maul 5 and strength/8 min 3; rake 4 and `bleedDmg`; throttle 3 and strength/10 min 2). `item_procs.go:213`: `target.AddCondition(characters.ConditionBleeding, dur, mag, "itemproc")` → `_ = target.AddBuffMagnitude(buffs.BuffIdBleeding, dur, -mag, "itemproc")` (`dur` is already an int there; if it is a float64 from `params`, wrap it `int(dur)` as the old call effectively did).

- [ ] **Step 2: Readers**

Delete both `ConditionBleeding` blocks in `NewRound_AutoHeal.go`. `Death_PlayerAnnouncement.go:128`: `HasCondition(characters.ConditionBleeding)` → `HasBuffFlag(buffs.Bleeding)`.

- [ ] **Step 3: Tests**

- `predator_hooks_test.go` (8 sites): `AddCondition(characters.ConditionBleeding, 3, 5.0, "test")` → `_ = X.AddBuffMagnitude(buffs.BuffIdBleeding, 3, -5, "test")` (the 50.0 and 0.5 variants likewise, negated; note `-0.5` truncates to a tick of 0, exactly as `int(0.5)` gave 0 then the min-1 floor applied in the hook: read that test's assertion and keep the number, applying the floor in the producer if the test proves the hook's floor mattered); `RemoveCondition(ConditionBleeding)` → `X.RemoveBuff(buffs.BuffIdBleeding)`. Read what each test asserts (a bleed tick amount? then the assertion moves from `AutoHeal` to `UserRoundTick`/`MobRoundTick`; keep the number). Each test gets the seeding helper.
- `combat_drain_test.go`, `combat_throttle_test.go`, `spell_drainarea_test.go`: `HasCondition(ConditionBleeding)` → `HasBuff(buffs.BuffIdBleeding)`; `RemoveCondition` → `RemoveBuff`.
- `item_procs_test.go:247-253`: `HasCondition` → `HasBuff`; `GetConditionDuration == 6` → `target.Buffs.TriggersLeft(buffs.BuffIdBleeding) == 6`; `GetConditionMagnitude == 12` → `target.GetBuffs(buffs.BuffIdBleeding)[0].Magnitude == -12`.
- `command_readiness_drift_test.go:169-192`: the expected call-sequence strings name `target.Char.AddCondition`; change each to `target.Char.AddBuffMagnitude` (read how the test derives the sequence; it may parse source, in which case the new spelling is what it finds).

- [ ] **Step 4: Allowlist, verify, commit**

Allowlist the seven producer sites: `"former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution"`.
Run: `go build ./... && go test ./internal/actions/ ./internal/hooks/ ./internal/mobcommands/ -count=1` → `ok`; the guard; grep standalone for `ConditionBleeding` → only `conditions.go`.

```bash
git add internal/actions/combat_drain.go internal/actions/combat_hamstring.go internal/actions/combat_maul.go internal/actions/combat_rake.go internal/actions/combat_throttle.go internal/hooks/item_procs.go internal/hooks/NewRound_AutoHeal.go internal/hooks/Death_PlayerAnnouncement.go internal/hooks/predator_hooks_test.go internal/actions/combat_drain_test.go internal/actions/combat_throttle_test.go internal/hooks/item_procs_test.go internal/hooks/spell_drainarea_test.go internal/actions/command_readiness_drift_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): Bleeding is a tick record carrying the new bleeding flag"
```

---

### Task 10: Enchant withdrawal

**Files:**
- Modify: `internal/usercommands/skill.disenchant.go:62-72`, `internal/characters/validate.go:186-221`
- Modify: `internal/characters/conditions_pin_test.go`

- [ ] **Step 1: Producer**

`skill.disenchant.go:65-70`: → `_ = user.Character.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, penaltyRounds, reservePct, reservePool)` (the pool name rides on `Source`, exactly as the condition's `Source` did).

- [ ] **Step 2: Reader**

`validate.go:186-221` becomes:

```go
	// Enchant withdrawal records: each takes a fraction off the maximum of the
	// pool named by its Source, after the reservation clamp above. Unchanged
	// arithmetic from the condition it replaces.
	for _, b := range c.Buffs.List {
		if b.Expired() {
			continue
		}
		spec := buffs.GetBuffSpec(b.BuffId)
		if spec == nil {
			continue
		}
		v, ok := spec.Effects[buffs.EffectPoolMaxPct]
		if !ok {
			continue
		}
		mag := v.Literal
		if v.UsesMagnitude {
			mag = b.Magnitude
		}
		switch b.Source {
		case "health":
			penalty := int(math.Floor(float64(c.HealthMax.Value) * mag))
			c.HealthMax.Value -= penalty
			if c.HealthMax.Value < 1 {
				c.HealthMax.Value = 1
			}
			if c.Health > c.HealthMax.Value {
				c.Health = c.HealthMax.Value
			}
		case "stamina":
			penalty := int(math.Floor(float64(c.StaminaMax.Value) * mag))
			c.StaminaMax.Value -= penalty
			if c.StaminaMax.Value < 0 {
				c.StaminaMax.Value = 0
			}
			if c.Stamina > c.StaminaMax.Value {
				c.Stamina = c.StaminaMax.Value
			}
		case "conviction":
			penalty := int(math.Floor(float64(c.ConvictionMax.Value) * mag))
			c.ConvictionMax.Value -= penalty
			if c.ConvictionMax.Value < 0 {
				c.ConvictionMax.Value = 0
			}
			if c.Conviction > c.ConvictionMax.Value {
				c.Conviction = c.ConvictionMax.Value
			}
		}
	}
```
(The old loop applied the first matching condition then `break`; only one withdrawal can be held per id, so iterating records is equivalent. `Buffs.List` is exported; if it is not, add a `ForEachHeld(func(*Buff, *BuffSpec))` to `buffs` and use it.)

- [ ] **Step 3: Pins and the allowlist**

`conditions_pin_test.go` withdrawal setups → `_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.25, "health")` etc., with `defer buffs.SeedConditionRecordsForTest()()` and `c.Buffs.Validate(true)`. Literals 150, 100, 50, 40 stay.
Allowlist the site: `"former combat condition (withdrawal): the disenchant command narrates; must apply synchronously so Validate clamps the pool now"`.

Run: `go build ./... && go test ./internal/characters/ ./internal/usercommands/ -count=1` → `ok`; the guard; grep standalone for `ConditionEnchantWithdrawal` → only `conditions.go`.

```bash
git add internal/usercommands/skill.disenchant.go internal/characters/validate.go internal/characters/conditions_pin_test.go buff_apply_path_guard_test.go
git commit -m "refactor(conditions): enchant withdrawal is a record; Validate reads pool_max_pct"
```

---

### Task 11: Delete the enum and everything that only served it

**Files:**
- Delete: `internal/characters/conditions.go`, `internal/characters/conditions_test.go`
- Modify: `internal/characters/character.go:300` (the field), `internal/characters/combat.go:291-293` (blinded dodge branch), `internal/characters/sight.go` (`HasAnyBlindSource` drops the condition arm and its doc line), `internal/hooks/Life_Cascades.go:77-78`, `internal/hooks/NewRound_UserRoundTick.go:369-370`, `internal/hooks/NewRound_MobRoundTick.go:597-600` and its caller, `internal/buffs/buffspec.go` (`ConditionMirror` constant, its `AllFlags` entry, the display-flag comment), `internal/usercommands/conditions.go:40-46` (mirror skip), `modules/gmcp/gmcp.Char.go:584-590` (mirror skip)
- Modify tests: `internal/state/perception/integration_test.go` (`TestIntegration_ConditionBlindedSolo`, `TestIntegration_MixedSourceOrder` and any other case using `ConditionBlinded`: delete those cases; keep the buff-driven ones), `internal/actions/combat_fire_test.go:347`

- [ ] **Step 1: Delete, then let the compiler enumerate**

`git rm internal/characters/conditions.go internal/characters/conditions_test.go`; remove the `Conditions` field; run `go build ./... 2>&1 | head -40` and fix every reference listed: it must be only the sites named above. If the compiler names a production site not in this list, STOP: a condition consumer was missed, and it belongs in the task of its condition (report which).

- [ ] **Step 2: The blinded dead code**

`combat.go:291-293`: replace the `if c.HasCondition(ConditionBlinded) { score *= ... }` branch with `score *= c.Buffs.Effect(buffs.EffectDodgeMult)` (identity 1 when no record declares it), so the vocabulary key has a reader even though no shipped record declares it; update the comment to say so. `sight.go`: drop the `HasCondition(ConditionBlinded)` arm and the doc bullet naming it.

- [ ] **Step 3: Verify the flag is gone from data and code**

Grep standalone: `grep -rn "condition-mirror\|ConditionMirror" --include=*.go --include=*.yaml internal/ modules/ _datafiles/world/` → empty. `grep -rn "HasCondition\|AddCondition\|GetConditionMagnitude\|RemoveCondition\|TickConditions\|CombatCondition\|ConditionType" --include=*.go internal/ modules/ .` → empty (tests included).

Run: `go build ./... && go vet ./... && go test ./internal/... ./modules/... -count=1` → all `ok` (run `internal/playtestrun` standalone if it reds under load). Root guards: `go test . -count=1` → `ok` (the viewpoint registry may report a stale key for a deleted shield-decay line; remove that entry, and register any new candidate the inlined prune narration created, with a reason).

```bash
git add -u internal/characters internal/hooks internal/buffs internal/usercommands modules/gmcp internal/state/perception internal/actions messaging_surface_guard_test.go
git commit -m "refactor(conditions): delete the combat condition enum, its tick, and the mirror flag"
```
(`git add -u` stages modifications and deletions of TRACKED files only, which is what this task makes; confirm with `git status --short` that no untracked file is involved before committing.)

---

### Task 12: One list, one payload

**Files:**
- Modify: `internal/usercommands/conditions.go`, `modules/gmcp/gmcp.Char.go`, `_datafiles/html/public/webclient-pure.html:2130-2200`, `_datafiles/html/public/static/js/renderguard.js:6-11` (comment)

- [ ] **Step 1: The command**

`conditions.go`: delete the "Stage 9.8: Append active combat conditions" loop (the field is gone; the compiler already forced this in Task 11 if it was not done there). The buff loop is the list.

- [ ] **Step 2: GMCP**

In `gmcp.Char.go`:
- Delete the `Char.Affects` builder block (`:562-633`) and the `Affects` field from `GMCPCharModule_Payload`; rename `GMCPCharModule_Payload_Affect` to `GMCPCondition` fields: keep `Name, Description, DurationMax, DurationLeft, Type, Mods` and ADD `Duration string \`json:"duration"\`` carrying `conditionDurationLabel(roundsLeft)` (the status panel's word). Delete the old three-field `GMCPCondition`.
- The `Char.Conditions` block (`:719-731`) builds the map the old Affects builder built (hidden skipped; the mirror skip is already gone; permabuff → `DurationMax -1`, `Duration "sustained"`).
- Identifiers: `Char.Affects, Char.Conditions` (`:156`) → `Char.Conditions`; `Char.Vitals, Char.Conditions` stays. Update the doc comments at `:776` and `:1063`.
- `wantsGMCPPayload` needs no change.

- [ ] **Step 3: The web client**

`webclient-pure.html:2136-2175`: the statuses loop reads `charObj.Conditions` (a name-keyed map now) and renders each with `name`, `description`, and the server's `duration` word (drop `affectDurLabel` if nothing else uses it; grep first). Delete the second (array) loop. Update the comment at 2136-2138 and `renderguard.js:6-11`. Confirm `grep -n "Affects" _datafiles/html/public/webclient-pure.html` is empty afterwards (standalone).

- [ ] **Step 4: Tests and verify**

`grep -rn "Char.Affects\|Affects" --include=*_test.go modules/gmcp/` and fix any test that built the old payload. Run `go test ./modules/gmcp/ ./internal/usercommands/ -count=1` → `ok`. Boot the web client is Task 15's playtest; here, `go build ./...`.

```bash
git add internal/usercommands/conditions.go modules/gmcp/gmcp.Char.go _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/js/renderguard.js
git commit -m "feat(conditions): one list in the conditions command and one Char.Conditions payload; Char.Affects retired"
```

---

### Task 13: The root guard against a second timed-state collection

**Files:**
- Create: `timed_state_guard_test.go` (repo root)

- [ ] **Step 1: Write it**

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Slice 1 of the conditions unification deleted the second collection of
// timed state (characters.CombatCondition). This guard keeps it deleted: no
// struct under internal/ or modules/ may declare a slice or map field whose
// element type declares a field named Duration or RoundsLeft alongside a
// Magnitude, and no identifier may spell HasCondition, AddCondition or
// CombatCondition. Timed state is a buffs.Buff, read through Buffs.Effect.
var forbiddenTimedStateIdents = []string{"HasCondition", "AddCondition", "RemoveCondition", "CombatCondition", "ConditionType", "TickConditions"}

func TestNoSecondTimedStateCollection(t *testing.T) {
	fset := token.NewFileSet()
	var problems []string
	for _, root := range []string{"internal", "modules"} {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					for _, bad := range forbiddenTimedStateIdents {
						if x.Name == bad {
							problems = append(problems, filepath.ToSlash(path)+": "+fset.Position(x.Pos()).String()+" spells "+bad)
						}
					}
				case *ast.StructType:
					hasDuration, hasMagnitude := false, false
					for _, fld := range x.Fields.List {
						for _, nm := range fld.Names {
							if nm.Name == "Duration" || nm.Name == "RoundsLeft" {
								hasDuration = true
							}
							if nm.Name == "Magnitude" {
								hasMagnitude = true
							}
						}
					}
					if hasDuration && hasMagnitude && !strings.HasPrefix(filepath.ToSlash(path), "internal/buffs/") {
						problems = append(problems, filepath.ToSlash(path)+": "+fset.Position(x.Pos()).String()+" declares a Duration+Magnitude struct outside internal/buffs; timed state is a buffs.Buff")
					}
				}
				return true
			})
			return nil
		})
	}
	if len(problems) > 0 {
		t.Fatalf("%d timed-state problem(s):\n  %s\n\nTimed state on a character is a buffs.Buff record read through Buffs.Effect; see internal/buffs/context.md.", len(problems), strings.Join(problems, "\n  "))
	}
}
```

- [ ] **Step 2: Prove it can fail, then commit**

Run `go test . -run TestNoSecondTimedStateCollection -count=1` → `ok`. Sabotage: add `func (c *Character) HasCondition() bool { return false }` to `internal/characters/buffs.go`; `go vet ./internal/characters/`; the guard → FAIL naming the file; revert; `ok`. Second sabotage: add `type shadow struct{ Duration int; Magnitude float64 }` to `internal/combat/ai.go`; guard → FAIL; revert; `ok`.

```bash
git add timed_state_guard_test.go
git commit -m "test: root guard forbids a second timed-state collection or a HasCondition spelling"
```

---

### Task 14: Documentation, patch note, README, spec corrections

**Files:**
- Modify: `internal/buffs/context.md`, `internal/characters/context.md`, `internal/hooks/context.md`, `internal/combat/context.md`, `modules/gmcp/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`, the spec

- [ ] **Step 1: `internal/buffs/context.md`**

Add a top-level section after the Overview:

```markdown
## Conditions are buffs (slice 1 of the conditions unification, 2026-09-12)

There is ONE collection of timed state on a character: `Buffs`. The former
`characters.CombatCondition` enum (ten combat conditions with their own tick,
magnitude and display) is gone; each is a record under
`_datafiles/world/dogmud/buffs/` (79, 80, 117 to 123). **Do not add a second
collection, a `HasCondition`, or a Go enum of timed effects**; the root guard
`timed_state_guard_test.go` fails the build on one. Slice 2 renames this
package to `internal/conditions`.

### The record's strength
- `Buff.Magnitude float64` is per-instance, set by the applier through
  `AddBuffMagnitude(id, rounds, magnitude, source)` (on `Buffs`,
  `Character`, `UserRecord`, and `events.Buff.Magnitude`). A held record of the
  same id is refreshed and its magnitude overwritten, as `AddCondition` did.
- `BuffSpec.Effects` is the closed vocabulary (`effects.go`): `damage_mult`,
  `defense_mult`, `dodge_mult`, `regen_mult`, `mitigation_flat`,
  `pool_max_pct`, `attacks_cap`. A value is a number or the word `magnitude`.
  `Validate` refuses any other key.
- `Buffs.Effect(kind)` is the ONE reader door: multipliers multiply (identity
  1, a zero magnitude contributes nothing), flats and pool fractions sum, caps
  take the minimum. `HasEffect(kind)` reports presence. Readers:
  `combat_helpers.go` (swings, damage, defense), `characters/combat.go`
  (mitigation, dodge), `characters/validate.go` (pool maxima, pool named by
  `Buff.Source`), `NewRound_AutoHeal.go` (regen), `combat/ai.go` and the
  behaviour tree (already shielded).
- `tick_from_magnitude: true` marks a tick record whose per-round amount is
  the applier's SIGNED magnitude (negative harms), snapshotted into
  `TickAmount`: the spell dot (121) and bleed (122). Damaging ticks wake a
  sleeper and cancel `cancel-on-damage` records.
- Scaling stays in the appliers: a record carries base values; the applier
  hands a duration multiplier and a magnitude in. The spell scaling arc owns
  the formulas.

### Flags added
`bleeding` (death cause reads it, as it reads `poison`) and `quiet` (listed,
no start or end line; for a record reapplied every round it persists).

### Facts worth knowing
- Former conditions now persist across logout and restart, like every record.
- The prone recovery record (118) is applied in the round tick and expired by
  the same tick's `Trigger`, so its swing cap never reaches combat. That is
  faithful to the enum it replaced; making it bite is a filed owner call.
```

Update the Files table with `ids.go` and `effects.go`.

- [ ] **Step 2: The other five**

- `internal/characters/context.md`: delete the conditions sections (grep `Condition` for the five places named in the spec) and add one pointer: "Timed state is `Buffs`; see `internal/buffs/context.md`. The combat condition enum was deleted 2026-09-12."; update the regen notes (lines ~556-576) to name `Buffs.Effect(EffectRegenMult)` and the Regenerating record; update the prone recovery section to name the Recovering record and its inertness.
- `internal/hooks/context.md`: the round tick section names the damaging-tick cancel helpers; the regen hook section names the door; the shield decay helper is gone.
- `internal/combat/context.md`: the three readers name the door.
- `modules/gmcp/context.md`: `Char.Affects` retired; `Char.Conditions` carries the unified map with a `duration` word.
- Run `python tools/context_md_audit.py` and confirm no phantom in buffs, characters, hooks, combat, gmcp.

- [ ] **Step 3: Patch note, README, spec**

`docs/PATCH_NOTES.md`, a new dated entry at the top, player-facing, no numbers, no dashes:

```markdown
## 2026-09-12: Shields hold as long as they should, and afflictions follow you

A ward used to fade twice as fast while you were fighting, and it fell silent
when it faded out of combat. It now lasts its full span and always tells you
when it goes. Poison, bleeding, a shout's fervor and the rest of the effects
that come and go now live in one place with everything else that affects
you, so the conditions list and your client show each of them once, and a
short absence no longer wipes them away.
```

`docs/README.md`: rows for the spec (already added) and this plan. The spec: add to "Findings recorded while planning" anything the execution corrected.

```bash
git add internal/buffs/context.md internal/characters/context.md internal/hooks/context.md internal/combat/context.md modules/gmcp/context.md docs/PATCH_NOTES.md docs/README.md docs/superpowers/specs/2026-09-12-conditions-unification-slice-1-model-design.md
git commit -m "docs: conditions are buffs; the door, the vocabulary, the flags, the patch note"
```

---

### Task 15: Gate: suite, boot, playtest lane, PR

- [ ] **Step 1: The suite**

`gofmt -l ./internal ./modules ./cmd .` (ignore `vendor/`), `go vet ./...`, `go test ./... -count=1` (log to the scratchpad, read the exit code from the log, never `| tail`). `internal/playtestrun` standalone if red under load. Root package standalone.

- [ ] **Step 2: The boot check** per `dogmud-shipping`: detached worktree at `C:/tmp/dogmud-boot-check`, copy `config.yaml`, build to `boot-check.exe`, `timeout 180`, exit 124, zero panics, one `Server Ready`, and `buffs.LoadDataFiles` logging 109 or more buffs (102 + 7).

- [ ] **Step 3: The playtest lane**

Create `tools/playtest/scenarios/conditions-slice-1.yaml` and `tools/playtest/goals/scenarios/conditions-slice-1/{shouter,caster}.yaml`, modelled on `m3-item5a-lit.yaml` and its goals (read them). Roster: `shouter` = profile `veteran` (rhetoric 55, owns two flesh golems), `caster` = profile `specialist-caster` (check it knows `conviction-ward`, `mend-wounds` and `neural-toxin` or `blood-boil`; if not, add them under `spellbook:` in a copied profile `conditions-caster.yaml`, since the owner allows reshaping testers). Start room 5000 (Sable). Group goals, each quoting lines verbatim:
1. `ask sable arena 350` and fight; the shouter types `warcry` then `rally`; both read the `conditions` output showing "Warcry" and "Rally" ONCE each; the shouter quotes the end lines when they fade ("The fervor of the warcry fades from you." / "The strength of the rally drains from you.").
2. The caster casts `conviction-ward` on the shouter, then both leave the arena and rest; the shouter quotes "Your Minor Shield dissipates." when it ends, out of combat, and the caster reads the room line.
3. The caster casts the dot spell at an arena mob; both read the mob's `look` and the per-round "burns" line does not appear for a mob (mob holders have no client), while the shouter, mauled by a beast (or by the caster's companion if the fixture allows), quotes "Blood seeps from your wounds!" each round and "Your wounds stop bleeding." at the end.
4. The shouter, holding the ward, `quit`s and logs back in; `conditions` still lists Minor Shield.
Run with `go run ./cmd/playtestrun scenario --checkout <abs path> --scenario tools/playtest/scenarios/conditions-slice-1.yaml`, launched detached (PowerShell `Start-Process ... -NoNewWindow`, quoted path). Read both bridges' `events.jsonl`. Tear down with `docker rm -f dogmud-playtest-<run_id>-server-1`. Extract findings to memory (reports are gitignored). Any literal `{source}`, any duplicated list entry, a shield ending in silence, or a lost record after login is a defect.

- [ ] **Step 4: Push and PR**

```bash
git push -u origin feature/conditions-unification-slice-1-model
gh pr create --repo pruuk/DOGMud --base master --head feature/conditions-unification-slice-1-model --title "Conditions unification slice 1: one model for timed state" --body-file - <<'EOF'
Slice 1 of the conditions unification (owner ruling 2026-09-12: buffs absorb the ten combat conditions; model first, then the rename, then the wire format).

- The buff instance gains a per-instance magnitude; the spec gains the closed `effects` vocabulary (seven keys) and `tick_from_magnitude`; combat reads timed state through one door, `Buffs.Effect`, and writes it through one, `AddBuffMagnitude(id, rounds, magnitude, source)`.
- Nine live conditions became records 79, 80 and 117 to 123 with the exact numbers their producers set; the dodge-penalty blinded condition had no producer and is deleted.
- Deleted: the enum, `TickConditions`, both shield decay helpers, `condition-mirror`, the second poison-immunity check.
- One list in `conditions`; one GMCP payload `Char.Conditions` (Affects shape plus the duration word); `Char.Affects` retired; the web client reads the one payload.
- Stated behaviour changes (spec, section "Behaviour changes"): Minor Shield decays once per round and narrates its end everywhere; former conditions persist across logout; poison and bleed tick in the round tick and every damaging tick now wakes a sleeper; the two one-round penalties are quiet records; the tick lines change category.
- Findings: the prone recovery penalty never reached combat (applied and expired in the same round tick); the record reproduces that faithfully, fix filed.
- Net: pins for every number, recorded before the model moved and kept through it; `buffs.golden` re-recorded once for the new records, the other nine untouched; root guard against a second timed-state collection; the apply-path, notice and flag guards learn the new door and flags.
- Gate: full suite, boot check, playtest scenario `conditions-slice-1` (run id in the last commit message).

Spec: docs/superpowers/specs/2026-09-12-conditions-unification-slice-1-model-design.md
Plan: docs/superpowers/plans/2026-09-12-conditions-unification-slice-1-model.md

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
gh pr checks <n> --repo pruuk/DOGMud --watch
```
Merge on green with `--merge --delete-branch`. The owner deploys; do not.
