package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a, search rework: plain `search` can uncover a quest's search
// spot (a `verb: search` room_interact trigger) at the room difficulty, and a
// search refused by the shared special-move cooldown sends no quest event.

// leadsToFloorboard is the thorough line a plain search prints for the
// fixture's quest search spot.
const leadsToFloorboard = `You work through the room inch by inch, and your search leads you to the <ansi fg="noun">floorboard</ansi>.`

// searchEyes sets the searcher's perception for the test: high enough that a
// seeded roll at the room difficulty lands, or low enough that it misses.
func searchEyes(t *testing.T, alice *users.UserRecord, perception int) {
	t.Helper()
	old := alice.Character.Stats.Perception.ValueAdj
	alice.Character.Stats.Perception.ValueAdj = perception
	t.Cleanup(func() { alice.Character.Stats.Perception.ValueAdj = old })
	t.Cleanup(util.SetRandForTest(375))
}

func TestSearch_PlainSearchUncoversAQuestSearchSpot(t *testing.T) {
	alice, room := interactScene(t)
	searchEyes(t, alice, 100000)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999460, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})
	events.DrainQueuedMessagesForTest(alice.UserId)

	_, err := TryCommand("search", "", alice.UserId, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a plain search that wins its roll fires the search trigger")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n"), leadsToFloorboard)
}

func TestSearch_PlainSearchCanMissAQuestSearchSpot(t *testing.T) {
	alice, room := interactScene(t)
	searchEyes(t, alice, 1)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999461, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})
	events.DrainQueuedMessagesForTest(alice.UserId)

	_, err := TryCommand("search", "", alice.UserId, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a plain search that loses its roll fires nothing")
	got := strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n")
	require.NotContains(t, got, leadsToFloorboard)
	require.Contains(t, got, "You find nothing of interest.")
}

// A trigger that would not fire for this player (here, a step they are not
// on) is not rolled for: no line, so the search cannot hint at a quest the
// player does not have or has already finished.
func TestSearch_PlainSearchSkipsASpotWhoseTriggerWouldNotFire(t *testing.T) {
	alice, room := interactScene(t)
	searchEyes(t, alice, 100000)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999462, "room_interact", room.RoomId, questengine.TriggerDef{
			Noun: "floorboard", Verb: "search",
			Conditions: questengine.Conditions{Has: []string{"999462-never"}},
		})
	events.DrainQueuedMessagesForTest(alice.UserId)

	_, err := TryCommand("search", "", alice.UserId, 0)
	require.NoError(t, err)
	require.False(t, fired())
	got := strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n")
	require.NotContains(t, got, leadsToFloorboard)
	require.Contains(t, got, "You find nothing of interest.")
}

// After a special move, both searches are refused and send no quest event.
func TestSearch_RefusedAfterASpecialMoveSendsNoQuestEvent(t *testing.T) {
	for i, rest := range []string{"", "floorboard"} {
		t.Run("search "+rest, func(t *testing.T) {
			alice, room := interactScene(t)
			searchEyes(t, alice, 100000)
			interact := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
				999470+i, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})
			require.True(t, actions.ClaimSpecialMove(alice.Character), "fixture: a special move was just used")
			events.DrainQueuedMessagesForTest(alice.UserId)

			_, err := TryCommand("search", rest, alice.UserId, 0)
			require.NoError(t, err)
			require.False(t, interact(), "a refused search fires no room_interact")
			got := strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n")
			require.Contains(t, got, "You need a moment to recover before you can search.")
		})
	}
}
