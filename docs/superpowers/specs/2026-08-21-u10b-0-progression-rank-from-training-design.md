# U10b-0 - Progression Rank From Training (design)

**Status:** APPROVED 2026-08-21, then amended three times as blind
adversarial review falsified statements in it. **Sections 13, 14 and 15
supersede sections 1-12 where they conflict — read them FIRST.** Section 15.4
is the running list of what is now known false, including a premise sections 4
and 13.2 both assert. Delivery is PHASED; see 15.5.
**Sequencing:** this slice lands BEFORE U10b, hence the name (owner, 2026-08-21).
It is a prerequisite of U10b rather than a later arc slice: U10b's Tasks 2 and 4
largely dissolve into it (see section 8), so U10b cannot be re-planned until
this is settled.
**Supersedes:** the use-counter half of
`2026-08-21-u10b-progression-firing-design.md`.

---

## 1. The problem

Progression decay is keyed to a use counter, not to the thing being trained:

```go
// internal/characters/progression.go:172
virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
```

The counter only ever climbs. It is never normalised against the stat's actual
value, so over a long-lived character the two drift arbitrarily far apart, and
the counter (not the stat) decides how hard the stat is to raise.

Three consequences, all confirmed against source:

1. **A stat can be sealed permanently.** `progression.go:206` does
   `threshold := int(chance * 10000)`, so any chance below 0.01% truncates to
   zero and `roll < threshold` can never fire.
2. **Frequency is punished as if it were progress.** A stat that fires many
   times per round exhausts its curve without having grown. The existing
   per-stat multipliers in `StatProgressionMultipliers` are compensations for
   this, not balance choices.
3. **The counter is shared.** Every site that trains a stat pays into the same
   counter, so adding a high-frequency site degrades every other site that
   trains that stat. This is what made U10b's "uncontested class" unshippable:
   it recorded a full use and bought a tenth of a roll.

There is already a partial patch for (1) and (2) at `progression.go:176`, which
floors the virtual rank at the stat's value when the value exceeds the soft cap.
It is keyed on `c.GetStatValue`, which returns `.Value` = `Base + Training +
Mods`, and **`Mods` is equipment**. So gear currently leaks into progression
rank.

## 2. Evidence

`_datafiles/world/dogmud/users/3.yaml` (Meirok), against shipped config
(`BaseProgressionChance 0.12`, `ProgressionDecayBelowCap 3.0`,
`ProgressionDecayAboveCap 2.0`, `UsesPerRank 25`, `StatProgressionRate 2.25`):

| stat | use count | virtual rank | chance | threshold |
|---|---|---|---|---|
| dexterity | 39,772 | 1,590 | 9.2e-12 | **0, dead** |
| strength | 11,671 | 466 | 4.0e-5 | **0, dead** |
| perception | 2,679 | 152 (value-floored) | 1.3% | 130, alive |

Dexterity and strength cannot progress again under any roll. This is live in
production, not a U10b regression.

The config comment at `config.yaml:1006` records the bug being treated as a
tuning problem: "dashboard showed dex as the slowest stat even at 0.10; bumped
+50%". Multiplying 2.7e-11 by 1.5 still truncates to zero.

No save has a `vitality` key in `statusecount` at all, so vitality's rank has
been pinned at 0 for the life of the game. That is why `vitality: 4.5` is the
largest per-stat multiplier in the config.

## 3. The ruling (owner, 2026-08-21)

> Find a different way to have a progression curve without the use counters.
> Just figure it out based upon current raw rank (not the combined rank of raw
> stats + gear).

Use counters are removed as the decay input. The curve is keyed to how far the
character has actually raised the stat, excluding equipment.

## 4. Design

**Rank is the stat's `Training` value.**

`stats.StatInfo` decomposes as `Value = Base + Training + Mods`
(`internal/stats/stats.go:14-21`, verified by
`softcap_test.go:TestRecalculateSumsComponents`):

- `Base` holds the rolled/racial value, so it is the species baseline.
- `Mods` holds equipment and spell modifiers.
- `Training` holds exactly what progression has added. `IncreaseStat`
  increments it and nothing else (`internal/characters/skills.go:207-225`).

So `Training` is already gear-free by construction and species-normalised by
construction. No new field, no accessor arithmetic, no migration.

```go
// replaces progression.go:172-178
virtualRank := c.GetStatTraining(statName)
```

`GetStatTraining` is a new accessor mirroring `GetStatValue`
(`skills.go:229-245`) but reading `.Training`.

**The soft cap moves from 150 to 50.** `StatProgressionSoftCap` currently means
"virtual rank where progression slows sharply" and is expressed in counter
units. Under `Training` it must be expressed in trained points. A human
baseline is 100 and the old cap was 150, so the equivalent is 50 trained
points.

**The value floor at `progression.go:176` is deleted.** It exists only to stop
counter/value drift, which cannot occur once rank *is* the value. Deleting it
also removes the gear leak.

## 5. Parity

Keying to `Training` with `StatProgressionSoftCap: 50` reproduces the currently
documented figures at both ends, with no other retuning:

| anchor | computation | result | CLAUDE.md says |
|---|---|---|---|
| fresh stat, `Training 0` | `0.12 x 2.25` | **27.0%** | "roughly 27% per use" |
| stat at 150, `Training 50` | `0.12 x e^-3 x 2.25` | **1.34%** | "roughly 1.3% at virtual rank 150" |

The re-basing is therefore free. This is strong evidence that the counter was
never carrying information the stat value did not already hold.

Applied to Meirok:

| stat | `training` | today | under U10b-0 |
|---|---|---|---|
| strength | 21 | dead | 1.53% |
| dexterity | 12 | dead | 1.97% |
| perception | 51 | 1.3% | 1.29% |
| vitality | 14 | ~100% (rank pinned 0) | 52% (see section 7) |
| willpower | 35 | - | 3.31% |
| charisma | 30 | - | 0.98% |

Two live stats are revived and perception is untouched.

## 6. Skills

Skills get the same treatment and it is simpler: the skill's level IS its raw
rank, there is no equipment component, and `SkillSoftCap` is already 50.

```go
// replaces progression.go:86-97
virtualRank := c.Skills[resolveSkillName(skillName)]
```

