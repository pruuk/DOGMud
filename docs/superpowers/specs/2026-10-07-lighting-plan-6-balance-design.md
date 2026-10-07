# Lighting plan 6: balance pass (design)

Issue: #372 (graded room lighting arc, plan 6). Owner design session 2026-10-07.
Gate: the closing playtest (section 9), which absorbs the M5 PR 3 deferred
crime-in-the-dark playtest and #332's merchant night playtest.

Room review page (owner approved in chat 2026-10-07, revisions applied in
version 2): https://claude.ai/artifact/P9aLaHuK4PAShwaqo5W9FD

## Facts verified against source (master `48007844c`, 2026-10-07)

| Fact | Source |
|---|---|
| `lightscale.Absent()` returns `math.Inf(-1)`; `present(v)` excludes +/-Inf and NaN | `internal/lightscale/lightscale.go:27`, `:43` |
| `Combine(step, terms...)` = brightest + step*log2(sum 2^((t-best)/step)); never below the brightest term | `lightscale.go:55` |
| `Attenuate(step, light, fraction)` = light + step*log2(fraction); no floor; fraction <= 0 returns Absent, >= 1 returns light | `lightscale.go:97` |
| `Trim(step, others, max, target)` and `TrimDarkness(step, light, otherDark, max, floor)` solve the log combine for an adjustable source | `internal/lightscale/trim.go:30`, `:67` |
| The package doc already states "Zero is the darkest light that naturally occurs ... negative is magical darkness"; the math does not honour it | `lightscale.go:1-15` |
| Room light composition: sky (`Attenuate(celestial, skyLightFraction*skyFilter)`), lamp, fixtures, carried, all through `Combine`; `-Inf` result read as 0; darkness combined and subtracted; rounded, clamped -100..100 | `Room.composeWithFixtures`, `internal/rooms/lighting.go:137` |
| Other `lightscale` callers: `gametime/celestial.go:114,122,208` (SunLight Absent below horizon; CelestialLight combines sun and moons), `characters/vision.go:61` (infra reach combine), `rooms/light_trim.go:71,78,84`, `behaviortree/actions_item_light.go:127,188`, `conditions/light.go` (doc refs) | grep `lightscale\.` |
| Biome light fields: `BiomeInfo.SkyLight *float64` (`skylight`), `Lamp *int` (`lamp`), `MovementCost float64` (`movementcost`); rooms override with `Room.SkyLight` / `Room.Lamp` (`instance:"skip"`) | `internal/rooms/biomes.go:22,35,50`; `internal/rooms/rooms.go:102-103` |
| Biomes with a lamp: `city_thoroughfare` 52, `city_backstreet` 35, `interior` 50, `ether` 60, `spiderweb` 45 | `_datafiles/world/dogmud/biomes/*.yaml` |
| 98 rooms set `biome: city_thoroughfare` (97 town streets, squares, gates, bridges and quays in 14 zones, plus test_arena 200); 105 set `city_backstreet` explicitly (zones may also default to it) | grep of `rooms/*/*.yaml` |
| Arch lantern (item 55, room 4111) is a `fixture: light` item with tree `dusk_to_dawn`: `time_of_day period: night` then `set_light 52`, else `off`. `period: night` reads `gametime.IsNight()` | `items/other-0/55-arch_lantern.yaml`; `behaviors/items/dusk_to_dawn.yaml`; `internal/behaviortree/conditions_state.go` (`condTimeOfDay`); `internal/gametime/gametime.go:212` |
| Seeing through an exit: `messaging.SeesThroughExit` (predicates.go:96) needs `ParticipantSight != SightNone` and `ExitThroughWindow(light, strength, exitsAbove)` = `light >= exitsAbove - clampShift(strength)` (window.go:72). Infra plays no part. Callers: `actions/look.go:102` (`LookExitTooDark`), `actions/scan.go:110` | as cited |
| Own-room sight: `SightThroughWindow(light, strength, reach, blind, dim)`; infra gives shapes when `reach > 0 && light >= -reach` | `internal/messaging/window.go:34` |
| `LightTrimTarget = dazzle - clampShift(strength) - 1` (74 for normal eyes); trim runs in `Room.TrimLightFor` on `MoveToRoom` / `AddMob` for adjustable, unhooded records only | `messaging/window.go`; `rooms/light_trim.go:35` |
| Carried light strengths: candle 38 (cond 124), oil lantern 52 (125), hooded lantern 54 (127, adjustable), torch 56 (126), sunstone 46 (134). Glow spell (cond 1, adjustable) = 40 + stat/10 + skill/2, uncapped | condition YAML; `config.yaml:957-959` |
| Crime witnesses: `crimes.WitnessesInRoom` (crimes.go:229), `IdentifiedPerp` (:270): clear sight identifies, shapes only = unknown perpetrator, none = not a witness | `internal/crimes/crimes.go` |
| M5 PR 3 deferred playtest: Task 9 of `docs/superpowers/plans/completed/2026-09-22-messaging-m5-pr3-crime-in-the-dark.md` (five scenarios); written against the binary dark model (`GetVisibility`, `darkarea`, both now gone) | as cited |
| #408 (open): admin `server set day` rewinds the clock to that day's sunrise | `gh issue view 408` |

