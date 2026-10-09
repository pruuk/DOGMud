package rooms

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/mutators"
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

// nounModifierRoom puts room 1 of seedRegistry under one description
// modifier coloured by a seeded pattern.
func nounModifierRoom(t *testing.T, mod *mutators.TextModifier) (*Room, *users.UserRecord) {
	t.Helper()
	r, viewer := nounHighlightRoom(t)
	t.Cleanup(colorpatterns.SeedPatternsForTest(map[string][]int{"test-flame": {196, 202, 208}}))
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId:           "test-noun-modifier",
		DescriptionModifier: mod,
	}))
	GetZoneConfig("TestZone").Mutators.Add("test-noun-modifier")
	return r, viewer
}

// A recolour-only modifier (the default world's wildfire: replace with no
// text) coloured the description before nouns were highlighted, so the
// pattern's per-rune tags hid every noun from the highlighter.
func TestGetDetails_NounsHighlightUnderARecolourModifier(t *testing.T) {
	for _, tinymap := range [][]string{nil, {"~~~", "~~~"}} {
		r, viewer := nounModifierRoom(t, &mutators.TextModifier{
			Behavior:     mutators.TextReplace,
			ColorPattern: "test-flame",
		})
		var desc string
		if tinymap == nil {
			desc = GetDetails(r, viewer).Description
		} else {
			desc = GetDetails(r, viewer, tinymap).Description
		}
		for _, noun := range []string{"room", "cave"} {
			if !strings.Contains(desc, `<ansi fg="noun">`+noun+`</ansi>`) {
				t.Errorf("minimap=%v: noun %q is not highlighted under the recolour: %q", tinymap != nil, noun, desc)
			}
		}
		if m := tagInsideTag.FindString(desc); m != "" {
			t.Errorf("minimap=%v: a tag is written inside another tag (%q): %q", tinymap != nil, m, desc)
		}
	}
}

// A noun inside a coloured modifier's own text is highlighted like one in
// the description.
func TestGetDetails_NounsHighlightInsideAColouredModifier(t *testing.T) {
	r, viewer := nounModifierRoom(t, &mutators.TextModifier{
		Behavior:     mutators.TextAppend,
		Text:         "Smoke rolls over the water.",
		ColorPattern: "test-flame",
	})
	desc := GetDetails(r, viewer).Description
	if !strings.Contains(desc, `<ansi fg="noun">water</ansi>`) {
		t.Errorf("the noun in the coloured modifier is not highlighted: %q", desc)
	}
}

// A longer noun claims its span before a shorter one inside it, so the
// shorter noun never nests a tag inside the longer one's.
func TestHighlightNouns_LongerNounFirst(t *testing.T) {
	got := highlightNouns("Still deep water here, and water.", map[string]string{"water": "", "deep water": ""})
	want := `Still <ansi fg="noun">deep water</ansi> here, and <ansi fg="noun">water</ansi>.`
	if got != want {
		t.Errorf("highlightNouns = %q, want %q", got, want)
	}
}
