package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// attackerFixture builds a validated, unladen character with a deep stamina
// pool, so a charge is never clipped by an empty pool and the test measures the
// price rather than the pool floor.
//
// characters.New() seeds every skill at rank 1 through initAllSkills, so the
// combat skill of a fixture built this way is 1, NOT 0. Every magnitude in this
// file is quoted against the rank-1 multiplier for that reason.
func attackerFixture(t *testing.T) *characters.Character {
	t.Helper()

	c := characters.New()
	c.Stats.Strength.Base = 100
	c.Stats.Dexterity.Base = 100
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Validate()
	c.StaminaMax.Value = 1000
	c.Stamina = 1000
	return c
}

// loadAttackerToFraction puts the attacker at the given fraction of carry
// capacity. Capacity is pinned to a round 100 and overridden rather than derived
// from Strength so the ratio does not move when the stat defaults do.
//
// itemId must be unique across the whole test binary: the item spec registry is
// package-global.
func loadAttackerToFraction(t *testing.T, c *characters.Character, itemId int, fraction float64) {
	t.Helper()

	const capacity = 100.0
	load := capacity * fraction

	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: itemId,
		Name:   "test lead pig",
		Weight: load,
	})
	characters.ApplyMobOverrides(c, 0, 0, capacity)
	c.Items = append(c.Items, items.Item{ItemId: itemId})

	if got := c.GetCarriedWeight(); got < load {
		t.Fatalf("fixture carries %.1f, want %.1f; the item spec did not register", got, load)
	}
	if got := c.CarryCapacity(); got != capacity {
		t.Fatalf("fixture capacity is %.1f, want %.1f; the override did not take", got, capacity)
	}
}

// THE claim of U7 Task 7: the price scales with the swings thrown. Before this
// task the four combat wrappers charged DeductAttackStamina exactly once per
// round, so this test's four-swing character and its one-swing character paid
// the identical amount and a twelve-swing build attacked twelve times for the
// price of one.
func TestChargeAttackCostScalesWithSwings(t *testing.T) {
	one := attackerFixture(t)
	four := attackerFixture(t)

	ChargeAttackCost(one, 1)
	ChargeAttackCost(four, 4)

	spentOne := 1000 - one.Stamina
	spentFour := 1000 - four.Stamina

	if spentFour <= spentOne {
		t.Fatalf("four swings cost %d and one swing cost %d; the charge is not "+
			"scaling with swings, which is the whole point of this task",
			spentFour, spentOne)
	}
	// Per swing 1.096: one swing floors to a charge of 1, four swings (4.384)
	// floor to 4. Assert the ratio rather than the exact pair so a base retune
	// does not break the test.
	if float64(spentFour) < 3.0*float64(spentOne) {
		t.Errorf("four swings cost %d against one swing's %d; expected roughly "+
			"four times, so the per-swing multiplication is not reaching the pool",
			spentFour, spentOne)
	}
}

// Zero swings is a real state -- a round where the attacker had no weapon to
// swing, or the defender left before resolution -- and must not be charged as if
// it were one. A negative count is a caller bug and must not credit stamina back.
func TestChargeAttackCostZeroOrNegativeSwingsChargeNothing(t *testing.T) {
	for _, swings := range []int{0, -1, -12} {
		c := attackerFixture(t)
		res := ChargeAttackCost(c, swings)

		if c.Stamina != 1000 {
			t.Errorf("%d swings moved stamina to %d, want 1000 untouched", swings, c.Stamina)
		}
		if res.Charged != 0 {
			t.Errorf("%d swings reported Charged %d, want 0", swings, res.Charged)
		}
		if res.Short {
			t.Errorf("%d swings reported Short; nothing was demanded, so nothing "+
				"was short -- U8 reads this flag to strip the skill term and would "+
				"penalise a round that cost nothing", swings)
		}
	}
}

// A nil attacker must not panic. calculateCombat's callers are all non-nil today,
// but this is the pool-charging entry point for four wrappers and a panic here
// lands mid-combat-round.
func TestChargeAttackCostNilAttacker(t *testing.T) {
	if res := ChargeAttackCost(nil, 4); res.Charged != 0 || res.Short {
		t.Fatalf("nil attacker reported %+v, want the zero CostResult", res)
	}
}

