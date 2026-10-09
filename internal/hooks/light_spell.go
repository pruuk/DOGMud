package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// magnitudeSpellApplication reports how a spell's condition should be applied
// when it reads one of conditions.ScaledKinds from its magnitude: at a value
// and a duration scaled from the CASTER's primary stat and spellcasting skill
// (lighting plan 5a for light, 5c for nightvision and infra reach, 5d for
// darkness). Each kind has its own base + stat/D1 + skill/D2 trio
// (conditions.SpellScaledMagnitude, shared with the admin setcondition
// command); the duration comes from conditions.SpellScaledTriggers, where
// darkness reads its own trio and the other three share the light duration
// trio. Infra reach is capped at LightInfraReachCap and nightvision
// at the window shift cap there, so the record holds the value it acts at.
// ok is false for any other condition, which keeps its authored
// application. A light then trims to its HOLDER's eyes, who may not be the
// caster.
func magnitudeSpellApplication(spellData *spells.SpellData, caster *characters.Character, conditionId int) (magnitude float64, triggers int, ok bool) {
	if spellData == nil || caster == nil {
		return 0, 0, false
	}
	spec := conditions.GetConditionSpec(conditionId)
	if spec == nil {
		return 0, 0, false
	}
	kind, scaled := spec.ScaledKind()
	if !scaled {
		return 0, 0, false
	}
	stat := float64(spellData.CasterStatValue(caster.Stats))
	skill := float64(caster.GetSkillLevel(skills.Spellcasting))
	magnitude = conditions.SpellScaledMagnitude(kind, stat, skill)
	triggers = conditions.SpellScaledTriggers(kind, stat, skill)
	return magnitude, triggers, true
}

// shroudSpellApplication reports the magnitude Empathic Shroud's record 31
// carries: the CASTER's shroud score, the spell's stat plus Spellcasting rank
// x SkillWeight (#444, owner 2026-10-09). The holder hides at it in place of
// Dexterity plus Skullduggery (characters.HideBaseScore). The duration stays
// the authored one (triggers 0 on AddConditionMagnitude). ok is false for
// every other condition.
func shroudSpellApplication(spellData *spells.SpellData, caster *characters.Character, conditionId int) (magnitude float64, ok bool) {
	if conditionId != conditions.ConditionIdEmpathicShroud || spellData == nil || caster == nil {
		return 0, false
	}
	return characters.ShroudScore(spellData.CasterStatValue(caster.Stats), caster.GetSkillLevel(skills.Spellcasting)), true
}

// spellConditionTarget is what a spell condition lands on: a player record or
// a mob, both of which queue the narrating events.Condition.
type spellConditionTarget interface {
	AddCondition(conditionId int, source string)
	AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
	AddConditionTickScaled(conditionId int, scale float64, source string)
}

// applySpellCondition applies one of a spell's conditions to its target: a
// magnitude-scaled light or sight at the caster's scaled value and duration,
// a heal- or damage-over-time at the caster's spellTickScale (the apply hook
// computes the amount where it lands), anything else at its authored values.
// Every door queues events.Condition, so the holder reads the start notice
// either way.
func applySpellCondition(target spellConditionTarget, spellData *spells.SpellData, caster *characters.Character, conditionId int) {
	if mag, ok := shroudSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, 0, mag, "spell")
		return
	}
	if mag, trig, ok := magnitudeSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, trig, mag, "spell")
		return
	}
	if spec := conditions.GetConditionSpec(conditionId); spec != nil && spec.TickPool != "" {
		target.AddConditionTickScaled(conditionId, spellTickScale(caster), "spell")
		return
	}
	target.AddCondition(conditionId, "spell")
}
