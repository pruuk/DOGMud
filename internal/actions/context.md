# actions — Package Documentation

## Overview

The `actions` package provides the core runtime implementations of in-game
actions — both simple queries (Consider) and complex multi-phase operations
(Combat, Skill, Craft, etc.). Each action is implemented as a public function
that accepts an `Actor` (player or mob abstraction) and target context, then
returns a `*Result` struct with outcome data.

**Structure:**
- Actions are called by user commands (`internal/usercommands/`), mob commands
  (`internal/mobcommands/`), and behavior tree primitives
  (`internal/behaviortree/`).
- Each action returns a structured result for programmatic consumption.
- Shared actions own mechanical outcomes. Player-command wrappers retain
  private player-only rendering decisions; in particular, shared cost admission
  returns `characters.CostCommitResult` and never emits a refusal line for an
  actor.
- Skill progression (`OnStatUse`, `OnSkillUse`) is triggered within actions,
  not by callers.

---

## Actor Abstraction

The `Actor` interface unifies player and mob behavior:

```go
type Actor interface {
	GetCharacter() *characters.Character
	GetRoom() *rooms.Room
	SendText(cat messaging.Category, msg string)
	GetName() string
	IsPlayer() bool
	GetUserId() int                 // 0 for mobs
	GetMobInstanceId() int          // 0 for players
	AddCondition(conditionId int, source string)
	OnSkillUse(skill string) bool
	OnStatUse(stat string) bool
	AwardResolved(won bool, cands ...progression.Candidate)
}
```

`OnCriticalSuccess` / `OnCriticalFailure` were on this interface before U9 and
are gone; crit and fumble progression flows through `progression.BonusEvents`
plus `ApplyProgression` now. A few test fakes still carry the two methods
harmlessly.

`AwardResolved` (U10b-1) is the Best-of firing seam: candidates arrive already
rolled, the highest-rolling one earns a single event, paid at full weight on a
win and at `Balance.ProgressionFailureFraction` on a loss. Like `OnSkillUse` it
takes no userId -- `UserActor` supplies its own, `MobActor` supplies 0. Build
candidates with `Character.CandidateFor` in almost every case: it leaves
`Candidate.Stat` empty, which means "the skill's primary" and pays one roll.

The exception is a site that deliberately trains a stat OTHER than the skill's
primary. `ExecuteGrapple` is the one in this package: it hand-builds a
`progression.Candidate` naming `unarmed-combat` with `Stat: "strength"`, which
is what gives strength a faucet now that the old bare attack-side stat roll is
gone. ⚠️ A populated `Stat` that DIFFERS from the primary makes
`ApplyProgression` pay a SEPARATE stat roll on top of the skill's. That is
intended here and is the whole point, but it is also why you should not
populate `Stat` "for clarity" when it merely repeats the primary: doing so
would silently double-roll. `combat.DefenceSkillAndStat` follows the same rule
for block (weapon-combat/strength) and defy (rhetoric/willpower).

### Action-cost admission

`admitFullCost` is the internal voluntary-action seam. It creates a neutral
single-unit request, then delegates the full-or-refuse decision, pool update,
and fractional carry to `Character.QuoteActionCost` and `Character.CommitCost`.
It performs no direct `ApplyCost*` call and emits no private text. U8 shared
action results carry the returned `CostCommitResult`; user wrappers can
render a refused status through `CostRefusalText`, while equivalent mob wrappers
stay silent.

- **UserActor** (`actor_user.go`): wraps a `*users.UserRecord`, sends text via
  `user.SendText()`, skill progression goes through `user.Character.OnSkillUse()`.
- **MobActor** (`actor_mob.go`): wraps a `*mobs.Mob`; `SendText` is a no-op
  because a mob has no private player connection. A mob's room line goes out
  through `Say`/`Shout`/`SendHeard`/`SendSeen` below, not through the Actor
  interface itself (`SendRoomCommunication` is deleted, sight gates slice
  5b). Skill progression goes through `mob.Character.OnSkillUse()`.

---

## Drink (`drink.go`, drink path unification 2026-09-28)

`Drink(actor DrinkActor, rest string) DrinkResult` is the ONE drink body for
players, mobs and the AI companion. It was `usercommands.Drink`; the mob
command was a separate copy with no toxicity, no aging or crafter scaling and
no special potions. `usercommands.Drink` and `mobcommands.Drink` are now thin
wrappers, and the repo-root `drink_wrapper_guard_test.go`
(`TestDrinkWrappersDoNotReFork`) fails if either grows drink rules again.

- **`DrinkActor`** is `Actor` plus `AddConditionScaled` and
  `AddConditionMagnitude`. It is its own interface, not two new `Actor`
  methods, because many test fakes implement `Actor` and none of them drinks.
  `UserActor` and `MobActor` both satisfy it; each queues `events.Condition`
  through the event door, so the drinker reads the condition's start line.
- **`DrinkResult`** reports `Drank` (true for a spoiled potion too),
  `Spoiled`, `ItemId` and a `Refusal`.
- **`DrinkRefusal`**: `DrinkOK`, `DrinkRefuseBusy`, `DrinkRefuseGrappled`,
  `DrinkRefuseNotFound`, `DrinkRefuseNotDrinkable`, `DrinkRefuseToxicity`.

Every special potion (Ysolde's Purge, the Purging Draught, the Bloom Wafer,
the Catalyst of Unmaking, the Phial of Second Birth) applies fully to a mob
(owner ruling 2026-09-28). `SendText` is a no-op for a mob, so a mob drinks
silently apart from the room line. Both room lines go out through
`Room.SendTextVisualHidingNames` with the drinker's name: an observer at the
shapes tier reads "a figure", one who sees nothing reads nothing, and a player
drinker is excluded from their own line (`drink_room_line_sight_test.go`).
`drink_parity_test.go` holds the player and mob parity table.

`Drink` no longer snapshots a `tick_pool` condition's per-round amount (tick
amount at apply, 2026-09-28): the old `ComputeTickAmount(...)` /
`Conditions.SetTickAmount(...)` block after `AddConditionScaled` is gone.
`AddConditionScaled` still queues `events.Condition` with no `TickScale`, so
`hooks.setTickAmountAtApply` (`Condition_ApplyConditions`) computes the
amount where the record lands, at the fallback scale of 1.0 — a potion has
no caster to scale by. `DrinkActor` does not need
`AddConditionTickScaled`: a drink's tick amount was already scale-1.0 before
this slice, so nothing about a potion's strength changed, only where the
number is computed.

---

## Combat Actions

### Every player attack path MUST seed aggression

`SeedAggression(user, mob, room, freshAggro)` in `aggression.go` is what makes
an attack visible to the revenge, opinion and justice systems. An attack path
that does not call it is invisible to all three: no assault crime, no faction
rep hit, no bounty check, no witness knowledge, no revenge-mob seeding.

That was not hypothetical. Until 2026-08-14 only `attack`, `shoot` and (half of)
`taunt` seeded anything, so the ten melee specials and `throw` let a player open
on a faction NPC for free. Killing it was still caught by the death hook's
murder record, but assaulting and walking away cost nothing.

**You almost certainly do not need to call it directly.** Admission-gated
physical specials call `StageMeleeTarget` and carry its returned `Actor` into
their shared `Execute*` function. The action resolves that staged target
read-only, admits cost, consumes cooldown, then commits aggro and calls
`SeedAggression`. Invalid, stale, cooldown-blocked, and cost-refused attempts
therefore cannot seed combat side effects. Taunt uses the same staged engagement
contract. `AcquireMeleeTarget` is **deleted**: it was the pre-U8 eager
gate-and-engage helper, it had no production callers left, and keeping it would
have kept an engage-before-paying path available to a future command. `throw`
seeds from its own `engageAfterThrow` because it is an AoE.

The two halves fire on **deliberately different** conditions, and getting this
backwards spams crimes and bounties off a single engagement:

| Signal | Fires when | Why |
|---|---|---|
| `PlayerAttackedMob` event | **every** commitment | Seeder rules 6 and 9 want repeated aggression, not just the opener |
| Opinion bump + assault crime | **fresh aggro only** | Otherwise every kick of a long fight re-logs a crime and re-bumps rep |

"Fresh" is the caller's judgement because it differs by shape. Single-target
moves compare the *attacker's* prior aggro against this mob. An AoE like `throw`
judges it *per mob*, from that mob's own prior aggro, because the attacker's
aggro can only point at one target and attacker-side gating would record the
first mob hit and silently miss the rest.

`RecordAssaultCrime` lives here rather than in `internal/usercommands` for this
reason: the specials engage through this package, and `usercommands` already
depends on it.

### Thrown weapons: `throw` is the grenade verb, aimed throws are ranged

Settled 2026-08-14, recorded here so it is not re-argued:

- **`throw`** (in `internal/usercommands`) is untargeted **by design**. It takes
  an item, never a target, and resolves as a room AoE rolled independently
  against every hostile present. Same shape as an AoE spell. Do not add a target
  argument to it.
- **Aimed thrown weapons** (darts, javelins, throwing knives) belong under
  `ranged-combat` and `ExecuteFire`, which already has single-target resolution,
  cross-room shots, Perception-based aiming, reload machinery, and correct crime
  and revenge seeding. Skullduggery suits an improvised explosive; it does not
  suit a javelin.

Not yet built. The one open problem is that a thrown weapon is its own
ammunition, while `ExecuteFire` requires a wielded ranged weapon via
`findRangedWeaponSlot`. Either such a weapon equips and consumes itself on use,
or a `thrown` weapon subtype gets taught to that resolver. A feature, not a
refactor.

### Basic Attack

**Function:** `combat.AttackPlayerVsMob(user, mob)`, `combat.AttackMobVsPlayer(mob, user)`, etc.

Handled by the combat system (`internal/combat/`), not the actions package.
See `internal/combat/context.md` for full details.

### Special-Move Actions & Anatomy Gating (Phase 2)

The shared special-move actions (`ExecuteGrapple`, `ExecuteBash`,
`ExecuteTrip`, `ExecuteKick`, `ExecuteHamstring`) each carry a
defense-in-depth anatomy guard mirroring the AI `CanUse*` gate in
`internal/combat/ai.go` and the parity check in `command_readiness.go`:
grapple/submit need `arms`, trip/kick need `legs`, bash needs
`(shield|NaturalBash) AND (arms|NaturalBash)`. These three sites form a
`// SYNC POINT` triad — change one, change all; `command_readiness_drift_test.go`
asserts the gate and `CommandIsReady` stay in agreement (the `*_no_arms`
/`*_no_legs` rows). The action-entry guard is unreachable for players
(always humanoid) and reuses the nearest existing failure flag
(`GrappleImmune`, `NoShield`, `NoTarget`) rather than minting a new one.

**Retired:** `ExecuteBite`/`BiteResult` were removed — biting is now the
Phase-1 basic attack for fanged species (species `NaturalAttack`).
`mobOnlyCommands` no longer lists `bite`. `toxic-bite` (mutation) stays.

### Beast Special Moves (Phase 3)

Six new `Execute*` actions gated by exported species-identity predicates
from `internal/combat` (`SpeciesIsFanged`, `SpeciesIsClawed`,
`SpeciesIsHorned`, `SpeciesHasLifeDrain`, `SpeciesIsQuadrupedPredator`).
Each action checks the predicate at entry as defense-in-depth and returns
a `Not<Identity>` result if the caller's species doesn't qualify. The
same predicate is checked in `ai.go` (`CanUse*`) and
`command_readiness.go`; drift rows in `command_readiness_drift_test.go`
keep all three sync points in agreement.

**Phase 4 — the `hands` gate.** Each of the six beast actions (incl.
`ExecuteHamstring`, which gained an explicit identity gate in Phase 4)
ALSO rejects an actor with a `hands` body part at the entry — beast
natural-weapon moves are for true beasts, not tool-using humanoids. The
`*_hashands` drift rows pin it. `ExecuteDrain` is EXEMPT (LifeDrain-flag
gated), so armed undead still drain.

