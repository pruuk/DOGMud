package crafting

import "testing"

func TestOutputCountAndSalvageShare(t *testing.T) {
	one := &RecipeSpec{Output: RecipeOutput{ItemId: 1, Quantity: 0}, Ingredients: []RecipeIngredient{{ItemTag: `a`, Quantity: 2}}}
	if one.OutputCount() != 1 {
		t.Fatal("a recipe that does not say how many makes one")
	}
	if got := one.SalvageIngredients(func(int) int { return 0 }); len(got) != 1 || got[0].Quantity != 2 {
		t.Fatalf("a single output gives back its whole recipe: %+v", got)
	}

	lens := &RecipeSpec{Output: RecipeOutput{ItemId: 2, Quantity: 3},
		Ingredients: []RecipeIngredient{{ItemTag: `glass`, Quantity: 1}, {ItemTag: `grit`, Quantity: 7}}}
	if lens.OutputCount() != 3 {
		t.Fatal("three lenses")
	}
	// Every unit's share, summed over the three, never passes what one craft
	// consumed, whatever the rolls.
	for roll := 0; roll < 3; roll++ {
		total := map[string]int{}
		for unit := 0; unit < 3; unit++ {
			for _, ing := range lens.SalvageIngredients(func(int) int { return roll }) {
				total[ing.ItemTag] += ing.Quantity
			}
		}
		if total[`glass`] > 3 || total[`grit`] > 9 {
			t.Fatalf("roll %d: %v", roll, total)
		}
	}
	lucky := lens.SalvageIngredients(func(int) int { return 0 })
	unlucky := lens.SalvageIngredients(func(n int) int { return n - 1 })
	if len(lucky) != 2 || lucky[0].Quantity != 1 || lucky[1].Quantity != 3 {
		t.Fatalf("a lucky share: one glass, two grit plus the remainder: %+v", lucky)
	}
	if len(unlucky) != 1 || unlucky[0].ItemTag != `grit` || unlucky[0].Quantity != 2 {
		t.Fatalf("an unlucky share: no glass, two grit: %+v", unlucky)
	}
}
