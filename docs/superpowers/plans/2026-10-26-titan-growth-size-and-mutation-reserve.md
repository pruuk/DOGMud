# #680 Titan Growth Size and Mutation Reserve Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Titan Growth 3 makes a character Large (two-hander in the weapon hand; every hand with Colossus Form), and Titan Growth, Colossus Form and Extra Arms reserve Conviction in units under one eligibility rule for every mutation grant.

**Architecture:** `mutations` gains reserve units, a derived unit budget, a `size` effect and one `CanGain` filter that every pool and deepening pick uses. `characters` gains one size reader (`Size()`), a slot-aware `HandsRequiredAt`, and the body as a reservation source. `hooks` releases companions newest first when a gained rank passes the ceiling, on a `mutations.Gained` listener and at login.

**Tech Stack:** Go, testify, YAML world data.

**Spec:** `docs/superpowers/specs/2026-10-26-titan-growth-size-and-mutation-reserve-design.md` (approved 2026-10-26).

---

## Rules for every task

- Edit files with the Edit and Write tools only. Never rewrite a file from the shell (`sed -i`, Python `open(..., 'w')`, `>` into a tracked file). Hooks block it.
- `git add <named paths>` only. Never `-A`, `.`, `--all`, `commit -a`, `--no-verify`.
- Commit messages through a quoted heredoc, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Test binaries load Go defaults, not `config.yaml`. Every knob here ships the same value in both.
- No em or en dashes in Go comments, player copy or docs.
- `gofmt -l .` over the whole repo before every commit; it must print nothing.

## Facts verified against source (master `6521e0f8c`, 2026-10-26)

| # | Fact | Where |
|---|---|---|
| F1 | `PoolReservationCapPct ConfigFloat` declared | `internal/configs/config.balance.go:679` |
| F2 | Its defaulter (`<= 0 \|\| > 1` becomes 0.66) | `internal/configs/config.balance.spells.go:56-61`, inside `func (b *Balance) validateSpells()` |
| F3 | Shipped `PoolReservationCapPct: 0.66` | `_datafiles/config.yaml:2029` |
| F4 | `MutationEffect{Type, Target string; Value float64}`; `MutationSpec` fields end at `Prerequisites` | `internal/mutations/mutations.go:23-75` |
| F5 | `MutationSpec.Validate()` ends `return nil` after the legacy Con migration | `internal/mutations/mutations.go:87-130` |
| F6 | `GetWeightedPoolWithFloor` checks owned, conflict, body parts, moon; NOT prerequisites | `internal/mutations/mutations.go:262-297` |
| F7 | `GetGraphPool` checks owned, conflict, body parts, `PrereqsMet`, moon, then affinity | `internal/mutations/affinity.go:78-115` |
| F8 | `effectiveMax(id, globalMax)`, `CanDeepen(owned)`, `RollDeepening(owned)` | `internal/mutations/mutations.go:366-398` |
| F9 | `HasMutationFlag(owned, flag)`; `PrereqsMet(owned, spec)` | `mutations.go:533`; `graph.go:92` |
| F10 | `SeedMutationsForTest(map[string]*MutationSpec) func()` | `internal/mutations/test_helpers.go:6` |
| F11 | `DescribeEffect` switch ends `case "flag": return flagPhrase(e.Target)`; `flagPhrase` switch | `internal/mutations/describe.go:104-152` |
| F12 | `HandsRequired(i)`: shooting returns `Hands`; nil species returns `Hands`; Large returns 1; Small `Hands+1` | `internal/characters/worn.go:278-308` |
| F13 | `HandsRequired` callers: `hand_slots.go:98` (`Is2H`), `wear_slot.go:81,170,281,285`, `worn.go:512` | grep |
| F14 | `HandSlot{Label string; ItemPtr *items.Item}`; labels `wielded`, `offhand`, `extra arm 1..4` | `internal/characters/hand_slots.go:27-52` |
| F15 | `IsBlockedBy2H` builds a partner map and calls `HandSlot{ItemPtr: partner}.Is2H(c)` | `hand_slots.go:101-118` |
| F16 | `handCandidates` two-hander branch; `chooseArm` computes `twoHanded` before `target` | `internal/characters/wear_slot.go:163-185, 266-300` |
| F17 | Golden fixtures: `seedGoldenSpecies`, `goldenChar(extraArms, speciesId, dual)`, `goldenItem`, `goldenGreat`, `goldenSword`, `goldenMedium/Small/Large` | `internal/characters/wear_slot_golden_test.go:231-282` |
| F18 | `ChanceToTame` has no callers; the only other species size reader | `internal/combat/calculations.go:103-165` |
| F19 | `GetPoolReservation(pool, poolMax)` sums items, then Conviction companions | `internal/characters/validate.go:260-284` |
| F20 | `reserveSources`, `subject()`, `verb()`, `remedies()`, `ReservationRefusal` | `internal/characters/reservation.go:211-318, 672-688` |
| F21 | `poolMax(PoolConviction)` returns `c.ConvictionMax.Value` | `internal/characters/pools.go:256-266` |
| F22 | Player tick resets `MutationProgress = 0` BEFORE choosing deepen or acquire; acquire uses `GetGraphPool` | `internal/hooks/NewRound_UserRoundTick.go:440-510` |
| F23 | Mob tick resets progress before choosing; `applyAcquiredMutation(mob, id)` | `internal/hooks/NewRound_MobRoundTick.go:320-395` |
| F24 | `BloomAdvanceMutation` uses the global max only; `BloomSeedNewMutation` skips prerequisites | `internal/characters/bloom_mutation.go:35-160` |
| F25 | Bloom and Phial in `drink.go:424-470` emit no `mutations.Gained`; `drink.go` does not import `events` | `internal/actions/drink.go` |
| F26 | `mutations.Gained{UserId, MutationId, Rank, IsNew}`; listener `MutationGained_Reveal` registered `hooks.go:35` | `internal/mutations/reveal.go`; `internal/hooks/MutationGained_Reveal.go:110` |
| F27 | Login: `refreshCompanionReservesOnLogin(user)` runs BEFORE companions are spawned | `internal/hooks/PlayerSpawn_HandleJoin.go:77-82` |
| F28 | Companion release order (record before aggro), `RemoveCharm`, `TrackCharmed(id,false)`, `RemoveCompanion`, `mob.Command("despawn")` | `internal/usercommands/dismiss.go:90-160` |
| F29 | Guards run from the repo root (`TestMain` chdir); `repoRoot(t)` | `internal/guards/testmain_test.go`; `context.md:43-61` |
| F30 | `loadFightWorld(t)` loads mutations, species, items, mobs | `internal/guards/fight_fixtures_test.go:23` |
| F31 | Homunculus preset `{"titan-growth": 2}` | `internal/hooks/chrysifier_homunculus.go:157` |
| F32 | `extra-arms.yaml` `max_rank: 1`; text "four flailing limbs"; `character.go:310` comment "(0-2)" | data, code |
| F33 | `help species` Large line | `_datafiles/world/dogmud/templates/help/species.template:15-16` |

## File map

- Create `internal/mutations/reserve.go`: units, budget, `GrowsLarge`.
- Create `internal/mutations/eligibility.go`: `CanGain`, `MaxRankOf`, `RelatedFit`.
- Create `internal/characters/size.go`: `Size()`, `gripsInEveryHand()`.
- Create `internal/hooks/mutation_reserve_release.go`: release companions over the ceiling.
- Create tests: `internal/configs/config.balance.mutation_reserve_test.go`, `internal/mutations/reserve_test.go`, `internal/mutations/eligibility_test.go`, `internal/characters/size_hand_rule_test.go`, `internal/characters/mutation_reserve_test.go`, `internal/hooks/mutation_reserve_release_test.go`, `internal/guards/species_size_reader_guard_test.go`, `internal/guards/mutation_reserve_data_guard_test.go`.
- Modify as each task lists.

---

### Task 1: The `MutationReserveUnitPct` knob

