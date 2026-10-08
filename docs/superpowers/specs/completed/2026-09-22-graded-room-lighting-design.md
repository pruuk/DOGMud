# Graded room lighting

Replaces the coarse three-value room visibility with a graded light scale,
gives the three moons and the day/night cycle a visible purpose, and turns
nightvision and infravision into strength-bearing abilities that shift where
an observer can see rather than simply granting sight.

Written 2026-09-22 against master `389f4ad99`, immediately after messaging M5.

**One spec, several plans.** The plan split is at the foot.

---

## Facts verified against source

Read from the tree on 2026-09-22. Rows marked 🔴 corrected an assumption made
earlier in the same session.

| # | Fact | Evidence |
|---|------|----------|
| 1 | `Room.GetVisibility()` returns 0, 1 or 2. Base 2, night minus 1, dark biome minus 2, lit biome plus 1, mutator `LightMod` summed, any light source plus 1, clamped | `internal/rooms/rooms.go:145-190` |
| 2 | ⚠️ **CORRECTED BY PLAN 1.** The real count is **11 call sites, 9 of them binary**, plus the interface declaration and the `litRoom` stand-in. `look.go` reads the integer **twice**, not once | compiler enumeration after deleting `GetVisibility`; a repo-wide grep then found 20 more `_test.go` files `go build` cannot see |
| 3 | 🔴 **DISPROVEN BY PLAN 1. "2 buys nothing over 1" is FALSE.** Sight asks `>= 1`, but `look.go` gates seeing THROUGH an exit on `< 2`, so 2 buys corridor-peering. This is why plan 1 preserved **two** thresholds; collapsing them onto one would have silently changed exit-peering in every lit-but-not-bright room | `internal/usercommands/look.go:258` before migration, now `LightExitsAbove` |
| 4 | 🔴 Therefore **night alone never produces darkness**: base 2 minus 1 is 1, which reads lit | facts 1 and 3 together |
| 5 | `mutators.LightMod` is declared `-2 to 2` and is summed into visibility | `internal/mutators/mutators.go:70`, `rooms.go:170` |
| 6 | **6 of 24 shipped mutators declare a `lightmod`**: `weather_blizzard`, `weather_dust`, `weather_storm` darken; `bioluminescent_caves`, `foldweave_glow`, `hull_suppression` brighten | `_datafiles/world/dogmud/mutators/` |
| 7 | 🔴 **Four** biomes set `darkarea: true`: `cave`, `dungeon`, `swamp`, and `spiderweb` which no shipped room uses | `_datafiles/world/dogmud/biomes/` |
| 8 | 140 of 1387 rooms carry a dark biome, but **43 are permanently lit** by static `lightmod: 2` mutators (31 Crash Site Interior, 12 Foldweave). Genuinely dark: **97** | room YAML, verified per zone |
| 9 | The three moons already exist with smooth phase contributions in `[0,1]` | `internal/gametime/moonphase.go`, `moonContribution` |
| 10 | 🔑 **The moons are already load-bearing**, not flavour: they drive combat stat deltas and gate bloom mutations | `hooks/NewRound_DoCombat.go:498-504`, `characters/bloom_mutation.go:100` |
| 11 | Lore gives each moon a period, distance and visual size | `docs/world.md:52-56` |
| 12 | 🔑 Lore constrains the design: **"True darkness is rare, as at least one moon is usually visible."** Also: low mass, 20% of Luna, but closer | `docs/world.md:70` |
| 13 | **`chrysalis-glow` is the only light source in the game**, and it is a STARTER SPELL granted to every new character, commented "light source for caves" | `internal/characters/character.go`, `StarterSpells` |
| 14 | **No shipped item grants light.** Lanterns and candles are crafting materials with no condition | exhaustive item grep |
| 15 | Condition 1 Illumination carries the `lightsource` flag; 29 is NightVision; 85 is InfraredVision | `_datafiles/world/dogmud/conditions/` |
| 16 | **0 species, 0 mobs and 0 items grant InfraredVision**, so the `SightShapes` tier is currently unreachable | verified in M5 |
| 17 | `SightDecision` has exactly three values, and `ParticipantSight` is optics-only while `CanSeeClearly`/`CanSeeShapes` compose attention | `internal/messaging/predicates.go` |
| 18 | 🔑 **Modules depend on `internal`, never the reverse.** No file under `internal/` imports `modules/` | verified by grep, empty result |
| 19 | 🔑 **Weather and seasons already reach rooms through MUTATORS.** `ReconcileSeasons` applies `season-<track>-<season>` mutators to zone configs | `modules/weather/engine/apply.go:75-108` |
| 20 | Seasons are computed **per zone**, so climates can differ | `modules/weather/seasons/`, `ZoneSeasons` |
| 21 | `GameDate` carries `Hour24`, `Night`, `DayStart`, `NightStart`. It has **no season field** | `internal/gametime/gametime.go` |
| 22 | There are 12 named months | `internal/gametime/months.go` |
| 23 | A FUTURE note already awaits this work: compare visibility before and after a light-bearing actor moves, to reroll hidden actors | `internal/hooks/Awareness_LightChange.go:73` |
| 24 | ✅ **PLAN 1 SHIPPED 2026-09-22**, PR #159, master `7e200aa47`, not deployed. `GetVisibility` deleted, `LightLevel()` live, bands at 25/50/65, behaviour preserving | `internal/rooms/lighting.go`, `internal/configs/config.balance.lighting.go` |
| 25 | The shipped room count is **1386**, not 1387. Fact 8's dark-biome figure of 140 verified exactly: cave 120, dungeon 1, swamp 19, spiderweb 0 | `testdata/lighting_parity.golden`, biome histogram |
| 26 | 🔑 **`nightvision` has exactly ONE content path: condition 65, granted by item 30047 Cat's Eye Draught.** 500 rounds, costs `strength: -10`, also grants `see-hidden`, craftable at alchemy `skill_minimum: 12`, sold by no vendor | `items/consumables-30000/30047-cats_eye_draught.yaml:12-13`, `conditions/65-cats_eye_draught.yaml`, `recipes/alchemy/cats-eye-draught.yaml` |
| 27 | 🔴 **Condition 29 "Night Vision" is granted by NOTHING**, and **`infraredvision` (condition 85) is granted by NOTHING**. No item, spell, mob, species or mutation. Admin `setcondition` only, which is admin-role gated | exhaustive grep of items, spells, mobs, mutations, species |
| 28 | 🔴 **117 of 1386 rooms resolve to a synthetic `default` biome built in Go, not authored in data, and it is `LitArea: true`.** Those rooms can never darken by biome, so plan 3's ambient will leave them permanently bright until the gaps are authored | `internal/rooms/biomes.go` `LoadBiomeDataFiles`, ~100 boot warnings from `internal/mapper/mapper.go:1192` |
| 29 | No dazzle threshold knob exists. Plan 1 deliberately omitted it because nothing would read it | grep `internal/configs` for `Dazzle`, empty |

