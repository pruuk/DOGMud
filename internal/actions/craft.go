package actions

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/timber"
)

// CraftResult describes the outcome of an InitiateCraft call.
// Callers are responsible for all player-facing messaging.
type CraftResult struct {
	// Initiated is true when multi-round crafting has been started
	// (Activity machine transitioned to Crafting).
	Initiated bool
	// ImmediateComplete is true when the recipe had TimeRounds <= 0 and was
	// completed in a single call.
	ImmediateComplete bool
	// RecipeNotFound is true when no recipe matched the given name.
	RecipeNotFound bool
	// RecipeNotKnown is true when the actor's character doesn't have the recipe
	// in their KnownRecipes map.
	RecipeNotKnown bool
	// SkillTooLow is true when the actor's skill rank is below the recipe
	// minimum.
	SkillTooLow bool
	// WrongStation is true when the recipe requires a station the current room
	// does not provide.
	WrongStation bool
	// MissingIngredients is true when the actor lacks one or more ingredients.
	MissingIngredients bool
	// MissingTool is true when the recipe needs a tool (RecipeSpec.Tool) the
	// actor is not carrying; ToolNeeded names it for the message.
	MissingTool bool
	ToolNeeded  string
	// ForeignComponent is true when the recipe requires self-crafted
	// components (RequireOwnComponents) and a matching ingredient in the
	// actor's pools was made by someone else (or has no maker at all).
	ForeignComponent bool
	// AlreadyCrafting is true when the character already has an active
	// CraftingState.
	AlreadyCrafting bool
	// AmbiguousRecipes holds the display names of multiple KNOWN recipes that
	// all matched the query (e.g. `craft cloak` when the player knows both
	// cloak recipes). Player-only: mob actors always take the tightest match.
	AmbiguousRecipes []string
	// CannotSee is true when the actor cannot see clearly enough to work
	// (TooDarkToCraft). Checked first.
	CannotSee bool

	// Descriptive data filled in on all non-error paths (for messaging).
	RecipeName           string
	SkillName            string
	SkillLevel           int
	SkillMinimum         int
	TimeRounds           int // recipe.TimeRounds — for duration-description messaging
	StationNeeded        string
	MissingTag           string
	ForeignComponentName string               // name of the offending component (ForeignComponent only)
	OutputName           string               // display name of the produced item (immediate-complete only)
	Recipe               *crafting.RecipeSpec // resolved recipe; render its text through Recipe.Narrate
}

// resolveCraftRecipe resolves a craft query with known-recipe preference:
//
//   - exactly one KNOWN match → that recipe
//   - multiple KNOWN matches → nil + their names, tightest first (caller
//     prompts the player to be more specific)
//   - no known match but candidates exist → the tightest candidate (the
//     known-recipe gate downstream yields the discovery message)
//   - nothing matched → nil, nil
func resolveCraftRecipe(char *characters.Character, name string) (*crafting.RecipeSpec, []string) {
	candidates := crafting.FindRecipesByName(name)
	if len(candidates) == 0 {
		return nil, nil
	}
	var known []*crafting.RecipeSpec
	for _, r := range candidates {
		if char.HasRecipe(r.RecipeId) {
			known = append(known, r)
		}
	}
	switch {
	case len(known) == 1:
		return known[0], nil
	case len(known) > 1:
		names := make([]string, 0, len(known))
		for _, r := range known {
			names = append(names, r.Name)
		}
		return nil, names
	default:
		return candidates[0], nil
	}
}

// StationSatisfied reports whether char may craft a recipe requiring
// recipeStation while standing in a room whose station is roomStation.
//
// ⚠️ THIS IS THE ONLY PLACE THE RULE LIVES, and it exists because it was
// previously copied into FIVE separate checks of which exactly ONE honoured
// Chrysifier's Walking Chrysalis (the `portable-workshop` flag). The mutation
// promises "no forge, no loom, no bench of any kind — make anything,
// anywhere", and a player holding it reported it doing nothing at all. It was
// half-working in the least visible way possible: the craft itself was allowed,
// while the recipe list said `locked`, the status column said `need forge`,
// enchanting refused outright, and storage would not release components
// off-station. Four of five signals said no, so the one that said yes was
// invisible.
//
// Take a station rule to this function. Do not re-inline it.
func StationSatisfied(char *characters.Character, recipeStation, roomStation string) bool {
	if recipeStation == "" || roomStation == recipeStation {
		return true
	}
	if char == nil {
		return false
	}
	// Walking Chrysalis makes the body itself the workshop.
	return mutations.HasPortableWorkshop(char.Mutations)
}

// TooDarkToCraft is the one statement of the craft sight rule for both
// actors (slice 5a): crafting is fine work, so shapes by infrared are not
// enough; it needs clear sight (awake, SightFull). The player's command also
// asks it before its storage pull and enchanting branch.
func TooDarkToCraft(actor Actor) bool {
	return !messaging.CanSeeClearly(actor.GetCharacter(), actor.GetRoom())
}

