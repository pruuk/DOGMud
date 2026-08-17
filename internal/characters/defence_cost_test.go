package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Each row pins the uncomposed request that must feed QuoteActionCost. The
// whole/carry literals then pin the final quote independently at neutral load
// and neutral skill cost. This catches feeding GetDefenseCostFloat's composed
// result back as Base, which would apply load, skill, and modifier twice.
func TestQuoteDefenseCost_UsesRawRequestMappingExactlyOnce(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.DefenceBaseStaminaCost = 10
	cfg.Balance.DodgeCostModifier = 1.25
	cfg.Balance.ParryCostModifier = 1.10
	cfg.Balance.BlockCostModifier = 1.15
	cfg.Balance.QuellBaseConvictionCost = 20
	cfg.Balance.DefyBaseConvictionCost = 30
	cfg.Balance.CostSkillMultAtZero = 1
	cfg.Balance.CostSkillMultAtMid = 1
	cfg.Balance.CostSkillMultAtCap = 1
	cfg.Balance.CostSkillMidRank = 25
	cfg.Balance.CostSkillCapRank = 100
	cfg.Balance.CostTotalMultiplierMax = 100
	configs.SetConfigForTest(t, cfg)

	c := New()
	c.Stamina = 100
	c.Conviction = 100

	tests := []struct {
		defence  string
		action   costs.Action
		pool     Pool
		base     float64
		modifier float64
		whole    int
		carry    float64
		price    float64
	}{
		{DefenseDodge, costs.ActionDodge, PoolStamina, 10, 1.25, 12, 0.5, 12.5},
		{DefenseParry, costs.ActionParry, PoolStamina, 10, 1.10, 11, 0, 11},
		{DefenseBlock, costs.ActionBlock, PoolStamina, 10, 1.15, 11, 0.5, 11.5},
		{DefenseQuell, costs.ActionQuell, PoolConviction, 20, 1.0, 20, 0, 20},
		{DefenseDefy, costs.ActionDefy, PoolConviction, 30, 1.0, 30, 0, 30},
	}

	for _, tc := range tests {
		t.Run(tc.defence, func(t *testing.T) {
			req, ok := defenseCostRequest(tc.defence)
			if !ok {
				t.Fatal("raw request mapping reported unknown defence")
			}
			if req.Action != tc.action || req.Pool != tc.pool || req.Base != tc.base ||
				req.Modifier != tc.modifier || req.Units != 1 {
				t.Fatalf("raw request = %+v, want action=%q pool=%q base=%.2f modifier=%.2f units=1",
					req, tc.action, tc.pool, tc.base, tc.modifier)
			}

			quote, ok := c.QuoteDefenseCost(tc.defence)
			if !ok || quote.state == nil {
				t.Fatal("QuoteDefenseCost did not return a valid mapped quote")
			}
			if quote.state.pool != tc.pool || quote.state.wholeDue != tc.whole ||
				math.Abs(quote.state.nextCarry-tc.carry) > 1e-9 || !quote.Affordable() {
				t.Fatalf("quote state pool=%q whole=%d carry=%.4f affordable=%v; want pool=%q whole=%d carry=%.4f affordable=true",
					quote.state.pool, quote.state.wholeDue, quote.state.nextCarry,
					quote.Affordable(), tc.pool, tc.whole, tc.carry)
			}
			if got := c.GetDefenseCostFloat(tc.defence); math.Abs(got-tc.price) > 1e-9 {
				t.Fatalf("compatibility price = %.4f, want raw request composed once to %.4f", got, tc.price)
			}
		})
	}

	if _, ok := defenseCostRequest("not-a-defence"); ok {
		t.Fatal("unknown defence unexpectedly has a raw request")
	}
	if _, ok := c.QuoteDefenseCost("not-a-defence"); ok {
		t.Fatal("unknown defence unexpectedly produced a quote")
	}
}

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

// The same ordering at a REALISTIC load, which is the case the unladen fixture
// above and the at-capacity fixture below both miss.
//
// At capacity the composed multiplier exceeds CostTotalMultiplierMax and all
// three defences converge on the same clamped number, so a reader could
// reasonably conclude the clamp eats the tuning in ordinary play. It does not: a
// realistically equipped character sits near 40% of capacity, where the costs
// are about dodge 1.735, block 1.597, parry 1.527 -- ordered, distinct, and
// nowhere near the 6.0 ceiling. This row is here to document that the clamp is
// NOT binding at normal loads. The convergence at capacity is accepted, not a
// defect.
func TestDefenceCostOrderingSurvivesARealisticLoad(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()
	loadToFraction(t, c, 99814, 0.40)

	dodge := c.GetDefenseCostFloat(DefenseDodge)
	parry := c.GetDefenseCostFloat(DefenseParry)
	block := c.GetDefenseCostFloat(DefenseBlock)

	if !(parry < block && block < dodge) {
		t.Errorf("at 40%% of capacity want parry < block < dodge, got "+
			"parry=%.3f block=%.3f dodge=%.3f", parry, block, dodge)
	}

	// The clamp caps the composed MULTIPLIER, so the price it would produce is
	// base x max. Staying under it is what "not binding" means here.
	bal := configs.GetBalanceConfig()
	ceiling := float64(bal.DefenceBaseStaminaCost) * float64(bal.CostTotalMultiplierMax)
	if dodge >= ceiling {
		t.Errorf("dodge at 40%% of capacity is %.3f, at or above the clamped "+
			"ceiling %.3f; the clamp is binding at a load it should not reach",
			dodge, ceiling)
	}
}

