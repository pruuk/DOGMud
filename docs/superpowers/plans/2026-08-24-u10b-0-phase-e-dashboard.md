# U10b-0 Phase E — Dashboard re-key

**Index:** `docs/superpowers/plans/2026-08-21-u10b-0-README.md`
**Spec:** `docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md`
— section **10** is this phase's requirements document. Sections 13–15 supersede
1–12 where they conflict.

**Predecessors:** Phases A (`1c5d10fd7`), B, C and D are all merged. Master is at
`4db7766f7`.

---

## Why this phase matters

`internal/web/admin.progression.go` is the **only** surface outside
`internal/characters` and the mob save code that reads a use counter. No in-game
command shows one. Phase C made the counters telemetry and re-keyed rank to
`Training` / skill level; Phase D retuned every multiplier on measured data.
**The dashboard still reports the pre-C model**, so it is currently the one
in-game surface that would show the Phase D retune misfiring, and it cannot.

Three things are wrong with it today, independent of U10b-0:

1. `avgDeviation = rank - calcExpectedRank(uses, mult, UsesPerRank)` is now
   noise, and once virtual rank *is* rank it collapses toward a structural zero
   — the curve term of every skill health score reports "perfectly healthy"
   whatever the truth is.
2. `progression_chance` is hand-rolled bare `CalculateProgressionChance(rank,
   softCap)`. It omits `StatProgressionRate` (**2.25 shipped**), every per-stat
   and per-skill multiplier Phase D solved, and the mutation/buff multipliers.
   It understates every stat chance by more than half before Phase D's per-stat
   values are even considered. **This is why the dashboard could never have
   surfaced the two sealed stats Phase B fixed.**
3. `buildStatHealth` reads `GetStatValue`, but `StatInfo.Value` is `yaml:"-"`
   and `loadRecentUserFiles` does a raw `yaml.Unmarshal` with no
   `Recalculate()`. **Every offline player reports 0** and lands in the `0-50`
   bucket, so the Stat Value Distribution chart is garbage for anyone not
   logged in.

---

## Standing rules (from the phase index — all seven still apply)

1. **No absolute line numbers for code an earlier task shifts.** Locate with
   `grep` at execution time, and verify the grep matched the symbol you meant.
2. **Go defaults move with shipped values.** A test binary never loads
   `config.yaml` — pin every knob a test depends on via `SetConfigForTest`.
3. Safety defaults use `<= 0`; only genuine off-switches use `< 0`.
4. **`characters.New()` calls `Validate()`**, which populates `Value` and
   hydrates `Base`. A fixture reproducing a *disk-loaded* character must be a
   raw `&characters.Character{}` literal.
5. Migrations must never assign `Base` blindly. (No migrations in this phase.)
6. A soft cap bounds nothing.
7. **`grep --include=*.go` cannot see `config.yaml` or the HTML template.**

Phase-E-specific corrections carried from the index:

- **Use `int(math.Ceil(uses))` in the inverse test.** `usesToReach` returns a
  float; truncating asks for one use less than rank `r` costs, so the inverse
  returns `r-1` for every non-integer case. v1 of the monolith made this
  mistake and the test failed for every rank ≥ 1.
- **Build the offline fixture as a raw `&characters.Character{}`, never
  `New()`.** A `New()`-based fixture cannot reproduce the defect and asserts
  vacuously (~50% flaky on the second assertion).
- **`statChanceProbe` must have a caller.** v1 defined it and never used it —
  a new `unused` finding on an `only-new-issues` lint gate, *and* it silently
  dropped spec §10.4 item 3.

---

## Facts verified against master `4db7766f7` before writing this plan

Do not re-derive these; do verify anything you are about to edit.

