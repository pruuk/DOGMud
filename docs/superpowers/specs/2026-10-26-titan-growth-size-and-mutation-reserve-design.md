# #680: Titan Growth makes you Large, and big bodies reserve Conviction

Issue: #680 (under epic #388). Absorbs #259 (Extra Arms max rank). Depends on
PR #682 (#681, `IsBlockedBy2H` reads wearer size), which must merge first.
Brainstormed with the owner 2026-10-26.

## Facts verified against source (master `ee03fe88d`, 2026-10-26)

| Fact | Where |
|---|---|
| Species size is read in two places: `HandsRequired` and `ChanceToTame` | `internal/characters/worn.go:299,303`; `internal/combat/calculations.go:131` |
| `ChanceToTame` has no callers (GoMud leftover) | grep of the whole repo finds only its definition |
| `HandsRequired`: Large makes any non-shooting weapon 1 hand; Small adds 1 hand | `internal/characters/worn.go:278-308` |
| Only humans (species 1, Medium) are selectable; 10 species are Large, 21 Medium, 13 Small | `_datafiles/world/dogmud/species/*.yaml` |
| No mutation reduces size (62 mutation files checked) | `_datafiles/world/dogmud/mutations/`; shrink idea filed as #683 |
| Titan Growth: no `max_rank`, so the global `MutationMaxLevel: 4` applies; pro +20% health, con -15 dodge | `mutations/titan-growth.yaml`, `config.yaml:1928` |
| Colossus Form: `max_rank: 1`, prerequisites Dense Muscles 1 and Titan Growth 1; +30% Strength, +25% health, `control-immune`, aggro magnet; -18 dodge | `mutations/colossus-form.yaml` |
| Extra Arms: `max_rank: 1`, but the hand code supports levels 1-4 (one arm per level, pairs B and C) and `ExtraArms` is capped at 4 | `mutations/extra-arms.yaml:4`; `internal/characters/hand_slots.go:27-52`; `validate.go:546-552` |
| Colossus cluster members: chitin-plating, colossus-form, dense-muscles, extra-arms, ossified-frame, titan-growth | `clusters:` in the mutation YAMLs |
| Titan Growth holders today: Flesh Golem and Earth Elemental species intrinsic rank 1 (both Large); Homunculus rank 2 (human, Medium); spawn list on 9541, 9547 (bears, Large), 9552 (earth elemental, Large), 229 Windscour Wyrm (aberration, Medium) | species 35, 37; `hooks/chrysifier_homunculus.go:157`; four mob files |
| `SpawnMutations` is `[]string`, each granted at rank 1, with no prerequisite check | `internal/mobs/mobs.go:127,716-745` |
| Reservation ceiling `PoolReservationCapPct: 0.66`; `GetPoolReservation` sums gear and (Conviction only) companions | `config.yaml:2029`; `internal/characters/validate.go:260-284` |
| Ceiling rule today: refuse the breaching action, never force a removal (D3/D4) | `internal/characters/reservation.go:14-27` |
| Message sources: `reserveSources{drainingItem, enchantment, companion}` drive subject and remedies | `reservation.go:211-318` |
| Companion knobs are all Go defaults (none in `config.yaml`) | `config.balance.go:883-890` |
| Mutation rank writers | `bloom_mutation.go:58,149,160`; `character.go:974`; `intrinsic.go:33`; `NewRound_MobRoundTick.go:349,405`; `NewRound_UserRoundTick.go:461,479`; `mobs.go:745,756`; `questengine/bridge.go:428` |
| Pools: `GetGraphPool` checks prerequisites; `GetWeightedPoolWithFloor` and `BloomSeedNewMutation` do not; `RollDeepening` and `BloomAdvanceMutation` pick owned mutations below a max (Bloom uses the global max only, ignoring `max_rank`, which is #259) | `affinity.go:92`; `mutations.go:236-266,386-398`; `bloom_mutation.go:35-160` |
| `GetWeightedPoolWithFloor` callers: `GrantRandomMutationRare` (Phial drink `drink.go:462`, pinnacle item tick `pinnacle_tick.go:213`, behaviour tree `actions_quest.go:69`), quest bridge `bridge.go:419`, mob spawn bonus roll `mobs.go:753` | grep |
| Companion login pass exists | `internal/hooks/companion_reserve_backfill.go:73` `refreshCompanionReservesOnLogin` |
| `RemoveCompanion(instanceId)` | `internal/characters/companions.go:141` |
| `help species` Large line: "2-Handed weapons require 1 hand. Can sometimes dual-wield 2-Handed weapons." | `templates/help/species.template:15` |

## Owner rulings (2026-10-24 and 2026-10-26)

1. Titan Growth rank 3 makes a character Large.
2. Large from a mutation lets only the main hand wield a two-hander as a
   one-hander. Colossus Form extends that to every hand. Large species keep
   every hand.
3. Colossus Form requires Titan Growth 4, so the fourth rank is the gate to the
   apex and never a dead end.
4. Titan Growth, Colossus Form and Extra Arms reserve Conviction. Anchor: the
   maximum builds are Titan 3 + Arms 3 and Titan 4 + Colossus + Arms 1, both at
   the ceiling. Titan 3 + Arms 2 leaves almost nothing for companions.
   Arms 4 + Titan 2 is over budget.
5. A rank whose reservation does not fit is never granted; a different, related
   mutation is granted instead. Gear does not count toward that test.
6. When a gained rank pushes the total past the ceiling, companions are
   dismissed newest first. Gear is never forced off.
7. Players and mobs share every rule here.
8. `ChanceToTame` is dead code and goes.
9. Prerequisites govern earned growth only. Authored mutation sets (species
   intrinsics, mob spawn lists, presets) may skip them but must fit the budget.

## Design

### 1. One size reader

`Character.Size()` (new, `internal/characters`) returns the species size,
stepped to Large when an owned mutation carries a `size` effect whose rank
threshold is met. Titan Growth gets the effect:

```yaml
pros:
  - type: size
    target: large
    value: 3        # the rank at which it applies
```

`size` is a threshold effect: it is not scaled by the mutation level curve and
does not go through `sumEffects`. `HandsRequired` reads `Size()` instead of
`species.Size`. `ChanceToTame` is deleted. A guard test fails if anything
outside `Size()` reads `species.Species.Size`.

### 2. The hand rule

`HandsRequired` gains the slot: a two-hander needs one hand when the wearer is
Large and either (a) the size comes from species, (b) the wearer owns a
mutation with the `colossal-grip` flag (Colossus Form gets it), or (c) the slot
is the main hand. Otherwise it needs its normal hands. Every hand check
(`HandSlot.Is2H`, `handCandidates`, `chooseArm`, and `IsBlockedBy2H` from
PR #682) passes the slot it is asking about, so display, equip and combat agree.
Small keeps its current rule untouched (#683 owns any change).

### 3. Reservation by units

Each reserving mutation declares units per rank:

```yaml
reserve_units: [1, 1, 1, 1]   # titan-growth
reserve_units: [1, 1, 1, 2]   # extra-arms
reserve_units: [1]            # colossus-form
```

A character's mutation units are the sum, over owned mutations, of the first
`rank` entries. One unit is `MutationReserveUnitPct` (new knob, shipped in
`config.yaml` at `0.11`) of max Conviction. The unit budget is
`floor(PoolReservationCapPct / MutationReserveUnitPct + 1e-9)`, which is 6 at
the shipped values; it is derived, never a second knob.

`GetPoolReservation("conviction")` adds `floor(max * units * unitPct)`, clamped
to the ceiling. Intrinsic species mutations count (golem companions reserve 11%
of their own Conviction, which is harmless and keeps one rule).

| Build | Units | Two-handers held | Result |
|---|---|---|---|
| Titan 4 + Colossus + Arms 1 | 6 | 3 (every hand) | at the ceiling |
| Titan 3 + Arms 3 | 6 | 2 (main hand, plus one pair) | at the ceiling |
| Arms 4 | 5 | 3 (three pairs) | about 44 Conviction left on an average character |
| Titan 3 + Arms 2 | 5 | 2 | about 44 left |
| Arms 4 + Titan 2 | 7 | none | never granted |

### 4. One eligibility filter for every grant

New `mutations.CanGain(owned, id, rank, sp) bool`: conflicts, body parts,
prerequisites, moon flavor, `max_rank`, and `FitsReserveBudget` (units after the
gain at or under the budget). Every pool and every deepening pick uses it:
`GetGraphPool`, `GetWeightedPoolWithFloor`, `RollDeepening`,
`BloomAdvanceMutation`, `BloomSeedNewMutation` and the mob spawn bonus roll.
This also closes two existing holes: the weighted pool and Bloom seed skip
prerequisites (an apex like Colossus Form can be granted without its
prerequisites today), and Bloom ignores `max_rank` (#259).

Related replacement:
- **Deepen:** candidates are owned mutations that `CanGain` the next rank.
  Candidates sharing a cluster with the refused pick are preferred; only if none
  exist does it fall back to any candidate, then to acquiring.
- **Acquire:** the pool is already affinity weighted toward the player's
  clusters, so dropping ineligible entries keeps the draw related.
- **Nothing fits:** the grant returns nothing. On the player and mob ticks the
  progress is kept (not reset) so it pays out on the next eligible event. Item,
  quest and Bloom grants already treat "nothing qualified" as a no-op.

Authored grants (species intrinsics, `SpawnMutations`, the Homunculus preset)
bypass the roll, so a guard test checks every authored set fits the budget.
Prerequisites govern earned growth only: an authored set may skip them (owner,
2026-10-26, option B). The four mobs that spawn with Colossus Form and Titan
Growth 1 (9541, 9547, 9552, 229) keep their data unchanged. Colossus Form on a
Titan Growth 1 mob adds its stats only, since its grip rule needs Large and the
three Large-species mobs already wield in every hand.

### 5. Companions dismissed, newest first

After any rank write (section 4 sites, plus login), if total Conviction
reservation exceeds the ceiling, companions are released newest first through
the existing dismissal path until it fits. Gear is never removed. The player
sees, per companion:

> Your changing body claims more of your will. {name} slips its bond and
> leaves you.

The login pass (`refreshCompanionReservesOnLogin`) runs the same check, which
covers saves that already hold over-budget companions when this deploys.

### 6. Messages name the body

`reserveSources` gains `body bool`. Subjects become "Your body", "Your body and
gear", "Your body and bonds", "Your body, gear and bonds" (verb "hold" for every
plural). `remedies()` never offers anything for the body: it cannot be removed.

### 7. Data and copy

- `extra-arms.yaml`: `max_rank: 4`; rank text no longer says "four flailing
  limbs" at three arms; `character.go:310` comment "(0-2)" corrected. Closes
  #259.
- `colossus-form.yaml`: prerequisite Titan Growth `min_level: 4`; add the
  `colossal-grip` flag.
- Help: `titan-growth`, `colossus-form`, `extra-arms`, `reservation` (the body
  as a source), and the `species` size lines (Large: main hand only for a grown
  character, every hand with Colossus Form or as a Large species).
- `docs/PATCH_NOTES.md` entry; `internal/characters/context.md` and
  `internal/mutations/context.md` updated.

## Testing

- Units and budget for every row of the table, including the derived budget at
  the shipped knobs.
- `Size()`: Medium at Titan 2, Large at Titan 3, Large species unchanged.
- Hand rule: grown character two-hander one-handed in the main hand only;
  with Colossus Form in every hand; a Large species in every hand.
- `CanGain` at each grant site: an over-budget rank is never written; a
  same-cluster fit is preferred; nothing fitting keeps the progress.
- Prerequisites now enforced by the weighted pool and Bloom seed.
- Newest-first dismissal on a rank gain and at login; gear untouched.
- Guards: no reader of species size outside `Size()`; every `reserve_units`
  list is as long as the mutation's max rank; every authored mutation set fits
  the budget (authored sets are exempt from prerequisites).

## Out of scope

- Shrinking to Small and light weapons (#683).
- Companion knob values (unchanged Go defaults).
