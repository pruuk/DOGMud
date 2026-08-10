package fileloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// mudlog.Error dereferences a package-level logger that is nil until it is set
// up, so a test that exercises a failure path panics instead of failing.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Chunk 2.8 — fileloader is the save path for authored content: behaviour
// trees, dialogue, quests, mutators, species. It used to carry its own copy of
// the careful-save dance (write `<path>.new`, rename over the target) with no
// fsync, so the careful path was atomic but not durable. It now defers to
// util.Save.
//
// These tests pin the OBSERVABLE contract that migration had to preserve, so a
// future change to util.Save cannot quietly alter what authored content saves
// look like on disk.
// ---------------------------------------------------------------------------

// saveProbe is a minimal Loadable whose Filepath includes a subdirectory, so
// the tests also exercise the directory creation SaveFlatFile does first.
type saveProbe struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
	rel   string
}

func (s saveProbe) Validate() error  { return nil }
func (s saveProbe) Filepath() string { return s.rel }
func (s saveProbe) Id() string       { return s.Name }

func TestSaveFlatFile_CarefulLeavesNoTempFileBehind(t *testing.T) {
	base := t.TempDir()
	probe := saveProbe{Name: "probe", Value: 7, rel: filepath.Join("sub", "probe.yaml")}

	if err := SaveFlatFile(base, probe, SaveCareful); err != nil {
		t.Fatalf("SaveFlatFile: %v", err)
	}

	final := filepath.Join(base, "sub", "probe.yaml")
	if _, err := os.Stat(final); err != nil {
		t.Fatalf("final file missing: %v", err)
	}

	// The temp sibling is an implementation detail that must never survive a
	// successful save: a stray `.new` is content the next reader can trip over.
	for _, litter := range []string{final + ".new", final + ".tmp"} {
		if _, err := os.Stat(litter); err == nil {
			t.Errorf("careful save left %s behind", filepath.Base(litter))
		}
	}
}

func TestSaveFlatFile_CarefulOverwriteIsCompleteNotAppended(t *testing.T) {
	base := t.TempDir()
	rel := filepath.Join("sub", "probe.yaml")

	if err := SaveFlatFile(base, saveProbe{Name: "first-and-much-longer", Value: 1, rel: rel}, SaveCareful); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := SaveFlatFile(base, saveProbe{Name: "second", Value: 2, rel: rel}, SaveCareful); err != nil {
		t.Fatalf("second save: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(base, rel))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	// A shorter second write must fully replace the first, not leave its tail
	// visible. That is the whole point of writing a fresh file and renaming.
	if want := "name: second\nvalue: 2\n"; string(got) != want {
		t.Errorf("overwrite left stale content.\n got: %q\nwant: %q", got, want)
	}
}

func TestSaveFlatFile_WithoutCarefulStillWritesTheFile(t *testing.T) {
	base := t.TempDir()
	rel := filepath.Join("sub", "probe.yaml")

	// The uncareful path is a real, supported mode (the CarefulSaveFiles knob
	// exists so an operator can trade durability for speed). It must still
	// produce a correct file.
	if err := SaveFlatFile(base, saveProbe{Name: "plain", Value: 3, rel: rel}); err != nil {
		t.Fatalf("SaveFlatFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(base, rel))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if want := "name: plain\nvalue: 3\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSaveFlatFile_RejectsNonYamlExtension(t *testing.T) {
	base := t.TempDir()

	// Guarding the extension is what stops an unmarshalable file being written
	// into a directory the loader will later walk and panic on.
	if err := SaveFlatFile(base, saveProbe{Name: "x", rel: "probe.json"}, SaveCareful); err == nil {
		t.Fatal("expected an error for a non-yaml path, got nil")
	}
}

func TestSaveAllFlatFiles_WritesEveryUnitAndLeavesNoLitter(t *testing.T) {
	base := t.TempDir()

	// Flat paths on purpose: unlike SaveFlatFile, SaveAllFlatFiles does NOT
	// create the target directory, it logs and skips. Real callers save back
	// into a tree they already loaded from.
	data := map[string]saveProbe{}
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		data[name] = saveProbe{Name: name, Value: len(name), rel: name + ".yaml"}
	}

	ct, err := SaveAllFlatFiles(base, data, SaveCareful)
	if err != nil {
		t.Fatalf("SaveAllFlatFiles: %v", err)
	}
	if ct != len(data) {
		t.Errorf("saved %d files, want %d", ct, len(data))
	}

	// SaveAllFlatFiles fans out across GOMAXPROCS workers, so this also covers
	// the concurrent case: no worker may leave a temp sibling behind.
	for name, probe := range data {
		final := filepath.Join(base, probe.rel)
		if _, err := os.Stat(final); err != nil {
			t.Errorf("%s: final file missing: %v", name, err)
		}
		if _, err := os.Stat(final + ".new"); err == nil {
			t.Errorf("%s: left a .new file behind", name)
		}
	}
}