**Files:**
- Modify: `internal/configs/config.balance.go:679`
- Modify: `internal/configs/config.balance.spells.go:56-61`
- Modify: `_datafiles/config.yaml:2029`
- Test: `internal/configs/config.balance.mutation_reserve_test.go`

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// #680: one mutation reserve unit as a fraction of max Conviction. Absent or
// out of range falls back to 0.11, so six units fill the 0.66 ceiling.
func TestMutationReserveUnitPctDefaults(t *testing.T) {
	for _, tc := range []struct {
		in, want float64
	}{
		{0, 0.11},
		{-1, 0.11},
		{0.9, 0.11}, // above the ceiling: no rank could ever fit
		{0.10, 0.10},
	} {
		b := Balance{PoolReservationCapPct: 0.66, MutationReserveUnitPct: ConfigFloat(tc.in)}
		b.validateSpells()
		if float64(b.MutationReserveUnitPct) != tc.want {
			t.Errorf("in %v: got %v, want %v", tc.in, b.MutationReserveUnitPct, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/configs/ -run TestMutationReserveUnitPctDefaults`
Expected: build failure, `b.MutationReserveUnitPct undefined`.

- [ ] **Step 3: Declare the knob** after F1:

```go
	MutationReserveUnitPct ConfigFloat `yaml:"MutationReserveUnitPct"` // One mutation reserve unit (#680) as a fraction of max Conviction (default 0.11)
```

- [ ] **Step 4: Default it** right after the `PoolReservationCapPct` block (F2):

```go
	if b.MutationReserveUnitPct <= 0 || b.MutationReserveUnitPct > b.PoolReservationCapPct {
		// 0.11 (#680): six units fill the 0.66 ceiling. A unit wider than the
		// ceiling would make every reserving rank unreachable.
		b.MutationReserveUnitPct = 0.11
	}
```

- [ ] **Step 5: Ship it in `config.yaml`** right after `PoolReservationCapPct: 0.66` (F3):

```yaml

  # One mutation reserve unit (#680), as a fraction of max Conviction.
  # Titan Growth, Colossus Form and Extra Arms set aside units per rank
  # (reserve_units in each mutation file). The unit budget is
  # PoolReservationCapPct / MutationReserveUnitPct rounded down: 6 at the
  # shipped values, so Titan 3 + Arms 3 and Titan 4 + Colossus + Arms 1 sit
  # exactly at the ceiling. 0 or wider than the ceiling falls back to 0.11.
  MutationReserveUnitPct: 0.11
```

- [ ] **Step 6: Run the test and the config package**

Run: `go test ./internal/configs/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.spells.go internal/configs/config.balance.mutation_reserve_test.go _datafiles/config.yaml
git commit -F - <<'EOF'
feat(config): MutationReserveUnitPct knob for mutation Conviction reserve (#680)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Reserve units, the budget and the size effect in `mutations`

**Files:**
- Modify: `internal/mutations/mutations.go` (struct F4, `Validate` F5)
- Create: `internal/mutations/reserve.go`
- Modify: `internal/mutations/describe.go` (F11)
- Test: `internal/mutations/reserve_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package mutations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedColossus seeds the three reserving mutations as #680 ships them, plus
// a same-cluster mutation with no units and an unrelated one.
func seedColossus(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedMutationsForTest(map[string]*MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 1},
			Pros: []MutationEffect{{Type: "size", Target: "large", Value: 3}}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			Clusters: []string{"colossus", "ravener"}, ReserveUnits: []int{1, 1, 1, 2}},
		"colossus-form": {MutationId: "colossus-form", Name: "Colossus Form", Rarity: 9, MaxRank: 1,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1},
			Prerequisites: []MutationPrereq{{Id: "dense-muscles", MinLevel: 1}, {Id: "titan-growth", MinLevel: 4}},
			Pros:          []MutationEffect{{Type: "flag", Target: "colossal-grip", Value: 1}}},
		"dense-muscles": {MutationId: "dense-muscles", Name: "Dense Muscles", Rarity: 3,
			Clusters: []string{"colossus"}},
		"padded-soles": {MutationId: "padded-soles", Name: "Padded Soles", Rarity: 3,
			Clusters: []string{"stalker"}},
	}))
}

func TestReserveUnitBudgetIsSixAtShippedKnobs(t *testing.T) {
	assert.Equal(t, 6, ReserveUnitBudget())
}

func TestOwnedReserveUnits(t *testing.T) {
	seedColossus(t)
	for _, tc := range []struct {
		owned map[string]int
		want  int
	}{
		{map[string]int{"titan-growth": 3, "extra-arms": 3}, 6},
		{map[string]int{"titan-growth": 4, "colossus-form": 1, "extra-arms": 1}, 6},
		{map[string]int{"extra-arms": 4}, 5},
		{map[string]int{"titan-growth": 3, "extra-arms": 2}, 5},
		{map[string]int{"dense-muscles": 4, "padded-soles": 2}, 0},
	} {
		assert.Equal(t, tc.want, OwnedReserveUnits(tc.owned), "%v", tc.owned)
	}
}

func TestFitsReserveBudget(t *testing.T) {
	seedColossus(t)
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 2}, "extra-arms", 3))
	assert.False(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 3}, "titan-growth", 4))
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 4, "colossus-form": 1}, "extra-arms", 1))
	assert.False(t, FitsReserveBudget(map[string]int{"extra-arms": 3, "titan-growth": 2}, "extra-arms", 4),
		"Arms 4 + Titan 2 is seven units")
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 3}, "dense-muscles", 2),
		"a rank with no units always fits")
}

func TestGrowsLargeAtTitanThree(t *testing.T) {
	seedColossus(t)
	assert.False(t, GrowsLarge(map[string]int{"titan-growth": 2}))
	assert.True(t, GrowsLarge(map[string]int{"titan-growth": 3}))
	assert.True(t, GrowsLarge(map[string]int{"titan-growth": 4}))
	assert.False(t, GrowsLarge(map[string]int{"extra-arms": 4}))
}

func TestValidateRejectsBadReserveAndSize(t *testing.T) {
	neg := &MutationSpec{MutationId: "x", Name: "X", Rarity: 1, ReserveUnits: []int{1, -1}}
	assert.Error(t, neg.Validate())
	badSize := &MutationSpec{MutationId: "y", Name: "Y", Rarity: 1,
		Pros: []MutationEffect{{Type: "size", Target: "huge", Value: 3}}}
	assert.Error(t, badSize.Validate())
	zeroRank := &MutationSpec{MutationId: "z", Name: "Z", Rarity: 1,
		Pros: []MutationEffect{{Type: "size", Target: "large", Value: 0}}}
	assert.Error(t, zeroRank.Validate())
}

func TestDescribeSizeAndGrip(t *testing.T) {
	assert.NotEmpty(t, DescribeEffect(MutationEffect{Type: "size", Target: "large", Value: 3}))
	assert.NotEmpty(t, DescribeEffect(MutationEffect{Type: "flag", Target: "colossal-grip", Value: 1}))
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/mutations/ -run 'ReserveUnit|FitsReserve|GrowsLarge|BadReserve|SizeAndGrip'`
Expected: build failure, `unknown field ReserveUnits`.

- [ ] **Step 3: Add the field** to `MutationSpec`, after `Prerequisites` (F4):

```go
	// ReserveUnits is the Conviction this mutation sets aside, in units of
	// MutationReserveUnitPct, one entry per rank (#680). Rank N holds the sum
	// of the first N entries. Empty reserves nothing. A guard requires the
	// list to be as long as the mutation's max rank.
	ReserveUnits []int `yaml:"reserve_units,omitempty"`
```

- [ ] **Step 4: Validate it.** In `Validate()`, immediately before the final `return nil` (F5):

```go
	for i, u := range m.ReserveUnits {
		if u < 0 {
			return fmt.Errorf("mutation %q: reserve_units[%d] must be >= 0, got %d", m.MutationId, i, u)
		}
	}
	for _, p := range m.Pros {
		if p.Type == "size" && (p.Target != "large" || p.Value < 1) {
			return fmt.Errorf("mutation %q: a size effect needs target large and a rank value of 1 or more", m.MutationId)
		}
	}
```

- [ ] **Step 5: Create `internal/mutations/reserve.go`**

```go
package mutations

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// ReserveUnitsAt returns the reserve units this mutation holds at the given
// rank (#680): the sum of the first `rank` entries of ReserveUnits.
func (s *MutationSpec) ReserveUnitsAt(rank int) int {
	total := 0
	for i := 0; i < rank && i < len(s.ReserveUnits); i++ {
		total += s.ReserveUnits[i]
	}
	return total
}

// OwnedReserveUnits returns the units every owned mutation holds at its rank.
func OwnedReserveUnits(owned map[string]int) int {
	total := 0
	for id, level := range owned {
		if spec := GetMutation(id); spec != nil {
			total += spec.ReserveUnitsAt(level)
		}
	}
	return total
}

// ReserveUnitBudget is how many units fit under the reservation ceiling. It is
// derived from PoolReservationCapPct and MutationReserveUnitPct, never a knob
// of its own, so the two can never disagree. The epsilon keeps 0.66 / 0.11
// from flooring to 5 on float error.
func ReserveUnitBudget() int {
	b := configs.GetBalanceConfig()
	unit := float64(b.MutationReserveUnitPct)
	if unit <= 0 {
		return 0
	}
	return int(math.Floor(float64(b.PoolReservationCapPct)/unit + 1e-9))
}

// FitsReserveBudget reports whether owning `id` at newRank keeps the owned
// units within ReserveUnitBudget. Gear and companions never count here: the
// budget is the body's alone (owner, 2026-10-26).
func FitsReserveBudget(owned map[string]int, id string, newRank int) bool {
	spec := GetMutation(id)
	if spec == nil {
		return true
	}
	units := OwnedReserveUnits(owned) - spec.ReserveUnitsAt(owned[id]) + spec.ReserveUnitsAt(newRank)
	return units <= ReserveUnitBudget()
}

// GrowsLarge reports whether an owned mutation's size effect has reached the
// rank it names (Titan Growth: 3). A size effect is a threshold, so it is
// read here and never through sumEffects or the level curve.
func GrowsLarge(owned map[string]int) bool {
	for id, level := range owned {
		spec := GetMutation(id)
		if spec == nil {
			continue
		}
		for _, p := range spec.Pros {
			if p.Type == "size" && p.Target == "large" && level >= int(p.Value) {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 6: Describe the new effects.** In `DescribeEffect` add before `case "flag":`

```go
	case "size":
		return "At its full growth you are large enough to wield a two-handed weapon in your weapon hand alone."
```

and in `flagPhrase` add beside `case "control-immune":`

```go
	case "colossal-grip":
		return "Every hand you have can wield a two-handed weapon alone."
```

- [ ] **Step 7: Run the package**

Run: `go test ./internal/mutations/`
Expected: PASS (existing tests seed no units, so nothing else moves).

- [ ] **Step 8: Commit**

```bash
git add internal/mutations/mutations.go internal/mutations/reserve.go internal/mutations/describe.go internal/mutations/reserve_test.go
git commit -F - <<'EOF'
feat(mutations): reserve units, derived unit budget and size effect (#680)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: One eligibility rule, `CanGain`, for every pool and deepening

**Files:**
- Create: `internal/mutations/eligibility.go`
- Modify: `internal/mutations/affinity.go:78-115` (`GetGraphPool`)
- Modify: `internal/mutations/mutations.go:262-297` (`GetWeightedPoolWithFloor`), `:374-398` (`CanDeepen`, `RollDeepening`)
- Test: `internal/mutations/eligibility_test.go`

- [ ] **Step 1: Write the failing tests** (reuses `seedColossus` from Task 2)

```go
package mutations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanGainColossusNeedsTitanFour(t *testing.T) {
	seedColossus(t)
	assert.False(t, CanGain(map[string]int{"titan-growth": 3, "dense-muscles": 1}, "colossus-form", 1, nil))
	assert.True(t, CanGain(map[string]int{"titan-growth": 4, "dense-muscles": 1}, "colossus-form", 1, nil))
}

func TestCanGainRefusesOverBudgetAndPastMaxRank(t *testing.T) {
	seedColossus(t)
	full := map[string]int{"titan-growth": 3, "extra-arms": 3}
	assert.False(t, CanGain(full, "titan-growth", 4, nil), "seven units")
	assert.True(t, CanGain(full, "dense-muscles", 1, nil), "no units, fits")
	assert.False(t, CanGain(map[string]int{"colossus-form": 1}, "colossus-form", 2, nil), "max_rank 1")
	assert.False(t, CanGain(map[string]int{"titan-growth": 2}, "titan-growth", 4, nil), "ranks go one at a time")
}

func TestWeightedPoolHonoursPrereqsAndBudget(t *testing.T) {
	seedColossus(t)
	pool := GetWeightedPool(map[string]int{"titan-growth": 1, "dense-muscles": 1}, nil)
	assert.NotContains(t, pool, "colossus-form", "prerequisites now apply to the weighted pool")

	full := GetWeightedPool(map[string]int{"titan-growth": 3, "extra-arms": 3}, nil)
	assert.NotContains(t, full, "colossus-form")
	assert.Contains(t, full, "dense-muscles")
}

func TestRelatedFitPrefersTheRefusedPicksCluster(t *testing.T) {
	seedColossus(t)
	assert.Equal(t, "dense-muscles", RelatedFit("titan-growth", []string{"dense-muscles", "padded-soles"}))
	assert.Equal(t, "padded-soles", RelatedFit("titan-growth", []string{"padded-soles"}))
	assert.Equal(t, "", RelatedFit("titan-growth", nil))
}

func TestRollDeepeningNeverPicksAnOverBudgetRank(t *testing.T) {
	seedColossus(t)
	owned := map[string]int{"titan-growth": 3, "extra-arms": 3, "dense-muscles": 1, "padded-soles": 1}
	sawDense := false
	for i := 0; i < 300; i++ {
		got := RollDeepening(owned)
		assert.Contains(t, []string{"dense-muscles", "padded-soles"}, got)
		sawDense = sawDense || got == "dense-muscles"
	}
	assert.True(t, sawDense)
}

func TestCanDeepenFalseWhenOnlyOverBudgetRanksRemain(t *testing.T) {
	seedColossus(t)
	assert.False(t, CanDeepen(map[string]int{"titan-growth": 3, "extra-arms": 3}))
	assert.True(t, CanDeepen(map[string]int{"titan-growth": 3, "extra-arms": 2}))
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/mutations/ -run 'CanGain|WeightedPoolHonours|RelatedFit|RollDeepeningNever|CanDeepenFalse'`
Expected: build failure, `undefined: CanGain`.

- [ ] **Step 3: Create `internal/mutations/eligibility.go`**

```go
package mutations

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// CanGain is the one eligibility rule for an earned mutation grant (#680).
// newRank 1 is an acquisition; above 1 deepens an owned mutation one step.
// Every pool and every deepening pick goes through it, so a rank the body
// cannot reserve for is never rolled and the grant falls to something else.
//
// Authored sets (species intrinsics, mob spawn lists, presets) do not come
// through here: prerequisites govern earned growth only (owner, 2026-10-26),
// and a guard checks authored sets against the budget.
func CanGain(owned map[string]int, id string, newRank int, sp *species.Species) bool {
	spec := GetMutation(id)
	if spec == nil || newRank < 1 || newRank > MaxRankOf(id) {
		return false
	}
	if newRank == 1 {
		if _, has := owned[id]; has {
			return false
		}
		if HasConflict(owned, id) || !spec.CanApplyTo(sp) || !PrereqsMet(owned, spec) {
			return false
		}
		if spec.MoonFlavor != 0 && spec.MoonFlavor != gametime.CurrentMoonFlavorBucket() {
			return false
		}
	} else if owned[id] != newRank-1 {
		return false
	}
	return FitsReserveBudget(owned, id, newRank)
}

// MaxRankOf is the deepest rank a mutation can reach: its max_rank, or the
// global MutationMaxLevel. Bloom used the global cap alone (#259).
func MaxRankOf(id string) int {
	return effectiveMax(id, int(configs.GetBalanceConfig().MutationMaxLevel))
}

// RelatedFit picks the replacement for a refused id from the candidates that
// do fit: one sharing a cluster with the refused id if any exists, otherwise
// any candidate, otherwise "".
func RelatedFit(refused string, fit []string) string {
	var same []string
	for _, id := range fit {
		if sharesCluster(refused, id) {
			same = append(same, id)
		}
	}
	if len(same) > 0 {
		return same[util.Rand(len(same))]
	}
	if len(fit) > 0 {
		return fit[util.Rand(len(fit))]
	}
	return ""
}

func sharesCluster(a, b string) bool {
	sa, sb := GetMutation(a), GetMutation(b)
	if sa == nil || sb == nil {
		return false
	}
	for _, ca := range sa.Clusters {
		for _, cb := range sb.Clusters {
			if ca == cb {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Route `GetGraphPool` through it.** In `affinity.go`, replace the five checks at the top of the loop (owned, `HasConflict`, `CanApplyTo`, `PrereqsMet`, moon) with:

```go
		if !CanGain(owned, id, 1, sp) {
			continue
		}
```

Delete the now-unused `moonBucket := gametime.CurrentMoonFlavorBucket()` line and drop the `gametime` import if nothing else in the file uses it (`go build ./internal/mutations/` will say).

- [ ] **Step 5: Route `GetWeightedPoolWithFloor` through it.** In `mutations.go`, keep the `minRarity` check and replace the owned, conflict, `CanApplyTo` and moon checks with the same `CanGain(owned, id, 1, sp)` block. Remove its `moonBucket` local likewise. This adds prerequisites and the budget to the Phial, pinnacle items, behaviour trees, quests and the mob spawn bonus roll.

- [ ] **Step 6: Rewrite `CanDeepen` and `RollDeepening`** (F8):

```go
// CanDeepen reports whether any owned mutation may take its next rank.
func CanDeepen(owned map[string]int) bool {
	for id, level := range owned {
		if CanGain(owned, id, level+1, nil) {
			return true
		}
	}
	return false
}

// RollDeepening picks an owned mutation to deepen. It rolls among every owned
// mutation below its max rank, as it always has; if the pick's next rank does
// not fit the reserve budget, a fitting one is chosen instead, preferring the
// pick's own cluster (#680). "" when nothing can deepen.
func RollDeepening(owned map[string]int) string {
	var open, fit []string
	for id, level := range owned {
		if level >= MaxRankOf(id) {
			continue
		}
		open = append(open, id)
		if CanGain(owned, id, level+1, nil) {
			fit = append(fit, id)
		}
	}
	if len(open) == 0 {
		return ""
	}
	sort.Strings(open)
	sort.Strings(fit)
	pick := open[util.Rand(len(open))]
	if CanGain(owned, pick, owned[pick]+1, nil) {
		return pick
	}
	return RelatedFit(pick, fit)
}
```

Add `"sort"` to `mutations.go` imports.

- [ ] **Step 7: Run the package**

Run: `go test ./internal/mutations/`
Expected: PASS. If an existing test seeded an apex without its prerequisites and expected it in `GetWeightedPool`, that test encoded the hole this task closes: update its fixture to own the prerequisites and note it in the commit.

- [ ] **Step 8: Commit**

```bash
git add internal/mutations/eligibility.go internal/mutations/affinity.go internal/mutations/mutations.go internal/mutations/eligibility_test.go
git commit -F - <<'EOF'
feat(mutations): CanGain, one eligibility rule for every grant (#680)

The weighted pool and deepening now honour prerequisites, max_rank and
the reserve budget; a refused deepening falls to a same-cluster fit.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: One size reader and the slot-aware hand rule

**Files:**
- Create: `internal/characters/size.go`
- Modify: `internal/characters/worn.go:278-308` (`HandsRequired`)
- Modify: `internal/characters/hand_slots.go:93-118` (`Is2H`, `IsBlockedBy2H`)
- Modify: `internal/characters/wear_slot.go:163-185` (`handCandidates`), `:266-300` (`chooseArm`)
- Modify: `internal/combat/calculations.go:103-165` (delete `ChanceToTame`)
- Test: `internal/characters/size_hand_rule_test.go`, `internal/guards/species_size_reader_guard_test.go`

- [ ] **Step 1: Write the failing character tests**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedGrowthMutations(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 1},
			Pros: []mutations.MutationEffect{{Type: "size", Target: "large", Value: 3}}},
		"colossus-form": {MutationId: "colossus-form", Name: "Colossus Form", Rarity: 9, MaxRank: 1,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1},
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "colossal-grip", Value: 1}}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 2}},
	}))
}

