package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// BreakOffMenuToFollow closes the follower's game menu or non-modal prompt
// because a leader is pulling them away, and tells the follower, the leader
// (when a player) and the room left behind, each through the sight rules
// (owner rule 2026-10-20, #375). Call it before queuing the follower's step.
func BreakOffMenuToFollow(follower *users.UserRecord, leader *characters.Character, leaderUserId int, room *rooms.Room) {
	if follower == nil || leader == nil || room == nil {
		return
	}
	topic, topicNames, had := follower.CloseMenuForFollow()
	if !had {
		return
	}
	leaderTag := `mobname`
	if leaderUserId > 0 {
		leaderTag = `username`
	}
	fName := `<ansi fg="username">` + follower.Character.Name + `</ansi>`
	lName := `<ansi fg="` + leaderTag + `">` + leader.Name + `</ansi>`
	toFollower, toLeader, toRoom := menu.FollowLines(fName, lName, topic)

	hideForFollower := append([]string{leader.Name}, topicNames...)
	follower.SendText(messaging.CategoryRoomExit,
		messaging.HideNames(toFollower, hideForFollower, room.DepartureSight(follower.Character, leader)))

	if leaderUserId > 0 {
		if lu := users.GetByUserId(leaderUserId); lu != nil {
			hideForLeader := append([]string{follower.Character.Name}, topicNames...)
			lu.SendText(messaging.CategoryRoomExit,
				messaging.HideNames(toLeader, hideForLeader, room.DepartureSight(lu.Character, leader)))
		}
	}

	roomNames := append([]string{follower.Character.Name, leader.Name}, topicNames...)
	// The leader has already left room, so every reader is judged with the
	// leader's own carried light counted in it (a mover is seen by their own
	// light on the way out).
	room.SendTextVisualToSnapshot(room.DepartureSnapshot(leader), messaging.CategoryRoomExit, toRoom, roomNames,
		follower.UserId, leaderUserId)
}