| Claim | Status |
|---|---|
| `internal/characters/progression.go` has `skillProgressionChance` and `statProgressionChance` | **Both unexported.** Spec §10.2 requires the exported form. Task 1. |
| `progressionRollDenominator` = `1000000`, unexported | Confirmed. The dashboard needs the same threshold arithmetic. Task 1. |
| Callers of the two unexported funcs outside `progression.go` | Three in-package `_test.go` files: `progression_faucet_test.go`, `progression_floor_test.go`, `progression_rank_test.go`. Mechanical rename. |
| `c.GetStatTraining(statName)` exists | `internal/characters/skills.go:255` (Phase A). |
| `bal.UsesPerRank` readers | **`admin.progression.go` only**, plus `internal/configs/smoke_test.go` asserting `> 0` and a stale comment in `internal/usercommands/go.go:46`. After this phase nothing computes with it. |
| `spells.GetAllSpells()` / `crafting.GetAll()` / `GetStarterRecipes()` | Plain map reads. **No panic on an unloaded registry — no `TestMain` needed.** |
| `internal/web` already has `TestMain` | `auth_test.go:19`. Do not add a second. |
| `auth_test.go` chdirs to repo root and calls `ReloadConfig()` | Restored via `t.Cleanup`, but tests share one binary — **pin config explicitly, never rely on ambient.** |
| `configs.SetConfigForTest(t, Config)` | `internal/configs/testing_support.go:30`, self-registering restore. |
| `StatInfo` fields | `Training`, `Value` (`yaml:"-"`), `ValueAdj`, `Racial`, `Base` (`yaml:"base,omitempty"`), `Mods`, `BaseAuthored`. `Base` **is** persisted, so `Base + Training` is readable offline. |
| Meirok's real save (`_datafiles/world/dogmud/users/3.yaml`) | str 115/21, dex 98/12, per 101/51, vit 104/14, wil 113/35, cha 93/30. The fixtures below use these. |
| Template anchors (`_datafiles/html/admin/progression/index.html`, 438 lines) | stall `<thead>` `:55-63`, `updateStallTable` `:222`, `updateStatDistChart` `:288`, `updatePlayerTable` `:345`, headers `:349`, skill tooltip `:360-364`, `updatePlayerStatTable` `:374`, stat cell push `:388-396`. Re-grep at execution time. |
| Stall JS bug | `:230` does `s.progression_chance / 100`, but Go emits a **0-1 fraction**. `expectedForNext` has been 100× too large, so the panel has effectively never fired. |

---

## Task 1: Export the chance expression and the roll threshold

**Files:** `internal/characters/progression.go`, and the three in-package test
files that call the old names.

Spec §10.2 names the required signatures. Phase C built the expression but left
it unexported, and its own doc comment says "Phase E needs to call the real
expression rather than a duplicate."

- [ ] **Step 1: Rename, do not wrap.**

```bash
grep -rn "statProgressionChance\|skillProgressionChance\|progressionRollDenominator" --include=*.go internal/
```

`statProgressionChance` → **`ProgressionChanceForStat`**
`skillProgressionChance` → **`ProgressionChanceForSkill`**

Update the doc comments: they reference the unexported names and say "Phase E
needs...". Keep the substance (mob gating, why rank is `Training`, why the floor
is applied last and why `chance > 0` guards it); fix the names; drop the
now-satisfied Phase E sentence.

- [ ] **Step 2: Add `ProgressionRollThreshold` and route both roll sites
      through it.**

The dashboard's dead-stat alarm must ask the *same* question the roll asks. Do
**not** export the denominator — export the arithmetic, so the alarm cannot
drift from production if the resolution ever changes again.

```go
// ProgressionRollThreshold converts a 0-1 progression chance into the integer
// threshold CheckStatProgression and CheckSkillProgression roll against.
//
// Exported so the admin dashboard can flag the failure mode this arithmetic
// used to cause: at the old resolution of 10,000, any chance below 0.01%
// truncated to a threshold of zero and the stat was not slow but SEALED. Two of
// a live character's six stats were in that state. Balance.ProgressionChanceFloor
// is what removes the seal; this is how you detect it coming back.
//
// A return of 0 for a positive chance means "cannot fire". A caller holding a
// chance of exactly 0 was already told "may not progress at all" by the mob
// gates and should not consult this.
func ProgressionRollThreshold(chance float64) int {
	return int(chance * progressionRollDenominator)
}
```

Replace `threshold := int(chance * progressionRollDenominator)` in **both**
`CheckStatProgression` and `CheckSkillProgression` with
`threshold := ProgressionRollThreshold(chance)`. Leave
`util.Rand(progressionRollDenominator)` alone — the roll's range is not the
caller's business.

- [ ] **Step 3: Fix the test call sites and verify.**

`progression_floor_test.go` uses `progressionRollDenominator` directly in four
places; it is in-package, so it may keep doing so. Only the two function names
need updating there.

```bash
gofmt -l internal/characters/
go build ./... && go test ./internal/characters/
```

- [ ] **Step 4: Commit.**

```
refactor(u10b-0): export the progression chance expression for Phase E

Spec section 10.2 requires ProgressionChanceForStat and
ProgressionChanceForSkill as the single callable source of the chance
production rolls. Phase C built the expression but left it unexported, so the
dashboard still hand-rolls bare CalculateProgressionChance and omits
StatProgressionRate (2.25 shipped) plus every multiplier Phase D solved.

ProgressionRollThreshold exports the arithmetic rather than the resolution, so
the dashboard's dead-stat alarm asks exactly the question the roll asks and
cannot drift from it.

Pure rename plus one extraction. No behaviour change.
```

---

## Task 2: Write the dashboard tests first

**File:** create `internal/web/admin_progression_test.go`.

- [ ] **Step 1: Write the tests.** They must fail before Tasks 3–5.

