package messaging

import "testing"

// #246: the combat templates write the possessive INSIDE the name tag, so a
// rendered line reads <ansi fg="username">Calabe's</ansi> (about 275 lines
// across _datafiles/world/*/combat-messages). Anonymize replaced the whole tag body,
// so a shapes reader read "A figure Iron Longsword delivers..." with the 's
// gone. The possessive belongs to the sentence, not the name: keep it.
func TestAnonymizeKeepsAPossessiveInsideTheTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"username at the start",
			`<ansi fg="username">Calabe's</ansi> <ansi fg="item">Iron Longsword</ansi> lands`,
			`<ansi fg="combat-anon">A figure</ansi>'s <ansi fg="item">Iron Longsword</ansi> lands`,
		},
		{
			"mobname mid sentence",
			`You dodge <ansi fg="mobname">Thornwall Thug's</ansi> swing`,
			`You dodge <ansi fg="combat-anon">a figure</ansi>'s swing`,
		},
		{
			"curly apostrophe",
			`<ansi fg="mobname-dup2">Thug #2’s</ansi> blade`,
			`<ansi fg="combat-anon">A figure</ansi>’s blade`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Anonymize(tc.in); got != tc.want {
				t.Fatalf("possessive lost or mangled:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// A name that merely ends in s keeps no stray apostrophe, and a possessive
// written OUTSIDE the tag (the other template shape) is untouched.
func TestAnonymizeNonPossessiveShapesUnchanged(t *testing.T) {
	tests := []struct{ in, want string }{
		{`<ansi fg="mobname">Silas</ansi> snarls`, `<ansi fg="combat-anon">A figure</ansi> snarls`},
		{`<ansi fg="username">Calabe</ansi>'s blade`, `<ansi fg="combat-anon">A figure</ansi>'s blade`},
	}
	for _, tc := range tests {
		if got := Anonymize(tc.in); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
