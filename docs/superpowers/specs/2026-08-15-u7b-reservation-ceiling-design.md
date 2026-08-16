# U7b: the reservation ceiling, and the companion power model

**Created:** 2026-08-15
**Slice:** U7b of the unified-contest arc. Runs immediately after U7
(merged, PR #48, `1f6251fb6`) and **must precede U8**.
**Roadmap:** [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md), section "U7b"
**Research inputs:** owner decisions 2026-08-15; two research passes recorded in
this document's Evidence section.

---

## 1. Why this slice exists

U8 introduces a skill-strip on insufficient resource. That turns an
over-reserved pool from a cosmetic oddity into a permanent, crippling penalty.
Today there is **no cap on reservation at all** for two of the three pools, and
the conviction "cap" is inert. So U8 cannot ship safely until this does.

Two independent problems are being solved together, because fixing either alone
produces a worse game than fixing both:

1. **Reservation is unbounded.** 96% health reservation is reachable with
   shipped gear and no mutation; 120% with one rank of Extra Arms.
2. **Companion power and price are incoherent.** Conjures cost three to twelve
   times what raises cost per point of pool fielded, for the same reservation.
   Corpse averaging destroys pet-tier differentiation as corpses grow. Three
   companions silently discard half their combat rounds.

---

## 2. Decisions (owner, 2026-08-15)

All of the following are **settled**. They are recorded here so the plan does
not relitigate them.

| # | Decision |
|---|---|
| D1 | **Cap total reservation at 66%** of a pool's max. |
| D2 | The cap applies **per pool**, to all three (health, stamina, conviction). |
| D3 | The **breaching action is refused**, not allowed to succeed and clamp. |
| D4 | A character already over the cap is **grandfathered**: refuse additions only, never force a dismissal. |
| D5 | The cap **subsumes `CanAffordCompanion`**, which is removed rather than kept alongside. |
| D6 | Companion stat pools use a **new formula** (section 3). |
| D7 | **Pet multipliers** replace base pools (section 3.2). |
| D8 | **Cast costs drop to a 30-50 band** (section 3.3). |
| D9 | **Reservation scales with the pet multiplier** (section 3.4). |
| D10 | **Both inverse-skill riders are IN** (section 4). |
| D11 | Companion reserve snapshots are **recomputed on login**. |
| D12 | `summon_base_pool` is **replaced** by `summon_pet_multiplier`; `summon_scaling_divisor` is **deleted**. |
| D13 | Behaviour fixes for fire, water, skeleton and hive swarm are **in scope** (section 5). |
| D14 | Enchant tier-up **skips** when it would breach the cap, and says why. |
| D15 | A **reservation readout** ships, in descriptive bands only. No numbers. |
| D16 | Dismissing a companion destroying its gear is **accepted as-is**. No change. |

### 2.1 Accepted outcomes, surfaced during execution. NOT defects.

Both emerged from the recomputed calibration in Task 4 and were put to the
owner. Both were **accepted with reasons**. Do not re-file either as a bug, and
do not "fix" them in a later slice without asking first.

**A novice summoner cannot field a top-tier conjure at all.** At manifestation 5
a magma elemental reserves more than the whole 66% ceiling allows, so it is
refused rather than merely expensive. **Owner: fine.** The higher summons are
already hard-gated by spell discovery, and discovering them means grinding
manifestation, so a character who can cast magma has the rank to hold it. The
gate is redundant with an existing gate, not a new wall.

**At the top end, pet tier stops costing companion slots.** At manifestation 55
with a rank-4 Manifester mutation, golem and magma both bottom out at the soft
count backstop of 5, so the reduction caps flatten the tier differentiation the
multiplier exists to create. **Owner: fine.** Reaching that state takes a long
time and the mutation ranks come slowly, so it describes an endgame character
who has earned it. `CompanionReserveTotalCap` (0.79) is the lever if that ever
changes.

---

## 3. The companion power model

### 3.1 The formula

Let `B = Charisma + (manifestation skill x 5)`.

```
conjured (no corpse):   pool = B x petMultiplier
raised   (corpse):      pool = ((B + corpsePool) / 2) x petMultiplier
```

This replaces the formula used for **companions**, which multiplied a per-spell
base pool by
`1 + cha/ManifestStatScaleChaFactor + manifestSkill x ManifestStatScaleSkillFactor`.

**`CalcCompanionStatPool` must be RENAMED AND KEPT, not deleted.** It has a
second production caller, `internal/behaviortree/actions_mob.go:78`
(`actSummonCompanion`), which spawns **authored boss adds**: the Sentinel at
`base_pool: 300`, the Core Guardian and Warden Prime at 50, Old Edrin at 60.
Routing those through the companion formula would nerf the Sentinel's adds by
roughly five times. The old function keeps its behaviour under a name that says
what it is now for, and the `ManifestStatScale*` knobs stay alive to serve it.

**Why the change.** Under the old shape the pet's base pool multiplied the
caster's power, and the corpse was averaged in afterwards. That made the
corpse's share grow until it swamped the pet choice: at a 1000-pool corpse a
skeleton fielded 587 and a golem 675, so five times the price bought 15% more
pet. Applying the multiplier **after** the average keeps every pet tier
proportionally separated at every corpse size.

**Known consequence, accepted:** mid-level summoners lose roughly 20%, because
`manifestation x 5` is flat where the old term multiplied a base pool. High-skill
summoners gain slightly. If newer summoners feel too weak in playtest, the lever
is the manifestation coefficient (5) or a flat constant added to `B`, not the
multipliers.

### 3.2 Pet multipliers

Conjures range 0.75 to 1.25 deliberately: they have no corpse to scale from, so
they need a higher ceiling to stay competitive before large corpses are
available.

| Family | Pet | Multiplier | Role |
|---|---|---|---|
| Conjure | magma | **1.25** | tank_taunter, 25% magical reflect |
| Conjure | earth | **1.05** | tank_taunter |
| Conjure | fire | **1.00** | melee_self_buff, 25% magical + 12% physical reflect |
| Conjure | air | **0.90** | pure_caster |
| Conjure | water | **0.75** | generic_fighter (after fix) |
| Raise | golem | **1.00** | tank_taunter |
| Raise | vampire | **0.83** | melee_self_buff |
| Raise | spectre | **0.75** | pure_caster |
| Raise | zombie | **0.67** | generic_fighter |
| Raise | wraith | **0.58** | pure_caster |
| Raise | skeleton | **0.50** | generic_fighter (after fix) |
| Summon | steppe spirit | **0.75** | generic_fighter, quest-gated, species damage x0.65 |
| Summon | hive swarm | **0.30** | generic_fighter (after fix) |

Air sits above water and remains the caster. Steppe spirit is nerfed from an
effective 1.03 to 0.75: it is quest-gated, which justifies it being good, not
best-in-game for a twelfth of the price.

### 3.3 Cast costs

Cast cost is a **one-time toll**. Companions persist across logout and reboot
with full state, so the cast price is amortised over the companion's entire
life and is the wrong place to carry differentiation. It becomes a low entry
gate; **reservation carries the real cost.**

| Pet | Cast CP | | Pet | Cast CP |
|---|---|---|---|---|
| magma | 50 | | golem | 50 |
| earth | 45 | | vampire | 45 |
| fire | 45 | | spectre | 40 |
| air | 40 | | zombie | 35 |
| water | 30 | | wraith | 35 |
| steppe spirit | 35 | | skeleton | 30 |
| hive swarm | 30 | | | |

This removes a self-excluding trap: `conjure-magma` at 450 was 89% of a maxed
summoner's entire conviction pool, uncastable outright for a mid-level one, and
**impossible for anyone already fielding companions**, because their reservation
had already reduced usable conviction below the cast price.

### 3.4 Reservation

```
baseReserve = CompanionReserveDefault (280) x petMultiplier
```

**Correction, 2026-08-15:** an earlier draft of this line called the knob
`CompanionReserveBase`. No such knob exists. The real field is
`CompanionReserveDefault`, Go default 280, **absent from `config.yaml`** and so
running on that default.

Replacing the current two flat tiers (280 and 352) shared across both families.
The ongoing budget now tracks pet power, which is what makes the cap a real
choice rather than a flat companion count.

---

## 4. The inverse-skill riders (D10, both IN)

### 4.1 Companion reserve

Compose `costs.SkillCostMultiplier(manifestation)` **on top of** the existing
manifestation reduction in `CalcCompanionReserve`. **Compose, do not replace.**

Replacing would be strictly worse than today at every rank: the U7 curve bottoms
at 0.40 while the existing reduction already reaches 0.45 at manifestation 55
and 0.21 with the Manifester mutation. A replacement would make companions
dearer for everyone, the opposite of intent.

**Known consequence, accepted:** composed, the curve double-counts manifestation
below rank 55 and is a 10% *penalty* at rank 0, only becoming a discount past
rank 25. This matches the settled decision on the item side and is deliberate.

### 4.2 Item reserve on the wearer's enchanting rank

Scale `GetTierReservePct`'s result by `costs.SkillCostMultiplier(enchanting)`.
A tier-4 8% enchant becomes 8.8% at enchanting 0, 8.0% at 25, 6.1% at 54, and
3.2% at 100.

The penalty half applies, consistent with 4.1.

---

## 5. Behaviour fixes in scope (D13)

These are not polish. Each one makes a multiplier a lie until fixed.

| Mob | Problem | Fix |
|---|---|---|
| 313 fire elemental | `melee_self_buff` archetype whose spellbook contains **no `self_offense` spell**, so its first branch can never match. It casts one ward at combat start, then returns Failure forever. | Give it vampire's setup: keep `melee_self_buff`, add `conviction-surge` (the game's only `self_offense` spell) to its spellbook. |
| 310 water elemental | **No `behavior_archetype` at all.** Falls to legacy AI, where an empty `combatcommands` entry returns "I acted" and consumes the round. Discards ~40% of rounds. | `behavior_archetype: generic_fighter` (the bandit DPS archetype). |
| 300 skeleton | Same, plus two pure-flavour emotes that also consume rounds. ~40% wasted. | `generic_fighter`. |
| 111 hive swarm | Same. ~50% wasted. | `generic_fighter`. |

