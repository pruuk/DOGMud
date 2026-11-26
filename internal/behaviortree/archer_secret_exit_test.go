package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781 owner ruling 13, plan D35: an archer chooses its shot, so it fires
// across a secret exit only once it has found it. Before that the target is
// out of its line of fire and try_fire fails over to melee.
func TestActTryFire_ASecretExitOnlyOnceFound(t *testing.T) {
	const here, there = 10, 11
	seedLitRooms(t, map[int]*rooms.Room{
		here: {RoomId: here, Zone: "test", Exits: map[string]exit.RoomExit{
			"north": {RoomId: there, Secret: true},
		}},
		there: {RoomId: there, Zone: "test", Exits: map[string]exit.RoomExit{
			"south": {RoomId: here},
		}},
	})
	target := seedTargetMob(t, 421, there)

	mob := newTestMob(t)
	mob.Character.RoomId = here
	mob.Character.Equipment.Weapon = rangedBow(true)
	mob.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
	queuedCmds(mob.InstanceId)
	ctx := &EvalContext{InstanceId: mob.InstanceId, RoomId: here}

	if got := LookupAction("try_fire")(nil, ctx); got != Failure {
		t.Fatalf("try_fire through an unfound secret exit = %v, want Failure", got)
	}
	if cmds := queuedCmds(mob.InstanceId); len(cmds) != 0 {
		t.Fatalf("nothing may be queued, got %v", cmds)
	}

	mob.Character.LearnSecretExit(here, "north")
	if got := LookupAction("try_fire")(nil, ctx); got != Success {
		t.Fatalf("try_fire through a found secret exit = %v, want Success", got)
	}
	if cmds := queuedCmds(mob.InstanceId); !contains(cmds, "shoot Target north") {
		t.Errorf("expected 'shoot Target north' queued, got %v", cmds)
	}
}

// The exit word the archer queues is one `shoot` matches: a temporary exit's
// key, never its title (a rift titled "glade-7 rift" is matched as glade-7).
func TestActTryFire_ATemporaryExitIsNamedByItsKey(t *testing.T) {
	const here, there = 10, 11
	seedLitRooms(t, map[int]*rooms.Room{
		here: {RoomId: here, Zone: "test", ExitsTemp: map[string]exit.TemporaryRoomExit{
			"glade-7": {RoomId: there, Title: "glade-7 rift"},
		}},
		there: {RoomId: there, Zone: "test"},
	})
	target := seedTargetMob(t, 422, there)

	mob := newTestMob(t)
	mob.Character.RoomId = here
	mob.Character.Equipment.Weapon = rangedBow(true)
	mob.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
	queuedCmds(mob.InstanceId)

	ctx := &EvalContext{InstanceId: mob.InstanceId, RoomId: here}
	if got := LookupAction("try_fire")(nil, ctx); got != Success {
		t.Fatalf("try_fire through a temporary exit = %v, want Success", got)
	}
	if cmds := queuedCmds(mob.InstanceId); !contains(cmds, "shoot Target glade-7") {
		t.Errorf("expected 'shoot Target glade-7' queued, got %v", cmds)
	}
}