This also deletes the use-count normalisation block at `progression.go:88-95`,
whose stated purpose is to stop frequently-fired skills exhausting the curve
faster than rarely-fired ones. That failure mode cannot occur once rank is the
skill level. The per-skill multipliers in `SkillProgressionMultipliers` remain
and keep their balance role (see section 7).

## 7. Config retuning required

This is the bulk of the work and it must not be skipped.

**`vitality: 4.5` must come down.** It was tuned against a stat whose counter
never moved. Under U10b-0 vitality decays like everything else and 4.5 yields 52%
per use at Meirok's training level. A starting value of 1.0 puts it in line with
perception and willpower; the playtest gate should move it.

**`strength: 0.20` and `dexterity: 0.15` must be re-derived.** They were lowered
because combat stats fire many times per round and exhausted the counter. Under
U10b-0 firing often no longer self-limits at all, so the per-stat multiplier
becomes the *only* frequency brake. They are still needed, possibly more than
before, but the current values were chosen to compensate for a mechanism that
no longer exists. Derive them against measured firing rates per round rather
than carrying the old numbers forward.

**Every other entry in `StatProgressionMultipliers` and
`SkillProgressionMultipliers` inherits the same caveat.** Each was tuned in a
world where frequency was self-punishing. Treat the whole block as needing
re-derivation, not migration.

**`UsesPerRank` becomes dead** for decay purposes. Do not delete the counters
themselves (see section 9).

## 8. What this dissolves in U10b

- **U10b Task 2 (the uncontested seam) collapses to a chance multiplier.** With
  no counter cost, "uncontested" is simply "roll at a lower rate". The separate
  `OnStatUseUncontested` / `OnSkillUseUncontested` methods may still be worth
  having for the class guard's benefit, but the design hazard that made them
  load-bearing is gone.
- **U10b Task 4 Step 4 becomes safe.** `movementTrainsSearch()` can be deleted
  without flooding anything, because recording a use no longer costs rank. The
  30-line rationale at `usercommands/go.go:35-64` is specifically an argument
  about counter flooding and stops applying. That comment must be removed in
  the same commit, not left to contradict the new model.
- **U10b's `UncontestedProgressionRate: 0.10` first guess becomes defensible**
  rather than structurally wrong.
- **U10b inherits two dashboard obligations from section 10.** Every new firing
  site it adds inflates the retained telemetry counters without a matching
  gain, so (a) discovery health's activity percentiles need re-reading once
  U10b lands, and (b) the deviation metric's "one roll per recorded use"
  assumption weakens in proportion to the uncontested mix. Neither breaks the
  dashboard; both make it read optimistic if left unexamined.
- The free-action faucet problem gets *sharper*, not softer: with no hidden
  counter cost, the per-roll chance is the only thing standing between a player
  and spamming a free `look`. U10b matters more after U10b-0, not less.

## 9. Save and migration impact

`statusecount` and `skillusecount` stay in the save and keep being written.
They are still useful telemetry (the balance dashboard reads them) and removing
them is a separate, larger change. They simply stop feeding the curve.

Keeping them is not free: the dashboard presently presents them *as* the curve,
so it must be re-keyed in this slice or it will keep reporting a retired model
in the owner's own balance tool. See section 10.

No field is renamed, no counter is re-keyed, nothing is migrated. `Training` is
already persisted for every character.

Existing characters see an immediate change in progression rate the moment this
deploys, upward for anyone whose counter had outrun their training and downward
for anyone whose counter lagged it. Meirok's revived dexterity and strength are
the clearest case. This is player-visible and needs a patch note.

## 10. The progression dashboard

`internal/web/admin.progression.go` (+ `_datafiles/html/admin/progression/
index.html`) is the **only reader of the use counters outside
`internal/characters` and the mob save code**, and it is built on them end to
end. It is also the tool the balance owner reads to decide whether progression
is healthy, so shipping U10b-0 without it means the dashboard keeps reporting a
model the engine no longer runs.

Verified by grep across `internal/` and `modules/` for `GetStatUseCount`,
`GetSkillUseCount`, `StatUseCount`, `SkillUseCount`: the only other hits are the
field declarations, `validate.go`'s dead-key pruning, the spawn/despawn hooks,
and `internal/mobs/instance_save.go`. **No in-game command surfaces a use
count.** The dashboard is the whole surface.

### 10.1 What breaks

| Site | Today | Under U10b-0 |
|---|---|---|
| `calcExpectedRank(uses, mult, bal)` | `uses x mult / UsesPerRank` | Meaningless. `UsesPerRank` is declared dead in section 7 |
| `buildSkillHealth` `avgDeviation` | `rank - calcExpectedRank(uses)` | **Identically zero by construction** once virtual rank *is* rank. The curve term of the health score collapses silently, scoring every skill as perfectly healthy |
| `buildSkillHealth` stall detection | `approxUsesAtRank = rank x UsesPerRank / mult` | Back-computed from the retired curve |
| `playerSkillJSON.VirtualRank` | expected rank from uses | Duplicate of `Rank` |
| `playerSkillJSON.ProgressionChance` | `CalculateProgressionChance(int(virtualRank), softCap)` | Rank input becomes correct, but see 10.2 - this expression is **already wrong today** |
| Stat tooltip `training=N \| uses=M` | uses is the load-bearing figure | The two swap roles; `training` becomes the curve input |
| `buildStatHealth` value buckets | `GetStatValue` | Should key on `Training`, which is what the curve now reads. Also fixes a live bug, see 10.5 |
| `playerTotalActivity` -> discovery percentiles | sum of both counters | **Survives.** Counters keep incrementing per section 9, and discovery health only ever used them as an engagement proxy, never as a curve input. See the caveat in 10.5 |

### 10.2 Chance math must come from the production function, not a copy

`statProgressionChance` (`progression.go:158`) is already documented as "the
single source of truth for the chance `CheckStatProgression` rolls against",
and the faucet test calls it directly for exactly that reason. The dashboard
does not - it hand-rolls bare `CalculateProgressionChance(rank, softCap)` and
omits every multiplier production applies:

- **stats** (`progression.go:183`): `x bonusMultiplier x mutStatMult x statMult
  x StatProgressionRate`. With `StatProgressionRate: 2.25` shipped, the
  dashboard understates every stat chance by more than half before per-stat
  multipliers are even considered.
- **skills** (`progression.go:110`): `x bonusMultiplier x
  GetProgressionMultiplier(skill) x mutSkillMult x buffSkillMult`.

