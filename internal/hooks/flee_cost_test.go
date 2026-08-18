package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Catches the round hook hardcoding includeSkill=true instead of carrying the
// command's short-payment decision into blocker resolution.
func TestHandlePlayerFlee_ShortCommandDoesNotProgressSkullduggery(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)

	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	room.Exits = nil // keep the full-skill mutation away from Look infrastructure
	blocker := mobs.GetInstance(100)
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.Stats.Dexterity.ValueAdj = 1
	u.Character.Skills = map[string]int{string(skills.Skullduggery): 100}
	blocker.Character.Stats.Dexterity.ValueAdj = 100
	blocker.Character.SetAggro(u.UserId, 0, characters.DefaultAttack)
	u.Character.SetAggro(0, blocker.InstanceId, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()

	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatalf("fixture did not enter Disengaging")
	}
	if !handlePlayerFlee(u, room, u.UserId) {
		t.Fatal("short flee was not resolved")
	}
	if got := u.Character.GetSkillUseCount(string(skills.Skullduggery)); got != 0 {
		t.Errorf("short flee progressed Skullduggery %d times, want 0", got)
	}
}
