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
// Denominated in WINS, not swings, and this matters. A badly outclassed
// attacker wins roughly 15% of contests, so "1% of swings" would demand about
// 6.7% of their wins be crits — a HIGHER per-win rate than an even match gets
// at 2.3%, which is incoherent. 1% of wins composes with the contest floor as
// two independent last resorts, each sized to the failure it protects.
//
// U6 Task 9 restated that denominator from HITS to CONTEST WINS. They were the
// same set of swings until Task 10 made a defensive win deal partial damage; see
// applyCritFloors for why the hit framing stops being answerable there.
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
// second hit floor stacked on ContestFloor.
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
//
// MARGIN CONTRACT, same as ContestCrit's: pass an ATTACK-signed margin, which
// since U3 means `contest.Result.Margin` UNNEGATED, because that field is
// already attack-positive. Never pass a dice.RollResult's `.Margin`
// (`res.AttackRoll.Margin`, `res.DefenseRoll.Margin`): internal/contest rolls
// via dice.Roll, which does not populate that field, so it is a silent constant
// zero and nothing crits again. Never pass `-Result.Margin` here either; that is
// the defensive sign and puts the crit on the losing side. All three mistakes
// compile and break no test but the sign guards.
func AttackContestCrit(margin float64, roll dice.RollResult) bool {
	return ApplyCritFloor(ContestCrit(margin, roll), AttackCritFloor())
}

// DefenseContestCrit is the defensive mirror, used where a defender fully
// negates an incoming spell or taunt.
//
// MARGIN CONTRACT: pass a DEFENCE-signed margin, which since U3 means
// `-contest.Result.Margin`, negated because that field is attack-positive.
// Never pass a dice.RollResult's `.Margin` (`res.DefenseRoll.Margin` is the
// tempting one): internal/contest rolls via dice.Roll, which does not populate
// that field, so it is a silent constant zero. Never pass the attack-positive
// `Result.Margin` unnegated either. Both callers, TryStoicResolve and
// TrySpellDeflection, do it correctly today; internal/combat/contest_sign_test.go
// is what keeps them that way.
func DefenseContestCrit(margin float64, roll dice.RollResult) bool {
	return ApplyCritFloor(ContestCrit(margin, roll), DefenseCritFloor())
}

// applyCritFloors applies both floors to a resolved melee swing.
//
// It runs at the very END of resolveDefenseOutcome, after every branch has
// settled the outcome, and it is the only correct place for it. The melee
// resolver treats an attack crit as forcing a hit — the crit step returns
// res.hit = true on attackCrit — so a floor evaluated any earlier would silently
// become a second hit floor stacked on ContestFloor.
//
// U6 Task 9: the two floors are denominated by WHO WON THE CONTEST, not by
// whether the swing hit. See the comment on the margin test below.
//
// Note it deliberately does NOT floor the margin. A floored contest already
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

	// A floored outcome carries the +-1 sentinel margin and represents an
	// outcome the contest did not actually produce. Promoting it would hand a
	// decisive result to the side that lost the roll. This mirrors the gate in
	// resolveDefenseOutcomeCore, and like it, it is belt-and-braces today (the
	// sentinel normalises to a near-zero z) but it is the DECLARED rule, so a
	// future retune of the sentinel cannot silently reintroduce floored crits.
	if best.floored {
		return
	}

	// DENOMINATORS, decided in U6. The attack floor applies to swings that WON
	// THE CONTEST; the defence floor to swings the DEFENCE won. Before U6 the
	// split was `res.hit` versus a miss, which stops being answerable once a
	// defensive win deals partial damage: every partially deflected swing has
	// res.hit == true while the defence is the side that won.
	//
	// best.margin is DEFENCE-positive, so `<= 0` means the ATTACK won. Same
	// expression, same sign convention, as resolveDefenseOutcomeCore's
	// attackWon.
	//
	// An UNCONTESTED swing lands here too, and deliberately so: its margin sits
	// at math.Inf(-1), which reads as a decisive attack win, which is exactly
	// what it is -- no defence was mounted. Pre-U6 that swing had res.hit ==
	// true and took this same branch, so routing it here preserves the old
	// behaviour rather than quietly dropping a 1% promotion.
	if best.margin <= 0 {
		res.crit = ApplyCritFloor(res.crit, attackFloor)
		return
	}

	// The DEFENCE won. Require that a defence was actually mounted: an empty
	// defenseType leaves setDefenseCritFlags with no flag to set, so a
	// promotion here would produce a crit that nothing downstream acts on.
	//
	// This guard belongs on this branch and not above it. Hoisting it would
	// also block the attack floor on uncontested swings, which is a behaviour
	// change and not one U6 intends. It is believed unreachable in melee today
	// (runBestOfAllDefense only leaves defenseType empty on the uncontested
	// path, which the margin test above has already claimed) and is kept as the
	// declared contract rather than on the strength of that reachability
	// argument.
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

		// U6 Task 10: a defence crit fully negates. Before Task 10 a defensive
		// win already carried res.hit == false, so a promotion here needed to
		// say nothing about damage. Now an ordinary defensive win lands with
		// res.hit == true and a partial damageMult, so WITHOUT these two lines a
		// floor-promoted defence crit would deal 0-50% damage while a rolled one
		// deals none. This restores the pre-U6 outcome for this path rather than
		// changing it.
		res.hit = false
		res.damageMult = 0.0
	}
}
