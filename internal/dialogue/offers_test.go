package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOfferIndex(t *testing.T) {
	t.Cleanup(SetOfferIndexForTest(nil))
	BuildOfferIndex([]*DialogueFile{
		{MobId: 9137, Topics: []Topic{
			{Key: "hunt_offer", Kind: KindOffer, Effects: Effects{GrantsQuest: "42-start"}},
			{Key: "hunt_turnin", Kind: KindHandIn, Effects: Effects{GrantsQuest: "42-end"}},
		}},
		{MobId: 9106, Topics: []Topic{{Key: "a", Effects: Effects{GrantsQuest: "32-start"}}}},
		{MobId: 9108, Topics: []Topic{{Key: "b", Effects: Effects{GrantsQuest: "32-start"}}}},
	})
	require.Equal(t, []int{9137}, Offerers(42))
	require.Equal(t, []int{9106, 9108}, Offerers(32))
	require.Empty(t, Offerers(28))

	// A save replaces that file's offers only.
	UpdateOfferIndex(&DialogueFile{MobId: 9106})
	require.Equal(t, []int{9108}, Offerers(32))
	require.Equal(t, []int{9137}, Offerers(42))
}

// A quest is resumable (so abandonable) only when some offering topic's
// exclusions name no token or flag of another quest: such an offer hides for
// good once the player gains that other quest, and a shelved quest behind it
// would be lost (#375 phase 5; quest 88's road_toke excludes 31-start).
func TestOfferIndex_Resumable(t *testing.T) {
	t.Cleanup(SetOfferIndexForTest(nil))
	BuildOfferIndex([]*DialogueFile{
		{MobId: 1, Topics: []Topic{
			{Key: "own", Kind: KindOffer, Gates: Gates{QuestRequired: []string{"30-end"},
				QuestExcluded: []string{"42-start", "42-end"}}, Effects: Effects{GrantsQuest: "42-start"}},
			{Key: "other", Kind: KindOffer, Gates: Gates{QuestExcluded: []string{"88-start", "31-start"}},
				Effects: Effects{GrantsQuest: "88-start"}},
			{Key: "otherflag", Kind: KindOffer, Gates: Gates{QuestFlagExcluded: map[string]string{"11-branch": "rhett"}},
				Effects: Effects{GrantsQuest: "13-start"}},
			{Key: "ownflag", Kind: KindOffer, Gates: Gates{QuestFlagExcluded: map[string]string{"34-boon": "might"}},
				Effects: Effects{GrantsQuest: "34-start"}},
		}},
		{MobId: 2, Topics: []Topic{
			{Key: "a", Kind: KindOffer, Gates: Gates{QuestExcluded: []string{"50-start", "29-end"}}, Effects: Effects{GrantsQuest: "50-start"}},
		}},
		{MobId: 3, Topics: []Topic{
			{Key: "b", Kind: KindOffer, Effects: Effects{GrantsQuest: "50-start"}},
		}},
	})
	require.True(t, Resumable(42), "its own tokens are not held while shelved")
	require.False(t, Resumable(88), "31-start hides the only offer for good")
	require.False(t, Resumable(13), "another quest's flag hides the only offer")
	require.True(t, Resumable(34), "its own flag is fine")
	require.True(t, Resumable(50), "one clean offer is enough")
	require.Equal(t, []int{3}, Resumers(50))
	require.Equal(t, []int{2, 3}, Offerers(50))
	require.False(t, Resumable(28), "not offered at all")
	require.Equal(t, []int{13, 88}, Unresumable())

	// A save that drops the clean offer leaves 50 offered but not resumable.
	UpdateOfferIndex(&DialogueFile{MobId: 3})
	require.False(t, Resumable(50))
	require.Equal(t, []int{13, 50, 88}, Unresumable())
}
