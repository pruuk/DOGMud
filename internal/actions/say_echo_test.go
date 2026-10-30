package actions

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// A player's say still echoes next door unbudgeted (#317): even with the
// neighbour over budget, nobody is left out of the echo.
func TestPlayerSayEcho_IsNeverBudgeted(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = 5
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(6000)

	here := &rooms.Room{RoomId: 3195, Exits: map[string]exit.RoomExit{"north": {RoomId: 3196}}}
	next := &rooms.Room{RoomId: 3196, Exits: map[string]exit.RoomExit{"south": {RoomId: 3195}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{3195: here, 3196: next}, map[string]*rooms.ZoneConfig{}))
	speaker := users.NewTestUser(31951, "echospk", "Echospk", 0)
	speaker.Character.RoomId = 3195
	neighbour := users.NewTestUser(31961, "echonb", "Echonb", 0)
	neighbour.Character.RoomId = 3196
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{31951: speaker, 31961: neighbour}))
	here.AddPlayer(31951)
	next.AddPlayer(31961)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	ambient.ChargeForTest(31961, 6005)

	Say(NewUserActorInRoom(speaker, here), "Hello?")
	var echo *events.Message
	for _, m := range events.DrainQueuedRoomMessagesForTest(3196) {
		if strings.Contains(m.Text, "You hear someone talking.") {
			echo = &m
			break
		}
	}
	if echo == nil {
		t.Fatal("a player's say must still echo into the neighbouring room")
	}
	if slices.Contains(echo.ExcludeUserIds, 31961) {
		t.Fatalf("a player's say is never budgeted, excludes %v", echo.ExcludeUserIds)
	}
}
