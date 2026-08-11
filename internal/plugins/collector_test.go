package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// mudlog.Error dereferences a package-level logger that is nil until it is set
// up, so any test touching a failure path panics instead of failing. This is
// the first test file in the package, so it owns TestMain.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func TestCollector_CollectsInOrderAndReports(t *testing.T) {
	c := newPendingCollector()

	p1 := &Plugin{name: "alpha", version: "1.0"}
	p2 := &Plugin{name: "beta", version: "1.0"}

	c.add(p1, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/a.dat", Data: []byte("a")})
	c.add(p2, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/b.dat", Data: []byte("b")})

	got := c.writes()
	if len(got) != 2 {
		t.Fatalf("collected %d writes, want 2", len(got))
	}
	if got[0].Path != "/tmp/a.dat" || got[1].Path != "/tmp/b.dat" {
		t.Errorf("writes out of order: %q then %q", got[0].Path, got[1].Path)
	}
}

// A callback that fails may have collected a PARTIAL snapshot. Persisting half
// of a plugin's state is worse than leaving the previous complete file alone
// for another cycle, so its writes are dropped.
func TestCollector_DiscardDropsOnlyThatPluginsWrites(t *testing.T) {
	c := newPendingCollector()

	good := &Plugin{name: "good", version: "1.0"}
	bad := &Plugin{name: "bad", version: "1.0"}

	c.add(good, savequeue.PendingWrite{Path: "/tmp/good.dat", Data: []byte("g")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad1.dat", Data: []byte("b1")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad2.dat", Data: []byte("b2")})

	c.discard(bad)

	got := c.writes()
	if len(got) != 1 {
		t.Fatalf("after discard got %d writes, want 1", len(got))
	}
	if got[0].Path != "/tmp/good.dat" {
		t.Errorf("kept the wrong write: %q", got[0].Path)
	}
}

func TestCollector_DiscardOfAnUnknownPluginIsSafe(t *testing.T) {
	c := newPendingCollector()
	c.add(&Plugin{name: "a", version: "1.0"}, savequeue.PendingWrite{Path: "/tmp/a.dat"})

	c.discard(&Plugin{name: "never-added", version: "1.0"})

	if len(c.writes()) != 1 {
		t.Errorf("discarding an unknown plugin changed the collected set")
	}
}

// writes() returns a snapshot, not a view. The collector is mutated between a
// prepare and its drain, so a caller holding an earlier result must not see it
// change underneath them. Returning c.entries directly would be an easy and
// silent regression.
func TestCollector_WritesSnapshotSurvivesLaterMutation(t *testing.T) {
	c := newPendingCollector()
	p := &Plugin{name: "p", version: "1.0"}
	other := &Plugin{name: "other", version: "1.0"}

	c.add(p, savequeue.PendingWrite{Path: "/tmp/first.dat", Data: []byte("1")})
	snapshot := c.writes()

	c.add(other, savequeue.PendingWrite{Path: "/tmp/second.dat", Data: []byte("2")})
	c.discard(p)

	if len(snapshot) != 1 {
		t.Fatalf("snapshot length changed to %d after mutating the collector", len(snapshot))
	}
	if snapshot[0].Path != "/tmp/first.dat" {
		t.Errorf("snapshot content changed to %q", snapshot[0].Path)
	}
}

// Interleaved owners are what exercise the in-place filter: the write index
// trails the read index across multiple gaps. A contiguous block does not.
func TestCollector_DiscardWithInterleavedOwners(t *testing.T) {
	c := newPendingCollector()
	good := &Plugin{name: "good", version: "1.0"}
	bad := &Plugin{name: "bad", version: "1.0"}

	c.add(good, savequeue.PendingWrite{Path: "/tmp/g1.dat"})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/b1.dat"})
	c.add(good, savequeue.PendingWrite{Path: "/tmp/g2.dat"})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/b2.dat"})
	c.add(good, savequeue.PendingWrite{Path: "/tmp/g3.dat"})

	c.discard(bad)

	got := c.writes()
	want := []string{"/tmp/g1.dat", "/tmp/g2.dat", "/tmp/g3.dat"}
	if len(got) != len(want) {
		t.Fatalf("got %d writes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Path != want[i] {
			t.Errorf("write %d = %q, want %q", i, got[i].Path, want[i])
		}
	}
}

// While a collector is active, WriteBytes must NOT touch the disk.
func TestWriteBytes_CollectsInsteadOfWritingWhileCollecting(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	p := &Plugin{name: "probe", version: "1.0"}

	collecting = newPendingCollector()
	defer func() { collecting = nil }()

	if err := p.WriteBytes("state", []byte("payload")); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	got := collecting.writes()
	if len(got) != 1 {
		t.Fatalf("collected %d writes, want 1", len(got))
	}
	if string(got[0].Data) != "payload" {
		t.Errorf("collected data = %q, want %q", got[0].Data, "payload")
	}
	if got[0].Kind != "plugin" {
		t.Errorf("Kind = %q, want plugin", got[0].Kind)
	}

	// Nothing may exist on disk yet.
	if entries, _ := os.ReadDir(filepath.Join(dir, "probe-v1-0")); len(entries) != 0 {
		t.Errorf("WriteBytes wrote to disk while collecting: %d file(s)", len(entries))
	}
}

// With no collector, the existing synchronous behaviour is unchanged.
func TestWriteBytes_WritesSynchronouslyWithNoCollector(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	collecting = nil
	p := &Plugin{name: "probe", version: "1.0"}

	if err := p.WriteBytes("state", []byte("payload")); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	got, err := p.ReadBytes("state")
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("read back %q, want %q", got, "payload")
	}
}

// useTempWriteFolder points plugin storage at a temp dir for the duration of a
// test. writeFolderPath and writeFolderReady are package-level state set by
// plugins.Load(), which a unit test does not run.
func useTempWriteFolder(t *testing.T, dir string) func() {
	t.Helper()
	prevPath, prevReady := writeFolderPath, writeFolderReady
	writeFolderPath, writeFolderReady = dir, true
	return func() { writeFolderPath, writeFolderReady = prevPath, prevReady }
}
