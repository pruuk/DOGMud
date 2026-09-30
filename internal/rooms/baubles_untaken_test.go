package rooms

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Found baubles left untaken for BaubleUntakenHours (24 by default) vanish;
// younger ones, carried-and-dropped ones and ordinary items stay.
func TestRemoveUntakenBaubles(t *testing.T) {
	now := time.Unix(7_000_000, 0)

	old := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000001`}
	old.LeaveBaubleAt(`on the shelf`, 5, now.Add(-25*time.Hour))
	young := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000002`}
	young.LeaveBaubleAt(``, 0, now.Add(-23*time.Hour))
	dropped := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000003`} // carried once: no mark
	sword := items.Item{ItemId: 1}

	r := &Room{RoomId: 5, Items: []items.Item{old, young, dropped, sword}}
	before := r.Items

	if n := r.removeUntakenBaubles(now); n != 1 {
		t.Fatalf("removed %d, want 1", n)
	}
	if len(r.Items) != 3 {
		t.Fatalf("left %+v", r.Items)
	}
	for _, it := range r.Items {
		if it.Bauble == `B0000001` {
			t.Fatal("the old one is gone")
		}
	}
	if before[0].Bauble != `B0000001` {
		t.Fatal("the old slice was not compacted in place")
	}

	if n := r.removeUntakenBaubles(now); n != 0 {
		t.Fatal("nothing more to remove")
	}
	if n := r.removeUntakenBaubles(now.Add(time.Hour)); n != 1 || len(r.Items) != 2 {
		t.Fatalf("the young one goes at 24 hours: %d %+v", n, r.Items)
	}
}

// A player's own room keeps everything on its floor: a find left there never
// expires.
func TestRemoveUntakenBaubles_NeverInAPrivateRoom(t *testing.T) {
	now := time.Unix(7_000_000, 0)
	SetPrivateRoomCheck(func(roomId int) bool { return roomId == 6 })
	defer SetPrivateRoomCheck(nil)

	old := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000009`}
	old.LeaveBaubleAt(``, 0, now.Add(-100*time.Hour))
	private := &Room{RoomId: 6, Items: []items.Item{old}}
	if n := private.removeUntakenBaubles(now); n != 0 || len(private.Items) != 1 {
		t.Fatalf("a private room lost its find: %d %+v", n, private.Items)
	}
	public := &Room{RoomId: 5, Items: []items.Item{old}}
	if n := public.removeUntakenBaubles(now); n != 1 {
		t.Fatal("an ordinary room kept an expired find")
	}
}
