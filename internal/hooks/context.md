# Hooks System Context

> **Read this first.** Every fenced `go` block below is **illustrative
> pseudo-code**, not a transcript of the source. Several use helper names that
> do not exist in the codebase (`Character.GetCurrentQuestToken`,
> `Character.ClearAggro`, `users.RemoveUser`, `Room.GetMobIds`); they are there
> to show the *shape* of a handler, not its exact calls. Verify any symbol
> against the source — or `codegraph_search` — before coding against it.
>
> The authoritative list of what this package listens to is
> **`RegisterListeners()` in `hooks.go`**. The listener count below is also
> historical: the package currently has 124 non-test files.

## Overview

The hooks system provides event-driven game logic through a collection of
specialized event listeners handling everything from combat rounds to quest
progression. It is the primary integration layer between the event system and
game mechanics — combat resolution, mob AI, player lifecycle, and system
maintenance.

## Architecture

The hooks system is built around several key categories:

### Core Components

**Event Registration System:**
- Centralized listener registration in `RegisterListeners()`
- Type-safe event handling with proper casting
- Ordered execution with priority support (events.Last)
- Comprehensive coverage of all game events

**Game Loop Hooks:**
- **NewRound Events**: Combat, healing, mob AI, player ticks
- **NewTurn Events**: Autosave, cleanup, condition management
- **Player Lifecycle**: Spawn, despawn, character changes
- **System Maintenance**: VM pruning, zombie cleanup, respawns

**Gameplay Integration:**
- **Combat System**: Full combat round processing with multi-target support
- **Quest System**: Progress tracking and reward distribution
- **Condition System**: Application, expiration, and effect processing
- **Audio System**: MSP sound effects and location-based music

## Key Features

### 1. **Comprehensive Game Loop Management**
- **Round Processing**: 15 different NewRound event handlers
- **Turn Processing**: 4 NewTurn event handlers for maintenance
- **Combat Integration**: Complete combat round resolution
- **Mob AI Processing**: Idle behavior and action execution

### 2. **Player Lifecycle Management**
- **Join/Leave Handling**: Player spawn and despawn processing
- **Character Updates**: Broadcasting character changes
- **Skill Progression**: Skill-use notifications and guide spawning (Level-up disabled in DOGMud)
- **Connection Management**: Zombie cleanup and inactive player handling

### 3. **Quest and Progression Systems**
- **Quest Processing**: Multi-step quest advancement and rewards
- **Item Integration**: Quest item requirements and rewards
- **Skill Advancement**: Skill-based quest completion
- **Progression Distribution**: Skill progression rewards and notifications

### 4. **System Maintenance and Optimization**
- **Automatic Cleanup**: Zombie connections, expired conditions, ephemeral rooms
- **Resource Management**: VM pruning, memory optimization
- **Data Persistence**: Automatic user saves and data integrity
- **Performance Monitoring**: Event processing and system health

## Event Listener Categories

### NewRound Event Handlers (14 handlers)
```go
// Core game loop processing every round
events.RegisterListener(events.NewRound{}, InactivePlayers)       // Handle AFK players
events.RegisterListener(events.NewRound{}, UpdateZoneMutators)    // Update zone effects
events.RegisterListener(events.NewRound{}, CheckNewDay)           // Day/night cycle
events.RegisterListener(events.NewRound{}, SpawnLootGoblin)       // Special mob spawning
events.RegisterListener(events.NewRound{}, UserRoundTick)         // Player round processing
events.RegisterListener(events.NewRound{}, MobRoundTick)          // NPC round processing
events.RegisterListener(events.NewRound{}, HandleRespawns)        // Mob respawning
events.RegisterListener(events.NewRound{}, DoCombat)              // Combat resolution
events.RegisterListener(events.NewRound{}, AutoHeal)              // Natural healing
events.RegisterListener(events.NewRound{}, IdleMobs)              // Mob idle behavior
```

### NewTurn Event Handlers (4 handlers)
```go
// System maintenance every turn (multiple rounds)
events.RegisterListener(events.NewTurn{}, CleanupZombies)         // Remove disconnected users
events.RegisterListener(events.NewTurn{}, AutoSave)               // Automatic data saves
events.RegisterListener(events.NewTurn{}, PruneConditions)             // Remove expired conditions
events.RegisterListener(events.NewTurn{}, ActionPoints)           // Regenerate action points
```

### Player Lifecycle Handlers
```go
// Player connection and character management
events.RegisterListener(events.PlayerSpawn{}, HandleJoin)         // Player login processing
events.RegisterListener(events.PlayerDespawn{}, HandleLeave, events.Last) // Player logout (final)
events.RegisterListener(events.PlayerDrop{}, HandlePlayerDrop)    // Unexpected disconnection
events.RegisterListener(events.CharacterCreated{}, BroadcastNewChar) // New character announcements
events.RegisterListener(events.CharacterChanged{}, BroadcastNewChar) // Character update announcements
```

### Game Mechanics Handlers
```go
// Core gameplay systems
events.RegisterListener(events.Quest{}, HandleQuestUpdate)        // Quest progression
events.RegisterListener(events.Condition{}, ApplyConditions)               // Condition application
events.RegisterListener(events.LevelUp{}, SendLevelNotifications) // Level-up messages
events.RegisterListener(events.LevelUp{}, CheckGuide)             // Guide NPC spawning
events.RegisterListener(events.ItemOwnership{}, CheckItemQuests)  // Item-based quests
events.RegisterListener(events.MobIdle{}, HandleIdleMobs)         // Mob AI behavior
```

## Combat System Integration

**Names in the dark.** Crit effect lines (`sendCritEffectTrio`), the
return-damage recoil lines (`emitReturnDamageText`), counter lines
(`actions.SendCounterTrio`) and spell lines between two parties
(`spellAudience` in `spell_audience.go`, used by the appliers in
`spell_effects.go` and `spell_help_effects.go`,
`sendSpellChannelDefenceMessages` and
`resolvePurgeAffliction`, which takes a `purgeTarget`: a player, a mob such as a
charmed companion, or the caster) all go through `messaging.SendTrio`, so a reader who
cannot see the other party reads "something", or "a figure" with infrared.
A self-cast (the caster is its own target, `spellEffectCtx.selfCast`) has no
second party to pair against: the helpful appliers send the caster's own
line and a room line naming the caster once, through `SendTrio` with
`selfCastAudience` (`Actee` is `messaging.NoLine`), so a shapes-only
observer reads "a figure". **M4d PR 3** first moved the player self-cast
room lines off `sendVisualRoomText`, which never calls `messaging.HideNames`;
parity slice 3b made the same pair serve a mob casting on itself.
The retarget notice ("You turn your attention to X!") is built once, by
`actions.RetargetNotice` (`internal/actions/retarget_notice.go`), for
`DoCombat`'s validate-aggro pass, `emitRetargetMessage`, and the mob-departure
retarget in `actions.ClearRoomAggroOnDeparture` (moved from `mobcommands` by
flee parity slice 4a, so `RelocateMob` and a walking `Go` share it); it hides X by the
reader's sight and is not suppressed in the dark, because each caller picks
the new target from whoever is already attacking the reader. The ordinary
melee swing lines used to be rewritten here too, by `replaceDarknessMessages`
(`NewRound_DoCombat_helpers.go`); **M4d PR 2 deleted that function.** Identity
hiding for a swing's personal lines now happens upstream, inside
`internal/combat`, before this package ever sees the `AttackResult` — see
`hideIdentitiesInPersonalLines` in `internal/combat/context.md`. This
package's own job is unchanged for everything ELSE it renders (the trio
lines above, the wait-round personal lines below). The wait-round
participant (personal) lines (`handleCombatWaitRound` in
`NewRound_DoCombat_resolution.go`, drained from `combat.GetWaitMessages`'s
authored `{actor}`/`{actee}` text) have no swing events for identity hiding to
act on, so they judge sight directly with the swing path's own predicate
(`messaging.CanSeeSightImpairedOnly`) and, for a participant without clear
sight, send one fixed dark line instead of the authored one, the swing
path's convention, because the authored wait lines name the weapon as well
as the foe. Infrared readers get the same dark line as full darkness here,
as they do on the swing path; only clear sight reads the authored text.
**M4d PR 3** moved the wait-round's ROOM lines out of this package entirely:
`combat.GetWaitMessages` now delivers them itself through `messaging.SendTrio`
(see `internal/combat/context.md`'s "Wait-round room-line identity hiding"),
so `handleCombatWaitRound` no longer drains
`roundResult.MessagesToSourceRoom`/`MessagesToTargetRoom` — that drain was
removed as dead code rather than kept as a no-op, because `GetWaitMessages`
never populates those fields on any branch any more.

### Combat Round Processing
```go
func DoCombat(e events.Event) events.ListenerReturn {
    evt := e.(events.NewRound)
    
    // Process all active combat encounters
    for _, user := range users.GetAllActiveUsers() {
        if user.Character.IsAggro() {
            // Handle player combat
            processCombatRound(user)
        }
    }
    
    // Process mob vs mob combat
    for _, mobInstanceId := range mobs.GetAllMobInstanceIds() {
        mob := mobs.GetInstance(mobInstanceId)
        if mob != nil && mob.Character.IsAggro() {
            processMobCombat(mob)
        }
    }
    
    return events.Continue
}

// Combat processing includes:
// - Multi-target combat resolution
// - Weapon durability and breakage
// - Death handling and consequences
// - Experience and loot distribution
// - Combat state management
```

### Archer Re-engagement Exemption (`archerReengageable`)

Normally a mob with no active Aggro is skipped in the combat loop (no btree
eval). An exception fires for kiting archer mobs: `archerReengageable(mob,
room, round)` returns true when ALL of the following hold:

1. The mob has an equipped ranged weapon (main or offhand).
2. `mob.CombatMemory` is non-nil.
3. The memory has not expired per `CombatMemoryDuration` (Balance config,
   default 300 rounds) — prevents stale memories triggering indefinitely.
4. The remembered target's last-seen room is the mob's own room **or**
   exactly one exit away (the bounded spatial engagement window).

When true, the mob proceeds to its behavior tree even without Aggro, allowing
a kiting archer that just retreated (clearing its Aggro) to `try_fire` on the
remembered target in the same round rather than standing inert for a full tick.
Non-archer mobs are unaffected; the unconditional nil-aggro skip applies to them.

## Quest System Integration

### Quest Progress Handling
```go
func HandleQuestUpdate(e events.Event) events.ListenerReturn {
    evt := e.(events.Quest)
    
    user := users.GetByUserId(evt.UserId)
    if user == nil {
        return events.Cancel
    }
    
    // Validate quest progression
    if !quests.IsTokenAfter(user.Character.GetCurrentQuestToken(), evt.QuestToken) {
        return events.Cancel
    }
    
    // Update quest progress
    user.Character.SetQuestFlag(evt.QuestToken)
    
    // Check for quest completion
    quest := quests.GetQuest(evt.QuestToken)
    if quest != nil && isQuestComplete(quest, evt.QuestToken) {
        distributeQuestRewards(user, quest)
    }
    
    return events.Continue
}

// Quest processing includes:
// - Multi-step quest validation
// - Item requirement checking
// - Skill-based quest completion
// - Reward distribution (gold, items, experience, skills)
// - Chained quest activation
```

## Player Lifecycle Management

### Player Join Processing
```go
func HandleJoin(e events.Event) events.PlayerSpawn {
    evt := e.(events.PlayerSpawn)
    
    user := users.GetByUserId(evt.UserId)
    if user == nil {
        return events.Cancel
    }
    
    // Handle first-time login
    if user.Character.Level == 1 && user.Character.Experience == 0 {
        handleNewPlayerSetup(user)
    }
    
    // Broadcast join message
    broadcastPlayerJoin(user)
    
    return events.Continue
}
```

### Player Leave Processing
```go
func HandleLeave(e events.Event) events.ListenerReturn {
    evt := e.(events.PlayerDespawn)
    
    user := users.GetByUserId(evt.UserId)
    if user == nil {
        return events.Cancel
    }
    
    // Save user data
    if err := user.Save(); err != nil {
        mudlog.Error("HandleLeave", "userId", evt.UserId, "error", err)
    }
    
    // Clean up combat state
    user.Character.ClearAggro()
    
    // Broadcast leave message
    broadcastPlayerLeave(user)
    
    return events.Continue
}
```

## System Maintenance Hooks

### Automatic Cleanup
```go
// Zombie connection cleanup
func CleanupZombies(e events.Event) events.ListenerReturn {
    evt := e.(events.NewTurn)
    
    expirationTurn := evt.TurnNumber - configs.GetNetworkConfig().LogoutRounds
    expiredZombies := users.GetExpiredZombies(expirationTurn)
    
    for _, userId := range expiredZombies {
        user := users.GetByUserId(userId)
        if user != nil {
            user.Save()
            users.RemoveUser(userId)
        }
    }
    
    return events.Continue
}

// Condition expiration management (abridged; see NewTurn_PruneConditions.go).
// The holder's line comes from the spec's end narration, which reads
// ConditionSpec.EndUserNotice (authored end_actee, else the generic
// "<Name> has expired.", nothing for a secret condition); ApplyConditions
// reads StartUserNotice the same way. Slice C, 2026-09-12.
func PruneConditions(e events.Event) events.ListenerReturn {
    // Prune player conditions, room by room
    for _, roomId := range rooms.GetRoomsWithPlayers() {
        room := rooms.LoadRoom(roomId)
        for _, uId := range room.GetPlayers(rooms.FindWithConditions) {
            user := users.GetByUserId(uId)
            for _, conditionInfo := range user.Character.Conditions.Prune() {
                // send the end narration to the holder and the room
            }
        }
    }
    
    // Prune mob conditions
    for _, mobInstanceId := range mobs.GetAllMobInstanceIds() {
        mob := mobs.GetInstance(mobInstanceId)
        if mob != nil {
            mob.Character.Conditions.Prune()
        }
    }
    
    return events.Continue
}
```

### Automatic Saves
```go
func AutoSave(e events.Event) events.ListenerReturn {
    evt := e.(events.NewTurn)
    
    // Save all active users periodically
    if evt.TurnNumber%configs.GetGamePlayConfig().AutoSaveFrequency == 0 {
        for _, user := range users.GetAllActiveUsers() {
            if err := user.Save(); err != nil {
                mudlog.Error("AutoSave", "userId", user.UserId, "error", err)
            }
        }
    }
    
    return events.Continue
}
```

## Audio and Visual Effects

### MSP Sound System
```go
func PlaySound(e events.Event) events.ListenerReturn {
    evt := e.(events.MSP)
    
    user := users.GetByUserId(evt.UserId)
    if user == nil || !user.ClientSettings().IsMsp() {
        return events.Continue
    }
    
    // Send MSP sound command
    soundCommand := fmt.Sprintf("!!SOUND(%s)", evt.SoundFile)
    user.SendText(soundCommand)
    
    return events.Continue
}

// Location-based music changes
func LocationMusicChange(e events.Event) events.ListenerReturn {
    evt := e.(events.RoomChange)
    
    user := users.GetByUserId(evt.UserId)
    if user == nil {
        return events.Continue
    }
    
    room := rooms.LoadRoom(evt.RoomId)
    if room != nil && room.MusicFile != "" {
        if user.LastMusic != room.MusicFile {
            user.PlayMusic(room.MusicFile)
            user.LastMusic = room.MusicFile
        }
    }
    
    return events.Continue
}
```

## Mob Round Tick (`NewRound_MobRoundTick.go`)

The MobRoundTick handler runs every round and processes per-mob updates including
condition triggers, stat/skill progression, pack scaling, and mutation acquisition.

### Pack Scaling (before per-mob loop)
```go
// TickPackSurvival returns []PackBonus — data structs to avoid import cycle
// with the rooms package. The hook handles room messaging and world events.
if b.PackScalingEnabled {
    for _, bonus := range mobs.TickPackSurvival() {
        // Emit room message: "The <group> pack moves with renewed coordination."
        // Emit WorldEvent{Type: PackStrengthened}
        // Significance: first bonus → Local, reaching max → Regional
    }
}
```

### Mob Mutation Acquisition (inside per-mob loop)
After condition triggers and before `Validate()`:
```go
// Guard: MobMutationEnabled && mob.Character.Aggro != nil
// Progress: += MutationProgressGainPerRound * MobMutationRate
// Threshold: MutationBaseProgress * MutationProgressScale^mutationLoad
// On acquire/deepen:
//   - Room flavor text
//   - EmitWorldEvent(MobMutationGained/Advanced)
//   - Significance based on mutation rarity (>=8 Global, >=5 Regional, else Local)
//   - Deepening to level 3 bumps significance one tier
```

### Per-Mob Loop Order
1. Condition trigger checks
2. Stat/skill progression (`MobProgressionEnabled`)
3. **Mutation acquisition** (`MobMutationEnabled`)
4. `Character.Validate()`

## The damaging condition tick (both round ticks, conditions unification 2026-09-12)

Since the ten combat conditions became records, every dot in the game ticks in
ONE place. The eleven records that carry a `tick_pool` and a NEGATIVE tick are
39 Venom, 40 Spore Toxin, 78 Toxic Cloud, 89 Throttled (the one STAMINA dot),
94 Cold Discharge, 96 Hull Discharge, 97 Arc Trap, 106 Searing Backlash, 115
Rending Bleed, and 121 Poisoned / 122 Bleeding, whose sign rides on the
magnitude their producer passes (`tick_from_magnitude`) rather than on an
authored `tick_percent`. The same branch runs the seven healing tick records
(5, 6, 7, 32, 33, 47, 50), so read it as "every tick record", not "every dot".
List them with `grep -l tick_pool _datafiles/world/dogmud/conditions/*.yaml` rather
than trusting this enumeration.

That one place is the `tick_pool` branch of the condition trigger loop
in `NewRound_UserRoundTick.go` and its mirror `tickMobConditions` in
`NewRound_MobRoundTick.go`. There is no longer a separate condition tick
(`TickConditions`, deleted) and no separate poison and bleed block in
`NewRound_AutoHeal.go` (deleted). This moves poison and bleed harm EARLIER in
the round: the round ticks run before `DoCombat`, AutoHeal ran after it.

**Both ticks fill a zero tick amount through `fillZeroTickAmount`**
(`condition_tick_amount.go`, drink path unification 2026-09-28; narrowed by
tick amount at apply, 2026-09-28). Before this only the player tick filled
it, so every event-applied heal-over-time and damage-over-time on a MOB was
inert; the mob round tick now heals and damages over time like the player
tick.

A `tick_pool` condition applied through the event queue (spells, potions,
area and mutator conditions, hazard-room dots) no longer arrives at the tick
with `TickAmount` 0: `Condition_ApplyConditions` computes it immediately
after the record lands, via `setTickAmountAtApply(targetChar, conditionInfo,
evt.ConditionId, evt.TickScale)` — `tickPoolMax(c, pool)` (the holder's max
for `"health"`/`"stamina"`/`"conviction"`, extracted so both functions share
it) times the spec's `TickPercent`/`TickVariance`/`TickMin`, at the
applier's `TickScale` (0 means 1.0; a spell passes `spellTickScale(caster)`,
everything else passes none). A refresh recomputes it too, so a recast
rescales an existing record rather than leaving its first cast's amount
stuck. `fillZeroTickAmount` is now the fallback for a `tick_pool` condition
added SYNCHRONOUSLY, bypassing `Condition_ApplyConditions` entirely — e.g.
`Character.AddConditionMagnitude` for a former combat condition — where
`TickAmount` genuinely can still be 0 at tick time; it computes the amount
from the holder's pool at scaling 1.0, caches it with `SetTickAmount`, and
returns it; read the RETURNED value, not `condition.TickAmount`.

