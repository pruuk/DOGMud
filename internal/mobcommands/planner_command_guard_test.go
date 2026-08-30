package mobcommands

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planCommandPattern finds the verb a planner hands back, e.g.
//
//	return PlanResult{Command: "wander", Status: StatusRunning}
//	return PlanResult{Command: "flee " + exit, ...}
var planCommandPattern = regexp.MustCompile(`Command:\s*"([a-z]+)`)

// Every command a planner emits must be a registered MOB command.
//
// ⚠️ Nothing validates this at runtime. world.go's TryCommand falls through to
// an unhandled-command path that makes the mob emote
//
//	<name> looks a little confused (<cmd> <rest>).
//
// to the entire room -- so an unregistered verb is not a silent no-op, it is
// visible spam attached to every mob running that planner, every tick.
//
// This test exists because `internal/planners/survival.go` returned "rest" as
// its default for a hurt out-of-combat mob. "rest" is not a mob command, not a
// user command, not a command at all. It reached live play and was reported by
// the owner on 2026-08-30: "Grass Snake looks a little confused (rest )."
func TestEveryPlannerCommandIsRegistered(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	plannersDir := filepath.Join(filepath.Dir(filepath.Dir(thisFile)), "planners")

	entries, err := os.ReadDir(plannersDir)
	require.NoError(t, err, "internal/planners must be readable")

	registered := map[string]bool{}
	for _, c := range GetAllMobCommands() {
		registered[c] = true
	}
	require.NotEmpty(t, registered, "the mob command registry must be populated")

	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(plannersDir, e.Name()))
		require.NoError(t, err)

		for _, m := range planCommandPattern.FindAllStringSubmatch(string(src), -1) {
			verb := m[1]
			checked++
			assert.True(t, registered[verb],
				"internal/planners/%s emits %q, which is not a registered mob "+
					"command. An unregistered verb makes every mob running this "+
					"planner emote \"looks a little confused (%s ...)\" to the "+
					"whole room. Register it in mobcommands, or emit a verb that "+
					"exists (\"noop\" is the do-nothing one).",
				e.Name(), verb, verb)
		}
	}

	require.Greater(t, checked, 5,
		"expected to find planner commands; a zero-length scan would pass vacuously")
}
