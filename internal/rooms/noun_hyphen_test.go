package rooms

import "testing"

// A hyphen matches a space in nouns, on both sides (#375): the room text
// shows a highlighted `nave-lamps` while the noun key is `nave lamps`.
func TestFindNoun_HyphenMatchesSpace(t *testing.T) {
	r := &Room{Nouns: map[string]string{
		"nave lamps":   "Lamps burn.",
		"lantern-hook": "A hook.",
		"bench-vise":   "A vise.",
		"floorboard":   "A plank.",
		"floor board":  ":floorboard",
	}}
	tests := []struct{ typed, want string }{
		{"nave-lamps", "nave lamps"},
		{"nave lamps", "nave lamps"},
		{"nave-lamp", "nave lamps"},
		{"nave lamp", "nave lamps"},
		{"lantern hook", "lantern-hook"},
		{"lantern-hook", "lantern-hook"},
		{"lantern-hooks", "lantern-hook"},
		{"bench-vise", "bench-vise"},
		{"bench vise", "bench-vise"},
		{"floor-board", "floorboard"},
	}
	for _, tt := range tests {
		got, desc := r.FindNoun(tt.typed)
		if got != tt.want || desc == "" {
			t.Errorf("FindNoun(%q) = %q (%q), want %q", tt.typed, got, desc, tt.want)
		}
	}
}

func TestFindNoun_ExactHyphenKeyWinsOverSpaceKey(t *testing.T) {
	r := &Room{Nouns: map[string]string{
		"bench-vise": "hyphen",
		"bench vise": "space",
	}}
	if k, _ := r.FindNoun("bench-vise"); k != "bench-vise" {
		t.Errorf("got %q want bench-vise", k)
	}
	if k, _ := r.FindNoun("bench vise"); k != "bench vise" {
		t.Errorf("got %q want bench vise", k)
	}
}

func TestNounNamedBy_HyphenMatchesSpace(t *testing.T) {
	r := &Room{Nouns: map[string]string{
		"nave lamps": "Lamps burn.",
		"bench-vise": "A vise.",
	}}
	tests := []struct{ typed, want string }{
		{"nave-lamps", "nave lamps"},
		{"nave-lamp", "nave lamps"},
		{"bench vise", "bench-vise"},
		{"bench-vise", "bench-vise"},
		{"found-the-lamps", ""},
	}
	for _, tt := range tests {
		if got := r.NounNamedBy(tt.typed); got != tt.want {
			t.Errorf("NounNamedBy(%q) = %q, want %q", tt.typed, got, tt.want)
		}
	}
}

func TestFindHiddenNoun_HyphenMatchesSpace(t *testing.T) {
	r := &Room{HiddenNouns: map[string]HiddenNoun{"secret door": {}, "wall-crack": {}}}
	for typed, want := range map[string]string{"secret-door": "secret door", "wall crack": "wall-crack"} {
		if k, _, ok := r.FindHiddenNoun(typed); !ok || k != want {
			t.Errorf("FindHiddenNoun(%q) = %q,%v want %q", typed, k, ok, want)
		}
	}
}
