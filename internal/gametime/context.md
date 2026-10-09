# Game Time Context

## Purpose

`internal/gametime` converts the server's monotonic round counter into an
in-world calendar — year, month, week, day, hour, minute, day/night — and
back again. Nothing here ticks: there is no goroutine and no stored clock.
Every answer is derived on demand from `util.GetRoundCount()` and the
`RoundsPerDay` / `RoundSeconds` timing config.

DOGMud extends the upstream calendar with **three moons** whose phases feed
stat modifiers and mutation pacing, and with a zodiac year name.

## Files

- **gametime.go** — `GameDate`, `RoundTimer`, day/night control, and the
  period-string arithmetic (`AddPeriod`, `GetLastPeriod`).
- **celestial.go**: the solar and lunar light model, `NightHoursAt`,
  `SunLight`, `MoonLight`, `CelestialLight`. Plan 3a of the graded lighting
  arc.
- **months.go** — `MonthName(month int) string`.
- **moonphase.go**: the three moons and their stat contribution, plus
  `PhasesAtRound`.
- **zodiac.go** — `GetZodiac(year int) string`.
- **copyover.go** — copyover contributor so a hot restart does not jump the
  clock.

## `GameDate`

```go
type GameDate struct {
    RoundNumber      uint64
    RoundsPerDay     int
    NightHoursPerDay int
    Year, Month, Week, Day, Hour, Hour24 int
    Minute      int
    MinuteFloat float64
    AmPm        string
    Night       bool
    DayStart, NightStart int
}
```

`GetDate(forceRound ...uint64) GameDate` is the entry point — no argument means
"now," an argument means "the calendar at that round." `ReCalculate()` refreshes
a `GameDate` in place after its `RoundNumber` is changed. `String(symbolOnly
...bool)` renders it for display. `Add(adjustHours, adjustDays, adjustYears)`
returns a shifted copy.

## Period strings

`AddPeriod(periodStr string) uint64` is the most-used function in the package:
it takes a human period and returns the **absolute round number** that period
lands on, measured from the receiver's round.

Accepted forms:

- `"10 days"` — quantity + unit. Units: `years`, `months`, `weeks`, `days`,
  `hours`, `minutes`, `seconds`, `rounds`. The unit is matched on its first
  three letters, so `second`, `seconds` and `secs` all work.
- Seconds are ALWAYS real seconds, with or without `real`/`irl`, and a `game`
  modifier does not change that: `"30 seconds"`, `"30 real seconds"` and
  `"30 game seconds"` are all 8 rounds at `RoundSeconds: 4`. They go through
  `Timing.SecondsToRounds`, which rounds UP to a whole round, so a seconds
  period never ends before the time it names and is at least one round
  (`"1 second"` is 1). `rooms.RealPeriodOK` rejects `game`/`gametime` on a
  seconds unit so spawn data cannot claim a clock it does not get. History:
  before 2026-09-30 there was no seconds branch and `"60 real seconds"` fell
  through to the rounds failover as 60 ROUNDS (four minutes); for a while
  after, bare seconds ran on the game clock, where `"30 seconds"` was one
  round, about four real seconds.
- Only seconds carry the never-early guarantee. Game minutes ROUND DOWN
  (`math.Floor` of `qty * RoundsPerDay / 1440`), so at 900 rounds a day
  `"48 minutes"` is exactly 30 rounds but `"1 minute"` is 0 rounds and ends at
  once. Real minutes, hours, days, weeks, months and years multiply a per-unit round
  count taken by integer division of the unit's seconds by `RoundSeconds`, so
  they are exact only when `RoundSeconds` divides 60 (at 4, `"2 real
  minutes"` is 30 rounds). Game hours, days, weeks, months and years step the
  calendar through `Add`.
- An unknown unit does not fail: it falls through to a failover that reads the
  quantity as rounds (`"7 flurbles"` is 7 rounds). `rooms.RealPeriodOK` keeps
  a copy of the unit vocabulary to catch typos in spawn data; add any new unit
  there too.
- bare period names such as `daily`, `weekly`, `noon`, `midnight`, `sunrise`,
  `sunset`.
- `"2 irl days"` / `"2 real days"` — real-world time, converted through
  `RoundSeconds`. `irl` and `real` are interchangeable.
