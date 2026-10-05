package characters

import "testing"

// A save from before the rename keeps its rank, uses and known recipe under
// the new names, and running the migration again changes nothing.
func TestCarpentryMigratesToWoodwork(t *testing.T) {
	c := &Character{
		Skills:        map[string]int{"carpentry": 7},
		SkillUseCount: map[string]int{"carpentry": 12},
		KnownRecipes:  map[string]int{"carpenters-workbench": 1},
	}
	c.validateSkillMigrations()
	if c.Skills["woodwork"] != 7 || c.SkillUseCount["woodwork"] != 12 {
		t.Errorf("woodwork rank %d uses %d, want 7 and 12", c.Skills["woodwork"], c.SkillUseCount["woodwork"])
	}
	if _, ok := c.Skills["carpentry"]; ok {
		t.Error("the carpentry key should be gone")
	}
	if _, ok := c.KnownRecipes["woodworking-bench"]; !ok {
		t.Error("the known workbench recipe should carry over")
	}
	if _, ok := c.KnownRecipes["carpenters-workbench"]; ok {
		t.Error("the old recipe id should be gone")
	}
	c.validateSkillMigrations()
	if c.Skills["woodwork"] != 7 || c.SkillUseCount["woodwork"] != 12 {
		t.Errorf("a second run changed the rank or uses: %d, %d", c.Skills["woodwork"], c.SkillUseCount["woodwork"])
	}
}
