# PR #208 review: craft mining wear fixes, security lens

Blind reviewer `craft_mining_wear_fixes:security`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed this piece for security and found no critical or high issues. I found no gold or item duplication, no authorization bypass, no injection that reaches other players, and no unbounded per-round work in the mining, smelting, wear and repair code. Each guard I checked held:
- A mining job runs its checks again when it finishes: vein stock, pick tier, tool present, and still in the starting room. Two players racing for the same last load cannot both get it.
- Veins live in the room's long-term data and the round counter persists across restarts, so a crash does not refill them.
- The new walk-in buy-price cap and the scrap memory hold the shop-side buy-back loop closed.
- No new metal good is stocked by an NPC shop at a price that crafting could turn into profit: smelting ratios are value-neutral, and the gear margins are ordinary crafting margins on mined inputs.

Two medium issues remain where the new wear economy does not match the existing shop rules:
1. Affixed gear skips every wear rule when sold. Broken affixed gear sells at full price, and the shop puts it back on its shelf at full price for the next buyer.
2. Merchant repair skips the shop gates (asleep, too dark) that buy and sell enforce.

## Coverage

Files read in full at the PR head (C:/tmp/dogmud-pr208-review):
- internal/actions/mine.go, internal/actions/repair.go, internal/actions/gradable.go
- internal/mining/mining.go and internal/mining/vein.go
- internal/usercommands/mine.go and internal/usercommands/repair.go
- internal/characters/gear_wear.go and internal/hooks/gear_wear.go
- internal/items/tools.go, the GetSpec/GetRawSpec/Equals parts of internal/items/items.go, and applyCondition in internal/items/grade_effects.go

Diffs read:
- the 52cfbeda4..3ce674451 diff for the salvaging and crafting branches of NewRound_UserRoundTick.go (including JobLeftBehind and AbandonCraft)
- go.go, activity.go, sell.go, shops/buyrules.go, shops/shopinventory.go (Scrap)
- combat attackresult/combat/counter/skill_moves, combat_bash, combat_fire, combat_shared_helpers, NewRound_DoCombat_unified
- gather/tools.go and gather.go, chop.go, harvest.go, carcass.go, craft.go
- the usercommands table (the `mine` and `repair` entries are not allowed in combat; `prospect` is), and the main.go loader

Content checked:
- mining.yaml, Mine Foreman Dagna's shop, and the ore, ingot, wire and gear item values
- the smelt, alloy, wire, gold gear and gem ring recipes, and which NPC shops stock the new item ids
- the shipped config.yaml wear and repair knobs

Leads checked and set aside (none are reportable):
- **Fresh veins in instanced copies.** Rooms built from a template start with no vein data, but no instanced zone today has a mineable biome (interior, dungeon or ether), so nothing is exploitable now.
- **Wear laundering by selling partly worn gear.** Gear under 85% wear is shelved as a fresh copy, but with WornSellPenalty 0.6 and RepairCostRatio 0.5 this costs more than repairing, so there is no profit.
- **Repairing gold out of nowhere.** I found no repair-for-gold loop.
- **Panic on a bad mining.yaml.** It only fails at boot; loadAllDataFiles has a single caller at startup.
- **Unbounded scrap map.** It is bounded by the number of item ids.
- **ANSI tags in `mine <word>`.** The echo goes only to the sender.
- **Ore-name probing with `mine <ore>`.** It gets around OreKnown's naming gate, but the reward on success names the ore anyway, so this is cosmetic.
- **Ingredients ruined when a crafter is moved by someone else.** A griefing angle only; I did not check whether any summon or push command exists.

Not covered:
- the help templates, docs and tests
- the remaining item, recipe and mob YAML (I only read their values)
- each changed file's context.md
- buy.go's full affixed purchase flow beyond the stored price
- salvage of worn gear, which is existing code

## Findings (2)

<a id="f054"></a>
### F054 [medium] Merchant repair skips the shop gates (asleep, too dark) that buy and sell enforce

