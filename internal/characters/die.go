package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// Die routes a character through the Life machine death sequence.
//
// For players (UserId != 0) the full three-step cascade fires:
//
//	Dead → Respawning → Alive
//
// For mobs (UserId == 0) only the Dead transition fires; the
// instance-cleanup observer (Death_MobInstanceCleanup.go) handles
// despawn, and mobs never enter the Respawning state.
//
// Prechecks live in hooks.RouteAttributedDeath, NOT at call sites.
//
// This comment used to tell callers to pre-check three things. Two of those
// claims were false and are removed rather than carried forward:
//
//   - "ReviveOnDeath buff (already handled at each call site)" — it was not.
//     Only the two suicide commands checked it, so the buff was inert on every
//     combat and damage-over-time death until U5c centralised the check.
//   - "Shadow Realm zone guard (player sites only)" — no such guard exists
//     anywhere in this repository. The only occurrence was this line.
//
// LastSuicideRound remains a real dedupe, and remains the suicide commands' own
// concern: it guards a command double-firing, not harm-driven death, which is
// deduped by Character.DeathQueued instead.
//
// IDEMPOTENCE IS MOB-ONLY, and the difference bites. The !IsAlive() guard below
// stops a second call for a MOB, which stays at Dead. It does NOT for a player:
// this function cascades Dead -> Respawning -> Alive and RETURNS WITH THE PLAYER
// ALIVE, so a second call re-runs the whole cascade — corpse, announcement,
// bounty resolution, jail cleanup, every AfterTransition observer on Dead.
// That is why the backstop sweeps gate on DeathQueued rather than on health.
func (c *Character) Die(killer state.ActorRef, trigger string) {
	if !c.IsAlive() {
		return
	}

	// U5c: this character's life state is being resolved HERE, so any
	// CharacterDied event still in flight for them is now stale. Clearing the
	// token invalidates it — hooks.RouteAttributedDeath refuses to act without
	// it. Every Die caller therefore invalidates a pending event for free.
	//
	// This must be done for players in particular. Die cascades them back to
	// Alive, so the !IsAlive() guard above cannot catch a second call, and a
	// stale event would otherwise run the whole death cascade again.
	c.DeathQueued = false

	damageSnapshot := snapshotDamageMap(c.PlayerDamage)

	_ = c.Life.TransitionToDead(
		life.DeadData{
			Killer:    killer,
			DamageMap: damageSnapshot,
		},
		state.TransitionReason{
			Trigger: trigger,
			Actor:   killer,
		},
	)

	// Mobs stay at Dead — the instance cleanup observer fires
	// synchronously in the AfterTransition chain above and
	// despawns the mob.  The Life machine for mob characters
	// does not advance past Dead.
	if c.GetUserId() == 0 {
		return
	}

	_ = c.Life.TransitionToRespawning(
		life.RespawningData{DestRoomId: c.ResolveRespawnRoom()},
		state.TransitionReason{Trigger: life.TriggerRespawnReady},
	)
	_ = c.Life.TransitionToAlive(
		state.TransitionReason{Trigger: life.TriggerRespawnComplete},
	)
}

// snapshotDamageMap returns a shallow copy of src so that
// observers see a stable damage map even if the original is
// cleared by Life_Cascades.go after the transition.
func snapshotDamageMap(src map[int]int) map[int]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
