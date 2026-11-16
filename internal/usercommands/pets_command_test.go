package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/require"
)

// #755: `pets` said "Pets not recognized." to a player travelling with a
// companion, with no pointer to dismiss. It answers now: the companion list
// (whose footer names dismiss), the familiar, or the no-companion line.
func TestPetsCommand_AnswersForCompanionsAndFamiliars(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	prevComp, prevPet := user.Character.Companions, user.Character.Pet
	t.Cleanup(func() { user.Character.Companions, user.Character.Pet = prevComp, prevPet })

	cmd, ok := userCommands["pets"]
	require.True(t, ok, "pets is a command")

	run := func() string {
		events.DrainQueuedMessagesForTest(user.UserId)
		handled, err := cmd.Func("", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	}

	user.Character.Companions = nil
	user.Character.Pet = pets.Pet{}
	require.Contains(t, run(), refusal.NoCompanionLine)

	user.Character.Pet = pets.Pet{Type: "cat", Name: "Whisk"}
	out := run()
	require.Contains(t, out, "Whisk")
	require.Contains(t, out, "help pets")

	user.Character.Companions = []characters.CompanionInfo{{Name: "Fleshy", InstanceId: 987654}}
	out = run()
	require.Contains(t, out, "Fleshy")
	require.Contains(t, out, "dismiss")
}
