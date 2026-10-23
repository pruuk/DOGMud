package knowledge

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// Knowledge writes ride the autosave cycle (internal/savequeue) instead of
// fsyncing on the event loop per observation. Profiling an idle server at
// d4db22500 put RecordRoutineObservers -> util.Save at ~62% of game CPU: every
// caravan step had each crew member fsync its file twice (RecordObservation,
// then RecordMet). These tests pin the replacement contract: memory is updated
// at once, disk is written once per dirty file per flush, and a clean shutdown
// loses nothing.

// withObserverNames gives the test observers stable filenames (the real lookup
// needs loaded mob specs) and restores the production lookup afterwards.
func withObserverNames(t *testing.T, names map[int]string) {
	t.Helper()
	prev := observerNameFor
	observerNameFor = func(id int) string { return names[id] }
	t.Cleanup(func() { observerNameFor = prev })
}

func withRound(t *testing.T, r uint64) {
	t.Helper()
	roundForTest = func() uint64 { return r }
	t.Cleanup(func() { roundForTest = nil })
}

// withTestQueue points the package at a private queue so a test's prepared
// entries never leak into another test, and restores the shared one after.
func withTestQueue(t *testing.T) *savequeue.Queue {
	t.Helper()
	prev := autosaveQueue
	q := savequeue.New()
	SetAutosaveQueue(q)
	t.Cleanup(func() { autosaveQueue = prev })
	return q
}

