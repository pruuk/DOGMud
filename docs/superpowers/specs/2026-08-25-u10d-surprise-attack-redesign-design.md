# U10d — Surprise-Attack Redesign

**Date:** 2026-08-25
**Arc:** Unified Resolution (U0–U12). Split from U10 on 2026-08-21.
**Behaviour change:** Yes, deliberately and substantially.
**Depends on:** U1 (contest core), U6b (`ResolveChannelAttack` seam), U9 (progression layer).
**Roadmap row:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, plan table, **U10d**.

---

## 1. Why this slice exists

Surprise attack is the last attack in the game that never joined the unified
resolution seam. Every other channel — melee, ranged, spell, taunt, the sixteen
special moves, throw, steal, sneak detection, flee, the grapple family — resolves
through one contest against a real defender. Surprise attack resolves against
nothing at all.

The roadmap recorded this as one defect. Investigation on 2026-08-25 found
**three overlapping mechanics**, two live and one dead, plus a fourth latent
bug. All four are in scope.

### 1.1 What actually ships today

**Mechanic 1 — the pre-combat burst** (`internal/actions/surprise_attack.go`,
389 lines). Fires before combat is joined. Iterates every equipped weapon
(primary, offhand, up to four extra arms from the Extra Arms mutation, falling
back to fists) and swings each one.

The primary weapon is appended with `hitPenalty: 0.0`. The only hit check in the
function is `util.Rand(100) < int(hitPenalty*100)`, so for the primary that test
is `roll < 0`, which never fires.

**And it is worse than "the primary auto-hits": EVERY weapon auto-hits.** The
offhand and extra-arm penalties read from five config knobs that are absent from
`config.yaml`, and their validators are shaped
`if b.X < 0 || b.X > 1.0 { b.X = 0.10 }` (`config.balance.misc.go:259-273`). An
absent key unmarshals to **0**, which is neither negative nor above 1.0, so the
defaulting branch never runs and the knob stays **0.0** — not the 0.10 / 0.25 /
0.40 / 0.55 / 0.70 the comments advertise. With every `hitPenalty` at 0, the
self-penalty roll can never fire for any limb.

So today's burst is an unconditional multi-weapon auto-hit volley: main hand,
offhand, and up to four extra arms, every one of them guaranteed to land.

**There is no defender *contest* anywhere in the function** — no roll, no
defence, no skill, no stat of the target's. A surprise attack against a novice
and against the Elemental Queen resolve on identical rolls.

The target's **equipment does** enter, once: `surprise_attack.go:86` reads
`targetChar.GetPhysicalMitigation()` and `:279-284` applies half of it. So an
armoured target already takes materially less. An earlier draft of this spec
claimed equipment "never enters"; that was wrong, and it matters, because it
means deleting the half-mitigation bypass (2.3) is a real change rather than the
removal of a no-op.

The burst is also incoherent about which stat drives it. It computes raw damage
from **Strength**, passes **skullduggery** in as the skill-multiplier rank, and
then scales the result by a **Dexterity**-derived `surpriseMult` of
`max(1.0, (dex + skullduggeryRank)/100)`. Three different stats, no contest.

It applies half of the target's physical mitigation as a "crit-like bypass".

**Mechanic 2 — the round-1 forced crit** (`internal/combat/combat.go:403`).
Independently of the burst, an aggro type of `characters.SurpriseAttack` sets
`backstabCrit = true` in `calculateCombat`, which forces a crit on the first
swing of the first ordinary combat round. `calcHitDamage` returns
`false // consume backstab` after that one swing, so **only the first swing
crits, not the round.**

So a stealth opener currently double-dips: a free uncontested volley, and then a
forced crit in the round that follows.

**Mechanic 3 — the surprise-round boundary. Dead at BOTH ends.**

An earlier draft of this spec said `SurpriseLeft` "is set correctly" and merely
lacked a consumer. **That was wrong**, and it was the single most dangerous
error in the document: the blind review of 2026-08-25 found the flag is never
true in production at all.

*The producer is broken.* `advanceToEngaged` decides the flag with
`SurpriseLeft: prevEngaging.Reason.Trigger == TriggerSurpriseAttack`
(`combatphase.go:299`) — it reads `EngagingData.Reason`. But
`TransitionToEngaging(d EngagingData, r state.TransitionReason)` stores the
caller's struct verbatim, `m.engaging = &d` (`combatphase.go:234`), and **never
copies `r` into `d.Reason`**. The only production caller,
`internal/characters/combat_state_compat.go:135-149`, builds
`EngagingData{Target, RoundsUntil}` and passes the trigger *only* in the second
argument. So `d.Reason` is always the zero value and that comparison is always
false.

*The consumer is missing.* `(*Machine).OnCombatRoundEnd()` — the only function
that fires the `OnEndOfRoundIfSurprise` callback and clears the flag — has
exactly one caller in the repository: `combatphase_test.go:181`. The block it
lives in is still labelled `=== STUBS — Implementations land in Tasks 6-8. ===`.

*Why the tests pass anyway.* `combatphase_test.go:152/163/173` set
`EngagingData.Reason` by hand, which production never does. The unit tests
exercise a path the game cannot reach.

**The important subtlety: two different readers, only one broken.**
`Awareness_Cascades.go` reads the **transition reason `r`**, which *is* passed
correctly, so `Hidden` genuinely is preserved through a surprise engagement.
`advanceToEngaged` reads `d.Reason`, which is not. That split is exactly why the
bug is invisible: the visible half (staying hidden) works, and the half nothing
consumed (the flag) does not.

**U10d DELETES this machinery rather than repairing it** (section 3). The 2.1
scope decision means nothing needs a round-scoped flag, so fixing a producer
whose only consumer is being removed would add a live code path nothing uses.

Recorded because it nearly went the other way: an earlier draft of the plan
opened with a task to fix `TransitionToEngaging`. Had the every-swing design
survived, that fix would have been mandatory — and without it every mechanism
built on the snapshot would have compiled, passed its unit tests against
hand-built machines, and done nothing whatsoever in the live game.

**Latent bug 4 — the ambusher never stops being hidden by design.** Because
mechanic 3 is dead, nothing consumes `SurpriseLeft`. `Awareness_Cascades`
deliberately *preserves* `Hidden` on a surprise trigger and then waits for a
callback that never arrives. The ambusher's `Hidden` breaks only incidentally,
via `handleCombatRound`'s `ForceVisible`, which acts on the **defender** — that
is, only once the ambusher is themselves attacked.

### 1.2 What the redesign is for

Owner intent, stated 2026-08-25:

> "I'm ok with the autocrit for all hits for that round and maybe even a
> multiplier based upon skullduggery on top since players are forgoing having
> companions to use surprise strike. However, I think we should still have the
> player roll for the hit and I'd also like to unify the mechanism in with the
> rest of the stuff we built as much as possible."

and:

> "I want to make the sneak and surprise strike playstyle a viable option when
> weighed against having powerful companions."

So: **keep the payoff large, but make it earned against the defender, and run it
on the machinery the arc already built.**

**A note on the premise.** A search on 2026-08-25 found no mechanical exclusion
between companions and stealth: nothing breaks `Hidden` for having a companion
out, and no command gates on it. "Forgoing companions" is a real *opportunity*
cost (conviction reservation, skill investment, build focus) but not a rule the
code enforces. The design does not depend on it being one.

---

## 2. The design

### 2.1 Shape: ONE opening strike, and stealth breaks immediately

`actions.SurpriseAttack` is **deleted**. There is no pre-combat burst.

**Exactly one attack is special: the opening strike.** It contests normally, and
if it wins the contest it crits, carrying the stacked skullduggery multiplier
(2.4). Every other swing of that round is an **ordinary attack** — ordinary hit
resolution, ordinary crit chance, ordinary damage. **Stealth breaks
immediately.**

