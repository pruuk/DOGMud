package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
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
	QueueCondition(evt events.Condition)
}

// applySpellCondition applies one of a spell's conditions to its target: a
// magnitude-scaled light or sight at the caster's scaled value and duration,
// a heal- or damage-over-time at the caster's spellTickScale (the apply hook
// computes the amount where it lands), anything else at its authored values.
// The event names the caster (casterRef) and carries the crit marker for the
// caster's start line (messaging M6 slice 1), so the holder reads the start
// lines with the caster's name, and a tick that kills credits the caster.
func applySpellCondition(target spellConditionTarget, spellData *spells.SpellData, caster *characters.Character, conditionId int,
	casterRef state.ActorRef, crit bool) {
	evt := events.Condition{ConditionId: conditionId, Source: "spell", Caster: casterRef, CasterCrit: crit}
	if mag, ok := shroudSpellApplication(spellData, caster, conditionId); ok {
		evt.Magnitude = mag
	} else if mag, trig, ok := magnitudeSpellApplication(spellData, caster, conditionId); ok {
		evt.Magnitude, evt.Triggers = mag, trig
	} else if spec := conditions.GetConditionSpec(conditionId); spec != nil && spec.TickPool != "" {
		evt.TickScale = spellTickScale(caster)
	}
	target.QueueCondition(evt)
}