---

## What is wrong today

The room-level location is right. `LightMod` mutators already let weather fight
lighting. Four things are wrong:

1. **The scale is binary in practice** (facts 1, 3). Two of the three values
   are indistinguishable to sight.
2. **Light sources are counted, not weighed** (fact 1). A candle and a bonfire
   are both `+1`, and ten of either are still `+1`.
3. **Night is inert** (fact 4), so the day/night cycle and the moons that
   already exist (fact 9) never change what anybody can see.
4. **The middle sight tier is unreachable** (fact 16). Lighting jumps from
   nothing to everything, so "you can tell someone is there, not who" never
   happens in an ordinary room.

---

## The model

### The scale

**Light runs from −100 to 100.** Zero is the absence of light. Negative is
magical darkness: light actively removed rather than merely absent.

### Vision is a window, not a bonus

🔑 **This is the load-bearing idea.** An ability does not add light to the
room. It moves where the observer's usable band sits on the scale.

A normal observer:

| Light | Band | Result |
|---|---|---|
| 75 to 100 | too bright | `SightFull`, plus a dazzle penalty |
| 50 to 75 | perfect | `SightFull` |
| 25 to 50 | too dim | `SightShapes`, names hide |
| below 25 | blind | `SightNone` |

**NightVision shifts the whole window left by its strength, up to 24 points,
and is blind below 1.** So maximum nightvision sees perfectly from 26 to 51,
reads shapes from 1 to 26, and is blind below 1. Because the window moves
rather than widening, its upper edge falls too: a nightvision user is dazzled
in ordinary daylight, and takes a slightly steeper dazzle penalty than a
normal observer.

