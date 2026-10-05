package mobs

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// HarvestTagExists reports whether some item carries the component tag. It
// is the tagExists callback for species.HarvestTable.Validate.
func HarvestTagExists(tag string) bool {
	return items.FindSpecByComponentTag(tag) != nil
}

// HarvestItemExists reports whether an item id is defined. It is the
// itemExists callback for species.HarvestTable.Validate.
func HarvestItemExists(itemId int) bool {
	return items.GetItemSpec(itemId) != nil
}

// ResolveHarvest is what this mob's carcass gives: its species table with the
// mob's own harvest overriding it per section (species.MergeHarvest). A mob of
// an unknown species resolves to its own table alone.
func ResolveHarvest(m *Mob) species.HarvestTable {
	if m == nil {
		return species.HarvestTable{}
	}
	var base *species.HarvestTable
	if sp := species.GetSpecies(m.Character.SpeciesId); sp != nil {
		base = sp.Harvest
	}
	return species.MergeHarvest(base, m.Harvest)
}

// ValidateMobHarvest panics on any mob template whose harvest table names an
// unknown material, tool or quantity. Called from main after mobs and items
// load, beside species.ValidateSpeciesHarvest.
func ValidateMobHarvest() {
	mobsMu.RLock()
	defer mobsMu.RUnlock()
	for id, m := range mobs {
		if err := m.Harvest.Validate(HarvestTagExists, HarvestItemExists); err != nil {
			panic(fmt.Sprintf("mob %d (%s): %v", id, m.Character.Name, err))
		}
	}
}