func grown(t *testing.T, extraArms, titan int, colossus bool) *Character {
	t.Helper()
	seedGoldenSpecies(t)
	seedGrowthMutations(t)
	c := goldenChar(extraArms, goldenMedium, true)
	c.Mutations["titan-growth"] = titan
	if colossus {
		c.Mutations["colossus-form"] = 1
	}
	return c
}

func TestSizeStepsToLargeAtTitanThree(t *testing.T) {
	assert.Equal(t, species.Medium, grown(t, 0, 2, false).Size())
	assert.Equal(t, species.Large, grown(t, 0, 3, false).Size())
	seedGoldenSpecies(t)
	assert.Equal(t, species.Large, goldenChar(0, goldenLarge, true).Size())
	assert.Equal(t, species.Small, goldenChar(0, goldenSmall, true).Size())
}

func TestGrownWithoutColossusGripsInTheWeaponHandOnly(t *testing.T) {
	c := grown(t, 2, 3, false)
	great := goldenItem(9201, goldenGreat, false)
	assert.Equal(t, 1, c.HandsRequiredAt(great, "wielded"))
	assert.Equal(t, 2, c.HandsRequiredAt(great, "extra arm 1"))
	assert.Equal(t, 2, c.HandsRequired(great), "no named hand: the grip every hand has")

	c.Equipment.Weapon = great
	c.Equipment.ExtraArm1 = goldenItem(9202, goldenGreat, false)
	assert.False(t, c.IsBlockedBy2H("offhand"))
	assert.True(t, c.IsBlockedBy2H("extra arm 2"))
}