> **Scope decision, owner, 2026-08-25.** An earlier draft of this spec had *every
> landed swing of round 1* crit. That was retired for two reasons. First, the
> blind review showed `calcSwingCount` issues up to four swings **per weapon**,
> so "the whole round crits" meant far more crits than the design had modelled
> and the damage table was wrong in the direction that mattered. Second, and
> decisively, the owner preferred the simpler fiction: *"you lose stealth
> immediately, the rest of the hits roll like normal attacks."*
>
> The simplification is not just tuning. It **deletes** the entire round-boundary
> problem described in 1.1 mechanic 3 — which was broken at both ends — rather
> than requiring it to be fixed. See section 3.

**The slice has two halves, and they are now symmetric.** Section 2.2 to 2.7
describe the **melee** opening strike; section 2.8 the **ranged** one. Both are a
single contested attack that crits on a win and carries the same stacked
multiplier and the same knob. Neither confers anything on the rest of the round.

### 2.2 The contest is ordinary melee

**Note on where this plugs in.** Melee does *not* resolve through
`combat.ResolveChannelAttack`. U6b deliberately left melee on its own scoring
loop while making it consume the same defence-set and name builders as every
other channel ("melee keeps its scoring loop but consumes the same name
builder"). So the melee half of U10d plugs into the **melee path**.

**Ranged is the opposite**, and this is why the slice covers both (section 2.8).
`ExecuteFire` already goes through `ExecuteSkillMove` → `ResolveChannelAttack`
with a real `AttackSide` carrying `ForceCrit`. So "crit if the contest is won"
has to exist in **two** places, because the arc has two attack paths:

| Path | Used by | Where `critOnWin` lives |
|---|---|---|
| melee scoring loop | melee auto-attack | a parameter on `resolveDefenseOutcomeCore`, beside `forceCrit` |
| channel seam | ranged (and every special move) | a field on `combat.AttackSide`, beside `ForceCrit` |

The two must stay semantically identical. Pin that with a shared test rather
than trusting the names to drift in step.

The attack side is therefore exactly what round 1 would have rolled anyway —
`combat.calcAttackScore`, unchanged:

```
attackScore = GetEffectiveDexterity()
            + GetCombatSkillLevel() * SkillWeight
            - penalty, then the usual stamina / prone / grapple / darkness terms
```

`GetCombatSkillLevel()` is **weapon-appropriate** — weapon-combat for an armed
attacker, unarmed-combat for fists — so an unarmed ambusher is scored on the
skill they actually used. Do not hardcode `skills.WeaponCombat`.

The defence is the equipment-gated best-of-all via `DefenceEntriesFor`. For
`ChannelMelee` that set is **dodge, parry and block** — three, not five.
`DefenceSetFor` (`internal/combat/defence_sets.go:43-45`) gives quell only to
`ChannelSpellMental` and defy only to `ChannelSocial`, and `DefenceEntriesFor`
intersects the channel table with the equipment gate, so it can never *add* a
defence the channel omits.

An earlier draft of this spec said "five-defence best-of-all" in three places.
That was wrong. It matters twice over: the melee-versus-ranged trade in 2.8.2
rests on the size of that gap, and any narration or test written against quell
or defy answering a melee swing would be an unreachable branch.

Whichever of the three wins is charged normally and progresses normally.

**Skullduggery does not feed the attack roll.** It amplifies the payoff only
(2.3). Two reasons: skullduggery already gated entry — the attacker had to win
the sneak contest to be `Hidden` at all — and keeping the roll ordinary is what
makes the seam genuinely shared rather than a special case wearing the seam's
clothes.

**On a lost contest the opening strike is not upgraded**, and is narrated by the
winning defence's ordinary vocabulary. Losing the contest is the target having
sensed the ambush.

> **"Simply misses" is wrong, and an earlier draft of this spec said it.** Since
> U6 Task 10 a defensive win is **not** a clean miss: the swing still lands for
> partial damage, with `res.hit == true`, `res.defended == true` and
> `damageMult` between 0.0 and 0.5. So a defended opening strike deals ordinary
> deflected damage.
>
> **The implementation hazard this creates.** `calcHitDamage` is called on every
> `res.hit`, deflections included. If the opening-strike flag alone selects the
> crit branch, a *defended* opening strike would roll the full stacked mean and
> consume the flag, then have `damageMult` applied — still delivering roughly
> half of a maximum ambush on a swing the defender won. The crit branch must be
> selected by the crit verdict, and the stack must apply only when the contest
> was actually won.
>
> Spec test 2 must therefore assert "not upgraded", **not** "zero damage".

### 2.3 `critOnWin`: crit if it lands

`resolveDefenseOutcomeCore` already carries a `forceCrit bool` parameter — that
is the sleeping-victim case, and it forces the **win**. U10d adds a second,
distinct parameter beside it:

```go
// critOnWin upgrades a WON contest to a crit. It does NOT decide the contest.
//
// Deliberately NOT forceCrit. forceCrit forces the WIN outright; a defender
// never gets to answer it. critOnWin respects the contest in full: the
// defender rolls and may win, and on a defender win nothing is upgraded
// because there is no hit to upgrade.
//
// The two are independent and may both be true (a sleeping ambush target),
// in which case forceCrit decides the outcome and critOnWin is redundant.
critOnWin bool
```

**Set for the opening strike ONLY**, never for the rest of the round.

Its value comes from the same signal that drives the opening strike itself:
`sourceChar.Aggro.Type == characters.SurpriseAttack`, consumed once. That signal
demonstrably works today — it is what fires `backstabCrit` — which is why this
design needs no snapshot and no round-scoped flag (section 3).

**Compared with today,** the change is not that more swings crit — the count is
the same, one — but that the crit is now *earned*. Today `backstabCrit` forces
the first swing to crit unconditionally; under U10d the defender can deny it.
The compensation for that new risk is the skullduggery stack (2.4).

Crit damage already rolls off `sdp.rawDmgForCrit`, the **unmitigated** mean, times
`critDmgMult`. Therefore the burst's old half-mitigation bypass is **strictly
weaker than what a crit already grants** and is deleted rather than migrated.
Nothing is lost.

### 2.4 The opening strike and the skullduggery stack

One swing per surprise round — the **primary weapon**, or fists when unarmed — is
the **opening strike**. It multiplies its crit worth by the skullduggery crit
term on top of the ordinary combat-skill term.

This lands in `combat.buildDamageParams`, which is already where `critDmgMult`
is computed as `CritDamageMultiplier(combatSkillLevel)`. For the opening strike
of a surprise round it becomes:

```
openingStrikeCritMult =
      CritDamageMultiplier(GetCombatSkillLevel())
    * CritDamageMultiplier(skullduggeryRank)
    * SurpriseOpeningStrikeMultiplier
```

where `CritDamageMultiplier(rank) = CritDamageBase + CritDamagePerSkill × rank`
(shipped: `2.0 + 0.05 × rank`).

Offhand and extra-arm swings that round keep the ordinary
`CritDamageMultiplier(GetCombatSkillLevel())`, unchanged.

**Why only the opening strike.** Stacking on every swing multiplies across up to
six weapons. Modelled against the owner's own character
(`_datafiles/world/dogmud/users/3.yaml`: Str base 115, weapon-combat 69,
skullduggery 50, extra-arms 1, health 545), with a 1.5x weapon,
`MeleeDamageScale` 0.52 and `GlobalDamageMultiplier` 0.5:

**The stat is `Strength.ValueAdj`, not `Base`.** `StatInfo.Recalculate` sets
`Value = Racial(Base) + Training + Mods` and `ValueAdj = Value`
(`internal/stats/stats.go:85-89`). The save carries
`strength: {base: 115, training: 21}`, so the live figure is **at least 136**,
before any equipment `Mods`. An earlier draft of this spec used an unexplained
120 and every figure below was consequently low by more than a tenth.

```
SkillMultiplier caps at SkillSoftCap 50 -> 3.0
raw = 136 * 3.0 * 1.5 * 0.52 * 0.5              = 159
CritDamageMultiplier(69) = 2.0 + 0.05*69        = 5.45
CritDamageMultiplier(50) = 2.0 + 0.05*50        = 4.50
```

| Variant | Damage | vs a 545 HP veteran |
|---|---|---|
| Today's forced first-swing crit | 159 x 5.45 = **867** | 1.6x |
| **U10d opening strike** | 159 x 5.45 x 4.50 = **3,902** | **7.2x** |

These are floors, not estimates: `Mods` from equipment pushes Strength higher
still.

**This is the whole bonus.** The remaining swings of the round are ordinary
attacks rolling around the mitigated mean, exactly as they would in any other
round. An earlier draft applied the crit to every swing, which — once
`calcSwingCount`'s four-swings-per-weapon cap was accounted for — reached roughly
11,800 at these ranks and about 21,000 with the full Extra Arms mutation. Those
figures are recorded here only so nobody reintroduces the every-swing reading
believing it was ever modelled and accepted.

Even a novice stealth build (both skills 5) reaches `2.25 x 2.25 = 5.06x` and
one-shots newbie-tier mobs. Bounding the stack to the opening strike preserves
the assassination fantasy and the "gigantic hit" the owner asked for while
removing the unbounded multiplication across weapons.

Expected round-1 total at the owner's ranks, all swings landing:
`3,902 + 867 x 2 ≈ 5,600` into one target — against a defender who genuinely got
to defend, and zero if the contest is lost.

**What it is replacing is not small either.** Because all five penalty knobs sit
at 0.0 (1.1), today's burst already lands *every* weapon unconditionally, and
then round 1 adds a forced crit on top. The honest comparison is not "one
guaranteed hit becomes a contest" but "a guaranteed multi-weapon volley plus a
free crit becomes a contested round". U10d is less of a straight buff than the
table alone suggests.

### 2.5 Cost

**Unchanged: the shared `special-move` cooldown is still consumed.** Opening
from stealth costs the attacker their special move for `SpecialMoveCooldown`
(shipped: 4) rounds. Note this is one shared timer across eighteen verbs, so an
ambush denies bash, trip, kick and the rest for those rounds. That is the
intended tradeoff.

The cooldown try moves from the deleted `SurpriseAttack` into
`actions.EngageAggroType`, which keeps the existing contract: a hidden attacker
whose cooldown is unavailable opens as an ordinary attack, not a surprise round.

### 2.6 Progression

**Today's burst awards no combat progression at all.** It resolves entirely
outside `calculateCombat`, so `applyCombatProgression` never sees it. Its only
award is one bare `actor.OnSkillUse(string(skills.Skullduggery))` at
`surprise_attack.go:360`, off the U9 seam.

#### 2.6.1 What the surprise round inherits for free

Because the surprise round *is* a melee round, it picks up melee's progression
wholesale via `applyCombatProgression`. Nothing needs building for any of this.

**Attacker:**

| Award | When | Path |
|---|---|---|
| `strength` stat use | unconditionally, once per round | `emitAttackerStatGain` |
| `dexterity` stat use | unconditionally, once per round | `emitAttackerStatGain` |
| weapon-combat *or* unarmed-combat | per clean weapon hit | `OrdinaryEvents`, keyed on `WeaponHitInfo.SkillTag` |
| that skill's primary stat (dexterity) | same event | `OnSkillUseScaled` rolls the primary itself |
| crit bonus tier | once per round | `BonusEvents`, `progression.Classify` |

**A correction about the crit tier, and why the opening strike usually will not
pay it.** An earlier draft said a successful ambush *always* pays the attacker's
once-per-round crit bonus. It does not, and under the 2.1 scope it will usually
**not**.

`AttackResult.Crit` is a **per-swing** flag, reset at the top of every swing
(`combat.go:432-434`), so by the time the bonus tier reads it outside the weapon
loop it reflects **only the last swing** — a quirk `applyCombatProgression`
documents in its own comment (`NewRound_DoCombat_unified.go:697-700`) and
explicitly hands to U10b.

The opening strike is the **first** swing. So in any multi-swing round its crit
has been overwritten by the time the bonus tier reads the flag, and the ambush
pays no crit bonus unless the last swing happens to crit on its own. A
single-swing round pays it.

U10d does **not** fix this. It is the same last-swing semantics every melee round
already has; changing it is a firing-condition change and therefore U10b's. It is
recorded because it is counter-intuitive: the most decisive blow in the game
routinely earns no crit progression bonus.

**Defender — all of it new, because there was no contest before:**

- defence skill and defence stat, once per defence type used
  (`processDefenderProgression`)
- the crit-received **toughening** stat — vitality for the physical channel —
  via `BonusEvents` `ClassObserved`

So being ambushed now teaches you to take a hit. Under the current auto-hit
burst it teaches the victim nothing whatsoever.

#### 2.6.2 Skullduggery: replaced on the seam, not dropped

The bare `OnSkillUse` call dies with the file. It is **replaced**, not removed:

- A **second attacker `progression.Outcome`** carrying
  `AttackerSkill: "skullduggery"`, applied with
  `ApplyProgression(..., progression.SideAttacker, ...)`. A second `Outcome` is
  structurally required: `Outcome` holds exactly one `AttackerSkill`, and the
  first already carries the weapon or unarmed combat skill.
- **Fires once per surprise round**, on the same condition the combat skill
  uses — at least one clean hit. Success-only, matching the convention U10's
  new sites already adopted.
- **`AttackerStat` left empty.** `ApplyProgression` calls `OnSkillUseScaled`,
  which already rolls the skill's primary stat, and only rolls `ev.Stat`
  separately when it names a *different* stat. Setting it would be a no-op at
  best and a duplicate at worst.

**Stated consequence, so it is not discovered as a surprise later:** dexterity
is rolled **three times** in a surprise round — once unconditionally from
`emitAttackerStatGain`, once as the combat skill's primary, once as
skullduggery's primary (`SkillPrimaryStats["skullduggery"] == "dexterity"`,
same as weapon-combat). This is accepted. A surprise round happens once per
engagement behind a shared 4-round cooldown, and an ambush is genuinely a
dexterity act.

