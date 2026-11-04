package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/facts"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
	"github.com/stretchr/testify/require"
)

const hintTimerMobId = 9903

// hintTimerFixture is a gossiper who knows one ordinary fact and, when
// withHint, one fact tagged hint, in room 4242, with GossipIntervalRounds high
// enough that ordinary gossip stays out of the way after its first line.
func hintTimerFixture(t *testing.T, hintInterval int, withHint bool) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.GossipIntervalRounds = 100000
	cfg.Balance.GossipHintIntervalRounds = configs.ConfigInt(hintInterval)
	configs.SetConfigForTest(t, cfg)

	tmp := t.TempDir()
	prev := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev})
	})
	facts.ResetForTest()
	t.Cleanup(facts.ResetForTest)
	worldevents.ResetForTest()
	t.Cleanup(worldevents.ResetForTest)
	t.Cleanup(gossip.SeedForTest(map[string][]string{
		"fact-trail":   {"HINT {description}"},
		"fact-default": {"OTHER {description}"},
		"fallback":     {"Quiet day."},
	}))

	declare := func(id string, tags ...string) {
		require.NoError(t, facts.Declare(id, facts.DeclareOpts{
			Description: "something happened.", Significance: worldevents.Local, Zone: "TestZone", Tags: tags,
		}))
		facts.RecordKnowsFact(hintTimerMobId, id, facts.SourceWitnessed)
	}
	declare("timer-other")
	if withHint {
		declare("timer-hint", "trail", facts.HintTag)
	}

	room := &rooms.Room{RoomId: 4242, Zone: "TestZone"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{4242: room}, nil))
	g := &mobs.Mob{
		MobId: hintTimerMobId, InstanceId: 201, HomeRoomId: 4242, ActivityLevel: 7,
		Groups: []string{"gossiper"},
		Character: characters.Character{
			Name: "old Fen", Zone: "TestZone", RoomId: 4242, Health: 50,
			Conditions: conditions.New(), Cooldowns: map[string]int{},
		},
	}
	g.Character.HealthMax.Value = 100
	if withHint {
		// Ordinary gossip can draw the hint fact too; hold it back so these
		// tests see only the hint timer.
		g.SetTempData("lastGossipRound", max(util.GetRoundCount(), 1))
	}
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{201: g}))
	events.DrainQueuedInputsForTest(201)
	return g, room
}

func spokeHint(in []string) bool {
	for _, line := range in {
		if strings.HasPrefix(line, "ambient gossip say HINT ") {
			return true
		}
	}
	return false
}

func TestGossipHint_SpokenToAListenerThenWaitsTheInterval(t *testing.T) {
	g, room := hintTimerFixture(t, 150, true)
	room.AddPlayer(1)

	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	require.True(t, spokeHint(events.DrainQueuedInputsForTest(201)), "a first listener hears a hint")

	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	require.False(t, spokeHint(events.DrainQueuedInputsForTest(201)), "no second hint inside the interval")

	g.SetTempData("lastGossipHintRound", util.GetRoundCount()-150)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	require.True(t, spokeHint(events.DrainQueuedInputsForTest(201)), "a hint again once the interval has passed")
}

func TestGossipHint_NotSpentOnAnEmptyRoom(t *testing.T) {
	_, room := hintTimerFixture(t, 150, true)

	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	require.False(t, spokeHint(events.DrainQueuedInputsForTest(201)), "nobody here, so no hint")

	room.AddPlayer(1)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	require.True(t, spokeHint(events.DrainQueuedInputsForTest(201)), "the timer waits for someone to hear it")
}

func TestGossipHint_ZeroIsOff(t *testing.T) {
	_, room := hintTimerFixture(t, 0, true)
	room.AddPlayer(1)
	for i := 0; i < 20; i++ {
		HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
		require.False(t, spokeHint(events.DrainQueuedInputsForTest(201)), "0 turns the hint timer off")
	}
}

func TestGossipHint_NoHintFactMeansOrdinaryGossip(t *testing.T) {
	_, room := hintTimerFixture(t, 150, false)
	room.AddPlayer(1)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 201})
	got := events.DrainQueuedInputsForTest(201)
	require.False(t, spokeHint(got))
	require.Contains(t, strings.Join(got, "|"), "ambient gossip say OTHER ")
}
