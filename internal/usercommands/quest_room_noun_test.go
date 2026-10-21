package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

// Quest trigger nouns accept every name the room accepts (#375 batch 4
// re-verify N1). Room 5526 aliases `board` and `floor board` to its
// `floorboard` noun, and quest 63 listens on `noun: floorboard`; before this,
// `look floor board` showed the plank but the quest did not advance. These
// tests drive TryCommand so both quest events (room_interact before dispatch,
// command_issued after it) carry the room's noun key.

// listenForRoomNoun registers a one-trigger quest that grants questId-start on
// event in room roomId for noun, and returns a probe reporting whether it fired.
func listenForRoomNoun(t *testing.T, userHasQuest func(string) bool, clear func(string), questId int, event string, roomId int, trig questengine.TriggerDef) func() bool {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	token := fmt.Sprintf("%d-start", questId)
	trig.Event = event
	trig.Room = roomId
	trig.Actions = []questengine.ActionDef{{Grant: token}}
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId:  questId,
		Name:     "room noun probe",
		Steps:    []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{trig},
	})
	clear(token)
	t.Cleanup(func() { clear(token) })
	return func() bool { return userHasQuest(token) }
}

func TestQuestTriggerNoun_RoomAliasesReachTheRoomNoun(t *testing.T) {
	cases := []struct {
		name  string
		event string
		trig  questengine.TriggerDef
		typed string
		want  bool
	}{
		{"room_interact via two-word alias", "room_interact", questengine.TriggerDef{Noun: "floorboard"}, "floor board", true},
		{"room_interact via one-word alias", "room_interact", questengine.TriggerDef{Noun: "floorboard"}, "board", true},
		{"room_interact by the key itself", "room_interact", questengine.TriggerDef{Noun: "floorboard"}, "floorboard", true},
		{"command_issued via two-word alias", "command_issued", questengine.TriggerDef{Command: "look", Noun: "floorboard"}, "floor board", true},
		{"a sentence that mentions the noun", "room_interact", questengine.TriggerDef{Noun: "floorboard"}, "at the board over there", false},
		{"typed text that is no room noun still matches", "room_interact", questengine.TriggerDef{Noun: "guild mark"}, "guild mark", true},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			alice, _, room := speechWrapperScene(t)
			oldNouns := room.Nouns
			room.Nouns = map[string]string{
				"floorboard":  "A plank that sits a little proud.",
				"board":       ":floorboard",
				"floor board": ":floorboard",
			}
			t.Cleanup(func() { room.Nouns = oldNouns })

			fired := listenForRoomNoun(t, alice.Character.HasQuest, alice.Character.ClearQuestToken,
				999300+i, tc.event, room.RoomId, tc.trig)
			_, err := TryCommand("look", tc.typed, alice.UserId, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, fired(), "look %s", tc.typed)
		})
	}
}
