# PR #208 review: craft mining wear fixes, unification lens

Blind reviewer `craft_mining_wear_fixes:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Unification review of the mining, gear-wear/repair and review-fix commits (52cfbeda4..3ce674451), judged at PR head 3ce674451. The biggest problem is merchant repair in internal/actions/repair.go. It finds its own merchant and skips the gates that list, buy and sell share: the sleeping-merchant gate and the lighting 5b dealing gate (ShopSightRefusal). It also names the merchant and the player in narration that bypasses the name-hiding path. Self-repair is instant and free, skips the timed-activity system, and never wears the tool. The craft path that RepairRecipe copies its gates from does wear the tool.

internal/mining is a near line-for-line clone of internal/timber, which an earlier commit of this PR added, rather than one shared regrowing-node type. The same applies to RoomVein vs RoomStand and to the give-or-drop closure, now in three gather actions. The rift-room guard reads a raw `rift_run` temp-data string in two places. The baseline Room.IsEphemeral() check would also cover instanced zones. ShieldPtr re-implements HasAnyShield's predicate. The scrap price memory is a second, faster-decaying counter beside the existing overstock decay. Rare-find clamps and the ore-recognition threshold are hardcoded Go literals rather than balance knobs.

Correct reuse, worth crediting:
- gather.Roll goes through crafting.RunSalvageContest, the shared contest path, and pays messaging.SightMult once.
- mine/prospect/chop use messaging.CanSeeClearly; ResolveMine narrates through messaging.SendTrio, which hides names.
- Mining jobs ride the existing activity Salvaging state machine with a prefix key, like chop and carcass. JobLeftBehind was added consistently to every room-bound start site (craft, enchant, carcass, chop, mine, corpse salvage).
- Every new balance knob is declared in config.balance.go with defaults in validateGathering and is surfaced in _datafiles/config.yaml. The legal-zero knobs follow the negative-only default idiom.
- RepairCost uses shops.GradedValue. The buy-price cap uses CalcBuyPrice with PricingBaseline (the one pricing baseline).
- AbandonCraft uses crafting.ConsumeIngredients, and RepairRecipe reuses StationSatisfied and ToolSatisfied.
- applyCondition mirrors applyGrade's field set and scaleInt, so wear and grade scale the same stats.
- Broken tools are filtered inside gather.asTool, so every tool consumer sees them consistently.
- mining.LoadDataFiles follows the timber loader contract (fail-loud on broken content, World validators).

## Coverage

I read in full: internal/mining/mining.go, internal/mining/vein.go, internal/actions/mine.go, internal/actions/repair.go, internal/usercommands/mine.go, internal/usercommands/repair.go, internal/characters/gear_wear.go, internal/hooks/gear_wear.go and internal/actions/gradable.go. I read the diffs in this range for internal/items/tools.go, internal/items/grade_effects.go, internal/items/items.go, internal/shops/shopinventory.go, internal/shops/buyrules.go, internal/actions/sell.go, internal/actions/craft.go, internal/gather/*, internal/combat/* (skill_moves, counter, attackresult), internal/actions/combat_bash.go and combat_fire.go, the NewRound hooks, internal/state/activity, the go/chop/craft/salvage command changes, config.balance.go, config.balance.gathering.go, the _datafiles/config.yaml knob additions and the usercommands registration. For comparison I read timber/stand.go, chop.go RoomStand, harvest.go's rare-chance block, gather.go, and these baseline mechanisms: shop_sight.go, sleeping_target.go, sell.go resolveMerchant, rooms SendTextVisual*, Room.IsEphemeral/OwnedChunk.AddRoom, HasAnyShield and overstock_decay. I checked the instanced zones' biomes against mining.yaml pools.

Not reviewed in depth:
- The help templates, item, recipe and smelting YAML content, and docs.
- wilderness_trades_content_test.go and the unit tests.
- internal/actions/toolwear_test.go and gradable_test.go.
- Whether the crafting smelt recipes are balanced.
- The combat crit plumbing beyond the OnCritLanded call sites.

Note: timber, gather, rifts and IsGearType are not in baseline c696c117a. Earlier commits of this same PR introduced them, so the timber/mining duplication finding is intra-PR divergence rather than bypass of a shipped mechanism.

## Findings (7)

<a id="f056"></a>
### F056 [medium] Merchant repair bypasses the shared sleeping-merchant and dealing sight gates

`internal/actions/repair.go:115` · status **confirmed** · reported as high

Repairer() does its own scan of room.GetMobs(rooms.FindMerchant) and returns any mob whose ShopCraftSupport matches. Repair() then charges and repairs. Neither function calls the gates that every other merchant transaction goes through: ShopClosedForSleep / RefuseMobIfAsleep / TargetAsleep (internal/actions/sleeping_target.go) and ShopSightRefusal / ShopSightRefusalText (internal/actions/shop_sight.go, lighting plan 5b, described there as 'the one line every refusing verb prints'). ListRepairs also prints the merchant's real name (m.Character.Name, line 190) with no sight check. The repair cost also skips barterDiscount, which shop_sight.go documents as 'the ONE place a shop's bartering discount is computed'.

**Failure scenario.** At 2am the town smith sleeps on the spot in his shop. list, buy and sell all refuse with 'X is fast asleep', but `repair sword` succeeds: the sleeping smith takes the gold and hands back a mended sword. Similarly, in an unlit shop below the faces band, sell refuses with 'You can't make out the goods well enough to deal.' Yet `repair` lists 'Brannoc will mend it for 12 gold' by name and completes the deal. That leaks a name the sight-gates work hides, and contradicts the sibling verbs.

**Existing mechanism.** actions.ShopClosedForSleep / RefuseMobIfAsleep / TargetAsleep (internal/actions/sleeping_target.go); actions.ShopSightRefusal + ShopSightRefusalText + barterDiscount (internal/actions/shop_sight.go)

**Suggested fix.** In usercommands.Repair, put the ShopClosedForSleep/RefuseMobIfAsleep and ShopSightRefusal gates on the merchant path, as buy and sell do. Have Repairer skip mobs where TargetAsleep is true. Apply barterDiscount to RepairCost if repair counts as a deal.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
repair.go:115-124 `for _, mobId := range room.GetMobs(rooms.FindMerchant) { if m := mobs.GetInstance(mobId); m != nil && m.ShopCraftSupport == trade { return m } }`. `grep -n -i "sight\|asleep\|IsFree\|Activity\|barter" internal/actions/repair.go internal/usercommands/repair.go` returns nothing (rc=1). Compare usercommands/buy.go:22, sell.go:23 and list.go:34 `if asleep := actions.ShopClosedForSleep(room); asleep != nil {...}`, list.go:58 `if actions.TargetAsleep(&mob.Character)`, and sell.go:72-79 `if ShopSightRefusal(seller.GetCharacter(), room) { ... ShopSightRefusalText`.
```

- **confirmed** (medium): The claim reproduces from the code as written. Repairer (internal/actions/repair.go:115-125) returns the first FindMerchant mob whose ShopCraftSupport matches the trade. It never checks TargetAsleep, and nothing on the call path does either. The `repair` command is registered at usercommands.go:204 as {Repair, false, false, false}, with no dispatch-level gate. usercommands/repair.go:14-23 calls straight into actions.ListRepairs or actions.Repair. Repair (repair.go:251-287) then debits the player's gold, credits the shop inventory or the mob, and mends the item. It never calls ShopClosedForSleep, RefuseMobIfAsleep, TargetAsleep or ShopSightRefusal. ListRepairs (line 190) and the cannot-afford and success lines (264, 285) print m.Character.Name with no light-band check.  The sibling verbs are gated. buy, sell and list call ShopClosedForSleep (usercommands/buy.go:22, sell.go:23, list.go:34). The sight gate is in actions/buy.go:361, actions/sell.go:74 and usercommands/list.go:40, and housing_shop.go also gates both sleep and sight. The doc comment on TargetAsleep says 113 of 132 schedules put an NPC to sleep in its own workplace, so the sleeping-smith scenario is common.  I am lowering the severity from high to medium. This is a gameplay consistency gap and a name leak, not a security or data-loss bug. The player does get the service they pay for. The barterDiscount part of the claim is the weakest: repair is a service fee with its own knob (RepairCostRatio), not a buy or sell price, so skipping bartering is arguably a design choice. The sleep and sight gates are clearly missing.

- **confirmed** (medium): Nothing else stops this. internal/actions/repair.go and internal/usercommands/repair.go are both new in the PR (311 lines added since c696c117a). Neither file calls any sleep gate or sight gate. Repairer (repair.go:115-125) scans room.GetMobs(rooms.FindMerchant) and returns the first mob whose ShopCraftSupport matches the trade, without checking TargetAsleep. Repair (repair.go:208-288) then takes char.Gold, credits the shop inventory or the mob, and repairs the item. The usercommand wrapper calls actions.ListRepairs or actions.Repair directly with no guard. The registry entry (usercommands.go:204, `repair`: {Repair, false, false, false}) adds no generic sleep or light gate. The gates exist and the sibling verbs use them: buy.go:22, sell.go:23 and list.go:34 call ShopClosedForSleep; list.go:40, housing_shop.go:88, actions/buy.go:361 and actions/sell.go:74 call ShopSightRefusal. The TargetAsleep doc comment says that 113 of 132 schedules put the NPC to sleep in its own workplace, so a sleeping smith taking gold is the normal night-time case, not an edge case. ListRepairs line 190 prints m.Character.Name and the repair messages at lines 264 and 285 print it too, all with no light-band check, so the name leaks in the dark. I lowered severity from high to medium for two reasons. First, the harm is inconsistent gameplay and a name leak, not corrupted data or exploitable gold. Second, the barterDiscount part is the weakest of the sub-claims: repair is a service cost and not a shop buy or sell, so leaving out the bartering discount is arguably a design choice. The sleep and sight gaps are real.

- **confirmed** (medium): The core defect holds. internal/actions/repair.go and internal/usercommands/repair.go are both new in the PR (311 lines added against c696c117a). Neither file checks sleep or sight anywhere.  - **Sleep:** `Repairer` (repair.go:115-125) returns the first merchant in the room whose ShopCraftSupport matches the trade. It does not check `TargetAsleep`, `ShopClosedForSleep` or `RefuseMobIfAsleep`. `Repair` (repair.go:251-287) then takes the gold, credits the shop or the mob, and mends the item. sleeping_target.go:28-31 calls `RefuseIfAsleep` "the single gate every player-initiated interaction with an NPC should pass through". It also notes that most shopkeepers sleep in their own workplace (113 of 132 schedules). So the reviewer's 2am scenario will really happen: buy, sell and list refuse, but repair works. - **Sight:** `ShopSightRefusal` is checked by buy.go:361, sell.go:74, list.go:40 and housing_shop.go:88. Repair never checks it. `ListRepairs` (repair.go:190) and both `Repair` messages (lines 264 and 285) print `m.Character.Name` with no light check. That leaks the merchant's name below the faces band and contradicts the sibling verbs.  **Where the claim is overstated:** - **Bartering:** `barterDiscount` is documented as the one place a shop's price cut on bought goods or bonus on sold goods is computed. Repair is a paid service priced by `RepairCost` from GradedValue times the configured RepairCostRatio. It is neither a buy nor a sell, so leaving out the barter discount is a design choice, not a broken unification. - **Severity:** The impact is a consistency and immersion bug: a sleeping or unseen merchant still provides a fair, configured service, plus a name leak in the dark. There is no gold duplication, no lost item, no crash and no exploit beyond the player getting a repair they should have been refused. That fits medium better than high.

</details>

<a id="f057"></a>
### F057 [medium] internal/mining is a line-for-line clone of internal/timber instead of a shared regrowing-node type

`internal/mining/vein.go:1` · status **confirmed** · reported as medium

vein.go duplicates timber/stand.go apart from identifier names: the struct shape {id, Stock, Max, Updated, worked-out flag}, the five long-term-data keys, the Store interface, asInt, Load/Save, Refill==Regrow, RoundsToNextLoad==RoundsToNextTree, Dig==Fell, PickOre==PickSpecies (same quarter-of-total neighbour bonus) and NewVein==NewStand. actions.RoomVein copies actions.RoomStand: rift guard, biome fallback, pool lookup, removed-id restart, seed or regrow-reroll, then save. neighbourOres copies neighbourSpecies. ResolveMine's give-or-drop closure is the third copy, after chop.go:264 and harvest.go:420. Any fix to refill arithmetic, persistence typing (asInt) or neighbour weighting must now be made twice, and the two copies are already starting to drift. For example, mining.Pool takes a roomId for per-room pools and timber.Pool does not.

**Failure scenario.** Suppose a future fix lands in timber's Regrow, for example the Updated reset when Stock>=Max that makes a full stand's refill clock restart on every read. Veins keep the old behaviour, so lumberjacking and mining disagree on regrowth timing with no test failing. The same goes for a persistence change such as handling the uint64 Updated round in asInt after YAML round-trips.

**Existing mechanism.** internal/timber Stand / LoadStand / SaveStand / Regrow / PickSpecies / NewStand, and actions.RoomStand (both added earlier in this same PR; not in baseline c696c117a)

**Suggested fix.** Extract one generic per-room node type (stock, max, updated, depleted, Load/Save with a key prefix, Refill, Take, weighted PickWithNeighbours) and one RoomNode(room, pool, prefix, knobs) helper. Have timber and mining keep only their YAML schemas. Hoist the give-or-drop closure into one actions helper.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
vein.go `func (v *Vein) Refill(now uint64, regrowRounds int) (cameBack bool) { if regrowRounds < 1 || now <= v.Updated || v.Stock >= v.Max { if v.Stock >= v.Max { v.Updated = now } ...` vs timber/stand.go `func (st *Stand) Regrow(now uint64, regrowRounds int) (cameBack bool) { if regrowRounds < 1 || now <= st.Updated || st.Stock >= st.Max { if st.Stock >= st.Max { st.Updated = now } ...`. mine.go:28-64 RoomVein vs chop.go:33-67 RoomStand. mine.go:67 neighbourOres vs chop.go:71 neighbourSpecies. `grep -rn "Dropped = true\|room.AddItem(itm, false)" internal/actions` shows chop.go:264, harvest.go:420 and mine.go:251.
```

- **confirmed** (medium): The duplication reproduces exactly as claimed. internal/mining/vein.go (179 lines) and internal/timber/stand.go (180 lines) are the same code with identifiers renamed. Matching pieces: the struct shape (Ore/Species, Stock, Max, Updated, WorkedOut/Felled), the five long-term-data keys (prefix `mining.` vs `timber.`), the Store interface, asInt, Load/Save, Refill/Regrow (same Updated=now reset when full), RoundsToNextLoad/RoundsToNextTree, Dig/Fell, PickOre/PickSpecies (same total/4 neighbour bonus) and NewVein/NewStand. Only the identifiers and comments differ. actions.RoomVein (mine.go:30-64) has the same flow as actions.RoomStand (chop.go:33-67): nil guard, rift_run guard, biome fallback to the zone biome, pool lookup, removed-id restart, seed or regrow-reroll, save. neighbourOres and neighbourSpecies are also the same. The drop-on-full closure appears at chop.go:264, harvest.go:420 and mine.go:251. The drift claim holds as well: mining.Pool(roomId, zone, biome) at mining.go:254 takes a roomId, while timber.Pool(zone, biome) at timber.go:220 does not. Neither package exists in baseline c696c117a (git ls-tree returned nothing), so this PR introduces both copies. Because one generic regrowing-node type, parameterised by key prefix, would serve both, keeping two copies is a real maintainability and unification defect. It is not a runtime bug, so medium is the right severity.

</details>

<a id="f124"></a>
### F124 [low] Rift-room guard reads a raw 'rift_run' temp-data string instead of the existing Room.IsEphemeral(), and misses instanced zones

`internal/actions/mine.go:39` · status **confirmed** · reported as medium

RoomVein (and RoomStand at chop.go:39) refuse rift rooms with room.GetTempData(`rift_run`) != nil. The key is a literal string set at internal/rifts/runtime.go:387; the comment says it is read here because rifts imports actions. The problem being guarded is 'a room rebuilt with empty long-term data reseeds a full vein'. That applies to every ephemeral room, not only rifts. Baseline already exposes the test: Room.IsEphemeral() (rooms.go:152, RoomId >= ephemeralRoomIdMinimum). Rift rooms are ephemeral (OwnedChunk.AddRoom assigns ephemeralRoomIdMinimum + ...), and CreateZoneInstance clones instanced zones into ephemeral rooms whose Zone and Biome still match the mining.yaml pools.

**Failure scenario.** An instanced zone with a cave or mountains biome, or one listed under mining.yaml zones, is cloned through CreateZoneInstance. Every entry gets fresh rooms with empty long-term data, so each run hands out full, newly seeded veins. That is the exact farming loop the rift guard was written to stop. No instanced zone is mineable today (crash_site_interior and instance_arena are interior, the jail cell is dungeon, the oasis is ether), so this is latent. Separately, renaming the `rift_run` key in internal/rifts compiles cleanly and silently reopens the rift exploit in both chop and mine.

**Existing mechanism.** rooms.Room.IsEphemeral() / rooms.IsEphemeralRoomId (internal/rooms/rooms.go, internal/rooms/ephemeral.go)

**Suggested fix.** Replace both literal-key checks with `if room.IsEphemeral() { return ..., false }`. If rifts need to be singled out for some other reason, export the key as a constant from a package both can import (for example rooms).

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
mine.go:39 `if room.GetTempData(`rift_run`) != nil {`. chop.go:39 has the same line. rifts/runtime.go:387 `room.SetTempData(`rift_run`, run.Id)`. rooms.go:152 `func (r *Room) IsEphemeral() bool { return r.RoomId >= ephemeralRoomIdMinimum }`. ephemeral_owned.go:103 `r.RoomId = ephemeralRoomIdMinimum + (c.id * ephemeralChunkSize) + slot`. Baseline rifts/portal.go itself already uses `room.IsEphemeral()` to refuse rifts inside instances.
```

- **confirmed** (low): The facts in the claim hold up. RoomVein (mine.go:39) and RoomStand (chop.go:39) both guard with the literal string room.GetTempData(`rift_run`). That key is set only at rifts/runtime.go:387, and nothing ties the two strings together at compile time. Room.IsEphemeral() (rooms.go:157) is the codebase's existing test for "this room is rebuilt and its long-term data won't persist", and rift rooms meet it: OwnedChunk.AddRoom assigns ephemeralRoomIdMinimum + offset (ephemeral_owned.go:103).  Other places already use IsEphemeral for the same kind of guard. The baseline search_bauble.go:502 (now :543) uses it as an anti-farming guard, and rifts/portal.go:221 uses it too. Zone instances clone their rooms through CreateEphemeralRoomIds, which calls LoadRoomTemplate and gives the clone an ephemeral id. The vein state lives only in LongTermData (mining/vein.go LoadVein/SaveVein), so every new instance would start with a full vein.  So the mechanism is reinvented, and it covers less than the existing one would. The exploit is latent, though: of the four instanced zones (crash_site_interior, instance_arena, instance_jail_cell, instance_planar_oasis), none is listed under mining.yaml zones (Pothole Coulee, Labyrinth, Ironwind, Eastern Highlands, Cascade Pass, Stillwater). The claim also says their biomes are not cave, mountains or cliffs; I did not check that myself. The brittleness of the string key is real, but it is a maintenance hazard and not live behavior. I'm downgrading it from medium to low: it is a valid unification and consistency finding, but nothing is exploitable today.

</details>

<a id="f125"></a>
### F125 [low] Rare-find clamps and the ore-recognition threshold are hardcoded literals, already diverging between the two copies of the formula

`internal/actions/mine.go:198` · status **confirmed** · reported as low

gemChance computes base * perception/100 * gather.RareMult(tier), the same expression as harvest.go:248 for rare carcass parts, then clamps it with Go literals. Mining uses 0.005..0.25; harvest uses 0.02..0.9. OreKnown gates naming an ore on perception*SightMult >= 100 + 5*(tier-2), also literals. Every other number in this feature is a Balance knob surfaced in config.yaml (MiningGemChance, RareToolMult*, GatherRareBaseChance), so these are the only parts of the rare-find and ore-recognition tuning that cannot be retuned without a code change. The same formula is maintained in two places with different bounds.

**Failure scenario.** An operator raises MiningGemChance or RareToolMultMasterwork in config.yaml to make gems more common. Past the hidden 0.25 cap nothing changes, and config.yaml shows no hint why. A balance pass that unifies rare-find odds must find and edit two differently clamped copies in mine.go and harvest.go.

**Existing mechanism.** Balance knobs in internal/configs/config.balance.go surfaced through _datafiles/config.yaml (e.g. GatherRareBaseChance, RareToolMult*); gather.RareMult

**Suggested fix.** Add one gather.RareChance(base, perception, tier, min, max) helper used by both harvest and mine, with the floor and cap as Balance knobs (per job if they must differ). Make the OreKnown base and step knobs too.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
mine.go:197-198 `c := base * float64(perception) / 100.0 * gather.RareMult(pickTier); return math.Max(0.005, math.Min(0.25, c))`. harvest.go:248-249 `chance := base * float64(in.Perception) / 100.0 * gather.RareMult(tier); chance = math.Max(0.02, math.Min(0.9, chance))`. mine.go:124 `return eye >= 100+5*float64(ore.Tier-2)`.
```

- **confirmed** (low): The cited code is in the PR head exactly as quoted, and both files are new in this PR: neither internal/actions/mine.go nor internal/actions/harvest.go exists at c696c117a. So the PR itself adds two copies of the rare-find expression `base * perception/100 * gather.RareMult(tier)`, and each copy has its own hardcoded clamp: mining uses 0.005..0.25 and harvest uses 0.02..0.9. OreKnown's threshold `100+5*(tier-2)` is also a literal. Every other input to these formulas is a Balance knob: MiningGemChance, GatherRareBaseChance and RareToolMult* are all in config.yaml and config.balance.go. The failure scenario holds. With the shipped values (MiningGemChance 0.04, masterwork RareMult 2.0), the 0.25 cap starts to bind at Perception 312.5. Below that, raising MiningGemChance far enough also hits a cap that neither config.yaml nor config.balance.go mentions; their mining comments describe the formula with no clamp. The harvest clamp is at least written down in a config.balance.go comment ("clamped 0.02..0.9") but not in config.yaml. The different bounds look deliberate (gems are meant to be rarer), so this is a tuning and consistency gap, not a correctness bug. Low severity is right.

</details>

<a id="f126"></a>
### F126 [low] Self-repair skips the timed-activity system and tool wear that the craft path it mirrors applies

`internal/actions/repair.go:240` · status **confirmed** · reported as medium

RepairRecipe copies InitiateCraft's gates (HasRecipe, skill minimum, StationSatisfied, ToolSatisfied). On a match, Repair calls itm.Repair() at once. It starts no activity, runs no contest, consumes nothing and never calls WearRecipeTool. Both craft completion paths call WearRecipeTool (craft.go:267 for instant recipes, NewRound_UserRoundTick.go:663 for timed ones). Neither Repair nor the usercommand checks Activity.IsFree, so it also runs in the middle of another job. The other room-bound labour in this PR rides the shared Activity machine.

**Failure scenario.** A smith with the iron sword recipe, standing at a forge with a hammer, types `repair sword` after every fight. Each repair restores 60 points of durability in zero rounds with no materials, and the hammer takes no wear. The hammer never needs repair itself, so the gear-wear economy (RepairCostRatio, WornSellPenalty, scrap pricing) never touches a crafter. Repair can also run while the player is mid-craft or mid-mine.

**Existing mechanism.** actions.WearRecipeTool (internal/actions/craft.go); activity.Machine TransitionToCrafting/TransitionToSalvaging timed jobs (internal/state/activity)

**Suggested fix.** Call WearRecipeTool(actor, r) on the self-repair path at minimum. Refuse when !char.Activity.IsFree(). Consider running self-repair as a short timed Crafting job so movement and abandonment rules apply uniformly.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
repair.go:240-249 `if r := RepairRecipe(char, room, *itm); r != nil { itm.Repair(); res.Repaired, res.Self = true, true ... return res }`. craft.go:369-375 `func WearRecipeTool(actor Actor, recipe *crafting.RecipeSpec) { ... WearUsedTool(actor, t, ok) }`, called at craft.go:267 and NewRound_UserRoundTick.go:663 but not from repair.go.
```

- **confirmed** (low): The code reproduces as claimed. In repair.go:240-249, when RepairRecipe matches, Repair calls itm.Repair() straight away and returns. It starts no Activity transition, consumes nothing, and never calls WearRecipeTool. A grep shows WearRecipeTool has exactly two callers, craft.go:267 and NewRound_UserRoundTick.go:663, and none in repair code. The usercommand (internal/usercommands/repair.go) goes straight to actions.Repair with no IsFree check, while its sibling labour commands (mine.go:33, chop.go:33, salvage.go:32, carcass.go:98) all check user.Character.IsFree(). So repair can run in the middle of a craft or a mine, and the hammer never wears.  I lowered the severity because part of the claim describes the documented design. docs/economy/wilderness-trades.md:290 and the PATCH_NOTES entry both say self-repair is "free" once the recipe, skill, station and tool are present. Being instant and costing no gold or materials is therefore the stated intent, not a defect. Two parts are real inconsistencies with the craft path it copies: the tool never takes wear, and there is no IsFree gate. The gear-wear economy is only partly bypassed: a crafter still needs the recipe, the skill and the station, so the problem is narrower than the claim's "never touches a crafter".

</details>

<a id="f127"></a>
### F127 [low] Scrap price memory is a second, differently paced decay counter beside the existing overstock decay

`internal/shops/shopinventory.go:452` · status **confirmed** · reported as low

ShopInventory.Scrap / ScrapHeld / AddScrap is a new per-item counter. It is added into `current` before WalkInBuyPrice so scrapped buys slide the price, and it decays lazily one unit per ShopScrapDecayRounds (900). Shelved walk-in goods already have this mechanism: StockEntry.Current plus LastGrewRound, drained by TickOverstockDecay at ShopOverstockDecayRounds (21600) and ShopOverstockDecayQty. The two counters feed the same price curve with different decay rules. Scrap entries are never pruned and are not visible to the throughput and stock-event bookkeeping.

**Failure scenario.** A player sells five forged iron picks, then five iron swords that are past 85% wear, to the same smith. The pick and sword buy prices recover about 24 times faster (one unit per 900 rounds) than a shelved good's would after the same volume (one per 21600). Tuning ShopOverstockDecayRounds has no effect on scrap. The yaml `scrap:` map keeps zero-count entries for every item ever scrapped.

**Existing mechanism.** shops.StockEntry.Current/LastGrewRound + shops.TickOverstockDecay (internal/shops/overstock_decay.go), knobs ShopOverstockDecayRounds/ShopOverstockDecayQty

**Suggested fix.** Either track scrap as a non-listable StockEntry (Current that is never sold from) so TickOverstockDecay governs it, or document why scrap must decay on its own clock. In either case, prune zero-held entries.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
shopinventory.go `func (si *ShopInventory) ScrapHeld(itemId int, now uint64) int { ... if rate := uint64(configs.GetBalanceConfig().ShopScrapDecayRounds); rate > 0 && now > sc.Round { n -= int((now - sc.Round) / rate) }`. buyrules.go `current += shopInv.ScrapHeld(spec.ItemId, now)`. Baseline overstock_decay.go:24 `func TickOverstockDecay(si *ShopInventory, round uint64) []DecayedUnit` driven by ShopOverstockDecayRounds (config.balance.go:882, default 21600).
```

- **confirmed** (low): Every mechanical part of the claim matches the code. ShopInventory.Scrap (map[int]ScrapCount) is a new counter. ScrapHeld subtracts one unit per ShopScrapDecayRounds (900 in config.yaml:1844). buyrules.go adds it to `current` before WalkInBuyPrice and the CalcBuyPrice resale cap. Shelved goods instead decay through TickOverstockDecayWith, one unit (ShopOverstockDecayQty: 1) per ShopOverstockDecayRounds (21600 in config.yaml:1435). So the two counters feed the same walk-in price input at about 24x different paces, and the overstock knobs do not reach scrap.  Map entries are never removed. AddScrap overwrites an entry and ScrapHeld clamps it to 0, but the key stays, so the persisted `scrap:` map keeps one stale entry for every item id ever scrapped. That growth is bounded by the number of distinct item ids, so it costs little.  The "just reuse StockEntry" framing is weaker than claimed. StockEntry.Current is the shop's sellable shelf stock. The whole point of the scrap path (sell.go:433-455, `noResale := items.NeverResold(itemSpec) || item.WearFraction() >= items.BadlyWornFraction`) is to keep these goods off the shelf so wear is not laundered. TickOverstockDecay also only works above a RestockQty baseline and skips components. Reusing it as-is would put scrap on the shelf, so some separate or flagged counter is justified.  The separate pace is a deliberate, documented knob (config.balance.go:628-631) with its own yaml key, not an accident. What remains is a real but minor consistency point: there is a second decay rule with no pruning, and it sits outside StockEvents and throughput bookkeeping. It is not a correctness bug.

</details>

<a id="f139"></a>
### F139 [low] ShieldPtr re-implements HasAnyShield's shield predicate over a different slot set and spec view

`internal/characters/gear_wear.go:67` · status **refuted** · reported as low

ShieldPtr re-derives 'is a shield' as `spec.Type == items.Offhand && (spec.PhysicalMitigation > 0 || spec.Subtype == items.Wearable)` over Equipment.GetAllItemPtrs() using GetRawSpec. The baseline predicate is Character.HasAnyShield (hand_slots.go:162), which reads the weapon and arm slots via getWeaponAndArmItems using GetSpec. Bash gating uses HasShield/HasAnyShield and shield-bash wear uses ShieldPtr, so the two can disagree.

**Failure scenario.** An offhand holdable that gains mitigation only through enchanting makes GetSpec's PhysicalMitigation > 0, while GetRawSpec's stays 0. HasAnyShield then says the character has a shield and allows `bash`, but ShieldPtr returns nil, so a critical shield bash wears nothing. The baseline doc comment names that enchant case explicitly. Any future change to the shield definition in HasAnyShield will not reach gear wear.

**Existing mechanism.** characters.Character.HasAnyShield / getWeaponAndArmItems (internal/characters/hand_slots.go)

**Suggested fix.** Factor one isShieldSpec(spec) predicate and one pointer-returning arm-slot scan shared by HasAnyShield and ShieldPtr.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
gear_wear.go:67-81 `for _, p := range c.Equipment.GetAllItemPtrs() { ... spec := p.GetRawSpec(); if spec.Type == items.Offhand && (spec.PhysicalMitigation > 0 || spec.Subtype == items.Wearable) { return p } }`. hand_slots.go:162-175 `slots := c.getWeaponAndArmItems() ... spec := slot.GetSpec(); if spec.Type == items.Offhand && (spec.PhysicalMitigation > 0 || spec.Subtype == items.Wearable) { return true }` with the comment 'an offhand holdable that gains mitigation through enchanting shields you'.
```

- **refuted** (none): The failure scenario depends on GetSpec reporting PhysicalMitigation > 0 while GetRawSpec reports 0. The code cannot produce that. GetSpec (items.go:346) is GetRawSpec plus applyGrade, applyBowWood and applyCondition, and all of those scale mitigation through scaleInt. scaleInt (grade_effects.go:73) returns v unchanged when v == 0, so a raw mitigation of 0 can never become positive. Enchant and affix changes go into the instance's i.Spec override, and GetRawSpec returns that override first (items.go:362), so HasAnyShield and ShieldPtr see the same enchanted value. The doc comment's "gains mitigation through enchanting" case is therefore detected by both. The slot-set difference has no practical effect either. GetAllItemPtrs also covers non-arm slots and extra-arm slots past c.ExtraArms, but an Offhand-type item only reaches the offhand or arm slots. One divergence is possible in theory: a shield with raw mitigation 1, worn hard enough to round to 0, would drop out of HasAnyShield but not ShieldPtr. The comment says every authored offhand is subtype Wearable, and that subtype short-circuits the predicate identically in both, so even this case does not occur. What remains is a duplicated predicate that is textually identical. That is a minor tidy-up note (ShieldPtr could share a helper with HasAnyShield), not a defect that reproduces.

</details>