**InfraredVision is nightvision plus a downward extension**, and it carries
**two numbers**. Above zero it behaves exactly as nightvision, shifting the
window left by its nightvision strength, subject to the same 24 point cap.
Below zero its separate **infra reach** decides how far down it still reads
`SightShapes`, sensing heat rather than seeing. The two are independent: a
creature can sense heat deeply while being no better than a normal observer at
using faint light.

🔑 **Magical darkness is therefore definable rather than arbitrary:** it is
any light below the reach of the strongest infravision. That is the only state
in which nothing sees at all.

### The ladder this produces

| Situation | Light | Normal | NightVision 24 | Infra 30 |
|---|---|---|---|---|
| Summer noon, clear | 78 | dazzled, barely | dazzled | dazzled |
| Spring or autumn noon | 70 | perfect | dazzled | dazzled |
| Winter noon | 62 | perfect | dazzled | dazzled |
| Dawn or dusk | 55 | perfect | dazzled | dazzled |
| Night, three moons full | 40 | shapes | perfect | perfect |
| Night, typical phases | 30 | shapes | perfect | perfect |
| Night, all three new | 12 | **blind** | shapes | shapes |
| Cave, unlit | 0 | blind | blind | shapes |
| Magical darkness | −40 | blind | blind | **blind** |

Two properties worth naming. A moonless night is **the only outdoor case that
blinds a normal person**, which satisfies the lore constraint at fact 12.

And the dazzle band from 75 up is **barely touched by ordinary conditions**.
Summer noon at 78 just crosses its lower edge, deliberately, so midsummer
midday is the one time of year daylight itself makes a normal person squint.
Everything well inside that band is exceptional: glare off snow or desert, a
light spell, a flare. Spring, autumn and winter noon all stay in the perfect
band, so daylight is comfortable for three seasons out of four.

### Composition

Room light is built in three steps.

**1. Celestial ambient.** The sun contributes on a curve that peaks at local
noon and falls to zero through dusk, scaled by a **seasonal peak**: summer 78,
spring and autumn 70, winter 62. The three moons contribute at night, each by
its own phase.

🔑 **The moon values are derived from lore, not invented.** Brightness at the
ground scales as angular area times albedo, and angular area scales as the
square of apparent size (fact 11):

| Moon | Apparent size | Relative area | Character |
|---|---|---|---|
| Swiftmoon | 2x Luna | 4.0 | large, closest, fastest cycle |
| The Wanderer | ~Luna | 1.0 | the baseline |
| The Eye | small but explicitly bright | ~0.25 area, high albedo | distant, reflective |

So Swiftmoon dominates a bright night, the Wanderer is the steady middle, and
the Eye is a small sharp contributor on the longest cycle. Because the periods
are 4.7, 10.6 and 21.1 days, all three sitting near new at once is genuinely
uncommon, which is exactly what fact 12 requires and what makes the
Convergence Festival at `world.md:780` the same arithmetic pointed the other
way.

**2. Occlusion.** Weather **multiplies** the celestial term, it does not
subtract from it. A blizzard blocks a fraction of whatever light is falling,
so it is devastating at midnight and merely gloomy at noon. Subtracting a flat
amount would be absurd in both directions.

**3. Sources, with diminishing returns.** Every light source in the room is
ranked by magnitude. The brightest contributes in full, the next at half, the
next at a quarter, halving and **rounding down**, stopping when a term rounds
below 1. Darkness sources stack identically among themselves. The two totals
then meet: `light = ambient_after_occlusion + lights − darks`.

