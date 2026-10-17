package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// The slam split (#541, owner rulings S1 to S3, 2026-10-16): slam carried
// every heavy, hoofed, amorphous and elemental body, so its lines named fists
// and feet that half its users lack. Each body now fights through its own
// file. Pins the shipped data so a later edit cannot send a species back.
func TestSlamSplitAssignments(t *testing.T) {
	all := dogmudSpecies(t)
	want := map[int]items.ItemSubType{
		37: items.Slam,    // earth elemental, stone fists
		45: items.Slam,    // automaton, servo-driven fists
		26: items.Trample, // horse, hooves
		15: items.Lash,    // fungal colony, spore tendrils
		16: items.Lash,    // slime, pseudopods
		36: items.Blast,   // water
		38: items.Blast,   // air
		39: items.Blast,   // fire
		40: items.Blast,   // magma
		41: items.Blast,   // sand
		42: items.Blast,   // storm
		43: items.Blast,   // ice
		44: items.Claws,   // smoke: it materialises claws and rakes (S3)
	}
	for id, attack := range want {
		sp := all[id]
		if sp == nil {
			t.Fatalf("species %d missing", id)
		}
		if sp.NaturalAttack != attack {
			t.Errorf("species %d (%s) natural_attack = %q, want %q", id, sp.Name, sp.NaturalAttack, attack)
		}
	}
	if got := all[44].UnarmedName; got != "smoke-wrought claws" {
		t.Errorf("smoke elemental unarmedname = %q, want %q", got, "smoke-wrought claws")
	}

	// No species is left on slam outside the two heavy bodies.
	for id, sp := range all {
		if sp.NaturalAttack == items.Slam && id != 37 && id != 45 {
			t.Errorf("species %d (%s) still uses slam; slam is for heavy bodies only", id, sp.Name)
		}
	}
}
