package templates

import (
	"strings"
	"testing"
	"text/template"
)

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

// The directive as it appears in help/spell.template. Rendering it here catches
// the two ways the fix could fail silently: defensename not being registered in
// the func map (Process would return a template error at runtime, on a page
// nobody tests), and "none" printing an empty Resisted-by row instead of
// omitting the line.
func TestSpellTemplateDefenceRow(t *testing.T) {
	const fragment = `{{ with defensename .TargetDefenseType -}}` +
		"Resisted by: {{ . }}\n" +
		`{{ end -}}`

	tmpl, err := template.New("frag").Funcs(funcMap).Parse(fragment)
	if err != nil {
		t.Fatalf("parsing the spell.template defence row: %v", err)
	}

	for _, tc := range []struct {
		defenceType string
		want        string
	}{
		{"social", "Resisted by: defy\n"},  // charm, the only social spell
		{"mental", "Resisted by: quell\n"}, // 9 spells
		{"physical", "Resisted by: dodge or block\n"},
		{"none", ""}, // 13 spells: the whole row must disappear
		{"", ""},
	} {
		var sb strings.Builder
		if err := tmpl.Execute(&sb, struct{ TargetDefenseType string }{tc.defenceType}); err != nil {
			t.Fatalf("executing with %q: %v", tc.defenceType, err)
		}
		if got := sb.String(); got != tc.want {
			t.Errorf("defence row for %q = %q, want %q", tc.defenceType, got, tc.want)
		}
	}
}
