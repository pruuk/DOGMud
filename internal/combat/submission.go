package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// SubmissionTier classifies the outcome of a per-round submission
// attempt roll into one of four bands. See spec
// "Per-round submission tick mechanics".
type SubmissionTier int

const (
	SubTierBad     SubmissionTier = iota // Attempter overcommits; falls Prone, pair breaks to Standing
	SubTierNeutral                       // Failed but no consequence; pair stays
	SubTierSuccess                       // Sub locks; outcome resolves via attempter's SubmissionPolicy
	SubTierCrit                          // Sub locks AND recipient is Stunned next round (T10 buff)
)

func (t SubmissionTier) String() string {
	switch t {
	case SubTierBad:
		return "bad"
	case SubTierNeutral:
		return "neutral"
	case SubTierSuccess:
		return "success"
	case SubTierCrit:
		return "crit"
	default:
		return "unknown"
	}
}

// SubmissionAttemptResult is the output of RollSubmissionAttempt.
// All fields are useful downstream: SubType drives narration +
// body-part mapping, Tier drives the outcome branch, AttackerScore /
// DefenderScore are exposed for analytics + debugging, and
// AttackerZScore is the basis for the BAD tier only — since U6b Task 13
// the stun-crit tier derives from the normalized contest margin vs
// CritBarFor, not from this self-relative z.
type SubmissionAttemptResult struct {
	SubType        position.SubmissionType
	Tier           SubmissionTier
	AttackerScore  float64
	DefenderScore  float64
	AttackerZScore float64
	DefenderZScore float64
	Margin         float64 // attacker margin (positive = attacker won)
}

// RollSubmissionAttempt rolls a fresh opposed Strength + Unarmed-
// combat-skill check between attempter and recipient. This is a
// SEPARATE roll from the chunk-4b drift roll — drift gates the
// opportunity, this roll resolves the attempt.
//
// Formula (U6b Task 13: the old sub-only skill weight knob (1.5) — a one-off regime
// shared with nothing else — is deleted; both sides use the global
// SkillWeight like every other additive contest score):
//
//	attackerScore = attempter.Strength
//	              + attempter.UnarmedCombatSkill * SkillWeight
//	defenderScore = recipient.Strength
//	              + recipient.Vitality
//	              + recipient.UnarmedCombatSkill * SkillWeight
//
// Stun-crit is a margin crit: the normalized contest margin measured
// against CritBarFor over both sides' unarmed-combat ranks. The old
// self-relative z-threshold form was opponent-blind (a flat ~2%
// stun rate); modelling puts the margin form at 18-62% depending on
// matchup, accepted for playtest.
func RollSubmissionAttempt(
	attempter *characters.Character,
	recipient *characters.Character,
	subType position.SubmissionType,
) SubmissionAttemptResult {
	cfg := configs.GetBalanceConfig()
	skillWeight := float64(cfg.SkillWeight)
	atkRank := attempter.GetSkillLevel(skills.UnarmedCombat)
	defRank := recipient.GetSkillLevel(skills.UnarmedCombat)

	atkScore := float64(attempter.Stats.Strength.ValueAdj) +
		float64(atkRank)*skillWeight
	defScore := float64(recipient.Stats.Strength.ValueAdj) +
		float64(recipient.Stats.Vitality.ValueAdj) +
		float64(defRank)*skillWeight

	res := RunContest(atkScore, []contest.Entry{{Score: defScore}})

	// A floored outcome carries the ±1 sentinel margin and can never crit.
	stunCrit := !res.Floored &&
		normalizedContestMargin(res.Margin, res.AttackRoll) >= CritBarFor(atkRank, defRank)

	return SubmissionAttemptResult{
		SubType:        subType,
		Tier:           ClassifySubmissionTier(res.Success, stunCrit, res.AttackRoll.ZScore),
		AttackerScore:  atkScore,
		DefenderScore:  defScore,
		AttackerZScore: res.AttackRoll.ZScore,
		DefenderZScore: res.DefenseRoll.ZScore,
		Margin:         res.Margin,
	}
}

// ClassifySubmissionTier maps (success, stun-crit, attacker z-score) to a
// tier per the spec table. Exposed for unit testing of the boundary
// conditions independently from the dice roll.
//
// U6b Task 13: stunCrit arrives pre-decided (margin vs CritBarFor, see
// RollSubmissionAttempt) and only promotes a SUCCESSFUL roll. The bad band
// stays on the self-relative z ON PURPOSE — a fumble is the attempter's own
// blunder, not a contested outcome.
func ClassifySubmissionTier(success bool, stunCrit bool, attackerZ float64) SubmissionTier {
	if success {
		if stunCrit {
			return SubTierCrit
		}
		return SubTierSuccess
	}
	if attackerZ < float64(configs.GetBalanceConfig().SubBadZThreshold) {
		return SubTierBad
	}
	return SubTierNeutral
}

// Role discriminates whether a sub attempt comes from the top
// (controller) side of a grapple or the bottom (controlled) side.
// Used by Position_SubmissionTick + Position_Messaging to pick the
// right submission pool and narration.
type Role int

const (
	RoleTop    Role = iota
	RoleBottom Role = iota
)

func (r Role) String() string {
	switch r {
	case RoleTop:
		return "top"
	case RoleBottom:
		return "bottom"
	default:
		return "unknown"
	}
}
