package combatphase

import "github.com/GoMudEngine/GoMud/internal/state"

// validTransitions enforces the Combat Phase invariant matrix.
// Vetoes layer additional rules on top.
var validTransitions = state.TransitionTable[State]{
	Idle: {Engaging},
	// Engaging on RETARGET during the wind-up (U12c-0b). U12c-0 added the
	// Engaged case and missed this one; both are "switching targets takes a
	// moment", one state apart.
	// Disengaging is a REGRESSION FIX, and the regression was one day old.
	//
	// The original table (2026-05-13) had Engaging: {Engaged, Idle}, and that
	// was harmless because nothing could park an actor in Engaging: it was a
	// brief wind-up you always left. Then U12c-0 and U12c-0b made a RETARGET
	// land, and a retarget goes Engaged -> Engaging. Anything that retargets
	// you every round -- reciprocal aggro, a companion taunting your target
	// away, your own `target` command -- now holds you in Engaging
	// indefinitely, and without this edge `flee` is refused EVERY round with a
	// generic "You can't break away just yet."
	//
	// Verified in play: refused across ~15 consecutive rounds of a live fight
	// while standing and un-grappled (U12c-2 adversarial playtest, run
	// 1dba21d1b7994159), and reproduced in one test below.
	//
	// Fleeing during a wind-up is also just correct: you have not thrown the
	// blow yet. A failed flee returns you to Engaged via the Disengaging row,
	// which is where a retarget would have landed you anyway.
	Engaging: {Engaged, Idle, Engaging, Disengaging}, // Idle on cancel/target-died
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
