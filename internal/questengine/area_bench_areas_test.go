package questengine

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/areas"
)

// The area half of BenchmarkNotifyRoomEnter (area_bench_test.go): kept in
// its own file so the benchmark file alone builds on a tree without areas.
func init() {
	benchAreaTriggers = func(e *Engine) (int, func()) {
		const first, size = 990001, 30
		zones := map[int]string{}
		ids := make([]int, 0, size)
		for id := first; id < first+size; id++ {
			zones[id] = "Bench Zone"
			ids = append(ids, id)
		}
		restore := areas.InstallForTest([]areas.Area{{Key: "bench_area", Name: "the bench area", Rooms: ids}}, zones)
		q := &QuestDef{QuestId: 990000, Name: "Bench", Steps: []QuestStep{{Id: "start"}, {Id: "end"}}}
		for i := 0; i < 50; i++ {
			step := fmt.Sprintf("s%d", i)
			q.Triggers = append(q.Triggers, TriggerDef{
				Event: "room_enter", Area: "bench_area",
				Conditions: Conditions{Has: []string{"990000-start"}, Missing: []string{"990000-" + step}},
				Actions:    []ActionDef{{Grant: "990000-" + step}},
			})
		}
		e.RegisterQuest(q)
		return first + size/2, restore
	}
}
