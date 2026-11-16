package tips

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func TestValidate(t *testing.T) {
	if err := Validate([]string{"Try help.", "Rest to heal."}); err != nil {
		t.Errorf("valid tips refused: %v", err)
	}
	if err := Validate([]string{"Try help.", "  "}); err == nil {
		t.Error("a blank tip was accepted")
	}
}

func TestNext_RotatesInOrderAndWraps(t *testing.T) {
	defer SeedForTest([]string{"a", "b", "c"})()
	var got []string
	for range 5 {
		got = append(got, Next())
	}
	want := []string{"a", "b", "c", "a", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rotation = %q, want %q", got, want)
		}
	}
}

func TestNext_EmptyStoreIsBlank(t *testing.T) {
	defer SeedForTest(nil)()
	if got := Next(); got != "" {
		t.Errorf("Next() on an empty store = %q, want \"\"", got)
	}
}

func TestLoad_MissingFileIsAnEmptyStore(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)
	defer SeedForTest([]string{"stale"})()

	Load()

	if Count() != 0 {
		t.Errorf("Count() = %d after loading a world with no tips file, want 0", Count())
	}
}

func TestLoad_ReadsTheTipsKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tips.yaml"), []byte("tips:\n  - first tip\n  - second tip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dir)
	configs.SetConfigForTest(t, cfg)
	defer SeedForTest(nil)()

	Load()

	if all := All(); len(all) != 2 || all[0] != "first tip" || all[1] != "second tip" {
		t.Errorf("All() = %q", all)
	}
}
