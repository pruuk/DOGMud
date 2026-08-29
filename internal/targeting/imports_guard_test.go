package targeting

import (
	"go/build"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	modulePrefix = "github.com/GoMudEngine/GoMud/"
	targetingPkg = modulePrefix + "internal/targeting"
	forbiddenPkg = modulePrefix + "internal/combat"
)

// TestTargetingDoesNotImportCombat pins layering decision 1 from the U12a
// plan. internal/combat contains a Commit call site (combat.go:409) that U12b
// migrates onto this package. If targeting ever imports combat -- directly or
// through another internal package -- that migration becomes an import cycle
// and U12b stalls.
//
// The weakest-mob strategy gets its score function by injection instead; see
// SetPowerScoreFn.
//
// Implemented on stdlib go/build rather than golang.org/x/tools/go/packages so
// this guard costs the module no new dependency. It walks internal/ packages
// transitively, which is where a cycle could actually form.
func TestTargetingDoesNotImportCombat(t *testing.T) {
	seen := map[string]bool{}
	var path []string

	var walk func(pkg string, trail []string) []string
	walk = func(pkg string, trail []string) []string {
		if seen[pkg] {
			return nil
		}
		seen[pkg] = true

		p, err := build.Import(pkg, ".", 0)
		if err != nil {
			// A package that will not resolve cannot contribute a cycle.
			return nil
		}
		for _, imp := range p.Imports {
			if !strings.HasPrefix(imp, modulePrefix) {
				continue
			}
			next := append(append([]string{}, trail...), imp)
			// Prefix match, not equality: internal/combat has no sub-packages
			// today, but one added later would slip past an exact-string
			// check and reintroduce the cycle silently.
			if imp == forbiddenPkg || strings.HasPrefix(imp, forbiddenPkg+"/") {
				return next
			}
			if found := walk(imp, next); found != nil {
				return found
			}
		}
		return nil
	}

	path = walk(targetingPkg, []string{targetingPkg})

	assert.Nil(t, path,
		"internal/targeting must not depend on internal/combat; path was: %s",
		strings.Join(path, " -> "))
}
