# PR #208 review: housing, correctness lens

Blind reviewer `housing:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I read the housing piece (f546918ec..f7310ed77) as it stands at the PR head, with a correctness lens. Files covered: the core of internal/housing, the room-layer hooks it adds, the strongbox lock gates, and the user and mob command wiring. Many risky paths hold up: persist-before-publish ordering, purchases under the registry lock, the strongbox seal, deed accounting, quarantine, and capture before a reload. I found five problems, none critical:
1. A lodging's floor has no cap, and the change turns off all three mechanisms that normally clear floors. Every command anyone runs inside a house then re-serialises every item in the house to YAML, under the world lock. The cost grows without limit, and any player can drive it (the Hollow Oak sells a home for 125 gold).
2. Nothing ever gives a unit back to its building: there is no eviction, release or inactivity expiry. With 100 units per building and 8 rooms per lodger, a building fills permanently.
3. When the loader rejects a house file, a deferred "normalize deeds" save can still write it, under a file name taken from the file's own contents. That can overwrite another player's valid house file. This only happens with a malformed or hand-edited file.
4. The bed's "rest twice as fast" bonus is a fixed +2 in a condition file. That only doubles regen while config.yaml's regen knob stays at 0.02.
5. The loader never checks that two buildings use different housing item ids or door rooms, and two lookups resolve by Go map order, so the result can change from one boot to the next. No shipped data triggers this yet.

## Coverage

Read in full at the PR head: internal/housing/{housing,registry,persistence,containers,overlay,door,purchase,offers,guests,use_items,furnishings,crafted,terms}.go, the parts of voice.go that render and validate lines, internal/rooms/routing_hooks.go, and the PR diffs to rooms/{roommanager,save_and_load,autosave_prepare,container,baubles_untaken}.go, usercommands/{go,usercommands,unlock,picklock,get,remove,use,sleep,buy,list,look}.go, usercommands/{house,housing_shop}.go, mobcommands/{mobcommands,unlock}.go, actions/{sleep,steal}.go, behaviortree/{actions_housing,conditions_state}.go, items/{items,never_bought}.go, main.go and world.go. Also checked the four shipped building YAMLs, the door room exits (each a self-loop with lock 255), unit templates and coordinates, condition 131, the config.yaml regen and loot-goblin knobs, gamelock.IsLocked, util.Save/QuarantineCorrupt/ReadLivingState, and the lock gates in put/get/lock/plant/defuse, the gmcp container payload and the aicompanion container handling.

Checked and found sound: Purchase, extension and guest-key writes all persist before they publish and run under the registry lock; the strongbox seal and per-command unlock (the defer runs before capture, and steal, plant, picklock and unlock refuse a sealed box); the routed door cannot be unlocked by the 'enter from the other side' path because it points back at its own room; capture before a data reload; login placement redirects to the door when entry is refused; extension cell clashes are rechecked by checkLinks under the lock.

Not done: the tests (*_test.go) and context.md, deliberately; the long tail of voice.go default lines; tools/housing_units.py beyond the coordinate scheme; the mapper's behaviour on overlapping house coordinates (units sit at x=0,1,2,... on one plane, so an extension east of one unit lands in a neighbouring unit's cell, but the zone is non_cartesian, so I did not report it); consent, security and unification beyond what touched correctness. Leads I set aside as too speculative: an item's shared Spec pointer or Adjectives slice between the house record and the live room could hide an in-place mutation from capture; a failed capture at room unload would let the stale record come back on reload (needs a disk error); buy words such as 'box', 'safe' or 'bench' are intercepted whenever a landlord is in the room, which only matters if a merchant shares a landlord's room (none do today).

## Findings (5)

<a id="f018"></a>
### F018 [medium] Unbounded house floors are fully YAML-compared on every command inside a house (and fully rewritten on every change) under the world lock

`internal/housing/containers.go:319` · status **confirmed** · reported as medium

Every command run by any player or mob in a unit room calls AfterUserCommand or AfterMobCommand (world.go). That calls captureHouseAt, which runs capture() on every loaded room of the house. capture() calls sameItems on the floor (Items and Stash) and on each container. When the UUID pass finds the lists equal, which is the normal case for a command that changed nothing (`look`, `say`), sameItems YAML-marshals every item on both sides (lines 328-334). It does this while holding housing.mu, and the game loop is single-threaded. Containers are capped at 10 items (ContainerSizeMax) but floors have no cap. This PR also exempts every unit room from the three systems that keep floor counts down: daily floor decay (main.go:1946, rooms.SetFloorDecayExempt with housing.IsUnitRoom), the scavengers (main.go:1940, Private: housing.IsUnitRoom) and bauble expiry (baubles_untaken.go IsPrivateRoom). config.yaml line 154 says those systems are what keep item counts down now that the loot goblin is retired. So the work done per command grows with everything ever dropped in a house, across up to 8 rooms. On top of that, every change re-marshals the whole House and fsyncs it (saveHouse), and clone() copies every item slice. The autosave backstop (CaptureOnSave inside PrepareAllInstanceWrites, under the world lock) repeats the full compare for every loaded unit room.

**Failure scenario.** A player buys a Hollow Oak home for 125 gold (no standing needed), forages or loots junk, and drops a few thousand items on the floors of their rooms. Nothing ever removes them. From then on, each command they or a guest type in the house costs about 2N reflective YAML marshals (N = items on the floors of the loaded rooms), and each drop also rewrites and fsyncs an N-item house file, all on the game loop. At tens of microseconds per marshal, a few thousand items add tens to hundreds of milliseconds to every such command. That lag hits every connected player, and a scripted or macro'd player can repeat it at will.

**Existing mechanism.** The floor-decay and scavenger systems exist to bound floor item counts. A per-room floor cap, or a dirty flag set by the item-moving paths, would avoid the full re-compare.

**Suggested fix.** Cap items per lodging floor, for example a balance knob in config.balance.go such as HousingFloorItemMax enforced on drop, throw and stash into unit rooms, as put.go does with ContainerSizeMax. Also make capture cheap when nothing changed: compare a per-room change counter or hash that the drop, get, put and stash paths bump, and only fall back to YAML compare when it moved.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
containers.go:319-336:
  for i := range a { if !a[i].Equals(b[i]) { return false } }
  for i := range a { ya, errA := yaml.Marshal(a[i]); yb, errB := yaml.Marshal(b[i]); ... }
containers.go:264-266: floor, fi, recorded := cur.floorOf(r.RoomId) ... !sameItems(floor.Items, r.Items) || !sameItems(floor.Stash, r.Stash)
containers.go:391-408 AfterUserCommand -> captureHouseAt -> Capture for every loaded room of the house
rooms.AddItem (rooms.go:1280) has no cap; put.go caps only containers (ContainerSizeMax: 10)
main.go:1946 rooms.SetFloorDecayExempt(... || housing.IsUnitRoom(roomId)); main.go:1940 Private: housing.IsUnitRoom; baubles_untaken.go: if IsPrivateRoom(r.RoomId) { return 0 }
config.yaml:154 "Retired 2026-09-30: the city scavengers and the daily floor decay ... keep item counts down instead."
hollow_oak.yaml:94 price: 125 (no faction)
```