This is a pre-existing drift, not a U10b-0 regression, but it is the reason the
dashboard could never have surfaced the section 2 evidence table.

**Required:** export the stat function and extract its missing skill twin, both
in `internal/characters`:

```go
func (c *Character) ProgressionChanceForStat(statName string, bonus float64) float64
func (c *Character) ProgressionChanceForSkill(skillName string, bonus float64) float64
```

`CheckStatProgression` / `CheckSkillProgression` call these; the dashboard calls
these; nothing recomputes the expression. The skill twin is a straight
extraction of the inline block at `progression.go:100-110`, which currently has
no callable form at all. Dashboard calls pass `bonus = 1.0` (the neutral,
non-crit baseline) and the UI must say so, since crit paths roll higher.

### 10.3 Re-deriving the metrics that used the counter

Deviation and stall detection are worth keeping - they are the panels that
answer "is anyone stuck?" - but they need the inverse of the new curve rather
than a division by `UsesPerRank`.

Expected uses to advance from rank `r` to `r+1` is `1 / chance(r)`, so expected
cumulative uses to *reach* rank `R` is:

```
usesToReach(R) = sum over r in [0, R) of 1 / chance(r)
```

with `chance(r)` from the exported function in 10.2 at `bonus = 1.0`. Both
metrics fall out:

- **`expectedRank(uses)`** = largest `R` where `usesToReach(R) <= uses`.
  `avgDeviation = actualRank - expectedRank(uses)` recovers a real curve term
  instead of a structural zero.
- **Stall** = `usesSinceGain = uses - usesToReach(rank)`, compared against
  `1 / chance(rank)` exactly as today.

The loop runs over at most `softCap` ranks, so cost is trivial. `calcExpectedRank`
is deleted.

**State the approximation in the panel.** This model assumes one roll per
recorded use. U10b introduces the uncontested class, which records a use and
rolls at `UncontestedProgressionRate`, so deviation degrades into a lower bound
as the uncontested mix grows. It is a triage signal, not a measurement.

### 10.4 The stat side, which does not exist yet

Stats currently get one value-bucket chart and a tooltip. The entire dead-stat
class from section 2 lives in stats, so this is the half of the dashboard that
matters most after U10b-0. Add:

1. **Per-stat next-chance column** in the player stat table, from
   `ProgressionChanceForStat`, alongside `training` (the curve input) and the
   raw value.
2. **A dead-stat alarm.** `CheckStatProgression` rolls
   `threshold := int(chance * 10000)` (`progression.go:206`), so any chance
   below 0.01% truncates to zero and the stat can never fire again. Flag
   `threshold == 0` as an error state and `threshold < 10` as a warning. This
   is the live-ops counterpart of the regression test in Done-when item 5: the
   test stops it re-landing, the alarm catches it if it does.
3. **Stat stall detection**, mirroring 10.3 against `Training`. The existing
   Stall Detection panel is skills-only.
4. **Distribution keyed on `Training`,** not `Value`. `Value` includes `Mods`,
   which section 4 deliberately removes from the curve, so a value histogram no
   longer describes progression difficulty. Keep the value histogram as a second
   series if the population view is still wanted.

### 10.5 Pre-existing defects to fix in the same pass

- **Offline players report stat value 0.** `buildStatHealth` calls
  `GetStatValue`, which reads `StatInfo.Value` - tagged `yaml:"-"`
  (`stats.go:16`), so it is never persisted. `loadRecentUserFiles` does a raw
  `yaml.Unmarshal` and never calls `Recalculate()`, so **every offline player
  lands in the `0-50` bucket** and the Stat Value Distribution chart is
  garbage for anyone not logged in. The same file's `getStatBaseValue` carries
  a comment acknowledging exactly this hazard; it was fixed in the player table
  and missed in the distribution. Owner ruling 2026-08-21: fix here.
- **Delete the local accessor duplicates.** `getStatBaseValue` and
  `getStatTraining` in `admin.progression.go` are hand-rolled switch
  statements that predate the `GetStatTraining` accessor section 4 adds. Both
  go; the web package calls the `characters` accessors.
- **Relabel, do not delete, the counters.** Per section 9 they remain as
  telemetry. Rename the player-table "Activity" column and the tooltips so they
  read as engagement volume, not progression progress, and note on the panel
  that they no longer drive the curve. Leaving them labelled as-is is precisely
  the failure mode this section exists to prevent.
- **Activity percentiles will inflate under U10b.** Discovery health flags
  `too_easy` / `too_hidden` off 10th/50th percentiles of total activity. U10b
  adds uncontested firing sites on free actions, which raises counters without
  raising engagement proportionally. The flags do not break, but their
  thresholds should be re-read after U10b lands, not trusted across the seam.

### 10.6 Not in scope

`internal/mobs/instance_save.go` persists `stat_use_count` /
`skill_use_count` per mob instance. These become vestigial like the player
counters and are left alone; instance saves are wiped by the smoke-test SOP
anyway. `validate.go`'s pruning of dead skill keys from `SkillUseCount` becomes
decorative and is also left alone.

### 10.7 Files

- `internal/characters/progression.go` - export `ProgressionChanceForStat`,
  extract `ProgressionChanceForSkill`, route both `Check*Progression` through
  them.
- `internal/web/admin.progression.go` - re-key every metric, add the stat
  columns and alarms, delete `calcExpectedRank` and the two accessor
  duplicates.
- `_datafiles/html/admin/progression/index.html` - stall table now spans stats
  and skills, new stat columns, dead-stat badge, relabelled telemetry. Note the
  admin pages render through `SafeDom`; keep the existing `tr()` / `esc()`
  helpers rather than interpolating HTML.

## 11. Risks and open questions

1. **This is a buff to veterans.** Anyone with a flooded counter gets their
   progression back. That is the intent, but it should be modelled before
   deploy, not discovered.
2. **Mob progression** uses the same code with `MobProgressionRate`,
   `MobStatCap` and `MobSkillCap`. `MobSkillCap: 3` means mob skill rank is
   near zero under U10b-0 where it could previously be counter-inflated. Verify
   mobs do not become materially faster learners.
3. **The `Training` field is writable by admin tooling** (`admin.skillset`,
   training-point spend). Under U10b-0 that directly sets progression difficulty,
   where before it only set the value. Confirm no admin path can hand a
   character a permanently cheap stat.
