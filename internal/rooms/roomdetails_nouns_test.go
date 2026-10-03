package rooms

import (
	"strings"
	"testing"
)

// A noun that is also a word inside the tiny map's markup ("map" is in
// every tag's fg="map-room") is highlighted in the prose only. Highlighted
// after the map was put beside the prose, it was wrapped in a noun tag in
// the middle of the map's attribute, and the map rendered as raw markup.
func TestGetDetails_NounsDoNotReachIntoTheTinyMap(t *testing.T) {
	r, viewer := detailsSightRoom(t, "city")
	r.Description = `A hand-drawn map is pinned to the wall beside a lantern.`
	r.Nouns = map[string]string{`map`: `A map of the Marches.`}
	tag := `<ansi fg="map-room"><ansi fg="map-interior" bg="mapbg-interior">⌂</ansi></ansi>`
	tiny := []string{`╔═════╗`, `║` + tag + `─•─⩕║`, `║     ║`, `╚═════╝`}

	d := GetDetails(r, viewer, tiny)
	if !strings.Contains(d.Description, `<ansi fg="noun">map</ansi> is pinned`) {
		t.Fatalf("the noun is highlighted in the prose: %q", d.Description)
	}
	if !strings.Contains(d.Description, `║`+tag+`─•─⩕║`) {
		t.Fatalf("the tiny map's markup is untouched: %q", d.Description)
	}
	if strings.Count(d.Description, `fg="noun"`) != 1 {
		t.Fatalf("one highlight, in the prose only: %q", d.Description)
	}
}
