package shops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.4 / finding 6 — a corrupt shop file must not read as a brand-new shop.
//
// loadFromDisk returned nil for BOTH "no file" and "unparseable file", and
// RegisterShop treats nil as "seed from template". So one malformed byte reset
// a merchant's stock, gold and restock timers to opening-day defaults, and the
// reset looked exactly like normal initialisation. The shops/ directory is
// explicitly persistent living economy state (CLAUDE.md) — it is never wiped by
// the instance-save SOP precisely because it cannot be regenerated.
// ---------------------------------------------------------------------------

// withShopDataDir points FilePaths.DataFiles at a temp dir for one test.
func withShopDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := configs.GetFilePathsConfig().DataFiles.String()
	configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dir})
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev})
	})
	return dir
}

func templateShop() ShopInventory {
	return ShopInventory{
		Gold: 500,
		Stock: []StockEntry{
			{ItemId: 1001, RestockQty: 10, MaxStock: 20},
		},
	}
}

// writeCorruptShopFile writes raw bytes to the production shop path, so the
// test exercises the same path RegisterShop reads from. Named distinctly from
// persistence_test.go's writeShopFile, which marshals a valid inventory.
func writeCorruptShopFile(t *testing.T, zone string, mobId, roomId int, body string) string {
	t.Helper()
	p := shopPath(zone, mobId, roomId)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func quarantined(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)
	return m
}

// Control leg: no file really is a fresh shop, and must stay silent.
func TestRegisterShop_AbsentFileSeedsFromTemplateAndQuarantinesNothing(t *testing.T) {
	withShopDataDir(t)
	ClearCache()

	inv := RegisterShop("testzone", 42, 100, templateShop())

	require.NotNil(t, inv)
	assert.Equal(t, 500, inv.Gold, "a genuinely new shop seeds from template")
	assert.Empty(t, quarantined(t, shopPath("testzone", 42, 100)),
		"an absent file is not a corruption")
}

// The finding: a corrupt file must be preserved, not silently replaced by a
// fresh shop whose first save overwrites the damaged economy.
func TestRegisterShop_CorruptFileIsQuarantinedNotSilentlyReseeded(t *testing.T) {
	withShopDataDir(t)
	ClearCache()

	corrupt := "gold: 12345\nstock:\n  - itemid: 1001\n    current: 7\n  \x00bad: [unclosed\n"
	path := writeCorruptShopFile(t, "testzone", 42, 100, corrupt)

	inv := RegisterShop("testzone", 42, 100, templateShop())

	// The game keeps working — a merchant with no inventory is broken.
	require.NotNil(t, inv)

	// But the damaged economy is preserved rather than overwritten.
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr),
		"the corrupt file must be moved aside so the reseeded shop's first save cannot destroy it")

	q := quarantined(t, path)
	require.Len(t, q, 1, "exactly one quarantine file expected")
	got, err := os.ReadFile(q[0])
	require.NoError(t, err)
	assert.Equal(t, corrupt, string(got), "quarantine preserves the original bytes verbatim")
}

// After quarantine the shop must be able to save again, otherwise the merchant
// is permanently unable to persist.
func TestRegisterShop_QuarantineLeavesTheShopAbleToSave(t *testing.T) {
	withShopDataDir(t)
	ClearCache()

	path := writeCorruptShopFile(t, "testzone", 42, 100, "this: [is not valid\n")

	inv := RegisterShop("testzone", 42, 100, templateShop())
	require.NotNil(t, inv)

	require.NoError(t, SaveShop("testzone", 42, 100),
		"the quarantined path must be writable again")

	_, err := os.Stat(path)
	require.NoError(t, err, "a fresh shop file must exist after the save")
	assert.Len(t, quarantined(t, path), 1, "the recovery must not create a second quarantine")
}

// A valid file must still load its persisted economy rather than reseeding.
func TestRegisterShop_ValidFileIsLoadedNotReseeded(t *testing.T) {
	withShopDataDir(t)
	ClearCache()

	require.NoError(t, os.MkdirAll(filepath.Dir(shopPath("testzone", 42, 100)), 0o755))
	inv := RegisterShop("testzone", 42, 100, templateShop())
	require.NotNil(t, inv)
	inv.Gold = 77
	require.NoError(t, SaveShop("testzone", 42, 100))

	ClearCache()
	reloaded := RegisterShop("testzone", 42, 100, templateShop())

	require.NotNil(t, reloaded)
	assert.Equal(t, 77, reloaded.Gold,
		"a valid save must be honoured, not overwritten by template defaults")
}