func TestColossusGripsInEveryHand(t *testing.T) {
	c := grown(t, 2, 4, true)
	great := goldenItem(9211, goldenGreat, false)
	assert.Equal(t, 1, c.HandsRequiredAt(great, "extra arm 1"))
	c.Equipment.Weapon = great
	c.Equipment.ExtraArm1 = goldenItem(9212, goldenGreat, false)
	assert.False(t, c.IsBlockedBy2H("offhand"))
	assert.False(t, c.IsBlockedBy2H("extra arm 2"))
}

func TestColossusOnAMediumCharacterGivesNoGrip(t *testing.T) {
	// Authored sets may skip prerequisites (owner, 2026-10-26): Colossus Form
	// at Titan Growth 1 adds its stats only.
	c := grown(t, 0, 1, true)
	assert.Equal(t, 2, c.HandsRequiredAt(goldenItem(9221, goldenGreat, false), "wielded"))
}

func TestGrownAutoEquipPutsATwoHanderInTheWeaponHandAlone(t *testing.T) {
	c := grown(t, 0, 3, false)
	choice, refusal := c.ChooseWornSlot(goldenItem(9231, goldenGreat, false), 0)
	require.Empty(t, refusal)
	require.Len(t, choice.Slots, 1)
	assert.Same(t, &c.Equipment.Weapon, choice.Slots[0].Item)
}

func TestGrownArmEquipRules(t *testing.T) {
	c := grown(t, 0, 3, false)
	great := goldenItem(9241, goldenGreat, false)

	choice, refusal := c.ChooseWornSlot(great, 1)
	require.Empty(t, refusal)
	assert.Len(t, choice.Slots, 1, "arm 1 takes the two-hander alone")

	_, refusal = c.ChooseWornSlot(great, 2)
	assert.Contains(t, refusal, "needs a pair of arms", "the offhand never grips a two-hander alone")
}
```

`SlotChoice.Slots` is `[]WornSlot`, and `WornSlot.Item` is the pointer to the real slot field (`worn.go:40-44`).

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/characters/ -run 'Size|Grown|Colossus'`
Expected: build failure, `c.Size undefined`, `c.HandsRequiredAt undefined`.

- [ ] **Step 3: Create `internal/characters/size.go`**

