# internal/lightscale

The arithmetic of the graded light scale. Pure: no config, no globals, no locks.

## What it is for

The scale runs -100 to 100 and is perceptual, not linear. Zero is the darkest
naturally occurring light (an unlit cave). Negative is magical darkness.

One constant relates the scale to physical light: the **doubling step**, how
many points twice as much light is worth. It is `LightDoublingStep` in config,
shipped at 8, and it is passed in rather than read here.

## Surface

| Symbol | Purpose |
|---|---|
| `Absent() float64` | A term that is not present at all, distinct from a dark term |
| `Combine(step float64, terms ...float64) float64` | Every present term together |
| `Attenuate(step, light, fraction float64) float64` | A transmission fraction applied to one term |
| `Trim(step, others, max, target float64) float64` | The one solve: the output an adjustable source runs at so the combine of `others` and itself lands on `target` (`trim.go`) |
| `TrimDarkness(step, light, otherDark, max, floor float64) float64` | A darkness's trim (lighting plan 5d, ruling D2): `Trim(step, otherDark, max, light - floor)`, Absent light read as 0, Absent when the budget is 0 or less (`trim.go`) |

## Traps

- **Absent is not zero.** A cave has no sky; a sky contributing zero would make
  the cave brighter, because two terms at zero combine to one step above zero.
  `Attenuate` with fraction 0 returns `Absent()` for this reason.
- **A multiplier is a subtraction here.** Half the light is minus one step.
- `Combine` skips NaN as well as -Inf, so one bad caller cannot poison a room.
- Both functions coerce a non-positive step to 1 rather than dividing by zero.
  `Trim` does the same.
- **`Trim` solves the combine exactly; it is not `target - others`.** It
  returns `Absent()` when the combine already reaches `target` without the
  source, or when the needed term would fall below 0. A NaN `target` or `max`
  returns `Absent()`.
- **Darkness uses the same solve (lighting plan 5d).** Darknesses combine among
  themselves by the halving rule and the result is subtracted from the light,
  so keeping the room at or above `floor` is `Combine(otherDark, d) <= light -
  floor`: `Trim` with that budget as its target. The old linear `Darkens`
  branch and the `Polarity` type are DELETED; one solve serves both.

## Who uses it

`internal/gametime` (sun plus moons), `internal/rooms` (ambient plus lamp plus
carried light, minus carried darkness; `Trim` and `TrimDarkness` from
`light_trim.go`). Plan 4 added weather occlusion; plan 5d added darkness on
the same functions.
