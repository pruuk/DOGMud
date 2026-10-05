# PR #208 review: floor decay scavengers, unification lens

Blind reviewer `floor_decay_scavengers:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Reviewed the floor-decay and city-scavenger piece (f7310ed77..3ab2d1094) with a unification and reuse lens. The contributor reused a lot correctly:
- **Pickups:** every scavenger pickup goes through actions.TakeFloorItem, actions.GetGoldFromFloor and actions.TooDarkToGet, so the shared pickup and dark rules apply.
- **Bauble bookkeeping:** both removal paths call baubles.MarkVanished. The litter rule reuses Item.BaubleUntakenFor and BaubleBelongsTo.
- **Lines:** they are rendered through narration.Substitute (TokenActor) and sent through Room.SendTextVisual. The pipeline anonymizes mobname tags at shapes sight.
- **Pathing:** paths come from mapper.GetPath, and idle flavor goes through mob.GetIdleCommand plus npcidle.TryReplace.
- **Room pinning:** IsEssential was extended by group name, the same way foragers and caravans are pinned.
- **Removal pattern:** the fresh-slice removal copies removeUntakenBaubles. The scavenger daily reset emits ItemOwnership events.
- **Knobs:** they are declared in config.balance.go and surfaced in config.yaml.

Four divergences were found:
1. **Two floor-pickup paths run on the same mob.** The pre-existing idle-hook floor grab (hooks.EquipBestFloorItem) still runs on scavengers before scavenger_step, so it bypasses the new litter rule, the dark gate and the daily haul reset.
2. **A second, unguarded "player home" hook.** Floor decay protects player homes through a new single-slot setter wired in main.go. The existing rooms.IsPrivateRoom / SetPrivateRoomCheck mechanism already does that job and is what the bauble and goblin sweeps use.
3. **The new knobs cannot honour 0, and there is no off switch.** The project has an established convention for this (negative = off, absent = default) that was not used.
4. **The loot goblin was retired only by a config value.** Its hook, config struct, room scan, portal and admin paths all stay live.

## Coverage

I read these in full at the PR head:
- internal/rooms/floor_decay.go
- internal/hooks/NewRound_FloorDecay.go
- internal/scavenger/scavenger.go and walk.go
- internal/behaviortree/actions_scavenger.go
- the diffs to rooms.go, save_and_load.go, mobs.go, main.go, hooks.go, config.balance.go, config.balance.misc.go and config.yaml
- the scavenger archetype YAML and one scavenger mob YAML (9820)

For comparison I checked these baseline and PR-head mechanisms:
- rooms.IsPrivateRoom / SetPrivateRoomCheck (routing_hooks.go) and housing.RegisterRoomHooks
- removeUntakenBaubles
- GetRoomWithMostItems and the loot goblin hook and config
- the MobIdle_HandleIdleMobs ordering (EquipBestFloorItem before TryMobBehavior; shouldRecoverDisplacedHome)
- itemvalue.CanScanFloorLoot / CanEquipFromGive
- the steal takeFromMob draw
- messaging RenderForRecipient / Anonymize and the room send helpers
- actions/room_lines.go
- the forager territory wander
- the ferry loader (fileloader)
- the baubles and combat knob-validation conventions

Not reviewed:
- The test files (actions_scavenger_test.go, floor_decay_test.go, scavenger_test.go).
- The 15 mob YAMLs beyond 9820, and the room spawninfo edits.
- scavengers.yaml content.
- boot_smoke_test and messaging_surface_guard_test.
- Whether later PR commits touch these files. I judged the files as they stand at the head of my range (3ab2d1094). The worktree HEAD may be later; I did not diff that.
- Whether production config overrides re-enable LootGoblin.RoomId; I could not check that.

I found no authored template floor items or SpawnInfo item spawns in the scavenger city zones, so finding 1's impact is on player litter, not world props.

## Findings (4)

<a id="f026"></a>
### F026 [medium] The idle hook's floor grab (EquipBestFloorItem) runs on scavengers before scavenger_step, so a second pickup path skips the litter rule, the dark gate and the daily haul reset

`internal/hooks/MobIdle_HandleIdleMobs.go:211` · status **confirmed** · reported as medium

The project already has a mob floor-pickup path: hooks.EquipBestFloorItem, gated by itemvalue.CanScanFloorLoot. It runs on every idle tick at line ~211, before behaviortree.TryMobBehavior at line 240.

CanEquipFromGive excludes only these archetypes: noncombat_passive, noncombat_questgiver, noncombat_shopkeeper, prey and combat_passive. The new scavengers use behavior_archetype `scavenger` and `archetype: fighting`, so they pass the gate.

Each idle tick, a scavenger therefore runs two pickup mechanisms with different rules:
- **EquipBestFloorItem** takes any floor item that scores as an upgrade. It calls room.RemoveItem directly. It never consults rooms.Room.FloorItemIsLitter or actions.TooDarkToGet, and it does not go through actions.TakeFloorItem. It equips the item and announces it with its own 'picks up X and dons it' line.
- **scavengerPickUp** is the new path. Its own doc says every pickup goes through TakeFloorItem and the litter rule.

The PR designs the scavenger's pickup around the shared mechanism, but never opts the scavenger out of the older one, which still runs first. As a result, the design's guarantees do not hold for anything the older grab takes.

**Failure scenario.** 1. A player drops an ordinary sword in a New Plymouth Common street.
2. Old Mags (level 3, `archetype: fighting`) walks in. On her next idle tick, EquipBestFloorItem runs before scavenger_step, scores the sword as an upgrade, and equips it.
3. The room sees the generic 'picks up ... and dons it' line, not her authored pickup line.
4. The item is now in her Equipment, not in Character.Items. scavengerDailyReset clears only Character.Items and excess gold, so the sword is never taken away at midnight.
5. actions.takeFromMob steals via m.Character.GetRandomItem, the carried items, so a pickpocket cannot recover it either. Only killing her returns it.

The same grab also works in a room too dark for TakeFloorItem, because EquipBestFloorItem has no TooDarkToGet check.

**Existing mechanism.** hooks.EquipBestFloorItem / itemvalue.CanScanFloorLoot (internal/itemvalue/equip_eligibility.go), the existing mob floor-loot path the scavenger must opt out of

**Suggested fix.** Make scavengers ineligible for the floor-loot scan. Either add `scavenger` to the CanEquipFromGive archetype exclusion list, or have CanScanFloorLoot refuse mobs that have a scavenger profile. That leaves scavengerPickUp as the only pickup path for these mobs.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
MobIdle_HandleIdleMobs.go:
'''go
// Floor-loot scan: wild non-charmed combat mobs pick up
// gear upgrades they find on the room floor (chunk 2.3).
if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
    EquipBestFloorItem(mob, room)
}
'''
This is followed later by `if behaviortree.TryMobBehavior(mob.InstanceId, ...)` at line 240.

mob_equip_best_floor_item.go:29: `if !itemvalue.CanScanFloorLoot(&mob.Character, mob.BehaviorArchetype)`; line 56: `room.RemoveItem(bestItem, false)`; line 60: `actions.EquipItem(...)`.

equip_eligibility.go: `switch behaviorArchetype { case "noncombat_passive", "noncombat_questgiver", "noncombat_shopkeeper", "prey", "combat_passive": return false }`.

9820-old_mags.yaml: `behavior_archetype: scavenger`, `archetype: fighting`.

actions_scavenger.go:542-543: `haul := mob.Character.Items; mob.Character.Items = nil`. Equipment is untouched.
```

