package baubles

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The catalog sweep. A record is only useful while some item points at it,
// and items leave the world in many ways the catalog never hears about
// (junked, eaten by a script, left on a corpse that decayed, on a character
// that was deleted, in a room file that was wiped). So rather than hook
// every one of them, a sweep periodically collects every bauble id any item
// still points at, in the live world and in every save file, and prunes the
// records nothing has pointed at for KeepDuration.
//
// Two phases. Collect (runSweep): the live world under the mud lock,
// through the sources registered with RegisterLiveSource (users, rooms,
// mobs, shops, guilds, the auction house), then every save file under
// DataFiles off the lock (sweep_disk.go). Apply (applySweep): under the
// catalog lock, a record something points at gets LastSeenAt = now and
// UnseenSweeps = 0, any other counts one more unseen sweep; then each
// changed shard is written without the records now prunable (prunableAt),
// before they leave memory (persistShardPruning).
//
// Fail closed: a collection that went wrong anywhere (a live source that
// panicked, a file that could not be read, or that names a bauble and does
// not parse) applies nothing at all, so a store it could not see never
// looks empty.
//
// Why a record must stay unseen for minUnseenSweeps sweeps AND
// KeepDuration: an item moving between stores while a sweep looks (from a
// room file into a live room, say) can be missed by that one sweep. Nothing
// is lost to a miss unless it repeats for the whole keep window. A store
// the sweep never looks at would repeat forever; the repo-root guards
// (TestItemWalkersVisitEveryItemField,
// TestEveryItemHolderIsASweepRootOrTransient) are what stop that.
//
// A sold record that something points at again (a crash rolled the
// seller's save back past the sale) is simply seen, so it stays. Its status
// is left as sold: every record is sellable already (sales.go), a save file
// on disk can lag a sale by one autosave, and rewriting the sale on that
// evidence would erase real ones. A bauble on a shop's secondhand shelf
// (baubles slice D) is sold and still referenced (the shops source walks
// AffixedStock), so it stays; a buyback makes it unsold (MarkBought).

// applySweep folds one complete collection (refs: every record id some item
// points at) into the catalog at now, and returns how many records were
// referenced, how many it pruned, and how many shard writes failed. A
// failed shard write prunes nothing for that shard and leaves it dirty
// (persistShardPruning); the count is returned rather than dropped, so a
// sweep whose write failed cannot be mistaken for one that fully succeeded.
// Records found after now are left alone.
func applySweep(now time.Time, refs map[string]bool, keep time.Duration) (referenced int, pruned int, shardErrors int) {
	shards := map[int]bool{}

	cat.mu.Lock()
	if cat.dir == `` {
		cat.mu.Unlock()
		return 0, 0, 0
	}
	for id, r := range cat.records {
		seq, ok := seqOf(id)
		if !ok || r.FoundAt.After(now) {
			continue
		}
		shard := shardOf(seq)
		if refs[id] {
			referenced++
			// Never earlier than a sighting already recorded: a clock
			// stepped back between sweeps must not shorten the keep window.
			if now.After(r.LastSeenAt) {
				r.LastSeenAt = now
			}
			r.UnseenSweeps = 0
			shards[shard] = true
			continue
		}
		if r.LastSeenAt.IsZero() {
			// Deploy grace: a record from before the sweep existed was never
			// seen. Its keep window starts at the first sweep that counts it,
			// or everything found over KeepDuration ago would go at the second
			// sweep, hours after the deploy.
			r.LastSeenAt = now
			shards[shard] = true
		}
		if r.UnseenSweeps < minUnseenSweeps {
			r.UnseenSweeps++
			shards[shard] = true
		}
		if r.prunableAt(now, keep) {
			shards[shard] = true
		}
	}
	cat.mu.Unlock()

	prune := func(r *Record) bool { return r.prunableAt(now, keep) }
	for shard := range shards {
		n, err := cat.persistShardPruning(shard, prune)
		pruned += n
		if err != nil {
			shardErrors++
		}
	}
	return referenced, pruned, shardErrors
}

// LiveWalk visits every item one live store holds. It runs under the mud
// lock and must only read.
type LiveWalk func(visit func(*items.Item))

var (
	liveMu      sync.Mutex
	liveSources = map[string]LiveWalk{}
	// expectedSources are the live stores a sweep must see before it may
	// apply anything (ExpectLiveSources).
	expectedSources []string
)

