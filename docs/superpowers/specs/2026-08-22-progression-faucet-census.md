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

1. **`forage` success rate per attempt**, by biome and by Search rank. Sets
   perception's realised rate against its 150/hr ceiling.
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
