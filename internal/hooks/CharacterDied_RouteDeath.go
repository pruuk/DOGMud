package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RouteAttributedDeath resolves a death queued by ApplyHarm.
//
// Death is resolved HERE rather than at the harm site because Die fires its
// observers synchronously and Death_MobInstanceCleanup despawns the instance
// inside that call. Killing inline would pull instances out from under any loop
// damaging several targets -- the AoE loop in usercommands.Throw is a live
// example.
//
// This is also the single place the prechecks Die's doc used to delegate to
// callers now live. They were not in fact handled at each call site: only the
// suicide commands checked ReviveOnDeath, so the buff was inert on every combat
// and damage-over-time death before U5c.
func RouteAttributedDeath(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.CharacterDied)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "CharacterDied", "Actual Type", e.Type())
		return events.Cancel
	}

	char := resolveDyingCharacter(evt)
	if char == nil {
		// The victim went away between the blow and the flush. Nothing to do;
		// Die would be a no-op anyway.
		return events.Continue
	}

	// Something else already resolved this death. Clear the marker so the
	// character is not left permanently unkillable.
	if !char.IsAlive() {
		char.DeathQueued = false
		return events.Continue
	}

	if char.HasBuffFlag(buffs.ReviveOnDeath) {
		reviveInsteadOfDeath(char)
		char.DeathQueued = false
		return events.Continue
	}

	char.Die(state.ActorRef{
		UserId:        evt.KillerUserId,
		MobInstanceId: evt.KillerMobInstanceId,
	}, evt.Trigger)

	char.DeathQueued = false

	return events.Continue
}

// resolveDyingCharacter re-resolves the victim at flush time. It may be gone.
func resolveDyingCharacter(evt events.CharacterDied) *characters.Character {

	if evt.MobInstanceId != 0 {
		if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
			return &m.Character
		}
		return nil
	}

	if evt.UserId != 0 {
		if u := users.GetByUserId(evt.UserId); u != nil {
			return u.Character
		}
	}

	return nil
}

// reviveInsteadOfDeath mirrors the revive branch in mobcommands/suicide.go:
// full heal, announce, consume the buff.
//
// Health MUST come back above zero. Skipping the death while leaving health
// negative just hands the kill to the backstop sweep on the next tick, which
// would revive nothing and lose the attribution as well.
func reviveInsteadOfDeath(char *characters.Character) {

	char.Health = char.HealthMax.Value

	if room := rooms.LoadRoom(char.RoomId); room != nil {
		room.SendTextVisual(messaging.CategoryBuffApply,
			`<ansi fg="mobname">`+char.Name+`</ansi> is suddenly revived in a shower of sparks!`,
		)
	}

	char.CancelBuffsWithFlag(buffs.ReviveOnDeath)
}

// shouldSweepReap reports whether the backstop sweep should kill this
// character.
//
// The condition is deliberately NOT "Health <= 0". A character reaped by the
// sweep is dying but NOT queued: it reached zero outside ApplyHarm. Skipping on
// health would skip the entire population the sweep exists for, and nothing
// would ever die on the non-harm paths.
func shouldSweepReap(char *characters.Character) bool {

	if char == nil {
		return false
	}

	return char.Health <= 0 && char.IsAlive() && !char.DeathQueued
}
