package dialogue

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Content writes a quest flag as "<questId>-<key>" (setsQuestFlag:
// {key: "74-allegiance", ...}), and the boot check resolves it through
// quests.ValidateFlag. The validators' world must accept the same spelling,
// or every correct flag reference is a problem. Ported from the deleted
// RegistryValidators test (#375 Task 21).
func TestRegistryWorld_FlagDeclaredUsesTheBootSpelling(t *testing.T) {
	quests.RegisterFlags(99001, []quests.QuestFlagDef{{Key: "path", Values: []string{"left", "right"}}})

	w, _ := RegistryWorld(nil) // file problems are not under test here
	if w.FlagDeclared == nil {
		t.Fatal("RegistryWorld must wire FlagDeclared")
	}
	if !w.FlagDeclared("99001-path", "left") {
		t.Error(`FlagDeclared("99001-path", "left") = false; the spelling content and boot use must pass`)
	}
	if w.FlagDeclared("99001-path", "up") {
		t.Error(`FlagDeclared("99001-path", "up") = true; an undeclared value must fail`)
	}
	if w.FlagDeclared("99002-path", "left") {
		t.Error(`FlagDeclared("99002-path", "left") = true; another quest's flag must fail`)
	}
}
