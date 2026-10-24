package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// atFullPools returns char with Health and Stamina at their maximums: the
// build PowerScore rates (#301), as opposed to the live state the fight read
// reads. char is a by-value copy, so no live character is touched. This file
// exists so internal/guards/pool_mutation_guard_test.go can exempt exactly these two writes.
func atFullPools(char characters.Character) characters.Character {
	char.Health = char.HealthMax.Value
	char.Stamina = char.StaminaMax.Value
	return char
}