- **confirmed** (medium): The claim reproduces exactly from the code as written. world.go calls housing.AfterUserCommand (line 1049) after every player command and housing.AfterMobCommand (line 1111) after every mob command. When the actor is in a unit room, captureHouseAt runs Capture on every loaded room of that house. capture() holds housing.mu and calls sameItems on each container and on the floor's Items and Stash. When the lengths and UUIDs match, which is the normal no-change case, sameItems falls through to a second loop that YAML-marshals every item on both sides (containers.go:328-334). So a command that changes nothing costs 2N marshals.  Nothing bounds N for floors: - rooms.AddItem has no cap. The only capacity knobs I found are StorageCapacity and GuildVaultCapacity, and neither applies to floors. - main.go:1946-1947 exempts unit rooms from floor decay (`|| housing.IsUnitRoom(roomId)`). - main.go:1940 marks them Private for the scavengers. - baubles_untaken.go:34 returns early for IsPrivateRoom. - config.yaml confirms the loot goblin is retired (RoomId: 0) in favour of those exempted systems.  Any change triggers saveHouse on the whole cloned House, plus clone() twice, and the CaptureOnSave room save hook repeats the compare at autosave.  This is a self-inflicted, effort-gated slowdown on the single-threaded loop, not a crash or data loss. A player has to carry thousands of items in over many trips, and the per-marshal cost is an estimate, not a measurement. That is why I keep the severity at medium rather than raising it.

</details>

<a id="f019"></a>
### F019 [medium] No path ever returns a unit to the pool: buildings can be exhausted permanently

`internal/housing/registry.go:447` · status **confirmed** · reported as medium

