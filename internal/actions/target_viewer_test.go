package actions

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func viewerTestHide(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "target_viewer_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

// viewerTestRoom holds Aliceia (7701), who looks, and Kesh (7702), who hides.
func viewerTestRoom(t *testing.T) (*rooms.Room, *users.UserRecord) {
	t.Helper()
	viewer := users.NewTestUser(7701, "aliceia", "Aliceia", 97701)
	hider := users.NewTestUser(7702, "kesh", "Kesh", 97702)
	viewerTestHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7701: viewer, 7702: hider}))
	room := &rooms.Room{RoomId: 7700}
	room.AddPlayer(7701)
	room.AddPlayer(7702)
	return room, viewer
}

func TestResolveTargetActor_ViewerCannotNameAHiddenCreature(t *testing.T) {
	room, viewer := viewerTestRoom(t)
	_, err := ResolveTargetActor(room, "kesh", ResolveTargetOptions{Viewer: viewer.Character})
	require.True(t, errors.Is(err, ErrTargetNotFound), "got %v", err)
}

func TestResolveTargetActor_NoViewerStillReachesThem(t *testing.T) {
	room, _ := viewerTestRoom(t)
	target, err := ResolveTargetActor(room, "kesh")
	require.NoError(t, err, "staff tools pass no viewer and must still reach a hidden player")
	require.Equal(t, 7702, target.GetUserId())
}
