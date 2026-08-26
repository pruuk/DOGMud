package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// AttackTarget is the resolved target from FindAttackTarget.
// Exactly one of UserId / MobInstanceId will be non-zero when Found is true.
type AttackTarget struct {
	UserId        int
	MobInstanceId int
	Found         bool
}

// FindAttackTarget resolves a non-empty target string into an AttackTarget.
//
// It handles wildcard patterns (`*`, `*mob`, `*user`/`*ANYONE`) and named
// targets via room.FindByName. Self-attack is prevented using the caller's
// own IDs (actorUserId for player actors, actorMobInstanceId for mob actors).
//
// Auto-target resolution (empty rest) is NOT handled here — that logic
// diverges significantly between user and mob sides and must stay in each
// wrapper. Call this only when rest is non-empty.
//
// The Hidden → SurpriseAttack promotion is NOT done here either — call
// EngageAggroType (below) for that, and leave the Hidden buff for the combat
// loop's CancelIfCombat pass.
func FindAttackTarget(rest string, room *rooms.Room, actorUserId int, actorMobInstanceId int) AttackTarget {

	result := AttackTarget{}

	if rest[0] == '*' { // choose a target at random — friend or foe

		if rest == `*` { // * ANYONE

			// Mobs exclude themselves from the mobs pool; users exclude
			// themselves from the players pool. Build both lists with the
			// appropriate exclusion applied.
			allMobs := []int{}
			for _, mobInstanceId := range room.GetMobs() {
				if mobInstanceId == actorMobInstanceId {
					continue // mob can't target itself
				}
				allMobs = append(allMobs, mobInstanceId)
			}

			allPlayers := []int{}
			for _, userId := range room.GetPlayers() {
				if userId == actorUserId {
					continue // user can't target themselves
				}
				allPlayers = append(allPlayers, userId)
			}

			total := len(allMobs) + len(allPlayers)
			if total == 0 {
				return result
			}

			randomSelection := util.Rand(total)
			if randomSelection < len(allMobs) {
				result.MobInstanceId = allMobs[randomSelection]
				result.Found = true
			} else {
				randomSelection -= len(allMobs)
				result.UserId = allPlayers[randomSelection]
				result.Found = true
			}

		} else if rest == `*mob` { // *mob — ANY MOB (excluding self for mobs)

			allMobs := []int{}
			for _, mobInstanceId := range room.GetMobs() {
				if mobInstanceId == actorMobInstanceId {
					continue
				}
				allMobs = append(allMobs, mobInstanceId)
			}

			if len(allMobs) > 0 {
				result.MobInstanceId = allMobs[util.Rand(len(allMobs))]
				result.Found = true
			}

		} else { // *user / *ANYONE / any other * prefix — ANY PLAYER (excluding self for users)

			allPlayers := []int{}
			for _, userId := range room.GetPlayers() {
				if userId == actorUserId {
					continue
				}
				allPlayers = append(allPlayers, userId)
			}

			if len(allPlayers) > 0 {
				result.UserId = allPlayers[util.Rand(len(allPlayers))]
				result.Found = true
			}
		}

	} else {
		// Named target — delegate to room search
		playerId, mobInstanceId := room.FindByName(rest)
		result.UserId = playerId
		result.MobInstanceId = mobInstanceId
		result.Found = (playerId > 0 || mobInstanceId > 0)
	}

	// Self-attack prevention
	if result.UserId == actorUserId && actorUserId != 0 {
		result.UserId = 0
		result.Found = result.MobInstanceId > 0
	}
	if result.MobInstanceId == actorMobInstanceId && actorMobInstanceId != 0 {
		result.MobInstanceId = 0
		result.Found = result.UserId > 0
	}

	return result
}

// EngageAggroType reports the Aggro type a new engagement should carry, and
// claims the surprise opener's cost.
//
// A hidden attacker whose special-move cooldown is unavailable opens as an
// ORDINARY attack. That contract predates U10d: callers must not pre-check
// IsHidden and must not assume hidden implies surprise.
//
// U10d: the pre-combat burst this used to fire is gone. The opening strike of
// the ordinary combat round is the surprise now.
//
// surpriseOnCooldown separates the TWO reasons aggroType can come back as
// DefaultAttack. Without it a caller cannot tell "this attacker was never
// hidden" from "this attacker WAS hidden and the shared special-move timer
// refused the opener" — and the second has to be spoken. SetAggro cascades
// Hidden -> Revealing whatever the aggro type is (internal/hooks/
// Awareness_Cascades.go), so a refused melee ambusher spends their cover and
// gains nothing; silence there reads as a broken feature rather than a
// cooldown. This is the same signal FireResult.SurpriseOnCooldown carries on
// the ranged half (combat_fire.go), deliberately named the same so both halves
// of the ambush grep as one feature.
//
// It is returned as a second value rather than folded into a result struct so
// that the SetAggro argument at every call site stays a plain variable assigned
// straight from this call — which is exactly the shape ambush_parity_guard_test
// enforces across all three engagement paths.
//
// surpriseOnCooldown is true ONLY when the attacker was hidden. An ordinary
// attacker asked for no opener and must not be told one was refused.
func EngageAggroType(actor Actor, target Actor) (aggroType characters.AggroType, surpriseOnCooldown bool) {
	if target == nil || actor.GetCharacter() == nil {
		return characters.DefaultAttack, false
	}
	if !actor.GetCharacter().IsHidden() {
		return characters.DefaultAttack, false
	}
	cfg := configs.GetBalanceConfig()
	if !actor.GetCharacter().TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return characters.DefaultAttack, true
	}
	return characters.SurpriseAttack, false
}
