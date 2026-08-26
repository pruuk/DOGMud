# U10b-1: the progression firing convention

Design spec. Written 2026-08-26. Owner-approved shape; see "Decisions taken"
for what was chosen and what was rejected.

Slice U10b of the unified resolution arc, split into **U10b-1** (this spec, the
convention plus the mechanical migration) and **U10b-2** (the three faucets
that change how the game feels). U10b has never been started; it is a different
slice from **U10b-0** (rank from training), which shipped 2026-08-24. The names
are confusable and have caused a wrong answer before, so check the roadmap row
for a shipped marker rather than trusting the name.

## 1. The problem

Progression fires under ten different conventions with no rule. The
2026-08-19 firing audit cataloged them. Three slices have landed since, so its
numbers are stale: the live count is **128 sites across 49 files** (the audit
said 135/52), and two of its five findings were fixed under it by U9.

What survives is that no single rule governs when a progression event fires.
The U9 seam (`internal/progression`) fixed what an event CARRIES and
deliberately deferred when it fires, in as many words:

> Populate a side's Skill/Stat only if that side earns an ordinary event under
> the CALL SITE's existing rules.

That deferred decision is this slice.

## 1.5 THIS SPEC IS THE DECISION RECORD FOR BOTH SLICES

**Split 2026-08-26 (owner).** The work is delivered as two slices. **This spec
covers both** and is the durable record: nothing below is to be relitigated when
the second slice is planned.

| Slice | Delivers | Playtest signal |
|---|---|---|
| **U10b-1** — the firing convention | Best-of, `AwardResolved`, the fraction, `Outcome.Defended`, the defence unification, and the rule wired to **every** site that resolves: melee attacker and defender, both channel defences, concentration, the spell attacker, search, track, forage, craft, salvage, the sixteen skullduggery sites, mob crafters. **Each site keeps its CURRENT resolution.** | One: does failing at something teach you a little? |
| **U10b-1b** — resolution onto the core | The Category B conversions (search x4, track, forage to `AgainstDifficulty`; craft and salvage to `RunWithFloors`), the craft difficulty formula, the authored material tier, the salvage difficulty basis, the two floors, and the hidden-detection fix | Craft feel, search and track odds, the economy |

**Why the line falls there.** The firing rule does not need the conversion.
Craft already resolves (`util.Rand(100) < chance`), so won-or-lost is known and
`AwardResolved(won)` works on today's roll. The same holds for salvage, search,
track and forage. The conversion changes **how** a site resolves; the convention
changes **when** it awards. Splitting on that line keeps each slice's playtest
readable: U10b-1 has one signal, U10b-1b has the balance signals.

⚠️ **Craft, salvage, search, track and forage ARE progression-firing sites and
are in U10b-1.** Only their *difficulty design* moves to U10b-1b. An earlier
sentence in this session said otherwise and was wrong.

Sections below are marked **[1]** or **[1b]** where they belong to one slice.

## 2. Decisions taken

| # | Decision | Slice | Rejected alternative |
|---|---|---|---|
| 1 | U10b splits into the convention and the faucets (U10b-2: mob archers, tick-regen) | both | One slice covering every axis; the playtest could not separate the signals |
| 2 | One event per RESOLVED attempt; a loss pays a fraction | **1** | Strict success-only, which leaves a fumble paying more than an ordinary miss |
| 3 | A single global `ProgressionFailureFraction`, 0.35, sentinel-defaulted | **1** | Per-channel knobs; cost-scaled fractions |
| 4 | Both SKILL and STAT take the fraction | **1** | Scaling the skill only, so a loss paid a full-rate stat roll |
| 5 | **BEST-OF selection**: one event per resolved action, for the single highest-rolling candidate; tiebreak highest skill level, then fixed slice order | **1** | Awarding every candidate, which double-rolls a shared stat and pays a three-defence round three times |
| 6 | **Every candidate is rolled the same way**, `dice.RollStat(stat + skill * SkillWeight)` | **1** | Admitting a candidate with no roll, which ties at zero and lets the tiebreak delete it |
| 7 | Best-of is **self-damping**; do not guard against rich-get-richer | **1** | Adding a corrective, when the winner is already the skill least able to use the event |
| 8 | Progression follows the **FINAL** outcome, so a floored win pays full | **1** | Reading the pre-floor dice |
| 9 | **UNIFY the defence divergence**, win or lose on both paths, awarding the best-quoted defence | **1** | Deferring it, leaving the arc's stated goal unmet at its closing gate |
| 10 | Concentration comes under the rule (**three** sites, including throttle) | **1** | Leaving it success-only, so a broken concentration teaches nothing |
| 11 | The spell attacker comes under the rule: one cast, one event, won if ANY target was hit | **1** | Leaving it on `CastComplete`, where a defended cast pays a full event |
| 12 | Mob crafters award on the same rule as players | **1** | Success-only, reintroducing the inconsistency the slice removes |
| 13 | `picklock` is win-only; it is a pin minigame, not a contest | **1** | Treating it as a resolved contest with a loss branch |
| 14 | First-kill progression is **DELETED**; keep `KD.AddMobKill` | **1** | Keeping it as a named exception |
| 15 | Item procs stay off-core, breadcrumbed | **1** | Claiming them |
| 16 | Delete the stranded mob-follow roll; pursuit becomes authored behavior | **1** | Fixing or keeping the roll |
| 17 | Convert the off-core rolled sites to the contest core | **1b** | Leaving Category B off-core with the arc closing around it |
| 18 | Craft score is `stat + skill * SkillWeight`; `recipe.SkillMinimum` drives the DIFFICULTY | **1b** | A bespoke anchored formula, which cancelled the recipe term and ignored the crafter's stat |
| 19 | Craft difficulty uses an **AUTHORED material tier**, 5 buckets, absent means 1.0, guarded for new materials, backfill owed before U11 | **1b** | Gold value (measured as noise against recipe tier); `rarity_tier` (an inverted stock cap) |
| 20 | **Salvage difficulty IS the item's craft difficulty**, fallback gold value | **1b** | Inventing a base, which no single value satisfies |
| 21 | Craft and salvage keep today's extremes via `contest.RunWithFloors` | **1b** | Deleting the clamps, leaving no mercy band |
| 22 | Fix hidden detection so a hider's score counts in both paths | **1b** | Leaving a flat threshold that ignores the hider |
| 23 | Accepted: `skill_minimum` 50/65 recipes need substantial mastery; mob crafter throughput RISES | **1b** | Retuning to avoid them |

