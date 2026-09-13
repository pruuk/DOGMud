package perception

import "github.com/GoMudEngine/GoMud/internal/state"

// transitions enforces the Perception invariant matrix. Two states,
// two edges. Re-entry (Sighted→Sighted or Blinded→Blinded) is not in
// the table — callers should check current state before firing the
// transition to avoid ErrInvalidTransition.
var transitions = state.TransitionTable[State]{
	Sighted: {Blinded},
	Blinded: {Sighted},
}

// Trigger reason constants. Used in state.TransitionReason.Trigger.
//
// There used to be TriggerConditionAdded / TriggerConditionRemoved here for a
// third blind source, the ConditionBlinded combat condition. That condition
// never had a producer anywhere in the tree, and the conditions unification
// (slice 1, 2026-09-12) deleted the enum it lived in; the constants went with
// it. Blindness has exactly two sources, both buffs.
const (
	TriggerBuffApplied = "buff_applied"
	TriggerBuffExpired = "buff_expired"
)

// Blind-source buff IDs. Detected by ID rather than by flag because
// the existing buff YAMLs don't carry a "blinded" flag — they only
// have stat mods. Adding flags to the YAML would touch data; detecting
// by ID keeps chunk 6 dormant on the data side.
const (
	BuffIdBlinded            = 3  // _datafiles/world/dogmud/buffs/3-blinded.yaml
	BuffIdFlashbangBlindness = 77 // _datafiles/world/dogmud/buffs/77-flashbang_blindness.yaml
)
