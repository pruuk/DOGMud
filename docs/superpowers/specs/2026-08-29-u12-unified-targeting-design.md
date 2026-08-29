# U12 — Unified targeting and target-switching

**Date:** 2026-08-29
**Arc:** Unified Contest Resolution (`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`)
**Status:** design approved, plan not yet written
**Ships as:** four slices, **U12a** (the seam, proven on the migrated call sites),
**U12b** (the write sweep), **U12c-1** (the read migration), **U12c-2** (the collapse)

---

## 0. Facts verified against source

Every number below was read from source at HEAD `f07edb248` on 2026-08-29.
Re-verify before acting on any of them: this table is evidence, not memory.

| Fact | Value | Where |
|---|---|---|
| `SetAggro` call sites, non-test | **47**, across **21** files | `grep -rn "SetAggro(" --include=*.go internal/ modules/` |
| `EndAggro()` call sites, non-test | **43**, across **24** files | same pattern |
| Combined write surface | **90 sites** | the two above |
| Non-test lines mentioning `.Aggro` | **294** | `grep -rn "\.Aggro\b" \| grep -v _test` |
| `.Aggro` field breakdown, non-test | `!= nil` 84 · `UserId` 61 · `MobInstanceId` 59 · `== nil` 41 · `RoundsWaiting` 20 · `Type` 18 · `SpellInfo` 5 · assignment 4 | field-level grep |
| `Aggro.ExitName` writers | **0** (dead field) | `NewRound_DoCombat_unified.go:238` states it outright |
| `Aggro.SpellInfo` external consumers | **1** (`Death_InboundAggroCleanup.go:52`); the other 4 reads are inside `combat_state_compat.go` itself | grep |
| `combat_state_compat.go` | **211** lines | `wc -l` |
| `internal/state/combatphase` non-test | **530** lines (`combatphase.go` 501, `transitions.go` 29) | `wc -l` |
| `combatphase.RegisterMachine` production callers | **0** (only `combatphase_test.go`) | grep; also asserted at `actions/combat_fire.go:408` |
| `combatphase` production importers | **8** files | grep |
| Roadmap's named U12 surface | **1090** non-test lines, not the ~780 the roadmap row claims | `wc -l` over the six named files |
| Import direction | `behaviortree` → `actions`, never the reverse | grep both ways |
| `activity.CastingData` vs `SpellAggroInfo` | CastingData is a strict **superset**: `SpellId`, `SpellRest`, `TargetUserIds`, `TargetMobInstanceIds` all present | `activity.go:42-56` vs `combat_state_compat.go` |

Two figures quoted during design discussion were wrong and are corrected here:
the write surface is **47** `SetAggro` sites (not 48), and `EndAggro` is a
**second writer with 43 more sites** that was missed on the first pass.

---

## 1. Problem

Targeting is the last unrationalised subsystem in the arc. Three defects, all
structural.

**1.1 Three stores.** A combat target lives in `Character.Aggro`, in
`Character.CombatPhase`'s Engaged state, and (for non-combat picks) in
`EvalContext.SoftTarget`. `Aggro` and `CombatPhase` are kept in sync by a
dual-write inside `SetAggro`, an invariant held by convention with nothing
enforcing it. The accessors disagree about which one leads: `character.go:744`
and `:780` both declare CombatPhase "the primary source of truth" while 294
non-test references read `.Aggro` directly and bypass that ordering entirely.

