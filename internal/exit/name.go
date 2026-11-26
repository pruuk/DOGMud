package exit

import (
	"fmt"
	"unicode/utf8"
)

// MaxNameLetters is the longest a named exit may be, in letters, so its name
// fits a 48px deck button at 12px text (owner ruling 18, 2026-11-26, touch arc
// spec docs/superpowers/specs/2026-11-22-moving-by-touch-design.md). The room
// loader (rooms.Room.Validate), the mutator loader (mutators.MutatorSpec.
// Validate), the builders, the behaviour-tree add_temp_exit check, the shipped
// content guard (internal/guards/exit_name_length_guard_test.go) and the
// constant temporary exits (ferry plank, mob rift) all read this one number.
//
// Exempt: the ten compass words, which have fixed buttons, and the instance
// portal exit keyed "<zone>-<userId>" (behaviortree actOpenInstancePortal),
// whose button the owner ruled is wired to that player's exit with a label
// that leaves the number off.
const MaxNameLetters = 6

// IsCompassWord reports whether name is one of the ten compass words: the
// four cardinals, the four diagonals, up and down. It is the one list;
// actions.IsCompassDirection reads it.
func IsCompassWord(name string) bool {
	switch name {
	case "north", "south", "east", "west", "up", "down",
		"northwest", "northeast", "southwest", "southeast":
		return true
	}
	return false
}

// CheckName refuses a named exit longer than MaxNameLetters letters. A
// compass word is always accepted.
func CheckName(name string) error {
	if IsCompassWord(name) {
		return nil
	}
	if n := utf8.RuneCountInString(name); n > MaxNameLetters {
		return fmt.Errorf("exit %q is %d letters; a named exit is at most %d letters so it fits a deck button (owner ruling 18)", name, n, MaxNameLetters)
	}
	return nil
}
