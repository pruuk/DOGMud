package hooks

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// roomIdleChancePer10k is a room's chance per round, out of 10000, of an
// idle line (#317): Balance.RoomIdleChancePct, four times that for room -1,
// scaled by the zone's ambientmultiplier.
func roomIdleChancePer10k(room *rooms.Room) int {
	chance := float64(configs.GetBalanceConfig().RoomIdleChancePct)
	if room.RoomId == -1 {
		chance *= 4
	}
	chance *= rooms.ZoneAmbientMultiplier(room.Zone)
	return int(math.Round(chance * 100))
}

// sendRoomIdleLine sends one room idle line through the ambient budget
// (#317). A room with a single idle line shows it at most once per visit: a
// player who already saw it since walking in is left out and not charged.
func sendRoomIdleLine(room *rooms.Room, msg string, singleLine bool) {
	var already []int
	if singleLine {
		already = ambient.SingleLineHeard(room.RoomId, room.GetPlayers())
	}
	plan := ambient.Plan(ambient.Line{
		Room:      room,
		Source:    ambient.SourceRoomIdle,
		Perceives: ambient.Sees(room),
		Exclude:   already,
	})
	sendVisualRoomText(room, messaging.CategoryRoomDescription, msg, plan.Exclude...)
	if singleLine {
		ambient.MarkSingleLineHeard(room.RoomId, plan.Delivered)
	}
}