| Action | Gate | Effects |
|--------|------|---------|
| `ExecuteRake` | clawed | Damage + bleed stack |
| `ExecuteMaul` | fanged | Heavier damage + bleed stack |
| `ExecutePounce` | quadruped predator, not grappling | Knockdown + damage (no bleed) |
| `ExecuteGore` | horned | Damage + knockback |
| `ExecuteDrain` | `LifeDrain` flag | Damage + heal attacker (`damage × DrainHealRatio`) via `Character.Heal` |
| `ExecuteThrottle` | fanged | Damage + bleed stack + Throttled condition #89 (stamina DoT) + cast interrupt via `InterruptTargetCast` |

**Bleeds stack (slice 1b, owner ruling 2026-09-14).** Rake, maul, hamstring,
drain (`ExecuteDrain` and each landed target of `ExecuteDrainArea`) and throttle each add one
stack to the target's 122 Bleeding record on a landed hit, through
`AddConditionMagnitude(conditions.ConditionIdBleeding, rounds, -amount, source)`. The stack's
per-round amount is `bleedPerRound` (`bleed.go`): the attacker's Strength
`ValueAdj` divided by the move's `<Move>BleedStrengthDivisor` knob, floored at
`<Move>BleedMin`; its rounds are `<Move>BleedRounds`. All fifteen knobs live in
the Bleed stacks block of `config.yaml`. Stacks from repeated hits add up and
keep ticking after the fight; see "Stacking records" in
`internal/conditions/context.md`. Each result's `BleedDmg` carries the per-round
amount of the stack just added, and nothing outside this package reads it.

**`InterruptTargetCast`** is a shared helper that reuses the engine's
existing `activity.TriggerCastCancel` cast-cancel path (conviction
refund included). No new silence flag is introduced. `ExecuteThrottle`
gates the call behind an opposed concentration contest (U10): the
throttler's grip (`Dex + unarmed-combat×SkillWeight`) against the
target's hold (`Wil + spellcasting×SkillWeight`) via
`combat.RunConcentrationContest`, floored only by `ConcentrationFloor`
(0.02) — not the old flat `ThrottleInterruptChance` coin flip, which is
deleted. A held contest fires success-only spellcasting progression for
the target instead of an interrupt.

---

## Flee (`flee.go`, `relocate_mob.go`, slice 4a)

A player and a mob flee through the same two functions. Before 4a a mob's
flee was free, instant, and ignored roots, standing, and whether it was even
fighting; a player's flee paid stamina, took a round to resolve, and could be
blocked. Now both go through `BeginFlee` (the command half) and `ResolveFlee`
(the round half), and `usercommands.Flee` / `mobcommands.Flee` are thin
wrappers that render the outcome.

- **`FleeGate(c *characters.Character) FleeRefusal`** is every refusal a flee
  can know before it transitions: rooted (`conditions.NoMovement`), no-flee
  (`conditions.NoFlee`), already disengaging, not in combat, grappled
  (`IsStandingGrapple`/`IsGroundGrapple`), not standing. It duplicates
  `CombatPhase`'s position veto on purpose, because that veto is registered by
  `internal/hooks`; without the duplicate a flee's outcome in a package that
  does not link `hooks` would silently differ.
