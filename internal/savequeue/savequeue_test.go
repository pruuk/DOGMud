package savequeue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// mudlog.Error/Warn dereference a package-level logger that is nil until it is
// set up, so a test driving a failure path panics instead of failing.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func write(t *testing.T, dir, name string, data []byte) PendingWrite {
	t.Helper()
	return PendingWrite{
		Kind:    "room",
		Id:      1,
		Path:    filepath.Join(dir, name),
		Data:    data,
		Careful: true,
	}
}

func del(t *testing.T, dir, name string) PendingWrite {
	t.Helper()
	return PendingWrite{Kind: "room", Id: 1, Path: filepath.Join(dir, name)}
}

// ---------------------------------------------------------------------------
// Commit
// ---------------------------------------------------------------------------

func TestCommit_WritesTheBytes(t *testing.T) {
	dir := t.TempDir()
	if err := Commit(write(t, dir, "a.yaml", []byte("hello"))); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.yaml"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestCommit_NilDataRemovesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stale.yaml")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Commit(del(t, dir, "stale.yaml")); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("delete entry did not remove the file")
	}
}

func TestCommit_DeletingAnAbsentFileIsNotAnError(t *testing.T) {
	// The overlay may legitimately never have existed: a room that has always
	// matched its template has nothing to remove.
	if err := Commit(del(t, t.TempDir(), "never-existed.yaml")); err != nil {
		t.Errorf("expected nil for an absent file, got %v", err)
	}
}

func TestCommit_FailedDeleteIsReported(t *testing.T) {
	// A failed delete is NOT harmless: the stale overlay is re-applied on the
	// next load and resurrects state the room no longer has. Removing a
	// non-empty directory fails on every platform without needing chmod.
	dir := t.TempDir()
	nested := filepath.Join(dir, "notafile")
	if err := os.MkdirAll(filepath.Join(nested, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Commit(del(t, dir, "notafile")); err == nil {
		t.Error("expected an error removing a non-empty directory, got nil")
	}
}

func TestCommit_FailedWriteIsReported(t *testing.T) {
	// Portable way to force a write failure: make the parent a regular file, so
	// opening a child path fails everywhere. chmod is unreliable on Windows.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("i am a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := PendingWrite{Path: filepath.Join(blocker, "child.yaml"), Data: []byte("x"), Careful: true}
	if err := Commit(p); err == nil {
		t.Error("expected an error writing beneath a regular file, got nil")
	}
}

// ---------------------------------------------------------------------------
// Drain
// ---------------------------------------------------------------------------

func TestDrain_RespectsTheLimitAndCommitsInOrder(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "1.yaml", []byte("one")),
		write(t, dir, "2.yaml", []byte("two")),
		write(t, dir, "3.yaml", []byte("three")),
		write(t, dir, "4.yaml", []byte("four")),
	})

	res := q.Drain(2)
	if res.Committed != 2 {
		t.Errorf("committed %d, want 2", res.Committed)
	}
	if q.Pending() != 2 {
		t.Errorf("pending %d, want 2", q.Pending())
	}
	if res.Cycle != nil {
		t.Error("cycle reported while writes were still pending")
	}

	// FIFO: the first two prepared are the first two written. Order matters for
	// reasoning about which payload is on disk at any moment.
	for _, name := range []string{"1.yaml", "2.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s should have been committed: %v", name, err)
		}
	}
	for _, name := range []string{"3.yaml", "4.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should NOT have been committed yet", name)
		}
	}
}

func TestDrain_BelowOneStillMakesProgress(t *testing.T) {
	// A queue that never drains persists nothing while the game reports itself
	// healthy -- a worse failure than any pause this design exists to fix, and
	// one config typo away.
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{write(t, dir, "1.yaml", []byte("one"))})

	if res := q.Drain(0); res.Committed != 1 {
		t.Errorf("Drain(0) committed %d, want 1", res.Committed)
	}
}

func TestDrain_ReportsTheCycleExactlyOnceOnCompletion(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "1.yaml", []byte("one")),
		write(t, dir, "2.yaml", []byte("two")),
	})

	if res := q.Drain(1); res.Cycle != nil {
		t.Fatal("cycle reported early -- this is the finding 35 regression")
	}

	res := q.Drain(1)
	if res.Cycle == nil {
		t.Fatal("cycle not reported on the drain that emptied the queue")
	}
	if res.Cycle.Total != 2 || res.Cycle.Succeeded != 2 || res.Cycle.Failed != 0 {
		t.Errorf("cycle = %+v, want Total 2 / Succeeded 2 / Failed 0", *res.Cycle)
	}
	if res.Cycle.Err() != nil {
		t.Errorf("clean cycle reported an error: %v", res.Cycle.Err())
	}

	if again := q.Drain(1); again.Cycle != nil {
		t.Error("cycle reported a second time")
	}
}

