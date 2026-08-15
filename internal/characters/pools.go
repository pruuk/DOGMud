package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// Pool identifies one of the three resource pools.
//
// Deliberately a STRING, matching the vocabulary already used by
// GetPoolReservation(pool string, ...) and BuffSpec.TickPool. An int enum would
// have made this the third pool vocabulary in the package and forced a
// translation function the moment U7 makes costs reserve-aware.
//
// The pools themselves are plain int fields on Character. Only the MAX values
// are StatInfo, read via .Value.
type Pool string

const (
	PoolHealth     Pool = "health"
	PoolStamina    Pool = "stamina"
	PoolConviction Pool = "conviction"
)

// CostResult reports what a partial cost charge actually did.
//
// Returned by value rather than as a bare int so later chunks can add fields
// without re-touching the ~78 call sites U5b routes. U8 in particular needs
// Short to know it must strip the skill term from the action's roll, and U7
// will change how Short is computed when affordability becomes reserve-aware --
// that belongs inside this helper, once, not at every site.
type CostResult struct {
	Charged int  // amount actually taken from the pool
	Short   bool // the actor could not pay in full
}

// PoolValue reads the current value of a pool.
func (c *Character) PoolValue(p Pool) int {
	switch p {
	case PoolHealth:
		return c.Health
	case PoolStamina:
		return c.Stamina
	case PoolConviction:
		return c.Conviction
	}
	return 0
}

// poolMax reads a pool's ceiling.
func (c *Character) poolMax(p Pool) int {
	switch p {
	case PoolHealth:
		return c.HealthMax.Value
	case PoolStamina:
		return c.StaminaMax.Value
	case PoolConviction:
		return c.ConvictionMax.Value
	}
	return 0
}

// setPool writes a pool. Unexported so every caller goes through a primitive
// and cannot bypass the floor rules.
func (c *Character) setPool(p Pool, v int) {
	switch p {
	case PoolHealth:
		c.Health = v
	case PoolStamina:
		c.Stamina = v
	case PoolConviction:
		c.Conviction = v
	}
}

// CanAfford reports whether the character could pay amount from pool.
//
// Reads the RAW pool. GetPoolReservation is deliberately NOT consulted: no
// affordability check in the codebase consults it today, and making costs
// reserve-aware would change the cost of every action for every companion
// holder. That is U7/U8 work and a named behaviour change when it lands.
func (c *Character) CanAfford(pool Pool, amount int) bool {
	if amount <= 0 {
		return true
	}
	return c.PoolValue(pool) >= amount
}

// ApplyCost pays amount from pool IN FULL or not at all, reporting whether it
// paid.
//
// Use this where refusing is safe because the actor retains a meaningful
// alternative action: movement, standing, spellcasting, mutation special moves.
// They can still auto-attack, so declining costs them a short wait rather than
// their participation.
//
// Use ApplyCostPartial instead wherever refusal would leave the actor helpless.
// See the assignment table in the U5a plan; it is the authority, and the split
// is NOT "volitional vs involuntary" and NOT "uses a cooldown" -- both framings
// were tried and both are wrong.
//
// Floor rule 1: never drives a pool below 0. Paying a cost you exactly afford
// legitimately lands the pool on 0; the rule forbids being scraped out.
//
// A non-positive amount is free, so it succeeds and changes nothing. That stops
// a negative "cost" being used as a backdoor heal.
//
// Emits no event and performs no validation, deliberately. No direct pool
// mutation in this codebase emits; adding one here would add an emission at
// every site U5b routes.
func (c *Character) ApplyCost(pool Pool, amount int) bool {
	if amount <= 0 {
		return true
	}
	current := c.PoolValue(pool)
	if current < amount {
		return false
	}
	c.setPool(pool, current-amount)
	return true
}

// ApplyCostPartial charges up to amount from pool, taking whatever is there when
// the actor cannot pay in full, and reports what happened.
//
// Use this wherever refusal would leave the actor helpless: auto-attacks,
// defensive avoidance, grapple upkeep, and flee. Death from exhaustion was tried
// in this game and players hated it -- an actor who cannot act may as well be
// dead. Refusal also removes them from the contest entirely, and therefore from
// the reach of the 5.9/5.10 contest floors, which is the mechanical reason this
// primitive exists.
//
// The punishment for being short is losing the SKILL TERM from the action's
// roll, not losing the action. Applying that penalty is chunk U8; this helper
// only reports CostResult.Short so U8 has something to read.
//
// Never drives the pool below 0. A non-positive amount charges nothing and is
// not Short.
func (c *Character) ApplyCostPartial(pool Pool, amount int) CostResult {
	if amount <= 0 {
		return CostResult{}
	}
	current := c.PoolValue(pool)
	if current <= 0 {
		return CostResult{Charged: 0, Short: true}
	}
	charged := amount
	if charged > current {
		charged = current
	}
	c.setPool(pool, current-charged)
	return CostResult{Charged: charged, Short: charged < amount}
}

