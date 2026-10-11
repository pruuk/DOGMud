# Mitigation curve: a knee, diminishing returns and a 90% cap (#465)

**Status:** DRAFT, for owner review (2026-10-10).
**Issue:** #465. **Explorer:** the owner tuned the numbers on the Mitigation Curve Lab page
(knee 50%, cap 90%, aim 5 points, bend 40%).

## Goal

Today every damage channel adds up its raw mitigation from all sources, then clamps it at
75%. Shipped content already stacks raw mitigation far past that, so the clamp is doing all
the balance work and every source past the clamp is wasted. This change replaces the clamp
with a curve:

- linear up to a **knee** (50%);
- **diminishing returns** above the knee;
- a **hard cap of 90%**, reachable in principle but not by today's best builds;
- the part of mitigation **above the knee also softens crits**, which today ignore
  mitigation entirely (owner, 2026-10-10).

The owner chose these numbers knowing that heavily invested players get tougher than today,
mid builds slightly weaker, and that some high-end content (the crash site ship) may get
harder. A playtest judges the feel; any retune is a config edit.

## Facts verified against source (master `b33676790`)

| Fact | Source |
|---|---|
| Each getter returns `(gear points x gear effectiveness + non-gear points) / 100`, uncapped | `internal/characters/combat.go:194`, `:235`, `:273` |
| Physical non-gear: effect `mitigation_flat`, mutation `natural_armor`, statmod `physical_mitigation`, species `NaturalArmor` | `combat.go:187-192` |
| Magical: mutation `magical_damage_reduction` x100, statmod `magical_mitigation`, effect `mitigation_magical`; conviction likewise | `combat.go:230-233`, `:268-271` |
| Cap knobs `PhysicalMitigationCap`, `MagicalMitigationCap`, `ConvictionMitigationCap` | `internal/configs/config.balance.go:343-345` |
| Validator resets a cap that is `<= 0` or `> 1.0` to 0.75 | `internal/configs/config.balance.combat.go:378-386` |
| Shipped caps 0.75, 0.75, 0.75 | `git show HEAD:_datafiles/config.yaml`, lines 1199-1201 |
| `ApplyMitigation(raw, pct, cap)` = `raw * (1 - min(max(pct, 0), cap))`; `MitigationCap(channel)` reads the knobs, unknown channel 0.75 | `internal/combat/damage_pipeline.go:105`, `:119` |
| 11 non-test call sites of `MitigationCap(` | `grep -rn "MitigationCap(" internal modules` |
| A crit skips mitigation: `CritOrMitigatedDamageScaled` uses `raw * CritDamageMultiplier * bonus` on a crit, `ApplyMitigation` otherwise | `internal/combat/crit_damage.go:95-112` |
| Melee crit: `critMean := sdp.rawDmgForCrit * sdp.critDmgMult`, unmitigated | `internal/combat/combat_helpers.go:1477` |
| Armor piercing: `MitigationMultiplier` (stomp 0.5) scales raw mitigation before the cap | `internal/combat/skill_moves.go:94`, `:202` |
| `PowerScore` averages the three raw getters with no cap | `internal/combat/calculations.go:101` |
| `status` shows `mitigationQuality(raw)`; "fortified" from 70% | `internal/templates/templatesfunctions.go:365` |
| No curve or diminishing-returns helper exists; nearest prior art is `compressContestGap` (`u/(1+k*u)`) | `internal/combat/run_contest.go:91` |

Real numbers (estimates from shipped YAML, one channel maximised at a time): best drop-able
gear gives about 91% physical, 82% magical, 56% conviction raw. A heavily invested build
(gear, enchantments, potions, a level-4 mutation) reaches about 181% physical, 164% magical,
186% conviction; a ward on top reaches about 270-330% physical. The toughest mob, The
Sentinel (9552), has raw 75% physical, 97% magical, 93% conviction. The crash site mobs
(Repair Frame 9585, Grapnel Warden 9586, Hull Sweeper 9587) have about 35% physical and 25%
magical, below the knee.

## Design

### 1. The curve

One pure function in `internal/combat`:

```
EffectiveMitigation(raw float64, c MitigationCurve) float64
  raw <= 0          -> 0
  raw <= knee       -> raw
  otherwise         -> x = raw - knee
                       min(knee + (cap + aim - knee) * x / (x + bend), cap)
```

