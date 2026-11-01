package guards

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestQuaffWrappersDoNotReFork: the potion rules live in actions.Quaff. If a
// command wrapper applies a condition, toxicity, an item use, or a mutation
// change itself, the player and mob paths have forked again (the mob path
// was a 49-line copy missing toxicity, potency and every special potion
// until 2026-09-28). The wrappers are quaff.go since #375 phase 7; drink.go
// is the food command now (usercommands/eat.go).
func TestQuaffWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`AddCondition|AddToxicity|UseItem|ScourMutations|AddBloomAddiction|GrantRandomMutation|BloomSeedNewMutation|BloomAdvanceMutation|SetTickAmount`)
	for _, path := range []string{"internal/usercommands/quaff.go", "internal/mobcommands/quaff.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(b), "actions.Quaff(") {
			t.Errorf("%s does not call actions.Quaff", path)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s applies potion rules itself (%q); call actions.Quaff instead", path, b[loc[0]:loc[1]])
		}
	}
}