Also: mobs 312 (air) and 313 (fire) carry spellbooks but **no `spellcasting`
skill entry**, unlike 302/303/304 which all set `spellcasting: 1`. They cast at
skill 0. Set `spellcasting: 1` on both.

---

## 6. Enforcement surface

The cap cannot be enforced only on the adding action.

### 6.1 Actions to refuse

| Path | Refusal channel |
|---|---|
| Equipping / wielding (`wear`, `wield`, `hold` all alias to `equip`) | `Character.Wear` already returns `failureReason`. `EquipItem` is the shared seam. |
| Enchanting, pre-flight | `resolveEnchantSlot` already returns an error message before the multi-round activity starts. **Preferred gate.** |
| Enchanting, completion | Already re-checks the target is equipped and refunds materials otherwise. |
| Summon / conjure / raise | `resolveCompanionSummon` refuses before consuming component or corpse. |
| Charm | `resolveCharmSpell`. |
| Brood-mother auto-spawn | **Currently bypasses every gate.** Must be added. |
| Chrysifier homunculus auto-spawn | **Currently bypasses every gate.** Must be added. |
| Companion reserve backfill on login | Stamps a reserve with no budget check. Must respect the cap. |

**Message-quality gaps to close while here:** `internal/actions/sell.go`
discards the failure reason; `internal/mobcommands/equip.go` discards it
entirely; `gearup` infers success by diffing the equipment set and emits nothing
on refusal, so a mob-side refusal is currently invisible on every mob path.

