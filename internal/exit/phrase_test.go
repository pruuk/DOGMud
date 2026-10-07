package exit

import "testing"

func TestPhrasesRenderUpAndDownNaturally(t *testing.T) {
	cases := []struct {
		name, from, toward, departure string
	}{
		{"up", "from above", "up", "upward"},
		{"down", "from below", "down", "downward"},
		{"Up", "from above", "up", "upward"},
		{"north", `from the <ansi fg="exit">north</ansi>`, "toward the north",
			`towards the <ansi fg="exit">north</ansi> exit`},
		{"trapdoor", `from the <ansi fg="exit">trapdoor</ansi>`, "toward the trapdoor",
			`towards the <ansi fg="exit">trapdoor</ansi> exit`},
	}
	for _, c := range cases {
		if got := FromPhrase(c.name); got != c.from {
			t.Errorf("FromPhrase(%q) = %q, want %q", c.name, got, c.from)
		}
		if got := TowardPhrase(c.name); got != c.toward {
			t.Errorf("TowardPhrase(%q) = %q, want %q", c.name, got, c.toward)
		}
		if got := DeparturePhrase(c.name); got != c.departure {
			t.Errorf("DeparturePhrase(%q) = %q, want %q", c.name, got, c.departure)
		}
	}
}

// Combat and shooting templates put the exit name after "the": "A shot from
// the {entrancename} finds you", "towards the {exitname} direction", "from
// beyond the {exitname}". PhraseVerticalExits rewrites those phrases once the
// tokens are filled, so up and down read as directions and every other exit
// is left exactly as rendered.
func TestPhraseVerticalExits(t *testing.T) {
	cases := []struct {
		line  string
		exits []string
		want  string
	}{
		{`A shot from the up finds you as you fall.`, []string{"up"},
			`A shot from above finds you as you fall.`},
		{`A shot from the north finds you as you fall.`, []string{"north"},
			`A shot from the north finds you as you fall.`},
		{`Orc fires at you from the <ansi fg="exit">down</ansi> direction.`, []string{"x", "down"},
			`Orc fires at you from below.`},
		{`Orc prepares to attack towards the <ansi fg="exit">up</ansi>.`, []string{"up"},
			`Orc prepares to attack upward.`},
		{`Orc prepares to attack towards the <ansi fg="exit">north</ansi>.`, []string{"north"},
			`Orc prepares to attack towards the <ansi fg="exit">north</ansi>.`},
		{`Orc glances toward the down direction.`, []string{"down"},
			`Orc glances downward.`},
		{`You aim through the <ansi fg="exit">down</ansi> direction, but miss.`, []string{"down"},
			`You aim downward, but miss.`},
		{`A shot streaks in from beyond the <ansi fg="exit">up</ansi> and strikes Bob!`, []string{"up"},
			`A shot streaks in from somewhere above and strikes Bob!`},
		{`Orc waits, eyes on the <ansi fg="exit">up</ansi>.`, []string{"up"},
			`Orc waits, eyes on the way up.`},
		{`Orc holds position, watching the down.`, []string{"down"},
			`Orc holds position, watching the way down.`},
		// Only a vertical name that was actually filled in is rewritten.
		{`A shot from the up finds you.`, []string{"north"},
			`A shot from the up finds you.`},
		{`Orc fires towards the upper deck.`, []string{"up"},
			`Orc fires towards the upper deck.`},
	}
	for _, c := range cases {
		if got := PhraseVerticalExits(c.line, c.exits...); got != c.want {
			t.Errorf("PhraseVerticalExits(%q, %v)\n got %q\nwant %q", c.line, c.exits, got, c.want)
		}
	}
}