```go
package web

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/stats"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// testBalance pins every knob these tests depend on. A test binary never loads
// config.yaml, and auth_test.go's setup calls ReloadConfig() on a shared
// binary, so nothing ambient is trustworthy here.
func testBalance(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 0.12
	cfg.Balance.ProgressionDecayBelowCap = 3.0
	cfg.Balance.ProgressionDecayAboveCap = 2.0
	cfg.Balance.StatProgressionSoftCap = 50
	cfg.Balance.SkillSoftCap = 50
	cfg.Balance.StatProgressionRate = 2.25
	cfg.Balance.ProgressionChanceFloor = 1e-5
	cfg.Balance.StatProgressionMultipliers = map[string]float64{
		"strength": 1.0, "dexterity": 1.0, "perception": 1.0,
		"vitality": 1.0, "willpower": 1.0, "charisma": 1.0,
	}
	configs.SetConfigForTest(t, cfg)
}

func TestUsesToReach_IsIncreasingAndConvex(t *testing.T) {
	testBalance(t)
	c := characters.New()
	chanceAt := func(rank int) float64 {
		c.Stats.Perception.Training = rank
		return c.ProgressionChanceForStat("perception", 1.0)
	}

	prev, prevStep := 0.0, 0.0
	for r := 1; r <= 40; r++ {
		got := usesToReach(chanceAt, r)
		if got <= prev {
			t.Fatalf("usesToReach(%d) = %v, not greater than %v", r, got, prev)
		}
		step := got - prev
		if r > 1 && step <= prevStep {
			t.Errorf("rank %d costs %v, not more than rank %d's %v; curve not decaying", r, step, r-1, prevStep)
		}
		prev, prevStep = got, step
	}
}

// NOTE the Ceil. usesToReach returns a float and expectedRankForUses answers
// "the highest rank affordable within this many uses", so truncating would ask
// for one use less than rank r costs and the inverse would return r-1 for every
// non-integer case.
func TestExpectedRankForUses_InvertsUsesToReach(t *testing.T) {
	testBalance(t)
	c := characters.New()
	chanceAt := func(rank int) float64 {
		c.Skills = map[string]int{"weapon-combat": rank}
		return c.ProgressionChanceForSkill("weapon-combat", 1.0)
	}

	for _, wantRank := range []int{0, 1, 5, 15, 30} {
		uses := usesToReach(chanceAt, wantRank)
		if math.IsInf(uses, 1) {
			t.Fatalf("rank %d unreachable", wantRank)
		}
		if got := expectedRankForUses(chanceAt, int(math.Ceil(uses)), 50); got != wantRank {
			t.Errorf("expectedRankForUses(%d) = %d, want %d", int(math.Ceil(uses)), got, wantRank)
		}
	}
}

// The dashboard must display the chance production rolls. Bare
// CalculateProgressionChance omitted StatProgressionRate and every per-stat,
// per-skill, mutation and buff multiplier -- the reason this page could never
// have surfaced the sealed stats Phase B fixed.
func TestPlayerOverview_ChanceMatchesProduction(t *testing.T) {
	testBalance(t)

	c := characters.New()
	c.Name = "Fixture"
	c.Stats.Strength.Base = 115
	c.Stats.Strength.Training = 21
	c.Stats.Strength.Recalculate()
	c.Skills = map[string]int{"weapon-combat": 12}

	out := buildPlayerOverview([]*users.UserRecord{{UserId: 1, Character: c}})
	if len(out) != 1 {
		t.Fatalf("expected 1 player, got %d", len(out))
	}
	if want, got := c.ProgressionChanceForStat("strength", 1.0), out[0].Stats["strength"].ProgressionChance; math.Abs(got-want) > 1e-12 {
		t.Errorf("dashboard stat chance %.12f, production %.12f", got, want)
	}
	if want, got := c.ProgressionChanceForSkill("weapon-combat", 1.0), out[0].Skills["weapon-combat"].ProgressionChance; math.Abs(got-want) > 1e-12 {
		t.Errorf("dashboard skill chance %.12f, production %.12f", got, want)
	}
}

// Phase B's floor makes the sealed state unreachable in production, so this
// alarm is a regression detector. Force the condition and confirm it fires.
func TestPlayerOverview_DeadStatAlarm(t *testing.T) {
	testBalance(t)

	c := characters.New()
	c.Name = "Meirok"
	c.Stats.Dexterity.Base = 98
	c.Stats.Dexterity.Training = 12
	c.Stats.Dexterity.Recalculate()

	out := buildPlayerOverview([]*users.UserRecord{{UserId: 3, Character: c}})
	if dex := out[0].Stats["dexterity"]; dex.Dead || dex.ProgressionChance <= 0 {
		t.Errorf("dexterity reported dead at training 12; chance %.9f", dex.ProgressionChance)
	}

	cfg := configs.GetConfig()
	cfg.Balance.ProgressionChanceFloor = 0 // disable the floor to reach the condition
	cfg.Balance.StatProgressionMultipliers = map[string]float64{"dexterity": 1e-12}
	configs.SetConfigForTest(t, cfg)

	out = buildPlayerOverview([]*users.UserRecord{{UserId: 3, Character: c}})
	if !out[0].Stats["dexterity"].Dead {
		t.Error("dead-stat alarm did not fire when the threshold truncates to 0")
	}
}

// The live pre-existing bug. StatInfo.Value is yaml:"-" and loadRecentUserFiles
// unmarshals raw YAML without Recalculate(), so an offline player reports 0.
//
// The fixture must NOT use characters.New(): New() calls Validate(), which
// populates Value and hydrates Base, so a New()-based fixture cannot reproduce
// the defect and asserts vacuously.
func TestStatHealth_OfflinePlayerIsNotBucketedAsZero(t *testing.T) {
	testBalance(t)

	c := &characters.Character{
		Name: "Offline",
		Stats: stats.Statistics{
			Strength: stats.StatInfo{Base: 115, Training: 21}, // Value deliberately 0
		},
	}
	if c.Stats.Strength.Value != 0 {
		t.Fatalf("fixture precondition failed: Value = %d, want 0", c.Stats.Strength.Value)
	}

	got := buildStatHealth([]*users.UserRecord{{UserId: 9, Character: c}})
	if got["strength"].Distribution["0-50"] != 0 {
		t.Errorf("offline player bucketed as 0-50: %v", got["strength"].Distribution)
	}
	if got["strength"].Distribution["101-150"] != 1 {
		t.Errorf("expected the 136-point player in 101-150: %v", got["strength"].Distribution)
	}
}

// Spec section 10.4 item 4: the curve reads Training, so a Training histogram
// is the population view that describes progression difficulty. The value
// histogram stays as a second series.
func TestStatHealth_TrainingDistribution(t *testing.T) {
	testBalance(t)

	c := &characters.Character{
		Name: "Offline",
		Stats: stats.Statistics{
			Perception: stats.StatInfo{Base: 101, Training: 51},
			Vitality:   stats.StatInfo{Base: 104, Training: 14},
		},
	}
	got := buildStatHealth([]*users.UserRecord{{UserId: 9, Character: c}})
	if got["perception"].TrainingDistribution["51+"] != 1 {
		t.Errorf("perception training 51 not in 51+: %v", got["perception"].TrainingDistribution)
	}
	if got["vitality"].TrainingDistribution["11-25"] != 1 {
		t.Errorf("vitality training 14 not in 11-25: %v", got["vitality"].TrainingDistribution)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail for the right reason** — undefined
      symbols and wrong values, not a panic in an unrelated registry.

```bash
go test ./internal/web/ -run "TestUsesToReach|TestExpectedRank|TestPlayerOverview|TestStatHealth" -v
```

---

## Task 3: Replace `calcExpectedRank` with the curve inverse

**File:** `internal/web/admin.progression.go`.

- [ ] **Step 1:** `grep -n "func calcExpectedRank" internal/web/admin.progression.go`.
      Delete the function **and its doc comment** — the comment describes a
      model the engine no longer runs, and orphaning it is worse than leaving it.

- [ ] **Step 2:** Add the inverse. Spec §10.3: expected uses from rank `k` to
      `k+1` is `1/chance(k)`, so cumulative uses to reach rank `R` is the sum
      below `R`.

```go
// usesToReach returns the expected cumulative recorded uses needed to reach
// rank r, given a per-rank chance function.
//
// This is the inverse of the progression curve and replaces the pre-U10b-0
// uses/UsesPerRank division. Returns +Inf if any rank below r has zero chance.
func usesToReach(chanceAt func(rank int) float64, r int) float64 {
	total := 0.0
	for k := 0; k < r; k++ {
		ch := chanceAt(k)
		if ch <= 0 {
			return math.Inf(1)
		}
		total += 1.0 / ch
	}
	return total
}