### 2.1 This spec supersedes the 2026-08-21 U10b spec

An earlier spec and a 2405-line plan live on branch
`feature/u10b-progression-firing` (also on origin). **The plan failed its blind
adversarial review with four blockers and must not be executed.** The spec
carries eight owner decisions; U10b-0 then dissolved the use-counter premise
several of them rested on.

What changed, decided 2026-08-26:

| Topic | 2026-08-21 | Now |
|---|---|---|
| Does losing train? | No, "losing no longer trains", accepted twice | **Yes, at a fraction** |
| First-kill progression | Deleted | **Deleted** (unchanged) |
| Regen | Merged into an "uncontested" class | **Deferred to U10b-2** |
| Defence timing | Success only, everywhere | **UNIFIED HERE** (owner, 08-26): best-quoted defence, win or lose, on both paths |
| Class model | Three classes plus a bonus layer | One rule plus a fraction |

The 08-21 argument for success-only was that the contest floors guarantee
everyone wins sometimes and early progression is rapid, so nothing stalls. It
was overturned because it is not monotonic (see 3.2) and because a failed craft
destroys materials and teaches nothing.

Two of its arguments are simply **stale rather than overturned**: it reasons at
length about dropping `look` and `consider` to a trickle, and both award no
progression at all today.

What is **carried forward unchanged**: the `Defended` polarity ruling (3.1.1),
floor-granted saves training the defender (3.1.1), the mob-spell gate asymmetry
(5.5), and the toughen path staying crit-only with no damage-magnitude gate.

## 3. The rule (owner, 2026-08-26)

**Progression resolves BEST-OF, exactly like the defensive rolls.**

> One resolved action produces **one** progression event, for the single
> highest-rolling candidate skill. Tiebreak: highest skill level, then a fixed
> arbitrary order. **Full on success, partial on failure.** Crits and critical
> failures are a **separate channel** and never take part in the selection.
> Progression follows the **final** outcome: a floor that turns a loss into a
> success pays **full**, like any other success; a ceiling that turns a win into
> a failure pays **partial**.

- A win awards `Multiplier: 1.0`; a loss awards
  `Multiplier: ProgressionFailureFraction`.
- `Exceptional` remains the bonus layer, unchanged and unselected.
- Per-skill tuning uses the existing `SkillProgressionMultipliers` map. No
  second fraction is introduced.

### 3.0 Why Best-of settles the hard cases

The game already resolves defence Best-of-all. Progression adopting the same
shape is not an analogy, it is the same mechanism, and it removes three problems
that otherwise need bespoke handling at every site:

1. **Two skills that share a stat cannot double-roll it.** Skullduggery and
   weapon-combat both have dexterity as their primary stat, so a surprise strike
   awarding both would roll dexterity twice. Best-of picks **one** skill, so the
   collision cannot arise. No per-site suppression flag, no two-`Outcome`
   pattern, no rule for an implementer to remember.
2. **Melee defence stops awarding once per defence TYPE.**
   `processDefenderProgression` loops `defenceTypesUsed` and awards each, so a
   defender with dodge, parry and block can take three events in one round (and
   two of those train the same skill, since parry and block both map to
   weapon-combat). Best-of makes it one.
3. **A command that runs several rolls awards once.** `search` runs six checks;
   salvage rolls per ingredient unit. Best-of over the candidates is the same
   rule, not a special case bolted on.

### 3.0.1a EVERY candidate is rolled the same way

🔴 **A candidate without a roll is a defect, not a special case.** Every
candidate carries `dice.RollStat(primaryStat + skillLevel * SkillWeight)`,
composed exactly as any other score in the game. No candidate is admitted with a
score built a different way, and none is admitted with no roll at all.

An earlier draft left this unstated, and the consequence was concrete: in a
surprise attack, **skullduggery is never rolled**. It is read as a LEVEL at
`internal/combat/crit_damage.go:74` to scale the crit multiplier
(`CritDamageMultiplier(attacker.GetSkillLevel(skills.Skullduggery))`). So its
candidate had no roll, tied with weapon-combat at zero, and the level tiebreak
handed the award to weapon-combat **deterministically, every time**, silently
deleting the faucet U10d shipped on 2026-08-25.

Rolling every candidate identically also makes the selection behave. Skullduggery
and weapon-combat share **dexterity** as their primary stat, so the comparison
reduces to the two skill levels plus proportional variance: the higher skill
usually wins, sometimes does not, and by 3.0.2 gains least when it does.

Note the roll used for **selection** need not be a roll that decided the action.
Where a contest already produced one for a candidate, reuse it. Where a skill
contributed without being contested, roll it for selection. The rule is that all
candidates in one selection are commensurable, not that each traces to a
resolution.

### 3.0.2 Best-of is self-damping. Do not add a guard against it.

The obvious objection is rich-get-richer: if only the top roll trains, a
character's strongest skill wins every selection and the weaker one never
progresses.

**It damps itself** (owner, 2026-08-26). The highest roll correlates with the
higher skill, and `CalculateProgressionChance` is monotonically **decreasing**
in rank, so the skill that wins Best-of is the one with the **lowest** chance of
converting that event into a point. Winning the selection with your best skill
buys you the least. The mechanism therefore spends most of its awards on the
skill that can least use them, which is the opposite of a runaway loop.

The second-order worry, that a secondary skill is starved on a multi-candidate
site, is also not serious in practice. Skullduggery is the only skill in that
position and it has **sixteen** other firing sites; being outrolled on a
once-per-combat conditional opener costs it almost nothing.

Worth noting, not worth engineering around. **A blind review may raise this as a
finding; it is dispositioned here, not open.**

### 3.0.1 Floors follow the FINAL outcome

`contest.RunWithFloors` flips an outcome with probability `f`. Progression reads
the flipped result, not the underlying roll:

| Situation | Award |
|---|---|
| Won on the dice | full |
| Lost on the dice, **floor flipped it to a win** | **full**, like any other success |
| Won on the dice, **ceiling flipped it to a loss** | **partial**, like any other failure |
| Lost on the dice | partial |

This keeps one sentence true everywhere: *you get full progression for a
success*. It also means `Floored` continues to suppress the **bonus** channel
(a floor overrode the dice, so no crit or fumble happened) without that
suppression leaking into the ordinary event.

