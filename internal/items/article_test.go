package items

import "testing"

func TestIndefiniteArticle(t *testing.T) {
	for name, want := range map[string]string{
		"Iron Lockpicks":    "some",
		"Leather Boots":     "some",
		"Iron Sword":        "an",
		"Practice Sword":    "a",
		"Cutlass":           "a",
		"Aegis":             "an",
		"Ivory Handled Fan": "an",
		"":                  "a",
	} {
		if got := IndefiniteArticle(name); got != want {
			t.Errorf("IndefiniteArticle(%q) = %q, want %q", name, got, want)
		}
	}
}