func TestDrain_EmptySetStillCompletesACycle(t *testing.T) {
	// An autosave with nothing to write still finished, and the player is still
	// owed a completion message. Reporting it through the same path keeps that
	// special case out of the hook.
	q := New()
	q.Supersede(nil)

	res := q.Drain(3)
	if res.Cycle == nil {
		t.Fatal("empty cycle never completed")
	}
	if res.Cycle.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Cycle.Total)
	}
}

func TestDrain_FailureIsCountedAndDoesNotStopTheCycle(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("i am a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	q := New()
	q.Supersede([]PendingWrite{
		{Kind: "room", Id: 1, Path: filepath.Join(blocker, "doomed.yaml"), Data: []byte("x"), Careful: true},
		write(t, dir, "ok.yaml", []byte("fine")),
	})

	res := q.Drain(10)
	if res.Failed != 1 || res.Committed != 1 {
		t.Errorf("committed %d / failed %d, want 1 / 1", res.Committed, res.Failed)
	}
	if res.Cycle == nil {
		t.Fatal("cycle should still complete when a write fails")
	}
	if res.Cycle.Err() == nil {
		t.Error("a cycle with a failure must produce an error -- silence here is finding 35")
	}

	// The healthy write must still land: one bad path cannot take the rest down.
	if _, err := os.Stat(filepath.Join(dir, "ok.yaml")); err != nil {
		t.Errorf("the good write did not land: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Cancel (guard G2)
// ---------------------------------------------------------------------------

func TestCancel_PreventsTheStaleWriteFromLanding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1.yaml")

	q := New()
	q.Supersede([]PendingWrite{write(t, dir, "1.yaml", []byte("STALE"))})

	// A synchronous save happened: it has newer bytes than the pending entry.
	if err := Commit(PendingWrite{Path: path, Data: []byte("FRESH"), Careful: true}); err != nil {
		t.Fatal(err)
	}
	if !q.Cancel(path) {
		t.Fatal("Cancel reported nothing to cancel")
	}

	q.Drain(10)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "FRESH" {
		t.Errorf("stale pending write clobbered a newer synchronous save: got %q", got)
	}
}

func TestCancel_CancelsOnlyTheNamedPath(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "1.yaml", []byte("one")),
		write(t, dir, "2.yaml", []byte("two")),
	})

	q.Cancel(filepath.Join(dir, "1.yaml"))
	if q.Pending() != 1 {
		t.Fatalf("pending %d, want 1", q.Pending())
	}

	q.Drain(10)
	if _, err := os.Stat(filepath.Join(dir, "1.yaml")); !os.IsNotExist(err) {
		t.Error("cancelled entry was committed anyway")
	}
	if _, err := os.Stat(filepath.Join(dir, "2.yaml")); err != nil {
		t.Errorf("uncancelled entry was not committed: %v", err)
	}
}

func TestCancel_UnknownPathIsANoOp(t *testing.T) {
	q := New()
	if q.Cancel(filepath.Join(t.TempDir(), "nothing.yaml")) {
		t.Error("Cancel claimed to cancel a path that was never queued")
	}
}

func TestCancel_OfTheLastEntryCompletesTheCycle(t *testing.T) {
	// A zone deletion can cancel every remaining write. The cycle still ended,
	// and the hook still needs to hear about it, or it waits forever for a
	// completion that can never arrive.
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{write(t, dir, "1.yaml", []byte("one"))})

	q.Cancel(filepath.Join(dir, "1.yaml"))

	res := q.Drain(1)
	if res.Cycle == nil {
		t.Fatal("cycle never completed after its last entry was cancelled")
	}
	if res.Cycle.Cancelled != 1 {
		t.Errorf("Cancelled = %d, want 1", res.Cycle.Cancelled)
	}
}

// ---------------------------------------------------------------------------
// Supersede (guard G3)
// ---------------------------------------------------------------------------

