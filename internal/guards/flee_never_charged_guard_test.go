package guards

import (
	"os"
	"strings"
	"testing"
)

// Owner ruling 2026-11-16 (#401): a flee is never refused for lack of
// stamina. Flee pays its own stamina partially in actions.BeginFlee (a short
// fleer still runs, without the skill practice), and the escape moves the
// fleer without pricing a step. actions.ChargeMove and QuoteMove refuse an
// exhausted mover, so no flee path may call them.
func TestFleeNeverChargesAStep(t *testing.T) {
	files := []string{
		"internal/actions/flee.go",
		"internal/usercommands/flee.go",
		"internal/mobcommands/flee.go",
		"internal/hooks/NewRound_DoCombat_helpers.go",
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (a moved file must be listed here again)", f, err)
		}
		for _, call := range []string{"ChargeMove(", "QuoteMove(", "QuoteMobStep("} {
			if strings.Contains(string(raw), call) {
				t.Errorf("%s calls %s: a flee path must never price a step, or an exhausted fleer could be refused", f, call)
			}
		}
	}
}
