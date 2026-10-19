package rooms

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// A room spawn entry's conditionids take effect at spawn, like a mob
// template's. Prepare set them with SetPermanentConditions after NewMobById
// had already run its Validate(true), so a room entry carrying condition 9
// spawned a visible ambusher until something else refreshed its conditions.
func TestARoomSpawnEntrysConditionsApplyAtSpawn(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
	const mobId = 7851
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		mobId: {MobId: mobId, Zone: "TestZone", Character: characters.Character{Name: "Lurker"}},
	}, map[int]*mobs.Mob{}))

	room := &Room{RoomId: 7852, SpawnInfo: []SpawnInfo{{MobId: mobId, ConditionIds: []int{9}}}}
	room.Prepare(false)

	ids := room.GetMobs()
	if len(ids) != 1 {
		t.Fatalf("the room spawned %d mobs, want 1", len(ids))
	}
	m := mobs.GetInstance(ids[0])
	if m == nil {
		t.Fatal("the spawned instance is missing")
	}
	t.Cleanup(func() { mobs.SetInstanceForTest(ids[0], nil) })

	if len(m.Character.GetConditions(9)) == 0 {
		t.Error("the room entry's condition 9 is not on the spawned mob")
	}
	if !m.Character.IsHidden() {
		t.Error("a room spawn entry with conditionids [9] must spawn a hidden mob")
	}
}

// A spawn entry's message is narrated when its mob RESPAWNS in front of
// players (#620: "A fresh straw dummy is hauled out" in the Drill Yard was
// parsed and never sent). A first spawn stays quiet.
func TestARoomSpawnEntrysMessageNarratesARespawn(t *testing.T) {
	const mobId = 7853
	const line = "A fresh straw dummy is hauled out and set on the pivot, ready for work."
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		mobId: {MobId: mobId, Zone: "TestZone", Character: characters.Character{Name: "Training Dummy"}},
	}, map[int]*mobs.Mob{}))

	for _, c := range []struct {
		name      string
		despawned uint64
		want      []string
	}{
		{"respawn", 10, []string{line}},
		{"first spawn", 0, []string{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := sightTestRoom(t, "city")
			r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
			r.SpawnInfo = []SpawnInfo{{MobId: mobId, Message: line, RespawnRate: "5 rounds", DespawnedRound: c.despawned}}
			r.Prepare(false)
			ids := r.GetMobs()
			if len(ids) != 1 {
				t.Fatalf("the room spawned %d mobs, want 1", len(ids))
			}
			t.Cleanup(func() { mobs.SetInstanceForTest(ids[0], nil) })

			if got := sightTestPlain(events.DrainQueuedMessagesForTest(7411)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("player read %q, want %q", got, c.want)
			}
		})
	}
}
