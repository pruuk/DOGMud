package rooms

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Floor decay (city scavengers, 2026-09-30). The loot goblin used to portal
// to the room with the most items and carry everything off. It is retired:
// the cities are kept tidy by their scavenger NPCs (internal/scavenger), and
// every other floor in the world thins out once per real-world day.
//
// Once a day (UTC midnight), each item lying on a floor has a chance to be
// removed for good: FloorDecayBaseChancePct for a lone item, plus
// FloorDecayPerExtraItemPct for every other decayable item in the room, so a
// big pile clears faster than a single dropped sword (5 items at 10 + 5 each
// is 30% apiece). Every item rolls on its own.
//
// The pass is owed per room and paid lazily. Room.FloorDecayDay is the last
// day the room's floor was decayed through; it rides the instance save, so
// a room that sat unloaded for three days is owed three passes (capped at
// floorDecayMaxCatchUpDays), rolled one after another the next time the
// periodic check sees it loaded (DecayLoadedFloors). The clock starts when a
// clean floor gets its first decayable item (AddItem), so an item dropped
// today is never rolled for days it did not lie there.
//
// Never decayed:
//   - rooms a player is standing in (deferred until they leave, so nothing
//     vanishes in front of anyone);
//   - ephemeral rooms (they are discarded whole);
//   - rooms the registered exemption covers (SetFloorDecayExempt; the city
//     scavengers register their patrol rooms, since a scavenger keeps those);
//   - items the world itself put there: the room template's own floor items
//     and SpawnInfo items (tutorial props, respawning finds);
//   - quest-granting items (ItemSpec.QuestToken);
//   - a found bauble still lying untaken, which has its own, shorter clock
//     (baubles_untaken.go).
//
// The stash (hidden items) is not the floor and is never decayed, nor is
// floor gold.

// floorDecayMaxCatchUpDays caps how many missed daily passes a room is rolled
// for at once. It is a safety bound, not a balance number: by 30 passes even
// a lone item has been rolled away with near certainty.
const floorDecayMaxCatchUpDays = 30

// floorDecayNow is the clock, swapped by tests.
var floorDecayNow = time.Now

// floorDecayExempt reports rooms whose floors are kept by something else.
// Set once at boot (SetFloorDecayExempt); nil exempts nothing.
var floorDecayExempt func(roomId int) bool

// SetFloorDecayExempt registers the one exemption predicate. The scavenger
// package sets it to "a scavenger patrols this room". A setter instead of an
// import keeps rooms free of the packages that sit above it.
func SetFloorDecayExempt(fn func(roomId int) bool) {
	floorDecayExempt = fn
}

// IsFloorDecayExempt reports whether the registered exemption covers roomId.
func IsFloorDecayExempt(roomId int) bool {
	return floorDecayExempt != nil && floorDecayExempt(roomId)
}

// DecayDay is the real-world day number (UTC) of t: the unit the daily floor
// decay and the scavengers' daily reset both count in.
func DecayDay(t time.Time) int64 {
	return t.UTC().Unix() / 86400
}

// FloorDecayChancePct is the chance, in percent, that each decayable item on
// a floor holding itemCount of them is removed by one daily pass: basePct for
// the first item and perExtraPct more for each one beyond it, held to 0-100.
func FloorDecayChancePct(itemCount, basePct, perExtraPct int) int {
	if itemCount <= 0 {
		return 0
	}
	chance := basePct + perExtraPct*(itemCount-1)
	if chance < 0 {
		return 0
	}
	if chance > 100 {
		return 100
	}
	return chance
}

// authoredFloorItemIds is the set of item ids a room template lays on its own
// floor, or nil when it lays none.
func authoredFloorItemIds(template *Room) map[int]bool {
	if template == nil || len(template.Items) == 0 {
		return nil
	}
	ids := make(map[int]bool, len(template.Items))
	for _, itm := range template.Items {
		ids[itm.ItemId] = true
	}
	return ids
}

// isAuthoredFloorItem reports whether the world itself put this item on this
// floor: the room template lists it, or the room's SpawnInfo spawns it on the
// floor. Those are content, not litter.
func (r *Room) isAuthoredFloorItem(itm items.Item) bool {
	if r.authoredItemIds[itm.ItemId] {
		return true
	}
	for _, si := range r.SpawnInfo {
		if si.ItemId > 0 && si.ItemId == itm.ItemId && si.Container == `` {
			return true
		}
	}
	return false
}