### Lighting knobs (declared `internal/configs/config.balance.go:1161-1329`, validated in `config.balance.lighting.go`)

In shipped `config.yaml` (read from `git show HEAD:`), all at their Go defaults:
`DarknessCombatPenalty` 0.80, `DazzleCap` 0.80, `LightDazzleAbove` 75, the six
`LightSpell*`, the three `LightNightVisionSpell*`, the three `LightInfraSpell*`,
`LightInfraReachCap` 50, `LightInfraPenaltyFloor` 0.90, the six `LightDarknessSpell*`.

ABSENT from `config.yaml` (Go default applies): `LightBlindBelow` 25,
`LightDimBelow` 50, `LightExitsAbove` 65, `LightDefaultVisionStrength` 12,
`LightDoublingStep` 8, `WorldLatitude` 46.5, `LightEquinoxNoon` 70,
`LightStarlight` 10, `LightMoonsFull` 35, `LightMoonWeightSwiftmoon` 4.0,
`LightMoonWeightWanderer` 1.0, `LightMoonWeightEye` 0.5. Constant, not a knob:
`LightWindowShiftCap` 24.

## Owner rulings this session (do not relitigate)

- **O1 Light scale meaning.** 0 is the darkest a room can be without active
  magical darkness. Negative values mean magical darkness and nothing else.
- **O2 The formula is wrong, not just unfloored.** It must not be able to reach a
  negative number without a darkness source, by construction. Rebuild it.
- **O3 Minimum reading for real light is 3**, kept as a rounding guard on top of
  the rebuild.
- **O4 City lamps by hour: option A.** Street lamps burn from dusk to dawn as a
  biome property, not 98 placed lamp-post items.
- **O5 Seeing through an exit needs 55** for normal eyes (was 65). Night vision
  lowers it as it already does.
- **O6 Infravision shows shapes through exits**, never names.
- **O7 Rooms that promise light:** the per-room table on the review page,
  including the owner's raised daylight fractions, entrance rooms 3101 and 5255
  partly lit, 6404 given a little light, and rewording 6405.
- **O8 `dense_forest` movement cost 1.4.**
- **O9 The cave trim stays as is** (only strong glow casters reach it); correct
  the notes that say it never fires.

## 1. Rebuild the light arithmetic (`internal/lightscale`)

Light is carried in points as today, but every operation works on linear
brightness, where nothing is exactly zero:

- `B(p) = 2^(p/step) - 1` for a light term, so `B(0) = 0`.
- `p(B) = step * log2(1 + B)`, its inverse; `p(0) = 0`.
- **Combine** lights: `p(sum of B(t_i))`.
- **Attenuate** (a window, grate, canopy or weather): `p(fraction * B(light))`.
- **Absent**: a term with `B = 0`. "No sky reaches here" and "no light" are now
  the same statement, so the `-Inf` sentinel and the `v = 0` special case in
  `composeWithFixtures` go away. The old Absent doc's reason for the
  sentinel (two zeros combining brighter than one) no longer holds: `B(0)+B(0)`
  is still 0.
