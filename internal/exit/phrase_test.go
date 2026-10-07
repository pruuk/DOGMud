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