// expectedRankForUses returns the highest rank whose cumulative expected cost
// is within uses -- the inverse of usesToReach. A zero-chance rank counts as
// NOT reached, matching usesToReach's +Inf for the same input.
//
// CAVEAT: assumes one progression roll per recorded use. U10b introduces an
// uncontested class that records a use and rolls at a damped rate, so this
// degrades into a LOWER BOUND as the uncontested mix grows. Triage signal, not
// measurement -- the panel says so.
func expectedRankForUses(chanceAt func(rank int) float64, uses int, maxRank int) int {
	total := 0.0
	for r := 0; r < maxRank; r++ {
		ch := chanceAt(r)
		if ch <= 0 {
			return r
		}
		total += 1.0 / ch
		if total > float64(uses) {
			return r
		}
	}
	return maxRank
}
```

---

## Task 4: JSON types and the two probes

- [ ] **Step 1: Extend the JSON types.**

```go
type playerSkillJSON struct {
	Rank              int     `json:"rank"`
	Tier              string  `json:"tier"`
	UseCount          int     `json:"use_count"`          // telemetry only since U10b-0
	ExpectedRank      int     `json:"expected_rank"`      // curve inverse, not UsesPerRank
	ProgressionChance float64 `json:"progression_chance"` // 0-1 fraction at bonus 1.0
	Dead              bool    `json:"dead"`               // roll threshold truncates to 0
	Fragile           bool    `json:"fragile"`            // threshold < 10; one config nudge from dead
}

