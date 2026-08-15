package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

func (c *Character) DeductActionPoints(amount int) bool {

	if c.ActionPoints < amount {
		return false
	}
	c.ActionPoints -= amount
	if c.ActionPoints < 0 {
		c.ActionPoints = 0
	}
	return true
}

// GetMovementStaminaCost calculates the stamina cost for movement based on
// terrain difficulty and encumbrance.
// terrainMultiplier: 1.0 = normal terrain, 2.0 = rough terrain, etc.
// Returns stamina cost (2-20 stamina range).
func (c *Character) GetMovementStaminaCost(terrainMultiplier float64) int {
	b := configs.GetBalanceConfig()
	baseCost := float64(b.MovementBaseStaminaCost)
	maxCost := float64(b.MovementMaxStaminaCost)

	// Apply terrain multiplier
	cost := baseCost * terrainMultiplier

	// Calculate encumbrance multiplier based on weight
	encumbranceMultiplier := 1.0
	carriedWeight := c.GetCarriedWeight()
	capacity := c.CarryCapacity()

	if carriedWeight > capacity {
		// Overencumbered: scale from 1.0 to 5.0 based on how much over capacity
		overAmount := carriedWeight - capacity
		overRatio := overAmount / capacity
		// Cap at 5x multiplier when carrying 2x capacity
		encumbranceMultiplier = 1.0 + math.Min(overRatio*4.0, 4.0)
	}

	// Apply encumbrance multiplier
	cost *= encumbranceMultiplier

	// Phase 24.2: Apply mutation movement speed modifier (Hasted, Extra Legs, etc.)
	if moveMod := mutations.GetMovementSpeedModifier(c.Mutations); moveMod != 0 {
		cost *= (1.0 - moveMod) // positive moveMod = faster = less cost
	}

	// Sneaking costs extra stamina — moving carefully is harder.
	// HiddenMoveStaminaMultiplier (default 3.0) stacks multiplicatively
	// with encumbrance: over-encumbered hidden movement is brutal (5×3 = 15× cost).
	if c.IsHidden() {
		cost *= float64(b.HiddenMoveStaminaMultiplier)
	}

	// Cap at maximum stamina cost
	if cost > maxCost {
		cost = maxCost
	}

	// Minimum 1 stamina
	if cost < 1.0 {
		cost = 1.0
	}

	return int(math.Ceil(cost))
}

// GetAttackStaminaCost calculates the stamina cost for making an attack.
// Cost is based on weapon type (or unarmed if no weapon).
func (c *Character) GetAttackStaminaCost() int {
	// Check main hand weapon
	if c.Equipment.Weapon.ItemId > 0 {
		weaponSpec := c.Equipment.Weapon.GetSpec()
		return weaponSpec.GetAttackStaminaCost()
	}

	// Check offhand weapon (dual wielding)
	if c.Equipment.Offhand.ItemId > 0 {
		offhandSpec := c.Equipment.Offhand.GetSpec()
		return offhandSpec.GetAttackStaminaCost()
	}

	// Unarmed combat costs less stamina
	return int(configs.GetBalanceConfig().UnarmedAttackStaminaCost)
}

// DeductAttackStamina deducts stamina for an attack and returns the actual cost deducted.
// If character doesn't have enough stamina, deducts what they have and returns that amount.
//
// Deprecated: use ApplyCostPartial. Its pay-what-you-can behaviour is correct
// but nothing downstream strips the skill term yet; U8 adds that.
func (c *Character) DeductAttackStamina() int {
	cost := c.GetAttackStaminaCost()
	return c.ApplyCostPartial(PoolStamina, cost).Charged
}

// DefensePool names the resource pool a defence is paid out of.
//
// The physical three cost STAMINA; quell and defy cost CONVICTION. Before U6
// there was no mapping at all and every call site simply assumed PoolStamina,
// which is why the old stamina-only cost function returning 0 for quell and
// defy would have made both defences free with no compile error and no failing
// test.
//
// Charge through the PAIR -- DefensePool for the pool, GetDefenseCostFloat for
// the amount. Either alone can charge the wrong thing; together they cannot.
//
// An unrecognised defence name maps to PoolStamina, where GetDefenseCostFloat
// also returns 0, so the pair charges nothing rather than draining an arbitrary
// pool.
func DefensePool(defenseType string) Pool {
	switch defenseType {
	case DefenseQuell, DefenseDefy:
		return PoolConviction
	default:
		return PoolStamina
	}
}

