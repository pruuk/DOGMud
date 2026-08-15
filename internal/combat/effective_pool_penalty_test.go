package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Regression cover for the U7 Task 11 EffectivePoolMax sweep and for the floor
// value that makes it correct.
//
// Every site below reads a percentage-OF-MAX ratio for an actor whose CURRENT
// pool RecalculateStats has already clamped to max - reservation. Reading the
// RAW max there compares a clamped numerator against an unclamped denominator,
// which parks a reserved character permanently partway down the depletion
// curve: they can never reach ratio 1.0, however rested they are. Reverting any
// one of these one-line edits used to leave the whole suite green, which is why
// these tests exist.
//
// Each test pits a RESERVED character at their full REACHABLE pool against an
// identical character who is genuinely that depleted. Under the fix the first is
// unpenalised and the second is not. Under a revert to the raw max the two are
// indistinguishable, because the raw-max read is exactly the claim that they are
// the same character.

// reserveSeedBase keeps this file's synthetic item ids clear of every other test
// in the package; the item spec registry is package-global.
const reserveSeedBase = 999870

// reservedAndDepleted builds two characters with an identical 100-point pool.
//
// The first reserves `pct` of it and is at their FULL reachable pool. The second
// reserves nothing and is genuinely down to that same absolute value. Their
// current pool values are equal by construction, so any behavioural difference
// between them comes only from the denominator.
func reservedAndDepleted(t *testing.T, pool characters.Pool, pct float64) (reserved, depleted *characters.Character) {
	t.Helper()

	spec := &items.ItemSpec{
		ItemId:  reserveSeedBase + int(pct*100),
		Name:    "leeching collar",
		Type:    items.Neck,
		Subtype: items.Wearable,
	}
	switch pool {
	case characters.PoolStamina:
		spec.ReserveStaminaPct = pct
	case characters.PoolHealth:
		spec.ReserveHealthPct = pct
	case characters.PoolConviction:
		spec.ReserveConvictionPct = pct
	}
	items.RegisterTestItemSpec(spec)

	const rawMax = 100
	reachable := rawMax - int(rawMax*pct)

	build := func(equip bool) *characters.Character {
		c := characters.New()
		c.Stats.Strength.Base = 100
		c.Stats.Dexterity.Base = 100
		c.Stats.Vitality.Base = 100
		c.Stats.Strength.Recalculate()
		c.Stats.Dexterity.Recalculate()
		c.Stats.Vitality.Recalculate()
		if equip {
			c.Equipment.Neck = items.New(spec.ItemId)
		}
		c.Validate()
		setCombatPositionParallel(c, position.Standing)

		// Set the pools AFTER Validate: Validate derives the maxima from .Base
		// and would overwrite these. Every pool is pinned so no unrelated
		// depletion penalty leaks into the assertion.
		c.HealthMax.Value = rawMax
		c.StaminaMax.Value = rawMax
		c.ConvictionMax.Value = rawMax
		c.Health, c.Stamina, c.Conviction = rawMax, rawMax, rawMax
		switch pool {
		case characters.PoolStamina:
			c.Stamina = reachable
		case characters.PoolHealth:
			c.Health = reachable
		case characters.PoolConviction:
			c.Conviction = reachable
		}
		return c
	}

	reserved = build(true)
	depleted = build(false)

	if got := reserved.EffectivePoolMax(pool); got != reachable {
		t.Fatalf("fixture: reserved EffectivePoolMax(%s) = %d, want %d; the item spec did not register", pool, got, reachable)
	}
	if got := depleted.EffectivePoolMax(pool); got != rawMax {
		t.Fatalf("fixture: unreserved EffectivePoolMax(%s) = %d, want %d", pool, got, rawMax)
	}
	return reserved, depleted
}

