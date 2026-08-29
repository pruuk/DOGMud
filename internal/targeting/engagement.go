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
	}
	if c.Activity != nil {
		e.Casting = c.Activity.IsCasting()
	}
	e.Ranged = c.Equipment.Weapon.GetSpec().Subtype == items.Shooting

	return e
}