- **confirmed** (medium): The claim holds when traced through the code. Before line 211 of HandleIdleMobs, the only early returns are for a nil mob, a Sleeping flag and companionai.RouteIdle. None of them excludes a scavenger. The handler then calls EquipBestFloorItem(mob, room) unconditionally, and the behavior tree, where scavenger_step lives (archetypes/scavenger.yaml), runs only afterwards at line ~240.  EquipBestFloorItem's only gate is CanScanFloorLoot, which is CanEquipFromGive plus not charmed. CanEquipFromGive rejects species with the Weapon slot disabled and five named behavior archetypes. 'scavenger' is not among them. Old Mags is speciesid 1, humanoid, charm_immune and archetype fighting, so she passes.  ProfileFor('fighting', 'scavenger') falls through the behavior switch to the stat fallback and returns PhysicalBruiser. An unequipped mob therefore scores a plain sword as a positive-delta upgrade. The function then calls room.RemoveItem and StoreItem, followed by actions.EquipItem. It has no TooDarkToGet check, no FloorItemIsLitter check and no TakeFloorItem call, and it broadcasts its own 'picks up X and wields/dons it' line.  The equipped item leaves Character.Items. scavengerDailyReset (behaviortree/actions_scavenger.go:138-139) clears only Character.Items, so the daily reset never touches equipped gear. The PR adds no opt-out anywhere: grep for "scavenger" in internal/hooks and internal/itemvalue finds only the floor-decay hook. The PR's diff to MobIdle_HandleIdleMobs.go adds only the npcidle.TryReplace call, so the older path is left intact for the new archetype.  The pickpocket point (takeFromMob using carried items) is consistent with steal.go, though I did not read past the gold section. That does not change the core finding. Severity stays medium: the scavenger takes non-litter upgrades, in the dark, outside the design's daily reset and outside its authored message.