```go
package characters

import (
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Size is the one reader of a character's size (#680). It is the species
// size, stepped to Large when a mutation's size effect has reached its rank
// (Titan Growth 3). An unregistered species reads as Medium. A guard forbids
// reading species size anywhere else.
func (c *Character) Size() species.Size {
	if mutations.GrowsLarge(c.Mutations) {
		return species.Large
	}
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && sp.Size != `` {
		return sp.Size
	}
	return species.Medium
}

// gripsInEveryHand reports whether a Large character holds a two-hander alone
// in every hand, not only the weapon hand: a Large species, or a character
// carrying the colossal-grip flag (Colossus Form). Only meaningful when
// Size() is Large.
func (c *Character) gripsInEveryHand() bool {
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && sp.Size == species.Large {
		return true
	}
	return mutations.HasMutationFlag(c.Mutations, `colossal-grip`)
}
```

- [ ] **Step 4: Replace `HandsRequired`** in `worn.go` (F12) with:

```go
// HandsRequired is HandsRequiredAt with no hand named: the grip the character
// has in every hand. The "too many hands" checks and the whole-pair
// two-hander search read this.
func (c *Character) HandsRequired(i items.Item) int {
	return c.HandsRequiredAt(i, ``)
}

// HandsRequiredAt returns how many hands item i needs in the named hand slot
// ("wielded", "offhand", "extra arm 1" to "extra arm 4"; "" for no particular
// hand).
//
// Shooting weapons ignore size. A Large species holds anything in one hand.
// A character grown Large by mutation holds a two-hander alone in the weapon
// hand only, and in every hand once they carry Colossus Form (#680). Small
// needs one hand more.
func (c *Character) HandsRequiredAt(i items.Item, slot string) int {
	if i.ItemId < 1 {
		return 0
	}
	iSpec := i.GetSpec()
	if iSpec.Subtype == items.Shooting {
		return iSpec.Hands
	}
	switch c.Size() {
	case species.Large:
		if slot == `wielded` || c.gripsInEveryHand() {
			return 1
		}
	case species.Small:
		return iSpec.Hands + 1
	}
	return iSpec.Hands
}
```

- [ ] **Step 5: Make `Is2H` and `IsBlockedBy2H` slot-aware** (F15):

```go
func (s HandSlot) Is2H(c *Character) bool {
	if s.ItemPtr == nil || s.ItemPtr.ItemId < 1 {
		return false
	}
	return c.HandsRequiredAt(*s.ItemPtr, s.Label) >= 2
}
```

and in `IsBlockedBy2H` replace the map and return with:

```go
	partners := map[string]HandSlot{
		"offhand":     {"wielded", &c.Equipment.Weapon},
		"extra arm 2": {"extra arm 1", &c.Equipment.ExtraArm1},
		"extra arm 4": {"extra arm 3", &c.Equipment.ExtraArm3},
	}
	partner, ok := partners[slot]
	if !ok {
		return false
	}
	return partner.Is2H(c)
```

