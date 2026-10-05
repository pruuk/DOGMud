# PR #208 review: floor decay scavengers, security lens

Blind reviewer `floor_decay_scavengers:security`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Security review of the floor-decay and city-scavenger piece (f7310ed77..3ab2d1094), judged at the PR head. Four findings. (1) Medium: the scavenger opens a laundering route for bauble value. A player drops their own honest bauble in a patrolled city room, the scavenger picks it up, and the player pickpockets it back. The bauble is then marked stolen goods, and a fence pays 60% of its value instead of the honest 50%. This gets around the guard in markPocketStolen that was written to block exactly this through `give`. The shipped config has BaublesEnabled set to false, which holds it back today. (2) Medium: an operator has no way to turn floor decay off. A 0 in FloorDecayBaseChancePct or FloorDecayCheckRounds is quietly replaced with 10 / 15, so the hook's own `every == 0` off switch can never run. An operator who sets 0 to stop item deletion still gets player items deleted every day. The old system could be switched off (LootGoblin.RoomId: 0). (3) Low/medium: a player can make the server do quadratic work and stop the city from being cleaned. Once a scavenger's pack is full by weight, every idle tick in a littered room runs TakeFloorItem on every litter item. Each attempt fails and rolls back through room.AddItem, which calls noteFloorItemAdded, which runs a full litterCount scan. That is O(n^2) per tick. Meanwhile patrolled rooms are excluded from decay, so a jammed scavenger leaves its city's floors growing until the midnight reset. (4) Low: litter is decided by ItemId alone. Any item a player drops is exempt from decay and from scavengers if its ItemId matches an item the room's template or SpawnInfo places on the floor. No secret leaks, no auth bypass and no injection found: pickup lines use the viewer-agnostic DisplayName (the safe generic bauble name), housing unit rooms are excluded from both decay and the scavenger pools, and the scavengers are charm-immune.

## Coverage

I read these in full at the PR head: internal/rooms/floor_decay.go, internal/hooks/NewRound_FloorDecay.go, internal/scavenger/scavenger.go and walk.go, internal/behaviortree/actions_scavenger.go, the scavenger archetype and Old Mags mob YAML, and the diffs to config.yaml, config.balance.go, config.balance.misc.go, rooms.go, save_and_load.go, mobs.go and main.go. Outside the PR I cross-checked: actions/get.go (TakeFloorItem), actions/transfer.go, characters StoreItem/CarryCapacity, steal.go, steal_pocket.go (markPocketStolen), baubles/theft.go, sell_bauble.go pricing, items/bauble_viewer.go (DisplayName is the viewer-agnostic, safe version, so no leak), housing IsUnitRoom and its population, rooms.IsPrivateRoom, and the mob auto-equip hook. Checked and found clean: a player's dropped item is not in a room's authored list, the housing exclusion covers both pools and decay, scavengers are charm-immune and the 15 mobs share one shape (only Old Mags read in full), map iteration matches the other roomManager.rooms sweeps, and nothing new was injectable in the narration lines. Not read in detail: the other 14 mob YAMLs, scavengers.yaml, the room spawninfo edits, tests, the context.md files, boot_smoke_test.go and messaging_surface_guard_test.go. Out of scope for this security lens: the floor-decay exemption calls housing.IsUnitRoom directly instead of the existing rooms.IsPrivateRoom hook (rooms/routing_hooks.go:288). Crash-timed save duplication (an item picked from a floor whose room save is older) is a general problem that existed before this PR and is not specific to it.

## Findings (4)

<a id="f025"></a>
### F025 [medium] Scavenger turns a dropped honest bauble into fence-premium stolen goods (bypasses the GivenTo anti-laundering guard)

`internal/behaviortree/actions_scavenger.go:184` · status **confirmed** · reported as medium

scavengerPickUp takes any floor item that FloorItemIsLitter accepts. A bauble a player drops has no BaubleLeftAt, because only the search paths call LeaveBaubleAt, so it counts as litter and goes into the scavenger's inventory. Pickpocketing a bauble from a mob (steal.go:334) calls markPocketStolen. That function skips the stolen mark only when baubles.Record.GivenTo(mob) is true, and only `give` sets that through MarkGiven. The guard exists so a player cannot hand a bauble to a mob and steal it back to make it stolen goods; the comment says 'no fence premium, no heat'. The scavenger is a new way to put a player's bauble into a mob's pocket, and it never records GivenTo, so stealing it back calls baubles.MarkStolen and Record.StolenGoods() becomes true. A fence pays FencePrice = BaubleFenceBuyPct (60) percent, against BaublePrice = ShopBuyRatio (0.50) for an honest bauble.

