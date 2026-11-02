package seeders

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// WitnessResponse is how a mob reacts to witnessing (or being the victim of)
// a crime against another mob committed by a player.
type WitnessResponse int

const (
	// ResponseRevenge: combat-capable non-guard — seed a personal revenge goal.
	ResponseRevenge WitnessResponse = iota
	// ResponseAlarm: noncombatant civilian — momentary fright reaction; the
	// law (5.1 crime record + guard enforcement) handles the actual response.
	ResponseAlarm
	// ResponseReportOnly: guard (or nil) — seed nothing; 5.1 crime record +
	// RunGuardEnforcement enforce. A personal revenge goal would derail proper
	// enforcement.
	ResponseReportOnly
)

// classifyWitnessResponse decides how a mob should respond. Pure (no side
// effects). Guard takes precedence over the noncombatant check.
func classifyWitnessResponse(m *mobs.Mob) WitnessResponse {
	if m == nil {
		return ResponseReportOnly
	}
	if mobs.IsGuardMob(m.Groups) {
		return ResponseReportOnly
	}
	if m.IsNonCombatant() {
		return ResponseAlarm
	}
	return ResponseRevenge
}

// seedWitnessResponse classifies the mob and performs the matching effect.
// Used for both the direct victim and each room witness (victim at the higher
// victim priority). The victim is never a noncombatant (you cannot steal from
// or attack a non_combatant mob), so the victim only ever hits the guard or
// revenge branch.
func seedWitnessResponse(m *mobs.Mob, playerId, priority int) {
	respondToCrime(m, func() {
		seedRevengeGoalIfAbsent(m, "player", playerId, priority)
	})
}

// respondToCrime classifies the mob and performs the matching effect; revenge
// is the caller's, since a theft's and an assault's grudge differ in who holds
// them (seedTheftResponse).
func respondToCrime(m *mobs.Mob, revenge func()) {
	switch classifyWitnessResponse(m) {
	case ResponseRevenge:
		revenge()
	case ResponseAlarm:
		alarmReaction(m)
	case ResponseReportOnly:
		// no-op: the 5.1 crime record + RunGuardEnforcement handle it.
	}
}

// seedShapesOnlyWitnessResponse handles a witness who only made out shapes
// and movement, not a face (crimes.Witnesses.ShapesOnly). Owner ruling
// 2026-09-22: split by RESPONSE, not by list. A shapes-only witness that
// would classify ResponseRevenge cannot seed a goal naming the player by
// ID, because it never identified who to hunt — so it gets alarmReaction
// instead, exactly like the noncombatant ResponseAlarm case, since
// alarmReaction names nobody. ResponseReportOnly (guard, or nil) stays a
// no-op in this tier too, same as the identifying tier: a personal
// reaction would still derail enforcement, and a guard that only glimpsed
// shapes has nothing more to report than one that saw clearly.
func seedShapesOnlyWitnessResponse(m *mobs.Mob) {
	switch classifyWitnessResponse(m) {
	case ResponseReportOnly:
		// no-op, exactly as in the identifying tier.
	default:
		// ResponseAlarm and ResponseRevenge both land here: neither can
		// name the player by ID from shapes alone, so both get the
		// nameless alarm reaction instead of a targeted goal.
		alarmReaction(m)
	}
}

// alarmReaction is a momentary fright reaction for a noncombatant witness — a
// room-visible emote plus a single step toward an exit. No persistent goal
// (deliberately avoids the survival-goal-pruned-at-full-HP behavior). The
// actual "report" is the 5.1 crime record fired by the steal/attack action.
func alarmReaction(m *mobs.Mob) {
	if m == nil {
		return
	}
	m.Command("emote recoils and cries out, then hurries for the nearest way out.")
	room := rooms.LoadRoom(m.Character.RoomId)
	if room == nil {
		return
	}
	for exitName := range room.Exits { // map order is randomized -> a random exit
		m.Command(exitName)
		break
	}
}
