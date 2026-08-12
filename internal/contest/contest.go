// Package contest is the shared resolution seam for every opposed roll in the
// game: it rolls one attack against N defences and reports which defence did
// best.
//
// It deliberately does NOT compute scores, spend resources, apply mitigation, or
// know what a Character is. Callers build fully-modified scores and hand them
// over. That keeps this package a leaf — it imports only internal/dice — so
// heavy packages and light ones alike can call it without a cycle.
//
// The purity is also what makes it testable. A Go test binary never loads
// _datafiles/config.yaml, so a core that read balance config would be tested
// against Go defaults, and any knob that legitimately defaults to zero would
// make its assertions vacuously true.
package contest

import (
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// Entry is one contestant on the defending side.
//
// Score must be FULLY modified by the caller — every multiplier, penalty and
// bonus already applied. This package does no score maths.
//
// Name identifies the winner to the caller. An empty Name is legal and is how a
// contest against a static difficulty is expressed: one entry, no name.
type Entry struct {
	Name  string
	Score float64
}

// Result is the outcome of one contest.
type Result struct {
	// AttackRoll is the single roll every entry contested. Always populated,
	// even when nothing was contested, so callers can still read its ZScore for
	// legacy self-relative checks such as fumbles.
	AttackRoll dice.RollResult

	// DefenseRoll is the roll of the entry that defended best. Zero when
	// Contested is false.
	DefenseRoll dice.RollResult

	// Margin is ATTACK-POSITIVE: AttackRoll.Value - DefenseRoll.Value.
	// Positive means the attacker won.
	//
	// This is the opposite of internal/combat's bestDefenseResult.margin, which
	// is defence-positive. That mismatch is exactly why mixing the two compiles
	// cleanly and silently puts crit on the losing side. This package is the
	// single convention; converters live at the caller's boundary.
	//
	// Zero when Contested is false — NEVER an infinity. bestDefenseResult
	// initialises its margin to math.Inf(-1) and only overwrites it inside the
	// defence loop, so a defender with no usable defence leaves it there;
	// negated, that reads as an infinitely decisive attack and crits every swing.
	Margin float64

	// Winner is the Name of the entry that defended best. Empty when Contested
	// is false — and note it is ALSO empty for a legitimately-unnamed static
	// difficulty entry, so test Contested, never Winner, to ask whether a
	// contest happened.
	Winner string

	// Contested reports whether any entry was rolled against.
	Contested bool
}

// Run rolls the attack ONCE and contests it against every entry, reporting the
// entry that defended by the widest margin.
//
// The attack is rolled once on purpose: all defences contest the same swing, so
// a defender with three defences gets three chances to beat one roll rather than
// three separate swings to survive.
//
// Every defence is rolled with the ATTACKER's standard deviation. Downstream
// crit maths divides the margin by stdDev*sqrt(2) on the strength of that, so
// rolling a defence with its own spread would silently shift crit rates
// everywhere.
func Run(atkScore float64, entries []Entry) Result {
	stdDev := dice.StdDevFor(atkScore)
	attackRoll := dice.Roll(atkScore, stdDev)

	res := Result{AttackRoll: attackRoll}

	for _, e := range entries {
		defenseRoll := dice.Roll(e.Score, stdDev)
		margin := attackRoll.Value - defenseRoll.Value

		// The best defence is the one that leaves the SMALLEST attack-positive
		// margin. First entry always wins the comparison because Contested is
		// still false.
		if !res.Contested || margin < res.Margin {
			res.Contested = true
			res.Margin = margin
			res.Winner = e.Name
			res.DefenseRoll = defenseRoll
		}
	}

	return res
}

// AgainstDifficulty contests a score against a fixed number rather than against
// an opponent — searching a room, following a trail, foraging, recovering from
// prone with nobody holding you down.
//
// It is deliberately the same code path as Run, so a difficulty check produces
// the same crit, fumble and margin semantics as any other contest. The
// alternative — a separate threshold helper — is how the codebase ended up with
// several unrelated ways to decide the same kind of question.
//
// The result has no Winner name. Ask Contested, not Winner, to find out whether
// a contest happened.
func AgainstDifficulty(score, difficulty float64) Result {
	return Run(score, []Entry{{Score: difficulty}})
}
