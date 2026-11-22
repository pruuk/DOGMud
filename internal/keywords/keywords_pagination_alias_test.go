package keywords

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// TestPagingWordsOpenPaginationHelp loads the dogmud world's real
// keywords.yaml and proves every word a player might try for the pager
// reaches `help pagination` (#943, group playtest 2026-11-21).
func TestPagingWordsOpenPaginationHelp(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadAliases()

	for _, word := range []string{"page", "pages", "paging", "pager", "pagesize", "more"} {
		require.Equal(t, "pagination", TryHelpAlias(word), "help %s must open help pagination", word)
	}
}
