# PR #208 review: cross cutting, security lens

Blind reviewer `cross_cutting:security`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed PR #208 statically at head 3ce674451, looking for exploits that only appear when its systems combine. Two cross-system bugs are confirmed and there is one low inconsistency.

1. **(high)** Merchant chests can be emptied without the theft being recorded. A companion is allowed to put things into a merchant's chest, and a full chest spills a random item onto the floor. That item never gets its stolen mark, so the player can sell it anywhere at full price, even back to the merchant it came from. The PR closed this hole for players but not for mobs.
2. **(medium)** A companion can butcher a carcass the player has already skinned or cut parts from, because the mob salvage path ignores how much of the carcass is used up. This gives a second set of leather and sinew from one kill. The meat it produces also never spoils, and the job needs no knife and wears no tool.
3. **(low)** A fence refuses to buy back goods stolen from its own chest at any time. An honest merchant buys its own stolen goods back once they have cooled.

Several suspected exploits were checked and do not hold up:
- Rift veins and tree stands are blocked, and no currently instanced zone has a biome that can be mined or chopped.
- Floor decay and scavengers skip player homes and ephemeral rooms.
- Recall and teleport into a house are stopped by the housing entry guard (`rooms.checkEntry` in `MoveToRoom`).
- Strongboxes stay locked to companions until the moment they act, so a companion cannot take from them.
- Lodging-house deeds and keys are refused by every sell path, including a companion's.
- A shop drops an item's grade when it shelves it, and its buy price is capped, so buying and reselling makes no profit.
- Repairing at a merchant costs more than the extra sale price it brings (shipped `RepairCostRatio` 0.5 against `WornSellPenalty` 0.6 times the 0.5 buy ratio).
- Grade and spoilage fields keep stacks apart, and spoilage runs on an absolute clock, so storing goods does not stop it.
- Hot stolen goods cannot be salvaged or crafted, and a companion cannot take from a merchant chest.

## Coverage

