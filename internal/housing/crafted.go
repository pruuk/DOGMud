package housing

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Crafted furniture (wilderness trades, phase 5). A woodworker's chest, bed
// frame or workbench is an item whose spec names a furnishing kind
// (items.ItemSpec.Furnishing). Its owner places it in their own lodging with
// "use", exactly as they would place the deed it stands in for, in any
// building: a chest is a container, a bed frame a bed, a workbench a
// woodworking bench station. Placing it spends the item, like a deed.

// furnishingKind is the furnishing an item places, or "" when it is not
// crafted furniture.
func furnishingKind(itemId int) string {
	spec := items.GetItemSpec(itemId)
	if spec == nil {
		return ``
	}
	return spec.Furnishing
}

// IsCraftedFurnishing reports whether an item is crafted furniture.
func IsCraftedFurnishing(itemId int) bool {
	return furnishingKind(itemId) != ``
}

// workbenchStation is the station a crafted workbench installs: the
// woodworking bench, when some recipe uses it.
func workbenchStation() string {
	for _, st := range stationTypes() {
		if st == `woodworking_bench` {
			return StationName(st)
		}
	}
	return ``
}

func useCraftedFurnishing(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, b Building, kind string, args string, rest string, deedSend func(string)) {
	// The placing code speaks of deeds; this is a piece of furniture.
	send := func(msg string) {
		msg = strings.ReplaceAll(msg, `The deed stays folded.`, `You keep it for now.`)
		msg = strings.ReplaceAll(msg, `Try the deed in another of your rooms.`, `Try another of your rooms.`)
		msg = strings.ReplaceAll(msg, `You fold the deed away for another day.`, `You keep it for another day.`)
		msg = strings.ReplaceAll(msg, `The deed will not take`, `It will not go in`)
		deedSend(msg)
	}
	switch kind {
	case items.FurnishingChest:
		useContainerDeed(user, room, itm, h, b, false, args, rest, send)
	case items.FurnishingBed:
		useBedDeed(user, room, itm, h, send)
	case items.FurnishingWorkbench:
		station := workbenchStation()
		if station == `` {
			send(`There is nothing a workbench is good for just now. Keep it for later.`)
			return
		}
		useStationDeed(user, room, itm, h, station, rest, send)
	default:
		send(`You can't find a place for that here.`)
	}
}
