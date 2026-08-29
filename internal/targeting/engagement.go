package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
)

// Engagement is the one place to ask "what kind of engagement is this?".
//
// The STORED/DERIVED split below is load-bearing and must not be "optimised".
//
//   - Ranged is DERIVED, so it is never stored: SetAggro already re-infers it
//     from the weapon subtype on every single call.
//   - OpeningUnspent is STORED and is NOT derivable from anything. U10d made
//     stealth break immediately, so "this engagement opened from concealment"
//     survives only as remembered state. Deriving it from IsHidden() would
//     reintroduce the bug U10d fixed.
type Engagement struct {
	Phase          combatphase.State // STORED
	Target         state.ActorRef    // STORED
	OpeningUnspent bool              // STORED  ambush opening not yet thrown
	Casting        bool              // DERIVED from the activity machine
	Ranged         bool              // DERIVED from the equipped weapon subtype

	// SpellTargets carries the aimed targets of a spell-cast aggro, which do
	// NOT live in Target.
	//
	// characters.SetCast (spells.go:208) builds Aggro{Type: SpellCast,
	// SpellInfo: ...} and never writes UserId or MobInstanceId, so they stay
	// zero for the whole cast. Reading only Target would report "no target"
	// for a mob mid-cast that IsAggro (combat_state_compat.go:191) happily
	// reports as aggro'd, which is a live disagreement with production, not a
	// theoretical one.
	//
	// Target and SpellTargets are therefore both consulted by IsAimedAt.
	// Callers that only look at Target will be wrong about casters.
	SpellTargets []state.ActorRef
}

// IsAimedAt reports whether this engagement is directed at ref, mirroring
// characters.IsAggro's semantics: the plain target first, then the spell-cast
// target lists.
//
// It exists so consumers cannot accidentally reimplement the half of that
// check that only looks at Target -- which is exactly the bug an adversarial
// review caught in the first draft of this package.
func (e Engagement) IsAimedAt(ref state.ActorRef) bool {
	if ref.IsZero() {
		return false
	}
	if e.Target == ref {
		return true
	}
	for _, t := range e.SpellTargets {
		if t == ref {
			return true
		}
	}
	return false
}

// EngagementOf composes the authoritative sources at read time.
//
// It is PURE. It stores nothing and consumes nothing, so there is no value to
// go stale and nothing to demote. Spending the opening strike is
// ConsumeOpeningStrike's job and has exactly one caller.
//
// U12a NOTE: Target and OpeningUnspent read Character.Aggro, because that is
// what the 294 production read sites do today and this slice changes no
// behaviour. U12c repoints them at CombatPhase and deletes Aggro.
func EngagementOf(c *characters.Character) Engagement {
	e := Engagement{Phase: combatphase.Idle}
	if c == nil {
		return e
	}

	if c.CombatPhase != nil {
		e.Phase = c.CombatPhase.State()
	}
	if c.Aggro != nil {
		e.Target = state.ActorRef{
			UserId:        c.Aggro.UserId,
			MobInstanceId: c.Aggro.MobInstanceId,
		}
		e.OpeningUnspent = c.Aggro.Type == characters.SurpriseAttack

		// A spell-cast aggro carries its targets in SpellInfo and leaves the
		// plain ids zero. Mirrors the SpellCast branch of IsAggro.
		if c.Aggro.Type == characters.SpellCast {
			for _, uId := range c.Aggro.SpellInfo.TargetUserIds {
				e.SpellTargets = append(e.SpellTargets, state.ActorRef{UserId: uId})
			}
			for _, mId := range c.Aggro.SpellInfo.TargetMobInstanceIds {
				e.SpellTargets = append(e.SpellTargets, state.ActorRef{MobInstanceId: mId})
			}
		}
	}
	if c.Activity != nil {
		e.Casting = c.Activity.IsCasting()
	}
	e.Ranged = c.Equipment.Weapon.GetSpec().Subtype == items.Shooting

	return e
}

// ConsumeOpeningStrike spends the ambush opening and reports whether it was
// there to spend. It is the ONE deliberate side effect in this package and
// must have exactly one caller: the swing loop, on the swing that is THROWN.
//
// It exists as a separate call precisely because EngagementOf is pure. Today
// calculateCombat reads Aggro.Type and demotes it in the same breath, which is
// why U10d had to add AttackResult.WasSurpriseAttack to carry the fact past
// the read. Splitting the query from the consumption is what stops a casual
// reader spending an ambush by asking about it.
//
// The engagement itself survives: only the opening is spent.
//
// U12a NOTE: this has NO production caller yet. calculateCombat keeps doing
// its own read-and-demote until U12c, which is a behavioural site and out of
// this slice's scope. It is built and tested here so the whole API can be
// reviewed as one thing.
func ConsumeOpeningStrike(c *characters.Character) bool {
	if c == nil || c.Aggro == nil || c.Aggro.Type != characters.SurpriseAttack {
		return false
	}
	c.SetAggro(c.Aggro.UserId, c.Aggro.MobInstanceId, characters.DefaultAttack)
	return true
}
