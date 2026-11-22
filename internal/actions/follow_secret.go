package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// LearnSecretExitFollowing records that c, moving through exitName of room
// behind the one it follows, now knows that exit when it is secret (#781,
// owner ruling 2: a player who moves through a secret exit with the leader
// they follow learns it; ruling 13 applies it to a companion or charmed NPC
// moving with its leader, and to a packmate behind its alpha). It is called
// when the follow step is issued, after the follower's sight check and before
// the step runs, because the step answers to the follower's own known-exit
// rule. An exit that is not secret, or no such exit: nothing.
func LearnSecretExitFollowing(c *characters.Character, room *rooms.Room, exitName string) {
	if c == nil || room == nil {
		return
	}
	if ex, ok := room.GetExitInfo(exitName); ok && ex.Secret {
		c.LearnSecretExit(room.RoomId, exitName)
	}
}
