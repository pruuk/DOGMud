package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a, spec section 4 (owner ruling 5): look and search are the
// only interaction verbs. The look family (look, l, examine, exam, read)
// raises room_interact with Verb "look"; `search <noun>` raises it with Verb
// "search"; every other verb typed at a room feature triggers nothing, and
// one no command handles gets the look-or-search nudge.

// interactScene is the lit fixture room holding a `floorboard` noun, with the
// shipped command aliases loaded, so `l` and `exam` resolve the way a player
// types them.
func interactScene(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	seedShippedCommandAliases(t)
	alice, _, room := speechWrapperScene(t)
	oldNouns := room.Nouns
	room.Nouns = map[string]string{"floorboard": "A plank that sits a little proud."}
	t.Cleanup(func() { room.Nouns = oldNouns })
	oldCooldowns := alice.Character.Cooldowns
	alice.Character.Cooldowns = nil
	t.Cleanup(func() { alice.Character.Cooldowns = oldCooldowns })
	return alice, room
}

func TestRoomInteract_LookFamilyFiresALookTrigger(t *testing.T) {
	for i, cmd := range []string{"look", "l", "examine", "exam", "read"} {
		for j, verb := range []string{"", "look"} {
			t.Run(cmd+"/verb="+verb, func(t *testing.T) {
				alice, room := interactScene(t)
				fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
					999400+i*2+j, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: verb})
				_, err := TryCommand(cmd, "floorboard", alice.UserId, 0)
				require.NoError(t, err)
				require.True(t, fired(), "%s floorboard must fire a verb=%q trigger", cmd, verb)
			})
		}
	}
}

func TestRoomInteract_OtherVerbsFireNothing(t *testing.T) {
	for i, cmd := range []string{"say", "whisper", "help", "get"} {
		t.Run(cmd, func(t *testing.T) {
			alice, room := interactScene(t)
			fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
				999420+i, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard"})
			_, _ = TryCommand(cmd, "floorboard", alice.UserId, 0)
			require.False(t, fired(), "%s floorboard must not fire a room_interact trigger", cmd)
		})
	}
}

func TestRoomInteract_SearchFiresOnlyASearchTrigger(t *testing.T) {
	t.Run("search fires verb search", func(t *testing.T) {
		alice, room := interactScene(t)
		fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
			999430, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})
		_, err := TryCommand("search", "floorboard", alice.UserId, 0)
		require.NoError(t, err)
		require.True(t, fired())
	})
	t.Run("search does not fire an empty (look) verb", func(t *testing.T) {
		alice, room := interactScene(t)
		fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
			999431, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard"})
		_, err := TryCommand("search", "floorboard", alice.UserId, 0)
		require.NoError(t, err)
		require.False(t, fired())
	})
	t.Run("look does not fire verb search", func(t *testing.T) {
		alice, room := interactScene(t)
		fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
			999432, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard", Verb: "search"})
		_, err := TryCommand("look", "floorboard", alice.UserId, 0)
		require.NoError(t, err)
		require.False(t, fired())
	})
}

// A trigger names the template room; inside an instance of it the player
// stands in an ephemeral copy, and the trigger still fires (as command_issued
// always has, through rooms.OriginalRoomId).
func TestRoomInteract_FiresInAnInstanceOfTheRoom(t *testing.T) {
	alice, room := interactScene(t)
	const templateId = 4_242
	t.Cleanup(rooms.SetOriginalRoomIdForTest(room.RoomId, templateId))
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999440, "room_interact", templateId, questengine.TriggerDef{Noun: "floorboard"})
	_, err := TryCommand("look", "floorboard", alice.UserId, 0)
	require.NoError(t, err)
	require.True(t, fired())
}

// usageLine is a line as SendText queues it on CategoryUsage.
func usageLine(s string) string { return `<ansi fg="usage">` + s + "</ansi>\n" }

// The exact nudge copy, pinned so a change to it is a deliberate one.
func TestInteractNudgeText(t *testing.T) {
	require.Equal(t,
		`Try <ansi fg="command">look floorboard</ansi> or <ansi fg="command">search floorboard</ansi>.`,
		InteractNudgeText("floorboard"))
}

func TestRoomInteract_UnknownVerbAtAFeatureIsNudged(t *testing.T) {
	alice, room := interactScene(t)
	fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
		999450, "room_interact", room.RoomId, questengine.TriggerDef{Noun: "floorboard"})
	events.DrainQueuedMessagesForTest(alice.UserId)
	handled, err := TryCommand("pry", "floorboard", alice.UserId, 0)
	require.NoError(t, err)
	require.True(t, handled, "the nudge answers the line, so no 'not recognized' follows")
	require.False(t, fired())
	require.Equal(t,
		[]string{usageLine(InteractNudgeText("floorboard"))},
		events.DrainQueuedMessagesForTest(alice.UserId))
}

func TestRoomInteract_NudgeNamesADiscoveredHiddenNounOnly(t *testing.T) {
	alice, room := interactScene(t)
	oldHidden := room.HiddenNouns
	room.HiddenNouns = map[string]rooms.HiddenNoun{"loose stone": {Description: "A stone."}}
	t.Cleanup(func() { room.HiddenNouns = oldHidden })

	handled, err := TryCommand("pry", "loose stone", alice.UserId, 0)
	require.NoError(t, err)
	require.False(t, handled, "an undiscovered hidden noun is never confirmed")

	alice.Character.AddDiscovery(room.RoomId, "loose stone")
	events.DrainQueuedMessagesForTest(alice.UserId)
	handled, err = TryCommand("pry", "loose stone", alice.UserId, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t,
		[]string{usageLine(InteractNudgeText("loose stone"))},
		events.DrainQueuedMessagesForTest(alice.UserId))
}

func TestRoomInteract_UnknownVerbAtNothingIsStillUnrecognized(t *testing.T) {
	alice, _ := interactScene(t)
	handled, err := TryCommand("pry", "teapot", alice.UserId, 0)
	require.NoError(t, err)
	require.False(t, handled)
}

// read keeps its job for carried things; with nothing carried by that name and
// no trigger to answer, it shows the room feature as look would.
func TestRead_FallsBackToTheRoomFeature(t *testing.T) {
	alice, _ := interactScene(t)
	events.DrainQueuedMessagesForTest(alice.UserId)
	_, err := TryCommand("read", "floorboard", alice.UserId, 0)
	require.NoError(t, err)
	got := strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n")
	require.Contains(t, got, "A plank that sits a little proud.")
	require.NotContains(t, got, "aren't carrying")

	_, err = TryCommand("read", "teapot", alice.UserId, 0)
	require.NoError(t, err)
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(alice.UserId), "\n"), "aren't carrying")
}