- **Darkness** terms combine the same way among themselves and are subtracted
  in points after the light is composed, exactly as 5d does today. That
  subtraction is the only way a room goes below 0.
- **Floor (O3):** after the light terms are combined and before darkness is
  subtracted, a room with any real light (`B > 0`) reads at least
  `LightRealMinimum` (new knob, 3). A room with none reads 0.

Properties the tests pin:

- A single term reads its own value: `p(B(x)) = x` for every x >= 0.
- Combine never reads below its brightest term, and two equal terms read one
  step brighter (52 + 52 = 59.9, rounds 60).
- Attenuate never reads below 0 for any fraction in (0, 1].
- Above 25, every reading differs from today's by under 0.5 points
  (the difference shrinks as 2^(-p/step)).
- With no darkness source, no room at any hour, season or weather reads below 0.

Expected changes, only at the dim end: cell 5105 at night with no moons goes
from -16.6 to 2.4 (then floored to 3); starlight alone stays 10; a window's
light at noon is unchanged to within half a point.

**Trim and TrimDarkness** are re-derived for the linear sum (solve
`B(target) = B(others) + B(x)` for x). Their callers (`rooms/light_trim.go`)
and their contracts do not change.

**Other `Combine` users.** Infra reach (`characters/vision.go:61`) and
celestial light (`gametime/celestial.go:208`) use the same operator, so they
change with it: single sources read the same, small sums read a little lower
(two reaches of 5 go from 13 to 8.5). The plan records the before and after for
each shipped reach source and confirms with the owner if any shipped value moves
by more than one point.

## 2. Street lamps from dusk to dawn (O4)

- Biomes gain a night-only lamp flag (YAML name settled in the plan, e.g.
  `lampatnight: true`). When set, the biome's `lamp` joins the composition only
  while `gametime.IsNight()` is true, the same test the arch lantern's
  `dusk_to_dawn` tree uses, so every lamp in the world lights and goes out
  together.
- `city_thoroughfare` (52) and `city_backstreet` (35) set it. `interior` (50),
  `ether` (60) and `spiderweb` (45) do not: indoor lamps and magical glows burn
  at all hours.
- A room `lamp:` override stays an all-hours lamp (an inn's lamps, a cell's
  torch), whatever its biome.
- Effect: by day the streets read their daylight alone (equinox noon about 72,
  solstice noon about 73.6, so no street dazzles); at night they read as today
  (52 to 54 main streets, 36 to 43 backstreets).
- Room text that already says lamps are lit in the evening (5803 and kin) now
  matches. No other room text changes for this section.

## 3. Seeing through exits (O5, O6)

- `LightExitsAbove` set to **55** in `config.yaml`. Validation is unchanged (it
  must stay at or above `LightBlindBelow`).
- Result for normal eyes: bare lamplit street at night (52 to 54) no; with a
  candle 55 yes, just; with a lantern 60 or torch 62 yes; a torch alone in a
  cave 56 yes.
- **Infravision through an exit.** When the light test fails but the viewer has
  infra reach, they see the next room's occupants as shapes ("a figure", the
  same anonymous form the room roster uses) for warm bodies whose room light is
  at or above `-reach`, never names, never the room's description or items.
  This covers `scan` and `look <exit>`. It never upgrades a view the light test
  already grants. The exact rendering of `look <exit>` with shapes only follows
  the existing shapes vocabulary and is fixed in the plan.

## 4. Rooms that promise light (O7)

The review page's 45 rooms, as approved:

- **Room `lamp:` (all hours), 14 rooms:** 3109 35, 310 35, 503 45, 488 30,
  317 50, 314 40, 6032 45, 204 60, 497 60, 498 64, 493 30, 6200 30, 301 28,
  496 28.
- **Room `skylight:` (daylight through an opening), 9 rooms:** 3101 0.50,
  5255 0.50, 6403 0.35, 6411 0.25, 490 0.25, 4127 0.25, 3102 0.20, 6407 0.15,
  6404 0.10. Equinox noon readings about 64, 64, 60, 56, 56, 56, 53, 50, 45.
