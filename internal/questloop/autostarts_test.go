package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// AutoStarts names the quests whose -start a trigger or a behaviour tree
// grants (owner ruling 6, #375 phase 5): such a grant would take a shelved
// quest back up without the player choosing to. Offers, completion rewards,
// item questtokens and grants of other steps do not count.
func TestAutoStarts(t *testing.T) {
	w := World{
		Quests: []*quests.Quest{
			{QuestId: 30, Triggers: []quests.TriggerDef{
				{Event: "room_enter", Room: 5200, Actions: []quests.ActionDef{{Grant: "30-start"}}},
			}},
			{QuestId: 40, Triggers: []quests.TriggerDef{
				{Event: "mob_death", Mob: 1, Actions: []quests.ActionDef{{Grant: "40-track"}}},
				{Event: "item_gain", Item: 2, Actions: []quests.ActionDef{{Sequence: &quests.SequenceDef{
					OnComplete: []quests.ActionDef{{Grant: "41-start"}}}}}},
			}, Rewards: quests.QuestReward{QuestId: "44-start"}},
		},
		Dialogue: []*dialogue.DialogueFile{{MobId: 9, Topics: []dialogue.Topic{
			{Key: "offer", Kind: dialogue.KindOffer, Effects: dialogue.Effects{GrantsQuest: "45-start"}},
		}}},
		External: []ExternalGrant{
			{Where: "room 6467 tree grant_quest 30-start", Token: "30-start"},
			{Where: "mob 9100 tree grant_quest 42-start", Token: "42-start", MobId: 9100},
			{Where: "item 40035 questtoken", Token: "43-start", ItemId: 40035},
		},
	}
	require.Equal(t, map[int][]string{
		30: {"quest 30 trigger room_enter room=5200 grants=30-start", "room 6467 tree grant_quest 30-start"},
		41: {"quest 40 trigger item_gain item=2 grants=41-start"},
		42: {"mob 9100 tree grant_quest 42-start"},
	}, AutoStarts(w))
}
