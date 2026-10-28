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
