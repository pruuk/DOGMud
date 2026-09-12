package species

import "testing"

// The guard that would have caught buff 29 on the day it broke: a species
// referencing a buff id with no definition must fail the boot, not run for
// months with 67 mobs silently blind.
func TestValidateSpeciesBuffIdsPanicsOnMissingBuff(t *testing.T) {
	orig := allSpecies
	t.Cleanup(func() { allSpecies = orig })
	allSpecies = map[int]*Species{
		99: {SpeciesId: 99, Name: "Fixture", BuffIds: []int{123456}},
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a species referencing a buff that does not exist")
		}
	}()
	ValidateSpeciesBuffIds(func(int) bool { return false })
}

func TestValidateSpeciesBuffIdsAcceptsKnownBuffs(t *testing.T) {
	orig := allSpecies
	t.Cleanup(func() { allSpecies = orig })
	allSpecies = map[int]*Species{
		99: {SpeciesId: 99, Name: "Fixture", BuffIds: []int{29}},
	}
	ValidateSpeciesBuffIds(func(id int) bool { return id == 29 })
}

// A species with no buffids at all must not trip the guard.
func TestValidateSpeciesBuffIdsIgnoresSpeciesWithNoBuffs(t *testing.T) {
	orig := allSpecies
	t.Cleanup(func() { allSpecies = orig })
	allSpecies = map[int]*Species{
		98: {SpeciesId: 98, Name: "Plain"},
	}
	ValidateSpeciesBuffIds(func(int) bool { return false })
}
