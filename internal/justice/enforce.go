package justice

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// EnforceAction reports one enforcement decision (returned for tests/telemetry).
type EnforceAction struct {
	UserId    int
	Severity  Severity
	Escalated bool // a prior warning escalated to attack this tick
}

type warnOutcome int

const (
	warnOutcomeNone warnOutcome = iota
	warnOutcomeWarn
	warnOutcomeAttack
)

type arrestOutcome int

const (
	arrestOutcomeNone    arrestOutcome = iota // pending, still within grace
	arrestOutcomeDeclare                      // first sight — stamp & say intent
	arrestOutcomeHaul                         // past grace — call ExecuteArrest
	arrestOutcomeAttack                       // resist policy — issue attack command
)

// resolveWarn is the pure escalation decision for a Warn-severity player.
func resolveWarn(alreadyWarned bool, warnedRound, nowRound, grace uint64) warnOutcome {
	if !alreadyWarned {
		return warnOutcomeWarn
	}
	if nowRound >= warnedRound && nowRound-warnedRound >= grace {
		return warnOutcomeAttack
	}
	return warnOutcomeNone
}

// resolveArrest is the pure escalation decision for a SeverityArrest player.
// resist=true means the player's ArrestPolicy is resist.
// pending=true means the guard already stamped a pending-arrest round.
func resolveArrest(resist, pending bool, pendingRound, nowRound, grace uint64) arrestOutcome {
	if resist {
		return arrestOutcomeAttack
	}
	if !pending {
		return arrestOutcomeDeclare
	}
	if nowRound >= pendingRound && nowRound-pendingRound >= grace {
		return arrestOutcomeHaul
	}
	return arrestOutcomeNone
}

// miscDataRound reads a round value stored in MiscData under key, tolerating
// the numeric kinds a YAML round-trip can produce.
func miscDataRound(misc map[string]any, key string) (uint64, bool) {
	v, ok := misc[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case uint64:
		return n, true
	case int64:
		return uint64(n), true
	case int:
		return uint64(n), true
	case float64:
		return uint64(n), true
	}
	return 0, false
}

// defaultGuardWarnGraceRounds mirrors the GuardWarnGraceRounds config default
// (internal/configs/config.balance.mobs.go); used when the knob is unset.
const defaultGuardWarnGraceRounds = 50

func warnGraceRounds() uint64 {
	v := configs.GetBalanceConfig().GuardWarnGraceRounds
	if v < 1 {
		return defaultGuardWarnGraceRounds
	}
	return uint64(v)
}

// defaultArrestGraceRounds mirrors the ArrestResistGraceRounds config default.
const defaultArrestGraceRounds = 3

func arrestGraceRounds() uint64 {
	v := configs.GetBalanceConfig().ArrestResistGraceRounds
	if v < 1 {
		return defaultArrestGraceRounds
	}
	return uint64(v)
}

// arrestStampLapseRounds is how long a declaration holds: the grace, then as
// long again for a guard in the room to make the haul. Derived from the one
// knob, ArrestResistGraceRounds, rather than a second (#241).
func arrestStampLapseRounds() uint64 { return 2 * arrestGraceRounds() }

// liveArrestStamp reads the player's declared arrest and reports whether it
// still holds in roomId at nowRound: made in this room and no older than
// arrestStampLapseRounds. A stamp that does not hold is cleared, so this
// sighting declares afresh. Walking out of the room drops the stamp at once
// (LapseArrestStampOnMove), so a player who leaves and comes back inside the
// window hears the declaration again too; the room check here is the
// backstop for a move no RoomChange reported.
func liveArrestStamp(player *characters.Character, roomId int, nowRound uint64) (uint64, bool) {
	round, ok := miscDataRound(player.MiscData, keyArrestPendingRound)
	if !ok {
		return 0, false
	}
	stampRoom, roomOk := miscDataRound(player.MiscData, keyArrestPendingRoom)
	if roomOk && stampRoom == uint64(roomId) && nowRound >= round && nowRound-round <= arrestStampLapseRounds() {
		return round, true
	}
	clearArrestStamp(player)
	return 0, false
}

