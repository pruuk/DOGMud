# Skill weight knee and crit damage cap

**Status:** designed with the owner 2026-10-31; awaiting owner review of this file.
**Problem (owner 2026-10-31):** an endgame power leak. Meirok (prod, player 24: weapon-combat 89,
unarmed-combat 78, three weapons through extra arms, maxed armour, hungering-touch on every weapon)
solos the Crash Site at 200 to 400 gold far too easily. New and mid player tuning feels right and
must not move.
**Evidence:** the slider model at https://claude.ai/artifact/3hkDFFAvDCXG1yERUdHyqL (melee only,
one mob at a time; formulas below).

## Facts verified against source (master `739891dde`, 2026-10-31)

| Fact | Source |
|---|---|
| `SkillWeight` ships 5.0; Go default 2.0 when absent or `<= 0`, set in two validators | `_datafiles/config.yaml:1181`; `internal/configs/config.balance.go:336`; `config.balance.misc.go:362-363`; `config.balance.progression.go:63-64` |
| 24 production sites outside `internal/configs` read `SkillWeight` and multiply a rank by it inline; there is no shared helper | grep `\.SkillWeight\b` over `internal/` and `modules/`, tests excluded (list below) |
| The channel attack score for every spell, taunt and skill move is one site: `AttackSide.score` | `internal/combat/defence_multiplier.go:98` |
| Melee attack `Dex + rank*SW`, the five defences `stat + rank*SW`, and swing count `(1 + rank*SW/50)` | `internal/combat/combat_helpers.go:607`, `internal/characters/combat.go:286`, `internal/combat/combat_helpers.go:241,262` |
| Crit damage = `CritDamageBase + CritDamagePerSkill * rank`, no cap; ships 2.0 and 0.05 (6.45x at rank 89) | `internal/combat/crit_damage.go:51-63`; `_datafiles/config.yaml:1201-1202` |
| `CritDamagePerSkill <= 0` resets to 0.05; `CritDamageBase <= 0` resets to 2.0 | `internal/configs/config.balance.combat.go:418-423` |
| Every crit multiplier comes from `CritDamageMultiplier` (8 production call sites, all routed through it) | grep `CritDamageMultiplier(` |
| `SkillMultiplier` (raw damage) already stops growing at rank 50 | `internal/combat/damage_pipeline.go:23-42` |
| Gap compression (k 2.80) caps how far a big score lead raises hit and crit rates | `internal/combat/run_contest.go:105-127`; `_datafiles/config.yaml:638` |
| Grandmaster starts at rank 65 (`level <= 64` is master) | `internal/skills/skills.go:441-460` (`GetSkillRankDescription`) |

The 24 sites: `actions/choke_interrupt.go:31`, `actions/consider.go:59,64`,
`actions/skill_helpers.go:110`, `actions/steal.go:94`, `characters/cast_helpers.go:26`,
`characters/combat.go:286`, `characters/progression_award_resolved.go:131`,
`characters/shroud_hide.go:28,48`, `combat/combat_helpers.go:241,262,607`,
`combat/defence_multiplier.go:98`, `combat/flee.go:129,144`, `combat/grapple.go:84`,
`combat/skill_moves.go:284`, `combat/submission.go:87`, `crafting/difficulty.go:27`,
`hooks/combat_shared_helpers.go:179`, `hooks/Position_GrappleTick.go:767`,
`hooks/recovery_contest.go:26`, `hooks/spell_help_effects.go:226`.

## Design (owner rulings 2026-10-31)

### 1. The skill weight knee

A skill rank's contribution to any score becomes:

```
weighted(rank) = rank * SkillWeight                                   rank <= knee
               = knee * SkillWeight + (rank - knee) * SkillWeightAboveKnee   rank >  knee
```

- New knobs beside `SkillWeight`: `SkillWeightKneeRank` (ships **65**, the grandmaster threshold)
  and `SkillWeightAboveKnee` (ships **1.0**). `SkillWeight` stays **5.0**.
- `SkillWeightKneeRank: 0` means no knee (today's straight line). Go default 0, so tests and a
  config without the key behave as today. `SkillWeightAboveKnee` below 0 resets to
  `SkillWeight`; 0 is legal (ranks above the knee add nothing).
- One helper, `configs.WeightedSkill(rank float64) float64`, holds the formula. All 24 sites call
  it instead of multiplying by `SkillWeight`. A guard test fails if any production file outside
  `internal/configs` reads `SkillWeight` directly, and proves it can fail with a planted file.
- **Everywhere, both sides.** The knee applies at every site: combat attack and defence, swing
  count, spells, taunts, skill moves, grapple, flee, stealth, search, steal, consider, crafting
  difficulty, concentration and the progression candidate roll. Players and mobs share it (mobs
  in these zones have combat skill 0 or 1, so it never reaches them in practice).
- Effect at weapon-combat 89: 445 points today, 349 after. Ranks at or below 65 are unchanged.

### 2. The crit damage cap

```
CritDamageMultiplier(rank) = CritDamageBase + CritDamagePerSkill * min(rank, CritDamageRankCap)
```

- `CritDamagePerSkill` drops **0.05 to 0.025** (config only). New knob `CritDamageRankCap` ships
  **65**; 0 means no cap, Go default 0. `CritDamageBase` stays 2.0.
- The maximum crit becomes 2.0 + 0.025 x 65 = **3.625x** (owner: "slightly above 3.5, but close
  enough").
- **This one moves every player, by the owner's choice:** rank 10 crits go 2.5x to 2.25x, rank 25
  3.25x to 2.63x, rank 50 4.5x to 3.25x, Meirok's 6.45x to 3.625x. Mobs (rank 1) go 2.05x to
  2.025x.

### Not changed

`SkillMultiplier` (already capped at 50), swing cap of 4 per weapon, gap compression, crit bars,
item mitigation. Extra arms giving three weapons is a mutation design question, out of scope.

## Testing

- `WeightedSkill`: knee 0 equals `rank * SkillWeight` for every rank; knee 65 / above 1.0 gives 325
  at 65 and 349 at 89; a rank below the knee is unchanged; `SkillWeightAboveKnee` 0 flattens above
  the knee.
- `CritDamageMultiplier`: cap 0 is today's line; cap 65 with 0.025 gives 3.625 at 89 and at 200,
  2.25 at 10; rank 0 still returns the base.
- Config: absent keys load 0 (no knee, no cap); shipped values read 5.0, 65, 1.0, 0.025, 65.
- Guard: no direct `SkillWeight` read outside `internal/configs`; a planted probe fails it.
- Existing combat, contest, stealth and crafting suites stay green (tests run on Go defaults, where
  the knee and cap are off), plus one test per family with the knee pinned on showing a rank-89
  score drops and a rank-50 score does not.
- Boot check, full suite, lint, as `dogmud-shipping`.

## Docs

`internal/configs/context.md` (the helper and knobs), `internal/combat/context.md` (crit cap),
the `dogmud-balance-config` and `dogmud-combat` skills if they quote the crit line or SkillWeight,
`docs/PATCH_NOTES.md` (player voice, no numbers: grandmaster skill still pays, but past it the
returns flatten, and critical hits hit less hard at every level).
