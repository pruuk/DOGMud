package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Stow deposits all crafting components (component bag + is_component backpack
// items) into the room's storage, leaving gear/consumables/quest items behind.
// Shorthand for the component-only case of `storage add all`.
func Stow(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !room.IsStorage {
		user.SendText(messaging.CategoryRefusal, `There is nowhere to stow anything here.`)
		return true, nil
	}
	storageCap := room.StorageCapacity
	if storageCap <= 0 {
		storageCap = 20
	}

	comps := append([]items.Item{}, user.Character.ComponentItems...)
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.GetSpec().IsComponent {
			comps = append(comps, itm)
		}
	}

	stowed := 0
	full, fadedAny := false, false
	for _, itm := range comps {
		if user.ItemStorage.SlotCount() >= storageCap {
			full = true
			break
		}
		stored, faded := storageAddQuiet(user, itm)
		if stored {
			stowed++
		}
		fadedAny = fadedAny || faded
	}
	switch {
	case stowed == 0 && fadedAny && !full:
		// quest gear faded with its own line (#940); nothing else to say
	case stowed == 0 && full:
		user.SendText(messaging.CategoryRefusal, `Storage is full.`)
	case stowed == 0:
		user.SendText(messaging.CategoryRefusal, `You have no crafting components to stow.`)
	case full:
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You stow %d component(s); storage filled up before the rest.`, stowed))
	default:
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You stow %d crafting component(s).`, stowed))
	}
	return true, nil
}
