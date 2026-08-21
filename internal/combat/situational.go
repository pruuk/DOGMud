package combat

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SituationalAttackMult composes the attacker-side situational multipliers per
// the DECLARED table below. Every cell is deliberate; absence is deliberate.
//
//	modifier            melee  ranged  specials  spell  social
//	prone attacker        Y      Y        Y        N      N     (you cast/talk fine from the ground)
//	resource depletion    Y      Y        Y        Y*     Y*    (*already applied in damage; here it reaches ACCURACY on physical channels only — see spec 2.3)
//	encumbrance           N (cost-side only, U7's domain — not an accuracy term)
//
// "Specials" ride the channel they declare: the physical maneuvers
// (bash/kick/trip/gore/...) pass ChannelMelee and fire passes ChannelRanged,
// so the two physical columns cover all sixteen of them. Both spell channels
// answer the spell column — target_defense_type: physical only changes which
// defence answers the cast, not whether the caster is casting.
//
// The implementation reads the EXISTING knobs (ProneAttackMultiplier; the
// stamina resource-penalty family via ResourceMultiplier, EffectivePoolMax
// denominator per U7 Task 11) — no new numbers, and the same terms melee's
// calcAttackScore has always applied. Prone-DEFENDER vulnerability
// (ProneVulnerabilityMultiplier) is deliberately NOT here: it is a property
// of the target, stays melee-only, and this function takes no defender.
//
// Sleeping defenders: the auto-crit snapshot (forceCrit) now reaches EVERY
// channel via ChannelDefenceResult — CLAUDE.md always promised "the entire
// first round of attacks against them auto-crits" and only melee delivered.
// See AttackSide.ForceCrit and SleepingForceCrit below.
func SituationalAttackMult(attacker *characters.Character, channel AttackChannel) float64 {
	if attacker == nil {
		return 1.0
	}
	mult := 1.0
	switch channel {
	case ChannelMelee, ChannelRanged:
		bal := configs.GetBalanceConfig()
		if attacker.IsProne() || attacker.IsSupine() {
			mult *= float64(bal.ProneAttackMultiplier)
		}
		mult *= ResourceMultiplier(attacker.Stamina,
			attacker.EffectivePoolMax(characters.PoolStamina),
			float64(bal.StaminaPenaltyMax))
	}
	return mult
}

// sleepingSnapshot is the round-start sleeping-victim snapshot, published once
// per round by hooks.DoCombat (which owns the walk over users and mob
// instances). It exists because cancel-on-damage clears the Sleeping buff on
// the FIRST hit, so every later attack in the same round would miss the
// forced-crit window if it read only the live flag. The maps are read-only
// after publication and the game loop is single-threaded, so a bare package
// var is the same discipline the rest of the engine uses.
var sleepingSnapshot struct {
	round          uint64
	userIds        map[int]bool
	mobInstanceIds map[int]bool
}

// PublishSleepingSnapshot records which users and mob instances were asleep
// when the given round began. hooks.DoCombat calls it once at the top of every
// combat round; SleepingForceCrit consults it for the rest of that round.
func PublishSleepingSnapshot(round uint64, userIds map[int]bool, mobInstanceIds map[int]bool) {
	sleepingSnapshot.round = round
	sleepingSnapshot.userIds = userIds
	sleepingSnapshot.mobInstanceIds = mobInstanceIds
}

// SleepingForceCrit reports whether an attack resolving NOW against defender
// falls under the sleeping-victim auto-crit contract: the defender carries the
// live Sleeping flag (a command-driven attack caught them still asleep), OR
// they were asleep when the current round began (the DoCombat snapshot), so a
// victim woken by this round's first hit still eats the rest of the round's
// attacks as crits. Callers thread the verdict through AttackSide.ForceCrit.
//
// A stale snapshot (any other round) says nothing: only the live flag answers
// then. Nil defenders never force anything.
func SleepingForceCrit(defender *characters.Character) bool {
	if defender == nil {
		return false
	}
	if defender.HasBuffFlag(buffs.Sleeping) {
		return true
	}
	if sleepingSnapshot.round != util.GetRoundCount() {
		return false
	}
	if defender.MobInstanceId > 0 {
		return sleepingSnapshot.mobInstanceIds[defender.MobInstanceId]
	}
	if uid := defender.GetUserId(); uid > 0 {
		return sleepingSnapshot.userIds[uid]
	}
	return false
}