// GetDefenseCostFloat prices one defence, and is the entry point every caller
// that can spend a fractional cost should use.
//
// The three PHYSICAL defences run through costs.Calc:
//
//	DefenceBaseStaminaCost x encumbrance x inverse-skill x per-action modifier
//
// The modifiers (dodge 1.25, block 1.15, parry 1.10) are the whole reason this
// returns a float. They differ by as little as five percent, and the old
// integer formula truncated every one of them onto the same handful of whole
// numbers; charge the result through Character.ApplyCostFloat, which banks the
// remainder, and the difference survives as an average instead of vanishing.
//
// ALL FIVE defences price through the one formula, quell and defy included.
// Every action with a governing skill takes the inverse-skill discount, mental
// and social no exception: quell is governed by spellcasting and defy by
// rhetoric, so a practised caster spends less conviction turning a working aside
// than a novice does, exactly as a practised fighter spends less stamina
// blocking. Their bases are the two conviction knobs rather than the shared
// stamina one, and their per-action modifier is a neutral 1.0 -- there is no
// third conviction-defence to tune them against.
//
// What keeps encumbrance off them is the costs registry, not this function:
// ActionQuell and ActionDefy are registered Physical: false, so costs.Calc
// never applies the encumbrance multiplier to either. That row is the single
// point of truth for it, and flipping it would move the price of quell under
// load -- which is what TestQuellAndDefyAreFlatAndUnencumbered asserts against.
//
// The principled price for a mounted defence is still a FRACTION OF THE
// INCOMING ACTION's cost -- a save should scale with what it is answering --
// but that needs the attacker's cost threaded through seven call sites and a
// signature change on the defence path. Deferred on purpose; the flat base is
// tunable today.
//
// An unrecognised defence costs 0 rather than being priced as something else,
// which pairs with DefensePool mapping it to PoolStamina: together they charge
// nothing instead of draining an arbitrary pool.
func (c *Character) GetDefenseCostFloat(defenseType string) float64 {
	bal := configs.GetBalanceConfig()

	var action costs.Action
	var base, modifier float64

	switch defenseType {
	case DefenseDodge:
		action = costs.ActionDodge
		base, modifier = float64(bal.DefenceBaseStaminaCost), float64(bal.DodgeCostModifier)
	case DefenseParry:
		action = costs.ActionParry
		base, modifier = float64(bal.DefenceBaseStaminaCost), float64(bal.ParryCostModifier)
	case DefenseBlock:
		action = costs.ActionBlock
		base, modifier = float64(bal.DefenceBaseStaminaCost), float64(bal.BlockCostModifier)
	case DefenseQuell:
		action = costs.ActionQuell
		base, modifier = float64(bal.QuellBaseConvictionCost), 1.0
	case DefenseDefy:
		action = costs.ActionDefy
		base, modifier = float64(bal.DefyBaseConvictionCost), 1.0
	default:
		return 0
	}

	spec := costs.SpecFor(action)
	cost := costs.Calc(costs.Input{
		Base:      base,
		Carried:   c.GetCarriedWeight(),
		Capacity:  c.CarryCapacity(),
		Physical:  spec.Physical,
		SkillRank: c.GetSkillLevel(spec.Skill),
		HasSkill:  spec.HasSkill,
		Modifier:  modifier,
	})

	// A mounted defence must never come out FREE. This is the float sibling of
	// atLeastOneCost, and it deliberately does not floor at 1: ApplyCostFloat
	// banks the sub-integer remainder, so a mastered caster's 0.8 conviction is
	// really paid over five saves, and clamping it up to a whole point would
	// delete the mastery discount the paragraph above grants. Zero is the only
	// value that is genuinely free forever, so zero is what this catches.
	//
	// It is unreachable under any validated Balance -- validateCombat forces
	// every base positive and both multiplier curves are positive by
	// construction -- and exists so a hand-built Balance cannot silently make a
	// defence cost nothing.
	if cost <= 0 {
		return float64(atLeastOneCost(int(base)))
	}
	return cost
}

// GetDefenseCost returns what a defence costs in the pool DefensePool names for
// it, as a whole number.
//
// USE GetDefenseCostFloat. NO production caller uses this one any more: the
// melee site migrated in U7 Task 6 and ResolveChannelDefence (the ranged, spell
// and social channels) followed, which is what fixed block against a physical
// spell falling from four stamina to one. The only remaining callers are the
// defence-cost tests in this package, which use it to pin exactly how lossy it
// is. Rounding a U7 cost to an integer destroys the tuning the float version
// exists to carry -- at the shipped base and modifiers all three physical
// defences floor to 1 here, so a caller using this cannot see the difference
// between dodging, parrying and blocking.
//
// Floored, not rounded, with a floor of 1: a defence that costs nothing is not
// a defence, and rounding up would overcharge every defence in the game by up
// to a whole point.
func (c *Character) GetDefenseCost(defenseType string) int {
	cost := c.GetDefenseCostFloat(defenseType)
	if cost <= 0 {
		return 0
	}
	return atLeastOneCost(int(cost))
}

