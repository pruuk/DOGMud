package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func TestCharQuests_AbandonableFlag(t *testing.T) {
	t.Cleanup(questengine.ResetEngineForTest())
	steps := []quests.QuestStep{{Id: "start", Description: "a"}, {Id: "track", Description: "b"}, {Id: "end", Description: "c"}}
	eng := questengine.GetEngine()
	eng.RegisterQuest(&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: steps})
	eng.RegisterQuest(&quests.Quest{QuestId: 28, Name: "Waking to Gaius", Steps: steps})
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{42: {9137}}))

	u := menuTestUser(t, 9831, 98831, true)
	u.Character.QuestProgress = map[int]string{42: "track", 28: "start"}

	g := &GMCPCharModule{}
	data, _ := g.GetCharNode(u, `Char.Quests`)
	got := map[int]bool{}
	for _, r := range data.([]GMCPCharModule_Payload_Quest) {
		got[r.Id] = r.Abandonable
	}
	require.Equal(t, map[int]bool{42: true, 28: false}, got)
}

func TestCharQuestsAbandon_ShelvesOfferedQuestOnly(t *testing.T) {
	t.Cleanup(questengine.ResetEngineForTest())
	steps := []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "end"}}
	eng := questengine.GetEngine()
	eng.RegisterQuest(&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: steps})
	eng.RegisterQuest(&quests.Quest{QuestId: 28, Name: "Waking to Gaius", Steps: steps})
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: steps},
		&quests.Quest{QuestId: 28, Name: "Waking to Gaius", Steps: steps},
	))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{42: {9137}}))

	u := menuTestUser(t, 9832, 98832, true)
	u.Character.QuestProgress = map[int]string{42: "track", 28: "start"}
	events.DrainQueuedQuestsForTest()

	op := func(id string) {
		(&GMCPModule{}).handleCharOp(GMCPCharOp{
			ConnectionId: 98832,
			Command:      `Char.Quests.Abandon`,
			Payload:      []byte(`{"id":` + id + `}`),
		})
	}

	op(`28`)
	require.False(t, u.Character.IsShelved(28), "no offer: nothing changes")
	require.Equal(t, "start", u.Character.QuestProgress[28])
	require.Empty(t, events.DrainQueuedQuestsForTest(), "a refusal refreshes nothing")

	op(`42`)
	require.True(t, u.Character.IsShelved(42))
	_, held := u.Character.QuestProgress[42]
	require.False(t, held)
	refresh := events.DrainQueuedQuestsForTest()
	require.Len(t, refresh, 1, "the panel is re-sent through one MarkerOnly quest event")
	require.True(t, refresh[0].MarkerOnly)
}

func TestCharQuestsFocus_GoesThroughFocusQuest(t *testing.T) {
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 85, Name: "Secret", Secret: true, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	))
	u := menuTestUser(t, 9833, 98833, true)
	u.Character.QuestProgress = map[int]string{42: "start", 85: "start"}
	focus := func(id string) {
		(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: 98833, Command: `Char.Quests.Focus`,
			Payload: []byte(`{"id":` + id + `}`)})
	}
	focus(`85`)
	require.Equal(t, 0, u.Character.LastQuestId)
	focus(`42`)
	require.Equal(t, 42, u.Character.LastQuestId)
}
