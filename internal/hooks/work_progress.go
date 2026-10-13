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
// a recipe that is no longer registered, so the line never goes blank. A name
// that already starts with an article ("The Blackrazor") takes no second one.
//
// An enchanting (its CraftingData carries the target slot) names the item being
// enchanted instead, as its start line "You begin enchanting <item>" does; an
// item no longer in that slot drops the name rather than guessing one.
func craftProgressLine(ch *characters.Character, cd activity.CraftingData) string {
	phrase := workProgressPhrase(cd.RoundsComplete, cd.RoundsTotal)
	if cd.TargetSlot != "" {
		if ch != nil {
			if itm := ch.Equipment.GetSlotPointer(cd.TargetSlot); itm != nil && itm.ItemId > 0 {
				return fmt.Sprintf(`<ansi fg="yellow">You keep enchanting <ansi fg="itemname">%s</ansi>; %s.</ansi>`,
					itm.DisplayName(), phrase)
			}
		}
		return fmt.Sprintf(`<ansi fg="yellow">You keep enchanting; %s.</ansi>`, phrase)
	}
	name := cd.RecipeId
	if recipe := crafting.GetRecipe(cd.RecipeId); recipe != nil && recipe.Name != "" {
		name = recipe.Name
	}
	if !startsWithArticle(name) {
		name = "the " + name
	}
	return fmt.Sprintf(`<ansi fg="yellow">You keep working on %s; %s.</ansi>`, name, phrase)
}

// startsWithArticle reports whether name opens with "the", "a" or "an" as a
// whole word, in any case.
func startsWithArticle(name string) bool {
	first, _, _ := strings.Cut(name, " ")
	switch strings.ToLower(first) {
	case "the", "a", "an":
		return true
	}
	return false
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
			name := itm.DisplayName()
			article := "the "
			if startsWithArticle(name) {
				article = "" // "A Beeswax Votive Candle" takes no second article
			}
			return fmt.Sprintf(`<ansi fg="yellow">You keep salvaging %s<ansi fg="itemname">%s</ansi>; %s.</ansi>`,
				article, name, phrase)
		}
	}
	return fmt.Sprintf(`<ansi fg="yellow">You keep salvaging; %s.</ansi>`, phrase)
}
