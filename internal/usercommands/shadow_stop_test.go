package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowStopUser is a player with Skullduggery 3 in a bare room, and the room.
func shadowStopUser(t *testing.T, userId int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		actions.ShadowingConditionId: {ConditionId: actions.ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
	u := users.NewTestUser(userId, "stopper", "Stopper", 0)
	u.Character.Skills = map[string]int{string(skills.Skullduggery): 3}
	events.DrainQueuedMessagesForTest(userId)
	return u, &rooms.Room{RoomId: 99401}
}

// `shadow stop` ends a live shadow through actions.EndShadow: target and
// condition gone, cooldown running, the stop line sent.
func TestShadowStop_EndsTheShadow(t *testing.T) {
	const userId = 7401
	u, room := shadowStopUser(t, userId)
	u.Character.SetMiscData("shadow-target-mob", 88401)
	if err := u.Character.AddCondition(actions.ShadowingConditionId, false); err != nil {
		t.Fatalf("add condition 87: %v", err)
	}

	if handled, err := Shadow("stop", u, room, 0); !handled || err != nil {
		t.Fatalf("Shadow(stop) = %v, %v", handled, err)
	}

	if tu, tm := actions.ShadowTargetOf(u.Character); tu != 0 || tm != 0 {
		t.Errorf("ShadowTargetOf after stop = (%d, %d), want (0, 0)", tu, tm)
	}
	u.Character.Conditions.Prune()
	if u.Character.HasCondition(actions.ShadowingConditionId) {
		t.Error("condition 87 survived shadow stop")
	}
	if u.Character.GetCooldown(skills.Skullduggery.String("shadow")) <= 0 {
		t.Error("shadow stop started no cooldown")
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You stop shadowing your target.") {
		t.Errorf("messages %q, want the stop line", msgs)
	}
}

// With no shadow, `shadow stop` says so and starts nothing.
func TestShadowStop_WithNoShadowSaysSo(t *testing.T) {
	const userId = 7402
	u, room := shadowStopUser(t, userId)

	if handled, err := Shadow("stop", u, room, 0); !handled || err != nil {
		t.Fatalf("Shadow(stop) = %v, %v", handled, err)
	}

	if u.Character.GetCooldown(skills.Skullduggery.String("shadow")) != 0 {
		t.Error("stopping no shadow started a cooldown")
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You aren't shadowing anyone.") {
		t.Errorf("messages %q, want the not-shadowing line", msgs)
	}
}