- **`BeginFlee(actor Actor, preferredExit string) FleeBegin`** runs the gate,
  publishes a pending `characters.FleeAdmission`, transitions the character to
  `combatphase.Disengaging`, then quotes and partially commits the flee
  stamina cost (`configs.Balance.FleeStaminaCost`, halved by
  `FlightFleeStaminaMult` when `mutations.IsFlying`). A short payment never
  refuses the flee; it drops Skullduggery from the round's blocker contest
  instead (`FleeBegin.Short`). `preferredExit` is carried to the round (a
  kiting archer's retreat-toward-home exit); empty means a random passable
  exit at resolution.
- **`ResolveFlee(actor Actor, room *rooms.Room) FleeOutcome`** is the round: it
  takes the ready admission, refuses a fleer grappled since the command,
  runs `combat.ResolveFleeBlockers`, awards Skullduggery only when the
  contest happened and the fleer paid in full, picks the exit
  (`fleeExit`, preferred-then-random), and settles `CombatPhase.ResolveFlee`
  (success or back to Engaged). It does not move the fleer — the caller does,
  because a player's move differs from a mob's (`Look`, charmed followers and
  `room_enter` for one; `RelocateMob` and the `mob_flee` behaviour event for
  the other).
- **`RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room, sneaking bool)`**
  is the mob's move with no gate and no charge: walking
  (`mobcommands.Go`, after its own lock check, passing `actions.MobIsSneaking(mob)`)
  and a successful flee (`hooks.handleMobFlee`, passing
  `actions.MobIsSneaking(mob)`) both end here. It removes the mob from `from`, calls
  `ClearRoomAggroOnDeparture`, adds it to `dest`, narrates both sides
  (sight-gated, with a sound fallback) unless `sneaking`, plays the movement
  sounds either way, and pulls an NPC party's idle (not in-combat) members
  through the same exit. A sneaking mob sends no exit, entry or next-room
  line, as a sneaking player never has (parity slice 6, ruling D1).
- **`ClearRoomAggroOnDeparture(room *rooms.Room, departingInstanceId int)`**
  moved here from the (still unexported at the call site) `mobcommands`
  version; retargets or releases players and mobs in `room` that were
  targeting the departing mob.
- **The wrappers must not re-fork the mechanics.** `usercommands.Flee` and
  `mobcommands.Flee` may only call `BeginFlee` and render `FleeBegin`;
  `hooks.handlePlayerFlee` and `hooks.handleMobFlee` may only call
  `ResolveFlee` and render `FleeOutcome`. `flee_wrapper_guard_test.go` (repo
  root) greps the four files for a second flee-cost or flee-resolution
  implementation.

---

## Movement (`move.go`, movement parity 4b)

Before this slice a mob's walk was free (no action points, no stamina) and
rolled no hidden-detection contest either way; a player's walk paid both and
rolled detection twice (as the sneaking mover and as the newcomer). Every
live mob also held 0 action points, because nothing ever set or regenerated
them (`internal/mobs/context.md`, `internal/characters/context.md`). Now a
player and a mob pay the same price for the same step and run the same
detection contests, through this file. `usercommands.Go` and
`mobcommands.Go` keep their own lock gates, narration and Search roll
placement and call these bodies once, right before relocating.
`move_wrapper_guard_test.go` (repo root) greps both wrapper files for a
second price or detection implementation.

**Cost:**

```go
type MoveRefusal int
const (
	MoveOK MoveRefusal = iota
	MoveRefuseEncumbered
	MoveRefuseTired
	MoveRefuseExhausted
)

type MoveCharge struct {
	Refusal     MoveRefusal
	ActionCost  int     // 10, or 50 over carry capacity
	StaminaCost float64 // fractional stamina price, banked through the carry
	Winded      bool    // paid, and stamina is now under a quarter of its reachable max
	Never       bool    // refused, and this actor could not pay the step even fully rested
}
func (m MoveCharge) OK() bool

func MovePrice(c *characters.Character, dest *rooms.Room) (actionCost int, staminaCost float64)
func QuoteMove(actor Actor, dest *rooms.Room) MoveCharge  // prices, spends nothing
func ChargeMove(actor Actor, dest *rooms.Room) MoveCharge // pays, or refunds AP on a stamina refusal
func QuoteMobStep(mob *mobs.Mob, exitName string) MoveCharge // quotes a mob's next step by exit name; a step that does not resolve quotes as affordable
func TrainSearchOnMove(actor Actor) // the rare Search training a completed step earns
```

`MovePrice` reads the destination biome by NAME (`rooms.GetBiome(dest.Biome)`)
and multiplies by `FlightMoveStaminaMult` when the mover is flying. The
action-point price is the hardcoded 10 (50 over carry capacity), not a knob.
`QuoteMove` and `ChargeMove` both settle a MOB'S action points first
(`SettleActionPoints`, see `internal/characters/context.md`) and never settle
a player's: the per-turn hook (`hooks.ActionPoints`) already credits players,
and settling here too would double-credit the same turns. `Never` is true
when the actor could not pay the step even fully rested (pool max below the
price), so a walker can tell "wait, it will recover" from "give up, it never
will" (spec deviation: added beyond the spec's plain refusal).
**`ChargeMove` must run AFTER a wrapper's lock gate and any exit-message
requeue**, so a door that stays locked costs nothing and a requeued step is
charged once, not twice.

**Detection:**

```go
type EntryDetectionResult struct {
	StillSneaking bool // false once a sneaking mover has been spotted
}
func EntryDetection(mover Actor, dest *rooms.Room, sneaking bool) EntryDetectionResult
```

(Spec deviation: the spec names both a function and its result type
`EntryDetection`; Go forbids that, so the result type is
`EntryDetectionResult`.) Symmetric for a player or a mob mover (owner ruling
3): a sneaking mover rolls against every player observer first (the one who
spots it is told) then every mob observer (silent), skipping the mover's
allies (`alliesOf`). For a player that is its player party, as on master,
plus its own charmed mobs and companions (`GetCharmIds`); for
a mob it is its NPC party and, when charmed, its owner, the owner's party and
the owner's other charmed mobs and companions. Once the mover is not
sneaking, whether it never was or was just spotted, it rolls against every
hidden occupant of `dest` and calls `mover.AwardResolved` on BOTH outcomes
(U10b-2), win or lose, because a search roll happened either way. A mob
newcomer skips its `alliesOf` side there too, so a pet or party member
following a hidden leader never reveals it or trains Search on it; a player
newcomer skips only itself, exactly as on master. Moved from `usercommands.Go`, unchanged in shape; a mob mover's lines
differ only in name colour (`moverName`).

**The rare Search roll** (`movementTrainsSearch`, `TrainSearchOnMove`) is
unchanged in mechanism from before this slice, moved here so a mob's
completed step can earn it too. The full rationale, moved verbatim from
`usercommands.movementTrainsSearch`:

> U7 prices movement partly on the actor's search rank, so travelling has to
> be able to earn that discount -- today movement trains nothing at all,
> which leaves nearly every live character at rank one with no way to
> improve it by walking. But walking must stay the SLOW road: search is
> already easy to raise through forage, search and track, and it should
> never be the case that the best way to become a tracker is to pace back
> and forth.
>
> The rarity is in whether the use is RECORDED, not in the odds attached to
> it.
>
> NOTE: the original reason no longer holds. CheckSkillProgression derived
> its decay from the use count (virtualRank = useCount / UsesPerRank), so
> recording a use per step would have buried the counter and devalued
> forage, search and track. Since U10b-0 Phase C the rank IS the skill
> level, so frequency no longer exhausts the curve and UsesPerRank drives
> nothing.
>
> The gate stays anyway, on the simpler ground below: a roll now happens on
> every recorded use, so recording one per room step would make walking the
> fastest route to a search rank regardless of how the curve decays. Scaling
> the odds down instead is not equivalent -- it would still pay out steadily
> for an activity that costs the player nothing.
>
> STALE FIGURES, kept for intent only: at the shipped 1-in-200 gate this was
> reckoned at roughly 7,700 room moves to search rank 10 and around 33,000
> to rank 35, against roughly 8,300 moves to rank 35 at a 1-in-50 gate.
> Those numbers were computed under the retired useCount/UsesPerRank model
> and have NOT been recomputed against the level-keyed curve or the Phase D
> multipliers. The intent they encode still stands -- "an eye for the road
> picked up over a very long time", not a training strategy -- but do not
> quote the counts.
>
> Second-order effect, and deliberate: search also feeds hidden-creature
> detection on room entry (Perception + Search against Dex + Skullduggery)
> and foraging yields. So a well-travelled character slowly grows harder to
> sneak up on and slightly better at living off the land. That is the
> intended flavour of the change -- please do not "fix" it.
>
> A zero or negative `MovementSearchTrainChance` switches the feature off.

---

## Sight and gates: get, look, remove, equip, craft (slice 5a)

A mob now gets, looks, removes, equips and crafts by the same rules a player
does, through five shared bodies. `sight_gates_wrapper_guard_test.go` (repo
root) is an AST test over `internal/usercommands/{get,look,remove,equip,
craft,gearup}.go` and their `internal/mobcommands` twins: it parses each file
with comments stripped and fails if a wrapper itself evaluates
`ParticipantSight`/`CanSeeClearly`/`CanSeeShapes`, `IsCursed`/`Spellcasting`/
`IsActing`, or touches hand-pair internals (`GetHandPairs`, `HandsRequired`,
`ItemPtr`) directly instead of calling into the shared body below. A sixth
row pins `usercommands/equip.go`'s arm branch to `Character.WearInArm` rather
than reimplementing placement.

**`get.go`**: `ErrTooDark` (`:19`) refuses a pickup when
`messaging.ParticipantSight` is `SightNone`; shapes are enough to grope for
an item, so only total darkness gates it. `ErrExploding` (`:22`) stops a
sweep on an item tagged `exploding`. `TooDarkToGet(actor Actor) bool` (`:27`)
is the one statement of that sight rule, asked first by the player's `get` so
its container, corpse and bag branches stay refused too. `TakeFloorItem(actor
Actor, item items.Item, stash bool) error` (`:42`) is the shared body: dark,
then exploding, then `ErrHouseholdBauble` (never for a stash pull), then
`TransferItemToBackpack`. `GetItemFromFloor(actor, itemName, stash)
GetItemResult` (`:69`) asks `TooDarkToGet` BEFORE searching the floor, so a
blind actor learns nothing about what is there (`Found: false, Err:
ErrTooDark`) rather than a refusal naming an item it should not be able to
see. `GetGoldFromFloor(actor, amount) error` (`:83`) refuses the same way
before `FloorPickupGold`.

**`look.go`** (new file, slice 5a): `ResolveLook(actor Actor, lookAt string)
LookResolution` (`:49`) is the shared look body, in the player's exact order:
sight first, then no-target (the room), then a creature (resolved with the
looker itself as `ResolveTargetOptions.Viewer`, so nothing it does not
perceive can be named), then a sealed crate or a known room container
(`lookNamesAnObject`, `:116`), then an exit (direction alias resolved, then
through-sight, then lock). `LookKind` (`:12`) is `LookBlind`, `LookTooDark`,
`LookRoom`, `LookCreature`, `LookExit`, `LookExitTooDark`, `LookExitLocked`,
`LookOther`. Sight `SightNone` splits by cause (#364): `LookBlind` for a
looker whose `Perception` is `Blinded` (the same check `ParticipantSight`
answers `SightNone` on first), `LookTooDark` for one the room is too dark
for, so a caller can say that light would help.
`LookOther` is deliberately one bucket for "anything else": the crate and
container case defers to it rather than getting its own kind, so each
wrapper's own noun and item resolution (which differs between the player and
a mob) runs in its own order afterward, exactly as it does today.
`LookResolution.PetUserId` (`:40`) is set only at `NamesCreatures` (clear
sight) and is NOT a `LookKind`: the player resolves a pet AFTER carried items
and room nouns, so turning it into a kind would move the pet check ahead of
those and change look order. Each wrapper reads `PetUserId` at its own
existing pet step instead.

**`remove_equip.go`**: `CursedHolds(char, item) (holds, overridden bool)`
(`:127`) is the one statement of the remove-side curse rule: a cursed item
stays on a living wearer (`char.Health <= 0` lifts it, since a corpse holds
nothing back) unless `Spellcasting >= 4`, which overrides it silently
(`overridden` reports that for callers that want to say so). Equip's own
curse rule is separate and has no override — see
`characters.CursedRefusal` in `internal/characters/context.md`.
`RemoveEquipment(actor, itemName) RemoveEquipResult` (`:142`) gates Busy
(`IsActing`, checked before any lookup, so a busy actor with nothing worn
still gets the busy line) then not-found then, through the shared
`removeWorn(actor, matchItem) RemoveEquipResult` (`:155`), Cursed.
`RemoveEquipResult` (`:108`) carries `Busy`, `Cursed` (the curse holds, item
stays on) and `CursedOverridden` (it was cursed but Spellcasting lifted it)
alongside the existing `Removed`/`Err`. `RemoveAllEquipment(actor)
RemoveAllResult` (`:200`) is `remove all` for both actors: one Busy check,
then every worn item taken through `removeWorn` BY IDENTITY off
`char.Equipment.GetAllItems()`, not by name — so two same-named items, one
cursed, cannot make the loop re-test the cursed one in place of the other.
`RemoveAllResult` (`:190`) reports `Removed` and `Cursed` (left on) as
separate slices; a wrapper renders both, one line summarizing which pieces
would not come off.

`EquipItemInArm(actor, itemName string, arm int) EquipItemResult` (`:36`) is
`equip X armN` (spec ruling 11): the same `equipItem` body as `EquipItem`
(`:29`) but placement is forced through `Character.WearInArm` instead of
plain `Wear`, so the arm path gets every gate `Wear` has (strength, hands,
curse, reservation) instead of the old hand-rolled arm placement.
`EquipItemResult.ArmLabel` (`:20`) is set only by `EquipItemInArm`, from
`Character.ArmLabel(arm)`, and names the hand that took the item ("offhand",
"extra arm 1") for the wrapper's line.

**`craft.go`**: `TooDarkToCraft(actor Actor) bool` (`:129`) is
`!messaging.CanSeeClearly(...)`: crafting is fine work, so shapes by infrared
are not enough — it needs full, awake sight, unlike the `get`/`look` gates
above which pass at shapes. `InitiateCraft` checks it FIRST (`:148`), before
`IsCrafting`, so a dark room refuses immediately with no other gate's state
leaking through. `CraftResult.CannotSee` (`:51`) carries the refusal; every
other `CraftResult` field is unset on that path. The player's `usercommands.
Craft` asks the same gate before its storage pull and enchanting branch, so
neither can start work `InitiateCraft` would refuse.

---

## Speech and emotes (sight gates 5b)

Player speech and free-form emotes used to name the actor to every listener
in any light, while mob speech was two-tier with a lit-room shortcut that
named the speaker to a blinded listener, and a mob shout neither revealed the
shouter nor carried its words next door. Five shared bodies now own every
speech and emote room line for both a player and a mob; `speech_wrapper_guard_test.go`
(repo root) is an AST test over `internal/usercommands/{say,shout,rally,
warcry,emote}.go` and their `internal/mobcommands` twins: it fails if a
wrapper itself hides a name, judges sight, reveals, walks the neighbours or
wakes sleepers, and it also pins each wrapper's call count on its shared
body, one per room line the wrapper sends.

**`say.go`**: `Say(actor Actor, text string) SayResult` reveals a hidden
speaker, echoes "You hear someone talking." through the exits
(`Room.SendTextToExits`), fires `events.Communication`, then sends the room
line through the private `sendSpoken`. `FormatSayText` builds the line
itself, so both the player and the mob path share the one formatter
(`speech_wrapper_guard_test.go`'s `TestSayRoomLineHasOneFormatter` pins it).

**`shout.go`**: `Shout(actor Actor, text string) ShoutResult` reveals a
hidden shouter, sends the room line through `sendSpoken` on
`messaging.CategoryShout`, then walks every adjacent room
(`Room.ForEachAdjacentRoom`) with an anonymous line carrying the words: a
player's copy goes out through `otherRoom.SendTextCommunication` (still
deafen-filtered, byte-identical to before), a mob's through plain
`otherRoom.SendText` (authored, unfiltered). `wakeSleepers` wakes every other
sleeper in the shouter's own room; next door is out of scope.

**`room_lines.go`**: the three senders every speech and emote wrapper calls
into, never its own room line:

- `SendHeard(actor, cat, text)` is for authored text that is heard, not
  chatter (a rally, a warcry): every listener hears it whatever they can
  see, through `Room.SendTextHidingNames` with `messaging.HideNames`. Never
  deafen-filtered. A player actor excludes itself from its own line.
- `SendSeen(actor, cat, text, chatter)` is an emote's visual twin: the name
  at clear sight, "a figure" at shapes, nothing for a listener who cannot
  see. `chatter` is true only for a player's free-form line, which routes
  through `Room.SendVisualCommunicationHidingNames` to keep the deafen
  filter; every other line (empty, alias, and every mob emote) goes through
  `Room.SendTextVisualHidingNames` unfiltered (owner ruling 6).
- `sendSpoken(actor, room, cat, line, stillHidden)` (unexported) is what
  `Say` and `Shout` share: every listener hears the words, the speaker's
  name hidden per listener. A player's line goes through
  `Room.SendCommunicationHidingNames` (deafen-filtered); a mob's through
  `Room.SendTextHidingNames` with `messaging.HideSpeakerNames` (unfiltered).
  A speaker still hidden after the reveal attempt is rewritten to
  `messaging.SightNone` for everyone first (`HideSpeakerNames`), so a speaker
  who somehow stayed hidden is unseen by all.

**The deafen split (owner rulings 3, 6 and 7)**: a player's own words are
chatter and the Deafened moderation flag still applies to them
(`events.Message.HiddenFromDeafened`, read by `internal/hooks`); an NPC's
lines are authored content and are never deafen-filtered, whatever the
wrapper's own `chatter` argument says for a mob caller. Rally, warcry and an
authored emote are heard/seen by sight rules only, with no deafen check at
all, because `SendHeard`/`SendTextHidingNames`/`SendTextVisualHidingNames`
never consult the flag.

**`merchantSay` now only calls `Say`.** The unexported `merchantSay` in
`internal/actions/sell.go` and the package-local `merchantSay` in
`internal/usercommands/offer.go` (used by both `offer` and `appraise`) each
call `actions.Say(&actions.MobActor{...}, line)` and nothing else; neither
hand-rolls a room line any more. `internal/hooks/justice_wiring.go`'s guard
`say` callback is the same shape: it only calls `actions.Say`.

The `Social` files row (`say.go`, `emote.go`, `emote_aliases.go`) gains
`shout.go` and `room_lines.go`.

---

## Naming and aiming in the dark (follow-up slice A)

- **`ResolveTargetOptions.Viewer`**: every player command that names a creature
  passes the player; a creature they do not perceive cannot be named. Staff
  tools and mob callers pass none. `lookup_viewer_guard_test.go` in the repo
  root registers every lookup as viewer or plain, with the reason.
- **`FindAttackTarget(rest, room, userId, mobId, viewer)`**: the named branch
  and the `*`, `*mob`, `*user` pools skip what `viewer` does not perceive.
- **`InitiateCast`** runs `admitCastAim` (`cast_admission.go`) for a player's
  single-target casts of either kind and harmful multi casts: clear sight
  allows names, foes and shapes; shapes only allows the caster's own foe and
  `shape` / `N.shape` / `shape#N` (figures are perceived players then mobs, in
  room order); no sight refuses. Refusals are narrated, set
  `RefusalExplained`, and spend nothing.
