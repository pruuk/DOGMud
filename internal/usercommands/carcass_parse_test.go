package usercommands

import "testing"

// Review fix: "harvest roe deer" names the deer and lists it; "harvest wolf
// fang" names a part on the wolf.
func TestPhraseNamesCorpse(t *testing.T) {
	cases := []struct {
		phrase, name string
		want         bool
	}{
		{"roe deer", "Roe Deer", true},
		{"deer", "Roe Deer", true},
		{"steppe wolf corpse", "Steppe Wolf", true},
		{"wolf fang", "Steppe Wolf", false},
		{"deer hide", "Roe Deer", false},
		{"", "Roe Deer", false},
	}
	for _, c := range cases {
		if got := phraseNamesCorpse(c.phrase, c.name); got != c.want {
			t.Errorf("phraseNamesCorpse(%q, %q) = %v, want %v", c.phrase, c.name, got, c.want)
		}
	}
}
