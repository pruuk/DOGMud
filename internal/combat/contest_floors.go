package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
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
// than at each reader, and since U4 that claim is unconditional: the maneuver
// floors are read HERE and nowhere else. U3 left flee.go as the last exception
// and U4 migrated it, so there is no exception any more. Deliberately stated as
// a rule and not as a site count, because a count that is right today is the
// next stale comment.
//
// Two of the migrated sites -- Position_GrappleTick.go and
// NewRound_MobRoundTick.go -- used to reach the SAME config keys through a
// private duplicate accessor, maneuverHitFloor/maneuverResistFloor, declared in
// internal/hooks/spell_resolution.go. They were therefore invisible to a grep
// for ManeuverFloors, which is how an earlier sweep missed Position_GrappleTick
// entirely. U3 deleted that duplicate pair, and internal/hooks now has no floor
// accessors of its own and does not import internal/contest at all; it reaches
// the core only through this package. Do not reintroduce a duplicate.
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
	hit, _ := ManeuverFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit)
}

// RunWithSpellFloors contests attackScore against a single defenseScore using the
// SPELL floor pair.
//
// It exists for the same reason as RunWithManeuverFloors, but on the spell
// pair's OWN numbers -- do not carry the maneuver figures across, as those
// describe maneuver sites. The standing claim is the same shape: after U3 the
// spell floors are read HERE and nowhere else. Again a rule rather than a site
// count, so it stays true as sites come and go.
//
// The readers are avoidance.go's TrySpellDeflection plus internal/hooks
// (charm_spell.go and spell_resolution.go), which previously reached the SAME
// config keys through a private spellHitFloor/spellResistFloor duplicate. U3
// deleted that duplicate, and internal/hooks now has no floor accessors of its
// own and does not import internal/contest at all. Do not reintroduce one.
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
	hit, _ := SpellFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit)
}

// ContestFloors is the GLOBAL contest floor pair, for uncertain outcomes that
// are not maneuvers and not spells: sneaking, stealing, planting, defusing,
// shadowing, noticing someone hidden.
//
// It reads dice.ContestFloors() rather than configs.GetBalanceConfig() -- unlike
// ManeuverFloors and SpellFloors, which read config directly. That asymmetry is
// deliberate and load-bearing:
//
//   - main.go seeds the dice globals from Balance.MinContestSuccessChance and
//     Balance.MinContestResistChance, so in production the two routes are the
//     same two config keys and the same values.
//   - internal/behaviortree/actions_skullduggery_test.go pins the floors with
//     dice.SetContestFloors. Reading config here would disconnect that pin.
//   - A Go test binary never loads config.yaml. The global pair therefore
//     measures 0.05 in a test (a real package var), while the maneuver and spell
//     pairs measure 0 -- the 0.05 in config.balance.misc.go is a validation
//     fallback whose condition (< 0 || > 0.50) lets the zero value through.
//     Routing this pair through config would silently change it to 0 under test.
//
// U6 owns collapsing the two routes; do not "tidy" this into a config read.
func ContestFloors() (hit, resist float64) {
	return dice.ContestFloors()
}

// RunWithGlobalFloors contests attackScore against a single defenseScore using
// the GLOBAL contest floor pair.
//
// It is the exact mirror of dice.OpposedRollStat FOR THE RETURNED BOOLEAN, and
// exists so the 17 out-of-combat contests migrated in U4 keep reading the pair
// they have always read.
//
// "For the boolean" is not hedging. contest.Run builds its rolls with dice.Roll,
// not dice.OpposedRoll, so Result.AttackRoll and Result.DefenseRoll carry
// .Success = false and .Margin = 0 ALWAYS -- where the dice path populated both
// on every call and re-stamped them on a floor flip. Read Result.Margin and
// Result.Success; never read a margin off either RollResult. Misreading this
// nearly shipped broken spell-deflection crits in U2.
//
// WHY THIS IS NOT RunWithManeuverFloors OR RunWithSpellFloors. config.yaml ships
// all three pairs at 0.05, so calling the wrong one is invisible in production:
// it passes every test and every playtest, and becomes a live balance bug the
// moment U6 retunes one pair. The pairs say different things about the COST OF A
// SINGLE FAILURE -- a maneuver burns the whole round, an out-of-combat attempt is
// one shot plus a consequence -- and that distinction is what is being preserved,
// not the number. floor_pair_guard_test.go at the repo root is the guard.
//
// The single entry is deliberately unnamed, so the returned Result.Winner is
// always "". Read Result.Contested, never Result.Winner, to ask whether a
// contest happened.
//
// TRANSITIONAL, like its two siblings: U6 may delete or reshape all three.
func RunWithGlobalFloors(attackScore, defenseScore float64) contest.Result {
	hit, _ := ContestFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit)
}
