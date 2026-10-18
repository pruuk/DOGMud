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
