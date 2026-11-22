package hooks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #940: a spoiled grenade falls out of the bandolier onto the ground. A
// quest's grenade fades instead: it was going to leave its holder.
func TestSpoiledQuestBombFadesInsteadOfFalling(t *testing.T) {
	const roomId, userId, bombId = 94401, 94401, 94401
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		bombId: {ItemId: bombId, Name: "Firebomb", Type: items.Grenade, Subtype: items.Throwable},
	}))
	for _, quest := range []bool{true, false} {
		room := &rooms.Room{RoomId: roomId}
		t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{roomId: room}, map[string]*rooms.ZoneConfig{}))
		u := users.NewTestUser(userId, "bomber", "Bomber", 94401)
		u.Character = characters.New()
		u.Character.Name = "Bomber"
		u.Character.RoomId = roomId
		t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
		// Preserved with no preserving bandolier worn reads as spoiled
		// (items.Item.AgingAt), the shortest way to a spoiled bomb.
		u.Character.PotionItems = []items.Item{{ItemId: bombId, Preserved: true, QuestGiven: quest}}
		events.DrainQueuedMessagesForTest(userId)

		ejectSpoiledFromBandolier(u, 1)

		out := strings.Join(events.DrainQueuedMessagesForTest(userId), "\n")
		if len(u.Character.PotionItems) != 0 {
			t.Fatalf("quest=%v: the bomb is still in the bandolier", quest)
		}
		fade := fmt.Sprintf(actions.QuestGearFadesLine, "Firebomb")
		if quest {
			if len(room.Items) != 0 {
				t.Fatalf("quest gear reached the floor: %v", room.Items)
			}
			if !strings.Contains(out, fade) {
				t.Fatalf("no fade line: %q", out)
			}
		} else {
			if len(room.Items) != 1 {
				t.Fatalf("a bought bomb falls to the floor, got %d items there", len(room.Items))
			}
			if strings.Contains(out, "fades away") {
				t.Fatalf("a bought bomb does not fade: %q", out)
			}
		}
	}
}
