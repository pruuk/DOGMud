package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// #375 phase 0, Task 15 review: a player may open a non-modal prompt while
// already sitting or fighting (mail while resting, picklock in a fight). It
// closes only on a NEW interrupt, not on any push while the same rule holds.

func sitDown(t *testing.T, u *UserRecord) {
	t.Helper()
	if err := u.Character.Position.Inner().TransitionTo(position.Sitting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Sitting: %v", err)
	}
}

func standUp(t *testing.T, u *UserRecord) {
	t.Helper()
	if err := u.Character.Position.Inner().TransitionTo(position.Standing, state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Standing: %v", err)
	}
}

func startCombat(t *testing.T, u *UserRecord) {
	t.Helper()
	if err := u.Character.CombatPhase.TransitionToEngaging(combatphase.EngagingData{Target: state.ActorRef{MobInstanceId: 7}}, state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Engaging: %v", err)
	}
	if !u.Character.IsInCombat() {
		t.Fatal("fixture not in combat")
	}
}

func nonModalPrompt(u *UserRecord) {
	p, _ := u.StartPrompt("mail", "Bob")
	p.NonModal = true
	p.SetAsideLine = "You set your unsent mail aside."
	p.AskConfirm("Send?", "Yes, send it", "No")
}

func TestNonModalPrompt_OpenedSittingSurvivesAPushWhileStillSitting(t *testing.T) {
	u := menuUser()
	sitDown(t, u)
	nonModalPrompt(u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() == nil {
		t.Fatal("a prompt opened while sitting closed on an unrelated push")
	}
}

func TestNonModalPrompt_OpenedSittingClosesWhenCombatStarts(t *testing.T) {
	u := menuUser()
	sitDown(t, u)
	nonModalPrompt(u)
	events.DrainQueuedMessagesForTest(u.UserId)
	startCombat(t, u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() != nil {
		t.Fatal("combat is a new interrupt and must close the prompt")
	}
	out := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	if !strings.Contains(out, "You set your unsent mail aside.") {
		t.Fatalf("interrupt close did not speak the set-aside line: %q", out)
	}
}

func TestNonModalPrompt_OpenedStandingClosesOnSitting(t *testing.T) {
	u := menuUser()
	nonModalPrompt(u)
	sitDown(t, u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() != nil {
		t.Fatal("sitting is a new interrupt for a prompt opened standing")
	}
}

func TestNonModalPrompt_StandAndSitAgainIsANewInterrupt(t *testing.T) {
	u := menuUser()
	sitDown(t, u)
	nonModalPrompt(u)
	standUp(t, u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() == nil {
		t.Fatal("standing up is not an interrupt")
	}
	sitDown(t, u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() != nil {
		t.Fatal("sitting down again after standing is a new interrupt")
	}
}

func TestNonModalPrompt_RoomChangeAlwaysCloses(t *testing.T) {
	u := menuUser()
	sitDown(t, u)
	nonModalPrompt(u)
	u.CloseMenusForRoomChange()
	if u.GetPrompt() != nil {
		t.Fatal("a room change must close a non-modal prompt")
	}
}

func TestRoutePromptInput_NonAnswerSpeaksTheSetAsideLineFirst(t *testing.T) {
	u := menuUser()
	nonModalPrompt(u)
	events.DrainQueuedMessagesForTest(u.UserId)
	next, had := u.RoutePromptInput("north", true)
	if next != "north" || !had || u.GetPrompt() != nil {
		t.Fatalf("next %q had %v prompt %v", next, had, u.GetPrompt())
	}
	out := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	if !strings.Contains(out, "You set your unsent mail aside.") {
		t.Fatalf("set-aside line missing: %q", out)
	}
}

func TestRoutePromptInput_NoSetAsideLineSaysNothing(t *testing.T) {
	u := menuUser()
	p, _ := u.StartPrompt("guild", "disband")
	p.NonModal = true
	p.AskMenu(menu.Confirm("Disband?", "Yes, disband it", "No, keep it"))
	events.DrainQueuedMessagesForTest(u.UserId)
	u.RoutePromptInput("north", true)
	if out := events.DrainQueuedMessagesForTest(u.UserId); len(out) != 0 {
		t.Fatalf("expected silence, got %q", out)
	}
}
