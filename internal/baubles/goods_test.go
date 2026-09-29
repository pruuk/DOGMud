package baubles

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Stolen goods from a merchant chest heat and cool like a stolen bauble:
// hot for HeatDuration (three real days by default), and only in the heat
// area they were taken in.
func TestGoodsHeat(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	it := items.Item{ItemId: 1, StolenFrom: "Smith Brindle", StolenFromMob: 337}
	if GoodsHot(it, now) {
		t.Fatal("still in the chest: not stolen, not hot")
	}
	it.MarkTaken(1, "Stillwater", now.Add(-time.Hour))

	if !GoodsHot(it, now) || !GoodsHotIn(it, "Stillwater", now) {
		t.Fatal("taken an hour ago in Stillwater: hot there")
	}
	if GoodsHotIn(it, "Thornwall City", now) {
		t.Fatal("another town has not heard of it")
	}
	if !ItemIsHotIn(it, "Stillwater", now) || ItemIsHotIn(it, "Thornwall City", now) {
		t.Fatal("ItemIsHotIn, which storage and the auction house ask, agrees")
	}
	later := now.Add(HeatDuration())
	if GoodsHot(it, later) || GoodsHotIn(it, "Stillwater", later) {
		t.Fatal("cooled after the heat duration")
	}
	if !it.IsStolen() {
		t.Fatal("cooled goods are still stolen goods, for the fence's cut")
	}
}
