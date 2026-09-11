package hooks

// U6b Task 10 — the counter tier's spell-exit wiring. A defensive crit
// against a cast earns the defender one free counter-swing, fired here at the
// four spell quadrants (player->mob, player->player, mob->mob, mob->player).
// BOTH directions matter: wiring only the player-attacker direction would
// hand mobs a counter immunity nobody decided.

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
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
// The channel-correct counter narration (U6b Task 11, the counter-quell pool:
// the working put down, the gap stepped through) is dispatched with the same
// audience routing the melee crit-effects use (CategoryHitMelee: the
// counter-swing IS a melee answer). Dispatching here is ordering-correct for
// spells: the cast's own outcome narration has already been sent by the time
// these exits fire. Nil user records represent mob participants, which
// receive no private text.
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

	var countered messaging.Recipient
	counteredId := 0
	if casterUser != nil {
		countered = casterUser
		counteredId = casterUser.UserId
	}
	actions.SendCounterTrio(room, res, countered, counteredId)
	return res
}
