package hooks

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// idleMobFixture: the hooks fixture's Skeleton (instance 100, room 1,
// TestZone) with the given activity level and pool and the two idle scales
// pinned.
func idleMobFixture(t *testing.T, al int, pool []string, chatter, movement float64) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.MobIdleChatterScale = configs.ConfigFloat(chatter)
	cfg.Balance.MobIdleMovementScale = configs.ConfigFloat(movement)
	configs.SetConfigForTest(t, cfg)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.ActivityLevel = al
	m.IdleCommands = pool
	events.DrainQueuedInputsForTest(100)
	return m
}

func idleInputs(rounds int) []string {
	var all []string
	for i := 0; i < rounds; i++ {
		HandleIdleMobs(events.MobIdle{MobInstanceId: 100})
		all = append(all, events.DrainQueuedInputsForTest(100)...)
	}
	return all
}

func TestHandleIdleMobs_IdleEmoteGoesOutAsAmbient(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote rattles its bones."}, 1, 1)
	got := idleInputs(1)
	require.Contains(t, got, "ambient mob_chatter emote rattles its bones.")
	require.NotContains(t, got, "lookfortrouble", "a round that picked a command does not also look for trouble")
}

func TestHandleIdleMobs_SemicolonStaysInOneLine(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote plunges the iron in the trough; steam rises."}, 1, 1)
	got := idleInputs(1)
	require.Contains(t, got, "ambient mob_chatter emote plunges the iron in the trough; steam rises.")
	for _, in := range got {
		require.False(t, strings.HasPrefix(strings.TrimSpace(in), "steam"), "the text after ';' ran as its own command: %v", got)
	}
}

func TestHandleIdleMobs_OtherChatterVerbsAreNotWrapped(t *testing.T) {
	idleMobFixture(t, 100, []string{"consume corpse"}, 1, 1)
	require.Contains(t, idleInputs(1), "consume corpse")
}

func TestHandleIdleMobs_ChatterScaleZeroSilences(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote rattles its bones.", "say Boo."}, 0, 1)
	got := idleInputs(200)
	for _, in := range got {
		require.False(t, strings.HasPrefix(in, "ambient "), "MobIdleChatterScale 0 must silence idle chatter, got %q", in)
	}
	require.Equal(t, 200, countExact(got, "lookfortrouble"), "every silent round looks for trouble")
}

func TestHandleIdleMobs_MovementRollsApartFromChatter(t *testing.T) {
	idleMobFixture(t, 100, []string{"wander"}, 0, 1)
	require.Contains(t, idleInputs(1), "wander", "chatter at 0 must not stop movement")

	idleMobFixture(t, 100, []string{"wander"}, 1, 0)
	require.NotContains(t, idleInputs(200), "wander", "MobIdleMovementScale 0 must stop idle movement")
}

func TestHandleIdleMobs_WanderMultipliers(t *testing.T) {
	m := idleMobFixture(t, 100, []string{"wander"}, 1, 1)
	m.WanderMultiplier = rooms.MultiplierPtr(0)
	require.NotContains(t, idleInputs(200), "wander", "mob wandermultiplier 0 must stop its idle movement")

	idleMobFixture(t, 100, []string{"wander"}, 1, 1)
	rooms.GetZoneConfig("TestZone").WanderMultiplier = rooms.MultiplierPtr(0)
	require.NotContains(t, idleInputs(200), "wander", "zone wandermultiplier 0 must stop idle movement there")

	idleMobFixture(t, 100, []string{"emote hums."}, 1, 1)
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(0)
	for _, in := range idleInputs(200) {
		require.False(t, strings.HasPrefix(in, "ambient "), "zone ambientmultiplier 0 must silence idle chatter, got %q", in)
	}
}

// The idle roll only queues a movement command; whether the mob changes
// room is decided when the command runs (mobcommands counts it there). So
// the roll itself counts no move, and a sneak, which rides the movement
// roll but hides in place, is never counted at all.
func TestHandleIdleMobs_RollCountsNoMove(t *testing.T) {
	for _, verb := range []string{"sneak", "wander"} {
		t.Run(verb, func(t *testing.T) {
			idleMobFixture(t, 100, []string{verb}, 1, 1)
			cfg := configs.GetConfig()
			cfg.Balance.AmbientTelemetry = true
			configs.SetConfigForTest(t, cfg)
			t.Cleanup(ambient.ResetForTest())

			require.Contains(t, idleInputs(1), verb, "fixture premise: the roll picked %s", verb)
			require.Empty(t, ambient.MovesForTest(1), "the idle roll must not count a move")
		})
	}
}

func countExact(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

func gossiperFixture(t *testing.T, interval int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.GossipIntervalRounds = configs.ConfigInt(interval)
	configs.SetConfigForTest(t, cfg)
	g := &mobs.Mob{
		MobId: 114, InstanceId: 200, HomeRoomId: 1, ActivityLevel: 7,
		Groups: []string{"humanoid", "gossiper"},
		Character: characters.Character{
			Name: "old Fen", Zone: "TestZone", RoomId: 1, Health: 50,
			Conditions: conditions.New(), Cooldowns: map[string]int{},
		},
	}
	g.Character.HealthMax.Value = 100
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{200: g}))
	t.Cleanup(gossip.SeedForTest(map[string][]string{"fallback": {"Quiet day."}}))
	events.DrainQueuedInputsForTest(200)
}

func TestHandleIdleMobs_GossipGoesOutAsAmbient(t *testing.T) {
	gossiperFixture(t, 75)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 200})
	got := events.DrainQueuedInputsForTest(200)
	require.True(t, slices.Contains(got, "ambient gossip say Quiet day."), "gossip must go through the ambient command, got %v", got)
}

func TestHandleIdleMobs_GossipZeroIsOff(t *testing.T) {
	gossiperFixture(t, 0)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 200})
	for _, in := range events.DrainQueuedInputsForTest(200) {
		require.NotContains(t, in, "Quiet day", "GossipIntervalRounds 0 must stop gossip")
	}
}