// ExpectLiveSources declares every live store a sweep must see. A sweep
// that runs while one of them is not registered (a module that is not
// built in, or the sweeper started before the main package registered its
// sources) fails closed rather than read that store as empty, and so does
// a sweep that runs before any expectation is declared. The main package
// declares them from baubleSweepSourceNames (bauble_sweep.go), the list
// TestBaubleSweepSourcesMatchTheGuardedRoots holds to the guarded roots.
func ExpectLiveSources(names ...string) {
	liveMu.Lock()
	defer liveMu.Unlock()
	expectedSources = append([]string{}, names...)
}

// ExpectedLiveSourceNames lists the declared live stores, sorted.
func ExpectedLiveSourceNames() []string {
	liveMu.Lock()
	defer liveMu.Unlock()
	out := append([]string{}, expectedSources...)
	sort.Strings(out)
	return out
}

// RegisterLiveSource names a store of live items for the sweep; the same
// name replaces the earlier walk. The core stores are registered by the
// main package (bauble_sweep.go), the auction house by its module.
func RegisterLiveSource(name string, walk LiveWalk) {
	liveMu.Lock()
	defer liveMu.Unlock()
	liveSources[name] = walk
}

// LiveSourceNames lists the registered live stores, sorted.
func LiveSourceNames() []string {
	liveMu.Lock()
	defer liveMu.Unlock()
	out := make([]string, 0, len(liveSources))
	for name := range liveSources {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// SnapshotSourcesForTest returns the registered live sources and the
// declared expectations, so a test that calls RegisterLiveSource or
// ExpectLiveSources can restore them afterwards with RestoreSourcesForTest
// (typically from t.Cleanup) instead of leaking its own registrations into
// later tests.
func SnapshotSourcesForTest() (sources map[string]LiveWalk, expected []string) {
	liveMu.Lock()
	defer liveMu.Unlock()
	sources = make(map[string]LiveWalk, len(liveSources))
	maps.Copy(sources, liveSources)
	expected = append([]string{}, expectedSources...)
	return sources, expected
}

// RestoreSourcesForTest replaces the registered live sources and declared
// expectations wholesale, undoing whatever a test registered or declared
// since SnapshotSourcesForTest.
func RestoreSourcesForTest(sources map[string]LiveWalk, expected []string) {
	liveMu.Lock()
	defer liveMu.Unlock()
	liveSources = sources
	expectedSources = expected
}

type namedWalk struct {
	name string
	walk LiveWalk
}

func liveSourceList() []namedWalk {
	liveMu.Lock()
	defer liveMu.Unlock()
	out := make([]namedWalk, 0, len(liveSources))
	for name, walk := range liveSources {
		out = append(out, namedWalk{name: name, walk: walk})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].name < out[b].name })
	return out
}

// sweepLock and sweepUnlock hold the world still while the live stores are
// walked.
var sweepLock, sweepUnlock = util.LockMud, util.UnlockMud

var errNoLiveSources = errors.New(`no live item sources are registered, so the live world cannot be seen`)

// collectLive walks every live store under the mud lock and returns how
// long it held the lock. The clock starts once the lock is taken, so time
// spent waiting for it is not counted: the status line and the boot
// smoke's 50 ms rule measure the hold, which is what stalls the game.
func collectLive(add func(id string)) (time.Duration, error) {
	sources := liveSourceList()
	if len(sources) == 0 {
		return 0, errNoLiveSources
	}
	if err := checkExpectedSources(sources); err != nil {
		return 0, err
	}
	sweepLock()
	defer sweepUnlock()
	start := time.Now()
	for _, s := range sources {
		if err := walkLiveSource(s, add); err != nil {
			return time.Since(start), err
		}
	}
	return time.Since(start), nil
}

var errNoExpectedSources = errors.New(`no expected live sources are declared, so a missing store cannot be told from an empty one`)

// checkExpectedSources fails when a declared store is not registered: its
// items would be unseen, and a store the sweep cannot see must not read as
// empty.
func checkExpectedSources(sources []namedWalk) error {
	expected := ExpectedLiveSourceNames()
	if len(expected) == 0 {
		return errNoExpectedSources
	}
	have := map[string]bool{}
	for _, s := range sources {
		have[s.name] = true
	}
	for _, name := range expected {
		if !have[name] {
			return fmt.Errorf(`live source %s is expected but not registered, so its items cannot be seen`, name)
		}
	}
	return nil
}

func walkLiveSource(s namedWalk, add func(id string)) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf(`live source %s: panic: %v`, s.name, r)
		}
	}()
	s.walk(func(it *items.Item) {
		if it != nil && it.Bauble != `` {
			add(it.Bauble)
		}
	})
	return nil
}

