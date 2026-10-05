package gather

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// Tool is the tool a gathering job will use: which item it is, and what that
// item counts as for this job.
type Tool struct {
	Item       items.Item
	Type       items.ToolType
	Tier       items.ToolTier // effective tier: authored tier nudged by the instance grade
	Speed      float64        // >1 finishes jobs faster
	Improvised bool           // a weapon standing in for the tool (always crude)
}

// Mult is the tool's multiplier on the gatherer's stat term, read from
// Balance.ToolMult*.
func (t Tool) Mult() float64 {
	return TierMult(t.Tier)
}

// TierMult is the Balance multiplier for a tool tier. ToolTierNone (no tool
// at all) is treated as crude: working bare-handed is no better than working
// with an improvised edge.
func TierMult(tier items.ToolTier) float64 {
	b := configs.GetBalanceConfig()
	switch tier {
	case items.ToolTierIron:
		return float64(b.ToolMultIron)
	case items.ToolTierSteel:
		return float64(b.ToolMultSteel)
	case items.ToolTierMasterwork:
		return float64(b.ToolMultMasterwork)
	}
	return float64(b.ToolMultCrude)
}

// BestTool finds the best tool of type want that the character is wielding,
// wearing or carrying. "Best" is highest effective tier, then a real tool over
// an improvised one, then the faster tool. Equipped items are checked before
// carried ones, so a tie goes to what is in hand.
//
// Items in the bandolier and component bag are not searched: a knife does not
// live there.
func BestTool(c *characters.Character, want items.ToolType) (Tool, bool) {
	if c == nil {
		return Tool{}, false
	}
	candidates := append(c.Equipment.GetAllItems(), c.Items...)
	return bestToolFrom(candidates, want)
}

// bestToolFrom is BestTool over an explicit item list (pure, for tests).
func bestToolFrom(candidates []items.Item, want items.ToolType) (Tool, bool) {
	var best Tool
	found := false
	for _, itm := range candidates {
		t, ok := asTool(itm, want)
		if !ok {
			continue
		}
		if !found || better(t, best) {
			best = t
			found = true
		}
	}
	return best, found
}

func asTool(itm items.Item, want items.ToolType) (Tool, bool) {
	if itm.ItemId < 1 || itm.IsBroken() {
		return Tool{}, false // a broken tool is no use until it is repaired
	}
	spec := itm.GetSpec()
	if spec.Tool != nil {
		if spec.Tool.Type != want {
			return Tool{}, false
		}
		speed := spec.Tool.Speed
		if speed <= 0 {
			speed = 1.0
		}
		return Tool{
			Item:  itm,
			Type:  want,
			Tier:  items.EffectiveToolTier(spec.Tool.Tier, itm.Quality),
			Speed: speed,
		}, true
	}
	if tier, ok := items.ImprovisedTool(spec, want); ok {
		return Tool{Item: itm, Type: want, Tier: tier, Speed: 1.0, Improvised: true}, true
	}
	return Tool{}, false
}

func better(a, b Tool) bool {
	if a.Tier != b.Tier {
		return a.Tier > b.Tier
	}
	if a.Improvised != b.Improvised {
		return !a.Improvised
	}
	return a.Speed > b.Speed
}

// RareMult is the Balance multiplier a tool tier puts on the chance of a rare
// find (RareToolMult*). ToolTierNone is treated as crude.
func RareMult(tier items.ToolTier) float64 {
	b := configs.GetBalanceConfig()
	switch tier {
	case items.ToolTierIron:
		return float64(b.RareToolMultIron)
	case items.ToolTierSteel:
		return float64(b.RareToolMultSteel)
	case items.ToolTierMasterwork:
		return float64(b.RareToolMultMasterwork)
	}
	return float64(b.RareToolMultCrude)
}

// WearTool adds one finished job's wear to the tool t names, on the
// character's own copy of it (wielded, worn or carried). When that breaks the
// tool its name is returned; otherwise "". A broken tool stays where it is,
// useless until repaired. An improvised weapon never wears as a tool.
func WearTool(c *characters.Character, t Tool) (brokenName string) {
	if c == nil || t.Improvised || t.Item.ItemId < 1 {
		return ``
	}
	for _, p := range c.Equipment.GetAllItemPtrs() {
		if !p.Equals(t.Item) {
			continue
		}
		if p.AddToolWear(1) {
			return p.NameSimple()
		}
		return ``
	}
	for j := range c.Items {
		if !c.Items[j].Equals(t.Item) {
			continue
		}
		if c.Items[j].AddToolWear(1) {
			return c.Items[j].NameSimple()
		}
		return ``
	}
	return ``
}