### 3.1 Scope of the rule, by channel

| Channel | Under the rule? |
|---|---|
| Opposed contest (`combat.RunContest`) | Yes |
| Roll vs static difficulty (`contest.AgainstDifficulty`) | Yes |
| Floored static difficulty (`contest.RunWithFloors`) | Yes, on the FINAL outcome (3.0.1) |
| Concentration (`combat.RunConcentrationContest`) | **Yes.** Its THREE sites award spellcasting only on success today, so a broken concentration teaches nothing. It is a contest; it comes under the rule. UNIFY |
| Regen tick (`OnRegenTick`) | **No.** No roll against anything; passive. Goes to U10b-2 |
| Crit / critical-failure | **No.** These are the bonus layer on top of a base event, not base events. No fraction, no separate gate |
| Non-rolled deliberate actions | **No.** Fire once on completion, unscaled. "Success" is vacuous |
| Authored grants (`actGrantProgression`) | **No.** Tutorial scripted grant, deliberate exception |

`OnFirstMobKill` is **deleted**, not excepted. See 5.4.

### 3.1.1 Which side lost: gate on `Defended`, never on `!Success`

**This is the highest-risk line in the slice.** `contest.Result.Success` means
the **attacker** won. `!res.Success` is NOT "the defender won": under
`side.ForceCrit` (a sleeping victim) the attack wins with `Success == false`.
Reconstructing the predicate inverts the entire fraction, and a mirrored test
fake would still pass.

Settled by the owner 2026-08-21 and carried forward unchanged: **gate on
`out.Defended`**, set at `internal/combat/defence_multiplier.go`, and route it
through a single named helper rather than re-deriving it per site. `Margin` is
attack-positive on the channel path and is negated at several places; do not
count on a remembered line number, and note the two stale source comments in
5.8.

A **floor-granted save still trains the defender** (owner, 2026-08-21).
Awarding where `out.Defended` is set delivers that with no extra condition, and
correctly excludes `side.ForceCrit`.

### 3.2 Why "resolved attempt" and not "success"

The roadmap worded the target as "one event per success, with crit and
critical-failure as a separate bonus on top". Read literally, an ordinary
failure pays nothing while a critical failure pays a bonus.

🔴 **An earlier draft of this spec claimed the fraction makes the ordering
`crit > win > loss > fumble`. That is arithmetically FALSE.** Ordinary and bonus
events are applied **additively** (`EventsForContest` is `OrdinaryEvents` plus
`BonusEvents`) and the chance functions are linear in the multiplier below the
clamp, so with the shipped `CritProgressionBonus: 2.0`:

| Outcome | Ordinary | Bonus | Total |
|---|---|---|---|
| Crit (won and crit) | 1.00 | 2.00 | **3.00** |
| Fumble (lost and fumbled) | 0.35 | 2.00 | **2.35** |
| Ordinary win | 1.00 | none | **1.00** |
| Ordinary loss | 0.35 | none | **0.35** |

The real ordering is **crit > fumble > win > loss**, and a fumble pays 2.35x an
ordinary win. The fraction does **not** close the "failing badly teaches more
than failing normally" gap; it moves the ratio from `0 to 2.0` into
`0.35 to 2.35`, so failing badly still teaches about 6.7x more than failing
normally.

**The decision to adopt the fraction stands, on its other legs:**

- A failed craft consumes the materials and teaches nothing (below). That is the
  case the fraction exists to fix, and it does fix it.
- Both shapes are already live and inconsistent: convention 1 is
  clean-hit-only (melee, special moves) while convention 3 is
  roll-happened-win-or-lose (sneak, search, track, flee). One rule has to win,
  and collapsing convention 3 into success-only would be a real nerf to the
  stealth and exploration families.

Whether a fumble **should** out-pay a win is a live question, but it is
pre-existing behaviour that this slice neither creates nor is required to
solve. It belongs to U10b-2 with the other faucets, and is recorded there
rather than silently inherited.

The sharpest case is crafting. On a failed craft the ingredients are consumed
and no progression fires at all:

```go
if roll < chance {
    ...
    user.Character.OnSkillUseScaled(recipe.Skill, user.UserId, craftBonus)
} else {
    user.Character.Items, ... = crafting.ConsumeIngredients(...)  // materials gone
    user.SendText(..., recipe.FailureMessage)                     // nothing learned
}
```

### 3.3 The implementation seam already exists

`ApplyProgression` routes an ordinary event through
`OnSkillUseScaled(ev.Skill, userId, ev.Multiplier)`. The fraction therefore
needs no new machinery: it is a `Multiplier` on the ordinary event.
`OrdinaryEvents` currently hardcodes `Multiplier: 1.0`, so `Outcome` gains a
field naming which side won and `OrdinaryEvents` scales the losing side.

**Both halves take the fraction (owner, 2026-08-26).** The stat half of an
ordinary event is NOT scaled today:

```go
if ev.Stat != "" && ev.Stat != skills.GetSkillPrimaryStat(ev.Skill) {
    c.OnStatUse(ev.Stat, userId)   // full weight, ignores ev.Multiplier
}
```

Left alone, a loss would pay a fractional skill roll and a full stat roll. The
ruling is that **skill and stat both take the fraction** unless implementation
turns up a hard reason otherwise, in which case the reason is written down
rather than left implicit.

This needs a scaled stat entry point, because the problem is wider than the
line above. `OnSkillUseScaled` itself rolls the skill's primary stat at an
**unscaled 1.0** (`internal/characters/progression.go`), so passing a fraction
through it damps the skill and leaves the governing stat at full rate. The
2026-08-21 plan reached the same conclusion and built a separate method for
it; do that rather than threading a multiplier through the existing call.

## 4. The census

Twenty production sites decide an uncertain outcome with a raw roll instead of
the contest core. Only eight carry a breadcrumb, so the roadmap's Category B
row undercounts by more than half. `contest.AgainstDifficulty` was built for
these and has **zero production callers**.

