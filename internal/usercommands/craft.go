package usercommands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// craftDeliver sends a self-only crafting response through SendTrio. Every
// line the craft command's own delivery switch sends is addressed to the
// crafter alone -- a recipe refusal, a station or skill gate, a "you begin"
// notice -- with no second party and no room broadcast, so Actee and Observer
// are always NoLine and Room is left unset. That absence is what makes the
// move text-identical: SendTrio only hides a name when it has a room to judge
// sight by (see messaging.hideForReader), so a self-only line with no Room
// passes through unchanged regardless of what it contains.
func craftDeliver(user *users.UserRecord, cat messaging.Category, text string) {
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(cat, text),
		Actee:    messaging.NoLine,
		Observer: messaging.NoLine,
	}, messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		ActeeName: messaging.NoName,
	})
}

// craftDeliverInstant sends an instant-complete craft's two lines -- the
// crafter's own success line and the room's observer line -- through
// messaging.SendTrio. Unlike craftDeliver, this path HAS a room line that
// names the crafter (recipe.Narrate's {actor} substitution), so it needs a
// real Audience with Room set: that is what lets SendTrio's HideNames catch
// the name for a shapes-only observer, rather than leaving it to the
// tag-based messaging.Anonymize stage alone. Shared by both instant-complete
// sites: case result.ImmediateComplete in Craft(), and completeCraft (the
// enchanting instant path).
func craftDeliverInstant(user *users.UserRecord, room *rooms.Room, roles narration.Roles) {
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="green">%s</ansi>`, roles.Actor)),
		Actee:    messaging.NoLine,
		Observer: messaging.Say(messaging.CategoryEmote, roles.Observer),
	}, messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		ActeeName: messaging.NoName,
		Room:      room,
	})
}

// Craft handles the `craft` and `craft list` commands (Stage 13.1).
func Craft(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	rest = strings.TrimSpace(rest)

	// ── craft (bare) → only recipes craftable right now, here ─────────────────
	if rest == "" {
		return craftCraftableNow(user, room), nil
	}
	// ── craft list / craft all → full sectioned view ──────────────────────────
	if lc := strings.ToLower(rest); lc == "list" || lc == "all" {
		return craftList(user, room), nil
	}

	// ── You cannot craft what you cannot see ──────────────────────────────────
	//
	// Owner ruling 2026-09-21. Listing what you know is memory and stays
	// allowed above; an ATTEMPT to make something is refused.
	//
	// Clear sight, not shapes (actions.TooDarkToCraft): it folds blindness, an
	// unlit room and NightVision into one verdict, and crafting is fine work,
	// so making out warm shapes by infrared is not enough to do it. Sibling
	// refusals live in get.go, loot.go and shoot.go.
	if actions.TooDarkToCraft(&actions.UserActor{User: user, Room: room}) {
		user.SendText(messaging.CategorySystem, `You can't see well enough to work on anything here.`)
		return true, nil
	}

	// ── Enchanting: needs player-specific disambiguation before delegating ─────
	// Peek at the recipe first to route enchanting separately.
	// Input may be "recipe-name item-name" so try progressively shorter
	// prefixes: "honed-edge knuckles" → try "honed-edge knuckles", then
	// "honed-edge". This handles hyphenated recipe names with an item target.
	recipe := crafting.FindRecipeByName(rest)
	if recipe == nil {
		words := strings.Fields(rest)
		for i := len(words) - 1; i >= 1; i-- {
			candidate := strings.Join(words[:i], " ")
			if r := crafting.FindRecipeByName(candidate); r != nil {
				recipe = r
				break
			}
		}
	}
	// ⚠️ MUST RUN BEFORE EVERY DISPATCH BELOW. See the function's own comment.
	ensureComponentsFromStorage(user, room, recipe)

	// Enchanting dispatches here, AFTER the pull above. It used to sit before it
	// and return, which is why enchanting never drew from storage.
	if recipe != nil && crafting.IsEnchantingRecipe(recipe) {
		return craftEnchanting(rest, recipe, user, room)
	}

	// ── Normal craft path: delegate to shared action ──────────────────────────
	actor := &actions.UserActor{User: user, Room: room}
	result := actions.InitiateCraft(actor, rest)

	switch {
	case result.CannotSee:
		// Unreachable today: the early refusal above already returns before
		// InitiateCraft runs. The case exists because CannotSee is part of the
		// result's contract for every caller, including mobs and the companion.
		craftDeliver(user, messaging.CategorySystem, `You can't see well enough to work on anything here.`)
		return true, nil

	case len(result.AmbiguousRecipes) > 0:
		list := make([]string, 0, len(result.AmbiguousRecipes))
		for _, n := range result.AmbiguousRecipes {
			list = append(list, fmt.Sprintf(`<ansi fg="cyan-bold">%s</ansi>`, n))
		}
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`You know more than one recipe like that: %s. Type more of the name to pick one.`,
			strings.Join(list, `, `)))
		return true, nil

	case result.RecipeNotFound:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">No recipe found for "%s". Type <ansi fg="cyan-bold">craft list</ansi> to see available recipes.</ansi>`,
			rest))
		return true, nil

	case result.RecipeNotKnown:
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">You don't know that recipe yet. Keep crafting to discover new ones!</ansi>`)
		return true, nil

	case result.AlreadyCrafting:
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi>`)
		return true, nil

	case result.SkillTooLow:
		craftDeliver(user, messaging.CategorySystem,
			craftSkillTooLowText(result.SkillName, result.SkillMinimum, result.SkillLevel))
		return true, nil

	case result.WrongStation:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">You need to be at a %s to craft that.</ansi>`,
			result.StationNeeded))
		return true, nil

	case result.MissingTool:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">You need a %s to craft that.</ansi> (<ansi fg="command">help tools</ansi>)`,
			result.ToolNeeded))
		return true, nil

	case result.MissingIngredients:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">You are missing: %s.</ansi>`,
			storageAwareMissingTag(user, result.Recipe, result.MissingTag)))
		return true, nil

	case result.ForeignComponent:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">The %s must be your own work — it bears another maker's mark.</ansi>`,
			result.ForeignComponentName))
		return true, nil

	case result.ImmediateComplete:
		// InitiateCraft leaves skill progression to the caller (see its doc
		// comment), and every other completion path awards it: the multi-round
		// player path (NewRound_UserRoundTick.go), the multi-round mob path
		// (NewRound_MobRoundTick.go), and the mob immediate path
		// (mobcommands/craft.go). This one was missed, so instant recipes gave
		// players no crafting progression while mobs got it.
		// U10b-1 Task 16. won is unconditionally TRUE here, and that is not a
		// shortcut: ImmediateComplete means the recipe had TimeRounds <= 0, and
		// InitiateCraft completes those without ever running the craft contest.
		// An instant recipe cannot fail, so there is no loss branch to pay a
		// fraction on. Only the two MULTI-ROUND sites roll
		// (NewRound_UserRoundTick, NewRound_MobRoundTick).
		//
		// U10b-3: recipe difficulty moved to discovery, so there is no bonus
		// left to carry and plain AwardResolved is correct.
		user.Character.AwardResolved(user.UserId, true,
			user.Character.CandidateFor(result.SkillName))
		roles := result.Recipe.Narrate(crafting.PhaseSuccess, textutil.TokenContext{
			ActorName:      user.Character.GetCharacterName(true),
			ActorPlainName: user.Character.GetCharacterName(false),
		})
		craftDeliverInstant(user, room, roles)
		return true, nil

	case result.Initiated:
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="yellow">You begin crafting %s... (%s)</ansi>`,
			result.RecipeName, craftTimeDesc(result.TimeRounds)))

		// Quest engine: command notification
		bridge := questengine.NewGameBridge(user, room.RoomId)
		questengine.GetEngine().Notify("command", questengine.EventDetails{
			UserId:  user.UserId,
			RoomId:  room.RoomId,
			Command: "craft",
		}, bridge, bridge)
		return true, nil
	}

	return true, nil
}

// ensureComponentsFromStorage draws any missing components for recipe out of
// the player's storage so a craft can proceed. No-op when the components are
// already carried, when the recipe is nil or unknown, or when the player could
// not craft here anyway. All-or-nothing: PlanStoragePull returns
// complete=false if storage cannot cover the whole shortfall, and then nothing
// moves.
//
// ⚠️ THIS MUST RUN BEFORE EVERY DISPATCH IN Craft(), AND ITS POSITION IS THE
// ONLY THING THAT MAKES IT CORRECT. It was previously an inline block sitting
// BELOW the enchanting route, which returned first — so enchanting never pulled
// at all and `craft honed-edge weapon` reported "You are missing:
// binding-paste" with 152 of them in storage. Both halves were individually
// right; only the order was wrong, which is exactly the kind of defect a unit
// test on either half cannot see.
//
// It is a named function rather than an inline block so that the next dispatch
// path added to Craft() has something obvious to sit below, and so the guard in
// craft_storage_order_test.go can assert no `return` sneaks in above it.
//
// ⚠️ THE SAME FAMILY OF DEFECT CAME BACK ON 2026-09-21, and the earlier fix
// above did not cover it. `craft setting` reported "You are missing:
// copper-wire." with 39 Copper Wire in the bank. This time the ORDER was
// right: the pull ran, found it could not cover chrysalis-shard as well, and
// correctly moved nothing. What was wrong was the REPORT. Every refusal below
// was written from crafting.HasIngredients, which sees only what is carried
// and names the first short tag in recipe order, and copper-wire is listed
// first. So the fix is not ordering but crafting.HasIngredientsWithStorage,
// reached through storageAwareMissingTag at all three player-facing sites.
//
// Read those two together: running before the dispatch is necessary and is
// not sufficient. A path that runs after the pull still has to ask a
// storage-aware question, because all-or-nothing means the pull legitimately
// leaves a shortfall in place.
//
// ⚠️ Running before the dispatch also means this function owns every gate
// that would refuse the craft anyway. IsCrafting is one: it is not returned
// until actions.InitiateCraft below, so without the guard in the clause
// below, a busy player had their bank emptied onto their person and was then
// told to finish what they were doing. Take such a gate INTO the clause; an
// early return in Craft() would skip the pull for every path beneath it,
// which is the original defect again.
//
// ⚠️ The station clause MUST track actions.StationSatisfied, which honours
// Chrysifier's Walking Chrysalis. When those two disagreed, a Chrysifier could
// craft anywhere but never receive components off-station, which read as the
// mutation doing nothing at all.
func ensureComponentsFromStorage(user *users.UserRecord, room *rooms.Room, recipe *crafting.RecipeSpec) {
	// char.IsCrafting() is the SAME predicate actions.InitiateCraft uses for
	// its AlreadyCrafting result, and the two must not drift: this function
	// runs above the dispatch, so a busy player used to have components
	// hauled out of the bank onto their person and then be refused anyway
	// with "You are already working on something."
	if recipe == nil ||
		user.Character.IsCrafting() ||
		!actions.StationSatisfied(user.Character, recipe.Station, room.Station) ||
		!actions.ToolSatisfied(user.Character, recipe) ||
		!user.Character.HasRecipe(recipe.RecipeId) {
		return
	}
	if ok, _ := crafting.HasIngredients(user.Character.Items, user.Character.ComponentItems, recipe); ok {
		return
	}
	pull, complete := crafting.PlanStoragePull(recipe, user.Character.Items, user.Character.ComponentItems, user.ItemStorage.GetItems())
	if !complete {
		return
	}
	for _, itm := range pull {
		// storageRemoveQuiet places the item on the character FIRST and only
		// removes it from storage if that succeeds — so an over-encumbered player
		// cannot destroy banked components.
		if storageRemoveQuiet(user, itm) {
			craftDeliver(user, messaging.CategoryLoot, fmt.Sprintf(`You draw <ansi fg="item">%s</ansi> from storage.`, itm.DisplayName()))
		} else {
			craftDeliver(user, messaging.CategorySystem, `You're too encumbered to draw any more from storage.`)
			return
		}
	}
}

