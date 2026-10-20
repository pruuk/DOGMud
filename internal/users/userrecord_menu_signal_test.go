package users

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// signals drains the MenuChanged events queued for u and returns their Open
// values in queue order.
func signals(u *UserRecord) []bool {
	out := []bool{}
	for _, m := range events.DrainQueuedMenuChangedForTest(u.UserId) {
		out = append(out, m.Open)
	}
	return out
}

// signalUser is a menuUser with the event queue cleared of its leftovers.
func signalUser() *UserRecord {
	u := menuUser()
	events.DrainQueuedMenuChangedForTest(u.UserId)
	return u
}

func wantSignals(t *testing.T, u *UserRecord, want ...bool) {
	t.Helper()
	if want == nil {
		want = []bool{}
	}
	if got := signals(u); !reflect.DeepEqual(got, want) {
		t.Fatalf("MenuChanged Open values %v, want %v", got, want)
	}
}

func TestMenuChanged_OpenThenClose(t *testing.T) {
	u := signalUser()
	u.OpenMenu(testMenu(), menu.Session{})
	u.CloseMenu(menu.CloseLeft)
	wantSignals(t, u, true, false)
}

func TestMenuChanged_OpenThenReplace(t *testing.T) {
	u := signalUser()
	u.OpenMenu(testMenu(), menu.Session{})
	u.OpenMenu(testMenu(), menu.Session{})
	wantSignals(t, u, true)
}

func TestMenuChanged_PromptThenClear(t *testing.T) {
	u := signalUser()
	u.StartPrompt("x", "")
	u.ClearPrompt()
	wantSignals(t, u, true, false)
}

func TestMenuChanged_ClearWithNothingOpen(t *testing.T) {
	u := signalUser()
	u.ClearPrompt()
	wantSignals(t, u)
}

func TestMenuChanged_ChoiceThatOpensTheNextMenuStaysOpen(t *testing.T) {
	u := signalUser()
	var reopened bool
	u.OpenMenu(testMenu(), menu.Session{OnChoose: func(menu.Entry) {
		_, reopened = u.OpenMenu(testMenu(), menu.Session{})
	}})
	wantSignals(t, u, true)
	u.RouteMenuInput("1", true)
	if !reopened || !u.HasOpenMenu() {
		t.Fatalf("follow-up menu: reopened %v open %v", reopened, u.HasOpenMenu())
	}
	wantSignals(t, u)
}

func TestMenuChanged_ChoiceThatOpensNothingCloses(t *testing.T) {
	u := signalUser()
	u.OpenMenu(testMenu(), menu.Session{})
	u.RouteMenuInput("1", true)
	wantSignals(t, u, true, false)
}

// One interrupt that closes a game menu and a non-modal prompt together says
// "closed" once. The two cannot be open together through the public API, so
// the fixture sets the menu directly.
func TestMenuChanged_OneInterruptClosesMenuAndPromptOnce(t *testing.T) {
	u := signalUser()
	p, _ := u.StartPrompt("guild", "")
	p.NonModal = true
	r := &rec{}
	u.sessionMu.Lock()
	u.activeMenu = &openMenu{m: testMenu(), s: r.session()}
	u.sessionMu.Unlock()
	wantSignals(t, u, true)

	startCrafting(t, u)
	u.CheckMenuInterrupts()
	if u.HasOpenMenu() || u.GetPrompt() != nil {
		t.Fatalf("open menu %v prompt %v", u.HasOpenMenu(), u.GetPrompt() != nil)
	}
	if len(r.closed) != 1 || r.closed[0] != menu.CloseBusy {
		t.Fatalf("closed %v", r.closed)
	}
	wantSignals(t, u, false)
}

func TestStartPrompt_ReplacesAnOpenGameMenu(t *testing.T) {
	u, r := signalUser(), &rec{}
	u.OpenMenu(testMenu(), r.session())
	u.StartPrompt("guild", "")
	if u.HasOpenMenu() || u.GetPrompt() == nil {
		t.Fatalf("open menu %v prompt %v", u.HasOpenMenu(), u.GetPrompt() != nil)
	}
	if len(r.closed) != 1 || r.closed[0] != menu.CloseReplaced {
		t.Fatalf("closed %v", r.closed)
	}
	wantSignals(t, u, true)
}

// TestStartPrompt_ReplacedMenuOnCloseRunsWithoutTheLock: an OnClose that
// reads the user would deadlock if it ran under sessionMu.
func TestStartPrompt_ReplacedMenuOnCloseRunsWithoutTheLock(t *testing.T) {
	u := signalUser()
	sawPrompt := false
	u.OpenMenu(testMenu(), menu.Session{OnClose: func(menu.CloseReason) {
		sawPrompt = u.GetPrompt() != nil
	}})
	u.StartPrompt("guild", "")
	if !sawPrompt {
		t.Fatal("OnClose ran before the prompt was in place")
	}
}

func TestMenuGate_Rules(t *testing.T) {
	const quitId, sleepId = 0, 15
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		quitId:  {ConditionId: quitId, Name: "Meditating", TriggerCount: 10},
		sleepId: {ConditionId: sleepId, Name: "Sleeping", TriggerCount: 10, Flags: []conditions.Flag{conditions.Sleeping}},
	})
	defer cleanup()

	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, u *UserRecord)
		want  menu.CloseReason
	}{
		{"none", func(*testing.T, *UserRecord) {}, menu.CloseNone},
		{"dead", func(t *testing.T, u *UserRecord) {
			u.Character.Life = life.NewMachine()
			must(t, u.Character.Life.Inner().TransitionTo(life.Dead, state.TransitionReason{}))
		}, menu.CloseDead},
		{"combat", func(t *testing.T, u *UserRecord) {
			u.Character.CombatPhase = combatphase.NewMachine()
			must(t, u.Character.CombatPhase.Inner().TransitionTo(combatphase.Engaging, state.TransitionReason{}))
		}, menu.CloseCombat},
		{"busy", func(t *testing.T, u *UserRecord) { startCrafting(t, u) }, menu.CloseBusy},
		{"asleep", func(t *testing.T, u *UserRecord) {
			if !u.Character.Conditions.AddCondition(sleepId, false) {
				t.Fatal("fixture: sleep condition not added")
			}
		}, menu.CloseAsleep},
		{"quitting", func(t *testing.T, u *UserRecord) {
			if !u.Character.Conditions.AddCondition(quitId, false) {
				t.Fatal("fixture: quit condition not added")
			}
		}, menu.CloseQuitting},
		{"sitting is not a gate", func(t *testing.T, u *UserRecord) {
			must(t, u.Character.Position.Inner().TransitionTo(position.Sitting, state.TransitionReason{}))
		}, menu.CloseNone},
		{"prone is not a gate", func(t *testing.T, u *UserRecord) {
			must(t, u.Character.Position.Inner().TransitionTo(position.Prone, state.TransitionReason{}))
		}, menu.CloseNone},
		{"controlled", func(t *testing.T, u *UserRecord) {
			u.Character.Control = control.NewMachine()
			must(t, u.Character.Control.TransitionToControlled(state.TransitionReason{}))
		}, menu.CloseControlled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := menuUser()
			c.setup(t, u)
			if got := menuGate(u.Character); got != c.want {
				t.Fatalf("gate %v, want %v", got, c.want)
			}
		})
	}
}
