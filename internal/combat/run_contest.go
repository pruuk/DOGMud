package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunContest is THE entry point for every opposed contest in the game.
//
// It exists so that Balance.ContestFloor is read in exactly one place. Before
// U6 there were three wrapper pairs (maneuver, spell, global) over eight knobs,
// and because config.yaml shipped all three pairs at similar values, wiring a
// site to the wrong pair was invisible in production and became a live balance
// bug the moment one pair was retuned. One value removes the failure mode
// rather than guarding against it.
//
// The floor is passed down rather than read inside internal/contest, which is a
// config-free leaf by design.
//
// SCOPE: opposed contests only. Static-difficulty rolls -- search, track,
// forage, concentration -- are roadmap categories B and C and are deliberately
// unfloored. Do not route them here to "unify" them.
func RunContest(atkScore float64, entries []contest.Entry) contest.Result {
	return contest.RunWithFloors(atkScore, entries, float64(configs.GetBalanceConfig().ContestFloor))
}