- **Text edits:** 5255 rewords "the daylight dies behind you ... the way a door
  shuts" to fit a partly lit entrance, and fixes "The world you came from lies
  back east" (the way out is `up`, to 5254 Mine Mouth). 6405 rewords "at the
  far edge of your lamplight" so it does not assume a carried light. All copy
  follows `dogmud-player-copy` (80-column wrap, no numbers).
- **No change, 22 rooms:** the rest, including 507 (lit while Whisper, mob 273,
  wears its Oil Lantern).

## 5. Dense forest (O8)

`dense_forest.yaml` `movementcost: 1.4` (forest stays 1.0; snow is 1.4). Data
only; `actions.MovePrice` reads it through `BiomeInfo.GetMovementCost`.

## 6. Cave trim (O9)

No code change. Correct the notes (handoff memory, #372 body checklist) that
say cave trims never fire: a glow spell cast at 75 or more trims to 74 on entry.

## 7. Config surfacing

- Add every knob in the ABSENT list above to `config.yaml` at its current
  value, except `LightExitsAbove` at 55, each with a one-line comment in the
  file's existing style.
- Add `LightRealMinimum: 3` (declared, validated: 0 to `LightBlindBelow - 1`,
  default 3).
- Delete the stale comment in `config.balance.lighting.go` claiming no lighting
  knobs appear in `config.yaml`.
- `config.yaml` carries skip-worktree: build the commit from the
  `git show HEAD:` blob (`dogmud-balance-config`).

## 8. Housekeeping

- Update `internal/lightscale/context.md` (new arithmetic, no `-Inf`
  sentinel), `internal/rooms/context.md` (night-only lamp, no `v = 0` branch),
  `internal/messaging/context.md` (infra through exits).
- PATCH_NOTES entry in player terms.
- Refresh the stale checklist in the #372 body.

## 9. Closing playtest (the gate)

Run with `playtest-scenario` (ephemeral goals, `--checkout`). Because of #408,
reach a target hour by advancing rounds, not `server set day`.

1. **Crime in the dark (rewritten M5 PR 3 Task 9)** under graded light, the five
   scenarios kept: clear sight identifies; no sight records an unknown
   perpetrator with no rep change; shapes only (now via a real source, e.g.
   Heat Sight or the Pitsense Tincture, not a hand-applied condition 85) stays
   unknown but seeds revenge; night vision identifies; a sleeping witness is no
   witness. Plus: an unlit backstreet at night is a low-risk scene, a lamplit
   main street is not.
2. **#332 merchant night checks:** with no carried light at night, trade with
   Siv and the other lantern keepers, keepers show by name, pinned keepers stay
   put.
3. **Street lamps:** a main street reads daylight only at noon and lamplit after
   dusk; lamps change at the same moment as the 4111 arch lantern.
4. **Through exits:** bare lamplit street at night cannot `look` through an
   exit; with a lantern it can; an infravision character sees "a figure" in the
   next room, never a name.
5. **Relit rooms:** visit 3109, 497, 6403 by day, and 5255; each reads as its
   text says.
6. **No natural negatives:** holding cell 5105 at night with no moons reads as
   barely lit, not darker than a sealed cave.

Findings go to GitHub issues (`--repo pruuk/DOGMud`).

## Testing

- **Before the rebuild:** record a light golden over a fixed spread of rooms
  (every biome, every room with a `lamp` or `skylight` override, the 45 review
  rooms), hours (midnight, dawn, noon, dusk), seasons (both solstices, an
  equinox) and moon states. After the rebuild every diff against it must be a
  predicted one: rises at the dim end, nothing above 25 moving 0.5 or more.
- Unit tests for section 1's properties, the floor, Trim and TrimDarkness
  round-trips, and the night-only lamp at the IsNight boundary.
- A world test: with no darkness source present, no shipped room reads below 0
  at any sampled hour, season, moon state or weather.
- The #207 night-trade guard test stays green (night streets are unchanged).

## Out of scope

Open lighting bugs and copy nits filed separately (#214 to #224, #248, #251,
#260, #261, #315, #383, #408, #409) stay as filed. Biome lamp values themselves
(52, 35) are not retuned.
