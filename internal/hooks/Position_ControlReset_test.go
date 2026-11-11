package hooks_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	_ "github.com/GoMudEngine/GoMud/internal/hooks" // wire init() observers
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// heldInMount puts a fresh character in Mount as the held side, with its
// Control machine at Controlled, the state the drift roll leaves a player
// in after losing grapple rounds to a mob.
func heldInMount(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	partner := position.GrappleData{Partner: state.ActorRef{MobInstanceId: 900}}
	if err := c.Position.TransitionToClinch(partner, state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err != nil {
		t.Fatalf("setup clinch: %v", err)
	}
	if err := c.Position.TransitionToMount(partner, state.TransitionReason{Trigger: position.TriggerTakedownMount}); err != nil {
		t.Fatalf("setup mount: %v", err)
	}
	_ = c.Control.TransitionToControlled(state.TransitionReason{Trigger: control.TriggerDriftLoss})
	if !c.IsBeingControlled() {
		t.Fatal("setup: expected Controlled")
	}
	return c
}

// A grapple that ends on one side only (a solo stand, the consistency
// check's force-break, a death fallback) used to leave Control at
// Controlled, so the player read as "held" (no talking, menus closed, mobs
// preferring them as a target) long after the fight. Leaving the grapple
// family by any route must reset Control to Neutral (owner bug report
// 2026-11-11: grappled by a bandit scout, which later fled and was killed).
func TestControlResetsWhenGrappleEndsSolo(t *testing.T) {
	c := heldInMount(t)
	if err := c.Position.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerStandCommand}); err != nil {
		t.Fatalf("stand: %v", err)
	}
	if got := c.Control.State(); got != control.Neutral {
		t.Fatalf("Control after leaving the grapple = %v, want Neutral", got)
	}
	if c.IsBeingControlled() {
		t.Fatal("still reads as held after the grapple ended")
	}
}

// A mob that mounts a player is in control and the player is held, and the
// break frees both. Before Validate built Control for mob instances, the
// mob's side was nil and TransitionPair set neither side (bug, owner
// 2026-11-11).
func TestMobMountingPlayerSetsControlBothWays(t *testing.T) {
	mob := characters.New()
	mob.ResetForMobInstance() // the mob-instance path: machines cleared, then Validate
	mob.MobInstanceId = 901
	mob.Validate()
	player := characters.New()
	player.SetUserId(902)

	enter := state.TransitionReason{Trigger: position.TriggerGrappleEntry}
	if err := position.TransitionPair(mob, player, position.Clinch, enter); err != nil {
		t.Fatalf("clinch: %v", err)
	}
	if err := position.TransitionPair(mob, player, position.Mount, state.TransitionReason{Trigger: position.TriggerTakedownMount}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	if mob.Control == nil || !mob.IsController() {
		t.Fatalf("mob is not in control after mounting the player (Control nil = %v)", mob.Control == nil)
	}
	if !player.IsBeingControlled() {
		t.Fatalf("player Control = %v, want Controlled", player.Control.State())
	}

	if err := position.TransitionPair(mob, player, position.Standing, state.TransitionReason{Trigger: position.TriggerGrappleBreak}); err != nil {
		t.Fatalf("break: %v", err)
	}
	if mob.Control.State() != control.Neutral || player.Control.State() != control.Neutral {
		t.Fatalf("after break: mob %v, player %v, want both Neutral", mob.Control.State(), player.Control.State())
	}
}

// Moving between grapple positions keeps the dominance the drift roll built.
func TestControlKeptBetweenGrapplePositions(t *testing.T) {
	c := heldInMount(t)
	partner := position.GrappleData{Partner: state.ActorRef{MobInstanceId: 900}}
	if err := c.Position.TransitionToSideControl(partner, state.TransitionReason{Trigger: position.TriggerPositionAdvance}); err != nil {
		t.Fatalf("Mount to SideControl: %v", err)
	}
	if got := c.Control.State(); got != control.Controlled {
		t.Fatalf("Control changed on a grapple-to-grapple move: %v, want Controlled", got)
	}
}
