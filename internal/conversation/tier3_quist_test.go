package conversation

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// Tier 3: Quist's hand-in of The Margin Notation queues The Undercroft only
// when exactly one offer on his file opens on 73-end. This runs the real
// nextOffers logic over the shipped file, so a second offer gated on 73-end
// would fail here before it silently stopped the pitch.
func TestQuistHasExactlyOneNextOfferAtTheEndOfTheMargin(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/the_confluence/9441.yaml")
	require.NoError(t, err)
	var quist dialogue.DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &quist))

	require.Equal(t, []string{"undercroft_offer"}, nextOffers(&quist, "73-end", held("73-end")))
	require.Empty(t, nextOffers(&quist, "73-end", held("73-end", "74-start")), "a player already on 74 is not pitched it again")
}
