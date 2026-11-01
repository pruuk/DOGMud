package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// pinStealMinRank sets Balance.StealMinRank for one test. Call it after any
// other config pin (pinStealCooldown, setupHousehold): it copies the config
// as it stands.
func pinStealMinRank(t *testing.T, rank int) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.StealMinRank = configs.ConfigInt(rank)
	configs.SetConfigForTest(t, c)
}

// The theft rank gate is Balance.StealMinRank (#375 phase 7), the same for a
// player and a mob thief. At the shipped 1 a fresh character can try; at 2
// the same character is refused.
func TestSteal_MinRankComesFromConfig(t *testing.T) {
	t.Run("player at the default", func(t *testing.T) {
		actor := newStealPlayerActor(100, 1)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.NotEqual(t, "not advanced enough", res.Reason, "rank 1 passes the shipped gate: %+v", res)
	})
	t.Run("player below a raised gate", func(t *testing.T) {
		pinStealMinRank(t, 2)
		actor := newStealPlayerActor(100, 1)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.Equal(t, "not advanced enough", res.Reason)
	})
	t.Run("mob thief at the default", func(t *testing.T) {
		mark := users.NewTestUser(3021, "mark", "Mark", 0)
		mark.Character.Gold = 50
		defer users.SeedUsersForTest(map[int]*users.UserRecord{3021: mark})()
		actor := newStealMobActor(100, 1)
		res := Steal(actor, StealOptions{TargetUserId: 3021})
		require.NotEqual(t, "not advanced enough", res.Reason, "a rank-1 mob passes the shipped gate: %+v", res)
	})
	t.Run("mob thief below a raised gate", func(t *testing.T) {
		pinStealMinRank(t, 3)
		mark := users.NewTestUser(3022, "mark", "Mark", 0)
		mark.Character.Gold = 50
		defer users.SeedUsersForTest(map[int]*users.UserRecord{3022: mark})()
		actor := newStealMobActor(100, 2)
		res := Steal(actor, StealOptions{TargetUserId: 3022})
		require.Equal(t, "not advanced enough", res.Reason)
	})
}
