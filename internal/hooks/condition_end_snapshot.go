package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// endLineSnapshot is the holder's room as everyone in it could see it just
// before a light or darkness record ran out, kept for that record's end line
// (#220).
type endLineSnapshot struct {
	roomId int
	snap   rooms.VisualSnapshot
}

// endLineSnapshots holds, for each light or darkness record that ran out on
// this round's tick, the room it ran out in as it looked a moment before.
//
// A record expires inside Conditions.Trigger on the round tick, and LightNow
// stops counting it at once; its end line goes out at the next turn's prune.
// Owner rule (2026-10-05): a line announcing a change to the room's light is
// judged against the state BEFORE the change. Judged at the prune, a light's
// end line met a room already dark and a darkness's a room already lit.
//
// Keyed by the record's pointer, which Prune hands back unchanged. Every
// entry lives at most until the next PruneConditions: the prune takes the
// ones it narrates and clears the rest (a record revived before it could be
// pruned, or one a death cascade pruned first), so nothing piles up. Written
// by the round ticks and read by the prune, both on the game-loop goroutine.
var endLineSnapshots = map[*conditions.Condition]endLineSnapshot{}

// keepEndLineSnapshots snapshots room for every light or darkness record c
// holds that will run out on the Trigger about to run. Call it just before
// c.Conditions.Trigger(), with the room c is in. The room is walked once,
// and only when such a record exists.
func keepEndLineSnapshots(c *characters.Character, room *rooms.Room) {
	if room == nil {
		return
	}
	var snap rooms.VisualSnapshot
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		if !rec.ExpiresOnNextTrigger(conditions.GetConditionSpec(rec.ConditionId)) {
			continue
		}
		if snap == nil {
			snap = room.VisualSnapshot()
		}
		endLineSnapshots[rec] = endLineSnapshot{roomId: room.RoomId, snap: snap}
	}
}

// takeEndLineSnapshot returns and forgets the snapshot kept for rec, for an
// end line sent to the room endRoomId. It is nil when none was kept (a record
// removed some other way than running out) and when the holder has moved
// since: the snapshot belongs to the room the record ran out in, and the
// prune sends to the room the holder is in now, so a line sent against a
// snapshot of another room would reach nobody there. Both fall back to the
// room as it is, which is the right judgement when no light changed in it.
func takeEndLineSnapshot(rec *conditions.Condition, endRoomId int) rooms.VisualSnapshot {
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