Three things the tick path does at the moment health harm lands, all of which
the old poison hook did and the condition tick path did NOT:

- `cancelCraftOrSalvageOnDamage(char)` and `cancelDamageConditions(char)`
  (`combat_shared_helpers.go`), so a damaging tick now wakes a sleeper and
  cancels a `cancel-on-damage` record.
- `tickCauseFor(spec)` (`tick_cause.go`) returns "poison" for a `poison`-flagged
  record and "bleeding out" for a `bleeding`-flagged one, and the tick stamps
  it on `Character.LastTickCause` with `LastTickCauseRound`. `deathCauseFor`
  (`Death_PlayerAnnouncement.go`) reads the held 121 or 122 record by id first
  and falls back to the stamp within one round, which is how a kill by the last
  tick of a record survives both the already-expired instance and `PruneConditions`.
- The per-trigger flavour line is skipped when the trigger also expires the
  record (`PruneConditions` narrates the end instead), but the harm itself is NOT:
  the player path used to gate the whole body on `!condition.Expired()` and silently
  dropped the only tick of a one-trigger record.

A stacking record (122 Bleeding) reaches this branch once per round with
`TickAmount` already set to the sum of its live stacks, so every item above
happens once however many stacks are live; see "Stacking records" in
`internal/conditions/context.md`.

**Prone recovery runs after the condition tick on both sides.** `tickMobConditions` runs
before `tickMobProneRecovery`, and since slice 1b (owner ruling 2026-09-14)
`UserRoundTick` calls `AttemptRecovery` after `Conditions.Trigger` too. A failed or
gated attempt adds the one-tick 118 Recovering record, which must still be live
when `DoCombat` reads `attacks_cap`; before slice 1b the player's own tick
expired it first. The player attempt is skipped when
`Character.Health <= 0` or `Character.DeathQueued`, because a lethal bleed or
poison tick just above may have queued the death, and a dying player must not
scramble to their feet.

`NewRound_AutoHeal.go` keeps only the REGEN half, and it reads the same
vocabulary rather than an enum. **The whole hook is behind
`if evt.RoundNumber%3 != 0 { return }`, so regen lands every THIRD round**, not
every round, and that gate is also why the poison and bleed harm that used to
live here landed every third round while the duration it counted down ran every
round. Both records tick every round since slice 1b. Four branches read `regen_mult`, and the two kinds do NOT read it the
same way:

- **Out of combat** (player and mob) there is base regen regardless, and the
  record is a multiplier ON it: the branch computes `healthRegen` /`hpRegen`
  first, already scaled by toxicity, the mutation regen multipliers and the
  room mutator multiplier, then applies
  `if regenMult := Conditions.Effect(conditions.EffectRegenMult); regenMult > 1.0`. The
  `> 1.0` test is what stands in for "is a record held", because `Effect` is a
  product with identity 1.0.
- **In combat** (player and mob) there is no base regen at all, so the whole
  branch is gated on `Conditions.HasEffect(conditions.EffectRegenMult)` and the heal is
  `HealthPerRound()` times the multiplier.

The player's "Your wounds knit closed." line is gated on `HasEffect` in BOTH
player branches, so an out-of-combat player regenerating without a record heals
silently. Record 120 Regenerating is what a heal spell or a corpse feed
applies, and its end is narrated by `PruneConditions` like any other record's.

---

## Mob AI and Behavior

### Idle Mob Processing
```go
func IdleMobs(e events.Event) events.ListenerReturn {
    evt := e.(events.NewRound)

    for _, mobInstanceId := range mobs.GetAllMobInstanceIds() {
        mob := mobs.GetInstance(mobInstanceId)
        if mob == nil || mob.Character.IsAggro() {
            continue
        }

        // Check activity level for idle behavior
        if util.Rand(100) < mob.ActivityLevel {
            events.AddToQueue(events.MobIdle{
                MobInstanceId: mobInstanceId,
            })
        }
    }

    return events.Continue
}
```