- **`HelpCharmAlly(m, sideUserId)`**: the one rule for which charmed mobs a
  helpful spell from `sideUserId`'s side may land on: charmed by that player
  or by a member of that player's party. `InitiateCast`'s single-target help
  (through `helpSingleMobAllowed`) and `hooks.spellHelpAreaTargets` both call
  it. A mob caster's single-target help follows the same sides: a charmed
  mob helps any player and its owner's side; an uncharmed mob helps itself
  and mobs charmed by no one (its packmates, a boss add's named boss), never
  a player or a pet. A refused help target is plain `NoTarget`, so a player
  hears `You don't see "x" here.`
- **`SendCounterTrio(room, res, countered, counteredUserId)`**: the one counter
  dispatch, used by `DispatchCounterMessages` and `hooks.fireSpellCounterTier`.
  It goes through `messaging.SendTrio`, so a counter in the dark names nobody.
- **`FireCounterTaunt(room, shape, counterer, countered, countererRecipient,
  countererId, counteredRecipient, counteredId)`**: the defy answer's shared
  dispatch, used by taunt's `counterTauntExit` and `hooks.fireSpellCounterTier`
  (a defied charm). Refuses a `shape` whose targeting is not single, the same
  gate the swing primitive carries. Dispatches directly via
  `SendText`/`SendTextVisual`, not `messaging.SendTrio`; moving the retort onto
  the darkness seam is M4d's.
- **`RetargetNotice(room, userId, target)`** (`retarget_notice.go`): builds
  "You turn your attention to X!" with X hidden by the reader's
  `ParticipantSight`; ok=false when target no longer resolves. Shared by
  `hooks`'s two round-driver retarget sites and `mobcommands`'s
  mob-departure retarget, since neither package is importable from the
  other but both import `actions`.

## Skill Actions

### Consider

**Function:** `Consider(actor, target) ConsiderResult`

Computes a power-ratio assessment of `target` from `actor`'s perspective.
- Returns `ConsiderResult` with `Ratio` (self power / target power), both
  absolute power values, and target name/type.
- **Progression:** Triggers `actor.OnStatUse("perception")`.
- **Messaging:** Sends a colored difficulty string to the actor (e.g.,
  "an easy opponent"). Mobs receive no feedback (SendText is a no-op).

---

## Skullduggery Actions (chunk 2.7)

### Sneak

**Function:** `Sneak(actor) SneakResult`

Attempts to transition the actor from Visible through Concealing to Hidden
after an opposed roll against every eligible observer in the room. A player
actor excludes themself and its `alliesOf` side (party members and its own
charmed mobs and companions, so your own pet never notices you); a mob excludes
itself.

- **Readiness and admission:** Already-Hidden, combat, activity, awareness,
  and room checks are read-only and run before cost admission. A valid attempt
  commits `ActionSneak` against Stamina using `SneakBaseStaminaCost`,
  Skullduggery's inverse-skill multiplier, physical encumbrance, and
  full-or-refuse policy. Only a paid admission may call
  `TransitionToConcealing` or roll observers.
- **Structured result:** `SneakResult.Cost` is the
  `characters.CostCommitResult` from admission. `CostRefused` means no
  awareness, cooldown, round, contest, or progression mutation occurred.
  `AlreadyHidden` and `InCombat` are pre-admission outcomes and therefore have
  a zero-value cost result.
- **Roll:** The sneaker uses effective Dexterity plus the Skullduggery skill
  multiplier and stealth bonuses, modified by light conditions per observer.
  Each observer uses effective Perception plus the Search skill multiplier.
  Resolution flows through `combat.RunContest`.
- **Success/failure:** Success resolves Concealing to Hidden, queues the Hidden
  condition mirror, sets the `sneaking` misc key, and returns `Success`. The first
  observer who wins resolves the actor back to Visible and populates
  `SpottedByName`. `RollHappened` distinguishes a contested attempt from an
  empty-room success.
- **Player wrapper ownership:** The user command owns the skill gate, busy and
  prior-failure-cooldown messages, stamina-refusal text, and player-facing
  success/failure text. It checks the prior failure cooldown with read-only
  `CooldownReady`; only a spotted paid attempt may apply
  `SneakFailCooldown`, and an absent/zero value remains disabled. Player
  Skullduggery progression runs only when `RollHappened` is true and only after
  paid resolution.
- **Mob wrapper ownership:** The mob command renders no refusal text and has no
  player failure cooldown. It returns immediately on `CostRefused`; only a
  successful paid attempt calls `OnSkillUse("skullduggery", 0)`.
- **`MobIsSneaking(mob *mobs.Mob) bool`** (`sneak.go`) is the one derivation of
  a mob's sneaking state: `IsHidden()` (the Awareness-backed condition), or
  the `sneaking` misc-data flag set while not yet hidden (`Sneak` sets it
  synchronously, ahead of the hidden condition's event landing). Walking
  (`mobcommands.Go`, both its ordinary exit step and its forced
  `go <roomId>` path used by `callforhelp`) and a successful flee
  (`hooks.handleMobFlee`) all call this rather than re-deriving it, so they
  cannot drift the way flee once did by checking only `IsHidden()`.

### Steal

**Function:** `Steal(actor, opts) StealResult`

Pickpockets a target mob or player, or robs an item from a room container.

**Three paths:**

1. **Mob pickpocket** (`opts.TargetMobInstanceId` set; `stealFromMob`):
   - Refused for any companion (charmed, the thief's own included, or bonded to
     the AI companion: "X is someone's companion. You can't steal from them.",
     reason `companion`), then for non-combatant or player-attack-immune mobs
     (`mobs.CheckPlayerHarm`), and below skullduggery rank 2.
   - One contest (`combat.RunContest`): the thief's Dexterity +
     skullduggery x SkillWeight (+ StealHiddenBonus when hidden) against
     `stealVictimScore` (the mob's Perception + skullduggery x
     SkillWeight). Skullduggery is awarded won or lost: for a mob thief on
     the roll, for a player at the reveal (`resolve`), since a skill-up line
     at the roll would give the outcome away, and a chance lost trains
     nothing. A player already in a pickpocket's pause starts no other
     (`pocketPending`: "Your hand is still in someone's pocket.").
   - A MOB thief's outcome is at once. A PLAYER's is held back for a pause
     (`steal_pocket.go`, `startPocketAttempt`): "You attempt to pick X's
     pocket...", then after `PocketDelay` (StealPocketSeconds at Dexterity 100,
     scaled by 100/Dexterity, kept between StealPocketMinSeconds and
     StealPocketMaxSeconds) the outcome is revealed under the mud lock
     (`resolve`). `StealResult.Pending` is set meanwhile. A FAILED roll is
     caught however the pause ends (owner ruling 2026-09-29; `caught`): with
     the thief and the mark both still in the theft room, `caughtByMob` as
     below; otherwise, or offline, the mark cries thief in its own room and
     `theftCrime` records the crime against the thief in the theft room, away
     mode (through the `pocketCrime` seam). The mark reveals and attacks
     only when it and the thief have both left the theft room and are
     together again (`markAttacksThief`); the bystanders there witness
     nothing. An online thief is told and trained on the loss. A mark that
     is gone or dead catches nobody. A SUCCESSFUL roll whose thief has left the room, gone offline, started
     fighting or come under attack by then, or whose mark has moved, died or
     gone, loses the chance ("You lose your chance at X's pocket."): nothing
     taken, nothing trained.
   - Success (`takeFromMob`): 75 to 100% of the mob's gold, one random item,
     and, for a player, a bauble: the one the mark carries
     (`carriedBauble`), or, when it carries none,
     BaublePickpocketChancePct (50) of the time a new one, its tier from
     the richer pickpocket weights (`baubles.PickPocketTier`), named during the
     pause (`baubles.Generate` at the attempt, `SourcePickpocket`, `Victim`
     the mark's authored name, never for anyone's companion or former
     companion, whose name a player may have chosen: `pocketBaubleAllowed`,
     `EverCharmed`). A naming not back when the pause ends is waited
     for up to BaublePickpocketGraceSecs, then given up (from the
     fallback corpus). It is minted pocket-sized (`baubles.MaxWeightFor`),
     marked
     stolen (`markPocketStolen`; so is a bauble the random item happens to
     be, unless a player gave it to this mob: `baubles.Record.GivenTo`), and named in the one success line with the rest. A bauble
     named for a lost chance goes into the mark's pocket (`intoPocket`),
     for the next attempt.
   - Failure (`caughtByMob`): "X catches you in the act!", the room sees it,
     then `thiefCaught` (revealed, then `theftCrime` in the act: a sleeper
     wakes, the crime with every faction witness in the room; then the attack,
     `markAttacksThief`, which spares a thief who is dead or downed).
     Away, `theftCrime` runs in the theft's room with the mark its only witness
     (`theftWitnesses`), judged by `pocketAttempt.markSaw`, its sight of the
     theft room taken at the attempt (`markSight`, judged as
     `crimes.WitnessesInRoom` judges a witness), so a thief who carried the
     only light off before the reveal is still named, and a mark asleep at
     the attempt saw nothing and names nobody (owner ruling 2026-09-29); no
     external witness. A mark that saw clearly learns who robbed it
     (`knowledge.RecordCrimeWitnessed`, which sets `HasMet`); only the
     last-seen room and round are skipped (`knowledge.RecordMet` is not
     called). Bystanders learn nothing.
   - Every pause is tracked (`pendingPockets`); copyover and shutdown call
     `FlushPocketAttempts` under the lock before saving, which reveals each
     at once. Tests resolve in line (`runPocketAttempt`, `pocketThief`,
     `pocketBaubleRoll` are variables; `bauble_testinit_test.go`).

2. **Player pickpocket** (`opts.TargetUserId` set):
   - Rolls `actor Dexterity + Skullduggery` vs `player Perception +
     Skullduggery` (note: Perception, not Dex).
   - If win: picks a random item from player inventory.
   - **Detection roll (extra):** If the steal succeeds, rolls `actor
     Dexterity + Skullduggery` vs `player Perception + Skullduggery` again
     to determine if the player **notices**. If player notices, they receive
     "You notice someone trying to pickpocket you!" message. Theft still
     succeeds either way.
   - **Messaging:** Actor always gets silent feedback. Player gets the
     detection message only if the second roll fails.

3. **Container rob** (`opts.RoomContainerId` set):
   - No opposed roll. Opens the container and removes an item.
   - Always succeeds if the container has items.
   - Returns item ID.

**Progression:** Triggers `actor.OnStatUse("dexterity")` and
`actor.OnSkillUse("skullduggery")`.

**Cooldown:** One key, `skullduggery:steal` (`skullduggeryCooldownKey`),
shared with Plant, lasting `StealCooldown` real seconds (60 shipped, 15
rounds at `RoundSeconds: 4`). Steal only CHECKS it (`CooldownReady`); it is
armed by the attempt itself. `newTheftAttempt` builds the thief's side once
(`theftAttempt`: rank, score, cooldown period) and each path reads the score
through `theftAttempt.score()` immediately before its contest, which arms the
cooldown as it hands the score over. So every refusal (no target, target
gone, a companion, an immune mob, rank below 2, an empty container, too
heavy to lift) leaves the cooldown unarmed, and no contest runs without
arming it. Before 2026-09-30 it was armed ahead of every refusal, so a fresh
character (rank 1) was refused on the first try and then locked out.

**Result struct:** `StealResult` (`Succeeded`, `Detected`, `StoleGold`,
`StoleItemId`, `StoleItemName`, `DefenderName`, `OnCooldown`, `Reason`,
`Pending`); read `steal.go` for the field comments.

### Plant

**Function:** `Plant(actor, opts) PlantResult`

Slips an item from the actor's backpack onto a target mob or into a room
container.

**Two paths:**

1. **Plant on mob** (`opts.TargetMobId` set):
   - Rolls `actor Dexterity + Skullduggery` vs `mob Dexterity +
     Skullduggery`.
   - If win: removes item from actor backpack, adds to mob inventory.
   - If fail: item stays with actor, plant fails.
   - **Messaging:** Actor gets success/failure feedback. Mob (if aware) may
     receive a discovery message on next interaction (not immediate).

2. **Plant in container** (`opts.RoomContainerId` set):
   - No opposed roll. Removes item from actor backpack, adds to container
     inventory.
   - Always succeeds if the container exists.

**Item lookup:** `opts.ItemTag` is a space-separated noun (e.g., "copper
coin"). The function searches actor backpack for a matching item by display
name / simple name.

**Progression:** Triggers `actor.OnStatUse("dexterity")` and
`actor.OnSkillUse("skullduggery")`.

**Cooldown:** Steal's (`skullduggery:steal`, `StealCooldown` real seconds),
checked up front and armed only by the attempt, through the same
`theftAttempt.score()`: a missing target or container spends nothing. An
unwatched container plant is an uncontested attempt and does arm it.

**Result struct:** `PlantResult` (`Succeeded`, `Detected`, `PlantedItemId`,
`DefenderName`, `OnCooldown`, `Reason`); read `plant.go` for the field
comments.

### Defuse

**Function:** `Defuse(actor, opts) DefuseResult`

Disarms a trap on a room container or exit. Optionally consumes a disarm kit
from the actor's backpack if `opts.UseKit` is true.

**Two paths:**

1. **Container trap** (`opts.ContainerId` set):
   - Finds the container, rolls opposed check: `actor Dexterity +
     Skullduggery` vs `container.TrapDifficulty`.
   - If win: removes the trap (sets `TrapId = 0`). Container is now safe.
   - If fail: actor takes damage (trap detonates). `actor.ApplyHealthChange(-damage)`.

2. **Exit trap** (`opts.ExitName` and `opts.Direction` set):
   - Finds the exit, rolls opposed check: same formula as containers.
   - On success: trap removed.
   - On fail: actor takes damage.

**Disarm kit consumption:** If `opts.UseKit` is true, the function searches
the actor's backpack for an item tagged "disarm kit" and consumes it on
success only. If no kit found, the action proceeds without it.

**Progression:** Triggers `actor.OnStatUse("dexterity")` and
`actor.OnSkillUse("skullduggery")`.

**Cooldown:** No cooldown (can defuse multiple traps per turn).

**Result struct:**
```go
type DefuseResult struct {
	Success          bool
	Message          string
	TrapDetonated    bool  // true if failed and trap triggered
	DamageDealt      int
}
```

### Scan

**Function:** `Scan(actor, opts) ScanResult`

Sweeps adjacent rooms for visible entities (non-hidden mobs/players).

**Mechanics:**
- **Adjacent rooms:** Scans in all four cardinal directions; lists any
  non-hidden mobs and players in the returned room descriptions.
- **Visibility:** Does not bypass hidden state — only visible entities are
  reported. Mobs/players who are hidden (condition 9) are not seen.
- **UserActor behavior:** Renders a "You sense:" list of adjacent-room
  entities with flavor text.
- **MobActor behavior:** Silent (no feedback).
- **Hostile-only mode:** `opts.HostileOnly = true` filters results to only
  entities the actor hates.

**Messaging:** UserActor receives "You sense: [adjacent rooms with entities]."
MobActor silent.

**Progression:** No stat/skill use triggered.

**Cooldown:** No cooldown.

**Result struct:**
```go
type ScanResult struct {
	Success        bool
	SightingFound  bool  // true if at least one entity seen
	Message        string
}
```

### Search

**Function:** `Search(actor, opts) SearchResult`

Three-tier discovery system: exits, stashed/hidden objects, and nouns.

**Mechanics:**
- **Tier 1 (Exits):** Lists all room exits (always succeeds for UserActor).
- **Tier 2 (Stashed items):** Searches containers and ground for hidden items.
- **Tier 3 (Hidden entities):** Detects hidden mobs/players in the room.
  Promoted to `ctx.SoftTarget` if hostile.
- **Ignores non-hostile Tier-3 hits:** If a hidden entity is not hostile,
  it is not set as SoftTarget.
- **UserActor behavior:** Renders progressive search feedback (what tiers
  were discovered, what was found).
- **MobActor behavior:** Silent; just seeds SoftTarget if hostile found.

**Messaging:** UserActor receives discovery feedback per tier. MobActor silent.

**Ending hiding:** a player's find ends each found creature's hiding for
everyone (`revealSpotted`), the way a spotted occupant's hiding ends on room
entry in `usercommands/go.go`. A player hider is told, by name only if they can
see the searcher. A mob's find ends nothing (slice F).

**Progression:** No stat/skill use triggered.

**Cooldown:** Shares the `search` key (configurable duration, typically
  2 rounds).

**Baubles (`search_bauble.go`, docs/baubles Phases 3, 4 and 5b).** After every
contested tier, a PLAYER's search takes a bauble roll (`baubles.RollFind`): a
chance set by the room's biome (`BaubleBiomeChancePct`: buildings 5%, streets
2 to 2.5%, wilderness 0.25%) and raised by the searcher's search skill
(`BaubleSkillFactor`, up to `BaubleSkillMaxBonus`), rationed to
`BaubleRollsPerWindow` rolls per room per `BaubleWindowMinutes` of real time.
Both bauble rolls pass `SightPenalty: 1 - messaging.SightMult(char, room)`; the
root sight guard watches `baubles.RollFind` and `actions.searchBaubleRoll`. A
find is NOT handed over on the spot: the player is told they are working
something loose, `SearchResult.
BaubleFound` is set, and `StartBaubleFind` hands a `BaubleDelivery` to a
goroutine. That goroutine names it with `baubles.Generate` (the model, or the
fallback corpus) WITHOUT the mud lock, waits out the rest of
`BaubleRevealSeconds`, then takes `util.LockMud()` once to `Mint` and deliver
(`deliver`): if the room it was found in is a household NOW (`HouseholdResident`:
indoors, a resident about), or the find was rolled as a household's
(`BaubleDelivery.Household`, asked at the search by `householdFind`, which
gives it the richer household tier weights through
`baubles.FindOpts.Household`; it stays the household's even if they have
stepped out by the delivery, so a richer find always has to be stolen), it
stays there, on the feature searched
(`BaubleDelivery.Spot`, "on the bookshelf"), owned by the household
(`items.Item.LeaveBaubleAt`, `baubles.MarkHousehold`); otherwise into the pack,
at the finder's feet if they cannot carry it, or onto the floor where it was
found if they logged off meanwhile. Anything left lying carries its spot and
the time (`baubleNow`) and vanishes after `BaubleUntakenHours` (rooms'
untaken sweep). Where it goes is settled BEFORE it is minted: a finder
offline, from a room that can no longer be loaded, gets nothing minted, so
no catalog record exists for a find that is nowhere.

Every delivery is tracked from start to finish (`pendingFind`,
`PendingBaubleDeliveries`): `startBaubleDelivery` tracks it before its
goroutine exists, so a copyover in the same pass of the game loop still
finds it. `FlushBaubleDeliveries`, called under the mud
lock by `triggerCopyover` (copyover.go) and the shutdown path (world.go)
before rooms and players are saved, finishes each one still on its way:
named if its naming came back, otherwise the corpus fallback it would have
been (`baubles.FallbackFor`). It cancels the naming, and the goroutine,
which needs the lock the flush holds, finds it delivered (`claim`) and
stands down. So a player told
"Something glints..." never loses the find to a copyover. Rules:

- Mobs never roll. Instance/ephemeral rooms, banks, storage and character
  rooms never roll (`baubleRoomAllowed`); excluded zones are the baubles
  package's check.
- The roll never sets `rolledAgainstSomething`, so offering a roll does not
  make a room a progression candidate, and a roll that finds nothing awards
  nothing.
- A FIND trains search (owner ruling, Phase 5b): `awardSearch` pays the one
  award as a win when `BaubleFound` is set, even in a room with no contest,
  and a find alongside lost contests makes the award a win. `BaubleFound` is
  in `FoundAnything()` (no "You find nothing of interest." after a find) but
  not in `foundByContest()`, which only reports the contested tiers.
- In this package's tests the default roll never finds
  (`bauble_testinit_test.go`); bauble tests stub it.
- A spent window, an excluded room and a failed roll are silent and identical.
- The minimum wait applies to corpus and generic finds too, so the timing
  never tells a player whether the model named their find.
- `BaubleRequest` copies AUTHORED room text only (title, description, noun
  keys), never signs or anything a player typed.
- `searchBaubleRoll`, `startBaubleDelivery`, `findBaubleRecipient` and
  `findBaubleRoom` are variables so tests run delivery in line.
- `BaublePlace(room)` is the room as the catalog records it. `StartBaubleFind`
  is also what the admin `bauble spawn` command uses. `bauble_admin.go` holds
  `BaubleRequestForRecord` (the request that would name a record now, for
  `bauble prompt` and `bauble regen`) and `RegenerateBauble` (model call off
  the lock, `baubles.ApplyRegenerated` under it, the admin told either way;
  `runBaubleJob` and `tellBaubleAdmin` are variables for tests).
  `BaubleRequestForRecord` omits a `PlayerKey` record's name. The find
  lines (`BaubleDelivery.deliver`) and the pickpocket success line
  (`takeFromMob`) name a bauble as their one reader sees it
  (`items.Item.DisplayNameFor`), so a finder reads their own finder-only
  bauble; everywhere else it is "Trinket".

**Targeted search (`search_feature.go`).** `SearchOptions.Feature` is what a
player typed after `search`. Empty searches the room as above. Otherwise,
BEFORE the cooldown, `FindSearchFeature` resolves it (leading articles and prepositions dropped, so
`search under the table` works) to a room noun (`room.FindNoun`, aliases and
plurals included), then a hidden noun THIS character has discovered, then a
container they can see. No match sets `SearchResult.FeatureNotFound` and the
search runs as a plain room search, as `search <anything>` always did. A
searcher whose sight here is `messaging.SightNone` (lighting plan 5c, the
test `look` refuses on) names no feature at all: what they typed is treated
exactly as a miss, so the reply cannot confirm the feature exists. At shapes
a feature is still named, as `look <noun>` still describes one. A match is a FULL
room search ("You search the X and snoop around for a bit..."): every
contested tier runs exactly as for a plain `search`, so a quest that expects
`search shelf` (or any room noun) to turn up its hidden item still works. Only
the bauble roll differs: the feature's own roll (`searchFeatureForBauble`),
never from the room's window. The hourly limit is for BAUBLES only: a feature
whose bauble roll was taken in the last `BaubleFeatureWindowMinutes` (60,
`baubles.FeatureSearchable`, claimed by `baubles.ClaimFeatureSearch`) is still
searched in full, and the room's ordinary roll is offered instead
(`SearchResult.FeatureSearched` records that). Nothing is refused, so the
limit never locks a player out of a quest item. On a find the request names the feature
(`GenRequest.Container`), carries its authored description
(`ContainerDescription`), and `SearchFeature.Spot()` ("on the bookshelf",
"beside the chest" for a container) is where it lies if left in the room.
Rules:

- Anti-oracle: an undiscovered hidden noun or container never matches, so it
  gets the same reply as a word that means nothing; a miss and an excluded
  room both read "You find nothing of interest.". That a feature was searched
  in the last hour is never said: the reply is the same either way.
- Progression: as for a room search (`awardSearch`).
- Players only; a mob's `Feature` is ignored and it searches the room.
- `SearchResult.Feature` is the room's canonical name for what was searched.
  `SearchFeature.WindowName()` (lower case) is the window key, so a noun and
  a container of the same name share one window, and an alias shares its
  noun's. `RoomSearchFeatures(room)` lists every feature, hidden ones
  included, for the admin `bauble window` view.

**Stolen baubles after the theft (`stolen_bauble.go`, docs/baubles Phase 6c).**
Selling is `sell_bauble.go`'s (above); storage (`usercommands/storage.go`,
`storageRefusesStolen`) and the auction house (`modules/auctions`,
`auctionRefusesStolen`) refuse a hot bauble. This file is the owner's side:

