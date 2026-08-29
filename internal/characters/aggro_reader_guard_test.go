package characters_test

// U12c-1 read-migration guard. Outside internal/characters and
// internal/targeting, nothing may read the Aggro struct directly: use
// IsInCombat() and CurrentCombatTarget().
//
// The matcher reports EVERY `x.Aggro`, including the .Type, .RoundsWaiting and
// .SpellInfo fields that U12c-1 does NOT migrate. That is deliberate: those
// files stay on the allowlist until U12c-2 clears them, so this allowlist does
// not reach empty in this slice. U12c-1 is done when every remaining entry is
// there ONLY for a U12c-2-owned field.

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

// notYetMigrated lists files still reading Aggro directly. DELETE entries as
// you migrate them.
//
// Populated from this guard's OWN failure output, which is the only list
// guaranteed to match what the matcher sees. Do NOT regenerate it from a grep:
// grep matches comments, the AST parser (mode 0) drops them, so a grep-derived
// list carries phantom entries that then fail the stale check below.
var notYetMigrated = map[string]bool{
	"actions/cast.go":                       true,
	"actions/combat_drain.go":               true,
	"actions/combat_fire.go":                true,
	"actions/combat_gore.go":                true,
	"actions/combat_hamstring.go":           true,
	"actions/combat_helpers.go":             true,
	"actions/combat_maul.go":                true,
	"actions/combat_pounce.go":              true,
	"actions/combat_rake.go":                true,
	"actions/combat_rally.go":               true,
	"actions/combat_taunt.go":               true,
	"actions/combat_throttle.go":            true,
	"actions/combat_warcry.go":              true,
	"actions/command_readiness.go":          true,
	"actions/defuse.go":                     true,
	"actions/melee_target.go":               true,
	"actions/plant.go":                      true,
	"actions/shadow.go":                     true,
	"actions/sneak.go":                      true,
	"actions/steal.go":                      true,
	"behaviortree/actions_archer.go":        true,
	"behaviortree/actions_mob.go":           true,
	"behaviortree/actions_scout.go":         true,
	"behaviortree/conditions_mob.go":        true,
	"behaviortree/conditions_position.go":   true,
	"combat/combat.go":                      true,
	"combat/flee.go":                        true,
	"conversationadapter/adapter.go":        true,
	"hooks/combat_retarget.go":              true,
	"hooks/companion_follow.go":             true,
	"hooks/Death_AlivenessSubstrate.go":     true,
	"hooks/Death_InboundAggroCleanup.go":    true,
	"hooks/NewRound_DoCombat.go":            true,
	"hooks/NewRound_DoCombat_helpers.go":    true,
	"hooks/NewRound_DoCombat_resolution.go": true,
	"hooks/NewRound_DoCombat_unified.go":    true,
	"hooks/NewRound_IdleMobs.go":            true,
	"hooks/pinnacle_tick.go":                true,
	"mobcommands/shoot.go":                  true,
	"mobs/crafter.go":                       true,
	"usercommands/attack.go":                true,
	"usercommands/go.go":                    true,
	"usercommands/shoot.go":                 true,
	"usercommands/stand.go":                 true,
	"usercommands/target.go":                true,
	"usercommands/throw.go":                 true,
	"users/userrecord.prompt.go":            true,
}

func internalDirForReaderGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(filepath.Dir(thisFile))
}

// aggroReaders returns file -> positions of every direct Aggro read outside
// the two exempt packages.
func aggroReaders(t *testing.T) map[string][]string {
	t.Helper()
	internalDir := internalDirForReaderGuard(t)
	found := map[string][]string{}

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
		if strings.HasPrefix(rel, "characters/") || strings.HasPrefix(rel, "targeting/") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Aggro" {
				found[rel] = append(found[rel], fset.Position(sel.Pos()).String())
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return found
}

func TestNoDirectAggroReadsOutsideTheSeam(t *testing.T) {
	readers := aggroReaders(t)

	var unexpected []string
	for file := range readers {
		if !notYetMigrated[file] {
			unexpected = append(unexpected, file)
		}
	}
	sort.Strings(unexpected)
	assert.Empty(t, unexpected,
		"these files read Aggro directly and are not on the U12c-1 allowlist. "+
			"Use IsInCombat() / CurrentCombatTarget(): %s",
		strings.Join(unexpected, ", "))

	var stale []string
	for file := range notYetMigrated {
		if len(readers[file]) == 0 {
			stale = append(stale, file)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale,
		"these files are on the U12c-1 allowlist but no longer read Aggro. "+
			"Delete their entries: %s", strings.Join(stale, ", "))
}