4. **Open:** should `ProgressionDecayBelowCap`/`AboveCap` keep their current
   values? Section 5 shows the endpoints match, but the shape between them is
   now measured in trained points rather than counter units, so the midpoint
   feel may differ. Model it.

## 12. Done when

1. `virtualRank` for stats derives from `Training`, for skills from skill level;
   no progression path reads a use counter.
2. `StatProgressionSoftCap` is 50 and the two parity anchors in section 5 hold
   as unit tests.
3. The value floor at `progression.go:176` and the skill normalisation block at
   `:88-95` are deleted, with the gear leak pinned by a regression test (a
   character with stat-boosting gear equipped rolls the same chance as one
   without).
4. `vitality` is retuned; `strength` and `dexterity` are re-derived against
   measured firing rates, with the derivation recorded in the commit.
5. A regression test pins that no stat with a plausible `Training` value
   produces `int(chance * 10000) == 0`.
6. Meirok's dexterity and strength are shown progressing again, by test or by
   playtest.
7. `ProgressionChanceForStat` / `ProgressionChanceForSkill` are the only places
   the chance expression exists; `Check*Progression` and the dashboard both
   call them, and a test pins that the dashboard figure equals the figure
   production rolls (section 10.2).
8. No dashboard metric reads `UsesPerRank`; `calcExpectedRank` and the two
   accessor duplicates in `admin.progression.go` are deleted (section 10.3,
   10.5).
9. The dashboard shows a per-stat next-chance figure and raises a dead-stat
   alarm when `int(chance * 10000) == 0`, verified against a fixture character
   built from Meirok's pre-fix numbers (section 10.4).
10. The Stat Value Distribution chart reports non-zero for an offline player
    (section 10.5) - the current live bug, pinned by test.
11. Patch note written, no raw numbers.
12. Adversarial playtest gate, per the content SOP. The dashboard is admin-only
    and outside the in-game harness, so it needs a separate check: load
    `/admin/progression` against a populated data set and confirm every panel
    renders and no figure contradicts a `characters` unit test.
13. **A written owner punchlist for `/admin/progression`** (owner request,
    2026-08-21). The harness cannot drive an admin page and a unit test cannot
    see a mislabelled column, so the last gate is the owner reading the real
    page on their local server. The punchlist is a deliverable of this slice,
    not an afterthought, and must be specific enough to check without reading
    the diff: per panel, what each figure should now be keyed to, which figures
    should have visibly *changed* against the pre-slice page, which should be
    unchanged (discovery health), and the two expected-value spot checks -
    Meirok's revived dexterity and strength showing a live chance, and an
    offline player showing a non-zero stat distribution.

---

## 13. Owner rulings after the blind review (2026-08-21)

Three blind adversarial reviewers (mechanism, tests, behaviour lenses) reviewed
the plan cut from this spec. They converged, and every blocker was re-verified
against source. **Four statements in sections 1-12 above are false**, and two of
them changed what this slice should do. This section supersedes them and
records the owner's rulings. Sections 1-12 are left as written so the review's
citations stay valid; where they conflict with this section, **this section
wins**.

### 13.1 The truncation is fixed, not relocated (ruling 1: approved)

`CheckStatProgression` rolls `threshold := int(chance * 10000)`
(`progression.go:206`), and `CheckSkillProgression` and `CheckRegenProgression`
do the same. Keying rank to `Training` moves the point at which a chance
truncates to zero; it does not remove it.

| per-stat multiplier | first `Training` where `int(chance*10000) == 0` |
|---|---|
| 0.20 (shipped strength) | 133 |
| 1.00 | 173 |
| 4.50 | 211 |

This is not theoretical. `internal/migration/0.16.0.go` records fyttyn's
vitality at `Base 85 + Training 195`. At the multiplier section 7 mandates
(1.0), that stat computes to `4.07e-5` and is **dead on arrival** - this slice
would re-create its own headline bug on the exact character the migration
exists for. Section 7's instruction to lower `strength` and `dexterity` makes it
worse: every reduction pulls the death point down.

**Fix, both halves:**

1. **Raise the roll resolution to 1,000,000** at all three roll sites. This is
   the quantisation fix and costs nothing - `util.Rand` is already integer.
2. **Add `ProgressionChanceFloor`** (new `Balance` knob, suggested default
   `1e-5`), applied as the last step of `ProgressionChanceForStat` and
   `ProgressionChanceForSkill`, and to `CheckRegenProgression`'s assembled
   chance. Progression then becomes asymptotically slow but never sealed, which
   is what section 1 says the design intends.

Resolution alone is insufficient - it moves the death point to `Training 247`
rather than removing it. The floor is what makes Done-when 5 satisfiable.

### 13.2 Mobs and companions mirror players (ruling 2)

**Section 4's claim that `Training` is "species-normalised by construction" is
false for every mob in the game.** `internal/mobs/mobs.go:532` distributes
`mob.StatPool` into `.Training` one point at a time (`Stats.X.Training++`), and
mob YAML sets no `base`. **A mob's `Training` IS its stat value.**

Consequences of shipping sections 1-12 unamended:

| subject | rank today | rank under U10b-0 as specified | effect |
|---|---|---|---|
| strength-20 mob | 0 | 20 | ~3x slower |
| strength-60 mob | 0 | 60 | ~30x slower |
| gold-scaled instance mob (`pool = goldPaid x statpool`) | 0 | thousands | **permanently frozen** |
| companion, any age | 0 every session | whole pool | **stat growth stops on day one** |

Companions are worst affected: they respawn today with an empty `StatUseCount`,
so they train at the maximum rate every session. Skills move the opposite way -
`MobSkillCap: 3` means a mob's skill rank never leaves the flat part of the
curve, so mobs become *faster* skill learners.

**Owner ruling: mobs and companions mirror players in both chance and
mechanism.** No mob-specific rank path, no special-casing in
`ProgressionChanceForStat`.

**Mechanism: move the spawn pool from `Training` to `Base`.**

```go
// internal/mobs/mobs.go:532 and the archetype branches below it
mob.Character.Stats.Strength.Base++   // was .Training++
```

