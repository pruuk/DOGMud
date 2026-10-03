package baubles

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Stolen goods that are not baubles: a merchant's goods taken out of its
// chest (internal/merchantchests). They carry their theft on the item itself
// (items.Item.StolenAt, StolenZone, StolenBy, StolenFromMob) rather than in
// a catalog record, but follow the same rules as a stolen bauble: hot for
// HeatDuration (Balance.BaubleStolenHeatHours) after the theft, and hot for
// selling, storing and auctioning only in the heat area it was taken in
// (HeatArea, Balance.BaubleHeatAreas). Anywhere else, or once cooled, they
// are ordinary goods to an honest merchant; a fence pays its cut for them
// either way.

// GoodsHot reports whether itm is stolen goods whose theft is recent:
// taken less than HeatDuration before now. While it is, its owner may
// recognise it on the thief anywhere, and a guard in the area of the theft
// may too (actions/stolen_bauble.go).
func GoodsHot(itm items.Item, now time.Time) bool {
	if !itm.IsStolen() {
		return false
	}
	return now.Before(time.Unix(itm.StolenAt, 0).Add(HeatDuration()))
}

// GoodsHotIn reports whether itm is hot (GoodsHot) in zone: in the same
// heat area as the zone it was taken in. Goods whose theft zone is unknown
// are hot everywhere while hot.
func GoodsHotIn(itm items.Item, zone string, now time.Time) bool {
	if !GoodsHot(itm, now) {
		return false
	}
	return itm.StolenZone == `` || HeatArea(itm.StolenZone) == HeatArea(zone)
}
