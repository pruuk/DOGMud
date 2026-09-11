package actions

import "github.com/GoMudEngine/GoMud/internal/characters"

// castViewer is the character whose perception limits a named cast: the
// caster when a player, nil (no limit) for a mob until slice F.
func castViewer(actor Actor) *characters.Character {
	if !actor.IsPlayer() {
		return nil
	}
	return actor.GetCharacter()
}
