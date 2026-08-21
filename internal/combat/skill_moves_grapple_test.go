package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

// makeGrappleTestParams builds SkillMoveParams that HIT (via the forced
// clean-win runner below) and roll a knockdown (KnockdownFactor 1000.0,
// overwhelming and certain with the floor off — U10 made knockdown an
// opposed contest, so callers asserting a deterministic KnockedDown must
// both pin the floor off and use a factor no defender score can contest),
// so the only variable under test is whether the FSM knockdown transition
// succeeds.
//
// U6b Task 7: converted off the legacy scalar-defence shape (AttackStat 1000
// vs DefenseStat 1 at x1 weights — a pinned defect) onto the seam. The hit is
// now forced through cleanWinRunner instead of pinContestFloorOff + a huge
// stat gap: on the seam a real roll can FUMBLE (~2% abort even on a won
// contest), which would make single-shot Hit asserts flaky.
func makeGrappleTestParams(defender *characters.Character) SkillMoveParams {
	atk := characters.New()
	return SkillMoveParams{
		Attacker: atk, Defender: defender,
		Channel: ChannelMelee,
		Attack: AttackSide{
			Stat: 1000, StatName: "strength",
			Skill: skills.WeaponCombat, SkillRank: 50,
			Mult: 1.0,
		},
		DamagePercent: 1.0, KnockdownFactor: 1000.0,
		DamageStat: 100, MitigationMultiplier: 1.0,
	}
}

// cleanWinRunner forces an undramatic attack win: contested, Success, a
// margin well under the crit bar, and ZScore 0 on both rolls (no crit, no
// fumble). The seam's defence-set/costing/progression plumbing still runs.
func cleanWinRunner(atkScore float64, entries []contest.Entry) contest.Result {
	stdDev := dice.StdDevFor(atkScore)
	return contest.Result{
		Contested: true,
		Winner:    entries[0].Name,
		Success:   true,
		Margin:    0.5 * stdDev * math.Sqrt2,
		AttackRoll: dice.RollResult{
			Value: atkScore, Mean: atkScore, StdDev: stdDev, ZScore: 0,
		},
		DefenseRoll: dice.RollResult{
			Mean: entries[0].Score, StdDev: stdDev, ZScore: 0,
		},
	}
}

// A standing target: the knockdown transition succeeds → KnockedDown true.
func TestExecuteSkillMove_KnockdownStanding(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	pinContestFloorOff(t)

	d := characters.New()
	d.HealthMax.Value = 1000
	d.Health = 1000
	setCombatPositionParallel(d, position.Standing)

	res := executeSkillMoveWithRunner(makeGrappleTestParams(d), cleanWinRunner)
	assert.True(t, res.Hit, "forced clean win should hit")
	assert.True(t, res.StatusApplied, "a landed maneuver must set StatusApplied")
	assert.True(t, res.KnockedDown, "standing target should be knocked down")
}

// Grapple move-collision fix: a target already in a controlled grapple position
// can't be knocked down (TransitionToProne fails), so KnockedDown must be false
// — no misleading "you knock them down!" narration. Damage still applies.
func TestExecuteSkillMove_KnockdownGrappledIsSuppressed(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	pinContestFloorOff(t)

	d := characters.New()
	d.HealthMax.Value = 1000
	d.Health = 1000
	setCombatPositionParallel(d, position.Clinch) // controlled grapple, not Standing

	res := executeSkillMoveWithRunner(makeGrappleTestParams(d), cleanWinRunner)
	assert.True(t, res.Hit, "the move still connects")
	assert.False(t, res.KnockedDown, "knockdown must be suppressed when the FSM transition fails")
	assert.Greater(t, res.Damage, 0, "damage still applies even without knockdown")
}
