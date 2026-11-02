package questengine

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// heldAtStepStart checks the granted quest's plain item_gain triggers as a step
// starts: one whose conditions pass and whose item the player already holds
// fires at once, telling the player why (its held_line, else the default line
// naming the item). An item picked up before the step began raises no item_gain
// of its own, so without this the step could not finish (#375 phase 7 tier 1,
// owner ruling: a held item is handled, never dropped and picked up again). It
// runs inside evaluate beside exploreAtStepStart, so its chain is bounded by the
// guard like any other.
func (e *Engine) heldAtStepStart(d EventDetails, player PlayerState, ctx ActionContext, guard *EvalGuard) NotifyResult {
	questId, _ := quests.TokenToParts(d.QuestToken)
	result := NotifyResult{}
	for _, t := range e.heldByQuest[questId] {
		if !player.HasItem(t.def.Item) || !EvalConditions(t.def.Conditions, player) {
			continue
		}
		line := heldLine(t.def)
		result.merge(e.fire("held", t, d, player, ctx, guard, func() {
			ctx.Narrate(narration.Variants{Actor: textutil.Pool(line)})
		}))
	}
	return result
}

func heldLine(t *TriggerDef) string {
	if t.HeldLine != "" {
		return t.HeldLine
	}
	if spec := items.GetItemSpec(t.Item); spec != nil && spec.Name != "" {
		return fmt.Sprintf("You already have the %s.", spec.Name)
	}
	return "You already have that."
}
