package actions

import (
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// LearnSecretExitFollowing records that c, moving through exitName of room
// behind the one it follows, now knows that exit when it is secret (#781,
// owner ruling 2: a player who moves through a secret exit with the leader
// they follow learns it; ruling 13 applies it to a companion or charmed NPC
// moving with its leader, and to a packmate behind its alpha). It is called
// when the follow step is issued, before the step runs, because the step
// answers to the follower's own known-exit rule. Where the path has a sight
// check (a party member, a player follower) it comes first; a companion, an
// NPC follower and a shadower step without one. An exit that is not secret,
// or no such exit: nothing.
func LearnSecretExitFollowing(c *characters.Character, room *rooms.Room, exitName string) {
	if c == nil || room == nil {
		return
	}
	if ex, ok := room.GetExitInfo(exitName); ok && ex.Secret {
		c.LearnSecretExit(room.RoomId, exitName)
	}
}

// LearnSecretExitWatching records that every onlooker in room who saw a
// movement line naming exitName of room now knows that exit when it is
// secret (#781, owner ruling 14: an onlooker who sees the mover learns the
// exit). Call it beside the line: "X leaves towards the <exit> exit" in the
// room left, "X enters from the <exit>" in the room entered (exitName is
// then the exit there leading back, "" when the line just says "arrives",
// which names nothing and teaches nothing), "X flees to the <exit> exit!".
//
// Who saw it is the decision the line itself was delivered at, never a
// second sight rule: players is the VisualSnapshot the line was sent
// against (room.VisualSnapshot() at the send, for a line judged by the room
// as it is now), npcs the MobVisualSnapshot taken at the same moment. An
// onlooker learns at SightFull and at SightShapes (a shapes-only reader's
// line still names the exit: only names become "a figure"); one at
// SightNone read the sound line, or nothing, and learns nothing. Like the
// line, it reaches only those in room now AND in the snapshot. mover is
// skipped: this rule changes nothing about what the mover knows. An NPC
// onlooker records on its own instance, as search does.
func LearnSecretExitWatching(room *rooms.Room, exitName string, mover *characters.Character, players rooms.VisualSnapshot, npcs rooms.MobVisualSnapshot) {
	if room == nil {
		return
	}
	if ex, ok := room.GetExitInfo(exitName); !ok || !ex.Secret {
		return
	}
	saw := func(d messaging.SightDecision) bool { return d != messaging.SightNone }
	for _, uid := range room.GetPlayers() {
		d, ok := players[uid]
		if !ok || !saw(d) {
			continue
		}
		if u := users.GetByUserId(uid); u != nil && u.Character != mover {
			u.Character.LearnSecretExit(room.RoomId, exitName)
		}
	}
	for _, miid := range room.GetMobs() {
		d, ok := npcs[miid]
		if !ok || !saw(d) {
			continue
		}
		if m := mobs.GetInstance(miid); m != nil && &m.Character != mover {
			m.Character.LearnSecretExit(room.RoomId, exitName)
		}
	}
}

// NPCMayFollowThrough reports whether the NPC c, stepping behind its leader
// through exitName of room, takes that exit (#781, owner ruling 13). An exit
// c knows (Room.ExitKnownTo): yes. A secret one c has not found: yes, and c
// learns it, when the leader is a mob (leaderUserId 0: a packmate behind its
// alpha, plan D17) or the player who charmed c; any other NPC following a
// player stops there. modules/follow and the shadow hook both ask it.
func NPCMayFollowThrough(c *characters.Character, room *rooms.Room, exitName string, leaderUserId int) bool {
	if room.ExitKnownTo(c, exitName) {
		return true
	}
	if leaderUserId > 0 && !c.IsCharmed(leaderUserId) {
		return false
	}
	LearnSecretExitFollowing(c, room, exitName)
	return true
}

// FollowExitTo is the exit key of room a follower steps through behind leader,
// who went from room to toRoomId, or "" when no exit leads there (a
// teleport). The move does not name its exit, so among the exits that lead
// there the one leader could have used wins: an exit that is not secret
// first (it teaches nothing), then a secret one leader knows, and a secret
// one leader does not know only when nothing else leads there (a scripted mob
// route takes it). Room and mutator exits (a mutator's takes the place of the
// room's, Room.GetExitInfo) come before temporary ones, then A to Z. A
// temporary exit answers with its key, which is what `go` matches, never its
// title. A nil leader knows no secret exit.
func FollowExitTo(room *rooms.Room, leader *characters.Character, toRoomId int) string {
	if room == nil {
		return ""
	}
	permanent := map[string]bool{}
	for name := range room.Exits {
		permanent[name] = true
	}
	for mut := range room.ActiveMutators {
		if spec := mut.GetSpec(); spec != nil {
			for name := range spec.Exits {
				permanent[name] = true
			}
		}
	}

	// Ranks, best first.
	const (
		rankPlain = iota
		rankTemporary
		rankSecretKnown
		rankSecretUnknown
	)
	type candidate struct {
		name string
		rank int
	}
	var found []candidate
	for name := range permanent {
		ex, ok := room.GetExitInfo(name)
		if !ok || ex.RoomId != toRoomId {
			continue
		}
		rank := rankPlain
		if ex.Secret {
			rank = rankSecretUnknown
			if room.ExitKnownTo(leader, name) {
				rank = rankSecretKnown
			}
		}
		found = append(found, candidate{name: name, rank: rank})
	}
	for name, ex := range room.ExitsTemp {
		if ex.RoomId == toRoomId {
			found = append(found, candidate{name: name, rank: rankTemporary})
		}
	}
	if len(found) == 0 {
		return ""
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].rank != found[j].rank {
			return found[i].rank < found[j].rank
		}
		return found[i].name < found[j].name
	})
	return found[0].name
}
