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

1. The read tells the player **what the fight would cost them**, not who
   wins: from "it won't lay a finger on you" to "you have zero chance".
2. **Verdict and pace are separate.** A single cost number mixes who comes out
   ahead with how fast the fight goes: the Straw Effigy (vast health, pillow
   hits) read "it would kill you quickly" for a fight hundreds of rounds long,
   or "won't lay a finger on you" once regeneration outpaced its hits. So the
   read gives a verdict (who comes out ahead, how cleanly) and two paces (how
   long you would take to bring it down, how long it would take to bring you
   down). Pace wording is general: how fast, never by what means (no melee
   versus spell split).
3. Eight verdict bands (below), including a stalemate for fights neither side
   would finish soon. The bottom band is measured from the other side: you
   would die before taking a sliver of its health.
4. Both characters are read **as they stand now**: buffs, wards, potions,
   enchantments (Hungering Touch's lifesteal) and conditions count. Special
   moves, ambush openers, parties and room effects are out of v1.
5. `consider` becomes an **opposed roll**: the considerer's Perception plus
   Search against the target's Charisma plus Rhetoric. The roll sets how clear
   the read is and the read trains Search (and Perception, its primary stat).
6. `consider` **claims the shared special-move cooldown**, so it competes with
   special moves and spells. Re-reading is allowed; it costs a move each time.
   The help file says plainly that it is a skill roll and results vary.
7. A lost roll hedges the wording and may slide a middle band one step toward
   the centre. The two extreme bands never slide. No extra slide on a critical
   failure: too cruel to new players, whose reads are already weaker.
8. **One set of shared helpers**, not similar-but-separate code. PowerScore
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

### Pace: how long each side needs

Each side's rounds-to-kill maps to one pace on a shared scale. Limits are
balance knobs in `config.balance.go` and `_datafiles/config.yaml` (initial
values below, tuned by the playtest):

| Pace | Rounds to kill |
|---|---|
| moments | <= `ConsiderPaceMoments` (2) |
| a short fight | <= `ConsiderPaceShort` (6) |
| a long fight | <= `ConsiderPaceLong` (20) |
| an age | more than that |
| never | no net damage (rounds to kill is infinite) |

Pace wording is general: it says how fast, never by what means. The read gives
two paces: how long you would take to bring it down, and how long it would
take to bring you down.

### The verdict bands

Checked in this order; limits are balance knobs as above:

| Band | Condition | Meaning (copy written separately) |
|---|---|---|
| 1 untouched | it deals you no net damage, or cost < `ConsiderBandUntouched` (0.05) | it won't lay a finger on you |
| 7 hopeless | dent < `ConsiderBandHopelessDent` (0.05) | what could you possibly be thinking? You have zero chance |
| S stalemate | both paces are "an age" | neither of you is ending this soon |
| 2 | cost < `ConsiderBandScratches` (0.25) | a few scratches at most |
| 3 | cost < `ConsiderBandHurt` (0.6) | you'll win, but you'll feel it |
| 4 | cost < `ConsiderBandBarely` (1.0) | you'd win, barely, if at all |
| 5 | cost < `ConsiderBandLethal` (3.0) | you'd likely lose |
| 6 | otherwise | you'd lose badly |

No verdict mentions pace ("quickly" is gone from band 6); pace is the second
line's job. Band 1 is checked first so a target that cannot hurt you never
reads hopeless just because you cannot hurt it either. Infinite rounds-to-kill
(zero net damage) is handled explicitly, not by floating-point accident.

Worked examples (the playtest confirms them):

- **Straw Effigy** (vast health, pillow hits): if your regeneration outpaces
  its hits, band 1 with paces "an age" / "never" ("It won't lay a finger on
  you. It would take you an age to bring down, and it could never bring you
  down."). If its hits get through, stalemate with "an age" / "an age".
- **A fragile hard hitter**: band 4 with paces "moments" / "a short fight".
- **Megalomania with a starter character**: band 7.

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
4. A muddy read slides verdict bands 2, 3, 5 and 6 one step toward band 4;
   bands 1, 4, 7 and stalemate stay. Paces never slide; on a muddy read their
   line is hedged by the opener. The slide is deterministic given the roll, so
   tests can pin it.
5. The roll trains through `actor.OnSkillUse("search")` on every roll that
   claimed the cooldown. The "Do not re-add a progression call" comment is
   rewritten to say the cooldown is the gate.
6. Output: an opener line by read quality ("You take a fresh look at X..."
   style; clear is confident, muddy hedges), the verdict line, then one pace
   line built from two clauses (your pace against it, its pace against you).
   Copy: 8 verdict lines, 5 + 5 pace clauses and 3 openers, written to the
   player-copy rules (no numbers, 80 columns).

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

## Drift guards

PowerScore, consider, mob consider and the companion must never drift apart
again. Four guards, cheapest first:

1. **No second copy.** A repo-root guard test (like the shipped-data guards)
   fails if `combat/calculations.go`, `actions/consider.go` or the companion
   reintroduces its own swing, hit or damage formula: PowerScore must call the
   layer 1 profiles, and consider and the companion must call `PredictFight`.
2. **Same inputs.** For a set of fixture characters, PowerScore's offense and
   defense terms equal the layer 1 profile outputs.
3. **Prediction matches combat.** For a few fixed matchups (a bare starter
   against a rat, a buffed fighter against a mid-level mob, a caster), run the
   real combat round code many times with a fixed seed; the predicted damage
   per round must land within a stated tolerance of the measured average. A
   combat change that bypasses the shared helpers turns this red. It also
   measures how rough the read really is, which informs the band limits.
4. **Verdicts pinned.** The Straw Effigy, a starter rat and the Megalomania
   case are named tests pinned to their expected verdict and paces.

## Testing

- Layer 1 and 2 helpers against hand-built characters, including a lifesteal
  weapon, a ward, a heal-over-time, and a spellcaster.
- Every band and pace at its limits, band 1 before band 7, stalemate, and
  zero-damage sides.
- Read tiers and the muddy slide with a fixed roll; bands 1, 4, 7 and
  stalemate never slide; paces never slide; the cooldown refusal does not roll
  or train.
- The four drift guards above.
- PowerScore before/after table as above.
- Playtest gate: consider the Straw Effigy, a starter rat and a mid-level mob,
  both bare and buffed, checking each against the worked examples; repeat a
  consider to see the cooldown and a changed read.

## Files expected to change

`internal/actions/consider.go` and its test, `internal/combat/calculations.go`,
`internal/combat/combat_helpers.go` (extractions), a new prediction file in
`internal/combat`, `internal/hooks/combat_shared_helpers.go` (spell mean
extraction), `internal/configs/config.balance.go`, `_datafiles/config.yaml`,
`internal/usercommands/consider.go`, `internal/mobcommands/consider.go`,
`modules/aicompanion/actions.go` and `combat.go`, both `consider.template`
files, `docs/PATCH_NOTES.md`, and the `context.md` of each touched package.
