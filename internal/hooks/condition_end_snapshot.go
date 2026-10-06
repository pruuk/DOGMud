package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// keepEndLineSnapshots is the round ticks' side of the end-line snapshot
// store (rooms.KeepEndLineSnapshot, #220): it snapshots room for every light
// or darkness record c holds that will run out on the Trigger about to run.
// Call it just before c.Conditions.Trigger(), with the room c is in. The
// snapshot is the room's one for this round (Room.EndLineRoundSnapshot),
// taken the first time any holder in it needs one, before any light or
// darkness there has run out this round, so every end line from one room on
// one round is judged against the same moment, across the user and mob
// ticks. It is taken only when such a record exists. The prune sends each
// end line against its snapshot (rooms.TakeEndLineSnapshot).
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
			snap = room.EndLineRoundSnapshot()
		}
		rooms.KeepEndLineSnapshot(rec, room.RoomId, snap)
	}
}