Five glows of 30 give `30 + 15 + 7 + 3 + 1 = 56`, so **stacking converges near
twice the brightest source** however many are brought. That preserves the
intuition that more lamps help, without letting a pile of cheap lights brute
force a strong darkness.

⚠️ **This makes relative magnitudes the whole balance lever**, and fact 13
makes that urgent: every character starts with a light spell. If the starter
glow and a darkness spell are close in magnitude, darkness is trivially
cancelled by anyone. These numbers belong in `config.yaml` beside the
project's other balance knobs, not in Go.

---

## Architecture

The split matters because only the last two rows are per-observer. Room light
is computed once; each watcher then judges it against their own window.

| Layer | Owns | Lives in |
|---|---|---|
| Celestial ambient | sun by hour, three moons by phase | `internal/gametime` |
| Occlusion and season | weather blocking, seasonal noon peak | **mutator specs, authored in YAML** |
| Room light | ambient, occlusion, stacked sources | `internal/rooms` |
| Vision window | where an observer's band sits | `internal/messaging` predicates |
| Sight tier | light judged against that window | the existing `SightDecision` |

🔑 **Seasons and weather stay in their module.** Facts 18 and 19 give the
answer: modules cannot be imported by `internal`, but weather already reaches
rooms by applying mutators, and mutators already carry light data. So a season
declares its effect on daylight **on its own mutator spec**. `internal` never
learns what a season is; it reads whatever mutators are active. Per-zone
seasons keep working unchanged because the mutators are already per-zone.

**The three sight tiers do not change.** `SightDecision` keeps its three
values and its best-to-worst ordering. Everything downstream, including the
crime gate that just landed and all of the narration work from M3 through M5,
keeps working because it asks a per-observer question that still has the same
three answers.

---

## Migration

Fact 2 is what makes this affordable. Of the 14 consumers of `GetVisibility`,
12 only ask "is it lit". Those migrate to a named predicate that says what
they actually mean. `look.go` is the one real consumer of the number and gets
the graded value.

`GetVisibility` itself is **deleted rather than kept as a shim**, so the
compiler enumerates every consumer. That is this project's established
refactoring idiom and it worked in M5 PR 3, where it found a fifth call site
the design had missed.

---

## Consequences accepted

- **Nightvision hurts in daylight.** That is the point of a window that moves
  rather than widens, and it is what gives strength a cost as well as a
  benefit.
- **A second identical lamp adds little.** Diminishing returns are deliberate
  and must be said plainly in the help text.
- **Dim rooms now hide names.** The `SightShapes` tier stops being unreachable
  (fact 16), which retroactively makes M5's crime gate and its shapes-only
  witness arm live content.
- **Every ambient, spell and item light value in the game must be retuned**
  against the new scale. The current `-2 to 2` `LightMod` vocabulary does not
  survive.

---

## Source strength, and the self-throttle

Owner ruling 2026-09-22, for plan 5.

**Strength and duration both scale from stat and skill.** The house idiom is
`stat / statDivisor + skill / skillDivisor` with both divisors as config
knobs, which is how unarmed damage already scales
(`internal/characters/combat.go:96-98`). `GetSkillLevel` runs 0 to 100
(`combat.go:105`), so an endgame caster at 175 stat and 65 skill yields
`175/10 + 65/2 = 50` on the light scale. Two knobs rather than one lets a
balance pass move stat and skill weighting independently.

⚠️ **Duration scaling overlaps the pending spell scaling unification arc**,
where `effect_type: buff` spells get no duration scaling at all today. Scale
it locally here, and record that the unification should absorb it rather than
leaving these spells as two new exceptions.

### The throttle

An adjustable source, spell or hooded item, trims itself so its bearer is
never dazzled by their own light. The target is the top of the bearer's own
perfect band, which the window model already defines: 75 for a normal bearer,
`75 - strength` for one running nightvision.

```
my output = clamp(my target - current room light, 0, my max strength)
```

🔑 **Adjustment happens ON EVENTS, not every round.** Owner ruling
2026-09-22. The trigger set is exactly:

