package messaging

import "testing"

func TestAnonymizeReplacesUsernameTag(t *testing.T) {
	in := `<ansi fg="username">Calabe</ansi> attacks`
	want := `<ansi fg="combat-anon">a figure</ansi> attacks`
	if got := Anonymize(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAnonymizeReplacesMobnameTag(t *testing.T) {
	in := `<ansi fg="mobname">Thornwall Thug</ansi> snarls`
	want := `<ansi fg="combat-anon">a figure</ansi> snarls`
	if got := Anonymize(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAnonymizeReplacesIndexedAndSuffixedMobnameTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"duplicate_two", `<ansi fg="mobname-dup2">Thornwall Thug #2</ansi> snarls`},
		{"duplicate_four", `<ansi fg="mobname-dup4">Thornwall Thug #4</ansi> snarls`},
		{"display_suffix", `<ansi fg="mobname-target">Thornwall Thug</ansi> snarls`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := `<ansi fg="combat-anon">a figure</ansi> snarls`
			if got := Anonymize(tc.in); got != want {
				t.Fatalf("indexed/suffixed mob identity leaked: got %q want %q", got, want)
			}
		})
	}
}

func TestAnonymizeReplacesPetnameTag(t *testing.T) {
	in := `<ansi fg="petname">Rex</ansi> follows`
	want := `<ansi fg="combat-anon">a figure</ansi> follows`
	if got := Anonymize(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAnonymizeReplacesMultipleNamesInOneLine(t *testing.T) {
	in := `<ansi fg="mobname">Thug</ansi> strikes ` +
		`<ansi fg="username">Calabe</ansi> with a longsword`
	want := `<ansi fg="combat-anon">a figure</ansi> strikes ` +
		`<ansi fg="combat-anon">a figure</ansi> with a longsword`
	if got := Anonymize(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAnonymizeLeavesOtherTagsAlone(t *testing.T) {
	in := `<ansi fg="hit-melee">strikes deeply</ansi>`
	if got := Anonymize(in); got != in {
		t.Fatalf("non-name tag must pass through, got %q", got)
	}
}

func TestAnonymizeEmpty(t *testing.T) {
	if got := Anonymize(""); got != "" {
		t.Fatalf("empty must pass through, got %q", got)
	}
}

// TestAnonymizeReplacesSuffixedUsernameTags guards the player half of the
// suffix rule. FormattedName.String renders `username-aggro` and
// `username-dead` as well as plain `username`. Until slice B,
// GetCharacterName(true) rendered `username-aggro` for any character not
// fighting a player, so the suffixed form was the COMMON one in room text; it
// is now the form a player reads for a foe fighting them. Only mob tags
// accepted a suffix, so a player's name reached infrared-only observers in
// full.
func TestAnonymizeReplacesSuffixedUsernameTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"aggro", `<ansi fg="username-aggro">Calabe</ansi> glows`},
		{"dead", `<ansi fg="username-dead">Calabe</ansi> glows`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := `<ansi fg="combat-anon">a figure</ansi> glows`
			if got := Anonymize(tc.in); got != want {
				t.Fatalf("suffixed player identity leaked: got %q want %q", got, want)
			}
		})
	}
}

// TestAnonymizeLeavesLookalikeTagsAlone keeps the suffix rule from swallowing
// a colour alias that merely starts with an identity word.
func TestAnonymizeLeavesLookalikeTagsAlone(t *testing.T) {
	in := `<ansi fg="usernames">roster</ansi> <ansi fg="mobnamex">x</ansi>`
	if got := Anonymize(in); got != in {
		t.Fatalf("non-identity tag was anonymized: got %q", got)
	}
}
