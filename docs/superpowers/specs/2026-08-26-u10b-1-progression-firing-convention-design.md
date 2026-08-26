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

## 2. Decisions taken

| # | Decision | Rejected alternative |
|---|---|---|
| 1 | Split U10b into U10b-1 (convention) and U10b-2 (faucets) | One slice covering all eight axes; the playtest could not have separated the signals |
| 2 | One event per RESOLVED attempt; a loss pays a fraction | Strict success-only, which leaves fumble paying more than an ordinary miss |
| 3 | A single global `ProgressionFailureFraction` | Per-channel knobs; cost-scaled fractions |
| 4 | Convert all 15 off-core rolled sites AND fix hidden detection now | Deferring the hidden-detection fix to U10b-2 |
| 5 | Delete the stranded mob-follow roll; pursuit becomes authored behavior | Fixing or keeping the roll |
| 6 | Item procs stay off-core, breadcrumbed | Claiming them in this slice |
| 7 | First-kill progression is DELETED | Keeping it as a named exception |
| 8 | Convert all 15, designing a craft/salvage difficulty basis | Converting only the 8 RollStat sites; converting nothing |
| 9 | Craft difficulty comes from an AUTHORED material tier, 5 buckets; absent means 1.0, and a guard blocks new untiered materials | Gold value (measured as noise against recipe tier); `rarity_tier` (an inverted stock cap with 26% gaps) |
| 10 | `recipe.SkillMinimum` is deliberately absent from the odds; it gates discovery and attempts | Adding it back, which a blind review wrongly demanded |
| 11 | Craft and salvage keep today's extremes via `contest.RunWithFloors` | Deleting the clamps, which would have left no mercy band at all |

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
| Defence timing | Success only, everywhere | **Deferred to U10b-2** |
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

## 3. The rule

**One `progression.Outcome` per resolved roll.**

- A win populates that side's `Skill`/`Stat` at `Multiplier: 1.0`.
- A loss populates them at `Multiplier: ProgressionFailureFraction`.
- `Exceptional` remains the bonus layer on top, unchanged.
- `Floored` still suppresses bonuses, unchanged.
- Per-skill tuning uses the existing `SkillProgressionMultipliers` map. No
  second fraction is introduced.

### 3.1 Scope of the rule, by channel

| Channel | Under the rule? |
|---|---|
| Opposed contest (`combat.RunContest`) | Yes |
| Roll vs static difficulty (`contest.AgainstDifficulty`) | Yes |
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

**Craft difficulty comes from the materials** (owner, 2026-08-26):

```
craftDifficulty = CraftBaseDifficulty * materialTierMult
craftScore      = CraftBaseDifficulty * (1 + levelsAboveMinimum * CraftSkillWeightPct)
```

with `CraftBaseDifficulty` 100, `CraftSkillWeightPct` 0.05, and
`materialTierMult` running **0.75 to 1.25** over the tier of the dearest
ingredient. `levelsAboveMinimum` is `skillLevel - recipe.SkillMinimum`.

Two properties this buys:

1. At `skillLevel == recipe.SkillMinimum` with mid-tier materials, score equals
   difficulty, so the contest is 50%. That is exactly today's shipped
   `CraftingBaseSuccessChance: 50`.
2. `CraftSkillWeightPct` is a PERCENTAGE, not an addend, so nine levels above
   the minimum reads the same on every recipe. A flat `+5` would pay 92.8% on a
   `skill_minimum: 0` recipe and 73% on a `skill_minimum: 40` one, because
   success depends on the ratio (5.1).

### 5.1.1.1 `recipe.SkillMinimum` is deliberately absent from the odds

It is **not** an oversight that `SkillMinimum` does not appear above, and it
must not be added back.

Requiring both properties in 5.1.1 forces it out. Property 1 says
`difficulty(min) == score(min)` for every recipe, so any `SkillMinimum` term
must appear identically on both sides, and property 2 then makes it cancel
everywhere. An earlier draft of this spec wrote the anchor
`(CraftBaseDifficulty + SkillMinimum * CraftSkillMinWeight)` into both sides;
it cancelled exactly, which made `CraftSkillMinWeight` a knob that could never
change an outcome. Removing it is the correction.

