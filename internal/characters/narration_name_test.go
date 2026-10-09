package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/pets"
)

// #453: GetCharacterName(true) is the narration name. Every production caller
// feeds it into a line about what the holder did ("{actee} seems to shimmer
// and fade from view"), so it carries the identity tag alone. The adjective
// span, the quest star and " and <pet>" belong to look and the rosters,
// which render through GetPlayerName / GetMobName, not this.
func TestGetCharacterName_TaggedIsTheIdentityTagAlone(t *testing.T) {
	c := perceivesChar(t, "Sil Vantage")
	c.Health = 10
	c.Adjectives = []string{`lit`}
	c.Pet = pets.Pet{Type: `dog`, Name: `Rex`}
	perceivesHide(t, c)

	got := c.GetCharacterName(true)
	if got != `<ansi fg="username">Sil Vantage</ansi>` {
		t.Fatalf("narration name must be the identity tag alone, got %q", got)
	}

	// look keeps every adjective and the pet.
	full := c.GetPlayerName(0).String()
	for _, want := range []string{`hidden`, ` and `} {
		if !strings.Contains(full, want) {
			t.Fatalf("GetPlayerName lost %q: %q", want, full)
		}
	}
}

// The suffix is part of the tag (the colour), not the adjective list, so a
// dead holder keeps it.
func TestGetCharacterName_TaggedKeepsTheDeadSuffix(t *testing.T) {
	c := perceivesChar(t, "Grix")
	c.Health = 0
	if got := c.GetCharacterName(true); got != `<ansi fg="username-dead">Grix</ansi>` {
		t.Fatalf("got %q", got)
	}
}
