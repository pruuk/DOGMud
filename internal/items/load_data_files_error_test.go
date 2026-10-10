package items

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #271: a broken item file made `reload items` panic, and the admin was
// never told. LoadDataFilesE returns the error, names the file, and leaves
// the previous items live.
func TestLoadDataFilesE_BadYamlReturnsErrorAndKeepsItems(t *testing.T) {
	const keptId = 999971
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		keptId: {ItemId: keptId, Name: "Kept Probe", Type: Object},
	}))

	dir := t.TempDir()
	itemDir := filepath.Join(dir, "items")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "999972-broken_probe.yaml"),
		[]byte("itemid: 999972\nname: [unclosed\n"), 0o644))

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dir)
	configs.SetConfigForTest(t, cfg)

	err := LoadDataFilesE()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken_probe", "the error names the file that broke")
	require.NotNil(t, GetItemSpec(keptId), "the previous items stay loaded")
}
