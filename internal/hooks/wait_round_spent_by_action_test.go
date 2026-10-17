package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A round a special move already spent and narrated is consumed WITHOUT the
// wait line. The 2026-10-17 playtest saw "You watch Arena Veteran Heavy
// closely, waiting for the perfect moment." right after a landed kick, and
// again after a landed bash: the kick had spent the round with
// SetRoundsWaiting(1), and the wait round then narrated it as a wait.
//
// The control: an ordinary wait (the engagement wind-up) still narrates.
func TestWaitRound_ARoundSpentByAnActionIsConsumedSilently(t *testing.T) {
	for _, tc := range []struct {
		name      string
		spend     bool
		wantLines int
	}{
		{"spent by a special move", true, 0},
		{"an ordinary wait (control)", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			restoreMsgs := seedWaitMessageFixture()
			defer restoreMsgs()

			util.SetRoundCountForTest(100)
			defer util.ResetRoundCountForTest()

			user1 := users.GetByUserId(1)
			require.NotNil(t, user1)
			mob := mobs.GetInstance(100)
			require.NotNil(t, mob)
			room := rooms.LoadRoom(1)
			require.NotNil(t, room)
			room.Lamp = rooms.LampPtr(90)

			armWaitOnMob(user1.Character, mob.InstanceId)
			if tc.spend {
				user1.Character.SpendRoundOnAction()
			} else {
				user1.Character.SetRoundsWaiting(1)
			}
			drainPlain(1)

			handled := handleCombatWaitRound(user1.Character, &mob.Character, combat.User, combat.Mob, user1, nil, room, room)
			require.True(t, handled, "the round must still be consumed")
			require.Zero(t, user1.Character.RoundsWaiting(), "the one-round budget must be spent")

			lines := drainPlain(1)
			require.Len(t, lines, tc.wantLines, "attacker lines: %v", lines)
			if tc.wantLines == 0 {
				require.Zero(t, countContaining(lines, "waiting for the perfect moment"))
			}
		})
	}
}
