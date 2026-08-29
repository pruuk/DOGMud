package hooks

import (
	"fmt"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ValidateAggro checks if a character's aggro target still exists and is
// alive. If the target is gone or dead, targeting.Release is called and false is
// returned. Returns false (without releasing) when Aggro is already nil.
func ValidateAggro(char *characters.Character) bool {
	if !char.IsInCombat() {
		return false
	}

	ref := char.CurrentCombatTarget()

	// An engagement with no target is invalid (can happen from stale state).
	// SpellCast and Disengaging intentionally have no target — they act on
	// self or the room, not another character — so they are valid no-target
	// states. (U12c-2 dissolved the Flee aggro type into Disengaging;
	// SpellCast follows.)
	if ref.IsZero() && !char.IsDisengaging() &&
		char.Aggro.Type != characters.SpellCast {
		targeting.Release(char, targeting.ReasonDisengage)
		return false
	}

	if ref.MobInstanceId > 0 {
		target := mobs.GetInstance(ref.MobInstanceId)
		if target == nil || target.Character.Health < 1 ||
			target.Character.RoomId != char.RoomId {
			targeting.Release(char, targeting.ReasonDisengage)
			return false
		}
	}

	if ref.UserId > 0 {
		target := users.GetByUserId(ref.UserId)
		if target == nil || target.Character.Health < 1 ||
			target.Character.RoomId != char.RoomId {
			targeting.Release(char, targeting.ReasonDisengage)
			return false
		}
	}

	return true
}

// RetargetOrEnd clears the character's current aggro and scans the room for a
// new target that is already attacking us (by userId or mobInstanceId). For
// charmed companions the scan also considers mobs attacking the charm owner.
//
// Returns true only if a new target was found AND THE COMMIT LANDED.
//
// U12c-0b: "the commit landed" is a real question now. targeting.Commit is
// void and can be refused by a combat-phase veto (dead target, non-combatant,
// despawning, respawn grace). This function releases FIRST, so a refused
// commit leaves Aggro nil -- and both callers in NewRound_DoCombat.go
// dereference char.Aggro on the strength of a true return. Returning a bare
// true was a nil-pointer panic that aborted the whole round's combat
// processing for every actor, caught only by the listener recover().
func RetargetOrEnd(char *characters.Character, room *rooms.Room,
	userId int, mobInstanceId int) bool {

	targeting.Release(char, targeting.ReasonDisengage)

	// Build a set of "our side" instance IDs for companion-aware scanning.
	// If we're a player, include our companions' instance IDs so we retarget
	// mobs attacking our companions (not just mobs attacking us directly).
	myMobIds := map[int]bool{}
	if mobInstanceId > 0 {
		myMobIds[mobInstanceId] = true
	}
	if userId > 0 {
		if owner := users.GetByUserId(userId); owner != nil {
			for _, comp := range owner.Character.Companions {
				if comp.InstanceId > 0 {
					myMobIds[comp.InstanceId] = true
				}
			}
		}
	}

	// Scan players in the room first — prefer player targets over companions.
	for _, pId := range room.GetPlayers(rooms.FindFighting) {
		attackingPlayer := users.GetByUserId(pId)
		if attackingPlayer == nil || !attackingPlayer.Character.IsInCombat() {
			continue
		}

		theirTarget := attackingPlayer.Character.CurrentCombatTarget()

		// Is this player attacking us?
		if (userId > 0 && theirTarget.UserId == userId) ||
			(mobInstanceId > 0 && theirTarget.MobInstanceId == mobInstanceId) {
			return targeting.Commit(char,
				state.ActorRef{UserId: attackingPlayer.UserId}, targeting.ReasonAttack)
		}
	}

	// Scan mobs in the room that are currently fighting.
	for _, instId := range room.GetMobs(rooms.FindFighting) {
		attackingMob := mobs.GetInstance(instId)
		if attackingMob == nil || !attackingMob.Character.IsInCombat() {
			continue
		}

		theirTarget := attackingMob.Character.CurrentCombatTarget()

		// Is this mob attacking us, or any of our companions?
		if (userId > 0 && theirTarget.UserId == userId) ||
			(theirTarget.MobInstanceId > 0 && myMobIds[theirTarget.MobInstanceId]) {
			return targeting.Commit(char,
				state.ActorRef{MobInstanceId: attackingMob.InstanceId}, targeting.ReasonAttack)
		}
	}

	// For charmed companions: also retarget mobs attacking the charm owner.
	if char.IsCharmed() {
		ownerId := char.GetCharmedUserId()
		if ownerId > 0 {
			for _, instId := range room.GetMobs(rooms.FindFighting) {
				attackingMob := mobs.GetInstance(instId)
				if attackingMob == nil || !attackingMob.Character.IsInCombat() {
					continue
				}
				if attackingMob.Character.CurrentCombatTarget().UserId == ownerId {
					return targeting.Commit(char,
						state.ActorRef{MobInstanceId: attackingMob.InstanceId}, targeting.ReasonAttack)
				}
			}
		}
	}

	return false
}

// CompanionAutoTarget checks whether an idle charmed mob should enter combat
// to defend its owner. It does nothing if the mob is already fighting or is
// not charmed. AutoAssist must be set on the companion entry for it to act.
//
// Priority:
//  1. If the owner is already fighting, attack the owner's current target.
//  2. Otherwise scan the room for mobs that are attacking the owner and
//     engage the first one found.
func CompanionAutoTarget(mob *mobs.Mob, room *rooms.Room) {
	// Already fighting — nothing to do.
	if mob.Character.IsInCombat() {
		return
	}

	if !mob.Character.IsCharmed() {
		return
	}

	ownerId := mob.Character.GetCharmedUserId()
	if ownerId == 0 {
		return
	}

	owner := users.GetByUserId(ownerId)
	if owner == nil {
		return
	}

	// Grace-period defense-in-depth: if the owner is grace-protected,
	// no mob should be aggressing them (SetAggro already gates at the
	// source), and the companion has nothing to defend against.
	if owner.Character.HasBuffFlag(buffs.NoAggroTarget) {
		return
	}

	// Verify AutoAssist flag on the companion entry.
	comp := owner.Character.GetCompanionByInstanceId(mob.InstanceId)
	if comp == nil || !comp.AutoAssist {
		return
	}

	// If owner is fighting, join their fight immediately.
	if owner.Character.IsInCombat() {
		ownerTarget := owner.Character.CurrentCombatTarget()
		if ownerTarget.UserId > 0 {
			mob.Command(fmt.Sprintf("attack @%d", ownerTarget.UserId))
			return
		}
		if ownerTarget.MobInstanceId > 0 {
			mob.Command(fmt.Sprintf("attack #%d", ownerTarget.MobInstanceId))
			return
		}
	}

	// Owner is idle — scan room for mobs attacking the owner and intercept.
	for _, instId := range room.GetMobs(rooms.FindFighting) {
		attackingMob := mobs.GetInstance(instId)
		if attackingMob == nil || !attackingMob.Character.IsInCombat() {
			continue
		}
		if attackingMob.Character.CurrentCombatTarget().UserId == ownerId {
			mob.Command(fmt.Sprintf("attack #%d", attackingMob.InstanceId))
			return
		}
	}
}
