package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// #435: every corpse line hides the dead one's name below clear sight (#428),
// so name matching must too. A viewer who cannot see clearly reaches a corpse
// by the word "corpse" alone, newest first and countable ("2.corpse"); a name
// that would confirm whose it is matches nothing. Clear sight matches by name
// as before. Fixture (detailsCorpseRoom): City Beggar, City Beggar, Deadric,
// oldest to newest, in a dark cave.
func TestFindCorpse_BelowClearSightMatchesOnlyTheWordCorpse(t *testing.T) {
	for _, c := range []struct {
		name  string
		infra bool
		want  messaging.SightDecision
	}{
		{"sees nothing", false, messaging.SightNone},
		{"shapes only", true, messaging.SightShapes},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, viewer := detailsCorpseRoom(t)
			if c.infra && !viewer.Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
				t.Fatal("precondition: the viewer should now carry infrared")
			}
			if got := messaging.ParticipantSight(viewer.Character, r); got != c.want {
				t.Fatalf("precondition: viewer sight = %v, want %v", got, c.want)
			}
			for _, named := range []string{"deadric corpse", "deadric", "city beggar corpse", "beggar"} {
				if idx := r.FindCorpseIndex(named, viewer.Character); idx != -1 {
					t.Errorf("FindCorpseIndex(%q) = %d at %v, want -1: the name confirms whose corpse it is", named, idx, c.want)
				}
				if _, ok := r.FindCorpse(named, viewer.Character); ok {
					t.Errorf("FindCorpse(%q) matched at %v; the name confirms whose corpse it is", named, c.want)
				}
			}
			if idx := r.FindCorpseIndex("corpse", viewer.Character); idx != 2 {
				t.Errorf(`FindCorpseIndex("corpse") = %d, want 2 (the newest)`, idx)
			}
			if idx := r.FindCorpseIndex("2.corpse", viewer.Character); idx != 1 {
				t.Errorf(`FindCorpseIndex("2.corpse") = %d, want 1 (the second newest)`, idx)
			}
			if got, ok := r.FindCorpse("corpse", viewer.Character); !ok || got.Character.Name != "Deadric" {
				t.Errorf(`FindCorpse("corpse") = %q, %v; want the newest, Deadric`, got.Character.Name, ok)
			}
		})
	}
}

func TestFindCorpse_ClearSightMatchesByName(t *testing.T) {
	r, viewer := detailsCorpseRoom(t)
	lamp := 90
	r.Lamp = &lamp
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightFull {
		t.Fatalf("precondition: viewer sight = %v, want SightFull", got)
	}
	if idx := r.FindCorpseIndex("deadric corpse", viewer.Character); idx != 2 {
		t.Errorf(`FindCorpseIndex("deadric corpse") = %d, want 2`, idx)
	}
	if idx := r.FindCorpseIndex("beggar", viewer.Character); idx != 1 {
		t.Errorf(`FindCorpseIndex("beggar") = %d, want 1 (the newest beggar)`, idx)
	}
	if got, ok := r.FindCorpse("city beggar corpse", viewer.Character); !ok || got.Character.Name != "City Beggar" {
		t.Errorf(`FindCorpse("city beggar corpse") = %q, %v`, got.Character.Name, ok)
	}
}