// clearArrestStamp drops the player's declared arrest.
func clearArrestStamp(player *characters.Character) {
	player.SetMiscData(keyArrestPendingRound, nil)
	player.SetMiscData(keyArrestPendingRoom, nil)
}

// warnStampStaleAfter returns the number of rounds after which an unseen
// justice_warned_* stamp is considered stale and eligible for pruning.
// Delegates to lookbackFn so the two windows stay in sync with one config knob.
func warnStampStaleAfter() uint64 { return lookbackFn() }

// pruneStaleWarnStamps deletes justice_warned_* entries older than staleAfter
// rounds. Cold-rep warn stamps are never revisited once a player's rep
// recovers (the Warn branch stops running), so they would otherwise leak.
// Also deletes leftover per-guard justice_arrest_pending_* keys, which older
// builds wrote on the guard; the arrest stamp now lives on the player (#241).
// Leaves every other key alone.
// Deleting from a map during range over its keys is safe in Go.
func pruneStaleWarnStamps(md map[string]any, now, staleAfter uint64) {
	for key := range md {
		if strings.HasPrefix(key, "justice_arrest_pending_") {
			delete(md, key)
			continue
		}
		if !strings.HasPrefix(key, "justice_warned_") {
			continue
		}
		stamped, ok := miscDataRound(md, key)
		if !ok {
			continue
		}
		if now >= stamped && now-stamped > staleAfter {
			delete(md, key)
		}
	}
}

// executeArrestFn seam — tests override to intercept without live mobs.
var executeArrestFn = func(player *characters.Character, userId int, faction string, isMurder bool) bool {
	return ExecuteArrest(player, userId, faction, isMurder)
}

// guardFactionsFn seam: the factions a guard enforces for. Tests override,
// because factions.FactionsForMob needs loaded faction definitions.
var guardFactionsFn = factions.FactionsForMob

// Player MiscData keys for a declared arrest (#241). The stamp lives on the
// PLAYER, not on each guard, so one declaration is heard however many guards
// share the room, and it holds only where and while it was made
// (liveArrestStamp, LapseArrestStampOnMove).
const (
	keyArrestPendingRound = "justice_arrest_pending_round"
	keyArrestPendingRoom  = "justice_arrest_pending_room"
)

// LapseArrestStampOnMove is a RoomChange listener (hooks.RegisterListeners):
// a player who walks out of the room an arrest was declared in drops the
// declaration, so a guard who sees them next, even back in that same room
// inside the window, declares afresh before any haul (owner call 2026-10-10,
// #241). Only leaving the declared room counts: the event for the player's
// own arrival can be heard after a guard there has already declared.
func LapseArrestStampOnMove(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok || evt.UserId == 0 || evt.FromRoomId == evt.ToRoomId {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	stampRoom, ok := miscDataRound(user.Character.MiscData, keyArrestPendingRoom)
	if ok && stampRoom == uint64(evt.FromRoomId) {
		clearArrestStamp(user.Character)
	}
	return events.Continue
}

// LapseArrestStampOnDespawn is a PlayerDespawn listener
// (hooks.RegisterListeners): the stamp is saved with the character, so it is
// dropped on logout. Otherwise a player who logged out inside the window and
// came back to the same room could be hauled with no new declaration (#241).
func LapseArrestStampOnDespawn(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerDespawn)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	if user := users.GetByUserId(evt.UserId); user != nil {
		clearArrestStamp(user.Character)
	}
	return events.Continue
}

// firstFactionWithCell returns the first faction in order that owns a
// holding cell, or "" if none do. Used to pick the arresting faction when a
// guard belongs to several factions (e.g. guards + citizens).
func firstFactionWithCell(guardFactions []string) string {
	for _, f := range guardFactions {
		if cellRoomFn(f) != 0 {
			return f
		}
	}
	return ""
}

