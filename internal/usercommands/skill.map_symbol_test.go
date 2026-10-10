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
