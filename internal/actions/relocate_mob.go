package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ClearRoomAggroOnDeparture cleans up aggro for any player or mob still in the
// room that was targeting the departing mob. Without this, characters remain
// stuck "in combat" until the next combat round validates their aggro target.
func ClearRoomAggroOnDeparture(room *rooms.Room, departingInstanceId int) {
	// Clear player aggro targeting this mob
	for _, uid := range room.GetPlayers(rooms.FindFighting) {
		u := users.GetByUserId(uid)
		if u == nil || !u.Character.IsInCombat() {
			continue
		}
		if u.Character.CurrentCombatTarget().MobInstanceId == departingInstanceId {
			// Try to retarget another hostile mob in the room
			retargeted := false
			for _, mId := range room.GetMobs(rooms.FindFighting) {
				m := mobs.GetInstance(mId)
				if m == nil || !m.Character.IsInCombat() || mId == departingInstanceId {
					continue
				}
				// Is this mob attacking us or one of our companions?
				theirTarget := m.Character.CurrentCombatTarget()
				if theirTarget.UserId == uid {
					targeting.Commit(u.Character, state.ActorRef{MobInstanceId: mId}, targeting.ReasonAttack)
					if line, ok := RetargetNotice(room, uid, state.ActorRef{MobInstanceId: mId}); ok {
						u.SendText(messaging.CategorySystem, line)
					}
					retargeted = true
					break
				}
				// Check if attacking one of our companions
				for _, comp := range u.Character.Companions {
					if comp.InstanceId > 0 && theirTarget.MobInstanceId == comp.InstanceId {
						targeting.Commit(u.Character, state.ActorRef{MobInstanceId: mId}, targeting.ReasonAttack)
						if line, ok := RetargetNotice(room, uid, state.ActorRef{MobInstanceId: mId}); ok {
							u.SendText(messaging.CategorySystem, line)
						}
						retargeted = true
						break
					}
				}
				if retargeted {
					break
				}
			}
			if !retargeted {
				targeting.Release(u.Character, targeting.ReasonDisengage)
			}
		}
	}

	// Clear mob aggro targeting the departing mob
	for _, mId := range room.GetMobs(rooms.FindFighting) {
		m := mobs.GetInstance(mId)
		if m == nil || !m.Character.IsInCombat() {
			continue
		}
		if m.Character.CurrentCombatTarget().MobInstanceId == departingInstanceId {
			targeting.Release(&m.Character, targeting.ReasonDisengage)
		}
	}
}

// RelocateMob moves a mob from one room to the room through exitName, with no
// gate and no charge: the caller has already decided the move happens.
// Walking (mobcommands.Go, after its lock checks) and a successful flee
// (hooks.handleMobFlee) both end here. It drops aggro the old room held on the
// mob, narrates the exit and the entry (sight-gated, with a sound fallback),
// plays the movement sounds, and pulls an NPC party's idle members after
// their leader.
//
// sneaking is the mover's sneaking state, read by the caller before the move
// via MobIsSneaking (mirroring the derivation usercommands.Go reads a
// player's with). A sneaking mob sends no exit line, no entry line and
// nothing to the neighbouring rooms, as a sneaking player never has (parity
// slice 6, owner ruling D1); the movement sounds play for both, as they do
// on the player path.
func RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room, sneaking bool) {
	enterFrom := `from somewhere`
	if back := dest.FindExitTo(from.RoomId); back != `` {
		enterFrom = exit.FromPhrase(back)
	}

	from.RemoveMob(mob.InstanceId)
	ClearRoomAggroOnDeparture(from, mob.InstanceId)
	dest.AddMob(mob.InstanceId)

	c := configs.GetTextFormatsConfig()

	if !sneaking {
		from.SendTextVisualWithAudio(messaging.CategoryRoomExit,
			fmt.Sprintf(string(c.ExitRoomMessageWrapper),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaves %s.`, mob.Character.Name, exit.DeparturePhrase(exitName)),
			),
			`You hear footsteps moving away.`)

		dest.SendTextVisualWithAudio(messaging.CategoryRoomEntry,
			fmt.Sprintf(string(c.EnterRoomMessageWrapper),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> enters %s.`, mob.Character.Name, enterFrom),
			),
			`You hear footsteps approaching.`)

		dest.SendTextToExits(`You hear someone moving around.`, true, from.GetPlayers(rooms.FindAll)...)
	}

	from.PlaySound(`room-exit`, `movement`)
	dest.PlaySound(`room-enter`, `movement`)

	pullMobPartyThrough(mob, from, exitName)
}

// pullMobPartyThrough re-issues the leader's exit on every idle NPC party
// member still in the old room. In-combat members stay in their fight.
func pullMobPartyThrough(leader *mobs.Mob, from *rooms.Room, exitName string) {
	// NPC party coupled movement: if this mob is leading an NPC party,
	// queue the same exit command on every party-member mob still in
	// the old room. Mirrors the player-party follow logic in
	// internal/usercommands/go.go (a leader's movement command is
	// re-issued on each member in the same room). Without this,
	// follower mobs lag many rounds behind because they only re-target
	// via the polling party_follow_leader btree action.
	//
	// Skip in-combat followers: a follower with non-nil Aggro is
	// engaged with a target. Following the leader out of the room
	// would break combat and let the attacker disengage. The patrol
	// executor pauses the leader during combat so the leader
	// shouldn't be moving anyway, but defensive in case a path step
	// from a previous tick is in flight.
	if p := parties.GetByMobInstanceId(leader.InstanceId); p != nil {
		if p.Leader != nil && p.Leader.GetMobInstanceId() == leader.InstanceId {
			for _, member := range p.Members {
				memberInstId := member.GetMobInstanceId()
				if memberInstId == 0 || memberInstId == leader.InstanceId {
					continue
				}
				memberMob := mobs.GetInstance(memberInstId)
				if memberMob == nil {
					continue
				}
				// Only follow if the member is in the leader's old room.
				if memberMob.Character.RoomId != from.RoomId {
					continue
				}
				// Don't drag in-combat members out of their fight.
				if memberMob.Character.IsInCombat() {
					continue
				}
				memberMob.Command(exitName)
			}
		}
	}
}
