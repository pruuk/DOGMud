package crafting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// The prod defect this file pins (owner, 2026-09-21):
//
// `craft setting` answered "You are missing: copper-wire." while 39 Copper
// Wire sat in the player's bank. PlanStoragePull is all-or-nothing by owner
// ruling, so a shortfall it cannot fully cover pulls NOTHING, and the refusal
// was then written from HasIngredients, which only ever sees what is carried.
// copper-wire is the recipe's FIRST ingredient, so it is the first tag that
// check comes up short on, and the player was named the one component they
// had plenty of. The real blocker was chrysalis-shard, listed second and
// absent everywhere.
//
// The two tests below are the chrysalis-setting case reduced to its shape:
// ingredient A listed first and covered by storage, ingredient B listed
// second and absent.

// chrysalisShapedRecipe mirrors the shipped chrysalis-setting recipe's
// ingredient list, which is the only part of it this defect depends on.
func chrysalisShapedRecipe() *RecipeSpec {
	return &RecipeSpec{
		RecipeId: "test-chrysalis-setting",
		Name:     "Test Chrysalis Setting",
		Skill:    "jewelcrafting",
		Ingredients: []RecipeIngredient{
			{ItemTag: "copper-wire", Quantity: 1},
			{ItemTag: "chrysalis-shard", Quantity: 1},
		},
	}
}

// TestHasIngredientsWithStorage_NamesWhatStorageCannotCover is the prod bug.
func TestHasIngredientsWithStorage_NamesWhatStorageCannotCover(t *testing.T) {
	recipe := chrysalisShapedRecipe()
	storage := []items.Item{makeItem("copper-wire"), makeItem("copper-wire")}

	// The carried-only answer is the WRONG one, and pinning it here is what
	// makes the storage-aware answer below meaningful rather than incidental.
	carriedOk, carriedMissing := HasIngredients(nil, nil, recipe)
	if carriedOk {
		t.Fatalf("carried-only check should come up short, got ok=true")
	}
	if carriedMissing != "copper-wire" {
		t.Fatalf("carried-only check should name the first-listed tag, got %q", carriedMissing)
	}

	ok, missing := HasIngredientsWithStorage(nil, nil, storage, recipe)
	if ok {
		t.Fatalf("storage cannot cover chrysalis-shard, so the recipe is not satisfiable; got ok=true")
	}
	if missing != "chrysalis-shard" {
		t.Fatalf("missing tag = %q, want \"chrysalis-shard\": storage holds the copper wire, so naming it tells the player to fetch what they already have", missing)
	}
}

// TestHasIngredientsWithStorage_IsDeterministicInRecipeOrder guards the map.
//
// PlanStoragePull decides all-or-nothing with `for _, remaining := range
// shortfall`, which iterates a MAP. That is fine for a boolean and useless
// for picking a name, so the storage-aware report must walk
// recipe.Ingredients instead. A regression to map order passes roughly half
// the time on two ingredients, so this runs the answer many times over.
func TestHasIngredientsWithStorage_IsDeterministicInRecipeOrder(t *testing.T) {
	recipe := &RecipeSpec{
		RecipeId: "test-two-missing",
		Name:     "Test Two Missing",
		Skill:    "jewelcrafting",
		Ingredients: []RecipeIngredient{
			{ItemTag: "aaa-first-listed", Quantity: 1},
			{ItemTag: "zzz-second-listed", Quantity: 1},
		},
	}

	for i := range 500 {
		ok, missing := HasIngredientsWithStorage(nil, nil, nil, recipe)
		if ok {
			t.Fatalf("iteration %d: nothing is held anywhere, got ok=true", i)
		}
		if missing != "aaa-first-listed" {
			t.Fatalf("iteration %d: missing tag = %q, want the first ingredient in RECIPE order, \"aaa-first-listed\"", i, missing)
		}
	}
}

// TestHasIngredientsWithStorage_StorageCompletesTheRecipe is the other side of
// the same answer: when the bank does cover the shortfall there is nothing to
// name, which is what keeps the `craft list` row reading as ready.
func TestHasIngredientsWithStorage_StorageCompletesTheRecipe(t *testing.T) {
	recipe := chrysalisShapedRecipe()
	inv := []items.Item{makeItem("copper-wire")}
	storage := []items.Item{makeItem("chrysalis-shard")}

	ok, missing := HasIngredientsWithStorage(inv, nil, storage, recipe)
	if !ok {
		t.Fatalf("carried wire plus banked shard covers the recipe, got ok=false missing=%q", missing)
	}
	if missing != "" {
		t.Fatalf("missing tag = %q, want empty", missing)
	}
}
