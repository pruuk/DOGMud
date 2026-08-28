package mobs

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Crafter test helpers ──────────────────────────────────────────────────────

// craftContestStat is the perception a test crafter is given so its craft
// SCORE is nonzero.
//
// 🔴 Load-bearing since U10b-1b. Craft is now a CONTEST of
// stat + skill*SkillWeight against a recipe difficulty, and a bare test Mob has
// no stats at all. At score 0, dice.StdDevFor(0) is 0 and the attacker rolls a
// deterministic 0, so NO difficulty setting can force a win — the old helpers
// could not be repaired by tuning knobs alone.
const craftContestStat = 500

// giveCraftScore makes a test crafter capable of winning a craft contest.
// Call it on any mob passed to executeCraft / executeCraftLegacy.
func giveCraftScore(mob *Mob) {
	// ⚠️ Set .Base then Recalculate(), NOT .ValueAdj. crafting reads the stat
	// through characters.GetStatValue, which returns .Value — setting ValueAdj
	// leaves Value at 0 and the whole contest collapses to the skill term.
	mob.Character.Stats.Perception.Base = craftContestStat
	mob.Character.Stats.Strength.Base = craftContestStat
	mob.Character.Stats.Dexterity.Base = craftContestStat
	mob.Character.Stats.Perception.Recalculate()
	mob.Character.Stats.Strength.Recalculate()
	mob.Character.Stats.Dexterity.Recalculate()
}

// forceCraftSuccess makes the craft contest a near-certainty.
//
// It no longer touches CraftingBaseSuccessChance: that knob fed
// CalcSuccessChance, which U10b-1b retired as the craft decision. Forcing the
// contest instead means collapsing the difficulty to its floor AND suppressing
// the mercy floor, which otherwise flips 5% of ALL outcomes in either direction
// — including a guaranteed win.
//
// A tiny POSITIVE floor rather than 0, for HONESTY rather than necessity:
// 0 would in fact stick here (Config.Validate runs ONCE, at boot, and
// SetConfigForTest/AddOverlayOverrides never re-validate), but 0 is not a
// legal SHIPPED value and a test should not pin a state production forbids.
func forceCraftSuccess(t *testing.T) {
	t.Helper()
	prev := configs.GetBalanceConfig()
	configs.AddOverlayOverrides(map[string]any{
		"Balance.CraftBaseDifficulty": 1,
		"Balance.CraftSkillMinWeight": 0,
		"Balance.CraftFloor":          1e-12,
	})
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{
			"Balance.CraftBaseDifficulty": int(prev.CraftBaseDifficulty),
			"Balance.CraftSkillMinWeight": int(prev.CraftSkillMinWeight),
			"Balance.CraftFloor":          float64(prev.CraftFloor),
		})
	})
}

// forceCraftFailure makes the craft contest a near-impossibility, by the same
// mechanism in reverse: an enormous difficulty, with the mercy floor suppressed
// so it cannot rescue the attempt.
func forceCraftFailure(t *testing.T) {
	t.Helper()
	prev := configs.GetBalanceConfig()
	configs.AddOverlayOverrides(map[string]any{
		"Balance.CraftBaseDifficulty": 1000000,
		"Balance.CraftSkillMinWeight": 0,
		"Balance.CraftFloor":          1e-12,
	})
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{
			"Balance.CraftBaseDifficulty": int(prev.CraftBaseDifficulty),
			"Balance.CraftSkillMinWeight": int(prev.CraftSkillMinWeight),
			"Balance.CraftFloor":          float64(prev.CraftFloor),
		})
	})
}

// stubSaveShop replaces saveShopFn with a no-op that counts calls and
// returns nil. Restores the original on t.Cleanup and returns a pointer
// to the call count so tests can assert on it.
func stubSaveShop(t *testing.T) *int {
	t.Helper()
	orig := saveShopFn
	count := 0
	saveShopFn = func(zone string, mobId int, roomId int) error {
		count++
		return nil
	}
	t.Cleanup(func() { saveShopFn = orig })
	return &count
}

// makeCrafterMob returns a minimal Mob suitable for executeCraft tests.
func makeCrafterMob() *Mob {
	return &Mob{
		MobId:      MobId(9999),
		Zone:       "test_zone",
		HomeRoomId: 1,
		Crafter:    true,
		Character: characters.Character{
			Name: "Test Crafter",
		},
	}
}

// testRecipeID is used across executeCraft regression tests.
const testRecipeID = "test-crafter-recipe"

// ingredientTag and ingredientItemID are the component tag / item ID used as
// the recipe ingredient across all executeCraft tests.
const (
	crafterTestIngredientTag = "crafter-test-mat"
	crafterTestIngredientID  = 88001
	crafterTestOutputID      = 88002
)

// registerCrafterTestItems registers the minimal ItemSpecs needed by the
// executeCraft tests. Safe to call multiple times.
func registerCrafterTestItems() {
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId:       crafterTestIngredientID,
		Name:         "Test Material",
		ComponentTag: crafterTestIngredientTag,
	})
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: crafterTestOutputID,
		Name:   "Test Output Item",
	})
}

