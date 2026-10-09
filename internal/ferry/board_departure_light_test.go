package ferry

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

const (
	boardLightDock  = 9851
	boardLightDeck  = 9852
	boardLightOther = 9853
	boardLightCond  = 9854 // a carried light, literal strength 60
)

// #456 review G3, owner ruling 2026-10-09: a mover is seen by the light they
// carry on their own way out. Aliceia carries the only light on a dark dock
// and boards; Bobrick, left in the dark, saw her cross the gangplank by her
// own light. The line was judged after the move, when the light had left.
func TestBoard_GangplankLineIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		boardLightCond: {ConditionId: boardLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	dock := &rooms.Room{RoomId: boardLightDock, Zone: "FerryZone", Biome: "cave"}
	deck := &rooms.Room{RoomId: boardLightDeck, Zone: "FerryZone", Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{boardLightDock: dock, boardLightDeck: deck},
		map[string]*rooms.ZoneConfig{"FerryZone": {Name: "FerryZone", RoomId: boardLightDock,
			RoomIds: map[int]struct{}{boardLightDock: {}, boardLightDeck: {}}}}))

	mover := users.NewTestUser(1, "alice", "Aliceia", 1001)
	watcher := users.NewTestUser(2, "bob", "Bobrick", 1002)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: mover, 2: watcher}))
	for _, u := range []*users.UserRecord{mover, watcher} {
		u.Character.RoomId = boardLightDock
		dock.AddPlayer(u.UserId)
	}
	mover.Character.Gold = 50
	require.True(t, mover.Character.Conditions.AddCondition(boardLightCond, false))
	require.Equal(t, messaging.SightFull, dock.ParticipantSight(watcher.UserId), "fixture: the mover's light lights the dock")

	cfg := configs.GetConfig()
	cfg.GamePlay.FerriesEnabled = true
	configs.SetConfigForTest(t, cfg)
	rpd := int(configs.GetTimingConfig().RoundsPerDay)
	route := Route{RouteId: "test-packet", Name: "the Test Packet", DeckRoom: boardLightDeck,
		Ports: []Port{{DockRoom: boardLightDock}, {DockRoom: boardLightOther}}, CrossingHours: 2, LayoverHours: 2, Fare: 5}
	// Docked at port 0 now: the phase lands the current round at the cycle's start.
	cycle := uint64(2 * (hoursToRounds(route.LayoverHours, rpd) + hoursToRounds(route.CrossingHours, rpd)))
	route.PhaseOffsetRounds = int((cycle - util.GetRoundCount()%cycle) % cycle)
	saved := routes
	routes = map[string]Route{route.RouteId: route}
	t.Cleanup(func() { routes = saved })
	require.True(t, StateAt(route, util.GetRoundCount(), rpd).Docked, "fixture: the vessel is docked")
	events.DrainQueuedMessagesForTest(watcher.UserId)

	require.Equal(t, BoardOk, Board(mover, &mobs.Mob{}, boardLightDock, route.RouteId))
	require.Equal(t, boardLightDeck, mover.Character.RoomId, "fixture: the mover boarded")
	require.Equal(t, messaging.SightNone, dock.ParticipantSight(watcher.UserId), "fixture: the light left with the mover")

	got := strings.Join(events.DrainQueuedMessagesForTest(watcher.UserId), "")
	require.Contains(t, got, "Aliceia", "the watcher saw the mover board by her own light")
	require.Contains(t, got, "crosses the gangplank")
}
