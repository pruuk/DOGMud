package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// newHiddenAttackerAndTarget builds two players in room 1 with the full
// per-character state-machine wiring live, and puts the first one into
// Awareness Hidden via the production sneak path (TransitionToConcealing →
// ResolveConcealment(true)).
//
// Validate() is what fires characters.OnCharacterCreated, which is where
// Awareness_Cascades.go and CombatPhase_Vetoes.go register their handlers.
// users.NewTestUser seeds only Awareness and Position, so without the
// Validate() call CombatPhase would be nil and SetAggro's dual-write —
// and therefore the cascade under test — would silently no-op.
func newHiddenAttackerAndTarget(t *testing.T) (*users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()

	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "Human", UnarmedName: "fist"},
	}))

	atk := users.GetByUserId(1)
	require.NotNil(t, atk)
	def := users.GetByUserId(2)
	require.NotNil(t, def)

	for _, u := range []*users.UserRecord{atk, def} {
		u.Character.SpeciesId = 1
		u.Character.Validate()
	}

	require.NotNil(t, atk.Character.CombatPhase,
		"Validate() must build the CombatPhase machine — without it the cascade never runs")
	require.NotNil(t, atk.Character.Awareness)

	require.NoError(t, atk.Character.Awareness.TransitionToConcealing(
		awareness.ConcealingData{},
		state.TransitionReason{Trigger: "test_setup"},
	))
	atk.Character.Awareness.ResolveConcealment(true, state.TransitionReason{Trigger: "test_setup"})
	require.True(t, atk.Character.IsHidden(),
		"attacker must be Hidden before the test runs")

	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	return atk, def, room
}

// A hidden attacker is revealed the moment they engage, whether or not anyone
// retaliates. Regression test for the latent bug where an ambusher stayed Hidden
// until somebody attacked them back.
func TestHiddenAttacker_IsRevealedOnEngage_WithNoRetaliation(t *testing.T) {
	atk, def, _ := newHiddenAttackerAndTarget(t)

	atk.Character.SetAggro(def.UserId, 0, characters.SurpriseAttack)

	if atk.Character.IsHidden() {
		t.Fatalf("a surprise engagement must reveal the attacker immediately")
	}
}

// ...and they still get their opening strike in that same round, because the
// bonus keys off Aggro.Type, not IsHidden(). Get this ordering wrong and the
// whole feature silently does nothing.
func TestRevealedAmbusher_StillGetsTheOpeningStrike(t *testing.T) {
	atk, def, _ := newHiddenAttackerAndTarget(t)
	atk.Character.SetAggro(def.UserId, 0, characters.SurpriseAttack)

	require.NotNil(t, atk.Character.Aggro,
		"SetAggro must have taken — otherwise this test proves nothing")
	if atk.Character.Aggro.Type != characters.SurpriseAttack {
		t.Fatalf("revealing must not clear the SurpriseAttack aggro type")
	}
}