- `""` — returns the receiver's round unchanged.

`GetLastPeriod(periodName string, roundNumber uint64) uint64` is the inverse:
the most recent round at which the named period boundary occurred.

`PeriodLength(periodStr string) uint64` returns a period as a **duration** in
rounds rather than an absolute round. Use it when you want a length — it exists
so callers stop faking one by calling `AddPeriod` from an arbitrary origin and
subtracting the origin back out. Game-time months and years vary with the
calendar, so the result is measured from the current date.

Anything with an authored duration in YAML — mutator decay and respawn, shop
restock, schedule segments — routes through these two functions, which is why
they accept sloppy input rather than erroring. A malformed quantity silently
becomes `1`.

## Day / night

```go
func IsNight() bool
func SetToDay(roundAdjustment ...int)
func SetToNight(roundAdjustment ...int)
func SetTime(setToHour int, setToMinutes ...int)
```

The three setters shift the world clock and back the admin `time` command.
`SetToDay` / `SetToNight` (admin `server set day` / `server set night`, and
the fresh-world boot) always move FORWARD, to the first sunrise / sunset
strictly after now, plus the optional round adjustment (`-1` is the round
before the rollover). They find it in clock time (round plus the `settime`
offset) and settle on the exact round by `GameDate.Night` itself. Before
#408 they built from `GetLastPeriod` plus one day, which always landed on
the same day's boundary and so rewound the clock after it had passed.
Day/night is a **derived** property (`GameDate.Night`), recomputed per query —
there is no transition callback and no cached state to invalidate.

**`GameDate.NightHoursPerDay` is VESTIGIAL as of the graded lighting arc.**
`getDate` still stamps it from `Timing.NightHours`, but nothing reads it: the
day/night boundary is computed in `ReCalculate` from `NightHoursAt` (below),
which varies across the year, so a single per-day figure can no longer
describe it. It is kept rather than deleted because `GameDate` is a
serialised, widely passed struct. Do not read it: it will report eight
hours on a night that runs fifteen.

## Solar and lunar light (`celestial.go`, plan 3a)

`configs.Lighting.WorldLatitude` is the **only** seasonal input. Declination,
day length, sunrise, sunset and noon height all derive from that one degree
value, so there is no separate seasonal table to keep in sync with it.
Shipped at 46.5, mirroring Washington State: night runs 8h23m at midsummer
and 15h37m at midwinter.

```go
func NightHoursAt(latitudeDegrees float64, dayOfYear int) float64
func NightAt(latitudeDegrees float64, dayOfYear int, hour float64) bool
func SunLight(cfg configs.Lighting, dayOfYear int, hour float64) float64
func MoonLight(cfg configs.Lighting, swiftmoon, wanderer, eye float64) float64
func CelestialLightAt(cfg configs.Lighting, dayOfYear int, hour, swiftmoon, wanderer, eye float64) float64
func CelestialLight() float64
func LampsLitAt(night bool, celestial, streetSky float64, cfg configs.Lighting) bool
func LampsLit() bool
func DayPeriod(night bool, lampsLit bool, hour24 int) string
func SetStreetLampSkyFraction(f float64) float64
func StreetLampSkyFraction() float64
```

- `NightHoursAt` is how `GameDate.ReCalculate` places the day/night boundary,
  replacing the old flat `Timing.NightHours` cutoff. It clamps to 12 (polar
  day) or 0 (polar night) rather than returning NaN beyond the polar circles.
