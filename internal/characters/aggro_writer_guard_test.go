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

// notYetSwept lists files still holding a direct write. DELETE entries as you
// migrate them. When this is empty, U12b is done.
var notYetSwept = map[string]bool{
	"hooks/Death_InboundAggroCleanup.go": true,
	"hooks/Death_MobKillCredit.go":       true,
	"hooks/NewRound_DoCombat.go":         true,
	"hooks/NewRound_DoCombat_helpers.go": true,
	"hooks/NewRound_DoCombat_unified.go": true,
	"hooks/NewRound_IdleMobs.go":         true,
	"hooks/NewRound_MobRoundTick.go":     true,
	"hooks/Respawn_PlayerTeleport.go":    true,
	"hooks/charm_spell.go":               true,
	"hooks/chrysifier_homunculus.go":     true,
	"hooks/combat_retarget.go":           true,
	"hooks/companion_follow.go":          true,
	"hooks/companion_summon.go":          true,
	"hooks/manifester_companions.go":     true,
}

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

// TestSweepGuardIsNotVacuous proves the walker actually finds writes. A guard
// whose walk silently matched nothing would pass forever and protect nothing,
// which is exactly how a stale path table goes unnoticed.
func TestSweepGuardIsNotVacuous(t *testing.T) {
	offenders := directAggroWriters(t)

	require.NotEmpty(t, offenders,
		"the walk found no direct aggro writes anywhere. Either U12b is "+
			"complete and this test should be deleted along with the "+
			"allowlist, or the walk is broken and the guard is protecting "+
			"nothing.")
}