// registerCrafterTestRecipe registers the test recipe with 2 units of
// crafterTestIngredientTag → 1 crafterTestOutputID.
func registerCrafterTestRecipe() *crafting.RecipeSpec {
	recipe := &crafting.RecipeSpec{
		RecipeId:     testRecipeID,
		Name:         "Test Recipe",
		Skill:        "blacksmithing",
		SkillMinimum: 0,
		Ingredients: []crafting.RecipeIngredient{
			{ItemTag: crafterTestIngredientTag, Quantity: 2},
		},
		Output: crafting.RecipeOutput{ItemId: crafterTestOutputID, Quantity: 1},
	}
	crafting.RegisterRecipeForTest(recipe)
	return recipe
}

// newShopWithIngredients returns a freshly-registered ShopInventory that
// contains `qty` units of the test ingredient and is in the cache.
//
// RegisterShop seeds Current at the abundance level (not RestockQty), so we
// register then force Current=qty and RestockQty=0 to reflect crafter-item
// semantics (supply cart doesn't refill it, only crafting does).
func newShopWithIngredients(qty int) *shops.ShopInventory {
	shops.ClearCache()
	inv := shops.RegisterShop("test_zone", 9999, 1, shops.ShopInventory{
		Gold:         500,
		StartingGold: 500,
		Stock: []shops.StockEntry{
			{ItemId: crafterTestIngredientID, RestockQty: qty, MaxStock: 20},
		},
	})
	// Force the exact stock the test expects, then zero RestockQty so the
	// entry behaves as a crafter-sourced item.
	if e := inv.GetStock(crafterTestIngredientID); e != nil {
		e.Current = qty
		e.RestockQty = 0
	}
	return inv
}

// ── executeCraft regression tests ────────────────────────────────────────────

// TestTickMobCraft_SuppressesRestockInCaravanServedZones verifies that the
// balance config correctly identifies caravan-served zones, confirming the
// zone-check guard added to TickMobCraft is wired to the right predicate.
//
// Full end-to-end suppression (crafter in Stillwater not receiving auto-
// restocked materials) is verified by the Task 15 smoke test. This test
// confirms the helper callable from within the mobs package returns the
// correct value for both a served zone and a non-served zone.
func TestTickMobCraft_SuppressesRestockInCaravanServedZones(t *testing.T) {
	cfg := configs.GetBalanceConfig()

	// Stillwater is in the default CaravanServedZones list.
	require.True(t, cfg.IsCaravanServedZone("Stillwater"),
		"Stillwater must be in CaravanServedZones so TickMobCraft skips material restock")

	// A generic zone must NOT be identified as caravan-served.
	require.False(t, cfg.IsCaravanServedZone("TestZone"),
		"TestZone must not appear in CaravanServedZones")
}

// TestRegisterMobShop_SeedsStartingGoldFromYAML pins the 2026-05-04
// hotfix: RegisterMobShop must read mob.Character.Gold instead of
// hardcoding 500. Phase 7 of the vendor-types-economy-polish plan
// bumped specialist YAMLs to 1000g and general YAMLs to 5000g; the
// seeder previously ignored the field and re-seeded at 500g across
// the board.
func TestRegisterMobShop_SeedsStartingGoldFromYAML(t *testing.T) {
	defer shops.ClearCache()

	mob := &Mob{
		MobId:      9001,
		Zone:       "TestZone",
		HomeRoomId: 99,
		Character: characters.Character{
			Name: "test specialist",
			Gold: 1000,
			Shop: characters.Shop{}, // need at least one of shop/crafter
		},
		Crafter:                 true,
		CrafterRestockMaterials: []int{1},
		ShopCraftSupport:        "alchemy",
	}

	RegisterMobShop(mob)

	inv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
	require.NotNil(t, inv, "shop should be registered")
	assert.Equal(t, 1000, inv.StartingGold, "specialist should seed at 1000g from YAML, not the legacy 500g default")
	assert.Equal(t, 1000, inv.Gold, "current gold should match starting gold on fresh seed")
}

// TestPrewarmShopForSpawnPlacement_ForwardsGold pins the gold-forwarding
// in the synthetic Mob. The prewarm path runs at boot for every shop
// placement before the real mob spawns, so if it drops Gold the floor
// (500) wins everywhere — exactly what happened in prod after the
// 2026-05-04 hotfix landed but before this second-pass fix.
func TestPrewarmShopForSpawnPlacement_ForwardsGold(t *testing.T) {
	defer shops.ClearCache()

	template := &Mob{
		MobId: 9003,
		Zone:  "TestZone",
		Character: characters.Character{
			Name: "test general",
			Gold: 5000,
			Shop: characters.Shop{},
		},
		Crafter:                 true,
		CrafterRestockMaterials: []int{1},
		ShopCraftSupport:        "general",
	}

	PrewarmShopForSpawnPlacement(template, 99)

	inv := shops.GetShopInventory(template.Zone, int(template.MobId), 99)
	require.NotNil(t, inv, "prewarm should register the shop")
	assert.Equal(t, 5000, inv.StartingGold,
		"general-store gold from the template's YAML must flow through prewarm path, not get lost in the synthetic Mob")
}

