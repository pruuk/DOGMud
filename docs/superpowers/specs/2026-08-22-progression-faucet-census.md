# Progression Faucet Census

**Date:** 2026-08-22
**Status:** measured. Supersedes every uses/hour figure in
`docs/superpowers/plans/2026-08-22-u10b-0-phase-d-balance.md` revisions 1 and 2.

## Why this exists

The U10b-0 Phase D balance plan failed two rounds of blind adversarial review.
Both failures had one root cause: uses/hour were **asserted as free constants**
when they are **derived** from cooldown keys, gates, equipment exclusivity, and
the `SkillPrimaryStats` mapping. Tuning twenty-one multipliers against a wrong
faucet map produces numbers that look rigorous and mean nothing.

So the map was extracted mechanically instead of reasoned about. Every figure
below is from code at `fd2fcbbfa`, not from argument.

**Method:** `tools/balance/faucet_census.py` walks `internal/` and `modules/`,
finds all 85 non-test call sites of `OnStatUse`, `OnSkillUse`,
`OnSkillUseScaled`, `CheckStatProgression`, `CheckSkillProgression` and
`CheckRegenProgression`, and records for each the enclosing function, the
literal argument, every `if` guard between the function opening and the call,
and every cooldown key referenced in that function.

**Scope limit of the raw census:** it reports cooldown keys found in the
*enclosing function only*. 63 of the 85 sites show none, but most of those are
combat-hook sites that are round-paced, or command paths whose throttle lives
in a caller. The raw output is the starting point; the classification tables
below are the answer.

**Two extraction traps that produced wrong answers on the first pass**, both
now handled — flagging them because they will recur:

1. **Cooldown keys are written in both quote styles.** `assess` uses
   `` TryCooldown(`assess`, ...) `` with backticks. A `grep` for
   `TryCooldown("` reports it as unthrottled. It is not.
2. **`cast` was missing from the shared-cooldown verb list for the same
   reason** — `` char.TryCooldown(`special-move`, ...) `` at
   `internal/actions/cast.go:279`. This is the single most load-bearing fact in
   the document and a double-quote grep misses it.

## Headline: one cooldown key gates eighteen verbs

`"special-move"` is a **single shared key** at `SpecialMoveCooldown: 4` rounds.
These all draw on it:

> cast · taunt · warcry · rally · throw · surprise-attack · bash · drain ·
> gore · grapple · hamstring · kick · maul · pounce · rake · reload ·
> throttle · trip · (and every mutation action, via `mutation_helpers.go:99`)

**Consequence:** `rhetoric`, `spellcasting`, `manifestation` and much of
`skullduggery` are **not independent faucets**. They share one budget of
**225 uses/hour** absolute, or roughly **22.5/hour** at the owner's 10%
engagement ruling. Revision 2 of the plan modelled them as four separate
budgets totalling 173+/hour and was wrong by roughly 8x in aggregate.

**Ranged combat is on this budget too, indirectly.** Firing is not gated, but
`reload` is (`combat_reload.go:133`), and a weapon must be loaded to fire
(`combat_fire.go:98`). So sustained shooting costs one shared slot per shot.
The code comment at `combat_fire.go:65` states this intent explicitly.

## Cooldown groups, with real ceilings

At `RoundSeconds: 4` → 900 rounds/hour.

| Key | Duration | Verbs sharing it | Absolute ceiling |
|---|---|---|---|
| `special-move` | 4 rounds | the 18 above | **225/hr, shared** |
| `search` | 2 rounds (`search`), **1 round** (`track`) | search, track | 450/hr, shared |
| `forage` | 6 rounds | forage | 150/hr |
| `assess` | 6 rounds | assess | 150/hr |
| `skullduggery:steal` | **60 real seconds** | steal, plant | **60/hr, shared** |
| `skullduggery:shadow` | 5 rounds | shadow | 180/hr |