| Owner | Count | Sites |
|---|---|---|
| **U10b-1** | 15 | `actions/search.go` x6, `actions/track.go`, `forager/forage_core.go`, craft x4 (`NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`, `mobs/crafter.go` x2), salvage x3 (`actions/salvage.go`, `crafting/salvage.go` x2) |
| **U12** | 2 | `ChanceToSwitchTarget` roll sites at `NewRound_DoCombat_helpers.go:969` and `usercommands/target.go:170`. Target switching is U12's declared surface |
| **Breadcrumb only** | 2 | `hooks/item_procs.go:71`, `handleMobWeaponPickup` |
| **Deleted** | 1 | `go.go:668-698`, the stranded mob-follow roll. See 5.4 |

Note `ChanceToSwitchTarget` is NOT duplicated code: the formula is properly
shared from `combat/calculations.go:217`. What is duplicated is the roll
pattern around it at both call sites.

## 5. Work

### 5.1 Convert the 15 sites onto the contest core

Search, track and forage go to `contest.AgainstDifficulty`. Craft and salvage go
to `contest.RunWithFloors` with a per-system floor, because they need the mercy
band their clamps provide today (5.1.1.2).

**Do NOT route any of them through `combat.RunContest`.** Its doc comment
reserves it for opposed contests and says static-difficulty rolls are
deliberately unfloored, so unifying them there would be against the arc's own
design rather than in service of it.

Both `AgainstDifficulty` and `RunWithFloors` are on the root guard's watch list
(`contest_floor_guard_test.go`), and `internal/combat/contest_site_guard_test.go`
requires every contest site to be owned. Each new caller therefore needs a
`guardedRollExemptions` entry and a `contestSiteOwners` entry, both with a
written reason. That is the guards working, not an obstacle: budget for it in
the same commit as each conversion rather than discovering it five commits
later.

**These conversions are NOT no-ops.** An earlier draft of this spec claimed they
were, in the style of U1 through U5. That was wrong. `contest.Run` rolls the
difficulty side as well as the attacker:

```go
stdDev := dice.StdDevFor(atkScore)
attackRoll := dice.Roll(atkScore, stdDev)
for _, e := range entries {
    defenseRoll := dice.Roll(e.Score, stdDev)   // the threshold is rolled too
```

Today `search.go` compares one roll against a FIXED number. Adding a second
roll widens the distribution and compresses outcomes toward 50%. At the shipped
`RollSpread: 0.15`, against the 125 threshold:

| Search score | Today | Converted |
|---|---|---|
| 100 | 4.8% | 11.9% |
| 125 | 50% | 50% |
| 150 | 86.7% | 78.4% |
| 175 | 97.2% | 91.1% |

Accepted deliberately: a weak searcher improves and an expert loses their
near-certainty, which is the same crit, fumble and margin treatment every other
contest gets, and is what `AgainstDifficulty`'s own doc comment argues for.

Note also that `stdDev` derives from the ATTACKER's score and is reused for the
difficulty roll, so success depends on the RATIO of difficulty to score, never
on the gap. Every formula below is built as a ratio for that reason.

### 5.1.1 Craft and salvage need a difficulty basis designed, not migrated

Craft and salvage are not `RollStat`-vs-threshold. They are flat uniform
percentages, so there is nothing to contest against:

```go
chance := crafting.CalcSuccessChance(sl, recipe.SkillMinimum)  // a PERCENT
roll := util.Rand(100)
if roll < chance {
```

**Craft is an ordinary contest** (owner, 2026-08-26). The crafter's score is
composed the way every other score in the game is composed, and the recipe
supplies the difficulty:

```
craftScore      = primaryStat + craftSkill * SkillWeight          // SkillWeight 5, as everywhere
craftDifficulty = (CraftBaseDifficulty + recipe.SkillMinimum * CraftSkillMinWeight)
                  * materialTierMult
```

with `CraftBaseDifficulty` **100**, `CraftSkillMinWeight` **5**, and
`materialTierMult` running **0.75 to 1.25** over the tier of the dearest
ingredient.

**`CraftBaseDifficulty` is 100 because 100 is the human stat baseline.** The
difficulty therefore reads as "a baseline human holding exactly the recipe's
minimum skill", and a crafter who is exactly that scores
`100 + min*5` against a difficulty of `100 + min*5`, which is 50%. That is
today's shipped `CraftingBaseSuccessChance: 50`, reproduced without a special
case.

What this buys, all four at once:

| Property | |
|---|---|
| 50/50 at the minimum for a baseline crafter | yes |
| `recipe.SkillMinimum` genuinely drives difficulty | yes |
| Advanced recipes take more mastery to become routine | yes |
| The crafter's **stat** matters, as in every other contest | yes |

Mastery at neutral tier, stat 100:

| Levels above minimum | `skill_minimum: 0` | `skill_minimum: 40` |
|---|---|---|
| 0 | 50% | 50% |
| 5 | 83% | 64% |
| 9 | 93% | 73% |
| 20 | 99% | 88% |
| 30 | 99% | 94% |

A masterwork recipe needs roughly thirty levels above its minimum to become
routine where a simple one needs nine. That is the intent: you rarely spoil the
fifteenth masterwork helm you have ever made, and you are a coin flip on your
first.

### 5.1.1.1 Do not invent a bespoke score formula

🔴 An earlier draft of this spec composed the score as
`(CraftBaseDifficulty + SkillMinimum * CraftSkillMinWeight) * (1 + levels * 0.05)`,
putting the recipe's anchor on **both** sides. That cancelled exactly, which
made `CraftSkillMinWeight` a knob that could never change an outcome, removed
`SkillMinimum` from the odds entirely, and **ignored the crafter's stat
altogether**. It also left material tier as the only difficulty signal, so with
no materials tiered yet every recipe in the game would have had identical odds.

The correction is not a cleverer formula. It is to compose the score the way
the rest of the game composes scores: `stat + skill * SkillWeight`. Anything
else is a special case that has to justify itself, and this one could not.

### 5.1.1.2 The mercy band: floors, not deleted clamps

🔴 An earlier draft retired `CraftingMinSuccessChance` (5),
`CraftingMaxSuccessChance` (95), `SalvageMinChance` (0.15) and
`SalvageMaxChance` (0.85) "in favour of `ContestFloor`". **That was false.**
`contest.AgainstDifficulty` calls `Run`, never `RunWithFloors`, so it applies
**no floor at all**, and `combat.RunContest`'s doc comment forbids routing
static-difficulty rolls through it:

> SCOPE: opposed contests only. Static-difficulty rolls -- search, track,
> forage, concentration -- are roadmap categories B and C and are deliberately
> unfloored. Do not route them here to "unify" them.

