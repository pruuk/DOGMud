# U5c — attributed death

**Date:** 2026-08-14
**Arc:** unified contest resolution (U0–U11)
**Roadmap:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
**Parent spec:** `docs/superpowers/specs/2026-08-12-unified-contest-resolution-design.md`
**Depends on:** U5b-2 (merged, `7e506d53a`)
**Status:** approved, ready for planning

---

## Summary

Death currently happens on a deferred round-tick sweep that calls
`Die(state.ActorRef{}, ...)` with an empty killer. U5c moves death to the moment
harm lands, carrying a real killer reference and the overkill magnitude, and
keeps the sweep only as a backstop for paths that never call `ApplyHarm`.

---

## What is actually broken

The roadmap says "kill attribution is lost". That is half right, and the half
that is wrong would have shaped this slice badly, so it is corrected here.

**Mob kill credit already works.** `Death_MobKillCredit` reads
`DeadData.DamageMap`, not `DeadData.Killer`, so XP and first-kill credit are
unaffected by the empty ref. U5c is not "fix kill credit".

Six of the eight production `Die` call sites pass an empty `state.ActorRef{}`:

| Site | Victim |
|---|---|
| `hooks/Buff_ApplyBuffs.go:132` | either (DoT tick) |
| `hooks/NewRound_AutoHeal.go:58` | player |
| `hooks/NewRound_DoCombat.go:224` | mob (sweep) |
| `hooks/NewRound_DoCombat.go:433` | player |
| `hooks/NewRound_DoCombat.go:447` | mob |
| `hooks/NewRound_MobRoundTick.go:125` | mob (sweep) |

Only `usercommands/suicide.go` and `mobcommands/suicide.go` name a killer.

### The genuine costs

1. **A zombie window.** A character at or below 0 HP stays `IsAlive()` until a
   sweep reaps it.
2. **Two live consumers of `DeadData.Killer` receive nothing.**
   - `Death_PlayerCorpse.go:40` — gated on `!d.Killer.IsZero()`, so **no gold
     transfers to the killer** on any anonymous path.
   - `PlayerDeath_BountyResolve.go:63` — the guard-kill branch requires
     `killer.IsMob() && killer.MobInstanceId > 0`, so **a bounty-carrying player
     killed by a faction guard is never recorded as a guard kill**. That is the
     justice loop's primary case.
3. **Overkill magnitude is discarded**, and U6's margin-scaled work wants it.

### Three pre-existing defects found while specifying this

- **`ReviveOnDeath` is inert outside the suicide command.** `Die`'s doc claims
  the buff is "already handled at each call site". Only `mobcommands/suicide.go`
  checks it. No combat or DoT death path does, and it is not handled anywhere in
  the `Death_*` observer chain. The flag is used by
  `_datafiles/world/default/buffs/35-death_protection.yaml`.
- **The "Shadow Realm zone guard" in `die.go:21` does not exist.** The only
  occurrence in the repository is that comment.
- **The next swinger steals the credit.** A mob's own turn is skipped once
  `Health <= 0` (`NewRound_DoCombat.go:232`), but incoming attacks are not
  guarded, so whoever swings *next* triggers `Die` at `:446` and is credited
  rather than whoever landed the lethal blow.

All three are fixed here, because centralising death is the moment they stop
being cheap to fix separately.

---

## Design decisions

| Decision | Choice | Why |
|---|---|---|
| Death timing | Event-queued, drained by the existing event loop | Fires outside the damaging call stack, so a mob cannot despawn mid-iteration |
| Overkill | Recorded on the death record | U6 needs it; the record is being built anyway |
| Player deaths | Same path as mobs | One mechanism, no second path to drift |
| `Die` prechecks | Centralised | ~78 harm sites otherwise means ~78 chances to forget one |
| Post-lethal hits | Land, render coup de grace | Refusal loses a player their attacks that round |
| Post-lethal damage credit | Counts toward `DamageMap` | A swing that lands is a swing that counts |

