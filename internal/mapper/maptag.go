package mapper

import (
	"fmt"
	"strings"
)

// LegendSlug turns a map legend name into the slug its colour aliases are
// keyed by: lower case, every space a hyphen ("Deep Water" is "deep-water").
// The colour aliases are keyed by this slug, so a spaced name would match no
// alias and the tile would lose its colour. The room minimap (look), the map
// command and the map legend template (templates funcMap "mapslug") all use
// this one slug.
func LegendSlug(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

// ColorizeLegendLine wraps every legend symbol on one rendered map line in
// its colour tags. It walks the line once, rune by rune, so a symbol that
// also occurs inside a tag already written is never rewritten (a
// strings.Replace per symbol rewrote it whenever map order put that symbol's
// pass second).
func ColorizeLegendLine(line string, legend map[rune]string) string {
	var b strings.Builder
	for _, r := range line {
		name, ok := legend[r]
		if !ok {
			b.WriteRune(r)
			continue
		}
		slug := LegendSlug(name)
		fmt.Fprintf(&b, `<ansi fg="map-room"><ansi fg="map-%s" bg="mapbg-%s">%c</ansi></ansi>`, slug, slug, r)
	}
	return b.String()
}
