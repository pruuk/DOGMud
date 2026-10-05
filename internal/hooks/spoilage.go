package hooks

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// spoilageSweepEvery is how often (in rounds) a player's raw goods are
// checked for rot. Spoilage is measured in game hours, so a check every ten
// rounds (40 seconds) is far finer than it needs to be and costs one pass over
// the backpack and component bag.
const spoilageSweepEvery = 10

// sweepSpoiledGoods throws out the harvested raw goods (meat, organs, raw
// hides) a player carries that have rotted, and says so. Only items stamped
// at harvest are on a clock (items.Item.Spoils); shop-bought goods never rot.
// Goods in bank storage are not swept: storage is a cold store.
func sweepSpoiledGoods(user *users.UserRecord) {
	now := util.GetRoundCount()
	if now%spoilageSweepEvery != uint64(user.UserId%spoilageSweepEvery) {
		return
	}
	spoiled := actions.SpoiledItems(user.Character, now)
	if len(spoiled) == 0 {
		return
	}
	names := make([]string, 0, len(spoiled))
	for _, itm := range spoiled {
		if user.Character.RemoveItem(itm) {
			names = append(names, fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, itm.DisplayName()))
			events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
		}
	}
	if len(names) > 0 {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="yellow">The smell gives it away: %s has gone rotten, and you throw it out.</ansi> Cure, smoke or sell raw goods before they turn.`,
			strings.Join(names, `, `)))
	}
}
