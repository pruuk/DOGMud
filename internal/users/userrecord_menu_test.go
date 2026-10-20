package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

func testMenu() menu.Menu {
	return menu.Menu{Heading: "Pick", Entries: []menu.Entry{{Id: "a", Label: "Alpha"}, {Id: "b", Label: "Beta"}}, Leave: "Leave"}
}

type rec struct {
	chosen []string
	closed []menu.CloseReason
}

func (r *rec) session() menu.Session {
	return menu.Session{
		OnChoose: func(e menu.Entry) { r.chosen = append(r.chosen, e.Id) },
		OnClose:  func(c menu.CloseReason) { r.closed = append(r.closed, c) },
	}
}

// menuUser is a test user whose character has an Activity machine, so the
// busy rule can be exercised. Life and Control are nil, which the gate's
// predicates read as alive and not controlled; Position starts Standing.
func menuUser() *UserRecord {
	u := NewTestUser(1, "tester", "Mira", 0)
	u.Character.Activity = activity.NewMachine()
	return u
}

// startCrafting makes the character busy without a push: the fixture never
// calls WireBusyObservers, so no observer runs.
func startCrafting(t *testing.T, u *UserRecord) {
	t.Helper()
	if err := u.Character.Activity.Inner().TransitionTo(activity.Crafting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Crafting: %v", err)
	}
}

func TestOpenMenu_ChooseByNumberCallsOnChooseAndCloses(t *testing.T) {
	u, r := menuUser(), &rec{}
	if _, ok := u.OpenMenu(testMenu(), r.session()); !ok {
		t.Fatal("menu did not open")
	}
	if !u.RouteMenuInput("2", true) {
		t.Fatal("a number was not consumed")
	}
	if len(r.chosen) != 1 || r.chosen[0] != "b" || u.HasOpenMenu() {
		t.Fatalf("chosen %v, open %v", r.chosen, u.HasOpenMenu())
	}
	if len(r.closed) != 1 || r.closed[0] != menu.CloseChosen {
		t.Fatalf("closed %v", r.closed)
	}
}

func TestRouteMenuInput_QueuedLinesNeverAnswer(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if u.RouteMenuInput("1", false) {
		t.Fatal("a queued line answered the menu")
	}
	if !u.HasOpenMenu() || len(r.chosen) != 0 {
		t.Fatal("a queued line changed the menu")
	}
}

