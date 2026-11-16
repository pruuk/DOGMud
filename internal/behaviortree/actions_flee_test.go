package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A flee out of combat is refused by the player's rules (slice 4a), but
// authored trees flee out of combat: the thief steals on mob_idle and then
// flees. actFlee walks away instead when the mob is not fighting.
func TestActFlee_OutOfCombatWalksAway(t *testing.T) {
	const here, there = 12, 13
	// The walk is quoted first, which resolves the exit through the
	// direction aliases; a test binary has none loaded.
	t.Cleanup(keywords.SeedKeywordsForTest())
	cleanup := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		here:  {RoomId: here, Zone: "test", Exits: map[string]exit.RoomExit{"north": {RoomId: there}}},
		there: {RoomId: there, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: here}}},
	}, map[string]*rooms.ZoneConfig{})
	defer cleanup()

	mob := newTestMob(t)
	mob.Character.RoomId = here
	// Rested enough to pay the step the walk quotes.
	mob.Character.Stamina = 100
	mob.Character.StaminaMax.Value = 100
	mob.Character.Conditions = conditions.New()
	queuedCmds(mob.InstanceId)

	if got := LookupAction("flee")(nil, &EvalContext{InstanceId: mob.InstanceId, RoomId: here}); got != Success {
		t.Fatalf("flee = %v, want Success", got)
	}
	if cmds := queuedCmds(mob.InstanceId); !contains(cmds, "go north") {
		t.Errorf("out-of-combat flee queued %v, want 'go north'", cmds)
	}
}

func TestActFlee_InCombatFlees(t *testing.T) {
	const here = 14
	cleanup := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		here: {RoomId: here, Zone: "test"},
	}, map[string]*rooms.ZoneConfig{})
	defer cleanup()

	target := seedTargetMob(t, 406, here)
	mob := newTestMob(t)
	mob.Character.RoomId = here
	mob.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
	queuedCmds(mob.InstanceId)

	if got := LookupAction("flee")(nil, &EvalContext{InstanceId: mob.InstanceId, RoomId: here}); got != Success {
		t.Fatalf("flee = %v, want Success", got)
	}
	if cmds := queuedCmds(mob.InstanceId); !contains(cmds, "flee") {
		t.Errorf("in-combat flee queued %v, want 'flee'", cmds)
	}
}
