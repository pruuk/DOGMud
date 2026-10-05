# PR #208 review: floor decay scavengers, correctness lens

Blind reviewer `floor_decay_scavengers:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed the floor-decay and city-scavenger piece (f7310ed77..3ab2d1094) as it stands at the PR head. I found one real correctness bug and two lower-severity issues.

1. **Medium:** the daily decay keeps one clock per room, not per item. An item dropped into a room that already holds old, unpaid litter is rolled for every day the old litter is owed, up to 30. A sword a player put down a minute ago can therefore vanish almost certainly, which breaks the stated rule that "an item dropped today is never rolled for days it did not lie there".
2. **Low:** balance validation turns 0 into the default for all three decay knobs. An operator cannot switch decay off, and the hook's `every == 0` early exit can never run.
3. **Low:** the rooms a scavenger patrols are exempt from decay whether or not that scavenger is alive, and it has no carry room left. Nothing else cleans those rooms when it is dead or full.

These checked out clean:
- boot and YAML content: home rooms and spawninfo all match, and no spawninfo key is duplicated.
- the overlay timing for `authoredItemIds`.
- the "scavenger" group: no existing mob uses it, so `IsEssential` picks up no unintended mobs.
- nil safety: `GetSpec` returns a value, so no nil dereference.
- a broken `scavengers.yaml` on reload: the panic is recovered.
- rollback of a failed pickup when the scavenger's pack is full.
- the loot goblin: its spawn is disabled through config and through the room's spawninfo.

## Coverage

I read the following in full at the PR head:
- `internal/rooms/floor_decay.go`
- `internal/hooks/NewRound_FloorDecay.go`
- `internal/scavenger/scavenger.go` and `walk.go`
- `internal/behaviortree/actions_scavenger.go`
- the scavenger archetype YAML
- one scavenger mob YAML
- the head of `scavengers.yaml`

I also read the diffs for `rooms.go`, `save_and_load.go`, `mobs.go`, `main.go`, the config and balance validation, `boot_smoke_test.go`, and the room spawninfo changes. Every home room was cross-checked against its spawninfo mobid.

I followed these existing mechanisms to check how the new code interacts with them:
- `actions.TakeFloorItem` and `TransferItemToBackpack`
- `Character.StoreItem` and `CarryCapacity`
- `RoomMaintenance` unload
- `HandleRespawns`
- the Prepare spawn path
- `SaveRoomTemplate` replacement
- `housing.IsUnitRoom`
- mob saved-instance restore
- the old loot goblin hook

I did not read the test files (blind review), the `context.md`, `PATCH_NOTES` or `messaging_surface_guard` changes, or the remaining 14 scavenger mob YAMLs and the rest of `scavengers.yaml` beyond spot checks. I ran no tests, and I did not use codegraph or check what idle-tick cadence non-essential mobs get.

## Findings (3)

<a id="f024"></a>
### F024 [medium] Catch-up decay passes are applied to items dropped minutes ago (room-level clock, not per-item)

`internal/rooms/floor_decay.go:161` · status **confirmed** · reported as medium

The room's clock (`FloorDecayDay`) only restarts when litter lands on a floor that holds no litter (`noteFloorItemAdded` returns early when `litterCount() > 0`). `decayFloor` then rolls `today - FloorDecayDay` passes (up to 30) over every litter item in the room. An item added to a room that already holds old, unpaid litter inherits the old item's debt. The design comment at lines 27-29 promises the opposite: "an item dropped today is never rolled for days it did not lie there".

**Failure scenario.** 1. Day D: a corpse rots in wilderness room R and drops its loot to the floor (`rooms.go` ~line 2880, via `AddItem`), stamping `FloorDecayDay = D`.
2. Nobody visits R, so it unloads after `RoomUnloadRounds` (450 rounds, about 30 minutes) once more than `RoomUnloadThreshold` (200) rooms are loaded. That is the normal state for the wilds.
3. Day D+10: a player walks into R. Decay is deferred because a player is present. The player drops a valuable sword to free carry weight and steps out to fight next door.
4. Within `FloorDecayCheckRounds` (about 1 minute), `decayFloor` runs 10 passes. With 5 old items plus the sword, each roll is at least 35%, so the sword survives with probability of roughly 0.65^10, about 1%.

The player loses an item they put down a minute ago. The intended outcome is one 10-35% roll at the next midnight.

**Suggested fix.** Give each item its own clock. One way is to stamp a drop day on the item instance in `AddItem` and roll each item for `min(today - itemDay, cap)` passes. The other is to settle the room's owed passes before the new item is appended: call the decay or settle in `noteFloorItemAdded` when the room is unoccupied, or advance `FloorDecayDay` to today after paying the old debt, so new arrivals start clean.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
`floor_decay.go:157-165`:
'''
if !r.FloorItemIsLitter(itm) { return }
if r.litterCount() > 0 { return }
r.FloorDecayDay = DecayDay(floorDecayNow())
'''

`floor_decay.go:192-198`:
'''
passes := 1
if r.FloorDecayDay > 0 { passes = int(today - r.FloorDecayDay) }
if passes > floorDecayMaxCatchUpDays { passes = floorDecayMaxCatchUpDays }
'''

`floor_decay.go:203-212` then rolls every litter item on each pass.

`_datafiles/config.yaml:135` `RoomUnloadRounds: 450`, `:138` `RoomUnloadThreshold: 200`.
```

