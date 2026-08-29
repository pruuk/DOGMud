package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/targeting"
)

// Flee makes a mob disengage from combat and move to a random adjacent room.
// The flee attempt is gated by an opposed roll: mob Dex+Skullduggery vs each
// attacker's Dex+UnarmedCombat. If any attacker wins the roll, the mob is
// blocked and stays in the room.
func Flee(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Non-combatant mobs never flee (they should never be in combat)
	if mob.IsNonCombatant() {
		return true, nil
	}

	// A no-flee state (e.g. the Blood Frenzy mutation state) forbids retreat —
	// mirrors the player-flee gate so a frenzied mob cannot panic-flee.
	if mob.Character.HasBuffFlag(buffs.NoFlee) {
		return true, nil
	}

	// Can't flee while grappled. Chunk 4b R3: FSM-driven — covers all
	// 11 grapple states via the two rollups (legacy enum only knew the
	// Clinched / Grounded buckets). Mirrors the player-flee
	// grapple-block UX in handlePlayerFlee — the player needs to see
	// that their grapple is holding the mob even when the mob tries
	// to break free. Pre-chunk-4b this returned silently, which made
	// the grapple feel passive.
	if mob.Character.IsStandingGrapple() || mob.Character.IsGroundGrapple() {
		room.SendTextVisual(messaging.CategoryGrappleFlow,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to break free but you've got them locked down!`, mob.Character.Name))
		return true, nil
	}

	// Shared opposed-roll blocker resolution (combat.ResolveFleeBlockers).
	// Identical math to the player-flee path in handlePlayerFlee; the
	// helper handles both player and mob blockers, the prone penalty,
	// and the targeting filter.
	blocker, contested := combat.ResolveFleeBlockers(&mob.Character, room, true)
	// Parity with the player path in handlePlayerFlee: practice for breaking
	// off is awarded by the wrapper, and only when a contest actually occurred.
	// MobSkillCap in CheckSkillProgression bounds what a mob can reach.
	// U10b-1 Task 18: won is "got away". blocker != nil means something
	// intercepted the flee, which is the loss. The `contested` gate is
	// unchanged and is not the firing rule: an uncontested flee resolved no
	// contest, so there is no loss to pay a fraction on.
	if contested {
		mob.Character.AwardResolved(0, blocker == nil,
			mob.Character.CandidateFor(string(skills.Skullduggery)))
	}
	if blocker != nil {
		room.SendTextVisual(messaging.CategoryRoomExit,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to flee but is blocked!`, mob.Character.Name))
		return true, nil
	}

	// If in combat, clear aggro
	if mob.Character.IsInCombat() {
		targeting.Release(&mob.Character, targeting.ReasonDisengage)
	}

	// Get a random exit (skips secret and locked exits)
	exitName, _ := room.GetRandomExit()
	if exitName == "" {
		// Cornered — no exits available. Surface the panic visibly so
		// the room knows the mob tried and failed (pre-fix this was
		// silent, making the mob appear to randomly exit combat).
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> looks around frantically for an escape but finds none!`, mob.Character.Name))
		return true, nil
	}

	// Send flee message before moving
	room.SendTextVisual(messaging.CategoryRoomExit,
		fmt.Sprintf(`<ansi fg="mobname">%s</ansi> flees!`, mob.Character.Name))

	// Move via the existing Go command
	Go(exitName, mob, room)

	behaviortree.TryMobBehavior(mob.InstanceId, behaviortree.EventContext{
		EventType: "mob_flee",
		RoomId:    mob.Character.RoomId,
	})

	return true, nil
}