type playerStatJSON struct {
	Value             int     `json:"value"`
	Training          int     `json:"training"` // the curve's rank input since U10b-0
	UseCount          int     `json:"use_count"`
	ExpectedRank      int     `json:"expected_rank"`
	ProgressionChance float64 `json:"progression_chance"`
	Dead              bool    `json:"dead"`
	Fragile           bool    `json:"fragile"`
}
```

**Remove `VirtualRank`** — under U10b-0 it duplicates `Rank`. Grep the template
for `virtual_rank` before deleting (it appears unreferenced there; confirm).

`Fragile` is spec §10.4 item 2's warning tier (`threshold < 10`), which the
monolith dropped. It is the early warning for the exact class of failure the
alarm exists to catch.

- [ ] **Step 2: Add both probes.**

```go
// skillChanceProbe returns c's progression chance at a hypothetical skill rank,
// for the curve-inverse helpers. Works on a shallow copy with its own Skills
// map so the caller's record is untouched.
//
// Character contains shared pointers, so this is safe ONLY because
// ProgressionChanceForSkill reads just IsMob, Skills, Mutations and
// HasBuffFlag -- none of which the copy mutates beyond Skills. Recalculate() is
// not needed: the curve reads the level, never Value.
//
// Called up to softCap times per subject per player, and each call copies the
// Character struct. If a dashboard render ever becomes slow, memoise per
// (subject, rank) rather than per player.
func skillChanceProbe(c *characters.Character, skillName string) func(int) float64 {
	return func(rank int) float64 {
		probe := *c
		probe.Skills = map[string]int{skillName: rank}
		return probe.ProgressionChanceForSkill(skillName, 1.0)
	}
}

// statChanceProbe is the stat-side mirror, varying Training. The curve reads
// Training alone since Phase C, so no Recalculate() is needed here either.
func statChanceProbe(c *characters.Character, statName string) func(int) float64 {
	return func(rank int) float64 {
		probe := *c
		switch statName {
		case "strength":
			probe.Stats.Strength.Training = rank
		case "dexterity":
			probe.Stats.Dexterity.Training = rank
		case "perception":
			probe.Stats.Perception.Training = rank
		case "vitality":
			probe.Stats.Vitality.Training = rank
		case "willpower":
			probe.Stats.Willpower.Training = rank
		case "charisma":
			probe.Stats.Charisma.Training = rank
		}
		return probe.ProgressionChanceForStat(statName, 1.0)
	}
}
```

**Both get callers in Task 5.**

---

## Task 5: Rewrite the aggregation

- [ ] **Step 1: `buildPlayerOverview` — skills loop.**

```go
		skillsMap := make(map[string]playerSkillJSON)
		for _, sk := range skills.GetAllSkillNames() {
			name := string(sk)
			rank := 0
			if c.Skills != nil {
				rank = c.Skills[name]
			}
			useCount := c.GetSkillUseCount(name)
			progChance := c.ProgressionChanceForSkill(name, 1.0)
			threshold := characters.ProgressionRollThreshold(progChance)

			skillsMap[name] = playerSkillJSON{
				Rank:              rank,
				Tier:              skills.GetSkillRankDescription(rank),
				UseCount:          useCount,
				ExpectedRank:      expectedRankForUses(skillChanceProbe(c, name), useCount, softCap),
				ProgressionChance: progChance,
				Dead:              threshold == 0,
				Fragile:           threshold > 0 && threshold < 10,
			}
		}
```

- [ ] **Step 2: `buildPlayerOverview` — stats loop.** This is spec §10.4, the
      half that never existed.

```go
		statsMap := make(map[string]playerStatJSON, len(statNames))
		for _, statName := range statNames {
			progChance := c.ProgressionChanceForStat(statName, 1.0)
			useCount := c.GetStatUseCount(statName)
			threshold := characters.ProgressionRollThreshold(progChance)

			statsMap[statName] = playerStatJSON{
				Value:             getStatBaseValue(c, statName),
				Training:          c.GetStatTraining(statName),
				UseCount:          useCount,
				ExpectedRank:      expectedRankForUses(statChanceProbe(c, statName), useCount, int(bal.StatProgressionSoftCap)),
				ProgressionChance: progChance,
				Dead:              threshold == 0,
				Fragile:           threshold > 0 && threshold < 10,
			}
		}