**Why the award is not simply dropped.**
`SkillProgressionMultipliers[Skullduggery] = 0.83` was solved on measured
play-time rates in U10b-0 Phase D (`tools/balance/u10b_solve_v3.py`). Removing a
firing site changes the basis that figure was fitted against, so dropping the
award would quietly make skullduggery progress slower than the solve intended.
That is a retune, and it should not happen as an unexamined side effect of a
combat redesign.

#### 2.6.3 Out of scope, explicitly handed to U10b

Skullduggery has **18** progression sites and **none** of them is on the U9
seam. U9 routed melee, channel defences, spells and taunt; U10b's Category C is
crafting, salvage and forage. The stealth family was claimed by neither.

U10d converts exactly one — its own. The remaining **17** stay bare
`OnSkillUse` / `CheckSkillProgression` calls and belong to **U10b**
("progression firing consistency"), which is still open and whose 135-site
firing audit already enumerates them:

| File | Sites |
|---|---|
| `internal/actions/steal.go` | 3 |
| `internal/actions/plant.go` | 3 |
| `internal/actions/shadow.go` | 2 |
| `internal/usercommands/skill.skullduggery.sneak.go` | 2 |
| `internal/usercommands/picklock.go` | 2 |
| `internal/actions/defuse.go` | 1 |
| `internal/usercommands/throw.go` | 1 |
| `internal/mobcommands/flee.go` | 1 |
| `internal/mobcommands/sneak.go` | 1 |
| `internal/hooks/NewRound_DoCombat_helpers.go` | 1 |

> **`internal/mobcommands/sneak.go:19` is easy to miss** and an earlier draft of
> this spec did miss it, undercounting by one. It calls
> `OnSkillUse("skullduggery", 0)` with a **string literal** rather than
> `skills.Skullduggery`, so it does not appear in the obvious symbol search.
> Grep the literal as well as the constant. It is also the mob path — the same
> player/mob asymmetry this spec is otherwise careful about.

