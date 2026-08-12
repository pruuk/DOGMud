package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// Contest floors for combat maneuvers (roadmap chunk 5.9c).
//
// The floor value tracks the COST OF A SINGLE FAILURE, which is the principle
// the whole 5.9 arc settled on:
//
//	melee swing        fraction of a round, many per fight   0.15
//	maneuver / spell   consumes the WHOLE round              0.05
//	out-of-combat      one shot, plus a consequence          0.05
//
// So maneuvers sit with spells, not with the swing they superficially resemble.
// A grapple that fails costs the round; a swing that misses costs a slice of one.
func ManeuverFloors() (hit, resist float64) {
	bal := configs.GetBalanceConfig()
	return float64(bal.MinManeuverHitChance), float64(bal.MinManeuverResistChance)
}

// SpellFloors is the spell pair.
//
// Spells carry their own pair rather than reusing the non-combat contest floors
// because the cost of a failure is different in kind: a fizzle burns the
// caster's round, and more than one round for a long cast. The hit floor keeps
// an outmatched caster from being guaranteed to waste rounds; the resist floor
// keeps an outmatched TARGET from being auto-hit with no agency, which matters
// because mobs cast at players too (see hooks.resolveMobSpellAgainstPlayer).
//
// It also serves combat-side abilities that resolve against a spell rather than
// against a maneuver -- TrySpellDeflection is one. Using the maneuver pair there
// would be a coincidence of value, not a shared rule.
func SpellFloors() (hit, resist float64) {
	bal := configs.GetBalanceConfig()
	return float64(bal.MinSpellHitChance), float64(bal.MinSpellResistChance)
}

// RunWithManeuverFloors contests attackScore against a single defenseScore using
// the MANEUVER floor pair.
//
// It exists so that the maneuver pair is fetched in one obvious place rather
// than at each reader. U3 migrated every maneuver site onto it except the two
// in flee.go, which a later chunk owns; those still call ManeuverFloors()
// directly.
//
// Two of the migrated sites -- Position_GrappleTick.go and
// NewRound_MobRoundTick.go -- used to reach the SAME config keys through a
// private duplicate accessor, maneuverHitFloor/maneuverResistFloor, declared in
// internal/hooks/spell_resolution.go. They were therefore invisible to a grep
// for ManeuverFloors, which is how an earlier sweep missed Position_GrappleTick
// entirely. U3 deleted that duplicate pair; do not reintroduce one.
//
// The collapse is partial by design: ManeuverFloors stays exported, and a
// caller needing a best-of-N defence must still hand-roll contest.RunWithFloors
// because this takes a single defenseScore. The goal is one obvious path for
// the common case, not a guarantee that no site can drift.
//
// NAMED FOR THE FLOOR PAIR, NOT A CHANNEL, DELIBERATELY. The callers do not
// share a damage channel: TryStoicResolve is the CONVICTION channel and still
// uses the maneuver pair. Nor do they share a skill-weight convention --
// bash/kick/trip/ranged/grapple run at x1, submissions at x1.5, grapple drift at
// x2.2/x2.0, and taunt at x5. Do NOT read shared use of this wrapper as
// permission to retune these callers together; the only thing they share is the
// floor pair, which is a statement about the cost of a single failure.
//
// The single entry is deliberately unnamed, so the returned Result.Winner is
// always "". Read Result.Contested, never Result.Winner, to ask whether a
// contest happened.
//
// TRANSITIONAL, mirroring contest.RunWithFloors, which carries the same warning:
// U6 may delete or reshape both when it reconciles the two floor styles. Do not
// build new permanent behaviour on this without checking where that landed.
func RunWithManeuverFloors(attackScore, defenseScore float64) contest.Result {
	hit, resist := ManeuverFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit, resist)
}

// RunWithSpellFloors contests attackScore against a single defenseScore using the
// SPELL floor pair.
//
// It exists for the same reason as RunWithManeuverFloors, but on the spell
// pair's OWN numbers -- do not carry the maneuver figures across, as those
// describe maneuver sites. U3 migrated all six spell-floor readers onto it:
// avoidance.go's TrySpellDeflection, plus the five in internal/hooks
// (charm_spell.go and four sites in spell_resolution.go) that previously went
// through a private spellHitFloor/spellResistFloor duplicate, now deleted.
//
// As with its sibling the collapse is partial: SpellFloors stays exported, and
// best-of-N callers still hand-roll contest.RunWithFloors because this takes a
// single defenseScore.
//
// NAMED FOR THE FLOOR PAIR, NOT A CHANNEL, DELIBERATELY. Its callers are the
// ones that resolve against a SPELL rather than against a maneuver -- and the
// converse holds too: sharing the maneuver pair with TryStoicResolve, which is
// the conviction channel, would be a coincidence of value rather than a shared
// rule. Callers here likewise do not share a skill-weight convention, so this
// wrapper is not a licence to retune them together.
//
// The single entry is deliberately unnamed, so the returned Result.Winner is
// always "". Read Result.Contested, never Result.Winner, to ask whether a
// contest happened.
//
// TRANSITIONAL, mirroring contest.RunWithFloors: U6 may delete or reshape both.
func RunWithSpellFloors(attackScore, defenseScore float64) contest.Result {
	hit, resist := SpellFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit, resist)
}
