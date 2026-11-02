package seeders

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/goals"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

const ruleNameWitnessOfTheftToRevenge = "witness_of_theft_to_revenge"
const theftVictimRevengePriority = 90
const theftWitnessRevengePriority = 60

func init() {
	goals.SetStaleGoalCheck(isStaleTheftGoal)
}

// OnTheft is the public entry point called from the steal-action handler
// when a player successfully steals from a mob. Gives the victim (priority 90)
// and every other mob sharing the room (priority 60, since witnesses care less
// than the direct victim) a grudge against the thief, by seedTheftResponse.
//
// No event subscription is used here because events.ItemOwnership lacks a
// theft subtype to filter on — this is the architectural-exception pattern
// (same as rule 3 / craft_materials_to_wealth_item): the caller in
// actions/steal.go invokes OnTheft directly at the success point.
//
// Skips silently when thiefUserId == 0 or victimMob == nil.
func OnTheft(thiefUserId int, victimMob *mobs.Mob, item items.Item) {
	if thiefUserId == 0 || victimMob == nil {
		return
	}

	// Classify and respond for the direct victim at high priority.
	seedTheftResponse(victimMob, thiefUserId, theftVictimRevengePriority)

	// Respond for every other mob sharing the victim's room at a lower
	// witness priority. Another copy of the victim's own type is not one of
	// them: robbing one creature angers that creature, not its kind.
	room := rooms.LoadRoom(victimMob.Character.RoomId)
	if room == nil {
		return
	}
	for _, witnessInstId := range room.GetMobs() {
		if witnessInstId == victimMob.InstanceId {
			continue
		}
		witness := mobs.GetInstance(witnessInstId)
		if witness == nil || witness.MobId == victimMob.MobId {
			continue
		}
		seedTheftResponse(witness, thiefUserId, theftWitnessRevengePriority)
	}
}

// seedTheftResponse is seedWitnessResponse with theft's grudge: guards and
// noncombatants answer as they do any crime; anyone else takes it personally.
//
// Owner ruling 2026-11-01: a generic mob's theft grudge is that individual's,
// and its death ends it. A one-of-a-kind NPC keeps it on the type, where type
// and individual are the same. Interim: the better model is pruuk/DOGMud#752
// under the behaviour arc #374.
func seedTheftResponse(m *mobs.Mob, thiefUserId, priority int) {
	respondToCrime(m, func() {
		if isOneOfAKind(int(m.MobId), m.Zone) {
			seedRevengeGoalIfAbsent(m, "player", thiefUserId, priority)
			return
		}
		seedIndividualRevengeGoal(m, thiefUserId, priority)
	})
}

// seedIndividualRevengeGoal gives this one mob the revenge-mob goal the
// revenge-mob planner already reads, without touching its type's goal file.
func seedIndividualRevengeGoal(m *mobs.Mob, targetUserId, priority int) {
	_, err := goals.AddIndividual(m, &goals.Goal{
		Type:     "revenge-mob",
		Priority: priority,
		Params: map[string]any{
			"target_kind": "player",
			"target_id":   targetUserId,
		},
	})
	if err != nil {
		mudlog.Warn("seeders.seedIndividualRevengeGoal: AddIndividual failed",
			"mob_id", int(m.MobId), "target_id", targetUserId, "error", err)
	}
}

// isOneOfAKind reports whether a mob template is a single person rather than a
// kind of creature: exactly one room's spawninfo places it, and it has a
// dialogue file of its own. A single spawn alone is not enough, since most
// wildlife and many bandits are placed once, and a dialogue file alone is not
// enough, since a talking kind (the goblin shaman) can be placed many times.
//
// Inferred from data because no authored field says so (see pruuk/DOGMud#752).
// A silent boss reads as generic, and the only difference that makes is that
// its grudge ends at its death: there is one of it, so no other copy can join.
func isOneOfAKind(mobId int, zone string) bool {
	return len(rooms.SpawnRooms(mobId)) == 1 && dialogue.Load(mobId, zone) != nil
}

// isStaleTheftGoal is the goal-file rule that ends the old model: a theft
// grudge a generic type's file still holds from before the ruling above would
// keep every copy of it hunting. Only a theft seeded a player-targeted
// revenge-mob at exactly these priorities: assault seeds 75 and 50, a friend's
// death 85 against a mob, and no archetype defaults one. A one-of-a-kind NPC
// keeps its goal, and a template it cannot find keeps its goal too.
func isStaleTheftGoal(mobId int, g *goals.Goal) bool {
	if g.Type != "revenge-mob" {
		return false
	}
	if g.Priority != theftVictimRevengePriority && g.Priority != theftWitnessRevengePriority {
		return false
	}
	if kind, _ := g.Params["target_kind"].(string); kind != "player" {
		return false
	}
	spec := mobs.GetMobSpec(mobs.MobId(mobId))
	if spec == nil {
		return false
	}
	return !isOneOfAKind(mobId, spec.Zone)
}
