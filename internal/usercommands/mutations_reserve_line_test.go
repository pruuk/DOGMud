package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/require"
)

// #680 playtest seat A: `help titan-growth` names the conviction reserve as a
// drawback, but `mutations` listed only effects, and reserve units are not an
// effect. A reserving mutation now shows the drawback in both places.
func TestMutations_ReservingMutationListsTheReserveDrawback(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"testbig":   {MutationId: "testbig", Name: "Test Big", Description: "Big.", ReserveUnits: []int{1, 1}},
		"testsmall": {MutationId: "testsmall", Name: "Test Small", Description: "Small."},
	}))

	user, room := getTestUserAndRoom(t)
	prev := user.Character.Mutations
	t.Cleanup(func() { user.Character.Mutations = prev })

	render := func(owned map[string]int) string {
		user.Character.Mutations = owned
		events.DrainQueuedMessagesForTest(user.UserId)
		_, err := Mutations("", user, room, 0)
		require.NoError(t, err)
		return mutationsWrapAnsiTag.ReplaceAllString(
			strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), "")
	}

	require.Contains(t, render(map[string]int{"testbig": 1}), "conviction in reserve")
	require.NotContains(t, render(map[string]int{"testsmall": 1}), "conviction in reserve")
}
