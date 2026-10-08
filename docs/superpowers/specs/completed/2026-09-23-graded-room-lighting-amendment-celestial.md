# Graded room lighting: celestial amendment

Amends `2026-09-22-graded-room-lighting-design.md`. Written 2026-09-23 against
master `350cb6e88`, after plans 1 and 2 shipped (PR #159, #160).

This document **supersedes named sections** of the original spec. Where the two
disagree, this one wins. Everything not named here stands unchanged.

Owner decisions taken during this design session are marked 🅾️.

---

## Facts verified against source

Every row read from the tree on 2026-09-23. Nothing here is recalled.

| # | Fact | Evidence |
|---|------|----------|
| 1 | Shipped timing: `RoundsPerDay: 900`, `NightHours: 8`, `RoundSeconds: 4`. Night is 20:00 to 04:00, and **one game day is one real hour**, so a 365 day year passes in about 15 real days | `_datafiles/config.yaml:207,210` |
| 2 | `NightHours` is read in exactly **three** places outside config validation, all in one file | `internal/gametime/gametime.go:183`, `:202-204`, `:511` |
| 3 | `IsNight()` / `GameDate.Night` has **14 non-test consumers**, every one reading a single world-wide boolean | repo grep |
| 4 | **No shipped data uses a `sunrise` or `sunset` decayrate.** Every such line in the loaded world is commented out; the two live ones are in `_datafiles/world/default`, which is not the loaded world | `config.yaml:232` (`DataFiles: _datafiles/world/dogmud`), grep of both trees |
| 5 | Shipped biome histogram over 1386 rooms: city 477, land 224, cave 120, **default 117**, farmland 100, cliffs 97, water 91, forest 61, mountains 27, fort 22, swamp 19, house 15, shore 14, dungeon 1, desert 1 | `testdata/lighting_parity.golden` |
| 6 | 🔴 **`litarea` biomes cover 855 of 1386 rooms, 62%** (city, land, fort, house, plus the synthetic `default`). Under the current model none of them can ever darken | fact 5 + biome YAML |
| 7 | 🔴 **`land` is the generic outdoor fallback and is flagged `litarea: true`.** Its rooms are open air: *Valley Road*, *The Market Square* ("sun-flooded", "washing snapping on lines") | `biomes/land.yaml`, `rooms/amber_valley/6055`, `6056` |
| 8 | 🔴 **`fort` mixes open sky and buried interior in one biome.** *Training Yard*, *The High Rim*, *Sparring Ring* are outdoor; *Buried Vault* and *Tower Base* ("a round shaft of grey stone climbing up into gloom... winds up into the dark") are not. It is `litarea: true` | fort room titles, `rooms/pothole_coulee/5240` |
| 9 | 🔴 **81 of the 117 biome-less rooms are the zone `a_dark_forest`**, permanently bright because a missing biome key falls through to a synthetic Go biome with `LitArea: true`. The rest are 21 `endless_trashheap` and 10 instance/shadow rooms | orphan scan, `internal/rooms/biomes.go` `LoadBiomeDataFiles` |
| 10 | `BiomeInfo.Indoor` exists and **lighting reads it nowhere**. Its only consumers are the `outdooronly` mutator filter and the weather module | `rooms.go:2931`, `modules/weather/engine/worldreader.go:13` |
| 11 | 🔑 **`Biome` is a per-ROOM yaml field**, so the indoor axis is already room-granular. Mis-classified rooms are a data problem, not a missing mechanism | `internal/rooms/rooms.go:90` |
| 12 | 🔴 **`new_plymouth_sewers` is 20 rooms of `biome: city`**, which carries no `indoor` flag, so outdoor-only weather mutators render underground. Room 6403 is "a brick barrel-vault... grey light falls in a single column through the pried drain-cap above" | room 6403, `biomes/city.yaml`, `mutators.go:74` |
| 13 | `new_plymouth_temple` is 25 `city` rooms split roughly evenly between genuine outdoor (*Temple Gate Plaza*, *Censer Court*, *Garden of Repose*) and genuine interior (*Grand Temple Sanctuary*, *Archive Stacks*, *Seminary Dormitory*) | room titles |
| 14 | `LightLevel()` has **18 non-test call sites**, several inside per-round loops | `NewRound_DoCombat_helpers.go:422`, `MobIdle_HandleIdleMobs.go:115`, `NewRound_UserRoundTick.go:211`, + 15 others |
| 15 | 🔴 **No `Room.IsLit()` exists.** Fifteen sites hand-roll `room.LightLevel() >= int(configs.GetBalanceConfig().LightBlindBelow)`, each copying a 424-field struct to read one int. Plan 1's design promised a named predicate; it was never built | repo grep |
| 16 | Measured (AMD 5900X, `-benchtime=200000x`, bench file since deleted): `GetBalanceConfig` **99.75 ns**, `GetTimingConfig` 8.23 ns, `GetDate` 11.79 ns, `GetAllPhases` **246.2 ns**, full uncached celestial **332.0 ns**, per-room skylight+combine **103.4 ns** | `go test ./internal/rooms/ -bench` |
| 17 | 🔑 **None of plan 1's or plan 2's lighting knobs appear in `config.yaml`.** They run entirely on Go defaults, so their config shape can still be changed for free | grep of `_datafiles/config.yaml` |
| 18 | Moon lore: Swiftmoon "Large (2x Luna)", The Wanderer "Similar to Luna", The Eye "Small, bright". Periods 4.7 / 10.6 / 21.1 days. "True darkness is rare, as at least one moon is usually visible" | `docs/world.md:52-56`, `:70` |
| 19 | 🔴 **Five zones declare a `defaultbiome` that has no biome file**: `plains` (`dustwalk_road`, `marches_spur_road`, `stillwater`), `river` (`watchers_crossing`), `default` (`newcomer_antechamber`). Currently inert, because every room in those zones carries its own biome key, but any new room without one falls through to the lit synthetic default | zone-config scan against `biomes/` listing |
| 20 | `house` is coupled to weather in two places beyond its biome file | `modules/weather/content/emotes.go:244` (`surfaceIndoorBiomes`), `_datafiles/world/dogmud/weather/climate/house.yaml` |
| 21 | Weather climate archetypes bind by biome NAME in Go, with `fort` explicitly left unbound because it is indoor | `modules/weather/sim/climate.go:158-179` |
| 22 | Forage tables key on biome name but reference neither `house` nor `fort`, so the biome changes below do not touch foraging | `internal/forager/forage_core.go:18-46` |
| 23 | `undergroundBiomes` is `{cave, dungeon}`; `surfaceIndoorBiomes` is `{house, fort, spiderweb}`. Classification is required to be TOTAL and `biome_coupling_test.go` fails the build otherwise | `modules/weather/content/emotes.go:227-247` |

---

## What this amends

| Original spec said | This says | Why |
|---|---|---|
| "Zero is the absence of light" | **Zero is the darkest light that naturally occurs.** Absence is negative infinity | On a log scale two sources at 0 legitimately read brighter than one. "Absence" makes the arithmetic incoherent |
| Sources stack by halving; `light = ambient + lights − darks` | **One log-combine operator over all terms**, ambient included | The halving rule gives a lantern-lit street at noon `70 + 55 = 125`. Every lit city room would be past the dazzle edge all day |
| Seasonal noon peak 78 / 70 / 62, delivered by season mutators | **Derived from `WorldLatitude`.** The peak table and the mutator delivery are deleted before being built | One astronomical input produces day length and noon height together. Two independent knobs could contradict each other |
| Weather multiplies the celestial term | **Unchanged in intent, but a multiplier is a subtraction on a log scale.** Biome shade, roofs and weather are one operator | Collapses plan 4's occlusion into a term plan 3 already built |
| Biomes carry `darkarea` / `litarea` booleans | **Deleted.** Replaced by a per-biome `skylight` fraction and a per-room `lamp` value | Fact 6: the booleans hold 62% of the world permanently lit, and cannot express "dim gold" versus "street lantern" |
| `NightHours` sets the day/night boundary | **No longer read for the day/night boundary at all.** `WorldLatitude` always drives it | 🔴 See the correction below. The fallback this row originally described was incoherent |
| (plan 5) A light source trims to avoid dazzling its bearer | 🅾️ **Darkness sources trim symmetrically**, to the bottom of the bearer's usable band | Owner ruling. See "Plan 5 amendment" |

---

## The model

### The scale

Light runs −100 to 100 and is **perceptual, not linear**. Zero is the darkest
naturally occurring light, roughly an unlit cave. Negative is magical darkness:
light actively removed. The three sight bands from plan 1 are unchanged, and the
window model from plan 2 is untouched.

### One combination operator

```
combined = brightest + step · log2( Σ 2^((sᵢ − brightest) / step) )
```

`step` is `LightDoublingStep`, 🅾️ **8**. It answers one question: how many points
is twice as much light. Two equal lamps read 8 points brighter than one, four
read 16, and a much weaker source adds nothing.

🔑 **The same constant governs three things**, which is why it is one knob:
combining sources, applying a sky fraction (`+ step · log2(sky)`), and the shape
of the daylight curve. A sky fraction of 0 removes the term entirely rather than
contributing zero.

The original spec's stated intuition survives verbatim: "stacking converges near
twice the brightest" becomes "each doubling of the lamps adds `step` points",
which is the same claim without the blow-up. Its own worked example, five glows
of 30, reads 49 here against the 56 it predicted, because that prediction
assumed halving rather than a doubling step.

### Celestial ambient, derived from latitude

🅾️ `WorldLatitude` is **46.5**, mirroring Washington State.

```
δ(N)   = −23.44° · cos( 2π(N + 10) / 365 )
H      = arccos( clamp( −tan φ · tan δ, −1, 1 ) )        // half-day angle
alt(h) = asin( sin φ sin δ + cos φ cos δ cos(2π(h − 12)/24) )
sun(h) = sunFull + step · log2( sin alt(h) )
```

🔑 **The sun has no "off" case.** Below the horizon `sin alt` is non-positive and
the term is simply absent, the same way a cave with `skylight: 0` is absent. This
removes a special case rather than adding one.

🔑 **Two knobs disappear.** The flatness exponent is gone, because `sin` is
already flat near its peak. The seasonal peak table is gone, because noon height
falls out of declination.

🔴 **CORRECTION, 2026-09-23, found while implementing.** This spec originally
demoted `NightHours` to "the fallback used only when `WorldLatitude` is zero".
**That could not work, and would have made this entire model unreachable.**

Go cannot distinguish an unset float from an authored zero, and fact 17 says
none of the lighting knobs appear in `config.yaml`, so **the shipped
configuration is a bare `Balance`, whose `WorldLatitude` is zero.** Honouring
zero would therefore have shipped DOGMud at no latitude: a flat eight-hour
night, no seasons, and every line of the celestial model built, wired and never
once reached. It was verified by probe rather than argued: a bare `Balance`
validates to `WorldLatitude = 0`.

**Zero therefore means UNSET and coerces to 46.5**, the `LightDefaultVisionStrength`
idiom, and the `NightHours` fallback is **deleted rather than repaired**. There
is no path from day length back to `Timing.NightHours`; that knob stays for
upstream compatibility but nothing reads it for the day/night boundary. An
operator wanting an equator-like world of twelve-hour nights all year authors a
latitude near zero, such as 0.001.

🪤 **The general lesson, which applies to every knob this arc adds:** a "zero
means disabled" convention is only safe for a knob that ships a value in
`config.yaml`. For a knob that runs on Go defaults, zero *is* the shipped value,
so "disabled" and "unset" are the same state and the disabled branch is what
ships.

At 46.5°, calibrating equinox noon to 🅾️ **70**:

| | Night | Noon |
|---|---|---|
| Midsummer | 8h23m | 73 |
| Equinox | 12h00m | 70 |
| Midwinter | 15h37m | 62 |

⚠️ **Natural daylight never dazzles.** Midsummer noon at 73 sits under the 75
edge. The original spec wanted midsummer midday to make a normal person squint;
real geometry at this latitude and calibration will not do that, and equinox
would have to be about 72 for it to. Dazzle therefore remains entirely
exceptional, which matches plan 2 having shipped it with no mechanical effect.

> **Correction (lighting plan 3d, 2026-09-25).** The sky ALONE never
> reaches 75, so this callout is still true as written. But a normal
> observer's TOTAL light can: the sky combines with a carried light or a
> `city_thoroughfare` lamp, and near midsummer noon both combines cross 75
> (about 75.1 for a carried light outdoors, 75 for the thoroughfare) with
> no vision ability involved. The owner ruled this intended: a hooded
> lantern that adjusts, or the right spell, is what a player should reach
> for instead. Plan 5 must price it. See the "What this calibration does to
> dazzle" correction below.

⚠️ **The annual mean night is 12 hours at any latitude**, against the shipped flat
8. Night rises from 33% of the day to 50%, and 65% at midwinter. This is a
consequence of the latitude, not a dial.

**Deliberately omitted:** atmospheric extinction (a low winter sun crosses far
more air), and the lore's "10-15% difference in seasonal lengths" from orbital
eccentricity (`world.md:44`). Both are refinements on a model that is already
derived; neither changes a band. Extinction was prototyped and set to zero.

### The three moons

Phase functions are **unchanged** — `GetSwiftmoonPhase` and friends already
return a smooth 0 to 1. What is new is an intensity weight per moon:

| Moon | Lore | Weight | Derived? |
|---|---|---|---|
| Swiftmoon | 2x Luna's apparent size | **4.0** | Yes. Brightness goes as angular *area*, so twice the size is four times the area |
| The Wanderer | similar to Luna | **1.0** | Yes. The baseline |
| The Eye | "Small, bright" | **0.5** | ⚠️ **No.** A reading of prose: about 0.25 area at roughly twice the albedo. Its config comment must say so |

🔑 **Albedo is inside the weight, not separate from it.** From the ground a large
dull moon and a small bright one are indistinguishable; only the product is
observable. Splitting them would be two knobs with one effect.

Weighted intensity plus a starlight floor is mapped onto the scale by a two-point
calibration: 🅾️ all-new reads **10**, all-full reads **35**.

### Composition

```
celestial = combine( sun(h), moons )
ambient   = celestial + step · log2( skylight )      // absent when skylight = 0
light     = combine( ambient, roomLamp, …carried sources )   // − darkness, plan 5
```

🔑 **The room's own lamp joins the same stack rather than acting as a floor.**
Otherwise a lantern-lit tavern plus a carried torch double-counts.

---

## Measured outcomes at this calibration

Sampled every ~3 game minutes across a full 365-day year at φ = 46.5°, with
`step 8`, equinox noon 70, moons 10 to 35.

```
moonlight: min 10.0   median 27.8   max 34.9     (below the blind edge on 38% of nights)
all three moons under 10% full at once:  0.59% of the time

room                          night hours blind/shapes/full     whole year
Market Square    land            37% /  63% /   0%             19% / 37% / 44%
Greenford Gate   city main        0% /   0% / 100%              0% /  0% /100%
Riverside Row    city lane        0% / 100% /   0%              0% / 54% / 46%
Reedwash Descent swamp          100% /   0% /   0%             51% / 20% / 29%
A Dark Forest    (none)          97% /   3% /   0%             49% / 16% / 35%
Tower Base       fort           100% /   0% /   0%             66% / 34% /  0%

nightvision 24, outdoors at night:   blind 0%   shapes 42%   full sight 58%
```

Four properties worth holding onto, because they are what this calibration *is*:

🔑 **Moonlight tops out below the full-sight edge.** A normal observer without a
lamp **never reads full sight outdoors at night**, any night of the year. Names
are hidden across the whole wilderness after dark. The moons' entire mechanical
job is deciding blind versus shapes.

🔑 **The trees make the dark, not the sky.** Open ground is blind on 37% of night
hours; under canopy at `skylight: 0.45` it is 97%. This satisfies fact 18's lore
line honestly, and it makes the sky fraction the thing that creates darkness.

🔑 **A lamp at 55 is mechanically static.** *Greenford Gate* never leaves full
sight in either direction. It still moves 55 to 72 numerically, so transition
notices fire, but no observer's sight tier ever changes there. That is the
intended reading of "a main street is safe".

🔑 **Nightvision 24 is worth drinking**: full sight on 58% of outdoor night hours
and never blind. Given it is one craftable potion (spec fact 26), that is right.

---

## Architecture

| Layer | Owns | Lives in |
|---|---|---|
| Celestial ambient | latitude, declination, solar altitude, three moons | `internal/gametime` |
| Sky fraction and lamp | per-biome default, per-room override | `internal/rooms` + biome and room YAML |
| Room light | the combine over ambient, lamp, mutators, light-bearers | `internal/rooms` |
| Vision window | unchanged from plan 2 | `internal/messaging` |

🔑 **Seasons no longer cross the module boundary.** The original spec routed the
seasonal peak through weather-module mutators because `internal` cannot import
`modules`. Day length is *astronomical* and global; the weather module's seasons
are *climate* and per-zone. A monsoon zone and a temperate zone at the same
latitude share a day length and differ in rain. So the coupling simply does not
arise, and per-zone occlusion in plan 4 is unaffected.

### Performance

🔴 **Caching is not required, and the original instinct to build it was wrong.**
Measured (fact 16), the full uncached celestial computation is 332 ns. At 500
`LightLevel()` calls in a round that is 218 µs against a 4000 ms round: 0.005%.
Scaling by a conservative 5× for the 1 CPU production droplet still leaves 0.03%.

What transfers between machines is the **ratio**, not the absolute: the ratios in
fact 16 hold on any CPU. Anything absolute should be measured against
`reference_prod_perf_baseline`, not a desktop.

**Therefore:** a single-entry memo keyed on round number, because it is three
lines and makes the cost provably nil. **Explicitly NOT a per-room cache and NOT
a warm-all-rooms pass** — the celestial term is one global value for the whole
world, and per-room light is computed on demand only for rooms something asks
about. Cost tracks activity, not the 1386 rooms.

🔴 **The real cost is `GetBalanceConfig` (fact 15, 16), and plan 3 fixes it:**

1. `LightLevel()` reads no config; thresholds are passed in, the precedent
   `SightThroughWindow` set in plan 2 for the same reason.
2. Add `configs.GetLightThresholds()`, a small struct read under the existing
   lock without copying `Balance`. This is the `GetTimingConfig` idiom, 8 ns
   against 99. **Free today** because fact 17 says no operator config holds
   these knobs yet.
3. Build the `Room.IsLit()` plan 1 promised and collapse the 15 hand-rolled
   pairs onto it.

Net: plan 3 adds real celestial math and leaves the call sites cheaper than they
are now.

---

## Content

🅾️ Owner chose the wide scope: biomes, the orphans, and the city street pass.

**Biome vocabulary.**

- `skylight` (0.0 to 1.0) and `lamp` (scale value) replace `darkarea` / `litarea`
  on every biome. The booleans are **deleted**, so the compiler and the loader
  enumerate every consumer — this project's established refactoring idiom.
- Per-room `skylight` and `lamp` overrides, because fact 8 puts *Training Yard*
  and *Buried Vault* in the same biome.
- 🅾️ **New biome `sewer`.** Fact 12 is a live defect independent of this arc:
  weather currently renders inside the New Plymouth sewers. `sewer` must join
  `undergroundBiomes` (fact 23), which `biome_coupling_test.go` enforces.
- 🅾️ **New biome `interior`, absorbing `house`.** `house` is deleted; its 15
  rooms and the mis-biomed city interiors of fact 13 become `interior`. Touches
  fact 20's two weather couplings and fact 21's climate bindings. Fact 22 says
  foraging is unaffected.
- 🅾️ **New biomes `plains` and `river`**, which fact 19 shows five zones already
  ask for by name.
- `land` and `fort` lose their lit status (facts 7, 8).

**Room sweep.**

- The 117 orphans get real biomes: 81 `a_dark_forest`, 21 `endless_trashheap`,
  10 instance and shadow rooms.
- The city pass sorts 477 rooms into three buckets, not two: main street
  (lamp ~55), side street (lamp ~35), and **actually indoor and mis-biomed**.
  `new_plymouth_temple` (25) and `new_plymouth_crafting` (27) are the next
  candidates after the sewers.

---

## Transition notices

🅾️ Owner requirement. Player-facing text when light changes, on **two** triggers:

1. **Moving between rooms whose light differs.**
2. **The room's own light crossing a band around a standing player** — dawn
   breaking, weather lifting, a lamp going out.

`internal/hooks/Awareness_LightChange.go` already subscribes to `RoomChange` and
`EquipmentChange`, but both handlers are stubs whose bodies are FUTURE notes
about stealth re-rolls. The event plumbing exists; the notice does not.

Text goes through `internal/narration`, per the messaging arc. A notice fires on
a **band** change for the observer, not on every numeric change, or a player
standing outdoors at dawn would be spammed every round.

---

## Plan 5 amendment: the darkness throttle

🅾️ Owner ruling. The original spec gave the self-throttle to light sources only,
targeting the top of the bearer's comfortable band. **Darkness sources trim
symmetrically**, to the bottom of the bearer's *usable* band:

| Caster | Darkness trims the room to | Who can still see |
|---|---|---|
| Normal | `LightBlindBelow` | nobody, the caster included |
| Nightvision 24 | `windowFloor` (1) | the caster, in shapes |
| Infravision reach 30 | −30 | the caster only; even nightvision 24 is blind, its floor being 1 |

🔑 **This makes infravision the real darkness weapon** rather than a slightly
better nightvision, and it gives plan 2's otherwise-decorative reach a job.

🔑 **Subtraction is the correct operator for darkness on a log scale**, and now
there is a reason rather than an assertion: subtracting points divides the light
by a fixed ratio. A darkness of 40 at step 8 is "one thirty-second of whatever
was here", taking noon from 70 to 30 and a lit street from 55 to 15. **Darkness
is a nuisance by day and a weapon by night**, for exactly the reason a blizzard is.

⚠️ **Honest limit.** Entering a room lets you re-trim **your own** source, never
someone else's. The play is bringing a counter-source, and the original spec's
entry-order rule means the last actor through the door sets the level. Plan 5's
help text must not promise more than that.

All other throttle rules from the original spec are unchanged: adjustment happens
on events, never on a round tick; a source is created at full strength and does
not trim until its bearer moves; sources already in the room do not re-trim when
someone else arrives.

### What this calibration does to dazzle, which plan 5 inherits

🔑 **Dazzle stops being a weather condition and becomes purely a play mechanic.**
Natural daylight tops out at 73 and can never reach 75 at any latitude-derived
peak, so a normal observer is dazzled by exactly two things: a shifted window, or
an artificial source strong enough to push a room past 75. That is precisely the
"light becomes a weapon" case, and it means plan 5 is not merely the plan that
*may* give dazzle teeth, it is the only plan in which dazzle is reachable at all.

> **Correction (lighting plan 3d, 2026-09-25).** "Only plan in which dazzle
> is reachable" is wrong: the sky's 73 combines with an ordinary carried
> light or a `city_thoroughfare`'s lamp, and near midsummer noon both
> combines land at or just above 75 (about 75.1 for a carried light
> outdoors, 75 for the thoroughfare), reachable by any player today with no
> vision ability at all. The owner ruled it intended: "they should have
> used the spell or a hooded lantern that adjusts; it makes the lantern
> actually valuable." Plan 3d's `internal/lightnotice` therefore ships
> dazzle lines that read true for any observer, not only a shifted-window
> one. This does not change the point below about the Cat's Eye Draught
> overhang, which is a second, independent way to be dazzled; it only
> retracts the claim that it is the ONLY way.

⚠️ **The Cat's Eye Draught overhang is structural, and plan 5 must price it.**
The draught runs `triggercount: 500` at `triggerrate: 1 round`
(`conditions/65-cats_eye_draught.yaml`), which against `RoundsPerDay: 900` is
**13h20m of game time**. Equinox night is 12h and midsummer night 8h23m, so for
most of the year a drinker who takes it at dusk is still under it after sunrise.
Their shifted dazzle edge is `75 − 24 = 51` against a daylight range of 62 to 73,
so they spend the tail of every dose penalised in daylight.

The original spec calls this intended: "Nightvision hurts in daylight. That is
the point of a window that moves rather than widens." This amendment does not
dispute that, it **quantifies** it. A 13h20m potion against a 12h night is not an
occasional mistake the player makes, it is the default outcome of drinking at the
start of the night. Plan 5 should decide deliberately between three answers and
not discover this at playtest: shorten the duration below the shortest night,
give the draught a cancel verb, or accept the overhang as the cost and say so in
the item description.

🅾️ **Owner ruling: accept the overhang, and say so on the tin.** The duration is
not shortened and no cancel verb is added. Every grant of a vision ability states
its daylight cost in the text the player reads.

🔑 **The cost differs per grant, so the text must be specific rather than
boilerplate.** A shifted window's dazzle edge is `windowDazzleEdge − strength`:

| Grant | Strength | Dazzle edge | Player-visible? |
|---|---|---|---|
| Cat's Eye Draught, condition 65 + item 30047 | 24 | **51** | Yes. Today the only player path |
| Night Vision, condition 29 | 18 | **57** | Visible, but admin `setcondition` only |
| InfraredVision, condition 85 | 12 | **63** | No, `secret: true`. Five cave mobs |

Against a daylight range of 62 to 73, the draught is dazzled through all of
daylight, condition 29 through nearly all of it, and condition 85 only around
midday in summer. A cave mob permanently dazzled by daylight is correct flavour
and needs no text, since it is secret and mobs read nothing.

**Mutations: none today.** No mutation grants either vision flag; the only three
data files mentioning them are conditions 29, 65 and 85. Plan 2 built
`mutations.FlagValue` as groundwork for the parked shapeshifter branch, so this
is written as a **standing convention rather than a one-off edit**: any future
mutation that grants a vision ability states its daylight cost in its
description, at the strength that mutation actually confers after
`LevelMultiplier(rank)` scaling.

⚠️ **The text lands in plan 5, not plan 3.** Dazzle has no mechanical effect until
plan 5 builds it, and a description that promises a penalty the game does not yet
apply is a lie to the player. Plan 3 records the convention; plan 5 writes the
words in the same slice that gives dazzle teeth, and applies it to the nightvision
spell, infravision spell and infravision potion it authors.

---

## Out of scope, and defects filed rather than fixed

- **Weather occlusion** stays plan 4. Plan 3 leaves the `-2 to 2` `LightMod`
  vocabulary alone and maps it onto the new scale at its existing meaning.
- **Darkness sources** stay plan 5. Plan 3 ships the light side of the combine
  only, following plan 1's rule that a knob nothing reads does not ship.
- **Dazzle gains no teeth in plan 3**, but it is not unowned. The original spec
  assigns "the dazzle penalty's exact effect on skills and combat rolls" to "the
  plan that implements vision abilities" (line 360), and plan 5's own "light
  becomes a weapon" paragraph (line 329) *is* the dazzle penalty: deliberately
  overloading a cave to blind its residents does nothing without one. **Plan 5
  owns it.** See the note below for how this calibration changes what it means.
- 🐛 **Weather renders in the New Plymouth sewers** (fact 12). Fixed here as a
  side effect of the `sewer` biome, but it predates this arc.
- 🐛 **Five zones name a biome that does not exist** (fact 19). Fixed here.
- 🐛 **Plan 1's promised `IsLit` predicate was never built** (fact 15). Fixed here.
- 📌 The lore's "Triple Full Moon ~every 44 days" (`world.md:65`) does not follow
  from periods of 4.7, 10.6 and 21.1; measured, all three sit within 10% of new
  only 0.59% of the time. A lore-versus-code mismatch worth a look during the
  Convergence Festival work, not this arc's to resolve.