- `RecognizeStolenBaubles(roomId, userId, mobInstanceId)`, called by the
  `RoomChange` listener (`hooks/RoomChange_StolenBaubleRecognition.go`): a
  player's move looks at that player's hot baubles; a mob's move lets that
  mob look. `isBaubleOwner`: the mob template it was lifted from
  (`StolenFromMob`; so any instance of that template), or for a household's
  bauble taken unwatched a `householdMember` whose `HomeRoomId` is that
  house, at home. The owner must be awake, alive and nobody's companion
  (`canRecognize`) and still in the room, and must make out shapes
  (`messaging.CanSeeShapes`: not blind, not in the dark); seeing only
  shapes, it recognises the bauble but names nobody ("a figure", the
  witnessing tiers). Only the thief
  (`StolenByUserId`) is ever accused, so nobody can be framed and the
  recognition cannot be spent on a friend.
  `stolenRecognitionRoll` pits `stealVictimScore` (the owner's own sight
  ramp included) against `carrierScore` (Dexterity + skullduggery ×
  SkillWeight + the hidden bonus). Recognised: `MarkRecognized`, the owner
  says so, then `thiefCaught` (the `stolenCaught` seam): the crime, its
  reputation and bounty, the attack. Once per theft
  (`Record.RecognizedSinceTheft`).