**1.2 No vocabulary for selection versus commitment.**
`behaviortree/actions_combat.go` contains two target-pickers 70 lines apart
using opposite conventions, with a comment at `:176` declaring one of them
illegal. Neither is a bug. `SoftTarget` exists so a thief archetype can pick a
victim **without starting a fight** (`types.go:48-60`, the "chunk-2.7 bug
class"); the predator archetype genuinely wants combat. The distinction is real
and the codebase had no name for it, so it surfaces as contradictory-looking
code plus a warning comment.

**1.3 A mutable enum standing in for a state.** `Aggro.Type` is demoted from
`SurpriseAttack` to `DefaultAttack` mid-round by `calculateCombat`
(`combat.go:406-410`). Two separate comments warn that a condition written
against it there "would never fire and nothing would fail"
(`NewRound_DoCombat_unified.go:687-690`, `attackresult.go:146-151`). U10d had to
add `AttackResult.WasSurpriseAttack` to route around it. The codebase has
already recorded the intended fix: `NewRound_DoCombat_helpers.go:904` reads
`// TODO Task 18: remove legacy Aggro.Type fallback once Aggro is gone.`

---

## 2. Design

A new package, `internal/targeting`, sitting at the same import level as
`internal/actions`. Both the player paths and `internal/behaviortree` import it;
neither is imported back.

```go
// Selection: who. Pure, no combat consequence.
func Select(c Criteria, s Scope) (state.ActorRef, bool)

// Commitment: enter combat with them. Absorbs today's SetAggro guards.
func Commit(a Actor, ref state.ActorRef, r Reason)

// Release: leave. Absorbs today's EndAggro.
func Release(a Actor, r Reason)

// Query: one place to ask "what kind of engagement is this?". Pure.
func EngagementOf(a Actor) Engagement

// The one deliberate side effect, with exactly one caller.
func ConsumeOpeningStrike(a Actor) bool
```

`Select` and `Commit` are separate verbs because the codebase already proved
they are separate concepts (1.2). Selecting is free; committing starts a fight.

### 2.1 `Engagement`: what is stored and what is derived

```go
type Engagement struct {
    Phase          combatphase.State // STORED   Idle|Engaging|Engaged|Disengaging
    Target         state.ActorRef    // STORED
    OpeningUnspent bool              // STORED   ambush opening not yet thrown
    Casting        bool              // DERIVED  from the activity machine
    Ranged         bool              // DERIVED  from equipped weapon subtype
}
```

The stored/derived split is load-bearing and must survive into the code as a
comment. `Ranged` is derivable, so it is never stored: `SetAggro` already
re-infers it from `Equipment.Weapon.GetSpec().Subtype` on every call.
`OpeningUnspent` is **not** derivable from anything, because U10d made stealth
break immediately, so "this engagement opened from concealment" exists only as
remembered state. A later change that "optimises" `OpeningUnspent` into a
derivation reintroduces the bug U10d fixed.

`EngagementOf` is **pure**. Today the read *is* the write: `calculateCombat`
reads `Aggro.Type`, copies it to the result, and demotes the source in three
consecutive lines. If `EngagementOf` inherited that, every caller asking "is
this an ambush?" would silently spend the ambush. Consumption is therefore an
explicit separate call with exactly one caller, the swing loop.

`AttackResult.WasSurpriseAttack` **stays**. It is not duplication: it is the
post-consumption record, read by progression, messaging and analytics that all
run after the state is spent.

### 2.2 What dissolves

Every `AggroType` member already has a home elsewhere. None of them moves into
`combatphase`. `combatphase` gains exactly **two** new fields, and neither is an
`AggroType` in disguise: `OpeningUnspent` (§2.1) and `RoundsWaiting` (§6.2,
added after the U12c design found it was a live counter with no other home).

| `AggroType` | Home |
|---|---|
| `Flee` | `combatphase.Disengaging`, already wired via `IsDisengaging()`; finishes the Task 18 TODO |
| `SpellCast` | `activity.Casting` + `activity.CastingData` |
| `Shooting` | derived from weapon subtype at point of use |
| `SurpriseAttack` | `Engagement.OpeningUnspent` (state) + `AttackResult.WasSurpriseAttack` (record) |
| `DefaultAttack` | the zero value: nothing special |

`Aggro.SpellInfo`'s single external consumer reads `activity.CastingData`, which
already holds all four of its fields verbatim. `Aggro.ExitName` is deleted as
dead.

### 2.3 Layering, and why there are no exemptions

Two constraints follow from the import graph and must not be quietly reversed.

**`internal/targeting` must NOT import `internal/combat`.** `Select`'s
weakest-mob strategy needs `combat.PowerScore`, but `internal/combat/combat.go:409`
is itself a `Commit` call site that U12b migrates. Importing `combat` would make
that migration an import cycle. The score is **injected** instead, following the
`userUntargetableFn` precedent already in `characters`
(`combat_state_compat.go:49`). A guard test fails if this is ever violated.

**Taunt is on the seam like everything else.** `targeting` imports `characters`,
so `characters` can never import `targeting` — but that is a constraint on where
targeting *logic* may live, not a licence for `characters` to keep committing.
An earlier draft exempted `characters/taunt_hold.go:22`, which would have put the
hole in the seam exactly where the traffic is: **taunt is the most frequent
retargeting mechanic in the game.**

It splits cleanly, because `ForceTauntAggro` has **zero callers inside
`internal/characters`** — its three production callers are
`actions/combat_taunt.go:311`, `:317` and `hooks/pinnacle_tick.go:481`, all in
packages that import `targeting` freely. The `SetAggro` at `taunt_hold.go:22` is
not an independent call site; it is the body of a targeting operation that
happens to live in the storage package.

| Concern | Lands in |
|---|---|
| The three lock fields (`character.go:155-157`) and `SetTauntHold` / `TauntHoldBlocks` / `ClearTauntHold` | `characters` — it is state, and the commit gate reads it |
| "Pin this actor onto that one, then engage" (`CommitTaunt`) | `targeting` — it is a commit with a hold |

`ForceTauntAggro` is deleted. `characters/taunt_hold.go` then contains no commit
at all, and **U12b's AST guard needs no whitelist**.

Two traps in that move, both cheap to get wrong and silent when wrong:

1. `CommitTaunt` must set the hold **before** committing, so the gate sees the
   new taunter as the locked target and lets that very set through. Reversed,
   every taunt no-ops against an existing hold.
2. `ReasonTaunt` must map to `DefaultAttack`. The hold gate pins only
   `DefaultAttack`/`Shooting`/`SurpriseAttack`, so a taunt committing as
   anything else could not hold its own target.

---

## 3. Why three slices

The obvious split is two: build the seam and migrate onto it, then collapse.
That was rejected because it bundles a **90-site mechanical sweep with a
brand-new API**, so every design decision in section 2 would be reviewed at the
bottom of a diff touching 45 files. That is the exact condition under which
U10d's four silent defects nearly shipped.

The obvious fix, mechanical first and design second, is worse and is recorded
here so it is not proposed again: if `Commit` and `Release` land across 90 sites
before `Select` exists, and designing `Select` then shows `Commit` needs a
different shape, all 90 sites are re-touched.

So the order is inverted. **U12a designs the whole package and proves it on four
real call sites, two on each side of the player/mob divide.** If the API is
wrong, that is discovered at four sites instead of ninety.

## 4. U12a — the seam, proven on a small set

**Behaviour change: none.** **Size: S.** All of the design, almost none of the
diff.

1. Build `internal/targeting` with the five members from section 2, plus a
   `context.md`.
2. `Commit` absorbs what `SetAggro` already does: grace-period guard,
   taunt-hold guard, grapple clearing on target change, wait-round computation,
   ranged inference. It takes a `state.ActorRef` and a typed `Reason` in place
   of the int pair and the overloaded `roundsWaitTime ...int` variadic, which
   means "sum these" at two call sites and "use weapon speed" at the other 45.
   `Commit` and `Release` keep dual-writing, so nothing observable changes.
3. Convert the **proof set** and nothing else:
   - `behaviortree` `target_random_player_in_room` (Select, no Commit)
   - `behaviortree` `target_weakest_mob_in_room` (Select then Commit)
   - `behaviortree` `attack`, whose **inline** third copy of the random-player
     picker (`actions_combat.go:38-46`) folds into `Select`
   - `actions.StageMeleeTarget`, the player equivalent
   - **taunt**: `actions/combat_taunt.go:311`, `:317` and
     `hooks/pinnacle_tick.go:481`, via `CommitTaunt` (see 2.3)
   Covering Select-without-Commit, Select-with-Commit, deferred commit, and
   commit-with-a-hold, on both sides of the player/mob divide.

   Taunt was added to the proof set after the first draft. It is the most
   frequent retargeting mechanic in the game, which makes it both the best
   available test of the API and the worst possible thing to leave off it.
4. Extend `internal/actions/ambush_parity_guard_test.go` so the player path and
   the behaviortree path are asserted to reach the same seam.
5. Measure `EngagementOf` on the combat hot path (risk 1) before anything else
   depends on it.

The remaining ~86 write sites still call `SetAggro` and `EndAggro` at the end of
this slice. That is intentional: two conventions coexist for exactly one slice,
which is the price of reviewing the API on its merits.

## 5. U12b — the mechanical sweep

**Behaviour change: none.** **Size: L.** Almost none of the design, all of the
diff.

1. Migrate the remaining write sites onto `Commit` and `Release`, for a total of
   **90** (47 `SetAggro`, 43 `EndAggro`).
2. AST guard, following U5b's no-direct-pool-mutation precedent: no production
   code writes `Aggro` or `CombatPhase` outside `internal/targeting`.
3. Divergence test: after any `Commit` or `Release`, the two stores agree.
4. ~~Delete `SetAggro` and `EndAggro`.~~ **CORRECTED during U12b.** They cannot
   be deleted: `(*Character).Charm` (`characters/charminfo.go:51`) releases
   aggro when the charmer was the current target, and `internal/characters` can
   never import `internal/targeting`. They instead remain what they already
   were in practice — the **package-internal storage primitives** — and U12b
   enforces the *caller* restriction instead: nothing outside
   `internal/characters` and `internal/targeting` may call them, pinned by
   `TestNoDirectAggroWritesOutsideTheSeam`. That is the stronger statement, and
   it survives into U12c, where those two methods are exactly where the
   dual-write collapse happens.

Reviewable as "did each site translate correctly?" with no design judgment
involved. The AST guard, not human attention, is what proves the sweep is
complete.

## 6. U12c — the collapse, in TWO slices

**Refined 2026-08-29 after U12b merged.** U12c splits the same way U12a/U12b
did, and for the same reason: the mechanical bulk must not drown the ~30 lines
that can actually break the game.

### 6.0 Facts that shaped the split

Verified at merged HEAD `5f1ca6b99`.

| Fact | Value |
|---|---|
| Non-test `.Aggro` references | **306** (hooks 97 · actions 73 · behaviortree 35 · characters 26 · usercommands 15 · rooms 12 · targeting 10 · others) |
| …that mean "am I in combat" (`!= nil` / `== nil`) | **127** |
| …that read the target (`UserId` / `MobInstanceId`) | **124** |
| …`RoundsWaiting` | 20 (17 of them the WRITE `= 1`) |
| …`Type` / `SpellInfo` | 21 / 7 |
| …passing the whole struct to `ResolveAggroTarget(*Aggro)` | 18 |
| `IsInCombat()` / `CurrentCombatTarget()` | **Already exist**, already prefer `CombatPhase` with an `Aggro` fallback (`character.go:746`, `:783`) |
| `EngagingData.Reason` | **DEAD** — never written, never read. The only `EngagingData{...}` literal (`combat_state_compat.go:138`) sets `Target` and `RoundsUntil` only |
| `combatphase.OnRoundTick()` | **Driven in production** (`NewRound_DoCombat.go:115`, `:281`) |

⚠️ **`RoundsWaiting` and `RoundsUntil` are NOT duplicates, and they are already
out of sync by construction.** Both are seeded identically by `SetAggro`, then
decremented by different code under different conditions — and the 20
`RoundsWaiting = 1` writes after special moves never touch `RoundsUntil`.
`Aggro.RoundsWaiting` is the live combat-pacing counter: it gates the swing
(`NewRound_DoCombat_unified.go:283`) and renders in the player's prompt
(`userrecord.prompt.go:708`). `RoundsUntil` only drives Engaging→Engaged.
Deleting the former without rehoming its semantics makes every special move
free.

### 6.1 U12c-1 — point the reads at the accessors

**Behaviour change: NONE. Size: L (the bulk).**

| Read | Becomes | Count |
|---|---|---|
| `c.Aggro != nil` / `== nil` | `c.IsInCombat()` / `!c.IsInCombat()` | 127 |
| `c.Aggro.UserId` / `.MobInstanceId` | `c.CurrentCombatTarget()`, returning `state.ActorRef` | 124 |
| `ResolveAggroTarget(c.Aggro)` | `ResolveAggroTarget(ref state.ActorRef)` | 18 |

The accessors already prefer `CombatPhase`, so this migrates reads onto seams
that are already correct. Guard-driven exactly like U12b: an allowlist that
shrinks to empty and fails on stale entries. No playtest.

### 6.2 U12c-2 — the actual collapse

**Behaviour change: yes. Size: M. Owns the arc's adversarial playtest.**

1. **`RoundsWaiting` moves to the combat phase machine** as its own field,
   cleared on Idle. That preserves today's behaviour exactly: `EndAggro` nils
   `Aggro`, so the counter dies with the engagement. It stays DISTINCT from
   `RoundsUntil`.

   **A comment block naming both counters is a REQUIRED deliverable of this
   slice, not a nicety.** The real defect today is not that there are two — it
   is that nothing anywhere says so, so each looks like the only one. The
   comment must state all five of these:

   - `RoundsUntil` is the **Engaging wind-up**: how many rounds before the
     engagement becomes active. `OnRoundTick` decrements it and calls
     `advanceToEngaged()` at zero, which is also what fires the `mob_engaged`
     behaviour-tree event.
   - `RoundsWaiting` is the **actor's round budget**: how many rounds before
     this actor may act again. `handleCombatWaitRound` decrements it *later in
     the same round*, and emits the wait messages.
   - They are seeded identically by the commit path, so during wind-up they
     march in lockstep. That is coincidence of seeding, not shared identity.
   - They **diverge in `Engaged` on purpose**: `RoundsUntil` exists only in
     `Engaging`, while the ~20 special-move `= 1` writes need a counter that
     still works once engaged.
   - `OnRoundTick`'s `Engaged` branch is a **deliberate no-op**. Making it
     decrement is the first step of unification, not a bug fix.

   ⚠️ **Deferred, deliberately: unifying them into one counter.** It is
   achievable and the end state is simpler, but it is a balance change wearing
   a refactor's clothes. One counter means one decrement point, and the two
   decrements happen at different moments in the round (`OnRoundTick` fires
   FIRST, at `NewRound_DoCombat.go:115`/`:281`; `handleCombatWaitRound` runs
   later during resolution). Collapsing them shortens every weapon wind-up and
   every special-move recovery by one round, unless compensated by seeding 2
   where the code says 1 — precisely the sort of invisible `+1` that becomes
   folklore. If it is wanted, it is its own post-arc slice with its own
   playtest, and it must also relocate `advanceToEngaged()` and verify
   `mob_engaged` still fires at the same point.
2. **`AggroType` dissolves** per the table in 2.2: `Flee`→`Disengaging`
   (finishing the standing `// TODO Task 18` at
   `NewRound_DoCombat_helpers.go:904`), `SpellCast`→`activity.Casting`,
   `Shooting`→derived, `SurpriseAttack`→`OpeningUnspent`.
3. **`SpellInfo`'s 7 reads** move to `activity.CastingData`, already a strict
   superset of `SpellAggroInfo`.
4. **`openingStrikeLeft`** graduates from a local in `calculateCombat` to
   engagement state, and `combat.go`'s demotion becomes `ConsumeOpeningStrike`
   — giving it the production caller it has deliberately lacked since U12a.
5. **Delete `EngagingData.Reason`.** This settles U10d's deferred question with
   the second branch of the either/or: it is dead, so it goes. The `r
   state.TransitionReason` PARAMETER is live (it reaches
   `m.inner.TransitionTo`) and stays. ⚠️ Do NOT repurpose either as a home for
   an engagement-kind enum: that moves the demotion bug rather than killing it.
6. **Delete the `Aggro` fallback branches** in `IsInCombat` and
   `CurrentCombatTarget`, then `Aggro`, `AggroType`, `SpellAggroInfo` and
   `combat_state_compat.go`.
7. `SetAggro`/`EndAggro` survive as the storage primitives (see §5 step 4),
   now writing `CombatPhase` alone.

---

## 7. Plugging into mob behaviors

All of this lands in **U12a**: the proof set in 4.3 is the complete list of
target-touching behaviortree actions, so mob behaviors are fully on the seam at
the end of the first slice. U12b and U12c touch no behavior code.

**The authored surface does not change.** Behavior trees reference actions by
string name through `actionRegistry`. Every registered name and parameter stays
identical; no behavior YAML file is touched. Only the action bodies change.

| Action | Becomes | Commits? |
|---|---|---|
| `target_random_player_in_room` | `Select(RandomPlayer)` → `ctx.SoftTarget` | **No.** Preserves the no-combat contract |
| `target_weakest_mob_in_room` | `Select(WeakestHatedMob{RatioBelow})` → `Commit` | **Yes.** Preserves today's behaviour |
| `attack` | its **inline** random-player fallback (a third copy of the picker, `actions_combat.go:38-46`) → `Select`; `EngageAggroType` unchanged; then `Commit` | **Yes** |
| `try_steal` / `try_plant` / `try_shadow` | unchanged; they read `ctx.SoftTarget` | n/a |

`ctx.SoftTarget` keeps its name and its `state.ActorRef` type. It stops being a
third store and becomes the result of `Select` that has not been `Commit`ted.
Downstream readers, including `conditions_skullduggery.go`, need no change.

Mob-specific strategies stay mob-specific: `HatesMob`, `combat.PowerScore` and
the companion-allegiance skip have no player equivalent. `Criteria` is a shared
vocabulary, not a claim that every strategy applies to both sides.

Two gates pinned by test rather than trusted:

- **`delayedActions` membership must not move.** `actions.go:87-91` deliberately
  excludes the target-setters from perception-scaled delay, because a delay
  would open a window where idle ticks re-fire before the target takes effect.
- **Player/mob parity**, via the extended ambush parity guard (4.4).

---

## 8. Scope boundaries and handoffs

- **The behavior unification arc does not rebuild this.** U12 owns *how* a
  target is chosen and committed. That arc still owns *when and why* a mob
  decides to. This must be written into that arc's notes so the seam is not
  reinvented.
- **Mob-side behavioural findings are handed off, not fixed here.** The audit
  reads the behaviortree side; anything behavioural it finds is written up for
  the behavior arc.
- **`combatphase.RegisterMachine` and the always-empty `Attackers()`** are
  already on U11's inbox. The audit confirms them; it does not reclaim them.

---

## 9. Risks

1. **`EngagementOf` is on the combat hot path**, called per actor per round.
   **MEASURED in U12a** (`BenchmarkEngagementOf`, Ryzen 9 5900X, 2026-08-29):

   | Path | ns/op | allocs/op |
   |---|---|---|
   | Melee / idle (the common case) | **53.31** | **0** |
   | Spell-cast | 99.19 | 2 (48 B) |

   The melee case is allocation-free and cheap enough to ignore. The spell-cast
   case allocates because `SpellTargets` is built per call, and that is the one
   number U12c-2 should watch: if the collapse ends up calling `EngagementOf`
   several times per actor per round, consider populating `SpellTargets` lazily
   or hoisting the call, rather than letting a mid-cast actor allocate on every
   read. It is not a problem at one call per actor per round.
2. **90 write sites and ~290 read sites** is the largest mechanical migration in
   the arc. This is why it is three slices: see section 3.
3. **U12c-2 changes behaviour immediately before U11's closing playtest.** U11 is
   the arc's gate and no code slice may land after it, so U12c-2's own adversarial
   playtest is mandatory, not optional.
4. **`Reason` versus `AggroType` could recreate the enum problem.** If the
   `TransitionReason` fix in 6.5 turns into "store the engagement kind on the
   transition", the demotion bug moves house rather than dying. `Reason`
   describes *why a transition happened*, never *what kind of engagement this
   is*.

---

## 10. Done when

1. `internal/targeting` exists with `Select`, `Commit`, `Release`,
   `EngagementOf`, `ConsumeOpeningStrike`, and a `context.md`.
2. Zero production writes to `Aggro` or `CombatPhase` outside it, enforced by an
   AST guard test.
3. `Aggro`, `AggroType`, `SpellAggroInfo` and `combat_state_compat.go` do not
   exist. `grep -rn "\.Aggro\b" internal/ modules/` returns nothing outside
   tests.
4. `NewRound_DoCombat_helpers.go:904`'s Task 18 TODO is gone because the thing
   it waited for happened.
5. Behavior YAML is byte-identical, and `delayedActions` membership is
   unchanged, both asserted by test.
6. The ambush parity guard covers targeting, not only ambush.
7. `EngagementOf`'s hot-path cost is measured and recorded.
8. U12c-2 passes an adversarial playtest before handoff, per the content SOP.
9. `SetAggro` and `EndAggro` no longer exist.