Deleting the clamps would have removed the mercy band and replaced it with
nothing. Uncapped salvage is the worse half: at salvage skill 50 the
craft-then-salvage loop would retain about 99.9% of materials against 80.75%
today, roughly a **250x reduction in the crafting material sink**, on the exact
loop that farms crafting skill.

Both existing clamp pairs are **symmetric about 50%**, so each is exactly a
contest floor. Call `contest.RunWithFloors` directly with a per-system floor:

| System | Today's clamp | Floor that reproduces it |
|---|---|---|
| Craft | 5% / 95% | `CraftFloor` **0.05** |
| Salvage | 15% / 85% | `SalvageFloor` **0.15** |

This keeps today's behaviour at both extremes, restores salvage's cap, and
expresses the clamp as a flip probability rather than a hard clip.
`RunWithFloors` is on the root guard's watch list, so each new caller needs a
`guardedRollExemptions` entry **with a written reason** (see 5.1.3).

### 5.1.1.3 Ingredient resolution must be DETERMINISTIC

🔴 `items.FindSpecByComponentTag` iterates a **Go map**, whose order is
randomised. Four items share `component_tag: bottle` (Clay Flask, Glass Vial,
Sealed Phial, Crystalline Decanter). Resolving a recipe's ingredient tag
through it would therefore re-roll the material tier on **every attempt**, so
an alchemy craft's success rate would swing between roughly 50% and 88% with no
cause a player could observe.

It is also wrong in principle: difficulty would ride on the recipe's declared
**tag** rather than on the item the player actually consumed, which contradicts
the player-facing claim that difficulty depends on the materials you are
working with.

Resolve the **concrete items that will be consumed**, deterministically, before
the roll, and use those both for the tier and for consumption, so the roll and
the consumption cannot disagree.

### 5.1.1.4 Salvage difficulty IS the item's craft difficulty

**As hard to unmake as it was to make** (owner, 2026-08-26).

```
salvageScore      = perception + salvageSkill * SkillWeight
salvageDifficulty = CraftDifficulty(recipe.SkillMinimum, recipeTierMult)
                    where recipe = crafting.GetRecipeByOutputItemId(item.ItemId)
```

Fallback to a gold-value-derived difficulty when the item has no recipe.
**That path is unreachable today**: zero items in
`_datafiles/world/dogmud/items/` carry `salvage_returns`, so only crafted items
are salvageable. Build the fallback anyway, but do not tune it.

This resolves two problems an earlier draft could not:

1. **No invented base.** A draft asked the implementer to pick a
   `SalvageDifficulty` base and a blind review proved **no single value works**:
   reproducing today's rate at skill 0 needs 123, at skill 15 needs ~172, at
   skill 25 needs ~199. Today's curve is `0.15 + 0.70*sqrt(s/50)` and a contest
   is a normal CDF of a ratio, which saturates earlier; the shapes cannot be
   reconciled. Deriving difficulty from the recipe stops trying: the curve is
   replaced deliberately rather than approximated badly.
2. **No tag-to-tier resolver.** A draft needed the tier of a material that does
   not exist yet (it is being *created* by the salvage), and the only tag
   resolver is `items.FindSpecByComponentTag`, which iterates a Go map and is
   forbidden by 5.1.1.3. Reading the recipe of the item **being consumed** needs
   no such lookup, and `GetRecipeByOutputItemId` is already indexed.

Behaviour, with both floors applied:

| Salvager | Item | Retention per craft-then-salvage cycle |
|---|---|---|
| Master, nine levels above a `skill_minimum: 0` recipe | its own work | ~0.78 (today ~0.81) |
| Novice, `skill_minimum: 0` | its own work | ~0.25 (today ~0.12) |
| Novice salvager | a `skill_minimum: 65` item | near-total loss |

The master case lands close to today. **Novices retain roughly twice as much**,
which is the gentler direction but should be watched in the playtest as a
material-sink signal.

### 5.1.2 Material tier is AUTHORED, not derived from gold value

Material tier is a **modifier** on a difficulty that `recipe.SkillMinimum`
already carries (5.1.1). That matters twice: an authoring error moves a recipe
by one bucket rather than defining it outright, and the slice can ship before
any material is tiered without every recipe collapsing to identical odds.

**On day one no material carries a tier**, so every multiplier is the neutral
1.0 and difficulty is `100 + SkillMinimum * 5`. That is a coherent shipping
state, not a broken one. Say so in the playtest goals: signal 3 asks whether
difficulty tracks the recipe, and the material half only comes alive as the
backfill lands.

**Gold value was tried and rejected** (owner, 2026-08-26). Measured across all
126 recipes, the 0.75 to 1.25 band appears inside almost every `skill_minimum`
bucket: it is noise, not signal. Named offenders, computed under the retired
formula where tier was the ONLY signal:

| Recipe | `skill_minimum` | Dearest ingredient | Odds at the minimum |
|---|---|---|---|
| Reduction Base | 50, top of alchemy | 3 gold | 88% |
| Chainmail Vest | 25 | 2 gold | 88% |
| Masterwork Plate Helm | 38 | 8 gold | 72% |
| Hearty Stew | 5, novice cooking | 50 gold | 28% |

A novice cook would fail Hearty Stew 72% of the time while a near-master smith
forged a Masterwork Plate Helm 72% of the time. Enchanting is uniformly
penalised, because chrysalis materials are the dearest in the game, so every
enchant recipe would fall from 50% at its minimum to between 12% and 28%.

Gold value is also a **stat-derived price**, not a rarity measure:
`AutoCalculateValue` prices mitigation at `(p^2+m^2+c^2)*17`, which spreads the
eight apex materials across a 60x range despite their being one tier.

**`ItemSpec.RarityTier` cannot be used either.** It is a vendor stock cap
(`shops.EffectiveMaxStock` computes `RarityTier * stockMultiplier`), so a HIGHER
value means MORE common, inverting the name. Its doc comment claims the set is
50/40/30/20/10 while the data holds 19 distinct values including 78, 82, 84, 86,
88 and 90, and only 153 of 208 material files carry it. Renormalising it is off
the table for this slice: it would move vendor stock levels world-wide at the
same time as craft difficulty.

**The design (owner, 2026-08-26):** a NEW authored field on material items,
separate from `RarityTier`, with **five buckets** shaped like the stock tiers.
Five buckets map onto the 0.75 to 1.25 band.

