package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// Reason says WHY a commit or release happened. It replaces the
// characters.AggroType enum at the seam boundary.
//
// It deliberately does NOT describe what kind of engagement resulted. That is
// Engagement's job, and conflating the two is exactly how Aggro.Type ended up
// being demoted mid-round. A Reason is a fact about a moment; an Engagement is
// a fact about a state.
type Reason int

const (
	ReasonAttack    Reason = iota // ordinary engagement
	ReasonSurprise                // opened from concealment
	ReasonRetaliate               // answering an incoming attack
	ReasonTaunt                   // pulled by a taunt; see CommitTaunt
	ReasonDisengage               // leaving combat
)

// Commit enters combat with ref.
//
// U12a: delegates to characters.SetAggro, so every guard (grace period,
// taunt-hold, grapple clearing, wait rounds, ranged inference) and the
// Aggro/CombatPhase dual-write are untouched. U12b migrates the remaining
// callers here; U12c moves the guard bodies in and deletes SetAggro.
func Commit(c *characters.Character, ref state.ActorRef, r Reason) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r))
}

// CommitAfter is Commit with an explicit extra wait, replacing SetAggro's
// overloaded roundsWaitTime variadic. Only two production sites pass one;
// everything else takes weapon speed, which is what Commit does.
func CommitAfter(c *characters.Character, ref state.ActorRef, r Reason, waitRounds int) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r), waitRounds)
}

// Release leaves combat.
func Release(c *characters.Character, r Reason) {
	if c == nil {
		return
	}
	c.EndAggro()
}

// aggroTypeFor is the ONLY translation between Reason and the legacy enum,
// and it exists only until U12c deletes AggroType. Keeping it in one function
// means the deletion is a single edit rather than a sweep.
//
// Shooting is deliberately absent: SetAggro infers it from the weapon subtype,
// and duplicating that inference here would let the two disagree.
func aggroTypeFor(r Reason) characters.AggroType {
	switch r {
	case ReasonSurprise:
		return characters.SurpriseAttack
	default:
		// ReasonTaunt lands here deliberately. The taunt-hold gate pins only
		// DefaultAttack/Shooting/SurpriseAttack, so a taunt that committed as
		// anything else could not hold its own target.
		return characters.DefaultAttack
	}
}

// CommitTaunt pins c onto ref for holdRounds, then commits.
//
// ORDER IS LOAD-BEARING. The hold is set BEFORE the commit so the taunt-hold
// gate sees the new taunter as the locked target and lets this very set
// through. It is also why a newer taunt cleanly overrides an older hold.
// Reversing the two lines makes every taunt silently no-op against an
// existing hold, and nothing would fail loudly.
//
// This was characters.ForceTauntAggro. It moved here because it is a
// targeting operation, not storage: leaving it in internal/characters would
// have exempted the game's most frequent retargeting mechanic from the seam.
func CommitTaunt(c *characters.Character, ref state.ActorRef, holdRounds int) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetTauntHold(ref.UserId, ref.MobInstanceId, holdRounds)
	Commit(c, ref, ReasonTaunt)
}

// ReasonForAggroType is the inverse of aggroTypeFor: it maps a legacy
// AggroType, as produced by actions.EngageAggroType, onto the Reason a caller
// should Commit with.
//
// It exists so callers never have to name characters.SurpriseAttack
// themselves. The ambush parity guard bans that reference outright, because
// deriving the surprise type locally (from IsHidden(), say) skips the shared
// special-move cooldown the other engagement paths pay -- the exact bug U10d
// fixed. Callers pass EngageAggroType's verdict straight through here instead.
func ReasonForAggroType(t characters.AggroType) Reason {
	switch t {
	case characters.SurpriseAttack:
		return ReasonSurprise
	default:
		return ReasonAttack
	}
}