- `StolenBaubleGiven(giver, m, itm)`, from `usercommands/give.go` after a
  transfer to a mob: a stolen bauble given to its owner is a return and
  cools (`MarkReturned`). Given by its thief, each of the owner's factions
  (`ownerFactions`) credits `returnShare`: the catch
  (`-CrimeRepDeltaTheft`) split into `BaubleReturnsPerCatch` parts with the
  remainder spread so that every that-many returns total exactly one catch
  (5 in thirds pays 1, 2, 2). Only reputation actually lost is earned back:
  a faction credits no more than `BaubleReturnsPerCatch` returns per open
  theft crime it holds naming the thief as the identified perpetrator
  (`identifiedTheftCatches`, the crimes log, unresolved only: a sentence
  served resolves them and restores reputation, and stale ones are
  forgotten), so a thief never caught gains nothing and stealing to return
  cannot farm reputation. Only returns credited since the oldest open catch
  count against that cap (`ReturnCredits(..., since)`, rounds compared), so
  returns made before a catch are not charged to it. A bauble earns credit
  once ever. Any other bauble a player gives a mob is marked a gift
  (`baubles.MarkGiven`).
- Seams: `stolenNow`, `recognitionRoll`, `stolenCarriers`, `stolenCaught`,
  `returnRepBump`, `ownerFactions`, `theftCatches`.

The owner's "points at" line and the household "caught trying to pocket" line
(`steal.go`) go through `SendTextVisualHidingNames` with the player's name.

**Household baubles (`household_bauble.go`).** A find in a household is left
in the room and belongs to it (`items.Item.BaubleHousehold` = the room id).
`isResident`: a person (the player species, a shopkeeper or a faction
member), awake, able to see, not `AutoAggro`, not anyone's companion.
`householdResidents` requires the room's biome to be `Indoor`.

Taking one is `steal`, never `get` (`usercommands/get.go` refuses, and `get
all` skips it, so a pickup can never start a crime by accident):
`Steal(actor, StealOptions{HouseholdItem: itm})` runs the ordinary steal
checks (skullduggery rank 2, the steal cooldown, the attacker score) and then
`stealHouseholdBauble`:

- Not this room's household's: refused.
- Too heavy: refused ("overloaded"), no crime.
- `stealObserverPass`, the container theft's observer contest (shared with
  `stealFromContainer`, and the combat contest-site allowlist's entry for
  both): the most watchful player or mob in the room (`stealVictimScore`,
  the thief's party excluded) contests the thief's score; no one there, no
  contest.
  Skullduggery is awarded, won or lost.
- Unseen: taken; marked stolen (`baubles.MarkStolen`).
- Spotted by one of the household: `householdCaught` = `thiefCaught`
  (steal.go, shared with `stealFromMob`): revealed, a sleeper wakes, a crime
  against the resident's factions (rep, bounty, witnesses' knowledge), and it
  attacks unless it is a non-combatant or player-attack-immune. The bauble
  stays.
- Spotted by someone else: revealed, no crime against the household.

`findHouseholdResidents`, `householdCaught`, `householdMember` and
`baubleNow` are variables for tests.

**Selling (`sell.go`, `sell_bauble.go`).** Both record the sale's progression
through one helper, `saleProgression(seller, mob)`, the single seam the
progression guard allows for a sale.

**Result struct:**
```go
type SearchResult struct {
	Success         bool
	HiddenHostile   bool  // true if hidden entity promoted to SoftTarget
	Message         string
}
```

---

## Sleep Action (chunk 3.3)

### Sleep

**Function:** `Sleep(actor, opts) SleepResult`

Applies condition 15 (Sleeping) to the actor. Combat-gated: fails if the
actor is currently in combat (`Aggro != nil`). Idempotent: if the
actor already has the Sleeping condition, returns `SleepResult.AlreadyAsleep
= true` with no additional condition applied.

- **Success:** Actor receives condition 15; returns `SleepResult.Success = true`.
- **Failure (combat):** Returns `Success = false` with messaging.
- **Messaging:** UserActor receives a "You lie down and close your eyes."
  message; the room sees "<Actor> lies down to sleep." MobActor messaging
  goes to the room only.
- **Progression:** No stat/skill progression triggered.
- **Cooldown:** None.

Entry points that call `Sleep`:
- `usercommands/sleep.go` — player `sleep` command
- `mobcommands/sleep.go` — mob `sleep` command
- `hooks/NewRound_IdleMobs_schedule.go` — schedule executor for
  `activity: sleeping` segments

**Result struct:**
```go
type SleepResult struct {
    Success       bool
    AlreadyAsleep bool
    Message       string
}
```

---

## Foraging & Salvage Actions (chunk 2.9)

### Forage

**Function:** `Forage(actor, opts) ForageResult`

Single forage attempt in the current room's biome. Gated by biome availability
and shared cooldown.

**Mechanics:**
- **Biome check:** Actor must have forager profile data for the room's biome.
  Returns Failure if biome not found in profile (wrong terrain type).
- **Cooldown:** Shares the `forage` key (6 rounds, config:
  `ForageActionCooldown`).
- **Item discovery:** On success, generates item based on biome's forage table.
  Returns `ForageResult.ItemId` and `ItemName`. On miss (roll failure) or
  cooldown, returns Failure.
- **Progression:** Triggers `actor.OnSkillUse("foraging")`.

**Result struct:**
```go
type ForageResult struct {
	Success   bool
	ItemId    int
	ItemName  string
	Message   string
}
```

### Salvage

**Function:** `Salvage(actor, opts) SalvageResult`

Single-tick salvage attempt. Modes: default targets first eligible corpse
(mob death items in room), optional `ItemUuid` targets a specific item.

**Mechanics:**
- **Corpse mode** (`opts.ItemUuid` empty): Scans room for corpse items
  (items with `on_corpse: true`). Salvages the first match. Returns Failure
  if no corpse items found.
- **Item mode** (`opts.ItemUuid` set): Salvages the specified item UUID
  directly (player per-tick invocation path).
- **Multi-round activity:** Salvage takes 1-5 rounds depending on ingredient
  gold value. Each ingredient is rolled independently per the skill-based
  recovery table.
- **Progression:** Triggers `actor.OnStatUse("perception")` and
  `actor.OnSkillUse("salvage")`.
- **Result:** Returns `SalvageResult` with success flag and recovered
  materials list (empty if roll failures on all ingredients).

**Result struct:**
```go
type SalvageResult struct {
	Success     bool
	Message     string
	RecoveredCount int  // number of materials recovered
}
```

### Shadow

**Function:** `Shadow(actor, opts) ShadowResult`

Follow a quarry while hidden. Player and mob actors, player and mob quarries,
all by the same rules (player/mob parity slice 6).

- **Gates:** the actor must be hidden (`Character.IsHidden`), not in combat,
  name a target, and be off the shadow cooldown (`Balance.ShadowCooldown`
  rounds; the Go default is 0, which a cooldown rounds to one round, and
  `config.yaml` ships 5).
- **Start:** stores the quarry (read back with `ShadowTargetOf`), applies
  `ShadowingConditionId` (condition 87, 25 rounds), sends the start line, and
  runs `ShadowSenseRoll`: `Detected` reports it; the shadow starts either way.
- **Following:** `hooks.RoomChangeShadowFollow` moves the shadower after its
  quarry on any named-exit move and runs the arrival check. This package owns
  no follow logic.
- **`ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool`** is the
  one shadow contest: the target's `CalcDetectionScore` (as attacker) against
  the shadower's `CalcSneakScoreVsObserver` times its `SightMult`, through
  `combat.RunContest`, in `room`'s light. It awards the shadower's
  Skullduggery on both outcomes (a win when unsensed), sends a player target
  "You sense someone following close behind you." when it senses the
  shadower, shows a mob target nothing, reveals no one, and returns whether
  the target sensed. Run at the start of a shadow and on each arrival.
- **`ShadowTargetOf(c) (userId, mobInstanceId int)`** reads the quarry. The
  two misc-data keys behind it are unexported constants, and nothing outside
  this package names them (`shadow_follow_guard_test.go`).
- **`ClearShadow(c)`** drops the quarry and condition 87 with no cooldown and
  no line: the stale-state guard and the death and logoff cleanups.
- **`EndShadow(actor, reason)`** is `ClearShadow` plus the cooldown plus
  `reason` to the actor: `shadow stop` and the spotted end.
- `RemoveCondition` only expires condition 87; it is pruned, and a player
  reads its end line, at the next turn.

**Result struct:**
```go
type ShadowResult struct {
	Succeeded  bool   // target id was stored and shadow tracking began
	Detected   bool   // target won the initial sense roll
	TargetName string // display name of the target
	OnCooldown bool   // attempt was blocked by shadow cooldown
	Reason     string // when Succeeded==false and !OnCooldown, why
}
```

### Track

**Function:** `Track(actor, opts) TrackResult`

Trail-read (passive sniffing) or active tracking on a resolved target.

**Mechanics:**
- **Trail-sniff (no-arg):** `opts.TargetNoun` empty; reads the room's trail
  data (bloodstain, scent, tracks left by previous passage). Returns success
  if a trail exists; failure if none.
- **Active track (target noun):** `opts.TargetNoun` or target from Event/Aggro;
  enters tracking mode on the target. On success (adjacent trail/recent
  sighting), applies condition 86 (Track status) and misc-data pair
  (`tracking-<userId>` or `tracking-<mobId>` with arrival timestamp).
  Seeds `ctx.SoftTarget` for downstream scout actions.
- **UserActor behavior:** Renders trail-sniff results or tracking status.
- **MobActor behavior:** Silent; just applies condition/misc-data and seeds
  SoftTarget.

**Messaging:** UserActor receives trail feedback or tracking status. MobActor
silent.

**Progression:** No stat/skill use triggered (scout actions planned as
  skill-less in Phase 1).

**Cooldown:** Shares the `search` key (typically 2 rounds).

**Result struct:**
```go
type TrackResult struct {
	Success        bool
	TrailFound     bool  // true if trail exists (sniff) or track active (active)
	TargetName     string
	Message        string
}
```

---

## Sell Action (chunk 5.4)

### Sell

**Function:** `Sell(seller Actor, opts SellOptions) SellResult`

Shared seller entry point for players and mobs. Resolves the first willing
merchant in the seller's room, then sells matching items.

**Two sell models — important distinction:**

1. **SALE (gold transfer) via `actions.Sell`**: The merchant evaluates the
   item via `EvaluateBuyRules` / `GetSellPrice`, returns an offer price, and
   the item transfers to the shop's stock. For **player** sellers the merchant's
   gold pool is drawn down by the sale price (`shopInv.Gold -= sellValue` or
   `mob.Character.Gold -= sellValue`), and a merchant-broke gate prevents
   sales the merchant can't afford. For **mob** sellers the seller is credited
   (`char.Gold += sellValue`) but **shop gold is not touched** — mob sales
   mint gold without bankrupting the shop (gated on `seller.IsPlayer()`).

2. **SUPPLY HANDOFF (free, no gold) via `forager.SellToVendor` and
   `forager.BackfillVendorFromChests`**: Forager delivery and chest backfill
   transfer items directly into vendor stock with no price computation and no
   gold movement. These are supply-side operations, not market transactions.