</details>

<a id="f085"></a>
### F085 [low] The new floor-decay knobs cannot honour 0 and have no off switch, contrary to the project's knob conventions; the DecayFloors `every == 0` guard is dead

`internal/configs/config.balance.misc.go:400` · status **confirmed** · reported as low

validateMisc resets FloorDecayBaseChancePct, FloorDecayPerExtraItemPct and FloorDecayCheckRounds to their defaults whenever the value is <= 0.

The project has explicit conventions for knobs where 0 is meaningful:
- 'absent (0) is the default, a negative value is "never"' (BaublePickpocketChancePct, config.balance.baubles.go:184)
- 'Slope 0 is legal (a flat bar); only a negative slope is corrected' (CritBarSkillSlope, config.balance.combat.go:423)

Floor decay is a system that permanently deletes items, but none of these patterns was used:
- The owner cannot turn floor decay off from config.yaml. Base 0, base -1 and check-rounds 0 all snap back to the defaults.
- The owner cannot set a flat per-room chance with no pile bonus, because PerExtra 0 snaps back to 5.
- The hook's `if every == 0 ... return` branch in NewRound_FloorDecay.go can never fire.
- An admin who sets 0 expecting 'off' gets 10 percent and 5 percent with no warning.

Retuning is supposed to be a config edit, not a code change. Here, disabling decay in an incident requires a code change.

**Failure scenario.** 1. A floor-decay bug is deleting player-dropped items, and the owner sets `FloorDecayBaseChancePct: 0` and `FloorDecayCheckRounds: 0` in config.yaml to stop it.
2. validateMisc rewrites them to 10 and 15.
3. DecayLoadedFloors keeps deleting items every ~15 rounds.

Separately, `FloorDecayPerExtraItemPct: 0`, intended as a flat 10 percent, still gives 30 percent for a five-item pile.

**Existing mechanism.** The established knob-validation convention in internal/configs/config.balance.baubles.go (BaublePickpocketChancePct: 0 is the default, negative means never) and config.balance.combat.go (CritBarSkillSlope: correct only negatives when 0 is legal)

**Suggested fix.** Follow the baubles convention:
- FloorDecayBaseChancePct: 0 (absent) is the default 10, a negative value means decay is off, and values over 100 cap at 100.
- FloorDecayPerExtraItemPct: correct only negatives, so 0 is a legal flat chance.
- Have decayFloor and DecayFloors return early when base < 0.

