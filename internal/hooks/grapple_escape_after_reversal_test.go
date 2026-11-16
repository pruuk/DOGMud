package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/grapplemessaging"
)

// #350: a reversal then an escape on the very next round read as two clean
// wins in a row. The escape that answers a reversal at once has its own line
// (owner: flavour only, the odds are unchanged); any other escape keeps the
// generic one.
func TestEscapeKey_RightAfterAReversal(t *testing.T) {
	lib := &grapplemessaging.Library{Escapes: map[string]grapplemessaging.TemplateTriad{
		"generic_escape":        {},
		"escape_after_reversal": {},
	}}
	reverser, reversed := &characters.Character{}, &characters.Character{}

	if got := escapeKey(reverser, 10, lib); got != "generic_escape" {
		t.Errorf("no reversal: %q, want generic_escape", got)
	}
	markGrappleReversal(reverser, reversed, 10)
	if got := escapeKey(reverser, 11, lib); got != "escape_after_reversal" {
		t.Errorf("escape the round after a reversal: %q, want escape_after_reversal", got)
	}
	if got := escapeKey(reverser, 13, lib); got != "generic_escape" {
		t.Errorf("escape rounds after a reversal: %q, want generic_escape", got)
	}

	delete(lib.Escapes, "escape_after_reversal")
	if got := escapeKey(reverser, 11, lib); got != "generic_escape" {
		t.Errorf("a library without the line falls back: %q", got)
	}
}