// SweepStatus is what the last sweep did, for `bauble status` and the log.
type SweepStatus struct {
	At          time.Time     // when it ran; zero if no sweep has run yet
	OK          bool          // false: it failed closed and applied nothing
	Err         string        // why it failed
	Skipped     bool          // the catalog was empty: nothing to look for
	Records     int           // records in the catalog afterwards
	Referenced  int           // records something still points at
	Pruned      int           // records it removed
	ShardErrors int           // shard writes that failed; that shard pruned nothing and stayed dirty
	Files       int           // data files it read
	Parsed      int           // of them, files that name a bauble
	Live        time.Duration // time holding the mud lock (not waiting for it)
	Disk        time.Duration // time reading data files
}

var (
	statusMu   sync.Mutex
	lastSweep  SweepStatus
	sweepRunMu sync.Mutex
)

// LastSweep is what the most recent sweep did.
func LastSweep() SweepStatus {
	statusMu.Lock()
	defer statusMu.Unlock()
	return lastSweep
}

// SweepInterval is the time between sweeps (Balance.BaubleSweepHours).
func SweepInterval() time.Duration {
	h := max(int(configs.GetBalanceConfig().BaubleSweepHours), 1)
	return time.Duration(h) * time.Hour
}

// RunSweep runs one complete sweep now against DataFiles and returns what
// it did. It takes the mud lock for the live half, so never call it while
// holding that lock (a command handler does): StartSweeper runs it on its
// own goroutine.
func RunSweep(now time.Time) SweepStatus {
	return runSweep(now, configs.GetFilePathsConfig().DataFiles.String())
}

func runSweep(now time.Time, root string) (st SweepStatus) {
	sweepRunMu.Lock()
	defer sweepRunMu.Unlock()
	st.At = now
	defer func() {
		if r := recover(); r != nil {
			st.OK = false
			st.Err = fmt.Sprintf(`panic: %v`, r)
		}
		statusMu.Lock()
		lastSweep = st
		statusMu.Unlock()
		logSweep(st)
	}()

	st.Records = Count()
	if st.Records == 0 {
		st.OK, st.Skipped = true, true
		return st
	}
	refs := map[string]bool{}
	add := func(id string) { refs[id] = true }

	live, err := collectLive(add)
	st.Live = live
	if err != nil {
		st.Err = err.Error()
		return st
	}
	start := time.Now()
	st.Files, st.Parsed, err = scanDisk(root, now, add)
	st.Disk = time.Since(start)
	if err != nil {
		st.Err = err.Error()
		return st
	}
	st.Referenced, st.Pruned, st.ShardErrors = applySweep(now, refs, KeepDuration())
	st.Records = Count()
	st.OK = true
	return st
}

func logSweep(st SweepStatus) {
	switch {
	case !st.OK:
		mudlog.Error(`baubles`, `action`, `sweep`, `result`, `failed; nothing pruned`, `error`, st.Err)
	case st.Skipped:
		mudlog.Info(`baubles`, `action`, `sweep`, `result`, `catalog empty`)
	default:
		mudlog.Info(`baubles`, `action`, `sweep`, `records`, st.Records, `referenced`, st.Referenced, `pruned`, st.Pruned,
			`shard_errors`, st.ShardErrors, `files`, st.Files, `parsed`, st.Parsed, `live`, st.Live.String(), `disk`, st.Disk.String())
	}
}

// sweepLoop runs a sweep at once, then again every interval, until stop
// closes.
func sweepLoop(stop <-chan struct{}, interval func() time.Duration, run func()) {
	for {
		run()
		t := time.NewTimer(interval())
		select {
		case <-stop:
			t.Stop()
			return
		case <-t.C:
		}
	}
}

var (
	sweeperMu   sync.Mutex
	sweeperStop chan struct{}
	sweeperDone chan struct{}
)

// StartSweeper runs the catalog sweep at once and then every
// SweepInterval, on its own goroutine. Call once at boot, after the world
// is loaded and the live sources are registered. A second call does
// nothing.
func StartSweeper() {
	sweeperMu.Lock()
	defer sweeperMu.Unlock()
	if sweeperStop != nil {
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	sweeperStop, sweeperDone = stop, done
	go func() {
		defer close(done)
		sweepLoop(stop, SweepInterval, func() { RunSweep(time.Now().UTC()) })
	}()
}

// StopSweeper stops the sweeper and waits (up to 30 seconds) for a sweep in
// progress to finish its writes. Call at shutdown before SaveAll, never
// while holding the mud lock (a sweep in progress may be waiting on it).
func StopSweeper() {
	sweeperMu.Lock()
	stop, done := sweeperStop, sweeperDone
	sweeperStop, sweeperDone = nil, nil
	sweeperMu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		mudlog.Warn(`baubles`, `action`, `stop sweeper`, `result`, `a sweep was still running after 30s; not waiting`)
	}
}
