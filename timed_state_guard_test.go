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
	// Per root, not a single total: a walk that silently covered only one of
	// the two trees would still clear a combined floor.
	minFiles := map[string]int{"internal": 100, "modules": 10}
	parsedFiles := map[string]int{}
	for _, root := range []string{"internal", "modules"} {
		walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			parsedFiles[root]++
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					for _, bad := range forbiddenTimedStateIdents {
						if x.Name == bad {
							problems = append(problems, filepath.ToSlash(fset.Position(x.Pos()).String())+" spells "+bad)
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
						problems = append(problems, filepath.ToSlash(fset.Position(x.Pos()).String())+" declares a Duration+Magnitude struct outside internal/buffs; timed state is a buffs.Buff")
					}
				}
				return true
			})
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", root, walkErr)
		}
	}
	for _, root := range []string{"internal", "modules"} {
		if parsedFiles[root] < minFiles[root] {
			t.Fatalf("only parsed %d Go files under %s/ (want at least %d); the walk is not exercising the guard", parsedFiles[root], root, minFiles[root])
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d timed-state problem(s):\n  %s\n\nTimed state on a character is a buffs.Buff record read through Buffs.Effect; see internal/buffs/context.md.", len(problems), strings.Join(problems, "\n  "))
	}
}
