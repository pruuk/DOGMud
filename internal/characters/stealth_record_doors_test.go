package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Record 9 drives a Visible character into Hidden whichever door adds it
// (sight gates playtest fixes, F5). AddCondition and AddConditionScaled
// called hideForStealthRecord and AddConditionMagnitude did not, so a 9
// added through the combat door left IsHidden false while the record
// claimed hidden.
func TestStealthRecord_EveryAddDoorHides(t *testing.T) {
	doors := map[string]func(c *Character) error{
		"AddCondition":          func(c *Character) error { return c.AddCondition(9, false) },
		"AddConditionScaled":    func(c *Character) error { return c.AddConditionScaled(9, 1.0) },
		"AddConditionMagnitude": func(c *Character) error { return c.AddConditionMagnitude(9, 0, 0, "test") },
	}
	for name, add := range doors {
		t.Run(name, func(t *testing.T) {
			restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
				9: {ConditionId: 9, Name: "Hidden",
					Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
			})
			defer restore()

			c := New()
			c.Awareness = awareness.NewMachine()
			if c.IsHidden() {
				t.Fatal("precondition: a fresh character is visible")
			}
			if err := add(c); err != nil {
				t.Fatalf("adding record 9 failed: %v", err)
			}
			if !c.IsHidden() {
				t.Errorf("%s added record 9 but left the character visible", name)
			}
		})
	}
}