// craftSkillCloseFraction is the share of a recipe's skill minimum at which
// the refusal reads "just beyond" rather than "well beyond". A ratio, not a
// point gap, because recipe minimums run from single digits to 65.
const craftSkillCloseFraction = 0.8

// craftSkillTooLowText is the one skill-too-low refusal for craft. It never
// prints the recipe minimum or the player's skill (owner ruling 2026-09-13).
func craftSkillTooLowText(skillName string, minimum, level int) string {
	if float64(level) >= float64(minimum)*craftSkillCloseFraction {
		return fmt.Sprintf(
			`<ansi fg="red">That recipe is just beyond your %s skill. A little more practice should do it.</ansi>`,
			skillName)
	}
	return fmt.Sprintf(
		`<ansi fg="red">That recipe is well beyond your %s skill for now.</ansi>`,
		skillName)
}

// craftEnchanting handles the enchanting sub-path of craft, which requires
// player-specific target disambiguation not available to mob actors.
func craftEnchanting(rest string, recipe *crafting.RecipeSpec, user *users.UserRecord, room *rooms.Room) (bool, error) {
	// Known-recipe gate
	if !user.Character.HasRecipe(recipe.RecipeId) {
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">You don't know that recipe yet. Keep crafting to discover new ones!</ansi>`)
		return true, nil
	}

	// Already crafting?
	if user.Character.IsCrafting() {
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi>`)
		return true, nil
	}

	// Skill gate
	skillLevel := user.Character.GetSkillLevel(skills.SkillTag(recipe.Skill))
	if skillLevel < recipe.SkillMinimum {
		craftDeliver(user, messaging.CategorySystem,
			craftSkillTooLowText(recipe.Skill, recipe.SkillMinimum, skillLevel))
		return true, nil
	}

	// Station check
	if !actions.StationSatisfied(user.Character, recipe.Station, room.Station) {
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">You need to be at a %s to craft that.</ansi>`,
			strings.ReplaceAll(recipe.Station, "_", " ")))
		return true, nil
	}

	// Ingredient check
	ok, missing := crafting.HasIngredients(user.Character.Items, user.Character.ComponentItems, recipe)
	if !ok {
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">You are missing: %s.</ansi>`,
			storageAwareMissingTag(user, recipe, missing)))
		return true, nil
	}

	// Self-crafted-component check (require_own_components)
	if ownOk, offendingName := crafting.CheckOwnComponents(recipe, user.Character.Items, user.Character.ComponentItems, user.Character.Name); !ownOk {
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">The %s must be your own work — it bears another maker's mark.</ansi>`,
			offendingName))
		return true, nil
	}

	// Slot-based target resolution
	// Strip the recipe name from the input to get the optional slot specifier.
	specifier := ""
	recipeName := strings.ToLower(recipe.Name)
	restLower := strings.ToLower(strings.TrimSpace(rest))
	if strings.HasPrefix(restLower, recipeName) {
		specifier = strings.TrimSpace(rest[len(recipeName):])
	} else if strings.HasPrefix(restLower, strings.ToLower(recipe.RecipeId)) {
		specifier = strings.TrimSpace(rest[len(recipe.RecipeId):])
	}

	slotLabel, targetItem, errMsg := resolveEnchantSlot(&user.Character.Equipment, recipe.TargetType, specifier)
	if errMsg != "" {
		craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s</ansi>`, errMsg))
		return true, nil
	}

	if targetItem == nil {
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">Could not find a valid item in that slot.</ansi>`)
		return true, nil
	}

	// U7b: refuse a breaching enchant BEFORE the multi-round activity starts.
	// Refusing here costs the player nothing, where refusing at completion can
	// only refund materials after the rounds are already spent.
	//
	// Subtracting what the target item already reserves is what makes
	// re-enchanting work: the old enchantment is replaced rather than stacked,
	// so only the difference is new.
	if def := enchantments.GetEnchantment(recipe.EnchantType); def != nil && def.ReservePool != "" {
		pool := characters.Pool(def.ReservePool)
		added := user.Character.EnchantReserveAt(recipe.EnchantType, 0, targetItem.GetSpec().Hands, pool) -
			user.Character.ItemReserveOnPool(*targetItem, pool)
		if user.Character.WouldBreachReservationCap(pool, added) {
			craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s</ansi>`,
				user.Character.ReservationRefusal(pool, added)))
			return true, nil
		}
	}

	// Safety: complete immediately if time_rounds <= 0
	if recipe.TimeRounds <= 0 {
		completeCraft(user, room, recipe)
		return true, nil
	}

	// Start multi-round enchanting with the resolved slot.
	craftData := activity.CraftingData{
		RecipeId:    recipe.RecipeId,
		RoundsTotal: recipe.TimeRounds,
		TargetSlot:  slotLabel,
		RoomId:      user.Character.RoomId,
	}
	if err := user.Character.Activity.TransitionToCrafting(
		craftData,
		state.TransitionReason{
			Trigger: activity.TriggerCraftBegin,
			Actor:   state.ActorRef{UserId: user.UserId},
		},
	); err != nil {
		craftDeliver(user, messaging.CategorySystem, `<ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi>`)
		return true, nil
	}
	craftDeliver(user, messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="yellow">You begin enchanting <ansi fg="itemname">%s</ansi>... (%s)</ansi>`,
		targetItem.DisplayName(), craftTimeDesc(recipe.TimeRounds)))

	// Quest engine: command notification
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "craft",
	}, bridge, bridge)

	return true, nil
}

