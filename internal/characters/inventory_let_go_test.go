package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// #940: a command that lets an item go takes an ordinary copy before the
// quest's, in the backpack, the bandolier and the component bag. A handle or
// an explicit N. is the player's own pick and is kept.
func TestFindToLetGoPrefersAnOrdinaryCopy(t *testing.T) {
	const saltId, otherId = 94701, 94702
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		saltId:  {ItemId: saltId, Name: "Pouch of Salt", Type: items.Object},
		otherId: {ItemId: otherId, Name: "Pouch of Shot", Type: items.Object},
	}))
	quest := items.New(saltId)
	quest.QuestGiven = true
	bought := items.New(saltId)

	for name, run := range map[string]func(c *Character) (items.Item, bool){
		"backpack": func(c *Character) (items.Item, bool) {
			c.Items = []items.Item{quest, bought}
			return c.FindInBackpackToLetGo("salt")
		},
		"bandolier": func(c *Character) (items.Item, bool) {
			c.PotionItems = []items.Item{quest, bought}
			return c.FindInPotionsToLetGo("salt")
		},
		"components": func(c *Character) (items.Item, bool) {
			c.ComponentItems = []items.Item{quest, bought}
			return c.FindInComponentsToLetGo("salt")
		},
	} {
		got, ok := run(New())
		if !ok || got.UUID != bought.UUID {
			t.Errorf("%s: picked %+v, want the bought copy", name, got)
		}
	}

	c := New()
	c.Items = []items.Item{quest}
	if got, ok := c.FindInBackpackToLetGo("salt"); !ok || !got.IsQuestGiven() {
		t.Fatal("with only the quest copy, it is the one found")
	}

	c.Items = []items.Item{quest, bought}
	if got, _ := c.FindInBackpackToLetGo(ItemHandleSigil + quest.UUID.String()); got.UUID != quest.UUID {
		t.Fatal("a handle names the quest copy itself")
	}

	// An explicit count, 1 included, is the player's own pick.
	for _, typed := range []string{"1.salt", "salt#1"} {
		if got, _ := c.FindInBackpackToLetGo(typed); got.UUID != quest.UUID {
			t.Fatalf("%s: picked %+v, want the first copy as typed", typed, got)
		}
	}

	// Never another item the name also fits.
	c.Items = []items.Item{quest, items.New(otherId)}
	if got, _ := c.FindInBackpackToLetGo("pouch of salt"); got.ItemId != saltId {
		t.Fatalf("swapped in a different item: %d", got.ItemId)
	}
}
