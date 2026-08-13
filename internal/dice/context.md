# Dice Context

## Purpose

`internal/dice` is the engine's randomness. Every stat contest, attack, defence,
skill check, and damage roll goes through it. It is **distribution-based, not
die-based**: a roll is a sample from a normal distribution around a mean, so
crits and fumbles live in the tails rather than on a flat d20.

There is a companion write-up with win-probability tables in
`internal/dice/README.md`.

## The rule that matters

**For a single non-contested stat roll, call `RollStat`.** It takes only a mean
and derives the standard deviation from the global spread factor:

```go
func RollStat(mean float64) RollResult
func StdDevFor(mean float64) float64        // mean × RollSpread
func SetRollSpread(factor float64)
```

`Roll` takes an explicit `stdDev` and is **low-level**. Use it only where
variance is genuinely not proportional to the stat -- weapon damage variance
taken from an item spec, for example.

`util.Rand` and `util.LogRoll` are **not** used for hit or attack checks.

### Opposed contests do NOT live here any more

**Do not reach for an opposed roll in this package.** As of roadmap chunk U4,
every opposed contest in the game resolves through `internal/contest`, and
callers reach it through one of three wrappers in
`internal/combat/contest_floors.go`, chosen by the **cost of a single failure**:

| Wrapper | Floor pair | For |
|---|---|---|
| `combat.RunWithGlobalFloors` | global | out of combat: stealth, theft, traps, detection |
| `combat.RunWithManeuverFloors` | maneuver | maneuvers, flee -- burns the whole round |
| `combat.RunWithSpellFloors` | spell | spells |

All three return a `contest.Result`; read `.Success`.

`OpposedRollStat` and `OpposedRollStatWithFloors` still exist and are still
floored, but they are **`Deprecated:`** and have **zero production callers**.
U6 deletes them. They survive for two reasons only: `internal/dice` delegates
between them internally, and `internal/combat/global_floors_test.go` uses
`OpposedRollStat` as the oracle proving U4 was a no-op.

**Four functions here are now guarded** by `contest_floor_guard_test.go` at the
repo root, for two different reasons:

| Function | Guarded because |
|---|---|
| `OpposedRollStatRaw`, `OpposedRoll` | **unfloored** -- the original chunk 5.9 hazard |
| `OpposedRollStat`, `OpposedRollStatWithFloors` | **deprecated** -- the risk is drift back onto the legacy path |

Calling any of them outside `internal/dice` fails that test. `contest.Run` and
`contest.AgainstDifficulty` are guarded too, and are genuinely unfloored --
migrating a floored caller onto either silently deletes its floor.

A second guard, `floor_pair_guard_test.go`, pins which floor pair each migrated
site uses. All three pairs ship at `0.05`, so wiring a contest to the wrong one
is invisible to every behavioural test and becomes a live balance bug the moment
one pair is retuned.

## `RollSpread`

The single master randomness knob, `GamePlay.RollSpread` in `config.yaml`
(default `0.15`). Changing it rescales every roll in the engine at once.

Z-score thresholds are unaffected by it: `ZScore >= 2.0` is a critical,
`ZScore <= -2.0` is a fumble or backfire — roughly 2.3% each, whatever the
spread.

## Public API

Rolling:

```go
func Roll(mean, stdDev float64) RollResult
func RollInt(mean, stdDev float64) int
func RollClamped(mean, stdDev, min, max float64) RollResult
func RollIntClamped(mean, stdDev, min, max float64) int
func RollBetween(min, max float64) float64
func RollBetweenInt(min, max int) int
func RollTable(weights []int) int
func RollStatArray(count int, mean, stdDev, min, max float64) []int
```

Contests:

```go
// Floored (chunk 5.9a/5.10) -- use these.
func OpposedRollStat(atk, def float64) (bool, float64, RollResult, RollResult)
func OpposedRollStatWithFloors(atk, def, floorSuccess, floorResist float64) (bool, float64, RollResult, RollResult)
func SetContestFloors(minSuccess, minResist float64)
func ContestFloors() (minSuccess, minResist float64)

// Unfloored -- guarded by contest_floor_guard_test.go.
func OpposedRollStatRaw(atk, def float64) (bool, float64, RollResult, RollResult)
func OpposedRoll(attackerStat, defenderStat, stdDev float64) (bool, float64, RollResult, RollResult)

func DifficultyCheck(stat, difficulty, stdDev float64) RollResult
func CompareRolls(roll1, roll2 RollResult) int
func Percentile(chance float64) (bool, float64)
```

Damage:

```go
func RollDamage(baseDamage, variance, minDamage float64) float64
func RollDamageInt(baseDamage, variance, minDamage float64) int
```

Criticals:

```go
func CriticalCheck(result RollResult, criticalThreshold, fumbleThreshold float64) (bool, bool)
func RollWithCriticals(mean, stdDev, critThreshold, fumbleThreshold float64) (RollResult, bool, bool)
```

Analysis (no randomness — for tuning, tests, and documentation):

```go
func SuccessChance(stat, difficulty, stdDev float64) float64
func OpposedSuccessChance(attackerStat, defenderStat, stdDev float64) float64
func ExpectedMargin(stat, difficulty float64) float64
func AverageResult(mean, stdDev float64) float64
func GetPercentile(mean, stdDev, percentile float64) float64
func StandardDeviation(statRange, randomnessFactor float64) float64
func DiceToDistribution(dCount, dSides, bonus int) (mean, stdDev float64)
```

`DiceToDistribution` is the bridge for authored content still expressed as
`2d6+3`: it converts that notation to the equivalent mean and standard
deviation so the same distribution machinery can roll it.

## `RollResult`

Carries the rolled value plus the z-score, which is what makes crit and fumble
detection uniform across every subsystem. `String()` renders it for debug logs.

## Gotchas

- **`SetRollSpread` is global and process-wide.** Tests that change it must
  restore it, or every later test in the binary inherits the new variance.
- **Analysis functions are deterministic; rolling functions are not.** Do not
  use `SuccessChance` where you meant `Roll` — it will silently always succeed
  or always fail depending on how you compare it.
- **`OpposedRollStat` and `OpposedRollStatRaw` share a signature.** Any future
  rename in this area is therefore **not** compiler-verified: swapping the names
  in one pass leaves callers compiling fine against the other function's
  semantics. Chunk 5.10 did the swap in two ordered steps, each proving the old
  identifier was gone before the next began. Do the same, and note that
  `OpposedRollStat` is also a prefix of `OpposedRollStatWithFloors`, so a naive
  replace-all mangles the longer name.
- **A floor save is a BARE success.** When a floor flips an outcome, the margin
  is reduced to the smallest value carrying the new sign. Anything scaling an
  effect by margin must not read a floor save as a rout.
- **`RollTable` takes weights, not probabilities.** They need not sum to
  anything in particular.
- **Clamping changes the distribution.** `RollClamped` piles probability mass
  on the bounds; a clamped roll is no longer normal, and z-score-based crit
  detection on top of it will misbehave at the edges.
- **Never show a raw roll to a player.** Player-facing text uses descriptive
  language (`combat.GetDamageDescription`), not numbers.

## Dependencies

`configs` (for `RollSpread`) and the standard library `math`/`math/rand`.

## Consumers

`combat`, `hooks`, `mobs`, `characters`, `skills`, `crafting`, `spells`,
`justice` — effectively every gameplay system.
