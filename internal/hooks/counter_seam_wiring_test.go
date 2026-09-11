package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fireSpellCounterTier is the spell exits' counter dispatch. A player (Aliceia)
// crit-defends the Skeleton's cast in the dark and counters: her line must not
// name the Skeleton she cannot see.
func TestSpellCounterTier_InTheDarkNamesNobody(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(
		sequencedContestRunner(t, &calls, attackWinContest(t)))
	t.Cleanup(restore)
	drainPlain(1)
	drainPlain(2)

	defender := users.GetByUserId(1)
	caster := mobs.GetInstance(100)
	res := fireSpellCounterTier(rooms.LoadRoom(1),
		combat.ChannelDefenceResult{Defended: true, DefensiveCrit: true},
		combat.ChannelSpellMental, defender.Character, &caster.Character, defender, nil)
	require.True(t, res.Countered, "precondition: the counter must fire")

	counterer := drainPlain(1)
	assert.GreaterOrEqual(t, countContaining(counterer, "COUNTER"), 1)
	assert.Equal(t, 0, countContaining(counterer, "Skeleton"),
		"the counterer cannot see who they struck back at")
	assert.Equal(t, 0, countContaining(drainPlain(2), "COUNTER"),
		"an observer who cannot see gets no counter line")
}
