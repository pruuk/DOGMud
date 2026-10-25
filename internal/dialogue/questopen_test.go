package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func offerFile() *DialogueFile {
	return &DialogueFile{MobId: 9112, Topics: []Topic{
		{Key: "cull_offer", Kind: KindOffer, Quest: 53, Label: "Cull the Wash", Text: "t"},
		{Key: "chat", Kind: KindInfo, Label: "Chat", Text: "t"},
	}}
}

func TestEligible_OfferHiddenWhenQuestNotOpen(t *testing.T) {
	df := offerFile()
	open := false
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		HasHeard:  func(string) bool { return false },
		QuestOpen: func(id int) bool { require.Equal(t, 53, id); return open },
	}
	require.False(t, df.Eligible(&df.Topics[0], ps))
	require.True(t, df.Eligible(&df.Topics[1], ps), "info topics ignore QuestOpen")
	require.Equal(t, "", df.Marker(ps), "no [!] while the quest is not open")
	open = true
	require.True(t, df.Eligible(&df.Topics[0], ps))
}

func TestEligible_NilQuestOpenFailsOffersClosed(t *testing.T) {
	df := offerFile()
	ps := &PlayerState{HasQuest: func(string) bool { return false }, HasHeard: func(string) bool { return false }}
	require.False(t, df.Eligible(&df.Topics[0], ps))
}