Document the off value in config.yaml.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
config.balance.misc.go (PR):
'''go
if b.FloorDecayBaseChancePct <= 0 { b.FloorDecayBaseChancePct = 10 }
if b.FloorDecayPerExtraItemPct <= 0 { b.FloorDecayPerExtraItemPct = 5 }
if b.FloorDecayCheckRounds <= 0 { b.FloorDecayCheckRounds = 15 }
'''
NewRound_FloorDecay.go: `every := uint64(configs.GetBalanceConfig().FloorDecayCheckRounds); if every == 0 || ...`.

Compare config.balance.baubles.go:184-192: `case ... == 0: default; case < 0: = -1 (never)`; and config.balance.combat.go:423: `Slope 0 is legal (a flat bar); only a negative slope is corrected`.
```

- **confirmed** (low): The claim reproduces from the code as written. validateMisc, called from Balance validation at config.balance.go:1490, resets all three floor-decay knobs to their defaults when the value is <= 0. So the YAML values 0 and -1 can never reach the consumers.  - **Check cadence:** the DecayFloors hook (registered unconditionally at hooks.go:52) reads FloorDecayCheckRounds after validation. It is always >= 1, so its `every == 0` early return is dead code. - **Chance:** decayFloor reads base and perExtra after validation. It also has a guard, `chance <= 0 → break`, and that guard is unreachable through config for the same reason. - **No other off switch:** the only other gate is the exemption predicate from SetFloorDecayExempt. It is set in code (main.go:1946) for scavenger-patrolled rooms, so it does not work as an owner toggle. - **Results:** an owner who sets Base 0 / CheckRounds 0 still gets 10% / every 15 rounds. PerExtra 0 still gives 10 + 4*5 = 30% for five items. Turning decay off therefore needs a code change.  One part of the claim is overstated. `<= 0 → default` is the dominant pattern in validateMisc, used by the neighbouring Scavenger knobs and most others. The exceptions (BaublePickpocketChancePct, CritBarSkillSlope) are knobs where 0 or "never" is meaningful. So this does not break a universal convention. It misses the existing opt-out pattern on a knob that permanently deletes items, and that pattern would fit naturally here.  The impact is real but low: it limits operational control and nothing is wrongly deleted at the shipped values.

</details>

<a id="f086"></a>
### F086 [low] The loot goblin is retired only by setting a config value to 0; its hook, config struct, room scan, portal and admin paths remain as a parallel cleanup mechanism

`internal/hooks/hooks.go:51` · status **confirmed** · reported as low

The PR replaces the loot goblin with floor decay and scavengers, but deletes none of the goblin's code. Its only changes are LootGoblin.RoomId 139 -> 0 in config.yaml and removing mob 13's spawninfo from room 139. Still live at the PR head:
- the SpawnLootGoblin NewRound listener, still registered right next to the new DecayFloors
- configs.LootGoblin and its Validate
- rooms.GetRoomWithMostItems, which still reads GetLootGoblinConfig
- the mobcommands/portal.go goblin branch
- the `mob loot goblin` admin special case

The new floor_decay.go header says the goblin 'is retired'. In code, retirement depends entirely on one config value. config.yaml carries the skip-worktree bit (a CLAUDE.md tripwire), and the server also reads config overrides, so a stale or overridden RoomId can silently run both cleanup systems. The project's refactoring rule is a full removal sweep, not a hack-around.

**Failure scenario.** 1. A server whose on-disk config.yaml (skip-worktree desync) or config override still has `LootGoblin.RoomId: 139` boots this build.
2. SpawnLootGoblin keeps preparing room 139 every RoundCount rounds.
3. The spawninfo is gone, so no goblin spawns, but the hook still runs, and the goblin's helpers stay reachable through `mob loot goblin` and portal.go.

If the spawninfo is re-added, as upstream content still has for mob 13 in the default world, the goblin and the new systems would both clear floors, under different rules about private rooms and litter.

**Existing mechanism.** The loot goblin mechanism itself: hooks.SpawnLootGoblin, configs.LootGoblin, rooms.GetRoomWithMostItems, and the mobcommands/portal.go goblin branch

**Suggested fix.** Do the removal sweep:
- Unregister and delete SpawnLootGoblin.
- Delete configs.LootGoblin, its config.yaml section, GetRoomWithMostItems if it has no other callers, and the portal and admin goblin branches.
- Or, if the owner wants the goblin kept as an option, gate it behind an explicit Enabled flag and stop calling it retired.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
hooks.go:51-52 (PR head): `events.RegisterListener(events.NewRound{}, SpawnLootGoblin)` followed by `events.RegisterListener(events.NewRound{}, DecayFloors)`.

internal/configs/config.lootgoblin.go is still present at 3ab2d1094.

roommanager.go:497 still has `lgConfig := configs.GetLootGoblinConfig()`.

The config.yaml diff only changes `RoomId: 139` to `RoomId: 0` and adds the comment 'Retired 2026-09-30'.

git grep at the head still finds the goblin branches at internal/mobcommands/portal.go:43 and internal/usercommands/admin.mob.go:181-186.
```

