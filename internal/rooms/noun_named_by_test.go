package rooms

import "testing"

// TestNounNamedBy pins the room-noun resolution quest triggers use (#375
// batch 4 re-verify N1): every name the room accepts for a noun (its key, a
// data alias, a word of a multi-word name, a singular/plural spelling)
// resolves to the noun's key, and a phrase with a word that is no name of
// that noun (a sentence that only mentions it) resolves to nothing.
func TestNounNamedBy(t *testing.T) {
	r := &Room{
		Nouns: map[string]string{
			"floorboard":     "a proud plank",
			"board":          ":floorboard",
			"floor board":    ":floorboard",
			"old guild mark": "a faded mark",
			"stock ledger":   "a bound account book",
		},
	}
	tests := []struct {
		typed string
		want  string
	}{
		{"floorboard", "floorboard"},
		{"board", "floorboard"},
		{"floor board", "floorboard"},
		{"floorboards", "floorboard"},
		{"boards", "floorboard"},
		{"Floor Board", "floorboard"},
		{"guild mark", "old guild mark"},
		{"mark", "old guild mark"},
		{"old guild mark", "old guild mark"},
		{"ledger", "stock ledger"},
		// A sentence that mentions a noun names nothing.
		{"i found the board", ""},
		{"loose board", ""},
		{"gibberish", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := r.NounNamedBy(tt.typed); got != tt.want {
			t.Errorf("NounNamedBy(%q) = %q, want %q", tt.typed, got, tt.want)
		}
	}

	if got := (&Room{}).NounNamedBy("board"); got != "" {
		t.Errorf("room without nouns: NounNamedBy(board) = %q, want empty", got)
	}
}