// TestRegisterMobShop_AppliesGoldFloor pins the floor: if a content
// edit accidentally drops a mob's gold below 500, the seeder bumps
// it back up so the merchant has meaningful purchasing power.
func TestRegisterMobShop_AppliesGoldFloor(t *testing.T) {
	defer shops.ClearCache()

	mob := &Mob{
		MobId:      9002,
		Zone:       "TestZone",
		HomeRoomId: 99,
		Character: characters.Character{
			Name: "test underpaid",
			Gold: 100, // accidentally low
			Shop: characters.Shop{},
		},
		Crafter:                 true,
		CrafterRestockMaterials: []int{1},
		ShopCraftSupport:        "alchemy",
	}

	RegisterMobShop(mob)

	inv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
	require.NotNil(t, inv)
	assert.Equal(t, 500, inv.StartingGold, "values < 500 must floor up to 500")
}

// newShopWithStock registers a shop where the test ingredient has the given
// Current and MaxStock, allowing reserve-floor tests to control both.
// RestockQty is set to current for seeding, then zeroed to reflect crafter
// semantics (supply cart never refills).
func newShopWithStock(current, maxStock int) *shops.ShopInventory {
	shops.ClearCache()
	inv := shops.RegisterShop("test_zone", 9999, 1, shops.ShopInventory{
		Gold:         500,
		StartingGold: 500,
		Stock: []shops.StockEntry{
			{ItemId: crafterTestIngredientID, RestockQty: current, MaxStock: maxStock},
		},
	})
	if e := inv.GetStock(crafterTestIngredientID); e != nil {
		e.RestockQty = 0
	}
	return inv
}

// ── reserve-floor regression tests ────────────────────────────────────────────
// These five tests pin the CrafterIngredientReservePct behavior introduced to
// prevent crafter mobs from draining their own ingredient stock to a single
// unit and leaving players unable to buy.
//
// All tests exercise HasMaterialsWithReservePct directly — the function that
// holds the per-ingredient reserve check in the shop decision path.

// makeReserveRecipe builds a minimal single-ingredient recipe for reserve tests.
func makeReserveRecipe(ingTag string, ingQty int) *crafting.RecipeSpec {
	return &crafting.RecipeSpec{
		RecipeId: "reserve-test-recipe",
		Name:     "Reserve Test Recipe",
		Skill:    "blacksmithing",
		Ingredients: []crafting.RecipeIngredient{
			{ItemTag: ingTag, Quantity: ingQty},
		},
		Output: crafting.RecipeOutput{ItemId: crafterTestOutputID, Quantity: 1},
	}
}

// makeReserveShop builds a ShopInventory with a single stock entry at the
// specified Current / MaxStock — bypassing RegisterShop disk/cache logic.
func makeReserveShop(current, maxStock int) *shops.ShopInventory {
	return &shops.ShopInventory{
		Stock: []shops.StockEntry{
			{ItemId: crafterTestIngredientID, RestockQty: 0, MaxStock: maxStock, Current: current},
		},
	}
}

// TestReserveFloor_BlocksWhenBelowReserve verifies that HasMaterialsWithReservePct
// returns false when consuming would drop stock below MaxStock×ReservePct.
// MaxStock=20, Current=7, recipe needs 3, reserve=25%×20=5: 7-3=4 < 5 → blocked.
func TestReserveFloor_BlocksWhenBelowReserve(t *testing.T) {
	registerCrafterTestItems()
	recipe := makeReserveRecipe(crafterTestIngredientTag, 3)
	inv := makeReserveShop(7, 20)

	if shops.HasMaterialsWithReservePct(recipe, inv, 0.25) {
		t.Error("expected false when consuming 3 from Current=7 would drop to 4, below reserve=5")
	}
}

// TestReserveFloor_AllowsAtExactFloor verifies that HasMaterialsWithReservePct
// returns true when consuming leaves stock exactly at the reserve.
// MaxStock=20, Current=8, recipe needs 3, reserve=5: 8-3=5 = reserve → allowed.
func TestReserveFloor_AllowsAtExactFloor(t *testing.T) {
	registerCrafterTestItems()
	recipe := makeReserveRecipe(crafterTestIngredientTag, 3)
	inv := makeReserveShop(8, 20)

	if !shops.HasMaterialsWithReservePct(recipe, inv, 0.25) {
		t.Error("expected true when consuming 3 from Current=8 leaves exactly reserve=5")
	}
}

// TestReserveFloor_HardFloorOfOneWithSmallMaxStock verifies that with a very
// small MaxStock, 25% of 2 = 0.5 → int(0.5)=0 → floor bumps to 1.
// MaxStock=2, Current=2, recipe needs 1: reserve=max(1,0)=1, 2-1=1 >= 1 → allowed.
func TestReserveFloor_HardFloorOfOneWithSmallMaxStock(t *testing.T) {
	registerCrafterTestItems()
	recipe := makeReserveRecipe(crafterTestIngredientTag, 1)
	inv := makeReserveShop(2, 2)

	if !shops.HasMaterialsWithReservePct(recipe, inv, 0.25) {
		t.Error("expected true when hard floor of 1 still allows craft (Current=2, needs 1, reserve=1)")
	}
}

