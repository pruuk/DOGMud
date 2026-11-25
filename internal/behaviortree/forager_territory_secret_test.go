package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781, owner ruling 13: the forager's territory roam is a random pick, so it
// takes a secret exit into its territory only once the mob found it.
func TestNpcWanderTerritoryNeighbor_SkipsAnUnfoundSecretExit(t *testing.T) {
	const inst = 7811
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1: {RoomId: 1, Exits: map[string]exit.RoomExit{
			"north": {RoomId: 2, Secret: true},
		}},
		2: {RoomId: 2},
	}, map[string]*rooms.ZoneConfig{}))
	events.DrainQueuedInputsForTest(inst)
	t.Cleanup(func() { events.DrainQueuedInputsForTest(inst) })

	profile := &forager.ForagerProfile{TerritoryRooms: []int{1, 2}}
	mob := &mobs.Mob{InstanceId: inst}
	mob.Character = *characters.New()
	mob.Character.RoomId = 1
	ctx := &EvalContext{InstanceId: inst, RoomId: 1}

	npcWanderTerritoryNeighbor(profile, mob, ctx)
	if got := events.DrainQueuedInputsForTest(inst); len(got) != 0 {
		t.Fatalf("queued %v: the only way into the territory is an unfound secret exit", got)
	}

	mob.Character.LearnSecretExit(1, "north")
	npcWanderTerritoryNeighbor(profile, mob, ctx)
	if got := events.DrainQueuedInputsForTest(inst); len(got) != 1 || got[0] != "north" {
		t.Fatalf("queued %v, want the found north", got)
	}
}
