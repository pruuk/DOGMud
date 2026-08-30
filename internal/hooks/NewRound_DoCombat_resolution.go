package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// File: NewRound_DoCombat_resolution.go
//
// Shared helper(s) used by the unified combat-round handler in
// NewRound_DoCombat_unified.go. Originally home to per-quadrant phase
// helpers extracted during the 1.2a god-function refactor; those have
// since been removed by Stage 2b of the combat-quadrant unification
// work. Only handleCombatWaitRound remains here because it is invoked
// from phase1WaitRound (in the unified handler) for every quadrant.

// handleCombatWaitRound handles the RoundsWaiting > 0 short-circuit
// shared by the unified combat handler across all four quadrants.
// Returns true if the caller should return immediately (i.e. the
// attacker is still waiting).
//
// attackerUser is non-nil when the attacker is a player (PvM/PvP).
// defenderUser is non-nil when the defender is a player (MvP/PvP).
// viewerUserId is the user ID passed to sendVisualRoomText and
// sendDarkRoomCombatFallback — always the user participant (0 if neither
// side is a player, e.g. MvM).
func handleCombatWaitRound(
	attackerChar *characters.Character,
	defenderChar *characters.Character,
	roleSource combat.SourceTarget,
	roleTarget combat.SourceTarget,
	attackerUser *users.UserRecord,
	defenderUser *users.UserRecord,
	attackerRoom *rooms.Room,
	defenderRoom *rooms.Room,
	viewerUserId int,
) bool {
	// U12c-2: the guard and the decrement are ONE call now, so they cannot
	// drift apart. Note the debug line logs the value AFTER the decrement,
	// where it used to log before.
	if attackerChar.CombatPhase == nil || !attackerChar.ConsumeRoundWaiting() {
		return false
	}
	mudlog.Debug(`RoundsWaiting`, `User`, attackerChar.Name,
		`Rounds`, attackerChar.RoundsWaiting())

	roundResult := combat.GetWaitMessages(items.Wait, attackerChar, defenderChar, roleSource, roleTarget)

	// AttackResult drainage — each TaggedMessage carries the Category
	// the producer chose for that line (weapon subtype for hits,
	// defense verb for defenses).
	for _, msg := range roundResult.MessagesToSource {
		if attackerUser != nil {
			attackerUser.SendText(msg.Category, msg.Text)
		}
	}
	for _, msg := range roundResult.MessagesToTarget {
		if defenderUser != nil {
			defenderUser.SendText(msg.Category, msg.Text)
		}
	}
	for _, msg := range roundResult.MessagesToSourceRoom {
		sendVisualRoomText(attackerRoom, msg.Category, msg.Text, viewerUserId)
	}
	for _, msg := range roundResult.MessagesToTargetRoom {
		sendVisualRoomText(defenderRoom, msg.Category, msg.Text, viewerUserId)
	}
	sendDarkRoomCombatFallback(attackerRoom, viewerUserId)
	if defenderRoom != attackerRoom {
		sendDarkRoomCombatFallback(defenderRoom, viewerUserId)
	}
	return true
}
