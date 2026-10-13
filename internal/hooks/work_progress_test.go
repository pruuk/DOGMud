package hooks

import (
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkProgressPhrase_Bands(t *testing.T) {
	const (
		start  = "you've only just started"
		half   = "you're about halfway"
		nearly = "it's nearly done"
	)
	cases := map[int][]string{
		// index = rounds complete (0 unused); only incomplete rounds tick.
		2: {"", half},
		3: {"", start, nearly},
		4: {"", start, half, nearly},
		5: {"", start, half, half, nearly},
	}
	for total, want := range cases {
		for done := 1; done < total; done++ {
			assert.Equal(t, want[done], workProgressPhrase(done, total),
				"%d of %d rounds", done, total)
		}
	}
	assert.Equal(t, nearly, workProgressPhrase(3, 3), "a finished count reads nearly done")
	assert.Equal(t, nearly, workProgressPhrase(1, 0), "a bad total does not panic")
}

func TestCraftProgressLine_NamesTheRecipe(t *testing.T) {
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "healing-salve", Name: "Healing Salve"})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("healing-salve") })

	line := craftProgressLine(characters.New(), activity.CraftingData{RecipeId: "healing-salve", RoundsComplete: 1, RoundsTotal: 2})
	assert.Equal(t, `<ansi fg="yellow">You keep working on the Healing Salve; you're about halfway.</ansi>`, line)

	// A recipe that has gone missing falls back to its id rather than blanking.
	line = craftProgressLine(characters.New(), activity.CraftingData{RecipeId: "gone-recipe", RoundsComplete: 2, RoundsTotal: 3})
	assert.Contains(t, line, "the gone-recipe; it's nearly done.")
}

// Review finding D: a recipe whose name already carries an article reads
// without a second one ("the The Blackrazor" was the shipped bug).
func TestCraftProgressLine_ANameWithItsOwnArticleGetsNoSecond(t *testing.T) {
	for id, name := range map[string]string{
		"assemble-the-blackrazor": "The Blackrazor", // the shipped recipe's name
		"article-test-a":          "A Lantern of Glass",
		"article-test-an":         "an Ember Lamp",
	} {
		crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: id, Name: name})
		t.Cleanup(func() { crafting.UnregisterRecipeForTest(id) })

		line := craftProgressLine(characters.New(), activity.CraftingData{RecipeId: id, RoundsComplete: 1, RoundsTotal: 3})
		assert.Equal(t, `<ansi fg="yellow">You keep working on `+name+`; you've only just started.</ansi>`, line)
	}
	// A word that merely starts with the letters keeps its article.
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "article-test-theory", Name: "Theory Stone"})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("article-test-theory") })
	assert.Contains(t, craftProgressLine(characters.New(), activity.CraftingData{RecipeId: "article-test-theory", RoundsComplete: 1, RoundsTotal: 3}),
		"You keep working on the Theory Stone;")
}

// Review finding E: an enchanting's progress line names the item being
// enchanted, as its start line ("You begin enchanting <item>") does, not the
// recipe. An item no longer in the slot drops the name.
func TestCraftProgressLine_EnchantingNamesTheItem(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "honed-edge-test", Name: "Honed-Edge",
		EnchantType: "honed-edge", TargetType: "weapon"})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("honed-edge-test") })

	const swordId = 97513
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		swordId: {ItemId: swordId, Name: "notched longsword", Type: items.Weapon},
	}))
	ch := characters.New()
	ch.Equipment.Weapon = items.New(swordId)
	require.NotZero(t, ch.Equipment.Weapon.ItemId, "fixture: the sword is seeded")
	cd := activity.CraftingData{RecipeId: "honed-edge-test", TargetSlot: "wielded", RoundsComplete: 1, RoundsTotal: 2}

	line := craftProgressLine(ch, cd)
	assert.Equal(t, `<ansi fg="yellow">You keep enchanting <ansi fg="itemname">`+ch.Equipment.Weapon.DisplayName()+
		`</ansi>; you're about halfway.</ansi>`, line)
	assert.NotContains(t, line, "Honed-Edge")

	ch.Equipment.Weapon = items.Item{}
	assert.Equal(t, `<ansi fg="yellow">You keep enchanting; you're about halfway.</ansi>`, craftProgressLine(ch, cd))
}