`Value = Base + Training + Mods` is unchanged, so nothing downstream moves.
`Base` is already what "the baseline this creature started with" means for a
player, and after this it means the same for a mob. `Training` then holds
gains-since-spawn for everyone, `GetStatTraining` needs no `IsMob` branch, and
**gold-scaled instance mobs work automatically** - a bigger buy-in raises
`Base`, and progression difficulty is untouched by it.

**Caps:**

- **Stats: `+50` over whatever the mob started with.** Because `Training` is now
  gains-only, this is simply a hard cap at `Training >= 50`, applied to mobs
  only. It replaces `MobStatCap: 200`, which gated on `GetStatValue` and
  therefore froze any mob whose authored pool already exceeded 200 while
  leaving small mobs uncapped. Delete `MobStatCap` or repurpose it; do not
  leave both.
- **Skills: soft cap 20 for mobs.** New knob `MobSkillSoftCap` (default 20),
  passed to `CalculateProgressionChance` in place of `SkillSoftCap` when
  `c.IsMob`. This **replaces the `MobSkillCap: 3` hard cap**, which under a
  rank-is-level model keeps mobs permanently on the flat part of the curve.
  Owner's stated purpose: stop long-lived companions getting out of pocket
  while still letting them grow.

**Migration is required. Section 9's "no field is renamed, no counter is
re-keyed, nothing is migrated" is false under this ruling.** Neither save
format persists `Base`:

- `internal/mobs/instance_save.go:23-28` persists only
  `strength_training` ... `charisma_training`.
- `internal/characters/companions.go:62` persists only
  `stat_training` (a `map[string]int`).

So existing saves carry the whole pool in a `Training` field. Loaded under the
new code that reads as pure gains: rank enormous, `+50` cap already blown,
progression frozen. Required work:

1. Add `Base` to both save formats (additive, `omitempty`).
2. Migrate existing records: move the persisted `Training` into `Base` and set
   `Training` to 0. Mob instances may instead be discarded (the smoke-test SOP
   wipes them and they are not deployed to prod), but **companions live in
   `users/<id>.yaml` and must be migrated properly** - a player's pet is not
   disposable state.
3. A migration test pinning that a pre-U10b-0 companion save loads with its
   stat values unchanged and its progression rank at 0.

### 13.3 Vitality is retuned against the right faucet (ruling 3)

**Section 7's vitality instruction is built on a code path that is switched off
in production.** The "4.5 yields 52% per use" figure is the `OnCritReceived`
path. `progression.go:361-364` returns early when
`ObservedCritProgressionBonus <= 0`; that knob is **absent from
`_datafiles/config.yaml`**, and `config.balance.progression.go:28-30` corrects
only *negative* values, so an absent key stays `0` and short-circuits for
everyone. The existing test at `progression_faucet_test.go:50-56` documents
exactly this.

Vitality's live faucet is `OnRegenTick` -> `CheckRegenProgression`, whose base is
**`RegenProgressionBase: 0.01`** (`config.yaml:1061`) - two orders of magnitude
below `BaseProgressionChance: 0.12` - further scaled by `(1 - poolRatio)^curve`.

**`vitality: 4.5` is compensation for a faucet 100x smaller than every other
stat's, not a tuning artefact.** Section 7's instruction to set it to 1.0 would
cut the only working source of the stat behind `HealthMax = 5 + Vit x 3 + Str x
1`. Compounding it, Task 3's re-key of `regenDamperFactor` is itself a live
nerf: the damper returns **exactly 1.0 for vitality on every character today**
(nothing tracked vitality use, so its rank is 0), and becomes `e^(-3T/50)` -
0.43 at `Training 14`.

**Ruling: tune vitality roughly in line with the other stats, measured against
the regen faucet, with the damper's new bite included in the calculation.** Do
not carry section 7's 1.0 forward. Delete
`TestConfig_VitalityMultiplierWasRetuned`'s `< 4.5` assertion from the plan; the
target is parity of trained-points-per-hour, not a smaller number.

**Owner intent: the crit-received path should carry part of vitality's growth
for players doing challenging content.** That path is currently inert. To
deliver it, `ObservedCritProgressionBonus` must be **set explicitly in
`config.yaml`** (absence is not neutral here). Pick its value as part of the
same tuning pass, and note it interacts with U10b's toughen path.

### 13.4 The curve is retuned to hold the old pace (ruling 4)

**Section 5's "the re-basing is therefore free" holds pointwise only.** Both
anchors match as chances-at-a-rank, but the map from uses to rank changes shape
completely:

- Old: `rank = uses/25`, decay independent of gains. Integrating gives a
  saturating exponential with asymptote `337.5 x mult`.
- New: rank *is* the gain, so `dT/du = A e^(-kT)` and `T(u)` is logarithmic and
  unbounded.

Measured effect at equal trained value:

| subject | old uses | new uses | factor |
|---|---|---|---|
| perception (mult 1.0) to `Training 40` | ~158 | ~619 | **3.9x slower** |
| strength (mult 0.20) to `Training 21` | ~466 | ~779 | 1.7x slower |
| weapon-combat (mult 0.23) to level 25 | ~1,256 | ~2,103 | 1.7x slower |
| blacksmithing (mult 3.5) to level 25 | ~64 | ~138 | 2.2x slower |

A second, unstated design change: under the old model a skill with `mult < 1.0`
asymptoted at exactly `SkillSoftCap`, so level 50 was unreachable. Under the new
model it is reachable.

**Ruling: retune the curve so pace roughly matches the old behaviour.** This is
a required task, not a follow-up. It means deriving
`ProgressionDecayBelowCap` / `ProgressionDecayAboveCap` (and possibly
`BaseProgressionChance`) against measured trained-points-per-hour, **not**
preserving the two point anchors in section 5. Where the anchors and the pace
conflict, **pace wins** - the anchors were only ever evidence that the re-basing
was cheap, and that evidence is now known to be incomplete.

Done-when 2's parity anchors are downgraded accordingly: keep them as
documentation of the curve's endpoints, not as a gate.

### 13.5 The death penalty interaction is intended (ruling)

`internal/hooks/Death_PlayerCleanup.go:148` does `pick.info.Training -= amount`.
Under U10b-0 that means dying lowers the progression rank, so the lost point is
cheaper to regain than it was to earn.

**Owner ruling: this is a welcome outcome and must not be "fixed".** Death
already carries its own penalty; players who take risks should not be punished
twice. Document the interaction where the death penalty is implemented so a
future reader does not file it as a bug.

