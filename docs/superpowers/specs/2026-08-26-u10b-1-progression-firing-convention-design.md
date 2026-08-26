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
critical-failure as a separate bonus on top". Read literally that is not
monotonic: an ordinary failure pays nothing while a critical failure pays a
bonus, so failing badly teaches more than failing normally. Paying a fraction
on a resolved loss closes that hole and makes the payout ordering
crit > win > loss > fumble-with-its-bonus.

It also matters that both shapes are live today. Convention 1 is clean-hit-only
(melee, special moves); convention 3 is roll-happened-win-or-lose (sneak,
search, track, flee). Collapsing convention 3 into success-only would be a real
nerf to the stealth and exploration families.

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

### 5.1 Convert the 15 sites onto `contest.AgainstDifficulty`

Numbers preserved exactly, in the provable-no-op style of U1 through U5, with
the single exception in 5.2. This gives the static-difficulty channel its first
production callers and lets the rule in section 3 actually reach search, track,
forage, craft and salvage.

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

**Task 5.3 makes this slice rate-affecting.**
`SkillProgressionMultipliers[Skullduggery] = 0.83` was solved on measured
play-time rates in U10b-0 Phase D (`tools/balance/u10b_solve_v3.py`). Adding a
failure award to steal, plant and sneak changes the basis that number was
fitted against, so it must be re-solved rather than assumed stable.

Measure before and after against `_datafiles/logs/combat-analytics.jsonl`
(96,723 events) via `tools/balance/read_combat_analytics.py`. The buffer is
cumulative; never sum flush lines.

Combined with 5.2, this slice is **not** a no-op, and its playtest has to
distinguish three signals: the convention move, the stealth change, and the
skullduggery re-solve.

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
4. Skullduggery's multiplier is re-solved against measured rates, with the
   before and after recorded.
5. `contest.AgainstDifficulty` has production callers.
6. The guard's AST helper is proven to FAIL before it is made to pass, on a
   real `x.Character.OnStatUse(...)` selector-on-selector call.
7. No production site reaches progression without passing an entry point the
   guard can see, and every deliberate exception is named in an allow-list
   rather than merely escaping notice.
8. The adversarial playtest reports on the three signals in section 6
   separately.
