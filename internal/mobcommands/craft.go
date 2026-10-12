package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Craft lets a mob start a crafting operation via `craft <recipe-name>`.
// Useful for craftsperson NPC archetypes (blacksmiths, alchemists, etc.)
// that should visibly work on items during roleplay or scripted scenes.
//
// Completion is handled each round by the MobRoundTick crafting block.
func Craft(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return true, nil
	}

	actor := &actions.MobActor{Mob: mob, Room: room}
	result := actions.InitiateCraft(actor, rest)

	switch {
	case result.CannotSee:
		// Cannot see to work; waits for light (no-op for mobs)
		return true, nil

	case result.AlreadyCrafting:
		// Mob is already working; ignore silently (scripts may call too eagerly)
		return true, nil

	case result.RecipeNotFound, result.RecipeNotKnown:
		// Unknown or unavailable recipe — no-op for mobs
		return true, nil

	case result.SkillTooLow:
		// Mob's skill is insufficient; no-op
		return true, nil

	case result.WrongStation:
		// Not at the right station; no-op
		return true, nil

	case result.MissingIngredients:
		// Mob lacks materials; no-op
		return true, nil

	case result.ImmediateComplete:
		room.SendTextVisual(messaging.CategoryMobIdle, result.Recipe.MobRoomLine(
			crafting.PhaseSuccess, mob.Character.Name, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> works quickly and produces something.`,
				mob.Character.Name)))
		// U10b-1 Task 16. won is unconditionally TRUE: ImmediateComplete is a
		// TimeRounds <= 0 recipe, which InitiateCraft finishes without rolling.
		// See the identical note in internal/usercommands/craft.go.
		// U10b-3: recipe difficulty moved to discovery; no bonus rides here now.
		mob.Character.AwardResolved(0, true,
			mob.Character.CandidateFor(result.SkillName))
		return true, nil

	case result.Initiated:
		room.SendTextVisual(messaging.CategoryMobIdle, craftBeginLine(
			mob.Character.Name, result.Recipe))
		return true, nil
	}

	return true, nil
}

// craftBeginLine is the room line for a mob starting a timed craft. It names
// the recipe's output item; a recipe with no resolvable output (an enchant
// recipe, or an unknown item id) keeps the vague "something".
func craftBeginLine(mobName string, recipe *crafting.RecipeSpec) string {
	what := `something`
	if recipe != nil && items.GetItemSpec(recipe.Output.ItemId) != nil {
		itm := items.New(recipe.Output.ItemId)
		what = fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, itm.DisplayName())
	}
	return fmt.Sprintf(`<ansi fg="mobname">%s</ansi> begins working on %s.`, mobName, what)
}
