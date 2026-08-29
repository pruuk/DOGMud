package characters_test

// U12c-2 Task 1. Nothing outside internal/characters may CONSTRUCT an Aggro.
//
// U12b was contracted to land all 90 write sites on the seam and this one did
// not land: hooks/NewRound_DoCombat_unified.go built a targetless
// &characters.Aggro{Type: DefaultAttack} for an MvM defender. targeting.Commit
// refuses a zero ref, so no seam call reproduces it -- which is why U12c-1 left
// it alone rather than smuggle a behaviour change into a mechanical slice.
//
// It blocks deleting the field, so it is the first thing U12c-2 removes.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNothingOutsideCharactersConstructsAnAggro(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	internalDir := filepath.Dir(filepath.Dir(thisFile))

	filesWalked := 0
	var offenders []string
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(internalDir, path)
		require.NoError(t, relErr)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "characters/") {
			return nil
		}
		filesWalked++

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Aggro" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "characters" {
				offenders = append(offenders, fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)

	// A walk that matched no files and a genuinely clean tree both produce an
	// empty offender list. Prove the search could have succeeded before
	// believing it did.
	require.Greater(t, filesWalked, 100,
		"the walk only reached %d files, so an empty result proves nothing",
		filesWalked)

	sort.Strings(offenders)
	assert.Empty(t, offenders,
		"these sites construct a characters.Aggro directly. Use the "+
			"targeting seam: %s", strings.Join(offenders, ", "))
}
