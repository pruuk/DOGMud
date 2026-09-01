# Toxicity: Alchemical Tolerance and Graduated Feedback — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make potion toxicity a real, visible, alchemy-earned resource: tolerance
comes from brewing rather than toughness, every drinkable costs something, pressure
is visible before it hurts, and death clears the debt.

**Architecture:** Six small, independent changes behind one config seam. The
formula in `Character.GetToxicityMax` gains an alchemy term; `ToxicityBand` gains
two purely-cosmetic warning tiers below the first penalty tier; the death cascade
zeroes toxicity beside the buff strip that justifies it; nineteen item YAMLs get
their toxicity values set or rescaled; `config.yaml` stops running the subsystem on
invisible Go defaults; and the help system learns the topic exists.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v2` data files, Go text/template help pages,
`testify` for assertions.

**Source spec:** `docs/superpowers/specs/2026-08-31-toxicity-tolerance-and-visibility-design.md`

---

## Facts verified against source, 2026-08-31 (this plan's own pass)

Verified by reading the files at plan-writing time. Four entries **correct the
spec** — read this table, not the spec, where they disagree.

| Fact | Evidence |
|---|---|
| `GetToxicityMax` reads `c.Stats.Vitality.ValueAdj`, and `ValueAdj = Base + Training + Mods` | `internal/characters/resources.go:454`; `internal/stats/softcap_test.go:22` pins the sum |
| 🔴 **SPEC CORRECTION.** The spec's table computed Meirok from `base: 104`. His real prod vitality **with gear is 150** (owner, 2026-08-31). The spec's own prose already assumed this ("falls from about 130 to about 73") — only the table row was wrong | `_datafiles/world/dogmud/users/3.yaml:66`; owner |
| 🔴 **SPEC CORRECTION.** At the spec's `ToxicityAlchemyScale: 1.5` and vit 150, Meirok's max is 88.7 and the **6th** low-tier potion bites, missing the target. `ToxicityAlchemyScale: 2.5` yields **73.2** and makes **every row** of the spec's table land exactly | computed in Task 1's test; see the calibration table below |
| 🔴 **SPEC CORRECTION.** `status` **already shows** toxicity. The spec grepped `status.go` and found nothing, but the line lives in the template | `_datafiles/world/dogmud/templates/character/status.template:10` renders `{{ toxicityQuality .Character.ToxicityBandName 13 }}` |
| 🔴 **SPEC CORRECTION.** `help toxicity` **already resolves** — `GetHelpContents` processes `help/<name>` directly, and the template exists. `keywords.yaml` controls only the *index listing* and *aliases* | `internal/usercommands/help.go:191`; `_datafiles/world/dogmud/templates/help/toxicity.template` |
| `toxicityQuality` template func already exists and switches on the band **name string**, padding to a caller-given width | `internal/templates/templatesfunctions.go:315-332` |
| Exactly three places switch on the **numeric** band | `internal/users/userrecord.prompt.go:685`, `internal/hooks/NewRound_AutoHeal.go:75,98-116`, and nothing else |
| GMCP sends the band **name string**, and no web-client JS switches on it | `modules/gmcp/gmcp.Char.go:542`; `grep` of `_datafiles/html/public/static/js/` finds only an item-editor numeric field |
| The `<= 0 → 100` guard on `ToxicityBaseMax` makes an explicit `0` unreachable | `internal/configs/config.balance.combat.go:379-381` |
| `ConfigFloat` is a bare `float64` — there is **no** "was it set?" distinction, so absent and explicit-zero are identical | `internal/configs/config_types.go:12` |
| Test binaries see the **Go defaults**, because `GetBalanceConfig` validates a zero config | `internal/configs/config.balance.go:1014`; existing test comment `resources_test.go:68` |
| `configs.SetConfigForTest(t, c)` assigns **without** validating, so a test can pin an exact value | `internal/configs/testing_support.go:30` |
| `internal/characters` already imports `internal/skills`; `GetSkillLevel(skills.Alchemy)` returns trained rank plus equipment/buff StatMods | `internal/characters/skills.go:8,170`; `internal/skills/skills.go:40` |
| `resources.go` does **not** yet import `internal/skills` — the import must be added | `internal/characters/resources.go:3-10` |
| Death strips every buff at `Alive → Dead`; nothing anywhere clears `Toxicity` | `internal/hooks/Life_Cascades.go:62`; only two `Toxicity = 0` sites exist and both are floor clamps |
| `drink.go` **rejects** a potion that would exceed max, and Ysolde's Purge (40109) is the one hardcoded bypass | `internal/usercommands/drink.go:148-155`, `:33` |
| There are exactly **34** drinkables; **9** carry no `toxicity:` field | enumerated by `grep -rl '^subtype: drinkable'`; matches the spec |
| Existing toxicity tests assume `ToxicityBaseMax == 100` on a zero-Vitality character and **will break** when the default becomes 0 | `internal/characters/resources_test.go:68,93,121` |
| `keywords.yaml` help topics are grouped `help: command: <category>:`; `character:` already holds `conditions`, `encumbrance`, `progression` | `_datafiles/world/dogmud/keywords.yaml:19-40` |

### The calibration table this plan is fitted to

Shipped scales: `ToxicityBaseMax: 0`, `ToxicityAlchemyScale: 2.5`,
`ToxicityVitalityScale: 3`. Low tier costs 8, mid tier costs 11 (down from 14).
"Bites" = reaches the 50% queasy line, which is the first tier that carries a
penalty.

| Character | Alchemy | Vit | Max | Low tier (8) | Mid tier (11) |
|---|---:|---:|---:|---|---|
| Noob | 0 | 100 | 33.3 | 3rd bites (72%) | 2nd bites (66%) |
| Veteran, no alchemy | 0 | 150 | 50.0 | 4th bites (64%) | 3rd bites (66%) |
| Mid alchemist | 25 | 125 | 51.7 | 4th bites (62%) | 3rd bites (64%) |
| **Meirok (real prod save)** | **58** | **150** | **73.2** | **5th bites (55%)** | **4th bites (60%)** |

Every row matches the spec's intent. Task 2 pins this table in a test.

---

## File Structure

**Go — modified**

| File | Responsibility |
|---|---|
| `internal/configs/config.balance.go` | Declares the new `ToxicityAlchemyScale` field |
| `internal/configs/config.balance.combat.go` | Toxicity defaulting; the `ToxicityBaseMax` guard fix |
| `internal/characters/resources.go` | `GetToxicityMax` alchemy term; `ToxicityBand`/`ToxicityBandName` six-tier split |
| `internal/hooks/Life_Cascades.go` | Zero toxicity on `Alive → Dead` |
| `internal/hooks/NewRound_AutoHeal.go` | Onset messages for the six bands |
| `internal/users/userrecord.prompt.go` | `{tox}` colors for the six bands |
| `internal/templates/templatesfunctions.go` | `toxicityQuality` colors for the two new names |

**Go — created**

| File | Responsibility |
|---|---|
| `internal/configs/toxicity_config_test.go` | Pins that an explicit `ToxicityBaseMax: 0` survives validation |
| `internal/characters/toxicity_calibration_test.go` | Pins the four-row calibration table and the six bands |
| `internal/hooks/toxicity_death_test.go` | Pins that death clears toxicity |

**Data — modified**

| File | Responsibility |
|---|---|
| 19 item YAMLs under `_datafiles/world/dogmud/items/` | Backfill 9, retune mid tier 3, rescale heavy tier 7 |
| `_datafiles/config.yaml` | Six toxicity knobs made explicit (**skip-worktree** — see Task 6) |
| `_datafiles/world/dogmud/keywords.yaml` | Register `toxicity` topic + aliases |
| `_datafiles/world/dogmud/templates/help/toxicity.template` | Rewrite for the new formula and bands |
| `_datafiles/world/dogmud/templates/help/alchemy.template` | Say brewing raises tolerance; cross-link |
| `_datafiles/world/dogmud/templates/help/craft.template` | Cross-link |
| `_datafiles/world/dogmud/templates/help/health.template` | Cross-link |
| `docs/PATCH_NOTES.md` | Dated player-facing entry |

---

## Task 1: Config — make an explicit zero reachable, and add the alchemy scale

This is the spec's confirmed blocker and must land first: until the guard is
fixed, `ToxicityBaseMax: 0` is silently rewritten to 100 and none of the tuning
in later tasks can be tested at all.

**Files:**
- Modify: `internal/configs/config.balance.go:773-777`
- Modify: `internal/configs/config.balance.combat.go:375-390`
- Test: `internal/configs/toxicity_config_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/configs/toxicity_config_test.go`:

```go
package configs

import "testing"