// TestReserveFloor_BelowHardFloor verifies that MaxStock=2, Current=1,
// recipe needs 1: 1-1=0 < reserve=1 (hard floor) → blocked.
func TestReserveFloor_BelowHardFloor(t *testing.T) {
	registerCrafterTestItems()
	recipe := makeReserveRecipe(crafterTestIngredientTag, 1)
	inv := makeReserveShop(1, 2)

	if shops.HasMaterialsWithReservePct(recipe, inv, 0.25) {
		t.Error("expected false when Current=1, needs 1, reserve=1 → would drop to 0 < floor")
	}
}

// TestReserveFloor_PerIngredientCheck verifies that a recipe is blocked if ANY
// ingredient would drop below its reserve, even when others are fine.
func TestReserveFloor_PerIngredientCheck(t *testing.T) {
	const crafterTestIngredientID2 = 88003
	const crafterTestIngredientTag2 = "crafter-test-mat-2"

	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId:       crafterTestIngredientID2,
		Name:         "Test Material 2",
		ComponentTag: crafterTestIngredientTag2,
	})
	registerCrafterTestItems()

	// Two-ingredient recipe: 2 of mat1 + 2 of mat2 → output
	recipe := &crafting.RecipeSpec{
		RecipeId: "reserve-test-two-ing",
		Name:     "Two-Ingredient Reserve Test",
		Skill:    "blacksmithing",
		Ingredients: []crafting.RecipeIngredient{
			{ItemTag: crafterTestIngredientTag, Quantity: 2},
			{ItemTag: crafterTestIngredientTag2, Quantity: 2},
		},
		Output: crafting.RecipeOutput{ItemId: crafterTestOutputID, Quantity: 1},
	}

	inv := &shops.ShopInventory{
		Stock: []shops.StockEntry{
			// mat1: MaxStock=20, Current=8 → reserve=5, 8-2=6 >= 5 → OK
			{ItemId: crafterTestIngredientID, RestockQty: 0, MaxStock: 20, Current: 8},
			// mat2: MaxStock=20, Current=6 → reserve=5, 6-2=4 < 5 → BLOCKED
			{ItemId: crafterTestIngredientID2, RestockQty: 0, MaxStock: 20, Current: 6},
		},
	}

	if shops.HasMaterialsWithReservePct(recipe, inv, 0.25) {
		t.Error("expected false when second ingredient (mat2) would drop below reserve floor")
	}
}

// ── executeCraft regression tests ────────────────────────────────────────────
// These tests are the load-bearing regression suite for the crafter-consumption
// tracking fix. Each one will fail immediately if a future refactor reverts to
// the round-naive RemoveStock/AddStock path or drops the SaveShop call.

// TestExecuteCraft_ConsumesIngredientsViaRoundAwareRemove is THE regression
// check for bug #1: crafter-caused depletion must push CurrentDepletion so the
// dashboard's TtR scoring can see it.
func TestExecuteCraft_ConsumesIngredientsViaRoundAwareRemove(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	shopInv := newShopWithIngredients(2)
	_ = stubSaveShop(t)

	mob := makeCrafterMob()
	giveCraftScore(mob)

	// Run executeCraft (success or failure doesn't matter for ingredient consumption).
	executeCraft(mob, recipe, shopInv)

	// Ingredient stock should be zero.
	entry := shopInv.GetStock(crafterTestIngredientID)
	require.NotNil(t, entry, "ingredient entry must exist after craft")
	assert.Equal(t, 0, entry.Current,
		"ingredient stock must reach 0 after craft consumes 2 of 2")

	// THE regression check: CurrentDepletion must be set, proving RemoveStockAtRound
	// was used. If someone switches back to RemoveStock (round=0), this map stays
	// empty and this assertion fires.
	depRound, marked := shopInv.CurrentDepletion[crafterTestIngredientID]
	assert.True(t, marked,
		"CurrentDepletion[ingredientID] must be set — crafter depletion must be round-aware")
	assert.Greater(t, depRound, uint64(0),
		"depleted round must be > 0 (got 0 means round-naive RemoveStock was called)")

	// Counter check: undercounting bugs surface here.
	assert.Equal(t, 2, shopInv.ConsumedByCrafterCount,
		"ConsumedByCrafterCount must equal the actual units consumed (2)")
}