- **entering a room**, each mover in entry order,
- **a manual command**, or shutting a hood.

**A source does NOT trim when it is created.** A spell is cast at FULL
strength and stays there until its bearer moves. **Sources already in the room
do NOT re-trim** when someone else walks in, and nothing re-trims on a round
tick.

### Casting at full strength is the point

Owner ruling 2026-09-22. A strong caster's light flares blindingly when first
summoned, and only settles to that caster's own preference once they move.

This is deliberate on three counts. It keeps magic a committed act rather than
a thermostat. It means **the offensive flash is the DEFAULT rather than an
override**: casting a powerful light into a dark room is itself the weapon
against creatures whose windows sit low, with no special mode to invoke. And
it is one fewer trigger, because creation stops being an adjustment event at
all.

The manual control therefore exists for the opposite purpose from the one it
first appears to serve: not to force a light UP, which a fresh cast already
does, but to force it DOWN without walking somewhere.

### Why event-based, and not continuous

🔴 **Continuous adjustment is circular.** A source's output depends on room
light, and room light includes that source, so two adjustable lights would
each assume the other fixed and oscillate between rounds. Event-based
adjustment removes the circularity rather than managing it: each adjustment is
a one-shot computation against a room state that is stable at that instant,
and entry order is a real ordering the room already has, so no artificial
tie-break rule is needed.

🔑 **It also rescues darkness spells.** Under continuous adjustment every
light in the room would automatically crank up to compensate for an incoming
darkness spell, so darkness would essentially never work. Because lights are
already committed when the darkness lands, a darkness spell actually bites.

🔑 **The trigger already exists.** `internal/hooks/Awareness_LightChange.go`
fires on movement, already filters to actors who emit light, and carries a
FUTURE note (fact 23) to compare visibility before and after the move. This
rule lands on a hook that is already waiting for it.

### The moment this deliberately creates

If the room changes around a standing player, dawn breaking, weather lifting,
an enemy casting darkness, their light is now wrong and **stays wrong until
they act**. That is a moment, not a gap, and it wants a notice rather than a
silent correction: "your glow has become painfully bright". Automatic
correction here would be the continuous model sneaking back in, and would
neuter darkness a second time.

### Two consequences

🔑 **Light becomes a weapon.** A creature with nightvision has its window
shifted left, so its dazzle threshold is lower than a normal creature's.
Deliberately overloading a cave blinds precisely the things that live there.
This is why a **manual full-power override must exist** beside the automatic
throttle: the throttle is the default, not a cage.

**A nightvision bearer under-lights the party**, targeting 51 rather than 75
and leaving normal companions at the bottom of their perfect band. That is
correct behaviour and wants a help-text line, not a special case.

### What items are for

A hooded lantern trims on the same events a spell does, so it must be
justified on other grounds. The honest ones: a lantern costs no conviction,
requires no spellcasting skill, works while silenced, and can be handed to a
companion or a mob. Item 40038 Oil Lantern already ships as a crafting
material (fact 14 says nothing grants light today) and becomes functional
rather than being invented.

A hood also gives an item something a spell lacks: it can be **shut and
reopened by hand** without recasting, which is the cheap manual override for
the moment described above.

---

## Out of scope

- **Rebalancing what darkness does to combat.** `DarknessCombatPenalty`
  already exists and keeps its current meaning.
- **New content for dark places.** M5 measured that faction mobs and true
  darkness coexist in roughly 16 rooms. Whether to build more is a content
  decision, not this arc's.
- **The dazzle penalty's exact effect on skills and combat rolls.** The band
  is defined here; what the modifier multiplies is a balance question for the
  plan that implements vision abilities.

---

## The plan split

Six plans, each shipping on its own. The order is a dependency order.

