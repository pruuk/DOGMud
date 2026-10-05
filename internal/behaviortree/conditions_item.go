package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Item-subject conditions (lighting 5e, Rule 7). Each needs an item
// subject: the compiler refuses them in a mob or room tree (Rule 8).

// itemHolder is the character holding the subject item: its player or its
// mob. Nil for a floor item, a mob or room tree, or a holder that is gone.
func itemHolder(ctx *EvalContext) *characters.Character {
	if ctx == nil || ctx.Item == nil {
		return nil
	}
	if ctx.Item.UserId > 0 {
		if u := users.GetByUserId(ctx.Item.UserId); u != nil {
			return u.Character
		}
		return nil
	}
	if ctx.Item.MobInstanceId > 0 {
		if m := mobs.GetInstance(ctx.Item.MobInstanceId); m != nil {
			return &m.Character
		}
	}
	return nil
}

// condHolderAsleep: the holder holds the Sleeping flag. It reads
// actions.TargetAsleep, the predicate ShopClosedForSleep reads, not the
// schedule, so a keeper roused inside a sleeping segment counts as awake
// (owner ruling R2, spec X3).
func condHolderAsleep(_ map[string]any, ctx *EvalContext) Result {
	if c := itemHolder(ctx); c != nil && actions.TargetAsleep(c) {
		return Success
	}
	return Failure
}

// condWorn: the item is in an equipment slot.
func condWorn(_ map[string]any, ctx *EvalContext) Result {
	if ctx != nil && ctx.Item != nil && ctx.Item.Slot != `` {
		return Success
	}
	return Failure
}

// condInCombat: the holder is in combat.
func condInCombat(_ map[string]any, ctx *EvalContext) Result {
	if c := itemHolder(ctx); c != nil && c.IsInCombat() {
		return Success
	}
	return Failure
}
