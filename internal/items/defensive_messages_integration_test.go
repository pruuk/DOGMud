package items

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestDefenseMessageValidRepositoryPoolsLoadThroughRealLoader(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	originalItems := items
	originalAttackMessages := attackMessages
	originalDefenseMessages := defenseMessages
	t.Cleanup(func() {
		items = originalItems
		attackMessages = originalAttackMessages
		defenseMessages = originalDefenseMessages
	})
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadDataFiles()
	for _, defenseType := range []DefenseType{DefenseQuell, DefenseDefy} {
		group := defenseMessages[defenseType]
		if group == nil {
			t.Fatalf("real loader did not load %q", defenseType)
		}
		if err := group.Validate(); err != nil {
			t.Fatalf("loaded %q pool invalid: %v", defenseType, err)
		}
	}
}