### 6.2 Passive breaches, with no action to refuse

- **Enchant tier-up (D14).** Rolls in combat and **doubles** the reserve
  fraction at low tiers. A character at 64% can cross the cap having done
  nothing. Skip the tier-up when it would breach, and tell the player.
- `ConditionEnchantWithdrawal` shrinks the pool max *after* the reservation
  clamp, raising the share without touching the numerator.
- `BodyConvictionScale` deepens as body-pole mutations accrue.
- `MigrateEnchantments` on login re-applies definitions, so retuning any tier's
  `reserve_pct` grows existing reservations at load.

For all four: the cap must be evaluated **as a clamp on effect**, not only as a
gate on actions. Grandfathering (D4) means these never force a dismissal; they
simply cannot make things worse once over.

---

## 7. Migration

- **Recompute companion reserve snapshots on login (D11).** `conviction_reserve`
  is frozen at summon time, so existing companions would otherwise keep their
  old numbers indefinitely. Recomputing covers returning veterans.
- **No forced dismissals (D4).**
- **Expected outcome for the only affected character.** Meirok is at 78.2%
  conviction today (351 of 503). Under D9 his golems rebase from 352 to 280,
  reserving 126 each instead of 158. With the Shadowweave ward that is about
  292 of 503, **roughly 58%**, under the cap with both golems kept and before
  either rider is applied. No migration pain.

---

## 8. Player-facing surface

- **Reservation readout (D15), descriptive bands only.** Reuse
  `reserveShareBand` (`internal/usercommands/assess.go`), whose existing edges
  are 0.15 / 0.30 / 0.50 / 0.75. Add it to the `status` sheet alongside
  encumbrance and toxicity.
- **No hard numbers anywhere.** Encumbrance already complies: `status` renders
  `encumbranceQuality` (word plus colour) and `inventory` renders
  `[{{ .EncumbranceLabel }}]`. Match that.
- **Refusal messages must name reservation as the cause**, not exhaustion or a
  generic failure. `stand`'s existing disclosure is the pattern to copy.
