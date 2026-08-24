package templates

import "testing"

// The generic spell helpfile is the fallback for every spell without a
// dedicated template, and it renders `help spell charm`. It used to print the
// raw target_defense_type enum -- "Resisted by: social defense" -- which names
// nothing the game says anywhere else, and "Resisted by: none defense" for the
// thirteen spells that declare none.
//
// U10c made this visible: slice B gave charm target_defense_type: social, which
// switched the line on for the only social spell in the game, so a player read
// "social defense" here and "defy" in help charm.
func TestDefenseName(t *testing.T) {
	fn, ok := funcMap["defensename"].(func(string) string)
	if !ok {
		t.Fatal("defensename is not registered in the template func map, so the " +
			"spell helpfile falls back to printing the raw enum")
	}

	for _, tc := range []struct{ in, want string }{
		{"social", "defy"},
		{"mental", "quell"},
		{"physical", "dodge or block"},
		{"Social", "defy"},    // spell YAML is author-written; do not trust case
		{" mental ", "quell"}, // or whitespace
		// "" suppresses the line entirely, which is what none wants -- the
		// template guards on the RESULT, not on the raw field.
		{"none", ""},
		{"", ""},
		{"nonsense", ""},
	} {
		if got := fn(tc.in); got != tc.want {
			t.Errorf("defensename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