- **confirmed** (medium): The code does what the claim says. AddItem calls noteFloorItemAdded, which restarts the clock only when the floor holds no litter. When older litter is already there, the new item takes on the room's old FloorDecayDay. decayFloor then runs `today - FloorDecayDay` passes (capped at 30) over every litter item, the new one included. That breaks the promise in the header comment at lines 27-29.  The multi-day backlog arises because RoomMaintenance unloads rooms whatever is on their floor. The only gates are lastVisited, the unload threshold, not ephemeral and no essential mob. FloorDecayDay is saved with the instance (rooms.go:117), so when the room loads again it still owes the missed passes.  The deferral at decayFloor:183 (a player is in the room) is what lets the newcomer's item join the backlog. The player who walks in loads the room and stands in it. Their drop does not restamp the clock, because old litter is present. After they leave, the next check pays all the owed passes, and the new item is rolled in each one.  One correction to the scenario: "each roll is at least 35%" is wrong. FloorDecayChancePct is recomputed from litterCount() on every pass, so the chance falls as items are removed, down to a 10% floor. The sword's chance of surviving is therefore higher than 0.65^10 (about 1%). It is still far worse than the single 10-35% roll the comment promises, so the defect and the medium severity stand.

</details>

<a id="f080"></a>
### F080 [low] Balance validation coerces 0 to defaults, so floor decay can never be disabled and the hook's every==0 guard is dead

`internal/configs/config.balance.misc.go:400` · status **confirmed** · reported as low

`validateMisc` rewrites `FloorDecayBaseChancePct <= 0` to 10, `FloorDecayPerExtraItemPct <= 0` to 5, and `FloorDecayCheckRounds <= 0` to 15. Two consequences follow:
- An operator who sets `FloorDecayBaseChancePct: 0` or `FloorDecayCheckRounds: 0` to switch decay off silently gets the defaults instead.
- `FloorDecayPerExtraItemPct` cannot be set to 0 to make the chance flat.

The hook's `if every == 0 ... return` early exit (`NewRound_FloorDecay.go:15`) is unreachable, as is the `chance <= 0 { break }` branch for a zero base. This conflicts with the project's balance-config rule that 0 is a legal shipped value.

**Failure scenario.** Staff discover that floor decay is eating items (for example, the catch-up bug above) and set `FloorDecayCheckRounds: 0` or `FloorDecayBaseChancePct: 0` in `config.yaml` as an emergency off-switch. Validate rewrites both to 15 and 10, and decay keeps deleting player items.

**Existing mechanism.** The `dogmud-balance-config` convention: an absent key is meaningful because 0 is a legal shipped value. Default only when the value is negative or the key is absent.

**Suggested fix.** Coerce only negative values (< 0), so 0 means "off" for `CheckRounds` and `BaseChance` and "flat" for `PerExtra`. Alternatively, add an explicit `FloorDecayEnabled` bool.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
`config.balance.misc.go`:
'''
if b.FloorDecayBaseChancePct <= 0 { b.FloorDecayBaseChancePct = 10 }
if b.FloorDecayPerExtraItemPct <= 0 { b.FloorDecayPerExtraItemPct = 5 }
if b.FloorDecayCheckRounds <= 0 { b.FloorDecayCheckRounds = 15 }
'''

