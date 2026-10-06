package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
)

// DisarmResult represents the outcome of a disarm attempt
type DisarmResult struct {
	Success     bool
	Weapon      items.Item
	Message     string // For disarmer
	TargetMsg   string // For disarmed
	RoomMessage string // For observers
}

// AttemptCritDisarm attempts to disarm a target with a given percentage chance.
// Checks for PermaGear immunity and valid weapon before attempting.
// Returns DisarmResult with success status and messages.
func AttemptCritDisarm(source *characters.Character, target *characters.Character, disarmChance float64) DisarmResult {
	result := DisarmResult{
		Success: false,
	}

	// Check for PermaGear condition immunity
	if target.HasConditionFlag(conditions.PermaGear) {
		return result
	}

	// Check if target has a weapon equipped (0 = unarmed, <0 = disabled slot)
	if target.Equipment.Weapon.ItemId <= 0 {
		return result
	}

	// Roll percentile chance
	success, _ := dice.Percentile(disarmChance)
	if !success {
		return result
	}

	// Disarm successful!
	result.Success = true
	result.Weapon = target.Equipment.Weapon

	// Generate messages from the shipped store. Names are bare (no identity
	// ansi tag), matching the original Sprintf calls.
	weaponName := result.Weapon.NameSimple()
	ids := grappleMoveIdentities{ActorPlain: source.Name, ActeePlain: target.Name}
	if roles, ok := renderGrappleEvent(movenarration.EventKey("disarm"), ids, map[string]string{
		movenarration.TokenWeapon: weaponName,
	}); ok {
		result.Message = roles.Actor
		result.TargetMsg = roles.Actee
		result.RoomMessage = roles.Observer
	}

	// Remove the weapon from the hand and return it to the pack: never the
	// ground, and never lost to a full pack (owner, 2026-10-06). Queue the
	// EquipmentChange every other equipment change queues (#413), so item
	// trees hear on_unequip and the GMCP equipment panel refreshes.
	target.RemoveFromBody(result.Weapon)
	target.ReturnToPack(result.Weapon)
	events.AddToQueue(events.EquipmentChange{
		UserId:        target.GetUserId(),
		MobInstanceId: target.GetMobInstanceId(),
		ItemsRemoved:  []items.Item{result.Weapon},
	})

	return result
}

// SetGrappleOpportunity sets a 1-round grapple opportunity timer on a character.
// This grants a +15% grapple bonus on the next grapple attempt.
func SetGrappleOpportunity(char *characters.Character) {
	char.TimerSet("grapple-opportunity", "1 round")
}

// HasGrappleOpportunity checks if a character has an active grapple opportunity timer.
func HasGrappleOpportunity(char *characters.Character) bool {
	return char.TimerExists("grapple-opportunity") && !char.TimerExpired("grapple-opportunity")
}

// GetGrappleOpportunityBonus returns the grapple opportunity multiplier.
// Returns 1.15 if the character has an active opportunity, 1.0 otherwise.
func GetGrappleOpportunityBonus(char *characters.Character) float64 {
	if HasGrappleOpportunity(char) {
		return 1.15
	}
	return 1.0
}

// ClearGrappleOpportunity removes the grapple opportunity timer from a character.
func ClearGrappleOpportunity(char *characters.Character) {
	if char.Timers != nil {
		delete(char.Timers, "grapple-opportunity")
	}
}
