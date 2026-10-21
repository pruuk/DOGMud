package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)

// Knowledge files are written behind, on the autosave cycle.
//
// Every Record*/Forget* call mutates the cached ObserverFile and marks it
// dirty; none of them touches the disk. Each autosave prepare pass
// (hooks.PrepareAutosaveSet) snapshots every dirty file into a
// savequeue.PendingWrite, and the shared queue commits those through util.Save
// at Timing.AutosaveWritesPerTick, the same amortised path rooms, users and
// plugins use. Shutdown and copyover call SaveAll, which writes whatever is
// still dirty and cancels any older queued copy.
//
// Why: these writes used to be a synchronous util.Save (temp file, fsync,
// rename) per call, on the event loop. RecordRoutineObservers calls
// RecordObservation AND RecordMet for every mob already in the room on every
// forager or caravan step, so a moving caravan crew of N cost about
// N x (N-1) x 2 fsyncs per step. Profiling an idle server at d4db22500 put
// that at ~62% of game CPU (~12 saves a second). Now a file is written at most
// once per autosave cycle however often it changed.
//
// Durability trade-off, the same one autosave already makes for rooms and
// users: a crash (not a clean shutdown or copyover) loses the knowledge
// changes made since the last committed autosave cycle, at most one
// Timing.RoundsPerAutoSave interval plus that cycle's drain. Everything
// committed before it survives, because each write is still util.Save.

var (
	knowledgeCache   = make(map[int]*ObserverFile)
	knowledgeCacheMu sync.RWMutex
	saveMu           sync.Mutex // serializes SaveAll's disk writes (Windows file-lock safety)

	// autosaveQueue is the process-wide pending set, shared with rooms, users
	// and plugins through SetAutosaveQueue. SaveAll cancels into it (guard G2):
	// a synchronous write must drop any older queued copy of the same file.
	autosaveQueue = savequeue.New()
)

// WriteKind is the savequeue.PendingWrite.Kind of a knowledge file. The
// autosave drain uses it to confirm commits back to this package
// (MarkCommitted); Id carries the observer mob template id.
const WriteKind = "knowledge"

// SetAutosaveQueue points this package at the shared pending set. Called once
// at startup by the autosave hook. A nil is ignored: it would make every later
// Cancel panic, far from the cause.
func SetAutosaveQueue(q *savequeue.Queue) {
	if q != nil {
		autosaveQueue = q
	}
}

func knowledgeBaseDir() string {
	return filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "knowledge")
}

// observerFilePath returns the absolute path for the given observer mob template id.
// The mobName is used in the filename for human readability; mismatch is
// tolerated (filename is not the lookup key).
func observerFilePath(mobId int, mobName string) string {
	return filepath.Join(knowledgeBaseDir(),
		fmt.Sprintf("%d-%s.yaml", mobId, util.ConvertForFilename(mobName)))
}

// markDirtyLocked records that fc changed and must be written on the next
// flush. Caller must hold knowledgeCacheMu for writing.
func markDirtyLocked(fc *ObserverFile) {
	fc.gen++
}

// dirtySnapshot is one dirty file marshalled under the cache lock.
type dirtySnapshot struct {
	fc   *ObserverFile
	path string
	data []byte
	gen  uint64
}

// snapshotDirtyLocked marshals every dirty cached file, in observer id order.
// Caller must hold knowledgeCacheMu (a read lock is enough: nothing here
// mutates).
func snapshotDirtyLocked() ([]dirtySnapshot, error) {
	ids := make([]int, 0, len(knowledgeCache))
	for id, fc := range knowledgeCache {
		if fc.gen != fc.savedGen {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	out := make([]dirtySnapshot, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		fc := knowledgeCache[id]
		data, err := yaml.Marshal(fc)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("marshal observer file %d: %w", fc.ObserverMobId, err)
			}
			continue
		}
		out = append(out, dirtySnapshot{
			fc:   fc,
			path: observerFilePath(fc.ObserverMobId, fc.ObserverName),
			data: data,
			gen:  fc.gen,
		})
	}
	return out, firstErr
}

