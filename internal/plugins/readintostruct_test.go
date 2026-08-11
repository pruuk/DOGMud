package plugins

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// ---------------------------------------------------------------------------
// ReadIntoStruct used to get absent-vs-corrupt exactly backwards:
//
//	if err = yaml.Unmarshal(b, out); err == nil {
//	    return err          // nil on success
//	}
//	return nil              // ALSO nil on failure
//
// It could never report a parse failure, while it DID return an error for a
// merely-absent file. It reported the harmless case and hid the dangerous one.
// ---------------------------------------------------------------------------

type probeState struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

func TestReadIntoStruct_RoundTripsGoodData(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	p := &Plugin{name: "riprobe", version: "1.0"}
	if err := p.WriteStruct("state", probeState{Name: "alpha", Count: 7}); err != nil {
		t.Fatal(err)
	}

	var got probeState
	if err := p.ReadIntoStruct("state", &got); err != nil {
		t.Fatalf("ReadIntoStruct: %v", err)
	}
	if got.Name != "alpha" || got.Count != 7 {
		t.Errorf("got %+v, want {alpha 7}", got)
	}
}

// Absent is the ordinary first-run case and must be distinguishable, so a
// caller can seed defaults without treating it as damage.
func TestReadIntoStruct_AbsentIsItsOwnError(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	p := &Plugin{name: "riprobe", version: "1.0"}

	var got probeState
	err := p.ReadIntoStruct("never-written", &got)
	if !errors.Is(err, util.ErrStateAbsent) {
		t.Fatalf("err = %v, want ErrStateAbsent", err)
	}
	if errors.Is(err, util.ErrStateCorrupt) {
		t.Error("an absent file must not report as corrupt")
	}
}

// THE bug. A file that exists but does not parse must be reported, not
// swallowed. Before the fix this returned nil.
func TestReadIntoStruct_CorruptIsReportedNotSwallowed(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	p := &Plugin{name: "riprobe", version: "1.0"}
	if err := p.WriteStruct("state", probeState{Name: "alpha", Count: 7}); err != nil {
		t.Fatal(err)
	}

	_, fullPath := p.dataPath("state")
	if err := os.WriteFile(fullPath, []byte("name: [unclosed\n\tbad: :\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got probeState
	err := p.ReadIntoStruct("state", &got)
	if err == nil {
		t.Fatal("a corrupt file returned nil; the caller can never learn its data was lost")
	}
	if !errors.Is(err, util.ErrStateCorrupt) {
		t.Errorf("err = %v, want ErrStateCorrupt", err)
	}
}

// A partially applied unmarshal is worse than none: the caller cannot tell
// which fields are real. yaml.v2 populates as it walks, so this is reachable.
func TestReadIntoStruct_CorruptLeavesNoHalfAppliedData(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	p := &Plugin{name: "riprobe", version: "1.0"}
	_, fullPath := p.dataPath("state")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid for the first key, then broken: yaml.v2 applies `name` before it
	// reaches the fault.
	if err := os.WriteFile(fullPath, []byte("name: realvalue\ncount: [1, 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := probeState{Name: "PRESET", Count: 99}
	if err := p.ReadIntoStruct("state", &got); err == nil {
		t.Fatal("expected a corruption error")
	}

	if got.Name != "" || got.Count != 0 {
		t.Errorf("out = %+v; a failed read must leave the zero value, not a hybrid", got)
	}
}

// Contract rule 3: quarantine, never delete. The bytes are evidence, and the
// original path must then read as absent so the seed-defaults path takes over.
func TestReadIntoStruct_CorruptFileIsQuarantinedNotDeleted(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	p := &Plugin{name: "riprobe", version: "1.0"}
	_, fullPath := p.dataPath("state")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const garbage = "name: [unclosed\n"
	if err := os.WriteFile(fullPath, []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}

	var got probeState
	if err := p.ReadIntoStruct("state", &got); err == nil {
		t.Fatal("expected a corruption error")
	}

	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Error("the corrupt file is still in place; the next read will fail identically forever")
	}

	entries, err := os.ReadDir(filepath.Dir(fullPath))
	if err != nil {
		t.Fatal(err)
	}
	found := ""
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupt-") {
			found = e.Name()
		}
	}
	if found == "" {
		t.Fatal("no quarantined copy; the evidence was destroyed")
	}
	kept, err := os.ReadFile(filepath.Join(filepath.Dir(fullPath), found))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != garbage {
		t.Errorf("quarantined copy = %q, want the original bytes", kept)
	}

	// And the path now reads as absent, so the caller seeds defaults.
	if err := p.ReadIntoStruct("state", &got); !errors.Is(err, util.ErrStateAbsent) {
		t.Errorf("after quarantine err = %v, want ErrStateAbsent", err)
	}
}
