# Contest curve bounding

**Date:** 2026-08-14 (rev 3)
**Status:** design approved. Section 7 records the resolutions; four small items
remain genuinely open at the end of it.
**Arc:** unified contest resolution, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`

This is the U6 design. Two content changes are already landed; two code changes
follow. **The compression knee proposed in rev 1 is dropped** -- see section 3.

---

## 1. Problem

Combat outcomes depend only on the score RATIO. Because sigma is proportional to
the attacker's score, the sizes cancel:

```
mu_z = (a - d) / (0.15 * a * sqrt(2)) = (1 - d/a) / 0.212
```

Gaussian tails fall off as exp(-z^2/2), so everything resolves inside roughly
plus or minus 40% of parity. Simulated against the real Meirok, expected damage
per swing today is **0.00** against The Core Guardian: the trash-to-apex band is
infinite.

**Variance is provably the wrong lever.** For Meirok (455) against a pool-5000
mob, widening sigma to 0.30 x attacker still gives 0.0000%. Even sigma = 1.0 x
max(a, d), a pure-noise roll, only reaches 29.5%. `RollSpread` cannot fix this.

---

## 2. Step 0, already landed

### 2a. Every NPC skill reset to 1 -- commit `1cbadaa20`

The arc's model rests on "players scale through skill, mobs through stats", and
a merged comment at `combat_helpers.go:486-491` asserted that every mob is
combat skill 1 because no mob YAML defines a skills block.

**That was false.** 24 mob files defined a combat skill, topping out at
`weapon-combat: 100` on the Arena Champion. The claim survived because the block
is nested under `character:`, so a column-anchored grep for `^skills:` returns
nothing and reads as proof.

Two consequences nothing had modelled:

- `calcCritThreshold` computes `skillDiff = player - mob` with **no upper
  clamp**, so against a skill-100 mob a veteran's crit bar becomes
  `2.0 + 99*0.05 = 6.95` rather than the 1.5 floor everything else assumes.
- Mob defence gains `skill * SkillWeight`. At the shipped 5.0 a `weapon-combat:
  30` mob carried **+150** parry and block that no curve included.

76 files, 105 values, all plain value changes. The premise is now true by
construction rather than by accident. `spellbook:` entries are a separate block
holding per-spell proficiency and are deliberately untouched.

### 2b. The two static bosses restatted -- commit `2d2350db4`

**Instance mobs scale as `pool = goldPaid * template_multiplier`**
(`rooms/instances.go:262-276`, capped by `InstanceStatPoolCap`, default 50000).
The Planar Oasis royalty carry multiplier 4, so a 300g king is pool 1200 and a
100g king is pool 400. **Gold paid is the game's real difficulty dial**, and it
gives every static mob a gold equivalent.

Measured against it, the two hand-authored bosses sat far off the curve. Parity
for Meirok is about a 364g king (pool ~1456):

| | was | = gold | now | = gold |
|---|---:|---:|---:|---:|
| The Sentinel | 2400 | 600g | **2000** | 500g |
| The Core Guardian | 5000 | 1250g | **2800** | 700g |

The Core Guardian is intended as a party boss and remains one.

---

## 3. Why the knee was dropped

Rev 1 proposed compressing contest scores above a knee at `K=350, slope=0.05`.
Adversarial review killed it on four counts, any one of which is sufficient.

**3a. K collided with the skill cap.** `100 (baseline stat) + 50 (SkillSoftCap)
x 5.0 (SkillWeight) = 350`. K was numerically identical to "a baseline character
at the skill soft cap" -- not above the player range, but exactly on the design's
intended endgame. Meirok's real melee score is **455** (`Dex 110 +
weapon-combat 69 x 5`), not the 330 rev 1 modelled. His ranks 49-69, twenty-one
ranks of grinding, would have bought **5.25 contest points**.

**3b. It deleted a trade the same document called settled.** Meirok's defences
are dodge 375 / parry 449 / block 523, a 148-point spread expressing his build.
Post-knee they became 351 / 355 / 359, a spread of 7. The best shield in the game
would have gone from +31.5 contest points to +4.9, and past roughly 372 bare
-handed dodge overtakes it.

**3c. It flattened the gold dial**, which is the decisive one. Because a veteran
sits further above K than the mobs do, he compresses harder than they do:

| gold | pool | today | with knee |
|---:|---:|---:|---:|
| 300 | 1200 | 77.1% | 51.9% |
| 500 | 2000 | 5.7% | 46.0% |
| 750 | 3000 | 0.0% | 38.7% |
| 1000 | 4000 | 0.0% | 31.8% |

A 300g run and a 750g run converge to within 13 points while `CalcLootBudget`
keeps paying `sqrt(gold)`. That makes "always pay the maximum" correct and
deletes the middle of the game's only player-facing difficulty control.

**3d. Every situational multiplier would have been worth 5% of face value above
K** -- prone, grapple control, darkness, Rally, and the whole premise of U7
(encumbrance-driven defence costs) and U8 (exhausted actors lose the skill term).

Restatting two files achieves what the knee was for without any of this.

---

## 4. Change 1: one floor for every contest in the game

**`ContestFloor: 0.125`, a single symmetric value, replacing eight knobs.**

A symmetric floor `F` yields the bound `[F, 1-F]`, so one number gives
**12.5% / 87.5%**: hopelessly overmatched you still succeed one attempt in
eight, hopelessly overmatching you are still stopped one in eight.

| Deleted | Was |
|---|---|
| `MinAttackHitChance`, `MinDefenseChance` | 0.15 / 0.15, melee |
| `MinSpellHitChance`, `MinSpellResistChance` | 0.05 / 0.05 |
| `MinManeuverHitChance`, `MinManeuverResistChance` | 0.05 / 0.05 |
| `MinContestSuccessChance`, `MinContestResistChance` | 0.05 / 0.05 |

Also deleted: `ManeuverFloors`, `SpellFloors`, `ContestFloors`,
`RunWithManeuverFloors`, `RunWithSpellFloors`, `RunWithGlobalFloors` -- the whole
of `internal/combat/contest_floors.go`, whose reason for existing was keeping
three pairs straight. `floor_pair_guard_test.go` polices which site uses which
pair and goes with them. This also resolves the split flagged at
`contest_floors.go:141`, where the global pair reads `dice.ContestFloors()` while
the other two read config -- a comment that already says "U6 owns collapsing the
two routes".

The old per-channel values encoded the COST OF A SINGLE FAILURE. **That
reasoning is deliberately discarded in favour of one rule.** Out-of-combat
exposure is accepted explicitly: a novice picks any pocket one try in eight, a
blind guard spots a master thief one in eight. Owner's call: most of the time
the reckless attempt fails and is punished.

---

## 5. Change 2: floors resolve before crits, and a defence returns a multiplier

### 5a. The ordering defect

`MinAttackHitChance` and `MinDefenseChance` sit in step 3 of
`resolveDefenseOutcomeCore`, after crit resolution has already returned. Against
The Core Guardian the defence crits on 96.8% of swings and returns at step 2, so
the attack floor only ever evaluates on the remainder. Both floors are dead code
in exactly the matchups they were written for.

This defect is **physical-pipeline-only** (melee and ranged, which share
`combat.go:489`). `contest.RunWithFloors` already floors first and stamps
`Margin` as plus or minus 1, which `ContestCrit` normalises to a near-zero z so a
floored outcome cannot crit. The spell and maneuver paths have always been
correct; this change makes the physical path agree with them.

### 5b. The defence multiplier

Roadmap line 37 and section 4 of `2026-08-12-unified-contest-resolution-design.md`,
kept rather than dropped. Today "miss" and "deflect" are unrelated mechanisms:
`runBestOfAllDefense` produces a miss while `TrySpellDeflection` and
`TryStoicResolve` separately produce a flat 0.5 or 0.0.

- A bare defensive win mitigates **50%**.
- Mitigation rises linearly with the defender's margin, reaching **100%** at the
  crit threshold.
- A **defensive crit** fully negates, fires the counterattack, and is the only
  thing that can answer a crit attack.

The full lattice is section 4.2 of the original design doc and is unchanged.

### 5c. The resulting order

```
1. fumbles          unchanged, absolute (the one deliberate bypass)
2. organic winner   margin > 0 means the defence won (defence-positive)
3. floor flip       either side flips on ContestFloor; mark FLOORED
4. crit             only when NOT floored, on the side that actually won
5. damage mapping   attack crit  -> crit magnitude, bypasses item mitigation
                    attack won   -> full damage after item mitigation
                    defence won  -> margin-scaled 50-100% mitigated
                    defence crit -> zero damage plus counterattack