// InitiateCraft attempts to begin (or immediately complete) a crafting
// operation for actor using the recipe identified by recipeName.
//
// Enchanting recipes are intentionally NOT handled here — that path requires
// player-specific target disambiguation and stays in the user command wrapper.
//
// Callers are responsible for:
//   - Skill progression (OnSkillUse)
//   - Quest engine notifications
//   - All player-facing text
func InitiateCraft(actor Actor, recipeName string) CraftResult {
	char := actor.GetCharacter()
	room := actor.GetRoom()

	// ── Can the actor see to work? ────────────────────────────────────────────
	if TooDarkToCraft(actor) {
		return CraftResult{CannotSee: true}
	}

	// ── Already crafting? ─────────────────────────────────────────────────────
	if char.IsCrafting() {
		return CraftResult{AlreadyCrafting: true}
	}

	// ── Recipe lookup ─────────────────────────────────────────────────────────
	// Known-recipe preference: `craft cloak` resolves against the recipes the
	// actor KNOWS before falling back to the tightest overall match (whose
	// known-gate below then produces the keep-crafting-to-discover message).
	recipe, ambiguous := resolveCraftRecipe(char, recipeName)
	if recipe == nil && len(ambiguous) == 0 {
		return CraftResult{RecipeNotFound: true}
	}
	if len(ambiguous) > 0 {
		if actor.IsPlayer() {
			return CraftResult{AmbiguousRecipes: ambiguous}
		}
		// Mobs can't answer a prompt — take the tightest known match.
		recipe = crafting.FindRecipeByName(ambiguous[0])
	}

	res := CraftResult{
		RecipeName:   recipe.Name,
		SkillName:    recipe.Skill,
		SkillMinimum: recipe.SkillMinimum,
		TimeRounds:   recipe.TimeRounds,
		Recipe:       recipe,
	}

	// ── Known-recipe gate ─────────────────────────────────────────────────────
	if !char.HasRecipe(recipe.RecipeId) {
		res.RecipeNotKnown = true
		return res
	}

	// ── Skill level gate ──────────────────────────────────────────────────────
	skillLevel := char.GetSkillLevel(skills.SkillTag(recipe.Skill))
	res.SkillLevel = skillLevel
	if skillLevel < recipe.SkillMinimum {
		res.SkillTooLow = true
		return res
	}

	// ── Station check ─────────────────────────────────────────────────────────
	if !StationSatisfied(char, recipe.Station, room.Station) {
		res.StationNeeded = strings.ReplaceAll(recipe.Station, "_", " ")
		res.WrongStation = true
		return res
	}

	// ── Tool check (wilderness trades) ────────────────────────────────────────
	if !ToolSatisfied(char, recipe) {
		res.ToolNeeded = ToolName(recipe.Tool)
		res.MissingTool = true
		return res
	}

	// ── Ingredient check ──────────────────────────────────────────────────────
	ok, missingTag := crafting.HasIngredients(char.Items, char.ComponentItems, recipe)
	if !ok {
		res.MissingTag = missingTag
		res.MissingIngredients = true
		return res
	}

	// ── Self-crafted-component gate (require_own_components) ─────────────────
	if ownOk, offendingName := crafting.CheckOwnComponents(recipe, char.Items, char.ComponentItems, char.Name); !ownOk {
		res.ForeignComponentName = offendingName
		res.ForeignComponent = true
		return res
	}

	// ── Enchanting recipes: caller handles these (user-only complexity) ───────
	// We only proceed for normal crafting recipes here.
	if crafting.IsEnchantingRecipe(recipe) {
		// Return as if recipe not found so the user wrapper can take over.
		// Mob callers simply won't request enchanting recipes.
		return CraftResult{RecipeNotFound: true}
	}

	// ── Immediate completion (TimeRounds <= 0) ────────────────────────────────
	if recipe.TimeRounds <= 0 {
		// An instant recipe runs no contest, so its grade comes from its
		// inputs and tool alone (gather.CraftGrade with no result). Read the
		// inputs BEFORE they are consumed.
		selected := crafting.SelectIngredients(char.Items, char.ComponentItems, recipe)
		grade := RecipeGrade(char, recipe, nil, selected)
		wood := CraftWood(selected)
		// Provident Hands may preserve the materials entirely (efficient craft).
		if !char.CraftMaterialsSaved() {
			char.Items, char.ComponentItems = crafting.ConsumeIngredients(
				char.Items, char.ComponentItems, recipe)
		}
		for n := 0; n < recipe.OutputCount(); n++ {
			newItem := items.New(recipe.Output.ItemId)
			newItem.Quality = grade
			StampWood(&newItem, wood)
			newItem.CraftSkill = char.CraftQualityLevel(skillLevel) // Faithwrought quality lift
			// Maker's mark — same policy as the async completion path
			// (crafting.ShouldStampMakerName): components stamp regardless of
			// Type so require_own_components provenance works for
			// TimeRounds<=0 sub-recipes too.
			if crafting.ShouldStampMakerName(newItem.CraftSkill, newItem.GetSpec()) {
				newItem.MakerName = char.Name
			}
			char.StoreItem(newItem)
			res.OutputName = newItem.DisplayName()
		}
		WearRecipeTool(actor, recipe)
		res.ImmediateComplete = true
		return res
	}

	// ── Start multi-round crafting ────────────────────────────────────────────
	craftData := activity.CraftingData{
		RecipeId:    recipe.RecipeId,
		RoundsTotal: recipe.TimeRounds,
		RoomId:      char.RoomId,
	}
	actorRef := state.ActorRef{
		UserId:        actor.GetUserId(),
		MobInstanceId: actor.GetMobInstanceId(),
	}
	if err := char.Activity.TransitionToCrafting(
		craftData,
		state.TransitionReason{
			Trigger: activity.TriggerCraftBegin,
			Actor:   actorRef,
		},
	); err != nil {
		res.AlreadyCrafting = true
		return res
	}

	res.Initiated = true
	return res
}

