package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// #218: your own gear needs no light. A looker who sees nothing, in the dark
// or blinded, still knows by touch what they carry: look names it, without
// its description (which sight reads). Anything else keeps the refusal.

const touchGearItemId = 99218

func seedTouchGear(t *testing.T, lamp int) (func(string) string, *characters.Character) {
	t.Helper()
	user, room := seedDarknessGateRoom(t, lamp)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		touchGearItemId: {ItemId: touchGearItemId, Name: "Oak Staff", Type: items.Weapon,
			Description: "A staff carved with running hares."},
	}))
	require.True(t, user.Character.StoreItem(items.New(touchGearItemId)))
	look := func(what string) string {
		return runGate(t, user, func() (bool, error) { return Look(what, user, room, 0) })
	}
	return look, user.Character
}

func TestLook_OwnGearInTheDarkIsKnownByTouch(t *testing.T) {
	look, _ := seedTouchGear(t, 0)
	out := look("staff")
	require.Contains(t, out, "You run your hands over your")
	require.Contains(t, out, "Oak Staff")
	require.NotContains(t, out, "running hares", "the description is what sight reads")
	require.NotContains(t, out, tooDarkToSeeLine)
}

func TestLook_OwnGearWhileBlindedIsKnownByTouch(t *testing.T) {
	look, char := seedTouchGear(t, 90)
	orig := char.Perception
	t.Cleanup(func() { char.Perception = orig })
	char.Perception = characters.New().Perception
	require.NoError(t, char.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	out := look("staff")
	require.Contains(t, out, "You run your hands over your")
	require.NotContains(t, out, blindLookLine)
}

// Something that is not yours keeps the refusal in the dark.
func TestLook_NotYourGearInTheDarkStillRefuses(t *testing.T) {
	look, _ := seedTouchGear(t, 0)
	out := look("lantern")
	require.Contains(t, out, tooDarkToSeeLine)
	require.NotContains(t, out, "You run your hands over your")
}