// TestToxicityBaseMaxAcceptsExplicitZero pins the fix for a guard whose shape
// made a legal value unreachable. ToxicityBaseMax: 0 is the SHIPPED value --
// tolerance is earned entirely from alchemy and vitality -- but the old
// `if b.ToxicityBaseMax <= 0 { b.ToxicityBaseMax = 100 }` rewrote it to 100 on
// every load, restoring the flat ceiling and reverting the tolerance model with
// no error anywhere. Nothing failed; the numbers were simply wrong.
func TestToxicityBaseMaxAcceptsExplicitZero(t *testing.T) {
	b := Balance{ToxicityBaseMax: 0}
	b.validateCombat()
	if b.ToxicityBaseMax != 0 {
		t.Errorf("ToxicityBaseMax = %v, want 0 -- an explicit zero must survive validation", b.ToxicityBaseMax)
	}
}

// TestToxicityBaseMaxRejectsNegative verifies the guard still rejects nonsense.
func TestToxicityBaseMaxRejectsNegative(t *testing.T) {
	b := Balance{ToxicityBaseMax: -5}
	b.validateCombat()
	if b.ToxicityBaseMax != 0 {
		t.Errorf("ToxicityBaseMax = %v, want 0 for a negative input", b.ToxicityBaseMax)
	}
}