func TestRouteMenuInput_OtherCommandClosesAndPassesThrough(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if u.RouteMenuInput("north", true) {
		t.Fatal("north was consumed")
	}
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseOtherCommand {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestRouteMenuInput_OutOfRangeKeepsMenuOpen(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if !u.RouteMenuInput("9", true) || !u.HasOpenMenu() {
		t.Fatal("out of range should be consumed and keep the menu")
	}
}

func TestRouteMenuInput_EnterLeaves(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	if !u.RouteMenuInput("", true) || u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseLeft {
		t.Fatalf("enter: open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestRouteMenuInput_NoMenuPassesThrough(t *testing.T) {
	u := menuUser()
	if u.RouteMenuInput("1", true) {
		t.Fatal("a line was consumed with no menu open")
	}
}

func TestOpenMenu_RefusedWhileBusy(t *testing.T) {
	u, r := menuUser(), &rec{}
	startCrafting(t, u)
	reason, ok := u.OpenMenu(testMenu(), r.session())
	if ok || reason != menu.CloseBusy {
		t.Fatalf("ok %v reason %v", ok, reason)
	}
}

func TestOpenMenu_RefusedWhileAPromptIsOpen(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.StartPrompt("build", "")
	if _, ok := u.OpenMenu(testMenu(), r.session()); ok {
		t.Fatal("opened over a prompt")
	}
}

func TestOpenMenu_ReplacesTheOpenOne(t *testing.T) {
	u, first, second := menuUser(), &rec{}, &rec{}
	u.OpenMenu(testMenu(), first.session())
	u.OpenMenu(testMenu(), second.session())
	if len(first.closed) != 1 || first.closed[0] != menu.CloseReplaced {
		t.Fatalf("first closed %v", first.closed)
	}
	if !u.HasOpenMenu() || len(second.closed) != 0 {
		t.Fatalf("second menu: open %v closed %v", u.HasOpenMenu(), second.closed)
	}
}

func TestChoice_RecheckedAgainstTheGate(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	// Busy without a push (the observer is not wired on this fixture).
	startCrafting(t, u)
	u.RouteMenuInput("1", true)
	if len(r.chosen) != 0 || len(r.closed) != 1 || r.closed[0] != menu.CloseBusy {
		t.Fatalf("chosen %v closed %v", r.chosen, r.closed)
	}
}

func TestChoice_StillValidFalseClosesWithContextLost(t *testing.T) {
	u, r := menuUser(), &rec{}
	s := r.session()
	s.StillValid = func() bool { return false }
	u.OpenMenu(testMenu(), s)
	u.RouteMenuInput("1", true)
	if len(r.chosen) != 0 || len(r.closed) != 1 || r.closed[0] != menu.CloseContextLost {
		t.Fatalf("chosen %v closed %v", r.chosen, r.closed)
	}
}

// TestChoice_CallbacksRunWithoutTheSessionLock re-enters the user from both
// callbacks; a callback run under sessionMu would deadlock here.
func TestChoice_CallbacksRunWithoutTheSessionLock(t *testing.T) {
	u := menuUser()
	reopened := false
	s := menu.Session{
		OnClose: func(menu.CloseReason) { _ = u.MenuOrPromptOpen() },
		OnChoose: func(menu.Entry) {
			_, reopened = u.OpenMenu(testMenu(), menu.Session{})
		},
	}
	u.OpenMenu(testMenu(), s)
	u.RouteMenuInput("1", true)
	if !reopened || !u.HasOpenMenu() {
		t.Fatalf("OnChoose could not open a follow-up menu: reopened %v open %v", reopened, u.HasOpenMenu())
	}
}

func TestCheckMenuInterrupts_ClosesOnBusy(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	startCrafting(t, u)
	u.CheckMenuInterrupts()
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseBusy {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

// TestMenuGate_SeizedReadsControlledNotOffFeet: every grapple position is
// off the feet, so the controlled rule must come before the off-feet rule
// for a seized player to read "as you are seized".
func TestMenuGate_SeizedReadsControlledNotOffFeet(t *testing.T) {
	u := menuUser()
	u.Character.Control = control.NewMachine()
	if err := u.Character.Control.TransitionToControlled(state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Controlled: %v", err)
	}
	if err := u.Character.Position.Inner().TransitionTo(position.Sitting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition to Sitting: %v", err)
	}
	if u.Character.IsStanding() {
		t.Fatal("fixture still standing; the test would not reach the order question")
	}
	if r := menuGate(u.Character); r != menu.CloseControlled {
		t.Fatalf("gate %v, want CloseControlled", r)
	}
}

// TestMenuGate_NilControlIsNotControlled: players loaded from a save have no
// Control machine; the gate must not panic and must not refuse.
func TestMenuGate_NilControlIsNotControlled(t *testing.T) {
	u := menuUser()
	u.Character.Control = nil
	if r := menuGate(u.Character); r != menu.CloseNone {
		t.Fatalf("gate %v, want CloseNone", r)
	}
}

func TestCheckMenuInterrupts_NoRuleKeepsTheMenu(t *testing.T) {
	u, r := menuUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	u.CheckMenuInterrupts()
	if !u.HasOpenMenu() || len(r.closed) != 0 {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
}

func TestCheckMenuInterrupts_NothingOpenIsUntouched(t *testing.T) {
	u := signalUser()
	startCrafting(t, u)
	u.CheckMenuInterrupts()
	if u.MenuOrPromptOpen() {
		t.Fatal("a user with nothing open now has something open")
	}
	wantSignals(t, u)
}

func TestCheckMenuInterrupts_ClosesANonModalPromptButNotAModalOne(t *testing.T) {
	u := menuUser()
	p, _ := u.StartPrompt("guild", "")
	p.NonModal = true
	startCrafting(t, u)
	u.CheckMenuInterrupts()
	if u.GetPrompt() != nil {
		t.Fatal("non-modal prompt survived an interrupt")
	}
	u2 := menuUser()
	u2.StartPrompt("room edit", "")
	startCrafting(t, u2)
	u2.CheckMenuInterrupts()
	if u2.GetPrompt() == nil {
		t.Fatal("modal builder prompt was closed by a game interrupt")
	}
}

func TestCloseMenuForFollow_ReturnsTopicAndClosesSilently(t *testing.T) {
	u, r := menuUser(), &rec{}
	s := r.session()
	s.Topic = "talk with Kael"
	s.TopicNames = []string{"Kael"}
	u.OpenMenu(testMenu(), s)
	topic, names, had := u.CloseMenuForFollow()
	if !had || topic != "talk with Kael" || len(names) != 1 || names[0] != "Kael" {
		t.Fatalf("topic %q names %v had %v", topic, names, had)
	}
	if u.HasOpenMenu() || len(r.closed) != 1 || r.closed[0] != menu.CloseFollow {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), r.closed)
	}
	if _, _, had := u.CloseMenuForFollow(); had {
		t.Fatal("reported a menu with none open")
	}
}

func TestMenuOpenState_TracksMenusAndPrompts(t *testing.T) {
	u, r := menuUser(), &rec{}
	if u.MenuOrPromptOpen() {
		t.Fatal("open at start")
	}
	u.OpenMenu(testMenu(), r.session())
	if !u.MenuOrPromptOpen() {
		t.Fatal("menu not counted")
	}
	u.CloseMenu(menu.CloseLeft)
	u.StartPrompt("x", "")
	if !u.MenuOrPromptOpen() {
		t.Fatal("prompt not counted")
	}
	u.ClearPrompt()
	if u.MenuOrPromptOpen() {
		t.Fatal("still open after clear")
	}
}
