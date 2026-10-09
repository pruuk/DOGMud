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
// Hidden. The same kind keeps the hide; a re-cast shroud keeps the stronger
// of the held and incoming scores, and the record carries it so a reload
// reads the same. A different kind: the stronger base score holds and the
// incumbent wins a tie. The loser's record is discarded, not cancelled, so no
// end line tells the room of a reveal that did not happen. refused reports
// that the incoming record was the one dropped.
//
// A record 9 that holds the hide is made permanent, as the Hidden cascade
// adds it (Awareness_Cascades.go): the event path adds 9 with isPermanent
// false (an admin setcondition, a mob's add_condition 9), and 9 has no
// triggercount, so it would otherwise be pruned next round, telling its end
// line, while the holder stays Hidden.
func (c *Character) settleHide(conditionId int, incoming awareness.HiddenData) (refused bool) {
	held, _ := c.Awareness.HiddenData()
	if held.Source == incoming.Source {
		if incoming.Source == awareness.HideShroud {
			if held.Score > incoming.Score {
				incoming.Score = held.Score
			}
			c.stampShroudRecordScore(incoming.Score)
			c.Awareness.SetHiddenData(incoming)
		} else {
			c.Conditions.AddCondition(conditionIdHidden, true)
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
		// The sneak is over; a stale flag would move the holder as a
		// sneaker once the shroud ends.
		c.SetMiscData(`sneaking`, nil)
	}
	c.Awareness.SetHiddenData(incoming)
	if incoming.Source == awareness.HideSneak {
		c.Conditions.AddCondition(conditionIdHidden, true)
	}
	return false
}

// stampShroudRecordScore writes score onto the held record 31 as its
// magnitude, so the record and the hide agree.
func (c *Character) stampShroudRecordScore(score float64) {
	for _, rec := range c.Conditions.GetConditions(conditions.ConditionIdEmpathicShroud) {
		rec.Magnitude = score
	}
}

// reconcileShroudHide keeps the shroud hide and record 31 in step. Validate
// calls it beside reconcilePerception.
//
// Ending: once no live record 31 remains (it ran out, the prune pass
// validates after pruning; or was removed, purged or cancelled) the hide is
// revealed. The reveal runs the Awareness cascade, which cancels any other
// hidden-flag record; with no record left that cascade's cancel is a no-op,
// so this cannot recurse.
//
// Reload: a loaded character gets a fresh, Visible Awareness machine while
// its saved records come back live. A Visible holder of a live 31 re-enters
// the shroud hide through hideForStealthRecord, the door record 31 lands
// through, at the record's score. A landing never reaches this branch
// Visible (the door hides first, or discards a refused 31), and every exit
// from Hidden cancels 31 with the other hidden-flag records, so a revealed
// hide does not come back here.
func (c *Character) reconcileShroudHide() {
	if c.Awareness == nil {
		return
	}
	if c.Awareness.State() == awareness.Visible && c.holdsLiveShroudRecord() {
		c.hideForStealthRecord(conditions.ConditionIdEmpathicShroud)
		return
	}
	if !c.HiddenByShroud() || c.holdsLiveShroudRecord() {
		return
	}
	_ = c.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerShroudEnded})
}

// DiscardEndedStealthRecords drops an ended (expired, not yet pruned) record
// 9 or 31 without its end line, and keeps a live one. It is for a SAVE
// BOUNDARY only: users.loadUserFromPath calls it before the load's
// Validate(true) (#451 review). Logout forces a hidden player visible, which
// cancels the record to expired, and the save happens before any prune, so
// the first prune after the next login told the room "emerges from the
// shadows" of a player who had just arrived. An expired record on a loaded
// character always ended before the save; its moment has passed.
//
// Never call it from Validate or the reconcile functions: an ordinary reveal
// in play leaves the same expired record, and the prune of it is what tells
// the room the hider emerged.
func (c *Character) DiscardEndedStealthRecords() {
	// A freshly loaded record list has no lookups yet, and Discard reads them.
	c.Conditions.Validate(true)
	for _, id := range []int{conditionIdHidden, conditions.ConditionIdEmpathicShroud} {
		for _, rec := range c.Conditions.List {
			if rec.ConditionId == id && rec.Expired() {
				c.Conditions.Discard(id)
				break
			}
		}
	}
}

// reconcileSneakHide is reconcileShroudHide's sibling for a sneak hide
// (#451). Validate calls it after reconcileShroudHide.
//
// Reload: a loaded character gets a fresh, Visible Awareness machine while
// its saved permanent record 9 comes back live, and the permanent rebuild
// (Validate(true)) found no source for a 9 on a Visible holder, so it
// expired and the prune pass told the room "emerges from the shadows". A
// Visible holder of a live 9 re-enters the sneak hide through
// hideForStealthRecord, the door record 9 lands through, so the rebuild
// that follows keeps the 9 (reapplyPermanentConditions sources a live 9 on
// a sneak-hidden holder) and the saved `sneaking` flag matches the hide.
// Every exit from Hidden cancels 9 with the other hidden-flag records
// (Awareness_Cascades.go), so a revealed hide does not come back here.
func (c *Character) reconcileSneakHide() {
	if c.Awareness == nil {
		return
	}
	if c.Awareness.State() == awareness.Visible && c.holdsLiveStealthRecord() {
		c.hideForStealthRecord(conditionIdHidden)
	}
}
