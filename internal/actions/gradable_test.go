package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Review fix: an ungraded copy of a material the world grades (a shop's
// resold hide, shop leather made from hides) caps a craft at one above
// standard; a material that is never graded (thread) does not cap it.
func TestCapUngradedGradable(t *testing.T) {
	cleanItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		900: {ItemId: 900, Name: "Test Hide", Type: items.Object, IsComponent: true, ComponentTag: "test-hide"},
		901: {ItemId: 901, Name: "Test Leather", Type: items.Object, IsComponent: true, ComponentTag: "test-leather"},
		902: {ItemId: 902, Name: "Test Thread", Type: items.Object, IsComponent: true, ComponentTag: "test-thread"},
	})
	defer cleanItems()
	cleanSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		900: {SpeciesId: 900, Name: "testbeast", Harvest: &species.HarvestTable{
			Skin: []species.HarvestEntry{{Item: "test-hide", Qty: 1}},
		}},
	})
	defer cleanSpecies()
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId:    "test-tan",
		Name:        "Test Tan",
		Ingredients: []crafting.RecipeIngredient{{ItemTag: "test-hide", Quantity: 1}},
		Output:      crafting.RecipeOutput{ItemId: 901, Quantity: 1},
	})
	defer crafting.UnregisterRecipeForTest("test-tan")

	fine := items.QualityStandard + 1
	cases := []struct {
		name string
		in   []items.Item
		want items.Quality
	}{
		{"ungraded hide caps", []items.Item{{ItemId: 900}}, fine},
		{"ungraded leather (made from hides) caps", []items.Item{{ItemId: 901}}, fine},
		{"thread never caps", []items.Item{{ItemId: 902}}, items.QualityPristine},
		{"graded hide is left to the normal cap", []items.Item{{ItemId: 900, Quality: items.QualityPristine}}, items.QualityPristine},
	}
	for _, c := range cases {
		if got := capUngradedGradable(items.QualityPristine, c.in); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := capUngradedGradable(items.QualityNone, []items.Item{{ItemId: 900}}); got != items.QualityNone {
		t.Errorf("an ungraded craft must stay ungraded, got %v", got)
	}
}
