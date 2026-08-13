package auctions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAuctionsAreBankOnly fails if any production file in this module reads or
// writes Character.Gold.
//
// WHY THIS IS A TEST AND NOT A COMMENT. Auctions are bank-only by design: Bid
// checks and debits Character.Bank, refundUser refunds to Character.Bank, and
// the seller payout settles to Character.Bank. Every money path agrees except
// one, and that one shipped two real defects at once in the `auction bid`
// command handler:
//
//  1. A pre-check `if amt > user.Character.Gold` rejected the bid on CARRIED
//     gold while Bid debited the BANK. A player with 0 carried and 5000 banked
//     could never bid at all.
//  2. On success the handler ALSO ran `user.Character.Gold -= amt` on top of
//     Bid's `Character.Bank -= bid`, charging the winner twice out of two
//     different pools. Refunds only ever return to the bank, so the
//     carried-gold half was not escrowed anywhere -- it was destroyed.
//
// TestBid_EscrowsAndRefunds did not catch either one, and could not have: it
// exercises the AuctionManager, which was always correct. The defect lived in
// the thin command handler wrapped around it, which no unit test reaches. So
// the durable guard is the invariant itself, checked against the source.
//
// If a future auction feature genuinely needs carried gold -- an at-the-podium
// cash sale, say -- this test is the deliberate speed bump. Decide that money
// can move between the two pools, write down where the escrow and the refund
// each live, and only then add the file to bankOnlyExemptions.
func TestAuctionsAreBankOnly(t *testing.T) {
	// Files allowed to touch Character.Gold, with the reason each one needs it.
	// Empty by design: no auction path should reach carried gold today.
	bankOnlyExemptions := map[string]string{}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read module dir: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := bankOnlyExemptions[name]; ok {
			continue
		}

		file, perr := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ParseComments)
		if perr != nil {
			// A syntax error is the compiler's problem to report, not this
			// test's. Skipping keeps failures attributable.
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Gold" {
				return true
			}
			// Match `<anything>.Character.Gold`, so a local variable named Gold
			// or an unrelated struct field does not trip the guard.
			inner, ok := sel.X.(*ast.SelectorExpr)
			if !ok || inner.Sel.Name != "Character" {
				return true
			}
			offenders = append(offenders, name+":"+
				strconv.Itoa(fset.Position(sel.Pos()).Line))
			return true
		})
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("auctions touched Character.Gold at:\n  %s\n\n"+
			"Auctions are bank-only: Bid debits Character.Bank, refundUser refunds "+
			"to it, and the seller payout settles to it. Mixing carried gold in "+
			"has already caused a double charge that destroyed gold. If this is "+
			"deliberate, add the file to bankOnlyExemptions with a reason.",
			strings.Join(offenders, "\n  "))
	}
}
