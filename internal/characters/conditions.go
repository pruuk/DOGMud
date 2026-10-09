package characters

import (
	"fmt"
	"math"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

func (c *Character) IsDisabled() bool {
	return c.Health <= 0
}

func (c *Character) HasConditionFlag(conditionFlag conditions.Flag) bool {
	return c.Conditions.HasFlag(conditionFlag, false)
}

// HasFlagFromAnySource returns true if the character has the given flag from
// either active conditions OR permanent mutation effects. Use this instead of
// HasConditionFlag when the check should also honor mutation-granted flags.
func (c *Character) HasFlagFromAnySource(conditionFlag conditions.Flag) bool {
	if c.Conditions.HasFlag(conditionFlag, false) {
		return true
	}
	return mutations.HasMutationFlag(c.Mutations, string(conditionFlag))
}

func (c *Character) CancelConditionsWithFlag(conditionFlag conditions.Flag) bool {
	heldStealth := c.holdsLiveStealthRecord()
	if c.Conditions.HasFlag(conditionFlag, true) {
		// A cancel that just ended the stealth record spends a spawn-time
		// hide (conditionids: [9]) for good, and it must do so BEFORE the
		// Validate(true) below: reapplyPermanentConditions would otherwise
		// revive record 9 from the permanent id. Revived during a reveal,
		// it sat live on a visible mob (a stray "emerges from the shadows"
		// later, a true mob_has_condition 9); revived on a direct cancel
		// (a mob's get, give, eat, show, aid) while the machine was still
		// Hidden, it kept the rescue below from ever driving the mob out
		// (sight gates playtest fixes, F5). Every path that ends stealth
		// reaches here: the direct cancels, and every Awareness exit from
		// Hidden through the cascade in Awareness_Cascades.go.
		if heldStealth && !c.holdsLiveStealthRecord() {
			c.RemovePermanentCondition(conditionIdHidden)
		}
		_ = c.Validate(true)
		// Hidden flag is special: the Awareness FSM mirrors the condition
		// via Awareness_Cascades.go. If a caller cancels the condition
		// directly (eat/drink/give/get/equip/spotted-on-entry/etc.)
		// without driving the FSM, IsHidden() stays true because the
		// FSM is still in Hidden — split source of truth. Drive the
		// FSM out here so every caller gets correct behavior without
		// having to know about the cascade. Re-entrancy: the cascade
		// re-fires CancelConditionsWithFlag(Hidden), but the condition is
		// already cancelled by the Validate above so HasFlag returns
		// false → no recursion.
		//
		// ⚠️ THE CONDITION IS ABOUT THE STATE, NOT THE ARGUMENT. It used
		// to read `conditionFlag == conditions.Hidden`, which asked "was Hidden the
		// flag you SELECTED by?" rather than "did stealth just end?".
		// Condition 9 Hidden carries BOTH `hidden` and `cancel-on-combat`, so
		// CancelCombatConditions -> CancelConditionsWithFlag(CancelIfCombat) really
		// did strip it while this rescue sat out, leaving the FSM in
		// Hidden and IsHidden() true. That is how a cross-room sniper
		// stayed hidden shot after shot (reported 2026-08-31): shoot.go's
		// own guard called CancelCombatConditions expecting stealth to drop,
		// and it silently did nothing.
		//
		// Asking the FSM directly is correct for every caller and cannot
		// drift again: if stealth is still on after a cancel, end it.
		if c.Awareness != nil && !c.Conditions.HasFlag(conditions.Hidden, false) &&
			c.Awareness.State() == awareness.Hidden {
			_ = c.Awareness.TransitionToRevealing(
				state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		}
		return true
	}
	return false
}

// CancelCombatConditions cancels all active conditions with the CancelIfCombat flag
// AND strips matching conditions from the permanentConditionIds list so they don't
// re-apply during Validate(). Call this when a character enters combat
// (as attacker or defender) or dies.
//
// Without the permanent condition strip, mobs seeded with CancelIfCombat-tagged
// conditions via `conditionids:` (e.g. Hidden on ambushers) would see the condition
// re-applied every Validate() call — the combat system would strip the
// active instance but the next Validate would put it right back. This
// surfaced as "(hidden)" tags persisting on ambushers mid-combat.
func (c *Character) CancelCombatConditions() {
	filtered := make([]int, 0, len(c.permanentConditionIds))
	for _, id := range c.permanentConditionIds {
		spec := conditions.GetConditionSpec(id)
		if spec == nil {
			filtered = append(filtered, id)
			continue
		}
		keep := true
		for _, f := range spec.Flags {
			if f == conditions.CancelIfCombat {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, id)
		}
	}
	c.permanentConditionIds = filtered

	c.CancelConditionsWithFlag(conditions.CancelIfCombat)
}

// RevealForCombat is what being attacked does to a defender: its
// combat-cancel conditions strip, and a hidden defender is forced visible.
// handleCombatRound calls it before a melee swing's lines; the special-move
// seams (combat.ExecuteSkillMove, combat.ExecuteGrappleMove) and the taunt
// call it before theirs, so no attack names a mob the room cannot see.
//
// The Awareness FSM is the canonical source of truth for IsHidden. Its
// cascade (Awareness_Cascades.go) fires only when the defender's OWN
// CombatPhase goes Idle to Engaging, which does not happen when a defender
// is targeted but never SetAggro's the attacker (e.g. a hidden mob grappled
// while it has no aggro of its own). So the FSM is forced out of Hidden here
// if the condition strip left it stale; the cascade re-strips the condition
// (a no-op, since it is already gone).
func (c *Character) RevealForCombat() {
	c.CancelCombatConditions()
	if c.Awareness != nil && c.IsHidden() {
		c.Awareness.ForceVisible(state.TransitionReason{
			Trigger: awareness.TriggerCombatEntered,
		})
	}
}

func (c *Character) HasCondition(conditionId int) bool {
	return c.Conditions.HasCondition(conditionId)
}

// RefreshCondition tops a held condition's triggers back up without resetting its
// round cadence or running Validate: a refresh changes no statmod or flag,
// so there is nothing for Validate to rebuild.
func (c *Character) RefreshCondition(conditionId int) bool {
	return c.Conditions.RefreshCondition(conditionId)
}

func (c *Character) AddCondition(conditionId int, isPermanent bool) error {
	conditionId = int(math.Abs(float64(conditionId)))
	if !c.Conditions.AddCondition(conditionId, isPermanent) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	c.hideForStealthRecord(conditionId)
	// Validate brings Perception in line with the blind sources.
	_ = c.Validate()
	return nil
}

// conditionIdHidden is the stealth record the Awareness machine mirrors
// (Awareness_Cascades.go): entering Hidden adds it, leaving Hidden cancels it.
const conditionIdHidden = 9

// hideForStealthRecord is the other half of that mirror. Record 9 added to a
// Visible character (a mob's spawn-time conditionids, an admin setcondition)
// drives Awareness into Hidden, so IsHidden agrees with the record. Before,
// only the sneak command entered Hidden, and an ambusher spawned with 9 was
// listed in the room and emoted (sight gates playtest fixes, F5). The
// cascade's own re-add arrives while the machine is already Hidden, and a
// sneak in flight is Concealing, so neither is driven twice.
//
// Only record 9: another hidden-flag record (Empathic Shroud, 31) is timed,
// and the cascade would pin a permanent 9 that outlives it.
//
// A refused hide (a busy character's Visible to Concealing is vetoed) takes
// the record back off, so no record 9 claims hidden on a character the
// machine kept Visible. A permanent 9 stays in the id list and tries again at
// the next Validate(true).
func (c *Character) hideForStealthRecord(conditionId int) {
	if conditionId != conditionIdHidden || c.Awareness == nil || c.Awareness.State() != awareness.Visible {
		return
	}
	reason := state.TransitionReason{Trigger: awareness.TriggerConditionApplied}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		c.Conditions.RemoveCondition(conditionIdHidden)
		return
	}
	c.Awareness.ResolveConcealment(true, reason)
}

// holdsLiveStealthRecord reports a live (unexpired) record 9. HasCondition
// cannot answer this: it also counts an expired record the prune pass has not
// yet removed.
func (c *Character) holdsLiveStealthRecord() bool {
	return len(c.Conditions.GetConditions(conditionIdHidden)) > 0
}

// AddConditionScaled adds a condition with its duration scaled by durationMult.
func (c *Character) AddConditionScaled(conditionId int, durationMult float64) error {
	conditionId = int(math.Abs(float64(conditionId)))
	if !c.Conditions.AddConditionScaled(conditionId, durationMult) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	c.hideForStealthRecord(conditionId)
	// Validate brings Perception in line with the blind sources.
	_ = c.Validate()
	return nil
}

// AddConditionMagnitude applies a record synchronously for an exact trigger count
// with a per-instance magnitude. It is what every former combat-condition site
// calls: those effects must be in place within the same round tick (a shout,
// a ward, a bleed) and their appliers narrate the moment themselves, so the
// record is silent-start or quiet and the event path's start notice is not
// wanted. The prune pass still narrates the end. triggers 0 means the spec's
// own triggercount. Every record that uses this door today ticks once a
// round, so the trigger count is the rounds; a stacking record takes it as the
// new stack's rounds. source overwrites the held record's Source on every
// call, so a stacking record carries its last applier's source.
func (c *Character) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) error {
	conditionId = int(math.Abs(float64(conditionId)))
	if !c.Conditions.AddConditionMagnitude(conditionId, triggers, magnitude) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	for _, b := range c.Conditions.GetConditions(conditionId) {
		b.Source = source
	}
	_ = c.Validate()
	return nil
}