### 13.6 Corrections to sections 1-12

- **Section 4** - "`Training` is gear-free and species-normalised by
  construction" is true for players and **false for mobs** until 13.2's pool
  move lands. After it lands, true for both.
- **Section 7** - the vitality rationale is wrong (13.3). The instruction to
  re-derive `strength` and `dexterity` stands, but its measurement procedure
  does not: `event=stat_use` is emitted only by `OnStatUse`
  (`progression.go:243`), and vitality's paths call `TrackStatUse` directly, so
  a log-grep returns zero vitality samples.
- **Section 9** - "nothing is migrated" is false under 13.2.
- **Section 10.1** - "`avgDeviation` becomes identically zero by construction"
  is **false**. `calcExpectedRank` remains use-count-derived, so the metric goes
  *stale and meaningless*, not zero. The remediation (re-derive from the curve
  inverse) is unchanged; only the stated reason was wrong.
- **Section 10.5 / risk 11.3** - the named admin paths are wrong.
  `admin.skillset` writes `Character.Skills`, not `Training`, and there is no
  training-point spend system in Go. The actual `Training` writers are
  `modules/gmcp/gmcp.Mob.go:319` (the mob web editor),
  `internal/hooks/Death_PlayerCleanup.go:148` (13.5), and
  `internal/hooks/PlayerSpawn_HandleJoin.go:115-120` (companion restore).
  Owner ruled cutting or disabling the editor's stat-write path is acceptable;
  it now sets progression difficulty, not just a value.
- **A fourth rank site exists.** Sections 4 and 6 name two; there are four -
  `progression.go:95`, `:172`, `:201` (a duplicate computed only to label a
  debug line), and **`:415` (`regenDamperFactor`)**. Leaving `:415` on the
  counter while `StatProgressionSoftCap` moves 150 -> 50 is destructive, not
  merely stale: the floor `statVal > softCap` becomes true for every stat on
  every character.
- **The Go default must move with the shipped value.**
  `config.balance.progression.go:11` defaults `StatProgressionSoftCap` to 150.
  Left there, every Go test binary validates a curve production never runs, and
  any deployment whose `config.yaml` lacks the key gets runaway progression.

### 13.7 Revised Done-when

Replaces section 12 items 2, 4 and 5; all other items stand.

2. The curve is retuned so trained-points-per-hour roughly matches pre-slice
   behaviour for a representative player (13.4), with the derivation recorded.
   The section 5 anchors are documentation, not a gate.
4. `vitality` is retuned against the **regen** faucet with the damper's new bite
   included, targeting parity with the other stats (13.3); `strength` and
   `dexterity` are re-derived against measured firing rates.
   `ObservedCritProgressionBonus` is set explicitly if the crit path is to carry
   part of vitality's growth.
5. No stat or skill can be sealed: roll resolution is 1,000,000 and
   `ProgressionChanceFloor` is applied at every roll site (13.1), pinned by a
   test that sweeps `Training` well past any reachable value **and** by a test
   using fyttyn's recorded `Training 195`.

New:

14. Mob and companion spawn pools live in `Base`; `Training` is gains-only for
    every character type; mobs are hard-capped at `Training 50` and use
    `MobSkillSoftCap` (13.2).
15. A migration moves existing companion `stat_training` into `Base`, pinned by
    a test that loads a pre-U10b-0 companion save with unchanged stat values and
    a progression rank of 0 (13.2).
16. The death-penalty interaction is documented as intended at
    `Death_PlayerCleanup.go` (13.5).

---

## 14. Owner rulings after the second blind review (2026-08-21)

Plan v2 also failed blind review. The findings clustered in the new Task 6 work
(the mob pool move), which is expected — it was the newest and least-reviewed.
Everything section 13 settled held up on re-verification.

### 14.1 The mob pool move needs `StatPoolTotal()` (ruling: approved)

Moving the spawn pool from `Training` to `Base` (13.2) breaks **five** production
sites that read summed mob `.Training` as "how much creature is there". Three are
player-visible:

| Site | Effect if unaddressed |
|---|---|
| `internal/usercommands/assess.go:47` | every corpse reports "barely a trace of essence" |
| `internal/hooks/companion_summon.go:191` (`corpseRaisePool`) | returns 0 for every corpse, so the `minPool` gate rejects all: **`raise` becomes unusable** |
| `internal/hooks/charm_spell.go:56` | charm resistance loses its pool term, so charm gets easier on every mob |
| `internal/hooks/NewRound_MobRoundTick.go:408` | same, in the charm re-roll contest |
| `modules/gmcp/gmcp.Mob.go:270,320` | the mob web editor reads and writes `.Training` as the mob's stats |

