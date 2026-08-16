package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// seedEnchantCapFixtures stands up the species, item and enchantment data every
// test in this file needs. internal/hooks loads no data files, so without the
// species seed anything that reaches HandsRequired panics, and without the item
// seed DisplayName has no spec to read.
//
// "test-greed" is deliberately steep: tier 0 holds 30% of the pool and tier 1
// holds 60%, which is the DOUBLING that makes the tier-up a real breach risk.
// Real enchantments double the same way at low tiers (1% to 2% to 4%).
func seedEnchantCapFixtures() func() {
	restoreSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	})
	restoreEnchants := enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-greed": {
			EnchantId:   "test-greed",
			Name:        "greed",
			ReservePool: "stamina",
			Tiers: []enchantments.TierDef{
				{Tier: 0, ReservePct: 0.30},
				{Tier: 1, ReservePct: 0.60, TierUpMessage: "The band drinks deeper."},
			},
		},
		"test-edge": {
			EnchantId: "test-edge",
			Name:      "edge",
			Tiers:     []enchantments.TierDef{{Tier: 0}, {Tier: 1, TierUpMessage: "The edge sharpens."}},
		},
	})
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		999960: {ItemId: 999960, Name: "greedy band", Type: items.Ring, Subtype: items.Wearable},
		999961: {ItemId: 999961, Name: "plain yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.20},
		999962: {ItemId: 999962, Name: "keen blade", Type: items.Weapon},
	})
	return func() {
		restoreItems()
		restoreEnchants()
		restoreSpecies()
	}
}

// newEnchantCapCharacter builds a character wearing a tier-0 "test-greed" ring
// whose accumulated uses already sit past the tier-up threshold, so the only
// thing standing between it and a tier-up is the roll and the ceiling.
func newEnchantCapCharacter() *characters.Character {
	c := characters.New()
	c.StaminaMax.Base = 100
	// 25 is the rider's neutral rank (CostSkillMidRank), so the reserved
	// percentages below are exactly the authored ones.
	c.Skills["enchanting"] = 25

	ring := items.New(999960)
	ring.EnchantType = "test-greed"
	ring.EnchantTier = 0
	ring.ReservePool = "stamina"
	ring.EnchantUses = 999
	c.Equipment.Ring = ring

	c.Validate()
	return c
}

// alwaysRoll makes the tier-up roll succeed every time, so these tests measure
// the ceiling rather than the dice.
func alwaysRoll(int) int { return 0 }

// D14. Tier-up is a PASSIVE breach: it rolls in combat and doubles the reserved
// fraction at low tiers, so a character just under the ceiling can cross it
// having taken no action at all. It must skip rather than be allowed.
func TestEnchantTierUpSkipsWhenItWouldBreach(t *testing.T) {
	defer seedEnchantCapFixtures()()

	c := newEnchantCapCharacter()

	// Tier 0 holds 30 against a 66 cap. Tier 1 would hold 60, an addition of 30,
	// landing at exactly 60 and INSIDE the cap. This one must be allowed.
	if enchantTierUpWouldBreach(c, &c.Equipment.Ring) {
		t.Errorf("a tier-up landing inside the cap must be allowed")
	}

	// Add a second reserving item. Now 30 + 20 = 50 today, and the tier-up's
	// extra 30 would make 80 against the same 66 cap.
	c.Equipment.Neck = items.New(999961)
	c.Validate()
	if !enchantTierUpWouldBreach(c, &c.Equipment.Ring) {
		t.Errorf("a tier-up crossing the cap must be skipped")
	}
}

// The skip must never fire for an enchantment that reserves nothing, or every
// non-reserving enchantment in the game would silently stop advancing.
func TestEnchantTierUpAllowsNonReservingEnchantments(t *testing.T) {
	defer seedEnchantCapFixtures()()

	c := characters.New()
	c.StaminaMax.Base = 100
	blade := items.New(999962)
	blade.EnchantType = "test-edge"
	blade.EnchantTier = 0
	c.Equipment.Weapon = blade
	c.Validate()

	if enchantTierUpWouldBreach(c, &c.Equipment.Weapon) {
		t.Errorf("an enchantment that reserves nothing must never be blocked from advancing")
	}
}

// The whole point of D14, driven through the real tick rather than the
// predicate: a breaching tier-up leaves the item exactly as it was, keeps the
// uses it earned, and TELLS the wearer why. A skip with no message is
// indistinguishable from a failed roll, and the player would never learn the
// cause.
func TestEnchantTick_BreachingTierUpIsSkippedAndExplained(t *testing.T) {
	defer seedEnchantCapFixtures()()

	c := newEnchantCapCharacter()
	c.Equipment.Neck = items.New(999961) // pushes the tier-up over the ceiling
	c.Validate()

	msgs := tickChrysalisEnchantments(c, alwaysRoll)

	if c.Equipment.Ring.EnchantTier != 0 {
		t.Errorf("a breaching tier-up must not apply, tier is now %d", c.Equipment.Ring.EnchantTier)
	}
	if c.Equipment.Ring.EnchantUses != 1000 {
		t.Errorf("a skipped tier-up must keep the uses it earned, got %d want 1000", c.Equipment.Ring.EnchantUses)
	}
	if c.Equipment.Ring.Spec != nil {
		t.Errorf("a skipped tier-up must not rewrite the item spec")
	}
	if len(msgs) != 1 {
		t.Fatalf("a skipped tier-up must say why exactly once, got %d messages: %v", len(msgs), msgs)
	}
	if !strings.Contains(strings.ToLower(msgs[0]), "reserve") {
		t.Errorf("the message must name reservation as the cause, got %q", msgs[0])
	}
	if strings.ContainsAny(msgs[0], "0123456789") {
		t.Errorf("a player-facing message must carry no raw numbers, got %q", msgs[0])
	}
}

// The mirror case, so the skip is not simply blocking everything: the same
// character with room to spare DOES advance.
func TestEnchantTick_TierUpStillFiresWhenThereIsRoom(t *testing.T) {
	defer seedEnchantCapFixtures()()

	c := newEnchantCapCharacter() // ring only: 30 today, 60 after, cap 66

	msgs := tickChrysalisEnchantments(c, alwaysRoll)

	if c.Equipment.Ring.EnchantTier != 1 {
		t.Fatalf("a tier-up with room to spare must apply, tier is %d", c.Equipment.Ring.EnchantTier)
	}
	if c.Equipment.Ring.EnchantUses != 0 {
		t.Errorf("an applied tier-up resets the use counter, got %d", c.Equipment.Ring.EnchantUses)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "The band drinks deeper.") {
		t.Errorf("the tier-up message must be sent, got %v", msgs)
	}
}
