package hooks

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// workProgressPhrase turns a timed activity's round count into a few words with
// no numbers (player copy never shows raw counters, #513). The edges are
// inclusive thirds, so the word shifts on the round a player would expect:
//
//	done <= total/3    -> "you've only just started"
//	done >= 2*total/3  -> "it's nearly done"
//	otherwise          -> "you're about halfway"
//
// Compared in integers (3*done vs total) to avoid rounding. A 2-round work reads
// halfway on its one tick; a 3-round work reads started then nearly done; 4 and
// 5 rounds read started, halfway, (halfway,) nearly done.
func workProgressPhrase(done, total int) string {
	switch {
	case total <= 0 || 3*done >= 2*total:
		return "it's nearly done"
	case 3*done <= total:
		return "you've only just started"
	default:
		return "you're about halfway"
	}
}

// craftProgressLine is the per-round line a crafter sees. It names the recipe the
// way the craft start line does (the recipe's Name); the id is only a fallback for
// a recipe that is no longer registered, so the line never goes blank.
func craftProgressLine(cd activity.CraftingData) string {
	name := cd.RecipeId
	if recipe := crafting.GetRecipe(cd.RecipeId); recipe != nil && recipe.Name != "" {
		name = recipe.Name
	}
	return fmt.Sprintf(`<ansi fg="yellow">You keep working on the %s; %s.</ansi>`,
		name, workProgressPhrase(cd.RoundsComplete, cd.RoundsTotal))
}

// salvageProgressLine is the per-round line a salvager sees. An item salvage
// names the item (found by uuid among everything carried, as the start line
// names it); a corpse salvage says "the corpse"; an item that can no longer be
// found drops the name rather than guessing one.
func salvageProgressLine(ch *characters.Character, sd activity.SalvagingData) string {
	phrase := workProgressPhrase(sd.RoundsComplete, sd.RoundsTotal)
	if strings.HasPrefix(sd.ItemUuid, "corpse:") {
		return fmt.Sprintf(`<ansi fg="yellow">You keep working over the corpse; %s.</ansi>`, phrase)
	}
	for _, itm := range ch.GetAllCarriedItems() {
		if itm.UUID.String() == sd.ItemUuid {
			return fmt.Sprintf(`<ansi fg="yellow">You keep salvaging the <ansi fg="itemname">%s</ansi>; %s.</ansi>`,
				itm.DisplayName(), phrase)
		}
	}
	return fmt.Sprintf(`<ansi fg="yellow">You keep salvaging; %s.</ansi>`, phrase)
}
