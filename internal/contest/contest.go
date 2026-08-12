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
	"math/rand"

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

	// Success reports whether the ATTACKER won. Ordinarily this is just
	// Margin > 0, but RunWithFloors can flip it without changing the sign of
	// the sentinel margin it stamps, so callers that care about the outcome
	// must read this rather than re-deriving it from Margin.
	Success bool

	// Floored reports whether a contest floor CHANGED this outcome.
	//
	// Without it, the only way to ask is comparing Margin against the +-1
	// sentinel: that means knowing an internal constant, is ambiguous against a
	// genuine roll landing exactly there, and breaks silently if the sentinel is
	// ever retuned. Roadmap section 8 names floor-reliance rate as something
	// that must be MODELLED before U6 flips the defence model, so it has to be
	// answerable cheaply.
	Floored bool
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

	res.Success = res.Contested && res.Margin > 0

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

// RunWithFloors is Run plus the 5.9 contest floors: a last-resort probability
// that an outcome is flipped, so a hopelessly outclassed actor is never simply
// incapable and an overwhelming one is never simply guaranteed.
//
// TRANSITIONAL. This exists so U2-U5 can be provable no-ops. The codebase has
// TWO floor styles and this reproduces only one: melee applies its floors AFTER
// the contest, in resolveDefenseOutcomeCore, flipping a hit with no margin
// involved; spell and maneuver apply theirs INSIDE the roll and need the
// sentinel margin to stop a floored hit from also critting. Roadmap section 8
// lists reconciling the two as an OPEN question for U6, which may delete or
// reshape this function entirely. Do not build new permanent behaviour on it
// without checking where that landed.
//
// It reproduces dice.OpposedRollStatWithFloors exactly, because callers are
// being migrated onto it and must not change behaviour:
//
//   - At most ONE floor is rolled per call. If the attack lost, only
//     floorSuccess is considered; if it won, only floorResist. Drawing both
//     would change the outcome distribution.
//   - A flipped outcome carries a SENTINEL margin of +1 or -1, not its real
//     margin. This is load-bearing: ContestCrit normalises that sentinel to a
//     near-zero z, which is the only reason a floor-granted hit cannot also be
//     a critical hit. Leak the real margin here and a hopeless attacker rescued
//     by the floor would crit.
//   - Both floors clamp to [0, 0.5]. Above that a floor stops being a last
//     resort and becomes the dominant term.
//
// An uncontested result is returned untouched — there is no outcome to flip.
func RunWithFloors(atkScore float64, entries []Entry, floorSuccess, floorResist float64) Result {
	res := Run(atkScore, entries)
	if !res.Contested {
		return res
	}

	floorSuccess = clampFloor(floorSuccess)
	floorResist = clampFloor(floorResist)

	switch {
	case !res.Success && floorSuccess > 0 && rand.Float64() < floorSuccess:
		res.Success, res.Margin, res.Floored = true, 1, true
	case res.Success && floorResist > 0 && rand.Float64() < floorResist:
		res.Success, res.Margin, res.Floored = false, -1, true
	}

	return res
}

// clampFloor bounds a floor to [0, 0.5], matching dice.clampFloor. Duplicated
// rather than imported because dice keeps it unexported, and this package takes
// its tunables as parameters by design.
func clampFloor(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 0.5 {
		return 0.5
	}
	return v
}