// TestExecuteCraft_SuccessAddsOutputViaRoundAwareAdd is THE regression check
// for bug #2: craft output must push a completed StockEvent when the output
// slot was previously depleted (Kerra's longsword bug).
func TestExecuteCraft_SuccessAddsOutputViaRoundAwareAdd(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	_ = stubSaveShop(t)
	forceCraftSuccess(t)

	// Pre-condition: output slot depleted and marked.
	shops.ClearCache()
	shopInv := shops.RegisterShop("test_zone", 9999, 1, shops.ShopInventory{
		Gold:         500,
		StartingGold: 500,
		Stock: []shops.StockEntry{
			{ItemId: crafterTestIngredientID, RestockQty: 0, MaxStock: 20, Current: 2},
			{ItemId: crafterTestOutputID, RestockQty: 0, MaxStock: 10, Current: 0},
		},
	})
	// Mark the output as currently depleted (round 500) — simulates it having
	// sold out before the crafter had a chance to make more.
	if shopInv.CurrentDepletion == nil {
		shopInv.CurrentDepletion = map[int]uint64{}
	}
	shopInv.CurrentDepletion[crafterTestOutputID] = 500

	mob := makeCrafterMob()
	giveCraftScore(mob)
	executeCraft(mob, recipe, shopInv)

	// Output must be present.
	outEntry := shopInv.GetStock(crafterTestOutputID)
	require.NotNil(t, outEntry, "output StockEntry must exist after craft")
	assert.GreaterOrEqual(t, outEntry.Current, 1,
		"output stock must be >= 1 after successful craft")

	// THE regression check: CurrentDepletion for the output must be cleared,
	// and a completed StockEvent must have been pushed. If someone switches back
	// to AddStock (round=0), StockEvents stays empty and this fires.
	_, stillDepleted := shopInv.CurrentDepletion[crafterTestOutputID]
	assert.False(t, stillDepleted,
		"CurrentDepletion[outputID] must be cleared after craft refilled the slot")

	events := shopInv.StockEvents[crafterTestOutputID]
	require.NotEmpty(t, events,
		"StockEvents[outputID] must have at least one completed event — AddStockAtRound must have been used")
	assert.Greater(t, events[0].RefilledRound, uint64(0),
		"completed StockEvent must have RefilledRound > 0")
}

// TestExecuteCraft_SuccessCallsSaveShop verifies that executeCraft calls the
// save hook after a successful craft. Persistence is required for freshly-
// crafted items to survive a server restart.
func TestExecuteCraft_SuccessCallsSaveShop(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	shopInv := newShopWithIngredients(2)
	forceCraftSuccess(t)
	saveCount := stubSaveShop(t)

	mob := makeCrafterMob()
	giveCraftScore(mob)
	executeCraft(mob, recipe, shopInv)

	assert.Equal(t, 1, *saveCount,
		"executeCraft must call SaveShop exactly once after a successful craft")
}

// TestExecuteCraft_FailureStillConsumesIngredients verifies that even a
// failed craft burns its materials and increments ConsumedByCrafterCount.
func TestExecuteCraft_FailureStillConsumesIngredients(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	shopInv := newShopWithIngredients(2)
	_ = stubSaveShop(t)
	forceCraftFailure(t)

	mob := makeCrafterMob()
	giveCraftScore(mob)
	result := executeCraft(mob, recipe, shopInv)

	assert.False(t, result.Success, "craft must have failed with forced-failure config")

	entry := shopInv.GetStock(crafterTestIngredientID)
	require.NotNil(t, entry)
	assert.Equal(t, 0, entry.Current,
		"failed craft must still consume ingredients (Current must reach 0)")

	assert.Equal(t, 2, shopInv.ConsumedByCrafterCount,
		"ConsumedByCrafterCount must reflect consumption even on failure")
}

// TestExecuteCraft_OutputItemFreshlyCreated is the direct regression test for
// the Kerra "craft success not in shop list" bug. When the output item has no
// pre-existing StockEntry, executeCraft must create one and GetStock must
// return a non-nil entry with Current >= 1 after a successful craft.
func TestExecuteCraft_OutputItemFreshlyCreated(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	_ = stubSaveShop(t)
	forceCraftSuccess(t)

	// Shop has ONLY the ingredient — no StockEntry for the output at all.
	shops.ClearCache()
	shopInv := shops.RegisterShop("test_zone", 9999, 1, shops.ShopInventory{
		Gold:         500,
		StartingGold: 500,
		Stock: []shops.StockEntry{
			{ItemId: crafterTestIngredientID, RestockQty: 0, MaxStock: 20, Current: 2},
		},
	})

	// Verify pre-condition: output entry doesn't exist yet.
	require.Nil(t, shopInv.GetStock(crafterTestOutputID),
		"pre-condition: output item must not be in stock before craft")

	mob := makeCrafterMob()
	giveCraftScore(mob)
	result := executeCraft(mob, recipe, shopInv)

	require.True(t, result.Success, "craft must succeed with forced-success config")

	// THE Kerra regression check: GetStock must find the entry.
	outEntry := shopInv.GetStock(crafterTestOutputID)
	require.NotNil(t, outEntry,
		"GetStock(outputID) must be non-nil — AddStockAtRound must create the entry when missing")
	assert.GreaterOrEqual(t, outEntry.Current, 1,
		"newly-created output StockEntry must have Current >= 1")
}

// ── schedule gate helpers ─────────────────────────────────────────────────────

// roundForHourCrafter scans all rounds in one game day and returns the first
// roundOfDay whose Hour24 equals targetHour. Returns -1 when the target hour
// is not representable with the current RoundsPerDay config.
func roundForHourCrafter(hour int) int {
	c := configs.GetTimingConfig()
	rpd := int(c.RoundsPerDay)
	for r := 0; r < rpd; r++ {
		hourFloat, _ := math.Modf(float64(r) / float64(rpd) * 24)
		if int(hourFloat) == hour {
			return r + rpd // offset so we never send round 0
		}
	}
	return -1
}

