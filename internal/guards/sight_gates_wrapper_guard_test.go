package guards

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"testing"
)

// TestSightGateWrappersDoNotReFork: the 5a gates live in shared bodies
// (actions.TooDarkToGet, TakeFloorItem, ResolveLook, CursedHolds,
// RemoveEquipment, RemoveAllEquipment, TooDarkToCraft; Character.Wear,
// WearInArm, CursedRefusal, ChooseWornSlot). If a command wrapper evaluates
// sight, a curse, busyness or a slot itself, the player and mob paths have
// forked again. Comments are dropped before matching (the files name these
// words in comments), by printing each file's AST parsed without them.
func TestSightGateWrappersDoNotReFork(t *testing.T) {
	rows := []struct {
		files     []string
		forbidden *regexp.Regexp
		fix       string
	}{
		{[]string{"internal/usercommands/get.go", "internal/mobcommands/get.go"},
			regexp.MustCompile("ParticipantSight|CanSeeShapes|CanSeeClearly|`exploding`|\"exploding\"|SightNone|HasAdjective"),
			"ask actions.TooDarkToGet or go through actions.TakeFloorItem"},
		{[]string{"internal/usercommands/look.go", "internal/mobcommands/look.go"},
			regexp.MustCompile(`ParticipantSight|SeesThroughExit|CanSeeClearly|ResolveTargetActor|CanSeeShapes|FindByPetName`),
			"go through actions.ResolveLook"},
		{[]string{"internal/usercommands/remove.go", "internal/mobcommands/remove.go"},
			regexp.MustCompile(`IsCursed|IsActing|refuseWhileBusy|Spellcasting`),
			"go through actions.RemoveEquipment / RemoveAllEquipment"},
		{[]string{"internal/usercommands/equip.go", "internal/mobcommands/equip.go", "internal/usercommands/gearup.go", "internal/mobcommands/gearup.go"},
			regexp.MustCompile(`IsCursed|Spellcasting|CursedRefusal|ChooseWornSlot|\.Wear\(`),
			"go through actions.EquipItem / EquipItemInArm"},
		{[]string{"internal/usercommands/equip.go"},
			regexp.MustCompile(`GetHandPairs|HandsRequired|ItemPtr`),
			"arm placement lives in Character.WearInArm"},
		{[]string{"internal/usercommands/craft.go", "internal/mobcommands/craft.go"},
			regexp.MustCompile(`CanSeeClearly|ParticipantSight|CanSeeShapes`),
			"ask actions.TooDarkToCraft"},
	}
	for _, row := range rows {
		for _, path := range row.files {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0) // no ParseComments: comments dropped
			if err != nil {
				t.Fatalf("parse %s (test must run from the repo root): %v", path, err)
			}
			var code bytes.Buffer
			if err := printer.Fprint(&code, fset, file); err != nil {
				t.Fatalf("print %s: %v", path, err)
			}
			if loc := row.forbidden.FindIndex(code.Bytes()); loc != nil {
				t.Errorf("%s applies a 5a gate itself (%q); %s", path, code.Bytes()[loc[0]:loc[1]], row.fix)
			}
		}
	}
}
