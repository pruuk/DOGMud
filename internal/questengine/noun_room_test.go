package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Quest trigger nouns accept every name the room accepts (#375 batch 4
// re-verify N1). The command handlers resolve the typed words through the
// room (rooms.Room.NounNamedBy) and pass the room's own noun key as RoomNoun;
// a trigger noun matches that key when no trigger matches the typed text.

func notifyLookRoomNoun(e *Engine, player *fullMockPlayer, typed, roomNoun string) NotifyResult {
	ctx := newFullMockActionContext(1, player)
	return e.Notify("room_interact", EventDetails{
		UserId: 1, RoomId: 100, Verb: "look", Noun: typed, RoomNoun: roomNoun,
	}, player, ctx)
}

func TestTriggerNoun_RoomAliasFiresRoomNounTrigger(t *testing.T) {
	for _, typed := range []string{"floor board", "board"} {
		e := nounEngine(map[int]string{1: "floorboard"})
		p := newFullMockPlayer(100)
		assert.True(t, notifyLookRoomNoun(e, p, typed, "floorboard").Handled,
			"look %s (the room resolves it to floorboard) should fire a noun: floorboard trigger", typed)
		assert.True(t, p.HasQuest(questTokenFor(1)))
	}
}

func TestTriggerNoun_HyphenMatchesSpace(t *testing.T) {
	// Room text shows `processional-reliefs`; the trigger noun is spaced.
	e := nounEngine(map[int]string{1: "processional reliefs"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e, p, "processional-reliefs", "").Handled)
	assert.True(t, p.HasQuest(questTokenFor(1)))

	// Singular/plural still applies on top.
	e2 := nounEngine(map[int]string{1: "processional reliefs"})
	p2 := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e2, p2, "processional-relief", "").Handled)

	// A hyphenated trigger noun matches the spaced room noun.
	e3 := nounEngine(map[int]string{1: "nave-lamps"})
	p3 := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e3, p3, "lamps", "nave lamps").Handled)
}

func TestTriggerNoun_TypedTextStillMatchesWithoutRoomNoun(t *testing.T) {
	// A trigger noun that is not a room noun still matches the typed text.
	e := nounEngine(map[int]string{1: "guild mark"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e, p, "guild mark", "").Handled)
	assert.True(t, p.HasQuest(questTokenFor(1)))

	// And when the room resolves the words to some other key.
	e2 := nounEngine(map[int]string{1: "guild mark"})
	p2 := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e2, p2, "guild mark", "old guild mark").Handled)
	assert.True(t, p2.HasQuest(questTokenFor(1)))
}

func TestTriggerNoun_TypedExactPreferredOverRoomNoun(t *testing.T) {
	// Quest 63 authors both `board` and `floorboard`: `look board` fires the
	// board trigger only, never both.
	e := nounEngine(map[int]string{1: "board", 2: "floorboard"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e, p, "board", "floorboard").Handled)
	assert.True(t, p.HasQuest(questTokenFor(1)))
	assert.False(t, p.HasQuest(questTokenFor(2)), "floorboard trigger must not fire when board matches the typed text")
}

func TestTriggerNoun_NumberVariantPreferredOverRoomNoun(t *testing.T) {
	e := nounEngine(map[int]string{1: "board", 2: "floorboard"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLookRoomNoun(e, p, "boards", "floorboard").Handled)
	assert.True(t, p.HasQuest(questTokenFor(1)))
	assert.False(t, p.HasQuest(questTokenFor(2)), "floorboard trigger must not fire when a number variant of the typed text matches")
}

func TestTriggerNoun_RoomNounMatchingBothWaysFiresOnce(t *testing.T) {
	// The typed text IS the room noun: one trigger, one grant, one fire.
	// No conditions, so only the evaluation itself can stop a second fire.
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 1,
		Name:    "once",
		Steps:   []QuestStep{{Id: "1-a", Description: "a"}},
		Triggers: []TriggerDef{{
			Event: "room_interact", Room: 100, Noun: "floorboard",
			Actions: []ActionDef{{SendText: "fired"}},
		}},
	})
	for _, typed := range []string{"floorboard", "floor board"} {
		p := newFullMockPlayer(100)
		ctx := newFullMockActionContext(1, p)
		assert.True(t, e.Notify("room_interact", EventDetails{
			UserId: 1, RoomId: 100, Verb: "look", Noun: typed, RoomNoun: "floorboard",
		}, p, ctx).Handled)
		assert.Len(t, ctx.sentTexts, 1, "look %s must fire the trigger exactly once", typed)
	}
}

func TestTriggerNoun_RoomNounDoesNotWidenOtherFilters(t *testing.T) {
	e := nounEngine(map[int]string{1: "floorboard"})
	p := newFullMockPlayer(100)
	ctx := newFullMockActionContext(1, p)
	// Wrong verb: the trigger is `verb: look`.
	assert.False(t, e.Notify("room_interact", EventDetails{
		UserId: 1, RoomId: 100, Verb: "get", Noun: "board", RoomNoun: "floorboard",
	}, p, ctx).Handled)
	// Wrong room.
	assert.False(t, e.Notify("room_interact", EventDetails{
		UserId: 1, RoomId: 101, Verb: "look", Noun: "board", RoomNoun: "floorboard",
	}, p, ctx).Handled)
}