// PrepareDirtyWrites snapshots every dirty observer file as a durable pending
// write for the autosave queue: one entry per dirty file, however many times it
// changed since the last flush.
//
// Files stay dirty until MarkCommitted confirms the commit, so an entry the
// queue discards (Supersede on an undrained set) or fails to write is simply
// prepared again on the next cycle.
//
// Called from hooks.PrepareAutosaveSet, on MainWorker under the world lock. A
// marshal failure skips that file (it stays dirty) and is returned; the other
// files still prepare.
func PrepareDirtyWrites() ([]savequeue.PendingWrite, error) {
	knowledgeCacheMu.Lock()
	snaps, err := snapshotDirtyLocked()
	for _, s := range snaps {
		s.fc.preparedGen = s.gen
	}
	knowledgeCacheMu.Unlock()

	if len(snaps) == 0 {
		return nil, err
	}
	// util.Save does not create directories, and the knowledge dir is created
	// lazily on first write.
	if mkErr := os.MkdirAll(knowledgeBaseDir(), 0o755); mkErr != nil {
		return nil, fmt.Errorf("mkdir knowledge dir: %w", mkErr)
	}

	writes := make([]savequeue.PendingWrite, 0, len(snaps))
	for _, s := range snaps {
		writes = append(writes, savequeue.PendingWrite{
			Kind:    WriteKind,
			Id:      s.fc.ObserverMobId,
			Path:    s.path,
			Data:    s.data,
			Careful: true, // always the atomic+durable path, as before
		})
	}
	return writes, err
}

// MarkCommitted records that the latest snapshot prepared for this observer is
// on disk. The autosave drain calls it for each successful WriteKind commit.
//
// It marks the PREPARED generation saved, not the current one, so a mutation
// that landed after the snapshot keeps the file dirty. The queue only ever
// holds entries from the latest prepare (Supersede replaces the whole set), so
// a confirmed commit always belongs to preparedGen.
func MarkCommitted(observerMobId int) {
	knowledgeCacheMu.Lock()
	defer knowledgeCacheMu.Unlock()
	fc, ok := knowledgeCache[observerMobId]
	if !ok {
		return
	}
	if fc.preparedGen > fc.savedGen {
		fc.savedGen = fc.preparedGen
	}
}

// SaveAll writes every dirty observer file now, synchronously. Shutdown and
// copyover call it BEFORE flushing the autosave queue: it cancels any older
// queued copy of each file it writes, so the queue cannot land stale bytes on
// top of it, and each file is written once.
//
// Must run on MainWorker (it cancels into the shared queue, which is not safe
// for concurrent use). Returns the first error; the files that failed stay
// dirty.
func SaveAll() error {
	saveMu.Lock()
	defer saveMu.Unlock()

	knowledgeCacheMu.RLock()
	snaps, firstErr := snapshotDirtyLocked()
	knowledgeCacheMu.RUnlock()

	if len(snaps) == 0 {
		return firstErr
	}
	if err := os.MkdirAll(knowledgeBaseDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir knowledge dir: %w", err)
	}

	for _, s := range snaps {
		autosaveQueue.Cancel(s.path)
		if err := util.Save(s.path, s.data); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("write %q: %w", s.path, err)
			}
			continue
		}
		knowledgeCacheMu.Lock()
		if s.gen > s.fc.savedGen {
			s.fc.savedGen = s.gen
		}
		knowledgeCacheMu.Unlock()
	}
	return firstErr
}

func loadObserverFileFromDisk(mobId int, mobName string) *ObserverFile {
	path := observerFilePath(mobId, mobName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fc := &ObserverFile{}
	if err := yaml.Unmarshal(data, fc); err != nil {
		return nil
	}
	return fc
}

// loadOrLazyInit returns the cached *ObserverFile for the given observer
// mob id, loading from disk on first access. If neither cache nor disk
// has data, an empty ObserverFile is created and cached. Mirrors the
// chunk 1.3 double-check-lock pattern.
func loadOrLazyInit(mobId int, mobName string) *ObserverFile {
	knowledgeCacheMu.RLock()
	if fc, ok := knowledgeCache[mobId]; ok {
		knowledgeCacheMu.RUnlock()
		return fc
	}
	knowledgeCacheMu.RUnlock()

	if fc := loadObserverFileFromDisk(mobId, mobName); fc != nil {
		knowledgeCacheMu.Lock()
		if cached, ok := knowledgeCache[mobId]; ok {
			knowledgeCacheMu.Unlock()
			return cached
		}
		knowledgeCache[mobId] = fc
		knowledgeCacheMu.Unlock()
		return fc
	}

	fc := &ObserverFile{
		ObserverMobId: mobId,
		ObserverName:  mobName,
		Records:       []*Record{},
	}
	knowledgeCacheMu.Lock()
	if cached, ok := knowledgeCache[mobId]; ok {
		knowledgeCacheMu.Unlock()
		return cached
	}
	knowledgeCache[mobId] = fc
	knowledgeCacheMu.Unlock()
	return fc
}
