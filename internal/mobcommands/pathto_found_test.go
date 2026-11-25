package mobcommands

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// `pathto found <ids>` is a planner's errand (#781, ruling 13): the route
// takes only the secret exits the mob found. Without the word it is a
// scripted route and takes them all. homeOnly is the `home-impossible` route,
// marked or not.
func TestPathtoTargets(t *testing.T) {
	for _, c := range []struct {
		rest      string
		ids       []int
		foundOnly bool
		homeOnly  bool
	}{
		{mobs.PathtoFoundOnly + " 12 13", []int{12, 13}, true, false},
		{"home 4", []int{7, 4}, false, false},
		{"home", []int{7}, false, true},
		{mobs.PathtoFoundOnly + " home", []int{7}, true, true},
		{"7", []int{7}, false, false},
	} {
		ids, foundOnly, homeOnly := pathtoTargets(c.rest, 7)
		if !reflect.DeepEqual(ids, c.ids) || foundOnly != c.foundOnly || homeOnly != c.homeOnly {
			t.Errorf("pathtoTargets(%q) = %v %v %v, want %v %v %v", c.rest, ids, foundOnly, homeOnly, c.ids, c.foundOnly, c.homeOnly)
		}
	}
}

// The builder and the reader agree.
func TestPathtoCommand_RoundTrips(t *testing.T) {
	if got := mobs.PathtoCommand(true, "12", "13"); got != "pathto "+mobs.PathtoFoundOnly+" 12 13" {
		t.Errorf("found route: %q", got)
	}
	if got := mobs.PathtoCommand(false, "home"); got != "pathto home" {
		t.Errorf("scripted route: %q", got)
	}
}
