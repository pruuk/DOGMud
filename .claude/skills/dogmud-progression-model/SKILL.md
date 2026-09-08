---
name: dogmud-progression-model
description: Use when touching stats, skills, advancement, or resource pools. Covers that stats center on 100 with no soft cap and no player ceiling at all, that rank is StatInfo.Training and use counters are telemetry only, that the chance expression lives in exactly two functions which nothing may recompute, that a save stat base is not what the game reads because production uses ValueAdj, and that compression must never be reintroduced because it shrinks every resource pool too.
---

## ValueAdj, not base

A save file's `base:` field is not the number the game uses. Production reads
`ValueAdj`, which is `Base + Training + Mods`, and `Value == ValueAdj` always.

```go
// internal/stats/stats.go:85 - pinned by internal/stats/softcap_test.go:22
ValueAdj = Base + Training + Mods
```

`base:` in `_datafiles/world/dogmud/users/<id>.yaml` is only the first term.
`training:` sits right beside it in the same YAML block and is easy to skim
past. `Mods` comes from worn equipment and is not in the save at all; it is
computed at load, so a save file alone cannot tell you the real number. Even
a correct `Base + Training` read is a floor, not the value, because `Mods`
depends on what the character currently has equipped.

This bit for real on 2026-08-31: a tuning spec fitted its whole table to a
character's `base: 104`, ignoring `training: 21` on the next line and all
gear. The character's actual value was 150. The tell was that the document's
own prose ("falls from about 130 to about 73") implied a different input than
its table (104 to 73); when a document's narrative and its table imply
different inputs, recompute from source rather than picking one.

When fitting any formula against a real character save, always read
`ValueAdj`, never the save's `base:` field alone. When the real number
matters, ask rather than infer from a save file. [[reference-stat-valueadj-includes-training-and-mods]]

## No compression, ever

There is no soft cap on stat values. `ValueAdj == Value` always; stats are
used raw. Compression was removed 2026-08-02. It was inherited from
upstream, hid roughly 10 points from three veteran characters, and because
`HealthMax`, `StaminaMax`, `ConvictionMax` and `ActionPointsMax` are also
`stats.StatInfo` and call the same `Recalculate()`, it was silently
shrinking every resource pool by roughly 40 percent. Do not reintroduce
compression in `StatInfo.Recalculate()`; anything added there hits the pools
too.

`StatProgressionSoftCap` (default 50, shipped 50) is the number of trained
points at which progression slows sharply. It is not a ceiling on stat
values. The old anti-exploit floor in `CheckStatProgression` is gone
(U10b-0 Phase C): it existed because a use counter could be low while the
stat value was high, which cannot happen now that the rank IS the gains.

## No player ceiling

`IncreaseStat` and `IncreaseSkill` contain no bound check whatsoever. There
is a `TestIncreaseSkill_NoCap` regression test (verified:
`internal/characters/progression_test.go:111`). Players have no hard
ceiling at all.

The only hard ceilings are mob-only, gated on `c.IsMob`: `MobStatTrainingCap`
and `MobSkillTrainingCap`. Both are declared in `internal/configs/config.balance.go`
and defaulted in `internal/configs/config.balance.mobs.go` (default 50 for
stats, default 25 for skills). They are enforced inside the two chance
functions rather than the `Check*` functions; `MobStatTrainingCap` is
checked at two sites in `internal/characters/progression.go` (lines 241 and
575). Both cap gains, not value, so a mob authored at base 250 is no harder
to train than one at 180.

**Beware `MobStatCap` / `MobSkillCap`.** These still exist as legacy config
knobs (also declared in `config.balance.go`, defaulted in
`config.balance.mobs.go` to 200 and 3 respectively) and are still validated,
which makes stale references to them read as plausible. They are superseded
and **enforce nothing**. If you find code or a plan that reads `MobStatCap`
or `MobSkillCap` expecting it to gate anything, that is the trap: the field
compiles, the validator runs, and nothing downstream consumes the value for
enforcement. The live gate is the `*TrainingCap` pair above.

## The curve

Curve (`characters.CalculateProgressionChance`, `internal/characters/progression.go:85`):
below the soft cap `base × exp(-decayBelow × rank/softCap)`, above it the
decay continues with `decayAbove` rather than reaching zero. Stat rolls also
multiply by `StatProgressionRate`.

Shipped config differs from the Go defaults. Label which is which:

| Knob | Go default | Shipped (`_datafiles/config.yaml`) |
|---|---|---|
| `BaseProgressionChance` | 0.30 | **0.12** |
| `StatProgressionRate` | 1.0 | **2.25** |
| `StatProgressionSoftCap` | 50 | 50 |

With shipped config a fresh stat is roughly 27 percent per use, falling to
roughly 1.3 percent at 50 trained points.

A progression roll happens on every use, and the odds depend on how far the
stat or skill has already come, not on how often it has been used. Rank is
`StatInfo.Training` for a stat and the skill level for a skill (U10b-0 Phase
C). Use counters are still recorded in saves and shown on the admin
dashboard, but they are telemetry only and no longer feed the curve.
`UsesPerRank` is now read by nothing at all (U10b-0 Phase E removed its last
two consumers); the knob and `internal/configs/smoke_test.go`'s assertion
that it is positive survive only so an existing `config.yaml` still
validates.

Equipment cannot make a stat harder to train. The retired value floor read
`GetStatValue`, which includes `Mods`, so wearing a stat item raised your
own difficulty. Rank now reads `Training` alone.