Splitting this way keeps U10d's playtest attributable to the combat redesign
rather than to a 16-site progression sweep landing in the same change.

> **Naming hazard.** **U10b-0** ("progression rank from training", phases A–F)
> shipped 2026-08-24 as `d29996d4d` / PR #60. **U10b** ("progression firing
> consistency") is a *different* slice and has never been started. Its roadmap
> row carries no shipped marker. Do not read one as the other.

#### 2.6.4 The ranged surprise strike

A surprise shot awards:

| Award | Path | Note |
|---|---|---|
| `ranged-combat` | `usercommands/shoot.go:199`, a bare `OnSkillUse` | **off the U9 seam.** Pre-existing; see below. |
| `perception` | rolled as ranged-combat's primary stat | `SkillPrimaryStats["ranged-combat"] == "perception"` |
| crit bonus tier | the channel seam, `defence_multiplier.go:548-578` | always pays, since a landed surprise shot always crits |
| `skullduggery` | **added by U10d**, seam-routed, same shape as 2.6.2 | once per surprise shot, on a hit |

No triple-roll problem here, unlike melee (2.6.2): ranged-combat's primary is
perception and skullduggery's is dexterity, so the two events roll different
stats.

**An off-seam finding, recorded not fixed, and narrower than an earlier draft
claimed.** The only award for the deliberate `shoot` action is a bare
`OnSkillUse` in the **player** wrapper (`usercommands/shoot.go:199`), with no mob
equivalent — so **no mob earns ranged-combat progression from firing**.

It does *not* follow that mob archers earn none at all. A Shooting-subtype weapon
maps to `skills.RangedCombat` in `characters.CombatSkillTagForItem`
(`internal/characters/skills.go:327-329`), and `applyCombatProgression`'s
per-weapon loop keys on `WeaponHitInfo.SkillTag` built from exactly that
function. So any character, mob included, that **melee** auto-attacks with a bow
equipped does earn seam-routed ranged-combat progression. (`unloadedMeleeDamageCap`
exists because that case is real.) The gap is specific: firing, not archery.

Note also for 2.2: `GetCombatSkillLevel()` has **three** branches, not two —
weapon-combat, unarmed-combat, and ranged-combat for a Shooting-subtype weapon.

Both belong to U10b alongside the skullduggery family. U10d does not touch them,
because changing what an ordinary shot awards is a change to every shot in the
game and would contaminate this slice's playtest with archer-mob scaling.

### 2.7 Edge cases: deliberately not special-cased

Decided by the owner on 2026-08-25: **ship as-is and let playtest speak.**

- **A sleeping target** already carries `ForceCrit` from
  `snapshotSleepingVictims`, so an opening strike against a sleeper is an
  uncontested stacked crit — no defence roll at all. Defensible as an
  assassination. Note there is no sleep-inducing spell in the data; the reachable
  vectors are scheduled NPCs during `activity: sleeping` segments and players who
  typed `sleep` for the regen.
- **An already-engaged target** is not excluded, so re-hiding mid-fight could
  produce a second opening strike. The brake is the shared 4-round `special-move`
  cooldown. Note the *other* brake an earlier draft cited — "the difficulty of
  re-hiding in combat" — is weaker than assumed: `Sneak`'s only combat gate is
  `char.Aggro != nil` (`actions/sneak.go:64-66`), and a successful sneak sets no
  cooldown, so after a target dies and `RetargetOrEnd` clears aggro, re-hiding is
  cheap.
- **Mobs get the full mechanic, including the stacked opening strike.** Owner
  decision, 2026-08-25, asked and answered explicitly. Consequences, stated
  plainly rather than buried: hidden hostile mobs (`buffids: [9]`) exist in
  several zones including early ones, **crits bypass mitigation entirely so armour
  is not a counter**, and a first-hub ambusher's opening round rises from roughly
  half a new player's health to nearly all of it. Mobs also train, capped by
  `MobSkillTrainingCap` (25), and mob instance state persists in
  `mobs.instances/`, so a long-lived ambusher keeps its ranks.

All three are recorded so the playtest probes them deliberately. The mob case in
particular should be walked by a **fresh character**, not an established one.

### 2.8 The ranged surprise strike

Added to scope on 2026-08-25 at the owner's request: a stealth build should be
able to open with a bow, not only a blade.

**One shot, not a round.** `shoot` already fires exactly one shot, so the ranged
opener is naturally the single "opening strike" and needs no per-weapon logic.
It gets the same stacked crit as the melee opening strike:

```
CritDamageMultiplier(rangedCombatRank)
  * CritDamageMultiplier(skullduggeryRank)
  * SurpriseOpeningStrikeMultiplier
```

**It plugs in through `AttackSide.CritOnWin`**, since `ExecuteFire` already
builds an `AttackSide` and routes through `ExecuteSkillMove` →
`ResolveChannelAttack`. The attack side is unchanged: Perception +
ranged-combat, per U6b Assumption 2 (an aimed shot is a deliberate-move action,
not an auto-attack swing). As with melee, **skullduggery amplifies but does not
aim.**

#### 2.8.1 Three brakes that melee gets free and ranged does not

These are the reason this is a design addition rather than wiring. Each was
verified against the code on 2026-08-25.

**1. Firing reveals the shooter in ONE case, and the spec previously got this
backwards.** An earlier draft asserted that firing never breaks stealth,
reasoning from a grep: there is indeed no `TransitionToRevealing`, no
`CancelBuffsWithFlag(buffs.Hidden)` and no `ForceVisible` anywhere in
`combat_fire.go`, `usercommands/shoot.go` or `mobcommands/shoot.go`. The grep is
right and the conclusion was wrong — the reveal happens **indirectly**:

```
ExecuteFire  (!crossRoom && char.Aggro == nil)
  -> char.SetAggro(..., characters.DefaultAttack)          combat_fire.go:243
  -> CombatPhase.TransitionToEngaging(Trigger: TriggerAttackCommand)
                                              combat_state_compat.go:135-149
  -> Awareness_Cascades Idle->Engaging, trigger != TriggerSurpriseAttack
  -> Hidden -> Revealing                      Awareness_Cascades.go:30-42
```

So a hidden, not-yet-engaged shooter taking a **same-room opening shot is
already revealed today**.

That leaves the real gaps narrower and different from what the earlier draft
described:

- **cross-room shots**, which skip `SetAggro` entirely and so never cascade;
- **shots taken while already engaged** (`char.Aggro != nil`), which also skip it.

Cross-room is excluded from the bonus anyway (brake 3). The already-engaged case
is the one that genuinely needed closing: without a reveal, a player who re-hides
mid-fight could fire repeated maximum-bonus shots.

> **U10d makes a surprise shot reveal the shooter explicitly**, rather than
> relying on a side effect of `SetAggro` that does not fire on two of the three
> paths. Belt and braces: the explicit call is a no-op when the cascade already
> revealed them, and load-bearing when it did not.

**2. Fire deliberately never burns the special-move cooldown.** The comment at
the `RecordAndWait` call says so explicitly: *"Fire never burns the special-move
cooldown — only the combat round."* So the brake chosen for the melee ambush
(2.5) does not exist here by default.

> **A surprise shot burns the shared `special-move` cooldown**, matching melee.
> An ordinary shot continues not to.

**What that actually costs the archer: exactly one shot.** Two earlier drafts of
this spec both got this wrong, in opposite directions — first "the existing ranged
rotation is untouched", then "roughly 8 rounds of not shooting". Neither is right.

`reload` reads and writes the same `special-move` timer
(`combat_reload.go:86,133`) and costs no combat round of its own, and firing
always unloads (`combat_fire.go:249`). So the rotation is already gated:

```
ORDINARY:  R1 shoot, reload (cd->R5) | R2 shoot | R3-4 dry | R5 reload+shoot | R6 shoot
SURPRISE:  R1 shoot (cd->R5)         |          | R2-4 dry | R5 reload+shoot | R6 shoot
```

