package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Pin: a poisoned player at 1 health dies to the poison tick and the death
// cause reads "poison". Setup migrates in Task 8; the assertions do not.
func TestPin_PoisonTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	u := users.GetByUserId(1)
	u.Character.Health = 1
	u.Character.AddCondition(characters.ConditionPoisoned, 10, 5.0, "pin") // SETUP: Task 8

	// regen lands first (1 point at this fixture) and the poison tick of 5 follows: 1 + 1 - 5 <= 0
	AutoHeal(events.NewRound{RoundNumber: 30}) // SETUP: Task 8 runs UserRoundTick instead
	require.LessOrEqual(t, u.Character.Health, 0, "the poison tick must take the last point")
	require.True(t, u.Character.HasCondition(characters.ConditionPoisoned)) // SETUP: Task 8 tests the poison flag
}

// TestPin_DeathCauseOrder pins deathCauseFor (extracted from
// Death_PlayerAnnouncement.go's dmgCt==0 branch, lines 117-134 originally,
// now its own function so it can be pinned without driving the Life FSM or
// capturing worldevents.EmitWorldEvent). Order matters: poisoned wins over
// bleeding because the poison check runs first.
func TestPin_DeathCauseOrder(t *testing.T) {
	newChar := func() *characters.Character {
		c := &characters.Character{}
		c.Buffs.Validate(true)
		return c
	}

	t.Run("poisoned", func(t *testing.T) {
		c := newChar()
		c.AddCondition(characters.ConditionPoisoned, 10, 5.0, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})

	t.Run("bleeding", func(t *testing.T) {
		c := newChar()
		c.AddCondition(characters.ConditionBleeding, 10, 3.0, "pin")
		require.Equal(t, "bleeding out", deathCauseFor(c))
	})

	t.Run("poisoned and bleeding, poison wins", func(t *testing.T) {
		c := newChar()
		c.AddCondition(characters.ConditionPoisoned, 10, 5.0, "pin")
		c.AddCondition(characters.ConditionBleeding, 10, 3.0, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})
}
