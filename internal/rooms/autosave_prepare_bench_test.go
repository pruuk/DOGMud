package rooms

import (
	"testing"
)

// Acceptance benchmarks for roadmap chunk 3.6b-1.
//
// The claim being checked: after the split, the cost held under the world lock
// is the PREPARE pass, and prepare costs roughly what a CLEAN room save cost
// before -- i.e. a fully dirty world now costs about what a fully clean one
// used to, instead of 14x more.
//
// 3.6a baseline, per room:
//
//	clean save (no file written)   0.26 ms
//	dirty save (whole thing)       3.63 ms
//	of which util.Save             3.46 ms   <- this is what left the lock
//
// Target: prepare <= 0.3 ms/room.
//
//	go test ./internal/rooms/ -bench 'PrepareAll' -benchtime 20x -run '^$'
//	go test ./internal/rooms/ -bench 'RoomPrepare' -benchtime 300x -run '^$'
//
// IMPORTANT: these are warm-page-cache local-SSD numbers, and they are NOT a
// production guarantee. PrepareInstanceWrite calls LoadRoomTemplate, which is an
// UNCACHED disk read and YAML parse per room per cycle and accounts for about
// 64% of prepare. On the droplet with a cold cache this can be materially
// worse, and unlike the durable write it cannot be amortised -- the diff needs
// the template, so it is inside the atomic pass by construction. Template
// caching is a separate slice and is the named prerequisite for this figure
// holding in prod. The AutoSave log line's prepareMs is the real check.

// One dirty room, prepare only. Compare against
// BenchmarkRoomSavePhase_WholeDirtySave (3.63 ms): the difference is the
// durable write that no longer happens under the lock.
func BenchmarkRoomPrepareOnly_Dirty(b *testing.B) {
	_, roomId, restore := benchSetupOneRoom(b)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		b.Fatal("room load returned nil")
	}
	r.Gold = 250

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := PrepareInstanceWrite(*r); err != nil {
			b.Fatal(err)
		}
	}
}

// A clean room prepares a DELETE, so it skips the marshal. This is the floor.
func BenchmarkRoomPrepareOnly_Clean(b *testing.B) {
	_, roomId, restore := benchSetupOneRoom(b)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		b.Fatal("room load returned nil")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := PrepareInstanceWrite(*r); err != nil {
			b.Fatal(err)
		}
	}
}

// The whole-world prepare pass, which is what actually holds the lock.
//
// Against BenchmarkSaveAllRooms_1000Dirty this is the headline number for
// 3.6b-1: the pathological fully-dirty case should now land near the old CLEAN
// cost rather than near the old dirty cost.
func benchPrepareAll(b *testing.B, n int, dirty bool) {
	tempDir := b.TempDir()
	restore := useBenchDataFiles(b, tempDir)
	defer restore()

	cleanup := seedBenchRooms(b, tempDir, n, dirty)

	// Defers run LIFO, so StopTimer fires BEFORE cleanup. That ordering is not
	// cosmetic: cleanup calls removeRoomFromMemory, which calls
	// SaveRoomInstance, so tearing down 1000 DIRTY rooms performs 1000 real
	// durable writes. Go stops the benchmark clock when the function returns,
	// not when the b.N loop ends, so without this the teardown lands inside the
	// measurement.
	//
	// It cost real time to find: it inflated dirty prepare from 0.13ms/room to
	// 0.72ms/room and made prepare look 6x dependent on dirtiness, which is the
	// exact property this sub-chunk claims to remove. A CPU profile showed
	// SaveRoomInstance at 62% of a benchmark that never calls it.
	defer cleanup()
	defer b.StopTimer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := PrepareAllInstanceWrites(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrepareAllInstanceWrites_1000Dirty(b *testing.B) { benchPrepareAll(b, 1000, true) }
func BenchmarkPrepareAllInstanceWrites_1000Clean(b *testing.B) { benchPrepareAll(b, 1000, false) }
