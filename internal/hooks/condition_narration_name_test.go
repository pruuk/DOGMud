package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #453: a condition line names its holder by the identity tag alone. The
// playtest read "Sil Vantage (hidden) seems to shimmer and fade from view."
// and "Sil Vantage (☀️Lit) sits down and begins to meditate.": the adjective
// span look prints had leaked into narration. The holder here carries a lit
// lantern and a pet, so every one of those decorations is in play.

// narrationDecorations are the pieces look adds to a name that a narrated
// line must never carry: the adjective span and the " and <pet>" tail.
var narrationDecorations = []string{`fg="black-bold">(`, `fg="petname"`, ` and `}

func assertNoNameDecorations(t *testing.T, line string) {
	t.Helper()
	for _, bad := range narrationDecorations {
		assert.NotContains(t, line, bad, "narration carried a look decoration: %q", line)
	}
}

func decorateHolder(t *testing.T) *users.UserRecord {
	t.Helper()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	require.True(t, holder.Character.EmitsLight(), "precondition: the holder is lit")
	holder.Character.Pet = pets.Pet{Type: `dog`, Name: `Rex`}
	return holder
}

func TestConditionNarration_PlayerHolderNameCarriesNoDecorations(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	holder := decorateHolder(t)
	defer func() { holder.Character.Pet = pets.Pet{} }()

	t.Run("start", func(t *testing.T) {
		events.DrainQueuedMessagesForTest(2)
		ApplyConditions(events.Condition{UserId: 1, ConditionId: glowConditionId})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "glows.")
		require.NotEmpty(t, line)
		assert.True(t, strings.HasPrefix(plainText(line), "Aliceia glows."), "got %q", plainText(line))
		assertNoNameDecorations(t, line)
	})

	t.Run("trigger", func(t *testing.T) {
		require.True(t, holder.Character.Conditions.AddCondition(shiverConditionId, false))
		events.DrainQueuedMessagesForTest(2)
		UserRoundTick(events.NewRound{RoundNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "shivers.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})

	t.Run("end", func(t *testing.T) {
		require.True(t, holder.Character.Conditions.AddCondition(fadeConditionId, false))
		expire(t, holder.Character.Conditions.List, fadeConditionId)
		events.DrainQueuedMessagesForTest(2)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "fades.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})
}

func TestConditionNarration_MobHolderNameCarriesNoAdjectives(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Conditions.AddCondition(lanternConditionId, false))
	require.True(t, mob.Character.EmitsLight(), "precondition: the mob is lit")

	t.Run("start", func(t *testing.T) {
		events.DrainQueuedMessagesForTest(2)
		ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "glows.")
		require.NotEmpty(t, line)
		assert.Contains(t, line, `fg="mobname`)
		assertNoNameDecorations(t, line)
	})

	t.Run("trigger", func(t *testing.T) {
		require.True(t, mob.Character.Conditions.AddCondition(shiverConditionId, false))
		events.DrainQueuedMessagesForTest(2)
		MobRoundTick(events.NewRound{RoundNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "shivers.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})

	t.Run("end", func(t *testing.T) {
		require.True(t, mob.Character.Conditions.AddCondition(fadeConditionId, false))
		expire(t, mob.Character.Conditions.List, fadeConditionId)
		events.DrainQueuedMessagesForTest(2)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "fades.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})
}
