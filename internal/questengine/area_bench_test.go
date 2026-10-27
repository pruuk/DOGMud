package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// The room_enter benchmark (#375 phase 3b slice 4, spec bar: the per-call
// cost of Notify("room_enter") for shipped content rises no more than 10%).
//
//	go test ./internal/questengine/ -run x -bench NotifyRoomEnter -count=5
//
// "shipped" registers every shipped quest and nothing else, so it runs the
// same on a tree without areas (master before slice 4): copy this file
// alone into such a tree to get the baseline. The "areas50" cases add 50
// synthetic room_enter triggers scoped to one 30-room area; they need
// area_bench_areas_test.go, which sets benchAreaTriggers, and are skipped
// without it.

// benchAreaTriggers, when set, installs the 30-room bench area for the
// benchmark's lifetime, registers 50 area-scoped room_enter triggers on e,
// and returns a room inside the area and the restore func.
var benchAreaTriggers func(e *Engine) (insideRoom int, restore func())

// benchOutsideRoom is a room no shipped room_enter trigger and no bench area
// names.
const benchOutsideRoom = 1

func benchShippedEngine(b *testing.B) *Engine {
	b.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("no caller")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", "*.yaml"))
	if err != nil || len(files) == 0 {
		b.Fatalf("no shipped quests found: %v", err)
	}
	e := NewEngine()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		q := &QuestDef{}
		if err := yaml.Unmarshal(data, q); err != nil {
			b.Fatalf("%s: %v", f, err)
		}
		if q.QuestId > 0 {
			e.RegisterQuest(q)
		}
	}
	b.Logf("registered %d shipped quests, %d room_enter triggers", len(e.quests), len(e.triggerIndex["room_enter"]))
	return e
}

func benchRoomEnter(b *testing.B, e *Engine, roomId int) {
	p := newFullMockPlayer(roomId)
	ctx := newFullMockActionContext(1, p)
	d := EventDetails{UserId: 1, RoomId: roomId}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Notify("room_enter", d, p, ctx)
	}
}

func BenchmarkNotifyRoomEnter(b *testing.B) {
	b.Run("shipped", func(b *testing.B) {
		benchRoomEnter(b, benchShippedEngine(b), benchOutsideRoom)
	})
	for _, where := range []string{"outside", "inside"} {
		b.Run("areas50/"+where, func(b *testing.B) {
			if benchAreaTriggers == nil {
				b.Skip("no areas in this tree")
			}
			e := benchShippedEngine(b)
			inside, restore := benchAreaTriggers(e)
			defer restore()
			room := benchOutsideRoom
			if where == "inside" {
				room = inside
			}
			benchRoomEnter(b, e, room)
		})
	}
}
