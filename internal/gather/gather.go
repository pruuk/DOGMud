// Package gather is the shared roll behind the wilderness trades: skinning,
// butchering, chopping, sawing and (later) foraging. It answers three
// questions for any gathering job: did it work, what grade came out, and which
// tool was used.
//
// The roll is deliberately weighted toward the gatherer's BODY and TOOL, not
// their skill:
//
//	score      = avg(StatA, StatB) * toolMult + skill * Balance.GatherSkillWeight
//	difficulty = Balance.GatherBaseDifficulty + target tier
//
// GatherSkillWeight (1.5) is far below the craft SkillWeight (5.0), so thirty
// ranks of skill are worth about what a better tool is worth. The grade comes
// from the contest margin, measured in roll standard deviations, and is capped
// by the tool's tier: only a masterwork tool can produce pristine material.
package gather

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Job describes one kind of gathering work.
type Job struct {
	Name  string // for logs and messages: "skin", "butcher", "chop"
	StatA string // stat names as characters.GetStatValue takes them
	StatB string

	// Skill gives a light edge (Balance.GatherSkillWeight per rank). Empty
	// means the job uses no skill at all, which is right for pure labour
	// such as chopping.
	Skill skills.SkillTag

	// Tool is the tool type the job uses. Empty means the job uses no tool:
	// no multiplier and no grade cap beyond pristine.
	Tool items.ToolType
	// ToolRequired refuses the job outright without the tool. When false and
	// no tool is found, the job runs at the crude multiplier and crude cap.
	ToolRequired bool
}

// Jobs available today. Later phases add their own (chop, saw, forage).
var (
	JobSkin = Job{
		Name: `skin`, StatA: `dexterity`, StatB: `perception`,
		Skill: skills.Salvage, Tool: items.ToolKnife, ToolRequired: true,
	}
	JobButcher = Job{
		Name: `butcher`, StatA: `strength`, StatB: `dexterity`,
		Skill: skills.Salvage, Tool: items.ToolKnife, ToolRequired: true,
	}
	// JobChop is pure labour: strength to drive the axe, vitality to keep
	// swinging. No skill.
	JobChop = Job{
		Name: `chop`, StatA: `strength`, StatB: `vitality`,
		Tool: items.ToolAxe, ToolRequired: true,
	}
	// JobMine is labour too: strength to drive the pick, vitality to keep at
	// it underground. No skill.
	JobMine = Job{
		Name: `mine`, StatA: `strength`, StatB: `vitality`,
		Tool: items.ToolPick, ToolRequired: true,
	}
)

// Result is the outcome of one gathering roll.
type Result struct {
	// Success means the job produced something. Grade is its quality.
	Success bool
	Grade   items.Quality

	// NoTool is set when the job requires a tool and none was found. Nothing
	// was rolled.
	NoTool bool

	Tool    Tool // zero when no tool was used
	HasTool bool

	Score      float64
	Difficulty float64
	// Sigmas is the contest margin in roll standard deviations
	// (positive = won). Zero when the outcome was flipped by the mercy floor.
	Sigmas  float64
	Floored bool
}

// Inputs are the numbers a roll is made from. Split out of Roll so the grade
// logic can be tested without a character.
type Inputs struct {
	StatAvg    float64
	SkillLevel int
	ToolTier   items.ToolTier // ToolTierNone = no tool used
	UsesTool   bool           // the job has a tool type at all
	SightMult  float64        // messaging.SightMult; 1.0 in full light
	TargetTier float64        // added to GatherBaseDifficulty
}

// Roll runs one gathering roll for a character working in room. targetTier is
// the target's contribution to difficulty (mob level and size, tree species
// tier...). The gatherer has to see the work, so the score pays the sight ramp
// (messaging.SightMult) here, once; a nil room means full light (tests, and
// any caller with no room to see in).
func Roll(c *characters.Character, room messaging.RoomVisibility, job Job, targetTier float64) Result {
	res := Result{}

	tier := items.ToolTierNone
	if job.Tool != `` {
		if t, ok := BestTool(c, job.Tool); ok {
			res.Tool, res.HasTool = t, true
			tier = t.Tier
		} else if job.ToolRequired {
			res.NoTool = true
			return res
		}
	}

	skill := 0
	if job.Skill != `` {
		skill = c.GetSkillLevel(job.Skill)
	}

	in := Inputs{
		StatAvg:    (float64(c.GetStatValue(job.StatA)) + float64(c.GetStatValue(job.StatB))) / 2,
		SkillLevel: skill,
		ToolTier:   tier,
		UsesTool:   job.Tool != ``,
		SightMult:  1.0,
		TargetTier: targetTier,
	}
	if room != nil {
		in.SightMult = messaging.SightMult(c, room)
	}
	r := rollInputs(in)
	r.Tool, r.HasTool = res.Tool, res.HasTool
	return r
}

// Score is the gatherer's side of the contest for the given inputs.
func Score(in Inputs) float64 {
	b := configs.GetBalanceConfig()
	mult := 1.0
	if in.UsesTool {
		mult = TierMult(in.ToolTier)
	}
	sight := in.SightMult
	if sight <= 0 {
		sight = 1.0
	}
	return (in.StatAvg*mult + float64(in.SkillLevel)*float64(b.GatherSkillWeight)) * sight
}

