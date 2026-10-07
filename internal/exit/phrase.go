package exit

import (
	"fmt"
	"strings"
)

// Movement and look lines drop an exit name into "from the %s" or "toward
// the %s", which reads well for "north" or "trapdoor" and badly for the two
// vertical exits: "enters from the up", "You peer toward the up" (#430).
// These helpers phrase up and down as a direction and every other exit as a
// place.

// vertical returns "up" or "down" when exitName is one of the two vertical
// exits, ignoring case, and "" otherwise.
func vertical(exitName string) string {
	switch strings.ToLower(strings.TrimSpace(exitName)) {
	case "up":
		return "up"
	case "down":
		return "down"
	}
	return ""
}

// FromPhrase is where an arrival comes from: "from above", "from below", or
// `from the <exit>` with the exit name in the exit colour.
func FromPhrase(exitName string) string {
	switch vertical(exitName) {
	case "up":
		return "from above"
	case "down":
		return "from below"
	}
	return fmt.Sprintf(`from the <ansi fg="exit">%s</ansi>`, exitName)
}

// TowardPhrase is the direction of a look: "up", "down", or "toward the
// <exit>" (uncoloured, as the peer lines have always been).
func TowardPhrase(exitName string) string {
	if v := vertical(exitName); v != "" {
		return v
	}
	return "toward the " + exitName
}

// DeparturePhrase is the direction of a departure: "upward", "downward", or
// `towards the <exit> exit` with the exit name in the exit colour.
func DeparturePhrase(exitName string) string {
	if v := vertical(exitName); v != "" {
		return v + "ward"
	}
	return fmt.Sprintf(`towards the <ansi fg="exit">%s</ansi> exit`, exitName)
}
