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

	// ReasonShoot preserves an existing Shooting engagement across a re-commit.
	//
	// It exists ONLY so ReasonForAggroType can round-trip losslessly, and no
	// call site should choose it directly: SetAggro infers Shooting from the
	// equipped weapon on any DefaultAttack commit, which is how a shooting
	// engagement is normally established.
	//
	// Without it, usercommands/target.go silently changed behaviour. That gate
	// explicitly permits switching targets while Aggro.Type is Shooting, and
	// passed the existing type straight back through. Collapsing it to
	// DefaultAttack and relying on re-inference is identical ONLY while the
	// weapon is still a Shooting subtype; swap the weapon between the shot and
	// the target switch and the engagement quietly became DefaultAttack.
	ReasonShoot
)

// Commit enters combat with ref, and reports whether the engagement actually
// STARTED.
//
// ⚠️ A commit CAN be refused. Since U12c-0b the combat-phase vetoes are
// load-bearing (dead target, non-combatant, despawning, respawn grace, and the
// actor's own state), and SetAggro writes nothing when the transition is
// refused. The grace-period and taunt-hold guards refuse too, and always have.
//
// The return value exists because the alternative cost us a nil-pointer panic:
// hooks.RetargetOrEnd released aggro, committed, and returned a bare true, so
// a refused commit left Aggro nil while its callers dereferenced it. Anything
// that acts on "we are now fighting" -- messaging especially -- must consult
// this rather than assume.
//
// Ignoring the result is legal Go and is correct for the many sites that only
// want best-effort engagement. It is NOT correct for anything that then speaks
// to a player about the fight.
//
// U12a: delegates to characters.SetAggro, so every guard (grace period,
// taunt-hold, grapple clearing, wait rounds, ranged inference) and the
// Aggro/CombatPhase dual-write are untouched. U12c-2 moves the guard bodies in
// and deletes SetAggro.
func Commit(c *characters.Character, ref state.ActorRef, r Reason) bool {
	if c == nil || ref.IsZero() {
		return false
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r))
	return committedTo(c, ref)
}

// CommitAfter is Commit with an explicit extra wait, replacing SetAggro's
// overloaded roundsWaitTime variadic. Only two production sites pass one;
// everything else takes weapon speed, which is what Commit does.
//
// Returns whether the engagement started, for the reasons on Commit.
func CommitAfter(c *characters.Character, ref state.ActorRef, r Reason, waitRounds int) bool {
	if c == nil || ref.IsZero() {
		return false
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r), waitRounds)
	return committedTo(c, ref)
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
	case ReasonShoot:
		return characters.Shooting
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
// Returns whether the engagement started, for the reasons on Commit. A taunt
// that reports false pulled nobody, and must not be narrated as if it had.
func CommitTaunt(c *characters.Character, ref state.ActorRef, holdRounds int) bool {
	if c == nil || ref.IsZero() {
		return false
	}
	c.SetTauntHold(ref.UserId, ref.MobInstanceId, holdRounds)
	return Commit(c, ref, ReasonTaunt)
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
// It round-trips every type a production site can actually be holding.
// TestReasonRoundTrip pins that.
func ReasonForAggroType(t characters.AggroType) Reason {
	switch t {
	case characters.SurpriseAttack:
		return ReasonSurprise
	case characters.Shooting:
		return ReasonShoot
	default:
		return ReasonAttack
	}
}

// committedTo reports whether THIS commit landed, by checking that the target
// now on record is the one that was asked for.
//
// `c.Aggro != nil` is NOT sufficient and was the first version of this: a
// refused commit leaves the PREVIOUS engagement in place, which is non-nil, so
// every refusal reported success. The ids are compared rather than the whole
// struct because SetAggro legitimately rewrites the aggro TYPE (it re-infers
// Shooting from the equipped weapon); it never rewrites the ids.
func committedTo(c *characters.Character, ref state.ActorRef) bool {
	return c.Aggro != nil &&
		c.Aggro.UserId == ref.UserId &&
		c.Aggro.MobInstanceId == ref.MobInstanceId
}