- `NightAt` (lighting plan 6) is that boundary as a predicate: night is
  centred on midnight and `NightHoursAt` long. `GameDate.Night` (and so
  `IsNight`) is computed through it, so a caller holding a day and an hour
  (the light goldens, the street lamp's tests) asks the same question
  without moving the round counter.
- `LampsLitAt` (lighting plan 6, owner ruling O4 as amended) is the street
  lamps' rule, pure: lit while `night` OR the clear-sky `celestial`,
  attenuated by `streetSky` (the dimmest street-lamp biome's sky fraction)
  and rounded as a room level is, reads below `cfg.DimBelow` (the faces
  edge, `LightDimBelow`), a lamplighter working by eye, so a midwinter 08:00
  (day, sky about 40) keeps its lamps. 🔑 Testing the street's view of the
  sky rather than the sky itself is what stops a street dipping to 49
  (shapes) for the round its lamps go out, which the first rule did about
  240 times a year at its 0.95 fraction. Absent celestial is below any edge;
  NaN leaves it to `night`. `LampsLit` is it on the current round:
  `IsNight()`, `CelestialLight()`, `StreetLampSkyFraction()` and
  `configs.GetLightingConfig()`. `internal/rooms` registers the fraction
  through `SetStreetLampSkyFraction` whenever the biome registry loads or a
  test seeds it (rooms imports gametime, never the reverse); unregistered it
  is 1, the open sky. 🔑 Its input is the clear sky, before weather, so a
  storm lights no lamp and every street lamp in the world changes in the
  same round. Three readers share it:
  the biome street lamp (`rooms.BiomeInfo.StreetLamp` through
  `Room.LightLevel`), the behaviour condition `time_of_day period:
  lamplit` (the North Gate arch lantern's `dusk_to_dawn` tree), and the
  `time` command through `DayPeriod`.
- `DayPeriod` names the part of the day for the `time` command: "night"
  while it is night, "dusk" (from noon) or "dawn" (before noon) while the
  lamps are lit but it is not yet night, else "day". Before it, `time`
  said "daytime" at a 4PM winter dusk the street already read as dim
  (#382 playtest).
- `SunLight` is the sun's own contribution to the sky, calibrated so an
  equinox noon reads exactly `cfg.EquinoxNoon`. 🔑 **The sun is Absent below
  the horizon, not zero.** `sin(altitude) <= 0` returns `lightscale.Absent()`
  directly, so night needs no separate branch: since lighting plan 6
  `lightscale.Combine` reads Absent as a term of 0, which adds nothing.
  The night-trade guard still reads `SunLight` Absent as "the sun is down".
- `MoonLight` is the three moons' combined contribution (each moon's phase
  from `PhasesAtRound`/`GetAllPhases`, below), interpolated between a
  starlight anchor and a full-moon anchor on a logarithmic intensity axis.
- `CelestialLightAt` (lighting plan 6) is the pure combine of `SunLight`
  and `MoonLight` for a day, an hour and three moon fullnesses.
  `CelestialLight` calls it, and the light goldens and the world guard call
  it directly for moments no round gives (every moon new, or every moon
  full), so they cannot drift from the live sky.
- `CelestialLight` combines both into the sky's light for the whole world at
  the current round, memoized per round so 500 `Room.LightLevel()` calls in
  one round compute it once. It reads `PhasesAtRound(round)`, never
  `GetAllPhases()`, so the memoized sun and moon terms always come from the
  same captured round rather than two different ones under one cache key.

### Two traps for the next test author

- **A bare `Balance{}` has `WorldLatitude` zero, and validation COERCES that
  to 46.5** (`internal/configs/config.balance.lighting.go`) rather than
  honouring it. Right for production: until lighting plan 6 surfaced the
  lighting knobs in `_datafiles/config.yaml`, production genuinely ran a bare
  latitude, and any config file that omits the key still does. A test that
  wants a specific latitude must
  set `Balance.WorldLatitude` and call `Validate()` explicitly.
- **A test binary's `Timing` defaults are not the shipped ones.** Any test
  that touches night must pin BOTH `Timing` (`RoundsPerDay`, `NightHours`,
  `RoundSeconds`) and `Balance.WorldLatitude`, validate both, and install
  them with `configs.SetConfigForTest`, or it asserts on arithmetic derived
  from whatever Go zero-values happened to be in scope. `nightlength_test.go`'s
  `pinTiming` helper is the pattern to copy.

**`roundDateCache` carries no config fingerprint.** It is keyed on the round
number alone (`gametime.go`), so a test that changes lighting or timing
config and then asks about a round another test already cached silently
gets that other test's answer. `ClearDateCacheForTest()` exists for this;
call it both before sampling and via `t.Cleanup`. This is not hypothetical:
`TestShippedConfigHasASeasonalNight` once passed a deliberately broken
latitude coercion because an earlier test in the same file had already
cached those exact rounds under a different, still-pinned latitude, and the
break only showed up running the test in isolation.

**`celestial.go`'s single-slot memo (`celestialRound`/`celestialValue`) has
the same trap**, keyed on the round alone with no config fingerprint.
`ClearCelestialMemoForTest()` exists for it, same pattern: call it both
before sampling and via `t.Cleanup`, alongside `ClearDateCacheForTest()`.

## The three moons

Cycle lengths are multiples of `RoundsPerDay`, matching the lore in
`docs/world.md`:

| Moon           | Period mult | Influences                          |
|----------------|-------------|-------------------------------------|
| Swiftmoon      | 4.7         | Dexterity, Strength                 |
| The Wanderer   | 10.6        | Vitality, Willpower                 |
| The Eye        | 21.1        | Perception, Charisma; mutation rate |

```go
func GetSwiftmoonPhase() float64
func GetWandererPhase() float64
func GetEyePhase() float64
func GetAllPhases() (swiftmoon, wanderer, eye float64)
func PhasesAtRound(roundNum uint64) (swift, wander, eye float64)
func CurrentMoonFlavorBucket() int
func MoonStatDelta(phase, maxMod float64, base int) int
```

Phase is a smooth `[0.0, 1.0]` **contribution**, not a raw cycle position: the
linear phase percent is passed through `(1 - cos(2π·p)) / 2`, so 0.0 is new
moon, 1.0 is full, and both quarters read 0.5. `MoonStatDelta` turns that into
an integer stat adjustment scaled off a base value.

**Use `GetAllPhases` when you need more than one** — each single-moon getter
recomputes all three and discards two. **Use `PhasesAtRound` instead of
`GetAllPhases` when a caller already has a pinned round number** (plan 3a's
`CelestialLight` is the example): it takes that round rather than reading
the live counter itself, so two round-dependent values cannot drift apart.

## Other API

```go
type RoundTimer struct { RoundStart uint64; Period string }
func (r RoundTimer) Expired() bool

func MonthName(month int) string
func GetZodiac(year int) string
func CopyoverContributor() copyover.Contributor
```

`RoundTimer` is the small serialisable "started at round X, lasts for period P"
helper embedded in other packages' YAML.

## Gotchas

- **`RoundsPerDay == 0` short-circuits the moons to zero** rather than dividing
  by zero. A misconfigured timing block therefore reads as "all moons new," not
  as a crash — check config before hunting a phase bug.
- **`AddPeriod` returns an absolute round, not a duration.** Compare it against
  `util.GetRoundCount()`, never against a length.
- **Real-world conversion used `84600` seconds per day until 2026-07-31** — an
  upstream digit transposition that made every `irl` period run 0.9% short. It
  is now the named constant `secondsPerRealDay = 86400`.
- **`GetDate()` is memoized by round in `roundDateCache`, and that cache
  carries no config fingerprint.** Since the graded lighting arc,
  `GameDate.Night` derives from `Balance.WorldLatitude`, so a test that pins a
  different latitude and reuses a round another test already computed
  silently inherits that test's answer. See "Two traps for the next test
  author," above, and call `ClearDateCacheForTest()`.
- **There is no `GetTimeConfig`** — timing lives in
  `configs.GetTimingConfig()`.

## Dependencies

`configs` (timing block, and `configs.Lighting` for the celestial model),
`util` (round count), `copyover`, `lightscale` (the `Combine` operator
`CelestialLight` composes the sun and moons on). No dependency on rooms,
users, or mobs: this package sits low in the graph and is safe to import
almost anywhere.

## Consumers

`mutators` (decay/respawn), `shops` (restock), `mobs` (schedules), `conditions`,
`rooms`, `usercommands`, `internal/hooks`, and `modules/weather`.

## `GameDate.HoursAfterDusk` (lighting 5e)

`HoursAfterDusk()` is how many game hours have passed since dusk, in
[0, 24), read from the same unrounded night boundary `Night` uses (the
world's latitude and the day of the year; kept in two unexported fields so
the struct's serialised shape does not change). Before dusk it reads more
than the day's daylight, so it never reads "just after dusk" in the
afternoon. The behaviour-tree condition `time_of_day` reads it for
`period: after_dusk` with `hours: N` (the sunstone's faint hour).
