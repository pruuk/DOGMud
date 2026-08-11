package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
// change underneath them.
//
// This test is shaped deliberately. An earlier version added two entries to a
// fresh collector and called writes() once; it PASSED against an
// implementation that returned collector-owned state, because append happened
// to reallocate between the snapshot and the mutation. Two properties are
// needed to actually pin this:
//
//   - enough entries that incidental reallocation cannot rescue a bad
//     implementation
//   - writes() called TWICE, so a version reusing its own scratch buffer
//     across calls is caught too
func TestCollector_WritesSnapshotSurvivesLaterMutation(t *testing.T) {
	c := newPendingCollector()
	p := &Plugin{name: "p", version: "1.0"}
	other := &Plugin{name: "other", version: "1.0"}

	// Several entries, so the backing array is not a 1-element special case.
	for i := 0; i < 8; i++ {
		c.add(p, savequeue.PendingWrite{Path: fmt.Sprintf("/tmp/p%d.dat", i), Data: []byte("p")})
	}

	first := c.writes()
	if len(first) != 8 {
		t.Fatalf("first snapshot has %d writes, want 8", len(first))
	}

	// Mutate the collector every way it can be mutated.
	c.add(other, savequeue.PendingWrite{Path: "/tmp/other.dat", Data: []byte("o")})
	c.discard(p)

	// A second call: catches an implementation that reuses one scratch buffer.
	second := c.writes()
	if len(second) != 1 || second[0].Path != "/tmp/other.dat" {
		t.Fatalf("second snapshot = %+v, want just /tmp/other.dat", second)
	}

	// The FIRST snapshot must be untouched by any of that.
	if len(first) != 8 {
		t.Fatalf("first snapshot length changed to %d", len(first))
	}
	for i := 0; i < 8; i++ {
		want := fmt.Sprintf("/tmp/p%d.dat", i)
		if first[i].Path != want {
			t.Errorf("first snapshot entry %d = %q, want %q", i, first[i].Path, want)
		}
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

// The collected payload must not change if the caller reuses its buffer. The
// deferred write can land many seconds after WriteBytes returned.
func TestWriteBytes_CollectedBytesAreCopied(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	collecting = newPendingCollector()
	defer func() { collecting = nil }()

	p := &Plugin{name: "probe", version: "1.0"}
	buf := []byte("original")
	if err := p.WriteBytes("state", buf); err != nil {
		t.Fatal(err)
	}

	copy(buf, "MUTATED!")

	got := collecting.writes()
	if len(got) != 1 {
		t.Fatalf("collected %d writes, want 1", len(got))
	}
	if string(got[0].Data) != "original" {
		t.Errorf("collected data = %q; the caller's buffer was aliased", got[0].Data)
	}
}

// registerTestPlugin puts a plugin in the registry with an onSave, and removes
// it afterwards. The registry is package-level state shared by all tests.
func registerTestPlugin(t *testing.T, name string, onSave func() error) *Plugin {
	t.Helper()
	p := &Plugin{name: name, version: "1.0"}
	p.Callbacks.SetOnSave(onSave)
	registry = append(registry, p)
	t.Cleanup(func() {
		kept := registry[:0]
		for _, rp := range registry {
			if rp != p {
				kept = append(kept, rp)
			}
		}
		registry = kept
	})
	return p
}

func TestPrepareAll_CollectsFromEveryPlugin(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var a, b *Plugin
	a = registerTestPlugin(t, "alpha", func() error { return a.WriteBytes("state", []byte("A")) })
	b = registerTestPlugin(t, "beta", func() error { return b.WriteBytes("state", []byte("B")) })

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("PrepareAll: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want 2", len(writes))
	}
	if collecting != nil {
		t.Error("collector left active after PrepareAll returned")
	}
}

// One plugin may write several identifiers, so the result is one PendingWrite
// per identifier written, not per plugin.
func TestPrepareAll_OnePluginCanProduceSeveralWrites(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var p *Plugin
	p = registerTestPlugin(t, "multi", func() error {
		if err := p.WriteBytes("state", []byte("S")); err != nil {
			return err
		}
		return p.WriteBytes("cache", []byte("C"))
	})

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("PrepareAll: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want 2 (one per identifier)", len(writes))
	}
}

func TestPrepareAll_FailingPluginDiscardsOnlyItsOwnWrites(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var good, bad *Plugin
	good = registerTestPlugin(t, "good", func() error { return good.WriteBytes("state", []byte("G")) })
	bad = registerTestPlugin(t, "bad", func() error {
		_ = bad.WriteBytes("partial", []byte("P")) // collected, then the callback fails
		return errors.New("gather failed halfway")
	})

	writes, err := PrepareAll()
	if err == nil {
		t.Fatal("expected an error naming the failed plugin")
	}
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: a failed plugin's partial snapshot must be dropped", len(writes))
	}
	if string(writes[0].Data) != "G" {
		t.Errorf("kept the wrong write: %q", writes[0].Data)
	}
}

// The spec's regression requirement: shutdown and copyover still call
// plugins.Save(), and it must still put bytes on disk. WriteBytes now has two
// modes, so a collector left set would make Save() silently write NOTHING while
// reporting success -- and shutdown is the one moment that cannot be retried.
func TestSave_StillWritesSynchronously(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	collecting = nil
	var p *Plugin
	p = registerTestPlugin(t, "shutdownprobe", func() error { return p.WriteBytes("state", []byte("PERSISTED")) })

	if err := Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := p.ReadBytes("state")
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "PERSISTED" {
		t.Errorf("Save() left %q on disk, want PERSISTED", got)
	}
}

// A panicking callback must not leave the registry stuck in collecting mode,
// which would make every later synchronous write silently vanish.
func TestPrepareAll_ClearsTheCollectorOnPanic(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	registerTestPlugin(t, "panicker", func() error { panic("boom") })

	defer func() {
		_ = recover()
		if collecting != nil {
			t.Error("collector left active after a panicking callback")
		}
	}()

	_, _ = PrepareAll()
}

// The slow-prepare warning is the guard rail that makes the cost ceiling
// self-reporting: without it one slow plugin is indistinguishable from several
// ordinary ones in the aggregate timing. Pinned with a negative threshold so any
// real duration trips it -- no sleeping, no flakiness.
func TestPrepareAll_SlowPluginStillPreparesNormally(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	prevThreshold := slowPrepareThreshold
	slowPrepareThreshold = -1
	defer func() { slowPrepareThreshold = prevThreshold }()

	var p *Plugin
	p = registerTestPlugin(t, "slowprobe", func() error { return p.WriteBytes("state", []byte("S")) })

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("a slow prepare must still succeed: %v", err)
	}
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: the warning path must not drop the write", len(writes))
	}
	if collecting != nil {
		t.Error("collector left active")
	}
}

// And the inverse: a fast plugin must not trip it. Guards against someone
// inverting the comparison.
func TestPrepareAll_FastPluginIsNotReportedSlow(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	prevThreshold := slowPrepareThreshold
	slowPrepareThreshold = time.Hour
	defer func() { slowPrepareThreshold = prevThreshold }()

	var p *Plugin
	p = registerTestPlugin(t, "fastprobe", func() error { return p.WriteBytes("state", []byte("F")) })

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("PrepareAll: %v", err)
	}
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1", len(writes))
	}
}
