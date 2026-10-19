package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Ruling D: bears' default attack is maul, cats' is pounce.
func TestBearsMaulAndCatsPounce(t *testing.T) {
	all := dogmudSpecies(t)
	if got := all[3].NaturalAttack; got != items.Maul {
		t.Errorf("bear natural_attack = %q, want maul", got)
	}
	if got := all[11].NaturalAttack; got != items.Pounce {
		t.Errorf("feline natural_attack = %q, want pounce", got)
	}
	for _, sub := range []items.ItemSubType{items.Maul, items.Pounce} {
		if err := validateNaturalAttack(&Species{SpeciesId: 1, Name: "x", NaturalAttack: sub}); err != nil {
			t.Errorf("%s must be a valid natural attack: %v", sub, err)
		}
	}
}