1. ✅ **SHIPPED, PR #159, master `7e200aa47`** (see fact 24). **The scale and
   the bands.** The graded value, the normal observer's band
   thresholds, and every consumer migrated. Thresholds and the mapping are
   chosen so every shipped room keeps its current classification, so this plan
   is behaviour-preserving despite replacing the whole model.

   ⚠️ **NightVision and InfraredVision keep their current flag shortcuts in
   this plan.** Replacing them with windows is plan 2. They are left alone
   here because the window model is a deliberate behaviour change for them:
   today a NightVision holder sees fully in a pitch dark room, and under the
   window model that holder is blind below 1. Changing both the scale and
   their behaviour in one plan would make the behaviour-preserving guarantee
   impossible to assert.

2. **Vision windows.** NightVision gains strength, InfraredVision gains
   strength and its negative reach, magical darkness gets its floor, and the
   flag shortcuts left in plan 1 are replaced.

   🔴 **This MUST land before ambient.** Plan 3 creates genuinely dim rooms
   for the first time. A dim room is caught by the band check before plan 1's
   flag shortcut is reached, so a NightVision holder would silently regress
   from full sight to shapes at night. The window model is what makes a dim
   room correct for them.

   **Owner decisions, 2026-09-22, after facts 26 and 27 were verified:**

   - **The Cat's Eye Draught is REWORDED, not rescued.** Under the window
     model its nightvision leaves the drinker blind in an unlit cave, which is
     where its current description promises the most. Rather than bolt an
     infra reach onto a potion that never claimed heat-sense, plan 2 rewrites
     its description and its start and end messages so it reads as a
     dim-and-moonlight tool. It keeps `see-hidden` and its `strength: -10`
     cost. The true-dark answer arrives in plan 5 as its own content.
   - **Infravision is built AND granted to mobs in this plan.** Fact 27 says
     nothing grants it, and shipping a mechanism with no consumer is the
     unread-surface trap that had to be stripped from M5 PR 3 before merge.
     Granting it to cave-dwelling mobs makes both halves of the window live,
     and makes "mobs perceive darkness" real rather than notional.
   - **Dazzle gets NO penalty here.** The too-bright band becomes reachable in
     this plan, because a nightvision holder at light 60 or 70 sits above their
     shifted upper edge. It resolves as `SightFull` with no mechanical effect
     and **no `LightDazzleAbove` knob ships**, following plan 1's own rule
     against shipping a knob nothing reads. Dazzle earns teeth when something
     reads them.
   - **Plan 2 ships on its own.** Nothing deploys until the messaging arc AND
     this entire arc are finished, so the plan-2-to-plan-3 window in which
     nightvision is strictly worse is never experienced by a player.

3. **Celestial ambient.** The sun curve, the three moons derived from lore,
   and the seasonal noon peak delivered through season mutators. This is the
   plan that makes the day/night cycle matter.
4. **Occlusion.** Weather mutators become multiplicative blockers, and the
   `-2 to 2` `LightMod` vocabulary is retired in favour of the new scale.
5. **Light and darkness as play.** A darkness spell, light-bearing items
   (fact 14 says there are none today), and `chrysalis-glow` retuned against
   the scale it now lives on. Strength and duration scale from stat and skill.
   A spell is cast at FULL strength and trims only when its bearer moves, so
   the blinding flash of a fresh cast is the default weapon against creatures
   whose windows sit low, and the manual control exists to force a light DOWN
   rather than up. See "Source strength, and the self-throttle".

   **Owner addition, 2026-09-22:** this slice also ships **a nightvision
   spell, an infravision spell, and an infravision potion**. Fact 27 is why:
   today a single crafted potion is the entire vision-ability surface in the
   game. Plan 2 deliberately rewords that potion down to a dim-light tool
   rather than widening it, so this slice is where seeing in TRUE dark becomes
   obtainable, and where it is priced. Build these against the window model
   plan 2 established: a spell grants a nightvision strength, and the
   infravision spell and potion grant a negative reach as well.
6. **Balance and the deferred gate.** Retune every ambient and source value,
   then run **M5 PR 3's deferred adversarial playtest**, which this arc exists
   to make meaningful.

Plan 1 is the risky one and carries no features. Plans 2 through 5 are each a
new input to a model that already works. Plan 6 is where it is made to feel
right.
