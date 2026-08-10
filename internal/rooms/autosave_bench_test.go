package rooms

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// Autosave cost benchmarks for roadmap chunk 3.6a (finding 36).
//
// Autosave runs inside the NewTurn event with the global world lock held, so
// its duration is time during which no player acts and no round advances. These
// benchmarks separate the two costs that make up that pause, because they scale
// very differently and the obvious assumption about them is wrong:
//
//   CLEAN room  — the room matches its template, so SaveRoomInstance computes
//                 the reflection diff, finds nothing to persist, and writes NO
//                 file. Cost is CPU only.
//   DIRTY room  — the room has instance state (dropped gold here), so the diff
//                 is non-empty and the room writes a file through the durable
//                 path, including the fsync added in chunk 2.1.
//
// Run:
//   go test ./internal/rooms/ -bench SaveAllRooms -benchtime 10x -run '^$'
//
// The ratio between the two is what decides whether dirty-tracking would help.

// seedBenchRooms creates n template rooms on disk, loads them into the room
// manager, and optionally makes each one dirty. Returns a cleanup func.
func seedBenchRooms(b *testing.B, tempDir string, n int, dirty bool) func() {
	b.Helper()

	ids := make([]int, 0, n)
	for i := 0; i < n; i++ {
		roomId := 900000 + i
		tpl := &Room{
			RoomId:      roomId,
			Zone:        "benchzone",
			Title:       "Bench Room",
			Description: "A room used to measure autosave cost.",
		}
		data, err := yaml.Marshal(tpl)
		if err != nil {
			b.Fatal(err)
		}
		templatePath := filepath.Join(tempDir, "rooms", "benchzone", fmt.Sprintf("%d.yaml", roomId))
		if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(templatePath, data, 0o644); err != nil {
			b.Fatal(err)
		}
		roomManager.roomIdToFileCache[roomId] = fmt.Sprintf("benchzone/%d.yaml", roomId)

		r := LoadRoomInstance(roomId)
		if r == nil {
			b.Fatalf("failed to load seeded room %d", roomId)
		}
		if dirty {
			// Gold is instance state, so the diff is non-empty and the room
			// actually writes a file.
			r.Gold = 100 + i
		}
		if err := addRoomToMemory(r); err != nil {
			b.Fatalf("addRoomToMemory: %v", err)
		}
		ids = append(ids, roomId)
	}

	if err := os.MkdirAll(filepath.Join(tempDir, "rooms.instances", "benchzone"), 0o755); err != nil {
		b.Fatal(err)
	}

	return func() {
		for _, id := range ids {
			if r := getRoomFromMemory(id); r != nil {
				removeRoomFromMemory(r)
			}
			delete(roomManager.roomIdToFileCache, id)
		}
	}
}

func benchSaveAllRooms(b *testing.B, n int, dirty bool) {
	tempDir := b.TempDir()
	restore := useBenchDataFiles(b, tempDir)
	defer restore()

	cleanup := seedBenchRooms(b, tempDir, n, dirty)

	// StopTimer before cleanup (defers run LIFO). removeRoomFromMemory calls
	// SaveRoomInstance, so tearing down 1000 dirty rooms performs 1000 real
	// durable writes, and Go stops the clock at function return rather than at
	// the end of the b.N loop. Fixed 2026-08-10 alongside the 3.6b-1 prepare
	// benchmarks, where the same flaw inflated a number by 6x.
	defer cleanup()
	defer b.StopTimer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := SaveAllRooms(); err != nil {
			b.Fatal(err)
		}
	}
}

// The realistic steady state: the whole world loaded, almost nothing changed.
func BenchmarkSaveAllRooms_1000Clean(b *testing.B) { benchSaveAllRooms(b, 1000, false) }

// The worst case: every loaded room has instance state and writes a file
// through the durable path. This is the number that decides whether autosave
// needs dirty tracking.
func BenchmarkSaveAllRooms_1000Dirty(b *testing.B) { benchSaveAllRooms(b, 1000, true) }

// Smaller sets, to confirm the cost is linear in the loaded-set size.
func BenchmarkSaveAllRooms_100Clean(b *testing.B) { benchSaveAllRooms(b, 100, false) }
func BenchmarkSaveAllRooms_100Dirty(b *testing.B) { benchSaveAllRooms(b, 100, true) }

// useBenchDataFiles is the benchmark twin of useTempDataFiles (careful_save_test.go),
// which takes *testing.T.
func useBenchDataFiles(b *testing.B, tempDir string) func() {
	b.Helper()
	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        tempDir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		b.Fatal(err)
	}
	return func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	}
}
