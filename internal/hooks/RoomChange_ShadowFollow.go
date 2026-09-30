package hooks

// RoomChange_ShadowFollow.go: parity slice 6. The one place a shadower follows
// its quarry and the one place the shadow is checked on arrival, for players
// and mobs alike on both sides. It replaced a loop in usercommands/go.go
// (player movers, player shadowers only) and MobRoomChange_ShadowFollow.go
// (mob movers, player shadowers only); a mob shadower was never moved.

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowSpottedLine is what a shadower reads when it arrives in its quarry's
// room no longer hidden.
const shadowSpottedLine = "You've been spotted -- your shadow ends."

// RoomChangeShadowFollow runs two passes on every RoomChange.
//
// Follow: when the mover left through a named exit (shadowExitTo), every
// player and mob still in the old room whose shadow names the mover queues
// the same exit, if it is still hidden. A shadower whose target is set but
// whose Shadowing condition is gone is stale: its state is cleared
// (actions.ClearShadow) and it stays put. A teleport has no named exit and
// moves no one.
//
// Arrival: when the mover is itself shadowing someone now in the room it
// entered, the shadow is checked. Not hidden ends it (actions.EndShadow with
// shadowSpottedLine and the cooldown); still hidden, the quarry makes the
// sense roll (actions.ShadowSenseRoll) in that room's light.
//
// A RoomChange is only dispatched when events.ProcessEvents later pops it
// off the queue and runs its listeners one at a time, which happens after
// the code that queued the move returns, whether that was a player command,
// a fleeing mob, or anything else that moves a character. By then the
// mover's entry detection has already run, so IsHidden reflects any reveal.
// RoomChange.Unseen was captured before detection, which is why it is never
// read here.
func RoomChangeShadowFollow(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok {
		return events.Continue
	}
	if evt.UserId == 0 && evt.MobInstanceId == 0 {
		return events.Continue
	}
	shadowFollowPass(evt)
	shadowArrivalPass(evt)
	return events.Continue
}

// shadowFollower is one character in the mover's old room that may be
// shadowing it, with the queue its follow command goes on. *users.UserRecord
// and *mobs.Mob share Command's signature.
type shadowFollower struct {
	char *characters.Character
	cmd  interface{ Command(string, ...float64) }
}

func shadowFollowPass(evt events.RoomChange) {
	fromRoom := rooms.LoadRoom(evt.FromRoomId)
	if fromRoom == nil {
		return
	}
	exitName := shadowExitTo(fromRoom, evt.ToRoomId)
	if exitName == "" {
		return
	}

	var followers []shadowFollower
	for _, userId := range fromRoom.GetPlayers(rooms.FindAll) {
		if userId == evt.UserId {
			continue
		}
		if u := users.GetByUserId(userId); u != nil {
			followers = append(followers, shadowFollower{char: u.Character, cmd: u})
		}
	}
	for _, instId := range fromRoom.GetMobs(rooms.FindAll) {
		if instId == evt.MobInstanceId {
			continue
		}
		if m := mobs.GetInstance(instId); m != nil {
			followers = append(followers, shadowFollower{char: &m.Character, cmd: m})
		}
	}

	for _, f := range followers {
		userId, mobInstanceId, live := shadowOf(f.char)
		if !shadowNamesMover(evt, userId, mobInstanceId) {
			continue
		}
		if !live {
			actions.ClearShadow(f.char)
			continue
		}
		if !f.char.IsHidden() {
			continue
		}
		f.cmd.Command(exitName)
	}
}

func shadowArrivalPass(evt events.RoomChange) {
	dest := rooms.LoadRoom(evt.ToRoomId)
	if dest == nil {
		return
	}

	var shadower actions.Actor
	if evt.UserId > 0 {
		u := users.GetByUserId(evt.UserId)
		if u == nil {
			return
		}
		shadower = actions.NewUserActorInRoom(u, dest)
	} else {
		m := mobs.GetInstance(evt.MobInstanceId)
		if m == nil {
			return
		}
		shadower = actions.NewMobActorInRoom(m, dest)
	}

	userId, mobInstanceId, live := shadowOf(shadower.GetCharacter())
	if !live {
		return
	}

	var target actions.Actor
	switch {
	case userId > 0:
		u := users.GetByUserId(userId)
		if u == nil || u.Character.RoomId != evt.ToRoomId {
			return
		}
		target = actions.NewUserActorInRoom(u, dest)
	case mobInstanceId > 0:
		m := mobs.GetInstance(mobInstanceId)
		if m == nil || m.Character.RoomId != evt.ToRoomId {
			return
		}
		target = actions.NewMobActorInRoom(m, dest)
	default:
		return
	}

	if !shadower.GetCharacter().IsHidden() {
		actions.EndShadow(shadower, shadowSpottedLine)
		return
	}
	actions.ShadowSenseRoll(shadower, target, dest)
}

// shadowOf is c's shadow target and whether the shadow is live (the
// Shadowing condition is held).
func shadowOf(c *characters.Character) (userId, mobInstanceId int, live bool) {
	userId, mobInstanceId = actions.ShadowTargetOf(c)
	return userId, mobInstanceId, c.HasCondition(actions.ShadowingConditionId)
}

// shadowNamesMover reports whether a shadow target is the RoomChange's mover.
func shadowNamesMover(evt events.RoomChange, userId, mobInstanceId int) bool {
	if evt.UserId > 0 {
		return userId == evt.UserId
	}
	return mobInstanceId > 0 && mobInstanceId == evt.MobInstanceId
}

// shadowExitTo is the command word that leads from room to toRoomId, or ""
// when no exit does (a teleport). It returns the exit's map KEY, which is what
// `go` matches, looking in the permanent exits, then the temporary exits,
// then the active mutators' exits. Room.FindExitTo is not used: for a
// temporary exit it returns the Title, which need not be the key.
func shadowExitTo(room *rooms.Room, toRoomId int) string {
	for exitName, exitInfo := range room.Exits {
		if exitInfo.RoomId == toRoomId {
			return exitName
		}
	}
	for exitName, exitInfo := range room.ExitsTemp {
		if exitInfo.RoomId == toRoomId {
			return exitName
		}
	}
	for mut := range room.ActiveMutators {
		for exitName, exitInfo := range mut.GetSpec().Exits {
			if exitInfo.RoomId == toRoomId {
				return exitName
			}
		}
	}
	return ""
}
