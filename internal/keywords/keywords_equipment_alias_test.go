package keywords

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// TestEquipmentIsInventory loads the shipped DOGMud keywords.yaml and proves
// `equipment` reaches `inventory`, whose listing shows what is worn (#260: a
// new player typed `equipment` and was told it was not a command, while the
// short `eq` already worked).
func TestEquipmentIsInventory(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadAliases()

	require.Equal(t, "inventory", TryCommandAlias("eq"), "fixture: `eq` must already alias inventory")
	require.Equal(t, "inventory", TryCommandAlias("equipment"),
		"the dogmud keywords.yaml must alias `equipment` to `inventory`")
}
