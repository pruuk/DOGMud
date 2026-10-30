package ambient

import (
	"fmt"
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	ambRoom = 3170
	ambZone = "AmbientZone"
	ambNow  = uint64(10000)
	listenA = 3171
	listenB = 3172
)

// seedAmbientRoom: one room in ambZone (zone multiplier zoneMult, nil =
// unset) holding the given players, the budget pinned to budget rounds and
// the round count at ambNow.
func seedAmbientRoom(t *testing.T, budget int, zoneMult *float64, userIds ...int) *rooms.Room {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ResetForTest())

	room := &rooms.Room{RoomId: ambRoom, Zone: ambZone}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{ambRoom: room},
		map[string]*rooms.ZoneConfig{ambZone: {Name: ambZone, AmbientMultiplier: zoneMult}}))
	seed := map[int]*users.UserRecord{}
	for _, id := range userIds {
		u := users.NewTestUser(id, fmt.Sprintf("amb%d", id), fmt.Sprintf("Amb%d", id), 0)
		u.Character.RoomId = ambRoom
		seed[id] = u
		room.AddPlayer(id)
	}
	t.Cleanup(users.SeedUsersForTest(seed))
	rooms.MarkRoomOccupancy(ambRoom, len(userIds), 0)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(ambNow)
	return room
}

func sorted(ids []int) []int {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestPlan_BudgetOffDeliversToEveryone(t *testing.T) {
	room := seedAmbientRoom(t, 0, nil, listenA, listenB)
	for i := 0; i < 3; i++ {
		r := Plan(Line{Room: room, Source: SourceRoomIdle})
		if len(r.Exclude) != 0 || !slices.Equal(sorted(r.Delivered), []int{listenA, listenB}) {
			t.Fatalf("budget 0, line %d: exclude %v delivered %v, want everyone", i, r.Exclude, r.Delivered)
		}
	}
}

func TestPlan_TwoLinesInAWindowReachAListenerOnce(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	if r := Plan(Line{Room: room, Source: SourceMobChatter}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("first line: delivered %v, want the listener", r.Delivered)
	}
	if r := Plan(Line{Room: room, Source: SourceWeather}); len(r.Delivered) != 0 || !slices.Equal(r.Exclude, []int{listenA}) {
		t.Fatalf("second line in the window: delivered %v exclude %v, want skipped", r.Delivered, r.Exclude)
	}
	util.SetRoundCountForTest(ambNow + 4)
	if r := Plan(Line{Room: room, Source: SourceGossip}); len(r.Delivered) != 0 {
		t.Fatalf("round +4 is still inside a 5-round window, delivered %v", r.Delivered)
	}
	util.SetRoundCountForTest(ambNow + 5)
	if r := Plan(Line{Room: room, Source: SourceItemChatter}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("round +5 opens the window again, delivered %v", r.Delivered)
	}
}

func TestPlan_ASecondListenerWithBudgetStillHears(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	ChargeForTest(listenA, ambNow+5)
	r := Plan(Line{Room: room, Source: SourceRoomIdle})
	if !slices.Equal(r.Exclude, []int{listenA}) || !slices.Equal(r.Delivered, []int{listenB}) {
		t.Fatalf("exclude %v delivered %v, want A skipped and B hearing", r.Exclude, r.Delivered)
	}
}

func TestPlan_CallerExcludeIsNeverCharged(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	r := Plan(Line{Room: room, Source: SourceRoomIdle, Exclude: []int{listenA}})
	if !slices.Equal(r.Exclude, []int{listenA}) || len(r.Delivered) != 0 {
		t.Fatalf("caller exclude: exclude %v delivered %v", r.Exclude, r.Delivered)
	}
	if r := Plan(Line{Room: room, Source: SourceRoomIdle}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("a player the caller left out was charged anyway: delivered %v", r.Delivered)
	}
}

func TestPlan_APlayerWhoCannotPerceiveIsNotCharged(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	blindB := func(u *users.UserRecord) bool { return u.UserId != listenB }
	r := Plan(Line{Room: room, Source: SourceRoomIdle, Perceives: blindB})
	if !slices.Equal(r.Delivered, []int{listenA}) || slices.Contains(r.Exclude, listenB) {
		t.Fatalf("delivered %v exclude %v: B neither delivered nor excluded", r.Delivered, r.Exclude)
	}
	if r := Plan(Line{Room: room, Source: SourceWeather}); !slices.Equal(r.Delivered, []int{listenB}) {
		t.Fatalf("B, who did not see the first line, must still have budget; delivered %v", r.Delivered)
	}
}

func TestBudgetWindow_ZoneMultiplier(t *testing.T) {
	for _, c := range []struct {
		name       string
		budget     int
		mult       *float64
		wantWindow uint64
		wantHushed bool
	}{
		{"unset", 4, nil, 4, false},
		{"2.0 halves it", 4, rooms.MultiplierPtr(2), 2, false},
		{"0.5 doubles it", 4, rooms.MultiplierPtr(0.5), 8, false},
		{"0 with the budget on is hushed", 4, rooms.MultiplierPtr(0), 0, true},
		{"0 with the budget off is nothing", 0, rooms.MultiplierPtr(0), 0, false},
		{"never below one round", 1, rooms.MultiplierPtr(4), 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			seedAmbientRoom(t, c.budget, c.mult)
			w, h := budgetWindow(ambZone)
			if w != c.wantWindow || h != c.wantHushed {
				t.Fatalf("window %d hushed %v, want %d %v", w, h, c.wantWindow, c.wantHushed)
			}
		})
	}
}

