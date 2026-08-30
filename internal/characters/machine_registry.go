package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// ActorRef returns this Character's identity as a state.ActorRef, carrying
// BOTH id fields.
//
// This exists because engagement_storage.go built its actor ref as
// state.ActorRef{UserId: c.userId} alone. Nothing calls SetUserId on a mob, so
// a mob's ref was the zero value and combatphase.RecordInboundAttacker
// early-returned on ActorRef.IsZero() -- a second, independent break sitting
// behind the empty machine registry. Build refs with this method, never by
// hand.
func (c *Character) ActorRef() state.ActorRef {
	return state.ActorRef{
		UserId:        c.userId,
		MobInstanceId: c.MobInstanceId,
	}
}

// syncMachineRegistry binds this Character's five state machines to its
// ActorRef in the per-package registries. It is idempotent and safe to call in
// any order, which it has to be:
//
//	player login   Validate(true) THEN SetUserId   users.go:516 -> :551
//	alt switch     Validate()     THEN SetUserId   userrecord.go:832 -> :835
//	new character  SetUserId with NO Validate()    users.go:443 -> :461
//	mob spawn      MobInstanceId  THEN Validate()  mobs.go:358
//
// so it is called from BOTH Validate() and SetUserId() and converges on one
// correct binding whichever runs first.
//
// A ZERO REF IS NEVER ADMITTED. The registry is a map keyed by ActorRef, so
// the zero ref is a SINGLE key: admitting it would alias every unidentified
// character onto one entry, and lookupMachine would hand combat another
// character's machines. That is strictly worse than the empty registry this
// replaces, and it is exactly what registering from fireCharacterCreated alone
// would have done to every player.
func (c *Character) syncMachineRegistry() {
	ref := c.ActorRef()
	if ref.IsZero() {
		return
	}

	// Identity changed (alt switch, or a late SetUserId after a mob-shaped
	// load): drop the stale binding before creating the new one.
	if !c.registeredRef.IsZero() && c.registeredRef != ref {
		c.unregisterMachinesAt(c.registeredRef)
	}

	// Nil machines are legitimate here -- CreateUser reaches SetUserId before
	// any Validate() has built them. The later Validate() re-syncs.
	if c.CombatPhase != nil {
		combatphase.RegisterMachine(ref, c.CombatPhase)
	}
	if c.Awareness != nil {
		awareness.RegisterMachine(ref, c.Awareness)
	}
	if c.Life != nil {
		life.RegisterMachine(ref, c.Life)
	}
	if c.Activity != nil {
		activity.RegisterMachine(ref, c.Activity)
	}
	if c.Position != nil {
		position.RegisterMachine(ref, c.Position)
	}

	c.registeredRef = ref
}

// UnregisterMachines drops this Character's registry bindings. Call it on
// logout and on mob despawn.
//
// The registry is process-global and lives for the life of the server, so
// without this every mob instance ever spawned is retained forever. Wiring
// registration without teardown trades a dead feature for a slow leak.
func (c *Character) UnregisterMachines() {
	c.unregisterMachinesAt(c.registeredRef)
	c.registeredRef = state.ActorRef{}
}

func (c *Character) unregisterMachinesAt(ref state.ActorRef) {
	if ref.IsZero() {
		return
	}
	combatphase.UnregisterMachine(ref)
	awareness.UnregisterMachine(ref)
	life.UnregisterMachine(ref)
	activity.UnregisterMachine(ref)
	position.UnregisterMachine(ref)
}