func (c *Character) TrackConditionStarted(conditionId int) {
	c.Conditions.Started(conditionId)
}

func (c *Character) GetConditions(conditionId ...int) []*conditions.Condition {
	return c.Conditions.GetConditions(conditionId...)
}

func (c *Character) RemoveCondition(conditionId int) {
	conditionId = int(math.Abs(float64(conditionId)))
	c.Conditions.RemoveCondition(conditionId)
	// Validate brings Perception in line with the blind sources that remain.
	_ = c.Validate()
}

// Used with SpawnInfo to gift spawning mobs with permanent conditions.
//
// The list is copied: both spawn callers pass a slice they do not own (the
// mob template's ConditionIds through a shallow copy, a room's
// spawnInfo.ConditionIds), so an instance that edits its own list must not
// write through to the template every later spawn reads.
func (c *Character) SetPermanentConditions(conditionIds []int) {
	c.permanentConditionIds = append([]int(nil), conditionIds...)
}

// RemovePermanentCondition removes a condition ID from the permanent condition list so
// it won't be re-applied during Validate(). Use this when a permanent condition
// should be permanently lost (e.g., revealing a hidden mob).
//
// It builds a new list rather than shifting the old one in place, so even a
// list that still shares a template's backing array cannot rewrite the
// template (an in-place delete turned a Pale Lurker's [9, 85] into [85, 85]).
func (c *Character) RemovePermanentCondition(conditionId int) {
	if !slices.Contains(c.permanentConditionIds, conditionId) {
		return
	}
	kept := make([]int, 0, len(c.permanentConditionIds)-1)
	for _, id := range c.permanentConditionIds {
		if id != conditionId {
			kept = append(kept, id)
		}
	}
	c.permanentConditionIds = kept
}