**Failure scenario.** Baubles are enabled. A player holds an honest rare bauble worth 200, which sells for 100 at an honest merchant. They drop it in an empty room in Old Mags' pool and wait for her pickup line. Her pack holds only that bauble, so GetRandomItem is certain to pick it, and the player (or an alt, which also avoids crime consequences on the main) pickpockets her. The bauble is now stolen goods and a fence pays 120. The player gains 20% on every bauble, repeatably, by a route the existing GivenTo guard was written to close.

**Existing mechanism.** baubles.MarkGiven / Record.GivenTo, the provenance guard that already handles give-then-steal-back

**Suggested fix.** When a scavenger picks up a bauble, record it the way a gift is recorded: call baubles.MarkGiven(itm.Bauble, int(mob.MobId)) so a later pickpocket does not mint stolen goods. Alternatively, never let scavengers take baubles at all and leave them to the floor decay or the untaken clock.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
actions_scavenger.go:177-190: `floor := append(room.Items[:0:0], room.Items...)` / `if !room.FloorItemIsLitter(itm) { continue }` / `actions.TakeFloorItem(actor, itm, false)`. floor_decay.go:134-140 skips only an untaken bauble (BaubleLeftAt>0) or a household bauble. LeaveBaubleAt callers are only search_bauble.go:340/381/396/411 and steal.go:369, so `drop` never sets it. steal.go:331-335: `if itemStolen, found := m.Character.GetRandomItem(); found { ... if itemStolen.IsBauble() && actor.IsPlayer() { markPocketStolen(...) }`. steal_pocket.go:193-197: `if rec, ok := baubles.Get(it.Bauble); ok && rec.GivenTo(int(m.MobId)) { // A player gave it to this mob ... no fence premium, no heat ... return }`. sell_bauble.go: `case fence && rec.StolenGoods(): price = FencePrice(rec.Value)`. config.yaml: BaubleFenceBuyPct: 60, ShopBuyRatio: 0.50, BaublesEnabled: false (the mitigation for now).
```

- **confirmed** (medium): I traced the whole chain in the code, and it works as claimed.  1. A dropped bauble counts as litter. The only things that set BaubleLeftAt are LeaveBaubleAt calls in search_bauble.go and steal.go:369. Character.StoreItem (characters/inventory.go:178) clears it, and the comment in items/bauble_placement.go says "Dropping it again does not set them". So BaubleUntakenFor returns false for a dropped bauble, and FloorItemIsLitter (floor_decay.go:125-141) returns true unless the item is authored, a quest token or a household bauble.  2. scavengerPickUp (actions_scavenger.go:177-190) loops over the floor items, keeps the ones that pass FloorItemIsLitter, and calls actions.TakeFloorItem. TakeFloorItem (get.go:42-63) refuses only darkness, exploding items and household baubles, so the bauble goes into the mob's backpack. No MarkGiven call happens anywhere on that path.  3. In steal.go:331-335, GetRandomItem takes an item from the mob, and a bauble taken by a player goes to markPocketStolen. markPocketStolen (steal_pocket.go:192-205) skips the stolen mark only when rec.GivenTo(mobId) is true. The only non-test caller of MarkGiven is StolenBaubleGiven (stolen_bauble.go:442), which runs only on `give`. So baubles.MarkStolen runs, and StolenGoods() becomes true.  4. sell_bauble.go:123 handles `case fence && rec.StolenGoods()` with FencePrice, which pays BaubleFenceBuyPct percent of the value. That is 60 in config.yaml, both on disk and in HEAD. An honest sale pays ShopBuyRatio, which is 0.50.  Is the route new? At baseline c696c117a I found no mob content or script that sends a `get` floor pickup: git grep for a `get` idle command or Command("get") found nothing. Before this PR, the only ways a player could put an honest bauble into a mob's pocket went through `give`, which the guard covers. The scavenger is a new route around a guard that was written on purpose.  What limits the impact: BaublesEnabled is false in the shipped config. The pickpocket still counts as a crime for whoever does it, though an alt can take that risk. The gain is only about 10% of the bauble's value (60% against 50%). That makes it a real but modest economy exploit, so medium, and arguably low while baubles are off.

</details>

<a id="f082"></a>
### F082 [low] A scavenger at full carry weight does O(n^2) work every idle tick, and patrolled floors stop being cleaned

`internal/behaviortree/actions_scavenger.go:178` · status **confirmed** · reported as low

StoreItem refuses an item once carried weight would pass 2x capacity. After that, every idle tick in a littered room calls TakeFloorItem on every litter item. Each call does room.RemoveItem (O(n)), StoreItem fails, and the rollback calls room.AddItem. AddItem now calls noteFloorItemAdded, which runs litterCount(), a full scan of the floor through FloorItemIsLitter with a SpawnInfo loop and bauble checks. That is O(n^2) per tick per jammed scavenger, and each rollback also moves the item to the end of r.Items. Patrolled rooms are exempt from floor decay (main.go SetFloorDecayExempt), so while the scavenger is full nothing clears those floors until its midnight haul reset.

**Failure scenario.** Over a few trips a player drops a few hundred cheap heavy items (stones, junk armour) into one or more rooms in a scavenger's pool. The scavenger fills by weight after a few pickups. From then on, every idle tick it spends lingering in those rooms (30 to 150 s each) runs n x (RemoveItem + AddItem + litterCount over n) on the game loop, about 250k litter checks per tick at n=500, and players can do this to all fifteen scavengers. Those city floors also never decay, so the pile stays and keeps costing, and it grows every day as the player adds more.

**Suggested fix.** Check capacity before trying a pickup: skip items whose weight would not fit, or stop the loop after the first capacity failure. Make noteFloorItemAdded short-circuit (stop at the first litter item, or skip the restamp on a rollback). When a scavenger cannot carry more, consider letting its rooms fall back to normal decay.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
actions_scavenger.go:178-190 loops all litter, calling TakeFloorItem with `continue` on error. transfer.go TransferItemToBackpack: `removeFrom(item); if !toChar.StoreItem(item) { rollback(item) ...}`, and get.go passes `func(i items.Item) { room.AddItem(i, stash) }` as the rollback. rooms.go AddItem (PR): `r.noteFloorItemAdded(item)`. floor_decay.go:157-165: `if !r.FloorItemIsLitter(itm) { return } if r.litterCount() > 0 { return }`, where litterCount (144-152) scans every item with no short-circuit. inventory.go:185 `if newWeight > capacity*2.0 { return false }`. main.go exempts `scavenger.IsPatrolledRoom(roomId)` from decay.
```

