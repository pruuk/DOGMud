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
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
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

// seedRecipes teaches a companion the recipes of its trades that its skill
// has reached (the beginner ones at least). Their
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
		skill = strings.ToLower(strings.TrimSpace(skill))
		// Beyond the beginner recipes, whatever her own skill has reached:
		// a seasoned cook knows more dishes than a camp hand, and one who
		// cooks often learns more, as a player discovers recipes by rank.
		// The same reading the engine's own craft gate makes (actions
		// craft.go), gear and conditions included.
		rank := mob.Character.GetSkillLevel(skills.SkillTag(skill))
		for _, r := range crafting.GetAllForSkill(skill) {
			if r == nil || r.LearnOnly || r.SkillMinimum > rank {
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
	Min    int // the recipe's skill_minimum: how accomplished a piece of work it is
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
		// The rest of InitiateCraft's gates, so nothing is offered that the
		// attempt would refuse: her skill, components she made herself where
		// the recipe insists, and no enchanting (it needs an item to work on,
		// and the engine's mob path declines it).
		if mob.Character.GetSkillLevel(skills.SkillTag(r.Skill)) < r.SkillMinimum || crafting.IsEnchantingRecipe(r) {
			continue
		}
		if ok, _ := crafting.CheckOwnComponents(r, mob.Character.Items, mob.Character.ComponentItems, mob.Character.Name); !ok {
			continue
		}
		// The craft command finds a recipe by name, as a player types it,
		// not by id. Only offer one whose name finds this very recipe.
		if !craftsAs(mob, r) {
			continue
		}
		out = append(out, recipeOption{
			Ref: fmt.Sprintf(`k%d`, len(out)+1), Id: r.RecipeId, Name: r.Name, Skill: r.Skill, Rounds: r.TimeRounds, Min: r.SkillMinimum,
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// craftableElsewhere lists what she knows and has the makings for, but not
// the place: a cooking fire for the meat she was handed, a bench for the
// herbs. So the model can say so, and take her where there is one, rather
// than not knowing the meat in her pack could be cooked at all.
func craftableElsewhere(mob *mobs.Mob, p *Profile, room *rooms.Room) []string {
	if room == nil || len(p.Crafts) == 0 || len(mob.Character.KnownRecipes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(mob.Character.KnownRecipes))
	for id := range mob.Character.KnownRecipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []string
	for _, id := range ids {
		r := crafting.GetRecipe(id)
		if r == nil || r.LearnOnly || r.Station == `` || actions.StationSatisfied(&mob.Character, r.Station, room.Station) {
			continue
		}
		if ok, _ := crafting.HasIngredients(mob.Character.Items, mob.Character.ComponentItems, r); !ok {
			continue
		}
		if mob.Character.GetSkillLevel(skills.SkillTag(r.Skill)) < r.SkillMinimum || crafting.IsEnchantingRecipe(r) {
			continue
		}
		out = append(out, fmt.Sprintf(`%s (%s), at %s`, r.Name, r.Skill, stationWords(r.Station)))
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// craftsAs reports whether "craft <recipe name>" from her resolves to this
// recipe, as actions.InitiateCraft resolves it: the tightest name match
// among the recipes she knows.
func craftsAs(mob *mobs.Mob, r *crafting.RecipeSpec) bool {
	for _, c := range crafting.FindRecipesByName(r.Name) {
		if mob.Character.HasRecipe(c.RecipeId) {
			return c.RecipeId == r.RecipeId
		}
	}
	return false
}

// craftCommand is the ordinary craft command for a recipe: by name, as a
// player would type it. "craft <id>" finds nothing for any recipe whose id
// is not also its name (grilled-meat is "Grilled Meat").
func craftCommand(o recipeOption) string {
	return `craft ` + strings.ToLower(o.Name)
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

// callingDrive is how much a trade must mean to her (her archetype's skill
// preference, 0..1) before it is a calling: something she sets to work at
// whenever she can, rather than now and then when nothing else is on.
const callingDrive = 0.5

// tradeDrive is how much a trade means to her: her archetype's preference
// for that skill.
func tradeDrive(p *Profile, skill string) float64 {
	if p == nil {
		return 0
	}
	return p.Archetype.Skills[strings.ToLower(strings.TrimSpace(skill))]
}

// craftChoice is the piece of work she would pick, and how much its trade
// means to her.
type craftChoice struct {
	recipeOption
	drive float64
}

// bestCraft picks what she would make out of what she can make here: the
// trade that means most to her, then the most accomplished piece of work
// in it (the highest skill_minimum she has reached), then by name so the
// choice is steady. Never call it with an empty list.
func bestCraft(p *Profile, made []recipeOption) craftChoice {
	best := craftChoice{recipeOption: made[0], drive: tradeDrive(p, made[0].Skill)}
	for _, o := range made[1:] {
		d := tradeDrive(p, o.Skill)
		switch {
		case d > best.drive,
			d == best.drive && o.Min > best.Min,
			d == best.drive && o.Min == best.Min && o.Name < best.Name:
			best = craftChoice{recipeOption: o, drive: d}
		}
	}
	return best
}

// tradeAtHand sets a companion to work at her calling when she is idle at
// the right station with the makings in her pack: a cook at a fire with
// meat and salt cooks, a herbalist at a bench with roots brews. It runs on
// her idle tick, ahead of the slower pastime roll, at most once every
// IdleTradeSeconds and with her drive as the chance, so a born cook starts
// almost at once and someone with a passing interest leaves it to the
// pastime roll. Reports whether she started something.
func (m *AICompanionModule) tradeAtHand(c *controller, mob *mobs.Mob, now int64) bool {
	if now-c.lastCraft < int64(m.cfg.IdleTradeSeconds) {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	// Her skill may have grown since she was greeted: what she knows keeps
	// up with it, as a player's recipes do.
	seedRecipes(mob, c.profile)
	made := craftableHere(mob, c.profile, room)
	if len(made) == 0 {
		return false
	}
	pick := bestCraft(c.profile, made)
	if pick.drive < callingDrive {
		return false
	}
	c.lastCraft = now // whether or not the mood takes her this time
	if float64(util.Rand(1000))/1000.0 >= pick.drive {
		return false
	}
	m.startCraft(c, mob, pick.recipeOption, now)
	return true
}

// startCraft begins a piece of work with the ordinary craft command, so the
// station, the ingredients, the skill check and the time it takes are all
// the engine's.
func (m *AICompanionModule) startCraft(c *controller, mob *mobs.Mob, o recipeOption, now int64) {
	c.lastCraft = now
	c.lastPastime = now
	out := m.issue(c, mob, `craft`, craftCommand(o), `craft:`+o.Id, o.Name, ``, 0.5, util.GetRoundCount())
	if out.Pending != nil {
		c.pendingAct = out.Pending
	}
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
					mob.Character.StoreItem(back)
				}
			}
		}
	}
}