`assess.go:46` states the intent outright ("Sum all stat training values as a
proxy for the creature's total power") and `corpseRaisePool`'s doc records the
cross-system invariant ("the same Training sum `assess` reports, so the two
agree on a corpse's worth"). **Two of the five have tests that pass anyway**,
because they hand-build fixtures rather than spawning a mob.

Reading `Base` (or `Base + Training`) instead does **not** work: after the move
`Base` also carries the species baseline, 600 for a human mob, which exceeds
`assess`'s top band of 500 for every creature in the game.

**Ruling: add `Character.StatPoolTotal()`** in `internal/characters`, returning
the authored pool plus gains:

```
sum(Base) - sum(speciesBase) + sum(Training)
```

All five sites call it. Every existing calibration is preserved exactly, and the
concept those sites were open-coding gets a name. The GMCP editor's stat
**write** path is cut per 13.6.

### 14.2 Pace is held with the multipliers, not the decay knob (ruling: approved)

Section 13.4 required retuning the curve to hold the pre-slice pace. As stated
that is unattainable: `CalculateProgressionChance` reads **one**
`ProgressionDecayBelowCap` for stats and skills, and solving for per-subject
pace demands an 8x spread of values (perception ~0.153, blacksmithing ~0.30,
strength ~0.86, weapon-combat ~1.25). The perception figure alone would require
sustaining 94% of the rank-0 rate out to 40 trained points, which deletes the
decay over the entire range players occupy rather than retuning it.

**The two knobs do different jobs and 13.4 conflated them:**

- `ProgressionDecayBelowCap` sets the curve's **shape** — how much harder a
  trained point is at rank 40 than at rank 0.
- The per-stat / per-skill multiplier sets the **rate**. Under the new model
  `usesToReach(T)` is inversely proportional to it.

**Ruling: fix the decay for shape, then solve the per-subject multipliers for
pace.** That yields per-subject pace matching with no new config, and it folds
into the multiplier re-derivation section 13.3 already requires. Supersedes
13.4's instruction to derive the decay constants against pace.

### 14.3 Regen progression must measure the pool the character can reach

Owner requirement: the regen progression tick must key off the pool **remaining
after reservation**, not the raw pool, or a character farms progression by
standing around with a large reserved pool.

**Verified: both call sites already do this.**
`internal/hooks/NewRound_AutoHeal.go:269-271` (players) and `:365-367` (mobs)
subtract `GetPoolReservation` before passing `max`, and the player-side comment
cites the fyttyn vitality exploit as the reason.

**The exposure is structural, not present.** `OnRegenTick(current, max int, ...)`
trusts its `max` argument, six call sites depend on getting it right, and **no
test pins it**. With companions mirroring players after 13.2, a reserved pet
would farm it too.

**Ruling: make it unreachable.** Change `OnRegenTick` to take a
`characters.Pool` and derive the effective max internally via the existing
`EffectivePoolMax` (`internal/characters/pools.go:313`), so no caller can pass a
raw max. Add a regression test: a character at full effective pool with a large
reservation must produce a progression chance of exactly 0.

**Trap for the implementer:** `EffectivePoolMax`'s doc comment says
"Regeneration is a deliberate exception and still reads the raw max". That refers
to the regen **amount** (`HealthPerRound` / `StaminaPerRound` /
`ConvictionPerRound` in `resources.go`) and remains correct. Only the progression
**ratio** moves to the effective max. Do not change the amount.

### 14.4 Mob skills get a hard cap of 25 (ruling)

Section 13.2 set `MobSkillSoftCap: 20`. A soft cap does not bound anything:
`CalculateProgressionChance` never returns 0 above the cap (it continues
decaying), and 13.1's new floor guarantees it never quantises to 0 either. With
the `MobSkillCap` hard early-return deleted, mob and companion skills would grow
without bound — the opposite of the stated purpose.

**Ruling: `MobSkillTrainingCap: 25` as a hard ceiling**, alongside
`MobSkillSoftCap: 20` for the decay shape. This mirrors the stat side, which
already keeps a hard `MobStatTrainingCap`.

### 14.5 The chance floor does not apply to the regen faucet

13.1 applied `ProgressionChanceFloor` at all three roll sites. That is wrong for
`CheckRegenProgression`: its chance is *deliberately* proportional to pool
depletion, so at 99% of pool it is around 1e-8 and the floor would lift it ~220x
— on a term designed to vanish. It would also partly cancel the regen damper
13.3 requires be included in vitality's derivation, leaving two tasks optimising
the same number in opposite directions.

**Ruling: scope the floor to the two rank-driven sites**
(`ProgressionChanceForStat`, `ProgressionChanceForSkill`).
`CheckRegenProgression` keeps the raised roll resolution but no floor.

---

## 15. Corrections after the third blind review (2026-08-21)

Plan v3 failed blind review as v1 and v2 did, and the findings reached the
content files for the first time. One of them falsifies a premise this spec
states twice. Sections 13 and 14 stand; this section corrects what they did not
know.

### 15.1 Mob templates author `training:` directly — 599 of 641 files

**Sections 4 and 13.2 both assert that a mob's `Training` comes only from the
runtime stat-pool distribution in `internal/mobs/mobs.go`. That is false.**

`grep -rl "training:" _datafiles/world/dogmud/mobs/` returns **599 of 641 mob
files**. They author stat values directly under `character.stats`:

```yaml
# _datafiles/world/dogmud/mobs/stillwater/332-sump_dweller.yaml
  speciesid: 23
  stats:
    strength:  {training: 22}
    perception:{training: 12}
    vitality:  {training: 120}
```

Only **six** `base:` lines exist world-wide (all in `13-loot_goblin.yaml`, and
they merely restate that species' baseline). So mob authoring has always used
`training:` to mean "baseline for this creature", and the engine then adds the
rolled `StatPool` into the same field. A mob's `Training` is therefore
**authored baseline + rolled pool**, and never zero on spawn for 93% of the
roster.

**Seventeen templates already carry a stat at or above 50:**

| authored | mob |
|---|---|
| 100000 | `9614-straw_effigy` (vitality — pseudo-invulnerability for quest 28) |
| 120 | `332-sump_dweller` |
| 100 | `9005-training_post`, `229-windscour_wyrm` |
| 75 | `228-stone_beetle_queen` |
| 70 | `9152-the_unfolded`, `9135-spirit_of_the_swamp` |
| 64 | `9151-unbound_fold` |
| 60 | `9143-alpha_pack_leader`, `9123-stone_blooded_beast`, `374-caravan_wagon`, `359-lars`, `272-chrysalis_phantom` |
| 58 | `9169-bowmaster_skell` |
| 55 | `275-old_edrin` |
| 50 | `9115-bandit_captain`, `61-arena_champion` |

**Consequences had this shipped unamended:**

1. `MobStatTrainingCap: 50` (13.2) would **freeze all seventeen at spawn** —
   precisely the "permanently frozen" failure the owner ruling exists to remove.
2. Every one of the 599 would start partway down the decay curve rather than at
   rank 0, so "mobs mirror players" would be false for 93% of them.
3. Phase C's spawn-pool move would not achieve its purpose, because the authored
   half would still sit in `Training`.

**Resolution (owner, 2026-08-21): fold authored `training:` into `base:` before
anything else.** Per stat, per mob:

```
base_new = (existing base, else the species base for that stat) + authored training
```

then delete the `training:` key. Value is unchanged: the loader hydrates `Base`
from species only when `Base == 0` (`internal/characters/validate.go:50-67`) and
adds authored `training`; afterwards `base:` carries both and hydration
correctly skips a non-zero `Base`.

**The fold must be per-species.** Species baselines are not uniform — they run
from 0 (`20-orb` has no `stats:` block at all) to 6000 (`99-ascended`), and only
species 1 sums to 600. A constant would silently change the power of every mob
not on a human baseline.

**It must also be line-based, not a YAML round-trip.** `9614-straw_effigy`'s
`training: 100000` carries a nine-line comment explaining that no engine
invulnerability flag exists, that high HP is the mechanism, and that the value
was raised from 1000 after a player got stranded on 2026-07-18. A library
round-trip destroys that, and the reason the value looks absurd goes with it.

**A permanent gate follows from this:** no mob template may carry authored
`Training`. Training means gains-since-spawn; a template has not spawned. The
gate is a test, not a one-time migration check, because the mob web editor and
any builder can reintroduce the convention.

**`docs/schemas/mob.md` must change with it** — the authoring convention is now
`base:`.

### 15.2 Section 13.6's ruling on the mob web editor is superseded

13.6 recorded the owner ruling that cutting `modules/gmcp/gmcp.Mob.go`'s
stat-write path was acceptable, on the grounds that under U10b-0 writing
`Training` sets a mob's **progression difficulty** rather than merely its stats.

**15.1's fold removes that hazard.** After it, authored stats live in `Base`, so
the editor writes a value rather than a rank, and the objection no longer
applies.

**Revised ruling: re-point the editor at `Base`, do not cut it.** It is a
legitimate authoring surface for exactly the values 15.1 relocates, and removing
it would delete a builder capability to solve a problem that no longer exists.
Rename its wire field from `StatTraining` to `StatBase` so the payload says what
it means, and update the admin page's client-side reference in the same change.

Note the editor also has a **pre-existing** defect worth tracking separately: it
destroys `#` comments on save (already recorded against the Wave 7 admin-builder
review). Combined with 15.1, saving `9614-straw_effigy` through the editor would
silently delete the comment explaining its 100000.

### 15.3 Section 14.3's instruction is incomplete — `EffectivePoolMax` never returns 0

14.3 ruled that `OnRegenTick` should derive the effective pool internally "via
the existing `EffectivePoolMax`". That is the right shape, but taken literally it
**opens the faucet it was written to close.**

`EffectivePoolMax` (`internal/characters/pools.go:313-318`) is **floored at 1**
and its own doc says so. The current call sites are not:

```go
healthMax := user.Character.HealthMax.Value - user.Character.GetPoolReservation("health", ...)
```

At 100% or greater reservation — reachable with stacked Chrysalis enchantments,
as `EffectivePoolMax`'s comment confirms — the hand-computed max goes `<= 0` and
`OnRegenTick` returns early: no progression, which is correct. Substituting
`EffectivePoolMax` gives `max = 1`, `current = 0`, `ratio = 0`, and therefore the
**maximum** depletion chance, every tick, standing still.

**Amendment: handle the fully-reserved case explicitly.** A character whose
reservation meets or exceeds its pool must get no regen progression, and that
must be pinned by a test that actually constructs such a reservation rather than
skipping when none exists.

### 15.4 Running list of statements in this spec now known false

The header no longer counts them; this is the list. Each is superseded where
noted, and none of sections 1–12 should be relied on without checking here
first.

| Statement | Where | Superseded by |
|---|---|---|
| `Training` is species-normalised by construction | §4 | 13.2, then 15.1 — true only after the fold |
| Rank is computed at two sites | §4, §6 | 13.6 — there are four |
| `vitality: 4.5` is a broken tuning artefact | §7 | 13.3 — it compensates a faucet 100x smaller |
| Nothing is migrated | §9 | 13.2, 15.1 — two save formats and 599 data files |
| `avgDeviation` becomes identically zero | §10.1 | 13.6 — it goes stale, not zero |
| The admin write paths are `admin.skillset` and training-point spend | §10.5, §11.3 | 13.6 — neither writes `Training` |
| The re-basing is free | §5 | 14.2 — pointwise only; pace changes 1.7x to 3.9x |
| A mob's `Training` comes only from the spawn pool | §4, §13.2 | **15.1** |
| Cut the mob editor's stat-write path | §13.6 | **15.2** |
| Derive the effective pool via `EffectivePoolMax` | §14.3 | **15.3** |

### 15.5 Delivery is phased

Three monolithic implementation plans (`d2382b16a`, `224222ba8`, `ede78aea1`)
each failed blind adversarial review. The findings were real every round and
moved outward each time — arithmetic, then the Go blast radius, then the content
files. The conclusion is not that the reviews were harsh but that **this slice
is too large to plan correctly in one document**: each part's discovered facts
change the next part's design, and 15.1 is the clearest case, since the design
rested on a claim about 641 data files that nothing had checked.

The work now ships in phases, each independently shippable and reviewable, with
later phases planned only after their predecessor lands. Index:
`docs/superpowers/plans/2026-08-21-u10b-0-README.md`.

Phase A (15.1's fold plus the two accessors) is **value-neutral by
construction** and changes no progression rate. That is both its safety property
and its verification story.

---

## Appendix A: U10b amendments this forces

Recorded here so the U10b plan revision has a checklist. These are edits to
`2026-08-21-u10b-progression-firing-design.md` and its plan, not U10b-0 work.

- Section 2's uncontested-class definition changes from "damped roll on a
  tracked use" to plain "damped roll". The counter-cost hazard is gone.
- Tasks 2 and 4 shrink substantially; Task 4 Step 4 becomes safe to ship.
- The blind adversarial review of 2026-08-21 found four blockers and twelve
  majors in the U10b plan that are independent of U10b-0 and still stand. The
  survivors after U10b-0 are: the missed direct-call sites (player sneak both
  branches, `picklock.go:153/262`, `behaviortree/actions_progression.go:24`),
  the class guard's AST blind spot on `x.Character.OnStatUse(...)`, the
  invented test fixtures, and the stale `internal/progression/seam_guard_test.go`.
- Owner rulings 2026-08-21 to fold into the U10b spec:
  - A floor-granted save DOES train the defender. Implement by awarding at the
    point `out.Defended = true` is set (`defence_multiplier.go:401`), which
    yields this with no extra condition and correctly excludes `side.ForceCrit`.
  - `search` is EXEMPT from win-only gating; its `rolledAgainstSomething` gate
    is stricter than win-only where it matters (no repeat farming of a stripped
    room) and does not tax beginners. PENDING owner confirmation.
  - `track` IS gated on success (roll >= 125, which is 50% for a fresh
    character). PENDING owner confirmation.
  - `salvage` is EXEMPT; the unconditional item destruction at
    `salvage.go:251` already serves as the brake. PENDING owner confirmation.

