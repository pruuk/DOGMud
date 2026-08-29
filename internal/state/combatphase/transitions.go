package combatphase

import "github.com/GoMudEngine/GoMud/internal/state"

// validTransitions enforces the Combat Phase invariant matrix.
// Vetoes layer additional rules on top.
var validTransitions = state.TransitionTable[State]{
	Idle: {Engaging},
	// Engaging on RETARGET during the wind-up (U12c-0b). U12c-0 added the
	// Engaged case and missed this one; both are "switching targets takes a
	// moment", one state apart.
	Engaging: {Engaged, Idle, Engaging}, // Idle on cancel/target-died
	// Engaging on RETARGET (U12c-0). Switching targets mid-fight is a fresh
	// engagement: new target, fresh wind-up, then back to Engaged. Without
	// this, TransitionToEngaging failed on every retarget and SetAggro
	// discarded the error, so CurrentTarget kept returning the PREVIOUS
	// enemy — which the {target} and {targethealth} prompt tokens render.
	Engaged:     {Disengaging, Idle, Engaging}, // Idle direct on death/despawn
	Disengaging: {Idle, Engaged},               // Engaged on flee failure
}

// Trigger reason constants. Use these in TransitionReason.Trigger
// to ensure stable strings across the codebase.
const (
	TriggerAttackCommand   = "attack_command"
	TriggerSurpriseAttack  = "surprise_attack"
	TriggerEngagementReady = "engagement_ready"
	TriggerFleeCommand     = "flee_command"
	TriggerFleeSuccess     = "flee_success"
	TriggerFleeFailure     = "flee_failure"
	TriggerTargetDied      = "target_died"
	TriggerSelfDied        = "self_died"
	TriggerTargetOutOfRoom = "target_out_of_room"
	TriggerCharm           = "charm_acquired"
	TriggerCombatantToggle = "combatant_toggle"
	TriggerForceIdle       = "force_idle"
)