- **confirmed** (low): Every factual element of the claim reproduces at the PR head (3ce674451). The SpawnLootGoblin listener is still registered next to DecayFloors. configs.LootGoblin, its Validate and GetLootGoblinConfig are all intact. rooms.GetRoomWithMostItems still reads the goblin config. The portal "loot" branch and the admin `mob spawn loot goblin` special case are still present. The PR's only goblin-related changes are `RoomId: 139 -> 0` (with a "Retired 2026-09-30" comment) and deleting room 139's spawninfo. The mob definition `_datafiles/world/dogmud/mobs/endless_trashheap/13-loot_goblin.yaml` also still exists. So floor_decay.go's statement that the goblin "is retired" is enforced only by the `c.RoomId != 0` guard in SpawnLootGoblin and admin.mob.go.  The practical failure scenario is weak. With RoomId 0 the hook does nothing. Even with a stale RoomId of 139, the spawninfo is gone, so room.Prepare spawns nothing, and the portal-loot branch only runs for a mob scripted to use it, which is the goblin. Both cleanup systems could run together only if someone re-adds the spawninfo and restores RoomId. That is a real but unlikely path. This is a partial removal (a hack-around rather than a full sweep), which goes against the project's refactoring rule. It is not a runtime bug, so low severity is right.

</details>

<a id="f087"></a>
### F087 [low] Floor decay adds a second, unguarded player-home exemption hook instead of using rooms.IsPrivateRoom, which the sibling sweeps use

`internal/rooms/floor_decay.go:179` · status **confirmed** · reported as medium

