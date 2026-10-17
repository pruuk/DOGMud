package dialogue

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Content writes a quest flag as "<questId>-<key>" (setsQuestFlag:
// {key: "74-allegiance", ...}), and the boot check resolves it through
// quests.ValidateFlag. The editor's validator must accept the same spelling,
// or every correct flag reference blocks a save.
func TestRegistryValidators_FlagDeclaredUsesTheBootSpelling(t *testing.T) {
	quests.RegisterFlags(99001, []quests.QuestFlagDef{{Key: "path", Values: []string{"left", "right"}}})

	v := RegistryValidators()
	if !v.FlagDeclared("99001-path", "left") {
		t.Error(`FlagDeclared("99001-path", "left") = false; the spelling content and boot use must pass`)
	}
	if v.FlagDeclared("99001-path", "up") {
		t.Error(`FlagDeclared("99001-path", "up") = true; an undeclared value must fail`)
	}
	if v.FlagDeclared("99002-path", "left") {
		t.Error(`FlagDeclared("99002-path", "left") = true; another quest's flag must fail`)
	}
}
