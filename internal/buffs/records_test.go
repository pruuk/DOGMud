package buffs

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// dogmudDataDirForBuffsTest finds the repo's real world data, mirroring
// internal/narration/snapshot_test.go's dogmudDataDir: this file lives at
// internal/buffs/records_test.go, so the repo root is two levels up.
func dogmudDataDirForBuffsTest(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	return filepath.Join(root, "_datafiles", "world", "dogmud")
}

// loadRealDogmudBuffs points the engine's file-paths config at the real
// DOGMud world data and loads it through the production loader
// (buffs.LoadDataFiles, called here as LoadDataFiles since this file is
// package buffs), then restores the package's buff map to whatever it held
// before the test. Mirrors setupRealStores in
// internal/narration/snapshot_test.go, which is the only other place in the
// repo that boots the real buff files through their loader; this package's
// own shipped_flags_test.go instead yaml.Unmarshals a single file directly,
// which cannot exercise Validate()'s RoundInterval derivation or the
// duplicate-id/filename checks LoadAllFlatFiles performs.
func loadRealDogmudBuffs(t *testing.T) {
	t.Helper()

	orig := buffs
	t.Cleanup(func() { buffs = orig })

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dogmudDataDirForBuffsTest(t))
	// Buff 0 (Meditating) derives its TriggerCount from LogoutRounds at
	// Validate time and refuses a count below 1; the shipped config.yaml
	// says 3, but that file carries skip-worktree and is not read here.
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)

	LoadDataFiles()
}

// TestShippedConditionRecordsMatchTestHelperShape loads the real dogmud buff
// files (79, 80, 117 to 123) through the production loader, then compares
// each one's mechanical shape against what SeedConditionRecordsForTest
// builds. That helper is the shape every other package's fixture trusts
// (hooks, combat); if a shipped YAML file drifts from it, those fixtures
// would keep passing against a shape production no longer ships.
func TestShippedConditionRecordsMatchTestHelperShape(t *testing.T) {
	loadRealDogmudBuffs(t)

	ids := []int{
		BuffIdWarcry, BuffIdRally, BuffIdOffBalance, BuffIdRecovering,
		BuffIdMinorShield, BuffIdRegenerating, BuffIdPoisoned, BuffIdBleeding,
		BuffIdEnchantWithdrawal,
	}

	// BEFORE: snapshot what the shipped YAML files actually loaded.
	shipped := map[int]*BuffSpec{}
	for _, id := range ids {
		got := GetBuffSpec(id)
		if got == nil {
			t.Fatalf("buff id %d did not load from the shipped dogmud buff files", id)
		}
		shipped[id] = got
	}

	// AFTER: overlay the helper's hardcoded shape on top and read it back.
	restore := SeedConditionRecordsForTest()
	defer restore()

	for _, id := range ids {
		want := GetBuffSpec(id)
		got := shipped[id]

		if !reflect.DeepEqual(got.Flags, want.Flags) {
			t.Errorf("buff %d: Flags = %v, want %v", id, got.Flags, want.Flags)
		}
		if !reflect.DeepEqual(got.Effects, want.Effects) {
			t.Errorf("buff %d: Effects = %v, want %v", id, got.Effects, want.Effects)
		}
		if got.TickPool != want.TickPool {
			t.Errorf("buff %d: TickPool = %q, want %q", id, got.TickPool, want.TickPool)
		}
		if got.TickFromMagnitude != want.TickFromMagnitude {
			t.Errorf("buff %d: TickFromMagnitude = %v, want %v", id, got.TickFromMagnitude, want.TickFromMagnitude)
		}
		if got.TriggerCount != want.TriggerCount {
			t.Errorf("buff %d: TriggerCount = %d, want %d", id, got.TriggerCount, want.TriggerCount)
		}
		if got.RoundInterval != want.RoundInterval {
			t.Errorf("buff %d: RoundInterval = %d, want %d", id, got.RoundInterval, want.RoundInterval)
		}
		if got.StartUserText != want.StartUserText {
			t.Errorf("buff %d: StartUserText = %q, want %q", id, got.StartUserText, want.StartUserText)
		}
		if got.TriggerUserText != want.TriggerUserText {
			t.Errorf("buff %d: TriggerUserText = %q, want %q", id, got.TriggerUserText, want.TriggerUserText)
		}
		if got.EndUserText != want.EndUserText {
			t.Errorf("buff %d: EndUserText = %q, want %q", id, got.EndUserText, want.EndUserText)
		}
		if got.EndRoomText != want.EndRoomText {
			t.Errorf("buff %d: EndRoomText = %q, want %q", id, got.EndRoomText, want.EndRoomText)
		}
	}
}
