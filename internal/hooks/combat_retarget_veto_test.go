package hooks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRetargetOrEnd_NeverClaimsSuccessWithoutEvidence is the U12c-0b
// regression.
//
// RetargetOrEnd releases FIRST and then commits. Once U12c-0b made a vetoed
// transition refuse the write, a refused commit left Aggro nil while the
// function still returned a bare `true`. Both callers in NewRound_DoCombat.go
// (`:74` and `:134`) dereference char.Aggro on the strength of that return, so
// a vetoed retarget was a nil-pointer panic — one that aborts the ENTIRE
// round's combat processing for every actor, surviving only because the event
// listener recovers.
//
// The guard is source-level rather than behavioural on purpose: driving a veto
// through RetargetOrEnd needs a loaded room, a mob registry, a live event queue
// and a wired veto set, while the regression itself is the literal token
// `return true`.
//
// It asserts the SHAPE-INDEPENDENT rule — no bare `true` may be returned from
// this function at all — so it keeps holding whether the result comes from
// `return targeting.Commit(...)`, from a local, or from anything else.
func TestRetargetOrEnd_NeverClaimsSuccessWithoutEvidence(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset,
		filepath.Join(filepath.Dir(thisFile), "combat_retarget.go"), nil, 0)
	require.NoError(t, err)

	var fn *ast.FuncDecl
	for _, d := range parsed.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "RetargetOrEnd" {
			fn = f
			break
		}
	}
	require.NotNil(t, fn, "RetargetOrEnd must exist for this guard to mean anything")

	commits := 0
	bareTrues := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok &&
					pkg.Name == "targeting" && sel.Sel.Name == "Commit" {
					commits++
				}
			}
		case *ast.ReturnStmt:
			if len(node.Results) != 1 {
				return true
			}
			if id, ok := node.Results[0].(*ast.Ident); ok && id.Name == "true" {
				bareTrues++
				t.Errorf("RetargetOrEnd returns a bare `true` at %s. It releases "+
					"aggro BEFORE committing, and targeting.Commit can be REFUSED "+
					"by a combat-phase veto, so a bare true leaves Aggro nil while "+
					"claiming success — and both callers in NewRound_DoCombat.go "+
					"dereference it. Return the Commit result instead.",
					fset.Position(node.Pos()))
			}
		}
		return true
	})

	require.Zero(t, bareTrues)
	require.Equal(t, 3, commits,
		"expected 3 targeting.Commit sites in RetargetOrEnd (player scan, mob "+
			"scan, charmed-owner scan); found %d — the guard's shape has drifted "+
			"from the function it protects", commits)
}
