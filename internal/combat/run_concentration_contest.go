package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunConcentrationContest resolves a caster's hold on a forming spell. The
// caster is always the attack side; disruption is a static difficulty
// (damagePct x10, position lattice x10) or a live opposing score (the
// throttle interrupt). It is the ONE place Balance.ConcentrationFloor is
// read, exactly as RunContest is for ContestFloor — concentration gets its
// own, much smaller mercy band because the standard floor would break a
// master one disruption in eight.
//
// Success = the caster HELD.
func RunConcentrationContest(casterScore, disruption float64) contest.Result {
	return contest.RunWithFloors(casterScore,
		[]contest.Entry{{Score: disruption}},
		float64(configs.GetBalanceConfig().ConcentrationFloor))
}
