package rooms

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// removeSpoiledGoods throws out harvested raw goods (meat, organs, raw hides:
// items on a spoilage clock, items.Item.Spoils) lying on the floor or stashed
// in the room once they have rotted, and tells anyone present. It returns how
// many it removed.
//
// Only the floor and the stash: a room container (a house chest among them)
// is persisted through its own owner, and a player holding goods has their
// own sweep (hooks/spoilage.go). Like the untaken-bauble sweep it never
// compacts a slice in place.
func (r *Room) removeSpoiledGoods(roundNow uint64) int {
	removed := 0
	rotted := []string{}
	sweep := func(pool []items.Item, visible bool) []items.Item {
		hasRotten := false
		for i := range pool {
			if pool[i].IsSpoiled(roundNow) {
				hasRotten = true
				break
			}
		}
		if !hasRotten {
			return pool
		}
		kept := make([]items.Item, 0, len(pool))
		for i := range pool {
			if pool[i].IsSpoiled(roundNow) {
				removed++
				if visible {
					rotted = append(rotted, fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, pool[i].DisplayName()))
				}
				continue
			}
			kept = append(kept, pool[i])
		}
		return kept
	}
	r.Items = sweep(r.Items, true)
	r.Stash = sweep(r.Stash, false)
	if len(rotted) > 0 && len(r.players) > 0 {
		r.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(
			`Flies have found it: %s rots away to nothing.`, strings.Join(rotted, `, `)))
	}
	return removed
}
