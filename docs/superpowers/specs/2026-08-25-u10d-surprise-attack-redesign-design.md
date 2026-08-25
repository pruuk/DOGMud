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
is `roll < 0`, which never fires. **Every primary surprise swing is an
unconditional auto-hit.** The roll applies only to offhand and extra-arm swings,
and even there it is a flat *self*-penalty, not a contest.

There is no defender term anywhere in the function. The target's stats, skills,
defences and equipment never enter. A surprise attack against a novice and
against the Elemental Queen resolve identically.

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

**Mechanic 3 — the surprise-round boundary. This is dead code.**
`combatphase.EngagedData.SurpriseLeft` is set correctly when the
`Engaging → Engaged` transition carries `TriggerSurpriseAttack`, and
`internal/hooks/Awareness_Cascades.go` registers an `OnEndOfRoundIfSurprise`
callback to move the ambusher `Hidden → Revealing` when the surprise round ends.

The only function that fires that callback and clears the flag is
`(*Machine).OnCombatRoundEnd()`. Its sole caller in the entire repository is
`internal/state/combatphase/combatphase_test.go:181`. **Production never calls
it.** The block it lives in is still labelled `=== STUBS — Implementations land
in Tasks 6-8. ===`.

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

### 2.1 Shape: one surprise round, no separate action

`actions.SurpriseAttack` is **deleted**. There is no pre-combat burst.

Being `Hidden` when combat is joined makes **round 1 a surprise round**, resolved
by the ordinary melee path. Every swing that round is an ordinary contested
swing with two modifications (2.2, 2.3).

Rationale: once the defender gets a real roll and the attack side is ordinary
melee, the burst and round 1 roll *the same contest with the same stat and the
same skill*. Keeping both would hand a stealth opener two melee rounds in round 1
and require two code paths to stay in step forever.

**The slice has two halves.** Sections 2.2 to 2.7 describe the **melee** opener,
which is the bulk of the work. Section 2.8 adds a **single ranged surprise
shot** so a stealth build can open with a bow rather than only a blade. They
share the payoff formula and the `SurpriseOpeningStrikeMultiplier` knob, and
they differ in where they plug in (2.2) and in which brakes they need (2.8.1).

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

The defence is the equipment-gated five-defence best-of-all via
`DefenceEntriesFor` — dodge, parry, block, quell and defy all answer normally,
are charged normally, and progress normally.

**Skullduggery does not feed the attack roll.** It amplifies the payoff only
(2.3). Two reasons: skullduggery already gated entry — the attacker had to win
the sneak contest to be `Hidden` at all — and keeping the roll ordinary is what
makes the seam genuinely shared rather than a special case wearing the seam's
clothes.

**On a lost contest the swing simply misses**, exactly like any other melee
swing, and is narrated by the winning defence's ordinary vocabulary. There is no
consolation damage and no partial payoff. Losing the contest is the target
having sensed the ambush.

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

Its value comes from the round snapshot (section 3), not from a live `IsHidden()`
read. Set for **every swing** of a surprise round. So every landed swing that round
crits — which is a genuine buff over today, where `calcHitDamage` consumes
`backstab` after a single swing.

Crit damage already rolls off `sdp.rawDmgForCrit`, the **unmitigated** mean, times
`critDmgMult`. Therefore the burst's old half-mitigation bypass is **strictly
weaker than what crit-on-hit already grants** and is deleted rather than
migrated. Nothing is lost.

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

```
SkillMultiplier caps at SkillSoftCap 50 -> 3.0
raw = 120 * 3.0 * 1.5 * 0.52 * 0.5              = 140
CritDamageMultiplier(69) = 2.0 + 0.05*69        = 5.45
CritDamageMultiplier(50) = 2.0 + 0.05*50        = 4.50
```

| Variant | Per swing | vs a 545 HP veteran |
|---|---|---|
| Today, first swing only | 140 x 5.45 = **765** | 1.4x |
| Stacked, one swing | 140 x 5.45 x 4.50 = **3,443** | 6.3x |
| Stacked on all three weapons | **~10,300** | 19x |

Even a novice stealth build (both skills 5) reaches `2.25 x 2.25 = 5.06x` and
one-shots newbie-tier mobs. Bounding the stack to the opening strike preserves
the assassination fantasy and the "gigantic hit" the owner asked for while
removing the unbounded multiplication across weapons.

