package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// #253: the mob map symbol must survive ASCII conversion, or the cell
// vanishes and the row's frame shifts.
func TestMobMapSymbolSurvivesAscii(t *testing.T) {
	if got := mobMapSymbol(false); got != '☠' {
		t.Errorf("utf-8 symbol = %q, want ☠", got)
	}
	sym := mobMapSymbol(true)
	if sym >= 0x80 {
		t.Fatalf("ascii symbol %q is not ASCII", sym)
	}
	if got := util.ConvertToAscii(string(sym)); got != string(sym) {
		t.Errorf("ascii symbol changed by conversion: %q", got)
	}
}

// #253 follow-up: the markers the map command draws convert to one ASCII
// character each, so an ASCII map cell keeps its width.
func TestMapCodeMarkersConvertToOneAsciiCharacter(t *testing.T) {
	for name, r := range map[string]rune{
		"you": '@', "player, npc, party member": '☺', "friend": '☹', "mob": mobMapSymbol(true),
	} {
		got := util.ConvertToAscii(string(r))
		if len(got) != 1 || got[0] >= 0x80 {
			t.Errorf("%s: %q converts to %q, not one ASCII character", name, r, got)
		}
	}
}