`StealCooldown` (60) and `ShadowCooldown` (5) are **absent from
`config.yaml`** and running on Go defaults. `search`'s "2 rounds" and
`forage`'s "6 rounds" are **hardcoded string literals, not knobs** — retuning
either requires a code change, or the multiplier must absorb the whole
adjustment.

`track` and `search` sharing one key at different durations means the two
commands interfere with each other in a way neither file mentions.

## Faucets with no cooldown at all

These are bounded only by typing speed, opportunity, or an unrelated pacer:

| Faucet | Trains | Real bound |
|---|---|---|
| `look <target>` (`look.go:85`) | perception | **none** |
| `consider` (`consider.go:27`) | perception | **none** |
| `buy` (`buy.go:786`) | bartering, charisma | **none, and awards PER UNIT** inside the purchase loop |
| `sell` (`sell.go:377`) | bartering, charisma | **none, and awards PER UNIT** |
| `salvage` (`salvage.go:166,252`) | salvage | activity duration only; takes no difficulty bonus |
| `craft` | the six crafts | recipe `time_rounds` (median 5, mode 4) |
| `picklock` | skullduggery | opportunity: locked doors |
| `defuse` | skullduggery | opportunity: traps |
| movement (`go.go:387`) | search | `MovementSearchTrainChance: 0.005` per successful move |

`look` and `consider` are the genuinely unbounded stat faucet in the game. At
one command per second that is 3,600 perception uses/hour, against `forage`'s
150 ceiling. Plan revision 2 proposed multiplying this path 4–6x.

## Gates that make a ceiling unreachable

A cooldown bounds how often you may *attempt*. A gate decides whether the
attempt *pays*. Three matter:

- **`search` (`search.go:241`)** awards progression only `if
  rolledAgainstSomething`. The comment labels it an *"anti-botting gate"*. Two
  of six candidate types are once-per-character-per-room via
  `HasDiscovery`/`AddDiscovery`. Across 1,436 authored room files there are 5
  secret-exit rooms, 12 hidden-noun rooms, 1 stash and 0 hidden containers. So
  the 450/hr ceiling is unreachable by construction, and **`search` cannot be
  ground**. Revision 2 fitted perception's multiplier to it anyway.
- **`forage` (`forage.go:142`)** awards only on a successful find — but yield
  is biome-driven and **renewable**, so it rolls forever. This is the
  structural difference from `search`, and it is why **forage is the correct
  anchor for perception** (owner ruling, from play). It also produces crafting
  materials, so it is the loop that couples the perception and crafting
  economies.
- **`assess`** is corpse-bound.

## Stat mapping corrections

- **The offhand fist adds ZERO strength.** `emitAttackerStatGain(atk,
  "strength", ...)` fires once per exchange **outside** the `WeaponHits` loop
  (`NewRound_DoCombat_unified.go:659`); the per-hit events use
  `GetSkillPrimaryStat`, which is **dexterity** for both `weapon-combat` and
  `unarmed-combat`.
- **Block and the offhand fist are mutually exclusive.** The fist requires
  `Offhand.ItemId == 0 && !IsBlockedBy2H("offhand")`
  (`combat_helpers.go:283`); block requires a shield. No character trains both.
  `tools/playtest/profiles/mid.yaml` carries no offhand and therefore **can
  never block** — it cannot measure the block rows at all.
- **`rhetoric` has a third faucet:** `awardRhetoricUse`
  (`skill_helpers.go:100`) trains it from **warcry** and **rally**, not just
  taunt and the defy defence. Both callers are on the shared cooldown.
- **Player charisma has no direct faucet.** The only two `OnStatUse("charisma")`
  sites reachable by a player are `buy.go` and `sell.go`; all other charisma
  arrives via the primary-stat auto-roll for rhetoric, bartering and
  manifestation.
