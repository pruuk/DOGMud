package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedDryRunQuest registers one quest with a room_gate trigger in room 500
// (conditions: missing 90-start) that grants 90-start and sends text, a
// quest_granted follow-on that would grant 90-end, and search_at and look_at
// triggers in rooms 500 and 501 used by the SearchFeatures tests.
func seedDryRunQuest() *Engine {
	q := &QuestDef{
		QuestId: 90,
		Name:    "Dry Run",
		Steps: []QuestStep{
			{Id: "90-start", Description: "Begin"},
			{Id: "90-end", Description: "End"},
		},
		Triggers: []TriggerDef{
			{
				Event:      "room_gate",
				Room:       500,
				Conditions: Conditions{Missing: []string{"90-start"}},
				Actions: []ActionDef{
					{SendText: "A voice says: not yet."},
					{Grant: "90-start"},
				},
			},
			{
				Event:      "quest_granted",
				QuestToken: "90-start",
				Actions:    []ActionDef{{Grant: "90-end"}},
			},
			{Event: "search_at", Room: 500, Noun: "statue", Actions: []ActionDef{{SendText: "x"}}},
			{Event: "search_at", Room: 500, Noun: "altar", Actions: []ActionDef{{SendText: "x"}}},
			{Event: "search_at", Room: 500, Noun: "statue", Conditions: Conditions{Has: []string{"90-start"}}, Actions: []ActionDef{{SendText: "y"}}},
			{Event: "look_at", Room: 500, Noun: "mural", Actions: []ActionDef{{SendText: "x"}}},
			{Event: "search_at", Room: 501, Noun: "well", Actions: []ActionDef{{SendText: "x"}}},
			{Event: "room_gate", Room: 500, Noun: "gate", Actions: []ActionDef{{SendText: "x"}}},
		},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	return e
}

func TestWouldFire_TrueWhenConditionsHold(t *testing.T) {
	e := seedDryRunQuest()
	player := newFullMockPlayer(500)

	assert.True(t, e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: 500}, player))
}

func TestWouldFire_FalseWhenConditionsFail(t *testing.T) {
	e := seedDryRunQuest()
	player := newFullMockPlayer(500)
	player.quests["90-start"] = true

	assert.False(t, e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: 500}, player))
}

// A gate guards its room whatever the player's tokens: a mob has none to read.
func TestGuardsRoom_ReadsTheFieldsAndNotTheConditions(t *testing.T) {
	e := seedDryRunQuest()
	assert.True(t, e.GuardsRoom(500))
	assert.False(t, e.GuardsRoom(501))
	assert.False(t, NewEngine().GuardsRoom(500))
}

// "It matches as a real step does": a gate on an area or a zone guards each of
// its rooms and no other room.
func TestGuardsRoom_MatchesAnAreaAndAZone(t *testing.T) {
	installGrove(t)
	gate := func(area, zone string) *Engine {
		e := NewEngine()
		e.RegisterQuest(&QuestDef{
			QuestId: 91, Name: "Area Gate",
			Steps: []QuestStep{{Id: "start"}},
			Triggers: []TriggerDef{{
				Event: "room_gate", Area: area, Zone: zone,
				Conditions: Conditions{Missing: []string{"91-start"}},
				Actions:    []ActionDef{{SendText: "x"}},
			}},
		})
		return e
	}

	byArea := gate("grove", "")
	assert.True(t, byArea.GuardsRoom(5312))
	assert.False(t, byArea.GuardsRoom(100))

	byZone := gate("", "Pothole Coulee")
	assert.True(t, byZone.GuardsRoom(5311))
	assert.False(t, byZone.GuardsRoom(100))
}

func TestWouldFire_FalseWhenFieldsDoNotMatch(t *testing.T) {
	e := seedDryRunQuest()
	player := newFullMockPlayer(501)

	assert.False(t, e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: 501}, player))
	assert.False(t, e.WouldFire("no_such_event", EventDetails{UserId: 1, RoomId: 500}, player))
}

func TestWouldFire_UsesTheSameNounTierAsNotify(t *testing.T) {
	e := seedDryRunQuest()
	player := newFullMockPlayer(500)

	// "statues" reaches `noun: statue` through the singular/plural tier.
	assert.True(t, e.WouldFire("search_at", EventDetails{UserId: 1, RoomId: 500, Noun: "statues"}, player))
	// The room-noun tier: typed words name the room noun "altar".
	assert.True(t, e.WouldFire("search_at", EventDetails{UserId: 1, RoomId: 500, Noun: "stone table", RoomNoun: "altar"}, player))
	assert.False(t, e.WouldFire("search_at", EventDetails{UserId: 1, RoomId: 500, Noun: "ceiling"}, player))
}

func TestWouldFire_HasNoSideEffects(t *testing.T) {
	e := seedDryRunQuest()
	player := newFullMockPlayer(500)

	for i := 0; i < 3; i++ {
		assert.True(t, e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: 500}, player))
	}
	// No token granted, the chained quest_granted trigger did not run.
	assert.False(t, player.HasQuest("90-start"))
	assert.False(t, player.HasQuest("90-end"))
	assert.Empty(t, player.flags)

	// The real Notify still fires afterwards: the dry run consumed nothing.
	ctx := newFullMockActionContext(1, player)
	result := e.Notify("room_gate", EventDetails{UserId: 1, RoomId: 500}, player, ctx)
	assert.True(t, result.Handled)
	assert.Equal(t, []string{"A voice says: not yet."}, ctx.sentTexts)
	assert.Equal(t, []string{"90-start", "90-end"}, ctx.granted)
}

func TestSearchFeatures_SortedDistinctSearchNounsForRoom(t *testing.T) {
	e := seedDryRunQuest()

	assert.Equal(t, []string{"altar", "statue"}, e.SearchFeatures(500))
	assert.Equal(t, []string{"well"}, e.SearchFeatures(501))
	assert.Empty(t, e.SearchFeatures(502))
}
