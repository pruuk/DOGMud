# #514: every attack interrupts a craft

Issue #514, found in the PR #508 follow-up playtest (run `de5e5248fcc02506`).
Owner rulings 2026-10-13: option A (one seam, `ApplyHarm`), and attrition
does not interrupt.

## Problem

A craft or salvage is meant to stop when its maker takes damage. Only three
hand-placed calls do that (`cancelCraftOrSalvageOnDamage`): an ordinary melee
hit and a player's or mob's damage-over-time tick. Special moves (gore, kick,
bash, maul, and the rest through `combat.ExecuteSkillMove`), spell damage, and
thrown weapons never reach a cancel, so a crafter gored by a boar keeps
crafting and the room later reads the success line.

## Rulings

- **R1. One seam.** The interrupt moves into `Character.ApplyHarm`, which the
  code already documents as the universal damage seam, so every present and
  future attack is covered. The three hand-placed calls go.
- **R2. Attrition does not interrupt.** The pinnacle blade's toll, toxicity
  sickness and a stuck mob's decay are "not an attack" (the existing rule in
  `pinnacle_tick.go`: no sleep-wake, no aggro). They move to a new
  `ApplyAttrition` primitive that changes the pool exactly as `ApplyHarm` does
  but never interrupts work.

## Facts verified against source (master `8267eb06f`)

| Fact | Where |
|---|---|
| `func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int`: returns 0 for `amount <= 0`; applies via `applyVitalChange`; calls `OnDamageTaken`; queues `CharacterDied` once on lethal health harm | `internal/characters/pools.go:644-689` |
| Its comment: "ApplyHarm is the universal damage seam ... melee, spells, maneuvers, damage-over-time and toxicity all pass through this one point"; `ApplyHealthChange` delegates negative deltas to it | `internal/characters/pools.go:651-655`, `internal/characters/resources.go:350-353` |
| The three hand-placed cancels: melee `handlePlayerConcentrationBreak` (`if DamageToTarget > 0`), the player tick right after `ApplyHarm(PoolHealth, ...)`, the mob tick right after `ApplyHarm(PoolHealth, ...)` | `internal/hooks/NewRound_DoCombat_helpers.go:1217`, `NewRound_UserRoundTick.go:341`, `NewRound_MobRoundTick.go:255` |
| `cancelCraftOrSalvageOnDamage` transitions Crafting or Salvaging to Free with `TriggerDamageInterrupt` and tells a player "The pain interrupts your crafting." / "...your salvaging." | `internal/hooks/combat_shared_helpers.go:566-593` |
| The room's stop line comes from the craft-ended hook on any non-complete, non-death exit; it skips a dying crafter (`Health < 1 || !IsAlive()`) | `internal/hooks/Activity_CraftEnded.go`; `activity.Machine.OnCraftEnded` |
| There is no salvage-ended callback | `internal/state/activity/activity.go` (only `OnCraftEnded`) |
| Attrition callers using `ApplyHarm(PoolHealth, ...)`: pinnacle toll (comment: "non-combat attrition ... bypassing the damage hooks entirely"), toxicity sickness, stuck-mob decay | `internal/hooks/pinnacle_tick.go:178`, `internal/hooks/NewRound_AutoHeal.go:93`, `internal/mobcommands/pathto.go:25` |
| Other health harm callers (all attacks or self-inflicted blasts): combat.go x8, skill_moves.go, spell_effects.go x3, throw.go x2, DoCombat_unified x2, riposte in combat_shared_helpers, the two tick sites, a spoiled firebomb in your pack | `grep -rn "ApplyHarm(characters.PoolHealth\|ApplyHealthChange("` |
| Non-health `ApplyHarm` (conviction, stamina) must not interrupt | `internal/actions/combat_taunt.go`, `combat_counter.go`, tick sites `:358`, `:364` |

## Design

1. **`characters`:** `ApplyHarm` becomes a thin call to an unexported
   `harm(pool, amount, source, interrupt bool)` holding today's body. When
   `interrupt` is true, the pool is health and the applied amount is above
   zero, `harm` ends any Crafting or Salvaging activity with
   `TriggerDamageInterrupt` (after the vital change, so a killing blow is
   already at zero health when the craft-ended hook runs and sends no stop
   line). `ApplyAttrition(pool, amount, source)` calls `harm` with
   `interrupt` false. Nothing else in `ApplyHarm` changes.
2. **`activity`:** `OnSalvageEnded(name, fn(SalvagingData, TransitionReason))`,
   the twin of `OnCraftEnded`, fired by `TransitionToFree` and `ForceFree`
   when the machine leaves Salvaging.
3. **`hooks`:** the crafter's own "The pain interrupts your crafting." moves
   into the craft-ended hook (trigger `damage_interrupt`, player, not dying).
   A new salvage-ended handler, wired the same way, sends "The pain interrupts
   your salvaging." on the same conditions. `cancelCraftOrSalvageOnDamage`
   and its three calls are deleted.
4. **Attrition callers** switch to `ApplyAttrition`: pinnacle toll, toxicity
   sickness, stuck-mob decay.
5. **Root guard:** no non-test file outside `internal/characters` and
   `internal/state/activity` may use `TriggerDamageInterrupt`, so the seam
   stays the only place damage ends work.

## Corners

- Absorbed damage (applied 0) does not interrupt.
- Conviction and stamina harm do not interrupt.
- A spoiled firebomb in your own pack interrupts (it is a blast, not attrition).
- Casting's concentration break is a separate, contested design and is not
  touched.
- The "wake a sleeper / drop cancel-on-damage conditions" call
  (`cancelDamageConditions`) has the same hand-placed shape; out of scope,
  noted for a follow-up issue.

## Testing

- `characters`: `ApplyHarm` on a crafting character ends the craft with
  `damage_interrupt`; the same for salvaging; `ApplyAttrition` leaves the
  craft running; conviction harm leaves it running; harm fully absorbed
  leaves it running.
- `activity`: `OnSalvageEnded` fires once with the snapshot on leaving
  Salvaging, never for Crafting or Casting, never on a vetoed transition.
- `hooks`: the existing craft-ended and damage tests move from
  `cancelCraftOrSalvageOnDamage` to `ApplyHarm`; a killing blow sends neither
  the room line nor the crafter line; a salvager is told; a skill-move hit
  (`combat.ExecuteSkillMove`) on a crafting player interrupts the craft (the
  #514 case).
- Root guard proven able to fail.
- Playtest: gored by a boar mid-craft, the room reads the stop line and the
  crafter reads "The pain interrupts your crafting."; a pinnacle-blade wielder
  finishes a craft.