The surprise opener consumes the cooldown that reload would have used, so the
archer loses the **R2 shot** and rejoins the ordinary cadence at R5. One shot, not
a dead window. That is a fair price for the opener and needs no separate cooldown
key.

**3. Cross-room shots are aggro-free AND uncounterable by design.** A cross-room
shot never calls `SetAggro`, and `counterSkillMoveExit` is reach-gated so the
cross-room shot is "the ONE uncounterable attack" (owner decision, U6b Task 10).
Layered onto anonymous narration, a cross-room stacked crit would let a player
kill a boss from the next room with no retaliation, no counter, and no way for
the target to learn who did it.

> **The stacked bonus is SAME-ROOM ONLY.** A cross-room shot from stealth
> remains an ordinary shot: no `CritOnWin`, no skullduggery term, no cooldown
> charge, no reveal. `ExecuteFire` already computes `crossRoom`, so this is a
> gate on an existing local, not new plumbing.

#### 2.8.2 The ranged opener is easier to land. That is intended.

`ChannelRanged`'s defence set is **narrower than melee's, by exactly one
defence**: melee answers with dodge, parry and block; ranged answers with dodge
and block (`DefenceSetFor`, `defence_sets.go:43-46`). **Parry is the whole
difference.** An earlier draft claimed the gap was three defences, because it
wrongly credited melee with quell and defy as well; the corrected gap is
narrower but still real, and still favours the archer.

So a ranged surprise strike is **easier to land** than a melee one while hitting
at least as hard — and in practice considerably harder.

**The number, which an earlier draft never computed.** `ExecuteFire` uses
`shotMult := weapon.DamageMultiplier * RangedShotScale` (`combat_fire.go:253`),
and ranged `damage_multiplier` values run far above melee's: the Ironhorn Warbow
(`items/weapons-10000/10046`) is **7.50**, the arbalest 7.00, against roughly 1.5
for a good one-handed melee weapon. At the owner's real **Perception 152**
(`users/3.yaml:43-45`), with ranged-combat 50 and skullduggery 50, on the
**undetuned** 7.50 bow:

```
raw   = 152 * 3.0 * 7.50 * 0.52 * 0.5   =    889
crit  * CritDamageMultiplier(50) = 4.50 =  4,001
stack * CritDamageMultiplier(50) = 4.50 = 18,005
```

**About 18,000 from one roll**, versus the melee opening strike's ~3,900 with a
mid-tier sword or ~9,760 with Blackrazor — and with one fewer defence answering
it. That is what 2.8.3's detune exists to bring down.

**Owner decision, 2026-08-25: ship it and let the playtest speak.** No ranged
normalisation, no separate knob. The number is written down here so the playtest
knows what it is looking at and so a later retune is a decision rather than a
discovery.

**The easier hit rate is a design decision, not a tolerated imbalance** (owner,
2026-08-25). The fiction carries it: shooting someone from cover is genuinely
easier than crossing open ground to put a knife in them.

> **The compensating half of that trade is GONE, and this must be read with open
> eyes.** When the owner endorsed this asymmetry, the melee ambusher was paid in
> **volume** — every swing of the round crit, so a dual-wielder or Extra Arms
> build converted a harder approach into several critting hits while the archer
> got one shot. The 2.1 scope decision removed that: **both** openers are now
> exactly one upgraded strike.
>
> So the openers no longer trade hit-rate against volume. They are the same
> shape, and the ranged one is **both easier to land and roughly 3.3x larger**
> (~13,000 against ~3,900), because ranged weapon multipliers reach 7.5 where
> melee's reach about 1.5.

Comparing like with like — best weapon against best weapon, which an earlier
draft failed to do by pitting the top bow against a mid-tier sword:

| | Ease of landing | Opening strike |
|---|---|---|
| Melee, Blackrazor 3.75 | harder — answers dodge, parry and block | **~9,760** |
| Ranged, Ironhorn 2.75 + unengaged 2.75x + ranged ambush 0.5x | easier — answers dodge and block; **no parry** | **~9,080** |
| Melee, typical sword 1.30 | harder | ~3,380 |

So the two top-end openers land at **near parity**, with melee marginally ahead —
not the 3.3x gap an earlier draft claimed, which came from comparing the best bow
against a mid-tier sword. The compensation is no longer "melee gets volume" (2.1
removed that) but **the archer is half-strength whenever anything is attacking
them** (2.8.3).

The playtest should still treat "is the melee opener worth taking, versus just
using a bow?" as a primary question. If the answer is no, the lever is
`RangedUnengagedDamageMultiplier` or the bow table — both now exist as dials.

Do not "fix" the narrow ranged defence set as part of a future surprise-attack
change. If it is ever revisited, it must be revisited as a property of **every**
shot in the game.

#### 2.8.3 The ranged economy: a detuned bow line and an unengaged bonus

Added to scope 2026-08-25 after the opener numbers surfaced the real problem.
**The bow line is not slightly high, it is a different line entirely:**

| | Range | Top |
|---|---|---|
| Melee `damage_multiplier` | 0.85 – 1.50 | Blackrazor **3.75** (a deliberate legendary outlier, 2.5x the next weapon) |
| Ranged `damage_multiplier` | 2.00 – 7.50 | Ironhorn Warbow **7.50** |

A *Training Bow* worth 5 gold is 4.00 — higher than every melee weapon in the
game except Blackrazor.

**That inflation is not gratuitous, which is why it cannot simply be removed.**
It compensates for two things melee does not pay: a shot is **one** attack where
melee gets up to four swings per weapon (`calcSwingCount`), and **reload burns
the shared 4-round `special-move` cooldown** (`combat_reload.go:86,133`) while
costing no combat round of its own. Measured at veteran ranks, sustained ranged
already sits at roughly **half** of melee. Detuning the multipliers alone would
take it to about a fifth and end archery as a build.

**So the compensation moves from a flat multiplier to a situational one.**

##### The rule

A ranged attack is multiplied by `RangedUnengagedDamageMultiplier` when **nothing
in the room is currently targeting the shooter**.

> **Do NOT use `Character.Attackers()`.** An earlier draft of this spec did,
> reasoning from its docstring — *"Replaces room-scan loops for 'who's attacking
> me?' The list is updated atomically by the Combat Phase framework on every
> transition."* **That list is never populated in production.**
>
> Its only writer is `RecordInboundAttacker`, called from exactly one place
> (`combatphase.go:236`) behind `if target := lookupMachine(d.Target); target !=
> nil`. `lookupMachine` reads `machineRegistry`, written only by
> `combatphase.RegisterMachine` — which has **zero production callers**; every
> call is in `combatphase_test.go`. The registry is empty at runtime, so
> `Attackers()` always returns an empty slice.
>
> A second, independent break sits behind it: `SetAggro` passes
> `Actor: state.ActorRef{UserId: c.userId}` (`combat_state_compat.go:143`), and a
> mob's `c.userId` is 0, so the ref is zero and `RecordInboundAttacker`
> early-returns on `a.IsZero()`. Even with a working registry, **no mob attacking
> a player would ever register.**
>
> Had this shipped, the bonus would have applied **unconditionally** — a flat
> ranged buff on top of a flat bow nerf, with the entire situational design
> inert. This is the *same failure mode* this spec spends section 1.1 diagnosing
> for `SurpriseLeft`, arrived at the same way: by trusting a doc comment instead
> of reading the call sites. `recoveryContest`
> (`internal/hooks/recovery_contest.go:31`) is already silently inert for exactly
> this reason.

Use a room scan instead, which is what the rest of the combat code already does
(`internal/hooks/combat_retarget.go:80-122` scans `room.GetMobs(rooms.FindFighting)`
and compares `Aggro.UserId`). The condition is: **no actor in the shooter's room
has the shooter as its current aggro target.**

**Recorded for U11, not fixed here:** `combatphase.RegisterMachine` /
`machineRegistry` / `RecordInboundAttacker` / `Character.Attackers()` are dead
infrastructure with at least one existing inert consumer. Either wire them up or
delete them, but they should not sit in the tree looking usable.

