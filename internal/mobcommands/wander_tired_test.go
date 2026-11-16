package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// wholeStepCost makes one step cost several whole stamina. Test binaries load
// the Go default base (0.5), which prices a step under 1: that is carried as a
// fraction and a mob with an empty pool still walks. The shipped base is 1.5.
func wholeStepCost(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.MovementBaseStaminaCost = 5
	configs.SetConfigForTest(t, cfg)
}

// An exhausted wanderer does not count a wander it cannot walk, so it is not
// sent home early for steps it never took. The rested control proves the
// fixture reaches the counting branch at all (Wander only counts a step into
// the mob's own zone).
func TestWander_ExhaustedMobDoesNotCount(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stamina   int
		wantCount int
	}{
		{"rested control counts the step", 100, 1},
		{"exhausted mob does not", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			wholeStepCost(t)
			mob, room := getTestMobAndRoom(t)

			north, ok := room.Exits["north"]
			require.True(t, ok, "fixture premise: room 1 has a north exit")
			dest := rooms.LoadRoom(north.RoomId)
			require.NotNil(t, dest)
			origExits := room.Exits
			room.Exits = map[string]exit.RoomExit{"north": north}
			t.Cleanup(func() { room.Exits = origExits })
			mob.Character.Zone = dest.Zone

			mob.MaxWander = -1
			mob.WanderCount = 0
			mob.Character.StaminaMax.Value = 100
			mob.Character.Stamina = tc.stamina

			handled, err := Wander("", mob, room)
			require.True(t, handled)
			require.NoError(t, err)
			require.Equal(t, tc.wantCount, mob.WanderCount)
		})
	}
}
