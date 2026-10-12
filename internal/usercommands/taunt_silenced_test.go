package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const silencedTauntConditionId = 9502

// A silenced player's taunt runs the real action (no stub) and is told so,
// and the cooldown and conviction are untouched (M6 slice 3).
func TestPlayerTauntSilencedIsToldAndPaysNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		silencedTauntConditionId: {ConditionId: silencedTauntConditionId, Name: "Test Silenced", Flags: []conditions.Flag{conditions.Silenced}},
	}))
	attacker := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	require.NoError(t, attacker.Character.AddCondition(silencedTauntConditionId, true))
	before := attacker.Character.Conviction
	events.DrainQueuedMessagesForTest(attacker.UserId)

	handled, err := Taunt("bobrick", attacker, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	found := false
	for _, line := range events.DrainQueuedMessagesForTest(attacker.UserId) {
		if strings.Contains(line, "You cannot make a sound.") {
			found = true
		}
	}
	require.True(t, found, "a silenced taunter must be told they cannot make a sound")
	require.Equal(t, before, attacker.Character.Conviction, "nothing may be paid")
}
