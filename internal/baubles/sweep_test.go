package baubles

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// withLiveSources replaces the registered live sources for one test, and
// expects exactly those.
func withLiveSources(t *testing.T, sources map[string]LiveWalk) {
	t.Helper()
	liveMu.Lock()
	saved, savedExpected := liveSources, expectedSources
	liveSources = map[string]LiveWalk{}
	expectedSources = nil
	for name, walk := range sources {
		liveSources[name] = walk
		expectedSources = append(expectedSources, name)
	}
	liveMu.Unlock()
	t.Cleanup(func() {
		liveMu.Lock()
		liveSources, expectedSources = saved, savedExpected
		liveMu.Unlock()
	})
}

// holding is a live source whose items point at ids.
func holding(ids ...string) LiveWalk {
	return func(visit func(*items.Item)) {
		for _, id := range ids {
			it := items.Item{ItemId: items.BaubleItemId, Bauble: id}
			visit(&it)
		}
	}
}

// A sweep that could not see everything applies nothing.
func TestRunSweepFailsClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		sources  map[string]LiveWalk
		expect   []string // replaces the expected sources when set
		noExpect bool     // no expected sources declared at all
		files    map[string]string
		readErr  bool
		want     string
	}{
		`no live sources`: {want: `no live item sources`},
		// The auction house registers itself; if it is not built in, or the
		// core sources were registered after the sweeper started, a store
		// is missing and must not read as empty.
		`an expected live source is not registered`: {
			sources: map[string]LiveWalk{`rooms`: holding()},
			expect:  []string{`auctions`, `rooms`},
			want:    `live source auctions is expected but not registered`,
		},
		`the expected live sources were never declared`: {
			sources:  map[string]LiveWalk{`auctions`: holding()},
			noExpect: true,
			want:     `no expected live sources are declared`,
		},
		`a live source panics`: {
			sources: map[string]LiveWalk{`rooms`: func(func(*items.Item)) { panic(`boom`) }},
			want:    `live source rooms: panic: boom`,
		},
		`a save names a bauble and does not parse`: {
			sources: map[string]LiveWalk{`rooms`: holding()},
			files:   map[string]string{`users/5.yaml`: "character:\n  items:\n  - bauble: B0000001\n   bad: [\n"},
			want:    `parse users/5.yaml`,
		},
		`a save cannot be read`: {
			sources: map[string]LiveWalk{`rooms`: holding()},
			files:   map[string]string{`users/6.yaml`: "userid: 6\n"},
			readErr: true,
			want:    `read users/6.yaml`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			SetDirForTest(t.TempDir())
			t.Cleanup(func() { items.SetBaubleResolver(nil) })
			withLiveSources(t, tc.sources)
			if tc.expect != nil || tc.noExpect {
				liveMu.Lock()
				expectedSources = tc.expect
				liveMu.Unlock()
			}
			root := t.TempDir()
			writeDataFiles(t, root, tc.files)
			if tc.readErr {
				orig := sweepReadFile
				sweepReadFile = func(string) ([]byte, error) { return nil, errors.New(`permission denied`) }
				t.Cleanup(func() { sweepReadFile = orig })
			}
			now := time.Now().UTC()
			id := sweepRecord(t, `Doomed If Pruned`, func(r *Record) {
				r.FoundAt, r.UnseenSweeps = now.Add(-400*24*time.Hour), 1
			})

			st := runSweep(now, root)
			if st.OK || !strings.Contains(st.Err, tc.want) {
				t.Fatalf("status %+v, want a failure containing %q", st, tc.want)
			}
			if r, ok := Get(id); !ok || r.UnseenSweeps != 1 {
				t.Fatalf("a failed sweep applies nothing: %+v %v", r, ok)
			}
			if got := LastSweep(); got.OK || got.Err != st.Err {
				t.Fatalf("LastSweep %+v, want the failure", got)
			}
		})
	}
}

// Live and disk references both keep a record; a record neither holds goes
// on the second sweep. The status counts what happened.
func TestRunSweepSeesLiveAndDisk(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleCatalogKeepDays = 30 })
	now := time.Now().UTC()
	old := now.Add(-60 * 24 * time.Hour)
	live := sweepRecord(t, `Held In Hand`, func(r *Record) { r.FoundAt = old })
	disk := sweepRecord(t, `Kept In A Bank`, func(r *Record) { r.FoundAt = old })
	gone := sweepRecord(t, `Junked Long Ago`, func(r *Record) { r.FoundAt, r.LastSeenAt = old, old })
	withLiveSources(t, map[string]LiveWalk{`users`: holding(live)})
	root := t.TempDir()
	writeDataFiles(t, root, map[string]string{
		`users/9.yaml`: "itemstorage:\n  slots:\n  - item:\n      itemid: 900\n      bauble: " + disk + "\n    count: 1\n",
	})

	if st := runSweep(now, root); !st.OK || st.Referenced != 2 || st.Pruned != 0 {
		t.Fatalf("first sweep %+v", st)
	}
	st := runSweep(now.Add(time.Hour), root)
	if !st.OK || st.Referenced != 2 || st.Pruned != 1 || st.Records != 2 || st.Files != 1 || st.Parsed != 1 {
		t.Fatalf("second sweep %+v, want 2 referenced, 1 pruned, 2 left, 1 file read and parsed", st)
	}
	if _, ok := Get(gone); ok {
		t.Fatal("the record nothing holds is pruned")
	}
	if got := LastSweep(); !got.OK || got.Pruned != 1 {
		t.Fatalf("LastSweep %+v", got)
	}
}