func removeObserverFile(t *testing.T, id int, name string) string {
	t.Helper()
	path := observerFilePath(id, name)
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

// commitAll stands in for the autosave drain: commit each prepared write and
// confirm it, the way drainPendingWrites does for Kind == WriteKind.
func commitAll(t *testing.T, writes []savequeue.PendingWrite) {
	t.Helper()
	for _, w := range writes {
		if err := savequeue.Commit(w); err != nil {
			t.Fatalf("commit %s: %v", w.Path, err)
		}
		MarkCommitted(w.Id)
	}
}

func TestRecordObservation_UpdatesMemoryAndWritesNoFile(t *testing.T) {
	resetCache()
	withTestQueue(t)
	withObserverNames(t, map[int]string{7101: "batch_obs_a"})
	withRound(t, 100)
	path := removeObserverFile(t, 7101, "batch_obs_a")

	RecordObservation(7101, MobSubject(50), 900)
	RecordMet(7101, MobSubject(50), 900, SourceWitnessed)

	r := Get(7101, MobSubject(50))
	if r == nil || !r.HasMet || r.LastSeenRoom != 900 || len(r.Observations) != 1 {
		t.Fatalf("observation not readable from memory at once: %+v", r)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("an observation wrote %s on the event loop (stat err %v); writes belong to the flush", path, err)
	}
}

func TestPrepareDirtyWrites_OneWritePerDirtyFile(t *testing.T) {
	resetCache()
	withTestQueue(t)
	withObserverNames(t, map[int]string{7201: "batch_obs_b", 7202: "batch_obs_c", 7203: "batch_obs_clean"})
	withRound(t, 100)
	removeObserverFile(t, 7201, "batch_obs_b")
	removeObserverFile(t, 7202, "batch_obs_c")

	// A caravan step: both calls per observer, several subjects, several steps.
	for room := 900; room < 905; room++ {
		roundForTest = func() uint64 { return uint64(room) }
		for _, subj := range []int{50, 51, 52} {
			RecordObservation(7201, MobSubject(subj), room)
			RecordMet(7201, MobSubject(subj), room, SourceWitnessed)
		}
	}
	RecordObservation(7202, MobSubject(50), 900)
	_ = Get(7203, MobSubject(50)) // loaded, never mutated: not dirty

	writes, err := PrepareDirtyWrites()
	if err != nil {
		t.Fatalf("PrepareDirtyWrites: %v", err)
	}
	if len(writes) != 2 {
		ids := make([]int, 0, len(writes))
		for _, w := range writes {
			ids = append(ids, w.Id)
		}
		t.Fatalf("got %d writes for observers %v, want exactly 2 (one per dirty file: 7201, 7202)", len(writes), ids)
	}
	seen := map[string]bool{}
	for _, w := range writes {
		if w.Kind != WriteKind {
			t.Errorf("write kind %q, want %q", w.Kind, WriteKind)
		}
		if !w.Careful {
			t.Errorf("knowledge write for %d is not routed through the durable path", w.Id)
		}
		if seen[w.Path] {
			t.Errorf("path %s prepared twice in one flush", w.Path)
		}
		seen[w.Path] = true
	}

	commitAll(t, writes)

	again, err := PrepareDirtyWrites()
	if err != nil {
		t.Fatalf("PrepareDirtyWrites (second): %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("%d committed, unchanged file(s) were prepared again", len(again))
	}

	loaded := loadObserverFileFromDisk(7201, "batch_obs_b")
	if loaded == nil || len(loaded.Records) != 3 {
		t.Fatalf("flushed file did not carry all three records: %+v", loaded)
	}
}

// A prepared entry can be discarded without committing (savequeue.Supersede
// drops an undrained set, a commit can fail). The file must stay dirty until a
// commit is confirmed, and a mutation that lands after the snapshot must not be
// marked saved by the commit of the older bytes.
func TestPrepareDirtyWrites_StaysDirtyUntilCommitConfirmed(t *testing.T) {
	resetCache()
	withTestQueue(t)
	withObserverNames(t, map[int]string{7301: "batch_obs_d"})
	withRound(t, 100)
	removeObserverFile(t, 7301, "batch_obs_d")

	RecordObservation(7301, MobSubject(50), 900)
	first, _ := PrepareDirtyWrites()
	if len(first) != 1 {
		t.Fatalf("want 1 write, got %d", len(first))
	}

	// Never committed: it must come back.
	retry, _ := PrepareDirtyWrites()
	if len(retry) != 1 {
		t.Fatalf("an uncommitted write was dropped: got %d writes on the next flush", len(retry))
	}

	// Mutate after the snapshot, then confirm the OLD snapshot.
	roundForTest = func() uint64 { return 101 }
	RecordObservation(7301, MobSubject(50), 901)
	commitAll(t, retry)

	after, _ := PrepareDirtyWrites()
	if len(after) != 1 {
		t.Fatalf("a mutation after the snapshot was marked saved by the older commit: got %d writes", len(after))
	}
}

// Shutdown and copyover call SaveAll: every dirty file is written now, and any
// older copy still queued is cancelled so it cannot land on top of the newer.
func TestSaveAll_PersistsDirtyAndCancelsQueuedCopy(t *testing.T) {
	resetCache()
	q := withTestQueue(t)
	withObserverNames(t, map[int]string{7401: "batch_obs_e"})
	withRound(t, 100)
	path := removeObserverFile(t, 7401, "batch_obs_e")

	RecordObservation(7401, MobSubject(50), 900)
	writes, _ := PrepareDirtyWrites()
	q.Supersede(writes)

	roundForTest = func() uint64 { return 200 }
	RecordObservation(7401, MobSubject(50), 950)

	if err := SaveAll(); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	if q.Pending() != 0 {
		t.Errorf("SaveAll left %d older knowledge write(s) queued to overwrite it", q.Pending())
	}
	if err := q.FlushAll(); err != nil {
		t.Fatalf("FlushAll: %v", err)
	}

	loaded := loadObserverFileFromDisk(7401, "batch_obs_e")
	if loaded == nil || len(loaded.Records) != 1 || loaded.Records[0].LastSeenRoom != 950 {
		t.Fatalf("shutdown flush did not persist the newest state to %s: %+v", path, loaded)
	}
	if rest, _ := PrepareDirtyWrites(); len(rest) != 0 {
		t.Errorf("SaveAll did not mark its files saved: %d still dirty", len(rest))
	}
}

// The trade-off, pinned: a crash between flushes loses what changed since the
// last flush and nothing before it.
func TestCrashBetweenFlushes_LosesOnlyTheUnflushedTail(t *testing.T) {
	resetCache()
	withTestQueue(t)
	withObserverNames(t, map[int]string{7501: "batch_obs_f"})
	withRound(t, 100)
	removeObserverFile(t, 7501, "batch_obs_f")

	RecordObservation(7501, MobSubject(50), 900)
	writes, _ := PrepareDirtyWrites()
	commitAll(t, writes)

	roundForTest = func() uint64 { return 200 }
	RecordObservation(7501, MobSubject(50), 950)

	resetCache() // the crash: memory gone, no flush ran

	r := Get(7501, MobSubject(50))
	if r == nil {
		t.Fatal("the flushed observation was lost")
	}
	if r.LastSeenRound != 100 || len(r.Observations) != 1 {
		t.Errorf("after a crash the record should be the last flush (round 100, 1 observation), got round %d with %d", r.LastSeenRound, len(r.Observations))
	}
}

func TestSetAutosaveQueue_IgnoresNil(t *testing.T) {
	before := autosaveQueue
	SetAutosaveQueue(nil)
	if autosaveQueue != before {
		t.Error("SetAutosaveQueue(nil) replaced the live queue")
	}
}
