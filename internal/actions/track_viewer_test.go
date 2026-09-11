package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// `track <name>` reads a creature standing in the room and names it. It has its
// own matcher rather than the shared lookup, so it needed the perception rule
// too: without it a sneaking player is named to anyone who types their name.
func TestFindPresentTargetByNoun_HiddenCreatureIsNotNamed(t *testing.T) {
	room, viewer := viewerTestRoom(t)

	if name, _, found := findPresentTargetByNoun(viewer.Character, room, "kesh", 7701); found {
		t.Fatalf("tracking named the hidden player as %q", name)
	}
}

func TestFindPresentTargetByNoun_NoViewerIsUnchanged(t *testing.T) {
	room, _ := viewerTestRoom(t)

	name, isMob, found := findPresentTargetByNoun(nil, room, "kesh", 7701)
	require.True(t, found, "a mob tracker has no viewer and reads everyone, as before")
	require.False(t, isMob)
	require.Equal(t, "Kesh", name)
}
