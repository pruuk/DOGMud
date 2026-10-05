# PR #208 review: craft wood tools gear, unification lens

Blind reviewer `craft_wood_tools_gear:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Unification/reuse review of the wilderness trades phases 4-5 piece (range 46766d13b..52cfbeda4: timber, chop, tool ladder and wear, grade effects, wood traits, woodwork, hunter gear), judged at the PR head. The contributor mostly reused the existing mechanisms well. Every new roll goes through the shared resolution: chop uses gather.Roll, then crafting.RunSalvageContest, and survey uses contest.AgainstDifficulty. The new site is registered in both contest guard tables (contest_site_guard_test.go and contest_floor_guard_test.go). Sight is handled by messaging.SightMult and messaging.CanSeeClearly. Narration goes through messaging.SendTrio with ActorName set, so observers who cannot see get the anonymized name. Felling runs on the existing Salvaging activity under a prefix key, as harvest already does. Grade pricing goes through the existing shops QualityValueMultiplier path in buyrules, and hunter goods use the existing vendor_categories/CraftSupport buy gate. The carpentry-to-woodwork skill migration follows the established validateSkillMigrations pattern. Crafted furniture reuses the housing deed placement functions. affixgen was correctly switched to GetRawSpec so a grade is never baked into an override. Almost every new balance number is a config.balance knob, and I confirmed Timber*, ToolDurability*, RareToolMult* and Grade* are all present in config.yaml.

The divergences found:
- **Medium: the NeverResold rule is hand-copied at four call sites.** The two NPC crafter paths that put output on a shelf are missed. This is latent today, because I found no mob that lists a forged-tool recipe.
- **Medium: grade-to-durability multipliers are hardcoded.** Every other per-grade table is a config knob.
- **Low: a new room spoilage sweep skips the hooks of its two sibling sweeps.** It runs only on the occupied-room round tick, unlike removeUntakenBaubles (Prepare and LoadRoomInstance) and the floor decay pass.
- **Low: further hardcoded chop numbers.** These are the bark chance, the branch count and the species-recognition difficulty.
- **Low: the species-recognition gate is leaky.** The `chop <name>` argument and the chop result messages both reveal the species, and each survey rerolls.
- **Low: three small copies.** The shop abundance-seeding formula is copied into reconcileShop, the store-or-drop-plus-ownership-event block is copied into chop, and deed text is rewritten with strings.ReplaceAll instead of being parameterized.

Sibling note outside this range: the later mining code (internal/mining, actions/mine.go) is a near line-for-line copy of internal/timber and RoomStand: Vein/Stand, PickOre/PickSpecies, Refill/Regrow and neighbourOres/neighbourSpecies. Its recognition rule (OreKnown: a fixed Perception threshold, no roll) also disagrees with SpeciesKnown (a contest roll). A shared per-room renewable-resource helper would have kept the two from drifting.

## Coverage

I read these in full at the PR head: internal/timber/{timber,stand,wood}.go, internal/actions/{chop,toolwear,craft (diff)}.go, internal/usercommands/chop.go, internal/gather/tools.go, gather.Roll/Score, internal/items/{tools,grade_effects}.go, and the items.go/itemspec.go/stacking.go diffs. I also read the diffs of internal/shops/{persistence,shopinventory,craftdecision}.go, actions/sell.go, caravan/visit.go, combat_fire.go and combat_reload.go, the NewRound_UserRoundTick chop and craft hooks, characters/validate.go, skills.go, housing/{crafted,use_items}.go, rooms/spoilage.go and the config.balance.go additions. I compared these against these existing mechanisms:
- the contest package (AgainstDifficulty) and CalcSearchScore with the search/track pattern
- messaging.SendTrio and its Audience struct
- the floor-decay and bauble-untaken room sweeps, including where they are called
- the shop seeding in RegisterShop
- every path that adds stock to a shelf (sell, caravan, forager, mobs/crafter executeCraft, pickSelfGearRecipe, executeCraftLegacy)
- item BreakChance/BreakTest
- QualityValueMultiplier and the enchantment override and baseline code
- fileloader.LoadFlatFile and the ferry loader
- the forged-tool recipes and whether any mob uses them (grep of _datafiles/world/dogmud/mobs found none)

What I skimmed or did not cover:
- **Skimmed:** the harvest.go and forage.go diffs (RareMult, MinTool, sickle extra draws), species/harvest.go and forager/forage_core.go.
- **Not reviewed:** tests, docs and PATCH_NOTES, context.md files, the narration golden file, content YAML (items, recipes, timber.yaml) beyond identifying the tool items, the hunter gear recipe YAML, and planners/mutations/aicompanion one-line renames.
- **Out of scope:** gear combat wear (characters/gear_wear.go), repair and applyCondition come from later commits (c45ffe974, 3ce674451), so I did not review them, apart from noting that BreakChance and Wear now coexist as two breakage systems.

## Findings (8)

<a id="f114"></a>
### F114 [low] Chop's byproduct odds and species-recognition difficulty are hardcoded

`internal/actions/chop.go:28` · status **confirmed** · reported as low

The bark chance (const chopBarkChance = 40, rolled as util.Rand(100) < chopBarkChance at :274), the branch count (1+util.Rand(2) at :272) and the recognition difficulty in SpeciesKnown (100+15*(tier-2) at :127) are balance numbers written in Go. The same piece put the rest of lumberjacking in Balance: TimberEase, TimberTierDifficulty, TimberStandMin/Max, TimberRegrowRounds, TimberChopRoundsBase and TimberMaxLogs. These four values cannot be tuned from config.yaml, and the later mining sibling (OreKnown) uses a different hardcoded recognition rule.

**Failure scenario.** Bark turns out to flood the market, or rare species are too hard to identify. The owner looks for a Timber* knob in config.yaml and finds none, so the change needs a code edit and a deploy instead of a config edit.

**Existing mechanism.** The Balance Timber* knob block added in internal/configs/config.balance.go and config.balance.gathering.go.

**Suggested fix.** Add TimberBarkChancePct, TimberBranchMin/Max, TimberRecognizeBase and TimberRecognizePerTier knobs with defaults and config.yaml entries.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
chop.go:28 `const chopBarkChance = 40`. chop.go:272 `give(branch.ItemId, 1+util.Rand(2))`. chop.go:274 `if sp.BarkItemId != 0 && util.Rand(100) < chopBarkChance {`. chop.go:127 `return contest.AgainstDifficulty(score, 100+15*float64(sp.Tier-2)).Success`.
```

- **confirmed** (low): Every citation matches the PR head. The bark chance, the branch count and the species-recognition difficulty are fixed numbers in Go. Meanwhile the Timber* knobs (TimberEase, TimberTierDifficulty, TimberStandMin/Max, TimberRegrowRounds, TimberChopRoundsBase, TimberMaxLogs) are declared in config.balance.go, defaulted in config.balance.gathering.go and shipped in _datafiles/config.yaml. None of them covers bark odds, the branch yield or the recognition threshold. SpeciesKnown also writes 15 per tier as a literal, even though TimberTierDifficulty already ships at 15 for the felling roll, so the same idea is set in two places. The mining sibling, OreKnown, uses a different rule: a raw Perception threshold of 100+5*(tier-2) with no contest roll, against chop's AgainstDifficulty search contest at 100+15*(tier-2). That supports the inconsistency point. The cost is that tuning these values needs a code change and a deploy, not a config edit. Nothing breaks, so the severity stays low.

</details>

<a id="f115"></a>
### F115 [low] ResolveChop hand-rolls the store-or-drop and ItemOwnership block that harvest already has

`internal/actions/chop.go:259` · status **confirmed** · reported as low

The give closure in ResolveChop re-implements "items.New, set Quality, StoreItem else room.AddItem, queue events.ItemOwnership", which ResolveHarvest already does (harvest.go:411-423). The later mine.go adds a third copy. The copies already differ: harvest stamps itm.CraftedRound = now, which feeds the spoilage clock and age readers, and chop does not. Chop sets a Dropped flag and harvest does not. A future rule for gathered goods (an ownership event change, a stamp, a weight check) has to be applied to each copy.

**Failure scenario.** A bark or branch item is later given a spoil_after, or any reader keys on CraftedRound for gathered goods. Chopped goods then carry CraftedRound 0 while harvested goods carry the harvest round, so the same reader treats the two differently.

**Existing mechanism.** The harvest goods-store loop in actions.ResolveHarvest (internal/actions/harvest.go). No shared helper exists yet; this piece was the point at which to extract one.

**Suggested fix.** Extract actions.giveGathered(actor, room, itemId, qty, grade, now) (store-or-drop, CraftedRound and ownership event) and use it from harvest, chop and mine.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
chop.go:252-268: `itm := items.New(itemId) ... itm.Quality = roll.Grade; if char.StoreItem(itm) { if actor.GetUserId() != 0 { events.AddToQueue(events.ItemOwnership{...}) } } else { room.AddItem(itm, false); res.Dropped = true }`. harvest.go:411-423: the same block plus `itm.CraftedRound = now`.
```

- **confirmed** (low): The duplication is real and matches the description exactly. ResolveChop's give closure (chop.go:252-268) does items.New, sets Quality, calls StoreItem, queues ItemOwnership for users, and otherwise calls room.AddItem and sets res.Dropped. ResolveHarvest's goods loop (harvest.go:407-424) is the same block, except that it also stamps itm.CraftedRound = now and does not set a Dropped flag. mine.go:239-256 holds a third copy that matches chop and also omits CraftedRound. The divergence the claim names is real. Per items/spoilage.go:12-22, an instance with CraftedRound == 0 never spoils, and stacking.go:58 compares CraftedRound when either item spoils, so if chopped or mined goods ever got a spoil_after, they would silently never spoil. All three files are new in this PR (git diff c696c117a..HEAD shows chop.go +307, mine.go +308, harvest.go +575), so the three copies were written together rather than inherited from older code, which makes "extract one helper" the consistent choice. There is no live defect today: I found no spoil_after on any bark, branch, log or ore item, so for now the gap is a latent consistency risk. Low severity is right.

</details>

<a id="f116"></a>
### F116 [low] Crafted furniture changes deed narration with strings.ReplaceAll instead of passing its own wording

`internal/housing/crafted.go:47` · status **confirmed** · reported as low

useCraftedFurnishing reuses the deed placement functions, which is the right move. It adapts their player text by substring-replacing four deed phrases in the output. Any edit to a deed message breaks the rewrite silently, and lines it does not cover still speak of deeds or porters. A crafted bed frame is narrated as "Two porters carry in a bed, frame first and then the mattress". Wording that differs only by item kind belongs in a parameter of the shared functions, not in a filter on their output.

**Failure scenario.** A copy editor changes "The deed stays folded." to "The deed stays in your pack." in containers.go. The player who uses a crafted chest in a full room now reads about a deed they do not have, and no test pins the rewrite. Even today, placing a crafted bed tells the player that porters carried a bed in.

**Existing mechanism.** The useContainerDeed, useBedDeed and useStationDeed functions in internal/housing.

**Suggested fix.** Pass a small wording struct (noun, keep line, arrival line) into the deed functions, or have them take an isCrafted flag, so the narration is chosen and not patched afterwards.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
crafted.go:47-50 `msg = strings.ReplaceAll(msg, `The deed stays folded.`, `You keep it for now.`)`, and three more such replacements. furnishings.go:251 `send(`Two porters carry in a bed, frame first and then the mattress...`)` is not rewritten.
```

- **confirmed** (low): The claim holds as written. In crafted.go:44-52, useCraftedFurnishing wraps deedSend in a closure that runs four strings.ReplaceAll calls on exact deed phrases, then hands that closure to useContainerDeed, useBedDeed and useStationDeed. The shared functions take no wording or item-kind parameter, so the rewrite works only while each deed sentence matches its pattern byte for byte. Lines outside those patterns pass through unchanged. useBedDeed's success line (furnishings.go:251) says "Two porters carry in a bed, frame first and then the mattress..." and the room line at :252 also mentions porters. A crafted bed frame therefore produces porter narration. The chest path does the same: containers.go:165/167 say "A porter hauls in your %s". No _test.go file in internal/housing contains "You keep it for now", "The deed stays folded" or "porters", so no test pins the rewrite. If a deed message changes, the rewrite stops matching and nothing fails. This is a maintainability and copy-consistency problem, not a functional bug, so low severity fits.

</details>

<a id="f117"></a>
### F117 [low] Grade-to-durability multipliers are hardcoded while every other per-grade table is a config knob

`internal/items/tools.go:103` · status **confirmed** · reported as medium

toolGradeDurability hardcodes crude 0.75, fine 1.25, superb 1.5 and pristine 2.0. These multiply both ToolDurability and GearDurability. This piece added config knobs for every other per-grade or per-tier table: Grade{Damage,Speed,Weight,Armor}{Crude..Pristine}, ToolDurability{Tier}, ToolMult* and RareToolMult*. The existing QualityValueMultiplier also reads QualityValue{Crude..Pristine} from Balance. So how long a pristine tool lasts relative to a standard one is the only grade curve that a config.yaml edit cannot retune. The project rule is that balance numbers live in config.balance.go and are surfaced in config.yaml.

**Failure scenario.** An owner retunes tool and gear longevity in config.yaml, for example lowering ToolDurabilityIron because pristine tools never break. The 2.0x pristine and 0.75x crude spread stays fixed and needs a Go change and a deploy. A test written against a durability expectation also cannot follow a shipped override. The constants only change when code changes, so they silently drift from the rest of the grade knobs.

**Existing mechanism.** configs.Balance knobs surfaced in _datafiles/config.yaml, following the QualityValue*/Grade* pattern (items.QualityValueMultiplier, items.GearGradeMults).

**Suggested fix.** Add GradeDurability{Crude,Fine,Superb,Pristine} to config.balance.gathering.go with defaults and config.yaml entries, and read them in toolGradeDurability the way GearGradeMults reads its knobs.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
tools.go:103 `func toolGradeDurability(q Quality) float64 { switch q { case QualityCrude: return 0.75 ... case QualityPristine: return 2.0 } return 1.0 }`, used by ToolDurability and GearDurability. In contrast, config.balance.go adds GradeDamageCrude..GradeArmorPristine and ToolDurabilityCrude..Masterwork, and quality.go:82 QualityValueMultiplier reads b.QualityValueCrude..Pristine.
```

- **confirmed** (low): The claim checks out against the code. At the PR head, internal/items/tools.go:103 toolGradeDurability hardcodes crude 0.75, fine 1.25, superb 1.5 and pristine 2.0, with 1.0 as the default. Both Item.ToolDurability (line 146) and Item.GearDurability (line ~180) multiply their base by this value. In the same PR, every neighbouring per-grade or per-tier curve is a Balance knob surfaced in _datafiles/config.yaml: ToolDurability{Crude..Masterwork}, ToolMult*, RareToolMult*, GearDurabilityWeapon/Armor and Grade{Damage..}. QualityValue{Crude..Pristine} was already a Balance knob before the PR. So the durability grade spread is the only grade curve that an owner cannot retune in config, which breaks the project rule that balance numbers live in config. I am lowering the severity to low. Nothing is functionally wrong; it is a consistency and retunability gap. The owner can still change overall longevity through the base ToolDurability*/GearDurability* knobs, and only the relative spread between grades is fixed.

</details>

<a id="f118"></a>
### F118 [low] The NeverResold shelf rule is copied by hand into four sites and misses both NPC crafter output paths

`internal/mobs/crafter.go:557` · status **confirmed** · reported as medium

The rule that a forged tool of iron tier or better is never put on a shelf (items.NeverResold) is not enforced where stock is added. Instead it is copied into four callers: sell.go, caravan/visit.go, shops/craftdecision.go (EvaluateCraftOptions) and shops/persistence.go (reconcileShop). Two other paths that put crafted output on a shelf have no check. (1) TickMobCraft's Priority-1 selector, pickSelfGearRecipe (crafter.go:430), returns before EvaluateCraftOptions is called (crafter.go:338 vs 343). executeCraft then does shopInv.AddStockAtRound(recipe.Output.ItemId, ...) at crafter.go:557. (2) executeCraftLegacy calls mob.Character.Shop.StockItem(recipe.Output.ItemId) at crafter.go:616 with no check. reconcileShop does strip such entries again, but only when the shop next registers at boot or load, so a minted tool sits for sale until then.

**Failure scenario.** The forged tools are weapon-type items (10050 iron skinning knife, 10052 iron cleaver, 10053 woodcutter's axe, 10054 steel felling axe, and others), so itemvalue.IsUpgrade can return true for them. Say a content author adds iron-skinning-knife or steel-felling-axe to a blacksmith's crafter_recipes, the same way other blacksmithing recipes are listed. Once the mob has the ingredients, pickSelfGearRecipe picks the recipe and executeCraft shelves the steel axe. Players can then buy the tool that the design reserves for player smiths, until the next restart's reconcileShop removes it. A future stock path (scavenger resale, warehouse withdraw) will also miss the rule unless someone remembers to copy it a fifth time.

**Existing mechanism.** shops.ShopInventory.AddStockAtRound (internal/shops/shopinventory.go:209), which is the single point every shelf addition already goes through, plus Shop.StockItem for the legacy path. The check belongs there, not in each caller.

**Suggested fix.** Enforce items.NeverResold inside ShopInventory.AddStockAtRound (refuse the add) and inside the legacy Shop.StockItem. Alternatively, filter it in pickSelfGearRecipe and pickEligibleRecipe as EvaluateCraftOptions does. Then drop the copies in the callers.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
grep NeverResold at the head shows only these call sites: actions/sell.go:436, caravan/visit.go:124, shops/craftdecision.go:150, shops/persistence.go:170 and :184. crafter.go:338 `if selfRecipe := pickSelfGearRecipe(mob, recipeIds, shopInv, reservePct); selfRecipe != nil { return tagRestock(executeCraft(...)) }` runs before crafter.go:343 `shops.EvaluateCraftOptions(...)`. crafter.go:557 `shopInv.AddStockAtRound(recipe.Output.ItemId, 1, round)`. crafter.go:616 `mob.Character.Shop.StockItem(recipe.Output.ItemId)`. The forged-tool recipes exist in recipes/blacksmithing/{iron-skinning-knife,steel-felling-axe,woodcutters-axe,...}.yaml. Grepping the mobs dir for them currently finds nothing, so this is latent.
```

- **confirmed** (low): The code matches the claim. The PR added a NeverResold guard to EvaluateCraftOptions, which is the Priority-2 selector. It did not add one to the Priority-1 selector, pickSelfGearRecipe, which runs first and goes straight to executeCraft. executeCraft then calls shopInv.AddStockAtRound with no check. The legacy path, executeCraftLegacy, calls Shop.StockItem with no check either. AddStockAtRound itself has no tool check, so the rule is enforced only by the call sites that copy it. The forged tools are type: weapon. Example: 10054 Steel Felling Axe has hands: 2, damage, and tool tier 3. So itemvalue.IsUpgrade can plausibly return true, and the Priority-1 path is reachable once such a recipe is in a crafter's recipe list. reconcileShop removes these entries only when the shop registers or loads.  I am lowering severity from medium to low because the problem is latent. A shop's recipe list comes only from mob.CrafterRecipeIds (crafter.go:66, :327), and no shop learns recipes at runtime. Grepping the dogmud mobs for 1005x IDs and forged-tool recipe names finds only Corwin Ashlade, and his entry is 10057, a self bow, not a recipe for a forged tool. So no shipped content triggers this today. It still matters under the project rule "finish sibling code paths you made inconsistent": the PR guarded one crafter selector and left the one beside it unguarded. Putting the check in AddStockAtRound and StockItem, or at least in pickSelfGearRecipe and the legacy path, would close the gap.

</details>

<a id="f119"></a>
### F119 [low] The new room spoilage sweep skips the hooks its two sibling floor sweeps rely on

`internal/rooms/spoilage.go:20` · status **confirmed** · reported as low

removeSpoiledGoods is a third bespoke sweep that removes floor items. Its doc says it is modelled on the untaken-bauble sweep, but it is wired only into Room.RoundTick (rooms.go:2770). RoundTick only runs for rooms a user is standing in (hooks/NewRound_UserRoundTick.go:155). removeUntakenBaubles is also called from Room.Prepare (rooms.go:925) and from LoadRoomInstance (save_and_load.go:218). Those comments say this is so an expired item is gone "before anything can touch the room... a mob wandering in loads a room without preparing it". Spoilage did not copy those hooks. It also contradicts the floor-decay rule that nothing vanishes in front of anyone (floor_decay.go header): spoilage removes items only while a player is present, with a broadcast.

**Failure scenario.** A hunter butchers a deer, leaves the raw meat on the floor and walks off. Days later the meat has long passed SpoilAfter, but no player has been in the room, so it is never swept. A scavenger or other mob wandering in loads the room without Prepare and can pick up the rotten goods. Mob inventories have no spoilage sweep (the player sweep is hooks/spoilage.go), so the meat never rots. Alternatively, a player walks in and Prepare shows the rotten cut on look and lets them `get` it before the next round tick.

**Existing mechanism.** The bauble untaken sweep's call sites (Room.Prepare, LoadRoomInstance in internal/rooms/save_and_load.go) and the rooms/floor_decay.go policy.

**Suggested fix.** Call removeSpoiledGoods (without the broadcast) from Prepare and LoadRoomInstance next to removeUntakenBaubles. Better, fold the three floor-expiry predicates into one sweep helper so new expiry rules inherit the hooks.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
rooms.go:2767-2770 `r.removeUntakenBaubles(time.Now())` / `r.removeSpoiledGoods(roundNow)` appear only in RoundTick. grep shows removeUntakenBaubles is also called at rooms.go:925 (Prepare) and save_and_load.go:218 (LoadRoomInstance), and removeSpoiledGoods has no other callers. NewRound_UserRoundTick.go:155 `room.RoundTick()` is called per occupied room.
```

- **confirmed** (low): The claim holds as written. removeSpoiledGoods (internal/rooms/spoilage.go:20) has exactly one production caller, Room.RoundTick (rooms.go:2770). RoundTick is called only from hooks/NewRound_UserRoundTick.go:155, which loops over rooms.GetRoomsWithPlayers(). The sibling removeUntakenBaubles is also called from Room.Prepare (rooms.go:925, commented "Before anyone sees the floor") and from LoadRoomInstance (save_and_load.go:218, commented "gone before anything can touch the room... a mob wandering in loads a room without preparing it"). Spoilage did not copy either hook, even though its own doc says it is modelled on the bauble sweep.  The consequences reproduce from the code: (a) Rotten goods stay on the floor of an unoccupied room indefinitely. Only the probabilistic daily floor decay would eventually clear them. (b) A player entering such a room gets Prepare with no spoilage sweep. They see the rotten item and can `get` it before the next round tick. After that the player-side sweep (hooks/spoilage.go) throws it out within 10 rounds. IsSpoiled is enforced only in shops/buyrules.go and the harvest/player sweeps, not on get or crafting, so the window is real but short. (c) A mob can pick up floor items in a room with no players, for example a city scavenger via scavengerPickUp -> actions.TakeFloorItem with no spoilage check. Mob inventories have no spoilage sweep, since SpoiledItems is used only by hooks/spoilage.go for users.  Impact is small: these are cosmetic or short-window effects, and the scavenger case is tidying litter anyway. The "nothing vanishes in front of anyone" contradiction is weak, because the bauble sweep also runs in RoundTick with players present. The inconsistency with the sibling sweep's hooks is real, so the finding is confirmed at low severity.

</details>

<a id="f120"></a>
### F120 [low] reconcileShop re-implements the abundance seeding formula from RegisterShop

`internal/shops/persistence.go:188` · status **confirmed** · reported as low

reconcileShop seeds newly added template stock entries with its own copy of the fresh-shop formula, min(MaxStock, ceil(RestockQty*AbundanceThreshold)), which is already inline at persistence.go:119. The two copies already differ: the fresh-shop path forces Current = 0 for crafted entries (RestockQty <= 0), while reconcileShop leaves whatever Current the template carried. The two branches of the same RegisterShop also filter differently. A brand-new shop's seeding does not apply the NeverResold filter, while a reloaded shop's reconcile does.

**Failure scenario.** Someone retunes the seeding rule after the fresh-shop arbitrage notes, for example to seed at a different curve point. If they edit only the documented block at :110-125, new template entries added to existing shops keep the old rule, so fresh and reconciled shops open at different prices for the same item. Or a mob template authored with a non-zero Current on a crafted entry is zeroed on a fresh shop but kept on a reconciled one.

**Existing mechanism.** The seeding loop in shops.RegisterShop (internal/shops/persistence.go:110-125).

**Suggested fix.** Extract seedStockEntry(e StockEntry, cfg PricingConfig) StockEntry and call it from both branches. Apply the NeverResold filter in one place for both.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
persistence.go:119 `abundant := int(math.Ceil(float64(rq) * cfg.AbundanceThreshold))` (fresh seed, with `if rq <= 0 { Current = 0 }`). persistence.go:188 `abundant := int(math.Ceil(float64(e.RestockQty) * cfg.AbundanceThreshold))` (reconcile, with no else branch for RestockQty <= 0).
```

- **confirmed** (low): The PR does add a second copy of the abundance seeding formula. reconcileShop (persistence.go:187-193) repeats the fresh-shop seed at :113-124 (ceil(RestockQty*AbundanceThreshold) capped at MaxStock), and there is no shared helper. If someone retunes one copy and not the other, new entries in reconciled shops would open at a different price point than the same entries in fresh shops. That makes this a real maintainability/unification finding.  Two of the claimed divergences cannot happen today, though. The only caller, mobs/crafter.go:107, builds the template with RestockQty 5 or 3 and never sets Current. So the "crafted entry with a non-zero Current" case can't come from the current code.  The NeverResold asymmetry is real. The fresh path at :110-125 does not filter forged tools. reconcileShop removes them, but only on a later registration or reload. So a fresh shop whose template lists a tool above crude tier would stock it until the next reload. Whether this happens depends on content: a mob shop list would have to include such a tool, and I did not check that.  Low severity is right. This is duplicated logic with one latent inconsistency, not a live defect.

</details>

<a id="f121"></a>
### F121 [low] The species-recognition sight gate is bypassed by `chop <name>` and by chop's own messages

`internal/usercommands/chop.go:46` · status **confirmed** · reported as low

SurveyTrees hides a rare species' name unless SpeciesKnown wins a Perception/Search contest. Every other output of the same feature names the species regardless. Chop matches the player's argument against sp.Name and answers "There's no %s here" on a mismatch, so it works as an oracle. ResolveChop prints sp.Name in both the success and failure lines (chop.go:286, :289), and the logs received carry the species name. SpeciesKnown is also rerolled on every survey and every under-tiered chop, so spamming survey reveals the name anyway. The gate costs a contest-site registration but hides nothing, which is a visibility inconsistency inside one feature.

**Failure scenario.** A low-Perception player surveys and reads "an unfamiliar hardwood you can't put a name to". They type `chop ironwood`. If the job starts, the stand is ironwood; if the reply is "There's no ironwood here", it is not. Or they chop once and read "The ironwood creaks, leans and comes down".

**Suggested fix.** Route every player-facing species name through one helper that applies SpeciesKnown, cached per character and room (or per stand) so it is not rerolled. Make chopWordMatches accept the species name only when it is known.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
usercommands/chop.go:46 `if want := ...; want != `` && !chopWordMatches(want, sp.Name) { ... "There's no %s here..." }`. actions/chop.go:286 `...comes down with a crash...`, sp.Name, summarizeTaken(taken)`. chop.go:289 `Your axe keeps glancing off the %s`, sp.Name. chop.go:118-127 SpeciesKnown rolls contest.AgainstDifficulty on each call.
```

- **confirmed** (low): The claim holds as written. SurveyTrees (actions/chop.go:153-157) hides the species name behind SpeciesKnown. Chop (usercommands/chop.go:46) checks the player's argument against sp.Name with chopWordMatches (equality, prefix or substring) before any stock, axe or tier check, and answers "There's no %s here" when it does not match. Any argument that matches moves on to a different reply: the stumps line, no-axe, axe-too-poor, or the job starting. So `chop <guess>` tells you yes or no on the species. That works in the dark as well, because the CanSeeClearly gate runs before it. ResolveChop's success and failure lines (actions/chop.go:286, :289) print sp.Name without any SpeciesKnown check. SpeciesKnown (actions/chop.go:118-128) rolls contest.AgainstDifficulty fresh on every call, so repeating `survey` eventually shows the name. The same oracle shape is in the new mine.go:46 (mineWordMatches against ore.Name), and that file also has a "can't put a name to" gate at actions/mine.go:157, so mining probably shares the leak. The effect is cosmetic and only affects information, not gameplay, so low severity is right.

</details>