// Difficulty is the target's side of the contest.
func Difficulty(in Inputs) float64 {
	return float64(configs.GetBalanceConfig().GatherBaseDifficulty) + in.TargetTier
}

func rollInputs(in Inputs) Result {
	score := Score(in)
	diff := Difficulty(in)
	cr := crafting.RunSalvageContest(score, diff)
	return gradeFrom(cr, score, diff, in)
}

// gradeFrom turns a contest result into a Result. Split out so it can be
// tested with fixed contest outcomes.
//
// Grades:
//   - a win gives standard, plus one grade per GatherGradeStepSigma of margin
//   - a narrow loss (within half a step) still gives crude: something came off
//     the carcass, just badly. This is what keeps a novice from walking away
//     empty-handed most of the time.
//   - a win granted by the mercy floor gives crude
//   - anything else gives nothing
//
// The grade is then capped by the tool tier (items.ToolTier.MaxGrade). A job
// with no tool type is capped only at pristine; a tool job done without its
// tool is capped as crude.
func gradeFrom(cr contest.Result, score, diff float64, in Inputs) Result {
	res := Result{Score: score, Difficulty: diff, Floored: cr.Floored}

	step := float64(configs.GetBalanceConfig().GatherGradeStepSigma)
	if step <= 0 {
		step = 1.0
	}

	if !cr.Floored {
		// Both rolls use the attacker's stdDev (contest.Run), so the spread of
		// the margin is stdDev*sqrt(2).
		sd := cr.AttackRoll.StdDev * math.Sqrt2
		if sd > 0 {
			res.Sigmas = cr.Margin / sd
		}
	}

	switch {
	case cr.Success && cr.Floored:
		res.Grade = items.QualityCrude
	case cr.Success:
		res.Grade = items.QualityStandard + items.Quality(int(res.Sigmas/step))
	case !cr.Floored && res.Sigmas > -0.5*step:
		res.Grade = items.QualityCrude
	default:
		return res // nothing
	}

	maxGrade := items.QualityPristine
	if in.UsesTool {
		tier := in.ToolTier
		if tier == items.ToolTierNone {
			tier = items.ToolTierCrude
		}
		maxGrade = tier.MaxGrade()
	}
	if res.Grade > maxGrade {
		res.Grade = maxGrade
	}
	res.Grade = res.Grade.Clamp()
	res.Success = true
	return res
}

// Rounds scales a job's base duration by the tool's speed, never below 1.
func Rounds(base int, t Tool, hasTool bool) int {
	if base < 1 {
		base = 1
	}
	if !hasTool || t.Speed <= 0 {
		return base
	}
	r := int(math.Ceil(float64(base) / t.Speed))
	if r < 1 {
		r = 1
	}
	return r
}

// CraftGrade is the grade of a crafted output (wilderness trades, phase 3).
//
// Grading only applies when it means something: when at least one consumed
// input carries a grade, or the recipe needs a tool. Otherwise the output is
// ungraded (QualityNone), so every existing recipe behaves exactly as before.
//
// cr is the craft contest; nil for an instant recipe, which runs none and
// starts at standard. A win is standard plus one grade per
// GatherGradeStepSigma of margin; a win granted by the mercy floor is crude.
// The result is then capped twice:
//   - one grade above the WORST graded input, so a pristine output cannot be
//     made from crude hides by mixing in one good one;
//   - by the tool's tier, when the recipe has a tool.
func CraftGrade(cr *contest.Result, consumed []items.Item, recipeHasTool bool, toolTier items.ToolTier) items.Quality {
	return CraftGradeOutput(cr, consumed, recipeHasTool, toolTier, false)
}

// CraftGradeOutput is CraftGrade with the output in view. outputIsTool marks
// a recipe that makes a tool (a forged knife, a felling axe): a tool is always
// graded, even from ungraded ingots, because its grade is what sets how well
// it works and how long it lasts (items.EffectiveToolTier, ToolDurability).
// The smith's margin decides it, so a master smith's steel knife can come out
// pristine and work like a masterwork one.
func CraftGradeOutput(cr *contest.Result, consumed []items.Item, recipeHasTool bool, toolTier items.ToolTier, outputIsTool bool) items.Quality {
	worst := items.QualityNone
	for _, itm := range consumed {
		if itm.Quality.Valid() && (worst == items.QualityNone || itm.Quality < worst) {
			worst = itm.Quality
		}
	}
	if worst == items.QualityNone && !recipeHasTool && !outputIsTool {
		return items.QualityNone
	}

	grade := items.QualityStandard
	if cr != nil {
		if cr.Floored {
			grade = items.QualityCrude
		} else if sd := cr.AttackRoll.StdDev * math.Sqrt2; sd > 0 && cr.Margin > 0 {
			step := float64(configs.GetBalanceConfig().GatherGradeStepSigma)
			if step <= 0 {
				step = 1.0
			}
			grade += items.Quality(int((cr.Margin / sd) / step))
		}
	}

	if worst != items.QualityNone && grade > worst+1 {
		grade = worst + 1
	}
	if recipeHasTool {
		tier := toolTier
		if tier == items.ToolTierNone {
			tier = items.ToolTierCrude
		}
		if m := tier.MaxGrade(); grade > m {
			grade = m
		}
	}
	return grade.Clamp()
}