A unit counts as vacant only if roomHouse and held do not contain it (vacantUnitsLocked). Entries enter roomHouse through indexLocked (Purchase, useExtension, every capture) and are only cleared by resetLocked, which is followed by a reload from the same files. Nothing in the package or its callers deletes a house: there is no eviction, release, sell-back, inactivity expiry, account-deletion hook or staff command. Each building ships exactly 100 units, and one account can hold up to max_rooms = 8 of them. Units owned by abandoned or deleted accounts, or held because their file is broken, are never recycled. The loader also never removes a house whose owner account no longer exists.

**Failure scenario.** In the Hollow Oak (no standing gate, home 125 gold, extensions at 3x the running spend), about 13 lodgers who extend to 8 rooms, or 100 cheap single-room accounts, take every unit. From then on every other player gets 'Every room is let' or 'No rooms left to add', and only a hand edit of the living-state files on the droplet changes that. Players who quit for good still hold their units forever.

**Suggested fix.** Add a staff command to release a house: persist its removal first (move the file aside, never delete it, per the living-state contract), then unindex it and re-lay its rooms as vacant. Consider an inactivity rule that hands what is inside back to the owner. Either way, make the capacity limit a decision in the spec rather than a side effect.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
grep -rn "delete(houses\|delete(roomHouse\|delete(ownerHouse\|Evict\|Release" internal/housing/*.go (non-test): no matches. The same grep finds indexLocked/holdLocked, so it was capable of finding a mutation.
registry.go:447-462 vacantUnitsLocked skips any room in roomHouse or held
registry.go:466-472 indexLocked is the only writer of roomHouse besides resetLocked
unit_rooms per building: 100 for each of back_court_lodgings, hollow_oak, quillhouse and the_burrows; max_rooms: 8 in all four
```

- **confirmed** (medium): I read the code at the PR head, and the claim holds as written. vacantUnitsLocked (registry.go:447-462) skips any unit room that appears in roomHouse or held. The only writers of roomHouse are indexLocked (registry.go:466-472) and resetLocked (registry.go:122-129), and resetLocked only replaces the maps with empty ones before a reload.  Nothing in internal/housing deletes from houses, roomHouse, ownerHouse or held. A grep for delete( across the package finds only r.Nouns and r.Exits deletes, in overlay.go and furnishings.go. The player command surface in internal/usercommands/house.go has only info/guests/list, revoke/kick/remove (which drop a guest) and leave (which takes a guest's key off someone else's lodging). None of them lets an owner give up a unit. No staff or admin path, and no users-package hook, calls into housing to release a house.  When loadHouse rejects a file it holds that file's rooms rather than freeing them. Nothing in the loader checks whether the owner account still exists; it only rejects owner_user_id <= 0.  The shipped building YAMLs set max_rooms: 8, and unit_rooms lists are present for each building. So the only way a unit goes back to the pool is a hand edit of the living-state files.  This is a missing lifecycle feature, not a crash. Severity depends on population, but the permanent lockout without a staff remedy justifies medium.

</details>

<a id="f072"></a>
### F072 [low] Bed 'twice as fast' is a fixed +2 recovery statmod that only doubles regen at today's PlayerHealthRegenPct

`internal/actions/sleep.go:22` · status **confirmed** · reported as low

Condition 131 (Sleeping in a Bed) adds healthrecovery, staminarecovery and convictionrecovery of 2. HealthPerRound adds statmod/100 to the config knob PlayerHealthRegenPct, so the bed adds a flat 0.02. The condition file says '2 doubles it', and the landlord sells the bed as 'rest twice as fast'. That holds only while PlayerHealthRegenPct, PlayerStaminaRegenPct and PlayerConvictionRegenPct stay at 0.02, and only for a sleeper with no other recovery statmods from gear. The balance number lives in content and is coupled to a config knob by a comment.

**Failure scenario.** An owner retunes PlayerStaminaRegenPct in config.yaml, for example back to 0.01 (it was 0.01 before tuning 39.2). Bed sleep then triples stamina regen instead of doubling it, while the player-facing text still says twice as fast. A sleeper wearing +5 recovery gear gets about 1.3x, not 2x.

**Existing mechanism.** configs.GetBalanceConfig() is the home for balance numbers; SleepRegenMultiplier is already applied to sleeping regen this way in resources.go

**Suggested fix.** Add a balance knob such as BedSleepRegenMultiplier to config.balance.go and config.yaml, and apply it in HealthPerRound, StaminaPerRound and ConvictionPerRound when the sleeper has the bed condition (as SleepRegenMultiplier is applied), instead of a fixed statmod.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
_datafiles/world/dogmud/conditions/131-sleeping_in_a_bed.yaml: "players regen 0.02 of each pool a round (PlayerHealthRegenPct ...), so 2 doubles it"; statmods healthrecovery: 2 ...
internal/characters/resources.go:381-386: pct := float64(b.PlayerHealthRegenPct) ... pct += float64(c.StatMod("healthrecovery")) / 100.0
config.yaml:1198-1200 PlayerHealthRegenPct/PlayerStaminaRegenPct/PlayerConvictionRegenPct: 0.02
furnishings.go:168 o.Note = `A bed. Sleep in it to rest twice as fast`
```

- **confirmed** (low): The claim matches the code. Condition 131 adds a flat +2 to each recovery statmod. HealthPerRound, StaminaPerRound and ConvictionPerRound (internal/characters/resources.go) each start from the config knob Player*RegenPct, add StatMod/100, multiply by the raw pool max, and then apply SleepRegenMultiplier to the sum. So the bed doubles the pre-multiplier regen only when the knob is exactly 0.02 and the sleeper has no other recovery statmods. The arithmetic in the claim checks out: - With +5 recovery gear: (0.02 + 0.05 + 0.02) / (0.02 + 0.05) is about 1.29x. - With the knob retuned to 0.01: (0.01 + 0.02) / 0.01 = 3x.  The PR backs the "twice as fast" promise with this content number. The promise appears in about 8 player-facing strings: the condition description, furnishings.go lines 168, 251 and 374, voice.go lines 25 and 87, and hollow_oak.yaml. The only link to the knob is a YAML comment. A multiplier would be the existing pattern here, either a balance knob composed like SleepRegenMultiplier or a scale on base. Nothing guards the coupling, but the shipped values make it correct today. That makes it a latent tuning and maintainability problem, not a live bug, so low severity holds.

</details>

<a id="f073"></a>
### F073 [low] Housing item ids and door rooms are not unique across buildings; lookups then depend on map iteration order

`internal/housing/guests.go:33` · status **confirmed** · reported as low

Building.Validate checks that the seven housing item ids are distinct only within one building (seenItem is per building). validateBuildingsAgainstWorld checks unit rooms across buildings but not item ids or door rooms. buildingForKey returns the first building in a range over the buildings map, which Go randomises. BuildingForDoor returns doorBuilding[roomId][0], and that slice is filled by ranging over the loaded map in rebuild(), so its order also changes per boot. `visit` and useGuestKey rely on these lookups.

**Failure scenario.** A content author reuses an item id across two buildings, for example one shared guest key (57) or a shared container deed. A guest key bought for a house in building A is resolved against building B on roughly half of boots and refused ('The lock does not know the lodging this key was cut for'). If two buildings share a door room, `visit <name>` uses the wrong building's door exit on some boots. No current shipped data triggers this; it is a latent authoring trap, since the loader accepts the data.

**Existing mechanism.** validateBuildingsAgainstWorld already keeps a cross-building map (unitOwner) for unit rooms

**Suggested fix.** In validateBuildingsAgainstWorld, panic when two buildings share any housing item id, as is already done for unit rooms. Either forbid two buildings sharing a door room, or sort doorBuilding entries by building id.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
housing.go:202-208: seenItem := map[int]bool{} ... per-building only
guests.go:33-42: for _, b := range buildings { if b.GuestKeyItemId == itemId { return *b, true } }
guests.go:45-53: for _, id := range doorBuilding[roomId] { ... return *b, true }
registry.go:96-103: for id, b := range loaded { ... doorBuilding[bb.DoorRoom] = append(doorBuilding[bb.DoorRoom], id) }
```

- **confirmed** (low): The claim checks out against the code as written. It is a latent authoring trap; no shipped data triggers it. Building.Validate deduplicates the seven housing item ids only inside one building, because seenItem is a fresh map for each building. validateBuildingsAgainstWorld keeps a cross-building map for unit rooms only, and it checks no item id or door room across buildings. Both lookups then depend on Go's random map iteration order: - buildingForKey ranges over the buildings map and returns the first building whose GuestKeyItemId matches. - BuildingForDoor returns the first id in doorBuilding[roomId]. rebuild() fills that slice by ranging over the loaded map, so its order changes from boot to boot.  UseItem routes every guest key through buildingForKey before anything else. If two buildings share a guest key item id, useGuestKey can get the wrong building, and then either `room.RoomId != b.DoorRoom` fails ("Present the key at the door of ...") or the ownerHouse lookup for {wrongBuilding, host} fails ("The lock does not know the lodging..."). Visit uses BuildingForDoor for the door exit and route. doorBuilding being a slice, and ItemIssuedHere iterating every building at a door, suggest shared doors were anticipated. BuildingForDoor still picks [0] without a stable order.  The four shipped buildings all use distinct item ids (55-59/75-76, 60-64/77-78, 65-69/79-80, 70-74/81-82) and distinct door rooms (5625, 6570, 6671, 6773), so today it does not reproduce. Housing is new in this PR: registry.go is absent at c696c117a. The fix is to extend the existing unitOwner pattern in validateBuildingsAgainstWorld to item ids, and to door rooms or sort doorBuilding. Severity stays low.

</details>

<a id="f074"></a>
### F074 [low] Deferred deed-normalization save runs on rejected house files and can overwrite another lodger's house file

`internal/housing/persistence.go:185` · status **confirmed** · reported as low

loadOneHouse registers `defer func(){ if h.DeedsIssued != before { saveHouse(h) } }()` right after the building check, before the remaining checks: entry room vs file name, owner present, rooms are units of the building, rooms not already owned, checkLinks, duplicate owner. Each of those checks returns through reject(), and reject() is documented as leaving the file 'where it is for staff to repair'. The deferred save still runs. saveHouse writes to housePath(h.BuildingId, h.EntryRoom()), the entry room from the file's contents, not the file being loaded. normalizeDeeds raises DeedsIssued whenever deedsUsed > DeedsIssued, or for a legacy file with RoomsPaid > PricePaid, so a malformed file with extra rooms trips it.

**Failure scenario.** Staff hand-edit housing/back_court_lodgings/6470.yaml and leave room_ids: [6475, 6470, 6476] with deeds_issued: 0. 6475 is a legitimate house of another account in 6475.yaml. On boot, 6470.yaml is rejected ('entry room 6475 does not match the file name') and its rooms are held. The deferred save then writes this bogus house over 6475.yaml. Because the directory listing was taken first, 6475.yaml is read next in the same loop and indexed as the bogus owner's house. The real owner's record (guests, containers, floor items) is gone, and every later capture persists the loss. Even when there is no collision, a rejected file is rewritten, or a second copy is created under a new name that a later boot indexes while the original stays held.

**Suggested fix.** Run normalizeDeeds and its save only after every check has passed, just before indexLocked. Never write a rejected file. Write only to the path the file was loaded from, never to a path derived from its contents.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
persistence.go:183-191:
  before := h.DeedsIssued
  h.normalizeDeeds(*b)
  defer func() { if h.DeedsIssued != before { if err := saveHouse(h); ... } }()
persistence.go:192-211: the entry-room, owner, unit, already-owned, checkLinks and duplicate-owner checks each `return reject(...)` after the defer is registered
persistence.go:60: return util.Save(housePath(h.BuildingId, h.EntryRoom()), b)
housing.go:494-496: if used := h.deedsUsed(b); h.DeedsIssued < used { h.DeedsIssued = used }
```

- **confirmed** (low): The code reads exactly as the claim says. In internal/housing/persistence.go, loadOneHouse sets up the deferred saveHouse at lines 185-191. That happens after the building checks (173-179) and before the entry-room, owner, unit, already-owned, checkLinks and duplicate-owner checks (192-211). Each of those returns through reject(), and the comment at 158-160 says a rejected file "stays where it is for staff to repair." The deferred closure still runs on a reject, and nothing in it checks whether the house was indexed. Its condition is only h.DeedsIssued != before.  normalizeDeeds (housing.go:485-497) raises DeedsIssued to deedsUsed. deedsUsed is len(RoomIds) minus the tier's room count, so any file with more rooms than its tier and a low deeds_issued trips it.  saveHouse writes to housePath(h.BuildingId, h.EntryRoom()), and EntryRoom() is RoomIds[0] (housing.go:453-457). That is the entry room from the file's contents, not the name of the file being loaded. So for a file rejected with "entry room X does not match the file name", the save writes to X.yaml. If another house lives there, it is overwritten.  The directory entries are read and sorted before the per-file loop (persistence.go:81-87), so 6475.yaml is processed after 6470.yaml. It is then read with the bogus content. Its entry room now matches its file name, and the earlier reject only put rooms in `held`, which loadOneHouse never checks (only roomHouse and ownerHouse). So the bogus house can be indexed under the wrong owner, as the scenario describes.  Every step of the failure is statically determined; none depends on runtime behavior. The trigger needs a malformed or hand-edited file (an entry mismatch plus surplus rooms), so low severity is right, but the consequence when it fires is silent loss of another player's house record. A secondary effect also holds: other rejects rewrite the rejected file in place, which goes against the "left for staff" contract, though that rewrite is mostly benign.

</details>