// ToolSatisfied reports whether char carries the tool a recipe needs (or the
// recipe needs none). The one statement of the rule: InitiateCraft, the craft
// list status and the ready/locked buckets all ask it.
func ToolSatisfied(char *characters.Character, recipe *crafting.RecipeSpec) bool {
	if recipe == nil || recipe.Tool == `` {
		return true
	}
	_, ok := gather.BestTool(char, recipe.Tool)
	return ok
}

// RecipeToolTier is the tier of the best tool char has for the recipe, and
// whether the recipe needs one.
func RecipeToolTier(char *characters.Character, recipe *crafting.RecipeSpec) (items.ToolTier, bool) {
	if recipe == nil || recipe.Tool == `` {
		return items.ToolTierNone, false
	}
	t, ok := gather.BestTool(char, recipe.Tool)
	if !ok {
		return items.ToolTierNone, true
	}
	return t.Tier, true
}

// RecipeGrade is the grade a craft gives its output (gather.CraftGradeOutput):
// graded inputs, a recipe tool, or an output that is a tool or a piece of
// gear (a weapon, armour, a shield, jewelry: items.IsGearType) each make it
// graded, so every crafted weapon and armour piece is graded by the
// crafter's margin and works accordingly (items/grade_effects.go). cr is nil
// for an instant recipe. consumed is what the craft will spend.
func RecipeGrade(char *characters.Character, recipe *crafting.RecipeSpec, cr *contest.Result, consumed []items.Item) items.Quality {
	toolTier, hasTool := RecipeToolTier(char, recipe)
	alwaysGraded := false
	if recipe != nil {
		if spec := items.GetItemSpec(recipe.Output.ItemId); spec != nil && (spec.Tool != nil || items.IsGearType(spec.Type)) {
			alwaysGraded = true
		}
	}
	grade := gather.CraftGradeOutput(cr, consumed, hasTool, toolTier, alwaysGraded)
	// An ungraded copy of a material the world grades (shop stock, which
	// loses its grade) counts as standard for the input cap (review fix).
	return capUngradedGradable(grade, consumed)
}

// CraftWood is the wood a crafted output inherits from what it was made of:
// the first consumed item that carries a wood (a stave, a bundle of shafts),
// else the first log of a known timber species. "" when nothing was wooden.
func CraftWood(consumed []items.Item) string {
	for _, itm := range consumed {
		if itm.Wood != `` {
			return itm.Wood
		}
	}
	for _, itm := range consumed {
		if sp := timber.SpeciesForLog(itm.ItemId); sp != nil {
			return sp.Id
		}
	}
	return ``
}

// StampWood gives a newly crafted item its wood when its spec carries one.
func StampWood(itm *items.Item, wood string) {
	if wood == `` || itm == nil {
		return
	}
	if spec := items.GetItemSpec(itm.ItemId); spec != nil && spec.CarriesWood {
		itm.Wood = wood
	}
}

// WearRecipeTool wears the tool a finished craft used, if the recipe has one.
func WearRecipeTool(actor Actor, recipe *crafting.RecipeSpec) {
	if recipe == nil || recipe.Tool == `` || actor == nil {
		return
	}
	t, ok := gather.BestTool(actor.GetCharacter(), recipe.Tool)
	WearUsedTool(actor, t, ok)
}

// ToolName is a tool type as a player reads it ("bone saw").
func ToolName(t items.ToolType) string {
	return strings.ReplaceAll(string(t), `_`, ` `)
}

// AbandonCraft spends a craft's materials without making anything: the
// crafter walked away from the work, or was carried off, before it was done
// (wilderness trades review). The half-worked materials are ruined.
func AbandonCraft(char *characters.Character, recipeId string) {
	if char == nil {
		return
	}
	recipe := crafting.GetRecipe(recipeId)
	if recipe == nil {
		return
	}
	char.Items, char.ComponentItems = crafting.ConsumeIngredients(char.Items, char.ComponentItems, recipe)
}

// JobLeftBehind reports whether an activity begun in startRoom must be given
// up because the actor is now in a different room. startRoom 0 means the job
// did not record one (any room will do).
func JobLeftBehind(startRoom, currentRoom int) bool {
	return startRoom != 0 && startRoom != currentRoom
}
