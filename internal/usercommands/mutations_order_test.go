package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/require"
)

// The 2026-11-04 closing playtest saw the list reorder between runs: it ranged
// over the mutation map. Sorted by id, it reads the same every time.
func TestMutations_ListReadsTheSameEveryTime(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	specs := map[string]*mutations.MutationSpec{}
	owned := map[string]int{}
	var want []string
	for _, id := range []string{"a-one", "b-two", "c-three", "d-four", "e-five", "f-six"} {
		specs[id] = &mutations.MutationSpec{MutationId: id, Name: "Name " + id, Description: "x"}
		owned[id] = 1
		want = append(want, "Name "+id)
	}
	t.Cleanup(mutations.SeedMutationsForTest(specs))

	user, room := getTestUserAndRoom(t)
	prev := user.Character.Mutations
	user.Character.Mutations = owned
	t.Cleanup(func() { user.Character.Mutations = prev })

	for range 20 {
		events.DrainQueuedMessagesForTest(user.UserId)
		_, err := Mutations("", user, room, 0)
		require.NoError(t, err)

		var got []string
		for _, msg := range events.DrainQueuedMessagesForTest(user.UserId) {
			plain := mutationsWrapAnsiTag.ReplaceAllString(msg, "")
			for _, id := range want {
				if strings.Contains(plain, id+" (") {
					got = append(got, id)
				}
			}
		}
		require.Equal(t, want, got)
	}
}
