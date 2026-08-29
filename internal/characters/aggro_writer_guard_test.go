package characters_test

// U12b sweep guard. Outside internal/characters and internal/targeting,
// nothing may write a combat target directly: every engagement goes through
// targeting.Commit / CommitAfter / CommitTaunt / Release.
//
// The allowlist is keyed by FILE and shrinks to empty as the sweep proceeds.
// It is deliberately not keyed by package: a package-level entry would hide a
// newly-added direct write inside an already-listed package, which is the
// failure mode contest_site_guard_test.go was written to avoid.
//
// internal/characters is exempt because it OWNS the storage: (*Character).Charm
// clears aggro internally and cannot import targeting (targeting imports
// characters). internal/targeting is exempt because Commit and Release are
// implemented in terms of these methods.

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

// notYetSwept is EMPTY, and it must stay empty. U12b swept all 88 sites. A new
// entry here is not a migration aid any more, it is a hole in the seam: make
// the call through internal/targeting instead.
//
// It survives as an empty map rather than being deleted so that a future slice
// with its own staged migration has the mechanism ready, and so the guard's
// stale-entry check keeps proving the list means something.
var notYetSwept = map[string]bool{}

func internalDirForSweepGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file")
	return filepath.Dir(filepath.Dir(thisFile))
}

// directAggroWriters walks internal/ and returns file -> positions of every
// direct SetAggro/EndAggro reference outside the two exempt packages.
func directAggroWriters(t *testing.T) map[string][]string {
	t.Helper()
	internalDir := internalDirForSweepGuard(t)
	offenders := map[string][]string{}

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

		// The two packages that legitimately touch the storage.
		if strings.HasPrefix(rel, "characters/") || strings.HasPrefix(rel, "targeting/") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "SetAggro" || sel.Sel.Name == "EndAggro" {
				offenders[rel] = append(offenders[rel], fset.Position(sel.Pos()).String())
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return offenders
}

func TestNoDirectAggroWritesOutsideTheSeam(t *testing.T) {
	offenders := directAggroWriters(t)

	var unexpected []string
	for file := range offenders {
		if !notYetSwept[file] {
			unexpected = append(unexpected, file)
		}
	}
	sort.Strings(unexpected)

	assert.Empty(t, unexpected,
		"these files write a combat target directly and are not on the U12b "+
			"allowlist. Use targeting.Commit / CommitAfter / CommitTaunt / "+
			"Release instead: %s", strings.Join(unexpected, ", "))

	// The allowlist must not rot: an entry naming a file that no longer has a
	// direct write is a stale entry hiding nothing, and it must be deleted so
	// the list keeps meaning something.
	var stale []string
	for file := range notYetSwept {
		if len(offenders[file]) == 0 {
			stale = append(stale, file)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale,
		"these files are on the U12b allowlist but no longer write aggro "+
			"directly. Delete their entries from notYetSwept: %s",
		strings.Join(stale, ", "))
}

// TestSweepGuardIsNotVacuous proves the guard is still capable of failing.
//
// Now that the sweep is complete the walk correctly finds NOTHING, so "it
// found something" can no longer be the evidence — that is exactly when a
// silently-broken walker would start passing forever. Two independent checks
// replace it: the walk must actually visit the tree, and the matcher must
// detect a real call.
func TestSweepGuardIsNotVacuous(t *testing.T) {
	internalDir := internalDirForSweepGuard(t)

	scanned := 0
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			scanned++
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, scanned, 100,
		"the walk visited only %d production files; internal/ is far larger "+
			"than that, so the walker is broken and the guard protects nothing",
		scanned)

	// And the matcher must fire on a real call.
	fset := token.NewFileSet()
	parsed, parseErr := parser.ParseFile(fset, "probe.go",
		"package p\nfunc f(c C) { c.EndAggro() }\n", 0)
	require.NoError(t, parseErr)

	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok &&
			(sel.Sel.Name == "SetAggro" || sel.Sel.Name == "EndAggro") {
			found = true
		}
		return true
	})
	require.True(t, found, "the SetAggro/EndAggro matcher must detect a real call")
}
