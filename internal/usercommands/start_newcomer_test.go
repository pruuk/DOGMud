package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The newcomer marker is a flag on The Awakening now, read by Hadwen's and
// Toke's teaching topics. Quest 21 is retired.
func TestGrantNewcomerMarkerSetsTheFlag(t *testing.T) {
	u := users.NewTestUser(9731, "newcomer", "Newcomer", 0)
	grantNewcomerMarker(u)
	require.Equal(t, "new", u.Character.GetQuestFlag("30-newcomer"))
	require.Empty(t, u.Character.QuestProgress, "no quest token any more")
}
