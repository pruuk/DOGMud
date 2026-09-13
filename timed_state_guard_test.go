package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Slice 1 of the conditions unification deleted the second collection of
// timed state (characters.CombatCondition). This guard keeps it deleted: no
// struct under internal/ or modules/ may declare a field named Duration or
// RoundsLeft alongside a Magnitude outside internal/buffs, and no identifier
// may spell HasCondition, AddCondition or CombatCondition. Timed state is a
// buffs.Buff, read through Buffs.Effect; see internal/buffs/context.md.
var forbiddenTimedStateIdents = []string{"HasCondition", "AddCondition", "RemoveCondition", "CombatCondition", "ConditionType", "TickConditions"}

func TestNoSecondTimedStateCollection(t *testing.T) {
	fset := token.NewFileSet()
	var problems []string
	parsedFiles := 0
	for _, root := range []string{"internal", "modules"} {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			parsedFiles++
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					for _, bad := range forbiddenTimedStateIdents {
						if x.Name == bad {
							problems = append(problems, filepath.ToSlash(path)+": "+fset.Position(x.Pos()).String()+" spells "+bad)
						}
					}
				case *ast.StructType:
					hasDuration, hasMagnitude := false, false
					for _, fld := range x.Fields.List {
						for _, nm := range fld.Names {
							if nm.Name == "Duration" || nm.Name == "RoundsLeft" {
								hasDuration = true
							}
							if nm.Name == "Magnitude" {
								hasMagnitude = true
							}
						}
					}
					if hasDuration && hasMagnitude && !strings.HasPrefix(filepath.ToSlash(path), "internal/buffs/") {
						problems = append(problems, filepath.ToSlash(path)+": "+fset.Position(x.Pos()).String()+" declares a Duration+Magnitude struct outside internal/buffs; timed state is a buffs.Buff")
					}
				}
				return true
			})
			return nil
		})
	}
	if parsedFiles < 100 {
		t.Fatalf("only parsed %d Go files under internal/ and modules/; the walk is not exercising the guard", parsedFiles)
	}
	if len(problems) > 0 {
		t.Fatalf("%d timed-state problem(s):\n  %s\n\nTimed state on a character is a buffs.Buff record read through Buffs.Effect; see internal/buffs/context.md.", len(problems), strings.Join(problems, "\n  "))
	}
}