- **confirmed** (low): Every link in the claimed chain is in the PR head as written. actScavengerStep runs scavengerDailyReset, then scavengerPickUp on every tick, before the idle and step branches. So a scavenger that cannot pick anything up retries the whole floor each tick.  scavengerPickUp copies room.Items and calls TakeFloorItem on each litter item, treating any error as `continue`. TakeFloorItem goes through TransferItemToBackpack, which removes the item first (room.RemoveItem, a linear scan from the back). It then calls StoreItem, which refuses once carried weight would pass capacity*2. The rollback is room.AddItem, which now calls noteFloorItemAdded before appending.  For a litter item, noteFloorItemAdded calls litterCount(). That scans every r.Items entry through FloorItemIsLitter (a SpawnInfo loop, a GetSpec lookup and bauble checks) and never stops early. So one jammed tick costs about n * (O(n) remove + O(n) litter scan), which is O(n^2). Each rollback also appends the item at the end of r.Items, reordering the floor.  The patrolled-room exemption is real: main.go:1946 passes scavenger.IsPatrolledRoom (or a housing unit) to SetFloorDecayExempt. While a scavenger is full, nothing clears those floors until scavengerDailyReset drops its haul on the next real-world day. I found no cap on how many items a room floor can hold.  Low severity fits. It is a slow, griefable CPU cost on the game loop that needs hundreds of items to matter, not a correctness or data-safety issue.

</details>

<a id="f083"></a>
### F083 [low] Floor decay cannot be disabled: a 0 knob is silently replaced with a default, and the hook's off switch is dead code

`internal/configs/config.balance.misc.go:400` · status **confirmed** · reported as medium

validateMisc rewrites FloorDecayBaseChancePct <= 0 to 10, FloorDecayPerExtraItemPct <= 0 to 5, and FloorDecayCheckRounds <= 0 to 15. DecayFloors has an `every == 0` branch meant to disable the pass, but validation guarantees it never sees 0. No knob turns off this permanent deletion of player items. The loot goblin it replaces could be switched off (LootGoblin.RoomId: 0). Project rule: 0 is a legal shipped value and an absent key means something.

**Failure scenario.** An operator wants no player items deleted, for example during an event or while a decay bug is investigated, and sets FloorDecayBaseChancePct: 0 or FloorDecayCheckRounds: 0. At the next UTC midnight every loaded room with litter still loses items at 10% plus 5% per extra item, and nothing logs that the setting was overridden. The items are gone; decayFloor emits no ItemOwnership event and keeps no record.