// TestToxicityDivisorsStillDefault verifies the <= 0 guard is KEPT on the two
// divisors. These are denominators in GetToxicityMax and must never be zero, so
// their guard shape is correct and deliberately differs from ToxicityBaseMax's.
func TestToxicityDivisorsStillDefault(t *testing.T) {
	b := Balance{}
	b.validateCombat()
	if b.ToxicityAlchemyScale != 2.5 {
		t.Errorf("ToxicityAlchemyScale = %v, want 2.5", b.ToxicityAlchemyScale)
	}
	if b.ToxicityVitalityScale != 3 {
		t.Errorf("ToxicityVitalityScale = %v, want 3", b.ToxicityVitalityScale)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/configs/ -run TestToxicity -v`

Expected: compile error `b.ToxicityAlchemyScale undefined`. That is the correct
first failure — the field does not exist yet.

- [ ] **Step 3: Declare the new field**

In `internal/configs/config.balance.go`, the toxicity block currently reads:

```go
	ToxicityDecayPerTick      ConfigFloat `yaml:"ToxicityDecayPerTick"`      // Points decayed per regen tick (default 1.0)
	ToxicityBaseMax           ConfigFloat `yaml:"ToxicityBaseMax"`           // Base max before vitality bonus (default 100)
	ToxicityVitalityScale     ConfigFloat `yaml:"ToxicityVitalityScale"`     // Vitality divisor for max bonus (default 5)
```

Replace those three lines with:

```go
	ToxicityDecayPerTick      ConfigFloat `yaml:"ToxicityDecayPerTick"`      // Points decayed per regen tick (default 1.0)
	ToxicityBaseMax           ConfigFloat `yaml:"ToxicityBaseMax"`           // Flat floor on tolerance; 0 = entirely earned (default 0)
	ToxicityAlchemyScale      ConfigFloat `yaml:"ToxicityAlchemyScale"`      // Alchemy divisor for max bonus (default 2.5)
	ToxicityVitalityScale     ConfigFloat `yaml:"ToxicityVitalityScale"`     // Vitality divisor for max bonus (default 3)
```

- [ ] **Step 4: Fix the guard and set the defaults**

In `internal/configs/config.balance.combat.go`, the toxicity block currently reads:

```go
	// ── TOXICITY ────────────────────────────────────────────────────────────
	if b.ToxicityDecayPerTick <= 0 {
		b.ToxicityDecayPerTick = 1.0
	}
	if b.ToxicityBaseMax <= 0 {
		b.ToxicityBaseMax = 100
	}
	if b.ToxicityVitalityScale <= 0 {
		b.ToxicityVitalityScale = 5
	}
```

Replace that whole span with:

```go
	// ── TOXICITY ────────────────────────────────────────────────────────────
	if b.ToxicityDecayPerTick <= 0 {
		b.ToxicityDecayPerTick = 1.0
	}
	// ToxicityBaseMax is a flat floor on tolerance, and 0 is the SHIPPED value:
	// tolerance is earned entirely from alchemy and vitality. Guard only against
	// a negative. The old `<= 0 -> 100` guard made that legal zero unreachable,
	// silently restoring a flat 100 ceiling and reverting the whole tolerance
	// model with nothing failing anywhere. ConfigFloat is a bare float64, so an
	// absent key and an explicit 0 are indistinguishable -- which is exactly why
	// the default has to BE zero rather than be patched in by a guard.
	if b.ToxicityBaseMax < 0 {
		b.ToxicityBaseMax = 0
	}
	// The next two are DIVISORS in Character.GetToxicityMax and must never be
	// zero, so the `<= 0` shape is correct here and must stay. Do not "tidy"
	// them to match ToxicityBaseMax above -- the difference is deliberate.
	if b.ToxicityAlchemyScale <= 0 {
		b.ToxicityAlchemyScale = 2.5
	}
	if b.ToxicityVitalityScale <= 0 {
		b.ToxicityVitalityScale = 3
	}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/configs/ -run TestToxicity -v`

Expected: PASS, three tests.

- [ ] **Step 6: Run the whole configs package**

Run: `go test ./internal/configs/`

Expected: PASS. If `smoke_test.go` asserts `ToxicityBaseMax > 0`, that assertion
encoded the bug — delete that one assertion and note why in the commit message.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go internal/configs/toxicity_config_test.go
git commit -F - <<'MSG'
fix(config): let ToxicityBaseMax be explicitly zero

The guard read `if b.ToxicityBaseMax <= 0 { b.ToxicityBaseMax = 100 }`, so
the value the tolerance model actually wants -- zero, meaning tolerance is
earned entirely rather than granted flat -- was rewritten to 100 on every
load. Nothing failed. The ceiling simply stayed flat and the model reverted.

ConfigFloat is a bare float64, so absent and explicit-zero cannot be told
apart; the default therefore has to be zero rather than patched in by a
guard. Guard only against a negative now.

Adds ToxicityAlchemyScale (2.5) and moves ToxicityVitalityScale to 3 so the
Go defaults match what config.yaml will ship. Test binaries never read
config.yaml, so leaving the defaults behind would make every toxicity test
measure a formula the game does not use.

The two divisors keep their `<= 0` guard on purpose -- they are denominators
and must never be zero. A comment says so, because making the three guards
look alike is what would reintroduce the bug.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 2: Tolerance is earned by brewing

**Files:**
- Modify: `internal/characters/resources.go:450-455` and its import block at `:3-10`
- Modify: `internal/characters/resources_test.go:65-150` (three tests assume a 100 base max)
- Test: `internal/characters/toxicity_calibration_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/toxicity_calibration_test.go`:

```go
package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// toxChar builds a character with a known alchemy rank and vitality.
// Stat setup is `.Base` then Recalculate(), which is what makes ValueAdj real.
func toxChar(alchemy, vitality int) *Character {
	c := &Character{Skills: map[string]int{string(skills.Alchemy): alchemy}}
	c.Stats.Vitality.Base = vitality
	c.Stats.Vitality.Recalculate()
	return c
}

// pinToxicityScales pins the SHIPPED scales so this test measures the tuning the
// game actually runs, not whatever the Go defaults drift to later.
// SetConfigForTest assigns without validating and self-registers the restore.
func pinToxicityScales(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ToxicityBaseMax = 0
	c.Balance.ToxicityAlchemyScale = 2.5
	c.Balance.ToxicityVitalityScale = 3
	configs.SetConfigForTest(t, c)
}

// TestToxicityCalibrationTable pins the whole design target: which potion in a
// row is the one that first pushes you into a band that actually costs you
// something. A later edit to either scale that moves any of these numbers is a
// balance change and must be a deliberate one -- this test is the tripwire.
//
// Low tier costs 8 (healing salve, stamina tonic, conviction draught).
// Mid tier costs 11 (warrior's brew, preacher's tincture, windrunner draught).
// "Bites" means reaching 50%, the first band carrying a penalty.
func TestToxicityCalibrationTable(t *testing.T) {
	pinToxicityScales(t)

	// firstBiting returns the 1-based index of the first potion of cost `each`
	// that reaches the 50% penalty line, or 0 if the drink gate rejects one first.
	firstBiting := func(c *Character, each float64) int {
		max := c.GetToxicityMax()
		for i := 1; i <= 12; i++ {
			total := float64(i) * each
			if total > max { // drink.go rejects a potion that would exceed max
				return 0
			}
			if total/max >= 0.50 {
				return i
			}
		}
		return 0
	}

	cases := []struct {
		name             string
		alchemy, vit     int
		wantMax          float64
		wantLow, wantMid int
	}{
		{"noob", 0, 100, 33.33, 3, 2},
		{"veteran, no alchemy", 0, 150, 50.00, 4, 3},
		{"mid alchemist", 25, 125, 51.67, 4, 3},
		{"meirok, real prod save", 58, 150, 73.20, 5, 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := toxChar(tc.alchemy, tc.vit)

			if got := c.GetToxicityMax(); math.Abs(got-tc.wantMax) > 0.01 {
				t.Errorf("GetToxicityMax() = %.2f, want %.2f", got, tc.wantMax)
			}
			if got := firstBiting(c, 8); got != tc.wantLow {
				t.Errorf("low tier (8): potion %d bites, want %d", got, tc.wantLow)
			}
			if got := firstBiting(c, 11); got != tc.wantMid {
				t.Errorf("mid tier (11): potion %d bites, want %d", got, tc.wantMid)
			}
		})
	}
}

// TestToleranceIsEarnedNotInherited is the flavour claim as an assertion: a
// veteran who never brewed is LESS tolerant than a mid-level dabbler. Toughness
// is not tolerance. That inversion is the entire point of the change, and it is
// the first thing a "simplification" back to a vitality-only formula would break.
func TestToleranceIsEarnedNotInherited(t *testing.T) {
	pinToxicityScales(t)

	toughVeteran := toxChar(0, 150)    // brawny, never touched a still
	dabblingBrewer := toxChar(40, 100) // ordinary body, real alchemy practice

	if dabblingBrewer.GetToxicityMax() <= toughVeteran.GetToxicityMax() {
		t.Errorf("brewer max %.1f must EXCEED tough-but-untrained veteran max %.1f",
			dabblingBrewer.GetToxicityMax(), toughVeteran.GetToxicityMax())
	}
}

// TestZeroToleranceCharacterIsSafe guards the divide-by-zero path. With
// ToxicityBaseMax at 0, a character with no alchemy and no vitality has a max of
// 0 -- which every consumer must treat as "no toxicity system", never as a
// divide. All three readers already early-return on max <= 0; this pins that.
func TestZeroToleranceCharacterIsSafe(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 0)
	if got := c.GetToxicityMax(); got != 0 {
		t.Fatalf("GetToxicityMax() = %v, want 0", got)
	}
	c.Toxicity = 50 // nonsense state, but must not panic or divide

	if got := c.ToxicityBand(); got != 0 {
		t.Errorf("ToxicityBand() = %d, want 0 at zero max", got)
	}
	if got := c.ToxicitySicknessDamage(); got != 0 {
		t.Errorf("ToxicitySicknessDamage() = %d, want 0 at zero max", got)
	}
	if r, p, d := c.GetToxicityPenalties(); r != 1.0 || p != 1.0 || d != 1.0 {
		t.Errorf("GetToxicityPenalties() = %v/%v/%v, want all 1.0 at zero max", r, p, d)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/characters/ -run 'TestToxicityCalibrationTable|TestToleranceIsEarned' -v`

Expected: FAIL. `GetToxicityMax()` still ignores alchemy, so the noob row reports
`33.33` correctly by luck but Meirok reports `50.00` instead of `73.20`, and
`TestToleranceIsEarnedNotInherited` fails with brewer `33.3` vs veteran `50.0`.

- [ ] **Step 3: Add the alchemy term**

In `internal/characters/resources.go`, add `skills` to the import block so it reads:

```go
import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)
```

Then replace `GetToxicityMax` (currently at `:450-455`) with:

```go
// GetToxicityMax returns the maximum toxicity this character can handle.
//
//	max = ToxicityBaseMax + alchemy/AlchemyScale + Vitality/VitalityScale
//
// Tolerance is EARNED BY BREWING, not by being tough. A veteran who never
// touched a still is less tolerant than a mid-level dabbler -- you build a
// tolerance by handling the stuff, and that inversion is the point of the
// formula. ToxicityBaseMax ships at 0 so tolerance is entirely earned; it
// survives as a knob so a flat floor can be restored without a code change.
//
// Both divisors are guaranteed non-zero by validateCombat.
func (c *Character) GetToxicityMax() float64 {
	bal := configs.GetBalanceConfig()
	alchemy := float64(c.GetSkillLevel(skills.Alchemy))
	return float64(bal.ToxicityBaseMax) +
		alchemy/float64(bal.ToxicityAlchemyScale) +
		float64(c.Stats.Vitality.ValueAdj)/float64(bal.ToxicityVitalityScale)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/characters/ -run 'TestToxicityCalibrationTable|TestToleranceIsEarned|TestZeroTolerance' -v`

Expected: PASS, all rows.

- [ ] **Step 5: Repair the three pre-existing tests that assumed a base max of 100**

`internal/characters/resources_test.go` builds bare `&Character{}` values and
comments "zero Vitality → GetToxicityMax == ToxicityBaseMax (100)". That is no
longer true — the default is now 0, so those characters have a max of 0 and the
tests measure nothing. Give each one a real tolerance instead.

In `TestAddToxicity_ClampsToMaxAndReturnsTrue` (`:67`), replace the first line:

```go
	c := &Character{} // zero Vitality → GetToxicityMax == ToxicityBaseMax (100)
```

with:

```go
	// ToxicityBaseMax now ships at 0 -- tolerance is earned from alchemy and
	// vitality -- so a bare Character has a max of 0 and this test would clamp
	// against nothing. Give it a real ceiling of exactly 100: vitality 300 at
	// the default VitalityScale of 3.
	c := &Character{}
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
```

In `TestToxicitySicknessDamage` (`:91`), replace:

```go
	max := c.GetToxicityMax() // 100 (zero Vitality)
```

Look at the two lines above it — the character is built there. Change that
construction the same way, so the whole preamble reads:

```go
	// Vitality 300 / VitalityScale 3 == a max of exactly 100, which keeps every
	// arithmetic expectation below unchanged now that ToxicityBaseMax is 0.
	c := &Character{}
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.HealthMax.Mods = 1000
	c.HealthMax.Recalculate()
	max := c.GetToxicityMax() // 100
```

If the existing test sets `HealthMax` by a different route, keep whatever it
already does and change only the character construction and the comment — the
damage assertions depend on `HealthMax.Value == 1000` and must keep working.

In `TestToxicityBand` (`:119`), replace:

```go
	max := c.GetToxicityMax()
```

and the construction above it with:

```go
	// Vitality 300 / VitalityScale 3 == a max of exactly 100, so the percentage
	// cases below still read as plain percentages.
	c := &Character{}
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	max := c.GetToxicityMax() // 100
```

Leave the band expectations alone for now — Task 3 renumbers them.

- [ ] **Step 6: Run the whole characters package**

Run: `go test ./internal/characters/`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/resources.go internal/characters/resources_test.go internal/characters/toxicity_calibration_test.go
git commit -F - <<'MSG'
feat(toxicity): tolerance is earned by brewing, not by being tough

GetToxicityMax was BaseMax + Vitality/5, which gave a fresh character 120 and
a veteran 130 -- an 8% spread for a 50% difference in vitality. Nothing a
player did moved it and the number expressed nothing about them.

It now reads alchemy as well:

  max = BaseMax + alchemy/2.5 + Vitality/3

with BaseMax shipping at 0, so tolerance is entirely earned. The flavour falls
out of the maths: a veteran who never brewed is LESS tolerant than a mid-level
dabbler, because you build a tolerance by handling the stuff.

Fitted against Meirok's real prod save (alchemy 58, vitality 150 with gear):
max 73.2, and the fifth low-tier potion is the one that bites. A test pins
that row and three others so a later scale edit cannot quietly move where the
cost lands.

Note the fit uses Vitality.ValueAdj (base + training + mods), which is what
the code has always read -- an earlier draft computed from the save's `base:`
field alone and landed a potion off.

Three existing tests built bare Characters and relied on the old flat 100
ceiling. They now set vitality 300 for a max of exactly 100, so every
arithmetic expectation in them is preserved.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 3: Two warning tiers below the first penalty tier

Bands become finer than penalties. **No penalty threshold moves** — this changes
only what a player can see.

Band numbering goes from four values to six. Three call sites switch on the
number and all three must move together.

**Files:**
- Modify: `internal/characters/resources.go` (`ToxicityBand`, `ToxicityBandName`)
- Modify: `internal/characters/resources_test.go` (`TestToxicityBand` expectations)
- Modify: `internal/users/userrecord.prompt.go:683-698`
- Modify: `internal/hooks/NewRound_AutoHeal.go:96-117`
- Modify: `internal/templates/templatesfunctions.go:319-328`
- Test: `internal/characters/toxicity_calibration_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/characters/toxicity_calibration_test.go`:

```go
// TestToxicityBandsAreFinerThanPenalties pins the deliberate asymmetry this
// change introduces. ToxicityBand and GetToxicityPenalties used to mirror each
// other exactly and the code said so. They no longer do: bands 1 and 2 are pure
// feedback and carry NO penalty, so a player can watch pressure build before it
// costs anything. Restoring the symmetry would silently delete both warning
// tiers, which is why this test exists.
func TestToxicityBandsAreFinerThanPenalties(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 300) // max exactly 100, so ratios read as percentages
	if got := c.GetToxicityMax(); got != 100 {
		t.Fatalf("fixture max = %v, want 100", got)
	}

	cases := []struct {
		pct         float64
		wantBand    int
		wantName    string
		wantPenalty bool
	}{
		{0.00, 0, "clear", false},
		{0.14, 0, "clear", false},
		{0.15, 1, "sour", false},      // NEW -- visible, costs nothing
		{0.29, 1, "sour", false},      // NEW
		{0.30, 2, "unsettled", false}, // NEW -- visible, costs nothing
		{0.49, 2, "unsettled", false}, // NEW
		{0.50, 3, "queasy", true},     // unchanged penalty threshold
		{0.74, 3, "queasy", true},
		{0.75, 4, "sick", true}, // unchanged penalty threshold
		{0.89, 4, "sick", true},
		{0.90, 5, "critical", true}, // unchanged penalty threshold
		{1.00, 5, "critical", true},
	}

	for _, tc := range cases {
		c.Toxicity = 100 * tc.pct

		if got := c.ToxicityBand(); got != tc.wantBand {
			t.Errorf("at %.0f%%: ToxicityBand() = %d, want %d", tc.pct*100, got, tc.wantBand)
		}
		if got := c.ToxicityBandName(); got != tc.wantName {
			t.Errorf("at %.0f%%: ToxicityBandName() = %q, want %q", tc.pct*100, got, tc.wantName)
		}

		regen, per, dex := c.GetToxicityPenalties()
		penalised := regen != 1.0 || per != 1.0 || dex != 1.0
		if penalised != tc.wantPenalty {
			t.Errorf("at %.0f%%: penalised = %v, want %v (regen %.2f per %.2f dex %.2f)",
				tc.pct*100, penalised, tc.wantPenalty, regen, per, dex)
		}
	}
}

// TestBandNamesFitTheStatusColumn guards a layout constraint that is invisible
// in Go: status.template pads the band name to 13 visual characters, so a
// longer name would push the box border off and break the whole status screen.
func TestBandNamesFitTheStatusColumn(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 300)
	for pct := 0.0; pct <= 1.0; pct += 0.05 {
		c.Toxicity = 100 * pct
		if n := c.ToxicityBandName(); len(n) > 13 {
			t.Errorf("band name %q is %d chars, exceeds the 13-char status column", n, len(n))
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/characters/ -run TestToxicityBandsAreFiner -v`

Expected: FAIL — at 15% the band is still `0`/`"clear"`, and at 50% it is still
`1`/`"queasy"` rather than `3`.

- [ ] **Step 3: Split the bands**

In `internal/characters/resources.go`, replace `ToxicityBand` and
`ToxicityBandName` (currently at `:496-553`) with:

```go
// ToxicityBand returns the toxicity severity band:
//
//	0 = clear     (<15%)   no penalty
//	1 = sour      (>=15%)  no penalty -- FEEDBACK ONLY
//	2 = unsettled (>=30%)  no penalty -- FEEDBACK ONLY
//	3 = queasy    (>=50%)  penalty
//	4 = sick      (>=75%)  penalty
//	5 = critical  (>=90%)  penalty + acute HP damage
//
// ⚠️ These thresholds DELIBERATELY DO NOT MIRROR GetToxicityPenalties. Bands 1
// and 2 exist so a player can watch pressure building before it costs anything;
// they have no counterpart in the penalty table on purpose. The two functions
// used to mirror each other exactly and the comments said so -- "restoring" that
// symmetry silently deletes both warning tiers, which is the whole feature.
// TestToxicityBandsAreFinerThanPenalties will fail if you do.
func (c *Character) ToxicityBand() int {
	max := c.GetToxicityMax()
	if max <= 0 {
		return 0
	}
	ratio := c.Toxicity / max
	switch {
	case ratio >= 0.90:
		return 5
	case ratio >= 0.75:
		return 4
	case ratio >= 0.50:
		return 3
	case ratio >= 0.30:
		return 2
	case ratio >= 0.15:
		return 1
	default:
		return 0
	}
}

// ToxicityBandName returns the descriptive tier word for the current band.
// Names must stay at or under 13 characters: status.template pads this into a
// 13-wide column and a longer word breaks the box border.
func (c *Character) ToxicityBandName() string {
	switch c.ToxicityBand() {
	case 5:
		return "critical"
	case 4:
		return "sick"
	case 3:
		return "queasy"
	case 2:
		return "unsettled"
	case 1:
		return "sour"
	default:
		return "clear"
	}
}
```

- [ ] **Step 4: Add the matching warning to the penalty function**

Still in `internal/characters/resources.go`, replace the doc comment on
`GetToxicityPenalties` (currently at `:492-495`, the lines beginning
`// GetToxicityPenalties returns stat multipliers`) with:

```go
// GetToxicityPenalties returns stat multipliers based on toxicity threshold.
// Returns (regenMult, perceptionMult, dexterityMult) where 1.0 = no penalty.
//
// ⚠️ These thresholds are 50/75/90 and DELIBERATELY COARSER than ToxicityBand's
// six tiers. Do not add entries here to "match" the band list: bands 1 and 2 are
// feedback-only warnings that must stay free. Changing this table is a balance
// change, not a tidy-up.
```

- [ ] **Step 5: Update the prompt token colors**

In `internal/users/userrecord.prompt.go`, the `{tox}` case (currently at
`:683-698`) has a three-way color switch. Replace the whole `case` with:

```go
			case `{tox}`:
				// Toxicity band tier word, colored by severity. Blank at band 0
				// so a default prompt stays quiet for an unaffected player. The
				// two lowest bands are warnings that carry no penalty, so they
				// get muted colors -- visible, but not alarming.
				band := u.Character.ToxicityBand()
				if band > 0 {
					bandName := u.Character.ToxicityBandName()
					var toxColor string
					switch band {
					case 1:
						toxColor = "black-bold" // sour -- barely there
					case 2:
						toxColor = "white" // unsettled -- noticeable, harmless
					case 3:
						toxColor = "yellow" // queasy -- first band that costs you
					case 4:
						toxColor = "red"
					default:
						toxColor = "red-bold"
					}
					promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, toxColor, bandName))
				}
```

- [ ] **Step 6: Update the onset messages**

In `internal/hooks/NewRound_AutoHeal.go`, replace the whole band-crossing block
(currently at `:96-117`, from `// Notify player when toxicity crosses` through
the closing brace of the `if newToxBand != prevToxBand` statement) with:

```go
		// Notify player when toxicity crosses a named threshold — once per
		// crossing, not every tick. Bands 1 and 2 carry no penalty; their lines
		// exist so a player can feel pressure building before it costs anything.
		if newToxBand := user.Character.ToxicityBand(); newToxBand != prevToxBand {
			if newToxBand > prevToxBand {
				// Worsening — band-specific onset messages.
				switch newToxBand {
				case 1:
					user.SendText(messaging.CategoryWarning,
						`A faint sourness settles on the back of your tongue.`)
				case 2:
					user.SendText(messaging.CategoryWarning,
						`Your stomach sits uneasy, and the taste will not wash away.`)
				case 3:
					user.SendText(messaging.CategoryWarning,
						`A faint nausea settles in and will not quite lift.`)
				case 4:
					user.SendText(messaging.CategoryWarning,
						`Your hands have a fine tremor now, and your sight swims at the edges.`)
				case 5:
					user.SendText(messaging.CategoryWarning,
						`Your whole body is in revolt — sweat, shakes, the taste of metal.`)
				}
			} else {
				// Improving — one relief line for any downward crossing.
				user.SendText(messaging.CategoryWarning,
					`The worst of the sickness ebbs; you can breathe a little easier.`)
			}
		}
```

Note the three existing strings are preserved, shifted from bands 1/2/3 to 3/4/5,
so the wording a player already knows still matches the tier it always meant.

- [ ] **Step 7: Update the status-screen colors**

In `internal/templates/templatesfunctions.go`, replace the `switch bandName`
inside `toxicityQuality` (currently at `:319-328`) with:

```go
			var color string
			switch bandName {
			case "sour":
				color = "black-bold" // feedback only, no penalty
			case "unsettled":
				color = "white" // feedback only, no penalty
			case "queasy":
				color = "yellow" // first band that carries a penalty
			case "sick":
				color = "red"
			case "critical":
				color = "red-bold"
			default: // "clear"
				color = "green"
			}
```

- [ ] **Step 8: Update the pre-existing band test**

`internal/characters/resources_test.go`'s `TestToxicityBand` (`:119`) asserts the
old four-tier numbering. Its cases now duplicate the new
`TestToxicityBandsAreFinerThanPenalties` and would fail. Delete the whole
`TestToxicityBand` function and leave this in its place:

```go
// TestToxicityBand moved to toxicity_calibration_test.go as
// TestToxicityBandsAreFinerThanPenalties, which covers the same thresholds plus
// the two feedback-only bands and asserts that no penalty threshold moved with
// them. Do not re-add a four-tier version here.
```

- [ ] **Step 9: Run every affected package**

Run: `go build ./... && go test ./internal/characters/ ./internal/hooks/ ./internal/users/ ./internal/templates/`

Expected: PASS. A compile error naming a band number is the point of this task —
it means a fourth numeric call site exists that this plan did not find. Fix it in
the same shape and note it in the commit message.

- [ ] **Step 10: Commit**

```bash
git add internal/characters/resources.go internal/characters/resources_test.go internal/characters/toxicity_calibration_test.go internal/users/userrecord.prompt.go internal/hooks/NewRound_AutoHeal.go internal/templates/templatesfunctions.go
git commit -F - <<'MSG'
feat(toxicity): show pressure building before it hurts

The first band sat at 50% of max, so everything from clear to half-poisoned
rendered identically and the prompt token printed nothing at all there. A
player had no way to see toxicity accumulating and no way to learn the
mechanic existed until it was already costing them.

Two bands are added below the first penalty band -- "sour" at 15% and
"unsettled" at 30% -- and both are FEEDBACK ONLY. GetToxicityPenalties and
ToxicitySicknessDamage keep their 50/75/90 thresholds untouched. This changes
what a player can see, never what they suffer.

ToxicityBand and GetToxicityPenalties used to mirror each other exactly and
the comments said so. That relationship is now deliberately broken, and both
functions carry a comment saying why, because "restoring" the symmetry would
silently delete both warning tiers. A test pins the asymmetry.

Band numbering therefore goes 0-3 to 0-5, updating all three numeric call
sites together: the {tox} prompt token, the onset messages, and the status
colors. The three existing onset strings keep their wording and shift to the
tiers they always described.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 4: Toxicity clears on death

**Files:**
- Modify: `internal/hooks/Life_Cascades.go:61-66`
- Test: `internal/hooks/toxicity_death_test.go` (create)

- [ ] **Step 1: Find the package's existing life-transition test harness FIRST**

Before writing the test, find out how this package already drives a life
transition. Do not invent a harness:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
grep -rln "life.Dead" --include=*_test.go internal/
grep -rn "life.Alive\|AfterTransition" internal/hooks/Life_Cascades.go | head
```

Copy the setup from whatever existing test drives `Alive → Dead` verbatim, and
substitute it for `applyLifeCascade` in the next step.

If no such harness exists, the cascade is only reachable through the state
machine's `AfterTransition` observer and is awkward to drive directly. In that
case do **not** build one. Instead assert the invariant at the level the package
does expose, and put the real coverage in the Task 8 playtest: die while toxicity
is above zero, then confirm `status` reads `clear` on respawn. Say which route
you took in the commit message.

- [ ] **Step 2: Write the failing test**

Create `internal/hooks/toxicity_death_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// TestDeathClearsToxicity pins the rule that toxicity is the PRICE OF AN EFFECT,
// so with no effect there is no price. Death already strips every buff, which
// takes away the potion effects the toxicity was paid for; leaving the toxicity
// behind charges the player a second time for a benefit they no longer hold.
// They have also already lost the potions, the materials and the brewing time.
//
// It also closes a death spiral. ToxicitySicknessDamage deals acute HP damage
// above 90% that scales past the cap and can kill, and decay HALVES above 75% --
// slowest exactly where it is most dangerous. Without this clear a player could
// die of toxicity, respawn at 5% health still at critical toxicity, and die
// again with no way out. The alchemy formula sharpened that: a low-alchemy
// character's ceiling is near 33, so the danger band is far easier to reach than
// it was under the old flat 120.
func TestDeathClearsToxicity(t *testing.T) {
	c := &characters.Character{}
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.Toxicity = 95

	// Replace with the package's real harness, found in Step 1.
	applyLifeCascade(t, c, life.Alive, life.Dead)

	if c.Toxicity != 0 {
		t.Errorf("Toxicity = %v after death, want 0", c.Toxicity)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/hooks/ -run TestDeathClearsToxicity -v`

Expected: FAIL with `Toxicity = 95 after death, want 0`.

- [ ] **Step 4: Clear toxicity in the death cascade**

In `internal/hooks/Life_Cascades.go`, the `Alive → Dead` branch currently reads:

```go
				// 5. Buffs (non-permanent) → cancel all.
				c.CancelBuffsWithFlag(buffs.All)

				// 6. Conditions slice → clear.
				c.Conditions = nil
```

Replace that span with:

```go
				// 5. Buffs (non-permanent) → cancel all.
				c.CancelBuffsWithFlag(buffs.All)

				// 5b. Toxicity → clear. THIS MUST STAY BESIDE THE BUFF STRIP
				// ABOVE, because that strip is what justifies it: toxicity is
				// the price of a potion's effect, and the line above has just
				// removed every effect the player paid for. Leaving toxicity
				// behind would charge them twice for a benefit they no longer
				// hold -- they have already lost the potions, the materials and
				// the brewing time. Separating these two lines is what
				// re-creates the bug, and it also reopens a death spiral:
				// toxicity above 90% deals acute HP damage and decays at half
				// speed, so a player could respawn at 5% health still critical
				// and die again with no way out.
				c.Toxicity = 0

				// 6. Conditions slice → clear.
				c.Conditions = nil
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/hooks/ -run TestDeathClearsToxicity -v`

Expected: PASS.

- [ ] **Step 6: Run the package**

Run: `go test ./internal/hooks/`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/Life_Cascades.go internal/hooks/toxicity_death_test.go
git commit -F - <<'MSG'
fix(toxicity): clear toxicity on death

Nothing cleared it. The only two `Toxicity = 0` sites in the codebase are
floor clamps, and death reset Health, Stamina and Conviction to 5% while
leaving toxicity exactly where it was.

Death already strips every buff, so the potion effects the toxicity was paid
for are gone. Toxicity is the price of an effect; with no effect there is no
price. The player has also already lost the potions, the materials and the
brewing time, so leaving the toxicity behind charges them twice.

This also closes a death spiral that the new alchemy formula sharpened.
Toxicity above 90% deals acute HP damage that scales past the cap and can
kill, and decay halves above 75% -- slowest exactly where it is most
dangerous. A player could die of toxicity, respawn at 5% health still at
critical, and die again with no way out. A low-alchemy character's ceiling is
now near 33 rather than 120, so that band is far easier to reach.

The clear sits immediately beside the buff strip that justifies it, with a
comment tying them, because separating them is what re-creates the bug.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 5: Every drinkable costs something, and the heavy tier still fits

Nineteen YAML edits in three groups. `drink.go:167` gates on
`if itemSpec.Toxicity > 0`, so a missing field means `AddToxicity` never fires and
the potion is free.

The heavy-tier rescale is **required, not cosmetic**: `drink.go:150` rejects any
potion that would push you past max. With Meirok's ceiling falling from 130 to
73.2, Chrysalis Catalyst (90) and Mutagen Brew (80) would become permanently
undrinkable. Each new value is the old value times 73.2/130, so every potion keeps
the same relative bite it has today.

**Files (all under `_datafiles/world/dogmud/items/`):**

*Group A — backfill the nine that cost nothing (add a `toxicity:` line):*

| File | Value | Reasoning |
|---|---:|---|
| `consumables-30000/30024-herbal_tea.yaml` | 3 | trivial comfort drink |
| `consumables-30000/30001-small_red_potion.yaml` | 6 | basic heal, deliberately under the alchemy salve's 8 |
| `consumables-30000/30028-minor_antidote.yaml` | 6 | small remedy |
| `consumables-30000/30029-clarity_tonic.yaml` | 8 | low-tier utility |
| `consumables-30000/30012-conviction_draught.yaml` | 8 | **matches its twin 30038** |
| `consumables-30000/30030-fire_resistance_draught.yaml` | 11 | mid-tier ward |
| `consumables-30000/30032-berserker_elixir.yaml` | 25 | **matches its twin 30049** |
| `consumables-30000/30067-catalyst_of_unmaking.yaml` | 34 | sits with the other unmaking-tier brews |
| `materials-40000/40181-phial_of_second_birth.yaml` | 45 | sits with mutagen brew |

*Group B — mid tier 14 → 11 (edit the existing line):*

| File | From | To |
|---|---:|---:|
| `consumables-30000/30039-warriors_brew.yaml` | 14 | 11 |
| `consumables-30000/30040-preachers_tincture.yaml` | 14 | 11 |
| `consumables-30000/30041-windrunner_draught.yaml` | 14 | 11 |

*Group C — heavy tier rescaled to the new ceiling (edit the existing line):*

| File | From | To |
|---|---:|---:|
| `consumables-30000/30056-chrysalis_catalyst.yaml` | 90 | 50 |
| `consumables-30000/30055-mutagen_brew.yaml` | 80 | 45 |
| `consumables-30000/30052-purging_draught.yaml` | 60 | 34 |
| `consumables-30000/30053-essence_of_growth.yaml` | 60 | 34 |
| `consumables-30000/30054-savants_infusion.yaml` | 60 | 34 |
| `materials-40000/40109-ysoldes_purge.yaml` | 45 | 26 |
| `materials-40000/40108-bloom_wafer.yaml` | 35 | 20 |

- [ ] **Step 1: Confirm the starting state**

Run:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
for f in $(grep -rl "^subtype: drinkable" _datafiles/world/dogmud/items/ | sort); do
  id=$(grep -m1 '^itemid:' "$f" | awk '{print $2}')
  nm=$(grep -m1 '^name:' "$f" | cut -d' ' -f2-)
  tx=$(grep -m1 '^toxicity:' "$f" | awk '{print $2}')
  printf "%-6s %-32s %s\n" "$id" "$nm" "${tx:-NONE}"
done
```

Expected: 34 rows, exactly 9 reading `NONE` — the nine in Group A.

- [ ] **Step 2: Apply Group A (the backfill)**

`toxicity:` is a top-level scalar. Append it to each file, matching the existing
placement in a file that already has one (`30039-warriors_brew.yaml:16` sits among
the other top-level scalars, not inside a nested block).

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/world/dogmud/items"
printf '\ntoxicity: 3\n'  >> consumables-30000/30024-herbal_tea.yaml
printf '\ntoxicity: 6\n'  >> consumables-30000/30001-small_red_potion.yaml
printf '\ntoxicity: 6\n'  >> consumables-30000/30028-minor_antidote.yaml
printf '\ntoxicity: 8\n'  >> consumables-30000/30029-clarity_tonic.yaml
printf '\ntoxicity: 8\n'  >> consumables-30000/30012-conviction_draught.yaml
printf '\ntoxicity: 11\n' >> consumables-30000/30030-fire_resistance_draught.yaml
printf '\ntoxicity: 25\n' >> consumables-30000/30032-berserker_elixir.yaml
printf '\ntoxicity: 34\n' >> consumables-30000/30067-catalyst_of_unmaking.yaml
printf '\ntoxicity: 45\n' >> materials-40000/40181-phial_of_second_birth.yaml
```

⚠️ **Appending is only safe if the appended line lands at top level.** A line
appended with no leading blank line can be swallowed into a trailing folded `>-`
description block and become part of the prose instead of a key. The `\n` prefix
above prevents that. Verify in Step 4 rather than assuming.

- [ ] **Step 3: Apply Groups B and C (the retunes)**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/world/dogmud/items"
sed -i 's/^toxicity: 14$/toxicity: 11/' consumables-30000/30039-warriors_brew.yaml
sed -i 's/^toxicity: 14$/toxicity: 11/' consumables-30000/30040-preachers_tincture.yaml
sed -i 's/^toxicity: 14$/toxicity: 11/' consumables-30000/30041-windrunner_draught.yaml
sed -i 's/^toxicity: 90$/toxicity: 50/' consumables-30000/30056-chrysalis_catalyst.yaml
sed -i 's/^toxicity: 80$/toxicity: 45/' consumables-30000/30055-mutagen_brew.yaml
sed -i 's/^toxicity: 60$/toxicity: 34/' consumables-30000/30052-purging_draught.yaml
sed -i 's/^toxicity: 60$/toxicity: 34/' consumables-30000/30053-essence_of_growth.yaml
sed -i 's/^toxicity: 60$/toxicity: 34/' consumables-30000/30054-savants_infusion.yaml
sed -i 's/^toxicity: 45$/toxicity: 26/' materials-40000/40109-ysoldes_purge.yaml
sed -i 's/^toxicity: 35$/toxicity: 20/' materials-40000/40108-bloom_wafer.yaml
```

- [ ] **Step 4: Verify the end state**

Re-run the Step 1 command. Expected: 34 rows, **zero** reading `NONE`, and the two
same-named pairs agreeing:

```
30012  Conviction Draught               8
30038  Conviction Draught               8
30032  Berserker Elixir                 25
30049  Berserker Elixir                 25
```

Then confirm every file still parses and no appended line was swallowed:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git diff --stat _datafiles/world/dogmud/items/     # expect exactly 19 files
git diff _datafiles/world/dogmud/items/ | grep '^+' | grep -v '^+++'
```

Expected: 19 files; every added line is either `toxicity: N` or blank. If any
added line appears indented or inside a description block, fix that file by hand.

- [ ] **Step 5: Confirm nothing became undrinkable**

Run:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
python -c "
mx = 58/2.5 + 150/3   # Meirok, real prod save
print('Meirok max %.1f' % mx)
for n,t in [('Chrysalis Catalyst',50),('Mutagen Brew',45),('Phial of Second Birth',45),
            ('Savant/Essence/Purging/Unmaking',34),(\"Ysolde's Purge\",26),('Bloom Wafer',20)]:
    print('%-34s %3d  %s' % (n, t, 'REJECTED' if t > mx else '%.0f%% of max' % (100*t/mx)))
"
```

Expected: `Meirok max 73.2` and **no** line reading `REJECTED`.

- [ ] **Step 6: Commit**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git add _datafiles/world/dogmud/items/
git commit -F - <<'MSG'
feat(items): every drinkable now costs toxicity, and the heavy tier still fits

Nine of the 34 drinkables carried no toxicity field at all -- the whole
pre-alchemy generation. drink.go gates on `Toxicity > 0`, so those nine were
free, and a player could dodge the mechanic entirely by drinking only old
potions. Worse, two of them share a NAME with a toxic alchemy twin, so
Conviction Draught and Berserker Elixir behaved differently depending on which
copy the matcher happened to pick. Each twin now takes its partner's value
exactly, so which one you grab stops mattering.

Mid tier drops 14 to 11. No ceiling can satisfy both design targets at the old
values: for the fifth low-tier potion and the fourth mid-tier potion to be the
ones that bite, mid must cost about 1.25x low, and 14 vs 8 is 1.75x. That is a
ratio, so moving the ceiling moves both together and never separates them.
Lowering mid rather than raising low keeps the basic healing salve gentle,
which matters most for new players -- they have the least tolerance now.

The heavy tier is rescaled by 73.2/130, the ratio the ceiling itself moved.
This is required, not cosmetic: drink.go REJECTS a potion that would push you
past max, so Chrysalis Catalyst at 90 and Mutagen Brew at 80 would have become
permanently undrinkable for a character who can drink them today. Each new
value preserves that potion's old percentage of max, so the mutation
progression path stays open and nothing a player can drink now becomes
refused.

Merging the two duplicate item IDs is deliberately out of scope -- players own
both, and an item merge is a migration.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 6: Make the six knobs explicit in `config.yaml`

All six run on invisible Go defaults today (`grep -c -i toxicity _datafiles/config.yaml` → 0).

⚠️ **`_datafiles/config.yaml` carries `skip-worktree`.** The working copy holds
local-only `HttpPort`, `LogLevel` and `Playtest` settings that must not be
committed. Build the commit from the `git show HEAD:` blob with only the intended
lines spliced in, and restore the flag afterwards. Do **not** use
`git update-index --cacheinfo` — it clears skip-worktree.

- [ ] **Step 1: Confirm the starting state and find the insertion point**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git ls-files -v _datafiles/config.yaml          # expect a leading 'S' = skip-worktree
grep -c -i toxicity _datafiles/config.yaml       # expect 0
grep -n "CounterDamagePercent\|Bloom" _datafiles/config.yaml | head
```

Note the line number of a nearby combat-balance knob — the new block goes beside
it, inside the same `Balance:` mapping and at the same indentation.

- [ ] **Step 2: Add the block to the working copy**

Insert into `_datafiles/config.yaml`, at the indentation the surrounding
`Balance:` keys use (two spaces in the current file):

```yaml
  # ── TOXICITY ──────────────────────────────────────────────────────────────
  # Tolerance is EARNED BY BREWING, not by being tough:
  #   max = ToxicityBaseMax + alchemy/AlchemyScale + Vitality/VitalityScale
  # A veteran who never touched a still is less tolerant than a mid-level
  # dabbler. Fitted so the fifth low-tier potion is the one that first costs a
  # skilled alchemist something; internal/characters/toxicity_calibration_test.go
  # pins that fit, so editing these two scales will fail that test on purpose.
  ToxicityBaseMax: 0            # flat floor; 0 = tolerance is entirely earned
  ToxicityAlchemyScale: 2.5     # divisor on alchemy skill
  ToxicityVitalityScale: 3      # divisor on vitality
  ToxicityDecayPerTick: 1.0     # unchanged from the Go default
  ToxicitySicknessDamagePct: 0.02
  ToxicityHighDecaySlowMult: 0.5
```

- [ ] **Step 3: Verify it parses and the unit tests still pass**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
go test ./internal/configs/ -run TestToxicity -v
grep -n -A 8 "ToxicityBaseMax" _datafiles/config.yaml
```

Expected: PASS, and the block present at the right indentation. The live
confirmation that the zero survives a real load comes at Task 8 Step 5 — a unit
test cannot prove it, because test binaries never read `config.yaml`.

- [ ] **Step 4: Build the committed version from HEAD's blob**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"

# Keep the local working copy safe before touching anything.
cp _datafiles/config.yaml /tmp/config-local.yaml

# Start from HEAD's blob, which has none of the local HttpPort / LogLevel /
# Playtest settings, and splice in ONLY the toxicity block from Step 2.
git show HEAD:_datafiles/config.yaml > /tmp/config-commit.yaml
```

Open `/tmp/config-commit.yaml` and insert the same block from Step 2 at the same
place. Then prove nothing else moved:

```bash
diff <(git show HEAD:_datafiles/config.yaml) /tmp/config-commit.yaml
```

Expected: **only** the toxicity block as added lines. If anything else appears,
stop and redo the splice — a stray line here leaks local settings to production.

- [ ] **Step 5: Commit the blob version, then restore the working copy**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git update-index --no-skip-worktree _datafiles/config.yaml
cp /tmp/config-commit.yaml _datafiles/config.yaml
git add _datafiles/config.yaml
git commit -F - <<'MSG'
config: make the six toxicity knobs explicit

All six were absent, so the subsystem ran entirely on invisible Go defaults
and nobody reading config.yaml could tell what the game actually did.

ToxicityBaseMax ships at 0 -- tolerance is earned entirely from alchemy and
vitality. That value was unreachable until the validator guard was fixed: the
old `<= 0 -> 100` rewrote it on every load and silently restored a flat
ceiling.

AlchemyScale 2.5 and VitalityScale 3 are fitted against Meirok's real prod
save so the fifth low-tier potion is the one that first costs him something.
toxicity_calibration_test.go pins that fit, so editing either scale fails a
test on purpose rather than quietly moving where the cost lands.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG

cp /tmp/config-local.yaml _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
```

⚠️ The last two lines are **not optional**. Restoring the local copy before
re-setting the flag is the order that works; reversing it leaves the local
settings staged for the next commit.

`/tmp/config-local.yaml` was captured in Step 4, *after* Step 2 added the toxicity
block, so the restored working copy carries both the new knobs and the local-only
settings. That is what the Task 8 server boots.

- [ ] **Step 6: Verify the flag came back and nothing leaked**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git ls-files -v _datafiles/config.yaml            # expect a leading 'S'
git status --porcelain _datafiles/config.yaml     # expect NO output
grep -n "HttpPort\|LogLevel" _datafiles/config.yaml   # local settings still present
grep -c Toxicity _datafiles/config.yaml           # expect 6 in the local copy too
git show HEAD:_datafiles/config.yaml | grep -c Toxicity   # expect 6 in the commit

# The committed HttpPort / LogLevel must still be whatever HEAD~1 had, NOT the
# local values. This diff is the real proof and must show only toxicity lines:
git diff HEAD~1 HEAD -- _datafiles/config.yaml
```

If the `S` flag is missing, re-run `git update-index --skip-worktree
_datafiles/config.yaml` before doing anything else — leaving it off will leak the
local settings into a later commit.

---

## Task 7: Make the mechanic findable and the helpfile true

`help toxicity` already resolves (`GetHelpContents` processes `help/<name>`
directly), but the topic is **absent from the `help` index**, has no aliases, and
its page describes the old vitality-only formula. `alchemy.template` never
mentions toxicity at all.

⚠️ **Help templates parse LAZILY and template function names resolve at PARSE
time**, so a typo passes both `go build` and boot and reaches a player as
`[TEMPLATE ERROR]`. Every template edit here must be checked by **rendering the
page**, not by booting.

**Files:**
- Modify: `_datafiles/world/dogmud/keywords.yaml` (two places)
- Modify: `_datafiles/world/dogmud/templates/help/toxicity.template`
- Modify: `_datafiles/world/dogmud/templates/help/alchemy.template`
- Modify: `_datafiles/world/dogmud/templates/help/craft.template`
- Modify: `_datafiles/world/dogmud/templates/help/health.template`

- [ ] **Step 1: Register the topic in the help index**

In `_datafiles/world/dogmud/keywords.yaml`, the `character:` list (starting line
19) is alphabetical and currently ends `- status`. Insert `toxicity` after it:

```yaml
      - status
      - toxicity
```

Check the surrounding entries first — if `status` is not the last item, keep the
list alphabetical rather than appending.

- [ ] **Step 2: Add the aliases**

In the same file, in the alias map (the block containing `craft:` at line 254),
add beside the other crafting entries:

```yaml
  toxicity:         [toxic, poison, poisoned, tolerance]
```

- [ ] **Step 3: Rewrite the helpfile**

Replace the whole of `_datafiles/world/dogmud/templates/help/toxicity.template`
with the following. Prose is hard-wrapped under 80 columns, carries no raw
numbers, and uses no en or em dashes.

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="skill">toxicity</ansi>

Every potion you drink leaves something behind. Toxicity is what
builds up, and how much your body will take is your tolerance.

<ansi fg="yellow">━━━ Tolerance Is Earned ━━━</ansi>

  Your tolerance comes mostly from <ansi fg="command">alchemy</ansi>. Working with
  reagents, handling fumes and tasting your own brews teaches
  your body to cope with them. A brewer of middling skill can
  drink more than a far tougher fighter who has never touched
  a still.

  Vitality helps too, but it helps less. Being strong is not
  the same as being used to it.

<ansi fg="yellow">━━━ How It Works ━━━</ansi>

  Drinking a potion adds its toxicity to your total.
  Toxicity fades on its own, a little at a time.
  If a potion would take you past your limit, your body
  refuses it and the potion is not consumed.
  Dying clears your toxicity completely, along with every
  potion effect you were carrying.

<ansi fg="yellow">━━━ What You Will Feel ━━━</ansi>

  Watch the <ansi fg="command">status</ansi> screen, or add the toxicity tag to your
  prompt with <ansi fg="command">set-prompt</ansi>. It moves through these stages:

  <ansi fg="green">clear</ansi>       nothing to speak of
  <ansi fg="black-bold">sour</ansi>        a faint taste you cannot wash away
  <ansi fg="white">unsettled</ansi>   your stomach is uneasy

  Neither of those two costs you anything. They are a warning,
  so you can see it coming.

  <ansi fg="yellow">queasy</ansi>      senses dull, recovery slows
  <ansi fg="red">sick</ansi>        reflexes suffer as well
  <ansi fg="red-bold">critical</ansi>    recovery and awareness both fail badly,
              and the poison starts doing real harm

  At <ansi fg="red-bold">critical</ansi> the toxicity also clears more slowly, so it
  is far easier to get into than out of. Do not sit there.

<ansi fg="yellow">━━━ Spoiled Potions ━━━</ansi>

  Drinking a spoiled potion inflicts triple the normal toxicity
  and causes nausea. Skilled alchemists can detect spoiled brews
  by examining them, so look for the telltale signs.

<ansi fg="yellow">━━━ Purging ━━━</ansi>

  The purging draught clears all active potion effects and all
  toxicity, but leaves you weakened afterward.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help alchemy</ansi>, <ansi fg="command">help drink</ansi>, <ansi fg="command">help craft</ansi>,
  <ansi fg="command">help health</ansi>, <ansi fg="command">help status</ansi>
```

- [ ] **Step 4: Cross-link from alchemy**

In `_datafiles/world/dogmud/templates/help/alchemy.template`, add a section
immediately before the existing `See also:` footer:

```
<ansi fg="yellow">━━━ Tolerance ━━━</ansi>

  Practising alchemy raises your tolerance for the potions you
  drink. The more skilled a brewer you become, the more you can
  take before the toxicity starts to tell on you. See
  <ansi fg="command">help toxicity</ansi>.
```

Then extend that file's `See also:` list. Its last line currently reads:

```
  <ansi fg="command">help foraging</ansi>, <ansi fg="command">help skills</ansi>
```

Change it to:

```
  <ansi fg="command">help foraging</ansi>, <ansi fg="command">help skills</ansi>, <ansi fg="command">help toxicity</ansi>
```

- [ ] **Step 5: Cross-link from craft and health**

In `_datafiles/world/dogmud/templates/help/craft.template`, change the final line
of the `See also:` block from:

```
  <ansi fg="command">help foraging</ansi>, <ansi fg="command">help skills</ansi>, <ansi fg="command">help inventory</ansi>
```

to:

```
  <ansi fg="command">help foraging</ansi>, <ansi fg="command">help skills</ansi>, <ansi fg="command">help inventory</ansi>,
  <ansi fg="command">help toxicity</ansi>
```

In `_datafiles/world/dogmud/templates/help/health.template`, the `See also:` block
currently ends:

```
  <ansi fg="command">help armor</ansi>
```

Change it to:

```
  <ansi fg="command">help armor</ansi>
  <ansi fg="command">help toxicity</ansi>
```

- [ ] **Step 6: Render every edited page rather than trusting the boot**

Start the server (Task 8's worktree binary is fine) and, in game, run each of:

```
help toxicity
help toxic
help poison
help tolerance
help alchemy
help craft
help health
help
```

Expected:
- All four toxicity spellings render the same page.
- No page shows `[TEMPLATE ERROR]` anywhere.
- `help` with no arguments now **lists** `toxicity` under the character group.
- The band words in the toxicity page render in their colors, matching what
  `status` shows.

⚠️ A boot test will **not** catch a template typo here. Only rendering will.

- [ ] **Step 7: Commit**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git add _datafiles/world/dogmud/keywords.yaml _datafiles/world/dogmud/templates/help/
git commit -F - <<'MSG'
docs(help): make toxicity findable, and true

The page existed and resolved, but nothing pointed at it: the topic was
absent from the help index, had no aliases, and the alchemy page never
mentioned toxicity at all. In practice the mechanic was undocumented.

toxicity is now listed under the character topics and answers to toxic,
poison, poisoned and tolerance. alchemy, craft and health all link to it, and
alchemy gains a short section saying that brewing is what raises tolerance,
since that is the least guessable part of the model.

The page itself described the old vitality-only formula. It now names alchemy
as the tolerance source, walks the bands in words, says plainly that the two
lowest ones cost nothing, warns that critical clears more slowly than it
fills, and tells the player that dying clears it.

Every edited page was checked by rendering it in game. Help templates parse
lazily, so a typo passes build and boot and reaches a player as a template
error.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 8: Full verification, patch notes, and the content playtest gate

- [ ] **Step 1: Format and build**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
gofmt -l internal/ modules/     # must print NOTHING
go build ./...
```

- [ ] **Step 2: Run every touched package**

```bash
go test ./internal/configs/ ./internal/characters/ ./internal/hooks/ ./internal/users/ ./internal/templates/ ./internal/usercommands/ ./modules/gmcp/
```

Expected: PASS everywhere. `modules/gmcp` is included because
`gmcp.Char.go:542` ships `ToxicityBandName()` to the web client and its test
asserts on the string.

- [ ] **Step 3: Run the full suite**

```bash
go test ./...
```

Expected: PASS. Note the recorded flake: roughly 2.3% of attack rolls fumble and
always miss, so a combat test failing once is worth re-running before
investigating.

- [ ] **Step 4: Wipe instance saves, then boot-test in an isolated worktree**

Stale instance saves shadow template edits, and the server must be **down** for
the rooms wipe.

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
rm -rf _datafiles/world/dogmud/mobs.instances _datafiles/world/dogmud/rooms.instances

git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code 124 is the **success** case: the timeout fired because the server stayed
up. Do not grep for the bare word `panic` — `GamePlay.MapConsistencyEnforce`
legitimately has the *value* `panic`.

Clean up: `git worktree remove --force C:/tmp/dogmud-boot-check` (if Windows holds
a lock, `rm -rf` then `git worktree prune`).

- [ ] **Step 5: Confirm the blocker is actually dead in a real load**

The whole plan turns on `ToxicityBaseMax: 0` surviving a live config load. A unit
test is not proof, because test binaries never read `config.yaml`.

In game on a **fresh** character (no alchemy, ordinary vitality), run `status`.
Toxicity must read `clear`. Then drink two low-tier potions and check `status`
again: a fresh character's ceiling is near 33, so two salves should already read
`sick`. If two salves still read `clear`, the ceiling is still flat at 120 and the
guard fix did not reach the running config — stop and re-check Task 1 and Task 6
before going further.

Then verify the death clear the same way: get toxicity above zero, die, and
confirm `status` reads `clear` on respawn.

- [ ] **Step 6: Write the patch notes entry**

Add to the top of `docs/PATCH_NOTES.md`, below the `# DOGMud Patch Notes` heading
and above the current first entry. Player-facing framing, no raw numbers, no em
dashes:

```markdown
## 2026-08-31: Potions ask something of you now

Drinking potion after potion used to cost you nothing worth noticing. Some of the
older brews were free outright, and even the ones that were not took so much to
build up that most people never saw the mechanic at all. Two potions that shared
a name did not even agree with each other about the price.

Every drinkable now has a cost, the two same-named pairs agree, and how much you
can take before it tells on you comes mostly from alchemy rather than from being
tough. Working with reagents teaches your body to cope with them, so a brewer of
middling skill can drink more than a far hardier fighter who has never touched a
still. Practising the skill raises what you can handle.

You can also see it building now. There are two new stages below anything that
actually hurts you, so a faint sourness and an unsettled stomach turn up as a
warning long before your hands start shaking. Neither of them costs you a thing.
They show on your status screen and in your prompt, and the toxicity help page
explains the whole thing, which it was not really doing before.

Dying now clears toxicity as well. Death already strips every potion effect you
were carrying, so keeping the price of effects you no longer have was charging
you twice for the same drink.
```

- [ ] **Step 7: Commit the patch notes**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git add docs/PATCH_NOTES.md
git commit -F - <<'MSG'
docs: patch notes for the toxicity tolerance and feedback change

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

- [ ] **Step 8: Adversarial playtest — REQUIRED, not optional**

This task authors player-facing content (help pages, band words, onset messages,
patch notes), so the project's Content Playtest-Review Gate applies. Boot-clean
verifies the system, never the experience.

Run the harness with an explicitly critical mandate:

```text
/playtest local --checkout C:/Users/Calabe Davis/workspace/DOGMud bug-finder 2026-08-03-prepush-sweep.yaml
```

Drive the real player flow end to end and read **every** line of output. The
things this change can plausibly get wrong, none of which a boot test sees:

- Do the band words read naturally in the prompt, or does `unsettled` crowd it?
- Does `status` still line up? The band name is padded into a 13-wide column and
  a border that shifts by one character is very visible.
- Do the onset messages fire once per crossing, or repeat every tick?
- Does the relief message fire on **every** downward crossing, including the two
  new ones, and does it read oddly when stepping down from `unsettled` to `sour`
  where nothing was ever wrong?
- Does the rejection message ("Your body rejects the potion") now fire often
  enough on a low-tolerance character to feel punishing rather than informative?
- Do all four help aliases work, and does any page show `[TEMPLATE ERROR]`?

Fix what it finds, re-run if needed, and only then hand it to the owner.

- [ ] **Step 9: Ship through a PR**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git push -u origin HEAD
gh pr create --repo pruuk/DOGMud --base master --head "$(git branch --show-current)" --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ Always pass `--repo pruuk/DOGMud`. This repo is a fork and `gh` defaults to the
**upstream parent**; a bare `gh pr create` has opened a PR against upstream before.

A green check is not proof on its own. Confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed` before merging, then:

```bash
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

Use `--merge`, not `--squash` — the per-commit messages carry the evidence and
reasoning that a squash would flatten.

---

## Done when

Mapped to the spec's own acceptance list, plus the two items this plan added.

1. All 34 drinkables carry a toxicity value, and the two same-named pairs agree — Task 5 Step 4.
2. `GetToxicityMax` reads alchemy, and Meirok's real save yields the fifth low-tier potion as the one that bites — Task 2, pinned by `TestToxicityCalibrationTable`.
3. Two feedback bands exist below the first penalty band, and no penalty threshold has moved — Task 3, pinned by `TestToxicityBandsAreFinerThanPenalties`.
4. Toxicity is zero after death, pinned by a test, and the clear sits beside the buff strip it is justified by — Task 4.
5. `help toxicity` resolves *and is listed*, and alchemy, craft and health link to it — Task 7, verified by rendering.
6. Boot clean, and a test pins the calibration table so a later knob edit cannot silently move where the fifth potion lands — Task 8 Step 4, Task 2.
7. ➕ **The explicit `ToxicityBaseMax: 0` survives a real config load** — Task 1, confirmed in play at Task 8 Step 5. Without this the entire model silently reverts.
8. ➕ **No potion that is drinkable today becomes undrinkable** — Task 5 Step 5.

## Out of scope

- Merging the duplicate potion items (30012/30038, 30032/30049). Players own both; that is a migration.
- Changing the penalty thresholds or the acute damage curve. Feedback only.
- Retuning decay. `ToxicityDecayPerTick` stays at 1.0; the model is fitted to a burst of potions, and changing both at once would make neither measurable.
- Replacing the hardcoded `ysoldesPurgeItemId` bypass at `drink.go:33` with a real ItemSpec field. The rescale in Task 5 removes the need for it here.