I read these in full at the PR head: actions/sell.go (diff), sell_stolen.go, merchant_chest.go, repair.go, gradable.go, salvage.go (salvageCorpse and storeRecovered), mine.go and chop.go (rift guards), the craft.go diff, and harvest.go ResolveHarvest. Also items/quality.go, grade_effects.go, stacking.go, spoilage.go and never_bought.go; shops/buyrules.go and shopinventory.go (diffs); merchantchests/restock.go; housing/containers.go, crafted.go, door.go, the start of use_items.go and the eject code in guests.go; rooms/floor_decay.go and the entry-guard and NoRecall code in routing_hooks.go; the MoveToRoom entry check in roommanager.go; the rifts/lost.go forfeit and find logic and the portal private-room exclusion; mining/vein.go; scavenger daily reset and pickup; the aicompanion sell, put and take_from actions, finds.go, the roster.go handOver code and the bonding code in commands.go; mobcommands put.go and the salvage.go diff; the usercommands get.go, put.go, go.go and storage.go diffs; and the fold anchor and fold recall hooks.\n\nShipped balance numbers come from `git show HEAD:_datafiles/config.yaml`.\n\nNot covered in depth: the rest of rifts (runtime and generation, hunter, puzzle rewards, keys), roomlife, lookdetail and npcidle, the companion consent and API paths (another reviewer's lens), the housing purchase, offer and extension accounting, gear-wear hooks, the forage overlay in rift rooms, and whether a held companion (HoldPosition while its owner sneaks) can stay in a host's house after its owner is ejected (plausible but unverified, so not reported). I made no runtime or test verification, as the rules required.

## Findings (3)

<a id="f065"></a>
### F065 [medium] Companion or mob corpse salvage ignores what a harvest already took: skin then salvage pays twice, and the meat never spoils

`internal/actions/salvage.go:136` · status **confirmed** · reported as medium

The PR added harvest state to carcasses: Corpse.Skinned, Butchered and HarvestedParts. It routes the player's salvage command away from any carcass that has a harvest table (usercommands/salvage.go startCorpseSalvage tells the player to skin or butcher instead). The shared action salvageCorpse, which mobs and AI companions use (mobcommands.Salvage, then actions.Salvage with TargetCorpse), was not updated. It picks any non-prunable corpse with salvage returns, never checks CarcassTable, Skinned, Butchered or HarvestedParts, pays the full corpse-salvage table, and removes the corpse. A harvest only removes a carcass once it is fully used up (skinned and butchered). So a carcass that has only been skinned, or only had targeted parts cut, stays in the room and a companion can then salvage it. The companion's butcher pastime and its `salvage` action pick it as eligible: butcherable() checks only Prunable, MobId, HasLoot, LootAllowed and salvage returns. storeRecovered also creates items with items.New and never stamps CraftedRound. Salvaged raw meat therefore never spoils, while butchered meat is stamped (harvest.go `itm.CraftedRound = now`) and rots. The salvage also needs no knife, runs no gather contest and wears no tool.

**Failure scenario.** 1. The player kills a wolf or deer (the species table falls back to corpseSalvageTable 'animal', which returns raw meat x1, leather strip x2 and sinew x1) and types `skin <corpse>`. They get a graded pelt, and the corpse is marked Skinned but not Butchered, so it stays.
2. Their AI companion (for example Mara, whose profile has the butcher pastime) idles, and butcherHere picks this corpse because butcherable() ignores Skinned. She issues `salvage <mobId>:<round>`, and salvageCorpse pays raw meat, 2 leather strips and sinew, then deletes the corpse.
3. Result per kill: a hide plus leather strips and sinew, where skinning and butchering would give only the skin and butcher sections. The raw meat she produces is unstamped and never spoils, which bypasses the spoilage system the PR adds. Targeted `harvest <corpse> <part>` followed by companion salvage double-dips the same way.

**Existing mechanism.** actions.CarcassTable / SectionEntries and the Corpse.Skinned/Butchered/PartTaken state the PR itself added. salvageCorpse (and butcherable) should consult them the way startCorpseSalvage does.

**Suggested fix.** In salvageCorpse, skip any corpse where CarcassTable(c) has a non-empty table (or at least any that is Skinned, Butchered or has HarvestedParts), so the mob and companion path matches the player gate. Better still, have the companion butcher pastime issue `butcher <corpse>` through ResolveHarvest so it pays tool, grade and spoilage like a player.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
salvage.go:121-143 salvageCorpse loop: filters Prunable, MobId<=0, the target id and round, and `len(crafting.LookupCorpseSalvageForMob(...)) > 0`. No Skinned, Butchered, HarvestedParts or CarcassTable check. Line 186: `room.RemoveCorpse(target)`. storeRecovered (salvage.go:408-426): `newItem := items.New(matSpec.ItemId); char.StoreItem(newItem)`, with no CraftedRound set.
harvest.go:391-401 marks Skinned/Butchered/HarvestedParts; line 442 removes the corpse only when `corpse.Spent() && !corpse.HasLoot()` (Spent = Skinned && Butchered).
usercommands/salvage.go: `if table, _, ok := actions.CarcassTable(corpse); ok && !table.Empty() { ...try skin/butcher...; return }`, a player-only gate.
modules/aicompanion/finds.go:123-150 butcherable(): no Skinned or Butchered check.
items/spoilage.go: Spoils() returns false when CraftedRound == 0.
```

- **confirmed** (medium): The claim holds as the code is written. salvageCorpse (internal/actions/salvage.go:116-186) picks a corpse based only on Prunable, MobId<=0, the optional mobId:round target and whether LookupCorpseSalvageForMob returns anything non-empty. It also refuses a corpse that still has loot. It never reads Skinned, Butchered, HarvestedParts or CarcassTable. It pays the full RollSalvageReturnsFromSpec and then calls room.RemoveCorpse. Harvesting edits the carcass in place (corpse := &room.Corpses[idx]) and removes it only when corpse.Spent() (Skinned && Butchered) && !HasLoot (harvest.go:442). So a carcass that has only been skinned, or has only had targeted parts cut, stays in the room and is still a valid salvage target. The only CarcassTable gate is in the player path (usercommands/salvage.go:163). The mob path (mobcommands/salvage.go to actions.Salvage with TargetCorpse) has no such gate. The companion calls it through butcherable() (finds.go:123-150), and three call sites use it: autonomy.go:346 and 430 through butcherHere, and actions.go:542/548 for a directed salvage. butcherable() does not check harvest state either. storeRecovered (salvage.go:408-426) uses items.New with no CraftedRound stamp, and items/spoilage.go says CraftedRound==0 never spoils. The PR gives raw_meat (40014) and wild_hare_meat (40064) a SpoilAfter, and harvest.go stamps itm.CraftedRound = now, so meat from companion salvage escapes spoilage while butchered meat rots. The companion's ability to salvage an untouched carcass existed before the PR. What the PR adds is the double-dip after a partial harvest and the spoilage bypass, both reachable through normal companion play. This is an economy and consistency defect, not a security hole, but medium is a fair severity.

</details>

<a id="f066"></a>
### F066 [medium] Mob put into a merchant chest spills its goods onto the floor unmarked, so they come out clean

`internal/mobcommands/put.go:104` · status **confirmed** · reported as high

The PR blocked players from putting things into a merchant chest. Its own comment in usercommands/put.go:61-67 says why: filling the chest past ContainerSizeMax spills a random item onto the floor, which gets its goods out without the take contest and without marking them stolen. The mob sibling, mobcommands.Put, was left without that guard. It only checks container.Lock.IsLocked(), then runs the same overflow code (lines 104-122), which moves a random container item to room.Items. Restocked goods carry StolenFrom/StolenFromMob but StolenAt=0, and only two paths ever set StolenAt: usercommands/get.go:579 (taking from the chest) and steal.go:631. Picking an item up off the floor (TakeFloorItem / a plain get) never calls MarkTaken. So a spilled merchant good is IsMerchantGoods() but not IsStolen(). Every stolen-goods rule tests IsStolen(), so none of them apply: heat, honest-merchant refusal, the hot-goods block on storage, salvage and crafting, and recognition by the owner or a guard. The companion's `put` action (modules/aicompanion/actions.go:453-464) only checks canPartWithItem and that the container exists and is still there. It has no IsMerchantChest guard, while its `take_from` sibling (loot.go mobCompanionTakeout) does have one.

**Failure scenario.** 1. The player picks the merchant chest's lock. The picklock contest is watched, but once the chest is open it stays open until the RelockInterval passes.
2. The player hands the AI companion a pile of cheap junk and asks her to stow it in the chest. Her `put` action issues `put <item> chest` through mobcommands.Put. No merchant-chest check runs, and no WatchMerchantChest observer contest runs either.
3. The chest holds the restock (items_max 3) plus the junk. Once it passes ContainerSizeMax (shipped 10), each further put drops a random item on the floor, often one of the merchant's goods.
4. The player types `get <item>` from the floor. TakeFloorItem never marks it, so it has StolenAt=0 and IsStolen() is false.
5. The player sells it to the same merchant at the honest price, or banks or salvages it, with no heat and no chance of being recognised. Repeating the junk cycle (put, the junk spills, pick it up, put again) empties every chest good. The junk itself is also destroyed at the next Restock (c.Items = nil).

**Existing mechanism.** actions.IsMerchantChest, already used for the player Put guard and in the companion takeout. The same guard belongs in mobcommands.Put (or the overflow spill should run MarkChestGoodsTaken on what it drops).

**Suggested fix.** Add `if actions.IsMerchantChest(room, containerName) { return true, nil }` to mobcommands.Put, matching usercommands.Put. For defence in depth, the overflow spill should never drop an IsMerchantGoods item, or should mark it taken by the putter.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
usercommands/put.go:61-67 (player guard):
	// A merchant's chest takes nothing from strangers. Filling one past
	// ContainerSizeMax spills a random item on the floor, which would pry its
	// goods out without the take contest and without marking them stolen.
	if actions.IsMerchantChest(room, containerName) { ... return }

mobcommands/put.go:47-50 has only `if container.Lock.IsLocked() { return true, nil }`, then lines 104-122:
	if len(container.Items) > int(configs.GetGamePlayConfig().ContainerSizeMax) {
		randItemToRemove := util.Rand(len(container.Items))
		oopsItem := container.Items[randItemToRemove]
		...
		room.AddItem(oopsItem, false)

grep MarkChestGoodsTaken|MarkTaken( (non-test): only get.go:579 (container path, gated by IsMerchantChest) and steal.go:631.
items.go: IsStolen() = StolenFrom != `` && StolenAt > 0.
modules/aicompanion/actions.go:453-464 `case put:` has no IsMerchantChest check; loot.go mobCompanionTakeout does (`if actions.IsMerchantChest(room, name) { return true, nil }`).
Shipped config: ContainerSizeMax: 10; merchant_chests.yaml items_max: 3.
```

- **confirmed** (medium): The claim holds up against the code as written. The PR adds merchant chests (internal/merchantchests did not exist at c696c117a) and guards the player `put` with actions.IsMerchantChest. The reason given in its own comment is exactly the overflow spill. internal/mobcommands/put.go is byte-identical to baseline and has no such guard: once the chest is unlocked, it accepts the item, and when len(container.Items) > ContainerSizeMax it moves a random container item to room.Items.  The only thing that diverts the spill is the room.SpawnInfo check (spn.Container == containerName). Restock (merchantchests/restock.go:145-170) fills the chest directly with items.New and does not go through SpawnInfo, so that check does not protect the goods. Restocked goods get StolenFrom/StolenFromMob only, with StolenAt still 0. MarkTaken is called (outside tests) only from MarkChestGoodsTaken, which only usercommands/get.go:579 calls on the container path, and from steal.go:631. The floor pickup (actions.TakeFloorItem via usercommands/get.go:52) never marks. IsMerchantGoods() has no consumers outside items.go itself, so every stolen-goods rule (sell.go:133/355, etc.) keys on IsStolen(), which is false for a spilled good.  The companion `put` action (aicompanion/actions.go:453-464) checks only carried, canPartWithItem, and that the container exists and is still there. It has no IsMerchantChest check, unlike loot.go:146 on the takeout side. So an owner-directed companion can trigger the spill. No WatchMerchantChest contest runs on the mob path.  I lowered severity from high to medium for these reasons: - The exploit still requires first winning the watched picklock contest. - It requires an AI companion. - It requires about 8 or more junk items, since ContainerSizeMax is 10 and there are 1 to 3 goods. - Each restock yields only 1 to 3 goods.  It is still a real bypass of the PR's own stated invariant, and the fix belongs in the shared mob path.

- **confirmed** (medium): I found nothing that blocks this path, so the claim holds. The player put has an IsMerchantChest guard, but mobcommands.Put only checks container.Lock.IsLocked() and then runs the overflow spill.  Two things looked like possible protections. Neither one applies. - **SpawnInfo protection does not cover the goods.** The spill skips items that SpawnInfo places in the container. Merchant goods come from merchantchests.Restock (items.New, StolenFrom/StolenFromMob set, c.AddItem), not from room.SpawnInfo, so they are not protected. - **Nothing marks spilled goods.** MarkTaken/MarkChestGoodsTaken is called only at get.go:579 (taking from a container) and steal.go:631. IsMerchantGoods is not checked anywhere else outside items.go. A spilled good therefore has StolenAt=0, and IsStolen() is false.  The companion route reaches this code: - scene.go adds every container that is not hidden, and syncContainer sets merchant chests to Hidden=false. - The `put` case in actions.go:453-464 has no merchant-chest guard. m.issue calls mob.Command, which dispatches to mobcommands.Put through mobcommands.go:70. - loot.go:146 has the guard on the take side only.  Shipped values also allow it: ContainerSizeMax is 10 and items_max is 3, so roughly 7 or more junk puts start the spill.  I lowered the severity from high to medium. The exploit needs a successful picklock contest, which is watched, and an opted-in AI companion. Each spill picks a random item, so some spills drop junk instead of goods. Even so, it launders merchant goods so they escape every IsStolen rule, and it repeats.

- **confirmed** (medium): The bypass is real and I could not refute it. The PR added an IsMerchantChest guard to the player put path (usercommands/put.go:61-67) and to the companion's take-out path (loot.go:146). It left the mob put sibling (internal/mobcommands/put.go) with only a lock check, and the PR did not touch that file. The overflow code in mobcommands/put.go:104-122 has one carve-out that could have saved the goods: an item whose ItemId matches a room.SpawnInfo entry for that container stays in. Merchant goods do not come from SpawnInfo. merchantchests.Restock fills c.Items directly with items.New, so that guard does not protect them.  The companion's `put` action (actions.go:453-464) is in ownerDrivenOnly, so her owner can direct it. It checks only canPartWithItem and that the container exists in the scene. scene.go:197 lists every room container, the merchant chest included. Restocked goods have StolenFrom and StolenFromMob set but StolenAt=0. Only get.go:579 (the container path, gated by IsMerchantChest) and steal.go:631 call MarkTaken. IsStolen() requires StolenAt>0, and every downstream rule keys on IsStolen(): the honest-sale refusal in sell.go, the hot-goods checks in offer.go, recognition in stolen_bauble.go:271, and goods heat in baubles/goods.go:24. A spilled good picked up off the floor is therefore clean.  The 'high' rating overstates the impact, for these reasons: 1. The chest must first be picked open. Picking is itself watched (WatchMerchantChest with MerchantChestPick), and a thief who is caught has the chest slammed shut and relocked. 2. The exploit needs the opt-in AI companion module plus at least 8 junk items to push the chest past ContainerSizeMax 10. 3. Each spill drops a random item, so a given drop is only sometimes a good. The payout is capped at 1 to 3 goods per chest per restock (items_min 1, items_max 3). 4. Once the lock is open, the thief could take the same goods anyway with `get`. The exploit saves them the take contest and the heat; it does not open goods that were otherwise out of reach.  So this is a real consistency hole in a theft protection, an incomplete sibling guard that should be fixed, but the economic damage is bounded. Medium.

</details>

<a id="f130"></a>
### F130 [low] An honest owner buys its own stolen goods back once cooled; a fence never does

`internal/actions/sell.go:355` · status **confirmed** · reported as low

Stolen-goods handling treats the robbed merchant differently depending on whether it is a fence. fenceInRoom and sellStolenToFence refuse goods from the fence's own chest whether they are hot or cold. sellOneToMerchant, by contrast, refuses an honest merchant's own goods only while they are hot in the area (stolenGoodsHotHere). After BaubleStolenHeatHours the original owner (item.StolenFromMob == mob.MobId) buys them at the ordinary price and shelves them for resale, and the theft marks are lost because the shelf keeps only the item id. The "That's mine!" refusal text exists only for the hot case. The goods are still IsStolen() (StolenAt is never cleared), so this is a rule inconsistency rather than a new gold source: any other honest merchant pays the same.

**Failure scenario.** A thief lifts an item from Merchant X's chest, waits out the heat (carrying it unrecognised, or keeping it in a housing container, which unlike bank storage accepts hot goods), then types `sell <item>` to X itself. X pays the full honest price and restocks its own stolen item, while a fence robbed the same way would refuse forever.

**Existing mechanism.** StolenGoodsRefusal already has the owner-specific line; apply the StolenFromMob == mob check on the honest path as the fence path does.

**Suggested fix.** In sellOneToMerchant, refuse when item.IsStolen() && item.StolenFromMob == int(mob.MobId), whatever the heat (or decide deliberately that both kinds of merchant buy back cold goods, and make them consistent).

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
sell.go:355-363:
	if item.IsStolen() {
		if IsFence(mob) { return sellStolenToFence(...) }
		if stolenGoodsHotHere(item, room) { merchantSay(room, mob, StolenGoodsRefusal(mob, item)); return 0, SellStopRejected }
	}
sell_stolen.go sellStolenToFence: `// A fence will not buy back what was lifted from its own chest.\n if item.StolenFromMob > 0 && int(mob.MobId) == item.StolenFromMob { ... SellStopRejected }`, which does not depend on heat.
```

- **confirmed** (low): The code does what the claim says. In sell.go:355-363 the honest-merchant branch refuses stolen goods only when stolenGoodsHotHere is true. That calls baubles.GoodsHotIn, which returns false once StolenAt + HeatDuration has passed, or when the item is in a different heat area. Nothing on the honest path compares item.StolenFromMob with mob.MobId, so the merchant the goods were taken from buys them back through the ordinary pricing path once they have cooled. The fence path is different. fenceInRoom (sell_stolen.go:106) skips the robbed fence, and sellStolenToFence (sell_stolen.go:121) refuses it whatever the heat. The owner-specific "That's mine!" line in StolenGoodsRefusal is reachable on the honest path only while the goods are hot. Owner recognition (recognizeGoodsOn, stolen_bauble.go) also requires GoodsHot, so a cooled item draws no reaction from its owner either.  Mitigating points: - The file header comment in sell_stolen.go says "once they have cooled, an honest merchant buys them as ordinary goods", so the honest-owner case is at least consistent with the documented rule. - The bauble sell path (sell_bauble.go) has no owner check either, so the extra check is the fence's, not missing from the honest path. - No new gold source exists, because any other honest merchant would pay the same price.  So this is a real inconsistency in the rules, not an exploit. Low severity stands. I did not verify the side claim that shelving loses the theft marks.

</details>
