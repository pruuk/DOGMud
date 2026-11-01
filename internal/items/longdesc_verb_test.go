package items

import (
	"strings"
	"testing"
)

// Look text names the command that takes the item (#375 phase 7): quaff a
// potion, drink a cooked drink, eat food.
func TestLongDescriptionNamesTheVerb(t *testing.T) {
	var i Item
	for _, c := range []struct {
		spec ItemSpec
		want string
	}{
		{ItemSpec{Type: Potion, Subtype: Drinkable}, `<ansi fg="command">quaff</ansi>`},
		{ItemSpec{Type: Drink, Subtype: Edible}, `<ansi fg="command">drink</ansi>`},
		{ItemSpec{Type: Food, Subtype: Edible}, `<ansi fg="command">eat</ansi>`},
	} {
		if got := i.longDescriptionFrom(c.spec); !strings.Contains(got, c.want) {
			t.Errorf("%s/%s: %q, want it to name %s", c.spec.Type, c.spec.Subtype, got, c.want)
		}
	}
}