**Absent tier means multiplier 1.0**, not an error and not the cheapest tier. A
material with no authored tier is neutral, so partial coverage cannot silently
make a recipe easy.

**A guard fails any NEW material authored without an explicit tier.** Pin the
set of item files currently missing it, in the same style as this slice's other
guards; a new file missing the field then fails, while existing ones are
grandfathered.

🔴 **The backfill is OWED BEFORE THE ARC CLOSES** (owner, 2026-08-26). It is not
part of this slice, but it is not open-ended either: **it must land before
U11**, which is the arc's closing gate and runs last. Until it does, the
material half of craft difficulty is inert and the crafting playtest cannot
answer signal 3.

Concretely, U11 must not be declared done while any material in the
grandfathered list still lacks a tier. Express that as a **test**, not as prose
in a roadmap: U11's own row already carries the obligation to ship its "Done
when" list as tests, precisely because U6 was declared done with two criteria
false and nothing failed. Shrink the grandfathered set to empty and the guard
written in this slice becomes the backfill's completion check for free.

### 5.2 Fix hidden detection

Two of `search.go`'s checks answer "does the observer spot the hider?" against
a flat threshold of 135 that **never reads the hider's sneak score**, while
`usercommands/go.go` resolves the same question as a proper opposed contest. A
hider's skill decides the outcome in one path and is ignored in the other. Mobs
reach the broken path too, via `behaviortree/actions_scout.go`'s `actTrySearch`.

Reconcile the two implementations onto the opposed form.

**This is the slice's one deliberate behaviour change.** The playtest must
separate "the convention moved" from "stealth got better against searchers".

### 5.3 Route the 16 skullduggery sites onto the U9 seam

`actions/steal.go` x3, `actions/plant.go` x3, `actions/shadow.go` x2,
`usercommands/skill.skullduggery.sneak.go` x2, `usercommands/picklock.go` x2,
`actions/defuse.go`, `usercommands/throw.go`, `mobcommands/flee.go`,
`hooks/NewRound_DoCombat_helpers.go`.

Two traps:

1. **`Outcome` holds exactly one `AttackerSkill`.** Awarding both a combat
   skill and skullduggery for one action needs TWO Outcomes, not one.
2. **`SkillPrimaryStats["skullduggery"] == "dexterity"`**, the same as
   weapon-combat. Awarding both rolls dexterity twice on top of the
   unconditional attacker stat gain.

### 5.4 Deletions

- **`go.go:668-698`**, the stranded hostile mob-follow roll. It sits on the
  ordinary movement path, which `go.go:125` refuses outright while the player
  is in combat, and `go.go:240` says so: "this is the gate that makes flee the
  only player-initiated disengage while in combat." A successful flee calls
  `EndAggro` then `MoveToRoom` and commands only charmed mobs to follow. The
  roll's only reachable window is a stale-aggro edge case. Pursuit is being
  redesigned as authored mob behavior in the behavior unification arc, so this
  is dead weight, not a feature to preserve.
- **`OnCriticalSuccess` and `OnCriticalFailure`**, which have zero production
  references and survive only as stub methods on fake actors in NINE test
  files. Remove both from the `progression/seam_guard_test.go` allow-list,
  which currently vouches for two symbols that do not exist.
- **First-kill-of-a-type progression** (owner, ruled 2026-08-21 and reaffirmed
  2026-08-26). Delete `Character.OnFirstMobKill`, both call sites in
  `hooks/Death_MobKillCredit.go` (killer and party members), and the
  player-facing message "Defeating a new foe hones your combat instincts!".
  **Keep `KD.AddMobKill`** and the kill tracking around it: that bookkeeping
  feeds the kill and bestiary displays and is not progression.

### 5.4.1 Four sites bypass the seam entirely

These call `CheckSkillProgression` directly, so they touch no entry point and
no guard would see them. They also never call `TrackSkillUse`, which is the
mechanism behind the fyttyn exploit:

- `usercommands/skill.skullduggery.sneak.go` at the **failure branch** and its
  sibling success site
- `usercommands/picklock.go`, two sites
- `behaviortree/actions_progression.go` (`actGrantProgression`), multiplier
  1000.0, a guaranteed unclassed grant. This one is a **deliberate authored
  exception** per 3.1 and stays, but it must be named in the guard's allow-list
  rather than merely escaping notice.

The 2026-08-21 plan missed the player sneak sites and found only
`mobcommands/sneak.go`. Do not repeat that.

### 5.5 Mob-spell gate asymmetry

Carried forward from the 2026-08-21 spec, which had it in scope and which this
spec's first draft dropped. The player spell path applies a self-cast
progression penalty, zeroes progression for an area cast that found no targets,
and gates on `spellBonus > 0`. The mob path has **none of the three** and fires
unconditionally on `CastComplete`. That is a firing-condition inconsistency and
belongs here: the mob path adopts the player path's gates.

### 5.5.0 UNIFY the defence divergence (owner, 2026-08-26)

The melee and channel defence paths disagree today, and the source says so:

> U9: the ordinary defence award is unchanged in WHEN it fires, whenever the
> contest ran, win or lose, which is what this path has always done and is
> deliberately different from melee's defence-used gate. That divergence is
> recorded in the firing audit and is U10b's to reconcile.

An earlier draft deferred this to U10b-2. **The owner ruled 2026-08-26 to unify
it here.** That is the point of the arc.

**The unified rule: award the defence that was QUOTED, win or lose.** The
channel path already does this. Melee awards only a defence that *won*
(`defenceTypesUsed` collects `DefenseUsed != DefenseNone`, which is stamped
only in the defended-win branches), so melee moves to the channel's shape:
`runBestOfAllDefense` already rolls every available defence and picks a best,
so a best-quoted defence exists even in a round where nothing landed.

This resolves a problem that has no other answer: under a win-only reading, a
round in which the defender was hit by everything has **no skill to name**, and
`progression.Event` forbids an empty `Skill`. Awarding the best-quoted defence
gives the fraction a target on every resolved round.

🔴 **Know the direction of travel on each path**, because they are opposite and
the re-solve depends on it:

| Path | Today | After | Change |
|---|---|---|---|
| Channel defence (dodge, parry, block, quell, defy) | full event win or lose | full on a win, fraction on a loss | **a cut** |
| Melee defence | award only when a defence won | best-quoted defence, win or lose | **a gain** |
| Melee attacker (per clean hit) | 0.5752 events/swing | 0.7239 | **+26%** |

