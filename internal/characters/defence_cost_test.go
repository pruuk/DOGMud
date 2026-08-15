package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Dodge is deliberately the most expensive defence: moving the whole body is
// more tiring than interposing a weapon or a shield.
//
// This is the assertion the integer entry point cannot make. All three costs
// live inside a quarter of a stamina point of each other, so as whole numbers
// they are the same number; the ordering only exists because the cost is a
// float and the charge banks its remainder.
func TestDefenceCostOrderingDodgeIsDearest(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()

	dodge := c.GetDefenseCostFloat(DefenseDodge)
	parry := c.GetDefenseCostFloat(DefenseParry)
	block := c.GetDefenseCostFloat(DefenseBlock)

	if !(parry < block && block < dodge) {
		t.Fatalf("want parry < block < dodge, got parry=%.3f block=%.3f dodge=%.3f",
			parry, block, dodge)
	}
}

// Quell and defy are mental and social: they cost conviction and must never
// take the encumbrance multiplier.
func TestQuellAndDefyAreFlatAndUnencumbered(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()

	q := c.GetDefenseCostFloat(DefenseQuell)
	d := c.GetDefenseCostFloat(DefenseDefy)

	if q <= 0 || d <= 0 {
		t.Fatalf("quell and defy must cost something, got %.3f and %.3f", q, d)
	}

	// Load the character to capacity. The physical three all get dearer; these
	// two must not move at all. Registered Physical: false in the costs registry
	// is what guarantees it, and this is the assertion that notices if that row
	// is ever flipped.
	loadToCapacity(t, c, 99811)

	if got := c.GetDefenseCostFloat(DefenseQuell); got != q {
		t.Errorf("quell cost moved under load: %.3f -> %.3f; quell is mental and "+
			"must not take the encumbrance multiplier", q, got)
	}
	if got := c.GetDefenseCostFloat(DefenseDefy); got != d {
		t.Errorf("defy cost moved under load: %.3f -> %.3f; defy is social and "+
			"must not take the encumbrance multiplier", d, got)
	}
}

// A laden defender pays more to defend. This is the whole point of routing the
// physical three through costs.Calc: before U7 a defence cost the same whether
// the defender was empty-handed or hauling their own weight in ore.
//
// Weight is set the way internal/behaviortree already does it -- register a
// heavy item spec so GetCarriedWeight resolves to a real number, and override
// carry capacity so the ratio does not depend on Strength defaults. No
// production code exists for testability here.
func TestLadenDefenderPaysMoreToDefend(t *testing.T) {
	unladen := New()
	unladen.Stats.Strength.Base = 100
	unladen.Stats.Strength.Recalculate()
	unladen.Validate()

	laden := New()
	laden.Stats.Strength.Base = 100
	laden.Stats.Strength.Recalculate()
	laden.Validate()
	loadToCapacity(t, laden, 99812)

	for _, def := range []string{DefenseDodge, DefenseParry, DefenseBlock} {
		light := unladen.GetDefenseCostFloat(def)
		heavy := laden.GetDefenseCostFloat(def)
		if heavy <= light {
			t.Errorf("%s: laden cost %.3f is not above unladen cost %.3f; the "+
				"encumbrance multiplier is not reaching the defence price",
				def, heavy, light)
		}
	}
}

// An unrecognised defence costs nothing rather than charging an arbitrary pool.
func TestUnknownDefenceCostsNothing(t *testing.T) {
	c := New()
	c.Validate()
	if got := c.GetDefenseCostFloat("not-a-defence"); got != 0 {
		t.Fatalf("unknown defence priced at %.3f, want 0", got)
	}
}

// loadToCapacity puts the character exactly at carry capacity, which is where
// the encumbrance curve tops out at CostEncumbranceMax. Note that the composed
// product then exceeds CostTotalMultiplierMax and all three physical defences
// clamp to the same number, so a laden fixture can prove that load is priced
// but NOT that the per-defence ordering survives -- that is what the unladen
// ordering test is for.
//
// itemId must be unique per test: the item spec registry is package-global and
// shared across the whole test binary.
func loadToCapacity(t *testing.T, c *Character, itemId int) {
	t.Helper()

	const load = 40.0
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: itemId,
		Name:   "test lead pig",
		Weight: load,
	})
	ApplyMobOverrides(c, 0, 0, load)
	c.Items = append(c.Items, items.Item{ItemId: itemId})

	if got := c.GetCarriedWeight(); got < load {
		t.Fatalf("fixture carries %.1f, want %.1f; the item spec did not register",
			got, load)
	}
}