`internal/actions/repair.go:115` · status **confirmed** · reported as medium

Repairer returns any merchant mob in the room whose ShopCraftSupport matches the trade. Repair then takes the player's gold, pays the shop and repairs the item. It never calls the gates every other shop transaction uses: actions.ShopClosedForSleep (an asleep merchant is closed) and actions.ShopSightRefusal (below the faces light band you cannot deal). Buy, sell, list and housing_shop all apply these. Merchant shops and shop lights (#207) shipped these rules just before this PR, and repair goes around them. The repair command handler (usercommands/repair.go) adds no check either.

**Failure scenario.** At night the Pothole Coulee smith is asleep and the room is dark. `buy`, `sell` and `list` all refuse ('You can't make out the goods well enough to deal' or the shop-closed line). `repair sword` still works: Smith Rusk 'takes your sword and N gold, and hands it back mended' while asleep in the dark. This breaks the shop-hours rule the lighting work enforces, and it is a second place where merchant availability is decided differently.

**Existing mechanism.** actions.ShopClosedForSleep (internal/actions/sleeping_target.go:78) and actions.ShopSightRefusal / ShopSightRefusalText (internal/actions/shop_sight.go:20-28)

**Suggested fix.** In Repair (merchant branch) and in ListRepairs, refuse with ShopSightRefusalText when ShopSightRefusal(char, room), and skip a merchant for whom TargetAsleep is true, or call ShopClosedForSleep, in the same order sell.go uses.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
repair.go:115-125: `func Repairer(room *rooms.Room, trade string) *mobs.Mob { ... for _, mobId := range room.GetMobs(rooms.FindMerchant) { if m := mobs.GetInstance(mobId); m != nil && m.ShopCraftSupport == trade { return m } } ...`
repair.go:252-282: the gold transfer and itm.Repair() with no sleep or sight check
sell.go:73-79: `if ShopSightRefusal(seller.GetCharacter(), room) { ... return SellResult{Reason: SellStopNoSight} }`
sleeping_target.go:78 `func ShopClosedForSleep(room *rooms.Room) *mobs.Mob`, called from buy.go, sell.go, usercommands/list.go and usercommands/housing_shop.go (grep); repair.go does not call either function
```

- **confirmed** (medium): The claim holds as written. `repair.go` is new in this PR: `git diff c696c117a --stat` shows it with 288 added lines, plus `usercommands/repair.go` with 23. `Repairer` (`internal/actions/repair.go:115-125`) returns the first `FindMerchant` mob whose `ShopCraftSupport` matches the trade. It never checks sleep. `FindMerchant` in `rooms.go:1742` only tests `mob.HasShop()`, so a sleeping merchant is returned. On the paid path, `Repair` (`repair.go:252-282`) checks gold, debits the player, credits the shop through `GetShopInventory`/`SaveShop`, and calls `itm.Repair()`. It never calls `ShopClosedForSleep` or `ShopSightRefusal`. The handler `usercommands/repair.go` only trims the argument and calls `actions.ListRepairs` or `actions.Repair`, with no gate. A grep for both gate functions finds them in `buy.go`, `sell.go`, `list.go` and `housing_shop.go` and nowhere in the repair files. So a sleeping merchant in a dark room will still take gold and mend the item, while list, buy and sell refuse in the same room.  `ListRepairs` has the same gap: it advertises "X will mend it for N gold" from a sleeping merchant in the dark.  This is a missing-gate and consistency defect rather than a true security hole: the player pays full price, so nothing is gained for free. It does go around the shop-hours and lighting rules that shipped in #207, which is exactly the unification concern, so medium is fair.  The self-repair path (crafting the item yourself at a workbench) has no gate either. That is arguably fine, since it involves no merchant.

</details>

<a id="f055"></a>
### F055 [medium] Broken affixed gear sells at full price and goes back on the shelf at full price, skipping the wear penalty and scrap rules

`internal/actions/sell.go:372` · status **confirmed** · reported as medium

The review fix adds two wear rules for selling gear. EvaluateBuyRules lowers the price by wear (value x (1 - wear fraction x WornSellPenalty)). Gear that is broken or badly worn (85% or more) is bought as scrap and not shelved. Affixed items never reach either rule. sellOneToMerchant prices an affixed item with affixedSellPrice, which is GetSpec().Value x BuyRatio. GetSpec's applyCondition only scales damage and mitigation, never Value, so wear does not change that price. The stocking branch then runs `if item.Affixed {... AddAffixedStock(item, item.GetSpec().Value, ...)}` before the `noResale` branch, so the exact broken item is shelved at its full undamaged Value. buy.go then sells it at that stored price (`price := e.Price`).

**Failure scenario.** A player wears an affixed sword until it is broken (Wear >= Durability) and sells it. They get the same Value x 0.5 an undamaged one would earn, while a broken plain sword of the same value earns about 0.4 x of that. The shop shelves the broken sword at its full Value. A second player then pays full price for a broken item that 'works badly' (GearBrokenMult). A seller can also colluding-alt this: dump broken affixed gear at full price instead of paying RepairCost, so the wear economy does not apply to the highest-value gear in the game.

**Existing mechanism.** shops.EvaluateBuyRules wear penalty (buyrules.go:88-91) and the noResale/AddScrap branch in sellOneToMerchant

**Suggested fix.** Apply the same wear multiplier inside affixedSellPrice (or pass the item's WearFraction through it). Send affixed items with WearFraction >= BadlyWornFraction to the scrap branch instead of AddAffixedStock. Or, at minimum, shelve them at a wear-reduced Price.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
sell.go:372-375: `if item.Affixed { sellValue = affixedSellPrice(item, shops.PricingConfigFromBalance()) } else if shopInv != nil { ... EvaluateBuyRules ...`
sell.go:116-121 affixedSellPrice: `price := int(math.Ceil(float64(item.GetSpec().Value) * cfg.BuyRatio))` (no wear term)
sell.go:435-440: `noResale := ... || item.WearFraction() >= items.BadlyWornFraction; if item.Affixed { if shopInv != nil { ... shopInv.AddAffixedStock(item, item.GetSpec().Value, c, ...)` (noResale is never consulted for affixed)
buyrules.go:88-91: the wear penalty exists only inside EvaluateBuyRules
grade_effects.go:108-123 applyCondition: scales DamageMultiplier/mitigation only, not Value
buy.go:597: `price := e.Price`
```

- **confirmed** (medium): The claim reproduces exactly as written. In sellOneToMerchant, an affixed item is priced by affixedSellPrice (GetSpec().Value x BuyRatio) before EvaluateBuyRules can run, and EvaluateBuyRules is the only place the WornSellPenalty term exists. GetSpec runs applyCondition when Wear > 0, but that only scales DamageMultiplier and mitigation/block, never Value, so wear never touches the affixed price. In the stocking branch, `if item.Affixed` comes before `else if shopInv != nil && noResale`. noResale is computed but never checked for affixed items, so a broken or badly worn affixed item is shelved through AddAffixedStock at its full GetSpec().Value. AddAffixedStock stores the Item value itself, Wear field included. buy.go then lists it at e.Price, less only the barter discount. Affixed gear does wear. GearDurability reads GetRawSpec, which honours a non-nil Spec, and returns a durability for any weapon or armour type. characters/gear_wear.go calls AddWear on strikes, armour and bows with no Affixed or IsSpecial exclusion. The gear-wear system is new in this PR (internal/characters/gear_wear.go is not in baseline c696c117a), while the affixed sell and shelf path is older code. That makes this a sibling path the PR's wear rules left out, not a pre-existing bug. Medium is right: it is an economy exploit (dodging repair costs, and a second player buys a broken item at full price), not a confidentiality or integrity hole.

</details>
