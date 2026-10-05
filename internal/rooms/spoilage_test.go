package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

func TestRemoveSpoiledGoods(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Raw Meat", SpoilAfter: "2 days"},
		2: {ItemId: 2, Name: "Bone"},
	})
	defer cleanup()

	fresh := items.Item{ItemId: 1, CraftedRound: 1000}
	rotten := items.Item{ItemId: 1, CraftedRound: 1000}
	end := rotten.SpoilRound()
	shopMeat := items.Item{ItemId: 1} // unstamped: never spoils
	bone := items.Item{ItemId: 2, CraftedRound: 1000}

	r := &Room{RoomId: 1, Items: []items.Item{rotten, bone, shopMeat}, Stash: []items.Item{fresh}}
	if n := r.removeSpoiledGoods(end - 1); n != 0 {
		t.Fatalf("nothing has rotted yet, removed %d", n)
	}
	if n := r.removeSpoiledGoods(end); n != 2 {
		t.Fatalf("the floor meat and the stashed meat both rot at the spoil round, removed %d", n)
	}
	if len(r.Items) != 2 || len(r.Stash) != 0 {
		t.Errorf("bone and unstamped meat stay, got floor %+v stash %+v", r.Items, r.Stash)
	}
}
