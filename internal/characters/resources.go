package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
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

// GetDefenseStaminaCost returns stamina cost for a defense type (Stage 7.1)
func (c *Character) GetDefenseStaminaCost(defenseType string) int {
	bal := configs.GetBalanceConfig()

	baseCost := 0
	multiplier := 1.0

	switch defenseType {
	case DefenseDodge:
		baseCost = int(bal.DodgeBaseStaminaCost)
		multiplier = float64(bal.DodgeMultiplier)
	case DefenseParry:
		baseCost = int(bal.ParryBaseStaminaCost)
		multiplier = float64(bal.ParryMultiplier)
	case DefenseBlock:
		baseCost = int(bal.BlockBaseStaminaCost)
		multiplier = float64(bal.BlockMultiplier)
	default:
		return 0
	}

	cost := int(float64(baseCost) * multiplier)
	if cost < 1 {
		cost = 1 // minimum 1 stamina
	}
	return cost
}

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