The +26% attacker figure is sound. **Any defender-side figure computed as a
uniform +47% is wrong**: it assumes today's defender award is win-only, which is
false on the channel path. Melee's defender award also fires **once per round**,
not per swing, so its inflation shrinks as swing count rises (roughly +17% at
two swings, +8% at three). Re-solve each defence skill against its own
measured rate, not against a single headline number.

### 5.5.1 WIRE THE RULE. The seam is not the feature.

🔴 The first draft of the plan converted fifteen roll sites and **never applied
the firing rule to any of them**, and added `Outcome.Defended` without
populating it anywhere in production. The slice would have shipped its headline
rule wired to nothing, with every gate green.

Each of these is a required deliverable, not an implication:

1. **Every converted site awards under the rule.** `search.go`, `track.go`,
   `forage_core.go`, the four craft sites and the salvage sites must award
   through `OrdinaryEventsScaled`, so a resolved loss pays the fraction. The
   failed craft in 3.2 is the case this whole slice is justified by; if it still
   teaches nothing at the end, the slice failed.
2. **`Outcome.Defended` is populated from combat's `out.Defended`** on the melee
   path, the five-defence channel path and the spell path. Without this the
   fraction reaches only the sites converted by hand.
3. **Mob crafters award on the same rule as players.** `mobs/crafter.go` awards
   inside its success branch only. Leaving it there reintroduces exactly the
   firing-condition inconsistency this slice exists to remove.
4. **The spell attacker comes under the rule.** Today
   `NewRound_DoCombat_helpers.go:385` awards `OnSkillUseScaled(castSkill, ...)`
   on `CastComplete` gated only on `spellBonus > 0`, so a **defended cast still
   pays a full event**. It builds no `Outcome` at all.

   ⚠️ **A multi-target cast is not a special case.** The apparent obstacle is
   that several targets give several outcomes and therefore no single
   `Defended`. The per-command rule (5.5.2) already answers it: **one cast is
   one resolved action, so one event, won if ANY target was hit.** That is the
   same collapse `search` gets for its six rolls. Do not invent a per-target
   award or a second rule.

### 5.5.2 One command, many rolls: collapse to ONE event

`search.go` fires **one** `OnSkillUse(Search)` per invocation today, gated on
`rolledAgainstSomething`, no matter how many of its six checks ran. Under a
naive reading of "one event per resolved roll" a room holding a hidden exit, a
hidden container, a stashed item, a hidden noun and a hidden mob would pay
**five** events where it pays one.

This is not a special case: it is Best-of (3.0) applied to a command's own
rolls. Where a command runs several (search's six, track's grades, salvage's
per-unit rolls), the candidates are those rolls and Best-of picks one. A command
that resolved at least one roll and won none pays the fraction; a command that
won any pays full weight.

Today's sites are already close to this. `search.go:242` awards once per
invocation gated on `rolledAgainstSomething`; `track.go:128` and
`salvage.go:166`/`:252` award once unconditionally. **They award a FULL event
today, win or lose**, so moving them under the rule pays them full on a win and
the fraction on a loss. Record that direction honestly in 6; it is a
redistribution, not a universal gain.

### 5.5.3 Unscaled side effects of `OnSkillUseScaled`

`OnSkillUseScaled` does three things besides rolling the skill: it tracks the
use, it **grants mutation cluster drift** at a full unscaled
`MutationAffinityPerSkillUse`, and it **emits the `SkillUsed` quest event**.

Awarding on losses roughly doubles how often it is called. Untouched, that
roughly doubles **mutation acquisition**, a major character-identity system
whose rate was deliberately tuned for "sustained play, not one fight", and it
turns every "use this skill N times" quest into "fail at it N times".

Both must scale with the event's multiplier, or be moved out of the scaled path
so a fractional award does not pay a full side effect.

### 5.6 The config knob

`ProgressionFailureFraction ConfigFloat` in `internal/configs/config.balance.go`
beside `CritProgressionBonus`, defaulted in `validateProgression()`.

**The defaulting idiom in that file would silently break it.** An absent YAML
key unmarshals to `0`, which is neither `< 0` nor `> 1.0`:

```go
if b.ProgressionFailureFraction < 0 || b.ProgressionFailureFraction > 1.0 {
    b.ProgressionFailureFraction = 0.35   // NEVER FIRES on an absent key
}
```

The knob would ship at zero, failure would pay nothing, and the whole slice
would look inert. Use a negative sentinel (`-1` meaning unset) or an explicit
presence check, and add a test that a config **omitting** the key still lands
on the intended default.

Only this one knob is fixed here. The wider sweep of this pattern belongs to
the separate `config.yaml` audit project and must keep its scope.

### 5.7 Guard test

A test that fails when a new rolled site does not route through the seam.
`internal/progression/seam_guard_test.go` is the closest prior art and is the
one to extend; the 2026-08-21 plan never mentioned it and would have left it
stale.

🔴 **The existing AST helper cannot fail for the bug it names.** In
`contest_site_guard_test.go` the walker does
`pkg, ok := v.X.(*ast.Ident); if !ok { return true }`, and bails. But
`x.Character.OnStatUse(...)` is a **selector on a selector**, not an identifier,
and that is the dominant call shape in this code. A guard written on that helper
ships undelivered and green. Fix the helper first and prove it fails before it
passes.

Assert the guard against real fixtures. The 2026-08-21 plan referenced
`repoRootChdir` and `newTestCharacter`, **neither of which exists** in the repo;
the real ones are `newProgressionTestCharacter` and `configs.SetConfigForTest`.

### 5.8 Breadcrumbs, stale comments, and roadmap correction

Add `NOTE` comments to the 12 untracked sites. Correct the roadmap's Category B
row to say 20 rather than 8, and hand the two `ChanceToSwitchTarget` sites to
U12 explicitly.

Two source comments in `internal/combat/defence_multiplier.go` are wrong and
misled the last plan. Fix them while in the file rather than trusting them:

1. A comment claims a forced-crit defence "was still progressed exactly as on
   the melee path". **It is not** progressed on the melee path.
2. A comment cites a `Margin` negation at one line number; the real negations
   are at several other lines. Re-derive rather than quoting it.

