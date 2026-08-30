# Contest Gap Compression + Archetype Stat Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop a modest power advantage producing a majority-crit outcome, by compressing the score gap before it is rolled, and stop a `casting` mob's physical defence collapsing to one fifteenth of its stat pool.

**Architecture:** Two config-gated changes that must ship together. `combat.RunContest` — the single entry point for all 25 opposed-contest call sites — compresses the attacker's lead over the defender by a configurable exponent before rolling. Separately, `mobs.newMobByIdInternal`'s stat-pool distribution moves from a hardcoded 80/20 archetype split to normalised per-stat weights. Both knobs are identities at their defaults, so either can be switched off in production without a rebuild.

**Tech Stack:** Go, `gopkg.in/yaml.v2`, `testify`, PowerShell/Bash on Windows, `gh` pinned to `--repo pruuk/DOGMud`.

**Spec:** [`2026-08-30-contest-gap-compression-design.md`](../specs/2026-08-30-contest-gap-compression-design.md)

**Branch:** `feature/contest-gap-compression` (already exists: three spec commits plus the oasis tier fix).

---

## Context the engineer needs

### The contest model, read from source

`internal/contest/contest.go:96-120`:

```go
func Run(atkScore float64, entries []Entry) Result {
	stdDev := dice.StdDevFor(atkScore)      // :97  ONE stdDev, from the ATTACK score
	attackRoll := dice.Roll(atkScore, stdDev)
	for _, e := range entries {
		defenseRoll := dice.Roll(e.Score, stdDev)  // :103  defender uses it too
		margin := attackRoll.Value - defenseRoll.Value
		if !res.Contested || margin < res.Margin { ... }   // :109 best = smallest margin
	}
	res.Success = res.Contested && res.Margin > 0
}
```

Because both rolls share one standard deviation, the normalized margin is
exactly `N(mean, 1)` where `mean = (A-D)/(0.15·A·√2)`. Crit fires when that
normalized margin clears `CritBarFor` (`combat/margin_crit.go:132`), which
floors at **1.5**. Hence:

| score ratio | hit% | crit% |
|---|---|---|
| 1.00 | 50.0 | 6.7 |
| 1.50 | 94.2 | **52.8** |
| 3.00 | 99.9 | 95.0 |

**A 50% power edge already means a majority of hits crit**, and a crit skips
mitigation entirely (`crit_damage.go:96-104`), worth up to 17.6× against armour.

### Two rules that will bite you

1. **`_datafiles/config.yaml` has `skip-worktree`.** Never `git add` it from
   disk. Build the commit from the `git show HEAD:` blob, and restore the
   on-disk copy afterwards. Task 3 and Task 6 spell this out.
2. **An absent config key unmarshals to `0`.** For an exponent, `|gap|^0 == 1`,
   which would collapse every mismatch in the game to a one-point gap. Both
   validators here must key on `<= 0`, never `< 0`. This exact trap left five
   knobs silently pinned at zero, found on 2026-08-30.

### Multi-entry contests

Verified: of the 25 `RunContest` call sites, all but one pass a single
`[]contest.Entry{{Score: X}}`. The one exception passes a list. Compression
therefore measures the gap against the **highest** defence score in the set, so
that adding a weak defence to a set can never increase compression and make that
weak defence decisive.

---

## File Structure

**Modify:**

| Path | Change |
|---|---|
| `internal/configs/config.balance.go` | 3 new `ConfigFloat` fields |
| `internal/configs/config.balance.combat.go` | validator for `ContestGapCompression` |
| `internal/configs/config.balance.mobs.go` | validators for the two archetype weights |
| `internal/combat/run_contest.go` | apply compression; add `compressContestGap` |
| `internal/mobs/mobs.go:546-560` | weighted archetype distribution |
| `_datafiles/config.yaml` | 3 keys + comments |
| `_datafiles/world/dogmud/mobs/instance_planar_oasis/318-sand_elemental.yaml` | `statpool: 1` → `2` (tier fix) |
| `_datafiles/world/dogmud/mobs/instance_planar_oasis/319-storm_elemental.yaml` | `statpool: 1` → `2` (tier fix) |

**Create:**

| Path | Responsibility |
|---|---|
| `internal/combat/gap_compression_test.go` | identity, parity invariance, monotonicity, ahead-only, multi-entry rule |
| `internal/combat/gap_compression_guard_test.go` | `RunContest` is the only site applying compression |
| `internal/configs/gap_compression_config_test.go` | absent-key safety and clamping for all three knobs |
| `internal/mobs/archetype_distribution_test.go` | normalisation, identity, measured distribution shape |

---

## Task 1: The compression function

Pure function first, with no config and no wiring, so its behaviour is pinned before anything depends on it.

