package items

import "testing"

func TestArticlePrefix(t *testing.T) {
	for name, want := range map[string]string{
		"Iron Lockpicks":    "some ",
		"Leather Boots":     "some ",
		"Iron Sword":        "an ",
		"Practice Sword":    "a ",
		"Cutlass":           "a ",
		"Aegis":             "an ",
		"Ivory Handled Fan": "an ",
		"":                  "a ",
		// A name that already starts with an article gets no second one
		// (2026-10-17 newbie-duo playtest: "You find an A Sheaf of Gleaned
		// Grain.").
		"A Sheaf of Gleaned Grain": "",
		"An Old Map":               "",
		"the Crown of Ash":         "",
		"Some Dried Herbs":         "",
		// A word that only starts like an article is not one.
		"Anvil":           "an ",
		"Theodric's Pick": "a ",
		"Somber Mask":     "a ",
	} {
		if got := ArticlePrefix(name); got != want {
			t.Errorf("ArticlePrefix(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestMidSentenceName: an item's own leading article reads lowercase after
// "You find" (#620: "You find A Sheaf of Gleaned Grain."), and every other
// capital stays.
func TestMidSentenceName(t *testing.T) {
	for name, want := range map[string]string{
		"A Sheaf of Gleaned Grain": "a Sheaf of Gleaned Grain",
		"An Old Map":               "an Old Map",
		"The Crown of Ash":         "the Crown of Ash",
		"Some Dried Herbs":         "some Dried Herbs",
		"the Crown of Ash":         "the Crown of Ash",
		"Iron Sword":               "Iron Sword",
		"Anvil":                    "Anvil",
		"Theodric's Pick":          "Theodric's Pick",
		"A":                        "A",
		"":                         "",
	} {
		if got := MidSentenceName(name); got != want {
			t.Errorf("MidSentenceName(%q) = %q, want %q", name, got, want)
		}
	}
	// The pair reads as one clean phrase.
	n := "A Sheaf of Gleaned Grain"
	if got := ArticlePrefix(n) + MidSentenceName(n); got != "a Sheaf of Gleaned Grain" {
		t.Errorf("ArticlePrefix+MidSentenceName = %q", got)
	}
}

// A quest item's DisplayName opens with a coloured star, and a name can be
// coloured: the article is judged on the visible name after the tags and
// the star, and the name's own article is lowercased there, so a recipe
// line reads "To receive ★an Old Key", never "To receive a ★An Old Key"
// (review of plan 3b-3).
func TestArticleIsJudgedOnTheVisibleName(t *testing.T) {
	star := `<ansi fg="questflag">★</ansi>`
	for name, want := range map[string]string{
		star + "An Old Key":                   "",
		star + "Old Key":                      "an ",
		star + "Iron Lockpicks":               "some ",
		star + "Map":                          "a ",
		`<ansi fg="red">Iron Sword</ansi>`:    "an ",
		`<ansi fg="red">The Crown</ansi> (x)`: "",
	} {
		if got := ArticlePrefix(name); got != want {
			t.Errorf("ArticlePrefix(%q) = %q, want %q", name, got, want)
		}
	}
	for name, want := range map[string]string{
		star + "An Old Key": star + "an Old Key",
		star + "Old Key":    star + "Old Key",
		star + "The":        star + "The",
	} {
		if got := MidSentenceName(name); got != want {
			t.Errorf("MidSentenceName(%q) = %q, want %q", name, got, want)
		}
	}
	if got := NameWithoutArticle(star + "A Culvert Slime"); got != star+"Culvert Slime" {
		t.Errorf("NameWithoutArticle = %q", got)
	}
	// The real thing: a quest item's DisplayName.
	itm := Item{ItemId: 99400, Spec: &ItemSpec{ItemId: 99400, Name: "An Old Key", QuestToken: "q-1"}}
	n := itm.DisplayName()
	if got := ArticlePrefix(n) + MidSentenceName(n); got != star+"an Old Key" {
		t.Errorf("a quest item reads %q", got)
	}
}

// TestNameWithoutArticle: a compound noun that brings its own article or count
// drops the name's ("4 Culvert Slime corpses", #572).
func TestNameWithoutArticle(t *testing.T) {
	for name, want := range map[string]string{
		"A Culvert Slime": "Culvert Slime",
		"An Ox":           "Ox",
		"The Old White":   "Old White",
		"Sewer Rat":       "Sewer Rat",
		"Anvil":           "Anvil",
		"The":             "The",
		"":                "",
	} {
		if got := NameWithoutArticle(name); got != want {
			t.Errorf("NameWithoutArticle(%q) = %q, want %q", name, got, want)
		}
	}
}