**SellAllSellable mode** (`opts.SellAllSellable = true`): Mob inventory-sweep.
Iterates every item in the seller's backpack that is not a quest token, not a
crafting material, and has `Value > 0`; calls `sellOneToMerchant` for each.
Used by the goal planner's wealth-gold sell step.

**Options struct:**
```go
type SellOptions struct {
    ItemName        string // ignored when SellAllSellable
    Quantity        int    // 1, N, or UnlimitedSell (math.MaxInt)
    SellAllSellable bool   // mob inventory-sweep mode
    MerchantName    string // optional target; "" = first willing merchant
}
```

**Result struct:**
```go
type SellResult struct {
    Sold         int
    TotalGold    int
    Reason       SellStopReason
    LastItemName string
    Mixed        bool // the items sold were not all the same thing; do not pluralise LastItemName
}
```

`SellStopReason` values: `SellStopSoldAll` (normal), `SellStopNoItem`,
`SellStopNoMerchant`, `SellStopMerchantBroke` (player path only),
`SellStopRejected`, `SellStopNoSight` (lighting plan 5b, see below).

**Messaging:** Player sellers receive "You don't have that item." and merchant
speech lines synchronously (not via the mob's async command queue — see
`merchantSay` in `sell.go`). Mob sellers receive no feedback text.

**Progression:** Triggers `seller.OnSkillUse("bartering")` and
`mob.Character.OnStatUse("charisma")` on each successful sale.

Entry points that call `Sell`:
- `usercommands/sell.go` — player `sell` command
- `mobcommands/sell.go` — mob `sell` command
- `internal/planners/` — goal planner's wealth-gold save-up sell step

**Baubles (`sell_bauble.go`, docs/baubles Phase 2).** A bauble is item 900 plus
a catalog id, so it cannot go through the ItemId-keyed pricing and stocking
above. `sellOneToMerchant` hands it to `sellBaubleToMerchant`, and
`resolveMerchant` probes it with `baubleOfferFor`:

- Price: `BaublePrice(catalog value)` = value × `ShopBuyRatio`, rounded up,
  at least 1. No scarcity curve, no barter bonus (the affixed-loot spread).
- Who buys: living-economy shops whose `CraftSupport` is listed in
  `Balance.BaubleBuyerCraftSupports` (default `general`, `jewelcrafting`;
  `baubleShopBuys`), and every legacy merchant.
  Living-economy shops also keep their `ShopGoldReserveRatio` reserve.
- Refusals are spoken: unknown record, wrong kind of shop, can't afford,
  and (Phase 6c) a hot stolen bauble at an honest merchant (`baubleSayHot`,
  which hints at a fence).
- Stolen goods (Phase 6c): `IsFence(mob)` is a mob in one of
  `Balance.BaubleFenceGroups` (default `fence`; the roster is in the plan,
  Phase 6c, and `TestEveryTownHasAFenceNearby` checks every town has one in
  or near it, that every fence is a non-combatant shopkeeper, and that
  Torvan Cresk, whom quest 14 has players fight, is not one). `IsFence`
  delegates to `mobs.Mob.IsFence`. A fence whose shop exists only for the
  trade carries no `craft_support`, so `EvaluateBuyRules` refuses it any
  ordinary loot and its gold is kept for baubles
  (`TestStolenBauble_AFenceShopRefusesOrdinaryLootButBuysBaubles`). A
  fence is a merchant like any other and pays from its shop's gold
  (`baubleMerchantGold`: the living-economy shop's, or a legacy merchant's
  purse). A fence buys every bauble, paying `FencePrice` (value ×
  `BaubleFenceBuyPct`, 60%, rounded up) for any stolen one not given back
  since (`Record.StolenGoods`), hot or cold, and the honest `BaublePrice`
  for the rest. An honest merchant refuses one hot WHERE IT TRADES
  (`baubles.Record.HotIn(zone, now)`: stolen within `BaubleStolenHeatHours`,
  in the same heat area, the zone or its `BaubleHeatAreas` group; read
  through the `baubleNowForSale` clock; a shop that buys no trinkets says so
  first) and buys it anywhere else, or once cooled, at the honest price.
  `baubleOfferFor(item, shopInv, fence, zone)`: the sale room's zone, or
  `merchantZone(mob)` for `offer`/`appraise`; `resolveMerchant(room, probe,
  playerSale)` sends a bauble to the best offer in the room that the
  merchant can pay (`bestBaubleMerchant`; a merchant's gold constrains only
  a player's sale), and `sellNamed` picks the buyer again for each bauble.
- The shelf (slice D): a player's sale of an average or rare, non-retired
  bauble (`baubles.Record.Shelvable`, the rule shared with the auction win
  path in `modules/auctions`) to a living-economy shop puts it on the shop's
  secondhand shelf (`AffixedStock`) at its catalog value, held out of sight
  until `baubles.ShelfHoldUntil` while it is hot. A mob's sale, a legacy
  merchant, a cheap or a retired bauble still leaves the world. Every sale
  marks the record sold (`baubles.MarkSold`) and saves a living shop. A shop
  holding `ShopAffixedStockCap` hot baubles refuses another shelvable hot
  one (`baubleSayBackroomFull`, an interest refusal, so the next merchant is
  tried). `buy` (`tryPurchaseFromInventory`) offers the shelf in
  `ListedIndexes` order, never a held entry, selects by position
  (`util.FindMatchIndexIn`, so `buy 2.trinket` takes the second), names a
  bauble in the buyer's own view (`NameFor`, `DisplayNameFor` on the
  buyer's line), trims the listed cap lazily, rolls back with
  `RestoreAffixedStock`, and calls `baubles.MarkBought` on a buyback.
- `BaubleOfferFrom(item, mob)` is the same offer for the `offer` and
  `appraise` commands. `mobs.GetSellPrice` returns 0 for any bauble.
- `sellNamed` names each sale by the item actually sold (not the probe), and
  sets `SellResult.Mixed` when they differ, so `sell all bauble` reports
  "3 items" rather than pluralising one bauble's name.

**See also:** `internal/forager/vendor_sell.go` (`forager.SellToVendor`) and
`internal/forager/chest_backfill.go` (`forager.BackfillVendorFromChests`) for
the free supply-handoff paths — these are NOT routed through `actions.Sell`.

### Shop sight gate (lighting plan 5b, `shop_sight.go`)

`list`, `buy` and `sell` all need full sight to deal: below the faces band
(`messaging.LightBand`) a customer can't make out the goods.

- **`ShopSightRefusal(c *characters.Character, room *rooms.Room) bool`**,
  true when `messaging.LightBand(c, room) < messaging.BandFaces`. A nil room
  reads as no refusal (callers have already handled "no room" their own way).
  Mirrors `ShopClosedForSleep` beside it in `sleeping_target.go`: `Buy` and
  `Sell` check it only once a merchant is confirmed present in the room
  (`SellStopNoSight` / `BuyReasonNoSight`), so "there's no merchant here"
  still wins over a sight refusal, and a nil-Mob `MobActor` test fixture
  (no merchant path) never calls `GetCharacter()` unguarded. `usercommands.List`
  checks it directly, right after its own sleep gate.
- **`ShopSightRefusalText`**, the one line every refusing verb prints:
  "You can't make out the goods well enough to deal."
- **Content keeps every shop above the gate at night** (merchants slice,
  2026-09-30). Street and stall keepers wear an Oil Lantern (40038) in the
  `light` equipment slot; shops inside buildings authored with an outdoor
  biome carry room overrides `skylight: 0.15`, `lamp: 50`. The root guard
  `shop_night_trade_guard_test.go` spawns every real shop keeper alone in
  each room that spawns it and asserts `ShopSightRefusal` is false for a
  bare `characters.Character` at every sunless hour of three sample days
  while the keeper's schedule has it awake in that room. A new shop in a
  dark room fails it. An equipped light also shines while its keeper
  sleeps, which is harmless to trade because `ShopClosedForSleep` refuses
  first.
- **`barterDiscount(char *characters.Character, room *rooms.Room, maxFrac float64) float64`**,
  the ONE place the bartering discount is computed: skill-derived, capped at
  `maxFrac` (skill 50 reaches the cap), times `messaging.SightMult(char, room)`.
  A dazzled haggler (bright band, still full sight) bargains worse than a
  comfortable one; a nil room reads as comfortable (mult 1.0), matching
  `SightMult`'s own nil-room reading. Replaces three previously hand-rolled,
  slightly inconsistent discount computations in `buy.go` and `sell.go`.
  `maxFrac` is always the shipped knob, never a Go literal:
  `configs.GetBalanceConfig().BarterMaxDiscount` for a buyer's price cut
  (`tryPurchaseFromInventory` in `buy.go`), `.BarterMaxBonus` for a seller's
  price bump (`sellOneToMerchant` in `sell.go`); the two differ because buy
  and sell name different knobs, so the caller reads its own before calling in.

A mob actor gets no refusal text: `MobActor.SendText` is already a no-op, so
the gate calls `SendText` unconditionally guarded on `buyer.IsPlayer()` for
readability, matching every other refusal in `buy.go`/`sell.go`. Both refusal
sends use `messaging.CategorySystem`, matching `ShopClosedForSleep`'s own
refusal category and the neighbouring "Visit a merchant" lines.

---

## Counter tier (U6b Task 10)

`combat_counter.go` wires the counter tier on this package's exits:

- `counterSkillMoveExit(actor, defender, move, shape combatvocab.Attack, sameRoom)` fires
  `combat.ExecuteCounter` at every `ExecuteSkillMove` consumer's
  defensive-crit exit (bash/gore/hamstring/kick/maul/pounce/rake/throttle/
  trip/drain, plus `ExecuteFire` with `sameRoom = !crossRoom` —
  the cross-room shot is the one single-target attack that cannot be
  countered). It refuses results
  carrying `SkillMoveResult.IsCounter`, so a counter never earns a counter.
- `executeCounterTaunt(counterer, target)` is the defy carve-out: a defy CRIT
  counter-TAUNTS instead of counter-swinging. Its dispatch is the exported
  `FireCounterTaunt`, shared by taunt's exit here (`counterTauntExit`) and the
  spell exit in `internal/hooks` (a defied charm), because `internal/combat`
  cannot import this package.
  It bypasses the special-move cooldown, U8 admission cost, and all aggro
  mutation (owner decisions 2026-08-19), reuses only the contest + damage
  shape of taunt resolution, and never inspects its own contest's
  `DefensiveCrit`. `TauntResult.Counter` carries its outcome.

Counter-swings route through the seam, so the ORIGINAL attacker defends them
and is charged + progressed for it (the countered-party economy).

Narration is rendered by `internal/combat` from the pool of the defence that
won (`items.CounterPoolFor`; the defy answer from `counter-defy` via
`combat.BuildCounterTauntMessages`). SEQUENCING:
`counterSkillMoveExit` does
NOT dispatch — messages render in call order and the wrappers narrate after
`ExecuteX` returns, so the `CounterResult` rides up on each action's result
struct (`Counter` field on Bash/Drain/Fire/Gore/Hamstring/Kick/Maul/Pounce/
Rake/Throttle/Trip results) and the command
wrapper calls the exported `DispatchCounterMessages(actor, res)` AFTER its
own outcome text. The defy counter-taunt still dispatches immediately, from
the exported `FireCounterTaunt` (shared by `counterTauntExit` and the
`internal/hooks` spell exit for a defied charm; Task 10's review accepted the
taunt path's ordering).

## Available Actions Summary

| Action | Package | Actor→Target | Returns | Messaging | Cooldown |
|--------|---------|---|---|---|---|
| Consider | actions | self vs target | ConsiderResult | player only | none |
| Defuse | actions | self vs trap | DefuseResult | varies | none |
| Drink | actions | self | DrinkResult | both | none |
| Forage | actions | self vs biome | ForageResult | varies | shared |
| Plant | actions | self vs mob/container | PlantResult | varies | shared |
| Salvage | actions | self vs corpse/item | SalvageResult | varies | none |
| Scan | actions | self → adjacent | ScanResult | user only | none |
| Search | actions | self vs room | SearchResult | user only | shared |
| Shadow | actions | self→target | ShadowResult | varies | ShadowCooldown (start, stop, spotted) |
| Sneak | actions | self vs room | SneakResult | silent | shared |
| Steal | actions | self vs mob/player/container | StealResult | varies | shared |
| ExecuteFire | actions | self vs target (same/adjacent room) | FireResult | both | shared (special-move), EVERY shot |
| BeginFlee | actions | self (command half) | FleeBegin | wrapper renders | none |
| ResolveFlee | actions | self vs blocker (round half) | FleeOutcome | wrapper renders | none |
| Sell | actions | self vs merchant | SellResult | player only | none |
| Sleep | actions | self | SleepResult | varies | none |
| Track | actions | self vs trail/target | TrackResult | user only | shared |

---

## Options Structs

All chunk 2.7+ actions expose `<Verb>Options` structs for caller-side target
structuring:

```go
type DefuseOptions struct {
	ContainerId int    // container with trap
	Direction   string // cardinal (north/south/east/west)
	ExitName    string // friendly exit name
	UseKit      bool   // whether to consume disarm kit
	// ContainerId checked first; if 0, uses Direction+ExitName
}

type PlantOptions struct {
	ItemTag           string // noun phrase from command
	TargetMobId       int    // mob to plant on
	RoomContainerId   int    // container to plant in
	// Only one of the two should be set; TargetMobId checked first
}

type ScanOptions struct {
	HostileOnly bool // if true, only return entities actor hates
}

type SearchOptions struct {
	Feature string // `search <feature>`; empty searches the room (search_feature.go)
}

type ShadowOptions struct {
	TargetMobInstanceId int // mob to shadow
	TargetUserId        int // player to shadow
	// Exactly one should be set; the mob id is checked first
}

// Sneak has NO options struct. Its entry point is Sneak(actor Actor)
// SneakResult: it targets self against every observer in the room, so there is
// nothing to configure.

type StealOptions struct {
	TargetMobId       int    // mob to pickpocket
	TargetUserId      string // player to pickpocket
	RoomContainerId   int    // container to rob
	// Only one of the three should be set; first non-zero wins
}

type TrackOptions struct {
	TargetNoun string // optional target noun for active track
	// Empty = trail-sniff; non-empty = active track on resolved target
}
```

---

## Caller Integration

**User commands** (`internal/usercommands/`): Parse CLI args into
`<Verb>Options`, call the action function, process the result struct.

**Mob commands** (`internal/mobcommands/`): Build options from command args
or script context, call the action function.

**Behavior trees** (`internal/behaviortree/`): BTree action primitives
(`try_sneak`, `try_steal`, etc.) populate options from `EvalContext.Event`
and `mob.Character.Aggro` context, call the action function, return
Success/Failure based on the result.

---

## Cooldown System

Skullduggery actions (Sneak, Steal, Plant) share a single cooldown key
(`"skullduggery"`). Config: `SkullduggeryActionCooldown` (default 10 rounds).

- Tracked in `Character.Cooldowns` map (string → int remaining rounds).
- Cooldowns decrement each round via combat hooks.
- Expired cooldowns are cleaned up lazily when checked.

`"special-move"` is ONE shared timer across every special move
(`SpecialMoveCooldown`, 4 rounds shipped). **Claim it through
`ClaimSpecialMove` and read it through `SpecialMoveReady`
(`special_move_cooldown.go`), never by hand.** The tag used to be typed at 56
call sites in five idioms, and they had already drifted: one path ran a
hardcoded `"1 rounds"` against everyone else's configured value, and nothing
could see it.

`ExecuteFire` claims it on **every shot**, ordinary or not, because firing
chambers its own next round. That is not a rate change: before the fold, the
shot claimed nothing and the separate `reload` claimed instead, so a
shot-plus-reload cycle already cost exactly one burn.

⚠️ **The claim does NOT gate the ambush.** A shot from stealth is a surprise
shot because the shooter was hidden, full stop. The old gate refused the opener
whenever the timer was spent — which meant refusing it whenever the player had
reloaded, the very thing you must do to have something to ambush with. Now that
an ordinary shot claims too, re-adding the gate would deny the opener after ANY
previous shot. Stealth is what limits ambushes: the shot reveals you and `sneak`
refuses in combat, so an engagement yields one opener.

Melee is DIFFERENT and deliberately so: `EngageAggroType` still gates its opener
on the timer, because melee has no per-swing claim to collide with.

`FireResult.Revealed` is the companion flag: a surprise shot gives the shooter's
position away, which `IsSneaking` (a snapshot taken before any reveal) does not
tell you. `FireResult.Chambered` carries the auto-reload's outcome, and its
`NoAmmo` / `BundleEmptied` flags are how running dry reaches the player.

---

## Dependencies

- `internal/characters` — Character stats, conditions, inventory, cooldowns
- `internal/combat`: power calculations (Consider), and contest resolution.
  The stealth, theft, trap and detection contests in `sneak.go`, `shadow.go`,
  `steal.go`, `plant.go` and `defuse.go` resolve through
  `combat.RunContest(attackScore, []contest.Entry{{Score: defenseScore}})`
  (U4 routed them to a wrapper, U6 collapsed every wrapper into this one entry
  point). For OPPOSED contests, do not reach `internal/contest` directly; this
  package goes through `internal/combat`.

  **Sight ramp on score-only rolls (lighting plan 5b).** Every hand-built
  score here pays `messaging.SightMult` on the party who needs to SEE, once
  per roll per party. `CalcDetectionScore(c, room messaging.RoomVisibility)`
  applies it for the OBSERVER (pass the observer's room; nil is unity; the
  hider's side already folds light in through `CalcSneakScoreVsObserver`,
  which takes the room as a `messaging.RoomVisibility`, usually a hoisted
  `messaging.FixedLight`, and counts it lit for an observer whose
  `messaging.LightBand` is not `BandDark`; lighting plan 5c replaced the old
  `roomLit || nightvision flag` test), so
  every detection caller (sneak, go, search's `spotsHider`, track's opposed
  contest, the steal/plant/shadow notice rolls) gets it by construction.
  `stealVictimScore(c, room)` does the same for the theft and plant
  victims and container bystanders: noticing is the victim's roll, so the
  victim pays their own eyes inside the helper.
  Actor-side sites multiply where the score is computed: the thief's and
  planter's attack score (once in `Steal`/`Plant`, feeding all three
  sub-paths), the shadower's sneak score, the defuser's score, the searcher's
  static-tier score in `Search`, the tracker's `searchScore` in `Track` (fed
  to `resolveTrailDetail`, which stays pure), the forager's
  `ForageAttempt.SearchScore` (`ForageCore` stays pure), the salvager's
  `score` in `Salvage`, and the caster's `hold` in `ExecuteThrottle`'s cast
  interrupt. Voice contests never call it.

  **The one exception, added by U10b-1b Phase A: STATIC-DIFFICULTY checks call
  `contest.AgainstDifficulty` directly**, because there is no `internal/combat`
  wrapper for them and deliberately never will be — `combat.RunContest`'s doc
  comment reserves itself for opposed contests and says static-difficulty rolls
  stay unfloored ("Do not route them here to 'unify' them"). `search.go`'s four
  non-stealth tiers take that path, as do `track.go`'s three trail-read bands
  (`resolveTrailDetail`, a NESTED ladder so a finer band cannot be won without
  the coarser one).

  The OPPOSED checks in the same two files go to `combat.RunContest` instead,
  because they have a real opponent and take `ContestFloor`: `search.go`'s two
  hidden-entity checks (`spotsHider`, U10b-1b Phase C — reconciled onto the form
  `usercommands/go.go` already used) and `track.go`'s named-target contest,
  where the quarry defends with its own sneak score. (A separate case, `surprise_attack.go`, had no hit
  resolution at all — not a flat threshold but an unconditional auto-hit with
  no defender term anywhere. U10d deleted it outright rather than giving it a
  contest: the opening strike of the ordinary combat round is the surprise
  now, and `EngageAggroType` in `combat_attack.go` is all that remains here.)
