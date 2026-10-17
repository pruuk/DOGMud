package usercommands

import "testing"

// TestSkillNameWidthFitsTheLongestName feeds the skills template's name
// column (see internal/templates TestSkillsRankColumnLinesUp).
func TestSkillNameWidthFitsTheLongestName(t *testing.T) {
	if got := skillNameWidth(map[string]int{"search": 1}); got != 12 {
		t.Errorf("short names keep the 12-wide column, got %d", got)
	}
	if got := skillNameWidth(map[string]int{"search": 1, "unarmed-combat": 2, "blacksmithing": 1}); got != 14 {
		t.Errorf("the column must fit unarmed-combat, got %d", got)
	}
}