// setTestHourForCrafter pins the global round count so gametime.GetDate()
// resolves to a known Hour24. Returns a cleanup func that restores the
// previous round count. Usage: defer setTestHourForCrafter(t, 19)()
func setTestHourForCrafter(t *testing.T, hour int) func() {
	t.Helper()
	prev := util.GetRoundCount()
	r := roundForHourCrafter(hour)
	if r < 0 {
		t.Skipf("hour %d not representable with RoundsPerDay=%d — skip schedule gate test",
			hour, configs.GetTimingConfig().RoundsPerDay)
	}
	util.SetRoundCountForTest(uint64(r))
	actual := gametime.GetDate().Hour24
	if actual != hour {
		util.SetRoundCountForTest(prev)
		t.Skipf("setTestHourForCrafter(%d): gametime resolved Hour24=%d — skip schedule gate test",
			hour, actual)
	}
	return func() { util.SetRoundCountForTest(prev) }
}

// minimalCrafterMobForTest returns the smallest Mob that passes TickMobCraft's
// first three guards (Crafter=true, CrafterEnabled by config, Aggro==nil).
// It force-enables CrafterEnabled for the duration of the test and restores
// the original value via t.Cleanup.
func minimalCrafterMobForTest(t *testing.T) *Mob {
	t.Helper()
	prev := configs.GetBalanceConfig()
	configs.AddOverlayOverrides(map[string]any{
		"Balance.CrafterEnabled": true,
	})
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{
			"Balance.CrafterEnabled": bool(prev.CrafterEnabled),
		})
	})
	return &Mob{
		MobId:      MobId(9998),
		Zone:       "test_zone",
		HomeRoomId: 1,
		Crafter:    true,
		Character: characters.Character{
			Name: "Schedule Gate Test Crafter",
		},
	}
}

// ── schedule gate tests ───────────────────────────────────────────────────────

// TestTickMobCraft_ScheduleGate_BlocksWhenActivityNotCraft verifies that
// TickMobCraft returns nil during a schedule segment whose activity is not
// "craft" (the tavern segment in the Kerra fixture, hour 19).
func TestTickMobCraft_ScheduleGate_BlocksWhenActivityNotCraft(t *testing.T) {
	mob := minimalCrafterMobForTest(t)
	giveCraftScore(mob)
	mob.ScheduleId = "thornwall_smith"
	mob.Character.RoomId = 9012 // tavern room — activity="" in fixture

	registerScheduleForTest(kerraTestSchedule())
	defer unregisterScheduleForTest("thornwall_smith")

	// Pin to hour 19: Kerra's tavern segment (18-22, activity="").
	defer setTestHourForCrafter(t, 19)()

	result := TickMobCraft(mob)
	if result != nil {
		t.Errorf("expected nil during non-craft segment (hour=19, activity=\"\"), got %+v", result)
	}
}

// TestTickMobCraft_ScheduleGate_NoScheduleIsUnaffected verifies that the gate
// is a no-op for mobs without a schedule_id. The function may return nil for
// other reasons (e.g., no shop, no recipes) but the gate must not preempt.
// We confirm this by asserting the gate doesn't panic and the function returns
// cleanly — we don't assert the result value.
func TestTickMobCraft_ScheduleGate_NoScheduleIsUnaffected(t *testing.T) {
	mob := minimalCrafterMobForTest(t)
	giveCraftScore(mob)
	mob.ScheduleId = "" // no schedule — gate must be a no-op

	// Should complete without panic regardless of result.
	_ = TickMobCraft(mob)
}

// TestExecuteCraft_SuccessAlwaysRoutesToShop is the regression for the
// 2026-05-06 fix: Priority-1 self-gear-upgrade crafts (recipes where
// pickSelfGearRecipe flagged the output as an upgrade for the mob)
// previously called mob.Character.StoreItem(itm), routing the output
// to the mob's inventory rather than the shop. Iron daggers, bucklers,
// and other gear-grade items never reached the shop list. The fix:
// executeCraft always routes output to shopInv.Stock regardless of
// the craft selection priority.
func TestExecuteCraft_SuccessAlwaysRoutesToShop(t *testing.T) {
	registerCrafterTestItems()
	recipe := registerCrafterTestRecipe()
	shopInv := newShopWithIngredients(2)
	_ = stubSaveShop(t)
	forceCraftSuccess(t)

	mob := makeCrafterMob()
	giveCraftScore(mob)
	preInventoryCount := len(mob.Character.Items)

	result := executeCraft(mob, recipe, shopInv)
	require.True(t, result.Success)

	// Output must be in shop stock.
	outEntry := shopInv.GetStock(crafterTestOutputID)
	require.NotNil(t, outEntry,
		"output must land in shopInv.Stock, not mob inventory")
	assert.GreaterOrEqual(t, outEntry.Current, 1)

	// Mob's backpack count must be unchanged — output must NOT have
	// gone to mob.Character.StoreItem.
	assert.Equal(t, preInventoryCount, len(mob.Character.Items),
		"output must not be stored in mob's inventory; that was the Kerra "+
			"regression where gear-grade crafts disappeared from the shop")
}

