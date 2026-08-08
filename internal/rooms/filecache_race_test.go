package rooms

import (
	"fmt"
	"sync"
	"testing"
)

// Finding 8: roomIdToFileCache was read and written with no synchronization.
// The global MUD lock does not serialize these writes, because GetAutoComplete
// (world.go) holds only util.RLockMud() and calls rooms.LoadRoom, so several
// connection goroutines can be in an uncached room load at once.
//
// Concurrent map writes are a `fatal error`, NOT a panic. recover() cannot
// catch it and the whole server dies. These tests must be run under -race to
// be meaningful; without it a torn map access may simply get lucky.

// Concurrent first-writes to distinct keys. This is the tab-completion
// scenario: several players in different uncached rooms at the same moment.
func TestFileCache_ConcurrentDistinctWrites(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}

	const goroutines = 32
	const perGoroutine = 25

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := g*perGoroutine + i
				rm.setCachedFilePath(id, fmt.Sprintf("zone/%d.yaml", id))
			}
		}(g)
	}
	wg.Wait()

	if got := len(rm.cachedFilePathIds()); got != goroutines*perGoroutine {
		t.Errorf("cached ids = %d, want %d", got, goroutines*perGoroutine)
	}
}

// Concurrent readers and writers on the SAME key. A read racing a write on one
// bucket is what actually trips the runtime detector.
func TestFileCache_ConcurrentReadWriteSameKey(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}
	const roomId = 42
	rm.setCachedFilePath(roomId, "zone/42.yaml")

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if g%2 == 0 {
					rm.setCachedFilePath(roomId, fmt.Sprintf("zone/%d-%d.yaml", g, i))
				} else if path, ok := rm.cachedFilePath(roomId); ok && path == "" {
					t.Errorf("cached path present but empty")
				}
			}
		}(g)
	}
	wg.Wait()
}

// Mixed traffic across every accessor at once, which is closest to real
// runtime: loads writing, GetAllRoomIds ranging, zone teardown deleting, and
// the memory reporter snapshotting.
func TestFileCache_ConcurrentMixedAccessors(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}
	for i := 0; i < 200; i++ {
		rm.setCachedFilePath(i, fmt.Sprintf("zone/%d.yaml", i))
	}

	var wg sync.WaitGroup
	work := []func(int){
		func(i int) { rm.setCachedFilePath(i%200, fmt.Sprintf("zone/%d.yaml", i%200)) },
		func(i int) { rm.setCachedFilePathIfAbsent(500+i, "zone/new.yaml") },
		func(i int) { rm.cachedFilePath(i % 200) },
		func(i int) { rm.cachedFilePathIds() },
		func(i int) { rm.deleteCachedFilePath(1000 + i) },
		func(i int) { rm.cachedFilePathStats() },
		func(i int) {
			rm.rewriteCachedFilePaths([]int{i % 200}, func(p string) string { return p })
		},
	}

	for _, fn := range work {
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(fn func(int)) {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					fn(i)
				}
			}(fn)
		}
	}
	wg.Wait()
}

// setCachedFilePathIfAbsent must decide presence and store under ONE lock.
// A check-then-set split would let two racing first-loads both write.
func TestFileCache_SetIfAbsentKeepsFirstWriter(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}
	const roomId = 7
	rm.setCachedFilePath(roomId, "original/7.yaml")

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rm.setCachedFilePathIfAbsent(roomId, fmt.Sprintf("overwrite-%d/7.yaml", g))
		}(g)
	}
	wg.Wait()

	if got, _ := rm.cachedFilePath(roomId); got != "original/7.yaml" {
		t.Errorf("cached path = %q, want the original entry preserved", got)
	}
}

// The actual reported path: concurrent GetFilePath. This is what
// GetAutoComplete triggers via rooms.LoadRoom under nothing but a read lock.
// Mixes cache hits with misses so both the read and the write path run
// concurrently. Misses walk a non-existent tree and return "" quickly.
func TestFileCache_ConcurrentGetFilePath(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}
	for i := 0; i < 50; i++ {
		rm.setCachedFilePath(i, fmt.Sprintf("zone/%d.yaml", i))
	}

	var wg sync.WaitGroup
	for g := 0; g < 24; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				// Half hit the warm entries, half miss and take the
				// search-then-write path.
				if g%2 == 0 {
					rm.GetFilePath(i)
				} else {
					rm.GetFilePath(100000 + g*50 + i)
				}
			}
		}(g)
	}
	wg.Wait()

	// Warm entries must survive untouched.
	for i := 0; i < 50; i++ {
		if got, ok := rm.cachedFilePath(i); !ok || got != fmt.Sprintf("zone/%d.yaml", i) {
			t.Fatalf("warm entry %d corrupted: %q (present=%v)", i, got, ok)
		}
	}
}

// cachedFilePathIds must hand back a copy. If it leaked the live map's
// backing, a caller ranging it while a load writes would crash.
func TestFileCache_IdsSnapshotIsIndependent(t *testing.T) {
	rm := &RoomManager{roomIdToFileCache: make(map[int]string)}
	rm.setCachedFilePath(1, "zone/1.yaml")

	ids := rm.cachedFilePathIds()
	rm.setCachedFilePath(2, "zone/2.yaml")

	if len(ids) != 1 {
		t.Errorf("snapshot grew from %d to %d after a later write; it is not independent", 1, len(ids))
	}
}
