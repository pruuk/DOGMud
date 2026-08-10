package mobs

// HarmBlock is the reason a player-initiated harmful action against a mob must
// be refused. HarmAllowed means the action may proceed.
type HarmBlock uint8

const (
	// HarmAllowed means no protection applies and the action may proceed.
	HarmAllowed HarmBlock = iota
	// HarmBlockedCompanion means the mob is somebody's charmed companion.
	HarmBlockedCompanion
	// HarmBlockedNonCombatant means the mob is flagged non_combatant and
	// cannot be attacked, stolen from, or targeted by harm spells.
	HarmBlockedNonCombatant
	// HarmBlockedAttackImmune means the mob is flagged player_attack_immune:
	// players may not attack it, though it can still fight them.
	HarmBlockedAttackImmune
)

// Blocked reports whether this result refuses the action.
func (h HarmBlock) Blocked() bool { return h != HarmAllowed }

// CheckPlayerHarm is the single authorization policy for player-initiated
// harmful actions: melee, special moves, ranged fire, thrown weapons, theft,
// item procs, target switching, and harmful spells at both cast time and
// resolution time.
//
// Before this existed, each path re-implemented the policy inline and they had
// drifted: melee blocked non-combatants and attack-immune mobs, harmful
// single-target casting blocked only companions and non-combatants, and
// harmful multi-target and area casting blocked nothing at all. A quest or
// tutorial NPC protected from melee could still be killed with a spell (review
// finding 3).
//
// Companion outranks the other reasons so the player gets the most informative
// message. Any player's companion is off-limits, not only the actor's own.
//
// A nil mob returns HarmAllowed: callers nil-check their own registry lookups,
// and a target that does not exist must not produce a rejection message about
// a protection it does not have.
func CheckPlayerHarm(m *Mob) HarmBlock {
	if m == nil {
		return HarmAllowed
	}
	if m.Character.IsCharmed() {
		return HarmBlockedCompanion
	}
	if m.IsNonCombatant() {
		return HarmBlockedNonCombatant
	}
	if m.PlayerAttackImmune {
		return HarmBlockedAttackImmune
	}
	return HarmAllowed
}