##### Why this shape

It makes the archer's weakness **situational rather than flat**, and the fiction
carries it: you cannot aim while someone is hitting you.

- Opening from stealth, you are unengaged. Bonus applies.
- Your first same-room shot makes the target engage you, so the bonus drops until
  you break away. **Hit-and-run is rewarded by the same rule that punishes
  standing and trading.**
- A cross-room shot never engages you, so sniping keeps the bonus.
- Fighting behind a tank keeps the bonus. Being the tank does not.

##### The numbers

**All figures below are computed at the owner's REAL Perception of 152**
(`users/3.yaml:43-45`, base 101 + training 51; `StatInfo.Recalculate` sets
`Value = Racial + Training + Mods`). An earlier draft computed the melee rows at
the owner's real Strength but the ranged rows at an unexplained "Perception 110",
which understated every ranged figure by 38% and made a 1.35x gap look like
parity. Compute both sides on the same character or the comparison is worthless.

| | Today | Proposed |
|---|---|---|
| Top bow multiplier | 7.50 | **2.75** |
| `RangedUnengagedDamageMultiplier` | — | **2.75** |
| `SurpriseRangedStrikeMultiplier` | — | **0.5** |
| Unengaged shot, raw | 889 | **897** (~101% of today) |
| Engaged shot, raw | 889 | **326** (~37%) |
| Surprise opener | ~18,005 | **~9,080** |

Against a best-melee opener of **~9,756**, the ranged ambush lands at about 93% —
close, but melee keeps the single biggest hit in the game.

**The top bow must NOT match Blackrazor** (owner, 2026-08-25). Blackrazor's 3.75
is earned by a month-long quest chain, party content for materials, and heavy
crafting; the Ironhorn Warbow is purchasable at 2500 gold. A bought weapon
matching a legendary crafted one is a content-value error regardless of the
combat maths.

At 2.75 the top bow sits at roughly **1.8x the best ordinary melee weapon**
(Heavy Greatsword 1.50) — a real per-shot advantage for a single-shot weapon —
and well under Blackrazor.

The resulting surprise opener, **~9,080**, sits at about 93% of the best-melee
opener of ~9,760 — close enough to be a real alternative, far enough that melee
keeps the single biggest hit in the game, and paid for by the archer dropping to
37% whenever anything is attacking them.

##### Opener and sustained were coupled. A third knob decouples them.

Both the opener and the sustained shot scale with the product
`bow × RangedUnengagedDamageMultiplier`, so with two knobs they could not be tuned
independently. Pinning the opener to melee parity fixed that product near 4.1 and
dropped sustained archery to **55% of today with a tank, 37% solo** — a real nerf
even in the intended playstyle.

**Owner decision, 2026-08-25: add a ranged-specific ambush knob** (reversing an
earlier preference for handling it with the bow table alone, taken before these
numbers existed). *"Just make a separate ranged surprise config knob. Turn up the
non-tanking multiplier, set the ranged surprise strike knob to be lower than
melee's."*

`SurpriseRangedStrikeMultiplier` touches the **ranged opener alone**, so
`RangedUnengagedDamageMultiplier` is free to restore sustained archery without
inflating the ambush:

| Knob | Shipped | Touches |
|---|---|---|
| `RangedUnengagedDamageMultiplier` | **2.75** | every unengaged shot |
| `SurpriseRangedStrikeMultiplier` | **0.5** | the ranged opener only |
| `SurpriseOpeningStrikeMultiplier` | 1.0 | the melee opener only |

Result, all computed at the owner's real stats (Str 136, Per 152):

| | Raw / damage | vs today |
|---|---|---|
| Unengaged shot | 897 | **101%** |
| Engaged shot | 326 | **37%** |
| Ranged opener | **~9,080** | — |
| Melee opener (Blackrazor) | ~9,756 | — |

So sustained archery with a front-line is restored to today's level, being the
tank costs roughly two thirds of it, and the ranged ambush lands at about 93% of
the melee one — big, but no longer the biggest hit in the game.

**Why 0.5 and not 1.0.** The ranged opener inherits `RangedUnengagedDamageMultiplier`
(it is unengaged by definition), so without a counterweight raising that knob to
fix sustained would have pushed the opener to ~18,000. The ranged ambush knob
exists to absorb exactly that. It is **deliberately lower than melee's 1.0**, per
the owner: a bow already answers one fewer defence.

##### Bow detune table

Scaled so the top bow is 2.75. All eight files under
`_datafiles/world/dogmud/items/weapons-10000/`:

| Weapon | Now | New |
|---|---|---|
| Ironhorn Warbow | 7.50 | **2.75** |
| Arbalest | 7.00 | **2.55** |
| Relic Sidearm | 6.00 | **2.20** |
| Hunting Bow | 5.50 | **2.00** |
| Training Bow | 4.00 | **1.45** |
| Primitive Pistol | 3.50 | **1.30** |
| Hand Crossbow | 3.00 | **1.10** |
| Sling | 2.00 | **0.75** |

Note the Training Bow drops from 4.00 — which exceeded every melee weapon in the
game bar Blackrazor — to 1.45, just under a Steel Longsword. That one was
indefensible at any tuning.

##### It compounds with the surprise stack

**Owner decision, 2026-08-25, taken with the consequence stated.** A surprise
shot is unengaged by definition, so it receives both the stacked skullduggery
crit and the unengaged bonus.

With the top bow at 2.75, `RangedUnengagedDamageMultiplier` 2.75 and
`SurpriseRangedStrikeMultiplier` 0.5, the opener lands at roughly **9,080**
against a best-melee opener of ~9,756 — about **93%**, with melee keeping the
biggest single hit.

Getting there took two corrections worth recording. The compounding decision was
taken when the bow was drafted at 3.75; the detune to 2.75 was then chosen
believing it produced parity, but that belief rested on computing the ranged row
at Perception 110 while the melee row used the owner's real stats. Recomputed on
one basis, 2.75 with a 2.0x knob leaves ranged **1.35x ahead** — the very ratio
the detune was meant to remove. Dropping the knob to 1.5 is what actually closes
it.

The alternative offered was to treat the two as alternatives rather than
multipliers. It was declined deliberately: the archer's ambush is *meant* to be
one of the largest single hits in the game, paid for by the archer being
half-strength whenever anything is attacking them.

##### Player-facing consequence

This is a real mechanic a player must be able to discover, not a hidden
modifier. It needs helpfile coverage and, ideally, a perceptible cue when the
bonus is lost. A player whose damage silently halves the moment something turns
on them will read it as a bug. See section 6.

#### 2.8.4 What ranged does NOT get

- No multi-shot surprise. One shot is the whole opener.
- No cross-room bonus (2.8.1, brake 3).
- No change to ordinary (non-surprise) shots of any kind.

---

## 3. Stealth breaks immediately, and the round-boundary machinery is DELETED

An earlier draft of this spec devoted this section to *building* a working
surprise-round boundary. The scope decision in 2.1 removes the need for one
entirely, and the right move is now deletion rather than repair.

### 3.1 Why there is nothing to build

The bonus applies to exactly one attack and is consumed by it. Nothing needs to
persist across the round, so nothing needs a round-scoped flag, a snapshot, or an
end-of-round consumer.

The signal for "this is the opening strike" already exists and already works:
`sourceChar.Aggro.Type == characters.SurpriseAttack`, demoted to `DefaultAttack`
by `SetAggro` on first use inside `calculateCombat` (`combat.go:403-407`). That
is exactly the consume-once mechanism `backstabCrit` uses today, and unlike
`SurpriseLeft` it demonstrably fires in production.

### 3.2 Stealth breaks by DELETING a special case

`Awareness_Cascades.go:36-38` currently special-cases a surprise engagement to
**preserve** `Hidden`:

```go
if r.Trigger == combatphase.TriggerSurpriseAttack {
    return // preserve Hidden through Engaging for surprise
}
```

