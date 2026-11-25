package planners

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781, owner ruling 13: the survival planner's flee exit is the mob's own
// choice, so a secret exit is one only once the mob found it. mobs.Flee
// honours a named exit without asking whether it is secret.
func TestPickRandomExit_TakesOnlyAFoundSecretExit(t *testing.T) {
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1: {RoomId: 1, Exits: map[string]exit.RoomExit{
			"north": {RoomId: 2, Secret: true},
		}},
		2: {RoomId: 2},
	}, map[string]*rooms.ZoneConfig{}))
	mob := &mobs.Mob{}
	mob.Character.RoomId = 1

	for range 50 {
		if got := pickRandomExit(mob); got != "" {
			t.Fatalf("picked %q: the only exit is an unfound secret", got)
		}
	}
	mob.Character.LearnSecretExit(1, "north")
	if got := pickRandomExit(mob); got != "north" {
		t.Fatalf("picked %q, want the found north", got)
	}
}