Expected round-1 total at the owner's ranks, all swings landing:
`3,443 + 765 x 2 ≈ 5,000` into one target — against a defender who genuinely
got to defend, and zero if the contest is lost.

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

Note the crit tier: **every landed swing of a surprise round crits**, so
`res.Crit` is true whenever the ambush connects. A successful ambush therefore
*always* pays the attacker's once-per-round crit bonus. That is new.

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

Skullduggery has **17** progression sites and **none** of them is on the U9
seam. U9 routed melee, channel defences, spells and taunt; U10b's Category C is
crafting, salvage and forage. The stealth family was claimed by neither.

U10d converts exactly one — its own. The remaining **16** stay bare
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
| `internal/hooks/NewRound_DoCombat_helpers.go` | 1 |

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

**Two more off-seam findings, recorded not fixed.** `ranged-combat`'s only
ordinary progression is that bare `OnSkillUse` in the **player** wrapper, so
**mob archers earn no ranged-combat progression at all**. Both belong to U10b
alongside the skullduggery family; U10d does not touch them, because changing
what an ordinary shot awards is a change to every shot in the game and would
contaminate this slice's playtest.

### 2.7 Edge cases: deliberately not special-cased

Decided by the owner on 2026-08-25: **ship as-is and let playtest speak.**

- **A sleeping target** already carries `ForceCrit` from
  `snapshotSleepingVictims`, so a surprise round against a sleeper is an
  uncontested stacked crit. That is a defensible assassination outcome.
- **An already-engaged target** is not excluded, so re-hiding mid-fight could in
  principle produce a second ambush. The shared special-move cooldown and the
  difficulty of re-hiding in combat are the current brakes.

Both are recorded here so the playtest knows to probe them specifically.

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

**1. Firing does not break stealth. At all.** There is no
`TransitionToRevealing`, no `CancelBuffsWithFlag(buffs.Hidden)` and no
`ForceVisible` anywhere in `internal/actions/combat_fire.go`,
`internal/usercommands/shoot.go` or `internal/mobcommands/shoot.go`. `IsHidden()`
is read once, at `combat_fire.go:153`, and only to set `FireResult.IsSneaking`,
which drives **narration anonymity** and nothing else. Today that is harmless,
because a hidden shot is an ordinary shot. Attach a stacked crit to it and a
hidden archer fires a maximum-bonus shot every round, indefinitely, without ever
being revealed.

> **U10d therefore makes a surprise shot break stealth.** The shooter
> transitions `Hidden → Revealing` on firing a surprise shot. This is a
> behaviour change to the ranged path and must be called out in its own right,
> not folded silently into the redesign.

**2. Fire deliberately never burns the special-move cooldown.** The comment at
the `RecordAndWait` call says so explicitly: *"Fire never burns the special-move
cooldown — only the combat round."* So the brake chosen for the melee ambush
(2.5) does not exist here by default.

> **A surprise shot burns the shared `special-move` cooldown**, matching melee.
> An ordinary shot continues not to. The charge is conditional on the shot being
> a surprise shot, so the existing ranged rotation is untouched.

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

`ChannelRanged`'s defence set is **narrower than melee's**: dodge for every
defender, block only for a shielded one. No parry, no quell, no defy. So a
ranged surprise strike is **easier to land** than a melee one while hitting at
least as hard.

**This is a design decision, not a tolerated imbalance** (owner, 2026-08-25).
The fiction carries it: shooting someone from cover is genuinely easier than
crossing open ground to put a knife in them. The melee ambusher takes the harder
road and is paid in a different currency — the melee surprise round applies its
crit to **every** swing, so a dual-wielder or an Extra Arms build converts a
successful approach into several critting hits, where the archer gets exactly
one shot.

So the two openers are balanced against each other by **hit rate versus volume**:

| | Ease of landing | Payoff if it lands |
|---|---|---|
| Melee ambush | harder — answers five defences | every swing of the round crits; opening strike stacks |
| Ranged shot | easier — answers dodge, and block only if shielded | one shot, stacked |

Do not "fix" the narrow ranged defence set as part of a future surprise-attack
change. If it is ever revisited, it must be revisited as a property of **every**
shot in the game, and this trade has to be re-balanced deliberately at the same
time.

#### 2.8.3 What ranged does NOT get

- No multi-shot surprise. One shot is the whole opener.
- No cross-room bonus (2.8.1, brake 3).
- No change to ordinary (non-surprise) shots of any kind.

---

