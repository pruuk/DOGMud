package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

// #512: the player's go now stops work through actions.InterruptWorkOnMove,
// the helper a mob's move shares. The player's lines are unchanged, and a
// cast walks on.
func TestGo_MovementInterruptsWorkButNotACast(t *testing.T) {
	cases := []struct {
		name  string
		begin func(*activity.Machine) error
		line  string // "" means the work goes on and nothing is said
	}{
		{"crafting", func(m *activity.Machine) error {
			return m.TransitionToCrafting(activity.CraftingData{RecipeId: "go-test", RoundsTotal: 5},
				state.TransitionReason{Trigger: activity.TriggerCraftBegin})
		}, "Your movement interrupts your crafting."},
		{"salvaging", func(m *activity.Machine) error {
			return m.TransitionToSalvaging(activity.SalvagingData{ItemUuid: "go-test", RoundsTotal: 5},
				state.TransitionReason{Trigger: activity.TriggerSalvageBegin})
		}, "Your movement interrupts your salvaging."},
		{"casting", func(m *activity.Machine) error {
			return m.TransitionToCasting(activity.CastingData{SpellId: "sparks", FoldsNeeded: 3},
				state.TransitionReason{Trigger: activity.TriggerCastBegin})
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			user, room := getTestUserAndRoom(t)
			if user.Character.Activity == nil {
				user.Character.Activity = activity.NewMachine()
			}
			require.NoError(t, tc.begin(user.Character.Activity))
			user.Character.ActionPoints = 100
			events.DrainQueuedMessagesForTest(user.UserId)

			handled, err := Go("north", user, room, 0)
			require.True(t, handled)
			require.NoError(t, err)
			require.NotEqual(t, room.RoomId, user.Character.RoomId, "fixture: the player walked")

			got := hoodTestText(user.UserId)
			if tc.line == "" {
				require.True(t, user.Character.Activity.IsCasting(), "movement never interrupts a cast")
				require.NotContains(t, got, "Your movement interrupts")
				return
			}
			require.True(t, user.Character.Activity.IsFree(), "the move stopped the work")
			require.Contains(t, got, `<ansi fg="red">`+tc.line+`</ansi>`)
		})
	}
}