**Stranded folds (#242).** A mob's fold step (`handleMobFoldCasting`) runs
only in the combat round, so a mob released mid-fold would hold its spell
forever, unspoken. `IdleMobs` ends it through `endStrandedFold` /
`fizzleMobFold` ("<caster>'s spell fizzles." by sight, the sputter by
sound): before the release when a player target walked out, and, for any
mob out of combat still holding a HARMFUL fold, every round, which covers a
mob target that walked or fled out (`actions.ClearRoomAggroOnDeparture`
releases the caster) and every other release. A help fold out of combat
(an idle `cast conviction-ward`) is not swept; that it never resolves is a
separate, pre-existing problem.

```go
func HandleIdleMobs(e events.Event) events.ListenerReturn {
    evt := e.(events.MobIdle)

    mob := mobs.GetInstance(evt.MobInstanceId)
    if mob == nil {
        return events.Continue
    }

    // --- Crafter tick (fires on restock cycle, not every idle tick) ---
    // TickMobCraft returns a CraftResult only when a craft is attempted.
    // The hook handles room messaging and world event emission to avoid
    // import cycles in the mobs package.
    craftRoom := rooms.LoadRoom(mob.Character.RoomId) // loaded once
    if result := mobs.TickMobCraft(mob, combat.SightRoom(craftRoom)); result != nil {
        // Emit room flavor text (success/failure)
        // Emit MobCraftedRare world event if SkillMinimum >= CrafterRareThreshold
    }

    // Execute idle command (runs alongside crafting)
    idleCommand := mob.GetIdleCommand()
    if idleCommand != "" {
        mob.Command(idleCommand)
    }

    return events.Continue
}
```

### Schedule executor (chunk 3.2)

- `NewRound_IdleMobs_schedule.go`: schedule executor branch inserted
  between the conversation guard and path-walker in `HandleIdleMobs`.
  On every tick: resolves the current segment via `mobs.CurrentSegment`,
  swaps `mob.IdleCommands` on segment transition, queues a `pathto`
  toward the segment `target_room`, falls back to `pathto home` after
  `ScheduleMaxPathRetries` consecutive failed path attempts.
- `MobIdle_HandleIdleMobs`: `TickMobCraft` now respects the schedule
  `activity:` gate — it returns nil immediately when the mob's current
  segment `activity` is not `"craft"`.

### Patrol executor (chunk 3.4)

- `NewRound_IdleMobs_patrol.go`: `patrolTickPlan` (pure decision)
  + `applyPatrolPlan` (side effects). Runs in IdleMobs AFTER the
  schedule branch, so a schedule-stamped `active_patrol_id`
  (from an `activity: patrol` segment) is visible. Reads-and-
  clears the stamp; falls back to `mob.PatrolId` for standalone
  patrols.
- `NewRound_IdleMobs_schedule.go`: stamps `active_patrol_id`
  MiscData in `applySchedulePlan` when the current segment has
  `activity: patrol`.

### Movement parity 4b: a tired mob waits on its path

`advanceMobPath(mob) bool` (`NewRound_IdleMobs.go`) is the path walker,
extracted out of `IdleMobs`'s per-mob loop body; it returns true when the mob
is busy with its path this round (a step issued, a re-path queued, or a
tired wait) so the caller does not also queue an idle command. Before this
slice a mob's step was free and unconditional; now `quoteMobPathStep(mob)
(mobs.PathRoom, actions.MoveCharge, bool)` peeks the mob's next step
(`mob.Path.Peek()`, which does NOT advance the queue) and quotes it with
`actions.QuoteMobStep` before `advanceMobPath` issues it. Three outcomes:

- **Affordable** (`quote.OK()`): `mob.Path.Next()` advances the queue and the
  step is issued exactly as before.
- **Not affordable now, but could be once rested** (`!quote.OK() &&
  !quote.Never`): the mob WAITS. The path is left untouched (nothing is
  dequeued) and `advanceMobPath` re-quotes the same step next round. This
  replaced the old "not standing in `Current().RoomId()` re-paths through
  remaining waypoints" behaviour for this case (fact V21 predates 4b): a
  mob that simply cannot pay is not off its path, so it must not re-path.
- **Never affordable, even rested** (`quote.Never`, `MoveCharge.Never`):
  `mob.Path.Clear()` and, if home, `WanderCount` resets. A cleared path
  hands the mob back to the idle command, schedule and patrol fallbacks
  instead of parking it on an impossible step forever.

`mobPathStepWaiting(mob) bool` re-runs `quoteMobPathStep` and reports the
middle case above (a step the walker would take that is affordable-later,
not-affordable-now). The patrol executor's `applyPatrolPlan` (`WantsPath`
branch, `NewRound_IdleMobs_patrol.go`) calls it before incrementing
`patrol_path_fail_count`: a mob resting mid-path has not failed to path, and
counting the rest would trip the `MaxPathRetries` home fallback on any
sufficiently long rest. The schedule executor's own path-progress counting
(`NewRound_IdleMobs_schedule.go`) is unaffected — only a *new* `pathto` issue
counts there (fact V22), which a waiting mob never triggers.

### Conversation executor (chunk 3.6)

- `NewRound_IdleMobs_conversations.go`: conversation branch runs in
  the idle-mob per-tick handler AFTER schedule and patrol branches.
  Calls `conversations.TryStart(mob, roomMobIds)` to attempt starting
  a new conversation; if successful, skips idle command dispatch for
  this round. Per-round during an active conversation, calls
  `conversations.TickConversation(mob, partnerId)` to advance one line,
  returning control when the exchange finishes. Gating: both NPCs must
  be fully idle (no combat, no sleep, no existing conversation, off
  cooldown) and have an active relationship edge.
- Player-arrival boost: `internal/usercommands/go.go` calls
  `conversations.TryStart(character, room.GetMobIds())` when a player
  enters a room, applying `ConversationPlayerArrivalBoostPct` chance to
  trigger a conversation between cohabiting NPCs. This adds ambient life
  to busy rooms without burdening the continuous idle tick.

## Integration Patterns

### Event System Integration
```go
// All hooks integrate with the event system
- events.RegisterListener()        // Register event handlers
- events.AddToQueue()             // Queue new events from handlers
- events.Continue/Cancel          // Control event processing flow
```

### Cross-System Communication
```go
// Hooks coordinate between systems
- users.GetByUserId()             // User management integration
- rooms.LoadRoom()                // Room system integration
- mobs.GetInstance()              // Mob system integration
- combat.AttackPlayerVsMob()      // Combat system integration
```

## Usage Examples

> **These examples are illustrative, not compilable.** They use placeholder
> event and helper names to show the *shape* of a listener. For a real
> registration, read `RegisterListeners()` in `hooks.go` — that is the single
> function where every listener in this package is wired up, and it is the
> authoritative list of what the engine actually listens to.

### Listener registration

Listeners are not registered individually from scattered files. `hooks.go`
exposes exactly one entry point:

```go
func RegisterListeners()
```

`main.go` calls it once at start-up. Adding a hook means adding a listener
registration inside that function and a handler file alongside it — there is no
`RegisterCustomHook`, and modules register their own listeners through the
plugin API instead (see `internal/plugins`).

The handler shape is:

```text
func <HandlerName>(e events.Event) events.ListenerReturn {
    evt, ok := e.(events.<EventType>)
    if !ok {
        return events.Continue
    }
    // ... work ...
    return events.Continue
}
```

(Illustrative shape, not a real symbol. Real handlers in this package follow
it: see `wireCombatPhaseVetoes`, `wireInboundAggroCleanup`.)

**Returning the wrong `ListenerReturn` swallows the event** for every listener
behind you. `events.Continue` is almost always what you want.

### Event Processing Flow
```go
// Example of event flow through hooks
// 1. Player attacks mob
events.AddToQueue(events.Combat{
    AttackerId: userId,
    TargetId:   mobInstanceId,
})

// 2. Combat hook processes attack
func DoCombat(e events.Event) events.ListenerReturn {
    // Resolve combat
    result := combat.AttackPlayerVsMob(user, mob)
    
    // Check for death
    if mob.Character.Health <= 0 {
        events.AddToQueue(events.MobDeath{
            MobInstanceId: mobInstanceId,
            KillerId:      userId,
        })
    }
    
    return events.Continue
}
```

## Combat State Machine Integration (chunk 0)

Four files in the hooks package wire the Combat Phase machine into the
engine without creating import cycles (the characters package cannot
import hooks; hooks import characters and register via `OnCharacterCreated`).

### CombatPhase_Vetoes.go

Registers the seven veto callbacks on every new `Character` via
`characters.OnCharacterCreated(wireCombatPhaseVetoes)`.

Each veto reads the current character field for its concern. Future
chunks replace each closure body as the corresponding machine lands
(e.g., `RegisterLifeCheck` will read `c.LifeMachine.State() == Alive`
once the Life machine ships in chunk 2).

| Veto registration | Reads |
|-------------------|-------|
| `RegisterCombatantVeto` | `c.IsCombatant()` |
| `RegisterActivityCheck` | `c.IsActing()` (negated) — queries Activity machine |
| `RegisterLifeCheck` | `c.Health > 0` |
| `RegisterPositionCheck` | `c.IsStanding()` (Position FSM, chunk 4b R5) |
| `RegisterTargetCombatantCheck` | target's `IsCombatant()` via users/mobs lookup |
| `RegisterTargetLifeCheck` | target's `Health > 0` via users/mobs lookup |
| `RegisterTargetPresenceCheck` | player grace condition (`NoAggroTarget`) check |

### CombatPhase_BtreeEvents.go

Registers an `AfterTransition` cascade that fires btree transition events
whenever a mob's Combat Phase state changes. Player characters also have
`CombatPhase` but the btree system only fires for mob instances.

Events fired (once per state transition, not per round):
- `mob_engaging` — `Idle → Engaging`
- `mob_engaged` — `Engaging → Engaged` (after `RoundsUntil` countdown)
- `mob_disengaging` — `Engaged → Disengaging` (flee initiated). Before flee
  parity slice 4a a mob's flee never went through `CombatPhase` at all (it
  resolved instantly in the old `mobcommands.Flee`), so this event never
  actually fired for a mob; `actions.BeginFlee` now drives every mob flee
  through the same `TransitionToDisengaging` a player uses, so it fires for
  real.
- `mob_combat_ended` — any → `Idle` (target died, flee succeeded, etc.)

Tick events (`mob_combat_round`, `mob_idle`) fire from the round driver
via `DispatchTickEvent`, not from this file.

Mob ownership is resolved via `findMobOwningCharacter`, an O(N) scan
over all mob instances that compares `Character` pointer identity. This
is acceptable because transition events fire at most once per state
change (not per round).

### CombatPhase_FleeCancellation.go

Registers an `AfterTransition` callback (`wireFleeCancellationMessage`, hook
label `flee_terminal_cancellation`) on every character, player or mob. When an
admitted flee moves from `Disengaging` to `Idle` for a terminal reason other
than flee success or the character's own death, the callback retracts the
one-use flee admission via `Character.CancelFleeAdmission`. It then looks up a
player by the character (`users.GetByUserId` and a pointer-identity check) and
sends the terminal line only when one is found, so a mob's cancellation is
silent. This covers target-death and combat-cleanup paths that remove the
character from the next combat round before `handlePlayerFlee` /
`handleMobFlee` can resolve the attempt. Before slice 4a this only ever
touched players (the label was `player_flee_terminal_cancellation`); a mob's
flee did not use the admission handoff at all.

### CombatPhase_CompanionAssist.go

Registers `SubscribeAttackersChange` on every character. When a charmed
companion's inbound attacker list grows (new attacker recorded), the
handler reactively directs the companion's owner and sibling companions
to join the fight — without waiting for the next round tick.

Behavioral parity with the old polling path in `NewRound_DoCombat`:
- Same `AutoAssist` flag check on the companion entry
- Same `NoAggroTarget` grace-period guard on the owner
- Sibling companions in the same room are also assisted

An owner who sees nothing in their room is not sent to `attack @<id>` /
`attack #<id>` (#454): the sight gate would refuse the id form and tell them
"You don't see them here." every round. This path and
`handleCompanionOwnerAssist` (`NewRound_DoCombat_helpers.go`) both skip such
an owner through `memberSeesSomething`, the check party auto-assist uses.

The polling `CompanionAutoTarget` in `combat_retarget.go` remains as a
fallback. Duplicate attack commands are benign (second attempt is vetoed
by the already-fighting state).

### combat_retarget.go

Contains three functions moved from the deleted `aggro_helpers.go` in
chunk 0's sunset pass. Still consumed by `NewRound_DoCombat`.

- **`ValidateAggro(char)`** — checks if the character's `Aggro` target
  still exists and is alive in the same room; calls `EndAggro()` and
  returns false if stale.
- **`RetargetOrEnd(char, room, userId, mobInstanceId)`** — clears current
  aggro and scans the room for a new target already attacking the
  character (or the character's companions). Returns true if a new target
  was found and `SetAggro` was called.
- **`CompanionAutoTarget(mob, room)`** — polling fallback for companion
  auto-assist. Runs once per round in `NewRound_DoCombat`. Directs idle
  companions to join the owner's fight or intercept mobs attacking the
  owner.

The "You turn your attention to X!" builder used to live here as
`retargetNotice`; it moved to `actions.RetargetNotice`
(`internal/actions/retarget_notice.go`) so `mobcommands.go`'s mob-departure
retarget could share it too, since nothing imports `hooks` but both `hooks`
and `mobcommands` import `actions`.

### Round driver dispatch (NewRound_DoCombat.go)

The round driver reads Combat Phase state instead of legacy `Aggro`:

- `c.IsInCombat()` replaces `c.Aggro != nil` in the "who is fighting?"
  loop.
- `c.CombatPhase.OnRoundTick()` advances `Engaging` → `Engaged` when
  `RoundsUntil` hits zero.
- `c.CombatPhase.DispatchTickEvent()` fires `mob_combat_round` or
  `mob_idle` btree events per character per round.
- **`handleMobFlee(mob *mobs.Mob, room *rooms.Room) bool`**
  (`NewRound_DoCombat_helpers.go`, flee parity slice 4a) is the mob twin of
  `handlePlayerFlee`: both call the shared `actions.ResolveFlee` and only
  render their own room lines. It runs in the mob in-combat block, after
  `handleMobFoldCasting` and the aggro-validate/retarget pass, at the same
  point `handlePlayerFlee` runs in the player pass; returning true skips
  combat for the mob this round. On an escape it calls the uncharged
  `actions.RelocateMob` (a player's flee pays no movement cost either) and
  fires the `mob_flee` behaviour event. Before this slice a mob's `Flee`
  resolved instantly in `mobcommands.Flee` with no round-later step at all.
  A mob still casting never reaches it, but `mobcommands.Flee` drops the
  cast when it begins the flee, as the player's command does.
- **`PackFlee`** (`MobDeath_PackFlee.go`) queues `flee` only on packmates for
  which `actions.FleeGate` returns `FleeOK`, and counts only those toward the
  scatter line. That covers the owner ruling that idle packmates stay put
  (`FleeRefuseNotInCombat`) and also skips a fighting packmate that cannot
  begin a flee (knocked down, grappled, rooted, frenzied), which would
  otherwise be announced scattering while it stayed.

### Verbosity gating (combat_verbosity.go)

Implements the player-configurable combat-text verbosity system (full /
medium / light). Touch-points live in `dispatchCritAndMessaging`
(`NewRound_DoCombat_unified.go`) and two round-end flushes called from
`NewRound_DoCombat.go`:

- **`dispatchCritAndMessaging`** — drains participant lines via
  `drainParticipantLines` (viewer's own level) and room lines via
  `drainSpectatorLines` (one step lower per spectator,
  `user.GetCombatVerbosity().OneStepLower()`). Medium suppresses
  dodge/parry/block lines; Light suppresses all individual hit lines.
  The floor rule (incoming hit-category lines always pass to the
  defender regardless of setting) is enforced here. Sight-gating is NOT
  uniform: participant tally recording (below) gates on
  `messaging.CanSeeSightImpairedOnly` (no sleep gate), while
  `recordSpectatorTallies` gates on `messaging.CanSeeClearly` (sleep-
  gated). Read the source at the call site before assuming either one;
  they answer different questions on purpose (see that function's
  comments).
- **`recordTallyFor` / `recordSpectatorTallies`** — when a viewer's
  effective verbosity is Light, the AttackResult's swing data is
  recorded into a per-viewer `combatTally` accumulator instead of
  being sent immediately.
- **`flushCombatTallies`** — called once at the end of `DoCombat` after
  all AttackResults for the round are processed. Renders and emits one
  compact summary line per fight pair per viewer.
- **`markBlindCombatant` / `flushBlindCombatNotices`** (M4d PR 2, Task
  4): a second, independent per-round accumulator
  (`roundBlindCombatants`, a `map[int]bool`) tracking players who
  fought this round (as attacker or defender, via
  `dispatchCritAndMessaging`) while their sight verdict was not
  `SightFull`, using the same `CanSeeSightImpairedOnly`-derived
  booleans (`srcCanSee`/`tgtCanSee`) that section already computes.
  Shapes-only viewers are included, not just fully blind ones: the
  notice keys on the sight VERDICT, while the score rides the sight ramp
  (lighting plan 5b, `internal/combat/context.md`), and only when the
  player's `messaging.SightMult` in the room is below 1.0 (lighting plan
  5c). A dazzled combatant with full sight gets no blind notice, by
  design: the glare notice below is theirs. Membership in this set is
  the "fought this round" signal; `roundTallies` cannot serve that role
  because it only contains Light-verbosity viewers who could ALSO see
  clearly (recording is skipped for a blind participant precisely to
  avoid leaking a named summary, see the `srcCanSee`/`tgtCanSee` gate
  comments). `flushBlindCombatNotices`, called once at the end of
  `DoCombat` beside `flushCombatTallies`, sends
  `messaging.CategoryCombatBlindWarning` once per blind combatant and
  clears the set. Not floor-protected: it goes through the viewer's
  ordinary `Verbosity.Suppresses` gate, and the category is in
  `suppressibleAtLight` only (`internal/messaging/verbosity.go`, owner
  ruling): it passes at Full and Medium and is suppressed at Light.
- **`markGlareCombatant` / `flushGlareCombatNotices`** (#319): the blind
  notice's twin for glare. A dazzled fighter is `SightFull`, so the blind
  notice never speaks for them, yet glare lowers their `SightMult` all the
  same. `markGlareCombatant`, called beside `markBlindCombatant` in
  `dispatchCritAndMessaging` with the same verdicts, records a player who
  sees clearly while `messaging.ComfortDistance`'s bright fraction is above
  0 and `SightMult` is below 1.0 (`roundGlareCombatants`).
  `flushGlareCombatNotices`, called right after `flushBlindCombatNotices` at
  the end of `DoCombat`, sends `glareCombatNoticeText` ("The glare is too
  bright, so your attacks and defense are weaker.") on the same category and
  verbosity gate, but ONCE PER FIGHT: `glareToldThisFight` remembers who was
  told, and the flush forgets every player no longer
  `Character.IsInCombat()`, so a later fight tells them again. A notice
  suppressed at Light is not counted as told.

### Attacker progression firing (U10b-1 Task 10)

`processAttackerProgression` (`NewRound_DoCombat_helpers.go`) fires the round's
ordinary attacker awards: ONE per weapon that SWUNG, routed through
`Character.AwardResolved` with a single `CandidateFor` candidate. Full weight on
a clean hit, `Balance.ProgressionFailureFraction` otherwise. It replaced an
`if !wh.CleanHit { continue }` gate, so a weapon whose swings were all deflected
or missed used to train nothing at all.

Two consequences worth knowing before touching it:

- **Awards per weapon per round went from `P(clean hit)` (measured 0.3856) to
  1.0**, roughly 2.6x. Any retune of `skills.SkillProgressionMultipliers` has to
  be fitted against 1.0, not against a hit rate.
- **The count of awards is the count of hand slots that swung.**
  `collectAttackWeapons` contributes a fist per empty hand slot, so bare hands
  take two unarmed-combat awards, a one-handed wielder takes one weapon-combat
  plus one unarmed-combat from the empty offhand, and a two-handed wielder takes
  one. `AttackResult.WeaponHits` is therefore never empty in production and no
  round-level fallback belongs beside the loop.

### Surprise-attack skullduggery progression (U10d)

`NewRound_DoCombat_unified.go`'s per-round progression pass awards a SECOND,
separate `progression.Outcome` (skill `skullduggery`, stat left empty) when
`res.WasSurpriseAttack && res.CleanHit` — outside the per-weapon `WeaponHits`
loop, on purpose, so a multi-swing ambush round pays the ambush bonus once,
not once per landed swing. It keys on `AttackResult.WasSurpriseAttack`
(`internal/combat/attackresult.go`), not on `Aggro.Type`: `calculateCombat`
demotes `SurpriseAttack` to `DefaultAttack` the moment it arms the opening
strike (see `internal/combat/context.md` "U10d — the opening strike"), so by
the time this progression pass runs, `Aggro.Type` always reads
`DefaultAttack` and a condition written against it would never fire.

## Awareness State Machine Integration (chunk 1)

Four files in the hooks package wire the Awareness machine into the
engine without creating import cycles (the characters package cannot
import hooks; hooks import characters and register via `OnCharacterCreated`).

### Awareness_Vetoes.go

Registers the activity check and detection-roll veto callbacks on every new
`Character` via `characters.OnCharacterCreated(wireAwarenessVetoes)`.

Each veto reads the current character field for its concern.

| Veto registration | Reads |
|-------------------|-------|
| `RegisterActivityCheck` | `c.IsActing()` (negated) — queries Activity machine |
| `RegisterDetectionCheck` | validates sneak attempt is proceeding (scaffold) |

### Awareness_Cascades.go

Registers an `AfterTransition` callback on the Awareness machine. When
the machine transitions away from or into the `Hidden` state, the hook
applies or removes condition #9 to keep the visible effect synchronized with
the invisible state.

A shroud hide (#444) is the exception: when the hide's data says
`awareness.HideShroud` (stored by `ResolveConcealmentAs` before the
transition), the Hidden cascade adds no record 9, so the hide is Hidden plus
record 31 and its end is told once, by 31's end line. `light_spell.go`'s
`shroudSpellApplication` stamps the caster's `characters.ShroudScore` on
record 31 as its magnitude, and `ApplyConditions` tells no start room line
for a hidden-flag record landing on a holder already hidden.

Also registers an `AfterTransition` callback on the Combat Phase machine
that reveals a hidden character on `Idle → Engaging`. A surprise attack is
**not** exempt: stealth breaks the instant the ambusher engages. The
ambusher keeps their opening strike anyway, because that bonus reads
`Character.Aggro.Type` (in `combat.calculateCombat`, well after this
cascade has run), not `IsHidden()`.
`internal/hooks/surprise_reveal_test.go` pins both halves — the reveal
fires, and it does not clear `Aggro.Type`.

`SetAggro` writes `c.Aggro` *before* it dual-writes the Combat Phase
transition. That order matters for anything observing `Idle → Engaging`
— notably the `mob_engaging` btree event in
`CombatPhase_BtreeEvents.go`, whose actions can read `Aggro`. It is
**not** currently covered by a test, and reversing it would not break
the opening strike, which reads `Aggro.Type` later in the round.

Events and cascades (per state transition, not per round):
- Awareness `Visible → Hidden`: apply condition #9 (not for a shroud hide) + room text "sneaks away"
- Awareness `Hidden → Visible`: remove condition #9 + room text "emerges from hiding"; clear the `sneaking` misc key (every exit from Hidden, #444), so `usercommands.Go` does not move a visible player as a sneaker
- Combat Phase `Idle → Engaging`: trigger Awareness reveal cascade

### Awareness_LightChange.go

Scaffolding for future light-source re-roll mechanics. Registers a
`OnCharacterCreated` callback to set up the listener registration hooks
for light-state-change events. Today a no-op pending full light-system
design; the file exists to document the integration point for future
chapters. Since lighting plan 5a the room-change listener asks
`Character.EmitsLight()` (any light term right now, so a hooded or
trimmed-off source does not count) instead of the deleted `lightsource`
flag.

### Vision-scaled spells (`light_spell.go`, lighting plan 5a; generalised lighting plan 5c)

A spell condition whose spec declares one of `conditions.ScaledKinds`
(`light_strength`, `nightvision_strength`, `infra_reach`, and since lighting
plan 5d `darkness_strength`) as `magnitude` (via
`ConditionSpec.ScaledKind`) is applied at a value and duration scaled from the
CASTER's primary stat and Spellcasting skill:
`magnitudeSpellApplication(spellData, caster, conditionId) (magnitude float64,
triggers int, ok bool)` — renamed from `lightSpellApplication` in lighting
plan 5c, when it stopped being light-only. Its magnitude comes from
`conditions.SpellScaledMagnitude(kind, stat, skill)`, shared with the admin
`setcondition` command, which picks the scaled kind's own
base/stat-divisor/skill-divisor trio (`SpellStrength*` for light,
`NightVisionSpell*` for nightvision, `InfraSpell*` for infra reach,
`DarknessSpellStrength*` for darkness, all on `configs.Lighting`), caps an
infra-reach result at `Lighting.InfraReachCap` and a nightvision result at
`configs.LightWindowShiftCap` (`conditions.CapScaledMagnitude`). Its duration
comes from `conditions.SpellScaledTriggers(kind, stat, skill)` (lighting plan
5d): darkness reads its own `DarknessSpellDuration*` trio, the other three
kinds share `SpellDuration*` (triggers floored at 1).
`ok` is false for any other condition, which keeps its authored application.
`applySpellCondition(target, spellData, caster, conditionId)` is the one door
the one spell-condition applier, `applySpellConditionEffect`
(`spell_help_effects.go`, every pairing since parity slice 3b), calls: a magnitude-scaled light or sight
goes through `AddConditionMagnitude`; a `tick_pool` condition (a heal- or
damage-over-time) goes through `AddConditionTickScaled` at
`spellTickScale(caster)` (`spell_tick_scale.go`, tick amount at apply,
2026-09-28); anything else through `AddCondition`. All three sit on the small
`spellConditionTarget` interface a `*users.UserRecord` and a `*mobs.Mob` both
satisfy. The record then trims to its HOLDER's eyes, who may not be the
caster.

`spellTickScale(caster)` is the one caster formula for a spell tick's scale:
the caster's `Spellcasting` `combat.SkillMultiplier` times an equipped
weapon's `SpellDamageMultiplier`, adjusted for
`mutations.GearEffectivenessMultiplier`. It replaced three copies of the same
arithmetic that used to run AFTER `applySpellCondition` queued the event, one
per player/mob-caster/mob-self-cast site in `spell_resolution.go`, and poke
`Conditions.SetTickAmount` directly on a record that, on a first application,
did not exist yet — the "Compute tick snapshot" blocks, all now deleted
(`actions/drink.go` dropped its own matching snapshot the same way, at a flat
1.0 rather than `spellTickScale`, since a potion has no caster). The amount
is computed once `TickScale` reaches `hooks.setTickAmountAtApply` in
`Condition_ApplyConditions`, where the record is guaranteed to exist; see
"The damaging condition tick" below.

`sendConditionEndRoomText(r, snap, msg, names, skip...)`
(`NewTurn_PruneConditions.go`, #220) judges a light's or a darkness's end
line against the room as it was just before the record ended. The record
expires inside `Conditions.Trigger` on the round tick (or a player cancels
it), and its light stops counting at once, but the line goes out at the next
turn's prune; judged by the room then, a light's line met a room already
dark and a darkness's a room already lit. So both round ticks call
`keepEndLineSnapshots(c, room)` (`condition_end_snapshot.go`) just before
`Trigger`: for every held light or darkness record whose
`ExpiresOnNextTrigger(spec)` is true it keeps the room's snapshot for this
round (`Room.EndLineRoundSnapshot()`, one per room per round, taken before
any light or darkness there has run out, so two lights going out together
are judged against the same moment) with `rooms.KeepEndLineSnapshot`, keyed
by the record's pointer; `usercommands`
cancel keeps one with `rooms.KeepEndLineSnapshotsBeforeRemoval` before it
removes the record. The store lives in package `rooms` so both can reach
it. The prune takes it with `rooms.TakeEndLineSnapshot(rec, roomId)` and
sends with `Room.SendTextVisualToSnapshot`; with none kept (a record removed
by a path the store's doc comment lists), or when the holder has moved rooms
between the snapshot and the prune (the snapshot belongs to the room the
record ended in), the line is judged by the room as it is. The prune calls
`rooms.ClearEndLineSnapshots()` for whatever it did not take, so no entry
outlives the turn after it was kept.

`sendConditionStartRoomText` (`Condition_ApplyConditions.go`, lighting plan
5d, ruling D6 as amended by the owner on 2026-10-05) is its counterpart for
start lines: a darkness source's start line ("A pall of dark spores gathers
around X.") is judged against a snapshot of the room taken BEFORE the
condition is added (`darknessStartSnapshot`, a `rooms.VisualSnapshot` taken
only when `spec.IsDarknessSource()` and not a refresh) and sent with
`Room.SendTextVisualToSnapshot`. The record is already held when the line
goes out, so the observers it has just blinded still see it, and one who was
already blind in a dark room learns nothing. Every other start line is judged
by the room as it is. A darkness's end line needs nothing: the room is
lighter by then. `EquipBestFloorItem` (`mob_equip_best_floor_item.go`) takes
the same snapshot before every floor pickup it equips (#447), not only a
darkness.

### Logout_AwarenessCleanup.go

Registers an `OnPlayerDespawn` listener that calls `character.Awareness.ForceVisible()`
to ensure the awareness machine is reset on logout. Prevents stale awareness
state or leaks if a character is reused or respawned.

Before it forces the quitter visible, `onPlayerDespawnForAwareness` stores
under the user temp-data key `despawnUnseenByKey` the players in the room
who could not make them out (`playersNotPerceiving`, by
`Character.Perceives`). `HandleLeave` runs after it (`events.Last`) and its
`removeAndAnnounceDespawn` takes `Room.VisualSnapshot()` before
`RemovePlayer`, then sends the `player-despawn` line with
`SendTextVisualToSnapshot`, excluding those players (#456): the quitter is
seen going by their own light, and a hidden quitter is not named to a room
that never saw them.

The same rule holds for every other departure line in this package: take
`VisualSnapshot()` of the room being left before the mover goes, then send
with `SendTextVisualToSnapshot`. `resolveFoldRecall` ("folds through the
Veil and vanishes!", visual only: no sound exists for a fold),
`TransportCompanions` ("X follows Y.") and `PushCompanionsToRoom` ("X is
swept out of the room!", whose name is now identity-tagged and hidden at
shapes) all do. Arrival lines stay judged after the move.

## Attributed death routing (U5c)

`CharacterDied_RouteDeath.go` is the **single place a harm-driven death is
resolved**. `ApplyHarm` queues an `events.CharacterDied` at the harm site;
`RouteAttributedDeath` (registered in `hooks.go`) resolves it, outside the
damaging call stack so no mob instance despawns mid-loop.

It owns the prechecks `Die`'s doc used to delegate to callers:

- **`ReviveOnDeath`** — heal above zero, cancel the condition, no death, clear
  `DeathQueued`. Before U5c only the two suicide commands checked this, so the
  condition was inert on every combat and DoT death.
- **Already resolved** — clear `DeathQueued` and return, so a character is never
  left permanently unkillable.

Note this file is a **listener**, not a Life-machine observer. The `Death_*.go`
family wires through `characters.OnCharacterCreated` +
`c.Life.Inner().AfterTransition(...)`; this one is an ordinary event listener and
follows the `<Event>_<Action>.go` naming used by `Condition_ApplyConditions.go`.

### The five backstops, and the rule they all follow

Five inline death checks remain — `handleAffected` (players and mobs, the only
check covering players hit in combat), the mob sweep at the top of
`NewRound_DoCombat`, `NewRound_MobRoundTick`, `NewRound_AutoHeal`, and
`Condition_ApplyConditions`. All are **backstops** for paths that never call `ApplyHarm`,
and all gate on `shouldSweepReap`.

**They skip on `DeathQueued`, never on health.** A character reaped by a backstop
is dying but not queued. Skipping on health would skip the entire population
they exist for. Reaping a queued victim instead would, for a mob, lose the
killer; for a **player** it would run the whole death cascade twice, because
`Die` cascades back to `Alive` and its own guard cannot catch the second call.

Each logs when it fires. That log going quiet is the evidence every harm path
now routes through `ApplyHarm`; if it is noisy, it names the path that does not.

`NewRound_AutoHeal`'s early `continue` matters as much as its death call: a dying
player must skip regen either way, or they heal back above zero before the queued
death resolves and the kill is silently cancelled.

## Life Machine Cascade + Death/Respawn Observers (chunk 2)

Fourteen files in the hooks package wire the Life machine into the
engine without creating import cycles. Each file registers its
observer via `characters.OnCharacterCreated(wireXxx)` at `init()`
time. Player-only observers gate on `c.GetUserId() != 0`; mob-only
observers gate on `c.MobInstanceId != 0`.

### Life_Cascades.go

Cross-machine cleanup that fires on two Life transitions:

**Alive → Dead:**
- Forces Combat Phase to `Idle` (`ForceIdle`)
- Forces Awareness to `Visible` (`ForceVisible`)
- Transitions Activity machine to `Free` (via separate `activity_life_dead`
  observer in `Activity_Cascades.go` — see Activity Machine section below)
- (The legacy `CombatPosition` reset and `GrappleControllerId` clear
  that previously lived here were deleted in chunk 4b R4. The
  `position_life_dead` observer in `Position_Cascades.go` owns the
  Position FSM death cascade.)
- Cancels EVERY active condition, permanent ones included:
  `CancelConditionsWithFlag(conditions.All)` reaches `Conditions.HasFlag(All, true)`, which
  skips only records already `Expired()` and never consults `Permanent`. (The
  separate `c.Conditions = nil` clear that sat beside it was deleted with the
  combat condition enum on 2026-09-12; the former conditions are ordinary
  records and the condition cancel covers them.)
- Bumps `Character.LifeEpoch`, beside the condition cancel. That cancel only
  reaches HELD conditions. A condition still queued on `events.Condition` is stamped with the
  epoch it was aimed at, and `ApplyConditions` refuses one whose epoch no longer
  matches (or whose holder is not alive), with no add and no notice. The epoch
  is the test rather than `IsAlive` or `DeathQueued` because of flush order:
  the killing swing queues its `CharacterDied` before its on-hit condition, and
  `RouteAttributedDeath` cascades a player back to Alive with `DeathQueued`
  cleared before the condition flushes. A ReviveOnDeath save ends no life, so the
  blow's condition still lands on the revived character. Pinned by
  `condition_after_death_test.go` (playtest 7d0dad99c4709fc0: a Rending Bleed from
  the killing blow killed the respawned player a second time).

**Dead → Respawning:**
- First removes, silently, every record still expired-but-held, which is
  every record the death strip expired plus any that expired earlier that
  turn and was not pruned yet. The strip only expires records; left for the next `NewTurn_PruneConditions` pass, each one's
  end line reached the respawned player and their new room ("Your wounds stop
  bleeding." in the Mending Hut, playtest 7d0dad99c4709fc0). It waits for
  this transition rather than pruning beside the strip because
  `deathCauseFor` in the death announcement reads the held Bleeding and
  Poisoned records by id, and an Alive → Dead observer registered after this
  cascade would otherwise find them gone. It queues `ConditionsTriggered` so the
  client's conditions panel refreshes. A record that ran out or was
  cancelled and pruned before the death still narrates; anything still
  expired-but-held at respawn is removed silently. Pinned by
  `death_strip_end_lines_test.go`.
- Refills all resource pools to 5% of max
- Applies `NoAggroTarget` grace condition (#81)
- Clears live `PlayerDamage` map (snapshot already in `DeadData`)
- Queues `CharacterVitalsChanged` event

### Death observers

| File | Purpose |
|------|---------|
| `Death_PlayerCleanup.go` | Stat decay + skill rust penalties, KD tracking (death count), party death notifications |
| `Death_PlayerAnnouncement.go` | Room broadcast, global broadcast, `events.PlayerDeath` queue, worldevents PvE emit, weakened/darkness text, instance ejection |
| `Death_PlayerCorpse.go` | Player corpse creation in the death room |
| `Death_InboundAggroCleanup.go` | Clears mobs and companions that were targeting the dying actor; fires for both player and mob deaths |
| `Death_MobLoot.go` | Carried and equipped item drop, gold drop, dark-room sound cue, mob corpse creation |
| `Death_AlivenessSubstrate.go` | Fires `events.MobDeath`; downstream subscribers handle faction rep, opinion update, crime recording, knowledge propagation, bounty resolution |
| `Death_MobInstanceCleanup.go` | `DeleteMobInstance`, `DestroyInstance`, `CleanupMobSpawns`, `RemoveMob` |
| `Death_MobBroadcast.go` | Room "X has died" broadcast, Guide tempdata, worldevents `MobKilledByPlayer` |
| `Death_MobBehaviorTree.go` | Fires `mob_die` btree event with primary killer's `UserId` |
| `Death_MobKillCredit.go` | `EndAggro` on killers, `KD.AddMobKill`, `OnFirstMobKill`, party kill credit |
| `Death_MobCharmCleanup.go` | `TrackRecentDeath`, `RemoveCharm`, reverse-track player `TrackCharmed` |
| `MobDeath_TrackingCleanup.go` | Clears `tracking-mob` / `tracking-display-count` + condition 86, and drops any shadow on the dying mob through `actions.ClearShadow` (found with `actions.ShadowTargetOf`; no cooldown, no line), on every player and mob (chunk 2.8; parity slice 6) |

### Respawn observers

| File | Purpose |
|------|---------|
| `Respawn_PlayerTeleport.go` | `rooms.MoveToRoom` to `c.ResolveRespawnRoom()` destination; belt-and-suspenders `EndAggro` |
| `Respawn_PlayerAutoLook.go` | Fires `u.Command("look")` for room-render UX after respawn teleport |
| `PlayerDespawn_TrackingCleanup.go` | Clears `tracking-user` / `tracking-display-count` + condition 86, and drops any shadow on the departing user through `actions.ClearShadow` (no cooldown, no line), on every other player and every mob (chunk 2.8; parity slice 6) |

### Wiring pattern

All fourteen files follow the same registration pattern:

```go
func init() {
    characters.OnCharacterCreated(wireXxx)
}

func wireXxx(c *characters.Character) {
    c.Life.Inner().AfterTransition(func(from, to life.State,
        r state.TransitionReason) {
        if from != life.Alive || to != life.Dead {
            return
        }
        // ... observer logic, gated by c.GetUserId() != 0
        // or c.MobInstanceId != 0 as appropriate
    })
}
```

The `AfterTransition` callbacks on the `state.Machine[State]` inner
framework call all registered observers synchronously before
returning control to the caller. This means by the time
`c.Life.TransitionToDead(...)` returns, all death-cascade side
effects have already fired.

## Activity Machine Cascade + Observers (chunk 3)

One file in the hooks package wires the Activity machine into the engine
without creating import cycles (same pattern as chunks 0-2).

### Activity_Cascades.go

Registers one `AfterTransition` observer via
`characters.OnCharacterCreated(wireActivityCrossMachineCascades)`.

**`activity_life_dead` handler — Life `Alive → Dead` → Activity `→ Free`:**

When the Life machine transitions `Alive → Dead`, the handler calls
`c.Activity.TransitionToFree(TriggerDeath)` if any activity is in
flight. This repoints the chunk-2 pre-wire in `Life_Cascades.go` (which
niled `CastingState` and `CraftingState` directly) onto a proper
Activity-side observer. All three active states (Casting, Crafting,
Salvaging) transition to Free; there is no casting exemption for the
death cascade.

**Combat-entry cancellation — implemented via veto, not cascade:**

Crafting and Salvaging block the character from entering combat
(`Idle → Engaging`). This is implemented in `CombatPhase_Vetoes.go` —
`RegisterActivityCheck` returns `!c.IsCrafting() && !c.IsSalvaging()`,
so the veto fires only when one of those two activities is active.
Casting is exempt (cast IS a combat action — the character continues
casting through combat entry, with damage handled separately via the
concentration-break path). A separate `AfterTransition` cascade for
combat-entry was evaluated and removed as unreachable (the veto fires
before the transition succeeds for craft/salvage; nothing to cascade
for casting).

### Call-site wirings (not AfterTransition)

Movement and damage interrupts do not fit the machine-to-machine
`AfterTransition` pattern; they are wired directly at their call sites:

| Interrupt | Location | Trigger fired |
|-----------|----------|---------------|
| Movement (Crafting/Salvaging) | `internal/usercommands/go.go` | `TriggerMovementInterrupt` |
| Damage taken (Crafting/Salvaging) | `cancelCraftOrSalvageOnDamage` in `combat_shared_helpers.go` | `TriggerDamageInterrupt` |
| Damage taken (Casting) | `clearCastingActivity` in `combat_shared_helpers.go` | `TriggerConcentrationBreak` on roll failure |

Completion triggers are fired by per-tick consumers after a successful
`Advance*` call:

| Completion | Location | Trigger fired |
|------------|----------|---------------|
| Cast completes | `processFoldRound` in `NewRound_UserRoundTick.go` | `TriggerCastComplete` |
| Craft completes (player) | inline craft-tick block in `NewRound_UserRoundTick.go` | `TriggerCraftComplete` |
| Craft completes (mob) | inline craft-tick block in `NewRound_MobRoundTick.go` | `TriggerCraftComplete` |
| Salvage completes (player) | inline salvage-tick block in `NewRound_UserRoundTick.go` | `TriggerSalvageComplete` |
| Salvage completes (mob) | inline salvage-tick block in `NewRound_MobRoundTick.go` | `TriggerSalvageComplete` |

## Position Cascade + Observers (chunks 4a + 4b)

Four files in the hooks package wire the Position machine into the
engine (same import-cycle-free pattern as chunks 0-3). One file
scaffolded the cascade in 4a; three more landed in 4b with the
control-axis cutover.

### Position_Cascades.go (chunk 4a)

Registers one `AfterTransition` observer on the Life machine via
`characters.OnCharacterCreated(wirePositionCrossMachineCascades)`.

**`position_life_dead` handler — Life `Alive → Dead` → Position → Standing:**

When the Life machine transitions `Alive → Dead`, the handler calls
`c.Position.TransitionToStanding(TriggerDeath)` if the Position machine
is non-nil and not already `Standing`. This ensures that a character
who dies while grappled or knocked down returns to the `Standing` default.

This observer is now the sole Position reset on death. Chunk 4b R4
deleted the chunk-2 `Life_Cascades.go` pre-wire that previously reset
`c.CombatPosition = PositionStanding` and `c.GrappleControllerId = 0`
directly. Those legacy fields no longer exist (T21 sunset).

**Integration tests** in `Position_Cascades_test.go` cover four scenarios:
- PO-037: Standing at death → remains Standing (no-op observer path)
- PO-038: Mount at death → cascades to Standing
- PO-039: Guard at death → cascades to Standing
- PO-040: BackGround at death → cascades to Standing

### Position_GrappleTick.go (chunk 4b / 4b-fixup-2)

Per-round grapple observer registered via `events.RegisterListener`
(`processGrappleTick`) in the file's `init()`. Fires once per round.
Iterates all active players and mobs; for each grappling character,
resolves the partner and processes the pair exactly once per round
(deduplication via a `seen map[state.ActorRef]bool`).

**Chunk 4b-fixup-2 T8** replaced the old `IsController()`-filtered
single-side iteration with pair-aware deduplication. This fixed the
symmetric-position regression (Clinch / HalfGuard / Turtle) where
both sides returned `IsController() == false` and the drift roll
never fired.

For each pair, `processGrapplePairFromIteration` calls
`determineDriftAttacker` to pick the controller arg, then delegates
to `processGrapplePair`:

**`determineDriftAttacker`** picks the drift-roll attacker-arg. Priority:
1. Whoever has the more controller-leaning `Control` state
   (`control.Controlling` < `Neutral` < `Controlled`).
2. Tiebreaker: whoever has `GrappleData.IsAggressor == true`.
3. Final fallback: iteration-order (lhs).

### Score formula (2026-05-19 rework)

Each side's per-round score is computed by
`grappleScore(c, isAggressor, cfg, includeSkill)`:

```
score = (0.7·Str + 0.3·Dex + skill_coef·UnarmedCombat)
        × stamina_multiplier × encumbrance_multiplier
```

`processGrapplePairWithContest` then multiplies each side's score by that
side's own `messaging.SightMult` in the pair's room (lighting plan 5b), after
`grappleScore` and before the contest runner, so the injected runner in tests
sees the post-ramp scores.

where `skill_coef = 2.2` for the aggressor (the side that initiated
the grapple via `grapple` command or btree `grapple` primitive) and
`2.0` for the defender. Symmetric in shape — no role-based unilateral
stat bonus. Position bias is already captured by `ControlLevel` state
initialization (chunk 4b-fixup-2); the formula doesn't double-encode it.

The aggressor flag (`GrappleData.IsAggressor`) is set once by
`ApplyGrappleResult.markAggressor` at grapple entry and persists for
the grapple's lifetime, regardless of any later reversals.

Body-armor `EscapeModifier` is NOT read by the formula. The field
remains on `ItemSpec` for backward compatibility and possible future
re-purposing (sub eligibility, armor resistance, etc.). The legacy
`escapeModifierFromBody` helper was deleted in the 2026-05-19 rework.

The grapple-skill is `UnarmedCombat` per its own definition
(`internal/skills/skills.go:29`: "Fist/body attacks & defense,
grappling"). Earlier versions of this formula read `WeaponCombat`
by mistake; that bug auto-escaped every grapple for any unarmed-
trained player.

`includeSkill` is the maintenance-admission gate. A participant who cannot
fully pay that round keeps the Strength/Dexterity base plus the existing
stamina-depletion and grapple-encumbrance effectiveness multipliers, but loses
only the Unarmed Combat term. The other participant's skill term is independent.

See `docs/superpowers/specs/completed/2026-05-19-grapple-drift-formula-rework-design.md`
for the design rationale and sample z-score table.

For each pair inside `processGrapplePair`:

1. **Per-round maintenance admission** — Each participant independently
   multiplies `GrappleStaminaCostPerRound` by the controller or controlled role
   multiplier, then quotes `costs.ActionGrappleMaintain` against Stamina and
   commits with `characters.CostPartial`. This composes the shared physical
   encumbrance and inverse-Unarmed cost multipliers, preserves fractional carry,
   and floors the pool at zero. Both commits happen before the contest, including
   on the round that resolves an escape. A short player receives one private
   grapple-flow line; the partner, observers, and NPC participants do not.
   Before either quote, `processGrapplePairWithContest` rejects pointer-aliased
   participants. A corrupted self-linked symmetric grapple is force-broken
   through the existing solo consistency path, with no pool, carry, warning, or
   drift-contest mutation.

   This is intentionally not a flat upkeep subtraction: role adjusts the base
   before the shared physical load and inverse-Unarmed calculation, and each
   participant owns a separate quote and fractional carry update.

2. **Opposed control roll** — Score values computed for both sides via
   the formula above, with `grappleStaminaMultiplier` and the encumbrance
   multiplier already baked in and each participant's own admission deciding
   whether Unarmed Combat is included. Produces a signed ZScore representing
   the controller's margin.

   The roll itself is not made here. Since U3 this package makes no
   contest of its own: it calls `combat.RunContest` for the grapple
   drift, the spell sites, and riposte-trip and auto-bash via
   `combat.ExecuteSkillMove`. Charm no longer appears in that list:
   U10c deleted its per-tick re-roll ladder outright and moved the
   cast onto the (spell, social) contest the seam already ran. It imports
   `internal/contest` for the `Entry` type only and must never call that
   package's `Run`, `AgainstDifficulty` or `RunWithFloors`. The private
   floor accessors this package used to keep, `maneuverHitFloor` /
   `maneuverResistFloor` / `spellHitFloor` / `spellResistFloor`, were
   deleted in U3 because they were a second copy of the same config keys
   and were invisible to a grep for the exported accessors. Do not
   reintroduce them; U6 reduced all eight of those knobs to a single
   `Balance.ContestFloor`, read only in
   `internal/combat/run_contest.go`.

   The signed z here is `res.Margin / (res.AttackRoll.StdDev * math.Sqrt2)`.
   U6b Task 14 landed the sqrt(2) correction the U3 no-op deliberately
   deferred: the pre-fix code divided by `StdDev` alone, inflating every
   drift z by about 41%. Floor-forced rounds keep the ±1 sentinel margin,
   which normalises to ~0 and lands in the Hold band by design — the long
   comment at the site in `Position_GrappleTick.go` explains why that is
   kept and must not be "fixed".

3. **Outcome resolution via `position.ResolveOutcome`** — Passes the
   controller, signed ZScore, and defender's posture to the resolver,
   which returns an `Outcome` struct describing the kind
   (Advance / Degrade / Reversal / Escape / Hold) and target position
   if applicable.

4. **Transition application** — Dispatches the outcome:
   - `OutcomeAdvance` / `OutcomeDegrade`: calls `applyAdvanceOrEscape`
     to transition to the target position.
   - `OutcomeReversal`: calls `applyReversal` to swap controller and
     controlled roles and transition to the reversed position.
   - `OutcomeEscape`: calls `applyAdvanceOrEscape(newTarget=Standing)`
     to break the grapple.
   - `OutcomeHold`: no transition; advances the round in-place.
   - Resets per-grapple cooldown maps on escape (when breaking to
     Standing).

5. **ControlLevel shift (`applyControlShift`)** — After the position
   outcome is applied, `applyControlShift(controller, controlled, z)`
   updates both sides' `Character.Control` FSM state based on the
   z-score magnitude:
   - `|z| < 0.5`: no shift
   - `0.5 ≤ |z| < 1.5`: 1 stable-state step
   - `|z| ≥ 1.5`: 2 stable-state steps
   Winner shifts toward Controlling; loser shifts toward Controlled.
   Each step fires the boundary-cross callback when crossing
   LosingControl or BecomingControlled.

6. **Outcome messaging** — Calls `emitOutcomeMessages` to dispatch
   outcome-specific template messages (Advance / Degrade / Reversal /
   Escape) and `emitHoldFlavor` + `emitStrikingApexFlavor` on Hold
   rounds.

**Boundary-cross callback (chunk 4b-fixup-2 T13):**

Registered at `init()` via `control.RegisterBoundaryCrossCallback`.
When `applyControlShift` drives a side's ControlLevel through a
boundary (LosingControl or BecomingControlled), the callback fires
`emitGradientMessage(self, transient, from, to)`. This:
- Resolves the gradient key from the transient state + direction
  (`gradientKeyForCrossing`).
- Looks up the `GradientTriad` in `grappleOutcomesLib.Gradients`.
- Dispatches Self / Partner / Observers messages with name
  substitution and per-grapple cooldown (preventing repeated messages
  for the same gradient within one fight).

**Grapple Messaging Library (`LoadGrappleMessaging`, `loadGrappleLib`):**
`main.go` calls the exported `LoadGrappleMessaging` at boot. It reads
`<configured world>/messaging/grapple_outcomes.yaml` through
`grapplemessaging.LoadFromDataFiles`, runs `ValidateCompleteness`, and PANICS on
either failing: grapple is event narration, and the two-tier loader policy in
`internal/narration/context.md` puts event stores in the fail-the-boot tier.
It does not consult `grappleLibOnce`, it spends it, so the check runs at boot
even when a test in this package reached the store first. `loadGrappleLib` is
the lazy `sync.Once` path that remains for exactly that case, and it keeps its
log-and-degrade behaviour because it can run before a logger exists.
Organizes templates into six maps by outcome kind:
`Advancements`, `Degradations`, `Reversals`, `Escapes`, `Holds`, and
`StrikingApex`. Per-grapple cooldown tracking via
`Character.PerGrappleMessageCooldowns` (map of `bool` for per-outcome
deduplication) and `Character.PerGrappleMessageCooldownsLastRound`
(map of `uint64` for sparse hold-flavor every ~4 rounds).

**Outcome-driven messaging (`emitOutcomeMessages`):**
Selects template triad (Controller / Controlled / Observers variants)
by outcome kind and position name. Each speaker variant picks from a
pool via `grapplemessaging.PickTemplate` with per-grapple cooldown
to ensure variety. On Hold, defers to `emitHoldFlavor` and
`emitStrikingApexFlavor` for special handling.

**Hold-round flavor (`emitHoldFlavor`):**
Fires sparse flavor text every ~4 rounds (configurable via
`holdEmitEveryRounds`). Uses a "hold_last_round" key in the
last-round map to track emission timing per position state.

**Mount-strike flavor (`emitStrikingApexFlavor`):**
Fires only on Hold rounds at Mount position. Adds a single-speaker
message visible to the controller; observers see the combat damage
text from the combat system.

### Position_Messaging.go (chunk 4b)

Per-round support observer subscribed to the NewRound walker via
`processGrappleTick`. After grapple control drift and transitions are
resolved, fires supplementary messaging:

- **`fireStaminaWarningIfLow`** — One-shot "you're getting gassed" beat
  when stamina drops below `GrappleStaminaLowThreshold` (config,
  default 0.25). `IsLowGrappleStamina()` is the predicate. Reuses the
  per-grapple cooldown map for deduplication.

- **Submission messaging callbacks** — `fireSubmissionOpeningMessage` and
  `fireSubmissionResolutionMessage` are registered as hooks with the
  combat package to fire outcome-specific templates when submissions are
  attempted or resolved. Templates loaded from
  `<configured world>/messaging/position_control.yaml`, which M4b-1 moved under
  the world tree from `_datafiles/messages/`. `main.go` calls the exported
  `LoadPositionMessages` at boot, which reads and parses the file and PANICS on
  either failing (event tier); `loadPositionMessages` is the lazy `sync.Once`
  path kept for tests, and the boot loader spends that Once rather than
  consulting it. Opening messages vary by submission type (armlock,
  choke, etc.); resolution messages vary by outcome (Mercy / Subdue /
  Cripple / Lethal).

- **Role keys**: every audience in `position_control.yaml` is keyed `actor`,
  `actee` or `observer` as of messaging M4b-1, which collapsed the store's
  three authored key vocabularies: `attacker`/`target`/`room` on
  `submissionMsgTriple`, `self`/`room` on `positionMessageTemplates`'s
  `StaminaWarning`, and `controller`/`controlled` on the gradient and
  transition side keys. The Go field names keep the old spellings; only the
  yaml tags moved. Two things to know before editing the file: the mirror of
  `submissionMsgTriple` in the repo root's `shipped_narration_data_guard_test.go`
  decodes with `KnownFields(true)` and must be renamed in lockstep, and the
  gradient state named `controlled` is authored data rather than a role, so it
  kept its spelling while the side key beside it did not.

- **Tokens** — `position_control.yaml` is on the canonical vocabulary as of
  messaging M4a: `{actor}` and `{actee}` for the two grapplers, plus the
  store's own `{position}`, `{old_position}` and `{new_position}`. Rendering
  goes through `narration.Substitute`; the local `substitute` this file used
  to carry is gone, and `token_engine_guard_test.go` at the repo root fails the
  build if a replacement grows back. `substitutionsForCharacter` puts the
  controller in the
  actor slot; `staminaWarningSubstitutions` overrides that with the character
  the warning fires for, because the stamina room line is about the reader
  rather than about the controller. The store's golden is
  `internal/narration/testdata/stores/position_control.golden`.

**Names in the dark (M4d PR 3).** `sendCharacterMsg` (self + room dispatch
for `fireStaminaWarningIfLow`) and `sendSubmissionTriple` (attempter +
recipient + room dispatch for the submission opening/resolution messages)
both now go through `messaging.SendTrio` instead of `UserRecord.SendText` +
`Room.SendTextVisual`. Before this, the stamina warning's room line named
the warned character with a bare `Character.Name` — no ansi identity tag —
so a shapes-only room observer read the real name straight through; only a
tag-wrapped name is caught by `messaging.Anonymize`. `sendCharacterMsg`
keeps `c` (the character the warning is ABOUT, per
`staminaWarningSubstitutions`'s asymmetric mapping above, not necessarily the
controller) as `ActorName`, unchanged.

`sendSubmissionTriple` was a deeper leak: `position_control.yaml`'s
submission templates name the OTHER grappler inside the actor and actee
lines too, not only the observer line (e.g. `submission.opening.armbar`'s
actor line reads "You isolate {actee}'s arm..."). Those personal lines rode
`UserRecord.SendText` — the audio channel, which has no sight gate and no
anonymize stage at all — so a grappler in the dark read the other
grappler's real name in every submission line, not just the room echo. Both
personal lines now go through `SendTrio`'s `Actor`/`Actee` roles, which each
hide the OTHER party's name from that line's own reader by that reader's own
sight; the room line hides both names from each observer the same way the
gradient room broadcasts already did. `attempter` is `Actor`, `recipient` is
`Actee`, matching the `{actor}`/`{actee}` substitution both submission
callers already build.

Cooldowns reset when the grapple ends (any `TransitionToStanding` via
escape, break, or death).

### Position_SubmissionTick.go (chunk 4d)

Per-round submission-attempt observer registered via
`events.RegisterListener(events.NewRound{}, processSubmissionTick)`
in its own `init()`. Runs AFTER `Position_GrappleTick.go` because the
two files sort alphabetically — `SubmissionTick` > `GrappleTick` — so
registration order within the package guarantees the drift snapshot is
fresh when this observer reads it.

**Per-round flow for each active character:**

1. Skip non-controllers (pair is processed once from the controller side).
2. Resolve the grapple partner via `resolvePartner(c)`.
3. Call `EvaluateSubAttempt(controller, controlled)`, which reads
   `c.LastDriftRoll` (the `DriftRollSnapshot` written by
   `Position_GrappleTick` this same round):
   - Top window: `MarginAttacker > SubmissionAttemptAlpha` AND
     `IsTopSubEligible(posState, controlLevel)`.
   - Bottom window: `(-MarginAttacker) > SubmissionAttemptAlpha` OR
     `DefenderZScore >= SubmissionAttemptCritZ`, AND
     `IsBottomSubEligible`.
   - Returns the eligible `Role` (RoleTop / RoleBottom) and a bool.
4. If eligible, pick a sub type via `pickSubmissionRoundRobin` (advances
   `c.LastSubmissionAttempted` index, cycling through the position's
   pool).
5. Call `combat.RollSubmissionAttempt(attempter, recipient, subType,
   combat.SightRoom(rooms.LoadRoom(attempter.RoomId)))` (the room feeds both
   sides' sight ramp, lighting plan 5b).
6. Call `combat.ResolveSubmissionOutcome(attempter, recipient, result, role)`,
   then hand the `combat.SubmissionOutcomeEffects` it returns to
   `narrateSubmissionEffects` (in `Position_Messaging.go`): the hook narrates
   Stunned (condition 84) and Broken Limb (condition 83) to a player victim right after
   the outcome, because the combat package sends no player text and both conditions
   are applied synchronously and flagged `silent-start`.

The `LastDriftRoll.Round` field is compared against the current round
counter to reject stale snapshots (e.g., character just logged in).

**Cross-references:**
- `Character.LastDriftRoll` — see `internal/characters/context.md`
  "Chunk-4d submission fields".
- Roll formula and tier table — see `internal/combat/context.md`
  "Submission System".
- Position eligibility predicates — see `internal/state/position/context.md`
  "Submissions".

**Death_PlayerAnnouncement gate (T8):** When a death is triggered via
`life.TriggerSubmission`, `Death_PlayerAnnouncement.go` skips the
standard "you have been slain" global broadcast and instead emits a
submission-specific room message. The gate reads `d.NoDeprogression`
from `DeadData` to distinguish subdue/cripple (quiet, local) from
lethal (global announce). Source: `internal/hooks/Death_PlayerAnnouncement.go`.

### Chunk 4e sub-interrupt hook

**Position_SubmissionTick.go** reads `Character.SubInterruptDamageThisRound`
(accumulated by combat's `chunk4eAccumulateSubInterruptDamage` hook) before
resolving sub outcomes. If > 0, forces the tier to `SubTierBad` regardless
of the roll result. Both submitter and partner accumulators are reset to 0
at the end of each per-pair tick via defer.

### Chunk 4f (U10: opposed contest, not chance-based)

**`processFoldRound`** in `combat_shared_helpers.go` replaced the earlier
deterministic 100% break gates (Prone/Supine/Grapple from chunks 4a-4e), then
U10 replaced the chunk-4f chance-based curve with an opposed contest:

1. Reads `position.PositionDisruptionDmgEquiv(posState, ctrlState)` from
   `internal/state/position/disruption.go`. Returns 0 for `Standing`
   (check skipped entirely).
2. Multiplies the damage%-equivalent by 10 and runs
   `combat.RunConcentrationContest(hold, dmgPctEquiv*10)`
   (`internal/combat/run_concentration_contest.go`): the caster's hold,
   `concentrationScore(char)` (`Wil + spellcasting×SkillWeight`) times the
   caster's `messaging.SightMult` in their room (lighting plan 5b; applied at
   both concentration call sites, not inside `concentrationScore`), against
   that difficulty, floored only by
   `Balance.ConcentrationFloor` (0.02).
3. On a WON contest, one success-only `ApplyProgression` spellcasting event
   fires and fold accumulation continues normally.
4. On a lost contest, calls `clearCastingActivity(TriggerConcentrationBreak)`
   and returns `ProneBroke: true` or `GrappleBroke: true` (caller routes
   messages).

**Layered disruption:** The damage-path `checkConcentrationBreak` and the
throttle-path (`ExecuteThrottle`, `internal/actions/combat_throttle.go`) can
each independently break the same cast in the same round — all three routes
share the one `RunConcentrationContest` entry point. The Activity machine
does not have a concept of "already broke this round" — two breaks in one
round simply fire `TransitionToFree` twice; the second call is a no-op from
the `Free` state.

**Target gone, mob caster (#242):** `fizzleMobFold(mob, room, cs)`
(`NewRound_DoCombat_helpers.go`) is the one ending for a mob fold whose target
is gone: it clears the cast, records the concentration failure and calls
`sendMobSpellFailed(mob, room, "fizzles")` (seen, or heard as
`messaging.SoundSpellSputtersOut`). Three paths reach it: the fold step's
`TargetGone` (a dead or logged-out target), `IdleMobs` releasing a mob whose
target walked out mid-fold (the fold ends before the release; outside combat
no fold step would run again), and `resolveMobSpell` completing with no target
left in the room.

Cross-references:
- `internal/state/position/disruption.go` — per-position dmg%-equivalent table
- `internal/state/position/context.md` — chunk 4f status + Guard inversion note
- `combat.RunConcentrationContest` — the shared contest entry point
  (`internal/combat/run_concentration_contest.go`)

### Reach pipeline integration (chunk 4c)

No new hook files for 4c. The reach penalty is applied inline within
the existing per-swing helpers in `NewRound_DoCombat_helpers.go`:

- **Reach-adjusted damage:** `buildWeaponSetup` calls
  `combat.CalcReachAdjustedItemMult(weapon, attacker)` instead of
  reading `weaponSpec.DamageMultiplier` directly. Long weapons take a
  multiplicative penalty in grapple positions.
- **Bludgeon narration:** `buildAttackMessages` calls
  `combat.ShouldBludgeon(reach, radius)` before `items.GetAttackMessage`.
  When true, bladed weapon subtypes (Slashing, Cleaving, Stabbing,
  Shooting) swap to `Bludgeoning` vocabulary so fiction tracks math.
  Natural-blunt and caster subtypes are exempt.

### Position_ConsistencyCheck.go (chunk 4b)

Periodic invariant checker registered via
`wirePositionConsistencyCheck`. Walks character pairs and calls
`position.ValidateGrapplePair(a, b)` to verify:

- If `a.IsGrappling()` and references `b` via `GrappleData.Partner`,
  then `b.IsGrappling()` and references `a` symmetrically.
- Pair role relationship is consistent (one controller, one controlled,
  or mutual neutral).
- No orphan grapples (character in a grapple state with no Partner
  except Turtle).

Logs WARN on any invariant violation. Cheap to run (small pair
universe in any one room); intended as a safety net during 4b's
parallel-write window.

## Presence Machine Observers (chunk 5)

Four files wire the Presence machine into the engine (same
import-cycle-free `OnCharacterCreated` pattern as chunks 0-4).

### NewRound_PresenceTick.go

Registered as a `NewRound` listener. Fires timeout-driven Presence
transitions for every active player and mob each round.

- **Players:** reads `roundNow - lastInputRound`; fires `Active→Idle`,
  `Idle→AFK`, or `AFK→Disconnected` when the round delta exceeds the
  corresponding config threshold.
- **Mobs:** reads `roundNow - lastTargetFoundRound`; fires
  `Active→Dormant` (gated by essential-mob veto) or
  `Dormant→Despawning` (same veto) when thresholds are exceeded.
- **Spawning mobs:** transitions `Spawning→Active` on the first tick
  after creation.

**Ordering:**
- Runs AFTER `NewRound_DoCombat`: attacks that landed this round
  transition `Dormant→Active` (T7 wake-on-attack) before PresenceTick
  evaluates timeouts, so the now-Active mob is not immediately bounced
  back to Dormant.
- Runs BEFORE `NewRound_AutoHeal`: ensures freshly-woken Active mobs
  are eligible for heal-tick logic in the same round.
- Runs BEFORE `NewRound_IdleMobs`: mobs that PresenceTick transitions to
  `Despawning` get their terminal-tick removal in the same round without
  needing an extra tick.

### Essential-mob vetoes (in `internal/mobs/mobs.go` `Validate()`)

NOT a hook file — registered inline in mob `Validate()` immediately
after `NewMobPresence()`. Three vetoes (Active→Dormant, Active→Despawning,
Dormant→Despawning) all share one policy closure:

- Returns `VetoError` when `!mob.Despawns() || mob.IsEssential() ||
  mob.Character.IsCharmed()`. Shopkeepers, foragers and caravan crew are
  permanently Active. A charmed creature is Active only WHILE charmed --
  since U10c bonds expire, so the veto lapses with the bond and the
  ex-companion becomes eligible again like any other mob.

### RoomChange_PresencePlayerEntry.go

Registered as a `RoomChange` event listener. Fires only on player
entries (`evt.UserId != 0`). When a player enters a room, any mob in
that room whose Presence is `Dormant` is transitioned `Dormant→Active`
with `TriggerPlayerEntry` (also resets `LastDormantEntryRound = 0`).
This is the T11 room-entry wake path; T7 (auto-wake on attack) is wired
inline in `internal/combat/combat.go`'s `AttackPlayerVsMob` /
`AttackMobVsMob`.

### RoomChange_StolenBaubleRecognition.go

Registered as a `RoomChange` event listener (docs/baubles Phase 6c). On
any move into a room it routes to `actions.RecognizeStolenBaubles`: a
player's move (`evt.UserId`) looks only at that player's baubles, a mob's
move (`evt.MobInstanceId`) lets only that mob look. An NPC robbed of a
bauble that is still hot may recognise it on whoever carries it (a
contest; a catch through `thiefCaught`), once per theft. The call goes
through the `recognizeStolenBaubles` variable so tests can see the
routing; `TestStolenBaubleRecognitionIsRegistered` reads `hooks.go` to
guard the registration.

### RoomChange_ShadowFollow.go

Registered as a `RoomChange` event listener (player/mob parity slice 6); it
replaced a loop in `usercommands/go.go` and `MobRoomChange_ShadowFollow.go`,
which moved only player shadowers. `RoomChangeShadowFollow` runs two passes:

- **Follow.** `shadowExitTo` resolves the exit's map key from `Exits`, then
  `ExitsTemp`, then the active mutators' exits (not `Room.FindExitTo`, which
  returns a temporary exit's title). No exit (a teleport): nothing. Otherwise
  every player and mob in the old room whose `actions.ShadowTargetOf` names
  the mover is checked: no condition 87 means stale, `actions.ClearShadow`;
  not hidden, it stays; else it queues the exit through one `Command` call
  (users and mobs share the signature).
- **Arrival.** When the mover has a live shadow whose quarry is in the room
  it entered: not hidden, `actions.EndShadow` with "You've been spotted --
  your shadow ends."; still hidden, `actions.ShadowSenseRoll` in that room.

It reads `IsHidden()` live, never `RoomChange.Unseen`: listeners run after the
moving command returns, so entry detection has already run.
`TestRoomChangeShadowFollowIsRegistered` reads `hooks.go`, and
`shadow_follow_guard_test.go` keeps it the only follow site.

### Scheduler observer (in `validate.go` + `mobs.go`)

NOT a hook file — registered inline at each Presence-machine
construction site. The observer captures `*Character` by closure and
fires `c.CancelAllScheduled()` when `to == Disconnected` (player) or
`to == Despawning` (mob), wiping pending scheduled transitions across
every machine on this character.

### CombatPhase veto integration

`wireCombatPhaseVetoes` in `CombatPhase_Vetoes.go` populates
`RegisterTargetPresenceCheck` (the seventh veto) with a closure that
reads the target's `Presence.State()` and blocks for `Disconnected` and
`Despawning`. Idle/AFK/Dormant targets are explicitly NOT blocked.
See `internal/state/combatphase/context.md` for the full veto chain.

## Companion reservation and the U7b ceiling (2026-08-15)

Every path in this package that can raise a character's pool reservation now
consults `characters.WouldBreachReservationCap` first and refuses rather than
writing past the ceiling. There are five, and two of them never had any
affordability check at all before U7b.

### Login recompute (`companion_reserve_backfill.go`, `PlayerSpawn_HandleJoin.go`)

```go
func companionBaseReserveFor(mobId int) int
func refreshCompanionReserves(ch *characters.Character) bool
func refreshCompanionReservesOnLogin(user *users.UserRecord)
func companionRebaseNotice(ch *characters.Character) string
```

`refreshCompanionReserves` **replaced** `backfillCompanionReserves`, which only
stamped records that loaded as 0. It now recomputes **every** companion's
`ConvictionReserve` from what that mob id would be charged today, and returns
true if any moved. `ConvictionReserve` is deliberately frozen at summon time so
it cannot drift mid-life, which makes login the only place a rebase can reach a
returning veteran.

`refreshCompanionReservesOnLogin` wraps it and **tells the player** when the
recompute left them further past the ceiling than they were. That disclosure is
not politeness: companion reserve is priced partly off manifestation,
`GetSkillLevel` counts equipment stat mods, and `skill_manifestation` is in the
gold-scaled loot affix pool, so selling a `+manifestation` item makes every
companion dearer at the next login with nothing happening in between.

It **never dismisses a companion**, whatever the total comes to. Reservation is
refused on addition only.

### Auto-spawn gates

`spawnBroodFloor` (`manifester_companions.go`) and `spawnHomunculus`
(`chrysifier_homunculus.go`) both gate on the ceiling **before** creating the
mob. Neither had any check before: they wrote a reservation into a pool that
might have had no room for it, every round, forever. Both return nil on
refusal, which their callers already handle by backing off ten rounds. The
homunculus refusal is **spoken**, because that path is the one most likely to
bite a crafter and a silent failure would read as the apex being broken.
`HomunculusConvictionReserve` dropped from 1000 to **300** for the same reason.

### Cast gates

`resolveCompanionSummon` (`companion_summon.go`) and `applyMobEffect_charm`
(`charm_spell.go`) replaced the deleted `CanAffordCompanion` with two separate
refusals: the companion **count** cap and the reservation **ceiling**, reported
separately so a player at their companion limit is not wrongly told they lack
conviction. Summon reserve is derived from the spell's
`SummonPetMultiplier` through `characters.CompanionReserveBase`; charm passes 0,
meaning unscaled, so charm's price did not move.

U10c renamed the charm arm from `resolveCharmSpell` to `applyMobEffect_charm`
and moved it into the effect dispatcher (now `applySpellEffect`), because the old function ran a
second private `RunContest` on top of the one the cast had already run and
discarded -- one cast resolved twice and the player saw both narrations. The
flat reserve is deliberate (spec 3.8): a sewer rat and an Elemental King tie up
the same conviction, because charm's price is the DANGER, not the invoice.

### Enchant tier-up and craft completion (`NewRound_UserRoundTick.go`)

```go
const enchantTierUpBlockedCooldown = `enchant-tierup-blocked`

func enchantTierUpWouldBreach(ch *characters.Character, itm *items.Item) bool
func enchantApplyWouldBreach(ch *characters.Character, itm *items.Item, enchantType string) (characters.Pool, bool)
func tickChrysalisEnchantments(ch *characters.Character, randN func(int) int) []string
```

Tier-up is a **passive** breach with no action to refuse: it rolls every combat
round on every Chrysalis-enchanted equipped item and doubles the reserved
fraction at low tiers, so a character sitting just under the ceiling can cross
it mid-fight having done nothing. `tickChrysalisEnchantments` skips the advance
and says why, throttled by a cooldown because the roll retries every round. It
deliberately does **not** reset `EnchantUses`, so the item stays ready to
advance the moment its wearer makes room.

`tickChrysalisEnchantments` was extracted from an inline loop in
`UserRoundTick` and takes its roll source as a parameter (production passes
`util.Rand`) so the ceiling behaviour can be driven deterministically from a
test rather than waiting on a 2%-per-round die. It returns lines to send;
they all belong to `messaging.CategorySkillProgress`.

`enchantApplyWouldBreach` guards the enchanting **craft completion**.
`usercommands/craft.go` refuses before the work starts, but the rounds in
between are not free of change: a worn enchantment can tier up mid-craft and a
lapsing condition can shrink the pool the ceiling is measured against. Refusing here
still returns the materials. Subtracting what the target already reserves is
what makes re-enchanting work, since the old enchantment is replaced rather
than stacked.

## Mob Area Harm Targets

`resolveMobSpell` takes the targets of a mob's area harm spell from
`mobAreaHarmTargets` (`mob_area_harm.go`), read when the spell resolves:
every creature but the caster and non-combatants, every person, and for a
charmed caster not its owner or its owner's other companions. A caster
that `companionai.IsBondedCompanion` says is bonded also spares whatever its
owner (`GetCharmedUserId`) could not harm: a creature `mobs.CheckPlayerHarm`
blocks, and a person `(*rooms.Room).CanPvp` refuses to the owner or who is
in the owner's party. With the aicompanion module off nothing is bonded and
nothing changes.

## Spell Duration System

`calcSpellDuration` lives here, in `spell_resolution.go`, not in
`internal/spells/`; an earlier draft of this doc got that wrong.

```go
func calcSpellDuration(baseFolds int, spellcastingSkill int, willpower int) int
```

Formula: `baseFolds × (10 + willpower/20 + spellcastingSkill/2)`, rounded
(`math.Round`) and floored at 10. `baseFolds < 1` is treated as unset and
defaults to 4 before the multiply, so a spell YAML that omits `base_folds`
still gets a sane duration rather than a zero one.

There are three call sites, one per effect, since parity slice 3, and each
reads the caster through `spellCasterStatAndSkill` (the spell's primarystat
through `CasterStatValue`, and the school's cast skill):

- **Shield: full duration, no divisor.** `applySpellShield`
  (`spell_help_effects.go`) passes `calcSpellDuration(...)` unmodified to
  `AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, ...)` as the trigger
  count (record 119 ticks once a round, so triggers and rounds coincide).
- **Heal: `/2`, floored at 6.** `applySpellHeal` (`spell_help_effects.go`)
  computes `calcSpellDuration(...) / 2`, then clamps `durationRounds < 6` up
  to 6, before
  `AddConditionMagnitude(conditions.ConditionIdRegenerating, durationRounds, regenMult, ...)`.
- **DoT: `/3`, floored at 3.** `applySpellDot` (`spell_effects.go`) computes
  `calcSpellDuration(...) / 3`, then clamps `dotDuration < 3` up to 3, and
  passes that rounds figure straight to
  `AddConditionMagnitude(conditions.ConditionIdPoisoned, dotDuration, ...)`: record 121 ticks
  every round (slice 1b; it was every third round before). See
  `internal/conditions/context.md` under "Cadence".

**Crit never touches a duration, and never touches a help spell.** A
harmful spell's crit shows in its damage and its `[CRIT!]` tag. Heal and
shield used to carry a player-only crit bump (x2 above 1x regen, x1.5
shield) that no cast could reach, because a help spell never enters the
contest, the only source of a crit; parity slice 3b deleted both (owner
ruling, 2026-09-28).

## Spell effects (`spell_effects.go`, `spell_help_effects.go`, parity slices 3a and 3b)

Every spell effect on one target goes through one `spellEffectCtx` and one
dispatcher, `applySpellEffect`, whoever casts it and whoever it hits: player
on mob, player on player, mob on itself, mob on mob, mob on player. The four
contested resolvers in `spell_resolution.go` (`resolveAgainstMob`,
`resolveAgainstPlayer`, `resolveMobSpellAgainstMob`,
`resolveMobSpellAgainstPlayer`) keep their names and their one
`runSpellChannelAttack` call each, then build a context per target: the
caster's `*characters.Character`, caster and target as `actions.Actor`
(`*actions.UserActor` or `*actions.MobActor`; only tests pass a nil caster),
the room, the spell, the magnitude and the contest result. Refs come from the
actor (`casterRef`, `targetRef`), not the character.

The harmful effects have one applier each: `applySpellDamage`,
`applySpellDot`, `applySpellKnockdown`. Each starts the fight through
`commitHarmfulSpellAggro`: the target turns on the caster if it was not
already fighting, the caster on the target likewise, and a player caster on a
mob calls `actions.SeedAggression` with freshness judged per target, as
`throw` does, which records the assault crime on a fresh engagement (owner
ruling, 2026-09-28). Damage and knockdown on a mob call `creditSpellDamage`
before the harm, as melee does with `TrackPlayerDamage`; the dot does not
(its ticks harm anonymously, a filed follow-up). A dot's duration reads the
spell's primarystat and the school's cast skill through
`spellCasterStatAndSkill`, not `actions.GetSpellStatAndSkill`, which is the
fold stat.

The helpful effects have one applier each in `spell_help_effects.go`:
`applySpellConditionEffect` (every named condition through
`applySpellCondition`'s event door), `applySpellHeal` (a Regenerating
record), `applySpellShield` (a Minor Shield record) and `applySpellPurge`
(cancels every poison). `applySpellDefaultEffect` serves an effect with no
applier of its own, and charm binds a mob through `applyMobEffect_charm`.
A defended status narrates the defence triad and applies nothing
(`spellStatusDefended`); a harmful condition or default spell still starts
the fight through `commitHarmfulSpellAggro`. When the caster is its own
target (`selfCast`), the caster reads its own line and the room reads the
caster named once (`selfCastAudience`). Heal and shield read the caster
through `spellCasterStatAndSkill` and never crit: a help spell never enters
the contest, the only source of a crit (owner ruling, 2026-09-28). A player
healing a mob queues `events.Healed` for the AI companion. Every applier
narrates through `messaging.SendTrio` (players in the username tag, mobs
through `mobDisplayName`), so a mob's spell on a mob reaches the room and a
reader in the dark reads "something". `mobDisplayName` and the player branch
of `spellDefenceIdentity` are narration names (#453, #454 review): the
identity tag with its duplicate index and suffix colour, never an adjective
span, quest star or " and <pet>", so a recoil, spell or defence line never
reads "Skeleton (charmed)" or "Aliceia and Fang".

Every resolver takes a help spell (`attack_type: none`) through
`resolveHelpSpell` with `uncontestedSpellResult()`: no contest, no fumble,
no counter, one landed record. `resolveMobSpell`'s self branch (a mob on
itself) takes the same step and never applies a harmful spell to the
caster. A contested cast shares three steps: `applySpellBackfire` (every
caster kind is hurt, told, seen and recorded), `interruptSpellTarget` (a
configured boss-interrupt spell cancels any casting target through
`maybeInterruptSpellOnTarget`) and `recordSpellResolution`. `recordSpell`
is the analytics seam over `combat.RecordSpell`, swapped by tests the way
`runSpellChannelAttack` is.

`spellHelpAreaTargets` fills an area help spell's targets for both caster
kinds, replacing what the cast's initiation step put there. A player, or a
mob charmed by one, helps every player in the room and every mob charmed by
that player or by a member of that player's party (`actions.HelpCharmAlly`,
the rule single-target help in `actions.InitiateCast` also calls), so the
party's companions, the bonded AI
companion included, are healed and a stranger's pet is not. An uncharmed
mob helps itself and its `mobs.FindPackmatesInRoom` packmates, the rule its
behaviour tree's `cast_best_in_category` uses to pick whom to heal, and no
player.

`resolveMobDrainArea` keeps its own `actions.ExecuteDrainArea` contest; only
its lines use the context. `channel_defence_routing_test.go` parses
`spell_resolution.go`, `spell_effects.go` and `spell_help_effects.go` and
allows the contest seam only in the four resolvers.

## Counter tier wiring (U6b Task 10)

`counter_tier.go` hosts `fireSpellCounterTier`, called at all FOUR spell
quadrants in `spell_resolution.go` (`resolveAgainstMob`,
`resolveAgainstPlayer`, `resolveMobSpellAgainstMob`,
`resolveMobSpellAgainstPlayer`): a defensive crit against a cast fires
`combat.ExecuteCounter`, a free seam-routed counter-swing at the caster,
narrated from the pool of the defence that won (`items.CounterPoolFor`),
except a defy win (charm, the one social spell), which fires
`actions.FireCounterTaunt` instead of a swing, narrated from the counter-defy
pool. Both answers refuse a cast whose authored targeting is not single:
`combat.ExecuteCounter` and `actions.FireCounterTaunt` each carry the gate,
so an area cast earns no counter from either. Both directions are wired on
purpose; covering only the player-attacker direction would hand mobs a
counter immunity nobody decided. Spells are same-room by construction, so
the reach gate always passes here. Dispatching from these exits is
ordering-correct because the cast's own outcome has already been narrated
when they fire. `resolveMobDrainArea` dispatches no counters: a room-wide
drain is an area attack and earns none (counters slice, 2026-09-18).

A non-harm cast at a mob (`AttackType == combatvocab.AttackNone`: a heal on a
companion, an ally-mob buff) takes the uncontested shortcut at the top of
`resolveAgainstMob`/`resolveMobSpellAgainstMob` and returns before the seam
ever runs, so it can never reach the counter tier — a companion cannot
counter-swing the ally who just healed it (M4b-2).

Related, in `combat_shared_helpers.go`: melee riposte's damage fraction reads
`CounterDamagePercent` (shipped 0.5 — the old literal, behaviour unchanged),
and the block is skipped entirely at 0 because `CalcRawDamage` treats
`itemMult <= 0` as "unset" 0.30. Riposte stays UNCONTESTED (its historical
maths); only the cross-channel tier's swing runs through the seam. The
auto-trip/auto-bash `ExecuteSkillMove` calls carry `IsCounter`, which the
tier's wiring refuses — counters never recurse.

## Gotcha: prone auto-recovery became contested on 2026-08-30. It never was before.

`recoveryContest` (`recovery_contest.go`) builds the opposed prone-recovery roll
for `AttemptRecovery` by iterating `ch.Attackers()`, picking the strongest
same-room living holder, and returning a closure over `combat.RunContest`. It
returns `nil` -- documented in the function itself as *a free stand* -- when
nobody qualifies.

**For its whole life before U11, nobody ever qualified.** `Character.Attackers()`
reads the Combat Phase inbound-attacker list; that list is populated only
through `lookupMachine(d.Target)` in `TransitionToEngaging`, and `lookupMachine`
read a `machineRegistry` map that **no production code ever wrote to**. So the
loop body never executed, `best` stayed nil, and prone recovery was uncontested
for every character, always -- whatever U10 intended. Two consequences that
outlived the cause:

- The failure emotes (`NewRound_MobRoundTick.go:203`,
  `NewRound_UserRoundTick.go:255`, *"attempts to stand, but slips and falls in
  the chaos of battle"*) were unreachable strings.
- `AwardResolved` sits INSIDE the `if contestWin != nil` guard
  (`characters/skills.go`), so auto-recovery also never awarded unarmed-combat
  progression.

**Both are live now.** Do write code against `Character.Attackers()`.

### How it was fixed, and why not the obvious way

The registry map was **not** repaired -- it was deleted. A hand-maintained
`ActorRef -> Machine` map is a cache of a mapping `internal/users` and
`internal/mobs` already own, and adversarial review found five cache-coherence
bugs in a single afternoon: a throwaway record loaded for a password check
(`LoadUser`) evicting a live player's entry, `character new` leaving the retired
character bound forever, `character view`/`hire` replacing a registered mob's
Character wholesale, copyover restoring nobody, and an unsynchronised
read-modify-write between the connection goroutine and the round loop.

`combatphase.lookupMachine` now resolves on demand through an injected function
(`combatphase.SetMachineResolver`, wired in `machine_resolver.go` in THIS
package) that asks `users.GetByUserId` / `mobs.GetInstance`. There is no second
copy to drift, nothing to leak, and no teardown to forget. **If you add a new
way for an actor to enter the world, you need do nothing** -- provided it ends
up in `userManager` or `mobInstances`, which is what "in the world" means.

A second, independent break was fixed with it: `SetAggro` passed
`Actor: state.ActorRef{UserId: c.userId}` and nothing calls `SetUserId` on a
mob, so a mob's ref was the zero value and `RecordInboundAttacker` early-returned
on `ActorRef.IsZero()`. It now uses `c.ActorRef()`, which carries both id
fields. **Build actor refs with that method, never by hand.**

`actions/combat_fire.go`'s `shooterIsUnengaged` still uses a live room scan.
That is now deliberate rather than a workaround: it answers "is anything in this
room targeting the shooter", a wider question than "who has engaged me".

## Speech, emotes and the Deafened filter (sight gates 5b)

`Message_SendMessages.go` is the one place a queued `events.Message` becomes
delivered text; both of its branches gate on
`message.HiddenFromDeafened(user.Deafened)` (`internal/events`'s one
statement of the rule) before sending, rather than re-deriving it. Every
speech and emote room line now reaches this package through
`internal/actions`'s shared bodies (`Say`, `Shout`, `SendHeard`, `SendSeen`),
so nothing in `hooks` hides a speaker's name itself.

`justice_wiring.go`'s `init()` wires `justice.SetGuardSay` to a closure that
only calls `actions.Say(&actions.MobActor{...}, line)`: a guard's spoken
line goes through the same reveal, deafen split and name-hiding every other
NPC speaker gets, rather than a hand-rolled room broadcast.

## Dependencies

- `internal/events` - Event system for listener registration and event processing
- `internal/users` - User management for player-related hooks
- `internal/mobs` - NPC management for mob-related hooks
- `internal/combat` - Combat system for battle resolution
- `internal/quests` - Quest system for progression tracking
- `internal/rooms` - Room management for location-based events
- `internal/conditions` - Status effects for condition management
- `internal/configs` - Configuration management for system settings
- `internal/mutations` - Mutation system for mob mutation acquisition
- `internal/worldevents` - World event recording for emergent behavior milestones
- `internal/mudlog` - Logging system for debugging and monitoring
- `internal/state/combatphase` - Combat Phase state machine (chunk 0)
- `internal/state/awareness` - Awareness state machine (chunk 1)
- `internal/state/life` - Life state machine (chunk 2)
- `internal/state/position` - Position state machine (chunks 4a + 4b)
- `internal/state/control` - ControlLevel state machine (chunk 4b-fixup-2)
- `internal/state/presence` - Presence state machine (chunk 5)
## Files: one handler per file

139 non-test files (recounted for parity slice 6, which replaced
`MobRoomChange_ShadowFollow.go` with `RoomChange_ShadowFollow.go`). The
filename **is** the index. Each is named for the event it handles and the job
it does, so `NewRound_IdleMobs.go` is the idle-mob step of the new-round
event.

The prefix IS the event name, so there is no short list of them: 100 of the 139
files carry one and they spell 42 distinct events. The big ones are
`NewRound_*` (21 files), `Death_*` (11), `MobDeath_*` (7), `NewTurn_*`,
`Position_*` and `RoomChange_*` (5 each), `CombatPhase_*` (4), and
`Awareness_*` and `PlayerDespawn_*` (3 each); the other 33
prefixes carry one or two files apiece. Enumerate them with
`ls internal/hooks/*.go | sed 's/_.*//' | sort -u` rather than trusting a list
here. There is no `Input_*` or `Combat_*` prefix.

The remaining 39 files are shared helpers rather than handlers and carry no
prefix at all; they are the lowercase-named ones, for example
`combat_shared_helpers.go`, `spell_resolution.go`, `item_proc_dispatch.go`,
`machine_resolver.go`, `tick_cause.go` (the death-cause tag a damaging
health tick stamps; see "The damaging condition tick" above), and
`light_spell.go` (see "Vision-scaled spells" below). `hooks.go` is in that
set and is the odd one out: it is not a helper but the registration table.

Do not go looking for a registration in these files. **`hooks.go` holds
`RegisterListeners()`, and that one function wires every listener in the
package** — it is the authoritative list of what the engine reacts to.

Conventions:

- Combat logic belongs in `handleCombatRound`, not scattered across handlers.
- Behaviour-tree combat events fire **before** the legacy AI.
- A handler returns `events.Continue` unless it genuinely means to stop the
  event reaching later listeners.

## The item tick (`NewRound_ItemRoundTick.go`, lighting 5e)

`ItemRoundTick`, a `NewRound` listener, fires `item_idle` through
`behaviortree.TryItemBehavior` once a round for every item with a behaviour
tree that it reaches: every online player's worn slots then backpack; the
indexed mobs' worn slots then backpack (`items.MobHolders`); the indexed
rooms' floors (`items.RoomHolders`). Never a world walk. A visited holder
with nothing treed left drops out of the index; a room no longer loaded loses
its fixture outputs. Container contents are not visited. Last,
`behaviortree.EvictUnseenItemBTreeStates` drops the state of every item not
visited this round. `RegisterListeners` also sets `items.OnRoomHolderIndexed
= EvaluateRoomFixtures`, so a room's fixtures are lit the moment the room
joins the index (a fixture spawned, or a room loaded holding one).
`HandleJoin`'s companion gear restore re-indexes the companion
(`IndexTreedItems`). `EquipBestFloorItem` skips fixtures.

**Item voices (item behaviour slice 2).** The tick's `item_idle` speaks a
sentient item's ambient lines; the Pinnacle tick no longer has a voice
sub-tick (`tickVoices`, `pickVoiceEvent`, `tryEmitVoice`, `emitVoiceLine`,
`applyTauntPull` are deleted). Event lines fire from three sites, each
spelling its `EventType` literal so `behaviortree`'s vocabulary test sees
it: `ItemEquipEvents` (`EquipmentChange_ItemEvents.go`, an
`events.EquipmentChange` listener) fires `on_equip` into every treed item put
on, found in its slot by UUID, and `on_unequip` into every one taken off,
players and mobs alike; `MobDeathItemProcs` fires `on_kill` into every worn
treed item of each player with damage on the kill (`fireWornItemEvent`);
`tickHunger` fires `on_hunger_feeding` into the weapon's tree when
`HungerFeedingLineCooldownRounds` allows a line (MiscData
`pinnacle_hunger_msg_next_round`) and sends the plain "The blade feeds on
you" fallback when no tree handles it. `pinnacle_voice_next_round` is inert
in old saves. `testdata/item_voice_parity.golden` is the Pinnacle voice
path's 200-round record, frozen before the move; `TestItemVoiceParity`
holds the tree path to it.

**Item procs (item behaviour slice 3).** `item_procs.go` is deleted
(`dispatchItemProcs`, `procGateOpen`, `markProcCooldown`,
`procBearingItems`, `readMiscRound`; the four effects moved to
`behaviortree/actions_item_proc.go`). `fireItemProc(event, owner, other,
room, damage)` (`item_proc_dispatch.go`) reads `ItemProcsEnabled` first,
picks the one item the event reaches (`on_hit` and `on_spell_hit` the
weapon, `on_block` the offhand, `on_grapple` the body) and runs its tree
with `EventContext.Proc`. Its six call sites spell their `EventType`
literal: `NewRound_DoCombat_unified.go` (on_hit, on_block),
`Position_GrappleTick.go` (on_grapple, both sides),
`spell_effects.go` (on_spell_hit, two). `MobDeathItemProcs` no longer
dispatches a separate on_kill proc: its one `on_kill` event carries kill
lines and procs to every worn treed item (ruling S4). The Pinnacle tick
reads MiscData rounds through `characters.MiscRound`. The proc cooldown
key `pinnacle_proc_cd_*` is inert in old saves.
`testdata/item_proc_parity.golden` is the retired path's record (four
items, five events, hits, misses, cooldown windows and a probe draw each
round); `TestItemProcParity` holds the tree path to it.