```

`mult := skills.GetProgressionMultiplier(name)` and the `virtualRank` line go
away. Let the compiler find every newly-unused local.

- [ ] **Step 3: `buildSkillHealth` — re-derive deviation and stall.**

```go
			probe := skillChanceProbe(u.Character, skillName)

			expected := expectedRankForUses(probe, useCount, softCap)
			deviation := float64(rank - expected)
			deviationSum += deviation

			if deviation < worstDeviation || worstPlayer == "" {
				worstDeviation = deviation
				worstPlayer = u.Character.Name
			}

			usesAtRank := usesToReach(probe, rank)
			usesSinceGain := float64(useCount) - usesAtRank
			if usesSinceGain < 0 || math.IsInf(usesAtRank, 1) {
				usesSinceGain = 0
			}
			// chanceAtRank now comes from the probe -- the full production
			// expression -- rather than bare CalculateProgressionChance, so the
			// stall threshold means something different from before, and right.
			if chanceAtRank := probe(rank); chanceAtRank > 0 {
				if usesSinceGain/(1.0/chanceAtRank) > 2.0 {
					stallCount++
				}
			}
```

Delete the `mult := skills.GetProgressionMultiplier(skillName)` hoist and the
`bal.UsesPerRank` read.

- [ ] **Step 4: `buildStatHealth` — fix the offline bug and add the Training
      histogram.**

```go
type statHealthJSON struct {
	Distribution         map[string]int `json:"distribution"`
	TrainingDistribution map[string]int `json:"training_distribution"`
}
```

```go
	for _, statName := range statNames {
		distribution := map[string]int{
			"0-50": 0, "51-100": 0, "101-150": 0, "151+": 0,
		}
		// Spec 10.4 item 4: the curve reads Training, so a value histogram no
		// longer describes progression difficulty. Both series ship: value for
		// the population view, training for the curve view.
		trainingDist := map[string]int{
			"0": 0, "1-10": 0, "11-25": 0, "26-50": 0, "51+": 0,
		}
		for _, u := range playerList {
			if u.Character == nil {
				continue
			}
			// NOT GetStatValue: StatInfo.Value is yaml:"-" and
			// loadRecentUserFiles unmarshals raw YAML without calling
			// Recalculate(), so every OFFLINE player reported 0 and landed in
			// the 0-50 bucket. getStatBaseValue reads Base + Training, which is
			// what the save actually carries. This drops equipment Mods for
			// online players too, which is correct here: Mods are exactly what
			// U10b-0 removed from the curve.
			val := getStatBaseValue(u.Character, statName)
			switch {
			case val <= 50:
				distribution["0-50"]++
			case val <= 100:
				distribution["51-100"]++
			case val <= 150:
				distribution["101-150"]++
			default:
				distribution["151+"]++
			}

			switch tr := u.Character.GetStatTraining(statName); {
			case tr <= 0:
				trainingDist["0"]++
			case tr <= 10:
				trainingDist["1-10"]++
			case tr <= 25:
				trainingDist["11-25"]++
			case tr <= 50:
				trainingDist["26-50"]++
			default:
				trainingDist["51+"]++
			}
		}
		result[statName] = statHealthJSON{
			Distribution:         distribution,
			TrainingDistribution: trainingDist,
		}
	}
```

- [ ] **Step 5: Delete the accessor duplicate.** Remove `getStatTraining` from
      `admin.progression.go` — `c.GetStatTraining` (Phase A) replaces it.
      **Keep `getStatBaseValue`**: there is no `characters` equivalent for
      `Base + Training`, and Step 4 depends on it. Say so in its doc comment.

- [ ] **Step 6: Verify.**

```bash
gofmt -l internal/ modules/
go build ./... && go test ./internal/web/ ./internal/characters/ ./internal/configs/
```

- [ ] **Step 7: Commit.**

```
fix(u10b-0): re-key the progression dashboard to the live model

Every metric here derived from useCount/UsesPerRank, the knob Phase C retired.
Deviation and stall are re-derived from the inverse of the live curve
(cumulative 1/chance(r)) rather than deleted, so the panels that answer "is
anyone stuck" keep working -- and stop collapsing to a structural zero now that
virtual rank IS rank.

progression_chance now calls production instead of bare
CalculateProgressionChance, which omitted StatProgressionRate (2.25 shipped)
and every per-stat, per-skill, mutation and buff multiplier -- the reason this
page could never have surfaced the sealed stats Phase B fixed, nor a Phase D
multiplier landing wrong.

Adds the stat side that never existed: per-stat chance, expected rank, a
dead-stat alarm and a fragile warning tier, plus a Training histogram, since
the curve reads Training and a value histogram no longer describes difficulty.

