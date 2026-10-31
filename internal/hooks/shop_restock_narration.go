package hooks

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

var (
	restockNarrationMu sync.Mutex
	// restockNarratedAt is the round each shop mob (by instance id) last
	// announced a delivery. Bounded by the shop mobs alive; in memory only.
	restockNarratedAt = map[int]uint64{}
)

// restockNarrationDue reports whether a shop's restock at round now should
// print its delivery line, and records it when it does. Common stock refills
// every game hour, a couple of real minutes, so narrating each one spammed a
// player standing in the shop (#725). The restock itself always happens; only
// the line waits for Balance.ShopRestockNarrationRounds to pass. 0 narrates
// every restock.
func restockNarrationDue(instanceId int, now uint64) bool {
	window := uint64(configs.GetBalanceConfig().ShopRestockNarrationRounds)
	restockNarrationMu.Lock()
	defer restockNarrationMu.Unlock()
	if last, ok := restockNarratedAt[instanceId]; ok && window > 0 && now < last+window {
		return false
	}
	restockNarratedAt[instanceId] = now
	return true
}

// resetRestockNarrationForTest empties the narration memory and returns a
// restore func.
func resetRestockNarrationForTest() func() {
	restockNarrationMu.Lock()
	orig := restockNarratedAt
	restockNarratedAt = map[int]uint64{}
	restockNarrationMu.Unlock()
	return func() {
		restockNarrationMu.Lock()
		restockNarratedAt = orig
		restockNarrationMu.Unlock()
	}
}
