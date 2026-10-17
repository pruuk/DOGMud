package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A warcry or rally with no ally in the room still lifts the shouter and
// still spends the shared special-move cooldown, but its opening line must
// not claim to ignite or rally "your allies" (2026-10-17 playtest: a solo,
// out-of-combat warcry read "...ignites the fighting spirit of your allies!").
func TestShouts_SoloOpeningLineNamesNoAllies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		run       func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)
		condition int
	}{
		{"warcry", Warcry, conditions.ConditionIdWarcry},
		{"rally", Rally, conditions.ConditionIdRally},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			room := rooms.LoadRoom(1)

			user := users.GetByUserId(1)
			require.Nil(t, parties.Get(user.UserId), "fixture must be partyless")
			require.Empty(t, user.Character.GetCharmIds(), "fixture must have no companions")
			user.Character.ConvictionMax.Value = 1000
			user.Character.Conviction = 1000
			user.Character.Cooldowns = characters.Cooldowns{}
			user.Character.Conditions = conditions.New()
			events.DrainQueuedMessagesForTest(user.UserId)

			_, err := tc.run("", user, room, 0)
			require.NoError(t, err)
			lines := events.DrainQueuedMessagesForTest(user.UserId)

			require.True(t, user.Character.HasCondition(tc.condition), "a solo shout must still lift the shouter")
			require.False(t, user.Character.CooldownReady(actions.SpecialMoveCooldownTag),
				"a solo shout must still spend the special-move cooldown")
			require.NotEmpty(t, lines, "the shout sent the shouter nothing")
			for _, line := range lines {
				require.NotContains(t, strings.ToLower(line), "allies",
					"a solo %s told the shouter it reached allies: %q", tc.name, line)
			}
		})
	}
}
