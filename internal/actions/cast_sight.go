package actions

import "github.com/GoMudEngine/GoMud/internal/characters"

// castViewer is the character whose perception limits a named lookup: a
// creature it cannot perceive cannot be named.
//
// Slice F: this returns the MOB's character too. Before, it returned nil for a
// mob, which switched the perception filter off, so a mob could name a hidden
// creature in a cast or a track that a player could not.
func castViewer(actor Actor) *characters.Character {
	return actor.GetCharacter()
}
