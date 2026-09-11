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
