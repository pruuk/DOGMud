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

// pinRepeat sets the repeat limit knobs on top of seedAmbientRoom's config.
func pinRepeat(t *testing.T, limit, window int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientRepeatLimit = configs.ConfigInt(limit)
	cfg.Balance.AmbientRepeatWindowRounds = configs.ConfigInt(window)
	configs.SetConfigForTest(t, cfg)
}

func TestPlan_RepeatReachesAListenerOnce(t *testing.T) {
	room := seedAmbientRoom(t, 0, nil, listenA)
	pinRepeat(t, 1, 900)
	key := LineKey(`mob`, 7, `polishes a glass.`)
	if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: key}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("first time: delivered %v, want the listener", r.Delivered)
	}
	util.SetRoundCountForTest(ambNow + 899)
	if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: key}); len(r.Delivered) != 0 || !slices.Equal(r.Exclude, []int{listenA}) {
		t.Fatalf("repeat inside the window: delivered %v exclude %v, want left out", r.Delivered, r.Exclude)
	}
	if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: LineKey(`mob`, 7, `wipes the bar.`)}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("a different line: delivered %v, want the listener", r.Delivered)
	}
	util.SetRoundCountForTest(ambNow + 900)
	if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: key}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("window passed: delivered %v, want the listener", r.Delivered)
	}
}

func TestPlan_RepeatSparesAListenerWhoHasNotHeardIt(t *testing.T) {
	room := seedAmbientRoom(t, 0, nil, listenA, listenB)
	pinRepeat(t, 1, 900)
	key := LineKey(`room`, ambRoom, `A cart rattles past.`)
	Plan(Line{Room: room, Source: SourceRoomIdle, Key: key, Exclude: []int{listenB}}) // only A hears it
	r := Plan(Line{Room: room, Source: SourceRoomIdle, Key: key})
	if !slices.Equal(r.Exclude, []int{listenA}) || !slices.Equal(r.Delivered, []int{listenB}) {
		t.Fatalf("exclude %v delivered %v, want A left out and B hearing", r.Exclude, r.Delivered)
	}
}

func TestPlan_HeldBackRepeatDoesNotChargeTheBudget(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	pinRepeat(t, 1, 900)
	key := LineKey(`item`, 12, `Feed me.`)
	Plan(Line{Room: room, Source: SourceItemChatter, Key: key})
	util.SetRoundCountForTest(ambNow + 5)                       // budget open again
	Plan(Line{Room: room, Source: SourceItemChatter, Key: key}) // held back as a repeat
	if r := Plan(Line{Room: room, Source: SourceWeather, Key: LineKey(`weather`, 0, `Rain falls.`)}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("fresh line after a held-back repeat: delivered %v, want the listener (the repeat must not charge the budget)", r.Delivered)
	}
}

func TestPlan_BudgetHeldLineIsNotRecordedAsSeen(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	pinRepeat(t, 1, 900)
	ChargeForTest(listenA, ambNow+5)
	key := LineKey(`mob`, 7, `hums.`)
	Plan(Line{Room: room, Source: SourceMobChatter, Key: key}) // budget holds it back
	util.SetRoundCountForTest(ambNow + 5)
	if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: key}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("delivered %v: a line the budget held back was recorded as seen", r.Delivered)
	}
}

func TestPlan_RepeatOffSwitches(t *testing.T) {
	for _, tc := range []struct {
		name          string
		limit, window int
		key           string
	}{
		{"empty key", 1, 900, ``},
		{"limit 0", 0, 900, LineKey(`mob`, 7, `x`)},
		{"window 0", 1, 0, LineKey(`mob`, 7, `x`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room := seedAmbientRoom(t, 0, nil, listenA)
			pinRepeat(t, tc.limit, tc.window)
			for i := 0; i < 3; i++ {
				if r := Plan(Line{Room: room, Source: SourceMobChatter, Key: tc.key}); !slices.Equal(r.Delivered, []int{listenA}) {
					t.Fatalf("send %d: delivered %v, want the listener every time", i, r.Delivered)
				}
			}
		})
	}
}

func TestRoundTick_PrunesRepeatMemory(t *testing.T) {
	room := seedAmbientRoom(t, 0, nil, listenA)
	pinRepeat(t, 1, 900)
	Plan(Line{Room: room, Source: SourceMobChatter, Key: LineKey(`mob`, 7, `x`)})
	RoundTick(ambNow + 899)
	if n := RepeatKeysForTest(listenA); n != 1 {
		t.Fatalf("inside the window: %d keys remembered, want 1", n)
	}
	RoundTick(ambNow + 900)
	if n := RepeatKeysForTest(listenA); n != 0 {
		t.Fatalf("window passed: %d keys remembered, want 0", n)
	}
}
