package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Boot refuses a quest step with no destination (destinationDoor), so the
// admin spawn commands refuse a change that would strand one: `room set
// spawninfo clear` on the only room that spawns a quest's mob is refused and
// the room keeps its spawns.
func TestAdminRoomSpawnChange_RefusesStrandingAQuestStep(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	admin, room := getTestUserAndRoom(t)
	oldRole := admin.Role
	admin.Role = users.RoleAdmin
	t.Cleanup(func() { admin.Role = oldRole })

	const mobId = 987654 // no other room spawns it
	t.Cleanup(rooms.SeedTemplateSpawnsForTest(room.RoomId, mobId))
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{QuestId: 98765, Name: "Stranded", Steps: []quests.QuestStep{
		{Id: "find", Destination: quests.StepDestination{Mobs: []int{mobId}}},
		{Id: "end"},
	}}))
	oldSpawns := room.SpawnInfo
	room.SpawnInfo = []rooms.SpawnInfo{{MobId: mobId}}
	t.Cleanup(func() { room.SpawnInfo = oldSpawns })

	// Keeping the mob in the room is no change to refuse.
	assert.False(t, refusesSpawnChange(admin, room, []rooms.SpawnInfo{{MobId: mobId}}))

	events.DrainQueuedMessagesForTest(admin.UserId)
	handled, err := adminRoom_Set([]string{"set", "spawninfo", "clear"}, admin, room)
	require.NoError(t, err)
	require.True(t, handled)

	out := strings.Join(events.DrainQueuedMessagesForTest(admin.UserId), "\n")
	assert.Contains(t, out, "Refused")
	assert.Contains(t, out, "walk-destination 98765 step find")
	assert.Len(t, room.SpawnInfo, 1, "a refused clear keeps the room's spawns")
}
