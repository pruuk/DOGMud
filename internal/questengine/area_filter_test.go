package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installGrove installs area grove (5311-5313) and stubs the zone seam:
// 5311-5313 in Pothole Coulee, 100 in Thornwall.
func installGrove(t *testing.T) {
	t.Helper()
	zones := map[int]string{5311: "Pothole Coulee", 5312: "Pothole Coulee", 5313: "Pothole Coulee", 100: "Thornwall"}
	t.Cleanup(areas.InstallForTest([]areas.Area{{Key: "grove", Name: "the grove", Rooms: []int{5311, 5312, 5313}}}, zones))
	prev := roomZoneOf
	roomZoneOf = func(id int) string { return zones[id] }
	t.Cleanup(func() { roomZoneOf = prev })
}

func areaKillQuest(area, zone string) *QuestDef {
	return &QuestDef{
		QuestId: 330, Name: "Cull",
		Steps: []QuestStep{{Id: "start"}, {Id: "end"}},
		Triggers: []TriggerDef{{
			Event: "mob_death", Mob: 9110, Area: area, Zone: zone,
			Conditions: Conditions{Has: []string{"330-start"}, Missing: []string{"330-end"}},
			Actions:    []ActionDef{{Grant: "330-end"}},
		}},
	}
}

func TestAreaFilter(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(areaKillQuest("grove", ""))
	p := newFullMockPlayer(100)
	p.quests["330-start"] = true

	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9110, RoomId: 100}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("330-end"), "room 100 is not in the grove")
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9110, RoomId: 5312}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("330-end"), "room 5312 is in the grove")
}

// EventAt is how the hook sources (mob_death, item_gain, item_give,
// skill_use, quest_granted) build their event: an instance room matches by
// its template, so an area or room filter holds inside an instance.
func TestEventAtMatchesTheTemplateRoom(t *testing.T) {
	installGrove(t)
	t.Cleanup(rooms.SetOriginalRoomIdForTest(1_000_000_777, 5312))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5312}, EventAt(1, 1_000_000_777))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 100}, EventAt(1, 100))

	e := NewEngine()
	e.RegisterQuest(areaKillQuest("grove", ""))
	p := newFullMockPlayer(1_000_000_777)
	p.quests["330-start"] = true
	d := EventAt(1, 1_000_000_777)
	d.MobId = 9110
	e.Notify("mob_death", d, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("330-end"), "a kill in an instance of 5312 is in the grove")
}

// The walk's synthetic event stands in a room of the trigger's area, or of
// its zone, when the trigger names no room: otherwise the filter would make
// every area step a dead end.
func TestWalk_AreaAndZoneTriggersFinish(t *testing.T) {
	installGrove(t)
	prev := templateRoomZones
	templateRoomZones = func() map[int]string {
		return map[int]string{5311: "Pothole Coulee", 5312: "Pothole Coulee", 5313: "Pothole Coulee", 100: "Thornwall"}
	}
	t.Cleanup(func() { templateRoomZones = prev })
	for _, set := range []func(*quests.TriggerDef){
		func(td *quests.TriggerDef) { td.Area = "grove" },
		func(td *quests.TriggerDef) { td.Zone = "Thornwall" },
	} {
		w := walkWorld()
		set(&w.Quests[0].Triggers[0])
		require.Empty(t, Walk(w, questloop.Sources{}))
	}
}

func TestZoneFilter(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(areaKillQuest("", "Thornwall"))
	p := newFullMockPlayer(100)
	p.quests["330-start"] = true

	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9110, RoomId: 5312}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("330-end"), "room 5312 is not in Thornwall")
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9110, RoomId: 100}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("330-end"), "room 100 is in Thornwall")
}