The rooms package already has a registered private-room predicate: rooms.SetPrivateRoomCheck / IsPrivateRoom in routing_hooks.go. housing.RegisterRoomHooks wires it to housing.IsUnitRoom, and it is guarded by routingHooksMu. Existing floor sweeps consult it directly:
- removeUntakenBaubles at baubles_untaken.go:34
- GetRoomWithMostItems (the loot goblin's room scan) at roommanager.go:535

The new decayFloor never calls IsPrivateRoom. Instead the PR adds a parallel single-slot hook, SetFloorDecayExempt, stored in a plain package variable with no lock. main.go then wires housing.IsUnitRoom into that closure a second time. The scavenger World.Private also takes housing.IsUnitRoom directly instead of rooms.IsPrivateRoom.

This splits one rule across two registries:
- **Wiring dependency:** player-home protection in floor decay now depends on main.go's loadAllDataFiles composing the closure correctly.
- **Single slot:** the setter 'registers the one exemption predicate', so any later SetFloorDecayExempt call (a reload, a test, a future feature) silently replaces it and drops the housing protection.
- **Unguarded write:** the closure is written while the NewRound DecayFloors listener reads it, with no lock. routing_hooks guards its sibling hooks with routingHooksMu.
- **Drift:** if the definition of a private room ever changes in SetPrivateRoomCheck (for example a guild hall or ephemeral-owned room), the bauble sweep and goblin scan follow it but floor decay does not.

**Failure scenario.** 1. Any code path calls rooms.DecayLoadedFloors without main.go's closure installed, or a future feature calls rooms.SetFloorDecayExempt with its own predicate. Either way the closure is replaced or nil.
2. The next DecayFloors tick rolls items on a player-housing unit floor. Those items are deleted permanently.
3. In the same configuration, removeUntakenBaubles still protects that floor, because it reads IsPrivateRoom.

The two sweeps disagree about the same room, and one destroys player property.

**Existing mechanism.** rooms.IsPrivateRoom / rooms.SetPrivateRoomCheck (internal/rooms/routing_hooks.go), registered by housing.RegisterRoomHooks

**Suggested fix.** In decayFloor, check `IsPrivateRoom(r.RoomId)` directly, as removeUntakenBaubles does, and keep SetFloorDecayExempt for the scavenger patrol set only. Guard that setter with routingHooksMu, or move it into routing_hooks.go. Pass `Private: rooms.IsPrivateRoom` to the scavenger World.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
floor_decay.go:56: `var floorDecayExempt func(roomId int) bool` (no mutex).

floor_decay.go:179: `if floorDecayExempt != nil && floorDecayExempt(r.RoomId) {`. There is no IsPrivateRoom call in the file.

baubles_untaken.go:34: `if IsPrivateRoom(r.RoomId) { return 0 }`.

roommanager.go:535: `if IsPrivateRoom(cRoomId) { continue }`.

routing_hooks.go:145: `func SetPrivateRoomCheck(fn PrivateRoomCheck) { routingHooksMu.Lock() ...`.

housing/door.go:18: `rooms.SetPrivateRoomCheck(IsUnitRoom)`.

main.go (PR): `rooms.SetFloorDecayExempt(func(roomId int) bool { return scavenger.IsPatrolledRoom(roomId) || housing.IsUnitRoom(roomId) })` and `Private: housing.IsUnitRoom,`.
```

- **confirmed** (low): The structural facts are right, so I'm confirming this as a consistency finding. The failure scenario does not happen with the code as written, so I'm lowering the severity from medium to low.  What holds up: - decayFloor (floor_decay.go:179) never calls IsPrivateRoom. It reads a single-slot package variable, `floorDecayExempt`, that has no lock. - main.go:1946 wires `housing.IsUnitRoom` into that closure. That is the same predicate housing.RegisterRoomHooks (main.go:305) already registers through rooms.SetPrivateRoomCheck. - The scavenger's `Private:` field (main.go:1940) takes `housing.IsUnitRoom` directly. - Sibling sweeps use `IsPrivateRoom` instead: baubles_untaken.go:34, roommanager.go:535 and save_and_load.go:299.  So the player-home rule is wired twice, and floor decay would not pick up a future change to the private-room definition. That part of the reuse/unification point is valid.  Why the failure scenario does not reproduce: 1. The only non-test caller of SetFloorDecayExempt is main.go loadAllDataFiles. The only other caller is a test that restores nil with defer. 2. A reload (world.go:207, the System `reload` command) calls loadAllDataFiles(true), which reinstalls the same composed closure, so housing protection is never dropped. 3. The reload handler is an event listener, and DecayFloors is also an event listener (hooks.go:52). Both appear to run on the event loop, so the "unguarded write races the reader" case is theoretical. At worst it is a benign pointer swap of an equivalent closure. IsUnitRoom itself takes housing's own mu.RLock. 4. No code path today runs DecayLoadedFloors with housing unprotected, and the two sweeps do not disagree today.  The separate hook does have a real reason: it also carries `scavenger.IsPatrolledRoom`, which is not a private-room concept. The cleaner form would have decayFloor check `IsPrivateRoom(r.RoomId)` directly and keep the hook for scavenger patrols only. That is a low-severity consistency cleanup, not a medium defect that destroys player property.

</details>
