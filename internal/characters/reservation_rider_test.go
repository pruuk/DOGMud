package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Fixture note, verified rather than assumed: Validate() runs ensureAllSkills,
// which floors EVERY known skill at rank 1. So a rank set before Validate() is
// silently raised to 1 and a "rank 0" fixture would quietly test rank 1
// instead. Every fixture below therefore sets the enchanting rank AFTER
// Validate(), which is also why rank 1 is asserted alongside rank 0: rank 1 is
// the lowest an actually-validated character ever reads.
func withEnchantingRank(c *Character, rank int) *Character {
	c.Skills[string(skills.Enchanting)] = rank
	return c
}

// D10 section 4.2: an enchantment's reservation scales by the WEARER's
// enchanting rank through the U7 inverse-skill band. The penalty half applies,
// consistently with the companion side: a rank-0 wearer pays 1.10x.
func TestGetPoolReservation_ScalesEnchantmentsByEnchantingRank(t *testing.T) {
	defer enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-drain": {
			EnchantId:   "test-drain",
			Name:        "Test Drain",
			ReservePool: "stamina",
			Tiers:       []enchantments.TierDef{{Tier: 0, ReservePct: 0.10}},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999940: {ItemId: 999940, Name: "drained cloak", Type: items.Neck},
	})()

	newWearer := func(enchantingRank int) *Character {
		c := New()
		c.StaminaMax.Base = 1000
		itm := items.New(999940)
		itm.EnchantType = "test-drain"
		itm.EnchantTier = 0
		itm.ReservePool = "stamina"
		c.Equipment.Neck = itm
		c.Validate()
		return withEnchantingRank(c, enchantingRank)
	}

	// rank 0   -> 0.10 * 1.100 = 0.1100 -> 110
	// rank 1   -> 0.10 * 1.096 = 0.1096 -> 109
	// rank 25  -> 0.10 * 1.000 = 0.1000 -> 100
	// rank 100 -> 0.10 * 0.400 = 0.0400 ->  40
	for _, tc := range []struct {
		rank int
		want int
	}{
		{0, 110},
		{1, 109},
		{25, 100},
		{100, 40},
	} {
		c := newWearer(tc.rank)
		if c.StaminaMax.Value != 1000 {
			t.Fatalf("test setup invariant broken: StaminaMax.Value = %d, want 1000", c.StaminaMax.Value)
		}
		if got := c.GetPoolReservation("stamina", 1000); got != tc.want {
			t.Errorf("enchanting %d: reservation = %d, want %d", tc.rank, got, tc.want)
		}
	}
}

// Pinnacle-item reserve_*_pct is deliberately NOT scaled. That reservation is
// the item's price, not a piece of craft the wearer's skill has any purchase on,
// and scaling it would hand every enchanter a discount on gear they never made.
func TestGetPoolReservation_DoesNotScalePinnacleItemReserve(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999941: {ItemId: 999941, Name: "pinnacle band", Type: items.Ring, ReserveStaminaPct: 0.10},
	})()

	for _, rank := range []int{0, 1, 25, 100} {
		c := New()
		c.StaminaMax.Base = 1000
		c.Equipment.Ring = items.New(999941)
		c.Validate()
		withEnchantingRank(c, rank)
		if got := c.GetPoolReservation("stamina", 1000); got != 100 {
			t.Errorf("enchanting %d: pinnacle reservation = %d, want a flat 100", rank, got)
		}
	}
}

// The per-item helper and the total must agree by construction: the enforcement
// sites price a swap with the former and test it against the latter. Before this
// task GetPoolReservation ran its own inlined copy of the loop with no rider, so
// the two silently returned different numbers and nothing went red.
//
// The fixture deliberately mixes an enchanted item, a plain pinnacle item and a
// stacking item that reserves through BOTH mechanisms, at an enchanting rank
// that is neither the neutral rank nor the cap, so any future re-inlining of
// either half fails here.
func TestItemReserveOnPool_SumsToGetPoolReservation(t *testing.T) {
	defer enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-drain": {
			EnchantId:   "test-drain",
			Name:        "Test Drain",
			ReservePool: "stamina",
			Tiers:       []enchantments.TierDef{{Tier: 0, ReservePct: 0.09}},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999942: {ItemId: 999942, Name: "band a", Type: items.Ring, ReserveStaminaPct: 0.07},
		999943: {ItemId: 999943, Name: "band b", Type: items.Ring, ReserveStaminaPct: 0.11},
		999944: {ItemId: 999944, Name: "drained cowl", Type: items.Head},
	})()

	c := New()
	c.StaminaMax.Base = 1000
	c.Equipment.Ring = items.New(999942)

	// Ring2 reserves through the spec AND carries an enchantment, the stacking
	// case the per-item helper documents.
	band := items.New(999943)
	band.EnchantType = "test-drain"
	band.EnchantTier = 0
	band.ReservePool = "stamina"
	c.Equipment.Ring2 = band

	cowl := items.New(999944)
	cowl.EnchantType = "test-drain"
	cowl.EnchantTier = 0
	cowl.ReservePool = "stamina"
	c.Equipment.Head = cowl

	c.Validate()
	withEnchantingRank(c, 54)

	sum := 0
	for _, itm := range c.Equipment.GetAllItems() {
		sum += c.ItemReserveOnPool(itm, PoolStamina)
	}
	total := c.GetPoolReservation("stamina", 1000)
	if sum != total {
		t.Fatalf("per-item sum %d != GetPoolReservation %d; the two must not be able to drift", sum, total)
	}
	// Pin the absolute figure too, so a change that breaks BOTH halves the same
	// way cannot slip through an equality-only assertion.
	// rank 54 -> 1.00 + (0.40 - 1.00) * (54 - 25) / (100 - 25) = 0.768.
	// spec: floor(1000 * 0.07) + floor(1000 * 0.11) = 70 + 110 = 180.
	// enchant x2: floor(1000 * 0.09 * 0.768) = 69 each = 138.
	if total != 318 {
		t.Errorf("total reservation = %d, want 318 (180 spec + 138 enchant at rank 54)", total)
	}
}
