package util

import "testing"

// FindMatchIndexIn is FindMatchIn by position (baubles slice D): the same
// algorithm, so a shop's two rows with one name are told apart by index and
// `buy 2.trinket` takes the second. The cases are TestFindMatchIn's.
func TestFindMatchIndexIn_SameAnswersAsFindMatchIn(t *testing.T) {
	items := []string{"SWORD", "SHINING SWORD", "SHIELD", "BIG HELM", "HELMET", "GEM"}
	cases := []struct {
		search          string
		match, closeIdx int
	}{
		{"", -1, -1},
		{"SWORD", 0, 0},
		{"sword#2", -1, 1},
		{"HELM", -1, 4},
		{"G", -1, 5},
		{"iel", -1, 2},
		{"helm#2", -1, 3}, // #865: next distinct item (was HELMET again)
	}
	for _, tc := range cases {
		m, c := FindMatchIndexIn(tc.search, items...)
		if m != tc.match || c != tc.closeIdx {
			t.Errorf("FindMatchIndexIn(%q) = (%d, %d), want (%d, %d)", tc.search, m, c, tc.match, tc.closeIdx)
		}
		wantM, wantC := "", ""
		if tc.match >= 0 {
			wantM = items[tc.match]
		}
		if tc.closeIdx >= 0 {
			wantC = items[tc.closeIdx]
		}
		if gm, gc := FindMatchIn(tc.search, items...); gm != wantM || gc != wantC {
			t.Errorf("FindMatchIn(%q) = (%q, %q), want (%q, %q): the two must agree", tc.search, gm, gc, wantM, wantC)
		}
	}
}

// #865: `buy pot` at Trader Onna took the Small Red Potion because the
// contains fallback took the first candidate. Fallback candidates are ranked:
// whole word, then word start, then mid-word; ties keep list order.
func TestFindMatchIndexIn_FallbackRanking(t *testing.T) {
	onna := []string{"Small Red Potion", "Rusty Pot"} // Onna's real item names
	cases := []struct {
		name            string
		search          string
		items           []string
		match, closeIdx int
	}{
		// The bug: whole word beats a word that merely starts with it.
		{"onna pot", "pot", onna, -1, 1},
		{"onna 2.pot", "2.pot", onna, -1, 0},
		{"word start beats mid-word", "pot", []string{"Spotted Cloak", "Small Red Potion"}, -1, 1},
		{"whole word beats mid-word", "pot", []string{"Spotted Cloak", "Rusty Pot"}, -1, 1},
		{"tie keeps list order", "pot", []string{"Red Potion", "Blue Potion"}, -1, 0},
		{"tie keeps list order 2.", "2.pot", []string{"Red Potion", "Blue Potion"}, -1, 1},
		{"whole word, middle of name", "pot", []string{"Small Red Potion", "Big Pot Helm"}, -1, 1},
		// Must not change: a lone potion is still found.
		{"single substring hit", "pot", []string{"Small Red Potion", "Iron Sword"}, -1, 0},
		{"mid-word only", "ott", []string{"Iron Sword", "Spotted Cloak"}, -1, 1},
		{"no hit", "zzz", onna, -1, -1},
		// Must not change: exact and starts-with come before the fallback.
		{"exact", "rusty pot", onna, 1, 1},
		{"exact beats earlier prefix", "pot", []string{"Potion", "Pot"}, 1, 0},
		{"prefix beats whole-word later", "pot", []string{"Potion", "Rusty Pot"}, -1, 0},
		// Numbered targets walk one order: starts-with hits, then ranked contains.
		{"2.pot reaches Rusty Pot after Potion", "2.pot", []string{"Potion", "Rusty Pot"}, -1, 1},
		{"1.pot is Potion", "pot", []string{"Potion", "Rusty Pot"}, -1, 0},
		{"3.pot is none", "3.pot", []string{"Potion", "Rusty Pot"}, -1, -1},
		{"apostrophe removed: word start", "edvar", []string{"Spotted Edvar", "Edvar's Ring"}, -1, 1},
		{"number prefix on prefix", "2.pot", []string{"Potion", "Pot", "Potato"}, -1, 1},
	}
	for _, tc := range cases {
		m, c := FindMatchIndexIn(tc.search, tc.items...)
		if m != tc.match || c != tc.closeIdx {
			t.Errorf("%s: FindMatchIndexIn(%q, %v) = (%d, %d), want (%d, %d)", tc.name, tc.search, tc.items, m, c, tc.match, tc.closeIdx)
		}
	}
}

// Two entries with one name: the index says which one.
func TestFindMatchIndexIn_TellsSameNamesApart(t *testing.T) {
	names := []string{"Iron Sword", "Trinket", "Trinket"}
	for search, want := range map[string][2]int{
		"trinket":   {1, 1},
		"2.trinket": {2, 2},
		"trinket#2": {2, 2},
		"3.trinket": {-1, -1},
		"trink":     {-1, 1},
		"2.trink":   {-1, 2},
	} {
		m, c := FindMatchIndexIn(search, names...)
		if m != want[0] || c != want[1] {
			t.Errorf("FindMatchIndexIn(%q) = (%d, %d), want (%d, %d)", search, m, c, want[0], want[1])
		}
	}
}
