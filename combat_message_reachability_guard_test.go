package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Owner ruling C (spec 2026-10-16 #541 section 3): round combat releases a
// target in another room before the wait round or the swing
// (hooks.resolveCombatTarget), so a separate block can never render. The
// decoder rejects one at load; this guard names the file and line. The exit
// tokens only ever filled separate lines, so a line carrying one would read
// "unknown".
func TestCombatMessagesHaveNoSeparateBlock(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "combat-messages", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no combat-message files: the glob is wrong, not the data")
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		if hit := unreachableCombatLine(string(raw)); hit != "" {
			t.Errorf("%s: %s", f, hit)
		}
	}
}

// unreachableCombatLine returns the first line that can never render, or "".
func unreachableCombatLine(src string) string {
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "separate:") || strings.Contains(trimmed, "remote_observer:") {
			return fmt.Sprintf("line %d: a separate block cannot render: %s", i+1, trimmed)
		}
		if strings.Contains(line, "{exitname}") || strings.Contains(line, "{entrancename}") {
			return fmt.Sprintf("line %d: an exit token only ever filled a separate line: %s", i+1, trimmed)
		}
	}
	return ""
}

// The guard must be able to fail.
func TestUnreachableCombatLine_CanFail(t *testing.T) {
	for _, src := range []string{
		"options:\n  wait:\n    separate:\n      beginner: []\n",
		"options:\n  wait:\n    together:\n      beginner:\n      - actor: 'You aim through the {exitname}.'\n",
	} {
		if unreachableCombatLine(src) == "" {
			t.Errorf("missed an unreachable line in %q", src)
		}
	}
	if hit := unreachableCombatLine("# {exitname} - a header comment\noptions:\n"); hit != "" {
		t.Errorf("flagged a comment: %s", hit)
	}
}