### Why not kill inline in `ApplyHarm`

`Die` fires its observers synchronously, and `Death_MobInstanceCleanup` despawns
the instance inside that call. Killing inline means an instance can vanish in
the middle of any loop that damages several targets. `usercommands.Throw`'s AoE
loop over `room.GetMobs()` is a live example, and combat resolution emits
messaging after the damage call.

### Why not flush at driver boundaries

Deterministic, but every driver has to remember to flush, and a site forgetting
to participate is exactly the failure mode that produced today's six anonymous
`Die` calls.

---

## Architecture

### 1. Detection at the harm site

`Character.ApplyHarm(pool, amount, source)` has carried the source since U5a.
When `pool == PoolHealth`, the change drives health below 1, the victim is
`IsAlive()`, and `DeathQueued` is not already set, it sets `DeathQueued` and
queues the event rather than dying inline. The `DeathQueued` guard is what makes
the killing-blow path fire exactly once.

### 2. The record

A new `events.CharacterDied` carrying:

- victim identity (user id or mob instance id)
- `Killer state.ActorRef` — the `source` already in hand
- `Overkill int` — how far below zero the lethal blow drove health
- `Trigger string` — e.g. `life.TriggerHealthZero`

An empty `Killer` is preserved rather than fabricated. Environmental damage with
no source is anonymous *by truth*, and that is a different thing from today's
anonymity by accident.

### 3. The observer

`internal/hooks/CharacterDied_RouteDeath.go`, registered in
`hooks.RegisterListeners`. Note it is a **listener**, not a Life-machine
observer: the `Death_*.go` family wires through
`c.Life.Inner().AfterTransition(...)`, so this one follows the
`<Event>_<Action>.go` naming used by `Buff_ApplyBuffs.go` instead.

1. Re-resolve the victim. It may already be gone; no-op on nil.
2. Re-check `IsAlive()`. A second queued event for the same victim is inert.
3. Run the centralised prechecks:
   - `ReviveOnDeath` — heal above 0, cancel the buff, **no death**.
   - Suicide dedupe (`LastSuicideRound`).
4. Call `Character.Die(killer, trigger)`.

### 4. Two distinct states, and they must not be conflated

This is the easiest thing in the design to get wrong, because the obvious
simplification is broken.

**Dying** — `Health < 1 && IsAlive()`. Needs no new flag. The scattered
`Health <= 0` guards already in combat are partial implementations of this idea;
U5c makes it consistent. It drives combat targeting and coup de grace rendering.
The killing-blow path fires exactly once, on the first blow across that line.

**Death queued** — an explicit runtime-only marker (`DeathQueued bool` on
`Character`, `yaml:"-"`), set when `ApplyHarm` queues the event and cleared when
the death resolves or a revive cancels it.

They are not the same state, and the sweeps depend on the difference. A
character reaped by a sweep is by definition **dying but not queued**: it reached
0 HP without going through `ApplyHarm`. If the sweeps skipped everything that is
merely *dying*, they would skip exactly the population they exist to reap and
nothing would ever die on the non-harm paths.

So: the sweeps skip on `DeathQueued`, never on `Health <= 0`.

### 5. Coup de grace

Hits landing on a dying target in the same round still resolve: they deal
damage, they count toward `DamageMap`, and they render a **coup de grace**
message set rather than ordinary hit text or a second kill message. New pool
under `_datafiles/world/dogmud/combat-messages/`, following that directory's
existing `optionid` + `options` token-substitution pattern.

They do **not** re-queue a death, do not re-attribute, and do not change the
recorded overkill.

### 6. The backstop

Both sweeps stay — `NewRound_DoCombat.go:219` and
`NewRound_MobRoundTick.go:125` — for anything that reaches 0 HP without going
through `ApplyHarm`. Two changes:

- They **skip any character whose `DeathQueued` marker is set** — never on
  `Health <= 0`, per section 4. Without this the sweep reaps the victim first,
  `Die`'s idempotence makes the attributed event a no-op, and attribution is
  silently lost while everything still looks correct. This is the subtle failure
  mode of the whole design.
- They **log when they fire**, so we learn which paths still bypass the harm
  helper instead of assuming none do. That log going quiet is the evidence the
  migration is complete.

### 7. Deletions

Per the arc's "delete as you migrate" rule: the anonymous `Die` calls made
redundant by harm-site routing, and the phantom Shadow Realm line in `Die`'s
doc comment.

---

## Timing

Everything runs single-threaded under `util.LockMud()`. `EventLoop` takes the
same lock, so the flush cannot interleave with turn processing: queued deaths
land after the current turn block completes, not mid-round.

The zombie window therefore shrinks from "until the next sweep" (up to a full
round) to "the remainder of the current turn block", and the dying target is
inert for that remainder by the invariant in section 4.

---

## Edge cases

| Case | Behaviour |
|---|---|
| Victim despawned before the flush | Observer no-ops on nil; `Die` is idempotent regardless |
| Two lethal blows same tick (AoE plus DoT) | Second event inert via the `IsAlive()` re-check |
| Killer despawned before the flush | Ref may be stale. `attributeBountyKill` already tolerates `killerFactions` returning nothing |
| Legitimately sourceless harm | Empty ref preserved, death is genuinely anonymous |
| `ReviveOnDeath` fires | Must heal above 0. Skipping the death while leaving health negative just hands the kill to the sweep next tick |
| Sweep races a queued death | Sweep skips on `DeathQueued`, never on `Health <= 0` (sections 4 and 6) |
| Revive cancels a queued death | Observer clears `DeathQueued` alongside healing, or the victim can never be killed again |

---

## Testing

**Unit**

- Lethal health harm queues exactly one `CharacterDied` with the correct killer
  and overkill.
- Non-lethal harm queues none.
- Stamina and conviction harm never queue, at any magnitude.
- Observer honours `ReviveOnDeath`: heals, cancels the buff, does not die, and
  **clears `DeathQueued`** so the character remains killable afterwards.
- A duplicate event for the same victim is inert.
- A character that is dying but never queued (0 HP via a non-harm path) is still
  reaped by the sweep. This is the test that catches conflating the two states in
  section 4.

**Regression**

- Gold transfers to the killer on a mob-killed-player death
  (`Death_PlayerCorpse`).
- The guard-kill branch fires in `attributeBountyKill` when a faction guard
  lands the kill.
- The lethal blow is credited, not the next swing.

**Backstop**

- The sweep still reaps a character that reaches 0 outside a harm path.
- The sweep does not pre-empt a pending attributed death.

**Safety**

- No player-sourced harm can reach another player. `PVP: disabled`, player
  `HarmArea` targets mobs only, and `Throw` iterates `room.GetMobs()` only.
  Asserted by test so a future change cannot quietly open that door.

**Arc rules**

- No balance literal under `internal/`; any tuning value goes in `config.yaml`.
- `context.md` updates ship in the same PR.

---

## Out of scope

- **U6's three modelling gates.** Untouched: special-move skill weight,
  counterattack frequency, crit-floor denominator.
- **`MobDeath.KillerMobInstanceId`**, whose own comment admits it is
  "last-aggro-target, not last-hit". A real killer ref could improve it. Not
  required by U5c; noted so the option is visible.
- **PvP.** Disabled, and nothing here changes that.

## Follow-ups

- **Death degradation bias.** Observed on prod: death is meant to degrade a
  random stat and a random skill, but appears to always hit skullduggery and
  dexterity. Investigate after U5c lands. Note that the obvious search terms
  (`degrade`, `DeathPenalty`, `DecreaseSkill`) do not locate it; start from the
  `Death_*` observers.
- **Verify the shipped `GoldLossFraction`.** Absent from `config.yaml`, so the
  Go default applies. If it is 0, the gold-transfer fix is latent rather than
  visible.