// ── TickMobShopBaselineRestock tests ─────────────────────────────────────────

func TestTickMobShopBaselineRestock_NoOpForCrafter(t *testing.T) {
	mob := &Mob{Crafter: true}
	if TickMobShopBaselineRestock(mob) {
		t.Fatal("crafter should no-op (uses TickMobCraft path)")
	}
}

func TestTickMobShopBaselineRestock_RefillsCommonTiersOnCadence(t *testing.T) {
	// Override RoundsPerDay to 48 so roundsPerHour=2 (non-zero). The default
	// RoundsPerDay=20 gives roundsPerHour=0 which makes cadence=0 and breaks
	// the before/after cadence assertions. Restore on cleanup.
	prevRPD := int(configs.GetTimingConfig().RoundsPerDay)
	if err := configs.AddOverlayOverrides(map[string]any{"Timing.RoundsPerDay": 48}); err != nil {
		t.Fatalf("failed to override RoundsPerDay: %v", err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"Timing.RoundsPerDay": prevRPD})
	})

	items.RegisterTestItemSpec(&items.ItemSpec{ItemId: 7001, RarityTier: 50})
	items.RegisterTestItemSpec(&items.ItemSpec{ItemId: 7002, RarityTier: 30})
	shops.RegisterShop("gsr_zone", 7700, 1, shops.ShopInventory{
		Zone:         "gsr_zone",
		StartingGold: 500,
		Stock: []shops.StockEntry{
			{ItemId: 7001, MaxStock: 20, RestockQty: 5},
			{ItemId: 7002, MaxStock: 20, RestockQty: 5},
		},
	})
	inv := shops.GetShopInventory("gsr_zone", 7700, 1)
	inv.RemoveStock(7001, inv.GetStock(7001).Current) // RegisterShop seeds Current; deplete to 0
	inv.RemoveStock(7002, inv.GetStock(7002).Current)

	mob := &Mob{Zone: "gsr_zone", HomeRoomId: 1}
	mob.MobId = 7700

	b := configs.GetBalanceConfig()
	roundsPerHour := uint64(configs.GetTimingConfig().RoundsPerDay) / 24
	cadence := uint64(shops.RestockCadenceHours(b, 50)) * roundsPerHour
	if cadence == 0 {
		t.Skip("cadence=0 even after RoundsPerDay override — skip cadence assertions")
	}

	util.SetRoundCountForTest(100000)
	if TickMobShopBaselineRestock(mob) {
		t.Fatal("first call should stamp, not restock")
	}
	util.SetRoundCountForTest(100000 + cadence - 1)
	if TickMobShopBaselineRestock(mob) {
		t.Fatal("before cadence should not restock")
	}
	util.SetRoundCountForTest(100000 + cadence + 1)
	if !TickMobShopBaselineRestock(mob) {
		t.Fatal("after cadence should restock the common tier")
	}
	inv = shops.GetShopInventory("gsr_zone", 7700, 1)
	if got := inv.GetStock(7001).Current; got != 5 {
		t.Fatalf("tier-50 entry: Current = %d, want 5", got)
	}
	if got := inv.GetStock(7002).Current; got != 0 {
		t.Fatalf("tier-30 entry should stay caravan-gated: Current = %d, want 0", got)
	}
}

// U10b-1 Task 17: a FAILED mob craft trains at ProgressionFailureFraction.
//
// The mob-crafter path was success-only, exactly as the player paths were
// before Task 16. Ingredients are consumed regardless of the roll, so a failure
// already cost the crafter materials and taught them nothing.
//
// ⚠️ pinConfigForTest MUST set MobProgressionEnabled -- it is a ConfigBool, so
// it stays FALSE in a test binary, and every mob-side progression assertion
// would otherwise be checking a path that returns 0 without ever reaching the
// award. The plan calls this out for exactly this task.
//
// The verdict is pinned by collapsing the craft CONTEST's difficulty onto one
// value, so the roll is exact rather than statistical.
// pinMobCraftChance pins the craft outcome to certain success (pct >= 100) or
// certain failure (pct <= 0).
//
// It no longer pins CraftingMin/MaxSuccessChance: U10b-1b retired
// CalcSuccessChance as the craft decision, so those knobs no longer affect the
// outcome and a test pinning them would pass or fail at random. It now collapses
// or inflates the CONTEST difficulty, and suppresses the mercy floor, which
// otherwise flips 5% of outcomes in both directions.
//
// CraftFloor takes a tiny POSITIVE value, not 0: the validator corrects <=0 back
// to 0.05 because a 0 floor is not a legal shipped value.
func pinMobCraftChance(t *testing.T, pct int) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	if pct >= 100 {
		cfg.Balance.CraftBaseDifficulty = 1
	} else {
		cfg.Balance.CraftBaseDifficulty = 1000000
	}
	cfg.Balance.CraftSkillMinWeight = 0
	cfg.Balance.CraftFloor = configs.ConfigFloat(1e-12)
	configs.SetConfigForTest(t, cfg)
}

