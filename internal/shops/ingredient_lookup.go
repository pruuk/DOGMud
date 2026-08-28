package shops

import (
	"github.com/GoMudEngine/GoMud/internal/items"
)

// FindStockedIngredient resolves a recipe ingredient tag to the specific item a
// shop crafter will actually consume, choosing from what the shop HAS.
//
// 🔴 IT EXISTS TO REPLACE items.FindSpecByComponentTag ON THIS PATH, which
// ranges a Go map (`for _, spec := range items`) and therefore answers
// differently between calls. Four items share component_tag "bottle" (Clay
// Flask, Glass Vial, Sealed Phial, Crystalline Decanter). Two independent
// callers drew from that map and could disagree with each other:
//
//	HasMaterialsWithReservePct verified there was enough GLASS VIAL stock,
//	then executeCraft deducted a CRYSTALLINE DECANTER.
//
// That is not merely the wrong item. RemoveStockAtRound returns 0 when the
// drawn item is not stocked at all, so the mismatch could deduct NOTHING and
// the shop crafted for free — an unbounded goods faucet with no material cost,
// invisible because both halves looked correct in isolation.
//
// Resolution is deterministic and drawn from ShopInventory.Stock, which is a
// SLICE and so has a stable order:
//
//   - only items the shop actually stocks are considered, which is the whole
//     point: the availability check and the deduction now ask the same question
//     of the same data;
//   - among those, the LOWEST MaterialTier wins, matching the cheapest-material
//     rule player crafting uses, so a shop spends its clay flasks before its
//     decanters;
//   - ties break on stock order.
//
// Returns nil when the shop stocks nothing matching the tag. Callers must treat
// that as "cannot craft" rather than as "free".
func FindStockedIngredient(shopInv *ShopInventory, tag string) *items.ItemSpec {
	if shopInv == nil || tag == "" {
		return nil
	}

	var best *items.ItemSpec
	for i := range shopInv.Stock {
		spec := items.GetItemSpec(shopInv.Stock[i].ItemId)
		if spec == nil || spec.ComponentTag != tag {
			continue
		}
		if best == nil || spec.MaterialTier < best.MaterialTier {
			best = spec
		}
	}
	return best
}