// atLeastOneCost floors a mounted defence's cost at 1, matching the clamp the
// three stamina defences have carried since Stage 7.1. A defence that costs
// nothing is not a defence.
func atLeastOneCost(cost int) int {
	if cost < 1 {
		return 1
	}
	return cost
}

// U7 Task 6 deleted GetDefenseStaminaCost. It was stamina-only, returned 0 for
// the two conviction defences, and its base x multiplier arithmetic is now the
// per-action modifier arm of costs.Calc. Use GetDefenseCostFloat.

// ApplyHealthChange applies a SIGNED health delta and returns the applied
// delta. It is not a legacy path: it survives U5b because it owns a side effect
// the primitives deliberately do not have.
//
// The eight melee call sites in internal/combat/combat.go depend on the
// CancelCombatBuffs below, which reaches CancelBuffsWithFlag -> Validate(true)
// -> a full stat recalculation. Routing them straight at ApplyHarm would drop
// the on-death combat-buff cancel for every melee kill in the game.
//
// Ordering note (verified, U5b-1): the pre-U5b implementation called
// CancelCombatBuffs BEFORE writing c.Health, then overwrote c.Health
// unconditionally. Validate does read and write c.Health -- the reservation
// clamp, the enchant-withdrawal shrink and validatePoolClamps all do -- but
// every one of those writes is guarded by `c.Health > <positive>`, so none can
// fire once health is negative, and in the old order any that did fire was
// discarded by the unconditional write that followed. After-write is therefore
// exactly equivalent for both c.Health and the return, and it lets the
// arithmetic live in the primitives.
// source is who dealt the harm, and it is REQUIRED rather than optional.
//
// U5c: ApplyHarm queues an attributed death when health crosses below 1, so a
// wrapper that swallowed the source would make every death routed through it
// anonymous. This function used to pass state.ActorRef{} unconditionally, which
// covered all eight combat damage sites -- melee, the commonest death in the
// game. Pass the OTHER participant: the character taking the damage is not the
// one who caused it.
//
// A zero ActorRef is still legitimate for genuinely sourceless harm
// (environmental damage, attrition). It must be a deliberate choice at the call
// site, not a default this function imposes.
func (c *Character) ApplyHealthChange(healthChange int, source state.ActorRef) int {
	var applied int
	if healthChange < 0 {
		applied = -c.ApplyHarm(PoolHealth, -healthChange, source)
	} else {
		applied = c.ApplyRestore(PoolHealth, healthChange)
	}

	// Any drop below 0 means dead; cancel combat-scoped buffs. Death itself is
	// processed by the per-round hooks (NewRound_DoCombat + NewRound_AutoHeal);
	// this function only applies the raw change.
	if c.Health < 0 {
		c.CancelCombatBuffs()
	}

	return applied
}

func (c *Character) Heal(hp int) int {
	startHP := c.Health

	c.Health += hp
	if c.Health > c.HealthMax.Value {
		c.Health = c.HealthMax.Value
	}

	return c.Health - startHP
}

func (c *Character) HealthPerRound() int {
	b := configs.GetBalanceConfig()
	pct := float64(b.PlayerHealthRegenPct)
	if c.IsMob {
		pct = float64(b.MobHealthRegenPct)
	}
	// StatMod reinterpreted as percentage bonus (e.g. 5 → +5%)
	pct += float64(c.StatMod(string(statmods.HealthRecovery))) / 100.0
	base := int(pct * float64(c.HealthMax.Value))
	if base < 1 {
		base = 1
	}
	// Chunk 3.3: 5× regen while sleeping.
	if c.HasBuffFlag(buffs.Sleeping) {
		if mult := float64(b.SleepRegenMultiplier); mult > 0 {
			base = int(float64(base) * mult)
		}
	}
	return base
}

func (c *Character) StaminaPerRound() int {
	b := configs.GetBalanceConfig()
	pct := float64(b.PlayerStaminaRegenPct)
	if c.IsMob {
		pct = float64(b.MobStaminaRegenPct)
	}
	pct += float64(c.StatMod(string(statmods.StaminaRecovery))) / 100.0
	base := int(pct * float64(c.StaminaMax.Value))
	if base < 1 {
		base = 1
	}
	// Apply stamina_regen_multiplier mutations
	if mult := mutations.GetStaminaRegenMultiplier(c.Mutations); mult != 0 {
		base = int(float64(base) * (1.0 + mult))
		if base < 1 {
			base = 1
		}
	}
	// Chunk 3.3: 5× regen while sleeping (composes on top of mutation modifier).
	if c.HasBuffFlag(buffs.Sleeping) {
		if mult := float64(b.SleepRegenMultiplier); mult > 0 {
			base = int(float64(base) * mult)
		}
	}
	return base
}