## 3. The round boundary — the part that must be built

The design says "every hit **that round**". Today there is no working round
boundary (1.1, mechanic 3). U10d builds it, mirroring the existing sleeping
snapshot precisely. `snapshotSleepingVictims` already invites this in its
docstring:

> "Future first-hit-crit triggers (surprise attack, backstab, etc.) can add
> parallel snapshot checks at this same site."

**At the top of `DoCombat`** (`internal/hooks/NewRound_DoCombat.go`), alongside
`snapshotSleepingVictims`: snapshot which users and mob instances are in a
surprise round this tick, and publish to `internal/combat` in the same shape as
`combat.PublishSleepingSnapshot`.

**At the end of `DoCombat`**: call `(*Machine).OnCombatRoundEnd()` for engaged
characters. This fires the already-registered `OnEndOfRoundIfSurprise` callback,
clears `SurpriseLeft`, and moves the ambusher `Hidden → Revealing`. The stub
comment block in `combatphase.go` is removed.

**Why a snapshot and not a live read.** The same reason the sleeping snapshot
exists. `Hidden` breaks mid-round through several paths (the defender's
`CancelCombatBuffs`, `ForceVisible` on retaliation, cancel-on-damage). A live
read would let the attacker's own first swing cancel the surprise for their
second weapon, making multi-weapon behaviour depend on iteration order.

This also fixes latent bug 4: the flag is now consumed, so an ambusher stops
being `Hidden` after their surprise round whether or not anyone retaliates.

---

## 4. Deletions

| Target | Note |
|---|---|
| `internal/actions/surprise_attack.go` | Entire file, 389 lines: `SurpriseAttack`, `SurpriseAttackOpts`, `SurpriseAttackResult`. `EngageAggroType` is preserved and relocated. |
| `backstabCrit` in `combat.calculateCombat` | Replaced by the snapshot plus `critOnWin`. |
| the `backstab` parameter and `// consume backstab` return in `calcHitDamage` | Single-swing consumption is exactly the behaviour being replaced. |
| `SurpriseAttackOffhandPenalty` | Config knob. Absent from `config.yaml`, so it has been running on its Go default 0.10. |
| `SurpriseAttackExtraArm1Penalty` | As above, default 0.25. |
| `SurpriseAttackExtraArm2Penalty` | As above, default 0.40. |
| `SurpriseAttackExtraArm3Penalty` | As above, default 0.55. |
| `SurpriseAttackExtraArm4Penalty` | As above, default 0.70. |
| the `=== STUBS ===` comment block in `combatphase.go` | The stubs are implemented by section 3. |

The five penalty knobs die with the per-weapon self-penalty concept: every weapon
now contests properly, so a flat self-miss chance per limb is redundant. Their
validators in `internal/configs/config.balance.misc.go` go with them.

**Call sites to update:** `internal/usercommands/attack.go` (PvM, PvP, and the
party-member path at :189), `internal/mobcommands/attack.go`,
`internal/behaviortree/actions_combat.go`.

---

## 5. Config

**One knob added, five deleted. Net minus four.**

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
- A **missed** surprise round must narrate as the defence that won (dodge, parry,
  block, quell, defy), not as a generic whiff. This is new: the ambush could not
  previously be defended.
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

---

## 7. Testing

**Behaviour**

1. The contest is real: a high-defence target denies the surprise round at a rate
   matching the ordinary melee contest for the same scores.
2. `critOnWin` is not `forceCrit`: on a lost contest the result is a miss with
   zero damage, never a crit.
3. Every landed swing of round 1 crits; no swing of round 2 does.
4. The opening strike carries the stacked multiplier; offhand and extra-arm
   swings carry the weapon term only.
5. `SurpriseOpeningStrikeMultiplier` scales only the opening strike.
6. An ambusher who loses the contest still pays the special-move cooldown and
   still leaves stealth.
7. A hidden attacker on cooldown opens as an ordinary attack, not a surprise
   round (preserves `EngageAggroType`'s existing contract).

**The boundary**

8. `SurpriseLeft` is cleared after exactly one round.
9. The ambusher transitions `Hidden → Revealing` at the end of their surprise
   round even when nobody retaliates. Regression test for latent bug 4.
10. The snapshot is stable across the round: a mid-round `Hidden` break does not
    downgrade a later weapon's swing.

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
15. The attacker's crit bonus tier is paid **once** for the round, not once per
    critting swing, despite every landed swing critting.

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
