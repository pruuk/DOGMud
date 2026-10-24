package guards

import (
	"os"
	"regexp"
	"testing"
)

// TestDrinkWrappersDoNotReFork: the drink rules live in actions.Drink. If a
// command wrapper applies a condition, toxicity, an item use, or a mutation
// change itself, the player and mob paths have forked again (the mob path
// was a 49-line copy missing toxicity, potency and every special potion
// until 2026-09-28).
func TestDrinkWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`AddCondition|AddToxicity|UseItem|ScourMutations|AddBloomAddiction|GrantRandomMutation|BloomSeedNewMutation|BloomAdvanceMutation|SetTickAmount`)
	for _, path := range []string{"internal/usercommands/drink.go", "internal/mobcommands/drink.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s applies drink rules itself (%q); call actions.Drink instead", path, b[loc[0]:loc[1]])
		}
	}
}
