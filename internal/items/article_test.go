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