This is the intended curve, and a blind review that flagged it as a defect was
wrong: **50/50 at the recipe's minimum and near-certain by your fifteenth
masterwork helm** is realistic, and it holding for every recipe means
`SkillMinimum` reads consistently to a player across the whole crafting tree.

`SkillMinimum` keeps three real jobs: the hard gate on **discovery**, the hard
gate on **attempting**, and the progression reward via
`CraftDifficultyProgressionScale`. Difficulty is carried by the materials.

`CraftBaseDifficulty` also cancels. It stays only to keep both numbers in
stat-scale range so `dice.StdDevFor` does not hit its `mean < 1.0` floor, and
its doc comment must say it deliberately does not affect odds, so the pending
`config.yaml` audit does not flag it as orphaned.

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

**Salvage** mirrors the craft shape per ingredient unit, with difficulty driven
by the tier of the ingredient being reclaimed, so dear materials are harder to
recover than cheap ones.

### 5.1.2 Material tier is AUTHORED, not derived from gold value

Material tier is the **only** signal carrying craft difficulty (5.1.1.1), so it
has to be real.

**Gold value was tried and rejected** (owner, 2026-08-26). Measured across all
126 recipes, the 0.75 to 1.25 band appears inside almost every `skill_minimum`
bucket: it is noise, not signal. Named offenders:

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
grandfathered. Backfilling the grandfathered materials is a recorded follow-up,
not part of this slice.

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
  references and survive only as stub methods on fake actors in five test
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

### 5.5.2 One command, many rolls: collapse to ONE event

`search.go` fires **one** `OnSkillUse(Search)` per invocation today, gated on
`rolledAgainstSomething`, no matter how many of its six checks ran. Under a
naive reading of "one event per resolved roll" a room holding a hidden exit, a
hidden container, a stashed item, a hidden noun and a hidden mob would pay
**five** events where it pays one.

The rule is **one event per resolved COMMAND**, not per internal roll. Where a
command runs several rolls (search's six, track's two grades, salvage's
per-unit rolls), it awards once. A command that resolved at least one roll and
won none pays the fraction; a command that won any pays full weight.

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

**EVERY fitted multiplier is invalidated, not just skullduggery.** An earlier
draft re-solved only `SkillProgressionMultipliers[Skullduggery] = 0.83`, which
was fitted on measured play-time rates in U10b-0 Phase D
(`tools/balance/u10b_solve_v3.py`). The reasoning that justified re-solving it
applies identically to weapon-combat, dodge, parry, block, spellcasting and
rhetoric: awarding on a resolved loss raises attacker events per swing by
roughly **26%** and defender events by roughly **47%**, computed from the
clean-hit rate of 0.5752 in the analytics log. Re-solve them all, or record in
the commit why a given one is exempt.

Measure before and after against `_datafiles/logs/combat-analytics.jsonl`
(96,723 events) via `tools/balance/read_combat_analytics.py`. The buffer is
cumulative; never sum flush lines.

**This slice is not a no-op in four independent ways**, and the playtest must
report on each separately rather than as one impression:

1. **The convention move.** Does failing at something now visibly teach a
   little?
2. **The stealth change** (5.2). Is a skilled hider now meaningfully harder to
   find, judged from both sides?
3. **Crafting feel** (5.1.1). Does difficulty track the materials? Does mastery
   read the same on a novice recipe and an advanced one?
4. **Search, track and forage odds** (5.1). Weak searchers improve sharply and
   experts lose their near-certainty. Hidden exits and containers are permanent
   one-shot unlocks (`AddDiscovery`), so this is a pacing change rather than an
   ongoing faucet, but it should be checked against any quest that assumes
   searching is hard.

**Economy risks to watch** (5.1.1.2): the craft-then-salvage material sink, and
shop restock throughput, since mob crafters ship at skill 1 and dynamic pricing
keys on the stock-to-restock ratio.

## 7. Explicitly out of scope

- The three faucets: melee-vs-channel defence divergence, mob archer
  ranged-combat progression, and the tick-regen route. All U10b-2.
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
