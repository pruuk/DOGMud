package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: flee_blocker_sight_test.go
//
// A blocked flee sends the fleeing player their OWN line naming the blocker.
// The room line beside it already hides names by sight; the player's own line
// must read the blocker at the fleer's sight too.

// blockedFleeLines arms a flee the mob blocker is certain to win and returns
// the plain lines the fleeing player read. dark selects a pitch-dark room
// versus one pinned fully lit.
func blockedFleeLines(t *testing.T, dark bool) []string {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	room.Exits = nil
	if dark {
		darken(t, 1)
	} else {
		room.Lamp = rooms.LampPtr(90)
	}
	blocker := mobs.GetInstance(100)
	require.NotNil(t, blocker)
	require.NoError(t, u.Character.Validate())
	u.Character.StaminaMax.Value = 1000
	u.Character.Stamina = 1000
	u.Character.Stats.Dexterity.ValueAdj = 1
	blocker.Character.Stats.Dexterity.ValueAdj = 100
	blocker.Character.SetAggro(u.UserId, 0, characters.DefaultAttack)
	u.Character.SetAggro(0, blocker.InstanceId, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()

	_, err := usercommands.Flee("", u, room, 0)
	require.NoError(t, err)
	require.True(t, u.Character.IsDisengaging(), "fixture did not enter Disengaging")
	drainPlain(1)
	require.True(t, handlePlayerFlee(u, room, u.UserId))
	return drainPlain(1)
}

func TestBlockedFlee_DarkFleerDoesNotReadBlockerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	lines := blockedFleeLines(t, true)
	require.NotZero(t, countContaining(lines, "from fleeing"), "fixture must produce the blocked line: %v", lines)
	require.Zero(t, countContaining(lines, "Skeleton"), "dark fleer must not read the blocker's name: %v", lines)
}

func TestBlockedFlee_ClearSightFleerReadsBlockerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	lines := blockedFleeLines(t, false)
	require.NotZero(t, countContaining(lines, "Skeleton blocks you from fleeing"), "lit fleer should read the blocker's name: %v", lines)
}