func mobCrafterForProgressionTest(skill string) *Mob {
	m := &Mob{MobId: MobId(9302), Zone: "test_zone", Crafter: true}
	m.Character.Name = "Progression Crafter"
	m.Character.Skills = map[string]int{skill: 1}
	// Craft is a contest since U10b-1b, so a crafter with no stats scores 0 and
	// can never win regardless of difficulty.
	giveCraftScore(m)
	return m
}

func TestExecuteCraftLegacy_AFailedCraftStillAwards(t *testing.T) {
	const skill = "blacksmithing"

	// Precondition, asserted rather than assumed: without MobProgressionEnabled
	// this test would pass against an award that never fires.
	pinMobCraftChance(t, 0) // difficulty inflated + floor suppressed: ALWAYS fails
	if !bool(configs.GetBalanceConfig().MobProgressionEnabled) {
		t.Fatal("precondition: MobProgressionEnabled must be true or every mob award returns 0")
	}

	mob := mobCrafterForProgressionTest(skill)
	before := mob.Character.GetSkillUseCount(skill)

	executeCraftLegacy(mob, &crafting.RecipeSpec{
		RecipeId: "u10b1-mobcraft-fail", Skill: skill, SkillMinimum: 0,
	})

	if got := mob.Character.GetSkillUseCount(skill) - before; got != 1 {
		t.Fatalf("%s use count rose by %d after a FAILED mob craft, want 1", skill, got)
	}
}

// The mirror, so the test above cannot pass against an implementation that
// awards on failure only or awards twice.
func TestExecuteCraftLegacy_ASucceedingCraftAwardsOnce(t *testing.T) {
	const skill = "blacksmithing"

	pinMobCraftChance(t, 100) // always succeeds

	mob := mobCrafterForProgressionTest(skill)
	before := mob.Character.GetSkillUseCount(skill)

	result := executeCraftLegacy(mob, &crafting.RecipeSpec{
		RecipeId: "u10b1-mobcraft-win", Skill: skill, SkillMinimum: 0,
	})

	if !result.Success {
		t.Fatal("fixture premise: a chance of 100 must succeed")
	}
	if got := mob.Character.GetSkillUseCount(skill) - before; got != 1 {
		t.Fatalf("%s use count rose by %d after a successful mob craft, want exactly 1", skill, got)
	}
}

// The mob-crafter award scales with recipe difficulty, like every other craft
// site.
//
// This path used bare OnSkillUse and had NEVER applied
// 1 + SkillMinimum*CraftDifficultyProgressionScale, so a shopkeeper crafting a
// demanding recipe trained exactly as fast as one crafting a trivial one -- a
// firing-rule inconsistency, since the bonus is part of the AWARD rather than
// the resolution.
//
// Exact, not statistical: at ProgressionFailureFraction 0 a LOSING award
// short-circuits before any roll whatever the bonus, so a bonus of 0 must
// suppress a WINNING award too. If the bonus were being dropped, the win would
// advance regardless and this fails.
func TestExecuteCraftLegacy_TheAwardScalesWithRecipeDifficulty(t *testing.T) {
	const skill = "blacksmithing"

	pinMobCraftChance(t, 100) // always succeeds, so only the BONUS can suppress it
	cfg := configs.GetConfig()
	cfg.Balance.CraftDifficultyProgressionScale = 0
	cfg.Balance.BaseProgressionChance = 1.0
	cfg.Balance.StatProgressionRate = 1.0
	cfg.Balance.SkillProgressionMultipliers = nil
	configs.SetConfigForTest(t, cfg)

	// SkillMinimum 0 with scale 0 gives craftBonus exactly 1.0 -- the control.
	control := mobCrafterForProgressionTest(skill)
	beforeControl := control.Character.Skills[skill]
	executeCraftLegacy(control, &crafting.RecipeSpec{
		RecipeId: "u10b1-bonus-control", Skill: skill, SkillMinimum: 0,
	})
	if got := control.Character.Skills[skill] - beforeControl; got != 1 {
		t.Fatalf("control: %s advanced by %d at bonus 1.0, want 1; the fixture is not pinned to certainty", skill, got)
	}

	// A NEGATIVE scale drives craftBonus to zero at SkillMinimum 100, which can
	// only reach the award if the bonus is actually applied.
	cfg = configs.GetConfig()
	cfg.Balance.CraftDifficultyProgressionScale = configs.ConfigFloat(-0.01)
	configs.SetConfigForTest(t, cfg)

	suppressed := mobCrafterForProgressionTest(skill)
	beforeSuppressed := suppressed.Character.Skills[skill]
	executeCraftLegacy(suppressed, &crafting.RecipeSpec{
		RecipeId: "u10b1-bonus-zero", Skill: skill, SkillMinimum: 100,
	})
	if got := suppressed.Character.Skills[skill] - beforeSuppressed; got != 0 {
		t.Errorf("%s advanced by %d on a WON craft whose difficulty bonus is 0; the bonus is not reaching the award", skill, got)
	}
	if got := suppressed.Character.GetSkillUseCount(skill); got == 0 {
		t.Error("the award did not fire at all; the bonus must scale the weight, not skip the award")
	}
}
