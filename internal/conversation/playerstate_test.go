package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestPlayerState_HeardCallbacksUseTheCharacter(t *testing.T) {
	u := users.NewTestUser(7101, "heard", "Heardia", 7101)
	ps := PlayerState(u)
	require.False(t, ps.HasHeard("94.a"))
	ps.MarkHeard("94.a")
	require.True(t, u.Character.HasHeard("94.a"))
	require.True(t, ps.HasHeard("94.a"))
}