- `internal/users` — Player character management
- `internal/mobs` — NPC management
- `internal/rooms` — Room context, containers, exits
- `internal/items` — Item specs, damage calculations
- `internal/baubles`: Bauble catalog records, for pricing and marking sales (`sell_bauble.go`)
- `internal/conditions` — Condition system (Hidden condition for Sneak/Shadow)
- `internal/skills` — Skill progression and names
- `internal/hooks`: `RoomChangeShadowFollow` moves a shadower after its quarry (Shadow sets the state it reads)

---

## Files

The package is one file per action, plus a small shared core. Naming is the
map: `combat_*.go` is a combat special, `mutation_*.go` a mutation active, and
the rest are ordinary verbs.

| Group | Files |
|-------|-------|
| Actor abstraction | `actor.go`, `actor_user.go`, `actor_mob.go` |
| Readiness gates | `action_readiness.go`, `command_readiness.go` |
| Targeting | `target_resolution.go`, `target_helpers.go`, `melee_target.go`, `sleeping_target.go` |
| Shared helpers | `combat_helpers.go`, `skill_helpers.go`, `mutation_helpers.go`, `aggression.go`, `bleed.go` (`bleedPerRound`) |
| Combat specials | `combat_attack.go`, `combat_bash.go`, `combat_counter.go`, `combat_drain.go`, `combat_fire.go`, `combat_gore.go`, `combat_grapple.go`, `combat_hamstring.go`, `combat_kick.go`, `combat_maul.go`, `combat_pounce.go`, `combat_rake.go`, `combat_rally.go`, `combat_reload.go`, `combat_taunt.go`, `combat_throttle.go`, `combat_trip.go`, `combat_warcry.go` |
| Casting | `cast.go`, `cast_interrupt.go` |
| Flee | `flee.go`, `relocate_mob.go` |
| Mutation actives | `mutation_cocoon.go`, `mutation_venom_coat.go` |
| Stealth / perception | `sneak.go`, `shadow.go`, `search.go`, `search_bauble.go` (roll and delayed delivery), `search_feature.go` (`search <feature>`), `scan.go`, `track.go`, `steal.go`, `steal_pocket.go` (a player's pickpocket pause and bauble) |
| Items & economy | `get.go` (`GetItemFromFloor` refuses a household's bauble (`BaubleBelongsTo`) with `ErrHouseholdBauble`, the item found and nothing moved, for every taker: a player's `get`, a mob's, a companion's, a scavenger's (owner ruling 2026-09-29); its gates sit in one early-return block), `drop.go`, `give.go`, `transfer.go`, `buy.go`, `sell.go`, `sell_bauble.go`, `stolen_bauble.go` (heat, recognition, returns), `remove_equip.go`, `shop_sight.go`, `drink.go` |
| Trades | `craft.go`, `salvage.go`, `forage.go`, `plant.go`, `defuse.go` |
| Movement & state | `go.go`, `sleep.go`, `consider.go` |
| Social | `say.go`, `emote.go`, `emote_aliases.go` |
| Divergences | `divergences.go` — deliberate departures from upstream behaviour |

**The actor seam is the point of this package.** `actions.Actor` lets one
implementation serve both players and mobs, which is what keeps user and mob
commands in parity instead of drifting apart.

## Fixtures (lighting 5e)

`TakeFloorItem` refuses a fixture (`items.Item.IsFixture`) with `ErrFixture`
right after `ErrTooDark`, for every taker: a player's `get`, a mob's, a
companion's, a scavenger's. `steal`'s floor branch only ever takes a
household bauble, and the player command names a fixture as fixed in place
before it gets there, but only to one who can see the floor
(`TooDarkToGet`). `TargetAsleep` is also the predicate the item condition
`holder_asleep` reads (owner ruling R2).
