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

// TestRetargetOrEnd_ReportsWhetherTheCommitLanded is the U12c-0b regression.
//
// RetargetOrEnd releases FIRST and then commits. Once U12c-0b made a vetoed
// transition refuse the write, a refused commit left Aggro nil while the
// function still returned a bare `true`. Both callers in NewRound_DoCombat.go
// dereference char.Aggro on the strength of that true, so a vetoed retarget
// was a nil-pointer panic — one that aborts the ENTIRE round's combat
// processing for every actor, surviving only because the event listener
// recovers.
//
// This is a source-level guard rather than a behavioural one on purpose:
// driving a veto through RetargetOrEnd needs a loaded room, a mob registry, a
// live event queue and a wired veto set, while the regression itself is the
// literal token `return true` sitting after a targeting.Commit call.
func TestRetargetOrEnd_ReportsWhetherTheCommitLanded(t *testing.T) {
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

	// Every return statement that FOLLOWS a targeting.Commit call must report
	// whether the commit landed, never a bare `true`.
	commits := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			expr, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := expr.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "targeting" || sel.Sel.Name != "Commit" {
				continue
			}
			commits++

			require.Less(t, i+1, len(block.List),
				"a targeting.Commit at %s is the last statement in its block; "+
					"it must be followed by a return reporting whether it landed",
				fset.Position(call.Pos()))

			ret, ok := block.List[i+1].(*ast.ReturnStmt)
			require.True(t, ok,
				"the statement after targeting.Commit at %s must be a return",
				fset.Position(call.Pos()))
			require.Len(t, ret.Results, 1)

			if lit, isIdent := ret.Results[0].(*ast.Ident); isIdent {
				require.NotEqual(t, "true", lit.Name,
					"RetargetOrEnd returns a bare `true` after targeting.Commit at %s. "+
						"Commit is void and can be REFUSED by a combat-phase veto, and "+
						"this function released first — so a refused commit leaves Aggro "+
						"nil and both callers in NewRound_DoCombat.go panic dereferencing "+
						"it. Return `char.Aggro != nil` instead.",
					fset.Position(call.Pos()))
			}
		}
		return true
	})

	require.Equal(t, 3, commits,
		"expected 3 targeting.Commit sites in RetargetOrEnd (player scan, mob "+
			"scan, charmed-owner scan); found %d — the guard's shape has drifted "+
			"from the function it protects", commits)
}