## The two functions

The chance expression lives in exactly two functions, and nothing may
recompute it: `Character.ProgressionChanceForStat`
(`internal/characters/progression.go:228`) and `ProgressionChanceForSkill`
(`internal/characters/progression.go:115`). Production rolls them, the tests
pin them, and the admin dashboard displays them.

The dashboard used to hand-roll bare `CalculateProgressionChance`, which
silently dropped `StatProgressionRate` and every per-stat, per-skill,
mutation and buff multiplier. That drift is why a truncation bug which
sealed two of a live character's stats read as a tuning problem for months.

`characters.ProgressionRollThreshold(chance)` (`internal/characters/progression.go:54`)
is the matching seam for the integer roll threshold; the denominator behind
it stays unexported on purpose, so a caller cannot drift from production if
the resolution changes again.

## Which paths train which stat

Verified by grep and reading, 2026-08-22, post-U10b-0 Phase C. Read this
before touching any `StatProgressionMultipliers` value; prior plan versions
modelled vitality against the wrong faucet entirely.

Two distinct entry points are often confused: `OnStatUse(stat, userId)`
tracks the counter and rolls progression; `TrackStatUse(stat)` increments the
counter only, with no roll. A literal grep for `OnStatUse("dexterity"` finds
nothing, which is misleading: combat passes the stat as a variable. Do not
conclude a stat has no faucet from a literal grep.

| stat | faucets |
|---|---|
| strength | every attack (`emitAttackerStatGain`); block defence; `blacksmithing` skill use |
| dexterity | every attack; dodge and parry defence; `weapon-combat`, `unarmed-combat`, `skullduggery`, `tailoring`, `jewelcrafting` skill use |
| perception | `consider`, `look <target>`, `shoot`; `ranged-combat`, `search`, `alchemy`, `cooking`, `enchanting`, `salvage` skill use |
| vitality | regen tick; taking a physical crit (dormant, see below); no direct `OnStatUse` caller |
| willpower | quell and defy defence; `spellcasting` skill use; taking a magical crit (dormant) |
| charisma | shop buy/sell (mob side); `rhetoric`, `bartering`, `manifestation` skill use; taking a conviction crit (dormant) |

Every skill use also trains its primary stat. `OnSkillUse` ends with
`c.OnStatUse(skills.GetSkillPrimaryStat(skillName), userId)`. The map is
`SkillPrimaryStats` in `internal/skills/skills.go`. This roughly doubles the
faucet count for dexterity and perception and is easy to miss.

Both attack-side stats fire per attack, not per round
(`NewRound_DoCombat_unified.go`: `emitAttackerStatGain(atk, "strength", ...)`
then `"dexterity"`), so a multi-attack round trains each several times. That
is why `strength: 0.20` and `dexterity: 0.15` are the lowest multipliers
shipped.

**The coupling that makes tuning non-separable.** `CheckRegenProgression`
applies `GetStatProgressionMultiplier(statName)` and
`regenDamperFactor(statName)` on top of its own base. The per-stat
multiplier is shared between a stat's combat/skill faucet and its regen
faucet. You cannot retune strength for combat pace without also rescaling
how much strength the stamina-regen tick grants. This applies to all four
stats that regen trains: health trains vitality and willpower, stamina
trains strength and vitality, conviction trains willpower and charisma.

Vitality is the extreme case, not a special case: regen is its only faucet,
so its multiplier is purely a regen dial. A shipped `vitality: 4.5` was
compensation for `RegenProgressionBase: 0.01`, which is 100x smaller than
`BaseProgressionChance`; it is not a tuning artefact to be casually cut.

**The crit-toughen faucet is dormant.** `OnCritReceived(damageChannel,
userId)` trains the stat that taking a critical hit toughens, via
`ToughenStatFor`: physical trains vitality, magical trains willpower,
conviction trains charisma. It runs `TrackStatUse` then
`CheckStatProgression`, so it is a full faucet with the per-stat multiplier
applied, but it is dead in production right now. It early-returns on
`ObservedCritProgressionBonus <= 0`, and that knob is absent from
`config.yaml`. Its validator uses the `< 0` idiom (a deliberate off-switch
reading), so an absent key stays at the Go zero value 0 rather than being
corrected to a nonzero default.

If this knob is ever turned on, cutting the affected multiplier and enabling
the knob must be solved together, not as two independent changes, because
turning it on switches a second faucet on for vitality (and willpower and
charisma) at the same moment an existing multiplier assumption changes.

Manifestation is trained by more than `assess`: the cast path picks the
skill from the spell's school (`castSkill = skills.Manifestation` when
`spellData.HasSchool(spells.SchoolManifestation)`), so all 14
`SchoolManifestation` spells train it, plus `assess`. Grepping for a string
literal where production passes a variable is the same class of error as the
dexterity case above; when a faucet looks absent, find the variable call
before concluding it does not exist. [[reference-stat-progression-faucet-map]]

## Sources

Lifted verbatim from CLAUDE.md:
- "Stat & Progression System" (lines 362-425)

Folded memory files:
- [[reference-stat-valueadj-includes-training-and-mods]]
- [[reference-stat-progression-faucet-map]]

Deliberately left out of this skill (belongs to other systems, not the
progression model): the faucet map's "Casting is CP-bound over an hour, not
cooldown-bound" section (spell economy, not stat progression) and its note
about fielded companions reserving conviction.
