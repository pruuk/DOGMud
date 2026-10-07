package exit

import (
	"fmt"
	"regexp"
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

// verticalLinePhrase matches the ways the combat and shooting templates put a
// filled exit name after "the": "from the {entrancename} direction", "from
// beyond the {exitname}", "through/towards/toward the {exitname}", "on the
// {exitname}" and "watching the {exitname}", with or without the exit colour
// tag and the trailing "direction". Only "up" and "down" are matched.
var verticalLinePhrase = regexp.MustCompile(
	`\b(from beyond|from|through|towards|toward|on|watching) the (?:<ansi fg="exit">)?(up|down)\b(?:</ansi>)?(?: direction\b)?`)

// PhraseVerticalExits rewrites a rendered combat or shooting line so an up or
// down exit reads as a direction: "from the up" becomes "from above", "from
// beyond the up" becomes "from somewhere above", "through/towards the up"
// becomes "upward", and "on/watching the up" becomes "on/watching the way
// up". exitNames are the values the line's exit tokens were filled with; a
// phrase is rewritten only when its direction is one of them, so a cardinal
// or named exit ("from the north", "through the trapdoor") is left as
// rendered.
func PhraseVerticalExits(line string, exitNames ...string) string {
	filled := map[string]bool{}
	for _, n := range exitNames {
		if v := vertical(n); v != "" {
			filled[v] = true
		}
	}
	if len(filled) == 0 {
		return line
	}
	return verticalLinePhrase.ReplaceAllStringFunc(line, func(m string) string {
		parts := verticalLinePhrase.FindStringSubmatch(m)
		prep, dir := parts[1], parts[2]
		if !filled[dir] {
			return m
		}
		place := "above"
		if dir == "down" {
			place = "below"
		}
		switch prep {
		case "from":
			return "from " + place
		case "from beyond":
			return "from somewhere " + place
		case "through", "towards", "toward":
			return dir + "ward"
		}
		return prep + " the way " + dir
	})
}