// THE floor. EffectivePoolMax floors at 1, not 0, and the difference is a
// balance inversion rather than a rounding detail.
//
// ResourceMultiplier reads a non-positive max as "no penalty" and returns 1.0.
// So under a floor of 0 a character who reserves their WHOLE pool, which stacked
// Chrysalis enchantments reach (a two-handed item doubles its reserve share),
// got FULL swing count, FULL hit chance and FULL melee damage off a permanently
// empty pool. Before U7 that same character took the maximum penalty.
func TestTotalReservationTakesTheMaximumDepletionPenalty(t *testing.T) {
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId:            reserveSeedBase + 1,
		Name:              "famished collar",
		Type:              items.Neck,
		Subtype:           items.Wearable,
		ReserveStaminaPct: 0.60,
	})
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId:            reserveSeedBase + 2,
		Name:              "famished belt",
		Type:              items.Belt,
		Subtype:           items.Wearable,
		ReserveStaminaPct: 0.60,
	})

	c := characters.New()
	c.Equipment.Neck = items.New(reserveSeedBase + 1)
	c.Equipment.Belt = items.New(reserveSeedBase + 2)
	c.Validate()
	c.StaminaMax.Value = 100
	c.Stamina = 0 // 120 reserved against 100: the pool can never hold anything

	effMax := c.EffectivePoolMax(characters.PoolStamina)
	if effMax != 1 {
		t.Fatalf("EffectivePoolMax(stamina) = %d under total reservation, want 1. "+
			"A floor of 0 makes every consumer below return its no-penalty answer", effMax)
	}

	penaltyMax := float64(configs.GetBalanceConfig().StaminaPenaltyMax)
	if penaltyMax <= 0 {
		t.Fatalf("StaminaPenaltyMax is %v in this test env; the assertion below would be vacuous", penaltyMax)
	}

	got := ResourceMultiplier(c.Stamina, effMax, penaltyMax)
	want := 1.0 - penaltyMax
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("ResourceMultiplier on a totally reserved pool = %.4f, want %.4f "+
			"(the MAXIMUM penalty). Getting 1.0 here means the max floored to 0 and "+
			"ResourceMultiplier bailed to its no-penalty branch, handing a permanently "+
			"empty pool full combat effectiveness", got, want)
	}
	if got >= 1.0 {
		t.Errorf("ResourceMultiplier = %.4f: a permanently empty pool is being treated as UNPENALISED", got)
	}
}

// combat_helpers.go calcSwingCount. Swings are the largest single lever in the
// round, so a reserved attacker losing them at a full pool is a real nerf that
// no amount of resting can undo.
func TestSwingCountUsesTheReachableStaminaPool(t *testing.T) {
	reserved, depleted := reservedAndDepleted(t, characters.PoolStamina, 0.90)

	// weaponSpeed is passed straight in rather than derived from an item, so the
	// base swing count sits where the stamina multiplier changes the ROUNDED
	// result. calcSwingCount returns an int and caps at 4, so a base near 4.3
	// pins the rested attacker on the cap while the ~0.77 depletion multiplier
	// drops the exhausted one clear of it. A base of 2.1 (weaponSpeed 2.0) does
	// NOT work: 2.1 and 1.62 both round to 2 and the test passes on a revert.
	const weaponSpeed = 6.0

	full := calcSwingCount(reserved, weaponSpeed, 0, false)
	worn := calcSwingCount(depleted, weaponSpeed, 0, false)

	if full <= worn {
		t.Errorf("a fully rested 90%%-reserved attacker throws %d swings and a genuinely "+
			"exhausted one throws %d. They must differ: the reserved attacker is at their "+
			"REACHABLE maximum. Equal counts mean calcSwingCount is dividing by "+
			"StaminaMax.Value instead of EffectivePoolMax", full, worn)
	}
}