// classifyRecipe buckets a known recipe for the current room: "ready" (makeable
// now, incl. from this room's storage), "missing" (skill+station OK, lack mats),
// or "locked" (wrong/absent station or skill too low).
func classifyRecipe(user *users.UserRecord, room *rooms.Room, r *crafting.RecipeSpec) string {
	lvl := user.Character.GetSkillLevel(skills.SkillTag(r.Skill))
	if lvl < r.SkillMinimum {
		return "locked"
	}
	if !actions.StationSatisfied(user.Character, r.Station, room.Station) {
		return "locked"
	}
	if !actions.ToolSatisfied(user.Character, r) {
		return "locked"
	}
	if ok, _ := crafting.HasIngredients(user.Character.Items, user.Character.ComponentItems, r); ok {
		return "ready"
	}
	// Completable by pulling from the player's storage (auto-pulled at craft time).
	if storageCompletable(user, r) {
		return "ready"
	}
	return "missing"
}

// craftRecipeRow renders a single recipe line (name + ingredients + station +
// time) using the shared craftList row style, WITHOUT the leading [X] indicator.
// Used by the bare-craft "Ready to Craft" view.
func craftRecipeRow(r *crafting.RecipeSpec) string {
	ingredientList := ingredientSummary(r)
	stationStr := ""
	if r.Station != "" {
		stationStr = fmt.Sprintf(" [%s]", strings.ReplaceAll(r.Station, "_", " "))
	}
	if r.Tool != "" {
		stationStr += fmt.Sprintf(" (%s)", actions.ToolName(r.Tool))
	}
	displayName := r.Name
	if crafting.IsEnchantingRecipe(r) && r.TargetType != "" {
		displayName = fmt.Sprintf("%s (%s)", r.Name, r.TargetType)
	}
	return fmt.Sprintf(
		`  <ansi fg="green">[V]</ansi> <ansi fg="white">%-26s</ansi> — %s  <ansi fg="dark-cyan">%s, %s</ansi>`,
		displayName, ingredientList, stationStr, craftTimeDesc(r.TimeRounds))
}

