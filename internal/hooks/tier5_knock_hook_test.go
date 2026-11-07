package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// A real Accept of The Knock: the menu queues the Quest event, and this package's
// HandleQuestUpdate is what bridges a dialogue grant to the quest_granted trigger
// that speaks the knock. The usercommands rig lands the token by hand, so it never
// saw this line, and a player who accepted would have paid and been told nothing.
func TestTheKnock_ARealAcceptTeachesTheKnock(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	drainEventQueueOnCleanup(t)
	world := filepath.Join("..", "..", "_datafiles", "world", "dogmud")

	data, err := os.ReadFile(filepath.Join(world, "dialogue", "new_plymouth_docks", "9305.yaml"))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(data, &df))
	t.Cleanup(dialogue.InstallForTest(&df))

	var qs []*quests.Quest
	for _, f := range []string{"102-the_knock.yaml", "64-the_skip.yaml", "67-the_bloom_trail.yaml"} {
		src, err := os.ReadFile(filepath.Join(world, "quests", f))
		require.NoError(t, err)
		var q quests.Quest
		require.NoError(t, yaml.Unmarshal(src, &q))
		qs = append(qs, &q)
	}
	t.Cleanup(quests.SeedQuestsForTest(qs...))
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(quests.GetQuestById(102))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{102: {9305}}))

	user := users.GetByUserId(1)
	require.NotNil(t, user)
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.MobId = 9305
	mob.Zone = "New Plymouth Docks"
	room := rooms.LoadRoom(user.Character.RoomId)
	room.Lamp = rooms.LampPtr(90)
	user.Character.Gold = 1000
	user.Character.QuestProgress = map[int]string{}
	user.Character.SetQuestFlag("64-outcome", "reported")
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedQuestsForTest()

	conversation.Talk(user, room, mob)
	number := ""
	for i, e := range df.Menu("npc", conversation.PlayerState(user)).Entries {
		if e.Id == "q102_offer" {
			number = string(rune('1' + i))
		}
	}
	require.NotEmpty(t, number, "the offer is on the menu")
	require.True(t, user.RouteMenuInput(number, true))
	events.DrainQueuedMessagesForTest(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")

	queued := events.DrainQueuedQuestsForTest()
	require.Len(t, queued, 1, "the Accept queues the grant")
	require.Equal(t, "102-start", queued[0].QuestToken)
	require.Equal(t, 500, user.Character.Gold, "the gold has gone")

	events.DrainQueuedMessagesForTest(1)
	HandleQuestUpdate(queued[0])

	out := strings.Join(strings.Fields(strings.Join(events.DrainQueuedMessagesForTest(1), " ")), " ")
	require.Contains(t, out, "three short, then one")
	require.Equal(t, "start", user.Character.QuestProgress[102])
}
