package mobcommands

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// `pathto found <ids>` is a planner's errand (#781, ruling 13): the route
// takes only the secret exits the mob found. Without the word it is a
// scripted route and takes them all.
func TestPathtoTargets(t *testing.T) {
	ids, foundOnly := pathtoTargets(mobs.PathtoFoundOnly+" 12 13", 7)
	if !foundOnly || !reflect.DeepEqual(ids, []int{12, 13}) {
		t.Fatalf("found route: %v %v, want [12 13] true", ids, foundOnly)
	}
	ids, foundOnly = pathtoTargets("home 4", 7)
	if foundOnly || !reflect.DeepEqual(ids, []int{7, 4}) {
		t.Fatalf("scripted route: %v %v, want [7 4] false", ids, foundOnly)
	}
}