Also re-anchor `TestCritReceivedProgression_DecaysWithRank`, which is
load-bearing (it pins the owner's decay condition via the real
`statProgressionChance`) but is named and documented after `OnCritReceived`,
a symbol this slice removes. Re-anchor it **first**, or the next cleanup sweep
silently un-pins the condition.

## 6. Risk

**This slice is a REDISTRIBUTION, not a universal gain.** The owner has ruled
that the classification does not change the work, and it does not, but it
changes every number fed to the re-solve, so it is recorded here rather than
assumed.

Direction of travel, computed from the clean-hit rate 0.5752 in the analytics
log. **Do not quote a single headline figure**: three of these have the
opposite sign to the first.

| Path | Today | Direction |
|---|---|---|
| Melee attacker | awards per clean hit only | **+26%** (0.5752 to 0.7239 per swing) |
| Melee defender, 1 defence type | awards only a defence that WON | **+47%** |
| Melee defender, 3 types, 3 swings | awards once **per TYPE**, so up to three events | **a cut**, roughly −21% |
| Channel defender (quell, defy) | awards a FULL event win or lose | **a cut**, roughly −26% to −58% |
| search, track, steal, sneak, salvage | award a FULL event win or lose | **a cut**, roughly −32% to −57% |

Two consequences of that table:

- **The melee defender's sign depends on gear.** A defender with one defence
  type gains; one with dodge, parry and block loses, because Best-of collapses
  three events into one. Note parry and block **both** map to weapon-combat, so
  today a shield user can take two weapon-combat rolls in a single round.
- **Every fitted multiplier is invalidated**, not just
  `SkillProgressionMultipliers[Skullduggery] = 0.83`. Re-solve weapon-combat,
  unarmed-combat (which is what **dodge** trains), spellcasting (**quell**),
  rhetoric (**defy**), salvage and search too, or record why one is exempt.

⚠️ **`combat-analytics.jsonl` is combat-only.** Search, track, forage, salvage
and skullduggery have **no measurement basis**, so their multipliers are set by
judgement and confirmed in the playtest, not solved. Say so in the commit rather
than implying they were fitted.

Measure before and after against `_datafiles/logs/combat-analytics.jsonl`
(96,723 events) via `tools/balance/read_combat_analytics.py`. The buffer is
cumulative; never sum flush lines.

**This slice is not a no-op in four independent ways**, and the playtest must
report on each separately rather than as one impression:

1. **The convention move.** Does failing at something now visibly teach a
   little?
2. **The stealth change** (5.2). Is a skilled hider now meaningfully harder to
   find, judged from both sides?
3. **Crafting feel** (5.1.1). Does difficulty track the recipe? Does an advanced
   recipe need more mastery than a simple one? **No material is tiered at
   ship**, so the material half is neutral until the backfill (5.1.2).
4. **Search, track and forage odds** (5.1). Weak searchers improve sharply and
   experts lose their near-certainty. Hidden exits and containers are permanent
   one-shot unlocks (`AddDiscovery`), so this is a pacing change rather than an
   ongoing faucet, but check it against any quest that assumes searching is hard.

### 6.1 Accepted consequences (owner, 2026-08-26)

Both were found by review, both are ruled acceptable, and both belong in the
playtest goals so a tester reports rather than re-discovers them.

- **The 26 recipes at `skill_minimum` 50 or 65 need a lot of mastery.** Success
  is a ratio, so the skill needed above the minimum scales with
  `100 + 5*min`. At stat 100 a `skill_minimum: 65` recipe reads about 66% nine
  levels above its minimum where a `skill_minimum: 0` recipe reads about 89%.
  Stat carries real weight here, so a high-stat crafter closes much of that gap.
- **Mob crafter throughput goes UP, not down.** Crafter mobs are authored at
  stat 104 to 115 with `blacksmithing: 1`, so their score is comfortably
  positive and their success rate rises roughly 1.4x to 3x in the
  `skill_minimum` 3 to 10 band. Dynamic pricing keys on the stock-to-restock
  ratio, so the risk is crafted goods drifting toward the **0.25x floor**, not
  the ceiling. An earlier draft recorded this backwards.

**Also watch:** the craft-then-salvage material sink, and passive defence
training, which becomes the cheapest repeatable losing action now that a lost
round pays.

## 7. Explicitly out of scope

- Two faucets: mob archer
  ranged-combat progression and the tick-regen route. Both U10b-2. The defence
  divergence was the third; the owner moved it INTO this slice (5.5.0).
- Item proc gating (`hooks/item_procs.go:71`). Breadcrumbed, not converted.
- Target switching. U12's surface.
- Mob pursuit as a feature. The behavior unification arc.
- The wider `config.yaml` validator sweep. Its own project.

## 8. Done when

1. Every rolled site routes through the seam, and a guard test fails a new one
   that does not.
2. A config omitting `ProgressionFailureFraction` still yields the intended
   default, proven by test.
3. A hider's sneak score affects both hidden-detection paths, proven by test.
4. Every fitted skill multiplier is re-solved against measured rates, with the
   before and after recorded, or its exemption justified in the commit.
5. A **failed craft awards the fraction** rather than teaching nothing, proven
   by test. This is the case the slice is justified by (3.2); if it does not
   hold, nothing else matters.
6. `Outcome.Defended` is populated in production on the melee, channel and
   spell paths, proven by test, not merely added to the struct.
7. A craft's difficulty is **deterministic** across repeated attempts with the
   same recipe and materials, proven by test (5.1.1.3).
8. Craft and salvage keep today's extremes: a floor near 5% and 95% for craft,
   15% and 85% for salvage (5.1.1.2).
9. A new material authored without an explicit tier **fails a guard**; an
   existing one without a tier resolves to multiplier 1.0 (5.1.2).
10. The guard's AST helper is proven to FAIL before it is made to pass, on a
    real `x.Character.OnStatUse(...)` selector-on-selector call.
11. No production site reaches progression without passing an entry point the
    guard can see, and every deliberate exception is named in an allow-list
    with a written reason rather than merely escaping notice.
12. A command that runs several rolls awards **one** event, not one per roll
    (5.5.2), proven by test on `search`.
13. Mutation cluster drift and the `SkillUsed` quest event scale with the
    event's multiplier (5.5.3), proven by test.
14. The adversarial playtest reports on the four signals in section 6
    separately.