// RunGuardEnforcement scans players in the room and applies warn/attack
// against wanted players for this guard, managing warn-grace memory in the
// guard's MiscData. Returns the actions taken (for tests). Both the per-round
// tick (now) and a future protection-faction btree action (later) call this.
func RunGuardEnforcement(mob *mobs.Mob, room *rooms.Room, nowRound uint64) []EnforceAction {
	if mob == nil || room == nil || mob.Character.IsInCombat() || mob.Character.IsCharmed() {
		return nil
	}
	guardFactions := guardFactionsFn(mob)
	if len(guardFactions) == 0 {
		return nil
	}

	// Sweep stale warn stamps once per tick per guard. Warn stamps are written
	// when a Cold-rep player is first sighted, but never cleared once the
	// player's rep recovers (the Warn branch simply stops firing). Without this
	// sweep they accumulate on the guard's MiscData indefinitely.
	pruneStaleWarnStamps(mob.Character.MiscData, nowRound, warnStampStaleAfter())

	grace := warnGraceRounds()
	var acts []EnforceAction

	for _, uid := range room.GetPlayers(rooms.FindAll) {
		user := users.GetByUserId(uid)
		if user == nil {
			continue
		}
		if user.Character.HasConditionFlag(conditions.NoAggroTarget) ||
			user.Character.IsHidden() || user.Character.Health < 1 {
			continue
		}
		// A player already serving a sentence is in custody — guards leave them
		// be. Without this, a wandering guard enters the cell and re-arrests or
		// attacks someone already locked up (5.1c smoke BUG-04).
		if _, jailed := miscDataRound(user.Character.MiscData, keyJailUntilRound); jailed {
			continue
		}

		sev := Verdict(guardFactions, uid)
		switch sev {
		case SeverityAttack:
			// Intentionally leaves any prior warn-stamp in MiscData; it is
			// inert while SeverityAttack applies and harmless if rep recovers.
			mob.Command(fmt.Sprintf("attack @%d", uid))
			acts = append(acts, EnforceAction{uid, SeverityAttack, false})
		case SeverityWarn:
			key := fmt.Sprintf("justice_warned_%d", uid)
			warnedRound, warned := miscDataRound(mob.Character.MiscData, key)
			switch resolveWarn(warned, warnedRound, nowRound, grace) {
			case warnOutcomeWarn:
				guardSayFn(room, mob, "Move along. You're not welcome here.")
				mob.Character.SetMiscData(key, nowRound)
				acts = append(acts, EnforceAction{uid, SeverityWarn, false})
			case warnOutcomeAttack:
				mob.Command(fmt.Sprintf("attack @%d", uid))
				acts = append(acts, EnforceAction{uid, SeverityAttack, true})
			}
		case SeverityArrest:
			resist := user.Character.ArrestPolicy == characters.ArrestResist
			pendingRound, pending := liveArrestStamp(user.Character, room.RoomId, nowRound)
			switch resolveArrest(resist, pending, pendingRound, nowRound, arrestGraceRounds()) {
			case arrestOutcomeAttack:
				mob.Command(fmt.Sprintf("attack @%d", uid))
				acts = append(acts, EnforceAction{uid, SeverityAttack, false})
			case arrestOutcomeDeclare:
				guardSayFn(room, mob,
					"No more moving along. You're under arrest. Come quietly.")
				user.Character.SetMiscData(keyArrestPendingRound, nowRound)
				user.Character.SetMiscData(keyArrestPendingRoom, room.RoomId)
				acts = append(acts, EnforceAction{uid, SeverityArrest, false})
			case arrestOutcomeHaul:
				faction := firstFactionWithCell(guardFactions)
				if faction == "" {
					// No arresting faction owns a cell, so no haul; leave the
					// player's stamp; it lapses on its own (liveArrestStamp).
					break
				}
				executeArrestFn(user.Character, uid, faction, false)
				clearArrestStamp(user.Character)
				acts = append(acts, EnforceAction{uid, SeverityArrest, true})
			}
		}
	}
	return acts
}

// guardSayFn speaks a guard's line. Default is a no-op so package justice has no
// dependency on internal/actions; internal/hooks wires the real broadcaster at
// init (hooks/justice_wiring.go). Injectability also breaks the actions↔justice
// import cycle so crime sites in internal/actions can call MaybeDeclareBounty.
var guardSayFn = func(room *rooms.Room, mob *mobs.Mob, line string) {}

// SetGuardSay installs the guard-speech implementation (called once from
// internal/hooks at init). A nil fn is ignored, keeping the no-op default.
func SetGuardSay(fn func(room *rooms.Room, mob *mobs.Mob, line string)) {
	if fn != nil {
		guardSayFn = fn
	}
}
