package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #697: the quest engine hears a quaff only after the drinker has read "You
// drink the ...", so a step's line ("The salve spreads warm through you")
// never arrives before the drink it reports. The quest engine hears the
// command as quaff (#375 phase 7).
func TestQuaff_DrinkLineComesBeforeTheQuestLine(t *testing.T) {
	const userId, tonicId = 7311, 39311
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tonicId: {ItemId: tonicId, Name: "test tonic", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
	}))
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 98,
		Name:    "Quaff Order Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "quaff",
			Actions: []quests.ActionDef{{Grant: "98-start"}, {SendText: "QUEST LINE"}},
		}},
	})
	c := characters.New()
	c.Name = "Quaffer"
	c.RoomId = 99999
	u := &users.UserRecord{UserId: userId, Character: c}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	require.True(t, c.StoreItem(items.New(tonicId)))
	events.DrainQueuedMessagesForTest(userId)

	res := Quaff(NewUserActorInRoom(u, newEmptyTestRoom(t)).(DrinkActor), "test tonic")
	require.True(t, res.Drank, "the tonic must be drunk: %+v", res)

	msgs := events.DrainQueuedMessagesForTest(userId)
	drank, quest := -1, -1
	for i, m := range msgs {
		if drank < 0 && strings.Contains(m, "You drink the") {
			drank = i
		}
		if quest < 0 && strings.Contains(m, "QUEST LINE") {
			quest = i
		}
	}
	require.GreaterOrEqual(t, drank, 0, "no drink line: %q", msgs)
	require.GreaterOrEqual(t, quest, 0, "the quaff did not reach the quest engine as quaff: %q", msgs)
	require.Less(t, drank, quest, "the quest line came before the drink: %q", msgs)
}
