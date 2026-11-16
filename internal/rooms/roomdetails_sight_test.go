package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const detailsSightScoutId = 7414

// detailsSightRoom is sightTestRoom plus one mob, the Midroad Scout, so the
// roster holds two players and a creature besides the viewer, Ordel (7413).
func detailsSightRoom(t *testing.T, biome string) (*Room, *users.UserRecord) {
	t.Helper()
	r := sightTestRoom(t, biome)
	m := &mobs.Mob{InstanceId: detailsSightScoutId}
	m.Character.Name = "Midroad Scout"
	m.Character.RoomId = r.RoomId
	m.Character.Conditions = conditions.New()
	m.Character.Awareness = awareness.NewMachine()
	m.Character.Health = 10
	m.Character.HealthMax.Value = 10
	mobs.SetInstanceForTest(detailsSightScoutId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(detailsSightScoutId, nil) })
	r.AddMob(detailsSightScoutId)
	return r, users.GetByUserId(7413)
}

func detailsRoster(d RoomTemplateDetails) []string {
	return append(append([]string{}, d.VisiblePlayers...), d.VisibleMobs...)
}

// A viewer who makes out only shapes must not learn who is here: every entry
// is the same anonymous figure HideNames uses, with no health, shop or other
// adjective, and the count survives.
func TestGetDetails_ShapesViewerReadsFiguresNotNames(t *testing.T) {
	r, viewer := detailsSightRoom(t, "cave")
	if !viewer.Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
		t.Fatal("precondition: the viewer should now carry infrared")
	}
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightShapes {
		t.Fatalf("precondition: viewer sight = %v, want SightShapes", got)
	}

	d := GetDetails(r, viewer)
	roster := detailsRoster(d)
	if len(roster) != 3 {
		t.Fatalf("shapes roster has %d entries %q, want 3 (two players, one mob)", len(roster), roster)
	}
	for _, entry := range roster {
		if entry != messaging.UnseenFigure(messaging.SightShapes) {
			t.Errorf("shapes roster entry %q, want exactly %q", entry, messaging.UnseenFigure(messaging.SightShapes))
		}
	}
	joined := strings.Join(roster, " ")
	for _, leak := range []string{"Aliceia", "Bobrick", "Midroad", "Scout", "%", "mobname", "username"} {
		if strings.Contains(joined, leak) {
			t.Errorf("shapes roster leaks %q: %q", leak, joined)
		}
	}
}

// A viewer who sees nothing lists nobody: the roster of a room they cannot see
// is not theirs to read (look refuses before this; who and peering do not).
func TestGetDetails_BlindViewerListsNobody(t *testing.T) {
	r, viewer := detailsSightRoom(t, "cave")
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightNone {
		t.Fatalf("precondition: viewer sight = %v, want SightNone", got)
	}
	if roster := detailsRoster(GetDetails(r, viewer)); len(roster) != 0 {
		t.Fatalf("a viewer who sees nothing read the roster %q", roster)
	}
}

// Clear sight is unchanged: names, as before.
func TestGetDetails_ClearViewerReadsNames(t *testing.T) {
	r, viewer := detailsSightRoom(t, "cave")
	lamp := 90
	r.Lamp = &lamp
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightFull {
		t.Fatalf("precondition: viewer sight = %v, want SightFull", got)
	}
	joined := strings.Join(detailsRoster(GetDetails(r, viewer)), " ")
	for _, name := range []string{"Aliceia", "Bobrick", "Midroad Scout"} {
		if !strings.Contains(joined, name) {
			t.Errorf("clear roster %q is missing %q", joined, name)
		}
	}
	if strings.Contains(joined, "a figure") {
		t.Errorf("clear roster anonymized someone: %q", joined)
	}
}

// #283: the roster never shows a health percent. The owner ruled health is
// hidden from player view; a wounded mob in clear sight still lists by name.
func TestGetDetails_RosterShowsNoHealthPercent(t *testing.T) {
	r, viewer := detailsSightRoom(t, "cave")
	lamp := 90
	r.Lamp = &lamp
	mobs.GetInstance(detailsSightScoutId).Character.Health = 4
	joined := strings.Join(detailsRoster(GetDetails(r, viewer)), " ")
	if !strings.Contains(joined, "Midroad Scout") {
		t.Fatalf("precondition: clear roster %q is missing the scout", joined)
	}
	if strings.Contains(joined, "%") {
		t.Errorf("roster shows a health percent: %q", joined)
	}
}
