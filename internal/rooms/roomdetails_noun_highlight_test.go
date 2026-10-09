package rooms

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// tagInsideTag matches an ansi open tag whose attributes hold another '<',
// which is what noun highlighting wrote when it ran over the minimap.
var tagInsideTag = regexp.MustCompile(`<ansi [^>]*<`)

func nounHighlightRoom(t *testing.T) (*Room, *users.UserRecord) {
	t.Helper()
	t.Cleanup(seedRegistry())
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7492: users.NewTestUser(7492, "nouner", "Nouner", 97492),
	}))
	r := roomManager.rooms[1]
	r.Description = "A dark room opens onto a cave."
	r.Nouns = map[string]string{"water": "Cold water.", "cave": "A cave.", "room": "A room."}
	return r, users.GetByUserId(7492)
}

func mapTile(slug string, symbol string) string {
	return `<ansi fg="map-room"><ansi fg="map-` + slug + `" bg="mapbg-` + slug + `">` + symbol + `</ansi></ansi>`
}

// #455: noun highlighting ran over each description line after the
// minimap had been appended to it, so a noun missing from the description
// ("water") matched inside a map tag: fg="map-deep-water" became
// fg="map-deep-<ansi fg="noun">water</ansi>" and the tag printed as text.
func TestGetDetails_NounHighlightSkipsTheMinimap(t *testing.T) {
	r, viewer := nounHighlightRoom(t)
	row := mapTile("deep-water", "~") + mapTile("cave", "C") + mapTile("deep-water", "~")
	tinymap := []string{row, row, row}

	desc := GetDetails(r, viewer, tinymap).Description
	if m := tagInsideTag.FindString(desc); m != "" {
		t.Fatalf("a tag is written inside another tag's attribute (%q) in %q", m, desc)
	}
	if strings.Count(desc, `fg="map-deep-water"`) != 6 || strings.Count(desc, `fg="map-cave"`) != 3 {
		t.Errorf("the minimap tags were rewritten: %q", desc)
	}
	for _, noun := range []string{"room", "cave"} {
		if !strings.Contains(desc, `<ansi fg="noun">`+noun+`</ansi>`) {
			t.Errorf("noun %q is not highlighted in the description: %q", noun, desc)
		}
	}
}
