package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A refused flee must tell the player what to DO about it.
//
// The flee veto is IsStanding(), not grapple. flee.go named only the grapple
// case, so a knocked-down player fell through to "You can't break away just
// yet." -- which reads as a timing problem and never mentions standing up.
// Knockdown is common (trips, bashes, sweeps, kicks and double fumbles all
// cause it) and is exactly when a player wants to run, so this is the refusal
// most worth explaining.
func fleeRefusalFor(t *testing.T, knockDown bool) string {
	t.Helper()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	require.NotNil(t, u)
	require.NoError(t, u.Character.Validate())
	u.Character.StaminaMax.Value = 1000
	u.Character.Stamina = 1000

	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()

	if knockDown {
		require.NoError(t, u.Character.Position.TransitionToProne(position.ProneData{},
			state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}))
		require.False(t, u.Character.IsStanding(), "precondition: knocked down")
		require.False(t, u.Character.IsStandingGrapple() || u.Character.IsGroundGrapple(),
			"precondition: knocked down but NOT grappled -- the case that was unexplained")
	}

	events.DrainQueuedMessagesForTest(u.UserId)
	_, err := usercommands.Flee("", u, room, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(u.UserId), " ")
}

func TestFleeRefusal_KnockedDownTellsYouToStand(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	out := fleeRefusalFor(t, true)

	require.Contains(t, strings.ToLower(out), "stand",
		"a knocked-down player must be told that standing up is what unblocks "+
			"fleeing; got %q", out)
	require.NotContains(t, out, "break away just yet",
		"the generic timing line hides the real reason; got %q", out)
}

// The control: standing and un-grappled, a flee is NOT refused at all, so the
// test above cannot pass by refusing everything.
func TestFleeRefusal_StandingPlayerIsNotRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	out := fleeRefusalFor(t, false)

	require.NotContains(t, strings.ToLower(out), "stand up first",
		"a standing player must not be told to stand up; got %q", out)
	require.NotContains(t, out, "break away just yet",
		"a standing, un-grappled player in combat must be allowed to try; got %q", out)
}
