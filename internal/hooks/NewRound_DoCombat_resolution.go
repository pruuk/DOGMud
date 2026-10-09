package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
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

// waitRoundDarkAttackerLine is sent to the attacker in place of the
// authored MessagesToSource line when the attacker cannot see clearly.
// waitRoundDarkDefenderLine is sent to the defender in place of the
// authored MessagesToTarget line when the defender cannot see clearly.
// Package-level so the tests can reference them directly.
// Both are FORMAT strings taking the other party's plain name, which is then
// hidden by the reader's own sight through messaging.HideNames.
//
// 🔑 They used to be fixed sentences with the word "something" baked in, which
// made them BINARY: a reader who could make out warm shapes and a reader who
// was fully blind got the identical line. In play on 2026-09-21 that produced
// "Something hangs back in the dark, biding its time." sitting among a dozen
// lines that all correctly said "A figure", because everything else had moved
// onto the three-tier verdict and this had not.
//
// The name is deliberately UNTAGGED. This branch only runs for a reader who is
// not fully sighted, so HideNames always replaces it and an identity colour
// would never render; leaving the tag off also avoids guessing between
// mobname and username for an attacker who may be either.
const (
	waitRoundDarkAttackerLine = `<ansi fg="yellow">You bide your time, straining to place %s.</ansi>`
	waitRoundDarkDefenderLine = `<ansi fg="attack-bad">%s hangs back, biding its time.</ansi>`
)

// handleCombatWaitRound handles the RoundsWaiting > 0 short-circuit
// shared by the unified combat handler across all four quadrants.
// Returns true if the caller should return immediately (i.e. the
// attacker is still waiting).
//
// attackerUser is non-nil when the attacker is a player (PvM/PvP).
// The fight sound (sendUnsightedCombatSound) excludes BOTH combatants, since
// in PvP each already reads their own wait line.
//
// The authored MessagesToSource/MessagesToTarget lines from
// combat.GetWaitMessages name the weapon as well as the foe (e.g. "Something
// holds their Rusted Cleaver steady, eyes fixed on you."), so hiding just the
// name is not enough. A participant without clear sight
// (messaging.CanSeeSightImpairedOnly: darkness or blindness, not sleep, see
// the predicate's own comment for why a sleeper must still read what hit
// them; infrared does not count as clear) instead reads one fixed dark line, the
// same convention the swing path (dispatchCritAndMessaging /
// replaceDarknessMessages) already uses. The fixed line is only sent when
// there was an authored line to begin with, so an empty authored list stays
// silent as before. Room lines are delivered inside combat.GetWaitMessages
// itself via messaging.SendTrio, not drained here; see the comment at the
// drain's old call site below.
func handleCombatWaitRound(
	attackerChar *characters.Character,
	defenderChar *characters.Character,
	roleSource combat.SourceTarget,
	roleTarget combat.SourceTarget,
	attackerUser *users.UserRecord,
	defenderUser *users.UserRecord,
	attackerRoom *rooms.Room,
	defenderRoom *rooms.Room,
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

	// Sight gate, the swing path's convention: a participant without clear
	// sight gets one fixed dark line instead of the authored one, because
	// the authored line names the weapon as well as the foe.
	if attackerUser != nil {
		if messaging.CanSeeSightImpairedOnly(attackerChar, attackerRoom) {
			for _, msg := range roundResult.MessagesToSource {
				attackerUser.SendText(msg.Category, msg.Text)
			}
		} else if len(roundResult.MessagesToSource) > 0 {
			attackerUser.SendText(roundResult.MessagesToSource[0].Category,
				messaging.HideNames(
					fmt.Sprintf(waitRoundDarkAttackerLine, defenderChar.Name),
					[]string{defenderChar.Name},
					messaging.ParticipantSight(attackerChar, attackerRoom)))
		}
	}
	if defenderUser != nil {
		if messaging.CanSeeSightImpairedOnly(defenderChar, defenderRoom) {
			for _, msg := range roundResult.MessagesToTarget {
				defenderUser.SendText(msg.Category, msg.Text)
			}
		} else if len(roundResult.MessagesToTarget) > 0 {
			defenderUser.SendText(roundResult.MessagesToTarget[0].Category,
				messaging.HideNames(
					fmt.Sprintf(waitRoundDarkDefenderLine, attackerChar.Name),
					[]string{attackerChar.Name},
					messaging.ParticipantSight(defenderChar, defenderRoom)))
		}
	}

	// No MessagesToSourceRoom/MessagesToTargetRoom drain here: wait-round
	// room lines are delivered by combat.GetWaitMessages itself, through
	// messaging.SendTrio's Observer/RemoteObserver seats, so each reader's
	// own sight can hide names -- a raw drain through sendVisualRoomText
	// could not do that. That is why a room-line handler has no room-line
	// handling of its own here.
	excludes := playerExcludeIdsFromUsers(attackerUser, defenderUser)
	sendUnsightedCombatSound(attackerRoom, excludes...)
	if defenderRoom != attackerRoom {
		sendUnsightedCombatSound(defenderRoom, excludes...)
	}
	return true
}

// playerExcludeIdsFromUsers is the user ids of whichever of the two combatants
// are players, for a room broadcast that must skip both.
func playerExcludeIdsFromUsers(attackerUser, defenderUser *users.UserRecord) []int {
	excludes := []int{}
	if attackerUser != nil {
		excludes = append(excludes, attackerUser.UserId)
	}
	if defenderUser != nil {
		excludes = append(excludes, defenderUser.UserId)
	}
	return excludes
}