Update the doc comment's second sentence to: "It reads the wearer's grip in the partner's own hand through Is2H, so a Large wearer's two-hander leaves the partner free (#681, #680)."

- [ ] **Step 6: The weapon hand first in `handCandidates`.** In the two-hander branch (F16), immediately before `return fill, swap, \`You have no free pair...\``, insert:

```go
		// #680: a character grown Large without Colossus Form holds a
		// two-hander alone in the weapon hand only, so that hand is offered
		// first, as one slot.
		if c.HandsRequiredAt(i, pairs[0].First.Label) < 2 {
			main := occupant(pairs[0].First.ItemPtr)
			fill = append([]slotCandidate{main}, fill...)
			swap = append([]slotCandidate{main}, swap...)
		}
```

- [ ] **Step 7: `chooseArm` reads the target hand.** Replace

```go
	pair := pairs[pairIdx]
	twoHanded := c.HandsRequired(i) >= 2
```

and the later `target := pair.First` / `if slotInPair == 1 { target = pair.Second }` block with, in this order:

```go
	pair := pairs[pairIdx]
	target := pair.First
	if slotInPair == 1 {
		target = pair.Second
	}
	twoHanded := c.HandsRequiredAt(i, target.Label) >= 2
```

Leave everything after unchanged.

- [ ] **Step 8: Run the character tests and the golden**

Run: `go test ./internal/characters/`
Expected: PASS, including `wear_slot_golden_test.go` (a Large species still grips everywhere, so the golden does not move).

- [ ] **Step 9: Write the guard**, `internal/guards/species_size_reader_guard_test.go`:

```go
package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #680: Character.Size() (internal/characters/size.go) is the one reader of
// species size. A second reader would ignore a size a mutation grants.
var speciesSizeRead = regexp.MustCompile(`\.Size\s*(==|!=)\s*species\.|switch\s+[\w.]+\.Size\s*\{`)

func TestSpeciesSizeReadOnlyInSize(t *testing.T) {
	root := repoRoot(t)
	var bad []string
	for _, dir := range []string{"internal", "modules"} {
		filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/species/") || rel == "internal/characters/size.go" {
				return nil
			}
			src, _ := os.ReadFile(path)
			for n, line := range strings.Split(string(src), "\n") {
				if speciesSizeRead.MatchString(line) {
					bad = append(bad, rel+":"+itoa(n+1)+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
	}
	if len(bad) > 0 {
		t.Fatalf("species size read outside Character.Size():\n%s", strings.Join(bad, "\n"))
	}
}
```

`itoa` is the guards package helper in `durable_write_guard_test.go:285`.

- [ ] **Step 10: Prove the guard can fail.** Run it now, before deleting `ChanceToTame`:

Run: `go test ./internal/guards/ -run TestSpeciesSizeReadOnlyInSize`
Expected: FAIL naming `internal/combat/calculations.go:131`.

- [ ] **Step 11: Delete `ChanceToTame`** (F18): remove the whole function from `calculations.go`, then `go build ./...` and drop any import it leaves unused.

- [ ] **Step 12: Run the guard and the touched packages**

Run: `go test ./internal/guards/ -run TestSpeciesSizeReadOnlyInSize && go test ./internal/characters/ ./internal/combat/ ./internal/usercommands/`
Expected: PASS.

- [ ] **Step 13: Commit**

```bash
git add internal/characters/size.go internal/characters/worn.go internal/characters/hand_slots.go internal/characters/wear_slot.go internal/characters/size_hand_rule_test.go internal/combat/calculations.go internal/guards/species_size_reader_guard_test.go
git commit -F - <<'EOF'
feat(characters): Size() reader and the grown-Large hand rule (#680)

Titan Growth 3 makes a character Large: a two-hander in the weapon hand
alone, every hand with Colossus Form. Deletes the uncalled ChanceToTame.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: The body as a Conviction reservation source

**Files:**
- Modify: `internal/characters/validate.go:276-281` (`GetPoolReservation`)
- Modify: `internal/characters/reservation.go` (F20)
- Test: `internal/characters/mutation_reserve_test.go`

- [ ] **Step 1: Write the failing tests** (reuses `grown` from Task 4)

```go
package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutationUnitsReserveConviction(t *testing.T) {
	c := grown(t, 0, 3, false)
	// 3 units of 0.11 of 400 = 132.
	assert.Equal(t, 132, c.GetPoolReservation("conviction", 400))
	assert.Equal(t, 0, c.GetPoolReservation("health", 400), "the body reserves Conviction only")
}

func TestFullBudgetReservesTheCeilingExactly(t *testing.T) {
	c := grown(t, 1, 4, true) // 4 + 1 + 1 = 6 units
	assert.Equal(t, reservationCapFor(400), c.GetPoolReservation("conviction", 400))
}

func TestReservationNamesTheBody(t *testing.T) {
	c := grown(t, 0, 3, false)
	c.ConvictionMax.Value = 400
	subject, verb := c.ReservationHolders(PoolConviction)
	assert.Equal(t, "Your body", subject)
	assert.Equal(t, "holds", verb)

	c.Companions = append(c.Companions, CompanionInfo{Name: "a wolf", ConvictionReserve: 40})
	subject, verb = c.ReservationHolders(PoolConviction)
	assert.Equal(t, "Your body and bonds", subject)
	assert.Equal(t, "hold", verb)
}

func TestRefusalWithOnlyTheBodyHoldingOffersNoRemedy(t *testing.T) {
	c := grown(t, 3, 3, false) // six units, at the ceiling
	c.ConvictionMax.Value = 400
	msg := c.ReservationRefusal(PoolConviction, 10)
	assert.Contains(t, msg, "Your body")
	assert.NotContains(t, msg, "make room: .")
	assert.Contains(t, msg, "nothing you put down will free it")
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/characters/ -run 'MutationUnits|FullBudget|NamesTheBody|OnlyTheBody'`
Expected: FAIL, reservation 0 where 132 expected.

- [ ] **Step 3: Price the body.** In `reservation.go`, after `enchantReserveAtWithMax`, add:

```go
// mutationReserveWithMax is what the body sets aside on Conviction (#680):
// the owned mutations' reserve units times MutationReserveUnitPct of the pool,
// floored and clamped to the ceiling. Takes the pool max for the same
// mid-derivation reason itemReserveOnPoolWithMax does.
func (c *Character) mutationReserveWithMax(poolMax int) int {
	units := mutations.OwnedReserveUnits(c.Mutations)
	if units <= 0 || poolMax <= 0 {
		return 0
	}
	pct := float64(configs.GetBalanceConfig().MutationReserveUnitPct)
	r := int(math.Floor(float64(poolMax) * float64(units) * pct))
	if cap := reservationCapFor(poolMax); r > cap {
		r = cap
	}
	return r
}
```

Add the `mutations` import to `reservation.go`.

- [ ] **Step 4: Count it.** In `GetPoolReservation` (F19), inside `if pool == "conviction" {` after the companion loop:

```go
		// The body: Titan Growth, Colossus Form and Extra Arms (#680).
		total += c.mutationReserveWithMax(poolMax)
```

- [ ] **Step 5: Name it.** In `reservation.go`:

Add `body bool // reserving mutations, conviction only; nothing removes it` to `reserveSources`.

In `reserveSourcesOn`, inside `if p == PoolConviction {` before the companion loop:

```go
		if c.mutationReserveWithMax(poolMax) > 0 {
			s.body = true
		}
```

Replace `any`, `merge`, `subject` and `verb`:

```go
func (s reserveSources) any() bool { return s.gear() || s.companion || s.body }

func (s reserveSources) merge(o reserveSources) reserveSources {
	return reserveSources{
		drainingItem: s.drainingItem || o.drainingItem,
		enchantment:  s.enchantment || o.enchantment,
		companion:    s.companion || o.companion,
		body:         s.body || o.body,
	}
}

func (s reserveSources) subject() string {
	var held []string
	if s.body {
		held = append(held, `body`)
	}
	if s.gear() {
		held = append(held, `gear`)
	}
	if s.companion {
		held = append(held, `bonds`)
	}
	if len(held) == 0 {
		return `What you carry`
	}
	return `Your ` + joinWithAnd(held)
}

// verb agrees with subject(): "holds" for one singular holder (body, gear,
// or the fallback), "hold" for bonds or any list.
func (s reserveSources) verb() string {
	n := 0
	for _, b := range []bool{s.body, s.gear(), s.companion} {
		if b {
			n++
		}
	}
	if n == 0 || (n == 1 && !s.companion) {
		return `holds`
	}
	return `hold`
}
```

Keep the existing doc comments above each, adding to `subject`'s: "The body comes first: it is the one holder nothing can remove."

In `ReservationRefusal`, replace the final `return` with:

```go
	tail := `You cannot take on more until you make room: ` + sources.remedies() + `.`
	if sources.remedies() == `` {
		// Only the body holds it, and the body cannot be put down (#680).
		tail = `Your changed body holds that share, and nothing you put down will free it.`
	}
	return sources.subject() + ` already ` + sources.verb() + ` ` + share +
		` of your ` + pool + ` in reserve. ` + tail
```

- [ ] **Step 6: Run the package**

Run: `go test ./internal/characters/`
Expected: PASS. Existing message tests carry no reserving mutations, so their text is unchanged.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/validate.go internal/characters/reservation.go internal/characters/mutation_reserve_test.go
git commit -F - <<'EOF'
feat(characters): reserving mutations hold Conviction as the body (#680)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Grant sites keep progress and honour the rule

**Files:**
- Modify: `internal/hooks/NewRound_UserRoundTick.go:440-510`
- Modify: `internal/hooks/NewRound_MobRoundTick.go:320-395`
- Modify: `internal/characters/bloom_mutation.go:35-160`
- Test: `internal/characters/bloom_mutation_reserve_test.go`

The weighted and graph pools already honour `CanGain` (Task 3). Three things remain: the ticks reset progress before knowing whether anything was granted, a failed deepening never falls back to acquiring, and Bloom has its own pickers.

- [ ] **Step 1: Write the failing Bloom tests** (reuse `grown`)

```go
package characters

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBloomAdvanceSkipsOverBudgetRanks(t *testing.T) {
	// Six units: neither Titan Growth 4 nor Extra Arms 4 fits, and the only
	// unowned seeded mutation (Colossus Form) lacks Titan Growth 4.
	c := grown(t, 3, 3, false)
	id, _ := c.BloomAdvanceMutation(rand.New(rand.NewSource(1)))
	assert.Equal(t, "", id)
	assert.Equal(t, 3, c.Mutations["titan-growth"])
	assert.Equal(t, 3, c.Mutations["extra-arms"])
}

func TestBloomSeedHonoursPrerequisites(t *testing.T) {
	for i := int64(0); i < 50; i++ {
		c := grown(t, 0, 1, false)
		id, _ := c.BloomSeedNewMutation(rand.New(rand.NewSource(i)))
		assert.NotEqual(t, "colossus-form", id, "Titan Growth 1 does not meet Colossus Form")
	}
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/characters/ -run 'BloomAdvanceSkips|BloomSeedHonours'`
Expected: FAIL (Bloom deepens Titan Growth to 4; Bloom seeds Colossus Form).

- [ ] **Step 3: Bloom advance.** In `BloomAdvanceMutation`, replace `if e.level < maxLevel {` with

```go
			if mutations.CanGain(c.Mutations, e.id, e.level+1, nil) {
```

and delete the `maxLevel` local. The loop already walks from the strongest down, so the first fitting rank wins, and the fall-through still seeds.

- [ ] **Step 4: Bloom seed.** In `BloomSeedNewMutation`, replace the owned, `HasConflict`, `CanApplyTo` and moon checks inside the candidate loop with

```go
		if !mutations.CanGain(c.Mutations, id, 1, sp) {
			continue
		}
```

Remove imports this leaves unused (`gametime`, `configs`).

- [ ] **Step 5: Player tick.** In `NewRound_UserRoundTick.go` inside `if user.Character.MutationProgress >= threshold {` (F22):
  1. Delete `user.Character.MutationProgress = 0` and the `mutations.DecayAffinity(...)` line from the top of the block.
  2. Declare `granted := false` before `doDeepen`.
  3. In the deepen branch, set `granted = true` inside `if mutId != "" {`.
  4. Change `} else if canAcquire {` to `}` followed by `if !granted && canAcquire {` so a deepening that found nothing falls to acquiring, and set `granted = true` inside `if len(pool) > 0 {` after the rank is written.
  5. After the acquire block, add:

```go
							// Progress is spent only on a grant. When nothing
							// fits (#680) it carries to the next event.
							if granted {
								user.Character.MutationProgress = 0
								// Mutation-graph drift fades on each mutation event so recent behavior dominates.
								mutations.DecayAffinity(user.Character.ClusterAffinity, float64(mb.MutationAffinityDecay))
							}
```

- [ ] **Step 6: Mob tick.** Same shape in `NewRound_MobRoundTick.go` (F23): remove the early `mob.Character.MutationProgress = 0` and `DecayAffinity`; track `granted`; after a deepen that returns `""`, fall through to the acquire block instead of `return`; reset progress and decay affinity only when `granted`. Keep the existing `return` after a successful deepen.

- [ ] **Step 7: Run**

Run: `go test ./internal/characters/ ./internal/hooks/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/characters/bloom_mutation.go internal/characters/bloom_mutation_reserve_test.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewRound_MobRoundTick.go
git commit -F - <<'EOF'
feat(mutations): grant sites keep progress when nothing fits; Bloom honours CanGain (#680)

Closes the Bloom half of #259: Bloom no longer deepens past max_rank.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Companions released newest first when a gain passes the ceiling

**Files:**
- Modify: `internal/mutations/reveal.go` (F26)
- Modify: `internal/hooks/MutationGained_Reveal.go:110-128`
- Create: `internal/hooks/mutation_reserve_release.go`
- Modify: `internal/hooks/hooks.go:35`
- Modify: `internal/hooks/PlayerSpawn_HandleJoin.go:70-77` (F27)
- Modify: `internal/hooks/companion_reserve_backfill.go:40-44, 62-71` (comments only)
- Modify: `internal/actions/drink.go:424-470` (F25)
- Test: `internal/hooks/mutation_reserve_release_test.go`

- [ ] **Step 1: Write the failing test.** `releaseCompanionsOverCeiling` takes a `*characters.Character` and returns the released records, so it is testable without a live user or mob; a thin wrapper does the messaging and despawn.

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/assert"
)

func seedTitan(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7,
			ReserveUnits: []int{1, 1, 1, 1}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 2}},
	}))
}

func TestReleaseNewestCompanionsUntilUnderCeiling(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400 // ceiling 264
	ch.Mutations = map[string]int{"titan-growth": 3, "extra-arms": 2} // 5 units = 220
	ch.Companions = []characters.CompanionInfo{
		{Name: "old wolf", InstanceId: 1, ConvictionReserve: 30},
		{Name: "new wolf", InstanceId: 2, ConvictionReserve: 30},
	}
	released := releaseCompanionsOverCeiling(ch)
	// 220 + 60 = 280 > 264: drop the newest (250 fits).
	if assert.Len(t, released, 1) {
		assert.Equal(t, "new wolf", released[0].Name)
	}
	assert.Len(t, ch.Companions, 1)
}

func TestNoReleaseWithoutReservingMutations(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 100
	ch.Companions = []characters.CompanionInfo{{Name: "wolf", InstanceId: 1, ConvictionReserve: 90}}
	assert.Empty(t, releaseCompanionsOverCeiling(ch), "D4: an overage with no body share is grandfathered")
}

func TestBondedCompanionsAreNeverReleased(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400
	ch.Mutations = map[string]int{"titan-growth": 4, "extra-arms": 3} // 7 units, clamped to 264
	ch.Companions = []characters.CompanionInfo{{Name: "Jev", InstanceId: 1, SourceType: characters.CompanionBonded}}
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
}
```

If `characters.New()` recomputes `ConvictionMax` from stats on a later call, set it immediately before `releaseCompanionsOverCeiling` and make the function read the pool through `ReservationOverages()` without calling `RecalculateStats` (the wrapper recalculates before calling it; see Step 3).

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/hooks/ -run 'ReleaseNewest|NoReleaseWithout|BondedCompanions'`
Expected: build failure, `undefined: releaseCompanionsOverCeiling`.

- [ ] **Step 3: Create `internal/hooks/mutation_reserve_release.go`**

```go
package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// releaseCompanionsOverCeiling drops companion records, newest first, while
// the Conviction reservation is past the ceiling (#680). It acts only when
// the body holds part of the pool: a gained rank of Titan Growth, Colossus
// Form or Extra Arms crowds out bonds, while an overage from re-pricing or
// gear alone stays grandfathered (D4). Bonded companions hold no reserve and
// are never released. Returns the records it removed.
func releaseCompanionsOverCeiling(ch *characters.Character) []characters.CompanionInfo {
	if mutations.OwnedReserveUnits(ch.Mutations) == 0 {
		return nil
	}
	var released []characters.CompanionInfo
	for ch.ReservationOverages().Conviction > 0 {
		idx := -1
		for i := len(ch.Companions) - 1; i >= 0; i-- {
			if ch.Companions[i].ConvictionReserve > 0 {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		comp := ch.Companions[idx]
		ch.Companions = append(ch.Companions[:idx], ch.Companions[idx+1:]...)
		released = append(released, comp)
	}
	return released
}

// releaseCompanionsForUser runs the release for a live player and makes each
// one real: the creature leaves peacefully (no grudge: the owner's body
// changed, nobody betrayed anyone), the player is told, and the pool is
// republished.
func releaseCompanionsForUser(user *users.UserRecord) {
	user.Character.RecalculateStats()
	released := releaseCompanionsOverCeiling(user.Character)
	if len(released) == 0 {
		return
	}
	for _, comp := range released {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`Your changing body claims more of your will. <ansi fg="mobname">%s</ansi> slips its bond and leaves you.`,
			comp.Name))
		mob := mobs.GetInstance(comp.InstanceId)
		if mob == nil {
			continue
		}
		cmd := ``
		if mob.Character.Charmed != nil {
			cmd = mob.Character.Charmed.ExpiredCommand
		}
		if mob.Character.IsCharmed(user.UserId) {
			mob.Character.RemoveCharm()
		}
		user.Character.TrackCharmed(comp.InstanceId, false)
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> slips its bond and leaves.`, comp.Name), user.UserId)
		}
		if comp.SourceType == characters.CompanionCharmed {
			runCharmExpiredCommand(mob, cmd)
			continue
		}
		mob.Command("despawn")
	}
	user.Character.RecalculateStats()
	events.AddToQueue(events.CharacterVitalsChanged{UserId: user.UserId})
}

// MutationGained_ReleaseCompanions runs the release after every player rank
// gain, including the silent Bloom and Phial grants.
func MutationGained_ReleaseCompanions(e events.Event) events.ListenerReturn {
	evt, ok := e.(mutations.Gained)
	if !ok {
		return events.Continue
	}
	if user := users.GetByUserId(evt.UserId); user != nil {
		releaseCompanionsForUser(user)
	}
	return events.Continue
}
```

`Character.Charmed` is a `*CharmInfo` (`character.go:141`), nil when not charmed, hence the guard. `room.SendTextVisual(category, text, excludeUserId)` is the call `dismiss.go` uses (F28).

- [ ] **Step 4: Register the listener** after the reveal listener in `hooks.go:35`:

```go
	// #680: a gained rank of a reserving mutation may crowd out companions.
	events.RegisterListener(mutations.Gained{}, MutationGained_ReleaseCompanions)
```

- [ ] **Step 5: A silent Gained for Bloom and the Phial.** Add to `mutations.Gained`:

```go
	Silent     bool // the grant site already narrated it (Bloom, Phial): no reveal
```

In `MutationGained_Reveal`, right after the `user == nil` check:

```go
	if evt.Silent {
		return events.Continue
	}
```

In `drink.go` add the `events` import, and after each grant emit:

```go
		if mutId != "" {
			events.AddToQueue(mutations.Gained{UserId: user.UserId, MutationId: mutId,
				Rank: char.Mutations[mutId], IsNew: char.Mutations[mutId] == 1, Silent: true})
		}
```

(Bloom block: after `mutId` is set. Phial block: inside `if granted != ""` with `granted` in place of `mutId`.) Read the surrounding function first for the user record's variable name; `actor` and `char` are in scope at F25, so use whichever exposes `UserId` (the `DrinkResult` path uses `actor`).

- [ ] **Step 6: Run it at login.** In `PlayerSpawn_HandleJoin.go`, right after `refreshCompanionReservesOnLogin(user)`:

```go
	// #680: a body that grew while away (or before this rule shipped) may
	// hold more than the ceiling leaves for bonds. Runs before the companions
	// are spawned, so only records are dropped.
	releaseCompanionsForUser(user)
```

Update the comment above `refreshCompanionReservesOnLogin` ("This never dismisses anyone") and the two "Nothing is dismissed" / "It NEVER dismisses" comments in `companion_reserve_backfill.go` to add: "The one exception is the body's own share (#680), handled by releaseCompanionsForUser."

- [ ] **Step 7: Run**

Run: `go test ./internal/hooks/ ./internal/actions/ ./internal/mutations/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/mutations/reveal.go internal/hooks/MutationGained_Reveal.go internal/hooks/mutation_reserve_release.go internal/hooks/mutation_reserve_release_test.go internal/hooks/hooks.go internal/hooks/PlayerSpawn_HandleJoin.go internal/hooks/companion_reserve_backfill.go internal/actions/drink.go
git commit -F - <<'EOF'
feat(hooks): release companions newest first when the body passes the ceiling (#680)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: World data, help and the data guard

**Files:**
- Modify: `_datafiles/world/dogmud/mutations/titan-growth.yaml`, `extra-arms.yaml`, `colossus-form.yaml`
- Modify: `internal/characters/character.go:310` (comment)
- Modify: help templates `titan-growth`, `colossus-form`, `extra-arms`, `reservation`, `species` in `_datafiles/world/dogmud/templates/help/`
- Test: `internal/guards/mutation_reserve_data_guard_test.go`

- [ ] **Step 1: Write the guard first**

```go
package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// #680: every reserve_units list covers every rank, and every authored
// mutation set (species intrinsics, mob spawn lists, the Homunculus preset)
// fits the reserve budget. Authored sets may skip prerequisites (owner,
// 2026-10-26); they may not skip the budget.
func TestMutationReserveData(t *testing.T) {
	mobsById := loadFightWorld(t)

	for _, spec := range mutations.AllSpecs() {
		if len(spec.ReserveUnits) > 0 && len(spec.ReserveUnits) != mutations.MaxRankOf(spec.MutationId) {
			t.Errorf("%s: reserve_units has %d entries, max rank is %d",
				spec.MutationId, len(spec.ReserveUnits), mutations.MaxRankOf(spec.MutationId))
		}
	}
	for _, want := range []string{"titan-growth", "extra-arms", "colossus-form"} {
		if s := mutations.GetMutation(want); s == nil || len(s.ReserveUnits) == 0 {
			t.Errorf("%s must declare reserve_units", want)
		}
	}

	budget := mutations.ReserveUnitBudget()
	check := func(who string, set map[string]int) {
		if u := mutations.OwnedReserveUnits(set); u > budget {
			t.Errorf("%s: authored mutations hold %d units, budget %d", who, u, budget)
		}
	}
	for _, sp := range species.GetAllSpecies() {
		check("species "+sp.Name, sp.IntrinsicMutations)
	}
	for id, m := range mobsById {
		set := map[string]int{}
		for _, mid := range m.SpawnMutations {
			set[mid] = 1
		}
		if sp := species.GetSpecies(m.Character.SpeciesId); sp != nil {
			for mid, r := range sp.IntrinsicMutations {
				set[mid] += r
			}
		}
		check("mob "+itoa(id), set)
	}
	check("homunculus preset", map[string]int{"titan-growth": 2})
}
```

`species.GetAllSpecies()` returns `[]Species` (`species.go:89`); `IntrinsicMutations` is `map[string]int` (`species.go:76`).

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/guards/ -run TestMutationReserveData`
Expected: FAIL, the three mutations "must declare reserve_units".

- [ ] **Step 3: `titan-growth.yaml`**: add under `pros:`

```yaml
  - type: size
    target: large
    value: 3
```

and at the end of the file:

```yaml
reserve_units: [1, 1, 1, 1]
```

- [ ] **Step 4: `extra-arms.yaml`**: set `max_rank: 4` and drop the "binary (no deepen)" comment; replace `description:` and `visual:`:

```yaml
max_rank: 4  # one new arm per rank, up to six arms (#259)
description: New arms sprout from your torso, one with each change, each able to wield a weapon of its own. The grotesque sight of extra flailing limbs draws hostile attention and unsettles everyone around you.
visual: Extra arms extend from their torso, moving with surprising coordination.
reserve_units: [1, 1, 1, 2]
```

- [ ] **Step 5: `colossus-form.yaml`**: prerequisite `titan-growth` `min_level: 4`; add to `pros:`

```yaml
  - type: flag
    target: colossal-grip
    value: 1
```

and `reserve_units: [1]`.

- [ ] **Step 6: Comment fix.** `character.go:310`: change "(0-2)" to "(0-4)".

- [ ] **Step 7: Help copy.** Load the `dogmud-player-copy` skill first (80-column wrap, no numbers, no dashes). Edit:
  - `titan-growth.template` Benefits: add "At its fullest you grow large enough to wield a two-handed weapon in your weapon hand alone." Add a Drawbacks line: "Your growing body holds part of your conviction in reserve, leaving less for companions."
  - `colossus-form.template`: Requires becomes "Dense Muscles and the fullest Titan Growth"; Benefits add "every hand you have can wield a two-handed weapon alone"; Drawbacks add the same reserve line.
  - `extra-arms.template` (create it if absent, matching the other templates' layout, and add it to `docs/README.md` only if templates are indexed there): one arm per change, up to six; the reserve drawback, with the last change costing more.
  - `reservation.template`: in the section listing what holds reserve, add your body (Titan Growth, Colossus Form, Extra Arms), which nothing you put down frees, and that a body growing past what your bonds leave room for sends the newest companion away.
  - `species.template:15-16` Large line: "2-Handed weapons require 1 hand. A body grown large by mutation grips one only in the weapon hand, until Colossus Form."
- [ ] **Step 8: Run the guard, the world boot checks and the template guards**

Run: `go test ./internal/guards/ ./internal/mutations/ ./internal/characters/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/mutations/titan-growth.yaml _datafiles/world/dogmud/mutations/extra-arms.yaml _datafiles/world/dogmud/mutations/colossus-form.yaml internal/characters/character.go _datafiles/world/dogmud/templates/help/titan-growth.template _datafiles/world/dogmud/templates/help/colossus-form.template _datafiles/world/dogmud/templates/help/extra-arms.template _datafiles/world/dogmud/templates/help/reservation.template _datafiles/world/dogmud/templates/help/species.template internal/guards/mutation_reserve_data_guard_test.go
git commit -F - <<'EOF'
feat(world): Titan Growth size, Extra Arms to rank 4, Colossus needs Titan 4, reserve units (#680, #259)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(Drop `extra-arms.template` from the `git add` if it already existed and was not changed.)

---

### Task 9: Docs

**Files:**
- Modify: `internal/mutations/context.md`, `internal/characters/context.md`, `internal/hooks/context.md`
- Modify: `docs/PATCH_NOTES.md`, `docs/README.md` (this plan's row)

- [ ] **Step 1: `context.md` files.** Name only symbols that exist (`codegraph_search` each): `mutations` gains `ReserveUnits`, `ReserveUnitsAt`, `OwnedReserveUnits`, `ReserveUnitBudget`, `FitsReserveBudget`, `GrowsLarge`, `CanGain`, `MaxRankOf`, `RelatedFit`, the `size` effect, the `colossal-grip` flag, `Gained.Silent`. `characters` gains `Size`, `HandsRequiredAt`, the body reservation source. `hooks` gains `releaseCompanionsForUser` and `MutationGained_ReleaseCompanions`. Then run `python tools/context_md_audit.py` and fix anything it reports.
- [ ] **Step 2: PATCH_NOTES.** Add a dated 2026-10-26 entry in the file's existing style, player-facing, no numbers: Titan Growth makes you large at its later ranks; Colossus Form now needs the fullest Titan Growth and lets every hand wield a two-hander; Extra Arms grows up to six arms; these mutations hold part of your conviction in reserve, so a deeply grown body fields fewer companions and sends the newest away when it outgrows them.
- [ ] **Step 3: Commit** (the plan's own README row landed with the plan)

```bash
git add internal/mutations/context.md internal/characters/context.md internal/hooks/context.md docs/PATCH_NOTES.md docs/README.md
git commit -F - <<'EOF'
docs: #680 context.md, patch notes and plan index

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Full gate and PR

- [ ] **Step 1:** Load `dogmud-shipping` and follow its pre-push gate: `gofmt -l .` (empty), `go vet ./...`, `go test ./...`, `golangci-lint run --new-from-merge-base=origin/master`, and the detached-worktree boot check (this change touches world YAML).
- [ ] **Step 2:** Push and open the PR with `gh pr create --repo pruuk/DOGMud --base master`. Body: what players see first, then the mechanism; `Closes #680` and `Closes #259` as the only closing keywords; ends with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- [ ] **Step 3:** Report the PR URL and the gate output. Do not merge without the controller's review.
