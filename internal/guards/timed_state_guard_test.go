package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Slice 1 of the conditions unification deleted the second collection of
// timed state (characters.CombatCondition). This guard keeps it deleted: no
// struct under internal/ or modules/ may declare a field named Duration,
// RoundsLeft, TriggersLeft, TriggerCount or RoundCounter alongside a
// Magnitude outside internal/conditions (a struct with those exact shapes is
// a copy of conditions.Condition itself, wherever it lives). Timed state is
// a conditions.Condition, read through Conditions.Effect; see
// internal/conditions/context.md.
//
// The deleted collection's own method names (HasCondition, AddCondition,
// RemoveCondition) used to sit in the forbidden-identifier list below too,
// but the ONE surviving primitive's API now uses those exact spellings
// (conditions.Conditions.HasCondition, and so on), so the plain
// spelling can no longer tell the deleted collection apart from the real
// one. The struct check above covers the same danger (a second
// Duration+Magnitude collection), so only the deleted type's own leftover
// names, never reused by the rename, stay forbidden by identifier.
var forbiddenTimedStateIdents = []string{"CombatCondition", "ConditionType", "TickConditions"}

// timedStateDurationFieldNames are every field name conditions.Condition (or
// its held Stack) uses to spell "how long is left": a struct outside
// internal/conditions that pairs one of these with a Magnitude field is a
// second copy of the primitive's own shape, wherever it is declared.
var timedStateDurationFieldNames = map[string]bool{
	"Duration": true, "RoundsLeft": true, "TriggersLeft": true,
	"TriggerCount": true, "RoundCounter": true,
}

// timedStateWrapperMethodRE matches a method name that spells the primitive's
// own verb set (AddCondition, AddConditionScaled, HasConditionFlag,
// RemoveCondition, RefreshCondition, TickConditions, CancelConditionsWithFlag,
// PruneConditions, and so on -- prefix, not exact, since the real API adds
// suffixes like Scaled/Magnitude/Flag/WithFlag). A method with a receiver
// spelling this pattern outside timedStateWrapperPackages is either a
// reintroduced rival primitive or a new wrapper nobody added to the allowlist
// -- either way, the guard should see it named, not let it pass silently.
var timedStateWrapperMethodRE = regexp.MustCompile(`^(Add|Has|Remove|Refresh|Tick|Cancel|Prune)Condition`)

// timedStateWrapperPackages: every package that declares a method matching
// timedStateWrapperMethodRE today (confirmed by
// `grep -rnE 'func \([^)]+\) (Add|Has|Remove|Refresh|Tick|Cancel|Prune)Condition[A-Za-z]*\(' internal modules`,
// non-test files, 2026-09-14): internal/conditions defines the primitive
// itself; internal/characters wraps it on Character (and drives the
// Awareness/Perception cascades a bare Conditions.AddCondition cannot see);
// internal/users and internal/mobs each wrap it once more for their own
// holder type (UserRecord, Mob), matching the event-path source-string
// signature the apply-path guard (condition_apply_path_guard_test.go) pins;
// internal/actions wraps it again on UserActor/MobActor so combat code can
// call either holder through one Actor interface. Add a new package here
// only after confirming with the same grep why it needs its own wrapper.
var timedStateWrapperPackages = []string{
	filepath.Join("internal", "conditions"),
	filepath.Join("internal", "characters"),
	filepath.Join("internal", "users"),
	filepath.Join("internal", "mobs"),
	filepath.Join("internal", "actions"),
}

func timedStateInWrapperPackage(path string) bool {
	for _, p := range timedStateWrapperPackages {
		if strings.HasPrefix(path, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// timedStateReceiverTypeName returns the method's receiver type name (stripped
// of a leading "*"), or "" if fd is not a method.
func timedStateReceiverTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

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
				if os.IsNotExist(err) {
					// A test elsewhere can create and remove a temp file under
					// the tree while packages test in parallel; a vanished
					// entry has nothing to scan.
					return nil
				}
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
			slashPath := filepath.ToSlash(path)
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
							if timedStateDurationFieldNames[nm.Name] {
								hasDuration = true
							}
							if nm.Name == "Magnitude" {
								hasMagnitude = true
							}
						}
					}
					if hasDuration && hasMagnitude && !strings.HasPrefix(slashPath, "internal/conditions/") {
						problems = append(problems, filepath.ToSlash(fset.Position(x.Pos()).String())+" declares a Duration+Magnitude struct outside internal/conditions; timed state is a conditions.Condition")
					}
				case *ast.FuncDecl:
					recvType := timedStateReceiverTypeName(x)
					if recvType != "" && timedStateWrapperMethodRE.MatchString(x.Name.Name) && !timedStateInWrapperPackage(path) {
						problems = append(problems, filepath.ToSlash(fset.Position(x.Pos()).String())+" declares "+recvType+"."+x.Name.Name+
							", a second wrapper around the timed-state primitive outside the allowlisted packages; route through conditions.Conditions or add the package to timedStateWrapperPackages with a reason")
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
		t.Fatalf("%d timed-state problem(s):\n  %s\n\nTimed state on a character is a conditions.Condition record read through Conditions.Effect; see internal/conditions/context.md.", len(problems), strings.Join(problems, "\n  "))
	}
}
