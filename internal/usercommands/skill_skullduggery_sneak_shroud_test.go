package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #444: a player hidden by a weaker Empathic Shroud who sneaks takes the hide
// over and is told so; under a stronger shroud they are told they are
// already hidden, as before.
func TestSneakCommand_UnderAShroud(t *testing.T) {
	cases := []struct {
		name        string
		dex         int
		shroud      float64
		want        string
		shroudHolds bool
	}{
		{"weaker shroud: the sneak takes over", 400, 10, "You let the shroud fall away and slip into the shadows on your own.", false},
		{"stronger shroud: already hidden", 1, 9999, "You're already hidden!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
				9: {ConditionId: 9, Name: "Hidden",
					Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
				conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
					TriggerCount: 16, RoundInterval: 1,
					Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
			}))
			room := &rooms.Room{RoomId: 9950, Zone: "SneakShroud", Lamp: rooms.LampPtr(90)}
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9950: room}, map[string]*rooms.ZoneConfig{}))
			sneaker := users.NewTestUser(9951, "shrouded", "Shrouded", 0)
			sneaker.Character.Skills = map[string]int{string(skills.Skullduggery): 1}
			sneaker.Character.Stats.Dexterity.Base = tc.dex
			sneaker.Character.RoomId = room.RoomId
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9951: sneaker}))
			room.AddPlayer(9951)
			require.NoError(t, sneaker.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, tc.shroud, "spell"))
			require.True(t, sneaker.Character.HiddenByShroud(), "fixture: the shroud must hide the sneaker")
			sneaker.Character.StaminaMax.Value = 100
			sneaker.Character.Stamina = 100
			events.DrainQueuedMessagesForTest(9951)

			handled, err := Sneak("", sneaker, room, 0)
			require.True(t, handled)
			require.NoError(t, err)

			out := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(
				strings.Join(events.DrainQueuedMessagesForTest(9951), "\n"), "")
			require.Contains(t, out, tc.want)
			require.True(t, sneaker.Character.IsHidden())
			require.Equal(t, tc.shroudHolds, sneaker.Character.HiddenByShroud())
		})
	}
}
