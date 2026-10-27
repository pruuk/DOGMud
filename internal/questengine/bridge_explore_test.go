package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// The bridge reads the character's per-zone map.
func TestGameBridge_MappedCount(t *testing.T) {
	installGrove(t)
	u := users.NewTestUser(1, "testuser", "TestChar", 0)
	u.Character.VisitedRooms = map[string][]int{"Pothole Coulee": {5311, 5313, 5400}, "Thornwall": {100, 5312}}
	var mr MapReader = NewGameBridge(u, 1)
	assert.Equal(t, 2, mr.MappedCount([]int{5311, 5312, 5313}), "5312 is on the map under another zone, so it does not count")
	assert.Equal(t, 3, mr.MappedCount([]int{5311, 5313, 100}))
	assert.Equal(t, 0, mr.MappedCount(nil))
}

// NotifyRoomMapped is the game's newly-mapped hook into CheckExplore.
func TestNotifyRoomMapped(t *testing.T) {
	installGrove(t)
	t.Cleanup(ResetEngineForTest())
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 340, Name: "Know the Grove",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "explore"}, {Id: "end"}},
	}))
	GetEngine().RegisterQuest(&QuestDef{
		QuestId: 340, Name: "Know the Grove",
		Steps: []QuestStep{{Id: "start"}, {Id: "explore"}, {Id: "end"}},
		Triggers: []TriggerDef{{
			Event: "room_enter", Area: "grove", Distinct: "all",
			Conditions: Conditions{Has: []string{"340-explore"}, MissingFlag: map[string]string{"340-known": "yes"}},
			Actions:    []ActionDef{{SetFlag: &QuestFlagAction{Key: "340-known", Value: "yes"}}},
		}},
	})
	u := users.NewTestUser(1, "testuser", "TestChar", 0)
	u.Character.QuestProgress = map[int]string{340: "explore"}
	u.Character.VisitedRooms = map[string][]int{"Pothole Coulee": {5311, 5312}}

	events.DrainQueuedQuestsForTest()
	t.Cleanup(func() { events.DrainQueuedQuestsForTest() })
	assert.False(t, NotifyRoomMapped(u, 5312).Handled)
	// The explore step's progress moved (2 of 3), so the tracker refreshes.
	assert.Equal(t, []events.Quest{{UserId: 1, QuestToken: "340-explore", MarkerOnly: true}}, events.DrainQueuedQuestsForTest())
	assert.False(t, NotifyRoomMapped(u, 100).Handled)
	assert.Empty(t, events.DrainQueuedQuestsForTest(), "a room of no area moves no explore step")
	u.Character.VisitedRooms["Pothole Coulee"] = append(u.Character.VisitedRooms["Pothole Coulee"], 5313)
	assert.True(t, NotifyRoomMapped(u, 5313).Handled)
	assert.Equal(t, "yes", u.Character.GetQuestFlag("340-known"))
	assert.False(t, NotifyRoomMapped(nil, 5313).Handled)
}
