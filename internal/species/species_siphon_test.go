package species

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// A siphon species steals life through the drain special move, which only
// fires for a species with lifedrain set, so siphon without it is a lie.
func TestValidateSiphonHasLifeDrain(t *testing.T) {
	bare := &Species{SpeciesId: 7, Name: "ghoul", NaturalAttack: items.Siphon}
	if err := validateSiphonHasLifeDrain(bare); err == nil {
		t.Fatal("siphon without lifedrain must error")
	} else if !strings.Contains(err.Error(), "7") || !strings.Contains(err.Error(), "ghoul") {
		t.Errorf("error should name the species id and name, got: %v", err)
	}

	draining := &Species{SpeciesId: 8, Name: "wraith", NaturalAttack: items.Siphon, LifeDrain: true}
	if err := validateSiphonHasLifeDrain(draining); err != nil {
		t.Errorf("siphon with lifedrain should validate: %v", err)
	}

	for _, attack := range []items.ItemSubType{items.Bite, items.Claws, items.Gore, "", items.Unarmed} {
		s := &Species{SpeciesId: 9, Name: "beast", NaturalAttack: attack}
		if err := validateSiphonHasLifeDrain(s); err != nil {
			t.Errorf("natural_attack %q without lifedrain should validate: %v", attack, err)
		}
	}
}

// Wraiths and spectres siphon life with their basic attacks (owner
// 2026-10-14); the spectre's unarmed name reads "cold touch" so siphon lines
// such as "turns their {itemname} on you" read right. Pins the shipped data so
// a later edit cannot quietly send them back to the generic lines.
func TestShippedSpectralSpeciesSiphon(t *testing.T) {
	all := dogmudSpecies(t)
	want := map[int]string{32: "touch", 33: "cold touch"}
	for id, unarmed := range want {
		sp := all[id]
		if sp == nil {
			t.Fatalf("species %d missing", id)
		}
		if sp.NaturalAttack != items.Siphon {
			t.Errorf("species %d (%s) natural_attack = %q, want %q", id, sp.Name, sp.NaturalAttack, items.Siphon)
		}
		if !sp.LifeDrain {
			t.Errorf("species %d (%s) must keep lifedrain: true", id, sp.Name)
		}
		if sp.UnarmedName != unarmed {
			t.Errorf("species %d (%s) unarmedname = %q, want %q", id, sp.Name, sp.UnarmedName, unarmed)
		}
	}
}
