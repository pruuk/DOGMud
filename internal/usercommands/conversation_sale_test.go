package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A sale topic teaches two seeded recipes for a price (#375 phase 7 tier 3b).
const saleTalkYAML = `mobid: 1
zone: TestZone
topics:
  - key: teach_x
    kind: sale
    label: "Secret of X"
    playerLine: "Will you teach me X?"
    text: "X is not for everyone."
    notReadyText: "Come back when you have practiced at the forge."
    paidText: "Paid. It is yours."
    chargesGold: 1500
    teachesRecipes: [zz-sale-a, zz-sale-b]
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

func seedSaleRecipes(t *testing.T) {
	t.Helper()
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "zz-sale-a", Name: "Sale Alpha", Skill: "zzcraft", SkillMinimum: 10})
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "zz-sale-b", Name: "Sale Beta", Skill: "zzcraft2", SkillMinimum: 20})
	t.Cleanup(func() {
		crafting.UnregisterRecipeForTest("zz-sale-a")
		crafting.UnregisterRecipeForTest("zz-sale-b")
	})
}

func TestSale_WithoutTheSkillTheNpcNamesTheTradesAndNoCardOpens(t *testing.T) {
	seedSaleRecipes(t)
	for name, skills := range map[string]map[string]int{
		"none":       nil,
		"one of two": {"zzcraft": 10},
		"too low":    {"zzcraft": 10, "zzcraft2": 19},
	} {
		t.Run(name, func(t *testing.T) {
			user, mob, done := fixtureWith(t, saleTalkYAML)
			defer done()
			user.Character.Gold = 5000
			user.Character.Skills = skills
			conversation.Talk(user, mustRoom(t, user), mob)
			drained(1)
			drained(2)

			require.True(t, user.RouteMenuInput("1", true), "the sale is topic 1")
			mine, theirs := drained(1), drained(2)
			require.Contains(t, theirs, "Will you teach me X?")
			require.Contains(t, theirs, "Come back when you have practiced at the forge.")
			require.NotContains(t, theirs, "X is not for everyone.", "no pitch without the skill")
			require.NotContains(t, mine, "Price:")
			require.NotContains(t, mine, "Accept", "no card")
			require.Contains(t, mine, "Secret of X", "the line stays on the reopened menu")
			require.True(t, user.HasOpenMenu())
			require.Equal(t, 5000, user.Character.Gold, "nothing charged")
			require.False(t, user.Character.HasRecipe("zz-sale-a"))
			require.False(t, user.Character.HasRecipe("zz-sale-b"))
		})
	}
}

func TestSale_WithTheSkillPitchesShowsThePriceAndAcceptTeachesBoth(t *testing.T) {
	seedSaleRecipes(t)
	user, mob, done := fixtureWith(t, saleTalkYAML)
	defer done()
	user.Character.Gold = 5000
	user.Character.Skills = map[string]int{"zzcraft": 10, "zzcraft2": 20}
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	require.True(t, user.RouteMenuInput("1", true))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "X is not for everyone.")
	require.Contains(t, mine, "Price: 1,500 gold.")
	require.Contains(t, mine, "Accept")
	require.Equal(t, 5000, user.Character.Gold, "nothing is taken before Accept")

	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	mine, theirs = drained(1), drained(2)
	require.Equal(t, 3500, user.Character.Gold)
	require.True(t, user.Character.HasRecipe("zz-sale-a"))
	require.True(t, user.Character.HasRecipe("zz-sale-b"))
	require.Contains(t, theirs, "Paid. It is yours.")
	require.Contains(t, mine, "Sale Alpha")
	require.Contains(t, mine, "Sale Beta")
	require.True(t, user.HasOpenMenu())
	require.NotContains(t, mine, "Secret of X", "every recipe known: the line is gone")
	require.Contains(t, mine, "Why do your bones rattle?")
}

func TestSale_LeavingTheCardChargesNothing(t *testing.T) {
	seedSaleRecipes(t)
	user, mob, done := fixtureWith(t, saleTalkYAML)
	defer done()
	user.Character.Gold = 5000
	user.Character.Skills = map[string]int{"zzcraft": 10, "zzcraft2": 20}
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	drained(1)
	require.True(t, user.RouteMenuInput("2", true), "2 is Leave")
	mine := drained(1)
	require.Equal(t, 5000, user.Character.Gold)
	require.False(t, user.Character.HasRecipe("zz-sale-a"))
	require.Contains(t, mine, "Secret of X")
}

func TestSale_TooLittleGoldTeachesNothingAndTheLineStays(t *testing.T) {
	seedSaleRecipes(t)
	user, mob, done := fixtureWith(t, saleTalkYAML)
	defer done()
	user.Character.Gold = 1499
	user.Character.Skills = map[string]int{"zzcraft": 10, "zzcraft2": 20}
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	drained(1)
	drained(2)
	require.True(t, user.RouteMenuInput("1", true), "Accept")
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "You don't have enough gold for that.")
	require.NotContains(t, theirs, "Paid. It is yours.")
	require.Equal(t, 1499, user.Character.Gold)
	require.False(t, user.Character.HasRecipe("zz-sale-a"))
	require.Contains(t, mine, "Secret of X")
}

// openSaleCard talks to the fixture NPC, chooses the sale and leaves its
// Accept / Leave card open, for the tests that change the world before Accept.
func openSaleCard(t *testing.T) (user *users.UserRecord, mob *mobs.Mob, done func()) {
	t.Helper()
	seedSaleRecipes(t)
	user, mob, done = fixtureWith(t, saleTalkYAML)
	user.Character.Gold = 5000
	user.Character.Skills = map[string]int{"zzcraft": 10, "zzcraft2": 20}
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true), "the sale is topic 1")
	require.Contains(t, drained(1), "Price: 1,500 gold.", "fixture: the card is open")
	drained(2)
	return user, mob, done
}

func requireSaleRefused(t *testing.T, user *users.UserRecord, mine string) {
	t.Helper()
	require.Contains(t, mine, "That offer is no longer open.")
	require.Equal(t, 5000, user.Character.Gold, "nothing charged")
	require.False(t, user.Character.HasRecipe("zz-sale-a"))
	require.False(t, user.Character.HasRecipe("zz-sale-b"))
}

// acceptSale re-checks the sale: the skill may be lost between the card and
// Accept.
func TestSale_SkillLostBetweenTheCardAndAcceptTeachesNothing(t *testing.T) {
	user, _, done := openSaleCard(t)
	defer done()
	user.Character.Skills = map[string]int{"zzcraft": 10, "zzcraft2": 19}

	require.True(t, user.RouteMenuInput("1", true), "Accept")
	mine, theirs := drained(1), drained(2)
	requireSaleRefused(t, user, mine)
	require.NotContains(t, theirs, "Paid. It is yours.")
}

// Knowing every recipe by Accept (learned another way meanwhile) hides the sale.
func TestSale_AlreadyKnowingEveryRecipeByAcceptChargesNothing(t *testing.T) {
	user, _, done := openSaleCard(t)
	defer done()
	user.Character.LearnRecipe("zz-sale-a")
	user.Character.LearnRecipe("zz-sale-b")

	require.True(t, user.RouteMenuInput("1", true), "Accept")
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "That offer is no longer open.")
	require.Equal(t, 5000, user.Character.Gold, "nothing charged")
	require.NotContains(t, theirs, "Paid. It is yours.")
}

// Accept after the NPC walked off: nothing happens and nothing is taken.
func TestSale_AcceptAfterTheNpcLeftTheRoomTakesNothing(t *testing.T) {
	user, mob, done := openSaleCard(t)
	defer done()
	mob.Character.RoomId = 2

	require.True(t, user.RouteMenuInput("1", true), "Accept")
	drained(1)
	drained(2)
	require.Equal(t, 5000, user.Character.Gold, "nothing charged")
	require.False(t, user.Character.HasRecipe("zz-sale-a"))
	require.False(t, user.Character.HasRecipe("zz-sale-b"))
}
