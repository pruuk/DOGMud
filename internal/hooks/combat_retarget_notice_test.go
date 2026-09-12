package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "You turn your attention to X!" printed X regardless of whether the reader
// could see. RetargetOrEnd picks whoever is already attacking you, so the
// notice stands in the dark; only the name is hidden.

func TestRetargetNotice_LitRoomNamesTheTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)

	line, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 100})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to Skeleton!", plainText(line))

	line, ok = retargetNotice(room, 1, state.ActorRef{UserId: 2})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to Bobrick!", plainText(line))
}

func TestRetargetNotice_DarkRoomHidesTheTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)

	line, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 100})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to something!", plainText(line))

	line, ok = retargetNotice(room, 1, state.ActorRef{UserId: 2})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to something!", plainText(line))
}

func TestRetargetNotice_UnresolvedTargetSaysNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)

	_, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 999})
	assert.False(t, ok)
	_, ok = retargetNotice(room, 1, state.ActorRef{})
	assert.False(t, ok)
}
