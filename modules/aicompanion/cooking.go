package aicompanion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Trades (cooking and anything else a profile lists). A companion knows the
// beginner recipes of the trades its profile names, and can work them
// wherever the game lets a player work them: at a station room such as a
// cooking fire, or anywhere at all for recipes that need none.
//
// DOGMud has no way to build a fire: `station` is a property of a room, and
// the tinderbox in the shops is scenery. So a companion cooks at an inn
// hearth or a camp, not on the open road, exactly as a player does. Nothing
// here gives it an ability a player lacks.

// seedRecipes teaches a companion the beginner recipes of its trades. Their
// skills rise by use like anyone's, and the recipes are re-seeded whenever
// it is spawned, so nothing depends on the mob save carrying them.
func seedRecipes(mob *mobs.Mob, p *Profile) {
	if len(p.Crafts) == 0 {
		return
	}
	if mob.Character.KnownRecipes == nil {
		mob.Character.KnownRecipes = map[string]int{}
	}
	for _, skill := range p.Crafts {
		for _, r := range crafting.GetAllForSkill(strings.ToLower(strings.TrimSpace(skill))) {
			if r == nil || r.LearnOnly || r.SkillMinimum > 0 {
				continue
			}
			if _, known := mob.Character.KnownRecipes[r.RecipeId]; !known {
				mob.Character.KnownRecipes[r.RecipeId] = 1
			}
		}
	}
}

// recipeOption is something the companion could make right here and now.
type recipeOption struct {
	Ref    string
	Id     string
	Name   string
	Skill  string
	Rounds int
}

// craftableHere lists what it knows, has the makings for, and has the place
// for: the same three tests the player craft command applies.
func craftableHere(mob *mobs.Mob, p *Profile, room *rooms.Room) []recipeOption {
	if room == nil || len(p.Crafts) == 0 || len(mob.Character.KnownRecipes) == 0 {
		return nil
	}
	// Crafting needs clear sight, for her as for anyone (slice 5a): offer
	// nothing the actual craft attempt would refuse, so autonomy never picks
	// a craft that would not start and the prompt lists no recipe she cannot
	// see to make. This reuses actions.TooDarkToCraft, the same predicate
	// InitiateCraft asks, rather than cannotSee (perception.go), which does
	// not consult sleep: a sleeping companion in a lit room could otherwise
	// list a recipe her own craft attempt then silently refused.
	if actions.TooDarkToCraft(actions.NewMobActorInRoom(mob, room)) {
		return nil
	}
	ids := make([]string, 0, len(mob.Character.KnownRecipes))
	for id := range mob.Character.KnownRecipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var out []recipeOption
	for _, id := range ids {
		r := crafting.GetRecipe(id)
		if r == nil || r.LearnOnly {
			continue
		}
		if !actions.StationSatisfied(&mob.Character, r.Station, room.Station) {
			continue
		}
		if ok, _ := crafting.HasIngredients(mob.Character.Items, mob.Character.ComponentItems, r); !ok {
			continue
		}
		out = append(out, recipeOption{
			Ref: fmt.Sprintf(`k%d`, len(out)+1), Id: r.RecipeId, Name: r.Name, Skill: r.Skill, Rounds: r.TimeRounds,
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// craftLinesFor renders what it could make, for the prompt.
func craftLinesFor(opts []recipeOption) []string {
	var out []string
	for _, o := range opts {
		line := fmt.Sprintf(`[%s] %s (%s`, o.Ref, o.Name, o.Skill)
		if o.Rounds > 1 {
			line += fmt.Sprintf(`, takes a little while`)
		}
		out = append(out, line+`)`)
	}
	return out
}

// findRecipeOption resolves a [k] ref from the list the model was shown.
func findRecipeOption(opts []recipeOption, ref string) (recipeOption, bool) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	for _, o := range opts {
		if o.Ref == ref {
			return o, true
		}
	}
	return recipeOption{}, false
}

// rawFoodCount counts ingredients she is carrying for the trades she works,
// which is what makes cooking worth doing when she passes a fire.
func rawFoodCount(mob *mobs.Mob, p *Profile) int {
	if len(p.Crafts) == 0 {
		return 0
	}
	wanted := map[string]bool{}
	for _, skill := range p.Crafts {
		for _, r := range crafting.GetAllForSkill(strings.ToLower(strings.TrimSpace(skill))) {
			if r == nil || r.SkillMinimum > 0 {
				continue
			}
			for _, ing := range r.Ingredients {
				wanted[strings.ToLower(ing.ItemTag)] = true
			}
		}
	}
	n := 0
	count := func(list []items.Item) {
		for i := range list {
			if tag := list[i].GetSpec().ComponentTag; tag != `` && wanted[strings.ToLower(tag)] {
				n++
			}
		}
	}
	count(mob.Character.Items)
	count(mob.Character.ComponentItems)
	return n
}

// equipStartingKit gives a companion the few things its profile says it
// owns, and puts on what it can wear.
func equipStartingKit(mob *mobs.Mob, p *Profile) {
	for _, id := range p.StartingItems {
		if id <= 0 {
			continue
		}
		it := items.New(id)
		if it.ItemId == 0 {
			continue
		}
		if !mob.Character.StoreItem(it) {
			continue
		}
		if isWearable(&it) {
			if returned, worn, _ := mob.Character.Wear(it); worn {
				mob.Character.RemoveItem(it)
				for _, back := range returned {
					mob.Character.ReturnToPack(back) // never refuses: it was worn a moment ago
				}
			}
		}
	}
}
