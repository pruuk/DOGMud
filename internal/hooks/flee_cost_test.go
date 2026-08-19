package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Catches the round hook hardcoding includeSkill=true instead of carrying the
// command's short-payment decision into blocker resolution.
func TestHandlePlayerFlee_ShortCommandDoesNotProgressSkullduggery(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)

	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	room.Exits = nil // keep legacy/default resolution away from Look infrastructure
	blocker := mobs.GetInstance(100)
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.Stats.Dexterity.ValueAdj = 1
	u.Character.Skills = map[string]int{string(skills.Skullduggery): 100}
	blocker.Character.Stats.Dexterity.ValueAdj = 100
	blocker.Character.SetAggro(u.UserId, 0, characters.DefaultAttack)
	u.Character.SetAggro(0, blocker.InstanceId, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()

	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatalf("fixture did not enter Disengaging")
	}
	events.DrainQueuedMessagesForTest(u.UserId)
	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("short flee was not resolved")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 0 {
		t.Errorf("short flee progressed Skullduggery %d times, want 0", got)
	}
	blocked := false
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "blocks you from fleeing") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatal("short hook resolution did not produce the mob blocker outcome")
	}

	// The prior admission is consumed. A legacy sentinel with no new command
	// therefore defaults to full skill, and because the blocker is STILL
	// targeting the fleer an opposed roll happens, so the wrapper awards
	// exactly one skullduggery use.
	//
	// The blocker deliberately keeps its aggro here. An earlier version of this
	// test called blocker.Character.EndAggro() first, which left nothing to
	// contest -- and still expected an award, because the award used to fire
	// unconditionally inside ResolveFleeBlockers before any roll. It is now
	// gated on a contest having happened, so the fixture has to supply one.
	u.Character.Aggro = &characters.Aggro{Type: characters.Flee}
	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("legacy/default flee was not resolved")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 1 {
		t.Errorf("legacy resolution after consumed short attempt progressed skill %d times, want 1", got)
	}

	// And with nothing left targeting the fleer, a further legacy resolution
	// runs no contest and therefore must not award anything more.
	blocker.Character.EndAggro()
	u.Character.Aggro = &characters.Aggro{Type: characters.Flee}
	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("uncontested legacy flee was not resolved")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 1 {
		t.Errorf("uncontested flee progressed skill to %d, want it to stay at 1", got)
	}
}

// Catches consuming admission only after the grapple early return. Grapple can
// begin after the command enters Disengaging but before the asynchronous round.
func TestHandlePlayerFlee_GrappleCancellationConsumesAdmission(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	setCombatPositionParallel(u.Character, position.Clinch)

	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("grapple cancellation did not handle flee")
	}
	if _, admitted := usercommands.TakeFleeAdmission(u); admitted {
		t.Fatal("grapple cancellation left the short admission reusable")
	}
}

// Catches leaving admission behind on the no-exit failure path.
func TestHandlePlayerFlee_NoExitConsumesAdmission(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	room.Exits = nil
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}

	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("no-exit failure did not handle flee")
	}
	if _, admitted := usercommands.TakeFleeAdmission(u); admitted {
		t.Fatal("no-exit failure left the short admission reusable")
	}
}

// Catches interpreting an already-consumed phase admission as a fresh legacy
// flee. A reentrant handler must leave the first resolver's Disengaging state
// and progression untouched.
func TestHandlePlayerFlee_ReentrantConsumptionDoesNotResolveTwice(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	room.Exits = nil
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	includeSkill, admitted := usercommands.TakeFleeAdmission(u)
	if !admitted || includeSkill {
		t.Fatal("fixture did not consume a short admission")
	}

	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("reentrant flee was not recognized")
	}
	if !u.Character.IsDisengaging() {
		t.Fatal("reentrant consumer resolved the first in-flight attempt")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 0 {
		t.Fatalf("reentrant consumer progressed Skullduggery %d times, want 0", got)
	}
}

// Catches a target-death cascade canceling Disengaging before the next combat
// round. The paid attempt needs one terminal line at cancellation time because
// EndAggro can remove the player from handlePlayerFlee's later round path.
// The subsequent true legacy sentinel must keep its full-skill default and
// cannot leave the canceled short admission reusable.
func TestHandlePlayerFlee_TerminalCancellationRetractsAdmission(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	room.Exits = nil
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatal("fixture did not publish an admitted Disengaging flee")
	}
	events.DrainQueuedMessagesForTest(u.UserId)

	u.Character.CombatPhase.ForceIdle(state.TransitionReason{
		Trigger: combatphase.TriggerTargetDied,
		Actor:   state.ActorRef{UserId: u.UserId},
	})
	if u.Character.CombatPhase.State() != combatphase.Idle {
		t.Fatalf("terminal cancellation left state %v, want Idle", u.Character.CombatPhase.State())
	}
	terminalLines := 0
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "fight ends before you need to flee") {
			terminalLines++
		}
	}
	if terminalLines != 1 {
		t.Fatalf("target-death cancellation terminal lines = %d, want 1", terminalLines)
	}
	if handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("terminally canceled flee was resolved again")
	}

	// A subsequent legacy-only flee has no cost admission and therefore defaults
	// to full skill. Point the mob at the player so an opposed roll actually
	// happens: skullduggery practice is awarded by the wrapper for a contest
	// that took place, not for the act of typing flee. The fixture above only
	// ever set the PLAYER's aggro, which is not what ResolveFleeBlockers reads.
	mobs.GetInstance(100).Character.SetAggro(u.UserId, 0, characters.DefaultAttack)
	u.Character.Aggro = &characters.Aggro{Type: characters.Flee}
	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("legacy/default flee was not resolved")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 1 {
		t.Fatalf("legacy flee after terminal cancellation progressed skill %d times, want 1", got)
	}
	mobs.GetInstance(100).Character.EndAggro()
	if _, admitted := usercommands.TakeFleeAdmission(u); admitted {
		t.Fatal("terminal cancellation left the old short admission reusable")
	}
}