- **`bartering` is not time-bound at all.** Per-unit awards with no cooldown
  mean `sell all` on a 200-item stack fires 200 uses from one command. No
  uses/hour figure can be fitted to it; it needs a different treatment.

## Difficulty bonus: use the mean, not the median

`chance` is **linear** in the bonus multiplier, so expected rate over a mixed
hour tracks the **mean**, not the median. Both distributions are right-skewed:

| pool | mean | median | plan rev 2 used |
|---|---|---|---|
| 126 craft recipes | **1.4724** | 1.40 | 1.40 |
| 59 spells | **1.2780** | 1.25 | 1.25 |
| 14 manifestation spells | **1.3393** | **1.325** | 1.35 (wrong even as a median) |

`spellBonus` additionally carries `SelfCastProgressionMultiplier: 0.5` for
self-only `HelpSingle` casts, and is **0** — no roll at all — for
`HarmArea`/`HarmMulti` that hit nothing. `salvage` takes no bonus (bare
`OnSkillUse`).

## What still needs empirical measurement

The structure above is now known. What the code cannot tell us:

1. ~~**`forage` success rate per attempt**~~ — **ANSWERED, closed-form.** Forage
   success is a plain gaussian check, so it is computable rather than sampled:
   `searchScore = Perception + SkillMultiplier(searchRank)×25`
   (`skill_helpers.go:87`), rolled `Normal(score, 0.15×score)` against a
   per-biome difficulty (`forage_core.go:129`). See
   `tools/balance/forage_rate.py`.

   | profile | per / rank | score | forest | mountains | realised uses/hr |
   |---|---|---|---|---|---|
   | fresh | 100 / 0 | 125.0 | 60.5% | 21.2% | 90.8 |
   | early | 100 / 10 | 147.4 | 89.2% | 63.0% | 133.8 |
   | mid | 110 / 20 | 166.6 | 96.9% | 85.7% | 145.3 |
   | practised | 120 / 35 | 186.8 | 99.1% | 95.3% | 148.7 |
   | soft-capped | 130 / 50 | 205.0 | 99.7% | 98.3% | 149.6 |

   **The find rate saturates by Search rank ~20.** Past that the 6-round
   cooldown is the only binding constraint, so forage's realised perception
   rate is essentially its **150/hr ceiling** for anyone past early game. Only
   a genuinely fresh character in a hard biome (mountains/cliffs, 21%/14%) is
   yield-bound rather than cooldown-bound.

   Two consequences for the retune. First, forage's rate is a **constant**, not
   a curve — the multiplier carries all of the tuning, since the cooldown is a
   hardcoded literal. Second, `look`/`consider` at ~3,600/hr is roughly **24x**
   forage's best case, so tuning perception to forage makes the ungated path
   24x over-rewarding unless it is gated too.

   **Open design question:** what does "an hour of grinding forage" mean for
   engagement? The owner's 10% ruling covers combat, where finding mobs
   dominates. Foraging is stationary and repeatable, so its realised engagement
   is much closer to 100% — which is the difference between 15 and 150
   uses/hour, a 10x swing in the solved multiplier.
2. **Realised engagement** — how much of an hour is actually spent in the
   activity versus travelling. The owner's 10% ruling is an estimate.
3. **Clean-hit probability** in real combat, which sets the weapon-combat and
   unarmed-combat rates. Revision 2 assumed 0.5 with no support; the
   best-of-all defence resolution makes the true figure structurally lower.
4. **How often the shared `special-move` budget is actually saturated.** The
   225/hr ceiling is only binding if players spend every slot.

Items 1 and 3 are measurable from a single instrumented playtest counting the
`"event","stat_use"` and `"event","skill_use"` log lines
(`progression.go:302,323`). **Not** the `"check"` lines at `:163`, `:269`,
`:521` — those sit inside `if roll < threshold` and fire only on a **gain**,
so they count successes rather than uses.

## Manifestation: measured, and neither bound the owner named is binding

