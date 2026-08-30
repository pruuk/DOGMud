package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
)

// ActorRef returns this Character's identity as a state.ActorRef, carrying
// BOTH id fields.
//
// This exists because engagement_storage.go built its actor ref as
// state.ActorRef{UserId: c.userId} alone. Nothing calls SetUserId on a mob, so
// a mob's ref was the zero value and combatphase.RecordInboundAttacker
// early-returned on ActorRef.IsZero() -- which meant no mob attacking anyone
// was ever recorded. Build refs with this method, never by hand.
func (c *Character) ActorRef() state.ActorRef {
	return state.ActorRef{
		UserId:        c.userId,
		MobInstanceId: c.MobInstanceId,
	}
}

// SyncMachineSelf records this Character's identity on its CombatPhase machine,
// which uses it as the fallback actor ref when a TransitionReason carries none.
//
// Called from Validate() and SetUserId(), because identity arrives at different
// moments for the two actor types: a mob has its MobInstanceId before Validate()
// runs (mobs.go:358), while every player path assigns userId AFTER Validate()
// (users.go:520 -> :558; userrecord.go:832 -> :835), and CreateUser reaches
// SetUserId with no Validate() at all.
//
// ⚠️ There is deliberately NO registry to populate here. An earlier version of
// this file maintained an ActorRef -> Machine map; see the "Machine resolution"
// comment in internal/state/combatphase/combatphase.go for why that was wrong
// and what replaced it. This writes one plain field on this Character's own
// machine, so it cannot leak, alias another character, or go stale globally.
func (c *Character) SyncMachineSelf() {
	ref := c.ActorRef()
	if ref.IsZero() || c.CombatPhase == nil {
		return
	}
	c.CombatPhase.SetSelf(ref)
}