6. crit floors      the 1% pair -- see 7c, they cannot stay unchanged
```

Floored outcomes map through the existing plus/minus 1 sentinel: a floored hit
reads as an attack that barely won and deals full damage; a floored save reads
as a bare defensive win and mitigates 50%. Neither can crit.

---

## 6. Expected outcomes

Monte Carlo of the exact resolution order, 200k swings per cell, seed 20260814,
`C:/tmp/u6sim/final3.py`. The baseline column runs the **shipped** branch order
(crits first, then floors); the after column runs 5c. Mobs are skill 1 per step
0; bosses are at their new pools. `conn` = swings dealing any damage, `part` =
partially deflected, `defcrit` = defensive crit, `E` = expected damage relative
to a full non-crit hit.

**Meirok** (Dex 110, Str 136, weapon-combat 69, attack score 455, crit mult 5.45):

| Mob | conn | crit | part | defcrit | E |
|---|---|---|---|---|---|
| A Wheeling Hawk | 97.7 to 97.7 | 97.3 to 84.9 | 12.4 | 0.0 to 0.0 | 5.31 to 4.69 |
| Blind Stalker | 97.4 to 97.7 | 95.4 to 83.4 | 12.2 | 0.0 to 0.0 | 5.22 to 4.63 |
| Hull Warden | 96.9 to 97.7 | 92.5 to 81.0 | 12.3 | 0.0 to 0.0 | 5.09 to 4.52 |
| The Old White | 91.0 to 97.7 | 60.7 to 53.1 | 13.4 | 0.0 to 0.0 | 3.61 to 3.27 |
| The Sentinel | 9.6 to 61.1 | 0.0 to 0.0 | 47.5 | 41.9 to 36.7 | 0.10 to **0.22** |
| The Core Guardian | 0.2 to 12.9 | 0.0 to 0.0 | 0.7 | 96.8 to 84.8 | 0.00 to **0.12** |

Band: **infinite to 39.1x**.

**New player** (attack 125): band infinite to 13.9x. Hull Warden E 0.32 to 0.51,
The Old White 0.00 to 0.12.
**Mid player** (attack 215): band infinite to 21.4x. The Old White E 0.24 to
0.41, The Sentinel 0.00 to 0.12.

Reading:

- **The dogwalk survives.** Meirok still deals 4.69x a full hit to trash. The
  drop from 5.31 is the 87.5% ceiling biting his crit rate, which is the point of
  having a ceiling.
- **The Sentinel becomes a real fight**: 9.6% of swings connecting becomes 61.1%,
  and expected damage more than doubles. Half of that is the restat, half the
  multiplier.
- **The Core Guardian stays a party boss**, chippable at 12.9% rather than
  helpless at 0.2%.
- **The mid band comes alive.** A new player against Hull Warden and a mid player
  against The Old White both gain roughly 60% expected damage, entirely through
  the `part` column.

**Two caveats on these numbers, both from review:**

1. `defcrit` is the **per-swing** defensive crit rate, not the counterattack
   frequency. `combat.go:459-461` resets `Crit`/`Fumble`/`DoubleFumble` per swing
   but NOT `ParryCritDetected`/`DodgeCritDetected`/`BlockCritDetected`, which are
   round-level and read once at `combat_shared_helpers.go:219/252/295`. Actual
   counterattack frequency is `1 - (1-p)^swings`, capped at one of each type per
   round.
2. `E` is per swing, not rounds-to-kill. Target HP also scales with `statpool`
   (`HealthMax = 5 + Vit*3 + Str*1`), roughly 186 HP for the Hawk against ~4100
   for the Core Guardian. Rounds-to-kill spans far more than 39x. Quote E for
   damage comparisons only.

---

## 7. Resolved 2026-08-14 -- most of these were already settled in the arc

Rev 2 raised nine "open questions". Owner review found that six of them either
had answers already recorded in the roadmap or were me re-fragmenting a design
whose stated purpose is unification. Recorded here so they are not re-opened a
third time.

**7.1 One resolver, all channels. NOT a per-channel design.** The roadmap's
defence-set table already specifies this:

| Attack type | Applicable defences | N |
|---|---|---|
| Melee | dodge, parry, block | 3 |
| Ranged | dodge, block | 2 |
| Spell, physical | dodge, block | 2 |
| Spell, mental | **quell** (`Wil + spellcasting x5`) | 1 |
| Taunt / social | **defy** (`Wil + rhetoric x5`) | 1 |

Same opposed roll, same resolution order, same floor, same partial-damage
mapping. N=1 is a defence set of size one, not a different mechanism.
`resolveDefenseOutcomeCore` therefore stops being melee-specific and takes the
defence set as a parameter; every channel calls it.

`TrySpellDeflection`'s second roll on the defender's Perception **is** the
legacy fragmentation U6 deletes. Losing Perception as a spell-defence stat is
the intended outcome, not a cost -- quell replaces it. An earlier draft argued
for preserving it and was wrong.

**7.2 Maneuvers join the partial mechanism.** Bash, trip, kick and the other
`skill_moves.go` sites use the same floor and the same partial tier. **A partial
trip deals damage without the knockdown.** The status effect is the part that
stays binary; the damage is not.

**7.3 Concentration is already specified in U10 and needs no exception.**
Roadmap U10 records a **trigger threshold of ~10% of a pool -- below that, no
roll at all**, with disruption difficulties of prone 250, grappled 300, and
damage `damagePct x 10`. So the `damage > 0` gate at
`combat_shared_helpers.go:108` becomes a 10%-of-pool gate and the "casters roll
on 96% of incoming swings" problem never arises.

On the floor: concentration is `Wil + spellcasting x5` against a **static
difficulty**. That is category B in the arc's own taxonomy
(`UNIFIED_RESOLUTION_ROADMAP.md:61`), not an opposed contest. **`ContestFloor`
governs opposed contests only.** Static-difficulty rolls are a separate category
the arc drew before U1, so no exception is needed and none is added.

**7.4 Crafters interrupted by any damage: accepted as intended.**

**7.5 The "helpless target loses its save" case is a grapple artifact, not a
floor problem.** `filterDefensesForThirdParty` hardcodes "attacking a grappled
third party, only block applies", and zero defences remain only when the target
has no shield. That is a defence-set rule written as a filter function instead
of data -- exactly what 7.1's table replaces. Making the set data-driven
dissolves it.

**7.6 No disable knob for Change 2, and none is added.** The arc has a hard
no-deploy-until-playtested gate, so this cannot reach production untested. A
`DefenseBareMitigation` shipping at 1.0 would have disabled the multiplier
exactly; it is unnecessary given the gate.

**7.7 `res.Hit` goes broad.** Reflect, lifesteal and on-hit procs all compute
from `res.DamageToTarget`, the damage ACTUALLY DEALT after mitigation
(`NewRound_DoCombat_unified.go:384`, `:403`, `:157`). A partially deflected
swing therefore returns proportionally less. The `reflect_damage.go:6-15`
incident does not recur: there, hit rate rose 4x while every hit still dealt
full damage. Verified, not assumed.

**7.8 The floor stays PER CONTEST, not per action.** Sneak rolls once per
observer and flee once per blocker, so the ceiling compounds against the actor.
That is correct by design: fleeing a pack should be harder than fleeing one mob,
and sneaking past eight observers harder than past one.

**7.9 Crit floor denominators.** Attack floor applies to **swings that won the
contest**. Defence floor triggers on **the defence winning the contest**, not on
"the attack missed". Floor-driven ripostes disappearing is accepted and is
arguably more realistic: fighting for your life against a superior opponent, you
are unlikely to counterattack.

### Still genuinely open

- **The two hidden-detection sites in `actions/search.go`** answer the same
  question as `usercommands/go.go`'s opposed contest but with a flat 135
  threshold that ignores the hider's score entirely, so skill decides the
  outcome on one path and is ignored on the other. Marked **UNASSIGNED** at
  `UNIFIED_RESOLUTION_ROADMAP.md:123`; still needs a chunk. The rest of
  `search.go`, plus `track.go` and `forager`, **stay static by decision**
  (roadmap:145-160, clarified 2026-08-13): there is genuinely no opponent and
  inventing one is worse.
- **Integer damage floors diverge**: melee floors at 0
  (`calcHitDamage:1150,1156`), spells and taunt at 1
  (`CritOrMitigatedDamage:77-79`, "a hit that lands must do something; 0 reads
  to the player as a bug"). Under partial deflection a heavily mitigated melee
  swing can round to 0. Melee should match spells.
- **`on_block` procs receive a literal `0` damage argument**
  (`NewRound_DoCombat_unified.go:160-170`) on the reasoning that a block deals
  none. The multiplier falsifies that premise.
- **`ContestFloor: 0` must fail validation.** A Go test binary never loads
  `config.yaml`, so a `[0, 0.5)` check would leave the floor at 0 and silently
  switch it off in every test.

## 8. Corrections to rev 1

Recorded so they are not reintroduced.

- "Every mob has combat skill 1" was **false**; it is now true by construction.
  See 2a.
- "Shipped pools run 20 to 5000" was **false**: five mobs ship at 0, and real
  content ships 1, 2, 3, 4 and up. 20 was the lowest *modelled* mob.
- "Melee contests Dexterity, quell contests Willpower" is true only for the
  **mental** line. `spellDefenseValue` (`spell_resolution.go:995-1006`) returns
  Dexterity for `target_defense_type: physical`, and the harm-spell book splits
  **11 physical / 9 mental**. The remaining 13 `none` entries are summon and
  conjure spells with no target to defend, which is correct for them. The "spells
  are the intended counterplay to a bruiser" conclusion therefore holds for nine
  spells, not thirty-three, and is **no longer marked settled**.
- The veteran profile was modelled at stat 130 / skills 40 / score 330. The real
  veteran is Dex 110, weapon-combat 69, score **455**.
- "Expected damage is exactly 0.00" and a table showing 0.15 both appeared in
  rev 1. The 0.00 came from a closed-form model with floors off; the corrected
  simulation with the shipped branch order gives 0.00, so the claim stands but
  the table that contradicted it was the wrong one.

---

## 9. Implementation

**Files**

- `internal/configs/config.balance.go` -- add `ContestFloor`; delete the eight
  floor knobs and `SpellAvoidanceDamageMultiplier` /
  `RhetoricAvoidanceDamageMultiplier`.
- `internal/configs/config.balance.misc.go` (all eight floors),
  `config.balance.combat.go` (`RhetoricAvoidanceDamageMultiplier`),
  `config.balance.spells.go` (`SpellAvoidanceDamageMultiplier`) -- defaults and
  validation. Three files, not the two rev 1 named.
- `internal/contest/contest.go` -- apply the single floor internally.
- `internal/combat/contest_floors.go` -- delete.
- `internal/combat/avoidance.go` -- delete, per its own `:52` note.
- `internal/combat/combat_helpers.go` -- restructure `resolveDefenseOutcomeCore`;
  add a `Floored` field to `bestDefenseResult` and stop discarding
  `contest.Result` in `runBestOfAllDefense:686-720`.
- `internal/combat/crit_floor.go` -- see 7c.
- `internal/dice/contest_floors.go` -- owns `ContestFloors`/`SetContestFloors`
  plus the deprecated `OpposedRollStat*`. Retire the package-var route.
- `main.go:197-199` -- seeds `dice.SetContestFloors` from two deleted knobs.
  **Compile break on the first edit if missed.**
- `internal/actions/combat_taunt.go:209` -- the only `TryStoicResolve` caller.
- `internal/hooks/spell_resolution.go` -- five `TrySpellDeflection` callers, plus
  the "deflect"/"resolve" player text.
- `internal/hooks/NewRound_DoCombat_helpers.go` -- darkness message selection
  branches on `DefenseUsed` above `Hit`; also `cancelCraftOrSalvageOnDamage:734`.
- `internal/hooks/NewRound_DoCombat_unified.go` -- `on_block` proc damage
  argument, and the mutation-drift gates at `:141`/`:144`/`:168`.
- `contest_floor_guard_test.go` (repo root) -- its premise inverts when
  `contest.Run` becomes floored. `floor_pair_guard_test.go` -- delete.
- `_datafiles/config.yaml` -- one knob in, ten out.
- `context.md` for `internal/contest`, `internal/combat`, `internal/dice`.

**Tests that will break** (verified, not predicted): `configs/smoke_test.go:36-43`
(compile), `combat/contest_floors_test.go`, `combat/global_floors_test.go`,
`combat/contest_sign_test.go`, `combat/hitroll_test.go` (12 tests asserting
`res.hit` as a bool), `combat/crit_floor_test.go`, `dice/contest_floors_test.go`,
`combat/integration_combat_test.go`, `actions/contest_sign_taunt_test.go`,
`behaviortree/actions_skullduggery_test.go:103-105` (pins floors via
`dice.SetContestFloors`), `contest/contest_test.go`.

`TestRegression_DefenseFloorAlwaysApplies` (`combat/regression_test.go:83`) will
**not** break -- it only asserts `best.margin` is not -Inf and `defenseType` is
non-empty, both satisfied by the sentinel. That is itself a gap: nothing
regression-tests the melee floor after this change.

**Contests that bypass `contest.Run` entirely** and inherit nothing:
`actions/search.go` (six sites), `actions/track.go:124`,
`forager/forage_core.go:129`, `characters/skills.go:84` (`AttemptRecovery`),
`combat/skill_moves.go:87` (knockdown). Decide whether they join or stay out.

**Verification.** A boot test proves nothing about how this feels. Per the
project's content gate this needs an adversarial playtest pass before it is
called done, and it is inside the arc's no-deploy window regardless. Step 0
changed 78 data files and has not yet been boot-tested.

---

## 10. Method note

Figures from a Monte Carlo (200k swings per cell, seed 20260814) of the exact
order in `resolveDefenseOutcomeCore`, including fumbles, crit short-circuits,
best-of-three defence with independent rolls, and the floors. World inputs are
real: `statpool:` from `_datafiles/world/dogmud/mobs/`, the archetype split from
`internal/mobs/mobs.go:527-560`, defence formulas from
`Character.GetDefenseScore`, the melee attack score from `combat_helpers.go:408`
(Dexterity plus combat skill times `SkillWeight`, **not** Strength), and Meirok's
stats and skills from `_datafiles/world/dogmud/users/3.yaml`.