`MitigationCurve` holds `Knee`, `Cap`, `Aim`, `Bend`. `aim` sets where the curve would level
off without the cap: at 0 it only approaches the cap, above 0 the cap is reachable. The curve
is continuous at the knee, increasing, and never exceeds the cap. A NaN or negative raw
returns 0 (the guard `compressContestGap` uses).

With the shipped numbers (knee 0.50, cap 0.90, aim 0.05, bend 0.40):

| Raw | 50% | 75% | 91% | 100% | 164% | 181% | 270% | 330% | 370% |
|---|---|---|---|---|---|---|---|---|---|
| Effective | 50% | 67% | 73% | 75% | 83% | 84% | 88% | 89% | 90% |

### 2. Config knobs

Twelve knobs in `config.balance.go`, four per channel: `PhysicalMitigationKnee`,
`PhysicalMitigationCap` (existing, meaning kept: the hard cap), `PhysicalMitigationAim`,
`PhysicalMitigationBend`, and the same for `Magical` and `Conviction`. Shipped in
`_datafiles/config.yaml`: 0.50, 0.90, 0.05, 0.40 for every channel. The validator enforces
`0 < knee < cap <= 0.95`, `aim >= 0`, `bend > 0`, and on a bad value falls back to the
shipped defaults with a log line, as `validateCombat` does today.

`config.yaml` carries the skip-worktree bit: the commit is built from the `git show HEAD:`
blob plus the new keys, never from disk (`dogmud-balance-config`).

### 3. Where the curve applies

`MitigationCap(channel)` becomes `MitigationCurveFor(channel) MitigationCurve`.
`ApplyMitigation(raw, pct, curve)` uses `EffectiveMitigation(pct, curve)` in place of the
clamp. Every one of the 11 call sites keeps its shape and passes the curve. Armor piercing
still scales raw mitigation before the curve, so a stomp halves raw, then bends.

The readers that skip the cap today read the effective value:
- `PowerScore` averages the three effective values, so `consider`, mob flee and target
  choices and the leaderboard see what combat sees;
- `status` passes the effective value to `mitigationQuality`, so its bands describe what
  the player actually has.

### 4. Crits

A crit is reduced by the overflow above the knee:

```
critReduction = max(0, EffectiveMitigation(pct, curve) - curve.Knee)
critMean      = raw * CritDamageMultiplier(rank) * bonus * (1 - critReduction)
```

This lands in `CritOrMitigatedDamageScaled` and in the melee crit path
(`combat_helpers.go`, `critMean`), using the same effective value after armor piercing. A
target at or below the knee takes full crits, as today. The defence multiplier stays skipped
on crits. At 85% effective, a crit with multiplier 2.0 lands near 1.3x raw and a normal hit
at 0.15x raw, so a crit is still about nine normal hits.

### 5. Player text

No new player lines. No help page states the 75% cap today (`grep -rl "75%"
_datafiles/world/dogmud/templates/help` finds nothing); the plan re-checks help pages that
describe armor or mitigation in words. `PATCH_NOTES.md` explains the change in
words: protection keeps paying off past where it used to stop, very heavy protection now
also softens critical hits, and nothing reaches total immunity.

## Testing

- Curve: continuity at the knee, monotonic over a sweep, never above the cap, the table in
  section 1 within rounding, NaN and negative raw, `aim = 0` never reaching the cap.
- Config: defaults, each invalid value falling back, shipped values loaded from
  `config.yaml` (test binaries load Go defaults, so the shipped-value test reads the file).
- Combat: the existing pins move (`damage_pipeline_test.go`, `regression_test.go`
  `TestRegression_MitigationCapEnforced`, `reflect_damage_test.go`, `counter_tier_test.go`,
  the integration and parity tests); each is updated to the curve, not deleted.
- Crits: a crit on a target below the knee is unchanged; above it, reduced by the overflow;
  melee and spell paths agree.
- `PowerScore` and `status` read the effective value.
- Python balance models under `tools/balance/` that assume a 0.75 cap get a note in the plan;
  they are not part of the game.
- Playtest: an invested tank against The Sentinel and the North Road bandits, a crit-heavy
  build against a high-mitigation mob, and the crash site interior (the owner expects it may
  get harder). File findings as issues; retune by config.

## Out of scope

Retuning gear, wards, mutations or mobs; per-source curves; changing armor piercing.
