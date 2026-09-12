package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func hideForTest(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "slice_f_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

// A LIT room: this is about hiding, not darkness. A scout with no see-hidden
// must not sense a hider, so mobs and players cannot disagree about who is
// visible (slice A introduced Perceives as the single rule).
func TestCondRoomHasHiddenEntityUsesPerceives(t *testing.T) {
	m, room := sightScene(t, "city")
	u := users.NewTestUser(8130, "kesh", "Kesh", 98130)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8130: u}))
	room.AddPlayer(8130)
	hideForTest(t, u.Character)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condRoomHasHiddenEntity(nil, ctx),
		"a scout with no see-hidden must not sense a hider")
}

// A visible player is not a hidden entity either way.
func TestCondRoomHasHiddenEntityIgnoresVisiblePlayers(t *testing.T) {
	m, room := sightScene(t, "city")
	u := users.NewTestUser(8131, "ordel", "Ordel", 98131)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8131: u}))
	room.AddPlayer(8131)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condRoomHasHiddenEntity(nil, ctx))
}
