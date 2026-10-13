# Consider: a fight read built on combat's own math (#301)

Status: design, approved in brainstorm with the owner 2026-10-13. Issue #301.

## Problem

`consider` reported "An even contest" against targets the player then killed
in one round. The verdict maps `combat.PowerScore` ratios to eight bands, and
every ratio in (1, 2] reads "even". PowerScore is a sum of offense, defense,
durability, skills, mutations and a kill/death term, so a fight's real edge
grows much faster than the ratio (roughly with its square when both offense
and durability favour one side). The bands are also lopsided: 0.95 reads
"They have the upper hand" while 1.9 reads "even".

PowerScore also carries its own copy of the swings-per-round formula, separate
from the one combat uses, so the two can drift.

## Decisions (owner, 2026-10-13)

1. The verdict tells the player **what the fight would cost them**, not who
   wins: from "it won't lay a finger on you" to "you have zero chance".
2. Seven bands (below). The bottom band is measured from the other side: you
   would die before taking a sliver of its health.
3. Both characters are read **as they stand now**: buffs, wards, potions,
   enchantments (Hungering Touch's lifesteal) and conditions count. Special
   moves, ambush openers, parties and room effects are out of v1.
4. `consider` becomes an **opposed roll**: the considerer's Perception plus
   Search against the target's Charisma plus Rhetoric. The roll sets how clear
   the read is and the read trains Search (and Perception, its primary stat).
5. `consider` **claims the shared special-move cooldown**, so it competes with
   special moves and spells. Re-reading is allowed; it costs a move each time.
   The help file says plainly that it is a skill roll and results vary.
6. A lost roll hedges the wording and may slide a middle band one step toward
   the centre. The two extreme bands never slide. No extra slide on a critical
   failure: too cruel to new players, whose reads are already weaker.
7. **One set of shared helpers**, not similar-but-separate code. PowerScore
   keeps its absolute one-number shape, but its offense and defense terms are
   rebuilt on the same helpers as the fight prediction.

## Facts verified against source

Read from `origin/master` at `8267eb06f` on 2026-10-13.

| Fact | Where |
|---|---|
| Verdict bands live in `predictionFor`; consider calls `combat.PowerScore` for both sides | `internal/actions/consider.go:68`, `:36-37` |
| Consider is deliberately progression-free, with a "Do not re-add" comment | `internal/actions/consider.go:27-31` |
| `PowerScore(char characters.Character) float64`; its own swings formula, capped at 4 | `internal/combat/calculations.go:20`, `:43`, `:52` |
| Combat's swing count: `calcSwingCount(sourceChar, weapon, weaponSpeed, extraAttacks, isOffhand) int` | `internal/combat/combat_helpers.go:217` |
| Attack plan and weapons: `buildAttackPlan`, `collectAttackWeapons` | `combat_helpers.go:300`, `:316` |
| Per-weapon setup and damage params: `buildWeaponSetup`, `buildDamageParams` | `combat_helpers.go:413`, `:492` |
| Attack score: `calcAttackScore(source, target, weapon, penalty, ctx combatContext)` (needs a `combatContext`) | `combat_helpers.go:567` |
| `CalcRawDamage(stat, skillRank, itemMult, channel) float64`, pure | `internal/combat/damage_pipeline.go:72` |
| `ApplyMitigation`, `EffectiveMitigationFor`, pure; mitigation already folds wards, potions, shield spells, natural armor, gear | `damage_pipeline.go:106`, `:115`; `internal/characters/combat.go:155` |
| Defence score per type: `(*Character).GetDefenseScoreFor(defenseType, includeSkill)` | `internal/characters/combat.go:284` |
| Analytic hit chance: `dice.OpposedSuccessChance(attackerStat, defenderStat, stdDev) float64`, pure; spread `dice.StdDevFor(mean)` | `internal/dice/dice.go:171`, `:435` |
| Opposed roll with contest floors: `dice.OpposedRollStat(atk, def) (bool, float64, RollResult, RollResult)` | `internal/dice/contest_floors.go:60` |
| Crit bar and crit size: `CritBarFor(atkRank, defRank)`, `CritDamageMultiplier(skillRank)` | `internal/combat/crit_bar.go:34`, `crit_damage.go:51` |
| Live defence and contest roll dice and write state (`runBestOfAllDefenseWithRunner`, `RunContest`), so a prediction cannot call them | `combat_helpers.go:703`; `run_contest.go:23` |
| Lifesteal: `atkChar.StatMod("lifesteal_pct")` heals that share of damage dealt; Hungering Touch grants it | `internal/hooks/NewRound_DoCombat_unified.go:390`; `_datafiles/world/dogmud/enchantments/hungering-touch.yaml` |
| Spell damage is computed only in `rollSpellDamage` (rolls dice, in `hooks`) | `internal/hooks/combat_shared_helpers.go:72` |
| Shared cooldown: `ClaimSpecialMove(char) bool`, `SpecialMoveReady(char) bool`; casting shares it | `internal/actions/special_move_cooldown.go:27`, `:38`; `internal/actions/cast.go:365` |
| Shipped `SpecialMoveCooldown: 4`, `SkillWeight: 5.0` | `_datafiles/config.yaml:784`, `:1155` |
| Training: `(*Character).OnSkillUse(skillName, userId) bool`; actors expose `OnSkillUse` | `internal/characters/progression.go:370`; `internal/actions/actor_user.go:79` |
| `skills.Search`, `skills.Rhetoric`; Search's primary stat is perception, Rhetoric's is charisma | `internal/skills/skills.go:37`, `:33`, `:332-333` |
| PowerScore consumers: behavior-tree ratio checks, targeting, leaderboard | `internal/behaviortree/conditions_combat.go:44`; `internal/targeting/select.go:91`, `:114`; `modules/leaderboards/leaderboards.go:211` |
| Ratio thresholds in shipped behavior trees: lookout `above 1.0`, thief `above 1.5`, predator `ratio_below 0.85` | `behaviors/archetypes/lookout.yaml:49-50`, `thief.yaml:50-51`, `predator.yaml:44` |
| AI companion reads `Consider(...).Ratio`: its own verdict copy (has drifted, no "badly outmatched") and a flee check `odds < 0.4` | `modules/aicompanion/actions.go:717`; `modules/aicompanion/combat.go:322`, `:912` |
| Callers: player `usercommands/consider.go:55`, mob `mobcommands/consider.go` | as named |
| Verdict strings are asserted in `consider_test.go`; listed in both help templates; quoted by no quest or dialogue | `internal/actions/consider_test.go:135`; `_datafiles/world/{dogmud,default}/templates/help/consider.template` |

## Design

### Layer 1: one character's profile (no opponent)

New pure functions in `internal/combat`, no dice, no state writes:

- **Offense profile.** Swings per round from `calcSwingCount` over the real
  attack plan (`collectAttackWeapons`, offhand and extra arms included),
  expected raw damage per swing by channel through `CalcRawDamage` and the
  weapon multipliers `buildWeaponSetup` already applies, crit share and crit
  size from `CritBarFor` / `CritDamageMultiplier`, lifesteal share from
  `StatMod("lifesteal_pct")`, and the attack score inputs.
- **Defense profile.** Effective mitigation by channel (`EffectiveMitigationFor`),
  the best-of-all defence scores (`GetDefenseScoreFor`), and health gained or
  lost per round from conditions already on the character (heal-over-time,
  damage-over-time).
- **Spell offense.** The expected (mean) part of `rollSpellDamage` moves into a
  shared, pure helper that both `rollSpellDamage` and the profile call; the
  dice stay in `rollSpellDamage`. The plan settles the package after checking
  import edges (`hooks` imports `combat`, so the helper cannot live in `hooks`).
  A character with a spellbook adds its best offensive spell's expected damage
  divided by `SpecialMoveCooldown`, because casting shares that cooldown.

Where combat computes an input inline today (the attack score's multipliers in
`calcAttackScore`, the defence adjustments inside the live loop), the input is
extracted into a function that both the live path and the profile call. No
input is recomputed by a copy.

### Layer 2: a matchup

`PredictFight(a, b)` combines the two profiles:

- a's hit chance against b: `dice.OpposedSuccessChance` of a's attack score
  against b's best defence score, with `dice.StdDevFor`. This deliberately
  approximates the live contest: it ignores the contest floors, gap
  compression, fumbles and partial-defence degrees. The read is meant to be
  rough, and the bands are wide enough to absorb it.
- a's damage per round to b: swings x hit chance x expected damage through b's
  mitigation (crit share included), plus spell offense, then minus b's
  heal-over-time and plus b's damage-over-time; a's lifesteal reduces a's own
  incoming damage. Each net rate floors at 0.
- rounds for a to kill b, and for b to kill a, from current health.
- **Cost** = rounds for you to kill it x its net damage per round to you /
  your current health (the share of your health the fight takes).
- **Dent** = rounds for it to kill you x your net damage per round to it / its
  current health (the share of its health you take before dying).

### The seven bands

Checked in this order; limits are balance knobs in `config.balance.go` and
`_datafiles/config.yaml` (initial values below, tuned by the playtest):

| # | Condition | Meaning (copy written separately) |
|---|---|---|
| 1 | it deals you no net damage, or cost < `ConsiderBandUntouched` (0.05) | it won't lay a finger on you |
| 7 | dent < `ConsiderBandHopelessDent` (0.05) | what could you possibly be thinking? You have zero chance |
| 2 | cost < `ConsiderBandScratches` (0.25) | a few scratches at most |
| 3 | cost < `ConsiderBandHurt` (0.6) | you'll win, but you'll feel it |
| 4 | cost < `ConsiderBandBarely` (1.0) | you'd win, barely, if at all |
| 5 | cost < `ConsiderBandLethal` (3.0) | it would likely kill you |
| 6 | otherwise | it would kill you quickly |

Band 1 is checked first so a target that cannot hurt you never reads hopeless
just because you cannot hurt it either. Rounds-to-kill of infinity (zero net
damage) is handled explicitly, not by floating-point accident.

### The read roll (players only)

1. `consider` claims the shared special-move cooldown with `ClaimSpecialMove`.
   If it is running, the player gets the existing not-ready refusal: no roll,
   no cost, no training.
2. Read score = Perception (`ValueAdj`) + Search rank x `SkillWeight`. Target's
   score = Charisma (`ValueAdj`) + Rhetoric rank x `SkillWeight`.
3. `dice.OpposedRollStat(read, target)`. Read quality:
   - **clear**: won, and margin / `dice.StdDevFor(read)` >= `ConsiderClearReadZ` (1.0)
   - **fair**: won
   - **muddy**: lost
4. A muddy read slides bands 2 to 6 one step toward band 4; bands 1, 4 and 7
   stay. The slide is deterministic given the roll, so tests can pin it.
5. The roll trains through `actor.OnSkillUse("search")` on every roll that
   claimed the cooldown. The "Do not re-add a progression call" comment is
   rewritten to say the cooldown is the gate.
6. Output: an opener line by read quality ("You take a fresh look at X..."
   style; clear is confident, muddy hedges), then the band line. 7 band lines
   plus 3 openers, written to the player-copy rules (no numbers, 80 columns).

Mobs (`mobcommands/consider.go`) and the AI companion use `PredictFight`
directly: no roll, no cooldown, no training. The companion's flee check moves
from the PowerScore ratio to the prediction (flee when the fight is predicted
lethal, band 5 or worse), and its private verdict copy is deleted in favour of
the shared band text. `ConsiderResult` carries the band and the read quality;
`Ratio` is removed once nothing reads it.

### PowerScore on the shared helpers

`PowerScore` keeps its signature and one-number meaning for the behavior
trees, targeting and the leaderboard. Its offense term reads the layer 1
offense profile (deleting the duplicate swing formula at `calculations.go:43`)
and its defense term the layer 1 defense profile. Durability, skills,
mutations and the kill/death term are unchanged.

Because its values move, the plan builds a before/after table of PowerScore
for a spread of real mobs and sample players and checks the three shipped
thresholds (lookout 1.0, thief 1.5, predator 0.85). Thresholds are retuned only
if the table shows a behavior flips for a matchup that matters.

## Out of scope (v1)

Special-move bonuses (knockdown, grapple control), ambush openers, party and
companion help, room effects and mutators, and resource pools running dry.
A party fight reads as one on one.

## Testing

- Layer 1 and 2 helpers against hand-built characters, including a lifesteal
  weapon, a ward, a heal-over-time, and a spellcaster.
- Every band at its limits, band 1 before band 7, and zero-damage sides.
- Read tiers and the muddy slide with a fixed roll; bands 1, 4 and 7 never
  slide; the cooldown refusal does not roll or train.
- A guard that the live combat path and the profile call the same extracted
  input functions (no second copy of the swing formula or attack multipliers).
- PowerScore before/after table as above.
- Playtest gate: consider the Straw Effigy (band 7), a starter rat (band 1)
  and a mid-level mob, both bare and buffed; repeat a consider to see the
  cooldown and a changed read.

## Files expected to change

`internal/actions/consider.go` and its test, `internal/combat/calculations.go`,
`internal/combat/combat_helpers.go` (extractions), a new prediction file in
`internal/combat`, `internal/hooks/combat_shared_helpers.go` (spell mean
extraction), `internal/configs/config.balance.go`, `_datafiles/config.yaml`,
`internal/usercommands/consider.go`, `internal/mobcommands/consider.go`,
`modules/aicompanion/actions.go` and `combat.go`, both `consider.template`
files, `docs/PATCH_NOTES.md`, and the `context.md` of each touched package.
