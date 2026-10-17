package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Lighting plan 5c playtest finding 1, the track sibling. `track <name>` on a
// creature standing in the room used to answer before any roll, by name, on
// the reasoning "you can see it". A tracker who makes out only shapes cannot,
// so the free read is for full sight only; below it, the name is not
// confirmed by the in-room shortcut.
const trackSightScoutId = 9481

func trackSightRoom(t *testing.T, lamp int) (*rooms.Room, *trackFakeActor) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	room := newTrackTestRoom(9480)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(lamp)
	scout := newTrackTestMob(trackSightScoutId, "Midroad Scout", room.RoomId)
	mobs.SetInstanceForTest(trackSightScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(trackSightScoutId, nil) })
	room.AddMob(trackSightScoutId)
	return room, newTrackFakeActor("Tracker", room, true, 9482)
}

func trackSightSent(a *trackFakeActor) string { return strings.Join(a.sent, "\n") }

func TestTrack_PresentCreatureIsNamedOnlyWithFullSight(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		sight messaging.SightDecision
		named bool
	}{
		{"full sight reads the sign by name", 90, messaging.SightFull, true},
		{"shapes gets no free read by name", 35, messaging.SightShapes, false},
		{"no sight gets no free read by name", -50, messaging.SightNone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			room, actor := trackSightRoom(t, c.lamp)
			if got := messaging.ParticipantSight(actor.char, room); got != c.sight {
				t.Fatalf("precondition: sight at lamp %d = %v, want %v", c.lamp, got, c.sight)
			}
			// Several fresh trackers, so a lucky trail roll cannot hide a
			// leak: the in-room lines are the ones under test and neither
			// may fire. Fresh, because each track spends a cooldown.
			sent := ""
			for i := 0; i < 20; i++ {
				a := newTrackFakeActor("Tracker", room, true, actor.userId)
				Track(a, TrackOptions{TargetNoun: "scout"})
				sent += trackSightSent(a) + "\n"
			}
			inRoom := strings.Contains(sent, "is right here with you") || strings.Contains(sent, "is in the room with you")
			if c.named && !inRoom {
				t.Fatalf("full sight should read the present scout by name, got %q", sent)
			}
			if !c.named && inRoom {
				t.Fatalf("sight %v was told the scout is here by name: %q", c.sight, sent)
			}
		})
	}
}