// Quell and defy are mental and social: they cost conviction and must never
// take the encumbrance multiplier. "Flat" here means flat UNDER LOAD, not flat
// full stop -- both take the inverse-skill discount like every other action with
// a governing skill, which is what TestSkilledDefenderPaysLessToQuellAndDefy
// covers.
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
	// two must not move at all. The Physical: false rows in the costs registry
	// are what guarantee it, and since both defences now price through
	// costs.Calc those rows are load-bearing: flipping either to Physical: true
	// multiplies in an encumbrance of CostEncumbranceMax here and fails this
	// assertion. Before quell and defy were routed through Calc this test would
	// NOT have noticed the flip -- the early return made the rows decorative.
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

// The exact price of quell and defy under full load, to the last decimal.
//
// This is the assertion that PINS the registry rows rather than merely noticing
// a change. TestQuellAndDefyAreFlatAndUnencumbered compares laden against
// unladen; this one states what the number must BE -- base x inverse-skill and
// nothing else -- at the one load where a stray encumbrance term would be
// largest. Flip ActionQuell to Physical: true in internal/costs/action.go and
// this fails immediately, with the encumbrance multiplier visible in the
// reported number.
func TestQuellAndDefyPriceThroughTheirRegistryRows(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()
	loadToCapacity(t, c, 99813)

	bal := configs.GetBalanceConfig()

	cases := []struct {
		def   string
		base  float64
		skill skills.SkillTag
	}{
		{DefenseQuell, float64(bal.QuellBaseConvictionCost), skills.Spellcasting},
		{DefenseDefy, float64(bal.DefyBaseConvictionCost), skills.Rhetoric},
	}

	for _, tc := range cases {
		want := tc.base * costs.SkillCostMultiplier(c.GetSkillLevel(tc.skill))
		got := c.GetDefenseCostFloat(tc.def)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s at capacity costs %.4f, want %.4f (base x inverse-skill "+
				"with NO encumbrance term); a mismatch this size means the %s "+
				"registry row is no longer Physical: false",
				tc.def, got, want, tc.def)
		}
	}
}

// A practised caster spends less conviction turning a working aside than a
// novice does, and a practised orator less refusing an insult. Every action with
// a governing skill takes the inverse-skill discount; quell and defy are not
// exceptions to that rule just because they are not physical.
func TestSkilledDefenderPaysLessToQuellAndDefy(t *testing.T) {
	bal := configs.GetBalanceConfig()
	capRank := int(bal.CostSkillCapRank)

	for _, tc := range []struct {
		def   string
		skill skills.SkillTag
	}{
		{DefenseQuell, skills.Spellcasting},
		{DefenseDefy, skills.Rhetoric},
	} {
		novice := New()
		novice.Validate()

		master := New()
		master.Validate()
		master.SetSkill(string(tc.skill), capRank)

		cheap := master.GetDefenseCostFloat(tc.def)
		dear := novice.GetDefenseCostFloat(tc.def)

		if cheap >= dear {
			t.Errorf("%s costs %.4f at rank %d and %.4f at rank %d; the "+
				"inverse-skill discount is not reaching the non-physical defences",
				tc.def, cheap, capRank, dear, novice.GetSkillLevel(tc.skill))
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

// loadToFraction puts the character at the given fraction of carry capacity.
// Capacity is pinned to a round 100 and overridden rather than derived from
// Strength so the ratio does not move when the stat defaults do.
//
// itemId must be unique per test: the item spec registry is package-global and
// shared across the whole test binary.
func loadToFraction(t *testing.T, c *Character, itemId int, fraction float64) {
	t.Helper()

	const capacity = 100.0
	load := capacity * fraction

	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: itemId,
		Name:   "test lead pig",
		Weight: load,
	})
	ApplyMobOverrides(c, 0, 0, capacity)
	c.Items = append(c.Items, items.Item{ItemId: itemId})

	if got := c.GetCarriedWeight(); got < load {
		t.Fatalf("fixture carries %.1f, want %.1f; the item spec did not register",
			got, load)
	}
	if got := c.CarryCapacity(); got != capacity {
		t.Fatalf("fixture capacity is %.1f, want %.1f; the override did not take",
			got, capacity)
	}
}
