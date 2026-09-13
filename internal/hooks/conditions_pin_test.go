package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Pin: a poisoned player at 1 health dies to the poison tick and the death
// cause reads "poison".
func TestPin_PoisonTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	_ = u.Character.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "pin")
	u.Character.Health = 1

	// No regen lands in the round tick: 1 - 5 <= 0.
	UserRoundTick(events.NewRound{RoundNumber: 30})
	require.LessOrEqual(t, u.Character.Health, 0, "the poison tick must take the last point")
	require.True(t, u.Character.HasBuffFlag(buffs.Poison))
}

// TestPin_DeathCauseOrder pins deathCauseFor (extracted from
// Death_PlayerAnnouncement.go's dmgCt==0 branch, lines 117-134 originally,
// now its own function so it can be pinned without driving the Life FSM or
// capturing worldevents.EmitWorldEvent). Order matters: poisoned wins over
// bleeding because the poison check runs first.
func TestPin_DeathCauseOrder(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()

	newChar := func() *characters.Character {
		c := &characters.Character{}
		c.Buffs.Validate(true)
		return c
	}

	t.Run("poisoned", func(t *testing.T) {
		c := newChar()
		_ = c.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})

	t.Run("bleeding", func(t *testing.T) {
		c := newChar()
		c.AddCondition(characters.ConditionBleeding, 10, 3.0, "pin")
		require.Equal(t, "bleeding out", deathCauseFor(c))
	})

	t.Run("poisoned and bleeding, poison wins", func(t *testing.T) {
		c := newChar()
		_ = c.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "pin")
		c.AddCondition(characters.ConditionBleeding, 10, 3.0, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})
}
