package characters

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Empathic Shroud is a real hide (#444, owner ruling 2026-10-09): "a real
// hide/sneak with the spellcasting rank and stat replacing dex +
// skullduggery for the future opposed rolls." Only one hide holds at a time,
// the stronger: "Only one or the other should exist (the strongest)."
//
// A shroud hide carries record 31 and no record 9 (Awareness_Cascades.go
// skips the 9 for it), so its end is told once, by 31's own end line.

// shroudSpellId is the spell whose stat a shroud with no magnitude (an admin
// setcondition) reads for the holder's own score.
const shroudSpellId = "empathic-shroud"

// ShroudScore is an Empathic Shroud hide score: the spell's stat plus
// Spellcasting rank x SkillWeight. The spell hook stamps the CASTER's score
// on the record as its magnitude (hooks.applySpellCondition).
func ShroudScore(stat, spellcastingRank int) float64 {
	return float64(stat) + float64(spellcastingRank)*float64(configs.GetBalanceConfig().SkillWeight)
}

// OwnShroudScore is the score c would cast the shroud at: the spell's stat
// (willpower as shipped; willpower too if the spell is not loaded) plus c's
// Spellcasting rank x SkillWeight.
func (c *Character) OwnShroudScore() float64 {
	stat := c.Stats.Willpower.ValueAdj
	if sd := spells.GetSpell(shroudSpellId); sd != nil {
		stat = sd.CasterStatValue(c.Stats)
	}
	return ShroudScore(stat, c.GetSkillLevel(skills.Spellcasting))
}

// SneakBaseScore is the base of a sneak hide's score: Dexterity plus
// Skullduggery rank x SkillWeight. Mutation stealth and the light modifiers
// apply on top in actions.CalcSneakScore, to a sneak and a shroud alike, so
// the stronger-hide comparison leaves them out.
func (c *Character) SneakBaseScore() float64 {
	return float64(c.Stats.Dexterity.ValueAdj) +
		float64(c.GetSkillLevel(skills.Skullduggery))*float64(configs.GetBalanceConfig().SkillWeight)
}

// HideBaseScore is the base score c hides at in every opposed roll
// (actions.CalcSneakScore): the shroud's score while the shroud hides c,
// SneakBaseScore otherwise.
func (c *Character) HideBaseScore() float64 {
	if d, ok := c.hiddenData(); ok && d.Source == awareness.HideShroud {
		return d.Score
	}
	return c.SneakBaseScore()
}

// HiddenByShroud reports whether Empathic Shroud, not a sneak, is what keeps
// c hidden.
func (c *Character) HiddenByShroud() bool {
	d, ok := c.hiddenData()
	return ok && d.Source == awareness.HideShroud
}

// SneakOverShroud turns a shroud hide into a sneak hide with no reveal and no
// roll (the holder is already hidden): record 31 is discarded without its end
// line, the hide becomes a sneak, and record 9 carries it from here. The
// caller decides the sneak is the stronger and charges the sneak's cost
// (actions.Sneak). Reports whether there was a shroud hide to take over.
func (c *Character) SneakOverShroud() bool {
	if !c.HiddenByShroud() {
		return false
	}
	c.Conditions.Discard(conditions.ConditionIdEmpathicShroud)
	c.Awareness.SetHiddenData(awareness.HiddenData{Source: awareness.HideSneak})
	_ = c.AddCondition(conditionIdHidden, true)
	return true
}

func (c *Character) hiddenData() (awareness.HiddenData, bool) {
	if c.Awareness == nil {
		return awareness.HiddenData{}, false
	}
	return c.Awareness.HiddenData()
}

// holdsLiveShroudRecord reports a live (unexpired) record 31. HasCondition
// cannot answer this: it also counts an expired record the prune pass has not
// yet removed.
func (c *Character) holdsLiveShroudRecord() bool {
	return len(c.Conditions.GetConditions(conditions.ConditionIdEmpathicShroud)) > 0
}

// shroudRecordScore is the score the held record 31 hides at: its magnitude
// (the caster's score, stamped by the spell), or the holder's own score for a
// record with none.
func (c *Character) shroudRecordScore() float64 {
	for _, rec := range c.Conditions.GetConditions(conditions.ConditionIdEmpathicShroud) {
		if rec.Magnitude > 0 {
			return rec.Magnitude
		}
	}
	return c.OwnShroudScore()
}

// hideScoreOf is the base score a hide holds c at.
func (c *Character) hideScoreOf(d awareness.HiddenData) float64 {
	if d.Source == awareness.HideShroud {
		return d.Score
	}
	return c.SneakBaseScore()
}

// settleHide resolves a stealth record landing on a character already
// Hidden. The same kind keeps the hide (a re-cast shroud takes the new
// record's score). A different kind: the stronger base score holds and the
// incumbent wins a tie. The loser's record is discarded, not cancelled, so no
// end line tells the room of a reveal that did not happen. refused reports
// that the incoming record was the one dropped.
func (c *Character) settleHide(conditionId int, incoming awareness.HiddenData) (refused bool) {
	held, _ := c.Awareness.HiddenData()
	if held.Source == incoming.Source {
		if incoming.Source == awareness.HideShroud {
			c.Awareness.SetHiddenData(incoming)
		}
		return false
	}
	if c.hideScoreOf(incoming) <= c.hideScoreOf(held) {
		c.Conditions.Discard(conditionId)
		return true
	}
	if held.Source == awareness.HideShroud {
		c.Conditions.Discard(conditions.ConditionIdEmpathicShroud)
	} else {
		c.Conditions.Discard(conditionIdHidden)
		c.RemovePermanentCondition(conditionIdHidden)
	}
	c.Awareness.SetHiddenData(incoming)
	return false
}

// reconcileShroudHide ends a shroud hide once no live record 31 remains: it
// ran out (the prune pass validates after pruning), was removed, purged or
// cancelled. Validate calls it beside reconcilePerception. The reveal runs
// the Awareness cascade, which cancels any other hidden-flag record; with no
// record left that cascade's cancel is a no-op, so this cannot recurse.
func (c *Character) reconcileShroudHide() {
	if !c.HiddenByShroud() || c.holdsLiveShroudRecord() {
		return
	}
	_ = c.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerShroudEnded})
}