That preservation existed to keep the ambusher hidden across a multi-swing
surprise round. There is no such round any more.

**Delete the branch.** A hidden attacker then falls through to the ordinary
`Idle → Engaging` cascade that every other attacker already takes, and is moved
`Hidden → Revealing` at the moment they engage. "You lose stealth immediately"
is thereby implemented by removing code, not adding it.

Ordering is safe: `SetAggro` writes `Aggro.Type = SurpriseAttack` **before** the
FSM transition that triggers the cascade (`combat_state_compat.go:123-149`), and
the opening strike reads `Aggro.Type`, not `IsHidden()`. So the attacker is
revealed and still gets their opening strike.

This also resolves **latent bug 4** (1.1) outright: the ambusher can no longer
remain `Hidden` indefinitely, because nothing preserves it in the first place.

### 3.3 What gets deleted

All of it is unreachable or unused once 3.1 and 3.2 land, and **none of it has
ever done anything in production** (1.1, mechanic 3):

- `EngagedData.SurpriseLeft` and the `advanceToEngaged` line that sets it
- `(*Machine).OnCombatRoundEnd()` and `endOfRoundIfSurpriseCallbacks`
- `(*Machine).OnEndOfRoundIfSurprise()` and its registration in
  `Awareness_Cascades.go:47-52`
- `awareness.TriggerSurpriseRoundEnd`, if nothing else references it
- the `=== STUBS — Implementations land in Tasks 6-8. ===` banner
- the `combatphase_test.go` tests that exercise the hand-filled `Reason` path

**Do NOT "fix" `TransitionToEngaging` to carry its reason.** An earlier draft of
this plan added exactly that fix. It is correct in isolation and now pointless:
the only consumer of `EngagingData.Reason` is the `SurpriseLeft` line being
deleted. Fixing a producer whose sole consumer is going away adds a live code
path nothing needs.

> **Record it, though.** `TransitionToEngaging` silently dropping its
> `TransitionReason` is a real latent trap for any *future* consumer of
> `EngagingData.Reason`. U10d removes today's only victim; it does not make the
> function correct. Note it for U11's sweep.

---

## 4. Deletions

| Target | Note |
|---|---|
| `internal/actions/surprise_attack.go` | Entire file, 389 lines: `SurpriseAttack`, `SurpriseAttackOpts`, `SurpriseAttackResult`. `EngageAggroType` is preserved and relocated. |
| `backstabCrit` in `combat.calculateCombat` | Renamed and repurposed, not deleted: the same consume-once flag now selects the opening strike, and `critOnWin` decides whether it crits. |
| the `backstab` parameter and `// consume backstab` return in `calcHitDamage` | Single-swing consumption is exactly the behaviour being replaced. |
| `SurpriseAttackOffhandPenalty` | Config knob. Absent from `config.yaml` and **running at 0.0, NOT its advertised 0.10 default** — see below. |
| `SurpriseAttackExtraArm1Penalty` | As above. Advertises 0.25, runs at 0.0. |
| `SurpriseAttackExtraArm2Penalty` | As above. Advertises 0.40, runs at 0.0. |
| `SurpriseAttackExtraArm3Penalty` | As above. Advertises 0.55, runs at 0.0. |
| `SurpriseAttackExtraArm4Penalty` | As above. Advertises 0.70, runs at 0.0. |
| `EngagedData.SurpriseLeft` + the `advanceToEngaged` line setting it | Never true in production (1.1). No consumer after 2.1. |
| `(*Machine).OnCombatRoundEnd` + `endOfRoundIfSurpriseCallbacks` | Only caller was a test. |
| `(*Machine).OnEndOfRoundIfSurprise` + its registration in `Awareness_Cascades.go:47-52` | Fired by nothing. |
| the `TriggerSurpriseAttack` preservation branch in `Awareness_Cascades.go:36-38` | Deleting it IS the "stealth breaks immediately" rule (3.2). |
| `awareness.TriggerSurpriseRoundEnd` | If nothing else references it after the above. |
| the `=== STUBS ===` comment block in `combatphase.go` | The stubs are deleted, not implemented (3.3). |

The five penalty knobs die with the per-weapon self-penalty concept: every weapon
now contests properly, so a flat self-miss chance per limb is redundant. Their
validators in `internal/configs/config.balance.misc.go` go with them.

> **They are not "running on their defaults", and the difference matters.**
> The validators read
> `if b.X < 0 || b.X > 1.0 { b.X = <default> }`. An absent YAML key unmarshals to
> **0**, which is neither negative nor above 1.0, so the defaulting branch never
> executes and the value stays 0. Deleting them is therefore removing knobs that
> have been **inert at zero**, not knobs carrying live tuning — which is why
> today's burst auto-hits on every limb (1.1) rather than only the primary.
>
> **This validator shape is a trap, not a one-off.** Any knob whose legitimate
> range includes 0 and whose validator only rejects out-of-range values can never
> be defaulted. Worth a sweep in U11's config audit; out of scope here.

**Call sites to update:** `internal/usercommands/attack.go` (PvM, PvP, and the
party-member path at :189), `internal/mobcommands/attack.go`,
`internal/behaviortree/actions_combat.go`.

---

## 5. Config

**Three knobs added, five deleted. Net minus two.**

All three default to **1.0** in Go — a neutral no-op — with the live values in
`config.yaml`. Validate with `<= 0`, never the `< 0 || > 1.0` shape that left the
five deleted knobs inert at zero (section 4).

```yaml
  # RangedUnengagedDamageMultiplier: ranged damage multiplier applied when
  # nothing in the room is targeting the shooter. 1.0 = no change. This is the
  # archer's compensation for firing once where melee swings up to four times per
  # weapon, and for reload sharing the special-move cooldown. It replaces the flat
  # inflation the bow damage_multiplier line used to carry, so an archer shooting
  # from safety is as strong as before while an archer in contact is not.
  RangedUnengagedDamageMultiplier: 2.75

  # SurpriseRangedStrikeMultiplier: the ranged counterpart of
  # SurpriseOpeningStrikeMultiplier, touching the ranged opening shot ALONE.
  # Deliberately below the melee value: a shot answers one fewer defence (no
  # parry), and the opener already inherits RangedUnengagedDamageMultiplier
  # because it is unengaged by definition. Without this counterweight, raising
  # that knob to fix sustained archery would push the ambush to roughly 18,000.
  SurpriseRangedStrikeMultiplier: 0.5
```

```yaml
  # SurpriseOpeningStrikeMultiplier: extra multiplier on the opening strike of
  # a surprise round, applied on top of the stacked weapon-combat and
  # skullduggery crit multipliers. 1.0 = no change. Exists so the ambush can be
  # retuned WITHOUT moving CritDamageBase / CritDamagePerSkill, which are global
  # and would move every crit in the game.
  SurpriseOpeningStrikeMultiplier: 1.0
```

Declared in `internal/configs/config.balance.go` as `ConfigFloat`; defaulted and
validated in `internal/configs/config.balance.combat.go` next to the other crit
knobs. Default 1.0, rejected and reset if `<= 0`.

The knob applies to **both** openers — the melee opening strike and the ranged
surprise shot — so a single dial tunes "how big is an ambush" regardless of
weapon. If playtest shows the two need to diverge, split it then rather than
shipping two knobs on a guess.

Everything else reuses existing tuned knobs: `CritDamageBase`,
`CritDamagePerSkill`, `SkillWeight`, `SpecialMoveCooldown`, `RangedShotScale`,
`SkillMultiplierBase`/`Max`, `SkillSoftCap`.

---

## 6. Player-facing copy

The `*[SURPRISE ATTACK]*` prefix already exists in `calculateCombat` and stays.
Required copy work:

- The **opening strike** needs its own narration, distinct from the other swings
  of the surprise round, so the player can tell which swing was the big one.