**Suggested fix.** Let 0 through for FloorDecayBaseChancePct and FloorDecayCheckRounds (clamp only negative values) and document 0 as off, so the existing `every == 0` and `chance <= 0` branches become the off switch.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
config.balance.misc.go:400-408: `if b.FloorDecayBaseChancePct <= 0 { b.FloorDecayBaseChancePct = 10 }` ... `if b.FloorDecayCheckRounds <= 0 { b.FloorDecayCheckRounds = 15 }`. hooks/NewRound_FloorDecay.go:21-24: `every := uint64(configs.GetBalanceConfig().FloorDecayCheckRounds)` / `if every == 0 || evt.RoundNumber%every != 0 { return events.Continue }`, where the `every == 0` arm cannot be reached. floor_decay.go:204-206 `if chance <= 0 { break }` cannot be reached either, because base is never 0.
```

- **confirmed** (low): The claim checks out against the code as written. Balance.Validate (config.balance.go:1490) calls validateMisc. At config.balance.misc.go:400-408, validateMisc changes any value <= 0 to a default: FloorDecayBaseChancePct becomes 10, FloorDecayPerExtraItemPct becomes 5 and FloorDecayCheckRounds becomes 15. Because of that, the `every == 0` arm in hooks/NewRound_FloorDecay.go:22 can never be reached through validated config, and neither can `chance <= 0 { break }` in rooms/floor_decay.go:205. FloorDecayChancePct is base + extras capped at 100, and base is never below 10, so the chance is never 0. No knob turns the pass off. The only rooms skipped are exempt (scavenger), ephemeral or occupied ones. At baseline c696c117a the loot goblin did have an off switch: SpawnLootGoblin is gated on `c.RoomId != 0`, and config.yaml documented "If zero (0), no loot goblin spawning". So the new mechanism lost the operator's ability to disable cleanup.  I lowered the severity from medium to low for three reasons: 1. This is not a security exposure. Only the operator can edit config.yaml, and no player can trigger or abuse it. 2. Turning a value <= 0 into a default is the same pattern every other entry in validateMisc uses, so this follows the codebase convention rather than adding a new hazard. The real defect is that the off branches are dead code and nothing documents that floor decay cannot be disabled. 3. The damage the failure scenario describes is the designed behaviour of the feature (litter decays). It is not data loss caused by the bug. The only operator who loses something is one who expected 0 to work as an off switch.  The claim that this "silently" overrides the setting also holds: validateMisc logs nothing when it replaces a value.

</details>

<a id="f084"></a>
### F084 [low] Litter is matched by ItemId alone, so a player's copy of any template or SpawnInfo floor item never decays and is never picked up

`internal/rooms/floor_decay.go:109` · status **confirmed** · reported as low

isAuthoredFloorItem treats any floor item whose ItemId appears in the room template's Items, or in a floor SpawnInfo entry, as world content. It does not check which instance it is. A player-dropped item with that id is therefore never litter: it is skipped by the daily decay and by scavengers, and it does not count toward the pile-size chance.

**Failure scenario.** A room's template or SpawnInfo puts item X on its floor. A player drops any number of their own X copies there, or in another room that spawns X, and stores them indefinitely, outside both cleanup systems the PR presents as the only floor cleanup now that the loot goblin is retired.

**Suggested fix.** Mark authored or spawned floor items on the instance (for example a flag set when the template or SpawnInfo places them, or their UUIDs), and match on that instead of ItemId.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
floor_decay.go:109-118: `if r.authoredItemIds[itm.ItemId] { return true }` / `for _, si := range r.SpawnInfo { if si.ItemId > 0 && si.ItemId == itm.ItemId && si.Container == `` { return true } }`. save_and_load.go (PR) builds authoredItemIds from template ItemIds only.
```

- **confirmed** (low): The code does what the claim says. isAuthoredFloorItem (floor_decay.go:109-119) matches on ItemId alone, with no instance identity, ownership or count check. FloorItemIsLitter returns false for any item it matches. The same predicate gates three things: decayFloor's removal roll, litterCount (pile size and the clock start) and the scavengers. So a player-dropped copy of a template or floor-SpawnInfo item id, in a room whose template or SpawnInfo names that id, is never decayed and never collected.  One part of the scenario overreaches. "Or in another room that spawns X" is true only for that other room. The match is per room (r.authoredItemIds and r.SpawnInfo), so a copy of X dropped anywhere else is litter.  Impact is narrow. On disk only 2 room files carry a template `items:` list and about 25 carry an `itemid:` (some of those may be container spawns, which are excluded). The exploit amounts to stashing copies of that one specific item id on that one room's floor. It is a hoarding loophole, not a real security boundary. Severity stays low.

</details>
