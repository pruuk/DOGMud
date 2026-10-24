package questloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaysInto_ExposesTheIndex(t *testing.T) {
	w := goodWorld()
	ways := WaysInto(w)

	require.Len(t, ways["53-start"], 1)
	require.NotNil(t, ways["53-start"][0].Topic)
	require.Equal(t, 9112, ways["53-start"][0].MobId)
	require.Equal(t, "mob 9112 topic cull_offer", ways["53-start"][0].Where)

	require.Len(t, ways["53-firstkill"], 1)
	require.NotNil(t, ways["53-firstkill"][0].Trigger)
	require.Equal(t, 53, ways["53-firstkill"][0].Owner)
	require.Contains(t, ways["53-firstkill"][0].Where, "quest 53 trigger mob_death")

	require.Len(t, ways["53-end"], 1)
	require.Equal(t, "cull_in", ways["53-end"][0].Topic.Key)
}
