package messaging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// CategoryScreen carries laid-out screens whose lines are not sentences: the
// map's legend ends on a name and the achievements headings are bare
// category names. The normalizer must leave them exactly as drawn, and the
// category must otherwise look as CategorySystem did, where they rode.
func TestScreenCategoryLeavesLinesAsDrawn(t *testing.T) {
	require.Equal(t, "screen", CategoryScreen.String())
	for _, line := range []string{
		`<ansi fg="cyan-bold">Combat</ansi>`,
		` <ansi fg="map-legend"> [</ansi>~ Shore`,
		`o o o-o`,
		`  [x] a apple`,
	} {
		require.Equal(t, line, Normalize(CategoryScreen, line), "a screen line must not be rewritten")
	}
	// The probe can fail: the same heading on System gets the stop the
	// 2026-10-17 playtest saw.
	require.Equal(t, `<ansi fg="cyan-bold">Combat.</ansi>`, Normalize(CategorySystem, `<ansi fg="cyan-bold">Combat</ansi>`))

	require.False(t, shouldWrap(CategoryScreen), "System never wrapped these screens")
	for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
		require.False(t, v.Suppresses(CategoryScreen), "verbosity %v suppresses screen", v)
	}

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "ansi-aliases.yaml"))
	require.NoError(t, err)
	var doc struct {
		Colors map[string]any `yaml:"colors"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	require.Contains(t, doc.Colors, "screen")
	require.Equal(t, doc.Colors["system"], doc.Colors["screen"], "screen must keep system's colour")
}