// A shard write that fails is counted rather than swallowed: applySweep
// used to drop persistShardPruning's error, so a sweep whose shard write
// failed reported a clean OK. That shard prunes nothing and stays dirty
// (TestApplySweepWriteFailurePrunesNothing), and the failure now shows in
// the status.
func TestRunSweepCountsShardErrors(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleCatalogKeepDays = 30 })
	now := time.Now().UTC()
	old := now.Add(-60 * 24 * time.Hour)
	sweepRecord(t, `Stuck Marble`, func(r *Record) { r.FoundAt, r.LastSeenAt = old, old })
	withLiveSources(t, map[string]LiveWalk{`users`: holding()})
	root := t.TempDir()

	orig := shardWriter
	shardWriter = func(string, int, []*Record) error { return errors.New(`disk full`) }
	t.Cleanup(func() { shardWriter = orig })

	st := runSweep(now, root)
	if st.ShardErrors != 1 {
		t.Fatalf("status %+v, want 1 shard error", st)
	}
	if got := LastSweep(); got.ShardErrors != 1 {
		t.Fatalf("LastSweep %+v, want 1 shard error", got)
	}
}

// A crash rolled the seller's save back past the sale: the bauble is in the
// pack again. Its record stays, still marked sold, for as long as it is
// held; once it is gone for good it goes like any other.
func TestRunSweepKeepsARolledBackSale(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleCatalogKeepDays = 30 })
	now := time.Now().UTC()
	longAgo := now.Add(-40 * 24 * time.Hour)
	id := sweepRecord(t, `Painted Wooden Horse`, func(r *Record) {
		r.FoundAt, r.Status, r.SoldAt, r.SoldValue = longAgo.Add(-24*time.Hour), StatusSold, longAgo, 6
	})
	withLiveSources(t, map[string]LiveWalk{`users`: holding()})
	root := t.TempDir()
	writeDataFiles(t, root, map[string]string{`users/3.yaml`: "character:\n  items:\n  - itemid: 900\n    bauble: " + id + "\n"})

	for i := range 3 {
		if st := runSweep(now.Add(time.Duration(i)*time.Hour), root); !st.OK || st.Pruned != 0 {
			t.Fatalf("sweep %d: %+v", i, st)
		}
	}
	r, ok := Get(id)
	if !ok || r.Status != StatusSold || r.UnseenSweeps != 0 {
		t.Fatalf("a held sold record is kept and left sold: %+v %v", r, ok)
	}

	writeDataFiles(t, root, map[string]string{`users/3.yaml`: "character:\n  items: []\n"})
	runSweep(r.LastSeenAt.Add(time.Hour), root)
	runSweep(r.LastSeenAt.Add(KeepDuration()), root)
	if _, ok := Get(id); ok {
		t.Fatal("gone for the keep window after its last sighting, it is pruned")
	}
}

// Live times the hold of the mud lock, not the wait for it.
func TestRunSweepLiveTimesTheHoldNotTheWait(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	sweepRecord(t, `Timed Marble`, nil)
	withLiveSources(t, map[string]LiveWalk{`users`: holding()})
	origLock := sweepLock
	sweepLock = func() {
		time.Sleep(200 * time.Millisecond) // a busy game loop
		origLock()
	}
	t.Cleanup(func() { sweepLock = origLock })
	if st := runSweep(time.Now().UTC(), t.TempDir()); !st.OK || st.Live >= 150*time.Millisecond {
		t.Fatalf("status %+v: Live counted the wait for the lock", st)
	}
}

// An empty catalog has nothing to look for: no scan.
func TestRunSweepSkipsAnEmptyCatalog(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	withLiveSources(t, nil)
	if st := runSweep(time.Now().UTC(), t.TempDir()); !st.OK || !st.Skipped || st.Files != 0 {
		t.Fatalf("status %+v, want OK and skipped", st)
	}
}

// Sweeps and ordinary catalog writes at once: run under -race.
func TestSweepRacesWithCatalogWrites(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	now := time.Now().UTC()
	ids := []string{}
	for range 20 {
		ids = append(ids, sweepRecord(t, `Glass Marble`, func(r *Record) { r.FoundAt = now.Add(-60 * 24 * time.Hour) }))
	}
	withLiveSources(t, map[string]LiveWalk{`users`: holding(ids[:10]...)})
	root := t.TempDir()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 5 {
			runSweep(now.Add(time.Duration(i)*time.Hour), root)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 50 {
			v := i%5 + 1
			Update(ids[i%20], func(r *Record) { r.Value = v })
			Get(ids[(i+7)%20])
		}
	}()
	wg.Wait()
	for _, id := range ids[:10] {
		if _, ok := Get(id); !ok {
			t.Fatalf("held record %s was pruned", id)
		}
	}
}

func TestSweepIntervalFromConfig(t *testing.T) {
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleSweepHours = 3 })
	if got := SweepInterval(); got != 3*time.Hour {
		t.Fatalf("interval %v, want 3h", got)
	}
}

// The loop runs a sweep at once, then again every interval, until stopped.
func TestSweepLoopRunsUntilStopped(t *testing.T) {
	stop := make(chan struct{})
	runs := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		sweepLoop(stop, func() time.Duration { return time.Millisecond }, func() {
			select {
			case runs <- struct{}{}:
			default:
			}
		})
		close(done)
	}()
	for range 3 {
		select {
		case <-runs:
		case <-time.After(2 * time.Second):
			t.Fatal("the loop stopped running sweeps")
		}
	}
	close(stop)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop did not stop")
	}
}
