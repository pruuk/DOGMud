package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// TrySpellDeflection rolls a defensive avoidance check for the target of a
// damage spell. Returns a damage multiplier: 1.0 (no deflection), the
// configured avoidance multiplier (partial deflection), or 0.0 (crit = full
// negation).
//
// Roll: Defender Perception + Spellcasting vs Attacker Willpower + Spellcasting.
//
// The defender always receives spellcasting and perception progression from
// the attempt regardless of outcome.
func TrySpellDeflection(attacker *characters.Character, defender *characters.Character, defenderUserId int) float64 {
	cfg := configs.GetBalanceConfig()
	skillWeight := float64(cfg.SkillWeight)

	atkSpellcasting := float64(attacker.GetSkillLevel(skills.Spellcasting)) * skillWeight
	attackScore := float64(attacker.Stats.Willpower.ValueAdj) + atkSpellcasting

	defSpellcasting := float64(defender.GetSkillLevel(skills.Spellcasting)) * skillWeight
	defenseScore := float64(defender.GetEffectivePerception()) + defSpellcasting

	floorHit, floorResist := SpellFloors()
	res := contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, floorHit, floorResist)

	defender.OnSkillUse(string(skills.Spellcasting), defenderUserId)
	defender.OnStatUse("perception", defenderUserId)

	if !res.Success {
		// A full negation is the defensive mirror of a crit, so it derives from
		// the same normalized margin.
		//
		// SIGN: contest.Result.Margin is ATTACK-positive and this is the
		// DEFENDER's crit check, so it is negated exactly here. This previously
		// read defRoll.Margin, which dice.OpposedRoll had already negated for
		// the defender; contest.Run uses dice.Roll, which does NOT populate
		// RollResult.Margin at all, so reading that field now would silently
		// pass zero and no deflection would ever crit again.
		if DefenseContestCrit(-res.Margin, res.DefenseRoll) {
			return 0.0
		}
		return float64(cfg.SpellAvoidanceDamageMultiplier)
	}

	return 1.0
}

// TODO(U3): still on dice.OpposedRollStatWithFloors while its sibling
// TrySpellDeflection above has moved to internal/contest. Deliberate, not an
// oversight: this is the conviction channel and roadmap U3 owns it. Note that
// U6 removes BOTH as parallel mechanisms, folding them into the defence
// multiplier, so do not invest in unifying them here.
//
// TryStoicResolve rolls a defensive avoidance check for the target of a
// hostile rhetoric attack. Returns a damage multiplier: 1.0 (no resolve),
// the configured avoidance multiplier (partial), or 0.0 (crit = full
// negation).
//
// Roll: Defender Willpower + Rhetoric vs Attacker Charisma + Rhetoric.
//
// The defender always receives rhetoric and willpower progression from the
// attempt regardless of outcome.
func TryStoicResolve(attacker *characters.Character, defender *characters.Character, defenderUserId int) float64 {
	cfg := configs.GetBalanceConfig()
	skillWeight := float64(cfg.SkillWeight)

	atkRhetoric := float64(attacker.GetSkillLevel(skills.Rhetoric)) * skillWeight
	attackScore := float64(attacker.Stats.Charisma.ValueAdj) + atkRhetoric

	defRhetoric := float64(defender.GetSkillLevel(skills.Rhetoric)) * skillWeight
	defenseScore := float64(defender.Stats.Willpower.ValueAdj) + defRhetoric

	floorHit, floorResist := ManeuverFloors()
	success, _, _, defRoll := dice.OpposedRollStatWithFloors(attackScore, defenseScore, floorHit, floorResist)

	defender.OnSkillUse(string(skills.Rhetoric), defenderUserId)
	defender.OnStatUse("willpower", defenderUserId)

	if !success {
		// Defensive mirror of a crit; see TrySpellDeflection for why
		// defRoll.Margin is used unnegated.
		if DefenseContestCrit(defRoll.Margin, defRoll) {
			return 0.0
		}
		return float64(cfg.RhetoricAvoidanceDamageMultiplier)
	}

	return 1.0
}
