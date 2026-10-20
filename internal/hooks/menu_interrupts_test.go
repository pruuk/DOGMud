package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func TestMenuCloseOnRoomChange_ClosesTheMoversMenu(t *testing.T) {
	u := users.NewTestUser(41, "mover", "Mira", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{41: u})()
	closed := menu.CloseNone
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{OnClose: func(r menu.CloseReason) { closed = r }})

	MenuCloseOnRoomChange(events.RoomChange{UserId: 41, FromRoomId: 1, ToRoomId: 2})

	if u.HasOpenMenu() || closed != menu.CloseRoomChange {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), closed)
	}
}

func TestMenuCloseOnRoomChange_IgnoresMobsAndSameRoom(t *testing.T) {
	u := users.NewTestUser(42, "stayer", "Ilo", 0)
	// A user whose id equals the mob instance id: a handler that wrongly keyed
	// off MobInstanceId would close this menu.
	bystander := users.NewTestUser(9, "bystander", "Oro", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{42: u, 9: bystander})()
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{})
	bystander.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{})

	MenuCloseOnRoomChange(events.RoomChange{MobInstanceId: 9, FromRoomId: 1, ToRoomId: 2})
	if !bystander.HasOpenMenu() {
		t.Fatal("menu closed by a mob move")
	}
	MenuCloseOnRoomChange(events.RoomChange{UserId: 42, FromRoomId: 3, ToRoomId: 3})

	if !u.HasOpenMenu() {
		t.Fatal("menu closed by a mob move or a same-room event")
	}
}

func TestApplyConditions_SleepClosesTheMenu(t *testing.T) {
	const sleepCond = 8962
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepCond: {ConditionId: sleepCond, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	u := users.NewTestUser(43, "sleeper", "Dusk", 0)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{43: u})()
	closed := menu.CloseNone
	u.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{OnClose: func(r menu.CloseReason) { closed = r }})

	ApplyConditions(events.Condition{UserId: 43, ConditionId: sleepCond, LifeEpoch: u.Character.LifeEpoch})

	if u.HasOpenMenu() || closed != menu.CloseAsleep {
		t.Fatalf("open %v closed %v", u.HasOpenMenu(), closed)
	}
}
