# internal/lightscale

The arithmetic of the graded light scale. Pure: no config, no globals, no locks.

## What it is for

The scale runs -100 to 100 and is perceptual. Zero is the darkest a place can
be without active magical darkness (an unlit cave). Negative is magical
darkness and nothing else (lighting plan 6, owner ruling O1).

One constant relates the scale to physical light: the **doubling step**, how
many points twice as much light is worth. It is `LightDoublingStep` in config,
shipped at 8, and it is passed in rather than read here.

## The arithmetic (lighting plan 6)

Every operation works on **linear brightness**, not on points:

- `B(p) = 2^(p/step) - 1` for a light term, so `B(0) = 0`; a term at or below
  0, NaN, +Inf or `Absent()` reads `B = 0`.
- `p(B) = step * log2(1 + B)`, the inverse.
- `Combine` is `p(sum of B)`; `Attenuate` is `p(fraction * B)`.

So no sum or fraction of real light can read below 0, a single term reads
itself, and two equal terms read a little under one step brighter (52 and 52
read 59.9). Well above the dim end the result matches the old log-domain
operators to within half a point; the gap shrinks as `2^(-p/step)` and is
under 0.5 for readings at or above about 37 at step 8 (not 25: at 25 two equal
terms read about 1.4 lower than before).

## Surface

| Symbol | Purpose |
|---|---|
| `Absent() float64` | A marker (`-Inf`) for a term that is not there: a sun below the horizon, a trimmed source switched off, an unlit fixture. Reads as `B = 0`; `Combine` and `Attenuate` never return it |
| `Combine(step float64, terms ...float64) float64` | Every term together, on the linear sum; 0 for none |
| `Attenuate(step, light, fraction float64) float64` | A transmission fraction applied to one term; never below 0; 0 for a fraction at or below 0 |
| `Trim(step, others, max, target float64) float64` | The one solve: the output an adjustable source runs at so `B(target) = B(others) + B(out)`, capped at `max` (`trim.go`) |
| `TrimDarkness(step, light, otherDark, max, floor float64) float64` | A darkness's trim (lighting plan 5d, ruling D2): `Trim(step, otherDark, max, light - floor)`, Absent light read as 0, Absent when the budget is 0 or less (`trim.go`) |

## Traps

- **No sky and no light are the same statement now.** The old `-Inf`
  sentinel existed because two log-domain zeros combined one step brighter
  than one; `B(0) + B(0)` is still 0. `Absent()` survives only as the "off" /
  "not there" marker that `conditions.SetLightOutput` and `internal/itemlight`
  read; arithmetic treats it as 0.
- **A multiplier is not a fixed subtraction any more.** Half the light is very
  nearly one step down by day and less near 0, where there is little light
  left to take (a night storm takes 3 to 8 points, not 8).
- `Combine` skips NaN and reads negative light terms as 0, so one bad caller
  cannot poison a room and a light term can never darken one.
- Every function coerces a non-positive step to 1 rather than dividing by zero.
- **`Combine` and `Attenuate` always return a finite number.** `B(p)`
  overflows a float64 once `p/step` passes about 1024 (an uncapped Glow, or a
  tiny authored `LightDoublingStep`); both then fall back to the log domain
  relative to the brightest term, which is the linear sum to rounding at that
  magnitude. `B` and `p` use `Expm1` and `Log1p` so a sliver near 0 keeps its
  precision. The room still clamps in floating point before its int
  conversion (`rooms.levelOfRaw`), so nothing that slips past here can wrap a
  blinding room round to -100.
- **`Trim` solves the linear sum exactly; it is not `target - others`.** It
  returns `Absent()` when `others` already reaches `target`, or when the
  needed term has no brightness. With no other light it returns
  `min(target, max)`. A NaN `target` or `max` returns `Absent()`.
- **Darkness uses the same solve (lighting plan 5d).** Darknesses combine
  among themselves on the same sum and the room subtracts the result in
  points, so keeping the room at or above `floor` is
  `Combine(otherDark, d) <= light - floor`: `Trim` with that budget as its
  target.
- **The floor is not here.** `LightRealMinimum` (any real light reads at
  least 3) is the room's rule, applied in `rooms.composeWithFixtures`.

## Who uses it

`internal/gametime` (sun plus moons), `internal/rooms` (sky, lamp, fixtures
and carried light, minus darkness; `Trim` and `TrimDarkness` from
`light_trim.go`), `internal/characters` (infra reach sources combine through
`Combine`), `internal/behaviortree` (`Absent` as an item light's "off").
