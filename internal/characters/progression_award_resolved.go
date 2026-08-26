package characters

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// AwardResolved is THE entry point for U10b-1's firing rule on a resolved
// action. ONE event, for the Best-of winner. Full weight on a win,
// ProgressionFailureFraction on a loss.
//
// The actor is always the ATTACKER side: this function is called by the party
// who took the action, about their own action, so won == false means the actor
// lost and Outcome.Defended is therefore !won. Inverting that pair scales the
// defender's event instead, which ApplyProgression then never applies for
// SideAttacker -- so a lost action would silently pay a FULL weight, unlost
// award. TestAwardResolved_ALossIsTheActorsLoss is the guard.
//
// This is also the ONE place ProgressionFailureFraction is read. internal/
// progression is deliberately dependency-free and reads no config (a Go test
// binary never loads _datafiles/config.yaml, so a package that read balance
// config would be tested against Go defaults rather than shipped values), so
// the knob has to enter at the boundary. Keeping that boundary a single
// function is what stops call sites from each fetching the knob and drifting.
//
// Candidates are rolled BEFORE they arrive here, by CandidateFor. Pass every
// skill the action could plausibly train; BestOf picks exactly one.
func (c *Character) AwardResolved(userId int, won bool, candidates ...progression.Candidate) {
	if c == nil {
		return
	}
	best, ok := progression.BestOf(candidates)
	if !ok {
		return
	}
	frac := float64(configs.GetBalanceConfig().ProgressionFailureFraction)
	o := progression.Outcome{
		AttackerSkill: best.Skill,
		AttackerStat:  best.Stat,
		Defended:      !won,
	}
	c.ApplyProgression(progression.OrdinaryEventsScaled(o, frac),
		progression.SideAttacker, userId, util.GetRoundCount())
}

// CandidateFor builds the standard progression Candidate for one skill: the
// skill's level, and one dice.RollStat against stat + level*SkillWeight -- the
// same additive shape combat composes its scores with. Every candidate in a
// Best-of contest must be rolled this way, so no call site hand-rolls one.
//
// It leaves Candidate.Stat EMPTY, which means "the skill's primary stat". That
// is what every current caller wants, and it is not merely cosmetic: a
// populated Stat that differs from the primary makes ApplyProgression pay a
// SECOND stat roll on top of the one OnSkillUseScaled already made.
//
// NOTE FOR THE CALL-SITE CONVERSIONS (U10b-1 Task 7 onward): the defences need
// a variant of this that takes an EXPLICIT stat, and that variant must roll on
// the EXPLICIT stat's value, not on the primary's. combat.DefenceSkillAndStat
// awards block as weapon-combat/STRENGTH and defy as rhetoric/WILLPOWER, while
// those two skills' primaries are dexterity and charisma. Rolling the primary
// while awarding the explicit stat would mean a block candidate competes on
// dexterity and then trains strength, and nothing in the type system, the
// tests, or the game text would show it. dodge, parry and quell each award
// their own skill's primary and are served correctly by this function as it
// stands. Deliberately not built yet -- it has no caller until Task 7.
//
// ONE DELIBERATE EXCEPTION, shipped by Task 9: hooks.bestSwingDefence does not
// call this function at all. It builds its Candidates from the defence rolls
// the melee contest ALREADY made that round, because re-rolling would stack a
// second source of randomness on top of the roll that already decided the
// swing. Those Candidates are selection-only -- they never reach
// ApplyProgression, so the populated-Stat hazard above does not apply to them.
// Any OTHER site that needs a Candidate must still come through here.
//
// An unknown skill has no primary stat and therefore no stat value to roll, so
// it comes back as the zero Candidate: inert, awarding nothing, and losing
// every contest it enters. Returning a named candidate rolled against
// GetStatValue("") == 0 instead would let a typo win a Best-of on a lucky roll
// and then reach CheckSkillProgression, which banners an unrecognised name to
// the player verbatim. It is logged rather than swallowed, since an unknown
// skill here is a programming error at a call site.
func (c *Character) CandidateFor(skill string) progression.Candidate {
	if c == nil || skill == "" {
		return progression.Candidate{}
	}
	stat := skills.GetSkillPrimaryStat(skill)
	if stat == "" {
		mudlog.Error("Progression", "err", "CandidateFor called with a skill that has no primary stat", "skill", skill)
		return progression.Candidate{}
	}
	level := c.GetSkillLevel(skills.SkillTag(skill))
	weight := float64(configs.GetBalanceConfig().SkillWeight)
	score := float64(c.GetStatValue(stat)) + float64(level)*weight
	return progression.Candidate{
		Skill: skill,
		Roll:  dice.RollStat(score).Value,
		Level: level,
	}
}
