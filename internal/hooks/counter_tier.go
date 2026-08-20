package hooks

// U6b Task 10 — the counter tier's spell-exit wiring. A defensive crit
// against a cast earns the defender one free counter-swing, fired here at the
// four spell quadrants (player->mob, player->player, mob->mob, mob->player).
// BOTH directions matter: wiring only the player-attacker direction would
// hand mobs a counter immunity nobody decided.

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// fireSpellCounterTier fires the counter tier at one spell exit. Spell
// targets always share the caster's room, so the reach gate passes true by
// construction (the cross-room shot — internal/actions.ExecuteFire — is the
// one uncounterable attack).
//
// The generic Task 10 narration is dispatched with the same audience routing
// the melee crit-effects use (CategoryHitMelee: the counter-swing IS a melee
// answer); Task 11 replaces it with channel-correct counter triads. Nil user
// records represent mob participants, which receive no private text.
//
// Recursion is impossible here by construction: casts are never made under
// IsCounter (the counter-swing is a melee-shaped ExecuteSkillMove, never a
// cast), and ExecuteCounter marks its own swing IsCounter.
func fireSpellCounterTier(room *rooms.Room, out combat.ChannelDefenceResult,
	channel combat.AttackChannel, defender, caster *characters.Character,
	defenderUser, casterUser *users.UserRecord) combat.CounterResult {

	if !out.DefensiveCrit {
		return combat.CounterResult{}
	}
	res := combat.ExecuteCounter(defender, caster, channel, true)
	if !res.Countered {
		return res
	}

	if defenderUser != nil && res.DefenderMsg != "" {
		defenderUser.SendText(messaging.CategoryHitMelee, res.DefenderMsg)
	}
	if casterUser != nil && res.AttackerMsg != "" {
		casterUser.SendText(messaging.CategoryHitMelee, res.AttackerMsg)
	}
	if res.RoomMsg != "" {
		exclude := []int{}
		if defenderUser != nil {
			exclude = append(exclude, defenderUser.UserId)
		}
		if casterUser != nil {
			exclude = append(exclude, casterUser.UserId)
		}
		sendVisualRoomText(room, messaging.CategoryHitMelee, res.RoomMsg, exclude...)
	}
	return res
}
