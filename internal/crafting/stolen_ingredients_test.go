package crafting

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Hot stolen goods (a merchant chest's) are not ingredients: every count,
// selection and consumption goes through componentTagOf, which gives them no
// tag while they are hot, so crafting cannot shed their heat. Cooled, they
// are ordinary materials.

func stolenItem(tag string) items.Item {
	it := makeItem(tag)
	it.StolenFrom = "Smith Brindle"
	it.StolenFromMob = 337
	it.MarkTaken(1, "Stillwater", time.Now().Add(-time.Hour))
	return it
}

func TestStolenGoodsAreNotIngredients(t *testing.T) {
	recipe := &RecipeSpec{
		RecipeId:    "test-blade",
		Ingredients: []RecipeIngredient{{ItemTag: "steel-ingot", Quantity: 1}},
	}

	if ok, missing := HasIngredients([]items.Item{stolenItem("steel-ingot")}, nil, recipe); ok || missing != "steel-ingot" {
		t.Fatalf("a stolen ingot must not count: ok=%v missing=%q", ok, missing)
	}
	if ok, _ := HasIngredientsWithStorage(nil, nil, []items.Item{stolenItem("steel-ingot")}, recipe); ok {
		t.Fatal("nor may storage supply a stolen one")
	}

	inv := []items.Item{stolenItem("steel-ingot"), makeItem("steel-ingot")}
	if ok, _ := HasIngredients(inv, nil, recipe); !ok {
		t.Fatal("a clean ingot beside it still counts")
	}
	left, _ := ConsumeIngredients(inv, nil, recipe)
	if len(left) != 1 || !left[0].IsStolen() {
		t.Fatalf("the clean ingot is spent and the stolen one kept, got %+v", left)
	}
}

func TestCooledStolenGoodsAreIngredientsAgain(t *testing.T) {
	recipe := &RecipeSpec{
		RecipeId:    "test-blade",
		Ingredients: []RecipeIngredient{{ItemTag: "steel-ingot", Quantity: 1}},
	}
	cold := stolenItem("steel-ingot")
	cold.StolenAt = time.Now().Add(-100 * time.Hour).Unix()
	if ok, _ := HasIngredients([]items.Item{cold}, nil, recipe); !ok {
		t.Fatal("a cooled stolen ingot is an ordinary ingot again")
	}
}
