package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A bonded companion harms only what her owner could harm. The aicompanion
// module checks a single target when she starts a harmful spell
// (harmAllowed), but an area spell folds over several rounds and lands on
// whoever is in the room when it resolves, not whoever was there when she
// began. So this filter is the authority on who an area harm spell cast by
// a bonded companion lands on (the module's areaHarmAllowed only asks that
// it would land on someone, and on nobody she refuses to fight). It is
// applied here, at resolution, against each target, asked as her owner: a
// creature by mobs.CheckPlayerHarm (as a player's own area spell is,
// playerHarmTargetPermitted), a person by (*rooms.Room).CanPvp with her
// owner as the attacker and not a member of her owner's party.
//
// The engine's own functions answer, with the owner read off the charm
// (GetCharmedUserId), so no seam is needed; companionai.IsBondedCompanion
// says whether the caster is bonded and is false while the module is off,
// when a bonded companion is an ordinary one and nothing changes.

// mobAreaHarmTargets is who a mob's area harm spell lands on when it
// resolves: every creature in the room but itself and non-combatants, and
// every person. A charmed caster spares its owner and its owner's other
// companions; a bonded one also spares whatever its owner could not harm.
func mobAreaHarmTargets(caster *mobs.Mob, room *rooms.Room) (mobIds []int, userIds []int) {
	return mobAreaHarmTargetsSparing(caster, room, false)
}

// mobAreaHarmTargetsSparing is mobAreaHarmTargets for a spell that may spare
// its caster's own side (SpellData.SparesAllies, opt-in per spell: a boss
// whose blast should miss its own adds).
func mobAreaHarmTargetsSparing(caster *mobs.Mob, room *rooms.Room, spareAllies bool) (mobIds []int, userIds []int) {
	charmedByUserId := caster.Character.GetCharmedUserId()
	owner, bonded := bondedAreaHarmOwner(caster)

	allMobs := room.GetMobs(rooms.FindAll)
	mobIds = make([]int, 0, len(allMobs))
	for _, mId := range allMobs {
		if mId == caster.InstanceId {
			continue // don't target self
		}
		// If this mob is charmed by a player, don't hit that player's other companions
		// Also never hit non-combatant mobs (shopkeepers etc.)
		if m := mobs.GetInstance(mId); m != nil {
			if m.IsNonCombatant() {
				continue
			}
			if charmedByUserId > 0 && m.Character.IsCharmed(charmedByUserId) {
				continue
			}
			if bonded && !bondedMayHarmMob(m) {
				continue
			}
			// A spell that spares allies misses the caster's own side: an
			// uncharmed mob sharing one of its groups and not fighting it.
			if spareAllies && charmedByUserId == 0 && !bonded && !m.Character.IsCharmed() &&
				sharesGroup(caster, m) && m.Character.CurrentCombatTarget().MobInstanceId != caster.InstanceId {
				continue
			}
		}
		mobIds = append(mobIds, mId)
	}

	allUsers := room.GetPlayers(rooms.FindAll)
	userIds = make([]int, 0, len(allUsers))
	for _, pId := range allUsers {
		// If charmed, don't hit the owner
		if charmedByUserId > 0 && pId == charmedByUserId {
			continue
		}
		if bonded && !bondedMayHarmPlayer(owner, room, pId) {
			continue
		}
		// A wild caster does not know an undetected sneaker is there.
		if charmedByUserId == 0 && !bonded {
			if u := users.GetByUserId(pId); u != nil && u.Character.IsHidden() {
				continue
			}
		}
		userIds = append(userIds, pId)
	}
	return mobIds, userIds
}

// bondedAreaHarmOwner returns the owner a mob answers to for area harm, and
// whether it answers to one at all. An owner who is not online answers for
// nothing, so a bonded caster whose owner cannot be read harms no person.
func bondedAreaHarmOwner(caster *mobs.Mob) (*users.UserRecord, bool) {
	ownerId := caster.Character.GetCharmedUserId()
	if ownerId <= 0 || !companionai.IsBondedCompanion(caster.InstanceId) {
		return nil, false
	}
	return users.GetByUserId(ownerId), true
}

// bondedMayHarmMob reports whether a bonded companion's area harm may land
// on this creature: only if her owner's own area spell could.
func bondedMayHarmMob(target *mobs.Mob) bool {
	return !mobs.CheckPlayerHarm(target).Blocked()
}

// bondedMayHarmPlayer reports whether a bonded companion's area harm may
// land on this person: only if her owner could fight them here.
func bondedMayHarmPlayer(owner *users.UserRecord, room *rooms.Room, targetUserId int) bool {
	if owner == nil || owner.Character == nil || targetUserId == owner.UserId {
		return false
	}
	target := users.GetByUserId(targetUserId)
	if target == nil || target.Character == nil {
		return false
	}
	if room.CanPvp(owner, target) != nil {
		return false
	}
	if p := parties.Get(owner.UserId); p != nil && p.IsMember(targetUserId) {
		return false
	}
	return true
}

// sharesGroup reports whether two mobs name a group in common (mobs.Mob
// Groups: the side a mob identifies with).
func sharesGroup(a, b *mobs.Mob) bool {
	for _, ga := range a.Groups {
		if ga == `` {
			continue
		}
		for _, gb := range b.Groups {
			if ga == gb {
				return true
			}
		}
	}
	return false
}
