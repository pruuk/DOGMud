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