// craftCraftableNow prints only the recipes the player can craft right now in
// the current room (materials in hand or completable from this room's storage).
func craftCraftableNow(user *users.UserRecord, room *rooms.Room) bool {
	all := crafting.GetAll()

	// Collect known + ready recipes, grouped by skill for a stable ordering.
	bySkill := make(map[string][]*crafting.RecipeSpec)
	skillSet := make(map[string]struct{})
	ready := 0
	for id, r := range all {
		if !user.Character.HasRecipe(id) {
			continue
		}
		if classifyRecipe(user, room, r) != "ready" {
			continue
		}
		bySkill[r.Skill] = append(bySkill[r.Skill], r)
		skillSet[r.Skill] = struct{}{}
		ready++
	}

	user.SendText(messaging.CategorySystem, ``)
	user.SendText(messaging.CategorySystem, `<ansi fg="green-bold"> .:. Ready to Craft .:.</ansi>`)

	if ready == 0 {
		user.SendText(messaging.CategorySystem, ``)
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">Nothing you can craft here right now — try <ansi fg="cyan-bold">craft list</ansi> to see everything you know.</ansi>`)
		user.SendText(messaging.CategorySystem, ``)
		return true
	}

	skillNames := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skillNames = append(skillNames, sk)
	}
	sort.Strings(skillNames)

	for _, skillName := range skillNames {
		recipes := bySkill[skillName]
		sort.SliceStable(recipes, func(i, j int) bool { return recipes[i].Name < recipes[j].Name })
		user.SendText(messaging.CategorySystem, ``)
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="yellow">%s</ansi>`,
			titleCase(strings.ReplaceAll(skillName, "-", " "))))
		for _, r := range recipes {
			user.SendText(messaging.CategorySystem, craftRecipeRow(r))
		}
	}

	user.SendText(messaging.CategorySystem, ``)
	user.SendText(messaging.CategorySystem, `<ansi fg="cyan">Type <ansi fg="cyan-bold">craft list</ansi> to see every recipe you know.</ansi>`)
	user.SendText(messaging.CategorySystem, ``)
	return true
}

// craftList prints all known recipes, sectioned by craftability — Ready to
// craft, Missing ingredients, then Locked — with per-recipe status details.
func craftList(user *users.UserRecord, room *rooms.Room) bool {
	all := crafting.GetAll()
	if len(all) == 0 {
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">No crafting recipes are currently available.</ansi>`)
		return true
	}

	// Filter to only known recipes
	known := make(map[string]*crafting.RecipeSpec)
	for id, r := range all {
		if user.Character.HasRecipe(id) {
			known[id] = r
		}
	}

	if len(known) == 0 {
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">You haven't discovered any crafting recipes yet.</ansi>`)
		return true
	}

	// Bucket every known recipe by craftability, keyed by skill within the
	// bucket so we can preserve the existing per-skill grouping inside sections.
	type bucket struct {
		bySkill map[string][]*crafting.RecipeSpec
	}
	buckets := map[string]*bucket{
		"ready":   {bySkill: map[string][]*crafting.RecipeSpec{}},
		"missing": {bySkill: map[string][]*crafting.RecipeSpec{}},
		"locked":  {bySkill: map[string][]*crafting.RecipeSpec{}},
	}
	for _, r := range known {
		b := buckets[classifyRecipe(user, room, r)]
		b.bySkill[r.Skill] = append(b.bySkill[r.Skill], r)
	}

	// Overall completion accounting (known vs total across all skills).
	totalKnown := 0
	grandTotal := 0
	{
		countedSkills := make(map[string]struct{})
		for _, r := range known {
			countedSkills[r.Skill] = struct{}{}
		}
		for skillName := range countedSkills {
			allForSkill := crafting.GetAllForSkill(skillName)
			for _, r := range allForSkill {
				grandTotal++
				if user.Character.HasRecipe(r.RecipeId) {
					totalKnown++
				}
			}
		}
	}

	user.SendText(messaging.CategorySystem, ``)
	user.SendText(messaging.CategorySystem, `<ansi fg="cyan-bold"> .:. Crafting Recipes .:.</ansi>`)

	// Ordered sections, actionable ones first.
	sections := []struct {
		key    string
		header string
	}{
		{"ready", `<ansi fg="green-bold">Ready to craft</ansi>`},
		{"missing", `<ansi fg="yellow-bold">Missing ingredients</ansi>`},
		{"locked", `<ansi fg="red-bold">Locked (station or skill)</ansi>`},
	}

	for _, sec := range sections {
		b := buckets[sec.key]
		if len(b.bySkill) == 0 {
			continue
		}
		user.SendText(messaging.CategorySystem, ``)
		user.SendText(messaging.CategorySystem, fmt.Sprintf(` %s`, sec.header))

		// Sort skill names within the section for stable output.
		skillNames := make([]string, 0, len(b.bySkill))
		for sk := range b.bySkill {
			skillNames = append(skillNames, sk)
		}
		sort.Strings(skillNames)

		for _, skillName := range skillNames {
			skillLevel := user.Character.GetSkillLevel(skills.SkillTag(skillName))

			recipes := b.bySkill[skillName]
			sort.SliceStable(recipes, func(i, j int) bool { return recipes[i].Name < recipes[j].Name })

			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`  <ansi fg="yellow">%s</ansi> <ansi fg="white">(%s)</ansi>`,
				titleCase(strings.ReplaceAll(skillName, "-", " ")), skills.GetSkillRankDescription(skillLevel)))

			for _, r := range recipes {
				indicator, reason := recipeStatus(user, room, r, skillLevel)
				ingredientList := ingredientSummary(r)
				stationStr := ""
				if r.Station != "" {
					stationStr = fmt.Sprintf(" [%s]", strings.ReplaceAll(r.Station, "_", " "))
				}
				if r.Tool != "" {
					stationStr += fmt.Sprintf(" (%s)", actions.ToolName(r.Tool))
				}
				// Enchanting recipes target an equipped item; annotate the
				// recipe name with the slot for at-a-glance lookup.
				displayName := r.Name
				if crafting.IsEnchantingRecipe(r) && r.TargetType != "" {
					displayName = fmt.Sprintf("%s (%s)", r.Name, r.TargetType)
				}
				if reason != "" {
					user.SendText(messaging.CategorySystem, fmt.Sprintf(
						`    <ansi fg="red">[%s]</ansi> <ansi fg="white">%-26s</ansi> — %s  <ansi fg="red">%s</ansi><ansi fg="dark-cyan">%s, %s</ansi>`,
						indicator, displayName, ingredientList, reason, stationStr, craftTimeDesc(r.TimeRounds)))
				} else {
					user.SendText(messaging.CategorySystem, fmt.Sprintf(
						`    <ansi fg="green">[%s]</ansi> <ansi fg="white">%-26s</ansi> — %s  <ansi fg="dark-cyan">%s, %s</ansi>`,
						indicator, displayName, ingredientList, stationStr, craftTimeDesc(r.TimeRounds)))
				}
			}
		}
	}

	// Overall completion
	overallDesc := recipeCompletionTier(totalKnown, grandTotal)
	user.SendText(messaging.CategorySystem, ``)
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="cyan">Overall recipe knowledge: %s</ansi>`, overallDesc))
	user.SendText(messaging.CategorySystem, ``)
	return true
}

// recipeCompletionTier returns a descriptive tier for how many recipes
// are known out of a total. No hard numbers shown to the player.
func recipeCompletionTier(known, total int) string {
	if total <= 0 {
		return "unknown"
	}
	pct := float64(known) / float64(total) * 100
	switch {
	case pct < 15:
		return "a handful of recipes"
	case pct < 35:
		return "a modest collection"
	case pct < 60:
		return "a solid repertoire"
	case pct < 85:
		return "an extensive catalog"
	default:
		return "near-complete mastery"
	}
}

// recipeStatus returns the indicator character and a blocking reason string.
// indicator is "✓" if craftable, "✗" otherwise. reason is "" if craftable.
func recipeStatus(user *users.UserRecord, room *rooms.Room, r *crafting.RecipeSpec, skillLevel int) (string, string) {
	if skillLevel < r.SkillMinimum {
		return "X", fmt.Sprintf("%s skill required", skills.GetSkillRankDescription(r.SkillMinimum))
	}
	if !actions.StationSatisfied(user.Character, r.Station, room.Station) {
		return "X", fmt.Sprintf("need %s", strings.ReplaceAll(r.Station, "_", " "))
	}
	if !actions.ToolSatisfied(user.Character, r) {
		return "X", fmt.Sprintf("need %s", actions.ToolName(r.Tool))
	}
	// Storage counts toward BOTH halves of this answer. Components are
	// auto-pulled at craft time, so a recipe the bank can complete shows as
	// ready (matching the "Ready to craft" section), and one it cannot is
	// short of whatever the bank ALSO lacks, which is not necessarily the
	// first tag a carried-only count comes up short on.
	ok, missing := crafting.HasIngredientsWithStorage(
		user.Character.Items, user.Character.ComponentItems, user.ItemStorage.GetItems(), r)
	if !ok {
		return "X", fmt.Sprintf("missing %s", missing)
	}
	return "V", ""
}

// storageAwareMissingTag re-answers "what are you actually missing?" with the
// player's storage counted alongside what they carry, and is what every
// player-facing craft refusal must print.
//
// 🐛 Prod defect, owner 2026-09-21. `craft setting` said "You are missing:
// copper-wire." to a player with 39 Copper Wire in the bank. The storage pull
// is all-or-nothing by owner ruling, so a shortfall the bank cannot fully
// cover moves nothing at all, and the refusal was then written from a
// carried-only count that names the FIRST short tag in recipe order.
// chrysalis-setting lists copper-wire first; the real blocker was
// chrysalis-shard, listed second and absent everywhere.
//
// ⚠️ This lives in the command layer, not in actions.InitiateCraft, because
// storage hangs off the USER RECORD and InitiateCraft is shared with mobs,
// which have none. storageCompletable is the same split for the same reason.
//
// fallback is the carried-only tag the caller already has. It is returned
// when there is no recipe to recompute against, and when the storage-aware
// count says the recipe IS satisfiable -- which only happens if the pull was
// cut short by encumbrance, and in that case the carried-only answer is the
// truthful one.
func storageAwareMissingTag(user *users.UserRecord, r *crafting.RecipeSpec, fallback string) string {
	if r == nil {
		return fallback
	}
	ok, missing := crafting.HasIngredientsWithStorage(
		user.Character.Items, user.Character.ComponentItems, user.ItemStorage.GetItems(), r)
	if ok {
		return fallback
	}
	return missing
}

// storageCompletable reports whether recipe r could be crafted right now by
// auto-pulling its missing components from the player's storage.
//
// ⚠️ Enchanting recipes USED to be excluded here, on the grounds that they
// "route to craftEnchanting, which does not pull". They pull now — the route
// moved below the pull in Craft() — so excluding them would make the recipe
// list claim a craft is impossible that would in fact succeed.
func storageCompletable(user *users.UserRecord, r *crafting.RecipeSpec) bool {
	_, complete := crafting.PlanStoragePull(r, user.Character.Items, user.Character.ComponentItems, user.ItemStorage.GetItems())
	return complete
}

// ingredientSummary returns a short comma-separated ingredient list.
func ingredientSummary(r *crafting.RecipeSpec) string {
	parts := make([]string, 0, len(r.Ingredients))
	for _, ing := range r.Ingredients {
		parts = append(parts, fmt.Sprintf("%dx %s", ing.Quantity, ing.ItemTag))
	}
	return strings.Join(parts, ", ")
}

// completeCraft resolves a craft instantly (used when time_rounds <= 0).
func completeCraft(user *users.UserRecord, room *rooms.Room, recipe *crafting.RecipeSpec) {
	user.Character.Items, user.Character.ComponentItems = crafting.ConsumeIngredients(user.Character.Items, user.Character.ComponentItems, recipe)
	newItem := items.New(recipe.Output.ItemId)
	user.Character.StoreItem(newItem)
	roles := recipe.Narrate(crafting.PhaseSuccess, textutil.TokenContext{
		ActorName:      user.Character.GetCharacterName(true),
		ActorPlainName: user.Character.GetCharacterName(false),
	})
	craftDeliverInstant(user, room, roles)
}

// titleCase capitalises the first letter of each space-separated word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// craftTimeDesc returns a qualitative description for crafting duration.
func craftTimeDesc(rounds int) string {
	switch {
	case rounds <= 1:
		return "instant"
	case rounds <= 3:
		return "quick"
	case rounds <= 6:
		return "moderate"
	case rounds <= 10:
		return "lengthy"
	default:
		return "prolonged"
	}
}
