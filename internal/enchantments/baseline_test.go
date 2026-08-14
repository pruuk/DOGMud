package enchantments

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Enchanting used to reset DamageMultiplier, Damage and the three mitigations
// to the BARE TEMPLATE before applying tier effects, which silently destroyed
// affix scaling — including the scaling bought with the gold paid to enter an
// instance (affixgen.CalcLootBudget turns goldPaid into affix ranks, and
// damage_mult_phys adds +0.05 to DamageMultiplier per rank).
//
// The observed symptom on prod: enchanting a set of affixed claws dropped
// damage output by about 16%.
func TestApplyTier_PreservesAffixScaling(t *testing.T) {
	def := &EnchantmentDef{
		EnchantId: "test-edge",
		Tiers: []TierDef{
			{Tier: 0, Effects: map[string]int{"damage_multiplier_bonus": 6}},
			{Tier: 1, Effects: map[string]int{"damage_multiplier_bonus": 12}},
		},
	}

	base := items.ItemSpec{ItemId: 1, DamageMultiplier: 0.45}
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{1: &base})
	defer restore()

	// An affixed instance drop: base 0.45 plus three gold-bought ranks.
	affixed := base
	affixed.DamageMultiplier = 0.60
	item := items.Item{ItemId: 1, Spec: &affixed, Affixed: true}

	ApplyTier(&item, def, 0)

	got := item.Spec.DamageMultiplier
	want := 0.66 // 0.60 affixed + 0.06 tier bonus
	if !approxEq(got, want) {
		t.Fatalf("DamageMultiplier = %.4f, want %.4f — affix scaling was wiped", got, want)
	}
}

// Re-applying a tier must not stack the bonus on top of itself. That is what
// the original reset existed to prevent, and the baseline has to preserve that
// property while no longer destroying affix scaling.
func TestApplyTier_DoesNotStackAcrossTiers(t *testing.T) {
	def := &EnchantmentDef{
		EnchantId: "test-edge",
		Tiers: []TierDef{
			{Tier: 0, Effects: map[string]int{"damage_multiplier_bonus": 6}},
			{Tier: 1, Effects: map[string]int{"damage_multiplier_bonus": 12}},
		},
	}

	base := items.ItemSpec{ItemId: 1, DamageMultiplier: 0.45}
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{1: &base})
	defer restore()

	affixed := base
	affixed.DamageMultiplier = 0.60
	item := items.Item{ItemId: 1, Spec: &affixed, Affixed: true}

	ApplyTier(&item, def, 0) // 0.60 + 0.06
	ApplyTier(&item, def, 1) // must be 0.60 + 0.12, NOT 0.66 + 0.12
	ApplyTier(&item, def, 1) // re-applying the same tier must be idempotent

	got := item.Spec.DamageMultiplier
	want := 0.72
	if !approxEq(got, want) {
		t.Fatalf("DamageMultiplier = %.4f, want %.4f — tiers stacked", got, want)
	}
}

// An item with no affix scaling behaves exactly as before.
func TestApplyTier_PlainItemUnchangedInBehaviour(t *testing.T) {
	def := &EnchantmentDef{
		EnchantId: "test-edge",
		Tiers:     []TierDef{{Tier: 0, Effects: map[string]int{"damage_multiplier_bonus": 6}}},
	}

	base := items.ItemSpec{ItemId: 1, DamageMultiplier: 0.45}
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{1: &base})
	defer restore()

	item := items.Item{ItemId: 1}

	ApplyTier(&item, def, 0)

	if got, want := item.Spec.DamageMultiplier, 0.51; !approxEq(got, want) {
		t.Fatalf("DamageMultiplier = %.4f, want %.4f", got, want)
	}
}

func approxEq(a, b float64) bool {
	d := a - b
	return d < 0.0001 && d > -0.0001
}

// Stripping an enchantment must return the item to its PRE-ENCHANT state, not
// to the bare template. Nil-ing the override spec would discard affix and
// gold-bought scaling, which is the same loss ApplyTier used to inflict.
func TestStripEnchantment_RestoresAffixScalingNotTemplate(t *testing.T) {
	def := &EnchantmentDef{
		EnchantId: "test-edge",
		Tiers:     []TierDef{{Tier: 0, Effects: map[string]int{"damage_multiplier_bonus": 6}}},
	}

	base := items.ItemSpec{ItemId: 1, DamageMultiplier: 0.45}
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{1: &base})
	defer restore()

	affixed := base
	affixed.DamageMultiplier = 0.60
	item := items.Item{ItemId: 1, Spec: &affixed, Affixed: true}

	ApplyTier(&item, def, 0)
	StripEnchantment(&item)

	if item.Spec == nil {
		t.Fatal("strip dropped the override spec, reverting to the bare template and losing affix scaling")
	}
	if got, want := item.Spec.DamageMultiplier, 0.60; !approxEq(got, want) {
		t.Fatalf("DamageMultiplier = %.4f, want %.4f (the pre-enchant value)", got, want)
	}
	if item.EnchantBaseline != nil {
		t.Error("baseline should be cleared once consumed by the strip")
	}
}