// combat_helpers.go calcAttackScore. A float, so this pins the exact value
// rather than an integer edge.
func TestAttackScoreUsesTheReachableStaminaPool(t *testing.T) {
	reserved, depleted := reservedAndDepleted(t, characters.PoolStamina, 0.90)

	ctx := combatContext{sourceCanSee: true, targetCanSee: true}
	target := characters.New()
	target.Validate()
	setCombatPositionParallel(target, position.Standing)

	full := calcAttackScore(reserved, target, 0, ctx)
	worn := calcAttackScore(depleted, target, 0, ctx)

	if full <= worn {
		t.Errorf("attack score for a fully rested 90%%-reserved attacker is %.3f against "+
			"%.3f for a genuinely exhausted one; they must differ. Equal scores mean "+
			"calcAttackScore is dividing by StaminaMax.Value instead of EffectivePoolMax", full, worn)
	}

	// And the rested reserved attacker must be at NO stamina penalty at all:
	// their pool is full, for them.
	unreserved := characters.New()
	unreserved.Stats.Strength.Base = 100
	unreserved.Stats.Dexterity.Base = 100
	unreserved.Stats.Vitality.Base = 100
	unreserved.Stats.Strength.Recalculate()
	unreserved.Stats.Dexterity.Recalculate()
	unreserved.Stats.Vitality.Recalculate()
	unreserved.Validate()
	setCombatPositionParallel(unreserved, position.Standing)
	unreserved.StaminaMax.Value = 100
	unreserved.Stamina = 100

	if clean := calcAttackScore(unreserved, target, 0, ctx); math.Abs(full-clean) > 0.0001 {
		t.Errorf("attack score at a FULL reachable pool is %.4f but %.4f for an "+
			"unreserved character at a full raw pool; a reserved character must reach "+
			"multiplier 1.0 when rested", full, clean)
	}
}

// combat_helpers.go buildDamageParams. Melee damage scales off the HEALTH pool,
// so this covers the third of the three mechanical sites and the health pool
// rather than stamina.
func TestMeleeDamageUsesTheReachableHealthPool(t *testing.T) {
	reserved, depleted := reservedAndDepleted(t, characters.PoolHealth, 0.90)

	target := characters.New()
	target.Validate()
	setCombatPositionParallel(target, position.Standing)

	fist := items.Item{ItemId: 0}
	fullWs := buildWeaponSetup(reserved, target, fist, 0, 1)
	wornWs := buildWeaponSetup(depleted, target, fist, 0, 1)

	full := buildDamageParams(reserved, target, fullWs, 0, User).dmgMean
	worn := buildDamageParams(depleted, target, wornWs, 0, User).dmgMean

	if full <= worn {
		t.Errorf("melee damage for a fully rested 90%%-reserved attacker is %.3f against "+
			"%.3f for a genuinely wounded one; they must differ. Equal means "+
			"buildDamageParams is dividing by HealthMax.Value instead of EffectivePoolMax", full, worn)
	}
}

// ai.go mob target scoring. Mobs pick their special moves off the target's
// health PERCENTAGE, and the target is routinely a player. Two shipped items
// alone (The Blackrazor 40183 at reserve_health_pct 0.25, worn as a weapon, and
// the Seething Prism 40187 at 0.15, worn on the neck) reserve 40% of health, so
// against the raw max that player reads as 60% health at a completely full pool
// and every mob in the game treats them as wounded prey for their whole career.
//
// ScoreMaul is the cheapest of the six target-side scorers to drive: no shield,
// no position and no aggro preconditions, and its only health term is a
// finisher bonus below 50%.
func TestMobScorersReadTheTargetsReachableHealthPool(t *testing.T) {
	reserved, _ := reservedAndDepleted(t, characters.PoolHealth, 0.90)

	mob := &mobs.Mob{}
	mob.Character = *characters.New()

	healthy := characters.New()
	healthy.Validate()
	healthy.HealthMax.Value = 100
	healthy.Health = 100

	full := ScoreMaul(mob, reserved)
	clean := ScoreMaul(mob, healthy)

	if full != clean {
		t.Errorf("ScoreMaul scores a 90%%-reserved target at FULL health %d against %d "+
			"for an unreserved target at full health. A reserved player at a full pool "+
			"must not read as wounded prey; the scorer is dividing by HealthMax.Value "+
			"instead of EffectivePoolMax", full, clean)
	}

	// And a genuinely wounded target must still attract the finisher bonus, or
	// the assertion above would pass on a scorer that ignores health entirely.
	wounded := characters.New()
	wounded.Validate()
	wounded.HealthMax.Value = 100
	wounded.Health = 10
	if hurt := ScoreMaul(mob, wounded); hurt <= clean {
		t.Errorf("ScoreMaul scores a genuinely wounded target %d against %d for a "+
			"healthy one; the finisher bonus is not firing and this test proves nothing", hurt, clean)
	}
}