**Files:**
- Modify: `internal/combat/run_contest.go`
- Create: `internal/combat/gap_compression_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/combat/gap_compression_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/stretchr/testify/assert"
)

// p = 1.0 must be a TRUE identity, not an approximation. It is the default, so
// any drift here is a silent balance change for every contest in the game.
func TestCompressContestGap_IdentityAtOne(t *testing.T) {
	for _, tc := range []struct{ atk, def float64 }{
		{455, 92}, {105, 185}, {200, 200}, {1, 1},
	} {
		got := compressContestGap(tc.atk, []contest.Entry{{Score: tc.def}}, 1.0)
		assert.Equal(t, tc.def, got[0].Score, "atk=%v def=%v", tc.atk, tc.def)
	}
}

// Compression must never touch a contest the attacker is not winning. This is
// what stops it buffing underdogs, which is a separate design decision.
func TestCompressContestGap_LeavesUnderdogsAlone(t *testing.T) {
	got := compressContestGap(105, []contest.Entry{{Score: 185}}, 0.5)
	assert.Equal(t, 185.0, got[0].Score, "an attacker behind on score must be unchanged")

	got = compressContestGap(200, []contest.Entry{{Score: 200}}, 0.5)
	assert.Equal(t, 200.0, got[0].Score, "an exactly even contest must be unchanged")
}

// The headline behaviour: a lead is compressed toward the defender.
func TestCompressContestGap_CompressesALead(t *testing.T) {
	// gap 363, p=0.5 -> sqrt(363) = 19.05 -> defence rises to 455 - 19.05
	got := compressContestGap(455, []contest.Entry{{Score: 92}}, 0.5)
	assert.InDelta(t, 435.95, got[0].Score, 0.01)

	// gap 363, p=0.75 -> 363^0.75 = 83.09 -> defence rises to 455 - 83.09
	got = compressContestGap(455, []contest.Entry{{Score: 92}}, 0.75)
	assert.InDelta(t, 371.91, got[0].Score, 0.01)
}

// A lower exponent must never produce a LARGER effective score. Monotonicity is
// what makes the knob dialable in play without surprises.
func TestCompressContestGap_MonotonicInExponent(t *testing.T) {
	prev := compressContestGap(455, []contest.Entry{{Score: 92}}, 1.0)[0].Score
	for _, p := range []float64{0.9, 0.85, 0.8, 0.75, 0.6, 0.5} {
		got := compressContestGap(455, []contest.Entry{{Score: 92}}, p)[0].Score
		assert.Greater(t, got, prev, "p=%v must raise the defence at least as far as the step above", p)
		prev = got
	}
}

// Each defence is compressed independently, and the ordering must survive: a
// stronger defence must stay stronger, or a mixed set could have its best
// defence overtaken by a worse one.
func TestCompressContestGap_PreservesDefenceOrdering(t *testing.T) {
	out := compressContestGap(455, []contest.Entry{
		{Score: 27}, {Score: 92}, {Score: 352},
	}, 0.75)

	assert.Less(t, out[0].Score, out[1].Score)
	assert.Less(t, out[1].Score, out[2].Score)
	for i, e := range out {
		assert.Less(t, e.Score, 455.0, "entry %d must stay below the attack score", i)
	}
}

// It must not mutate the caller's slice.
func TestCompressContestGap_DoesNotMutateInput(t *testing.T) {
	in := []contest.Entry{{Score: 92}}
	_ = compressContestGap(455, in, 0.75)
	assert.Equal(t, 92.0, in[0].Score, "the caller's entries must be untouched")
}

// Degenerate inputs must not produce NaN or a negative score.
func TestCompressContestGap_HandlesDegenerateInput(t *testing.T) {
	assert.Empty(t, compressContestGap(100, nil, 0.75), "no defenders, nothing to compress")
	assert.Empty(t, compressContestGap(100, []contest.Entry{}, 0.75))
	assert.Equal(t, 0.0, compressContestGap(0, []contest.Entry{{Score: 0}}, 0.75)[0].Score)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/combat/ -run TestCompressContestGap -v`
Expected: FAIL — `undefined: compressContestGap`

- [ ] **Step 3: Implement it**

In `internal/combat/run_contest.go`, add `"math"` to the imports and append:

