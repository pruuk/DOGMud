package mutators

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
)

// Owner ruling 18 (2026-11-26): a mutator exit is a named exit like a room's,
// so the mutator loader refuses one longer than exit.MaxNameLetters.
func TestMutatorValidate_RefusesAnExitNameLongerThanAButton(t *testing.T) {
	spec := MutatorSpec{MutatorId: "pushed-boulder", Exits: map[string]exit.RoomExit{
		"north":   {RoomId: 1},
		"pathway": {RoomId: 2}, // 7 letters
	}}
	err := spec.Validate()
	if err == nil {
		t.Fatal("a seven-letter mutator exit must be refused")
	}
	if !strings.Contains(err.Error(), "pushed-boulder") || !strings.Contains(err.Error(), `"pathway"`) {
		t.Errorf("error %q must name the mutator and the exit", err)
	}

	delete(spec.Exits, "pathway")
	spec.Exits["path"] = exit.RoomExit{RoomId: 2}
	if err := spec.Validate(); err != nil {
		t.Errorf("a short name refused: %v", err)
	}
}