- A **missed** surprise round must narrate as the defence that won — **dodge,
  parry or block**, the only three `ChannelMelee` offers (2.2). Do not write copy
  for quell or defy answering a melee swing; those branches are unreachable. A
  missed *shot* can only be dodged or blocked. This is new either way: the ambush
  could not previously be defended at all.
- **No hard numbers.** Damage is reported through
  `combat.GetDamageDescription(amount, targetMaxHP)`, never a raw figure.
- 80-character wrap; no en or em dashes in player copy; ESL-clear phrasing.
- Helpfile coverage for the stealth opener, cross-linked from `help sneak`,
  `help hide`, `help skullduggery` and `help combat`, and registered in
  `_datafiles/world/dogmud/keywords.yaml`. An unregistered helpfile never appears
  in the topic index. The helpfile must cover **both** openers, and must say
  plainly that a shot from stealth gives away your position and that the big
  bonus only applies in the same room.
- **The ranged surprise shot needs its own narration**, distinct from an
  ordinary shot, plus a line telling the shooter they have been revealed. A
  player who loses stealth silently will read it as a bug.
- Existing ranged narration already goes anonymous for a hidden shooter
  (`FireResult.IsSneaking`). Check that the reveal and the anonymity read
  coherently together in one round: the shot is anonymous, then the shooter is
  exposed.
- **The unengaged bonus must be discoverable** (2.8.3). A player whose ranged
  damage silently halves the moment something turns on them will file it as a
  bug, and they will be right to. Required:
  - A **helpfile** statement that shooting is far more effective when nothing is
    attacking you, and that this is why archers want a companion, a party
    front-line, or distance.
  - A **perceptible cue at the transition** — one line the first time a shot is
    taken while engaged, phrased as the archer being unable to steady the shot.
    Do not repeat it every round; once per engagement is enough.
  - No numbers in either. Describe the feel, not the multiplier.

---

## 7. Testing

**Behaviour**

1. The contest is real: a high-defence target denies the opening strike at a rate
   matching the ordinary melee contest for the same scores.
2. `critOnWin` is not `forceCrit`: on a lost contest the opening strike is **not
   upgraded**. Assert no crit and no stacked multiplier — **not** "zero damage",
   which has been false since U6 Task 10 (2.2).
3. **Exactly one swing of the round is upgraded.** Every other swing rolls as an
   ordinary attack, with ordinary crit odds and ordinary mitigation. This is the
   regression test for the retired every-swing reading.
4. A **defended** opening strike neither carries the stacked multiplier nor
   consumes the opening-strike flag prematurely. See the hazard note in 2.2.
5. A **floored** contest never produces an upgraded opening strike
   (`crit_floor.go:122` — a sentinel margin must not be promoted).
6. `SurpriseOpeningStrikeMultiplier` scales the opening strike and nothing else.
7. An ambusher who loses the contest still pays the special-move cooldown and
   still leaves stealth.
8. A hidden attacker on cooldown opens as an ordinary attack (preserves
   `EngageAggroType`'s existing contract).

**Stealth breaks immediately**

9. A hidden attacker is `Revealing` from the moment they engage, whether or not
   anyone retaliates, and whether or not the opening strike landed. Regression
   test for latent bug 4.
10. The attacker still receives their opening strike in the same round they are
    revealed — the bonus keys off `Aggro.Type`, not `IsHidden()` (3.2). This is
    the ordering test; get it wrong and the feature silently does nothing.
11. The deleted `SurpriseLeft` / `OnCombatRoundEnd` / `OnEndOfRoundIfSurprise`
    surface is gone and nothing references it.

**Progression**

11. A landed surprise round awards skullduggery exactly **once**, not once per
    weapon hit.
12. A surprise round that lands **no** clean hit awards no skullduggery
    (success-only).
13. The attacker's combat skill (weapon-combat or unarmed-combat, whichever the
    weapon selects) still progresses per clean hit alongside skullduggery.
14. The defender earns defence-skill progression and the crit-received
    toughening stat from a surprise round. This is the regression test for
    "being ambushed teaches the victim nothing", which is today's behaviour.
15. The attacker's crit bonus tier is paid **at most once** for the round. Note
    2.6.1: because `AttackResult.Crit` reflects only the LAST swing, a
    multi-swing ambush usually pays no crit bonus at all. Assert the documented
    behaviour, not the intuitive one.

**The ranged surprise strike**

16. A same-room surprise shot crits on a won contest and carries the stacked
    skullduggery multiplier.
17. A surprise shot **reveals the shooter**: `Hidden → Revealing` on firing.
    This is the anti-sniping regression test — without it a hidden archer fires
    a maximum-bonus shot every round forever.
18. A surprise shot burns the shared `special-move` cooldown; an **ordinary**
    shot still does not.
19. A **cross-room** shot from stealth is an ordinary shot: no crit upgrade, no
    skullduggery term, no cooldown charge, no reveal.
20. A surprise shot awards skullduggery once, and ranged-combat as before.
21. `AttackSide.CritOnWin` and the melee `critOnWin` parameter produce the same
    verdict for the same contest inputs. Shared test — the two paths must not
    drift.

**The ranged economy (2.8.3)**

22. A shot with **no** inbound attackers carries `RangedUnengagedDamageMultiplier`;
    a shot with one or more does not.
23. The transition is live: the same shooter loses the bonus once a target
    engages them, within the same fight, with no reload or re-equip.
24. A **cross-room** shot keeps the bonus, because it never engages the shooter.
25. The bonus **compounds** with the surprise opening shot (owner decision,
    2.8.3). Pin the product explicitly so a later "simplification" into
    alternatives is caught.
26. The bonus applies to **ranged only** — a melee swing is unaffected regardless
    of the attacker's inbound list.
27. Set to 1.0, each of the three multipliers is a true no-op on its own path.
27a. `SurpriseRangedStrikeMultiplier` scales the **ranged** opener only, and
    `SurpriseOpeningStrikeMultiplier` the **melee** opener only. Cross-wiring them
    is the single most likely implementation slip — the ranged opener would land
    near 18,000 instead of ~9,080 — so assert each knob moves exactly one number.
28. The eight detuned bow multipliers match the 2.8.3 table exactly. A cheap
    table-driven test over the YAML beats trusting eight hand edits.

**Parity and guards**

22. Mob and player ambushers resolve identically (mobs reach this through
    `behaviortree/actions_combat.go`).
23. A site guard in the arc's existing `internal/combat/contest_site_guard_test.go`
    style asserting no production path produces an uncontested surprise hit, so
    the auto-hit cannot be reintroduced.

**Gates**

24. `gofmt -l internal/ modules/` clean; `go build ./...`; tests for every
    touched package.
25. Isolated detached-worktree boot test to `boot-check.exe`, `Server Ready`
    confirmed, exit 124 expected.
26. **Adversarial in-game playtest**, mandatory per the content SOP because this
    ships new player-facing copy. Probe specifically: the sleeping-target
    interaction (2.7), mid-fight re-hiding (2.7), multi-weapon and Extra Arms
    configurations, and whether the ambush feels competitive with a companion
    build rather than merely lethal.

---

## 8. Out of scope

- **Cross-room surprise shots.** In scope for the *ranged* opener generally
  (2.8), but the stacked bonus is same-room only. A cross-room shot from stealth
  stays an ordinary shot, for the reasons in 2.8.1 brake 3.
- **Widening the ranged defence set.** `ChannelRanged` answers with dodge, plus
  block only for a shielded defender. That makes a ranged surprise easier to
  land than a melee one (2.8.2). Correcting it changes every shot in the game,
  not just surprise shots.
- **The off-seam ranged-combat award** at `usercommands/shoot.go:199`, and the
  fact that mob archers earn no ranged-combat progression at all (2.6.4). Both
  are U10b's.
- **The unowned static-difficulty checks** in `actions/search.go`,
  `actions/track.go` and `forager/forage_core.go` remain UNASSIGNED per the
  roadmap.
- **U12** (targeting audit) and **U11** (docs, helpfile sweep, closing playtest
  gate) follow this slice and are unaffected by it.
