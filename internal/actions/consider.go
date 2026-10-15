package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ConsiderResult is the outcome of a consider (#301).
type ConsiderResult struct {
	Read           FightRead
	Quality        ReadQuality // ReadNone for a mob or the companion, or a refused read
	OnCooldown     bool        // a player's read refused: the special-move cooldown is running
	TargetName     string
	TargetIsPlayer bool
}

// considerContest rolls a player's read. Tests pin it.
var considerContest = combat.RunContest

// Consider reads a fight from actor's side (#301): a verdict and two paces
// predicted from combat's own math (ReadFight).
//
// A player's consider is a skill read. It claims the shared special-move
// cooldown, so it competes with special moves and spells; then rolls
// Perception plus Search against the target's Charisma plus Rhetoric through
// combat.RunContest, the reader's score paying the sight ramp. The roll sets
// how clear the read is, a lost roll slides a middle verdict toward barely,
// and every roll trains Search (and through it Perception) through
// AwardResolved. The cooldown is the gate on that training: consider was an
// ungated perception faucet until U10b-0 Phase D removed its progression, and
// consider_no_progression_test.go keeps the training behind the claim.
//
// A mob's consider (and the AI companion's) is the bare prediction: no roll,
// no cooldown, no training, no text.
func Consider(actor Actor, target Actor) ConsiderResult {
	selfChar := actor.GetCharacter()
	targetChar := target.GetCharacter()
	result := ConsiderResult{TargetName: target.GetName(), TargetIsPlayer: target.IsPlayer()}

	if !actor.IsPlayer() {
		result.Read = ReadFight(selfChar, targetChar)
		return result
	}

	if !ClaimSpecialMove(selfChar) {
		result.OnCooldown = true
		refusal.Cooldown(actor, "consider anyone")
		return result
	}

	bal := configs.GetBalanceConfig()
	readScore := float64(selfChar.Stats.Perception.ValueAdj) +
		float64(selfChar.GetSkillLevel(skills.Search))*float64(bal.SkillWeight)
	// sight ramp (plan 5b): the reader needs to see the target to size it up.
	// The target's side is voice and bearing, so it takes none.
	readScore *= messaging.SightMult(selfChar, combat.SightRoom(actor.GetRoom()))
	targetScore := float64(targetChar.Stats.Charisma.ValueAdj) +
		float64(targetChar.GetSkillLevel(skills.Rhetoric))*float64(bal.SkillWeight)
	roll := considerContest(readScore, []contest.Entry{{Name: string(skills.Rhetoric), Score: targetScore}})
	result.Quality = readQualityFor(roll, readScore, bal)
	// The resolved-contest convention (U10b-1): full weight on a won read,
	// Balance.ProgressionFailureFraction on a lost one.
	actor.AwardResolved(roll.Success, selfChar.CandidateFor(string(skills.Search)))

	result.Read = ReadFight(selfChar, targetChar)
	if result.Quality == ReadMuddy {
		result.Read.Verdict = muddySlide(result.Read.Verdict)
	}

	considerType := "mob"
	if result.TargetIsPlayer {
		considerType = "user"
	}
	name := fmt.Sprintf(`<ansi fg="%sname">%s</ansi>`, considerType, result.TargetName)
	actor.SendText(messaging.CategorySystem, openerText(result.Quality, name))
	actor.SendText(messaging.CategorySystem, verdictLine(result.Read))
	actor.SendText(messaging.CategorySystem, paceLineFor(result.Read, result.Quality))
	return result
}
