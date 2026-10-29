package main

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// The offer index door names the quests an NPC offers that cannot be
// abandoned because every offer hides on another quest's token (#375 phase 5).
func TestOfferIndexDoor_NamesUnresumableOffers(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	got := offerIndexDoorFrom([]*dialogue.DialogueFile{
		{MobId: 1, Topics: []dialogue.Topic{
			{Key: "a", Kind: dialogue.KindOffer, Gates: dialogue.Gates{QuestExcluded: []string{"88-start", "31-start"}},
				Effects: dialogue.Effects{GrantsQuest: "88-start"}},
			{Key: "b", Kind: dialogue.KindOffer, Gates: dialogue.Gates{QuestExcluded: []string{"42-start", "42-end"}},
				Effects: dialogue.Effects{GrantsQuest: "42-start"}},
		}},
	})
	if !slices.Equal(got, []int{88}) {
		t.Fatalf("logged %v, want [88]", got)
	}
	if !dialogue.Resumable(42) || dialogue.Resumable(88) {
		t.Fatalf("index not built: 42 %v, 88 %v", dialogue.Resumable(42), dialogue.Resumable(88))
	}
}
