package usercommands

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// gearupWearPrefix starts each wear gearup queues: by item id, so the
// command finds that very item.
const gearupWearPrefix = `wear !`

func Gearup(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `change equipment`) {
		return true, nil
	}

	// Each wear below is its own engine command, outside the player's
	// pending client commands. A repeat gearup while the last one's wears
	// still wait would add a whole pack's worth again each time, so it is
	// refused until they have run (a few milliseconds each).
	if events.QueuedEngineInputs(user.UserId, gearupWearPrefix) > 0 {
		refusal.StillGearingUp(user)
		return true, nil
	}

	allBackpackItems := user.Character.GetAllBackpackItems()
	wearableCount := 0
	wornSomething := false

	// Sort by value descending so best items get equipped first
	sort.Slice(allBackpackItems, func(i, j int) bool {
		return allBackpackItems[i].GetSpec().Value > allBackpackItems[j].GetSpec().Value
	})

	for _, itm := range allBackpackItems {
		itmSpec := itm.GetSpec()

		if itmSpec.Type != items.Weapon && itmSpec.Subtype != items.Wearable {
			continue
		}

		wearableCount++

		// Delegate to wear command — it handles pair logic, slot finding,
		// and displacement automatically. The -1 delay means immediate.
		user.Command(fmt.Sprintf(gearupWearPrefix+`%d`, itm.ItemId), -1)
		wornSomething = true
	}

	if !wornSomething {
		if wearableCount == 0 {
			user.SendText(messaging.CategoryRefusal, "You have nothing to wear.")
		} else {
			user.SendText(messaging.CategoryRefusal, "You're already wearing everything you can!")
		}
	}

	return true, nil
}
