package characters

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// SpeechState is whether a character can make itself heard (M6 slice 3).
type SpeechState int

const (
	// Speaks: words work; a taunt reads from the rhetoric pool.
	Speaks SpeechState = iota
	// Speechless: the species has no voice; a taunt reads from the wordless
	// pool (snarls, roars), as a howl does.
	Speechless
	// Silenced: something stops the character making any sound; a taunt or
	// howl is refused. Wins over the species.
	Silenced
)

// SpeechState answers the one question every taunt path asks.
func (c *Character) SpeechState() SpeechState {
	if c.HasFlagFromAnySource(conditions.Silenced) {
		return Silenced
	}
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && !sp.CanSpeak() {
		return Speechless
	}
	return Speaks
}

// TauntPool names the taunt-messages pool this character's taunt narrates
// from: "wordless" for a Speechless character, "rhetoric" otherwise. A
// Silenced character never gets as far as narration (the taunt is refused), so
// it reads rhetoric here like any speaker.
//
// The names are plain strings because this package cannot import
// internal/combat; combat.TauntPoolRhetoric and combat.TauntPoolWordless are
// the same two spellings, and a test in internal/mobcommands pins that.
func (c *Character) TauntPool() string {
	if c.SpeechState() == Speechless {
		return "wordless"
	}
	return "rhetoric"
}
