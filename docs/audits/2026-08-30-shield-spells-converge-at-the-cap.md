# Shield spells all converge on the same value past a modest skill threshold

**Date:** 2026-08-30
**Status:** filed for the **spell scaling unification arc** (owner decision)
**Reported by:** owner, from live playtest — "all the shield type spells give the
same condition/buff and it seems like the outcome is the same as well for all of
them (ex. Chrysalis Cocoon and Conviction Ward seem to do the same amount of
shielding)"

Point-in-time report. Verify against source before acting.

**Verdict: the report is correct.** The magnitude scaling works; the mitigation
cap erases it. This is a design collision, not a broken calculation, which is
why it wants a decision rather than a patch.

---

## The mechanism

`internal/hooks/spell_resolution.go`, `case "shield"`:

```go
shieldBonus := (spellData.CasterStatValue(user.Character.Stats) + weightedSkill) / 3
if magnitude > 0 {
    shieldBonus = int(math.Round(float64(shieldBonus) * float64(magnitude) / 100.0))
}
target.Character.AddCondition(characters.ConditionShield, duration, float64(shieldBonus), "spell")
```

where `weightedSkill = spellcasting × SkillWeight` (**5.0** shipped).

That magnitude scaling is correct and live. `magnitude` comes from
`spellData.EffectMagnitude` (`resolveSpell`, `spell_resolution.go:85`), and both
spells set it:

| Spell | `effect_magnitude` | `buff_ids` |
|---|---|---|
| `conviction-ward` | **75** | none |
| `chrysalis-cocoon` | **125** | `[52]` |

## Why it does not reach the player

`ConditionShield`'s magnitude is consumed by `GetPhysicalMitigation()`
(`internal/characters/combat.go:185`) as a **percentage**, added to gear and
other non-gear sources. The total is then clamped by `ApplyMitigation`
(`combat/damage_pipeline.go:112`) at `PhysicalMitigationCap`, shipped **0.75**.

So the shield is an unbounded percentage feeding a hard-capped percentage.

Measured, zero gear, `SkillWeight` 5.0, cap 75%:

| Caster | Ward (75) | Cocoon (125) | After cap |
|---|---|---|---|
| novice — Wil 80, skill 5 | 26 | 44 | 26% vs 44% — distinct |
| mid — Wil 100, skill 20 | 50 | 82 | 50% vs 75% — distinct |
| veteran — Wil 113, skill 48 | 88 | 146 | **75% vs 75% — IDENTICAL** |
| Meirok-ish — Wil 120, skill 50 | 92 | 154 | **75% vs 75% — IDENTICAL** |

**Past roughly Wil 100 / spellcasting 25, every shield spell saturates the cap
on its own, before any armor.** Wearing gear lowers the crossover further,
because the cap is on the total.

Adding more shield tiers cannot help: a 200-magnitude spell would also land on
75.

## Three things that hide it further

1. **Identical player text.** Both print *"A shimmering magical barrier forms
   around you, bolstering your defenses."* with no strength indicator, so there
   is no in-game signal even in the range where they genuinely differ.
2. **Identical condition.** Both apply `ConditionShield`, so `conditions` shows
   the same entry either way.
3. **Duration differs but is not shown.** `calcSpellDuration(BaseFolds, ...)`
   with `base_folds` 4 (ward) against 8 (cocoon), so the cocoon does last
   materially longer. The player is never told, by design.

## The one real difference that survives

`chrysalis-cocoon` declares `buff_ids: [52]` (**Chrysalis Shell**:
`magical_mitigation: 15`, `conviction_mitigation: 15`). `conviction-ward` has no
`buff_ids`.

So the cocoon covers three channels while the ward is physical-only, and that
difference is intact — it is simply invisible unless the caster is taking
magical or conviction damage. Note the irony that the spell named **Conviction
Ward** is the one with no conviction mitigation.

## Options, none chosen

Recorded so the arc has somewhere to start; each has a different feel.

1. **Scale into the remaining headroom.** A shield claims a fraction of the gap
   between current mitigation and the cap, so a stronger shield is always
   strictly better but never saturates. Preserves the cap's purpose.
2. **Give shields their own cap**, separate from gear mitigation, so a shield
   and armor do not compete for the same 75%.
3. **Spend magnitude on something other than raw percentage past saturation** —
   duration, channel coverage, or a damage-absorption pool rather than a
   percentage.
4. **Lower the base formula** so the cap is only reachable at extreme skill.
   Simplest, but only moves the threshold rather than removing the convergence.

Whichever is picked, the **player-facing text should differentiate the spells**,
since even a correct fix is invisible today.

## Scope note

The `case "shield"` handler is shared by every `effect_type: shield` spell, so
this applies to any shield added later, not only these two. Only two exist
today.

Filed to the spell scaling unification arc per the owner, 2026-08-30. That arc
already owns the related finding that `effect_type: buff` (17 of 56 spells) gets
no duration scaling at all.
