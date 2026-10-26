package characters

import (
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Size is the one reader of a character's size (#680). It is the species
// size, stepped to Large when a mutation's size effect has reached its rank
// (Titan Growth 3). An unregistered species reads as Medium. A guard forbids
// reading species size anywhere else.
func (c *Character) Size() species.Size {
	if mutations.GrowsLarge(c.Mutations) {
		return species.Large
	}
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && sp.Size != `` {
		return sp.Size
	}
	return species.Medium
}

// gripsInEveryHand reports whether a Large character holds a two-hander alone
// in every hand, not only the weapon hand: a Large species, or a character
// carrying the colossal-grip flag (Colossus Form). Only meaningful when
// Size() is Large.
func (c *Character) gripsInEveryHand() bool {
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && sp.Size == species.Large {
		return true
	}
	return mutations.HasMutationFlag(c.Mutations, `colossal-grip`)
}
