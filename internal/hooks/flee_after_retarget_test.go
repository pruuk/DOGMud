package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// End-to-end guard for the U12c-0 flee regression, driven through the REAL
// player command rather than the state machine in isolation.
//
// The machine-level test (combatphase/flee_during_windup_test.go) proves the
// transition is allowed. This proves the thing a player actually experiences:
// typing `flee` after being retargeted does not come back with
// "You can't break away just yet."
//
// It is written against the command because that is where the regression was
// visible: 15 consecutive refusals in a live fight, with the machine looking
// perfectly healthy from the inside.
func TestFleeCommand_WorksAfterARetarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	require.NotNil(t, u)
	require.NotNil(t, room)
	require.NoError(t, u.Character.Validate())

	u.Character.StaminaMax.Value = 1000
	u.Character.Stamina = 1000

	// Engage, finish the wind-up, then RETARGET -- which is what reciprocal
	// aggro, a taunt pulling your target away, or the `target` command does,
	// and which puts the actor back into Engaging.
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	require.Equal(t, combatphase.Engaged, u.Character.CombatPhase.State())

	u.Character.SetAggro(0, 100, characters.DefaultAttack, 1)
	require.Equal(t, combatphase.Engaging, u.Character.CombatPhase.State(),
		"precondition: a retarget re-enters the wind-up")

	events.DrainQueuedMessagesForTest(u.UserId)
	_, err := usercommands.Flee("", u, room, 0)
	require.NoError(t, err)

	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "can't break away just yet") {
			t.Fatalf("flee was refused after a retarget -- the U12c-0 regression is back: %q", msg)
		}
	}

	require.Equal(t, combatphase.Disengaging, u.Character.CombatPhase.State(),
		"the flee attempt must actually put the actor into Disengaging")
}
