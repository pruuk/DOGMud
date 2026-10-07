package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #428 (and the ground-listing half of #276): the "On the Ground" corpses
// follow the viewer's sight like the roster does. A viewer who makes out
// shapes only reads one "corpse of a figure" per corpse, the hidden form
// look's corpse observer line already writes, uncolored by kind so a mob's
// corpse and a player's read alike; a viewer who sees nothing lists none;
// clear sight still names them.
func detailsCorpseRoom(t *testing.T) (*Room, *users.UserRecord) {
	t.Helper()
	r, viewer := detailsSightRoom(t, "cave")
	orig := r.Corpses
	t.Cleanup(func() { r.Corpses = orig })
	beggar := Corpse{MobId: 12}
	beggar.Character.Name = "City Beggar"
	beggar2 := beggar
	deadric := Corpse{UserId: 777}
	deadric.Character.Name = "Deadric"
	r.Corpses = []Corpse{beggar, beggar2, deadric}
	return r, viewer
}

func TestGetDetails_ShapesViewerReadsCorpsesOfFigures(t *testing.T) {
	r, viewer := detailsCorpseRoom(t)
	if !viewer.Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
		t.Fatal("precondition: the viewer should now carry infrared")
	}
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightShapes {
		t.Fatalf("precondition: viewer sight = %v, want SightShapes", got)
	}
	corpses := GetDetails(r, viewer).VisibleCorpses
	want := messaging.HideNames(`corpse of Deadric`, []string{`Deadric`}, messaging.SightShapes)
	if len(corpses) != 3 {
		t.Fatalf("shapes corpses %q, want 3 entries", corpses)
	}
	for _, c := range corpses {
		if c != want {
			t.Errorf("shapes corpse entry %q, want exactly %q", c, want)
		}
	}
	joined := strings.Join(corpses, " ")
	for _, leak := range []string{"Beggar", "Deadric", "user-corpse", "mob-corpse", "2 "} {
		if strings.Contains(joined, leak) {
			t.Errorf("shapes corpses leak %q: %q", leak, joined)
		}
	}
}

func TestGetDetails_BlindViewerListsNoCorpses(t *testing.T) {
	r, viewer := detailsCorpseRoom(t)
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightNone {
		t.Fatalf("precondition: viewer sight = %v, want SightNone", got)
	}
	if corpses := GetDetails(r, viewer).VisibleCorpses; len(corpses) != 0 {
		t.Fatalf("a viewer who sees nothing read the corpses %q", corpses)
	}
}

func TestGetDetails_ClearViewerReadsCorpseNames(t *testing.T) {
	r, viewer := detailsCorpseRoom(t)
	lamp := 90
	r.Lamp = &lamp
	joined := strings.Join(GetDetails(r, viewer).VisibleCorpses, " ")
	for _, want := range []string{`<ansi fg="mob-corpse">2 City Beggar corpses</ansi>`, `<ansi fg="user-corpse">Deadric corpse</ansi>`} {
		if !strings.Contains(joined, want) {
			t.Errorf("clear corpses %q are missing %q", joined, want)
		}
	}
}
