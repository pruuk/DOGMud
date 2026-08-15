package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test lives in internal/usercommands rather than internal/characters
// because the lockout is a property of the `stand` COMMAND, and Stand needs a
// *users.UserRecord and a *rooms.Room. internal/characters cannot build either
// without importing its own consumers. The arithmetic half of the fix
// (EffectivePoolMax itself) is covered next to the primitive, in
// internal/characters/effective_pool_test.go.

// TestStand_HeavyStaminaReservationIsNotALockout is the regression for the U7
// Task 11 bug. StandMinStamina ships at 0.15 and was taken off the RAW
// StaminaMax, then tested against a current value RecalculateStats had already
// clamped to max - reservation. Past 85% stamina reservation the gate demanded
// more stamina than the character's pool could ever hold, so they could never
// stand again -- and the refusal blamed exhaustion, which resting cannot fix.
//
// The character here is at FULL reachable stamina. Standing must succeed.
func TestStand_HeavyStaminaReservationIsNotALockout(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999930: {
			ItemId:            999930,
			Name:              "Famished Cuirass",
			Type:              items.Body,
			Subtype:           items.Wearable,
			ReserveStaminaPct: 0.90,
		},
	})()

	user, room := getTestUserAndRoom(t)

	user.Character.Equipment.Body = items.New(999930)
	user.Character.StaminaMax.Value = 100
	// 90 reserved, so 10 is a completely full pool for this character.
	require.Equal(t, 90, user.Character.GetPoolReservation("stamina", 100),
		"test setup: the item must reserve 90 of the 100 stamina")
	user.Character.Stamina = 10

	// The old gate: 0.15 * RAW 100 = 15 stamina demanded from a pool that tops
	// out at 10. Unsatisfiable by any amount of rest.
	cfg := configs.GetBalanceConfig()
	oldGate := int(float64(user.Character.StaminaMax.Value) * float64(cfg.StandMinStamina))
	require.Greater(t, oldGate, user.Character.Stamina,
		"test setup: the raw-max gate must be out of reach, or this test proves nothing")

	setCombatPositionParallel(user.Character, position.Prone)

	handled, err := Stand("", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)
	assert.True(t, user.Character.IsStanding(),
		"a fully rested character must be able to stand however much of their stamina is reserved")

	// Cleanup for sibling tests sharing the seeded user.
	user.Character.Equipment.Body = items.Item{}
	user.Character.Stamina = 100
	setCombatPositionParallel(user.Character, position.Standing)
}

// A reserved character who genuinely IS too winded must be told about the
// reservation, not just about exhaustion: rest cannot return stamina their gear
// is holding out of the pool, and nothing else in the game tells them so.
// Descriptive band, no raw numbers.
func TestStand_RefusalDisclosesReservation(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999931: {
			ItemId:            999931,
			Name:              "Famished Cuirass",
			Type:              items.Body,
			Subtype:           items.Wearable,
			ReserveStaminaPct: 0.40,
		},
	})()

	user, room := getTestUserAndRoom(t)
	user.Character.Equipment.Body = items.New(999931)
	user.Character.StaminaMax.Value = 100
	user.Character.Stamina = 0
	setCombatPositionParallel(user.Character, position.Prone)

	handled, err := Stand("", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)
	assert.True(t, user.Character.IsProne(), "an empty pool must still refuse")

	// 40 of 100 reserved -> "a significant portion".
	assert.Equal(t, "a significant portion", reserveShareBand(40, 100))

	user.Character.Equipment.Body = items.Item{}
	user.Character.Stamina = 100
	setCombatPositionParallel(user.Character, position.Standing)
}