`NewRound_FloorDecay.go`:
'''
every := uint64(configs.GetBalanceConfig().FloorDecayCheckRounds)
if every == 0 || evt.RoundNumber%every != 0 {
'''
```

- **confirmed** (low): The code does what the claim says. `validateMisc` (lines 400-408 of internal/configs/config.balance.misc.go, added by the PR) rewrites `<= 0` to 10, 5 and 15 for the three FloorDecay knobs. That means a configured 0 can never reach the runtime. You cannot use FloorDecayCheckRounds: 0 or FloorDecayBaseChancePct: 0 to turn decay off, and you cannot set FloorDecayPerExtraItemPct: 0 for a flat chance. Two things keep the severity low. (1) Nothing documents 0 as an off switch. The yaml comments and the struct doc only give "Default N" and say nothing about what 0 does. The same `<= 0 -> default` pattern is used for every neighbouring knob in validateMisc (ScavengerStep*, MailSendCooldownRounds, GuildVaultCapacity and others), so this matches local convention rather than breaking it. (2) "Dead guard" overstates it. The `every == 0` check in the hook also protects the modulo against a divide-by-zero whenever validation is bypassed, for example in test binaries that use raw Go defaults. Calling it defensive is fairer than calling it dead. The real cost is that operators have no config kill switch for floor decay. The emergency-off scenario would fail silently. The fallback is to raise FloorDecayCheckRounds very high, but that only delays the pass and does not skip it, because the daily debt accumulates (capped by floorDecayMaxCatchUpDays). Per the dogmud-balance-config convention ("0 is a legal shipped value"), PerExtra = 0 is a legitimate tuning value that cannot be expressed.

</details>

<a id="f081"></a>
### F081 [low] Patrol rooms are permanently decay-exempt with no fallback when their scavenger is dead or full

`main.go:1922` · status **confirmed** · reported as low

`SetFloorDecayExempt` exempts every room in a scavenger's pool unconditionally, through `IsPatrolledRoom`, which reads only the static pool set. The scavenger is the only cleanup for those rooms, but there are two ways it can stop working:
- It is killable (`non_combatant: false`, it fights back). Its home room is re-Prepared only when a player stands in or next to it: `HandleRespawns` iterates `GetRoomsWithPlayers`, and movement calls `Prepare(true)`. The old loot goblin had its own `SpawnLootGoblin` hook to force its room's Prepare; the scavengers have no equivalent after boot.
- Its pickups are bounded by carry weight. `StoreItem` refuses past 2x `CarryCapacity`, and the haul is cleared only once a day.

While the scavenger is dead and not respawned, or full, litter in its whole district accumulates with no decay at all.

**Failure scenario.** 1. A player kills Old Mags (9820) in a remote Common room for her haul.
2. No player happens to step into or next to home room 5618 for hours, so she does not respawn.
3. Every Common pool room is still exempt from decay, so all litter dropped there piles up indefinitely until someone wanders by 5618.

The same happens on a busy day once her pack reaches its weight cap: pickups fail (`TakeFloorItem` errors and the code continues) until UTC midnight.

**Existing mechanism.** `SpawnLootGoblin` (`internal/hooks/NewRound_SpawnLootGoblin.go`) periodically Prepares a fixed room so its custodian mob respawns.

**Suggested fix.** Periodically Prepare `scavenger.AnchorRooms()`, as the goblin hook did for its room. Alternatively, make the exemption conditional on the profile's scavenger instance being alive, or let exempt rooms fall back to decay when their scavenger has been missing for more than a day.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
`main.go:1922-1924`:
'''
rooms.SetFloorDecayExempt(func(roomId int) bool {
	return scavenger.IsPatrolledRoom(roomId) || housing.IsUnitRoom(roomId)
})
'''

`NewRound_HandleRespawns.go`: `for _, roomId := range rooms.GetRoomsWithPlayers() { ... room.Prepare(false) }`. Scavenger home rooms are Prepared only once, at boot (`main.go` ~1953-1961).

`characters/inventory.go:185`: `if newWeight > capacity*2.0 { return false }`.

`actions_scavenger.go:184`: `if err := actions.TakeFloorItem(actor, itm, false); err != nil { continue }`.
```

- **confirmed** (low): The claim holds as the code is written. The decay exemption is unconditional: `IsPatrolledRoom` only checks the static `patrolled` map, and nothing checks whether the scavenger is alive or has room to carry more. Old Mags (9820) is killable (`non_combatant: false`, `hostile: false`, `itemdropchance: 100`). Her respawn depends on `Room.Prepare` running on home room 5618, and nothing runs it on a schedule. Only these call `Prepare` outside tests: `HandleRespawns`, which covers only `GetRoomsWithPlayers`; the room-load path in roommanager and the look commands, which run when a player is in or next to the room; the admin commands; `SpawnLootGoblin`, which handles only the configured loot-goblin RoomId; and the boot loops in main.go, which prepare `scavenger.AnchorRooms()` once. So a dead scavenger whose home room no player visits stays dead, and every room in her pool goes without decay until then. The weight-cap part also holds: `StoreItem` refuses an item past 2x `CarryCapacity`, and `scavengerPickUp` skips any item whose `TakeFloorItem` call errors, until the daily reset. The suggested existing mechanism is real. `SpawnLootGoblin` runs `Prepare` on a fixed room every N rounds, and the PR does not extend it, or anything like it, to the scavenger anchors. Severity stays low. The result is litter building up in one district, not lost data or a security problem, and it ends as soon as a player passes near the home room or the daily reset clears the haul.

</details>
