package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-02: a search where the player sees nothing at all does
// not move a quest step. The sight decision is the game's own
// (messaging.ParticipantSight), so a carried light or working night vision
// counts through the existing rules.

const searchDarkTorchCondition = 9801

// darkSearchScene puts user 1 alone in a room at light 0, where a bare
// character sees nothing, with a torch condition registered but not worn.
func darkSearchScene(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := seedDarknessGateRoom(t, 0)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		searchDarkTorchCondition: {ConditionId: searchDarkTorchCondition, Name: "Test Search Torch", RoundInterval: 1, TriggerCount: 100,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	user.Character.Cooldowns = characters.Cooldowns{}
	t.Cleanup(func() { user.Character.Cooldowns = characters.Cooldowns{} })
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "fixture: a bare character sees nothing here")
	return user, room
}

func carryTorch(t *testing.T, user *users.UserRecord, room *rooms.Room) {
	t.Helper()
	require.NoError(t, user.Character.AddCondition(searchDarkTorchCondition, true))
	require.True(t, user.Character.EmitsLight(), "fixture: the torch burns")
	require.NotEqual(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "fixture: the torch lets them see")
}

func searchOutput(t *testing.T, user *users.UserRecord, room *rooms.Room) string {
	t.Helper()
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Search("", user, room, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

func TestSearch_InTotalDarknessAQuestStepDoesNotMove(t *testing.T) {
	user, room := darkSearchScene(t)
	fired := listenForCommand(t, user, 999601, "search")

	out := searchOutput(t, user, room)
	require.False(t, fired(), "a search in total darkness must not move the step")
	require.Contains(t, out, searchTooDarkLine)
	require.NotContains(t, out, "You find nothing of interest.", "no second, misleading line")
}

func TestSearch_WithALightTheSameStepMoves(t *testing.T) {
	user, room := darkSearchScene(t)
	carryTorch(t, user, room)
	fired := listenForCommand(t, user, 999602, "search")

	out := searchOutput(t, user, room)
	require.True(t, fired(), "a search with a light counts as normal")
	require.NotContains(t, out, searchTooDarkLine)
}

func TestSearch_InTheDarkWithNoLiveStepSaysNothingExtra(t *testing.T) {
	user, room := darkSearchScene(t)
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)

	out := searchOutput(t, user, room)
	require.NotContains(t, out, searchTooDarkLine, "a player on no quest reads no extra line")
	require.Contains(t, out, "You find nothing of interest.")
}

func TestSearch_InTheDarkAStepAlreadyDoneSaysNothingExtra(t *testing.T) {
	user, room := darkSearchScene(t)
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999603,
		Name:    "dark search probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "command",
			Command:    "search",
			Conditions: questengine.Conditions{Missing: []string{"999603-start"}},
			Actions:    []questengine.ActionDef{{Grant: "999603-start"}},
		}},
	})
	user.Character.GiveQuestToken("999603-start")
	t.Cleanup(func() { user.Character.ClearQuestToken("999603-start") })

	out := searchOutput(t, user, room)
	require.NotContains(t, out, searchTooDarkLine, "the step is done, so nothing would have moved")
}

// A `verb: search` room_interact spot (quest 76) follows the same rule: it is
// not rolled in the dark, so it cannot fire, and the line says why.
func TestSearch_InTheDarkAQuestSearchSpotIsNotUncovered(t *testing.T) {
	// The lit scene the other search-spot tests use, put out: no sky, no lamp.
	user, room := interactScene(t)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		searchDarkTorchCondition: {ConditionId: searchDarkTorchCondition, Name: "Test Search Torch", RoundInterval: 1, TriggerCount: 100,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "fixture: dark")
	searchEyes(t, user, 100000)
	fired := listenForRoomNoun(t, user.Character.HasQuest, user.Character.ClearQuestToken,
		999604, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})

	out := searchOutput(t, user, room)
	require.False(t, fired(), "a spot is not uncovered in the dark")
	require.Contains(t, out, searchTooDarkLine)
	require.NotContains(t, out, "You find nothing of interest.")

	carryTorch(t, user, room)
	user.Character.Cooldowns = characters.Cooldowns{}
	// Taking the torch recalculated the stats, which dropped the test's
	// perception, and the dark search drew from the seeded roll: set both again.
	searchEyes(t, user, 100000)
	out = searchOutput(t, user, room)
	require.True(t, fired(), "with a light the same search uncovers it")
	require.NotContains(t, out, searchTooDarkLine)
}

// A blinded searcher in a lit room is told what `look` tells them: a light
// would not help, so the line must not suggest one (#364). The step still
// does not move.
func TestSearch_BlindedInALitRoomReadsTheBlindLineAndTheStepDoesNotMove(t *testing.T) {
	user, room := interactScene(t)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(user.Character, room), "fixture: lit")
	fired := listenForCommand(t, user, 999605, "search")
	blindForSpeechTest(t, user)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "fixture: blind")

	out := searchOutput(t, user, room)
	require.False(t, fired(), "a blinded search moves no step")
	require.Contains(t, out, "You can't see anything!")
	require.NotContains(t, out, searchTooDarkLine)
	require.NotContains(t, out, "You find nothing of interest.")
}

// A search in the dark that turns something up says so and stops there: the
// too-dark line would contradict a find.
func TestSearch_InTheDarkWhatTheSearchFoundIsNotFollowedByTheTooDarkLine(t *testing.T) {
	user, room := darkSearchScene(t)
	oldHidden := room.HiddenNouns
	room.HiddenNouns = map[string]rooms.HiddenNoun{"loose stone": {Description: "A stone."}}
	t.Cleanup(func() { room.HiddenNouns = oldHidden })
	searchEyes(t, user, 100000)
	fired := listenForCommand(t, user, 999606, "search")

	out := searchOutput(t, user, room)
	require.False(t, fired(), "the step still does not move in the dark")
	require.Contains(t, out, "loose stone", "fixture: the search found the hidden noun")
	require.NotContains(t, out, searchTooDarkLine)
}
