package items

import (
	"github.com/GoMudEngine/GoMud/internal/gametime"
)

// Raw-goods spoilage (wilderness trades). A raw material whose spec declares
// SpoilAfter (a game-time period such as "1 day") rots that long after it was
// harvested. Harvest stamps Item.CraftedRound with the round the material
// came off the carcass; that is the clock.
//
// An instance with CraftedRound == 0 never spoils. Every shop-bought or
// pre-existing raw material is unstamped, so adding SpoilAfter to an existing
// spec (raw meat) cannot rot anything a player already owns, and vendor stock
// stays fresh. Only harvested goods age.
//
// Processing is how a hunter stops the clock: curing a hide, smoking meat or
// rendering fat produces a different item whose spec has no SpoilAfter.

// Spoils reports whether this instance is on a spoilage clock at all.
func (i *Item) Spoils() bool {
	if i.ItemId < 1 || i.CraftedRound == 0 {
		return false
	}
	return i.GetSpec().SpoilAfter != ``
}

// SpoilRound is the round at which this instance rots, or 0 when it never
// does (see Spoils).
func (i *Item) SpoilRound() uint64 {
	if !i.Spoils() {
		return 0
	}
	return gametime.GetDate(i.CraftedRound).AddPeriod(i.GetSpec().SpoilAfter)
}

// IsSpoiled reports whether the instance has rotted by round now.
func (i *Item) IsSpoiled(now uint64) bool {
	r := i.SpoilRound()
	return r > 0 && now >= r
}

// Freshness is how much of the item's shelf life remains at round now, from
// 1.0 (just harvested) to 0.0 (rotten). Items that never spoil are always 1.0.
func (i *Item) Freshness(now uint64) float64 {
	end := i.SpoilRound()
	if end == 0 {
		return 1.0
	}
	if now >= end {
		return 0
	}
	if now <= i.CraftedRound || end <= i.CraftedRound {
		return 1.0
	}
	return float64(end-now) / float64(end-i.CraftedRound)
}

// FreshnessValueMultiplier scales a sell price by freshness: full price while
// fresh, sliding to half price at the moment it spoils. Rotten goods are not
// bought at all (shops.EvaluateBuyRules refuses them).
func (i *Item) FreshnessValueMultiplier(now uint64) float64 {
	return 0.5 + 0.5*i.Freshness(now)
}