Also fixes a live pre-existing bug: buildStatHealth read GetStatValue, but
StatInfo.Value is yaml:"-" and loadRecentUserFiles never calls Recalculate(),
so every offline player reported 0 and the Stat Value Distribution chart was
garbage for anyone not logged in.
```

---

## Task 6: The template

**File:** `_datafiles/html/admin/progression/index.html`. Admin pages render
through `SafeDom` — **keep the `tr()` / `esc()` / `raw()` helpers**; do not
interpolate HTML.

Re-grep anchors before editing:

```bash
grep -n "updateStallTable\|updatePlayerStatTable\|'Player', 'Activity'\|chanceDecimal\|updateStatDistChart\|statDistChart\|virtual_rank" _datafiles/html/admin/progression/index.html
```

- [ ] **Step 1: Stall table spans stats and skills, and reads the server.**

The current JS recomputes staleness in the browser, duplicating the Go math and
getting it wrong: it divides an already-0-to-1 fraction by 100, so
`expectedForNext` is 100× too large and the panel has effectively never fired.

```javascript
  function updateStallTable(d) {
    var tbody = document.querySelector('#stallTable tbody');
    tbody.innerHTML = '';
    var stalls = [];

    function push(p, kind, what, rank, expected, dead, fragile) {
      // Behind by one rank is noise; this panel is for genuinely stuck
      // progression, and for anything that cannot progress at all.
      if (dead || fragile || (expected - rank) >= 2) {
        stalls.push({player: p, kind: kind, what: what, rank: rank,
                     expected: expected, behind: expected - rank,
                     dead: dead, fragile: fragile});
      }
    }

    d.players.forEach(function(p) {
      Object.entries(p.skills).forEach(function(e) {
        if (e[1].use_count > 0) { push(p.name, 'skill', e[0], e[1].rank, e[1].expected_rank, e[1].dead, e[1].fragile); }
      });
      Object.entries(p.stats).forEach(function(e) {
        if (e[1].use_count > 0) { push(p.name, 'stat', e[0], e[1].training, e[1].expected_rank, e[1].dead, e[1].fragile); }
      });
    });

    stalls.sort(function(a, b) {
      if (a.dead !== b.dead) { return a.dead ? -1 : 1; }
      return b.behind - a.behind;
    });

    if (stalls.length === 0) {
      document.getElementById('noStalls').style.display = '';
      document.getElementById('stallTable').style.display = 'none';
      return;
    }
    document.getElementById('noStalls').style.display = 'none';
    document.getElementById('stallTable').style.display = '';

    var html = '';
    stalls.forEach(function(s) {
      var status;
      if (s.dead) {
        status = raw('<span class="badge badge-danger">DEAD - cannot progress</span>');
      } else if (s.fragile) {
        status = raw('<span class="badge badge-warning">FRAGILE - near the roll floor</span>');
      } else {
        status = raw('<span class="badge badge-warning">behind by ' + esc(s.behind) + '</span>');
      }
      html += tr([s.player, s.kind, s.what, s.rank, s.expected, status]);
    });
    tbody.innerHTML = html;
  }
```

Update the stall `<thead>` to six columns: `Player`, `Kind`, `Skill / Stat`,
`Rank`, `Expected Rank`, `Status`.

- [ ] **Step 2: Stat cells get the chance and the badge.** Replace the cell push
      in `updatePlayerStatTable`:

```javascript
        if (s) {
          var v = s.value;
          var clr = v < 90 ? '#e74c3c' : v < 100 ? '#e67e22' : v < 120 ? '#3498db' : v < 150 ? '#2ecc71' : '#f1c40f';
          var chancePct = (s.progression_chance * 100).toFixed(3) + '%';
          var badge = s.dead ? ' <span class="badge badge-danger">DEAD</span>'
                    : s.fragile ? ' <span class="badge badge-warning">FRAGILE</span>'
                    : ' <span class="text-muted">(' + esc(chancePct) + ')</span>';
          cells.push({
            v: raw(esc(v) + badge),
            title: 'training=' + s.training + ' (this IS the progression rank)'
                 + ' | expected from uses=' + s.expected_rank
                 + ' | next=' + chancePct + ' at neutral bonus'
                 + ' | uses=' + s.use_count + ' (telemetry only)',
            style: 'color:' + clr
          });
        } else {
          cells.push('-');
        }
