package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two auto-spawn paths (brood-mother floor, Chrysifier homunculus) fire from
// the round tick with no player action behind them, and until U7b neither
// consulted any budget at all. Their spawn bodies need mob templates and a live
// room, neither of which this package loads, so what is pinned here is the gate
// itself: the exact predicate both bodies now run before they touch the world,
// and the text the homunculus speaks when it refuses.

// overCappedOwner builds a character whose conviction reservation already sits
// at the ceiling, so any further companion is a breach.
func overCappedOwner(t *testing.T) *characters.Character {
	t.Helper()
	ch := characters.New()
	ch.ConvictionMax.Base = 500
	require.NoError(t, ch.Validate())

	cap := ch.ReservationCap(characters.PoolConviction)
	require.Greater(t, cap, 0, "test setup: the conviction cap must be a real number")
	require.True(t, ch.AddCompanion(characters.CompanionInfo{
		InstanceId: 1, Name: "an existing companion", ConvictionReserve: cap,
	}))
	return ch
}

// spawnBroodFloor's gate. It refuses silently: the caller (tickBroodMotherFloor)
// treats nil as "back off ten rounds", which is the pre-existing failure
// handling this reuses.
func TestBroodFloorGate_RefusesWhenItWouldBreach(t *testing.T) {
	ch := overCappedOwner(t)

	reserve := ch.CalcCompanionReserve(broodFloorReserve)
	assert.Greater(t, reserve, 0, "a brood spawn must cost something")
	assert.True(t, ch.WouldBreachReservationCap(characters.PoolConviction, reserve),
		"a brood floor spawn on a full pool must be refused, not written anyway")
}

// spawnHomunculus's gate, plus the message it sends. Spoken rather than silent:
// the caller backs off for ten rounds on nil, so a silent refusal would read to
// the player exactly like the apex being broken.
func TestHomunculusGate_RefusesAndNamesReservation(t *testing.T) {
	ch := overCappedOwner(t)

	base := int(configs.GetBalanceConfig().HomunculusConvictionReserve)
	assert.Equal(t, 300, base, "owner decision 2026-08-15: the base is 300, not 1000")

	reserve := ch.CalcCompanionReserve(base)
	require.True(t, ch.WouldBreachReservationCap(characters.PoolConviction, reserve),
		"a homunculus on a full pool must be refused, not written anyway")

	msg := `Your homunculus stirs and will not hold its shape. ` +
		ch.ReservationRefusal(characters.PoolConviction, reserve)

	assert.Contains(t, msg, "reserve",
		"the refusal must name reservation, or the player has no way to learn that resting will not fix it")
	assert.NotContains(t, msg, "conviction points")
	for _, digit := range "0123456789" {
		assert.NotContains(t, msg, string(digit),
			"player-facing copy carries no raw numbers")
	}
	assert.False(t, strings.ContainsAny(msg, "–—"),
		"no en or em dashes in player copy")
	// No 80-column assertion: SendText wraps per recipient through
	// messaging.RenderForRecipient(LineWidth), so a single unwrapped sentence is
	// the correct payload shape here, matching the charm and summon refusals.
}

// A character INSIDE the ceiling is not refused. The gate must bite only on a
// genuine breach, or it would silently break both apexes for everyone.
func TestAutoSpawnGates_AllowWhenThereIsRoom(t *testing.T) {
	ch := characters.New()
	ch.ConvictionMax.Base = 5000
	require.NoError(t, ch.Validate())

	assert.False(t, ch.WouldBreachReservationCap(characters.PoolConviction,
		ch.CalcCompanionReserve(broodFloorReserve)))
	assert.False(t, ch.WouldBreachReservationCap(characters.PoolConviction,
		ch.CalcCompanionReserve(int(configs.GetBalanceConfig().HomunculusConvictionReserve))))
}
