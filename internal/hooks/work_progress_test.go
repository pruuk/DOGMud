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

	line := craftProgressLine(activity.CraftingData{RecipeId: "healing-salve", RoundsComplete: 1, RoundsTotal: 2})
	assert.Equal(t, `<ansi fg="yellow">You keep working on the Healing Salve; you're about halfway.</ansi>`, line)

	// A recipe that has gone missing falls back to its id rather than blanking.
	line = craftProgressLine(activity.CraftingData{RecipeId: "gone-recipe", RoundsComplete: 2, RoundsTotal: 3})
	assert.Contains(t, line, "the gone-recipe; it's nearly done.")
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

	// Corpse salvage and an item no longer carried both still read cleanly.
	assert.Equal(t, `<ansi fg="yellow">You keep working over the corpse; it's nearly done.</ansi>`,
		salvageProgressLine(user.Character, activity.SalvagingData{
			ItemUuid: "corpse:12", RoundsComplete: 2, RoundsTotal: 3}))
	assert.Equal(t, `<ansi fg="yellow">You keep salvaging; you've only just started.</ansi>`,
		salvageProgressLine(user.Character, activity.SalvagingData{
			ItemUuid: "no-such-uuid", RoundsComplete: 1, RoundsTotal: 3}))
}
