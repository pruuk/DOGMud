package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// roomIdleFixture: the hooks fixture with room 1 lit and given idle lines,
// RoomIdleChancePct and the budget pinned, a fresh budget, a seeded die.
func roomIdleFixture(t *testing.T, lines []string, pct, budget int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.RoomIdleChancePct = configs.ConfigInt(pct)
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.SetRandForTest(317))
	t.Cleanup(util.ResetRoundCountForTest)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	room.IdleMessages = lines
	for _, id := range []int{1, 2} {
		require.NoError(t, users.GetByUserId(id).Character.Validate())
	}
	drainPlain(1)
	drainPlain(2)
}

func tickRounds(from, to uint64) {
	for r := from; r <= to; r++ {
		util.SetRoundCountForTest(r)
		UserRoundTick(events.NewRound{RoundNumber: r})
	}
}

func TestRoomIdle_SingleLineOncePerVisit(t *testing.T) {
	roomIdleFixture(t, []string{"A cart rattles past."}, 100, 0)
	tickRounds(1000, 1005)
	require.Equal(t, 1, countContaining(drainPlain(2), "cart rattles past"), "a one-line room shows its line once per visit")

	// User 2 walks out and back in: a new visit.
	u2 := users.GetByUserId(2)
	rooms.LoadRoom(1).RemovePlayer(2)
	u2.Character.RoomId = 2
	rooms.LoadRoom(2).AddPlayer(2)
	tickRounds(1006, 1006)
	rooms.LoadRoom(2).RemovePlayer(2)
	u2.Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(2)
	tickRounds(1007, 1010)
	require.Equal(t, 1, countContaining(drainPlain(2), "cart rattles past"), "back in, user 2 sees the line once more")
	require.Equal(t, 1, countContaining(drainPlain(1), "cart rattles past"), "user 1 never left and saw it once in all")
}

func TestRoomIdle_ZeroChanceIsOff(t *testing.T) {
	roomIdleFixture(t, []string{"A cart rattles past.", "A dog barks.", "Bells ring."}, 0, 0)
	tickRounds(1000, 1050)
	got := drainPlain(2)
	require.Zero(t, countContaining(got, "cart rattles")+countContaining(got, "dog barks")+countContaining(got, "bells ring"),
		"RoomIdleChancePct 0 must show no idle line: %v", got)
}

func TestRoomIdle_BudgetCapsALongStay(t *testing.T) {
	lines := []string{"A cart rattles past.", "A dog barks.", "Bells ring."}
	count := func(budget int) int {
		roomIdleFixture(t, lines, 100, budget)
		tickRounds(1000, 1009)
		got := drainPlain(2)
		return countContaining(got, "cart rattles") + countContaining(got, "dog barks") + countContaining(got, "bells ring")
	}
	free := count(0)
	capped := count(5)
	require.GreaterOrEqual(t, free, 3, "without a budget ten rounds at 100%% show several lines")
	require.LessOrEqual(t, capped, 2, "a 5-round budget allows at most two lines in ten rounds")
	require.GreaterOrEqual(t, capped, 1)
}

func TestRoomIdleChancePer10k(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.RoomIdleChancePct = 3
	configs.SetConfigForTest(t, cfg)
	room := rooms.LoadRoom(1)
	require.Equal(t, 300, roomIdleChancePer10k(room))
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(2)
	require.Equal(t, 600, roomIdleChancePer10k(room), "zone ambientmultiplier 2.0 doubles the chance")
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(0)
	require.Equal(t, 0, roomIdleChancePer10k(room), "zone ambientmultiplier 0 hushes room idle lines")
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = nil
	limbo := &rooms.Room{RoomId: -1, Zone: "TestZone"}
	require.Equal(t, 1200, roomIdleChancePer10k(limbo), "room -1 keeps four times the chance")
}
