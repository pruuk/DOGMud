package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Chunk 5.11e — crit floors.
//
// Once 5.11d/g made crit derive from the opposed-roll margin, a badly
// outclassed attacker's crit rate went to literally zero: the margin can never
// reach two sigma, so no amount of luck produces a telling blow. The floors are
// the same last-resort idea as 5.9's hit floors, applied to crit.
//
// Denominated in HITS, not swings, and this matters. A badly outclassed
// attacker hits roughly 15% of the time, so "1% of swings" would demand about
// 6.7% of their hits be crits — a HIGHER per-hit rate than an even match gets
// at 2.3%, which is incoherent. 1% of hits composes with the 5.9 hit floor as
// two independent last resorts, each sized to the failure it protects.
//
// ORDERING IS LOAD-BEARING. See applyCritFloors.

// AttackCritFloor is the floor probability that a landed hit is a critical hit.
func AttackCritFloor() float64 {
	return float64(configs.GetBalanceConfig().MinAttackCritChance)
}

// DefenseCritFloor is the floor probability that a successful defence is a
// defensive critical (full negation, riposte, and so on).
func DefenseCritFloor() float64 {
	return float64(configs.GetBalanceConfig().MinDefenseCritChance)
}

// ApplyCritFloor returns isCrit, promoted to true with probability `floor` when
// it was false. It only ever promotes, never demotes, so a real crit is
// untouched and a floor of 0 disables the mechanic entirely.
//
// CALL ONLY ONCE THE HIT OUTCOME IS FINAL. In every channel a crit implies a
// hit, so promoting before the hit is settled turns this into an undeclared
// second hit floor stacked on MinAttackHitChance.
func ApplyCritFloor(isCrit bool, floor float64) bool {
	if isCrit || floor <= 0 {
		return isCrit
	}
	// util.Rand(100) yields 0..99, so `< floor*100` gives floor as a
	// probability, matching how the 5.9 hit floors are rolled.
	return util.Rand(100) < int(floor*100)
}

// AttackContestCrit is ContestCrit plus the attack-side floor, for the spell
// and conviction channels. Every call site invokes it only after `success` has
// already been decided and the miss branch has returned, which is what makes
// the floor safe there.
func AttackContestCrit(margin float64, roll dice.RollResult) bool {
	return ApplyCritFloor(ContestCrit(margin, roll), AttackCritFloor())
}

// DefenseContestCrit is the defensive mirror, used where a defender fully
// negates an incoming spell or taunt. Pass a defence-signed margin
// (defRoll.Margin), never the attack-positive one.
func DefenseContestCrit(margin float64, roll dice.RollResult) bool {
	return ApplyCritFloor(ContestCrit(margin, roll), DefenseCritFloor())
}

// applyCritFloors applies both floors to a resolved melee swing.
//
// It runs at the very END of resolveDefenseOutcome, after every branch has
// settled res.hit, and it is the only correct place for it. The melee resolver
// treats an attack crit as forcing a hit — step 2 returns res.hit = true on
// attackCrit — so a floor evaluated any earlier would silently become a hit
// floor and leak straight through MinDefenseChance.
//
// Note it deliberately does NOT floor the margin. A 5.9 floor save already
// carries the +-1 margin sentinel, and flooring the margin would corrupt every
// downstream effect that scales by it.
//
// Floors are parameters rather than config reads so the ordering guard can be
// tested deterministically at floor 1.0.
func applyCritFloors(res *hitResolution, result *AttackResult, best bestDefenseResult, attackFloor, defenseFloor float64) {
	// A fumble is the attacker's own blunder, not a contested outcome. It is
	// neither a promotable hit nor a defensive success.
	if res.fumble {
		return
	}

	if res.hit {
		res.crit = ApplyCritFloor(res.crit, attackFloor)
		return
	}

	// The swing missed, so this is the defender's side. Require that a defence
	// was actually mounted: defenseType "" means the defender never acted and
	// the miss came from the 5.9 defence floor, which is not something to
	// reward with a riposte. setDefenseCritFlags would also have no flag to
	// set.
	if best.defenseType == "" {
		return
	}

	wasCrit := res.defenseCrit
	res.defenseCrit = ApplyCritFloor(res.defenseCrit, defenseFloor)
	if res.defenseCrit && !wasCrit {
		// Downstream riposte / auto-trip / auto-bash wiring reads the
		// per-defence flags, so a promotion that skipped these would produce a
		// crit that nothing acts on.
		setDefenseCritFlags(result, best)
	}
}
