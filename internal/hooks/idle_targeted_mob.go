package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// mobIsTargetedInRoom reports whether any player standing in roomId currently
// has mobInstanceId as their combat target.
//
// Chunk 5.14. The `attack` command sets only the PLAYER's aggro
// (usercommands/attack.go). The mob does not learn it is engaged until combat
// resolves on the NEXT round, in Phase 7 handleAggroAndAssist
// (NewRound_DoCombat_unified.go). Because IdleMobs gated purely on the mob's own
// IsInCombat(), that left a one-round window in which the mob was still "idle":
// a goal planner could move it out of the room, the queued attack then found no
// target, and no aggro was ever established. The loop repeated indefinitely --
// reported from play as ten minutes of chasing a shaman with zero hits landed.
// The "slight delay before the fight starts" is the same window from the other
// side.
//
// Checking who is TARGETING the mob rather than marking the mob at attack time
// covers every engagement path -- melee, spells, ranged -- instead of only the
// one command.
//
// The two lookups are injected so this is unit-testable without the room and
// user registries (users.userManager is package-private). This follows the
// callback-injection pattern already used for rooms.SetCompanionTransport.
func mobIsTargetedInRoom(
	mobInstanceId int,
	roomId int,
	playersIn func(roomId int) []int,
	targetOf func(userId int) state.ActorRef,
) bool {
	// A player with no target yields a zero-value ActorRef, whose
	// MobInstanceId is 0. Without this guard a mob with a zero instance id
	// would match every idle player and pin itself forever.
	if mobInstanceId < 1 {
		return false
	}

	for _, userId := range playersIn(roomId) {
		if targetOf(userId).MobInstanceId == mobInstanceId {
			return true
		}
	}

	return false
}

// roomPlayerIds is the production adapter for mobIsTargetedInRoom's player
// lookup.
func roomPlayerIds(roomId int) []int {
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return nil
	}
	return room.GetPlayers()
}

// userCombatTarget is the production adapter for mobIsTargetedInRoom's target
// lookup. A missing user yields the zero ActorRef, which cannot match a valid
// mob instance id.
func userCombatTarget(userId int) state.ActorRef {
	user := users.GetByUserId(userId)
	if user == nil {
		return state.ActorRef{}
	}
	return user.Character.CurrentCombatTarget()
}