func (c *Character) ConvictionPerRound() int {
	b := configs.GetBalanceConfig()
	pct := float64(b.PlayerConvictionRegenPct)
	if c.IsMob {
		pct = float64(b.MobConvictionRegenPct)
	}
	pct += float64(c.StatMod(string(statmods.ConvictionRecovery))) / 100.0
	base := int(pct * float64(c.ConvictionMax.Value))
	if base < 1 {
		base = 1
	}
	// Chunk 3.3: 5× regen while sleeping.
	if c.HasBuffFlag(buffs.Sleeping) {
		if mult := float64(b.SleepRegenMultiplier); mult > 0 {
			base = int(float64(base) * mult)
		}
	}
	return base
}

// GetToxicityMax returns the maximum toxicity this character can handle.
// Formula: BaseMax + Vitality / VitalityScale
func (c *Character) GetToxicityMax() float64 {
	bal := configs.GetBalanceConfig()
	return float64(bal.ToxicityBaseMax) + float64(c.Stats.Vitality.ValueAdj)/float64(bal.ToxicityVitalityScale)
}

// AddToxicity adds (or removes, if amount is negative) toxicity, clamping to the
// valid [0, max] range. Returns true (always applies — toxicity rides at max rather
// than silently rejecting the add, so high-toxicity harm can accrue).
func (c *Character) AddToxicity(amount float64) bool {
	c.Toxicity += amount
	if max := c.GetToxicityMax(); c.Toxicity > max {
		c.Toxicity = max
	}
	if c.Toxicity < 0 {
		c.Toxicity = 0
	}
	return true
}

// ToxicitySicknessDamage returns the acute HP damage to apply this regen tick from
// dangerously high toxicity (0 below the top ≥90% band). Percentage-of-max-HP, scaled
// by how deep into / past the top band the character is — the canon "shortened life":
// sustained max toxicity poisons the body and, if ignored, can kill (the AutoHeal
// non-combat death check catches Health<1).
func (c *Character) ToxicitySicknessDamage() int {
	max := c.GetToxicityMax()
	if max <= 0 {
		return 0
	}
	ratio := c.Toxicity / max
	if ratio < 0.90 {
		return 0
	}
	bal := configs.GetBalanceConfig()
	over := (ratio - 0.90) / 0.10 // 0 at the 90% line, 1.0 at max, >1 past max
	dmg := float64(c.HealthMax.Value) * float64(bal.ToxicitySicknessDamagePct) * (1.0 + over)
	if dmg < 1 {
		dmg = 1
	}
	return int(dmg)
}

// GetToxicityPenalties returns stat multipliers based on toxicity threshold.
// Returns (regenMult, perceptionMult, dexterityMult) where 1.0 = no penalty.
func (c *Character) GetToxicityPenalties() (float64, float64, float64) {
	max := c.GetToxicityMax()
	if max <= 0 {
		return 1.0, 1.0, 1.0
	}
	ratio := c.Toxicity / max

	switch {
	case ratio >= 0.90:
		return 0.60, 0.80, 0.90 // -40% regen, -20% Per, -10% Dex
	case ratio >= 0.75:
		return 0.80, 0.90, 0.90 // -20% regen, -10% Per, -10% Dex
	case ratio >= 0.50:
		return 0.90, 0.90, 1.0 // -10% regen, -10% Per
	default:
		return 1.0, 1.0, 1.0 // no penalty
	}
}

// ToxicityBand returns the toxicity severity band:
//
//	0 = clear   (<50%)
//	1 = queasy  (>=50%)
//	2 = sick    (>=75%)
//	3 = critical (>=90%)
//
// Thresholds mirror GetToxicityPenalties exactly.
func (c *Character) ToxicityBand() int {
	max := c.GetToxicityMax()
	if max <= 0 {
		return 0
	}
	ratio := c.Toxicity / max
	switch {
	case ratio >= 0.90:
		return 3
	case ratio >= 0.75:
		return 2
	case ratio >= 0.50:
		return 1
	default:
		return 0
	}
}

// ToxicityBandName returns the descriptive tier word for the current band.
func (c *Character) ToxicityBandName() string {
	switch c.ToxicityBand() {
	case 3:
		return "critical"
	case 2:
		return "sick"
	case 1:
		return "queasy"
	default:
		return "clear"
	}
}

// Where 1000 = a full round
func (c *Character) MovementCost() int {
	modifier := 3                                    // by default they should be able to move 3 times per round.
	modifier += int(c.Stats.Dexterity.ValueAdj / 15) // Every 15 dexterity, get an extra movement
	return int(1000 / modifier)
}
