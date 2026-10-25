package gmcp

// Item-template reference scan for Build.Item.Delete. Lives in the gmcp layer
// (not internal/items) because it imports mobs/rooms/quests/crafting, which
// themselves import items — putting it in items would cycle. The scan runs
// against injected iterators so it is unit-testable without those packages.

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

type mobRef struct {
	id   int
	name string
	ids  []int
}

// refIterators yields each candidate reference source; injected so the scan is
// testable with fakes.
type refIterators struct {
	mobs       func(yield func(mobRef))
	recipes    func(yield func(recipeId string, outputId int))
	containers func(yield func(roomId int, itemIds []int))
	// questRefs is what the delete would break at the boot doors: the quest
	// loop's refs rule and the dialogue item check (itemQuestRefs).
	questRefs func(itemId int) []itemRef
}

func scanItemReferencesWith(itemId int, it refIterators) []itemRef {
	out := []itemRef{}
	if it.mobs != nil {
		it.mobs(func(m mobRef) {
			if containsInt(m.ids, itemId) {
				out = append(out, itemRef{Kind: "mob", Id: fmt.Sprintf("mob %d (%s)", m.id, m.name)})
			}
		})
	}
	if it.recipes != nil {
		it.recipes(func(rid string, outputId int) {
			if outputId == itemId {
				out = append(out, itemRef{Kind: "recipe", Id: "recipe " + rid})
			}
		})
	}
	if it.containers != nil {
		it.containers(func(roomId int, itemIds []int) {
			if containsInt(itemIds, itemId) {
				out = append(out, itemRef{Kind: "container", Id: fmt.Sprintf("room %d chest", roomId)})
			}
		})
	}
	if it.questRefs != nil {
		out = append(out, it.questRefs(itemId)...)
	}
	return out
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// scanItemReferences wires the real world iterators. Recipe INGREDIENTS reference
// items by component tag (not id), so only the recipe OUTPUT id is scanned. Shop
// stock is dynamic living state (self-heals on restock), so it is NOT scanned.
// Quests and dialogue are checked by the boot doors' own rules
// (itemQuestRefs), which visit every item field they carry.
func scanItemReferences(itemId int) []itemRef {
	return scanItemReferencesWith(itemId, refIterators{
		questRefs: itemQuestRefs,
		mobs: func(yield func(mobRef)) {
			for _, m := range mobs.AllMobTemplates() {
				ids := append(append([]int{}, m.LootPool...), m.CrafterRestockMaterials...)
				yield(mobRef{id: int(m.MobId), name: m.Character.Name, ids: ids})
			}
		},
		recipes: func(yield func(string, int)) {
			for id, r := range crafting.GetAll() {
				yield(id, r.Output.ItemId)
			}
		},
		containers: func(yield func(int, []int)) {
			for _, roomId := range rooms.GetAllRoomIds() {
				r := rooms.LoadRoom(roomId)
				if r == nil || len(r.Containers) == 0 {
					continue
				}
				ids := []int{}
				for _, c := range r.Containers {
					for _, it := range c.Items {
						ids = append(ids, it.ItemId)
					}
				}
				if len(ids) > 0 {
					yield(roomId, ids)
				}
			}
		},
	})
}
