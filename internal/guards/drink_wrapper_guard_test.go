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

// TestFoodWrappersDoNotReFork: the food rules (the lookup order, the busy and
// grapple gates, spoilage, the room line) live in actions.ConsumeFood, for
// players and mobs alike. The two paths forked once already: the mob body had
// no edible-first pass, no bandolier and no busy gate until #375 phase 7.
func TestFoodWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`AddCondition|UseItem|FindIn|IsActing|IsGrappling|IsSpoiledFood|SendText`)
	for _, path := range []string{
		"internal/usercommands/eat.go",
		"internal/usercommands/drink.go",
		"internal/mobcommands/eat.go",
		"internal/mobcommands/drink.go",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(b), "actions.ConsumeFood(") {
			t.Errorf("%s does not call actions.ConsumeFood", path)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s applies food rules itself (%q); call actions.ConsumeFood instead", path, b[loc[0]:loc[1]])
		}
	}
}
