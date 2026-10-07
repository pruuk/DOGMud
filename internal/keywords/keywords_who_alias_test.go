package keywords

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// TestWhoIsOnline loads each shipped world's real keywords.yaml and proves
// `who` is an alias of `online`, as a command and as a help topic (owner,
// 2026-10-06, #421: the room-roster `who` is retired; `look` already shows
// who is in the room).
func TestWhoIsOnline(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	for _, world := range []string{"dogmud", "default"} {
		t.Run(world, func(t *testing.T) {
			cfg := configs.GetConfig()
			cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", world))
			configs.SetConfigForTest(t, cfg)

			LoadAliases()

			require.Equal(t, "online", TryCommandAlias("who"),
				"the %s world's keywords.yaml must alias `who` to `online`", world)
			require.Equal(t, "online", TryHelpAlias("who"),
				"the %s world's keywords.yaml must alias `help who` to `help online`", world)
		})
	}
}
