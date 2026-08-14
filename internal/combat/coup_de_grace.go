package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// IsDying reports whether a character has taken a lethal blow whose death has
// not resolved yet.
//
// Health-based ON PURPOSE. This is the state the renderer and combat targeting
// care about, and it is true the instant the blow lands, before the queued
// death reaches the event flush.
//
// It is NOT the same as Character.DeathQueued, which tracks whether a
// CharacterDied event is in flight and is what the backstop sweeps skip on.
// Conflating the two breaks the sweeps: a character reaped by a sweep is dying
// but NOT queued, so a sweep skipping on "dying" would skip the entire
// population it exists for.
func IsDying(char *characters.Character) bool {
	if char == nil {
		return false
	}
	return char.Health < 1 && char.IsAlive()
}
