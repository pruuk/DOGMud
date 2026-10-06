package rooms

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
)

// endLineSnapshot is the holder's room as everyone in it could see it just
// before a light or darkness record ended, kept for that record's end line
// (#220).
type endLineSnapshot struct {
	roomId int
	snap   VisualSnapshot
}

// endLineSnapshots holds, for each light or darkness record that ended since
// the last prune, the room it ended in as it looked a moment before.
//
// A record ends in one of two ways, and both stop LightNow counting it at
// once while its end line waits for the next turn's prune: it runs out
// inside Conditions.Trigger on the round tick (the hooks round ticks keep a
// snapshot first), or a player cancels it (usercommands cancel keeps one with
// KeepEndLineSnapshotsBeforeRemoval). Owner rule (2026-10-05): a line
// announcing a change to the room's light is judged against the state BEFORE
// the change. Judged at the prune, a light's end line met a room already dark
// and a darkness's a room already lit.
//
// Each watcher's decision is frozen at the moment of the snapshot until the
// line is sent, as with every other VisualSnapshot user: a watcher blinded or
// fallen asleep in between still gets the line, and one who arrived in
// between does not.
//
// Paths left alone, each of which removes a record without a snapshot, so
// its end line (if any) is judged by the room as the prune finds it:
//   - the worn-item refresh (characters.reapplyPermanentConditions): item
//     lights and darknesses (124 to 127, 132, 134) author no end line;
//   - the death cascade (CancelConditionsWithFlag(All)): a dying player is
//     teleported to respawn within the same cascade (hooks
//     Respawn_PlayerTeleport.go), so a snapshot would be dropped by the room
//     check in TakeEndLineSnapshot anyway; a dying mob's light ends with its
//     death, which its death line already announces;
//   - the generic removers (a negative condition event, start_remove_conditions,
//     the behaviour tree's remove_condition, the purge draught, pinnacle
//     revocation): no shipped content uses them on the two records with an
//     end line, conditions 1 and 131. One that starts to must call
//     KeepEndLineSnapshotsBeforeRemoval first.
//
// Keyed by the record's pointer, which Prune hands back unchanged. Every
// entry lives at most until the next prune: the prune takes the ones it
// narrates and ClearEndLineSnapshots drops the rest (a record revived before
// it could be pruned, or one a death cascade pruned first), so nothing piles
// up. Writers and the reader all run on the game-loop goroutine today; the
// mutex keeps the store safe should that change.
var (
	endLineMu        sync.Mutex
	endLineSnapshots = map[*conditions.Condition]endLineSnapshot{}
)

// KeepEndLineSnapshot keeps snap, taken of room roomId, for rec's end line.
// Take snap just before rec stops counting (before the Trigger it runs out
// on, or before it is removed early).
func KeepEndLineSnapshot(rec *conditions.Condition, roomId int, snap VisualSnapshot) {
	if rec == nil || snap == nil {
		return
	}
	endLineMu.Lock()
	defer endLineMu.Unlock()
	endLineSnapshots[rec] = endLineSnapshot{roomId: roomId, snap: snap}
}

// TakeEndLineSnapshot returns and forgets the snapshot kept for rec, for an
// end line sent to the room endRoomId. It is nil when none was kept (a record
// removed by a path left alone above) and when the holder has moved since:
// the snapshot belongs to the room the record ended in, and the prune sends
// to the room the holder is in now, so a line sent against a snapshot of
// another room would reach nobody there. Both fall back to the room as it is
// now. For a holder who moved that is the right judgement: no light changed
// in the new room. For a record removed without a snapshot it is the known
// gap the list above names.
func TakeEndLineSnapshot(rec *conditions.Condition, endRoomId int) VisualSnapshot {
	endLineMu.Lock()
	defer endLineMu.Unlock()
	kept, ok := endLineSnapshots[rec]
	if !ok {
		return nil
	}
	delete(endLineSnapshots, rec)
	if kept.roomId != endRoomId {
		return nil
	}
	return kept.snap
}

// ClearEndLineSnapshots drops every kept snapshot. The prune calls it once it
// has narrated what it will, so no entry outlives it; tests call it to start
// clean.
func ClearEndLineSnapshots() {
	endLineMu.Lock()
	defer endLineMu.Unlock()
	clear(endLineSnapshots)
}

// KeepEndLineSnapshotsBeforeRemoval snapshots c's room for every held light
// or darkness record of conditionId, for its end line. Call it just before
// removing the condition early (a cancel), while the room still shows it.
// The room is walked once, and only when such a record exists.
func KeepEndLineSnapshotsBeforeRemoval(c *characters.Character, conditionId int) {
	room := LoadRoom(c.RoomId)
	if room == nil {
		return
	}
	var snap VisualSnapshot
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		if rec.ConditionId != conditionId {
			continue
		}
		if snap == nil {
			snap = room.VisualSnapshot()
		}
		KeepEndLineSnapshot(rec, room.RoomId, snap)
	}
}
