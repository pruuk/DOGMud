package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// EngageLines is one coordinated draw from the attacker's prepare pool: the
// attacker's, the defender's and the room's line for the same moment, and
// the hit band they ride.
type EngageLines struct {
	Category messaging.Category
	Actor    string
	Actee    string
	Observer string
}

// EngageMessages draws the engage moment from the attacker's prepare pool
// (owner ruling A, spec 2026-10-16 #541 section 2). The pool is chosen by
// AttackSubtype directly, never meleeDisplaySubtype, so a bow user squares
// up with shooting.yaml's own prepare lines, as its wait lines already do.
// It does not count as an attack for the stance word (only a wait round
// does). An unloaded store returns empty lines and the caller keeps its
// plain line.
func EngageMessages(attacker, defender *characters.Character, attackerType, defenderType SourceTarget) EngageLines {
	subtype := AttackSubtype(attacker)
	msgs := items.GetPreAttackMessage(subtype, items.Prepare)
	roles := msgs.Together.Render(attacker.GetCombatSkillLevel(),
		preAttackTokens(attacker, defender, attackerType, defenderType), nil)
	return EngageLines{
		Category: CategoryForWeaponSubtype(subtype),
		Actor:    roles.Actor,
		Actee:    roles.Actee,
		Observer: roles.Observer,
	}
}

// EngageLineFor picks what one participant reads: the authored engage line
// at full sight, otherwise the plain line with the other party's name hidden
// at that reader's sight (D2: an authored line names the weapon, which a
// reader who cannot see must not learn; the wait round makes the same call
// in hooks.handleCombatWaitRound). An empty authored line also falls back.
func EngageLineFor(sight messaging.SightDecision, authored, plain string, names []string) string {
	if sight == messaging.SightFull && authored != "" {
		return authored
	}
	return messaging.HideNames(plain, names, sight)
}