- Helpfiles: whatever documents companions, enchanting and reservation must
  reflect the cap. 80-char wrap, ESL-clear, no en/em dashes.

---

## 9. Deliberately NOT in this slice

- **U8's skill-strip.** This slice is its prerequisite, not its delivery.
- **Repairing the poisoned `EnchantBaseline` on Meirok's arena tower shield**
  (`return_damage: 165` against a correct 25). The stacking bug is historical,
  not live: `EnchantBaseline.RestoreInto` now runs before tier effects, so newly
  enchanted items are clean. That one item's baseline was captured with the
  accumulation already baked in. The owner intends to destroy it by dismissing
  the golem holding it. **If any other item is found in the same state, that is
  a separate fix.**
- **Companion area spells hitting non-owner players.** The filter excludes only
  `charmedByUserId`, so a companion's `sparks` hits party members. Real, narrow,
  and not this slice.
- **`symbiotic-bond`'s decorative value.** `companion_empowerment: 0.15` is read
  only as a `> 0` gate; the actual effect is a flat buff. Contradicts the
  project's multiplier-over-flat convention. Separate.
- **Retuning `CompanionSoftCap`.** The conviction budget, not the count, is the
  real limit; the count backstop can be revisited once the cap is live.

---

## 10. Evidence and traps

Everything below was verified against the code on 2026-08-15. Each is a trap
that would otherwise be rediscovered mid-build.

1. **`GetPoolReservation` has no `IsMob` gate.** Companions reserve, on prod,
   today: Meirok's golems wear enchanted gear reserving their own health and
   conviction. Any design that assumes reservation is player-only is wrong.
2. **Only FOUR raw-max reads need moving, not six.** The two behaviour-tree
   packmate sites go through `FindPackmatesInRoom`, which **skips charmed
   mobs**, and every companion is charmed. Genuinely affected, all self-side:
   the three reads in `internal/combat/ai.go` and `hpPercent` in
   `actions_archer.go`. Correct the other two files' comments instead.
3. **`ManifestStatScaleChaFactor` ships at 150, not 200.** The `200` in
   `CalcCompanionStatPool`'s doc comment and fallback is stale and unreachable.
   This is moot once the formula is replaced, but do not use it to sanity-check
   the new numbers against the old.
4. **`CanAffordCompanion` is a 100% conviction-only cap, not a partial budget.**
   `CompanionCastingFloorPct` defaults to 0.0 and is absent from `config.yaml`,
   so the check reduces to "must not exceed the max". Two auto-spawn paths never
   call it at all.
5. **`summon_scaling_divisor` has never been read.** Declared in the struct,
   present in all thirteen summon YAMLs, zero production readers.
6. **Corpse pools are far smaller than intuition suggests.** Across 639 mob
   templates, `statpool` has median 34, p75 66, p90 150, p99 550. Instance mobs
   scale as `goldPaid x templateStatPool` (template pools are multipliers:
   1 trash, 2 tough, 3 boss), capped at 50000.
7. **`return_damage` is not in the affix pool**, so it cannot come from
   gold-scaled instance loot. Equipment `return_damage` feeds the **physical**
   channel and is **uncapped**.
8. **Reflect lives on the species record**, not on buffs and not on the mob
   file. Species 39 (fire) and 40 (magma) declare `return_damage: 25`, channel
   magical; summon mobs 313 and 314 carry those species. Species 39 also carries
   `kinetic-backlash` intrinsically, worth 12% physical plus a rider buff.
9. **Read `config.yaml`, never the Go defaults.** Several shipped values differ
   sharply. Absence is meaningful, and `0` is a legal shipped value.

### Expected outcomes to verify in playtest

| Corpse | golem | skeleton | Notes |
|---|---|---|---|
| trash @100g (100) | 253 | 127 | conjure magma (508) wins outright |
| boss @100g (400) | 403 | 202 | magma still wins |
| boss @250g (1000) | 703 | 352 | raises pull ahead |
| Core Guardian (2800) | 1603 | 802 | raises scale without limit |

**Rounding is `math.Round`, which is half away from zero.** The 100-corpse
skeleton row is `126.5`, so it rounds to **127**, not 126. An earlier draft of
this table said 126 because it was computed in Python, whose `round()` uses
banker's rounding and breaks ties to even. Any check of these figures must use
Go semantics.

Crossover: a golem needs a **609-pool corpse** to beat a magma elemental, which
is roughly a boss instance at 200g or a tough at 300g. Below that, conjures are
the reliable floor; above it, raises are the gold-purchased ceiling.
