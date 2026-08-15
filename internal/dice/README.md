# Dice Package — Statistical Distribution Roller

A statistical dice rolling system for DOGMud based on normal (Gaussian)
distributions. Every attack, defense, spell check, grapple, and skill roll
in the engine flows through this package.

---

## The Master Randomness Knob: `RollSpread`

**This is the single most impactful balance lever in the game.**

All stat-based rolls compute their standard deviation as:

```
stdDev = stat_value × RollSpread
```

`RollSpread` defaults to **0.15** (15%) and is loaded from
`_datafiles/config.yaml` at server startup via `SetRollSpread`.  Changing it
re-scales the spread of *every single dice roll in the engine simultaneously*
— no individual files need to be touched.

### Effect on Gameplay

At the human baseline of `stat = 100`:

| RollSpread | stdDev | 120 vs 100 win% | Feel |
|------------|--------|-----------------|------|
| 0.05       | 5.0    | ~99%            | Near-deterministic. High stats dominate completely. |
| 0.10       | 10.0   | ~92%            | Low variance. Skill matters most. |
| **0.15**   | **15.0** | **~78%**      | **DEFAULT — balanced, recommended.** |
| 0.20       | 20.0   | ~67%            | Higher variance. Upsets are common. |
| 0.30       | 30.0   | ~57%            | Chaotic. Luck rivals skill. |

### Crit / Fumble Rates

**Not affected by `RollSpread`.** Criticals trigger when a roll's z-score
is ≥ +2.0 (or ≤ −2.0), which is always ~2.3% per roll regardless of the
absolute standard deviation — the z-score distribution is invariant to
scaling.

### Wiring

```go
// main.go — called once at startup after configs.ReloadConfig()
dice.SetRollSpread(float64(configs.GetGamePlayConfig().RollSpread))
```

Changing `config.yaml` requires a server restart.

---

## Stat-Based Roll API (use these for all game mechanics)

### `RollStat(mean float64) RollResult`

Single stat-based roll. Standard deviation is derived automatically from
`StdDevFor(mean)`. Use this for skill checks, damage/heal rolls, and
knockdown-chance rolls.

```go
result := dice.RollStat(attackerStat)
// result.Value    — the actual roll outcome
// result.ZScore   — >= 2.0 crit, <= -2.0 fumble
```

### Opposed contests moved out of this package (chunk U4)

**Do not call an opposed roll from `internal/dice`.** Every contest in the game
now resolves through `internal/contest`, reached by `combat.RunContest`
(`internal/combat/run_contest.go`):

```go
if combat.RunContest(attackScore, []contest.Entry{{Score: defenseScore}}).Success {
    // succeeded
}
```

U4 routed callers through three wrappers over per-channel floor pairs, picked by
the cost of a single failure. U6 deleted all three in favour of one symmetric
`Balance.ContestFloor`.

The reason for having a floor at all is unchanged (chunk 5.9a/5.10): an outmatched initiator
keeps a last-resort chance of success, and an overwhelming favourite is never
certain. Without them a stat-100 thief against a stat-150 mark succeeded 0.9% of
the time and a stat-200 thief against a stat-100 mark succeeded 99.1%.

### The four legacy opposed functions, all guarded

`OpposedRollStat`, `OpposedRollStatWithFloors`, `OpposedRollStatRaw` and
`OpposedRoll` still exist, and `contest_floor_guard_test.go` at the repo root
fails if any is called outside `internal/dice` without an exemption entry
carrying a written reason. Two different reasons:

- `OpposedRollStatRaw` / `OpposedRoll` apply **no floor**. That is the original
  chunk 5.9 hazard.
- `OpposedRollStat` / `OpposedRollStatWithFloors` are floored but
  **`Deprecated:`** with zero production callers as of U4. The risk guarded here
  is drift back onto the legacy path. U6 deletes them.

`contest.Run` and `contest.AgainstDifficulty` are guarded too, and are genuinely
unfloored: migrating a floored caller onto either silently deletes its floor.

One caller legitimately floors on its own -- combat's `resolveAttack` floors a
computed hit *chance* rather than a roll outcome, so it cannot route through a
roll-level floor. It is exempted by file, not by directory.

---

## Low-Level API (explicit stdDev — use sparingly)

These exist for cases where variance is **not** proportional to a stat, such
as weapon damage variance derived from dice-notation item specs.

### `Roll(mean, stdDev float64) RollResult`
### `OpposedRoll(atk, def, stdDev float64) (bool, float64, RollResult, RollResult)`
### `StdDevFor(mean float64) float64`

Returns `mean × RollSpread` (floor 1.0).  Prefer calling `RollStat` directly, or
a `combat.RunWith*Floors` wrapper for a contest, rather than computing this
yourself.

---

## RollResult Fields

```go
type RollResult struct {
    Value       float64 // The actual roll outcome
    Mean        float64 // The distribution mean (stat value)
    StdDev      float64 // Standard deviation used
    ZScore      float64 // Standard deviations from mean (crit/fumble detection)
    Percentile  float64 // 0–100 percentile of this roll
    Success     bool    // True if the roll beat its check target
    Margin      float64 // Margin of success (positive) or failure (negative)
    Description string  // Human-readable description
}
```

---

## Z-Score Reference

| Z-Score | Percentile | Interpretation |
|---------|------------|----------------|
| −3.0    | 0.1%       | Extremely low  |
| −2.0    | 2.3%       | **Fumble threshold** |
| −1.0    | 15.9%      | Below average  |
|  0.0    | 50.0%      | Average        |
| +1.0    | 84.1%      | Above average  |
| +2.0    | 97.7%      | **Crit threshold** |
| +3.0    | 99.9%      | Extremely high |

---

## Utility Functions

```go
dice.RollDamage(base, variance, min float64) float64   // explicit variance damage
dice.DiceToDistribution(dCount, dSides, bonus int)     // NdM+bonus → (mean, stdDev)
dice.Percentile(chance float64) (bool, float64)        // simple % check
dice.RollBetween(min, max float64) float64
dice.RollBetweenInt(min, max int) int
dice.RollTable(weights []int) int                      // weighted loot table
```

---

## Testing

```bash
go test ./internal/dice/... -v
go test ./internal/dice/... -bench=.
```
