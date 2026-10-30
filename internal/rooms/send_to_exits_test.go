package rooms

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
)

// SendTextToExitsExcluding asks for the excludes once per neighbouring room
// with an exit back, and puts them on that room's message only (#317).
func TestSendTextToExitsExcluding_PerNeighbourExcludes(t *testing.T) {
	a := &Room{RoomId: 3190, Exits: map[string]exit.RoomExit{"north": {RoomId: 3191}, "east": {RoomId: 3192}}}
	b := &Room{RoomId: 3191, Exits: map[string]exit.RoomExit{"south": {RoomId: 3190}}}
	c := &Room{RoomId: 3192, Exits: map[string]exit.RoomExit{"west": {RoomId: 3190}}}
	defer SeedRoomsForTest(map[int]*Room{3190: a, 3191: b, 3192: c}, map[string]*ZoneConfig{})()
	defer events.DrainAllQueuedEventsForTest()

	asked := map[int]int{}
	a.SendTextToExitsExcluding("You hear someone talking.", true, func(tgt *Room) []int {
		asked[tgt.RoomId]++
		return []int{tgt.RoomId * 10}
	})
	for _, tc := range []struct {
		room    int
		exit    string
		exclude int
	}{{3191, "south", 31910}, {3192, "west", 31920}} {
		msgs := events.DrainQueuedRoomMessagesForTest(tc.room)
		if len(msgs) != 1 {
			t.Fatalf("room %d: %d messages, want 1", tc.room, len(msgs))
		}
		m := msgs[0]
		if !strings.Contains(m.Text, tc.exit) || !strings.Contains(m.Text, "You hear someone talking.") || !m.IsQuiet {
			t.Errorf("room %d message %+v", tc.room, m)
		}
		if !slices.Equal(m.ExcludeUserIds, []int{tc.exclude}) {
			t.Errorf("room %d excludes %v, want only its own [%d]", tc.room, m.ExcludeUserIds, tc.exclude)
		}
		if asked[tc.room] != 1 {
			t.Errorf("room %d: excludes asked %d times, want once", tc.room, asked[tc.room])
		}
	}

	// The plain sender still puts one list on every neighbour.
	a.SendTextToExits("A bell rings.", false, 7)
	for _, room := range []int{3191, 3192} {
		msgs := events.DrainQueuedRoomMessagesForTest(room)
		if len(msgs) != 1 || !slices.Equal(msgs[0].ExcludeUserIds, []int{7}) {
			t.Errorf("SendTextToExits room %d: %+v", room, msgs)
		}
	}
}
