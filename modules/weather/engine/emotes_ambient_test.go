package engine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/modules/weather/content"
	"github.com/GoMudEngine/GoMud/modules/weather/seasons"
	"github.com/GoMudEngine/GoMud/modules/weather/sim"
)

// seedWeatherRoom: room 1 in zoneA with players 9001 and 9002, the budget
// pinned, the round at 7000.
func seedWeatherRoom(t *testing.T, budget int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	// An outdoor biome, so the seasonal table's outdoor "default" pool is
	// the one picked (an unknown biome falls back to whatever the registry
	// holds).
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"plains": {BiomeId: "plains", Name: "Plains"},
	}))
	room := &rooms.Room{RoomId: 1, Zone: "zoneA", Biome: "plains"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, map[string]*rooms.ZoneConfig{}))
	seed := map[int]*users.UserRecord{}
	for _, id := range []int{9001, 9002} {
		u := users.NewTestUser(id, "w"+string(rune('a'+id-9001)), "W"+string(rune('a'+id-9001)), 0)
		u.Character.RoomId = 1
		seed[id] = u
		room.AddPlayer(id)
	}
	t.Cleanup(users.SeedUsersForTest(seed))
	rooms.MarkRoomOccupancy(1, 2, 0)
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.DrainQueuedMessagesForTest(9001)
	events.DrainQueuedMessagesForTest(9002)
}

var calmSeason = struct {
	zoneSeasons map[sim.ZoneId]seasons.ZoneSeason
	tables      content.SeasonalTables
}{
	zoneSeasons: map[sim.ZoneId]seasons.ZoneSeason{"zoneA": {Track: "temperate", Season: "winter"}},
	tables: content.SeasonalTables{
		{Track: "temperate", Season: "winter"}: {Outdoor: map[string][]string{"default": {"SEASON LINE"}}},
	},
}

func emitCalm(oneIn int, roll func(int) int) int {
	return EmitAmbient(nil, nil, sim.Config{}, map[sim.ZoneId]sim.WeatherType{},
		calmSeason.zoneSeasons, content.Tables{}, calmSeason.tables, 30, 100, oneIn, roll)
}

func TestEmitAmbient_SeasonalOneIn(t *testing.T) {
	seedWeatherRoom(t, 0)
	zero := func(int) int { return 0 }
	if sent := emitCalm(0, zero); sent != 0 {
		t.Fatalf("SeasonalEmoteOneIn 0 must never send a seasonal line, sent %d", sent)
	}
	if sent := emitCalm(1, zero); sent != 1 {
		t.Fatalf("SeasonalEmoteOneIn 1 sends on every pass, sent %d", sent)
	}
	one := func(int) int { return 1 }
	if sent := emitCalm(4, one); sent != 0 {
		t.Fatalf("a draw of 1 out of 4 is not the 1 in 4, sent %d", sent)
	}
}

// The repeat limit: with no budget, the same weather line twice reaches each
// listener once; a different line still reaches them.
func TestSendAmbientWeather_RepeatReachesEachListenerOnce(t *testing.T) {
	seedWeatherRoom(t, 0)
	cfg := configs.GetConfig()
	cfg.Balance.AmbientRepeatLimit = 1
	cfg.Balance.AmbientRepeatWindowRounds = 900
	configs.SetConfigForTest(t, cfg)
	room := rooms.LoadRoom(1)
	sendAmbientWeather(room, "Rain falls softly.")
	sendAmbientWeather(room, "Rain falls softly.")
	sendAmbientWeather(room, "Wind gusts.")
	for _, uid := range []int{9001, 9002} {
		got := strings.Join(events.DrainQueuedMessagesForTest(uid), "\n")
		if n := strings.Count(got, "Rain falls softly."); n != 1 {
			t.Errorf("%d read the repeated line %d times, want 1: %q", uid, n, got)
		}
		if n := strings.Count(got, "Wind gusts."); n != 1 {
			t.Errorf("%d read a different line %d times, want 1: %q", uid, n, got)
		}
	}
}

func TestEmitAmbient_SeasonalLineHonoursTheBudget(t *testing.T) {
	seedWeatherRoom(t, 5)
	ambient.ChargeForTest(9001, 7005)
	emitCalm(1, func(int) int { return 0 })
	if got := strings.Join(events.DrainQueuedMessagesForTest(9001), ""); strings.Contains(got, "SEASON LINE") {
		t.Errorf("9001 is over budget and must skip the line, got %q", got)
	}
	if got := strings.Join(events.DrainQueuedMessagesForTest(9002), ""); !strings.Contains(got, "SEASON LINE") {
		t.Errorf("9002 has budget and must get the line, got %q", got)
	}
}