// FloorItemIsLitter reports whether a floor item is ordinary litter: something
// a player or mob left, rather than something the world placed or a quest
// needs. Only litter decays, and only litter is picked up by the city
// scavengers.
func (r *Room) FloorItemIsLitter(itm items.Item) bool {
	if r.isAuthoredFloorItem(itm) {
		return false
	}
	if itm.GetSpec().QuestToken != `` {
		return false
	}
	// A found bauble lying untaken keeps its own clock (and a household's
	// belongs to the house).
	if _, untaken := itm.BaubleUntakenFor(floorDecayNow()); untaken {
		return false
	}
	if itm.BaubleBelongsTo(r.RoomId) {
		return false
	}
	return true
}

// litterCount is how many floor items are litter.
func (r *Room) litterCount() int {
	n := 0
	for _, itm := range r.Items {
		if r.FloorItemIsLitter(itm) {
			n++
		}
	}
	return n
}

// noteFloorItemAdded starts the room's decay clock when litter lands on a
// floor that held none, so the new item is first rolled at the next day
// boundary rather than for days it was not there. Call before appending.
func (r *Room) noteFloorItemAdded(itm items.Item) {
	if !r.FloorItemIsLitter(itm) {
		return
	}
	if r.litterCount() > 0 {
		return
	}
	r.FloorDecayDay = DecayDay(floorDecayNow())
}

// decayFloor pays every daily pass this room owes through now's day. roll(n)
// returns 0..n-1. It returns how many items were removed and whether the room
// was processed at all (false when nothing was owed, it is exempt, occupied
// or has no litter; the room is left untouched then).
func (r *Room) decayFloor(now time.Time, roll func(int) int) (removed int, processed bool) {
	today := DecayDay(now)
	if r.FloorDecayDay >= today {
		return 0, false
	}
	if r.IsEphemeral() {
		return 0, false
	}
	if floorDecayExempt != nil && floorDecayExempt(r.RoomId) {
		return 0, false
	}
	// Never under someone's nose: try again once they have gone.
	if len(r.players) > 0 {
		return 0, false
	}
	// A clean floor is not stamped, so a room nobody litters is never
	// rewritten. Its clock starts when litter arrives (noteFloorItemAdded).
	if r.litterCount() == 0 {
		return 0, false
	}

	passes := 1 // a floor that has never been decayed owes one pass
	if r.FloorDecayDay > 0 {
		passes = int(today - r.FloorDecayDay)
	}
	if passes > floorDecayMaxCatchUpDays {
		passes = floorDecayMaxCatchUpDays
	}

	cfg := configs.GetBalanceConfig()
	base, perExtra := int(cfg.FloorDecayBaseChancePct), int(cfg.FloorDecayPerExtraItemPct)

	for p := 0; p < passes; p++ {
		chance := FloorDecayChancePct(r.litterCount(), base, perExtra)
		if chance <= 0 {
			break
		}
		// A fresh slice: never compact r.Items in place, in case a caller
		// holds the old one (as removeUntakenBaubles does).
		kept := make([]items.Item, 0, len(r.Items))
		for _, itm := range r.Items {
			if r.FloorItemIsLitter(itm) && roll(100) < chance {
				if itm.IsBauble() {
					baubles.MarkVanished(itm.Bauble, now)
				}
				removed++
				continue
			}
			kept = append(kept, itm)
		}
		r.Items = kept
	}

	r.FloorDecayDay = today
	return removed, true
}

// DecayLoadedFloors runs the daily floor decay over every loaded room that
// owes a pass. Rooms that are not loaded keep what they owe on their
// FloorDecayDay and pay it once they are loaded again. Returns how many rooms
// were processed and how many items were removed in all.
func DecayLoadedFloors(now time.Time) (roomsProcessed int, itemsRemoved int) {
	for _, room := range roomManager.rooms {
		removed, processed := room.decayFloor(now, util.Rand)
		if processed {
			roomsProcessed++
			itemsRemoved += removed
		}
	}
	return roomsProcessed, itemsRemoved
}