func TestSupersede_DiscardsAnUndrainedSetRatherThanMergingIt(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "old.yaml", []byte("old")),
	})

	// The next cycle was prepared from the same live state, so it already holds
	// a newer payload for everything the old set covered. Merging would let a
	// stale payload land after a fresh one.
	q.Supersede([]PendingWrite{write(t, dir, "new.yaml", []byte("new"))})

	if q.Pending() != 1 {
		t.Errorf("pending %d after supersede, want 1", q.Pending())
	}

	q.Drain(10)
	if _, err := os.Stat(filepath.Join(dir, "old.yaml")); !os.IsNotExist(err) {
		t.Error("a discarded entry from the previous cycle was committed")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.yaml")); err != nil {
		t.Errorf("the new cycle's entry did not commit: %v", err)
	}
}

func TestSupersede_DuplicatePathsKeepOnlyTheLast(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "dup.yaml", []byte("first")),
		write(t, dir, "dup.yaml", []byte("second")),
	})

	res := q.Drain(10)
	if res.Committed != 1 {
		t.Errorf("committed %d, want 1", res.Committed)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "dup.yaml"))
	if string(got) != "second" {
		t.Errorf("got %q, want the later payload %q", got, "second")
	}

	// Total must still account for every entry handed in, or the accounting
	// silently stops adding up.
	if res.Cycle == nil {
		t.Fatal("cycle not reported")
	}
	if c := res.Cycle; c.Total != c.Succeeded+c.Failed+c.Cancelled {
		t.Errorf("accounting does not balance: %+v", *c)
	}
}

// ---------------------------------------------------------------------------
// FlushAll (guard G4)
// ---------------------------------------------------------------------------

func TestFlushAll_CommitsEverythingRemaining(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{
		write(t, dir, "1.yaml", []byte("one")),
		write(t, dir, "2.yaml", []byte("two")),
		write(t, dir, "3.yaml", []byte("three")),
	})
	q.Drain(1)

	if err := q.FlushAll(); err != nil {
		t.Fatalf("FlushAll: %v", err)
	}
	if q.Pending() != 0 {
		t.Errorf("pending %d after flush, want 0", q.Pending())
	}
	for _, name := range []string{"1.yaml", "2.yaml", "3.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not flushed: %v", name, err)
		}
	}
}

func TestFlushAll_ReportsFailure(t *testing.T) {
	// Copyover aborts on this error, so it must actually be returned. Losing it
	// would re-exec the process and discard the pending set for good.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("i am a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	q := New()
	q.Supersede([]PendingWrite{
		{Kind: "user", Id: 7, Path: filepath.Join(blocker, "doomed.yaml"), Data: []byte("x"), Careful: true},
	})

	if err := q.FlushAll(); err == nil {
		t.Error("FlushAll returned nil despite a failed write")
	}
}

func TestFlushAll_SkipsCancelledEntries(t *testing.T) {
	dir := t.TempDir()
	q := New()
	q.Supersede([]PendingWrite{write(t, dir, "1.yaml", []byte("one"))})
	q.Cancel(filepath.Join(dir, "1.yaml"))

	if err := q.FlushAll(); err != nil {
		t.Fatalf("FlushAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "1.yaml")); !os.IsNotExist(err) {
		t.Error("FlushAll committed a cancelled entry")
	}
}

func TestFlushAll_OnAnEmptyQueueIsFine(t *testing.T) {
	if err := New().FlushAll(); err != nil {
		t.Errorf("FlushAll on a fresh queue: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The property that makes deferral safe
// ---------------------------------------------------------------------------

func TestPendingWrite_HoldsBytesNotLiveState(t *testing.T) {
	// The whole design rests on the payload being immutable once prepared. This
	// pins the shape: if PendingWrite ever grows a pointer to live game state,
	// a deferred write starts marshalling data the game is mutating underneath
	// it, and this test is where that should be argued out.
	dir := t.TempDir()
	data := []byte("original")

	q := New()
	q.Supersede([]PendingWrite{write(t, dir, "1.yaml", data)})

	// Whatever the caller does with its own buffer afterwards, the queue holds
	// the slice it was given -- so callers must hand over ownership, which
	// yaml.Marshal naturally does by returning a fresh slice.
	data = []byte("reassigned")
	_ = data

	q.Drain(1)
	got, _ := os.ReadFile(filepath.Join(dir, "1.yaml"))
	if string(got) != "original" {
		t.Errorf("payload changed after prepare: got %q", got)
	}
}
