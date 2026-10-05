# PR #208 review: craft mining wear fixes, correctness lens

Blind reviewer `craft_mining_wear_fixes:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Correctness review of the mining, ores and smelting, gear wear and repair, and final review-fix commits (52cfbeda4..3ce674451), judged as the code stands at the PR head. The core mechanics hold up when read: vein refill and dig arithmetic, round-count persistence, job-bound-to-room checks, crit wear routing (no double wear between the melee path and OnCritLanded), the gear spec override (it is built from the raw spec, so the grade is not baked in) and the boot validation of mining.yaml. I checked mining.yaml against the content: every referenced item, zone and room exists, and the biome names match what the rooms actually use. Three findings survived: one medium and two low. (1) Affixed gear skips the new wear economics completely. A broken affixed item sells at its full price, is never scrapped, and goes back on the secondhand shelf at full price still broken. (2) `mine <ore>` gives a yes/no answer that names a rare ore without the Perception gate that `prospect` applies. (3) A latent infinite loop in gradableSets on the craft-completion path. It is triggered by a recipe that the crafting validator accepts, but no shipped recipe has that shape today.

## Coverage

Read in full at the PR head: internal/mining/mining.go, internal/mining/vein.go, internal/actions/mine.go, repair.go, gradable.go and toolwear.go; internal/usercommands/mine.go and repair.go; internal/characters/gear_wear.go; internal/hooks/gear_wear.go; internal/items/tools.go and grade_effects.go; _datafiles/world/dogmud/mining.yaml. Read the diffs of: NewRound_UserRoundTick.go (job-left-behind, mine completion, the craft tool recheck), NewRound_DoCombat_unified.go, combat_shared_helpers.go, combat/skill_moves.go, counter.go, attackresult.go, combat.go (swing loop: Hit and Crit aggregation), combat_fire.go, combat_bash.go, craft.go (AbandonCraft, JobLeftBehind), go.go, carcass.go, chop.go, salvage.go, gather/tools.go, gather.go, items.go GetSpec, sell.go (full stock-update section), shops/buyrules.go, shopinventory.go (Scrap), config.balance.gathering.go and config.balance.go, plus the new mobs (Dagna, the trapper and smith shop entries), room 5254 and a sample of the new items and recipes.

Checked:
- mining.yaml item, zone and room ids, and biome names, against the data files.
- Shipped config.yaml (HEAD blob) carries every legal-zero knob that changed from <=0 to <0.
- GoldChange in EquipmentChange is a notification pattern consistent with other sites (no double deduction).
- Enchantment overrides start from the raw spec, not the wear- or grade-adjusted spec.
- The round count persists (gametime.SetRoundCount).
- Room LongTermDataStore is yaml-persisted.
- The rift_run temp key is really stamped by internal/rifts.

Not covered in depth:
- The 20+ new recipe and item YAMLs individually (I sampled the masterwork pick, gold ore and ingot, smelt-gold and alloy-bronze).
- The help templates and the PATCH_NOTES and docs.
- The narration golden file.
- The tests (per blind rules).
- Whether WalkInBuyPrice vs the CalcBuyPrice cap is numerically sound for walk-in goods.

Considered and dropped as not bugs or design choices:
- Free, instant, unlimited self-repair when you can craft the item.
- Gear in the 60 to 85% wear band shelved as a fresh copy; not profitable versus repair.
- AbandonCraft ignoring Provident Hands.
- Veins reset when instance saves are wiped; same as timber.

## Findings (3)

<a id="f053"></a>
### F053 [medium] Affixed gear bypasses WornSellPenalty and the scrap rule: broken affixed gear sells at full price and is reshelved at full price, still broken

`internal/actions/sell.go:437` · status **confirmed** · reported as medium

The review fix applies two rules to worn gear. The wear sell penalty is applied only inside shops.EvaluateBuyRules (buyrules.go:89). The scrap-instead-of-shelve decision is noResale (sell.go:436). Affixed items (instance loot, the best gear in the game) take a separate pricing branch at sell.go:373: `if item.Affixed { sellValue = affixedSellPrice(...) }`. affixedSellPrice (sell.go:116-122) is `ceil(item.GetSpec().Value * BuyRatio)`, and GetSpec does not scale Value by wear (applyCondition touches only DamageMultiplier and the mitigations). The stock branch then tests `if item.Affixed` (line 437) BEFORE `else if shopInv != nil && noResale`. So a broken affixed item is never scrapped. It is pushed onto the secondhand shelf by AddAffixedStock(item, item.GetSpec().Value, ...) with its Wear intact, and buy.go:597 lists it at e.Price, the full undamaged value.

**Failure scenario.** A player's affixed sword reaches Wear == Durability (broken; GetSpec damage at GearBrokenMult 0.25). They `sell` it. The payout is Value*BuyRatio, identical to a pristine copy, so WornSellPenalty (0.6) never applies. That is cheaper than paying the merchant repair cost of Value*1.0*RepairCostRatio, about 0.5*Value. The shop shelves the same broken instance and the next buyer pays full Value for a sword that does a quarter of its damage. The sell.go:433-435 comment ('nor is broken or badly worn gear [shelved]') is false for this whole class of item.

**Existing mechanism.** shops.EvaluateBuyRules wear-penalty block (buyrules.go:88-91) and the noResale scrap branch (sell.go:445); both need to cover the affixed path too.

**Suggested fix.** Apply the same `1 - WearFraction*WornSellPenalty` factor in affixedSellPrice, or factor it into a shared helper used by both paths. Check noResale before the Affixed branch, or include it in that check, so badly worn and broken affixed gear is scrapped (AddScrap) rather than shelved. Alternatively, shelve it at a wear-reduced price.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
sell.go:373-376: `if item.Affixed { sellValue = affixedSellPrice(item, shops.PricingConfigFromBalance()) } else if shopInv != nil { ... EvaluateBuyRules ...`
sell.go:116-117: `price := int(math.Ceil(float64(item.GetSpec().Value) * cfg.BuyRatio))`
sell.go:436-441: `noResale := items.NeverResold(itemSpec) || item.WearFraction() >= items.BadlyWornFraction` / `if item.Affixed { if shopInv != nil { ... shopInv.AddAffixedStock(item, item.GetSpec().Value, c, ...)`
buyrules.go:88-91: the wear penalty is applied only here, inside EvaluateBuyRules.
buy.go:597: `price := e.Price`
```

- **confirmed** (medium): The claim reproduces from the code as written. Affixed weapons and armour do wear. Their per-instance Spec is returned by GetRawSpec, IsWearableGear covers Weapon and the armour types, Durability() falls through to GearDurability, and CritWearStriker and the armour-wear paths call AddWear on equipped gear with no Affixed exclusion. The sell price for affixed items comes from affixedSellPrice, which is ceil(GetSpec().Value * BuyRatio). GetSpec only applies applyCondition, which scales DamageMultiplier and the mitigations and BlockRating, never Value, so wear has no effect on the payout. The WornSellPenalty block exists only inside shops.EvaluateBuyRules, which sits in the `else if shopInv != nil` branch and is never reached for Affixed. The stock branch checks `if item.Affixed` before `else if shopInv != nil && noResale`, so a broken affixed item is never scrapped. It goes to AddAffixedStock with Price = GetSpec().Value, and AddAffixedStock stores the Item with its Wear intact. buy.go then lists it at e.Price less barter only. The sell.go comment that broken or badly worn gear is not shelved is therefore false for affixed gear. The whole wear system is new in this PR (items/tools.go is absent at c696c117a), so this is an incomplete sibling path introduced by the PR, not a pre-existing gap. Medium is appropriate: it is an economy exploit (it skips repair cost and launders broken gear back onto the shelf at full price), not a crash or a security hole.

</details>

<a id="f122"></a>
### F122 [low] gradableSets can loop forever on a validator-legal recipe whose output.item_id < 1, hanging the game loop on craft completion

`internal/actions/gradable.go:67` · status **confirmed** · reported as low

The fixed-point loop skips a recipe only if `ids[r.Output.ItemId]` is set or IsEnchantingRecipe(r) is true. When a recipe is graded it calls addId(r.Output.ItemId) and sets changed = true. addId returns early for id <= 0 (line 30) and never records the id, so a graded recipe with output id <= 0 sets changed = true on every pass and the loop never ends. crafting's validator (crafting.go:80) accepts output.item_id < 1 when enchant_type is set, but IsEnchantingRecipe needs BOTH enchant_type and target_type. A recipe with enchant_type, no target_type and a tool or a gradable ingredient therefore loads cleanly and is not skipped. capUngradedGradable calls gradableSets from RecipeGrade on every multi-round craft completion that consumed an ungraded gradable input, inside the user round tick.

**Failure scenario.** A content author adds an enchanting recipe and leaves out target_type (enchant_type set, tool: carving_knife). It boots fine. The next time any player finishes a craft that consumed an ungraded gradable input with a grade above fine, the round tick spins forever in gradableSets and the server hangs. No shipped recipe has this shape today (verified: every enchant_type recipe has a target_type), so the risk is latent.

**Suggested fix.** In the loop, skip any recipe with r.Output.ItemId <= 0, or set changed only when addId actually inserted a new id. Optionally make crafting.Validate require target_type whenever enchant_type is set.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
gradable.go:29-33: `addId := func(id int) { if id <= 0 { return } ids[id] = true ...`
gradable.go:67-89: `for changed := true; changed; { changed = false; for _, r := range recipes { if r == nil || ids[r.Output.ItemId] || crafting.IsEnchantingRecipe(r) { continue } ... if graded { addId(r.Output.ItemId); changed = true } } }`
crafting.go:80: `if r.Output.ItemId < 1 && r.EnchantType == "" {` (the only output-id check)
crafting.go:626-627: `return recipe.EnchantType != "" && recipe.TargetType != ""`
```

- **confirmed** (low): The claim holds when the code is read as written. In the fixed-point loop, a recipe is skipped only when ids[r.Output.ItemId] is already set, when IsEnchantingRecipe(r) is true, or when r is nil. A recipe with output.item_id 0 never gets into ids, because addId returns early for any id <= 0. So if such a recipe counts as graded (it has a tool, or one of its ingredient tags is gradable), the loop sets changed = true on every pass and never ends.  The validator lets this recipe shape load: it rejects output.item_id < 1 only when enchant_type is empty. Nothing requires target_type. The only other mention of TargetType in the crafting package is the struct field itself. IsEnchantingRecipe needs both enchant_type and target_type, so a recipe with enchant_type set and target_type missing is not skipped.  The code path is live. RecipeGrade calls capUngradedGradable on every craft. That function calls gradableSets only when the grade is above standard+1 and a consumed item is ungraded, which matches the claimed trigger. The risk is latent: the reviewer says no shipped recipe has this shape today. Low severity is right.

</details>

<a id="f123"></a>
### F123 [low] `mine <ore>` tells you whether a guess is right, revealing rare ores the Perception gate on prospect is meant to hide

`internal/usercommands/mine.go:46` · status **confirmed** · reported as low

OreKnown (actions/mine.go:115-125) gates naming a tier 3 or 4 ore (silver, gold, lake-iron, basalt-iron) on Perception, and Prospect shows 'a glinting ore you can't put a name to' when the gate fails. Mine checks the typed ore word against the real ore BEFORE the stock and pick checks. A wrong guess gets 'There's no %s here'. A right guess falls through to the worked-out message, the 'You need a pick' message or the pick-tier message. Any player, even without a pick, can type `mine gold` or `mine silver` and read the answer.

**Failure scenario.** A low-Perception character with no pick stands in an Eastern Highlands mountain room and prospects: 'a glinting ore you can't put a name to'. They type `mine gold` and get 'There's no gold here'. They type `mine silver` and get 'You need a pick to mine.' The ore is now identified as silver, bypassing OreKnown.

**Existing mechanism.** actions.OreKnown

**Suggested fix.** When !OreKnown, give the same reply for any ore word (for example, treat any word as matching, or say 'You can't tell what this rock carries'). Or move the name check after the pick checks and make it OreKnown-aware.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
usercommands/mine.go:46-50: `if want := ...; want != `` && !mineWordMatches(want, ore.Name) { user.SendText(... `There's no %s here...`, want)); return }` runs before mine.go:51 (stock) and mine.go:55-58 (no pick).
actions/mine.go:157-161: Prospect hides the name unless OreKnown.
```

- **confirmed** (low): The claim reproduces from the code as written. Both files are new in PR #208. In usercommands/mine.go, `mine <word>` compares the typed word to the real `ore.Name` (lines 46-50) before any OreKnown check, and before the stock check (line 51) and the pick check (lines 55-58). A wrong guess always gets "There's no %s here". A right guess goes on to a different message (worked out, "You need a pick to mine.", the pick-tier message or the job start), so the reply works as a yes/no oracle on the ore's name.  The PR clearly means to hide the name. Prospect shows "a glinting ore you can't put a name to" when OreKnown fails (actions/mine.go:157-161), and the pick-tier branch in usercommands/mine.go:61-65 deliberately picks a generic message when OreKnown fails. The name check at line 46 skips that guard.  Two details make the leak worse than stated: 1. `mineWordMatches` uses `strings.Contains`, so single letters work too. For example, `mine g` separates gold from silver. 2. Only four ores are tier 3 or above (lake-iron, basalt-iron, silver, gold), and Prospect's "hard, stubborn rock" line already says the ore is tier 3 or 4. So two or three guesses are enough.  The only requirement is that the room is lit well enough to see (CanSeeClearly). No pick is needed.  Severity stays low. This is an information leak about which ore a seam holds, not an exploit. A successful mine hands over the named ore item anyway, so the protection only holds for a player who has not yet mined the seam.

</details>
