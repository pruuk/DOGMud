package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/require"
)

// TestRaptorLegsKickBonus pins the Raptor Legs kick branch so it is verified
// without a full combat harness.
func TestRaptorLegsKickBonus(t *testing.T) {
	dmg, kd := raptorLegsKickBonus(map[string]int{}, 0.80, 0.924)
	if dmg != 0.80 || kd != 0.924 {
		t.Fatalf("no mutation → unchanged, got dmg=%v kd=%v", dmg, kd)
	}
	dmg2, kd2 := raptorLegsKickBonus(map[string]int{"raptor-legs": 1}, 0.80, 0.924)
	if !(dmg2 > 0.80 && kd2 > 0.924) {
		t.Fatalf("raptor-legs should raise kick damage + knockdown, got dmg=%v kd=%v", dmg2, kd2)
	}
}

// TestMutationActivesDoNotDoubleCharge catches mutation variants adding a
// private stamina payment on top of the shared trip/kick admission.
func TestMutationActivesDoNotDoubleCharge(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		8102: {SpeciesId: 8102, Name: "target", BodyParts: []string{"arms", "hands", "legs"}},
		8201: {SpeciesId: 8201, Name: "mutant", BodyParts: []string{"arms", "hands", "legs"}},
	})
	defer cleanup()

	t.Run("tailsweep pays only through trip", func(t *testing.T) {
		actor, char, _ := newSpecialMoveAdmissionActor(t, 8201, 50, 0, false)
		char.Mutations = map[string]int{}
		char.Mutations["tail"] = 1
		result := ExecuteTrip(actor)
		cost := embeddedSpecialMoveCost(t, result)
		require.Equal(t, TripTailsweep, result.Variant)
		require.Equal(t, 4, cost.Charged)
		require.Equal(t, 46, char.Stamina)
	})

	t.Run("raptor kick pays only through kick", func(t *testing.T) {
		actor, char, _ := newSpecialMoveAdmissionActor(t, 8201, 50, 0, false)
		char.Mutations = map[string]int{}
		char.Mutations["raptor-legs"] = 1
		char.Items = nil
		result := ExecuteKick(actor)
		cost := embeddedSpecialMoveCost(t, result)
		require.Equal(t, KickStandard, result.Variant)
		require.Equal(t, 4, cost.Charged)
		require.Equal(t, 46, char.Stamina)
	})
}