The owner's framing was that manifestation is gated by *"how fast you can kill
(corpses for assess and raise) OR how fast your conviction regens (standing in
a temple with the regen multiplier)"*. Measured, **the cheapest path needs
neither**.

**The "temple" is the `sanctuary` room mutator**
(`_datafiles/world/dogmud/mutators/sanctuary.yaml`), `regenmultiplier: 5.0`,
applied in `roomRegenMultiplier` → `regenMultFromSpecs`. It is the only room
mutator in the game with a regen multiplier.

**Conviction supply.** `ConvictionMax = 5 + Cha×3 + Wil×1`. For a mid character
(Cha 100, Wil 100) that is **405**, and `ConvictionPerRound = floor(405 × 0.02)
= 8/round` at the shipped `PlayerConvictionRegenPct: 0.02`. Regen fires every
round in `NewRound_AutoHeal`.

| | CP/round | CP/hour |
|---|---|---|
| anywhere | 8 | 7,200 |
| in a `sanctuary` room | 40 | 36,000 |

**Spell costs** (all 14 manifestation spells, from their YAML):

| spell | cost | difficulty | waitrounds | needs |
|---|---|---|---|---|
| conjure-water | 30 | 15 | 2 | nothing |
| raise-skeleton | 30 | 10 | 4 | corpse (`summon_requires_corpse: true`) |
| summon-hive-swarm | 30 | 45 | 3 | nothing |
| conjure-air | 40 | 30 | 3 | nothing |
| raise-zombie | 35 | 20 | 6 | corpse, pool ≥ 60 |
| conjure-magma | 50 | 55 | 5 | nothing |
| raise-golem | 50 | 50 | **16** | corpse |
| charm | 120 | 60 | 1 | a hostile mob |

**The binding constraint is the shared `special-move` cooldown, not CP.** At 4
rounds that is 225 casts/hour. Comparing:

| path | CP bound (no sanctuary) | CP bound (sanctuary) | waitrounds bound | **actual** |
|---|---|---|---|---|
| conjure-water (30) | 240/hr | 1,200/hr | 450/hr | **225/hr — cooldown** |
| median spell (40) | 180/hr | 900/hr | — | 180/hr, or 225 in sanctuary |
| charm (120) | 60/hr | 300/hr | 900/hr | 60/hr, or 225 in sanctuary |
| raise-golem (50) | 144/hr | 720/hr | **56/hr** | **56/hr — waitrounds** |

So a player grinding manifestation spams **conjure-water**: cheapest, lowest
difficulty, no corpse, no target, `waitrounds` below the shared cooldown. It
runs at the **cooldown ceiling of 225/hr even outside a sanctuary**, and it is
not combat-gated at all — you can stand still and do it, so its realised
engagement is near 100%, not 10%.

Two secondary constraints exist and do **not** bind: `CompanionSoftCap` is 5
(Go default, absent from `config.yaml`), and summons hold conviction in reserve
against a `PoolReservationCapPct: 0.66` ceiling — but **`dismiss` has no
cooldown and costs no special-move slot**, so the summon/dismiss cycle is free.

**Consequence for the retune:** manifestation is one of the *easiest* tracks to
grind, not one of the hardest. Any multiplier fitted to an assumed corpse or
regen bottleneck will over-reward it by roughly 4x (225/hr realised against the
~56/hr the raise-line framing implies).

## Live confirmation, and a failed measurement run

An instrumented session was run on current code
(`tools/playtest/goals/2026-08-22-u10b0-phase-d-rate-measurement.yaml`, run
`d1e12f2437c85587`, commit `787b6fcbd`) to measure clean-hit rate and fight
length post-arc. **It failed its primary goal** and is reported in full at
`tools/playtest/reports/2026-08-22-local-feature-tester-phase-d-rate-measurement.md`.

It did confirm two census findings live:

- **The ungated perception faucet is real.** The very first `consider` issued
  printed `STATISTIC INCREASED / perception`. One command, no cooldown, no
  gate.
