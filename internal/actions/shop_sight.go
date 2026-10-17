package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ShopSightRefusal is the sight gate on dealing (lighting plan 5b): below the
// faces band you cannot make out the goods, so list, buy and sell refuse. It
// mirrors ShopClosedForSleep beside it. Above faces the deal goes through and
// the bartering discount takes SightMult, so a dazzled haggler bargains worse.
//
// A nil room reads as no refusal, since callers that reach this gate have
// already handled "no room at all" their own way, so this never has to.
// Gating here also keeps a typed-nil *rooms.Room from reaching
// messaging.LightBand, which takes it as a non-nil RoomVisibility and would
// panic reading it.
func ShopSightRefusal(c *characters.Character, room *rooms.Room) bool {
	if room == nil {
		return false
	}
	return messaging.LightBand(c, room) < messaging.BandFaces
}

// MerchantHere reports whether room holds anyone to deal with: a merchant mob,
// or a merchant player other than selfUserId. Every shop verb asks it (or its
// own narrower merchant check) BEFORE ShopSightRefusal, so a room with no
// merchant answers with the verb's no-merchant line at every light, and the
// sight refusal never hints at a merchant who is not there (2026-10-17
// pre-deploy playtest).
func MerchantHere(room *rooms.Room, selfUserId int) bool {
	if room == nil {
		return false
	}
	if len(room.GetMobs(rooms.FindMerchant)) > 0 {
		return true
	}
	for _, uid := range room.GetPlayers(rooms.FindMerchant) {
		if uid != selfUserId {
			return true
		}
	}
	return false
}

// ShopSightRefusalText is the one line every refusing verb prints.
const ShopSightRefusalText = "You can't make out the goods well enough to deal."

// barterDiscount is the ONE place a shop's bartering discount is computed
// (lighting plan 5b): the raw skill-derived discount, capped at maxFrac,
// times SightMult, so a dazzled haggler bargains worse than a comfortable
// one, who reads SightMult 1.0 and sees no change at all.
//
// maxFrac is the shipped knob, never a Go literal: Balance.BarterMaxDiscount
// for a buyer's price cut, Balance.BarterMaxBonus for a seller's price bump
// (both from configs.GetBalanceConfig(), read by the caller since buy and
// sell name different knobs).
//
// A nil room reads as comfortable (SightMult 1.0, unmultiplied), the same
// reading messaging.SightMult itself gives a nil room. This only matters to
// a caller exercising tryPurchaseFromInventory directly with no Room set on
// its Actor fixture, and a typed-nil *rooms.Room must never reach SightMult
// (see ShopSightRefusal's doc comment).
func barterDiscount(char *characters.Character, room *rooms.Room, maxFrac float64) float64 {
	barterSkill := char.GetSkillLevel(skills.Bartering)
	if barterSkill <= 0 {
		return 0
	}
	discount := float64(barterSkill) / 50.0 * maxFrac
	if discount > maxFrac {
		discount = maxFrac
	}
	if room == nil {
		return discount
	}
	return discount * messaging.SightMult(char, room)
}