var anyDigit = regexp.MustCompile(`[0-9]`)

// The real round tick, not just the formatter: a crafting user's per-round
// line carries the recipe's name and no numerals.
func TestUserRoundTick_CraftProgressNamesTheRecipeAndHasNoDigits(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "healing-salve", Name: "Healing Salve"})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("healing-salve") })

	// The fixture's character is a bare struct literal with no Activity machine;
	// characters.New() wires one, as in production.
	crafter := users.GetByUserId(1)
	orig := crafter.Character
	t.Cleanup(func() { crafter.Character = orig })
	crafter.Character = characters.New()
	crafter.Character.Name = orig.Name
	crafter.Character.RoomId = orig.RoomId
	crafter.Character.SetUserId(1)
	require.NoError(t, crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "healing-salve", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	))
	drainPlain(1)

	UserRoundTick(events.NewRound{RoundNumber: 1})

	var found string
	for _, l := range drainPlain(1) {
		if countContaining([]string{l}, "keep working on") == 1 {
			found = l
		}
	}
	require.NotEmpty(t, found, "the tick sent no craft progress line")
	assert.Contains(t, found, "Healing Salve")
	assert.False(t, anyDigit.MatchString(found), "progress line has a numeral: %q", found)
	assert.LessOrEqual(t, len(found), 80)
}

func TestSalvageProgressLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	itm := items.New(1)
	user.Character.Items = append(user.Character.Items, itm)
	t.Cleanup(func() { user.Character.Items = user.Character.Items[:len(user.Character.Items)-1] })

	line := salvageProgressLine(user.Character, activity.SalvagingData{
		ItemUuid: itm.UUID.String(), RoundsComplete: 1, RoundsTotal: 2})
	assert.Contains(t, line, "You keep salvaging the ")
	assert.Contains(t, line, itm.DisplayName())
	assert.Contains(t, line, "; you're about halfway.")
	assert.False(t, anyDigit.MatchString(plainText(line)), "%q", line)

	// An item whose name carries its own article takes no second one
	// ("the A Beeswax Votive Candle" was the shipped bug; 26 shipped items
	// start with A, An or The).
	const candleId = 97514
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		candleId: {ItemId: candleId, Name: "A Beeswax Votive Candle"},
	}))
	candle := items.New(candleId)
	require.Equal(t, "A Beeswax Votive Candle", candle.DisplayName(), "fixture")
	user.Character.Items = append(user.Character.Items, candle)
	t.Cleanup(func() { user.Character.Items = user.Character.Items[:len(user.Character.Items)-1] })
	assert.Equal(t, `<ansi fg="yellow">You keep salvaging <ansi fg="itemname">A Beeswax Votive Candle</ansi>; you're about halfway.</ansi>`,
		salvageProgressLine(user.Character, activity.SalvagingData{
			ItemUuid: candle.UUID.String(), RoundsComplete: 1, RoundsTotal: 2}))

	// Corpse salvage and an item no longer carried both still read cleanly.
	assert.Equal(t, `<ansi fg="yellow">You keep working over the corpse; it's nearly done.</ansi>`,
		salvageProgressLine(user.Character, activity.SalvagingData{
			ItemUuid: "corpse:12", RoundsComplete: 2, RoundsTotal: 3}))
	assert.Equal(t, `<ansi fg="yellow">You keep salvaging; you've only just started.</ansi>`,
		salvageProgressLine(user.Character, activity.SalvagingData{
			ItemUuid: "no-such-uuid", RoundsComplete: 1, RoundsTotal: 3}))
}