- **The offhand fist fires as modelled** — one exchange narrated both
  `Your Iron Longsword TEARS THROUGH...` and `...slam them with your fists!`,
  confirming two `WeaponHits` entries from a 1H weapon with an empty offhand.

**Why it failed, and what it means for the rig:** `mid.yaml` starts in an urban
hub (room 462, Thornwall City) eight moves from any hostile. The first matched
enemy was hidden and fled; the only reliable enemy died in one round and
respawns slowly. Add this to the reviewer's finding that `mid` cannot exercise
8 of 16 tracks: **it also cannot reach matched combat inside a 30-minute
budget.** The container's analytics were never flushed
(`FlushIntervalSec: 300`) before the watchdog tore it down, so the swings that
did occur were lost.

**Consequence for the retune:** the best available clean-hit measurement
remains the **96,723-event historical dataset at 0.5752**, with the explicit
caveat that it predates U6b, U9, U10 and Phases A/B/C. That is three orders of
magnitude more data than a 30-minute session can produce, and its weakness is a
stated caveat rather than a missing number. Revision 2's assumed 0.5 is wrong
by ~15% against it either way.


## Crit instrumentation gap, and what can actually crit

Found while modelling vitality (2026-08-22).

**`RecordSpecialMove` cannot record a crit.** Its signature takes `hit` but no
`crit`, and it never sets `Crit`, `AttackZScore` or `DefenseZScore`
(`internal/combat/analytics.go:280`). Every special move therefore reads **0.0%
crits** in the analytics across 96,723 events:

| type | events | hits | crits |
|---|---|---|---|
| unarmed | 70,616 | 40,167 | 2,976 (4.2%) |
| weapon | 21,612 | 13,405 | 1,734 (8.0%) |
| spell | 459 | 363 | 10 (2.2%) |
| **taunt** | **331** | **199** | **0** |
| grapple / kick / trip / bash / hamstring / pounce / stomp / tailsweep / howl / shoot / throw / maul / bite / gore / knee / submit / throttle / rake | ~1,800 combined | — | **0** |

Zero crits across ~1,800 special-move events is not luck (taunt alone at a 2%
rate would give P(0 in 331) = 0.13%); it is the missing parameter. **The combat
dashboard silently under-reports crits**, and any crit rate computed from this
file is a melee-and-spell rate only.

**Special-move crits ARE implemented; only the RECORDER is stale.** This was
checked after the fact, because "the analytics show zero" reads like a dropped
feature and is not one. `combat_taunt.go:236` takes `isCrit := out.AttackerCrit`
-- a real margin-derived verdict from the unified seam -- and a taunt crit
bypasses the target's conviction mitigation AND scales by the taunter's rhetoric
rank through `CritOrMitigatedDamage`. bash and kick carry `ForceCrit` and their
own crit-tier handling. The unified-crit goal was met in gameplay; only
`RecordSpecialMove`, which predates it, never gained the parameter.

**What can crit, from code rather than data.** `channelDamageChannel`
(`defence_multiplier.go:561`) maps `ChannelMelee`/`ChannelRanged` -> "physical",
`ChannelSpellPhysical`/`ChannelSpellMental` -> "magical", `ChannelSocial` ->
"conviction"; `ToughenStatFor` then gives vitality / willpower / charisma. Taunt
DOES route through `ResolveChannelAttack(ChannelSocial, ...)`
(`combat_taunt.go:176`), so **a taunt crit structurally toughens the target's
charisma** — it simply cannot be observed today.

**There is no stamina crit.** The three damage channels are physical, magical
and conviction. Stamina is a cost pool, not a damage channel, so nothing
toughens off stamina damage through the crit path.

**Followup:** add a `crit` parameter to `RecordSpecialMove` and thread it from
the special-move resolutions, so the dashboard and any future balance work see
the real rate.
