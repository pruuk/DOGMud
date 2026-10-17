package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #546: ExecuteTaunt pays its conviction, then re-resolves the target and
// claims the shared timer. When either fails after the payment it returns the
// paid cost with NoTarget or OnCooldown, and the command used to answer with
// the same line as a free refusal, hiding the spent cost and inviting a second
// paid try. A paid refusal now says the effort is spent; an unpaid one keeps
// the ordinary line.
func TestTaunt_APaidRefusalSaysTheEffortIsSpent(t *testing.T) {
	paid := characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolConviction, Charged: 5}
	for _, tc := range []struct {
		name   string
		result actions.TauntResult
		want   string
	}{
		{"paid, target gone", actions.TauntResult{Cost: paid, NoTarget: true}, refusal.NoTargetSpentText("taunt")},
		{"paid, timer taken", actions.TauntResult{Cost: paid, OnCooldown: true}, refusal.CooldownSpentText("taunt")},
		{"unpaid, no target (control)", actions.TauntResult{NoTarget: true}, refusal.NoTargetText("taunt")},
		{"unpaid, on cooldown (control)", actions.TauntResult{OnCooldown: true}, refusal.CooldownText("taunt")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			user := users.GetByUserId(1)
			original := executeTauntAction
			executeTauntAction = func(actions.Actor) actions.TauntResult { return tc.result }
			t.Cleanup(func() { executeTauntAction = original })
			room := rooms.LoadRoom(1)
			events.DrainQueuedMessagesForTest(user.UserId)

			handled, err := Taunt("bobrick", user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)

			lines := events.DrainQueuedMessagesForTest(user.UserId)
			require.Len(t, lines, 1, "a refusal is one line: %v", lines)
			require.Contains(t, lines[0], tc.want)
			if strings.Contains(tc.name, "unpaid") {
				require.NotContains(t, lines[0], "effort is spent")
			}
		})
	}
}