```

- [ ] **Step 3: Relabel telemetry.** `var headers = ['Player', 'Activity
      (telemetry)'];`, and the skill tooltip:

```javascript
          cells.push({
            v: raw(esc(s.rank) + ' ' + tierBadge(s.tier)),
            title: 'rank=' + s.rank + ' (this IS the progression rank)'
                 + ' | expected from uses=' + s.expected_rank
                 + ' | next=' + (s.progression_chance * 100).toFixed(3) + '%'
                 + ' | uses=' + s.use_count + ' (telemetry only)'
          });
```

- [ ] **Step 4: Training histogram.** Add a third canvas to the Population
      Distribution panel headed `Stat Training Distribution (all players)`, and
      an `updateStatTrainingChart(d)` modelled on `updateStatDistChart` with
      `labels = ['0','1-10','11-25','26-50','51+']` reading
      `e[1].training_distribution`. Reuse `statColors`. Call it wherever
      `updateStatDistChart` is called, and give it its own chart variable with
      the same `destroy()` guard.

- [ ] **Step 5: Standing caveat note.** Inside the Skill Health card body,
      before the table:

```html
          <p class="text-muted small mb-2">
            Progression rank is the skill level (skills) or trained points
            (stats). Use counts are engagement telemetry only and no longer
            affect how hard anything is to raise. Expected Rank comes from the
            curve and assumes one progression roll per recorded use, so treat it
            as a triage signal rather than a measurement. Chances are shown at a
            neutral bonus; critical successes roll higher.
          </p>
```

- [ ] **Step 6: Verify it is SERVED, not just written.** The user runs the local
      server — ask them to reload, or hand them the curl. **Do not start or kill
      a server.**

```bash
curl -s http://localhost:<HttpPort>/admin/progression | grep -c "telemetry only"     # want >= 1
curl -s http://localhost:<HttpPort>/admin/progression/data | head -c 1200
```

Confirm the JSON carries `expected_rank`, `dead`, `fragile` and
`progression_chance` under **both** `skills` and `stats`, and
`training_distribution` under `stats`.

- [ ] **Step 7: Commit.**

```
feat(u10b-0): dashboard template reads the new fields

Stall detection stops recomputing staleness in JS -- that duplicated the Go
math and did it wrong, dividing an already-0-to-1 fraction by 100, so
expectedForNext was 100x too large and the panel effectively never fired. It
now reads the server's expected_rank and spans stats as well as skills, because
the dead-stat class it exists to catch lives entirely in stats.

Adds the Training histogram alongside the value histogram: the curve reads
trained points, so that is the series that describes difficulty.

Rendering keeps the SafeDom tr()/esc() helpers.
```

---

## Task 7: Gates and ship

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`; `go test ./internal/web/ ./internal/characters/
      ./internal/configs/ ./internal/skills/`.
- [ ] **Boot test in an isolated detached worktree** on non-default ports, per
      the pre-push SOP. Build to a fixed `boot-check.exe` path. **Exit code 124
      is the success case.** Never grep the bare word `panic`.
- [ ] `docs/PATCH_NOTES.md` — this phase is admin-only and player-invisible.
      Phase F owns the arc's player-facing entry; add nothing player-facing here
      unless the owner asks.
- [ ] `_datafiles/config.yaml` is **not** touched by this phase. If it turns out
      it must be, build the commit from the `git show HEAD:` blob, never disk.
- [ ] Push, `gh pr create --repo pruuk/DOGMud --base master --head
      feature/u10b-0-phase-e-dashboard --fill`, watch the checks, merge
      `--merge` (not `--squash`), delete the branch.
- [ ] After merge, delete a stray `refs/tags/master` if it re-seeds on origin.

**No adversarial playtest gate on this phase.** It authors no player-facing
content — that gate is Phase F's, and the arc's pre-deploy playtest is still
owed regardless.

---

## Owner punchlist for `/admin/progression` after this lands

Read the page with the local server running and confirm:

1. **Meirok's stats** show a per-stat next-chance. Perception (training 51) is
   above the soft cap and must show a small but **non-zero** chance with no DEAD
   badge — that is Phase B's floor working. Strength (21) and dexterity (12)
   must be visibly larger than perception's.
2. **No stat anywhere shows DEAD.** If one does, Phase B's floor is not reaching
   that path, and that is a blocker.
3. **Skill Health avg deviation is not identically 0.00 for every skill.** A
   column of exact zeros means the curve term collapsed and the inverse is not
   wired.
4. **An offline player's stat values are not all 0** in the Stat Value
   Distribution chart — nobody in the `0-50` bucket who should not be there.
5. **The Training histogram** puts Meirok's perception in `51+` and his
   dexterity in `11-25`.
6. **Every tooltip that mentions a use count says "telemetry only".**

---

## Carried forward to Phase F

- **After this phase, nothing computes with `Balance.UsesPerRank`.**
  `internal/configs/smoke_test.go` still asserts it is positive and
  `internal/usercommands/go.go:46` still carries a comment explaining the
  retired `virtualRank = useCount / UsesPerRank` model. Phase F decides: keep
  the knob as documented-dead, or remove it and the smoke assertion together.
- **`config.yaml`'s vitality comment contradicts its own value.** The block
  above `vitality: 2.2` still reads "NOT retuned by Phase D ... needs its own
  slice", directly above an inline comment saying the vitality slice solved it.
  Phase F cleanup, built from the `git show HEAD:` blob.
- **The instance-save wipe destroys exactly the legacy mob saves Phase C's
  migration handles.** Exercise that migration before wiping, or it ships
  untested.
