package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 2 (and 13 for NPCs): moving through a secret exit behind
// the leader teaches it; an ordinary exit records nothing.
func TestLearnSecretExitFollowing(t *testing.T) {
	room := &rooms.Room{RoomId: 61, Exits: map[string]exit.RoomExit{
		"west":  {RoomId: 62, Secret: true},
		"north": {RoomId: 63},
	}}
	c := &characters.Character{}
	LearnSecretExitFollowing(c, room, "north")
	LearnSecretExitFollowing(c, room, "nowhere")
	require.Empty(t, c.Discoveries, "an ordinary or missing exit records nothing")
	LearnSecretExitFollowing(c, room, "west")
	require.True(t, c.KnowsSecretExit(61, "west"))
	LearnSecretExitFollowing(nil, room, "west") // must not panic
}
