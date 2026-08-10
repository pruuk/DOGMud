package rooms

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// Phase breakdown of one room's autosave cost, for roadmap chunk 3.6a.
//
// The whole-set benchmarks showed a dirty room costs ~3.56ms against ~0.26ms
// for a clean one. That is the number 3.6b has to be designed against, but only
// if we know WHICH phase owns it, because each phase has a different escape:
//
//   loadTemplate  — a disk read per room, per save, of IMMUTABLE authored
//                   content. Escapable by caching (no lock implications).
//   diff          — reflection over every exported field. Escapable by dirty
//                   tracking. Must stay under the lock: it reads live state.
//   marshal       — turning the diff into bytes. Must stay under the lock for
//                   the same reason, but produces an IMMUTABLE []byte.
//   write         — util.Save: temp file, fsync, rename. Escapable from the
//                   lock entirely, because bytes are already immutable.
//
// If write dominates, moving it outside the lock caps the pause regardless of
// how many rooms are dirty. If marshal or diff dominates, that buys much less
// and the target is different.
//
// Run:
//   go test ./internal/rooms/ -bench 'RoomSavePhase' -benchtime 200x -run '^$'

// benchInstancePayload is a representative instance overlay: what a room with
// dropped gold and a couple of items actually persists.
func benchInstancePayload() map[string]interface{} {
	return map[string]interface{}{
		"roomid": 900001,
		"gold":   250,
		"items": []map[string]interface{}{
			{"itemid": 20008, "uses": 3},
			{"itemid": 20012},
		},
	}
}

func benchSetupOneRoom(b *testing.B) (tempDir string, roomId int, restore func()) {
	b.Helper()
	tempDir = b.TempDir()
	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        tempDir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		b.Fatal(err)
	}

	roomId = 900001
	tpl := &Room{RoomId: roomId, Zone: "benchzone", Title: "Bench Room", Description: "Phase benchmark room."}
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
	if err := os.MkdirAll(filepath.Join(tempDir, "rooms.instances", "benchzone"), 0o755); err != nil {
		b.Fatal(err)
	}

	return tempDir, roomId, func() {
		delete(roomManager.roomIdToFileCache, roomId)
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	}
}

// Phase 1: the per-room template re-read. Pure disk I/O on immutable data.
func BenchmarkRoomSavePhase_LoadTemplate(b *testing.B) {
	_, roomId, restore := benchSetupOneRoom(b)
	defer restore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if r := LoadRoomTemplate(roomId); r == nil {
			b.Fatal("template load returned nil")
		}
	}
}

// Phase 3: marshalling the computed overlay into bytes.
func BenchmarkRoomSavePhase_Marshal(b *testing.B) {
	payload := benchInstancePayload()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := yaml.Marshal(payload); err != nil {
			b.Fatal(err)
		}
	}
}

// Phase 4: the durable write. This is the phase that could move outside the
// world lock, because its input is already immutable bytes.
func BenchmarkRoomSavePhase_DurableWrite(b *testing.B) {
	tempDir, _, restore := benchSetupOneRoom(b)
	defer restore()

	data, err := yaml.Marshal(benchInstancePayload())
	if err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(tempDir, "rooms.instances", "benchzone", "900001.yaml")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := util.Save(path, data, true); err != nil {
			b.Fatal(err)
		}
	}
}

// Control: the whole SaveRoomInstance for one dirty room, so the phases can be
// checked against the total they are meant to explain.
func BenchmarkRoomSavePhase_WholeDirtySave(b *testing.B) {
	_, roomId, restore := benchSetupOneRoom(b)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		b.Fatal("room load returned nil")
	}
	r.Gold = 250

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := SaveRoomInstance(*r); err != nil {
			b.Fatal(err)
		}
	}
}
