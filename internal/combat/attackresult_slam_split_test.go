package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// The slam split (#541): trample and lash keep slam's blunt band, blast takes
// the caster band siphon uses, so no swing changes colour by accident.
func TestCategoryForWeaponSubtype_SlamSplit(t *testing.T) {
	cases := map[items.ItemSubType]messaging.Category{
		items.Slam:    messaging.CategoryHitBlunt,
		items.Trample: messaging.CategoryHitBlunt,
		items.Lash:    messaging.CategoryHitBlunt,
		items.Blast:   messaging.CategoryHitCaster,
	}
	for sub, want := range cases {
		if got := CategoryForWeaponSubtype(sub); got != want {
			t.Errorf("CategoryForWeaponSubtype(%q) = %v, want %v", sub, got, want)
		}
	}
}