// ApplyCostFloat charges a fractional cost, banking the remainder so the average
// charged converges on the amount asked for. Not exactly: cumulative charged is
// floor(cumulative amount), so it under-charges by strictly less than 1 over any
// run of actions -- 100 charges of 2.3 total 229, not 230. The bound does not
// grow, which is the point.
//
// Every U7 cost is a float: base x encumbrance x inverse skill x per-action
// modifier. Rounding each action to an integer destroys the small factors --
// verified: dodge, parry and block all collapse to the same number for a
// low-skill character at every base this game would ship. The carry is per pool
// and per character, and is not persisted.
//
// The deduction itself is delegated to ApplyCostPartial so the floor rules and
// the Short flag stay in exactly one place.
//
// A non-finite amount is treated as free and banks nothing. NaN and infinity are
// reachable here -- a cost is a product of four config-sourced floats -- and
// letting one through poisons the pool's carry PERMANENTLY: NaN survives the
// floor, every later charge floors to NaN, int(NaN) converts to the minimum
// int64, and ApplyCostPartial reads that as non-positive and charges nothing.
// One bad value would make that pool cost-free for the rest of the session,
// with no log, no panic and no failing test. costs.EncumbranceMultiplier guards
// the same way at the other end.
//
// SHORT WHEN THE FLOORED CHARGE IS ZERO: it is false, even on an empty pool.
// Short means "the actor could not pay what this action demanded", and a floored
// charge of zero demanded nothing -- the whole amount went into the bank. U8
// reads Short to strip the skill term from the action's roll, so returning true
// here would penalise an action that cost nothing, and would then penalise the
// SAME fraction again on the later action that floors it to one or more and
// legitimately reports Short against the empty pool. Within the carry the debt
// is not forgiven, only deferred, so it is charged and reported exactly once.
// This also matches the sibling contract: ApplyCostPartial treats a non-positive
// amount as free and not Short.
//
// Once the carry hands a whole number to the pool, though, the subject changes:
// the full floored amount is always removed from the carry, even when the pool
// was too empty to cover it, and that unpaid remainder is written off rather
// than re-banked. (No contradiction with the paragraph above -- the carry defers
// a FRACTION until it is worth charging; a charge the POOL could not cover is
// simply not reclaimed.) An exhausted actor would otherwise accumulate an
// unbounded debt and be slammed with the whole backlog on the first tick their
// pool refilled. Being short is already punished by the lost skill term; it must
// not also become a loan.
func (c *Character) ApplyCostFloat(pool Pool, amount float64) CostResult {
	// NaN fails every comparison, so `amount <= 0` alone does NOT stop it.
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return CostResult{}
	}

	if c.costCarry == nil {
		c.costCarry = make(map[Pool]float64, 3)
	}

	debt := c.costCarry[pool] + amount
	whole := math.Floor(debt)
	c.costCarry[pool] = debt - whole

	if whole <= 0 {
		return CostResult{}
	}

	return c.ApplyCostPartial(pool, int(whole))
}

// applyVitalChange is the single signed pipeline behind ApplyHarm and
// ApplyRestore. Negative delta is harm, positive is restore.
//
// It is unexported on purpose. Sign inversion is this codebase's signature
// failure mode -- the resolution arc has already lost time to attack-positive
// versus defence-positive margins -- so call sites get positive-only wrappers
// and cannot get the direction wrong.
func (c *Character) applyVitalChange(pool Pool, delta int) int {
	current := c.PoolValue(pool)
	next := current + delta

	if delta > 0 {
		if max := c.poolMax(pool); next > max {
			next = max
		}
	} else if pool != PoolHealth && next < 0 {
		// Floor rule 2: stamina and conviction stop at empty.
		// Floor rule 3: health does NOT, deliberately.
		next = 0
	}

	c.setPool(pool, next)
	return next - current
}