// Load is priced. Attacking is registered Physical: true in the costs registry,
// so the encumbrance multiplier reaches it exactly as it reaches dodge, parry
// and block. Flip that row to Physical: false and this fails.
func TestLadenAttackerPaysMorePerSwing(t *testing.T) {
	unladen := attackerFixture(t)

	laden := attackerFixture(t)
	loadAttackerToFraction(t, laden, 99861, 0.90)

	light := attackCostPerSwing(unladen)
	heavy := attackCostPerSwing(laden)

	if heavy <= light {
		t.Fatalf("laden per-swing cost %.4f is not above unladen %.4f; the "+
			"encumbrance multiplier is not reaching the attack price", heavy, light)
	}
}

// Practice discounts the swing. costs.SkillCostMultiplier runs INVERSE to skill,
// unlike combat.SkillMultiplier in this same package, which scales damage UPWARD
// from an identical signature. A mix-up between the two compiles clean and lands
// as a large error in the wrong direction, so this asserts the DIRECTION, not
// just that the two differ.
func TestSkilledAttackerPaysLessPerSwing(t *testing.T) {
	novice := attackerFixture(t)

	veteran := attackerFixture(t)
	// Unarmed fixtures: GetCombatSkillTag returns unarmed-combat with no weapon
	// equipped, so setting weapon-combat here would leave the veteran at rank 1
	// and the test would compare a character with itself.
	veteran.Skills[string(skills.UnarmedCombat)] = 60

	green := attackCostPerSwing(novice)
	trained := attackCostPerSwing(veteran)

	if trained >= green {
		t.Fatalf("a rank-60 attacker pays %.4f per swing against a rank-1 "+
			"attacker's %.4f; the skill term is inverted or absent. Check that "+
			"the cost path reads costs.SkillCostMultiplier and NOT "+
			"combat.SkillMultiplier", trained, green)
	}
}

// The magnitude, stated rather than merely compared.
//
// Arithmetic for an unladen rank-1 attacker, at the shipped Go defaults (none of
// the U7 knobs are in config.yaml yet -- Task 12 owns that file):
//
//	base        AttackBaseStaminaCost              = 1.0
//	encumbrance carrying nothing, r = 0             = 1.0
//	skill       rank 1, linear from (0, 1.10) to (25, 1.00):
//	              1.10 + (1.00 - 1.10) x (1/25)     = 1.096
//	modifier    AttackCostModifier                  = 1.0
//
//	1.0 x 1.0 x 1.096 x 1.0 = 1.096 stamina for ONE swing.
//
// Three swings, a newcomer's typical round, is therefore about 3.29 -- against
// the flat 4 they paid per round under the old unarmed weapon cost. The
// newcomer is very slightly CHEAPER; the change bites the multi-swing veteran,
// which is the intent.
func TestUnladenRankOneAttackCostMagnitude(t *testing.T) {
	c := attackerFixture(t)

	if rank := c.GetCombatSkillLevel(); rank != 1 {
		t.Fatalf("fixture combat skill is %d, want 1; the magnitude below is "+
			"quoted against rank 1 and means nothing at another rank", rank)
	}

	const want = 1.096
	got := attackCostPerSwing(c)
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("one unladen rank-1 swing costs %.4f, want %.4f "+
			"(base 1.0 x encumbrance 1.0 x skill 1.096 x modifier 1.0)", got, want)
	}

	// Three swings, the newcomer round quoted above.
	if round := got * 3; math.Abs(round-3.288) > 0.005 {
		t.Errorf("a three-swing round costs %.4f, want about 3.29", round)
	}
}

