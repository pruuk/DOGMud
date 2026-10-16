package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// 2026-08-03 crafting-focus audit: 13 active commands could fire mid-craft.
// Each newly gated command must refuse (not interrupt) while the Activity
// machine is occupied, matching the shoot/reload convention. The refusal is
// refusal.WhileBusy (M7 slice 3, #371).

const brUserId = 9141

func seedBusyUser(t *testing.T) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()
	u := users.NewTestUser(brUserId, "busybee", "Busybee", uint64(brUserId))
	u.Character.Activity = activity.NewMachine()
	if err := u.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "test", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	); err != nil {
		t.Fatalf("fixture: could not enter Crafting state: %v", err)
	}
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{brUserId: u})
	room := &rooms.Room{RoomId: 999902}
	events.DrainQueuedMessagesForTest(brUserId)
	return u, room, func() {
		events.DrainQueuedMessagesForTest(brUserId)
		cleanUsers()
	}
}

func TestBusyCommands_RefuseWhileCrafting(t *testing.T) {
	cases := []struct {
		name  string
		doing string
		run   func(u *users.UserRecord, r *rooms.Room) (bool, error)
	}{
		{"eat", "eat", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Eat("bread", u, r, 0) }},
		{"drink", "drink", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Drink("water", u, r, 0) }},
		{"sleep", "sleep", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Sleep("", u, r, 0) }},
		{"use", "use that", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Use("lever", u, r, 0) }},
		{"equip", "change equipment", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Equip("sword", u, r, 0) }},
		{"remove", "change equipment", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Remove("sword", u, r, 0) }},
		{"gearup", "change equipment", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Gearup("", u, r, 0) }},
		{"throw", "throw anything", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Throw("rock north", u, r, 0) }},
		{"forage", "forage", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Forage("", u, r, 0) }},
		{"search", "search", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Search("", u, r, 0) }},
		{"track", "track", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Track("wolf", u, r, 0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, room, cleanup := seedBusyUser(t)
			defer cleanup()

			handled, err := tc.run(u, room)
			if err != nil || !handled {
				t.Fatalf("command errored: handled=%v err=%v", handled, err)
			}
			msgs := strings.Join(events.DrainQueuedMessagesForTest(brUserId), "\n")
			if !strings.Contains(msgs, refusal.WhileBusyText(tc.doing)) {
				t.Errorf("no busy refusal sent; got:\n%s", msgs)
			}
			if !u.Character.IsCrafting() {
				t.Error("crafting state must be untouched (refuse, not interrupt)")
			}
		})
	}
}

// Cancelling a track is always allowed — only starting one is gated.
func TestTrackStop_AllowedWhileCrafting(t *testing.T) {
	u, room, cleanup := seedBusyUser(t)
	defer cleanup()

	handled, err := Track("stop", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("track stop errored: handled=%v err=%v", handled, err)
	}
	msgs := strings.Join(events.DrainQueuedMessagesForTest(brUserId), "\n")
	if strings.Contains(msgs, "while you're busy.") {
		t.Errorf("track stop must not be busy-refused:\n%s", msgs)
	}
}

// Skill-gated commands refuse only when the command is visible at all.
func TestStealPicklock_RefuseWhileCrafting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		doing string
		run   func(u *users.UserRecord, r *rooms.Room) (bool, error)
	}{
		{"steal", "steal", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Steal("coin rat", u, r, 0) }},
		{"picklock", "pick a lock", func(u *users.UserRecord, r *rooms.Room) (bool, error) { return Picklock("north", u, r, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, room, cleanup := seedBusyUser(t)
			defer cleanup()
			if u.Character.Skills == nil {
				u.Character.Skills = map[string]int{}
			}
			u.Character.Skills["skullduggery"] = 2

			handled, err := tc.run(u, room)
			if err != nil || !handled {
				t.Fatalf("command errored: handled=%v err=%v", handled, err)
			}
			msgs := strings.Join(events.DrainQueuedMessagesForTest(brUserId), "\n")
			if !strings.Contains(msgs, refusal.WhileBusyText(tc.doing)) {
				t.Errorf("no busy refusal sent; got:\n%s", msgs)
			}
		})
	}
}