// ApplyHarm applies amount of harm from source to pool and returns the amount
// ACTUALLY applied.
//
// Floor rule 2: stamina and conviction floor at 0.
//
// Floor rule 3: health is NOT floored. It must be allowed below zero so overkill
// magnitude survives -- U6's margin-scaled work needs it -- and because
// validatePoolClamps carries an explicit "No lower Health clamp" comment.
// Note the reason is NOT that death detection reads the negative value: every
// death gate tests < 1 or <= 0, so a floor would not break death. Do not add one
// anyway, however wrong a negative number looks in a debugger.
//
// The return is the APPLIED delta, not the requested amount; they differ when a
// floor bites. Callers keeping a result struct in sync with the pool --
// NewRound_DoCombat_unified does -- must add the returned value.
//
// source is unused in U5a and is present ON PURPOSE, so U5b routes each call site
// once and U5c can add attributed death without a second pass. Be realistic about
// its reach: the direct combat, spell and maneuver sites have an actor in hand,
// but damage-over-time, toxicity and attrition sites do NOT -- buffs.Buff has no
// applier field, so those pass the zero value and stay anonymous until the buff
// system carries one. That is not U5 work.
//
// Deliberately does not call Die, cancel buffs, validate, or emit. In particular
// this is NOT built on ApplyHealthChange, and the reason is worth stating because
// the chain is not obvious:
//
//	ApplyHealthChange, on crossing below zero
//	  -> CancelCombatBuffs
//	  -> CancelBuffsWithFlag(CancelIfCombat)
//	  -> Buffs.HasFlag(flag, true)   // the `true` EXPIRES the matching buffs;
//	                                 // despite its name this is a mutator
//	  -> Validate(true)
//
// The recalculation is NOT a death behaviour. Buffs carry stat modifiers, so once
// buffs are removed the character's stats are stale until something reconciles
// them, and Validate is what does that -- along with skill migrations, FSM
// initialisation and the pool clamps, which this path does not need but gets
// anyway.
//
// So routing the other damage sites through ApplyHealthChange would run a full
// character validation on every spell fumble and every damage-over-time tick,
// and routing combat.go's 8 sites away from it would stop melee kills cancelling
// combat buffs. Hence the split: those 8 keep the wrapper, everything else uses
// this primitive.
func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int {
	if amount <= 0 {
		return 0
	}

	applied := -c.applyVitalChange(pool, -amount)

	// U5c: lethal health harm queues an ATTRIBUTED death rather than killing
	// inline. Inline would despawn a mob synchronously (Die fires its observers
	// in-band, and Death_MobInstanceCleanup removes the instance) and pull it
	// out from under any loop damaging several targets -- the AoE loop in
	// usercommands.Throw is a live example.
	//
	// The DeathQueued guard is what makes the killing blow fire exactly once.
	// Later hits that round still land and still count toward the damage map,
	// but they neither re-queue nor re-attribute; they render as a coup de
	// grace instead. Note this is deliberately NOT the "dying" predicate
	// (Health < 1 && IsAlive()), which is health-based and is what the sweeps
	// must NOT skip on.
	if pool == PoolHealth && !c.DeathQueued && c.Health < 1 && c.IsAlive() {
		c.DeathQueued = true

		overkill := 0
		if c.Health < 0 {
			overkill = -c.Health
		}

		events.AddToQueue(events.CharacterDied{
			UserId:              c.GetUserId(),
			MobInstanceId:       c.MobInstanceId,
			KillerUserId:        source.UserId,
			KillerMobInstanceId: source.MobInstanceId,
			Overkill:            overkill,
			Trigger:             life.TriggerHealthZero,
		})
	}

	return applied
}

// ApplyRestore restores amount to pool and returns the amount ACTUALLY restored,
// which is less than requested when the pool hits its maximum.
//
// Restore is the same pipeline as harm with the sign flipped, which is why the
// two share applyVitalChange. Restoring a character whose health is negative
// works normally and is not special-cased.
//
// Takes no source: nothing downstream attributes healing today, and inventing a
// parameter nobody fills is how ApplyHarm's source would have become decoration.
func (c *Character) ApplyRestore(pool Pool, amount int) int {
	if amount <= 0 {
		return 0
	}
	return c.applyVitalChange(pool, amount)
}

// DisplayHealth returns health clamped for player-facing output.
//
// The pools deliberately store overkill -- U6 reads the magnitude of a killing
// blow, and ApplyHarm floors stamina and conviction at 0 but never health -- so
// a negative value is correct in the model and wrong on the wire.
//
// Every display surface must call this: the prompt, the status template, GMCP
// vitals, GMCP enemies, and the playtest beacon. renderVitalBar and
// targetHealthDesc already clamp internally and do not need it.
func (c *Character) DisplayHealth() int {
	if c.Health < 0 {
		return 0
	}
	return c.Health
}