// A Meirok-shaped veteran: carrying about 37 percent of capacity, combat skill
// 69, twelve swings a round.
//
//	encumbrance r = 0.37, below the 0.75 knee:
//	              1.0 + (1.5 - 1.0) x (0.37 / 0.75)  = 1.2467
//	skill       rank 69, linear from (25, 1.00) to (100, 0.40):
//	              1.00 + (0.40 - 1.00) x (44/75)     = 0.648
//
//	1.0 x 1.2467 x 0.648 x 1.0 = 0.8078 per swing, about 9.69 for twelve.
//
// Against a flat 6 per round before. The veteran pays MORE, and the newcomer
// above pays slightly less: the change redistributes cost onto exactly the
// builds that were getting their extra swings free.
func TestVeteranShapedAttackCostMagnitude(t *testing.T) {
	c := attackerFixture(t)
	loadAttackerToFraction(t, c, 99862, 0.37)
	c.Skills[string(skills.UnarmedCombat)] = 69

	const wantPerSwing = 0.8078
	got := attackCostPerSwing(c)
	if math.Abs(got-wantPerSwing) > 0.001 {
		t.Fatalf("a laden rank-69 swing costs %.4f, want %.4f "+
			"(base 1.0 x encumbrance 1.2467 x skill 0.648 x modifier 1.0)",
			got, wantPerSwing)
	}

	if round := got * 12; math.Abs(round-9.69) > 0.02 {
		t.Errorf("a twelve-swing round costs %.4f, want about 9.69", round)
	}
}

// The round-level behaviour, end to end: drive calculateCombat exactly as a
// wrapper does, and check that the number it reports is the number of swings
// that actually resolved and that the attacker is charged per swing off it.
//
// SwingEvents is the independent witness. calculateCombat appends one per swing
// from a different statement than the SwingsThrown increment, so if the counter
// were reset by the per-swing flag reset block, or failed to carry across the
// weapon loop, the two would disagree here. An unarmed attacker collects TWO
// fist "weapons" (main hand and offhand), so this fixture genuinely exercises
// the across-weapons aggregation rather than a single weapon's inner loop.
func TestSwingsThrownDrivesThePerSwingCharge(t *testing.T) {
	pinContestFloorOff(t)

	atk := attackerFixture(t)
	atk.Stats.Dexterity.Base = 200
	atk.Stats.Dexterity.Recalculate()
	// calculateCombat reads sourceChar.Aggro.Type unguarded; Aggro is nil on a
	// fresh character and every production caller is mid-combat. DefaultAttack,
	// not SurpriseAttack, which would crit every swing.
	atk.SetAggro(0, 0, characters.DefaultAttack)

	def := characters.New()
	def.Stats.Dexterity.Base = 100
	def.Stats.Vitality.Base = 100
	def.Stats.Dexterity.Recalculate()
	def.Stats.Vitality.Recalculate()
	def.Validate()
	def.HealthMax.Value = 100000
	def.Health = 100000
	def.StaminaMax.Value = 100000
	def.Stamina = 100000

	res := calculateCombat(atk, def, User, Mob, combatContext{
		sourceCanSee: true, targetCanSee: true,
	})

	if res.SwingsThrown != len(res.SwingEvents) {
		t.Fatalf("SwingsThrown = %d but %d swings resolved; the counter is being "+
			"reset per swing or is not carrying across the weapon loop",
			res.SwingsThrown, len(res.SwingEvents))
	}
	if res.SwingsThrown < 2 {
		t.Fatalf("fixture threw %d swings; this test needs several to distinguish "+
			"a per-swing charge from a flat per-round one", res.SwingsThrown)
	}
	if len(res.WeaponHits) < 2 {
		t.Fatalf("fixture used %d weapons; an unarmed attacker should collect two "+
			"fists, and the across-weapons aggregation is untested with one",
			len(res.WeaponHits))
	}

	perSwing := attackCostPerSwing(atk)
	before := atk.Stamina
	ChargeAttackCost(atk, res.SwingsThrown)
	spent := before - atk.Stamina

	want := perSwing * float64(res.SwingsThrown)
	// ApplyCostFloat banks the sub-integer remainder, so a single charge is the
	// floor of the amount asked for -- never more, and never a whole point less.
	if float64(spent) > want || float64(spent) <= want-1.0 {
		t.Errorf("%d swings charged %d stamina, want floor(%.3f); the round is "+
			"not being priced per swing", res.SwingsThrown, spent, want)
	}
	// The flat per-round charge this replaced was 4 stamina for an unarmed
	// attacker, independent of swing count. Prove the new number tracks the
	// swings rather than sitting at a constant.
	if math.Abs(want-4.0) < 0.5 {
		t.Errorf("a %d-swing round costs %.3f, which is indistinguishable from "+
			"the flat 4 it replaced; the fixture is not throwing enough swings "+
			"to demonstrate the change", res.SwingsThrown, want)
	}
}