```go
// compressContestGap narrows how far an attacker's score sits above the
// defence it is being rolled against, before either side is rolled.
//
//	effectiveDefence = attack - (attack - defence) ^ p   when attack > defence
//	effectiveDefence = defence                            otherwise
//
// WHY. Both rolls in a contest draw from ONE standard deviation taken from the
// attack score (contest.go:97,103), so the normalized margin is exactly
// N((A-D)/(0.15*A*sqrt2), 1) and crit fires when it clears a bar that floors at
// 1.5. That makes a 50% score edge a 52.8% crit rate, and a crit skips
// mitigation entirely. Compressing the gap is what pulls that back without
// touching parity.
//
// ⚠️ Only when the attacker is AHEAD. Symmetric compression would take an
// underdog from a 0.6% hit rate to 40.8%, which is a separate design decision
// about whether weak things can threaten strong ones and must not ride along
// with a crit fix.
//
// ⚠️ It raises the DEFENCE rather than lowering the attack, and that is
// load-bearing. contest.Run derives the roll spread from the attack score it is
// handed (contest.go:97) and rolls the defender with it too (:103). Lowering the
// attack would shrink the spread as well, and since crit is measured in units of
// that spread the compression would largely cancel itself: against a defence of
// 48, compressing the attack leaves crit at 94.3% where compressing the defence
// gives 28.7%. Moving the defence leaves atkScore, and the spread, as they are.
//
// ⚠️ Each entry is compressed independently, so a mixed defence set keeps its
// internal ordering: a stronger defence stays stronger.
//
// p == 1.0 telescopes to attack - (attack - defence) = defence, an exact
// identity, and is the default.
func compressContestGap(atkScore float64, entries []contest.Entry, p float64) []contest.Entry {
	if p >= 1.0 || len(entries) == 0 {
		return entries
	}

	out := make([]contest.Entry, len(entries))
	copy(out, entries)
	for i := range out {
		gap := atkScore - out[i].Score
		if gap <= 0 {
			continue // attacker is not ahead of THIS defence; leave it alone
		}
		out[i].Score = atkScore - math.Pow(gap, p)
	}
	return out
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/combat/ -run TestCompressContestGap -v`
Expected: PASS (6 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/combat/run_contest.go internal/combat/gap_compression_test.go
git commit -F - <<'EOF'
feat(combat): add the contest gap compression function

Pure function, not yet wired. Narrows how far an attacker sits above the
defence before either side rolls:

  effectiveDefence = attack - (attack-defence)^p   when ahead
  effectiveDefence = defence                        otherwise

p = 1.0 is an exact identity, pinned by test, because it is the default and
any drift is a silent balance change across all 25 contest call sites.

Ahead-only by design: symmetric compression takes an underdog from a 0.6%
hit rate to 40.8%, which is a separate design decision and must not ride
along with a crit fix.

Measures against the STRONGEST defence in a set so that adding a weak
defence can never increase compression and hand that defence the win.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
```

---

## Task 2: The compression config knob

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.combat.go`
- Create: `internal/configs/gap_compression_config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/gap_compression_config_test.go`:

```go
package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ⚠️ An absent key unmarshals to 0, and |gap|^0 == 1 -- every mismatch in the
// game would collapse to a one-point gap. So this validator must key on <= 0,
// never < 0. That exact trap left StealCooldown, StealHiddenBonus,
// ShadowCooldown, SneakFailCooldown and PackScatterRounds pinned at zero.
func TestContestGapCompression_AbsentKeyIsIdentity(t *testing.T) {
	b := Balance{}
	b.Validate()
	assert.Equal(t, ConfigFloat(1.0), b.ContestGapCompression,
		"an absent exponent must be 1.0 (no compression). If this is ever 0, "+
			"deleting the line from config.yaml collapses every mismatched "+
			"contest in the game to a one-point gap, with no error anywhere.")
}

func TestContestGapCompression_Clamped(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{-1.0, 1.0}, // nonsense
		{0.0, 1.0},  // reads as unset
		{1.5, 1.0},  // this knob only compresses, never expands
		{0.75, 0.75},
		{0.5, 0.5},
		{1.0, 1.0},
	} {
		b := Balance{ContestGapCompression: ConfigFloat(tc.in)}
		b.Validate()
		assert.Equal(t, ConfigFloat(tc.want), b.ContestGapCompression, "input %v", tc.in)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/configs/ -run TestContestGapCompression -v`
Expected: FAIL — `b.ContestGapCompression undefined`

- [ ] **Step 3: Declare the field**

In `internal/configs/config.balance.go`, immediately after the
`KnockdownFrequencyScale` line, add:

```go
	ContestGapCompression            ConfigFloat `yaml:"ContestGapCompression"`            // Exponent on an attacker's score lead before rolling. 1.0 = no compression (default). 0.75 shipped. Only reduces; values above 1.0 are refused
```

- [ ] **Step 4: Add the validator**

In `internal/configs/config.balance.combat.go`, inside `validateCombat()`, add:

```go
	// ⚠️ `<= 0`, NOT `< 0`. An absent key unmarshals to 0 and |gap|^0 == 1,
	// which would collapse every mismatched contest in the game to a one-point
	// gap. Same trap that pinned StealCooldown and four others at zero.
	// Above 1.0 is refused because this knob compresses; it must not expand.
	if b.ContestGapCompression <= 0 || b.ContestGapCompression > 1.0 {
		b.ContestGapCompression = 1.0
	}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/configs/ -run TestContestGapCompression -v`
Expected: PASS (2 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go internal/configs/gap_compression_config_test.go
git commit -m "feat(config): add ContestGapCompression, identity at its default

Validator keys on <= 0, not < 0: an absent key unmarshals to 0 and
|gap|^0 == 1, which would collapse every mismatched contest in the game to
a one-point gap. Pinned by a test that validates an empty Balance{}.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Wire compression into RunContest

**Files:**
- Modify: `internal/combat/run_contest.go`
- Modify: `internal/combat/gap_compression_test.go`
- Modify: `_datafiles/config.yaml`

- [ ] **Step 1: Write the failing test**

Append to `internal/combat/gap_compression_test.go`:

```go
// Parity must be invariant at EVERY exponent. Compression only ever touches
// mismatches, and that property is what makes this safe to ship behind a knob:
// it structurally cannot disturb an even fight.
func TestRunContest_ParityUnaffectedByCompression(t *testing.T) {
	const trials = 40000
	const score = 200.0

	rate := func(p float64) float64 {
		SetContestGapCompressionForTest(t, p)
		wins := 0
		for i := 0; i < trials; i++ {
			if RunContest(score, []contest.Entry{{Score: score}}).Success {
				wins++
			}
		}
		return float64(wins) / trials
	}

	for _, p := range []float64{1.0, 0.75, 0.5} {
		assert.InDelta(t, 0.50, rate(p), 0.02, "parity win rate at p=%v", p)
	}
}

// The headline: a large lead wins less overwhelmingly once compressed.
func TestRunContest_CompressionReducesLopsidedWins(t *testing.T) {
	const trials = 40000

	rate := func(p float64) float64 {
		SetContestGapCompressionForTest(t, p)
		wins := 0
		for i := 0; i < trials; i++ {
			if RunContest(455, []contest.Entry{{Score: 92}}).Success {
				wins++
			}
		}
		return float64(wins) / trials
	}

	full := rate(1.0)
	compressed := rate(0.75)
	assert.Greater(t, full, 0.99, "uncompressed, a 455-vs-92 attacker wins nearly always")
	assert.Less(t, compressed, full, "compression must reduce a lopsided win rate")
	assert.Greater(t, compressed, 0.90, "but 0.75 should still leave a strong attacker dominant")
}
```

- [ ] **Step 2: Add the test seam**

In `internal/combat/run_contest.go`, append:

```go
// SetContestGapCompressionForTest overrides the configured exponent for one
// test and restores it afterwards. Tests cannot use config.yaml: a Go test
// binary never loads it, so the knob reads its struct zero value.
func SetContestGapCompressionForTest(t *testing.T, p float64) {
	t.Helper()
	prev := contestGapCompressionOverride
	contestGapCompressionOverride = &p
	t.Cleanup(func() { contestGapCompressionOverride = prev })
}

var contestGapCompressionOverride *float64

func contestGapCompression() float64 {
	if contestGapCompressionOverride != nil {
		return *contestGapCompressionOverride
	}
	p := float64(configs.GetBalanceConfig().ContestGapCompression)
	if p <= 0 || p > 1.0 {
		// A test binary never loads config.yaml, so the field reads 0 here.
		// Identity is the only safe reading of "unset".
		return 1.0
	}
	return p
}
```

Add `"testing"` to the imports of that file.

- [ ] **Step 3: Apply it in RunContest**

Replace the body of `RunContest`:

```go
func RunContest(atkScore float64, entries []contest.Entry) contest.Result {
	// Compression happens HERE, before the roll, and nowhere else. Putting it
	// at the single entry point is what makes it impossible for a contest to
	// opt out silently -- see gap_compression_guard_test.go.
	// Compresses the DEFENCE entries, never atkScore: contest.Run derives the
	// roll spread from the attack score, so lowering it would shrink the spread
	// and cancel most of the compression.
	entries = compressContestGap(atkScore, entries, contestGapCompression())
	return contest.RunWithFloors(atkScore, entries, float64(configs.GetBalanceConfig().ContestFloor))
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/combat/ -run "TestRunContest_Parity|TestRunContest_Compression" -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Add the config key, respecting skip-worktree**

`_datafiles/config.yaml` carries `skip-worktree`. Build the commit from the
HEAD blob, not from disk:

```bash
SP=$(mktemp -d) && echo "$SP"
cp _datafiles/config.yaml "$SP/disk.new"
git show HEAD:_datafiles/config.yaml > "$SP/head.new"
```

Now edit **both** `$SP/disk.new` and `$SP/head.new`, inserting the block below
immediately after the `ContestFloor` entry. `disk.new` becomes the copy that
goes back on disk (it carries the owner's local settings); `head.new` becomes
the copy that is committed. They must receive the SAME edit.

```yaml
  #
  # ContestGapCompression: exponent applied to an attacker's score LEAD before
  # either side rolls.  effectiveDefence = attack - (attack-defence)^p
  #
  # Both rolls in a contest share one standard deviation taken from the attack
  # score, so the normalized margin is N((A-D)/(0.15*A*sqrt2), 1) and crit
  # fires when it clears a bar that floors at 1.5. Uncompressed that makes a
  # 50% score edge a 52.8% crit rate, and a crit SKIPS MITIGATION entirely.
  #
  #   ratio   hit%   crit%     (p = 1.0)
  #    1.00   50.0     6.7
  #    1.50   94.2    52.8
  #    3.00   99.9    95.0
  #
  # At p = 0.75, Meirok (455) against a 92-defence elemental goes from
  # 98.8% crit to 77.0%, and against a 352-defence royal from 33.3% to 13.5%.
  #
  # ⚠️ Parity is INVARIANT at every exponent -- compression only touches
  # mismatches, so it cannot disturb an even fight.
  # ⚠️ Applies only when the attacker is AHEAD. Symmetric compression would
  # take an underdog from a 0.6% hit rate to 40.8%, a separate decision.
  # ⚠️ Reaches EVERY opposed contest, not just melee: steal, sneak, flee and
  # the grapple family all resolve through combat.RunContest.
  # ⚠️ 0 is NOT an off-switch. It reads as unset and becomes 1.0, so deleting
  # this line cannot silently collapse every mismatch to a one-point gap.
  # Lower = flatter. 1.0 = off.
  ContestGapCompression: 0.80
```

Then commit from the HEAD-derived copy and restore the disk copy:

```bash
git update-index --no-skip-worktree _datafiles/config.yaml
cp "$SP/head.new" _datafiles/config.yaml
git add _datafiles/config.yaml internal/combat/run_contest.go internal/combat/gap_compression_test.go
git diff --cached _datafiles/config.yaml | grep -E "^-" | grep -v "^---"   # expect NO removals
git commit -m "feat(combat): compress the attacker's score lead before rolling

Applied in RunContest, the single entry point for all 25 opposed-contest
call sites, so no contest can opt out silently.

Ships at 0.80. Parity is invariant at every exponent, which is the property
that makes this safe behind a knob.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
cp "$SP/disk.new" _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml    # expect lowercase 'S'
```

---

## Task 4: Guard that compression has exactly one site

**Files:**
- Create: `internal/combat/gap_compression_guard_test.go`

- [ ] **Step 1: Write the guard**

```go
package combat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compression must be applied in exactly ONE place. A second site would double
// compress; a bypassing site would opt that contest out of the change without
// anybody noticing, which is precisely the failure mode the arc's single-seam
// design exists to prevent.
func TestCompressionHasExactlyOneCallSite(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	sites := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		if n := strings.Count(string(src), "compressContestGap("); n > 0 {
			sites[e.Name()] = n
		}
	}

	assert.Equal(t, map[string]int{"run_contest.go": 2}, sites,
		"compressContestGap must appear only in run_contest.go: once as its "+
			"definition and once at the RunContest call. A second call site "+
			"double-compresses; a contest that bypasses RunContest opts out of "+
			"the change silently.")
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/combat/ -run TestCompressionHasExactlyOneCallSite -v`
Expected: PASS

- [ ] **Step 3: Prove it can fail**

Temporarily add `_ = compressContestGap` to any non-test file in
`internal/combat/`, re-run, confirm FAIL, then remove it and confirm PASS.
A guard that cannot fail is not evidence.

- [ ] **Step 4: Commit**

```bash
git add internal/combat/gap_compression_guard_test.go
git commit -m "test(combat): guard that gap compression has exactly one call site

A second site double-compresses; a contest bypassing RunContest opts out of
the change silently. Verified the guard fails by adding a second reference.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Archetype weight config

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.mobs.go`
- Modify: `internal/configs/gap_compression_config_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/configs/gap_compression_config_test.go`:

```go
// Absent weights must reproduce TODAY's 80/20 archetype split, not 0/0. A zero
// weight pair would make every distribution degenerate.
func TestArchetypeWeights_AbsentKeysKeepTodaysSplit(t *testing.T) {
	b := Balance{}
	b.Validate()
	assert.Equal(t, ConfigFloat(0.80/3), b.ArchetypePrimaryStatWeight,
		"absent primary weight must reproduce today's 26.7% per primary stat")
	assert.Equal(t, ConfigFloat(0.20/3), b.ArchetypeSecondaryStatWeight,
		"absent secondary weight must reproduce today's 6.7% per non-primary stat")
}

func TestArchetypeWeights_RejectNonPositive(t *testing.T) {
	b := Balance{
		ArchetypePrimaryStatWeight:   ConfigFloat(-1),
		ArchetypeSecondaryStatWeight: ConfigFloat(0),
	}
	b.Validate()
	assert.Equal(t, ConfigFloat(0.80/3), b.ArchetypePrimaryStatWeight)
	assert.Equal(t, ConfigFloat(0.20/3), b.ArchetypeSecondaryStatWeight)
}

// A secondary weight above the primary would inverte the archetype. Refuse it.
func TestArchetypeWeights_SecondaryMayNotExceedPrimary(t *testing.T) {
	b := Balance{
		ArchetypePrimaryStatWeight:   ConfigFloat(0.15),
		ArchetypeSecondaryStatWeight: ConfigFloat(0.25),
	}
	b.Validate()
	assert.Equal(t, b.ArchetypePrimaryStatWeight, b.ArchetypeSecondaryStatWeight,
		"a secondary weight above the primary inverts the archetype; clamp to equal")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/configs/ -run TestArchetypeWeights -v`
Expected: FAIL — fields undefined

- [ ] **Step 3: Declare the fields**

In `internal/configs/config.balance.go`, after `MobDamageMultiplier`:

```go
	ArchetypePrimaryStatWeight       ConfigFloat `yaml:"ArchetypePrimaryStatWeight"`       // Relative weight per PRIMARY stat when distributing a mob's stat pool. Normalised in code (default 0.2667 = today's 80%/3)
	ArchetypeSecondaryStatWeight     ConfigFloat `yaml:"ArchetypeSecondaryStatWeight"`     // Relative weight per NON-PRIMARY stat. Equal to primary reproduces the uniform archetype (default 0.0667 = today's 20%/3)
```

- [ ] **Step 4: Add the validators**

In `internal/configs/config.balance.mobs.go`, inside `validateMobs()`:

```go
	// Absent keys must reproduce TODAY's split, not zero: a zero pair makes
	// every archetype distribution degenerate. Weights are RELATIVE and are
	// normalised at the point of use, so they need not sum to anything.
	if b.ArchetypePrimaryStatWeight <= 0 {
		b.ArchetypePrimaryStatWeight = ConfigFloat(0.80 / 3)
	}
	if b.ArchetypeSecondaryStatWeight <= 0 {
		b.ArchetypeSecondaryStatWeight = ConfigFloat(0.20 / 3)
	}
	// A secondary weight above the primary would invert the archetype, making a
	// "fighting" mob favour mental stats. Clamp rather than honour it.
	if b.ArchetypeSecondaryStatWeight > b.ArchetypePrimaryStatWeight {
		b.ArchetypeSecondaryStatWeight = b.ArchetypePrimaryStatWeight
	}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/configs/ -run TestArchetypeWeights -v`
Expected: PASS (3 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/configs/
git commit -m "feat(config): add archetype stat distribution weights

Relative weights, normalised at the point of use, so an author writes the
ratio they mean and cannot express a distribution that fails to sum to 1.

Absent keys reproduce TODAY's 80/20 split rather than zero, since a zero
pair makes every distribution degenerate. A secondary weight above the
primary is clamped: it would invert the archetype.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Apply the weights to stat distribution

**Files:**
- Modify: `internal/mobs/mobs.go:546-560`
- Create: `internal/mobs/archetype_distribution_test.go`
- Modify: `_datafiles/config.yaml`

- [ ] **Step 1: Write the failing test**

Create `internal/mobs/archetype_distribution_test.go`:

```go
package mobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Weights are RELATIVE: 3*0.25 + 3*0.15 = 1.2, which is not a distribution.
// Normalisation is what lets an author write the ratio they mean.
func TestArchetypeWeights_Normalise(t *testing.T) {
	primary, secondary := archetypeStatShares(0.25, 0.15)

	assert.InDelta(t, 0.2083, primary, 0.0001)
	assert.InDelta(t, 0.1250, secondary, 0.0001)
	assert.InDelta(t, 1.0, 3*primary+3*secondary, 1e-9,
		"the six shares must sum to exactly 1")
}

// Today's shipped behaviour must be reproducible, so the change can be switched
// off by configuration alone.
func TestArchetypeWeights_ReproduceTodaysSplit(t *testing.T) {
	primary, secondary := archetypeStatShares(0.80/3, 0.20/3)

	assert.InDelta(t, 0.80/3, primary, 1e-9)
	assert.InDelta(t, 0.20/3, secondary, 1e-9)
}

// Equal weights must reproduce the uniform ("") archetype exactly, so the knob
// spans the full range from today's specialisation to no archetype at all.
func TestArchetypeWeights_EqualIsUniform(t *testing.T) {
	primary, secondary := archetypeStatShares(0.2, 0.2)

	assert.InDelta(t, 1.0/6, primary, 1e-9)
	assert.InDelta(t, 1.0/6, secondary, 1e-9)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/mobs/ -run TestArchetypeWeights -v`
Expected: FAIL — `undefined: archetypeStatShares`

- [ ] **Step 3: Implement the share helper**

In `internal/mobs/mobs.go`, above `newMobByIdInternal`:

```go
// archetypeStatShares normalises the two relative archetype weights into
// per-stat probabilities. There are three primary stats and three non-primary.
//
// The weights are RELATIVE on purpose: the owner's chosen values, 0.25 and
// 0.15, sum to 1.2 across six stats and are not a distribution. Normalising
// here means an author writes the ratio they mean and cannot express something
// that fails to sum to 1.
//
// Equal weights reproduce the uniform ("") archetype exactly, so the knob spans
// the full range from today's 80/20 specialisation to no archetype at all.
func archetypeStatShares(primaryWeight, secondaryWeight float64) (primary, secondary float64) {
	total := 3*primaryWeight + 3*secondaryWeight
	if total <= 0 {
		return 1.0 / 6, 1.0 / 6
	}
	return primaryWeight / total, secondaryWeight / total
}
```

- [ ] **Step 4: Use it in the distribution loop**

> The loop is extracted into its own function in **Task 7 Step 1**. Do the
> minimal in-place edit here and let Task 7 move it; doing both at once makes
> the "pure move" check in Task 7 Step 2 meaningless.

In `internal/mobs/mobs.go`, replace the `case "fighting":` and `case "casting":`
arms of the `switch mob.Archetype` (currently lines 548-560) with:

```go
				case "fighting":
					// Weighted split, configurable since 2026-08-30. Was a
					// hardcoded 80/20, which gave a CASTING mob's Dexterity one
					// fifteenth of its pool -- and Dexterity is the whole
					// physical defence term, so a 1300-pool Elemental Queen
					// defended exactly as well as a 325-pool sand elemental.
					if util.Rand(1000) < int(primaryShare*3*1000) {
						statIdx = util.Rand(3) // 0=Str, 1=Dex, 2=Vit
					} else {
						statIdx = 3 + util.Rand(3) // 3=Per, 4=Wil, 5=Cha
					}
				case "casting":
					if util.Rand(1000) < int(primaryShare*3*1000) {
						statIdx = 3 + util.Rand(3)
					} else {
						statIdx = util.Rand(3)
					}
```

and immediately before the `for i := 0; i < statPool; i++ {` loop, add:

```go
			bal := configs.GetBalanceConfig()
			primaryShare, _ := archetypeStatShares(
				float64(bal.ArchetypePrimaryStatWeight),
				float64(bal.ArchetypeSecondaryStatWeight))
```

> **Do NOT touch the `tank` arm.** It carries its own bespoke six-way
> distribution (25% Cha, 20% Vit, 15% each Str/Dex/Wil, 10% Per) that is not a
> primary/secondary split and is out of scope for this change.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/mobs/ -run TestArchetypeWeights -v && go test ./internal/mobs/`
Expected: PASS, then `ok`

- [ ] **Step 6: Add the config keys**

Using the same skip-worktree procedure as Task 3 Step 5, add after
`MobDamageMultiplier`:

```yaml
  #
  # Archetype stat distribution. RELATIVE weights, normalised in code, so they
  # need not sum to anything. Three primary stats and three non-primary.
  #
  # Today's shipped behaviour was a hardcoded 80/20 group split, i.e. 0.2667 and
  # 0.0667 here. That gave a CASTING mob's Dexterity one fifteenth of its pool,
  # and Dexterity is the whole physical defence term -- so the 1300-pool
  # Elemental Queen defended exactly as well as a 325-pool sand elemental.
  #
  # 0.25 / 0.15 normalises to 0.2083 and 0.1250, a 62.5/37.5 group split.
  #
  # ⚠️ The pool is FIXED, so raising the non-primary share necessarily lowers
  # the primary share. Casters gain physical defence; FIGHTERS LOSE IT. Against
  # Meirok, a royal fighter's Dexterity falls 347 -> 271 and its crit-taken rate
  # rises 31.5% -> 66.7% on this change ALONE. It is only acceptable alongside
  # ContestGapCompression, which takes it to 19.7%.
  # ⚠️ Equal weights reproduce the uniform archetype exactly.
  # ⚠️ Flattening also flattens mob OFFENCE: a caster's Willpower share falls
  # from 26.7% to 20.8%, so its spells soften too.
  # ⚠️ Distribution happens ONCE at spawn. Existing instances keep their rolled
  # stats until they despawn; wipe instance saves to see this on live content.
  ArchetypePrimaryStatWeight: 0.25
  ArchetypeSecondaryStatWeight: 0.15
```

- [ ] **Step 7: Commit**

```bash
git commit -m "feat(mobs): weight archetype stat distribution by config

Was a hardcoded 80/20, giving a casting mob's Dexterity one fifteenth of
its pool. Dexterity is the whole physical defence term, so the 1300-pool
Elemental Queen defended exactly as well as a 325-pool sand elemental.

Weights are relative and normalised in code, so an author writes the ratio
they mean. Equal weights reproduce the uniform archetype exactly, so the
knob spans today's specialisation through to no archetype.

The tank archetype is deliberately untouched: it carries a bespoke six-way
distribution that is not a primary/secondary split.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Extract the distribution loop, then measure its shape

A normalisation unit test proves the arithmetic, not the loop. The loop is
currently buried inside `newMobByIdInternal` and cannot be called directly, so
extract it first as a pure move.

**Files:**
- Modify: `internal/mobs/mobs.go`
- Modify: `internal/mobs/archetype_distribution_test.go`

- [ ] **Step 1: Extract the loop, no behaviour change**

In `internal/mobs/mobs.go`, move the whole `for i := 0; i < statPool; i++ {...}`
body out of `newMobByIdInternal` into:

```go
// distributeStatPool spreads statPool points across the six stats according to
// the mob's archetype. Extracted from newMobByIdInternal so the resulting
// distribution can be measured directly; behaviour is unchanged by the move.
//
// primaryShare is the per-stat probability for one PRIMARY stat, already
// normalised by archetypeStatShares.
func distributeStatPool(mob *Mob, statPool int, primaryShare float64) {
	for i := 0; i < statPool; i++ {
		var statIdx int
		switch mob.Archetype {
		case "fighting":
			if util.Rand(1000) < int(primaryShare*3*1000) {
				statIdx = util.Rand(3)
			} else {
				statIdx = 3 + util.Rand(3)
			}
		case "casting":
			if util.Rand(1000) < int(primaryShare*3*1000) {
				statIdx = 3 + util.Rand(3)
			} else {
				statIdx = util.Rand(3)
			}
		case "tank":
			// Bespoke six-way split, deliberately NOT a primary/secondary
			// distribution and out of scope for the archetype weights.
			r := util.Rand(100)
			switch {
			case r < 25:
				statIdx = 5
			case r < 45:
				statIdx = 2
			case r < 60:
				statIdx = 0
			case r < 75:
				statIdx = 1
			case r < 90:
				statIdx = 4
			default:
				statIdx = 3
			}
		default:
			statIdx = util.Rand(6)
		}

		switch statIdx {
		case 0:
			mob.Character.Stats.Strength.Base++
		case 1:
			mob.Character.Stats.Dexterity.Base++
		case 2:
			mob.Character.Stats.Vitality.Base++
		case 3:
			mob.Character.Stats.Perception.Base++
		case 4:
			mob.Character.Stats.Willpower.Base++
		case 5:
			mob.Character.Stats.Charisma.Base++
		}
	}
}
```

Replace the removed block in `newMobByIdInternal` with:

```go
			bal := configs.GetBalanceConfig()
			primaryShare, _ := archetypeStatShares(
				float64(bal.ArchetypePrimaryStatWeight),
				float64(bal.ArchetypeSecondaryStatWeight))
			distributeStatPool(&mob, statPool, primaryShare)
```

- [ ] **Step 2: Verify the move changed nothing**

Run: `go test ./internal/mobs/`
Expected: `ok`. If anything fails, the move was not pure — fix it before
continuing rather than adjusting a test.

- [ ] **Step 3: Write the measurement test**

Append to `internal/mobs/archetype_distribution_test.go`:

```go
// The unit tests above prove the ARITHMETIC. This proves the LOOP: that a
// spawned mob's stats actually land in the intended proportions.
func TestArchetypeDistribution_MatchesIntendedShares(t *testing.T) {
	const pool = 60000 // large enough that sampling noise is well under a percent

	primaryShare, secondaryShare := archetypeStatShares(0.25, 0.15)

	m := &Mob{}
	m.Archetype = "casting"

	before := m.Character.Stats.Strength.Base + m.Character.Stats.Dexterity.Base +
		m.Character.Stats.Vitality.Base + m.Character.Stats.Perception.Base +
		m.Character.Stats.Willpower.Base + m.Character.Stats.Charisma.Base

	distributeStatPool(m, pool, primaryShare)

	s := m.Character.Stats
	physical := s.Strength.Base + s.Dexterity.Base + s.Vitality.Base
	mental := s.Perception.Base + s.Willpower.Base + s.Charisma.Base
	require.Equal(t, pool, physical+mental-before, "every point must land somewhere")

	// A casting mob's PHYSICAL stats are the NON-primary group.
	assert.InDelta(t, 3*secondaryShare, float64(physical)/float64(pool), 0.02,
		"casting mob physical share")
	assert.InDelta(t, 3*primaryShare, float64(mental)/float64(pool), 0.02,
		"casting mob mental share")
}

// The mirror: a fighting mob's physical stats are the PRIMARY group.
func TestArchetypeDistribution_FightingMirrorsCasting(t *testing.T) {
	const pool = 60000
	primaryShare, _ := archetypeStatShares(0.25, 0.15)

	m := &Mob{}
	m.Archetype = "fighting"
	distributeStatPool(m, pool, primaryShare)

	s := m.Character.Stats
	physical := s.Strength.Base + s.Dexterity.Base + s.Vitality.Base
	assert.InDelta(t, 3*primaryShare, float64(physical)/float64(pool), 0.02)
}
```

Add `"github.com/stretchr/testify/require"` to that file's imports.

- [ ] **Step 4: Run**

Run: `go test ./internal/mobs/ -run TestArchetypeDistribution -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/mobs/
git commit -m "test(mobs): extract the stat-pool loop and pin its measured shape

Normalisation arithmetic and the distribution LOOP are different claims.
The loop was buried in newMobByIdInternal and could not be called, so it is
extracted as a pure move first, verified by running the mobs suite before
and after.

The tank archetype keeps its bespoke six-way split unchanged.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Joint verification

Neither change is shippable alone. This task proves the combined effect matches the spec before any human plays it.

**Files:**
- Create: `internal/combat/oasis_regression_test.go`

- [ ] **Step 1: Write the test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/stretchr/testify/assert"
)

// The spec's headline claim, measured against live dice rather than the model.
//
// Neither change is shippable alone: compression leaves the Elemental Queen at
// ~97% crit, and redistribution ALONE takes the royal fighters from 31.5% to
// 66.7% crit by stripping 76 Dexterity. This pins the combined outcome.
func TestOasisCritRatesAfterCompression(t *testing.T) {
	const trials = 40000
	const meirokAttack = 455.0

	critRate := func(def, p float64) float64 {
		SetContestGapCompressionForTest(t, p)
		crits := 0
		for i := 0; i < trials; i++ {
			res := RunContest(meirokAttack, []contest.Entry{{Score: def}})
			if !res.Floored && AttackContestCritAt(res.Margin, res.AttackRoll, 1.5) {
				crits++
			}
		}
		return float64(crits) / trials
	}

	// 92 defence: sand elemental, storm elemental and the Queen all land here
	// today, which is exactly the archetype problem Task 6 addresses.
	assert.Greater(t, critRate(92, 1.0), 0.95, "uncompressed, a 92-defence mob is crit nearly every hit")
	assert.Less(t, critRate(92, 0.75), 0.85, "compression must pull that back materially")

	// 352 defence: the royal fighters, the one tier already behaving.
	assert.Less(t, critRate(352, 0.75), critRate(352, 1.0),
		"compression must not make a competitive matchup worse")

	// 162 defence: the Queen AFTER Task 6 redistribution.
	assert.Less(t, critRate(162, 0.75), 0.70,
		"the Queen after redistribution plus compression must be well off the ceiling")
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/combat/ -run TestOasisCritRates -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/combat/oasis_regression_test.go
git commit -m "test(combat): pin the combined oasis crit outcome

Measured against live dice, not the analytic model. Neither change ships
alone, so this asserts the joint result: a 92-defence mob falls off the
crit ceiling, the royal fighters do not get worse, and the Queen after
redistribution lands well clear of saturation.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Pre-push gates

- [ ] **Step 1: Format and build**

```bash
gofmt -l internal/ modules/     # expect no output
go build ./...                  # expect no output
```

- [ ] **Step 2: Full suite**

```bash
go test ./internal/...
```
Expected: no `FAIL` lines. Note ~56 tests fail under `-count=2` for
pre-existing reasons unrelated to this branch; do not chase them.

- [ ] **Step 3: Confirm `LogToFile: false`**

```bash
grep -n "LogToFile" _datafiles/config.yaml
```
Expected: `false`.

- [ ] **Step 4: Isolated boot test**

```bash
git worktree add --detach C:/tmp/dogmud-gapc HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-gapc/_datafiles/config.yaml
cd C:/tmp/dogmud-gapc && go build -o boot.exe .
timeout 180 ./boot.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is SUCCESS** — the timeout fired because the server stayed up.
Do NOT grep for the bare word `panic`: `GamePlay.MapConsistencyEnforce`
legitimately has the *value* `panic`.

Clean up: `git worktree remove --force C:/tmp/dogmud-gapc`

- [ ] **Step 5: Wipe instance saves**

Stat distribution happens once at spawn, so existing instances keep their old
rolled stats. With the server DOWN:

```bash
rm -rf _datafiles/world/dogmud/mobs.instances _datafiles/world/dogmud/rooms.instances
```

> Do NOT wipe `shops/`, `guilds/` or `moderation/` — persistent living state.

---

## Task 10: Adversarial playtest gate

Required by the standing content SOP, and doubly so here: this touches every opposed contest in the game.

- [ ] **Step 1: Write the goals file**

Create a goals file under `tools/playtest/goals/` with `ephemeral:`. Cover:

1. **Combat feel against mismatched content** — does beating trash still feel
   like beating trash, or does it now feel grindy? Crit should be noticeably
   rarer, ordinary hits more common.
2. **Competitive fights** — against something near your own level, nothing
   should have changed. Parity is invariant by construction; verify it is.
3. **🔴 The non-combat contests.** This reaches `steal`, `plant`, `sneak`,
   `shadow` and `flee`. A strong thief robbing a weak mark is now harder.
   Exercise each and report whether it feels broken or merely different.
4. **Mob defence** — casters should now survive melee noticeably longer;
   fighters slightly less long.

- [ ] **Step 2: Run it**

```
/playtest local --checkout <abs> bug-finder <goals>.yaml
```

> **Fixture warning.** Use content that survives several rounds. The Drill
> Yard's three NPCs cannot hit back at all, and nothing in the starting area can
> knock a player down. Budget the tutorial (`ask vorn train`) if the run needs
> real combat.

- [ ] **Step 3: Extract findings**

Playtest reports are gitignored. Anything worth keeping goes into a memory
topic file or `docs/audits/`, or it is lost.

- [ ] **Step 4: Open the PR**

```bash
git push -u origin feature/contest-gap-compression
gh pr create --repo pruuk/DOGMud --base master --head feature/contest-gap-compression --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run view <id> --repo pruuk/DOGMud --log-failed    # green is not proof
```

> 🚨 `gh` defaults to the fork PARENT. Every command MUST carry
> `--repo pruuk/DOGMud`.

Merge with `--merge` (no-ff), never `--squash`.

---

## Definition of Done

- [ ] `ContestGapCompression` ships at 0.80; `p=1.0` is a proven exact identity
- [ ] Parity win rate is 50% at every exponent, measured on live dice
- [ ] Underdog outcomes are unchanged at every exponent
- [ ] Compression has exactly one call site, guarded, and the guard is proven to fail
- [ ] Archetype weights ship at 0.25 / 0.15, normalised, with equal weights reproducing the uniform archetype
- [ ] Absent keys reproduce today's behaviour for all three knobs, each pinned by a test on an empty `Balance{}`
- [ ] Measured distribution shape matches intended shares
- [ ] Combined oasis outcome pinned against live dice
- [ ] `gofmt`, `go build`, full suite clean; isolated boot reaches `Server Ready`
- [ ] Instance saves wiped so the distribution change is visible
- [ ] Adversarial playtest covers steal / sneak / flee, not just combat
- [ ] PR open against `pruuk/DOGMud`, checks confirmed via `--log-failed`

---

## Out of scope, recorded so nobody widens it

- **`SkillWeight` 5.0 against mob combat skill 1**, the root of the score gap.
  Meirok's 48 ranks are 240 of his 455; every mob contributes 5.
- **Crits skipping mitigation.** Compression reduces how OFTEN a crit happens,
  not what it is worth (up to 17.6× against armour). Making crits mitigate
  before multiplying is a separate, larger decision.
- **The `tank` archetype's bespoke distribution.**
- ~~Oasis tier authoring~~ — **FIXED on this branch.** Sand and storm elementals
  carry gear and were always intended as tough (`statpool: 2`); both were
  authored `statpool: 1` and spawned at trash tier. Corrected before the tables
  above were computed, so every oasis figure in this plan uses the fixed pools.
