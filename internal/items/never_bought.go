package items

import "sync"

// Items no merchant ever buys, whatever their value or vendor categories
// say. internal/housing registers the items its landlords sell (deeds,
// vouchers, guest keys): what a player pays for them is a gold sink, and
// selling them on to a shop must never give any of it back. The sell action
// (internal/actions/sell.go, sellOneToMerchant) refuses them.
var (
	neverBoughtMu sync.RWMutex
	neverBought   = map[int]bool{}
)

// SetNeverBought replaces the set of item ids no merchant buys.
func SetNeverBought(itemIds []int) {
	set := make(map[int]bool, len(itemIds))
	for _, id := range itemIds {
		set[id] = true
	}
	neverBoughtMu.Lock()
	neverBought = set
	neverBoughtMu.Unlock()
}

// IsNeverBought reports whether no merchant may buy itemId.
func IsNeverBought(itemId int) bool {
	neverBoughtMu.RLock()
	defer neverBoughtMu.RUnlock()
	return neverBought[itemId]
}
