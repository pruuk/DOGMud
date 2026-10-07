package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// A shot across a vertical exit used to arrive "from beyond the up" and leave
// "upward" only by the accident of "{exitname}ward" (#430), which also gave
// "gateward" for a named exit. The shipped shoot store is rendered for up and
// for north on both the departure and the known-origin arrival.
func observerRole(r narration.Roles) string      { return r.Observer }
func acteeObserverRole(r narration.Roles) string { return r.ActeeObserver }

func TestShootNarration_VerticalExitsReadAsDirections(t *testing.T) {
	if err := movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"); err != nil {
		t.Fatalf("load special-move narration: %v", err)
	}
	ids := moveIdentities{Actor: "Orc", ActorPlain: "Orc", Actee: "Bob", ActeePlain: "Bob"}
	cases := []struct {
		event movenarration.EventKey
		exit  string
		role  func(r narration.Roles) string
		want  string
	}{
		{"fire_depart", "up", observerRole, "Orc fires their bow upward."},
		{"fire_depart", "north", observerRole, `Orc fires their bow towards the <ansi fg="exit">north</ansi>.`},
		{"arrival_known_hit", "up", acteeObserverRole, "A shot streaks in from somewhere above and strikes Bob!"},
		{"arrival_known_hit", "down", acteeObserverRole, "A shot streaks in from somewhere below and strikes Bob!"},
		{"arrival_known_hit", "north", acteeObserverRole, `A shot streaks in from beyond the <ansi fg="exit">north</ansi> and strikes Bob!`},
	}
	for _, c := range cases {
		roles, ok := renderMoveEvent("shoot", c.event, ids, map[string]string{
			movenarration.TokenWeapon:   "bow",
			movenarration.TokenExitName: c.exit,
		})
		if !ok {
			t.Fatalf("%s: event not rendered", c.event)
		}
		if got := c.role(roles); got != c.want {
			t.Errorf("%s via %s: got %q, want %q", c.event, c.exit, got, c.want)
		}
	}
}
