package playtestprofiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoTemplatesSanitize(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "playtest", "profiles")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("repo profiles directory not present")
	}
	for _, id := range KnownTemplateIDs {
		t.Run(id, func(t *testing.T) {
			u, err := LoadTemplate(dir, id)
			require.NoError(t, err)
			require.NotNil(t, u.Character)
			require.Empty(t, u.Password)
			if id == "admin" {
				require.Equal(t, "admin", u.Role)
			} else {
				require.Equal(t, "user", u.Role)
			}
		})
	}
}

// TestRepoTemplatesSanitize walks the allowlist to the files. It cannot see a
// file with no allowlist entry, and that is exactly how slice-a-infrared.yaml
// shipped: playtestrun refuses an unregistered profile with "unknown profile",
// but only once a scenario actually names it, so a full green suite said
// nothing and the failure surfaced as a dead playtest lane instead.
func TestRepoTemplatesAreAllRegistered(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "playtest", "profiles")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("repo profiles directory not present")
	}

	known := map[string]bool{}
	for _, id := range KnownTemplateIDs {
		known[id] = true
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	seen := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		seen++
		id := strings.TrimSuffix(e.Name(), ".yaml")
		require.True(t, known[id],
			"tools/playtest/profiles/%s.yaml has no KnownTemplateIDs entry, so every scenario naming %q dies with \"unknown profile\"", id, id)
	}
	require.NotZero(t, seen, "no profile YAML found at all: the walk is broken, not the profiles")
}
