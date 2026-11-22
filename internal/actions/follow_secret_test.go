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

// #781: the exit a follower takes behind its leader is the one the leader
// could have used. Among the exits to the destination, one that teaches
// nothing comes first, then a secret one the leader knows; a secret one the
// leader does not know only when nothing else leads there (a scripted mob
// route). A temporary exit answers with its key, not its title.
func TestFollowExitTo(t *testing.T) {
	room := &rooms.Room{RoomId: 61,
		Exits: map[string]exit.RoomExit{
			"crack":  {RoomId: 62, Secret: true},
			"north":  {RoomId: 62},
			"hollow": {RoomId: 63, Secret: true},
			"west":   {RoomId: 64, Secret: true},
		},
		ExitsTemp: map[string]exit.TemporaryRoomExit{
			"gap": {RoomId: 63, Title: "a narrow gap"},
		},
	}
	leader := &characters.Character{}
	leader.LearnSecretExit(61, "crack")
	leader.LearnSecretExit(61, "west")

	require.Equal(t, "north", FollowExitTo(room, leader, 62), "a plain exit teaches nothing, so it comes first")
	require.Equal(t, "gap", FollowExitTo(room, leader, 63), "the leader could not have used the secret hollow")
	require.Equal(t, "west", FollowExitTo(room, leader, 64), "the leader's own secret exit")
	require.Equal(t, "west", FollowExitTo(room, nil, 64), "nothing else leads there: a scripted route")
	require.Equal(t, "", FollowExitTo(room, leader, 99), "no exit: a teleport")
}

// #781, owner ruling 13: an NPC behind its leader takes an exit it knows. At a
// secret exit it has not found, it learns and goes on when the leader is a
// mob (a packmate behind its alpha, D17) or a player who charmed it; any other
// NPC following a player stops there.
func TestNPCMayFollowThrough(t *testing.T) {
	room := &rooms.Room{RoomId: 71, Exits: map[string]exit.RoomExit{
		"west":  {RoomId: 72, Secret: true},
		"north": {RoomId: 73},
	}}

	stray := &characters.Character{}
	require.True(t, NPCMayFollowThrough(stray, room, "north", 5), "a plain exit")
	require.False(t, NPCMayFollowThrough(stray, room, "west", 5), "a stray behind a player stops")
	require.False(t, stray.KnowsSecretExit(71, "west"))

	pet := &characters.Character{Charmed: characters.NewCharm(5, -1, "")}
	require.True(t, NPCMayFollowThrough(pet, room, "west", 5), "a companion goes on")
	require.True(t, pet.KnowsSecretExit(71, "west"), "and learns it")

	packmate := &characters.Character{}
	require.True(t, NPCMayFollowThrough(packmate, room, "west", 0), "behind a mob leader")
	require.True(t, packmate.KnowsSecretExit(71, "west"), "and learns it")
}
