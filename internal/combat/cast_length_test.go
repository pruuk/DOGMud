package combat

import "testing"

// Bands follow the shipped spread, not round numbers: base_folds runs 2 to 36
// across the spell list and clusters hard at 4 to 8, so a linear scale would
// call almost everything the same thing.
func TestGetCastLengthDescription(t *testing.T) {
	for _, tc := range []struct {
		folds int
		want  string
	}{
		{2, "a moment"}, // the fastest spells in the list
		{3, "a moment"},
		{4, "brief"}, // the biggest cluster
		{6, "brief"},
		{8, "sustained"},
		{12, "sustained"},
		{16, "long"},
		{20, "long"},
		{24, "very long"},
		{36, "very long"}, // charm, the longest channel in the game
	} {
		if got := GetCastLengthDescription(tc.folds); got != tc.want {
			t.Errorf("GetCastLengthDescription(%d) = %q, want %q", tc.folds, got, tc.want)
		}
	}
}

// Charm must read as the longest thing available, or the helpfile's "long
// channel, requires deep concentration" is contradicted by its own label row.
func TestGetCastLengthDescription_CharmIsTheLongestBand(t *testing.T) {
	charm := GetCastLengthDescription(36)
	nextLongest := GetCastLengthDescription(28)
	if charm != "very long" {
		t.Errorf("charm (36 folds) reads %q, want the top band", charm)
	}
	if charm != nextLongest {
		t.Logf("note: 28 folds reads %q, charm %q", nextLongest, charm)
	}
	if GetCastLengthDescription(2) == charm {
		t.Error("the fastest and slowest spells in the game read identically, " +
			"so the row tells the player nothing")
	}
}
