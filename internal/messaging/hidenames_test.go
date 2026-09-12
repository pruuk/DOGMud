package messaging

import "testing"

func TestHideNames(t *testing.T) {
	const anon = `<ansi fg="combat-anon">`
	cases := []struct {
		name  string
		text  string
		names []string
		sight SightDecision
		want  string
	}{
		{
			name: "clear sight changes nothing", sight: SightFull,
			text: "You kick Bobrick!", names: []string{"Bobrick"},
			want: "You kick Bobrick!",
		},
		{
			name: "shapes, bare name mid-sentence", sight: SightShapes,
			text: "You kick Bobrick!", names: []string{"Bobrick"},
			want: "You kick " + anon + "a figure</ansi>!",
		},
		{
			name: "no sight, bare name at the start", sight: SightNone,
			text: "Bobrick kicks you!", names: []string{"Bobrick"},
			want: anon + "Something</ansi> kicks you!",
		},
		{
			name: "a whole identity tag goes with the name, possessive kept", sight: SightNone,
			text:  `<ansi fg="green"><ansi fg="username">Aliceia</ansi>'s Heal envelops you.</ansi>`,
			names: []string{"Aliceia"},
			want:  `<ansi fg="green">` + anon + `Something</ansi>'s Heal envelops you.</ansi>`,
		},
		{
			name: "a suffixed mob tag with a duplicate index goes whole", sight: SightShapes,
			text: `You hit <ansi fg="mobname-dup2">Rat #2</ansi>.`, names: []string{"Rat"},
			want: "You hit " + anon + "a figure</ansi>.",
		},
		{
			name: "capitalized after a sentence end, through tags", sight: SightNone,
			text:  `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> Kesh dodges and sweeps Bobrick to the ground!`,
			names: []string{"Kesh", "Bobrick"},
			want:  `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> ` + anon + "Something</ansi> dodges and sweeps " + anon + "something</ansi> to the ground!",
		},
		{
			name: "whole words only", sight: SightNone,
			text: "Keshara greets Kesh.", names: []string{"Kesh"},
			want: "Keshara greets " + anon + "something</ansi>.",
		},
		{
			name: "longest name first", sight: SightNone,
			text: "Kesh Vane waves.", names: []string{"Kesh", "Kesh Vane"},
			want: anon + "Something</ansi> waves.",
		},
		{
			name: "a name is never matched inside tag markup", sight: SightNone,
			text: `<ansi fg="green">Green hits green.</ansi>`, names: []string{"green"},
			want: `<ansi fg="green">Green hits ` + anon + "something</ansi>.</ansi>",
		},
		{
			name: "a later name cannot rewrite a replacement already made", sight: SightNone,
			text: "Kesh hits Bob.", names: []string{"Kesh", "anon", "ansi"},
			want: anon + "Something</ansi> hits Bob.",
		},
		{
			name: "underscore is part of a name", sight: SightNone,
			text: "Kesh_Two waves at Kesh.", names: []string{"Kesh"},
			want: "Kesh_Two waves at " + anon + "something</ansi>.",
		},
		{
			name: "a new line starts a sentence", sight: SightNone,
			text: "You fall.\nKesh laughs.", names: []string{"Kesh"},
			want: "You fall.\n" + anon + "Something</ansi> laughs.",
		},
		{
			name: "punctuation with no space is not a sentence start", sight: SightNone,
			text: `"Kesh!" Kesh laughs.`, names: []string{"Kesh"},
			want: `"` + anon + `something</ansi>!" ` + anon + "something</ansi> laughs.",
		},
		{
			name: "empty names are ignored", sight: SightNone,
			text: "Hello there.", names: []string{NoName, ""},
			want: "Hello there.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HideNames(tc.text, tc.names, tc.sight); got != tc.want {
				t.Fatalf("HideNames =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}

// A creature's name is printed in two forms: the raw authored name in a verb's
// own lines ("skeleton") and the display form inside an identity tag, which is
// title-cased and may carry a duplicate index ("Skeleton #2"). Both must be
// hidden from one Audience name. Matching loosely OUTSIDE a tag would hide
// ordinary words, so it stays exact there.
func TestHideNames_IdentityTagsMatchWhateverTheCase(t *testing.T) {
	const anon = "<ansi fg=\"combat-anon\">"
	cases := []struct {
		name  string
		text  string
		sight SightDecision
		want  string
	}{
		{
			name:  "the display form inside a tag",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> withstands your kick.",
			sight: SightNone,
			want:  anon + "Something</ansi> withstands your kick.",
		},
		{
			name:  "a duplicate index goes with it",
			text:  "You hit <ansi fg=\"mobname-dup2\">Skeleton #2</ansi>.",
			sight: SightShapes,
			want:  "You hit " + anon + "a figure</ansi>.",
		},
		{
			name:  "a bare word of the wrong case is left alone",
			text:  "Skeleton hits you.",
			sight: SightNone,
			want:  "Skeleton hits you.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HideNames(tc.text, []string{"skeleton"}, tc.sight); got != tc.want {
				t.Fatalf("HideNames =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}

// FormattedName.String prints a character's adjectives in a black-bold span
// right after the identity tag. Hiding the name and leaving "(dead)" or
// "(♥friend)" behind tells an unsighted reader what they could not see.
func TestHideNames_AdjectiveSpanGoesWithTheTag(t *testing.T) {
	const anon = "<ansi fg=\"combat-anon\">"
	cases := []struct {
		name  string
		text  string
		sight SightDecision
		want  string
	}{
		{
			name:  "one adjective",
			text:  "You recoil from striking <ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi>!",
			sight: SightNone,
			want:  "You recoil from striking " + anon + "something</ansi>!",
		},
		{
			name:  "adjectives after a duplicate index",
			text:  "<ansi fg=\"mobname-dup2\">Skeleton #2</ansi> <ansi fg=\"black-bold\">(♥friend|hidden)</ansi> recoils.",
			sight: SightShapes,
			want:  anon + "A figure</ansi> recoils.",
		},
		{
			name:  "a black-bold span that is not adjectives stays",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">hisses</ansi>.",
			sight: SightNone,
			want:  anon + "Something</ansi> <ansi fg=\"black-bold\">hisses</ansi>.",
		},
		{
			name:  "clear sight leaves everything",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi> recoils.",
			sight: SightFull,
			want:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi> recoils.",
		},
		{
			name:  "colour-patterned adjective, the production shape",
			text:  "You recoil from striking <ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(<ansi fg=\"52\">☠</ansi><ansi fg=\"88\">d</ansi><ansi fg=\"124\">e</ansi><ansi fg=\"160\">a</ansi><ansi fg=\"196\">d</ansi>)</ansi>!",
			sight: SightNone,
			want:  "You recoil from striking " + anon + "something</ansi>!",
		},
		{
			name:  "two adjectived names in one line",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(lost)</ansi> hits <ansi fg=\"mobname-dup2\">Skeleton #2</ansi> <ansi fg=\"black-bold\">(lost)</ansi>.",
			sight: SightNone,
			want:  anon + "Something</ansi> hits " + anon + "something</ansi>.",
		},
		{
			name:  "an identity tag nested inside the span cannot panic",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(<ansi fg=\"mobname\">Skeleton</ansi>)</ansi> hisses.",
			sight: SightNone,
			want:  anon + "Something</ansi> hisses.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HideNames(tc.text, []string{"skeleton"}, tc.sight); got != tc.want {
				t.Fatalf("HideNames =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}
