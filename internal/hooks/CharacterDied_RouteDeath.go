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

	// Restore through the primitive, not a direct write. The U5b pool-mutation
	// guard forbids assigning to a pool, and ApplyRestore owns the bounds, so
	// the hand-rolled clamp that would otherwise sit here is unnecessary.
	// Health is negative at this point, so the shortfall is max minus current.
	if missing := char.HealthMax.Value - char.Health; missing > 0 {
		char.ApplyRestore(characters.PoolHealth, missing)
	}

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
//
// The !DeathQueued half is not merely about preserving attribution, and the
// difference matters more for players than for mobs:
//
//   - A MOB stays at Dead once Die runs, so a second Die returns early on the
//     !IsAlive() guard. Reaping it first only loses the killer.
//   - A PLAYER does not. Die cascades Dead -> Respawning -> Alive (die.go), so
//     it RETURNS WITH THE PLAYER ALIVE and the early guard cannot catch a
//     second call. Reaping a player whose attributed death is already queued
//     therefore runs the entire death cascade TWICE -- corpse, announcement,
//     bounty resolution, jail cleanup, every AfterTransition observer on Dead.
//
// So this gate is load-bearing for correctness, not just for credit.
func shouldSweepReap(char *characters.Character) bool {

	if char == nil {
		return false
	}

	return char.Health <= 0 && char.IsAlive() && !char.DeathQueued
}