func TestPlan_HushedZoneSkipsEveryone(t *testing.T) {
	room := seedAmbientRoom(t, 5, rooms.MultiplierPtr(0), listenA, listenB)
	r := Plan(Line{Room: room, Source: SourceWeather})
	if len(r.Delivered) != 0 || !slices.Equal(sorted(r.Exclude), []int{listenA, listenB}) {
		t.Fatalf("hushed zone: delivered %v exclude %v", r.Delivered, r.Exclude)
	}
}

func TestAllHaveBudget(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	if !AllHaveBudget(room) {
		t.Fatal("fresh players all have budget")
	}
	ChargeForTest(listenB, ambNow+3)
	if AllHaveBudget(room) {
		t.Fatal("a conversation must not start while a present player is over budget")
	}
	empty := seedAmbientRoom(t, 5, nil)
	if !AllHaveBudget(empty) {
		t.Fatal("a room with no players has nobody to cut off")
	}
	off := seedAmbientRoom(t, 0, nil, listenA)
	ChargeForTest(listenA, ambNow+3)
	if !AllHaveBudget(off) {
		t.Fatal("with the budget off every conversation may start")
	}
}

func TestAllHaveBudget_HushedZone(t *testing.T) {
	hushed := seedAmbientRoom(t, 5, rooms.MultiplierPtr(0), listenA)
	if AllHaveBudget(hushed) {
		t.Fatal("a hushed zone (multiplier 0, budget on) starts no conversation")
	}
	off := seedAmbientRoom(t, 0, rooms.MultiplierPtr(0), listenA)
	if !AllHaveBudget(off) {
		t.Fatal("with the budget off a zero multiplier does not gate conversations")
	}
}

func TestSingleLineHeard_OncePerVisit(t *testing.T) {
	seedAmbientRoom(t, 0, nil, listenA, listenB)
	MarkSingleLineHeard(ambRoom, []int{listenA})
	if got := SingleLineHeard(ambRoom, []int{listenA, listenB}); !slices.Equal(got, []int{listenA}) {
		t.Fatalf("heard = %v, want only A", got)
	}
	users.GetByUserId(listenA).Character.RoomId = 1 // A walks out
	RoundTick(ambNow)
	users.GetByUserId(listenA).Character.RoomId = ambRoom // and back in
	if got := SingleLineHeard(ambRoom, []int{listenA}); len(got) != 0 {
		t.Fatalf("a new visit must show the line again, heard = %v", got)
	}
}

func TestRoundTick_PrunesSpentBudget(t *testing.T) {
	seedAmbientRoom(t, 5, nil, listenA)
	ChargeForTest(listenA, ambNow+2)
	util.SetRoundCountForTest(ambNow + 3)
	RoundTick(ambNow + 3)
	budgetMu.Lock()
	n := len(budgetNext)
	budgetMu.Unlock()
	if n != 0 {
		t.Fatalf("a spent budget entry must be pruned, %d left", n)
	}
}

func TestParseSource(t *testing.T) {
	for _, s := range Sources {
		if got, ok := ParseSource(string(s)); !ok || got != s {
			t.Errorf("ParseSource(%q) = %q,%v", s, got, ok)
		}
	}
	if _, ok := ParseSource("tips"); ok {
		t.Error("tips are exempt and are not a source")
	}
}