// reapplyPermanentConditions refreshes item, species, pet and permanent
// conditions against what the character has NOW. It builds one map of
// condition id to "this condition still has a source": the permanent-id list,
// the species, the pet and every item still worn set true. Every permanent
// record already held is entered false unless something sourced it, so a
// permanent record nothing grants any more is removed. That covers a record
// whose item was taken off, and one whose item yaml changed under a save and
// no longer grants it. Every id with a source is (re)added.
func (c *Character) reapplyPermanentConditions() {

	hasSource := map[int]bool{}

	// Permanent conditions associated with certain mobs.
	for _, conditionId := range c.permanentConditionIds {
		hasSource[conditionId] = true
	}

	// Conditions that come from a species.
	if rInfo := species.GetSpecies(c.SpeciesId); rInfo != nil {
		for _, conditionId := range rInfo.ConditionIds {
			hasSource[conditionId] = true
		}
	}

	// Conditions that come from a pet.
	if c.Pet.Exists() {
		for _, conditionId := range c.Pet.GetConditions() {
			hasSource[conditionId] = true
		}
	}

	// Permanent records with no source so far default to false, so they are
	// removed unless a worn item below still grants them.
	for _, b := range c.Conditions.List {
		if b.Permanent {
			if _, ok := hasSource[b.ConditionId]; !ok {
				hasSource[b.ConditionId] = false
			}
		}
	}

	// Conditions granted by items still worn.
	for _, itm := range c.GetAllWornItems() {
		for _, conditionId := range itm.GetSpec().WornConditionIds {
			hasSource[conditionId] = true
		}
	}

	for conditionId, sourced := range hasSource {
		if sourced {
			_ = c.AddCondition(conditionId, true)
		} else {
			c.RemoveCondition(conditionId)
		}
	}
}
