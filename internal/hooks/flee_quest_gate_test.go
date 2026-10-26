package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #588 end to end: a player whose only exit is quest-gated reads the same
// line as a player whose only exit is locked, stays put, and does not read
// the gate's own narration (flee asks the gate without firing it).
func TestHandlePlayerFlee_AllExitsGatedReadsNoExit(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	// A flee that wrongly gets through runs Look, which reads the legend.
	t.Cleanup(keywords.SeedKeywordsForTest())
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	if _, ok := room.GetExitInfo("north"); !ok || len(room.Exits) != 1 {
		t.Fatalf("fixture premise: room 1 has a single north exit, has %v", room.Exits)
	}
	const gateLine = "You have no reason to go up there."
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999589,
		Name:    "flee gate probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       room.Exits["north"].RoomId,
			Conditions: questengine.Conditions{Missing: []string{"999589-start"}, InRoom: 1},
			Actions:    []questengine.ActionDef{{SendText: gateLine}},
		}},
	})
	startFlee(t, u, room)
	events.DrainQueuedMessagesForTest(u.UserId)

	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("flee was not resolved")
	}
	out := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	if u.Character.RoomId != 1 {
		t.Fatalf("the fleer walked through the gate into room %d", u.Character.RoomId)
	}
	if !strings.Contains(out, "You can't find an exit!") {
		t.Fatalf("all gated should read as all locked, got %q", out)
	}
	if strings.Contains(out, gateLine) {
		t.Fatalf("flee fired the gate's narration: %q", out)
	}
}
